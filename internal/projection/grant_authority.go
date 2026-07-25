package projection

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

const grantProjectionStreamPrefix = "agent-grant/"

type grantProjectionRunState struct {
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
	phase             string
	leaseExpiresAt    time.Time
	terminal          bool
}

type grantProjectionRunReference struct {
	streamID  string
	sequence  int64
	eventID   string
	emittedAt time.Time
	state     grantProjectionRunState
}

type grantProjectionRecord struct {
	record      AgentGrant
	tokenHash   string
	lastEventID string
	streamSeq   int64
	requests    map[string]grantProjectionAuthorization
}

type grantProjectionAuthorization struct {
	grantID           string
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
	operation         string
}

type grantProjectionIssuePayload struct {
	GrantID           *string   `json:"grant_id"`
	WorkItemID        *string   `json:"work_item_id"`
	RunID             *string   `json:"run_id"`
	ClaimID           *string   `json:"claim_id"`
	ClaimGeneration   *int64    `json:"claim_generation"`
	RuntimeInstanceID *string   `json:"runtime_instance_id"`
	AgentInstanceID   *string   `json:"agent_instance_id"`
	AllowedOperations *[]string `json:"allowed_operations"`
	TokenHash         *string   `json:"token_hash"`
	IssuedAt          *string   `json:"issued_at"`
	ExpiresAt         *string   `json:"expires_at"`
	RunStream         *string   `json:"run_stream"`
	RunSequence       *int64    `json:"run_sequence"`
	RunEventID        *string   `json:"run_event_id"`
}

type grantProjectionAuthorizePayload struct {
	GrantID           *string `json:"grant_id"`
	WorkItemID        *string `json:"work_item_id"`
	RunID             *string `json:"run_id"`
	ClaimID           *string `json:"claim_id"`
	ClaimGeneration   *int64  `json:"claim_generation"`
	RuntimeInstanceID *string `json:"runtime_instance_id"`
	AgentInstanceID   *string `json:"agent_instance_id"`
	Operation         *string `json:"operation"`
	RequestID         *string `json:"request_id"`
	AuthorizedAt      *string `json:"authorized_at"`
	RunStream         *string `json:"run_stream"`
	RunSequence       *int64  `json:"run_sequence"`
	RunEventID        *string `json:"run_event_id"`
}

type grantProjectionRevokePayload struct {
	GrantID     *string `json:"grant_id"`
	Reason      *string `json:"reason"`
	RevokedAt   *string `json:"revoked_at"`
	RunStream   *string `json:"run_stream"`
	RunSequence *int64  `json:"run_sequence"`
	RunEventID  *string `json:"run_event_id"`
}

func isGrantAuthorityProjectionEvent(event journal.Event) bool {
	if !strings.HasPrefix(event.StreamID, grantProjectionStreamPrefix) {
		return false
	}
	switch event.Type {
	case "AgentGrantIssued", "AgentGrantAuthorized", "AgentGrantRevoked":
		return true
	default:
		return false
	}
}

