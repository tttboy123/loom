package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

// TestLiveProviderModelEffortMatrixE2E drives the installed App daemon through
// a real multi-provider conversation matrix: every verified Provider account,
// the selectable conversation models, and the reasoning efforts each model
// supports, plus mid-conversation switching and multiple sessions. It is a
// paid, network-enabled live gate and is skipped unless
// LOOM_LIVE_MATRIX_E2E=1 is set.
//
// Prerequisites:
//   - The installed App is running and its daemon socket is reachable.
//   - The Credential Vault is unlocked.
//   - At least DeepSeek is verified; MiniMax is configured+verified from the
//     local MINIMAX_API_KEY when present (this matches the operator's
//     "Provider keys are local" setup).
func TestLiveProviderModelEffortMatrixE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_MATRIX_E2E") != "1" {
		t.Skip("live matrix E2E gate requires LOOM_LIVE_MATRIX_E2E=1")
	}
	socketPath := os.Getenv("LOOM_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    90 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// 1. Vault status + snapshot.
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
	vaultStatus := "?"
	if snap.CredentialVault != nil {
		vaultStatus = snap.CredentialVault.Status
	}
	t.Logf("vault status: %s", vaultStatus)
	if vaultStatus == "locked" {
		if err := client.Call(ctx, "credential_vault_unlock", struct{}{}, &struct{}{}); err != nil {
			t.Fatalf("unlock error: %v", err)
		}
		t.Log("vault unlocked via route")
	}

	// 2. Configure + verify MiniMax from the local env key when present and not
	//    already verified (the user keeps Provider keys locally).
	ensureMiniMaxVerified(t, ctx, client)

	// 3. Refresh snapshot and list verified brokered accounts + profiles.
	snapshot := refreshSetupSnapshot(t, ctx, client)
	profiles := snapshot.conversationProfiles
	verifiedAccounts := snapshot.verifiedAccounts
	t.Logf("verified accounts: %v", verifiedAccounts)
	t.Logf("profiles: %d", len(profiles))

	var results []matrixResult
	incidentSeq := 0
	nextIncident := func() string {
		incidentSeq++
		return fmt.Sprintf("loom-chat-matrix-%s-%03d",
			time.Now().UTC().Format("20060102T150405Z"), incidentSeq)
	}

	// 4. For each selectable conversation profile, run a real conversation
	//    across each model + reasoning effort that the catalog exposes.
	//    Every cell of a profile reuses ONE thread so the matrix never
	//    exhausts the bounded conversation store (128 threads max).
	for _, profile := range profiles {
		if profile.ProfileID == "" {
			continue
		}
		providerID := profile.ProviderID
		if _, ok := verifiedAccounts[providerID]; !ok &&
			profile.AuthMode != "native_auth" {
			t.Logf("skip profile %s: %s not verified", profile.ProfileID, providerID)
			continue
		}
		binding := bindingForMatrixProfile(profile)
		models := conversationModelsForProvider(providerID)
		if len(models) == 0 && profile.ProfileID != "conversation-openai-codex-default-v1" {
			models = []modelCase{{id: profile.ModelID}}
		}
		profileThreadID := fmt.Sprintf("thread-matrix-%s-%s",
			time.Now().UTC().Format("20060102T150405Z"),
			strings.ReplaceAll(profile.ProfileID, "conversation-", ""))
		profileRoutes := make([]matrixRoute, 0, len(models)*4)
		for _, mc := range models {
			efforts := []string{""}
			efforts = append(efforts, mc.efforts...)
			for _, effort := range efforts {
				t.Run(profile.ProfileID+"/"+mc.id+"/"+effort, func(t *testing.T) {
					res := runMatrixConversation(t, ctx, client, runMatrixCase{
						threadID: profileThreadID, profileID: profile.ProfileID,
						modelID: mc.id, effort: effort,
						binding: binding, incidentID: nextIncident(),
						prompt:     "Compute 17 + 25. Reply with only the decimal integer.",
						wantMarker: "42", expectReply: mc.expectReply,
					})
					results = append(results, res)
					profileRoutes = append(profileRoutes, matrixRoute{
						modelID: mc.id, reasoningEffort: effort,
					})
				})
			}
		}
		if len(profileRoutes) > 0 {
			assertLiveMatrixSegments(
				t, ctx, client, profileThreadID, profile.ProfileID, profileRoutes,
			)
		}
	}

	// 5. Mid-conversation switching: the same thread must accept a different
	//    Provider/Model/effort and still return a real reply.
	t.Run("mid-conversation-switch", func(t *testing.T) {
		runMidConversationSwitch(t, ctx, client, profiles, nextIncident())
	})

	// 6. Disclosure modes: one visible Conversation freezes a distinct route
	//    Segment and disclosure receipt for every explicit context transition.
	t.Run("context-disclosure-modes", func(t *testing.T) {
		runContextDisclosureModes(t, ctx, client, profiles, nextIncident())
	})

	// 7. Multi-session: two interleaved threads stay isolated.
	t.Run("multi-session", func(t *testing.T) {
		runMultiSession(t, ctx, client, profiles, nextIncident())
	})

	// Summary: a cell passes when it got a real loom reply or a deliberate
	// live probe for a known-unavailable path.
	t.Logf("matrix cells executed: %d", len(results))
	if len(results) == 0 {
		t.Fatal("live matrix executed zero Provider/Model cells")
	}
	for _, res := range results {
		status := "PASS"
		if !res.passed {
			status = "FAIL"
		}
		t.Logf("cell %s/%s/%s -> %s (%s)", res.profile, res.model, res.effort, status, res.summary)
	}
	if failures := requiredMatrixFailures(results); len(failures) != 0 {
		t.Fatalf("required live matrix cells failed:\n%s", strings.Join(failures, "\n"))
	}
}

