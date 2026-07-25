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

func TestRunTriggeredPreparedRuntimeObservationOncePrevalidatesInputs(
	t *testing.T,
) {
	readModel, _, _ := projectedObservationReadModel(
		t, nil, "triggered-prevalidate",
	)
	factory := &appDiscoveryFactory{name: "unused"}
	observer := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{factory},
		readModel,
		&recordingRuntimeDiscoveryCommitter{},
		&recordingRuntimeStatusCommitter{},
	)
	var typedNilTrigger *recordingRuntimeObservationTrigger
	tests := []struct {
		name         string
		ctx          context.Context
		trigger      RuntimeObservationTrigger
		triggerCount *recordingRuntimeObservationTrigger
		observer     *PreparedProjectedRuntimeObserver
		want         error
	}{
		{
			name:         "nil context",
			triggerCount: &recordingRuntimeObservationTrigger{},
			observer:     observer,
			want:         ErrInvalidTriggeredPreparedRuntimeObservationRun,
		},
		{
			name:     "nil trigger",
			ctx:      context.Background(),
			observer: observer,
			want:     ErrInvalidTriggeredPreparedRuntimeObservationRun,
		},
		{
			name:     "typed nil trigger",
			ctx:      context.Background(),
			trigger:  typedNilTrigger,
			observer: observer,
			want:     ErrInvalidTriggeredPreparedRuntimeObservationRun,
		},
		{
			name:         "nil observer",
			ctx:          context.Background(),
			triggerCount: &recordingRuntimeObservationTrigger{},
			want:         ErrInvalidTriggeredPreparedRuntimeObservationRun,
		},
		{
			name:         "canceled",
			ctx:          canceledAppDiscoveryContext(),
			triggerCount: &recordingRuntimeObservationTrigger{},
			observer:     observer,
			want:         context.Canceled,
		},
		{
			name:         "deadline",
			ctx:          expiredAppDiscoveryContext(),
			triggerCount: &recordingRuntimeObservationTrigger{},
			observer:     observer,
			want:         context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trigger := test.trigger
			if test.triggerCount != nil {
				trigger = test.triggerCount
			}
			snapshot, plan, discovery, reconciliation, status, err :=
				RunTriggeredPreparedRuntimeObservationOnce(
					test.ctx,
					trigger,
					test.observer,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroConfiguredRuntimeObservationRun(
				t, snapshot, plan, discovery, reconciliation, status,
			)
			if test.triggerCount != nil &&
				test.triggerCount.calls != 0 {
				t.Fatalf(
					"trigger calls = %d, want zero",
					test.triggerCount.calls,
				)
			}
		})
	}
	if factory.calls != 0 {
		t.Fatalf("factory calls = %d, want zero", factory.calls)
	}
}

func TestRunTriggeredPreparedRuntimeObservationOnceTriggerFailures(
	t *testing.T,
) {
	readModel, _, _ := projectedObservationReadModel(
		t, nil, "triggered-trigger-errors",
	)
	factory := &appDiscoveryFactory{name: "unused"}
	discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
	statusCommitter := &recordingRuntimeStatusCommitter{}
	observer := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{factory},
		readModel,
		discoveryCommitter,
		statusCommitter,
	)

	t.Run("trigger error", func(t *testing.T) {
		sentinel := errors.New("trigger sentinel")
		trigger := &recordingRuntimeObservationTrigger{err: sentinel}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(
				context.Background(),
				trigger,
				observer,
			)
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want trigger sentinel", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if trigger.calls != 1 {
			t.Fatalf("trigger calls = %d, want one", trigger.calls)
		}
	})

	t.Run("trigger induced cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		trigger := &recordingRuntimeObservationTrigger{
			await: func(context.Context) error {
				cancel()
				return nil
			},
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(ctx, trigger, observer)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact canceled", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if trigger.calls != 1 {
			t.Fatalf("trigger calls = %d, want one", trigger.calls)
		}
	})

	if factory.calls != 0 ||
		discoveryCommitter.calls != 0 ||
		statusCommitter.calls != 0 {
		t.Fatalf(
			"observer work = factory:%d discovery:%d status:%d, want zero",
			factory.calls,
			discoveryCommitter.calls,
			statusCommitter.calls,
		)
	}
}

