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

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

func TestPreparedProjectedRuntimeObserverConstructionAndFactoryCopy(
	t *testing.T,
) {
	factory := &appDiscoveryFactory{name: "unused"}
	discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
	statusCommitter := &recordingRuntimeStatusCommitter{}
	observer, err := NewPreparedProjectedRuntimeObserver(
		[]discoveryscan.ProbeFactory{factory},
		nil,
		discoveryCommitter,
		statusCommitter,
	)
	if observer != nil ||
		!errors.Is(err, ErrInvalidPreparedProjectedRuntimeObserver) {
		t.Fatalf("observer/error = %#v/%v", observer, err)
	}
	if factory.calls != 0 ||
		discoveryCommitter.calls != 0 ||
		statusCommitter.calls != 0 {
		t.Fatal("invalid construction invoked a dependency")
	}

	readModel, _, _ := projectedObservationReadModel(
		t, nil, "prepared-copy",
	)
	current := appDiscoverySnapshot(t, "prepared-copy", 1)
	expected := mintAppDiscoveryCandidate(
		t, context.Background(), current, "prepared-copy",
	)
	var trace []string
	boundFactory := configuredObservationFactory(current, &trace)
	factories := []discoveryscan.ProbeFactory{boundFactory}
	discoveryCommitter = &recordingRuntimeDiscoveryCommitter{
		trace: &trace, candidate: expected,
	}
	statusCommitter = &recordingRuntimeStatusCommitter{
		err: errors.New("discovery path must not write status"),
	}
	observer, err = NewPreparedProjectedRuntimeObserver(
		factories, readModel, discoveryCommitter, statusCommitter,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(trace) != 0 ||
		discoveryCommitter.calls != 0 ||
		statusCommitter.calls != 0 {
		t.Fatal("construction performed observation work")
	}

	factories[0] = nil
	factories = append(factories[:0], nil)
	snapshot, plan, discovery, reconciliation, status, err :=
		observer.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() != current.Digest() ||
		plan.Kind() != RuntimeObservationWriteDiscovery ||
		discovery.CommitDigest() != expected.CommitDigest() {
		t.Fatal("observer did not preserve its copied factory binding")
	}
	assertZeroAppStatusRun(t, reconciliation, status)
	if discoveryCommitter.calls != 1 || statusCommitter.calls != 0 {
		t.Fatalf(
			"calls = discovery:%d status:%d, want 1/0",
			discoveryCommitter.calls,
			statusCommitter.calls,
		)
	}
	if want := []string{
		"factory:configured-observation",
		"probe:probe.prepared-copy",
		"commit",
	}; !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace = %#v, want %#v", trace, want)
	}
}