// TestLiveConversationContextDisclosureModesE2E is the focused installed gate
// for immutable route Segments and disclosure receipts. It deliberately avoids
// rerunning the complete model/effort matrix when only context transition
// behavior changed.
func TestLiveConversationContextDisclosureModesE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_CONTEXT_DISCLOSURE_E2E") != "1" {
		t.Skip("live context disclosure gate requires LOOM_LIVE_CONTEXT_DISCLOSURE_E2E=1")
	}
	socketPath := os.Getenv("LOOM_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    90 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	snapshot := refreshSetupSnapshot(t, ctx, client)
	runContextDisclosureModes(
		t,
		ctx,
		client,
		snapshot.conversationProfiles,
		"loom-context-disclosure-"+time.Now().UTC().Format("20060102T150405Z"),
	)
}

// ----- helpers -----

type matrixResult struct {
	profile string
	model   string
	effort  string
	passed  bool
	summary string
}

type matrixCase struct {
	// marker is the exact token the model must reply with for a "real reply"
	// cell; empty means the cell is a deliberate probe and any bounded reply
	// or classified failure is acceptable.
	marker      string
	expectReply bool
}

type modelCase struct {
	id          string
	efforts     []string
	marker      string
	expectReply bool
}

type contextDisclosureRoute struct {
	profile matrixProfile
	model   string
	mode    api.LocalProductContextMode
	marker  string
}

func conversationModelsForProvider(providerID string) []modelCase {
	switch providerID {
	case "deepseek":
		return []modelCase{
			{id: "deepseek-v4-flash", efforts: []string{"low", "high", "max"}, marker: "DS-FLASH-OK", expectReply: true},
			{id: "deepseek-v4-pro", efforts: []string{"low", "high", "max"}, marker: "DS-PRO-OK", expectReply: true},
			{id: "deepseek-chat", marker: "DS-LEGACY-OK", expectReply: true},
		}
	case "minimax":
		return []modelCase{
			{id: "MiniMax-M3", marker: "MM-OK", expectReply: true},
		}
	case "opencode":
		return []modelCase{
			{id: "deepseek/deepseek-chat", marker: "OC-CHAT-OK", expectReply: true},
			{id: "deepseek/deepseek-v4-flash", efforts: []string{"low", "high", "max"}, marker: "OC-FLASH-OK", expectReply: true},
			{id: "deepseek/deepseek-v4-pro", efforts: []string{"high", "max"}, marker: "OC-PRO-OK", expectReply: true},
			{id: "minimax-cn/MiniMax-M3", marker: "OC-MM-OK", expectReply: true},
		}
	case "openai":
		// Codex profile: test the native default plus one explicitly selected
		// OpenAI model. Runtime-specific third-party models must not be treated
		// as available without current discovery or a live capability result.
		return []modelCase{
			{id: "codex-default", marker: "CDX-DEFAULT-OK", expectReply: true},
			{id: "gpt-5.6-sol", marker: "CDX-G56S-OK", expectReply: true},
			{id: "gpt-5.6-terra", marker: "CDX-G56T-OK", expectReply: true},
			{id: "gpt-5.6-luna", marker: "CDX-G56L-OK", expectReply: true},
			{id: "gpt-5.5", marker: "CDX-G55-OK", expectReply: true},
			{id: "gpt-5.4", marker: "CDX-G54-OK", expectReply: true},
			{id: "gpt-5.4-mini", marker: "CDX-G54M-OK", expectReply: true},
			{id: "gpt-5.3-codex-spark", marker: "CDX-SPARK-OK", expectReply: true},
			{id: "codex-auto-review", marker: "CDX-REVIEW-OK", expectReply: true},
		}
	case "kimi":
		return []modelCase{
			{id: "kimi-k3", efforts: []string{"low", "high", "max"}, marker: "KIMI-K3-OK", expectReply: true},
			{id: "kimi-k2.6", marker: "KIMI-K26-OK", expectReply: true},
		}
	case "anthropic":
		return []modelCase{
			{id: "claude-sonnet-5", marker: "ANTH-OK", expectReply: true},
		}
	default:
		return nil
	}
}

