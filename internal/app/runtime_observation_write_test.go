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

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var errRuntimeObservationWriteCommitSentinel = errors.New(
	"runtime observation write commit rejected",
)

func TestRunRuntimeObservationWriteOncePrevalidatesContextAndPlanning(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "observation-prevalidate", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(
		t, previous, "observation-prevalidate",
	)
	validDiscovery := &recordingRuntimeDiscoveryCommitter{}
	validStatus := &recordingRuntimeStatusCommitter{}

	t.Run("nil context", func(t *testing.T) {
		plan, discovery, reconciliation, status, err :=
			RunRuntimeObservationWriteOnce(
				nil, projected, current, validDiscovery, validStatus,
			)
		if !errors.Is(err, ErrInvalidRuntimeObservationWriteRun) {
			t.Fatalf("error = %v, want invalid run", err)
		}
		assertZeroRuntimeObservationWriteRun(
			t, plan, discovery, reconciliation, status,
		)
	})

	for _, test := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "canceled", ctx: canceledAppDiscoveryContext(), want: context.Canceled},
		{name: "deadline", ctx: expiredAppDiscoveryContext(), want: context.DeadlineExceeded},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					test.ctx, projected, current, validDiscovery, validStatus,
				)
			if err != test.want {
				t.Fatalf("error = %v, want exact %v", err, test.want)
			}
			assertZeroRuntimeObservationWriteRun(
				t, plan, discovery, reconciliation, status,
			)
		})
	}

	invalidProjection := cloneProjectedRuntimeSnapshot(projected)
	record := invalidProjection.RuntimeInstances["runtime.observation-prevalidate.a"]
	record.DiscoveryEventID = ""
	invalidProjection.RuntimeInstances[record.ID] = record
	identityDrift := appStatusSnapshot(
		t, "observation-prevalidate", 1,
		loomruntime.RuntimeOffline, "device.other",
	)
	for _, test := range []struct {
		name      string
		projected projection.Snapshot
		current   loomruntime.RuntimeDiscoverySnapshot
		want      error
	}{
		{
			name: "invalid projection", projected: invalidProjection,
			current: current,
			want:    projection.ErrInvalidRuntimeStatusBaselineProjection,
		},
		{
			name: "invalid current", projected: projected,
			current: loomruntime.RuntimeDiscoverySnapshot{},
			want:    loomruntime.ErrInvalidRuntimeStatusSource,
		},
		{
			name: "stable identity drift", projected: projected,
			current: identityDrift,
			want:    loomruntime.ErrRuntimeStatusIdentityDrift,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			}
			statusCommitter := &recordingRuntimeStatusCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			}
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					context.Background(), test.projected, test.current,
					discoveryCommitter, statusCommitter,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroRuntimeObservationWriteRun(
				t, plan, discovery, reconciliation, status,
			)
			if discoveryCommitter.calls != 0 || statusCommitter.calls != 0 {
				t.Fatalf(
					"committer calls = discovery:%d status:%d, want zero",
					discoveryCommitter.calls, statusCommitter.calls,
				)
			}
		})
	}
	if validDiscovery.calls != 0 || validStatus.calls != 0 {
		t.Fatalf(
			"context prevalidation calls = discovery:%d status:%d, want zero",
			validDiscovery.calls, validStatus.calls,
		)
	}
}

