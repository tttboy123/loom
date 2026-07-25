package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"

	_ "modernc.org/sqlite"
)

func TestRunConfiguredRuntimeDiscoveryOncePrevalidatesInputs(t *testing.T) {
	factory := &appDiscoveryFactory{name: "unused"}
	snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
		nil,
		[]discoveryscan.ProbeFactory{factory},
		&recordingRuntimeDiscoveryCommitter{},
	)
	if !errors.Is(err, ErrInvalidRuntimeDiscoveryRun) {
		t.Fatalf("nil context error = %v, want ErrInvalidRuntimeDiscoveryRun", err)
	}
	assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
	if factory.calls != 0 {
		t.Fatalf("factory calls = %d, want 0", factory.calls)
	}

	for _, tt := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "canceled", ctx: canceledAppDiscoveryContext(), want: context.Canceled},
		{name: "deadline", ctx: expiredAppDiscoveryContext(), want: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			factory := &appDiscoveryFactory{name: "unused"}
			committer := &recordingRuntimeDiscoveryCommitter{}
			snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
				tt.ctx,
				[]discoveryscan.ProbeFactory{factory},
				committer,
			)
			if err != tt.want {
				t.Fatalf("error = %v, want exact %v", err, tt.want)
			}
			assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
			if factory.calls != 0 || committer.calls != 0 {
				t.Fatalf("calls = factory %d committer %d, want zero", factory.calls, committer.calls)
			}
		})
	}

	t.Run("nil and typed nil committer", func(t *testing.T) {
		for _, committer := range []RuntimeDiscoveryCommitter{
			nil,
			(*recordingRuntimeDiscoveryCommitter)(nil),
		} {
			factory := &appDiscoveryFactory{name: "unused"}
			snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
				context.Background(),
				[]discoveryscan.ProbeFactory{factory},
				committer,
			)
			if !errors.Is(err, ErrInvalidRuntimeDiscoveryRun) {
				t.Fatalf("committer %#v error = %v, want ErrInvalidRuntimeDiscoveryRun", committer, err)
			}
			assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
			if factory.calls != 0 {
				t.Fatalf("factory calls = %d, want 0", factory.calls)
			}
		}
	})
}

func TestRunConfiguredRuntimeDiscoveryOnceSkipsCommitForEmptyScan(t *testing.T) {
	committer := &recordingRuntimeDiscoveryCommitter{}
	empty, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		nil,
		committer,
	)
	if err != nil {
		t.Fatalf("empty run error = %v", err)
	}
	assertValidEmptyAppDiscoverySnapshot(t, empty)
	assertZeroAppDiscoveryCandidate(t, candidate)
	if committer.calls != 0 {
		t.Fatalf("empty committer calls = %d, want 0", committer.calls)
	}

	var trace []string
	allAbsent, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		[]discoveryscan.ProbeFactory{
			&appDiscoveryFactory{name: "first", trace: &trace},
			&appDiscoveryFactory{name: "second", trace: &trace},
		},
		committer,
	)
	if err != nil {
		t.Fatalf("all-absent run error = %v", err)
	}
	assertValidEmptyAppDiscoverySnapshot(t, allAbsent)
	assertZeroAppDiscoveryCandidate(t, candidate)
	if empty.Digest() != allAbsent.Digest() {
		t.Fatalf("empty digest = %q, all-absent = %q", empty.Digest(), allAbsent.Digest())
	}
	if got, want := trace, []string{"factory:first", "factory:second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("trace = %#v, want %#v", got, want)
	}
	if committer.calls != 0 {
		t.Fatalf("all-absent committer calls = %d, want 0", committer.calls)
	}
}

