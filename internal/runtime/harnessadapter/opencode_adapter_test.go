package harnessadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/prompting"
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
	resolvedExecutable, err := ResolveHarnessExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}
	executable = resolvedExecutable
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
	commands := &liveOpenCodeCommandRecorder{delegate: NewSystemHarnessCommandRunner()}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
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
		t.Fatalf(
			"%v (exit=%d stdout_bytes=%d stderr_bytes=%d events=%v errors=%v local_logs=%v command_error=%t)",
			err, commands.result.ExitCode, len(commands.result.Stdout),
			len(commands.result.Stderr), safeOpenCodeEventTypes(commands.result.Stdout),
			safeOpenCodeErrorMetadata(commands.result.Stdout),
			commands.safeLogs,
			commands.err != nil,
		)
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

func TestLiveOpenCodeControlArbitrationComponent(t *testing.T) {
	if os.Getenv("LOOM_LIVE_OPENCODE_CONTROL") != "1" {
		t.Skip("live OpenCode control arbitration requires LOOM_LIVE_OPENCODE_CONTROL=1")
	}
	executable := os.Getenv("LOOM_LIVE_OPENCODE_EXECUTABLE")
	if executable == "" {
		executable = "/Users/lune/Documents/Codex/devtools/npm/bin/opencode"
	}
	resolvedExecutable, err := ResolveHarnessExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	gateway := &controlMCPGatewayStub{}
	service, err := newHarnessControlMCP(context.Background(), registry, gateway)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if err := service.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-opencode-live-control",
		SegmentID:      "segment-opencode-live-control",
		AttemptID:      "attempt-opencode-live-control",
		IncidentID:     "incident-opencode-live-control",
		CatalogDigest:  controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	lease := service.Lease()
	systemPrompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeConversation, ProviderID: "opencode",
		ModelID: "opencode/big-pickle", HarnessAdapter: "opencode",
		ControlTools: lease.ToolNames,
	})
	if err != nil {
		t.Fatal(err)
	}
	tempPath := t.TempDir()
	if err := os.Chmod(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	commands := &liveOpenCodeCommandRecorder{delegate: NewSystemHarnessCommandRunner()}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: resolvedExecutable, WorkspacePath: t.TempDir(),
		HomePath: home, TempPath: tempPath, ModelID: "opencode/big-pickle",
		Prompt: []byte("请检查 Loom 当前 Runtime 状态。"), SystemPrompt: systemPrompt,
		Timeout: 3 * time.Minute, MaxOutputBytes: 64 << 10, ControlMCP: lease,
	}, nil)
	if err != nil {
		t.Fatalf(
			"%v (exit=%d stdout_bytes=%d stderr_bytes=%d events=%v errors=%v local_logs=%v command_error=%t)",
			err, commands.result.ExitCode, len(commands.result.Stdout),
			len(commands.result.Stderr), safeOpenCodeEventTypes(commands.result.Stdout),
			safeOpenCodeErrorMetadata(commands.result.Stdout),
			commands.safeLogs,
			commands.err != nil,
		)
	}
	batch, err := service.EndTurn()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result.Content) == "" || gateway.calls != 1 ||
		gateway.lastCall.ToolID != controltool.ToolRuntimesStatus ||
		len(batch.SessionAlignments) != 0 || len(batch.ConversationActions) != 0 {
		t.Fatalf("result=%q calls=%d call=%#v batch=%#v", result.Content, gateway.calls, gateway.lastCall, batch)
	}
}

