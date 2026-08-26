package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/provider"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestLiveOpenCodeGovernedMCPComponent(t *testing.T) {
	if os.Getenv("LOOM_LIVE_OPENCODE_MCP") != "1" {
		t.Skip("live OpenCode MCP component gate requires LOOM_LIVE_OPENCODE_MCP=1")
	}
	executable := os.Getenv("LOOM_LIVE_OPENCODE_EXECUTABLE")
	if executable == "" {
		executable = "/Users/lune/Documents/Codex/devtools/npm/bin/opencode"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("LOOM-OPENCODE-MCP-COMPONENT-OK")
	gateway := &harnessToolGatewayFixture{
		allowed: []permissions.ToolKind{permissions.ToolMCPTool},
		contents: map[permissions.ToolKind][]byte{
			permissions.ToolMCPTool: marker,
		},
	}
	binding := loomruntime.ToolCallBinding{
		ConversationID: "conversation-opencode-live-mcp", WorkItemID: "work-opencode-live-mcp",
		RunID: "run-opencode-live-mcp", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime.opencode.local", AgentInstanceID: "agent-opencode-live-mcp",
		ExecutionBindingDigest: strings.Repeat("1", 64),
		CapsuleDigest:          strings.Repeat("2", 64), ClaimID: "claim-opencode-live-mcp",
		IncidentID: "11111111-1111-4111-8111-111111111111",
		JourneyID:  "11111111-1111-4111-8111-111111111111",
	}
	service, err := newHarnessAttemptMCPWithContext(
		context.Background(), harnessAttemptMCPConfig{
			ToolGateway: gateway, ToolBinding: binding,
			AllowedTools: []permissions.ToolKind{permissions.ToolMCPTool},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{
		Commands: NewSystemHarnessCommandRunner(),
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: executable, WorkspacePath: t.TempDir(),
		HomePath: home, TempPath: t.TempDir(), ModelID: "opencode/big-pickle",
		Prompt:       []byte("Call loom_mcp_call exactly once with server mcp.live.probe, tool lookup, and arguments {\"value\":\"phase2d\"}. Then return the exact tool result marker."),
		SystemPrompt: "Use only the Loom-provided governed MCP tool. Do not call any other tool.",
		Timeout:      3 * time.Minute, MaxOutputBytes: 64 << 10,
		ContextMCP: service.Lease(),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(result.Content), marker) || len(gateway.envelopes) != 1 ||
		gateway.envelopes[0].Call != (permissions.ProposedCall{
			Tool: permissions.ToolMCPTool, Path: "mcp.live.probe/lookup",
			Command: `{"value":"phase2d"}`,
		}) {
		t.Fatalf("content=%q envelopes=%#v", result.Content, gateway.envelopes)
	}
	if err := service.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
}

func openCodeAdapterRequest(
	t testing.TB,
	providerID, modelID, reasoningEffort string,
) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:          "profile.opencode." + providerID + ".primary.r1",
		AdapterType: OpenCodeAdapterType,
		ProviderID:  providerID, ProviderAccountID: providerID + ".primary",
		ModelID: modelID, AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CredentialReference:  "credential-ref-" + providerID + "-primary",
		CredentialRevision:   4,
		ReasoningEffort:      reasoningEffort,
		RequiredCapabilities: []string{"reasoning_effort", "workspace_edit"},
		Timeout:              2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime.opencode.local", DeviceID: "device.local",
		AdapterType: OpenCodeAdapterType, DisplayName: "OpenCode",
		ExecutableVersion: "1.18.3", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"reasoning_effort", "workspace_edit"}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     "77777777-7777-4777-8777-777777777777",
		CorrelationID: "88888888-8888-4888-8888-888888888888",
		WorkItemID:    "work-opencode-1", RunID: "run-opencode-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime.opencode.local", SenderAgentInstanceID: "agent-opencode-1",
		Sequence: 1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 16, 15, 59, 59, 0, time.UTC),
		Payload:   []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Implement with OpenCode"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	return supervisor.AdapterRequest{
		WorkspacePath: root, HomePath: root, TempPath: root,
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-opencode-1", RunID: "run-opencode-1", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime.opencode.local", SenderAgentInstanceID: "agent-opencode-1",
		},
		ExecutionBinding: binding, Dispatch: dispatch, FrameSink: &frameSinkFixture{},
	}
}