func applyGrantAuthorityProjection(
	ctx context.Context,
	snapshot *Snapshot,
	runEvents []journal.Event,
	grantEvents []journal.Event,
) error {
	if snapshot == nil {
		return ErrInvalidProjectionEvent
	}
	runReferences, err := indexGrantProjectionRunReferences(ctx, runEvents)
	if err != nil {
		return err
	}
	records := make(map[string]*grantProjectionRecord)
	hashes := make(map[string]string)
	active := make(map[string]string)
	requests := make(map[string]map[string]grantProjectionAuthorization)
	lastByStream := make(map[string]string)
	sequenceByStream := make(map[string]int64)

	for _, event := range grantEvents {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !validGrantProjectionEnvelope(event) ||
			event.Seq != sequenceByStream[event.StreamID]+1 {
			return fmt.Errorf("%w: invalid AgentGrant envelope", ErrInvalidProjectionEvent)
		}
		runID := strings.TrimPrefix(event.StreamID, grantProjectionStreamPrefix)
		switch event.Type {
		case "AgentGrantIssued":
			var payload grantProjectionIssuePayload
			if err := decodeExactProjectionPayload(event, &payload); err != nil {
				return err
			}
			reference, issuedAt, expiresAt, operations, err :=
				validateGrantProjectionIssue(payload, event, runID, runReferences)
			if err != nil {
				return err
			}
			if _, exists := records[*payload.GrantID]; exists {
				return fmt.Errorf("%w: duplicate Grant ID", ErrInvalidProjectionEvent)
			}
			if _, exists := hashes[*payload.TokenHash]; exists {
				return fmt.Errorf("%w: duplicate Grant hash", ErrInvalidProjectionEvent)
			}
			if _, exists := active[runID]; exists {
				return fmt.Errorf("%w: multiple active Grants", ErrInvalidProjectionEvent)
			}
			if previous := lastByStream[event.StreamID]; previous != "" {
				if event.CausationID != previous {
					return fmt.Errorf("%w: Grant issue causation", ErrInvalidProjectionEvent)
				}
			} else if event.CausationID != reference.eventID {
				return fmt.Errorf("%w: initial Grant causation", ErrInvalidProjectionEvent)
			}
			record := &grantProjectionRecord{
				record: AgentGrant{
					ID:                *payload.GrantID,
					WorkItemID:        *payload.WorkItemID,
					RunID:             *payload.RunID,
					ClaimID:           *payload.ClaimID,
					ClaimGeneration:   *payload.ClaimGeneration,
					RuntimeInstanceID: *payload.RuntimeInstanceID,
					AgentInstanceID:   *payload.AgentInstanceID,
					AllowedOperations: operations,
					IssuedAt:          issuedAt,
					ExpiresAt:         expiresAt,
				},
				tokenHash:   *payload.TokenHash,
				lastEventID: event.ID,
				streamSeq:   event.Seq,
				requests:    make(map[string]grantProjectionAuthorization),
			}
			records[record.record.ID] = record
			hashes[record.tokenHash] = record.record.ID
			active[runID] = record.record.ID
		case "AgentGrantAuthorized":
			var payload grantProjectionAuthorizePayload
			if err := decodeExactProjectionPayload(event, &payload); err != nil {
				return err
			}
			record, authorization, err := validateGrantProjectionAuthorization(
				payload,
				event,
				runID,
				records,
				runReferences,
			)
			if err != nil {
				return err
			}
			if event.CausationID != record.lastEventID {
				return fmt.Errorf("%w: Grant authorization causation", ErrInvalidProjectionEvent)
			}
			if requests[runID] == nil {
				requests[runID] = make(map[string]grantProjectionAuthorization)
			}
			if _, exists := requests[runID][*payload.RequestID]; exists {
				return fmt.Errorf("%w: duplicate Grant RequestID", ErrInvalidProjectionEvent)
			}
			requests[runID][*payload.RequestID] = authorization
			record.requests[*payload.RequestID] = authorization
			record.lastEventID = event.ID
			record.streamSeq = event.Seq
		case "AgentGrantRevoked":
			var payload grantProjectionRevokePayload
			if err := decodeExactProjectionPayload(event, &payload); err != nil {
				return err
			}
			record, revokedAt, err := validateGrantProjectionRevocation(
				payload,
				event,
				runID,
				records,
				runReferences,
			)
			if err != nil {
				return err
			}
			if event.CausationID != record.lastEventID {
				return fmt.Errorf("%w: Grant revocation causation", ErrInvalidProjectionEvent)
			}
			record.record.RevokedAt = revokedAt
			record.record.RevocationReason = *payload.Reason
			record.lastEventID = event.ID
			record.streamSeq = event.Seq
			delete(active, runID)
		default:
			return fmt.Errorf("%w: unknown AgentGrant Event", ErrInvalidProjectionEvent)
		}
		lastByStream[event.StreamID] = event.ID
		sequenceByStream[event.StreamID] = event.Seq
	}

	for id, record := range records {
		projected := record.record
		projected.AllowedOperations = append(
			[]string(nil),
			record.record.AllowedOperations...,
		)
		snapshot.AgentGrants[id] = projected
	}
	return nil
}

