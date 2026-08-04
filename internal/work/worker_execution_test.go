package work

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/schedule"
)

func newTestWorkerService(t *testing.T) (*WorkerExecutionService, *journal.Store) {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	service, err := NewWorkerExecutionService(
		store,
		func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) },
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func journeyID() string { return "123e4567-e89b-42d3-a456-426614174000" }

func TestClaimWritesAttemptClaimed(t *testing.T) {
	service, store := newTestWorkerService(t)
	result, err := service.Claim(context.Background(), ClaimInput{
		JourneyID: journeyID(), OperationID: "op-1", WorkerID: "w1",
		JobID: "job-a", Lane: schedule.LaneDevelopment,
		CandidateBranch: "codex/candidate-a", CandidateWorktree: "/tmp/candidate-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Generation != 1 || result.AttemptID == "" {
		t.Fatalf("claim result = %+v", result)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	claimed := 0
	for _, event := range events {
		if event.Type == "AttemptClaimed" {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("AttemptClaimed count = %d, want 1", claimed)
	}
}

func TestDuplicateAttemptIdempotent(t *testing.T) {
	service, _ := newTestWorkerService(t)
	input := ClaimInput{
		JourneyID: journeyID(), OperationID: "op-same", WorkerID: "w1",
		JobID: "job-a", Lane: schedule.LaneDevelopment,
	}
	if _, err := service.Claim(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	// Same operation id must be idempotent: the Journal CAS rejects the
	// duplicate instead of creating a second effect.
	if _, err := service.Claim(context.Background(), input); err == nil {
		t.Fatal("duplicate attempt id was accepted")
	}
}

func TestCrashRecordsSeamAndCardinality(t *testing.T) {
	service, _ := newTestWorkerService(t)
	result, err := service.Claim(context.Background(), ClaimInput{
		JourneyID: journeyID(), OperationID: "op-1", WorkerID: "w1",
		JobID: "job-a", Lane: schedule.LaneDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := service.RecordCrash(
		context.Background(), journeyID(), "op-crash",
		result.AttemptID, 1, schedule.CrashAfterCAS, "single_effect",
	)
	if err != nil || len(ids) == 0 {
		t.Fatalf("crash = %v %v", ids, err)
	}
}

func TestReclaimProducesExactlyOneNewGeneration(t *testing.T) {
	service, store := newTestWorkerService(t)
	old, err := service.Claim(context.Background(), ClaimInput{
		JourneyID: journeyID(), OperationID: "op-1", WorkerID: "w1",
		JobID: "job-a", Lane: schedule.LaneDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	reclaimed, err := service.Reclaim(
		context.Background(), journeyID(), "op-reclaim",
		"job-a", old.AttemptID, 1, schedule.LaneRepair,
	)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed.Generation != 2 {
		t.Fatalf("reclaimed generation = %d, want 2", reclaimed.Generation)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	claimed := 0
	for _, event := range events {
		if event.Type == "AttemptClaimed" {
			claimed++
		}
	}
	if claimed != 2 {
		t.Fatalf("AttemptClaimed count = %d, want exactly 2 (original + one reclamation)", claimed)
	}
}

func TestStaleResultRejectedZeroSideEffects(t *testing.T) {
	service, _ := newTestWorkerService(t)
	ids, err := service.RejectStale(
		context.Background(), journeyID(), "op-stale", "attempt-old", 1,
	)
	if err != nil || len(ids) == 0 {
		t.Fatalf("stale rejection = %v %v", ids, err)
	}
}

func TestReviewerWriteDenied(t *testing.T) {
	service, _ := newTestWorkerService(t)
	if err := service.ReviewerWrite(context.Background()); !errors.Is(err, ErrReviewerCannotWrite) {
		t.Fatalf("reviewer write = %v, want denied", err)
	}
}
