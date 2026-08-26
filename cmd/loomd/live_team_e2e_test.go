package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/mode"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"
)

const liveMissionE2EObjective = "Complete this mixed-provider Team acceptance run now. Return a concise role-specific execution report containing the exact token LOOM-MIXED-TEAM-OK, your Provider, and your assigned role title. Do not ask for another instruction."

const liveFourProviderMissionE2EObjective = "Complete this four-Harness Team acceptance run now. Use only the supplied context and return a concise role-specific report containing the exact token LOOM-FOUR-PROVIDER-TEAM-OK, your Harness, Provider Account, Model, credential revision, and assigned role title. Do not ask for another instruction."

const liveOpenCodeMissionE2EObjective = "Complete this two-Agent acceptance now. Use only the context already supplied; do not call tools or request more context. Return a concise role-specific report containing the exact token LOOM-OPENCODE-TEAM-OK, your Harness, Provider, assigned role title, and the frozen execution binding as evidence. Do not ask for another instruction."

const liveOpenCodeNativeMissionE2EObjective = "Complete this OpenCode hosted two-Agent acceptance now. Use only the context already supplied; do not call tools or request more context. Return a concise role-specific report containing the exact token LOOM-OPENCODE-NATIVE-TEAM-OK, your Harness, Provider, assigned role title, and native auth mode. Do not ask for another instruction."

// TestLiveMixedProviderTeamE2E drives the installed App daemon through the
// G3/G4 acceptance gates: build and confirm a mixed-provider Agent Team
// (DeepSeek + MiniMax), run a real Mission through its explicit review gates,
// and verify per-Agent execution bindings + account-local accounting. It is a
// paid, network-enabled live gate and is skipped unless
// LOOM_LIVE_TEAM_E2E=1 is set.
//
// Prerequisites:
//   - The installed App is running and its daemon socket is reachable.
//   - The Credential Vault is unlocked.
//   - DeepSeek and MiniMax are already verified in the Vault, or their API keys
//     are supplied through the explicitly enabled live-test environment.
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
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
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

	// 3b. Archive any leftover live-mixed Teams first so the draft's captured
	// view version and catalog digest stay stable through confirm.
	archiveLiveTeams(t, ctx, client)

	// 3c. Configure Provider Account Policies + Model Rate Cards so the G6
	// accounting board carries exact policy revision, concurrency/budget
	// ceilings, token usage and both provider-reported and rate-card cost.
	configureProviderGovernance(t, ctx, client, "deepseek", "deepseek.primary", "deepseek-chat")
	configureProviderGovernance(t, ctx, client, "minimax", "minimax.primary", "MiniMax-M3")

	// 4. Build + confirm the 4-Agent mixed-provider Team (DeepSeek main +
	//    MiniMax bounded worker + DeepSeek reviewer + MiniMax researcher).
	team := confirmMixedProviderTeam(t, ctx, client, roleOptions, "Live Mixed Provider Team")

	// 7. Preflight a real Mission on this Team (paid real run).
	preflight, err := preflightMission(t, ctx, client, team, liveMissionE2EObjective)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	nodes := preflight.Preflight.Nodes
	ready := countStatus(nodes, "ready")
	t.Logf("preflight nodes=%d ready=%v", len(nodes), ready)
	if len(nodes) != 4 || ready != 4 {
		t.Fatalf("expected 4/4 ready nodes for the mixed-provider Team, got %d/%d: %#v",
			ready, len(nodes), nodes)
	}

	// 8. Run the Mission and verify per-Agent bindings + real progress.
	result, err := runMission(t, ctx, client, team, preflight)
	if err != nil {
		t.Fatalf("run mission: %v", err)
	}
	t.Logf("mission result: status=%s note=%q", result.Status, result.Note)
	switch result.Status {
	case "running", "awaiting_recovery", "blocked", "succeeded", "failed",
		"degraded", "human_required", "cancelled":
	default:
		t.Fatalf("mission start returned unrecognized status %q", result.Status)
	}

	// 9. The test harness is the external governance actor for this disposable
	//    Team: it reads each prepared review gate, explicitly accepts the result,
	//    and waits until the dependency graph reaches an authoritative terminal
	//    state. The executing Agents never accept their own output.
	board := driveMixedProviderMissionToCompletion(
		t, ctx, client, team.TeamInstanceID,
	)
	verifyMixedProviderNodeRows(t, board)

	// 10. Snapshot accounting rows: attempts + per-Account accounting coverage.
	verifyAccountingRows(t, ctx, client, team.TeamInstanceID)
	verifyMixedProviderAccountingCompletion(t, board)
}

// TestLiveFourProviderFourHarnessTeamE2E is the installed G3 exit gate. It
// proves that Codex/OpenAI, Claude Code/Anthropic, Loom Native/Kimi and Loom
// Native/MiniMax can be frozen and executed independently in one Team. The
// separate flag prevents the ordinary two-Provider live gate from silently
// claiming this broader credential and Harness matrix.
func TestLiveFourProviderFourHarnessTeamE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_FOUR_PROVIDER_TEAM_E2E") != "1" {
		t.Skip("four-Provider Team gate requires LOOM_LIVE_FOUR_PROVIDER_TEAM_E2E=1")
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snapshot struct {
		CredentialVault *struct {
			Status string `json:"status"`
		} `json:"credential_vault"`
	}
	_ = json.Unmarshal(raw, &snapshot)
	if snapshot.CredentialVault != nil && snapshot.CredentialVault.Status == "locked" {
		if err := client.Call(ctx, "credential_vault_unlock", struct{}{}, &struct{}{}); err != nil {
			t.Fatalf("unlock: %v", err)
		}
	}

	for _, prerequisite := range []struct {
		providerID string
		secret     string
	}{
		{"openai", os.Getenv("OPENAI_API_KEY")},
		{"anthropic", os.Getenv("ANTHROPIC_API_KEY")},
		{"kimi", os.Getenv("KIMI_API_KEY")},
		{"minimax", os.Getenv("MINIMAX_API_KEY")},
	} {
		ensureProviderVerified(t, ctx, client, prerequisite.providerID, prerequisite.secret)
	}

	roleOptions := refreshRoleOptions(t, ctx, client)
	selected := []roleOption{
		firstRoleForRuntime(roleOptions, "main", productCodexRuntimeInstanceID, "openai"),
		firstRoleForRuntime(roleOptions, "subagent", productClaudeCodeRuntimeInstanceID, "anthropic"),
		firstRoleForRuntime(roleOptions, "subagent", productKimiAgentRuntimeInstanceID, "kimi"),
		firstRoleForRuntime(roleOptions, "subagent", productMiniMaxAgentRuntimeInstanceID, "minimax"),
	}
	for index, role := range selected {
		if role.ID == "" || role.HarnessAdapter == "" || role.ProviderAccountID == "" ||
			role.ModelID == "" || role.AuthMode == "" || role.CredentialRevision <= 0 {
			t.Fatalf("four-Provider role %d is not independently brokered: %#v", index, role)
		}
		configureProviderGovernance(
			t, ctx, client, role.ProviderID, role.ProviderAccountID, role.ModelID,
		)
	}

	archiveLiveTeams(t, ctx, client)
	team := confirmFourProviderTeam(t, ctx, client, selected)
	preflight, err := preflightMission(t, ctx, client, team, liveFourProviderMissionE2EObjective)
	if err != nil {
		t.Fatalf("four-Provider preflight: %v", err)
	}
	if ready := countStatus(preflight.Preflight.Nodes, "ready"); len(preflight.Preflight.Nodes) != 4 || ready != 4 {
		t.Fatalf("expected 4/4 four-Provider nodes ready, got %d/%d: %#v",
			ready, len(preflight.Preflight.Nodes), preflight.Preflight.Nodes)
	}
	if _, err := runMission(t, ctx, client, team, preflight); err != nil {
		t.Fatalf("four-Provider mission: %v", err)
	}
	board := driveMixedProviderMissionToCompletion(t, ctx, client, team.TeamInstanceID)
	verifyFourProviderNodeAndAccountRows(t, board, selected)
}

// TestLiveOpenCodeAgentTeamE2E proves that OpenCode is an executable Team
// Harness, not only a conversation profile. The OpenCode Agent freezes its own
// brokered Provider Account, credential revision and model independently from
// the Loom Native peer using the same Provider.
func TestLiveOpenCodeAgentTeamE2E(t *testing.T) {
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

	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snapshot struct {
		CredentialVault *struct {
			Status string `json:"status"`
		} `json:"credential_vault"`
	}
	_ = json.Unmarshal(raw, &snapshot)
	if snapshot.CredentialVault != nil && snapshot.CredentialVault.Status == "locked" {
		if err := client.Call(ctx, "credential_vault_unlock", struct{}{}, &struct{}{}); err != nil {
			t.Fatalf("unlock: %v", err)
		}
	}

	ensureProviderVerified(t, ctx, client, "deepseek", os.Getenv("DEEPSEEK_API_KEY"))
	configureProviderGovernance(t, ctx, client, "deepseek", "deepseek.primary", "deepseek-chat")
	archiveLiveTeams(t, ctx, client)
	roleOptions := refreshRoleOptions(t, ctx, client)
	if role := firstRoleForRuntime(
		roleOptions, "subagent", productOpenCodeRuntimeInstanceID, "deepseek",
	); role.ID == "" {
		t.Fatalf("OpenCode Agent role missing from live catalog: %#v", roleOptions)
	}

	team := confirmOpenCodeAgentTeam(t, ctx, client, roleOptions)
	preflight, err := preflightMission(t, ctx, client, team, liveMissionE2EObjective)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if preflight.Preflight == nil || len(preflight.Preflight.Nodes) != 2 ||
		countStatus(preflight.Preflight.Nodes, "ready") != 2 {
		t.Fatalf("expected a 2/2 ready DeepSeek + OpenCode Team: %#v", preflight.Preflight)
	}
	if _, err := runMissionWithObjective(
		t, ctx, client, team, preflight, liveOpenCodeMissionE2EObjective,
	); err != nil {
		t.Fatalf("run mission: %v", err)
	}
	board := driveMixedProviderMissionToCompletion(t, ctx, client, team.TeamInstanceID)
	verifyOpenCodeAgentNodeRows(t, board)
}

