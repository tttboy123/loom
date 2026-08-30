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

func TestOpenCodeConversationClientRepairsLegacyPrivateDirectoryModes(t *testing.T) {
	runner := &openCodeConversationRunnerFixture{output: []byte("ok")}
	config := openCodeConversationConfig(t, runner)
	legacy := filepath.Join(config.PrivateRoot, "opencode")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewOpenCodeConversationClient(config); err != nil {
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
	t.Run("provider auth union retains only safe classification", func(t *testing.T) {
		payload := []byte(`{"type":"error","error":{"name":"ProviderAuthError","data":{"providerID":"minimax-cn","message":"api-key-auth-secret","responseBody":"response-secret","headers":{"authorization":"Bearer header-secret"},"secret":"raw-secret"}}}`)
		_, err := decodeOpenCodeConversation(payload, 32*1024)
		if !errors.Is(err, ErrOpenCodeConversationAuth) {
			t.Fatalf("error = %v", err)
		}
		failure, ok := ConversationFailureDetails(err)
		if !ok || failure.Code != "provider_auth" ||
			failure.Stage != "provider_auth" || failure.Retryable ||
			failure.ProviderCode != "" {
			t.Fatalf("failure = %#v, %v", failure, ok)
		}
		for _, secret := range []string{
			"minimax-cn", "api-key-auth-secret", "response-secret", "header-secret",
			"raw-secret",
		} {
			if strings.Contains(err.Error(), secret) ||
				strings.Contains(failure.UserMessage, secret) {
				t.Fatalf("error disclosed %q: %v", secret, err)
			}
		}
	})
	t.Run("structured provider errors retain safe classification", func(t *testing.T) {
		tests := []struct {
			name          string
			status        int
			retryable     bool
			want          error
			wantStage     string
			wantCode      string
			wantMessage   string
			wantRetryable bool
		}{
			{
				name: "auth", status: 401, want: ErrOpenCodeConversationAuth,
				wantStage: "provider_auth", wantCode: "provider_auth",
				wantMessage: "Provider rejected the selected account credential.",
			},
			{
				name: "balance", status: 402,
				want:      ErrOpenCodeConversationInsufficientBalance,
				wantStage: "provider_http", wantCode: "provider_insufficient_balance",
				wantMessage: "Provider account has insufficient balance.",
			},
			{
				name: "model unavailable", status: 404,
				want:      ErrOpenCodeConversationModelUnavailable,
				wantStage: "provider_http", wantCode: "provider_model_unavailable",
				wantMessage: "Selected model or Provider endpoint is unavailable.",
			},
			{
				name: "rate limit", status: 429, retryable: true,
				want:      ErrOpenCodeConversationRateLimit,
				wantStage: "provider_rate_limit", wantCode: "provider_rate_limit",
				wantMessage:   "Provider rate limit reached.",
				wantRetryable: true,
			},
			{
				name: "bad request", status: 400,
				want:      ErrOpenCodeConversationUnavailable,
				wantStage: "provider_http", wantCode: "provider_invalid_request",
				wantMessage: "Provider rejected the request parameters.",
			},
			{
				name: "retryable server failure", status: 503, retryable: true,
				want:      ErrOpenCodeConversationUnavailable,
				wantStage: "provider_http", wantCode: "provider_unavailable",
				wantMessage:   "Provider service is temporarily unavailable.",
				wantRetryable: true,
			},
			{
				name: "non-retryable server failure", status: 500,
				want:      ErrOpenCodeConversationUnavailable,
				wantStage: "provider_http", wantCode: "provider_unavailable",
				wantMessage: "Provider service is temporarily unavailable.",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				payload := []byte(`{"type":"error","error":{"name":"APIError","data":{"message":"invalid parameter; api-key-message-secret","statusCode":` +
					strconv.Itoa(test.status) + `,"isRetryable":` +
					strconv.FormatBool(test.retryable) + `,"responseBody":"response-body-secret","headers":{"authorization":"header-secret"},"secret":"raw-secret"}}}`)
				_, err := decodeOpenCodeConversation(payload, 32*1024)
				if !errors.Is(err, test.want) {
					t.Fatalf("status=%d error=%v want=%v", test.status, err, test.want)
				}
				failure, ok := ConversationFailureDetails(err)
				if !ok || failure.Stage != test.wantStage ||
					failure.Code != test.wantCode || failure.HTTPStatus != test.status ||
					failure.UserMessage != test.wantMessage ||
					failure.ProviderCode != "" ||
					failure.Retryable != test.wantRetryable {
					t.Fatalf("status=%d failure=%#v, %v", test.status, failure, ok)
				}
				for _, secret := range []string{
					"invalid parameter", "api-key-message-secret",
					"response-body-secret", "header-secret", "raw-secret",
				} {
					if strings.Contains(err.Error(), secret) ||
						strings.Contains(failure.UserMessage, secret) {
						t.Fatalf("status=%d error disclosed %q: %v", test.status, secret, err)
					}
				}
			})
		}
	})
	t.Run("retired model 400 retains safe model classification", func(t *testing.T) {
		payload := []byte(`{"type":"error","error":{"name":"APIError","data":{"message":"Error from provider: Model is unavailable; api-key-message-secret","statusCode":400,"isRetryable":false,"responseBody":"response-body-secret","headers":{"authorization":"header-secret"}}}}`)
		_, err := decodeOpenCodeConversation(payload, 32*1024)
		if !errors.Is(err, ErrOpenCodeConversationModelUnavailable) {
			t.Fatalf("error = %v", err)
		}
		failure, ok := ConversationFailureDetails(err)
		if !ok || failure.Stage != "provider_http" ||
			failure.Code != "provider_model_unavailable" ||
			failure.HTTPStatus != 400 || failure.Retryable {
			t.Fatalf("failure = %#v, %v", failure, ok)
		}
		for _, secret := range []string{
			"Error from provider", "api-key-message-secret",
			"response-body-secret", "header-secret",
		} {
			if strings.Contains(err.Error(), secret) ||
				strings.Contains(failure.UserMessage, secret) {
				t.Fatalf("error disclosed %q: %v", secret, err)
			}
		}
	})
	t.Run("unknown and malformed error unions fail closed", func(t *testing.T) {
		failures := map[string]string{
			"unknown name":            `{"name":"UnknownSecretError","data":{"statusCode":401,"isRetryable":false,"message":"unknown-secret"}}`,
			"missing name":            `{"data":{"statusCode":401,"isRetryable":false}}`,
			"missing data":            `{"name":"APIError"}`,
			"null data":               `{"name":"APIError","data":null}`,
			"array data":              `{"name":"APIError","data":[]}`,
			"legacy top-level status": `{"name":"APIError","statusCode":401,"isRetryable":false,"data":{}}`,
			"string status":           `{"name":"APIError","data":{"statusCode":"401","isRetryable":false}}`,
			"missing retryability":    `{"name":"APIError","data":{"statusCode":401}}`,
			"out of range status":     `{"name":"APIError","data":{"statusCode":999,"isRetryable":true}}`,
			"auth missing provider":   `{"name":"ProviderAuthError","data":{}}`,
		}
		for name, failure := range failures {
			t.Run(name, func(t *testing.T) {
				payload := []byte(`{"type":"error","error":` + failure + `}`)
				_, err := decodeOpenCodeConversation(payload, 32*1024)
				if err != ErrOpenCodeConversationUnavailable {
					t.Fatalf("error = %v", err)
				}
				if details, ok := ConversationFailureDetails(err); ok {
					t.Fatalf("unexpected failure details = %#v", details)
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatalf("error disclosed malformed payload: %v", err)
				}
			})
		}
	})
}