func TestLiveOpenCodeTerminalProposalArbitrationComponent(t *testing.T) {
	if os.Getenv("LOOM_LIVE_OPENCODE_PROPOSAL_CONTROL") != "1" {
		t.Skip("live OpenCode terminal Proposal requires LOOM_LIVE_OPENCODE_PROPOSAL_CONTROL=1")
	}
	executable := os.Getenv("LOOM_LIVE_OPENCODE_EXECUTABLE")
	if executable == "" {
		executable = "/Users/lune/Documents/Codex/devtools/npm/bin/opencode"
	}
	resolvedExecutable, err := ResolveHarnessExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	route, workspace := phase7ControlTurnReferences()
	now := time.Now().UTC()
	expectedObjective := "P7 source-probe opencode cancel governance check P7.PRIVATE.CONTENT." +
		strings.Repeat("a", 64)
	proposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID:           "proposal-opencode-live-terminal",
			ToolID:               controltool.ToolMissionsCreatePreview,
			TargetConversationID: "conversation-opencode-live-terminal",
			TargetContentDigest:  controltool.DigestBytes([]byte("target")),
			Argument:             expectedObjective,
			SegmentID:            "segment-opencode-live-terminal",
			AttemptID:            "attempt-opencode-live-terminal",
			IncidentID:           "incident-opencode-live-terminal",
			Route:                route,
			Workspace:            workspace,
			RegistryDigest:       registry.Digest(),
			CreatedAt:            now,
			ExpiresAt:            now.Add(5 * time.Minute),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &strictLiveOpenCodeProposalGateway{
		proposal: proposal, expectedObjective: expectedObjective,
	}
	server, err := OpenHarnessControlServer(context.Background(), registry, gateway)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := server.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-opencode-live-terminal",
		SegmentID:      "segment-opencode-live-terminal",
		AttemptID:      "attempt-opencode-live-terminal",
		IncidentID:     "incident-opencode-live-terminal",
		CatalogDigest:  controltool.DigestBytes([]byte("catalog")),
		Route:          *route,
		Workspace:      *workspace,
	}); err != nil {
		t.Fatal(err)
	}
	terminal, err := server.TerminalProposalCompleted()
	if err != nil {
		t.Fatal(err)
	}
	systemPrompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeConversation, ProviderID: "opencode",
		ModelID: "opencode/big-pickle", HarnessAdapter: "opencode",
		ControlTools: server.Lease().ToolNames,
	})
	if err != nil {
		t.Fatal(err)
	}
	tempPath := t.TempDir()
	if err := os.Chmod(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	commands := &liveOpenCodeCommandRecorder{delegate: NewSystemHarnessCommandRunner()}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	type runOutcome struct {
		result HarnessProcessResult
		err    error
	}
	runContext, cancelRun := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelRun()
	done := make(chan runOutcome, 1)
	go func() {
		result, runErr := runner.RunHarness(runContext, HarnessProcessRequest{
			ExecutablePath: resolvedExecutable, WorkspacePath: t.TempDir(),
			HomePath: home, TempPath: tempPath, ModelID: "opencode/big-pickle",
			Prompt:       []byte("Use the available Loom capability to prepare a reviewable Mission draft whose objective is exactly \"" + expectedObjective + "\". Put that text in the declared objective field; do not add a separate title field. Return the Loom review Proposal instead of a prose-only draft. Do not execute it."),
			SystemPrompt: systemPrompt, Timeout: 30 * time.Second,
			MaxOutputBytes: 64 << 10, ControlMCP: server.Lease(),
		}, nil)
		done <- runOutcome{result: result, err: runErr}
	}()
	terminalObserved := false
	var outcome runOutcome
	select {
	case <-terminal:
		terminalObserved = true
		cancelRun()
		outcome = <-done
	case outcome = <-done:
		select {
		case <-terminal:
			terminalObserved = true
		default:
		}
	case <-runContext.Done():
		outcome = <-done
	}
	batch, endErr := server.EndTurn()
	if endErr != nil {
		t.Fatal(endErr)
	}
	if !terminalObserved || gateway.validCalls != 1 || gateway.invalidCalls != 0 ||
		gateway.lastTool != controltool.ToolMissionsCreatePreview ||
		len(batch.ConversationActions) != 1 || len(batch.CompletedCalls) != 1 {
		t.Fatalf(
			"terminal=%t valid_calls=%d invalid_calls=%d tool=%s proposals=%d completed=%d run_error=%t exit=%d stdout_bytes=%d stderr_bytes=%d events=%v errors=%v local_logs=%v",
			terminalObserved, gateway.validCalls, gateway.invalidCalls, gateway.lastTool,
			len(batch.ConversationActions), len(batch.CompletedCalls), outcome.err != nil,
			commands.result.ExitCode, len(commands.result.Stdout), len(commands.result.Stderr),
			safeOpenCodeEventTypes(commands.result.Stdout),
			safeOpenCodeErrorMetadata(commands.result.Stdout), commands.safeLogs,
		)
	}
}

type strictLiveOpenCodeProposalGateway struct {
	proposal          controltool.ConversationActionProposal
	expectedObjective string
	validCalls        int
	invalidCalls      int
	lastTool          controltool.ToolID
}

