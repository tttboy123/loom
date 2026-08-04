package api

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/observability"
	_ "modernc.org/sqlite"
)

func TestLocalIntegrationAPIThinBoundary(t *testing.T) {
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
	service, err := app.NewLocalIntegrationService(store, integrationService, observabilityService)
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewLocalIntegrationAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := api.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Releases) != 0 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