// TestLiveOpenCodeNativeAgentTeamE2E proves that an installed OpenCode hosted
// account can execute a governed Agent Team without borrowing a Loom Vault
// credential or being mislabeled as DeepSeek/OpenAI brokered auth.
func TestLiveOpenCodeNativeAgentTeamE2E(t *testing.T) {
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

	archiveLiveTeams(t, ctx, client)
	roleOptions := refreshRoleOptions(t, ctx, client)
	if role := firstRoleForRuntime(
		roleOptions, "main", productOpenCodeRuntimeInstanceID, "opencode",
	); role.ID == "" {
		t.Fatalf("OpenCode native main role missing from live catalog: %#v", roleOptions)
	}
	if role := firstRoleForRuntime(
		roleOptions, "subagent", productOpenCodeRuntimeInstanceID, "opencode",
	); role.ID == "" {
		t.Fatalf("OpenCode native subagent role missing from live catalog: %#v", roleOptions)
	}

	team := confirmOpenCodeNativeAgentTeam(t, ctx, client, roleOptions)
	preflight, err := preflightMission(
		t, ctx, client, team, liveOpenCodeNativeMissionE2EObjective,
	)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if preflight.Preflight == nil || len(preflight.Preflight.Nodes) != 2 ||
		countStatus(preflight.Preflight.Nodes, "ready") != 2 {
		t.Fatalf("expected a 2/2 ready native OpenCode Team: %#v", preflight.Preflight)
	}
	if _, err := runMissionWithObjective(
		t, ctx, client, team, preflight, liveOpenCodeNativeMissionE2EObjective,
	); err != nil {
		t.Fatalf("run mission: %v", err)
	}
	board := driveMixedProviderMissionToCompletion(
		t, ctx, client, team.TeamInstanceID,
	)
	verifyOpenCodeNativeAgentNodeRows(t, board)
}

// TestLiveNewAttemptE2E verifies that a fresh, executable Team can be run a
// second time through the explicit new_attempt path. This is intentionally a
// separate gate from the historical Mission board: archived Team Definitions
// must remain read-only and must never be used as retry fixtures.
func TestLiveNewAttemptE2E(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	ensureProviderVerified(t, ctx, client, "deepseek", os.Getenv("DEEPSEEK_API_KEY"))
	ensureProviderVerified(t, ctx, client, "minimax", os.Getenv("MINIMAX_API_KEY"))
	configureProviderGovernance(t, ctx, client, "deepseek", "deepseek.primary", "deepseek-chat")
	configureProviderGovernance(t, ctx, client, "minimax", "minimax.primary", "MiniMax-M3")
	archiveLiveTeams(t, ctx, client)
	team := confirmMixedProviderTeam(t, ctx, client, refreshRoleOptions(t, ctx, client), "Live New Attempt Team")

	firstPreflight, err := preflightMission(t, ctx, client, team, liveMissionE2EObjective)
	if err != nil {
		t.Fatalf("first preflight: %v", err)
	}
	if ready := countStatus(firstPreflight.Preflight.Nodes, "ready"); ready != len(firstPreflight.Preflight.Nodes) {
		t.Fatalf("first preflight has blocked nodes: ready=%d nodes=%d", ready, len(firstPreflight.Preflight.Nodes))
	}
	first, err := runMission(t, ctx, client, team, firstPreflight)
	if err != nil {
		t.Fatalf("first mission: %v", err)
	}
	t.Logf("first mission: status=%s note=%q", first.Status, first.Note)
	waitForMissionSettled(t, ctx, client, team.TeamInstanceID)

	secondPreflight, err := preflightMissionWithAttempt(t, ctx, client, team, liveMissionE2EObjective, true)
	if err != nil {
		t.Fatalf("new-attempt preflight: %v", err)
	}
	if ready := countStatus(secondPreflight.Preflight.Nodes, "ready"); ready != len(secondPreflight.Preflight.Nodes) {
		t.Fatalf("new-attempt preflight has blocked nodes: ready=%d nodes=%d", ready, len(secondPreflight.Preflight.Nodes))
	}
	second, err := runMissionWithAttempt(t, ctx, client, team, secondPreflight, true)
	if err != nil {
		t.Fatalf("new-attempt mission: %v", err)
	}
	t.Logf("new-attempt mission: status=%s note=%q", second.Status, second.Note)
	switch second.Status {
	case "running", "awaiting_recovery", "blocked", "succeeded", "failed", "degraded", "human_required", "cancelled":
	default:
		t.Fatalf("new-attempt start returned unrecognized status %q", second.Status)
	}
}

// ----- helpers (mirroring live_matrix_e2e_test.go patterns) -----

type roleOption struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind"`
	HarnessAdapter     string `json:"harness_adapter"`
	ProviderID         string `json:"provider_id"`
	ProviderAccountID  string `json:"provider_account_id"`
	ModelID            string `json:"model_id"`
	AuthMode           string `json:"auth_mode"`
	CredentialRevision int64  `json:"credential_revision"`
	RuntimeID          string `json:"runtime_instance_id"`
	Responsibility     string `json:"responsibility"`
}

func refreshRoleOptions(t *testing.T, ctx context.Context, client *localipc.Client) []roleOption {
	t.Helper()
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
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

func firstRoleForRuntime(
	options []roleOption,
	kind string,
	runtimeID string,
	providerID string,
) roleOption {
	for _, role := range options {
		if role.Kind == kind && role.RuntimeID == runtimeID &&
			role.ProviderID == providerID {
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
		Roles             []builderRolePreview `json:"roles"`
		CompatibilityGaps []string             `json:"compatibility_gaps"`
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
	err := client.Call(ctx, "builder_confirm", liveBuilderConfirmParams(session, definitionID), &raw)
	if err != nil {
		return builderConfirmation{}, err
	}
	var confirmation builderConfirmation
	if err := json.Unmarshal(raw, &confirmation); err != nil {
		return builderConfirmation{}, fmt.Errorf("decode: %w", err)
	}
	return confirmation, nil
}

func liveBuilderConfirmParams(session builderSession, definitionID string) map[string]any {
	return map[string]any{
		"draft_id": session.DraftID, "expected_revision": session.Revision,
		"catalog_digest": session.CatalogDg, "view_version": session.ViewVer,
		"binding_digest": session.BindingDg,
		"definition_id":  definitionID, "scope": "reusable",
		"project_id": "", "confirm": true,
	}
}

func archiveLiveTeams(t *testing.T, ctx context.Context, client *localipc.Client, definitionIDs ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, id := range definitionIDs {
		want[id] = true
	}
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Logf("archive setup_snapshot: %v", err)
		return
	}
	var snap struct {
		SavedTeams []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			StreamHead int64  `json:"stream_head"`
		} `json:"saved_teams"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, saved := range snap.SavedTeams {
		if saved.Status != "active" || saved.StreamHead <= 0 ||
			!strings.HasPrefix(saved.ID, "team-live-mixed-") {
			continue
		}
		if len(want) > 0 && !want[saved.ID] {
			continue
		}
		if err := client.Call(ctx, "team_archive", map[string]any{
			"definition_id": saved.ID, "expected_head": saved.StreamHead,
		}, &struct{}{}); err != nil {
			t.Logf("archive team %s: %v", saved.ID, err)
		}
	}
	// Archiving is journal-authoritative but the Builder reads the projected
	// view. Wait for the projection to observe the archive before the next live
	// case starts, otherwise a valid subsequent edit can race an old view
	// version and create a false test failure.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		currentRaw, err := setupSnapshotEventually(t, ctx, client)
		if err != nil {
			return
		}
		var current struct {
			SavedTeams []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"saved_teams"`
		}
		if json.Unmarshal(currentRaw, &current) != nil {
			return
		}
		active := false
		for _, saved := range current.SavedTeams {
			if saved.Status != "active" || !strings.HasPrefix(saved.ID, "team-live-mixed-") {
				continue
			}
			if len(want) == 0 || want[saved.ID] {
				active = true
				break
			}
		}
		if !active {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Log("live Team archive projection did not settle before timeout")
}

// setupSnapshotEventually covers the bounded local-daemon route rebind window
// after a governance or projection write. It retries transport availability
// only; a structured remote business error is returned immediately.
func setupSnapshotEventually(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) (json.RawMessage, error) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var raw json.RawMessage
		err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		var remote *localipc.RemoteError
		recoverableStateUnavailable := errors.As(err, &remote) &&
			remote.Code == "state_unavailable" && remote.Recoverable
		if !errors.Is(err, localipc.ErrLocalProductUnavailable) &&
			!errors.Is(err, localipc.ErrProtocolTimeout) &&
			!recoverableStateUnavailable {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return nil, lastErr
}

type teamSummary struct {
	TeamInstanceID string `json:"team_instance_id"`
	DisplayName    string `json:"display_name"`
	SourceKind     string `json:"source_kind"`
	Confirmed      bool   `json:"confirmed"`
	Executable     bool   `json:"executable"`
	ReadOnly       bool   `json:"read_only"`
}

func refreshTeams(t *testing.T, ctx context.Context, client *localipc.Client) []teamSummary {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "snapshot", map[string]any{
		"after_team_id": "", "after_runtime_id": "", "after_run_id": "",
		"after_evidence_id": "", "limit": 64,
	}, &raw); err != nil {
		// Fall back to setup_snapshot (saved teams).
		if snapshot, err := setupSnapshotEventually(t, ctx, client); err != nil {
			t.Fatalf("snapshot: %v", err)
		} else {
			raw = snapshot
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
			LogicalNodeID             string `json:"logical_node_id"`
			Title                     string `json:"title"`
			Role                      string `json:"role"`
			ProviderID                string `json:"provider_id"`
			ModelID                   string `json:"model_id"`
			Status                    string `json:"status"`
			BlockReason               string `json:"block_reason"`
			FallbackConfigured        bool   `json:"fallback_configured"`
			FallbackApprovalRequired  bool   `json:"fallback_approval_required"`
			FallbackApprovalAvailable bool   `json:"fallback_approval_available"`
			FallbackApprovalVersion   int    `json:"fallback_approval_version"`
		} `json:"nodes"`
		PreflightDigest string `json:"preflight_digest"`
		ViewVersion     string `json:"view_version"`
	} `json:"preflight"`
}

func countStatus(nodes []struct {
	LogicalNodeID             string `json:"logical_node_id"`
	Title                     string `json:"title"`
	Role                      string `json:"role"`
	ProviderID                string `json:"provider_id"`
	ModelID                   string `json:"model_id"`
	Status                    string `json:"status"`
	BlockReason               string `json:"block_reason"`
	FallbackConfigured        bool   `json:"fallback_configured"`
	FallbackApprovalRequired  bool   `json:"fallback_approval_required"`
	FallbackApprovalAvailable bool   `json:"fallback_approval_available"`
	FallbackApprovalVersion   int    `json:"fallback_approval_version"`
}, status string) int {
	count := 0
	for _, node := range nodes {
		if node.Status == status {
			count++
		}
	}
	return count
}

func preflightMission(t *testing.T, ctx context.Context, client *localipc.Client, team teamSummary, objective string) (preflightEnvelope, error) {
	return preflightMissionWithAttempt(t, ctx, client, team, objective, false)
}

func preflightMissionWithAttempt(t *testing.T, ctx context.Context, client *localipc.Client, team teamSummary, objective string, newAttempt bool) (preflightEnvelope, error) {
	t.Helper()
	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		var raw json.RawMessage
		err := client.Call(ctx, "mission_execution", map[string]any{
			"schema_version": 1, "operation": "preflight",
			"mission_id":            "mission/" + team.TeamInstanceID,
			"team_instance_id":      team.TeamInstanceID,
			"work_package_id":       "work-package.coding",
			"work_package_digest":   "4eea514fca13aa241cd004277e31c9c1fe296d34646ceafd618809b6b22c8a4f",
			"objective":             objective,
			"new_attempt":           newAttempt,
			"expected_view_version": currentViewVersion(t, ctx, client),
			"correlation_id":        matrixOperationID(),
		}, &raw)
		if err != nil {
			var remote *localipc.RemoteError
			if errors.As(err, &remote) && remote.Code == "conflict" &&
				attempt < attempts-1 {
				continue
			}
			return preflightEnvelope{}, err
		}
		var envelope preflightEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return preflightEnvelope{}, fmt.Errorf("decode: %w", err)
		}
		return envelope, nil
	}
	return preflightEnvelope{}, errors.New("preflight view version conflict retries exhausted")
}