func (gateway *strictLiveOpenCodeProposalGateway) Call(
	_ context.Context,
	_ controltool.TurnContext,
	call controltool.Call,
) (controltool.Result, error) {
	gateway.lastTool = call.ToolID
	var input struct {
		Objective string `json:"objective"`
	}
	decoder := json.NewDecoder(bytes.NewReader(call.Arguments))
	decoder.DisallowUnknownFields()
	if call.ToolID != controltool.ToolMissionsCreatePreview ||
		decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		input.Objective != gateway.expectedObjective {
		gateway.invalidCalls++
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	gateway.validCalls++
	return controltool.Result{
		Content:        json.RawMessage(`{"requires_confirmation":true}`),
		ActionProposal: &gateway.proposal,
	}, nil
}

type liveOpenCodeCommandRecorder struct {
	delegate HarnessCommandRunner
	result   HarnessCommandResult
	err      error
	safeLogs []string
}

func (recorder *liveOpenCodeCommandRecorder) RunCommand(
	ctx context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	if os.Getenv("LOOM_LIVE_OPENCODE_SAFE_LOGS") == "1" {
		request.Environment = append(request.Environment, "OPENCODE_PRINT_LOGS=1")
	}
	result, err := recorder.delegate.RunCommand(ctx, request)
	recorder.result = result
	recorder.err = err
	recorder.safeLogs = safeOpenCodeLocalLogMetadata(result.Stdout, result.Stderr)
	return result, err
}

var openCodeSafeStackLocation = regexp.MustCompile(
	`(?:file://)?/[A-Za-z0-9_@+./-]+\.(?:ts|tsx|js|mjs):[0-9]+(?::[0-9]+)?`,
)
var openCodeSafeSchemaPath = regexp.MustCompile(`\["([A-Za-z0-9_.-]+)"\]`)

func safeOpenCodeLocalLogMetadata(stdout, stderr []byte) []string {
	refs := make(map[string]struct{})
	for _, line := range bytes.Split(stdout, []byte("\n")) {
		var event struct {
			Error struct {
				Data struct {
					Ref string `json:"ref"`
				} `json:"data"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &event) == nil && validOpenCodeSafeRef(event.Error.Data.Ref) {
			refs[event.Error.Data.Ref] = struct{}{}
		}
	}
	metadata := make([]string, 0, len(refs))
	for _, line := range bytes.Split(stderr, []byte("\n")) {
		matchedRef := ""
		for ref := range refs {
			if bytes.Contains(line, []byte(ref)) {
				matchedRef = ref
				break
			}
		}
		if matchedRef == "" {
			continue
		}
		cause := openCodeSafeLogStringField(line, "cause")
		first := cause
		if index := strings.IndexByte(first, '\n'); index >= 0 {
			first = first[:index]
		}
		firstDigest := sha256.Sum256([]byte(first))
		causeDigest := sha256.Sum256([]byte(cause))
		classes := make([]string, 0, 4)
		for _, class := range []string{
			"TypeError", "ReferenceError", "SyntaxError", "RangeError",
			"SchemaError", "PermissionError", "NotFoundError", "UnknownError",
		} {
			if strings.Contains(cause, class) {
				classes = append(classes, class)
			}
		}
		locations := openCodeSafeStackLocation.FindAllString(cause, 6)
		pathMatches := openCodeSafeSchemaPath.FindAllStringSubmatch(cause, 20)
		schemaPath := make([]string, 0, len(pathMatches))
		seenPath := make(map[string]struct{}, len(pathMatches))
		for _, match := range pathMatches {
			if len(match) != 2 {
				continue
			}
			if _, duplicate := seenPath[match[1]]; duplicate {
				continue
			}
			seenPath[match[1]] = struct{}{}
			schemaPath = append(schemaPath, match[1])
		}
		terms := make([]string, 0, 8)
		lowerCause := strings.ToLower(cause)
		for _, term := range []string{
			"agent", "config", "format", "json_schema", "message", "mcp",
			"permission", "plugin", "retrycount", "schema", "structuredoutput",
			"tool_arguments", "tool_name",
		} {
			if strings.Contains(lowerCause, term) {
				terms = append(terms, term)
			}
		}
		metadata = append(metadata, fmt.Sprintf(
			"ref=%s cause_bytes=%d cause_digest=%x first_bytes=%d first_digest=%x classes=%v schema_path=%v terms=%v locations=%v",
			matchedRef, len(cause), causeDigest[:8], len(first), firstDigest[:8],
			classes, schemaPath, terms, locations,
		))
	}
	return metadata
}

func validOpenCodeSafeRef(ref string) bool {
	if len(ref) != len("err_00000000") || !strings.HasPrefix(ref, "err_") {
		return false
	}
	for _, character := range ref[len("err_"):] {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func openCodeSafeLogStringField(line []byte, key string) string {
	prefix := []byte(" " + key + "=")
	index := bytes.Index(line, prefix)
	if index < 0 {
		return ""
	}
	value := bytes.TrimSpace(line[index+len(prefix):])
	if len(value) == 0 {
		return ""
	}
	if value[0] != '"' {
		if end := bytes.IndexByte(value, ' '); end >= 0 {
			value = value[:end]
		}
		return string(value)
	}
	var decoded string
	if json.NewDecoder(bytes.NewReader(value)).Decode(&decoded) != nil {
		return ""
	}
	return decoded
}

func safeOpenCodeEventTypes(payload []byte) []string {
	types := make([]string, 0, 8)
	for _, line := range bytes.Split(payload, []byte("\n")) {
		var event struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &event) == nil && event.Type != "" {
			types = append(types, event.Type)
		}
	}
	return types
}

func safeOpenCodeErrorMetadata(payload []byte) []string {
	metadata := make([]string, 0, 2)
	for _, line := range bytes.Split(payload, []byte("\n")) {
		var event struct {
			Type  string `json:"type"`
			Error struct {
				Name string `json:"name"`
				Data struct {
					StatusCode  int    `json:"statusCode"`
					IsRetryable *bool  `json:"isRetryable"`
					Message     string `json:"message"`
					Ref         string `json:"ref"`
				} `json:"data"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &event) != nil ||
			(event.Type != "error" && event.Type != "session.error") {
			continue
		}
		retryable := "unset"
		if event.Error.Data.IsRetryable != nil {
			retryable = fmt.Sprintf("%t", *event.Error.Data.IsRetryable)
		}
		lowerMessage := strings.ToLower(event.Error.Data.Message)
		messageDigest := sha256.Sum256([]byte(event.Error.Data.Message))
		categories := make([]string, 0, 6)
		classifiers := []struct {
			label   string
			needles []string
		}{
			{label: "agent", needles: []string{"agent"}},
			{label: "auth", needles: []string{"auth", "credential", "login"}},
			{label: "config", needles: []string{"config"}},
			{label: "database", needles: []string{"database", "sqlite"}},
			{label: "file", needles: []string{"file", "directory", "path"}},
			{label: "format", needles: []string{"format", "json"}},
			{label: "module", needles: []string{"module", "import"}},
			{label: "model", needles: []string{"model"}},
			{label: "network", needles: []string{"network", "connect", "timeout", "dns", "tls"}},
			{label: "not_found", needles: []string{"not found", "enoent", "cannot find", "missing"}},
			{label: "permission", needles: []string{"permission", "eacces", "read-only", "readonly", "erofs", "writable", "access"}},
			{label: "plugin", needles: []string{"plugin"}},
			{label: "provider", needles: []string{"provider", "available"}},
			{label: "loom_guard", needles: []string{"loom control arbitration requires a message object"}},
			{label: "schema", needles: []string{"schema", "structured"}},
			{label: "session", needles: []string{"session"}},
		}
		for _, classifier := range classifiers {
			for _, needle := range classifier.needles {
				if strings.Contains(lowerMessage, needle) {
					categories = append(categories, classifier.label)
					break
				}
			}
		}
		metadata = append(metadata, fmt.Sprintf(
			"name=%s ref=%s status=%d retryable=%s message_bytes=%d message_digest=%x categories=%v",
			event.Error.Name, event.Error.Data.Ref, event.Error.Data.StatusCode, retryable,
			len(event.Error.Data.Message), messageDigest[:8], categories,
		))
	}
	return metadata
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
		Content: "DeepSeek completed the governed task",
	}}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: "runtime.opencode.local",
		ExecutablePath:    "/opt/loom/bin/opencode",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
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
	request := openCodeAdapterRequest(t, "deepseek", "deepseek-v4-flash", "high")
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

func TestOpenCodeAdapterRejectsIncompatibleMiniMaxBeforeCredential(t *testing.T) {
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
	request := openCodeAdapterRequest(t, "minimax", "MiniMax-M3", "")
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err, ErrHarnessExecutionBindingChanged,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("incompatible MiniMax route reached credential or process: access=%d runner=%d",
			access.uses, runner.runs)
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
	foundGovernedAgent := false
	arguments := strings.Join(request.Arguments, "\n")
	if strings.Contains(arguments, "implement") ||
		strings.Contains(arguments, "role governance") ||
		!bytes.Equal(commands.stdinSnapshot, []byte("implement")) {
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
		if argument == "--agent" && index+1 < len(request.Arguments) &&
			request.Arguments[index+1] == openCodeGovernedAgentID {
			foundGovernedAgent = true
		}
	}
	for _, entry := range request.Environment {
		if entry == "DEEPSEEK_API_KEY=private-deepseek-key" {
			foundEnv = true
		}
		if strings.HasPrefix(entry, "OPENCODE_CONFIG_CONTENT=") &&
			strings.Contains(entry, `"permission":{"*":"deny"}`) &&
			strings.Contains(entry, `"share":"disabled"`) &&
			strings.Contains(entry, `"prompt":"role governance"`) &&
			strings.Contains(entry, `"temperature":0`) {
			foundGovernance = true
		}
	}
	if !foundModel || !foundVariant || !foundEnv || !foundGovernance ||
		!foundGovernedAgent {
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

	base.Prompt = []byte("implement")
	base.ReasoningEffort = ""
	base.SystemPrompt = "invalid\x00system"
	if _, err := runner.RunHarness(context.Background(), base, nil); !errors.Is(
		err, ErrInvalidOpenCodeAdapter,
	) {
		t.Fatalf("invalid system prompt error = %v", err)
	}
	if len(commands.requests) != 1 || !allHarnessBytesZero(base.Prompt) {
		t.Fatalf("invalid system prompt started command: %#v", commands.requests)
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
			`{"type":"error","error":{"name":"APIError","data":{"statusCode":400,"isRetryable":false,"message":"Error from provider: Model is unavailable."}}}`,
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

func TestOpenCodeProcessRunnerPreservesStructuredProviderFailure(t *testing.T) {
	commands := &harnessCommandRunnerFixture{
		exitCode: 1,
		stdout: []byte(
			`{"type":"error","error":{"name":"APIError","data":{"statusCode":503,"isRetryable":true,"message":"provider-body-secret"}}}`,
		),
	}
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
	failure, ok := provider.ConversationFailureDetails(err)
	if !errors.Is(err, provider.ErrOpenCodeConversationUnavailable) || !ok ||
		failure.Stage != "provider_http" || failure.Code != "provider_unavailable" ||
		failure.HTTPStatus != 503 || !failure.Retryable ||
		strings.Contains(err.Error(), "provider-body-secret") ||
		strings.Contains(err.Error(), "private-minimax-key") {
		t.Fatalf("error = %v failure = %#v, %v", err, failure, ok)
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

func TestOpenCodeProcessRunnerUsesPrivateStructuredArbitrationForControlTurns(t *testing.T) {
	tempPath := t.TempDir()
	if err := os.Chmod(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	workspacePath := t.TempDir()
	homePath := t.TempDir()
	prompt := []byte("prepare a Mission")
	secret := []byte("private-deepseek-key")
	controlLease := HarnessControlMCPLease{
		URL: "http://127.0.0.1:43124/mcp", Token: strings.Repeat("b", 64),
		ToolNames: []string{
			"loom_missions_create_preview", "loom_missions_search",
		},
	}
	var pluginPath, pluginSource string
	commands := &harnessCommandRunnerFixture{
		stdout: []byte(strings.Join([]string{
			`{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"I can help with that.","tool_name":"none","tool_arguments":{}}}}}`,
			`{"type":"step_finish","part":{"type":"step-finish"}}`,
		}, "\n")),
		inspect: func(request HarnessCommandRequest) error {
			environment := make(map[string]string, len(request.Environment))
			for _, entry := range request.Environment {
				name, value, found := strings.Cut(entry, "=")
				if found {
					environment[name] = value
				}
			}
			if environment[harnessControlMCPTokenEnv] != controlLease.Token {
				return errors.New("OpenCode child is missing the scoped control MCP token")
			}
			if environment["OPENCODE_DISABLE_PROJECT_CONFIG"] != "1" ||
				environment["OPENCODE_DISABLE_EXTERNAL_SKILLS"] != "1" ||
				environment["OPENCODE_DISABLE_LSP_DOWNLOAD"] != "1" ||
				environment["OPENCODE_DISABLE_AUTOUPDATE"] != "1" ||
				environment["NPM_CONFIG_OFFLINE"] != "true" ||
				environment["XDG_CONFIG_HOME"] != filepath.Join(tempPath, ".loom-opencode-xdg", "config") ||
				environment["XDG_CACHE_HOME"] != filepath.Join(tempPath, ".loom-opencode-xdg", "cache") ||
				environment["XDG_STATE_HOME"] != filepath.Join(tempPath, ".loom-opencode-xdg", "state") ||
				environment["XDG_DATA_HOME"] != filepath.Join(tempPath, ".loom-opencode-xdg", "data") ||
				environment["HOME"] != filepath.Join(tempPath, ".loom-opencode-home") {
				return errors.New("OpenCode control environment is not isolated")
			}
			configDirectory := filepath.Join(environment["XDG_CONFIG_HOME"], "opencode")
			configInfo, err := os.Lstat(configDirectory)
			if err != nil || !configInfo.IsDir() ||
				configInfo.Mode()&os.ModeSymlink != 0 ||
				configInfo.Mode().Perm() != 0o500 {
				return errors.New("OpenCode control config directory is not read-only")
			}
			isolatedHome, err := os.Lstat(environment["HOME"])
			if err != nil || !isolatedHome.IsDir() ||
				isolatedHome.Mode()&os.ModeSymlink != 0 ||
				isolatedHome.Mode().Perm() != 0o700 {
				return errors.New("OpenCode control HOME is not private")
			}
			var config struct {
				Plugin []string                   `json:"plugin"`
				MCP    map[string]json.RawMessage `json:"mcp"`
			}
			if json.Unmarshal([]byte(environment["OPENCODE_CONFIG_CONTENT"]), &config) != nil ||
				len(config.Plugin) != 1 || len(config.MCP) != 1 ||
				!strings.Contains(environment["OPENCODE_CONFIG_CONTENT"], `"loom_control"`) ||
				!strings.Contains(environment["OPENCODE_CONFIG_CONTENT"], controlLease.URL) ||
				!strings.Contains(environment["OPENCODE_CONFIG_CONTENT"], `{env:LOOM_CONTROL_TURN_TOKEN}`) ||
				strings.Contains(environment["OPENCODE_CONFIG_CONTENT"], controlLease.Token) {
				return errors.New("OpenCode control plugin config is missing")
			}
			pluginURL, err := url.Parse(config.Plugin[0])
			if err != nil || pluginURL.Scheme != "file" || pluginURL.Host != "" ||
				!filepath.IsAbs(pluginURL.Path) {
				return errors.New("OpenCode control plugin URL is invalid")
			}
			pluginPath = pluginURL.Path
			info, err := os.Lstat(pluginPath)
			if err != nil || !info.Mode().IsRegular() ||
				info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
				return errors.New("OpenCode control plugin is not a private regular file")
			}
			source, err := os.ReadFile(pluginPath)
			if err != nil {
				return err
			}
			pluginSource = string(source)
			return nil
		},
	}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: workspacePath,
		HomePath: homePath, TempPath: tempPath,
		ModelID: "deepseek/deepseek-chat", Prompt: prompt,
		SystemPrompt: "Loom governed conversation policy", Timeout: time.Minute,
		MaxOutputBytes: 64 << 10, ControlMCP: controlLease,
		RequiresCredential: true,
	}, secret)
	if err != nil || result.Content != "I can help with that." {
		t.Fatalf("result = %#v, %v; requests=%d inspect=%v", result, err, len(commands.requests), commands.inspectErr)
	}
	arguments := strings.Join(commands.requests[0].Arguments, "\n")
	if slicesContain(commands.requests[0].Arguments, "--pure") ||
		!strings.Contains(pluginSource, `"chat.message"`) ||
		!strings.Contains(pluginSource, `type: "json_schema"`) ||
		!strings.Contains(pluginSource, `~effect/Schema/Class/OutputFormatJsonSchema`) ||
		!strings.Contains(pluginSource, `~effect/Schema/Class/OutputFormatText`) ||
		!strings.Contains(pluginSource, `"tool.execute.after"`) ||
		!strings.Contains(pluginSource, `loom/control/acknowledged`) ||
		!strings.Contains(pluginSource, `acknowledgeControlCompletion`) ||
		!strings.Contains(pluginSource, `AbortSignal.timeout(5000)`) ||
		!strings.Contains(pluginSource, `"experimental.chat.messages.transform"`) ||
		strings.Contains(pluginSource, `client.mcp.disconnect`) ||
		!strings.Contains(pluginSource, `controlCompleted = true`) ||
		!strings.Contains(pluginSource, `"loom_missions_create_preview"`) ||
		!strings.Contains(pluginSource, `"loom_missions_search"`) ||
		strings.Contains(pluginSource, string(prompt)) ||
		strings.Contains(pluginSource, string(secret)) ||
		strings.Contains(pluginSource, controlLease.Token) ||
		strings.Contains(arguments, string(prompt)) ||
		strings.Contains(arguments, controlLease.Token) {
		t.Fatalf("unsafe OpenCode control arbitration: args=%q source=%q", arguments, pluginSource)
	}
	if pluginPath == "" {
		t.Fatal("OpenCode control plugin path was not observed")
	}
	if _, err := os.Lstat(pluginPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenCode control plugin survived the turn: %v", err)
	}
	if !allHarnessBytesZero(prompt) {
		t.Fatal("OpenCode control prompt remained after process completion")
	}
}

func TestOpenCodeProcessRunnerPreservesInternalCommandTimeout(t *testing.T) {
	commands := &harnessCommandRunnerFixture{err: context.DeadlineExceeded}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	prompt := []byte("inspect Loom status")
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: t.TempDir(),
		HomePath: t.TempDir(), TempPath: t.TempDir(),
		ModelID: "opencode/big-pickle", Prompt: prompt,
		Timeout: time.Minute, MaxOutputBytes: 64 << 10,
	}, nil)
	if !errors.Is(err, ErrHarnessProviderTimeout) || len(commands.requests) != 1 ||
		!allHarnessBytesZero(prompt) {
		t.Fatalf("timeout error=%v requests=%d prompt_zero=%t", err, len(commands.requests), allHarnessBytesZero(prompt))
	}
}

func TestOpenCodeProcessRunnerProjectsOnlySelectedNativeAuthIntoIsolatedControlTurn(t *testing.T) {
	tempPath := t.TempDir()
	if err := os.Chmod(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	homePath := t.TempDir()
	authDirectory := filepath.Join(homePath, ".local", "share", "opencode")
	if err := os.MkdirAll(authDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	authPath := filepath.Join(authDirectory, "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"opencode":{"type":"api","key":"selected-native-auth"},"unrelated":{"type":"api","key":"must-not-cross"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	commands := &harnessCommandRunnerFixture{
		stdout: []byte(strings.Join([]string{
			`{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"No Loom action is needed.","tool_name":"none","tool_arguments":{}}}}}`,
			`{"type":"step_finish","part":{"type":"step-finish"}}`,
		}, "\n")),
		inspect: func(request HarnessCommandRequest) error {
			environment := make(map[string]string, len(request.Environment))
			for _, entry := range request.Environment {
				name, value, found := strings.Cut(entry, "=")
				if found {
					environment[name] = value
				}
			}
			if environment["XDG_DATA_HOME"] != filepath.Join(tempPath, ".loom-opencode-xdg", "data") ||
				strings.Contains(environment["OPENCODE_AUTH_CONTENT"], "must-not-cross") ||
				!strings.Contains(environment["OPENCODE_AUTH_CONTENT"], "selected-native-auth") {
				return errors.New("OpenCode native auth projection is not exact or isolated")
			}
			return nil
		},
	}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: t.TempDir(),
		HomePath: homePath, TempPath: tempPath,
		ModelID: "opencode/big-pickle", Prompt: []byte("ordinary conversation"),
		SystemPrompt: "Loom governed conversation policy", Timeout: time.Minute,
		MaxOutputBytes: 64 << 10,
		ControlMCP: HarnessControlMCPLease{
			URL: "http://127.0.0.1:43124/mcp", Token: strings.Repeat("b", 64),
			ToolNames: []string{"loom_runtimes_status"},
		},
	}, nil)
	if err != nil || commands.inspectErr != nil {
		t.Fatalf("native auth projection error=%v inspect=%v", err, commands.inspectErr)
	}
}

func TestOpenCodeProcessRunnerRejectsControlIsolationDirectoryReplacement(t *testing.T) {
	tempPath := t.TempDir()
	if err := os.Chmod(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	replacement := t.TempDir()
	sentinel := filepath.Join(replacement, "sentinel")
	if err := os.WriteFile(sentinel, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	commands := &harnessCommandRunnerFixture{
		stdout: []byte(strings.Join([]string{
			`{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"No control action is needed.","tool_name":"none","tool_arguments":{}}}}}`,
			`{"type":"step_finish","part":{"type":"step-finish"}}`,
		}, "\n")),
		inspect: func(request HarnessCommandRequest) error {
			for _, entry := range request.Environment {
				home, found := strings.CutPrefix(entry, "HOME=")
				if !found {
					continue
				}
				if err := os.Remove(home); err != nil {
					return err
				}
				return os.Symlink(replacement, home)
			}
			return errors.New("OpenCode control HOME was not provided")
		},
	}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: t.TempDir(),
		HomePath: t.TempDir(), TempPath: tempPath,
		ModelID: "opencode/big-pickle", Prompt: []byte("check status"),
		SystemPrompt: "Loom governed conversation policy", Timeout: time.Minute,
		MaxOutputBytes: 64 << 10,
		ControlMCP: HarnessControlMCPLease{
			URL: "http://127.0.0.1:43124/mcp", Token: strings.Repeat("b", 64),
			ToolNames: []string{"loom_runtimes_status"},
		},
	}, nil)
	if !errors.Is(err, ErrHarnessProcessUnavailable) {
		t.Fatalf("directory replacement error = %v", err)
	}
	if content, readErr := os.ReadFile(sentinel); readErr != nil || string(content) != "outside" {
		t.Fatalf("replacement target changed: %q, %v", content, readErr)
	}
}

func TestOpenCodeProcessRunnerInvokesOnlyTheSelectedFrozenControlTool(t *testing.T) {
	var calls int
	var selectedName string
	var selectedArguments json.RawMessage
	var completedTools []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/mcp" ||
			request.URL.RawQuery != "" ||
			request.Header.Get("Authorization") != "Bearer "+strings.Repeat("c", 64) ||
			request.Header.Get("Content-Type") != "application/json" {
			http.Error(writer, "invalid_request", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "invalid_request", http.StatusBadRequest)
			return
		}
		defer zeroHarnessBytes(body)
		var envelope struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Method  string `json:"method"`
			Params  struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&envelope) != nil || envelope.JSONRPC != "2.0" {
			http.Error(writer, "invalid_request", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		if envelope.Method == harnessControlMCPCompletedMethod && envelope.ID == 2 {
			encoded, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": 2,
				"result": map[string]any{"tool_names": completedTools},
			})
			_, _ = writer.Write(encoded)
			return
		}
		if envelope.ID != 1 || envelope.Method != "tools/call" {
			http.Error(writer, "invalid_request", http.StatusBadRequest)
			return
		}
		calls++
		selectedName = envelope.Params.Name
		selectedArguments = append(json.RawMessage(nil), envelope.Params.Arguments...)
		completedTools = append(completedTools, selectedName)
		_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"proposal\":true}"}],"structuredContent":{"proposal":true},"isError":false}}`))
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Path = "/mcp"
	lease := HarnessControlMCPLease{
		URL: endpoint.String(), Token: strings.Repeat("c", 64),
		ToolNames: []string{"loom_missions_create_preview"},
	}
	contextLease := HarnessContextMCPLease{
		URL: "http://127.0.0.1:43125/mcp", Token: strings.Repeat("d", 64),
		ContextEnabled: true,
	}
	run := func(t *testing.T, events []string, toolNames []string) error {
		t.Helper()
		tempPath := t.TempDir()
		if err := os.Chmod(tempPath, 0o700); err != nil {
			t.Fatal(err)
		}
		commands := &harnessCommandRunnerFixture{stdout: []byte(strings.Join(events, "\n"))}
		runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
		if err != nil {
			t.Fatal(err)
		}
		requestLease := lease
		requestLease.ToolNames = append([]string(nil), toolNames...)
		_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
			ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: t.TempDir(),
			HomePath: t.TempDir(), TempPath: tempPath,
			ModelID: "minimax-cn/MiniMax-M3", Prompt: []byte("prepare a Mission"),
			SystemPrompt: "Loom governed conversation policy", Timeout: time.Minute,
			MaxOutputBytes: 64 << 10, ContextMCP: contextLease, ControlMCP: requestLease,
		}, nil)
		return err
	}
	structured := func(name, arguments string) string {
		return `{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"The proposal is ready for review.","tool_name":"` + name + `","tool_arguments":` + arguments + `}}}}`
	}
	complete := `{"type":"step_finish","part":{"type":"step-finish"}}`
	if err := run(t, []string{
		`{"type":"tool_use","part":{"type":"tool","tool":"loom_context_loom_read_context","state":{"status":"completed","input":{"item_id":"context-1"}}}}`,
		structured("loom_missions_create_preview", `{"title":"Alpha"}`), complete,
	}, lease.ToolNames); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || selectedName != "loom_missions_create_preview" ||
		string(selectedArguments) != `{"title":"Alpha"}` {
		t.Fatalf("calls=%d name=%q arguments=%s", calls, selectedName, selectedArguments)
	}

	t.Run("completed direct tool is authoritative and never replayed", func(t *testing.T) {
		before := calls
		completedTools = []string{"loom_missions_create_preview"}
		err := run(t, []string{
			`{"type":"tool_use","part":{"type":"tool","tool":"loom_control_loom_missions_create_preview","state":{"status":"completed","input":{"title":"Alpha"}}}}`,
			`{"type":"text","part":{"type":"text","text":"The Mission draft is ready for review."}}`,
			complete,
		}, lease.ToolNames)
		if err != nil || calls != before {
			t.Fatalf("error=%v calls=%d before=%d", err, calls, before)
		}
	})

	t.Run("repeated completed direct tools preserve ordered corroboration", func(t *testing.T) {
		before := calls
		completedTools = []string{
			"loom_missions_create_preview", "loom_missions_create_preview",
		}
		err := run(t, []string{
			`{"type":"tool_use","part":{"type":"tool","tool":"loom_control_loom_missions_create_preview","state":{"status":"completed","input":{"title":"Alpha"}}}}`,
			`{"type":"tool_use","part":{"type":"tool","tool":"loom_control_loom_missions_create_preview","state":{"status":"completed","input":{"title":"Beta"}}}}`,
			`{"type":"text","part":{"type":"text","text":"Both Mission drafts are ready for review."}}`,
			complete,
		}, lease.ToolNames)
		if err != nil || calls != before {
			t.Fatalf("error=%v calls=%d before=%d", err, calls, before)
		}
	})

	t.Run("uncorroborated direct tool event fails closed", func(t *testing.T) {
		before := calls
		completedTools = nil
		err := run(t, []string{
			`{"type":"tool_use","part":{"type":"tool","tool":"loom_control_loom_missions_create_preview","state":{"status":"completed","input":{"title":"Alpha"}}}}`,
			`{"type":"text","part":{"type":"text","text":"The Mission draft is ready for review."}}`,
			complete,
		}, lease.ToolNames)
		if !errors.Is(err, ErrHarnessControlMCP) || calls != before {
			t.Fatalf("error=%v calls=%d before=%d", err, calls, before)
		}
	})

	t.Run("unknown selected tool never reaches MCP", func(t *testing.T) {
		before := calls
		completedTools = nil
		err := run(t, []string{
			structured("loom_teams_create_preview", `{}`), complete,
		}, lease.ToolNames)
		if !errors.Is(err, ErrHarnessControlMCP) || calls != before {
			t.Fatalf("error=%v calls=%d want=%d", err, calls, before)
		}
	})

	t.Run("direct completed tool is never replayed", func(t *testing.T) {
		before := calls
		completedTools = []string{"loom_missions_create_preview"}
		err := run(t, []string{
			`{"type":"tool_use","part":{"type":"tool","tool":"loom_control_loom_missions_create_preview","state":{"status":"completed","input":{"title":"Alpha"}}}}`,
			structured("loom_missions_create_preview", `{"title":"Alpha"}`), complete,
		}, lease.ToolNames)
		if err != nil || calls != before {
			t.Fatalf("error=%v calls=%d want=%d", err, calls, before)
		}
	})

	t.Run("different direct and selected tools fail closed", func(t *testing.T) {
		before := calls
		completedTools = []string{"loom_missions_search"}
		err := run(t, []string{
			`{"type":"tool_use","part":{"type":"tool","tool":"loom_control_loom_missions_search","state":{"status":"completed","input":{"query":"Alpha"}}}}`,
			structured("loom_missions_create_preview", `{"title":"Alpha"}`), complete,
		}, []string{"loom_missions_create_preview", "loom_missions_search"})
		if !errors.Is(err, ErrHarnessControlMCP) || calls != before {
			t.Fatalf("error=%v calls=%d want=%d", err, calls, before)
		}
	})
}

func TestOpenCodeProcessRunnerInjectsExactRegistryControlTools(t *testing.T) {
	commands := &harnessCommandRunnerFixture{stdout: []byte(strings.Join([]string{
		`{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"ok","tool_name":"none","tool_arguments":{}}}}}`,
		`{"type":"step_finish","part":{"type":"step-finish"}}`,
	}, "\n"))}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	contextLease := HarnessContextMCPLease{
		URL: "http://127.0.0.1:43123/mcp", Token: strings.Repeat("a", 64),
		ContextEnabled: true,
	}
	controlLease := HarnessControlMCPLease{
		URL: "http://127.0.0.1:43124/mcp", Token: strings.Repeat("b", 64),
		ToolNames: []string{
			"loom_sessions_search", "loom_sessions_align_preview",
			"loom_missions_create_preview", "loom_missions_continue_preview",
			"loom_teams_create_preview", "loom_roundtables_open_preview",
		},
	}
	tempPath := t.TempDir()
	if err := os.Chmod(tempPath, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: tempPath,
		ModelID: "deepseek/deepseek-chat", Prompt: []byte("prepare a Mission"),
		SystemPrompt: "Loom governed conversation policy", Timeout: time.Minute,
		MaxOutputBytes: 64 << 10, ContextMCP: contextLease, ControlMCP: controlLease,
	}, []byte("private-deepseek-key"))
	if err != nil {
		t.Fatal(err)
	}
	var config, environment string
	for _, entry := range commands.requests[0].Environment {
		if strings.HasPrefix(entry, "OPENCODE_CONFIG_CONTENT=") {
			config = entry
		}
		environment += entry + "\n"
	}
	for _, expected := range []string{
		`"agent":{"loom-governed"`,
		`"plugin":["file://`,
		`"prompt":"Loom governed conversation policy"`,
		`"temperature":0`,
		`"loom_context"`, contextLease.URL, `{env:LOOM_CONTEXT_ATTEMPT_TOKEN}`,
		`"loom_control"`, controlLease.URL, `{env:LOOM_CONTROL_TURN_TOKEN}`,
		`"loom_control_loom_sessions_search":"allow"`,
		`"loom_control_loom_sessions_align_preview":"allow"`,
		`"loom_control_loom_missions_create_preview":"allow"`,
		`"loom_control_loom_missions_continue_preview":"allow"`,
		`"loom_control_loom_teams_create_preview":"allow"`,
		`"loom_control_loom_roundtables_open_preview":"allow"`,
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("OpenCode config missing %q: %s", expected, config)
		}
	}
	if strings.Contains(config, `"loom_control_*":"allow"`) ||
		strings.Contains(config, contextLease.Token) || strings.Contains(config, controlLease.Token) ||
		!strings.Contains(environment, harnessContextMCPTokenEnv+"="+contextLease.Token) ||
		!strings.Contains(environment, harnessControlMCPTokenEnv+"="+controlLease.Token) ||
		!strings.Contains(environment, harnessControlMCPURLEnv+"="+controlLease.URL) ||
		!slicesContain(commands.requests[0].Arguments, "--agent") ||
		!slicesContain(commands.requests[0].Arguments, openCodeGovernedAgentID) ||
		slicesContain(commands.requests[0].Arguments, "--pure") ||
		strings.Contains(strings.Join(commands.requests[0].Arguments, "\n"), "prepare a Mission") ||
		!bytes.Equal(commands.stdinSnapshot, []byte("prepare a Mission")) ||
		bytes.Contains(commands.stdinSnapshot, []byte("Loom governed conversation policy")) {
		t.Fatalf("unsafe OpenCode config=%s env=%s", config, environment)
	}
}

