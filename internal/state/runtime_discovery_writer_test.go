package state

import (
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestCommitRuntimeDiscoverySnapshotBuildsCanonicalAtomicEvents(t *testing.T) {
	snapshot := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 2)
	input := runtimeDiscoveryWriterCommitInput(snapshot)
	input.Events[0], input.Events[1] = input.Events[1], input.Events[0]
	appender := &recordingEventBatchAppender{}

	candidate, err := CommitRuntimeDiscoverySnapshot(
		context.Background(), appender, snapshot, input,
	)
	if err != nil {
		t.Fatalf("CommitRuntimeDiscoverySnapshot() error = %v", err)
	}
	if !candidate.Committed() ||
		candidate.SourceDiscoveryDigest() != snapshot.Digest() ||
		candidate.EventCount() != 2 ||
		!isRuntimeDiscoveryWriterDigest(candidate.CommitDigest()) {
		t.Fatalf("candidate = %#v", candidate)
	}
	if appender.calls != 1 || len(appender.batches) != 1 || len(appender.batches[0]) != 2 {
		t.Fatalf("appender = %#v", appender)
	}

	events := candidate.Events()
	observations := snapshot.Observations()
	for index, event := range events {
		observation := observations[index]
		meta := runtimeDiscoveryEventInputByInstance(input.Events, observation.Instance.ID)
		if event.ID != meta.EventID ||
			event.StreamID != "runtime_instance:"+observation.Instance.ID ||
			event.Seq != meta.Seq ||
			event.IdempotencyKey != meta.IdempotencyKey ||
			event.Type != "RuntimeInstanceDiscovered" ||
			event.SchemaVersion != 1 ||
			!event.EmittedAt.Equal(input.EmittedAt.UTC()) ||
			event.EmittedAt.Location() != time.UTC ||
			event.CorrelationID != input.DiscoveryID ||
			event.CausationID != "" {
			t.Fatalf("event[%d] = %#v", index, event)
		}
		var payload runtimeDiscoveryWriterPayload
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.DiscoveryDigest != snapshot.Digest() ||
			payload.SourceProbeID != observation.SourceProbeID ||
			payload.Instance.ID != observation.Instance.ID ||
			payload.Instance.DeviceID != observation.Instance.DeviceID ||
			payload.Instance.AdapterType != observation.Instance.AdapterType ||
			payload.Instance.DisplayName != observation.Instance.DisplayName ||
			payload.Instance.ExecutableVersion != observation.Instance.ExecutableVersion ||
			payload.Instance.Status != observation.Instance.Status ||
			!reflect.DeepEqual(payload.Instance.ObservedCapabilities, observation.Instance.ObservedCapabilities) ||
			payload.Instance.Capacity != observation.Instance.Capacity ||
			!reflect.DeepEqual(payload.ModelIDs, observation.ModelIDs) {
			t.Fatalf("payload[%d] = %#v, observation = %#v", index, payload, observation)
		}
		for _, forbidden := range []string{"PRIVATE_PATH", "PRIVATE_SECRET", "stdout", "stderr"} {
			if strings.Contains(string(event.PayloadJSON), forbidden) {
				t.Fatalf("payload disclosed %q: %s", forbidden, event.PayloadJSON)
			}
		}
	}
	if !reflect.DeepEqual(events, appender.batches[0]) {
		t.Fatalf("candidate/appender events differ: %#v / %#v", events, appender.batches[0])
	}
}