func TestRunConfiguredRuntimeDiscoveryOnceCommitsExactSnapshot(t *testing.T) {
	var trace []string
	alpha := &appDiscoveryProbe{
		id:    "alpha",
		trace: &trace,
		observations: []loomruntime.RuntimeObservation{{
			Instance: appDiscoveryInstance("runtime.alpha", "Alpha"),
			ModelIDs: []string{"model.z", "model.a"},
		}},
	}
	bravo := &appDiscoveryProbe{
		id:    "bravo",
		trace: &trace,
		observations: []loomruntime.RuntimeObservation{{
			Instance: appDiscoveryInstance("runtime.bravo", "Bravo"),
			ModelIDs: []string{"model.c", "model.b"},
		}},
	}
	committer := &recordingRuntimeDiscoveryCommitter{
		trace: &trace,
		commit: func(
			ctx context.Context,
			snapshot loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitCandidate, error) {
			received := snapshot.Observations()
			received[0].Instance.ObservedCapabilities[0] = "mutated"
			received[0].ModelIDs[0] = "mutated"
			return mintAppDiscoveryCandidate(t, ctx, snapshot, "exact"), nil
		},
	}

	snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		[]discoveryscan.ProbeFactory{
			presentAppDiscoveryFactory("bravo", bravo, &trace),
			&appDiscoveryFactory{name: "absent", trace: &trace},
			presentAppDiscoveryFactory("alpha", alpha, &trace),
		},
		committer,
	)
	if err != nil {
		t.Fatalf("RunConfiguredRuntimeDiscoveryOnce() error = %v", err)
	}
	if got, want := trace, []string{
		"factory:bravo",
		"factory:absent",
		"factory:alpha",
		"probe:alpha",
		"probe:bravo",
		"commit",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("trace = %#v, want %#v", got, want)
	}
	if committer.calls != 1 {
		t.Fatalf("committer calls = %d, want 1", committer.calls)
	}
	if !candidate.Committed() ||
		candidate.SourceDiscoveryDigest() != snapshot.Digest() ||
		candidate.EventCount() != 2 ||
		len(candidate.Events()) != 2 ||
		!isAppDiscoveryDigest(candidate.CommitDigest()) {
		t.Fatalf("candidate = %#v, snapshot digest = %q", candidate, snapshot.Digest())
	}
	observations := snapshot.Observations()
	if got, want := []string{
		observations[0].Instance.ID,
		observations[1].Instance.ID,
	}, []string{"runtime.alpha", "runtime.bravo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot order = %#v, want %#v", got, want)
	}
	if observations[0].Instance.ObservedCapabilities[0] != "apply_patch" ||
		observations[0].ModelIDs[0] != "model.a" {
		t.Fatalf("committer mutation escaped: %#v", observations[0])
	}

	events := candidate.Events()
	events[0].PayloadJSON[0] = '['
	if candidate.Events()[0].PayloadJSON[0] == '[' {
		t.Fatal("candidate Events accessor mutation escaped")
	}
	observations[0].ModelIDs[0] = "changed"
	if snapshot.Observations()[0].ModelIDs[0] != "model.a" {
		t.Fatal("snapshot Observations accessor mutation escaped")
	}
}

func TestRunConfiguredRuntimeDiscoveryOnceDiscoveryFailuresPreventCommit(t *testing.T) {
	sourceErr := errors.New("factory failed")
	for _, tt := range []struct {
		name    string
		factory discoveryscan.ProbeFactory
		want    error
	}{
		{
			name: "factory failure",
			factory: &appDiscoveryFactory{
				name: "factory-error",
				build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
					return nil, false, sourceErr
				},
			},
			want: sourceErr,
		},
		{
			name: "probe failure",
			factory: presentAppDiscoveryFactory(
				"probe-error",
				&appDiscoveryProbe{id: "probe", err: sourceErr},
				nil,
			),
			want: sourceErr,
		},
		{
			name: "invalid probe",
			factory: presentAppDiscoveryFactory(
				"invalid-probe",
				&appDiscoveryProbe{id: ""},
				nil,
			),
			want: loomruntime.ErrInvalidRuntimeProbe,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			committer := &recordingRuntimeDiscoveryCommitter{}
			snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
				context.Background(),
				[]discoveryscan.ProbeFactory{tt.factory},
				committer,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
			if committer.calls != 0 {
				t.Fatalf("committer calls = %d, want 0", committer.calls)
			}
		})
	}

	t.Run("canceled during observation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		probe := &appDiscoveryProbe{
			id: "cancel",
			observe: func(context.Context) ([]loomruntime.RuntimeObservation, error) {
				cancel()
				return nil, context.Canceled
			},
		}
		committer := &recordingRuntimeDiscoveryCommitter{}
		snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
			ctx,
			[]discoveryscan.ProbeFactory{presentAppDiscoveryFactory("cancel", probe, nil)},
			committer,
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
		if committer.calls != 0 {
			t.Fatalf("committer calls = %d, want 0", committer.calls)
		}
	})
}

