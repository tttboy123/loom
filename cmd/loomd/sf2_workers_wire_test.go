package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/work"
)

func TestSF2WorkersWireOverSocket(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-sf2-workers-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	database, err := sql.Open("sqlite", filepath.Join(root, "workers.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	now := func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) }
	execution, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	workersService, err := app.NewLocalWorkersService(store, now, time.Minute, execution)
	if err != nil {
		t.Fatal(err)
	}
	workersAPI, err := api.NewLocalWorkersAPI(workersService)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "sf2-workers-fixture",
		Handler: localipc.HandlerFunc(localProductHandlerWithComposition(
			nil, nil, nil, nil, nil, nil, nil, nil, workersAPI,
		)),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("SF2 workers IPC server did not become ready")
	}
	t.Cleanup(func() {
		cancel()
		<-done
	})
	client, err := localipc.NewClient(localipc.ClientConfig{SocketPath: socketPath, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	claimInput, _ := json.Marshal(map[string]any{
		"worker_id": "w1", "job_id": "job-a", "lane": "development",
		"candidate_branch": "codex/candidate-a",
	})
	var receipt app.WorkersCommandResult
	if err := client.CallJourney(ctx, journeyID, "workers_command", app.WorkersCommandRequest{
		JourneyID: journeyID, OperationID: "sf2-claim-1", Action: "claim", Input: claimInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Generation != 1 || receipt.AttemptID == "" {
		t.Fatalf("claim receipt = %+v", receipt)
	}
	var snapshot app.WorkersSnapshot
	if err := client.CallJourney(ctx, journeyID, "workers_snapshot", app.WorkersSnapshotRequest{
		JourneyID: journeyID, Limit: 64,
	}, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Attempts) != 1 || len(snapshot.Active) != 1 {
		t.Fatalf("workers snapshot = %+v", snapshot)
	}
	crashInput, _ := json.Marshal(map[string]any{
		"attempt_id": receipt.AttemptID, "generation": 1,
		"crash_seam": "after_cas", "crash_effect_cardinality": "single_effect",
	})
	if err := client.CallJourney(ctx, journeyID, "workers_command", app.WorkersCommandRequest{
		JourneyID: journeyID, OperationID: "sf2-crash-1", Action: "crash", Input: crashInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	reclaimInput, _ := json.Marshal(map[string]any{
		"job_id": "job-a", "attempt_id": receipt.AttemptID,
		"generation": 1, "lane": "repair",
	})
	var reclaimed app.WorkersCommandResult
	if err := client.CallJourney(ctx, journeyID, "workers_command", app.WorkersCommandRequest{
		JourneyID: journeyID, OperationID: "sf2-reclaim-1", Action: "reclaim", Input: reclaimInput,
	}, &reclaimed); err != nil {
		t.Fatal(err)
	}
	if reclaimed.Generation != 2 {
		t.Fatalf("reclaimed generation = %d, want 2", reclaimed.Generation)
	}
	var remote *localipc.RemoteError
	staleInput, _ := json.Marshal(map[string]any{
		"attempt_id": "attempt-old", "generation": 1,
	})
	if err := client.CallJourney(ctx, journeyID, "workers_command", app.WorkersCommandRequest{
		JourneyID: journeyID, OperationID: "sf2-stale-1", Action: "reject_stale", Input: staleInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	reviewInput, _ := json.Marshal(map[string]any{})
	if err := client.CallJourney(ctx, journeyID, "workers_command", app.WorkersCommandRequest{
		JourneyID: journeyID, OperationID: "sf2-review-1", Action: "review_write_denied", Input: reviewInput,
	}, &receipt); !errors.As(err, &remote) || remote.Code != "denied" {
		t.Fatalf("reviewer write error = %v, want denied code", err)
	}
}
