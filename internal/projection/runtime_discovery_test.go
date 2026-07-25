package projection

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

func TestRebuildRuntimeDiscoveryFactsFromJournal(t *testing.T) {
	ctx := context.Background()
	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	source, input := runtimeProjectionSource(t, "", 2)

	committed, err := state.CommitRuntimeDiscoverySnapshot(ctx, store, source, input)
	if err != nil {
		t.Fatalf("CommitRuntimeDiscoverySnapshot() error = %v", err)
	}
	if !committed.Committed() || committed.EventCount() != 2 {
		t.Fatalf("committed Candidate = %#v", committed)
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
		t.Fatalf("recreated Runtime snapshot mismatch: first=%#v second=%#v", first.Snapshot(), second.Snapshot())
	}

	got := first.Snapshot()
	if got.RuntimeInstances == nil || len(got.RuntimeInstances) != 2 {
		t.Fatalf("RuntimeInstances = %#v", got.RuntimeInstances)
	}
	observations := source.Observations()
	for _, observation := range observations {
		record, ok := got.RuntimeInstances[observation.Instance.ID]
		if !ok {
			t.Fatalf("missing RuntimeInstance %q", observation.Instance.ID)
		}
		event := runtimeProjectionEventByInstance(
			committed.Events(),
			observation.Instance.ID,
		)
		assertRuntimeProjectionRecord(
			t,
			record,
			observation,
			source.Digest(),
			input.DiscoveryID,
			event,
		)
	}
	if got.RuntimeInstances["runtime.a"].ExecutableVersion != "" {
		t.Fatalf("empty executable version was not preserved: %#v", got.RuntimeInstances["runtime.a"])
	}
}

func TestRebuildRuntimeDiscoveryFactsOrderEmptyAndUnrelated(t *testing.T) {
	empty := newForTestSource(eventSliceSource{})
	if err := empty.Rebuild(context.Background()); err != nil {
		t.Fatalf("empty Rebuild() error = %v", err)
	}
	if got := empty.Snapshot().RuntimeInstances; got == nil || len(got) != 0 {
		t.Fatalf("empty RuntimeInstances = %#v", got)
	}

	unrelated := projectionEvent(
		"event.status",
		"runtime_heartbeat:runtime.a",
		1,
		"key.status",
		"RuntimeHeartbeatObserved",
		map[string]string{"heartbeat": "observed"},
	)
	unrelated.CorrelationID = "heartbeat.check"
	unrelated.CausationID = "event.discovery"
	ignored := newForTestSource(eventSliceSource{events: []journal.Event{unrelated}})
	if err := ignored.Rebuild(context.Background()); err != nil {
		t.Fatalf("unrelated Rebuild() error = %v", err)
	}
	if got := ignored.Snapshot().RuntimeInstances; got == nil || len(got) != 0 {
		t.Fatalf("status Event inferred Runtime state: %#v", got)
	}

	source, input := runtimeProjectionSource(t, "1.2.3", 2)
	events := runtimeProjectionCommittedEvents(t, source, input)
	reordered := []journal.Event{
		cloneRuntimeProjectionEvent(events[1]),
		cloneRuntimeProjectionEvent(events[0]),
		cloneRuntimeProjectionEvent(events[1]),
	}
	first := newForTestSource(eventSliceSource{events: events})
	second := newForTestSource(eventSliceSource{events: reordered})
	if err := first.Rebuild(context.Background()); err != nil {
		t.Fatalf("ordered Rebuild() error = %v", err)
	}
	if err := second.Rebuild(context.Background()); err != nil {
		t.Fatalf("reordered Rebuild() error = %v", err)
	}
	if !reflect.DeepEqual(first.Snapshot(), second.Snapshot()) {
		t.Fatalf("reordered Runtime snapshot mismatch: first=%#v second=%#v", first.Snapshot(), second.Snapshot())
	}

	alternate := cloneRuntimeProjectionEvent(events[0])
	alternate.ID = "event.alternate"
	alternate.IdempotencyKey = "key.alternate"
	alternate.CorrelationID = "discovery.alternate"
	alternateProjection := newForTestSource(eventSliceSource{events: []journal.Event{alternate}})
	if err := alternateProjection.Rebuild(context.Background()); err != nil {
		t.Fatalf("alternate caller metadata Rebuild() error = %v", err)
	}
	record := alternateProjection.Snapshot().RuntimeInstances["runtime.a"]
	if record.DiscoveryID != "discovery.alternate" ||
		record.DiscoveryEventID != "event.alternate" {
		t.Fatalf("alternate caller metadata not preserved: %#v", record)
	}
}

