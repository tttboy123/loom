package harnessadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const (
	CodexAdapterType         = "codex"
	CodexProviderID          = "openai"
	CodexModelID             = "gpt-5.5-codex"
	CodexEndpoint            = "https://api.openai.com/v1/responses"
	CodexEndpointFingerprint = "ee0291cefbb5b6136483fb38ba9efe9264f9b685d5006c273e293a54b43a1883"
)

var ErrInvalidCodexAdapter = errors.New("invalid Codex Agent adapter")

type CodexAdapterConfig struct {
	RuntimeInstanceID       string
	ExecutablePath          string
	CredentialAccess        nativeadapter.CredentialAccess
	Diagnostics             nativeadapter.AgentAttemptDiagnosticRecorder
	Runner                  HarnessProcessRunner
	Now                     func() time.Time
	MaxOutputBytes          int
	ContextConformance      func(string, string) bool
	ToolGateway             loomruntime.AttemptToolGateway
	ToolConformance         func(string, string) bool
	ContinuationConformance func(string, string) bool
}

type codexAdapter struct {
	runtimeInstanceID       string
	executablePath          string
	credentialAccess        nativeadapter.CredentialAccess
	diagnostics             nativeadapter.AgentAttemptDiagnosticRecorder
	runner                  HarnessProcessRunner
	now                     func() time.Time
	maxOutputBytes          int
	contextConformance      func(string, string) bool
	toolGateway             loomruntime.AttemptToolGateway
	toolConformance         func(string, string) bool
	continuationConformance func(string, string) bool
}

func NewCodexAdapter(config CodexAdapterConfig) (supervisor.RuntimeAdapter, error) {
	if !validHarnessID(config.RuntimeInstanceID) || config.ExecutablePath == "" ||
		nilHarnessInterface(config.CredentialAccess) ||
		nilHarnessInterface(config.Diagnostics) ||
		nilHarnessInterface(config.Runner) || config.Now == nil ||
		config.ToolGateway != nil && nilHarnessInterface(config.ToolGateway) ||
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 {
		return nil, ErrInvalidCodexAdapter
	}
	contextConformance := config.ContextConformance
	if contextConformance == nil {
		contextConformance = executableHasContextRetrievalConformance
	}
	toolConformance := config.ToolConformance
	if toolConformance == nil {
		toolConformance = executableHasGovernedToolMCPConformance
	}
	continuationConformance := config.ContinuationConformance
	if continuationConformance == nil {
		continuationConformance = executableHasAgentInputContinuationConformance
	}
	return &codexAdapter{
		runtimeInstanceID:       config.RuntimeInstanceID,
		executablePath:          config.ExecutablePath,
		credentialAccess:        config.CredentialAccess,
		diagnostics:             config.Diagnostics,
		runner:                  config.Runner,
		now:                     config.Now,
		maxOutputBytes:          config.MaxOutputBytes,
		contextConformance:      contextConformance,
		toolGateway:             config.ToolGateway,
		toolConformance:         toolConformance,
		continuationConformance: continuationConformance,
	}, nil
}

func (*codexAdapter) AdapterType() string { return CodexAdapterType }

func (adapter *codexAdapter) RuntimeInstanceID() string {
	if adapter == nil {
		return ""
	}
	return adapter.runtimeInstanceID
}

func (adapter *codexAdapter) AcceptsAgentInputs() bool {
	if adapter == nil || adapter.continuationConformance == nil {
		return false
	}
	runner, ok := adapter.runner.(HarnessAgentInputRunner)
	return ok && !nilHarnessInterface(runner) && runner.SupportsAgentInputs() &&
		adapter.continuationConformance(CodexAdapterType, adapter.executablePath)
}

