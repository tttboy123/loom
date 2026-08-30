package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidAttemptPayloadFact   = errors.New("invalid Attempt payload fact")
	ErrAttemptPayloadFactAuthority = errors.New("Attempt payload authority unavailable")
	ErrAttemptPayloadFactConflict  = errors.New("Attempt payload fact conflict")
)

const (
	AttemptPayloadReconcileRepaired         = "repaired"
	AttemptPayloadReconcileAlreadyDelivered = "already_delivered"
	AttemptPayloadReconcileMissing          = "missing"
	AttemptPayloadReconcileBlocked          = "blocked"
)

type AttemptPayloadReconciliation struct {
	Authority        attemptpayload.Authority
	Binding          attemptpayload.Binding
	ExecutionBinding FrozenExecutionBinding
	RunPhase         string
	Result           string
	ErrorCode        string
}

type AttemptPayloadReconciliationReport struct {
	Outcomes []AttemptPayloadReconciliation
}

type AttemptPayloadAuthority struct {
	runs *Authority
}

var _ attemptpayload.FactAuthority = (*AttemptPayloadAuthority)(nil)

func NewAttemptPayloadAuthority(
	runs *Authority,
) (*AttemptPayloadAuthority, error) {
	if runs == nil || runs.store == nil {
		return nil, ErrInvalidAttemptPayloadFact
	}
	return &AttemptPayloadAuthority{runs: runs}, nil
}

func (authority *AttemptPayloadAuthority) Lookup(
	ctx context.Context,
	frozen attemptpayload.Authority,
	callID string,
	sequence int64,
) (attemptpayload.Fact, bool, error) {
	if authority == nil || !validAttemptPayloadAuthority(frozen) ||
		!validOpaqueID(callID) || sequence <= 0 {
		return attemptpayload.Fact{}, false, ErrInvalidAttemptPayloadFact
	}
	if _, err := authority.exactRun(ctx, frozen); err != nil {
		return attemptpayload.Fact{}, false, err
	}
	facts, err := authority.readFacts(ctx, frozen, callID, sequence)
	if err != nil {
		return attemptpayload.Fact{}, false, err
	}
	if !facts.found {
		if sequence > 1 {
			index, indexErr := authority.readCallIndex(ctx, frozen)
			if indexErr != nil {
				return attemptpayload.Fact{}, false, indexErr
			}
			if indexed, exists := index.bindings[sequence]; exists {
				if indexed.CallID != callID {
					return attemptpayload.Fact{}, false, ErrAttemptPayloadFactConflict
				}
				return attemptpayload.Fact{}, false, ErrAttemptPayloadFactAuthority
			}
		}
		return attemptpayload.Fact{}, false, nil
	}
	if facts.binding.CallID != callID || facts.binding.Sequence != sequence {
		return attemptpayload.Fact{}, false, ErrAttemptPayloadFactConflict
	}
	return facts.public(), true, nil
}

func (authority *AttemptPayloadAuthority) Accept(
	ctx context.Context,
	frozen attemptpayload.Authority,
	binding attemptpayload.Binding,
) error {
	return authority.accept(ctx, frozen, binding, "")
}

func (authority *AttemptPayloadAuthority) accept(
	ctx context.Context,
	frozen attemptpayload.Authority,
	binding attemptpayload.Binding,
	causationID string,
) error {
	if authority == nil || !validAttemptPayloadAuthority(frozen) ||
		!validAttemptPayloadBinding(binding) || binding.Scope != frozen.Scope {
		return ErrInvalidAttemptPayloadFact
	}
	run, err := authority.currentRun(ctx, frozen)
	if err != nil {
		return err
	}
	facts, err := authority.readFacts(
		ctx, frozen, binding.CallID, binding.Sequence,
	)
	if err != nil {
		return err
	}
	if facts.found {
		if facts.binding != binding ||
			(causationID != "" && facts.acceptedCausationID != causationID) {
			return ErrAttemptPayloadFactConflict
		}
		return nil
	}
	now, err := authority.runs.operationTime()
	if err != nil {
		return errors.Join(ErrAttemptPayloadFactAuthority, err)
	}
	payload := attemptPayloadFactPayloadFrom(
		frozen, binding, attemptpayload.FactAccepted, "",
	)
	eventID := attemptPayloadFactEventID("ToolResultAccepted", payload)
	if causationID == "" {
		causationID = run.lastEventID
	}
	event := newEvent(
		eventID, attemptPayloadFactStreamFor(
			frozen, binding.CallID, binding.Sequence,
		), 1,
		"ToolResultAccepted", now, frozen.IncidentID, causationID, payload,
	)
	if binding.Sequence > 1 {
		return authority.appendIndexedAccept(ctx, run, frozen, binding, event)
	}
	return authority.appendFact(ctx, run, 0, event)
}

