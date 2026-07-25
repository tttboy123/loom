package state

import (
	"context"
	"encoding/json"
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
)

func TestCommitRuntimeStatusTransitionsBuildsCanonicalAtomicEvents(t *testing.T) {
	candidate := runtimeStatusWriterCandidate(t, 2)
	input := runtimeStatusWriterInput(candidate)
	input.Events[0], input.Events[1] = input.Events[1], input.Events[0]
	appender := &recordingEventBatchAppender{}

	got, err := CommitRuntimeStatusTransitions(
		context.Background(), appender, candidate, input,
	)
	if err != nil {
		t.Fatalf("CommitRuntimeStatusTransitions() error = %v", err)
	}
	if !got.Committed() ||
		got.SourceReconciliationDigest() != candidate.CandidateDigest() ||
		got.BaselineDigest() != candidate.BaselineDigest() ||
		got.SourceDiscoveryDigest() != candidate.SourceDiscoveryDigest() ||
		got.EventCount() != 2 ||
		len(got.Events()) != 2 ||
		!runtimeStatusWriterDigest(got.CommitDigest()) {
		t.Fatalf("commit Candidate = %#v", got)
	}
	if appender.calls != 1 || len(appender.batches) != 1 ||
		len(appender.batches[0]) != 2 {
		t.Fatalf("appender = calls %d batches %#v", appender.calls, appender.batches)
	}

	transitions := candidate.Transitions()
	events := appender.batches[0]
	for index, event := range events {
		transition := transitions[index]
		metadata := runtimeStatusEventInputByInstance(
			input.Events, transition.RuntimeInstanceID,
		)
		if event.ID != metadata.EventID ||
			event.StreamID != "runtime_instance:"+transition.RuntimeInstanceID ||
			event.Seq != transition.PreviousSequence+1 ||
			event.IdempotencyKey != metadata.IdempotencyKey ||
			event.Type != "RuntimeInstanceStatusChanged" ||
			event.SchemaVersion != 1 ||
			!event.EmittedAt.Equal(input.EmittedAt.UTC()) ||
			event.EmittedAt.Location() != time.UTC ||
			event.CorrelationID != input.ReconciliationID ||
			event.CausationID != transition.PreviousEventID {
			t.Fatalf("event[%d] = %#v", index, event)
		}
		var payload runtimeStatusWriterPayload
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatalf("payload[%d] error = %v", index, err)
		}
		wantPayload := runtimeStatusWriterPayload{
			ReconciliationDigest:  candidate.CandidateDigest(),
			BaselineDigest:        candidate.BaselineDigest(),
			SourceDiscoveryDigest: candidate.SourceDiscoveryDigest(),
			SourceProbeID:         transition.SourceProbeID,
			RuntimeInstanceID:     transition.RuntimeInstanceID,
			DeviceID:              transition.DeviceID,
			AdapterType:           transition.AdapterType,
			FromStatus:            transition.FromStatus,
			ToStatus:              transition.ToStatus,
			PreviousEventID:       transition.PreviousEventID,
			PreviousSequence:      transition.PreviousSequence,
		}
		if !reflect.DeepEqual(payload, wantPayload) {
			t.Fatalf("payload[%d] = %#v, want %#v", index, payload, wantPayload)
		}
		wantJSON, err := json.Marshal(wantPayload)
		if err != nil {
			t.Fatal(err)
		}
		if string(event.PayloadJSON) != string(wantJSON) {
			t.Fatalf("payload JSON[%d] = %s, want %s", index, event.PayloadJSON, wantJSON)
		}
	}
	if !reflect.DeepEqual(got.Events(), events) {
		t.Fatalf("Candidate events = %#v, appender events = %#v", got.Events(), events)
	}
}

