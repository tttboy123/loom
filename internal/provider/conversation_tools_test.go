package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

var conversationControlTestTools = []ConversationControlTool{{
	Name:        "loom_sessions_search",
	Description: "Search frozen Loom Conversation metadata.",
	InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
}}

func TestOpenAICompatibleConversationControlFailsClosed(t *testing.T) {
	tests := []struct {
		name          string
		response      func(int) string
		execute       ConversationControlExecutor
		wantHTTPCalls int
		wantExecCalls int
	}{
		{
			name: "unknown tool",
			response: func(int) string {
				return openAIControlCallResponse("call-1", "shell")
			},
			wantHTTPCalls: 1,
		},
		{
			name: "duplicate call id across rounds",
			response: func(int) string {
				return openAIControlCallResponse("call-1", "loom_sessions_search")
			},
			execute: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"matches":[]}`), nil
			},
			wantHTTPCalls: 2,
			wantExecCalls: 1,
		},
		{
			name: "invalid tool result",
			response: func(int) string {
				return openAIControlCallResponse("call-1", "loom_sessions_search")
			},
			execute: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`[]`), nil
			},
			wantHTTPCalls: 1,
			wantExecCalls: 1,
		},
		{
			name: "terminal arbitration rejects omitted read loop",
			response: func(call int) string {
				return openAIControlCallResponse(
					fmt.Sprintf("call-%d", call), "loom_sessions_search",
				)
			},
			execute: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"matches":[]}`), nil
			},
			wantHTTPCalls: 5,
			wantExecCalls: 2,
		},
		{
			name: "duplicate response key",
			response: func(int) string {
				return `{"model":"deepseek-chat","model":"deepseek-chat","choices":[]}`
			},
			wantHTTPCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpCalls := 0
			execCalls := 0
			doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
				httpCalls++
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(test.response(httpCalls))),
					Request:    request,
				}, nil
			})
			client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
				Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			execute := test.execute
			if execute == nil {
				execute = func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage(`{"matches":[]}`), nil
				}
			}
			_, err = client.RespondConfiguredWithTools(
				context.Background(),
				[]ConversationMessage{{Role: "user", Content: "Inspect Loom."}},
				[]byte("private-test-key"), DeepSeekConversationModelID, "",
				conversationControlTestTools,
				func(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
					execCalls++
					return execute(ctx, name, input)
				},
			)
			if err == nil || httpCalls != test.wantHTTPCalls || execCalls != test.wantExecCalls {
				t.Fatalf("http=%d execute=%d error=%v", httpCalls, execCalls, err)
			}
		})
	}
}

