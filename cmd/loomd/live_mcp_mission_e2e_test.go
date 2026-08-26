package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
)

const (
	liveMCPMissionObjective = "This is a two-role governed MCP acceptance. The OpenCode subagent must call loom_mcp_call exactly once with server mcp.live.probe, tool lookup, and arguments {\"value\":\"MCP-LIVE-OK\"}; it must wait for the real tool result and report the exact token MCP-LIVE-OK plus the returned initialized and env fields. The Loom Native main coordinator must not call tools and should report the exact token MCP-COORDINATOR-OK. Do not claim a tool result without calling the tool."
)

func TestLiveMCPRevokeObjectiveCarriesDeterministicFallbackEvidence(t *testing.T) {
	route := liveMCPFallbackRoute{
		runtimeID:  productNativeAgentRuntimeInstanceID,
		providerID: "deepseek", providerAccountID: "deepseek.primary",
	}
	objective := liveMCPRevokeObjectiveForRoute(route)
	if len(objective) > 512 {
		t.Fatalf("fallback objective exceeds Agent title boundary: %d", len(objective))
	}
	for _, required := range []string{
		"first and only action",
		"MCP-FALLBACK-OK",
		"Harness=Loom-Native",
		"Provider=deepseek",
		"ProviderAccount=deepseek.primary",
		"Attempt=2",
		"FallbackApprovalVersion=1",
		"ToolExecution=none",
	} {
		if !strings.Contains(objective, required) {
			t.Fatalf("fallback objective missing %q: %s", required, objective)
		}
	}
}