func TestRunConfiguredRuntimeDiscoveryOnceRejectsCommitFailures(t *testing.T) {
	factory := presentAppDiscoveryFactory(
		"present",
		&appDiscoveryProbe{
			id: "probe",
			observations: []loomruntime.RuntimeObservation{{
				Instance: appDiscoveryInstance("runtime.one", "One"),
			}},
		},
		nil,
	)

	t.Run("source error remains inspectable without disclosure", func(t *testing.T) {
		sourceErr := errors.New("commit-source-failure")
		committer := &recordingRuntimeDiscoveryCommitter{
			secret: "secret-committer-value",
			err:    sourceErr,
		}
		snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
			context.Background(),
			[]discoveryscan.ProbeFactory{factory},
			committer,
		)
		if !errors.Is(err, sourceErr) {
			t.Fatalf("error = %v, want source error", err)
		}
		if strings.Contains(err.Error(), committer.secret) {
			t.Fatalf("error disclosed committer value: %v", err)
		}
		assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
		if committer.calls != 1 {
			t.Fatalf("committer calls = %d, want 1", committer.calls)
		}
	})

	t.Run("zero candidate", func(t *testing.T) {
		committer := &recordingRuntimeDiscoveryCommitter{}
		snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
			context.Background(),
			[]discoveryscan.ProbeFactory{factory},
			committer,
		)
		if !errors.Is(err, ErrRuntimeDiscoveryCommitResultMismatch) {
			t.Fatalf("error = %v, want ErrRuntimeDiscoveryCommitResultMismatch", err)
		}
		assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
		if committer.calls != 1 {
			t.Fatalf("committer calls = %d, want 1", committer.calls)
		}
	})

	t.Run("wrong source and count", func(t *testing.T) {
		for _, tt := range []struct {
			name  string
			count int
		}{
			{name: "wrong source", count: 1},
			{name: "wrong count", count: 2},
		} {
			t.Run(tt.name, func(t *testing.T) {
				alternate := appDiscoverySnapshot(t, "alternate", tt.count)
				wrong := mintAppDiscoveryCandidate(t, context.Background(), alternate, tt.name)
				committer := &recordingRuntimeDiscoveryCommitter{candidate: wrong}
				snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
					context.Background(),
					[]discoveryscan.ProbeFactory{factory},
					committer,
				)
				if !errors.Is(err, ErrRuntimeDiscoveryCommitResultMismatch) {
					t.Fatalf("error = %v, want ErrRuntimeDiscoveryCommitResultMismatch", err)
				}
				assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
				if committer.calls != 1 {
					t.Fatalf("committer calls = %d, want 1", committer.calls)
				}
			})
		}
	})

	t.Run("canceled by committer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		committer := &recordingRuntimeDiscoveryCommitter{
			commit: func(
				commitCtx context.Context,
				snapshot loomruntime.RuntimeDiscoverySnapshot,
			) (state.RuntimeDiscoveryCommitCandidate, error) {
				candidate := mintAppDiscoveryCandidate(t, commitCtx, snapshot, "cancel")
				cancel()
				return candidate, nil
			},
		}
		snapshot, candidate, err := RunConfiguredRuntimeDiscoveryOnce(
			ctx,
			[]discoveryscan.ProbeFactory{factory},
			committer,
		)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact context.Canceled", err)
		}
		assertZeroRuntimeDiscoveryRun(t, snapshot, candidate)
		if committer.calls != 1 {
			t.Fatalf("committer calls = %d, want 1", committer.calls)
		}
	})
}

func TestRunConfiguredRuntimeDiscoveryOnceRealJournalRetry(t *testing.T) {
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	committer := &boundAppDiscoveryCommitter{appender: store, suffix: "sqlite"}
	newFactory := func() discoveryscan.ProbeFactory {
		return presentAppDiscoveryFactory(
			"sqlite",
			&appDiscoveryProbe{
				id: "probe.sqlite",
				observations: []loomruntime.RuntimeObservation{{
					Instance: appDiscoveryInstance("runtime.sqlite", "SQLite"),
					ModelIDs: []string{"model.sqlite"},
				}},
			},
			nil,
		)
	}

	firstSnapshot, first, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		[]discoveryscan.ProbeFactory{newFactory()},
		committer,
	)
	if err != nil {
		t.Fatalf("first run error = %v", err)
	}
	if !first.Committed() || first.EventCount() != 1 ||
		first.SourceDiscoveryDigest() != firstSnapshot.Digest() {
		t.Fatalf("first candidate = %#v", first)
	}
	if got := appDiscoveryEventCount(t, db); got != 1 {
		t.Fatalf("first row count = %d, want 1", got)
	}

	secondSnapshot, second, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		[]discoveryscan.ProbeFactory{newFactory()},
		committer,
	)
	if err != nil {
		t.Fatalf("retry run error = %v", err)
	}
	if secondSnapshot.Digest() != firstSnapshot.Digest() ||
		second.CommitDigest() != first.CommitDigest() {
		t.Fatalf("retry changed result: snapshots (%q,%q), commits (%q,%q)",
			firstSnapshot.Digest(),
			secondSnapshot.Digest(),
			first.CommitDigest(),
			second.CommitDigest(),
		)
	}
	if got := appDiscoveryEventCount(t, db); got != 1 {
		t.Fatalf("retry row count = %d, want 1", got)
	}
	if committer.calls != 2 {
		t.Fatalf("committer calls = %d, want 2 across explicit runs", committer.calls)
	}
}

