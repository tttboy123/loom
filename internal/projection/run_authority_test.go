package projection

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

var runProjectionTime = time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC)

type mutableRunAuthoritySource struct {
	mu     sync.Mutex
	events []journal.Event
}

func (source *mutableRunAuthoritySource) Events(context.Context) ([]journal.Event, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	events := make([]journal.Event, len(source.events))
	for index, event := range source.events {
		events[index] = event
		events[index].PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	}
	return events, nil
}

func (source *mutableRunAuthoritySource) Set(events []journal.Event) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.events = events
}

func runAuthorityEvent(
	id string,
	streamID string,
	sequence int64,
	eventType string,
	correlationID string,
	causationID string,
	payload map[string]any,
) journal.Event {
	body, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return journal.Event{
		ID:             id,
		StreamID:       streamID,
		Seq:            sequence,
		IdempotencyKey: "idem-" + id,
		Type:           eventType,
		SchemaVersion:  1,
		EmittedAt:      runProjectionTime.Add(time.Duration(sequence) * time.Second),
		CorrelationID:  correlationID,
		CausationID:    causationID,
		PayloadJSON:    body,
	}
}

func runAuthorityStatusEvent(toStatus string) journal.Event {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return runAuthorityEvent(
		"runtime-status-"+toStatus,
		"runtime_instance:runtime-1",
		2,
		"RuntimeInstanceStatusChanged",
		"33333333-3333-4333-8333-333333333333",
		"runtime-discovery",
		map[string]any{
			"reconciliation_digest":   digest,
			"baseline_digest":         digest,
			"source_discovery_digest": digest,
			"source_probe_id":         "probe-1",
			"runtime_instance_id":     "runtime-1",
			"device_id":               "device-1",
			"adapter_type":            "pi-cli",
			"from_status":             "online",
			"to_status":               toStatus,
			"previous_event_id":       "runtime-discovery",
			"previous_sequence":       1,
		},
	)
}