func TestCommitRuntimeStatusTransitionsRejectsInvalidInputBeforeAppend(t *testing.T) {
	candidate := runtimeStatusWriterCandidate(t, 2)
	valid := runtimeStatusWriterInput(candidate)

	for _, appender := range []EventBatchAppender{
		nil,
		(*recordingEventBatchAppender)(nil),
	} {
		got, err := CommitRuntimeStatusTransitions(
			context.Background(), appender, candidate, valid,
		)
		if !errors.Is(err, ErrInvalidRuntimeStatusCommitInput) {
			t.Fatalf("nil appender error = %v", err)
		}
		assertZeroRuntimeStatusCommit(t, got)
	}
	got, err := CommitRuntimeStatusTransitions(
		nil, &recordingEventBatchAppender{}, candidate, valid,
	)
	if !errors.Is(err, ErrInvalidRuntimeStatusCommitInput) {
		t.Fatalf("nil context error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)

	for _, makeContext := range []func() context.Context{
		func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		},
		func() context.Context {
			ctx, cancel := context.WithDeadline(
				context.Background(), time.Now().Add(-time.Second),
			)
			cancel()
			return ctx
		},
	} {
		appender := &recordingEventBatchAppender{}
		ctx := makeContext()
		got, err := CommitRuntimeStatusTransitions(ctx, appender, candidate, valid)
		if !errors.Is(err, ctx.Err()) {
			t.Fatalf("context error = %v, want %v", err, ctx.Err())
		}
		assertZeroRuntimeStatusCommit(t, got)
		if appender.calls != 0 {
			t.Fatalf("appender calls = %d", appender.calls)
		}
	}

	zeroAppender := &recordingEventBatchAppender{}
	got, err = CommitRuntimeStatusTransitions(
		context.Background(),
		zeroAppender,
		loomruntime.RuntimeStatusReconciliationCandidate{},
		valid,
	)
	if !errors.Is(err, ErrInvalidRuntimeStatusCommitSource) {
		t.Fatalf("zero Candidate error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)
	if zeroAppender.calls != 0 {
		t.Fatalf("zero Candidate appender calls = %d", zeroAppender.calls)
	}

	noChange := runtimeStatusWriterNoChangeCandidate(t)
	emptyAppender := &recordingEventBatchAppender{}
	got, err = CommitRuntimeStatusTransitions(
		context.Background(),
		emptyAppender,
		noChange,
		RuntimeStatusCommitInput{
			ReconciliationID: "reconciliation.empty",
			EmittedAt:        valid.EmittedAt,
		},
	)
	if !errors.Is(err, ErrEmptyRuntimeStatusCommit) {
		t.Fatalf("empty Candidate error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)
	if emptyAppender.calls != 0 {
		t.Fatalf("empty Candidate appender calls = %d", emptyAppender.calls)
	}

	tests := []struct {
		name   string
		change func(RuntimeStatusCommitInput) RuntimeStatusCommitInput
	}{
		{"empty reconciliation", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.ReconciliationID = ""
			return input
		}},
		{"zero time", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.EmittedAt = time.Time{}
			return input
		}},
		{"nil events", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events = nil
			return input
		}},
		{"missing event", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events = input.Events[:1]
			return input
		}},
		{"extra event", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events = append(input.Events, RuntimeStatusEventInput{
				RuntimeInstanceID: "runtime.extra",
				EventID:           "event.status.extra",
				IdempotencyKey:    "key.status.extra",
				Seq:               2,
			})
			return input
		}},
		{"empty runtime", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].RuntimeInstanceID = ""
			return input
		}},
		{"unknown runtime", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].RuntimeInstanceID = "runtime.unknown"
			return input
		}},
		{"duplicate runtime", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[1].RuntimeInstanceID = input.Events[0].RuntimeInstanceID
			return input
		}},
		{"empty event id", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].EventID = ""
			return input
		}},
		{"duplicate event id", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[1].EventID = input.Events[0].EventID
			return input
		}},
		{"empty key", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].IdempotencyKey = ""
			return input
		}},
		{"duplicate key", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[1].IdempotencyKey = input.Events[0].IdempotencyKey
			return input
		}},
		{"zero sequence", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].Seq = 0
			return input
		}},
		{"below previous sequence", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[1].Seq = 1
			return input
		}},
		{"equal previous sequence", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].Seq--
			return input
		}},
		{"above exact next sequence", func(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
			input.Events[0].Seq++
			return input
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appender := &recordingEventBatchAppender{}
			input := test.change(cloneRuntimeStatusWriterInput(valid))
			got, err := CommitRuntimeStatusTransitions(
				context.Background(), appender, candidate, input,
			)
			if !errors.Is(err, ErrInvalidRuntimeStatusCommitInput) {
				t.Fatalf("error = %v", err)
			}
			assertZeroRuntimeStatusCommit(t, got)
			if appender.calls != 0 {
				t.Fatalf("appender calls = %d", appender.calls)
			}
		})
	}

	oversized := cloneRuntimeStatusWriterInput(valid)
	oversized.Events = make([]RuntimeStatusEventInput, 33)
	oversizedAppender := &recordingEventBatchAppender{}
	got, err = CommitRuntimeStatusTransitions(
		context.Background(), oversizedAppender, candidate, oversized,
	)
	if !errors.Is(err, ErrInvalidRuntimeStatusCommitInput) {
		t.Fatalf("oversized input error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)
	if oversizedAppender.calls != 0 {
		t.Fatalf("oversized appender calls = %d", oversizedAppender.calls)
	}
}

func TestCommitRuntimeStatusTransitionsRejectsAppenderFailureAndMismatchedResults(t *testing.T) {
	candidate := runtimeStatusWriterCandidate(t, 2)
	input := runtimeStatusWriterInput(candidate)
	appendErr := errors.New("append failed")
	got, err := CommitRuntimeStatusTransitions(
		context.Background(),
		&recordingEventBatchAppender{err: appendErr},
		candidate,
		input,
	)
	if !errors.Is(err, appendErr) {
		t.Fatalf("appender error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)

	tests := []struct {
		name   string
		result func([]journal.Event) []journal.Event
	}{
		{"nil", func([]journal.Event) []journal.Event { return nil }},
		{"short", func(events []journal.Event) []journal.Event {
			return cloneStateEvents(events[:1])
		}},
		{"long", func(events []journal.Event) []journal.Event {
			result := cloneStateEvents(events)
			return append(result, cloneStateEvent(events[0]))
		}},
		{"reordered", func(events []journal.Event) []journal.Event {
			result := cloneStateEvents(events)
			result[0], result[1] = result[1], result[0]
			return result
		}},
		{"id", mutateRuntimeStatusResult(func(event *journal.Event) { event.ID += ".changed" })},
		{"stream", mutateRuntimeStatusResult(func(event *journal.Event) { event.StreamID += ".changed" })},
		{"sequence", mutateRuntimeStatusResult(func(event *journal.Event) { event.Seq++ })},
		{"key", mutateRuntimeStatusResult(func(event *journal.Event) { event.IdempotencyKey += ".changed" })},
		{"type", mutateRuntimeStatusResult(func(event *journal.Event) { event.Type += ".changed" })},
		{"schema", mutateRuntimeStatusResult(func(event *journal.Event) { event.SchemaVersion++ })},
		{"time", mutateRuntimeStatusResult(func(event *journal.Event) {
			event.EmittedAt = event.EmittedAt.Add(time.Nanosecond)
		})},
		{"time location", mutateRuntimeStatusResult(func(event *journal.Event) {
			event.EmittedAt = event.EmittedAt.In(time.FixedZone("changed", 60*60))
		})},
		{"correlation", mutateRuntimeStatusResult(func(event *journal.Event) {
			event.CorrelationID += ".changed"
		})},
		{"causation", mutateRuntimeStatusResult(func(event *journal.Event) {
			event.CausationID += ".changed"
		})},
		{"payload", mutateRuntimeStatusResult(func(event *journal.Event) {
			event.PayloadJSON = append(event.PayloadJSON, ' ')
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appender := &recordingEventBatchAppender{result: test.result}
			got, err := CommitRuntimeStatusTransitions(
				context.Background(), appender, candidate, input,
			)
			if !errors.Is(err, ErrRuntimeStatusCommitResultMismatch) {
				t.Fatalf("error = %v", err)
			}
			assertZeroRuntimeStatusCommit(t, got)
			if appender.calls != 1 {
				t.Fatalf("appender calls = %d", appender.calls)
			}
		})
	}

	mutatingAppender := &recordingEventBatchAppender{
		result: func(events []journal.Event) []journal.Event {
			events[0].PayloadJSON[0] ^= 1
			return cloneStateEvents(events)
		},
	}
	got, err = CommitRuntimeStatusTransitions(
		context.Background(), mutatingAppender, candidate, input,
	)
	if !errors.Is(err, ErrRuntimeStatusCommitResultMismatch) {
		t.Fatalf("mutating appender error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)
}

func TestRuntimeStatusCommitCandidateMutationAndDigestIsolation(t *testing.T) {
	source := runtimeStatusWriterCandidate(t, 2)
	input := runtimeStatusWriterInput(source)
	beforeInput := cloneRuntimeStatusWriterInput(input)
	beforeSourceDigest := source.CandidateDigest()
	sourceTransitions := source.Transitions()
	sourceTransitions[0].ToStatus = loomruntime.RuntimeDisabled

	appender := &recordingEventBatchAppender{}
	candidate, err := CommitRuntimeStatusTransitions(
		context.Background(), appender, source, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, beforeInput) ||
		source.CandidateDigest() != beforeSourceDigest ||
		source.Transitions()[0].ToStatus == loomruntime.RuntimeDisabled {
		t.Fatal("source or input mutated")
	}

	events := candidate.Events()
	events[0].PayloadJSON[0] ^= 1
	events[0].ID += ".changed"
	if candidate.Events()[0].ID == events[0].ID ||
		reflect.DeepEqual(candidate.Events()[0].PayloadJSON, events[0].PayloadJSON) {
		t.Fatal("Events accessor leaked mutable data")
	}
	appender.batches[0][0].PayloadJSON[0] ^= 1
	if reflect.DeepEqual(candidate.Events()[0].PayloadJSON, appender.batches[0][0].PayloadJSON) {
		t.Fatal("appender batch aliases Candidate")
	}

	baseDigest := candidate.CommitDigest()
	mutations := []func(*RuntimeStatusCommitCandidate){
		func(value *RuntimeStatusCommitCandidate) { value.sourceReconciliationDigest = strings.Repeat("a", 64) },
		func(value *RuntimeStatusCommitCandidate) { value.baselineDigest = strings.Repeat("b", 64) },
		func(value *RuntimeStatusCommitCandidate) { value.sourceDiscoveryDigest = strings.Repeat("c", 64) },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].ID += ".changed" },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].StreamID += ".changed" },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].Seq++ },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].IdempotencyKey += ".changed" },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].Type += ".changed" },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].SchemaVersion++ },
		func(value *RuntimeStatusCommitCandidate) {
			value.events[0].EmittedAt = value.events[0].EmittedAt.Add(time.Nanosecond)
		},
		func(value *RuntimeStatusCommitCandidate) { value.events[0].CorrelationID += ".changed" },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].CausationID += ".changed" },
		func(value *RuntimeStatusCommitCandidate) { value.events[0].PayloadJSON[0] ^= 1 },
	}
	for index, mutate := range mutations {
		changed := cloneRuntimeStatusCommitCandidate(candidate)
		mutate(&changed)
		digest, err := digestRuntimeStatusCommitCandidate(changed)
		if err != nil {
			t.Fatalf("mutation %d error = %v", index, err)
		}
		if digest == baseDigest {
			t.Fatalf("mutation %d did not change digest", index)
		}
	}
	changedCount := cloneRuntimeStatusCommitCandidate(candidate)
	changedCount.eventCount++
	if !errors.Is(
		validateRuntimeStatusCommitCandidate(changedCount),
		ErrRuntimeStatusCommitDigestMismatch,
	) {
		t.Fatal("changed event count accepted")
	}
	forged := cloneRuntimeStatusCommitCandidate(candidate)
	forged.commitDigest = strings.Repeat("f", 64)
	if !errors.Is(
		validateRuntimeStatusCommitCandidate(forged),
		ErrRuntimeStatusCommitDigestMismatch,
	) {
		t.Fatal("forged commit digest accepted")
	}
}

