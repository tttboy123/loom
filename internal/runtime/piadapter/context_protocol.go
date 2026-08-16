package piadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

type piRPCContextStage uint8

const (
	piRPCContextAwaitResponse piRPCContextStage = iota
	piRPCContextAwaitAgentStart
	piRPCContextAwaitFirstTurn
	piRPCContextAwaitUserStart
	piRPCContextAwaitUserEnd
	piRPCContextAwaitToolAssistantStart
	piRPCContextAwaitToolAssistantUpdate
	piRPCContextAwaitToolAssistantEnd
	piRPCContextAwaitToolExecutionStart
	piRPCContextAwaitToolExecutionEnd
	piRPCContextAwaitToolResultStart
	piRPCContextAwaitToolResultEnd
	piRPCContextAwaitFirstTurnEnd
	piRPCContextAwaitSecondTurn
	piRPCContextAwaitFinalAssistantStart
	piRPCContextAwaitFinalAssistantUpdate
	piRPCContextAwaitFinalAssistantEnd
	piRPCContextAwaitSecondTurnEnd
	piRPCContextAwaitAgentEnd
	piRPCContextAwaitSettled
)

type piRPCContextResultReceipt struct {
	ItemID        string
	ContentDigest string
	ArtifactRef   string
	Trust         contextcapsule.TrustClass
	Scope         contextcapsule.Scope
	SourceType    contextcapsule.SourceType
	SourceRef     string
	ResultDigest  string
}

