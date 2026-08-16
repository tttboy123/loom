package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"

	_ "modernc.org/sqlite"
)

const (
	execTestCorrelation = "11111111-1111-4111-8111-111111111111"
	execTestJobA        = "job-exec-a"
)

func openExecStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/exec.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return journal.NewStore(db)
}

type fixedResolver struct {
	root string
}

func (resolver fixedResolver) Resolve(ctx context.Context, jobID string) (string, error) {
	return resolver.root, nil
}

type recordingDecisionRecorder struct {
	mu        sync.Mutex
	decisions []permissions.Verdict
}

func (recorder *recordingDecisionRecorder) RecordDecision(
	ctx context.Context,
	jobID string,
	call permissions.ProposedCall,
	verdict permissions.Verdict,
	denial permissions.Denial,
	approvalID, operationID, journeyID string,
) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.decisions = append(recorder.decisions, verdict)
	return nil
}

type recordingExecutor struct {
	mu        sync.Mutex
	edits     []EditRequest
	runs      []RunRequest
	failRun   bool
	beforeRun func() error
}

type privateReadExecutor struct {
	content    []byte
	beforeRead func() error
	readErr    error
	reads      int
	greps      int
}

type remoteToolExecutorFixture struct {
	allowed       []permissions.ToolKind
	content       []byte
	validateErr   error
	executeErr    error
	validateCalls int
	executeCalls  int
	beforeExecute func() error
}

func (fixture *remoteToolExecutorFixture) AllowedRemoteTools() []permissions.ToolKind {
	if fixture.allowed != nil {
		return append([]permissions.ToolKind(nil), fixture.allowed...)
	}
	return []permissions.ToolKind{
		permissions.ToolWebSearch,
		permissions.ToolWebFetch,
		permissions.ToolMCPTool,
	}
}

func TestP2DRemoteToolMustBeExplicitlyPublishedBeforeValidation(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-web-capability", nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatch := &recordingDispatchGate{}
	remote := &remoteToolExecutorFixture{
		allowed: []permissions.ToolKind{permissions.ToolWebFetch},
		content: []byte("must not execute"),
	}
	adapter := mustAdapter(
		t, store, projection, &recordingExecutor{},
		func() time.Time { return time.Date(2026, 8, 14, 11, 55, 0, 0, time.UTC) },
	).WithRemoteToolExecutor(remote)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-web-not-published",
		JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom Agent governance",
		},
		DispatchGate: dispatch, ResultCommitGate: &resultCommitGateFixture{store: store},
	})
	if err != nil || result.Verdict != permissions.VerdictDeny ||
		result.ErrorCode != "remote_tool_unavailable" || dispatch.wasCommitted() ||
		remote.validateCalls != 0 || remote.executeCalls != 0 {
		t.Fatalf(
			"result=%#v dispatch=%t validate=%d execute=%d err=%v",
			result, dispatch.wasCommitted(), remote.validateCalls, remote.executeCalls, err,
		)
	}
}

func (fixture *remoteToolExecutorFixture) ValidateProposal(permissions.ProposedCall) error {
	fixture.validateCalls++
	return fixture.validateErr
}

func (fixture *remoteToolExecutorFixture) ExecuteProposalContent(
	context.Context,
	permissions.ProposedCall,
) ([]byte, error) {
	if fixture.beforeExecute != nil {
		if err := fixture.beforeExecute(); err != nil {
			return nil, err
		}
	}
	fixture.executeCalls++
	return bytes.Clone(fixture.content), fixture.executeErr
}

type resultCommitGateFixture struct {
	store       *journal.Store
	err         error
	calls       int
	input       ExecutionResultCommitInput
	contentCopy []byte
}

func (gate *resultCommitGateFixture) CommitExecutionResult(
	_ context.Context,
	input ExecutionResultCommitInput,
) error {
	gate.calls++
	gate.input = input
	gate.contentCopy = bytes.Clone(input.Content)
	events, err := gate.store.ReadAll(context.Background())
	if err != nil {
		return err
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		return err
	}
	record, found := snapshot.Record(input.ExecutionID)
	if !found || record.AllowedAt == "" || record.Status != "allowed" || record.CompletedAt != "" {
		return errors.New("result gate ran outside allowed-before-terminal boundary")
	}
	return gate.err
}

func (executor *privateReadExecutor) Grep(context.Context, GrepRequest) (GrepResult, error) {
	executor.greps++
	return GrepResult{
		Content:       executor.content,
		ContentDigest: digestBytes(executor.content),
	}, nil
}

func (*privateReadExecutor) Edit(context.Context, EditRequest) (EditResult, error) {
	return EditResult{}, nil
}

func (*privateReadExecutor) Run(context.Context, RunRequest) (RunResult, error) {
	return RunResult{}, nil
}

func (executor *privateReadExecutor) Read(context.Context, ReadRequest) (ReadResult, error) {
	if executor.beforeRead != nil {
		if err := executor.beforeRead(); err != nil {
			return ReadResult{}, err
		}
	}
	executor.reads++
	return ReadResult{
		Content:       executor.content,
		ContentDigest: digestBytes(executor.content),
	}, executor.readErr
}