func TestRunConfiguredRuntimeDiscoveryOnceStaticBoundary(t *testing.T) {
	const filename = "runtime_discovery.go"
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", filename, err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", filename, err)
	}
	allowed := map[string]bool{
		"context":                          true,
		"errors":                           true,
		"reflect":                          true,
		"strings":                          true,
		"loom-pi-rebuild/internal/runtime": true,
		"loom-pi-rebuild/internal/runtime/discoveryscan": true,
		"loom-pi-rebuild/internal/state":                 true,
	}
	for _, imported := range parsed.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !allowed[path] {
			t.Fatalf("%s imports forbidden package %q", filename, path)
		}
	}
	for _, forbidden := range []string{
		"internal/journal",
		"internal/projection",
		"piadapter",
		"sql.",
		"os.",
		"exec.",
		"net.",
		"time.Now(",
		"uuid",
		"ticker",
		"Ticker",
		"Sleep(",
		"go func",
		"Reconcile",
		"StatusChanged",
		"Activate",
		"Reserve",
		"AppendBatch",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("%s contains forbidden behavior marker %q", filename, forbidden)
		}
	}
	for _, required := range []string{
		"candidate.SourceDiscoveryDigest() == snapshot.Digest()",
		"candidate.EventCount() == observationCount",
		"len(candidate.Events()) == observationCount",
		"validAppDiscoveryDigest(candidate.CommitDigest())",
	} {
		if !strings.Contains(string(source), required) {
			t.Fatalf("%s is missing result validation %q", filename, required)
		}
	}
}

type appDiscoveryFactory struct {
	name  string
	trace *[]string
	build func(context.Context) (loomruntime.RuntimeProbe, bool, error)
	calls int
}

func (f *appDiscoveryFactory) BuildProbe(
	ctx context.Context,
) (loomruntime.RuntimeProbe, bool, error) {
	f.calls++
	if f.trace != nil {
		*f.trace = append(*f.trace, "factory:"+f.name)
	}
	if f.build != nil {
		return f.build(ctx)
	}
	return nil, false, nil
}

type appDiscoveryProbe struct {
	id           string
	trace        *[]string
	observations []loomruntime.RuntimeObservation
	err          error
	observe      func(context.Context) ([]loomruntime.RuntimeObservation, error)
}

func (p *appDiscoveryProbe) ID() string {
	return p.id
}

func (p *appDiscoveryProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	if p.trace != nil {
		*p.trace = append(*p.trace, "probe:"+p.id)
	}
	if p.observe != nil {
		return p.observe(ctx)
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.observations, nil
}

type recordingRuntimeDiscoveryCommitter struct {
	trace     *[]string
	commit    func(context.Context, loomruntime.RuntimeDiscoverySnapshot) (state.RuntimeDiscoveryCommitCandidate, error)
	candidate state.RuntimeDiscoveryCommitCandidate
	err       error
	secret    string
	calls     int
}

func (c *recordingRuntimeDiscoveryCommitter) CommitRuntimeDiscovery(
	ctx context.Context,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) (state.RuntimeDiscoveryCommitCandidate, error) {
	c.calls++
	if c.trace != nil {
		*c.trace = append(*c.trace, "commit")
	}
	if c.commit != nil {
		return c.commit(ctx, snapshot)
	}
	return c.candidate, c.err
}

type echoAppDiscoveryAppender struct{}

func (echoAppDiscoveryAppender) AppendBatch(
	_ context.Context,
	events []journal.Event,
) ([]journal.Event, error) {
	return cloneAppDiscoveryEvents(events), nil
}

type boundAppDiscoveryCommitter struct {
	appender state.EventBatchAppender
	suffix   string
	calls    int
}

func (c *boundAppDiscoveryCommitter) CommitRuntimeDiscovery(
	ctx context.Context,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) (state.RuntimeDiscoveryCommitCandidate, error) {
	c.calls++
	return state.CommitRuntimeDiscoverySnapshot(
		ctx,
		c.appender,
		snapshot,
		appDiscoveryCommitInput(snapshot, c.suffix),
	)
}

func presentAppDiscoveryFactory(
	name string,
	probe loomruntime.RuntimeProbe,
	trace *[]string,
) discoveryscan.ProbeFactory {
	return &appDiscoveryFactory{
		name:  name,
		trace: trace,
		build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
			return probe, true, nil
		},
	}
}

