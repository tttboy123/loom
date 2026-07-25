package projection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type runProjectionStatusReference struct {
	streamID string
	sequence int64
	eventID  string
}

type runProjectionStatusFact struct {
	runtimeID string
	status    string
	capacity  int
}

type runProjectionStatusReferenceFields struct {
	RuntimeStatusStreamID *string `json:"runtime_status_stream_id"`
	RuntimeStatusSequence *int64  `json:"runtime_status_sequence"`
	RuntimeStatusEventID  *string `json:"runtime_status_event_id"`
}

type runProjectionBinding struct {
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
}

type runProjectionClaim struct {
	binding         runProjectionBinding
	previousBinding runProjectionBinding
	statusReference runProjectionStatusReference
	needsOldRelease bool
	reserved        bool
	oldReleased     bool
}

type runProjectionTerminal struct {
	binding         runProjectionBinding
	statusReference runProjectionStatusReference
	status          string
	released        bool
	outcome         bool
}

type runProjectionWork struct {
	record      WorkItem
	lastEventID string
	sequence    int64
}

type runProjectionRun struct {
	record                Run
	lastEventID           string
	sequence              int64
	prepareLeaseExpiresAt time.Time
}

type runProjectionClaimedPayload struct {
	WorkItemID            *string `json:"work_item_id"`
	RunID                 *string `json:"run_id"`
	ClaimID               *string `json:"claim_id"`
	ClaimGeneration       *int64  `json:"claim_generation"`
	RuntimeInstanceID     *string `json:"runtime_instance_id"`
	AgentInstanceID       *string `json:"agent_instance_id"`
	PrepareLeaseExpiresAt *string `json:"prepare_lease_expires_at"`
	runProjectionStatusReferenceFields
}

type runProjectionGenerationPayload struct {
	WorkItemID        *string `json:"work_item_id"`
	RunID             *string `json:"run_id"`
	ClaimID           *string `json:"claim_id"`
	ClaimGeneration   *int64  `json:"claim_generation"`
	RuntimeInstanceID *string `json:"runtime_instance_id"`
	AgentInstanceID   *string `json:"agent_instance_id"`
	runProjectionStatusReferenceFields
}

type runProjectionLeasePayload struct {
	WorkItemID             *string `json:"work_item_id"`
	RunID                  *string `json:"run_id"`
	ClaimID                *string `json:"claim_id"`
	ClaimGeneration        *int64  `json:"claim_generation"`
	RuntimeInstanceID      *string `json:"runtime_instance_id"`
	AgentInstanceID        *string `json:"agent_instance_id"`
	PreviousLeaseExpiresAt *string `json:"previous_lease_expires_at"`
	PrepareLeaseExpiresAt  *string `json:"prepare_lease_expires_at"`
}

type runProjectionTerminalPayload struct {
	WorkItemID        *string `json:"work_item_id"`
	RunID             *string `json:"run_id"`
	ClaimID           *string `json:"claim_id"`
	ClaimGeneration   *int64  `json:"claim_generation"`
	RuntimeInstanceID *string `json:"runtime_instance_id"`
	AgentInstanceID   *string `json:"agent_instance_id"`
	Status            *string `json:"status"`
	Reason            *string `json:"reason"`
	runProjectionStatusReferenceFields
}

type runProjectionCapacityPayload struct {
	WorkItemID            *string `json:"work_item_id"`
	RunID                 *string `json:"run_id"`
	ClaimID               *string `json:"claim_id"`
	ClaimGeneration       *int64  `json:"claim_generation"`
	RuntimeInstanceID     *string `json:"runtime_instance_id"`
	AgentInstanceID       *string `json:"agent_instance_id"`
	RuntimeStatusStreamID *string `json:"runtime_status_stream_id"`
	RuntimeStatusSequence *int64  `json:"runtime_status_sequence"`
	RuntimeStatusEventID  *string `json:"runtime_status_event_id"`
}

