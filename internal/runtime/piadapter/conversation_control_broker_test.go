//go:build unix

package piadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPiConversationControlBrokerDirectReplyNeverCallsMCP(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		_ *http.Request,
	) {
		calls.Add(1)
		http.Error(response, "unexpected", http.StatusInternalServerError)
	}))
	defer server.Close()
	adapter := testPiConversationControlBrokerAdapter(t, server.URL+"/mcp", []PiRPCConversationControlTool{{
		Name: "loom_sessions_search", Description: "Search Loom conversations.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"}},"required":["query"]}`),
	}})
	tool, err := adapter.resolveConversationControlTool(
		`{"tool_name":"loom_conversation_reply"}`,
	)
	if err != nil || tool.Name != "loom_conversation_reply" || calls.Load() != 0 {
		t.Fatalf("direct selection = %#v, %v; MCP calls=%d", tool, err, calls.Load())
	}
}

func TestPiConversationControlBrokerRejectsCrossModeFieldsWithoutCallingMCP(t *testing.T) {
	adapter := testPiConversationControlBrokerAdapter(t, "http://127.0.0.1:1/mcp", []PiRPCConversationControlTool{{
		Name: "loom_sessions_search", Description: "Search Loom conversations.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"}},"required":["query"]}`),
	}})
	if _, err := adapter.resolveConversationControlTool(
		`{"tool_name":"loom_conversation_reply","arguments_json":"{}"}`,
	); !errors.Is(err, ErrPiConversationControl) {
		t.Fatalf("cross-mode selection error = %v", err)
	}
	direct, err := adapter.resolveConversationControlTool(
		`{"tool_name":"loom_conversation_reply"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.resolveConversationControlArguments(
		context.Background(), direct, `{}`, nil,
	); !errors.Is(err, ErrPiConversationControl) {
		t.Fatalf("direct argument stage error = %v", err)
	}
}

func TestPiConversationControlBrokerCallsExactMCPTool(t *testing.T) {
	type observedCall struct {
		name          string
		arguments     string
		authorization string
	}
	observed := make(chan observedCall, 1)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		calls.Add(1)
		var envelope struct {
			Params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(request.Body).Decode(&envelope)
		observed <- observedCall{
			name: envelope.Params.Name, arguments: string(envelope.Params.Arguments),
			authorization: request.Header.Get("Authorization"),
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response,
			`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"Proposal ready."}],"structuredContent":{"requires_confirmation":true},"isError":false}}`)
	}))
	defer server.Close()
	adapter := testPiConversationControlBrokerAdapter(t, server.URL+"/mcp", []PiRPCConversationControlTool{{
		Name: "loom_missions_create_preview", Description: "Prepare Mission review.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"objective":{"type":"string"}},"required":["objective"]}`),
	}})
	tool, err := adapter.resolveConversationControlTool(
		`{"tool_name":"loom_missions_create_preview"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := adapter.resolveConversationControlArguments(
		context.Background(), tool, `{"objective":"Ship Phase 7"}`, nil,
	)
	if err != nil || response.Content != "I prepared a Loom proposal for your review." {
		t.Fatalf("proposal response = %#v, %v", response, err)
	}
	call := <-observed
	if call.name != "loom_missions_create_preview" ||
		call.arguments != `{"objective":"Ship Phase 7"}` ||
		call.authorization != "Bearer "+strings.Repeat("d", 64) ||
		calls.Load() != 1 {
		t.Fatalf("MCP call name=%q arguments=%q authorization_valid=%t",
			call.name, call.arguments, call.authorization == "Bearer "+strings.Repeat("d", 64))
	}
}

func TestPiConversationControlBrokerHonorsCallerDeadline(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(
		_ http.ResponseWriter,
		_ *http.Request,
	) {
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})
	adapter := testPiConversationControlBrokerAdapter(t, server.URL+"/mcp", []PiRPCConversationControlTool{{
		Name: "loom_sessions_search", Description: "Search Loom conversations.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"}},"required":["query"]}`),
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	tool, err := adapter.resolveConversationControlTool(
		`{"tool_name":"loom_sessions_search"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.resolveConversationControlArguments(
		ctx, tool, `{"query":"Phase 7"}`, nil,
	)
	if !errors.Is(err, ErrPiConversationControl) || time.Since(started) > time.Second {
		t.Fatalf("deadline error = %v elapsed=%s", err, time.Since(started))
	}
}

func TestPiConversationControlBrokerRejectsUntrustedSelectionWithoutDisclosure(t *testing.T) {
	adapter := testPiConversationControlBrokerAdapter(t, "http://127.0.0.1:1/mcp", []PiRPCConversationControlTool{{
		Name: "loom_sessions_search", Description: "Search Loom conversations.",
		InputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"}},"required":["query"]}`),
	}})
	secretText := "loom_private_project_token"
	_, err := adapter.resolveConversationControlTool(
		`{"tool_name":"` + secretText + `"}`,
	)
	if err == nil || !strings.Contains(err.Error(), "selection_tool") ||
		strings.Contains(err.Error(), secretText) {
		t.Fatalf("selection error disclosed content or lost stage: %v", err)
	}
}

func testPiConversationControlBrokerAdapter(
	t *testing.T,
	url string,
	tools []PiRPCConversationControlTool,
) *PiRPCConversationAdapter {
	t.Helper()
	prepared, err := preparePiConversationControlConfig(PiRPCConversationControlConfig{
		URL: url, Token: strings.Repeat("d", 64), Tools: tools,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &PiRPCConversationAdapter{
		bridge:  &piRPCBridgeAdapter{maxAssistantBytes: 4_096},
		control: &prepared,
	}
}
