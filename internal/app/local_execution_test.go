package app_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

func openExecutionAppStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/app.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return journal.NewStore(db)
}

func executionAppTempDir(t testing.TB) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestLocalExecutionServiceProposeAllowAndSnapshot(t *testing.T) {
	store := openExecutionAppStore(t)
	now := func() time.Time { return time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC) }
	root := executionAppTempDir(t)
	artifactStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	permAuth, err := permissions.NewAuthority(store, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permAuth.DefineProfile(context.Background(), permissions.ProfileInput{
		ProfileID: "app-exec-profile", Mode: permissions.ModeAuto, OwnedPaths: []string{"**"},
	}, "op-profile", "33333333-3333-4333-8333-333333333333"); err != nil {
		t.Fatal(err)
	}
	if _, err := permAuth.BindJob(context.Background(), "job-app", "app-exec-profile", "op-bind", "33333333-3333-4333-8333-333333333333"); err != nil {
		t.Fatal(err)
	}
	workerService, err := work.NewWorkerExecutionService(store, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workerService.Claim(context.Background(), work.ClaimInput{
		JourneyID: "33333333-3333-4333-8333-333333333333", OperationID: "op-claim",
		WorkerID: "worker-app", JobID: "job-app", Lane: schedule.LaneDevelopment,
		CandidateBranch: "codex/candidate-app", CandidateWorktree: root,
	}); err != nil {
		t.Fatal(err)
	}
	adapter, err := execution.NewAdapter(
		store, artifactStore, execution.NewSandboxExecutor(),
		app.NewJournalWorktreeResolverForTest(store), nil, nil, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalExecutionService(store, now, func() string { return "v1" }, adapter)
	if err != nil {
		t.Fatal(err)
	}
	journey := "33333333-3333-4333-8333-333333333333"
	input, _ := json.Marshal(map[string]any{
		"job_id": "job-app",
		"call": map[string]any{
			"tool": "Bash", "command": "printf app-execution", "path": "",
		},
	})
	result, err := service.ExecutionCommand(context.Background(), app.ExecutionCommandRequest{
		JourneyID: journey, OperationID: "op-propose", Action: "propose", Input: input,
	})
	if err != nil {
		t.Fatalf("ExecutionCommand() error = %v", err)
	}
	if result.Result.Verdict != permissions.VerdictAllow || result.Result.ExitCode != 0 {
		t.Fatalf("result = %+v", result.Result)
	}
	snapshot, err := service.ExecutionSnapshot(context.Background(), app.ExecutionSnapshotRequest{JourneyID: journey})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0].Status != "completed" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
