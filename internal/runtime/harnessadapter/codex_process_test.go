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
)

func TestCodexProcessUsesExactGatewayConfigAndClosedJSONL(t *testing.T) {
	tempPath := t.TempDir()
	gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
		BaseURL: "http://127.0.0.1:43124", Token: "one-time-codex-token",
	}}
	commands := &commandRunnerFixture{result: HarnessCommandResult{
		Stdout: []byte(
			`{"type":"thread.started","thread_id":"fixture"}` + "\n" +
				`{"type":"item.completed","item":{"id":"item-1","type":"agent_message","text":"Implemented the Codex change"}}` + "\n" +
				`{"type":"turn.completed","usage":{"input_tokens":140,"cached_input_tokens":25,"output_tokens":32,"reasoning_output_tokens":8}}` + "\n",
		),
		Stderr: []byte("raw Codex detail must be dropped"), ExitCode: 0,
	}}
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{
		Gateway: gateway, Commands: commands,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/codex",
		WorkspacePath:  "/private/tmp/loom/workspace",
		HomePath:       "/private/tmp/loom/home", TempPath: tempPath,
		ModelID: CodexModelID, ReasoningEffort: "high",
		Prompt:       []byte("Implement the bounded Codex change"),
		SystemPrompt: "Codex-specific Loom system instructions", Timeout: 2 * time.Minute,
		MaxOutputBytes: 64 << 10,
	}
	result, err := runner.RunHarness(
		context.Background(), request, []byte("private-openai-key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !allHarnessBytesZero(request.Prompt) {
		t.Fatal("Codex Harness prompt remained after process completion")
	}
	if gateway.providerID != CodexProviderID || gateway.modelID != CodexModelID ||
		string(gateway.secret) != "private-openai-key" {
		t.Fatalf("gateway = %#v", gateway)
	}
	command := commands.request
	if command.ExecutablePath != request.ExecutablePath || command.Directory != request.TempPath ||
		!bytes.Equal(command.Stdin, []byte("Implement the bounded Codex change")) ||
		command.Timeout != request.Timeout {
		t.Fatalf("command = %#v", command)
	}
	wantArguments := []string{
		"exec", "--ignore-user-config",
		"-c", `model_provider="loom_gateway"`,
		"-c", `model_providers.loom_gateway.name="Loom OpenAI Gateway"`,
		"-c", `model_providers.loom_gateway.base_url="http://127.0.0.1:43124/v1"`,
		"-c", `model_providers.loom_gateway.env_key="LOOM_CODEX_ATTEMPT_TOKEN"`,
		"-c", `model_providers.loom_gateway.wire_api="responses"`,
		"-c", `model_providers.loom_gateway.request_max_retries=0`,
		"-c", `model_providers.loom_gateway.stream_max_retries=0`,
		"-c", `model_instructions_file="` + harnessSystemPromptPath(tempPath) + `"`,
		"-c", `mcp_servers={}`,
		"-c", `features.shell_tool=false`,
		"-c", `features.unified_exec=false`,
		"-c", `features.apply_patch_freeform=false`,
		"-c", `features.tool_search=false`,
		"-c", `approval_policy="never"`,
		"--strict-config", "--json", "--ephemeral", "--ignore-rules",
		"--skip-git-repo-check", "--color", "never",
		"--sandbox", "read-only", "--cd", request.TempPath,
		"--model", CodexModelID,
		"-c", `model_reasoning_effort="high"`, "-",
	}
	if !reflect.DeepEqual(command.Arguments, wantArguments) {
		t.Fatalf("arguments = %#v", command.Arguments)
	}
	if _, err := os.Lstat(harnessSystemPromptPath(tempPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("system prompt file was not removed: %v", err)
	}
	environment := strings.Join(command.Environment, "\n")
	for _, expected := range []string{
		"CODEX_HOME=/private/tmp/loom/home",
		"LOOM_CODEX_ATTEMPT_TOKEN=one-time-codex-token",
		"CODEX_MANAGED_BY_LOOM=1",
	} {
		if !strings.Contains(environment, expected) {
			t.Fatalf("environment missing %q: %s", expected, environment)
		}
	}
	if strings.Contains(environment, "private-openai-key") ||
		strings.Contains(strings.Join(command.Arguments, "\n"), "Implement the bounded Codex change") ||
		strings.Contains(strings.Join(command.Arguments, "\n"), "fallback") {
		t.Fatalf("unsafe command args=%#v env=%s", command.Arguments, environment)
	}
	if result.Content != "Implemented the Codex change" || len(result.Stderr) != 0 ||
		result.Accounting == nil || result.Accounting.InputTokens != 140 ||
		result.Accounting.OutputTokens != 32 || result.Accounting.TotalTokens != 172 ||
		result.Accounting.CacheReadTokens != 25 || result.Accounting.CostObserved {
		t.Fatalf("result = %#v", result)
	}
}

func TestDecodeCodexResultFailsClosedWithoutFinalMessageAndUsage(t *testing.T) {
	for _, payload := range [][]byte{
		[]byte(`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}` + "\n"),
		[]byte(`{"type":"item.completed","item":{"type":"agent_message","text":"done"}}` + "\n"),
		[]byte(`{"type":"turn.failed","error":{"message":"private"}}` + "\n"),
	} {
		if _, err := decodeCodexResult(payload, 4096); err == nil {
			t.Fatalf("invalid Codex output accepted: %s", payload)
		}
	}
}

func TestCodexProcessInjectsOnlyAttemptContextMCPLease(t *testing.T) {
	gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
		BaseURL: "http://127.0.0.1:43124", Token: "provider-attempt-token",
	}}
	commands := &commandRunnerFixture{result: HarnessCommandResult{
		Stdout: []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"done"}}` + "\n" +
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}` + "\n"),
	}}
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{Gateway: gateway, Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	request := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/codex", WorkspacePath: "/private/tmp/workspace",
		HomePath: "/private/tmp/home", TempPath: t.TempDir(), ModelID: CodexModelID,
		Prompt: []byte("bounded"), SystemPrompt: "bounded system", Timeout: time.Minute,
		MaxOutputBytes: 4096, ContextMCP: HarnessContextMCPLease{
			URL: "http://127.0.0.1:43129/mcp", Token: strings.Repeat("a", 64),
		},
	}
	if _, err := runner.RunHarness(context.Background(), request, []byte("provider-secret")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gateway.policy.AllowedLoomTools, []string{"loom_read_context"}) {
		t.Fatalf("gateway policy = %#v", gateway.policy)
	}
	arguments := strings.Join(commands.request.Arguments, "\n")
	environment := strings.Join(commands.request.Environment, "\n")
	for _, expected := range []string{
		`mcp_servers.loom_context.url="http://127.0.0.1:43129/mcp"`,
		`mcp_servers.loom_context.bearer_token_env_var="LOOM_CONTEXT_ATTEMPT_TOKEN"`,
		`mcp_servers.loom_context.enabled_tools=["loom_read_context"]`,
		`mcp_servers.loom_context.tools.loom_read_context.approval_mode="approve"`,
	} {
		if !strings.Contains(arguments, expected) {
			t.Fatalf("arguments missing %q: %s", expected, arguments)
		}
	}
	if strings.Count(arguments, "mcp_servers.loom_context.url=") != 1 ||
		strings.Contains(arguments, request.ContextMCP.Token) ||
		!strings.Contains(environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token) ||
		strings.Contains(environment, "provider-secret") {
		t.Fatalf("unsafe MCP args=%s env=%s", arguments, environment)
	}
}

