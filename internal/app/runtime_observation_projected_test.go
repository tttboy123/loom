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

func TestRunProjectedConfiguredRuntimeObservationOncePrevalidatesInputs(
	t *testing.T,
) {
	readModel, _, _ := projectedObservationReadModel(t, nil, "projected-prevalidate")
	factory := &appDiscoveryFactory{name: "unused"}
	discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
	statusCommitter := &recordingRuntimeStatusCommitter{}
	for _, test := range []struct {
		name      string
		ctx       context.Context
		readModel *projection.Projection
		want      error
	}{
		{
			name: "nil context", readModel: readModel,
			want: ErrInvalidProjectedConfiguredRuntimeObservationRun,
		},
		{
			name: "nil read model", ctx: context.Background(),
			want: ErrInvalidProjectedConfiguredRuntimeObservationRun,
		},
		{
			name: "canceled", ctx: canceledAppDiscoveryContext(),
			readModel: readModel, want: context.Canceled,
		},
		{
			name: "deadline", ctx: expiredAppDiscoveryContext(),
			readModel: readModel, want: context.DeadlineExceeded,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, plan, discovery, reconciliation, status, err :=
				RunProjectedConfiguredRuntimeObservationOnce(
					test.ctx,
					[]discoveryscan.ProbeFactory{factory},
					test.readModel,
					discoveryCommitter,
					statusCommitter,
				)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroConfiguredRuntimeObservationRun(
				t, snapshot, plan, discovery, reconciliation, status,
			)
		})
	}
	if factory.calls != 0 ||
		discoveryCommitter.calls != 0 ||
		statusCommitter.calls != 0 {
		t.Fatalf(
			"calls = factory:%d discovery:%d status:%d, want zero",
			factory.calls, discoveryCommitter.calls, statusCommitter.calls,
		)
	}
}

func TestRunProjectedConfiguredRuntimeObservationOnceNonePaths(t *testing.T) {
	previous := appDiscoverySnapshot(t, "projected-none", 1)
	emptyReadModel, _, _ := projectedObservationReadModel(
		t, nil, "projected-none-empty",
	)
	populatedReadModel, _, _ := projectedObservationReadModel(
		t, &previous, "projected-none",
	)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	var typedNilStatus *recordingRuntimeStatusCommitter
	for _, test := range []struct {
		name      string
		factories []discoveryscan.ProbeFactory
		readModel *projection.Projection
	}{
		{name: "empty", readModel: emptyReadModel},
		{name: "absence only", readModel: populatedReadModel},
		{
			name: "unchanged",
			factories: []discoveryscan.ProbeFactory{
				configuredObservationFactory(previous, nil),
			},
			readModel: populatedReadModel,
		},
		{
			name: "all absent",
			factories: []discoveryscan.ProbeFactory{
				&appDiscoveryFactory{name: "absent"},
			},
			readModel: populatedReadModel,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, plan, discovery, reconciliation, status, err :=
				RunProjectedConfiguredRuntimeObservationOnce(
					context.Background(), test.factories, test.readModel,
					typedNilDiscovery, typedNilStatus,
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
		})
	}
}