func TestLiveMissionOpenCodeCallsMCPAndIsolatesMidflightRevoke(t *testing.T) {
	if os.Getenv("LOOM_LIVE_MCP_MISSION") != "1" {
		t.Skip("installed MCP Mission gate requires LOOM_LIVE_MCP_MISSION=1")
	}
	socketPath := os.Getenv("LOOM_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	startedMarker := requireLiveMCPMarker(t, "LOOM_LIVE_MCP_STARTED_MARKER")
	releaseMarker := requireLiveMCPMarker(t, "LOOM_LIVE_MCP_RELEASE_MARKER")
	_ = os.Remove(startedMarker)
	_ = os.Remove(releaseMarker)
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	ensureProviderVerified(t, ctx, client, "deepseek", os.Getenv("DEEPSEEK_API_KEY"))
	configureProviderGovernance(t, ctx, client, "deepseek", "deepseek.primary", "deepseek-chat")
	ensureProviderVerified(t, ctx, client, "minimax", os.Getenv("MINIMAX_API_KEY"))
	configureProviderGovernance(t, ctx, client, "minimax", "minimax.primary", "MiniMax-M3")
	archiveLiveTeams(t, ctx, client)
	roleOptions := refreshRoleOptions(t, ctx, client)
	if role := firstRoleForRuntime(
		roleOptions, "subagent", productOpenCodeRuntimeInstanceID, "deepseek",
	); role.ID == "" {
		t.Fatalf("OpenCode governed-tool role is not executable: %#v", roleOptions)
	}

	policy := providerPolicyLineage(t, ctx, client, "deepseek", "deepseek.primary")
	fallbackRoute := requireLiveMCPFallbackRoute(t)
	revokeObjective := liveMCPRevokeObjectiveForRoute(fallbackRoute)
	t.Logf("explicit fallback route: %s/%s", fallbackRoute.providerID, fallbackRoute.runtimeID)
	if os.Getenv("LOOM_LIVE_MCP_FALLBACK_ONLY") != "1" {
		positiveID, positiveDigest := configureLiveMCPEnrollment(t, ctx, client, policy, "positive")
		positiveTeam := confirmLiveMCPTeam(
			t, ctx, client, roleOptions, positiveID, positiveDigest, "positive", false,
			fallbackRoute,
		)
		positivePreflight, err := preflightMission(
			t, ctx, client, positiveTeam, liveMCPMissionObjective,
		)
		if err != nil {
			t.Fatal(err)
		}
		assertLiveMCPPreflight(t, positivePreflight, "ready")
		if _, err := runMissionWithObjective(
			t, ctx, client, positiveTeam, positivePreflight, liveMCPMissionObjective,
		); err != nil {
			t.Fatalf("start positive MCP Mission: %v", err)
		}
		positiveBoard := waitForLiveMCPPositiveExecution(
			t, ctx, client, positiveTeam.TeamInstanceID,
		)
		assertLiveMCPExecutionFact(t, positiveBoard, "MCP-LIVE-OK")
		assertLiveMCPPermissionFacts(t, positiveBoard, true)
	}

	revokeID, revokeDigest := configureLiveMCPEnrollment(t, ctx, client, policy, "revoke")
	revokeTeam := confirmLiveMCPTeam(
		t, ctx, client, roleOptions, revokeID, revokeDigest, "revoke", false,
		fallbackRoute,
	)
	revokePreflight, err := preflightMission(
		t, ctx, client, revokeTeam, revokeObjective,
	)
	if err != nil {
		t.Fatalf("preflight fallback Team %#v: %v", revokeTeam, err)
	}
	assertLiveMCPPreflight(t, revokePreflight, "ready")
	assertLiveMCPFallbackApproval(t, revokePreflight, false)
	if !approveOnePreparedFallback(t, ctx, client, revokeTeam.TeamInstanceID) {
		t.Fatal("prepared MCP fallback decision not found")
	}
	revokePreflight, err = preflightMission(
		t, ctx, client, revokeTeam, revokeObjective,
	)
	if err != nil {
		t.Fatal(err)
	}
	assertLiveMCPPreflight(t, revokePreflight, "ready")
	assertLiveMCPFallbackApproval(t, revokePreflight, true)
	revision := liveMCPEnrollmentRevision(t, ctx, client, revokeID)
	revokerClient, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 120 * time.Second,
	})
	if err != nil {
		t.Fatalf("revoker client: %v", err)
	}
	revokerReady := make(chan struct{})
	revokeResult := make(chan error, 1)
	revokeOperationID := matrixOperationID()
	go func() {
		close(revokerReady)
		if err := waitForLiveMCPMarkerPath(ctx, startedMarker); err != nil {
			revokeResult <- err
			return
		}
		revokeResult <- revokerClient.Call(
			ctx, "remote_tool_backend_enrollment_revoke", map[string]any{
				"enrollment_id": revokeID, "provider_id": "deepseek",
				"provider_account_id": "deepseek.primary",
				"expected_revision":   revision,
				"operation_id":        revokeOperationID,
			}, &struct{}{},
		)
	}()
	<-revokerReady
	if _, err := runMissionWithObjective(
		t, ctx, client, revokeTeam, revokePreflight, revokeObjective,
	); err != nil {
		t.Fatalf("start revocation MCP Mission: %v", err)
	}
	if err := <-revokeResult; err != nil {
		t.Fatalf("mid-flight revoke: %v", err)
	}
	if err := os.WriteFile(releaseMarker, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	revokedBoard := waitForLiveMCPApprovedFallback(
		t, ctx, client, revokeTeam.TeamInstanceID, fallbackRoute,
	)
	assertLiveMCPPermissionFacts(t, revokedBoard, true)
	afterRevoke, err := preflightMission(
		t, ctx, client, revokeTeam, revokeObjective,
	)
	if err != nil {
		t.Fatal(err)
	}
	mainReady := false
	subagentBlocked := false
	for _, node := range afterRevoke.Preflight.Nodes {
		if node.Role == "main" && node.Status == "ready" {
			mainReady = true
		}
		if node.Role == "subagent" && node.Status == "blocked" &&
			strings.Contains(node.BlockReason, "enrollment") {
			subagentBlocked = true
		}
	}
	if !subagentBlocked || !mainReady {
		t.Fatalf("post-revoke isolation subagentBlocked=%t mainReady=%t nodes=%#v",
			subagentBlocked, mainReady, afterRevoke.Preflight.Nodes)
	}
}

