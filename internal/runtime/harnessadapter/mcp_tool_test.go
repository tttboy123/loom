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

type explicitHarnessToolGatewayFixture struct {
	*harnessToolGatewayFixture
	allowedCalls int
}

type scopedHarnessToolGatewayFixture struct {
	*harnessToolGatewayFixture
	wantID     string
	wantDigest string
}

func (fixture *scopedHarnessToolGatewayFixture) AllowedToolCallsForBinding(
	binding loomruntime.FrozenExecutionBinding,
) []permissions.ToolKind {
	if binding.RemoteToolEnrollmentID != fixture.wantID ||
		binding.RemoteToolEnrollmentDigest != fixture.wantDigest {
		return []permissions.ToolKind{permissions.ToolRead}
	}
	return []permissions.ToolKind{permissions.ToolRead, permissions.ToolMCPTool}
}

func (fixture *explicitHarnessToolGatewayFixture) AllowedToolCalls() []permissions.ToolKind {
	fixture.allowedCalls++
	return fixture.harnessToolGatewayFixture.AllowedToolCalls()
}

func TestHarnessMCPToolNamesIncludesGenericMCPTool(t *testing.T) {
	lease := HarnessContextMCPLease{
		URL: "http://127.0.0.1:1/mcp", MCPToolEnabled: true,
	}
	if names := harnessMCPToolNames(lease); !reflect.DeepEqual(names, []string{"loom_mcp_call"}) {
		t.Fatalf("MCP-only names = %v", names)
	}
}

func TestHarnessAttemptToolsUseExactFrozenEnrollmentBinding(t *testing.T) {
	digest := strings.Repeat("a", 64)
	gateway := &scopedHarnessToolGatewayFixture{
		harnessToolGatewayFixture: &harnessToolGatewayFixture{
			allowed: []permissions.ToolKind{
				permissions.ToolRead, permissions.ToolMCPTool, permissions.ToolWebFetch,
			},
		},
		wantID: "enrollment.mcp.primary", wantDigest: digest,
	}
	allowed := allowedHarnessAttemptTools(gateway, loomruntime.FrozenExecutionBinding{
		RemoteToolEnrollmentID:     gateway.wantID,
		RemoteToolEnrollmentDigest: gateway.wantDigest,
	})
	if !reflect.DeepEqual(allowed, []permissions.ToolKind{
		permissions.ToolRead, permissions.ToolMCPTool,
	}) {
		t.Fatalf("scoped tools = %v", allowed)
	}
	wrong := allowedHarnessAttemptTools(gateway, loomruntime.FrozenExecutionBinding{
		RemoteToolEnrollmentID:     gateway.wantID,
		RemoteToolEnrollmentDigest: strings.Repeat("b", 64),
	})
	if !reflect.DeepEqual(wrong, []permissions.ToolKind{permissions.ToolRead}) {
		t.Fatalf("wrong-scope tools = %v", wrong)
	}
}