func TestRunProjectedConfiguredRuntimeObservationOnceDiscoveryPriority(
	t *testing.T,
) {
	_, statusChanged, previous := appStatusSources(
		t, "projected-discovery", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	readModel, _, _ := projectedObservationReadModel(
		t, &previous, "projected-discovery",
	)
	projectedBefore := readModel.Snapshot()
	mixed := runtimeWritePlanSnapshotFrom(
		t, statusChanged, statusChanged.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "projected inventory changed"
			return observations
		},
	)
	expected := mintAppDiscoveryCandidate(
		t, context.Background(), mixed, "projected-discovery",
	)
	discoveryCommitter := &recordingRuntimeDiscoveryCommitter{
		candidate: expected,
	}
	statusCommitter := &recordingRuntimeStatusCommitter{
		err: errors.New("mixed path must not write status"),
	}
	snapshot, plan, discovery, reconciliation, status, err :=
		RunProjectedConfiguredRuntimeObservationOnce(
			context.Background(),
			[]discoveryscan.ProbeFactory{
				configuredObservationFactory(mixed, nil),
			},
			readModel, discoveryCommitter, statusCommitter,
		)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() != mixed.Digest() ||
		plan.Kind() != RuntimeObservationWriteDiscovery ||
		plan.StatusTransitionCount() != 1 ||
		discovery.CommitDigest() != expected.CommitDigest() {
		t.Fatal("projected mixed cycle lost discovery priority")
	}
	if !reflect.DeepEqual(projectedBefore, readModel.Snapshot()) {
		t.Fatal("read-model Snapshot was mutated")
	}
	observations := snapshot.Observations()
	observations[0].ModelIDs[0] = "mutated"
	if reflect.DeepEqual(observations, snapshot.Observations()) {
		t.Fatal("returned discovery Snapshot accessor leaked mutation")
	}
	events := discovery.Events()
	events[0].PayloadJSON[0] ^= 0xff
	if reflect.DeepEqual(events, discovery.Events()) {
		t.Fatal("returned discovery commit accessor leaked mutation")
	}
	assertZeroAppStatusRun(t, reconciliation, status)
	if discoveryCommitter.calls != 1 || statusCommitter.calls != 0 {
		t.Fatalf(
			"calls = discovery:%d status:%d, want 1/0",
			discoveryCommitter.calls, statusCommitter.calls,
		)
	}
}

