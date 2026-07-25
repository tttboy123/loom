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
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

func TestPreparedRuntimeStatusCommitterRejectsInvalidBindings(t *testing.T) {
	var typedNilAppender *recordingStatusEventBatchAppender
	var typedNilProvider *recordingRuntimeStatusCommitInputProvider
	tests := []struct {
		name     string
		appender state.EventBatchAppender
		provider RuntimeStatusCommitInputProvider
	}{
		{name: "nil_appender", provider: &recordingRuntimeStatusCommitInputProvider{}},
		{
			name:     "typed_nil_appender",
			appender: typedNilAppender,
			provider: &recordingRuntimeStatusCommitInputProvider{},
		},
		{name: "nil_provider", appender: &recordingStatusEventBatchAppender{}},
		{
			name:     "typed_nil_provider",
			appender: &recordingStatusEventBatchAppender{},
			provider: typedNilProvider,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := NewPreparedRuntimeStatusCommitter(
				test.appender, test.provider,
			)
			if !errors.Is(err, ErrInvalidPreparedRuntimeStatusCommitter) {
				t.Fatalf("error = %v, want invalid committer", err)
			}
			if adapter != nil {
				t.Fatalf("adapter = %#v, want nil", adapter)
			}
		})
	}
}

func TestPreparedRuntimeStatusCommitterPrevalidatesRequests(t *testing.T) {
	candidate := appStatusReconciliationCandidate(t, "prevalidate", 1)
	validAppender := &recordingStatusEventBatchAppender{}
	validProvider := &recordingRuntimeStatusCommitInputProvider{}
	valid, err := NewPreparedRuntimeStatusCommitter(validAppender, validProvider)
	if err != nil {
		t.Fatal(err)
	}
	var nilReceiver *PreparedRuntimeStatusCommitter
	var zeroValue PreparedRuntimeStatusCommitter
	var typedNilAppender *recordingStatusEventBatchAppender
	var typedNilProvider *recordingRuntimeStatusCommitInputProvider

	tests := []struct {
		name    string
		adapter *PreparedRuntimeStatusCommitter
		ctx     context.Context
		want    error
	}{
		{
			name: "nil_receiver", adapter: nilReceiver,
			ctx:  context.Background(),
			want: ErrInvalidPreparedRuntimeStatusCommitter,
		},
		{
			name: "zero_value", adapter: &zeroValue,
			ctx:  context.Background(),
			want: ErrInvalidPreparedRuntimeStatusCommitter,
		},
		{
			name: "stored_typed_nil_appender",
			adapter: &PreparedRuntimeStatusCommitter{
				appender: typedNilAppender,
				provider: validProvider,
			},
			ctx:  context.Background(),
			want: ErrInvalidPreparedRuntimeStatusCommitter,
		},
		{
			name: "stored_typed_nil_provider",
			adapter: &PreparedRuntimeStatusCommitter{
				appender: validAppender,
				provider: typedNilProvider,
			},
			ctx:  context.Background(),
			want: ErrInvalidPreparedRuntimeStatusCommitter,
		},
		{name: "nil_context", adapter: valid, want: ErrInvalidPreparedRuntimeStatusCommitter},
		{
			name: "canceled_context", adapter: valid,
			ctx:  canceledAppDiscoveryContext(),
			want: context.Canceled,
		},
		{
			name: "expired_context", adapter: valid,
			ctx:  expiredAppDiscoveryContext(),
			want: context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commit, err := test.adapter.CommitRuntimeStatus(test.ctx, candidate)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroAppStatusCommit(t, commit)
		})
	}
	if validProvider.calls != 0 || validAppender.calls != 0 {
		t.Fatalf("dependency calls = provider:%d appender:%d, want zero",
			validProvider.calls, validAppender.calls)
	}
}

