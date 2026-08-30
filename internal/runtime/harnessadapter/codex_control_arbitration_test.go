package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/controltool"
)

func TestCodexControlArbitrationSchemaAndDecoder(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	tools := []string{"loom_sessions_search", "loom_missions_create_preview"}
	schema, err := codexControlSelectionOutputSchema(registry, tools)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		AdditionalProperties bool `json:"additionalProperties"`
		Properties           struct {
			ToolName struct {
				Enum []string `json:"enum"`
			} `json:"tool_name"`
			ToolArgumentsJSON struct {
				Description string `json:"description"`
			} `json:"tool_arguments_json"`
		} `json:"properties"`
	}
	if json.Unmarshal(schema, &decoded) != nil || decoded.AdditionalProperties ||
		len(decoded.Properties.ToolName.Enum) != 3 ||
		decoded.Properties.ToolName.Enum[0] != codexControlNoTool ||
		decoded.Properties.ToolName.Enum[1] != tools[0] ||
		decoded.Properties.ToolName.Enum[2] != tools[1] ||
		!strings.Contains(
			decoded.Properties.ToolArgumentsJSON.Description,
			`loom_missions_create_preview={"type":"object","additionalProperties":false,"properties":{"objective"`,
		) || strings.Contains(decoded.Properties.ToolArgumentsJSON.Description, `"title"`) {
		t.Fatalf("selection schema = %s", schema)
	}

	selection, err := decodeCodexControlSelection(
		`{"response":"Preparing the governed Mission review.","tool_name":"loom_missions_create_preview","tool_arguments_json":"{\"objective\":\"P7\"}"}`,
		tools,
	)
	if err != nil || selection.Response != "Preparing the governed Mission review." ||
		selection.ToolName != "loom_missions_create_preview" ||
		!bytes.Equal(selection.Arguments, json.RawMessage(`{"objective":"P7"}`)) {
		t.Fatalf("selection = %#v, error = %v", selection, err)
	}

	for name, payload := range map[string]string{
		"unknown tool":         `{"response":"x","tool_name":"loom_unknown","tool_arguments_json":"{}"}`,
		"none with arguments":  `{"response":"x","tool_name":"none","tool_arguments_json":"{\"query\":\"P7\"}"}`,
		"duplicate key":        `{"response":"x","response":"y","tool_name":"none","tool_arguments_json":"{}"}`,
		"extra field":          `{"response":"x","tool_name":"none","tool_arguments_json":"{}","extra":true}`,
		"non object arguments": `{"response":"x","tool_name":"loom_teams_search","tool_arguments_json":"[]"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCodexControlSelection(payload, tools); !errors.Is(
				err, ErrHarnessControlMCP,
			) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCodexControlArbitrationSystemPromptBindsDirectAndBrokeredToolUse(t *testing.T) {
	prompt, err := appendCodexControlArbitrationSystemPrompt("Loom governed policy")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"directly call that exact MCP tool", "select it in tool_name",
		"tool_arguments_json", "tool_name=none", "ordinary conversation",
	} {
		if !bytes.Contains([]byte(prompt), []byte(required)) {
			t.Fatalf("arbitration prompt missing %q: %s", required, prompt)
		}
	}
	if _, err := appendCodexControlArbitrationSystemPrompt(
		strings.Repeat("x", maxHarnessPromptBytes),
	); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("oversized prompt error = %v", err)
	}
}

func TestCodexSegmentSessionExecutesModelSelectedControlToolThenSummarizesResult(t *testing.T) {
	stream := newCodexAppServerSessionFixture()
	stream.turnContents = []string{
		`{"response":"I will inspect the governed Team catalog.","tool_name":"loom_teams_search","tool_arguments_json":"{\"query\":\"P7\",\"limit\":8}"}`,
		`{"response":"No governed Agent Teams matched P7."}`,
	}
	gateway := &codexControlGatewayFixture{}
	segment := openCodexControlSegmentSessionForTest(t, stream, gateway)
	defer func() { _ = segment.Close(context.Background()) }()

	if err := segment.BeginControlTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	result, err := segment.Respond(context.Background(), []byte("Find Agent Teams matching P7."))
	if err != nil {
		t.Fatal(err)
	}
	batch, err := segment.EndControlTurn()
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "No governed Agent Teams matched P7." || result.Accounting == nil ||
		result.Accounting.InputTokens != 3 || result.Accounting.OutputTokens != 5 ||
		result.Accounting.TotalTokens != 8 {
		t.Fatalf("result = %#v", result)
	}
	if gateway.calls != 1 || gateway.lastCall.ToolID != controltool.ToolTeamsSearch ||
		!bytes.Equal(gateway.lastCall.Arguments, json.RawMessage(`{"limit":8,"query":"P7"}`)) {
		t.Fatalf("gateway calls=%d call=%#v", gateway.calls, gateway.lastCall)
	}
	if len(batch.CompletedCalls) != 1 ||
		batch.CompletedCalls[0].ToolID != controltool.ToolTeamsSearch ||
		batch.CompletedCalls[0].Effect != controltool.EffectRead {
		t.Fatalf("completed calls = %#v", batch.CompletedCalls)
	}

	requests := stream.requestsSnapshot()
	if len(requests) != 4 || len(requests[2].OutputSchema) == 0 ||
		len(requests[2].AdditionalContext) != 0 || len(requests[3].OutputSchema) == 0 ||
		requests[3].AdditionalContextKind != "untrusted" ||
		requests[3].AdditionalContext != `{"teams":[]}` ||
		bytes.Contains([]byte(requests[3].Text), []byte(`{"teams":[]}`)) {
		t.Fatalf("Codex arbitration requests = %#v", requests)
	}
}

func TestCodexSegmentSessionAcceptsExactToolSelectionAfterDirectMCPCompletion(t *testing.T) {
	stream := newCodexAppServerSessionFixture()
	stream.turnContents = []string{
		`{"response":"No governed Agent Teams matched P7.","tool_name":"loom_teams_search","tool_arguments_json":"{\"query\":\"P7\",\"limit\":8}"}`,
	}
	gateway := &codexControlGatewayFixture{}
	segment := openCodexControlSegmentSessionForTest(t, stream, gateway)
	defer func() { _ = segment.Close(context.Background()) }()

	if err := segment.BeginControlTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	directResult, err := CallHarnessControlTool(
		context.Background(), segment.controlMCP.Lease(), "loom_teams_search",
		json.RawMessage(`{"query":"P7","limit":8}`),
	)
	zeroHarnessBytes(directResult)
	if err != nil {
		t.Fatal(err)
	}
	result, err := segment.Respond(context.Background(), []byte("Find Agent Teams matching P7."))
	if err != nil {
		t.Fatal(err)
	}
	batch, err := segment.EndControlTurn()
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "No governed Agent Teams matched P7." || gateway.calls != 1 ||
		len(batch.CompletedCalls) != 1 ||
		batch.CompletedCalls[0].ToolID != controltool.ToolTeamsSearch ||
		len(stream.requestsSnapshot()) != 3 {
		t.Fatalf("result=%#v calls=%d batch=%#v", result, gateway.calls, batch)
	}
}

func TestCodexSegmentSessionRejectsSelectionThatDiffersFromDirectMCPCompletion(t *testing.T) {
	stream := newCodexAppServerSessionFixture()
	stream.turnContents = []string{
		`{"response":"Preparing a Mission review.","tool_name":"loom_missions_create_preview","tool_arguments_json":"{\"title\":\"P7\",\"objective\":\"Verify P7\"}"}`,
	}
	gateway := &codexControlGatewayFixture{}
	segment := openCodexControlSegmentSessionForTest(t, stream, gateway)

	if err := segment.BeginControlTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	directResult, err := CallHarnessControlTool(
		context.Background(), segment.controlMCP.Lease(), "loom_teams_search",
		json.RawMessage(`{"query":"P7","limit":8}`),
	)
	zeroHarnessBytes(directResult)
	if err != nil {
		t.Fatal(err)
	}
	_, err = segment.Respond(
		context.Background(), []byte("Create a Mission named P7."),
	)
	if !errors.Is(err, ErrHarnessControlMCP) ||
		CodexControlArbitrationFailureStage(err) != codexControlFailureDirectMatch {
		t.Fatalf("mismatched selection error = %v", err)
	}
	if segment.Healthy() {
		t.Fatal("mismatched direct completion left Codex Segment Session healthy")
	}
}

func TestCodexSegmentSessionClassifiesSelectionDecodeFailureWithoutContent(t *testing.T) {
	stream := newCodexAppServerSessionFixture()
	stream.turnContents = []string{`{"response":"invalid envelope"}`}
	segment := openCodexControlSegmentSessionForTest(
		t, stream, &codexControlGatewayFixture{},
	)

	if err := segment.BeginControlTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	_, err := segment.Respond(context.Background(), []byte("bounded request"))
	if !errors.Is(err, ErrHarnessControlMCP) ||
		CodexControlArbitrationFailureStage(err) != codexControlFailureSelectionDecode {
		t.Fatalf("selection decode error = %v", err)
	}
}

func TestCodexSegmentSessionClassifiesBrokerInvalidRequestWithoutArguments(t *testing.T) {
	stream := newCodexAppServerSessionFixture()
	stream.turnContents = []string{
		`{"response":"Preparing Mission review.","tool_name":"loom_missions_create_preview","tool_arguments_json":"{\"objective\":\"P7\",\"title\":\"undeclared\"}"}`,
	}
	segment := openCodexControlSegmentSessionForTest(
		t, stream, &codexControlGatewayFixture{err: controltool.ErrInvalidCall},
	)

	if err := segment.BeginControlTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	_, err := segment.Respond(context.Background(), []byte("prepare Mission"))
	if !errors.Is(err, ErrHarnessControlMCP) ||
		CodexControlArbitrationFailureStage(err) != codexControlFailureBrokerInvalid {
		t.Fatalf("broker invalid request error = %v stage=%s",
			err, CodexControlArbitrationFailureStage(err))
	}
}

func TestCodexSegmentSessionReturnsOrdinaryArbitratedReplyWithoutTool(t *testing.T) {
	stream := newCodexAppServerSessionFixture()
	stream.turnContents = []string{
		`{"response":"Hello from the governed Codex route.","tool_name":"none","tool_arguments_json":"{}"}`,
	}
	gateway := &codexControlGatewayFixture{}
	segment := openCodexControlSegmentSessionForTest(t, stream, gateway)
	defer func() { _ = segment.Close(context.Background()) }()

	if err := segment.BeginControlTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	result, err := segment.Respond(context.Background(), []byte("Hello."))
	if err != nil {
		t.Fatal(err)
	}
	batch, err := segment.EndControlTurn()
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Hello from the governed Codex route." || gateway.calls != 0 ||
		len(batch.CompletedCalls) != 0 || len(stream.requestsSnapshot()) != 3 {
		t.Fatalf("result=%#v calls=%d batch=%#v", result, gateway.calls, batch)
	}
}

func TestHarnessControlMCPSuspendsCallsDuringCodexResultSummary(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	gateway := &codexControlGatewayFixture{}
	service, err := newHarnessControlMCP(context.Background(), registry, gateway)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if err := service.BeginTurn(codexControlTurnFixture()); err != nil {
		t.Fatal(err)
	}
	if err := service.suspendTurnCalls(); err != nil {
		t.Fatal(err)
	}
	result, err := CallHarnessControlTool(
		context.Background(), service.Lease(), "loom_teams_search",
		json.RawMessage(`{"query":"P7"}`),
	)
	zeroHarnessBytes(result)
	if !errors.Is(err, ErrHarnessControlMCP) || gateway.calls != 0 {
		t.Fatalf("suspended call error=%v calls=%d", err, gateway.calls)
	}
	if _, err := service.EndTurn(); err != nil {
		t.Fatal(err)
	}
}

func openCodexControlSegmentSessionForTest(
	t *testing.T,
	stream *codexAppServerSessionFixture,
	gateway controltool.Gateway,
) *CodexSegmentSession {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "private")
	for _, directory := range []string{home, workspace, privateRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareCodexAuthFixture(t, home)
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	segment, err := OpenCodexSegmentSession(
		context.Background(), CodexSegmentSessionConfig{
			ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
			WorkspacePath: workspace, PrivateRoot: privateRoot,
			ModelID: CodexModelID, ReasoningEffort: "high",
			SystemPrompt: "Loom governed conversation policy",
			Timeout:      time.Hour, MaxOutputBytes: 1 << 16,
			Sessions:        &codexContinuationSessionRunnerFixture{session: stream},
			ControlRegistry: registry, ControlGateway: gateway,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return segment
}

func codexControlTurnFixture() controltool.TurnContext {
	return controltool.TurnContext{
		ConversationID: "conversation-1", SegmentID: "segment-1", AttemptID: "attempt-1",
		CatalogDigest: controltool.DigestBytes([]byte("bounded catalog")),
	}
}

type codexControlGatewayFixture struct {
	calls    int
	lastCall controltool.Call
	err      error
}

func (gateway *codexControlGatewayFixture) Call(
	_ context.Context,
	_ controltool.TurnContext,
	call controltool.Call,
) (controltool.Result, error) {
	gateway.calls++
	gateway.lastCall = controltool.Call{
		ToolID: call.ToolID, Arguments: append(json.RawMessage(nil), call.Arguments...),
	}
	if gateway.err != nil {
		return controltool.Result{}, gateway.err
	}
	return controltool.Result{Content: json.RawMessage(`{"teams":[]}`)}, nil
}