func TestDecodeOpenCodeControlHarnessRequiresStructuredArbitration(t *testing.T) {
	payload := strings.Join([]string{
		`{"type":"tool_use","sessionID":"s1","part":{"type":"tool","tool":"loom_control_loom_missions_search","state":{"status":"completed","input":{"query":"alpha"}}}}`,
		`{"type":"tool_use","sessionID":"s1","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"I prepared the Mission draft for review.","tool_name":"loom_missions_create_preview","tool_arguments":{"title":"Alpha"}}}}}`,
		`{"type":"step_finish","sessionID":"s1","part":{"type":"step-finish"}}`,
	}, "\n")
	for _, line := range strings.Split(payload, "\n") {
		if rejectConversationDuplicateJSONKeys([]byte(line)) {
			t.Fatalf("valid event rejected for duplicate JSON keys: %s", line)
		}
	}
	completion, err := DecodeOpenCodeControlHarness([]byte(payload), 32*1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(completion.Content) != "I prepared the Mission draft for review." ||
		completion.SelectedTool == nil ||
		completion.SelectedTool.Name != "loom_missions_create_preview" ||
		string(completion.SelectedTool.Arguments) != `{"title":"Alpha"}` ||
		len(completion.CompletedToolNames) != 1 ||
		completion.CompletedToolNames[0] != "loom_control_loom_missions_search" {
		t.Fatalf("completion = %#v", completion)
	}

	ordinary := strings.Join([]string{
		`{"type":"tool_use","sessionID":"s2","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"Hello from OpenCode.","tool_name":"none","tool_arguments":{}}}}}`,
		`{"type":"step_finish","sessionID":"s2","part":{"type":"step-finish"}}`,
	}, "\n")
	completion, err = DecodeOpenCodeControlHarness([]byte(ordinary), 32*1024)
	if err != nil || string(completion.Content) != "Hello from OpenCode." ||
		completion.SelectedTool != nil || len(completion.CompletedToolNames) != 0 {
		t.Fatalf("ordinary completion = %#v, %v", completion, err)
	}

	direct := strings.Join([]string{
		`{"type":"tool_use","sessionID":"s3","part":{"type":"tool","tool":"loom_control_loom_runtimes_status","state":{"status":"completed","input":{}}}}`,
		`{"type":"text","sessionID":"s3","part":{"type":"text","text":"The Runtime status is ready."}}`,
		`{"type":"step_finish","sessionID":"s3","part":{"type":"step-finish"}}`,
	}, "\n")
	completion, err = DecodeOpenCodeControlHarness([]byte(direct), 32*1024)
	if err != nil || string(completion.Content) != "The Runtime status is ready." ||
		completion.SelectedTool != nil || len(completion.CompletedToolNames) != 1 ||
		completion.CompletedToolNames[0] != "loom_control_loom_runtimes_status" {
		t.Fatalf("direct completion = %#v, %v", completion, err)
	}
}

func TestDecodeOpenCodeControlHarnessFailsClosed(t *testing.T) {
	structured := func(input string) string {
		return `{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":` + input + `}}}`
	}
	complete := `{"type":"step_finish","part":{"type":"step-finish"}}`
	tests := map[string]string{
		"plain text only": strings.Join([]string{
			`{"type":"text","part":{"type":"text","text":"not structured"}}`, complete,
		}, "\n"),
		"duplicate structured output": strings.Join([]string{
			structured(`{"response":"one","tool_name":"none","tool_arguments":{}}`),
			structured(`{"response":"two","tool_name":"none","tool_arguments":{}}`),
			complete,
		}, "\n"),
		"structured output error": strings.Join([]string{
			`{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"error","input":{"response":"no","tool_name":"none","tool_arguments":{}}}}}`,
			complete,
		}, "\n"),
		"unknown structured field": strings.Join([]string{
			structured(`{"response":"no","tool_name":"none","tool_arguments":{},"extra":true}`),
			complete,
		}, "\n"),
		"duplicate argument key": strings.Join([]string{
			structured(`{"response":"no","tool_name":"loom_missions_create_preview","tool_arguments":{"title":"one","title":"two"}}`),
			complete,
		}, "\n"),
		"none with arguments": strings.Join([]string{
			structured(`{"response":"no","tool_name":"none","tool_arguments":{"title":"unexpected"}}`),
			complete,
		}, "\n"),
		"invalid selected tool": strings.Join([]string{
			structured(`{"response":"no","tool_name":"Loom_Missions_Create","tool_arguments":{}}`),
			complete,
		}, "\n"),
		"missing completion": structured(
			`{"response":"no","tool_name":"none","tool_arguments":{}}`,
		),
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeOpenCodeControlHarness(
				[]byte(payload), 32*1024,
			); !errors.Is(err, ErrOpenCodeConversationUnavailable) {
				t.Fatalf("error = %v", err)
			}
		})
	}
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
	for argument in "$@"; do
	  if [ "$argument" = "private prompt over stdin" ]; then
	    exit 30
	  fi
	done
	IFS= read -r prompt
	if [ "$prompt" != "private prompt over stdin" ]; then
	  exit 34
	fi
	if [ "${MINIMAX_API_KEY:-}" != "vault-exact" ]; then
  exit 31
