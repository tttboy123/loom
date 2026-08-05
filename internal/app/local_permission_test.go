package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"

	_ "modernc.org/sqlite"
)

func openPermAppStore(t testing.TB) *journal.Store {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s/perm-app.db?%s",
		t.TempDir(),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return journal.NewStore(db)
}

func mustPermService(t testing.TB, store *journal.Store) *LocalPermissionService {
	t.Helper()
	clock := func() time.Time { return time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC) }
	service, err := NewLocalPermissionService(store, clock, func() string { return "view-v1" })
	if err != nil {
		t.Fatalf("NewLocalPermissionService() error = %v", err)
	}
	return service
}

func permissionCommand(
	t testing.TB,
	service *LocalPermissionService,
	action string,
	input any,
) (PermissionCommandResult, error) {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return service.PermissionCommand(context.Background(), PermissionCommandRequest{
		JourneyID:   "11111111-1111-4111-8111-111111111111",
		OperationID: "op-" + action + "-" + fmt.Sprint(time.Now().UnixNano()),
		Action:      action, Input: body,
	})
}

func TestRed14_SnapshotAndAttentionAreReadOnly(t *testing.T) {
	store := openPermAppStore(t)
	service := mustPermService(t, store)
	input := permissions.ProfileInput{
		ProfileID: "profile-readonly", Mode: permissions.ModeDefault,
		OwnedPaths: []string{"src/**"},
	}
	if _, err := permissionCommand(t, service, "define_profile", input); err != nil {
		t.Fatalf("define_profile error = %v", err)
	}
	before := len(allAppEvents(t, store))

	snapshot, err := service.PermissionSnapshot(context.Background(), PermissionSnapshotRequest{})
	if err != nil {
		t.Fatalf("PermissionSnapshot() error = %v", err)
	}
	if len(snapshot.Profiles) != 1 || snapshot.Profiles[0].ProfileID != "profile-readonly" {
		t.Fatalf("snapshot profiles = %+v", snapshot.Profiles)
	}
	attention, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil {
		t.Fatalf("PermissionAttention() error = %v", err)
	}
	if len(attention.Decisions) != 0 {
		t.Fatalf("attention decisions = %+v, want none", attention.Decisions)
	}
	after := len(allAppEvents(t, store))
	if after != before {
		t.Fatalf("snapshot/attention wrote Journal: before=%d after=%d", before, after)
	}
}

func TestValidateCallAllowAndAskDecisionFact(t *testing.T) {
	store := openPermAppStore(t)
	service := mustPermService(t, store)
	profile := permissions.ProfileInput{
		ProfileID: "profile-vc", Mode: permissions.ModeDefault,
		OwnedPaths: []string{"src/**"},
	}
	if _, err := permissionCommand(t, service, "define_profile", profile); err != nil {
		t.Fatalf("define_profile error = %v", err)
	}
	if _, err := permissionCommand(t, service, "bind_job", struct {
		JobID     string `json:"job_id"`
		ProfileID string `json:"profile_id"`
	}{JobID: "job-vc", ProfileID: "profile-vc"}); err != nil {
		t.Fatalf("bind_job error = %v", err)
	}
	if _, err := permissionCommand(t, service, "add_rule", struct {
		Rule         permissions.Rule `json:"rule"`
		AuthorizedBy string           `json:"authorized_by"`
	}{Rule: permissions.Rule{
		RuleID: "r-vc-allow", Scope: permissions.ScopeJob, ScopeID: "job-vc",
		Action: permissions.ActionAllow, Tool: permissions.ToolBash, Pattern: "go test *",
	}, AuthorizedBy: "user-1"}); err != nil {
		t.Fatalf("add_rule error = %v", err)
	}

	allow, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-vc", Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "go test ./..."}})
	if err != nil {
		t.Fatalf("validate_call allow error = %v", err)
	}
	if allow.Verdict != permissions.VerdictAllow {
		t.Fatalf("verdict = %s, want allow", allow.Verdict)
	}
	if len(allAppEvents(t, store)) != 3 {
		t.Fatalf("allow path must not record a decision fact, events = %d", len(allAppEvents(t, store)))
	}

	ask, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "job-vc", Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "curl https://example.com"}})
	if err != nil {
		t.Fatalf("validate_call ask error = %v", err)
	}
	if ask.Verdict != permissions.VerdictAsk {
		t.Fatalf("verdict = %s, want ask", ask.Verdict)
	}
	if len(ask.EventIDs) != 1 {
		t.Fatalf("ask path must record one decision fact, got %d", len(ask.EventIDs))
	}
	attention, err := service.PermissionAttention(context.Background(), PermissionAttentionRequest{})
	if err != nil {
		t.Fatalf("PermissionAttention() error = %v", err)
	}
	if len(attention.Decisions) != 1 || attention.Decisions[0].JobID != "job-vc" {
		t.Fatalf("attention decisions = %+v", attention.Decisions)
	}
	if attention.Decisions[0].Command != "curl https://example.com" ||
		attention.Decisions[0].Tool != permissions.ToolBash {
		t.Fatalf("decision fact missing call details: %+v", attention.Decisions[0])
	}
}

func TestPermissionCommandUnknownActionAndUnboundJob(t *testing.T) {
	store := openPermAppStore(t)
	service := mustPermService(t, store)
	if _, err := permissionCommand(t, service, "bogus_action", struct{}{}); err == nil {
		t.Fatal("unknown action must error")
	}
	if _, err := permissionCommand(t, service, "validate_call", struct {
		JobID string                   `json:"job_id"`
		Call  permissions.ProposedCall `json:"call"`
	}{JobID: "unbound", Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: "cat x"}}); err != nil {
		t.Fatalf("unbound validate_call error = %v", err)
	}
}

func allAppEvents(t testing.TB, store *journal.Store) []journal.Event {
	t.Helper()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return events
}