func TestRunRuntimeObservationWriteOnceNoneAllowsPathScopedNilDependencies(
	t *testing.T,
) {
	unchanged := appDiscoverySnapshot(t, "observation-none", 1)
	projected := appProjectedRuntimeSnapshot(t, unchanged, "observation-none")
	empty, err := loomruntime.DiscoverRuntime(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var typedNilStatus *recordingRuntimeStatusCommitter

	for _, test := range []struct {
		name      string
		projected projection.Snapshot
		current   loomruntime.RuntimeDiscoverySnapshot
		discovery RuntimeDiscoveryCommitter
		status    RuntimeStatusCommitter
	}{
		{
			name: "empty", current: empty,
			discovery: nil, status: nil,
		},
		{
			name: "unchanged", projected: projected, current: unchanged,
			discovery: typedNilDiscovery, status: typedNilStatus,
		},
		{
			name: "absence only", projected: projected, current: empty,
			discovery: &recordingRuntimeDiscoveryCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			},
			status: &recordingRuntimeStatusCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			want, err := PlanRuntimeObservationWrite(
				context.Background(), test.projected, test.current,
			)
			if err != nil {
				t.Fatal(err)
			}
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					context.Background(), test.projected, test.current,
					test.discovery, test.status,
				)
			if err != nil {
				t.Fatal(err)
			}
			if plan != want || plan.Kind() != RuntimeObservationWriteNone {
				t.Fatalf("plan = %#v, want exact %#v", plan, want)
			}
			assertZeroAppDiscoveryCandidate(t, discovery)
			assertZeroAppStatusRun(t, reconciliation, status)
			if committer, ok :=
				test.discovery.(*recordingRuntimeDiscoveryCommitter); ok &&
				committer != nil &&
				committer.calls != 0 {
				t.Fatalf("discovery calls = %d, want zero", committer.calls)
			}
			if committer, ok :=
				test.status.(*recordingRuntimeStatusCommitter); ok &&
				committer != nil &&
				committer.calls != 0 {
				t.Fatalf("status calls = %d, want zero", committer.calls)
			}
		})
	}
}

func TestRunRuntimeObservationWriteOnceDiscoveryOwnsInventoryAndMixedPaths(
	t *testing.T,
) {
	_, statusChanged, previous := appStatusSources(
		t, "observation-discovery", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(
		t, previous, "observation-discovery",
	)
	mixed := runtimeWritePlanSnapshotFrom(
		t, statusChanged, "probe.observation-discovery.mixed",
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "changed inventory"
			return observations
		},
	)
	currentOnly := appDiscoverySnapshot(t, "observation-current-only", 1)
	inventoryOnly := runtimeWritePlanSnapshotFrom(
		t, previous, "probe.observation-discovery.inventory",
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.ExecutableVersion = "2.0.0"
			return observations
		},
	)

	for _, test := range []struct {
		name      string
		projected projection.Snapshot
		current   loomruntime.RuntimeDiscoverySnapshot
		wantMixed bool
	}{
		{name: "current only", current: currentOnly},
		{name: "inventory only", projected: projected, current: inventoryOnly},
		{name: "mixed inventory and status", projected: projected, current: mixed, wantMixed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			expectedCommit := mintAppDiscoveryCandidate(
				t, context.Background(), test.current, "observation-"+test.name,
			)
			discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
				commit: func(
					_ context.Context,
					got loomruntime.RuntimeDiscoverySnapshot,
				) (state.RuntimeDiscoveryCommitCandidate, error) {
					if got.Digest() != test.current.Digest() ||
						!reflect.DeepEqual(
							got.Observations(), test.current.Observations(),
						) {
						t.Fatal("discovery committer did not receive exact snapshot")
					}
					return expectedCommit, nil
				},
			}
			statusCommitter := &recordingRuntimeStatusCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			}
			wantPlan, err := PlanRuntimeObservationWrite(
				context.Background(), test.projected, test.current,
			)
			if err != nil {
				t.Fatal(err)
			}
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					context.Background(), test.projected, test.current,
					discoveryCommitter, statusCommitter,
				)
			if err != nil {
				t.Fatal(err)
			}
			if plan != wantPlan ||
				plan.Kind() != RuntimeObservationWriteDiscovery ||
				discovery.CommitDigest() != expectedCommit.CommitDigest() ||
				!reflect.DeepEqual(discovery.Events(), expectedCommit.Events()) {
				t.Fatalf("unexpected discovery result")
			}
			if test.wantMixed && plan.StatusTransitionCount() == 0 {
				t.Fatal("mixed observation did not retain status-transition fact")
			}
			assertZeroAppStatusRun(t, reconciliation, status)
			if discoveryCommitter.calls != 1 || statusCommitter.calls != 0 {
				t.Fatalf(
					"calls = discovery:%d status:%d, want 1/0",
					discoveryCommitter.calls, statusCommitter.calls,
				)
			}
		})
	}
}

