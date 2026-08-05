package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/schedule"
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
	adapter, err := execution.NewAdapter(
		store, artifactStore, execution.NewSandboxExecutor(),
		&productWorktreeResolver{store: store}, nil, decisionRecorder, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := newBridgeExecutionHook(adapter)
	if err != nil {
		t.Fatal(err)
	}
	return &wbridgeWireFixture{store: store, hook: hook, journey: journey}
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
	}
	if types["ToolExecutionProposed"] < 1 || types["ToolExecutionCompleted"] < 1 {
		t.Fatalf("journal missing execution facts: %+v", types)
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
