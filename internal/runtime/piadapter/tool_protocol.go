//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func (adapter *piRPCBridgeAdapter) acceptHybridRPCRecord(
	ctx context.Context,
	request *supervisor.AdapterRequest,
	state *piRPCState,
	fields map[string]json.RawMessage,
	recordType string,
) error {
	switch state.localRoute {
	case piRPCLocalRouteContext:
		return adapter.acceptContextRPCRecord(ctx, request, state, fields, recordType)
	case piRPCLocalRouteTool:
		return adapter.acceptToolRPCRecord(ctx, request, state, fields, recordType)
	}
	if state.contextStage != piRPCContextAwaitToolAssistantUpdate ||
		recordType != "message_update" {
		return adapter.acceptContextRPCRecord(ctx, request, state, fields, recordType)
	}
	rawEvent, ok := fields["assistantMessageEvent"]
	if !ok {
		return ErrPiRPCProtocol
	}
	eventFields, err := piRPCObject(rawEvent)
	if err != nil {
		return ErrPiRPCProtocol
	}
	event, ok := piRPCString(eventFields, "type")
	if !ok || event != "toolcall_end" {
		return adapter.acceptContextRPCRecord(ctx, request, state, fields, recordType)
	}
	rawCall, ok := eventFields["toolCall"]
	if !ok {
		return ErrPiRPCProtocol
	}
	callFields, err := piRPCObject(rawCall)
	if err != nil {
		return ErrPiRPCProtocol
	}
	name, ok := piRPCString(callFields, "name")
	if !ok {
		return ErrPiRPCProtocol
	}
	switch name {
	case "loom_read_context":
		state.localRoute = piRPCLocalRouteContext
		return adapter.acceptContextRPCRecord(ctx, request, state, fields, recordType)
	case "loom_tool":
		state.localRoute = piRPCLocalRouteTool
		return adapter.acceptToolRPCRecord(ctx, request, state, fields, recordType)
	default:
		return ErrPiRPCProtocol
	}
}