func TestPreparedRuntimeStatusCommitterDelegatesExactInputOnce(t *testing.T) {
	ctx := context.Background()
	candidate := appStatusReconciliationCandidate(t, "delegate", 2)
	input := appStatusCommitInput(candidate, "delegate")
	trace := make([]string, 0, 2)
	provider := &recordingRuntimeStatusCommitInputProvider{
		trace: &trace,
		prepare: func(
			_ context.Context,
			got loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitInput, error) {
			assertSameAppStatusReconciliation(t, got, candidate)
			transitions := got.Transitions()
			transitions[0].RuntimeInstanceID = "mutated"
			if got.Transitions()[0].RuntimeInstanceID == "mutated" {
				t.Fatal("provider received mutation-leaking Candidate")
			}
			return input, nil
		},
	}
	appender := &recordingStatusEventBatchAppender{trace: &trace}
	adapter, err := NewPreparedRuntimeStatusCommitter(appender, provider)
	if err != nil {
		t.Fatal(err)
	}
	var _ RuntimeStatusCommitter = adapter

	commit, err := adapter.CommitRuntimeStatus(ctx, candidate)
	if err != nil {
		t.Fatalf("CommitRuntimeStatus() error = %v", err)
	}
	if provider.calls != 1 || appender.calls != 1 {
		t.Fatalf("calls = provider:%d appender:%d, want 1/1",
			provider.calls, appender.calls)
	}
	if !reflect.DeepEqual(trace, []string{"provider", "appender"}) {
		t.Fatalf("trace = %#v, want provider then appender", trace)
	}
	if !commit.Committed() ||
		commit.SourceReconciliationDigest() != candidate.CandidateDigest() ||
		commit.EventCount() != candidate.TransitionCount() ||
		len(commit.Events()) != candidate.TransitionCount() {
		t.Fatalf("commit = %#v, want exact accepted S2-W23 Candidate", commit)
	}
	events := commit.Events()
	events[0].PayloadJSON[0] ^= 0xff
	if reflect.DeepEqual(events, commit.Events()) {
		t.Fatal("commit Event accessor leaked mutation")
	}
	input.Events[0].EventID = "mutated-after-return"
	if commit.Events()[0].ID == "mutated-after-return" {
		t.Fatal("provider input mutation changed committed Candidate")
	}
}

func TestPreparedRuntimeStatusCommitterProviderFailuresPreventAppend(t *testing.T) {
	candidate := appStatusReconciliationCandidate(t, "provider-errors", 1)
	sentinel := errors.New("provider failed")
	tests := []struct {
		name    string
		ctx     context.Context
		prepare func(
			context.Context,
			loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitInput, error)
		want        error
		wantWrapped error
	}{
		{
			name: "sentinel", ctx: context.Background(),
			prepare: func(
				context.Context,
				loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitInput, error) {
				return state.RuntimeStatusCommitInput{}, sentinel
			},
			want: ErrRuntimeStatusCommitInputFailed, wantWrapped: sentinel,
		},
		{
			name: "wrapped_canceled", ctx: context.Background(),
			prepare: func(
				context.Context,
				loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitInput, error) {
				return state.RuntimeStatusCommitInput{},
					errors.Join(errors.New("provider"), context.Canceled)
			},
			want: context.Canceled,
		},
		{
			name: "wrapped_deadline", ctx: context.Background(),
			prepare: func(
				context.Context,
				loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitInput, error) {
				return state.RuntimeStatusCommitInput{},
					errors.Join(errors.New("provider"), context.DeadlineExceeded)
			},
			want: context.DeadlineExceeded,
		},
		{
			name: "cancel_after_provider", ctx: context.Background(),
			prepare: func(
				ctx context.Context,
				got loomruntime.RuntimeStatusReconciliationCandidate,
			) (state.RuntimeStatusCommitInput, error) {
				cancelable := ctx.(*cancelOnDemandContext)
				cancelable.cancel()
				return appStatusCommitInput(got, "cancel-after-provider"), nil
			},
			want: context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appender := &recordingStatusEventBatchAppender{}
			provider := &recordingRuntimeStatusCommitInputProvider{
				prepare: test.prepare,
			}
			adapter, err := NewPreparedRuntimeStatusCommitter(appender, provider)
			if err != nil {
				t.Fatal(err)
			}
			ctx := test.ctx
			if test.name == "cancel_after_provider" {
				ctx = newCancelOnDemandContext()
			}
			commit, err := adapter.CommitRuntimeStatus(ctx, candidate)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if test.wantWrapped != nil && !errors.Is(err, test.wantWrapped) {
				t.Fatalf("error = %v, want wrapped %v", err, test.wantWrapped)
			}
			assertZeroAppStatusCommit(t, commit)
			if provider.calls != 1 || appender.calls != 0 {
				t.Fatalf("calls = provider:%d appender:%d, want 1/0",
					provider.calls, appender.calls)
			}
		})
	}
}