func currentViewVersion(t *testing.T, ctx context.Context, client *localipc.Client) string {
	t.Helper()
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
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
	return runMissionWithAttemptObjective(
		t, ctx, client, team, preflight, false, liveMissionE2EObjective,
	)
}

func runMissionWithAttempt(t *testing.T, ctx context.Context, client *localipc.Client, team teamSummary, preflight preflightEnvelope, newAttempt bool) (missionRunResult, error) {
	return runMissionWithAttemptObjective(
		t, ctx, client, team, preflight, newAttempt, liveMissionE2EObjective,
	)
}

func runMissionWithAttemptObjective(t *testing.T, ctx context.Context, client *localipc.Client, team teamSummary, preflight preflightEnvelope, newAttempt bool, objective string) (missionRunResult, error) {
	t.Helper()
	if preflight.Preflight == nil {
		return missionRunResult{}, errors.New("no preflight envelope")
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
			"new_attempt":           newAttempt,
			"expected_view_version": preflight.Preflight.ViewVersion,
			"preflight_digest":      preflight.Preflight.PreflightDigest,
			"correlation_id":        matrixOperationID(),
		}, &raw)
		if err != nil {
			var remote *localipc.RemoteError
			if errors.As(err, &remote) && remote.Code == "conflict" {
				t.Logf("mission start conflict attempt=%d stage=%s recoverable=%v", attempt+1, remote.Stage, remote.Recoverable)
				if attempt < attempts-1 {
					// The authoritative view moved after the review. Re-run
					// preflight so the next start uses a fresh digest.
					refreshed, preflightErr := preflightMissionWithAttempt(
						t, ctx, client, team, objective, newAttempt,
					)
					if preflightErr != nil {
						return missionRunResult{}, preflightErr
					}
					preflight = refreshed
					continue
				}
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
	return missionRunResult{}, errors.New("mission start view version conflict retries exhausted")
}

type teamBoardCostRow struct {
	Currency         string `json:"currency"`
	Source           string `json:"source"`
	AmountMicrounits int64  `json:"amount_microunits"`
}

type teamBoardAccountRow struct {
	ProviderID                 string             `json:"provider_id"`
	ProviderAccountID          string             `json:"provider_account_id"`
	ActiveAttempts             int                `json:"active_attempts"`
	AttemptCount               int                `json:"attempt_count"`
	FailedAttempts             int                `json:"failed_attempts"`
	RateLimitedAttempts        int                `json:"rate_limited_attempts"`
	ErrorRateBasisPoints       int                `json:"error_rate_basis_points"`
	BudgetAttemptCount         int                `json:"budget_attempt_count"`
	BudgetUnits                int64              `json:"budget_units"`
	PolicyAvailable            bool               `json:"policy_available"`
	PolicyRevision             int64              `json:"policy_revision"`
	PolicyDigest               string             `json:"policy_digest"`
	MaximumConcurrentAttempts  int                `json:"maximum_concurrent_attempts"`
	DispatchWindowSeconds      int64              `json:"dispatch_window_seconds"`
	MaximumDispatchStarts      int                `json:"maximum_dispatch_starts"`
	MaximumAssignedBudgetUnits int64              `json:"maximum_assigned_budget_units"`
	ActiveAssignedBudgetUnits  int64              `json:"active_assigned_budget_units"`
	AccountingAttemptCount     int                `json:"accounting_attempt_count"`
	UsageAttemptCount          int                `json:"usage_attempt_count"`
	InputTokens                int64              `json:"input_tokens"`
	OutputTokens               int64              `json:"output_tokens"`
	CacheReadTokens            int64              `json:"cache_read_tokens"`
	CacheWriteTokens           int64              `json:"cache_write_tokens"`
	TotalTokens                int64              `json:"total_tokens"`
	CostAttemptCount           int                `json:"cost_attempt_count"`
	Costs                      []teamBoardCostRow `json:"costs"`
	AggregationOverflow        bool               `json:"aggregation_overflow"`
}

type teamBoardNodeRow struct {
	LogicalNodeID           string `json:"logical_node_id"`
	Status                  string `json:"status"`
	CurrentAttempt          int    `json:"current_attempt"`
	WorkItemID              string `json:"work_item_id"`
	RunID                   string `json:"run_id"`
	RuntimeInstanceID       string `json:"runtime_instance_id"`
	AgentInstanceID         string `json:"agent_instance_id"`
	ExecutionBinding        bool   `json:"execution_binding_available"`
	HarnessAdapter          string `json:"harness_adapter"`
	ProviderID              string `json:"provider_id"`
	ProviderAccountID       string `json:"provider_account_id"`
	ModelID                 string `json:"model_id"`
	CredentialRevision      int64  `json:"credential_revision"`
	ContextCapsule          bool   `json:"context_capsule_available"`
	ContextCapsuleDigest    string `json:"context_capsule_digest"`
	RouteSegment            bool   `json:"route_segment_available"`
	RouteSegmentID          string `json:"route_segment_id"`
	RouteSegmentDigest      string `json:"route_segment_digest"`
	DisclosureReceipt       string `json:"context_disclosure_receipt_digest"`
	ProviderPolicy          bool   `json:"provider_account_policy_available"`
	ProviderPolicyRevision  int64  `json:"provider_account_policy_revision"`
	AccountingAvailable     bool   `json:"accounting_available"`
	UsageObserved           bool   `json:"usage_observed"`
	TotalTokens             int64  `json:"total_tokens"`
	FallbackConfigured      bool   `json:"fallback_configured"`
	FallbackConsumed        bool   `json:"fallback_consumed"`
	FallbackApprovalVersion int    `json:"fallback_approval_version"`
}

type teamBoardWire struct {
	SchemaVersion    int                   `json:"schema_version"`
	TeamInstanceID   string                `json:"team_instance_id"`
	PlanDigest       string                `json:"plan_digest"`
	Status           string                `json:"status"`
	ViewVersion      string                `json:"view_version"`
	Nodes            []teamBoardNodeRow    `json:"nodes"`
	ProviderAccounts []teamBoardAccountRow `json:"provider_accounts"`
	Cost             struct {
		Observed         bool   `json:"observed"`
		AmountMicrounits *int64 `json:"amount_microunits"`
		Currency         string `json:"currency"`
	} `json:"cost"`
}

func driveMixedProviderMissionToCompletion(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamInstanceID string,
) teamBoardWire {
	t.Helper()
	deadline := time.Now().Add(9 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			t.Fatalf("context expired waiting for mixed-provider Mission: %v", ctx.Err())
		}
		board := readTeamBoard(t, ctx, client, teamInstanceID)
		switch board.Status {
		case "succeeded":
			t.Logf("mixed-provider Mission completed with %d node rows", len(board.Nodes))
			return board
		case "failed", "cancelled", "blocked", "human_required", "degraded":
			t.Fatalf("mixed-provider Mission settled unsuccessfully: status=%s nodes=%#v",
				board.Status, board.Nodes)
		case "ready_for_review":
			if acceptOnePreparedReview(t, ctx, client, teamInstanceID) {
				continue
			}
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			t.Fatalf("context expired waiting for mixed-provider Mission: %v", ctx.Err())
		}
	}
	t.Fatal("mixed-provider Mission did not complete within 9 minutes")
	return teamBoardWire{}
}