func (authority *AttemptPayloadAuthority) Deliver(
	ctx context.Context,
	frozen attemptpayload.Authority,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if authority == nil || !validAttemptPayloadAuthority(frozen) ||
		!validAttemptPayloadBinding(binding) || binding.Scope != frozen.Scope ||
		!validAttemptPayloadDeliveryProof(proof) {
		return ErrInvalidAttemptPayloadFact
	}
	run, err := authority.currentRun(ctx, frozen)
	if err != nil {
		return err
	}
	facts, err := authority.readFacts(
		ctx, frozen, binding.CallID, binding.Sequence,
	)
	if err != nil {
		return err
	}
	if !facts.found || facts.binding != binding {
		return ErrAttemptPayloadFactConflict
	}
	if facts.status == attemptpayload.FactDelivered {
		if facts.proof != proof {
			return ErrAttemptPayloadFactConflict
		}
		return nil
	}
	now, err := authority.runs.operationTime()
	if err != nil {
		return errors.Join(ErrAttemptPayloadFactAuthority, err)
	}
	payload := attemptPayloadFactPayloadFrom(
		frozen, binding, attemptpayload.FactDelivered, proof,
	)
	eventID := attemptPayloadFactEventID("ToolResultDelivered", payload)
	event := newEvent(
		eventID, attemptPayloadFactStreamFor(
			frozen, binding.CallID, binding.Sequence,
		), 2,
		"ToolResultDelivered", now, frozen.IncidentID, facts.lastEventID, payload,
	)
	return authority.appendFact(ctx, run, 1, event)
}

func (authority *AttemptPayloadAuthority) ReconcileDelivered(
	ctx context.Context,
	payloads attemptpayload.Store,
) (AttemptPayloadReconciliationReport, error) {
	if authority == nil || payloads == nil || ctx == nil {
		return AttemptPayloadReconciliationReport{}, ErrInvalidAttemptPayloadFact
	}
	if err := ctx.Err(); err != nil {
		return AttemptPayloadReconciliationReport{}, errors.Join(
			ErrAttemptPayloadFactAuthority, err,
		)
	}
	facts, err := authority.deliveredFacts(ctx)
	if err != nil {
		return AttemptPayloadReconciliationReport{}, err
	}
	report := AttemptPayloadReconciliationReport{
		Outcomes: make([]AttemptPayloadReconciliation, 0, len(facts)),
	}
	validatedRuns := make(map[attemptpayload.Authority]RunRecord)
	for _, fact := range facts {
		run, validated := validatedRuns[fact.authority]
		if !validated {
			var runErr error
			run, runErr = authority.exactRun(ctx, fact.authority)
			if runErr != nil {
				return AttemptPayloadReconciliationReport{}, runErr
			}
			validatedRuns[fact.authority] = run
		}
		outcome := AttemptPayloadReconciliation{
			Authority: fact.authority, Binding: fact.state.binding,
			ExecutionBinding: run.ExecutionBinding(), RunPhase: run.Phase(),
		}
		payload, readErr := payloads.ReadAttemptPayload(ctx, fact.state.binding)
		if errors.Is(readErr, attemptpayload.ErrPayloadNotFound) {
			outcome.Result = AttemptPayloadReconcileMissing
			report.Outcomes = append(report.Outcomes, outcome)
			continue
		}
		if readErr != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return AttemptPayloadReconciliationReport{}, errors.Join(
					ErrAttemptPayloadFactAuthority, contextErr,
				)
			}
			outcome.Result = AttemptPayloadReconcileBlocked
			outcome.ErrorCode = "attempt_payload_unavailable"
			report.Outcomes = append(report.Outcomes, outcome)
			continue
		}
		status := payload.Status
		payload.Close()
		switch status {
		case attemptpayload.StatusDelivered:
			outcome.Result = AttemptPayloadReconcileAlreadyDelivered
		case attemptpayload.StatusPending:
			if markErr := payloads.MarkAttemptPayloadDelivered(
				ctx, fact.state.binding,
			); markErr != nil {
				if contextErr := ctx.Err(); contextErr != nil {
					return AttemptPayloadReconciliationReport{}, errors.Join(
						ErrAttemptPayloadFactAuthority, contextErr,
					)
				}
				if errors.Is(markErr, attemptpayload.ErrPayloadNotFound) {
					outcome.Result = AttemptPayloadReconcileMissing
				} else {
					outcome.Result = AttemptPayloadReconcileBlocked
					outcome.ErrorCode = "attempt_payload_commit_failed"
				}
			} else {
				outcome.Result = AttemptPayloadReconcileRepaired
			}
		default:
			outcome.Result = AttemptPayloadReconcileBlocked
			outcome.ErrorCode = "attempt_payload_invalid"
		}
		report.Outcomes = append(report.Outcomes, outcome)
	}
	return report, nil
}

