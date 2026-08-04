package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/observability"
	_ "modernc.org/sqlite"
)

func TestIntegrationSnapshotAndIntegrateFlow(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
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
	service, err := NewLocalIntegrationService(store, integrationService, observabilityService)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	input, _ := json.Marshal(map[string]any{
		"candidate_id": "candidate-1", "target_branch": "main",
		"base_commit": "base", "source_digest": "src", "evidence_digest": "ev",
		"dependency_digests": []string{},
	})
	result, err := service.CommitCommand(ctx, IntegrationCommandRequest{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-1", Action: "integrate", Input: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Disposition != "integrated" || result.ReleaseID == "" {
		t.Fatalf("result = %+v", result)
	}
	snapshot, err := service.ReadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Releases) != 1 {
		t.Fatalf("snapshot = %+v", snapshot.Releases)
	}
}
