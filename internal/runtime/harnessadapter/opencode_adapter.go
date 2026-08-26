package harnessadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	"loom-pi-rebuild/internal/provider"
)

const (
	OpenCodeAdapterType = "opencode"
	OpenCodeProviderID  = "opencode"
)

var ErrInvalidOpenCodeAdapter = errors.New("invalid OpenCode Agent adapter")

type OpenCodeAdapterConfig struct {
	RuntimeInstanceID  string
	ExecutablePath     string
	CredentialAccess   nativeadapter.CredentialAccess
	Diagnostics        nativeadapter.AgentAttemptDiagnosticRecorder
	Runner             HarnessProcessRunner
	Now                func() time.Time
	MaxOutputBytes     int
	ContextConformance func(string, string) bool
	ToolGateway        loomruntime.AttemptToolGateway
	ToolConformance    func(string, string) bool
}

type openCodeAdapter struct {
	runtimeInstanceID  string
	executablePath     string
	credentialAccess   nativeadapter.CredentialAccess
	diagnostics        nativeadapter.AgentAttemptDiagnosticRecorder
	runner             HarnessProcessRunner
	now                func() time.Time
	maxOutputBytes     int
	contextConformance func(string, string) bool
	toolGateway        loomruntime.AttemptToolGateway
	toolConformance    func(string, string) bool
}

func NewOpenCodeAdapter(config OpenCodeAdapterConfig) (supervisor.RuntimeAdapter, error) {
	if !validHarnessID(config.RuntimeInstanceID) || config.ExecutablePath == "" ||
		nilHarnessInterface(config.CredentialAccess) ||
		nilHarnessInterface(config.Diagnostics) ||
		nilHarnessInterface(config.Runner) || config.Now == nil ||
		config.ToolGateway != nil && nilHarnessInterface(config.ToolGateway) ||
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 {
		return nil, ErrInvalidOpenCodeAdapter
	}
	contextConformance := config.ContextConformance
	if contextConformance == nil {
		contextConformance = executableHasContextRetrievalConformance
	}
	toolConformance := config.ToolConformance
	if toolConformance == nil {
		toolConformance = executableHasGovernedToolMCPConformance
	}
	return &openCodeAdapter{
		runtimeInstanceID:  config.RuntimeInstanceID,
		executablePath:     config.ExecutablePath,
		credentialAccess:   config.CredentialAccess,
		diagnostics:        config.Diagnostics,
		runner:             config.Runner,
		now:                config.Now,
		maxOutputBytes:     config.MaxOutputBytes,
		contextConformance: contextConformance,
		toolGateway:        config.ToolGateway,
		toolConformance:    toolConformance,
	}, nil
}

func (*openCodeAdapter) AdapterType() string { return OpenCodeAdapterType }

func (adapter *openCodeAdapter) RuntimeInstanceID() string {
	if adapter == nil {
		return ""
	}
	return adapter.runtimeInstanceID
}

func (adapter *openCodeAdapter) AcceptsAgentInputs() bool {
	// OpenCode session continuation is not wired yet; agent inputs fail closed.
	return false
}