func (adapter *piRPCBridgeAdapter) acceptToolRPCRecord(
	ctx context.Context,
	request *supervisor.AdapterRequest,
	state *piRPCState,
	fields map[string]json.RawMessage,
	recordType string,
) error {
	switch state.contextStage {
	case piRPCContextAwaitResponse:
		if recordType != "response" ||
			!piRPCExactKeys(fields, "id", "type", "command", "success") {
			return ErrPiRPCProtocol
		}
		id, idOK := piRPCString(fields, "id")
		command, commandOK := piRPCString(fields, "command")
		success, successOK := piRPCBool(fields, "success")
		if !idOK || !commandOK || !successOK || id != state.messageID ||
			command != "prompt" || !success || request == nil {
			return ErrPiRPCProtocol
		}
		frame, err := adapter.execution.outboundFrame(
			*request, state.nextSequence, bridgev1.MessageAck,
			mustPiPayload(map[string]string{"message_id": request.Dispatch.MessageID()}),
		)
		if err != nil || request.FrameSink.AcceptFrame(ctx, frame) != nil {
			return ErrPiRPCProtocol
		}
		state.frames = append(state.frames, frame)
		state.nextSequence++
		state.dispatchAcknowledged = true
		state.responseSeen = true
		state.contextStage++
		return nil
	case piRPCContextAwaitAgentStart:
		if recordType != "agent_start" || !piRPCExactKeys(fields, "type") {
			return ErrPiRPCProtocol
		}
		state.agentStarted = true
		state.contextStage++
		return nil
	case piRPCContextAwaitFirstTurn, piRPCContextAwaitSecondTurn:
		if recordType != "turn_start" || !piRPCExactKeys(fields, "type") {
			return ErrPiRPCProtocol
		}
		state.turnOpen = true
		state.turnCount++
		if state.turnCount > piMaxSequentialToolCalls+1 {
			return ErrPiRPCProtocol
		}
		state.contextStage++
		return nil
	case piRPCContextAwaitUserStart:
		if recordType != "message_start" || !piRPCExactKeys(fields, "type", "message") ||
			!piRPCUserMessage(fields["message"], state.prompt) {
			return ErrPiRPCProtocol
		}
		state.messageOpen = true
		state.messageRole = "user"
		state.userSeen = true
		state.contextStage++
		return nil
	case piRPCContextAwaitUserEnd:
		if recordType != "message_end" || !piRPCExactKeys(fields, "type", "message") ||
			!piRPCUserMessage(fields["message"], state.prompt) {
			return ErrPiRPCProtocol
		}
		state.messageOpen = false
		state.messageRole = ""
		state.contextStage++
		return nil
	case piRPCContextAwaitToolAssistantStart:
		if recordType != "message_start" || !piRPCExactKeys(fields, "type", "message") {
			return ErrPiRPCProtocol
		}
		identity, responseID, ok := piRPCContextInitialAssistantIdentity(fields["message"])
		if !ok {
			return ErrPiRPCProtocol
		}
		state.assistantIdentity = identity
		state.contextInitialRID = responseID
		state.messageOpen = true
		state.messageRole = "assistant"
		state.assistantSeen = true
		state.contextStage++
		return nil
	case piRPCContextAwaitToolAssistantUpdate:
		if recordType != "message_update" ||
			!piRPCExactKeys(fields, "type", "message", "assistantMessageEvent") {
			return ErrPiRPCProtocol
		}
		complete, err := state.acceptLocalToolUpdate(
			fields["message"], fields["assistantMessageEvent"],
		)
		if err != nil {
			return err
		}
		if complete {
			state.contextStage++
		}
		return nil
	case piRPCContextAwaitToolAssistantEnd:
		if recordType != "message_end" || !piRPCExactKeys(fields, "type", "message") ||
			!piRPCLocalToolAssistant(
				fields["message"], state.toolCallID, state.toolEnvelope,
				state.allowedToolCalls(),
			) ||
			!state.assistantIdentity.acceptTerminal(fields["message"]) {
			return ErrPiRPCProtocol
		}
		state.toolAssistant = bytes.Clone(fields["message"])
		state.messageOpen = false
		state.contextStage++
		return nil
	case piRPCContextAwaitToolExecutionStart:
		if recordType != "tool_execution_start" ||
			!piRPCExactKeys(fields, "type", "toolCallId", "toolName", "args") ||
			!piRPCLocalToolIdentity(
				fields, state.toolCallID, state.toolEnvelope, state.allowedToolCalls(),
			) {
			return ErrPiRPCProtocol
		}
		state.contextStage++
		return nil
	case piRPCContextAwaitToolExecutionEnd:
		if recordType != "tool_execution_end" ||
			!piRPCExactKeys(fields, "type", "toolCallId", "toolName", "result", "isError") {
			return ErrPiRPCProtocol
		}
		isError, ok := piRPCBool(fields, "isError")
		if !ok || isError || !piRPCLocalToolNameAndID(fields, state.toolCallID) {
			return ErrPiRPCProtocol
		}
		result, err := piRPCLocalToolResult(fields["result"], state.toolEnvelope)
		if err != nil || state.toolExtension == nil ||
			!state.toolExtension.matchesWire(state.toolEnvelope, result) {
			return ErrPiRPCProtocol
		}
		state.toolWireResult = result
		audit := ToolCallAuditEntry{
			Verdict: result.Verdict,
			Tool:    string(state.toolEnvelope.Call.Tool),
		}
		if result.Verdict == permissions.VerdictAllow {
			audit.ExecutionID = result.ExecutionID
			audit.ResultNote = result.ResultNote
		} else {
			audit.DenialReason = result.DenialReason
		}
		state.toolResults = append(state.toolResults, audit)
		state.contextStage++
		return nil
	case piRPCContextAwaitToolResultStart, piRPCContextAwaitToolResultEnd:
		want := "message_start"
		if state.contextStage == piRPCContextAwaitToolResultEnd {
			want = "message_end"
		}
		if recordType != want || !piRPCExactKeys(fields, "type", "message") {
			return ErrPiRPCProtocol
		}
		result, err := piRPCLocalToolResultMessage(
			fields["message"], state.toolCallID, state.toolEnvelope,
		)
		if err != nil || !samePiToolResult(result, state.toolWireResult) {
			return ErrPiRPCProtocol
		}
		if state.contextStage == piRPCContextAwaitToolResultEnd {
			state.toolResultMessages = append(
				state.toolResultMessages, bytes.Clone(fields["message"]),
			)
		}
		state.contextStage++
		return nil
	case piRPCContextAwaitFirstTurnEnd:
		if recordType != "turn_end" ||
			!piRPCExactKeys(fields, "type", "message", "toolResults") ||
			!piRPCSemanticEqual(fields["message"], state.toolAssistant) {
			return ErrPiRPCProtocol
		}
		var results []json.RawMessage
		if json.Unmarshal(fields["toolResults"], &results) != nil || len(results) != 1 {
			return ErrPiRPCProtocol
		}
		result, err := piRPCLocalToolResultMessage(results[0], state.toolCallID, state.toolEnvelope)
		if err != nil || !samePiToolResult(result, state.toolWireResult) {
			return ErrPiRPCProtocol
		}
		if len(state.toolCallIDs) >= piMaxSequentialToolCalls ||
			len(state.toolResultMessages) != len(state.toolCallIDs)+1 {
			return ErrPiRPCProtocol
		}
		for _, prior := range state.toolCallIDs {
			if prior == state.toolCallID {
				return ErrPiRPCProtocol
			}
		}
		state.toolCallIDs = append(state.toolCallIDs, state.toolCallID)
		state.toolEnvelopes = append(state.toolEnvelopes, state.toolEnvelope)
		state.toolWireResults = append(state.toolWireResults, state.toolWireResult)
		state.toolAssistants = append(
			state.toolAssistants, bytes.Clone(state.toolAssistant),
		)
		state.turnOpen = false
		state.contextStage = piRPCContextAwaitSecondTurn
		return nil
	case piRPCContextAwaitFinalAssistantStart:
		if recordType != "message_start" || !piRPCExactKeys(fields, "type", "message") ||
			!piRPCAssistantTextMessage(fields["message"], "", false) {
			return ErrPiRPCProtocol
		}
		identity, responseID, ok := piRPCContextInitialAssistantIdentity(fields["message"])
		if !ok {
			return ErrPiRPCProtocol
		}
		state.assistantIdentity = identity
		state.contextInitialRID = responseID
		state.assistant = nil
		state.lastPartial = nil
		state.textStarted = false
		state.textEnded = false
		state.toolStarted = false
		state.toolEnded = false
		state.doneSeen = false
		state.messageOpen = true
		state.contextStage++
		return nil
	case piRPCContextAwaitFinalAssistantUpdate:
		if recordType != "message_update" ||
			!piRPCExactKeys(fields, "type", "message", "assistantMessageEvent") {
			return ErrPiRPCProtocol
		}
		eventFields, eventErr := piRPCObject(fields["assistantMessageEvent"])
		eventType, eventOK := piRPCString(eventFields, "type")
		if eventErr != nil || !eventOK {
			return ErrPiRPCProtocol
		}
		if eventType == "toolcall_start" || eventType == "toolcall_delta" ||
			eventType == "toolcall_end" {
			if len(state.toolCallIDs) >= piMaxSequentialToolCalls || state.textStarted {
				return ErrPiRPCProtocol
			}
			complete, err := state.acceptLocalToolUpdate(
				fields["message"], fields["assistantMessageEvent"],
			)
			if err != nil {
				return err
			}
			if complete {
				state.contextStage = piRPCContextAwaitToolAssistantEnd
			}
			return nil
		}
		if eventType != "text_start" && eventType != "text_delta" && eventType != "text_end" ||
			state.toolStarted {
			return ErrPiRPCProtocol
		}
		if err := adapter.acceptAssistantEvent(
			ctx, request, state, fields["message"], fields["assistantMessageEvent"],
		); err != nil {
			return err
		}
		if eventType == "text_start" && state.contextInitialRID != "" &&
			state.assistantIdentity.responseID != state.contextInitialRID {
			return ErrPiRPCProtocol
		}
		if state.textEnded {
			state.contextStage++
		}
		return nil
	case piRPCContextAwaitFinalAssistantEnd:
		if recordType != "message_end" || !piRPCExactKeys(fields, "type", "message") ||
			len(state.assistant) == 0 ||
			!piRPCAssistantTextMessage(fields["message"], string(state.assistant), true) ||
			!state.assistantIdentity.acceptTerminal(fields["message"]) {
			return ErrPiRPCProtocol
		}
		state.finalAssistant = bytes.Clone(fields["message"])
		state.doneSeen = true
		state.messageOpen = false
		state.contextStage++
		return nil
	case piRPCContextAwaitSecondTurnEnd:
		if recordType != "turn_end" ||
			!piRPCExactKeys(fields, "type", "message", "toolResults") ||
			!piRPCSemanticEqual(fields["message"], state.finalAssistant) ||
			!piRPCEmptyArray(fields["toolResults"]) {
			return ErrPiRPCProtocol
		}
		state.turnOpen = false
		state.contextStage++
		return nil
	case piRPCContextAwaitAgentEnd:
		if recordType != "agent_end" ||
			!piRPCExactKeys(fields, "type", "messages", "willRetry") {
			return ErrPiRPCProtocol
		}
		willRetry, ok := piRPCBool(fields, "willRetry")
		if !ok || willRetry || !state.validLocalToolAgentMessages(fields["messages"]) {
			return ErrPiRPCProtocol
		}
		state.agentEnded = true
		state.contextStage++
		return nil
	case piRPCContextAwaitSettled:
		if recordType != "agent_settled" || !piRPCExactKeys(fields, "type") {
			return ErrPiRPCProtocol
		}
		state.settled = true
		state.contextStage++
		return nil
	default:
		return ErrPiRPCProtocol
	}
}