func TestCommitRuntimeDiscoverySnapshotRejectsInvalidInputBeforeAppend(t *testing.T) {
	snapshot := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 2)
	valid := runtimeDiscoveryWriterCommitInput(snapshot)

	tests := []struct {
		name   string
		change func(*RuntimeDiscoveryCommitInput)
		want   error
	}{
		{name: "empty discovery id", change: func(i *RuntimeDiscoveryCommitInput) { i.DiscoveryID = "" }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "zero time", change: func(i *RuntimeDiscoveryCommitInput) { i.EmittedAt = time.Time{} }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "empty events", change: func(i *RuntimeDiscoveryCommitInput) { i.Events = nil }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "missing event", change: func(i *RuntimeDiscoveryCommitInput) { i.Events = i.Events[:1] }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "extra event", change: func(i *RuntimeDiscoveryCommitInput) { i.Events = append(i.Events, i.Events[0]) }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "empty runtime id", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[0].RuntimeInstanceID = "" }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "unknown runtime id", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[0].RuntimeInstanceID = "runtime.unknown" }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "duplicate runtime id", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[1].RuntimeInstanceID = i.Events[0].RuntimeInstanceID }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "empty event id", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[0].EventID = "" }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "duplicate event id", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[1].EventID = i.Events[0].EventID }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "empty key", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[0].IdempotencyKey = "" }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "duplicate key", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[1].IdempotencyKey = i.Events[0].IdempotencyKey }, want: ErrInvalidRuntimeDiscoveryCommitInput},
		{name: "zero sequence", change: func(i *RuntimeDiscoveryCommitInput) { i.Events[0].Seq = 0 }, want: ErrInvalidRuntimeDiscoveryCommitInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := cloneRuntimeDiscoveryWriterInput(valid)
			test.change(&input)
			appender := &recordingEventBatchAppender{}
			got, err := CommitRuntimeDiscoverySnapshot(
				context.Background(), appender, snapshot, input,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroRuntimeDiscoveryCommit(t, got)
			if appender.calls != 0 {
				t.Fatalf("AppendBatch calls = %d, want 0", appender.calls)
			}
		})
	}

	for _, appender := range []EventBatchAppender{nil, (*recordingEventBatchAppender)(nil)} {
		got, err := CommitRuntimeDiscoverySnapshot(context.Background(), appender, snapshot, valid)
		if !errors.Is(err, ErrInvalidRuntimeDiscoveryCommitInput) {
			t.Fatalf("nil appender error = %v", err)
		}
		assertZeroRuntimeDiscoveryCommit(t, got)
	}
	got, err := CommitRuntimeDiscoverySnapshot(nil, &recordingEventBatchAppender{}, snapshot, valid)
	if !errors.Is(err, ErrInvalidRuntimeDiscoveryCommitInput) {
		t.Fatalf("nil context error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	appender := &recordingEventBatchAppender{}
	got, err = CommitRuntimeDiscoverySnapshot(ctx, appender, snapshot, valid)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if appender.calls != 0 {
		t.Fatalf("canceled AppendBatch calls = %d", appender.calls)
	}

	deadlineCtx, deadlineCancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(-time.Second),
	)
	defer deadlineCancel()
	appender = &recordingEventBatchAppender{}
	got, err = CommitRuntimeDiscoverySnapshot(deadlineCtx, appender, snapshot, valid)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if appender.calls != 0 {
		t.Fatalf("deadline AppendBatch calls = %d", appender.calls)
	}
}

func TestCommitRuntimeDiscoverySnapshotRejectsEmptyAndOversizedSources(t *testing.T) {
	for _, snapshot := range []loomruntime.RuntimeDiscoverySnapshot{
		{},
		runtimeDiscoveryWriterSnapshot(t, "1.0.0", 0),
	} {
		appender := &recordingEventBatchAppender{}
		got, err := CommitRuntimeDiscoverySnapshot(
			context.Background(),
			appender,
			snapshot,
			RuntimeDiscoveryCommitInput{
				DiscoveryID: "discovery.empty",
				EmittedAt:   time.Now(),
			},
		)
		if !errors.Is(err, ErrEmptyRuntimeDiscoveryCommit) &&
			!errors.Is(err, ErrInvalidRuntimeDiscoveryCommitSource) {
			t.Fatalf("empty source error = %v", err)
		}
		assertZeroRuntimeDiscoveryCommit(t, got)
		if appender.calls != 0 {
			t.Fatalf("empty source AppendBatch calls = %d", appender.calls)
		}
	}

	oversized := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 33)
	appender := &recordingEventBatchAppender{}
	got, err := CommitRuntimeDiscoverySnapshot(
		context.Background(),
		appender,
		oversized,
		runtimeDiscoveryWriterCommitInput(oversized),
	)
	if !errors.Is(err, ErrInvalidRuntimeDiscoveryCommitSource) {
		t.Fatalf("oversized source error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if appender.calls != 0 {
		t.Fatalf("oversized source AppendBatch calls = %d", appender.calls)
	}
}

