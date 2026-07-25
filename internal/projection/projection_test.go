package projection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"net/url"
	"os"
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
		Teams:            map[string]TeamInstance{},
		AgentInstances:   map[string]AgentInstance{},
		RuntimeInstances: map[string]RuntimeInstance{},
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
		Teams:            map[string]TeamInstance{},
		AgentInstances:   map[string]AgentInstance{},
		RuntimeInstances: map[string]RuntimeInstance{},
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

func TestRebuildSavedTeamInstanceFactsFromJournal(t *testing.T) {
	ctx := context.Background()
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	events := savedTeamProjectionEvents(t, savedTeamProjectionOptions{
		dormantCount: 2,
	})
	if got, err := store.AppendBatch(ctx, events); err != nil || len(got) != 2 {
		t.Fatalf("AppendBatch() = (%#v,%v)", got, err)
	}

	first := New(db)
	if err := first.Rebuild(ctx); err != nil {
		t.Fatalf("first Rebuild() error = %v", err)
	}
	second := New(db)
	if err := second.Rebuild(ctx); err != nil {
		t.Fatalf("second Rebuild() error = %v", err)
	}
	if !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatalf("recreated snapshots differ: first=%#v second=%#v", first.Snapshot(), second.Snapshot())
	}
	assertSavedTeamProjectionSnapshot(t, first.Snapshot(), savedTeamProjectionOptions{
		dormantCount: 2,
	})
}

func TestRebuildSavedTeamInstanceFactsCardinalityScopeAndOrder(t *testing.T) {
	tests := []struct {
		name    string
		options savedTeamProjectionOptions
	}{
		{name: "project main only", options: savedTeamProjectionOptions{}},
		{name: "project one dormant", options: savedTeamProjectionOptions{dormantCount: 1}},
		{name: "project two dormant", options: savedTeamProjectionOptions{dormantCount: 2}},
		{name: "reusable same-ID shadow", options: savedTeamProjectionOptions{
			dormantCount:   2,
			teamScope:      "reusable",
			agentScope:     "reusable",
			teamVersion:    2,
			agentVersion:   3,
			teamProjectID:  "",
			agentProjectID: "",
		}},
		{name: "project Team reusable Agent fallback", options: savedTeamProjectionOptions{
			dormantCount:   2,
			teamScope:      "project",
			agentScope:     "reusable",
			teamProjectID:  "project.one",
			agentProjectID: "",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := savedTeamProjectionEvents(t, tt.options)
			projection := newForTestSource(eventSliceSource{
				events: []journal.Event{events[1], events[0]},
			})
			if err := projection.Rebuild(context.Background()); err != nil {
				t.Fatalf("Rebuild() error = %v", err)
			}
			assertSavedTeamProjectionSnapshot(t, projection.Snapshot(), tt.options)
		})
	}
}

