package harnessadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const (
	ClaudeCodeAdapterType         = "claude-code"
	ClaudeCodeProviderID          = "anthropic"
	ClaudeCodeModelID             = "claude-sonnet-5"
	ClaudeCodeEndpoint            = "https://api.anthropic.com/v1/messages"
	ClaudeCodeEndpointFingerprint = "dd9dd182b406f181aa1efb245fd4182c6588c3430bcab8d7db04199ab682acbc"

	maxHarnessPromptBytes = 64 << 10
)

var (
	ErrInvalidClaudeCodeAdapter       = errors.New("invalid Claude Code Agent adapter")
	ErrHarnessExecutionBindingChanged = errors.New("Harness execution binding changed")
	ErrHarnessProtocol                = errors.New("Harness Agent protocol failure")
	ErrHarnessProcessUnavailable      = errors.New("Harness process unavailable")
	ErrHarnessProviderUnavailable     = errors.New("Harness Provider unavailable")
	ErrHarnessProviderAuth            = errors.New("Harness Provider authentication failed")
	ErrHarnessProviderRateLimit       = errors.New("Harness Provider rate limited")
	ErrHarnessProviderRejected        = errors.New("Harness Provider rejected request")
	ErrHarnessProviderTimeout         = errors.New("Harness Provider request timed out")
	ErrHarnessProviderResponseTimeout = errors.New("Harness Provider response timed out")
)

type HarnessProcessRequest struct {
	ExecutablePath  string
	WorkspacePath   string
	HomePath        string
	TempPath        string
	ModelID         string
	ReasoningEffort string
	Prompt          []byte
	SystemPrompt    string
	Timeout         time.Duration
	MaxOutputBytes  int
	ContextMCP      HarnessContextMCPLease
	// RequiresCredential is true when the execution binding is brokered and a
	// non-empty credential secret must be injected. Native-auth harnesses
	// (for example OpenCode using its own auth store) run with an empty
	// secret.
	RequiresCredential bool
}

type HarnessProcessResult struct {
	Content    string
	Stderr     []byte
	Accounting *work.RunAccounting
}

type HarnessProcessRunner interface {
	RunHarness(
		context.Context,
		HarnessProcessRequest,
		[]byte,
	) (HarnessProcessResult, error)
}