func TestOpenCodeAdapterAdaptsProviderModelAndInjectsCredentialEnv(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-deepseek-key")}
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "Implemented with OpenCode DeepSeek",
	}}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: "runtime.opencode.local",
		ExecutablePath:    "/opt/loom/bin/opencode",
		CredentialAccess:  access,
		Diagnostics:       diagnostics,
		Runner:            runner,
		Now:               func() time.Time { return time.Date(2026, 8, 16, 16, 0, 0, 0, time.UTC) },
		MaxOutputBytes:    64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := openCodeAdapterRequest(t, "deepseek", "deepseek-v4-flash", "high")
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || !reflect.DeepEqual(access.binding, request.ExecutionBinding) ||
		runner.runs != 1 || string(runner.secret) != "private-deepseek-key" {
		t.Fatalf("access=%#v runner=%#v", access, runner)
	}
	// The model identity must be adapted to OpenCode's provider/model form.
	if runner.request.ModelID != "deepseek/deepseek-v4-flash" {
		t.Fatalf("model identity = %q", runner.request.ModelID)
	}
	if runner.request.ReasoningEffort != "high" {
		t.Fatalf("reasoning effort = %q", runner.request.ReasoningEffort)
	}
	if !strings.Contains(runner.request.SystemPrompt, "provider=deepseek") ||
		!strings.Contains(runner.request.SystemPrompt, "model=deepseek-v4-flash") {
		t.Fatalf("system prompt = %q", runner.request.SystemPrompt)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 ||
		string(frames[1].Payload()) != `{"delta":"Implemented with OpenCode DeepSeek"}` ||
		string(frames[2].Payload()) != `{"status":"succeeded","reason":""}` {
		t.Fatalf("frames = %#v", frames)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].ProviderID != "deepseek" ||
		diagnostics.records[0].ProviderAccountID != "deepseek.primary" ||
		diagnostics.records[0].ModelID != "deepseek-v4-flash" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
	_ = result
}

type rejectingOpenCodeFrameSink struct {
	frames []bridgev1.Frame
}

func (sink *rejectingOpenCodeFrameSink) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	if frame.Type() == bridgev1.MessageEvent {
		return errors.New("authorized frame observer unavailable")
	}
	return nil
}

func TestOpenCodeAdapterRecordsBridgePublicationFailureAfterProviderSuccess(t *testing.T) {
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "MiniMax completed the governed task",
	}}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: "runtime.opencode.local",
		ExecutablePath:    "/opt/loom/bin/opencode",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-minimax-key")},
		Diagnostics:       diagnostics,
		Runner:            runner,
		Now: func() time.Time {
			return time.Date(2026, 8, 16, 16, 0, 0, 0, time.UTC)
		},
		MaxOutputBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := openCodeAdapterRequest(t, "minimax", "MiniMax-M3", "")
	sink := &rejectingOpenCodeFrameSink{}
	request.FrameSink = sink

	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err, supervisor.ErrBridgeSession,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(sink.frames) != 2 || sink.frames[0].Type() != bridgev1.MessageAck ||
		sink.frames[1].Type() != bridgev1.MessageEvent {
		t.Fatalf("published frames = %#v", sink.frames)
	}
	if len(diagnostics.records) != 2 ||
		diagnostics.records[0].Result != "succeeded" ||
		diagnostics.records[1].Stage != "agent_attempt_response" ||
		diagnostics.records[1].Result != "failed" ||
		diagnostics.records[1].ErrorCode != "bridge_protocol_failed" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestOpenCodeAdapterRejectsCrossProviderModelBeforeCredential(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: "runtime.opencode.local",
		ExecutablePath:    "/opt/loom/bin/opencode",
		CredentialAccess:  access,
		Diagnostics:       &diagnosticRecorderFixture{},
		Runner:            runner,
		Now:               func() time.Time { return time.Now().UTC() },
		MaxOutputBytes:    64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	// A deepseek binding must never silently run an openai model.
	request := openCodeAdapterRequest(t, "deepseek", "openai/gpt-5", "high")
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err, ErrHarnessExecutionBindingChanged,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("cross-provider model reached credential or process: access=%d runner=%d",
			access.uses, runner.runs)
	}
}

