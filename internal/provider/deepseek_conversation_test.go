package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleConversationRunsBoundedControlToolLoop(t *testing.T) {
	calls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		if !strings.Contains(payload, `"name":"loom_missions_create_preview"`) ||
			!strings.Contains(payload, "Frozen Loom conversation control tools") ||
			strings.Contains(payload, "private-test-key") {
			t.Fatalf("control request[%d] = %s", calls, payload)
		}
		responseBody := `{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Preparing the review.","tool_calls":[{"id":"call-1","type":"function","function":{"name":"loom_missions_create_preview","arguments":"{\"objective\":\"Ship the governed release\"}"}}]},"finish_reason":"tool_calls"}]}`
		if calls == 2 {
			if !strings.Contains(payload, `"role":"tool"`) ||
				!strings.Contains(payload, `"tool_call_id":"call-1"`) ||
				!strings.Contains(payload, `\"requires_confirmation\":true`) ||
				strings.Contains(payload, "Preparing the review.") {
				t.Fatalf("tool result was not returned to Provider: %s", payload)
			}
			responseBody = `{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Mission proposal prepared."},"finish_reason":"stop"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(responseBody)),
			Request: request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var toolName string
	var arguments json.RawMessage
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
		}},
		func(_ context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
			toolName = name
			arguments = append(json.RawMessage(nil), input...)
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "Mission proposal prepared." || calls != 2 ||
		toolName != "loom_missions_create_preview" ||
		string(arguments) != `{"objective":"Ship the governed release"}` {
		t.Fatalf("response=%q calls=%d tool=%q args=%s error=%v", response, calls, toolName, arguments, err)
	}
}

func TestOpenAICompatibleConversationClassifiesToolCallFinishReasonDrift(t *testing.T) {
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-1","type":"function","function":{"name":"loom_missions_create_preview","arguments":"{\"objective\":\"Ship\"}"}}]},"finish_reason":"stop"}]}`,
			)),
			Request: request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	executed := false
	_, err = client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
		}},
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			executed = true
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || executed || failure.Code != "invalid_response" ||
		failure.ProviderCode != "response_tool_calls_finish_reason" {
		t.Fatalf("failure=%#v ok=%t executed=%t error=%v", failure, ok, executed, err)
	}
}

type deepSeekConversationDoer struct {
	request       *http.Request
	authorization string
	body          string
	model         string
	content       string
}

func (doer *deepSeekConversationDoer) Do(request *http.Request) (*http.Response, error) {
	doer.request = request
	doer.authorization = request.Header.Get("Authorization")
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	doer.body = string(body)
	model := doer.model
	if model == "" {
		model = DeepSeekConversationModelID
	}
	content := doer.content
	if content == "" {
		content = "Hello from DeepSeek"
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"` + model + `","choices":[{"message":{"role":"assistant","content":"` + content + `"}}]}`,
		)),
		Request: request,
	}, nil
}

func TestOpenAICompatibleConversationClientsUseFixedProviderContracts(t *testing.T) {
	tests := []struct {
		name             string
		providerID       string
		modelID          string
		endpoint         string
		newClient        func(OpenAICompatibleConversationConfig) (*DeepSeekConversationClient, error)
		wantTokenField   string
		forbidTokenField string
	}{
		{
			name: "Kimi", providerID: "kimi", modelID: KimiConversationModelID,
			endpoint: KimiConversationEndpoint, newClient: NewKimiConversationClient,
			wantTokenField: `"max_tokens":2048`, forbidTokenField: `"max_completion_tokens"`,
		},
		{
			name: "MiniMax", providerID: "minimax", modelID: MiniMaxConversationModelID,
			endpoint: MiniMaxConversationEndpoint, newClient: NewMiniMaxConversationClient,
			wantTokenField: `"max_completion_tokens":8192`, forbidTokenField: `"max_tokens"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := &deepSeekConversationDoer{
				model: test.modelID, content: "Hello from " + test.name,
			}
			client, err := test.newClient(OpenAICompatibleConversationConfig{
				Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Respond(
				context.Background(),
				[]ConversationMessage{{Role: "user", Content: "hello"}},
				[]byte("private-test-key"),
			)
			if err != nil {
				t.Fatal(err)
			}
			if response != "Hello from "+test.name || doer.request == nil ||
				doer.request.URL.String() != test.endpoint ||
				doer.authorization != "Bearer private-test-key" ||
				doer.request.Header.Get("Authorization") != "" ||
				!strings.Contains(doer.body, `"model":"`+test.modelID+`"`) ||
				!strings.Contains(doer.body, "provider="+test.providerID) ||
				!strings.Contains(doer.body, "model="+test.modelID) ||
				!strings.Contains(doer.body, "Do not infer or invent a different underlying model") ||
				strings.Contains(doer.body, "built on OpenAI") ||
				strings.Contains(doer.body, "GPT-series") ||
				!strings.Contains(doer.body, test.wantTokenField) ||
				strings.Contains(doer.body, test.forbidTokenField) ||
				strings.Contains(doer.body, "private-test-key") {
				t.Fatalf("response=%q request=%#v body=%q", response, doer.request, doer.body)
			}
		})
	}
}

