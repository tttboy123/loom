package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/queue"

	_ "modernc.org/sqlite"
)

func TestQueueCreateJobAppendsAndRebuilds(t *testing.T) {
	service := newQueueService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	input, err := json.Marshal(validQueueSubmission("job-a", "node-a"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-create-1", Action: "create_job", Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.JobID != "job-a" || result.Status != "admitted" {
		t.Fatalf("result = %+v", result)
	}
	snapshot, err := service.ReadQueueSnapshot(context.Background(), app.QueueSnapshotRequest{JourneyID: journeyID})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(snapshot.Jobs))
	}
	job := snapshot.Jobs[0]
	if job.JobID != "job-a" || job.Status != queue.StatusAdmitted {
		t.Fatalf("projected job = %+v", job)
	}
	if len(job.ExitConditions) != 1 || job.VerificationStrategy == "" || job.IntegrationStrategy == "" {
		t.Fatalf("admission-compiled fields missing: %+v", job)
	}
	rebuilt, err := service.ReadQueueSnapshot(context.Background(), app.QueueSnapshotRequest{JourneyID: journeyID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rebuilt.Jobs) != 1 || rebuilt.Jobs[0].Status != queue.StatusAdmitted {
		t.Fatalf("journal rebuild mismatch: %+v", rebuilt.Jobs)
	}
}

func TestQueueConcurrentCreateSingleCASWinner(t *testing.T) {
	service := newQueueService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	var winners int
	var errorsSeen []error
	var mutex sync.Mutex
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(sequence int) {
			defer wait.Done()
			input, _ := json.Marshal(validQueueSubmission("job-"+string(rune('a'+sequence)), "same-node"))
			_, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
				JourneyID:   journeyID,
				OperationID: "op-concurrent-" + string(rune('a'+sequence)),
				Action:      "create_job", Input: input,
			})
			mutex.Lock()
			defer mutex.Unlock()
			if err == nil {
				winners++
			} else {
				errorsSeen = append(errorsSeen, err)
			}
		}(index)
	}
	wait.Wait()
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
	if len(errorsSeen) != 1 || !errors.Is(errorsSeen[0], queue.ErrDuplicateWork) {
		t.Fatalf("losers = %v, want one duplicate-work rejection", errorsSeen)
	}
}

func TestQueueRejectsProtectedAuthorityClaim(t *testing.T) {
	service := newQueueService(t)
	submission := validQueueSubmission("job-p", "node-p")
	submission.ProtectedAuthorityPaths = []string{"internal/journal/store.go"}
	input, _ := json.Marshal(submission)
	_, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-protected-1", Action: "create_job", Input: input,
	})
	if !errors.Is(err, queue.ErrDenied) {
		t.Fatalf("protected claim err = %v, want ErrDenied", err)
	}
}

func TestQueueGapObserveConvergesOnOneGapID(t *testing.T) {
	service := newQueueService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	submission := queue.GapProposalSubmission{
		SourceType: "run_failure", SourceIDs: []string{"run-1"},
		SourceDigests: []string{"abc123"}, AffectedCapability: "scheduling",
		ObservedBehavior: "queue stalls", ExpectedBehavior: "queue drains",
		UserImpact: "delayed", Confidence: "high", Reproducibility: "always",
		PrivacyClass: "none", ProposedScope: "SF-W1 admission",
		OwnedPathClaims: []string{"internal/queue/admission.go"},
		RiskClass:       "low", Disposition: "propose_successor",
	}
	input, _ := json.Marshal(submission)
	first, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-gap-1", Action: "gap_observe", Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	secondSubmission := submission
	secondSubmission.SourceIDs = []string{"run-2"}
	secondInput, _ := json.Marshal(secondSubmission)
	second, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-gap-2", Action: "gap_observe", Input: secondInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.GapID != first.GapID {
		t.Fatalf("duplicate converged on %s, want %s", second.GapID, first.GapID)
	}
	if second.Disposition != "merge_duplicate" {
		t.Fatalf("duplicate disposition = %q, want merge_duplicate", second.Disposition)
	}
	snapshot, err := service.ReadQueueSnapshot(context.Background(), app.QueueSnapshotRequest{JourneyID: journeyID})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Gaps) != 1 {
		t.Fatalf("gaps = %d, want 1 (no WorkItem side effect from duplicate)", len(snapshot.Gaps))
	}
}

