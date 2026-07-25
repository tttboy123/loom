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
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

func TestRunConfiguredRuntimeObservationOncePrevalidatesAndPropagatesDiscovery(
	t *testing.T,
) {
	discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
	statusCommitter := &recordingRuntimeStatusCommitter{}
	oversized := make(
		[]discoveryscan.ProbeFactory,
		discoveryscan.MaxProbeFactories+1,
	)
	for index := range oversized {
		oversized[index] = &appDiscoveryFactory{name: "oversized"}
	}
	var typedNilFactory *appDiscoveryFactory
	invalidSnapshot := appDiscoverySnapshot(t, "cycle-invalid-observation", 1)
	invalidObservations := invalidSnapshot.Observations()
	invalidObservations[0].ModelIDs = []string{""}
	for _, test := range []struct {
		name      string
		ctx       context.Context
		factories []discoveryscan.ProbeFactory
		want      error
	}{
		{
			name: "nil context",
			want: ErrInvalidConfiguredRuntimeObservationRun,
		},
		{
			name: "canceled", ctx: canceledAppDiscoveryContext(),
			want: context.Canceled,
		},
		{
			name: "deadline", ctx: expiredAppDiscoveryContext(),
			want: context.DeadlineExceeded,
		},
		{
			name: "nil factory", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{nil},
			want:      discoveryscan.ErrInvalidConfiguredRuntimeDiscovery,
		},
		{
			name: "factory source error", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{&appDiscoveryFactory{
				name: "failed",
				build: func(context.Context) (
					loomruntime.RuntimeProbe, bool, error,
				) {
					return nil, false, errRuntimeObservationWriteCommitSentinel
				},
			}},
			want: discoveryscan.ErrRuntimeProbeFactoryFailed,
		},
		{
			name: "oversized_factory_set", ctx: context.Background(),
			factories: oversized,
			want:      discoveryscan.ErrInvalidConfiguredRuntimeDiscovery,
		},
		{
			name: "typed_nil_factory", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{typedNilFactory},
			want:      discoveryscan.ErrInvalidConfiguredRuntimeDiscovery,
		},
		{
			name: "present_nil_probe", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{&appDiscoveryFactory{
				name: "present-nil",
				build: func(context.Context) (
					loomruntime.RuntimeProbe, bool, error,
				) {
					return nil, true, nil
				},
			}},
			want: discoveryscan.ErrInvalidConfiguredRuntimeDiscovery,
		},
		{
			name: "absent_non_nil_probe", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{&appDiscoveryFactory{
				name: "absent-non-nil",
				build: func(context.Context) (
					loomruntime.RuntimeProbe, bool, error,
				) {
					return &appDiscoveryProbe{id: "unused"}, false, nil
				},
			}},
			want: discoveryscan.ErrInvalidConfiguredRuntimeDiscovery,
		},
		{
			name: "probe_source_error", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{
				presentAppDiscoveryFactory(
					"probe-error",
					&appDiscoveryProbe{
						id:  "probe.cycle-error",
						err: errRuntimeObservationWriteCommitSentinel,
					},
					nil,
				),
			},
			want: loomruntime.ErrRuntimeDiscoveryFailed,
		},
		{
			name: "invalid_discovered_observation", ctx: context.Background(),
			factories: []discoveryscan.ProbeFactory{
				presentAppDiscoveryFactory(
					"invalid-observation",
					&appDiscoveryProbe{
						id:           "probe.cycle-invalid-observation",
						observations: invalidObservations,
					},
					nil,
				),
			},
			want: loomruntime.ErrInvalidRuntimeModel,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, plan, discovery, reconciliation, status, err :=
				RunConfiguredRuntimeObservationOnce(
					test.ctx, test.factories, projection.Snapshot{},
					discoveryCommitter, statusCommitter,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroConfiguredRuntimeObservationRun(
				t, snapshot, plan, discovery, reconciliation, status,
			)
		})
	}
	if discoveryCommitter.calls != 0 || statusCommitter.calls != 0 {
		t.Fatalf(
			"committer calls = discovery:%d status:%d, want zero",
			discoveryCommitter.calls, statusCommitter.calls,
		)
	}
}