func TestCommitRuntimeDiscoverySnapshotRejectsAppenderFailureAndMismatchedResults(t *testing.T) {
	snapshot := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 2)
	input := runtimeDiscoveryWriterCommitInput(snapshot)
	appendErr := errors.New("append failed")
	got, err := CommitRuntimeDiscoverySnapshot(
		context.Background(),
		&recordingEventBatchAppender{err: appendErr},
		snapshot,
		input,
	)
	if !errors.Is(err, appendErr) {
		t.Fatalf("append error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)

	tests := []struct {
		name   string
		result func([]journal.Event) []journal.Event
	}{
		{name: "nil", result: func([]journal.Event) []journal.Event { return nil }},
		{name: "short", result: func(events []journal.Event) []journal.Event { return cloneStateEvents(events[:1]) }},
		{name: "long", result: func(events []journal.Event) []journal.Event {
			return append(cloneStateEvents(events), cloneStateEvent(events[0]))
		}},
		{name: "reordered", result: func(events []journal.Event) []journal.Event {
			return []journal.Event{cloneStateEvent(events[1]), cloneStateEvent(events[0])}
		}},
		{name: "mutated envelope", result: func(events []journal.Event) []journal.Event {
			result := cloneStateEvents(events)
			result[0].Seq++
			return result
		}},
		{name: "non utc emitted at location", result: func(events []journal.Event) []journal.Event {
			result := cloneStateEvents(events)
			result[0].EmittedAt = result[0].EmittedAt.In(time.FixedZone("non-utc", 3600))
			return result
		}},
		{name: "mutated payload", result: func(events []journal.Event) []journal.Event {
			result := cloneStateEvents(events)
			result[0].PayloadJSON[0] = '['
			return result
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appender := &recordingEventBatchAppender{result: test.result}
			got, err := CommitRuntimeDiscoverySnapshot(
				context.Background(), appender, snapshot, input,
			)
			if !errors.Is(err, ErrRuntimeDiscoveryCommitResultMismatch) {
				t.Fatalf("error = %v", err)
			}
			assertZeroRuntimeDiscoveryCommit(t, got)
			if appender.calls != 1 {
				t.Fatalf("AppendBatch calls = %d", appender.calls)
			}
		})
	}
}

func TestRuntimeDiscoveryCommitCandidateMutationAndDigestIsolation(t *testing.T) {
	snapshot := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 2)
	input := runtimeDiscoveryWriterCommitInput(snapshot)
	appender := &recordingEventBatchAppender{}
	first, err := CommitRuntimeDiscoverySnapshot(
		context.Background(), appender, snapshot, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	wantEvents := first.Events()
	wantDigest := first.CommitDigest()

	input.Events[0].EventID = "mutated"
	appender.batches[0][0].PayloadJSON[0] = '['
	accessor := first.Events()
	accessor[0].ID = "mutated"
	accessor[0].PayloadJSON[0] = '['
	observations := snapshot.Observations()
	observations[0].ModelIDs[0] = "mutated/model"
	if !reflect.DeepEqual(first.Events(), wantEvents) || first.CommitDigest() != wantDigest {
		t.Fatal("mutation changed Candidate")
	}

	equalInput := runtimeDiscoveryWriterCommitInput(snapshot)
	equal, err := CommitRuntimeDiscoverySnapshot(
		context.Background(), &recordingEventBatchAppender{}, snapshot, equalInput,
	)
	if err != nil || equal.CommitDigest() != wantDigest {
		t.Fatalf("equal commit = (%#v,%v)", equal, err)
	}

	changedSnapshot := runtimeDiscoveryWriterSnapshot(t, "2.0.0", 2)
	changedInput := runtimeDiscoveryWriterCommitInput(changedSnapshot)
	changed, err := CommitRuntimeDiscoverySnapshot(
		context.Background(), &recordingEventBatchAppender{}, changedSnapshot, changedInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed.CommitDigest() == wantDigest {
		t.Fatal("source/payload change did not change commit digest")
	}

	changedEnvelopeInput := runtimeDiscoveryWriterCommitInput(snapshot)
	changedEnvelopeInput.Events[0].EventID = "event.runtime.changed"
	changedEnvelope, err := CommitRuntimeDiscoverySnapshot(
		context.Background(),
		&recordingEventBatchAppender{},
		snapshot,
		changedEnvelopeInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if changedEnvelope.CommitDigest() == wantDigest {
		t.Fatal("envelope change did not change commit digest")
	}
}

func TestRuntimeDiscoveryCommitDigestCoversCompleteEvent(t *testing.T) {
	snapshot := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 2)
	baseline, err := CommitRuntimeDiscoverySnapshot(
		context.Background(),
		&recordingEventBatchAppender{},
		snapshot,
		runtimeDiscoveryWriterCommitInput(snapshot),
	)
	if err != nil {
		t.Fatal(err)
	}
	baselineDigest := baseline.CommitDigest()
	tests := []struct {
		name   string
		change func(*RuntimeDiscoveryCommitCandidate)
	}{
		{name: "source digest", change: func(c *RuntimeDiscoveryCommitCandidate) { c.sourceDiscoveryDigest = strings.Repeat("a", 64) }},
		{name: "event count", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events = c.events[:1] }},
		{name: "id", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].ID += ".changed" }},
		{name: "stream", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].StreamID += ".changed" }},
		{name: "sequence", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].Seq++ }},
		{name: "idempotency", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].IdempotencyKey += ".changed" }},
		{name: "type", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].Type += ".changed" }},
		{name: "schema", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].SchemaVersion++ }},
		{name: "time", change: func(c *RuntimeDiscoveryCommitCandidate) {
			c.events[0].EmittedAt = c.events[0].EmittedAt.Add(time.Nanosecond)
		}},
		{name: "correlation", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].CorrelationID += ".changed" }},
		{name: "causation", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].CausationID = "changed" }},
		{name: "payload", change: func(c *RuntimeDiscoveryCommitCandidate) { c.events[0].PayloadJSON[0] = '[' }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := cloneRuntimeDiscoveryWriterCandidate(baseline)
			test.change(&changed)
			digest, err := digestRuntimeDiscoveryCommitCandidate(changed)
			if err != nil {
				t.Fatal(err)
			}
			if digest == baselineDigest {
				t.Fatalf("%s did not change digest", test.name)
			}
		})
	}
}