type deliveredAttemptPayloadFact struct {
	authority attemptpayload.Authority
	state     attemptPayloadFactState
}

func (authority *AttemptPayloadAuthority) deliveredFacts(
	ctx context.Context,
) ([]deliveredAttemptPayloadFact, error) {
	events, err := authority.runs.store.ReadAll(ctx)
	if err != nil {
		return nil, errors.Join(ErrAttemptPayloadFactAuthority, err)
	}
	streams := make(map[string][]journal.Event)
	order := make([]string, 0)
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, "attempt-payload/") {
			continue
		}
		if _, found := streams[event.StreamID]; !found {
			order = append(order, event.StreamID)
		}
		streams[event.StreamID] = append(streams[event.StreamID], event)
	}
	results := make([]deliveredAttemptPayloadFact, 0, len(order))
	for _, streamID := range order {
		frozen, state, replayErr := replayFrozenAttemptPayloadFacts(streams[streamID])
		if replayErr != nil {
			return nil, replayErr
		}
		if state.status == attemptpayload.FactDelivered {
			results = append(results, deliveredAttemptPayloadFact{
				authority: frozen,
				state:     state,
			})
		}
	}
	return results, nil
}

func replayFrozenAttemptPayloadFacts(
	events []journal.Event,
) (attemptpayload.Authority, attemptPayloadFactState, error) {
	if len(events) == 0 || events[0].Type != "ToolResultAccepted" {
		return attemptpayload.Authority{}, attemptPayloadFactState{},
			ErrAttemptPayloadFactConflict
	}
	var payload attemptPayloadFactPayload
	if decodeExactPayload(events[0].PayloadJSON, &payload) != nil {
		return attemptpayload.Authority{}, attemptPayloadFactState{},
			ErrAttemptPayloadFactConflict
	}
	frozen := attemptpayload.Authority{
		Scope: payload.binding().Scope, ClaimID: payload.ClaimID,
		AgentInstanceID: payload.AgentInstanceID,
		IncidentID:      events[0].CorrelationID,
	}
	binding := payload.binding()
	if !validAttemptPayloadAuthority(frozen) ||
		events[0].StreamID != attemptPayloadFactStreamFor(
			frozen, binding.CallID, binding.Sequence,
		) {
		return attemptpayload.Authority{}, attemptPayloadFactState{},
			ErrAttemptPayloadFactConflict
	}
	state, err := replayAttemptPayloadFacts(
		frozen, binding.CallID, binding.Sequence, events,
	)
	if err != nil {
		return attemptpayload.Authority{}, attemptPayloadFactState{}, err
	}
	return frozen, state, nil
}

func (authority *AttemptPayloadAuthority) currentRun(
	ctx context.Context,
	frozen attemptpayload.Authority,
) (RunRecord, error) {
	run, err := authority.exactRun(ctx, frozen)
	if err != nil || run.phase != "running" {
		return RunRecord{}, ErrAttemptPayloadFactAuthority
	}
	return run, nil
}