func (executor *recordingExecutor) Edit(ctx context.Context, request EditRequest) (EditResult, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	executor.edits = append(executor.edits, request)
	return EditResult{RelativePath: request.RelativePath, ChangedFilesDigest: "sha256:edit"}, nil
}

func (executor *recordingExecutor) Run(ctx context.Context, request RunRequest) (RunResult, error) {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if executor.beforeRun != nil {
		if err := executor.beforeRun(); err != nil {
			return RunResult{}, err
		}
	}
	executor.runs = append(executor.runs, request)
	if executor.failRun {
		return RunResult{ExitCode: 1, OutputDigest: "sha256:out"}, fmt.Errorf("run failed")
	}
	return RunResult{ExitCode: 0, OutputDigest: "sha256:out", ChangedFilesDigest: "sha256:changed"}, nil
}

type recordingDispatchGate struct {
	mu        sync.Mutex
	committed bool
	input     ExecutionDispatchInput
	err       error
}

type recordingToolExecutionDiagnostics struct {
	mu      sync.Mutex
	records []ToolExecutionDiagnostic
}

func (recorder *recordingToolExecutionDiagnostics) RecordToolExecutionDiagnostic(
	_ context.Context,
	diagnostic ToolExecutionDiagnostic,
) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.records = append(recorder.records, diagnostic)
	return nil
}

func (recorder *recordingToolExecutionDiagnostics) snapshot() []ToolExecutionDiagnostic {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]ToolExecutionDiagnostic(nil), recorder.records...)
}

func (gate *recordingDispatchGate) CommitExecutionDispatch(
	_ context.Context,
	input ExecutionDispatchInput,
) error {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	gate.input = input
	if gate.err != nil {
		return gate.err
	}
	gate.committed = true
	return nil
}

func (gate *recordingDispatchGate) wasCommitted() bool {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	return gate.committed
}

func (executor *recordingExecutor) runCount() int {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return len(executor.runs)
}