func appDiscoveryInstance(id, displayName string) loomruntime.RuntimeInstance {
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                   id,
		DeviceID:             "device.local",
		AdapterType:          "adapter.local",
		DisplayName:          displayName,
		ExecutableVersion:    "1.0.0",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"go_test", "apply_patch"},
		Capacity:             1,
	})
	if err != nil {
		panic(err)
	}
	return instance
}

func appDiscoverySnapshot(
	t *testing.T,
	prefix string,
	count int,
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	observations := make([]loomruntime.RuntimeObservation, count)
	for index := range observations {
		suffix := string(rune('a' + index))
		observations[index] = loomruntime.RuntimeObservation{
			Instance: appDiscoveryInstance("runtime."+prefix+"."+suffix, prefix+" "+suffix),
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

func mintAppDiscoveryCandidate(
	t *testing.T,
	ctx context.Context,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	suffix string,
) state.RuntimeDiscoveryCommitCandidate {
	t.Helper()
	candidate, err := state.CommitRuntimeDiscoverySnapshot(
		ctx,
		echoAppDiscoveryAppender{},
		snapshot,
		appDiscoveryCommitInput(snapshot, suffix),
	)
	if err != nil {
		t.Fatalf("CommitRuntimeDiscoverySnapshot() error = %v", err)
	}
	return candidate
}

func appDiscoveryCommitInput(
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	suffix string,
) state.RuntimeDiscoveryCommitInput {
	observations := snapshot.Observations()
	events := make([]state.RuntimeDiscoveryEventInput, len(observations))
	for index, observation := range observations {
		events[index] = state.RuntimeDiscoveryEventInput{
			RuntimeInstanceID: observation.Instance.ID,
			EventID:           "event." + suffix + "." + observation.Instance.ID,
			IdempotencyKey:    "key." + suffix + "." + observation.Instance.ID,
			Seq:               1,
		}
	}
	return state.RuntimeDiscoveryCommitInput{
		DiscoveryID: "discovery." + suffix,
		EmittedAt:   time.Date(2026, 7, 25, 18, 0, 0, 0, time.UTC),
		Events:      events,
	}
}

func cloneAppDiscoveryEvents(events []journal.Event) []journal.Event {
	cloned := make([]journal.Event, len(events))
	for index, event := range events {
		cloned[index] = event
		cloned[index].PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	}
	return cloned
}

func assertZeroRuntimeDiscoveryRun(
	t *testing.T,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	candidate state.RuntimeDiscoveryCommitCandidate,
) {
	t.Helper()
	if snapshot.Digest() != "" || len(snapshot.Observations()) != 0 {
		t.Fatalf("snapshot = %#v, want zero", snapshot)
	}
	assertZeroAppDiscoveryCandidate(t, candidate)
}

func assertZeroAppDiscoveryCandidate(
	t *testing.T,
	candidate state.RuntimeDiscoveryCommitCandidate,
) {
	t.Helper()
	if candidate.Committed() ||
		candidate.SourceDiscoveryDigest() != "" ||
		candidate.EventCount() != 0 ||
		len(candidate.Events()) != 0 ||
		candidate.CommitDigest() != "" {
		t.Fatalf("candidate = %#v, want zero", candidate)
	}
}

func assertValidEmptyAppDiscoverySnapshot(
	t *testing.T,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) {
	t.Helper()
	if snapshot.Digest() == "" || len(snapshot.Observations()) != 0 {
		t.Fatalf("snapshot = %#v, want valid empty snapshot", snapshot)
	}
}

func isAppDiscoveryDigest(value string) bool {
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

func canceledAppDiscoveryContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredAppDiscoveryContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	cancel()
	return ctx
}

func openAppDiscoveryJournal(t *testing.T) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	path := filepath.Join(t.TempDir(), "journal.db")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", path, values.Encode()))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("journal.Migrate() error = %v", err)
	}
	return db
}

func appDiscoveryEventCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatalf("count events error = %v", err)
	}
	return count
}