func TestOpenCodeAdapterRejectsUnsupportedFrozenVariantBeforeCredential(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: "runtime.opencode.local",
		ExecutablePath:    "/opt/loom/bin/opencode",
		CredentialAccess:  access,
		Diagnostics:       &diagnosticRecorderFixture{},
		Runner:            runner,
		Now:               func() time.Time { return time.Now().UTC() },
		MaxOutputBytes:    64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := openCodeAdapterRequest(t, "deepseek", "deepseek-v4-pro", "low")
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err, ErrHarnessExecutionBindingChanged,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("unsupported variant reached credential or process: access=%d runner=%d",
			access.uses, runner.runs)
	}
}

func TestOpenCodeProcessRunnerUsesModelIdentityAndCredentialEnv(t *testing.T) {
	commands := &harnessCommandRunnerFixture{stdout: []byte(strings.Join([]string{
		`{"type":"message.part.updated","properties":{"part":{"type":"text","text":"bounded "}}}`,
		`{"type":"message.part.updated","properties":{"part":{"type":"text","text":"reply"}}}`,
		`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
	}, "\n"))}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	requestInput := HarnessProcessRequest{
		ExecutablePath:  "/opt/loom/bin/opencode",
		WorkspacePath:   "/workspace",
		HomePath:        "/Users/test",
		TempPath:        "/tmp/opencode",
		ModelID:         "deepseek/deepseek-v4-flash",
		ReasoningEffort: "high",
		Prompt:          []byte("implement"),
		SystemPrompt:    "role governance",
		Timeout:         time.Minute,
		MaxOutputBytes:  64 << 10,
	}
	result, err := runner.RunHarness(
		context.Background(),
		requestInput,
		[]byte("private-deepseek-key"),
	)
	if err != nil || result.Content != "bounded reply" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	if len(commands.requests) != 1 {
		t.Fatalf("commands = %#v", commands.requests)
	}
	request := commands.requests[0]
	if request.ExecutablePath != "/opt/loom/bin/opencode" {
		t.Fatalf("executable = %q", request.ExecutablePath)
	}
	foundModel := false
	foundVariant := false
	foundEnv := false
	foundGovernance := false
	arguments := strings.Join(request.Arguments, "\n")
	if strings.Contains(arguments, "implement") ||
		strings.Contains(arguments, "role governance") ||
		!bytes.Equal(commands.stdinSnapshot, []byte("role governance\n\nimplement")) {
		t.Fatalf("OpenCode prompt escaped stdin: args=%#v stdin=%q", request.Arguments, commands.stdinSnapshot)
	}
	if !allHarnessBytesZero(requestInput.Prompt) {
		t.Fatal("OpenCode Harness prompt remained after process completion")
	}
	for index, argument := range request.Arguments {
		if argument == "--model" && index+1 < len(request.Arguments) &&
			request.Arguments[index+1] == "deepseek/deepseek-v4-flash" {
			foundModel = true
		}
		if argument == "--variant" && index+1 < len(request.Arguments) &&
			request.Arguments[index+1] == "high" {
			foundVariant = true
		}
	}
	for _, entry := range request.Environment {
		if entry == "DEEPSEEK_API_KEY=private-deepseek-key" {
			foundEnv = true
		}
		if strings.HasPrefix(entry, "OPENCODE_CONFIG_CONTENT=") &&
			strings.Contains(entry, `"permission":{"*":"deny"}`) &&
			strings.Contains(entry, `"share":"disabled"`) {
			foundGovernance = true
		}
	}
	if !foundModel || !foundVariant || !foundEnv || !foundGovernance {
		t.Fatalf("arguments=%#v environment=%#v", request.Arguments, request.Environment)
	}
}

func TestOpenCodeProcessRunnerOmitsEmptyVariantAndRejectsUnsupportedVariant(t *testing.T) {
	commands := &harnessCommandRunnerFixture{stdout: []byte(strings.Join([]string{
		`{"type":"message.part.updated","properties":{"part":{"type":"text","text":"ok"}}}`,
		`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
	}, "\n"))}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	base := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "deepseek/deepseek-v4-pro", Prompt: []byte("implement"),
		Timeout: time.Minute, MaxOutputBytes: 64 << 10,
	}
	if _, err := runner.RunHarness(context.Background(), base, nil); err != nil {
		t.Fatal(err)
	}
	if slicesContain(commands.requests[0].Arguments, "--variant") {
		t.Fatalf("empty effort emitted variant: %#v", commands.requests[0].Arguments)
	}

	base.Prompt = []byte("implement")
	base.ReasoningEffort = "low"
	if _, err := runner.RunHarness(context.Background(), base, nil); !errors.Is(
		err, ErrInvalidOpenCodeAdapter,
	) {
		t.Fatalf("unsupported variant error = %v", err)
	}
	if len(commands.requests) != 1 {
		t.Fatalf("unsupported variant started command: %#v", commands.requests)
	}

	base.Prompt = []byte("implement")
	base.ReasoningEffort = " high "
	if _, err := runner.RunHarness(context.Background(), base, nil); !errors.Is(
		err, ErrInvalidOpenCodeAdapter,
	) {
		t.Fatalf("non-canonical variant error = %v", err)
	}
	if len(commands.requests) != 1 {
		t.Fatalf("non-canonical variant started command: %#v", commands.requests)
	}
}

