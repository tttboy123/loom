package work

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidRunAuthorityInput = errors.New("invalid run authority input")
	ErrRunAuthorityConflict     = errors.New("run authority conflict")
	ErrRunNotClaimable          = errors.New("run not claimable")
	ErrRunLeaseActive           = errors.New("run prepare lease active")
	ErrRunLeaseExpired          = errors.New("run prepare lease expired")
	ErrStaleClaimGeneration     = errors.New("stale claim generation")
	ErrRunAlreadyTerminal       = errors.New("run already terminal")
	ErrRuntimeUnavailable       = errors.New("runtime unavailable")
	ErrRuntimeCapacityExhausted = errors.New("runtime capacity exhausted")
	ErrRunIdentityIndexRequired = errors.New("Run identity index required")
)

const (
	maxAuthorityIDBytes = 128
	maxPrepareLease     = 5 * time.Minute
	runIdentityStreamID = "work-run-identity/v1"
)

type WorkItemAssignmentInput struct {
	WorkItemID      string
	Title           string
	RunID           string
	AgentInstanceID string
	CorrelationID   string
}

type RunClaimInput struct {
	WorkItemID           string
	RunID                string
	RuntimeInstanceID    string
	AgentInstanceID      string
	PrepareLeaseDuration time.Duration
	CorrelationID        string
}

type RunGenerationInput struct {
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	CorrelationID     string
}

type RunTerminalInput struct {
	RunGenerationInput
	Status string
	Reason string
}

type WorkItemRecord struct {
	id                          string
	title                       string
	status                      string
	runID                       string
	agentInstanceID             string
	approvalRequestID           string
	approvalRequestDigest       string
	verificationEventID         string
	verificationClaimGeneration int64
	acceptanceDecisionDigest    string
	sourceEvidenceID            string
	sourceEvidenceDigest        string
	verifierRequired            bool
	verifierEvidenceID          string
	verifierEvidenceDigest      string
	lastEventID                 string
	streamSequence              int64
}

type RunRecord struct {
	id                            string
	workItemID                    string
	phase                         string
	claimID                       string
	claimGeneration               int64
	runtimeInstanceID             string
	agentInstanceID               string
	prepareLeaseExpiresAt         time.Time
	terminalStatus                string
	terminalReason                string
	lastEventID                   string
	streamSequence                int64
	AssetRevisionBindings         []AssetRevisionBinding
	AssetRevisionSetDigest        string
	MaterializationManifestDigest string
	MaterializationRootDigest     string
}

// AssetRevisionBinding is the execution authority's immutable wire copy. The
// asset package owns lifecycle policy; the Run authority stores only exact
// lineage and therefore does not import or call that higher-level authority.
type AssetRevisionBinding struct {
	AssetKind    string `json:"asset_kind"`
	DefinitionID string `json:"definition_id"`
	RevisionID   string `json:"revision_id"`
	SHA256Digest string `json:"sha256_digest"`
	SourceScope  string `json:"source_scope"`
}

type AuthoritySnapshot struct {
	workItems []WorkItemRecord
	runs      []RunRecord
}

type Authority struct {
	store         *journal.Store
	now           func() time.Time
	random        io.Reader
	randomMu      sync.Mutex
	identityMu    sync.RWMutex
	identityReady bool
}

type authorityState struct {
	workItems              map[string]WorkItemRecord
	runs                   map[string]RunRecord
	runtimes               map[string]authorityRuntime
	heads                  map[string]int64
	runIdentities          map[string]runIdentityReservation
	runIdentityInitialized bool
}

type runIdentityReservation struct {
	runID              string
	workItemID         string
	agentInstanceID    string
	assignmentStreamID string
	assignmentSequence int64
	assignmentEventID  string
}

type authorityRuntime struct {
	id          string
	deviceID    string
	adapterType string
	status      string
	capacity    int
	statusHead  runtimeStatusReference
	statusFacts map[int64]runtimeStatusFact
	active      map[string]capacityBinding
}

type runtimeStatusReference struct {
	streamID string
	sequence int64
	eventID  string
}

type runtimeStatusFact struct {
	reference runtimeStatusReference
	status    string
	capacity  int
}

type capacityBinding struct {
	workItemID        string
	runID             string
	claimID           string
	claimGeneration   int64
	runtimeInstanceID string
	agentInstanceID   string
}

type claimReplay struct {
	binding         capacityBinding
	previousBinding capacityBinding
	statusReference runtimeStatusReference
	eventID         string
	needsOldRelease bool
	reserved        bool
	oldReleased     bool
}

type terminalReplay struct {
	binding         capacityBinding
	statusReference runtimeStatusReference
	eventID         string
	status          string
	released        bool
	outcome         bool
}

func NewAuthority(
	store *journal.Store,
	now func() time.Time,
	random io.Reader,
) (*Authority, error) {
	if store == nil || now == nil || nilInterface(random) {
		return nil, ErrInvalidRunAuthorityInput
	}
	return &Authority{store: store, now: now, random: random}, nil
}

func (authority *Authority) InitializeRunIdentityIndex(ctx context.Context) error {
	if authority == nil || ctx == nil {
		return ErrInvalidRunAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	now, err := authority.operationTime()
	if err != nil {
		return err
	}
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return err
	}
	state, err := replayAuthorityEvents(ctx, events)
	if err != nil {
		return err
	}
	assignments := make([]runIdentityReservation, 0, len(state.runs))
	seenRuns := make(map[string]runIdentityReservation, len(state.runs))
	for _, event := range events {
		if event.Type != "WorkItemAssigned" ||
			!strings.HasPrefix(event.StreamID, "work-item/") {
			continue
		}
		var payload struct {
			WorkItemID      *string `json:"work_item_id"`
			RunID           *string `json:"run_id"`
			AgentInstanceID *string `json:"agent_instance_id"`
			Status          *string `json:"status"`
		}
		if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
			payload.WorkItemID == nil || payload.RunID == nil ||
			payload.AgentInstanceID == nil || payload.Status == nil ||
			*payload.Status != "assigned" {
			return ErrRunAuthorityConflict
		}
		reservation := runIdentityReservation{
			runID: *payload.RunID, workItemID: *payload.WorkItemID,
			agentInstanceID:    *payload.AgentInstanceID,
			assignmentStreamID: event.StreamID,
			assignmentSequence: event.Seq,
			assignmentEventID:  event.ID,
		}
		if existing, exists := seenRuns[reservation.runID]; exists &&
			existing != reservation {
			return ErrRunAuthorityConflict
		}
		seenRuns[reservation.runID] = reservation
	}
	for _, reservation := range seenRuns {
		assignments = append(assignments, reservation)
	}
	sort.Slice(assignments, func(i, j int) bool {
		if assignments[i].runID != assignments[j].runID {
			return assignments[i].runID < assignments[j].runID
		}
		return assignments[i].assignmentEventID < assignments[j].assignmentEventID
	})
	eventIDs := make([]string, len(assignments))
	for index, reservation := range assignments {
		eventIDs[index] = reservation.assignmentEventID
	}
	digest := sha256.Sum256([]byte(strings.Join(eventIDs, "\n")))
	indexDigest := hex.EncodeToString(digest[:])
	if state.runIdentityInitialized {
		if !runIdentityMatches(state.runIdentities, seenRuns) {
			return ErrRunAuthorityConflict
		}
		authority.setRunIdentityReady()
		return nil
	}
	pending := make([]runIdentityReservation, 0, len(assignments))
	for _, reservation := range assignments {
		existing, exists := state.runIdentities[reservation.runID]
		if exists {
			if existing != reservation {
				return ErrRunAuthorityConflict
			}
			continue
		}
		pending = append(pending, reservation)
	}
	identityHead := state.heads[runIdentityStreamID]
	for offset := 0; offset < len(pending); offset += 15 {
		end := offset + 15
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[offset:end]
		expectations := []journal.StreamHeadExpectation{{
			StreamID: runIdentityStreamID, Sequence: identityHead,
		}}
		reservationEvents := make([]journal.Event, 0, len(batch))
		for _, reservation := range batch {
			expectations = append(expectations, journal.StreamHeadExpectation{
				StreamID: reservation.assignmentStreamID,
				Sequence: reservation.assignmentSequence,
			})
			identityHead++
			eventID := deterministicEventID(
				"WorkRunIdentityReserved", reservation.runID,
				reservation.workItemID, reservation.assignmentEventID,
			)
			reservationEvents = append(reservationEvents, newEvent(
				eventID, runIdentityStreamID, identityHead,
				"WorkRunIdentityReserved", now,
				"00000000-0000-4000-8000-000000000001",
				reservation.assignmentEventID,
				runIdentityPayload(reservation),
			))
		}
		if _, err := authority.store.AppendBatchIfStreamHeads(
			ctx,
			expectations,
			reservationEvents,
		); err != nil {
			return mapJournalWriteError(err)
		}
	}
	markerID := deterministicEventID(
		"WorkRunIdentityIndexInitialized",
		indexDigest,
	)
	marker := newEvent(
		markerID, runIdentityStreamID, identityHead+1,
		"WorkRunIdentityIndexInitialized", now,
		"00000000-0000-4000-8000-000000000001", "",
		struct {
			AssignmentDigest string `json:"assignment_digest"`
			AssignmentCount  int    `json:"assignment_count"`
		}{indexDigest, len(assignments)},
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: runIdentityStreamID,
			Sequence: identityHead,
		}},
		[]journal.Event{marker},
	); err != nil {
		return mapJournalWriteError(err)
	}
	authority.setRunIdentityReady()
	return nil
}