func TestMiniMaxConversationSeparatesHiddenReasoningFromVisibleContent(t *testing.T) {
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(string(body), `"reasoning_split":true`) {
			t.Fatalf("MiniMax request did not separate reasoning: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","reasoning_content":"private reasoning must not escape","content":"VISIBLE-ONLY"}}]}`,
			)),
			Request: request,
		}, nil
	})
	client, err := NewMiniMaxConversationClient(OpenAICompatibleConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
	)
	if err != nil || response != "VISIBLE-ONLY" || strings.Contains(response, "private reasoning") {
		t.Fatalf("response=%q error=%v", response, err)
	}
}

func TestMiniMaxConversationStripsInlineHiddenReasoningFromContent(t *testing.T) {
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"<think>private reasoning must not escape</think>\nVISIBLE-ONLY"}}]}`,
			)),
			Request: request,
		}, nil
	})
	client, err := NewMiniMaxConversationClient(OpenAICompatibleConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
	)
	if err != nil || response != "VISIBLE-ONLY" || strings.Contains(response, "private reasoning") {
		t.Fatalf("response=%q error=%v", response, err)
	}
}

func TestMiniMaxConversationRejectsUnterminatedInlineHiddenReasoning(t *testing.T) {
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"<think>private reasoning must not escape"}}]}`,
			)),
			Request: request,
		}, nil
	})
	client, err := NewMiniMaxConversationClient(OpenAICompatibleConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
	)
	if response != "" || err == nil {
		t.Fatalf("response=%q error=%v", response, err)
	}
}

func TestDeepSeekReasoningUsesExpandedBoundedCompletionBudget(t *testing.T) {
	doer := &deepSeekConversationDoer{}
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.RespondConfigured(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
		"deepseek-v4-flash",
		"high",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doer.body, `"max_tokens":8192`) ||
		!strings.Contains(doer.body, `"reasoning_effort":"high"`) {
		t.Fatalf("reasoning request does not carry the bounded expanded budget: %s", doer.body)
	}
}

func TestDeepSeekConversationUsesFixedBoundedNonStreamingRequest(t *testing.T) {
	doer := &deepSeekConversationDoer{}
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("private-test-key")
	response, err := client.Respond(context.Background(), []ConversationMessage{
		{Role: "user", Content: "hello"},
	}, secret)
	if err != nil {
		t.Fatal(err)
	}
	if response != "Hello from DeepSeek" || doer.request == nil ||
		doer.request.Method != http.MethodPost ||
		doer.request.URL.String() != "https://api.deepseek.com/chat/completions" ||
		doer.authorization != "Bearer private-test-key" ||
		!strings.Contains(doer.body, `"model":"deepseek-chat"`) ||
		!strings.Contains(doer.body, "provider=deepseek") ||
		!strings.Contains(doer.body, "model=deepseek-chat") ||
		!strings.Contains(doer.body, "Do not infer or invent a different underlying model") ||
		strings.Contains(doer.body, "built on OpenAI") ||
		strings.Contains(doer.body, "GPT-series") ||
		!strings.Contains(doer.body, `"stream":false`) ||
		strings.Contains(doer.body, "private-test-key") {
		t.Fatalf("response=%q request=%#v body=%q", response, doer.request, doer.body)
	}
}

func TestDeepSeekConversationKeepsLoomContextInSystemRole(t *testing.T) {
	doer := &deepSeekConversationDoer{}
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Respond(context.Background(), []ConversationMessage{
		{Role: "system", Content: `{"kind":"loom_role_context","items":[]}`},
		{Role: "user", Content: "SESSION-A"},
	}, []byte("private-test-key"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doer.body, `Loom-owned context capsule`) ||
		!strings.Contains(doer.body, `"content":"SESSION-A"`) ||
		strings.Contains(doer.body, `"role":"user","content":"{\"kind\":\"loom_role_context`) {
		t.Fatalf("context/user role boundary missing: %s", doer.body)
	}
}