func TestRunTriggeredPreparedRuntimeObservationOnceSuccessPaths(
	t *testing.T,
) {
	t.Run("none", func(t *testing.T) {
		readModel, _, _ := projectedObservationReadModel(
			t, nil, "triggered-none",
		)
		var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
		var typedNilStatus *recordingRuntimeStatusCommitter
		observer := mustTriggeredPreparedObserver(
			t,
			nil,
			readModel,
			typedNilDiscovery,
			typedNilStatus,
		)
		trigger := &recordingRuntimeObservationTrigger{}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(
				context.Background(),
				trigger,
				observer,
			)
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
		if trigger.calls != 1 {
			t.Fatalf("trigger calls = %d, want one", trigger.calls)
		}
	})

	t.Run("discovery priority and order", func(t *testing.T) {
		readModel, _, _ := projectedObservationReadModel(
			t, nil, "triggered-discovery",
		)
		current := appDiscoverySnapshot(t, "triggered-discovery", 1)
		expected := mintAppDiscoveryCandidate(
			t, context.Background(), current, "triggered-discovery",
		)
		var trace []string
		trigger := &recordingRuntimeObservationTrigger{trace: &trace}
		factory := triggeredObservationFactory(current, &trace)
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			trace:     &trace,
			candidate: expected,
		}
		statusCommitter := &recordingRuntimeStatusCommitter{
			err: errors.New("discovery path must not write status"),
		}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			discoveryCommitter,
			statusCommitter,
		)
		snapshot, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(
				context.Background(),
				trigger,
				observer,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != current.Digest() ||
			plan.Kind() != RuntimeObservationWriteDiscovery ||
			discovery.CommitDigest() != expected.CommitDigest() {
			t.Fatal("triggered observation lost discovery result")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if trigger.calls != 1 ||
			factory.calls != 1 ||
			discoveryCommitter.calls != 1 ||
			statusCommitter.calls != 0 {
			t.Fatal("triggered discovery call counts are not exact")
		}
		if want := []string{
			"trigger",
			"factory:triggered-observation",
			"probe:probe.triggered-discovery",
			"commit",
		}; !reflect.DeepEqual(trace, want) {
			t.Fatalf("trace = %#v, want %#v", trace, want)
		}
		observations := snapshot.Observations()
		observations[0].ModelIDs[0] = "mutated"
		if reflect.DeepEqual(observations, snapshot.Observations()) {
			t.Fatal("snapshot accessor leaked mutation")
		}
		events := discovery.Events()
		events[0].PayloadJSON[0] ^= 0xff
		if reflect.DeepEqual(events, discovery.Events()) {
			t.Fatal("discovery Event accessor leaked mutation")
		}
	})

	t.Run("status", func(t *testing.T) {
		_, current, previous := appStatusSources(
			t,
			"triggered-status",
			1,
			loomruntime.RuntimeOnline,
			loomruntime.RuntimeOffline,
		)
		readModel, _, _ := projectedObservationReadModel(
			t, &previous, "triggered-status",
		)
		factory := triggeredObservationFactory(current, nil)
		var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
		statusCommitter := &recordingRuntimeStatusCommitter{
			commit: func(
				ctx context.Context,
				candidate loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitCandidate, error) {
				return mintAppStatusCommit(
					t, ctx, candidate, "triggered-status",
				), nil
			},
		}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			typedNilDiscovery,
			statusCommitter,
		)
		trigger := &recordingRuntimeObservationTrigger{}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(
				context.Background(),
				trigger,
				observer,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != current.Digest() ||
			plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("triggered status returned wrong facts")
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
		if trigger.calls != 1 ||
			factory.calls != 1 ||
			statusCommitter.calls != 1 {
			t.Fatal("triggered status call counts are not exact")
		}
	})
}

