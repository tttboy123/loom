package projection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"

	_ "modernc.org/sqlite"
)

func TestRebuildFromCommittedJournalIsDeterministic(t *testing.T) {
	ctx := context.Background()
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)

	events := []journal.Event{
		projectionEvent("evt-mode", "mode-stream", 1, "idem-mode", "ModeSelected", map[string]string{
			"mode": "agent",
		}),
		projectionEvent("evt-work-create", "work-stream", 1, "idem-work-create", "WorkItemCreated", map[string]string{
			"work_item_id": "work-1",
			"title":        "Implement projection",
			"status":       "open",
		}),
		projectionEvent("evt-irrelevant", "other-stream", 1, "idem-irrelevant", "IrrelevantFact", map[string]string{
			"value": "ignored",
		}),
		projectionEvent("evt-evidence", "work-stream", 2, "idem-evidence", "EvidenceSubmitted", map[string]string{
			"evidence_id":  "evidence-1",
			"work_item_id": "work-1",
			"digest":       digestA,
		}),
		projectionEvent("evt-work-terminal", "work-stream", 3, "idem-work-terminal", "WorkItemTerminal", map[string]string{
			"work_item_id": "work-1",
			"status":       "accepted",
		}),
	}
	for _, event := range events {
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}

	first := New(db)
	if err := first.Rebuild(ctx); err != nil {
		t.Fatalf("first Rebuild() error = %v", err)
	}
	second := New(db)
	if err := second.Rebuild(ctx); err != nil {
		t.Fatalf("second Rebuild() error = %v", err)
	}

	want := Snapshot{
		Modes: map[string]string{"mode-stream": "agent"},
		WorkItems: map[string]WorkItem{
			"work-1": {ID: "work-1", Title: "Implement projection", Status: "accepted"},
		},
		Evidence: map[string]Evidence{
			"evidence-1": {ID: "evidence-1", WorkItemID: "work-1", Digest: digestA},
		},
	}
	if got := first.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("first Snapshot() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatalf("recreated projection snapshot mismatch: first=%#v second=%#v", first.Snapshot(), second.Snapshot())
	}
}

