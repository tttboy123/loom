package app

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var errProjectedRuntimeStatusCommitSentinel = errors.New(
	"projected Runtime status commit rejected",
)

func TestRunProjectedRuntimeStatusReconciliationOncePrevalidatesInputs(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "projected-prevalidate", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "projected-prevalidate")
	valid := &recordingRuntimeStatusCommitter{}
	var typedNil *recordingRuntimeStatusCommitter
	tests := []struct {
		name      string
		ctx       context.Context
		committer RuntimeStatusCommitter
		want      error
	}{
		{name: "nil_context", committer: valid, want: ErrInvalidProjectedRuntimeStatusRun},
		{
			name: "nil_committer", ctx: context.Background(),
			want: ErrInvalidProjectedRuntimeStatusRun,
		},
		{
			name: "typed_nil_committer", ctx: context.Background(),
			committer: typedNil, want: ErrInvalidProjectedRuntimeStatusRun,
		},
		{
			name: "canceled", ctx: canceledAppDiscoveryContext(),
			committer: valid, want: context.Canceled,
		},
		{
			name: "deadline", ctx: expiredAppDiscoveryContext(),
			committer: valid, want: context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciliation, commit, err :=
				RunProjectedRuntimeStatusReconciliationOnce(
					test.ctx, projected, current, test.committer,
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

func TestRunProjectedRuntimeStatusReconciliationOnceEmptyAndCurrentOnlySkipCommit(
	t *testing.T,
) {
	_, current, _ := appStatusSources(
		t, "projected-current-only", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	committer := &recordingRuntimeStatusCommitter{
		err: errors.New("must not commit"),
	}
	reconciliation, commit, err :=
		RunProjectedRuntimeStatusReconciliationOnce(
			context.Background(), projection.Snapshot{}, current, committer,
		)
	if err != nil {
		t.Fatal(err)
	}
	if !reconciliation.Reconciled() ||
		reconciliation.TransitionCount() != 0 ||
		reconciliation.SourceDiscoveryDigest() != current.Digest() {
		t.Fatalf("reconciliation = %#v, want valid zero transition", reconciliation)
	}
	assertZeroAppStatusCommit(t, commit)
	if committer.calls != 0 {
		t.Fatalf("committer calls = %d, want 0", committer.calls)
	}
}

func TestRunProjectedRuntimeStatusReconciliationOnceDelegatesExactBaseline(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "projected-delegate", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "projected-delegate")
	original := cloneProjectedRuntimeSnapshot(projected)
	committer := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			if candidate.TransitionCount() != 2 {
				t.Fatalf("transition count = %d, want 2", candidate.TransitionCount())
			}
			for _, transition := range candidate.Transitions() {
				if transition.PreviousEventID !=
					"event.projected-delegate."+transition.RuntimeInstanceID ||
					transition.PreviousSequence != 1 {
					t.Fatalf("transition provenance = %#v", transition)
				}
			}
			return mintAppStatusCommit(t, ctx, candidate, "projected"), nil
		},
	}
	reconciliation, commit, err :=
		RunProjectedRuntimeStatusReconciliationOnce(
			context.Background(), projected, current, committer,
		)
	if err != nil {
		t.Fatal(err)
	}
	if reconciliation.TransitionCount() != 2 ||
		commit.EventCount() != 2 ||
		committer.calls != 1 {
		t.Fatalf("unexpected result/calls")
	}
	if !reflect.DeepEqual(projected, original) {
		t.Fatal("projection Snapshot was mutated")
	}
}

func TestRunProjectedRuntimeStatusReconciliationOnceSelectsLatestProjectionProvenance(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "projected-provenance", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	discoveryOnly := appProjectedRuntimeSnapshot(
		t, previous, "projected-provenance",
	)
	runtimeID := "runtime.projected-provenance.a"
	discoveryEventID := discoveryOnly.RuntimeInstances[runtimeID].DiscoveryEventID
	tests := []struct {
		name       string
		projected  projection.Snapshot
		wantEvent  string
		wantSeq    int64
		commitName string
	}{
		{
			name: "first_status",
			projected: appProjectedRuntimeSnapshotWithStatus(
				discoveryOnly, runtimeID,
				discoveryEventID, 1,
				"event.status.first."+runtimeID, 2,
			),
			wantEvent: "event.status.first." + runtimeID, wantSeq: 2,
			commitName: "first-status",
		},
		{
			name: "consecutive_status",
			projected: appProjectedRuntimeSnapshotWithStatus(
				discoveryOnly, runtimeID,
				"event.status.previous."+runtimeID, 4,
				"event.status.current."+runtimeID, 5,
			),
			wantEvent: "event.status.current." + runtimeID, wantSeq: 5,
			commitName: "consecutive-status",
		},
		{
			name: "rediscovery_resets_status",
			projected: func() projection.Snapshot {
				snapshot := cloneProjectedRuntimeSnapshot(discoveryOnly)
				record := snapshot.RuntimeInstances[runtimeID]
				record.DiscoverySequence = 6
				snapshot.RuntimeInstances[runtimeID] = record
				return snapshot
			}(),
			wantEvent: discoveryEventID, wantSeq: 6,
			commitName: "rediscovery",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			committer := &recordingRuntimeStatusCommitter{
				commit: func(
					ctx context.Context,
					candidate loomruntime.RuntimeStatusReconciliationCandidate,
				) (state.RuntimeStatusCommitCandidate, error) {
					transition := candidate.Transitions()[0]
					if transition.PreviousEventID != test.wantEvent ||
						transition.PreviousSequence != test.wantSeq {
						t.Fatalf(
							"transition provenance = %#v, want %q at %d",
							transition, test.wantEvent, test.wantSeq,
						)
					}
					return mintAppStatusCommit(
						t, ctx, candidate, test.commitName,
					), nil
				},
			}

			reconciliation, commit, err :=
				RunProjectedRuntimeStatusReconciliationOnce(
					context.Background(), test.projected, current, committer,
				)
			if err != nil {
				t.Fatal(err)
			}
			if reconciliation.TransitionCount() != 1 ||
				commit.EventCount() != 1 ||
				committer.calls != 1 {
				t.Fatalf(
					"result/calls = %d/%d/%d, want 1/1/1",
					reconciliation.TransitionCount(),
					commit.EventCount(),
					committer.calls,
				)
			}
		})
	}
}

