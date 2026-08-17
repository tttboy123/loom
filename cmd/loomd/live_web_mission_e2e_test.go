package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"loom-pi-rebuild/internal/localipc"
)

// TestLiveMissionAgentCallsWebSearch is the final installed-live acceptance:
// a real Mission whose subagent holds a web_search Enrollment runs a
// web-requiring objective, and the Journal must record at least one execution
// fact carrying the WebSearch tool (i.e. the model actually invoked the Loom
// harness web tool against the live internet). No Codex.
func TestLiveMissionAgentCallsWebSearch(t *testing.T) {
	if os.Getenv("LOOM_LIVE_WEB_MISSION") != "1" {
		t.Skip("live web-mission reproduction requires LOOM_LIVE_WEB_MISSION=1 " +
			"(plus LOOM_LIVE_TEAM_E2E=1 and LOOM_LIVE_NET=1); currently exposes " +
			"the mission-start conflict for an enrollment-bound team (see evidence)")
	}
	socketPath := os.Getenv("LOOM_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	ensureProviderVerified(t, ctx, client, "deepseek", os.Getenv("DEEPSEEK_API_KEY"))
	configureProviderGovernance(t, ctx, client, "deepseek", "deepseek.primary", "deepseek-chat")
	archiveLiveTeams(t, ctx, client)

	// 1. Configure a fresh web_search Enrollment under deepseek.primary.
	policy := providerPolicyLineage(t, ctx, client, "deepseek", "deepseek.primary")
	enrollTS := time.Now().UTC().Format("20060102150405")
	webSearchID := "enroll-web-mission-" + enrollTS
	if err := client.Call(ctx, "remote_tool_backend_enrollment_configure", map[string]any{
		"enrollment_id": webSearchID, "backend_kind": "web_search",
		"adapter_id":  "builtin.search.deepseek.v1",
		"provider_id": "deepseek", "provider_account_id": "deepseek.primary",
		"provider_account_policy_version":  policy.Version,
		"provider_account_policy_revision": policy.Revision,
		"provider_account_policy_digest":   policy.Digest,
		"endpoint_fingerprint":             "948f1ecb6b48f91adc4e110d0351cd172b16450e9936d358992e0dfad7b863f3",
		"expected_revision":                0, "maximum_concurrent_calls": 2,
		"maximum_calls_per_attempt": 4, "timeout_seconds": 30,
		"maximum_result_bytes": 4096, "maximum_budget_units": 100000,
		"operation_id": matrixOperationID(),
	}, &struct{}{}); err != nil {
		t.Fatalf("configure web_search: %v", err)
	}
	enrollments := remoteToolEnrollments(t, ctx, client, "deepseek", "deepseek.primary")
	webDigest, webActive, webCurrent := enrollmentLineage(enrollments, webSearchID)
	if webDigest == "" || !webActive || !webCurrent {
		t.Fatalf("web_search enrollment not active/policy-current: %#v", enrollments)
	}

	// 2. Build a Team and bind the web_search Enrollment to the subagent.
	roleOptions := refreshRoleOptions(t, ctx, client)
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live Web Mission Team")
	session = editSession(t, ctx, client, session, "purpose",
		"Verify the agent calls the governed web_search tool on the live internet")
	mainRole := firstRole(roleOptions, "main", "deepseek")
	subRole := firstRole(roleOptions, "subagent", "deepseek")
	session = editSession(t, ctx, client, session, "main_role", mainRole.ID)
	defaultSubAgent := firstPreviewSubagent(session)
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_role", subRole.ID,
		defaultSubAgent.AgentDefinitionID,
	)
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_remote_tool_enrollment",
		webSearchID+":"+webDigest, defaultSubAgent.AgentDefinitionID,
	)
	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf("draft not confirmable: canConfirm=%v binding=%q gaps=%v",
			session.CanConfirm, session.BindingDg, session.Preview.CompatibilityGaps)
	}
	beforeTeamIDs := map[string]bool{}
	for _, existing := range refreshTeams(t, ctx, client) {
		beforeTeamIDs[existing.TeamInstanceID] = true
	}
	definitionID := "team-live-web-" + time.Now().UTC().Format("20060102T150405Z")
	if _, err := confirmSession(t, ctx, client, session, definitionID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	var team teamSummary
	for _, candidate := range refreshTeams(t, ctx, client) {
		if candidate.SourceKind == "saved_team" && candidate.Confirmed &&
			candidate.Executable && !beforeTeamIDs[candidate.TeamInstanceID] {
			team = candidate
		}
	}
	if team.TeamInstanceID == "" {
		t.Fatal("web mission team not materialized")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer cleanupCancel()
		archiveLiveTeams(t, cleanupCtx, client, definitionID)
	})

	// 3. Preflight must resolve the Enrollment-bound subagent.
	preflight, err := preflightMission(t, ctx, client, team)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	subagentReady := false
	for _, node := range preflight.Preflight.Nodes {
		if node.Role == "subagent" {
			subagentReady = node.Status == "ready"
		}
	}
	if !subagentReady {
		t.Fatalf("enrollment-bound subagent not ready: %#v", preflight.Preflight.Nodes)
	}

	// 4. Run a web-requiring objective: the model must use web_search.
	run, err := runMissionWithObjective(
		t, ctx, client, team, preflight,
		"Use the web_search tool to look up the GitHub repository multica-ai/multica "+
			"and report its one-line description plus the source URL.",
	)
	if err != nil {
		t.Fatalf("run mission: %v", err)
	}
	t.Logf("mission start status=%s note=%q", run.Status, run.Note)

	// 5. Wait for the board to leave running (bounded), then assert the
	//    Journal recorded a WebSearch execution fact.
	waitForMissionSettled(t, ctx, client, team.TeamInstanceID)
	webCalls := countJournalWebToolFacts(t, "WebSearch", "WebFetch")
	if webCalls == 0 {
		t.Fatalf("no web tool execution fact in Journal (model did not call web tool)")
	}
	t.Logf("journal web tool facts: WebSearch/WebFetch = %d", webCalls)
}