func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestOpenCodeProcessRunnerPreservesRetiredModelClassification(t *testing.T) {
	commands := &harnessCommandRunnerFixture{
		exitCode: 1,
		stdout: []byte(
			`{"type":"error","error":{"name":"APIError","data":{"statusCode":400,"message":"Error from provider: Model is unavailable."}}}`,
		),
	}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "opencode/retired-model", Prompt: []byte("implement"),
		Timeout: time.Minute, MaxOutputBytes: 64 << 10,
	}, nil)
	if !errors.Is(err, provider.ErrOpenCodeConversationModelUnavailable) ||
		strings.Contains(err.Error(), "Error from provider") {
		t.Fatalf("error = %v", err)
	}
}

func TestOpenCodeProcessRunnerAcceptsCompletedToolOnlySession(t *testing.T) {
	commands := &harnessCommandRunnerFixture{stdout: []byte(strings.Join([]string{
		`{"type":"message.part.updated","properties":{"part":{"type":"tool","tool":"Edit"}}}`,
		`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
	}, "\n"))}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "minimax-cn/MiniMax-M3", Prompt: []byte("implement"),
		Timeout: time.Minute, MaxOutputBytes: 64 << 10,
	}, []byte("private-minimax-key"))
	if err != nil || result.Content != "" {
		t.Fatalf("tool-only result=%#v error=%v", result, err)
	}
}

func TestOpenCodeProcessRunnerAcceptsStructuredCompletionAfterNonzeroCleanupExit(t *testing.T) {
	commands := &harnessCommandRunnerFixture{
		exitCode: 1,
		stdout: []byte(strings.Join([]string{
			`{"type":"message.part.updated","properties":{"part":{"type":"tool","tool":"Edit"}}}`,
			`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
		}, "\n")),
	}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "minimax-cn/MiniMax-M3", Prompt: []byte("implement"),
		Timeout: time.Minute, MaxOutputBytes: 64 << 10,
	}, []byte("private-minimax-key"))
	if err != nil || result.Content != "" {
		t.Fatalf("structured cleanup result=%#v error=%v", result, err)
	}
}

