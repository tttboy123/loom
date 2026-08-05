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
	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/observability"
	_ "modernc.org/sqlite"
)

func TestSF3IntegrationWireOverSocket(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-sf3-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	database, err := sql.Open("sqlite", filepath.Join(root, "integration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(database)
	now := func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) }
	integrationService, err := integration.NewIntegrationService(store, now)
	if err != nil {
		t.Fatal(err)
	}
	observabilityService, err := observability.NewObservabilityService(store, now)
	if err != nil {
		t.Fatal(err)
	}
	localIntegrationService, err := app.NewLocalIntegrationService(store, integrationService, observabilityService)
	if err != nil {
		t.Fatal(err)
	}
	integrationAPI, err := api.NewLocalIntegrationAPI(localIntegrationService)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(), BuildID: "sf3-integration-fixture",
		Handler: localipc.HandlerFunc(localProductHandlerWithComposition(
			nil, nil, nil, nil, nil, nil, nil, nil, nil, integrationAPI, nil, nil, nil,
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
		t.Fatal("SF3 integration IPC server did not become ready")
	}
	t.Cleanup(func() { cancel(); <-done })
	client, err := localipc.NewClient(localipc.ClientConfig{SocketPath: socketPath, Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	journeyID := "123e4567-e89b-42d3-a456-426614174000"
	integrateInput, _ := json.Marshal(map[string]any{
		"candidate_id": "candidate-1", "target_branch": "main",
		"base_commit": "base", "source_digest": "src", "evidence_digest": "ev",
		"dependency_digests": []string{},
	})
	var receipt app.IntegrationCommandResult
	if err := client.CallJourney(ctx, journeyID, "integration_command", app.IntegrationCommandRequest{
		JourneyID: journeyID, OperationID: "sf3-integrate-1", Action: "integrate", Input: integrateInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Disposition != "integrated" || receipt.ReleaseID == "" {
		t.Fatalf("integrate receipt = %+v", receipt)
	}
	// Competing stale integration must lose.
	var remote *localipc.RemoteError
	if err := client.CallJourney(ctx, journeyID, "integration_command", app.IntegrationCommandRequest{
		JourneyID: journeyID, OperationID: "sf3-integrate-2", Action: "integrate", Input: integrateInput,
	}, &receipt); !errors.As(err, &remote) || remote.Code != "conflict" {
		t.Fatalf("competing integration error = %v, want conflict", err)
	}
	canaryInput, _ := json.Marshal(map[string]any{
		"run_id": "run-1", "runtime_instance_id": "runtime.pi.earendil-works.0.82.1",
		"model_id": "loom-local/qwen2.5", "skill_digest": "skill",
	})
	if err := client.CallJourney(ctx, journeyID, "integration_command", app.IntegrationCommandRequest{
		JourneyID: journeyID, OperationID: "sf3-canary-1", Action: "start_canary", Input: canaryInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	frameInput, _ := json.Marshal(map[string]any{
		"attempt_id": "attempt-1", "generation": 2, "node_id": "n1",
		"kind": "human_required", "content": "needs approval", "authorized": true,
	})
	if err := client.CallJourney(ctx, journeyID, "integration_command", app.IntegrationCommandRequest{
		JourneyID: journeyID, OperationID: "sf3-frame-1", Action: "publish_frame", Input: frameInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	var snapshot app.IntegrationSnapshot
	if err := client.CallJourney(ctx, journeyID, "integration_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Releases) != 1 || len(snapshot.Canaries) != 2 ||
		len(snapshot.Timeline) != 1 || len(snapshot.Attention) != 1 {
		t.Fatalf("snapshot = releases %d canaries %d timeline %d attention %d",
			len(snapshot.Releases), len(snapshot.Canaries), len(snapshot.Timeline), len(snapshot.Attention))
	}
	adoptInput, _ := json.Marshal(map[string]any{
		"release_id": receipt.ReleaseID, "run_id": "run-later",
	})
	if err := client.CallJourney(ctx, journeyID, "integration_command", app.IntegrationCommandRequest{
		JourneyID: journeyID, OperationID: "sf3-adopt-1", Action: "adopt", Input: adoptInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
	rollbackInput, _ := json.Marshal(map[string]any{
		"release_id": receipt.ReleaseID, "rollback_to_id": "release-prior",
	})
	if err := client.CallJourney(ctx, journeyID, "integration_command", app.IntegrationCommandRequest{
		JourneyID: journeyID, OperationID: "sf3-rollback-1", Action: "rollback", Input: rollbackInput,
	}, &receipt); err != nil {
		t.Fatal(err)
	}
}
