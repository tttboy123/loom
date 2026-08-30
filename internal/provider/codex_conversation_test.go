package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func writeExecutableFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

type codexConversationRunnerFixture struct {
	request CodexConversationProcessRequest
	output  []byte
	err     error
}

func (runner *codexConversationRunnerFixture) RunCodexConversation(
	_ context.Context,
	request CodexConversationProcessRequest,
) ([]byte, error) {
	runner.request = request
	return append([]byte(nil), runner.output...), runner.err
}

func TestCodexConversationClientUsesBoundedReadOnlyEphemeralRequest(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	writeExecutableFixture(t, executable, "#!/bin/sh\nexit 0\n")
	runner := &codexConversationRunnerFixture{output: []byte("Hello from Codex\n")}
	client, err := NewCodexConversationClient(CodexConversationConfig{
		ExecutablePath: executable,
		HomePath:       root,
		PrivateRoot:    filepath.Join(root, "conversation"),
		Timeout:        2 * time.Minute,
		MaxOutputBytes: 32 * 1024,
		Runner:         runner,
	})
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.Respond(context.Background(), "user: hello")
	if err != nil {
		t.Fatal(err)
	}
	if response != "Hello from Codex" {
		t.Fatalf("response = %q", response)
	}
	if runner.request.ExecutablePath != executable ||
		runner.request.HomePath != root ||
		runner.request.PrivateRoot != filepath.Join(root, "conversation") ||
		runner.request.Prompt != "user: hello" ||
		runner.request.MaxOutputBytes != 32*1024 {
		t.Fatalf("request = %#v", runner.request)
	}
}

func TestCodexConversationClientRepairsLegacyPrivateDirectoryModes(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	writeExecutableFixture(t, executable, "#!/bin/sh\nexit 0\n")
	privateRoot := filepath.Join(root, "conversation")
	legacy := filepath.Join(privateRoot, ".tmp-legacy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCodexConversationClient(CodexConversationConfig{
		ExecutablePath: executable,
		HomePath:       root,
		PrivateRoot:    privateRoot,
		Timeout:        time.Minute,
		MaxOutputBytes: 1024,
		Runner:         &codexConversationRunnerFixture{output: []byte("ok")},
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("legacy private directory mode = %o", info.Mode().Perm())
	}
}