func TestPreparedRuntimeStatusCommitterPropagatesStateWriterFailures(t *testing.T) {
	ctx := context.Background()
	candidate := appStatusReconciliationCandidate(t, "state-errors", 1)
	appenderSentinel := errors.New("append failed")
	tests := []struct {
		name     string
		source   loomruntime.RuntimeStatusReconciliationCandidate
		input    state.RuntimeStatusCommitInput
		appender *recordingStatusEventBatchAppender
		want     error
	}{
		{
			name:     "zero_source",
			input:    state.RuntimeStatusCommitInput{},
			appender: &recordingStatusEventBatchAppender{},
			want:     state.ErrInvalidRuntimeStatusCommitSource,
		},
		{
			name: "invalid_input", source: candidate,
			input:    state.RuntimeStatusCommitInput{},
			appender: &recordingStatusEventBatchAppender{},
			want:     state.ErrInvalidRuntimeStatusCommitInput,
		},
		{
			name: "appender_error", source: candidate,
			input:    appStatusCommitInput(candidate, "append-error"),
			appender: &recordingStatusEventBatchAppender{err: appenderSentinel},
			want:     appenderSentinel,
		},
		{
			name: "result_mismatch", source: candidate,
			input: appStatusCommitInput(candidate, "result-mismatch"),
			appender: &recordingStatusEventBatchAppender{
				result: func([]journal.Event) []journal.Event { return nil },
			},
			want: state.ErrRuntimeStatusCommitResultMismatch,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &recordingRuntimeStatusCommitInputProvider{
				input: test.input,
			}
			adapter, err := NewPreparedRuntimeStatusCommitter(
				test.appender, provider,
			)
			if err != nil {
				t.Fatal(err)
			}
			commit, err := adapter.CommitRuntimeStatus(ctx, test.source)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertZeroAppStatusCommit(t, commit)
			if provider.calls != 1 {
				t.Fatalf("provider calls = %d, want 1", provider.calls)
			}
		})
	}
}

func TestPreparedRuntimeStatusCommitterS2W29RealJournalRetry(t *testing.T) {
	ctx := context.Background()
	baseline, current, previous := appStatusSources(
		t, "sqlite-prepared-status", 2,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	db := openAppDiscoveryJournal(t)
	store := journal.NewStore(db)
	if _, err := state.CommitRuntimeDiscoverySnapshot(
		ctx, store, previous, appDiscoveryCommitInput(previous, "status-prior"),
	); err != nil {
		t.Fatal(err)
	}
	provider := &recordingRuntimeStatusCommitInputProvider{
		prepare: func(
			_ context.Context,
			candidate loomruntime.RuntimeStatusReconciliationCandidate,
		) (state.RuntimeStatusCommitInput, error) {
			return appStatusCommitInput(candidate, "prepared-sqlite"), nil
		},
	}
	adapter, err := NewPreparedRuntimeStatusCommitter(store, provider)
	if err != nil {
		t.Fatal(err)
	}

	firstReconciliation, firstCommit, err :=
		RunObservedRuntimeStatusReconciliationOnce(
			ctx, baseline, current, adapter,
		)
	if err != nil {
		t.Fatal(err)
	}
	secondReconciliation, secondCommit, err :=
		RunObservedRuntimeStatusReconciliationOnce(
			ctx, baseline, current, adapter,
		)
	if err != nil {
		t.Fatal(err)
	}
	assertSameAppStatusReconciliation(
		t, firstReconciliation, secondReconciliation,
	)
	if firstCommit.CommitDigest() != secondCommit.CommitDigest() ||
		!reflect.DeepEqual(firstCommit.Events(), secondCommit.Events()) {
		t.Fatal("exact retry changed the committed fact set")
	}
	if provider.calls != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.calls)
	}
	if got := appDiscoveryEventCount(t, db); got != 4 {
		t.Fatalf("event row count = %d, want 4", got)
	}
}