func TestCodexProcessInjectsGovernedAttemptMCPTools(t *testing.T) {
	gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
		BaseURL: "http://127.0.0.1:43124", Token: "provider-attempt-token",
	}}
	commands := &commandRunnerFixture{result: HarnessCommandResult{
		Stdout: []byte(`{"type":"item.completed","item":{"type":"agent_message","text":"done"}}` + "\n" +
			`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}` + "\n"),
	}}
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{Gateway: gateway, Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	request := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/codex", WorkspacePath: "/private/tmp/workspace",
		HomePath: "/private/tmp/home", TempPath: t.TempDir(), ModelID: CodexModelID,
		Prompt: []byte("bounded"), SystemPrompt: "bounded system", Timeout: time.Minute,
		MaxOutputBytes: 4096, ContextMCP: HarnessContextMCPLease{
			URL: "http://127.0.0.1:43129/mcp", Token: strings.Repeat("c", 64),
			ContextEnabled: true, ReadEnabled: true, GrepEnabled: true,
		},
	}
	if _, err := runner.RunHarness(context.Background(), request, []byte("provider-secret")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gateway.policy.AllowedLoomTools, []string{
		"loom_grep_files", "loom_read_context", "loom_read_file",
	}) {
		t.Fatalf("gateway policy = %#v", gateway.policy)
	}
	arguments := strings.Join(commands.request.Arguments, "\n")
	appServerArguments := strings.Join(codexAppServerArguments(
		request, gateway.lease, "/private/tmp/loom-system-prompt.txt",
	), "\n")
	for _, expected := range []string{
		`mcp_servers.loom_context.enabled_tools=["loom_grep_files","loom_read_context","loom_read_file"]`,
		`mcp_servers.loom_context.tools.loom_grep_files.approval_mode="approve"`,
		`mcp_servers.loom_context.tools.loom_read_context.approval_mode="approve"`,
		`mcp_servers.loom_context.tools.loom_read_file.approval_mode="approve"`,
	} {
		if !strings.Contains(arguments, expected) {
			t.Fatalf("arguments missing %q: %s", expected, arguments)
		}
		if !strings.Contains(appServerArguments, expected) {
			t.Fatalf("app-server arguments missing %q: %s", expected, appServerArguments)
		}
	}
	for _, forbidden := range []string{"loom_bash", "loom_edit_file", "loom_web_search", "loom_mcp_tool"} {
		if strings.Contains(arguments, forbidden) {
			t.Fatalf("arguments exposed %q: %s", forbidden, arguments)
		}
		if strings.Contains(appServerArguments, forbidden) {
			t.Fatalf("app-server arguments exposed %q: %s", forbidden, appServerArguments)
		}
	}
}
