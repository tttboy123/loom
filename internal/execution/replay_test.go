package execution

import (
	"bytes"
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

func TestReplayRejectsInvalidRecoveryRequiredTransitions(t *testing.T) {
	now := execTime().UTC()
	executionID := "exec-recovery-invalid"
	stream := executionStreamID(execTestJobA, executionID)
	proposed := journal.Event{
		ID: "p-recovery", StreamID: stream, Seq: 1,
		IdempotencyKey: "recovery-proposed", Type: EventToolProposed,
		SchemaVersion: 1, EmittedAt: now,
		PayloadJSON: mustJSON(proposedPayload{
			JobID: execTestJobA, ExecutionID: executionID, CallDigest: "abc",
			Tool: "Bash", Command: "printf private", ProposedAt: now.Format(time.RFC3339Nano),
			Generation: 1, OperationID: "op-recovery", JourneyID: execTestCorrelation,
		}),
	}
	allowed := journal.Event{
		ID: "a-recovery", StreamID: stream, Seq: 2,
		IdempotencyKey: "recovery-allowed", Type: EventToolAllowed,
		SchemaVersion: 1, EmittedAt: now,
		PayloadJSON: mustJSON(allowedPayload{
			ExecutionID: executionID, AllowedAt: now.Format(time.RFC3339Nano),
		}),
	}
	recovery := func(id string, seq int64, code, action string) journal.Event {
		return journal.Event{
			ID: id, StreamID: stream, Seq: seq,
			IdempotencyKey: id + "-key", Type: EventToolRecoveryRequired,
			SchemaVersion: 2, EmittedAt: now,
			PayloadJSON: mustJSON(recoveryRequiredPayloadV2{
				ExecutionID: executionID, RecoveryCode: code,
				RecoveryAction: action, RequiredAt: now.Format(time.RFC3339Nano),
			}),
		}
	}

	tests := []struct {
		name   string
		events []journal.Event
	}{
		{
			name: "without allow",
			events: []journal.Event{
				proposed,
				recovery("r-without-allow", 2, "side_effect_unknown", "resolve_tool_recovery"),
			},
		},
		{
			name: "substituted code",
			events: []journal.Event{
				proposed, allowed,
				recovery("r-bad-code", 3, "retry_now", "resolve_tool_recovery"),
			},
		},
		{
			name: "substituted action",
			events: []journal.Event{
				proposed, allowed,
				recovery("r-bad-action", 3, "side_effect_unknown", "retry_tool"),
			},
		},
		{
			name: "duplicate recovery terminal",
			events: []journal.Event{
				proposed, allowed,
				recovery("r-first", 3, "side_effect_unknown", "resolve_tool_recovery"),
				recovery("r-second", 4, "side_effect_unknown", "resolve_tool_recovery"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ReplaySnapshot(test.events); !errors.Is(err, ErrInvalidExecutionEvent) {
				t.Fatalf("ReplaySnapshot() error = %v, want invalid event", err)
			}
		})
	}
}

func TestRecoveryRequiredPayloadIsContentFree(t *testing.T) {
	payload := mustJSON(recoveryRequiredPayloadV2{
		ExecutionID:  "exec-recovery-private",
		RecoveryCode: "side_effect_unknown", RecoveryAction: "resolve_tool_recovery",
		RequiredAt: execTime().Format(time.RFC3339Nano),
	})
	for _, forbidden := range [][]byte{
		[]byte(`"command"`), []byte(`"path"`), []byte("printf private"),
	} {
		if bytes.Contains(payload, forbidden) {
			t.Fatalf("recovery payload contains forbidden content %q: %s", forbidden, payload)
		}
	}
}