func TestCommitRuntimeStatusTransitionsRealJournalRetryAndConflicts(t *testing.T) {
	candidate := runtimeStatusWriterCandidate(t, 2)
	input := runtimeStatusWriterInput(candidate)
	store, db := openSavedTeamWriterStore(t)
	seedRuntimeStatusPreviousEvents(t, store, candidate)

	first, err := CommitRuntimeStatusTransitions(
		context.Background(), store, candidate, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := CommitRuntimeStatusTransitions(
		context.Background(), store, candidate, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, retry) {
		t.Fatalf("retry = %#v, first = %#v", retry, first)
	}
	wantRows := candidate.TransitionCount() * 2
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != wantRows {
		t.Fatalf("rows = %d, want %d", rows, wantRows)
	}

	idempotencyConflict := cloneRuntimeStatusWriterInput(input)
	idempotencyConflict.Events[0].EventID += ".changed"
	got, err := CommitRuntimeStatusTransitions(
		context.Background(), store, candidate, idempotencyConflict,
	)
	if !errors.Is(err, journal.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)

	stale := cloneRuntimeStatusWriterInput(input)
	for index := range stale.Events {
		stale.Events[index].EventID += ".stale"
		stale.Events[index].IdempotencyKey += ".stale"
	}
	got, err = CommitRuntimeStatusTransitions(
		context.Background(), store, candidate, stale,
	)
	if !errors.Is(err, journal.ErrSequenceConflict) {
		t.Fatalf("occupied exact-next error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != wantRows {
		t.Fatalf("rows after conflicts = %d, want %d", rows, wantRows)
	}

	partialStore, partialDB := openSavedTeamWriterStore(t)
	seedRuntimeStatusPreviousEvents(t, partialStore, candidate)
	recorder := &recordingEventBatchAppender{}
	if _, err := CommitRuntimeStatusTransitions(
		context.Background(), recorder, candidate, input,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := partialStore.Append(
		context.Background(), recorder.batches[0][0],
	); err != nil {
		t.Fatal(err)
	}
	got, err = CommitRuntimeStatusTransitions(
		context.Background(), partialStore, candidate, input,
	)
	if !errors.Is(err, journal.ErrPartialEventBatchConflict) {
		t.Fatalf("partial conflict error = %v", err)
	}
	assertZeroRuntimeStatusCommit(t, got)
	if err := partialDB.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != candidate.TransitionCount()+1 {
		t.Fatalf("partial rows = %d", rows)
	}
}

func TestRuntimeStatusCommitProductionBoundary(t *testing.T) {
	content, err := os.ReadFile("runtime_status_writer.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(), "runtime_status_writer.go", content, parser.ImportsOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if strings.HasPrefix(path, "loom-pi-rebuild/") &&
			path != "loom-pi-rebuild/internal/journal" &&
			path != "loom-pi-rebuild/internal/runtime" {
			t.Fatalf("forbidden project import %q", path)
		}
		for _, forbidden := range []string{
			"database/sql",
			"os/exec",
			"net",
			"path/filepath",
			"internal/projection",
			"internal/runtime/piadapter",
		} {
			if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
				t.Fatalf("forbidden import %q", path)
			}
		}
	}
	product := string(content)
	for _, forbidden := range []string{
		"RuntimeInstanceDiscovered",
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
			t.Fatalf("product contains forbidden surface %q", forbidden)
		}
	}
}

type runtimeStatusWriterPayload struct {
	ReconciliationDigest  string                    `json:"reconciliation_digest"`
	BaselineDigest        string                    `json:"baseline_digest"`
	SourceDiscoveryDigest string                    `json:"source_discovery_digest"`
	SourceProbeID         string                    `json:"source_probe_id"`
	RuntimeInstanceID     string                    `json:"runtime_instance_id"`
	DeviceID              string                    `json:"device_id"`
	AdapterType           string                    `json:"adapter_type"`
	FromStatus            loomruntime.RuntimeStatus `json:"from_status"`
	ToStatus              loomruntime.RuntimeStatus `json:"to_status"`
	PreviousEventID       string                    `json:"previous_event_id"`
	PreviousSequence      int64                     `json:"previous_sequence"`
}

type runtimeStatusWriterProbe struct {
	observations []loomruntime.RuntimeObservation
}

func (runtimeStatusWriterProbe) ID() string { return "probe.status-writer" }

func (p runtimeStatusWriterProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	result := make([]loomruntime.RuntimeObservation, len(p.observations))
	copy(result, p.observations)
	return result, nil
}

func runtimeStatusWriterCandidate(
	t *testing.T,
	count int,
) loomruntime.RuntimeStatusReconciliationCandidate {
	t.Helper()
	baseline := make([]loomruntime.RuntimeStatusBaseline, count)
	observations := make([]loomruntime.RuntimeObservation, count)
	for index := range baseline {
		suffix := string(rune('a' + index))
		id := "runtime." + suffix
		previous, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID:                   id,
			DeviceID:             "device.local",
			AdapterType:          "pi-cli",
			DisplayName:          "Runtime " + suffix,
			ExecutableVersion:    "1.0.0",
			Status:               loomruntime.RuntimeOnline,
			ObservedCapabilities: []string{"models", "version"},
			Capacity:             index + 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		current := previous
		current.Status = loomruntime.RuntimeOffline
		baseline[index] = loomruntime.RuntimeStatusBaseline{
			Instance:         previous,
			PreviousEventID:  "event.discovery." + id,
			PreviousSequence: int64(index + 1),
		}
		observations[index] = loomruntime.RuntimeObservation{
			Instance: current,
			ModelIDs: []string{"provider/model-" + suffix},
		}
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{runtimeStatusWriterProbe{observations: observations}},
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func runtimeStatusWriterNoChangeCandidate(
	t *testing.T,
) loomruntime.RuntimeStatusReconciliationCandidate {
	t.Helper()
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                   "runtime.a",
		DeviceID:             "device.local",
		AdapterType:          "pi-cli",
		DisplayName:          "Runtime a",
		ExecutableVersion:    "1.0.0",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"models", "version"},
		Capacity:             1,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{runtimeStatusWriterProbe{
			observations: []loomruntime.RuntimeObservation{{
				Instance: instance,
				ModelIDs: []string{"provider/model-a"},
			}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		context.Background(),
		[]loomruntime.RuntimeStatusBaseline{{
			Instance:         instance,
			PreviousEventID:  "event.discovery.runtime.a",
			PreviousSequence: 1,
		}},
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func runtimeStatusWriterInput(
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) RuntimeStatusCommitInput {
	transitions := candidate.Transitions()
	events := make([]RuntimeStatusEventInput, len(transitions))
	for index, transition := range transitions {
		events[index] = RuntimeStatusEventInput{
			RuntimeInstanceID: transition.RuntimeInstanceID,
			EventID:           "event.status." + transition.RuntimeInstanceID,
			IdempotencyKey:    "key.status." + transition.RuntimeInstanceID,
			Seq:               transition.PreviousSequence + 1,
		}
	}
	return RuntimeStatusCommitInput{
		ReconciliationID: "reconciliation.one",
		EmittedAt: time.Date(
			2026, 7, 25, 20, 0, 0, 123,
			time.FixedZone("fixture", 8*60*60),
		),
		Events: events,
	}
}

func cloneRuntimeStatusWriterInput(input RuntimeStatusCommitInput) RuntimeStatusCommitInput {
	input.Events = append([]RuntimeStatusEventInput(nil), input.Events...)
	return input
}

func runtimeStatusEventInputByInstance(
	inputs []RuntimeStatusEventInput,
	instanceID string,
) RuntimeStatusEventInput {
	for _, input := range inputs {
		if input.RuntimeInstanceID == instanceID {
			return input
		}
	}
	return RuntimeStatusEventInput{}
}

func mutateRuntimeStatusResult(
	mutate func(*journal.Event),
) func([]journal.Event) []journal.Event {
	return func(events []journal.Event) []journal.Event {
		result := cloneStateEvents(events)
		mutate(&result[0])
		return result
	}
}

func assertZeroRuntimeStatusCommit(
	t *testing.T,
	candidate RuntimeStatusCommitCandidate,
) {
	t.Helper()
	if !reflect.DeepEqual(candidate, RuntimeStatusCommitCandidate{}) {
		t.Fatalf("candidate = %#v, want zero", candidate)
	}
}

func cloneRuntimeStatusCommitCandidate(
	input RuntimeStatusCommitCandidate,
) RuntimeStatusCommitCandidate {
	input.events = cloneStateEvents(input.events)
	return input
}

func runtimeStatusWriterDigest(value string) bool {
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

func seedRuntimeStatusPreviousEvents(
	t *testing.T,
	store *journal.Store,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) {
	t.Helper()
	for _, transition := range candidate.Transitions() {
		_, err := store.Append(context.Background(), journal.Event{
			ID:             transition.PreviousEventID,
			StreamID:       "runtime_instance:" + transition.RuntimeInstanceID,
			Seq:            transition.PreviousSequence,
			IdempotencyKey: "key." + transition.PreviousEventID,
			Type:           "RuntimeInstanceDiscovered",
			SchemaVersion:  1,
			EmittedAt:      time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC),
			CorrelationID:  "discovery.previous",
			CausationID:    "",
			PayloadJSON:    []byte(`{"fixture":"previous"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