func configureLiveMCPEnrollment(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	policy providerPolicyLineageValue,
	suffix string,
) (string, string) {
	t.Helper()
	id := "enroll-mcp-mission-" + suffix + "-" + time.Now().UTC().Format("20060102150405")
	if err := client.Call(ctx, "remote_tool_backend_enrollment_configure", map[string]any{
		"enrollment_id": id, "backend_kind": "mcp_server",
		"adapter_id": "builtin.mcp.stdio.v1", "provider_id": "deepseek",
		"provider_account_id":              "deepseek.primary",
		"provider_account_policy_version":  policy.Version,
		"provider_account_policy_revision": policy.Revision,
		"provider_account_policy_digest":   policy.Digest,
		"endpoint_fingerprint":             "948f1ecb6b48f91adc4e110d0351cd172b16450e9936d358992e0dfad7b863f3",
		"mcp_server_id":                    "mcp.live.probe", "allowed_tools": []string{"lookup"},
		"expected_revision": 0, "maximum_concurrent_calls": 1,
		"maximum_calls_per_attempt": 4, "timeout_seconds": 120,
		"maximum_result_bytes": 4096, "maximum_budget_units": 100000,
		"operation_id": matrixOperationID(),
	}, &struct{}{}); err != nil {
		t.Fatalf("configure MCP Enrollment: %v", err)
	}
	digest, active, current := enrollmentLineage(
		remoteToolEnrollments(t, ctx, client, "deepseek", "deepseek.primary"), id,
	)
	if digest == "" || !active || !current {
		t.Fatalf("MCP Enrollment is not active/current: id=%s digest=%q", id, digest)
	}
	return id, digest
}

func confirmLiveMCPTeam(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	roleOptions []roleOption,
	enrollmentID string,
	enrollmentDigest string,
	suffix string,
	enrollmentOnMain bool,
	fallbackRoute liveMCPFallbackRoute,
) teamSummary {
	t.Helper()
	// Runtime observations can publish a newer role option while the preceding
	// live Mission runs. A new Builder session must use the current directory.
	roleOptions = refreshRoleOptions(t, ctx, client)
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live MCP "+suffix)
	session = editSession(t, ctx, client, session, "purpose", "Verify governed MCP execution and isolation")
	mainRuntime := productNativeAgentRuntimeInstanceID
	subagentRuntime := productOpenCodeRuntimeInstanceID
	if enrollmentOnMain {
		mainRuntime = productOpenCodeRuntimeInstanceID
		subagentRuntime = productOpenCodeRuntimeInstanceID
	}
	mainRole := firstRoleForRuntime(roleOptions, "main", mainRuntime, "deepseek")
	subagentRole := firstRoleForRuntime(
		roleOptions, "subagent", subagentRuntime, "deepseek",
	)
	defaultSubagent := firstPreviewSubagent(session)
	if mainRole.ID == "" || subagentRole.ID == "" || defaultSubagent.AgentDefinitionID == "" {
		t.Fatal("required MCP Team roles are unavailable")
	}
	session = editSession(t, ctx, client, session, "main_role", mainRole.ID)
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_role", subagentRole.ID,
		defaultSubagent.AgentDefinitionID,
	)
	if enrollmentOnMain {
		session = editSession(
			t, ctx, client, session, "main_remote_tool_enrollment",
			enrollmentID+":"+enrollmentDigest,
		)
	} else {
		session = editSessionTargeted(
			t, ctx, client, session, "subagent_remote_tool_enrollment",
			enrollmentID+":"+enrollmentDigest, defaultSubagent.AgentDefinitionID,
		)
	}
	if enrollmentOnMain {
		fallbackRole := firstRoleForRuntime(
			roleOptions, "main", fallbackRoute.runtimeID, fallbackRoute.providerID,
		)
		if fallbackRole.ID == "" {
			t.Fatal("Loom Native fallback role is unavailable")
		}
		session = editSession(
			t, ctx, client, session, "main_fallback_role", fallbackRole.ID,
		)
	} else if suffix == "revoke" {
		fallbackRole := firstRoleForRuntime(
			roleOptions, "subagent", fallbackRoute.runtimeID, fallbackRoute.providerID,
		)
		if fallbackRole.ID == "" {
			t.Fatal("Loom Native fallback role is unavailable")
		}
		session = editSessionTargeted(
			t, ctx, client, session, "subagent_fallback_role", fallbackRole.ID,
			defaultSubagent.AgentDefinitionID,
		)
	}
	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf("MCP Team draft not confirmable: gaps=%v", session.Preview.CompatibilityGaps)
	}
	before := make(map[string]bool)
	for _, team := range refreshTeams(t, ctx, client) {
		before[team.TeamInstanceID] = true
	}
	definitionID := "team-live-mcp-" + suffix + "-" + time.Now().UTC().Format("20060102T150405Z")
	if _, err := confirmSession(t, ctx, client, session, definitionID); err != nil {
		t.Fatal(err)
	}
	for _, team := range refreshTeams(t, ctx, client) {
		if team.SourceKind == "saved_team" && team.Confirmed && team.Executable &&
			!before[team.TeamInstanceID] {
			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				archiveLiveTeams(t, cleanupCtx, client, definitionID)
			})
			return team
		}
	}
	t.Fatal("confirmed MCP Team not found")
	return teamSummary{}
}