func TestRebuildSavedTeamInstanceFactsRejectsInvalidTeamFacts(t *testing.T) {
	tests := []struct {
		name   string
		change func(*journal.Event)
	}{
		{name: "empty event id", change: func(event *journal.Event) { event.ID = "" }},
		{name: "empty idempotency key", change: func(event *journal.Event) { event.IdempotencyKey = "" }},
		{name: "zero emitted at", change: func(event *journal.Event) { event.EmittedAt = time.Time{} }},
		{name: "wrong stream", change: func(event *journal.Event) { event.StreamID = "team_instance:other" }},
		{name: "wrong sequence", change: func(event *journal.Event) { event.Seq = 2 }},
		{name: "empty correlation", change: func(event *journal.Event) { event.CorrelationID = "" }},
		{name: "unexpected causation", change: func(event *journal.Event) { event.CausationID = "event.other" }},
		{name: "malformed payload", change: func(event *journal.Event) { event.PayloadJSON = []byte(`{`) }},
		{name: "unknown payload field", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["unknown"] = true
		})},
		{name: "missing dormant list", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload, "dormant_sub_agents")
		})},
		{name: "missing Team count", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload, "team_instance_count")
		})},
		{name: "missing Agent count", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload, "agent_instance_count")
		})},
		{name: "missing active count", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload, "active_sub_agent_count")
		})},
		{name: "missing WorkItem count", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload, "work_item_count")
		})},
		{name: "missing Team id", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["id"] = ""
		})},
		{name: "wrong work request", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["work_request_id"] = "request.other"
		})},
		{name: "invalid source kind", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["source_kind"] = "draft"
		})},
		{name: "invalid state", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["state"] = "running"
		})},
		{name: "zero version", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["team_definition_version"] = float64(0)
		})},
		{name: "invalid project scope", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["scope_identity"].(map[string]any)["project_id"] = ""
		})},
		{name: "missing Team project id", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload["team"].(map[string]any)["scope_identity"].(map[string]any), "project_id")
		})},
		{name: "missing Team generation id", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload["team"].(map[string]any)["scope_identity"].(map[string]any), "generation_id")
		})},
		{name: "invalid reusable scope", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			team := payload["team"].(map[string]any)
			team["team_definition_scope"] = "reusable"
		})},
		{name: "invalid Team digest", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team"].(map[string]any)["team_definition_digest"] = "invalid"
		})},
		{name: "source plan mismatch", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["source_plan_digest"] = digestA
		})},
		{name: "invalid record digest", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["source_record_set_digest"] = "invalid"
		})},
		{name: "invalid Team count", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team_instance_count"] = float64(2)
		})},
		{name: "invalid Agent count", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["agent_instance_count"] = float64(2)
		})},
		{name: "active dormant Agent", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["active_sub_agent_count"] = float64(1)
		})},
		{name: "future WorkItem", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["work_item_count"] = float64(1)
		})},
		{name: "too many dormant", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			dormant := payload["dormant_sub_agents"].([]any)
			payload["dormant_sub_agents"] = append(dormant, dormant[0])
		})},
		{name: "invalid dormant state", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["dormant_sub_agents"].([]any)[0].(map[string]any)["dormant"] = false
		})},
		{name: "duplicate dormant Agent", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			dormant := payload["dormant_sub_agents"].([]any)
			dormant[1].(map[string]any)["agent_definition_id"] =
				dormant[0].(map[string]any)["agent_definition_id"]
		})},
		{name: "unsorted dormant", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			dormant := payload["dormant_sub_agents"].([]any)
			dormant[0], dormant[1] = dormant[1], dormant[0]
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := savedTeamProjectionEvents(t, savedTeamProjectionOptions{dormantCount: 2})
			tt.change(&events[0])
			assertSavedTeamProjectionRebuildError(t, events)
		})
	}
}

func TestRebuildSavedTeamInstanceFactsRejectsInvalidAgentFacts(t *testing.T) {
	tests := []struct {
		name   string
		change func(*journal.Event)
	}{
		{name: "empty event id", change: func(event *journal.Event) { event.ID = "" }},
		{name: "empty idempotency key", change: func(event *journal.Event) { event.IdempotencyKey = "" }},
		{name: "zero emitted at", change: func(event *journal.Event) { event.EmittedAt = time.Time{} }},
		{name: "wrong stream", change: func(event *journal.Event) { event.StreamID = "agent_instance:other" }},
		{name: "wrong sequence", change: func(event *journal.Event) { event.Seq = 2 }},
		{name: "empty correlation", change: func(event *journal.Event) { event.CorrelationID = "" }},
		{name: "empty causation", change: func(event *journal.Event) { event.CausationID = "" }},
		{name: "malformed payload", change: func(event *journal.Event) { event.PayloadJSON = []byte(`{`) }},
		{name: "unknown payload field", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["unknown"] = true
		})},
		{name: "missing Agent id", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["main_agent"].(map[string]any)["id"] = ""
		})},
		{name: "zero version", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["main_agent"].(map[string]any)["agent_definition_version"] = float64(0)
		})},
		{name: "not Main", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["main_agent"].(map[string]any)["is_main"] = false
		})},
		{name: "invalid state", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["main_agent"].(map[string]any)["state"] = "running"
		})},
		{name: "invalid Agent scope", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["main_agent"].(map[string]any)["agent_definition_scope"] = "transient"
		})},
		{name: "missing Agent project id", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload["main_agent"].(map[string]any)["scope_identity"].(map[string]any), "project_id")
		})},
		{name: "missing Agent generation id", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			delete(payload["main_agent"].(map[string]any)["scope_identity"].(map[string]any), "generation_id")
		})},
		{name: "invalid binding accepted", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["runtime_binding"].(map[string]any)["accepted"] = false
		})},
		{name: "binding profile mismatch", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["runtime_binding"].(map[string]any)["profile_id"] = "profile.other"
		})},
		{name: "binding instance mismatch", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["runtime_binding"].(map[string]any)["instance_id"] = "runtime.other"
		})},
		{name: "invalid plan digest", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["source_plan_digest"] = "invalid"
		})},
		{name: "invalid record digest", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["source_record_set_digest"] = "invalid"
		})},
		{name: "zero Team created at", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["team_created_at"] = float64(0)
		})},
		{name: "invalid binding digest", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["binding_digest"] = "invalid"
		})},
		{name: "invalid discovery digest", change: mutateSavedTeamProjectionPayload(t, func(payload map[string]any) {
			payload["runtime_discovery_digest"] = "invalid"
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := savedTeamProjectionEvents(t, savedTeamProjectionOptions{dormantCount: 2})
			tt.change(&events[1])
			assertSavedTeamProjectionRebuildError(t, events)
		})
	}
}