func TestCommitRuntimeDiscoverySnapshotRealJournalRetryAndConflicts(t *testing.T) {
	snapshot := runtimeDiscoveryWriterSnapshot(t, "1.0.0", 2)
	input := runtimeDiscoveryWriterCommitInput(snapshot)
	store, db := openSavedTeamWriterStore(t)

	first, err := CommitRuntimeDiscoverySnapshot(context.Background(), store, snapshot, input)
	if err != nil {
		t.Fatal(err)
	}
	if got := savedTeamWriterRowCount(t, db); got != 2 {
		t.Fatalf("row count = %d", got)
	}
	retry, err := CommitRuntimeDiscoverySnapshot(context.Background(), store, snapshot, input)
	if err != nil || retry.CommitDigest() != first.CommitDigest() {
		t.Fatalf("retry = (%#v,%v)", retry, err)
	}
	if got := savedTeamWriterRowCount(t, db); got != 2 {
		t.Fatalf("retry row count = %d", got)
	}

	conflictInput := runtimeDiscoveryWriterCommitInput(snapshot)
	conflictInput.Events[0].EventID = "event.runtime.conflict"
	conflictInput.Events[0].IdempotencyKey = input.Events[0].IdempotencyKey
	got, err := CommitRuntimeDiscoverySnapshot(
		context.Background(), store, snapshot, conflictInput,
	)
	if !errors.Is(err, journal.ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if count := savedTeamWriterRowCount(t, db); count != 2 {
		t.Fatalf("conflict row count = %d", count)
	}

	sequenceInput := runtimeDiscoveryWriterCommitInput(snapshot)
	sequenceInput.Events[0].EventID = "event.runtime.sequence-conflict"
	sequenceInput.Events[0].IdempotencyKey = "key.runtime.sequence-conflict"
	got, err = CommitRuntimeDiscoverySnapshot(
		context.Background(), store, snapshot, sequenceInput,
	)
	if !errors.Is(err, journal.ErrSequenceConflict) {
		t.Fatalf("sequence conflict error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if count := savedTeamWriterRowCount(t, db); count != 2 {
		t.Fatalf("sequence conflict row count = %d", count)
	}

	partialStore, partialDB := openSavedTeamWriterStore(t)
	requestedAppender := &recordingEventBatchAppender{}
	if _, err := CommitRuntimeDiscoverySnapshot(
		context.Background(), requestedAppender, snapshot, input,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := partialStore.Append(context.Background(), requestedAppender.batches[0][0]); err != nil {
		t.Fatal(err)
	}
	got, err = CommitRuntimeDiscoverySnapshot(
		context.Background(), partialStore, snapshot, input,
	)
	if !errors.Is(err, journal.ErrPartialEventBatchConflict) {
		t.Fatalf("partial conflict error = %v", err)
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if count := savedTeamWriterRowCount(t, partialDB); count != 1 {
		t.Fatalf("partial row count = %d", count)
	}

	rollbackStore, rollbackDB := openSavedTeamWriterStore(t)
	if _, err := rollbackDB.ExecContext(context.Background(), `
		CREATE TRIGGER fail_runtime_discovery_second_insert
		BEFORE INSERT ON events
		WHEN NEW.id = 'event.runtime.b'
		BEGIN
			SELECT RAISE(ABORT, 'forced second insert failure');
		END
	`); err != nil {
		t.Fatal(err)
	}
	got, err = CommitRuntimeDiscoverySnapshot(
		context.Background(), rollbackStore, snapshot, input,
	)
	if err == nil {
		t.Fatal("forced second insert error = nil")
	}
	assertZeroRuntimeDiscoveryCommit(t, got)
	if count := savedTeamWriterRowCount(t, rollbackDB); count != 0 {
		t.Fatalf("rollback row count = %d", count)
	}
}

func TestRuntimeDiscoveryCommitProductionImportBoundary(t *testing.T) {
	path := filepath.Join(".", "runtime_discovery_writer.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		for _, forbidden := range []string{
			"database/sql", "net", "net/http", "os", "os/exec",
			"loom-pi-rebuild/internal/projection",
			"loom-pi-rebuild/internal/teams",
		} {
			if importPath == forbidden {
				t.Fatalf("runtime_discovery_writer.go imports forbidden %q", importPath)
			}
		}
	}
}

type runtimeDiscoveryWriterProbe struct {
	id           string
	observations []loomruntime.RuntimeObservation
}

func (p runtimeDiscoveryWriterProbe) ID() string { return p.id }

func (p runtimeDiscoveryWriterProbe) ObserveRuntime(context.Context) ([]loomruntime.RuntimeObservation, error) {
	result := make([]loomruntime.RuntimeObservation, len(p.observations))
	copy(result, p.observations)
	return result, nil
}

func runtimeDiscoveryWriterSnapshot(
	t *testing.T,
	version string,
	count int,
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	observations := make([]loomruntime.RuntimeObservation, count)
	for index := range observations {
		suffix := string(rune('a' + index))
		instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID:                   "runtime." + suffix,
			DeviceID:             "device.local",
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
		[]loomruntime.RuntimeProbe{runtimeDiscoveryWriterProbe{
			id:           "probe.writer",
			observations: observations,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func runtimeDiscoveryWriterCommitInput(
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) RuntimeDiscoveryCommitInput {
	observations := snapshot.Observations()
	events := make([]RuntimeDiscoveryEventInput, len(observations))
	for index, observation := range observations {
		events[index] = RuntimeDiscoveryEventInput{
			RuntimeInstanceID: observation.Instance.ID,
			EventID:           "event." + observation.Instance.ID,
			IdempotencyKey:    "key." + observation.Instance.ID,
			Seq:               1,
		}
	}
	return RuntimeDiscoveryCommitInput{
		DiscoveryID: "discovery.one",
		EmittedAt: time.Date(
			2026, 7, 25, 17, 30, 0, 123,
			time.FixedZone("fixture", 8*60*60),
		),
		Events: events,
	}
}

func cloneRuntimeDiscoveryWriterInput(input RuntimeDiscoveryCommitInput) RuntimeDiscoveryCommitInput {
	input.Events = append([]RuntimeDiscoveryEventInput(nil), input.Events...)
	return input
}

func runtimeDiscoveryEventInputByInstance(
	inputs []RuntimeDiscoveryEventInput,
	instanceID string,
) RuntimeDiscoveryEventInput {
	for _, input := range inputs {
		if input.RuntimeInstanceID == instanceID {
			return input
		}
	}
	return RuntimeDiscoveryEventInput{}
}

func assertZeroRuntimeDiscoveryCommit(t *testing.T, candidate RuntimeDiscoveryCommitCandidate) {
	t.Helper()
	if !reflect.DeepEqual(candidate, RuntimeDiscoveryCommitCandidate{}) {
		t.Fatalf("candidate = %#v, want zero", candidate)
	}
}

func cloneRuntimeDiscoveryWriterCandidate(
	input RuntimeDiscoveryCommitCandidate,
) RuntimeDiscoveryCommitCandidate {
	input.events = cloneRuntimeDiscoveryCommitEvents(input.events)
	return input
}

func isRuntimeDiscoveryWriterDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

type runtimeDiscoveryWriterPayload struct {
	DiscoveryDigest string                                `json:"discovery_digest"`
	SourceProbeID   string                                `json:"source_probe_id"`
	Instance        runtimeDiscoveryWriterInstancePayload `json:"instance"`
	ModelIDs        []string                              `json:"model_ids"`
}

type runtimeDiscoveryWriterInstancePayload struct {
	ID                   string                    `json:"id"`
	DeviceID             string                    `json:"device_id"`
	AdapterType          string                    `json:"adapter_type"`
	DisplayName          string                    `json:"display_name"`
	ExecutableVersion    string                    `json:"executable_version"`
	Status               loomruntime.RuntimeStatus `json:"status"`
	ObservedCapabilities []string                  `json:"observed_capabilities"`
	Capacity             int                       `json:"capacity"`
}