func assertLiveMCPPreflight(t *testing.T, preflight preflightEnvelope, status string) {
	t.Helper()
	if preflight.Preflight == nil {
		t.Fatal("MCP preflight missing")
	}
	for _, node := range preflight.Preflight.Nodes {
		if node.Status != status {
			t.Fatalf("MCP preflight node=%s status=%s block=%q", node.LogicalNodeID, node.Status, node.BlockReason)
		}
	}
}

func assertLiveMCPFallbackApproval(
	t *testing.T,
	preflight preflightEnvelope,
	approved bool,
) {
	t.Helper()
	for _, node := range preflight.Preflight.Nodes {
		if !node.FallbackConfigured {
			continue
		}
		if !node.FallbackApprovalRequired ||
			node.FallbackApprovalAvailable != approved ||
			(node.FallbackApprovalAvailable && node.FallbackApprovalVersion != 1) {
			t.Fatalf("MCP fallback approval state = %#v, approved=%t", node, approved)
		}
		return
	}
	t.Fatal("MCP fallback node missing")
}

func approveOnePreparedFallback(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamInstanceID string,
) bool {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "snapshot", map[string]any{
		"after_team_id": "", "after_runtime_id": "", "after_run_id": "",
		"after_evidence_id": "", "limit": 64,
	}, &raw); err != nil {
		t.Fatalf("snapshot prepared fallback: %v", err)
	}
	var snapshot struct {
		Prepared []app.MissionDecisionCommand `json:"prepared_decisions"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode prepared fallback: %v", err)
	}
	for _, command := range snapshot.Prepared {
		if command.TeamInstanceID != teamInstanceID || command.Kind != "fallback" {
			continue
		}
		var sheet app.MissionDecisionSheet
		if err := client.Call(ctx, "mission_decision", command, &sheet); err != nil {
			var remote *localipc.RemoteError
			if errors.As(err, &remote) && remote.Code == "conflict" {
				return false
			}
			t.Fatalf("read fallback decision: %v", err)
		}
		if !sheet.Prepared || !containsString(sheet.PreparedActions, "approve_fallback") {
			t.Fatalf("fallback decision is not prepared: %#v", sheet)
		}
		command.Operation = "submit"
		command.Action = "approve_fallback"
		command.CorrelationID = matrixOperationID()
		var result app.MissionDecisionResult
		if err := client.Call(ctx, "mission_decision", command, &result); err != nil {
			t.Fatalf("approve fallback decision: %v", err)
		}
		if !result.Authoritative || result.Status != "approved" {
			t.Fatalf("fallback approval was not authoritative: %#v", result)
		}
		return true
	}
	return false
}

func waitForLiveMCPPositiveExecution(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamID string,
) teamBoardWire {
	t.Helper()
	deadline := time.Now().Add(9 * time.Minute)
	for time.Now().Before(deadline) {
		board := readTeamBoard(t, ctx, client, teamID)
		openCodeSucceeded := false
		for _, node := range board.Nodes {
			if node.HarnessAdapter == harnessadapter.OpenCodeAdapterType &&
				node.Status == "succeeded" {
				openCodeSucceeded = true
				break
			}
		}
		if openCodeSucceeded && board.Status != "" && board.Status != "running" {
			return board
		}
		if board.Status == "ready_for_review" {
			_ = acceptOnePreparedReview(t, ctx, client, teamID)
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	t.Fatal("OpenCode MCP execution did not reach a successful terminal node")
	return teamBoardWire{}
}

func waitForLiveMCPApprovedFallback(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamID string,
	fallbackRoute liveMCPFallbackRoute,
) teamBoardWire {
	t.Helper()
	deadline := time.Now().Add(9 * time.Minute)
	var lastBoard teamBoardWire
	var fallbackSucceeded bool
	var fallbackAccounted bool
	var fallbackTerminalFailure bool
	var peerSucceeded bool
	for time.Now().Before(deadline) {
		board := readTeamBoard(t, ctx, client, teamID)
		lastBoard = board
		fallbackSucceeded = false
		fallbackAccounted = false
		fallbackTerminalFailure = false
		peerSucceeded = false
		for _, node := range board.Nodes {
			if node.LogicalNodeID != "main" && node.Status == "succeeded" &&
				node.HarnessAdapter == nativeadapter.LoomNativeAgentAdapterType &&
				node.ProviderID == fallbackRoute.providerID &&
				node.ProviderAccountID == fallbackRoute.providerAccountID &&
				node.CurrentAttempt == 2 && node.FallbackConfigured &&
				node.FallbackConsumed && node.FallbackApprovalVersion == 1 &&
				node.AccountingAvailable && node.UsageObserved &&
				node.TotalTokens > 0 {
				fallbackSucceeded = true
			}
			if node.LogicalNodeID != "main" && node.CurrentAttempt >= 2 &&
				node.FallbackConsumed &&
				(node.Status == "failed" || node.Status == "blocked" ||
					node.Status == "cancelled") {
				fallbackTerminalFailure = true
			}
			if node.LogicalNodeID == "main" && node.Status == "succeeded" &&
				node.HarnessAdapter == nativeadapter.LoomNativeAgentAdapterType {
				peerSucceeded = true
			}
		}
		for _, row := range board.ProviderAccounts {
			if row.ProviderID == fallbackRoute.providerID &&
				row.ProviderAccountID == fallbackRoute.providerAccountID &&
				row.AccountingAttemptCount > 0 &&
				row.UsageAttemptCount > 0 && row.TotalTokens > 0 {
				fallbackAccounted = true
				break
			}
		}
		if fallbackSucceeded && fallbackAccounted && peerSucceeded {
			return board
		}
		if fallbackTerminalFailure {
			t.Fatalf("approved fallback reached a terminal failure: status=%s nodes=%#v",
				board.Status, board.Nodes)
		}
		if board.Status == "ready_for_review" {
			_ = acceptOnePreparedReview(t, ctx, client, teamID)
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			t.Fatalf(
				"context ended while waiting for approved fallback: err=%v status=%s fallback_succeeded=%t fallback_accounted=%t peer_succeeded=%t terminal_failure=%t nodes=%#v accounts=%#v",
				ctx.Err(), lastBoard.Status, fallbackSucceeded, fallbackAccounted,
				peerSucceeded, fallbackTerminalFailure, lastBoard.Nodes,
				lastBoard.ProviderAccounts,
			)
		}
	}
	t.Fatalf(
		"approved Agent-local fallback did not satisfy the installed gate: status=%s fallback_succeeded=%t fallback_accounted=%t peer_succeeded=%t terminal_failure=%t nodes=%#v accounts=%#v",
		lastBoard.Status, fallbackSucceeded, fallbackAccounted, peerSucceeded,
		fallbackTerminalFailure, lastBoard.Nodes, lastBoard.ProviderAccounts,
	)
	return teamBoardWire{}
}

type liveMCPFallbackRoute struct {
	runtimeID         string
	providerID        string
	providerAccountID string
}

func liveMCPRevokeObjectiveForRoute(route liveMCPFallbackRoute) string {
	fallbackEvidence := strings.Join([]string{
		"MCP-FALLBACK-OK",
		"Harness=Loom-Native",
		"Provider=" + route.providerID,
		"ProviderAccount=" + route.providerAccountID,
		"Attempt=2",
		"FallbackApprovalVersion=1",
		"ToolExecution=none",
	}, " ")
	return "Exclusive role rules. " +
		"OpenCode subagent first and only action: call loom_mcp_call exactly once " +
		"with server mcp.live.probe, tool lookup, and arguments " +
		"{\"value\":\"wait-for-revoke\"}; emit no text first, wait for the real result, " +
		"and never answer. Loom Native fallback subagent: no tools; output exactly " +
		fallbackEvidence + ". Loom Native main: no tools; output exactly MCP-HEALTHY-PEER-OK."
}

func requireLiveMCPFallbackRoute(t *testing.T) liveMCPFallbackRoute {
	t.Helper()
	switch os.Getenv("LOOM_LIVE_MCP_FALLBACK_PROVIDER") {
	case "", "minimax":
		return liveMCPFallbackRoute{
			runtimeID:  productMiniMaxAgentRuntimeInstanceID,
			providerID: "minimax", providerAccountID: "minimax.primary",
		}
	case "deepseek":
		return liveMCPFallbackRoute{
			runtimeID:  productNativeAgentRuntimeInstanceID,
			providerID: "deepseek", providerAccountID: "deepseek.primary",
		}
	default:
		t.Fatalf("unsupported explicit fallback Provider")
		return liveMCPFallbackRoute{}
	}
}

func assertLiveMCPPermissionFacts(
	t *testing.T,
	board teamBoardWire,
	requireToolBinding bool,
) {
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
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(`
		SELECT DISTINCT json_extract(loop.payload_json, '$.work_item_id')
		FROM events AS plan
		JOIN json_each(plan.payload_json, '$.nodes') AS node
		JOIN events AS loop
		  ON loop.event_type = 'AttemptLoopStarted'
		 AND json_extract(loop.payload_json, '$.team_instance_id') = ?
		 AND json_extract(loop.payload_json, '$.agent_instance_id') =
		     json_extract(node.value, '$.agent_instance_id')
		 AND json_extract(loop.payload_json, '$.runtime_instance_id') = ?
		WHERE plan.event_type = 'TeamExecutionPlanned'
		  AND json_extract(plan.payload_json, '$.team_instance_id') = ?
		  AND json_extract(node.value, '$.runtime_instance_id') = ?`,
		board.TeamInstanceID, productOpenCodeRuntimeInstanceID,
		board.TeamInstanceID, productOpenCodeRuntimeInstanceID,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var workItemID string
		if err := rows.Scan(&workItemID); err != nil {
			t.Fatal(err)
		}
		if workItemID == "" {
			continue
		}
		var bindings int
		if err := database.QueryRow(
			`SELECT COUNT(*) FROM events WHERE event_type='JobPermissionBound' AND payload_json LIKE ?`,
			`%"job_id":"`+workItemID+`"%`,
		).Scan(&bindings); err != nil {
			t.Fatal(err)
		}
		wantBindings := 0
		if requireToolBinding {
			wantBindings = 1
		}
		if bindings != wantBindings {
			t.Fatalf(
				"OpenCode WorkItem permission bindings=%d want=%d work_item=%s",
				bindings, wantBindings, workItemID,
			)
		}
		found = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("OpenCode MCP node missing from board: %#v", board.Nodes)
	}
}

func assertLiveMCPExecutionFact(
	t *testing.T,
	board teamBoardWire,
	value string,
) {
	t.Helper()
	workItemID := ""
	for _, node := range board.Nodes {
		if node.HarnessAdapter == harnessadapter.OpenCodeAdapterType {
			workItemID = node.WorkItemID
			break
		}
	}
	if workItemID == "" {
		t.Fatalf("OpenCode MCP WorkItem missing from board: %#v", board.Nodes)
	}
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
		t.Fatal(err)
	}
	defer database.Close()
	callDigest := permissions.ProposedCallDigest(permissions.ProposedCall{
		Tool: permissions.ToolMCPTool, Path: "mcp.live.probe/lookup",
		Command: `{"value":"` + value + `"}`,
	})
	contentDigest := sha256.Sum256([]byte(
		"initialized=true env=0 secret= value=" + value,
	))
	outputDigest := "sha256:" + hex.EncodeToString(contentDigest[:])
	var completed int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM events AS proposed
		JOIN events AS completed ON completed.stream_id = proposed.stream_id
		WHERE proposed.event_type = 'ToolExecutionProposed'
		  AND completed.event_type = 'ToolExecutionCompleted'
		  AND json_extract(proposed.payload_json, '$.job_id') = ?
		  AND json_extract(proposed.payload_json, '$.tool') = 'MCPTool'
		  AND json_extract(proposed.payload_json, '$.call_digest') = ?
		  AND json_extract(completed.payload_json, '$.output_digest') = ?`,
		workItemID, callDigest, outputDigest,
	).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != 1 {
		t.Fatalf(
			"exact governed MCP completions=%d work_item=%s call=%s output=%s",
			completed, workItemID, callDigest, outputDigest,
		)
	}
}

func liveMCPEnrollmentRevision(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	id string,
) int64 {
	t.Helper()
	for _, enrollment := range remoteToolEnrollments(
		t, ctx, client, "deepseek", "deepseek.primary",
	) {
		if enrollment.EnrollmentID == id && enrollment.Status == "active" {
			return enrollment.Revision
		}
	}
	t.Fatalf("active MCP Enrollment %s not found", id)
	return 0
}

func requireLiveMCPMarker(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" || !filepath.IsAbs(value) {
		t.Fatalf("%s must be an absolute path", name)
	}
	return value
}

func waitForLiveMCPMarkerPath(
	ctx context.Context,
	path string,
) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.New("MCP call did not reach the installed stdio server")
}