func indexGrantProjectionRunReferences(
	ctx context.Context,
	events []journal.Event,
) (map[string]grantProjectionRunReference, error) {
	references := make(map[string]grantProjectionRunReference)
	states := make(map[string]grantProjectionRunState)
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(event.StreamID, "run/") {
			continue
		}
		runID := strings.TrimPrefix(event.StreamID, "run/")
		state := states[runID]
		switch event.Type {
		case "RunClaimed":
			var payload runProjectionClaimedPayload
			if err := decodeRunProjectionPayload(event, &payload); err != nil ||
				payload.WorkItemID == nil ||
				payload.RunID == nil ||
				payload.ClaimID == nil ||
				payload.ClaimGeneration == nil ||
				payload.RuntimeInstanceID == nil ||
				payload.AgentInstanceID == nil ||
				payload.PrepareLeaseExpiresAt == nil ||
				*payload.RunID != runID {
				return nil, fmt.Errorf("%w: invalid referenced Run claim", ErrInvalidProjectionEvent)
			}
			lease, err := time.Parse(time.RFC3339Nano, *payload.PrepareLeaseExpiresAt)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid referenced Run lease", ErrInvalidProjectionEvent)
			}
			state = grantProjectionRunState{
				workItemID:        *payload.WorkItemID,
				runID:             *payload.RunID,
				claimID:           *payload.ClaimID,
				claimGeneration:   *payload.ClaimGeneration,
				runtimeInstanceID: *payload.RuntimeInstanceID,
				agentInstanceID:   *payload.AgentInstanceID,
				phase:             "claimed",
				leaseExpiresAt:    lease,
			}
		case "RunPrepareLeaseExtended":
			var payload runProjectionLeasePayload
			if err := decodeRunProjectionPayload(event, &payload); err != nil ||
				!grantProjectionLeaseMatches(payload, state) {
				return nil, fmt.Errorf("%w: invalid referenced Run lease", ErrInvalidProjectionEvent)
			}
			lease, err := time.Parse(time.RFC3339Nano, *payload.PrepareLeaseExpiresAt)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid referenced Run lease time", ErrInvalidProjectionEvent)
			}
			state.leaseExpiresAt = lease
		case "RunStarted":
			var payload runProjectionGenerationPayload
			if err := decodeRunProjectionPayload(event, &payload); err != nil ||
				!grantProjectionGenerationMatches(payload, state) {
				return nil, fmt.Errorf("%w: invalid referenced Run start", ErrInvalidProjectionEvent)
			}
			state.phase = "running"
		case "RunTerminalCommitted":
			var payload runProjectionTerminalPayload
			if err := decodeRunProjectionPayload(event, &payload); err != nil ||
				!grantProjectionTerminalMatches(payload, state) {
				return nil, fmt.Errorf("%w: invalid referenced Run terminal", ErrInvalidProjectionEvent)
			}
			state.phase = "terminal"
			state.terminal = true
		default:
			return nil, fmt.Errorf("%w: unknown referenced Run Event", ErrInvalidProjectionEvent)
		}
		states[runID] = state
		references[event.ID] = grantProjectionRunReference{
			streamID:  event.StreamID,
			sequence:  event.Seq,
			eventID:   event.ID,
			emittedAt: event.EmittedAt,
			state:     state,
		}
	}
	return references, nil
}