func (authority *Authority) CreateAndAssign(
	ctx context.Context,
	input WorkItemAssignmentInput,
) (WorkItemRecord, RunRecord, error) {
	if err := validateContextAndAssignment(ctx, input); err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if !authority.runIdentityReady() {
		return WorkItemRecord{}, RunRecord{}, ErrRunIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	streamID := workItemStream(input.WorkItemID)
	state, err := authority.readStateFor(ctx, []string{
		streamID,
		runIdentityStreamID,
	})
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if !state.runIdentityInitialized {
		return WorkItemRecord{}, RunRecord{}, ErrRunIdentityIndexRequired
	}
	if existing, ok := state.workItems[input.WorkItemID]; ok {
		run := state.runs[input.RunID]
		if existing.title == input.Title &&
			existing.runID == input.RunID &&
			existing.agentInstanceID == input.AgentInstanceID &&
			run.id == input.RunID &&
			run.workItemID == input.WorkItemID &&
			run.agentInstanceID == input.AgentInstanceID {
			return existing.public(), run.public(), nil
		}
		return WorkItemRecord{}, RunRecord{}, ErrRunAuthorityConflict
	}
	if _, exists := state.runs[input.RunID]; exists {
		return WorkItemRecord{}, RunRecord{}, ErrRunAuthorityConflict
	}
	if _, exists := state.runIdentities[input.RunID]; exists {
		return WorkItemRecord{}, RunRecord{}, ErrRunAuthorityConflict
	}

	createdID := deterministicEventID(
		"WorkItemCreated", input.WorkItemID, input.Title, input.CorrelationID,
	)
	assignedID := deterministicEventID(
		"WorkItemAssigned", input.WorkItemID, input.RunID,
		input.AgentInstanceID, input.CorrelationID,
	)
	events := []journal.Event{
		newEvent(
			createdID, streamID, 1, "WorkItemCreated", now,
			input.CorrelationID, "",
			struct {
				WorkItemID string `json:"work_item_id"`
				Title      string `json:"title"`
				Status     string `json:"status"`
			}{input.WorkItemID, input.Title, "ready"},
		),
		newEvent(
			assignedID, streamID, 2, "WorkItemAssigned", now,
			input.CorrelationID, createdID,
			struct {
				WorkItemID      string `json:"work_item_id"`
				RunID           string `json:"run_id"`
				AgentInstanceID string `json:"agent_instance_id"`
				Status          string `json:"status"`
			}{input.WorkItemID, input.RunID, input.AgentInstanceID, "assigned"},
		),
	}
	identitySequence := state.heads[runIdentityStreamID] + 1
	events = append(events, newEvent(
		deterministicEventID(
			"WorkRunIdentityReserved", input.RunID,
			input.WorkItemID, assignedID,
		),
		runIdentityStreamID,
		identitySequence,
		"WorkRunIdentityReserved",
		now,
		input.CorrelationID,
		assignedID,
		runIdentityPayload(runIdentityReservation{
			runID: input.RunID, workItemID: input.WorkItemID,
			agentInstanceID:    input.AgentInstanceID,
			assignmentStreamID: streamID, assignmentSequence: 2,
			assignmentEventID: assignedID,
		}),
	))
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{
			{StreamID: streamID, Sequence: 0},
			{StreamID: runIdentityStreamID, Sequence: state.heads[runIdentityStreamID]},
		},
		events,
	); err != nil {
		return WorkItemRecord{}, RunRecord{}, mapJournalWriteError(err)
	}
	workItem := WorkItemRecord{
		id: input.WorkItemID, title: input.Title, status: "assigned",
		runID: input.RunID, agentInstanceID: input.AgentInstanceID,
		lastEventID: assignedID, streamSequence: 2,
	}
	run := RunRecord{
		id: input.RunID, workItemID: input.WorkItemID, phase: "unclaimed",
		agentInstanceID: input.AgentInstanceID, lastEventID: assignedID,
	}
	return workItem.public(), run.public(), nil
}

func (authority *Authority) Claim(
	ctx context.Context,
	input RunClaimInput,
) (WorkItemRecord, RunRecord, error) {
	if err := validateContextAndClaim(ctx, input); err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if !authority.runIdentityReady() {
		return WorkItemRecord{}, RunRecord{}, ErrRunIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	state, err := authority.readStateFor(ctx, runCommandStreams(
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	))
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	workItem, run, err := claimableRecords(state, input, now)
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	runtime, ok := state.runtimes[input.RuntimeInstanceID]
	if !ok || runtime.status != "online" || runtime.capacity <= 0 {
		return WorkItemRecord{}, RunRecord{}, ErrRuntimeUnavailable
	}
	if !runtime.currentStatusHead(state.heads) {
		return WorkItemRecord{}, RunRecord{}, ErrRunAuthorityConflict
	}
	reclaimingOwnReservation := run.phase == "claimed" &&
		run.claimGeneration > 0 &&
		runtime.active[capacityKey(run.id, run.claimGeneration)].runID == run.id
	if len(runtime.active) >= runtime.capacity && !reclaimingOwnReservation {
		return WorkItemRecord{}, RunRecord{}, ErrRuntimeCapacityExhausted
	}
	claimID, err := authority.newClaimID()
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}

	generation := run.claimGeneration + 1
	expiresAt := now.Add(input.PrepareLeaseDuration)
	runStreamID := runStream(input.RunID)
	statusStreamID := runtimeStatusStream(input.RuntimeInstanceID)
	capacityStreamID := runtimeCapacityStream(input.RuntimeInstanceID)
	runSequence := state.heads[runStreamID] + 1
	capacitySequence := state.heads[capacityStreamID] + 1
	statusReference := runtime.statusHead.payload()
	claimEventID := deterministicEventID(
		"RunClaimed", input.RunID, claimID,
		fmt.Sprint(generation), input.CorrelationID,
	)
	claimPayload := struct {
		WorkItemID            string `json:"work_item_id"`
		RunID                 string `json:"run_id"`
		ClaimID               string `json:"claim_id"`
		ClaimGeneration       int64  `json:"claim_generation"`
		RuntimeInstanceID     string `json:"runtime_instance_id"`
		AgentInstanceID       string `json:"agent_instance_id"`
		PrepareLeaseExpiresAt string `json:"prepare_lease_expires_at"`
		runtimeStatusReferencePayload
	}{
		input.WorkItemID, input.RunID, claimID, generation,
		input.RuntimeInstanceID, input.AgentInstanceID,
		expiresAt.Format(time.RFC3339Nano),
		statusReference,
	}
	capacityPayload := capacityEventPayload{
		WorkItemID: input.WorkItemID, RunID: input.RunID, ClaimID: claimID,
		ClaimGeneration: generation, RuntimeInstanceID: input.RuntimeInstanceID,
		AgentInstanceID:               input.AgentInstanceID,
		runtimeStatusReferencePayload: statusReference,
	}
	events := make([]journal.Event, 0, 3)
	events = append(events, newEvent(
		claimEventID, runStreamID, runSequence, "RunClaimed", now,
		input.CorrelationID, run.lastEventID, claimPayload,
	))
	if generation > 1 {
		releaseID := deterministicEventID(
			"RuntimeCapacityReleased", input.RunID,
			run.claimID, fmt.Sprint(run.claimGeneration), claimEventID,
		)
		events = append(events, newEvent(
			releaseID, capacityStreamID, capacitySequence,
			"RuntimeCapacityReleased", now, input.CorrelationID, claimEventID,
			capacityEventPayload{
				WorkItemID: input.WorkItemID, RunID: input.RunID,
				ClaimID: run.claimID, ClaimGeneration: run.claimGeneration,
				RuntimeInstanceID:             run.runtimeInstanceID,
				AgentInstanceID:               run.agentInstanceID,
				runtimeStatusReferencePayload: statusReference,
			},
		))
		capacitySequence++
	}
	reserveID := deterministicEventID(
		"RuntimeCapacityReserved", input.RunID, claimID,
		fmt.Sprint(generation), claimEventID,
	)
	events = append(events, newEvent(
		reserveID, capacityStreamID, capacitySequence,
		"RuntimeCapacityReserved", now, input.CorrelationID, claimEventID,
		capacityPayload,
	))
	expectations := []journal.StreamHeadExpectation{
		{StreamID: workItemStream(input.WorkItemID), Sequence: state.heads[workItemStream(input.WorkItemID)]},
		{StreamID: runStreamID, Sequence: state.heads[runStreamID]},
		{StreamID: statusStreamID, Sequence: runtime.statusHead.sequence},
		{StreamID: capacityStreamID, Sequence: state.heads[capacityStreamID]},
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx, expectations, events); err != nil {
		return WorkItemRecord{}, RunRecord{}, mapJournalWriteError(err)
	}
	run.phase = "claimed"
	run.claimID = claimID
	run.claimGeneration = generation
	run.runtimeInstanceID = input.RuntimeInstanceID
	run.agentInstanceID = input.AgentInstanceID
	run.prepareLeaseExpiresAt = expiresAt
	run.lastEventID = claimEventID
	run.streamSequence = runSequence
	return workItem.public(), run.public(), nil
}

func (authority *Authority) ExtendPrepareLease(
	ctx context.Context,
	input RunGenerationInput,
	duration time.Duration,
) (RunRecord, error) {
	if err := validateContextAndGeneration(ctx, input); err != nil ||
		duration <= 0 || duration > maxPrepareLease {
		return RunRecord{}, ErrInvalidRunAuthorityInput
	}
	if !authority.runIdentityReady() {
		return RunRecord{}, ErrRunIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return RunRecord{}, err
	}
	state, err := authority.readStateFor(ctx, runCommandStreams(
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	))
	if err != nil {
		return RunRecord{}, err
	}
	run, err := currentGenerationRun(state, input)
	if err != nil {
		return RunRecord{}, err
	}
	if run.phase != "claimed" {
		return RunRecord{}, ErrRunNotClaimable
	}
	if !now.Before(run.prepareLeaseExpiresAt) {
		return RunRecord{}, ErrRunLeaseExpired
	}
	expiresAt := now.Add(duration)
	if !expiresAt.After(run.prepareLeaseExpiresAt) {
		return RunRecord{}, ErrInvalidRunAuthorityInput
	}
	eventID := deterministicEventID(
		"RunPrepareLeaseExtended", input.RunID, input.ClaimID,
		fmt.Sprint(input.ClaimGeneration), expiresAt.Format(time.RFC3339Nano),
	)
	event := newEvent(
		eventID, runStream(input.RunID), run.streamSequence+1,
		"RunPrepareLeaseExtended", now, input.CorrelationID, run.lastEventID,
		struct {
			WorkItemID             string `json:"work_item_id"`
			RunID                  string `json:"run_id"`
			ClaimID                string `json:"claim_id"`
			ClaimGeneration        int64  `json:"claim_generation"`
			RuntimeInstanceID      string `json:"runtime_instance_id"`
			AgentInstanceID        string `json:"agent_instance_id"`
			PreviousLeaseExpiresAt string `json:"previous_lease_expires_at"`
			PrepareLeaseExpiresAt  string `json:"prepare_lease_expires_at"`
		}{
			input.WorkItemID, input.RunID, input.ClaimID, input.ClaimGeneration,
			input.RuntimeInstanceID, input.AgentInstanceID,
			run.prepareLeaseExpiresAt.Format(time.RFC3339Nano),
			expiresAt.Format(time.RFC3339Nano),
		},
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		runCommandExpectations(state, input),
		[]journal.Event{event},
	); err != nil {
		return RunRecord{}, mapJournalWriteError(err)
	}
	run.prepareLeaseExpiresAt = expiresAt
	run.lastEventID = eventID
	run.streamSequence++
	return run.public(), nil
}

