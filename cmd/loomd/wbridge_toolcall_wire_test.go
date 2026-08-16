package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/toolproposal"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

type wbridgeWireFixture struct {
	store   *journal.Store
	hook    *bridgeExecutionHook
	journey string
}

func newWBridgeWireFixture(
	t *testing.T,
	mode permissions.Mode,
	rules ...permissions.Rule,
) *wbridgeWireFixture {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/exec.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	now := func() time.Time { return time.Now().UTC() }
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	permAuth, err := permissions.NewAuthority(store, now)
	if err != nil {
		t.Fatal(err)
	}
	journey := "55555555-5555-4555-8555-555555555555"
	if _, err := permAuth.DefineProfile(context.Background(), permissions.ProfileInput{
		ProfileID: "wbridge-profile", Mode: mode, Rules: rules, OwnedPaths: []string{"**"},
	}, "op-profile", journey); err != nil {
		t.Fatal(err)
	}
	if _, err := permAuth.BindJob(context.Background(), "job-bridge", "wbridge-profile", "op-bind", journey); err != nil {
		t.Fatal(err)
	}
	workerService, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workerService.Claim(context.Background(), work.ClaimInput{
		JourneyID: journey, OperationID: "op-claim", WorkerID: "bridge-worker",
		JobID: "job-bridge", Lane: schedule.LaneDevelopment,
		CandidateBranch: "codex/candidate-bridge", CandidateWorktree: root,
	}); err != nil {
		t.Fatal(err)
	}
	decisionRecorder, err := newExecutionDecisionRecorder(store, now)
	if err != nil {
		t.Fatal(err)
	}
	approvalPort, err := newPermissionApprovalPort(store, now)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := execution.NewAdapter(
		store, artifactStore, execution.NewSandboxExecutor(),
		&productWorktreeResolver{store: store}, approvalPort, decisionRecorder, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := newBridgeExecutionHook(adapter, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &wbridgeWireFixture{store: store, hook: hook, journey: journey}
}

type capturingToolProposalStore struct {
	record toolproposal.Record
	puts   int
}

func (store *capturingToolProposalStore) PutToolProposal(
	_ context.Context,
	record toolproposal.Record,
) error {
	store.puts++
	store.record = toolproposal.Record{
		Binding: record.Binding, Content: append([]byte(nil), record.Content...),
	}
	return nil
}

func (*capturingToolProposalStore) ReadToolProposal(
	context.Context,
	toolproposal.Binding,
) (toolproposal.Record, error) {
	return toolproposal.Record{}, errors.New("not implemented")
}

func (*capturingToolProposalStore) LookupToolProposal(
	context.Context,
	toolproposal.ApprovalLookup,
) (toolproposal.Record, error) {
	return toolproposal.Record{}, errors.New("not implemented")
}

func (*capturingToolProposalStore) DeleteToolProposal(
	context.Context,
	toolproposal.Binding,
) error {
	return errors.New("not implemented")
}

func (fixture *wbridgeWireFixture) execute(
	t *testing.T,
	tool permissions.ToolKind,
	command, path string,
) (piadapter.ToolCallResult, error) {
	t.Helper()
	return fixture.hook.ExecuteToolCall(
		context.Background(),
		piadapter.ToolCallEnvelope{
			JobID: "job-bridge",
			Call:  permissions.ProposedCall{Tool: tool, Command: command, Path: path},
		},
		piadapter.ToolCallBinding{
			WorkItemID: "S5-W2", RunID: "run-bridge-1",
			ClaimGeneration: 1, JourneyID: fixture.journey,
		},
	)
}

func TestWBridgeHookAllowExecutes(t *testing.T) {
	fixture := newWBridgeWireFixture(t, permissions.ModeAuto)
	result, err := fixture.execute(t, permissions.ToolBash, "printf wbridge-ok", "")
	if err != nil {
		t.Fatalf("allow hook err: %v", err)
	}
	if result.Verdict != permissions.VerdictAllow || result.ExecutionID == "" {
		t.Fatalf("result = %+v", result)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]int{}
	for _, event := range events {
		types[event.Type]++
		if bytes.Contains(event.PayloadJSON, []byte("printf wbridge-ok")) {
			t.Fatalf("journal event %s persisted bridge tool arguments", event.Type)
		}
	}
	if types["ToolExecutionProposed"] < 1 || types["ToolExecutionCompleted"] < 1 {
		t.Fatalf("journal missing execution facts: %+v", types)
	}
}

func TestCOMP2DWBridgeResolvedToolAdvancesScopedTurn(t *testing.T) {
	fixture := newWBridgeWireFixture(t, permissions.ModeAuto)
	controller := &productAttemptTurnControllerFixture{}
	ctx, err := bindProductAttemptTurnScope(context.Background(), controller)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.hook.ExecuteToolCall(
		ctx,
		piadapter.ToolCallEnvelope{
			JobID: "job-bridge",
			Call: permissions.ProposedCall{
				Tool: permissions.ToolBash, Command: "printf scoped-turn",
			},
		},
		piadapter.ToolCallBinding{
			WorkItemID: "S5-W2", RunID: "run-bridge-scope",
			ClaimGeneration: 1, JourneyID: fixture.journey,
		},
	)
	if err != nil || result.Verdict != permissions.VerdictAllow {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(controller.sequences) != 1 || controller.sequences[0] != 1 {
		t.Fatalf("turn sequences=%v", controller.sequences)
	}
}

func TestWBridgeHookAskZeroExecution(t *testing.T) {
	fixture := newWBridgeWireFixture(t, permissions.ModeDefault)
	result, err := fixture.execute(t, permissions.ToolBash, "echo wbridge-ask", "")
	if err != nil {
		t.Fatalf("ask hook err: %v", err)
	}
	if result.Verdict != permissions.VerdictAsk {
		t.Fatalf("result verdict = %q, want ask", result.Verdict)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if strings.HasPrefix(event.Type, "ToolExecution") &&
			event.Type != "ToolExecutionProposed" {
			t.Fatalf("ask produced execution fact %s", event.Type)
		}
	}
}

func TestP2DWBridgeAskPersistsExactEncryptedProposalBoundary(t *testing.T) {
	fixture := newWBridgeWireFixture(t, permissions.ModeDefault)
	proposals := &capturingToolProposalStore{}
	fixture.hook.proposals = proposals
	call := permissions.ProposedCall{
		Tool: permissions.ToolBash, Command: "echo encrypted-approval-detail",
	}
	binding := piadapter.ToolCallBinding{
		ConversationID: "conversation-bridge", WorkItemID: "job-bridge",
		RunID: "run-bridge-proposal", ClaimGeneration: 2,
		RuntimeInstanceID: "runtime-bridge", AgentInstanceID: "agent-bridge",
		ExecutionBindingDigest: productAttemptLoopBytesDigest("binding", []byte("bridge")),
		CapsuleDigest:          productAttemptLoopBytesDigest("capsule", []byte("bridge")),
		ClaimID:                "claim-bridge", IncidentID: fixture.journey, JourneyID: fixture.journey,
	}
	result, err := fixture.hook.ExecuteToolCall(
		context.Background(),
		piadapter.ToolCallEnvelope{JobID: "job-bridge", Call: call},
		binding,
	)
	if err != nil || result.Verdict != permissions.VerdictAsk ||
		result.ApprovalID == "" || proposals.puts != 1 {
		t.Fatalf("ask result = %#v, puts=%d, %v", result, proposals.puts, err)
	}
	defer proposals.record.Close()
	if proposals.record.Binding.ApprovalID != result.ApprovalID ||
		proposals.record.Binding.ApprovalDigest != result.ApprovalDigest ||
		proposals.record.Binding.CallDigest != permissions.ProposedCallDigest(call) ||
		proposals.record.Binding.ConversationID != binding.ConversationID ||
		proposals.record.Binding.ExecutionBindingDigest != binding.ExecutionBindingDigest ||
		!bytes.Contains(proposals.record.Content, []byte("encrypted-approval-detail")) {
		t.Fatalf("captured proposal = %#v", proposals.record)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte("encrypted-approval-detail")) {
			t.Fatalf("journal event %s persisted proposal detail", event.Type)
		}
	}
}

func TestWBridgeHookDenyZeroExecution(t *testing.T) {
	fixture := newWBridgeWireFixture(t, permissions.ModeAuto, permissions.Rule{
		RuleID: "deny-rm", Scope: permissions.ScopeJob, ScopeID: "job-bridge",
		Action: permissions.ActionDeny, Tool: permissions.ToolBash, Pattern: "rm -rf /",
	})
	result, err := fixture.execute(t, permissions.ToolBash, "rm -rf /", "")
	if err != nil {
		t.Fatalf("deny hook err: %v", err)
	}
	if result.Verdict != permissions.VerdictDeny {
		t.Fatalf("result = %+v", result)
	}
	events, err := fixture.store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if strings.HasPrefix(event.Type, "ToolExecution") &&
			event.Type != "ToolExecutionProposed" &&
			event.Type != "ToolExecutionDenied" {
			t.Fatalf("deny produced execution fact %s", event.Type)
		}
	}
}

func TestProductAttemptReadAndGrepUsePathScopedParallelConflicts(t *testing.T) {
	invocation := productAttemptLoopInvocation{Binding: work.AttemptLoopBinding{
		AttemptID: "attempt-harness-tools",
		PayloadAuthority: attemptpayload.Authority{Scope: attemptpayload.Scope{
			WorkItemID: "work-harness-tools",
		}},
	}, TurnID: "turn-1", StepID: "step-1"}
	read := newProductAttemptToolDispatchGate(
		nil, nil, invocation,
		permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
		strings.Repeat("1", 64), "operation-read", 1,
	)
	grepSamePath := newProductAttemptToolDispatchGate(
		nil, nil, invocation,
		permissions.ProposedCall{Tool: permissions.ToolGrep, Path: "src/main.go", Pattern: "needle"},
		strings.Repeat("2", 64), "operation-grep", 2,
	)
	readOtherPath := newProductAttemptToolDispatchGate(
		nil, nil, invocation,
		permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/other.go"},
		strings.Repeat("3", 64), "operation-other", 3,
	)
	edit := newProductAttemptToolDispatchGate(
		nil, nil, invocation,
		permissions.ProposedCall{Tool: permissions.ToolEdit, Path: "src/main.go"},
		strings.Repeat("4", 64), "operation-edit", 4,
	)
	if read.callInput.ExecutionMode != work.ToolExecutionParallel ||
		grepSamePath.callInput.ExecutionMode != work.ToolExecutionParallel ||
		read.callInput.ConflictScopeDigest != grepSamePath.callInput.ConflictScopeDigest ||
		read.callInput.ConflictScopeDigest == readOtherPath.callInput.ConflictScopeDigest {
		t.Fatalf("read/grep conflicts = %#v %#v %#v", read.callInput, grepSamePath.callInput, readOtherPath.callInput)
	}
	if edit.callInput.ExecutionMode != work.ToolExecutionExclusive ||
		edit.callInput.ConflictScopeDigest == read.callInput.ConflictScopeDigest ||
		strings.Contains(read.callInput.ConflictScopeDigest, "src/main.go") {
		t.Fatalf("edit/read conflict policy = %#v / %#v", edit.callInput, read.callInput)
	}
}
