package projection

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

func TestRebuildRuntimeStatusFactsFromJournal(t *testing.T) {
	ctx := context.Background()
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	source, discoveryInput := runtimeProjectionSource(t, "1.0.0", 2)
	discoveryCommit, err := state.CommitRuntimeDiscoverySnapshot(
		ctx, store, source, discoveryInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	statusCandidate := runtimeStatusProjectionCandidate(
		t, source, discoveryCommit.Events(), loomruntime.RuntimeOffline,
	)
	statusInput := runtimeStatusProjectionInput(statusCandidate)
	statusCommit, err := state.CommitRuntimeStatusTransitions(
		ctx, store, statusCandidate, statusInput,
	)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal("fresh rebuilds differ")
	}

	snapshot := first.Snapshot()
	transitions := statusCandidate.Transitions()
	statusEvents := statusCommit.Events()
	discoveryEvents := discoveryCommit.Events()
	for index, transition := range transitions {
		record := snapshot.RuntimeInstances[transition.RuntimeInstanceID]
		observation := source.Observations()[index]
		discoveryEvent := runtimeProjectionEventByInstance(
			discoveryEvents, transition.RuntimeInstanceID,
		)
		statusEvent := runtimeProjectionEventByInstance(
			statusEvents, transition.RuntimeInstanceID,
		)
		if record.ID != observation.Instance.ID ||
			record.DeviceID != observation.Instance.DeviceID ||
			record.AdapterType != observation.Instance.AdapterType ||
			record.DisplayName != observation.Instance.DisplayName ||
			record.ExecutableVersion != observation.Instance.ExecutableVersion ||
			record.Status != string(transition.ToStatus) ||
			!reflect.DeepEqual(
				record.ObservedCapabilities,
				observation.Instance.ObservedCapabilities,
			) ||
			record.Capacity != observation.Instance.Capacity ||
			!reflect.DeepEqual(record.ModelIDs, observation.ModelIDs) ||
			record.DiscoveryDigest != source.Digest() ||
			record.SourceProbeID != observation.SourceProbeID ||
			record.DiscoveryID != discoveryInput.DiscoveryID ||
			record.DiscoveryEventID != discoveryEvent.ID ||
			record.DiscoverySequence != discoveryEvent.Seq ||
			!record.DiscoveredAt.Equal(discoveryEvent.EmittedAt) ||
			record.StatusReconciliationID != statusInput.ReconciliationID ||
			record.StatusReconciliationDigest != statusCandidate.CandidateDigest() ||
			record.StatusBaselineDigest != statusCandidate.BaselineDigest() ||
			record.StatusDiscoveryDigest != statusCandidate.SourceDiscoveryDigest() ||
			record.StatusSourceProbeID != transition.SourceProbeID ||
			!record.StatusChangedAt.Equal(statusEvent.EmittedAt) ||
			record.StatusChangedAt.Location() != time.UTC ||
			record.StatusEventID != statusEvent.ID ||
			record.StatusSequence != statusEvent.Seq ||
			record.StatusPreviousEventID != transition.PreviousEventID ||
			record.StatusPreviousSequence != transition.PreviousSequence {
			t.Fatalf("Runtime record[%d] = %#v", index, record)
		}
	}

	returned := first.Snapshot()
	returned.RuntimeInstances["runtime.a"].ObservedCapabilities[0] = "changed"
	returned.RuntimeInstances["runtime.a"].ModelIDs[0] = "changed"
	mutated := returned.RuntimeInstances["runtime.a"]
	mutated.Status = "changed"
	mutated.StatusEventID = "changed"
	returned.RuntimeInstances["runtime.a"] = mutated
	if reflect.DeepEqual(returned, first.Snapshot()) {
		t.Fatal("Snapshot accessor leaked mutable state")
	}
}