func TestPreparedProjectedRuntimeObserverNilReceiverAndNonePath(t *testing.T) {
	var nilObserver *PreparedProjectedRuntimeObserver
	snapshot, plan, discovery, reconciliation, status, err :=
		nilObserver.RunOnce(context.Background())
	if !errors.Is(err, ErrInvalidPreparedProjectedRuntimeObserver) {
		t.Fatalf("error = %v, want invalid observer", err)
	}
	assertZeroConfiguredRuntimeObservationRun(
		t, snapshot, plan, discovery, reconciliation, status,
	)

	readModel, _, _ := projectedObservationReadModel(
		t, nil, "prepared-none",
	)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var typedNilStatus *recordingRuntimeStatusCommitter
	observer, err := NewPreparedProjectedRuntimeObserver(
		nil, readModel, typedNilDiscovery, typedNilStatus,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, plan, discovery, reconciliation, status, err =
		observer.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() == "" ||
		plan.Kind() != RuntimeObservationWriteNone ||
		plan.SourceDiscoveryDigest() != snapshot.Digest() {
		t.Fatalf("snapshot/plan = %#v/%#v", snapshot, plan)
	}
	assertZeroAppDiscoveryCandidate(t, discovery)
	assertZeroAppStatusRun(t, reconciliation, status)

	snapshot, plan, discovery, reconciliation, status, err =
		observer.RunOnce(nil)
	if !errors.Is(
		err, ErrInvalidProjectedConfiguredRuntimeObservationRun,
	) {
		t.Fatalf("error = %v, want exact S2-W35 validation error", err)
	}
	assertZeroConfiguredRuntimeObservationRun(
		t, snapshot, plan, discovery, reconciliation, status,
	)
}

func TestPreparedProjectedRuntimeObserverDiscoveryAndStatusDelegation(
	t *testing.T,
) {
	t.Run("discovery priority", func(t *testing.T) {
		_, statusChanged, previous := appStatusSources(
			t, "prepared-discovery", 1,
			loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
		)
		readModel, _, _ := projectedObservationReadModel(
			t, &previous, "prepared-discovery",
		)
		mixed := runtimeWritePlanSnapshotFrom(
			t, statusChanged, statusChanged.Observations()[0].SourceProbeID,
			func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
				observations[0].Instance.DisplayName = "prepared inventory changed"
				return observations
			},
		)
		expected := mintAppDiscoveryCandidate(
			t, context.Background(), mixed, "prepared-discovery",
		)
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			candidate: expected,
		}
		statusCommitter := &recordingRuntimeStatusCommitter{
			err: errors.New("mixed path must not write status"),
		}
		observer, err := NewPreparedProjectedRuntimeObserver(
			[]discoveryscan.ProbeFactory{
				configuredObservationFactory(mixed, nil),
			},
			readModel,
			discoveryCommitter,
			statusCommitter,
		)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			observer.RunOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != mixed.Digest() ||
			plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 ||
			discovery.CommitDigest() != expected.CommitDigest() {
			t.Fatal("prepared observer lost discovery priority")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if discoveryCommitter.calls != 1 || statusCommitter.calls != 0 {
			t.Fatal("prepared observer called the wrong writer")
		}
	})

	t.Run("status", func(t *testing.T) {
		_, current, previous := appStatusSources(
			t, "prepared-status", 1,
			loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
		)
		readModel, _, _ := projectedObservationReadModel(
			t, &previous, "prepared-status",
		)
		var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
		statusCommitter := &recordingRuntimeStatusCommitter{
			commit: func(
				ctx context.Context,
				candidate loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitCandidate, error) {
				return mintAppStatusCommit(
					t, ctx, candidate, "prepared-status",
				), nil
			},
		}
		observer, err := NewPreparedProjectedRuntimeObserver(
			[]discoveryscan.ProbeFactory{
				configuredObservationFactory(current, nil),
			},
			readModel,
			typedNilDiscovery,
			statusCommitter,
		)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			observer.RunOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != current.Digest() ||
			plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("prepared observer returned wrong status facts")
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
		if statusCommitter.calls != 1 {
			t.Fatalf("status calls = %d, want one", statusCommitter.calls)
		}
	})
}