func TestRunProjectedRuntimeStatusReconciliationOnceRejectsInvalidProjection(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "projected-invalid", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	base := appProjectedRuntimeSnapshot(t, previous, "projected-invalid")
	runtimeID := "runtime.projected-invalid.a"
	record := base.RuntimeInstances[runtimeID]
	base = appProjectedRuntimeSnapshotWithStatus(
		base, runtimeID,
		record.DiscoveryEventID, record.DiscoverySequence,
		"event.status."+runtimeID, record.DiscoverySequence+1,
	)
	tests := []struct {
		name   string
		mutate func(*projection.Snapshot)
	}{
		{
			name: "oversized_map",
			mutate: func(snapshot *projection.Snapshot) {
				template := snapshot.RuntimeInstances[runtimeID]
				snapshot.RuntimeInstances =
					make(map[string]projection.RuntimeInstance, 33)
				for index := 0; index < 33; index++ {
					id := fmt.Sprintf("runtime.projected-invalid.%02d", index)
					entry := cloneProjectedRuntimeRecord(template)
					entry.ID = id
					entry.DiscoveryEventID = "event.discovery." + id
					entry.StatusPreviousEventID = entry.DiscoveryEventID
					entry.StatusEventID = "event.status." + id
					snapshot.RuntimeInstances[id] = entry
				}
			},
		},
		{
			name: "map_key_mismatch",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				delete(snapshot.RuntimeInstances, runtimeID)
				snapshot.RuntimeInstances["runtime.other"] = entry
			},
		},
		{
			name: "invalid_runtime_core",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.Status = "invalid"
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "invalid_runtime_identity",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.DeviceID = ""
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "noncanonical_models",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.ModelIDs = []string{"model/z", "model/a"}
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "invalid_discovery_digest",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.DiscoveryDigest = strings.Repeat("A", 64)
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "invalid_discovery_time",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.DiscoveredAt = time.Time{}
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "invalid_discovery_event_provenance",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.DiscoveryEventID = ""
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "partial_status_group",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.StatusReconciliationID = ""
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "invalid_status_sequence",
			mutate: func(snapshot *projection.Snapshot) {
				entry := snapshot.RuntimeInstances[runtimeID]
				entry.StatusSequence++
				snapshot.RuntimeInstances[runtimeID] = entry
			},
		},
		{
			name: "cross_record_event_identity_reuse",
			mutate: func(snapshot *projection.Snapshot) {
				left := snapshot.RuntimeInstances[runtimeID]
				right := cloneProjectedRuntimeRecord(left)
				right.ID = "runtime.projected-invalid.other"
				right.DiscoveryEventID = left.DiscoveryEventID
				right.StatusPreviousEventID = right.DiscoveryEventID
				right.StatusEventID = "event.status." + right.ID
				snapshot.RuntimeInstances[right.ID] = right
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projected := cloneProjectedRuntimeSnapshot(base)
			test.mutate(&projected)
			committer := &recordingRuntimeStatusCommitter{}
			reconciliation, commit, err :=
				RunProjectedRuntimeStatusReconciliationOnce(
					context.Background(), projected, current, committer,
				)
			if err != projection.ErrInvalidRuntimeStatusBaselineProjection {
				t.Fatalf(
					"error = %v, want exact invalid projection sentinel", err,
				)
			}
			assertZeroAppStatusRun(t, reconciliation, commit)
			if committer.calls != 0 {
				t.Fatalf("committer calls = %d, want 0", committer.calls)
			}
		})
	}
}