func TestRebuildSavedTeamInstanceFactsRejectsBrokenLinks(t *testing.T) {
	tests := []struct {
		name   string
		events func(*testing.T) []journal.Event
	}{
		{name: "missing Main", events: func(t *testing.T) []journal.Event {
			return savedTeamProjectionEvents(t, savedTeamProjectionOptions{})[:1]
		}},
		{name: "orphan Main", events: func(t *testing.T) []journal.Event {
			return savedTeamProjectionEvents(t, savedTeamProjectionOptions{})[1:]
		}},
		{name: "wrong Team id", events: mutateSavedTeamProjectionEvents(func(t *testing.T, events []journal.Event) {
			mutateSavedTeamProjectionEventPayload(t, &events[1], func(payload map[string]any) {
				payload["main_agent"].(map[string]any)["team_instance_id"] = "team-instance.other"
			})
		})},
		{name: "wrong work request", events: mutateSavedTeamProjectionEvents(func(_ *testing.T, events []journal.Event) {
			events[1].CorrelationID = "request.other"
		})},
		{name: "wrong causation", events: mutateSavedTeamProjectionEvents(func(_ *testing.T, events []journal.Event) {
			events[1].CausationID = "event.team.other"
		})},
		{name: "wrong plan digest", events: mutateSavedTeamProjectionEvents(func(t *testing.T, events []journal.Event) {
			mutateSavedTeamProjectionEventPayload(t, &events[1], func(payload map[string]any) {
				payload["source_plan_digest"] = digestA
			})
		})},
		{name: "wrong record digest", events: mutateSavedTeamProjectionEvents(func(t *testing.T, events []journal.Event) {
			mutateSavedTeamProjectionEventPayload(t, &events[1], func(payload map[string]any) {
				payload["source_record_set_digest"] = digestB
			})
		})},
		{name: "wrong created at", events: mutateSavedTeamProjectionEvents(func(t *testing.T, events []journal.Event) {
			mutateSavedTeamProjectionEventPayload(t, &events[1], func(payload map[string]any) {
				payload["team_created_at"] = float64(1_721_865_601)
			})
		})},
		{name: "duplicate Main", events: func(t *testing.T) []journal.Event {
			events := savedTeamProjectionEvents(t, savedTeamProjectionOptions{})
			second := cloneProjectionEvent(events[1])
			second.ID = "event.agent.second"
			second.StreamID = "agent_instance:agent-instance.second"
			second.IdempotencyKey = "key.agent.second"
			mutateSavedTeamProjectionEventPayload(t, &second, func(payload map[string]any) {
				payload["main_agent"].(map[string]any)["id"] = "agent-instance.second"
			})
			return append(events, second)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSavedTeamProjectionRebuildError(t, tt.events(t))
		})
	}
}