func TestRunConfiguredRuntimeObservationOnceNonePathsUsePathScopedDependencies(
	t *testing.T,
) {
	previous := appDiscoverySnapshot(t, "cycle-none", 1)
	projected := appProjectedRuntimeSnapshot(t, previous, "cycle-none")
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var typedNilStatus *recordingRuntimeStatusCommitter
	absent := &appDiscoveryFactory{name: "absent"}
	unchanged := configuredObservationFactory(previous, nil)

	for _, test := range []struct {
		name      string
		factories []discoveryscan.ProbeFactory
		projected projection.Snapshot
		discovery RuntimeDiscoveryCommitter
		status    RuntimeStatusCommitter
	}{
		{name: "empty"},
		{
			name: "all absent", factories: []discoveryscan.ProbeFactory{absent},
			discovery: typedNilDiscovery, status: typedNilStatus,
		},
		{
			name: "unchanged", factories: []discoveryscan.ProbeFactory{unchanged},
			projected: projected, discovery: typedNilDiscovery,
			status: typedNilStatus,
		},
		{
			name: "absence only", projected: projected,
			discovery: &recordingRuntimeDiscoveryCommitter{
				err: errors.New("must not commit discovery"),
			},
			status: &recordingRuntimeStatusCommitter{
				err: errors.New("must not commit status"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, plan, discovery, reconciliation, status, err :=
				RunConfiguredRuntimeObservationOnce(
					context.Background(), test.factories, test.projected,
					test.discovery, test.status,
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
			if committer, ok :=
				test.discovery.(*recordingRuntimeDiscoveryCommitter); ok &&
				committer != nil && committer.calls != 0 {
				t.Fatalf("discovery calls = %d, want zero", committer.calls)
			}
			if committer, ok :=
				test.status.(*recordingRuntimeStatusCommitter); ok &&
				committer != nil && committer.calls != 0 {
				t.Fatalf("status calls = %d, want zero", committer.calls)
			}
		})
	}
}

func TestRunConfiguredRuntimeObservationOnceDiscoveryPriority(t *testing.T) {
	_, changedStatus, previous := appStatusSources(
		t, "cycle-discovery", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "cycle-discovery")
	mixed := runtimeWritePlanSnapshotFrom(
		t, changedStatus, changedStatus.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "changed inventory"
			return observations
		},
	)
	currentOnly := appDiscoverySnapshot(t, "cycle-current-only", 1)

	for _, test := range []struct {
		name      string
		projected projection.Snapshot
		current   loomruntime.RuntimeDiscoverySnapshot
		mixed     bool
	}{
		{name: "current only", current: currentOnly},
		{name: "mixed inventory and status", projected: projected, current: mixed, mixed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected := mintAppDiscoveryCandidate(
				t, context.Background(), test.current, "cycle-"+test.name,
			)
			discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
				commit: func(
					_ context.Context,
					got loomruntime.RuntimeDiscoverySnapshot,
				) (state.RuntimeDiscoveryCommitCandidate, error) {
					if got.Digest() != test.current.Digest() {
						t.Fatal("committer received wrong snapshot")
					}
					return expected, nil
				},
			}
			statusCommitter := &recordingRuntimeStatusCommitter{
				err: errors.New("discovery path must not write status"),
			}
			snapshot, plan, discovery, reconciliation, status, err :=
				RunConfiguredRuntimeObservationOnce(
					context.Background(),
					[]discoveryscan.ProbeFactory{
						configuredObservationFactory(test.current, nil),
					},
					test.projected, discoveryCommitter, statusCommitter,
				)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Digest() != test.current.Digest() ||
				plan.Kind() != RuntimeObservationWriteDiscovery ||
				discovery.CommitDigest() != expected.CommitDigest() {
				t.Fatal("unexpected discovery result")
			}
			if test.mixed && plan.StatusTransitionCount() != 1 {
				t.Fatalf(
					"mixed status transition count = %d, want 1",
					plan.StatusTransitionCount(),
				)
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

func TestRunConfiguredRuntimeObservationOnceStatusPath(t *testing.T) {
	_, current, previous := appStatusSources(
		t, "cycle-status", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	projected := appProjectedRuntimeSnapshot(t, previous, "cycle-status")
	originalProjected := cloneProjectedRuntimeSnapshot(projected)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var received loomruntime.RuntimeStatusReconciliationCandidate
	statusCommitter := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			received = candidate
			return mintAppStatusCommit(t, ctx, candidate, "cycle-status"), nil
		},
	}
	snapshot, plan, discovery, reconciliation, status, err :=
		RunConfiguredRuntimeObservationOnce(
			context.Background(),
			[]discoveryscan.ProbeFactory{
				configuredObservationFactory(current, nil),
			},
			projected, typedNilDiscovery, statusCommitter,
		)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() != current.Digest() ||
		plan.Kind() != RuntimeObservationWriteStatus {
		t.Fatal("unexpected status snapshot/plan")
	}
	if !reflect.DeepEqual(projected, originalProjected) {
		t.Fatal("caller-supplied projection was mutated")
	}
	assertZeroAppDiscoveryCandidate(t, discovery)
	assertSameAppStatusReconciliation(t, reconciliation, received)
	if !status.Committed() || statusCommitter.calls != 1 {
		t.Fatal("status commit was not returned exactly once")
	}
}

func TestRunConfiguredRuntimeObservationOnceZeroesWriteFailuresWithoutRetry(
	t *testing.T,
) {
	current := appDiscoverySnapshot(t, "cycle-failure", 1)
	factories := []discoveryscan.ProbeFactory{
		configuredObservationFactory(current, nil),
	}
	t.Run("missing selected dependency", func(t *testing.T) {
		snapshot, plan, discovery, reconciliation, status, err :=
			RunConfiguredRuntimeObservationOnce(
				context.Background(), factories, projection.Snapshot{},
				nil, &recordingRuntimeStatusCommitter{},
			)
		if !errors.Is(err, ErrInvalidRuntimeObservationWriteRun) {
			t.Fatalf("error = %v, want invalid write run", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
	})

	t.Run("result mismatch", func(t *testing.T) {
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
		statusCommitter := &recordingRuntimeStatusCommitter{
			err: errors.New("opposite path must not run"),
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunConfiguredRuntimeObservationOnce(
				context.Background(), factories, projection.Snapshot{},
				discoveryCommitter, statusCommitter,
			)
		if !errors.Is(err, ErrRuntimeDiscoveryCommitResultMismatch) {
			t.Fatalf("error = %v, want discovery mismatch", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if discoveryCommitter.calls != 1 || statusCommitter.calls != 0 {
			t.Fatalf(
				"calls = discovery:%d status:%d, want 1/0",
				discoveryCommitter.calls, statusCommitter.calls,
			)
		}
	})

	t.Run("selected source error", func(t *testing.T) {
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
			err: errRuntimeObservationWriteCommitSentinel,
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunConfiguredRuntimeObservationOnce(
				context.Background(), factories, projection.Snapshot{},
				discoveryCommitter, nil,
			)
		if !errors.Is(err, errRuntimeObservationWriteCommitSentinel) {
			t.Fatalf("error = %v, want source sentinel", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if discoveryCommitter.calls != 1 {
			t.Fatalf("calls = %d, want one", discoveryCommitter.calls)
		}
	})

	t.Run("invalid projection and stable identity drift", func(t *testing.T) {
		projected := appProjectedRuntimeSnapshot(t, current, "cycle-failure")
		runtimeID := "runtime.cycle-failure.a"
		invalid := cloneProjectedRuntimeSnapshot(projected)
		record := invalid.RuntimeInstances[runtimeID]
		record.DiscoveryEventID = ""
		invalid.RuntimeInstances[runtimeID] = record
		drift := cloneProjectedRuntimeSnapshot(projected)
		record = drift.RuntimeInstances[runtimeID]
		record.DeviceID = "device.other"
		drift.RuntimeInstances[runtimeID] = record
		for _, test := range []struct {
			name      string
			projected projection.Snapshot
			want      error
		}{
			{
				name: "invalid projection", projected: invalid,
				want: projection.ErrInvalidRuntimeStatusBaselineProjection,
			},
			{
				name: "identity drift", projected: drift,
				want: loomruntime.ErrRuntimeStatusIdentityDrift,
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
				statusCommitter := &recordingRuntimeStatusCommitter{}
				snapshot, plan, discovery, reconciliation, status, err :=
					RunConfiguredRuntimeObservationOnce(
						context.Background(), factories, test.projected,
						discoveryCommitter, statusCommitter,
					)
				if !errors.Is(err, test.want) {
					t.Fatalf("error = %v, want %v", err, test.want)
				}
				assertZeroConfiguredRuntimeObservationRun(
					t, snapshot, plan, discovery, reconciliation, status,
				)
				if discoveryCommitter.calls != 0 || statusCommitter.calls != 0 {
					t.Fatal("planning error reached a committer")
				}
			})
		}
	})

	t.Run("canceled after selected write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		valid := mintAppDiscoveryCandidate(
			t, context.Background(), current, "cycle-canceled",
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
		snapshot, plan, discovery, reconciliation, status, err :=
			RunConfiguredRuntimeObservationOnce(
				ctx, factories, projection.Snapshot{},
				discoveryCommitter, nil,
			)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact canceled", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if discoveryCommitter.calls != 1 {
			t.Fatalf("calls = %d, want one", discoveryCommitter.calls)
		}
	})

	t.Run("explicit retry remains caller owned", func(t *testing.T) {
		valid := mintAppDiscoveryCandidate(
			t, context.Background(), current, "cycle-retry",
		)
		committer := &recordingRuntimeDiscoveryCommitter{candidate: valid}
		var firstSnapshot loomruntime.RuntimeDiscoverySnapshot
		var firstPlan RuntimeObservationWritePlanCandidate
		var firstCommit state.RuntimeDiscoveryCommitCandidate
		for attempt := 0; attempt < 2; attempt++ {
			snapshot, plan, discovery, reconciliation, status, err :=
				RunConfiguredRuntimeObservationOnce(
					context.Background(), factories, projection.Snapshot{},
					committer, nil,
				)
			if err != nil {
				t.Fatal(err)
			}
			assertZeroAppStatusRun(t, reconciliation, status)
			if attempt == 0 {
				firstSnapshot, firstPlan, firstCommit =
					snapshot, plan, discovery
			} else if snapshot.Digest() != firstSnapshot.Digest() ||
				plan != firstPlan ||
				discovery.CommitDigest() != firstCommit.CommitDigest() ||
				!reflect.DeepEqual(discovery.Events(), firstCommit.Events()) {
				t.Fatal("exact retry changed result")
			}
		}
		if committer.calls != 2 {
			t.Fatalf("calls = %d, want one per invocation", committer.calls)
		}
	})
}

func TestRunConfiguredRuntimeObservationOncePreservesOrderAndMutationIsolation(
	t *testing.T,
) {
	source := appDiscoverySnapshot(t, "cycle-order", 2)
	observations := source.Observations()
	var trace []string
	factories := []discoveryscan.ProbeFactory{
		presentAppDiscoveryFactory(
			"left",
			&appDiscoveryProbe{
				id:    observations[0].SourceProbeID + ".left",
				trace: &trace, observations: observations[:1],
			},
			&trace,
		),
		presentAppDiscoveryFactory(
			"right",
			&appDiscoveryProbe{
				id:    observations[1].SourceProbeID + ".right",
				trace: &trace, observations: observations[1:],
			},
			&trace,
		),
	}
	originalSecond := factories[1]
	first := factories[0].(*appDiscoveryFactory)
	first.build = func(context.Context) (
		loomruntime.RuntimeProbe, bool, error,
	) {
		factories[1] = nil
		return &appDiscoveryProbe{
			id:    observations[0].SourceProbeID + ".left",
			trace: &trace, observations: observations[:1],
		}, true, nil
	}
	expectedSnapshot, err := discoveryscan.DiscoverConfiguredRuntimes(
		context.Background(), factories,
	)
	if err != nil {
		t.Fatal(err)
	}
	factories[1] = originalSecond
	trace = nil
	expectedCommit := mintAppDiscoveryCandidate(
		t, context.Background(), expectedSnapshot, "cycle-order",
	)
	committer := &recordingRuntimeDiscoveryCommitter{
		candidate: expectedCommit,
	}
	snapshot, plan, discovery, reconciliation, status, err :=
		RunConfiguredRuntimeObservationOnce(
			context.Background(), factories,
			projection.Snapshot{}, committer, nil,
		)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{
		"factory:left", "factory:right",
		"probe:" + observations[0].SourceProbeID + ".left",
		"probe:" + observations[1].SourceProbeID + ".right",
	}; !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace = %#v, want %#v", trace, want)
	}
	if snapshot.Digest() != expectedSnapshot.Digest() ||
		plan.Kind() != RuntimeObservationWriteDiscovery ||
		discovery.CommitDigest() != expectedCommit.CommitDigest() {
		t.Fatal("ordered result changed")
	}
	if factories[1] != nil {
		t.Fatal("factory callback did not mutate caller slice as required by proof")
	}
	mutated := snapshot.Observations()
	mutated[0].ModelIDs[0] = "mutated"
	if reflect.DeepEqual(mutated, snapshot.Observations()) {
		t.Fatal("snapshot accessor leaked mutation")
	}
	events := discovery.Events()
	events[0].PayloadJSON[0] ^= 0xff
	if reflect.DeepEqual(events, discovery.Events()) {
		t.Fatal("commit accessor leaked mutation")
	}
	assertZeroAppStatusRun(t, reconciliation, status)
}

func TestRunConfiguredRuntimeObservationOnceRealSQLiteDiscoveryThenStatus(
	t *testing.T,
) {
	ctx := context.Background()
	previous := appDiscoverySnapshot(t, "cycle-sqlite", 1)
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx, store, previous, appDiscoveryCommitInput(previous, "cycle-prior"),
	); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "cycle sqlite changed"
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	discoveryProvider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			input := appDiscoveryCommitInput(mixed, "cycle-mixed")
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
		err: errors.New("mixed cycle must not write status"),
	}
	mixedProjected := readModel.Snapshot()
	for attempt := 0; attempt < 2; attempt++ {
		snapshot, plan, _, reconciliation, status, err :=
			RunConfiguredRuntimeObservationOnce(
				ctx,
				[]discoveryscan.ProbeFactory{
					configuredObservationFactory(mixed, nil),
				},
				mixedProjected, discoveryAdapter, statusPoison,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != mixed.Digest() ||
			plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 {
			t.Fatal("mixed configured cycle lost discovery priority")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
	}
	if statusPoison.calls != 0 || appDiscoveryEventCount(t, db) != 2 {
		t.Fatal("mixed retry wrote wrong path or duplicate")
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
			return appStatusCommitInput(candidate, "cycle-status"), nil
		},
	}
	statusAdapter, err := NewPreparedRuntimeStatusCommitter(store, statusProvider)
	if err != nil {
		t.Fatal(err)
	}
	discoveryPoison := &recordingRuntimeDiscoveryCommitter{
		err: errors.New("status cycle must not write discovery"),
	}
	statusProjected := readModel.Snapshot()
	for attempt := 0; attempt < 2; attempt++ {
		snapshot, plan, discovery, reconciliation, status, err :=
			RunConfiguredRuntimeObservationOnce(
				ctx,
				[]discoveryscan.ProbeFactory{
					configuredObservationFactory(statusOnly, nil),
				},
				statusProjected, discoveryPoison, statusAdapter,
			)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Digest() != statusOnly.Digest() ||
			plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("status configured cycle returned wrong path")
		}
		assertZeroAppDiscoveryCandidate(t, discovery)
	}
	if discoveryPoison.calls != 0 || appDiscoveryEventCount(t, db) != 3 {
		t.Fatal("status retry wrote wrong path or duplicate")
	}
	rows, err := db.Query(`SELECT event_type FROM events ORDER BY stream_id, seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var types []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			t.Fatal(err)
		}
		types = append(types, eventType)
	}
	if want := []string{
		"RuntimeInstanceDiscovered",
		"RuntimeInstanceDiscovered",
		"RuntimeInstanceStatusChanged",
	}; !reflect.DeepEqual(types, want) {
		t.Fatalf("event types = %#v, want %#v", types, want)
	}
}

func TestRunConfiguredRuntimeObservationOnceStaticBoundary(t *testing.T) {
	const filename = "runtime_observation_cycle.go"
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
		product, "discoveryscan.DiscoverConfiguredRuntimes(",
	) != 1 {
		t.Fatal("product must call S2-W26 exactly once")
	}
	if strings.Count(product, "RunRuntimeObservationWriteOnce(") != 1 {
		t.Fatal("product must call S2-W33 exactly once")
	}
	for _, marker := range []string{
		"RunConfiguredRuntimeDiscoveryOnce(",
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

func configuredObservationFactory(
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	trace *[]string,
) discoveryscan.ProbeFactory {
	observations := snapshot.Observations()
	probeID := "probe.configured-observation"
	if len(observations) > 0 {
		probeID = observations[0].SourceProbeID
	}
	return presentAppDiscoveryFactory(
		"configured-observation",
		&appDiscoveryProbe{
			id: probeID, trace: trace, observations: observations,
		},
		trace,
	)
}

func assertZeroConfiguredRuntimeObservationRun(
	t *testing.T,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	plan RuntimeObservationWritePlanCandidate,
	discovery state.RuntimeDiscoveryCommitCandidate,
	reconciliation loomruntime.RuntimeStatusReconciliationCandidate,
	status state.RuntimeStatusCommitCandidate,
) {
	t.Helper()
	if snapshot.Digest() != "" || len(snapshot.Observations()) != 0 {
		t.Fatalf("snapshot = %#v, want zero", snapshot)
	}
	assertZeroRuntimeObservationWriteRun(
		t, plan, discovery, reconciliation, status,
	)
}

func TestRunConfiguredRuntimeObservationOnceS2W26ErrorMatrixCoverage(
	t *testing.T,
) {
	source, err := os.ReadFile("runtime_observation_cycle_test.go")
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{
		"oversized_" + "factory_set",
		"typed_" + "nil_factory",
		"present_" + "nil_probe",
		"absent_" + "non_nil_probe",
		"probe_" + "source_error",
		"invalid_" + "discovered_observation",
	}
	for _, marker := range markers {
		if !strings.Contains(string(source), `name: "`+marker+`"`) {
			t.Errorf("missing direct S2-W26 error case %q", marker)
		}
	}
}
