//go:build darwin

package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
)

func TestClaudeContextMCPRealCLIContract(t *testing.T) {
	if os.Getenv("LOOM_P2D_CLAUDE_CONTEXT_CONTRACT") != "1" {
		t.Skip("Claude Context MCP component gate closed")
	}
	executable := os.Getenv("LOOM_P2D_CLAUDE_EXECUTABLE")
	if resolved, err := ResolveHarnessExecutable(executable); err != nil || resolved != executable {
		t.Fatal("exact Claude executable unavailable")
	}
	if version, err := harnessContextExecutableVersion(executable); err != nil ||
		!HasContextRetrievalConformance(ClaudeCodeAdapterType, version) {
		t.Fatal("Claude executable lacks Context MCP conformance attestation")
	}
	content := []byte("bounded authoritative context from Loom")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "current-task", ContentDigest: harnessContextDigest(content),
	}
	retriever := &harnessContextRetrieverFixture{
		want: proposal,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
			Trust:         contextcapsule.TrustAuthoritative,
			Scope:         contextcapsule.ScopeConversationShared,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "authority:current-task", Content: content,
		},
	}
	contextService, err := newHarnessContextMCP(retriever)
	if err != nil {
		t.Fatal(err)
	}
	defer contextService.Close()
	provider := &claudeContextProviderFixture{
		proposal: proposal, content: string(content),
	}
	gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
		Client: provider, Random: bytes.NewReader(make([]byte, 32)),
		MaxRequestBytes: 8 << 20, MaxResponseBytes: 8 << 20, MaxRequests: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	commands := &claudeContextCommandRecorder{
		delegate: NewSystemHarnessCommandRunner(), contextToken: contextService.Lease().Token,
	}
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
		Gateway: gateway, Commands: commands,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: executable, WorkspacePath: root, HomePath: root, TempPath: root,
		ModelID: ClaudeCodeModelID,
		Prompt:  []byte("Read the exact omitted Context item, then answer only: context accepted"),
		SystemPrompt: "Use loom_read_context exactly once with item_id current-task and content_digest " +
			proposal.ContentDigest + ".",
		Timeout: 45 * time.Second, MaxOutputBytes: 128 << 10,
		ContextMCP: contextService.Lease(),
	}, []byte("controlled-fake-anthropic-key"))
	if err != nil {
		t.Fatalf("real Claude Context MCP contract error = %v provider=%s requests=%d tools=%v request=%v rounds=%v mcp=%s command=%s",
			err, provider.failure(), provider.count(), provider.toolCatalog(), provider.requestShape(), provider.roundShapes(),
			contextService.metadata(), commands.snapshot())
	}
	if result.Content != "context accepted" || retriever.calls != 1 ||
		provider.count() != 3 || provider.failure() != "" || commands.snapshot() != "safe" {
		t.Fatalf("result=%#v broker_calls=%d requests=%d failure=%q tools=%v request=%v mcp=%s command=%s",
			result, retriever.calls, provider.count(), provider.failure(),
			provider.toolCatalog(), provider.requestShape(), contextService.metadata(), commands.snapshot())
	}
}

type claudeContextCommandRecorder struct {
	delegate     HarnessCommandRunner
	contextToken string
	mu           sync.Mutex
	state        string
}

func (recorder *claudeContextCommandRecorder) RunCommand(
	ctx context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	arguments := strings.Join(request.Arguments, "\n")
	environment := strings.Join(request.Environment, "\n")
	state := "safe"
	if strings.Contains(arguments, "controlled-fake-anthropic-key") ||
		strings.Contains(arguments, recorder.contextToken) ||
		strings.Contains(environment, "controlled-fake-anthropic-key") ||
		!strings.Contains(environment, harnessContextMCPTokenEnv+"="+recorder.contextToken) {
		state = "unsafe_transport"
	}
	result, err := recorder.delegate.RunCommand(ctx, request)
	recorder.mu.Lock()
	recorder.state = state
	recorder.mu.Unlock()
	return result, err
}

func (recorder *claudeContextCommandRecorder) snapshot() string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.state
}

type claudeContextProviderFixture struct {
	mu            sync.Mutex
	requests      int
	failureReason string
	proposal      contextcapsule.RetrievalProposal
	content       string
	tools         []string
	requestKeys   []string
	rounds        []string
}