func TestRebuildRuntimeDiscoveryFactsRediscoveryAndIdentityDrift(t *testing.T) {
	source, input := runtimeProjectionSource(t, "1.0.0", 1)
	first := runtimeProjectionCommittedEvents(t, source, input)[0]
	second := cloneRuntimeProjectionEvent(first)
	second.ID = "event.runtime.a.rediscovered"
	second.Seq = 2
	second.IdempotencyKey = "key.runtime.a.rediscovered"
	second.CorrelationID = "discovery.two"
	second.EmittedAt = first.EmittedAt.Add(time.Minute)
	second.PayloadJSON = mutateRuntimeProjectionPayload(t, second.PayloadJSON, func(payload map[string]any) {
		payload["discovery_digest"] = digestB
		payload["source_probe_id"] = "probe.rediscovered"
		instance := payload["instance"].(map[string]any)
		instance["display_name"] = "Runtime A updated"
		instance["executable_version"] = "2.0.0"
		instance["status"] = "offline"
		instance["observed_capabilities"] = []any{"models", "sandbox", "version"}
		instance["capacity"] = float64(3)
		payload["model_ids"] = []any{"provider/model-a", "provider/model-b"}
	})

	projection := newForTestSource(eventSliceSource{events: []journal.Event{second, first}})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("rediscovery Rebuild() error = %v", err)
	}
	record := projection.Snapshot().RuntimeInstances["runtime.a"]
	if record.DisplayName != "Runtime A updated" ||
		record.ExecutableVersion != "2.0.0" ||
		record.Status != "offline" ||
		record.Capacity != 3 ||
		!reflect.DeepEqual(record.ObservedCapabilities, []string{"models", "sandbox", "version"}) ||
		!reflect.DeepEqual(record.ModelIDs, []string{"provider/model-a", "provider/model-b"}) ||
		record.DiscoveryDigest != digestB ||
		record.SourceProbeID != "probe.rediscovered" ||
		record.DiscoveryID != "discovery.two" ||
		record.DiscoveryEventID != second.ID ||
		record.DiscoverySequence != 2 ||
		!record.DiscoveredAt.Equal(second.EmittedAt) {
		t.Fatalf("rediscovered Runtime record = %#v", record)
	}

	for _, test := range []struct {
		name  string
		field string
		value string
	}{
		{name: "device identity", field: "device_id", value: "device.other"},
		{name: "adapter identity", field: "adapter_type", value: "other-adapter"},
	} {
		t.Run(test.name, func(t *testing.T) {
			drift := cloneRuntimeProjectionEvent(second)
			drift.PayloadJSON = mutateRuntimeProjectionPayload(t, drift.PayloadJSON, func(payload map[string]any) {
				payload["instance"].(map[string]any)[test.field] = test.value
			})
			source := &mutableSource{}
			source.set([]journal.Event{first})
			projection := newForTestSource(source)
			if err := projection.Rebuild(context.Background()); err != nil {
				t.Fatalf("initial Rebuild() error = %v", err)
			}
			before := projection.Snapshot()
			source.set([]journal.Event{first, drift})
			if err := projection.Rebuild(context.Background()); !errors.Is(err, ErrInvalidProjectionEvent) {
				t.Fatalf("identity drift error = %v", err)
			}
			if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
				t.Fatalf("identity drift swapped Snapshot: got=%#v before=%#v", got, before)
			}
		})
	}
}