func TestOpenCodeProcessRunnerRejectsCompletedEmptyNoOpSession(t *testing.T) {
	commands := &harnessCommandRunnerFixture{stdout: []byte(
		`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
	)}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "minimax-cn/MiniMax-M3", Prompt: []byte("implement"),
		Timeout: time.Minute, MaxOutputBytes: 64 << 10,
	}, []byte("private-minimax-key"))
	if !errors.Is(err, provider.ErrOpenCodeConversationUnavailable) {
		t.Fatalf("empty no-op error = %v", err)
	}
}

func TestOpenCodeAdapterAcceptsEmptyFinalTextOnlyAfterGovernedToolActivity(t *testing.T) {
	result := HarnessProcessResult{}
	if _, err := normalizeOpenCodeHarnessProcessResult(result, 64<<10, nil); !errors.Is(
		err, ErrHarnessProtocol,
	) {
		t.Fatalf("empty result without tool activity error = %v", err)
	}
	service := &harnessContextMCP{
		prepared: []harnessPreparedDelivery{{context: false}},
	}
	if _, err := normalizeOpenCodeHarnessProcessResult(result, 64<<10, service); err != nil {
		t.Fatalf("governed tool-only result error = %v", err)
	}
}

func TestOpenCodeAdapterDropsUnusableFinalTextAfterGovernedToolActivity(t *testing.T) {
	service := &harnessContextMCP{
		prepared: []harnessPreparedDelivery{{context: false}},
	}
	for name, content := range map[string]string{
		"oversized": strings.Repeat("x", 257),
		"nul":       "completed\x00candidate",
	} {
		t.Run(name, func(t *testing.T) {
			normalized, err := normalizeOpenCodeHarnessProcessResult(
				HarnessProcessResult{Content: content}, 256, service,
			)
			if err != nil || normalized.Content != "" {
				t.Fatalf("normalized result = %#v, %v", normalized, err)
			}
		})
	}
}

func TestOpenCodeAdapterRecoversOnlyOpaqueFailureWithGovernedToolCandidate(t *testing.T) {
	service := &harnessContextMCP{
		prepared: []harnessPreparedDelivery{{context: false}},
	}
	if !recoverOpenCodeGovernedToolCandidate(ErrHarnessProcessUnavailable, service) {
		t.Fatal("opaque process failure should preserve a governed tool candidate")
	}
	if !recoverOpenCodeGovernedToolCandidate(
		provider.ErrOpenCodeConversationUnavailable, service,
	) {
		t.Fatal("opaque response failure should preserve a governed tool candidate")
	}
	for _, err := range []error{
		nil,
		provider.ErrOpenCodeConversationAuth,
		provider.ErrOpenCodeConversationRateLimit,
		context.DeadlineExceeded,
	} {
		if recoverOpenCodeGovernedToolCandidate(err, service) {
			t.Fatalf("classified failure %v recovered", err)
		}
	}
	if recoverOpenCodeGovernedToolCandidate(
		ErrHarnessProcessUnavailable, nil,
	) {
		t.Fatal("process failure without governed tool activity recovered")
	}
}

func TestOpenCodeProviderFailuresRemainActionable(t *testing.T) {
	tests := []struct {
		err       error
		reason    string
		stage     string
		retryable bool
	}{
		{provider.ErrOpenCodeConversationAuth, "provider_auth", "provider_auth", false},
		{provider.ErrOpenCodeConversationInsufficientBalance, "provider_insufficient_balance", "provider_http", false},
		{provider.ErrOpenCodeConversationModelUnavailable, "provider_model_unavailable", "provider_http", false},
		{provider.ErrOpenCodeConversationRateLimit, "provider_rate_limit", "provider_rate_limit", true},
	}
	for _, test := range tests {
		reason, stage, retryable := harnessFailure(test.err)
		if reason != test.reason || stage != test.stage || retryable != test.retryable {
			t.Fatalf("failure = %q, %q, %t", reason, stage, retryable)
		}
	}
}

func TestOpenCodeProcessRunnerInjectsScopedContextMCP(t *testing.T) {
	commands := &harnessCommandRunnerFixture{stdout: []byte(strings.Join([]string{
		`{"type":"message.part.updated","properties":{"part":{"type":"text","text":"ok"}}}`,
		`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
	}, "\n"))}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	lease := HarnessContextMCPLease{
		URL: "http://127.0.0.1:43123/mcp", Token: strings.Repeat("a", 64),
		ContextEnabled: true,
	}
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "deepseek/deepseek-chat", Prompt: []byte("implement"),
		SystemPrompt: "role governance", Timeout: time.Minute,
		MaxOutputBytes: 64 << 10, ContextMCP: lease,
	}, []byte("private-deepseek-key"))
	if err != nil {
		t.Fatal(err)
	}
	var configFound, tokenFound bool
	for _, entry := range commands.requests[0].Environment {
		switch {
		case strings.HasPrefix(entry, "OPENCODE_CONFIG_CONTENT="):
			configFound = strings.Contains(entry, `"loom_context"`) &&
				strings.Contains(entry, `"*":"deny"`) &&
				strings.Contains(entry, `"loom_context_*":"allow"`) &&
				strings.Contains(entry, `"share":"disabled"`) &&
				strings.Contains(entry, lease.URL) &&
				strings.Contains(entry, `{env:LOOM_CONTEXT_ATTEMPT_TOKEN}`) &&
				!strings.Contains(entry, lease.Token)
		case entry == "LOOM_CONTEXT_ATTEMPT_TOKEN="+lease.Token:
			tokenFound = true
		}
	}
	if !configFound || !tokenFound {
		t.Fatalf("OpenCode environment missing scoped MCP config: %#v", commands.requests[0].Environment)
	}
	if commands.requests[0].Directory != "/workspace" ||
		!reflect.DeepEqual(commands.requests[0].Arguments[len(commands.requests[0].Arguments)-2:], []string{"--dir", "/workspace"}) {
		t.Fatalf("OpenCode workspace binding = dir %q args %#v", commands.requests[0].Directory, commands.requests[0].Arguments)
	}
}