func validRunAuthorityProjectionEvents() []journal.Event {
	const correlation = "11111111-1111-4111-8111-111111111111"
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	lease := runProjectionTime.Add(time.Minute).Format(time.RFC3339Nano)
	return []journal.Event{
		runAuthorityEvent(
			"runtime-discovery", "runtime_instance:runtime-1", 1,
			"RuntimeInstanceDiscovered", "22222222-2222-4222-8222-222222222222", "",
			map[string]any{
				"discovery_digest": digest,
				"source_probe_id":  "probe-1",
				"instance": map[string]any{
					"id": "runtime-1", "device_id": "device-1",
					"adapter_type": "pi-cli", "display_name": "Fixture",
					"executable_version": "1.0.0", "status": "online",
					"observed_capabilities": []string{"models"}, "capacity": 1,
				},
				"model_ids": []string{},
			},
		),
		runAuthorityEvent(
			"runtime-reserve", "runtime_capacity:runtime-1", 1,
			"RuntimeCapacityReserved", correlation, "run-claimed",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"claim_generation": 1, "runtime_instance_id": "runtime-1",
				"agent_instance_id":        "agent-1",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		runAuthorityEvent(
			"runtime-release", "runtime_capacity:runtime-1", 2,
			"RuntimeCapacityReleased", correlation, "run-terminal",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"claim_generation": 1, "runtime_instance_id": "runtime-1",
				"agent_instance_id":        "agent-1",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		runAuthorityEvent(
			"work-created", "work-item/work-1", 1,
			"WorkItemCreated", correlation, "",
			map[string]any{
				"work_item_id": "work-1", "title": "Implement work-1", "status": "ready",
			},
		),
		runAuthorityEvent(
			"work-assigned", "work-item/work-1", 2,
			"WorkItemAssigned", correlation, "work-created",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"agent_instance_id": "agent-1", "status": "assigned",
			},
		),
		runAuthorityEvent(
			"work-ready", "work-item/work-1", 3,
			"WorkItemReadyForReview", correlation, "run-terminal",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"claim_generation": 1, "status": "ready_for_review",
			},
		),
		runAuthorityEvent(
			"run-claimed", "run/run-1", 1,
			"RunClaimed", correlation, "work-assigned",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"claim_generation": 1, "runtime_instance_id": "runtime-1",
				"agent_instance_id":        "agent-1",
				"prepare_lease_expires_at": lease,
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		runAuthorityEvent(
			"run-started", "run/run-1", 2,
			"RunStarted", correlation, "run-claimed",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"claim_generation": 1, "runtime_instance_id": "runtime-1",
				"agent_instance_id":        "agent-1",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
		runAuthorityEvent(
			"run-terminal", "run/run-1", 3,
			"RunTerminalCommitted", correlation, "run-started",
			map[string]any{
				"work_item_id": "work-1", "run_id": "run-1",
				"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
				"claim_generation": 1, "runtime_instance_id": "runtime-1",
				"agent_instance_id": "agent-1", "status": "succeeded", "reason": "",
				"runtime_status_stream_id": "runtime_instance:runtime-1",
				"runtime_status_sequence":  1,
				"runtime_status_event_id":  "runtime-discovery",
			},
		),
	}
}

func TestRunAuthorityProjectionRebuildFailureIsolation(t *testing.T) { // s3_w2_projection_rebuild_failure_isolation
	t.Parallel()

	t.Run("lexical stream order does not control semantic dependencies", func(t *testing.T) {
		events := validRunAuthorityProjectionEvents()
		snapshot, err := replay(context.Background(), events)
		if err != nil {
			t.Fatalf("replay() error = %v", err)
		}
		workItem := snapshot.WorkItems["work-1"]
		run := snapshot.Runs["run-1"]
		if workItem.ID != "work-1" || workItem.Status != "ready_for_review" ||
			workItem.RunID != "run-1" || workItem.AgentInstanceID != "agent-1" {
			t.Fatalf("projected work item = %#v", workItem)
		}
		if run.ID != "run-1" || run.WorkItemID != "work-1" ||
			run.Phase != "terminal" || run.ClaimGeneration != 1 ||
			run.RuntimeInstanceID != "runtime-1" ||
			run.TerminalStatus != "succeeded" || run.TerminalReason != "" {
			t.Fatalf("projected run = %#v", run)
		}
	})

	t.Run("cross stream facts must pair exactly", func(t *testing.T) {
		valid := validRunAuthorityProjectionEvents()
		cases := []struct {
			name   string
			mutate func([]journal.Event) []journal.Event
		}{
			{"missing reservation", func(events []journal.Event) []journal.Event {
				return append(events[:1], events[3:]...)
			}},
			{"orphan release", func(events []journal.Event) []journal.Event {
				events[1].PayloadJSON = projectionPayload(t, map[string]any{
					"work_item_id": "work-1", "run_id": "other-run",
					"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
					"claim_generation": 1, "runtime_instance_id": "runtime-1",
					"agent_instance_id":        "agent-1",
					"runtime_status_stream_id": "runtime_instance:runtime-1",
					"runtime_status_sequence":  1,
					"runtime_status_event_id":  "runtime-discovery",
				})
				return events
			}},
			{"terminal generation mismatch", func(events []journal.Event) []journal.Event {
				events[len(events)-1].PayloadJSON = projectionPayload(t, map[string]any{
					"work_item_id": "work-1", "run_id": "run-1",
					"claim_id":         "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
					"claim_generation": 2, "runtime_instance_id": "runtime-1",
					"agent_instance_id": "agent-1", "status": "succeeded", "reason": "",
					"runtime_status_stream_id": "runtime_instance:runtime-1",
					"runtime_status_sequence":  1,
					"runtime_status_event_id":  "runtime-discovery",
				})
				return events
			}},
			{"noncanonical claim identity", func(events []journal.Event) []journal.Event {
				var payload map[string]any
				if err := json.Unmarshal(events[6].PayloadJSON, &payload); err != nil {
					t.Fatal(err)
				}
				payload["claim_id"] = "not-a-canonical-uuid"
				events[6].PayloadJSON = projectionPayload(t, payload)
				return events
			}},
			{"outcome causation mismatch", func(events []journal.Event) []journal.Event {
				events[5].CausationID = "wrong-terminal"
				return events
			}},
		}
		for _, testCase := range cases {
			events := cloneProjectionEvents(valid)
			events = testCase.mutate(events)
			if _, err := replay(context.Background(), events); !errors.Is(err, ErrInvalidProjectionEvent) {
				t.Errorf("%s error = %v, want ErrInvalidProjectionEvent", testCase.name, err)
			}
		}
	})

	t.Run("persisted runtime status heads control replay ordering", func(t *testing.T) {
		t.Run("offline before referenced claim rejects", func(t *testing.T) {
			events := cloneProjectionEvents(validRunAuthorityProjectionEvents())
			events[0].PayloadJSON = projectionPayload(t, map[string]any{
				"discovery_digest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				"source_probe_id":  "probe-1",
				"instance": map[string]any{
					"id": "runtime-1", "device_id": "device-1",
					"adapter_type": "pi-cli", "display_name": "Fixture",
					"executable_version": "1.0.0", "status": "offline",
					"observed_capabilities": []string{"models"}, "capacity": 1,
				},
				"model_ids": []string{},
			})
			if _, err := replay(context.Background(), events); !errors.Is(
				err, ErrInvalidProjectionEvent,
			) {
				t.Fatalf("offline-before replay error = %v", err)
			}
		})

		t.Run("offline after referenced online head remains projectable", func(t *testing.T) {
			events := append(
				validRunAuthorityProjectionEvents(),
				runAuthorityStatusEvent("offline"),
			)
			snapshot, err := replay(context.Background(), events)
			if err != nil {
				t.Fatalf("offline-after replay error = %v", err)
			}
			if snapshot.Runs["run-1"].TerminalStatus != "succeeded" ||
				snapshot.RuntimeInstances["runtime-1"].Status != "offline" {
				t.Fatalf("offline-after snapshot = %#v", snapshot)
			}
		})

		t.Run("paired facts reject different status references", func(t *testing.T) {
			events := cloneProjectionEvents(validRunAuthorityProjectionEvents())
			var payload map[string]any
			if err := json.Unmarshal(events[1].PayloadJSON, &payload); err != nil {
				t.Fatal(err)
			}
			payload["runtime_status_event_id"] = "fabricated"
			events[1].PayloadJSON = projectionPayload(t, payload)
			if _, err := replay(context.Background(), events); !errors.Is(
				err, ErrInvalidProjectionEvent,
			) {
				t.Fatalf("paired-reference replay error = %v", err)
			}
		})

		t.Run("fabricated status sequence and event reject", func(t *testing.T) {
			events := cloneProjectionEvents(validRunAuthorityProjectionEvents())
			var payload map[string]any
			if err := json.Unmarshal(events[6].PayloadJSON, &payload); err != nil {
				t.Fatal(err)
			}
			payload["runtime_status_sequence"] = float64(99)
			payload["runtime_status_event_id"] = "fabricated"
			events[6].PayloadJSON = projectionPayload(t, payload)
			if _, err := replay(context.Background(), events); !errors.Is(
				err, ErrInvalidProjectionEvent,
			) {
				t.Fatalf("fabricated-reference replay error = %v", err)
			}
		})

		t.Run("terminal cleanup may reference current offline head", func(t *testing.T) {
			events := cloneProjectionEvents(validRunAuthorityProjectionEvents())
			events = append(events, runAuthorityStatusEvent("offline"))
			for _, index := range []int{2, 8} {
				var payload map[string]any
				if err := json.Unmarshal(events[index].PayloadJSON, &payload); err != nil {
					t.Fatal(err)
				}
				payload["runtime_status_sequence"] = float64(2)
				payload["runtime_status_event_id"] = "runtime-status-offline"
				events[index].PayloadJSON = projectionPayload(t, payload)
			}
			snapshot, err := replay(context.Background(), events)
			if err != nil {
				t.Fatalf("offline terminal replay error = %v", err)
			}
			if snapshot.Runs["run-1"].TerminalStatus != "succeeded" ||
				snapshot.RuntimeInstances["runtime-1"].Status != "offline" {
				t.Fatalf("offline terminal snapshot = %#v", snapshot)
			}
		})

		t.Run("later capacity increase cannot authorize an earlier over-reservation", func(t *testing.T) {
			valid := validRunAuthorityProjectionEvents()
			events := []journal.Event{
				valid[0], valid[1], valid[3], valid[4], valid[6],
			}
			const correlation = "11111111-1111-4111-8111-111111111111"
			const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			lease := runProjectionTime.Add(time.Minute).Format(time.RFC3339Nano)
			events = append(events,
				runAuthorityEvent(
					"runtime-rediscovery", "runtime_instance:runtime-1", 2,
					"RuntimeInstanceDiscovered",
					"44444444-4444-4444-8444-444444444444", "",
					map[string]any{
						"discovery_digest": digest,
						"source_probe_id":  "probe-2",
						"instance": map[string]any{
							"id": "runtime-1", "device_id": "device-1",
							"adapter_type": "pi-cli", "display_name": "Fixture",
							"executable_version": "1.0.0", "status": "online",
							"observed_capabilities": []string{"models"}, "capacity": 2,
						},
						"model_ids": []string{},
					},
				),
				runAuthorityEvent(
					"work-2-created", "work-item/work-2", 1,
					"WorkItemCreated", correlation, "",
					map[string]any{
						"work_item_id": "work-2", "title": "Implement work-2",
						"status": "ready",
					},
				),
				runAuthorityEvent(
					"work-2-assigned", "work-item/work-2", 2,
					"WorkItemAssigned", correlation, "work-2-created",
					map[string]any{
						"work_item_id": "work-2", "run_id": "run-2",
						"agent_instance_id": "agent-2", "status": "assigned",
					},
				),
				runAuthorityEvent(
					"run-2-claimed", "run/run-2", 1,
					"RunClaimed", correlation, "work-2-assigned",
					map[string]any{
						"work_item_id": "work-2", "run_id": "run-2",
						"claim_id":         "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
						"claim_generation": 1, "runtime_instance_id": "runtime-1",
						"agent_instance_id":        "agent-2",
						"prepare_lease_expires_at": lease,
						"runtime_status_stream_id": "runtime_instance:runtime-1",
						"runtime_status_sequence":  1,
						"runtime_status_event_id":  "runtime-discovery",
					},
				),
				runAuthorityEvent(
					"runtime-2-reserve", "runtime_capacity:runtime-1", 2,
					"RuntimeCapacityReserved", correlation, "run-2-claimed",
					map[string]any{
						"work_item_id": "work-2", "run_id": "run-2",
						"claim_id":         "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
						"claim_generation": 1, "runtime_instance_id": "runtime-1",
						"agent_instance_id":        "agent-2",
						"runtime_status_stream_id": "runtime_instance:runtime-1",
						"runtime_status_sequence":  1,
						"runtime_status_event_id":  "runtime-discovery",
					},
				),
			)
			if _, err := replay(context.Background(), events); !errors.Is(
				err, ErrInvalidProjectionEvent,
			) {
				t.Fatalf("historical over-capacity replay error = %v", err)
			}
		})
	})

	t.Run("failed rebuild preserves previous published snapshot", func(t *testing.T) {
		source := &mutableRunAuthoritySource{events: validRunAuthorityProjectionEvents()}
		projection := &Projection{
			source:      source,
			snapshot:    emptySnapshot(),
			rebuildGate: make(chan struct{}, 1),
		}
		if err := projection.Rebuild(context.Background()); err != nil {
			t.Fatal(err)
		}
		before := projection.Snapshot()
		malformed := cloneProjectionEvents(validRunAuthorityProjectionEvents())
		malformed[1].PayloadJSON = []byte(`{"orphan":true}`)
		source.Set(malformed)
		if err := projection.Rebuild(context.Background()); !errors.Is(err, ErrInvalidProjectionEvent) {
			t.Fatalf("malformed rebuild error = %v", err)
		}
		after := projection.Snapshot()
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("failed rebuild changed snapshot:\nbefore=%#v\nafter=%#v", before, after)
		}
	})

	t.Run("snapshot clone isolates run maps", func(t *testing.T) {
		snapshot, err := replay(context.Background(), validRunAuthorityProjectionEvents())
		if err != nil {
			t.Fatal(err)
		}
		cloned := snapshot.clone()
		run := cloned.Runs["run-1"]
		run.TerminalStatus = "mutated"
		cloned.Runs["run-1"] = run
		workItem := cloned.WorkItems["work-1"]
		workItem.RunID = "mutated"
		cloned.WorkItems["work-1"] = workItem
		if snapshot.Runs["run-1"].TerminalStatus != "succeeded" ||
			snapshot.WorkItems["work-1"].RunID != "run-1" {
			t.Fatal("clone aliases run/work maps")
		}
	})
}

func TestRunAuthorityProjectionRealJournalRestart(t *testing.T) {
	ctx := context.Background()
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	if _, err := store.AppendBatch(
		ctx, validRunAuthorityProjectionEvents(),
	); err != nil {
		t.Fatalf("AppendBatch() error = %v", err)
	}
	first := New(db)
	if err := first.Rebuild(ctx); err != nil {
		t.Fatalf("first Rebuild() error = %v", err)
	}
	before := first.Snapshot()

	var sequence int
	var name string
	var databasePath string
	if err := db.QueryRowContext(ctx, "PRAGMA database_list").Scan(
		&sequence, &name, &databasePath,
	); err != nil {
		t.Fatalf("database path query: %v", err)
	}
	if databasePath == "" {
		t.Fatal("database path is empty")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	reopened, err := sql.Open(
		"sqlite", fmt.Sprintf("file:%s?%s", databasePath, values.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	second := New(reopened)
	if err := second.Rebuild(ctx); err != nil {
		t.Fatalf("reopened Rebuild() error = %v", err)
	}
	if !reflect.DeepEqual(second.Snapshot(), before) {
		t.Fatalf(
			"reopened snapshot mismatch:\nbefore=%#v\nafter=%#v",
			before, second.Snapshot(),
		)
	}
}

func projectionPayload(t testing.TB, payload map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func cloneProjectionEvents(events []journal.Event) []journal.Event {
	cloned := make([]journal.Event, len(events))
	for index, event := range events {
		cloned[index] = event
		cloned[index].PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	}
	return cloned
}
