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
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

func TestBw1ExecutionHandlerWiredThroughComposition(t *testing.T) {
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
	journey := "44444444-4444-4444-8444-444444444444"
	if _, err := permAuth.DefineProfile(context.Background(), permissions.ProfileInput{
		ProfileID: "wire-profile", Mode: permissions.ModeAuto, OwnedPaths: []string{"**"},
	}, "op-profile", journey); err != nil {
		t.Fatal(err)
	}
	if _, err := permAuth.BindJob(context.Background(), "job-wire", "wire-profile", "op-bind", journey); err != nil {
		t.Fatal(err)
	}
	workerService, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workerService.Claim(context.Background(), work.ClaimInput{
		JourneyID: journey, OperationID: "op-claim", WorkerID: "wire-worker",
		JobID: "job-wire", Lane: schedule.LaneDevelopment,
		CandidateBranch: "codex/candidate-wire", CandidateWorktree: root,
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
	service, err := app.NewLocalExecutionService(store, now, func() string { return "v1" }, adapter)
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
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-exec-1", JourneyID: journey,
		Method: "execution_snapshot", Params: []byte(`{}`),
	})
	if !response.OK {
		t.Fatalf("execution_snapshot failed: %+v", response.Error)
	}
	input, _ := json.Marshal(map[string]any{
		"job_id": "job-wire",
		"call": map[string]any{
			"tool": "Bash", "command": "printf wire-execution", "path": "",
		},
	})
	response = handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-exec-2", JourneyID: journey,
		Method: "execution_command", Params: mustMarshalJSON(map[string]any{
			"operation_id": "op-wire-execute", "action": "propose",
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
	if result.Result.Verdict != permissions.VerdictAllow {
		t.Fatalf("wire result = %+v", result)
	}
}

func mustMarshalJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