func TestRebuildRuntimeStatusFactsConsecutiveAndRediscovery(t *testing.T) {
	discoveryEvents, statusEvents := runtimeStatusProjectionEvents(
		t, 1, loomruntime.RuntimeOffline,
	)
	firstStatus := statusEvents[0]
	secondStatus := runtimeStatusProjectionNextEvent(
		t,
		firstStatus,
		loomruntime.RuntimeOffline,
		loomruntime.RuntimeOnline,
	)
	projection := newForTestSource(eventSliceSource{events: []journal.Event{
		secondStatus,
		firstStatus,
		discoveryEvents[0],
	}})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("consecutive Rebuild() error = %v", err)
	}
	record := projection.Snapshot().RuntimeInstances["runtime.a"]
	if record.Status != string(loomruntime.RuntimeOnline) ||
		record.StatusEventID != secondStatus.ID ||
		record.StatusSequence != secondStatus.Seq ||
		record.StatusPreviousEventID != firstStatus.ID ||
		record.StatusPreviousSequence != firstStatus.Seq {
		t.Fatalf("consecutive record = %#v", record)
	}

	rediscovery := cloneRuntimeProjectionEvent(discoveryEvents[0])
	rediscovery.ID = "event.runtime.a.rediscovered"
	rediscovery.Seq = secondStatus.Seq + 1
	rediscovery.IdempotencyKey = "key.runtime.a.rediscovered"
	rediscovery.CorrelationID = "discovery.rediscovered"
	rediscovery.EmittedAt = secondStatus.EmittedAt.Add(time.Minute)
	rediscovery.PayloadJSON = mutateRuntimeProjectionPayload(
		t, rediscovery.PayloadJSON, func(payload map[string]any) {
			payload["discovery_digest"] = digestB
			payload["source_probe_id"] = "probe.rediscovered"
			instance := payload["instance"].(map[string]any)
			instance["status"] = string(loomruntime.RuntimeDisabled)
			instance["display_name"] = "Runtime a rediscovered"
		},
	)
	projection = newForTestSource(eventSliceSource{events: []journal.Event{
		rediscovery,
		secondStatus,
		discoveryEvents[0],
		firstStatus,
	}})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("rediscovery Rebuild() error = %v", err)
	}
	record = projection.Snapshot().RuntimeInstances["runtime.a"]
	if record.Status != string(loomruntime.RuntimeDisabled) ||
		record.DisplayName != "Runtime a rediscovered" ||
		record.DiscoveryEventID != rediscovery.ID ||
		record.DiscoverySequence != rediscovery.Seq ||
		record.DiscoveryDigest != digestB ||
		record.StatusReconciliationID != "" ||
		record.StatusReconciliationDigest != "" ||
		record.StatusBaselineDigest != "" ||
		record.StatusDiscoveryDigest != "" ||
		record.StatusSourceProbeID != "" ||
		!record.StatusChangedAt.IsZero() ||
		record.StatusEventID != "" ||
		record.StatusSequence != 0 ||
		record.StatusPreviousEventID != "" ||
		record.StatusPreviousSequence != 0 {
		t.Fatalf("rediscovered record = %#v", record)
	}
}