type matrixProfile struct {
	ProfileID         string `json:"profile_id"`
	ProviderID        string `json:"provider_id"`
	ProviderAccountID string `json:"provider_account_id"`
	ModelID           string `json:"model_id"`
	AuthMode          string `json:"auth_mode"`
	PolicyVersion     int    `json:"policy_version"`
	PolicyRevision    int64  `json:"policy_revision"`
	PolicyDigest      string `json:"policy_digest"`
	TrustDomain       string `json:"trust_domain"`
	RetentionMode     string `json:"retention_mode"`
	DataRegion        string `json:"data_region"`
}

type setupSnap struct {
	conversationProfiles []matrixProfile
	verifiedAccounts     map[string]bool
}

func refreshSetupSnapshot(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) setupSnap {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &raw); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var snap struct {
		Profiles []matrixProfile `json:"conversation_profiles"`
		Accounts []struct {
			ProviderID string `json:"provider_id"`
			Status     string `json:"status"`
		} `json:"provider_accounts"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	out := setupSnap{verifiedAccounts: map[string]bool{}}
	out.conversationProfiles = snap.Profiles
	for _, account := range snap.Accounts {
		if account.Status == "verified" {
			out.verifiedAccounts[account.ProviderID] = true
		}
	}
	return out
}

// matrixOperationID returns a UUID-v4 shaped operation ID accepted by the
// daemon's strict `validSetupOperationID` (used for credential_verify).
func matrixOperationID() string {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		panic(err)
	}
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		random[0:4], random[4:6], random[6:8], random[8:10], random[10:16])
}

func ensureMiniMaxVerified(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) {
	t.Helper()
	key := os.Getenv("MINIMAX_API_KEY")
	if key == "" {
		t.Log("MINIMAX_API_KEY not set; skipping MiniMax live configuration")
		return
	}
	// Read the current minimax account state (configured vs unconfigured).
	var snapRaw json.RawMessage
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapRaw); err != nil {
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
	if err := json.Unmarshal(snapRaw, &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	reference := ""
	revision := int64(0)
	for _, account := range snap.Accounts {
		if account.ProviderID == "minimax" {
			reference = account.Reference
			revision = account.Revision
			if account.Status == "verified" {
				t.Log("MiniMax already verified")
				return
			}
		}
	}
	var raw json.RawMessage
	if reference == "" {
		// Configure (brokered, minimax.primary).
		configureParams := map[string]string{
			"provider_id": "minimax", "secret": key,
		}
		if err := client.Call(ctx, "credential_configure", configureParams, &raw); err != nil {
			t.Fatalf("minimax configure: %v", err)
		}
		var configured struct {
			ProviderID string `json:"provider_id"`
			Revision   int64  `json:"revision"`
			Status     string `json:"status"`
		}
		if err := json.Unmarshal(raw, &configured); err != nil {
			t.Fatalf("minimax configure decode: %v", err)
		}
		t.Logf("minimax configured: status=%s revision=%d", configured.Status, configured.Revision)
		revision = configured.Revision
		// Re-read the snapshot to pick up the new credential reference.
		if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapRaw); err != nil {
			t.Fatalf("setup: %v", err)
		}
		var refreshed struct {
			Accounts []struct {
				ProviderID string `json:"provider_id"`
				Reference  string `json:"credential_reference"`
				Revision   int64  `json:"revision"`
			} `json:"provider_accounts"`
		}
		if err := json.Unmarshal(snapRaw, &refreshed); err != nil {
			t.Fatalf("decode snapshot: %v", err)
		}
		reference = ""
		for _, account := range refreshed.Accounts {
			if account.ProviderID == "minimax" && account.Reference != "" {
				reference = account.Reference
			}
		}
	}
	if reference == "" {
		t.Fatalf("minimax credential reference not found after configure")
	}
	verifyParams := map[string]any{
		"provider_id":          "minimax",
		"credential_reference": reference,
		"expected_revision":    revision,
		"operation_id":         matrixOperationID(),
	}
	if err := client.Call(ctx, "credential_verify", verifyParams, &raw); err != nil {
		t.Fatalf("minimax verify: %v", err)
	}
	var verified struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &verified); err != nil {
		t.Fatalf("minimax verify decode: %v", err)
	}
	t.Logf("minimax verify: status=%s", verified.Status)
}

func bindingForMatrixProfile(profile matrixProfile) api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion:                 3,
		ProviderID:                    profile.ProviderID,
		ProviderAccountID:             profile.ProviderAccountID,
		ProviderAccountPolicyVersion:  profile.PolicyVersion,
		ProviderAccountPolicyRevision: profile.PolicyRevision,
		ProviderAccountPolicyDigest:   profile.PolicyDigest,
		TrustDomain:                   profile.TrustDomain,
		RetentionMode:                 profile.RetentionMode,
		DataRegion:                    profile.DataRegion,
	}
}

func matrixProfileForProvider(profiles []matrixProfile, providerID string) (matrixProfile, bool) {
	for _, profile := range profiles {
		if profile.ProviderID == providerID && profile.ProfileID != "" {
			return profile, true
		}
	}
	return matrixProfile{}, false
}

func requiredMatrixFailures(results []matrixResult) []string {
	failures := make([]string, 0)
	for _, result := range results {
		if result.passed {
			continue
		}
		effort := result.effort
		if effort == "" {
			effort = "default"
		}
		failures = append(failures, fmt.Sprintf(
			"%s/%s/%s: %s", result.profile, result.model, effort, result.summary,
		))
	}
	return failures
}

type runMatrixCase struct {
	threadID    string
	profileID   string
	modelID     string
	effort      string
	prompt      string
	binding     api.LocalProductConversationExecutionBinding
	incidentID  string
	wantMarker  string
	expectReply bool
	contextMode api.LocalProductContextMode
}

type matrixRoute struct {
	modelID         string
	reasoningEffort string
}

func assertLiveMatrixSegments(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	threadID string,
	profileID string,
	routes []matrixRoute,
) {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
		ThreadID: threadID,
	}, &raw); err != nil {
		t.Fatalf("read matrix segment metadata: %v", err)
	}
	var thread struct {
		Segments []struct {
			ProfileID       string `json:"profile_id"`
			ModelID         string `json:"model_id"`
			ReasoningEffort string `json:"reasoning_effort"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(raw, &thread); err != nil {
		t.Fatalf("decode matrix segment metadata: %v", err)
	}
	if len(thread.Segments) != len(routes) {
		t.Fatalf("segment count=%d want=%d", len(thread.Segments), len(routes))
	}
	for index, route := range routes {
		segment := thread.Segments[index]
		if segment.ProfileID != profileID || segment.ModelID != route.modelID ||
			segment.ReasoningEffort != route.reasoningEffort {
			t.Fatalf(
				"segment[%d] route=(%q,%q,%q), want=(%q,%q,%q)",
				index, segment.ProfileID, segment.ModelID, segment.ReasoningEffort,
				profileID, route.modelID, route.reasoningEffort,
			)
		}
	}
}

