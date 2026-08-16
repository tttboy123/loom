package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestHarnessDispatchAcceptsCanonicalRoleContextPayload(t *testing.T) {
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-1", TeamID: "team-1",
			AgentID: "agent-claude", RoleID: "reviewer",
			ProviderID: "anthropic", ProviderAccountID: "anthropic.primary",
			ModelID: ClaudeCodeModelID, AuthMode: "brokered",
			ContextAdapterID:   "context:claude-code:v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			TokenBudget: 64,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
			Content:    []byte("Review the admitted implementation."),
			SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:phase-2d",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := decodeHarnessDispatch(payload)
	if err != nil || !strings.Contains(dispatch.Prompt, "Review the admitted implementation.") {
		t.Fatalf("context dispatch = %#v, %v", dispatch, err)
	}
}

type credentialAccessFixture struct {
	secret  []byte
	err     error
	uses    int
	binding loomruntime.FrozenExecutionBinding
}

func (access *credentialAccessFixture) UseCredential(
	ctx context.Context,
	binding loomruntime.FrozenExecutionBinding,
	use func(context.Context, []byte) error,
) error {
	access.uses++
	access.binding = binding
	if access.err != nil {
		return access.err
	}
	secret := append([]byte(nil), access.secret...)
	defer clearBytes(secret)
	return use(ctx, secret)
}

type processRunnerFixture struct {
	result  HarnessProcessResult
	err     error
	hook    func(context.Context, HarnessProcessRequest) error
	runs    int
	request HarnessProcessRequest
	secret  []byte
}

func (runner *processRunnerFixture) RunHarness(
	ctx context.Context,
	request HarnessProcessRequest,
	secret []byte,
) (HarnessProcessResult, error) {
	runner.runs++
	runner.request = request
	runner.request.Prompt = bytes.Clone(request.Prompt)
	runner.secret = append([]byte(nil), secret...)
	if runner.hook != nil {
		if err := runner.hook(ctx, request); err != nil {
			return HarnessProcessResult{}, err
		}
	}
	return runner.result, runner.err
}

type diagnosticRecorderFixture struct {
	records []nativeadapter.AgentAttemptDiagnostic
}

func (recorder *diagnosticRecorderFixture) RecordAgentAttemptDiagnostic(
	_ context.Context,
	record nativeadapter.AgentAttemptDiagnostic,
) error {
	recorder.records = append(recorder.records, record)
	return nil
}

type frameSinkFixture struct {
	frames []bridgev1.Frame
}

func (sink *frameSinkFixture) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	return nil
}