func isRunAuthorityProjectionEvent(event journal.Event) bool {
	switch {
	case strings.HasPrefix(event.StreamID, "work-item/"):
		return true
	case strings.HasPrefix(event.StreamID, "run/"):
		return true
	case strings.HasPrefix(event.StreamID, "runtime_capacity:"):
		return true
	default:
		return false
	}
}

func isRunAuthorityRuntimeReferenceEvent(event journal.Event) bool {
	return strings.HasPrefix(event.StreamID, "runtime_instance:") &&
		(event.Type == "RuntimeInstanceDiscovered" ||
			event.Type == "RuntimeInstanceStatusChanged")
}

func applyRunAuthorityProjection(
	ctx context.Context,
	snapshot *Snapshot,
	events []journal.Event,
) error {
	if snapshot == nil {
		return ErrInvalidProjectionEvent
	}
	statusFacts, err := indexRunProjectionStatusFacts(ctx, events)
	if err != nil {
		return err
	}
	workItems, runsByAssignment, outcomes, err := indexRunProjectionWorkItems(ctx, events)
	if err != nil {
		return err
	}
	claims := make(map[string]*runProjectionClaim)
	terminals := make(map[string]*runProjectionTerminal)
	runs, err := replayRunProjectionRuns(
		ctx, events, runsByAssignment, workItems, statusFacts, claims, terminals,
	)
	if err != nil {
		return err
	}
	if err := replayRunProjectionCapacity(
		ctx, events, snapshot, statusFacts, claims, terminals,
	); err != nil {
		return err
	}
	for eventID, claim := range claims {
		if !claim.reserved || claim.needsOldRelease && !claim.oldReleased {
			return fmt.Errorf("%w: incomplete claim %s", ErrInvalidProjectionEvent, eventID)
		}
	}
	for eventID, terminal := range terminals {
		outcome, ok := outcomes[eventID]
		if !ok || !terminal.released {
			return fmt.Errorf("%w: incomplete terminal %s", ErrInvalidProjectionEvent, eventID)
		}
		workItem := workItems[terminal.binding.workItemID]
		if err := validateRunProjectionOutcome(outcome, terminal, &workItem); err != nil {
			return err
		}
		workItems[workItem.record.ID] = workItem
		terminal.outcome = true
	}
	for _, outcome := range outcomes {
		if _, ok := terminals[outcome.CausationID]; !ok {
			return fmt.Errorf("%w: orphan WorkItem outcome", ErrInvalidProjectionEvent)
		}
	}
	for runID, run := range runs {
		snapshot.Runs[runID] = run.record
	}
	for workItemID, workItem := range workItems {
		snapshot.WorkItems[workItemID] = workItem.record
	}
	return nil
}

func indexRunProjectionStatusFacts(
	ctx context.Context,
	events []journal.Event,
) (map[runProjectionStatusReference]runProjectionStatusFact, error) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if isRunAuthorityRuntimeReferenceEvent(event) {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	facts := make(map[runProjectionStatusReference]runProjectionStatusFact)
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		var current RuntimeInstance
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			switch event.Type {
			case "RuntimeInstanceDiscovered":
				next, err := projectRuntimeDiscoveryEvent(event)
				if err != nil {
					return nil, err
				}
				if current.ID != "" &&
					(current.ID != next.ID ||
						current.DeviceID != next.DeviceID ||
						current.AdapterType != next.AdapterType) {
					return nil, ErrInvalidProjectionEvent
				}
				current = next
			case "RuntimeInstanceStatusChanged":
				next, err := projectRuntimeStatusEvent(event, current)
				if err != nil {
					return nil, err
				}
				current = next
			default:
				return nil, ErrInvalidProjectionEvent
			}
			reference := runProjectionStatusReference{
				streamID: streamID,
				sequence: event.Seq,
				eventID:  event.ID,
			}
			if streamID != "runtime_instance:"+current.ID {
				return nil, ErrInvalidProjectionEvent
			}
			facts[reference] = runProjectionStatusFact{
				runtimeID: current.ID,
				status:    current.Status,
				capacity:  current.Capacity,
			}
		}
	}
	return facts, nil
}

