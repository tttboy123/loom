package harnessadapter

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func openCodeAdapterRequest(
	t testing.TB,
	providerID, modelID string,
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
		ReasoningEffort:      "high",
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
	request := openCodeAdapterRequest(t, "deepseek", "deepseek-chat")
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || !reflect.DeepEqual(access.binding, request.ExecutionBinding) ||
		runner.runs != 1 || string(runner.secret) != "private-deepseek-key" {
		t.Fatalf("access=%#v runner=%#v", access, runner)
	}
	// The model identity must be adapted to OpenCode's provider/model form.
	if runner.request.ModelID != "deepseek/deepseek-chat" {
		t.Fatalf("model identity = %q", runner.request.ModelID)
	}
	if !strings.Contains(runner.request.SystemPrompt, "provider=deepseek") ||
		!strings.Contains(runner.request.SystemPrompt, "model=deepseek-chat") {
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
		diagnostics.records[0].ModelID != "deepseek-chat" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
	_ = result
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
	request := openCodeAdapterRequest(t, "deepseek", "openai/gpt-5")
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
	result, err := runner.RunHarness(
		context.Background(),
		HarnessProcessRequest{
			ExecutablePath: "/opt/loom/bin/opencode",
			WorkspacePath:  "/workspace",
			HomePath:       "/Users/test",
			TempPath:       "/tmp/opencode",
			ModelID:        "deepseek/deepseek-chat",
			Prompt:         []byte("implement"),
			SystemPrompt:   "role governance",
			Timeout:        time.Minute,
			MaxOutputBytes: 64 << 10,
		},
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
	foundEnv := false
	for index, argument := range request.Arguments {
		if argument == "--model" && index+1 < len(request.Arguments) &&
			request.Arguments[index+1] == "deepseek/deepseek-chat" {
			foundModel = true
		}
	}
	for _, entry := range request.Environment {
		if entry == "DEEPSEEK_API_KEY=private-deepseek-key" {
			foundEnv = true
		}
	}
	if !foundModel || !foundEnv {
		t.Fatalf("arguments=%#v environment=%#v", request.Arguments, request.Environment)
	}
}

type harnessCommandRunnerFixture struct {
	requests []HarnessCommandRequest
	stdout   []byte
	err      error
}

func (fixture *harnessCommandRunnerFixture) RunCommand(
	_ context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	fixture.requests = append(fixture.requests, request)
	if fixture.err != nil {
		return HarnessCommandResult{}, fixture.err
	}
	return HarnessCommandResult{Stdout: fixture.stdout, ExitCode: 0}, nil
}

func openCodeNativeAdapterRequest(t testing.TB) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile.opencode.native.v1", AdapterType: OpenCodeAdapterType,
		ProviderID: "opencode", ProviderAccountID: "",
		ModelID: "opencode/deepseek-v4-flash-free", AuthMode: loomruntime.AuthNative,
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
	if runner.request.ModelID != "opencode/deepseek-v4-flash-free" {
		t.Fatalf("model identity = %q", runner.request.ModelID)
	}
	if runner.request.RequiresCredential {
		t.Fatal("native auth must not require a credential secret")
	}
}
