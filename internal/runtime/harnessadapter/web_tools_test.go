package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestHarnessMCPToolNamesIncludesWebTools(t *testing.T) {
	if names := harnessMCPToolNames(HarnessContextMCPLease{
		URL: "http://127.0.0.1:1/mcp", WebSearchEnabled: true, WebFetchEnabled: true,
	}); !reflect.DeepEqual(names, []string{"loom_web_search", "loom_web_fetch"}) {
		t.Fatalf("web-only names = %v", names)
	}
	names := harnessMCPToolNames(HarnessContextMCPLease{
		URL: "http://127.0.0.1:1/mcp", ContextEnabled: true,
		WebSearchEnabled: true, WebFetchEnabled: true,
	})
	for _, want := range []string{
		"loom_read_context", "loom_web_fetch", "loom_web_search",
	} {
		found := false
		for _, name := range names {
			if name == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("names = %v missing %q", names, want)
		}
	}
}

func TestHarnessDecodeWebToolCalls(t *testing.T) {
	search, ok := decodeHarnessToolCall(
		permissions.ToolWebSearch,
		json.RawMessage(`{"query":"Loom governed handoff"}`),
	)
	if !ok || search.Tool != permissions.ToolWebSearch || search.Path != "Loom governed handoff" {
		t.Fatalf("search = %#v ok=%t", search, ok)
	}
	fetch, ok := decodeHarnessToolCall(
		permissions.ToolWebFetch,
		json.RawMessage(`{"url":"https://example.com/"}`),
	)
	if !ok || fetch.Tool != permissions.ToolWebFetch || fetch.Path != "https://example.com/" {
		t.Fatalf("fetch = %#v ok=%t", fetch, ok)
	}
	if _, ok := decodeHarnessToolCall(permissions.ToolWebSearch, json.RawMessage(`{}`)); ok {
		t.Fatal("empty query accepted")
	}
	if _, ok := decodeHarnessToolCall(permissions.ToolWebFetch, json.RawMessage(`{"url":"x","extra":1}`)); ok {
		t.Fatal("unknown field accepted")
	}
}

func TestHarnessAttemptMCPPublishesAndExecutesWebTools(t *testing.T) {
	gateway := &harnessToolGatewayFixture{
		allowed: []permissions.ToolKind{
			permissions.ToolWebSearch, permissions.ToolWebFetch,
		},
		contents: map[permissions.ToolKind][]byte{
			permissions.ToolWebSearch: []byte("1. Loom docs https://example.com/loom bounded search\n"),
			permissions.ToolWebFetch:  []byte("bounded public page content\n"),
		},
	}
	binding := loomruntime.ToolCallBinding{
		ConversationID: "conversation-web", WorkItemID: "work-web",
		RunID: "run-web", ClaimGeneration: 3, RuntimeInstanceID: "runtime-web",
		AgentInstanceID: "agent-web", ExecutionBindingDigest: strings.Repeat("1", 64),
		CapsuleDigest: strings.Repeat("2", 64), ClaimID: "claim-web",
		IncidentID: "11111111-1111-4111-8111-111111111111",
		JourneyID:  "11111111-1111-4111-8111-111111111111",
	}
	service, err := newHarnessAttemptMCPWithContext(
		context.Background(), harnessAttemptMCPConfig{
			ToolGateway: gateway, ToolBinding: binding,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if tools := service.toolNames(); !reflect.DeepEqual(tools, []string{"loom_web_fetch", "loom_web_search"}) {
		t.Fatalf("published web tools = %v", tools)
	}
	calls := []struct {
		name      string
		arguments map[string]any
		content   []byte
	}{
		{
			name:      "loom_web_search",
			arguments: map[string]any{"query": "Loom governed handoff"},
			content:   gateway.contents[permissions.ToolWebSearch],
		},
		{
			name:      "loom_web_fetch",
			arguments: map[string]any{"url": "https://example.com/"},
			content:   gateway.contents[permissions.ToolWebFetch],
		},
	}
	for index, call := range calls {
		params, marshalErr := json.Marshal(map[string]any{
			"name": call.name, "arguments": call.arguments,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		response, status := service.call(
			context.Background(), json.RawMessage(strconv.Itoa(index+1)), params,
		)
		if status != http.StatusOK || !bytes.Contains(response, bytes.TrimSpace(call.content)) {
			t.Fatalf("%s status=%d response=%s", call.name, status, response)
		}
	}
	if len(gateway.envelopes) != 2 ||
		gateway.envelopes[0].Call.Tool != permissions.ToolWebSearch ||
		gateway.envelopes[0].Call.Path != "Loom governed handoff" ||
		gateway.envelopes[1].Call.Tool != permissions.ToolWebFetch ||
		gateway.envelopes[1].Call.Path != "https://example.com/" {
		t.Fatalf("gateway envelopes = %#v", gateway.envelopes)
	}
}