func TestRunProjectedRuntimeStatusReconciliationOncePropagatesS2W29Failures(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "projected-propagation", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(
		t, previous, "projected-propagation",
	)
	tests := []struct {
		name      string
		ctx       context.Context
		current   loomruntime.RuntimeDiscoverySnapshot
		committer *recordingRuntimeStatusCommitter
		want      error
		wantCalls int
	}{
		{
			name: "invalid_discovery", ctx: context.Background(),
			current:   loomruntime.RuntimeDiscoverySnapshot{},
			committer: &recordingRuntimeStatusCommitter{},
			want:      loomruntime.ErrInvalidRuntimeStatusSource,
		},
		{
			name: "stable_identity_drift", ctx: context.Background(),
			current: appStatusSnapshot(
				t, "projected-propagation", 1,
				loomruntime.RuntimeOffline, "device.other",
			),
			committer: &recordingRuntimeStatusCommitter{},
			want:      loomruntime.ErrRuntimeStatusIdentityDrift,
		},
		{
			name: "committer_sentinel", ctx: context.Background(),
			current: current,
			committer: &recordingRuntimeStatusCommitter{
				err: errProjectedRuntimeStatusCommitSentinel,
			},
			want: errProjectedRuntimeStatusCommitSentinel, wantCalls: 1,
		},
		{
			name: "commit_result_mismatch", ctx: context.Background(),
			current:   current,
			committer: &recordingRuntimeStatusCommitter{},
			want:      ErrRuntimeStatusCommitResultMismatch, wantCalls: 1,
		},
		{
			name:      "context_after_baseline",
			ctx:       &cancelAfterErrCallsContext{cancelAt: 2},
			current:   current,
			committer: &recordingRuntimeStatusCommitter{},
			want:      context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciliation, commit, err :=
				RunProjectedRuntimeStatusReconciliationOnce(
					test.ctx, projected, test.current, test.committer,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroAppStatusRun(t, reconciliation, commit)
			if test.committer.calls != test.wantCalls {
				t.Fatalf(
					"committer calls = %d, want %d",
					test.committer.calls, test.wantCalls,
				)
			}
		})
	}
}