func (adapter *openCodeAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || ctx == nil || nilHarnessInterface(request.FrameSink) {
		return supervisor.AdapterResult{}, ErrInvalidOpenCodeAdapter
	}
	started := time.Now()
	if err := adapter.validateRequest(request); err != nil {
		diagnosticErr := adapter.recordDiagnostic(
			ctx, request, started, "agent_attempt_dispatch", "failed",
			"binding_changed", false,
		)
		return supervisor.AdapterResult{}, errors.Join(err, diagnosticErr)
	}
	dispatch, err := decodeHarnessDispatch(request.Dispatch.Payload())
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	prompt := []byte(dispatch.Prompt)
	defer zeroHarnessBytes(prompt)
	if containsHarnessCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityContextRetrieval,
	) && !adapter.contextConformance(OpenCodeAdapterType, adapter.executablePath) {
		return supervisor.AdapterResult{}, errors.Join(
			ErrHarnessProtocol, ErrHarnessContextMCP, ErrHarnessExecutionBindingChanged,
		)
	}
	if containsHarnessCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityGovernedToolLoop,
	) && (nilHarnessInterface(adapter.toolGateway) ||
		!adapter.toolConformance(OpenCodeAdapterType, adapter.executablePath)) {
		return supervisor.AdapterResult{}, errors.Join(
			ErrHarnessProtocol, ErrHarnessContextMCP, ErrHarnessExecutionBindingChanged,
		)
	}
	contextService, err := prepareHarnessAttemptMCP(ctx, request, adapter.toolGateway)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	if contextService != nil {
		defer contextService.Close()
	}
	systemPrompt, err := buildHarnessSystemPrompt(request.ExecutionBinding)
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrHarnessProtocol, err)
	}
	modelIdentity, adaptErr := provider.OpenCodeModelIdentity(
		request.ExecutionBinding.ProviderID, request.ExecutionBinding.ModelID,
	)
	if adaptErr != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrHarnessProtocol, adaptErr)
	}
	var processResult HarnessProcessResult
	requiresCredential := request.ExecutionBinding.AuthMode == loomruntime.AuthBrokered
	var credentialErr error
	if requiresCredential {
		credentialErr = adapter.credentialAccess.UseCredential(
			ctx,
			request.ExecutionBinding,
			func(leaseContext context.Context, secret []byte) error {
				processRequest := HarnessProcessRequest{
					ExecutablePath:     adapter.executablePath,
					WorkspacePath:      request.WorkspacePath,
					HomePath:           request.HomePath,
					TempPath:           request.TempPath,
					ModelID:            modelIdentity,
					ReasoningEffort:    request.ExecutionBinding.ReasoningEffort,
					Prompt:             prompt,
					SystemPrompt:       systemPrompt,
					Timeout:            request.ExecutionBinding.Timeout,
					MaxOutputBytes:     adapter.maxOutputBytes,
					ContextMCP:         harnessContextMCPLease(contextService),
					RequiresCredential: true,
				}
				candidate, runErr := adapter.runner.RunHarness(
					leaseContext, processRequest, secret,
				)
				if runErr == nil {
					processResult = candidate
				}
				return runErr
			},
		)
	} else {
		// Native auth: OpenCode uses its own auth store through HOME; no Loom
		// credential is leased or injected.
		processRequest := HarnessProcessRequest{
			ExecutablePath:  adapter.executablePath,
			WorkspacePath:   request.WorkspacePath,
			HomePath:        request.HomePath,
			TempPath:        request.TempPath,
			ModelID:         modelIdentity,
			ReasoningEffort: request.ExecutionBinding.ReasoningEffort,
			Prompt:          prompt,
			SystemPrompt:    systemPrompt,
			Timeout:         request.ExecutionBinding.Timeout,
			MaxOutputBytes:  adapter.maxOutputBytes,
			ContextMCP:      harnessContextMCPLease(contextService),
		}
		processResult, credentialErr = adapter.runner.RunHarness(
			ctx, processRequest, nil,
		)
	}
	if credentialErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = adapter.recordDiagnostic(
				context.WithoutCancel(ctx), request, started,
				"agent_attempt_dispatch", "failed", "timeout", true,
			)
			return supervisor.AdapterResult{}, ctxErr
		}
		// A coding Harness may lose its final prose/cleanup request after Loom has
		// already authorized, executed, validated, and durably staged workspace
		// edits. Preserve those edits as a candidate so the independent verifier
		// and dependent Agent can inspect them. Classified Provider failures and
		// attempts without governed tool activity remain fail-closed.
		if recoverOpenCodeGovernedToolCandidate(credentialErr, contextService) {
			if err := acknowledgeHarnessContextMCP(ctx, contextService); err != nil {
				return supervisor.AdapterResult{}, err
			}
			if err := adapter.recordDiagnostic(
				ctx, request, started, "agent_attempt_dispatch", "succeeded", "", false,
			); err != nil {
				return supervisor.AdapterResult{}, err
			}
			return adapter.publishAttempt(
				ctx, request, started, "", "succeeded", "", nil, nil,
			)
		}
		reason, stage, retryable := harnessFailure(credentialErr)
		if err := adapter.recordDiagnostic(
			ctx, request, started, stage, "failed", reason, retryable,
		); err != nil {
			return supervisor.AdapterResult{}, err
		}
		return adapter.publishAttempt(
			ctx, request, started, "", "failed", reason, nil, nil,
		)
	}
	processResult, err = normalizeOpenCodeHarnessProcessResult(
		processResult, adapter.maxOutputBytes, contextService,
	)
	if err != nil {
		_ = adapter.recordDiagnostic(
			ctx, request, started, "agent_attempt_response", "failed",
			"invalid_response", false,
		)
		return supervisor.AdapterResult{}, err
	}
	if err := acknowledgeHarnessContextMCP(ctx, contextService); err != nil {
		if diagnosticErr := adapter.recordDiagnostic(
			ctx, request, started, "context_delivery", "failed",
			"context_delivery_unavailable", true,
		); diagnosticErr != nil {
			return supervisor.AdapterResult{}, errors.Join(err, diagnosticErr)
		}
		return supervisor.AdapterResult{}, err
	}
	if err := adapter.recordDiagnostic(
		ctx, request, started, "agent_attempt_dispatch", "succeeded", "", false,
	); err != nil {
		return supervisor.AdapterResult{}, err
	}
	return adapter.publishAttempt(
		ctx, request, started, processResult.Content, "succeeded", "",
		processResult.Stderr, processResult.Accounting,
	)
}