func TestRebuildSavedTeamInstanceFactsFailurePreservesSnapshot(t *testing.T) {
	ctx := context.Background()
	source := &mutableSource{events: savedTeamProjectionEvents(t, savedTeamProjectionOptions{
		dormantCount: 2,
	})}
	projection := newForTestSource(source)
	if err := projection.Rebuild(ctx); err != nil {
		t.Fatalf("valid Rebuild() error = %v", err)
	}
	before := projection.Snapshot()

	invalid := savedTeamProjectionEvents(t, savedTeamProjectionOptions{dormantCount: 2})
	invalid[1].CausationID = "event.other"
	source.set(invalid)
	if err := projection.Rebuild(ctx); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("invalid Rebuild() error = %v", err)
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot after invalid links = %#v, want %#v", got, before)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	source.set(savedTeamProjectionEvents(t, savedTeamProjectionOptions{}))
	if err := projection.Rebuild(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Rebuild() error = %v", err)
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("snapshot after cancellation = %#v, want %#v", got, before)
	}

	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	if _, err := store.AppendBatch(ctx, savedTeamProjectionEvents(t, savedTeamProjectionOptions{})); err != nil {
		t.Fatalf("AppendBatch() error = %v", err)
	}
	fromDB := New(db)
	if err := fromDB.Rebuild(ctx); err != nil {
		t.Fatalf("database Rebuild() error = %v", err)
	}
	dbBefore := fromDB.Snapshot()
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := fromDB.Rebuild(ctx); err == nil {
		t.Fatal("closed database Rebuild() error = nil")
	}
	if got := fromDB.Snapshot(); !reflect.DeepEqual(got, dbBefore) {
		t.Fatalf("snapshot after closed database = %#v, want %#v", got, dbBefore)
	}
}

func TestRebuildSavedTeamInstanceFactsSerializesConcurrentRebuilds(t *testing.T) {
	ctx := context.Background()
	source := newBlockingSource()
	projection := newForTestSource(source)

	slowCall := source.setNext(savedTeamProjectionEvents(t, savedTeamProjectionOptions{
		dormantCount: 1,
	}))
	slowErr := make(chan error, 1)
	go func() {
		slowErr <- projection.Rebuild(ctx)
	}()
	slowCall.waitStarted(t)

	fastOptions := savedTeamProjectionOptions{
		dormantCount:   2,
		teamScope:      "reusable",
		agentScope:     "reusable",
		teamProjectID:  "",
		agentProjectID: "",
	}
	fastCall := source.setNext(savedTeamProjectionEvents(t, fastOptions))
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
	assertSavedTeamProjectionSnapshot(t, projection.Snapshot(), fastOptions)

	holdingCall := source.setNext(savedTeamProjectionEvents(t, savedTeamProjectionOptions{}))
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
		t.Fatalf("canceled waiter Rebuild() error = %v", err)
	}
	assertSavedTeamProjectionSnapshot(
		t, projection.Snapshot(), savedTeamProjectionOptions{},
	)
}

func TestRebuildSavedTeamInstanceFactsSnapshotMutationIsolation(t *testing.T) {
	projection := newForTestSource(eventSliceSource{events: savedTeamProjectionEvents(
		t, savedTeamProjectionOptions{dormantCount: 2},
	)})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	mutated := projection.Snapshot()
	team := mutated.Teams["team-instance.one"]
	team.DormantSubAgents[0].AgentDefinitionID = "mutated"
	mutated.Teams["team-instance.one"] = team
	delete(mutated.Teams, "team-instance.one")
	agent := mutated.AgentInstances["agent-instance.main"]
	agent.RuntimeInstanceID = "mutated"
	mutated.AgentInstances["agent-instance.main"] = agent
	delete(mutated.AgentInstances, "agent-instance.main")

	got := projection.Snapshot()
	if got.Teams["team-instance.one"].DormantSubAgents[0].AgentDefinitionID != "agent.sub.one" {
		t.Fatalf("Team after caller mutation = %#v", got.Teams)
	}
	if got.AgentInstances["agent-instance.main"].RuntimeInstanceID != "runtime.shared" {
		t.Fatalf("Agent after caller mutation = %#v", got.AgentInstances)
	}
}