type harnessCommandRunnerFixture struct {
	requests      []HarnessCommandRequest
	stdinSnapshot []byte
	stdout        []byte
	exitCode      int
	err           error
}

func (fixture *harnessCommandRunnerFixture) RunCommand(
	_ context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	fixture.requests = append(fixture.requests, request)
	fixture.stdinSnapshot = append([]byte(nil), request.Stdin...)
	if fixture.err != nil {
		return HarnessCommandResult{}, fixture.err
	}
	return HarnessCommandResult{Stdout: fixture.stdout, ExitCode: fixture.exitCode}, nil
}

func openCodeNativeAdapterRequest(t testing.TB) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile.opencode.native.v1", AdapterType: OpenCodeAdapterType,
		ProviderID: "opencode", ProviderAccountID: "",
		ModelID: "opencode/big-pickle", AuthMode: loomruntime.AuthNative,
		RequiredCapabilities: []string{"workspace_edit"},
		Timeout:              2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime.opencode.local", DeviceID: "device.local",
		AdapterType: OpenCodeAdapterType, DisplayName: "OpenCode",
		ExecutableVersion: "1.18.3", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"workspace_edit"}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     "77777777-7777-4777-8777-777777777777",
		CorrelationID: "88888888-8888-4888-8888-888888888888",
		WorkItemID:    "work-opencode-native-1", RunID: "run-opencode-native-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime.opencode.local", SenderAgentInstanceID: "agent-opencode-native-1",
		Sequence: 1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 16, 15, 59, 59, 0, time.UTC),
		Payload:   []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Answer with OpenCode native"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	return supervisor.AdapterRequest{
		WorkspacePath: root, HomePath: root, TempPath: root,
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-opencode-native-1", RunID: "run-opencode-native-1", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime.opencode.local", SenderAgentInstanceID: "agent-opencode-native-1",
		},
		ExecutionBinding: binding, Dispatch: dispatch, FrameSink: &frameSinkFixture{},
	}
}

func TestOpenCodeAdapterRunsNativeAuthWithoutCredential(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-leased")}
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "Native OpenCode answer",
	}}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: "runtime.opencode.local",
		ExecutablePath:    "/opt/loom/bin/opencode",
		CredentialAccess:  access,
		Diagnostics:       diagnostics,
		Runner:            runner,
		Now:               func() time.Time { return time.Date(2026, 8, 16, 16, 0, 0, 0, time.UTC) },
		MaxOutputBytes:    64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := openCodeNativeAdapterRequest(t)
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if access.uses != 0 {
		t.Fatalf("native auth must not lease a brokered credential: uses=%d", access.uses)
	}
	if runner.runs != 1 {
		t.Fatalf("runner runs = %d", runner.runs)
	}
	if runner.request.ModelID != "opencode/big-pickle" {
		t.Fatalf("model identity = %q", runner.request.ModelID)
	}
	if runner.request.RequiresCredential {
		t.Fatal("native auth must not require a credential secret")
	}
}