func TestHarnessAttemptMCPExplicitAllowedToolsOverrideGatewayCapabilities(t *testing.T) {
	gateway := &explicitHarnessToolGatewayFixture{harnessToolGatewayFixture: &harnessToolGatewayFixture{
		allowed: []permissions.ToolKind{permissions.ToolRead},
	}}
	service, err := newHarnessAttemptMCPWithContext(
		context.Background(), harnessAttemptMCPConfig{
			ToolGateway: gateway,
			ToolBinding: loomruntime.ToolCallBinding{
				ConversationID: "conversation-explicit", WorkItemID: "work-explicit",
				RunID: "run-explicit", ClaimGeneration: 1,
				RuntimeInstanceID: "runtime-explicit", AgentInstanceID: "agent-explicit",
				ExecutionBindingDigest: strings.Repeat("1", 64),
				CapsuleDigest:          strings.Repeat("2", 64), ClaimID: "claim-explicit",
				IncidentID: "11111111-1111-4111-8111-111111111111",
				JourneyID:  "11111111-1111-4111-8111-111111111111",
			},
			AllowedTools: []permissions.ToolKind{
				permissions.ToolMCPTool, permissions.ToolWebFetch, permissions.ToolBash,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if gateway.allowedCalls != 0 {
		t.Fatalf("AllowedToolCalls called %d times", gateway.allowedCalls)
	}
	if tools := service.toolNames(); !reflect.DeepEqual(tools, []string{
		"loom_mcp_call", "loom_run_command", "loom_web_fetch",
	}) {
		t.Fatalf("explicitly published tools = %v", tools)
	}
}

func TestHarnessDecodeGenericMCPToolCall(t *testing.T) {
	call, ok := decodeHarnessToolCall(
		permissions.ToolMCPTool,
		json.RawMessage(`{"server":"github","tool":"get_issue","arguments":{"z":2,"a":"loom"}}`),
	)
	if !ok || call != (permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "github/get_issue",
		Command: `{"a":"loom","z":2}`,
	}) {
		t.Fatalf("call = %#v ok=%t", call, ok)
	}

	oversized, err := json.Marshal(map[string]any{
		"server": "github", "tool": "get_issue",
		"arguments": map[string]any{"value": strings.Repeat("x", harnessMCPToolMaxArguments+1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := []string{
		`{"server":"github","server":"other","tool":"get_issue","arguments":{}}`,
		`{"server":"github","tool":"get_issue","arguments":{},"extra":true}`,
		`{"server":"github/issues","tool":"get_issue","arguments":{}}`,
		`{"server":"github","tool":"get issue","arguments":{}}`,
		`{"server":"github","tool":"get_issue","arguments":null}`,
		`{"server":"github","tool":"get_issue","arguments":[]}`,
		`{"server":"github","tool":"get_issue","arguments":{"owner":"a","owner":"b"}}`,
		`{"server":"github","tool":"get_issue","arguments":{"owner":"a\u0001"}}`,
		string(oversized),
	}
	for index, payload := range invalid {
		if got, accepted := decodeHarnessToolCall(
			permissions.ToolMCPTool, json.RawMessage(payload),
		); accepted {
			t.Fatalf("invalid[%d] accepted as %#v", index, got)
		}
	}
}

func TestHarnessAttemptMCPPublishesAndExecutesGenericMCPTool(t *testing.T) {
	gateway := &harnessToolGatewayFixture{
		allowed:  []permissions.ToolKind{permissions.ToolMCPTool},
		contents: map[permissions.ToolKind][]byte{permissions.ToolMCPTool: []byte("bounded MCP result\n")},
	}
	binding := loomruntime.ToolCallBinding{
		ConversationID: "conversation-mcp", WorkItemID: "work-mcp",
		RunID: "run-mcp", ClaimGeneration: 3, RuntimeInstanceID: "runtime-mcp",
		AgentInstanceID: "agent-mcp", ExecutionBindingDigest: strings.Repeat("1", 64),
		CapsuleDigest: strings.Repeat("2", 64), ClaimID: "claim-mcp",
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
	if !service.Lease().MCPToolEnabled {
		t.Fatal("MCP tool lease flag is false")
	}
	if tools := service.toolNames(); !reflect.DeepEqual(tools, []string{"loom_mcp_call"}) {
		t.Fatalf("published MCP tools = %v", tools)
	}

	definitions := service.mcpTools()
	if len(definitions) != 1 {
		t.Fatalf("tool definitions = %#v", definitions)
	}
	definition, ok := definitions[0].(map[string]any)
	if !ok {
		t.Fatalf("tool definition = %#v", definitions[0])
	}
	wantSchema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"server":    map[string]any{"type": "string"},
			"tool":      map[string]any{"type": "string"},
			"arguments": map[string]any{"type": "object"},
		},
		"required": []string{"server", "tool", "arguments"},
	}
	if definition["name"] != "loom_mcp_call" ||
		!reflect.DeepEqual(definition["inputSchema"], wantSchema) {
		t.Fatalf("MCP tool definition = %#v", definition)
	}
	if annotations, ok := definition["annotations"].(map[string]any); !ok ||
		annotations["readOnlyHint"] != false ||
		annotations["destructiveHint"] != true ||
		annotations["openWorldHint"] != true {
		t.Fatalf("generic MCP annotations = %#v", definition["annotations"])
	}

	params := json.RawMessage(`{"name":"loom_mcp_call","arguments":{"server":"github","tool":"get_issue","arguments":{"z":2,"a":"loom"}}}`)
	response, status := service.call(
		context.Background(), json.RawMessage(strconv.Itoa(1)), params,
	)
	if status != http.StatusOK || !bytes.Contains(response, []byte("bounded MCP result")) {
		t.Fatalf("status=%d response=%s", status, response)
	}
	if len(gateway.envelopes) != 1 || gateway.envelopes[0].Call != (permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "github/get_issue",
		Command: `{"a":"loom","z":2}`,
	}) {
		t.Fatalf("gateway envelopes = %#v", gateway.envelopes)
	}
}
