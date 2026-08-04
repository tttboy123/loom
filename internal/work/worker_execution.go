package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/schedule"
)

var (
	ErrInvalidWorkerRequest = errors.New("invalid worker request")
	ErrReviewerCannotWrite  = errors.New("reviewer may not write product state")
)

// WorkerExecutionService is the authoritative writer for worker/attempt
// facts. Every fact is an Event Journal record appended via
// AppendBatchIfStreamHeads CAS; the service never bypasses the Journal.
type WorkerExecutionService struct {
	store *journal.Store
	now   func() time.Time
	ttl   time.Duration
}

func NewWorkerExecutionService(
	store *journal.Store,
	now func() time.Time,
	ttl time.Duration,
) (*WorkerExecutionService, error) {
	if store == nil || now == nil || ttl <= 0 {
		return nil, ErrInvalidWorkerRequest
	}
	return &WorkerExecutionService{store: store, now: now, ttl: ttl}, nil
}

// ClaimInput carries the worker's claim for one Job/attempt.
type ClaimInput struct {
	JourneyID         string        `json:"journey_id"`
	OperationID       string        `json:"operation_id"`
	WorkerID          string        `json:"worker_id"`
	JobID             string        `json:"job_id"`
	Lane              schedule.Lane `json:"lane"`
	CandidateBranch   string        `json:"candidate_branch"`
	CandidateWorktree string        `json:"candidate_worktree"`
}

// AttemptClaimResult is the authoritative receipt.
type AttemptClaimResult struct {
	OperationID  string
	AttemptID    string
	JobID        string
	Generation   int64
	LeaseExpires string
	Lane         string
	EventIDs     []string
}

// Claim writes AttemptClaimed for a fresh generation.
func (service *WorkerExecutionService) Claim(ctx context.Context, input ClaimInput) (AttemptClaimResult, error) {
	if err := validateClaimInput(input); err != nil {
		return AttemptClaimResult{}, err
	}
	now := service.now().UTC()
	attemptID := workerAttemptID(input.WorkerID, input.JobID, 1)
	attempt := schedule.Attempt{
		AttemptID: attemptID, JobID: input.JobID, Generation: 1,
		LeaseExpiresAt: now.Add(service.ttl).Format(time.RFC3339Nano),
		ClaimCAS:       input.OperationID, Status: schedule.StatusClaimed,
		Lane: input.Lane, CandidateBranch: input.CandidateBranch,
		CandidateWorktree: input.CandidateWorktree,
		StartedAt:         now.Format(time.RFC3339Nano),
	}
	stream := "attempt/" + attemptID
	events, err := service.buildAttemptEvents(
		"AttemptClaimed", stream, input.OperationID, now, input.JourneyID, attempt,
	)
	if err != nil {
		return AttemptClaimResult{}, err
	}
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return AttemptClaimResult{}, err
	}
	return AttemptClaimResult{
		OperationID: input.OperationID, AttemptID: attemptID, JobID: input.JobID,
		Generation: 1, LeaseExpires: attempt.LeaseExpiresAt,
		Lane: string(input.Lane), EventIDs: eventIDsFrom(committed),
	}, nil
}

// RecordResultInput records a successful or failed attempt result.
type RecordResultInput struct {
	JourneyID       string                 `json:"journey_id"`
	OperationID     string                 `json:"operation_id"`
	AttemptID       string                 `json:"attempt_id"`
	Generation      int64                  `json:"generation"`
	Status          schedule.AttemptStatus `json:"status"`
	FailureClass    schedule.FailureClass  `json:"failure_class"`
	EvidenceDigests []string               `json:"evidence_digests"`
	CandidateReady  bool                   `json:"candidate_ready"`
	ReviewVerdict   string                 `json:"review_verdict"`
}

