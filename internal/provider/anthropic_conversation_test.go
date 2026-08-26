package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