func runMissionWithObjective(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	team teamSummary,
	preflight preflightEnvelope,
	objective string,
) (missionRunResult, error) {
	t.Helper()
	if preflight.Preflight == nil {
		return missionRunResult{}, fmt.Errorf("no preflight envelope")
	}
	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		var raw json.RawMessage
		err := client.Call(ctx, "mission_execution", map[string]any{
			"schema_version": 1, "operation": "start",
			"mission_id":            "mission/" + team.TeamInstanceID,
			"team_instance_id":      team.TeamInstanceID,
			"work_package_id":       "work-package.coding",
			"work_package_digest":   "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f",
			"objective":             objective,
			"expected_view_version": currentViewVersion(t, ctx, client),
			"preflight_digest":      preflight.Preflight.PreflightDigest,
			"correlation_id":        matrixOperationID(),
		}, &raw)
		if err != nil {
			var remote *localipc.RemoteError
			if errors.As(err, &remote) && remote.Code == "conflict" && attempt < attempts-1 {
				// The view advanced between preflight and start: re-preflight
				// with a fresh digest + view version, then retry.
				time.Sleep(2 * time.Second)
				fresh, preflightErr := preflightMission(t, ctx, client, team)
				if preflightErr != nil {
					return missionRunResult{}, preflightErr
				}
				preflight = fresh
				continue
			}
			return missionRunResult{}, err
		}
		var envelope struct {
			Result *struct {
				Status string `json:"status"`
				Note   string `json:"note"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return missionRunResult{}, fmt.Errorf("decode: %w", err)
		}
		if envelope.Result == nil {
			return missionRunResult{}, fmt.Errorf("start envelope has no result: %s", raw)
		}
		return missionRunResult{
			Status: envelope.Result.Status,
			Note:   envelope.Result.Note,
		}, nil
	}
	return missionRunResult{}, fmt.Errorf("mission start view version conflict retries exhausted")
}

func waitForMissionSettled(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamInstanceID string,
) {
	t.Helper()
	deadline := time.Now().Add(9 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			t.Fatalf("context expired waiting for mission: %v", ctx.Err())
		}
		board := readTeamBoard(t, ctx, client, teamInstanceID)
		status := board.Status
		if status != "" && status != "running" {
			t.Logf("mission board status=%s", status)
			return
		}
		select {
		case <-time.After(15 * time.Second):
		case <-ctx.Done():
			t.Fatalf("context expired waiting for mission: %v", ctx.Err())
		}
	}
	t.Fatal("mission did not settle within 9 minutes")
}

// countJournalWebToolFacts reads the installed daemon Journal (read-only) and
// counts execution facts whose payload names the web tool.
func countJournalWebToolFacts(t *testing.T, tools ...string) int {
	t.Helper()
	statePath := os.Getenv("LOOM_STATE_DB")
	if statePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		statePath = filepath.Join(home, "Library", "Application Support", "Loom", "state", "loom.db")
	}
	database, err := sql.Open("sqlite", "file:"+statePath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	defer database.Close()
	count := 0
	for _, tool := range tools {
		var row int
		if err := database.QueryRow(
			`SELECT COUNT(*) FROM events WHERE payload_json LIKE ?`,
			`%"tool":"`+tool+`"%`,
		).Scan(&row); err != nil {
			t.Fatalf("query journal: %v", err)
		}
		count += row
	}
	return count
}

var _ = strings.TrimSpace