func (authority *AttemptPayloadAuthority) exactRun(
	ctx context.Context,
	frozen attemptpayload.Authority,
) (RunRecord, error) {
	if ctx == nil {
		return RunRecord{}, ErrAttemptPayloadFactAuthority
	}
	if err := ctx.Err(); err != nil {
		return RunRecord{}, errors.Join(ErrAttemptPayloadFactAuthority, err)
	}
	state, err := authority.runs.readRunStateWithProviderAccount(
		ctx,
		runCommandStreams(
			frozen.WorkItemID, frozen.RunID, frozen.RuntimeInstanceID,
		),
		frozen.RunID,
	)
	if err != nil {
		return RunRecord{}, errors.Join(ErrAttemptPayloadFactAuthority, err)
	}
	run, err := currentGenerationRun(state, RunGenerationInput{
		WorkItemID: frozen.WorkItemID, RunID: frozen.RunID,
		ClaimID: frozen.ClaimID, ClaimGeneration: frozen.ClaimGeneration,
		RuntimeInstanceID: frozen.RuntimeInstanceID,
		AgentInstanceID:   frozen.AgentInstanceID,
		CorrelationID:     frozen.IncidentID,
	})
	if err != nil ||
		run.executionBinding.BindingDigest != frozen.ExecutionBindingDigest {
		return RunRecord{}, ErrAttemptPayloadFactAuthority
	}
	runEvents, err := authority.runs.store.ReadStream(ctx, runStream(run.id))
	if err != nil || len(runEvents) == 0 {
		return RunRecord{}, ErrAttemptPayloadFactAuthority
	}
	head := runEvents[len(runEvents)-1]
	if head.Seq != run.streamSequence || head.ID != run.lastEventID ||
		head.CorrelationID != frozen.IncidentID {
		return RunRecord{}, ErrAttemptPayloadFactAuthority
	}
	return run, nil
}

func (authority *AttemptPayloadAuthority) appendFact(
	ctx context.Context,
	run RunRecord,
	factHead int64,
	event journal.Event,
) error {
	_, err := authority.runs.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runStream(run.id), Sequence: run.streamSequence},
			{StreamID: event.StreamID, Sequence: factHead},
		},
		[]journal.Event{event},
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return ErrAttemptPayloadFactConflict
	}
	return errors.Join(ErrAttemptPayloadFactAuthority, err)
}

type attemptPayloadCallIndexState struct {
	head     int64
	lastID   string
	bindings map[int64]attemptpayload.Binding
}

func (authority *AttemptPayloadAuthority) appendIndexedAccept(
	ctx context.Context,
	run RunRecord,
	frozen attemptpayload.Authority,
	binding attemptpayload.Binding,
	factEvent journal.Event,
) error {
	index, err := authority.readCallIndex(ctx, frozen)
	if err != nil {
		return err
	}
	if indexed, exists := index.bindings[binding.Sequence]; exists {
		if indexed == binding {
			return ErrAttemptPayloadFactAuthority
		}
		return ErrAttemptPayloadFactConflict
	}
	if binding.Sequence != index.head+2 {
		return ErrAttemptPayloadFactConflict
	}
	if binding.Sequence == 2 {
		legacyEvents, readErr := authority.runs.store.ReadStream(
			ctx, attemptPayloadFactStream(frozen),
		)
		if readErr != nil {
			return errors.Join(ErrAttemptPayloadFactAuthority, readErr)
		}
		if len(legacyEvents) == 0 {
			return ErrAttemptPayloadFactConflict
		}
		if _, _, replayErr := replayFrozenAttemptPayloadFacts(legacyEvents); replayErr != nil {
			return replayErr
		}
	}
	indexPayload := attemptPayloadFactPayloadFrom(
		frozen, binding, attemptpayload.FactAccepted, "",
	)
	indexID := attemptPayloadCallIndexEventID(indexPayload)
	causationID := index.lastID
	if causationID == "" {
		causationID = run.lastEventID
	}
	indexEvent := newEvent(
		indexID, attemptPayloadCallIndexStream(frozen), index.head+1,
		"AttemptPayloadCallIndexed", factEvent.EmittedAt, frozen.IncidentID,
		causationID, indexPayload,
	)
	_, err = authority.runs.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: runStream(run.id), Sequence: run.streamSequence},
			{StreamID: indexEvent.StreamID, Sequence: index.head},
			{StreamID: factEvent.StreamID, Sequence: 0},
		},
		[]journal.Event{indexEvent, factEvent},
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return ErrAttemptPayloadFactConflict
	}
	return errors.Join(ErrAttemptPayloadFactAuthority, err)
}

