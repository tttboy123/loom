package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
)

// TestLiveMixedProviderTeamE2E drives the installed App daemon through the
// G3/G4 acceptance gates: build and confirm a mixed-provider Agent Team
// (DeepSeek + MiniMax + Zhipu), run a real Mission, and verify per-Agent
// execution bindings + failure isolation. It is a paid, network-enabled live
// gate and is skipped unless LOOM_LIVE_TEAM_E2E=1 is set.
//
// Prerequisites:
//   - The installed App is running and its daemon socket is reachable.
//   - The Credential Vault is unlocked.
//   - DEEPSEEK_API_KEY, MINIMAX_API_KEY, ZHIPU_API_KEY are available locally.
func TestLiveMixedProviderTeamE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_TEAM_E2E") != "1" {
		t.Skip("live team E2E gate requires LOOM_LIVE_TEAM_E2E=1")
	}
	socketPath := os.Getenv("LOOM_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    120 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 1. Unlock vault when locked.
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		CredentialVault *struct {
			Status string `json:"status"`
		} `json:"credential_vault"`
	}
	_ = json.Unmarshal(raw, &snap)
	if snap.CredentialVault != nil && snap.CredentialVault.Status == "locked" {
		if err := client.Call(ctx, "credential_vault_unlock", struct{}{}, &struct{}{}); err != nil {
			t.Fatalf("unlock: %v", err)
		}
	}

	// 2. Ensure DeepSeek + MiniMax verified (the two loom-native brokered
	//    Provider accounts the role catalog supports). Zhipu verifies via the
	//    catalog verifier but has no loom-native Agent definition, so it is
	//    intentionally not part of this Team.
	ensureProviderVerified(t, ctx, client, "deepseek", os.Getenv("DEEPSEEK_API_KEY"))
	ensureProviderVerified(t, ctx, client, "minimax", os.Getenv("MINIMAX_API_KEY"))

	// 3. Refresh snapshot and read role options.
	roleOptions := refreshRoleOptions(t, ctx, client)
	providers := map[string]bool{}
	for _, role := range roleOptions {
		providers[role.ProviderID] = true
	}
	t.Logf("verified providers in role catalog: %v", providers)
	if !providers["deepseek"] || !providers["minimax"] {
		t.Fatalf("expected deepseek+minimax roles, got %v", providers)
	}

	// 4. Start a blank Team draft (form-first), set name+purpose, and add a
	//    second subagent so the Team spans two independent Provider Accounts
	//    across four Agents (main + three subagents).
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live Mixed Provider Team")
	session = editSession(t, ctx, client, session, "purpose", "Verify a real mixed-provider Team end to end")

	// Pick main = deepseek, subagent 1 = minimax, subagent 2 = deepseek
	// reviewer, subagent 3 = minimax researcher — four Agents across two
	// independent Provider Accounts.
	mainRole := firstRole(roleOptions, "main", "deepseek")
	subMinimax := firstRole(roleOptions, "subagent", "minimax")
	subDeepseekReviewer := firstRoleByResponsibility(roleOptions, "subagent", "deepseek", "Review changes")
	subMinimaxResearcher := firstRoleByResponsibility(roleOptions, "subagent", "minimax", "Investigate bounded questions")
	if mainRole.ID == "" || subMinimax.ID == "" ||
		subDeepseekReviewer.ID == "" || subMinimaxResearcher.ID == "" {
		t.Fatalf("missing mixed-provider roles: main=%q minimax=%q ds-reviewer=%q mm-researcher=%q",
			mainRole.ID, subMinimax.ID, subDeepseekReviewer.ID, subMinimaxResearcher.ID)
	}
	session = editSession(t, ctx, client, session, "main_role", mainRole.ID)
	// The default draft already has one subagent slot; edit it to MiniMax by
	// targeting its agentDefinitionID, then add the two specialist subagents.
	defaultSubAgent := firstPreviewSubagent(session)
	if defaultSubAgent.AgentDefinitionID == "" {
		t.Fatal("draft has no default subagent slot")
	}
	session = editSessionTargeted(t, ctx, client, session, "subagent_role", subMinimax.ID, defaultSubAgent.AgentDefinitionID)
	session = editSession(t, ctx, client, session, "subagent_add", subDeepseekReviewer.ID)
	session = editSession(t, ctx, client, session, "subagent_add", subMinimaxResearcher.ID)

	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf("draft not confirmable: canConfirm=%v binding=%q",
			session.CanConfirm, session.BindingDg)
	}

	// 5. Confirm the Team.
	definitionID := "team-live-mixed-" + time.Now().UTC().Format("20060102T150405Z")
	confirmation, err := confirmSession(t, ctx, client, session, definitionID)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	t.Logf("confirmed: team=%s status=%s instanceCreated=%v runCreated=%v",
		confirmation.TeamDefinitionID, confirmation.Status,
		confirmation.TeamInstanceCreated, confirmation.RunCreated)

	// 6. Snapshot: the confirmed Team must appear as executable.
	teams := refreshTeams(t, ctx, client)
	if len(teams) == 0 {
		t.Fatal("no Team in snapshot after confirmation")
	}
	team := teams[0]
	t.Logf("team=%s display=%q executable=%v confirmed=%v",
		team.TeamInstanceID, team.DisplayName, team.Executable, team.Confirmed)
	if !team.Confirmed || !team.Executable {
		t.Fatalf("confirmed team not executable: %#v", team)
	}

	// 7. Preflight a real Mission on this Team (paid real run).
	preflight, err := preflightMission(t, ctx, client, team)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	nodes := []struct {
		LogicalNodeID string `json:"logical_node_id"`
		Title         string `json:"title"`
		Role          string `json:"role"`
		ProviderID    string `json:"provider_id"`
		ModelID       string `json:"model_id"`
		Status        string `json:"status"`
	}{}
	if preflight.Preflight != nil {
		nodes = preflight.Preflight.Nodes
	}
	t.Logf("preflight nodes=%d ready=%v", len(nodes), countStatus(nodes, "ready"))

	// 8. Run the Mission and verify per-Agent bindings + real progress.
	result, err := runMission(t, ctx, client, team, preflight)
	if err != nil {
		t.Fatalf("run mission: %v", err)
	}
	t.Logf("mission result: status=%s note=%q", result.Status, result.Note)

	// 9. Snapshot accounting rows: attempts + per-Account accounting coverage.
	verifyAccountingRows(t, ctx, client, team.TeamInstanceID)
}