func TestPreparedProjectedRuntimeObserverErrorsAndExplicitRetry(t *testing.T) {
	readModel, _, _ := projectedObservationReadModel(
		t, nil, "prepared-errors",
	)
	t.Run("configured_discovery_failure", func(t *testing.T) {
		configuredError := errors.New("configured discovery sentinel")
		configuredFactory := &appDiscoveryFactory{
			name: "configured-error",
			build: func(
				context.Context,
			) (loomruntime.RuntimeProbe, bool, error) {
				return nil, false, configuredError
			},
		}
		configuredDiscoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
		configuredStatusCommitter := &recordingRuntimeStatusCommitter{}
		observer, err := NewPreparedProjectedRuntimeObserver(
			[]discoveryscan.ProbeFactory{configuredFactory},
			readModel,
			configuredDiscoveryCommitter,
			configuredStatusCommitter,
		)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			observer.RunOnce(context.Background())
		if !errors.Is(err, configuredError) {
			t.Fatalf("error = %v, want configured sentinel", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if configuredFactory.calls != 1 {
			t.Fatalf("factory calls = %d, want one", configuredFactory.calls)
		}
		if configuredDiscoveryCommitter.calls != 0 {
			t.Fatalf(
				"discovery calls = %d, want zero",
				configuredDiscoveryCommitter.calls,
			)
		}
		if configuredStatusCommitter.calls != 0 {
			t.Fatalf(
				"status calls = %d, want zero",
				configuredStatusCommitter.calls,
			)
		}
	})

	current := appDiscoverySnapshot(t, "prepared-retry", 1)
	expected := mintAppDiscoveryCandidate(
		t, context.Background(), current, "prepared-retry",
	)
	committer := &recordingRuntimeDiscoveryCommitter{candidate: expected}
	observer, err := NewPreparedProjectedRuntimeObserver(
		[]discoveryscan.ProbeFactory{
			configuredObservationFactory(current, nil),
		},
		readModel,
		committer,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	var firstPlan RuntimeObservationWritePlanCandidate
	var firstCommit state.RuntimeDiscoveryCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			observer.RunOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if attempt == 0 {
			firstPlan, firstCommit = plan, discovery
			events := discovery.Events()
			events[0].PayloadJSON[0] ^= 0xff
		} else if plan != firstPlan ||
			discovery.CommitDigest() != firstCommit.CommitDigest() ||
			!reflect.DeepEqual(discovery.Events(), firstCommit.Events()) {
			t.Fatal("explicit retry retained or changed exact results")
		}
	}
	if committer.calls != 2 {
		t.Fatalf("calls = %d, want one per explicit invocation", committer.calls)
	}
}

func TestPreparedProjectedRuntimeObserverDirectErrorMatrix(t *testing.T) {
	mustObserver := func(
		factories []discoveryscan.ProbeFactory,
		readModel *projection.Projection,
		discoveryCommitter RuntimeDiscoveryCommitter,
		statusCommitter RuntimeStatusCommitter,
	) *PreparedProjectedRuntimeObserver {
		t.Helper()
		observer, err := NewPreparedProjectedRuntimeObserver(
			factories,
			readModel,
			discoveryCommitter,
			statusCommitter,
		)
		if err != nil {
			t.Fatal(err)
		}
		return observer
	}

	emptyReadModel, _, _ := projectedObservationReadModel(
		t, nil, "prepared-error-matrix-empty",
	)
	discoverySnapshot := appDiscoverySnapshot(
		t, "prepared-error-matrix-discovery", 1,
	)
	discoveryFactories := []discoveryscan.ProbeFactory{
		configuredObservationFactory(discoverySnapshot, nil),
	}
	validDiscovery := mintAppDiscoveryCandidate(
		t,
		context.Background(),
		discoverySnapshot,
		"prepared-error-matrix-discovery",
	)

	_, statusSnapshot, statusPrevious := appStatusSources(
		t,
		"prepared-error-matrix-status",
		1,
		loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
	)
	statusReadModel, _, _ := projectedObservationReadModel(
		t, &statusPrevious, "prepared-error-matrix-status",
	)
	statusFactories := []discoveryscan.ProbeFactory{
		configuredObservationFactory(statusSnapshot, nil),
	}

	contextDiscovery := &recordingRuntimeDiscoveryCommitter{}
	contextStatus := &recordingRuntimeStatusCommitter{}
	contextObserver := mustObserver(
		nil, emptyReadModel, contextDiscovery, contextStatus,
	)

	missingDiscoveryOpposite := &recordingRuntimeStatusCommitter{}
	missingDiscoveryObserver := mustObserver(
		discoveryFactories,
		emptyReadModel,
		nil,
		missingDiscoveryOpposite,
	)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	typedNilDiscoveryOpposite := &recordingRuntimeStatusCommitter{}
	typedNilDiscoveryObserver := mustObserver(
		discoveryFactories,
		emptyReadModel,
		typedNilDiscovery,
		typedNilDiscoveryOpposite,
	)
	errDiscoveryCommit := errors.New("prepared discovery committer error")
	discoveryErrorCommitter := &recordingRuntimeDiscoveryCommitter{
		err: errDiscoveryCommit,
	}
	discoveryErrorOpposite := &recordingRuntimeStatusCommitter{}
	discoveryErrorObserver := mustObserver(
		discoveryFactories,
		emptyReadModel,
		discoveryErrorCommitter,
		discoveryErrorOpposite,
	)
	discoveryMismatchCommitter := &recordingRuntimeDiscoveryCommitter{}
	discoveryMismatchOpposite := &recordingRuntimeStatusCommitter{}
	discoveryMismatchObserver := mustObserver(
		discoveryFactories,
		emptyReadModel,
		discoveryMismatchCommitter,
		discoveryMismatchOpposite,
	)

	missingStatusOpposite := &recordingRuntimeDiscoveryCommitter{}
	missingStatusObserver := mustObserver(
		statusFactories,
		statusReadModel,
		missingStatusOpposite,
		nil,
	)
	var typedNilStatus *recordingRuntimeStatusCommitter
	typedNilStatusOpposite := &recordingRuntimeDiscoveryCommitter{}
	typedNilStatusObserver := mustObserver(
		statusFactories,
		statusReadModel,
		typedNilStatusOpposite,
		typedNilStatus,
	)
	errStatusCommit := errors.New("prepared status committer error")
	statusErrorCommitter := &recordingRuntimeStatusCommitter{
		err: errStatusCommit,
	}
	statusErrorOpposite := &recordingRuntimeDiscoveryCommitter{}
	statusErrorObserver := mustObserver(
		statusFactories,
		statusReadModel,
		statusErrorOpposite,
		statusErrorCommitter,
	)
	statusMismatchCommitter := &recordingRuntimeStatusCommitter{}
	statusMismatchOpposite := &recordingRuntimeDiscoveryCommitter{}
	statusMismatchObserver := mustObserver(
		statusFactories,
		statusReadModel,
		statusMismatchOpposite,
		statusMismatchCommitter,
	)

	_, _, identityPrevious := appStatusSources(
		t,
		"prepared-error-matrix-identity",
		1,
		loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
	)
	identityReadModel, _, _ := projectedObservationReadModel(
		t, &identityPrevious, "prepared-error-matrix-identity",
	)
	identityDrift := appStatusSnapshot(
		t,
		"prepared-error-matrix-identity",
		1,
		loomruntime.RuntimeOffline,
		"device.other",
	)
	identityDiscovery := &recordingRuntimeDiscoveryCommitter{}
	identityStatus := &recordingRuntimeStatusCommitter{}
	identityObserver := mustObserver(
		[]discoveryscan.ProbeFactory{
			configuredObservationFactory(identityDrift, nil),
		},
		identityReadModel,
		identityDiscovery,
		identityStatus,
	)

	cancelAfterWriteContext, cancelAfterWrite := context.WithCancel(
		context.Background(),
	)
	cancelAfterWriteCommitter := &recordingRuntimeDiscoveryCommitter{
		commit: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitCandidate, error) {
			cancelAfterWrite()
			return validDiscovery, nil
		},
	}
	cancelAfterWriteOpposite := &recordingRuntimeStatusCommitter{}
	cancelAfterWriteObserver := mustObserver(
		discoveryFactories,
		emptyReadModel,
		cancelAfterWriteCommitter,
		cancelAfterWriteOpposite,
	)

	tests := []struct {
		name               string
		observer           *PreparedProjectedRuntimeObserver
		ctx                context.Context
		want               error
		discoveryCommitter *recordingRuntimeDiscoveryCommitter
		wantDiscoveryCalls int
		statusCommitter    *recordingRuntimeStatusCommitter
		wantStatusCalls    int
	}{
		{
			name:               "canceled_context",
			observer:           contextObserver,
			ctx:                canceledAppDiscoveryContext(),
			want:               context.Canceled,
			discoveryCommitter: contextDiscovery,
			statusCommitter:    contextStatus,
		},
		{
			name:               "expired_deadline",
			observer:           contextObserver,
			ctx:                expiredAppDiscoveryContext(),
			want:               context.DeadlineExceeded,
			discoveryCommitter: contextDiscovery,
			statusCommitter:    contextStatus,
		},
		{
			name:               "invalid_identity_drift",
			observer:           identityObserver,
			ctx:                context.Background(),
			want:               loomruntime.ErrRuntimeStatusIdentityDrift,
			discoveryCommitter: identityDiscovery,
			statusCommitter:    identityStatus,
		},
		{
			name:            "missing_discovery_committer",
			observer:        missingDiscoveryObserver,
			ctx:             context.Background(),
			want:            ErrInvalidRuntimeObservationWriteRun,
			statusCommitter: missingDiscoveryOpposite,
		},
		{
			name:            "typed_nil_discovery_committer",
			observer:        typedNilDiscoveryObserver,
			ctx:             context.Background(),
			want:            ErrInvalidRuntimeObservationWriteRun,
			statusCommitter: typedNilDiscoveryOpposite,
		},
		{
			name:               "discovery_committer_error",
			observer:           discoveryErrorObserver,
			ctx:                context.Background(),
			want:               errDiscoveryCommit,
			discoveryCommitter: discoveryErrorCommitter,
			wantDiscoveryCalls: 1,
			statusCommitter:    discoveryErrorOpposite,
		},
		{
			name:               "discovery_result_mismatch",
			observer:           discoveryMismatchObserver,
			ctx:                context.Background(),
			want:               ErrRuntimeDiscoveryCommitResultMismatch,
			discoveryCommitter: discoveryMismatchCommitter,
			wantDiscoveryCalls: 1,
			statusCommitter:    discoveryMismatchOpposite,
		},
		{
			name:               "missing_status_committer",
			observer:           missingStatusObserver,
			ctx:                context.Background(),
			want:               ErrInvalidRuntimeObservationWriteRun,
			discoveryCommitter: missingStatusOpposite,
		},
		{
			name:               "typed_nil_status_committer",
			observer:           typedNilStatusObserver,
			ctx:                context.Background(),
			want:               ErrInvalidRuntimeObservationWriteRun,
			discoveryCommitter: typedNilStatusOpposite,
		},
		{
			name:               "status_committer_error",
			observer:           statusErrorObserver,
			ctx:                context.Background(),
			want:               errStatusCommit,
			discoveryCommitter: statusErrorOpposite,
			statusCommitter:    statusErrorCommitter,
			wantStatusCalls:    1,
		},
		{
			name:               "status_result_mismatch",
			observer:           statusMismatchObserver,
			ctx:                context.Background(),
			want:               ErrRuntimeStatusCommitResultMismatch,
			discoveryCommitter: statusMismatchOpposite,
			statusCommitter:    statusMismatchCommitter,
			wantStatusCalls:    1,
		},
		{
			name:               "canceled_after_discovery_write",
			observer:           cancelAfterWriteObserver,
			ctx:                cancelAfterWriteContext,
			want:               context.Canceled,
			discoveryCommitter: cancelAfterWriteCommitter,
			wantDiscoveryCalls: 1,
			statusCommitter:    cancelAfterWriteOpposite,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot, plan, discovery, reconciliation, status, err :=
				test.observer.RunOnce(test.ctx)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroConfiguredRuntimeObservationRun(
				t, snapshot, plan, discovery, reconciliation, status,
			)
			if test.discoveryCommitter != nil &&
				test.discoveryCommitter.calls != test.wantDiscoveryCalls {
				t.Fatalf(
					"discovery calls = %d, want %d",
					test.discoveryCommitter.calls,
					test.wantDiscoveryCalls,
				)
			}
			if test.statusCommitter != nil &&
				test.statusCommitter.calls != test.wantStatusCalls {
				t.Fatalf(
					"status calls = %d, want %d",
					test.statusCommitter.calls,
					test.wantStatusCalls,
				)
			}
		})
	}
}

