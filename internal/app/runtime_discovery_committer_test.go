package app

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

var _ RuntimeDiscoveryCommitter = (*PreparedRuntimeDiscoveryCommitter)(nil)

func TestPreparedRuntimeDiscoveryCommitterRejectsInvalidBindings(t *testing.T) {
	validAppender := &recordingPreparedDiscoveryAppender{}
	validProvider := &recordingDiscoveryCommitInputProvider{}
	var typedNilAppender *recordingPreparedDiscoveryAppender
	var typedNilProvider *recordingDiscoveryCommitInputProvider

	for _, tt := range []struct {
		name     string
		appender state.EventBatchAppender
		provider RuntimeDiscoveryCommitInputProvider
	}{
		{name: "nil appender", provider: validProvider},
		{name: "typed nil appender", appender: typedNilAppender, provider: validProvider},
		{name: "nil provider", appender: validAppender},
		{name: "typed nil provider", appender: validAppender, provider: typedNilProvider},
	} {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := NewPreparedRuntimeDiscoveryCommitter(tt.appender, tt.provider)
			if !errors.Is(err, ErrInvalidPreparedRuntimeDiscoveryCommitter) {
				t.Fatalf("error = %v, want ErrInvalidPreparedRuntimeDiscoveryCommitter", err)
			}
			if adapter != nil {
				t.Fatalf("adapter = %#v, want nil", adapter)
			}
		})
	}
	if validAppender.calls != 0 || validProvider.calls != 0 {
		t.Fatalf("constructor invoked bindings: appender=%d provider=%d", validAppender.calls, validProvider.calls)
	}
}

func TestPreparedRuntimeDiscoveryCommitterPrevalidatesRequests(t *testing.T) {
	snapshot := appDiscoverySnapshot(t, "prevalidate", 1)
	var nilAdapter *PreparedRuntimeDiscoveryCommitter
	candidate, err := nilAdapter.CommitRuntimeDiscovery(context.Background(), snapshot)
	if !errors.Is(err, ErrInvalidPreparedRuntimeDiscoveryCommitter) {
		t.Fatalf("nil receiver error = %v", err)
	}
	assertZeroAppDiscoveryCandidate(t, candidate)

	t.Run("zero value", func(t *testing.T) {
		var adapter PreparedRuntimeDiscoveryCommitter
		candidate, err := adapter.CommitRuntimeDiscovery(context.Background(), snapshot)
		if !errors.Is(err, ErrInvalidPreparedRuntimeDiscoveryCommitter) {
			t.Fatalf("zero-value error = %v, want ErrInvalidPreparedRuntimeDiscoveryCommitter", err)
		}
		assertZeroAppDiscoveryCandidate(t, candidate)
	})

	for _, tt := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "nil", ctx: nil, want: ErrInvalidPreparedRuntimeDiscoveryCommitter},
		{name: "canceled", ctx: canceledAppDiscoveryContext(), want: context.Canceled},
		{name: "deadline", ctx: expiredAppDiscoveryContext(), want: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			appender := &recordingPreparedDiscoveryAppender{}
			provider := &recordingDiscoveryCommitInputProvider{}
			adapter := mustPreparedDiscoveryCommitter(t, appender, provider)
			candidate, err := adapter.CommitRuntimeDiscovery(tt.ctx, snapshot)
			if tt.ctx == nil {
				if !errors.Is(err, tt.want) {
					t.Fatalf("error = %v, want errors.Is(%v)", err, tt.want)
				}
			} else if err != tt.want {
				t.Fatalf("error = %v, want exact %v", err, tt.want)
			}
			assertZeroAppDiscoveryCandidate(t, candidate)
			if appender.calls != 0 || provider.calls != 0 {
				t.Fatalf("calls = appender %d provider %d, want zero", appender.calls, provider.calls)
			}
		})
	}
}