func TestRebuildCanonicalizesOutOfOrderAndDuplicateEvents(t *testing.T) {
	ctx := context.Background()
	mode := projectionEvent("evt-mode", "stream-a", 1, "idem-mode", "ModeSelected", map[string]string{"mode": "conversation"})
	create := projectionEvent("evt-create", "stream-b", 1, "idem-create", "WorkItemCreated", map[string]string{
		"work_item_id": "work-1",
		"title":        "Canonical replay",
		"status":       "open",
	})
	evidence := projectionEvent("evt-evidence", "stream-b", 2, "idem-evidence", "EvidenceSubmitted", map[string]string{
		"evidence_id":  "evidence-1",
		"work_item_id": "work-1",
		"digest":       digestB,
	})

	projection := newForTestSource(eventSliceSource{events: []journal.Event{evidence, create, mode, create}})
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	want := Snapshot{
		Modes: map[string]string{"stream-a": "conversation"},
		WorkItems: map[string]WorkItem{
			"work-1": {ID: "work-1", Title: "Canonical replay", Status: "open"},
		},
		Evidence: map[string]Evidence{
			"evidence-1": {ID: "evidence-1", WorkItemID: "work-1", Digest: digestB},
		},
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestRebuildRejectsConflictGapAndUnknownVersion(t *testing.T) {
	ctx := context.Background()
	valid := projectionEvent("evt-valid", "stream-a", 1, "idem-valid", "ModeSelected", map[string]string{"mode": "agent"})
	create := projectionEvent("evt-create", "stream-a", 1, "idem-create", "WorkItemCreated", map[string]string{
		"work_item_id": "work-1",
		"title":        "Projection",
		"status":       "open",
	})

	tests := []struct {
		name   string
		events []journal.Event
		want   error
	}{
		{
			name: "event id conflict",
			events: []journal.Event{
				valid,
				withProjectionEvent(valid, func(e *journal.Event) {
					e.StreamID = "stream-b"
					e.IdempotencyKey = "idem-conflicting-id"
				}),
			},
			want: ErrConflictingEvent,
		},
		{
			name: "stream sequence conflict",
			events: []journal.Event{
				valid,
				withProjectionEvent(valid, func(e *journal.Event) {
					e.ID = "evt-conflicting-seq"
					e.IdempotencyKey = "idem-conflicting-seq"
					e.PayloadJSON = payloadJSON(t, map[string]string{"mode": "conversation"})
				}),
			},
			want: ErrConflictingEvent,
		},
		{
			name: "sequence gap",
			events: []journal.Event{
				withProjectionEvent(valid, func(e *journal.Event) {
					e.Seq = 2
				}),
			},
			want: ErrSequenceGap,
		},
		{
			name: "unknown schema version",
			events: []journal.Event{
				withProjectionEvent(valid, func(e *journal.Event) {
					e.SchemaVersion = 2
				}),
			},
			want: ErrUnsupportedEventVersion,
		},
		{
			name: "invalid projection payload",
			events: []journal.Event{
				withProjectionEvent(valid, func(e *journal.Event) {
					e.PayloadJSON = []byte(`{"mode":"invalid"}`)
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
		{
			name: "malformed relevant payload",
			events: []journal.Event{
				withProjectionEvent(valid, func(e *journal.Event) {
					e.PayloadJSON = []byte(`{`)
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
		{
			name: "missing required work item field",
			events: []journal.Event{
				projectionEvent("evt-missing-work", "stream-a", 1, "idem-missing-work", "WorkItemCreated", map[string]string{
					"work_item_id": "work-1",
					"status":       "open",
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
		{
			name: "terminal before create",
			events: []journal.Event{
				projectionEvent("evt-terminal-before-create", "stream-a", 1, "idem-terminal-before-create", "WorkItemTerminal", map[string]string{
					"work_item_id": "work-1",
					"status":       "accepted",
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
		{
			name: "conflicting work item create",
			events: []journal.Event{
				create,
				projectionEvent("evt-create-conflict", "stream-a", 2, "idem-create-conflict", "WorkItemCreated", map[string]string{
					"work_item_id": "work-1",
					"title":        "Different",
					"status":       "open",
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
		{
			name: "evidence references unknown work item",
			events: []journal.Event{
				projectionEvent("evt-evidence-unknown", "stream-a", 1, "idem-evidence-unknown", "EvidenceSubmitted", map[string]string{
					"evidence_id":  "evidence-1",
					"work_item_id": "missing-work",
					"digest":       digestA,
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
		{
			name: "conflicting evidence repetition",
			events: []journal.Event{
				create,
				projectionEvent("evt-evidence", "stream-a", 2, "idem-evidence", "EvidenceSubmitted", map[string]string{
					"evidence_id":  "evidence-1",
					"work_item_id": "work-1",
					"digest":       digestA,
				}),
				projectionEvent("evt-evidence-conflict", "stream-a", 3, "idem-evidence-conflict", "EvidenceSubmitted", map[string]string{
					"evidence_id":  "evidence-1",
					"work_item_id": "work-1",
					"digest":       digestB,
				}),
			},
			want: ErrInvalidProjectionEvent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projection := newForTestSource(eventSliceSource{events: tt.events})
			err := projection.Rebuild(ctx)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Rebuild() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRebuildRejectsInvalidEvidenceDigestForms(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name   string
		digest string
	}{
		{name: "prefixed", digest: "sha256:" + digestA},
		{name: "short", digest: strings.Repeat("a", 63)},
		{name: "long", digest: strings.Repeat("a", 65)},
		{name: "uppercase", digest: strings.ToUpper(digestA)},
		{name: "nonhex", digest: strings.Repeat("g", 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projection := newForTestSource(eventSliceSource{events: []journal.Event{
				projectionEvent("evt-create", "stream-a", 1, "idem-create", "WorkItemCreated", map[string]string{
					"work_item_id": "work-1",
					"title":        "Digest validation",
					"status":       "open",
				}),
				projectionEvent("evt-evidence", "stream-a", 2, "idem-evidence", "EvidenceSubmitted", map[string]string{
					"evidence_id":  "evidence-1",
					"work_item_id": "work-1",
					"digest":       tt.digest,
				}),
			}})

			err := projection.Rebuild(ctx)
			if !errors.Is(err, ErrInvalidProjectionEvent) {
				t.Fatalf("Rebuild() error = %v, want ErrInvalidProjectionEvent", err)
			}
		})
	}
}

func TestRebuildAcceptsRepeatedIdenticalProjectedFacts(t *testing.T) {
	ctx := context.Background()
	projection := newForTestSource(eventSliceSource{events: []journal.Event{
		projectionEvent("evt-create-1", "stream-a", 1, "idem-create-1", "WorkItemCreated", map[string]string{
			"work_item_id": "work-1",
			"title":        "Repeated facts",
			"status":       "open",
		}),
		projectionEvent("evt-create-2", "stream-a", 2, "idem-create-2", "WorkItemCreated", map[string]string{
			"work_item_id": "work-1",
			"title":        "Repeated facts",
			"status":       "open",
		}),
		projectionEvent("evt-evidence-1", "stream-a", 3, "idem-evidence-1", "EvidenceSubmitted", map[string]string{
			"evidence_id":  "evidence-1",
			"work_item_id": "work-1",
			"digest":       digestA,
		}),
		projectionEvent("evt-evidence-2", "stream-a", 4, "idem-evidence-2", "EvidenceSubmitted", map[string]string{
			"evidence_id":  "evidence-1",
			"work_item_id": "work-1",
			"digest":       digestA,
		}),
	}})
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	got := projection.Snapshot()
	if len(got.WorkItems) != 1 || got.WorkItems["work-1"] != (WorkItem{ID: "work-1", Title: "Repeated facts", Status: "open"}) {
		t.Fatalf("WorkItems = %#v, want one repeated fact", got.WorkItems)
	}
	if len(got.Evidence) != 1 || got.Evidence["evidence-1"] != (Evidence{ID: "evidence-1", WorkItemID: "work-1", Digest: digestA}) {
		t.Fatalf("Evidence = %#v, want one repeated fact", got.Evidence)
	}
}

func TestFailedRebuildPreservesPreviousSnapshot(t *testing.T) {
	ctx := context.Background()
	source := &mutableSource{events: []journal.Event{
		projectionEvent("evt-mode", "stream-a", 1, "idem-mode", "ModeSelected", map[string]string{"mode": "agent"}),
	}}
	projection := newForTestSource(source)
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("valid Rebuild() error = %v", err)
	}
	before := projection.Snapshot()

	invalid := append([]journal.Event{}, source.events...)
	invalid = append(invalid, projectionEvent("evt-gap", "stream-a", 3, "idem-gap", "ModeSelected", map[string]string{"mode": "conversation"}))
	source.set(invalid)
	if err := projection.Rebuild(ctx); !errors.Is(err, ErrSequenceGap) {
		t.Fatalf("invalid Rebuild() error = %v, want ErrSequenceGap", err)
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot after failed rebuild = %#v, want preserved %#v", got, before)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	source.set(source.events[:1])
	if err := projection.Rebuild(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Rebuild() error = %v, want context.Canceled", err)
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot after canceled rebuild = %#v, want preserved %#v", got, before)
	}
}

func TestSnapshotIsReadOnlyCopy(t *testing.T) {
	ctx := context.Background()
	projection := newForTestSource(eventSliceSource{events: []journal.Event{
		projectionEvent("evt-mode", "stream-a", 1, "idem-mode", "ModeSelected", map[string]string{"mode": "agent"}),
		projectionEvent("evt-create", "stream-b", 1, "idem-create", "WorkItemCreated", map[string]string{
			"work_item_id": "work-1",
			"title":        "Read only",
			"status":       "open",
		}),
		projectionEvent("evt-evidence", "stream-b", 2, "idem-evidence", "EvidenceSubmitted", map[string]string{
			"evidence_id":  "evidence-1",
			"work_item_id": "work-1",
			"digest":       digestA,
		}),
	}})
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	mutated := projection.Snapshot()
	mutated.Modes["stream-a"] = "conversation"
	mutated.WorkItems["work-1"] = WorkItem{ID: "work-1", Title: "mutated", Status: "accepted"}
	mutated.Evidence["evidence-1"] = Evidence{ID: "evidence-1", WorkItemID: "work-2", Digest: digestB}
	delete(mutated.Modes, "stream-a")
	delete(mutated.WorkItems, "work-1")
	delete(mutated.Evidence, "evidence-1")

	got := projection.Snapshot()
	if got.Modes["stream-a"] != "agent" {
		t.Fatalf("mode after caller mutation = %q, want agent", got.Modes["stream-a"])
	}
	if got.WorkItems["work-1"] != (WorkItem{ID: "work-1", Title: "Read only", Status: "open"}) {
		t.Fatalf("work item after caller mutation = %#v", got.WorkItems["work-1"])
	}
	if got.Evidence["evidence-1"] != (Evidence{ID: "evidence-1", WorkItemID: "work-1", Digest: digestA}) {
		t.Fatalf("evidence after caller mutation = %#v", got.Evidence["evidence-1"])
	}
}

func TestRebuildJournalFailurePreservesPreviousSnapshot(t *testing.T) {
	ctx := context.Background()
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	if _, err := store.Append(ctx, projectionEvent("evt-mode", "stream-a", 1, "idem-mode", "ModeSelected", map[string]string{"mode": "agent"})); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	projection := New(db)
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("valid Rebuild() error = %v", err)
	}
	before := projection.Snapshot()

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := projection.Rebuild(ctx); err == nil {
		t.Fatal("Rebuild() error = nil, want Journal query failure")
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot after Journal query failure = %#v, want preserved %#v", got, before)
	}
}

func TestRebuildSerializesConcurrentCandidatesAndCanceledWaiterDoesNotSwap(t *testing.T) {
	ctx := context.Background()
	source := newBlockingSource()
	projection := newForTestSource(source)

	slowCall := source.setNext([]journal.Event{
		projectionEvent("evt-slow", "stream-a", 1, "idem-slow", "ModeSelected", map[string]string{"mode": "conversation"}),
	})
	slowErr := make(chan error, 1)
	go func() {
		slowErr <- projection.Rebuild(ctx)
	}()
	slowCall.waitStarted(t)

	fastCall := source.setNext([]journal.Event{
		projectionEvent("evt-fast", "stream-a", 1, "idem-fast", "ModeSelected", map[string]string{"mode": "agent"}),
	})
	fastErr := make(chan error, 1)
	go func() {
		fastErr <- projection.Rebuild(ctx)
	}()

	slowCall.release()
	if err := <-slowErr; err != nil {
		t.Fatalf("slow Rebuild() error = %v", err)
	}
	fastCall.waitStarted(t)
	fastCall.release()
	if err := <-fastErr; err != nil {
		t.Fatalf("fast Rebuild() error = %v", err)
	}
	if got := projection.Snapshot(); got.Modes["stream-a"] != "agent" {
		t.Fatalf("snapshot after serialized rebuilds = %#v, want newest agent snapshot", got)
	}

	holdingCall := source.setNext([]journal.Event{
		projectionEvent("evt-hold", "stream-a", 1, "idem-hold", "ModeSelected", map[string]string{"mode": "conversation"}),
	})
	holdingErr := make(chan error, 1)
	go func() {
		holdingErr <- projection.Rebuild(ctx)
	}()
	holdingCall.waitStarted(t)

	waiterCtx, cancel := context.WithCancel(ctx)
	canceledErr := make(chan error, 1)
	go func() {
		canceledErr <- projection.Rebuild(waiterCtx)
	}()
	cancel()
	holdingCall.release()
	if err := <-holdingErr; err != nil {
		t.Fatalf("holding Rebuild() error = %v", err)
	}
	if err := <-canceledErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter Rebuild() error = %v, want context.Canceled", err)
	}
	if got := projection.Snapshot(); got.Modes["stream-a"] != "conversation" {
		t.Fatalf("snapshot after canceled waiter = %#v, want holding rebuild only", got)
	}
}

type eventSliceSource struct {
	events []journal.Event
}

func (s eventSliceSource) Events(ctx context.Context) ([]journal.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]journal.Event(nil), s.events...), nil
}

type mutableSource struct {
	mu     sync.Mutex
	events []journal.Event
}

func (s *mutableSource) set(events []journal.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append([]journal.Event(nil), events...)
}

func (s *mutableSource) Events(ctx context.Context) ([]journal.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]journal.Event(nil), s.events...), nil
}

type blockingSource struct {
	mu    sync.Mutex
	calls []*blockingCall
}

type blockingCall struct {
	events    []journal.Event
	started   chan struct{}
	releaseCh chan struct{}
}

func newBlockingSource() *blockingSource {
	return &blockingSource{}
}

func (s *blockingSource) setNext(events []journal.Event) *blockingCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	call := &blockingCall{
		events:    append([]journal.Event(nil), events...),
		started:   make(chan struct{}),
		releaseCh: make(chan struct{}),
	}
	s.calls = append(s.calls, call)
	return call
}

func (s *blockingSource) Events(ctx context.Context) ([]journal.Event, error) {
	s.mu.Lock()
	if len(s.calls) == 0 {
		s.mu.Unlock()
		return nil, errors.New("missing blocking source call")
	}
	call := s.calls[0]
	s.calls = s.calls[1:]
	s.mu.Unlock()
	close(call.started)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-call.releaseCh:
		return append([]journal.Event(nil), call.events...), nil
	}
}

func (c *blockingCall) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-c.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for source to start")
	}
}

func (c *blockingCall) release() {
	close(c.releaseCh)
}

func newForTestSource(source source) *Projection {
	return &Projection{
		source:      source,
		snapshot:    emptySnapshot(),
		rebuildGate: make(chan struct{}, 1),
	}
}

func openProjectionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "journal.db")
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", dbPath, values.Encode()))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("journal.Migrate() error = %v", err)
	}
	return db
}

func projectionEvent(id, streamID string, seq int64, idempotencyKey string, eventType string, payload map[string]string) journal.Event {
	return journal.Event{
		ID:             id,
		StreamID:       streamID,
		Seq:            seq,
		IdempotencyKey: idempotencyKey,
		Type:           eventType,
		SchemaVersion:  1,
		EmittedAt:      time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC),
		PayloadJSON:    mustPayload(payload),
	}
}

func withProjectionEvent(event journal.Event, mutate func(*journal.Event)) journal.Event {
	mutate(&event)
	return event
}

func payloadJSON(t *testing.T, payload map[string]string) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func mustPayload(payload map[string]string) []byte {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return data
}

const (
	digestA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	digestB = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)