func TestOpenAICompatibleConversationControlLetsModelCorrectInvalidArguments(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := openAIControlCallResponse("call-1", "loom_sessions_search")
		switch httpCalls {
		case 2:
			if !strings.Contains(payload, `"role":"tool"`) ||
				!strings.Contains(payload, `\"code\":\"invalid_request\"`) ||
				!strings.Contains(payload, `\"retryable\":true`) ||
				strings.Contains(payload, "private invalid arguments") {
				t.Fatalf("safe correction result missing: %s", payload)
			}
			responseBody = openAIControlCallResponse("call-2", "loom_sessions_search")
		case 3:
			if !strings.Contains(payload, `\"matches\":[]`) {
				t.Fatalf("successful corrected result missing: %s", payload)
			}
			responseBody = `{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Corrected."},"finish_reason":"stop"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Inspect Loom."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			if execCalls == 1 {
				return nil, conversationControlFailureFixture{
					code: "invalid_request", message: "private invalid arguments",
				}
			}
			return json.RawMessage(`{"matches":[]}`), nil
		},
	)
	if err != nil || response != "Corrected." || httpCalls != 3 || execCalls != 2 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlCorrectsOmittedFrozenToolSelection(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		responseBody := openAIControlCallResponse(
			fmt.Sprintf("call-search-%d", httpCalls), "loom_sessions_search",
		)
		if httpCalls == 4 {
			payload := string(body)
			if !strings.Contains(payload, `tool availability correction`) ||
				strings.Contains(payload, `"name":"loom_sessions_search"`) ||
				!strings.Contains(payload, `"name":"loom_missions_create_preview"`) {
				t.Fatalf("availability correction was not least privilege: %s", payload)
			}
			responseBody = openAIControlCallResponse(
				"call-mission", "loom_missions_create_preview",
			)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		parallelConversationControlTestTools(),
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			execCalls++
			switch name {
			case "loom_sessions_search":
				return json.RawMessage(`{"matches":[]}`), nil
			case "loom_missions_create_preview":
				return json.RawMessage(`{"requires_confirmation":true}`), nil
			default:
				return nil, errors.New("availability correction escaped frozen tools")
			}
		},
	)
	if err != nil || response != "" || httpCalls != 4 || execCalls != 3 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlRepairsInvalidProposalWithOnlySelectedTool(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := openAIControlCallResponse(
			"call-mission-1", "loom_missions_create_preview",
		)
		if httpCalls == 2 {
			if !strings.Contains(payload, `"tool_choice":"required"`) ||
				!strings.Contains(payload, `argument repair`) ||
				!strings.Contains(payload, `\"required_fields\":[\"objective\"]`) ||
				!strings.Contains(payload, `"name":"loom_missions_create_preview"`) ||
				strings.Contains(payload, `"name":"loom_sessions_search"`) ||
				strings.Contains(payload, `"name":"loom_conversation_reply"`) {
				t.Fatalf("proposal argument repair was not least privilege: %s", payload)
			}
			responseBody = openAIControlCallResponseWithArguments(
				"call-mission-2", "loom_missions_create_preview",
				`{"objective":"Repair this Mission"}`,
			)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		[]ConversationControlTool{
			conversationControlTestTools[0],
			{
				Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
				InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
				StopAfterSuccess: true,
			},
		},
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			execCalls++
			if name != "loom_missions_create_preview" {
				return nil, errors.New("repair escaped selected proposal tool")
			}
			if execCalls == 1 {
				return nil, conversationControlFailureFixture{
					code: "invalid_request", message: "private invalid arguments",
				}
			}
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 2 || execCalls != 2 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlReselectsAfterArgumentRepairLimit(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := openAIControlCallResponse(
			fmt.Sprintf("call-mission-%d", httpCalls),
			"loom_missions_create_preview",
		)
		if httpCalls == 4 {
			var requestBody struct {
				Messages []json.RawMessage `json:"messages"`
				Tools    []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tools"`
			}
			if err := json.Unmarshal(body, &requestBody); err != nil {
				t.Fatal(err)
			}
			available := make(map[string]struct{}, len(requestBody.Tools))
			for _, tool := range requestBody.Tools {
				available[tool.Function.Name] = struct{}{}
			}
			_, hasTeam := available["loom_teams_create_preview"]
			_, hasDirect := available["loom_conversation_reply"]
			_, hasMission := available["loom_missions_create_preview"]
			_, hasSearch := available["loom_sessions_search"]
			if !strings.Contains(payload, `tool reselection`) ||
				len(requestBody.Messages) != 2 ||
				!hasTeam || !hasDirect || hasMission || hasSearch {
				t.Fatalf("repair-exhausted tool was not excluded: %s", payload)
			}
			responseBody = openAIControlCallResponseWithArguments(
				"call-team", "loom_teams_create_preview",
				`{"purpose":"Govern this task"}`,
			)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	tools := []ConversationControlTool{
		conversationControlTestTools[0],
		{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
			StopAfterSuccess: true,
		},
		{
			Name: "loom_teams_create_preview", Description: "Prepare Team review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"purpose":{"type":"string"}},"required":["purpose"]}`),
			StopAfterSuccess: true,
		},
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Team proposal."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "", tools,
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			execCalls++
			if name == "loom_missions_create_preview" {
				return nil, conversationControlFailureFixture{
					code: "invalid_request", message: "private invalid arguments",
				}
			}
			if name != "loom_teams_create_preview" {
				return nil, errors.New("reselection escaped proposal tools")
			}
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 4 || execCalls != 4 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlClassifiesTerminalToolFailure(t *testing.T) {
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				openAIControlCallResponse("call-1", "loom_sessions_search"),
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
	_, err = client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Inspect Loom."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return nil, conversationControlFailureFixture{
				code: "tool_unavailable", message: "private tool failure",
			}
		},
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.Code != "invalid_response" ||
		failure.Stage != "conversation_dispatch" ||
		failure.ProviderCode != "control_tool_tool_unavailable" || failure.Retryable ||
		errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "private tool failure") {
		t.Fatalf("failure=%#v ok=%t error=%v", failure, ok, err)
	}
}

func TestOpenAICompatibleConversationControlStopsAfterSuccessfulProposal(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				openAIControlCallResponse("call-1", "loom_missions_create_preview"),
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
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		}},
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 1 || execCalls != 1 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlPrioritizesUniqueParallelProposal(t *testing.T) {
	names := make([]string, MaxConversationControlCalls+1)
	for index := 0; index < len(names)-1; index++ {
		names[index] = "loom_sessions_search"
	}
	names[len(names)-1] = "loom_missions_create_preview"
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(openAIParallelControlCallResponse(names))),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 32 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	executed := ""
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		parallelConversationControlTestTools(),
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			executed = name
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || executed != "loom_missions_create_preview" {
		t.Fatalf("response=%q executed=%q error=%v", response, executed, err)
	}
	_, err = selectOpenAITerminalConversationControlCall(
		[]openAIConversationControlCall{
			{Function: openAIConversationControlFunction{Name: "loom_missions_create_preview"}},
			{Function: openAIConversationControlFunction{Name: "loom_missions_create_preview"}},
		},
		map[string]ConversationControlTool{
			"loom_missions_create_preview": {StopAfterSuccess: true},
		},
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.ProviderCode != "response_terminal_tool_calls" {
		t.Fatalf("ambiguous terminal failure=%#v ok=%t error=%v", failure, ok, err)
	}
}

func TestOpenAICompatibleConversationControlRequiresTypedSelectionAndSupportsDirectReply(t *testing.T) {
	httpCalls := 0
	execCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := openAIControlCallResponse(
			"call-direct-1", "loom_conversation_reply",
		)
		switch httpCalls {
		case 1:
			if !strings.Contains(payload, `"tool_choice":"required"`) ||
				!strings.Contains(payload, `"name":"loom_conversation_reply"`) ||
				!strings.Contains(payload, `"required":["decision"]`) ||
				!strings.Contains(payload, `"no_product_tool_matches"`) {
				t.Fatalf("initial typed selection contract missing: %s", payload)
			}
		case 2:
			if !strings.Contains(payload, `"tools"`) ||
				!strings.Contains(payload, `"tool_choice":"required"`) ||
				!strings.Contains(payload, `"name":"loom_conversation_reply"`) ||
				strings.Contains(payload, `call-direct-1`) ||
				!strings.Contains(payload, `independent typed recheck`) {
				t.Fatalf("direct reply typed recheck is incomplete: %s", payload)
			}
			responseBody = openAIControlCallResponse(
				"call-direct-2", "loom_conversation_reply",
			)
		case 3:
			if strings.Contains(payload, `"tools"`) ||
				strings.Contains(payload, `"tool_choice"`) ||
				strings.Contains(payload, `call-direct-1`) ||
				strings.Contains(payload, `call-direct-2`) ||
				!strings.Contains(payload, `answer_request_directly`) {
				t.Fatalf("direct answer continuation retained product authority: %s", payload)
			}
			responseBody = `{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Direct answer."},"finish_reason":"stop"}]}`
		default:
			t.Fatalf("unexpected HTTP call %d", httpCalls)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Explain this without a product action."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			return nil, errors.New("direct reply must not enter product authority")
		},
	)
	if err != nil || response != "Direct answer." || httpCalls != 3 || execCalls != 0 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlRecoversPrematureDirectReply(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		responseBody := openAIControlCallResponse(
			"call-direct", "loom_conversation_reply",
		)
		if httpCalls == 2 {
			payload := string(body)
			if !strings.Contains(payload, `"tool_choice":"required"`) ||
				!strings.Contains(payload, `"name":"loom_conversation_reply"`) ||
				strings.Contains(payload, `call-direct`) ||
				!strings.Contains(payload, `independent typed recheck`) {
				t.Fatalf("premature direct reply was not rechecked: %s", payload)
			}
			responseBody = openAIControlCallResponse(
				"call-mission", "loom_missions_create_preview",
			)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	executed := ""
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		}},
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			executed = name
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 2 ||
		executed != "loom_missions_create_preview" {
		t.Fatalf("response=%q http=%d executed=%q error=%v", response, httpCalls, executed, err)
	}
}