func TestRunRuntimeObservationWriteOnceStatusDelegatesExactS2W31Path(
	t *testing.T,
) {
	_, current, previous := appStatusSources(
		t, "observation-status", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "observation-status")
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var received loomruntime.RuntimeStatusReconciliationCandidate
	statusCommitter := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			received = candidate
			return mintAppStatusCommit(t, ctx, candidate, "observation-status"), nil
		},
	}
	wantPlan, err := PlanRuntimeObservationWrite(
		context.Background(), projected, current,
	)
	if err != nil {
		t.Fatal(err)
	}
	plan, discovery, reconciliation, status, err :=
		RunRuntimeObservationWriteOnce(
			context.Background(), projected, current,
			typedNilDiscovery, statusCommitter,
		)
	if err != nil {
		t.Fatal(err)
	}
	if plan != wantPlan || plan.Kind() != RuntimeObservationWriteStatus {
		t.Fatalf("plan = %#v, want exact %#v", plan, wantPlan)
	}
	assertZeroAppDiscoveryCandidate(t, discovery)
	assertSameAppStatusReconciliation(t, reconciliation, received)
	if !status.Committed() ||
		status.SourceReconciliationDigest() != reconciliation.CandidateDigest() ||
		status.EventCount() != 2 ||
		statusCommitter.calls != 1 {
		t.Fatalf("unexpected status result/calls")
	}
}

func TestRunRuntimeObservationWriteOnceRejectsSelectedPathFailuresWithoutRetry(
	t *testing.T,
) {
	discoveryCurrent := appDiscoverySnapshot(t, "observation-failure-discovery", 1)
	_, statusCurrent, statusPrevious := appStatusSources(
		t, "observation-failure-status", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	statusProjected := appProjectedRuntimeSnapshot(
		t, statusPrevious, "observation-failure-status",
	)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var typedNilStatus *recordingRuntimeStatusCommitter

	t.Run("missing selected discovery dependency", func(t *testing.T) {
		statusCommitter := &recordingRuntimeStatusCommitter{
			err: errRuntimeObservationWriteCommitSentinel,
		}
		plan, discovery, reconciliation, status, err :=
			RunRuntimeObservationWriteOnce(
				context.Background(), projection.Snapshot{}, discoveryCurrent,
				typedNilDiscovery, statusCommitter,
			)
		if !errors.Is(err, ErrInvalidRuntimeObservationWriteRun) {
			t.Fatalf("error = %v, want invalid run", err)
		}
		assertZeroRuntimeObservationWriteRun(
			t, plan, discovery, reconciliation, status,
		)
		if statusCommitter.calls != 0 {
			t.Fatalf("opposite status calls = %d, want zero", statusCommitter.calls)
		}
	})

	t.Run("missing selected status dependency", func(t *testing.T) {
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			err: errRuntimeObservationWriteCommitSentinel,
		}
		plan, discovery, reconciliation, status, err :=
			RunRuntimeObservationWriteOnce(
				context.Background(), statusProjected, statusCurrent,
				discoveryCommitter, typedNilStatus,
			)
		if !errors.Is(err, ErrInvalidRuntimeObservationWriteRun) {
			t.Fatalf("error = %v, want invalid run", err)
		}
		assertZeroRuntimeObservationWriteRun(
			t, plan, discovery, reconciliation, status,
		)
		if discoveryCommitter.calls != 0 {
			t.Fatalf(
				"opposite discovery calls = %d, want zero",
				discoveryCommitter.calls,
			)
		}
	})

	for _, test := range []struct {
		name      string
		committer *recordingRuntimeDiscoveryCommitter
		want      error
	}{
		{
			name: "discovery source error",
			committer: &recordingRuntimeDiscoveryCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			},
			want: errRuntimeObservationWriteCommitSentinel,
		},
		{
			name:      "discovery result mismatch",
			committer: &recordingRuntimeDiscoveryCommitter{},
			want:      ErrRuntimeDiscoveryCommitResultMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			statusCommitter := &recordingRuntimeStatusCommitter{
				err: errors.New("opposite path must not run"),
			}
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					context.Background(), projection.Snapshot{},
					discoveryCurrent, test.committer, statusCommitter,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroRuntimeObservationWriteRun(
				t, plan, discovery, reconciliation, status,
			)
			if test.committer.calls != 1 || statusCommitter.calls != 0 {
				t.Fatalf(
					"calls = selected:%d opposite:%d, want 1/0",
					test.committer.calls, statusCommitter.calls,
				)
			}
		})
	}

	for _, test := range []struct {
		name      string
		committer *recordingRuntimeStatusCommitter
		want      error
	}{
		{
			name: "status source error",
			committer: &recordingRuntimeStatusCommitter{
				err: errRuntimeObservationWriteCommitSentinel,
			},
			want: errRuntimeObservationWriteCommitSentinel,
		},
		{
			name:      "status result mismatch",
			committer: &recordingRuntimeStatusCommitter{},
			want:      ErrRuntimeStatusCommitResultMismatch,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
				err: errors.New("opposite path must not run"),
			}
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					context.Background(), statusProjected, statusCurrent,
					discoveryCommitter, test.committer,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroRuntimeObservationWriteRun(
				t, plan, discovery, reconciliation, status,
			)
			if test.committer.calls != 1 || discoveryCommitter.calls != 0 {
				t.Fatalf(
					"calls = selected:%d opposite:%d, want 1/0",
					test.committer.calls, discoveryCommitter.calls,
				)
			}
		})
	}

	t.Run("canceled after discovery commit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		valid := mintAppDiscoveryCandidate(
			t, context.Background(), discoveryCurrent, "observation-canceled",
		)
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			commit: func(
				context.Context,
				loomruntime.RuntimeDiscoverySnapshot,
			) (state.RuntimeDiscoveryCommitCandidate, error) {
				cancel()
				return valid, nil
			},
		}
		plan, discovery, reconciliation, status, err :=
			RunRuntimeObservationWriteOnce(
				ctx, projection.Snapshot{}, discoveryCurrent,
				discoveryCommitter, nil,
			)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact canceled", err)
		}
		assertZeroRuntimeObservationWriteRun(
			t, plan, discovery, reconciliation, status,
		)
		if discoveryCommitter.calls != 1 {
			t.Fatalf("discovery calls = %d, want one", discoveryCommitter.calls)
		}
	})

	t.Run("explicit retry is caller owned", func(t *testing.T) {
		valid := mintAppDiscoveryCandidate(
			t, context.Background(), discoveryCurrent, "observation-retry",
		)
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			candidate: valid,
		}
		var firstPlan RuntimeObservationWritePlanCandidate
		var firstCommit state.RuntimeDiscoveryCommitCandidate
		for attempt := 0; attempt < 2; attempt++ {
			plan, discovery, reconciliation, status, err :=
				RunRuntimeObservationWriteOnce(
					context.Background(), projection.Snapshot{},
					discoveryCurrent, discoveryCommitter, nil,
				)
			if err != nil {
				t.Fatal(err)
			}
			assertZeroAppStatusRun(t, reconciliation, status)
			if attempt == 0 {
				firstPlan, firstCommit = plan, discovery
			} else if plan != firstPlan ||
				discovery.CommitDigest() != firstCommit.CommitDigest() ||
				!reflect.DeepEqual(discovery.Events(), firstCommit.Events()) {
				t.Fatal("exact caller retry changed result")
			}
		}
		if discoveryCommitter.calls != 2 {
			t.Fatalf("calls = %d, want exactly one per invocation", discoveryCommitter.calls)
		}
	})
}