func (state *piRPCState) acceptLocalToolUpdate(
	message json.RawMessage,
	raw json.RawMessage,
) (bool, error) {
	fields, err := piRPCObject(raw)
	if err != nil {
		return false, ErrPiRPCProtocol
	}
	event, ok := piRPCString(fields, "type")
	if !ok || !piRPCAssistantEventShape(fields, piRPCDiagnosticEventOf(event)) ||
		!piRPCZero(fields["contentIndex"]) {
		return false, ErrPiRPCProtocol
	}
	switch event {
	case "toolcall_start":
		if state.toolStarted || !state.assistantIdentity.acceptUpdate(fields["partial"], true) {
			return false, ErrPiRPCProtocol
		}
		if state.contextInitialRID != "" &&
			state.assistantIdentity.responseID != state.contextInitialRID {
			return false, ErrPiRPCProtocol
		}
		state.toolStarted = true
		return false, nil
	case "toolcall_delta":
		if !state.toolStarted || state.toolEnded || !validPiContextToolDelta(fields["delta"]) ||
			!state.assistantIdentity.acceptUpdate(fields["partial"], false) {
			return false, ErrPiRPCProtocol
		}
		return false, nil
	case "toolcall_end":
		if !state.toolStarted || state.toolEnded ||
			!piRPCSemanticEqual(message, fields["partial"]) ||
			!state.assistantIdentity.acceptUpdate(fields["partial"], false) {
			return false, ErrPiRPCProtocol
		}
		allowed := state.allowedToolCalls()
		callID, envelope, err := piRPCLocalToolCall(fields["toolCall"], allowed)
		if err != nil || !piRPCLocalToolAssistant(message, callID, envelope, allowed) ||
			!piRPCLocalToolAssistant(fields["partial"], callID, envelope, allowed) {
			return false, ErrPiRPCProtocol
		}
		state.toolCallID = callID
		state.toolEnvelope = envelope
		state.toolEnded = true
		return true, nil
	default:
		return false, ErrPiRPCProtocol
	}
}