func acceptOnePreparedReview(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamInstanceID string,
) bool {
	t.Helper()
	raw, err := productSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("snapshot prepared reviews: %v", err)
	}
	var snapshot struct {
		Prepared []app.MissionDecisionCommand `json:"prepared_decisions"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode prepared reviews: %v", err)
	}
	for _, command := range snapshot.Prepared {
		if command.TeamInstanceID != teamInstanceID || command.Kind != "review" {
			continue
		}
		var sheet app.MissionDecisionSheet
		if err := client.Call(ctx, "mission_decision", command, &sheet); err != nil {
			var remote *localipc.RemoteError
			if errors.As(err, &remote) && remote.Code == "conflict" {
				return false
			}
			t.Fatalf("read review gate %s: %v", command.LogicalNodeID, err)
		}
		if !sheet.Prepared || !containsString(sheet.PreparedActions, "accept_result") {
			t.Fatalf("review gate is not prepared for acceptance: %#v", sheet)
		}
		command.Operation = "submit"
		command.Action = "accept_result"
		command.CorrelationID = matrixOperationID()
		var result app.MissionDecisionResult
		if err := client.Call(ctx, "mission_decision", command, &result); err != nil {
			var remote *localipc.RemoteError
			if errors.As(err, &remote) && remote.Code == "conflict" {
				return false
			}
			t.Fatalf("accept review gate %s: %v", command.LogicalNodeID, err)
		}
		if !result.Authoritative || result.Status != "accepted" {
			t.Fatalf("review acceptance was not authoritative: %#v", result)
		}
		t.Logf("accepted review gate for node=%s attempt=%d",
			command.LogicalNodeID, command.AttemptNumber)
		return true
	}
	return false
}

func productSnapshotEventually(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) (json.RawMessage, error) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var raw json.RawMessage
		err := client.Call(ctx, "snapshot", map[string]any{
			"after_team_id": "", "after_runtime_id": "", "after_run_id": "",
			"after_evidence_id": "", "limit": 64,
		}, &raw)
		if err == nil {
			return raw, nil
		}
		lastErr = err
		var remote *localipc.RemoteError
		recoverableStateUnavailable := errors.As(err, &remote) &&
			remote.Code == "state_unavailable" && remote.Recoverable
		if !errors.Is(err, localipc.ErrLocalProductUnavailable) &&
			!errors.Is(err, localipc.ErrProtocolTimeout) &&
			!recoverableStateUnavailable {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func verifyMixedProviderNodeRows(t *testing.T, board teamBoardWire) {
	t.Helper()
	if board.Status != "succeeded" || len(board.Nodes) != 4 {
		t.Fatalf("expected a succeeded four-node Mission, got status=%s nodes=%d",
			board.Status, len(board.Nodes))
	}
	providerCounts := map[string]int{}
	for _, node := range board.Nodes {
		if node.Status != "succeeded" || node.CurrentAttempt < 1 ||
			node.WorkItemID == "" || node.RunID == "" ||
			node.RuntimeInstanceID == "" || node.AgentInstanceID == "" ||
			!node.ExecutionBinding || node.HarnessAdapter == "" ||
			node.ProviderID == "" || node.ProviderAccountID == "" ||
			node.ModelID == "" || node.CredentialRevision < 1 ||
			!node.ContextCapsule || node.ContextCapsuleDigest == "" ||
			!node.RouteSegment || node.RouteSegmentID == "" ||
			node.RouteSegmentDigest == "" || node.DisclosureReceipt == "" ||
			!node.ProviderPolicy || node.ProviderPolicyRevision < 1 ||
			!node.AccountingAvailable || !node.UsageObserved || node.TotalTokens <= 0 {
			t.Fatalf("node lacks completed governed execution lineage: %#v", node)
		}
		providerCounts[node.ProviderID]++
	}
	if providerCounts["deepseek"] != 2 || providerCounts["minimax"] != 2 {
		t.Fatalf("expected two independently bound nodes per Provider, got %v", providerCounts)
	}
}

func verifyMixedProviderAccountingCompletion(t *testing.T, board teamBoardWire) {
	t.Helper()
	completed := map[string]int{}
	for _, row := range board.ProviderAccounts {
		if row.ProviderID != "deepseek" && row.ProviderID != "minimax" {
			continue
		}
		if row.AttemptCount < 2 || row.ActiveAttempts != 0 ||
			row.FailedAttempts != 0 || row.AccountingAttemptCount != row.AttemptCount ||
			row.UsageAttemptCount != row.AttemptCount || row.TotalTokens <= 0 {
			t.Fatalf("mixed-provider account did not complete cleanly: %#v", row)
		}
		completed[row.ProviderID] = row.AttemptCount
	}
	if completed["deepseek"] < 2 || completed["minimax"] < 2 {
		t.Fatalf("missing completed per-Provider accounting: %v", completed)
	}
}

func verifyOpenCodeAgentNodeRows(t *testing.T, board teamBoardWire) {
	t.Helper()
	if board.Status != "succeeded" || len(board.Nodes) != 2 {
		t.Fatalf("expected a succeeded two-node OpenCode Mission, got status=%s nodes=%d",
			board.Status, len(board.Nodes))
	}
	seenLoomNative := false
	seenOpenCode := false
	for _, node := range board.Nodes {
		if node.Status != "succeeded" || node.CurrentAttempt < 1 ||
			node.WorkItemID == "" || node.RunID == "" ||
			node.RuntimeInstanceID == "" || node.AgentInstanceID == "" ||
			!node.ExecutionBinding || node.HarnessAdapter == "" ||
			node.ProviderID == "" || node.ModelID == "" ||
			!node.ContextCapsule || node.ContextCapsuleDigest == "" ||
			!node.RouteSegment || node.RouteSegmentID == "" ||
			node.RouteSegmentDigest == "" || node.DisclosureReceipt == "" {
			t.Fatalf("node lacks governed execution lineage: %#v", node)
		}
		switch node.HarnessAdapter {
		case nativeadapter.LoomNativeAgentAdapterType:
			seenLoomNative = true
			if node.ProviderAccountID != "deepseek.primary" ||
				node.ProviderID != "deepseek" ||
				node.CredentialRevision < 1 || !node.ProviderPolicy ||
				node.ProviderPolicyRevision < 1 || !node.AccountingAvailable ||
				!node.UsageObserved || node.TotalTokens <= 0 {
				t.Fatalf("DeepSeek peer lost brokered governance: %#v", node)
			}
		case harnessadapter.OpenCodeAdapterType:
			seenOpenCode = true
			if node.ProviderID != "deepseek" ||
				node.ProviderAccountID != "deepseek.primary" ||
				node.CredentialRevision < 1 ||
				node.ModelID != "deepseek/"+nativeadapter.DeepSeekAgentModelID ||
				!node.ProviderPolicy || node.ProviderPolicyRevision < 1 {
				t.Fatalf("OpenCode brokered binding is not truthful: %#v", node)
			}
		default:
			t.Fatalf("unexpected Harness in OpenCode Team: %#v", node)
		}
	}
	if !seenLoomNative || !seenOpenCode {
		t.Fatalf("missing independently bound DeepSeek/OpenCode peers: %#v", board.Nodes)
	}
}

func verifyOpenCodeNativeAgentNodeRows(t *testing.T, board teamBoardWire) {
	t.Helper()
	if board.Status != "succeeded" || len(board.Nodes) != 2 {
		t.Fatalf(
			"expected a succeeded two-node native OpenCode Mission, got status=%s nodes=%d",
			board.Status, len(board.Nodes),
		)
	}
	for _, node := range board.Nodes {
		if node.Status != "succeeded" || node.CurrentAttempt < 1 ||
			node.HarnessAdapter != harnessadapter.OpenCodeAdapterType ||
			node.RuntimeInstanceID != productOpenCodeRuntimeInstanceID ||
			node.ProviderID != "opencode" || node.ProviderAccountID != "" ||
			node.ModelID != provider.OpenCodeConversationDefaultModel ||
			node.CredentialRevision != 0 || node.ProviderPolicy ||
			node.ProviderPolicyRevision != 0 ||
			!node.ExecutionBinding || !node.ContextCapsule ||
			node.ContextCapsuleDigest == "" || !node.RouteSegment ||
			node.RouteSegmentDigest == "" || node.DisclosureReceipt == "" {
			t.Fatalf("native OpenCode node lost truthful governed binding: %#v", node)
		}
	}
	for _, account := range board.ProviderAccounts {
		if account.ProviderID == "opencode" {
			t.Fatalf("native auth invented a Vault Provider Account row: %#v", account)
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func readTeamBoard(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamInstanceID string,
) teamBoardWire {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "timeline_page", map[string]any{
		"team_instance_id": teamInstanceID, "cursor": "", "limit": 64,
	}, &raw); err != nil {
		t.Fatalf("timeline_page: %v", err)
	}
	var page struct {
		Board teamBoardWire `json:"board"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("decode timeline board: %v", err)
	}
	return page.Board
}