func TestOpenCodeProcessRunnerRejectsForgedControlToolList(t *testing.T) {
	commands := &harnessCommandRunnerFixture{}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	prompt := []byte("prepare a Mission")
	_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/opencode", WorkspacePath: "/workspace",
		HomePath: "/Users/test", TempPath: "/tmp/opencode",
		ModelID: "opencode/big-pickle", Prompt: prompt, Timeout: time.Minute,
		MaxOutputBytes: 64 << 10, ControlMCP: HarnessControlMCPLease{
			URL: "http://127.0.0.1:43124/mcp", Token: strings.Repeat("b", 64),
			ToolNames: []string{"loom_sessions_search", "loom_sessions_search"},
		},
	}, nil)
	if !errors.Is(err, ErrInvalidOpenCodeAdapter) || len(commands.requests) != 0 ||
		!allHarnessBytesZero(prompt) {
		t.Fatalf("forged control result error=%v commands=%d zero=%t", err, len(commands.requests), allHarnessBytesZero(prompt))
	}
}

type harnessCommandRunnerFixture struct {
	requests      []HarnessCommandRequest
	stdinSnapshot []byte
	stdout        []byte
	exitCode      int
	err           error
	inspect       func(HarnessCommandRequest) error
	inspectErr    error
}

func (fixture *harnessCommandRunnerFixture) RunCommand(
	_ context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	fixture.requests = append(fixture.requests, request)
	fixture.stdinSnapshot = append([]byte(nil), request.Stdin...)
	if fixture.inspect != nil {
		if err := fixture.inspect(request); err != nil {
			fixture.inspectErr = err
			return HarnessCommandResult{}, err
		}
	}
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
