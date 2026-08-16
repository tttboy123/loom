package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/queue"
)

var ErrInvalidQueueRequest = errors.New("invalid local queue request")

type QueueSnapshotRequest struct {
	JourneyID string `json:"-"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"cursor"`
}

type QueueSnapshot struct {
	ViewVersion string                    `json:"view_version"`
	NextCursor  string                    `json:"next_cursor"`
	Jobs        []queue.QueueJob          `json:"jobs"`
	Gaps        []queue.GapProposal       `json:"gaps"`
	Successors  []queue.SuccessorProposal `json:"successors"`
}

type QueueCommandRequest struct {
	JourneyID           string               `json:"-"`
	OperationID         string               `json:"operation_id"`
	Action              string               `json:"action"`
	ExpectedStreamHeads []journal.StreamHead `json:"expected_stream_heads"`
	Input               json.RawMessage      `json:"input"`
}

type QueueCommandResult struct {
	OperationID         string   `json:"operation_id"`
	Action              string   `json:"action"`
	ViewVersion         string   `json:"view_version"`
	EventIDs            []string `json:"event_ids"`
	JobID               string   `json:"job_id,omitempty"`
	GapID               string   `json:"gap_id,omitempty"`
	SuccessorProposalID string   `json:"successor_proposal_id,omitempty"`
	Disposition         string   `json:"disposition,omitempty"`
	Status              string   `json:"status,omitempty"`
	ConflictReason      string   `json:"conflict_reason,omitempty"`
}

type LocalQueueService struct {
	store       *journal.Store
	now         func() time.Time
	viewVersion func() string
	arbiter     *queue.ConflictArbiter
	admissionMu sync.Mutex
}

func NewLocalQueueService(
	store *journal.Store,
	now func() time.Time,
	viewVersion func() string,
) (*LocalQueueService, error) {
	if store == nil || now == nil || viewVersion == nil {
		return nil, ErrInvalidQueueRequest
	}
	return &LocalQueueService{
		store: store, now: now, viewVersion: viewVersion,
		arbiter: queue.NewConflictArbiter(),
	}, nil
}