func TestRebuildRuntimeStatusFactsRejectsSemanticDrift(t *testing.T) {
	discoveryEvents, statusEvents := runtimeStatusProjectionEvents(
		t, 1, loomruntime.RuntimeOffline,
	)
	discovery := discoveryEvents[0]
	status := statusEvents[0]
	baseProjection := newForTestSource(eventSliceSource{events: []journal.Event{
		discovery,
		status,
	}})
	if err := baseProjection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	existing := baseProjection.Snapshot().RuntimeInstances["runtime.a"]
	validNext := runtimeStatusProjectionNextEvent(
		t, status, loomruntime.RuntimeOffline, loomruntime.RuntimeOnline,
	)

	tests := []struct {
		name   string
		change func(journal.Event) journal.Event
	}{
		{"event type", func(event journal.Event) journal.Event {
			event.Type = "RuntimeStatusChangedOther"
			return event
		}},
		{"unknown runtime", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["runtime_instance_id"] = "runtime.unknown"
			})
		}},
		{"device drift", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["device_id"] = "device.other"
			})
		}},
		{"adapter drift", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["adapter_type"] = "other-adapter"
			})
		}},
		{"from mismatch", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["from_status"] = string(loomruntime.RuntimeDisabled)
			})
		}},
		{"equal status", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["to_status"] = string(loomruntime.RuntimeOffline)
			})
		}},
		{"unknown from status", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["from_status"] = "unknown"
			})
		}},
		{"unknown to status", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["to_status"] = "unknown"
			})
		}},
		{"stale previous id", func(event journal.Event) journal.Event {
			event.CausationID = discovery.ID
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["previous_event_id"] = discovery.ID
			})
		}},
		{"stale previous sequence", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["previous_sequence"] = float64(discovery.Seq)
			})
		}},
		{"future previous sequence", func(event journal.Event) journal.Event {
			event.Seq++
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["previous_sequence"] = float64(status.Seq + 1)
			})
		}},
		{"causation mismatch", func(event journal.Event) journal.Event {
			event.CausationID = discovery.ID
			return event
		}},
		{"lower sequence", func(event journal.Event) journal.Event {
			event.Seq = 1
			return event
		}},
		{"equal sequence", func(event journal.Event) journal.Event {
			event.Seq--
			return event
		}},
		{"higher sequence", func(event journal.Event) journal.Event {
			event.Seq++
			return event
		}},
		{"sequence overflow", func(event journal.Event) journal.Event {
			event.PayloadJSON = bytes.Replace(
				event.PayloadJSON,
				[]byte(`"previous_sequence":2`),
				[]byte(`"previous_sequence":9223372036854775807`),
				1,
			)
			return event
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := projectRuntimeStatusEvent(test.change(validNext), existing)
			if !errors.Is(err, ErrInvalidProjectionEvent) {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(got, RuntimeInstance{}) {
				t.Fatalf("record = %#v, want zero", got)
			}
		})
	}

	beforeDiscovery, err := projectRuntimeStatusEvent(status, RuntimeInstance{})
	if !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("status-before-discovery error = %v", err)
	}
	if !reflect.DeepEqual(beforeDiscovery, RuntimeInstance{}) {
		t.Fatalf("status-before-discovery record = %#v", beforeDiscovery)
	}
}

func TestRebuildRuntimeStatusFactsRejectsInvalidEnvelopeAndPayload(t *testing.T) {
	discoveryEvents, statusEvents := runtimeStatusProjectionEvents(
		t, 1, loomruntime.RuntimeOffline,
	)
	discovery := discoveryEvents[0]
	valid := statusEvents[0]

	tests := []struct {
		name   string
		change func(journal.Event) journal.Event
	}{
		{"empty id", func(event journal.Event) journal.Event { event.ID = ""; return event }},
		{"empty key", func(event journal.Event) journal.Event { event.IdempotencyKey = ""; return event }},
		{"empty correlation", func(event journal.Event) journal.Event { event.CorrelationID = ""; return event }},
		{"empty causation", func(event journal.Event) journal.Event { event.CausationID = ""; return event }},
		{"zero sequence", func(event journal.Event) journal.Event { event.Seq = 0; return event }},
		{"zero time", func(event journal.Event) journal.Event { event.EmittedAt = time.Time{}; return event }},
		{"non UTC", func(event journal.Event) journal.Event {
			event.EmittedAt = event.EmittedAt.In(time.FixedZone("changed", 60*60))
			return event
		}},
		{"schema", func(event journal.Event) journal.Event { event.SchemaVersion = 2; return event }},
		{"stream", func(event journal.Event) journal.Event { event.StreamID += ".changed"; return event }},
		{"malformed", func(event journal.Event) journal.Event {
			event.PayloadJSON = []byte(`{"reconciliation_digest":`)
			return event
		}},
		{"unknown field", func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["unknown"] = "value"
			})
		}},
		{"duplicate field", func(event journal.Event) journal.Event {
			event.PayloadJSON = append(
				bytes.TrimSuffix(event.PayloadJSON, []byte("}")),
				[]byte(`,"to_status":"disabled"}`)...,
			)
			return event
		}},
	}
	for _, field := range []string{
		"reconciliation_digest",
		"baseline_digest",
		"source_discovery_digest",
		"source_probe_id",
		"runtime_instance_id",
		"device_id",
		"adapter_type",
		"from_status",
		"to_status",
		"previous_event_id",
		"previous_sequence",
	} {
		field := field
		tests = append(tests,
			struct {
				name   string
				change func(journal.Event) journal.Event
			}{
				name: "missing " + field,
				change: func(event journal.Event) journal.Event {
					return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
						delete(payload, field)
					})
				},
			},
			struct {
				name   string
				change func(journal.Event) journal.Event
			}{
				name: "null " + field,
				change: func(event journal.Event) journal.Event {
					return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
						payload[field] = nil
					})
				},
			},
		)
	}
	for _, field := range []string{
		"reconciliation_digest", "baseline_digest", "source_discovery_digest",
	} {
		field := field
		tests = append(tests, struct {
			name   string
			change func(journal.Event) journal.Event
		}{
			name: "invalid " + field,
			change: func(event journal.Event) journal.Event {
				return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
					payload[field] = strings.Repeat("G", 64)
				})
			},
		})
	}
	tests = append(tests, struct {
		name   string
		change func(journal.Event) journal.Event
	}{
		name: "empty source probe",
		change: func(event journal.Event) journal.Event {
			return mutateRuntimeStatusProjectionEvent(t, event, func(payload map[string]any) {
				payload["source_probe_id"] = ""
			})
		},
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := &mutableSource{}
			source.set([]journal.Event{discovery})
			projection := newForTestSource(source)
			if err := projection.Rebuild(context.Background()); err != nil {
				t.Fatal(err)
			}
			before := projection.Snapshot()
			source.set([]journal.Event{discovery, test.change(valid)})
			err := projection.Rebuild(context.Background())
			if err == nil {
				t.Fatal("Rebuild() error = nil")
			}
			if !reflect.DeepEqual(projection.Snapshot(), before) {
				t.Fatal("failed rebuild swapped Snapshot")
			}
		})
	}
}