func (authority *Authority) Start(
	ctx context.Context,
	input RunGenerationInput,
) (WorkItemRecord, RunRecord, error) {
	if err := validateContextAndGeneration(ctx, input); err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if !authority.runIdentityReady() {
		return WorkItemRecord{}, RunRecord{}, ErrRunIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	state, err := authority.readStateFor(ctx, runCommandStreams(
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	))
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	run, err := currentGenerationRun(state, input)
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if run.phase != "claimed" {
		return WorkItemRecord{}, RunRecord{}, ErrRunNotClaimable
	}
	if !now.Before(run.prepareLeaseExpiresAt) {
		return WorkItemRecord{}, RunRecord{}, ErrRunLeaseExpired
	}
	runtime, ok := state.runtimes[input.RuntimeInstanceID]
	if !ok || runtime.status != "online" {
		return WorkItemRecord{}, RunRecord{}, ErrRuntimeUnavailable
	}
	if !runtime.currentStatusHead(state.heads) {
		return WorkItemRecord{}, RunRecord{}, ErrRunAuthorityConflict
	}
	workItem := state.workItems[input.WorkItemID]
	eventID := deterministicEventID(
		"RunStarted", input.RunID, input.ClaimID,
		fmt.Sprint(input.ClaimGeneration),
	)
	event := newEvent(
		eventID, runStream(input.RunID), run.streamSequence+1,
		"RunStarted", now, input.CorrelationID, run.lastEventID,
		generationEventPayload(input, runtime.statusHead.payload()),
	)
	expectations := []journal.StreamHeadExpectation{
		{StreamID: workItemStream(input.WorkItemID), Sequence: state.heads[workItemStream(input.WorkItemID)]},
		{StreamID: runStream(input.RunID), Sequence: run.streamSequence},
		{StreamID: runtimeStatusStream(input.RuntimeInstanceID), Sequence: runtime.statusHead.sequence},
		{StreamID: runtimeCapacityStream(input.RuntimeInstanceID), Sequence: state.heads[runtimeCapacityStream(input.RuntimeInstanceID)]},
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(ctx, expectations, []journal.Event{event}); err != nil {
		return WorkItemRecord{}, RunRecord{}, mapJournalWriteError(err)
	}
	workItem.status = "running"
	run.phase = "running"
	run.lastEventID = eventID
	run.streamSequence++
	return workItem.public(), run.public(), nil
}

func (authority *Authority) CommitTerminal(
	ctx context.Context,
	input RunTerminalInput,
) (WorkItemRecord, RunRecord, error) {
	if err := validateTerminalInput(ctx, input); err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if !authority.runIdentityReady() {
		return WorkItemRecord{}, RunRecord{}, ErrRunIdentityIndexRequired
	}
	now, err := authority.operationTime()
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	state, err := authority.readStateFor(ctx, runCommandStreams(
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	))
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	run, ok := state.runs[input.RunID]
	if !ok || run.workItemID != input.WorkItemID {
		return WorkItemRecord{}, RunRecord{}, ErrRunNotClaimable
	}
	if run.phase == "terminal" {
		workItem := state.workItems[input.WorkItemID]
		if run.claimID == input.ClaimID &&
			run.claimGeneration == input.ClaimGeneration &&
			run.runtimeInstanceID == input.RuntimeInstanceID &&
			run.agentInstanceID == input.AgentInstanceID &&
			run.terminalStatus == input.Status &&
			run.terminalReason == input.Reason {
			return workItem.public(), run.public(), nil
		}
		return WorkItemRecord{}, RunRecord{}, ErrRunAlreadyTerminal
	}
	run, err = currentGenerationRun(state, input.RunGenerationInput)
	if err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	if run.phase != "claimed" && run.phase != "running" {
		return WorkItemRecord{}, RunRecord{}, ErrRunNotClaimable
	}
	runtime, ok := state.runtimes[input.RuntimeInstanceID]
	if !ok {
		return WorkItemRecord{}, RunRecord{}, ErrRuntimeUnavailable
	}
	if !runtime.currentStatusHead(state.heads) {
		return WorkItemRecord{}, RunRecord{}, ErrRunAuthorityConflict
	}
	statusReference := runtime.statusHead.payload()
	workItem := state.workItems[input.WorkItemID]
	terminalID := deterministicEventID(
		"RunTerminalCommitted", input.RunID, input.ClaimID,
		fmt.Sprint(input.ClaimGeneration), input.Status, input.Reason,
	)
	runEvent := newEvent(
		terminalID, runStream(input.RunID), run.streamSequence+1,
		"RunTerminalCommitted", now, input.CorrelationID, run.lastEventID,
		struct {
			WorkItemID        string `json:"work_item_id"`
			RunID             string `json:"run_id"`
			ClaimID           string `json:"claim_id"`
			ClaimGeneration   int64  `json:"claim_generation"`
			RuntimeInstanceID string `json:"runtime_instance_id"`
			AgentInstanceID   string `json:"agent_instance_id"`
			Status            string `json:"status"`
			Reason            string `json:"reason"`
			runtimeStatusReferencePayload
		}{
			input.WorkItemID, input.RunID, input.ClaimID, input.ClaimGeneration,
			input.RuntimeInstanceID, input.AgentInstanceID, input.Status, input.Reason,
			statusReference,
		},
	)
	workSequence := state.heads[workItemStream(input.WorkItemID)] + 1
	workStatus := input.Status
	workEventType := "WorkItemTerminal"
	if input.Status == "succeeded" {
		workStatus = "ready_for_review"
		workEventType = "WorkItemReadyForReview"
	}
	workEventID := deterministicEventID(
		workEventType, input.WorkItemID, input.RunID,
		fmt.Sprint(input.ClaimGeneration), workStatus,
	)
	workEvent := newEvent(
		workEventID, workItemStream(input.WorkItemID), workSequence,
		workEventType, now, input.CorrelationID, terminalID,
		struct {
			WorkItemID      string `json:"work_item_id"`
			RunID           string `json:"run_id"`
			ClaimGeneration int64  `json:"claim_generation"`
			Status          string `json:"status"`
		}{input.WorkItemID, input.RunID, input.ClaimGeneration, workStatus},
	)
	capacityStreamID := runtimeCapacityStream(input.RuntimeInstanceID)
	capacitySequence := state.heads[capacityStreamID] + 1
	releaseID := deterministicEventID(
		"RuntimeCapacityReleased", input.RunID, input.ClaimID,
		fmt.Sprint(input.ClaimGeneration), terminalID,
	)
	releaseEvent := newEvent(
		releaseID, capacityStreamID, capacitySequence,
		"RuntimeCapacityReleased", now, input.CorrelationID, terminalID,
		capacityEventPayload{
			WorkItemID: input.WorkItemID, RunID: input.RunID,
			ClaimID: input.ClaimID, ClaimGeneration: input.ClaimGeneration,
			RuntimeInstanceID:             input.RuntimeInstanceID,
			AgentInstanceID:               input.AgentInstanceID,
			runtimeStatusReferencePayload: statusReference,
		},
	)
	expectations := []journal.StreamHeadExpectation{
		{StreamID: workItemStream(input.WorkItemID), Sequence: state.heads[workItemStream(input.WorkItemID)]},
		{StreamID: runStream(input.RunID), Sequence: run.streamSequence},
		{StreamID: runtimeStatusStream(input.RuntimeInstanceID), Sequence: runtime.statusHead.sequence},
		{StreamID: capacityStreamID, Sequence: state.heads[capacityStreamID]},
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx, expectations, []journal.Event{runEvent, workEvent, releaseEvent},
	); err != nil {
		return WorkItemRecord{}, RunRecord{}, mapJournalWriteError(err)
	}
	workItem.status = workStatus
	workItem.lastEventID = workEventID
	workItem.streamSequence = workSequence
	run.phase = "terminal"
	run.terminalStatus = input.Status
	run.terminalReason = input.Reason
	run.lastEventID = terminalID
	run.streamSequence++
	return workItem.public(), run.public(), nil
}

func (authority *Authority) Snapshot(ctx context.Context) (AuthoritySnapshot, error) {
	if ctx == nil {
		return AuthoritySnapshot{}, ErrInvalidRunAuthorityInput
	}
	if err := ctx.Err(); err != nil {
		return AuthoritySnapshot{}, err
	}
	state, err := authority.readState(ctx)
	if err != nil {
		return AuthoritySnapshot{}, err
	}
	workItems := make([]WorkItemRecord, 0, len(state.workItems))
	for _, record := range state.workItems {
		workItems = append(workItems, record.public())
	}
	sort.Slice(workItems, func(i, j int) bool { return workItems[i].id < workItems[j].id })
	runs := make([]RunRecord, 0, len(state.runs))
	for _, record := range state.runs {
		runs = append(runs, record.public())
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].id < runs[j].id })
	return AuthoritySnapshot{workItems: workItems, runs: runs}, nil
}