func (authority *AttemptPayloadAuthority) readCallIndex(
	ctx context.Context,
	frozen attemptpayload.Authority,
) (attemptPayloadCallIndexState, error) {
	events, err := authority.runs.store.ReadStream(
		ctx, attemptPayloadCallIndexStream(frozen),
	)
	if err != nil {
		return attemptPayloadCallIndexState{}, errors.Join(
			ErrAttemptPayloadFactAuthority, err,
		)
	}
	return replayAttemptPayloadCallIndex(frozen, events)
}

func replayAttemptPayloadCallIndex(
	authority attemptpayload.Authority,
	events []journal.Event,
) (attemptPayloadCallIndexState, error) {
	state := attemptPayloadCallIndexState{
		bindings: make(map[int64]attemptpayload.Binding, len(events)),
	}
	streamID := attemptPayloadCallIndexStream(authority)
	seenCalls := make(map[string]struct{}, len(events))
	for index, event := range events {
		var payload attemptPayloadFactPayload
		bindingSequence := int64(index + 2)
		if event.StreamID != streamID || event.Seq != int64(index+1) ||
			event.Type != "AttemptPayloadCallIndexed" ||
			event.CorrelationID != authority.IncidentID ||
			event.ID != event.IdempotencyKey || event.CausationID == "" ||
			decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.ClaimID != authority.ClaimID ||
			payload.AgentInstanceID != authority.AgentInstanceID ||
			payload.binding().Scope != authority.Scope ||
			payload.Sequence != bindingSequence ||
			payload.Status != attemptpayload.FactAccepted || payload.Proof != "" ||
			!validAttemptPayloadBinding(payload.binding()) ||
			event.ID != attemptPayloadCallIndexEventID(payload) {
			return attemptPayloadCallIndexState{}, ErrAttemptPayloadFactConflict
		}
		if index > 0 && event.CausationID != state.lastID {
			return attemptPayloadCallIndexState{}, ErrAttemptPayloadFactConflict
		}
		binding := payload.binding()
		if _, duplicate := seenCalls[binding.CallID]; duplicate {
			return attemptPayloadCallIndexState{}, ErrAttemptPayloadFactConflict
		}
		seenCalls[binding.CallID] = struct{}{}
		state.bindings[binding.Sequence] = binding
		state.head = event.Seq
		state.lastID = event.ID
	}
	return state, nil
}

type attemptPayloadFactState struct {
	found               bool
	binding             attemptpayload.Binding
	status              attemptpayload.FactStatus
	proof               attemptpayload.DeliveryProof
	acceptedCausationID string
	lastEventID         string
}

func (state attemptPayloadFactState) public() attemptpayload.Fact {
	return attemptpayload.Fact{
		Binding: state.binding, Status: state.status, Proof: state.proof,
	}
}

func (authority *AttemptPayloadAuthority) readFacts(
	ctx context.Context,
	frozen attemptpayload.Authority,
	callID string,
	sequence int64,
) (attemptPayloadFactState, error) {
	events, err := authority.runs.store.ReadStream(
		ctx, attemptPayloadFactStreamFor(frozen, callID, sequence),
	)
	if err != nil {
		return attemptPayloadFactState{}, errors.Join(ErrAttemptPayloadFactAuthority, err)
	}
	return replayAttemptPayloadFacts(frozen, callID, sequence, events)
}

