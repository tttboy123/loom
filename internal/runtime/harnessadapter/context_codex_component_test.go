//go:build darwin

package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
)

func TestCodexContextMCPRealCLIContract(t *testing.T) {
	if os.Getenv("LOOM_P2D_CODEX_CONTEXT_CONTRACT") != "1" {
		t.Skip("Codex Context MCP component gate closed")
	}
	executable := os.Getenv("LOOM_P2D_CODEX_EXECUTABLE")
	if resolved, err := ResolveHarnessExecutable(executable); err != nil || resolved != executable {
		t.Fatal("exact Codex executable unavailable")
	}
	if version, err := harnessContextExecutableVersion(executable); err != nil ||
		!HasContextRetrievalConformance(CodexAdapterType, version) {
		t.Fatal("Codex executable lacks Context MCP conformance attestation")
	}
	content := []byte("bounded observed context from Loom")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "diff-detail", ContentDigest: harnessContextDigest(content),
		ArtifactRef: "artifact:diff-1",
	}
	retriever := &harnessContextRetrieverFixture{
		want: proposal,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
			Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
			ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceObservation,
			SourceRef: "evidence:diff-1", ArtifactRef: proposal.ArtifactRef, Content: content,
		},
	}
	contextService, err := newHarnessContextMCP(retriever)
	if err != nil {
		t.Fatal(err)
	}
	defer contextService.Close()
	provider := &codexContextProviderFixture{
		proposal: proposal, content: string(content),
	}
	gateway, err := NewOpenAIAttemptGateway(AttemptGatewayConfig{
		Client: provider, Random: bytes.NewReader(make([]byte, 32)),
		MaxRequestBytes: 8 << 20, MaxResponseBytes: 8 << 20, MaxRequests: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	commandRecorder := &codexContextCommandRecorder{delegate: NewSystemHarnessCommandRunner()}
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{
		Gateway: gateway, Commands: commandRecorder,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	result, err := runner.RunHarness(context.Background(), HarnessProcessRequest{
		ExecutablePath: executable, WorkspacePath: root, HomePath: root, TempPath: root,
		ModelID: CodexModelID, ReasoningEffort: "high",
		Prompt: []byte("Read the exact omitted Context item, then answer only: context accepted"),
		SystemPrompt: "Use loom_read_context exactly once with item_id diff-detail, content_digest " +
			proposal.ContentDigest + ", and artifact_ref artifact:diff-1.",
		Timeout: 45 * time.Second, MaxOutputBytes: 64 << 10,
		ContextMCP: contextService.Lease(),
	}, []byte("controlled-fake-provider-key"))
	if err != nil {
		t.Fatalf("real Codex Context MCP contract error = %v, provider=%s mcp=%s events=%v",
			err, provider.failure(), contextService.metadata(), commandRecorder.snapshot())
	}
	if result.Content != "context accepted" || retriever.calls != 1 ||
		provider.count() != 2 || provider.failure() != "" {
		t.Fatalf("result=%#v broker_calls=%d requests=%d failure=%q tools=%v mcp=%s events=%v",
			result, retriever.calls, provider.count(), provider.failure(),
			provider.toolCatalog(), contextService.metadata(), commandRecorder.snapshot())
	}
}

type codexContextCommandRecorder struct {
	delegate HarnessCommandRunner
	mu       sync.Mutex
	events   []string
}

func (recorder *codexContextCommandRecorder) RunCommand(
	ctx context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	result, err := recorder.delegate.RunCommand(ctx, request)
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	for _, line := range bytes.Split(result.Stdout, []byte("\n")) {
		var event struct {
			Type       string `json:"type"`
			ServerName string `json:"server_name"`
			Status     string `json:"status"`
			Item       struct {
				Type   string `json:"type"`
				Server string `json:"server"`
				Tool   string `json:"tool"`
				Status string `json:"status"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
			} `json:"item"`
		}
		if json.Unmarshal(line, &event) == nil && event.Type != "" {
			category := ""
			if event.Item.Error != nil {
				category = codexContextErrorCategory(event.Item.Error.Message)
			}
			recorder.events = append(recorder.events,
				event.Type+":"+event.Item.Type+":"+event.Item.Server+":"+
					event.Item.Tool+":"+event.Item.Status+":"+category)
		}
	}
	return result, err
}

func codexContextErrorCategory(message string) string {
	for _, category := range []string{
		"unknown", "not found", "invalid", "denied", "timeout", "failed",
		"connection", "transport", "schema", "request", "approval", "client",
		"argument", "permission", "closed", "unavailable", "deserialize",
	} {
		if strings.Contains(strings.ToLower(message), category) {
			return strings.ReplaceAll(category, " ", "_")
		}
	}
	if message != "" {
		return "other"
	}
	return ""
}

func (recorder *codexContextCommandRecorder) snapshot() []string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]string(nil), recorder.events...)
}

type codexContextProviderFixture struct {
	mu            sync.Mutex
	requests      int
	failureReason string
	proposal      contextcapsule.RetrievalProposal
	content       string
	tools         []string
}

func (fixture *codexContextProviderFixture) Do(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 8<<20))
	if err != nil {
		fixture.fail("request_body")
		return nil, err
	}
	fixture.mu.Lock()
	fixture.requests++
	sequence := fixture.requests
	fixture.mu.Unlock()
	if request.URL.String() != CodexEndpoint ||
		request.Header.Get("Authorization") != "Bearer controlled-fake-provider-key" ||
		!bytes.Contains(body, []byte(`"model":"`+CodexModelID+`"`)) {
		fixture.fail("request_boundary")
	}
	if sequence == 1 {
		fixture.captureTools(body)
		if codexContextHasTool(body, "tool_search") ||
			codexContextToolName(body) == "" || bytes.Contains(body, []byte(fixture.content)) {
			fixture.fail("first_round_exact_catalog")
		}
		arguments, _ := json.Marshal(map[string]string{
			"item_id": fixture.proposal.ItemID, "content_digest": fixture.proposal.ContentDigest,
			"artifact_ref": fixture.proposal.ArtifactRef,
		})
		return codexContextResponse(codexContextNamespaceToolEvents(string(arguments))), nil
	}
	if sequence == 2 {
		if !bytes.Contains(body, []byte(`"type":"function_call_output"`)) ||
			!bytes.Contains(body, []byte(fixture.content)) {
			fixture.fail("second_round_context")
		}
		return codexContextResponse(codexContextTextEvents("context accepted")), nil
	}
	fixture.fail("extra_provider_request")
	return codexContextResponse(codexContextTextEvents("unexpected")), nil
}

func (fixture *codexContextProviderFixture) fail(reason string) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.failureReason == "" {
		fixture.failureReason = reason
	}
}

func (fixture *codexContextProviderFixture) failure() string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.failureReason
}

func (fixture *codexContextProviderFixture) count() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.requests
}

func (fixture *codexContextProviderFixture) captureTools(body []byte) {
	var payload struct {
		Tools []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.tools = fixture.tools[:0]
	for _, tool := range payload.Tools {
		fixture.tools = append(fixture.tools, tool.Type+":"+tool.Name)
	}
}

func (fixture *codexContextProviderFixture) toolCatalog() []string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return append([]string(nil), fixture.tools...)
}

func codexContextResponse(events string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(events)),
	}
}

func codexContextToolName(body []byte) string {
	var payload struct {
		Tools []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	for _, tool := range payload.Tools {
		if tool.Type == "function" && strings.Contains(tool.Name, "loom_read_context") {
			return tool.Name
		}
	}
	return ""
}

func codexContextHasTool(body []byte, name string) bool {
	var payload struct {
		Tools []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	for _, tool := range payload.Tools {
		if tool.Type == name || tool.Name == name {
			return true
		}
	}
	return false
}

func codexContextSearchEvents() string {
	item := map[string]any{
		"type": "tool_search_call", "call_id": "search_context_1",
		"execution": "client", "arguments": map[string]any{
			"query": "Loom exact omitted context item", "limit": 1,
		},
	}
	return codexContextSSE([]map[string]any{
		{"type": "response.created", "sequence_number": 0, "response": codexContextResponseObject("resp_context_search", nil, "in_progress", 0, 0)},
		{"type": "response.output_item.done", "sequence_number": 1, "output_index": 0, "item": item},
		{"type": "response.completed", "sequence_number": 2, "response": codexContextResponseObject("resp_context_search", []any{item}, "completed", 2, 1)},
	})
}

func codexContextNamespaceToolEvents(arguments string) string {
	item := map[string]any{
		"id": "fc_context_1", "type": "function_call", "status": "completed",
		"call_id": "call_context_1", "namespace": "mcp__loom_context",
		"name": "loom_read_context", "arguments": arguments,
	}
	return codexContextSSE([]map[string]any{
		{"type": "response.created", "sequence_number": 0, "response": codexContextResponseObject("resp_context_1", nil, "in_progress", 0, 0)},
		{"type": "response.output_item.done", "sequence_number": 1, "output_index": 0, "item": item},
		{"type": "response.completed", "sequence_number": 2, "response": codexContextResponseObject("resp_context_1", []any{item}, "completed", 4, 2)},
	})
}

func codexContextTextEvents(text string) string {
	item := map[string]any{
		"id": "msg_context_2", "type": "message", "status": "completed", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
	}
	return codexContextSSE([]map[string]any{
		{"type": "response.created", "sequence_number": 0, "response": codexContextResponseObject("resp_context_2", nil, "in_progress", 0, 0)},
		{"type": "response.output_item.added", "sequence_number": 1, "output_index": 0, "item": item},
		{"type": "response.output_text.delta", "sequence_number": 2, "item_id": "msg_context_2", "output_index": 0, "content_index": 0, "delta": text},
		{"type": "response.output_text.done", "sequence_number": 3, "item_id": "msg_context_2", "output_index": 0, "content_index": 0, "text": text},
		{"type": "response.output_item.done", "sequence_number": 4, "output_index": 0, "item": item},
		{"type": "response.completed", "sequence_number": 5, "response": codexContextResponseObject("resp_context_2", []any{item}, "completed", 5, 2)},
	})
}

func codexContextResponseObject(id string, output []any, status string, input, outputTokens int) map[string]any {
	if output == nil {
		output = []any{}
	}
	return map[string]any{
		"id": id, "object": "response", "created_at": 1, "status": status,
		"model": CodexModelID, "output": output, "parallel_tool_calls": false,
		"tool_choice": "auto", "tools": []any{},
		"usage": map[string]any{
			"input_tokens": input, "input_tokens_details": map[string]any{"cached_tokens": 0},
			"output_tokens": outputTokens, "output_tokens_details": map[string]any{"reasoning_tokens": 0},
			"total_tokens": input + outputTokens,
		},
	}
}

func codexContextSSE(events []map[string]any) string {
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