func TestRunRuntimeObservationWriteOnceIsMutationIsolated(t *testing.T) {
	current := appDiscoverySnapshot(t, "observation-mutation", 1)
	originalObservations := current.Observations()
	expectedCommit := mintAppDiscoveryCandidate(
		t, context.Background(), current, "observation-mutation",
	)
	discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
		commit: func(
			_ context.Context,
			got loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitCandidate, error) {
			observations := got.Observations()
			observations[0].Instance.ObservedCapabilities[0] = "mutated"
			observations[0].ModelIDs[0] = "mutated"
			if reflect.DeepEqual(observations, got.Observations()) {
				t.Fatal("committer snapshot accessor leaked mutation")
			}
			return expectedCommit, nil
		},
	}
	plan, discovery, reconciliation, status, err :=
		RunRuntimeObservationWriteOnce(
			context.Background(), projection.Snapshot{}, current,
			discoveryCommitter, nil,
		)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(current.Observations(), originalObservations) {
		t.Fatal("source discovery snapshot changed")
	}
	if plan.SourceDiscoveryDigest() != current.Digest() {
		t.Fatal("plan lost exact discovery provenance")
	}
	events := discovery.Events()
	events[0].PayloadJSON[0] ^= 0xff
	if reflect.DeepEqual(events, discovery.Events()) {
		t.Fatal("returned discovery Event accessor leaked mutation")
	}
	assertZeroAppStatusRun(t, reconciliation, status)
}