func TestRebuildRuntimeDiscoveryFactsRejectsInvalidPayloadAndEnvelope(t *testing.T) {
	source, input := runtimeProjectionSource(t, "1.0.0", 1)
	valid := runtimeProjectionCommittedEvents(t, source, input)[0]

	tests := []struct {
		name   string
		change func(*journal.Event)
		want   error
	}{
		{name: "malformed payload", change: func(event *journal.Event) { event.PayloadJSON = []byte("{") }, want: ErrInvalidProjectionEvent},
		{name: "extra payload field", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				payload["extra"] = true
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "extra instance field", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				payload["instance"].(map[string]any)["extra"] = true
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "missing digest", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				delete(payload, "discovery_digest")
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "invalid digest", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				payload["discovery_digest"] = "ABC"
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "empty source probe", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				payload["source_probe_id"] = ""
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "missing executable version", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				delete(payload["instance"].(map[string]any), "executable_version")
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "missing capabilities", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				delete(payload["instance"].(map[string]any), "observed_capabilities")
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "missing model ids", change: func(event *journal.Event) {
			event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
				delete(payload, "model_ids")
			})
		}, want: ErrInvalidProjectionEvent},
		{name: "empty runtime id", change: runtimeProjectionInstanceMutation(t, "id", ""), want: ErrInvalidProjectionEvent},
		{name: "empty device id", change: runtimeProjectionInstanceMutation(t, "device_id", ""), want: ErrInvalidProjectionEvent},
		{name: "empty adapter type", change: runtimeProjectionInstanceMutation(t, "adapter_type", ""), want: ErrInvalidProjectionEvent},
		{name: "empty display name", change: runtimeProjectionInstanceMutation(t, "display_name", ""), want: ErrInvalidProjectionEvent},
		{name: "invalid status", change: runtimeProjectionInstanceMutation(t, "status", "busy"), want: ErrInvalidProjectionEvent},
		{name: "zero capacity", change: runtimeProjectionInstanceMutation(t, "capacity", float64(0)), want: ErrInvalidProjectionEvent},
		{name: "duplicate capabilities", change: runtimeProjectionInstanceMutation(t, "observed_capabilities", []any{"models", "models"}), want: ErrInvalidProjectionEvent},
		{name: "noncanonical capabilities", change: runtimeProjectionInstanceMutation(t, "observed_capabilities", []any{"version", "models"}), want: ErrInvalidProjectionEvent},
		{name: "empty model id", change: runtimeProjectionTopLevelMutation(t, "model_ids", []any{""}), want: ErrInvalidProjectionEvent},
		{name: "duplicate model ids", change: runtimeProjectionTopLevelMutation(t, "model_ids", []any{"model.a", "model.a"}), want: ErrInvalidProjectionEvent},
		{name: "noncanonical model ids", change: runtimeProjectionTopLevelMutation(t, "model_ids", []any{"model.b", "model.a"}), want: ErrInvalidProjectionEvent},
		{name: "wrong stream", change: func(event *journal.Event) { event.StreamID = "runtime_instance:other" }, want: ErrInvalidProjectionEvent},
		{name: "empty event id", change: func(event *journal.Event) { event.ID = "" }, want: ErrInvalidProjectionEvent},
		{name: "empty idempotency key", change: func(event *journal.Event) { event.IdempotencyKey = "" }, want: ErrInvalidProjectionEvent},
		{name: "empty correlation", change: func(event *journal.Event) { event.CorrelationID = "" }, want: ErrInvalidProjectionEvent},
		{name: "nonempty causation", change: func(event *journal.Event) { event.CausationID = "cause" }, want: ErrInvalidProjectionEvent},
		{name: "zero timestamp", change: func(event *journal.Event) { event.EmittedAt = time.Time{} }, want: ErrInvalidProjectionEvent},
		{name: "non utc timestamp", change: func(event *journal.Event) {
			event.EmittedAt = event.EmittedAt.In(time.FixedZone("non-utc", 3600))
		}, want: ErrInvalidProjectionEvent},
		{name: "zero sequence", change: func(event *journal.Event) { event.Seq = 0 }, want: ErrSequenceGap},
		{name: "unsupported schema", change: func(event *journal.Event) { event.SchemaVersion = 2 }, want: ErrUnsupportedEventVersion},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := cloneRuntimeProjectionEvent(valid)
			test.change(&event)
			projection := newForTestSource(eventSliceSource{events: []journal.Event{event}})
			if err := projection.Rebuild(context.Background()); !errors.Is(err, test.want) {
				t.Fatalf("Rebuild() error = %v, want %v", err, test.want)
			}
			got := projection.Snapshot()
			if got.RuntimeInstances == nil || len(got.RuntimeInstances) != 0 {
				t.Fatalf("invalid Event produced Runtime projection: %#v", got.RuntimeInstances)
			}
		})
	}
}

