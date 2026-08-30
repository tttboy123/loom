package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAnthropicConversationRunsBoundedControlToolLoop(t *testing.T) {
	calls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		if !strings.Contains(payload, `"name":"loom_roundtables_open_preview"`) ||
			!strings.Contains(payload, "Frozen Loom conversation control tools") ||
			strings.Contains(payload, "private-anthropic-key") {
			t.Fatalf("Anthropic control request[%d] = %s", calls, payload)
		}
		responseBody := `{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[{"type":"tool_use","id":"toolu-1","name":"loom_roundtables_open_preview","input":{}}],"stop_reason":"tool_use"}`
		if calls == 2 {
			if !strings.Contains(payload, `"type":"tool_result"`) ||
				!strings.Contains(payload, `"tool_use_id":"toolu-1"`) ||
				!strings.Contains(payload, `\"requires_confirmation\":true`) {
				t.Fatalf("Anthropic tool result was not returned: %s", payload)
			}
			responseBody = `{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[{"type":"text","text":"RoundTable proposal prepared."}],"stop_reason":"end_turn"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(responseBody)),
			Request: request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	var toolName string
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Open the RoundTable."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_roundtables_open_preview", Description: "Prepare RoundTable review.",
			InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
		}},
		func(_ context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
			toolName = name
			if string(input) != `{}` {
				t.Fatalf("Anthropic tool input = %s", input)
			}
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "RoundTable proposal prepared." || calls != 2 ||
		toolName != "loom_roundtables_open_preview" {
		t.Fatalf("response=%q calls=%d tool=%q error=%v", response, calls, toolName, err)
	}
}

type anthropicConversationDoer struct {
	request *http.Request
	apiKey  string
	body    string
	model   string
	content string
}

func (doer *anthropicConversationDoer) Do(request *http.Request) (*http.Response, error) {
	doer.request = request
	doer.apiKey = request.Header.Get("x-api-key")
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	doer.body = string(body)
	model := doer.model
	if model == "" {
		model = AnthropicConversationModelID
	}
	content := doer.content
	if content == "" {
		content = "Hello from Anthropic"
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"type":"message","role":"assistant","model":"` + model +
				`","content":[{"type":"text","text":"` + content +
				`"}],"stop_reason":"end_turn"}`,
		)),
		Request: request,
	}, nil
}

func TestAnthropicConversationUsesFixedBoundedMessagesContract(t *testing.T) {
	doer := &anthropicConversationDoer{}
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Respond(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "hello"}},
		[]byte("private-anthropic-key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if response != "Hello from Anthropic" || doer.request == nil ||
		doer.request.Method != http.MethodPost ||
		doer.request.URL.String() != AnthropicConversationEndpoint ||
		doer.apiKey != "private-anthropic-key" ||
		doer.request.Header.Get("x-api-key") != "" ||
		doer.request.Header.Get("anthropic-version") != "2023-06-01" ||
		!strings.Contains(doer.body, `"model":"`+AnthropicConversationModelID+`"`) ||
		!strings.Contains(doer.body, "provider=anthropic") ||
		!strings.Contains(doer.body, "model="+AnthropicConversationModelID) ||
		!strings.Contains(doer.body, "Do not infer or invent a different underlying model") ||
		strings.Contains(doer.body, "built on OpenAI") ||
		strings.Contains(doer.body, "GPT-series") ||
		!strings.Contains(doer.body, `"max_tokens":2048`) ||
		!strings.Contains(doer.body, `"system":`) ||
		strings.Contains(doer.body, "private-anthropic-key") {
		t.Fatalf("response=%q request=%#v body=%q", response, doer.request, doer.body)
	}
}

func TestAnthropicConversationKeepsLoomContextInSystemField(t *testing.T) {
	doer := &anthropicConversationDoer{}
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Respond(context.Background(), []ConversationMessage{
		{Role: "system", Content: `{"kind":"loom_role_context","items":[]}`},
		{Role: "user", Content: "SESSION-A"},
	}, []byte("private-anthropic-key"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doer.body, `Loom-owned context capsule`) ||
		!strings.Contains(doer.body, `"content":"SESSION-A"`) ||
		strings.Contains(doer.body, `"role":"user","content":"{\"kind\":\"loom_role_context`) {
		t.Fatalf("context/user role boundary missing: %s", doer.body)
	}
}

func TestAnthropicConversationRejectsModelAndContentBlockDrift(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{
			name: "model",
			body: `{"type":"message","role":"assistant","model":"other-model",` +
				`"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn"}`,
		},
		{
			name: "tool block",
			body: `{"type":"message","role":"assistant","model":"` +
				AnthropicConversationModelID + `","content":[{"type":"tool_use",` +
				`"id":"tool-1","name":"shell","input":{}}],"stop_reason":"tool_use"}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
				Client: anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(test.body)),
						Request:    request,
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
				[]byte("private-anthropic-key"),
			)
			failure, ok := ConversationFailureDetails(err)
			if !ok || failure.Code != "invalid_response" || failure.Retryable {
				t.Fatalf("failure=%#v ok=%v error=%v", failure, ok, err)
			}
		})
	}
}

type anthropicConversationDoerFunc func(*http.Request) (*http.Response, error)

func (do anthropicConversationDoerFunc) Do(request *http.Request) (*http.Response, error) {
	return do(request)
}