func TestOpenAICompatibleConversationControlRecoversDirectReplyAfterReadPreflight(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := openAIControlCallResponse(
			"call-search", "loom_sessions_search",
		)
		switch httpCalls {
		case 1:
		case 2:
			if !strings.Contains(payload, `read-only or corrective`) {
				t.Fatalf("read preflight continuation rule missing: %s", payload)
			}
			responseBody = openAIControlCallResponse(
				"call-direct", "loom_conversation_reply",
			)
		case 3:
			if !strings.Contains(payload, `"tool_choice":"required"`) ||
				!strings.Contains(payload, `read-only or corrective`) ||
				!strings.Contains(payload, `independent typed recheck`) ||
				!strings.Contains(payload, `terminal tool arbitration`) ||
				strings.Contains(payload, `"description":"Search frozen Loom Conversation metadata."`) ||
				strings.Contains(payload, `call-direct`) {
				t.Fatalf("post-read direct reply was not independently rechecked: %s", payload)
			}
			responseBody = openAIControlCallResponse(
				"call-mission", "loom_missions_create_preview",
			)
		default:
			t.Fatalf("unexpected HTTP call %d", httpCalls)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	executed := make([]string, 0, 2)
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		parallelConversationControlTestTools(),
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			executed = append(executed, name)
			if name == "loom_missions_create_preview" {
				return json.RawMessage(`{"requires_confirmation":true}`), nil
			}
			return json.RawMessage(`{"matches":[]}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 3 ||
		!reflect.DeepEqual(executed, []string{
			"loom_sessions_search", "loom_missions_create_preview",
		}) {
		t.Fatalf("response=%q http=%d executed=%v error=%v", response, httpCalls, executed, err)
	}
}

func TestMiniMaxConversationControlSelectionUsesDeterministicSampling(t *testing.T) {
	executed := ""
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		if !strings.Contains(payload, `"tool_choice":"required"`) ||
			!strings.Contains(payload, `"temperature":0`) ||
			!strings.Contains(payload, `"seed":7`) {
			t.Fatalf("MiniMax selection is not deterministic: %s", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-mission","type":"function","function":{"name":"loom_missions_create_preview","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			)),
			Request: request,
		}, nil
	})
	client, err := NewMiniMaxConversationClient(OpenAICompatibleConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), MiniMaxConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		}},
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			executed = name
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || executed != "loom_missions_create_preview" {
		t.Fatalf("response=%q executed=%q error=%v", response, executed, err)
	}
}

func TestMiniMaxConversationControlCorrectsOneUntypedInitialSelection(t *testing.T) {
	httpCalls := 0
	execCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		if !strings.Contains(payload, `"tool_choice":"required"`) {
			t.Fatalf("required typed selection missing on call %d: %s", httpCalls, payload)
		}
		responseBody := `{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"I would create a Mission."},"finish_reason":"stop"}]}`
		if httpCalls == 2 {
			if !strings.Contains(payload, conversationRequiredToolSelectionInstruction) {
				t.Fatalf("bounded typed-selection correction missing: %s", payload)
			}
			responseBody = `{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-mission","type":"function","function":{"name":"loom_missions_create_preview","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewMiniMaxConversationClient(OpenAICompatibleConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), MiniMaxConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		}},
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			execCalls++
			if name != "loom_missions_create_preview" {
				t.Fatalf("unexpected Tool execution: %s", name)
			}
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 2 || execCalls != 1 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestMiniMaxConversationControlCorrectionGetsIndependentBoundedRequestBudget(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		delay := 55 * time.Millisecond
		if httpCalls == 2 {
			delay = 45 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-request.Context().Done():
			return nil, request.Context().Err()
		case <-timer.C:
		}
		responseBody := `{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"I would create a Mission."},"finish_reason":"stop"}]}`
		if httpCalls == 2 {
			responseBody = `{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-mission","type":"function","function":{"name":"loom_missions_create_preview","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewMiniMaxConversationClient(OpenAICompatibleConversationConfig{
		Client: doer, Timeout: 80 * time.Millisecond, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), MiniMaxConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		}},
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 2 || execCalls != 1 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestOpenAICompatibleConversationControlRejectsProseDuringTypedRecheck(t *testing.T) {
	httpCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		responseBody := openAIControlCallResponse(
			"call-direct", "loom_conversation_reply",
		)
		if httpCalls == 2 {
			responseBody = `{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Untyped answer."},"finish_reason":"stop"}]}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewDeepSeekConversationClient(DeepSeekConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return nil, errors.New("untyped recheck must not enter product authority")
		},
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.ProviderCode != "response_required_tool_choice" || httpCalls != 2 {
		t.Fatalf("typed recheck failure=%#v ok=%t calls=%d error=%v", failure, ok, httpCalls, err)
	}
}

func TestOpenAICompatibleConversationControlRejectsUntypedOrAmbiguousDirectReply(t *testing.T) {
	httpCalls := 0
	execCalls := 0
	doer := deepSeekConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"prose"},"finish_reason":"stop"}]}`,
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
	_, err = client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Inspect Loom."}},
		[]byte("private-test-key"), DeepSeekConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			return json.RawMessage(`{"matches":[]}`), nil
		},
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.ProviderCode != "response_required_tool_choice" ||
		httpCalls != 2 || execCalls != 0 {
		t.Fatalf("required selection failure=%#v ok=%t http=%d execute=%d error=%v", failure, ok, httpCalls, execCalls, err)
	}

	_, err = selectOpenAITerminalConversationControlCall(
		[]openAIConversationControlCall{
			{Function: openAIConversationControlFunction{Name: "loom_conversation_reply"}},
			{Function: openAIConversationControlFunction{Name: "loom_sessions_search"}},
		},
		map[string]ConversationControlTool{
			"loom_conversation_reply": {DirectReply: true},
			"loom_sessions_search":    {},
		},
	)
	failure, ok = ConversationFailureDetails(err)
	if !ok || failure.ProviderCode != "response_direct_reply_tool_calls" {
		t.Fatalf("ambiguous direct reply failure=%#v ok=%t error=%v", failure, ok, err)
	}
}

func TestAnthropicConversationControlFailsClosed(t *testing.T) {
	tests := []struct {
		name          string
		response      func(int) string
		execute       ConversationControlExecutor
		wantHTTPCalls int
		wantExecCalls int
	}{
		{
			name: "unknown tool",
			response: func(int) string {
				return anthropicControlCallResponse("toolu-1", "shell")
			},
			wantHTTPCalls: 1,
		},
		{
			name: "duplicate call id across rounds",
			response: func(int) string {
				return anthropicControlCallResponse("toolu-1", "loom_sessions_search")
			},
			execute: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"matches":[]}`), nil
			},
			wantHTTPCalls: 2,
			wantExecCalls: 1,
		},
		{
			name: "invalid tool result",
			response: func(int) string {
				return anthropicControlCallResponse("toolu-1", "loom_sessions_search")
			},
			execute: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`[]`), nil
			},
			wantHTTPCalls: 1,
			wantExecCalls: 1,
		},
		{
			name: "terminal arbitration rejects omitted read loop",
			response: func(call int) string {
				return anthropicControlCallResponse(
					fmt.Sprintf("toolu-%d", call), "loom_sessions_search",
				)
			},
			execute: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"matches":[]}`), nil
			},
			wantHTTPCalls: 5,
			wantExecCalls: 2,
		},
		{
			name: "duplicate response key",
			response: func(int) string {
				return `{"type":"message","type":"message","role":"assistant","model":"claude-sonnet-5","content":[],"stop_reason":"end_turn"}`
			},
			wantHTTPCalls: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpCalls := 0
			execCalls := 0
			doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
				httpCalls++
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(test.response(httpCalls))),
					Request:    request,
				}, nil
			})
			client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
				Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			execute := test.execute
			if execute == nil {
				execute = func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage(`{"matches":[]}`), nil
				}
			}
			_, err = client.RespondConfiguredWithTools(
				context.Background(),
				[]ConversationMessage{{Role: "user", Content: "Inspect Loom."}},
				[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
				conversationControlTestTools,
				func(ctx context.Context, name string, input json.RawMessage) (json.RawMessage, error) {
					execCalls++
					return execute(ctx, name, input)
				},
			)
			if err == nil || httpCalls != test.wantHTTPCalls || execCalls != test.wantExecCalls {
				t.Fatalf("http=%d execute=%d error=%v", httpCalls, execCalls, err)
			}
		})
	}
}

func TestAnthropicConversationControlMarksCorrectableToolError(t *testing.T) {
	httpCalls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := anthropicControlCallResponse("toolu-1", "loom_sessions_search")
		switch httpCalls {
		case 2:
			if !strings.Contains(payload, `"is_error":true`) ||
				!strings.Contains(payload, `\"code\":\"invalid_request\"`) ||
				strings.Contains(payload, "private invalid arguments") {
				t.Fatalf("safe Anthropic correction result missing: %s", payload)
			}
			responseBody = anthropicControlCallResponse("toolu-2", "loom_sessions_search")
		case 3:
			if !strings.Contains(payload, `\"matches\":[]`) {
				t.Fatalf("successful Anthropic result missing: %s", payload)
			}
			responseBody = `{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[{"type":"text","text":"Corrected."}],"stop_reason":"end_turn"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Inspect Loom."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			if execCalls == 1 {
				return nil, conversationControlFailureFixture{
					code: "invalid_request", message: "private invalid arguments",
				}
			}
			return json.RawMessage(`{"matches":[]}`), nil
		},
	)
	if err != nil || response != "Corrected." || httpCalls != 3 || execCalls != 2 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestAnthropicConversationControlCorrectsOmittedFrozenToolSelection(t *testing.T) {
	httpCalls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		responseBody := anthropicControlCallResponse(
			fmt.Sprintf("toolu-search-%d", httpCalls), "loom_sessions_search",
		)
		if httpCalls == 4 {
			payload := string(body)
			if !strings.Contains(payload, `tool availability correction`) ||
				strings.Contains(payload, `"name":"loom_sessions_search"`) ||
				!strings.Contains(payload, `"name":"loom_missions_create_preview"`) {
				t.Fatalf("Anthropic availability correction was not least privilege: %s", payload)
			}
			responseBody = anthropicControlCallResponse(
				"toolu-mission", "loom_missions_create_preview",
			)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		parallelConversationControlTestTools(),
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			execCalls++
			switch name {
			case "loom_sessions_search":
				return json.RawMessage(`{"matches":[]}`), nil
			case "loom_missions_create_preview":
				return json.RawMessage(`{"requires_confirmation":true}`), nil
			default:
				return nil, errors.New("availability correction escaped frozen tools")
			}
		},
	)
	if err != nil || response != "" || httpCalls != 4 || execCalls != 3 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestAnthropicConversationControlReselectsAfterArgumentRepairLimit(t *testing.T) {
	httpCalls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := anthropicControlCallResponse(
			fmt.Sprintf("toolu-mission-%d", httpCalls),
			"loom_missions_create_preview",
		)
		if httpCalls == 4 {
			var requestBody struct {
				Messages []json.RawMessage `json:"messages"`
				Tools    []struct {
					Name string `json:"name"`
				} `json:"tools"`
			}
			if err := json.Unmarshal(body, &requestBody); err != nil {
				t.Fatal(err)
			}
			available := make(map[string]struct{}, len(requestBody.Tools))
			for _, tool := range requestBody.Tools {
				available[tool.Name] = struct{}{}
			}
			_, hasTeam := available["loom_teams_create_preview"]
			_, hasDirect := available["loom_conversation_reply"]
			_, hasMission := available["loom_missions_create_preview"]
			_, hasSearch := available["loom_sessions_search"]
			if !strings.Contains(payload, `tool reselection`) ||
				len(requestBody.Messages) != 1 ||
				!hasTeam || !hasDirect || hasMission || hasSearch {
				t.Fatalf("repair-exhausted Anthropic tool was not excluded: %s", payload)
			}
			responseBody = anthropicControlCallResponseWithInput(
				"toolu-team", "loom_teams_create_preview",
				`{"purpose":"Govern this task"}`,
			)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	tools := []ConversationControlTool{
		conversationControlTestTools[0],
		{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
			StopAfterSuccess: true,
		},
		{
			Name: "loom_teams_create_preview", Description: "Prepare Team review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"purpose":{"type":"string"}},"required":["purpose"]}`),
			StopAfterSuccess: true,
		},
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Team proposal."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		tools,
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			execCalls++
			if name == "loom_missions_create_preview" {
				return nil, conversationControlFailureFixture{
					code: "invalid_request", message: "private invalid arguments",
				}
			}
			if name != "loom_teams_create_preview" {
				return nil, errors.New("reselection escaped proposal tools")
			}
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 4 || execCalls != 4 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestAnthropicConversationControlStopsAfterSuccessfulProposal(t *testing.T) {
	httpCalls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				anthropicControlCallResponse("toolu-1", "loom_missions_create_preview"),
			)),
			Request: request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	execCalls := 0
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		[]ConversationControlTool{{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		}},
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || httpCalls != 1 || execCalls != 1 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestAnthropicConversationControlPrioritizesUniqueParallelProposal(t *testing.T) {
	names := make([]string, MaxConversationControlCalls+1)
	for index := 0; index < len(names)-1; index++ {
		names[index] = "loom_sessions_search"
	}
	names[len(names)-1] = "loom_missions_create_preview"
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(anthropicParallelControlCallResponse(names))),
			Request:    request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 32 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	executed := ""
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		parallelConversationControlTestTools(),
		func(_ context.Context, name string, _ json.RawMessage) (json.RawMessage, error) {
			executed = name
			return json.RawMessage(`{"requires_confirmation":true}`), nil
		},
	)
	if err != nil || response != "" || executed != "loom_missions_create_preview" {
		t.Fatalf("response=%q executed=%q error=%v", response, executed, err)
	}
}