fi
if [ -n "${LOOM_UNRELATED_SECRET:-}" ]; then
  exit 32
fi
if [ "${OPENCODE_CONFIG_CONTENT:-}" != '{"permission":"deny","share":"disabled"}' ]; then
  exit 33
fi
case "$TMPDIR" in
  "$PWD"/process-*) ;;
  *) exit 35 ;;
esac
if [ "$(stat -f '%Lp' "$TMPDIR")" != "700" ]; then
  exit 36
fi
mkdir -m 755 "$TMPDIR/child-default-mode"
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
			Prompt:            "private prompt over stdin",
			MaxOutputBytes:    4096,
		},
	)
	if err != nil || string(output) != "MM-ENV-OK" {
		t.Fatalf("output=%q error=%v", output, err)
	}
	entries, err := os.ReadDir(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("ephemeral OpenCode process state remained: %v", entries)
	}
}

func TestSystemOpenCodeConversationRunnerCleansPrivateTempAfterFailure(t *testing.T) {
	root := t.TempDir()
	privateRoot := filepath.Join(root, "private")
	if err := os.Mkdir(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "opencode")
	writeExecutableFixture(t, executable, `#!/bin/sh
set -eu
mkdir -m 755 "$TMPDIR/child-default-mode"
exit 71
`)
	_, err := NewSystemOpenCodeConversationRunner().RunOpenCodeConversation(
		context.Background(),
		OpenCodeConversationProcessRequest{
			ExecutablePath: executable,
			HomePath:       root,
			PrivateRoot:    privateRoot,
			ModelID:        "opencode/big-pickle",
			Prompt:         "bounded failure",
			MaxOutputBytes: 4096,
		},
	)
	if !errors.Is(err, ErrOpenCodeConversationUnavailable) {
		t.Fatalf("error = %v", err)
	}
	entries, err := os.ReadDir(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed OpenCode process state remained: %v", entries)
	}
}