// configureProviderGovernance configures the Provider Account Policy (exact
// concurrency + budget ceilings and disclosure) and the Model Rate Card for an
// account so the G6 board carries policy revision, ceilings and rate-card
// cost estimates.
func configureProviderGovernance(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	providerID string,
	providerAccountID string,
	modelID string,
) {
	t.Helper()
	// Read the current policy revision + rate-card revision so the configure
	// calls are idempotent across live-gate reruns.
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("setup_snapshot: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID        string `json:"provider_id"`
			ProviderAccountID string `json:"provider_account_id"`
			PolicyRevision    int64  `json:"policy_revision"`
			RateCards         []struct {
				ModelID  string `json:"model_id"`
				Revision int64  `json:"revision"`
			} `json:"rate_cards"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	policyRevision := int64(0)
	rateCardRevision := int64(0)
	rateCardPresent := false
	for _, account := range snap.Accounts {
		if account.ProviderID != providerID ||
			account.ProviderAccountID != providerAccountID {
			continue
		}
		policyRevision = account.PolicyRevision
		for _, rateCard := range account.RateCards {
			if rateCard.ModelID == modelID {
				rateCardPresent = true
				rateCardRevision = rateCard.Revision
			}
		}
	}
	if policyRevision == 0 {
		if err := client.Call(ctx, "provider_account_policy_configure", map[string]any{
			"provider_id": providerID, "provider_account_id": providerAccountID,
			"expected_revision":             0,
			"maximum_concurrent_attempts":   4,
			"dispatch_window_seconds":       3600,
			"maximum_dispatch_starts":       1000,
			"maximum_assigned_budget_units": 1_000_000,
			"trust_domain":                  "local_runtime", "retention_mode": "limited_retention",
			"data_region": "local", "operation_id": matrixOperationID(),
		}, &raw); err != nil {
			t.Fatalf("policy %s: %v", providerID, err)
		}
	}
	if !rateCardPresent {
		if err := client.Call(ctx, "provider_model_rate_card_configure", map[string]any{
			"provider_id": providerID, "provider_account_id": providerAccountID,
			"model_id": modelID, "expected_revision": rateCardRevision,
			"currency": "USD", "input_token_basis": "input_excludes_cache",
			"input_microunits_per_million":       270_000,
			"output_microunits_per_million":      1_100_000,
			"cache_read_microunits_per_million":  27_000,
			"cache_write_microunits_per_million": 270_000,
			"rounding_mode":                      "ceiling_per_attempt",
			"operation_id":                       matrixOperationID(),
		}, &raw); err != nil {
			t.Fatalf("rate card %s: %v", providerID, err)
		}
	}
	t.Logf("governance configured for %s/%s (%s)", providerID, providerAccountID, modelID)
}

// verifyAccountingRows asserts the G6 accounting/governance board: every
// Provider Account that participated shows the exact attempt, error-rate,
// budget, policy, token, cost and ceiling fields, and accounting coverage is
// internally consistent (never exceeds attempt counts).
func verifyAccountingRows(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	teamInstanceID string,
) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Minute)
	var board teamBoardWire
	for {
		board = readTeamBoard(t, ctx, client, teamInstanceID)
		accounted := 0
		for _, row := range board.ProviderAccounts {
			if row.AccountingAttemptCount > 0 {
				accounted++
			}
		}
		if accounted > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if len(board.ProviderAccounts) == 0 {
		t.Fatalf("board has no Provider Account rows")
	}
	seen := map[string]bool{}
	for _, row := range board.ProviderAccounts {
		seen[row.ProviderID+":"+row.ProviderAccountID] = true
		if row.AttemptCount < 0 || row.FailedAttempts < 0 ||
			row.FailedAttempts > row.AttemptCount ||
			row.RateLimitedAttempts < 0 ||
			row.RateLimitedAttempts > row.AttemptCount ||
			row.AccountingAttemptCount < 0 ||
			row.AccountingAttemptCount > row.AttemptCount ||
			row.UsageAttemptCount < 0 ||
			row.UsageAttemptCount > row.AccountingAttemptCount ||
			row.CostAttemptCount < 0 ||
			row.CostAttemptCount > row.AccountingAttemptCount ||
			row.InputTokens < 0 || row.OutputTokens < 0 ||
			row.CacheReadTokens < 0 || row.CacheWriteTokens < 0 ||
			row.TotalTokens < 0 ||
			row.TotalTokens != row.InputTokens+row.OutputTokens {
			t.Fatalf("accounting row inconsistent: %#v", row)
		}
		t.Logf(
			"account %s/%s attempts=%d failed=%d rateLimited=%d errorRateBps=%d accounting=%d usage=%d cost=%d tokens=%d budgetUnits=%d policyRev=%d maxConcurrent=%d overflow=%v",
			row.ProviderID, row.ProviderAccountID, row.AttemptCount,
			row.FailedAttempts, row.RateLimitedAttempts, row.ErrorRateBasisPoints,
			row.AccountingAttemptCount, row.UsageAttemptCount, row.CostAttemptCount,
			row.TotalTokens, row.BudgetUnits, row.PolicyRevision,
			row.MaximumConcurrentAttempts, row.AggregationOverflow,
		)
		if row.MaximumConcurrentAttempts <= 0 || row.DispatchWindowSeconds <= 0 {
			t.Fatalf("account %s lacks ceiling fields: %#v", row.ProviderID, row)
		}
		if !row.PolicyAvailable || row.PolicyRevision <= 0 || row.PolicyDigest == "" {
			t.Fatalf("account %s lacks policy lineage: %#v", row.ProviderID, row)
		}
		// Failed/cancelled attempts legitimately carry no usage/cost
		// accounting; coverage must never exceed attempts and completed
		// attempts must carry exact token usage.
		if row.AccountingAttemptCount > 0 {
			if row.UsageAttemptCount == 0 || row.TotalTokens <= 0 {
				t.Fatalf("account %s accounting lacks token usage: %#v",
					row.ProviderID, row)
			}
		}
		for _, cost := range row.Costs {
			if cost.Currency == "" || cost.Source == "" || cost.AmountMicrounits < 0 {
				t.Fatalf("account %s invalid cost row: %#v", row.ProviderID, cost)
			}
		}
	}
	if !seen["deepseek:deepseek.primary"] {
		t.Fatalf("board missing deepseek accounting row: %#v", board.ProviderAccounts)
	}
	completedAccounting := false
	for _, row := range board.ProviderAccounts {
		if row.AccountingAttemptCount > 0 {
			completedAccounting = true
		}
	}
	if !completedAccounting {
		t.Fatalf("no account completed an attempt with accounting: %#v",
			board.ProviderAccounts)
	}
	if board.Cost.Observed {
		if board.Cost.Currency == "" || board.Cost.AmountMicrounits == nil ||
			*board.Cost.AmountMicrounits < 0 {
			t.Fatalf("board cost observation inconsistent: %#v", board.Cost)
		}
	}
}

// confirmMixedProviderTeam builds and confirms the 4-Agent mixed-provider
// Team (DeepSeek main + MiniMax bounded worker + DeepSeek reviewer + MiniMax
// researcher) and returns the newly materialized TeamInstance.
func confirmMixedProviderTeam(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	roleOptions []roleOption,
	displayName string,
) teamSummary {
	t.Helper()
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", displayName)
	session = editSession(t, ctx, client, session, "purpose",
		"Verify a real mixed-provider Team end to end")
	mainRole := firstRole(roleOptions, "main", "deepseek")
	subMinimax := firstRole(roleOptions, "subagent", "minimax")
	subDeepseekReviewer := firstRoleByResponsibility(
		roleOptions, "subagent", "deepseek", "Review changes",
	)
	subMinimaxResearcher := firstRoleByResponsibility(
		roleOptions, "subagent", "minimax", "Investigate bounded questions",
	)
	if mainRole.ID == "" || subMinimax.ID == "" ||
		subDeepseekReviewer.ID == "" || subMinimaxResearcher.ID == "" {
		t.Fatalf("missing mixed-provider roles: main=%q minimax=%q ds-reviewer=%q mm-researcher=%q",
			mainRole.ID, subMinimax.ID, subDeepseekReviewer.ID, subMinimaxResearcher.ID)
	}
	session = editSession(t, ctx, client, session, "main_role", mainRole.ID)
	defaultSubAgent := firstPreviewSubagent(session)
	if defaultSubAgent.AgentDefinitionID == "" {
		t.Fatal("draft has no default subagent slot")
	}
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_role", subMinimax.ID,
		defaultSubAgent.AgentDefinitionID,
	)
	session = editSession(t, ctx, client, session, "subagent_add", subDeepseekReviewer.ID)
	session = editSession(t, ctx, client, session, "subagent_add", subMinimaxResearcher.ID)
	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf("draft not confirmable: canConfirm=%v binding=%q gaps=%v",
			session.CanConfirm, session.BindingDg, session.Preview.CompatibilityGaps)
	}
	beforeTeamIDs := map[string]bool{}
	for _, existing := range refreshTeams(t, ctx, client) {
		beforeTeamIDs[existing.TeamInstanceID] = true
	}
	definitionID := "team-live-mixed-" + time.Now().UTC().Format("20060102T150405Z")
	confirmation, err := confirmSession(t, ctx, client, session, definitionID)
	if err != nil {
		stage, diagnosticErr := diagnoseLiveSavedTeamMaterialization(ctx, definitionID)
		var remote *localipc.RemoteError
		if errors.As(err, &remote) {
			t.Fatalf(
				"confirm: code=%s stage=%s recoverable=%v; local materialization stage=%s: %v",
				remote.Code, remote.Stage, remote.Recoverable, stage, diagnosticErr,
			)
		}
		t.Fatalf("confirm: %v; local materialization stage=%s: %v", err, stage, diagnosticErr)
	}
	t.Logf("confirmed: team=%s status=%s instanceCreated=%v runCreated=%v",
		confirmation.TeamDefinitionID, confirmation.Status,
		confirmation.TeamInstanceCreated, confirmation.RunCreated)
	teams := refreshTeams(t, ctx, client)
	var team teamSummary
	for _, candidate := range teams {
		if candidate.SourceKind == "saved_team" && candidate.Confirmed &&
			candidate.Executable && !candidate.ReadOnly &&
			!beforeTeamIDs[candidate.TeamInstanceID] {
			team = candidate
			break
		}
	}
	if team.TeamInstanceID == "" {
		t.Fatalf("new confirmed team missing from snapshot: %#v", teams)
	}
	t.Logf("team=%s display=%q executable=%v confirmed=%v",
		team.TeamInstanceID, team.DisplayName, team.Executable, team.Confirmed)
	if !team.Confirmed || !team.Executable {
		t.Fatalf("confirmed team not executable: %#v", team)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer cleanupCancel()
		archiveLiveTeams(t, cleanupCtx, client, definitionID)
	})
	return team
}

func confirmFourProviderTeam(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	selected []roleOption,
) teamSummary {
	t.Helper()
	if len(selected) != 4 {
		t.Fatalf("four-Provider selection must contain four roles: %#v", selected)
	}
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live Four Provider Team")
	session = editSession(t, ctx, client, session, "purpose",
		"Verify four independent Harness and Provider Account bindings end to end")
	session = editSession(t, ctx, client, session, "main_role", selected[0].ID)
	defaultSubagent := firstPreviewSubagent(session)
	if defaultSubagent.AgentDefinitionID == "" {
		t.Fatal("four-Provider draft has no default subagent slot")
	}
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_role", selected[1].ID,
		defaultSubagent.AgentDefinitionID,
	)
	session = editSession(t, ctx, client, session, "subagent_add", selected[2].ID)
	session = editSession(t, ctx, client, session, "subagent_add", selected[3].ID)
	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf("four-Provider draft not confirmable: canConfirm=%v binding=%q gaps=%v",
			session.CanConfirm, session.BindingDg, session.Preview.CompatibilityGaps)
	}

	beforeTeamIDs := map[string]bool{}
	for _, existing := range refreshTeams(t, ctx, client) {
		beforeTeamIDs[existing.TeamInstanceID] = true
	}
	definitionID := "team-live-four-provider-" + time.Now().UTC().Format("20060102T150405Z")
	confirmation, err := confirmSession(t, ctx, client, session, definitionID)
	if err != nil {
		stage, diagnosticErr := diagnoseLiveSavedTeamMaterialization(ctx, definitionID)
		t.Fatalf("confirm four-Provider Team: %v; local materialization stage=%s: %v",
			err, stage, diagnosticErr)
	}
	if !confirmation.TeamInstanceCreated {
		t.Fatalf("four-Provider Team was not materialized: %#v", confirmation)
	}
	for _, candidate := range refreshTeams(t, ctx, client) {
		if candidate.SourceKind == "saved_team" && candidate.Confirmed &&
			candidate.Executable && !candidate.ReadOnly &&
			!beforeTeamIDs[candidate.TeamInstanceID] {
			t.Cleanup(func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cleanupCancel()
				archiveLiveTeams(t, cleanupCtx, client, definitionID)
			})
			return candidate
		}
	}
	t.Fatalf("new four-Provider Team missing after confirmation: %#v", refreshTeams(t, ctx, client))
	return teamSummary{}
}

func verifyFourProviderNodeAndAccountRows(
	t *testing.T,
	board teamBoardWire,
	selected []roleOption,
) {
	t.Helper()
	if len(board.Nodes) != len(selected) {
		t.Fatalf("four-Provider board nodes=%d want=%d: %#v",
			len(board.Nodes), len(selected), board.Nodes)
	}
	expected := make(map[string]roleOption, len(selected))
	for _, role := range selected {
		expected[role.ProviderID+":"+role.ProviderAccountID] = role
	}
	seen := map[string]bool{}
	for _, node := range board.Nodes {
		key := node.ProviderID + ":" + node.ProviderAccountID
		role, ok := expected[key]
		if !ok {
			t.Fatalf("unexpected four-Provider node binding %q: %#v", key, node)
		}
		if seen[key] {
			t.Fatalf("duplicate four-Provider node binding %q: %#v", key, board.Nodes)
		}
		seen[key] = true
		if node.Status != "succeeded" || !node.ExecutionBinding ||
			node.HarnessAdapter != role.HarnessAdapter ||
			node.RuntimeInstanceID != role.RuntimeID ||
			node.ModelID != role.ModelID ||
			node.CredentialRevision != role.CredentialRevision ||
			!node.ContextCapsule || node.ContextCapsuleDigest == "" ||
			!node.RouteSegment || node.RouteSegmentDigest == "" ||
			node.DisclosureReceipt == "" || !node.ProviderPolicy ||
			node.ProviderPolicyRevision <= 0 {
			t.Fatalf("four-Provider node did not freeze exact role binding: role=%#v node=%#v",
				role, node)
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("four-Provider nodes incomplete: seen=%v expected=%v", seen, expected)
	}

	accountSeen := map[string]bool{}
	for _, row := range board.ProviderAccounts {
		key := row.ProviderID + ":" + row.ProviderAccountID
		if _, ok := expected[key]; !ok {
			continue
		}
		accountSeen[key] = true
		if row.ActiveAttempts != 0 || row.AttemptCount < 1 || row.FailedAttempts != 0 ||
			!row.PolicyAvailable || row.PolicyRevision <= 0 || row.PolicyDigest == "" ||
			row.AccountingAttemptCount < 1 || row.UsageAttemptCount < 1 ||
			row.TotalTokens <= 0 {
			t.Fatalf("four-Provider account row incomplete for %s: %#v", key, row)
		}
	}
	if len(accountSeen) != len(expected) {
		t.Fatalf("four-Provider accounting rows incomplete: seen=%v expected=%v",
			accountSeen, expected)
	}
}

func confirmOpenCodeAgentTeam(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	roleOptions []roleOption,
) teamSummary {
	t.Helper()
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live OpenCode Agent Team")
	session = editSession(t, ctx, client, session, "purpose",
		"Verify OpenCode native auth and brokered Provider Agents in one Mission")
	mainRole := firstRoleForRuntime(
		roleOptions, "main", productNativeAgentRuntimeInstanceID, "deepseek",
	)
	openCodeRole := firstRoleForRuntime(
		roleOptions, "subagent", productOpenCodeRuntimeInstanceID, "deepseek",
	)
	defaultSubAgent := firstPreviewSubagent(session)
	if mainRole.ID == "" || openCodeRole.ID == "" || defaultSubAgent.AgentDefinitionID == "" {
		t.Fatalf("missing OpenCode Team roles: main=%q opencode=%q default=%q",
			mainRole.ID, openCodeRole.ID, defaultSubAgent.AgentDefinitionID)
	}
	session = editSession(t, ctx, client, session, "main_role", mainRole.ID)
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_role", openCodeRole.ID,
		defaultSubAgent.AgentDefinitionID,
	)
	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf("OpenCode draft not confirmable: canConfirm=%v binding=%q gaps=%v",
			session.CanConfirm, session.BindingDg, session.Preview.CompatibilityGaps)
	}

	beforeTeamIDs := map[string]bool{}
	for _, existing := range refreshTeams(t, ctx, client) {
		beforeTeamIDs[existing.TeamInstanceID] = true
	}
	definitionID := "team-live-mixed-opencode-" + time.Now().UTC().Format("20060102T150405Z")
	confirmation, err := confirmSession(t, ctx, client, session, definitionID)
	if err != nil {
		stage, diagnosticErr := diagnoseLiveSavedTeamMaterialization(ctx, definitionID)
		t.Fatalf("confirm OpenCode Team: %v; local materialization stage=%s: %v",
			err, stage, diagnosticErr)
	}
	if !confirmation.TeamInstanceCreated {
		t.Fatalf("OpenCode Team was not materialized: %#v", confirmation)
	}
	for _, candidate := range refreshTeams(t, ctx, client) {
		if candidate.SourceKind == "saved_team" && candidate.Confirmed &&
			candidate.Executable && !candidate.ReadOnly &&
			!beforeTeamIDs[candidate.TeamInstanceID] {
			t.Cleanup(func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cleanupCancel()
				archiveLiveTeams(t, cleanupCtx, client, definitionID)
			})
			return candidate
		}
	}
	t.Fatalf("new OpenCode Team missing from snapshot")
	return teamSummary{}
}

func confirmOpenCodeNativeAgentTeam(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	roleOptions []roleOption,
) teamSummary {
	t.Helper()
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live OpenCode Native Team")
	session = editSession(t, ctx, client, session, "purpose",
		"Verify OpenCode hosted native auth in one governed Mission")
	mainRole := firstRoleForRuntime(
		roleOptions, "main", productOpenCodeRuntimeInstanceID, "opencode",
	)
	subagentRole := firstRoleForRuntime(
		roleOptions, "subagent", productOpenCodeRuntimeInstanceID, "opencode",
	)
	defaultSubAgent := firstPreviewSubagent(session)
	if mainRole.ID == "" || subagentRole.ID == "" ||
		defaultSubAgent.AgentDefinitionID == "" {
		t.Fatalf(
			"missing native OpenCode Team roles: main=%q subagent=%q default=%q",
			mainRole.ID, subagentRole.ID, defaultSubAgent.AgentDefinitionID,
		)
	}
	session = editSession(t, ctx, client, session, "main_role", mainRole.ID)
	session = editSessionTargeted(
		t, ctx, client, session, "subagent_role", subagentRole.ID,
		defaultSubAgent.AgentDefinitionID,
	)
	if !session.CanConfirm || session.BindingDg == "" {
		t.Fatalf(
			"native OpenCode draft not confirmable: canConfirm=%v binding=%q gaps=%v",
			session.CanConfirm, session.BindingDg, session.Preview.CompatibilityGaps,
		)
	}

	beforeTeamIDs := map[string]bool{}
	for _, existing := range refreshTeams(t, ctx, client) {
		beforeTeamIDs[existing.TeamInstanceID] = true
	}
	definitionID := "team-live-mixed-opencode-native-" +
		time.Now().UTC().Format("20060102T150405Z")
	confirmation, err := confirmSession(t, ctx, client, session, definitionID)
	if err != nil {
		stage, diagnosticErr := diagnoseLiveSavedTeamMaterialization(ctx, definitionID)
		t.Fatalf(
			"confirm native OpenCode Team: %v; local materialization stage=%s: %v",
			err, stage, diagnosticErr,
		)
	}
	if !confirmation.TeamInstanceCreated {
		t.Fatalf("native OpenCode Team was not materialized: %#v", confirmation)
	}
	for _, candidate := range refreshTeams(t, ctx, client) {
		if candidate.SourceKind == "saved_team" && candidate.Confirmed &&
			candidate.Executable && !candidate.ReadOnly &&
			!beforeTeamIDs[candidate.TeamInstanceID] {
			t.Cleanup(func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(
					context.Background(), 30*time.Second,
				)
				defer cleanupCancel()
				archiveLiveTeams(t, cleanupCtx, client, definitionID)
			})
			return candidate
		}
	}
	t.Fatalf("new native OpenCode Team missing from snapshot")
	return teamSummary{}
}

// diagnoseLiveSavedTeamMaterialization makes an isolated SQLite snapshot and
// replays materialization there. It never writes the installed journal and never
// reads credentials or prompt content.
func diagnoseLiveSavedTeamMaterialization(
	ctx context.Context,
	definitionID string,
) (string, error) {
	statePath := os.Getenv("LOOM_STATE_PATH")
	if statePath == "" {
		statePath = "/Users/lune/Library/Application Support/Loom/state/loom.db"
	}
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		return "database_open", err
	}
	temporaryRoot, err := os.MkdirTemp("", "loom-team-materialization-diagnostic-")
	if err != nil {
		_ = database.Close()
		return "database_copy", err
	}
	defer os.RemoveAll(temporaryRoot)
	diagnosticPath := temporaryRoot + "/loom.db"
	if _, err := database.ExecContext(ctx, "VACUUM INTO ?", diagnosticPath); err != nil {
		_ = database.Close()
		return "database_copy", err
	}
	if err := database.Close(); err != nil {
		return "database_copy", err
	}
	database, err = sql.Open("sqlite", diagnosticPath)
	if err != nil {
		return "database_copy_open", err
	}
	defer database.Close()
	readModel := projection.New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		return "projection_rebuild", err
	}
	view := readModel.GlobalReadView()
	record, ok := view.TeamDefinition(definitionID)
	if !ok {
		return "definition_projection", errors.New("committed Team definition not projected")
	}
	catalog, err := productSetupCatalogForView(ctx, view)
	if err != nil {
		return "catalog_rebuild", err
	}
	definition, selections, profiles, err := productSavedTeamMaterializationInputs(record, catalog)
	if err != nil {
		return "materialization_inputs", err
	}
	scope := agents.ScopeIdentity{ProjectID: productSavedTeamResolutionProjectID}
	if definition.Scope() == teams.TeamDefinitionScopeProject {
		scope = definition.ScopeIdentity()
	}
	binding, err := teams.BuildSavedTeamRuntimeBinding(
		[]teams.TeamDefinition{definition},
		definition.ID(),
		scope,
		catalog.AgentDefinitions,
		profiles,
		catalog.RuntimeDiscovery,
		selections,
	)
	if err != nil {
		return "runtime_binding", err
	}
	resolutionCatalog := teams.TeamResolutionCatalogInput{
		AgentDefinitions:       append([]agents.AgentDefinition{}, catalog.AgentDefinitions...),
		RuntimeProfiles:        append([]runtime.RuntimeProfile{}, profiles...),
		TeamDefinitions:        []teams.TeamDefinition{definition},
		MainAgentDefinitionIDs: []string{definition.MainAgentDefinitionID()},
		DefaultMainAgentID:     definition.MainAgentDefinitionID(),
		ReusableDefaultTeamID:  definition.ID(),
	}
	if definition.Scope() == teams.TeamDefinitionScopeProject {
		resolutionCatalog.ProjectDefaultTeamID = definition.ID()
		resolutionCatalog.ReusableDefaultTeamID = ""
	}
	intent := mode.Intent{
		Trigger: mode.TriggerSelectTeam, TargetID: definition.ID(),
		Text: "diagnose confirmed saved Team",
	}
	plan, err := teams.BuildSavedTeamInstantiationPlan(
		intent, scope, resolutionCatalog, binding,
		catalog.RuntimeDiscovery, selections,
	)
	if err != nil {
		return "instantiation_plan", err
	}
	identity, commit, err := newProductSavedTeamMaterializationIdentity(time.Now().UTC())
	if err != nil {
		return "materialization_identity", err
	}
	records, err := teams.BuildSavedTeamInstanceRecordSet(
		plan, intent, scope, resolutionCatalog, binding,
		catalog.RuntimeDiscovery, selections, identity,
	)
	if err != nil {
		return "instance_records", err
	}
	mainRuntime, ok := view.RuntimeInstance(binding.MainBinding().RuntimeInstanceID)
	if !ok {
		return "runtime_projection", errors.New("main runtime is absent from projection")
	}
	binding, err = binding.WithMainRuntimeDiscoveryDigest(mainRuntime.DiscoveryDigest)
	if err != nil {
		return "runtime_discovery_digest", err
	}
	committed, err := state.CommitSavedTeamInstanceRecordSet(
		ctx, journal.NewStore(database), records, plan, intent, scope,
		resolutionCatalog, binding, catalog.RuntimeDiscovery, selections,
		identity, commit,
	)
	if err != nil {
		return "instance_commit", err
	}
	if !committed.Committed() || committed.EventCount() != 2 {
		return "instance_commit_result", errors.New("unexpected commit result")
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return "post_commit_projection", err
	}
	return "post_commit_projection", errors.New("diagnostic clone materialized successfully")
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
	reference, revision, status := providerCredentialState(t, ctx, client, providerID)
	if reference == "" {
		t.Fatalf("%s credential reference not found", providerID)
	}
	if status == "revoked" {
		if err := client.Call(ctx, "credential_replace", map[string]any{
			"provider_id": providerID, "provider_account_id": providerID + ".primary",
			"credential_reference": reference, "expected_revision": revision, "secret": key,
		}, &raw); err != nil {
			t.Fatalf("%s replace: %v", providerID, err)
		}
		reference, revision, _ = providerCredentialState(t, ctx, client, providerID)
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
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
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
	reference, revision, _ := providerCredentialState(t, ctx, client, providerID)
	return reference, revision
}

func providerCredentialState(t *testing.T, ctx context.Context, client *localipc.Client, providerID string) (string, int64, string) {
	t.Helper()
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID string `json:"provider_id"`
			Reference  string `json:"credential_reference"`
			Revision   int64  `json:"revision"`
			Status     string `json:"status"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, account := range snap.Accounts {
		if account.ProviderID == providerID && account.Reference != "" {
			return account.Reference, account.Revision, account.Status
		}
	}
	return "", 0, ""
}

var _ = app.BuilderStartCommand{}
var _ = strings.TrimSpace

// TestLiveSingleAgentFailureIsolationE2E drives the G4 failure-isolation gate
// installed-live: after revoking the MiniMax broker credential, only the two
// MiniMax-bound Agents block at preflight with the exact credential reason
// while both DeepSeek-bound peers stay ready and dispatch, and the accounting
// board isolates the failure to the MiniMax account. It is a paid,
// network-enabled live gate and is skipped unless LOOM_LIVE_TEAM_E2E=1.
func TestLiveSingleAgentFailureIsolationE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_TEAM_E2E") != "1" {
		t.Skip("live team E2E gate requires LOOM_LIVE_TEAM_E2E=1")
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
	ensureProviderVerified(t, ctx, client, "minimax", os.Getenv("MINIMAX_API_KEY"))
	configureProviderGovernance(t, ctx, client, "deepseek", "deepseek.primary", "deepseek-chat")
	configureProviderGovernance(t, ctx, client, "minimax", "minimax.primary", "MiniMax-M3")
	archiveLiveTeams(t, ctx, client)
	roleOptions := refreshRoleOptions(t, ctx, client)
	team := confirmMixedProviderTeam(t, ctx, client, roleOptions, "Live Isolation Team")

	// Revoke the MiniMax credential: the two MiniMax-bound Agents must block,
	// the two DeepSeek-bound peers must stay ready.
	reference, revision := providerCredential(t, ctx, client, "minimax")
	if reference == "" {
		t.Fatal("minimax credential reference missing")
	}
	if err := client.Call(ctx, "credential_revoke", map[string]any{
		"provider_id": "minimax", "provider_account_id": "minimax.primary",
		"credential_reference": reference, "expected_revision": revision,
	}, &struct{}{}); err != nil {
		t.Fatalf("revoke minimax: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 60*time.Second,
		)
		defer cleanupCancel()
		// Restore MiniMax when the broker accepts a fresh configuration;
		// otherwise leave a note (a full state reset re-imports it).
		reference, revision, status := providerCredentialState(
			t, cleanupCtx, client, "minimax",
		)
		if reference == "" || status != "revoked" {
			return
		}
		if err := client.Call(cleanupCtx, "credential_replace", map[string]any{
			"provider_id": "minimax", "provider_account_id": "minimax.primary",
			"credential_reference": reference, "expected_revision": revision,
			"secret": os.Getenv("MINIMAX_API_KEY"),
		}, &struct{}{}); err != nil {
			t.Logf("cleanup minimax restore skipped: %v", err)
			return
		}
		reference, revision, _ = providerCredentialState(t, cleanupCtx, client, "minimax")
		if err := client.Call(cleanupCtx, "credential_verify", map[string]any{
			"provider_id":          "minimax",
			"credential_reference": reference,
			"expected_revision":    revision,
			"operation_id":         matrixOperationID(),
		}, &struct{}{}); err != nil {
			t.Logf("cleanup minimax verify skipped: %v", err)
		}
	})

	preflight, err := preflightMission(t, ctx, client, team, liveMissionE2EObjective)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	readyDeepSeek, blockedMiniMax := 0, 0
	blockedReason := ""
	for _, node := range preflight.Preflight.Nodes {
		t.Logf("  node %s provider=%s status=%s block=%q",
			node.LogicalNodeID, node.ProviderID, node.Status, node.BlockReason)
		switch {
		case node.Status == "ready" && node.ProviderID == "deepseek":
			readyDeepSeek++
		case node.Status == "blocked" && node.ProviderID == "minimax":
			blockedMiniMax++
			blockedReason = node.BlockReason
		}
	}
	if readyDeepSeek != 2 || blockedMiniMax != 2 {
		t.Fatalf("expected 2 ready DeepSeek + 2 blocked MiniMax, got ready=%d blocked=%d: %#v",
			readyDeepSeek, blockedMiniMax, preflight.Preflight.Nodes)
	}
	if !strings.Contains(blockedReason, "Credential") {
		t.Fatalf("MiniMax block reason is not credential-scoped: %q", blockedReason)
	}

	// Start: DeepSeek peers dispatch (real calls), MiniMax nodes stay blocked.
	if _, err := runMission(t, ctx, client, team, preflight); err != nil {
		t.Fatalf("run mission: %v", err)
	}

	// Board isolation: DeepSeek account shows attempts (peers proceeded);
	// MiniMax account is isolated with blocked attempts and no accounting.
	deadline := time.Now().Add(3 * time.Minute)
	var board teamBoardWire
	for {
		board = readTeamBoard(t, ctx, client, team.TeamInstanceID)
		deepAttempts := 0
		for _, row := range board.ProviderAccounts {
			if row.ProviderID == "deepseek" {
				deepAttempts += row.AttemptCount
			}
		}
		if deepAttempts > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	for _, row := range board.ProviderAccounts {
		t.Logf("isolation row %s/%s attempts=%d accounting=%d",
			row.ProviderID, row.ProviderAccountID, row.AttemptCount,
			row.AccountingAttemptCount)
		if row.ProviderID == "deepseek" && row.AttemptCount == 0 {
			t.Fatalf("DeepSeek peer had no attempts: %#v", board.ProviderAccounts)
		}
		if row.ProviderID == "minimax" && row.AttemptCount > 0 &&
			row.AccountingAttemptCount > 0 {
			t.Fatalf("MiniMax blocked attempts must carry no accounting: %#v", row)
		}
	}
}

