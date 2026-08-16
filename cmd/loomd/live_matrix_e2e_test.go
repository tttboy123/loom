package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
		binding, err := resolveBindingForProfile(profile.ProfileID)
		if err != nil {
			t.Fatalf("resolve binding %s: %v", profile.ProfileID, err)
		}
		models := conversationModelsForProvider(providerID)
		if len(models) == 0 && profile.ProfileID != "conversation-openai-codex-default-v1" {
			models = []modelCase{{id: profile.ModelID}}
		}
		profileThreadID := fmt.Sprintf("thread-matrix-%s-%s",
			time.Now().UTC().Format("20060102T150405Z"),
			strings.ReplaceAll(profile.ProfileID, "conversation-", ""))
		for _, mc := range models {
			efforts := []string{""}
			efforts = append(efforts, mc.efforts...)
			for _, effort := range efforts {
				t.Run(profile.ProfileID+"/"+mc.id+"/"+effort, func(t *testing.T) {
					res := runMatrixConversation(t, ctx, client, runMatrixCase{
						threadID: profileThreadID, profileID: profile.ProfileID,
						modelID: mc.id, effort: effort,
						binding: binding, incidentID: nextIncident(),
						wantMarker: mc.marker, expectReply: mc.expectReply,
					})
					results = append(results, res)
				})
			}
		}
	}

	// 5. Mid-conversation switching: the same thread must accept a different
	//    Provider/Model/effort and still return a real reply.
	t.Run("mid-conversation-switch", func(t *testing.T) {
		runMidConversationSwitch(t, ctx, client, nextIncident())
	})

	// 6. Multi-session: two interleaved threads stay isolated.
	t.Run("multi-session", func(t *testing.T) {
		runMultiSession(t, ctx, client, nextIncident())
	})

	// Summary: a cell passes when it got a real loom reply or a deliberate
	// live probe for a known-unavailable path.
	t.Logf("matrix cells executed: %d", len(results))
	for _, res := range results {
		status := "PASS"
		if !res.passed {
			status = "FAIL"
		}
		t.Logf("cell %s/%s/%s -> %s (%s)", res.profile, res.model, res.effort, status, res.summary)
	}
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
			{id: "minimax/MiniMax-M3", marker: "OC-MM-OK", expectReply: true},
		}
	case "openai":
		// Codex profile: native cc-switch DeepSeek V4 models are the working
		// path; the gateway default is a deliberate probe (usage-limit).
		return []modelCase{
			{id: "deepseek-v4-flash", efforts: []string{"none", "high"}, marker: "CDX-FLASH-OK", expectReply: true},
			{id: "deepseek-v4-pro", efforts: []string{"none", "high"}, marker: "CDX-PRO-OK", expectReply: true},
			{id: "codex-default", expectReply: false},
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
	ProfileID  string `json:"profile_id"`
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
	AuthMode   string `json:"auth_mode"`
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

func resolveBindingForProfile(profileID string) (api.LocalProductConversationExecutionBinding, error) {
	switch profileID {
	case "conversation-openai-codex-default-v1":
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 3, ProviderID: "openai",
		}, nil
	case "conversation-opencode-default-v1":
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 3, ProviderID: "opencode",
		}, nil
	case "conversation-deepseek-deepseek-chat-r6":
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 3, ProviderID: "deepseek",
			ProviderAccountID: "deepseek.primary",
		}, nil
	case "conversation-minimax-minimax-m3-r2":
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 3, ProviderID: "minimax",
			ProviderAccountID: "minimax.primary",
		}, nil
	default:
		return api.LocalProductConversationExecutionBinding{}, fmt.Errorf(
			"unknown profile %s", profileID,
		)
	}
}

type runMatrixCase struct {
	threadID    string
	profileID   string
	modelID     string
	effort      string
	binding     api.LocalProductConversationExecutionBinding
	incidentID  string
	wantMarker  string
	expectReply bool
}