func TestSystemOpenCodeConversationRunnerClassifiesEventBeforeExitCode(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
		want      error
		wantCode  string
	}{
		{
			name: "balance", status: 402,
			want:     ErrOpenCodeConversationInsufficientBalance,
			wantCode: "provider_insufficient_balance",
		},
		{
			name: "bad request", status: 400,
			want:     ErrOpenCodeConversationUnavailable,
			wantCode: "provider_invalid_request",
		},
		{
			name: "retryable server failure", status: 503, retryable: true,
			want:     ErrOpenCodeConversationUnavailable,
			wantCode: "provider_unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			executable := filepath.Join(root, "opencode")
			errorEvent := `{"type":"error","error":{"name":"APIError","data":{"statusCode":` +
				strconv.Itoa(test.status) + `,"isRetryable":` +
				strconv.FormatBool(test.retryable) + `,"message":"must not escape","responseBody":"body-secret","headers":{"authorization":"header-secret"}}}}`
			writeExecutableFixture(
				t, executable,
				"#!/bin/sh\nprintf '%s\\n' '"+errorEvent+"'\nexit 1\n",
			)
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
			if !errors.Is(err, test.want) ||
				strings.Contains(err.Error(), "must not escape") ||
				strings.Contains(err.Error(), "body-secret") ||
				strings.Contains(err.Error(), "header-secret") {
				t.Fatalf("error=%v", err)
			}
			failure, ok := ConversationFailureDetails(err)
			if !ok || failure.Code != test.wantCode ||
				failure.HTTPStatus != test.status ||
				failure.Retryable != test.retryable {
				t.Fatalf("failure=%#v, %v", failure, ok)
			}
		})
	}
}