func TestClaudeCodeAdapterConsumesOnlyItsFrozenAnthropicAccount(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-anthropic-key")}
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "Implemented the bounded change",
		Accounting: &work.RunAccounting{
			UsageObserved: true,
			InputTokens:   120, OutputTokens: 31, TotalTokens: 151,
			CacheReadTokens: 20,
			CostObserved:    true, CostMicrounits: 275000, CostCurrency: "USD",
			CostSource: work.CostSourceHarnessReported,
		},
	}}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
		RuntimeInstanceID: "runtime.claude-code.local",
		ExecutablePath:    "/opt/loom/bin/claude",
		CredentialAccess:  access,
		Diagnostics:       diagnostics,
		Runner:            runner,
		Now: func() time.Time {
			return time.Date(2026, 8, 10, 14, 0, 0, 0, time.UTC)
		},
		MaxOutputBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	if access.uses != 1 || !reflect.DeepEqual(access.binding, request.ExecutionBinding) {
		t.Fatalf("credential access = uses %d binding %#v", access.uses, access.binding)
	}
	if runner.runs != 1 || string(runner.secret) != "private-anthropic-key" {
		t.Fatalf("runner = runs %d secret %q", runner.runs, runner.secret)
	}
	if runner.request.ExecutablePath != "/opt/loom/bin/claude" ||
		runner.request.WorkspacePath != request.WorkspacePath ||
		runner.request.HomePath != request.HomePath ||
		runner.request.TempPath != request.TempPath ||
		runner.request.ModelID != ClaudeCodeModelID ||
		!bytes.Equal(runner.request.Prompt, []byte("Implement the bounded change")) ||
		!strings.Contains(runner.request.SystemPrompt, "provider=anthropic") ||
		!strings.Contains(runner.request.SystemPrompt, "model="+ClaudeCodeModelID) ||
		!strings.Contains(runner.request.SystemPrompt, "MCPTool") ||
		strings.Contains(runner.request.SystemPrompt, "Edit") ||
		strings.Contains(runner.request.SystemPrompt, "Write") ||
		strings.Contains(runner.request.SystemPrompt, "WebSearch") ||
		runner.request.MaxOutputBytes != 64<<10 {
		t.Fatalf("process request = %#v", runner.request)
	}
	accounting, ok := result.Accounting()
	if !ok || accounting.InputTokens != 120 || accounting.OutputTokens != 31 ||
		accounting.TotalTokens != 151 || accounting.CacheReadTokens != 20 ||
		!accounting.CostObserved || accounting.CostMicrounits != 275000 ||
		accounting.CostCurrency != "USD" {
		t.Fatalf("accounting = %#v, %t", accounting, ok)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 || frames[0].Type() != bridgev1.MessageAck ||
		frames[1].Type() != bridgev1.MessageEvent ||
		string(frames[1].Payload()) != `{"delta":"Implemented the bounded change"}` ||
		frames[2].Type() != bridgev1.MessageResult ||
		string(frames[2].Payload()) != `{"status":"succeeded","reason":""}` {
		t.Fatalf("frames = %#v", frames)
	}
	if len(diagnostics.records) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
	diagnostic := diagnostics.records[0]
	if diagnostic.IncidentID != request.Dispatch.CorrelationID() ||
		diagnostic.ProviderID != ClaudeCodeProviderID ||
		diagnostic.ProviderAccountID != "anthropic.primary" ||
		diagnostic.ModelID != ClaudeCodeModelID ||
		diagnostic.Stage != "agent_attempt_dispatch" ||
		diagnostic.Result != "succeeded" || diagnostic.ErrorCode != "" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}

func TestClaudeCodeAdapterUsesVersionLockedPersistentContinuationForAgentInputs(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-anthropic-key")}
	inputs := &codexNoAgentInputSourceFixture{}
	runner := &codexAdapterContinuationRunnerFixture{result: HarnessProcessResult{
		Content: "continued in one Claude session",
	}}
	adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
		RuntimeInstanceID: "runtime.claude-code.local",
		ExecutablePath:    "/opt/loom/bin/claude",
		CredentialAccess:  access,
		Diagnostics:       &diagnosticRecorderFixture{},
		Runner:            runner,
		Now:               func() time.Time { return time.Now().UTC() },
		MaxOutputBytes:    64 << 10,
		ContinuationConformance: func(adapterType, path string) bool {
			return adapterType == ClaudeCodeAdapterType && path == "/opt/loom/bin/claude"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	consumer, ok := adapter.(loomruntime.AgentInputConsumer)
	if !ok || !consumer.AcceptsAgentInputs() {
		t.Fatal("version-locked Claude Adapter did not advertise Agent Input support")
	}
	request := claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
	request.AgentInputs = inputs
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if runner.oneShotRuns != 0 || runner.continuationRuns != 1 ||
		runner.inputs != inputs || access.uses != 1 {
		t.Fatalf("runner=%#v access=%d", runner, access.uses)
	}
}

func TestClaudeCodeAdapterRejectsAgentInputsWithoutContinuationConformance(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &codexAdapterContinuationRunnerFixture{}
	adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
		RuntimeInstanceID:       "runtime.claude-code.local",
		ExecutablePath:          "/opt/loom/bin/claude",
		CredentialAccess:        access,
		Diagnostics:             &diagnosticRecorderFixture{},
		Runner:                  runner,
		Now:                     func() time.Time { return time.Now().UTC() },
		MaxOutputBytes:          64 << 10,
		ContinuationConformance: func(string, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	consumer, ok := adapter.(loomruntime.AgentInputConsumer)
	if !ok || consumer.AcceptsAgentInputs() {
		t.Fatal("unverified Claude executable advertised Agent Input support")
	}
	request := claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
	request.AgentInputs = &codexNoAgentInputSourceFixture{}
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrHarnessExecutionBindingChanged) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.continuationRuns != 0 || runner.oneShotRuns != 0 {
		t.Fatalf("drift reached credential or process: access=%d runner=%#v", access.uses, runner)
	}
}

func TestClaudeCodeAdapterInjectsAttemptScopedContextMCPAfterAuthorityValidation(t *testing.T) {
	request := claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
	request = harnessContextAdapterRequest(t, request, "context:claude-code:v1")
	access := &credentialAccessFixture{secret: []byte("private-anthropic-key")}
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "done", Accounting: &work.RunAccounting{
			UsageObserved: true, InputTokens: 1, OutputTokens: 1, TotalTokens: 2,
		},
	}}
	adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/claude", CredentialAccess: access,
		Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(adapterType, path string) bool {
			return adapterType == ClaudeCodeAdapterType && path == "/opt/loom/bin/claude"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || runner.runs != 1 ||
		!validHarnessContextMCPLease(runner.request.ContextMCP) ||
		!strings.Contains(runner.request.SystemPrompt, "MCP") {
		t.Fatalf("access=%d runner=%#v", access.uses, runner)
	}
}

