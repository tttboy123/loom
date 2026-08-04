package observability

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	_ "modernc.org/sqlite"
)

func newObsFixture(t *testing.T) (*ObservabilityService, *journal.Store) {
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
	service, err := NewObservabilityService(
		store,
		func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	return service, store
}

func TestUnauthorizedFrameRejected(t *testing.T) {
	service, _ := newObsFixture(t)
	_, err := service.PublishFrame(context.Background(), FrameInput{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-1", AttemptID: "attempt-1",
		Generation: 1, NodeID: "n1", Kind: "output", Content: "x", Authorized: false,
	})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized frame = %v", err)
	}
}

func TestStaleFrameRejected(t *testing.T) {
	service, _ := newObsFixture(t)
	_, err := service.PublishFrame(context.Background(), FrameInput{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-1", AttemptID: "attempt-1",
		Generation: 0, NodeID: "n1", Kind: "output", Content: "x", Authorized: true,
	})
	if !errors.Is(err, ErrStaleFrame) {
		t.Fatalf("stale frame = %v", err)
	}
}

func TestMalformedFrameRejected(t *testing.T) {
	service, _ := newObsFixture(t)
	_, err := service.PublishFrame(context.Background(), FrameInput{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-1", AttemptID: "attempt-1",
		Generation: 1, NodeID: "n1", Kind: "output", Content: "", Authorized: true,
	})
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("malformed frame = %v", err)
	}
}

func TestAuthorizedFramePublishedAndProjected(t *testing.T) {
	service, store := newObsFixture(t)
	ids, err := service.PublishFrame(context.Background(), FrameInput{
		JourneyID:   "123e4567-e89b-42d3-a456-426614174000",
		OperationID: "op-1", AttemptID: "attempt-1",
		Generation: 2, NodeID: "n1", Kind: "human_required",
		Content: "needs approval", Authorized: true,
	})
	if err != nil || len(ids) == 0 {
		t.Fatalf("publish = %v %v", ids, err)
	}
	timeline, err := BuildTimeline(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline.Frames) != 1 {
		t.Fatalf("timeline = %+v", timeline.Frames)
	}
	attention, err := BuildAttention(context.Background(), store)
	if err != nil {
		t.Fatal(err)
	}
	if len(attention) != 1 || attention[0].Lane != "human" {
		t.Fatalf("attention = %+v", attention)
	}
}