func validateGrantProjectionIssue(
	payload grantProjectionIssuePayload,
	event journal.Event,
	runID string,
	references map[string]grantProjectionRunReference,
) (grantProjectionRunReference, time.Time, time.Time, []string, error) {
	if payload.GrantID == nil ||
		payload.WorkItemID == nil ||
		payload.RunID == nil ||
		payload.ClaimID == nil ||
		payload.ClaimGeneration == nil ||
		payload.RuntimeInstanceID == nil ||
		payload.AgentInstanceID == nil ||
		payload.AllowedOperations == nil ||
		payload.TokenHash == nil ||
		payload.IssuedAt == nil ||
		payload.ExpiresAt == nil ||
		payload.RunStream == nil ||
		payload.RunSequence == nil ||
		payload.RunEventID == nil ||
		*payload.RunID != runID ||
		!validRunProjectionCanonicalUUID(*payload.GrantID) ||
		!validRunProjectionCanonicalUUID(*payload.ClaimID) ||
		!validGrantProjectionHash(*payload.TokenHash) {
		return grantProjectionRunReference{}, time.Time{}, time.Time{}, nil,
			fmt.Errorf("%w: invalid Grant issue", ErrInvalidProjectionEvent)
	}
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, *payload.IssuedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, *payload.ExpiresAt)
	operations := append([]string(nil), (*payload.AllowedOperations)...)
	reference, ok := references[*payload.RunEventID]
	if issuedErr != nil || expiresErr != nil ||
		event.EmittedAt != issuedAt ||
		!expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > time.Hour ||
		!validGrantProjectionOperations(operations) ||
		!ok ||
		reference.streamID != *payload.RunStream ||
		reference.sequence != *payload.RunSequence ||
		reference.streamID != "run/"+runID ||
		issuedAt.Before(reference.emittedAt) ||
		reference.state.phase != "claimed" ||
		!issuedAt.Before(reference.state.leaseExpiresAt) ||
		!grantProjectionBindingMatchesIssue(payload, reference.state) {
		return grantProjectionRunReference{}, time.Time{}, time.Time{}, nil,
			fmt.Errorf("%w: invalid Grant Run reference", ErrInvalidProjectionEvent)
	}
	return reference, issuedAt, expiresAt, operations, nil
}

func validateGrantProjectionAuthorization(
	payload grantProjectionAuthorizePayload,
	event journal.Event,
	runID string,
	records map[string]*grantProjectionRecord,
	references map[string]grantProjectionRunReference,
) (*grantProjectionRecord, grantProjectionAuthorization, error) {
	if payload.GrantID == nil ||
		payload.WorkItemID == nil ||
		payload.RunID == nil ||
		payload.ClaimID == nil ||
		payload.ClaimGeneration == nil ||
		payload.RuntimeInstanceID == nil ||
		payload.AgentInstanceID == nil ||
		payload.Operation == nil ||
		payload.RequestID == nil ||
		payload.AuthorizedAt == nil ||
		payload.RunStream == nil ||
		payload.RunSequence == nil ||
		payload.RunEventID == nil ||
		*payload.RunID != runID ||
		!validRunProjectionCanonicalUUID(*payload.RequestID) {
		return nil, grantProjectionAuthorization{},
			fmt.Errorf("%w: invalid Grant authorization", ErrInvalidProjectionEvent)
	}
	record := records[*payload.GrantID]
	reference, ok := references[*payload.RunEventID]
	authorizedAt, timeErr := time.Parse(time.RFC3339Nano, *payload.AuthorizedAt)
	if record == nil ||
		record.record.RevocationReason != "" ||
		timeErr != nil ||
		event.EmittedAt != authorizedAt ||
		!authorizedAt.Before(record.record.ExpiresAt) ||
		!grantProjectionHasOperation(record.record.AllowedOperations, *payload.Operation) ||
		!ok ||
		reference.streamID != *payload.RunStream ||
		reference.sequence != *payload.RunSequence ||
		reference.streamID != "run/"+runID ||
		authorizedAt.Before(reference.emittedAt) ||
		!grantProjectionStateAuthorizable(reference.state, authorizedAt) ||
		!grantProjectionBindingMatchesAuthorization(payload, record.record, reference.state) {
		return nil, grantProjectionAuthorization{},
			fmt.Errorf("%w: invalid Grant authorization state", ErrInvalidProjectionEvent)
	}
	authorization := grantProjectionAuthorization{
		grantID:           record.record.ID,
		workItemID:        record.record.WorkItemID,
		runID:             record.record.RunID,
		claimID:           record.record.ClaimID,
		claimGeneration:   record.record.ClaimGeneration,
		runtimeInstanceID: record.record.RuntimeInstanceID,
		agentInstanceID:   record.record.AgentInstanceID,
		operation:         *payload.Operation,
	}
	return record, authorization, nil
}