func TestClaudeCodeAdapterRejectsContextExecutableDriftBeforeCredentialAccess(t *testing.T) {
	request := harnessContextAdapterRequest(
		t, claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID),
		"context:claude-code:v1",
	)
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/claude", CredentialAccess: access,
		Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(string, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrHarnessExecutionBindingChanged) ||
		!errors.Is(err, ErrHarnessContextMCP) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("drift reached credential=%d process=%d", access.uses, runner.runs)
	}
}

func TestClaudeCodeAdapterRejectsCrossProviderOrModelBindingBeforeCredentialAccess(
	t *testing.T,
) {
	tests := []struct {
		name       string
		providerID string
		modelID    string
	}{
		{name: "cross provider", providerID: "openai", modelID: ClaudeCodeModelID},
		{name: "model drift", providerID: ClaudeCodeProviderID, modelID: "claude-opus-5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
			runner := &processRunnerFixture{}
			diagnostics := &diagnosticRecorderFixture{}
			adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
				RuntimeInstanceID: "runtime.claude-code.local",
				ExecutablePath:    "/opt/loom/bin/claude",
				CredentialAccess:  access,
				Diagnostics:       diagnostics,
				Runner:            runner,
				Now:               func() time.Time { return time.Now().UTC() },
				MaxOutputBytes:    64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := claudeCodeAdapterRequest(t, test.providerID, test.modelID)
			if _, err := adapter.Execute(context.Background(), request); !errors.Is(
				err,
				ErrHarnessExecutionBindingChanged,
			) {
				t.Fatalf("Execute() error = %v", err)
			}
			if access.uses != 0 || runner.runs != 0 {
				t.Fatalf("rejected binding used credential=%d runner=%d", access.uses, runner.runs)
			}
			if len(diagnostics.records) != 1 ||
				diagnostics.records[0].Stage != "agent_attempt_dispatch" ||
				diagnostics.records[0].ErrorCode != "binding_changed" {
				t.Fatalf("diagnostics = %#v", diagnostics.records)
			}
		})
	}
}

