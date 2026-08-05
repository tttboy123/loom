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
	"loom-pi-rebuild/internal/queue"

	_ "modernc.org/sqlite"
)

func TestSF1QueueWireOverSocket(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-sf1-queue-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	database, err := sql.Open("sqlite", filepath.Join(root, "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	queueService, err := app.NewLocalQueueService(
		store,
		func() time.Time { return time.Date(2026, 8, 4, 21, 0, 0, 0, time.UTC) },
		func() string { return "queue-view-v1" },
	)
	if err != nil {
		t.Fatal(err)
	}
	queueAPI, err := api.NewLocalQueueAPI(queueService)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "sf1-queue-fixture",
		Handler: localipc.HandlerFunc(localProductHandlerWithComposition(
			nil, nil, nil, nil, nil, nil, nil, queueAPI, nil, nil, nil, nil, nil, nil, nil,
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
		t.Fatal("SF1 queue IPC server did not become ready")
	}
	client, err := localipc.NewClient(localipc.ClientConfig{SocketPath: socketPath, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	submission := queue.JobSubmission{
		JobID: "job-a", Source: "user_queued", DAGNodeID: "node-a",
		OwnedPaths:     []string{"internal/queue/model.go"},
		ResourceClaims: queue.ResourceClaims{Runtime: "pi", Slots: 1, Model: "m"},
		MaxAttempts:    3, CapabilityKind: "feature",
		ExitConditions:       []string{"focused tests pass"},
		VerificationStrategy: "focused + race",
		IntegrationStrategy:  "single-integrator",
		EligibilityAuthority: "user",
	}
	input, err := json.Marshal(submission)
	if err != nil {
		t.Fatal(err)
	}
	var receipt api.QueueCommandResult
	if err := client.CallJourney(ctx, journeyID, "queue_command", api.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "sf1-create-1", Action: "create_job", Input: input,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.JobID != "job-a" || len(receipt.EventIDs) != 2 {
		t.Fatalf("create receipt = %+v", receipt)
	}
	var snapshot api.QueueSnapshot
	if err := client.CallJourney(ctx, journeyID, "queue_snapshot", api.QueueSnapshotRequest{
		JourneyID: journeyID, Limit: 64,
	}, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].Status != queue.StatusAdmitted {
		t.Fatalf("queue snapshot = %+v", snapshot.Jobs)
	}
	duplicate := submission
	duplicate.JobID = "job-b"
	duplicateInput, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	var remote *localipc.RemoteError
	if err := client.CallJourney(ctx, journeyID, "queue_command", api.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "sf1-create-2", Action: "create_job", Input: duplicateInput,
	}, &receipt); !errors.As(err, &remote) || remote.Code != "conflict" {
		t.Fatalf("duplicate DAG node error = %v, want conflict code", err)
	}
	gap := queue.GapProposalSubmission{
		SourceType: "run_failure", SourceIDs: []string{"run-1"},
		SourceDigests: []string{"abc123"}, AffectedCapability: "scheduling",
		ObservedBehavior: "queue stalls", ExpectedBehavior: "queue drains",
		UserImpact: "delayed", Confidence: "high", Reproducibility: "always",
		PrivacyClass: "none", ProposedScope: "SF-W1 admission",
		OwnedPathClaims: []string{"internal/queue/admission.go"},
		RiskClass:       "low", Disposition: "propose_successor",
	}
	gapInput, err := json.Marshal(gap)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CallJourney(ctx, journeyID, "queue_command", api.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "sf1-gap-1", Action: "gap_observe", Input: gapInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	firstGapID := receipt.GapID
	duplicateGap := gap
	duplicateGap.SourceIDs = []string{"run-2"}
	duplicateGapInput, err := json.Marshal(duplicateGap)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CallJourney(ctx, journeyID, "queue_command", api.QueueCommandRequest{
		JourneyID: journeyID, OperationID: "sf1-gap-2", Action: "gap_observe", Input: duplicateGapInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.GapID != firstGapID || receipt.Disposition != "merge_duplicate" {
		t.Fatalf("duplicate gap = %+v, want converge on %s", receipt, firstGapID)
	}
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("server stop error = %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}