func (service *LocalQueueService) ReadQueueSnapshot(
	ctx context.Context,
	request QueueSnapshotRequest,
) (QueueSnapshot, error) {
	if service == nil || service.store == nil {
		return QueueSnapshot{}, ErrInvalidQueueRequest
	}
	limit := request.Limit
	if limit <= 0 || limit > 256 {
		limit = 64
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return QueueSnapshot{}, err
	}
	projection, err := queue.Replay(events)
	if err != nil {
		return QueueSnapshot{}, err
	}
	jobs := make([]queue.QueueJob, 0, len(projection.Jobs))
	for _, job := range projection.Jobs {
		if request.Cursor != "" && job.JobID <= request.Cursor {
			continue
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(left, right int) bool {
		if jobs[left].CreatedAt != jobs[right].CreatedAt {
			return jobs[left].CreatedAt < jobs[right].CreatedAt
		}
		return jobs[left].JobID < jobs[right].JobID
	})
	nextCursor := ""
	if len(jobs) > limit {
		jobs = jobs[:limit]
		nextCursor = jobs[len(jobs)-1].JobID
	}
	gaps := make([]queue.GapProposal, 0, len(projection.Gaps))
	for _, gap := range projection.Gaps {
		gaps = append(gaps, gap)
	}
	sort.Slice(gaps, func(left, right int) bool {
		return gaps[left].GapID < gaps[right].GapID
	})
	successors := make([]queue.SuccessorProposal, 0, len(projection.Successors))
	for _, successor := range projection.Successors {
		successors = append(successors, successor)
	}
	sort.Slice(successors, func(left, right int) bool {
		return successors[left].SuccessorProposalID < successors[right].SuccessorProposalID
	})
	return QueueSnapshot{
		ViewVersion: service.viewVersion(),
		NextCursor:  nextCursor,
		Jobs:        jobs,
		Gaps:        gaps,
		Successors:  successors,
	}, nil
}

func (service *LocalQueueService) CommitQueueCommand(
	ctx context.Context,
	request QueueCommandRequest,
) (QueueCommandResult, error) {
	if service == nil || service.store == nil {
		return QueueCommandResult{}, ErrInvalidQueueRequest
	}
	if !validQueueOperationID(request.OperationID) {
		return QueueCommandResult{}, ErrInvalidQueueRequest
	}
	switch request.Action {
	case "create_job":
		return service.createJob(ctx, request)
	case "cancel_job":
		return service.cancelJob(ctx, request)
	case "gap_observe":
		return service.gapObserve(ctx, request)
	case "successor_compile":
		return service.successorCompile(ctx, request)
	default:
		return QueueCommandResult{}, fmt.Errorf("%w: unknown action %q", ErrInvalidQueueRequest, request.Action)
	}
}

func (service *LocalQueueService) createJob(
	ctx context.Context,
	request QueueCommandRequest,
) (QueueCommandResult, error) {
	var submission queue.JobSubmission
	if err := json.Unmarshal(request.Input, &submission); err != nil {
		return QueueCommandResult{}, fmt.Errorf("%w: %v", ErrInvalidQueueRequest, err)
	}
	compiled, err := queue.CompileSubmission(submission)
	if err != nil {
		return QueueCommandResult{}, err
	}
	service.admissionMu.Lock()
	defer service.admissionMu.Unlock()
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return QueueCommandResult{}, err
	}
	projection, err := queue.Replay(events)
	if err != nil {
		return QueueCommandResult{}, err
	}
	active := make([]queue.QueueJob, 0, len(projection.Jobs))
	for _, job := range projection.Jobs {
		active = append(active, job)
	}
	if err := queue.ValidateDAG(active, compiled); err != nil {
		return QueueCommandResult{}, err
	}
	now := service.now().UTC()
	stream := "queue-job/" + compiled.JobID
	lane := queue.LaneDevelopment
	if compiled.Source == "repair" {
		lane = queue.LaneRepair
	}
	created := queue.QueueJob{
		JobID: compiled.JobID, Source: compiled.Source,
		DAGNodeID: compiled.DAGNodeID, Dependencies: compiled.Dependencies,
		Status: queue.StatusQueued, Lane: queue.LaneAdmission,
		OwnedPaths: compiled.OwnedPaths, MutexKeys: compiled.MutexKeys,
		ResourceClaims: compiled.ResourceClaims,
		AttemptCount:   0, MaxAttempts: compiled.MaxAttempts,
		CapabilityKind:          compiled.CapabilityKind,
		ExitConditions:          compiled.ExitConditions,
		VerificationStrategy:    compiled.VerificationStrategy,
		IntegrationStrategy:     compiled.IntegrationStrategy,
		ProtectedAuthorityPaths: compiled.ProtectedAuthorityPaths,
		CreatedAt:               now.Format(time.RFC3339Nano),
		CorrelationID:           request.JourneyID,
	}
	batch, err := service.buildQueueEvents(
		"QueueJobCreated", stream, request.OperationID, now, request.JourneyID, created,
	)
	if err != nil {
		return QueueCommandResult{}, err
	}
	transition := queueJobTransition{JobID: compiled.JobID, Status: queue.StatusAdmitted, Lane: lane}
	admitted, err := service.buildQueueEvents(
		"QueueJobAdmitted", stream, request.OperationID, now, request.JourneyID, transition,
	)
	if err != nil {
		return QueueCommandResult{}, err
	}
	batch = append(batch, admitted...)
	committed, err := service.append(ctx, []string{stream}, batch)
	if err != nil {
		return QueueCommandResult{}, err
	}
	return QueueCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), EventIDs: eventIDsFrom(committed),
		JobID: compiled.JobID, Status: string(queue.StatusAdmitted),
	}, nil
}

func (service *LocalQueueService) cancelJob(
	ctx context.Context,
	request QueueCommandRequest,
) (QueueCommandResult, error) {
	var input struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(request.Input, &input); err != nil || input.JobID == "" {
		return QueueCommandResult{}, fmt.Errorf("%w: job_id required", ErrInvalidQueueRequest)
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return QueueCommandResult{}, err
	}
	projection, err := queue.Replay(events)
	if err != nil {
		return QueueCommandResult{}, err
	}
	job, ok := projection.Jobs[input.JobID]
	if !ok {
		return QueueCommandResult{}, fmt.Errorf("%w: unknown job", ErrInvalidQueueRequest)
	}
	if terminalQueueStatus(job.Status) {
		return QueueCommandResult{}, fmt.Errorf("%w: job %q already terminal", queue.ErrDenied, input.JobID)
	}
	now := service.now().UTC()
	transition := queueJobTransition{JobID: input.JobID, Status: queue.StatusCancelled, Lane: queue.LaneHuman}
	batch, err := service.buildQueueEvents(
		"QueueJobCancelled", "queue-job/"+input.JobID, request.OperationID,
		now, request.JourneyID, transition,
	)
	if err != nil {
		return QueueCommandResult{}, err
	}
	committed, err := service.append(ctx, []string{"queue-job/" + input.JobID}, batch)
	if err != nil {
		return QueueCommandResult{}, err
	}
	return QueueCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), EventIDs: eventIDsFrom(committed),
		JobID: input.JobID, Status: string(queue.StatusCancelled),
	}, nil
}