func (record WorkItemRecord) ID() string              { return record.id }
func (record WorkItemRecord) Title() string           { return record.title }
func (record WorkItemRecord) Status() string          { return record.status }
func (record WorkItemRecord) RunID() string           { return record.runID }
func (record WorkItemRecord) AgentInstanceID() string { return record.agentInstanceID }
func (record WorkItemRecord) VerificationEventID() string {
	return record.verificationEventID
}
func (record WorkItemRecord) AcceptanceDecisionDigest() string {
	return record.acceptanceDecisionDigest
}

func (record RunRecord) ID() string                       { return record.id }
func (record RunRecord) WorkItemID() string               { return record.workItemID }
func (record RunRecord) Phase() string                    { return record.phase }
func (record RunRecord) ClaimID() string                  { return record.claimID }
func (record RunRecord) ClaimGeneration() int64           { return record.claimGeneration }
func (record RunRecord) RuntimeInstanceID() string        { return record.runtimeInstanceID }
func (record RunRecord) AgentInstanceID() string          { return record.agentInstanceID }
func (record RunRecord) PrepareLeaseExpiresAt() time.Time { return record.prepareLeaseExpiresAt }
func (record RunRecord) TerminalStatus() string           { return record.terminalStatus }
func (record RunRecord) TerminalReason() string           { return record.terminalReason }

func (snapshot AuthoritySnapshot) WorkItems() []WorkItemRecord {
	return append([]WorkItemRecord(nil), snapshot.workItems...)
}

func (snapshot AuthoritySnapshot) Runs() []RunRecord {
	return append([]RunRecord(nil), snapshot.runs...)
}

func (authority *Authority) operationTime() (time.Time, error) {
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return time.Time{}, ErrInvalidRunAuthorityInput
	}
	return now, nil
}

func (authority *Authority) newClaimID() (string, error) {
	var raw [16]byte
	authority.randomMu.Lock()
	_, err := io.ReadFull(authority.random, raw[:])
	authority.randomMu.Unlock()
	if err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" +
		encoded[16:20] + "-" + encoded[20:32], nil
}

func (authority *Authority) readState(ctx context.Context) (authorityState, error) {
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return authorityState{}, err
	}
	return replayAuthorityEvents(ctx, events)
}

func (authority *Authority) readStateFor(
	ctx context.Context,
	streamIDs []string,
) (authorityState, error) {
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return authorityState{}, err
	}
	return replayAuthorityEventsSelective(ctx, snapshot.Events())
}

func replayAuthorityEvents(
	ctx context.Context,
	events []journal.Event,
) (authorityState, error) {
	return replayAuthorityEventsMode(ctx, events, false)
}

func replayAuthorityEventsSelective(
	ctx context.Context,
	events []journal.Event,
) (authorityState, error) {
	return replayAuthorityEventsMode(ctx, events, true)
}

func replayAuthorityEventsMode(
	ctx context.Context,
	events []journal.Event,
	allowOrphanCapacity bool,
) (authorityState, error) {
	if ctx == nil {
		return authorityState{}, ErrInvalidRunAuthorityInput
	}
	ordered := append([]journal.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StreamID != ordered[j].StreamID {
			return ordered[i].StreamID < ordered[j].StreamID
		}
		if ordered[i].Seq != ordered[j].Seq {
			return ordered[i].Seq < ordered[j].Seq
		}
		return ordered[i].ID < ordered[j].ID
	})
	state := authorityState{
		workItems:     make(map[string]WorkItemRecord),
		runs:          make(map[string]RunRecord),
		runtimes:      make(map[string]authorityRuntime),
		heads:         make(map[string]int64),
		runIdentities: make(map[string]runIdentityReservation),
	}
	byStream := make(map[string][]journal.Event)
	for _, event := range ordered {
		if err := ctx.Err(); err != nil {
			return authorityState{}, err
		}
		if event.StreamID == "" || event.Seq <= 0 ||
			event.Seq != state.heads[event.StreamID]+1 ||
			isRunAuthorityStream(event.StreamID) &&
				(event.ID == "" || event.IdempotencyKey == "" ||
					event.SchemaVersion != 1 || event.CorrelationID == "" ||
					event.EmittedAt.IsZero() ||
					event.EmittedAt.Location() != time.UTC) {
			return authorityState{}, ErrRunAuthorityConflict
		}
		state.heads[event.StreamID] = event.Seq
		byStream[event.StreamID] = append(byStream[event.StreamID], event)
	}
	if streamEvents := byStream[runIdentityStreamID]; len(streamEvents) > 0 {
		if err := replayRunIdentityStream(&state, streamEvents); err != nil {
			return authorityState{}, err
		}
	}

	outcomes := make(map[string]journal.Event)
	for streamID, streamEvents := range byStream {
		if strings.HasPrefix(streamID, "work-item/") {
			if err := replayWorkItemStream(&state, streamID, streamEvents, outcomes); err != nil {
				return authorityState{}, err
			}
		}
	}
	for streamID, streamEvents := range byStream {
		if strings.HasPrefix(streamID, "runtime_instance:") {
			if err := replayRuntimeIdentityStream(&state, streamID, streamEvents); err != nil {
				return authorityState{}, err
			}
		}
	}
	claims := make(map[string]*claimReplay)
	terminals := make(map[string]*terminalReplay)
	for streamID, streamEvents := range byStream {
		if strings.HasPrefix(streamID, "run/") {
			if err := replayRunStream(&state, streamID, streamEvents, claims, terminals); err != nil {
				return authorityState{}, err
			}
		}
	}
	for streamID, streamEvents := range byStream {
		if strings.HasPrefix(streamID, "runtime_capacity:") {
			if err := replayRuntimeCapacityStream(
				&state,
				streamID,
				streamEvents,
				claims,
				terminals,
				allowOrphanCapacity,
			); err != nil {
				return authorityState{}, err
			}
		}
	}
	for cause, operation := range claims {
		if !operation.reserved || operation.needsOldRelease && !operation.oldReleased {
			return authorityState{}, fmt.Errorf("%w: incomplete claim %s", ErrRunAuthorityConflict, cause)
		}
	}
	for cause, terminal := range terminals {
		outcome, ok := outcomes[cause]
		if !terminal.released || !ok {
			return authorityState{}, fmt.Errorf("%w: incomplete terminal %s", ErrRunAuthorityConflict, cause)
		}
		if err := validateOutcome(outcome, terminal); err != nil {
			return authorityState{}, err
		}
		terminal.outcome = true
		workItem := state.workItems[terminal.binding.workItemID]
		if terminal.status == "succeeded" {
			workItem.status = "ready_for_review"
		} else {
			workItem.status = terminal.status
		}
		workItem.lastEventID = outcome.ID
		workItem.streamSequence = outcome.Seq
		state.workItems[workItem.id] = workItem
	}
	return state, nil
}

func replayRunIdentityStream(
	state *authorityState,
	events []journal.Event,
) error {
	for _, event := range events {
		switch event.Type {
		case "WorkRunIdentityReserved":
			var payload struct {
				RunID              *string `json:"run_id"`
				WorkItemID         *string `json:"work_item_id"`
				AgentInstanceID    *string `json:"agent_instance_id"`
				AssignmentStreamID *string `json:"assignment_stream_id"`
				AssignmentSequence *int64  `json:"assignment_sequence"`
				AssignmentEventID  *string `json:"assignment_event_id"`
			}
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				payload.RunID == nil || payload.WorkItemID == nil ||
				payload.AgentInstanceID == nil || payload.AssignmentStreamID == nil ||
				payload.AssignmentSequence == nil || payload.AssignmentEventID == nil ||
				*payload.RunID == "" || *payload.WorkItemID == "" ||
				*payload.AgentInstanceID == "" ||
				*payload.AssignmentStreamID != workItemStream(*payload.WorkItemID) ||
				*payload.AssignmentSequence <= 0 ||
				*payload.AssignmentEventID == "" ||
				event.CausationID != *payload.AssignmentEventID {
				return ErrRunAuthorityConflict
			}
			reservation := runIdentityReservation{
				runID: *payload.RunID, workItemID: *payload.WorkItemID,
				agentInstanceID:    *payload.AgentInstanceID,
				assignmentStreamID: *payload.AssignmentStreamID,
				assignmentSequence: *payload.AssignmentSequence,
				assignmentEventID:  *payload.AssignmentEventID,
			}
			if existing, exists := state.runIdentities[reservation.runID]; exists {
				if existing != reservation {
					return ErrRunAuthorityConflict
				}
			} else {
				state.runIdentities[reservation.runID] = reservation
			}
		case "WorkRunIdentityIndexInitialized":
			var payload struct {
				AssignmentDigest *string `json:"assignment_digest"`
				AssignmentCount  *int    `json:"assignment_count"`
			}
			if state.runIdentityInitialized ||
				decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.AssignmentDigest == nil ||
				payload.AssignmentCount == nil ||
				!validSHA256Hex(*payload.AssignmentDigest) ||
				*payload.AssignmentCount < 0 {
				return ErrRunAuthorityConflict
			}
			state.runIdentityInitialized = true
		default:
			return ErrRunAuthorityConflict
		}
	}
	return nil
}