func TestRunProjectedConfiguredRuntimeObservationOnceStatusPath(t *testing.T) {
	_, current, previous := appStatusSources(
		t, "projected-status", 1,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	readModel, _, _ := projectedObservationReadModel(
		t, &previous, "projected-status",
	)
	var typedNilDiscovery *recordingRuntimeDiscoveryCommitter
	statusCommitter := &recordingRuntimeStatusCommitter{
		commit: func(
			ctx context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitCandidate, error) {
			return mintAppStatusCommit(t, ctx, candidate, "projected-status"), nil
		},
	}
	snapshot, plan, discovery, reconciliation, status, err :=
		RunProjectedConfiguredRuntimeObservationOnce(
			context.Background(),
			[]discoveryscan.ProbeFactory{
				configuredObservationFactory(current, nil),
			},
			readModel, typedNilDiscovery, statusCommitter,
		)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Digest() != current.Digest() ||
		plan.Kind() != RuntimeObservationWriteStatus ||
		reconciliation.TransitionCount() != 1 ||
		!status.Committed() {
		t.Fatal("unexpected projected status result")
	}
	assertZeroAppDiscoveryCandidate(t, discovery)
	if statusCommitter.calls != 1 {
		t.Fatalf("status calls = %d, want one", statusCommitter.calls)
	}
}

func TestRunProjectedConfiguredRuntimeObservationOnceZeroesErrorsAndDoesNotRetry(
	t *testing.T,
) {
	current := appDiscoverySnapshot(t, "projected-errors", 1)
	readModel, _, _ := projectedObservationReadModel(
		t, nil, "projected-errors",
	)
	t.Run("configured discovery error", func(t *testing.T) {
		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectedConfiguredRuntimeObservationOnce(
				context.Background(),
				[]discoveryscan.ProbeFactory{nil},
				readModel,
				&recordingRuntimeDiscoveryCommitter{},
				&recordingRuntimeStatusCommitter{},
			)
		if !errors.Is(
			err, discoveryscan.ErrInvalidConfiguredRuntimeDiscovery,
		) {
			t.Fatalf("error = %v, want configured discovery error", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
	})

	t.Run("selected write mismatch", func(t *testing.T) {
		discoveryCommitter := &recordingRuntimeDiscoveryCommitter{}
		statusCommitter := &recordingRuntimeStatusCommitter{
			err: errors.New("opposite path must not run"),
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectedConfiguredRuntimeObservationOnce(
				context.Background(),
				[]discoveryscan.ProbeFactory{
					configuredObservationFactory(current, nil),
				},
				readModel, discoveryCommitter, statusCommitter,
			)
		if !errors.Is(err, ErrRuntimeDiscoveryCommitResultMismatch) {
			t.Fatalf("error = %v, want result mismatch", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if discoveryCommitter.calls != 1 || statusCommitter.calls != 0 {
			t.Fatal("write mismatch retried or used opposite path")
		}
	})

	t.Run("canceled after selected write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		valid := mintAppDiscoveryCandidate(
			t, context.Background(), current, "projected-canceled",
		)
		committer := &recordingRuntimeDiscoveryCommitter{
			commit: func(
				context.Context,
				loomruntime.RuntimeDiscoverySnapshot,
			) (state.RuntimeDiscoveryCommitCandidate, error) {
				cancel()
				return valid, nil
			},
		}
		snapshot, plan, discovery, reconciliation, status, err :=
			RunProjectedConfiguredRuntimeObservationOnce(
				ctx,
				[]discoveryscan.ProbeFactory{
					configuredObservationFactory(current, nil),
				},
				readModel, committer, nil,
			)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact canceled", err)
		}
		assertZeroConfiguredRuntimeObservationRun(
			t, snapshot, plan, discovery, reconciliation, status,
		)
		if committer.calls != 1 {
			t.Fatalf("calls = %d, want one", committer.calls)
		}
	})

	t.Run("explicit retry", func(t *testing.T) {
		valid := mintAppDiscoveryCandidate(
			t, context.Background(), current, "projected-retry",
		)
		committer := &recordingRuntimeDiscoveryCommitter{candidate: valid}
		var firstPlan RuntimeObservationWritePlanCandidate
		var firstCommit state.RuntimeDiscoveryCommitCandidate
		for attempt := 0; attempt < 2; attempt++ {
			_, plan, discovery, reconciliation, status, err :=
				RunProjectedConfiguredRuntimeObservationOnce(
					context.Background(),
					[]discoveryscan.ProbeFactory{
						configuredObservationFactory(current, nil),
					},
					readModel, committer, nil,
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
				t.Fatal("exact caller retry changed results")
			}
		}
		if committer.calls != 2 {
			t.Fatalf("calls = %d, want one per invocation", committer.calls)
		}
	})
}

func TestRunProjectedConfiguredRuntimeObservationOnceRealSQLiteChain(
	t *testing.T,
) {
	ctx := context.Background()
	previous := appDiscoverySnapshot(t, "projected-sqlite", 1)
	readModel, store, db := projectedObservationReadModel(
		t, &previous, "projected-sqlite-prior",
	)
	mixed := runtimeWritePlanSnapshotFrom(
		t, previous, previous.Observations()[0].SourceProbeID,
		func(observations []loomruntime.RuntimeObservation) []loomruntime.RuntimeObservation {
			observations[0].Instance.DisplayName = "projected sqlite changed"
			observations[0].Instance.Status = loomruntime.RuntimeOffline
			return observations
		},
	)
	discoveryProvider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			context.Context,
			loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			input := appDiscoveryCommitInput(mixed, "projected-sqlite-mixed")
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
	var firstDiscovery state.RuntimeDiscoveryCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			RunProjectedConfiguredRuntimeObservationOnce(
				ctx,
				[]discoveryscan.ProbeFactory{
					configuredObservationFactory(mixed, nil),
				},
				readModel, discoveryAdapter, statusPoison,
			)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteDiscovery ||
			plan.StatusTransitionCount() != 1 {
			t.Fatal("mixed projected cycle lost discovery priority")
		}
		assertZeroAppStatusRun(t, reconciliation, status)
		if attempt == 0 {
			firstDiscovery = discovery
		} else if discovery.CommitDigest() != firstDiscovery.CommitDigest() ||
			!reflect.DeepEqual(discovery.Events(), firstDiscovery.Events()) {
			t.Fatal("mixed retry changed exact discovery commit")
		}
	}
	if appDiscoveryEventCount(t, db) != 2 || statusPoison.calls != 0 {
		t.Fatal("mixed retry appended duplicate or status Event")
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
			return appStatusCommitInput(candidate, "projected-sqlite-status"), nil
		},
	}
	statusAdapter, err := NewPreparedRuntimeStatusCommitter(store, statusProvider)
	if err != nil {
		t.Fatal(err)
	}
	discoveryPoison := &recordingRuntimeDiscoveryCommitter{
		err: errors.New("status path must not write discovery"),
	}
	var firstReconciliation loomruntime.RuntimeStatusReconciliationCandidate
	var firstStatus state.RuntimeStatusCommitCandidate
	for attempt := 0; attempt < 2; attempt++ {
		_, plan, discovery, reconciliation, status, err :=
			RunProjectedConfiguredRuntimeObservationOnce(
				ctx,
				[]discoveryscan.ProbeFactory{
					configuredObservationFactory(statusOnly, nil),
				},
				readModel, discoveryPoison, statusAdapter,
			)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Kind() != RuntimeObservationWriteStatus ||
			reconciliation.TransitionCount() != 1 ||
			!status.Committed() {
			t.Fatal("status projected cycle returned wrong facts")
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
				t.Fatal("status retry changed exact status commit")
			}
		}
	}
	if appDiscoveryEventCount(t, db) != 3 || discoveryPoison.calls != 0 {
		t.Fatal("status retry appended duplicate or discovery Event")
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
		RuntimeInstances["runtime.projected-sqlite.a"]
	if finalRecord.DisplayName != "projected sqlite changed" ||
		finalRecord.Status != string(loomruntime.RuntimeOnline) ||
		finalRecord.DiscoverySequence != 2 ||
		finalRecord.StatusSequence != 3 {
		t.Fatalf("final projected Runtime = %#v", finalRecord)
	}
}

func TestRunProjectedConfiguredRuntimeObservationOnceStaticBoundary(
	t *testing.T,
) {
	const filename = "runtime_observation_projected.go"
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
	if strings.Count(product, "readModel.Snapshot()") != 1 {
		t.Fatal("product must read projection Snapshot exactly once")
	}
	if strings.Count(
		product, "RunConfiguredRuntimeObservationOnce(",
	) != 1 {
		t.Fatal("product must call S2-W34 exactly once")
	}
	if strings.Index(product, "readModel.Snapshot()") >
		strings.Index(product, "RunConfiguredRuntimeObservationOnce(") {
		t.Fatal("projection Snapshot must be read before configured discovery")
	}
	for _, marker := range []string{
		"internal/journal", "internal/runtime/piadapter",
		"RuntimeInstanceDiscovered", "RuntimeInstanceStatusChanged",
		"IdempotencyKey", "EventID", "EmittedAt", "Rebuild(",
		"NewTicker", "Sleep(", "os.", "exec.", "net.", "http.", "sqlite",
		"daemon", "RuntimeProfile", "AgentGrant", "WorkItem", "Bridge",
	} {
		if strings.Contains(product, marker) {
			t.Errorf("forbidden product marker %q", marker)
		}
	}
}

func projectedObservationReadModel(
	t *testing.T,
	snapshot *loomruntime.RuntimeDiscoverySnapshot,
	suffix string,
) (*projection.Projection, *journal.Store, *sql.DB) {
	t.Helper()
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if snapshot != nil {
		if _, err := state.CommitRuntimeDiscoverySnapshot(
			context.Background(), store, *snapshot,
			appDiscoveryCommitInput(*snapshot, suffix),
		); err != nil {
			t.Fatal(err)
		}
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	return readModel, store, db
}

func TestRunProjectedConfiguredRuntimeObservationOnceSQLiteExactEventCoverage(
	t *testing.T,
) {
	source, err := os.ReadFile("runtime_observation_projected_test.go")
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{
		"SELECT event_" + "type, seq FROM events",
		"finalRecord.Discovery" + "Sequence != 2",
		"finalRecord.Status" + "Sequence != 3",
	}
	for _, marker := range markers {
		if !strings.Contains(string(source), marker) {
			t.Errorf("missing exact SQLite Event proof marker %q", marker)
		}
	}
}
