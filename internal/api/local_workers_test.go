package api

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

func TestLocalWorkersAPIRejectsNilService(t *testing.T) {
	if _, err := NewLocalWorkersAPI(nil); err == nil {
		t.Fatal("nil service accepted")
	}
}

func TestLocalWorkersAPIThinBoundary(t *testing.T) {
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
	execution, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalWorkersService(store, now, time.Minute, execution)
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewLocalWorkersAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := api.WorkersSnapshot(context.Background(), app.WorkersSnapshotRequest{
		JourneyID: "123e4567-e89b-42d3-a456-426614174000", Limit: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Attempts) != 0 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