func TestRunTriggeredPreparedRuntimeObservationOnceDirectErrorMatrix(
	t *testing.T,
) {
	emptyReadModel, _, _ := projectedObservationReadModel(
		t, nil, "triggered-errors-empty",
	)
	discoverySnapshot := appDiscoverySnapshot(
		t, "triggered-errors-discovery", 1,
	)
	validDiscovery := mintAppDiscoveryCandidate(
		t,
		context.Background(),
		discoverySnapshot,
		"triggered-errors-discovery",
	)
	_, statusSnapshot, statusPrevious := appStatusSources(
		t,
		"triggered-errors-status",
		1,
		loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
	)
	statusReadModel, _, _ := projectedObservationReadModel(
		t, &statusPrevious, "triggered-errors-status",
	)

	configuredError := errors.New("triggered configured discovery error")
	configuredFactory := &appDiscoveryFactory{
		name: "triggered-configured-error",
		build: func(
			context.Context,
		) (loomruntime.RuntimeProbe, bool, error) {
			return nil, false, configuredError
		},
	}
	configuredDiscovery := &recordingRuntimeDiscoveryCommitter{}
	configuredStatus := &recordingRuntimeStatusCommitter{}
	configuredObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{configuredFactory},
		emptyReadModel,
		configuredDiscovery,
		configuredStatus,
	)

	identityFactory := triggeredObservationFactory(
		appStatusSnapshot(
			t,
			"triggered-errors-identity",
			1,
			loomruntime.RuntimeOffline,
			"device.other",
		),
		nil,
	)
	_, _, identityPrevious := appStatusSources(
		t,
		"triggered-errors-identity",
		1,
		loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
	)
	identityReadModel, _, _ := projectedObservationReadModel(
		t, &identityPrevious, "triggered-errors-identity",
	)
	identityDiscovery := &recordingRuntimeDiscoveryCommitter{}
	identityStatus := &recordingRuntimeStatusCommitter{}
	identityObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{identityFactory},
		identityReadModel,
		identityDiscovery,
		identityStatus,
	)

	missingDiscoveryFactory := triggeredObservationFactory(
		discoverySnapshot, nil,
	)
	missingDiscoveryOpposite := &recordingRuntimeStatusCommitter{}
	missingDiscoveryObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{missingDiscoveryFactory},
		emptyReadModel,
		nil,
		missingDiscoveryOpposite,
	)
	typedNilDiscoveryFactory := triggeredObservationFactory(
		discoverySnapshot, nil,
	)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	typedNilDiscoveryOpposite := &recordingRuntimeStatusCommitter{}
	typedNilDiscoveryObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{typedNilDiscoveryFactory},
		emptyReadModel,
		typedNilDiscovery,
		typedNilDiscoveryOpposite,
	)
	discoveryCommitError := errors.New("triggered discovery commit error")
	discoveryErrorFactory := triggeredObservationFactory(discoverySnapshot, nil)
	discoveryErrorCommitter := &recordingRuntimeDiscoveryCommitter{
		err: discoveryCommitError,
	}
	discoveryErrorOpposite := &recordingRuntimeStatusCommitter{}
	discoveryErrorObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{discoveryErrorFactory},
		emptyReadModel,
		discoveryErrorCommitter,
		discoveryErrorOpposite,
	)
	discoveryMismatchFactory := triggeredObservationFactory(
		discoverySnapshot, nil,
	)
	discoveryMismatchCommitter := &recordingRuntimeDiscoveryCommitter{}
	discoveryMismatchOpposite := &recordingRuntimeStatusCommitter{}
	discoveryMismatchObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{discoveryMismatchFactory},
		emptyReadModel,
		discoveryMismatchCommitter,
		discoveryMismatchOpposite,
	)

	missingStatusFactory := triggeredObservationFactory(statusSnapshot, nil)
	missingStatusOpposite := &recordingRuntimeDiscoveryCommitter{}
	missingStatusObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{missingStatusFactory},
		statusReadModel,
		missingStatusOpposite,
		nil,
	)
	typedNilStatusFactory := triggeredObservationFactory(statusSnapshot, nil)
	var typedNilStatus *recordingRuntimeStatusCommitter
	typedNilStatusOpposite := &recordingRuntimeDiscoveryCommitter{}
	typedNilStatusObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{typedNilStatusFactory},
		statusReadModel,
		typedNilStatusOpposite,
		typedNilStatus,
	)
	statusCommitError := errors.New("triggered status commit error")
	statusErrorFactory := triggeredObservationFactory(statusSnapshot, nil)
	statusErrorCommitter := &recordingRuntimeStatusCommitter{
		err: statusCommitError,
	}
	statusErrorOpposite := &recordingRuntimeDiscoveryCommitter{}
	statusErrorObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{statusErrorFactory},
		statusReadModel,
		statusErrorOpposite,
		statusErrorCommitter,
	)
	statusMismatchFactory := triggeredObservationFactory(statusSnapshot, nil)
	statusMismatchCommitter := &recordingRuntimeStatusCommitter{}
	statusMismatchOpposite := &recordingRuntimeDiscoveryCommitter{}
	statusMismatchObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{statusMismatchFactory},
		statusReadModel,
		statusMismatchOpposite,
		statusMismatchCommitter,
	)

	cancelContext, cancelAfterWrite := context.WithCancel(context.Background())
	cancelFactory := triggeredObservationFactory(discoverySnapshot, nil)
	cancelCommitter := &recordingRuntimeDiscoveryCommitter{
		commit: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitCandidate, error) {
			cancelAfterWrite()
			return validDiscovery, nil
		},
	}
	cancelOpposite := &recordingRuntimeStatusCommitter{}
	cancelObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{cancelFactory},
		emptyReadModel,
		cancelCommitter,
		cancelOpposite,
	)

	tests := []struct {
		name               string
		ctx                context.Context
		observer           *PreparedProjectedRuntimeObserver
		factory            *appDiscoveryFactory
		want               error
		discoveryCommitter *recordingRuntimeDiscoveryCommitter
		wantDiscoveryCalls int
		statusCommitter    *recordingRuntimeStatusCommitter
		wantStatusCalls    int
	}{
		{
			name:               "configured discovery failure",
			ctx:                context.Background(),
			observer:           configuredObserver,
			factory:            configuredFactory,
			want:               configuredError,
			discoveryCommitter: configuredDiscovery,
			statusCommitter:    configuredStatus,
		},
		{
			name:               "invalid identity drift",
			ctx:                context.Background(),
			observer:           identityObserver,
			factory:            identityFactory,
			want:               loomruntime.ErrRuntimeStatusIdentityDrift,
			discoveryCommitter: identityDiscovery,
			statusCommitter:    identityStatus,
		},
		{
			name:            "missing discovery committer",
			ctx:             context.Background(),
			observer:        missingDiscoveryObserver,
			factory:         missingDiscoveryFactory,
			want:            ErrInvalidRuntimeObservationWriteRun,
			statusCommitter: missingDiscoveryOpposite,
		},
		{
			name:            "typed nil discovery committer",
			ctx:             context.Background(),
			observer:        typedNilDiscoveryObserver,
			factory:         typedNilDiscoveryFactory,
			want:            ErrInvalidRuntimeObservationWriteRun,
			statusCommitter: typedNilDiscoveryOpposite,
		},
		{
			name:               "discovery committer error",
			ctx:                context.Background(),
			observer:           discoveryErrorObserver,
			factory:            discoveryErrorFactory,
			want:               discoveryCommitError,
			discoveryCommitter: discoveryErrorCommitter,
			wantDiscoveryCalls: 1,
			statusCommitter:    discoveryErrorOpposite,
		},
		{
			name:               "discovery result mismatch",
			ctx:                context.Background(),
			observer:           discoveryMismatchObserver,
			factory:            discoveryMismatchFactory,
			want:               ErrRuntimeDiscoveryCommitResultMismatch,
			discoveryCommitter: discoveryMismatchCommitter,
			wantDiscoveryCalls: 1,
			statusCommitter:    discoveryMismatchOpposite,
		},
		{
			name:               "missing status committer",
			ctx:                context.Background(),
			observer:           missingStatusObserver,
			factory:            missingStatusFactory,
			want:               ErrInvalidRuntimeObservationWriteRun,
			discoveryCommitter: missingStatusOpposite,
		},
		{
			name:               "typed nil status committer",
			ctx:                context.Background(),
			observer:           typedNilStatusObserver,
			factory:            typedNilStatusFactory,
			want:               ErrInvalidRuntimeObservationWriteRun,
			discoveryCommitter: typedNilStatusOpposite,
		},
		{
			name:               "status committer error",
			ctx:                context.Background(),
			observer:           statusErrorObserver,
			factory:            statusErrorFactory,
			want:               statusCommitError,
			discoveryCommitter: statusErrorOpposite,
			statusCommitter:    statusErrorCommitter,
			wantStatusCalls:    1,
		},
		{
			name:               "status result mismatch",
			ctx:                context.Background(),
			observer:           statusMismatchObserver,
			factory:            statusMismatchFactory,
			want:               ErrRuntimeStatusCommitResultMismatch,
			discoveryCommitter: statusMismatchOpposite,
			statusCommitter:    statusMismatchCommitter,
			wantStatusCalls:    1,
		},
		{
			name:               "canceled after discovery write",
			ctx:                cancelContext,
			observer:           cancelObserver,
			factory:            cancelFactory,
			want:               context.Canceled,
			discoveryCommitter: cancelCommitter,
			wantDiscoveryCalls: 1,
			statusCommitter:    cancelOpposite,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trigger := &recordingRuntimeObservationTrigger{}
			snapshot, plan, discovery, reconciliation, status, err :=
				RunTriggeredPreparedRuntimeObservationOnce(
					test.ctx,
					trigger,
					test.observer,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroConfiguredRuntimeObservationRun(
				t, snapshot, plan, discovery, reconciliation, status,
			)
			if trigger.calls != 1 {
				t.Fatalf("trigger calls = %d, want one", trigger.calls)
			}
			if test.factory.calls != 1 {
				t.Fatalf("factory calls = %d, want one", test.factory.calls)
			}
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

func TestRunTriggeredPreparedRuntimeObservationOnceRealSQLiteChain(
	t *testing.T,
) {
	ctx := context.Background()
	previous := appDiscoverySnapshot(t, "triggered-sqlite", 1)
	readModel, store, db := projectedObservationReadModel(
		t, &previous, "triggered-sqlite-prior",
	)
	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "triggered sqlite changed"
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	discoveryProvider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			input := appDiscoveryCommitInput(mixed, "triggered-sqlite-mixed")
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
	discoveryObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{
			configuredObservationFactory(mixed, nil),
		},
		readModel,
		discoveryAdapter,
		statusPoison,
	)
	discoveryTrigger := &recordingRuntimeObservationTrigger{}
	var firstDiscovery state.RuntimeDiscoveryCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(
				ctx,
				discoveryTrigger,
				discoveryObserver,
			)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 {
			t.Fatal("triggered mixed cycle lost discovery priority")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if attempt == 0 {
			firstDiscovery = discovery
		} else if discovery.CommitDigest() != firstDiscovery.CommitDigest() ||
			!reflect.DeepEqual(discovery.Events(), firstDiscovery.Events()) {
			t.Fatal("triggered discovery retry changed exact commit")
		}
	}
	if discoveryTrigger.calls != 2 ||
		appDiscoveryEventCount(t, db) != 2 ||
		statusPoison.calls != 0 {
		t.Fatal("triggered discovery retry counts are not exact")
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
			return appStatusCommitInput(candidate, "triggered-sqlite-status"), nil
		},
	}
	statusAdapter, err := NewPreparedRuntimeStatusCommitter(store, statusProvider)
	if err != nil {
		t.Fatal(err)
	}
	discoveryPoison := &recordingRuntimeDiscoveryCommitter{
		err: errors.New("status path must not write discovery"),
	}
	statusObserver := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{
			configuredObservationFactory(statusOnly, nil),
		},
		readModel,
		discoveryPoison,
		statusAdapter,
	)
	statusTrigger := &recordingRuntimeObservationTrigger{}
	var firstReconciliation loomruntime.RuntimeStatusReconciliationCandidate
	var firstStatus state.RuntimeStatusCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			RunTriggeredPreparedRuntimeObservationOnce(
				ctx,
				statusTrigger,
				statusObserver,
			)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("triggered status cycle returned wrong facts")
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
				t.Fatal("triggered status retry changed exact commit")
			}
		}
	}
	if statusTrigger.calls != 2 ||
		appDiscoveryEventCount(t, db) != 3 ||
		discoveryPoison.calls != 0 {
		t.Fatal("triggered status retry counts are not exact")
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
		RuntimeInstances["runtime.triggered-sqlite.a"]
	if finalRecord.DisplayName != "triggered sqlite changed" ||
		finalRecord.Status != string(loomruntime.RuntimeOnline) ||
		finalRecord.DiscoverySequence != 2 ||
		finalRecord.StatusSequence != 3 {
		t.Fatalf("final triggered Runtime = %#v", finalRecord)
	}
}