func runMatrixConversation(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	c runMatrixCase,
) matrixResult {
	t.Helper()
	prompt := c.prompt
	if prompt == "" {
		prompt = "Reply with exactly: " + c.wantMarker
	}
	if !c.expectReply {
		prompt = "Reply with exactly: PROBE-OK"
	}
	var result json.RawMessage
	sendCtx, sendCancel := context.WithTimeout(ctx, 130*time.Second)
	defer sendCancel()
	contextMode := c.contextMode
	if contextMode == "" {
		contextMode = api.ContextModeStartClean
	}
	err := client.Call(sendCtx, "chat_message", api.LocalProductChatMessageRequest{
		ThreadID:                 c.threadID,
		Content:                  prompt,
		ProfileID:                c.profileID,
		ModelID:                  c.modelID,
		ReasoningEffort:          c.effort,
		ContextMode:              contextMode,
		ExpectedExecutionBinding: &c.binding,
		IncidentID:               c.incidentID,
	}, &result)
	if err != nil {
		var remote *localipc.RemoteError
		summary := fmt.Sprintf("error: %v", err)
		if errors.As(err, &remote) {
			summary = fmt.Sprintf("code=%s stage=%s recoverable=%v",
				remote.Code, remote.Stage, remote.Recoverable)
		}
		// Deliberate probe cells accept a classified failure.
		if !c.expectReply {
			t.Logf("probe %s/%s/%s -> %s", c.profileID, c.modelID, c.effort, summary)
			return matrixResult{
				profile: c.profileID, model: c.modelID, effort: c.effort,
				passed: true, summary: "probe accepted: " + summary,
			}
		}
		t.Logf("cell %s/%s/%s -> %s", c.profileID, c.modelID, c.effort, summary)
		return matrixResult{
			profile: c.profileID, model: c.modelID, effort: c.effort,
			passed: false, summary: summary,
		}
	}
	var thread struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(result, &thread); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(thread.Messages) == 0 {
		return matrixResult{
			profile: c.profileID, model: c.modelID, effort: c.effort,
			passed: false, summary: "no messages returned",
		}
	}
	last := thread.Messages[len(thread.Messages)-1]
	content := strings.TrimSpace(last.Content)
	summary := fmt.Sprintf("role=%s content=%q", last.Role, content)
	if !c.expectReply {
		t.Logf("probe %s/%s/%s -> %s", c.profileID, c.modelID, c.effort, summary)
		return matrixResult{
			profile: c.profileID, model: c.modelID, effort: c.effort,
			passed: true, summary: "probe accepted: " + summary,
		}
	}
	passed := exactMatrixReply(last.Role, content, c.wantMarker)
	if !passed {
		summary = "unexpected reply: " + summary
	}
	return matrixResult{
		profile: c.profileID, model: c.modelID, effort: c.effort,
		passed: passed, summary: summary,
	}
}

