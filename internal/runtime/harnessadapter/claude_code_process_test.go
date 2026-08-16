package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type attemptGatewayFixture struct {
	lease      AttemptGatewayLease
	err        error
	runs       int
	providerID string
	modelID    string
	secret     []byte
	policy     AttemptProviderToolPolicy
}

func (gateway *attemptGatewayFixture) WithCredential(
	_ context.Context,
	providerID string,
	modelID string,
	secret []byte,
	policy AttemptProviderToolPolicy,
	use func(AttemptGatewayLease) error,
) error {
	gateway.runs++
	gateway.providerID = providerID
	gateway.modelID = modelID
	gateway.secret = append([]byte(nil), secret...)
	gateway.policy = policy
	if gateway.err != nil {
		return gateway.err
	}
	return use(gateway.lease)
}

type commandRunnerFixture struct {
	result  HarnessCommandResult
	err     error
	hook    func(HarnessCommandRequest) error
	runs    int
	request HarnessCommandRequest
}

func (runner *commandRunnerFixture) RunCommand(
	_ context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	runner.runs++
	runner.request = request
	runner.request.Stdin = bytes.Clone(request.Stdin)
	if runner.hook != nil {
		if err := runner.hook(request); err != nil {
			return HarnessCommandResult{}, err
		}
	}
	return runner.result, runner.err
}

func TestHarnessProcessesFailClosedWhenSystemPromptCleanupFails(t *testing.T) {
	tests := []struct {
		name       string
		modelID    string
		providerID string
		stdout     []byte
		newRunner  func(AttemptCredentialGateway, HarnessCommandRunner) (HarnessProcessRunner, error)
	}{
		{
			name: "claude-code", modelID: ClaudeCodeModelID,
			providerID: ClaudeCodeProviderID,
			stdout:     []byte(`{"type":"result","subtype":"success","is_error":false,"result":"done","usage":{"input_tokens":1,"output_tokens":1}}`),
			newRunner: func(gateway AttemptCredentialGateway, commands HarnessCommandRunner) (HarnessProcessRunner, error) {
				return NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
					Gateway: gateway, Commands: commands,
				})
			},
		},
		{
			name: "codex", modelID: CodexModelID, providerID: CodexProviderID,
			stdout: []byte(
				`{"type":"item.completed","item":{"type":"agent_message","text":"done"}}` + "\n" +
					`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}}` + "\n",
			),
			newRunner: func(gateway AttemptCredentialGateway, commands HarnessCommandRunner) (HarnessProcessRunner, error) {
				return NewCodexProcessRunner(CodexProcessRunnerConfig{
					Gateway: gateway, Commands: commands,
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempPath := t.TempDir()
			commands := &commandRunnerFixture{
				result: HarnessCommandResult{Stdout: test.stdout},
				hook: func(HarnessCommandRequest) error {
					path := harnessSystemPromptPath(tempPath)
					if err := os.Remove(path); err != nil {
						return err
					}
					if err := os.Mkdir(path, 0o700); err != nil {
						return err
					}
					return os.WriteFile(filepath.Join(path, "cleanup-blocker"), []byte("x"), 0o600)
				},
			}
			gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
				BaseURL: "http://127.0.0.1:43124", Token: "attempt-token",
			}}
			runner, err := test.newRunner(gateway, commands)
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
				ExecutablePath: "/opt/loom/bin/" + test.name,
				WorkspacePath:  "/private/tmp/loom/workspace",
				HomePath:       "/private/tmp/loom/home", TempPath: tempPath,
				ModelID: test.modelID, Prompt: []byte("bounded user request"),
				SystemPrompt: "bounded private system instructions",
				Timeout:      time.Minute, MaxOutputBytes: 4096,
			}, []byte("private-provider-key"))
			if !errors.Is(err, ErrHarnessProtocol) {
				t.Fatalf("RunHarness cleanup error=%v provider=%s", err, test.providerID)
			}
		})
	}
}