func (service *WorkerExecutionService) RecordResult(ctx context.Context, input RecordResultInput) ([]string, error) {
	if input.AttemptID == "" || input.OperationID == "" {
		return nil, ErrInvalidWorkerRequest
	}
	if input.Status != schedule.StatusSucceeded && input.Status != schedule.StatusFailed {
		return nil, ErrInvalidWorkerRequest
	}
	now := service.now().UTC()
	stream := "attempt/" + input.AttemptID
	payload := map[string]any{
		"attempt_id":       input.AttemptID,
		"generation":       input.Generation,
		"status":           string(input.Status),
		"failure_class":    string(input.FailureClass),
		"evidence_digests": schedule.NormalizeStrings(input.EvidenceDigests),
		"finished_at":      now.Format(time.RFC3339Nano),
	}
	events, err := service.buildAttemptEvents(
		"AttemptResultRecorded", stream, input.OperationID, now, input.JourneyID, payload,
	)
	if err != nil {
		return nil, err
	}
	if input.CandidateReady {
		readyEvents, err := service.buildAttemptEvents(
			"CandidateReadyForReview", stream, input.OperationID, now, input.JourneyID,
			map[string]any{"attempt_id": input.AttemptID, "candidate_ready": true, "lane": "review"},
		)
		if err != nil {
			return nil, err
		}
		events = append(events, readyEvents...)
	}
	if input.ReviewVerdict != "" {
		verdictEvents, err := service.buildAttemptEvents(
			"ReviewVerdictRecorded", stream, input.OperationID, now, input.JourneyID,
			map[string]any{"attempt_id": input.AttemptID, "review_verdict": input.ReviewVerdict},
		)
		if err != nil {
			return nil, err
		}
		events = append(events, verdictEvents...)
	}
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return nil, err
	}
	return eventIDsFrom(committed), nil
}

// RecordCrash writes AttemptCrashed with the frozen crash seam and effect
// cardinality, truthfully.
func (service *WorkerExecutionService) RecordCrash(
	ctx context.Context,
	journeyID, operationID, attemptID string,
	generation int64,
	seam schedule.CrashSeam,
	effectCardinality string,
) ([]string, error) {
	if attemptID == "" || operationID == "" {
		return nil, ErrInvalidWorkerRequest
	}
	now := service.now().UTC()
	stream := "attempt/" + attemptID
	events, err := service.buildAttemptEvents(
		"AttemptCrashed", stream, operationID, now, journeyID,
		map[string]any{
			"attempt_id":               attemptID,
			"generation":               generation,
			"status":                   string(schedule.StatusCrashed),
			"crash_seam":               string(seam),
			"crash_effect_cardinality": effectCardinality,
		},
	)
	if err != nil {
		return nil, err
	}
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return nil, err
	}
	return eventIDsFrom(committed), nil
}