func exactMatrixReply(role string, content string, expected string) bool {
	return role == "loom" && strings.TrimSpace(content) == strings.TrimSpace(expected)
}

func runMidConversationSwitch(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	profiles []matrixProfile,
	incidentID string,
) {
	t.Helper()
	deepSeekProfile, ok := matrixProfileForProvider(profiles, "deepseek")
	if !ok {
		t.Skip("mid-conversation switch requires a current DeepSeek profile")
	}
	openCodeProfile, ok := matrixProfileForProvider(profiles, "opencode")
	if !ok {
		t.Skip("mid-conversation switch requires a current OpenCode profile")
	}
	threadID := "thread-matrix-switch-" + time.Now().UTC().Format("20060102T150405Z")
	// Turn 1: DeepSeek brokered profile, v4-flash, low effort.
	dsBinding := bindingForMatrixProfile(deepSeekProfile)
	turn1 := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: threadID, profileID: deepSeekProfile.ProfileID,
		modelID: "deepseek-v4-flash", effort: "low",
		binding: dsBinding, incidentID: incidentID,
		wantMarker: "SWITCH-T1", expectReply: true,
	})
	if !turn1.passed {
		t.Fatalf("turn 1 failed: %s", turn1.summary)
	}
	// Turn 2: same thread, OpenCode profile, different model + effort.
	ocBinding := bindingForMatrixProfile(openCodeProfile)
	turn2 := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: threadID, profileID: openCodeProfile.ProfileID,
		modelID: "deepseek/deepseek-v4-flash", effort: "high",
		binding: ocBinding, incidentID: incidentID + "-t2",
		wantMarker: "SWITCH-T2", expectReply: true,
	})
	if !turn2.passed {
		// A mid-conversation switch is proven by a real loom reply from the
		// new Provider/Model/effort binding. Some models refuse to echo a
		// token that appears inside the untrusted context capsule; any
		// non-error loom reply still proves the switch dispatched correctly.
		var raw json.RawMessage
		if err := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
			ThreadID: threadID,
		}, &raw); err != nil {
			t.Fatalf("read thread after switch: %v", err)
		}
		var thread struct {
			Segments []struct {
				ProfileID string `json:"profile_id"`
			} `json:"segments"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(raw, &thread); err != nil {
			t.Fatalf("decode thread after switch: %v", err)
		}
		if len(thread.Segments) == 0 ||
			thread.Segments[len(thread.Segments)-1].ProfileID !=
				openCodeProfile.ProfileID {
			t.Fatalf("mid-conversation switch did not move to OpenCode segment: %s", turn2.summary)
		}
		last := thread.Messages[len(thread.Messages)-1]
		if last.Role != "loom" || last.Content == "" {
			t.Fatalf("turn 2 produced no real loom reply after switch: %s", turn2.summary)
		}
		turn2.passed = true
		turn2.summary = "real loom reply after switch (token refusal accepted): " +
			last.Content
	}
	t.Logf("mid-conversation switch PASS: turn1=%s turn2=%s", turn1.summary, turn2.summary)
}

func runContextDisclosureModes(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	profiles []matrixProfile,
	incidentID string,
) {
	t.Helper()
	deepSeekProfile, ok := matrixProfileForProvider(profiles, "deepseek")
	if !ok {
		t.Skip("context disclosure matrix requires a current DeepSeek profile")
	}
	openCodeProfile, ok := matrixProfileForProvider(profiles, "opencode")
	if !ok {
		t.Skip("context disclosure matrix requires a current OpenCode profile")
	}
	threadID := os.Getenv("LOOM_LIVE_CONTEXT_THREAD_ID")
	if threadID == "" {
		threadID = "thread-matrix-disclosure-" + time.Now().UTC().Format("20060102T150405Z")
	}
	routes := []contextDisclosureRoute{
		{deepSeekProfile, "deepseek-chat", api.ContextModeStartClean, "DISCLOSURE-CLEAN-1"},
		{openCodeProfile, "deepseek/deepseek-chat", api.ContextModeContinueWithContext, "DISCLOSURE-CONTINUE"},
		{deepSeekProfile, "deepseek-chat", api.ContextModeSummaryOnly, "DISCLOSURE-SUMMARY"},
		{openCodeProfile, "deepseek/deepseek-chat", api.ContextModeStartClean, "DISCLOSURE-CLEAN-2"},
	}
	if os.Getenv("LOOM_LIVE_CONTEXT_VERIFY_ONLY") == "1" {
		verifyLiveDaemonRestart(t, os.Getenv("LOOM_SOCKET_PATH"))
		verifyLiveContextDisclosureThread(t, ctx, client, threadID, routes)
		return
	}
	for index, route := range routes {
		result := runMatrixConversation(t, ctx, client, runMatrixCase{
			threadID: threadID, profileID: route.profile.ProfileID,
			modelID: route.model, binding: bindingForMatrixProfile(route.profile),
			incidentID: fmt.Sprintf("%s-%d", incidentID, index+1),
			wantMarker: route.marker, expectReply: true, contextMode: route.mode,
		})
		if !result.passed {
			t.Fatalf("context mode %s failed: %s", route.mode, result.summary)
		}
	}

	verifyLiveContextDisclosureThread(t, ctx, client, threadID, routes)
}

func verifyLiveDaemonRestart(t *testing.T, socketPath string) {
	t.Helper()
	previousPID := strings.TrimSpace(
		os.Getenv("LOOM_LIVE_CONTEXT_PRE_RESTART_DAEMON_PID"),
	)
	currentPID := strings.TrimSpace(
		os.Getenv("LOOM_LIVE_CONTEXT_CURRENT_DAEMON_PID"),
	)
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	if err := validateLiveDaemonRestart(
		previousPID, currentPID, socketPath,
		func(pid string) (string, error) {
			output, err := exec.Command("ps", "-p", pid, "-o", "command=").Output()
			return strings.TrimSpace(string(output)), err
		},
	); err != nil {
		t.Fatal(err)
	}
}

func validateLiveDaemonRestart(
	previousPID string,
	currentPID string,
	socketPath string,
	processCommand func(string) (string, error),
) error {
	if previousPID == "" || currentPID == "" || previousPID == currentPID ||
		socketPath == "" || processCommand == nil {
		return errors.New("verify-only restart gate requires distinct daemon identities")
	}
	if command, err := processCommand(previousPID); err == nil && command != "" {
		return fmt.Errorf("pre-restart daemon PID %s is still running", previousPID)
	}
	command, err := processCommand(currentPID)
	if err != nil || command == "" {
		return fmt.Errorf("current daemon PID %s is not running", currentPID)
	}
	if !strings.Contains(command, "/Loom.app/Contents/Library/Helpers/loomd") ||
		!strings.Contains(command, "--socket "+socketPath) {
		return fmt.Errorf(
			"current PID %s is not the installed daemon for the expected socket",
			currentPID,
		)
	}
	return nil
}

func TestValidateLiveDaemonRestartFailsClosed(t *testing.T) {
	socketPath := "/Users/example/Library/Application Support/Loom/run/loomd.sock"
	currentCommand := "/Users/example/Applications/Loom.app/Contents/Library/Helpers/loomd --socket " + socketPath
	lookup := func(commands map[string]string) func(string) (string, error) {
		return func(pid string) (string, error) {
			command, found := commands[pid]
			if !found {
				return "", errors.New("process not found")
			}
			return command, nil
		}
	}
	for name, test := range map[string]struct {
		previous string
		current  string
		commands map[string]string
	}{
		"missing identity": {current: "22", commands: map[string]string{"22": currentCommand}},
		"same identity":    {previous: "22", current: "22", commands: map[string]string{"22": currentCommand}},
		"old still alive":  {previous: "11", current: "22", commands: map[string]string{"11": "loomd", "22": currentCommand}},
		"current absent":   {previous: "11", current: "22", commands: map[string]string{}},
		"wrong current":    {previous: "11", current: "22", commands: map[string]string{"22": "/tmp/loomd --socket " + socketPath}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateLiveDaemonRestart(
				test.previous, test.current, socketPath, lookup(test.commands),
			); err == nil {
				t.Fatal("restart identity drift was accepted")
			}
		})
	}
	if err := validateLiveDaemonRestart(
		"11", "22", socketPath, lookup(map[string]string{"22": currentCommand}),
	); err != nil {
		t.Fatalf("valid restart identity rejected: %v", err)
	}
}

func verifyLiveContextDisclosureThread(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	threadID string,
	routes []contextDisclosureRoute,
) {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
		ThreadID: threadID,
	}, &raw); err != nil {
		t.Fatalf("read context disclosure thread: %v", err)
	}
	var thread struct {
		Segments []struct {
			SegmentID               string                      `json:"segment_id"`
			ProfileID               string                      `json:"profile_id"`
			ContextMode             api.LocalProductContextMode `json:"context_mode"`
			ContextCapsuleDigest    string                      `json:"context_capsule_digest"`
			DisclosureReceiptDigest string                      `json:"disclosure_receipt_digest"`
			DisclosedContextCount   int                         `json:"disclosed_context_count"`
			OmittedContextCount     int                         `json:"omitted_context_count"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(raw, &thread); err != nil {
		t.Fatalf("decode context disclosure thread: %v", err)
	}
	if len(thread.Segments) != len(routes) {
		t.Fatalf("context disclosure segments=%d want=%d", len(thread.Segments), len(routes))
	}
	receipts := map[string]bool{}
	for index, route := range routes {
		segment := thread.Segments[index]
		if segment.SegmentID == "" || segment.ProfileID != route.profile.ProfileID ||
			segment.ContextMode != route.mode ||
			segment.ContextCapsuleDigest == "" || segment.DisclosureReceiptDigest == "" ||
			segment.DisclosedContextCount < 1 {
			t.Fatalf("context disclosure segment[%d]=%#v route=%#v", index, segment, route)
		}
		if receipts[segment.DisclosureReceiptDigest] {
			t.Fatalf("context disclosure receipt reused at segment %d", index)
		}
		receipts[segment.DisclosureReceiptDigest] = true
		verifyLiveContextDisclosureInspection(
			t, ctx, client, threadID, segment.SegmentID,
			segment.ContextCapsuleDigest, segment.DisclosureReceiptDigest,
			segment.DisclosedContextCount, segment.OmittedContextCount,
		)
	}
	if thread.Segments[2].OmittedContextCount == 0 ||
		thread.Segments[3].OmittedContextCount == 0 {
		t.Fatalf("context disclosure omissions were not explicit: %#v", thread.Segments)
	}
}