func mustAdapter(
	t testing.TB,
	store *journal.Store,
	projection *permissions.Projection,
	executor Executor,
	now func() time.Time,
) *Adapter {
	t.Helper()
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewAdapter(
		store, evidenceStore, executor,
		fixedResolver{root: t.TempDir()},
		nil, &recordingDecisionRecorder{}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func mustProfileProjection(t testing.TB, store *journal.Store, profileID string, ownedPaths []string) (*permissions.Projection, error) {
	t.Helper()
	authority, err := permissions.NewAuthority(store, func() time.Time {
		return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.DefineProfile(context.Background(), permissions.ProfileInput{
		ProfileID: profileID, Mode: permissions.ModeDefault, OwnedPaths: ownedPaths,
	}, "op-"+profileID, execTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.BindJob(context.Background(), execTestJobA, profileID, "op-bind-"+profileID, execTestCorrelation); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return permissions.Replay(events)
}

func TestRedB1_AllowExecutesExactlyOnce(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-allow", []string{"src/**"})
	if err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-1", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	}
	first, err := adapter.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	if first.Verdict != permissions.VerdictAllow || executor.runCount() != 1 {
		t.Fatalf("first result=%+v runs=%d", first, executor.runCount())
	}
	second, err := adapter.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	if executor.runCount() != 1 || second.ExecutionID != first.ExecutionID {
		t.Fatalf("idempotent replay re-executed: runs=%d second=%+v", executor.runCount(), second)
	}
}

func TestP2DExecutionCommitsDispatchBeforeSideEffect(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-dispatch-before-side-effect", nil)
	if err != nil {
		t.Fatal(err)
	}
	gate := &recordingDispatchGate{}
	executor := &recordingExecutor{}
	executor.beforeRun = func() error {
		if !gate.wasCommitted() {
			return errors.New("executor ran before dispatch commit")
		}
		return nil
	}
	now := func() time.Time { return time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	call := permissions.ProposedCall{Tool: permissions.ToolBash, Command: "git status"}
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-dispatch-before-side-effect",
		JourneyID: execTestCorrelation, Call: call, DispatchGate: gate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictAllow || executor.runCount() != 1 ||
		gate.input.JobID != execTestJobA || gate.input.ExecutionID != result.ExecutionID ||
		gate.input.CallDigest != permissions.ProposedCallDigest(call) ||
		gate.input.OperationID != "op-dispatch-before-side-effect" ||
		gate.input.CorrelationID != execTestCorrelation {
		t.Fatalf("dispatch/result mismatch: input=%#v result=%#v runs=%d", gate.input, result, executor.runCount())
	}
}

func TestP2DReadCommitsDispatchBeforeAccess(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-read-dispatch", nil)
	if err != nil {
		t.Fatal(err)
	}
	gate := &recordingDispatchGate{}
	executor := &privateReadExecutor{content: []byte("private read result")}
	executor.beforeRead = func() error {
		if !gate.wasCommitted() {
			return errors.New("read ran before dispatch commit")
		}
		return nil
	}
	adapter := mustAdapter(
		t, store, projection, executor,
		func() time.Time { return time.Date(2026, 8, 14, 10, 20, 0, 0, time.UTC) },
	)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-read-dispatch",
		JourneyID: execTestCorrelation, DispatchGate: gate,
		Call: permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
	})
	if err != nil || result.Verdict != permissions.VerdictAllow || executor.reads != 1 ||
		!bytes.Equal(result.Content, []byte("private read result")) {
		t.Fatalf("read result=%#v reads=%d err=%v", result, executor.reads, err)
	}
	result.Close()
}

func TestP2DInvalidGrepPatternFailsBeforeDispatch(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-grep-preflight", nil)
	if err != nil {
		t.Fatal(err)
	}
	gate := &recordingDispatchGate{}
	executor := &privateReadExecutor{content: []byte("must not be read")}
	adapter := mustAdapter(
		t, store, projection, executor,
		func() time.Time { return time.Date(2026, 8, 14, 10, 22, 0, 0, time.UTC) },
	)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-grep-preflight",
		JourneyID: execTestCorrelation, DispatchGate: gate,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolGrep, Path: "src/main.go", Pattern: "[",
		},
	})
	if err != nil || result.Verdict != permissions.VerdictDeny ||
		result.ErrorCode != "invalid_grep_pattern" || gate.wasCommitted() || executor.greps != 0 {
		t.Fatalf(
			"invalid Grep result=%#v dispatch=%t calls=%d err=%v",
			result, gate.wasCommitted(), executor.greps, err,
		)
	}
}

func TestP2DRemoteResultIsCommittedBeforeExecutionTerminal(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-web-result-commit", nil)
	if err != nil {
		t.Fatal(err)
	}
	dispatch := &recordingDispatchGate{}
	remote := &remoteToolExecutorFixture{content: []byte("bounded private search result")}
	remote.beforeExecute = func() error {
		if !dispatch.wasCommitted() {
			return errors.New("remote call preceded dispatch")
		}
		return nil
	}
	resultGate := &resultCommitGateFixture{store: store}
	adapter := mustAdapter(
		t, store, projection, &recordingExecutor{},
		func() time.Time { return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC) },
	).WithRemoteToolExecutor(remote)
	call := permissions.ProposedCall{
		Tool: permissions.ToolWebSearch, Path: "Loom Agent governance",
	}
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-web-result-commit",
		JourneyID: execTestCorrelation, Call: call,
		DispatchGate: dispatch, ResultCommitGate: resultGate,
	})
	if err != nil || result.Verdict != permissions.VerdictAllow ||
		remote.validateCalls != 1 || remote.executeCalls != 1 || resultGate.calls != 1 ||
		resultGate.input.OutputDigest != digestBytes(resultGate.contentCopy) ||
		!bytes.Equal(resultGate.contentCopy, []byte("bounded private search result")) {
		t.Fatalf(
			"remote result=%#v remote=%#v gate=%#v err=%v",
			result, remote, resultGate, err,
		)
	}
	result.Close()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, resultGate.contentCopy) ||
		bytes.Contains(encoded, []byte(call.Path)) {
		t.Fatalf("remote content/arguments leaked to Journal: %s", encoded)
	}
}

func TestP2DRemoteResultCommitFailureNeverReplaysRemoteCall(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-web-result-failure", nil)
	if err != nil {
		t.Fatal(err)
	}
	remote := &remoteToolExecutorFixture{content: []byte("private result must be zeroized")}
	resultGate := &resultCommitGateFixture{
		store: store, err: errors.New("encrypted payload unavailable"),
	}
	adapter := mustAdapter(
		t, store, projection, &recordingExecutor{},
		func() time.Time { return time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC) },
	).WithRemoteToolExecutor(remote)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-web-result-failure",
		JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom encrypted payload",
		},
		DispatchGate: &recordingDispatchGate{}, ResultCommitGate: resultGate,
	}
	result, err := adapter.Execute(context.Background(), proposal)
	if err != nil || result.Verdict != permissions.VerdictDeny ||
		result.ErrorCode != "result_persistence_failed" || remote.executeCalls != 1 {
		t.Fatalf("first failure result=%#v calls=%d err=%v", result, remote.executeCalls, err)
	}
	if !bytes.Equal(resultGate.input.Content, make([]byte, len(resultGate.input.Content))) {
		t.Fatalf("failed remote result was not zeroized: %q", resultGate.input.Content)
	}
	second, err := adapter.Execute(context.Background(), proposal)
	if err != nil || second.Verdict != permissions.VerdictDeny || remote.executeCalls != 1 {
		t.Fatalf("replay result=%#v calls=%d err=%v", second, remote.executeCalls, err)
	}
}

func TestP2DReadContentIsDigestOnlyAndRevalidatedOnReplay(t *testing.T) {
	store := openExecStore(t)
	_, err := mustProfileProjection(t, store, "p-read-replay", nil)
	if err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	privateContent := []byte("private adapter read content\n")
	path := filepath.Join(worktree, "src", "main.go")
	if err := os.WriteFile(path, privateContent, 0o600); err != nil {
		t.Fatal(err)
	}
	evidenceStore := mustEvidenceStore(t)
	now := func() time.Time { return time.Date(2026, 8, 14, 10, 25, 0, 0, time.UTC) }
	adapter, err := NewAdapter(
		store, evidenceStore, NewSandboxExecutor(), fixedResolver{root: worktree},
		nil, &recordingDecisionRecorder{}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-read-replay", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
	}
	first, err := adapter.Execute(context.Background(), proposal)
	if err != nil || !bytes.Equal(first.Content, privateContent) || first.OutputDigest == "" {
		t.Fatalf("first read=%#v err=%v", first, err)
	}
	evidenceBody, err := evidenceStore.ReadArtifact(context.Background(), first.EvidenceID, 4096)
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	eventBytes, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for label, body := range map[string][]byte{"journal": eventBytes, "evidence": evidenceBody} {
		if bytes.Contains(body, privateContent) || bytes.Contains(body, []byte("src/main.go")) {
			t.Fatalf("%s leaked Read content or path: %s", label, body)
		}
	}
	first.Close()
	second, err := adapter.Execute(context.Background(), proposal)
	if err != nil || !bytes.Equal(second.Content, privateContent) {
		t.Fatalf("same-content replay=%#v err=%v", second, err)
	}
	second.Close()
	if err := os.WriteFile(path, []byte("changed private content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), proposal); !errors.Is(err, ErrExecutionContent) {
		t.Fatalf("changed-content replay error=%v", err)
	}
	restarted, err := NewAdapter(
		store, evidenceStore, NewSandboxExecutor(), fixedResolver{root: worktree},
		nil, &recordingDecisionRecorder{}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Execute(context.Background(), proposal); !errors.Is(err, ErrExecutionContent) {
		t.Fatalf("restart changed-content replay error=%v", err)
	}
}

func TestP2DReadContentIsZeroizedWhenResultCommitFails(t *testing.T) {
	store := openExecStore(t)
	_, err := mustProfileProjection(t, store, "p-read-zeroize", nil)
	if err != nil {
		t.Fatal(err)
	}
	executor := &privateReadExecutor{content: []byte("private failed read result")}
	evidenceStore := mustEvidenceStore(t)
	adapter, err := NewAdapter(
		store, evidenceStore, executor, fixedResolver{root: t.TempDir()},
		nil, &recordingDecisionRecorder{},
		func() time.Time { return time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := evidenceStore.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-read-zeroize", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
	})
	if err != nil || result.Verdict != permissions.VerdictDeny || result.ErrorCode != "evidence_failed" {
		t.Fatalf("failed commit result=%#v err=%v", result, err)
	}
	if !bytes.Equal(executor.content, make([]byte, len(executor.content))) {
		t.Fatalf("failed Read content was not zeroized: %q", executor.content)
	}
}

func TestP2DReadReplayErrorZeroizesReturnedContent(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-read-replay-zeroize", nil)
	if err != nil {
		t.Fatal(err)
	}
	executor := &privateReadExecutor{content: []byte("initial private read")}
	adapter := mustAdapter(
		t, store, projection, executor,
		func() time.Time { return time.Date(2026, 8, 14, 10, 35, 0, 0, time.UTC) },
	)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-read-replay-zeroize",
		JourneyID: execTestCorrelation,
		Call:      permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
	}
	first, err := adapter.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	executor.content = []byte("private errored replay result")
	executor.readErr = errors.New("read replay failed")
	if _, err := adapter.Execute(context.Background(), proposal); !errors.Is(err, ErrExecutionContent) {
		t.Fatalf("replay error=%v", err)
	}
	if !bytes.Equal(executor.content, make([]byte, len(executor.content))) {
		t.Fatalf("errored replay content was not zeroized: %q", executor.content)
	}
}

func TestP2DExecutionDispatchFailureHasZeroSideEffects(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-dispatch-failure", nil)
	if err != nil {
		t.Fatal(err)
	}
	gate := &recordingDispatchGate{err: errors.New("dispatch authority unavailable")}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 14, 9, 5, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	_, err = adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-dispatch-failure",
		JourneyID:    execTestCorrelation,
		Call:         permissions.ProposedCall{Tool: permissions.ToolBash, Command: "git status"},
		DispatchGate: gate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if executor.runCount() != 0 {
		t.Fatalf("dispatch failure executed side effect: runs=%d", executor.runCount())
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil || len(snapshot.Records) != 1 ||
		snapshot.Records[0].Status != "failed" ||
		snapshot.Records[0].ErrorCode != "dispatch_not_committed" {
		t.Fatalf("dispatch failure snapshot=%#v err=%v", snapshot, err)
	}
}

func TestP2DExecutionEmitsContentFreeToolStages(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-tool-diagnostics", nil)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := &recordingToolExecutionDiagnostics{}
	gate := &recordingDispatchGate{}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	privateCommand := "git status"
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-tool-diagnostics",
		JourneyID:    execTestCorrelation,
		Call:         permissions.ProposedCall{Tool: permissions.ToolBash, Command: privateCommand},
		DispatchGate: gate, Diagnostics: diagnostics,
	})
	if err != nil || result.Verdict != permissions.VerdictAllow {
		t.Fatalf("execute result=%#v err=%v", result, err)
	}
	records := diagnostics.snapshot()
	wantStages := []ToolExecutionStage{
		ToolStageAuthorization,
		ToolStageSandboxPrepare,
		ToolStageBindingValidation,
		ToolStageDispatch,
		ToolStageResultValidation,
		ToolStageResultCommit,
	}
	if len(records) != len(wantStages) {
		t.Fatalf("tool diagnostics=%#v", records)
	}
	for index, record := range records {
		if record.Stage != wantStages[index] || record.Result != ToolDiagnosticSucceeded ||
			record.JobID != execTestJobA || record.ExecutionID != result.ExecutionID ||
			record.CallDigest != permissions.ProposedCallDigest(permissions.ProposedCall{
				Tool: permissions.ToolBash, Command: privateCommand,
			}) || record.Tool != permissions.ToolBash ||
			record.OperationID != "op-tool-diagnostics" ||
			record.CorrelationID != execTestCorrelation || record.Elapsed < 0 {
			t.Fatalf("tool diagnostic[%d]=%#v", index, record)
		}
		encoded, marshalErr := json.Marshal(record)
		if marshalErr != nil || bytes.Contains(encoded, []byte(`"command"`)) ||
			bytes.Contains(encoded, []byte(`"path"`)) {
			t.Fatalf("tool diagnostic leaked command: %s, %v", encoded, marshalErr)
		}
	}
}

func TestP2DEditValidationPrecedesDispatchCommit(t *testing.T) {
	store := openExecStore(t)
	authority, err := permissions.NewAuthority(store, func() time.Time {
		return time.Date(2026, 8, 14, 10, 10, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.DefineProfile(context.Background(), permissions.ProfileInput{
		ProfileID: "p-edit-preflight", Mode: permissions.ModeDefault,
		OwnedPaths: []string{"src/**"},
		Rules: []permissions.Rule{{
			RuleID: "allow-outside", Scope: permissions.ScopeJob, ScopeID: execTestJobA,
			Action: permissions.ActionAllow, Tool: permissions.ToolEdit, Pattern: "outside/**",
		}},
	}, "op-edit-preflight-profile", execTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.BindJob(
		context.Background(), execTestJobA, "p-edit-preflight",
		"op-edit-preflight-bind", execTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	projection, err := permissions.Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := &recordingToolExecutionDiagnostics{}
	gate := &recordingDispatchGate{}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 14, 10, 10, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-edit-preflight",
		JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolEdit, Path: "outside/private.txt", Command: "private edit body",
		},
		DispatchGate: gate, Diagnostics: diagnostics,
	})
	if err != nil || result.Verdict != permissions.VerdictDeny ||
		gate.wasCommitted() || len(executor.edits) != 0 {
		t.Fatalf("edit preflight result=%#v committed=%t edits=%d err=%v",
			result, gate.wasCommitted(), len(executor.edits), err)
	}
	records := diagnostics.snapshot()
	last := records[len(records)-1]
	if last.Stage != ToolStageBindingValidation || last.Result != ToolDiagnosticFailed ||
		last.ErrorCode != "path_outside_owned_scope" {
		t.Fatalf("edit preflight diagnostic=%#v", last)
	}
}

func TestP2DExecutionDispatchFailureReportsExactSafeStage(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-tool-diagnostic-failure", nil)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := &recordingToolExecutionDiagnostics{}
	gate := &recordingDispatchGate{err: errors.New("private dispatch authority detail")}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 14, 10, 5, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	_, err = adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-tool-diagnostic-failure",
		JourneyID:    execTestCorrelation,
		Call:         permissions.ProposedCall{Tool: permissions.ToolBash, Command: "git status"},
		DispatchGate: gate, Diagnostics: diagnostics,
	})
	if err != nil || executor.runCount() != 0 {
		t.Fatalf("dispatch result err=%v runs=%d", err, executor.runCount())
	}
	records := diagnostics.snapshot()
	last := records[len(records)-1]
	if last.Stage != ToolStageDispatch || last.Result != ToolDiagnosticFailed ||
		last.ErrorCode != "dispatch_not_committed" || last.Retryable {
		t.Fatalf("dispatch diagnostic=%#v", last)
	}
	encoded, marshalErr := json.Marshal(records)
	if marshalErr != nil || bytes.Contains(encoded, []byte("private dispatch authority detail")) ||
		bytes.Contains(encoded, []byte(`"command"`)) || bytes.Contains(encoded, []byte(`"path"`)) {
		t.Fatalf("failure diagnostic leaked private content: %s, %v", encoded, marshalErr)
	}
}

func TestP2DExecutorFailureReturnsControlledCodeOnly(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-controlled-executor-failure", nil)
	if err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{failRun: true}
	now := func() time.Time { return time.Date(2026, 8, 14, 10, 7, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-controlled-executor-failure",
		JourneyID: execTestCorrelation,
		Call:      permissions.ProposedCall{Tool: permissions.ToolBash, Command: "git status"},
	})
	if err != nil || result.Verdict != permissions.VerdictDeny ||
		result.ErrorCode != "run_failed" || result.Note != "run_failed" {
		t.Fatalf("controlled failure=%#v err=%v", result, err)
	}
	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil || bytes.Contains(encoded, []byte("private")) ||
		bytes.Contains(encoded, []byte("run failed")) {
		t.Fatalf("executor failure leaked raw detail: %s, %v", encoded, marshalErr)
	}
}

func TestRedB2_AskNeverExecutesUntilApproved(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-ask-1", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "curl https://example.com"},
	}
	result, err := adapter.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictAsk || executor.runCount() != 0 {
		t.Fatalf("ask must not execute: result=%+v runs=%d", result, executor.runCount())
	}
}

func TestRedB2_ApprovedResumeExecutesOnce(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	executor := &recordingExecutor{}
	call := permissions.ProposedCall{Tool: permissions.ToolBash, Command: "curl https://example.com"}
	digest := callDigest(call)
	rulesAuthority, err := rules.NewAuthority(store, &execTestAuthorizer{now: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	adapter := mustAdapterWithApprovalAuthority(
		t, store, projection, executor, rulesAuthority, now,
	)
	dispatchGate := &recordingDispatchGate{}
	record, err := rulesAuthority.RequestPermissionApproval(context.Background(), rules.PermissionApprovalInput{
		JobID: execTestJobA, CallDigest: digest, Tool: "Bash",
		Command: call.Command, RequestedAt: now(), CorrelationID: execTestCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rulesAuthority.DecidePermissionApproval(context.Background(),
		record.ID(), record.Digest(), "approved", "user-1", execTestCorrelation); err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-ask-2", JourneyID: execTestCorrelation,
		Call: call, DispatchGate: dispatchGate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictAllow || executor.runCount() != 1 {
		t.Fatalf("approved ask must execute once: result=%+v runs=%d", result, executor.runCount())
	}
	if result.ApprovalID != record.ID() {
		t.Fatalf("approval id mismatch: %s vs %s", result.ApprovalID, record.ID())
	}
	if dispatchGate.input.ApprovalID != record.ID() ||
		dispatchGate.input.ApprovalDigest != record.Digest() ||
		dispatchGate.input.ExecutionID != result.ExecutionID {
		t.Fatalf("approved dispatch binding = %#v", dispatchGate.input)
	}
}

func TestP2DApprovalIsConsumedByOneExactExecution(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-one-shot", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC) }
	rulesAuthority, err := rules.NewAuthority(store, &execTestAuthorizer{now: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	call := permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "printf one-shot",
	}
	record, err := rulesAuthority.RequestPermissionApproval(
		context.Background(),
		rules.PermissionApprovalInput{
			JobID: execTestJobA, CallDigest: callDigest(call), Tool: "Bash",
			RequestedAt: now(), CorrelationID: execTestCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rulesAuthority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "approved", "user-1",
		execTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{}
	adapter := mustAdapterWithApprovalAuthority(
		t, store, projection, executor, rulesAuthority, now,
	)
	first, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-one-shot-first",
		JourneyID: execTestCorrelation, Call: call,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Verdict != permissions.VerdictAllow || executor.runCount() != 1 {
		t.Fatalf("first execution = %#v, runs=%d", first, executor.runCount())
	}
	replayed, err := rulesAuthority.ConsumePermissionApproval(
		context.Background(),
		rules.PermissionApprovalConsumptionInput{
			ApprovalID: record.ID(), ApprovalDigest: record.Digest(),
			JobID: execTestJobA, CallDigest: callDigest(call),
			ConsumerID: first.ExecutionID, OperationID: "op-one-shot-first",
			CorrelationID: execTestCorrelation,
		},
	)
	if err != nil || replayed.Status() != "consumed" ||
		replayed.ConsumedBy() != first.ExecutionID {
		t.Fatalf("idempotent consumption replay = %#v, %v", replayed, err)
	}
	if _, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-one-shot-second",
		JourneyID: execTestCorrelation, Call: call,
	}); !errors.Is(err, ErrApprovalConsumed) {
		t.Fatalf("second approval use = %v", err)
	}
	if executor.runCount() != 1 {
		t.Fatalf("consumed approval executed twice: runs=%d", executor.runCount())
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	consumed := 0
	for _, event := range events {
		if event.Type == "PermissionApprovalConsumed" {
			consumed++
		}
	}
	if consumed != 1 {
		t.Fatalf("approval consumption facts = %d", consumed)
	}
}

func TestP2DApprovalConsumptionRaceExecutesOnlyOneOperation(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-one-shot-race", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 13, 13, 30, 0, 0, time.UTC) }
	rulesAuthority, err := rules.NewAuthority(store, &execTestAuthorizer{now: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	call := permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "printf one-shot-race",
	}
	record, err := rulesAuthority.RequestPermissionApproval(
		context.Background(),
		rules.PermissionApprovalInput{
			JobID: execTestJobA, CallDigest: callDigest(call), Tool: "Bash",
			RequestedAt: now(), CorrelationID: execTestCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rulesAuthority.DecidePermissionApproval(
		context.Background(), record.ID(), record.Digest(), "approved", "user-1",
		execTestCorrelation,
	); err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{}
	adapters := []*Adapter{
		mustAdapterWithApprovalAuthority(t, store, projection, executor, rulesAuthority, now),
		mustAdapterWithApprovalAuthority(t, store, projection, executor, rulesAuthority, now),
	}
	start := make(chan struct{})
	errorsByOperation := make([]error, len(adapters))
	var wait sync.WaitGroup
	wait.Add(len(adapters))
	for index := range adapters {
		index := index
		go func() {
			defer wait.Done()
			<-start
			_, errorsByOperation[index] = adapters[index].Execute(
				context.Background(),
				Proposal{
					JobID:       execTestJobA,
					OperationID: fmt.Sprintf("op-one-shot-race-%d", index),
					JourneyID:   execTestCorrelation, Call: call,
				},
			)
		}()
	}
	close(start)
	wait.Wait()
	succeeded, consumed := 0, 0
	for _, err := range errorsByOperation {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrApprovalConsumed):
			consumed++
		default:
			t.Fatalf("approval race error = %v", err)
		}
	}
	if succeeded != 1 || consumed != 1 || executor.runCount() != 1 {
		t.Fatalf(
			"approval race succeeded=%d consumed=%d runs=%d errors=%v",
			succeeded, consumed, executor.runCount(), errorsByOperation,
		)
	}
}

func TestP2DExecutionJournalDoesNotPersistToolArguments(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-content-free", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 13, 14, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, &recordingExecutor{}, now)
	secretArgument := "printf journal-secret-marker"
	if _, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-content-free",
		JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: secretArgument,
		},
	}); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte(secretArgument)) {
			t.Fatalf("event %s persisted tool arguments", event.Type)
		}
	}
}

type contentFailureExecutor struct{ marker string }

func (executor contentFailureExecutor) Edit(context.Context, EditRequest) (EditResult, error) {
	return EditResult{}, errors.New(executor.marker)
}

func (executor contentFailureExecutor) Run(context.Context, RunRequest) (RunResult, error) {
	return RunResult{}, errors.New(executor.marker)
}

func TestP2DExecutionJournalDoesNotPersistExecutorErrors(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-content-free-error", nil)
	if err != nil {
		t.Fatal(err)
	}
	marker := "provider-or-tool-secret-in-error"
	adapter := mustAdapter(
		t,
		store,
		projection,
		contentFailureExecutor{marker: marker},
		func() time.Time { return time.Date(2026, 8, 13, 14, 30, 0, 0, time.UTC) },
	)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-content-free-error",
		JourneyID: execTestCorrelation,
		Call:      permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny {
		t.Fatalf("result = %#v", result)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte(marker)) {
			t.Fatalf("event %s persisted executor error content", event.Type)
		}
	}
}

func mustAdapterWithApprovalAuthority(
	t testing.TB,
	store *journal.Store,
	_ *permissions.Projection,
	executor Executor,
	approvals *rules.Authority,
	now func() time.Time,
) *Adapter {
	t.Helper()
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewAdapter(
		store, evidenceStore, executor, fixedResolver{root: t.TempDir()},
		approvals, &recordingDecisionRecorder{}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

type execTestAuthorizer struct {
	now func() time.Time
}

func (authorizer *execTestAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	digest := sha256.Sum256([]byte("exec-rule-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedRuleSetActivation(
		request, "approver:permission-owner", fmt.Sprintf("%x", digest[:]),
		now, now.Add(time.Hour),
	)
}

func (authorizer *execTestAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	digest := sha256.Sum256([]byte("exec-decision-auth"))
	now := authorizer.now()
	return rules.NewAuthorizedApprovalDecision(
		request, "approver:permission-owner", fmt.Sprintf("%x", digest[:]),
		now, now.Add(time.Hour),
	)
}

func TestRedB3_DenyAndStaleNeverExecute(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-deny", nil)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := permissions.NewAuthority(store, func() time.Time {
		return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.AddRule(context.Background(), permissions.Rule{
		RuleID: "r-deny", Scope: permissions.ScopeRoot, ScopeID: "global",
		Action: permissions.ActionDeny, Tool: permissions.ToolBash, Pattern: "rm *",
	}, "owner", "op-rule", execTestCorrelation); err != nil {
		t.Fatal(err)
	}
	events, _ := store.ReadAll(context.Background())
	projection, err = permissions.Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-deny", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "rm -rf vendor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny || executor.runCount() != 0 {
		t.Fatalf("deny must not execute: result=%+v runs=%d", result, executor.runCount())
	}
	if _, err := authority.RetireProfile(context.Background(), "p-deny", "op-retire", execTestCorrelation); err != nil {
		t.Fatal(err)
	}
	events, _ = store.ReadAll(context.Background())
	projection, err = permissions.Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-stale", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stale.Verdict != permissions.VerdictDeny || executor.runCount() != 0 {
		t.Fatalf("stale must not execute: result=%+v runs=%d", stale, executor.runCount())
	}
}

func TestRedB6_CrashRecoveryNeverReexecutes(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-recover", []string{"src/**"})
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	executor := &recordingExecutor{}
	adapter := mustAdapter(t, store, projection, executor, now)
	// First execution completes; then simulate a pending-allowed record by
	// executing a second unique operation and never writing a terminal.
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-crash-1", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	}
	before := executor.runCount()
	if _, err := adapter.Execute(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	after := executor.runCount()
	if after != before+1 {
		t.Fatalf("first execution runs=%d", after)
	}
	if err := adapter.ReplayPending(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRedB1_AllowedWithoutTerminalNeverReexecutes(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-recover", []string{"src/**"})
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	executor := &recordingExecutor{}
	adapter := mustAdapter(t, store, projection, executor, now)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-orphan-1", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	}
	// Seed the Journal with Proposed + Allowed but no terminal fact, as if a
	// crash happened between the allowed fact and the completed append.
	call := callDigest(proposal.Call)
	execID := executionID(proposal.JobID, call, proposal.OperationID)
	stream := executionStreamID(proposal.JobID, execID)
	events, _ := store.ReadAll(context.Background())
	heads := streamHeads(events)
	proposed, buildErr := adapter.buildEvent("proposed", stream, proposal.JobID, execID, call,
		proposal, 1, now(), proposal.JourneyID)
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	allowed, buildErr := adapter.buildAllowedEvent(stream, execID, now())
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	if _, casErr := adapter.appendCAS(context.Background(), events, heads, stream,
		[]journal.Event{proposed, allowed}); casErr != nil {
		t.Fatal(casErr)
	}
	before := executor.runCount()
	if _, err := adapter.Execute(context.Background(), proposal); !errors.Is(err, ErrExecutionInterrupted) {
		t.Fatalf("retry after allowed-without-terminal error = %v", err)
	}
	if executor.runCount() != before {
		t.Fatalf("retry executed again: runs=%d", executor.runCount())
	}
	if err := adapter.ReplayPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, err = store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	recovered, found := snapshot.Record(execID)
	if !found || recovered.Status != "recovery_required" ||
		recovered.RecoveryCode != "side_effect_unknown" ||
		recovered.RecoveryAction != "resolve_tool_recovery" ||
		recovered.RecoveryRequiredAt == "" {
		t.Fatalf("recovery record=%#v found=%t", recovered, found)
	}
	result, err := adapter.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny ||
		!result.RecoveryRequired || result.RecoveryCode != "side_effect_unknown" ||
		result.RecoveryAction != "resolve_tool_recovery" || result.Retryable ||
		result.Note != "recovery required" {
		t.Fatalf("terminalized result = %+v", result)
	}
	if executor.runCount() != before {
		t.Fatalf("terminalized retry executed again: runs=%d", executor.runCount())
	}
}

func TestP2DPendingApprovalSurvivesRecoveryReconciliation(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-recovery-pending-ask", nil)
	if err != nil {
		t.Fatal(err)
	}
	executor := &recordingExecutor{}
	now := func() time.Time { return time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, executor, now)
	proposal := Proposal{
		JobID: execTestJobA, OperationID: "op-pending-approval-recovery",
		JourneyID: execTestCorrelation,
		Call:      permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf approval-needed"},
	}
	first, err := adapter.Execute(context.Background(), proposal)
	if err != nil || first.Verdict != permissions.VerdictAsk {
		t.Fatalf("initial ask=%#v err=%v", first, err)
	}
	if err := adapter.ReplayPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReplaySnapshot(events)
	if err != nil {
		t.Fatal(err)
	}
	record, found := snapshot.Record(first.ExecutionID)
	if !found || record.Status != "proposed" || record.RecoveryRequiredAt != "" {
		t.Fatalf("pending approval was terminalized: %#v found=%t", record, found)
	}
	second, err := adapter.Execute(context.Background(), proposal)
	if err != nil || second.Verdict != permissions.VerdictAsk || executor.runCount() != 0 {
		t.Fatalf("replayed ask=%#v runs=%d err=%v", second, executor.runCount(), err)
	}
}

type limitExecutor struct{}

func (limitExecutor) Edit(context.Context, EditRequest) (EditResult, error) {
	return EditResult{}, ErrExecutionLimit
}

func (limitExecutor) Run(context.Context, RunRequest) (RunResult, error) {
	return RunResult{}, ErrExecutionLimit
}

func TestRedB1_ExecutionLimitMapsToLimitExceededFact(t *testing.T) {
	store := openExecStore(t)
	projection, err := mustProfileProjection(t, store, "p-limit", []string{"src/**"})
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	adapter := mustAdapter(t, store, projection, limitExecutor{}, now)
	result, err := adapter.Execute(context.Background(), Proposal{
		JobID: execTestJobA, OperationID: "op-limit-1", JourneyID: execTestCorrelation,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "ls -la"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny {
		t.Fatalf("result = %+v", result)
	}
	events, _ := store.ReadAll(context.Background())
	var code string
	for _, event := range events {
		if event.Type != EventToolFailed {
			continue
		}
		var payload failedPayload
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		code = payload.ErrorCode
	}
	if code != "limit_exceeded" {
		t.Fatalf("failed fact error_code = %q, want limit_exceeded", code)
	}
}

func mustEvidenceStore(t testing.TB) *evidence.Store {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}