func TestClaudeCodeAdapterKeepsCredentialAndHarnessFailuresAgentLocal(t *testing.T) {
	tests := []struct {
		name       string
		accessErr  error
		runnerErr  error
		wantReason string
		wantStage  string
		retryable  bool
	}{
		{
			name: "credential revision unavailable", accessErr: nativeadapter.ErrAgentCredentialUnavailable,
			wantReason: "credential_unavailable", wantStage: "credential_lease_issue", retryable: true,
		},
		{
			name: "Vault decrypt failed",
			accessErr: credentials.WithCredentialFailureStage(
				credentials.CredentialStageVaultDecrypt,
				nativeadapter.ErrAgentCredentialUnavailable,
			),
			wantReason: "credential_unavailable",
			wantStage:  credentials.CredentialStageVaultDecrypt,
			retryable:  false,
		},
		{
			name: "provider rejected credential", runnerErr: ErrHarnessProviderAuth,
			wantReason: "provider_auth", wantStage: "provider_auth",
		},
		{
			name: "provider rate limited account", runnerErr: ErrHarnessProviderRateLimit,
			wantReason: "provider_rate_limit", wantStage: "provider_rate_limit", retryable: true,
		},
		{
			name: "Provider transport timed out", runnerErr: ErrHarnessProviderTimeout,
			wantReason: "timeout", wantStage: "provider_connect", retryable: true,
		},
		{
			name: "Provider response timed out", runnerErr: ErrHarnessProviderResponseTimeout,
			wantReason: "timeout", wantStage: "provider_http", retryable: true,
		},
		{
			name: "harness unavailable", runnerErr: ErrHarnessProcessUnavailable,
			wantReason: "harness_unavailable", wantStage: "agent_attempt_dispatch", retryable: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			access := &credentialAccessFixture{
				secret: []byte("private-anthropic-key"), err: test.accessErr,
			}
			runner := &processRunnerFixture{err: test.runnerErr}
			diagnostics := &diagnosticRecorderFixture{}
			adapter, err := NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
				RuntimeInstanceID: "runtime.claude-code.local",
				ExecutablePath:    "/opt/loom/bin/claude",
				CredentialAccess:  access,
				Diagnostics:       diagnostics,
				Runner:            runner,
				Now:               func() time.Time { return time.Now().UTC() },
				MaxOutputBytes:    64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
			result, err := adapter.Execute(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			frames := request.FrameSink.(*frameSinkFixture).frames
			if len(frames) != 2 || frames[1].Type() != bridgev1.MessageResult ||
				string(frames[1].Payload()) != `{"status":"failed","reason":"`+test.wantReason+`"}` {
				t.Fatalf("frames = %#v", frames)
			}
			if !result.DispatchAcknowledged() || !result.ResultAcknowledged() {
				t.Fatalf("result = %#v", result)
			}
			if len(diagnostics.records) != 1 ||
				diagnostics.records[0].Stage != test.wantStage ||
				diagnostics.records[0].ErrorCode != test.wantReason ||
				diagnostics.records[0].Retryable != test.retryable {
				t.Fatalf("diagnostics = %#v", diagnostics.records)
			}
		})
	}
}

func claudeCodeAdapterRequest(
	t testing.TB,
	providerID string,
	modelID string,
) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                   "profile.claude-code.anthropic.primary.r3",
		AdapterType:          ClaudeCodeAdapterType,
		ProviderID:           providerID,
		ProviderAccountID:    "anthropic.primary",
		ModelID:              modelID,
		AuthMode:             loomruntime.AuthBrokered,
		EndpointFingerprint:  ClaudeCodeEndpointFingerprint,
		CredentialReference:  "credential-ref-anthropic-primary",
		CredentialRevision:   3,
		RequiredCapabilities: []string{"workspace_edit"},
		Timeout:              2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime.claude-code.local", DeviceID: "device.local",
		AdapterType: ClaudeCodeAdapterType, DisplayName: "Claude Code",
		ExecutableVersion: "2.1.196", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"workspace_edit"}, Capacity: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "55555555-5555-4555-8555-555555555555",
		CorrelationID:         "66666666-6666-4666-8666-666666666666",
		WorkItemID:            "work-claude-1",
		RunID:                 "run-claude-1",
		ClaimGeneration:       1,
		RuntimeInstanceID:     "runtime.claude-code.local",
		SenderAgentInstanceID: "agent-claude-1",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             time.Date(2026, 8, 10, 13, 59, 59, 0, time.UTC),
		Payload:               []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Implement the bounded change"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	return supervisor.AdapterRequest{
		WorkspacePath: root,
		HomePath:      root,
		TempPath:      root,
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-claude-1", RunID: "run-claude-1",
			ClaimGeneration: 1, RuntimeInstanceID: "runtime.claude-code.local",
			SenderAgentInstanceID: "agent-claude-1",
		},
		ExecutionBinding: binding,
		Dispatch:         dispatch,
		FrameSink:        &frameSinkFixture{},
	}
}