type attemptPayloadFactPayload struct {
	PayloadID              string                       `json:"payload_id"`
	ConversationID         string                       `json:"conversation_id"`
	WorkItemID             string                       `json:"work_item_id"`
	RunID                  string                       `json:"run_id"`
	ClaimID                string                       `json:"claim_id"`
	ClaimGeneration        int64                        `json:"claim_generation"`
	RuntimeInstanceID      string                       `json:"runtime_instance_id"`
	AgentInstanceID        string                       `json:"agent_instance_id"`
	ExecutionBindingDigest string                       `json:"execution_binding_digest"`
	CapsuleDigest          string                       `json:"capsule_digest"`
	CallID                 string                       `json:"call_id"`
	Sequence               int64                        `json:"sequence"`
	ContentType            string                       `json:"content_type"`
	ContentDigest          string                       `json:"content_digest"`
	Status                 attemptpayload.FactStatus    `json:"status"`
	Proof                  attemptpayload.DeliveryProof `json:"proof,omitempty"`
}

func attemptPayloadFactPayloadFrom(
	authority attemptpayload.Authority,
	binding attemptpayload.Binding,
	status attemptpayload.FactStatus,
	proof attemptpayload.DeliveryProof,
) attemptPayloadFactPayload {
	return attemptPayloadFactPayload{
		PayloadID: binding.PayloadID, ConversationID: binding.ConversationID,
		WorkItemID: binding.WorkItemID, RunID: binding.RunID,
		ClaimID: authority.ClaimID, ClaimGeneration: binding.ClaimGeneration,
		RuntimeInstanceID:      binding.RuntimeInstanceID,
		AgentInstanceID:        authority.AgentInstanceID,
		ExecutionBindingDigest: binding.ExecutionBindingDigest,
		CapsuleDigest:          binding.CapsuleDigest, CallID: binding.CallID,
		Sequence: binding.Sequence, ContentType: binding.ContentType,
		ContentDigest: binding.ContentDigest, Status: status, Proof: proof,
	}
}

func (payload attemptPayloadFactPayload) binding() attemptpayload.Binding {
	return attemptpayload.Binding{
		PayloadID: payload.PayloadID,
		Scope: attemptpayload.Scope{
			ConversationID: payload.ConversationID, WorkItemID: payload.WorkItemID,
			RunID: payload.RunID, ClaimGeneration: payload.ClaimGeneration,
			RuntimeInstanceID:      payload.RuntimeInstanceID,
			ExecutionBindingDigest: payload.ExecutionBindingDigest,
			CapsuleDigest:          payload.CapsuleDigest,
		},
		CallID: payload.CallID, Sequence: payload.Sequence,
		ContentType: payload.ContentType, ContentDigest: payload.ContentDigest,
	}
}

func replayAttemptPayloadFacts(
	authority attemptpayload.Authority,
	callID string,
	sequence int64,
	events []journal.Event,
) (attemptPayloadFactState, error) {
	state := attemptPayloadFactState{}
	streamID := attemptPayloadFactStreamFor(authority, callID, sequence)
	for index, event := range events {
		if event.StreamID != streamID || event.Seq != int64(index+1) ||
			event.CorrelationID != authority.IncidentID ||
			event.ID != event.IdempotencyKey {
			return attemptPayloadFactState{}, ErrAttemptPayloadFactConflict
		}
		var payload attemptPayloadFactPayload
		if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.ClaimID != authority.ClaimID ||
			payload.AgentInstanceID != authority.AgentInstanceID ||
			payload.binding().Scope != authority.Scope ||
			!validAttemptPayloadBinding(payload.binding()) ||
			event.ID != attemptPayloadFactEventID(event.Type, payload) {
			return attemptPayloadFactState{}, ErrAttemptPayloadFactConflict
		}
		switch event.Type {
		case "ToolResultAccepted":
			if index != 0 || payload.Status != attemptpayload.FactAccepted ||
				payload.Proof != "" || event.CausationID == "" {
				return attemptPayloadFactState{}, ErrAttemptPayloadFactConflict
			}
			state = attemptPayloadFactState{
				found: true, binding: payload.binding(), status: payload.Status,
				acceptedCausationID: event.CausationID, lastEventID: event.ID,
			}
		case "ToolResultDelivered":
			if index != 1 || !state.found || payload.binding() != state.binding ||
				payload.Status != attemptpayload.FactDelivered ||
				!validAttemptPayloadDeliveryProof(payload.Proof) ||
				event.CausationID != state.lastEventID {
				return attemptPayloadFactState{}, ErrAttemptPayloadFactConflict
			}
			state.status = payload.Status
			state.proof = payload.Proof
			state.lastEventID = event.ID
		default:
			return attemptPayloadFactState{}, ErrAttemptPayloadFactConflict
		}
	}
	return state, nil
}