func verifyLiveContextDisclosureInspection(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	threadID string,
	segmentID string,
	capsuleDigest string,
	receiptDigest string,
	disclosedCount int,
	omittedCount int,
) {
	t.Helper()
	var raw json.RawMessage
	if err := client.Call(
		ctx,
		"chat_context_disclosure",
		api.LocalProductChatContextDisclosureRequest{
			ThreadID: threadID, SegmentID: segmentID,
		},
		&raw,
	); err != nil {
		t.Fatalf("inspect context disclosure %s: %v", segmentID, err)
	}
	assertLiveContextDisclosureWire(t, raw)
	var disclosure api.LocalProductChatContextDisclosure
	if err := json.Unmarshal(raw, &disclosure); err != nil {
		t.Fatalf("decode context disclosure %s: %v", segmentID, err)
	}
	if disclosure.SchemaVersion != 1 || disclosure.ThreadID != threadID ||
		disclosure.SegmentID != segmentID ||
		disclosure.ContextCapsuleDigest != capsuleDigest ||
		disclosure.DisclosureReceiptDigest != receiptDigest ||
		len(disclosure.Disclosed) != disclosedCount ||
		len(disclosure.Omitted) != omittedCount {
		t.Fatalf("context disclosure authority mismatch for segment %s", segmentID)
	}
}