func TestRunProjectedRuntimeStatusReconciliationOnceIsMutationIsolated(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "projected-mutation", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(
		t, previous, "projected-mutation",
	)
	original := cloneProjectedRuntimeSnapshot(projected)
	committer := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			received := candidate.Transitions()
			received[0].RuntimeInstanceID = "mutated"
			if candidate.Transitions()[0].RuntimeInstanceID == "mutated" {
				t.Fatal("committer-received reconciliation accessor leaked mutation")
			}
			return mintAppStatusCommit(t, ctx, candidate, "projected-mutation"), nil
		},
	}

	reconciliation, commit, err :=
		RunProjectedRuntimeStatusReconciliationOnce(
			context.Background(), projected, current, committer,
		)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(projected, original) {
		t.Fatal("projection Snapshot or nested slices were mutated")
	}
	transitions := reconciliation.Transitions()
	transitions[0].SourceProbeID = "mutated"
	if reflect.DeepEqual(transitions, reconciliation.Transitions()) {
		t.Fatal("returned reconciliation accessor leaked mutation")
	}
	events := commit.Events()
	events[0].PayloadJSON[0] ^= 0xff
	if reflect.DeepEqual(events, commit.Events()) {
		t.Fatal("returned commit Event accessor leaked mutation")
	}
}

func TestRunProjectedRuntimeStatusReconciliationOnceRealProjectionRetry(
	t *testing.T,
) {
	ctx := context.Background()
	baseline, current, previous := appStatusSources(
		t, "projected-sqlite", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	_ = baseline
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx, store, previous, appDiscoveryCommitInput(previous, "status-prior"),
	); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	provider := &recordingRuntimeStatusCommitInputProvider{
		prepare: func(
			_ context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitInput, error) {
			return appStatusCommitInput(candidate, "projected-sqlite"), nil
		},
	}
	adapter, err := NewPreparedRuntimeStatusCommitter(store, provider)
	if err != nil {
		t.Fatal(err)
	}
	projected := readModel.Snapshot()

	firstReconciliation, firstCommit, err :=
		RunProjectedRuntimeStatusReconciliationOnce(
			ctx, projected, current, adapter,
		)
	if err != nil {
		t.Fatal(err)
	}
	secondReconciliation, secondCommit, err :=
		RunProjectedRuntimeStatusReconciliationOnce(
			ctx, projected, current, adapter,
		)
	if err != nil {
		t.Fatal(err)
	}
	assertSameAppStatusReconciliation(
		t, firstReconciliation, secondReconciliation,
	)
	if firstCommit.CommitDigest() != secondCommit.CommitDigest() ||
		!reflect.DeepEqual(firstCommit.Events(), secondCommit.Events()) {
		t.Fatal("exact retry changed committed facts")
	}
	if got := appDiscoveryEventCount(t, db); got != 4 {
		t.Fatalf("event row count = %d, want 4", got)
	}
}

