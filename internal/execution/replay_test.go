package execution

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

func execTime() time.Time {
	return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
}

func mustJSON(value any) json.RawMessage {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return body
}

func TestReplaySnapshotRoundTrip(t *testing.T) {
	store := openExecStore(t)
	now := execTime().UTC()
	executionID := "exec-1"
	stream := executionStreamID(execTestJobA, executionID)
	events := []journal.Event{
		{
			ID: "p1", StreamID: stream, Seq: 1,
			IdempotencyKey: "k1", Type: EventToolProposed,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(proposedPayload{
				JobID: execTestJobA, ExecutionID: executionID, CallDigest: "abc",
				Tool: "Bash", Command: "ls", ProposedAt: now.Format(time.RFC3339Nano),
				Generation: 1, OperationID: "op", JourneyID: execTestCorrelation,
			}),
		},
		{
			ID: "a1", StreamID: stream, Seq: 2,
			IdempotencyKey: "k2", Type: EventToolAllowed,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(allowedPayload{ExecutionID: executionID, AllowedAt: now.Format(time.RFC3339Nano)}),
		},
		{
			ID: "c1", StreamID: stream, Seq: 3,
			IdempotencyKey: "k3", Type: EventToolCompleted,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(completedPayload{
				ExecutionID: executionID, ExitCode: 0, OutputDigest: "out",
				ChangedFilesDigest: emptyChangedDigest, DurationMS: 1,
				EvidenceID: "ev", CompletedAt: now.Format(time.RFC3339Nano),
			}),
		},
	}
	if _, err := store.AppendBatch(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	all, _ := store.ReadAll(context.Background())
	snapshot, err := ReplaySnapshot(all)
	if err != nil {
		t.Fatalf("ReplaySnapshot() error = %v", err)
	}
	record, ok := snapshot.Record(executionID)
	if !ok {
		t.Fatal("record missing")
	}
	if record.Status != "completed" || record.ExitCode != 0 {
		t.Fatalf("record = %+v", record)
	}
}

func TestReplayRejectsAllowedWithoutProposed(t *testing.T) {
	now := execTime().UTC()
	executionID := "exec-orphan"
	stream := executionStreamID(execTestJobA, executionID)
	events := []journal.Event{
		{
			ID: "a1", StreamID: stream, Seq: 1,
			IdempotencyKey: "k1", Type: EventToolAllowed,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(allowedPayload{ExecutionID: executionID, AllowedAt: now.Format(time.RFC3339Nano)}),
		},
	}
	if _, err := ReplaySnapshot(events); !errors.Is(err, ErrInvalidExecutionEvent) {
		t.Fatalf("ReplaySnapshot() error = %v, want invalid event", err)
	}
}

func TestReplayRejectsCompletedWithoutAllowed(t *testing.T) {
	now := execTime().UTC()
	executionID := "exec-noconsent"
	stream := executionStreamID(execTestJobA, executionID)
	events := []journal.Event{
		{
			ID: "p1", StreamID: stream, Seq: 1,
			IdempotencyKey: "k1", Type: EventToolProposed,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(proposedPayload{
				JobID: execTestJobA, ExecutionID: executionID, CallDigest: "abc",
				Tool: "Bash", Command: "ls", ProposedAt: now.Format(time.RFC3339Nano),
				Generation: 1, OperationID: "op", JourneyID: execTestCorrelation,
			}),
		},
		{
			ID: "c1", StreamID: stream, Seq: 2,
			IdempotencyKey: "k2", Type: EventToolCompleted,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(completedPayload{
				ExecutionID: executionID, ExitCode: 0, OutputDigest: "out",
				ChangedFilesDigest: emptyChangedDigest, DurationMS: 1,
				EvidenceID: "ev", CompletedAt: now.Format(time.RFC3339Nano),
			}),
		},
	}
	if _, err := ReplaySnapshot(events); !errors.Is(err, ErrInvalidExecutionEvent) {
		t.Fatalf("ReplaySnapshot() error = %v, want invalid event", err)
	}
}

func TestReplayRejectsDuplicateTerminal(t *testing.T) {
	now := execTime().UTC()
	executionID := "exec-dup"
	stream := executionStreamID(execTestJobA, executionID)
	events := []journal.Event{
		{
			ID: "p1", StreamID: stream, Seq: 1,
			IdempotencyKey: "k1", Type: EventToolProposed,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(proposedPayload{
				JobID: execTestJobA, ExecutionID: executionID, CallDigest: "abc",
				Tool: "Bash", Command: "ls", ProposedAt: now.Format(time.RFC3339Nano),
				Generation: 1, OperationID: "op", JourneyID: execTestCorrelation,
			}),
		},
		{
			ID: "a1", StreamID: stream, Seq: 2,
			IdempotencyKey: "k2", Type: EventToolAllowed,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(allowedPayload{ExecutionID: executionID, AllowedAt: now.Format(time.RFC3339Nano)}),
		},
		{
			ID: "c1", StreamID: stream, Seq: 3,
			IdempotencyKey: "k3", Type: EventToolCompleted,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(completedPayload{
				ExecutionID: executionID, ExitCode: 0, OutputDigest: "out",
				ChangedFilesDigest: emptyChangedDigest, DurationMS: 1,
				EvidenceID: "ev", CompletedAt: now.Format(time.RFC3339Nano),
			}),
		},
		{
			ID: "c2", StreamID: stream, Seq: 4,
			IdempotencyKey: "k4", Type: EventToolCompleted,
			SchemaVersion: 1, EmittedAt: now,
			PayloadJSON: mustJSON(completedPayload{
				ExecutionID: executionID, ExitCode: 0, OutputDigest: "out2",
				ChangedFilesDigest: emptyChangedDigest, DurationMS: 1,
				EvidenceID: "ev2", CompletedAt: now.Format(time.RFC3339Nano),
			}),
		},
	}
	if _, err := ReplaySnapshot(events); !errors.Is(err, ErrInvalidExecutionEvent) {
		t.Fatalf("ReplaySnapshot() error = %v, want invalid event", err)
	}
}
