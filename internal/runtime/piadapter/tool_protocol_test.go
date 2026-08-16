//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/permissions"
)

func TestPiRPCToolProtocolAcceptsNativeResultThenFinalText(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)
	lines, envelope, result := piRPCToolFixtureLines(t, request.Dispatch.MessageID())
	state := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			envelope: envelope, result: result, resolved: true,
		},
	}
	for index, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &state, []byte(line),
		); err != nil {
			t.Fatalf("line[%d] %s: %v", index, line, err)
		}
	}
	if !state.settled || state.turnCount != 2 || string(state.assistant) != "Used governed tool result." ||
		state.toolEnvelope != envelope || !samePiToolResult(state.toolWireResult, result) ||
		len(state.frames) != 2 {
		t.Fatalf("tool protocol state = %#v frames=%d", state, len(state.frames))
	}
	for _, frame := range state.frames {
		if bytes.Contains(frame.Payload(), []byte(envelope.Call.Command)) {
			t.Fatal("ToolCall arguments leaked to a Bridge frame")
		}
	}
	accounting, err := piRPCCombinedAccounting(state.toolAssistant, state.finalAssistant)
	if err != nil || !accounting.UsageObserved || !accounting.CostObserved {
		t.Fatalf("combined accounting = %#v, %v", accounting, err)
	}
}

func TestPiRPCToolProtocolAcceptsBoundedSequentialCallsThenFinalText(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)
	first := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call:  permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf first"},
	}
	second := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call:  permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf second"},
	}
	firstResult := ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-tool-1",
		ResultNote: "first completed", OutputDigest: strings.Repeat("b", 64),
	}
	secondResult := ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-tool-2",
		ResultNote: "second completed", OutputDigest: strings.Repeat("c", 64),
	}
	lines := piRPCSequentialToolFixtureLines(
		t, request.Dispatch.MessageID(), first, firstResult, second, secondResult,
	)
	state := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			resolved: true, envelope: second, result: secondResult,
			resolvedCalls: []piToolResolvedCall{
				{Sequence: 1, Envelope: first, Result: firstResult},
				{Sequence: 2, Envelope: second, Result: secondResult},
			},
		},
	}
	for index, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &state, []byte(line),
		); err != nil {
			t.Fatalf("line[%d] %s: %v", index, line, err)
		}
	}
	if !state.settled || state.turnCount != 3 || len(state.toolResults) != 2 ||
		len(state.toolCallIDs) != 2 || state.toolEnvelopes[0] != first ||
		state.toolEnvelopes[1] != second || string(state.assistant) != "Used governed tool result." {
		t.Fatalf("sequential protocol state=%#v", state)
	}
	for _, frame := range state.frames {
		if bytes.Contains(frame.Payload(), []byte(first.Call.Command)) ||
			bytes.Contains(frame.Payload(), []byte(second.Call.Command)) {
			t.Fatal("sequential ToolCall arguments leaked to Bridge")
		}
	}
}

func TestPiRPCToolProtocolRejectsResultSubstitution(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)
	lines, envelope, result := piRPCToolFixtureLines(t, request.Dispatch.MessageID())
	for index := range lines {
		if strings.Contains(lines[index], `"type":"tool_execution_end"`) {
			lines[index] = strings.Replace(
				lines[index], strings.Repeat("b", 64), strings.Repeat("f", 64), 1,
			)
			break
		}
	}
	state := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			envelope: envelope, result: result, resolved: true,
		},
	}
	for _, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &state, []byte(line),
		); err != nil {
			return
		}
	}
	t.Fatal("substituted native tool result was accepted")
}

