package toolbroker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

type MCPToolClient interface {
	CallTool(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// MCPRegistry binds Loom's non-secret server ID to an already initialized MCP
// client. Transport setup and credentials remain outside model-visible calls.
type MCPRegistry struct {
	clients map[string]MCPToolClient
}

func NewMCPRegistry(clients map[string]MCPToolClient) (*MCPRegistry, error) {
	if len(clients) == 0 {
		return nil, ErrInvalidConfig
	}
	owned := make(map[string]MCPToolClient, len(clients))
	for server, client := range clients {
		if !validIdentifier(server) || nilMCPClient(client) {
			return nil, ErrInvalidConfig
		}
		owned[server] = client
	}
	return &MCPRegistry{clients: owned}, nil
}

func (registry *MCPRegistry) CallTool(
	ctx context.Context,
	server string,
	tool string,
	arguments json.RawMessage,
) (string, error) {
	if registry == nil || ctx == nil || !validIdentifier(server) ||
		!validIdentifier(tool) || rejectDuplicateJSONKeys(arguments) != nil {
		return "", ErrInvalidCall
	}
	client := registry.clients[server]
	if nilMCPClient(client) {
		return "", ErrToolDenied
	}
	var decoded map[string]any
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil || decoded == nil ||
		decoder.Decode(&struct{}{}) != io.EOF {
		return "", ErrInvalidCall
	}
	request := mcp.CallToolRequest{}
	request.Params.Name = tool
	request.Params.Arguments = decoded
	request.Params.RawArguments = append(json.RawMessage(nil), arguments...)
	result, err := client.CallTool(ctx, request)
	if err != nil || result == nil || result.IsError ||
		result.StructuredContent != nil || len(result.Content) == 0 {
		return "", ErrToolFailed
	}
	parts := make([]string, 0, len(result.Content))
	for _, block := range result.Content {
		text, ok := block.(mcp.TextContent)
		if !ok || text.Type != "text" {
			return "", ErrToolFailed
		}
		value := strings.TrimSpace(text.Text)
		if value == "" || !safeText(value) {
			return "", ErrToolFailed
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n\n"), nil
}

func nilMCPClient(client MCPToolClient) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
