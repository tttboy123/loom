//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPiRPCContextProtocolAcceptsOneNativeReadThenFinalText(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)
	request := fixture.request(t)
	lines, content := piRPCContextFixtureLines(t, request.Dispatch.MessageID())
	state := piRPCState{
		contextMode: true, messageID: request.Dispatch.MessageID(),
		prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
	}
	for index, line := range lines {
		if err := adapter.acceptRPCLine(
			context.Background(), &request, &state, []byte(line),
		); err != nil {
			t.Fatalf("line[%d] %s: %v", index, line, err)
		}
	}
	if !state.settled || state.turnCount != 2 || string(state.assistant) != "Used bounded context." ||
		state.contextProposal.ItemID != "diff-detail" ||
		state.contextResult.ContentDigest != piContextDigest(content) ||
		len(state.frames) != 2 {
		t.Fatalf("context protocol state = %#v frames=%d", state, len(state.frames))
	}
	for _, frame := range state.frames {
		if bytes.Contains(frame.Payload(), content) {
			t.Fatal("retrieved Context content leaked to a Bridge frame")
		}
	}
	accounting, err := piRPCCombinedAccounting(state.contextAssistant, state.finalAssistant)
	if err != nil || !accounting.UsageObserved || !accounting.CostObserved {
		t.Fatalf("combined accounting = %#v, %v", accounting, err)
	}
}