func TestPiRPCToolProtocolAcceptsBoundReadContentWithoutBridgeLeak(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)
	privateContent := []byte("private source content from Read\n")
	digest := "sha256:" + piContextDigest(privateContent)
	envelope := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolRead, Path: "src/private.go",
		},
	}
	result := ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-read-1",
		ResultNote: "read completed", OutputDigest: digest, ContentDigest: digest,
	}
	lines := piRPCToolFixtureLinesWithResult(
		t, request.Dispatch.MessageID(), piRPCFixturePrompt,
		envelope, result, privateContent,
	)
	state := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			envelope: envelope, result: result, resolved: true,
		},
	}
	for index, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &state, []byte(line),
		); err != nil {
			t.Fatalf("line[%d]: %v", index, err)
		}
	}
	if !state.settled || state.toolWireResult.ContentDigest != digest ||
		state.toolEnvelope != envelope {
		t.Fatalf("Read protocol state=%#v", state)
	}
	for _, frame := range state.frames {
		if bytes.Contains(frame.Payload(), privateContent) ||
			bytes.Contains(frame.Payload(), []byte(envelope.Call.Path)) {
			t.Fatal("Read content or path leaked to a Bridge frame")
		}
	}

	tampered := append([]string(nil), lines...)
	for index := range tampered {
		if strings.Contains(tampered[index], `"type":"tool_execution_end"`) {
			tampered[index] = strings.Replace(
				tampered[index], "private source content from Read", "substituted source content here", 1,
			)
			break
		}
	}
	tamperedState := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			envelope: envelope, result: result, resolved: true,
		},
	}
	for _, line := range tampered {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &tamperedState, []byte(line),
		); err != nil {
			return
		}
	}
	t.Fatal("substituted Read content was accepted")
}

func TestPiRPCToolProtocolUsesFrozenRemoteToolSet(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)
	privateContent := []byte("private bounded WebSearch result\n")
	digest := "sha256:" + piContextDigest(privateContent)
	envelope := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom governance status",
		},
	}
	result := ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-web-search-1",
		ResultNote: "search completed", OutputDigest: digest, ContentDigest: digest,
	}
	lines := piRPCToolFixtureLinesWithResult(
		t, request.Dispatch.MessageID(), piRPCFixturePrompt,
		envelope, result, privateContent,
	)
	state := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			allowedTools: map[permissions.ToolKind]struct{}{
				permissions.ToolWebSearch: {},
			},
			envelope: envelope, result: result, resolved: true,
		},
	}
	for index, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &state, []byte(line),
		); err != nil {
			t.Fatalf("line[%d]: %v", index, err)
		}
	}
	if !state.settled || state.toolEnvelope != envelope ||
		state.toolWireResult.ContentDigest != digest {
		t.Fatalf("remote tool protocol state=%#v", state)
	}
	for _, frame := range state.frames {
		if bytes.Contains(frame.Payload(), privateContent) ||
			bytes.Contains(frame.Payload(), []byte(envelope.Call.Path)) {
			t.Fatal("remote result or query leaked to a Bridge frame")
		}
	}

	localOnly := piRPCState{
		toolMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
		toolExtension: &piToolExtension{
			envelope: envelope, result: result, resolved: true,
		},
	}
	for _, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &localOnly, []byte(line),
		); err != nil {
			return
		}
	}
	t.Fatal("local-only transcript accepted WebSearch")
}

func TestPiRPCHybridProtocolLocksSelectedRoute(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)

	t.Run("governed tool", func(t *testing.T) {
		lines, envelope, result := piRPCToolFixtureLines(t, request.Dispatch.MessageID())
		state := piRPCState{
			contextMode: true, toolMode: true,
			messageID: request.Dispatch.MessageID(), prompt: []byte(piRPCFixturePrompt),
			nextSequence: 2,
			toolExtension: &piToolExtension{
				envelope: envelope, result: result, resolved: true,
			},
		}
		for index, line := range lines {
			if err := adapter.acceptRPCLine(context.Background(), &request, &state, []byte(line)); err != nil {
				t.Fatalf("line[%d]: %v", index, err)
			}
		}
		if state.localRoute != piRPCLocalRouteTool || !state.settled {
			t.Fatalf("hybrid route=%q settled=%t", state.localRoute, state.settled)
		}
	})

	t.Run("context", func(t *testing.T) {
		lines, _ := piRPCContextFixtureLines(t, request.Dispatch.MessageID())
		state := piRPCState{
			contextMode: true, toolMode: true,
			messageID: request.Dispatch.MessageID(), prompt: []byte(piRPCFixturePrompt),
			nextSequence: 2,
		}
		for index, line := range lines {
			if err := adapter.acceptRPCLine(context.Background(), &request, &state, []byte(line)); err != nil {
				t.Fatalf("line[%d]: %v", index, err)
			}
		}
		if state.localRoute != piRPCLocalRouteContext || !state.settled {
			t.Fatalf("hybrid route=%q settled=%t", state.localRoute, state.settled)
		}
	})
}