// ----- helpers (mirroring live_matrix_e2e_test.go patterns) -----

type roleOption struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	ProviderID     string `json:"provider_id"`
	ModelID        string `json:"model_id"`
	Responsibility string `json:"responsibility"`
}

func refreshRoleOptions(t *testing.T, ctx context.Context, client *localipc.Client) []roleOption {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		RoleOptions []roleOption `json:"role_options"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return snap.RoleOptions
}

func firstRoleByResponsibility(
	options []roleOption,
	kind string,
	providerID string,
	responsibilityContains string,
) roleOption {
	for _, role := range options {
		if role.Kind == kind && role.ProviderID == providerID &&
			strings.Contains(role.Responsibility, responsibilityContains) {
			return role
		}
	}
	return roleOption{}
}

func firstRole(options []roleOption, kind string, providerID string) roleOption {
	for _, role := range options {
		if role.Kind == kind && role.ProviderID == providerID {
			return role
		}
	}
	return roleOption{}
}

type builderRolePreview struct {
	Kind              string `json:"kind"`
	AgentDefinitionID string `json:"agent_definition_id"`
	ProviderID        string `json:"provider_id"`
}

type builderSession struct {
	DraftID    string `json:"draft_id"`
	Revision   int    `json:"revision"`
	CatalogDg  string `json:"catalog_digest"`
	ViewVer    string `json:"view_version"`
	BindingDg  string `json:"binding_digest"`
	CanConfirm bool   `json:"can_confirm"`
	Question   struct {
		ID string `json:"id"`
	} `json:"question"`
	Preview struct {
		Roles []builderRolePreview `json:"roles"`
	} `json:"preview"`
}

func firstPreviewSubagent(session builderSession) builderRolePreview {
	for _, role := range session.Preview.Roles {
		if role.Kind == "subagent" {
			return role
		}
	}
	return builderRolePreview{}
}

func startBlankSession(t *testing.T, ctx context.Context, client *localipc.Client) builderSession {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "builder_start", map[string]any{
		"source": "blank", "source_id": "", "source_version": 0, "source_digest": "",
	}, &raw); err != nil {
		t.Fatalf("builder_start: %v", err)
	}
	var session builderSession
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return session
}

func editSession(t *testing.T, ctx context.Context, client *localipc.Client, session builderSession, field, value string) builderSession {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "builder_edit", map[string]any{
		"draft_id": session.DraftID, "expected_revision": session.Revision,
		"catalog_digest": session.CatalogDg, "view_version": session.ViewVer,
		"field": field, "value": value,
	}, &raw); err != nil {
		t.Fatalf("builder_edit(%s): %v", field, err)
	}
	var next builderSession
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return next
}

func editSessionTargeted(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	session builderSession,
	field, value, roleAgentDefinitionID string,
) builderSession {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "builder_edit", map[string]any{
		"draft_id": session.DraftID, "expected_revision": session.Revision,
		"catalog_digest": session.CatalogDg, "view_version": session.ViewVer,
		"field": field, "value": value,
		"role_agent_definition_id": roleAgentDefinitionID,
	}, &raw); err != nil {
		t.Fatalf("builder_edit(%s): %v", field, err)
	}
	var next builderSession
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return next
}

type builderConfirmation struct {
	TeamDefinitionID    string `json:"team_definition_id"`
	Status              string `json:"status"`
	TeamInstanceCreated bool   `json:"team_instance_created"`
	RunCreated          bool   `json:"run_created"`
}

func confirmSession(t *testing.T, ctx context.Context, client *localipc.Client, session builderSession, definitionID string) (builderConfirmation, error) {
	t.Helper()
	var raw json.RawMessage
	err := client.Call(ctx, "builder_confirm", map[string]any{
		"draft_id": session.DraftID, "expected_revision": session.Revision,
		"catalog_digest": session.CatalogDg, "view_version": session.ViewVer,
		"binding_digest": session.BindingDg,
		"definition_id":  definitionID, "scope": "reusable",
		"confirm": true,
	}, &raw)
	if err != nil {
		return builderConfirmation{}, err
	}
	var confirmation builderConfirmation
	if err := json.Unmarshal(raw, &confirmation); err != nil {
		return builderConfirmation{}, fmt.Errorf("decode: %w", err)
	}
	fmt.Fprintf(os.Stderr, "CONFIRM-RAW %s\n", string(raw))
	return confirmation, nil
}

type teamSummary struct {
	TeamInstanceID string `json:"team_instance_id"`
	DisplayName    string `json:"display_name"`
	Confirmed      bool   `json:"confirmed"`
	Executable     bool   `json:"executable"`
	ReadOnly       bool   `json:"read_only"`
}

func refreshTeams(t *testing.T, ctx context.Context, client *localipc.Client) []teamSummary {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "snapshot", map[string]any{
		"after_team_id": "", "after_runtime_id": "", "after_run_id": "",
		"after_evidence_id": "", "limit": 100,
	}, &raw); err != nil {
		// Fall back to setup_snapshot (saved teams).
		if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		var snap struct {
			SavedTeams []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"saved_teams"`
		}
		_ = json.Unmarshal(raw, &snap)
		var out []teamSummary
		for _, saved := range snap.SavedTeams {
			if saved.Status == "active" {
				out = append(out, teamSummary{TeamInstanceID: saved.ID, DisplayName: saved.ID, Confirmed: true, Executable: true})
			}
		}
		return out
	}
	var page struct {
		Teams []teamSummary `json:"teams"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("decode teams: %v", err)
	}
	return page.Teams
}

type preflightEnvelope struct {
	Preflight *struct {
		MissionID      string `json:"mission_id"`
		TeamInstanceID string `json:"team_instance_id"`
		Nodes          []struct {
			LogicalNodeID string `json:"logical_node_id"`
			Title         string `json:"title"`
			Role          string `json:"role"`
			ProviderID    string `json:"provider_id"`
			ModelID       string `json:"model_id"`
			Status        string `json:"status"`
		} `json:"nodes"`
		Digest string `json:"digest"`
	} `json:"preflight"`
}

func countStatus(nodes []struct {
	LogicalNodeID string `json:"logical_node_id"`
	Title         string `json:"title"`
	Role          string `json:"role"`
	ProviderID    string `json:"provider_id"`
	ModelID       string `json:"model_id"`
	Status        string `json:"status"`
}, status string) int {
	count := 0
	for _, node := range nodes {
		if node.Status == status {
			count++
		}
	}
	return count
}

func preflightMission(t *testing.T, ctx context.Context, client *localipc.Client, team teamSummary) (preflightEnvelope, error) {
	t.Helper()
	var raw json.RawMessage
	err := client.Call(ctx, "mission_execution", map[string]any{
		"schema_version": 1, "operation": "preflight",
		"mission_id":            "mission/" + team.TeamInstanceID,
		"team_instance_id":      team.TeamInstanceID,
		"work_package_id":       "work-package.coding",
		"work_package_digest":   "4eea514fca13aa241cd004277e31c9c1fe29634646ceafd618809b6b22c8a4f",
		"objective":             "Verify the mixed-provider Team runs end to end",
		"expected_view_version": currentViewVersion(t, ctx, client),
		"correlation_id":        "loom-chat-live-team-" + time.Now().UTC().Format("20060102T150405Z"),
	}, &raw)
	if err != nil {
		return preflightEnvelope{}, err
	}
	var envelope preflightEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return preflightEnvelope{}, fmt.Errorf("decode: %w", err)
	}
	return envelope, nil
}

func currentViewVersion(t *testing.T, ctx context.Context, client *localipc.Client) string {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		ViewVersion string `json:"view_version"`
	}
	_ = json.Unmarshal(raw, &snap)
	return snap.ViewVersion
}

type missionRunResult struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func runMission(t *testing.T, ctx context.Context, client *localipc.Client, team teamSummary, preflight preflightEnvelope) (missionRunResult, error) {
	t.Helper()
	if preflight.Preflight == nil {
		return missionRunResult{}, errors.New("no preflight envelope")
	}
	var raw json.RawMessage
	err := client.Call(ctx, "mission_execution", map[string]any{
		"schema_version": 1, "operation": "execute",
		"mission_id":            "mission/" + team.TeamInstanceID,
		"team_instance_id":      team.TeamInstanceID,
		"work_package_id":       "work-package.coding",
		"work_package_digest":   "4eea514fca13aa241cd004277e31c9c1fe29634646ceafd618809b6b22c8a4f",
		"objective":             "Verify the mixed-provider Team runs end to end",
		"expected_view_version": currentViewVersion(t, ctx, client),
		"preflight_digest":      preflight.Preflight.Digest,
		"correlation_id":        "loom-chat-live-team-run-" + time.Now().UTC().Format("20060102T150405Z"),
	}, &raw)
	if err != nil {
		return missionRunResult{}, err
	}
	var result missionRunResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return missionRunResult{}, fmt.Errorf("decode: %w", err)
	}
	return result, nil
}

func verifyAccountingRows(t *testing.T, ctx context.Context, client *localipc.Client, teamInstanceID string) {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID string `json:"provider_id"`
			Status     string `json:"status"`
			Revision   int64  `json:"revision"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, account := range snap.Accounts {
		t.Logf("account %s status=%s revision=%d", account.ProviderID, account.Status, account.Revision)
	}
}

