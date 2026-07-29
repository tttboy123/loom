package app

import (
	"context"
	"database/sql"
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
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

func TestRunProjectionSynchronizedRuntimeObservationOncePrevalidatesInputs(
	t *testing.T,
) {
	readModel, _, _ := projectedObservationReadModel(
		t, nil, "synchronized-prevalidate",
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
			want:         ErrInvalidProjectionSynchronizedRuntimeObservationRun,
		},
		{
			name:     "nil trigger",
			ctx:      context.Background(),
			observer: observer,
			want:     ErrInvalidProjectionSynchronizedRuntimeObservationRun,
		},
		{
			name:     "typed nil trigger",
			ctx:      context.Background(),
			trigger:  typedNilTrigger,
			observer: observer,
			want:     ErrInvalidProjectionSynchronizedRuntimeObservationRun,
		},
		{
			name:         "nil observer",
			ctx:          context.Background(),
			triggerCount: &recordingRuntimeObservationTrigger{},
			want:         ErrInvalidProjectionSynchronizedRuntimeObservationRun,
		},
		{
			name:         "zero value observer",
			ctx:          context.Background(),
			triggerCount: &recordingRuntimeObservationTrigger{},
			observer:     &PreparedProjectedRuntimeObserver{},
			want:         ErrInvalidProjectionSynchronizedRuntimeObservationRun,
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
				RunProjectionSynchronizedRuntimeObservationOnce(
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

func TestRunProjectionSynchronizedRuntimeObservationOnceTriggerAndRefreshFailures(
	t *testing.T,
) {
	t.Run("trigger error before refresh", func(t *testing.T) {
		readModel, _, db := projectedObservationReadModel(
			t, nil, "synchronized-trigger-error",
		)
		factory := &appDiscoveryFactory{name: "unused"}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			&recordingRuntimeDiscoveryCommitter{},
			&recordingRuntimeStatusCommitter{},
		)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		sentinel := errors.New("synchronized trigger sentinel")
		trigger := &recordingRuntimeObservationTrigger{err: sentinel}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
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
		if trigger.calls != 1 || factory.calls != 0 {
			t.Fatalf(
				"trigger/factory calls = %d/%d, want 1/0",
				trigger.calls,
				factory.calls,
			)
		}
	})

	t.Run("trigger induced cancellation before refresh", func(t *testing.T) {
		readModel, _, _ := projectedObservationReadModel(
			t, nil, "synchronized-trigger-cancel",
		)
		factory := &appDiscoveryFactory{name: "unused"}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			&recordingRuntimeDiscoveryCommitter{},
			&recordingRuntimeStatusCommitter{},
		)
		ctx, cancel := context.WithCancel(context.Background())
		trigger := &recordingRuntimeObservationTrigger{
			await: func(context.Context) error {
				cancel()
				return nil
			},
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
				ctx, trigger, observer,
			)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact canceled", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if trigger.calls != 1 || factory.calls != 0 {
			t.Fatalf(
				"trigger/factory calls = %d/%d, want 1/0",
				trigger.calls,
				factory.calls,
			)
		}
	})

	t.Run("pre observation refresh failure", func(t *testing.T) {
		readModel, _, db := projectedObservationReadModel(
			t, nil, "synchronized-pre-refresh",
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
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		trigger := &recordingRuntimeObservationTrigger{}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
				context.Background(),
				trigger,
				observer,
			)
		if !errors.Is(err, ErrRuntimeObservationProjectionRefresh) {
			t.Fatalf(
				"pre-refresh error = %v, want ErrRuntimeObservationProjectionRefresh",
				err,
			)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if trigger.calls != 1 ||
			factory.calls != 0 ||
			discoveryCommitter.calls != 0 ||
			statusCommitter.calls != 0 {
			t.Fatalf(
				"calls = trigger:%d factory:%d discovery:%d status:%d",
				trigger.calls,
				factory.calls,
				discoveryCommitter.calls,
				statusCommitter.calls,
			)
		}
	})
}

func TestRunProjectionSynchronizedRuntimeObservationOnceSuccessPathsAndOrder(
	t *testing.T,
) {
	const observableProof = "s2_w38_pre_post_refresh_observable"

	t.Run("s2_w38_success_none_exact_order", func(t *testing.T) {
		ctx := context.Background()
		previous := appDiscoverySnapshot(t, "synchronized-none-a", 1)
		late := appDiscoverySnapshot(t, "synchronized-none-b", 1)
		db := openAppDiscoveryJournal(t)
		store := journal.NewStore(db)
		if _, err := state.CommitRuntimeDiscoverySnapshot(
			ctx,
			store,
			previous,
			appDiscoveryCommitInput(previous, "synchronized-none-a"),
		); err != nil {
			t.Fatal(err)
		}
		readModel := projection.New(db)
		var trace []string
		factory := &appDiscoveryFactory{
			name:  "synchronized-none",
			trace: &trace,
			build: func(
				context.Context,
			) (loomruntime.RuntimeProbe, bool, error) {
				priorRecord := readModel.Snapshot().
					RuntimeInstances["runtime.synchronized-none-a.a"]
				if priorRecord.DiscoverySequence != 1 {
					return nil, false, errors.New(
						observableProof + ": pre-refresh did not expose A",
					)
				}
				if _, err := state.CommitRuntimeDiscoverySnapshot(
					ctx,
					store,
					late,
					appDiscoveryCommitInput(
						late,
						"synchronized-none-b",
					),
				); err != nil {
					return nil, false, err
				}
				observations := previous.Observations()
				return &appDiscoveryProbe{
					id:           observations[0].SourceProbeID,
					trace:        &trace,
					observations: observations,
				}, true, nil
			},
		}
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
		statusCommitter := &recordingRuntimeStatusCommitter{}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			discoveryCommitter,
			statusCommitter,
		)
		trigger := &recordingRuntimeObservationTrigger{trace: &trace}

		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
				ctx, trigger, observer,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != previous.Digest() ||
			plan.Kind() != RuntimeObservationWriteNone ||
			plan.SourceDiscoveryDigest() != snapshot.Digest() {
			t.Fatalf("none snapshot/plan = %#v/%#v", snapshot, plan)
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
		assertZeroAppStatusRun(t, reconciliation, status)
		if trigger.calls != 1 ||
			factory.calls != 1 ||
			discoveryCommitter.calls != 0 ||
			statusCommitter.calls != 0 {
			t.Fatalf(
				"none calls = trigger:%d factory:%d discovery:%d status:%d",
				trigger.calls,
				factory.calls,
				discoveryCommitter.calls,
				statusCommitter.calls,
			)
		}
		wantTrace := []string{
			"trigger",
			"factory:synchronized-none",
			"probe:" + previous.Observations()[0].SourceProbeID,
		}
		if !reflect.DeepEqual(trace, wantTrace) {
			t.Fatalf("none trace = %#v, want %#v", trace, wantTrace)
		}
		final := readModel.Snapshot()
		if final.RuntimeInstances["runtime.synchronized-none-a.a"].
			DiscoverySequence != 1 ||
			final.RuntimeInstances["runtime.synchronized-none-b.a"].
				DiscoverySequence != 1 {
			t.Fatalf(
				"%s: none post-refresh snapshot = %#v",
				observableProof,
				final.RuntimeInstances,
			)
		}
	})

	t.Run("s2_w38_success_discovery_exact_order", func(t *testing.T) {
		ctx := context.Background()
		previous := appDiscoverySnapshot(
			t, "synchronized-success-discovery", 1,
		)
		db := openAppDiscoveryJournal(t)
		store := journal.NewStore(db)
		if _, err := state.CommitRuntimeDiscoverySnapshot(
			ctx,
			store,
			previous,
			appDiscoveryCommitInput(
				previous,
				"synchronized-success-discovery-prior",
			),
		); err != nil {
			t.Fatal(err)
		}
		readModel := projection.New(db)
		mixed := runtimeWritePlanSnapshotFrom(
			t, previous, previous.Observations()[0].SourceProbeID,
			func(
				observations []loomruntime.RuntimeObservation,
			) []loomruntime.RuntimeObservation {
				observations[0].Instance.DisplayName =
					"synchronized discovery changed"
				observations[0].Instance.Status =
					loomruntime.RuntimeOffline
				return observations
			},
		)
		var trace []string
		factory := triggeredObservationFactory(mixed, &trace)
		provider := &recordingDiscoveryCommitInputProvider{
			prepare: func(
				context.Context,
				loomruntime.RuntimeDiscoverySnapshot,
			) (state.RuntimeDiscoveryCommitInput, error) {
				input := appDiscoveryCommitInput(
					mixed,
					"synchronized-success-discovery-next",
				)
				input.Events[0].Seq = 2
				return input, nil
			},
		}
		adapter, err := NewPreparedRuntimeDiscoveryCommitter(store, provider)
		if err != nil {
			t.Fatal(err)
		}
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			trace: &trace,
			commit: func(
				callCtx context.Context,
				candidate loomruntime.RuntimeDiscoverySnapshot,
			) (state.RuntimeDiscoveryCommitCandidate, error) {
				record := readModel.Snapshot().RuntimeInstances["runtime.synchronized-success-discovery.a"]
				if record.DiscoverySequence != 1 {
					return state.RuntimeDiscoveryCommitCandidate{},
						errors.New(
							observableProof +
								": discovery pre-refresh missing",
						)
				}
				return adapter.CommitRuntimeDiscovery(callCtx, candidate)
			},
		}
		statusCommitter := &recordingRuntimeStatusCommitter{}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			discoveryCommitter,
			statusCommitter,
		)
		trigger := &recordingRuntimeObservationTrigger{trace: &trace}

		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
				ctx, trigger, observer,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != mixed.Digest() ||
			plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 ||
			!discovery.Committed() {
			t.Fatal("discovery success tuple is not exact")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if trigger.calls != 1 ||
			factory.calls != 1 ||
			discoveryCommitter.calls != 1 ||
			statusCommitter.calls != 0 ||
			provider.calls != 1 {
			t.Fatalf(
				"discovery calls = trigger:%d factory:%d writer:%d opposite:%d provider:%d",
				trigger.calls,
				factory.calls,
				discoveryCommitter.calls,
				statusCommitter.calls,
				provider.calls,
			)
		}
		wantTrace := []string{
			"trigger",
			"factory:triggered-observation",
			"probe:" + mixed.Observations()[0].SourceProbeID,
			"commit",
		}
		if !reflect.DeepEqual(trace, wantTrace) {
			t.Fatalf("discovery trace = %#v, want %#v", trace, wantTrace)
		}
		record := readModel.Snapshot().
			RuntimeInstances["runtime.synchronized-success-discovery.a"]
		if record.DiscoverySequence != 2 ||
			record.DisplayName != "synchronized discovery changed" ||
			record.Status != string(loomruntime.RuntimeOffline) {
			t.Fatalf(
				"%s: discovery post-refresh record = %#v",
				observableProof,
				record,
			)
		}
	})

	t.Run("s2_w38_success_status_exact_order", func(t *testing.T) {
		ctx := context.Background()
		_, current, previous := appStatusSources(
			t,
			"synchronized-success-status",
			1,
			loomruntime.RuntimeOnline,
			loomruntime.RuntimeOffline,
		)
		db := openAppDiscoveryJournal(t)
		store := journal.NewStore(db)
		if _, err := state.CommitRuntimeDiscoverySnapshot(
			ctx,
			store,
			previous,
			appDiscoveryCommitInput(
				previous,
				"synchronized-success-status-prior",
			),
		); err != nil {
			t.Fatal(err)
		}
		readModel := projection.New(db)
		var trace []string
		factory := triggeredObservationFactory(current, &trace)
		provider := &recordingRuntimeStatusCommitInputProvider{
			prepare: func(
				_ context.Context,
				candidate loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitInput, error) {
				return appStatusCommitInput(
					candidate,
					"synchronized-success-status-next",
				), nil
			},
		}
		adapter, err := NewPreparedRuntimeStatusCommitter(store, provider)
		if err != nil {
			t.Fatal(err)
		}
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
		statusCommitter := &recordingRuntimeStatusCommitter{
			commit: func(
				callCtx context.Context,
				candidate loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitCandidate, error) {
				trace = append(trace, "commit-status")
				record := readModel.Snapshot().RuntimeInstances["runtime.synchronized-success-status.a"]
				if record.DiscoverySequence != 1 ||
					record.Status != string(loomruntime.RuntimeOnline) {
					return state.RuntimeStatusCommitCandidate{},
						errors.New(
							observableProof +
								": status pre-refresh missing",
						)
				}
				return adapter.CommitRuntimeStatus(callCtx, candidate)
			},
		}
		observer := mustTriggeredPreparedObserver(
			t,
			[]discoveryscan.ProbeFactory{factory},
			readModel,
			discoveryCommitter,
			statusCommitter,
		)
		trigger := &recordingRuntimeObservationTrigger{trace: &trace}

		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectionSynchronizedRuntimeObservationOnce(
				ctx, trigger, observer,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != current.Digest() ||
			plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("status success tuple is not exact")
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
		if trigger.calls != 1 ||
			factory.calls != 1 ||
			discoveryCommitter.calls != 0 ||
			statusCommitter.calls != 1 ||
			provider.calls != 1 {
			t.Fatalf(
				"status calls = trigger:%d factory:%d opposite:%d writer:%d provider:%d",
				trigger.calls,
				factory.calls,
				discoveryCommitter.calls,
				statusCommitter.calls,
				provider.calls,
			)
		}
		wantTrace := []string{
			"trigger",
			"factory:triggered-observation",
			"probe:" + current.Observations()[0].SourceProbeID,
			"commit-status",
		}
		if !reflect.DeepEqual(trace, wantTrace) {
			t.Fatalf("status trace = %#v, want %#v", trace, wantTrace)
		}
		record := readModel.Snapshot().
			RuntimeInstances["runtime.synchronized-success-status.a"]
		if record.DiscoverySequence != 1 ||
			record.StatusSequence != 2 ||
			record.Status != string(loomruntime.RuntimeOffline) {
			t.Fatalf(
				"%s: status post-refresh record = %#v",
				observableProof,
				record,
			)
		}
	})
}

func TestRunProjectionSynchronizedRuntimeObservationOnceDownstreamErrorMatrix(
	t *testing.T,
) {
	emptyReadModel, _, _ := projectedObservationReadModel(
		t, nil, "synchronized-errors-empty",
	)
	discoverySnapshot := appDiscoverySnapshot(
		t, "synchronized-errors-discovery", 1,
	)
	validDiscovery := mintAppDiscoveryCandidate(
		t,
		context.Background(),
		discoverySnapshot,
		"synchronized-errors-discovery",
	)
	_, statusSnapshot, statusPrevious := appStatusSources(
		t,
		"synchronized-errors-status",
		1,
		loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
	)
	statusReadModel, _, _ := projectedObservationReadModel(
		t, &statusPrevious, "synchronized-errors-status",
	)

	configuredError := errors.New("synchronized configured discovery error")
	configuredFactory := &appDiscoveryFactory{
		name: "synchronized-configured-error",
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
			"synchronized-errors-identity",
			1,
			loomruntime.RuntimeOffline,
			"device.other",
		),
		nil,
	)
	_, _, identityPrevious := appStatusSources(
		t,
		"synchronized-errors-identity",
		1,
		loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
	)
	identityReadModel, _, _ := projectedObservationReadModel(
		t, &identityPrevious, "synchronized-errors-identity",
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
	discoveryCommitError := errors.New("synchronized discovery commit error")
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
	statusCommitError := errors.New("synchronized status commit error")
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
				RunProjectionSynchronizedRuntimeObservationOnce(
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

func TestRunProjectionSynchronizedRuntimeObservationOnceRetainsSuccessfulTupleOnPostRefreshFailure(
	t *testing.T,
) {
	ctx := context.Background()
	readModel, store, db := projectedObservationReadModel(
		t, nil, "synchronized-post-refresh",
	)
	databasePath := runtimeObservationLoopDatabasePath(t, db)
	before := readModel.Snapshot()
	snapshot := appDiscoverySnapshot(t, "synchronized-post-refresh", 1)
	provider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			return appDiscoveryCommitInput(
				snapshot,
				"synchronized-post-refresh",
			), nil
		},
	}
	adapter, err := NewPreparedRuntimeDiscoveryCommitter(store, provider)
	if err != nil {
		t.Fatal(err)
	}
	closingCommitter := &recordingRuntimeDiscoveryCommitter{
		commit: func(
			callCtx context.Context,
			candidate loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitCandidate, error) {
			committed, commitErr := adapter.CommitRuntimeDiscovery(
				callCtx, candidate,
			)
			if commitErr == nil {
				_ = db.Close()
			}
			return committed, commitErr
		},
	}
	statusCommitter := &recordingRuntimeStatusCommitter{}
	factory := triggeredObservationFactory(snapshot, nil)
	observer := mustTriggeredPreparedObserver(
		t,
		[]discoveryscan.ProbeFactory{factory},
		readModel,
		closingCommitter,
		statusCommitter,
	)
	trigger := &recordingRuntimeObservationTrigger{}

	gotSnapshot, plan, discovery, reconciliation, status, err :=
		RunProjectionSynchronizedRuntimeObservationOnce(
			ctx, trigger, observer,
		)
	if !errors.Is(err, ErrRuntimeObservationProjectionRefresh) {
		t.Fatalf(
			"post-refresh error = %v, want ErrRuntimeObservationProjectionRefresh",
			err,
		)
	}
	if gotSnapshot.Digest() != snapshot.Digest() ||
		plan.Kind() != RuntimeObservationWriteDiscovery ||
		discovery.CommitDigest() == "" {
		t.Fatalf(
			"successful tuple was lost: snapshot=%#v plan=%#v discovery=%#v",
			gotSnapshot,
			plan,
			discovery,
		)
	}
	assertZeroAppStatusRun(t, reconciliation, status)
	if trigger.calls != 1 ||
		factory.calls != 1 ||
		closingCommitter.calls != 1 ||
		statusCommitter.calls != 0 ||
		provider.calls != 1 {
		t.Fatalf(
			"calls = trigger:%d factory:%d discovery:%d status:%d provider:%d",
			trigger.calls,
			factory.calls,
			closingCommitter.calls,
			statusCommitter.calls,
			provider.calls,
		)
	}
	if got := readModel.Snapshot(); !reflect.DeepEqual(got, before) {
		t.Fatal("failed post-refresh replaced the previous projection")
	}

	reopened, openErr := sql.Open("sqlite", "file:"+databasePath)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer reopened.Close()
	var count int
	if queryErr := reopened.QueryRow(
		`SELECT COUNT(*) FROM events`,
	).Scan(&count); queryErr != nil {
		t.Fatal(queryErr)
	}
	if count != 1 {
		t.Fatalf("authoritative event count = %d, want one", count)
	}
}

func TestRunProjectionSynchronizedRuntimeObservationOnceRealSQLiteChain(
	t *testing.T,
) {
	ctx := context.Background()
	previous := appDiscoverySnapshot(t, "synchronized-sqlite", 1)
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		store,
		previous,
		appDiscoveryCommitInput(previous, "synchronized-sqlite-prior"),
	); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)

	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "synchronized sqlite changed"
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	statusOnly := runtimeWritePlanSnapshotFrom(
		t, mixed, mixed.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.Status = loomruntime.RuntimeOnline
			return observations
		},
	)
	discoveryProvider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			input := appDiscoveryCommitInput(
				mixed,
				"synchronized-sqlite-mixed",
			)
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
	statusProvider := &recordingRuntimeStatusCommitInputProvider{
		prepare: func(
			_ context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitInput, error) {
			return appStatusCommitInput(
				candidate,
				"synchronized-sqlite-status",
			), nil
		},
	}
	statusAdapter, err := NewPreparedRuntimeStatusCommitter(
		store, statusProvider,
	)
	if err != nil {
		t.Fatal(err)
	}

	var scriptedCalls int
	factory := &appDiscoveryFactory{
		name: "synchronized-sqlite-script",
		build: func(
			context.Context,
		) (loomruntime.RuntimeProbe, bool, error) {
			scriptedCalls++
			selected := mixed
			if scriptedCalls == 2 {
				selected = statusOnly
			}
			if scriptedCalls > 2 {
				return nil, false, errors.New("unexpected scripted call")
			}
			observations := selected.Observations()
			return &appDiscoveryProbe{
				id:           observations[0].SourceProbeID,
				observations: observations,
			}, true, nil
		},
	}
	bindings := []discoveryscan.ProbeFactory{factory}
	observer := mustTriggeredPreparedObserver(
		t,
		bindings,
		readModel,
		discoveryAdapter,
		statusAdapter,
	)
	bindings[0] = &appDiscoveryFactory{
		name: "caller replacement must not be used",
		build: func(
			context.Context,
		) (loomruntime.RuntimeProbe, bool, error) {
			return nil, false, errors.New("caller replacement used")
		},
	}
	trigger := &recordingRuntimeObservationTrigger{}

	firstSnapshot, firstPlan, firstDiscovery,
		firstReconciliation, firstStatus, err :=
		RunProjectionSynchronizedRuntimeObservationOnce(
			ctx, trigger, observer,
		)
	if err != nil {
		t.Fatal(err)
	}
	if firstSnapshot.Digest() != mixed.Digest() ||
		firstPlan.Kind() != RuntimeObservationWriteDiscovery ||
		firstPlan.StatusTransitionCount() != 1 ||
		!firstDiscovery.Committed() {
		t.Fatal("first synchronized cycle lost discovery priority")
	}
	assertZeroAppStatusRun(t, firstReconciliation, firstStatus)
	firstRecord := readModel.Snapshot().
		RuntimeInstances["runtime.synchronized-sqlite.a"]
	if firstRecord.DisplayName != "synchronized sqlite changed" ||
		firstRecord.Status != string(loomruntime.RuntimeOffline) ||
		firstRecord.DiscoverySequence != 2 {
		t.Fatalf("post-discovery projection = %#v", firstRecord)
	}
	mutatedObservations := firstSnapshot.Observations()
	mutatedObservations[0].Instance.DisplayName = "mutated caller copy"
	mutatedEvents := firstDiscovery.Events()
	mutatedEvents[0].PayloadJSON = []byte(`{"mutated":true}`)
	if got := readModel.Snapshot().
		RuntimeInstances["runtime.synchronized-sqlite.a"].
		DisplayName; got != "synchronized sqlite changed" {
		t.Fatalf("caller result mutation changed projection: %q", got)
	}

	secondSnapshot, secondPlan, secondDiscovery,
		secondReconciliation, secondStatus, err :=
		RunProjectionSynchronizedRuntimeObservationOnce(
			ctx, trigger, observer,
		)
	if err != nil {
		t.Fatal(err)
	}
	if secondSnapshot.Digest() != statusOnly.Digest() ||
		secondPlan.Kind() != RuntimeObservationWriteStatus ||
		secondReconciliation.TransitionCount() != 1 ||
		!secondStatus.Committed() {
		t.Fatal("second synchronized cycle did not select status")
	}
	assertZeroAppDiscoveryCandidate(t, secondDiscovery)
	if trigger.calls != 2 ||
		factory.calls != 2 ||
		scriptedCalls != 2 ||
		discoveryProvider.calls != 1 ||
		statusProvider.calls != 1 {
		t.Fatalf(
			"calls = trigger:%d factory:%d scripted:%d discovery:%d status:%d",
			trigger.calls,
			factory.calls,
			scriptedCalls,
			discoveryProvider.calls,
			statusProvider.calls,
		)
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
	finalRecord := readModel.Snapshot().
		RuntimeInstances["runtime.synchronized-sqlite.a"]
	if finalRecord.DisplayName != "synchronized sqlite changed" ||
		finalRecord.Status != string(loomruntime.RuntimeOnline) ||
		finalRecord.DiscoverySequence != 2 ||
		finalRecord.StatusSequence != 3 {
		t.Fatalf("final synchronized Runtime = %#v", finalRecord)
	}
}