func TestAnthropicConversationControlRequiresTypedSelectionAndSupportsDirectReply(t *testing.T) {
	httpCalls := 0
	execCalls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		payload := string(body)
		responseBody := anthropicControlCallResponse(
			"toolu-direct-1", "loom_conversation_reply",
		)
		switch httpCalls {
		case 1:
			if !strings.Contains(payload, `"tool_choice":{"type":"any"}`) ||
				!strings.Contains(payload, `"name":"loom_conversation_reply"`) ||
				!strings.Contains(payload, `"required":["decision"]`) ||
				!strings.Contains(payload, `"no_product_tool_matches"`) {
				t.Fatalf("initial Anthropic typed selection contract missing: %s", payload)
			}
		case 2:
			if !strings.Contains(payload, `"tools"`) ||
				!strings.Contains(payload, `"tool_choice":{"type":"any"}`) ||
				!strings.Contains(payload, `"name":"loom_conversation_reply"`) ||
				strings.Contains(payload, `toolu-direct-1`) ||
				!strings.Contains(payload, `independent typed recheck`) {
				t.Fatalf("Anthropic direct reply typed recheck is incomplete: %s", payload)
			}
			responseBody = anthropicControlCallResponse(
				"toolu-direct-2", "loom_conversation_reply",
			)
		case 3:
			if strings.Contains(payload, `"tools"`) ||
				strings.Contains(payload, `"tool_choice"`) ||
				strings.Contains(payload, `toolu-direct-1`) ||
				strings.Contains(payload, `toolu-direct-2`) ||
				!strings.Contains(payload, `answer_request_directly`) {
				t.Fatalf("Anthropic direct answer retained product authority: %s", payload)
			}
			responseBody = `{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[{"type":"text","text":"Direct answer."}],"stop_reason":"end_turn"}`
		default:
			t.Fatalf("unexpected HTTP call %d", httpCalls)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Explain this without a product action."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			execCalls++
			return nil, errors.New("direct reply must not enter product authority")
		},
	)
	if err != nil || response != "Direct answer." || httpCalls != 3 || execCalls != 0 {
		t.Fatalf("response=%q http=%d execute=%d error=%v", response, httpCalls, execCalls, err)
	}
}