func TestQueueSuccessorCompileRejectsStaleEvidence(t *testing.T) {
	service := newQueueService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	gap := queue.GapProposalSubmission{
		SourceType: "run_failure", SourceIDs: []string{"run-1"},
		SourceDigests: []string{"authoritative"}, AffectedCapability: "scheduling",
		ObservedBehavior: "queue stalls", ExpectedBehavior: "queue drains",
		UserImpact: "delayed", Confidence: "high", Reproducibility: "always",
		PrivacyClass: "none", ProposedScope: "SF-W1 admission",
		OwnedPathClaims: []string{"internal/queue/admission.go"},
		RiskClass:       "low", Disposition: "propose_successor",
	}
	gapInput, _ := json.Marshal(gap)
	observed, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-gap-1", Action: "gap_observe", Input: gapInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	staleSubmission := validQueueSubmission("job-s", "node-s")
	staleSubmission.Source = "gap_proposal"
	successor := queue.SuccessorCompileRequest{
		GapID: observed.GapID, ExpectedSourceDigests: []string{"stale"},
		Submission: staleSubmission,
	}
	successorInput, _ := json.Marshal(successor)
	_, err = service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-spr-1", Action: "successor_compile", Input: successorInput,
	})
	if !errors.Is(err, queue.ErrDenied) {
		t.Fatalf("stale successor err = %v, want ErrDenied", err)
	}
	authorizedSubmission := validQueueSubmission("job-s", "node-s")
	authorizedSubmission.Source = "gap_proposal"
	authorized := queue.SuccessorCompileRequest{
		GapID: observed.GapID, ExpectedSourceDigests: []string{"authoritative"},
		Submission: authorizedSubmission,
	}
	authorizedInput, _ := json.Marshal(authorized)
	result, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-spr-2", Action: "successor_compile", Input: authorizedInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SuccessorProposalID == "" {
		t.Fatalf("successor proposal missing: %+v", result)
	}
	snapshot, err := service.ReadQueueSnapshot(context.Background(), app.QueueSnapshotRequest{JourneyID: journeyID})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Jobs) != 0 {
		t.Fatalf("successor compilation created a WorkItem side effect: %d jobs", len(snapshot.Jobs))
	}
}

func TestQueueCancelJobTerminal(t *testing.T) {
	service := newQueueService(t)
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	input, _ := json.Marshal(validQueueSubmission("job-c", "node-c"))
	if _, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-create-1", Action: "create_job", Input: input,
	}); err != nil {
		t.Fatal(err)
	}
	cancelInput, _ := json.Marshal(struct {
		JobID string `json:"job_id"`
	}{JobID: "job-c"})
	if _, err := service.CommitQueueCommand(context.Background(), app.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "op-cancel-1", Action: "cancel_job", Input: cancelInput,
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.ReadQueueSnapshot(context.Background(), app.QueueSnapshotRequest{JourneyID: journeyID})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Jobs[0].Status != queue.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", snapshot.Jobs[0].Status)
	}
}

func newQueueService(t *testing.T) *app.LocalQueueService {
	t.Helper()
	root := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(root, "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	service, err := app.NewLocalQueueService(
		store,
		func() time.Time { return time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC) },
		func() string { return "queue-view-v1" },
	)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func validQueueSubmission(jobID, node string) queue.JobSubmission {
	return queue.JobSubmission{
		JobID: jobID, Source: "user_queued", DAGNodeID: node,
		OwnedPaths:     []string{"internal/queue/model.go"},
		ResourceClaims: queue.ResourceClaims{Runtime: "pi", Slots: 1, Model: "m"},
		MaxAttempts:    3, CapabilityKind: "feature",
		ExitConditions:       []string{"focused tests pass"},
		VerificationStrategy: "focused + race",
		IntegrationStrategy:  "single-integrator",
		EligibilityAuthority: "user",
	}
}