func TestRebuildSavedTeamInstanceFactsProductionBoundary(t *testing.T) {
	content, err := os.ReadFile("projection.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(), "projection.go", content, parser.ImportsOnly,
	)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	allowed := map[string]bool{
		`"bytes"`:                            true,
		`"context"`:                          true,
		`"database/sql"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"fmt"`:                              true,
		`"sort"`:                             true,
		`"strings"`:                          true,
		`"sync"`:                             true,
		`"time"`:                             true,
		`"loom-pi-rebuild/internal/journal"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("projection.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

type savedTeamProjectionOptions struct {
	dormantCount   int
	teamScope      string
	agentScope     string
	teamVersion    int
	agentVersion   int
	teamProjectID  string
	agentProjectID string
}

func normalizeSavedTeamProjectionOptions(
	options savedTeamProjectionOptions,
) savedTeamProjectionOptions {
	if options.teamScope == "" {
		options.teamScope = "project"
		options.teamProjectID = "project.one"
	}
	if options.agentScope == "" {
		options.agentScope = "project"
		options.agentProjectID = "project.one"
	}
	if options.teamVersion == 0 {
		options.teamVersion = 1
	}
	if options.agentVersion == 0 {
		options.agentVersion = 1
	}
	return options
}

func savedTeamProjectionEvents(
	t *testing.T,
	options savedTeamProjectionOptions,
) []journal.Event {
	t.Helper()
	options = normalizeSavedTeamProjectionOptions(options)
	dormant := []any{
		map[string]any{
			"dormant":             true,
			"agent_definition_id": "agent.sub.one",
			"runtime_profile_id":  "profile.sub.one",
			"runtime_instance_id": "runtime.shared",
		},
		map[string]any{
			"dormant":             true,
			"agent_definition_id": "agent.sub.two",
			"runtime_profile_id":  "profile.sub.two",
			"runtime_instance_id": "runtime.shared",
		},
	}[:options.dormantCount]
	teamPayload := map[string]any{
		"team": map[string]any{
			"id":                      "team-instance.one",
			"work_request_id":         "request.one",
			"source_kind":             "saved_team",
			"team_definition_id":      "team.delivery",
			"team_definition_version": options.teamVersion,
			"team_definition_scope":   options.teamScope,
			"scope_identity": map[string]any{
				"project_id":    options.teamProjectID,
				"generation_id": "",
			},
			"team_definition_digest": digestA,
			"source_plan_digest":     digestB,
			"state":                  "created",
			"created_at":             int64(1_721_865_600),
		},
		"dormant_sub_agents":       dormant,
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_instance_count":      1,
		"agent_instance_count":     1,
		"active_sub_agent_count":   0,
		"work_item_count":          0,
	}
	mainPayload := map[string]any{
		"main_agent": map[string]any{
			"id":                       "agent-instance.main",
			"team_instance_id":         "team-instance.one",
			"agent_definition_id":      "agent.main",
			"agent_definition_version": options.agentVersion,
			"agent_definition_scope":   options.agentScope,
			"scope_identity": map[string]any{
				"project_id":    options.agentProjectID,
				"generation_id": "",
			},
			"runtime_profile_id":  "profile.main",
			"runtime_instance_id": "runtime.shared",
			"is_main":             true,
			"state":               "created",
		},
		"runtime_binding": map[string]any{
			"accepted":    true,
			"profile_id":  "profile.main",
			"instance_id": "runtime.shared",
		},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_created_at":          int64(1_721_865_600),
		"binding_digest":           digestB,
		"runtime_discovery_digest": digestA,
	}
	return []journal.Event{
		{
			ID:             "event.team.created",
			StreamID:       "team_instance:team-instance.one",
			Seq:            1,
			IdempotencyKey: "key.team.created",
			Type:           "TeamInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 2, 30, 0, 0, time.UTC),
			CorrelationID:  "request.one",
			PayloadJSON:    savedTeamProjectionPayload(t, teamPayload),
		},
		{
			ID:             "event.agent.created",
			StreamID:       "agent_instance:agent-instance.main",
			Seq:            1,
			IdempotencyKey: "key.agent.created",
			Type:           "AgentInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 2, 30, 0, 0, time.UTC),
			CorrelationID:  "request.one",
			CausationID:    "event.team.created",
			PayloadJSON:    savedTeamProjectionPayload(t, mainPayload),
		},
	}
}

func assertSavedTeamProjectionSnapshot(
	t *testing.T,
	snapshot Snapshot,
	options savedTeamProjectionOptions,
) {
	t.Helper()
	options = normalizeSavedTeamProjectionOptions(options)
	if len(snapshot.Teams) != 1 || len(snapshot.AgentInstances) != 1 {
		t.Fatalf("saved-Team snapshot = %#v", snapshot)
	}
	team := snapshot.Teams["team-instance.one"]
	if team.ID != "team-instance.one" ||
		team.WorkRequestID != "request.one" ||
		team.SourceKind != "saved_team" ||
		team.TeamDefinitionID != "team.delivery" ||
		team.TeamDefinitionVersion != options.teamVersion ||
		team.TeamDefinitionScope != options.teamScope ||
		team.ScopeIdentity.ProjectID != options.teamProjectID ||
		team.TeamDefinitionDigest != digestA ||
		team.SourcePlanDigest != digestB ||
		team.SourceRecordSetDigest != digestA ||
		team.State != "created" ||
		team.CreatedAt != 1_721_865_600 ||
		len(team.DormantSubAgents) != options.dormantCount ||
		team.TeamInstanceCount != 1 ||
		team.AgentInstanceCount != 1 ||
		team.ActiveSubAgentCount != 0 ||
		team.WorkItemCount != 0 ||
		team.CreationEventID != "event.team.created" {
		t.Fatalf("TeamInstance = %#v", team)
	}
	agent := snapshot.AgentInstances["agent-instance.main"]
	if agent.ID != "agent-instance.main" ||
		agent.TeamInstanceID != team.ID ||
		agent.WorkRequestID != team.WorkRequestID ||
		agent.AgentDefinitionID != "agent.main" ||
		agent.AgentDefinitionVersion != options.agentVersion ||
		agent.AgentDefinitionScope != options.agentScope ||
		agent.ScopeIdentity.ProjectID != options.agentProjectID ||
		agent.RuntimeProfileID != "profile.main" ||
		agent.RuntimeInstanceID != "runtime.shared" ||
		!agent.IsMain ||
		agent.State != "created" ||
		!agent.RuntimeBinding.Accepted ||
		agent.RuntimeBinding.ProfileID != agent.RuntimeProfileID ||
		agent.RuntimeBinding.InstanceID != agent.RuntimeInstanceID ||
		agent.SourcePlanDigest != team.SourcePlanDigest ||
		agent.SourceRecordSetDigest != team.SourceRecordSetDigest ||
		agent.TeamCreatedAt != team.CreatedAt ||
		agent.BindingDigest != digestB ||
		agent.RuntimeDiscoveryDigest != digestA ||
		agent.CreationEventID != "event.agent.created" ||
		agent.TeamCreationEventID != team.CreationEventID {
		t.Fatalf("AgentInstance = %#v", agent)
	}
}

func mutateSavedTeamProjectionPayload(
	t *testing.T,
	change func(map[string]any),
) func(*journal.Event) {
	t.Helper()
	return func(event *journal.Event) {
		mutateSavedTeamProjectionEventPayload(t, event, change)
	}
}

func mutateSavedTeamProjectionEventPayload(
	t *testing.T,
	event *journal.Event,
	change func(map[string]any),
) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	change(payload)
	event.PayloadJSON = savedTeamProjectionPayload(t, payload)
}

func mutateSavedTeamProjectionEvents(
	change func(*testing.T, []journal.Event),
) func(*testing.T) []journal.Event {
	return func(t *testing.T) []journal.Event {
		t.Helper()
		events := savedTeamProjectionEvents(t, savedTeamProjectionOptions{})
		change(t, events)
		return events
	}
}

func savedTeamProjectionPayload(t *testing.T, payload map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func cloneProjectionEvent(event journal.Event) journal.Event {
	event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	return event
}

func assertSavedTeamProjectionRebuildError(t *testing.T, events []journal.Event) {
	t.Helper()
	projection := newForTestSource(eventSliceSource{events: events})
	if err := projection.Rebuild(context.Background()); !errors.Is(err, ErrInvalidProjectionEvent) &&
		!errors.Is(err, ErrSequenceGap) {
		t.Fatalf("Rebuild() error = %v, want invalid saved-Team projection", err)
	}
	if got := projection.Snapshot(); len(got.Teams) != 0 || len(got.AgentInstances) != 0 {
		t.Fatalf("failed rebuild swapped saved-Team snapshot %#v", got)
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