func TestPreparedRuntimeDiscoveryCommitterDelegatesExactInputOnce(t *testing.T) {
	snapshot := appDiscoverySnapshot(t, "exact", 2)
	input := appDiscoveryCommitInput(snapshot, "prepared-exact")
	var trace []string
	provider := &recordingDiscoveryCommitInputProvider{
		trace: &trace,
		prepare: func(
			_ context.Context,
			got loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			if got.Digest() != snapshot.Digest() ||
				!reflect.DeepEqual(got.Observations(), snapshot.Observations()) {
				t.Fatalf("provider snapshot = %#v, want exact source", got)
			}
			mutated := got.Observations()
			mutated[0].ModelIDs[0] = "mutated"
			return input, nil
		},
	}
	appender := &recordingPreparedDiscoveryAppender{trace: &trace}
	adapter := mustPreparedDiscoveryCommitter(t, appender, provider)

	candidate, err := adapter.CommitRuntimeDiscovery(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("CommitRuntimeDiscovery() error = %v", err)
	}
	if got, want := trace, []string{"prepare", "append"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("trace = %#v, want %#v", got, want)
	}
	if provider.calls != 1 || appender.calls != 1 {
		t.Fatalf("calls = provider %d appender %d, want one each", provider.calls, appender.calls)
	}
	if !candidate.Committed() ||
		candidate.SourceDiscoveryDigest() != snapshot.Digest() ||
		candidate.EventCount() != 2 ||
		!isAppDiscoveryDigest(candidate.CommitDigest()) {
		t.Fatalf("candidate = %#v", candidate)
	}
	if snapshot.Observations()[0].ModelIDs[0] == "mutated" {
		t.Fatal("provider snapshot accessor mutation escaped")
	}
	events := candidate.Events()
	events[0].PayloadJSON[0] = '['
	if candidate.Events()[0].PayloadJSON[0] == '[' {
		t.Fatal("candidate accessor mutation escaped")
	}
}

func TestPreparedRuntimeDiscoveryCommitterProviderFailuresPreventAppend(t *testing.T) {
	snapshot := appDiscoverySnapshot(t, "provider-error", 1)
	sourceErr := errors.New("provider-source-error")

	t.Run("source error", func(t *testing.T) {
		appender := &recordingPreparedDiscoveryAppender{}
		provider := &recordingDiscoveryCommitInputProvider{
			secret: "secret-provider-value",
			err:    sourceErr,
		}
		adapter := mustPreparedDiscoveryCommitter(t, appender, provider)
		candidate, err := adapter.CommitRuntimeDiscovery(context.Background(), snapshot)
		if !errors.Is(err, ErrRuntimeDiscoveryCommitInputFailed) ||
			!errors.Is(err, sourceErr) {
			t.Fatalf("error = %v, want provider sentinel and source", err)
		}
		if strings.Contains(err.Error(), provider.secret) {
			t.Fatalf("error disclosed provider value: %v", err)
		}
		assertZeroAppDiscoveryCandidate(t, candidate)
		if provider.calls != 1 || appender.calls != 0 {
			t.Fatalf("calls = provider %d appender %d, want (1,0)", provider.calls, appender.calls)
		}
	})

	for _, contextErr := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(contextErr.Error(), func(t *testing.T) {
			appender := &recordingPreparedDiscoveryAppender{}
			provider := &recordingDiscoveryCommitInputProvider{err: fmtWrappedError(contextErr)}
			adapter := mustPreparedDiscoveryCommitter(t, appender, provider)
			candidate, err := adapter.CommitRuntimeDiscovery(context.Background(), snapshot)
			if err != contextErr {
				t.Fatalf("error = %v, want exact %v", err, contextErr)
			}
			assertZeroAppDiscoveryCandidate(t, candidate)
			if provider.calls != 1 || appender.calls != 0 {
				t.Fatalf("calls = provider %d appender %d, want (1,0)", provider.calls, appender.calls)
			}
		})
	}

	t.Run("context canceled after provider", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		appender := &recordingPreparedDiscoveryAppender{}
		provider := &recordingDiscoveryCommitInputProvider{
			prepare: func(
				context.Context,
				loomruntime.RuntimeDiscoverySnapshot,
			) (state.RuntimeDiscoveryCommitInput, error) {
				cancel()
				return appDiscoveryCommitInput(snapshot, "canceled"), nil
			},
		}
		adapter := mustPreparedDiscoveryCommitter(t, appender, provider)
		candidate, err := adapter.CommitRuntimeDiscovery(ctx, snapshot)
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact context.Canceled", err)
		}
		assertZeroAppDiscoveryCandidate(t, candidate)
		if provider.calls != 1 || appender.calls != 0 {
			t.Fatalf("calls = provider %d appender %d, want (1,0)", provider.calls, appender.calls)
		}
	})
}