func TestRebuildRuntimeDiscoveryFactsFailurePreservesSnapshot(t *testing.T) {
	sourceSnapshot, input := runtimeProjectionSource(t, "1.0.0", 1)
	valid := runtimeProjectionCommittedEvents(t, sourceSnapshot, input)
	invalid := cloneRuntimeProjectionEvent(valid[0])
	invalid.PayloadJSON = []byte("{}")

	source := &mutableSource{}
	source.set(valid)
	projection := newForTestSource(source)
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("initial Rebuild() error = %v", err)
	}
	before := projection.Snapshot()
	source.set([]journal.Event{invalid})
	if err := projection.Rebuild(context.Background()); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("invalid Rebuild() error = %v", err)
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed Rebuild swapped Snapshot: got=%#v before=%#v", got, before)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := projection.Rebuild(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Rebuild() error = %v", err)
	}
	if got := projection.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatalf("canceled Rebuild swapped Snapshot: got=%#v before=%#v", got, before)
	}

	rediscovered := cloneRuntimeProjectionEvent(valid[0])
	rediscovered.ID = "event.runtime.a.latest"
	rediscovered.Seq = 2
	rediscovered.IdempotencyKey = "key.runtime.a.latest"
	rediscovered.CorrelationID = "discovery.latest"
	rediscovered.PayloadJSON = mutateRuntimeProjectionPayload(t, rediscovered.PayloadJSON, func(payload map[string]any) {
		payload["discovery_digest"] = digestA
		payload["instance"].(map[string]any)["display_name"] = "latest"
	})
	blocking := newBlockingSource()
	firstCall := blocking.setNext(valid)
	secondCall := blocking.setNext([]journal.Event{valid[0], rediscovered})
	concurrent := newForTestSource(blocking)
	firstErr := make(chan error, 1)
	secondErr := make(chan error, 1)
	go func() { firstErr <- concurrent.Rebuild(context.Background()) }()
	firstCall.waitStarted(t)
	go func() { secondErr <- concurrent.Rebuild(context.Background()) }()
	firstCall.release()
	if err := <-firstErr; err != nil {
		t.Fatalf("first concurrent Rebuild() error = %v", err)
	}
	secondCall.waitStarted(t)
	secondCall.release()
	if err := <-secondErr; err != nil {
		t.Fatalf("second concurrent Rebuild() error = %v", err)
	}
	if got := concurrent.Snapshot().RuntimeInstances["runtime.a"].DisplayName; got != "latest" {
		t.Fatalf("serialized Runtime projection display = %q", got)
	}

	db := openProjectionTestDB(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		context.Background(), store, sourceSnapshot, input,
	); err != nil {
		t.Fatalf("CommitRuntimeDiscoverySnapshot() error = %v", err)
	}
	fromDB := New(db)
	if err := fromDB.Rebuild(context.Background()); err != nil {
		t.Fatalf("database Rebuild() error = %v", err)
	}
	dbBefore := fromDB.Snapshot()
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := fromDB.Rebuild(context.Background()); err == nil {
		t.Fatal("closed Journal Rebuild() error = nil")
	}
	if got := fromDB.Snapshot(); !reflect.DeepEqual(got, dbBefore) {
		t.Fatalf("closed Journal swapped Snapshot: got=%#v before=%#v", got, dbBefore)
	}
}