func TestRunRuntimeObservationWriteOnceRealSQLiteDiscoveryPriorityThenStatus(
	t *testing.T,
) {
	ctx := context.Background()
	previous := appDiscoverySnapshot(t, "observation-sqlite", 1)
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx, store, previous, appDiscoveryCommitInput(previous, "observation-prior"),
	); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}

	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, "probe.observation-sqlite.mixed",
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "observation sqlite changed"
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	discoveryProvider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			input := appDiscoveryCommitInput(mixed, "observation-mixed")
			for index := range input.Events {
				input.Events[index].Seq = 2
			}
			return input, nil
		},
	}
	discoveryAdapter, err := NewPreparedRuntimeDiscoveryCommitter(
		store, discoveryProvider,
	)
	if err != nil {
		t.Fatal(err)
	}
	statusPoison := &recordingRuntimeStatusCommitter{
		err: errors.New("mixed path must not write status"),
	}
	mixedProjected := readModel.Snapshot()
	var firstMixedPlan RuntimeObservationWritePlanCandidate
	var firstMixedCommit state.RuntimeDiscoveryCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		plan, discovery, reconciliation, status, err :=
			RunRuntimeObservationWriteOnce(
				ctx, mixedProjected, mixed, discoveryAdapter, statusPoison,
			)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 {
			t.Fatalf("mixed plan = %#v, want discovery with status fact", plan)
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if attempt == 0 {
			firstMixedPlan, firstMixedCommit = plan, discovery
		} else if plan != firstMixedPlan ||
			discovery.CommitDigest() != firstMixedCommit.CommitDigest() ||
			!reflect.DeepEqual(discovery.Events(), firstMixedCommit.Events()) {
			t.Fatal("mixed exact retry changed committed facts")
		}
	}
	if discoveryProvider.calls != 2 || statusPoison.calls != 0 {
		t.Fatalf(
			"mixed calls = discovery provider:%d status:%d, want 2/0",
			discoveryProvider.calls, statusPoison.calls,
		)
	}
	if got := appDiscoveryEventCount(t, db); got != 2 {
		t.Fatalf("event rows after mixed retry = %d, want 2", got)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}

	statusOnly := runtimeWritePlanSnapshotFrom(
		t, mixed, "probe.observation-sqlite.mixed",
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.Status = loomruntime.RuntimeOnline
			return observations
		},
	)
	statusProvider := &recordingRuntimeStatusCommitInputProvider{
		prepare: func(
			_ context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitInput, error) {
			return appStatusCommitInput(candidate, "observation-status"), nil
		},
	}
	statusAdapter, err := NewPreparedRuntimeStatusCommitter(store, statusProvider)
	if err != nil {
		t.Fatal(err)
	}
	discoveryPoison := &recordingRuntimeDiscoveryCommitter{
		err: errors.New("status path must not write discovery"),
	}
	statusProjected := readModel.Snapshot()
	var firstStatusPlan RuntimeObservationWritePlanCandidate
	var firstReconciliation loomruntime.RuntimeStatusReconciliationCandidate
	var firstStatusCommit state.RuntimeStatusCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		plan, discovery, reconciliation, status, err :=
			RunRuntimeObservationWriteOnce(
				ctx, statusProjected, statusOnly, discoveryPoison, statusAdapter,
			)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteStatus {
			t.Fatalf("status-only plan = %#v, want status", plan)
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
		if attempt == 0 {
			firstStatusPlan = plan
			firstReconciliation = reconciliation
			firstStatusCommit = status
		} else {
			if plan != firstStatusPlan {
				t.Fatal("status exact retry changed plan")
			}
			assertSameAppStatusReconciliation(
				t, reconciliation, firstReconciliation,
			)
			if status.CommitDigest() != firstStatusCommit.CommitDigest() ||
				!reflect.DeepEqual(status.Events(), firstStatusCommit.Events()) {
				t.Fatal("status exact retry changed committed facts")
			}
		}
	}
	if statusProvider.calls != 2 || discoveryPoison.calls != 0 {
		t.Fatalf(
			"status calls = provider:%d discovery:%d, want 2/0",
			statusProvider.calls, discoveryPoison.calls,
		)
	}
	if got := appDiscoveryEventCount(t, db); got != 3 {
		t.Fatalf("final event rows = %d, want 3", got)
	}
	rows, err := db.Query(
		`SELECT event_type FROM events ORDER BY stream_id, seq`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var eventTypes []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatal(err)
		}
		eventTypes = append(eventTypes, eventType)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{
		"RuntimeInstanceDiscovered",
		"RuntimeInstanceDiscovered",
		"RuntimeInstanceStatusChanged",
	}; !reflect.DeepEqual(eventTypes, want) {
		t.Fatalf("event types = %#v, want %#v", eventTypes, want)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	finalRecord := readModel.Snapshot().
		RuntimeInstances["runtime.observation-sqlite.a"]
	if finalRecord.Status != string(loomruntime.RuntimeOnline) ||
		finalRecord.DiscoverySequence != 2 ||
		finalRecord.StatusSequence != 3 {
		t.Fatalf("final projected Runtime = %#v", finalRecord)
	}
}