func indexRunProjectionWorkItems(
	ctx context.Context,
	events []journal.Event,
) (
	map[string]runProjectionWork,
	map[string]runProjectionRun,
	map[string]journal.Event,
	error,
) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "work-item/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	workItems := make(map[string]runProjectionWork)
	runs := make(map[string]runProjectionRun)
	outcomes := make(map[string]journal.Event)
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		workItemID := strings.TrimPrefix(streamID, "work-item/")
		var workItem runProjectionWork
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, nil, nil, err
			}
			if !validRunProjectionEnvelope(event) {
				return nil, nil, nil, ErrInvalidProjectionEvent
			}
			switch event.Type {
			case "WorkItemCreated":
				var payload struct {
					WorkItemID *string `json:"work_item_id"`
					Title      *string `json:"title"`
					Status     *string `json:"status"`
				}
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					payload.WorkItemID == nil || payload.Title == nil ||
					payload.Status == nil || *payload.WorkItemID != workItemID ||
					*payload.Title == "" || *payload.Status != "ready" ||
					event.Seq != 1 || event.CausationID != "" ||
					workItem.record.ID != "" {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				workItem = runProjectionWork{
					record: WorkItem{
						ID: workItemID, Title: *payload.Title, Status: "ready",
					},
					lastEventID: event.ID,
					sequence:    event.Seq,
				}
			case "WorkItemAssigned":
				var payload struct {
					WorkItemID      *string `json:"work_item_id"`
					RunID           *string `json:"run_id"`
					AgentInstanceID *string `json:"agent_instance_id"`
					Status          *string `json:"status"`
				}
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					workItem.record.ID == "" || payload.WorkItemID == nil ||
					payload.RunID == nil || payload.AgentInstanceID == nil ||
					payload.Status == nil || *payload.WorkItemID != workItemID ||
					*payload.RunID == "" || *payload.AgentInstanceID == "" ||
					*payload.Status != "assigned" ||
					event.Seq != workItem.sequence+1 ||
					event.CausationID != workItem.lastEventID {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				if _, duplicate := runs[*payload.RunID]; duplicate {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				workItem.record.Status = "assigned"
				workItem.record.RunID = *payload.RunID
				workItem.record.AgentInstanceID = *payload.AgentInstanceID
				workItem.lastEventID = event.ID
				workItem.sequence = event.Seq
				runs[*payload.RunID] = runProjectionRun{
					record: Run{
						ID: *payload.RunID, WorkItemID: workItemID,
						Phase:           "unclaimed",
						AgentInstanceID: *payload.AgentInstanceID,
					},
					lastEventID: event.ID,
				}
			case "WorkItemReadyForReview", "WorkItemTerminal":
				if workItem.record.RunID == "" ||
					event.Seq != workItem.sequence+1 ||
					event.CausationID == "" {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				if _, duplicate := outcomes[event.CausationID]; duplicate {
					return nil, nil, nil, ErrInvalidProjectionEvent
				}
				outcomes[event.CausationID] = event
				workItem.sequence = event.Seq
				workItem.lastEventID = event.ID
			default:
				return nil, nil, nil, ErrInvalidProjectionEvent
			}
		}
		if workItem.record.RunID == "" {
			return nil, nil, nil, ErrInvalidProjectionEvent
		}
		workItems[workItemID] = workItem
	}
	return workItems, runs, outcomes, nil
}

func replayRunProjectionRuns(
	ctx context.Context,
	events []journal.Event,
	assigned map[string]runProjectionRun,
	workItems map[string]runProjectionWork,
	statusFacts map[runProjectionStatusReference]runProjectionStatusFact,
	claims map[string]*runProjectionClaim,
	terminals map[string]*runProjectionTerminal,
) (map[string]runProjectionRun, error) {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "run/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		runID := strings.TrimPrefix(streamID, "run/")
		run, ok := assigned[runID]
		if !ok {
			return nil, ErrInvalidProjectionEvent
		}
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !validRunProjectionEnvelope(event) ||
				event.CausationID != run.lastEventID ||
				event.Seq != run.sequence+1 {
				return nil, ErrInvalidProjectionEvent
			}
			switch event.Type {
			case "RunClaimed":
				var payload runProjectionClaimedPayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionClaimPayload(payload, run.record) {
					return nil, ErrInvalidProjectionEvent
				}
				expiresAt, err := parseRunProjectionTime(*payload.PrepareLeaseExpiresAt)
				if err != nil || run.record.Phase == "running" ||
					run.record.Phase == "terminal" ||
					run.record.Phase == "claimed" &&
						event.EmittedAt.Before(run.prepareLeaseExpiresAt) {
					return nil, ErrInvalidProjectionEvent
				}
				reference := payload.runProjectionStatusReferenceFields.reference()
				if !validRunProjectionStatusReference(
					statusFacts, reference, *payload.RuntimeInstanceID, true,
				) {
					return nil, ErrInvalidProjectionEvent
				}
				previousBinding := bindingFromRun(run.record)
				previousGeneration := run.record.ClaimGeneration
				run.record.Phase = "claimed"
				run.record.ClaimID = *payload.ClaimID
				run.record.ClaimGeneration = *payload.ClaimGeneration
				run.record.RuntimeInstanceID = *payload.RuntimeInstanceID
				run.record.AgentInstanceID = *payload.AgentInstanceID
				run.record.PrepareLeaseExpiresAt = expiresAt
				run.prepareLeaseExpiresAt = expiresAt
				run.lastEventID = event.ID
				run.sequence = event.Seq
				claims[event.ID] = &runProjectionClaim{
					binding:         bindingFromRun(run.record),
					previousBinding: previousBinding,
					statusReference: reference,
					needsOldRelease: previousGeneration > 0,
				}
			case "RunPrepareLeaseExtended":
				var payload runProjectionLeasePayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionGeneration(
						payload.WorkItemID, payload.RunID, payload.ClaimID,
						payload.ClaimGeneration, payload.RuntimeInstanceID,
						payload.AgentInstanceID, run.record,
					) ||
					payload.PreviousLeaseExpiresAt == nil ||
					payload.PrepareLeaseExpiresAt == nil ||
					run.record.Phase != "claimed" {
					return nil, ErrInvalidProjectionEvent
				}
				previous, err := parseRunProjectionTime(*payload.PreviousLeaseExpiresAt)
				if err != nil || !previous.Equal(run.prepareLeaseExpiresAt) {
					return nil, ErrInvalidProjectionEvent
				}
				next, err := parseRunProjectionTime(*payload.PrepareLeaseExpiresAt)
				if err != nil || !next.After(previous) ||
					!event.EmittedAt.Before(previous) {
					return nil, ErrInvalidProjectionEvent
				}
				run.record.PrepareLeaseExpiresAt = next
				run.prepareLeaseExpiresAt = next
				run.lastEventID = event.ID
				run.sequence = event.Seq
			case "RunStarted":
				var payload runProjectionGenerationPayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionGenerationPayload(payload, run.record) ||
					run.record.Phase != "claimed" ||
					!event.EmittedAt.Before(run.prepareLeaseExpiresAt) {
					return nil, ErrInvalidProjectionEvent
				}
				reference := payload.runProjectionStatusReferenceFields.reference()
				if !validRunProjectionStatusReference(
					statusFacts, reference, run.record.RuntimeInstanceID, true,
				) {
					return nil, ErrInvalidProjectionEvent
				}
				run.record.Phase = "running"
				run.lastEventID = event.ID
				run.sequence = event.Seq
				workItem := workItems[run.record.WorkItemID]
				workItem.record.Status = "running"
				workItems[workItem.record.ID] = workItem
			case "RunTerminalCommitted":
				var payload runProjectionTerminalPayload
				if err := decodeRunProjectionPayload(event, &payload); err != nil ||
					!validRunProjectionTerminalPayload(payload, run.record) ||
					(run.record.Phase != "claimed" &&
						run.record.Phase != "running") {
					return nil, ErrInvalidProjectionEvent
				}
				reference := payload.runProjectionStatusReferenceFields.reference()
				if !validRunProjectionStatusReference(
					statusFacts, reference, run.record.RuntimeInstanceID, false,
				) {
					return nil, ErrInvalidProjectionEvent
				}
				run.record.Phase = "terminal"
				run.record.TerminalStatus = *payload.Status
				run.record.TerminalReason = *payload.Reason
				run.lastEventID = event.ID
				run.sequence = event.Seq
				terminals[event.ID] = &runProjectionTerminal{
					binding:         bindingFromRun(run.record),
					statusReference: reference,
					status:          run.record.TerminalStatus,
				}
			default:
				return nil, ErrInvalidProjectionEvent
			}
		}
		assigned[runID] = run
	}
	return assigned, nil
}