func (adapter *piRPCBridgeAdapter) acceptContextRPCRecord(
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
		complete, err := state.acceptContextToolUpdate(
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
			!piRPCContextToolAssistant(fields["message"], state.contextToolCallID, state.contextProposal) ||
			!state.assistantIdentity.acceptTerminal(fields["message"]) {
			return ErrPiRPCProtocol
		}
		state.contextAssistant = bytes.Clone(fields["message"])
		state.messageOpen = false
		state.contextStage++
		return nil
	case piRPCContextAwaitToolExecutionStart:
		if recordType != "tool_execution_start" ||
			!piRPCExactKeys(fields, "type", "toolCallId", "toolName", "args") ||
			!piRPCContextToolIdentity(fields, state.contextToolCallID, state.contextProposal) {
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
		if !ok || isError || !piRPCContextToolNameAndID(fields, state.contextToolCallID) {
			return ErrPiRPCProtocol
		}
		receipt, err := piRPCContextResult(fields["result"], state.contextProposal)
		if err != nil {
			return err
		}
		state.contextResult = receipt
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
		receipt, err := piRPCContextToolResultMessage(
			fields["message"], state.contextToolCallID, state.contextProposal,
		)
		if err != nil || receipt != state.contextResult {
			return ErrPiRPCProtocol
		}
		state.contextStage++
		return nil
	case piRPCContextAwaitFirstTurnEnd:
		if recordType != "turn_end" ||
			!piRPCExactKeys(fields, "type", "message", "toolResults") ||
			!piRPCSemanticEqual(fields["message"], state.contextAssistant) {
			return ErrPiRPCProtocol
		}
		var results []json.RawMessage
		if json.Unmarshal(fields["toolResults"], &results) != nil || len(results) != 1 {
			return ErrPiRPCProtocol
		}
		receipt, err := piRPCContextToolResultMessage(
			results[0], state.contextToolCallID, state.contextProposal,
		)
		if err != nil || receipt != state.contextResult {
			return ErrPiRPCProtocol
		}
		state.turnOpen = false
		state.contextStage++
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
		if eventErr != nil || !eventOK ||
			eventType != "text_start" && eventType != "text_delta" && eventType != "text_end" {
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
		if !ok || willRetry || !state.validContextAgentMessages(fields["messages"]) {
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

func (state *piRPCState) acceptContextToolUpdate(
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
		if state.toolStarted {
			state.contextRejectPoint = "tool_start_duplicate"
			return false, ErrPiRPCProtocol
		}
		if !state.assistantIdentity.acceptUpdate(fields["partial"], true) {
			state.contextRejectPoint = "tool_start_identity"
			return false, ErrPiRPCProtocol
		}
		if state.contextInitialRID != "" &&
			state.assistantIdentity.responseID != state.contextInitialRID {
			state.contextRejectPoint = "tool_start_response"
			return false, ErrPiRPCProtocol
		}
		state.toolStarted = true
		return false, nil
	case "toolcall_delta":
		if !state.toolStarted || state.toolEnded ||
			!validPiContextToolDelta(fields["delta"]) ||
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
		callID, proposal, err := piRPCContextToolCall(fields["toolCall"])
		if err != nil || !piRPCContextToolAssistant(message, callID, proposal) ||
			!piRPCContextToolAssistant(fields["partial"], callID, proposal) {
			return false, ErrPiRPCProtocol
		}
		state.contextToolCallID = callID
		state.contextProposal = proposal
		state.toolEnded = true
		return true, nil
	default:
		return false, ErrPiRPCProtocol
	}
}

func validPiContextToolDelta(raw json.RawMessage) bool {
	value, ok := piRPCString(map[string]json.RawMessage{"delta": raw}, "delta")
	return ok && value != "" && len(value) <= 2048 && utf8.ValidString(value) &&
		!piContextContentHasControls([]byte(value))
}

func piRPCContextInitialAssistantIdentity(
	raw json.RawMessage,
) (piRPCAssistantIdentityState, string, bool) {
	var identity piRPCAssistantIdentityState
	snapshot, ok := piRPCAssistantIdentitySnapshotOf(raw)
	if !ok || snapshot.responseModel || snapshot.cacheWrite1h || snapshot.reasoning {
		return identity, "", false
	}
	identity.timestamp = snapshot.timestamp
	return identity, snapshot.responseID, true
}

func piRPCContextToolCall(raw json.RawMessage) (string, contextcapsule.RetrievalProposal, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(fields, "type", "id", "name", "arguments") {
		return "", contextcapsule.RetrievalProposal{}, ErrPiRPCProtocol
	}
	kind, kindOK := piRPCString(fields, "type")
	id, idOK := piRPCString(fields, "id")
	name, nameOK := piRPCString(fields, "name")
	proposal, proposalOK := piRPCContextProposal(fields["arguments"])
	if !kindOK || !idOK || !nameOK || kind != "toolCall" ||
		name != "loom_read_context" || !validPiContextIdentifier(id, 512) || !proposalOK {
		return "", contextcapsule.RetrievalProposal{}, ErrPiRPCProtocol
	}
	return id, proposal, nil
}

func piRPCContextProposal(raw json.RawMessage) (contextcapsule.RetrievalProposal, bool) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCAllowedKeys(fields, []string{"item_id", "content_digest"}, "artifact_ref") {
		return contextcapsule.RetrievalProposal{}, false
	}
	itemID, itemOK := piRPCString(fields, "item_id")
	digest, digestOK := piRPCString(fields, "content_digest")
	artifact, artifactPresent := piRPCString(fields, "artifact_ref")
	if !itemOK || !digestOK || !validPiContextIdentifier(itemID, 512) ||
		len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" ||
		artifactPresent && !validPiContextIdentifier(artifact, 512) {
		return contextcapsule.RetrievalProposal{}, false
	}
	return contextcapsule.RetrievalProposal{
		ItemID: itemID, ContentDigest: digest, ArtifactRef: artifact,
	}, true
}

func piRPCContextToolAssistant(
	raw json.RawMessage,
	callID string,
	proposal contextcapsule.RetrievalProposal,
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
	stop, ok := piRPCString(fields, "stopReason")
	if !roleOK || !apiOK || !providerOK || !modelOK || !ok ||
		role != "assistant" || api != "openai-completions" ||
		provider != piRPCProviderID || model != piRPCModelID || stop != "toolUse" ||
		!piRPCUsage(fields["usage"]) ||
		!piRPCNonNegativeNumber(fields["timestamp"]) {
		return false
	}
	var content []json.RawMessage
	if json.Unmarshal(fields["content"], &content) != nil || len(content) != 1 {
		return false
	}
	id, found, err := piRPCContextToolCall(content[0])
	return err == nil && found == proposal && id == callID
}

func piRPCContextToolNameAndID(fields map[string]json.RawMessage, callID string) bool {
	id, idOK := piRPCString(fields, "toolCallId")
	name, nameOK := piRPCString(fields, "toolName")
	return idOK && nameOK && id == callID && name == "loom_read_context"
}

func piRPCContextToolIdentity(
	fields map[string]json.RawMessage,
	callID string,
	proposal contextcapsule.RetrievalProposal,
) bool {
	parsed, ok := piRPCContextProposal(fields["args"])
	return ok && parsed == proposal && piRPCContextToolNameAndID(fields, callID)
}

func piRPCContextResult(
	raw json.RawMessage,
	proposal contextcapsule.RetrievalProposal,
) (piRPCContextResultReceipt, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(fields, "content", "details") {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	details, err := piRPCObject(fields["details"])
	if err != nil || !piRPCExactKeys(details, "item_id", "content_digest") {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	itemID, itemOK := piRPCString(details, "item_id")
	digest, digestOK := piRPCString(details, "content_digest")
	if !itemOK || !digestOK || itemID != proposal.ItemID || digest != proposal.ContentDigest {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	var blocks []json.RawMessage
	if json.Unmarshal(fields["content"], &blocks) != nil || len(blocks) != 1 {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	block, err := piRPCObject(blocks[0])
	if err != nil || !piRPCExactKeys(block, "type", "text") {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	kind, kindOK := piRPCString(block, "type")
	text, textOK := piRPCString(block, "text")
	if !kindOK || !textOK || kind != "text" || len(text) > piContextExtensionMaxResultBytes ||
		!utf8.ValidString(text) {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	inner, err := piRPCContextInnerResult([]byte(text), proposal)
	if err != nil {
		return piRPCContextResultReceipt{}, err
	}
	sum := sha256.Sum256([]byte(text))
	inner.ResultDigest = hex.EncodeToString(sum[:])
	return inner, nil
}

func piRPCContextInnerResult(
	raw []byte,
	proposal contextcapsule.RetrievalProposal,
) (piRPCContextResultReceipt, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCAllowedKeys(fields, []string{
		"schema_version", "status", "item_id", "content_digest", "trust", "scope",
		"source_type", "source_ref", "content",
	}, "artifact_ref") || !bytes.Equal(fields["schema_version"], []byte("1")) {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	status, statusOK := piRPCString(fields, "status")
	itemID, itemOK := piRPCString(fields, "item_id")
	digest, digestOK := piRPCString(fields, "content_digest")
	trustRaw, trustOK := piRPCString(fields, "trust")
	scopeRaw, scopeOK := piRPCString(fields, "scope")
	sourceRaw, sourceOK := piRPCString(fields, "source_type")
	sourceRef, sourceRefOK := piRPCString(fields, "source_ref")
	content, contentOK := piRPCString(fields, "content")
	artifact, artifactPresent := piRPCString(fields, "artifact_ref")
	trust := contextcapsule.TrustClass(trustRaw)
	scope := contextcapsule.Scope(scopeRaw)
	source := contextcapsule.SourceType(sourceRaw)
	if !statusOK || !itemOK || !digestOK || !trustOK || !scopeOK || !sourceOK ||
		!sourceRefOK || !contentOK || status != "succeeded" || itemID != proposal.ItemID ||
		digest != proposal.ContentDigest || artifact != proposal.ArtifactRef ||
		artifactPresent != (proposal.ArtifactRef != "") || len(content) == 0 ||
		len(content) > piContextExtensionMaxContentBytes || piContextDigest([]byte(content)) != digest ||
		!utf8.ValidString(content) || piContextContentHasControls([]byte(content)) ||
		!validPiContextIdentifier(sourceRef, 512) || !piRPCContextClassification(trust, scope, source) {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	return piRPCContextResultReceipt{
		ItemID: itemID, ContentDigest: digest, ArtifactRef: artifact,
		Trust: trust, Scope: scope, SourceType: source, SourceRef: sourceRef,
	}, nil
}

func piRPCContextClassification(
	trust contextcapsule.TrustClass,
	scope contextcapsule.Scope,
	source contextcapsule.SourceType,
) bool {
	if scope == contextcapsule.ScopeSecretReferenceOnly ||
		source == contextcapsule.SourceCredentialReference {
		return false
	}
	switch source {
	case contextcapsule.SourceAuthority:
		return trust == contextcapsule.TrustAuthoritative
	case contextcapsule.SourceObservation:
		return trust == contextcapsule.TrustObserved
	case contextcapsule.SourceModelOutput:
		return trust == contextcapsule.TrustUntrusted
	default:
		return false
	}
}

func piRPCContextToolResultMessage(
	raw json.RawMessage,
	callID string,
	proposal contextcapsule.RetrievalProposal,
) (piRPCContextResultReceipt, error) {
	fields, err := piRPCObject(raw)
	if err != nil || !piRPCExactKeys(
		fields, "role", "toolCallId", "toolName", "content", "details", "isError", "timestamp",
	) || !piRPCContextToolNameAndID(fields, callID) || !piRPCNonNegativeNumber(fields["timestamp"]) {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	role, roleOK := piRPCString(fields, "role")
	isError, errorOK := piRPCBool(fields, "isError")
	if !roleOK || !errorOK || role != "toolResult" || isError {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	result, err := json.Marshal(struct {
		Content json.RawMessage `json:"content"`
		Details json.RawMessage `json:"details"`
	}{fields["content"], fields["details"]})
	if err != nil {
		return piRPCContextResultReceipt{}, ErrPiRPCProtocol
	}
	defer zeroPiRPCBytes(result)
	return piRPCContextResult(result, proposal)
}

func (state *piRPCState) validContextAgentMessages(raw json.RawMessage) bool {
	var messages []json.RawMessage
	if json.Unmarshal(raw, &messages) != nil || len(messages) != 4 ||
		!piRPCUserMessage(messages[0], state.prompt) ||
		!piRPCSemanticEqual(messages[1], state.contextAssistant) ||
		!piRPCSemanticEqual(messages[3], state.finalAssistant) {
		return false
	}
	receipt, err := piRPCContextToolResultMessage(
		messages[2], state.contextToolCallID, state.contextProposal,
	)
	return err == nil && receipt == state.contextResult
}

func piRPCCombinedAccounting(
	first json.RawMessage,
	second json.RawMessage,
) (work.RunAccounting, error) {
	return piRPCMessagesAccounting(first, second)
}

func piRPCMessagesAccounting(messages ...json.RawMessage) (work.RunAccounting, error) {
	if len(messages) == 0 {
		return work.RunAccounting{}, ErrPiRPCProtocol
	}
	add := func(left, right int64) (int64, bool) {
		return left + right, right <= math.MaxInt64-left
	}
	result := work.RunAccounting{}
	for _, message := range messages {
		current, err := piRPCFinalAccounting(message)
		if err != nil {
			return work.RunAccounting{}, err
		}
		var ok bool
		if result.InputTokens, ok = add(result.InputTokens, current.InputTokens); !ok {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		if result.OutputTokens, ok = add(result.OutputTokens, current.OutputTokens); !ok {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		if result.CacheReadTokens, ok = add(result.CacheReadTokens, current.CacheReadTokens); !ok {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		if result.CacheWriteTokens, ok = add(result.CacheWriteTokens, current.CacheWriteTokens); !ok {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		if result.TotalTokens, ok = add(result.TotalTokens, current.TotalTokens); !ok {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		if result.CostMicrounits, ok = add(result.CostMicrounits, current.CostMicrounits); !ok {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		result.UsageObserved = result.UsageObserved || current.UsageObserved
		result.CostObserved = result.CostObserved || current.CostObserved
		if result.CostCurrency == "" {
			result.CostCurrency = current.CostCurrency
		} else if current.CostCurrency != result.CostCurrency {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
		if result.CostSource == "" {
			result.CostSource = current.CostSource
		} else if current.CostSource != result.CostSource {
			return work.RunAccounting{}, ErrPiRPCProtocol
		}
	}
	if err := work.ValidateRunAccounting(result); err != nil {
		return work.RunAccounting{}, errors.Join(ErrPiRPCProtocol, err)
	}
	return result, nil
}

func zeroPiRPCBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