func TestOpenAICompatibleConversationAcceptsProviderResolvedModelAlias(t *testing.T) {
	doer := &deepSeekConversationDoer{
		model: "deepseek-chat-2026-08-01", content: "resolved model reply",
	}
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
	)
	if err != nil || response != "resolved model reply" {
		t.Fatalf("response=%q error=%v", response, err)
	}
}

func TestOpenAICompatibleConversationClassifiesSafeInvalidResponseReason(t *testing.T) {
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":null}}]}`,
				)),
				Request: request,
			}, nil
		}),
		Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.Code != "invalid_response" || failure.Stage != "provider_http" ||
		failure.HTTPStatus != http.StatusOK || failure.ProviderCode != "response_content" ||
		failure.Retryable {
		t.Fatalf("failure=%#v ok=%v error=%v", failure, ok, err)
	}
}

func TestOpenAICompatibleConversationClassifiesResponseBodyTimeout(t *testing.T) {
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(deepSeekConversationReaderFunc(func([]byte) (int, error) {
					return 0, context.DeadlineExceeded
				})),
				Request: request,
			}, nil
		}),
		Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-test-key"),
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.Stage != "provider_http" || failure.Code != "timeout" ||
		failure.UserMessage != "Provider connection timed out." || !failure.Retryable {
		t.Fatalf("failure=%#v ok=%v error=%v", failure, ok, err)
	}
}

func TestDeepSeekConversationRejectsRedirectAndOversizedResponse(t *testing.T) {
	client, err := NewSystemDeepSeekConversationClient(5*time.Second, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil || client.endpoint != "https://api.deepseek.com/chat/completions" {
		t.Fatalf("client = %#v", client)
	}
	oversized := &deepSeekConversationDoer{}
	oversizedClient, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 4097))),
				Request:    request,
			}, nil
		}),
		Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oversizedClient.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("key"),
	); err == nil {
		t.Fatal("oversized response accepted")
	}
	_ = oversized
}

