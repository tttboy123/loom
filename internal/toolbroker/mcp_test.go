package toolbroker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

type mcpToolClientFixture struct{ request mcp.CallToolRequest }

func (fixture *mcpToolClientFixture) CallTool(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	fixture.request = request
	return &mcp.CallToolResult{Content: []mcp.Content{
		mcp.TextContent{Type: "text", Text: "issue title"},
		mcp.TextContent{Type: "text", Text: "issue body"},
	}}, nil
}

func TestMCPRegistryCallsExactConfiguredClientAndAcceptsTextOnly(t *testing.T) {
	client := &mcpToolClientFixture{}
	registry, err := NewMCPRegistry(map[string]MCPToolClient{"github": client})
	if err != nil {
		t.Fatal(err)
	}
	content, err := registry.CallTool(
		context.Background(), "github", "get_issue",
		json.RawMessage(`{"owner":"earendil","number":42}`),
	)
	if err != nil || content != "issue title\n\nissue body" ||
		client.request.Params.Name != "get_issue" || client.request.Params.Arguments == nil {
		t.Fatalf("content=%q request=%#v error=%v", content, client.request, err)
	}
}

func TestMCPRegistryRejectsErrorAndNonTextContent(t *testing.T) {
	for _, result := range []*mcp.CallToolResult{
		{IsError: true, Content: []mcp.Content{mcp.TextContent{Type: "text", Text: "private raw error"}}},
		{Content: []mcp.Content{mcp.ImageContent{Type: "image", Data: "AA==", MIMEType: "image/png"}}},
	} {
		client := mcpToolClientFunc(func(
			context.Context,
			mcp.CallToolRequest,
		) (*mcp.CallToolResult, error) {
			return result, nil
		})
		registry, err := NewMCPRegistry(map[string]MCPToolClient{"github": client})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := registry.CallTool(context.Background(), "github", "get_issue", json.RawMessage(`{}`)); err == nil {
			t.Fatalf("unsafe MCP result accepted: %#v", result)
		}
	}
}

type mcpToolClientFunc func(
	context.Context,
	mcp.CallToolRequest,
) (*mcp.CallToolResult, error)

func (function mcpToolClientFunc) CallTool(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	return function(ctx, request)
}
