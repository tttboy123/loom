package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/work"
)

func newWorkersFixture(t *testing.T) *LocalWorkersService {
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
	execution, err := work.NewWorkerExecutionService(
		store,
		func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) },
		time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalWorkersService(
		store,
		func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) },
		time.Minute,
		execution,
	)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestWorkersSnapshotAndClaimFlow(t *testing.T) {
	service := newWorkersFixture(t)
	ctx := context.Background()
	jid := "123e4567-e89b-42d3-a456-426614174000"
	claimInput, _ := json.Marshal(map[string]any{
		"worker_id": "w1", "job_id": "job-a",
		"lane": "development", "candidate_branch": "codex/candidate-a",
	})
	result, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID: jid, OperationID: "op-1", Action: "claim", Input: claimInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != "claim" || result.AttemptID == "" || result.Generation != 1 {
		t.Fatalf("claim receipt = %+v", result)
	}
	snapshot, err := service.ReadWorkersSnapshot(ctx, WorkersSnapshotRequest{JourneyID: jid, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Attempts) != 1 || len(snapshot.Active) != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestWorkersCrashThenReclaim(t *testing.T) {
	service := newWorkersFixture(t)
	ctx := context.Background()
	jid := "123e4567-e89b-42d3-a456-426614174000"
	claimInput, _ := json.Marshal(map[string]any{
		"worker_id": "w1", "job_id": "job-a", "lane": "development",
	})
	claimed, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID: jid, OperationID: "op-1", Action: "claim", Input: claimInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	crashInput, _ := json.Marshal(map[string]any{
		"attempt_id": claimed.AttemptID, "generation": 1,
		"crash_seam": "after_cas", "crash_effect_cardinality": "single_effect",
	})
	if _, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID: jid, OperationID: "op-crash", Action: "crash", Input: crashInput,
	}); err != nil {
		t.Fatal(err)
	}
	reclaimInput, _ := json.Marshal(map[string]any{
		"job_id": "job-a", "attempt_id": claimed.AttemptID,
		"generation": 1, "lane": "repair",
	})
	reclaimed, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID: jid, OperationID: "op-reclaim", Action: "reclaim", Input: reclaimInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed.Generation != 2 {
		t.Fatalf("reclaimed generation = %d, want 2", reclaimed.Generation)
	}
}

func TestWorkersTestFailureRoutesToRepair(t *testing.T) {
	service := newWorkersFixture(t)
	ctx := context.Background()
	jid := "123e4567-e89b-42d3-a456-426614174000"
	claimInput, _ := json.Marshal(map[string]any{
		"worker_id": "w1", "job_id": "job-a", "lane": "test",
	})
	claimed, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID: jid, OperationID: "op-1", Action: "claim", Input: claimInput,
	})
	if err != nil {
		t.Fatal(err)
	}
	resultInput, _ := json.Marshal(map[string]any{
		"attempt_id": claimed.AttemptID, "generation": 1,
		"status": "failed", "failure_class": string(schedule.ClassTestDefect),
		"evidence_digests": []string{"abc123"},
	})
	if _, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID: jid, OperationID: "op-result", Action: "result", Input: resultInput,
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.ReadWorkersSnapshot(ctx, WorkersSnapshotRequest{JourneyID: jid, Limit: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Attempts) != 1 || string(snapshot.Attempts[0].Status) != "failed" {
		t.Fatalf("snapshot = %+v", snapshot.Attempts)
	}
}

func TestWorkersReviewerWriteDenied(t *testing.T) {
	service := newWorkersFixture(t)
	ctx := context.Background()
	_, err := service.CommitWorkersCommand(ctx, WorkersCommandRequest{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-review", Action: "review_write_denied",
	})
	if err == nil {
		t.Fatal("reviewer write was not denied")
	}
}