func TestRunProjectionSynchronizedRuntimeObservationOnceStaticBoundary(
	t *testing.T,
) {
	const filename = "runtime_observation_loop.go"
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
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt, *ast.ForStmt, *ast.RangeStmt:
			t.Errorf("forbidden recurrence/concurrency node %T", typed)
		case *ast.SelectorExpr:
			switch typed.Sel.Name {
			case "Append", "AppendBatch",
				"CommitRuntimeDiscoverySnapshot",
				"CommitRuntimeStatusTransitions",
				"NewTimer", "NewTicker", "After", "Sleep":
				t.Errorf("forbidden selector %q", typed.Sel.Name)
			}
		}
		return true
	})
	product := string(source)
	if strings.Count(
		product,
		"trigger.AwaitRuntimeObservation(ctx)",
	) != 1 {
		t.Fatal("product must await the underlying trigger exactly once")
	}
	if strings.Count(
		product,
		"RunTriggeredPreparedRuntimeObservationOnce(",
	) != 1 {
		t.Fatal("product must call exact S2-W37 once")
	}
	if strings.Count(product, "readModel.Rebuild(ctx)") != 2 {
		t.Fatal("product must pre-refresh and post-refresh exactly once")
	}
	if strings.Count(product, "readModel := observer.readModel") != 1 {
		t.Fatal("product must capture the exact observer read model once")
	}
	for _, marker := range []string{
		"RunProjectedConfiguredRuntimeObservationOnce(",
		"RunConfiguredRuntimeObservationOnce(",
		"RunRuntimeObservationWriteOnce(",
		"DiscoverConfiguredRuntimes(",
		"internal/journal",
		"internal/runtime/discoveryscan",
		"internal/runtime/piadapter",
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

func TestRunProjectionSynchronizedRuntimeObservationOnceRepair1SuccessCoverage(
	t *testing.T,
) {
	source, err := os.ReadFile("runtime_observation_loop_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"s2_" + "w38_success_none_exact_order",
		"s2_" + "w38_success_discovery_exact_order",
		"s2_" + "w38_success_status_exact_order",
		"s2_" + "w38_pre_post_refresh_observable",
	} {
		if count := strings.Count(string(source), marker); count != 1 {
			t.Errorf("coverage marker %q count = %d, want one", marker, count)
		}
	}
}

func runtimeObservationLoopDatabasePath(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var sequence int
		var name string
		var path string
		if err := rows.Scan(&sequence, &name, &path); err != nil {
			t.Fatal(err)
		}
		if name == "main" {
			return path
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("main database path not found")
	return ""
}