func TestRebuildRuntimeStatusFactsOrderDuplicateAbsenceAndMutationIsolation(t *testing.T) {
	discoveryEvents, statusEvents := runtimeStatusProjectionEvents(
		t, 2, loomruntime.RuntimeOffline,
	)
	events := append(
		cloneRuntimeProjectionEvents(statusEvents),
		cloneRuntimeProjectionEvents(discoveryEvents)...,
	)
	events = append(events, cloneRuntimeProjectionEvent(statusEvents[0]))
	projection := newForTestSource(eventSliceSource{events: events})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := projection.Snapshot()
	events[0].PayloadJSON[0] ^= 1
	events[0].ID = "changed"
	if !reflect.DeepEqual(projection.Snapshot(), before) {
		t.Fatal("source mutation changed Snapshot")
	}

	projection = newForTestSource(eventSliceSource{})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(projection.Snapshot().RuntimeInstances) != 0 {
		t.Fatal("empty source created Runtime")
	}

	source := &mutableSource{}
	source.set(discoveryEvents)
	projection = newForTestSource(source)
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	discovered := projection.Snapshot()
	source.set(nil)
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(discovered.RuntimeInstances) != 2 ||
		len(projection.Snapshot().RuntimeInstances) != 0 {
		t.Fatal("empty rebuild result is not a rebuildable empty projection")
	}
}

func TestRebuildRuntimeStatusFactsProductionBoundary(t *testing.T) {
	for _, name := range []string{"runtime_status.go", "runtime_discovery.go"} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(
			token.NewFileSet(), name, content, parser.ImportsOnly,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if strings.HasPrefix(path, "loom-pi-rebuild/") &&
				path != "loom-pi-rebuild/internal/journal" &&
				path != "loom-pi-rebuild/internal/runtime" {
				t.Fatalf("%s forbidden project import %q", name, path)
			}
			for _, forbidden := range []string{
				"internal/state",
				"internal/runtime/piadapter",
				"os/exec",
				"net",
				"path/filepath",
			} {
				if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
					t.Fatalf("%s forbidden import %q", name, path)
				}
			}
		}
		product := string(content)
		for _, forbidden := range []string{
			"AppendBatch(",
			"CommitRuntime",
			"DiscoverRuntime(",
			"ReconcileObservedRuntimeStatuses(",
			"database/sql",
			"exec.",
			"http.",
			"os.",
			"goroutine",
			"scheduler",
			"daemon",
			"credential",
			"AgentGrant",
			"RuntimeProfile",
		} {
			if strings.Contains(product, forbidden) {
				t.Fatalf("%s contains forbidden surface %q", name, forbidden)
			}
		}
	}
}