func replayWorkItemStream(
	state *authorityState,
	streamID string,
	events []journal.Event,
	outcomes map[string]journal.Event,
) error {
	workItemID := strings.TrimPrefix(streamID, "work-item/")
	var record WorkItemRecord
	for _, event := range events {
		switch event.Type {
		case "WorkItemCreated":
			var payload struct {
				WorkItemID *string `json:"work_item_id"`
				Title      *string `json:"title"`
				Status     *string `json:"status"`
			}
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				payload.WorkItemID == nil || payload.Title == nil || payload.Status == nil ||
				*payload.WorkItemID != workItemID || !validOpaqueID(*payload.Title) ||
				*payload.Status != "ready" || record.id != "" {
				return ErrRunAuthorityConflict
			}
			record = WorkItemRecord{
				id: workItemID, title: *payload.Title, status: "ready",
				lastEventID: event.ID, streamSequence: event.Seq,
			}
		case "WorkItemAssigned":
			var payload struct {
				WorkItemID      *string `json:"work_item_id"`
				RunID           *string `json:"run_id"`
				AgentInstanceID *string `json:"agent_instance_id"`
				Status          *string `json:"status"`
			}
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				record.id == "" || payload.WorkItemID == nil || payload.RunID == nil ||
				payload.AgentInstanceID == nil || payload.Status == nil ||
				*payload.WorkItemID != workItemID || *payload.Status != "assigned" ||
				event.CausationID != record.lastEventID {
				return ErrRunAuthorityConflict
			}
			record.status = "assigned"
			record.runID = *payload.RunID
			record.agentInstanceID = *payload.AgentInstanceID
			record.lastEventID = event.ID
			record.streamSequence = event.Seq
			state.runs[*payload.RunID] = RunRecord{
				id: *payload.RunID, workItemID: workItemID, phase: "unclaimed",
				agentInstanceID: *payload.AgentInstanceID,
				lastEventID:     event.ID,
			}
		case "WorkItemApprovalPaused":
			var payload struct {
				WorkItemID            *string `json:"work_item_id"`
				ApprovalRequestID     *string `json:"approval_request_id"`
				ApprovalRequestDigest *string `json:"approval_request_digest"`
				PreviousStatus        *string `json:"previous_status"`
				Status                *string `json:"status"`
			}
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				payload.WorkItemID == nil ||
				payload.ApprovalRequestID == nil ||
				payload.ApprovalRequestDigest == nil ||
				payload.PreviousStatus == nil ||
				payload.Status == nil ||
				*payload.WorkItemID != workItemID ||
				!validOpaqueID(*payload.ApprovalRequestID) ||
				!validSHA256Hex(*payload.ApprovalRequestDigest) ||
				*payload.PreviousStatus != "assigned" ||
				*payload.Status != "waiting_approval" ||
				record.status != "assigned" ||
				record.approvalRequestID != "" ||
				event.CausationID == "" {
				return ErrRunAuthorityConflict
			}
			record.status = "waiting_approval"
			record.approvalRequestID = *payload.ApprovalRequestID
			record.approvalRequestDigest = *payload.ApprovalRequestDigest
			record.lastEventID = event.ID
			record.streamSequence = event.Seq
		case "WorkItemApprovalResolved":
			var payload struct {
				WorkItemID            *string `json:"work_item_id"`
				ApprovalRequestID     *string `json:"approval_request_id"`
				ApprovalRequestDigest *string `json:"approval_request_digest"`
				PreviousStatus        *string `json:"previous_status"`
				Status                *string `json:"status"`
			}
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				payload.WorkItemID == nil ||
				payload.ApprovalRequestID == nil ||
				payload.ApprovalRequestDigest == nil ||
				payload.PreviousStatus == nil ||
				payload.Status == nil ||
				*payload.WorkItemID != workItemID ||
				*payload.ApprovalRequestID != record.approvalRequestID ||
				*payload.ApprovalRequestDigest != record.approvalRequestDigest ||
				*payload.PreviousStatus != "waiting_approval" ||
				!validApprovalResolutionStatus(*payload.Status) ||
				record.status != "waiting_approval" ||
				event.CausationID == "" {
				return ErrRunAuthorityConflict
			}
			record.status = *payload.Status
			record.approvalRequestID = ""
			record.approvalRequestDigest = ""
			record.lastEventID = event.ID
			record.streamSequence = event.Seq
		case "WorkItemReadyForReview", "WorkItemTerminal":
			if _, duplicate := outcomes[event.CausationID]; duplicate || event.CausationID == "" {
				return ErrRunAuthorityConflict
			}
			var payload struct {
				WorkItemID      *string `json:"work_item_id"`
				RunID           *string `json:"run_id"`
				ClaimGeneration *int64  `json:"claim_generation"`
				Status          *string `json:"status"`
			}
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.WorkItemID == nil || payload.RunID == nil ||
				payload.ClaimGeneration == nil || payload.Status == nil ||
				*payload.WorkItemID != workItemID ||
				*payload.RunID != record.runID ||
				*payload.ClaimGeneration <= 0 ||
				event.Seq != record.streamSequence+1 {
				return ErrRunAuthorityConflict
			}
			if event.Type == "WorkItemReadyForReview" {
				if *payload.Status != "ready_for_review" {
					return ErrRunAuthorityConflict
				}
			} else if *payload.Status != "failed" &&
				*payload.Status != "cancelled" {
				return ErrRunAuthorityConflict
			}
			outcomes[event.CausationID] = event
			record.status = *payload.Status
			record.lastEventID = event.ID
			record.streamSequence = event.Seq
		case "WorkItemVerificationCommitted":
			var payload workItemVerificationPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				!payload.valid(record) ||
				event.ID !=
					workItemVerificationPayloadEventID(payload) ||
				record.status != "ready_for_review" ||
				event.Seq != record.streamSequence+1 ||
				event.CausationID != record.lastEventID {
				return ErrRunAuthorityConflict
			}
			record.verificationEventID = event.ID
			record.verificationClaimGeneration =
				payload.ClaimGeneration
			record.acceptanceDecisionDigest = payload.AcceptanceDecisionDigest
			record.sourceEvidenceID = payload.SourceEvidenceID
			record.sourceEvidenceDigest = payload.SourceEvidenceDigest
			record.verifierRequired = payload.VerifierRequired
			record.verifierEvidenceID = payload.VerifierEvidenceID
			record.verifierEvidenceDigest =
				payload.VerifierEvidenceDigest
			record.lastEventID = event.ID
			record.streamSequence = event.Seq
		case "WorkItemDone", "WorkItemRejected":
			var payload workItemAcceptanceOutcomePayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				!payload.valid(record, event.Type) ||
				event.ID != acceptanceOutcomePayloadEventID(
					payload,
					event.Type,
				) ||
				record.verificationEventID == "" ||
				event.CausationID != record.verificationEventID ||
				event.Seq != record.streamSequence+1 {
				return ErrRunAuthorityConflict
			}
			record.status = payload.Status
			record.lastEventID = event.ID
			record.streamSequence = event.Seq
		default:
			return ErrRunAuthorityConflict
		}
	}
	if record.id != "" {
		state.workItems[record.id] = record
	}
	return nil
}

func replayRuntimeIdentityStream(
	state *authorityState,
	streamID string,
	events []journal.Event,
) error {
	runtimeID := strings.TrimPrefix(streamID, "runtime_instance:")
	runtime := authorityRuntime{
		id:          runtimeID,
		statusFacts: make(map[int64]runtimeStatusFact),
		active:      make(map[string]capacityBinding),
	}
	for _, event := range events {
		switch event.Type {
		case "RuntimeInstanceDiscovered":
			var payload runtimeDiscoveryPayload
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				payload.DiscoveryDigest == nil || payload.SourceProbeID == nil ||
				payload.Instance == nil || payload.ModelIDs == nil ||
				!validSHA256Hex(*payload.DiscoveryDigest) ||
				*payload.SourceProbeID == "" ||
				!validRuntimeDiscoveryInstance(*payload.Instance, runtimeID) ||
				!validRuntimeStatus(*payload.Instance.Status) ||
				*payload.Instance.Capacity < 0 {
				return ErrRunAuthorityConflict
			}
			var modelIDs []string
			if err := json.Unmarshal(payload.ModelIDs, &modelIDs); err != nil ||
				!validOrderedStrings(modelIDs) {
				return ErrRunAuthorityConflict
			}
			if runtime.statusHead.eventID != "" &&
				(runtime.deviceID != *payload.Instance.DeviceID ||
					runtime.adapterType != *payload.Instance.AdapterType) {
				return ErrRunAuthorityConflict
			}
			reference := runtimeStatusReference{
				streamID: streamID,
				sequence: event.Seq,
				eventID:  event.ID,
			}
			runtime.deviceID = *payload.Instance.DeviceID
			runtime.adapterType = *payload.Instance.AdapterType
			runtime.status = *payload.Instance.Status
			runtime.capacity = *payload.Instance.Capacity
			runtime.statusHead = reference
			runtime.statusFacts[event.Seq] = runtimeStatusFact{
				reference: reference,
				status:    runtime.status,
				capacity:  runtime.capacity,
			}
		case "RuntimeInstanceStatusChanged":
			if runtime.statusHead.eventID == "" {
				return ErrRunAuthorityConflict
			}
			var payload runtimeStatusPayload
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				payload.ReconciliationDigest == nil ||
				payload.BaselineDigest == nil ||
				payload.SourceDiscoveryDigest == nil ||
				payload.SourceProbeID == nil ||
				payload.RuntimeInstanceID == nil ||
				payload.DeviceID == nil ||
				payload.AdapterType == nil ||
				payload.FromStatus == nil ||
				payload.ToStatus == nil || payload.PreviousEventID == nil ||
				payload.PreviousSequence == nil ||
				!validSHA256Hex(*payload.ReconciliationDigest) ||
				!validSHA256Hex(*payload.BaselineDigest) ||
				!validSHA256Hex(*payload.SourceDiscoveryDigest) ||
				*payload.SourceProbeID == "" ||
				*payload.RuntimeInstanceID != runtimeID ||
				*payload.DeviceID != runtime.deviceID ||
				*payload.AdapterType != runtime.adapterType ||
				!validRuntimeStatus(*payload.FromStatus) ||
				!validRuntimeStatus(*payload.ToStatus) ||
				*payload.FromStatus == *payload.ToStatus ||
				*payload.FromStatus != runtime.status ||
				*payload.PreviousEventID != runtime.statusHead.eventID ||
				*payload.PreviousSequence != runtime.statusHead.sequence ||
				event.CausationID != runtime.statusHead.eventID ||
				event.Seq != runtime.statusHead.sequence+1 {
				return ErrRunAuthorityConflict
			}
			reference := runtimeStatusReference{
				streamID: streamID,
				sequence: event.Seq,
				eventID:  event.ID,
			}
			runtime.status = *payload.ToStatus
			runtime.statusHead = reference
			runtime.statusFacts[event.Seq] = runtimeStatusFact{
				reference: reference,
				status:    runtime.status,
				capacity:  runtime.capacity,
			}
		default:
			return ErrRunAuthorityConflict
		}
	}
	if runtime.statusHead.eventID == "" ||
		!runtime.currentStatusHead(state.heads) {
		return ErrRunAuthorityConflict
	}
	state.runtimes[runtime.id] = runtime
	return nil
}