func (fixture *claudeContextProviderFixture) Do(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 8<<20))
	if err != nil {
		fixture.fail("request_body")
		return nil, err
	}
	fixture.mu.Lock()
	fixture.requests++
	sequence := fixture.requests
	fixture.mu.Unlock()
	fixture.captureRequestShape(body)
	fixture.captureRound(sequence, body)
	if request.URL.String() != ClaudeCodeEndpoint+"?beta=true" ||
		request.Header.Get("x-api-key") != "controlled-fake-anthropic-key" ||
		!bytes.Contains(body, []byte(`"model":"`+ClaudeCodeModelID+`"`)) {
		fixture.fail("request_boundary")
	}
	hasContextTool := claudeContextHasTool(body, "mcp__loom_context__loom_read_context")
	if sequence == 1 {
		if hasContextTool {
			fixture.fail("startup_request_tools")
		}
		if bytes.Contains(body, []byte(fixture.content)) {
			fixture.fail("startup_content_disclosure")
		}
		return claudeContextResponse(claudeContextTextEvents("ready")), nil
	}
	if sequence == 2 {
		if !hasContextTool {
			fixture.fail("agent_round_tools")
		}
		if bytes.Contains(body, []byte(fixture.content)) {
			fixture.fail("agent_round_content_disclosure")
		}
		arguments, _ := json.Marshal(map[string]string{
			"item_id":        fixture.proposal.ItemID,
			"content_digest": fixture.proposal.ContentDigest,
		})
		return claudeContextResponse(claudeContextToolEvents(string(arguments))), nil
	}
	if sequence == 3 {
		if !bytes.Contains(body, []byte(`"type":"tool_result"`)) ||
			!bytes.Contains(body, []byte(fixture.content)) {
			fixture.fail("second_round_context")
		}
		return claudeContextResponse(claudeContextTextEvents("context accepted")), nil
	}
	fixture.fail("extra_provider_request")
	return claudeContextResponse(claudeContextTextEvents("unexpected")), nil
}

func (fixture *claudeContextProviderFixture) captureRound(sequence int, body []byte) {
	toolNames := claudeContextToolNames(body)
	shape := strconv.Itoa(sequence) + ":tool=" +
		map[bool]string{true: "yes", false: "no"}[claudeContextHasTool(body, "mcp__loom_context__loom_read_context")] +
		":content=" + map[bool]string{true: "yes", false: "no"}[bytes.Contains(body, []byte(fixture.content))] +
		":tools=" + strings.Join(toolNames, ",")
	fixture.mu.Lock()
	fixture.rounds = append(fixture.rounds, shape)
	fixture.mu.Unlock()
}

func (fixture *claudeContextProviderFixture) roundShapes() []string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]string(nil), fixture.rounds...)
}

func (fixture *claudeContextProviderFixture) captureRequestShape(body []byte) {
	var payload map[string]json.RawMessage
	if json.Unmarshal(body, &payload) != nil {
		return
	}
	var tools []struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(payload["tools"], &tools)
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.requestKeys = keys
	fixture.tools = fixture.tools[:0]
	for _, tool := range tools {
		fixture.tools = append(fixture.tools, tool.Name)
	}
}

func (fixture *claudeContextProviderFixture) toolCatalog() []string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]string(nil), fixture.tools...)
}

func (fixture *claudeContextProviderFixture) requestShape() []string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]string(nil), fixture.requestKeys...)
}

func (fixture *claudeContextProviderFixture) fail(reason string) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.failureReason == "" {
		fixture.failureReason = reason
	}
}

func (fixture *claudeContextProviderFixture) failure() string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.failureReason
}

func (fixture *claudeContextProviderFixture) count() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.requests
}

func claudeContextHasTool(body []byte, name string) bool {
	for _, candidate := range claudeContextToolNames(body) {
		if candidate == name {
			return true
		}
	}
	return false
}

func claudeContextToolNames(body []byte) []string {
	var payload struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	names := make([]string, 0, len(payload.Tools))
	for _, tool := range payload.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func claudeContextResponse(events string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(events)),
	}
}

func claudeContextToolEvents(arguments string) string {
	return claudeContextSSE([]map[string]any{
		{"type": "message_start", "message": claudeContextMessage("msg_context_tool", nil, nil)},
		{"type": "content_block_start", "index": 0, "content_block": map[string]any{
			"type": "tool_use", "id": "toolu_context_1",
			"name": "mcp__loom_context__loom_read_context", "input": map[string]any{},
		}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]any{
			"type": "input_json_delta", "partial_json": arguments,
		}},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]any{
			"stop_reason": "tool_use", "stop_sequence": nil,
		}, "usage": map[string]any{"output_tokens": 12}},
		{"type": "message_stop"},
	})
}

func claudeContextTextEvents(text string) string {
	return claudeContextSSE([]map[string]any{
		{"type": "message_start", "message": claudeContextMessage("msg_context_text", nil, nil)},
		{"type": "content_block_start", "index": 0, "content_block": map[string]any{
			"type": "text", "text": "",
		}},
		{"type": "content_block_delta", "index": 0, "delta": map[string]any{
			"type": "text_delta", "text": text,
		}},
		{"type": "content_block_stop", "index": 0},
		{"type": "message_delta", "delta": map[string]any{
			"stop_reason": "end_turn", "stop_sequence": nil,
		}, "usage": map[string]any{"output_tokens": 3}},
		{"type": "message_stop"},
	})
}

func claudeContextMessage(id string, content []any, stopReason any) map[string]any {
	if content == nil {
		content = []any{}
	}
	return map[string]any{
		"id": id, "type": "message", "role": "assistant", "model": ClaudeCodeModelID,
		"content": content, "stop_reason": stopReason, "stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens": 8, "output_tokens": 1,
			"cache_creation_input_tokens": 0, "cache_read_input_tokens": 0,
		},
	}
}

func claudeContextSSE(events []map[string]any) string {
	var output strings.Builder
	for _, event := range events {
		encoded, _ := json.Marshal(event)
		output.WriteString("event: ")
		output.WriteString(event["type"].(string))
		output.WriteString("\ndata: ")
		output.Write(encoded)
		output.WriteString("\n\n")
	}
	return output.String()
}