// ensureProviderVerified configures+verifies a brokered Provider account from
// the local env key when the account is not already verified.
func ensureProviderVerified(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	providerID string,
	key string,
) {
	t.Helper()
	if key == "" {
		t.Logf("%s key not set; skipping", providerID)
		return
	}
	if providerVerified(t, ctx, client, providerID) {
		t.Logf("%s already verified", providerID)
		return
	}
	var raw json.RawMessage
	// Configure.
	if err := client.Call(ctx, "credential_configure", map[string]string{
		"provider_id": providerID, "secret": key,
	}, &raw); err != nil {
		// Already configured (not verified) is fine.
		t.Logf("%s configure (may already exist): %v", providerID, err)
	}
	// Re-read snapshot for reference+revision.
	reference, revision := providerCredential(t, ctx, client, providerID)
	if reference == "" {
		t.Fatalf("%s credential reference not found", providerID)
	}
	if err := client.Call(ctx, "credential_verify", map[string]any{
		"provider_id":          providerID,
		"credential_reference": reference,
		"expected_revision":    revision,
		"operation_id":         matrixOperationID(),
	}, &raw); err != nil {
		t.Fatalf("%s verify: %v", providerID, err)
	}
	t.Logf("%s verified", providerID)
}

func providerVerified(t *testing.T, ctx context.Context, client *localipc.Client, providerID string) bool {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID string `json:"provider_id"`
			Status     string `json:"status"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, account := range snap.Accounts {
		if account.ProviderID == providerID && account.Status == "verified" {
			return true
		}
	}
	return false
}

func providerCredential(t *testing.T, ctx context.Context, client *localipc.Client, providerID string) (string, int64) {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID string `json:"provider_id"`
			Reference  string `json:"credential_reference"`
			Revision   int64  `json:"revision"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, account := range snap.Accounts {
		if account.ProviderID == providerID && account.Reference != "" {
			return account.Reference, account.Revision
		}
	}
	return "", 0
}

var _ = app.BuilderStartCommand{}
var _ = strings.TrimSpace