func (service *LocalQueueService) gapObserve(
	ctx context.Context,
	request QueueCommandRequest,
) (QueueCommandResult, error) {
	var input queue.GapProposalSubmission
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return QueueCommandResult{}, fmt.Errorf("%w: %v", ErrInvalidQueueRequest, err)
	}
	proposal, err := queue.CompileGap(input)
	if err != nil {
		return QueueCommandResult{}, err
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return QueueCommandResult{}, err
	}
	projection, err := queue.Replay(events)
	if err != nil {
		return QueueCommandResult{}, err
	}
	if existing, ok := queue.FindDuplicateGap(projection, proposal); ok {
		return QueueCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			ViewVersion: service.viewVersion(),
			EventIDs:    []string{},
			GapID:       existing.GapID, Disposition: "merge_duplicate",
		}, nil
	}
	now := service.now().UTC()
	proposal.CreatedAt = now.Format(time.RFC3339Nano)
	proposal.CorrelationID = request.JourneyID
	batch, err := service.buildQueueEvents(
		"GapProposalCreated", "queue-gap/"+proposal.GapID, request.OperationID,
		now, request.JourneyID, proposal,
	)
	if err != nil {
		return QueueCommandResult{}, err
	}
	committed, err := service.append(ctx, []string{"queue-gap/" + proposal.GapID}, batch)
	if err != nil {
		return QueueCommandResult{}, err
	}
	return QueueCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), EventIDs: eventIDsFrom(committed),
		GapID: proposal.GapID, Disposition: proposal.Disposition,
	}, nil
}

func (service *LocalQueueService) successorCompile(
	ctx context.Context,
	request QueueCommandRequest,
) (QueueCommandResult, error) {
	var input queue.SuccessorCompileRequest
	if err := json.Unmarshal(request.Input, &input); err != nil {
		return QueueCommandResult{}, fmt.Errorf("%w: %v", ErrInvalidQueueRequest, err)
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return QueueCommandResult{}, err
	}
	projection, err := queue.Replay(events)
	if err != nil {
		return QueueCommandResult{}, err
	}
	gap, ok := projection.Gaps[input.GapID]
	if !ok {
		return QueueCommandResult{}, fmt.Errorf("%w: unknown gap", ErrInvalidQueueRequest)
	}
	successor, err := queue.CompileSuccessor(gap, input)
	if err != nil {
		return QueueCommandResult{}, err
	}
	now := service.now().UTC()
	successor.CreatedAt = now.Format(time.RFC3339Nano)
	successor.CorrelationID = request.JourneyID
	batch, err := service.buildQueueEvents(
		"SuccessorProposalCreated", "queue-gap/"+gap.GapID, request.OperationID,
		now, request.JourneyID, successor,
	)
	if err != nil {
		return QueueCommandResult{}, err
	}
	committed, err := service.append(ctx, []string{"queue-gap/" + gap.GapID}, batch)
	if err != nil {
		return QueueCommandResult{}, err
	}
	return QueueCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: service.viewVersion(), EventIDs: eventIDsFrom(committed),
		GapID: gap.GapID, SuccessorProposalID: successor.SuccessorProposalID,
		Disposition: successor.Disposition,
	}, nil
}

type queueJobTransition struct {
	JobID  string          `json:"job_id"`
	Status queue.JobStatus `json:"status"`
	Lane   queue.Lane      `json:"lane"`
}

func (service *LocalQueueService) buildQueueEvents(
	eventType, stream, operationID string,
	now time.Time, journeyID string, payload any,
) ([]journal.Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidQueueRequest
	}
	seed := fmt.Sprintf("sf1\n%s\n%s\n%s\n%d", eventType, stream, operationID, 0)
	sum := sha256.Sum256([]byte(seed))
	eventID := "sf1-" + hex.EncodeToString(sum[:])[:32]
	key := fmt.Sprintf("sf1/%s/%s/%d", eventType, operationID, 0)
	return []journal.Event{{
		ID: eventID, StreamID: stream, Seq: 0, IdempotencyKey: key,
		Type: eventType, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: journeyID, PayloadJSON: data,
	}}, nil
}

func (service *LocalQueueService) append(
	ctx context.Context,
	streams []string,
	events []journal.Event,
) ([]journal.Event, error) {
	snapshot, err := service.store.ReadStreamSet(ctx, streams)
	if err != nil {
		return nil, err
	}
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	heads := make(map[string]int64, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
		heads[head.StreamID] = head.Sequence
	}
	counts := make(map[string]int, len(streams))
	for index := range events {
		stream := events[index].StreamID
		events[index].Seq = heads[stream] + int64(counts[stream]) + 1
		counts[stream]++
	}
	committed, err := service.store.AppendBatchIfStreamHeads(ctx, expectations, events)
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return nil, fmt.Errorf("%w: journal CAS conflict", queue.ErrDenied)
	}
	return committed, err
}

func validQueueOperationID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func terminalQueueStatus(status queue.JobStatus) bool {
	switch status {
	case queue.StatusIntegrated, queue.StatusRejected,
		queue.StatusCancelled, queue.StatusHumanRequired:
		return true
	default:
		return false
	}
}

func eventIDsFrom(events []journal.Event) []string {
	ids := make([]string, len(events))
	for index, event := range events {
		ids[index] = event.ID
	}
	return ids
}