func replayRunStream(
	state *authorityState,
	streamID string,
	events []journal.Event,
	claims map[string]*claimReplay,
	terminals map[string]*terminalReplay,
) error {
	runID := strings.TrimPrefix(streamID, "run/")
	run, ok := state.runs[runID]
	if !ok {
		return ErrRunAuthorityConflict
	}
	for _, event := range events {
		if event.CausationID != run.lastEventID ||
			event.Seq != run.streamSequence+1 {
			return ErrRunAuthorityConflict
		}
		switch event.Type {
		case "RunClaimed":
			var payload runClaimedPayload
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil {
				return fmt.Errorf("%w: decode RunClaimed %s: %v", ErrRunAuthorityConflict, event.ID, err)
			}
			if !payload.valid(run, runID) {
				return fmt.Errorf("%w: invalid RunClaimed %s", ErrRunAuthorityConflict, event.ID)
			}
			if payload.ClaimGeneration == nil || *payload.ClaimGeneration != run.claimGeneration+1 {
				return fmt.Errorf("%w: RunClaimed generation %s", ErrRunAuthorityConflict, event.ID)
			}
			expiresAt, err := parseUTC(*payload.PrepareLeaseExpiresAt)
			if err != nil || run.phase == "running" || run.phase == "terminal" ||
				run.phase == "claimed" && event.EmittedAt.Before(run.prepareLeaseExpiresAt) {
				return ErrRunAuthorityConflict
			}
			statusReference := payload.runtimeStatusReferenceFields.reference()
			runtime, exists := state.runtimes[*payload.RuntimeInstanceID]
			if !exists || !runtime.validatesStatusReference(statusReference, true) {
				return ErrRunAuthorityConflict
			}
			previousBinding := capacityBinding{
				workItemID: run.workItemID, runID: run.id,
				claimID: run.claimID, claimGeneration: run.claimGeneration,
				runtimeInstanceID: run.runtimeInstanceID,
				agentInstanceID:   run.agentInstanceID,
			}
			previousGeneration := run.claimGeneration
			run.phase = "claimed"
			run.claimID = *payload.ClaimID
			run.claimGeneration = *payload.ClaimGeneration
			run.runtimeInstanceID = *payload.RuntimeInstanceID
			run.agentInstanceID = *payload.AgentInstanceID
			run.prepareLeaseExpiresAt = expiresAt
			if payload.AssetRevisionBindings != nil {
				run.AssetRevisionBindings = append([]AssetRevisionBinding(nil), (*payload.AssetRevisionBindings)...)
				run.AssetRevisionSetDigest = *payload.AssetRevisionSetDigest
				run.MaterializationManifestDigest = *payload.MaterializationManifestDigest
				run.MaterializationRootDigest = *payload.MaterializationRootDigest
			}
			run.lastEventID = event.ID
			run.streamSequence = event.Seq
			claims[event.ID] = &claimReplay{
				binding: capacityBinding{
					workItemID: run.workItemID, runID: run.id,
					claimID: run.claimID, claimGeneration: run.claimGeneration,
					runtimeInstanceID: run.runtimeInstanceID,
					agentInstanceID:   run.agentInstanceID,
				},
				previousBinding: previousBinding,
				statusReference: statusReference,
				eventID:         event.ID, needsOldRelease: previousGeneration > 0,
			}
		case "RunPrepareLeaseExtended":
			var payload leaseExtendedPayload
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				!payload.valid(run) || run.phase != "claimed" {
				return ErrRunAuthorityConflict
			}
			previous, err := parseUTC(*payload.PreviousLeaseExpiresAt)
			if err != nil || !previous.Equal(run.prepareLeaseExpiresAt) {
				return ErrRunAuthorityConflict
			}
			next, err := parseUTC(*payload.PrepareLeaseExpiresAt)
			if err != nil || !next.After(previous) || !event.EmittedAt.Before(previous) {
				return ErrRunAuthorityConflict
			}
			run.prepareLeaseExpiresAt = next
			run.lastEventID = event.ID
			run.streamSequence = event.Seq
		case "RunStarted":
			var payload generationPayload
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				!payload.valid(run) || run.phase != "claimed" ||
				!event.EmittedAt.Before(run.prepareLeaseExpiresAt) {
				return ErrRunAuthorityConflict
			}
			statusReference := payload.runtimeStatusReferenceFields.reference()
			runtime, exists := state.runtimes[run.runtimeInstanceID]
			if !exists || !runtime.validatesStatusReference(statusReference, true) {
				return ErrRunAuthorityConflict
			}
			run.phase = "running"
			run.lastEventID = event.ID
			run.streamSequence = event.Seq
			workItem := state.workItems[run.workItemID]
			workItem.status = "running"
			state.workItems[workItem.id] = workItem
		case "RunTerminalCommitted":
			var payload terminalPayload
			if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
				!payload.valid(run) ||
				(run.phase != "claimed" && run.phase != "running") {
				return ErrRunAuthorityConflict
			}
			statusReference := payload.runtimeStatusReferenceFields.reference()
			runtime, exists := state.runtimes[run.runtimeInstanceID]
			if !exists || !runtime.validatesStatusReference(statusReference, false) {
				return ErrRunAuthorityConflict
			}
			run.phase = "terminal"
			run.terminalStatus = *payload.Status
			run.terminalReason = *payload.Reason
			run.lastEventID = event.ID
			run.streamSequence = event.Seq
			terminals[event.ID] = &terminalReplay{
				binding: capacityBinding{
					workItemID: run.workItemID, runID: run.id,
					claimID: run.claimID, claimGeneration: run.claimGeneration,
					runtimeInstanceID: run.runtimeInstanceID,
					agentInstanceID:   run.agentInstanceID,
				},
				statusReference: statusReference,
				eventID:         event.ID, status: run.terminalStatus,
			}
		default:
			return ErrRunAuthorityConflict
		}
	}
	state.runs[run.id] = run
	return nil
}

func replayRuntimeCapacityStream(
	state *authorityState,
	streamID string,
	events []journal.Event,
	claims map[string]*claimReplay,
	terminals map[string]*terminalReplay,
	allowOrphanCapacity bool,
) error {
	runtimeID := strings.TrimPrefix(streamID, "runtime_capacity:")
	runtime, ok := state.runtimes[runtimeID]
	if !ok {
		return ErrRunAuthorityConflict
	}
	for _, event := range events {
		if event.Type != "RuntimeCapacityReserved" &&
			event.Type != "RuntimeCapacityReleased" {
			return ErrRunAuthorityConflict
		}
		var payload capacityEventPayload
		if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
			payload.RuntimeInstanceID != runtimeID ||
			!runtime.validatesStatusReference(
				payload.statusReference(),
				event.Type == "RuntimeCapacityReserved",
			) {
			return ErrRunAuthorityConflict
		}
		key := capacityKey(payload.RunID, payload.ClaimGeneration)
		switch event.Type {
		case "RuntimeCapacityReserved":
			claim, ok := claims[event.CausationID]
			if runtime.active[key].runID != "" {
				return ErrRunAuthorityConflict
			}
			if ok &&
				(!equalCapacityBinding(claim.binding, payload.binding()) ||
					claim.statusReference != payload.statusReference()) {
				return ErrRunAuthorityConflict
			}
			if !ok && !allowOrphanCapacity {
				return ErrRunAuthorityConflict
			}
			runtime.active[key] = payload.binding()
			if ok {
				claim.reserved = true
			}
			statusFact := runtime.statusFacts[payload.RuntimeStatusSequence]
			if statusFact.reference != payload.statusReference() ||
				statusFact.capacity <= 0 ||
				len(runtime.active) > statusFact.capacity {
				return ErrRunAuthorityConflict
			}
		case "RuntimeCapacityReleased":
			active, exists := runtime.active[key]
			if !exists || !equalCapacityBinding(active, payload.binding()) {
				return ErrRunAuthorityConflict
			}
			delete(runtime.active, key)
			if terminal, ok := terminals[event.CausationID]; ok {
				if !equalCapacityBinding(terminal.binding, payload.binding()) ||
					terminal.statusReference != payload.statusReference() {
					return ErrRunAuthorityConflict
				}
				terminal.released = true
			} else if claim, ok := claims[event.CausationID]; ok && claim.needsOldRelease {
				if !equalCapacityBinding(claim.previousBinding, payload.binding()) ||
					claim.statusReference != payload.statusReference() {
					return ErrRunAuthorityConflict
				}
				claim.oldReleased = true
			} else if !allowOrphanCapacity {
				return ErrRunAuthorityConflict
			}
		}
	}
	state.runtimes[runtimeID] = runtime
	return nil
}

func validateOutcome(event journal.Event, terminal *terminalReplay) error {
	var payload struct {
		WorkItemID      *string `json:"work_item_id"`
		RunID           *string `json:"run_id"`
		ClaimGeneration *int64  `json:"claim_generation"`
		Status          *string `json:"status"`
	}
	if err := decodeExactPayload(event.PayloadJSON, &payload); err != nil ||
		payload.WorkItemID == nil || payload.RunID == nil ||
		payload.ClaimGeneration == nil || payload.Status == nil ||
		*payload.WorkItemID != terminal.binding.workItemID ||
		*payload.RunID != terminal.binding.runID ||
		*payload.ClaimGeneration != terminal.binding.claimGeneration {
		return ErrRunAuthorityConflict
	}
	if terminal.status == "succeeded" {
		if event.Type != "WorkItemReadyForReview" || *payload.Status != "ready_for_review" {
			return ErrRunAuthorityConflict
		}
	} else if event.Type != "WorkItemTerminal" || *payload.Status != terminal.status {
		return ErrRunAuthorityConflict
	}
	return nil
}

type runtimeDiscoveryPayload struct {
	DiscoveryDigest *string                   `json:"discovery_digest"`
	SourceProbeID   *string                   `json:"source_probe_id"`
	Instance        *runtimeDiscoveryInstance `json:"instance"`
	ModelIDs        json.RawMessage           `json:"model_ids"`
}

