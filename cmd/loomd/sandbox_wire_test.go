package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/sandbox"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

const sandboxWireJourney = "55555555-5555-4555-8555-555555555555"

func buildSandboxWireFixture(t testing.TB, gate execution.SandboxGate) (*execution.Adapter, *journal.Store) {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/sb.db?%s", t.TempDir(), values.Encode()))
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
	if _, err := permAuth.DefineProfile(context.Background(), permissions.ProfileInput{
		ProfileID: "sb-wire-profile", Mode: permissions.ModeAuto, OwnedPaths: []string{"**"},
	}, "op-sb-profile", sandboxWireJourney); err != nil {
		t.Fatal(err)
	}
	if _, err := permAuth.BindJob(context.Background(), "job-sb-wire", "sb-wire-profile", "op-sb-bind", sandboxWireJourney); err != nil {
		t.Fatal(err)
	}
	workerService, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workerService.Claim(context.Background(), work.ClaimInput{
		JourneyID: sandboxWireJourney, OperationID: "op-sb-claim", WorkerID: "sb-wire-worker",
		JobID: "job-sb-wire", Lane: schedule.LaneDevelopment,
		CandidateBranch: "codex/candidate-sb-wire", CandidateWorktree: root,
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
	if gate != nil {
		adapter = adapter.WithSandboxGate(gate)
	}
	return adapter, store
}

func wireSandboxProposal(t testing.TB, adapter *execution.Adapter, store *journal.Store) app.ExecutionCommandResult {
	t.Helper()
	service, err := app.NewLocalExecutionService(
		store, func() time.Time { return time.Now().UTC() },
		func() string { return "v1" }, adapter,
	)
	if err != nil {
		t.Fatal(err)
	}
	executionAPI, err := api.NewLocalExecutionAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithComposition(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, executionAPI, nil, nil, nil,
	)
	input, _ := json.Marshal(map[string]any{
		"job_id": "job-sb-wire",
		"call":   map[string]any{"tool": "Bash", "command": "printf sb-wire", "path": ""},
	})
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-sb-1", JourneyID: sandboxWireJourney,
		Method: "execution_command", Params: mustMarshalJSON(map[string]any{
			"operation_id": "op-sb-execute", "action": "propose",
			"input": json.RawMessage(input),
		}),
	})
	if !response.OK {
		t.Fatalf("execution_command failed: %+v", response.Error)
	}
	var result app.ExecutionCommandResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// TestSandboxWireDefaultOffPreservesLocalExecution: no gate means the B-W1
// local SandboxExecutor path is unchanged (default off).
func TestSandboxWireDefaultOffPreservesLocalExecution(t *testing.T) {
	adapter, store := buildSandboxWireFixture(t, nil)
	result := wireSandboxProposal(t, adapter, store)
	if result.Result.Verdict != permissions.VerdictAllow || result.Result.ExitCode != 0 {
		t.Fatalf("default-off local execution broken: %+v", result.Result)
	}
}

// TestSandboxWireRequiredUnavailableBackendFailsClosed: a Required policy for
// an unmounted backend (name mismatch) must deny with zero side effects.
func TestSandboxWireRequiredUnavailableBackendFailsClosed(t *testing.T) {
	gate := execution.NewBackendGate(nil, func(context.Context, string) (execution.SandboxPolicy, error) {
		return execution.SandboxPolicy{Required: true, Backend: "gVisor"}, nil
	})
	adapter, store := buildSandboxWireFixture(t, gate)
	result := wireSandboxProposal(t, adapter, store)
	if result.Result.Verdict != permissions.VerdictDeny {
		t.Fatalf("required unavailable sandbox must fail closed: %+v", result.Result)
	}
	if result.Result.Denial.Reason == "" || result.Result.ExitCode != 0 {
		t.Fatalf("deny must carry reason and zero side effects: %+v", result.Result)
	}
}

// TestSandboxWireLoopbackMountedExecutes: with the loopback backend mounted
// and a Required policy naming it, the allow path executes normally.
func TestSandboxWireLoopbackMountedExecutes(t *testing.T) {
	loopback, err := sandbox.NewLoopbackBackend(t.TempDir(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	gate := execution.NewBackendGate(loopback, func(context.Context, string) (execution.SandboxPolicy, error) {
		return execution.SandboxPolicy{Required: true, Backend: "loopback"}, nil
	})
	adapter, store := buildSandboxWireFixture(t, gate)
	result := wireSandboxProposal(t, adapter, store)
	if result.Result.Verdict != permissions.VerdictAllow || result.Result.ExitCode != 0 {
		t.Fatalf("mounted loopback must allow: %+v", result.Result)
	}
}
