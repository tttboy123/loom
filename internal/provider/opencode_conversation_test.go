package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type openCodeConversationRunnerFixture struct {
	output []byte
	err    error
	seen   OpenCodeConversationProcessRequest
}

func (fixture *openCodeConversationRunnerFixture) RunOpenCodeConversation(
	_ context.Context,
	request OpenCodeConversationProcessRequest,
) ([]byte, error) {
	fixture.seen = request
	if fixture.err != nil {
		return nil, fixture.err
	}
	return append([]byte(nil), fixture.output...), nil
}

func openCodeExecutable(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode")
	writeExecutableFixture(t, path, "#!/bin/sh\nexit 0\n")
	return path
}

func openCodeConversationConfig(
	t *testing.T,
	runner OpenCodeConversationProcessRunner,
) OpenCodeConversationConfig {
	return OpenCodeConversationConfig{
		ExecutablePath: openCodeExecutable(t),
		HomePath:       "/Users/test",
		PrivateRoot:    filepath.Join(t.TempDir(), "opencode"),
		ModelID:        "deepseek/deepseek-chat",
		Timeout:        time.Minute,
		MaxOutputBytes: 32 * 1024,
		Runner:         runner,
	}
}

func TestOpenCodeConversationClientRespondsBounded(t *testing.T) {
	runner := &openCodeConversationRunnerFixture{
		output: []byte("bounded OpenCode reply"),
	}
	client, err := NewOpenCodeConversationClient(
		openCodeConversationConfig(t, runner),
	)
	if err != nil {
		t.Fatalf("NewOpenCodeConversationClient error = %v", err)
	}
	response, err := client.Respond(context.Background(), "  summarize the bounded change  ")
	if err != nil || response != "bounded OpenCode reply" {
		t.Fatalf("Respond() = %q error=%v", response, err)
	}
	if runner.seen.Prompt != "summarize the bounded change" ||
		runner.seen.ModelID != "deepseek/deepseek-chat" ||
		runner.seen.MaxOutputBytes != 32*1024 {
		t.Fatalf("runner request = %#v", runner.seen)
	}
}