func (adapter *openCodeAdapter) publishAttempt(
	ctx context.Context,
	request supervisor.AdapterRequest,
	started time.Time,
	content, status, reason string,
	stderr []byte,
	accounting *work.RunAccounting,
) (supervisor.AdapterResult, error) {
	result, err := adapter.publish(
		ctx, request, content, status, reason, stderr, accounting,
	)
	if err == nil {
		return result, nil
	}
	_ = adapter.recordDiagnostic(
		context.WithoutCancel(ctx), request, started,
		"agent_attempt_response", "failed", "bridge_protocol_failed", true,
	)
	return result, err
}

func recoverOpenCodeGovernedToolCandidate(
	err error,
	contextService *harnessContextMCP,
) bool {
	if classifiedOpenCodeProcessFailure(err) {
		return false
	}
	opaqueFailure := errors.Is(err, ErrHarnessProcessUnavailable) ||
		errors.Is(err, provider.ErrOpenCodeConversationUnavailable)
	return opaqueFailure &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) &&
		harnessContextMCPHasToolActivity(contextService)
}

func normalizeOpenCodeHarnessProcessResult(
	result HarnessProcessResult,
	maximum int,
	contextService *harnessContextMCP,
) (HarnessProcessResult, error) {
	if validateHarnessProcessResult(result, maximum) == nil {
		return result, nil
	}
	if !harnessContextMCPHasToolActivity(contextService) ||
		len(result.Stderr) > maximum ||
		result.Accounting != nil && work.ValidateRunAccounting(*result.Accounting) != nil {
		return HarnessProcessResult{}, ErrHarnessProtocol
	}
	// The governed workspace is the candidate authority. OpenCode may emit an
	// empty, oversized, or otherwise unusable final prose response after all
	// authorized tool results have committed. Drop that prose and let the
	// independent verifier judge the staged workspace instead of discarding it.
	result.Content = ""
	return result, nil
}

func (adapter *openCodeAdapter) validateRequest(request supervisor.AdapterRequest) error {
	binding, err := loomruntime.ValidateFrozenExecutionBinding(request.ExecutionBinding)
	if err != nil || binding.HarnessAdapter != OpenCodeAdapterType ||
		binding.RuntimeInstanceID != adapter.runtimeInstanceID ||
		binding.RuntimeInstanceID != request.Binding.RuntimeInstanceID ||
		(binding.AuthMode != loomruntime.AuthBrokered &&
			binding.AuthMode != loomruntime.AuthNative) ||
		binding.AuthMode == loomruntime.AuthBrokered &&
			(binding.CredentialReference == "" || binding.CredentialRevision <= 0) ||
		!containsHarnessCapability(binding.Capabilities, "workspace_edit") {
		return ErrHarnessExecutionBindingChanged
	}
	modelIdentity, adaptErr := provider.OpenCodeModelIdentity(
		binding.ProviderID, binding.ModelID,
	)
	if adaptErr != nil || !validOpenCodeReasoningEffort(
		modelIdentity, binding.ReasoningEffort,
	) {
		return ErrHarnessExecutionBindingChanged
	}
	if request.Dispatch.Type() != bridgev1.MessageDispatch ||
		request.Dispatch.Sequence() != 1 ||
		request.Dispatch.WorkItemID() != request.Binding.WorkItemID ||
		request.Dispatch.RunID() != request.Binding.RunID ||
		request.Dispatch.ClaimGeneration() != request.Binding.ClaimGeneration ||
		request.Dispatch.RuntimeInstanceID() != request.Binding.RuntimeInstanceID ||
		request.Dispatch.SenderAgentInstanceID() != request.Binding.SenderAgentInstanceID {
		return errors.Join(ErrHarnessProtocol, supervisor.ErrBridgeSession)
	}
	return nil
}