func TestOpenAICompatibleConversationClassifiesSafeProviderFailures(t *testing.T) {
	tests := []struct {
		name              string
		status            int
		requestErr        error
		providerCode      string
		stage             string
		code              string
		userMessage       string
		retryable         bool
		retryAfterSeconds int64
	}{
		{name: "auth", status: http.StatusUnauthorized, providerCode: "invalid_api_key", stage: "provider_auth", code: "provider_auth", userMessage: "Provider rejected the selected account credential."},
		{name: "payment", status: http.StatusPaymentRequired, providerCode: "insufficient_balance", stage: "provider_http", code: "provider_insufficient_balance", userMessage: "Provider account has insufficient balance."},
		{name: "model", status: http.StatusNotFound, providerCode: "model_not_found", stage: "provider_http", code: "provider_model_unavailable", userMessage: "Selected model or Provider endpoint is unavailable."},
		{name: "invalid parameters", status: http.StatusUnprocessableEntity, providerCode: "invalid_parameter", stage: "provider_http", code: "provider_invalid_request", userMessage: "Provider rejected the request parameters."},
		{name: "rate limit", status: http.StatusTooManyRequests, providerCode: "rate_limit_exceeded", stage: "provider_rate_limit", code: "provider_rate_limit", userMessage: "Provider rate limit reached.", retryable: true, retryAfterSeconds: 18},
		{name: "server", status: http.StatusServiceUnavailable, providerCode: "server_error", stage: "provider_http", code: "provider_unavailable", userMessage: "Provider service is temporarily unavailable.", retryable: true},
		{name: "timeout", requestErr: context.DeadlineExceeded, stage: "provider_connect", code: "timeout", userMessage: "Provider connection timed out.", retryable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
				Client: deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
					if test.requestErr != nil {
						return nil, test.requestErr
					}
					response := &http.Response{
						StatusCode: test.status,
						Body: io.NopCloser(strings.NewReader(
							`{"error":{"code":"` + test.providerCode + `","message":"must never escape"}}`,
						)),
						Request: request,
						Header:  make(http.Header),
					}
					if test.retryAfterSeconds > 0 {
						response.Header.Set("Retry-After", "18")
					}
					return response, nil
				}),
				Timeout: time.Second, MaxResponseBytes: 4096,
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Respond(
				context.Background(),
				[]ConversationMessage{{Role: "user", Content: "hello"}},
				[]byte("private-test-key"),
			)
			failure, ok := ConversationFailureDetails(err)
			if !ok || failure.Stage != test.stage || failure.Code != test.code ||
				failure.Retryable != test.retryable || failure.HTTPStatus != test.status ||
				failure.ProviderCode != test.providerCode ||
				failure.UserMessage != test.userMessage ||
				failure.RetryAfterSeconds != test.retryAfterSeconds ||
				!errors.Is(err, ErrOpenAICompatibleConversationUnavailable) ||
				strings.Contains(err.Error(), "must never escape") {
				t.Fatalf("failure=%#v ok=%v error=%v", failure, ok, err)
			}
		})
	}
}

type deepSeekConversationDoerFunc func(*http.Request) (*http.Response, error)

func (do deepSeekConversationDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return do(request)
}

type deepSeekConversationReaderFunc func([]byte) (int, error)

func (read deepSeekConversationReaderFunc) Read(buffer []byte) (int, error) {
	return read(buffer)
}

// TestOpenAICompatibleConversationAcceptsContextCapsuleSizedMessage guards the
// regression where the Loom context capsule dispatch payload (allowed up to
// 32 KiB by the `context:loom-native:v1` adapter) grows with conversation
// history and exceeded the old 4096-byte per-message cap, making every later
// turn on a continuing thread fail with an opaque "conversation unavailable".
func TestOpenAICompatibleConversationAcceptsContextCapsuleSizedMessage(t *testing.T) {
	doer := &deepSeekConversationDoer{model: DeepSeekConversationModelID, content: "ok"}
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	capsule := strings.Repeat(`{"item":"content","payload":"x"}`, 700)
	if len(capsule) <= 4096 || len(capsule) > 32<<10 {
		t.Fatalf("capsule fixture size %d must be >4096 and <=32KiB", len(capsule))
	}
	if _, err := client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: capsule}},
		[]byte("private-test-key"),
	); err != nil {
		t.Fatalf("context-capsule-sized message rejected: %v", err)
	}
	if doer.request == nil || !strings.Contains(doer.body, "provider=deepseek") {
		t.Fatalf("request not dispatched: %#v", doer.request)
	}
}
