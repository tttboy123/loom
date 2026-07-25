package app

import (
	"context"
	"errors"
	"go/ast"
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

func TestRunObservedRuntimeStatusReconciliationOncePrevalidatesInputs(t *testing.T) {
	baseline, snapshot, _ := appStatusSources(
		t, "prevalidate", 1, loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	valid := &recordingRuntimeStatusCommitter{}
	var typedNil *recordingRuntimeStatusCommitter

	tests := []struct {
		name      string
		ctx       context.Context
		committer RuntimeStatusCommitter
		want      error
	}{
		{name: "nil_context", committer: valid, want: ErrInvalidRuntimeStatusRun},
		{
			name:      "nil_committer",
			ctx:       context.Background(),
			committer: nil,
			want:      ErrInvalidRuntimeStatusRun,
		},
		{
			name:      "typed_nil_committer",
			ctx:       context.Background(),
			committer: typedNil,
			want:      ErrInvalidRuntimeStatusRun,
		},
		{
			name:      "canceled_context",
			ctx:       canceledAppDiscoveryContext(),
			committer: valid,
			want:      context.Canceled,
		},
		{
			name:      "expired_context",
			ctx:       expiredAppDiscoveryContext(),
			committer: valid,
			want:      context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciliation, commit, err :=
				RunObservedRuntimeStatusReconciliationOnce(
					test.ctx, baseline, snapshot, test.committer,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroAppStatusRun(t, reconciliation, commit)
		})
	}
	if valid.calls != 0 {
		t.Fatalf("committer calls = %d, want 0", valid.calls)
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceSkipsNoChangeCommit(
	t *testing.T,
) {
	baseline, snapshot, _ := appStatusSources(
		t, "no-change", 2, loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	committer := &recordingRuntimeStatusCommitter{
		err: errors.New("must not be called"),
	}

	reconciliation, commit, err :=
		RunObservedRuntimeStatusReconciliationOnce(
			context.Background(), baseline, snapshot, committer,
		)
	if err != nil {
		t.Fatalf("RunObservedRuntimeStatusReconciliationOnce() error = %v", err)
	}
	if !reconciliation.Reconciled() ||
		reconciliation.TransitionCount() != 0 ||
		len(reconciliation.Transitions()) != 0 ||
		reconciliation.BaselineDigest() == "" ||
		reconciliation.SourceDiscoveryDigest() != snapshot.Digest() ||
		reconciliation.CandidateDigest() == "" {
		t.Fatalf("reconciliation = %#v, want valid no-change Candidate", reconciliation)
	}
	assertZeroAppStatusCommit(t, commit)
	if committer.calls != 0 {
		t.Fatalf("committer calls = %d, want 0", committer.calls)
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceNoChangeHonorsPostReconciliationCancellation(
	t *testing.T,
) {
	baseline, snapshot, _ := appStatusSources(
		t, "no-change-cancel", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOnline,
	)
	ctx := &cancelAfterErrCallsContext{cancelAt: 5}
	committer := &recordingRuntimeStatusCommitter{}

	reconciliation, commit, err :=
		RunObservedRuntimeStatusReconciliationOnce(
			ctx, baseline, snapshot, committer,
		)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	assertZeroAppStatusRun(t, reconciliation, commit)
	if ctx.calls != ctx.cancelAt {
		t.Fatalf("context Err() calls = %d, want %d", ctx.calls, ctx.cancelAt)
	}
	if committer.calls != 0 {
		t.Fatalf("committer calls = %d, want 0", committer.calls)
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceDelegatesExactCandidate(
	t *testing.T,
) {
	for _, count := range []int{1, 3} {
		t.Run(string(rune('0'+count))+"_transitions", func(t *testing.T) {
			baseline, snapshot, _ := appStatusSources(
				t, "delegate."+string(rune('0'+count)), count,
				loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
			)
			var received loomruntime.RuntimeStatusReconciliationCandidate
			committer := &recordingRuntimeStatusCommitter{
				commit: func(
					ctx context.Context,
					candidate loomruntime.RuntimeStatusReconciliationCandidate,
				) (state.RuntimeStatusCommitCandidate, error) {
					received = candidate
					return mintAppStatusCommit(t, ctx, candidate, "delegate"), nil
				},
			}

			reconciliation, commit, err :=
				RunObservedRuntimeStatusReconciliationOnce(
					context.Background(), baseline, snapshot, committer,
				)
			if err != nil {
				t.Fatalf("RunObservedRuntimeStatusReconciliationOnce() error = %v", err)
			}
			if committer.calls != 1 {
				t.Fatalf("committer calls = %d, want 1", committer.calls)
			}
			assertSameAppStatusReconciliation(t, received, reconciliation)
			if reconciliation.TransitionCount() != count ||
				commit.EventCount() != count ||
				len(commit.Events()) != count ||
				!commit.Committed() ||
				commit.SourceReconciliationDigest() !=
					reconciliation.CandidateDigest() ||
				commit.BaselineDigest() != reconciliation.BaselineDigest() ||
				commit.SourceDiscoveryDigest() !=
					reconciliation.SourceDiscoveryDigest() ||
				!isAppDiscoveryDigest(commit.CommitDigest()) {
				t.Fatalf("result mismatch: reconciliation=%#v commit=%#v",
					reconciliation, commit)
			}
		})
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceRejectsSourceAndCommitErrors(
	t *testing.T,
) {
	ctx := context.Background()
	baseline, snapshot, _ := appStatusSources(
		t, "errors", 1, loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)

	invalidBaseline := append([]loomruntime.RuntimeStatusBaseline(nil), baseline...)
	invalidBaseline[0].PreviousEventID = ""
	never := &recordingRuntimeStatusCommitter{}
	reconciliation, commit, err := RunObservedRuntimeStatusReconciliationOnce(
		ctx, invalidBaseline, snapshot, never,
	)
	if !errors.Is(err, loomruntime.ErrInvalidRuntimeStatusBaseline) {
		t.Fatalf("invalid baseline error = %v", err)
	}
	assertZeroAppStatusRun(t, reconciliation, commit)
	if never.calls != 0 {
		t.Fatalf("committer calls = %d, want 0", never.calls)
	}

	driftedSnapshot := appStatusSnapshot(
		t, "errors", 1, loomruntime.RuntimeOffline, "other-device",
	)
	reconciliation, commit, err = RunObservedRuntimeStatusReconciliationOnce(
		ctx, baseline, driftedSnapshot, never,
	)
	if !errors.Is(err, loomruntime.ErrRuntimeStatusIdentityDrift) {
		t.Fatalf("identity drift error = %v", err)
	}
	assertZeroAppStatusRun(t, reconciliation, commit)
	if never.calls != 0 {
		t.Fatalf("committer calls after drift = %d, want 0", never.calls)
	}

	sentinel := errors.New("commit rejected")
	rejecting := &recordingRuntimeStatusCommitter{err: sentinel}
	reconciliation, commit, err = RunObservedRuntimeStatusReconciliationOnce(
		ctx, baseline, snapshot, rejecting,
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("committer error = %v, want sentinel", err)
	}
	assertZeroAppStatusRun(t, reconciliation, commit)
	if rejecting.calls != 1 {
		t.Fatalf("rejecting calls = %d, want 1", rejecting.calls)
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	canceling := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			result := mintAppStatusCommit(t, ctx, candidate, "cancel-after")
			cancel()
			return result, nil
		},
	}
	reconciliation, commit, err = RunObservedRuntimeStatusReconciliationOnce(
		cancelCtx, baseline, snapshot, canceling,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("post-commit context error = %v, want canceled", err)
	}
	assertZeroAppStatusRun(t, reconciliation, commit)
	if canceling.calls != 1 {
		t.Fatalf("canceling calls = %d, want 1", canceling.calls)
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceRejectsCommitMismatches(
	t *testing.T,
) {
	ctx := context.Background()
	baseline, snapshot, _ := appStatusSources(
		t, "mismatch", 1, loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)

	alteredBaseline := append([]loomruntime.RuntimeStatusBaseline(nil), baseline...)
	alteredBaseline[0].PreviousEventID = "event.other"
	alteredCandidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		ctx, alteredBaseline, snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	alteredCommit := mintAppStatusCommit(t, ctx, alteredCandidate, "altered")

	otherBaseline, otherSnapshot, _ := appStatusSources(
		t, "other-source", 1, loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	otherCandidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		ctx, otherBaseline, otherSnapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	otherCommit := mintAppStatusCommit(t, ctx, otherCandidate, "other-source")

	manyBaseline, manySnapshot, _ := appStatusSources(
		t, "many", 2, loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	manyCandidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		ctx, manyBaseline, manySnapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	manyCommit := mintAppStatusCommit(t, ctx, manyCandidate, "many")

	tests := []struct {
		name      string
		candidate state.RuntimeStatusCommitCandidate
	}{
		{name: "zero_uncommitted", candidate: state.RuntimeStatusCommitCandidate{}},
		{name: "wrong_baseline_and_reconciliation", candidate: alteredCommit},
		{name: "wrong_discovery_source", candidate: otherCommit},
		{name: "wrong_event_count", candidate: manyCommit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			committer := &recordingRuntimeStatusCommitter{candidate: test.candidate}
			reconciliation, commit, err :=
				RunObservedRuntimeStatusReconciliationOnce(
					ctx, baseline, snapshot, committer,
				)
			if !errors.Is(err, ErrRuntimeStatusCommitResultMismatch) {
				t.Fatalf("error = %v, want result mismatch", err)
			}
			assertZeroAppStatusRun(t, reconciliation, commit)
			if committer.calls != 1 {
				t.Fatalf("committer calls = %d, want 1", committer.calls)
			}
		})
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceValidatesEveryCommitResultFact(
	t *testing.T,
) {
	ctx := context.Background()
	baseline, snapshot, _ := appStatusSources(
		t, "fact-matrix", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	reconciliation, err := loomruntime.ReconcileObservedRuntimeStatuses(
		ctx, baseline, snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	commit := mintAppStatusCommit(t, ctx, reconciliation, "fact-matrix")
	valid := runtimeStatusCommitResultFacts{
		committed:                  commit.Committed(),
		sourceReconciliationDigest: commit.SourceReconciliationDigest(),
		baselineDigest:             commit.BaselineDigest(),
		sourceDiscoveryDigest:      commit.SourceDiscoveryDigest(),
		eventCount:                 commit.EventCount(),
		eventAccessorCount:         len(commit.Events()),
		commitDigest:               commit.CommitDigest(),
	}
	if !validRuntimeStatusCommitResultFacts(reconciliation, valid) {
		t.Fatal("known-valid primitive result facts were rejected")
	}

	tests := []struct {
		name   string
		mutate func(*runtimeStatusCommitResultFacts)
	}{
		{
			name: "committed",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.committed = false
			},
		},
		{
			name: "reconciliation_digest",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.sourceReconciliationDigest = strings.Repeat("a", 64)
			},
		},
		{
			name: "baseline_digest",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.baselineDigest = strings.Repeat("b", 64)
			},
		},
		{
			name: "discovery_digest",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.sourceDiscoveryDigest = strings.Repeat("c", 64)
			},
		},
		{
			name: "event_count",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.eventCount++
			},
		},
		{
			name: "event_accessor_count",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.eventAccessorCount++
			},
		},
		{
			name: "commit_digest_shape",
			mutate: func(facts *runtimeStatusCommitResultFacts) {
				facts.commitDigest = strings.ToUpper(facts.commitDigest)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := valid
			test.mutate(&mutated)
			if validRuntimeStatusCommitResultFacts(reconciliation, mutated) {
				t.Fatalf("single-field mutation %q was accepted", test.name)
			}
		})
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceIsMutationIsolated(
	t *testing.T,
) {
	baseline, snapshot, _ := appStatusSources(
		t, "mutation", 2, loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	originalBaseline := cloneAppStatusBaselines(baseline)
	committer := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			transitions := candidate.Transitions()
			transitions[0].RuntimeInstanceID = "mutated"
			if candidate.Transitions()[0].RuntimeInstanceID == "mutated" {
				t.Fatal("Candidate transition accessor leaked mutation")
			}
			return mintAppStatusCommit(t, ctx, candidate, "mutation"), nil
		},
	}

	reconciliation, commit, err := RunObservedRuntimeStatusReconciliationOnce(
		context.Background(), baseline, snapshot, committer,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(baseline, originalBaseline) {
		t.Fatalf("baseline mutated: got %#v want %#v", baseline, originalBaseline)
	}
	events := commit.Events()
	events[0].PayloadJSON[0] ^= 0xff
	if reflect.DeepEqual(events, commit.Events()) {
		t.Fatal("commit Event accessor leaked mutation")
	}
	transitions := reconciliation.Transitions()
	transitions[0].SourceProbeID = "mutated"
	if reflect.DeepEqual(transitions, reconciliation.Transitions()) {
		t.Fatal("reconciliation accessor leaked mutation")
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceRealJournalRetry(
	t *testing.T,
) {
	ctx := context.Background()
	baseline, current, previous := appStatusSources(
		t, "sqlite-status", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx, store, previous, appDiscoveryCommitInput(previous, "status-prior"),
	); err != nil {
		t.Fatalf("CommitRuntimeDiscoverySnapshot() error = %v", err)
	}
	committer := &boundRuntimeStatusCommitter{
		appender: store,
		suffix:   "sqlite-status",
	}

	firstReconciliation, firstCommit, err :=
		RunObservedRuntimeStatusReconciliationOnce(
			ctx, baseline, current, committer,
		)
	if err != nil {
		t.Fatalf("first run error = %v", err)
	}
	secondReconciliation, secondCommit, err :=
		RunObservedRuntimeStatusReconciliationOnce(
			ctx, baseline, current, committer,
		)
	if err != nil {
		t.Fatalf("retry error = %v", err)
	}
	assertSameAppStatusReconciliation(
		t, firstReconciliation, secondReconciliation,
	)
	if firstCommit.CommitDigest() != secondCommit.CommitDigest() ||
		!reflect.DeepEqual(firstCommit.Events(), secondCommit.Events()) {
		t.Fatal("exact retry did not return the same committed fact set")
	}
	if committer.calls != 2 {
		t.Fatalf("committer calls = %d, want 2", committer.calls)
	}
	if got := appDiscoveryEventCount(t, db); got != 4 {
		t.Fatalf("event row count = %d, want 4", got)
	}
}

func TestRunObservedRuntimeStatusReconciliationOnceStaticBoundary(t *testing.T) {
	source, err := os.ReadFile("runtime_status.go")
	if err != nil {
		t.Fatalf("read runtime_status.go: %v", err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(), "runtime_status.go", source, 0,
	)
	if err != nil {
		t.Fatalf("parse runtime_status.go: %v", err)
	}
	allowed := map[string]bool{
		"context":                          true,
		"errors":                           true,
		"strings":                          true,
		"loom-pi-rebuild/internal/runtime": true,
		"loom-pi-rebuild/internal/state":   true,
	}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !allowed[path] {
			t.Fatalf("forbidden product import %q", path)
		}
	}

	forbiddenSelectors := map[string]bool{
		"journal":    true,
		"projection": true,
		"sql":        true,
		"exec":       true,
		"os":         true,
		"net":        true,
		"time":       true,
		"uuid":       true,
		"rand":       true,
	}
	forbiddenIdentifiers := map[string]bool{
		"AppendBatch":                    true,
		"BuildRuntimeStatusBaselines":    true,
		"DiscoverRuntime":                true,
		"DiscoverConfiguredRuntimes":     true,
		"CommitRuntimeStatusTransitions": true,
		"RuntimeStatusCommitInput":       true,
		"RuntimeStatusEventInput":        true,
		"NewTicker":                      true,
		"Sleep":                          true,
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt:
			t.Error("product contains a forbidden go statement")
		case *ast.CallExpr:
			if identifier, ok := typed.Fun.(*ast.Ident); ok {
				switch identifier.Name {
				case "make", "new", "append":
					t.Errorf("product contains forbidden allocation call %q",
						identifier.Name)
				}
			}
		case *ast.SelectorExpr:
			if owner, ok := typed.X.(*ast.Ident); ok &&
				forbiddenSelectors[owner.Name] {
				t.Errorf("product contains forbidden selector %s.%s",
					owner.Name, typed.Sel.Name)
			}
			if forbiddenIdentifiers[typed.Sel.Name] {
				t.Errorf("product contains forbidden call or selector %q",
					typed.Sel.Name)
			}
		case *ast.Ident:
			if forbiddenIdentifiers[typed.Name] {
				t.Errorf("product contains forbidden identifier %q", typed.Name)
			}
		}
		return true
	})
	for _, marker := range []string{
		"RuntimeInstanceStatusChanged",
		"runtime_instance:",
		"IdempotencyKey",
		"EventID",
		"EmittedAt",
	} {
		if strings.Contains(string(source), marker) {
			t.Errorf("product contains forbidden Event construction marker %q",
				marker)
		}
	}
}

type recordingRuntimeStatusCommitter struct {
	commit func(
		context.Context,
		loomruntime.RuntimeStatusReconciliationCandidate,
	) (state.RuntimeStatusCommitCandidate, error)
	candidate state.RuntimeStatusCommitCandidate
	err       error
	calls     int
}

func (c *recordingRuntimeStatusCommitter) CommitRuntimeStatus(
	ctx context.Context,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) (state.RuntimeStatusCommitCandidate, error) {
	c.calls++
	if c.commit != nil {
		return c.commit(ctx, candidate)
	}
	return c.candidate, c.err
}

type boundRuntimeStatusCommitter struct {
	appender state.EventBatchAppender
	suffix   string
	calls    int
}

func (c *boundRuntimeStatusCommitter) CommitRuntimeStatus(
	ctx context.Context,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) (state.RuntimeStatusCommitCandidate, error) {
	c.calls++
	return state.CommitRuntimeStatusTransitions(
		ctx, c.appender, candidate, appStatusCommitInput(candidate, c.suffix),
	)
}

type cancelAfterErrCallsContext struct {
	cancelAt int
	calls    int
}

func (*cancelAfterErrCallsContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (*cancelAfterErrCallsContext) Done() <-chan struct{} {
	return nil
}

func (c *cancelAfterErrCallsContext) Err() error {
	c.calls++
	if c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func (*cancelAfterErrCallsContext) Value(any) any {
	return nil
}

func appStatusSources(
	t *testing.T,
	prefix string,
	count int,
	previousStatus loomruntime.RuntimeStatus,
	currentStatus loomruntime.RuntimeStatus,
) (
	[]loomruntime.RuntimeStatusBaseline,
	loomruntime.RuntimeDiscoverySnapshot,
	loomruntime.RuntimeDiscoverySnapshot,
) {
	t.Helper()
	previous := appStatusSnapshot(
		t, prefix, count, previousStatus, "device.local",
	)
	current := appStatusSnapshot(
		t, prefix, count, currentStatus, "device.local",
	)
	observations := previous.Observations()
	baseline := make([]loomruntime.RuntimeStatusBaseline, len(observations))
	for index, observation := range observations {
		baseline[index] = loomruntime.RuntimeStatusBaseline{
			Instance: observation.Instance,
			PreviousEventID: "event.status-prior." +
				observation.Instance.ID,
			PreviousSequence: 1,
		}
	}
	return baseline, current, previous
}

func appStatusSnapshot(
	t *testing.T,
	prefix string,
	count int,
	status loomruntime.RuntimeStatus,
	deviceID string,
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	observations := make([]loomruntime.RuntimeObservation, count)
	for index := range observations {
		suffix := string(rune('a' + index))
		instance := appDiscoveryInstance(
			"runtime."+prefix+"."+suffix, prefix+" "+suffix,
		)
		instance.Status = status
		instance.DeviceID = deviceID
		normalized, err := loomruntime.NewRuntimeInstance(instance)
		if err != nil {
			t.Fatal(err)
		}
		observations[index] = loomruntime.RuntimeObservation{
			Instance: normalized,
			ModelIDs: []string{"model." + prefix + "." + suffix},
		}
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{&appDiscoveryProbe{
			id:           "probe." + prefix,
			observations: observations,
		}},
	)
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	return snapshot
}

func mintAppStatusCommit(
	t *testing.T,
	ctx context.Context,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
	suffix string,
) state.RuntimeStatusCommitCandidate {
	t.Helper()
	commit, err := state.CommitRuntimeStatusTransitions(
		ctx,
		echoAppDiscoveryAppender{},
		candidate,
		appStatusCommitInput(candidate, suffix),
	)
	if err != nil {
		t.Fatalf("CommitRuntimeStatusTransitions() error = %v", err)
	}
	return commit
}

func appStatusCommitInput(
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
	suffix string,
) state.RuntimeStatusCommitInput {
	transitions := candidate.Transitions()
	events := make([]state.RuntimeStatusEventInput, len(transitions))
	for index, transition := range transitions {
		events[index] = state.RuntimeStatusEventInput{
			RuntimeInstanceID: transition.RuntimeInstanceID,
			EventID: "event.status." + suffix + "." +
				transition.RuntimeInstanceID,
			IdempotencyKey: "key.status." + suffix + "." +
				transition.RuntimeInstanceID,
			Seq: transition.PreviousSequence + 1,
		}
	}
	return state.RuntimeStatusCommitInput{
		ReconciliationID: "reconciliation." + suffix,
		EmittedAt: time.Date(
			2026, 7, 25, 22, 0, 0, 0, time.UTC,
		),
		Events: events,
	}
}

func cloneAppStatusBaselines(
	baselines []loomruntime.RuntimeStatusBaseline,
) []loomruntime.RuntimeStatusBaseline {
	cloned := append([]loomruntime.RuntimeStatusBaseline(nil), baselines...)
	for index := range cloned {
		cloned[index].Instance.ObservedCapabilities = append(
			[]string(nil), cloned[index].Instance.ObservedCapabilities...,
		)
	}
	return cloned
}

func assertSameAppStatusReconciliation(
	t *testing.T,
	left loomruntime.RuntimeStatusReconciliationCandidate,
	right loomruntime.RuntimeStatusReconciliationCandidate,
) {
	t.Helper()
	if left.Reconciled() != right.Reconciled() ||
		left.BaselineDigest() != right.BaselineDigest() ||
		left.SourceDiscoveryDigest() != right.SourceDiscoveryDigest() ||
		left.TransitionCount() != right.TransitionCount() ||
		left.CandidateDigest() != right.CandidateDigest() ||
		!reflect.DeepEqual(left.Transitions(), right.Transitions()) {
		t.Fatalf("reconciliations differ: left=%#v right=%#v", left, right)
	}
}

func assertZeroAppStatusRun(
	t *testing.T,
	reconciliation loomruntime.RuntimeStatusReconciliationCandidate,
	commit state.RuntimeStatusCommitCandidate,
) {
	t.Helper()
	if reconciliation.Reconciled() ||
		reconciliation.BaselineDigest() != "" ||
		reconciliation.SourceDiscoveryDigest() != "" ||
		reconciliation.TransitionCount() != 0 ||
		len(reconciliation.Transitions()) != 0 ||
		reconciliation.CandidateDigest() != "" {
		t.Fatalf("reconciliation = %#v, want zero", reconciliation)
	}
	assertZeroAppStatusCommit(t, commit)
}

func assertZeroAppStatusCommit(
	t *testing.T,
	commit state.RuntimeStatusCommitCandidate,
) {
	t.Helper()
	if commit.Committed() ||
		commit.SourceReconciliationDigest() != "" ||
		commit.BaselineDigest() != "" ||
		commit.SourceDiscoveryDigest() != "" ||
		commit.EventCount() != 0 ||
		len(commit.Events()) != 0 ||
		commit.CommitDigest() != "" {
		t.Fatalf("commit = %#v, want zero", commit)
	}
}