func TestHarnessAttemptCancellationReapsProcessAndPrivatePrompt(t *testing.T) {
	tests := []struct {
		name      string
		modelID   string
		newRunner func(AttemptCredentialGateway, HarnessCommandRunner) (HarnessProcessRunner, error)
	}{
		{
			name: "claude-code", modelID: ClaudeCodeModelID,
			newRunner: func(gateway AttemptCredentialGateway, commands HarnessCommandRunner) (HarnessProcessRunner, error) {
				return NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
					Gateway: gateway, Commands: commands,
				})
			},
		},
		{
			name: "codex", modelID: CodexModelID,
			newRunner: func(gateway AttemptCredentialGateway, commands HarnessCommandRunner) (HarnessProcessRunner, error) {
				return NewCodexProcessRunner(CodexProcessRunnerConfig{
					Gateway: gateway, Commands: commands,
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			workspacePath := filepath.Join(root, "workspace")
			homePath := filepath.Join(root, "home")
			tempPath := filepath.Join(root, "temp")
			binPath := filepath.Join(root, "bin")
			for _, path := range []string{workspacePath, homePath, tempPath, binPath} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			executablePath := filepath.Join(binPath, test.name)
			script := `#!/bin/sh
sleep 30 &
child=$!
printf '%s' "$child" > "$TMPDIR/harness-child.pid"
wait "$child"
`
			if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
				BaseURL: "http://127.0.0.1:43124", Token: "attempt-token",
			}}
			runner, err := test.newRunner(gateway, NewSystemHarnessCommandRunner())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			answer := make(chan error, 1)
			go func() {
				_, runErr := runner.RunHarness(ctx, HarnessProcessRequest{
					ExecutablePath: executablePath, WorkspacePath: workspacePath,
					HomePath: homePath, TempPath: tempPath, ModelID: test.modelID,
					Prompt:       []byte("bounded user request"),
					SystemPrompt: "bounded private system instructions",
					Timeout:      time.Minute, MaxOutputBytes: 4096,
				}, []byte("private-provider-key"))
				answer <- runErr
			}()
			pidPath := filepath.Join(tempPath, "harness-child.pid")
			var pid int
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				value, readErr := os.ReadFile(pidPath)
				if readErr == nil {
					pid, err = strconv.Atoi(strings.TrimSpace(string(value)))
					if err == nil && pid > 1 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid <= 1 {
				cancel()
				t.Fatalf("Harness child process did not start: pid=%d err=%v", pid, err)
			}
			cancel()
			select {
			case err := <-answer:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("RunHarness cancellation=%v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Harness cancellation did not finish")
			}
			for _, path := range []string{
				harnessSystemPromptPath(tempPath), filepath.Join(tempPath, ".loom-private"),
			} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("private Prompt residue at %s: %v", path, err)
				}
			}
			processDeadline := time.Now().Add(2 * time.Second)
			for {
				killErr := syscall.Kill(pid, 0)
				if errors.Is(killErr, syscall.ESRCH) {
					break
				}
				if killErr != nil || time.Now().After(processDeadline) {
					t.Fatalf("Harness child %d survived cancellation: %v", pid, killErr)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestClaudeCodeProcessUsesAttemptGatewayWithoutExposingProviderSecret(t *testing.T) {
	tempPath := t.TempDir()
	gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
		BaseURL: "http://127.0.0.1:43123",
		Token:   "one-time-attempt-token",
	}}
	commands := &commandRunnerFixture{result: HarnessCommandResult{
		Stdout: []byte(`{
			"type":"result","subtype":"success","is_error":false,
			"result":"Changed two files safely","total_cost_usd":0.275,
			"usage":{"input_tokens":120,"output_tokens":31,
			"cache_read_input_tokens":20,"cache_creation_input_tokens":4}
		}`),
		Stderr:   []byte("raw provider detail must be dropped"),
		ExitCode: 0,
	}}
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
		Gateway:  gateway,
		Commands: commands,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/claude",
		WorkspacePath:  "/private/tmp/loom/workspace",
		HomePath:       "/private/tmp/loom/home",
		TempPath:       tempPath,
		ModelID:        ClaudeCodeModelID,
		Prompt:         []byte("Implement the bounded change"),
		SystemPrompt:   "Claude-specific Loom system instructions",
		Timeout:        2 * 60 * 1e9,
		MaxOutputBytes: 64 << 10,
	}
	result, err := runner.RunHarness(
		context.Background(),
		request,
		[]byte("private-anthropic-key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !allHarnessBytesZero(request.Prompt) {
		t.Fatal("Claude Code Harness prompt remained after process completion")
	}

	if gateway.runs != 1 || gateway.providerID != ClaudeCodeProviderID ||
		gateway.modelID != ClaudeCodeModelID ||
		string(gateway.secret) != "private-anthropic-key" {
		t.Fatalf("gateway = %#v", gateway)
	}
	if commands.runs != 1 {
		t.Fatalf("command runs = %d", commands.runs)
	}
	command := commands.request
	if command.ExecutablePath != request.ExecutablePath ||
		command.Directory != request.TempPath ||
		!bytes.Equal(command.Stdin, []byte("Implement the bounded change")) ||
		command.MaxOutputBytes != request.MaxOutputBytes {
		t.Fatalf("command = %#v", command)
	}
	wantArguments := []string{
		"--print",
		"--bare",
		"--no-session-persistence",
		"--output-format", "json",
		"--model", ClaudeCodeModelID,
		"--system-prompt-file", harnessSystemPromptPath(tempPath),
		"--permission-mode", "dontAsk",
		"--tools", "",
		"--allowedTools", "",
		"--disallowedTools", "Bash,Read,Edit,Write,Glob,Grep,WebFetch,WebSearch,Task,Skill,NotebookEdit,TodoWrite",
		"--disable-slash-commands",
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
	}
	if !reflect.DeepEqual(command.Arguments, wantArguments) {
		t.Fatalf("arguments = %#v", command.Arguments)
	}
	if _, err := os.Lstat(harnessSystemPromptPath(tempPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("system prompt file was not removed: %v", err)
	}
	environment := strings.Join(command.Environment, "\n")
	for _, required := range []string{
		"HOME=/private/tmp/loom/home",
		"TMPDIR=" + tempPath,
		"ANTHROPIC_BASE_URL=http://127.0.0.1:43123",
		"ANTHROPIC_API_KEY=one-time-attempt-token",
		"CLAUDE_CODE_PROVIDER_MANAGED_BY_HOST=1",
		"CLAUDE_CODE_SIMPLE=1",
		"DISABLE_TELEMETRY=1",
		"DISABLE_ERROR_REPORTING=1",
		"DISABLE_AUTOUPDATER=1",
	} {
		if !strings.Contains(environment, required) {
			t.Fatalf("environment missing %q: %s", required, environment)
		}
	}
	if strings.Contains(environment, "private-anthropic-key") ||
		strings.Contains(strings.Join(command.Arguments, "\n"), "Implement the bounded change") ||
		strings.Contains(strings.Join(command.Arguments, "\n"), "fallback") {
		t.Fatalf("unsafe command = args %#v env %s", command.Arguments, environment)
	}
	if result.Content != "Changed two files safely" || len(result.Stderr) != 0 ||
		result.Accounting == nil || result.Accounting.InputTokens != 120 ||
		result.Accounting.OutputTokens != 31 || result.Accounting.TotalTokens != 151 ||
		result.Accounting.CacheReadTokens != 20 ||
		result.Accounting.CacheWriteTokens != 4 ||
		result.Accounting.CostMicrounits != 275000 ||
		result.Accounting.CostCurrency != "USD" {
		t.Fatalf("result = %#v", result)
	}
}

func TestClaudeCodeProcessInjectsOnlyAttemptContextMCPLease(t *testing.T) {
	gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
		BaseURL: "http://127.0.0.1:43123", Token: "provider-attempt-token",
	}}
	commands := &commandRunnerFixture{result: HarnessCommandResult{
		Stdout: []byte(`{"type":"result","subtype":"success","is_error":false,"result":"done","usage":{"input_tokens":1,"output_tokens":1}}`),
	}}
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{Gateway: gateway, Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	request := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/claude", WorkspacePath: "/private/tmp/workspace",
		HomePath: "/private/tmp/home", TempPath: t.TempDir(), ModelID: ClaudeCodeModelID,
		Prompt: []byte("bounded"), SystemPrompt: "bounded system", Timeout: time.Minute,
		MaxOutputBytes: 4096, ContextMCP: HarnessContextMCPLease{
			URL: "http://127.0.0.1:43130/mcp", Token: strings.Repeat("b", 64),
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
	if !strings.Contains(arguments, `"type":"http"`) ||
		!strings.Contains(arguments, `"url":"http://127.0.0.1:43130/mcp"`) ||
		!strings.Contains(arguments, `Bearer ${LOOM_CONTEXT_ATTEMPT_TOKEN}`) ||
		!strings.Contains(arguments, "mcp__loom_context__loom_read_context") ||
		strings.Contains(arguments, request.ContextMCP.Token) ||
		!strings.Contains(environment, harnessContextMCPTokenEnv+"="+request.ContextMCP.Token) ||
		strings.Contains(environment, "provider-secret") {
		t.Fatalf("unsafe MCP args=%s env=%s", arguments, environment)
	}
}

func TestClaudeCodeProcessInjectsGovernedAttemptMCPTools(t *testing.T) {
	gateway := &attemptGatewayFixture{lease: AttemptGatewayLease{
		BaseURL: "http://127.0.0.1:43123", Token: "provider-attempt-token",
	}}
	commands := &commandRunnerFixture{result: HarnessCommandResult{
		Stdout: []byte(`{"type":"result","subtype":"success","is_error":false,"result":"done","usage":{"input_tokens":1,"output_tokens":1}}`),
	}}
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{Gateway: gateway, Commands: commands})
	if err != nil {
		t.Fatal(err)
	}
	request := HarnessProcessRequest{
		ExecutablePath: "/opt/loom/bin/claude", WorkspacePath: "/private/tmp/workspace",
		HomePath: "/private/tmp/home", TempPath: t.TempDir(), ModelID: ClaudeCodeModelID,
		Prompt: []byte("bounded"), SystemPrompt: "bounded system", Timeout: time.Minute,
		MaxOutputBytes: 4096, ContextMCP: HarnessContextMCPLease{
			URL: "http://127.0.0.1:43130/mcp", Token: strings.Repeat("d", 64),
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
	streamArguments := strings.Join(claudeCodeStreamArguments(
		request, "/private/tmp/loom-system-prompt.txt",
	), "\n")
	for _, expected := range []string{
		"mcp__loom_context__loom_grep_files",
		"mcp__loom_context__loom_read_context",
		"mcp__loom_context__loom_read_file",
	} {
		if !strings.Contains(arguments, expected) {
			t.Fatalf("arguments missing %q: %s", expected, arguments)
		}
		if !strings.Contains(streamArguments, expected) {
			t.Fatalf("stream arguments missing %q: %s", expected, streamArguments)
		}
	}
	for _, forbidden := range []string{"mcp__loom_context__loom_bash", "mcp__loom_context__loom_edit_file", "mcp__loom_context__loom_web_search"} {
		if strings.Contains(arguments, forbidden) {
			t.Fatalf("arguments exposed %q: %s", forbidden, arguments)
		}
		if strings.Contains(streamArguments, forbidden) {
			t.Fatalf("stream arguments exposed %q: %s", forbidden, streamArguments)
		}
	}
}

func TestClaudeCodeProcessClassifiesClosedFailures(t *testing.T) {
	tests := []struct {
		name       string
		gatewayErr error
		command    HarnessCommandResult
		commandErr error
		want       error
	}{
		{name: "gateway auth", gatewayErr: ErrHarnessProviderAuth, want: ErrHarnessProviderAuth},
		{name: "rate limit", gatewayErr: ErrHarnessProviderRateLimit, want: ErrHarnessProviderRateLimit},
		{name: "process unavailable", commandErr: errors.New("raw process error"), want: ErrHarnessProcessUnavailable},
		{name: "nonzero exit", command: HarnessCommandResult{ExitCode: 1}, want: ErrHarnessProcessUnavailable},
		{name: "invalid output", command: HarnessCommandResult{ExitCode: 0, Stdout: []byte(`{"type":"result","is_error":false}`)}, want: ErrHarnessProtocol},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempPath := t.TempDir()
			gateway := &attemptGatewayFixture{
				lease: AttemptGatewayLease{BaseURL: "http://127.0.0.1:43123", Token: "token"},
				err:   test.gatewayErr,
			}
			commands := &commandRunnerFixture{result: test.command, err: test.commandErr}
			runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
				Gateway: gateway, Commands: commands,
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.RunHarness(context.Background(), HarnessProcessRequest{
				ExecutablePath: "/opt/loom/bin/claude",
				WorkspacePath:  "/private/tmp/loom/workspace",
				HomePath:       "/private/tmp/loom/home", TempPath: tempPath,
				ModelID: ClaudeCodeModelID, Prompt: []byte("Fix it"),
				SystemPrompt: "Claude-specific Loom system instructions", Timeout: 2 * 60 * 1e9,
				MaxOutputBytes: 64 << 10,
			}, []byte("private-anthropic-key"))
			if !errors.Is(err, test.want) {
				t.Fatalf("RunHarness() error = %v, want %v", err, test.want)
			}
		})
	}
}