func replayRunProjectionCapacity(
	ctx context.Context,
	events []journal.Event,
	snapshot *Snapshot,
	statusFacts map[runProjectionStatusReference]runProjectionStatusFact,
	claims map[string]*runProjectionClaim,
	terminals map[string]*runProjectionTerminal,
) error {
	byStream := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "runtime_capacity:") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
	}
	for streamID, streamEvents := range byStream {
		sort.SliceStable(streamEvents, func(i, j int) bool {
			return streamEvents[i].Seq < streamEvents[j].Seq
		})
		runtimeID := strings.TrimPrefix(streamID, "runtime_capacity:")
		runtime, ok := snapshot.RuntimeInstances[runtimeID]
		if !ok || runtime.Capacity <= 0 {
			return ErrInvalidProjectionEvent
		}
		active := make(map[string]runProjectionBinding)
		for _, event := range streamEvents {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !validRunProjectionEnvelope(event) {
				return ErrInvalidProjectionEvent
			}
			var payload runProjectionCapacityPayload
			if err := decodeRunProjectionPayload(event, &payload); err != nil ||
				!validRunProjectionCapacityPayload(payload, runtimeID) {
				return ErrInvalidProjectionEvent
			}
			binding := payload.binding()
			reference := payload.statusReference()
			key := runProjectionCapacityKey(binding)
			switch event.Type {
			case "RuntimeCapacityReserved":
				claim, ok := claims[event.CausationID]
				if !ok || claim.binding != binding ||
					claim.statusReference != reference ||
					!validRunProjectionStatusReference(
						statusFacts, reference, runtimeID, true,
					) {
					return ErrInvalidProjectionEvent
				}
				if _, exists := active[key]; exists {
					return ErrInvalidProjectionEvent
				}
				active[key] = binding
				claim.reserved = true
				statusFact := statusFacts[reference]
				if statusFact.capacity <= 0 ||
					len(active) > statusFact.capacity {
					return ErrInvalidProjectionEvent
				}
			case "RuntimeCapacityReleased":
				current, exists := active[key]
				if !exists || current != binding {
					return ErrInvalidProjectionEvent
				}
				delete(active, key)
				if terminal, ok := terminals[event.CausationID]; ok {
					if terminal.binding != binding ||
						terminal.statusReference != reference ||
						!validRunProjectionStatusReference(
							statusFacts, reference, runtimeID, false,
						) {
						return ErrInvalidProjectionEvent
					}
					terminal.released = true
				} else if claim, ok := claims[event.CausationID]; ok {
					if !claim.needsOldRelease ||
						claim.previousBinding != binding ||
						claim.statusReference != reference ||
						!validRunProjectionStatusReference(
							statusFacts, reference, runtimeID, true,
						) {
						return ErrInvalidProjectionEvent
					}
					claim.oldReleased = true
				} else {
					return ErrInvalidProjectionEvent
				}
			default:
				return ErrInvalidProjectionEvent
			}
		}
	}
	return nil
}