func validateGrantProjectionRevocation(
	payload grantProjectionRevokePayload,
	event journal.Event,
	runID string,
	records map[string]*grantProjectionRecord,
	references map[string]grantProjectionRunReference,
) (*grantProjectionRecord, time.Time, error) {
	if payload.GrantID == nil ||
		payload.Reason == nil ||
		payload.RevokedAt == nil ||
		payload.RunStream == nil ||
		payload.RunSequence == nil ||
		payload.RunEventID == nil ||
		!validGrantProjectionRevocation(*payload.Reason) {
		return nil, time.Time{},
			fmt.Errorf("%w: invalid Grant revocation", ErrInvalidProjectionEvent)
	}
	record := records[*payload.GrantID]
	reference, ok := references[*payload.RunEventID]
	revokedAt, timeErr := time.Parse(time.RFC3339Nano, *payload.RevokedAt)
	if record == nil ||
		record.record.RunID != runID ||
		record.record.RevocationReason != "" ||
		timeErr != nil ||
		event.EmittedAt != revokedAt ||
		revokedAt.Before(record.record.IssuedAt) ||
		!ok ||
		reference.streamID != *payload.RunStream ||
		reference.sequence != *payload.RunSequence ||
		reference.streamID != "run/"+runID ||
		revokedAt.Before(reference.emittedAt) ||
		reference.state.runID != record.record.RunID ||
		reference.state.workItemID != record.record.WorkItemID ||
		reference.state.claimGeneration < record.record.ClaimGeneration {
		return nil, time.Time{},
			fmt.Errorf("%w: invalid Grant revocation state", ErrInvalidProjectionEvent)
	}
	return record, revokedAt, nil
}

func grantProjectionLeaseMatches(
	payload runProjectionLeasePayload,
	state grantProjectionRunState,
) bool {
	if payload.WorkItemID == nil ||
		payload.RunID == nil ||
		payload.ClaimID == nil ||
		payload.ClaimGeneration == nil ||
		payload.RuntimeInstanceID == nil ||
		payload.AgentInstanceID == nil ||
		payload.PreviousLeaseExpiresAt == nil ||
		payload.PrepareLeaseExpiresAt == nil {
		return false
	}
	previous, previousErr := time.Parse(time.RFC3339Nano, *payload.PreviousLeaseExpiresAt)
	next, nextErr := time.Parse(time.RFC3339Nano, *payload.PrepareLeaseExpiresAt)
	return state.phase == "claimed" &&
		*payload.WorkItemID == state.workItemID &&
		*payload.RunID == state.runID &&
		*payload.ClaimID == state.claimID &&
		*payload.ClaimGeneration == state.claimGeneration &&
		*payload.RuntimeInstanceID == state.runtimeInstanceID &&
		*payload.AgentInstanceID == state.agentInstanceID &&
		previousErr == nil && nextErr == nil &&
		previous.Equal(state.leaseExpiresAt) &&
		next.After(previous)
}

func grantProjectionGenerationMatches(
	payload runProjectionGenerationPayload,
	state grantProjectionRunState,
) bool {
	return payload.WorkItemID != nil &&
		payload.RunID != nil &&
		payload.ClaimID != nil &&
		payload.ClaimGeneration != nil &&
		payload.RuntimeInstanceID != nil &&
		payload.AgentInstanceID != nil &&
		*payload.WorkItemID == state.workItemID &&
		*payload.RunID == state.runID &&
		*payload.ClaimID == state.claimID &&
		*payload.ClaimGeneration == state.claimGeneration &&
		*payload.RuntimeInstanceID == state.runtimeInstanceID &&
		*payload.AgentInstanceID == state.agentInstanceID
}