func TestPreparedRuntimeDiscoveryCommitterPropagatesStateWriterFailures(t *testing.T) {
	nonempty := appDiscoverySnapshot(t, "state-failure", 1)
	empty, err := loomruntime.DiscoverRuntime(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	sourceErr := errors.New("appender-source-error")

	tests := []struct {
		name       string
		snapshot   loomruntime.RuntimeDiscoverySnapshot
		input      state.RuntimeDiscoveryCommitInput
		appender   *recordingPreparedDiscoveryAppender
		want       error
		appendCall int
	}{
		{
			name:     "empty source",
			snapshot: empty,
			input:    state.RuntimeDiscoveryCommitInput{},
			appender: &recordingPreparedDiscoveryAppender{},
			want:     state.ErrEmptyRuntimeDiscoveryCommit,
		},
		{
			name:     "invalid input",
			snapshot: nonempty,
			input:    state.RuntimeDiscoveryCommitInput{},
			appender: &recordingPreparedDiscoveryAppender{},
			want:     state.ErrInvalidRuntimeDiscoveryCommitInput,
		},
		{
			name:       "appender error",
			snapshot:   nonempty,
			input:      appDiscoveryCommitInput(nonempty, "appender-error"),
			appender:   &recordingPreparedDiscoveryAppender{err: sourceErr},
			want:       sourceErr,
			appendCall: 1,
		},
		{
			name:     "appender mismatch",
			snapshot: nonempty,
			input:    appDiscoveryCommitInput(nonempty, "appender-mismatch"),
			appender: &recordingPreparedDiscoveryAppender{
				append: func([]journal.Event) ([]journal.Event, error) { return nil, nil },
			},
			want:       state.ErrRuntimeDiscoveryCommitResultMismatch,
			appendCall: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &recordingDiscoveryCommitInputProvider{input: tt.input}
			adapter := mustPreparedDiscoveryCommitter(t, tt.appender, provider)
			candidate, err := adapter.CommitRuntimeDiscovery(context.Background(), tt.snapshot)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroAppDiscoveryCandidate(t, candidate)
			if provider.calls != 1 || tt.appender.calls != tt.appendCall {
				t.Fatalf("calls = provider %d appender %d, want (1,%d)",
					provider.calls, tt.appender.calls, tt.appendCall)
			}
		})
	}
}