func (adapter *codexAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || ctx == nil || nilHarnessInterface(request.FrameSink) {
		return supervisor.AdapterResult{}, ErrInvalidCodexAdapter
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
	) && !adapter.contextConformance(CodexAdapterType, adapter.executablePath) {
		return supervisor.AdapterResult{}, errors.Join(
			ErrHarnessProtocol, ErrHarnessContextMCP, ErrHarnessExecutionBindingChanged,
		)
	}
	if containsHarnessCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityGovernedToolLoop,
	) && (nilHarnessInterface(adapter.toolGateway) ||
		!adapter.toolConformance(CodexAdapterType, adapter.executablePath)) {
		return supervisor.AdapterResult{}, errors.Join(
			ErrHarnessProtocol, ErrHarnessContextMCP, ErrHarnessExecutionBindingChanged,
		)
	}
	var continuationRunner HarnessAgentInputRunner
	if request.AgentInputs != nil {
		var ok bool
		continuationRunner, ok = adapter.runner.(HarnessAgentInputRunner)
		if !ok || nilHarnessInterface(continuationRunner) ||
			!continuationRunner.SupportsAgentInputs() ||
			!adapter.continuationConformance(CodexAdapterType, adapter.executablePath) {
			return supervisor.AdapterResult{}, errors.Join(
				ErrHarnessProtocol, ErrHarnessExecutionBindingChanged,
			)
		}
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
	var processResult HarnessProcessResult
	credentialErr := adapter.credentialAccess.UseCredential(
		ctx,
		request.ExecutionBinding,
		func(leaseContext context.Context, secret []byte) error {
			processRequest := HarnessProcessRequest{
				ExecutablePath: adapter.executablePath,
				WorkspacePath:  request.WorkspacePath,
				HomePath:       request.HomePath, TempPath: request.TempPath,
				ModelID:         request.ExecutionBinding.ModelID,
				ReasoningEffort: request.ExecutionBinding.ReasoningEffort,
				Prompt:          prompt, SystemPrompt: systemPrompt,
				Timeout:        request.ExecutionBinding.Timeout,
				MaxOutputBytes: adapter.maxOutputBytes,
				ContextMCP:     harnessContextMCPLease(contextService),
			}
			var candidate HarnessProcessResult
			var runErr error
			if request.AgentInputs != nil {
				candidate, runErr = continuationRunner.RunHarnessWithAgentInputs(
					leaseContext, processRequest, secret, request.AgentInputs,
				)
			} else {
				candidate, runErr = adapter.runner.RunHarness(
					leaseContext, processRequest, secret,
				)
			}
			if runErr == nil {
				processResult = candidate
			}
			return runErr
		},
	)
	if credentialErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = adapter.recordDiagnostic(
				context.WithoutCancel(ctx), request, started,
				"agent_attempt_dispatch", "failed", "timeout", true,
			)
			return supervisor.AdapterResult{}, ctxErr
		}
		reason, stage, retryable := harnessFailure(credentialErr)
		if err := adapter.recordDiagnostic(
			ctx, request, started, stage, "failed", reason, retryable,
		); err != nil {
			return supervisor.AdapterResult{}, err
		}
		return adapter.publish(ctx, request, "", "failed", reason, nil, nil)
	}
	if err := validateHarnessProcessResult(processResult, adapter.maxOutputBytes); err != nil {
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
	return adapter.publish(
		ctx, request, processResult.Content, "succeeded", "",
		processResult.Stderr, processResult.Accounting,
	)
}