type ClaudeCodeAdapterConfig struct {
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

type claudeCodeAdapter struct {
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

type harnessDispatch struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Prompt        string `json:"prompt"`
}

func NewClaudeCodeAdapter(
	config ClaudeCodeAdapterConfig,
) (supervisor.RuntimeAdapter, error) {
	if !validHarnessID(config.RuntimeInstanceID) || config.ExecutablePath == "" ||
		nilHarnessInterface(config.CredentialAccess) ||
		nilHarnessInterface(config.Diagnostics) ||
		nilHarnessInterface(config.Runner) || config.Now == nil ||
		config.ToolGateway != nil && nilHarnessInterface(config.ToolGateway) ||
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 {
		return nil, ErrInvalidClaudeCodeAdapter
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
	return &claudeCodeAdapter{
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

func (*claudeCodeAdapter) AdapterType() string { return ClaudeCodeAdapterType }

func (adapter *claudeCodeAdapter) RuntimeInstanceID() string {
	if adapter == nil {
		return ""
	}
	return adapter.runtimeInstanceID
}

func (adapter *claudeCodeAdapter) AcceptsAgentInputs() bool {
	if adapter == nil || adapter.continuationConformance == nil {
		return false
	}
	runner, ok := adapter.runner.(HarnessAgentInputRunner)
	return ok && !nilHarnessInterface(runner) && runner.SupportsAgentInputs() &&
		adapter.continuationConformance(ClaudeCodeAdapterType, adapter.executablePath)
}

func (adapter *claudeCodeAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || ctx == nil || nilHarnessInterface(request.FrameSink) {
		return supervisor.AdapterResult{}, ErrInvalidClaudeCodeAdapter
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
	) && !adapter.contextConformance(ClaudeCodeAdapterType, adapter.executablePath) {
		return supervisor.AdapterResult{}, errors.Join(
			ErrHarnessProtocol, ErrHarnessContextMCP, ErrHarnessExecutionBindingChanged,
		)
	}
	if containsHarnessCapability(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityGovernedToolLoop,
	) && (nilHarnessInterface(adapter.toolGateway) ||
		!adapter.toolConformance(ClaudeCodeAdapterType, adapter.executablePath)) {
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
			!adapter.continuationConformance(ClaudeCodeAdapterType, adapter.executablePath) {
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
			if len(secret) == 0 || len(secret) > 8192 {
				return nativeadapter.ErrAgentCredentialUnavailable
			}
			processRequest := HarnessProcessRequest{
				ExecutablePath:  adapter.executablePath,
				WorkspacePath:   request.WorkspacePath,
				HomePath:        request.HomePath,
				TempPath:        request.TempPath,
				ModelID:         request.ExecutionBinding.ModelID,
				ReasoningEffort: request.ExecutionBinding.ReasoningEffort,
				Prompt:          prompt,
				SystemPrompt:    systemPrompt,
				Timeout:         request.ExecutionBinding.Timeout,
				MaxOutputBytes:  adapter.maxOutputBytes,
				ContextMCP:      harnessContextMCPLease(contextService),
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
		if diagnosticErr := adapter.recordDiagnostic(
			ctx, request, started, "agent_attempt_dispatch", "failed",
			"harness_protocol", false,
		); diagnosticErr != nil {
			return supervisor.AdapterResult{}, errors.Join(err, diagnosticErr)
		}
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
		ctx,
		request,
		processResult.Content,
		"succeeded",
		"",
		processResult.Stderr,
		processResult.Accounting,
	)
}

func (adapter *claudeCodeAdapter) validateRequest(
	request supervisor.AdapterRequest,
) error {
	binding, err := loomruntime.ValidateFrozenExecutionBinding(request.ExecutionBinding)
	if err != nil || binding.HarnessAdapter != ClaudeCodeAdapterType ||
		binding.RuntimeInstanceID != adapter.runtimeInstanceID ||
		binding.RuntimeInstanceID != request.Binding.RuntimeInstanceID ||
		binding.ProviderID != ClaudeCodeProviderID ||
		binding.ProviderAccountID == "" || binding.ModelID != ClaudeCodeModelID ||
		binding.AuthMode != loomruntime.AuthBrokered ||
		binding.EndpointFingerprint != ClaudeCodeEndpointFingerprint ||
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

func (adapter *claudeCodeAdapter) recordDiagnostic(
	ctx context.Context,
	request supervisor.AdapterRequest,
	started time.Time,
	stage,
	result,
	errorCode string,
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
			ModelID:           request.ExecutionBinding.ModelID,
			Stage:             stage, Elapsed: elapsed, Result: result,
			ErrorCode: errorCode, Retryable: retryable,
		},
	); err != nil {
		return nativeadapter.ErrAgentDiagnosticsUnavailable
	}
	return nil
}

func harnessFailure(err error) (reason, stage string, retryable bool) {
	switch {
	case errors.Is(err, nativeadapter.ErrAgentCredentialUnavailable):
		stage := credentials.CredentialFailureStage(err)
		if stage == "" {
			stage = credentials.CredentialStageLeaseIssue
		}
		return "credential_unavailable", stage, credentials.CredentialFailureRetryable(stage)
	case errors.Is(err, ErrHarnessProviderAuth):
		return "provider_auth", "provider_auth", false
	case errors.Is(err, ErrHarnessProviderRateLimit):
		return "provider_rate_limit", "provider_rate_limit", true
	case errors.Is(err, ErrHarnessProviderRejected):
		return "provider_rejected", "provider_http", false
	case errors.Is(err, ErrHarnessProviderUnavailable):
		return "provider_unavailable", "provider_http", true
	case errors.Is(err, ErrHarnessProviderTimeout):
		return "timeout", "provider_connect", true
	case errors.Is(err, ErrHarnessProviderResponseTimeout):
		return "timeout", "provider_http", true
	default:
		return "harness_unavailable", "agent_attempt_dispatch", true
	}
}

func validateHarnessProcessResult(result HarnessProcessResult, maximum int) error {
	content := strings.TrimSpace(result.Content)
	if content == "" || len(content) > maximum || !utf8.ValidString(content) ||
		strings.IndexByte(content, 0) >= 0 || len(result.Stderr) > maximum {
		return ErrHarnessProtocol
	}
	if result.Accounting != nil && work.ValidateRunAccounting(*result.Accounting) != nil {
		return ErrHarnessProtocol
	}
	return nil
}

func (adapter *claudeCodeAdapter) publish(
	ctx context.Context,
	request supervisor.AdapterRequest,
	content,
	status,
	reason string,
	stderr []byte,
	accounting *work.RunAccounting,
) (supervisor.AdapterResult, error) {
	records := []struct {
		kind    bridgev1.MessageType
		payload any
	}{
		{kind: bridgev1.MessageAck, payload: struct {
			MessageID string `json:"message_id"`
		}{MessageID: request.Dispatch.MessageID()}},
	}
	if status == "succeeded" {
		records = append(records, struct {
			kind    bridgev1.MessageType
			payload any
		}{kind: bridgev1.MessageEvent, payload: struct {
			Delta string `json:"delta"`
		}{Delta: strings.TrimSpace(content)}})
	}
	records = append(records, struct {
		kind    bridgev1.MessageType
		payload any
	}{kind: bridgev1.MessageResult, payload: struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}{Status: status, Reason: reason}})

	frames := make([]bridgev1.Frame, 0, len(records))
	for index, record := range records {
		payload, err := json.Marshal(record.payload)
		if err != nil {
			return supervisor.AdapterResult{}, ErrHarnessProtocol
		}
		now := adapter.now()
		if now.IsZero() || now.Location() != time.UTC {
			return supervisor.AdapterResult{}, ErrInvalidClaudeCodeAdapter
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
			return supervisor.AdapterResult{}, errors.Join(ErrHarnessProtocol, supervisor.ErrBridgeSession, err)
		}
		frames = append(frames, frame)
	}
	result, err := supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames: frames, Stderr: stderr, ExitCode: 0,
		DispatchAcknowledged: true, ResultAcknowledged: true,
		Accounting: accounting,
	})
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(ErrHarnessProtocol, err)
	}
	return result, nil
}

func decodeHarnessDispatch(payload []byte) (harnessDispatch, error) {
	if contextDispatch, err := contextcapsule.DecodeDispatchPayload(payload); err == nil {
		if len(contextDispatch.Prompt) > maxHarnessPromptBytes {
			return harnessDispatch{}, errors.Join(ErrHarnessProtocol, supervisor.ErrBridgeSession)
		}
		return harnessDispatch{
			SchemaVersion: 2,
			Kind:          "loom_context_capsule_prompt",
			Prompt:        contextDispatch.Prompt,
		}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var dispatch harnessDispatch
	if decoder.Decode(&dispatch) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		dispatch.SchemaVersion != 1 || dispatch.Kind != "pi_rpc_prompt" ||
		dispatch.Prompt == "" || len(dispatch.Prompt) > maxHarnessPromptBytes ||
		!utf8.ValidString(dispatch.Prompt) || strings.IndexByte(dispatch.Prompt, 0) >= 0 {
		return harnessDispatch{}, errors.Join(ErrHarnessProtocol, supervisor.ErrBridgeSession)
	}
	canonical, err := json.Marshal(dispatch)
	if err != nil || !bytes.Equal(canonical, payload) {
		return harnessDispatch{}, errors.Join(ErrHarnessProtocol, supervisor.ErrBridgeSession)
	}
	return dispatch, nil
}

func deterministicHarnessUUID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	digest[6] = digest[6]&0x0f | 0x40
	digest[8] = digest[8]&0x3f | 0x80
	encoded := hex.EncodeToString(digest[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32]
}

func containsHarnessCapability(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validHarnessID(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func nilHarnessInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