func (state *piRPCState) allowedToolCalls() map[permissions.ToolKind]struct{} {
	if state != nil && state.toolExtension != nil && len(state.toolExtension.allowedTools) > 0 {
		return state.toolExtension.allowedTools
	}
	return bridgeAllowedTools
}

func piRPCLocalToolCall(
	raw json.RawMessage,
	allowed map[permissions.ToolKind]struct{},
) (string, ToolCallEnvelope, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(fields, "type", "id", "name", "arguments") {
		return "", ToolCallEnvelope{}, ErrPiRPCProtocol
	}
	kind, kindOK := piRPCString(fields, "type")
	id, idOK := piRPCString(fields, "id")
	name, nameOK := piRPCString(fields, "name")
	arguments := bytes.Clone(fields["arguments"])
	defer zeroPiRPCBytes(arguments)
	envelope, decodeErr := decodeToolCallEnvelope(arguments, allowed)
	if !kindOK || !idOK || !nameOK || kind != "toolCall" || name != "loom_tool" ||
		!validPiContextIdentifier(id, 512) || decodeErr != nil {
		return "", ToolCallEnvelope{}, ErrPiRPCProtocol
	}
	return id, envelope, nil
}

func piRPCLocalToolAssistant(
	raw json.RawMessage,
	callID string,
	envelope ToolCallEnvelope,
	allowed map[permissions.ToolKind]struct{},
) bool {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCAllowedKeys(fields, []string{
		"role", "content", "api", "provider", "model", "usage", "stopReason", "timestamp",
	}, "responseId") {
		return false
	}
	role, roleOK := piRPCString(fields, "role")
	api, apiOK := piRPCString(fields, "api")
	provider, providerOK := piRPCString(fields, "provider")
	model, modelOK := piRPCString(fields, "model")
	stop, stopOK := piRPCString(fields, "stopReason")
	if !roleOK || !apiOK || !providerOK || !modelOK || !stopOK || role != "assistant" ||
		api != "openai-completions" || provider != piRPCProviderID || model != piRPCModelID ||
		stop != "toolUse" || !piRPCUsage(fields["usage"]) ||
		!piRPCNonNegativeNumber(fields["timestamp"]) {
		return false
	}
	var content []json.RawMessage
	if json.Unmarshal(fields["content"], &content) != nil || len(content) != 1 {
		return false
	}
	id, found, err := piRPCLocalToolCall(content[0], allowed)
	return err == nil && id == callID && found == envelope
}