func grantProjectionTerminalMatches(
	payload runProjectionTerminalPayload,
	state grantProjectionRunState,
) bool {
	return payload.WorkItemID != nil &&
		payload.RunID != nil &&
		payload.ClaimID != nil &&
		payload.ClaimGeneration != nil &&
		payload.RuntimeInstanceID != nil &&
		payload.AgentInstanceID != nil &&
		payload.Status != nil &&
		payload.Reason != nil &&
		*payload.WorkItemID == state.workItemID &&
		*payload.RunID == state.runID &&
		*payload.ClaimID == state.claimID &&
		*payload.ClaimGeneration == state.claimGeneration &&
		*payload.RuntimeInstanceID == state.runtimeInstanceID &&
		*payload.AgentInstanceID == state.agentInstanceID &&
		!state.terminal
}

func grantProjectionBindingMatchesIssue(
	payload grantProjectionIssuePayload,
	state grantProjectionRunState,
) bool {
	return *payload.WorkItemID == state.workItemID &&
		*payload.RunID == state.runID &&
		*payload.ClaimID == state.claimID &&
		*payload.ClaimGeneration == state.claimGeneration &&
		*payload.RuntimeInstanceID == state.runtimeInstanceID &&
		*payload.AgentInstanceID == state.agentInstanceID
}

func grantProjectionBindingMatchesAuthorization(
	payload grantProjectionAuthorizePayload,
	record AgentGrant,
	state grantProjectionRunState,
) bool {
	return *payload.WorkItemID == record.WorkItemID &&
		*payload.RunID == record.RunID &&
		*payload.ClaimID == record.ClaimID &&
		*payload.ClaimGeneration == record.ClaimGeneration &&
		*payload.RuntimeInstanceID == record.RuntimeInstanceID &&
		*payload.AgentInstanceID == record.AgentInstanceID &&
		state.workItemID == record.WorkItemID &&
		state.runID == record.RunID &&
		state.claimID == record.ClaimID &&
		state.claimGeneration == record.ClaimGeneration &&
		state.runtimeInstanceID == record.RuntimeInstanceID &&
		state.agentInstanceID == record.AgentInstanceID
}

func grantProjectionStateAuthorizable(
	state grantProjectionRunState,
	at time.Time,
) bool {
	if state.terminal {
		return false
	}
	switch state.phase {
	case "claimed":
		return at.Before(state.leaseExpiresAt)
	case "running":
		return true
	default:
		return false
	}
}

func validGrantProjectionEnvelope(event journal.Event) bool {
	return strings.HasPrefix(event.StreamID, grantProjectionStreamPrefix) &&
		strings.TrimPrefix(event.StreamID, grantProjectionStreamPrefix) != "" &&
		event.ID != "" &&
		event.IdempotencyKey == event.ID &&
		event.SchemaVersion == 1 &&
		event.EmittedAt.Location() == time.UTC &&
		validRunProjectionCanonicalUUID(event.CorrelationID)
}

func validGrantProjectionHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validGrantProjectionOperations(operations []string) bool {
	if len(operations) == 0 || len(operations) > 7 ||
		!sort.StringsAreSorted(operations) {
		return false
	}
	for index, operation := range operations {
		if !validGrantProjectionOperation(operation) ||
			index > 0 && operations[index-1] == operation {
			return false
		}
	}
	return true
}

func validGrantProjectionOperation(operation string) bool {
	switch operation {
	case "bridge.ack",
		"bridge.event",
		"bridge.evidence",
		"bridge.result",
		"bridge.heartbeat",
		"context.read",
		"evidence.stage":
		return true
	default:
		return false
	}
}

func grantProjectionHasOperation(operations []string, wanted string) bool {
	index := sort.SearchStrings(operations, wanted)
	return index < len(operations) && operations[index] == wanted
}

func validGrantProjectionRevocation(reason string) bool {
	switch reason {
	case "replaced", "expired", "terminal", "cancelled", "timeout", "operator":
		return true
	default:
		return false
	}
}