// Reclaim writes LeaseReclaimed + GenerationAdvanced + AttemptClaimed for
// exactly one new generation of a crashed/expired attempt.
func (service *WorkerExecutionService) Reclaim(
	ctx context.Context,
	journeyID, operationID, jobID string,
	oldAttemptID string,
	oldGeneration int64,
	lane schedule.Lane,
) (AttemptClaimResult, error) {
	if jobID == "" || oldAttemptID == "" || operationID == "" {
		return AttemptClaimResult{}, ErrInvalidWorkerRequest
	}
	now := service.now().UTC()
	newGeneration := oldGeneration + 1
	attemptID := workerAttemptID("reconciler", jobID, newGeneration)
	stream := "attempt/" + attemptID
	reclaimEvents, err := service.buildAttemptEvents(
		"LeaseReclaimed", stream, operationID, now, journeyID,
		map[string]any{
			"attempt_id": oldAttemptID, "job_id": jobID,
			"old_generation": oldGeneration, "new_generation": newGeneration,
		},
	)
	if err != nil {
		return AttemptClaimResult{}, err
	}
	advancedEvents, err := service.buildAttemptEvents(
		"GenerationAdvanced", stream, operationID, now, journeyID,
		map[string]any{"attempt_id": oldAttemptID, "generation": newGeneration},
	)
	if err != nil {
		return AttemptClaimResult{}, err
	}
	events := append(reclaimEvents, advancedEvents...)
	attempt := schedule.Attempt{
		AttemptID: attemptID, JobID: jobID, Generation: newGeneration,
		LeaseExpiresAt: now.Add(service.ttl).Format(time.RFC3339Nano),
		ClaimCAS:       operationID, Status: schedule.StatusClaimed, Lane: lane,
		StartedAt: now.Format(time.RFC3339Nano),
	}
	claimedEvents, err := service.buildAttemptEvents(
		"AttemptClaimed", stream, operationID, now, journeyID, attempt,
	)
	if err != nil {
		return AttemptClaimResult{}, err
	}
	events = append(events, claimedEvents...)
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return AttemptClaimResult{}, err
	}
	return AttemptClaimResult{
		OperationID: operationID, AttemptID: attemptID, JobID: jobID,
		Generation: newGeneration, LeaseExpires: attempt.LeaseExpiresAt,
		Lane: string(lane), EventIDs: eventIDsFrom(committed),
	}, nil
}

// RejectStale records StaleResultRejected with zero side effects for a late
// or stale-generation result.
func (service *WorkerExecutionService) RejectStale(
	ctx context.Context,
	journeyID, operationID, attemptID string,
	generation int64,
) ([]string, error) {
	if attemptID == "" || operationID == "" {
		return nil, ErrInvalidWorkerRequest
	}
	now := service.now().UTC()
	stream := "attempt/" + attemptID
	events, err := service.buildAttemptEvents(
		"StaleResultRejected", stream, operationID, now, journeyID,
		map[string]any{
			"attempt_id": attemptID, "generation": generation,
			"status": string(schedule.StatusStaleRejected),
		},
	)
	if err != nil {
		return nil, err
	}
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return nil, err
	}
	return eventIDsFrom(committed), nil
}

// ReviewerWrite is denied: the Reviewer lane is read-only and may never write
// product/attempt state.
func (service *WorkerExecutionService) ReviewerWrite(_ context.Context) error {
	return ErrReviewerCannotWrite
}

func validateClaimInput(input ClaimInput) error {
	if input.WorkerID == "" || input.JobID == "" || input.OperationID == "" {
		return ErrInvalidWorkerRequest
	}
	switch input.Lane {
	case schedule.LaneDevelopment, schedule.LaneRepair, schedule.LaneTest:
	default:
		return ErrInvalidWorkerRequest
	}
	return nil
}

func workerAttemptID(workerID, jobID string, generation int64) string {
	sum := sha256.Sum256([]byte("sf2\n" + workerID + "\n" + jobID + "\n" + fmt.Sprint(generation)))
	return "attempt-" + hex.EncodeToString(sum[:])[:24]
}

func (service *WorkerExecutionService) buildAttemptEvents(
	eventType, stream, operationID string,
	now time.Time, journeyID string, payload any,
) ([]journal.Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidWorkerRequest
	}
	seed := fmt.Sprintf("sf2\n%s\n%s\n%s\n%d", eventType, stream, operationID, 0)
	sum := sha256.Sum256([]byte(seed))
	eventID := "sf2-" + hex.EncodeToString(sum[:])[:32]
	key := fmt.Sprintf("sf2/%s/%s/%d", eventType, operationID, 0)
	return []journal.Event{{
		ID: eventID, StreamID: stream, Seq: 0, IdempotencyKey: key,
		Type: eventType, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: journeyID, PayloadJSON: data,
	}}, nil
}

func (service *WorkerExecutionService) append(
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
		return nil, fmt.Errorf("%w: journal CAS conflict", ErrInvalidWorkerRequest)
	}
	return committed, err
}

func eventIDsFrom(events []journal.Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