// TestLiveRemoteToolEnrollmentIsolationE2E drives the G5 Web/MCP diagnostics
// gate installed-live: configure a web_search + mcp_server Enrollment under a
// verified account, bind web_search to one Agent, verify the frozen binding +
// the governance directory, revoke the Enrollment and confirm only the bound
// Agent's preflight changes while unbound peers stay healthy, and confirm the
// production default exposes no unbound remote capability. It is skipped
// unless LOOM_LIVE_TEAM_E2E=1.
func TestLiveRemoteToolEnrollmentIsolationE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_TEAM_E2E") != "1" {
		t.Skip("live team E2E gate requires LOOM_LIVE_TEAM_E2E=1")
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

	// 1. Configure web_search + mcp_server Enrollments under deepseek.primary.
	policy := providerPolicyLineage(t, ctx, client, "deepseek", "deepseek.primary")
	enrollTS := time.Now().UTC().Format("20060102150405")
	webSearchID := "enroll-web-live-" + enrollTS
	mcpID := "enroll-mcp-live-" + enrollTS
	var raw json.RawMessage
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
	}, &raw); err != nil {
		t.Fatalf("configure web_search: %v", err)
	}
	if err := client.Call(ctx, "remote_tool_backend_enrollment_configure", map[string]any{
		"enrollment_id": mcpID, "backend_kind": "mcp_server",
		"adapter_id":  "builtin.mcp.stdio.v1",
		"provider_id": "deepseek", "provider_account_id": "deepseek.primary",
		"provider_account_policy_version":  policy.Version,
		"provider_account_policy_revision": policy.Revision,
		"provider_account_policy_digest":   policy.Digest,
		"endpoint_fingerprint":             "948f1ecb6b48f91adc4e110d0351cd172b16450e9936d358992e0dfad7b863f3",
		"mcp_server_id":                    "mcp.live.probe", "allowed_tools": []string{"lookup"},
		"expected_revision": 0, "maximum_concurrent_calls": 2,
		"maximum_calls_per_attempt": 4, "timeout_seconds": 30,
		"maximum_result_bytes": 4096, "maximum_budget_units": 100000,
		"operation_id": matrixOperationID(),
	}, &raw); err != nil {
		t.Fatalf("configure mcp_server: %v", err)
	}

	// 2. The governance directory shows both Enrollments active + policy-current.
	enrollments := remoteToolEnrollments(t, ctx, client, "deepseek", "deepseek.primary")
	webDigest, webActive, webCurrent := enrollmentLineage(enrollments, webSearchID)
	if webDigest == "" || !webActive || !webCurrent {
		t.Fatalf("web_search enrollment not active/policy-current: %#v", enrollments)
	}
	if _, mcpActive, mcpCurrent := enrollmentLineage(enrollments, mcpID); !mcpActive || !mcpCurrent {
		t.Fatalf("mcp_server enrollment not active/policy-current: %#v", enrollments)
	}

	// 3. Build a minimal Team (deepseek main + one deepseek subagent), bind the
	//    web_search Enrollment to the subagent.
	roleOptions := refreshRoleOptions(t, ctx, client)
	session := startBlankSession(t, ctx, client)
	session = editSession(t, ctx, client, session, "team_name", "Live Enrollment Team")
	session = editSession(t, ctx, client, session, "purpose",
		"Verify remote tool enrollment isolation")
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
	definitionID := "team-live-enroll-" + time.Now().UTC().Format("20060102T150405Z")
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
		t.Fatal("enrollment team not materialized")
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer cleanupCancel()
		archiveLiveTeams(t, cleanupCtx, client, definitionID)
	})

	// 4. Preflight: the Enrollment-bound subagent resolves (valid + active).
	preflight, err := preflightMission(t, ctx, client, team, liveMissionE2EObjective)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	byProvider := map[string]string{}
	for _, node := range preflight.Preflight.Nodes {
		t.Logf("  node %s provider=%s status=%s block=%q",
			node.LogicalNodeID, node.ProviderID, node.Status, node.BlockReason)
		byProvider[node.LogicalNodeID] = node.Status
	}
	subagentStatus := ""
	for _, node := range preflight.Preflight.Nodes {
		if node.Role == "subagent" {
			subagentStatus = node.Status
		}
	}
	if subagentStatus == "" {
		t.Fatal("no subagent node in preflight")
	}

	// 5. Revoke the web_search Enrollment: only the bound Agent's preflight
	//    changes; the unbound main stays ready and the default production
	//    exposes no unbound remote capability.
	revReference, revRevision := remoteToolEnrollmentRevokeLineage(
		t, ctx, client, "deepseek", "deepseek.primary", webSearchID,
	)
	if err := client.Call(ctx, "remote_tool_backend_enrollment_revoke", map[string]any{
		"enrollment_id": webSearchID, "provider_id": "deepseek",
		"provider_account_id": "deepseek.primary",
		"expected_revision":   revRevision,
		"operation_id":        matrixOperationID(),
	}, &struct{}{}); err != nil {
		t.Fatalf("revoke web_search: %v", err)
	}
	_ = revReference

	// Wait for the projection to reflect the revocation, then re-preflight.
	time.Sleep(2 * time.Second)
	for _, enrollment := range remoteToolEnrollments(
		t, ctx, client, "deepseek", "deepseek.primary",
	) {
		t.Logf("post-revoke directory enrollment %s status=%s", enrollment.EnrollmentID, enrollment.Status)
	}
	after, err := preflightMission(t, ctx, client, team, liveMissionE2EObjective)
	if err != nil {
		t.Fatalf("post-revoke preflight: %v", err)
	}
	mainAfter := ""
	subAfter := ""
	subBlock := ""
	for _, node := range after.Preflight.Nodes {
		t.Logf("  post-revoke node %s provider=%s status=%s block=%q",
			node.LogicalNodeID, node.ProviderID, node.Status, node.BlockReason)
		if node.Role == "subagent" {
			subAfter = node.Status
			subBlock = node.BlockReason
		}
		if node.Role == "main" {
			mainAfter = node.Status
		}
	}
	if mainAfter != "ready" {
		t.Fatalf("unbound main affected by enrollment revoke: %q", mainAfter)
	}
	// Only the bound Agent must be affected by the revocation: it blocks with
	// the enrollment-scoped reason while the unbound main stays ready.
	if subAfter != "blocked" || !strings.Contains(subBlock, "enrollment") {
		t.Fatalf("bound subagent must block after revocation (was %q before revoke), got status=%q block=%q",
			subagentStatus, subAfter, subBlock)
	}
	t.Logf("bound subagent before=%q after=%q block=%q (isolation confirmed)",
		subagentStatus, subAfter, subBlock)
}