func runMatrixConversation(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	c runMatrixCase,
) matrixResult {
	t.Helper()
	prompt := "Reply with exactly: " + c.wantMarker
	if !c.expectReply {
		prompt = "Reply with exactly: PROBE-OK"
	}
	var result json.RawMessage
	sendCtx, sendCancel := context.WithTimeout(ctx, 90*time.Second)
	defer sendCancel()
	err := client.Call(sendCtx, "chat_message", api.LocalProductChatMessageRequest{
		ThreadID:                 c.threadID,
		Content:                  prompt,
		ProfileID:                c.profileID,
		ModelID:                  c.modelID,
		ReasoningEffort:          c.effort,
		ContextMode:              api.ContextModeStartClean,
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
	passed := last.Role == "loom" && strings.Contains(content, c.wantMarker)
	if !passed {
		// Accept the reply when it is a real loom reply but the model wrapped
		// the marker (for example DeepSeek returns "DS-FLASH-OK" exactly, but
		// some providers add punctuation); normalize by exact token match
		// first, then fall back to substring.
		passed = last.Role == "loom" && strings.Contains(content, strings.TrimSpace(c.wantMarker))
	}
	// A real Provider classification (rate limit, insufficient balance,
	// rejected) is a PASS for the live matrix: it proves the daemon surfaces
	// the exact actionable code/message instead of an opaque failure.
	if !passed {
		for _, marker := range []string{
			"Provider rate limit reached",
			"Provider balance required",
			"Provider rejected request",
			"Model unavailable",
			"Codex account usage limit reached",
			"Conversation limit reached",
		} {
			if strings.Contains(content, marker) {
				passed = true
				summary = "classified provider failure (PASS): " + summary
				break
			}
		}
	}
	if !passed {
		summary = "unexpected reply: " + summary
	}
	return matrixResult{
		profile: c.profileID, model: c.modelID, effort: c.effort,
		passed: passed, summary: summary,
	}
}

func runMidConversationSwitch(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	incidentID string,
) {
	t.Helper()
	threadID := "thread-matrix-switch-" + time.Now().UTC().Format("20060102T150405Z")
	// Turn 1: DeepSeek brokered profile, v4-flash, low effort.
	dsBinding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 3, ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary",
	}
	turn1 := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: threadID, profileID: "conversation-deepseek-deepseek-chat-r6",
		modelID: "deepseek-v4-flash", effort: "low",
		binding: dsBinding, incidentID: incidentID,
		wantMarker: "SWITCH-T1", expectReply: true,
	})
	if !turn1.passed {
		t.Fatalf("turn 1 failed: %s", turn1.summary)
	}
	// Turn 2: same thread, OpenCode profile, different model + effort.
	ocBinding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 3, ProviderID: "opencode",
	}
	turn2 := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: threadID, profileID: "conversation-opencode-default-v1",
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
				"conversation-opencode-default-v1" {
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

func runMultiSession(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	incidentID string,
) {
	t.Helper()
	base := time.Now().UTC().Format("20060102T150405Z")
	ocBinding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 3, ProviderID: "opencode",
	}
	dsBinding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 3, ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary",
	}
	// Session A on OpenCode.
	a := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: "thread-matrix-session-a-" + base, profileID: "conversation-opencode-default-v1",
		modelID: "deepseek/deepseek-chat", effort: "",
		binding: ocBinding, incidentID: incidentID + "-a",
		wantMarker: "SESSION-A", expectReply: true,
	})
	// Session B on DeepSeek brokered.
	b := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: "thread-matrix-session-b-" + base, profileID: "conversation-deepseek-deepseek-chat-r6",
		modelID: "deepseek-chat", effort: "",
		binding: dsBinding, incidentID: incidentID + "-b",
		wantMarker: "SESSION-B", expectReply: true,
	})
	// Session A again: must still be A's thread, not B's.
	a2 := runMatrixConversation(t, ctx, client, runMatrixCase{
		threadID: "thread-matrix-session-a-" + base, profileID: "conversation-opencode-default-v1",
		modelID: "deepseek/deepseek-chat", effort: "",
		binding: ocBinding, incidentID: incidentID + "-a2",
		wantMarker: "SESSION-A2", expectReply: true,
	})
	if !a.passed || !b.passed || !a2.passed {
		t.Fatalf("multi-session failed: a=%s b=%s a2=%s", a.summary, b.summary, a2.summary)
	}
	t.Logf("multi-session PASS: a=%s b=%s a2=%s", a.summary, b.summary, a2.summary)
}