func validateRunProjectionOutcome(
	event journal.Event,
	terminal *runProjectionTerminal,
	workItem *runProjectionWork,
) error {
	var payload struct {
		WorkItemID      *string `json:"work_item_id"`
		RunID           *string `json:"run_id"`
		ClaimGeneration *int64  `json:"claim_generation"`
		Status          *string `json:"status"`
	}
	if err := decodeRunProjectionPayload(event, &payload); err != nil ||
		payload.WorkItemID == nil || payload.RunID == nil ||
		payload.ClaimGeneration == nil || payload.Status == nil ||
		*payload.WorkItemID != terminal.binding.workItemID ||
		*payload.RunID != terminal.binding.runID ||
		*payload.ClaimGeneration != terminal.binding.claimGeneration ||
		workItem.record.ID != terminal.binding.workItemID {
		return ErrInvalidProjectionEvent
	}
	if terminal.status == "succeeded" {
		if event.Type != "WorkItemReadyForReview" ||
			*payload.Status != "ready_for_review" {
			return ErrInvalidProjectionEvent
		}
		workItem.record.Status = "ready_for_review"
	} else {
		if event.Type != "WorkItemTerminal" || *payload.Status != terminal.status {
			return ErrInvalidProjectionEvent
		}
		workItem.record.Status = terminal.status
	}
	return nil
}