func TestRebuildRuntimeDiscoveryFactsSnapshotMutationIsolation(t *testing.T) {
	source, input := runtimeProjectionSource(t, "1.0.0", 1)
	events := runtimeProjectionCommittedEvents(t, source, input)
	projection := newForTestSource(eventSliceSource{events: events})
	if err := projection.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	first := projection.Snapshot()
	record := first.RuntimeInstances["runtime.a"]
	record.DisplayName = "mutated"
	record.ObservedCapabilities[0] = "mutated"
	record.ModelIDs[0] = "mutated"
	first.RuntimeInstances["runtime.a"] = record
	delete(first.RuntimeInstances, "runtime.a")
	events[0].PayloadJSON[0] = '['

	got := projection.Snapshot()
	record = got.RuntimeInstances["runtime.a"]
	if record.DisplayName != "Runtime a" ||
		!reflect.DeepEqual(record.ObservedCapabilities, []string{"models", "version"}) ||
		!reflect.DeepEqual(record.ModelIDs, []string{"provider/model-a"}) {
		t.Fatalf("stored Runtime projection was mutated: %#v", record)
	}
}

func TestRebuildRuntimeDiscoveryFactsProductionBoundary(t *testing.T) {
	allowed := map[string]bool{
		`"loom-pi-rebuild/internal/journal"`: true,
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, name := range []string{"projection.go", "runtime_discovery.go"} {
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(
			token.NewFileSet(),
			name,
			content,
			parser.ImportsOnly,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			if strings.HasPrefix(imported.Path.Value, `"loom-pi-rebuild/`) &&
				!allowed[imported.Path.Value] {
				t.Fatalf("%s imports forbidden package %s", name, imported.Path.Value)
			}
		}
		for _, forbidden := range []string{
			"internal/state",
			"internal/runtime/piadapter",
			"database/sql",
			"os/exec",
			"net/",
			"credential",
			"scheduler",
			"daemon",
			"PRIVATE_PATH",
			"PRIVATE_SECRET",
		} {
			if name == "projection.go" && forbidden == "database/sql" {
				continue
			}
			if strings.Contains(string(content), forbidden) {
				t.Fatalf("%s contains forbidden production surface %q", name, forbidden)
			}
		}
	}
}

type runtimeProjectionProbe struct {
	id           string
	observations []loomruntime.RuntimeObservation
}

func (p runtimeProjectionProbe) ID() string {
	return p.id
}

func (p runtimeProjectionProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	result := make([]loomruntime.RuntimeObservation, len(p.observations))
	for index, observation := range p.observations {
		observation.Instance.ObservedCapabilities = append(
			[]string(nil),
			observation.Instance.ObservedCapabilities...,
		)
		observation.ModelIDs = append([]string(nil), observation.ModelIDs...)
		result[index] = observation
	}
	return result, nil
}

type runtimeProjectionAppender struct {
	mu     sync.Mutex
	events []journal.Event
}

func (a *runtimeProjectionAppender) AppendBatch(
	ctx context.Context,
	events []journal.Event,
) ([]journal.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = cloneRuntimeProjectionEvents(events)
	return cloneRuntimeProjectionEvents(events), nil
}

func runtimeProjectionSource(
	t *testing.T,
	version string,
	count int,
) (loomruntime.RuntimeDiscoverySnapshot, state.RuntimeDiscoveryCommitInput) {
	t.Helper()
	observations := make([]loomruntime.RuntimeObservation, count)
	for index := range observations {
		suffix := string(rune('a' + index))
		instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID:                   "runtime." + suffix,
			DeviceID:             "device." + suffix,
			AdapterType:          "pi-cli",
			DisplayName:          "Runtime " + suffix,
			ExecutableVersion:    version,
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"models", "version"},
			Capacity:             index + 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		observations[index] = loomruntime.RuntimeObservation{
			Instance: instance,
			ModelIDs: []string{"provider/model-" + suffix},
		}
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{runtimeProjectionProbe{
			id:           "probe.projection",
			observations: observations,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]state.RuntimeDiscoveryEventInput, count)
	for index, observation := range snapshot.Observations() {
		inputs[index] = state.RuntimeDiscoveryEventInput{
			RuntimeInstanceID: observation.Instance.ID,
			EventID:           "event." + observation.Instance.ID,
			IdempotencyKey:    "key." + observation.Instance.ID,
			Seq:               1,
		}
	}
	return snapshot, state.RuntimeDiscoveryCommitInput{
		DiscoveryID: "discovery.one",
		EmittedAt: time.Date(
			2026, 7, 25, 19, 0, 0, 123,
			time.FixedZone("fixture", 8*60*60),
		),
		Events: inputs,
	}
}

func runtimeProjectionCommittedEvents(
	t *testing.T,
	source loomruntime.RuntimeDiscoverySnapshot,
	input state.RuntimeDiscoveryCommitInput,
) []journal.Event {
	t.Helper()
	appender := &runtimeProjectionAppender{}
	candidate, err := state.CommitRuntimeDiscoverySnapshot(
		context.Background(),
		appender,
		source,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	return candidate.Events()
}

func runtimeProjectionEventByInstance(
	events []journal.Event,
	instanceID string,
) journal.Event {
	for _, event := range events {
		if event.StreamID == "runtime_instance:"+instanceID {
			return event
		}
	}
	return journal.Event{}
}

func assertRuntimeProjectionRecord(
	t *testing.T,
	got RuntimeInstance,
	observation loomruntime.RuntimeObservation,
	discoveryDigest string,
	discoveryID string,
	event journal.Event,
) {
	t.Helper()
	if got.ID != observation.Instance.ID ||
		got.DeviceID != observation.Instance.DeviceID ||
		got.AdapterType != observation.Instance.AdapterType ||
		got.DisplayName != observation.Instance.DisplayName ||
		got.ExecutableVersion != observation.Instance.ExecutableVersion ||
		got.Status != string(observation.Instance.Status) ||
		!reflect.DeepEqual(got.ObservedCapabilities, observation.Instance.ObservedCapabilities) ||
		got.Capacity != observation.Instance.Capacity ||
		!reflect.DeepEqual(got.ModelIDs, observation.ModelIDs) ||
		got.DiscoveryDigest != discoveryDigest ||
		got.SourceProbeID != observation.SourceProbeID ||
		got.DiscoveryID != discoveryID ||
		!got.DiscoveredAt.Equal(event.EmittedAt) ||
		got.DiscoveredAt.Location() != time.UTC ||
		got.DiscoveryEventID != event.ID ||
		got.DiscoverySequence != event.Seq {
		t.Fatalf("Runtime record = %#v, observation = %#v, event = %#v", got, observation, event)
	}
}

func runtimeProjectionInstanceMutation(
	t *testing.T,
	field string,
	value any,
) func(*journal.Event) {
	t.Helper()
	return func(event *journal.Event) {
		event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
			payload["instance"].(map[string]any)[field] = value
		})
	}
}

func runtimeProjectionTopLevelMutation(
	t *testing.T,
	field string,
	value any,
) func(*journal.Event) {
	t.Helper()
	return func(event *journal.Event) {
		event.PayloadJSON = mutateRuntimeProjectionPayload(t, event.PayloadJSON, func(payload map[string]any) {
			payload[field] = value
		})
	}
}

func mutateRuntimeProjectionPayload(
	t *testing.T,
	payload []byte,
	change func(map[string]any),
) []byte {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	change(value)
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func cloneRuntimeProjectionEvents(events []journal.Event) []journal.Event {
	result := make([]journal.Event, len(events))
	for index, event := range events {
		result[index] = cloneRuntimeProjectionEvent(event)
	}
	return result
}

func cloneRuntimeProjectionEvent(event journal.Event) journal.Event {
	event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	return event
}