func TestRunRuntimeObservationWriteOnceStaticBoundary(t *testing.T) {
	const filename = "runtime_observation_write.go"
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read product: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		t.Fatalf("parse product: %v", err)
	}
	allowed := map[string]bool{
		"context":                             true,
		"errors":                              true,
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
	forbiddenSelectors := map[string]bool{
		"DiscoverRuntime":                true,
		"DiscoverConfiguredRuntimes":     true,
		"Rebuild":                        true,
		"BuildRuntimeStatusBaselines":    true,
		"CommitRuntimeDiscoverySnapshot": true,
		"CommitRuntimeStatusTransitions": true,
		"Append":                         true,
		"AppendBatch":                    true,
		"PrepareRuntimeDiscoveryCommit":  true,
		"PrepareRuntimeStatusCommit":     true,
		"NewTicker":                      true,
		"Sleep":                          true,
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt:
			t.Error("product contains forbidden go statement")
		case *ast.ForStmt, *ast.RangeStmt:
			t.Error("product contains forbidden retry/iteration statement")
		case *ast.CallExpr:
			if identifier, ok := typed.Fun.(*ast.Ident); ok {
				switch identifier.Name {
				case "make", "new", "append":
					t.Errorf(
						"product contains forbidden allocation call %q",
						identifier.Name,
					)
				}
			}
		case *ast.SelectorExpr:
			if forbiddenSelectors[typed.Sel.Name] {
				t.Errorf("product contains forbidden selector %q", typed.Sel.Name)
			}
		}
		return true
	})
	product := string(source)
	if strings.Count(product, "PlanRuntimeObservationWrite(") != 1 {
		t.Fatal("product must contain exactly one planning call")
	}
	if strings.Count(product, "CommitRuntimeDiscovery(") != 1 {
		t.Fatal("product must contain exactly one discovery commit call")
	}
	if strings.Count(
		product, "RunProjectedRuntimeStatusReconciliationOnce(",
	) != 1 {
		t.Fatal("product must contain exactly one S2-W31 call")
	}
	for _, marker := range []string{
		"internal/journal", "internal/runtime/discoveryscan",
		"RuntimeInstanceDiscovered", "RuntimeInstanceStatusChanged",
		"IdempotencyKey", "EventID", "EmittedAt", "NewTicker", "Sleep(",
		"os.", "exec.", "net.", "http.", "sqlite", "daemon",
		"RuntimeProfile", "AgentGrant", "WorkItem", "Bridge",
	} {
		if strings.Contains(product, marker) {
			t.Errorf("forbidden product marker %q", marker)
		}
	}
}

func assertZeroRuntimeObservationWriteRun(
	t *testing.T,
	plan RuntimeObservationWritePlanCandidate,
	discovery state.RuntimeDiscoveryCommitCandidate,
	reconciliation loomruntime.RuntimeStatusReconciliationCandidate,
	status state.RuntimeStatusCommitCandidate,
) {
	t.Helper()
	assertZeroRuntimeObservationWritePlan(t, plan)
	assertZeroAppDiscoveryCandidate(t, discovery)
	assertZeroAppStatusRun(t, reconciliation, status)
}