func prepareHarnessContextMCP(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (*harnessContextMCP, error) {
	return prepareHarnessAttemptMCP(ctx, request, nil)
}

func prepareHarnessAttemptMCP(
	ctx context.Context,
	request supervisor.AdapterRequest,
	toolGateway loomruntime.AttemptToolGateway,
) (*harnessContextMCP, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, errors.Join(ErrHarnessProtocol, ErrHarnessContextMCP)
	}
	contextCapable := containsHarnessCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityContextRetrieval,
	)
	toolCapable := containsHarnessCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityGovernedToolLoop,
	)
	retrieverPresent := request.ContextRetriever != nil &&
		!nilHarnessInterface(request.ContextRetriever)
	deliveryPresent := request.ContextDelivery != nil &&
		!nilHarnessInterface(request.ContextDelivery)
	if contextCapable != retrieverPresent || contextCapable != deliveryPresent ||
		request.ContextRetriever != nil && !retrieverPresent ||
		request.ContextDelivery != nil && !deliveryPresent ||
		toolCapable && nilHarnessInterface(toolGateway) {
		return nil, errors.Join(ErrHarnessProtocol, ErrHarnessContextMCP)
	}
	if !contextCapable && !toolCapable {
		return nil, nil
	}
	authority, err := contextcapsule.ValidateAuthorityRecord(request.ContextCapsule)
	if err != nil || authority.AgentID != request.Binding.SenderAgentInstanceID ||
		authority.ProviderID != request.ExecutionBinding.ProviderID ||
		authority.ProviderAccountID != request.ExecutionBinding.ProviderAccountID ||
		authority.ModelID != request.ExecutionBinding.ModelID ||
		authority.AuthMode != string(request.ExecutionBinding.AuthMode) {
		return nil, errors.Join(ErrHarnessProtocol, ErrHarnessContextMCP, err)
	}
	dispatch, err := contextcapsule.DecodeDispatchPayload(request.Dispatch.Payload())
	if err != nil || dispatch.CapsuleDigest != authority.CapsuleDigest ||
		dispatch.DisclosureReceiptDigest != authority.DisclosureReceiptDigest {
		return nil, errors.Join(ErrHarnessProtocol, ErrHarnessContextMCP, err)
	}
	config := harnessAttemptMCPConfig{}
	if contextCapable {
		config.Delivery = request.ContextDelivery
	}
	if toolCapable {
		segment, segmentErr := contextcapsule.ValidateRouteSegmentBinding(request.RouteSegment)
		if segmentErr != nil || segment.ConversationID != authority.ConversationID ||
			segment.TeamID != authority.TeamID || segment.AgentID != authority.AgentID ||
			segment.RoleID != authority.RoleID || segment.CapsuleDigest != authority.CapsuleDigest ||
			segment.ExecutionBindingDigest != request.ExecutionBinding.BindingDigest ||
			request.ClaimID == "" || request.IncidentID == "" ||
			request.IncidentID != request.Dispatch.CorrelationID() {
			return nil, errors.Join(
				ErrHarnessProtocol, ErrHarnessContextMCP, segmentErr,
			)
		}
		config.ToolGateway = toolGateway
		config.ToolBinding = loomruntime.ToolCallBinding{
			ConversationID: authority.ConversationID,
			WorkItemID:     request.Binding.WorkItemID, RunID: request.Binding.RunID,
			ClaimGeneration:        request.Binding.ClaimGeneration,
			RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
			AgentInstanceID:        request.Binding.SenderAgentInstanceID,
			ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
			CapsuleDigest:          authority.CapsuleDigest, ClaimID: request.ClaimID,
			IncidentID: request.IncidentID, JourneyID: request.Dispatch.CorrelationID(),
		}
	}
	service, err := newHarnessAttemptMCPWithContext(ctx, config)
	if err != nil {
		return nil, errors.Join(ErrHarnessProtocol, err)
	}
	return service, nil
}

func harnessContextMCPLease(service *harnessContextMCP) HarnessContextMCPLease {
	if service == nil {
		return HarnessContextMCPLease{}
	}
	return service.Lease()
}

func (adapter *codexAdapter) validateRequest(request supervisor.AdapterRequest) error {
	binding, err := loomruntime.ValidateFrozenExecutionBinding(request.ExecutionBinding)
	if err != nil || binding.HarnessAdapter != CodexAdapterType ||
		binding.RuntimeInstanceID != adapter.runtimeInstanceID ||
		binding.RuntimeInstanceID != request.Binding.RuntimeInstanceID ||
		binding.ProviderID != CodexProviderID || binding.ProviderAccountID == "" ||
		binding.ModelID != CodexModelID || binding.AuthMode != loomruntime.AuthBrokered ||
		binding.EndpointFingerprint != CodexEndpointFingerprint ||
		binding.CredentialReference == "" || binding.CredentialRevision <= 0 ||
		!containsHarnessCapability(binding.Capabilities, "workspace_edit") {
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

func (adapter *codexAdapter) recordDiagnostic(
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

func (adapter *codexAdapter) publish(
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
	if status == "succeeded" {
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
			return supervisor.AdapterResult{}, ErrInvalidCodexAdapter
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