func (adapter *openCodeAdapter) recordDiagnostic(
	ctx context.Context,
	request supervisor.AdapterRequest,
	started time.Time,
	stage, result, errorCode string,
	retryable bool,
) error {
	now := adapter.now()
	if now.IsZero() || now.Location() != time.UTC {
		return nativeadapter.ErrAgentDiagnosticsUnavailable
	}
	elapsed := time.Since(started)
	if elapsed < 0 {
		elapsed = 0
	}
	if err := adapter.diagnostics.RecordAgentAttemptDiagnostic(
		ctx,
		nativeadapter.AgentAttemptDiagnostic{
			OccurredAt: now, IncidentID: request.Dispatch.CorrelationID(),
			ProviderID:        request.ExecutionBinding.ProviderID,
			ProviderAccountID: request.ExecutionBinding.ProviderAccountID,
			ModelID:           request.ExecutionBinding.ModelID, Stage: stage,
			Elapsed: elapsed, Result: result, ErrorCode: errorCode, Retryable: retryable,
		},
	); err != nil {
		return nativeadapter.ErrAgentDiagnosticsUnavailable
	}
	return nil
}

func (adapter *openCodeAdapter) publish(
	ctx context.Context,
	request supervisor.AdapterRequest,
	content, status, reason string,
	stderr []byte,
	accounting *work.RunAccounting,
) (supervisor.AdapterResult, error) {
	records := []struct {
		kind bridgev1.MessageType
		body any
	}{
		{kind: bridgev1.MessageAck, body: struct {
			MessageID string `json:"message_id"`
		}{MessageID: request.Dispatch.MessageID()}},
	}
	if status == "succeeded" && content != "" {
		records = append(records, struct {
			kind bridgev1.MessageType
			body any
		}{kind: bridgev1.MessageEvent, body: struct {
			Delta string `json:"delta"`
		}{Delta: content}})
	}
	records = append(records, struct {
		kind bridgev1.MessageType
		body any
	}{kind: bridgev1.MessageResult, body: struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}{Status: status, Reason: reason}})
	frames := make([]bridgev1.Frame, 0, len(records))
	for index, record := range records {
		payload, err := json.Marshal(record.body)
		if err != nil {
			return supervisor.AdapterResult{}, ErrHarnessProtocol
		}
		now := adapter.now()
		if now.IsZero() || now.Location() != time.UTC {
			return supervisor.AdapterResult{}, ErrInvalidOpenCodeAdapter
		}
		frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID: deterministicHarnessUUID(
				request.Dispatch.MessageID(), string(record.kind), fmt.Sprintf("%d", index+2),
			),
			CorrelationID: request.Dispatch.CorrelationID(),
			WorkItemID:    request.Binding.WorkItemID, RunID: request.Binding.RunID,
			ClaimGeneration:       request.Binding.ClaimGeneration,
			RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
			SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
			Sequence:              int64(index + 2), Type: record.kind, EmittedAt: now,
			Payload: payload,
		})
		if err != nil {
			return supervisor.AdapterResult{}, errors.Join(ErrHarnessProtocol, supervisor.ErrBridgeSession)
		}
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, errors.Join(
				ErrHarnessProtocol, supervisor.ErrBridgeSession, err,
			)
		}
		frames = append(frames, frame)
	}
	result, err := supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames: frames, Stderr: stderr, ExitCode: 0,
		DispatchAcknowledged: true, ResultAcknowledged: true, Accounting: accounting,
	})
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrHarnessProtocol, err)
	}
	return result, nil
}