func decodeRunProjectionPayload(event journal.Event, target any) error {
	if err := rejectDuplicateRuntimeStatusPayloadFields(event.PayloadJSON); err != nil {
		return err
	}
	return decodeExactProjectionPayload(event, target)
}

func validRunProjectionEnvelope(event journal.Event) bool {
	return event.ID != "" &&
		event.StreamID != "" &&
		event.Seq > 0 &&
		event.IdempotencyKey != "" &&
		event.CorrelationID != "" &&
		!event.EmittedAt.IsZero() &&
		event.EmittedAt.Location() == time.UTC
}

func (fields runProjectionStatusReferenceFields) reference() runProjectionStatusReference {
	if fields.RuntimeStatusStreamID == nil ||
		fields.RuntimeStatusSequence == nil ||
		fields.RuntimeStatusEventID == nil {
		return runProjectionStatusReference{}
	}
	return runProjectionStatusReference{
		streamID: *fields.RuntimeStatusStreamID,
		sequence: *fields.RuntimeStatusSequence,
		eventID:  *fields.RuntimeStatusEventID,
	}
}

func validRunProjectionStatusReference(
	facts map[runProjectionStatusReference]runProjectionStatusFact,
	reference runProjectionStatusReference,
	runtimeID string,
	requireOnline bool,
) bool {
	if reference.streamID != "runtime_instance:"+runtimeID ||
		reference.sequence <= 0 ||
		reference.eventID == "" {
		return false
	}
	fact, ok := facts[reference]
	if !ok || fact.runtimeID != runtimeID {
		return false
	}
	return !requireOnline || fact.status == "online"
}

func validRunProjectionClaimPayload(
	payload runProjectionClaimedPayload,
	run Run,
) bool {
	return payload.WorkItemID != nil && payload.RunID != nil &&
		payload.ClaimID != nil && payload.ClaimGeneration != nil &&
		payload.RuntimeInstanceID != nil && payload.AgentInstanceID != nil &&
		payload.PrepareLeaseExpiresAt != nil &&
		*payload.WorkItemID == run.WorkItemID &&
		*payload.RunID == run.ID &&
		validRunProjectionCanonicalUUID(*payload.ClaimID) &&
		*payload.RuntimeInstanceID != "" &&
		*payload.AgentInstanceID == run.AgentInstanceID &&
		*payload.ClaimGeneration == run.ClaimGeneration+1 &&
		payload.reference() != (runProjectionStatusReference{})
}