func TestOpenCodeConversationClientFailsClosed(t *testing.T) {
	t.Run("runner error", func(t *testing.T) {
		client, err := NewOpenCodeConversationClient(openCodeConversationConfig(
			t, &openCodeConversationRunnerFixture{err: errors.New("boom")},
		))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Respond(context.Background(), "hello"); !errors.Is(
			err, ErrOpenCodeConversationUnavailable,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("empty reply", func(t *testing.T) {
		client, err := NewOpenCodeConversationClient(openCodeConversationConfig(
			t, &openCodeConversationRunnerFixture{output: []byte("   ")},
		))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Respond(context.Background(), "hello"); !errors.Is(
			err, ErrOpenCodeConversationUnavailable,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("invalid model", func(t *testing.T) {
		config := openCodeConversationConfig(t, &openCodeConversationRunnerFixture{})
		config.ModelID = "bad model"
		if _, err := NewOpenCodeConversationClient(config); !errors.Is(
			err, ErrInvalidOpenCodeConversationConfig,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("deadline remains classifiable", func(t *testing.T) {
		client, err := NewOpenCodeConversationClient(openCodeConversationConfig(
			t, &openCodeConversationRunnerFixture{err: context.DeadlineExceeded},
		))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Respond(context.Background(), "hello")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline classification lost: %v", err)
		}
	})
}

func TestDecodeOpenCodeConversationEventStream(t *testing.T) {
	event := func(body string) string {
		return `{"type":"message.part.updated","properties":{"sessionID":"s1","time":0,"part":` + body + `}}`
	}
	payload := strings.Join([]string{
		event(`{"id":"p1","messageID":"m1","sessionID":"s1","type":"text","text":"bounded "}`),
		event(`{"id":"p2","messageID":"m1","sessionID":"s1","type":"text","text":"reply"}`),
		`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
	}, "\n")
	decoded, err := decodeOpenCodeConversation([]byte(payload), 32*1024)
	if err != nil || string(decoded) != "bounded reply" {
		t.Fatalf("decoded=%q error=%v", decoded, err)
	}

	t.Run("auth error fails closed", func(t *testing.T) {
		bad := payload + "\n" + `{"type":"auth.error","error":"unauthorized"}`
		if _, err := decodeOpenCodeConversation([]byte(bad), 32*1024); !errors.Is(
			err, ErrOpenCodeConversationUnavailable,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("missing idle fails", func(t *testing.T) {
		if _, err := decodeOpenCodeConversation([]byte(payload), 32*1024); err != nil {
			// payload has idle; use a truncated one
			_ = err
		}
		truncated := strings.Join([]string{
			event(`{"id":"p1","messageID":"m1","sessionID":"s1","type":"text","text":"x"}`),
		}, "\n")
		if _, err := decodeOpenCodeConversation([]byte(truncated), 32*1024); !errors.Is(
			err, ErrOpenCodeConversationUnavailable,
		) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("tool parts fail closed", func(t *testing.T) {
		withTool := strings.Join([]string{
			event(`{"id":"p0","messageID":"m1","sessionID":"s1","type":"tool","tool":"x"}`),
			event(`{"id":"p1","messageID":"m1","sessionID":"s1","type":"text","text":"final"}`),
			`{"type":"session.idle","schema":{"sessionID":"s1"}}`,
		}, "\n")
		if _, err := decodeOpenCodeConversation(
			[]byte(withTool), 32*1024,
		); !errors.Is(err, ErrOpenCodeConversationUnavailable) {
			t.Fatalf("error=%v", err)
		}
		decoded, err := DecodeOpenCodeText([]byte(withTool), 32*1024)
		if err != nil || string(decoded) != "final" {
			t.Fatalf("agent decoded=%q error=%v", decoded, err)
		}
	})
	t.Run("malformed line fails", func(t *testing.T) {
		if _, err := decodeOpenCodeConversation(
			[]byte(`not-json`+"\n"+`{"type":"session.idle","schema":{}}`), 32*1024,
		); !errors.Is(err, ErrOpenCodeConversationUnavailable) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("structured provider errors retain safe classification", func(t *testing.T) {
		tests := []struct {
			status int
			want   error
		}{
			{status: 401, want: ErrOpenCodeConversationAuth},
			{status: 402, want: ErrOpenCodeConversationInsufficientBalance},
			{status: 404, want: ErrOpenCodeConversationModelUnavailable},
			{status: 429, want: ErrOpenCodeConversationRateLimit},
		}
		for _, test := range tests {
			payload := []byte(`{"type":"error","error":{"name":"APIError","statusCode":` +
				strconv.Itoa(test.status) + `,"message":"must not escape"}}`)
			_, err := decodeOpenCodeConversation(payload, 32*1024)
			if !errors.Is(err, test.want) || strings.Contains(err.Error(), "must not escape") {
				t.Fatalf("status=%d error=%v want=%v", test.status, err, test.want)
			}
		}
		payload := []byte(`{"type":"error","error":{"name":"APIError","data":{"statusCode":400,"message":"Error from provider: Model is unavailable."}}}`)
		_, err := decodeOpenCodeConversation(payload, 32*1024)
		if !errors.Is(err, ErrOpenCodeConversationModelUnavailable) ||
			strings.Contains(err.Error(), "Error from provider") {
			t.Fatalf("retired model error = %v", err)
		}
	})
}

func TestResolveOpenCodeNativeExecutableFollowsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "opencode-darwin-arm64")
	writeExecutableFixture(t, target, "#!/bin/sh\nexit 0\n")
	if canonical, err := filepath.EvalSymlinks(target); err == nil {
		target = canonical
	}
	link := filepath.Join(root, "opencode")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveOpenCodeNativeExecutable(link)
	if err != nil || resolved != target {
		t.Fatalf("resolved=%q error=%v", resolved, err)
	}
	direct, err := ResolveOpenCodeNativeExecutable(target)
	if err != nil || direct != target {
		t.Fatalf("direct=%q error=%v", direct, err)
	}
	if _, err := ResolveOpenCodeNativeExecutable(
		filepath.Join(root, "missing"),
	); err == nil {
		t.Fatal("missing executable accepted")
	}
}

func TestSystemOpenCodeConversationRunnerUsesOnlyExactLeaseEnvironment(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "opencode")
	writeExecutableFixture(t, executable, `#!/bin/sh
if [ "${MINIMAX_API_KEY:-}" != "vault-exact" ]; then
  exit 31
fi
if [ -n "${LOOM_UNRELATED_SECRET:-}" ]; then
  exit 32
fi
if [ "${OPENCODE_CONFIG_CONTENT:-}" != '{"permission":"deny","share":"disabled"}' ]; then
  exit 33
fi
printf '%s\n' '{"type":"text","part":{"type":"text","text":"MM-ENV-OK"}}'
printf '%s\n' '{"type":"session.idle"}'
`)
	t.Setenv("MINIMAX_API_KEY", "stale-parent-value")
	t.Setenv("LOOM_UNRELATED_SECRET", "must-not-enter-child")
	privateRoot := filepath.Join(root, "private")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := []byte("vault-exact")
	output, err := NewSystemOpenCodeConversationRunner().RunOpenCodeConversation(
		context.Background(),
		OpenCodeConversationProcessRequest{
			ExecutablePath:    executable,
			HomePath:          root,
			PrivateRoot:       privateRoot,
			ModelID:           "minimax-cn/MiniMax-M3",
			CredentialEnvName: "MINIMAX_API_KEY",
			Secret:            secret,
			Prompt:            "reply",
			MaxOutputBytes:    4096,
		},
	)
	if err != nil || string(output) != "MM-ENV-OK" {
		t.Fatalf("output=%q error=%v", output, err)
	}
}

func TestSystemOpenCodeConversationRunnerClassifiesEventBeforeExitCode(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "opencode")
	writeExecutableFixture(t, executable, `#!/bin/sh
printf '%s\n' '{"type":"error","error":{"name":"APIError","statusCode":402,"message":"must not escape"}}'
exit 1
`)
	privateRoot := filepath.Join(root, "private")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := NewSystemOpenCodeConversationRunner().RunOpenCodeConversation(
		context.Background(),
		OpenCodeConversationProcessRequest{
			ExecutablePath: executable,
			HomePath:       root,
			PrivateRoot:    privateRoot,
			ModelID:        "minimax-cn/MiniMax-M3",
			Prompt:         "reply",
			MaxOutputBytes: 4096,
		},
	)
	if !errors.Is(err, ErrOpenCodeConversationInsufficientBalance) ||
		strings.Contains(err.Error(), "must not escape") {
		t.Fatalf("error=%v", err)
	}
}