func TestCodexConversationClientRejectsInvalidAndOversizedResponses(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	writeExecutableFixture(t, executable, "#!/bin/sh\nexit 0\n")
	tests := []struct {
		name   string
		prompt string
		output []byte
	}{
		{name: "empty prompt", prompt: " ", output: []byte("unused")},
		{name: "empty response", prompt: "hello", output: []byte(" \n")},
		{name: "invalid utf8", prompt: "hello", output: []byte{0xff}},
		{name: "oversized response", prompt: "hello", output: make([]byte, 65)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &codexConversationRunnerFixture{output: test.output}
			client, err := NewCodexConversationClient(CodexConversationConfig{
				ExecutablePath: executable,
				HomePath:       root,
				PrivateRoot:    filepath.Join(root, "conversation-"+test.name),
				Timeout:        time.Second,
				MaxOutputBytes: 64,
				Runner:         runner,
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Respond(context.Background(), test.prompt)
			if !errors.Is(err, ErrCodexConversationUnavailable) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSystemCodexConversationRunnerUsesFixedReadOnlyInvocation(t *testing.T) {
	root := t.TempDir()
	privateRoot := filepath.Join(root, "conversation")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "codex")
	script := `#!/bin/sh
set -eu
[ "$1" = "exec" ]
[ "$2" = "--ephemeral" ]
[ "$3" = "--sandbox" ]
[ "$4" = "read-only" ]
[ "$5" = "--skip-git-repo-check" ]
[ "$6" = "--ignore-user-config" ]
[ "$7" = "--ignore-rules" ]
case "$TMPDIR" in
  "$PWD"/process-*) ;;
  *) exit 66 ;;
esac
[ "$(stat -f '%Lp' "$TMPDIR")" = "700" ]
mkdir -m 755 "$TMPDIR/child-default-mode"
[ "${LOOM_TEST_SECRET-}" = "" ]
output=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then
    shift
    output="$1"
  fi
  shift
done
[ -n "$output" ]
prompt=$(cat)
[ "$prompt" = "user: hello" ]
printf 'Read-only response\n' > "$output"
`
	writeExecutableFixture(t, executable, script)
	t.Setenv("LOOM_TEST_SECRET", "must-not-leak")

	output, err := NewSystemCodexConversationRunner().RunCodexConversation(
		context.Background(),
		CodexConversationProcessRequest{
			ExecutablePath: executable,
			HomePath:       root,
			PrivateRoot:    privateRoot,
			Prompt:         "user: hello",
			MaxOutputBytes: 1024,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "Read-only response\n" {
		t.Fatalf("output = %q", output)
	}
	entries, err := os.ReadDir(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("ephemeral Codex process state remained: %v", entries)
	}
}

func TestSystemCodexConversationRunnerCleansPrivateTempAfterFailure(t *testing.T) {
	root := t.TempDir()
	privateRoot := filepath.Join(root, "conversation")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "codex")
	writeExecutableFixture(t, executable, `#!/bin/sh
set -eu
mkdir -m 755 "$TMPDIR/child-default-mode"
exit 71
`)
	_, err := NewSystemCodexConversationRunner().RunCodexConversation(
		context.Background(),
		CodexConversationProcessRequest{
			ExecutablePath: executable,
			HomePath:       root,
			PrivateRoot:    privateRoot,
			Prompt:         "bounded failure",
			MaxOutputBytes: 1024,
		},
	)
	if !errors.Is(err, ErrCodexConversationUnavailable) {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed Codex process state remained: %v", entries)
	}
}

func TestCodexConversationArgumentsDisableEveryToolSurface(t *testing.T) {
	outputPath := "/private/tmp/response.txt"
	want := []string{
		"exec", "--ephemeral", "--sandbox", "read-only",
		"--skip-git-repo-check", "--ignore-user-config", "--ignore-rules",
	}
	for _, feature := range []string{
		"shell_tool", "unified_exec", "shell_snapshot", "apps",
		"enable_mcp_apps", "in_app_browser", "browser_use",
		"browser_use_external", "computer_use", "multi_agent",
		"image_generation", "code_mode_host", "tool_suggest",
	} {
		want = append(want, "--disable", feature)
	}
	want = append(want,
		"--color", "never", "-c", `approval_policy="never"`,
		"--output-last-message", outputPath, "-",
	)
	gatewayRequest := CodexConversationProcessRequest{
		ExecutablePath: "/opt/loom/bin/codex",
		HomePath:       "/Users/test",
		PrivateRoot:    "/private/codex",
		Prompt:         "implement",
		MaxOutputBytes: 32 * 1024,
	}
	if got := codexConversationArguments(outputPath, gatewayRequest); !reflect.DeepEqual(got, want) {
		t.Fatalf("arguments = %#v", got)
	}
}

func TestCodexConversationArgumentsNativeVsGateway(t *testing.T) {
	base := CodexConversationProcessRequest{
		ExecutablePath: "/opt/loom/bin/codex",
		HomePath:       "/Users/test",
		PrivateRoot:    "/private/codex",
		Prompt:         "implement",
		MaxOutputBytes: 32 * 1024,
	}
	gateway := codexConversationArguments("out.txt", base)
	if !containsString(gateway, "--ignore-user-config") ||
		containsString(gateway, "--model") {
		t.Fatalf("gateway args = %#v", gateway)
	}
	native := base
	native.ModelID = "deepseek-v4-flash"
	native.ReasoningEffort = "high"
	native.UseNativeAuth = true
	nativeArgs := codexConversationArguments("out.txt", native)
	if containsString(nativeArgs, "--ignore-user-config") ||
		!containsString(nativeArgs, "--model") ||
		!containsString(nativeArgs, "deepseek-v4-flash") ||
		!containsString(nativeArgs, `model_reasoning_effort="high"`) {
		t.Fatalf("native args = %#v", nativeArgs)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