func TestPiRPCContextProtocolRejectsSecondToolAndResultDrift(t *testing.T) {
	fixture := newPiRPCBridgeFixture(t, "success")
	runtimeAdapter, err := NewPiRPCBridgeAdapter(fixture.config())
	if err != nil {
		t.Fatal(err)
	}
	adapter := runtimeAdapter.(*piRPCBridgeAdapter)

	for _, test := range []struct {
		name   string
		mutate func([]string) []string
	}{
		{name: "result content digest drift", mutate: func(lines []string) []string {
			for index := range lines {
				if strings.Contains(lines[index], `"type":"tool_execution_end"`) {
					lines[index] = strings.Replace(lines[index], "bounded observed context", "tampered observed context", 1)
					break
				}
			}
			return lines
		}},
		{name: "second turn tool call", mutate: func(lines []string) []string {
			for index := range lines {
				if strings.Contains(lines[index], `"assistantMessageEvent":{"type":"text_start"`) {
					tool := piRPCContextToolAssistantFixture(
						"tool-call-2", "diff-detail", strings.Repeat("a", 64), "", "response-context-2",
					)
					lines[index] = `{"type":"message_update","message":` + tool +
						`,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + tool + `}}`
					break
				}
			}
			return lines
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := fixture.request(t)
			lines, _ := piRPCContextFixtureLines(t, request.Dispatch.MessageID())
			lines = test.mutate(lines)
			state := piRPCState{
				contextMode: true, messageID: request.Dispatch.MessageID(),
				prompt: []byte(piRPCFixturePrompt), nextSequence: 2,
			}
			rejected := false
			for _, line := range lines {
				if err := adapter.acceptRPCLine(
					context.Background(), &request, &state, []byte(line),
				); err != nil {
					rejected = true
					break
				}
			}
			if !rejected {
				t.Fatal("drifted Context tool sequence was accepted")
			}
		})
	}
}

func piRPCContextFixtureLines(t testing.TB, messageID string) ([]string, []byte) {
	t.Helper()
	content := []byte("bounded observed context")
	digest := piContextDigest(content)
	callID := "tool-call-context-1"
	firstResponseID := "response-context-1"
	secondResponseID := "response-context-2"
	user := piRPCFixtureUserMessage(piRPCFixturePrompt)
	empty := piRPCFixtureAssistantMessage("")
	toolAssistant := piRPCContextToolAssistantFixture(
		callID, "diff-detail", digest, "artifact:diff-1", firstResponseID,
	)
	resultTextBytes, err := json.Marshal(map[string]any{
		"schema_version": 1, "status": "succeeded", "item_id": "diff-detail",
		"content_digest": digest, "trust": "observed", "scope": "artifact_scoped",
		"source_type": "observation", "source_ref": "evidence:diff-1",
		"artifact_ref": "artifact:diff-1", "content": string(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	resultBytes, err := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(resultTextBytes)}},
		"details": map[string]string{"item_id": "diff-detail", "content_digest": digest},
	})
	if err != nil {
		t.Fatal(err)
	}
	toolResultBytes, err := json.Marshal(map[string]any{
		"role": "toolResult", "toolCallId": callID, "toolName": "loom_read_context",
		"content": []map[string]string{{"type": "text", "text": string(resultTextBytes)}},
		"details": map[string]string{"item_id": "diff-detail", "content_digest": digest},
		"isError": false, "timestamp": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := string(resultBytes)
	toolResult := string(toolResultBytes)
	emptyText := piRPCFixtureWithResponseID(
		strings.Replace(empty, `"content":[]`, `"content":[{"type":"text","text":""}]`, 1),
		secondResponseID,
	)
	final := piRPCFixtureWithResponseID(
		piRPCFixtureAssistantMessage("Used bounded context."), secondResponseID,
	)
	return []string{
		`{"id":"` + messageID + `","type":"response","command":"prompt","success":true}`,
		`{"type":"agent_start"}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + user + `}`,
		`{"type":"message_end","message":` + user + `}`,
		`{"type":"message_start","message":` + empty + `}`,
		`{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_start","contentIndex":0,"partial":` + toolAssistant + `}}`,
		`{"type":"message_update","message":` + toolAssistant + `,"assistantMessageEvent":{"type":"toolcall_end","contentIndex":0,"toolCall":` + piRPCContextToolCallFixture(callID, "diff-detail", digest, "artifact:diff-1") + `,"partial":` + toolAssistant + `}}`,
		`{"type":"message_end","message":` + toolAssistant + `}`,
		`{"type":"tool_execution_start","toolCallId":"` + callID + `","toolName":"loom_read_context","args":` + piRPCContextProposalFixture("diff-detail", digest, "artifact:diff-1") + `}`,
		`{"type":"tool_execution_end","toolCallId":"` + callID + `","toolName":"loom_read_context","result":` + result + `,"isError":false}`,
		`{"type":"message_start","message":` + toolResult + `}`,
		`{"type":"message_end","message":` + toolResult + `}`,
		`{"type":"turn_end","message":` + toolAssistant + `,"toolResults":[` + toolResult + `]}`,
		`{"type":"turn_start"}`,
		`{"type":"message_start","message":` + empty + `}`,
		`{"type":"message_update","message":` + emptyText + `,"assistantMessageEvent":{"type":"text_start","contentIndex":0,"partial":` + emptyText + `}}`,
		`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Used bounded context.","partial":` + final + `}}`,
		`{"type":"message_update","message":` + final + `,"assistantMessageEvent":{"type":"text_end","contentIndex":0,"content":"Used bounded context.","partial":` + final + `}}`,
		`{"type":"message_end","message":` + final + `}`,
		`{"type":"turn_end","message":` + final + `,"toolResults":[]}`,
		`{"type":"agent_end","messages":[` + user + `,` + toolAssistant + `,` + toolResult + `,` + final + `],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}, content
}

func piRPCContextToolAssistantFixture(
	callID string,
	itemID string,
	digest string,
	artifact string,
	responseID string,
) string {
	empty := piRPCFixtureAssistantMessage("")
	message := strings.Replace(
		empty, `"content":[]`, `"content":[`+piRPCContextToolCallFixture(callID, itemID, digest, artifact)+`]`, 1,
	)
	message = strings.Replace(message, `"stopReason":"stop"`, `"stopReason":"toolUse"`, 1)
	return piRPCFixtureWithResponseID(message, responseID)
}

func piRPCContextToolCallFixture(callID, itemID, digest, artifact string) string {
	return `{"type":"toolCall","id":"` + callID + `","name":"loom_read_context","arguments":` +
		piRPCContextProposalFixture(itemID, digest, artifact) + `}`
}

func piRPCContextProposalFixture(itemID, digest, artifact string) string {
	value := `{"item_id":"` + itemID + `","content_digest":"` + digest + `"`
	if artifact != "" {
		value += `,"artifact_ref":"` + artifact + `"`
	}
	return value + `}`
}