func attemptPayloadFactStream(authority attemptpayload.Authority) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/attempt-payload-facts/v1", authority.ConversationID,
		authority.WorkItemID, authority.RunID,
		fmt.Sprint(authority.ClaimGeneration), authority.RuntimeInstanceID,
		authority.ExecutionBindingDigest, authority.CapsuleDigest,
	}, "\x00")))
	return "attempt-payload/" + hex.EncodeToString(digest[:16])
}

func attemptPayloadFactStreamFor(
	authority attemptpayload.Authority,
	callID string,
	sequence int64,
) string {
	if sequence == 1 {
		return attemptPayloadFactStream(authority)
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/attempt-payload-facts/v2", authority.ConversationID,
		authority.WorkItemID, authority.RunID,
		fmt.Sprint(authority.ClaimGeneration), authority.RuntimeInstanceID,
		authority.ExecutionBindingDigest, authority.CapsuleDigest,
		callID, fmt.Sprint(sequence),
	}, "\x00")))
	return "attempt-payload/" + hex.EncodeToString(digest[:16])
}

func attemptPayloadCallIndexStream(authority attemptpayload.Authority) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"loom/attempt-payload-call-index/v1", authority.ConversationID,
		authority.WorkItemID, authority.RunID,
		fmt.Sprint(authority.ClaimGeneration), authority.RuntimeInstanceID,
		authority.ExecutionBindingDigest, authority.CapsuleDigest,
	}, "\x00")))
	return "attempt-payload-index/" + hex.EncodeToString(digest[:16])
}

func attemptPayloadCallIndexEventID(payload attemptPayloadFactPayload) string {
	return deterministicEventID(
		"AttemptPayloadCallIndexed", payload.PayloadID, payload.RunID,
		fmt.Sprint(payload.ClaimGeneration), payload.CallID,
		fmt.Sprint(payload.Sequence), payload.ContentDigest,
	)
}

func attemptPayloadFactEventID(
	eventType string,
	payload attemptPayloadFactPayload,
) string {
	return deterministicEventID(
		eventType, payload.PayloadID, payload.RunID,
		fmt.Sprint(payload.ClaimGeneration), payload.CallID,
		fmt.Sprint(payload.Sequence), payload.ContentDigest,
		string(payload.Status), string(payload.Proof),
	)
}

func validAttemptPayloadAuthority(value attemptpayload.Authority) bool {
	return validOpaqueID(value.ConversationID) && validOpaqueID(value.WorkItemID) &&
		validOpaqueID(value.RunID) && validCanonicalUUID(value.ClaimID) &&
		value.ClaimGeneration > 0 && validOpaqueID(value.RuntimeInstanceID) &&
		validOpaqueID(value.AgentInstanceID) &&
		validSHA256Hex(value.ExecutionBindingDigest) &&
		validSHA256Hex(value.CapsuleDigest) && validCanonicalUUID(value.IncidentID)
}

func validAttemptPayloadBinding(value attemptpayload.Binding) bool {
	return validOpaqueID(value.PayloadID) && validOpaqueID(value.ConversationID) &&
		validOpaqueID(value.WorkItemID) && validOpaqueID(value.RunID) &&
		value.ClaimGeneration > 0 && validOpaqueID(value.RuntimeInstanceID) &&
		validSHA256Hex(value.ExecutionBindingDigest) &&
		validSHA256Hex(value.CapsuleDigest) && validOpaqueID(value.CallID) &&
		value.Sequence > 0 &&
		(value.ContentType == attemptpayload.ContentTypeJSON ||
			value.ContentType == attemptpayload.ContentTypeTextUTF8) &&
		validSHA256Hex(value.ContentDigest)
}

func validAttemptPayloadDeliveryProof(value attemptpayload.DeliveryProof) bool {
	return value == attemptpayload.ProofProviderContinuation ||
		value == attemptpayload.ProofHarnessFinalOutput ||
		value == attemptpayload.ProofHarnessToolResponse ||
		value == attemptpayload.ProofRunStreamToolResult
}