func TestPreparedRuntimeStatusCommitterStaticBoundary(t *testing.T) {
	source, err := os.ReadFile("runtime_status_committer.go")
	if err != nil {
		t.Fatalf("read product: %v", err)
	}
	file, err := parser.ParseFile(
		token.NewFileSet(), "runtime_status_committer.go", source, 0,
	)
	if err != nil {
		t.Fatalf("parse product: %v", err)
	}
	allowed := map[string]bool{
		"context":                          true,
		"errors":                           true,
		"fmt":                              true,
		"loom-pi-rebuild/internal/runtime": true,
		"loom-pi-rebuild/internal/state":   true,
	}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if !allowed[path] {
			t.Fatalf("forbidden product import %q", path)
		}
	}
	forbiddenOwners := map[string]bool{
		"journal": true, "projection": true, "sql": true, "os": true,
		"exec": true, "net": true, "time": true, "uuid": true, "rand": true,
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt:
			t.Error("forbidden go statement")
		case *ast.CallExpr:
			if identifier, ok := typed.Fun.(*ast.Ident); ok {
				switch identifier.Name {
				case "make", "new", "append":
					t.Errorf("forbidden allocation call %q", identifier.Name)
				}
			}
		case *ast.SelectorExpr:
			if owner, ok := typed.X.(*ast.Ident); ok &&
				forbiddenOwners[owner.Name] {
				t.Errorf("forbidden selector %s.%s", owner.Name, typed.Sel.Name)
			}
		}
		return true
	})
	for _, marker := range []string{
		"RuntimeInstanceStatusChanged", "runtime_instance:",
		"IdempotencyKey", "EventID", "EmittedAt", "BuildRuntimeStatusBaselines",
		"ReconcileObservedRuntimeStatuses",
	} {
		if strings.Contains(string(source), marker) {
			t.Errorf("forbidden product marker %q", marker)
		}
	}
}

type recordingRuntimeStatusCommitInputProvider struct {
	trace   *[]string
	prepare func(
		context.Context,
		loomruntime.RuntimeStatusReconciliationCandidate,
	) (state.RuntimeStatusCommitInput, error)
	input state.RuntimeStatusCommitInput
	err   error
	calls int
}

func (p *recordingRuntimeStatusCommitInputProvider) PrepareRuntimeStatusCommit(
	ctx context.Context,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) (state.RuntimeStatusCommitInput, error) {
	p.calls++
	if p.trace != nil {
		*p.trace = append(*p.trace, "provider")
	}
	if p.prepare != nil {
		return p.prepare(ctx, candidate)
	}
	return p.input, p.err
}

type recordingStatusEventBatchAppender struct {
	trace  *[]string
	result func([]journal.Event) []journal.Event
	err    error
	calls  int
}

func (a *recordingStatusEventBatchAppender) AppendBatch(
	_ context.Context,
	events []journal.Event,
) ([]journal.Event, error) {
	a.calls++
	if a.trace != nil {
		*a.trace = append(*a.trace, "appender")
	}
	if a.err != nil {
		return nil, a.err
	}
	if a.result != nil {
		return a.result(events), nil
	}
	return cloneAppDiscoveryEvents(events), nil
}

type cancelOnDemandContext struct {
	context.Context
	cancel context.CancelFunc
}

func newCancelOnDemandContext() *cancelOnDemandContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &cancelOnDemandContext{Context: ctx, cancel: cancel}
}

func appStatusReconciliationCandidate(
	t *testing.T,
	prefix string,
	count int,
) loomruntime.RuntimeStatusReconciliationCandidate {
	t.Helper()
	baseline, snapshot, _ := appStatusSources(
		t, prefix, count,
		loomruntime.RuntimeOnline, loomruntime.RuntimeOffline,
	)
	candidate, err := loomruntime.ReconcileObservedRuntimeStatuses(
		context.Background(), baseline, snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}