type runtimeDiscoveryInstance struct {
	ID                   *string         `json:"id"`
	DeviceID             *string         `json:"device_id"`
	AdapterType          *string         `json:"adapter_type"`
	DisplayName          *string         `json:"display_name"`
	ExecutableVersion    *string         `json:"executable_version"`
	Status               *string         `json:"status"`
	ObservedCapabilities json.RawMessage `json:"observed_capabilities"`
	Capacity             *int            `json:"capacity"`
}

type runtimeStatusPayload struct {
	ReconciliationDigest  *string `json:"reconciliation_digest"`
	BaselineDigest        *string `json:"baseline_digest"`
	SourceDiscoveryDigest *string `json:"source_discovery_digest"`
	SourceProbeID         *string `json:"source_probe_id"`
	RuntimeInstanceID     *string `json:"runtime_instance_id"`
	DeviceID              *string `json:"device_id"`
	AdapterType           *string `json:"adapter_type"`
	FromStatus            *string `json:"from_status"`
	ToStatus              *string `json:"to_status"`
	PreviousEventID       *string `json:"previous_event_id"`
	PreviousSequence      *int64  `json:"previous_sequence"`
}

type runClaimedPayload struct {
	WorkItemID                    *string                 `json:"work_item_id"`
	RunID                         *string                 `json:"run_id"`
	ClaimID                       *string                 `json:"claim_id"`
	ClaimGeneration               *int64                  `json:"claim_generation"`
	RuntimeInstanceID             *string                 `json:"runtime_instance_id"`
	AgentInstanceID               *string                 `json:"agent_instance_id"`
	PrepareLeaseExpiresAt         *string                 `json:"prepare_lease_expires_at"`
	AssetRevisionBindings         *[]AssetRevisionBinding `json:"asset_revision_bindings"`
	AssetRevisionSetDigest        *string                 `json:"asset_revision_set_digest"`
	MaterializationManifestDigest *string                 `json:"materialization_manifest_digest"`
	MaterializationRootDigest     *string                 `json:"materialization_root_digest"`
	runtimeStatusReferenceFields
}

type generationPayload struct {
	WorkItemID        *string `json:"work_item_id"`
	RunID             *string `json:"run_id"`
	ClaimID           *string `json:"claim_id"`
	ClaimGeneration   *int64  `json:"claim_generation"`
	RuntimeInstanceID *string `json:"runtime_instance_id"`
	AgentInstanceID   *string `json:"agent_instance_id"`
	runtimeStatusReferenceFields
}

type leaseExtendedPayload struct {
	WorkItemID             *string `json:"work_item_id"`
	RunID                  *string `json:"run_id"`
	ClaimID                *string `json:"claim_id"`
	ClaimGeneration        *int64  `json:"claim_generation"`
	RuntimeInstanceID      *string `json:"runtime_instance_id"`
	AgentInstanceID        *string `json:"agent_instance_id"`
	PreviousLeaseExpiresAt *string `json:"previous_lease_expires_at"`
	PrepareLeaseExpiresAt  *string `json:"prepare_lease_expires_at"`
}

type terminalPayload struct {
	WorkItemID        *string `json:"work_item_id"`
	RunID             *string `json:"run_id"`
	ClaimID           *string `json:"claim_id"`
	ClaimGeneration   *int64  `json:"claim_generation"`
	RuntimeInstanceID *string `json:"runtime_instance_id"`
	AgentInstanceID   *string `json:"agent_instance_id"`
	Status            *string `json:"status"`
	Reason            *string `json:"reason"`
	runtimeStatusReferenceFields
}

type runtimeStatusReferenceFields struct {
	RuntimeStatusStreamID *string `json:"runtime_status_stream_id"`
	RuntimeStatusSequence *int64  `json:"runtime_status_sequence"`
	RuntimeStatusEventID  *string `json:"runtime_status_event_id"`
}

type runtimeStatusReferencePayload struct {
	RuntimeStatusStreamID string `json:"runtime_status_stream_id"`
	RuntimeStatusSequence int64  `json:"runtime_status_sequence"`
	RuntimeStatusEventID  string `json:"runtime_status_event_id"`
}