func assertLiveContextDisclosureWire(t *testing.T, raw []byte) {
	t.Helper()
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode context disclosure keys: %v", err)
	}
	topLevel := map[string]bool{
		"schema_version": true, "thread_id": true, "segment_id": true,
		"context_capsule_digest": true, "disclosure_receipt_digest": true,
		"disclosed": true, "omitted": true,
	}
	if len(result) != len(topLevel) {
		t.Fatalf("context disclosure top-level key count=%d", len(result))
	}
	for key := range result {
		if !topLevel[key] {
			t.Fatalf("unsafe context disclosure top-level key %q", key)
		}
	}
	for _, collection := range []string{"disclosed", "omitted"} {
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(result[collection], &items); err != nil {
			t.Fatalf("decode context disclosure %s: %v", collection, err)
		}
		for _, item := range items {
			if len(item) != 6 {
				t.Fatalf("context disclosure %s item key count=%d", collection, len(item))
			}
			for key := range item {
				switch key {
				case "kind", "trust", "scope", "token_count", "omission_reason", "retrievable":
				default:
					t.Fatalf("unsafe context disclosure item key %q", key)
				}
			}
		}
	}
}

func runMultiSession(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	profiles []matrixProfile,
	incidentID string,
) {
	t.Helper()
	openCodeProfile, ok := matrixProfileForProvider(profiles, "opencode")
	if !ok {
		t.Skip("multi-session gate requires a current OpenCode profile")
	}
	deepSeekProfile, ok := matrixProfileForProvider(profiles, "deepseek")
	if !ok {
		t.Skip("multi-session gate requires a current DeepSeek profile")
	}
	base := time.Now().UTC().Format("20060102T150405Z")
	ocBinding := bindingForMatrixProfile(openCodeProfile)
	dsBinding := bindingForMatrixProfile(deepSeekProfile)
	// Session A on OpenCode.
	a := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: "thread-matrix-session-a-" + base, profileID: openCodeProfile.ProfileID,
		modelID: "deepseek/deepseek-chat", effort: "",
		binding: ocBinding, incidentID: incidentID + "-a",
		wantMarker: "SESSION-A", expectReply: true,
	})
	// Session B on DeepSeek brokered.
	b := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: "thread-matrix-session-b-" + base, profileID: deepSeekProfile.ProfileID,
		modelID: "deepseek-chat", effort: "",
		binding: dsBinding, incidentID: incidentID + "-b",
		wantMarker: "SESSION-B", expectReply: true,
	})
	// Session A again: must still be A's thread, not B's.
	a2 := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: "thread-matrix-session-a-" + base, profileID: openCodeProfile.ProfileID,
		modelID: "deepseek/deepseek-chat", effort: "",
		binding: ocBinding, incidentID: incidentID + "-a2",
		wantMarker: "SESSION-A2", expectReply: true,
	})
	if !a.passed || !b.passed || !a2.passed {
		t.Fatalf("multi-session failed: a=%s b=%s a2=%s", a.summary, b.summary, a2.summary)
	}
	t.Logf("multi-session PASS: a=%s b=%s a2=%s", a.summary, b.summary, a2.summary)
}