func TestAnthropicConversationControlRejectsProseDuringTypedRecheck(t *testing.T) {
	httpCalls := 0
	doer := anthropicConversationDoerFunc(func(request *http.Request) (*http.Response, error) {
		httpCalls++
		responseBody := anthropicControlCallResponse(
			"toolu-direct", "loom_conversation_reply",
		)
		if httpCalls == 2 {
			responseBody = `{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[{"type":"text","text":"Untyped answer."}],"stop_reason":"end_turn"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    request,
		}, nil
	})
	client, err := NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: doer, Timeout: time.Second, MaxResponseBytes: 16 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.RespondConfiguredWithTools(
		context.Background(),
		[]ConversationMessage{{Role: "user", Content: "Create a Mission proposal."}},
		[]byte("private-anthropic-key"), AnthropicConversationModelID, "",
		conversationControlTestTools,
		func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return nil, errors.New("untyped recheck must not enter product authority")
		},
	)
	failure, ok := ConversationFailureDetails(err)
	if !ok || failure.ProviderCode != "response_required_tool_choice" || httpCalls != 2 {
		t.Fatalf("typed recheck failure=%#v ok=%t calls=%d error=%v", failure, ok, httpCalls, err)
	}
}

func openAIControlCallResponse(callID string, name string) string {
	arguments := `{}`
	if name == "loom_conversation_reply" {
		arguments = `{"decision":"no_product_tool_matches"}`
	}
	return openAIControlCallResponseWithArguments(callID, name, arguments)
}

func openAIControlCallResponseWithArguments(
	callID string,
	name string,
	arguments string,
) string {
	return fmt.Sprintf(
		`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":%q,"type":"function","function":{"name":%q,"arguments":%q}}]},"finish_reason":"tool_calls"}]}`,
		callID, name, arguments,
	)
}

func anthropicControlCallResponse(callID string, name string) string {
	input := `{}`
	if name == "loom_conversation_reply" {
		input = `{"decision":"no_product_tool_matches"}`
	}
	return anthropicControlCallResponseWithInput(callID, name, input)
}

func anthropicControlCallResponseWithInput(callID string, name string, input string) string {
	return fmt.Sprintf(
		`{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[{"type":"tool_use","id":%q,"name":%q,"input":%s}],"stop_reason":"tool_use"}`,
		callID, name, input,
	)
}

func parallelConversationControlTestTools() []ConversationControlTool {
	return []ConversationControlTool{
		conversationControlTestTools[0],
		{
			Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
			InputSchema:      json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			StopAfterSuccess: true,
		},
	}
}

func openAIParallelControlCallResponse(names []string) string {
	calls := make([]string, len(names))
	for index, name := range names {
		calls[index] = fmt.Sprintf(
			`{"id":"call-%d","type":"function","function":{"name":%q,"arguments":"{}"}}`,
			index+1, name,
		)
	}
	return fmt.Sprintf(
		`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[%s]},"finish_reason":"tool_calls"}]}`,
		strings.Join(calls, ","),
	)
}

func anthropicParallelControlCallResponse(names []string) string {
	calls := make([]string, len(names))
	for index, name := range names {
		calls[index] = fmt.Sprintf(
			`{"type":"tool_use","id":"toolu-%d","name":%q,"input":{}}`,
			index+1, name,
		)
	}
	return fmt.Sprintf(
		`{"type":"message","role":"assistant","model":"claude-sonnet-5","content":[%s],"stop_reason":"tool_use"}`,
		strings.Join(calls, ","),
	)
}

type conversationControlFailureFixture struct {
	code    string
	message string
}

func (failure conversationControlFailureFixture) Error() string {
	return failure.message
}

func (failure conversationControlFailureFixture) ConversationControlFailureCode() string {
	return failure.code
}
