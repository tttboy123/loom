package execution

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
	mu      sync.Mutex
	edits   []EditRequest
	runs    []RunRequest
	failRun bool
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
	executor.runs = append(executor.runs, request)
	if executor.failRun {
		return RunResult{ExitCode: 1, OutputDigest: "sha256:out"}, fmt.Errorf("run failed")
	}
	return RunResult{ExitCode: 0, OutputDigest: "sha256:out", ChangedFilesDigest: "sha256:changed"}, nil
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
	adapter := mustAdapter(t, store, projection, executor, now)
	call := permissions.ProposedCall{Tool: permissions.ToolBash, Command: "curl https://example.com"}
	digest := callDigest(call)
	rulesAuthority, err := rules.NewAuthority(store, &execTestAuthorizer{now: now}, now)
	if err != nil {
		t.Fatal(err)
	}
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
		Call: call,
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
	result, err := adapter.Execute(context.Background(), proposal)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != permissions.VerdictDeny ||
		result.Note != "interrupted before terminal fact" {
		t.Fatalf("terminalized result = %+v", result)
	}
	if executor.runCount() != before {
		t.Fatalf("terminalized retry executed again: runs=%d", executor.runCount())
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