func TestRunProjectedRuntimeStatusReconciliationOnceStaticBoundary(t *testing.T) {
	source, err := os.ReadFile("runtime_status_projection.go")
	if err != nil {
		t.Fatalf("read product: %v", err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(), "runtime_status_projection.go", source, 0,
	)
	if err != nil {
		t.Fatalf("parse product: %v", err)
	}
	allowed := map[string]bool{
		"context": true, "errors": true,
		"loom-pi-rebuild/internal/projection": true,
		"loom-pi-rebuild/internal/runtime":    true,
		"loom-pi-rebuild/internal/state":      true,
	}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !allowed[path] {
			t.Fatalf("forbidden product import %q", path)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt:
			t.Error("forbidden go statement")
		case *ast.SelectorExpr:
			if typed.Sel.Name == "Rebuild" ||
				typed.Sel.Name == "CommitRuntimeDiscoverySnapshot" ||
				typed.Sel.Name == "DiscoverRuntime" {
				t.Errorf("forbidden selector %q", typed.Sel.Name)
			}
		}
		return true
	})
	for _, marker := range []string{
		"RuntimeInstanceDiscovered", "RuntimeInstanceStatusChanged",
		"AppendBatch", "IdempotencyKey", "EventID", "EmittedAt",
	} {
		if strings.Contains(string(source), marker) {
			t.Errorf("forbidden product marker %q", marker)
		}
	}
}

func appProjectedRuntimeSnapshot(
	t *testing.T,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	suffix string,
) projection.Snapshot {
	t.Helper()
	input := appDiscoveryCommitInput(snapshot, suffix)
	result := projection.Snapshot{
		RuntimeInstances: make(map[string]projection.RuntimeInstance),
	}
	for _, observation := range snapshot.Observations() {
		instance := observation.Instance
		result.RuntimeInstances[instance.ID] = projection.RuntimeInstance{
			ID: instance.ID, DeviceID: instance.DeviceID,
			AdapterType: instance.AdapterType, DisplayName: instance.DisplayName,
			ExecutableVersion: instance.ExecutableVersion, Status: string(instance.Status),
			ObservedCapabilities: append([]string(nil), instance.ObservedCapabilities...),
			Capacity:             instance.Capacity,
			ModelIDs:             append([]string(nil), observation.ModelIDs...),
			DiscoveryDigest:      snapshot.Digest(),
			SourceProbeID:        observation.SourceProbeID,
			DiscoveryID:          input.DiscoveryID, DiscoveredAt: input.EmittedAt,
			DiscoveryEventID:  "event." + suffix + "." + instance.ID,
			DiscoverySequence: 1,
		}
	}
	return result
}

func cloneProjectedRuntimeSnapshot(input projection.Snapshot) projection.Snapshot {
	cloned := projection.Snapshot{
		RuntimeInstances: make(map[string]projection.RuntimeInstance),
	}
	for key, value := range input.RuntimeInstances {
		cloned.RuntimeInstances[key] = cloneProjectedRuntimeRecord(value)
	}
	return cloned
}

func cloneProjectedRuntimeRecord(
	input projection.RuntimeInstance,
) projection.RuntimeInstance {
	input.ObservedCapabilities = append(
		[]string(nil), input.ObservedCapabilities...,
	)
	input.ModelIDs = append([]string(nil), input.ModelIDs...)
	return input
}

func appProjectedRuntimeSnapshotWithStatus(
	input projection.Snapshot,
	runtimeID string,
	previousEventID string,
	previousSequence int64,
	statusEventID string,
	statusSequence int64,
) projection.Snapshot {
	projected := cloneProjectedRuntimeSnapshot(input)
	record := projected.RuntimeInstances[runtimeID]
	record.StatusReconciliationID = "reconciliation." + runtimeID
	record.StatusReconciliationDigest = strings.Repeat("b", 64)
	record.StatusBaselineDigest = strings.Repeat("c", 64)
	record.StatusDiscoveryDigest = strings.Repeat("d", 64)
	record.StatusSourceProbeID = "probe.status." + runtimeID
	record.StatusChangedAt = time.Date(2026, 7, 25, 21, 0, 0, 0, time.UTC)
	record.StatusEventID = statusEventID
	record.StatusSequence = statusSequence
	record.StatusPreviousEventID = previousEventID
	record.StatusPreviousSequence = previousSequence
	projected.RuntimeInstances[runtimeID] = record
	return projected
}