func TestRunTriggeredPreparedRuntimeObservationOnceStaticBoundary(t *testing.T) {
	const filename = "runtime_observation_trigger.go"
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read product: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
	if err != nil {
		t.Fatalf("parse product: %v", err)
	}
	allowed := map[string]bool{
		"context":                          true,
		"errors":                           true,
		"loom-pi-rebuild/internal/runtime": true,
		"loom-pi-rebuild/internal/state":   true,
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
			t.Errorf("forbidden recurrence/concurrency node %T", typed)
		case *ast.SelectorExpr:
			switch typed.Sel.Name {
			case "Rebuild", "Append", "AppendBatch",
				"CommitRuntimeDiscoverySnapshot",
				"CommitRuntimeStatusTransitions",
				"NewTimer", "NewTicker", "After", "Sleep":
				t.Errorf("forbidden selector %q", typed.Sel.Name)
			}
		}
		return true
	})
	product := string(source)
	nilCheck := "nilAppInterface(trigger)"
	awaitCall := "trigger.AwaitRuntimeObservation(ctx)"
	observerCall := "observer.RunOnce(ctx)"
	if strings.Count(product, nilCheck) != 1 ||
		strings.Count(product, awaitCall) != 1 ||
		strings.Count(product, observerCall) != 1 {
		t.Fatal("product must validate/await/delegate exactly once")
	}
	if !(strings.Index(product, nilCheck) <
		strings.Index(product, awaitCall) &&
		strings.Index(product, awaitCall) <
			strings.Index(product, observerCall)) {
		t.Fatal("product must validate then trigger then observe")
	}
	for _, marker := range []string{
		"RunProjectedConfiguredRuntimeObservationOnce(",
		"RunConfiguredRuntimeObservationOnce(",
		"RunRuntimeObservationWriteOnce(",
		"DiscoverConfiguredRuntimes(",
		"internal/journal", "internal/projection",
		"internal/runtime/discoveryscan", "internal/runtime/piadapter",
		"RuntimeInstanceDiscovered", "RuntimeInstanceStatusChanged",
		"IdempotencyKey", "EventID", "EmittedAt",
		"NewTimer", "NewTicker", "After(", "Sleep(",
		"make(chan", "select {", "go ", "for ", "range ",
		"os.", "exec.", "net.", "http.", "sqlite", "daemon",
		"RuntimeProfile", "AgentGrant", "WorkItem", "Bridge",
	} {
		if strings.Contains(product, marker) {
			t.Errorf("forbidden product marker %q", marker)
		}
	}
}

type recordingRuntimeObservationTrigger struct {
	trace *[]string
	await func(context.Context) error
	err   error
	calls int
}

func (trigger *recordingRuntimeObservationTrigger) AwaitRuntimeObservation(
	ctx context.Context,
) error {
	trigger.calls++
	if trigger.trace != nil {
		*trigger.trace = append(*trigger.trace, "trigger")
	}
	if trigger.await != nil {
		return trigger.await(ctx)
	}
	return trigger.err
}

func mustTriggeredPreparedObserver(
	t *testing.T,
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

func triggeredObservationFactory(
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	trace *[]string,
) *appDiscoveryFactory {
	observations := snapshot.Observations()
	probeID := "probe.triggered-observation"
	if len(observations) > 0 {
		probeID = observations[0].SourceProbeID
	}
	return &appDiscoveryFactory{
		name:  "triggered-observation",
		trace: trace,
		build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
			return &appDiscoveryProbe{
				id:           probeID,
				trace:        trace,
				observations: observations,
			}, true, nil
		},
	}
}