func piRPCLocalToolNameAndID(fields map[string]json.RawMessage, callID string) bool {
	id, idOK := piRPCString(fields, "toolCallId")
	name, nameOK := piRPCString(fields, "toolName")
	return idOK && nameOK && id == callID && name == "loom_tool"
}

func piRPCLocalToolIdentity(
	fields map[string]json.RawMessage,
	callID string,
	envelope ToolCallEnvelope,
	allowed map[permissions.ToolKind]struct{},
) bool {
	arguments := bytes.Clone(fields["args"])
	defer zeroPiRPCBytes(arguments)
	parsed, err := decodeToolCallEnvelope(arguments, allowed)
	return err == nil && parsed == envelope && piRPCLocalToolNameAndID(fields, callID)
}

func piRPCLocalToolResult(
	raw json.RawMessage,
	envelope ToolCallEnvelope,
) (ToolCallResult, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(fields, "content", "details") {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	details, err := piRPCObject(fields["details"])
	if err != nil || !piRPCAllowedKeys(
		details, []string{"call_digest", "tool", "verdict"}, "execution_id",
	) {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	digest, digestOK := piRPCString(details, "call_digest")
	tool, toolOK := piRPCString(details, "tool")
	verdict, verdictOK := piRPCString(details, "verdict")
	executionID, executionPresent := piRPCString(details, "execution_id")
	if !digestOK || !toolOK || !verdictOK ||
		digest != permissions.ProposedCallDigest(envelope.Call) || tool != string(envelope.Call.Tool) ||
		(verdict == string(permissions.VerdictAllow)) != executionPresent ||
		verdict != string(permissions.VerdictAllow) && verdict != string(permissions.VerdictDeny) ||
		executionPresent && executionID == "" {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	var blocks []json.RawMessage
	if json.Unmarshal(fields["content"], &blocks) != nil || len(blocks) != 1 {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	block, err := piRPCObject(blocks[0])
	if err != nil || !piRPCExactKeys(block, "type", "text") {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	kind, kindOK := piRPCString(block, "type")
	text, textOK := piRPCString(block, "text")
	if !kindOK || !textOK || kind != "text" || len(text) > piToolExtensionMaxResultBytes {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	wireEnvelope, result, err := decodePiToolExtensionResponse([]byte(text))
	if err != nil || wireEnvelope.Call.Tool != envelope.Call.Tool ||
		result.Verdict != permissions.Verdict(verdict) || result.ExecutionID != executionID {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	return result, nil
}

func piRPCLocalToolResultMessage(
	raw json.RawMessage,
	callID string,
	envelope ToolCallEnvelope,
) (ToolCallResult, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(
		fields, "role", "toolCallId", "toolName", "content", "details", "isError", "timestamp",
	) || !piRPCLocalToolNameAndID(fields, callID) ||
		!piRPCNonNegativeNumber(fields["timestamp"]) {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	role, roleOK := piRPCString(fields, "role")
	isError, errorOK := piRPCBool(fields, "isError")
	if !roleOK || !errorOK || role != "toolResult" || isError {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	result, err := json.Marshal(struct {
		Content json.RawMessage `json:"content"`
		Details json.RawMessage `json:"details"`
	}{fields["content"], fields["details"]})
	if err != nil {
		return ToolCallResult{}, ErrPiRPCProtocol
	}
	defer zeroPiRPCBytes(result)
	return piRPCLocalToolResult(result, envelope)
}

func (state *piRPCState) validLocalToolAgentMessages(raw json.RawMessage) bool {
	var messages []json.RawMessage
	toolCount := len(state.toolCallIDs)
	if json.Unmarshal(raw, &messages) != nil || toolCount < 1 ||
		len(state.toolEnvelopes) != toolCount || len(state.toolWireResults) != toolCount ||
		len(state.toolAssistants) != toolCount || len(state.toolResultMessages) != toolCount ||
		len(messages) != 2*toolCount+2 || !piRPCUserMessage(messages[0], state.prompt) ||
		!piRPCSemanticEqual(messages[len(messages)-1], state.finalAssistant) {
		return false
	}
	for index := 0; index < toolCount; index++ {
		assistantIndex := 1 + index*2
		resultIndex := assistantIndex + 1
		if !piRPCSemanticEqual(messages[assistantIndex], state.toolAssistants[index]) ||
			!piRPCSemanticEqual(messages[resultIndex], state.toolResultMessages[index]) {
			return false
		}
		result, err := piRPCLocalToolResultMessage(
			messages[resultIndex], state.toolCallIDs[index], state.toolEnvelopes[index],
		)
		if err != nil || !samePiToolResult(result, state.toolWireResults[index]) {
			return false
		}
	}
	return true
}

func (extension *piToolExtension) matchesWire(
	envelope ToolCallEnvelope,
	wire ToolCallResult,
) bool {
	if extension == nil {
		return false
	}
	extension.resultMu.Lock()
	defer extension.resultMu.Unlock()
	for _, call := range extension.resolvedCalls {
		if call.Envelope == envelope {
			return samePiToolResult(piToolWireResult(call.Result), wire)
		}
	}
	if !extension.resolved || extension.envelope != envelope {
		return false
	}
	internal := extension.result
	internal = piToolWireResult(internal)
	return samePiToolResult(internal, wire)
}
