package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidAgentInbox   = errors.New("invalid Agent inbox")
	ErrAgentInboxAuthority = errors.New("Agent inbox authority unavailable")
	ErrAgentInboxConflict  = errors.New("Agent inbox conflict")
)

type AgentInboxInputRecord struct {
	agentinbox.Binding
	Status             agentinbox.Status
	AdmissionEventID   string
	ConsumptionEventID string
}

type AgentInboxSnapshot struct {
	Binding     AttemptLoopBinding
	Inputs      []AgentInboxInputRecord
	Loop        AttemptLoopSnapshot
	LastEventID string
	Sequence    int64
}

type AgentInboxAuthority struct {
	loops *AttemptLoopAuthority
}

func NewAgentInboxAuthority(loops *AttemptLoopAuthority) (*AgentInboxAuthority, error) {
	if loops == nil || loops.runs == nil || loops.runs.store == nil || loops.payloads == nil {
		return nil, ErrInvalidAgentInbox
	}
	return &AgentInboxAuthority{loops: loops}, nil
}

func (authority *AgentInboxAuthority) admit(
	ctx context.Context,
	binding AttemptLoopBinding,
	input agentinbox.Binding,
) (AgentInboxSnapshot, error) {
	run, loop, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AgentInboxSnapshot{}, err
	}
	if existing, found := agentInboxInputByID(state, input.InputID); found {
		if existing.Binding == input {
			return state, nil
		}
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	if !validAgentInboxInput(binding, input) || loop.Status != AttemptLoopRunning ||
		input.OrderKey != int64(len(state.Inputs)+1) ||
		!validAgentInboxAdmissionTarget(loop, state, input) {
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	now, timeErr := authority.loops.runs.operationTime()
	if timeErr != nil {
		return AgentInboxSnapshot{}, errors.Join(ErrAgentInboxAuthority, timeErr)
	}
	payload := agentInboxEventPayload{
		Attempt: attemptLoopPayload(binding), Input: input,
		Status: agentinbox.StatusPending,
	}
	eventID := agentInboxEventID("AgentInputAdmitted", payload)
	causationID := state.LastEventID
	if causationID == "" {
		causationID = run.lastEventID
	}
	event := newEvent(
		eventID, agentInboxStream(binding), state.Sequence+1,
		"AgentInputAdmitted", now, binding.PayloadAuthority.IncidentID,
		causationID, payload,
	)
	if err := authority.append(ctx, run, loop, state, []journal.Event{event}); err != nil {
		return AgentInboxSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

// StartQueuedTurn consumes one exact Queue input in the same Journal batch as
// the immutable Turn transition. No UI or runtime observer can move it later.
func (authority *AgentInboxAuthority) startQueuedTurn(
	ctx context.Context,
	binding AttemptLoopBinding,
	input agentinbox.Binding,
	turn AttemptLoopTurnInput,
) (AgentInboxSnapshot, error) {
	run, loop, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AgentInboxSnapshot{}, err
	}
	record, found := agentInboxInputByID(state, input.InputID)
	if !found || record.Binding != input || input.Mode != agentinbox.ModeQueue ||
		turn.TurnID != input.TargetTurnID || turn.Sequence != input.TargetTurnSequence ||
		turn.InputDigest != input.ContentDigest || !validAttemptLoopTurnInput(turn) {
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	if record.Status == agentinbox.StatusConsumed {
		if existing, ok := attemptLoopTurnByID(loop, turn.TurnID); ok &&
			existing.Sequence == turn.Sequence && existing.InputDigest == turn.InputDigest {
			return state, nil
		}
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	if loop.Status != AttemptLoopRunning || attemptLoopHasOpenTurn(loop) ||
		len(loop.Turns) == 0 || len(loop.Turns) >= loop.Budget.MaxTurns ||
		loop.Turns[len(loop.Turns)-1].Status != AttemptTurnSucceeded ||
		turn.Sequence != len(loop.Turns)+1 ||
		!agentInboxQueueIsNext(state, record.InputID) {
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	now, timeErr := authority.loops.runs.operationTime()
	if timeErr != nil {
		return AgentInboxSnapshot{}, errors.Join(ErrAgentInboxAuthority, timeErr)
	}
	loopPayload := attemptLoopPayload(binding)
	loopPayload.TurnID, loopPayload.TurnSequence = turn.TurnID, turn.Sequence
	loopPayload.TurnInputDigest = turn.InputDigest
	loopEventID := attemptLoopEventID("TurnStarted", loopPayload)
	loopEvent := newEvent(
		loopEventID, attemptLoopStream(binding), loop.Sequence+1, "TurnStarted", now,
		binding.PayloadAuthority.IncidentID, loop.LastEventID, loopPayload,
	)
	inboxEvent := authority.consumptionEvent(
		binding, state, record, now, loopEventID,
	)
	if err := authority.append(
		ctx, run, loop, state, []journal.Event{loopEvent, inboxEvent},
	); err != nil {
		return AgentInboxSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

// StartTargetStep consumes all exact Steer/Inject inputs frozen for the next
// Step. Omitting a pending input for that target fails closed.
func (authority *AgentInboxAuthority) startTargetStep(
	ctx context.Context,
	binding AttemptLoopBinding,
	inputs []agentinbox.Binding,
	step AttemptLoopStepInput,
) (AgentInboxSnapshot, error) {
	run, loop, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AgentInboxSnapshot{}, err
	}
	records, allConsumed, selectErr := selectAgentInboxStepInputs(state, inputs, step)
	if selectErr != nil || len(records) == 0 || !validAttemptLoopStepInput(step) {
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	if allConsumed {
		if existing, ok := attemptLoopStepByID(loop, step.StepID); ok &&
			existing.Sequence == step.Sequence && existing.ModelInputDigest == step.ModelInputDigest {
			return state, nil
		}
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	turn, ok := attemptLoopOpenTurn(loop, step.TurnID)
	if !ok || loop.Status != AttemptLoopRunning || attemptLoopHasOpenStep(*turn) ||
		len(turn.Steps) >= loop.Budget.MaxStepsPerTurn ||
		step.Sequence != len(turn.Steps)+1 ||
		(step.Sequence > 1 && turn.Steps[len(turn.Steps)-1].Outcome != AttemptStepContinue) {
		return AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	now, timeErr := authority.loops.runs.operationTime()
	if timeErr != nil {
		return AgentInboxSnapshot{}, errors.Join(ErrAgentInboxAuthority, timeErr)
	}
	loopPayload := attemptLoopPayload(binding)
	loopPayload.TurnID, loopPayload.StepID = step.TurnID, step.StepID
	loopPayload.StepSequence, loopPayload.ModelInputDigest = step.Sequence, step.ModelInputDigest
	loopEventID := attemptLoopEventID("StepStarted", loopPayload)
	events := []journal.Event{newEvent(
		loopEventID, attemptLoopStream(binding), loop.Sequence+1, "StepStarted", now,
		binding.PayloadAuthority.IncidentID, loop.LastEventID, loopPayload,
	)}
	inboxState := state
	causationID := loopEventID
	for _, record := range records {
		event := authority.consumptionEvent(binding, inboxState, record, now, causationID)
		events = append(events, event)
		inboxState.LastEventID, inboxState.Sequence = event.ID, event.Seq
		causationID = event.ID
	}
	if err := authority.append(ctx, run, loop, state, events); err != nil {
		return AgentInboxSnapshot{}, err
	}
	return authority.Snapshot(ctx, binding)
}

func (authority *AgentInboxAuthority) Snapshot(
	ctx context.Context,
	binding AttemptLoopBinding,
) (AgentInboxSnapshot, error) {
	_, loop, state, err := authority.commandState(ctx, binding)
	if err != nil {
		return AgentInboxSnapshot{}, err
	}
	state.Loop = loop
	return cloneAgentInboxSnapshot(state), nil
}

func (authority *AgentInboxAuthority) commandState(
	ctx context.Context,
	binding AttemptLoopBinding,
) (RunRecord, AttemptLoopSnapshot, AgentInboxSnapshot, error) {
	if authority == nil || authority.loops == nil || ctx == nil {
		return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{}, ErrInvalidAgentInbox
	}
	run, _, err := authority.loops.commandState(ctx, binding)
	if err != nil {
		if errors.Is(err, ErrInvalidAttemptLoop) {
			return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{}, ErrInvalidAgentInbox
		}
		return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{},
			errors.Join(ErrAgentInboxAuthority, err)
	}
	loopStreamID := attemptLoopStream(binding)
	inboxStreamID := agentInboxStream(binding)
	streamSet, err := authority.loops.runs.store.ReadStreamSet(
		ctx, []string{loopStreamID, inboxStreamID},
	)
	if err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{},
			errors.Join(ErrAgentInboxAuthority, err)
	}
	loopEvents := make([]journal.Event, 0)
	inboxEvents := make([]journal.Event, 0)
	for _, event := range streamSet.Events() {
		switch event.StreamID {
		case loopStreamID:
			loopEvents = append(loopEvents, event)
		case inboxStreamID:
			inboxEvents = append(inboxEvents, event)
		default:
			return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{},
				ErrAgentInboxConflict
		}
	}
	loop, err := replayAttemptLoop(binding, loopEvents)
	if err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{}, err
	}
	state, err := replayAgentInbox(binding, inboxEvents)
	if err != nil {
		return RunRecord{}, AttemptLoopSnapshot{}, AgentInboxSnapshot{}, err
	}
	state.Loop = loop
	return run, loop, state, nil
}

func (authority *AgentInboxAuthority) consumptionEvent(
	binding AttemptLoopBinding,
	state AgentInboxSnapshot,
	record AgentInboxInputRecord,
	now time.Time,
	transitionEventID string,
) journal.Event {
	payload := agentInboxEventPayload{
		Attempt: attemptLoopPayload(binding), Input: record.Binding,
		Status: agentinbox.StatusConsumed, TransitionEventID: transitionEventID,
	}
	eventID := agentInboxEventID("AgentInputConsumed", payload)
	return newEvent(
		eventID, agentInboxStream(binding), state.Sequence+1,
		"AgentInputConsumed", now, binding.PayloadAuthority.IncidentID,
		state.LastEventID, payload,
	)
}

func (authority *AgentInboxAuthority) append(
	ctx context.Context,
	run RunRecord,
	loop AttemptLoopSnapshot,
	state AgentInboxSnapshot,
	events []journal.Event,
) error {
	_, err := authority.loops.runs.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runStream(run.id), Sequence: run.streamSequence},
			{StreamID: attemptLoopStream(state.Binding), Sequence: loop.Sequence},
			{StreamID: agentInboxStream(state.Binding), Sequence: state.Sequence},
		},
		events,
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return ErrAgentInboxConflict
	}
	return errors.Join(ErrAgentInboxAuthority, err)
}

type agentInboxEventPayload struct {
	Attempt           attemptLoopEventPayload `json:"attempt"`
	Input             agentinbox.Binding      `json:"input"`
	Status            agentinbox.Status       `json:"status"`
	TransitionEventID string                  `json:"transition_event_id,omitempty"`
}

func replayAgentInbox(
	binding AttemptLoopBinding,
	events []journal.Event,
) (AgentInboxSnapshot, error) {
	state := AgentInboxSnapshot{Binding: binding}
	streamID := agentInboxStream(binding)
	for index, event := range events {
		var payload agentInboxEventPayload
		if event.StreamID != streamID || event.Seq != int64(index+1) ||
			event.SchemaVersion != 1 ||
			event.CorrelationID != binding.PayloadAuthority.IncidentID ||
			event.ID != event.IdempotencyKey || event.CausationID == "" ||
			decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			!reflect.DeepEqual(payload.Attempt, attemptLoopPayload(binding)) ||
			event.ID != agentInboxEventID(event.Type, payload) ||
			(index > 0 && event.CausationID != state.LastEventID) {
			return AgentInboxSnapshot{}, ErrAgentInboxConflict
		}
		switch event.Type {
		case "AgentInputAdmitted":
			if payload.Status != agentinbox.StatusPending || payload.TransitionEventID != "" ||
				!validAgentInboxInput(binding, payload.Input) ||
				payload.Input.OrderKey != int64(len(state.Inputs)+1) {
				return AgentInboxSnapshot{}, ErrAgentInboxConflict
			}
			if _, found := agentInboxInputByID(state, payload.Input.InputID); found {
				return AgentInboxSnapshot{}, ErrAgentInboxConflict
			}
			state.Inputs = append(state.Inputs, AgentInboxInputRecord{
				Binding: payload.Input, Status: payload.Status, AdmissionEventID: event.ID,
			})
		case "AgentInputConsumed":
			if payload.Status != agentinbox.StatusConsumed ||
				!validOpaqueID(payload.TransitionEventID) {
				return AgentInboxSnapshot{}, ErrAgentInboxConflict
			}
			record, found := agentInboxInputByIDMutable(&state, payload.Input.InputID)
			if !found || record.Binding != payload.Input || record.Status != agentinbox.StatusPending {
				return AgentInboxSnapshot{}, ErrAgentInboxConflict
			}
			record.Status, record.ConsumptionEventID = agentinbox.StatusConsumed, event.ID
		default:
			return AgentInboxSnapshot{}, ErrAgentInboxConflict
		}
		state.LastEventID, state.Sequence = event.ID, event.Seq
	}
	return cloneAgentInboxSnapshot(state), nil
}

func validAgentInboxAdmissionTarget(
	loop AttemptLoopSnapshot,
	state AgentInboxSnapshot,
	input agentinbox.Binding,
) bool {
	switch input.Mode {
	case agentinbox.ModeQueue:
		if input.TargetStepID != "" || input.TargetStepSequence != 0 ||
			input.TargetTurnSequence != len(loop.Turns)+pendingQueueCount(state)+1 {
			return false
		}
		return !agentInboxTargetExists(state, input.TargetTurnID, "")
	case agentinbox.ModeSteer, agentinbox.ModeInject:
		if input.TargetTurnID != "" || input.TargetTurnSequence != 0 {
			return false
		}
		turn, ok := attemptLoopOpenTurn(loop, currentAgentInboxTurnID(loop))
		if !ok || len(turn.Steps) >= loop.Budget.MaxStepsPerTurn ||
			(len(turn.Steps) > 0 && !attemptLoopHasOpenStep(*turn) &&
				turn.Steps[len(turn.Steps)-1].Outcome != AttemptStepContinue) {
			return false
		}
		expected := len(turn.Steps) + 1
		if input.TargetStepSequence != expected {
			return false
		}
		for _, record := range state.Inputs {
			if record.Status == agentinbox.StatusPending &&
				(record.Mode == agentinbox.ModeSteer || record.Mode == agentinbox.ModeInject) &&
				record.TargetStepSequence == expected && record.TargetStepID != input.TargetStepID {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func selectAgentInboxStepInputs(
	state AgentInboxSnapshot,
	inputs []agentinbox.Binding,
	step AttemptLoopStepInput,
) ([]AgentInboxInputRecord, bool, error) {
	if len(inputs) == 0 {
		return nil, false, ErrAgentInboxConflict
	}
	records := make([]AgentInboxInputRecord, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	allConsumed := true
	consumedCount := 0
	for _, input := range inputs {
		if _, duplicate := seen[input.InputID]; duplicate ||
			(input.Mode != agentinbox.ModeSteer && input.Mode != agentinbox.ModeInject) ||
			input.TargetStepID != step.StepID || input.TargetStepSequence != step.Sequence {
			return nil, false, ErrAgentInboxConflict
		}
		seen[input.InputID] = struct{}{}
		record, found := agentInboxInputByID(state, input.InputID)
		if !found || record.Binding != input {
			return nil, false, ErrAgentInboxConflict
		}
		allConsumed = allConsumed && record.Status == agentinbox.StatusConsumed
		if record.Status == agentinbox.StatusConsumed {
			consumedCount++
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].OrderKey < records[j].OrderKey })
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusPending &&
			(record.Mode == agentinbox.ModeSteer || record.Mode == agentinbox.ModeInject) &&
			record.TargetStepID == step.StepID && record.TargetStepSequence == step.Sequence {
			if _, included := seen[record.InputID]; !included {
				return nil, false, ErrAgentInboxConflict
			}
		}
	}
	if consumedCount != 0 && consumedCount != len(records) {
		return nil, false, ErrAgentInboxConflict
	}
	return records, allConsumed, nil
}

func validAgentInboxInput(binding AttemptLoopBinding, input agentinbox.Binding) bool {
	if !validOpaqueID(input.PayloadID) || !validOpaqueID(input.InputID) ||
		!validOpaqueID(input.ConversationID) || !validOpaqueID(input.SegmentID) ||
		!validOpaqueID(input.AgentInstanceID) || !validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) || input.ClaimGeneration <= 0 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validSHA256Hex(input.ExecutionBindingDigest) ||
		!validSHA256Hex(input.CapsuleDigest) || input.OrderKey <= 0 ||
		input.ContentType != "text/plain" || !validSHA256Hex(input.ContentDigest) {
		return false
	}
	authority := binding.PayloadAuthority
	if input.ConversationID != authority.ConversationID ||
		input.AgentInstanceID != authority.AgentInstanceID ||
		input.WorkItemID != authority.WorkItemID || input.RunID != authority.RunID ||
		input.ClaimGeneration != authority.ClaimGeneration ||
		input.RuntimeInstanceID != authority.RuntimeInstanceID ||
		input.ExecutionBindingDigest != authority.ExecutionBindingDigest ||
		input.CapsuleDigest != authority.CapsuleDigest {
		return false
	}
	switch input.ContextScope {
	case agentinbox.ScopeConversationShared, agentinbox.ScopeTeamShared,
		agentinbox.ScopeAgentPrivate:
		if input.ScopeTargetID != "" {
			return false
		}
	case agentinbox.ScopeRoleRestricted, agentinbox.ScopeArtifact,
		agentinbox.ScopeSecretReference:
		if !validOpaqueID(input.ScopeTargetID) {
			return false
		}
	default:
		return false
	}
	if input.Mode == agentinbox.ModeQueue {
		return validOpaqueID(input.TargetTurnID) && input.TargetTurnSequence > 0
	}
	return (input.Mode == agentinbox.ModeSteer || input.Mode == agentinbox.ModeInject) &&
		validOpaqueID(input.TargetStepID) && input.TargetStepSequence > 0
}

func agentInboxQueueIsNext(state AgentInboxSnapshot, inputID string) bool {
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusPending && record.Mode == agentinbox.ModeQueue {
			return record.InputID == inputID
		}
	}
	return false
}

func pendingQueueCount(state AgentInboxSnapshot) int {
	count := 0
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusPending && record.Mode == agentinbox.ModeQueue {
			count++
		}
	}
	return count
}

func agentInboxTargetExists(state AgentInboxSnapshot, turnID, stepID string) bool {
	for _, record := range state.Inputs {
		if turnID != "" && record.TargetTurnID == turnID ||
			stepID != "" && record.TargetStepID == stepID {
			return true
		}
	}
	return false
}

func currentAgentInboxTurnID(loop AttemptLoopSnapshot) string {
	if len(loop.Turns) == 0 {
		return ""
	}
	return loop.Turns[len(loop.Turns)-1].TurnID
}

func agentInboxInputByID(
	state AgentInboxSnapshot,
	inputID string,
) (AgentInboxInputRecord, bool) {
	for _, record := range state.Inputs {
		if record.InputID == inputID {
			return record, true
		}
	}
	return AgentInboxInputRecord{}, false
}

func agentInboxInputByIDMutable(
	state *AgentInboxSnapshot,
	inputID string,
) (*AgentInboxInputRecord, bool) {
	for index := range state.Inputs {
		if state.Inputs[index].InputID == inputID {
			return &state.Inputs[index], true
		}
	}
	return nil, false
}

func cloneAgentInboxSnapshot(state AgentInboxSnapshot) AgentInboxSnapshot {
	state.Inputs = append([]AgentInboxInputRecord(nil), state.Inputs...)
	state.Loop = cloneAttemptLoopSnapshot(state.Loop)
	return state
}

func agentInboxStream(binding AttemptLoopBinding) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/agent-inbox/v1", attemptLoopStream(binding),
		binding.PayloadAuthority.AgentInstanceID,
		binding.PayloadAuthority.ExecutionBindingDigest,
		binding.PayloadAuthority.CapsuleDigest,
	}, "\x00")))
	return "agent-inbox/" + hex.EncodeToString(digest[:16])
}

func agentInboxEventID(eventType string, payload agentInboxEventPayload) string {
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(encoded)
	return deterministicEventID(eventType, hex.EncodeToString(digest[:]))
}