type capacityEventPayload struct {
	WorkItemID        string `json:"work_item_id"`
	RunID             string `json:"run_id"`
	ClaimID           string `json:"claim_id"`
	ClaimGeneration   int64  `json:"claim_generation"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	AgentInstanceID   string `json:"agent_instance_id"`
	runtimeStatusReferencePayload
}

func (payload runClaimedPayload) valid(run RunRecord, runID string) bool {
	lineagePresent := payload.AssetRevisionBindings != nil ||
		payload.AssetRevisionSetDigest != nil ||
		payload.MaterializationManifestDigest != nil ||
		payload.MaterializationRootDigest != nil
	if lineagePresent && (payload.AssetRevisionBindings == nil ||
		payload.AssetRevisionSetDigest == nil ||
		payload.MaterializationManifestDigest == nil ||
		payload.MaterializationRootDigest == nil) {
		return false
	}
	return payload.WorkItemID != nil && payload.RunID != nil &&
		payload.ClaimID != nil && payload.ClaimGeneration != nil &&
		payload.RuntimeInstanceID != nil && payload.AgentInstanceID != nil &&
		payload.PrepareLeaseExpiresAt != nil &&
		payload.runtimeStatusReferenceFields.complete() &&
		*payload.WorkItemID == run.workItemID && *payload.RunID == runID &&
		validCanonicalUUID(*payload.ClaimID) &&
		*payload.ClaimGeneration > 0 &&
		validOpaqueID(*payload.RuntimeInstanceID) &&
		*payload.AgentInstanceID == run.agentInstanceID
}

func (payload generationPayload) valid(run RunRecord) bool {
	return validGenerationIdentity(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) && payload.runtimeStatusReferenceFields.complete()
}

func (payload leaseExtendedPayload) valid(run RunRecord) bool {
	return validGenerationIdentity(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) &&
		payload.PreviousLeaseExpiresAt != nil &&
		payload.PrepareLeaseExpiresAt != nil
}

func (payload terminalPayload) valid(run RunRecord) bool {
	if !validGenerationIdentity(
		payload.WorkItemID, payload.RunID, payload.ClaimID,
		payload.ClaimGeneration, payload.RuntimeInstanceID,
		payload.AgentInstanceID, run,
	) || !payload.runtimeStatusReferenceFields.complete() ||
		payload.Status == nil || payload.Reason == nil {
		return false
	}
	return validTerminal(*payload.Status, *payload.Reason)
}

func validGenerationIdentity(
	workItemID *string,
	runID *string,
	claimID *string,
	claimGeneration *int64,
	runtimeInstanceID *string,
	agentInstanceID *string,
	run RunRecord,
) bool {
	return workItemID != nil && runID != nil && claimID != nil &&
		claimGeneration != nil && runtimeInstanceID != nil &&
		agentInstanceID != nil &&
		*workItemID == run.workItemID && *runID == run.id &&
		*claimID == run.claimID &&
		*claimGeneration == run.claimGeneration &&
		*runtimeInstanceID == run.runtimeInstanceID &&
		*agentInstanceID == run.agentInstanceID
}

func (fields runtimeStatusReferenceFields) complete() bool {
	return fields.RuntimeStatusStreamID != nil &&
		fields.RuntimeStatusSequence != nil &&
		fields.RuntimeStatusEventID != nil
}

func (fields runtimeStatusReferenceFields) reference() runtimeStatusReference {
	if !fields.complete() {
		return runtimeStatusReference{}
	}
	return runtimeStatusReference{
		streamID: *fields.RuntimeStatusStreamID,
		sequence: *fields.RuntimeStatusSequence,
		eventID:  *fields.RuntimeStatusEventID,
	}
}

func (payload capacityEventPayload) statusReference() runtimeStatusReference {
	return runtimeStatusReference{
		streamID: payload.RuntimeStatusStreamID,
		sequence: payload.RuntimeStatusSequence,
		eventID:  payload.RuntimeStatusEventID,
	}
}

func (payload capacityEventPayload) binding() capacityBinding {
	return capacityBinding{
		workItemID: payload.WorkItemID, runID: payload.RunID,
		claimID: payload.ClaimID, claimGeneration: payload.ClaimGeneration,
		runtimeInstanceID: payload.RuntimeInstanceID,
		agentInstanceID:   payload.AgentInstanceID,
	}
}

func generationEventPayload(
	input RunGenerationInput,
	statusReference runtimeStatusReferencePayload,
) any {
	return struct {
		WorkItemID        string `json:"work_item_id"`
		RunID             string `json:"run_id"`
		ClaimID           string `json:"claim_id"`
		ClaimGeneration   int64  `json:"claim_generation"`
		RuntimeInstanceID string `json:"runtime_instance_id"`
		AgentInstanceID   string `json:"agent_instance_id"`
		runtimeStatusReferencePayload
	}{
		input.WorkItemID, input.RunID, input.ClaimID, input.ClaimGeneration,
		input.RuntimeInstanceID, input.AgentInstanceID,
		statusReference,
	}
}

func validateContextAndAssignment(ctx context.Context, input WorkItemAssignmentInput) error {
	if ctx == nil || !validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.Title) || !validOpaqueID(input.RunID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidRunAuthorityInput
	}
	return ctx.Err()
}

func validateContextAndClaim(ctx context.Context, input RunClaimInput) error {
	if ctx == nil || !validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) || !validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validCanonicalUUID(input.CorrelationID) ||
		input.PrepareLeaseDuration <= 0 ||
		input.PrepareLeaseDuration > maxPrepareLease {
		return ErrInvalidRunAuthorityInput
	}
	return ctx.Err()
}

func validateContextAndGeneration(ctx context.Context, input RunGenerationInput) error {
	if ctx == nil || !validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) || !validCanonicalUUID(input.ClaimID) ||
		input.ClaimGeneration <= 0 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidRunAuthorityInput
	}
	return ctx.Err()
}

func validateTerminalInput(ctx context.Context, input RunTerminalInput) error {
	if err := validateContextAndGeneration(ctx, input.RunGenerationInput); err != nil {
		return err
	}
	if !validTerminal(input.Status, input.Reason) {
		return ErrInvalidRunAuthorityInput
	}
	return nil
}

func validTerminal(status, reason string) bool {
	switch status {
	case "succeeded":
		return reason == ""
	case "failed", "cancelled":
		return validOpaqueID(reason)
	default:
		return false
	}
}

func validRuntimeStatus(status string) bool {
	switch status {
	case "online", "offline", "incompatible", "disabled":
		return true
	default:
		return false
	}
}

func validRuntimeDiscoveryInstance(
	instance runtimeDiscoveryInstance,
	runtimeID string,
) bool {
	if instance.ID == nil || instance.DeviceID == nil ||
		instance.AdapterType == nil || instance.DisplayName == nil ||
		instance.ExecutableVersion == nil || instance.Status == nil ||
		instance.ObservedCapabilities == nil || instance.Capacity == nil ||
		*instance.ID != runtimeID || *instance.DeviceID == "" ||
		*instance.AdapterType == "" || *instance.DisplayName == "" ||
		!validRuntimeStatus(*instance.Status) || *instance.Capacity < 0 {
		return false
	}
	var capabilities []string
	return json.Unmarshal(instance.ObservedCapabilities, &capabilities) == nil &&
		validOrderedStrings(capabilities)
}

func validOrderedStrings(values []string) bool {
	for index, value := range values {
		if value == "" || index > 0 && value <= values[index-1] {
			return false
		}
	}
	return true
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') &&
			!(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func claimableRecords(
	state authorityState,
	input RunClaimInput,
	now time.Time,
) (WorkItemRecord, RunRecord, error) {
	workItem, ok := state.workItems[input.WorkItemID]
	if !ok || workItem.runID != input.RunID ||
		workItem.agentInstanceID != input.AgentInstanceID ||
		workItem.status != "assigned" {
		return WorkItemRecord{}, RunRecord{}, ErrRunNotClaimable
	}
	run, ok := state.runs[input.RunID]
	if !ok || run.workItemID != input.WorkItemID ||
		run.agentInstanceID != input.AgentInstanceID {
		return WorkItemRecord{}, RunRecord{}, ErrRunNotClaimable
	}
	switch run.phase {
	case "unclaimed":
		return workItem, run, nil
	case "claimed":
		if now.Before(run.prepareLeaseExpiresAt) {
			return WorkItemRecord{}, RunRecord{}, ErrRunLeaseActive
		}
		return workItem, run, nil
	case "terminal":
		return WorkItemRecord{}, RunRecord{}, ErrRunAlreadyTerminal
	default:
		return WorkItemRecord{}, RunRecord{}, ErrRunNotClaimable
	}
}

func validApprovalResolutionStatus(status string) bool {
	switch status {
	case "assigned", "blocked", "cancelled":
		return true
	default:
		return false
	}
}

func currentGenerationRun(
	state authorityState,
	input RunGenerationInput,
) (RunRecord, error) {
	run, ok := state.runs[input.RunID]
	if !ok || run.workItemID != input.WorkItemID {
		return RunRecord{}, ErrRunNotClaimable
	}
	if input.ClaimGeneration != run.claimGeneration ||
		input.ClaimID != run.claimID {
		return RunRecord{}, ErrStaleClaimGeneration
	}
	if input.RuntimeInstanceID != run.runtimeInstanceID ||
		input.AgentInstanceID != run.agentInstanceID {
		return RunRecord{}, ErrRunAuthorityConflict
	}
	return run, nil
}

func newEvent(
	id, streamID string,
	sequence int64,
	eventType string,
	emittedAt time.Time,
	correlationID, causationID string,
	payload any,
) journal.Event {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return journal.Event{
		ID: id, StreamID: streamID, Seq: sequence, IdempotencyKey: id,
		Type: eventType, SchemaVersion: 1, EmittedAt: emittedAt,
		CorrelationID: correlationID, CausationID: causationID,
		PayloadJSON: body,
	}
}

func deterministicEventID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "evt-" + hex.EncodeToString(digest[:16])
}

func workItemStream(id string) string        { return "work-item/" + id }
func runStream(id string) string             { return "run/" + id }
func runtimeStatusStream(id string) string   { return "runtime_instance:" + id }
func runtimeCapacityStream(id string) string { return "runtime_capacity:" + id }

func isRunAuthorityStream(streamID string) bool {
	return streamID == runIdentityStreamID ||
		strings.HasPrefix(streamID, "work-item/") ||
		strings.HasPrefix(streamID, "run/") ||
		strings.HasPrefix(streamID, "runtime_instance:") ||
		strings.HasPrefix(streamID, "runtime_capacity:")
}

func (authority *Authority) runIdentityReady() bool {
	authority.identityMu.RLock()
	defer authority.identityMu.RUnlock()
	return authority.identityReady
}

func (authority *Authority) setRunIdentityReady() {
	authority.identityMu.Lock()
	defer authority.identityMu.Unlock()
	authority.identityReady = true
}

func runIdentityPayload(reservation runIdentityReservation) struct {
	RunID              string `json:"run_id"`
	WorkItemID         string `json:"work_item_id"`
	AgentInstanceID    string `json:"agent_instance_id"`
	AssignmentStreamID string `json:"assignment_stream_id"`
	AssignmentSequence int64  `json:"assignment_sequence"`
	AssignmentEventID  string `json:"assignment_event_id"`
} {
	return struct {
		RunID              string `json:"run_id"`
		WorkItemID         string `json:"work_item_id"`
		AgentInstanceID    string `json:"agent_instance_id"`
		AssignmentStreamID string `json:"assignment_stream_id"`
		AssignmentSequence int64  `json:"assignment_sequence"`
		AssignmentEventID  string `json:"assignment_event_id"`
	}{
		reservation.runID,
		reservation.workItemID,
		reservation.agentInstanceID,
		reservation.assignmentStreamID,
		reservation.assignmentSequence,
		reservation.assignmentEventID,
	}
}

func runIdentityMatches(
	indexed map[string]runIdentityReservation,
	expected map[string]runIdentityReservation,
) bool {
	if len(indexed) != len(expected) {
		return false
	}
	for runID, reservation := range expected {
		if indexed[runID] != reservation {
			return false
		}
	}
	return true
}

func runCommandStreams(
	workItemID string,
	runID string,
	runtimeInstanceID string,
) []string {
	return []string{
		workItemStream(workItemID),
		runStream(runID),
		runtimeStatusStream(runtimeInstanceID),
		runtimeCapacityStream(runtimeInstanceID),
	}
}

func runCommandExpectations(
	state authorityState,
	input RunGenerationInput,
) []journal.StreamHeadExpectation {
	streams := runCommandStreams(
		input.WorkItemID,
		input.RunID,
		input.RuntimeInstanceID,
	)
	expectations := make([]journal.StreamHeadExpectation, len(streams))
	for index, streamID := range streams {
		expectations[index] = journal.StreamHeadExpectation{
			StreamID: streamID,
			Sequence: state.heads[streamID],
		}
	}
	return expectations
}

func (reference runtimeStatusReference) payload() runtimeStatusReferencePayload {
	return runtimeStatusReferencePayload{
		RuntimeStatusStreamID: reference.streamID,
		RuntimeStatusSequence: reference.sequence,
		RuntimeStatusEventID:  reference.eventID,
	}
}

func (runtime authorityRuntime) currentStatusHead(heads map[string]int64) bool {
	return runtime.statusHead.streamID == runtimeStatusStream(runtime.id) &&
		runtime.statusHead.sequence > 0 &&
		runtime.statusHead.eventID != "" &&
		heads[runtime.statusHead.streamID] == runtime.statusHead.sequence
}

func (runtime authorityRuntime) validatesStatusReference(
	reference runtimeStatusReference,
	requireOnline bool,
) bool {
	if reference.streamID != runtimeStatusStream(runtime.id) ||
		reference.sequence <= 0 ||
		reference.eventID == "" {
		return false
	}
	fact, ok := runtime.statusFacts[reference.sequence]
	if !ok || fact.reference != reference {
		return false
	}
	return !requireOnline || fact.status == "online"
}

func capacityKey(runID string, generation int64) string {
	return runID + "/" + fmt.Sprint(generation)
}

func equalCapacityBinding(left capacityBinding, right capacityBinding) bool {
	return left == right
}

func mapJournalWriteError(err error) error {
	switch {
	case errors.Is(err, journal.ErrStreamHeadConflict),
		errors.Is(err, journal.ErrSequenceConflict),
		errors.Is(err, journal.ErrIdempotencyConflict),
		errors.Is(err, journal.ErrPartialEventBatchConflict):
		return fmt.Errorf("%w: %v", ErrRunAuthorityConflict, err)
	default:
		return err
	}
}

func decodeExactPayload(payload []byte, target any) error {
	if !json.Valid(payload) || hasDuplicateJSONKeys(payload) {
		return ErrRunAuthorityConflict
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrRunAuthorityConflict
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return ErrRunAuthorityConflict
	}
	return nil
}

func hasDuplicateJSONKeys(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	return scanDuplicateValue(decoder)
}

func scanDuplicateValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return true
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return true
			}
			key, ok := keyToken.(string)
			if !ok {
				return true
			}
			if _, duplicate := seen[key]; duplicate {
				return true
			}
			seen[key] = struct{}{}
			if scanDuplicateValue(decoder) {
				return true
			}
		}
		_, err = decoder.Token()
		return err != nil
	case '[':
		for decoder.More() {
			if scanDuplicateValue(decoder) {
				return true
			}
		}
		_, err = decoder.Token()
		return err != nil
	default:
		return true
	}
}

func parseUTC(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, ErrRunAuthorityConflict
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() || parsed.Location() != time.UTC {
		return time.Time{}, ErrRunAuthorityConflict
	}
	return parsed, nil
}

func validOpaqueID(value string) bool {
	if value == "" || len(value) > maxAuthorityIDBytes ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range []byte(value) {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') ||
			(character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func (record WorkItemRecord) public() WorkItemRecord { return record }
func (record RunRecord) public() RunRecord {
	record.AssetRevisionBindings = append([]AssetRevisionBinding(nil), record.AssetRevisionBindings...)
	return record
}
