package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type gatewayHTTPFixture struct {
	response *http.Response
	err      error
	requests int
	request  *http.Request
	body     []byte
	auth     string
	bearer   string
}

type gatewayTimeoutFixture struct{}

func (gatewayTimeoutFixture) Error() string { return "controlled upstream timeout" }
func (gatewayTimeoutFixture) Timeout() bool { return true }

type gatewayTimeoutBodyFixture struct{}

func (gatewayTimeoutBodyFixture) Read([]byte) (int, error) {
	return 0, gatewayTimeoutFixture{}
}
func (gatewayTimeoutBodyFixture) Close() error { return nil }

func TestAttemptGatewayCancellationRevokesListenerBeforeCallbackReturns(t *testing.T) {
	gateway, err := NewOpenAIAttemptGateway(AttemptGatewayConfig{
		Client: &gatewayHTTPFixture{}, Random: bytes.NewReader(make([]byte, 32)),
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	leaseReady := make(chan AttemptGatewayLease, 1)
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- gateway.WithCredential(
			ctx, CodexProviderID, CodexModelID, []byte("private-openai-key"),
			AttemptProviderToolPolicy{},
			func(lease AttemptGatewayLease) error {
				leaseReady <- lease
				<-release
				return nil
			},
		)
	}()
	lease := <-leaseReady
	address := strings.TrimPrefix(lease.BaseURL, "http://")
	connection, err := net.DialTimeout("tcp4", address, time.Second)
	if err != nil {
		t.Fatalf("gateway did not listen before cancellation: %v", err)
	}
	_ = connection.Close()
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		connection, dialErr := net.DialTimeout("tcp4", address, 50*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		if dialErr != nil {
			break
		}
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("Attempt gateway listener survived context cancellation")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("WithCredential cancellation=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Attempt gateway callback did not finish")
	}
}

func (fixture *gatewayHTTPFixture) Do(request *http.Request) (*http.Response, error) {
	fixture.requests++
	fixture.request = request
	fixture.auth = request.Header.Get("x-api-key")
	fixture.bearer = request.Header.Get("Authorization")
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	fixture.body = body
	if fixture.response != nil {
		fixture.response.Request = request
	}
	return fixture.response, fixture.err
}

func TestAttemptGatewayRejectsProviderNativeToolsBeforeUpstream(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		modelID    string
		path       string
		authHeader string
		authPrefix string
		body       string
		policy     []string
		newGateway func(AttemptGatewayConfig) (AttemptCredentialGateway, error)
	}{
		{
			name: "codex shell", providerID: CodexProviderID, modelID: CodexModelID,
			path: "/v1/responses", authHeader: "Authorization", authPrefix: "Bearer ",
			body:       `{"model":"gpt-5.5-codex","tools":[{"type":"shell"}],"input":[]}`,
			policy:     []string{"loom_grep_files", "loom_read_context", "loom_read_file"},
			newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "codex dynamic tool search", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			body:   `{"model":"gpt-5.5-codex","tools":[{"type":"tool_search"}],"input":[]}`,
			policy: []string{"loom_read_context"}, newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "codex duplicate model", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			body:   `{"model":"substituted","model":"gpt-5.5-codex","tools":[],"input":[]}`,
			policy: []string{"loom_read_context"}, newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "codex duplicate tools", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			body:   `{"model":"gpt-5.5-codex","tools":[{"type":"shell"}],"tools":[],"input":[]}`,
			policy: []string{"loom_read_context"}, newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "codex duplicate tool type", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			body:   `{"model":"gpt-5.5-codex","tools":[{"type":"shell","type":"function","namespace":"mcp__loom_context","name":"loom_read_context"}],"input":[]}`,
			policy: []string{"loom_read_context"}, newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "claude read", providerID: ClaudeCodeProviderID, modelID: ClaudeCodeModelID,
			path: "/v1/messages", authHeader: "x-api-key",
			body:       `{"model":"claude-sonnet-5","tools":[{"name":"Read"}],"messages":[]}`,
			policy:     []string{"loom_grep_files", "loom_read_context", "loom_read_file"},
			newGateway: NewAnthropicAttemptGateway,
		},
		{
			name: "codex unapproved Loom MCP tool", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			body:   `{"model":"gpt-5.5-codex","tools":[{"type":"function","namespace":"mcp__loom_context","name":"loom_read_file"}],"input":[]}`,
			policy: []string{"loom_read_context"}, newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "claude mixed governed and native tools", providerID: ClaudeCodeProviderID,
			modelID: ClaudeCodeModelID, path: "/v1/messages", authHeader: "x-api-key",
			body:   `{"model":"claude-sonnet-5","tools":[{"name":"mcp__loom_context__loom_read_context"},{"name":"Read"}],"messages":[]}`,
			policy: []string{"loom_read_context"}, newGateway: NewAnthropicAttemptGateway,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := &gatewayHTTPFixture{}
			gateway, err := test.newGateway(AttemptGatewayConfig{
				Client: doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = gateway.WithCredential(
				context.Background(), test.providerID, test.modelID,
				[]byte("private-provider-key"),
				AttemptProviderToolPolicy{AllowedLoomTools: test.policy},
				func(lease AttemptGatewayLease) error {
					request, requestErr := http.NewRequest(
						http.MethodPost, lease.BaseURL+test.path, strings.NewReader(test.body),
					)
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set(test.authHeader, test.authPrefix+lease.Token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if !errors.Is(err, ErrHarnessProtocol) || doer.requests != 0 {
				t.Fatalf("error=%v provider_requests=%d", err, doer.requests)
			}
		})
	}
}

func TestAttemptGatewayForwardsOnlyFrozenLoomTools(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		modelID    string
		path       string
		authHeader string
		authPrefix string
		body       string
		response   *http.Response
		newGateway func(AttemptGatewayConfig) (AttemptCredentialGateway, error)
	}{
		{
			name: "codex namespaced function", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			body: `{"model":"gpt-5.5-codex","tools":[{"type":"function","namespace":"mcp__loom_context","name":"loom_read_context"}],"input":[]}`,
			response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			}, Body: io.NopCloser(strings.NewReader(
				"event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"namespace\":\"mcp__loom_context\",\"name\":\"loom_read_context\"}}\n\n",
			))},
			newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "claude prefixed MCP function", providerID: ClaudeCodeProviderID,
			modelID: ClaudeCodeModelID, path: "/v1/messages", authHeader: "x-api-key",
			body: `{"model":"claude-sonnet-5","tools":[{"name":"mcp__loom_context__loom_read_context"}],"messages":[]}`,
			response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{
				"Content-Type": []string{"application/json"},
			}, Body: io.NopCloser(strings.NewReader(
				`{"type":"message","content":[{"type":"tool_use","id":"allowed-1","name":"mcp__loom_context__loom_read_context","input":{}}]}`,
			))},
			newGateway: NewAnthropicAttemptGateway,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := &gatewayHTTPFixture{response: test.response}
			gateway, err := test.newGateway(AttemptGatewayConfig{
				Client: doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = gateway.WithCredential(
				context.Background(), test.providerID, test.modelID,
				[]byte("private-provider-key"),
				AttemptProviderToolPolicy{AllowedLoomTools: []string{"loom_read_context"}},
				func(lease AttemptGatewayLease) error {
					request, requestErr := http.NewRequest(
						http.MethodPost, lease.BaseURL+test.path, strings.NewReader(test.body),
					)
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set(test.authHeader, test.authPrefix+lease.Token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						_, _ = io.Copy(io.Discard, response.Body)
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if err != nil || doer.requests != 1 {
				t.Fatalf("error=%v provider_requests=%d", err, doer.requests)
			}
		})
	}
}

func TestAttemptGatewayRejectsProviderNativeToolOutputBeforeHarness(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		modelID    string
		path       string
		authHeader string
		authPrefix string
		request    string
		response   *http.Response
		newGateway func(AttemptGatewayConfig) (AttemptCredentialGateway, error)
	}{
		{
			name: "codex local shell call", providerID: CodexProviderID,
			modelID: CodexModelID, path: "/v1/responses",
			authHeader: "Authorization", authPrefix: "Bearer ",
			request: `{"model":"gpt-5.5-codex","input":[]}`,
			response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			}, Body: io.NopCloser(strings.NewReader(
				"event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"local_shell_call\",\"call_id\":\"native-1\",\"action\":{\"type\":\"exec\",\"command\":\"env\"}}}\n\n",
			))},
			newGateway: NewOpenAIAttemptGateway,
		},
		{
			name: "claude native read", providerID: ClaudeCodeProviderID,
			modelID: ClaudeCodeModelID, path: "/v1/messages", authHeader: "x-api-key",
			request: `{"model":"claude-sonnet-5","messages":[]}`,
			response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{
				"Content-Type": []string{"application/json"},
			}, Body: io.NopCloser(strings.NewReader(
				`{"type":"message","content":[{"type":"tool_use","id":"native-1","name":"Read","input":{"file_path":"/private"}}]}`,
			))},
			newGateway: NewAnthropicAttemptGateway,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := &gatewayHTTPFixture{response: test.response}
			gateway, err := test.newGateway(AttemptGatewayConfig{
				Client: doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			var harnessBody []byte
			err = gateway.WithCredential(
				context.Background(), test.providerID, test.modelID,
				[]byte("private-provider-key"),
				AttemptProviderToolPolicy{AllowedLoomTools: []string{"loom_read_context"}},
				func(lease AttemptGatewayLease) error {
					request, requestErr := http.NewRequest(
						http.MethodPost, lease.BaseURL+test.path, strings.NewReader(test.request),
					)
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set(test.authHeader, test.authPrefix+lease.Token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						harnessBody, _ = io.ReadAll(response.Body)
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if !errors.Is(err, ErrHarnessProtocol) || doer.requests != 1 ||
				bytes.Contains(harnessBody, []byte("local_shell_call")) ||
				bytes.Contains(harnessBody, []byte(`"name":"Read"`)) {
				t.Fatalf("error=%v provider_requests=%d harness_body=%s",
					err, doer.requests, harnessBody)
			}
		})
	}
}

func TestOpenAIAttemptGatewaySwapsBearerTokenAndFreezesResponsesModel(t *testing.T) {
	doer := &gatewayHTTPFixture{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n",
		)),
	}}
	gateway, err := NewOpenAIAttemptGateway(AttemptGatewayConfig{
		Client: doer, Random: bytes.NewReader(make([]byte, 32)),
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = gateway.WithCredential(
		context.Background(), CodexProviderID, CodexModelID,
		[]byte("private-openai-key"),
		AttemptProviderToolPolicy{},
		func(lease AttemptGatewayLease) error {
			request, requestErr := http.NewRequest(
				http.MethodPost,
				lease.BaseURL+"/v1/responses",
				strings.NewReader(`{"model":"gpt-5.5-codex","input":"bounded"}`),
			)
			if requestErr != nil {
				return requestErr
			}
			request.Header.Set("Authorization", "Bearer "+lease.Token)
			request.Header.Set("Content-Type", "application/json")
			response, requestErr := http.DefaultClient.Do(request)
			if requestErr == nil && response != nil {
				_, _ = io.Copy(io.Discard, response.Body)
				_ = response.Body.Close()
			}
			return requestErr
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if doer.requests != 1 || doer.request.URL.String() != CodexEndpoint ||
		doer.bearer != "Bearer private-openai-key" ||
		doer.request.Header.Get("Authorization") != "" ||
		!bytes.Contains(doer.body, []byte(`"model":"gpt-5.5-codex"`)) ||
		bytes.Contains(doer.body, []byte("private-openai-key")) {
		t.Fatalf("OpenAI upstream request=%#v bearer=%q body=%s", doer.request, doer.bearer, doer.body)
	}
}

func TestOpenAIAttemptGatewayRejectsAnthropicHealthProbe(t *testing.T) {
	doer := &gatewayHTTPFixture{}
	gateway, err := NewOpenAIAttemptGateway(AttemptGatewayConfig{
		Client: doer, Random: bytes.NewReader(make([]byte, 32)),
		MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = gateway.WithCredential(
		context.Background(), CodexProviderID, CodexModelID,
		[]byte("private-openai-key"),
		AttemptProviderToolPolicy{},
		func(lease AttemptGatewayLease) error {
			request, requestErr := http.NewRequest(http.MethodHead, lease.BaseURL+"/", nil)
			if requestErr != nil {
				return requestErr
			}
			response, requestErr := http.DefaultClient.Do(request)
			if requestErr == nil && response != nil {
				_ = response.Body.Close()
			}
			return requestErr
		},
	)
	if !errors.Is(err, ErrHarnessProtocol) || doer.requests != 0 {
		t.Fatalf("error=%v provider_requests=%d", err, doer.requests)
	}
}

func TestAnthropicAttemptGatewaySwapsOneTimeTokenForProviderCredential(t *testing.T) {
	doer := &gatewayHTTPFixture{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"type":"message","content":[]}`)),
	}}
	gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
		Client: doer,
		Random: bytes.NewReader([]byte{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
			16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
		}),
		MaxRequestBytes:  1 << 20,
		MaxResponseBytes: 1 << 20,
		MaxRequests:      8,
	})
	if err != nil {
		t.Fatal(err)
	}
	var lease AttemptGatewayLease
	err = gateway.WithCredential(
		context.Background(),
		ClaudeCodeProviderID,
		ClaudeCodeModelID,
		[]byte("private-anthropic-key"),
		AttemptProviderToolPolicy{},
		func(candidate AttemptGatewayLease) error {
			lease = candidate
			healthRequest, err := http.NewRequest(http.MethodHead, candidate.BaseURL+"/", nil)
			if err != nil {
				return err
			}
			healthResponse, err := http.DefaultClient.Do(healthRequest)
			if err != nil {
				return err
			}
			_ = healthResponse.Body.Close()
			if healthResponse.StatusCode != http.StatusNoContent {
				t.Fatalf("health response = %d", healthResponse.StatusCode)
			}
			request, err := http.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				candidate.BaseURL+"/v1/messages?beta=true",
				strings.NewReader(`{"model":"claude-sonnet-5","messages":[]}`),
			)
			if err != nil {
				return err
			}
			request.Header.Set("x-api-key", candidate.Token)
			request.Header.Set("anthropic-version", "2023-06-01")
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK ||
				string(body) != `{"type":"message","content":[]}` {
				t.Fatalf("gateway response = %d %s", response.StatusCode, body)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if lease.BaseURL == "" || lease.Token !=
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" ||
		strings.Contains(lease.BaseURL, "private-anthropic-key") {
		t.Fatalf("lease = %#v", lease)
	}
	if doer.requests != 1 || doer.request == nil ||
		doer.request.Method != http.MethodPost ||
		doer.request.URL.String() != ClaudeCodeEndpoint+"?beta=true" ||
		doer.auth != "private-anthropic-key" ||
		doer.request.Header.Get("x-api-key") != "" ||
		string(doer.body) != `{"model":"claude-sonnet-5","messages":[]}` ||
		bytes.Contains(doer.body, []byte("private-anthropic-key")) ||
		doer.request.Header.Get("anthropic-version") != "2023-06-01" {
		t.Fatalf("upstream = request %#v auth %q body %s", doer.request, doer.auth, doer.body)
	}
}

func TestAnthropicAttemptGatewayRejectsQueryDriftBeforeProvider(t *testing.T) {
	for _, query := range []string{"?beta=false", "?beta=true&beta=true", "?beta=true&other=1"} {
		t.Run(query, func(t *testing.T) {
			doer := &gatewayHTTPFixture{}
			gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
				Client: doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = gateway.WithCredential(
				context.Background(), ClaudeCodeProviderID, ClaudeCodeModelID,
				[]byte("private-anthropic-key"),
				AttemptProviderToolPolicy{},
				func(lease AttemptGatewayLease) error {
					request, requestErr := http.NewRequest(http.MethodPost,
						lease.BaseURL+"/v1/messages"+query,
						strings.NewReader(`{"model":"`+ClaudeCodeModelID+`","messages":[]}`))
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set("x-api-key", lease.Token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if !errors.Is(err, ErrHarnessProtocol) || doer.requests != 0 {
				t.Fatalf("error=%v provider_requests=%d", err, doer.requests)
			}
		})
	}
}

func TestAnthropicAttemptGatewayRejectsTokenAndModelDriftBeforeProvider(t *testing.T) {
	tests := []struct {
		name  string
		token string
		model string
	}{
		{name: "wrong token", token: "wrong-token", model: ClaudeCodeModelID},
		{name: "model drift", model: "claude-opus-5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := &gatewayHTTPFixture{}
			gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
				Client: doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = gateway.WithCredential(
				context.Background(), ClaudeCodeProviderID, ClaudeCodeModelID,
				[]byte("private-anthropic-key"),
				AttemptProviderToolPolicy{},
				func(lease AttemptGatewayLease) error {
					token := test.token
					if token == "" {
						token = lease.Token
					}
					request, requestErr := http.NewRequest(
						http.MethodPost,
						lease.BaseURL+"/v1/messages",
						strings.NewReader(`{"model":"`+test.model+`","messages":[]}`),
					)
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set("x-api-key", token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if !errors.Is(err, ErrHarnessProtocol) {
				t.Fatalf("WithCredential() error = %v", err)
			}
			if doer.requests != 0 {
				t.Fatalf("rejected request reached Provider %d times", doer.requests)
			}
		})
	}
}

func TestAnthropicAttemptGatewayClassifiesProviderFailure(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{status: http.StatusUnauthorized, want: ErrHarnessProviderAuth},
		{status: http.StatusForbidden, want: ErrHarnessProviderAuth},
		{status: http.StatusTooManyRequests, want: ErrHarnessProviderRateLimit},
		{status: http.StatusBadRequest, want: ErrHarnessProviderRejected},
		{status: http.StatusServiceUnavailable, want: ErrHarnessProviderUnavailable},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			doer := &gatewayHTTPFixture{response: &http.Response{
				StatusCode: test.status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"message":"raw Provider response must not escape"}}`,
				)),
			}}
			gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
				Client: doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = gateway.WithCredential(
				context.Background(), ClaudeCodeProviderID, ClaudeCodeModelID,
				[]byte("private-anthropic-key"),
				AttemptProviderToolPolicy{},
				func(lease AttemptGatewayLease) error {
					request, requestErr := http.NewRequest(
						http.MethodPost,
						lease.BaseURL+"/v1/messages",
						strings.NewReader(`{"model":"claude-sonnet-5","messages":[]}`),
					)
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set("x-api-key", lease.Token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						_, _ = io.Copy(io.Discard, response.Body)
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("WithCredential() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestAnthropicAttemptGatewayClassifiesProviderTimeout(t *testing.T) {
	tests := []struct {
		name string
		doer *gatewayHTTPFixture
		want error
	}{
		{
			name: "request", doer: &gatewayHTTPFixture{err: gatewayTimeoutFixture{}},
			want: ErrHarnessProviderTimeout,
		},
		{name: "response body", doer: &gatewayHTTPFixture{response: &http.Response{
			StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: gatewayTimeoutBodyFixture{},
		}}, want: ErrHarnessProviderResponseTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
				Client: test.doer, Random: bytes.NewReader(make([]byte, 32)),
				MaxRequestBytes: 1 << 20, MaxResponseBytes: 1 << 20, MaxRequests: 8,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = gateway.WithCredential(
				context.Background(), ClaudeCodeProviderID, ClaudeCodeModelID,
				[]byte("private-anthropic-key"),
				AttemptProviderToolPolicy{},
				func(lease AttemptGatewayLease) error {
					request, requestErr := http.NewRequest(
						http.MethodPost,
						lease.BaseURL+"/v1/messages",
						strings.NewReader(`{"model":"claude-sonnet-5","messages":[]}`),
					)
					if requestErr != nil {
						return requestErr
					}
					request.Header.Set("x-api-key", lease.Token)
					request.Header.Set("Content-Type", "application/json")
					response, requestErr := http.DefaultClient.Do(request)
					if requestErr == nil && response != nil {
						_ = response.Body.Close()
					}
					return requestErr
				},
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("WithCredential() error = %v, want %v", err, test.want)
			}
		})
	}
}