func TestPreparedProjectedRuntimeObserverRealSQLiteChain(t *testing.T) {
	ctx := context.Background()
	previous := appDiscoverySnapshot(t, "prepared-sqlite", 1)
	readModel, store, db := projectedObservationReadModel(
		t, &previous, "prepared-sqlite-prior",
	)
	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "prepared sqlite changed"
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	discoveryProvider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			input := appDiscoveryCommitInput(mixed, "prepared-sqlite-mixed")
			input.Events[0].Seq = 2
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
	discoveryObserver, err := NewPreparedProjectedRuntimeObserver(
		[]discoveryscan.ProbeFactory{
			configuredObservationFactory(mixed, nil),
		},
		readModel,
		discoveryAdapter,
		statusPoison,
	)
	if err != nil {
		t.Fatal(err)
	}
	var firstDiscovery state.RuntimeDiscoveryCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			discoveryObserver.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 {
			t.Fatal("prepared mixed cycle lost discovery priority")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if attempt == 0 {
			firstDiscovery = discovery
		} else if discovery.CommitDigest() != firstDiscovery.CommitDigest() ||
			!reflect.DeepEqual(discovery.Events(), firstDiscovery.Events()) {
			t.Fatal("prepared discovery retry changed exact commit")
		}
	}
	if appDiscoveryEventCount(t, db) != 2 || statusPoison.calls != 0 {
		t.Fatal("prepared discovery retry wrote duplicate or opposite Event")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}

	statusOnly := runtimeWritePlanSnapshotFrom(
		t, mixed, mixed.Observations()[0].SourceProbeID,
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
			return appStatusCommitInput(candidate, "prepared-sqlite-status"), nil
		},
	}
	statusAdapter, err := NewPreparedRuntimeStatusCommitter(store, statusProvider)
	if err != nil {
		t.Fatal(err)
	}
	discoveryPoison := &recordingRuntimeDiscoveryCommitter{
		err: errors.New("status path must not write discovery"),
	}
	statusObserver, err := NewPreparedProjectedRuntimeObserver(
		[]discoveryscan.ProbeFactory{
			configuredObservationFactory(statusOnly, nil),
		},
		readModel,
		discoveryPoison,
		statusAdapter,
	)
	if err != nil {
		t.Fatal(err)
	}
	var firstReconciliation loomruntime.RuntimeStatusReconciliationCandidate
	var firstStatus state.RuntimeStatusCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			statusObserver.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("prepared status cycle returned wrong facts")
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
		if attempt == 0 {
			firstReconciliation, firstStatus = reconciliation, status
		} else {
			assertSameAppStatusReconciliation(
				t, reconciliation, firstReconciliation,
			)
			if status.CommitDigest() != firstStatus.CommitDigest() ||
				!reflect.DeepEqual(status.Events(), firstStatus.Events()) {
				t.Fatal("prepared status retry changed exact commit")
			}
		}
	}
	if appDiscoveryEventCount(t, db) != 3 || discoveryPoison.calls != 0 {
		t.Fatal("prepared status retry wrote duplicate or opposite Event")
	}

	rows, err := db.Query(
		`SELECT event_type, seq FROM events ORDER BY stream_id, seq`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var eventTypes []string
	var sequences []int64
	for rows.Next() {
		var eventType string
		var sequence int64
		if err := rows.Scan(&eventType, &sequence); err != nil {
			t.Fatal(err)
		}
		eventTypes = append(eventTypes, eventType)
		sequences = append(sequences, sequence)
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
	if want := []int64{1, 2, 3}; !reflect.DeepEqual(sequences, want) {
		t.Fatalf("event sequences = %#v, want %#v", sequences, want)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	finalRecord := readModel.Snapshot().
		RuntimeInstances["runtime.prepared-sqlite.a"]
	if finalRecord.DisplayName != "prepared sqlite changed" ||
		finalRecord.Status != string(loomruntime.RuntimeOnline) ||
		finalRecord.DiscoverySequence != 2 ||
		finalRecord.StatusSequence != 3 {
		t.Fatalf("final prepared Runtime = %#v", finalRecord)
	}
}

func TestPreparedProjectedRuntimeObserverStaticBoundary(t *testing.T) {
	const filename = "runtime_observer.go"
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
		"loom-pi-rebuild/internal/runtime/discoveryscan": true,
		"loom-pi-rebuild/internal/state":                 true,
	}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !allowed[path] {
			t.Fatalf("forbidden product import %q", path)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt, *ast.ForStmt, *ast.RangeStmt:
			t.Errorf("forbidden concurrency/retry node %T", typed)
		case *ast.SelectorExpr:
			switch typed.Sel.Name {
			case "Rebuild", "Append", "AppendBatch",
				"CommitRuntimeDiscoverySnapshot",
				"CommitRuntimeStatusTransitions",
				"NewTicker", "Sleep":
				t.Errorf("forbidden selector %q", typed.Sel.Name)
			}
		}
		return true
	})
	product := string(source)
	if strings.Count(
		product, "RunProjectedConfiguredRuntimeObservationOnce(",
	) != 1 {
		t.Fatal("product must call S2-W35 exactly once")
	}
	for _, marker := range []string{
		"RunConfiguredRuntimeObservationOnce(",
		"RunRuntimeObservationWriteOnce(",
		"DiscoverConfiguredRuntimes(",
		"internal/journal", "internal/runtime/piadapter",
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

func TestPreparedProjectedRuntimeObserverRepair1ErrorMatrixCoverage(
	t *testing.T,
) {
	source, err := os.ReadFile("runtime_observer_test.go")
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{
		"canceled_" + "context",
		"expired_" + "deadline",
		"invalid_" + "identity_drift",
		"missing_" + "discovery_committer",
		"typed_nil_" + "discovery_committer",
		"discovery_" + "committer_error",
		"discovery_" + "result_mismatch",
		"missing_" + "status_committer",
		"typed_nil_" + "status_committer",
		"status_" + "committer_error",
		"status_" + "result_mismatch",
		"canceled_after_" + "discovery_write",
	}
	for _, marker := range markers {
		if !strings.Contains(string(source), `"`+marker+`"`) {
			t.Errorf("missing direct prepared-observer case %q", marker)
		}
	}
}

func TestPreparedProjectedRuntimeObserverRepair2ConfiguredDiscoveryCoverage(
	t *testing.T,
) {
	source, err := os.ReadFile("runtime_observer_test.go")
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{
		"configured_" + "discovery_failure",
		"configuredFactory.calls != " + "1",
		"configuredDiscoveryCommitter.calls != " + "0",
		"configuredStatusCommitter.calls != " + "0",
	}
	for _, marker := range markers {
		if !strings.Contains(string(source), marker) {
			t.Errorf("missing case-local configured-discovery proof %q", marker)
		}
	}
}