func TestPreparedRuntimeDiscoveryCommitterS2W27RealJournalRetry(t *testing.T) {
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	provider := &recordingDiscoveryCommitInputProvider{
		prepare: func(
			_ context.Context,
			snapshot loomruntime.RuntimeDiscoverySnapshot,
		) (state.RuntimeDiscoveryCommitInput, error) {
			return appDiscoveryCommitInput(snapshot, "prepared-sqlite"), nil
		},
	}
	adapter := mustPreparedDiscoveryCommitter(t, store, provider)
	newFactory := func() discoveryscan.ProbeFactory {
		return presentAppDiscoveryFactory(
			"prepared",
			&appDiscoveryProbe{
				id: "probe.prepared",
				observations: []loomruntime.RuntimeObservation{{
					Instance: appDiscoveryInstance("runtime.prepared", "Prepared"),
					ModelIDs: []string{"model.prepared"},
				}},
			},
			nil,
		)
	}

	firstSnapshot, first, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		[]discoveryscan.ProbeFactory{newFactory()},
		adapter,
	)
	if err != nil {
		t.Fatalf("first run error = %v", err)
	}
	secondSnapshot, second, err := RunConfiguredRuntimeDiscoveryOnce(
		context.Background(),
		[]discoveryscan.ProbeFactory{newFactory()},
		adapter,
	)
	if err != nil {
		t.Fatalf("retry run error = %v", err)
	}
	if firstSnapshot.Digest() != secondSnapshot.Digest() ||
		first.CommitDigest() != second.CommitDigest() {
		t.Fatalf("retry changed accepted result")
	}
	if got := appDiscoveryEventCount(t, db); got != 1 {
		t.Fatalf("event count = %d, want 1", got)
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2 explicit runs", provider.calls)
	}
}

func TestPreparedRuntimeDiscoveryCommitterStaticBoundary(t *testing.T) {
	const filename = "runtime_discovery_committer.go"
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
		"fmt":                              true,
		"reflect":                          true,
		"loom-pi-rebuild/internal/runtime": true,
		"loom-pi-rebuild/internal/state":   true,
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
		"discoveryscan",
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
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("%s contains forbidden behavior marker %q", filename, forbidden)
		}
	}
	for _, required := range []string{
		"state.CommitRuntimeDiscoverySnapshot(",
		"ErrRuntimeDiscoveryCommitInputFailed",
	} {
		if !strings.Contains(string(source), required) {
			t.Fatalf("%s is missing required delegation %q", filename, required)
		}
	}
}

type recordingDiscoveryCommitInputProvider struct {
	trace   *[]string
	prepare func(context.Context, loomruntime.RuntimeDiscoverySnapshot) (state.RuntimeDiscoveryCommitInput, error)
	input   state.RuntimeDiscoveryCommitInput
	err     error
	secret  string
	calls   int
}

func (p *recordingDiscoveryCommitInputProvider) PrepareRuntimeDiscoveryCommit(
	ctx context.Context,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) (state.RuntimeDiscoveryCommitInput, error) {
	p.calls++
	if p.trace != nil {
		*p.trace = append(*p.trace, "prepare")
	}
	if p.prepare != nil {
		return p.prepare(ctx, snapshot)
	}
	return p.input, p.err
}

type recordingPreparedDiscoveryAppender struct {
	trace  *[]string
	append func([]journal.Event) ([]journal.Event, error)
	err    error
	calls  int
}

func (a *recordingPreparedDiscoveryAppender) AppendBatch(
	_ context.Context,
	events []journal.Event,
) ([]journal.Event, error) {
	a.calls++
	if a.trace != nil {
		*a.trace = append(*a.trace, "append")
	}
	if a.append != nil {
		return a.append(events)
	}
	if a.err != nil {
		return nil, a.err
	}
	return cloneAppDiscoveryEvents(events), nil
}

func mustPreparedDiscoveryCommitter(
	t *testing.T,
	appender state.EventBatchAppender,
	provider RuntimeDiscoveryCommitInputProvider,
) *PreparedRuntimeDiscoveryCommitter {
	t.Helper()
	adapter, err := NewPreparedRuntimeDiscoveryCommitter(appender, provider)
	if err != nil {
		t.Fatalf("NewPreparedRuntimeDiscoveryCommitter() error = %v", err)
	}
	return adapter
}

func fmtWrappedError(err error) error {
	return &wrappedPreparedDiscoveryError{source: err}
}

type wrappedPreparedDiscoveryError struct {
	source error
}

func (e *wrappedPreparedDiscoveryError) Error() string {
	return "wrapped context"
}

func (e *wrappedPreparedDiscoveryError) Unwrap() error {
	return e.source
}