func piRPCToolFixtureLines(
	t testing.TB,
	messageID string,
) ([]string, ToolCallEnvelope, ToolCallResult) {
	return piRPCToolFixtureLinesForPrompt(
		t, messageID, piRPCFixturePrompt, "work-tool-1",
	)
}

func piRPCToolFixtureLinesForPrompt(
	t testing.TB,
	messageID string,
	prompt string,
	workItemID string,
) ([]string, ToolCallEnvelope, ToolCallResult) {
	t.Helper()
	envelope := ToolCallEnvelope{
		JobID: workItemID,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: "printf private-tool-command",
		},
	}
	result := ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-tool-1",
		ResultNote: "completed", ExitCode: 0,
		OutputDigest: strings.Repeat("b", 64),
	}
	return piRPCToolFixtureLinesWithResult(
		t, messageID, prompt, envelope, result, nil,
	), envelope, result
}

func piRPCToolFixtureLinesWithResult(
	t testing.TB,
	messageID string,
	prompt string,
	envelope ToolCallEnvelope,
	result ToolCallResult,
	content []byte,
) []string {
	t.Helper()
	callID := "tool-call-local-1"
	firstResponseID := "response-tool-1"
	secondResponseID := "response-tool-2"
	wire, err := marshalPiToolResultPayload(envelope, result, content)
	if err != nil {
		t.Fatal(err)
	}
	details := map[string]any{
		"call_digest": permissions.ProposedCallDigest(envelope.Call),
		"tool":        string(envelope.Call.Tool), "verdict": string(result.Verdict),
		"execution_id": result.ExecutionID,
	}
	resultBytes, err := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(wire)}},
		"details": details,
	})
	if err != nil {
		t.Fatal(err)
	}
	toolResultBytes, err := json.Marshal(map[string]any{
		"role": "toolResult", "toolCallId": callID, "toolName": "loom_tool",
		"content": []map[string]string{{"type": "text", "text": string(wire)}},
		"details": details, "isError": false, "timestamp": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	user := piRPCFixtureUserMessage(prompt)
	empty := piRPCFixtureAssistantMessage("")
	toolAssistant := piRPCToolAssistantFixture(callID, envelope, firstResponseID)
	emptyText := piRPCFixtureWithResponseID(
		strings.Replace(empty, `"content":[]`, `"content":[{"type":"text","text":""}]`, 1),
		secondResponseID,
	)
	final := piRPCFixtureWithResponseID(
		piRPCFixtureAssistantMessage("Used governed tool result."), secondResponseID,
	)
	toolResult := string(toolResultBytes)
	return []string{
		`{"id":"` + messageID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + user + `}`,
		`{"type":"message_end","message":` + user + `}`,
		`{"type":"message_start","message":` + empty + `}`,
		`{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + toolAssistant + `}}`,
		`{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_end","contentIndex":0,"toolCall":` + piRPCToolCallFixture(callID, envelope) + `,"partial":` + toolAssistant + `}}`,
		`{"type":"message_end","message":` + toolAssistant + `}`,
		`{"type":"tool_execution_start","toolCallId":"` + callID + `","toolName":"loom_tool","args":` + piRPCToolEnvelopeFixture(envelope) + `}`,
		`{"type":"tool_execution_end","toolCallId":"` + callID + `","toolName":"loom_tool","result":` + string(resultBytes) + `,"isError":false}`,
		`{"type":"message_start","message":` + toolResult + `}`,
		`{"type":"message_end","message":` + toolResult + `}`,
		`{"type":"turn_end","message":` + toolAssistant + `,"toolResults":[` + toolResult + `]}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + empty + `}`,
		`{"type":"message_update","message":` + emptyText + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyText + `}}`,
		`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Used governed tool result.","partial":` + final + `}}`,
		`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Used governed tool result.","partial":` + final + `}}`,
		`{"type":"message_end","message":` + final + `}`,
		`{"type":"turn_end","message":` + final + `,"toolResults":[]}`,
		`{"type":"agent_end","messages":[` + user + `,` + toolAssistant + `,` + toolResult + `,` + final + `],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}
}

func piRPCSequentialToolFixtureLines(
	t testing.TB,
	messageID string,
	first ToolCallEnvelope,
	firstResult ToolCallResult,
	second ToolCallEnvelope,
	secondResult ToolCallResult,
) []string {
	return piRPCSequentialToolFixtureLinesForPrompt(
		t,
		messageID,
		piRPCFixturePrompt,
		first,
		firstResult,
		second,
		secondResult,
	)
}

func piRPCSequentialToolFixtureLinesForPrompt(
	t testing.TB,
	messageID string,
	prompt string,
	first ToolCallEnvelope,
	firstResult ToolCallResult,
	second ToolCallEnvelope,
	secondResult ToolCallResult,
) []string {
	t.Helper()
	firstLines := piRPCToolFixtureLinesWithResult(
		t, messageID, prompt, first, firstResult, nil,
	)
	secondLines := piRPCToolFixtureLinesWithResult(
		t, messageID, prompt, second, secondResult, nil,
	)
	for index := range secondLines {
		secondLines[index] = strings.ReplaceAll(
			secondLines[index], "tool-call-local-1", "tool-call-local-2",
		)
		secondLines[index] = strings.ReplaceAll(
			secondLines[index], "response-tool-1", "response-tool-3",
		)
	}
	firstMessages := piRPCAgentEndMessages(t, firstLines[len(firstLines)-2])
	secondMessages := piRPCAgentEndMessages(t, secondLines[len(secondLines)-2])
	allMessages := []json.RawMessage{
		firstMessages[0], firstMessages[1], firstMessages[2],
		secondMessages[1], secondMessages[2], firstMessages[3],
	}
	encodedMessages, err := json.Marshal(allMessages)
	if err != nil {
		t.Fatal(err)
	}
	lines := append([]string(nil), firstLines[:14]...)
	lines = append(lines, secondLines[2])
	lines = append(lines, secondLines[5:14]...)
	lines = append(lines, firstLines[14:21]...)
	lines = append(lines,
		`{"type":"agent_end","messages":`+string(encodedMessages)+`,"willRetry":false}`,
		firstLines[len(firstLines)-1],
	)
	return lines
}

func piRPCAgentEndMessages(t testing.TB, line string) []json.RawMessage {
	t.Helper()
	var record struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal([]byte(line), &record); err != nil || len(record.Messages) != 4 {
		t.Fatalf("invalid agent_end fixture: %v %s", err, line)
	}
	return record.Messages
}

func piRPCToolAssistantFixture(
	callID string,
	envelope ToolCallEnvelope,
	responseID string,
) string {
	empty := piRPCFixtureAssistantMessage("")
	message := strings.Replace(
		empty, `"content":[]`, `"content":[`+piRPCToolCallFixture(callID, envelope)+`]`, 1,
	)
	message = strings.Replace(message, `"stopReason":"stop"`, `"stopReason":"toolUse"`, 1)
	return piRPCFixtureWithResponseID(message, responseID)
}

func piRPCToolCallFixture(callID string, envelope ToolCallEnvelope) string {
	return `{"type":"toolCall","id":"` + callID + `","name":"loom_tool","arguments":` +
		piRPCToolEnvelopeFixture(envelope) + `}`
}

func piRPCToolEnvelopeFixture(envelope ToolCallEnvelope) string {
	body, _ := json.Marshal(envelope)
	return string(body)
}