func validRunProjectionGenerationPayload(
	payload runProjectionGenerationPayload,
	run Run,
) bool {
	return validRunProjectionGeneration(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) && payload.reference() != (runProjectionStatusReference{})
}

func validRunProjectionTerminalPayload(
	payload runProjectionTerminalPayload,
	run Run,
) bool {
	if !validRunProjectionGeneration(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) || payload.Status == nil || payload.Reason == nil ||
		payload.reference() == (runProjectionStatusReference{}) {
		return false
	}
	switch *payload.Status {
	case "succeeded":
		return *payload.Reason == ""
	case "failed", "cancelled":
		return *payload.Reason != ""
	default:
		return false
	}
}

func validRunProjectionGeneration(
	workItemID *string,
	runID *string,
	claimID *string,
	claimGeneration *int64,
	runtimeInstanceID *string,
	agentInstanceID *string,
	run Run,
) bool {
	return workItemID != nil && runID != nil && claimID != nil &&
		claimGeneration != nil && runtimeInstanceID != nil &&
		agentInstanceID != nil &&
		*workItemID == run.WorkItemID &&
		*runID == run.ID &&
		*claimID == run.ClaimID &&
		*claimGeneration == run.ClaimGeneration &&
		*runtimeInstanceID == run.RuntimeInstanceID &&
		*agentInstanceID == run.AgentInstanceID
}

func validRunProjectionCapacityPayload(
	payload runProjectionCapacityPayload,
	runtimeID string,
) bool {
	return payload.WorkItemID != nil && payload.RunID != nil &&
		payload.ClaimID != nil && payload.ClaimGeneration != nil &&
		payload.RuntimeInstanceID != nil && payload.AgentInstanceID != nil &&
		payload.RuntimeStatusStreamID != nil &&
		payload.RuntimeStatusSequence != nil &&
		payload.RuntimeStatusEventID != nil &&
		*payload.WorkItemID != "" && *payload.RunID != "" &&
		*payload.ClaimID != "" && *payload.ClaimGeneration > 0 &&
		*payload.RuntimeInstanceID == runtimeID &&
		*payload.AgentInstanceID != "" &&
		payload.statusReference() != (runProjectionStatusReference{})
}

func (payload runProjectionCapacityPayload) binding() runProjectionBinding {
	return runProjectionBinding{
		workItemID:        *payload.WorkItemID,
		runID:             *payload.RunID,
		claimID:           *payload.ClaimID,
		claimGeneration:   *payload.ClaimGeneration,
		runtimeInstanceID: *payload.RuntimeInstanceID,
		agentInstanceID:   *payload.AgentInstanceID,
	}
}

func (payload runProjectionCapacityPayload) statusReference() runProjectionStatusReference {
	return runProjectionStatusReference{
		streamID: *payload.RuntimeStatusStreamID,
		sequence: *payload.RuntimeStatusSequence,
		eventID:  *payload.RuntimeStatusEventID,
	}
}

func bindingFromRun(run Run) runProjectionBinding {
	return runProjectionBinding{
		workItemID:        run.WorkItemID,
		runID:             run.ID,
		claimID:           run.ClaimID,
		claimGeneration:   run.ClaimGeneration,
		runtimeInstanceID: run.RuntimeInstanceID,
		agentInstanceID:   run.AgentInstanceID,
	}
}

func runProjectionCapacityKey(binding runProjectionBinding) string {
	return fmt.Sprintf("%s/%d", binding.runID, binding.claimGeneration)
}

func parseRunProjectionTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidProjectionEvent
	}
	return parsed, nil
}

func validRunProjectionCanonicalUUID(value string) bool {
	if len(value) != 36 ||
		value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' ||
		value[14] != '4' ||
		(value[19] != '8' && value[19] != '9' &&
			value[19] != 'a' && value[19] != 'b') {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			continue
		}
		if !(character >= '0' && character <= '9') &&
			!(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}
