package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"

	_ "modernc.org/sqlite"
)

func TestBp1PermissionHandlerWiredThroughComposition(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/dbg.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	service, err := app.NewLocalPermissionService(store, func() time.Time { return time.Now().UTC() },
		func() string { return "v1" }, nil)
	if err != nil {
		t.Fatal(err)
	}
	permissionAPI, err := api.NewLocalPermissionAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithComposition(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, permissionAPI, nil, nil, nil, nil)
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-1", JourneyID: "11111111-1111-4111-8111-111111111111",
		Method: "permissions_snapshot", Params: []byte(`{}`),
	})
	t.Logf("snapshot ok=%v err=%+v", response.OK, response.Error)
	if !response.OK {
		t.Fatalf("permissions_snapshot failed: %+v", response.Error)
	}
}

func TestBp1DaemonApprovalPortWiredEndToEnd(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/dbg.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	now := func() time.Time { return time.Now().UTC() }
	port, err := newPermissionApprovalPort(store, now)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalPermissionService(store, now, func() string { return "v1" }, port)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	input, _ := json.Marshal(permissions.ProfileInput{
		ProfileID: "profile-wire", Mode: permissions.ModeDefault,
	})
	if _, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID:   "11111111-1111-4111-8111-111111111111",
		OperationID: "op-wire-profile", Action: "define_profile", Input: input,
	}); err != nil {
		t.Fatal(err)
	}
	bind, _ := json.Marshal(map[string]any{"job_id": "job-wire", "profile_id": "profile-wire"})
	if _, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID:   "11111111-1111-4111-8111-111111111111",
		OperationID: "op-wire-bind", Action: "bind_job", Input: bind,
	}); err != nil {
		t.Fatal(err)
	}
	call, _ := json.Marshal(map[string]any{
		"job_id": "job-wire",
		"call": map[string]any{
			"tool": "Bash", "command": "curl https://example.com", "path": "",
		},
	})
	result, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID:   "11111111-1111-4111-8111-111111111111",
		OperationID: "op-wire-validate", Action: "validate_call", Input: call,
	})
	if err != nil {
		t.Fatalf("validate_call (real-time clock) error = %v", err)
	}
	if result.Verdict != permissions.VerdictAsk || result.ApprovalID == "" {
		t.Fatalf("ask result = %+v", result)
	}
	resolve, _ := json.Marshal(map[string]any{
		"approval_id": result.ApprovalID, "approval_digest": result.ApprovalDigest,
		"resolution": "allow", "resolved_by": "user-1",
	})
	resolved, err := service.PermissionCommand(ctx, app.PermissionCommandRequest{
		JourneyID:   "11111111-1111-4111-8111-111111111111",
		OperationID: "op-wire-resolve", Action: "resolve_approval", Input: resolve,
	})
	if err != nil {
		t.Fatalf("resolve_approval error = %v", err)
	}
	if resolved.Note == "" {
		t.Fatalf("resolve result = %+v", resolved)
	}
}