type providerPolicyLineageValue struct {
	Version  int    `json:"policy_version"`
	Revision int64  `json:"policy_revision"`
	Digest   string `json:"policy_digest"`
}

func providerPolicyLineage(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	providerID string,
	providerAccountID string,
) providerPolicyLineageValue {
	t.Helper()
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("setup_snapshot: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID        string `json:"provider_id"`
			ProviderAccountID string `json:"provider_account_id"`
			PolicyVersion     int    `json:"policy_version"`
			PolicyRevision    int64  `json:"policy_revision"`
			PolicyDigest      string `json:"policy_digest"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, account := range snap.Accounts {
		if account.ProviderID == providerID &&
			account.ProviderAccountID == providerAccountID &&
			account.PolicyRevision > 0 && account.PolicyDigest != "" {
			return providerPolicyLineageValue{
				Version:  account.PolicyVersion,
				Revision: account.PolicyRevision,
				Digest:   account.PolicyDigest,
			}
		}
	}
	t.Fatalf("policy lineage missing for %s/%s", providerID, providerAccountID)
	return providerPolicyLineageValue{}
}

type remoteToolEnrollmentLineage struct {
	Digest        string `json:"enrollment_digest"`
	Status        string `json:"status"`
	PolicyCurrent bool   `json:"policy_current"`
	Revision      int64  `json:"revision"`
	EnrollmentID  string `json:"enrollment_id"`
}

func remoteToolEnrollments(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	providerID string,
	providerAccountID string,
) []remoteToolEnrollmentLineage {
	t.Helper()
	raw, err := setupSnapshotEventually(t, ctx, client)
	if err != nil {
		t.Fatalf("setup_snapshot: %v", err)
	}
	var snap struct {
		Accounts []struct {
			ProviderID         string                        `json:"provider_id"`
			ProviderAccountID  string                        `json:"provider_account_id"`
			RemoteToolBackends []remoteToolEnrollmentLineage `json:"remote_tool_backends"`
		} `json:"provider_accounts"`
	}
	_ = json.Unmarshal(raw, &snap)
	for _, account := range snap.Accounts {
		if account.ProviderID == providerID &&
			account.ProviderAccountID == providerAccountID {
			return account.RemoteToolBackends
		}
	}
	return nil
}

func enrollmentLineage(
	enrollments []remoteToolEnrollmentLineage,
	enrollmentID string,
) (digest string, active bool, policyCurrent bool) {
	for _, enrollment := range enrollments {
		if enrollment.EnrollmentID == enrollmentID {
			return enrollment.Digest,
				enrollment.Status == "active",
				enrollment.PolicyCurrent
		}
	}
	return "", false, false
}

func remoteToolEnrollmentRevokeLineage(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	providerID string,
	providerAccountID string,
	enrollmentID string,
) (string, int64) {
	t.Helper()
	for _, enrollment := range remoteToolEnrollments(
		t, ctx, client, providerID, providerAccountID,
	) {
		if enrollment.EnrollmentID == enrollmentID {
			return enrollment.EnrollmentID, enrollment.Revision
		}
	}
	t.Fatalf("enrollment %s not found for revoke", enrollmentID)
	return "", 0
}