func runtimeStatusProjectionCandidate(
	t *testing.T,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	discoveryEvents []journal.Event,
	toStatus loomruntime.RuntimeStatus,
) loomruntime.RuntimeStatusReconciliationCandidate {
	t.Helper()
	observations := discovery.Observations()
	baseline := make([]loomruntime.RuntimeStatusBaseline, len(observations))
	currentObservations := make([]loomruntime.RuntimeObservation, len(observations))
	for index, observation := range observations {
		event := runtimeProjectionEventByInstance(
			discoveryEvents, observation.Instance.ID,
		)
		baseline[index] = loomruntime.RuntimeStatusBaseline{
			Instance:              observation.Instance,
			LastDiscoveryEventID:  event.ID,
			LastDiscoverySequence: event.Seq,
		}
		current := observation
		current.Instance.Status = toStatus
		currentObservations[index] = current
	}
	current, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{runtimeProjectionProbe{
			id:           "probe.status-projection",
			observations: currentObservations,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func runtimeStatusProjectionInput(
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) state.RuntimeStatusCommitInput {
	transitions := candidate.Transitions()
	events := make([]state.RuntimeStatusEventInput, len(transitions))
	for index, transition := range transitions {
		events[index] = state.RuntimeStatusEventInput{
			RuntimeInstanceID: transition.RuntimeInstanceID,
			EventID:           "event.status." + transition.RuntimeInstanceID,
			IdempotencyKey:    "key.status." + transition.RuntimeInstanceID,
			Seq:               transition.PreviousSequence + 1,
		}
	}
	return state.RuntimeStatusCommitInput{
		ReconciliationID: "reconciliation.status.one",
		EmittedAt: time.Date(
			2026, 7, 25, 21, 0, 0, 123,
			time.FixedZone("fixture", 8*60*60),
		),
		Events: events,
	}
}

func runtimeStatusProjectionEvents(
	t *testing.T,
	count int,
	toStatus loomruntime.RuntimeStatus,
) ([]journal.Event, []journal.Event) {
	t.Helper()
	discovery, discoveryInput := runtimeProjectionSource(t, "1.0.0", count)
	discoveryEvents := runtimeProjectionCommittedEvents(
		t, discovery, discoveryInput,
	)
	candidate := runtimeStatusProjectionCandidate(
		t, discovery, discoveryEvents, toStatus,
	)
	appender := &runtimeProjectionAppender{}
	commit, err := state.CommitRuntimeStatusTransitions(
		context.Background(),
		appender,
		candidate,
		runtimeStatusProjectionInput(candidate),
	)
	if err != nil {
		t.Fatal(err)
	}
	return discoveryEvents, commit.Events()
}

func runtimeStatusProjectionNextEvent(
	t *testing.T,
	previous journal.Event,
	fromStatus loomruntime.RuntimeStatus,
	toStatus loomruntime.RuntimeStatus,
) journal.Event {
	t.Helper()
	next := cloneRuntimeProjectionEvent(previous)
	next.ID += ".next"
	next.Seq++
	next.IdempotencyKey += ".next"
	next.CorrelationID += ".next"
	next.CausationID = previous.ID
	next.EmittedAt = previous.EmittedAt.Add(time.Minute)
	next.PayloadJSON = mutateRuntimeProjectionPayload(
		t, next.PayloadJSON, func(payload map[string]any) {
			payload["reconciliation_digest"] = digestB
			payload["baseline_digest"] = digestA
			payload["source_discovery_digest"] = digestB
			payload["from_status"] = string(fromStatus)
			payload["to_status"] = string(toStatus)
			payload["previous_event_id"] = previous.ID
			payload["previous_sequence"] = float64(previous.Seq)
		},
	)
	return next
}

func mutateRuntimeStatusProjectionEvent(
	t *testing.T,
	event journal.Event,
	mutate func(map[string]any),
) journal.Event {
	t.Helper()
	event.PayloadJSON = mutateRuntimeProjectionPayload(
		t, event.PayloadJSON, mutate,
	)
	return event
}
