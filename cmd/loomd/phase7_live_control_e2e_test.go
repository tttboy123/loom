package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/roundtable"
)

const (
	phase7LiveIdentityPreflightGate    = "LOOM_PHASE7_IDENTITY_PREFLIGHT"
	phase7LiveCodexProbeGate           = "LOOM_PHASE7_CODEX_PROBE"
	phase7LiveOpenCodeProbeGate        = "LOOM_PHASE7_OPENCODE_PROBE"
	phase7LiveLoomNativeProbeGate      = "LOOM_PHASE7_LOOM_NATIVE_PROBE"
	phase7LivePiProbeGate              = "LOOM_PHASE7_PI_PROBE"
	phase7LiveControlGate              = "LOOM_PHASE7_CONTROL_E2E"
	phase7LivePaidGate                 = "LOOM_PHASE7_ALLOW_PAID_REQUESTS"
	phase7LiveRestartGate              = "LOOM_PHASE7_ALLOW_APP_RESTART"
	phase7LiveExpectedBuildGate        = "LOOM_PHASE7_EXPECTED_BUILD"
	phase7LiveExpectedAppDigestGate    = "LOOM_PHASE7_EXPECTED_APP_SHA256"
	phase7LiveExpectedDaemonDigestGate = "LOOM_PHASE7_EXPECTED_DAEMON_SHA256"
	phase7LiveOpenCodeProviderGate     = "LOOM_PHASE7_OPENCODE_PROVIDER"
	phase7LiveLoomNativeProviderGate   = "LOOM_PHASE7_LOOM_NATIVE_PROVIDER"
	phase7LiveAvailableControlGate     = "LOOM_PHASE7_AVAILABLE_RUNTIME_CONTROL_E2E"
	phase7LiveClaudeWaiverGate         = "LOOM_PHASE7_CLAUDE_LIVE_WAIVER"
	phase7LiveThreadDiagnosticGate     = "LOOM_PHASE7_THREAD_DIAGNOSTIC"
	phase7LiveExpectedProfileGate      = "LOOM_PHASE7_EXPECTED_PROFILE"
	phase7LiveDiagnosticCancelGate     = "LOOM_PHASE7_DIAGNOSTIC_CANCEL"
	phase7LiveGovernanceMissionGate    = "LOOM_PHASE7_GOVERNANCE_MISSION_ID"
	phase7LiveGovernanceTeamGate       = "LOOM_PHASE7_GOVERNANCE_TEAM_ID"
)

var (
	errPhase7InstalledDaemonNotFound = errors.New("installed managed daemon not found")
	errPhase7ObserverModelsWarmup    = errors.New("observer models warmup incomplete")
	errPhase7PrivacySnapshotChanged  = errors.New("installed privacy snapshot changed")
)

const (
	phase7PrivacyScanMaximumFiles = 8_192
	phase7PrivacyScanMaximumBytes = int64(2 << 30)
	phase7PrivateContentPrefix    = "P7.PRIVATE.CONTENT."
)

const phase7LiveRoundTablePrompt = "Perform a detailed, evidence-focused review of this " +
	"isolated Phase 7 governance acceptance fixture. Do not access credentials or user content."

type phase7ExpectedInstalledIdentity struct {
	Build               string
	AppExecutableSHA256 string
	DaemonSHA256        string
}

// TestLivePhase7CodexTeamSearchE2E is a read-only installed probe for the
// Codex arbitration path. Three independent Conversations must each select and
// complete the exact Agent Team search Tool; no Proposal or product mutation is
// permitted.
func TestLivePhase7CodexTeamSearchE2E(t *testing.T) {
	if os.Getenv(phase7LiveCodexProbeGate) != "1" {
		t.Skip("Phase 7 Codex probe requires " + phase7LiveCodexProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "codex", "openai")
	for sequence := 1; sequence <= 3; sequence++ {
		threadID := fmt.Sprintf(
			"p7-codex-team-%x%x-%d", time.Now().UTC().UnixNano(), os.Getpid(), sequence,
		)
		request := api.LocalProductChatMessageRequest{
			ThreadID: threadID,
			Content: "请实际使用 Loom 的 Agent Team 搜索能力查找 P7。只执行这一项只读查询，" +
				"即使没有结果也不要猜测或创建提案。",
			ProfileID: profile.ProfileID, ModelID: profile.ModelID,
			ContextMode:              api.ContextModeStartClean,
			ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
		}
		var thread api.LocalProductChatThread
		if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
			t.Fatalf("Codex Team search probe %d: %v", sequence, err)
		}
		if failure := phase7LiveThreadFailure(thread); failure != "" {
			t.Fatalf("Codex Team search probe %d: %s", sequence, failure)
		}
		requirePhase7CompletedTools(t, thread, controltool.ToolTeamsSearch)
		requirePhase7LiveProposalToolSet(t, thread, nil)
	}
	t.Logf(
		"Codex installed Team search arbitration passed 3/3: profile=%s model=%s",
		profile.ProfileID, profile.ModelID,
	)
}

// TestLivePhase7CodexTextE2E proves the same installed arbitration envelope
// preserves ordinary Conversation replies without completing a Loom Tool.
func TestLivePhase7CodexTextE2E(t *testing.T) {
	if os.Getenv(phase7LiveCodexProbeGate) != "1" {
		t.Skip("Phase 7 Codex probe requires " + phase7LiveCodexProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "codex", "openai")
	threadID := fmt.Sprintf("p7-codex-text-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID:  threadID,
		Content:   "Reply with exactly: Loom-P7-CODEX-OK. Do not call a tool.",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatal(err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatal(failure)
	}
	if len(thread.Messages) == 0 ||
		!strings.Contains(thread.Messages[len(thread.Messages)-1].Content, "Loom-P7-CODEX-OK") {
		t.Fatal("Codex direct reply did not return the bounded marker")
	}
	requirePhase7CompletedTools(t, thread)
	if len(thread.Attempts) == 0 ||
		len(thread.Attempts[len(thread.Attempts)-1].CompletedControlTools) != 0 {
		t.Fatal("Codex ordinary reply unexpectedly completed a Loom Tool")
	}
	requirePhase7LiveProposalToolSet(t, thread, nil)
	t.Logf("Codex installed direct arbitration passed: profile=%s model=%s", profile.ProfileID, profile.ModelID)
}

// TestLivePhase7CodexMissionProposalE2E isolates Codex mutation arbitration
// from the full Runtime matrix. It creates one governed review Proposal and
// cancels it; confirmation and product execution are intentionally excluded.
func TestLivePhase7CodexMissionProposalE2E(t *testing.T) {
	if os.Getenv(phase7LiveCodexProbeGate) != "1" {
		t.Skip("Phase 7 Codex probe requires " + phase7LiveCodexProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "codex", "openai")
	runID := fmt.Sprintf("codex-probe-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	record := preparePhase7LiveMissionProposal(
		t, ctx, client, runID, "cancel", profile,
	)
	thread := decidePhase7LiveProposal(
		t, ctx, client, record, api.ControlDecisionCancel,
	)
	requirePhase7ProposalStatus(
		t, thread, record.proposal.ProposalID, controltool.ProposalCancelled,
	)
	requirePhase7DecisionReceipt(
		t, thread, record.proposal, controltool.ProposalDecisionCancel,
	)
	t.Logf(
		"Codex installed Mission Proposal arbitration passed: profile=%s model=%s",
		profile.ProfileID, profile.ModelID,
	)
}

// TestLivePhase7OpenCodeControlToolE2E is a one-turn installed probe used
// before the full fifteen-turn matrix. It creates only a review Proposal and
// cancels it; no Mission, Team or credential state is changed.
func TestLivePhase7OpenCodeControlToolE2E(t *testing.T) {
	if os.Getenv(phase7LiveOpenCodeProbeGate) != "1" {
		t.Skip("Phase 7 OpenCode probe requires " + phase7LiveOpenCodeProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveOpenCodeProfile(t, ctx, client)
	runID := fmt.Sprintf("probe-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	record := preparePhase7LiveMissionProposal(
		t, ctx, client, runID, "cancel", profile,
	)
	thread := decidePhase7LiveProposal(
		t, ctx, client, record, api.ControlDecisionCancel,
	)
	requirePhase7ProposalStatus(
		t, thread, record.proposal.ProposalID, controltool.ProposalCancelled,
	)
	requirePhase7DecisionReceipt(
		t, thread, record.proposal, controltool.ProposalDecisionCancel,
	)
	t.Logf(
		"OpenCode installed probe passed: profile=%s provider=%s account=%s model=%s",
		profile.ProfileID, profile.ProviderID, profile.ProviderAccountID, profile.ModelID,
	)
}

// TestLivePhase7OpenCodeTextE2E separates basic Provider dispatch from MCP
// tool selection while using the same installed, account-bound OpenCode Route.
func TestLivePhase7OpenCodeTextE2E(t *testing.T) {
	if os.Getenv(phase7LiveOpenCodeProbeGate) != "1" {
		t.Skip("Phase 7 OpenCode probe requires " + phase7LiveOpenCodeProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveOpenCodeProfile(t, ctx, client)
	threadID := fmt.Sprintf("p7-text-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID:  threadID,
		Content:   "Reply with exactly: Loom-P7-OPENCODE-OK. Do not call a tool.",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatalf("OpenCode text model turn: %v", err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatalf("OpenCode text model turn: %s", failure)
	}
	if len(thread.Messages) == 0 ||
		!strings.Contains(thread.Messages[len(thread.Messages)-1].Content, "Loom-P7-OPENCODE-OK") {
		t.Fatal("OpenCode text model turn did not return the bounded synthetic marker")
	}
	t.Logf(
		"OpenCode installed text probe passed: profile=%s provider=%s account=%s model=%s",
		profile.ProfileID, profile.ProviderID, profile.ProviderAccountID, profile.ModelID,
	)
}

// TestLivePhase7OpenCodeRuntimeStatusE2E isolates the installed read-only
// control Gateway from Proposal creation and terminal-Proposal cancellation.
func TestLivePhase7OpenCodeRuntimeStatusE2E(t *testing.T) {
	if os.Getenv(phase7LiveOpenCodeProbeGate) != "1" {
		t.Skip("Phase 7 OpenCode probe requires " + phase7LiveOpenCodeProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveOpenCodeProfile(t, ctx, client)
	threadID := fmt.Sprintf("p7-runtime-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID: threadID,
		Content: "Use the Loom Runtime status capability exactly once. " +
			"Return the observed status and do not prepare or execute a Proposal.",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatalf("OpenCode Runtime status turn: %v", err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatalf("OpenCode Runtime status turn: %s", failure)
	}
	requirePhase7CompletedTools(t, thread, controltool.ToolRuntimesStatus)
	requirePhase7LiveProposalToolSet(t, thread, nil)
	t.Logf(
		"OpenCode installed Runtime status passed: profile=%s provider=%s account=%s model=%s",
		profile.ProfileID, profile.ProviderID, profile.ProviderAccountID, profile.ModelID,
	)
}

// TestLivePhase7LoomNativeControlToolE2E is the one-turn Provider tool-call
// probe for Loom Native. It requires an explicit Provider selector, prepares
// one governed Mission Proposal and cancels it without executing product work.
func TestLivePhase7LoomNativeControlToolE2E(t *testing.T) {
	if os.Getenv(phase7LiveLoomNativeProbeGate) != "1" {
		t.Skip("Phase 7 Loom Native probe requires " + phase7LiveLoomNativeProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	providerID := strings.TrimSpace(os.Getenv(phase7LiveLoomNativeProviderGate))
	if providerID == "" {
		t.Fatal("Loom Native probe requires an explicit " + phase7LiveLoomNativeProviderGate)
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "loom-native", providerID)
	runID := fmt.Sprintf("native-probe-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	record := preparePhase7LiveMissionProposal(
		t, ctx, client, runID, "cancel", profile,
	)
	thread := decidePhase7LiveProposal(
		t, ctx, client, record, api.ControlDecisionCancel,
	)
	requirePhase7ProposalStatus(
		t, thread, record.proposal.ProposalID, controltool.ProposalCancelled,
	)
	requirePhase7DecisionReceipt(
		t, thread, record.proposal, controltool.ProposalDecisionCancel,
	)
	t.Logf(
		"Loom Native installed probe passed: profile=%s provider=%s account=%s model=%s",
		profile.ProfileID, profile.ProviderID, profile.ProviderAccountID, profile.ModelID,
	)
}

// TestLivePhase7LoomNativeNeedsYouE2E isolates the read-only governance Tool
// from the longer capability matrix. It performs no Proposal or product-state
// mutation and is still gated as a paid installed Provider request.
func TestLivePhase7LoomNativeNeedsYouE2E(t *testing.T) {
	if os.Getenv(phase7LiveLoomNativeProbeGate) != "1" {
		t.Skip("Phase 7 Loom Native probe requires " + phase7LiveLoomNativeProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	providerID := strings.TrimSpace(os.Getenv(phase7LiveLoomNativeProviderGate))
	if providerID == "" {
		t.Fatal("Loom Native probe requires an explicit " + phase7LiveLoomNativeProviderGate)
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "loom-native", providerID)
	threadID := fmt.Sprintf("p7-needs-you-probe-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID:  threadID,
		Content:   "请实际使用 Loom 能力列出当前 Needs You 中等待用户处理的事项。只执行这一项只读查询，不要创建提案。",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatalf("Loom Native Needs You model turn: %v", err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatalf("Loom Native Needs You model turn: %s", failure)
	}
	requirePhase7CompletedTools(t, thread, controltool.ToolGovernanceNeedsYou)
	if len(thread.ControlProposals) != 0 || len(thread.ActionProposals) != 0 {
		t.Fatal("read-only Needs You probe created a Proposal")
	}
	t.Logf(
		"Loom Native Needs You probe passed: profile=%s provider=%s account=%s model=%s",
		profile.ProfileID, profile.ProviderID, profile.ProviderAccountID, profile.ModelID,
	)
}

// TestLivePhase7LoomNativeMiniMaxTextE2E distinguishes a Provider Account
// failure from an OpenCode-specific adaptation failure without changing any
// credential or governed product state.
func TestLivePhase7LoomNativeMiniMaxTextE2E(t *testing.T) {
	if os.Getenv(phase7LiveLoomNativeProbeGate) != "1" {
		t.Skip("Phase 7 Loom Native probe requires " + phase7LiveLoomNativeProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	requirePhase7LiveProfileAbsent(t, ctx, client, "opencode", "minimax")
	profile := requirePhase7LiveExactProfile(t, ctx, client, "loom-native", "minimax")
	threadID := fmt.Sprintf("p7-native-minimax-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID:  threadID,
		Content:   "Reply with exactly: Loom-P7-MINIMAX-OK.",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatalf("Loom Native MiniMax text model turn: %v", err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatalf("Loom Native MiniMax text model turn: %s", failure)
	}
	if len(thread.Messages) == 0 ||
		!strings.Contains(thread.Messages[len(thread.Messages)-1].Content, "Loom-P7-MINIMAX-OK") {
		t.Fatal("Loom Native MiniMax text model turn did not return the bounded marker")
	}
}

// TestLivePhase7PiTextE2E separates the local model/RPC path from Pi's
// governed control extension. It does not call a Tool or mutate product state.
func TestLivePhase7PiTextE2E(t *testing.T) {
	if os.Getenv(phase7LivePiProbeGate) != "1" {
		t.Skip("Phase 7 Pi probe requires " + phase7LivePiProbeGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "pi", "loom-local")
	threadID := fmt.Sprintf("p7-pi-text-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID: threadID, Content: "Reply with exactly: Loom-P7-PI-OK.",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatalf("Pi text model turn: %v", err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatalf("Pi text model turn: %s", failure)
	}
	if len(thread.Messages) == 0 ||
		!strings.Contains(thread.Messages[len(thread.Messages)-1].Content, "Loom-P7-PI-OK") {
		t.Fatal("Pi text model turn did not return the bounded marker")
	}
}

// TestLivePhase7PiRouteControlToolE2E proves the installed Product path keeps
// the user's exact Route Profile authoritative over Context Capsule metadata.
// It cancels the resulting Proposal before returning.
func TestLivePhase7PiRouteControlToolE2E(t *testing.T) {
	if os.Getenv(phase7LivePiProbeGate) != "1" {
		t.Skip("Phase 7 Pi Route probe requires " + phase7LivePiProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "pi", "loom-local")
	threadID := fmt.Sprintf("p7-pi-route-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID: threadID,
		Content: "请实际使用 Loom 能力，准备把当前会话 Route 选择为 " + profile.ProfileID +
			" 的审阅提案。只准备这一项，不要执行。",
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatal(err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatal(failure)
	}
	requirePhase7CompletedTools(
		t, thread, controltool.ToolConversationRouteChangePreview,
	)
	requirePhase7LiveProposalToolSet(
		t, thread, []controltool.ToolID{controltool.ToolConversationRouteChangePreview},
	)
	requirePhase7LiveProposalTargets(t, thread, phase7LiveCapabilityFixture{
		profileID: profile.ProfileID,
	}, profile.HarnessAdapter, "conversation-route-change-preview")
	thread = cancelPhase7LiveSweepProposals(t, ctx, client, thread)
	requirePhase7SweepProposalsCancelled(
		t, thread, []controltool.ToolID{controltool.ToolConversationRouteChangePreview},
	)
	t.Logf("installed Pi Route Proposal bound exact profile and was cancelled: %s", profile.ProfileID)
}

// TestLivePhase7PiRoundTableSkipProposalE2E isolates the Pi argument stage for
// three similar RoundTable identifiers. The exact installed Session, Round and
// Seat must survive model selection and reach one cancelled review Proposal.
func TestLivePhase7PiRoundTableSkipProposalE2E(t *testing.T) {
	if os.Getenv(phase7LivePiProbeGate) != "1" {
		t.Skip("Phase 7 Pi probe requires " + phase7LivePiProbeGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	profile := requirePhase7LiveExactProfile(t, ctx, client, "pi", "loom-local")
	anchor := requirePhase7GovernanceAnchor(
		t, requirePhase7LiveProductSnapshot(t, ctx, client),
	)
	fixture, _ := createPhase7LiveRoundTableFixture(
		t, ctx, client, "pi-skip", anchor,
	)
	view, err := pausePhase7LiveRoundTableFixture(ctx, client, fixture)
	if err != nil {
		t.Fatal(err)
	}
	target, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesSkipPreview,
	)
	if err != nil {
		t.Fatal(err)
	}
	threadID := fmt.Sprintf("p7-pi-roundtable-skip-%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	request := api.LocalProductChatMessageRequest{
		ThreadID: threadID,
		Content: fmt.Sprintf(
			"请实际使用 Loom 能力，为 RoundTable Session %s、Round %s 的 Seat %s 准备 skip 审阅提案。只准备这一项，不要执行。",
			target.roundSessionID, target.roundID, target.seatID,
		),
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		t.Fatal(err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatal(failure)
	}
	requirePhase7CompletedTools(t, thread, controltool.ToolRoundtablesSkipPreview)
	requirePhase7LiveProposalToolSet(
		t, thread, []controltool.ToolID{controltool.ToolRoundtablesSkipPreview},
	)
	requirePhase7LiveProposalTargets(t, thread, phase7LiveCapabilityFixture{
		roundSessionID: target.roundSessionID,
		roundID:        target.roundID,
		seatID:         target.seatID,
	}, profile.HarnessAdapter, "roundtables-skip-preview")
	thread = cancelPhase7LiveSweepProposals(t, ctx, client, thread)
	requirePhase7SweepProposalsCancelled(
		t, thread, []controltool.ToolID{controltool.ToolRoundtablesSkipPreview},
	)
	if err := closePhase7LiveRoundTableFixture(ctx, client, fixture); err != nil {
		t.Fatalf("close isolated Pi RoundTable: %v", err)
	}
	t.Logf(
		"installed Pi RoundTable skip Proposal preserved exact Session/Round/Seat and was cancelled: %s/%s/%s",
		target.roundSessionID, target.roundID, target.seatID,
	)
}

// TestPhase7InstalledIdentityPreflight verifies the exact installed bundle,
// managed daemon and private UDS without a model call, credential operation or
// App restart. It is a safe precursor to the paid five-Runtime matrix.
func TestPhase7InstalledIdentityPreflight(t *testing.T) {
	if os.Getenv(phase7LiveIdentityPreflightGate) != "1" {
		t.Skip("Phase 7 identity preflight requires " + phase7LiveIdentityPreflightGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}
	pid, err := phase7InstalledDaemonPID(appPath, socketPath)
	if err != nil {
		t.Fatal(err)
	}
	client := newPhase7LiveClient(t, socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var snapshot struct {
		Providers []json.RawMessage `json:"providers"`
		Runtimes  []struct {
			RuntimeInstanceID string `json:"runtime_instance_id"`
			AdapterType       string `json:"adapter_type"`
			Status            string `json:"status"`
			Capacity          int    `json:"capacity"`
		} `json:"runtimes"`
		Profiles []app.ConversationProviderProfile `json:"conversation_profiles"`
		Accounts []struct {
			ProviderID        string `json:"provider_id"`
			ProviderAccountID string `json:"provider_account_id"`
			Status            string `json:"status"`
			Revision          int64  `json:"revision"`
		} `json:"provider_accounts"`
		Imports []struct {
			CandidateID         string   `json:"candidate_id"`
			TargetProviderID    string   `json:"target_provider_id"`
			ModelIDs            []string `json:"model_ids"`
			Current             bool     `json:"current"`
			CredentialAvailable bool     `json:"credential_available"`
		} `json:"credential_import_candidates"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Providers) == 0 || len(snapshot.Runtimes) == 0 ||
		len(snapshot.Profiles) == 0 {
		t.Fatalf(
			"installed catalog collapsed: providers=%d runtimes=%d profiles=%d",
			len(snapshot.Providers), len(snapshot.Runtimes), len(snapshot.Profiles),
		)
	}
	sort.Slice(snapshot.Profiles, func(left, right int) bool {
		return snapshot.Profiles[left].ProfileID < snapshot.Profiles[right].ProfileID
	})
	for _, profile := range snapshot.Profiles {
		t.Logf(
			"installed profile: id=%s harness=%s provider=%s account=%s model=%s auth=%s credential_revision=%d",
			profile.ProfileID, profile.HarnessAdapter, profile.ProviderID,
			profile.ProviderAccountID, profile.ModelID, profile.AuthMode,
			profile.CredentialRevision,
		)
	}
	for _, runtime := range snapshot.Runtimes {
		if runtime.AdapterType == "claude-code" {
			t.Logf(
				"installed Claude Runtime: id=%s status=%s capacity=%d",
				runtime.RuntimeInstanceID, runtime.Status, runtime.Capacity,
			)
		}
	}
	for _, account := range snapshot.Accounts {
		if account.ProviderID == "anthropic" {
			t.Logf(
				"installed Anthropic account: id=%s status=%s revision=%d",
				account.ProviderAccountID, account.Status, account.Revision,
			)
		}
	}
	for _, candidate := range snapshot.Imports {
		if candidate.TargetProviderID == "anthropic" {
			t.Logf(
				"installed Anthropic import candidate: id=%s current=%t credential_available=%t models=%v",
				candidate.CandidateID, candidate.Current,
				candidate.CredentialAvailable, candidate.ModelIDs,
			)
		}
	}
	productSnapshot := requirePhase7LiveProductSnapshot(t, ctx, client)
	anchor := requirePhase7GovernanceAnchor(t, productSnapshot)
	registry, err := loadPhase7RoundtableRegistry(
		filepath.Join(filepath.Dir(filepath.Dir(socketPath)), "roundtable-sessions.json"),
	)
	if err != nil {
		t.Fatalf("installed RoundTable navigation index is not acceptance-ready: %v", err)
	}
	t.Logf(
		"installed identity preflight passed: build=%s daemon_pid=%s providers=%d runtimes=%d profiles=%d governance_anchor=%s/%s registry_sessions=%d",
		expectedIdentity.Build, pid, len(snapshot.Providers), len(snapshot.Runtimes),
		len(snapshot.Profiles), anchor.missionID, anchor.teamInstanceID, len(registry),
	)
}

// TestPhase7InstalledControlThreadDiagnostic reads only non-secret Proposal
// binding metadata from one explicitly named installed thread. An explicit
// cleanup flag may cancel its pending test Proposal. It never logs messages,
// prompts, model output, tool arguments, or Provider responses.
func TestPhase7InstalledControlThreadDiagnostic(t *testing.T) {
	threadID := strings.TrimSpace(os.Getenv(phase7LiveThreadDiagnosticGate))
	if threadID == "" {
		t.Skip("Phase 7 thread diagnostic requires " + phase7LiveThreadDiagnosticGate)
	}
	expectedProfile := strings.TrimSpace(os.Getenv(phase7LiveExpectedProfileGate))
	if !validPhase7LiveDiagnosticID(threadID) ||
		(expectedProfile != "" && !validPhase7LiveDiagnosticID(expectedProfile)) {
		t.Fatal("invalid Phase 7 thread diagnostic target")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}
	client := newPhase7LiveClient(t, socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
		ThreadID: threadID,
	}, &thread); err != nil {
		t.Fatal(err)
	}
	for _, attempt := range thread.Attempts {
		completed := make([]string, 0, len(attempt.CompletedControlTools))
		for _, call := range attempt.CompletedControlTools {
			completed = append(completed, string(call.ToolID))
		}
		t.Logf(
			"attempt diagnostic: attempt=%s status=%s incident=%s stage=%s code=%s provider_code=%s retryable=%t completed_tools=%v",
			attempt.AttemptID, attempt.Status, attempt.IncidentID, attempt.FailureStage,
			attempt.FailureCode, attempt.ProviderCode, attempt.Retryable, completed,
		)
	}
	found := false
	actualProfile := ""
	for _, proposal := range thread.ActionProposals {
		payload := proposal.Payload
		profileID := ""
		missionID := ""
		teamInstanceID := ""
		sessionID := ""
		roundID := ""
		seatID := ""
		attemptID := ""
		if proposal.Payload != nil {
			profileID = payload.ProfileID
			missionID = payload.MissionID
			teamInstanceID = payload.TeamInstanceID
			sessionID = payload.SessionID
			roundID = payload.RoundID
			seatID = payload.SeatID
			attemptID = payload.AttemptID
		}
		t.Logf(
			"proposal binding: tool=%s status=%s profile=%s expected_profile=%s mission=%s team=%s session=%s round=%s seat=%s attempt=%s",
			proposal.ToolID, proposal.Status, profileID, expectedProfile,
			missionID, teamInstanceID, sessionID, roundID, seatID, attemptID,
		)
		if proposal.ToolID == controltool.ToolConversationRouteChangePreview {
			found = true
			actualProfile = profileID
		}
	}
	if expectedProfile != "" && !found {
		t.Fatal("Route Proposal unavailable in diagnostic thread")
	}
	if os.Getenv(phase7LiveDiagnosticCancelGate) == "1" {
		if !strings.HasPrefix(threadID, "p7-") {
			t.Fatal("diagnostic cancellation is restricted to Phase 7 test threads")
		}
		thread = cancelPhase7LiveSweepProposals(t, ctx, client, thread)
		if len(thread.ActionProposals) != 1 || thread.RequiresConfirmation ||
			(thread.ActionProposals[0].Status != controltool.ProposalCancelled &&
				thread.ActionProposals[0].Status != controltool.ProposalExpired) ||
			len(thread.ProposalDecisionReceipts) != 1 {
			t.Fatalf("diagnostic Route Proposal did not reach a terminal decision")
		}
		t.Logf(
			"diagnostic Route Proposal reached terminal status: %s",
			thread.ActionProposals[0].Status,
		)
		return
	}
	if expectedProfile != "" && actualProfile != expectedProfile {
		t.Fatalf(
			"Route proposal profile mismatch: got=%s want=%s",
			actualProfile, expectedProfile,
		)
	}
}

func validPhase7LiveDiagnosticID(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

// TestLivePhase7ControlToolsE2E is the bounded installed acceptance gate for
// model-selected Loom control tools. Each Runtime performs three bounded
// Mission Proposal turns for the full decision lifecycle. The available
// Runtimes then partition every admitted Tool across 28 single-Tool natural-
// language turns. The gate never changes credentials and never creates a
// Mission or Team. It creates only p7-prefixed, synthetic RoundTable Sessions
// linked to an existing P7 acceptance Mission, drives each Session through the
// exact action preconditions, and concludes it after cancelling every model-
// prepared Proposal. A final managed App restart proves the encrypted decisions
// and Tool audit restore.
//
// This gate can incur Provider charges and restarts the installed App. It is
// skipped unless all authorization variables below are set explicitly.
func TestLivePhase7ControlToolsE2E(t *testing.T) {
	runPhase7LiveControlToolsE2E(t, false)
}

// TestLivePhase7AvailableRuntimeControlToolsE2E is the installed acceptance
// gate when the operator explicitly waives Claude Code live calls because that
// product is unavailable. Claude source parity remains mandatory. The waiver
// cannot bypass a published executable Claude Profile.
func TestLivePhase7AvailableRuntimeControlToolsE2E(t *testing.T) {
	runPhase7LiveControlToolsE2E(t, true)
}

func runPhase7LiveControlToolsE2E(t *testing.T, withoutClaude bool) {
	t.Helper()
	gate := phase7LiveControlGate
	if withoutClaude {
		gate = phase7LiveAvailableControlGate
	}
	if os.Getenv(gate) != "1" {
		t.Skip("Phase 7 installed control-tool gate requires " + gate + "=1")
	}
	if withoutClaude && os.Getenv(phase7LiveClaudeWaiverGate) != "1" {
		t.Fatal("Claude live-call waiver requires " + phase7LiveClaudeWaiverGate + "=1")
	}
	if os.Getenv(phase7LivePaidGate) != "1" {
		t.Fatal("paid model calls require " + phase7LivePaidGate + "=1")
	}
	if os.Getenv(phase7LiveRestartGate) != "1" {
		t.Fatal("managed App restart requires " + phase7LiveRestartGate + "=1")
	}
	expectedIdentity, err := phase7ExpectedInstalledIdentityFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}

	appPath, socketPath, err := phase7InstalledPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledIdentity(appPath, expectedIdentity); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	client := newPhase7LiveClient(t, socketPath)
	var profiles []phase7LiveProfile
	if withoutClaude {
		profiles = requirePhase7AvailableRuntimeProfiles(t, ctx, client)
	} else {
		profiles = requirePhase7LiveProfiles(t, ctx, client)
	}
	beforePID, err := phase7InstalledDaemonPID(appPath, socketPath)
	if err != nil {
		t.Fatal(err)
	}

	runID := fmt.Sprintf("%x%x", time.Now().UTC().UnixNano(), os.Getpid())
	privateContentMarker, err := phase7LivePrivateContentMarker(runID)
	if err != nil {
		t.Fatal(err)
	}
	beforeState := requirePhase7LiveProductSnapshot(t, ctx, client)
	requirePhase7NoSentinelProductState(t, beforeState, runID)
	governanceAnchor := requirePhase7GovernanceAnchor(t, beforeState)
	records := make([]phase7LiveProposalRecord, 0, len(profiles)*3)
	for _, profile := range profiles {
		confirm := preparePhase7LiveMissionProposal(t, ctx, client, runID, "confirm", profile)
		thread := decidePhase7LiveProposal(
			t, ctx, client, confirm, api.ControlDecisionConfirm,
		)
		requirePhase7ProposalStatus(
			t, thread, confirm.proposal.ProposalID, controltool.ProposalConfirmed,
		)
		receipt := requirePhase7DecisionReceipt(
			t, thread, confirm.proposal, controltool.ProposalDecisionConfirm,
		)
		confirm.wantStatus = controltool.ProposalConfirmed
		confirm.wantDecision = controltool.ProposalDecisionConfirm
		confirm.receiptDigest = receipt.ReceiptDigest
		var replay api.LocalProductChatThread
		err := client.Call(ctx, "chat_control_decision", api.LocalProductChatControlDecisionRequest{
			ThreadID: confirm.threadID, ProposalID: confirm.proposal.ProposalID,
			ProposalDigest: confirm.proposal.ProposalDigest,
			Decision:       api.ControlDecisionConfirm,
		}, &replay)
		var remote *localipc.RemoteError
		if !errors.As(err, &remote) || remote.Code != "conflict" {
			t.Fatalf("%s replay result = %v", confirm.harness, err)
		}
		records = append(records, confirm)

		cancelled := preparePhase7LiveMissionProposal(t, ctx, client, runID, "cancel", profile)
		thread = decidePhase7LiveProposal(
			t, ctx, client, cancelled, api.ControlDecisionCancel,
		)
		requirePhase7ProposalStatus(
			t, thread, cancelled.proposal.ProposalID, controltool.ProposalCancelled,
		)
		receipt = requirePhase7DecisionReceipt(
			t, thread, cancelled.proposal, controltool.ProposalDecisionCancel,
		)
		cancelled.wantStatus = controltool.ProposalCancelled
		cancelled.wantDecision = controltool.ProposalDecisionCancel
		cancelled.receiptDigest = receipt.ReceiptDigest
		records = append(records, cancelled)

		records = append(records, preparePhase7LiveMissionProposal(
			t, ctx, client, runID, "expire", profile,
		))
	}

	sweepRecords := runPhase7LiveCapabilitySweep(
		t, ctx, client, runID, profiles, records[0].threadID, records[1].threadID,
		governanceAnchor, records[0].proposal.IncidentID,
	)
	requirePhase7NoSentinelProductState(
		t, requirePhase7LiveProductSnapshot(t, ctx, client), runID,
	)

	latestExpiry := time.Time{}
	for _, record := range records {
		if record.decisionCase == "expire" && record.proposal.ExpiresAt.After(latestExpiry) {
			latestExpiry = record.proposal.ExpiresAt
		}
	}
	if latestExpiry.IsZero() {
		t.Fatal("expiry proposals missing")
	}
	if wait := time.Until(latestExpiry.Add(time.Second)); wait > 0 {
		t.Logf("waiting %s for digest-bound Proposal expiry", wait.Round(time.Second))
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(wait):
		}
	}
	for index := range records {
		record := &records[index]
		if record.decisionCase != "expire" {
			continue
		}
		thread := decidePhase7LiveProposal(
			t, ctx, client, *record, api.ControlDecisionConfirm,
		)
		requirePhase7ProposalStatus(
			t, thread, record.proposal.ProposalID, controltool.ProposalExpired,
		)
		receipt := requirePhase7DecisionReceipt(
			t, thread, record.proposal, controltool.ProposalDecisionExpire,
		)
		record.wantStatus = controltool.ProposalExpired
		record.wantDecision = controltool.ProposalDecisionExpire
		record.receiptDigest = receipt.ReceiptDigest
	}

	client = restartPhase7InstalledApp(
		t, ctx, appPath, socketPath, beforePID, expectedIdentity,
	)
	for _, record := range records {
		var thread api.LocalProductChatThread
		if err := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
			ThreadID: record.threadID,
		}, &thread); err != nil {
			t.Fatalf("%s restored thread: %v", record.harness, err)
		}
		requirePhase7ProposalStatus(
			t, thread, record.proposal.ProposalID, record.wantStatus,
		)
		receipt := requirePhase7DecisionReceipt(
			t, thread, record.proposal, record.wantDecision,
		)
		if receipt.ReceiptDigest != record.receiptDigest {
			t.Fatalf(
				"%s restored receipt digest = %s, want %s",
				record.harness, receipt.ReceiptDigest, record.receiptDigest,
			)
		}
	}
	for _, record := range sweepRecords {
		var thread api.LocalProductChatThread
		if err := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
			ThreadID: record.threadID,
		}, &thread); err != nil {
			t.Fatalf("restored %s capability group %s: %v", record.harness, record.name, err)
		}
		requirePhase7CompletedTools(t, thread, record.expected...)
		requirePhase7SweepProposalsCancelled(t, thread, record.proposalTools)
	}
	requirePhase7NoSentinelProductState(
		t, requirePhase7LiveProductSnapshot(t, ctx, client), runID,
	)
	requirePhase7NoPlaintextPrivateContent(t, socketPath, privateContentMarker)
	if withoutClaude {
		t.Log("Phase 7 installed control matrix passed for Codex, OpenCode, Pi, and Loom Native under the explicit Claude live-call waiver")
		return
	}
	t.Log("Phase 7 installed control matrix passed for all five Runtime adapters")
}

type phase7LiveProfile struct {
	app.ConversationProviderProfile
}

type phase7LiveProposalRecord struct {
	harness       string
	decisionCase  string
	threadID      string
	proposal      controltool.ConversationActionProposal
	wantStatus    controltool.ProposalStatus
	wantDecision  controltool.ProposalDecision
	receiptDigest string
}

type phase7LiveCapabilityFixture struct {
	runID          string
	sourceA        string
	sourceB        string
	missionID      string
	teamInstanceID string
	roundSessionID string
	roundID        string
	seatID         string
	attemptID      string
	incidentID     string
	profileID      string
	modelID        string
}

type phase7LiveGovernanceAnchor struct {
	missionID      string
	teamInstanceID string
	agents         []api.LocalProductTeamAgentSummary
}

type phase7LiveGovernanceTargets struct {
	missionID      string
	teamInstanceID string
	roundSessionID string
	roundID        string
	seatID         string
	attemptID      string
}

type phase7LiveRoundTableFixture struct {
	suffix        string
	sessionID     string
	roundID       string
	moderatorSeat string
	anchor        phase7LiveGovernanceAnchor
	closed        bool
}

type phase7LiveCapabilityGroup struct {
	name          string
	prompt        string
	expected      []controltool.ToolID
	proposalTools []controltool.ToolID
}

type phase7LiveCapabilityRecord struct {
	harness       string
	name          string
	threadID      string
	expected      []controltool.ToolID
	proposalTools []controltool.ToolID
}

func phase7LiveCapabilityGroups(
	fixture phase7LiveCapabilityFixture,
) []phase7LiveCapabilityGroup {
	return []phase7LiveCapabilityGroup{
		{
			name: "sessions-search",
			prompt: "请实际使用 Loom 的会话搜索能力，搜索标题包含 P7 source 的会话。" +
				"只执行这一项只读查询，不要猜测、不要创建提案，完成后简短概括。",
			expected: []controltool.ToolID{controltool.ToolSessionsSearch},
		},
		{
			name: "sessions-align-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，把会话 %s 和 %s 以 summary-only 方式准备成对齐到当前会话的审阅提案。只准备这一项，不要执行。",
				fixture.sourceA, fixture.sourceB,
			),
			expected:      []controltool.ToolID{controltool.ToolSessionsAlignPreview},
			proposalTools: []controltool.ToolID{controltool.ToolSessionsAlignPreview},
		},
		{
			name: "missions-create-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，准备一个目标为 P7 %s verification 的 Mission 审阅提案。只准备这一项，不要执行。",
				fixture.runID,
			),
			expected:      []controltool.ToolID{controltool.ToolMissionsCreatePreview},
			proposalTools: []controltool.ToolID{controltool.ToolMissionsCreatePreview},
		},
		{
			name: "missions-continue-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为已阻塞 Mission %s 准备继续工作提案，指导语必须精确为 Continue safely。只准备这一项，不要执行。",
				fixture.missionID,
			),
			expected:      []controltool.ToolID{controltool.ToolMissionsContinuePreview},
			proposalTools: []controltool.ToolID{controltool.ToolMissionsContinuePreview},
		},
		{
			name: "teams-create-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，准备一个用途为 P7 %s verification 的 Agent Team 审阅提案。只准备这一项，不要执行。",
				fixture.runID,
			),
			expected:      []controltool.ToolID{controltool.ToolTeamsCreatePreview},
			proposalTools: []controltool.ToolID{controltool.ToolTeamsCreatePreview},
		},
		{
			name: "roundtables-open-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 Mission %s 准备打开 RoundTable 的审阅提案。只准备这一项，不要执行。",
				fixture.missionID,
			),
			expected:      []controltool.ToolID{controltool.ToolRoundtablesOpenPreview},
			proposalTools: []controltool.ToolID{controltool.ToolRoundtablesOpenPreview},
		},
		{
			name: "missions-search",
			prompt: "请实际使用 Loom 的 Mission 搜索能力查找 P7。只执行这一项只读查询，" +
				"即使没有结果也不要猜测或创建提案，完成后简短概括。",
			expected: []controltool.ToolID{controltool.ToolMissionsSearch},
		},
		{
			name: "missions-status",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 的 Mission 状态能力检查 %s。只执行这一项只读查询，即使找不到也不要猜测或创建提案。",
				fixture.missionID,
			),
			expected: []controltool.ToolID{controltool.ToolMissionsStatus},
		},
		{
			name: "teams-search",
			prompt: "请实际使用 Loom 的 Agent Team 搜索能力查找 P7。只执行这一项只读查询，" +
				"即使没有结果也不要猜测或创建提案。",
			expected: []controltool.ToolID{controltool.ToolTeamsSearch},
		},
		{
			name: "teams-status",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 的 Agent Team 状态能力检查 %s。只执行这一项只读查询，即使找不到也不要猜测或创建提案。",
				fixture.teamInstanceID,
			),
			expected: []controltool.ToolID{controltool.ToolTeamsStatus},
		},
		{
			name: "roundtables-status",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 的 RoundTable 状态能力检查 Session %s。只执行这一项只读查询，即使找不到也不要猜测或创建提案。",
				fixture.roundSessionID,
			),
			expected: []controltool.ToolID{controltool.ToolRoundtablesStatus},
		},
		{
			name:     "governance-needs-you",
			prompt:   "请实际使用 Loom 能力列出当前 Needs You 中等待用户处理的事项。只执行这一项只读查询，不要创建提案。",
			expected: []controltool.ToolID{controltool.ToolGovernanceNeedsYou},
		},
		{
			name:     "runtimes-status",
			prompt:   "请实际使用 Loom 能力列出当前 Runtime 状态。只执行这一项只读查询，不要猜测或创建提案。",
			expected: []controltool.ToolID{controltool.ToolRuntimesStatus},
		},
		{
			name:     "providers-status",
			prompt:   "请实际使用 Loom 能力列出当前 Provider 与账号状态。只执行这一项只读查询，不要猜测或创建提案。",
			expected: []controltool.ToolID{controltool.ToolProvidersStatus},
		},
		{
			name: "diagnostics-incident",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 的隐私安全诊断能力检查 Incident %s。只执行这一项只读查询，即使找不到也不要猜测或创建提案。",
				fixture.incidentID,
			),
			expected: []controltool.ToolID{controltool.ToolDiagnosticsIncident},
		},
		{
			name:     "workspace-status",
			prompt:   "请实际使用 Loom 能力检查当前会话的 workspace 身份与摘要。只执行这一项只读查询，不要创建提案。",
			expected: []controltool.ToolID{controltool.ToolWorkspaceStatus},
		},
		{
			name:     "conversation-route-status",
			prompt:   "请实际使用 Loom 能力检查当前会话冻结的 Harness、Provider、模型与推理配置。只执行这一项只读查询，不要创建提案。",
			expected: []controltool.ToolID{controltool.ToolConversationRouteStatus},
		},
		{
			name:     "library-search",
			prompt:   "请实际使用 Loom 的 Library 搜索能力查找 P7 Evidence。只执行这一项只读查询，即使没有结果也不要猜测或创建提案。",
			expected: []controltool.ToolID{controltool.ToolLibrarySearch},
		},
		{
			name: "conversation-route-change-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，准备把当前会话 Route 选择为 %s 的审阅提案。只准备这一项，不要执行。",
				fixture.profileID,
			),
			expected:      []controltool.ToolID{controltool.ToolConversationRouteChangePreview},
			proposalTools: []controltool.ToolID{controltool.ToolConversationRouteChangePreview},
		},
		{
			name: "conversation-model-change-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，准备把当前会话模型选择为 %s 的审阅提案。只准备这一项，不要执行。",
				fixture.modelID,
			),
			expected:      []controltool.ToolID{controltool.ToolConversationModelChangePreview},
			proposalTools: []controltool.ToolID{controltool.ToolConversationModelChangePreview},
		},
		{
			name:          "conversation-reasoning-change-preview",
			prompt:        "请实际使用 Loom 能力，准备把当前会话推理强度设置为 high 的审阅提案。只准备这一项，不要执行。",
			expected:      []controltool.ToolID{controltool.ToolConversationReasoningChangePreview},
			proposalTools: []controltool.ToolID{controltool.ToolConversationReasoningChangePreview},
		},
		{
			name:          "workspace-choose-preview",
			prompt:        "请实际使用 Loom 能力，准备打开由用户选择 workspace 文件夹的审阅提案。只准备这一项，不要猜测或选择本地路径，不要执行。",
			expected:      []controltool.ToolID{controltool.ToolWorkspaceChoosePreview},
			proposalTools: []controltool.ToolID{controltool.ToolWorkspaceChoosePreview},
		},
		{
			name: "teams-edit-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 Agent Team %s 准备编辑审阅提案，编辑指导必须精确为 Review role bindings。只准备这一项，不要执行。",
				fixture.teamInstanceID,
			),
			expected:      []controltool.ToolID{controltool.ToolTeamsEditPreview},
			proposalTools: []controltool.ToolID{controltool.ToolTeamsEditPreview},
		},
		{
			name: "roundtables-pause-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 RoundTable Session %s 的 Round %s 准备暂停审阅提案。只准备这一项，不要执行。",
				fixture.roundSessionID, fixture.roundID,
			),
			expected:      []controltool.ToolID{controltool.ToolRoundtablesPausePreview},
			proposalTools: []controltool.ToolID{controltool.ToolRoundtablesPausePreview},
		},
		{
			name: "roundtables-steer-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 RoundTable Session %s、Round %s、Seat %s、Attempt %s 准备 steer 审阅提案，指导必须精确为 Focus on evidence。只准备这一项，不要执行。",
				fixture.roundSessionID, fixture.roundID, fixture.seatID, fixture.attemptID,
			),
			expected:      []controltool.ToolID{controltool.ToolRoundtablesSteerPreview},
			proposalTools: []controltool.ToolID{controltool.ToolRoundtablesSteerPreview},
		},
		{
			name: "roundtables-retry-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 RoundTable Session %s、Round %s、Seat %s、Attempt %s 准备 retry 审阅提案，指导必须精确为 Retry from evidence。只准备这一项，不要执行。",
				fixture.roundSessionID, fixture.roundID, fixture.seatID, fixture.attemptID,
			),
			expected:      []controltool.ToolID{controltool.ToolRoundtablesRetryPreview},
			proposalTools: []controltool.ToolID{controltool.ToolRoundtablesRetryPreview},
		},
		{
			name: "roundtables-skip-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 RoundTable Session %s、Round %s 的 Seat %s 准备 skip 审阅提案。只准备这一项，不要执行。",
				fixture.roundSessionID, fixture.roundID, fixture.seatID,
			),
			expected:      []controltool.ToolID{controltool.ToolRoundtablesSkipPreview},
			proposalTools: []controltool.ToolID{controltool.ToolRoundtablesSkipPreview},
		},
		{
			name: "roundtables-replace-preview",
			prompt: fmt.Sprintf(
				"请实际使用 Loom 能力，为 RoundTable Session %s、Round %s 的 Seat %s 准备 replace 审阅提案。只准备这一项，不要执行。",
				fixture.roundSessionID, fixture.roundID, fixture.seatID,
			),
			expected:      []controltool.ToolID{controltool.ToolRoundtablesReplacePreview},
			proposalTools: []controltool.ToolID{controltool.ToolRoundtablesReplacePreview},
		},
	}
}

func requirePhase7LiveProfiles(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) []phase7LiveProfile {
	t.Helper()
	var snapshot struct {
		Vault    *app.CredentialVaultStatus        `json:"credential_vault"`
		Profiles []app.ConversationProviderProfile `json:"conversation_profiles"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	if snapshot.Vault == nil || snapshot.Vault.Status != "unlocked" {
		t.Fatalf("Credential Vault must already be unlocked; status = %#v", snapshot.Vault)
	}
	profiles, err := selectPhase7LiveProfilesForProviders(
		snapshot.Profiles, strings.TrimSpace(os.Getenv(phase7LiveOpenCodeProviderGate)),
		strings.TrimSpace(os.Getenv(phase7LiveLoomNativeProviderGate)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return profiles
}

func requirePhase7LiveOpenCodeProfile(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) phase7LiveProfile {
	t.Helper()
	var snapshot struct {
		Vault    *app.CredentialVaultStatus        `json:"credential_vault"`
		Profiles []app.ConversationProviderProfile `json:"conversation_profiles"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	if snapshot.Vault == nil || snapshot.Vault.Status != "unlocked" {
		t.Fatalf("Credential Vault must already be unlocked; status = %#v", snapshot.Vault)
	}
	profile, err := selectPhase7LiveOpenCodeProfile(
		snapshot.Profiles,
		strings.TrimSpace(os.Getenv(phase7LiveOpenCodeProviderGate)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func requirePhase7AvailableRuntimeProfiles(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) []phase7LiveProfile {
	t.Helper()
	var snapshot struct {
		Vault    *app.CredentialVaultStatus        `json:"credential_vault"`
		Profiles []app.ConversationProviderProfile `json:"conversation_profiles"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	if snapshot.Vault == nil || snapshot.Vault.Status != "unlocked" {
		t.Fatalf("Credential Vault must already be unlocked; status = %#v", snapshot.Vault)
	}
	profiles, err := selectPhase7AvailableRuntimeProfilesForProviders(
		snapshot.Profiles,
		strings.TrimSpace(os.Getenv(phase7LiveOpenCodeProviderGate)),
		strings.TrimSpace(os.Getenv(phase7LiveLoomNativeProviderGate)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return profiles
}

func requirePhase7LiveExactProfile(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	harnessAdapter string,
	providerID string,
) phase7LiveProfile {
	t.Helper()
	var snapshot struct {
		Vault    *app.CredentialVaultStatus        `json:"credential_vault"`
		Profiles []app.ConversationProviderProfile `json:"conversation_profiles"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	if snapshot.Vault == nil || snapshot.Vault.Status != "unlocked" {
		t.Fatalf("Credential Vault must already be unlocked; status = %#v", snapshot.Vault)
	}
	var selected app.ConversationProviderProfile
	for _, profile := range snapshot.Profiles {
		if profile.HarnessAdapter != harnessAdapter || profile.ProviderID != providerID ||
			profile.ProfileID == "" || profile.ModelID == "" {
			continue
		}
		if selected.ProfileID != "" {
			t.Fatalf("multiple executable %s/%s profiles", harnessAdapter, providerID)
		}
		selected = profile
	}
	if selected.ProfileID == "" {
		t.Fatalf("installed matrix missing executable %s/%s profile", harnessAdapter, providerID)
	}
	return phase7LiveProfile{selected}
}

func requirePhase7LiveProfileAbsent(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	harnessAdapter string,
	providerID string,
) {
	t.Helper()
	var snapshot struct {
		Profiles []app.ConversationProviderProfile `json:"conversation_profiles"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	for _, profile := range snapshot.Profiles {
		if profile.HarnessAdapter == harnessAdapter && profile.ProviderID == providerID {
			t.Fatalf("incompatible installed profile remains executable: %#v", profile)
		}
	}
}

func selectPhase7LiveProfiles(
	available []app.ConversationProviderProfile,
) ([]phase7LiveProfile, error) {
	return selectPhase7LiveProfilesForOpenCodeProvider(available, "")
}

func selectPhase7LiveProfilesForOpenCodeProvider(
	available []app.ConversationProviderProfile,
	openCodeProviderID string,
) ([]phase7LiveProfile, error) {
	return selectPhase7LiveProfilesForProviders(available, openCodeProviderID, "")
}

func selectPhase7LiveProfilesForProviders(
	available []app.ConversationProviderProfile,
	openCodeProviderID string,
	loomNativeProviderID string,
) ([]phase7LiveProfile, error) {
	if openCodeProviderID != "" && openCodeProviderID != "deepseek" &&
		openCodeProviderID != "minimax" && openCodeProviderID != "opencode" {
		return nil, errors.New("invalid OpenCode Provider selector")
	}
	if loomNativeProviderID != "" && loomNativeProviderID != "deepseek" &&
		loomNativeProviderID != "minimax" {
		return nil, errors.New("invalid Loom Native Provider selector")
	}
	byHarness := make(map[string][]app.ConversationProviderProfile)
	for _, profile := range available {
		if !phase7LiveProfileExecutable(profile) {
			continue
		}
		if profile.HarnessAdapter == "opencode" && openCodeProviderID != "" &&
			profile.ProviderID != openCodeProviderID {
			continue
		}
		if profile.HarnessAdapter == "loom-native" && loomNativeProviderID != "" &&
			profile.ProviderID != loomNativeProviderID {
			continue
		}
		byHarness[profile.HarnessAdapter] = append(byHarness[profile.HarnessAdapter], profile)
	}
	order := []string{"codex", "opencode", "claude-code", "pi", "loom-native"}
	selected := make([]phase7LiveProfile, 0, len(order))
	for _, harness := range order {
		candidates := byHarness[harness]
		sort.Slice(candidates, func(left, right int) bool {
			return phase7LiveProfilePriority(candidates[left]) <
				phase7LiveProfilePriority(candidates[right])
		})
		if len(candidates) == 0 {
			return nil, fmt.Errorf("installed Phase 7 matrix missing executable %s profile", harness)
		}
		selected = append(selected, phase7LiveProfile{candidates[0]})
	}
	return selected, nil
}

func selectPhase7LiveOpenCodeProfile(
	available []app.ConversationProviderProfile,
	openCodeProviderID string,
) (phase7LiveProfile, error) {
	if openCodeProviderID != "" && openCodeProviderID != "deepseek" &&
		openCodeProviderID != "minimax" && openCodeProviderID != "opencode" {
		return phase7LiveProfile{}, errors.New("invalid OpenCode Provider selector")
	}
	candidates := make([]app.ConversationProviderProfile, 0)
	for _, profile := range available {
		if profile.HarnessAdapter != "opencode" ||
			!phase7LiveProfileExecutable(profile) {
			continue
		}
		if openCodeProviderID != "" && profile.ProviderID != openCodeProviderID {
			continue
		}
		candidates = append(candidates, profile)
	}
	sort.Slice(candidates, func(left, right int) bool {
		return phase7LiveProfilePriority(candidates[left]) <
			phase7LiveProfilePriority(candidates[right])
	})
	if len(candidates) == 0 {
		return phase7LiveProfile{}, errors.New(
			"installed Phase 7 probe missing executable OpenCode profile",
		)
	}
	return phase7LiveProfile{candidates[0]}, nil
}

func selectPhase7AvailableRuntimeProfiles(
	available []app.ConversationProviderProfile,
	openCodeProviderID string,
) ([]phase7LiveProfile, error) {
	return selectPhase7AvailableRuntimeProfilesForProviders(
		available, openCodeProviderID, "",
	)
}

func selectPhase7AvailableRuntimeProfilesForProviders(
	available []app.ConversationProviderProfile,
	openCodeProviderID string,
	loomNativeProviderID string,
) ([]phase7LiveProfile, error) {
	for _, profile := range available {
		if profile.HarnessAdapter == "claude-code" &&
			phase7LiveProfileExecutable(profile) {
			return nil, errors.New(
				"available-Runtime matrix cannot bypass an executable Claude Code profile",
			)
		}
	}
	if openCodeProviderID != "" && openCodeProviderID != "deepseek" &&
		openCodeProviderID != "minimax" && openCodeProviderID != "opencode" {
		return nil, errors.New("invalid OpenCode Provider selector")
	}
	if loomNativeProviderID != "" && loomNativeProviderID != "deepseek" &&
		loomNativeProviderID != "minimax" {
		return nil, errors.New("invalid Loom Native Provider selector")
	}
	byHarness := make(map[string][]app.ConversationProviderProfile)
	for _, profile := range available {
		if !phase7LiveProfileExecutable(profile) {
			continue
		}
		if profile.HarnessAdapter == "opencode" && openCodeProviderID != "" &&
			profile.ProviderID != openCodeProviderID {
			continue
		}
		if profile.HarnessAdapter == "loom-native" && loomNativeProviderID != "" &&
			profile.ProviderID != loomNativeProviderID {
			continue
		}
		byHarness[profile.HarnessAdapter] = append(
			byHarness[profile.HarnessAdapter], profile,
		)
	}
	order := []string{"codex", "opencode", "pi", "loom-native"}
	selected := make([]phase7LiveProfile, 0, len(order))
	for _, harness := range order {
		candidates := byHarness[harness]
		sort.Slice(candidates, func(left, right int) bool {
			return phase7LiveProfilePriority(candidates[left]) <
				phase7LiveProfilePriority(candidates[right])
		})
		if len(candidates) == 0 {
			return nil, fmt.Errorf(
				"partial installed matrix missing executable %s profile", harness,
			)
		}
		selected = append(selected, phase7LiveProfile{candidates[0]})
	}
	return selected, nil
}

func phase7LiveProfileExecutable(profile app.ConversationProviderProfile) bool {
	switch profile.HarnessAdapter {
	case "codex":
		return profile.ProfileID == provider.CodexConversationProfileID &&
			profile.ProviderID == "openai" && profile.ProviderAccountID == "" &&
			profile.Protocol == "openai_responses" && profile.ModelID == "codex-default" &&
			profile.AuthMode == "native_auth" && profile.CredentialRevision == 0
	case "claude-code":
		return profile.ProfileID == provider.ClaudeCodeConversationProfileID &&
			profile.ProviderID == "anthropic" && profile.ProviderAccountID == "" &&
			profile.Protocol == "claude_code_agent" &&
			profile.ModelID == provider.AnthropicConversationModelID &&
			profile.AuthMode == "native_auth" && profile.CredentialRevision == 0
	case "pi":
		return profile.ProfileID == provider.PiConversationProfileID &&
			profile.ProviderID == "loom-local" && profile.ProviderAccountID == "" &&
			profile.Protocol == "pi_rpc" &&
			profile.ModelID == provider.PiConversationModelID &&
			profile.AuthMode == "native_auth" && profile.CredentialRevision == 0
	case "opencode":
		if profile.Protocol != "opencode_agent" {
			return false
		}
		if profile.AuthMode == "native_auth" {
			_, available := provider.SelectOpenCodeNativeModel([]string{profile.ModelID})
			return available && profile.ProfileID == provider.OpenCodeConversationProfileID &&
				profile.ProviderID == "opencode" && profile.ProviderAccountID == "" &&
				profile.CredentialRevision == 0
		}
		return profile.AuthMode == "brokered" && profile.CredentialRevision > 0 &&
			credentials.ValidProviderAccountIdentifier(
				profile.ProviderID, profile.ProviderAccountID,
			) && provider.OpenCodeBrokeredModelSupported(
			profile.ProviderID, profile.ModelID,
		) && profile.ProfileID == provider.OpenCodeConversationAccountProfileID(
			profile.ProviderID, profile.ProviderAccountID, profile.CredentialRevision,
		)
	case "loom-native":
		return phase7LiveLoomNativeProfileExecutable(profile)
	default:
		return false
	}
}

func phase7LiveLoomNativeProfileExecutable(
	profile app.ConversationProviderProfile,
) bool {
	if profile.AuthMode != "brokered" || profile.CredentialRevision <= 0 ||
		!credentials.ValidProviderAccountIdentifier(
			profile.ProviderID, profile.ProviderAccountID,
		) {
		return false
	}
	var profileID, protocol, modelID string
	switch profile.ProviderID {
	case "anthropic":
		profileID = provider.AnthropicConversationAccountProfileID(
			profile.ProviderAccountID, profile.CredentialRevision,
		)
		protocol = "anthropic_messages"
		modelID = provider.AnthropicConversationModelID
	case "deepseek":
		profileID = provider.DeepSeekConversationAccountProfileID(
			profile.ProviderAccountID, profile.CredentialRevision,
		)
		protocol = "openai_compatible"
		modelID = provider.DeepSeekConversationModelID
	case "kimi":
		profileID = provider.KimiConversationAccountProfileID(
			profile.ProviderAccountID, profile.CredentialRevision,
		)
		protocol = "openai_compatible"
		modelID = provider.KimiConversationModelID
	case "minimax":
		profileID = provider.MiniMaxConversationAccountProfileID(
			profile.ProviderAccountID, profile.CredentialRevision,
		)
		protocol = "openai_compatible"
		modelID = provider.MiniMaxConversationModelID
	default:
		return false
	}
	return profile.ProfileID == profileID && profile.Protocol == protocol &&
		profile.ModelID == modelID
}

func phase7LiveProfilePriority(profile app.ConversationProviderProfile) string {
	priority := 50
	switch profile.HarnessAdapter {
	case "opencode":
		if profile.AuthMode == "brokered" && profile.ProviderAccountID != "" &&
			profile.CredentialRevision > 0 {
			priority = 20
			if profile.ProviderID == "minimax" {
				priority = 0
			} else if profile.ProviderID == "deepseek" {
				priority = 10
			}
		} else if profile.ProviderID == "opencode" && profile.AuthMode == "native_auth" {
			priority = 30
		}
	case "loom-native":
		if profile.ProviderID == "deepseek" && profile.ProviderAccountID != "" {
			priority = 0
		}
	}
	return fmt.Sprintf("%02d\x00%s", priority, profile.ProfileID)
}

func phase7LivePrivateContentMarker(runID string) (string, error) {
	if runID == "" || len(runID) > 128 {
		return "", errors.New("invalid Phase 7 live run ID")
	}
	for _, character := range runID {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '-' {
			return "", errors.New("invalid Phase 7 live run ID")
		}
	}
	digest := sha256.Sum256([]byte("loom/phase7/private-content/v1\x00" + runID))
	return phase7PrivateContentPrefix + hex.EncodeToString(digest[:]), nil
}

func phase7LiveMissionObjective(
	runID string,
	harnessAdapter string,
	decisionCase string,
) (string, error) {
	marker, err := phase7LivePrivateContentMarker(runID)
	if err != nil {
		return "", err
	}
	if harnessAdapter == "" || len(harnessAdapter) > 64 ||
		decisionCase == "" || len(decisionCase) > 32 {
		return "", errors.New("invalid Phase 7 Mission objective identity")
	}
	return fmt.Sprintf(
		"P7 %s %s %s governance check %s",
		runID, harnessAdapter, decisionCase, marker,
	), nil
}

func preparePhase7LiveMissionProposal(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	runID string,
	decisionCase string,
	profile phase7LiveProfile,
) phase7LiveProposalRecord {
	t.Helper()
	harnessID := strings.ReplaceAll(profile.HarnessAdapter, "-", "")
	threadID := fmt.Sprintf("p7-%s-%s-%s", runID, harnessID, decisionCase)
	objective, err := phase7LiveMissionObjective(
		runID, profile.HarnessAdapter, decisionCase,
	)
	if err != nil {
		t.Fatal(err)
	}
	prompt := "Use the available Loom capability to prepare a reviewable Mission draft whose objective is exactly \"" + objective +
		"\". Put that text in the declared objective field; do not add a separate title field. Return the Loom review Proposal instead of a prose-only draft. Do not execute it and do not ask me to use a slash command."
	if profile.HarnessAdapter == "codex" || profile.HarnessAdapter == "pi" {
		prompt = "请使用当前可用的 Loom 能力，把“" + objective +
			"”准备成可审阅的 Mission 提案。必须返回 Loom 审阅 Proposal，不要只写文字草案、不要执行，也不要让我输入斜杠命令。"
	}
	request := api.LocalProductChatMessageRequest{
		ThreadID: threadID, Content: prompt,
		ProfileID: profile.ProfileID, ModelID: profile.ModelID,
		ContextMode:              api.ContextModeStartClean,
		ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
	}
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
		var failedThread api.LocalProductChatThread
		if readErr := client.Call(ctx, "chat_thread", api.LocalProductChatThreadRequest{
			ThreadID: threadID,
		}, &failedThread); readErr == nil {
			t.Fatalf(
				"%s %s model turn: %v; persisted_attempt=%s",
				profile.HarnessAdapter, decisionCase, err,
				phase7LiveThreadFailure(failedThread),
			)
		}
		t.Fatalf("%s %s model turn: %v", profile.HarnessAdapter, decisionCase, err)
	}
	if failure := phase7LiveThreadFailure(thread); failure != "" {
		t.Fatalf("%s %s model turn: %s", profile.HarnessAdapter, decisionCase, failure)
	}
	if len(thread.ActionProposals) != 1 || len(thread.ControlProposals) != 0 {
		t.Fatalf(
			"%s %s selected %d action and %d alignment Proposals; completed_tools=%v",
			profile.HarnessAdapter, decisionCase,
			len(thread.ActionProposals), len(thread.ControlProposals),
			phase7LiveCompletedToolIDs(thread),
		)
	}
	proposal := thread.ActionProposals[0]
	if !proposal.Valid() || proposal.SchemaVersion != 2 ||
		proposal.ToolID != controltool.ToolMissionsCreatePreview ||
		proposal.Action != controltool.ConversationActionMission ||
		proposal.Status != controltool.ProposalPending ||
		proposal.Route == nil || proposal.Route.HarnessAdapter != profile.HarnessAdapter ||
		proposal.Route.ProviderID != profile.ProviderID ||
		proposal.Route.ProviderAccountID != profile.ProviderAccountID ||
		proposal.Route.CredentialRevision != profile.CredentialRevision ||
		proposal.Route.ModelID != profile.ModelID || proposal.Argument != objective {
		t.Fatalf("%s %s returned an invalid or drifted Mission Proposal", profile.HarnessAdapter, decisionCase)
	}
	requirePhase7CompletedTools(t, thread, controltool.ToolMissionsCreatePreview)
	return phase7LiveProposalRecord{
		harness: profile.HarnessAdapter, decisionCase: decisionCase,
		threadID: threadID, proposal: proposal,
	}
}

func phase7LiveCompletedToolIDs(thread api.LocalProductChatThread) []controltool.ToolID {
	if len(thread.Attempts) == 0 {
		return nil
	}
	calls := thread.Attempts[len(thread.Attempts)-1].CompletedControlTools
	toolIDs := make([]controltool.ToolID, len(calls))
	for index, call := range calls {
		toolIDs[index] = call.ToolID
	}
	return toolIDs
}

func runPhase7LiveCapabilitySweep(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	runID string,
	profiles []phase7LiveProfile,
	sourceA string,
	sourceB string,
	governanceAnchor phase7LiveGovernanceAnchor,
	incidentID string,
) []phase7LiveCapabilityRecord {
	t.Helper()
	if len(profiles) < 4 || len(profiles) > 5 {
		t.Fatalf("capability sweep profiles = %d, want 4 or 5", len(profiles))
	}
	if incidentID == "" {
		t.Fatal("capability sweep requires a real Incident target")
	}
	sourceCatalog := []controltool.SessionReference{
		{ConversationID: sourceA, Title: "P7 source alpha", UpdatedAt: time.Now().UTC()},
		{ConversationID: sourceB, Title: "P7 source beta", UpdatedAt: time.Now().UTC()},
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definitions := registry.Definitions()
	records := make([]phase7LiveCapabilityRecord, 0, len(definitions))
	statusFixture, statusView := createPhase7LiveRoundTableFixture(
		t, ctx, client, "status-pause", governanceAnchor,
	)
	statusTarget, err := selectPhase7RoundTableActionTarget(
		statusView, controltool.ToolRoundtablesPausePreview,
	)
	if err != nil {
		t.Fatalf("isolated status/pause target: %v", err)
	}
	var retryFixture *phase7LiveRoundTableFixture
	var retryTarget phase7LiveGovernanceTargets
	for index := range definitions {
		profile := profiles[index%len(profiles)]
		fixture := phase7LiveCapabilityFixture{
			runID: runID, sourceA: sourceA, sourceB: sourceB,
			missionID:      governanceAnchor.missionID,
			teamInstanceID: governanceAnchor.teamInstanceID,
			roundSessionID: statusFixture.sessionID,
			roundID:        statusFixture.roundID,
			incidentID:     incidentID, profileID: profile.ProfileID,
			modelID: profile.ModelID,
		}
		groups := phase7LiveCapabilityGroups(fixture)
		if len(groups) != len(definitions) {
			t.Fatalf("capability turns = %d, want %d", len(groups), len(definitions))
		}
		group := groups[index]
		switch group.name {
		case "roundtables-pause-preview":
			fixture.roundSessionID = statusTarget.roundSessionID
			fixture.roundID = statusTarget.roundID
			group = phase7LiveCapabilityGroups(fixture)[index]
		case "roundtables-retry-preview",
			"roundtables-skip-preview",
			"roundtables-replace-preview":
			if retryFixture == nil {
				t.Fatal("cancelled RoundTable fixture was not prepared")
			}
			toolID := group.expected[0]
			target, _, targetErr := waitForPhase7LiveRoundTableTarget(
				ctx, client, retryFixture.sessionID, toolID, 10*time.Second,
			)
			if targetErr != nil {
				t.Fatalf("isolated %s target: %v", group.name, targetErr)
			}
			retryTarget = target
			fixture.roundSessionID = target.roundSessionID
			fixture.roundID = target.roundID
			fixture.seatID = target.seatID
			fixture.attemptID = target.attemptID
			group = phase7LiveCapabilityGroups(fixture)[index]
		}
		threadID := ""
		var thread api.LocalProductChatThread
		var steerFixture *phase7LiveRoundTableFixture
		for dispatch := 1; dispatch <= 2; dispatch++ {
			if group.name == "roundtables-steer-preview" {
				if steerFixture != nil {
					if err := closePhase7LiveRoundTableFixture(ctx, client, steerFixture); err != nil {
						t.Fatalf("close stale isolated steer fixture: %v", err)
					}
				}
				var steerView roundtable.View
				steerFixture, steerView = createPhase7LiveRoundTableFixture(
					t, ctx, client,
					fmt.Sprintf("steer-%d", dispatch), governanceAnchor,
				)
				target, targetErr := selectPhase7RoundTableActionTarget(
					steerView, controltool.ToolRoundtablesSteerPreview,
				)
				if targetErr != nil {
					t.Fatalf("isolated steer target: %v", targetErr)
				}
				fixture.roundSessionID = target.roundSessionID
				fixture.roundID = target.roundID
				fixture.seatID = target.seatID
				fixture.attemptID = target.attemptID
				group = phase7LiveCapabilityGroups(fixture)[index]
			}
			threadID = fmt.Sprintf("p7-%s-sweep-%02d", runID, index+1)
			if dispatch > 1 {
				threadID += fmt.Sprintf("-retry-%d", dispatch-1)
			}
			catalog := append([]controltool.SessionReference(nil), sourceCatalog...)
			catalog = append(catalog, controltool.SessionReference{
				ConversationID: threadID, Title: "P7 capability turn " + group.name,
				UpdatedAt: time.Now().UTC(),
			})
			request := api.LocalProductChatMessageRequest{
				ThreadID: threadID, Content: group.prompt,
				ProfileID: profile.ProfileID, ModelID: profile.ModelID,
				ContextMode:              api.ContextModeStartClean,
				ExpectedExecutionBinding: phase7LiveBinding(profile.ConversationProviderProfile),
				SessionCatalog:           catalog,
			}
			if err := client.Call(ctx, "chat_message", request, &thread); err != nil {
				t.Fatalf(
					"%s capability %s thread %s model turn: %v",
					profile.HarnessAdapter, group.name, threadID, err,
				)
			}
			failure := phase7LiveThreadFailure(thread)
			if failure == "" {
				break
			}
			if dispatch == 2 || !phase7LiveThreadFailureRetryable(thread) ||
				len(thread.ControlProposals) != 0 || len(thread.ActionProposals) != 0 {
				t.Fatalf(
					"%s capability %s thread %s model turn: %s",
					profile.HarnessAdapter, group.name, threadID, failure,
				)
			}
			t.Logf(
				"%s capability %s retrying one pre-Proposal transient failure on a fresh thread: %s",
				profile.HarnessAdapter, group.name, failure,
			)
		}
		requirePhase7CompletedTools(t, thread, group.expected...)
		requirePhase7LiveProposalToolSet(t, thread, group.proposalTools)
		if len(group.proposalTools) > 0 {
			thread = cancelPhase7LiveSweepProposals(t, ctx, client, thread)
			requirePhase7SweepProposalsCancelled(t, thread, group.proposalTools)
		}
		requirePhase7LiveProposalTargets(
			t, thread, fixture, profile.HarnessAdapter, group.name,
		)
		records = append(records, phase7LiveCapabilityRecord{
			harness: profile.HarnessAdapter, name: group.name, threadID: threadID,
			expected:      append([]controltool.ToolID(nil), group.expected...),
			proposalTools: append([]controltool.ToolID(nil), group.proposalTools...),
		})
		switch group.name {
		case "roundtables-pause-preview":
			if err := closePhase7LiveRoundTableFixture(ctx, client, statusFixture); err != nil {
				t.Fatalf("close isolated status/pause RoundTable: %v", err)
			}
			retryFixture, _ = createPhase7LiveRoundTableFixture(
				t, ctx, client, "retry-skip-replace", governanceAnchor,
			)
			if _, err := pausePhase7LiveRoundTableFixture(ctx, client, retryFixture); err != nil {
				t.Fatalf("pause isolated retry fixture: %v", err)
			}
			var targetErr error
			retryTarget, _, targetErr = waitForPhase7LiveRoundTableTarget(
				ctx, client, retryFixture.sessionID,
				controltool.ToolRoundtablesRetryPreview, 10*time.Second,
			)
			if targetErr != nil {
				t.Fatalf("cancelled Retry target did not materialize: %v", targetErr)
			}
		case "roundtables-steer-preview":
			if steerFixture == nil {
				t.Fatal("isolated steer fixture missing")
			}
			if err := closePhase7LiveRoundTableFixture(ctx, client, steerFixture); err != nil {
				t.Fatalf("close isolated steer RoundTable: %v", err)
			}
		case "roundtables-replace-preview":
			if retryFixture == nil || retryTarget.roundSessionID == "" {
				t.Fatal("isolated retry/skip/replace fixture missing")
			}
			if err := closePhase7LiveRoundTableFixture(ctx, client, retryFixture); err != nil {
				t.Fatalf("close isolated retry/skip/replace RoundTable: %v", err)
			}
		}
	}
	return records
}

func requirePhase7LiveProposalTargets(
	t *testing.T,
	thread api.LocalProductChatThread,
	fixture phase7LiveCapabilityFixture,
	harness string,
	capability string,
) {
	t.Helper()
	for _, proposal := range thread.ControlProposals {
		if proposal.ToolID != controltool.ToolSessionsAlignPreview ||
			len(proposal.Sources) != 2 {
			t.Fatalf("unexpected alignment target shape")
		}
		sources := map[string]bool{
			proposal.Sources[0].ConversationID: true,
			proposal.Sources[1].ConversationID: true,
		}
		if !sources[fixture.sourceA] || !sources[fixture.sourceB] ||
			proposal.ContextMode != controltool.ContextModeSummaryOnly {
			t.Fatal("alignment Proposal did not bind the exact source conversations")
		}
	}
	for _, proposal := range thread.ActionProposals {
		payload := proposal.Payload
		switch proposal.ToolID {
		case controltool.ToolMissionsCreatePreview,
			controltool.ToolTeamsCreatePreview:
			if payload != nil || !strings.Contains(proposal.Argument, fixture.runID) {
				t.Fatalf("%s did not bind the sentinel objective", proposal.ToolID)
			}
		case controltool.ToolMissionsContinuePreview:
			if payload == nil || payload.MissionID != fixture.missionID ||
				proposal.Argument != "Continue safely" {
				t.Fatal("Mission continuation did not bind the exact installed Mission")
			}
		case controltool.ToolRoundtablesOpenPreview:
			if payload == nil || payload.MissionID != fixture.missionID {
				t.Fatal("RoundTable open did not bind the exact installed Mission")
			}
		case controltool.ToolConversationRouteChangePreview:
			if payload == nil || payload.ProfileID != fixture.profileID {
				t.Fatal("Route proposal did not bind the exact profile")
			}
		case controltool.ToolConversationModelChangePreview:
			if payload == nil || payload.ModelID != fixture.modelID {
				t.Fatal("Model proposal did not bind the exact model")
			}
		case controltool.ToolConversationReasoningChangePreview:
			if payload == nil || payload.ReasoningEffort != "high" {
				t.Fatal("Reasoning proposal did not bind high")
			}
		case controltool.ToolWorkspaceChoosePreview:
			if payload != nil {
				t.Fatal("workspace picker Proposal exposed a model-selected target")
			}
		case controltool.ToolTeamsEditPreview:
			if payload == nil || payload.TeamInstanceID != fixture.teamInstanceID ||
				payload.Instruction != "Review role bindings" {
				t.Fatal("Team edit did not bind the exact installed Agent Team")
			}
		case controltool.ToolRoundtablesPausePreview:
			requirePhase7RoundTableTarget(
				t, payload, fixture, false, false, harness, capability, proposal.ToolID,
			)
		case controltool.ToolRoundtablesSteerPreview:
			requirePhase7RoundTableTarget(
				t, payload, fixture, true, true, harness, capability, proposal.ToolID,
			)
			if payload.Guidance != "Focus on evidence" {
				t.Fatal("RoundTable steer guidance drifted")
			}
		case controltool.ToolRoundtablesRetryPreview:
			requirePhase7RoundTableTarget(
				t, payload, fixture, true, true, harness, capability, proposal.ToolID,
			)
			if payload.Guidance != "Retry from evidence" {
				t.Fatal("RoundTable retry guidance drifted")
			}
		case controltool.ToolRoundtablesSkipPreview,
			controltool.ToolRoundtablesReplacePreview:
			requirePhase7RoundTableTarget(
				t, payload, fixture, true, false, harness, capability, proposal.ToolID,
			)
		default:
			t.Fatalf("unexpected Proposal Tool target %s", proposal.ToolID)
		}
	}
}

func requirePhase7RoundTableTarget(
	t *testing.T,
	payload *controltool.ConversationActionPayload,
	fixture phase7LiveCapabilityFixture,
	wantSeat bool,
	wantAttempt bool,
	harness string,
	capability string,
	toolID controltool.ToolID,
) {
	t.Helper()
	if payload == nil || payload.SessionID != fixture.roundSessionID ||
		payload.RoundID != fixture.roundID ||
		wantSeat && payload.SeatID != fixture.seatID ||
		!wantSeat && payload.SeatID != "" ||
		wantAttempt && payload.AttemptID != fixture.attemptID ||
		!wantAttempt && payload.AttemptID != "" {
		actual := controltool.ConversationActionPayload{}
		if payload != nil {
			actual = *payload
		}
		t.Fatalf(
			"%s capability %s tool %s RoundTable target drifted: session=%q want=%q round=%q want=%q seat=%q want=%q attempt=%q want=%q",
			harness, capability, toolID,
			actual.SessionID, fixture.roundSessionID,
			actual.RoundID, fixture.roundID,
			actual.SeatID, map[bool]string{true: fixture.seatID}[wantSeat],
			actual.AttemptID, map[bool]string{true: fixture.attemptID}[wantAttempt],
		)
	}
}

func requirePhase7LiveProductSnapshot(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
) api.LocalProductSnapshot {
	t.Helper()
	snapshot, err := waitForPhase7LiveProductSnapshot(
		ctx,
		func(
			ctx context.Context,
			request api.LocalProductSnapshotRequest,
		) (api.LocalProductSnapshot, error) {
			var page api.LocalProductSnapshot
			err := client.Call(ctx, "snapshot", request, &page)
			return page, err
		},
		250*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("complete product snapshot: %v", err)
	}
	return snapshot
}

func waitForPhase7LiveProductSnapshot(
	ctx context.Context,
	read phase7LiveSnapshotReader,
	retryDelay time.Duration,
) (api.LocalProductSnapshot, error) {
	for {
		snapshot, err := collectPhase7LiveProductSnapshot(ctx, read)
		if err == nil {
			return snapshot, nil
		}
		if !errors.Is(err, errPhase7ObserverModelsWarmup) {
			return api.LocalProductSnapshot{}, err
		}
		select {
		case <-ctx.Done():
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"observer model warmup did not settle: %w (last snapshot: %v)",
				ctx.Err(), err,
			)
		default:
		}
		if retryDelay <= 0 {
			continue
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"observer model warmup did not settle: %w (last snapshot: %v)",
				ctx.Err(), err,
			)
		case <-timer.C:
		}
	}
}

type phase7LiveSnapshotReader func(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error)

func collectPhase7LiveProductSnapshot(
	ctx context.Context,
	read phase7LiveSnapshotReader,
) (api.LocalProductSnapshot, error) {
	const (
		pageLimit = 64
		maxPages  = 128
	)
	if read == nil {
		return api.LocalProductSnapshot{}, errors.New("product snapshot reader is required")
	}

	request := api.LocalProductSnapshotRequest{Limit: pageLimit}
	var aggregate api.LocalProductSnapshot
	seenRuntimes := make(map[string]struct{})
	seenTeams := make(map[string]struct{})
	seenMissions := make(map[string]struct{})
	seenRuns := make(map[string]struct{})
	seenEvidence := make(map[string]struct{})
	seenAttention := make(map[string]struct{})
	runtimeComplete := false
	teamComplete := false
	runComplete := false
	evidenceComplete := false

	for pageNumber := 1; pageNumber <= maxPages; pageNumber++ {
		page, err := read(ctx, request)
		if err != nil {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"read page %d: %w", pageNumber, err,
			)
		}
		if page.Stale {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d is stale (%s)", pageNumber, page.Reason,
			)
		}
		if page.SchemaVersion < 1 || page.ViewVersion == "" {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d has incomplete authority metadata", pageNumber,
			)
		}
		if page.TeamPage != page.MissionPage {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d has divergent Team and Mission cursors", pageNumber,
			)
		}
		pageHasMore := page.RuntimePage.HasMore || page.TeamPage.HasMore ||
			page.RunPage.HasMore || page.EvidencePage.HasMore
		if page.Reason != "" {
			if page.Reason == "observer_models_timeout" {
				return api.LocalProductSnapshot{}, fmt.Errorf(
					"page %d is incomplete (%s): %w",
					pageNumber, page.Reason, errPhase7ObserverModelsWarmup,
				)
			}
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d is incomplete (%s)", pageNumber, page.Reason,
			)
		}
		if page.Partial != pageHasMore {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d has an unclassified partial state: partial=%t pagination=%t",
				pageNumber, page.Partial, pageHasMore,
			)
		}

		if pageNumber == 1 {
			aggregate = page
			aggregate.Runtimes = []api.LocalProductRuntimeSummary{}
			aggregate.Teams = []api.LocalProductTeamSummary{}
			aggregate.Missions = []api.LocalProductMissionSummary{}
			aggregate.Runs = []api.LocalProductRunSummary{}
			aggregate.Evidence = []api.LocalProductEvidenceSummary{}
			aggregate.Attention = []api.AttentionItem{}
		} else {
			if page.SchemaVersion != aggregate.SchemaVersion ||
				page.ViewVersion != aggregate.ViewVersion ||
				page.Health != aggregate.Health {
				return api.LocalProductSnapshot{}, fmt.Errorf(
					"page %d changed snapshot authority", pageNumber,
				)
			}
			if !reflect.DeepEqual(
				page.PreparedDecisions,
				aggregate.PreparedDecisions,
			) || !reflect.DeepEqual(page.SideTasks, aggregate.SideTasks) {
				return api.LocalProductSnapshot{}, fmt.Errorf(
					"page %d changed non-paged governance state", pageNumber,
				)
			}
		}

		if !runtimeComplete {
			if err := appendPhase7UniqueSnapshotRows(
				&aggregate.Runtimes, page.Runtimes, seenRuntimes,
				func(row api.LocalProductRuntimeSummary) string {
					return row.RuntimeInstanceID
				}, "Runtime",
			); err != nil {
				return api.LocalProductSnapshot{}, err
			}
		} else if len(page.Runtimes) != 0 || page.RuntimePage.HasMore {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d resumed completed Runtime pagination", pageNumber,
			)
		}
		if !teamComplete {
			if err := appendPhase7UniqueSnapshotRows(
				&aggregate.Teams, page.Teams, seenTeams,
				func(row api.LocalProductTeamSummary) string {
					return row.TeamInstanceID
				}, "Team",
			); err != nil {
				return api.LocalProductSnapshot{}, err
			}
			if err := appendPhase7UniqueSnapshotRows(
				&aggregate.Missions, page.Missions, seenMissions,
				func(row api.LocalProductMissionSummary) string {
					return row.MissionID
				}, "Mission",
			); err != nil {
				return api.LocalProductSnapshot{}, err
			}
		} else if len(page.Teams) != 0 || len(page.Missions) != 0 ||
			page.TeamPage.HasMore {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d resumed completed Team pagination", pageNumber,
			)
		}
		if !runComplete {
			if err := appendPhase7UniqueSnapshotRows(
				&aggregate.Runs, page.Runs, seenRuns,
				func(row api.LocalProductRunSummary) string { return row.RunID },
				"Run",
			); err != nil {
				return api.LocalProductSnapshot{}, err
			}
		} else if len(page.Runs) != 0 || page.RunPage.HasMore {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d resumed completed Run pagination", pageNumber,
			)
		}
		if !evidenceComplete {
			if err := appendPhase7UniqueSnapshotRows(
				&aggregate.Evidence, page.Evidence, seenEvidence,
				func(row api.LocalProductEvidenceSummary) string {
					return row.EvidenceID
				}, "Evidence",
			); err != nil {
				return api.LocalProductSnapshot{}, err
			}
		} else if len(page.Evidence) != 0 || page.EvidencePage.HasMore {
			return api.LocalProductSnapshot{}, fmt.Errorf(
				"page %d resumed completed Evidence pagination", pageNumber,
			)
		}
		for _, item := range page.Attention {
			if item.AttentionID == "" {
				return api.LocalProductSnapshot{}, errors.New(
					"product snapshot contains empty Attention identity",
				)
			}
			if _, exists := seenAttention[item.AttentionID]; exists {
				continue
			}
			seenAttention[item.AttentionID] = struct{}{}
			aggregate.Attention = append(aggregate.Attention, item)
		}

		request.AfterRuntimeID, runtimeComplete, err =
			advancePhase7SnapshotCursor(
				request.AfterRuntimeID, runtimeComplete,
				page.RuntimePage, "Runtime",
			)
		if err != nil {
			return api.LocalProductSnapshot{}, err
		}
		request.AfterTeamID, teamComplete, err = advancePhase7SnapshotCursor(
			request.AfterTeamID, teamComplete, page.TeamPage, "Team",
		)
		if err != nil {
			return api.LocalProductSnapshot{}, err
		}
		request.AfterRunID, runComplete, err = advancePhase7SnapshotCursor(
			request.AfterRunID, runComplete, page.RunPage, "Run",
		)
		if err != nil {
			return api.LocalProductSnapshot{}, err
		}
		request.AfterEvidenceID, evidenceComplete, err =
			advancePhase7SnapshotCursor(
				request.AfterEvidenceID, evidenceComplete,
				page.EvidencePage, "Evidence",
			)
		if err != nil {
			return api.LocalProductSnapshot{}, err
		}

		if runtimeComplete && teamComplete && runComplete && evidenceComplete {
			aggregate.Partial = false
			aggregate.Stale = false
			aggregate.Reason = ""
			aggregate.RuntimePage = api.LocalProductPageCursor{
				NextCursor: request.AfterRuntimeID,
			}
			aggregate.TeamPage = api.LocalProductPageCursor{
				NextCursor: request.AfterTeamID,
			}
			aggregate.MissionPage = aggregate.TeamPage
			aggregate.RunPage = api.LocalProductPageCursor{
				NextCursor: request.AfterRunID,
			}
			aggregate.EvidencePage = api.LocalProductPageCursor{
				NextCursor: request.AfterEvidenceID,
			}
			return aggregate, nil
		}
	}
	return api.LocalProductSnapshot{}, fmt.Errorf(
		"product snapshot exceeded %d pages", maxPages,
	)
}

func appendPhase7UniqueSnapshotRows[T any](
	destination *[]T,
	rows []T,
	seen map[string]struct{},
	id func(T) string,
	label string,
) error {
	for _, row := range rows {
		identity := id(row)
		if identity == "" {
			return fmt.Errorf("product snapshot contains empty %s identity", label)
		}
		if _, exists := seen[identity]; exists {
			return fmt.Errorf("product snapshot repeated %s %q", label, identity)
		}
		seen[identity] = struct{}{}
		*destination = append(*destination, row)
	}
	return nil
}

func advancePhase7SnapshotCursor(
	current string,
	complete bool,
	page api.LocalProductPageCursor,
	label string,
) (string, bool, error) {
	if complete {
		return current, true, nil
	}
	if page.NextCursor != "" {
		if page.NextCursor == current {
			return "", false, fmt.Errorf(
				"%s snapshot cursor did not advance from %q", label, current,
			)
		}
		current = page.NextCursor
	}
	if page.HasMore && page.NextCursor == "" {
		return "", false, fmt.Errorf(
			"%s snapshot page has more rows without a cursor", label,
		)
	}
	return current, !page.HasMore, nil
}

func TestCollectPhase7LiveProductSnapshotPaginatesIndependentViews(t *testing.T) {
	health := api.LocalProductHealth{
		Daemon: "serving_request", Journal: "available", Projection: "current",
	}
	sideTasks := []api.LocalProductSideTaskSummary{{SideTaskID: "side-existing"}}
	pages := []api.LocalProductSnapshot{
		{
			SchemaVersion: 1, ViewVersion: strings.Repeat("a", 64),
			Partial: true, Health: health,
			Runtimes: []api.LocalProductRuntimeSummary{{
				RuntimeInstanceID: "runtime.existing",
			}},
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team.existing.1",
			}},
			Missions: []api.LocalProductMissionSummary{{
				MissionID: "mission/team.existing.1",
			}},
			Runs: []api.LocalProductRunSummary{{RunID: "run.existing.1"}},
			Evidence: []api.LocalProductEvidenceSummary{{
				EvidenceID: "evidence.existing",
			}},
			Attention: []api.AttentionItem{{AttentionID: "attention.existing.1"}},
			SideTasks: sideTasks,
			RuntimePage: api.LocalProductPageCursor{
				NextCursor: "runtime.existing",
			},
			TeamPage: api.LocalProductPageCursor{
				NextCursor: "team.existing.1", HasMore: true,
			},
			MissionPage: api.LocalProductPageCursor{
				NextCursor: "team.existing.1", HasMore: true,
			},
			RunPage: api.LocalProductPageCursor{
				NextCursor: "run.existing.1", HasMore: true,
			},
			EvidencePage: api.LocalProductPageCursor{
				NextCursor: "evidence.existing",
			},
		},
		{
			SchemaVersion: 1, ViewVersion: strings.Repeat("a", 64), Health: health,
			Teams: []api.LocalProductTeamSummary{{
				TeamInstanceID: "team.existing.2",
			}},
			Missions: []api.LocalProductMissionSummary{{
				MissionID: "mission/team.existing.2",
			}},
			Runs:      []api.LocalProductRunSummary{{RunID: "run.existing.2"}},
			Attention: []api.AttentionItem{{AttentionID: "attention.existing.2"}},
			SideTasks: sideTasks,
			TeamPage: api.LocalProductPageCursor{
				NextCursor: "team.existing.2",
			},
			MissionPage: api.LocalProductPageCursor{
				NextCursor: "team.existing.2",
			},
			RunPage: api.LocalProductPageCursor{
				NextCursor: "run.existing.2",
			},
		},
	}
	readCount := 0
	snapshot, err := collectPhase7LiveProductSnapshot(
		context.Background(),
		func(
			_ context.Context,
			request api.LocalProductSnapshotRequest,
		) (api.LocalProductSnapshot, error) {
			if request.Limit != 64 {
				t.Fatalf("snapshot limit = %d, want 64", request.Limit)
			}
			if readCount == 1 && (request.AfterRuntimeID != "runtime.existing" ||
				request.AfterTeamID != "team.existing.1" ||
				request.AfterRunID != "run.existing.1" ||
				request.AfterEvidenceID != "evidence.existing") {
				t.Fatalf("second snapshot cursors = %#v", request)
			}
			if readCount >= len(pages) {
				return api.LocalProductSnapshot{}, errors.New("unexpected extra page")
			}
			page := pages[readCount]
			readCount++
			return page, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if readCount != 2 || len(snapshot.Runtimes) != 1 ||
		len(snapshot.Teams) != 2 || len(snapshot.Missions) != 2 ||
		len(snapshot.Runs) != 2 || len(snapshot.Evidence) != 1 ||
		len(snapshot.Attention) != 2 || len(snapshot.SideTasks) != 1 ||
		snapshot.Partial || snapshot.Stale || snapshot.TeamPage.HasMore ||
		snapshot.MissionPage.HasMore || snapshot.RunPage.HasMore ||
		snapshot.EvidencePage.HasMore {
		t.Fatalf("aggregated snapshot = %#v", snapshot)
	}
}

func TestCollectPhase7LiveProductSnapshotRejectsAuthorityDrift(t *testing.T) {
	readCount := 0
	_, err := collectPhase7LiveProductSnapshot(
		context.Background(),
		func(
			_ context.Context,
			_ api.LocalProductSnapshotRequest,
		) (api.LocalProductSnapshot, error) {
			readCount++
			version := strings.Repeat("a", 64)
			if readCount == 2 {
				version = strings.Repeat("b", 64)
			}
			return api.LocalProductSnapshot{
				SchemaVersion: 1, ViewVersion: version,
				Partial: readCount == 1,
				Health: api.LocalProductHealth{
					Daemon: "serving_request", Journal: "available", Projection: "current",
				},
				TeamPage: api.LocalProductPageCursor{
					NextCursor: fmt.Sprintf("team.%d", readCount),
					HasMore:    readCount == 1,
				},
				MissionPage: api.LocalProductPageCursor{
					NextCursor: fmt.Sprintf("team.%d", readCount),
					HasMore:    readCount == 1,
				},
			}, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "changed snapshot authority") {
		t.Fatalf("authority drift error = %v", err)
	}
}

func TestCollectPhase7LiveProductSnapshotRejectsUnpageablePartial(t *testing.T) {
	_, err := collectPhase7LiveProductSnapshot(
		context.Background(),
		func(
			_ context.Context,
			_ api.LocalProductSnapshotRequest,
		) (api.LocalProductSnapshot, error) {
			return api.LocalProductSnapshot{
				SchemaVersion: 1, ViewVersion: strings.Repeat("a", 64),
				Partial: true,
			}, nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "unclassified partial state") {
		t.Fatalf("unpageable partial error = %v", err)
	}
}

func TestWaitForPhase7LiveProductSnapshotRetriesOnlyObserverWarmup(t *testing.T) {
	reads := 0
	snapshot, err := waitForPhase7LiveProductSnapshot(
		context.Background(),
		func(
			_ context.Context,
			_ api.LocalProductSnapshotRequest,
		) (api.LocalProductSnapshot, error) {
			reads++
			page := api.LocalProductSnapshot{
				SchemaVersion: 1, ViewVersion: strings.Repeat("a", 64),
			}
			if reads == 1 {
				page.Partial = true
				page.Reason = "observer_models_timeout"
			}
			return page, nil
		},
		0,
	)
	if err != nil || reads != 2 || snapshot.ViewVersion == "" {
		t.Fatalf("warmup snapshot = %#v, reads = %d, error = %v", snapshot, reads, err)
	}

	reads = 0
	_, err = waitForPhase7LiveProductSnapshot(
		context.Background(),
		func(
			_ context.Context,
			_ api.LocalProductSnapshotRequest,
		) (api.LocalProductSnapshot, error) {
			reads++
			return api.LocalProductSnapshot{
				SchemaVersion: 1, ViewVersion: strings.Repeat("a", 64),
				Partial: true, Reason: "tentative_overflow",
			}, nil
		},
		0,
	)
	if err == nil || reads != 1 {
		t.Fatalf("nonretryable snapshot reads = %d, error = %v", reads, err)
	}
}

func requirePhase7GovernanceAnchor(
	t *testing.T,
	snapshot api.LocalProductSnapshot,
) phase7LiveGovernanceAnchor {
	t.Helper()
	expectedMissionID := strings.TrimSpace(os.Getenv(phase7LiveGovernanceMissionGate))
	expectedTeamID := strings.TrimSpace(os.Getenv(phase7LiveGovernanceTeamGate))
	if (expectedMissionID == "") != (expectedTeamID == "") ||
		expectedMissionID != "" &&
			(!validPhase7RoundtableRegistryID(expectedMissionID) ||
				!validPhase7RoundtableRegistryID(expectedTeamID)) {
		t.Fatalf(
			"explicit governance anchor requires valid %s and %s",
			phase7LiveGovernanceMissionGate, phase7LiveGovernanceTeamGate,
		)
	}
	anchor, err := selectPhase7GovernanceAnchor(
		snapshot, expectedMissionID, expectedTeamID,
	)
	if err != nil {
		teams := make(map[string]api.LocalProductTeamSummary, len(snapshot.Teams))
		for _, team := range snapshot.Teams {
			teams[team.TeamInstanceID] = team
		}
		logged := 0
		omitted := 0
		for _, mission := range snapshot.Missions {
			if mission.Status != "blocked" {
				continue
			}
			if logged >= 16 {
				omitted++
				continue
			}
			configured := 0
			mainConfigured := false
			for _, agent := range teams[mission.TeamInstanceID].Agents {
				if agent.BindingStatus == "configured" {
					configured++
					mainConfigured = mainConfigured || agent.RoleKind == "main"
				}
			}
			marked := strings.Contains(strings.ToLower(
				mission.MissionID+" "+mission.TeamInstanceID+" "+mission.Title,
			), "p7")
			t.Logf(
				"governance anchor candidate: mission=%s team=%s configured_agents=%d main_configured=%t p7_marked=%t",
				mission.MissionID, mission.TeamInstanceID, configured, mainConfigured, marked,
			)
			logged++
		}
		if omitted > 0 {
			t.Logf("governance anchor candidates omitted from diagnostic: %d", omitted)
		}
		t.Fatal(err)
	}
	return anchor
}

type phase7RoundtableViewReader func(context.Context, string) (roundtable.View, error)

func waitForPhase7RoundtableView(
	ctx context.Context,
	maximum time.Duration,
	sessionID string,
	read phase7RoundtableViewReader,
) (roundtable.View, error) {
	if ctx == nil || maximum < 0 || sessionID == "" || read == nil {
		return roundtable.View{}, errors.New("invalid RoundTable readiness probe")
	}
	deadline := time.Now().Add(maximum)
	for {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		view, err := read(callCtx, sessionID)
		cancel()
		if err == nil && view.Session.ID == sessionID {
			return view, nil
		}
		if maximum == 0 || !time.Now().Before(deadline) {
			return roundtable.View{}, errors.New("RoundTable projection readiness timeout")
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return roundtable.View{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func TestWaitForPhase7RoundtableViewRetriesBoundedStartup(t *testing.T) {
	reads := 0
	view, err := waitForPhase7RoundtableView(
		context.Background(), time.Second, "rt-ready",
		func(context.Context, string) (roundtable.View, error) {
			reads++
			if reads < 3 {
				return roundtable.View{}, errors.New("not ready")
			}
			return roundtable.View{Session: roundtable.Session{ID: "rt-ready"}}, nil
		},
	)
	if err != nil || view.Session.ID != "rt-ready" || reads != 3 {
		t.Fatalf("RoundTable readiness view=%s reads=%d error=%v", view.Session.ID, reads, err)
	}
	reads = 0
	if _, err := waitForPhase7RoundtableView(
		context.Background(), 0, "rt-missing",
		func(context.Context, string) (roundtable.View, error) {
			reads++
			return roundtable.View{}, errors.New("still unavailable")
		},
	); err == nil || reads != 1 {
		t.Fatalf("RoundTable timeout reads=%d error=%v", reads, err)
	}
}

func selectPhase7GovernanceAnchor(
	snapshot api.LocalProductSnapshot,
	expectedMissionID string,
	expectedTeamID string,
) (phase7LiveGovernanceAnchor, error) {
	teams := make(map[string]api.LocalProductTeamSummary, len(snapshot.Teams))
	for _, team := range snapshot.Teams {
		if team.TeamInstanceID != "" {
			teams[team.TeamInstanceID] = team
		}
	}
	missions := append([]api.LocalProductMissionSummary(nil), snapshot.Missions...)
	sort.Slice(missions, func(left, right int) bool {
		return missions[left].MissionID < missions[right].MissionID
	})
	candidates := make([]phase7LiveGovernanceAnchor, 0, 2)
	for _, mission := range missions {
		marked := strings.Contains(strings.ToLower(
			mission.MissionID+" "+mission.TeamInstanceID+" "+mission.Title,
		), "p7")
		if mission.Status != "blocked" || mission.MissionID == "" ||
			mission.TeamInstanceID == "" || expectedMissionID == "" && !marked {
			continue
		}
		if expectedMissionID != "" &&
			(mission.MissionID != expectedMissionID ||
				mission.TeamInstanceID != expectedTeamID) {
			continue
		}
		team, found := teams[mission.TeamInstanceID]
		if !found {
			continue
		}
		configured := make([]api.LocalProductTeamAgentSummary, 0, len(team.Agents))
		seen := make(map[string]struct{}, len(team.Agents))
		for _, agent := range team.Agents {
			if agent.BindingStatus != "configured" || agent.AgentDefinitionID == "" ||
				agent.RuntimeProfileID == "" ||
				(agent.RoleKind != "main" && agent.RoleKind != "subagent") {
				continue
			}
			if _, duplicate := seen[agent.AgentDefinitionID]; duplicate {
				continue
			}
			seen[agent.AgentDefinitionID] = struct{}{}
			configured = append(configured, agent)
		}
		mainIndex := -1
		for index := range configured {
			if configured[index].RoleKind == "main" {
				mainIndex = index
				break
			}
		}
		if mainIndex < 0 || len(configured) < roundtable.MinAgentSeats {
			continue
		}
		agents := []api.LocalProductTeamAgentSummary{configured[mainIndex]}
		for index := range configured {
			if index == mainIndex {
				continue
			}
			agents = append(agents, configured[index])
			if len(agents) == roundtable.MinAgentSeats {
				break
			}
		}
		candidates = append(candidates, phase7LiveGovernanceAnchor{
			missionID: mission.MissionID, teamInstanceID: mission.TeamInstanceID,
			agents: append([]api.LocalProductTeamAgentSummary(nil), agents...),
		})
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return phase7LiveGovernanceAnchor{}, errors.New(
			"installed acceptance has ambiguous P7 governance anchors; set the exact Mission and Team gates",
		)
	}
	return phase7LiveGovernanceAnchor{}, errors.New(
		"installed acceptance requires one P7 blocked Mission with two configured Agent bindings",
	)
}

func selectPhase7RoundTableActionTarget(
	view roundtable.View,
	toolID controltool.ToolID,
) (phase7LiveGovernanceTargets, error) {
	if view.Session.ID == "" || view.Session.Concluded || view.Session.Context == nil ||
		view.Session.Context.MissionID == "" || view.Session.Context.TeamID == "" ||
		len(view.Rounds) == 0 {
		return phase7LiveGovernanceTargets{}, errors.New("RoundTable action target is not current")
	}
	current := view.Rounds[len(view.Rounds)-1]
	base := phase7LiveGovernanceTargets{
		missionID:      view.Session.Context.MissionID,
		teamInstanceID: view.Session.Context.TeamID,
		roundSessionID: view.Session.ID,
		roundID:        current.ID,
	}
	if toolID == controltool.ToolRoundtablesPausePreview {
		if current.ID == "" || current.PauseRequested {
			return phase7LiveGovernanceTargets{}, errors.New("RoundTable round cannot be paused")
		}
		return base, nil
	}
	seatIDs := make([]string, 0, len(view.Seats))
	for seatID := range view.Seats {
		seatIDs = append(seatIDs, seatID)
	}
	sort.Strings(seatIDs)
	for _, seatID := range seatIDs {
		seat := view.Seats[seatID]
		if seat.ID != seatID || seatID == view.Session.ModeratorSeat ||
			!seat.Available || seat.Binding == nil {
			continue
		}
		candidate := base
		candidate.seatID = seatID
		switch toolID {
		case controltool.ToolRoundtablesSteerPreview,
			controltool.ToolRoundtablesRetryPreview:
			attemptIDs := make([]string, 0, len(view.Attempts))
			for attemptID := range view.Attempts {
				attemptIDs = append(attemptIDs, attemptID)
			}
			sort.Strings(attemptIDs)
			for _, attemptID := range attemptIDs {
				attempt := view.Attempts[attemptID]
				if attempt.AttemptID != attemptID || attempt.RoundID != current.ID ||
					attempt.SeatID != seatID ||
					attempt.MembershipRevision != seat.Binding.MembershipRevision ||
					attempt.SeatBindingDigest != seat.Binding.BindingDigest {
					continue
				}
				eligible := toolID == controltool.ToolRoundtablesSteerPreview &&
					attempt.Status == roundtable.SeatAttemptRunning
				eligible = eligible || toolID == controltool.ToolRoundtablesRetryPreview &&
					(attempt.Status == roundtable.SeatAttemptCancelled ||
						attempt.Status == roundtable.SeatAttemptFailed && attempt.Retryable)
				if eligible {
					candidate.attemptID = attemptID
					return candidate, nil
				}
			}
		case controltool.ToolRoundtablesSkipPreview,
			controltool.ToolRoundtablesReplacePreview:
			running := false
			for _, attempt := range view.Attempts {
				if attempt.RoundID == current.ID && attempt.SeatID == seatID &&
					attempt.Status == roundtable.SeatAttemptRunning {
					running = true
					break
				}
			}
			if running {
				continue
			}
			alreadySkipped := false
			for _, intervention := range view.Interventions {
				if intervention.Kind == roundtable.InterventionSkipSeat &&
					intervention.RoundID == current.ID && intervention.SeatID == seatID {
					alreadySkipped = true
					break
				}
			}
			if !alreadySkipped {
				return candidate, nil
			}
		default:
			return phase7LiveGovernanceTargets{}, errors.New("unsupported RoundTable action target")
		}
	}
	return phase7LiveGovernanceTargets{}, errors.New("no eligible RoundTable action target")
}

func createPhase7LiveRoundTableFixture(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	suffix string,
	anchor phase7LiveGovernanceAnchor,
) (*phase7LiveRoundTableFixture, roundtable.View) {
	t.Helper()
	if len(anchor.agents) != roundtable.MinAgentSeats || suffix == "" {
		t.Fatal("invalid isolated Phase 7 RoundTable fixture input")
	}
	fixtureToken := strings.ReplaceAll(matrixOperationID(), "-", "")[:16]
	fixture := &phase7LiveRoundTableFixture{
		suffix:        suffix,
		sessionID:     fmt.Sprintf("p7-rt-%s-%s", suffix, fixtureToken),
		roundID:       "p7-round-" + suffix,
		moderatorSeat: "p7-moderator",
		anchor: phase7LiveGovernanceAnchor{
			missionID: anchor.missionID, teamInstanceID: anchor.teamInstanceID,
			agents: append([]api.LocalProductTeamAgentSummary(nil), anchor.agents...),
		},
	}
	correlationID := matrixOperationID()
	var view roundtable.View
	if err := client.Call(ctx, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: roundtable.SchemaVersion,
		SessionID:     fixture.sessionID,
		ModeratorSeat: fixture.moderatorSeat,
		Title:         "Phase 7 isolated governance acceptance",
		Link: &roundtable.SessionLinkRequest{
			ConversationID: "p7-conversation-" + suffix + "-" + fixtureToken,
			MissionID:      anchor.missionID,
			TeamInstanceID: anchor.teamInstanceID,
		},
		CorrelationID: correlationID,
	}, &view); err != nil {
		t.Fatalf("create isolated RoundTable %s: %v", suffix, err)
	}
	t.Cleanup(func() {
		if fixture.closed {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := closePhase7LiveRoundTableFixture(cleanupCtx, client, fixture); err != nil {
			t.Logf("isolated RoundTable cleanup pending for %s: %v", fixture.sessionID, err)
		}
	})
	for index, agent := range anchor.agents {
		seatID := fmt.Sprintf("p7-seat-%s-%d", suffix, index+1)
		if err := client.Call(ctx, "roundtable_add_seat", productRoundtableAddSeatParams{
			SchemaVersion: roundtable.SchemaVersion,
			SessionID:     fixture.sessionID,
			SeatID:        seatID,
			DisplayName:   fmt.Sprintf("Phase 7 Agent %d", index+1),
			Selection: &roundtable.SeatBindingRequest{
				AgentDefinitionID: agent.AgentDefinitionID,
				TeamRoleKind:      agent.RoleKind,
				RuntimeProfileID:  agent.RuntimeProfileID,
			},
			CorrelationID: matrixOperationID(),
		}, &view); err != nil {
			t.Fatalf("add isolated RoundTable seat %d: %v", index+1, err)
		}
	}
	if err := client.Call(ctx, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: roundtable.SchemaVersion,
		SessionID:     fixture.sessionID,
		RoundID:       fixture.roundID,
		ModeratorSeat: fixture.moderatorSeat,
		Prompt:        phase7LiveRoundTablePrompt,
		CorrelationID: matrixOperationID(),
	}, &view); err != nil {
		t.Fatalf("open isolated RoundTable %s: %v", suffix, err)
	}
	if view.Session.ID != fixture.sessionID || len(view.Attempts) == 0 {
		t.Fatalf("isolated RoundTable %s did not start governed Attempts", suffix)
	}
	return fixture, view
}

func readPhase7LiveRoundTable(
	ctx context.Context,
	client *localipc.Client,
	sessionID string,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.Call(ctx, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: roundtable.SchemaVersion,
		SessionID:     sessionID,
	}, &view)
	return view, err
}

func ensurePhase7LiveRoundTableFixtureStarted(
	ctx context.Context,
	client *localipc.Client,
	fixture *phase7LiveRoundTableFixture,
) (roundtable.View, error) {
	if fixture == nil || fixture.closed ||
		len(fixture.anchor.agents) != roundtable.MinAgentSeats {
		return roundtable.View{}, errors.New("invalid isolated RoundTable fixture")
	}
	view, err := readPhase7LiveRoundTable(ctx, client, fixture.sessionID)
	if err != nil {
		return roundtable.View{}, err
	}
	if view.Session.Concluded {
		fixture.closed = true
		return view, nil
	}
	if view.Session.Context == nil ||
		view.Session.Context.MissionID != fixture.anchor.missionID ||
		view.Session.Context.TeamID != fixture.anchor.teamInstanceID {
		return roundtable.View{}, errors.New("isolated RoundTable authority drifted")
	}
	if len(view.Rounds) == 0 {
		for index, agent := range fixture.anchor.agents {
			seatID := fmt.Sprintf("p7-seat-%s-%d", fixture.suffix, index+1)
			if seat, found := view.Seats[seatID]; found {
				if !seat.Available || seat.Binding == nil ||
					seat.Binding.AgentDefinitionID != agent.AgentDefinitionID ||
					seat.Binding.TeamRoleKind != agent.RoleKind ||
					seat.Binding.RuntimeProfileID != agent.RuntimeProfileID {
					return roundtable.View{}, errors.New("isolated RoundTable seat drifted")
				}
				continue
			}
			if err := client.Call(ctx, "roundtable_add_seat", productRoundtableAddSeatParams{
				SchemaVersion: roundtable.SchemaVersion,
				SessionID:     fixture.sessionID,
				SeatID:        seatID,
				DisplayName:   fmt.Sprintf("Phase 7 Agent %d", index+1),
				Selection: &roundtable.SeatBindingRequest{
					AgentDefinitionID: agent.AgentDefinitionID,
					TeamRoleKind:      agent.RoleKind,
					RuntimeProfileID:  agent.RuntimeProfileID,
				},
				CorrelationID: matrixOperationID(),
			}, &view); err != nil {
				return roundtable.View{}, err
			}
		}
		if err := client.Call(ctx, "roundtable_open_round", productRoundtableOpenRoundParams{
			SchemaVersion: roundtable.SchemaVersion,
			SessionID:     fixture.sessionID,
			RoundID:       fixture.roundID,
			ModeratorSeat: fixture.moderatorSeat,
			Prompt:        phase7LiveRoundTablePrompt,
			CorrelationID: matrixOperationID(),
		}, &view); err != nil {
			return roundtable.View{}, err
		}
	}
	if len(view.Rounds) == 0 ||
		view.Rounds[len(view.Rounds)-1].ID != fixture.roundID ||
		len(view.Attempts) == 0 {
		return roundtable.View{}, errors.New("isolated RoundTable did not start")
	}
	return view, nil
}

func waitForPhase7LiveRoundTableTarget(
	ctx context.Context,
	client *localipc.Client,
	sessionID string,
	toolID controltool.ToolID,
	maximum time.Duration,
) (phase7LiveGovernanceTargets, roundtable.View, error) {
	deadline := time.Now().Add(maximum)
	for {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		view, readErr := readPhase7LiveRoundTable(callCtx, client, sessionID)
		cancel()
		if readErr == nil {
			if target, err := selectPhase7RoundTableActionTarget(view, toolID); err == nil {
				return target, view, nil
			}
		}
		if maximum == 0 || !time.Now().Before(deadline) {
			return phase7LiveGovernanceTargets{}, roundtable.View{},
				errors.New("RoundTable action target readiness timeout")
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return phase7LiveGovernanceTargets{}, roundtable.View{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func pausePhase7LiveRoundTableFixture(
	ctx context.Context,
	client *localipc.Client,
	fixture *phase7LiveRoundTableFixture,
) (roundtable.View, error) {
	if fixture == nil || fixture.closed {
		return roundtable.View{}, errors.New("invalid isolated RoundTable fixture")
	}
	view, err := readPhase7LiveRoundTable(ctx, client, fixture.sessionID)
	if err != nil {
		return roundtable.View{}, err
	}
	if view.Session.Concluded {
		fixture.closed = true
		return view, nil
	}
	if len(view.Rounds) == 0 || view.Rounds[len(view.Rounds)-1].ID != fixture.roundID {
		return roundtable.View{}, errors.New("isolated RoundTable latest round drifted")
	}
	if !view.Rounds[len(view.Rounds)-1].PauseRequested {
		if err := client.Call(ctx, "roundtable_pause_round", productRoundtablePauseRoundParams{
			SchemaVersion:  roundtable.SchemaVersion,
			SessionID:      fixture.sessionID,
			RoundID:        fixture.roundID,
			InterventionID: "p7-pause-" + fixture.sessionID,
			ModeratorSeat:  fixture.moderatorSeat,
			CorrelationID:  matrixOperationID(),
		}, &view); err != nil {
			return roundtable.View{}, err
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		view, err = readPhase7LiveRoundTable(ctx, client, fixture.sessionID)
		if err != nil {
			return roundtable.View{}, err
		}
		running := false
		for _, attempt := range view.Attempts {
			if attempt.RoundID == fixture.roundID &&
				attempt.Status == roundtable.SeatAttemptRunning {
				running = true
				break
			}
		}
		if !running {
			return view, nil
		}
		if !time.Now().Before(deadline) {
			return roundtable.View{}, errors.New("isolated RoundTable pause timeout")
		}
		select {
		case <-ctx.Done():
			return roundtable.View{}, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func latestPhase7RoundTableSeatAttempt(
	view roundtable.View,
	roundID string,
	seatID string,
) (roundtable.SeatAttempt, bool) {
	var latest roundtable.SeatAttempt
	found := false
	for _, attempt := range view.Attempts {
		if attempt.RoundID != roundID || attempt.SeatID != seatID ||
			(found && attempt.AttemptNumber <= latest.AttemptNumber) {
			continue
		}
		latest = attempt
		found = true
	}
	return latest, found
}

func waitForPhase7LiveSeatTerminal(
	ctx context.Context,
	client *localipc.Client,
	sessionID string,
	roundID string,
	seatID string,
	minimumAttempt int,
) (roundtable.View, roundtable.SeatAttempt, error) {
	deadline := time.Now().Add(4 * time.Minute)
	for {
		view, err := readPhase7LiveRoundTable(ctx, client, sessionID)
		if err != nil {
			return roundtable.View{}, roundtable.SeatAttempt{}, err
		}
		attempt, found := latestPhase7RoundTableSeatAttempt(view, roundID, seatID)
		if found && attempt.AttemptNumber >= minimumAttempt &&
			attempt.Status != roundtable.SeatAttemptRunning {
			return view, attempt, nil
		}
		if !time.Now().Before(deadline) {
			return roundtable.View{}, roundtable.SeatAttempt{},
				errors.New("isolated RoundTable cleanup Attempt timeout")
		}
		select {
		case <-ctx.Done():
			return roundtable.View{}, roundtable.SeatAttempt{}, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func closePhase7LiveRoundTableFixture(
	ctx context.Context,
	client *localipc.Client,
	fixture *phase7LiveRoundTableFixture,
) error {
	if fixture == nil || fixture.closed {
		return nil
	}
	view, err := ensurePhase7LiveRoundTableFixtureStarted(ctx, client, fixture)
	if err != nil {
		return err
	}
	if view.Session.Concluded {
		fixture.closed = true
		return nil
	}
	view, err = pausePhase7LiveRoundTableFixture(ctx, client, fixture)
	if err != nil {
		return err
	}
	if view.Session.Concluded {
		fixture.closed = true
		return nil
	}
	mainSeatID := ""
	for seatID, seat := range view.Seats {
		if seatID != fixture.moderatorSeat && seat.Binding != nil &&
			seat.Binding.TeamRoleKind == "main" {
			mainSeatID = seatID
			break
		}
	}
	if mainSeatID == "" {
		return errors.New("isolated RoundTable main seat is unavailable")
	}
	mainAttempt, found := latestPhase7RoundTableSeatAttempt(
		view, fixture.roundID, mainSeatID,
	)
	if !found {
		return errors.New("isolated RoundTable main Attempt is unavailable")
	}
	if mainAttempt.Status != roundtable.SeatAttemptSucceeded {
		seat := view.Seats[mainSeatID]
		if seat.Binding == nil ||
			(mainAttempt.Status != roundtable.SeatAttemptCancelled &&
				(mainAttempt.Status != roundtable.SeatAttemptFailed || !mainAttempt.Retryable)) {
			return errors.New("isolated RoundTable main Attempt cannot be completed")
		}
		if err := client.Call(ctx, "roundtable_retry_seat", productRoundtableRetrySeatParams{
			SchemaVersion:              roundtable.SchemaVersion,
			SessionID:                  fixture.sessionID,
			RoundID:                    fixture.roundID,
			InterventionID:             "p7-cleanup-retry-" + fixture.sessionID,
			ModeratorSeat:              fixture.moderatorSeat,
			SeatID:                     mainSeatID,
			AttemptID:                  mainAttempt.AttemptID,
			ExpectedMembershipRevision: seat.Binding.MembershipRevision,
			ExpectedSeatBindingDigest:  seat.Binding.BindingDigest,
			Guidance:                   "Complete this isolated Phase 7 acceptance cleanup safely.",
			CorrelationID:              matrixOperationID(),
		}, &view); err != nil {
			return err
		}
		view, mainAttempt, err = waitForPhase7LiveSeatTerminal(
			ctx, client, fixture.sessionID, fixture.roundID, mainSeatID,
			mainAttempt.AttemptNumber+1,
		)
		if err != nil {
			return err
		}
		if mainAttempt.Status != roundtable.SeatAttemptSucceeded {
			return errors.New("isolated RoundTable cleanup main Attempt did not succeed")
		}
	}
	seatIDs := make([]string, 0, len(view.Seats))
	for seatID := range view.Seats {
		seatIDs = append(seatIDs, seatID)
	}
	sort.Strings(seatIDs)
	for _, seatID := range seatIDs {
		if seatID == fixture.moderatorSeat || seatID == mainSeatID {
			continue
		}
		seat := view.Seats[seatID]
		if !seat.Available || seat.Binding == nil {
			continue
		}
		attempt, found := latestPhase7RoundTableSeatAttempt(view, fixture.roundID, seatID)
		if found && attempt.Status == roundtable.SeatAttemptSucceeded {
			continue
		}
		alreadySkipped := false
		for _, intervention := range view.Interventions {
			if intervention.Kind == roundtable.InterventionSkipSeat &&
				intervention.RoundID == fixture.roundID && intervention.SeatID == seatID {
				alreadySkipped = true
				break
			}
		}
		if alreadySkipped {
			continue
		}
		if err := client.Call(ctx, "roundtable_skip_seat", productRoundtableSkipSeatParams{
			SchemaVersion:              roundtable.SchemaVersion,
			SessionID:                  fixture.sessionID,
			RoundID:                    fixture.roundID,
			InterventionID:             "p7-cleanup-skip-" + seatID,
			ModeratorSeat:              fixture.moderatorSeat,
			SeatID:                     seatID,
			ExpectedMembershipRevision: seat.Binding.MembershipRevision,
			ExpectedSeatBindingDigest:  seat.Binding.BindingDigest,
			CorrelationID:              matrixOperationID(),
		}, &view); err != nil {
			return err
		}
	}
	if err := client.Call(ctx, "roundtable_conclude", productRoundtableConcludeParams{
		SchemaVersion: roundtable.SchemaVersion,
		SessionID:     fixture.sessionID,
		ModeratorSeat: fixture.moderatorSeat,
		CorrelationID: matrixOperationID(),
	}, &view); err != nil {
		return err
	}
	if !view.Session.Concluded {
		return errors.New("isolated RoundTable cleanup did not conclude")
	}
	fixture.closed = true
	return nil
}

const phase7RoundtableRegistryMaxBytes = 256 << 10

type phase7RoundtableRegistryDocument struct {
	SchemaVersion int                              `json:"schemaVersion"`
	Sessions      []phase7RoundtableRegistryRecord `json:"sessions"`
}

type phase7RoundtableRegistryRecord struct {
	MissionID string `json:"missionID"`
	SessionID string `json:"sessionID"`
}

func loadPhase7RoundtableRegistry(path string) (map[string]string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := validatePhase7RoundtableRegistryInfo(before); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	openedBefore, err := file.Stat()
	if err != nil {
		return nil, err
	}
	afterPath, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, openedBefore) ||
		!os.SameFile(openedBefore, afterPath) {
		return nil, errors.New("RoundTable registry identity changed")
	}
	if err := validatePhase7RoundtableRegistryInfo(openedBefore); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, phase7RoundtableRegistryMaxBytes+1))
	if err != nil || len(data) == 0 || len(data) > phase7RoundtableRegistryMaxBytes {
		return nil, errors.New("RoundTable registry size is invalid")
	}
	openedAfter, err := file.Stat()
	if err != nil || openedBefore.Size() != int64(len(data)) ||
		openedBefore.Size() != openedAfter.Size() ||
		!openedBefore.ModTime().Equal(openedAfter.ModTime()) {
		return nil, errors.New("RoundTable registry changed while reading")
	}
	if err := rejectPhase7DuplicateJSONKeys(data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document phase7RoundtableRegistryDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("RoundTable registry JSON is invalid")
	}
	if err := requirePhase7JSONEOF(decoder); err != nil ||
		document.SchemaVersion != 1 || len(document.Sessions) > 512 {
		return nil, errors.New("RoundTable registry contract is invalid")
	}
	registry := make(map[string]string, len(document.Sessions))
	seenSessions := make(map[string]struct{}, len(document.Sessions))
	for _, record := range document.Sessions {
		if !validPhase7RoundtableRegistryID(record.MissionID) ||
			!validPhase7RoundtableRegistryID(record.SessionID) {
			return nil, errors.New("RoundTable registry identity is invalid")
		}
		if _, found := registry[record.MissionID]; found {
			return nil, errors.New("RoundTable registry Mission identity is ambiguous")
		}
		if _, found := seenSessions[record.SessionID]; found {
			return nil, errors.New("RoundTable registry Session identity is ambiguous")
		}
		registry[record.MissionID] = record.SessionID
		seenSessions[record.SessionID] = struct{}{}
	}
	return registry, nil
}

func validatePhase7RoundtableRegistryInfo(info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 ||
		info.Size() <= 0 || info.Size() > phase7RoundtableRegistryMaxBytes ||
		stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 {
		return errors.New("RoundTable registry is not a private owner-only file")
	}
	return nil
}

func validPhase7RoundtableRegistryID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func rejectPhase7DuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return errors.New("RoundTable registry object key is invalid")
				}
				if _, found := seen[key]; found {
					return errors.New("RoundTable registry contains duplicate JSON keys")
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		default:
			return errors.New("RoundTable registry JSON delimiter is invalid")
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(); err != nil {
		return err
	}
	return requirePhase7JSONEOF(decoder)
}

func requirePhase7JSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("RoundTable registry contains trailing JSON")
	}
	return nil
}

func requirePhase7NoSentinelProductState(
	t *testing.T,
	snapshot api.LocalProductSnapshot,
	runID string,
) {
	t.Helper()
	contains := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(value, runID) {
				return true
			}
		}
		return false
	}
	for _, mission := range snapshot.Missions {
		if contains(mission.MissionID, mission.TeamInstanceID, mission.Title) {
			t.Fatal("control-tool gate created Mission state")
		}
	}
	for _, team := range snapshot.Teams {
		if contains(team.TeamInstanceID, team.TeamDefinitionID, team.DisplayName) {
			t.Fatal("control-tool gate created Agent Team state")
		}
	}
	for _, run := range snapshot.Runs {
		if contains(run.RunID, run.WorkItemID) {
			t.Fatal("control-tool gate created Run state")
		}
	}
	for _, evidence := range snapshot.Evidence {
		if contains(evidence.EvidenceID, evidence.WorkItemID) {
			t.Fatal("control-tool gate created Evidence state")
		}
	}
	for _, sideTask := range snapshot.SideTasks {
		if contains(
			sideTask.SideTaskID, sideTask.ParentMissionID,
			sideTask.ParentTeamInstanceID, sideTask.Title,
		) {
			t.Fatal("control-tool gate created side-task state")
		}
	}
}

func requirePhase7CompletedTools(
	t *testing.T,
	thread api.LocalProductChatThread,
	expected ...controltool.ToolID,
) {
	t.Helper()
	if err := validatePhase7CompletedTools(thread, expected...); err != nil {
		t.Fatal(err)
	}
}

func validatePhase7CompletedTools(
	thread api.LocalProductChatThread,
	expected ...controltool.ToolID,
) error {
	if len(thread.Attempts) == 0 {
		return errors.New("conversation has no Attempt audit")
	}
	attempt := thread.Attempts[len(thread.Attempts)-1]
	if attempt.Status != "succeeded" {
		return fmt.Errorf("latest Attempt status = %s, want succeeded", attempt.Status)
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil || !registry.ValidCompletedCalls(attempt.CompletedControlTools, 8) {
		return fmt.Errorf("invalid completed Tool audit: %#v, %v", attempt.CompletedControlTools, err)
	}
	want := make(map[controltool.ToolID]int, len(expected))
	got := make(map[controltool.ToolID]int, len(attempt.CompletedControlTools))
	for _, toolID := range expected {
		want[toolID]++
	}
	for _, call := range attempt.CompletedControlTools {
		got[call.ToolID]++
		if want[call.ToolID] == 0 {
			return fmt.Errorf("unexpected Tool audit: %s", call.ToolID)
		}
	}
	for toolID, count := range want {
		if got[toolID] != count {
			return fmt.Errorf("completed Tool IDs = %v, want %v", got, want)
		}
	}
	return nil
}

func equalPhase7ToolCounts(
	left map[controltool.ToolID]int,
	right map[controltool.ToolID]int,
) bool {
	if len(left) != len(right) {
		return false
	}
	for toolID, count := range left {
		if right[toolID] != count {
			return false
		}
	}
	return true
}

func requirePhase7LiveProposalToolSet(
	t *testing.T,
	thread api.LocalProductChatThread,
	expected []controltool.ToolID,
) {
	t.Helper()
	got := make(map[controltool.ToolID]int, len(expected))
	for _, proposal := range thread.ControlProposals {
		if !proposal.Valid() || proposal.Status != controltool.ProposalPending {
			t.Fatalf("invalid alignment Proposal: %#v", proposal)
		}
		got[proposal.ToolID]++
	}
	for _, proposal := range thread.ActionProposals {
		if !proposal.Valid() || proposal.Status != controltool.ProposalPending {
			t.Fatalf("invalid action Proposal: %#v", proposal)
		}
		got[proposal.ToolID]++
	}
	want := make(map[controltool.ToolID]int, len(expected))
	for _, toolID := range expected {
		want[toolID]++
	}
	if !equalPhase7ToolCounts(got, want) {
		t.Fatalf("Proposal Tool IDs = %v, want %v", got, want)
	}
}

func cancelPhase7LiveSweepProposals(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	thread api.LocalProductChatThread,
) api.LocalProductChatThread {
	t.Helper()
	type proposalReference struct {
		id     string
		digest string
	}
	references := make([]proposalReference, 0, len(thread.ControlProposals)+len(thread.ActionProposals))
	for _, proposal := range thread.ControlProposals {
		references = append(references, proposalReference{proposal.ProposalID, proposal.ProposalDigest})
	}
	for _, proposal := range thread.ActionProposals {
		references = append(references, proposalReference{proposal.ProposalID, proposal.ProposalDigest})
	}
	for _, proposal := range references {
		var updated api.LocalProductChatThread
		if err := client.Call(ctx, "chat_control_decision", api.LocalProductChatControlDecisionRequest{
			ThreadID: thread.ThreadID, ProposalID: proposal.id,
			ProposalDigest: proposal.digest, Decision: api.ControlDecisionCancel,
		}, &updated); err != nil {
			t.Fatalf("cancel capability Proposal %s: %v", proposal.id, err)
		}
		thread = updated
	}
	return thread
}

func requirePhase7SweepProposalsCancelled(
	t *testing.T,
	thread api.LocalProductChatThread,
	expected []controltool.ToolID,
) {
	t.Helper()
	if err := validatePhase7SweepProposalsCancelled(thread, expected); err != nil {
		t.Fatal(err)
	}
}

func validatePhase7SweepProposalsCancelled(
	thread api.LocalProductChatThread,
	expected []controltool.ToolID,
) error {
	proposalCount := len(thread.ControlProposals) + len(thread.ActionProposals)
	if proposalCount != len(expected) {
		return fmt.Errorf(
			"restored cancelled Proposal count = %d, want %d", proposalCount, len(expected),
		)
	}
	wanted := make(map[controltool.ToolID]int, len(expected))
	for _, toolID := range expected {
		wanted[toolID]++
	}
	receipts := make(map[string]controltool.ProposalDecisionReceipt)
	for _, receipt := range thread.ProposalDecisionReceipts {
		if _, found := receipts[receipt.ProposalID]; found {
			return errors.New("restored cancelled Proposal has duplicate decision receipts")
		}
		receipts[receipt.ProposalID] = receipt
	}
	if len(receipts) != len(expected) {
		return fmt.Errorf(
			"restored cancelled Proposal receipt count = %d, want %d",
			len(receipts), len(expected),
		)
	}
	seenProposalIDs := make(map[string]struct{}, len(expected))
	consume := func(proposalID string, toolID controltool.ToolID) error {
		if _, found := seenProposalIDs[proposalID]; found {
			return errors.New("restored cancelled Proposal identity is duplicated")
		}
		seenProposalIDs[proposalID] = struct{}{}
		if wanted[toolID] == 0 {
			return fmt.Errorf("restored unexpected cancelled Proposal Tool %s", toolID)
		}
		wanted[toolID]--
		return nil
	}
	for _, proposal := range thread.ControlProposals {
		if err := consume(proposal.ProposalID, proposal.ToolID); err != nil {
			return err
		}
		receipt, found := receipts[proposal.ProposalID]
		if proposal.Status != controltool.ProposalCancelled || !found ||
			receipt.Decision != controltool.ProposalDecisionCancel ||
			!receipt.MatchesSessionAlignmentProposal(proposal) {
			return errors.New("alignment Proposal was not restored with its exact cancellation receipt")
		}
	}
	for _, proposal := range thread.ActionProposals {
		if err := consume(proposal.ProposalID, proposal.ToolID); err != nil {
			return err
		}
		receipt, found := receipts[proposal.ProposalID]
		if proposal.Status != controltool.ProposalCancelled || !found ||
			receipt.Decision != controltool.ProposalDecisionCancel ||
			!receipt.MatchesConversationActionProposal(proposal) {
			return errors.New("action Proposal was not restored with its exact cancellation receipt")
		}
	}
	for toolID, remaining := range wanted {
		if remaining != 0 {
			return fmt.Errorf("restored cancelled Proposal Tool %s count drifted", toolID)
		}
	}
	return nil
}

func phase7LiveThreadFailure(thread api.LocalProductChatThread) string {
	if failure := thread.AvailabilityFailure; failure != nil {
		return fmt.Sprintf(
			"code=%s stage=%s incident=%s retryable=%t",
			failure.Code, failure.Stage, failure.IncidentID, failure.Retryable,
		)
	}
	for index := len(thread.Attempts) - 1; index >= 0; index-- {
		attempt := thread.Attempts[index]
		if attempt.FailureCode == "" && attempt.Status != "failed" {
			continue
		}
		return fmt.Sprintf(
			"code=%s stage=%s provider_code=%s incident=%s retryable=%t",
			attempt.FailureCode, attempt.FailureStage, attempt.ProviderCode,
			attempt.IncidentID,
			attempt.Retryable,
		)
	}
	return ""
}

func phase7LiveThreadFailureRetryable(thread api.LocalProductChatThread) bool {
	if failure := thread.AvailabilityFailure; failure != nil {
		return failure.Retryable
	}
	for index := len(thread.Attempts) - 1; index >= 0; index-- {
		attempt := thread.Attempts[index]
		if attempt.FailureCode != "" || attempt.Status == "failed" {
			return attempt.Retryable
		}
	}
	return false
}

func phase7LiveBinding(
	profile app.ConversationProviderProfile,
) *api.LocalProductConversationExecutionBinding {
	return &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: profile.HarnessAdapter,
		ProviderID: profile.ProviderID, ProviderAccountID: profile.ProviderAccountID,
		CredentialRevision: profile.CredentialRevision, ModelID: profile.ModelID,
		ProviderAccountPolicyVersion:  profile.PolicyVersion,
		ProviderAccountPolicyRevision: profile.PolicyRevision,
		ProviderAccountPolicyDigest:   profile.PolicyDigest,
		TrustDomain:                   profile.TrustDomain, RetentionMode: profile.RetentionMode,
		DataRegion: profile.DataRegion,
	}
}

func decidePhase7LiveProposal(
	t *testing.T,
	ctx context.Context,
	client *localipc.Client,
	record phase7LiveProposalRecord,
	decision api.LocalProductChatControlDecision,
) api.LocalProductChatThread {
	t.Helper()
	var thread api.LocalProductChatThread
	if err := client.Call(ctx, "chat_control_decision", api.LocalProductChatControlDecisionRequest{
		ThreadID: record.threadID, ProposalID: record.proposal.ProposalID,
		ProposalDigest: record.proposal.ProposalDigest, Decision: decision,
	}, &thread); err != nil {
		t.Fatalf("%s %s decision: %v", record.harness, record.decisionCase, err)
	}
	return thread
}

func requirePhase7ProposalStatus(
	t *testing.T,
	thread api.LocalProductChatThread,
	proposalID string,
	want controltool.ProposalStatus,
) {
	t.Helper()
	for _, proposal := range thread.ActionProposals {
		if proposal.ProposalID == proposalID {
			if proposal.Status != want {
				t.Fatalf("proposal %s status = %s, want %s", proposalID, proposal.Status, want)
			}
			return
		}
	}
	t.Fatalf("proposal %s missing", proposalID)
}

func requirePhase7DecisionReceipt(
	t *testing.T,
	thread api.LocalProductChatThread,
	proposal controltool.ConversationActionProposal,
	want controltool.ProposalDecision,
) controltool.ProposalDecisionReceipt {
	t.Helper()
	matches := make([]controltool.ProposalDecisionReceipt, 0, 1)
	for _, receipt := range thread.ProposalDecisionReceipts {
		if receipt.ProposalID == proposal.ProposalID {
			matches = append(matches, receipt)
		}
	}
	if len(matches) != 1 || !matches[0].Valid() ||
		!matches[0].MatchesConversationActionProposal(proposal) ||
		matches[0].Decision != want || matches[0].DecisionIncidentID == "" {
		t.Fatalf(
			"proposal %s decision receipt = %#v, want %s",
			proposal.ProposalID, matches, want,
		)
	}
	return matches[0]
}

func newPhase7ExpectedInstalledIdentity(
	build string,
	appDigest string,
	daemonDigest string,
) (phase7ExpectedInstalledIdentity, error) {
	build = strings.TrimSpace(build)
	appDigest = strings.TrimSpace(appDigest)
	daemonDigest = strings.TrimSpace(daemonDigest)
	if !validPhase7Build(build) || !validPhase7SHA256(appDigest) ||
		!validPhase7SHA256(daemonDigest) {
		return phase7ExpectedInstalledIdentity{}, errors.New(
			"installed identity requires exact build, App SHA-256 and daemon SHA-256",
		)
	}
	return phase7ExpectedInstalledIdentity{
		Build: build, AppExecutableSHA256: appDigest, DaemonSHA256: daemonDigest,
	}, nil
}

func phase7ExpectedInstalledIdentityFromEnvironment() (
	phase7ExpectedInstalledIdentity,
	error,
) {
	return newPhase7ExpectedInstalledIdentity(
		os.Getenv(phase7LiveExpectedBuildGate),
		os.Getenv(phase7LiveExpectedAppDigestGate),
		os.Getenv(phase7LiveExpectedDaemonDigestGate),
	)
}

func phase7InstalledPrivacyRoots(socketPath string) ([]string, error) {
	if !filepath.IsAbs(socketPath) || filepath.Clean(socketPath) != socketPath ||
		filepath.Base(socketPath) != "loomd.sock" ||
		filepath.Base(filepath.Dir(socketPath)) != "run" {
		return nil, errors.New("invalid installed privacy Socket path")
	}
	productRoot := filepath.Dir(filepath.Dir(socketPath))
	if productRoot == "." || productRoot == string(filepath.Separator) {
		return nil, errors.New("invalid installed privacy root")
	}
	return []string{
		filepath.Join(productRoot, "state"),
		filepath.Join(productRoot, "diagnostics"),
	}, nil
}

func requirePhase7NoPlaintextPrivateContent(
	t *testing.T,
	socketPath string,
	marker string,
) {
	t.Helper()
	roots, err := phase7InstalledPrivacyRoots(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		err = scanPhase7PlaintextPrivacyRoots(
			roots, marker, phase7PrivacyScanMaximumFiles,
			phase7PrivacyScanMaximumBytes,
		)
		if err == nil {
			return
		}
		if !errors.Is(err, errPhase7PrivacySnapshotChanged) {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal(err)
}

func scanPhase7PlaintextPrivacyRoots(
	roots []string,
	marker string,
	maximumFiles int,
	maximumBytes int64,
) error {
	markerDigest := strings.TrimPrefix(marker, phase7PrivateContentPrefix)
	if len(roots) == 0 || len(roots) > 4 || !validPhase7SHA256(markerDigest) ||
		maximumFiles <= 0 || maximumFiles > 65_536 || maximumBytes <= 0 ||
		maximumBytes > 8<<30 {
		return errors.New("invalid installed privacy scan")
	}
	markerBytes := []byte(marker)
	files := 0
	var totalBytes int64
	for _, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return errors.New("invalid installed privacy root")
		}
		rootInfo, err := os.Lstat(root)
		if err != nil || !phase7PrivacyOwnedDirectory(rootInfo) {
			return errors.New("installed privacy root is not private and exact")
		}
		err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, os.ErrNotExist) {
					return errPhase7PrivacySnapshotChanged
				}
				return walkErr
			}
			if info == nil || info.Mode()&os.ModeSymlink != 0 ||
				!phase7PrivacyOwned(info) {
				return errors.New("installed privacy entry is not exact")
			}
			if info.IsDir() {
				if info.Mode().Perm()&0o077 != 0 {
					return errors.New("installed privacy directory is not private")
				}
				return nil
			}
			if !info.Mode().IsRegular() || info.Size() < 0 {
				return errors.New("installed privacy entry is not regular")
			}
			files++
			if files > maximumFiles || info.Size() > maximumBytes-totalBytes {
				return errors.New("installed privacy scan exceeded its bound")
			}
			totalBytes += info.Size()
			contains, err := phase7InstalledFileContains(path, info, markerBytes)
			if err != nil {
				return err
			}
			if contains {
				return errors.New(
					"private conversation content appeared in plaintext installed storage",
				)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func phase7PrivacyOwned(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func phase7PrivacyOwnedDirectory(info os.FileInfo) bool {
	return info != nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 &&
		phase7PrivacyOwned(info) && info.Mode().Perm()&0o077 == 0
}

func phase7InstalledFileContains(
	path string,
	before os.FileInfo,
	marker []byte,
) (bool, error) {
	if len(marker) == 0 || before == nil || !before.Mode().IsRegular() {
		return false, errors.New("invalid installed privacy file")
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, errPhase7PrivacySnapshotChanged
		}
		return false, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || opened.Size() != before.Size() {
		return false, errPhase7PrivacySnapshotChanged
	}
	buffer := make([]byte, 64*1024+len(marker)-1)
	overlap := 0
	for {
		read, readErr := file.Read(buffer[overlap:])
		available := overlap + read
		if bytes.Contains(buffer[:available], marker) {
			return true, nil
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return false, readErr
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if read == 0 {
			return false, errPhase7PrivacySnapshotChanged
		}
		overlap = min(len(marker)-1, available)
		copy(buffer[:overlap], buffer[available-overlap:available])
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || after.Size() != before.Size() ||
		after.Mode()&os.ModeSymlink != 0 {
		return false, errPhase7PrivacySnapshotChanged
	}
	return false, nil
}

func phase7InstalledPaths() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("home directory unavailable: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", "", errors.New("home directory must be absolute")
	}
	appPath := strings.TrimSpace(os.Getenv("LOOM_INSTALLED_APP_PATH"))
	if appPath == "" {
		appPath = filepath.Join(home, "Applications", "Loom.app")
	}
	socketPath := strings.TrimSpace(os.Getenv("LOOM_SOCKET_PATH"))
	if socketPath == "" {
		socketPath = filepath.Join(
			home, "Library", "Application Support", "Loom", "run", "loomd.sock",
		)
	}
	if !filepath.IsAbs(appPath) || !filepath.IsAbs(socketPath) {
		return "", "", errors.New("installed App and Socket paths must be absolute")
	}
	return appPath, socketPath, nil
}

func validPhase7Build(value string) bool {
	if value == "" || len(value) > 16 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validPhase7SHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func requirePhase7InstalledIdentity(
	appPath string,
	expected phase7ExpectedInstalledIdentity,
) error {
	if !filepath.IsAbs(appPath) || filepath.Clean(appPath) != appPath ||
		filepath.Base(appPath) != "Loom.app" ||
		!validPhase7Build(expected.Build) ||
		!validPhase7SHA256(expected.AppExecutableSHA256) ||
		!validPhase7SHA256(expected.DaemonSHA256) {
		return errors.New("invalid installed App identity")
	}
	appInfo, err := os.Lstat(appPath)
	if err != nil || appInfo.Mode()&os.ModeSymlink != 0 || !appInfo.IsDir() {
		return errors.New("installed App bundle is not an exact directory")
	}
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	if _, err := phase7InstalledFileSHA256(plist); err != nil {
		return fmt.Errorf("inspect installed Info.plist: %w", err)
	}
	output, err := exec.Command(
		"/usr/libexec/PlistBuddy", "-c", "Print :CFBundleVersion", plist,
	).Output()
	if err != nil {
		return fmt.Errorf("read installed build: %w", err)
	}
	if actual := strings.TrimSpace(string(output)); actual != expected.Build {
		return fmt.Errorf("installed build = %s, expected %s", actual, expected.Build)
	}
	if output, err := exec.Command(
		"codesign", "--verify", "--deep", "--strict", appPath,
	).CombinedOutput(); err != nil {
		return fmt.Errorf(
			"verify installed App signature: %w (%s)",
			err, strings.TrimSpace(string(output)),
		)
	}
	appExecutable := filepath.Join(appPath, "Contents", "MacOS", "LoomLocalApp")
	daemon := filepath.Join(appPath, "Contents", "Library", "Helpers", "loomd")
	for _, executable := range []string{appExecutable, daemon} {
		info, err := os.Lstat(executable)
		if err != nil || info.Mode()&0o111 == 0 {
			return fmt.Errorf("installed executable is not executable: %s", executable)
		}
	}
	if err := requirePhase7InstalledFileDigest(
		appExecutable, expected.AppExecutableSHA256,
	); err != nil {
		return fmt.Errorf("installed App executable identity: %w", err)
	}
	if err := requirePhase7InstalledFileDigest(
		daemon, expected.DaemonSHA256,
	); err != nil {
		return fmt.Errorf("installed daemon identity: %w", err)
	}
	return nil
}

func requirePhase7InstalledFileDigest(path string, expected string) error {
	if !validPhase7SHA256(expected) {
		return errors.New("invalid expected SHA-256")
	}
	actual, err := phase7InstalledFileSHA256(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("SHA-256 = %s, expected %s", actual, expected)
	}
	return nil
}

func phase7InstalledFileSHA256(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return "", errors.New("installed bundle file is not exact and regular")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", errors.New("installed bundle file identity changed before hashing")
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	after, err := os.Lstat(path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 ||
		!after.Mode().IsRegular() || !os.SameFile(before, after) {
		return "", errors.New("installed bundle file identity changed while hashing")
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func phase7LiveClient(socketPath string) (*localipc.Client, error) {
	return localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 90 * time.Second,
		ExtendedTimeout: 90 * time.Second,
	})
}

func newPhase7LiveClient(t *testing.T, socketPath string) *localipc.Client {
	t.Helper()
	var client *localipc.Client
	err := waitForPhase7Readiness(
		context.Background(), 60*time.Second,
		func(probeCtx context.Context) error {
			candidate, err := phase7LiveClient(socketPath)
			if err != nil {
				return err
			}
			var snapshot json.RawMessage
			if err := candidate.Call(probeCtx, "setup_snapshot", struct{}{}, &snapshot); err != nil {
				return err
			}
			client = candidate
			return nil
		},
	)
	if err != nil || client == nil {
		t.Fatalf("local IPC readiness: %v", err)
	}
	return client
}

func waitForPhase7Readiness(
	ctx context.Context,
	maximum time.Duration,
	probe func(context.Context) error,
) error {
	if ctx == nil || maximum < 0 || probe == nil {
		return errors.New("invalid installed readiness probe")
	}
	deadline := time.Now().Add(maximum)
	for {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := probe(probeCtx)
		cancel()
		if err == nil {
			return nil
		}
		if maximum == 0 || !time.Now().Before(deadline) {
			return errors.New("installed readiness timeout")
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func TestWaitForPhase7ReadinessRetriesBoundedStartup(t *testing.T) {
	reads := 0
	if err := waitForPhase7Readiness(
		context.Background(), time.Second,
		func(context.Context) error {
			reads++
			if reads < 3 {
				return errors.New("not ready")
			}
			return nil
		},
	); err != nil || reads != 3 {
		t.Fatalf("installed readiness reads=%d error=%v", reads, err)
	}
	reads = 0
	if err := waitForPhase7Readiness(
		context.Background(), 0,
		func(context.Context) error {
			reads++
			return errors.New("still unavailable")
		},
	); err == nil || reads != 1 {
		t.Fatalf("installed readiness timeout reads=%d error=%v", reads, err)
	}
}

func restartPhase7InstalledApp(
	t *testing.T,
	ctx context.Context,
	appPath string,
	socketPath string,
	beforePID string,
	expectedIdentity phase7ExpectedInstalledIdentity,
) *localipc.Client {
	t.Helper()
	if output, err := exec.Command(
		"osascript", "-e", `tell application id "com.earendilworks.loom.local" to quit`,
	).CombinedOutput(); err != nil {
		t.Fatalf("quit installed App: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	if err := waitForPhase7DaemonExit(ctx, appPath, socketPath, beforePID); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("open", appPath).CombinedOutput(); err != nil {
		t.Fatalf("open installed App: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		client, clientErr := phase7LiveClient(socketPath)
		if clientErr == nil {
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			var snapshot json.RawMessage
			err := client.Call(probeCtx, "setup_snapshot", struct{}{}, &snapshot)
			cancel()
			if err == nil {
				afterPID, pidErr := phase7InstalledDaemonPID(appPath, socketPath)
				if pidErr == nil && afterPID != beforePID {
					if err := requirePhase7InstalledIdentity(
						appPath, expectedIdentity,
					); err != nil {
						t.Fatal(err)
					}
					return client
				}
				if pidErr != nil && !errors.Is(pidErr, errPhase7InstalledDaemonNotFound) {
					t.Fatal(pidErr)
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
	t.Fatal("installed App did not restore with a new managed daemon")
	return nil
}

func waitForPhase7DaemonExit(
	ctx context.Context,
	appPath string,
	socketPath string,
	beforePID string,
) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		pid, err := phase7InstalledDaemonPID(appPath, socketPath)
		if errors.Is(err, errPhase7InstalledDaemonNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if pid != beforePID {
			return errors.New("managed daemon identity changed before App relaunch")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("installed managed daemon did not exit")
}

func phase7InstalledDaemonPID(appPath string, socketPath string) (string, error) {
	helper := filepath.Join(appPath, "Contents", "Library", "Helpers", "loomd")
	output, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return "", err
	}
	return phase7InstalledDaemonPIDFromPS(string(output), helper, socketPath)
}

func phase7InstalledDaemonPIDFromPS(
	output string,
	helper string,
	socketPath string,
) (string, error) {
	if helper == "" || socketPath == "" {
		return "", errPhase7InstalledDaemonNotFound
	}
	match := ""
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		separator := strings.IndexAny(line, " \t")
		if separator <= 0 {
			continue
		}
		pid := line[:separator]
		command := strings.TrimSpace(line[separator:])
		if !validPhase7PID(pid) ||
			command != helper && !strings.HasPrefix(command, helper+" ") ||
			!phase7CommandHasExactSocket(command, socketPath) {
			continue
		}
		if match != "" {
			return "", errors.New("multiple installed managed daemons found")
		}
		match = pid
	}
	if match == "" {
		return "", errPhase7InstalledDaemonNotFound
	}
	return match, nil
}

func validPhase7PID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func phase7CommandHasExactSocket(command string, socketPath string) bool {
	marker := "--socket " + socketPath
	searchFrom := 0
	for searchFrom < len(command) {
		index := strings.Index(command[searchFrom:], marker)
		if index < 0 {
			return false
		}
		index += searchFrom
		beforeIsBoundary := index == 0 || command[index-1] == ' ' ||
			command[index-1] == '\t'
		after := command[index+len(marker):]
		afterIsBoundary := after == "" || after[0] == ' ' || after[0] == '\t'
		if beforeIsBoundary && afterIsBoundary {
			return true
		}
		searchFrom = index + 1
	}
	return false
}

func TestSelectPhase7LiveProfilesRequiresFiveDistinctExecutableHarnesses(t *testing.T) {
	deepSeekOpenCodeProfileID := provider.OpenCodeConversationAccountProfileID(
		"deepseek", "deepseek.primary", 2,
	)
	miniMaxOpenCodeProfileID := provider.OpenCodeConversationAccountProfileID(
		"minimax", "minimax.primary", 23,
	)
	deepSeekLoomProfileID := provider.DeepSeekConversationAccountProfileID(
		"deepseek.primary", 2,
	)
	miniMaxLoomProfileID := provider.MiniMaxConversationAccountProfileID(
		"minimax.primary", 23,
	)
	profiles := []app.ConversationProviderProfile{
		{ProfileID: provider.CodexConversationProfileID, HarnessAdapter: "codex", ProviderID: "openai", Protocol: "openai_responses", ModelID: "codex-default", AuthMode: "native_auth"},
		{ProfileID: deepSeekOpenCodeProfileID, HarnessAdapter: "opencode", ProviderID: "deepseek", ProviderAccountID: "deepseek.primary", Protocol: "opencode_agent", ModelID: "deepseek/deepseek-chat", AuthMode: "brokered", CredentialRevision: 2},
		{ProfileID: miniMaxOpenCodeProfileID, HarnessAdapter: "opencode", ProviderID: "minimax", ProviderAccountID: "minimax.primary", Protocol: "opencode_agent", ModelID: "minimax-cn/MiniMax-M3", AuthMode: "brokered", CredentialRevision: 23},
		{ProfileID: "opencode-unbound", HarnessAdapter: "opencode", ProviderID: "deepseek", Protocol: "opencode_agent", ModelID: "deepseek/deepseek-chat", AuthMode: "brokered"},
		{ProfileID: provider.OpenCodeConversationProfileID, HarnessAdapter: "opencode", ProviderID: "opencode", Protocol: "opencode_agent", ModelID: provider.OpenCodeConversationDefaultModel, AuthMode: "native_auth"},
		{ProfileID: provider.ClaudeCodeConversationProfileID, HarnessAdapter: "claude-code", ProviderID: "anthropic", Protocol: "claude_code_agent", ModelID: provider.AnthropicConversationModelID, AuthMode: "native_auth"},
		{ProfileID: provider.PiConversationProfileID, HarnessAdapter: "pi", ProviderID: "loom-local", Protocol: "pi_rpc", ModelID: provider.PiConversationModelID, AuthMode: "native_auth"},
		{ProfileID: miniMaxLoomProfileID, HarnessAdapter: "loom-native", ProviderID: "minimax", ProviderAccountID: "minimax.primary", Protocol: "openai_compatible", ModelID: provider.MiniMaxConversationModelID, AuthMode: "brokered", CredentialRevision: 23},
		{ProfileID: deepSeekLoomProfileID, HarnessAdapter: "loom-native", ProviderID: "deepseek", ProviderAccountID: "deepseek.primary", Protocol: "openai_compatible", ModelID: provider.DeepSeekConversationModelID, AuthMode: "brokered", CredentialRevision: 2},
	}
	selected, err := selectPhase7LiveProfiles(profiles)
	if err != nil || len(selected) != 5 {
		t.Fatalf("selected = %#v, error = %v", selected, err)
	}
	if selected[1].ProfileID != deepSeekOpenCodeProfileID ||
		selected[4].ProfileID != deepSeekLoomProfileID {
		t.Fatalf("profile precedence = %#v", selected)
	}
	selected, err = selectPhase7LiveProfilesForOpenCodeProvider(profiles, "deepseek")
	if err != nil || selected[1].ProfileID != deepSeekOpenCodeProfileID {
		t.Fatalf("DeepSeek OpenCode selection = %#v, error = %v", selected, err)
	}
	selected, err = selectPhase7LiveProfilesForProviders(
		profiles, "deepseek", "minimax",
	)
	if err != nil || selected[1].ProfileID != deepSeekOpenCodeProfileID ||
		selected[4].ProfileID != miniMaxLoomProfileID {
		t.Fatalf("explicit Provider selection = %#v, error = %v", selected, err)
	}
	if _, err := selectPhase7LiveProfilesForOpenCodeProvider(
		profiles, "credential-shaped-selector",
	); err == nil {
		t.Fatal("invalid OpenCode Provider selector was accepted")
	}
	if _, err := selectPhase7LiveProfilesForProviders(
		profiles, "deepseek", "credential-shaped-selector",
	); err == nil {
		t.Fatal("invalid Loom Native Provider selector was accepted")
	}
	if _, err := selectPhase7LiveProfiles(profiles[:4]); err == nil {
		t.Fatal("missing Pi and Loom Native profiles were accepted")
	}
	withoutClaude := append([]app.ConversationProviderProfile{}, profiles[:5]...)
	withoutClaude = append(withoutClaude, profiles[6:]...)
	available, err := selectPhase7AvailableRuntimeProfiles(
		withoutClaude, "deepseek",
	)
	if err != nil || len(available) != 4 || available[0].HarnessAdapter != "codex" ||
		available[1].ProfileID != deepSeekOpenCodeProfileID ||
		available[2].HarnessAdapter != "pi" || available[3].ProfileID != deepSeekLoomProfileID {
		t.Fatalf("available profile selection = %#v, error = %v", available, err)
	}
	available, err = selectPhase7AvailableRuntimeProfilesForProviders(
		withoutClaude, "deepseek", "minimax",
	)
	if err != nil || len(available) != 4 || available[1].ProfileID != deepSeekOpenCodeProfileID ||
		available[3].ProfileID != miniMaxLoomProfileID {
		t.Fatalf("available explicit Provider selection = %#v, error = %v", available, err)
	}
	if _, err := selectPhase7AvailableRuntimeProfiles(
		profiles, "deepseek",
	); err == nil {
		t.Fatal("available-Runtime matrix bypassed an executable Claude Code profile")
	}
	probe, err := selectPhase7LiveOpenCodeProfile(withoutClaude, "deepseek")
	if err != nil || probe.ProfileID != deepSeekOpenCodeProfileID {
		t.Fatalf("independent OpenCode probe profile = %#v, error = %v", probe, err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*app.ConversationProviderProfile)
	}{
		{name: "profile id", mutate: func(profile *app.ConversationProviderProfile) {
			profile.ProfileID = "conversation-anthropic-stale"
		}},
		{name: "protocol", mutate: func(profile *app.ConversationProviderProfile) {
			profile.Protocol = "anthropic_messages"
		}},
		{name: "auth mode", mutate: func(profile *app.ConversationProviderProfile) {
			profile.AuthMode = "brokered"
		}},
		{name: "account", mutate: func(profile *app.ConversationProviderProfile) {
			profile.ProviderAccountID = "anthropic.primary"
		}},
		{name: "revision", mutate: func(profile *app.ConversationProviderProfile) {
			profile.CredentialRevision = 1
		}},
	} {
		t.Run("reject Claude "+test.name+" drift", func(t *testing.T) {
			drifted := append([]app.ConversationProviderProfile(nil), profiles...)
			test.mutate(&drifted[5])
			if _, err := selectPhase7LiveProfiles(drifted); err == nil {
				t.Fatalf("drifted Claude profile was executable: %#v", drifted[5])
			}
		})
	}
}

func TestPhase7LiveProfileExecutableRequiresExactProjectionContract(t *testing.T) {
	valid := []app.ConversationProviderProfile{
		{
			ProfileID: provider.CodexConversationProfileID, HarnessAdapter: "codex",
			ProviderID: "openai", Protocol: "openai_responses",
			ModelID: "codex-default", AuthMode: "native_auth",
		},
		{
			ProfileID: provider.OpenCodeConversationProfileID, HarnessAdapter: "opencode",
			ProviderID: "opencode", Protocol: "opencode_agent",
			ModelID: provider.OpenCodeConversationDefaultModel, AuthMode: "native_auth",
		},
		{
			ProfileID: provider.OpenCodeConversationAccountProfileID(
				"deepseek", "deepseek.primary", 7,
			),
			HarnessAdapter: "opencode", ProviderID: "deepseek",
			ProviderAccountID: "deepseek.primary", Protocol: "opencode_agent",
			ModelID: "deepseek/deepseek-chat", AuthMode: "brokered",
			CredentialRevision: 7,
		},
		{
			ProfileID:      provider.ClaudeCodeConversationProfileID,
			HarnessAdapter: "claude-code", ProviderID: "anthropic",
			Protocol: "claude_code_agent", ModelID: provider.AnthropicConversationModelID,
			AuthMode: "native_auth",
		},
		{
			ProfileID: provider.PiConversationProfileID, HarnessAdapter: "pi",
			ProviderID: "loom-local", Protocol: "pi_rpc",
			ModelID: provider.PiConversationModelID, AuthMode: "native_auth",
		},
		{
			ProfileID: provider.DeepSeekConversationAccountProfileID(
				"deepseek.primary", 7,
			),
			HarnessAdapter: "loom-native", ProviderID: "deepseek",
			ProviderAccountID: "deepseek.primary", Protocol: "openai_compatible",
			ModelID: provider.DeepSeekConversationModelID, AuthMode: "brokered",
			CredentialRevision: 7,
		},
	}
	for _, profile := range valid {
		if !phase7LiveProfileExecutable(profile) {
			t.Fatalf("exact profile was rejected: %#v", profile)
		}
	}

	drifted := []app.ConversationProviderProfile{
		{ProfileID: provider.CodexConversationProfileID, HarnessAdapter: "codex", ProviderID: "openai", Protocol: "openai_responses", ModelID: "gpt-5.6-sol", AuthMode: "native_auth"},
		{ProfileID: provider.OpenCodeConversationProfileID, HarnessAdapter: "opencode", ProviderID: "opencode", Protocol: "openai_compatible", ModelID: provider.OpenCodeConversationDefaultModel, AuthMode: "native_auth"},
		{ProfileID: provider.OpenCodeConversationAccountProfileID("deepseek", "deepseek.primary", 7), HarnessAdapter: "opencode", ProviderID: "deepseek", ProviderAccountID: "deepseek.primary", Protocol: "opencode_agent", ModelID: "deepseek/deepseek-chat", AuthMode: "brokered", CredentialRevision: 8},
		{ProfileID: provider.ClaudeCodeConversationProfileID, HarnessAdapter: "claude-code", ProviderID: "anthropic", Protocol: "anthropic_messages", ModelID: provider.AnthropicConversationModelID, AuthMode: "native_auth"},
		{ProfileID: provider.PiConversationProfileID, HarnessAdapter: "pi", ProviderID: "loom-local", Protocol: "pi_rpc", ModelID: "qwen-drift", AuthMode: "native_auth"},
		{ProfileID: provider.DeepSeekConversationAccountProfileID("deepseek.primary", 7), HarnessAdapter: "loom-native", ProviderID: "deepseek", ProviderAccountID: "deepseek.primary", Protocol: "openai_compatible", ModelID: provider.DeepSeekConversationModelID, AuthMode: "brokered", CredentialRevision: 8},
		{ProfileID: "conversation-unknown", HarnessAdapter: "unknown", ProviderID: "unknown", Protocol: "unknown", ModelID: "unknown", AuthMode: "native_auth"},
	}
	for _, profile := range drifted {
		if phase7LiveProfileExecutable(profile) {
			t.Fatalf("drifted profile was executable: %#v", profile)
		}
	}
}

func TestPhase7LivePrivateContentMarkerIsContentOnlyAndClosed(t *testing.T) {
	runID := "probe-0123456789abcdef"
	marker, err := phase7LivePrivateContentMarker(runID)
	if err != nil {
		t.Fatal(err)
	}
	if marker == "" || !strings.HasPrefix(marker, phase7PrivateContentPrefix) ||
		strings.Contains(fmt.Sprintf("p7-%s-codex-confirm", runID), marker) {
		t.Fatalf("private content marker = %q", marker)
	}
	objective, err := phase7LiveMissionObjective(runID, "codex", "confirm")
	if err != nil || !strings.Contains(objective, marker) {
		t.Fatalf("private Mission objective = %q, error = %v", objective, err)
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	proposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID: "proposal-private-content", ToolID: controltool.ToolMissionsCreatePreview,
			TargetConversationID: "conversation-private-content",
			TargetContentDigest:  controltool.DigestBytes([]byte("target")),
			Argument:             objective,
			Route: &controltool.FrozenRouteReference{
				HarnessAdapter: "opencode", ProviderID: "opencode",
				ModelID:                "opencode/big-pickle",
				ExecutionBindingDigest: controltool.DigestBytes([]byte("binding")),
				ContextCapsuleDigest:   controltool.DigestBytes([]byte("capsule")),
			},
			Workspace: &controltool.FrozenWorkspaceReference{
				WorkspaceID:     "workspace-private-content",
				WorkspaceDigest: controltool.DigestBytes([]byte("workspace")),
			},
			RegistryDigest: registry.Digest(), IncidentID: "incident-private-content",
			SegmentID: "segment-private-content", AttemptID: "attempt-private-content",
			CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
		},
	)
	if err != nil || !proposal.Valid() || proposal.Argument != objective {
		t.Fatalf("private content marker was not admissible Mission content: valid=%t error=%v", proposal.Valid(), err)
	}
	for _, invalid := range []string{"", "contains/slash", strings.Repeat("a", 129)} {
		if _, err := phase7LivePrivateContentMarker(invalid); err == nil {
			t.Fatalf("invalid run ID %q produced a private content marker", invalid)
		}
	}
}

func TestPhase7InstalledPrivacyRootsRequireExactRunSocket(t *testing.T) {
	root := t.TempDir()
	roots, err := phase7InstalledPrivacyRoots(filepath.Join(root, "run", "loomd.sock"))
	if err != nil || !reflect.DeepEqual(roots, []string{
		filepath.Join(root, "state"), filepath.Join(root, "diagnostics"),
	}) {
		t.Fatalf("privacy roots = %#v, error = %v", roots, err)
	}
	for _, invalid := range []string{
		filepath.Join(root, "loomd.sock"),
		filepath.Join(root, "run", "alternate.sock"),
		"relative/run/loomd.sock",
	} {
		if _, err := phase7InstalledPrivacyRoots(invalid); err == nil {
			t.Fatalf("invalid installed Socket %q produced privacy roots", invalid)
		}
	}
}

func TestPhase7PlaintextPrivacyScanRejectsStateAndDiagnosticLeaks(t *testing.T) {
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	diagnosticRoot := filepath.Join(root, "diagnostics")
	for _, directory := range []string{stateRoot, diagnosticRoot} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	marker, err := phase7LivePrivateContentMarker("privacy-fixture")
	if err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(stateRoot, "encrypted-record.bin")
	diagnosticFile := filepath.Join(diagnosticRoot, "operational.jsonl")
	if err := os.WriteFile(stateFile, []byte("bounded-ciphertext"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diagnosticFile, []byte("{\"stage\":\"ok\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanPhase7PlaintextPrivacyRoots(
		[]string{stateRoot, diagnosticRoot}, marker, 16, 1<<20,
	); err != nil {
		t.Fatalf("privacy-safe roots rejected: %v", err)
	}

	if err := os.WriteFile(stateFile, []byte("prefix "+marker+" suffix"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanPhase7PlaintextPrivacyRoots(
		[]string{stateRoot, diagnosticRoot}, marker, 16, 1<<20,
	); err == nil || strings.Contains(err.Error(), marker) {
		t.Fatalf("state leak result = %v", err)
	}
	if err := os.WriteFile(stateFile, []byte("bounded-ciphertext"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(diagnosticFile, []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanPhase7PlaintextPrivacyRoots(
		[]string{stateRoot, diagnosticRoot}, marker, 16, 1<<20,
	); err == nil || strings.Contains(err.Error(), marker) {
		t.Fatalf("diagnostic leak result = %v", err)
	}
}

func TestPhase7LiveCapabilitySweepCoversBuiltinRegistryExactlyOnce(t *testing.T) {
	fixture := phase7LiveCapabilityFixture{
		runID: "fixture", sourceA: "source-a", sourceB: "source-b",
		missionID: "mission-fixture", teamInstanceID: "team-fixture",
		roundSessionID: "roundtable-fixture", roundID: "round-fixture",
		seatID: "seat-fixture", attemptID: "attempt-fixture",
		incidentID: "incident-fixture", profileID: "profile-fixture",
		modelID: "model-fixture",
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[controltool.ToolID]int)
	groups := phase7LiveCapabilityGroups(fixture)
	if len(groups) != len(registry.Definitions()) {
		t.Fatalf("capability turns = %d, want %d", len(groups), len(registry.Definitions()))
	}
	for _, group := range groups {
		if len(group.expected) != 1 || len(group.proposalTools) > 1 ||
			strings.Contains(group.prompt, "loom.") {
			t.Fatalf("invalid single-tool natural-language capability turn: %#v", group)
		}
		proposalSet := make(map[controltool.ToolID]struct{}, len(group.proposalTools))
		for _, toolID := range group.proposalTools {
			proposalSet[toolID] = struct{}{}
		}
		for _, toolID := range group.expected {
			definition, found := registry.Definition(toolID)
			if !found {
				t.Fatalf("capability group references unknown Tool %s", toolID)
			}
			_, proposed := proposalSet[toolID]
			if proposed != (definition.Effect == controltool.EffectProposal) {
				t.Fatalf("capability group effect drift for %s", toolID)
			}
			seen[toolID]++
		}
	}
	definitions := registry.Definitions()
	if len(seen) != len(definitions) {
		t.Fatalf("capability sweep covers %d Tools, want %d", len(seen), len(definitions))
	}
	for _, definition := range definitions {
		if seen[definition.ID] != 1 {
			t.Fatalf("capability sweep coverage for %s = %d, want 1", definition.ID, seen[definition.ID])
		}
	}
}

func TestValidatePhase7SweepProposalsCancelledRejectsMissingRestoredProposal(t *testing.T) {
	if err := validatePhase7SweepProposalsCancelled(
		api.LocalProductChatThread{},
		[]controltool.ToolID{controltool.ToolMissionsCreatePreview},
	); err == nil {
		t.Fatal("missing restored Proposal was accepted")
	}
	if err := validatePhase7SweepProposalsCancelled(
		api.LocalProductChatThread{}, nil,
	); err != nil {
		t.Fatalf("empty read-only turn should remain valid: %v", err)
	}
}

func TestSelectPhase7GovernanceAnchorRequiresBlockedTeamWithConfiguredAgents(t *testing.T) {
	blockedMission := api.LocalProductMissionSummary{
		MissionID: "mission-p7-blocked", TeamInstanceID: "team-p7-related", Status: "blocked",
	}
	snapshot := api.LocalProductSnapshot{
		Missions: []api.LocalProductMissionSummary{
			{MissionID: "mission-active", TeamInstanceID: "team-unrelated", Status: "running"},
			blockedMission,
		},
		Teams: []api.LocalProductTeamSummary{
			{TeamInstanceID: "team-unrelated"},
			{
				TeamInstanceID: "team-p7-related",
				Agents: []api.LocalProductTeamAgentSummary{
					{
						RoleKind: "main", AgentDefinitionID: "agent-main",
						RuntimeProfileID: "profile-main", BindingStatus: "configured",
					},
					{
						RoleKind: "subagent", AgentDefinitionID: "agent-reviewer",
						RuntimeProfileID: "profile-reviewer", BindingStatus: "configured",
					},
				},
			},
		},
	}
	anchor, err := selectPhase7GovernanceAnchor(snapshot, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if anchor.missionID != blockedMission.MissionID ||
		anchor.teamInstanceID != blockedMission.TeamInstanceID ||
		len(anchor.agents) != roundtable.MinAgentSeats {
		t.Fatalf("governance anchor = %#v", anchor)
	}

	snapshot.Teams[1].Agents[1].BindingStatus = "unavailable"
	if _, err := selectPhase7GovernanceAnchor(snapshot, "", ""); err == nil {
		t.Fatal("blocked Mission without two configured Agent bindings was accepted")
	}
	snapshot.Teams[1].Agents[1].BindingStatus = "configured"
	snapshot.Missions = append(snapshot.Missions, api.LocalProductMissionSummary{
		MissionID: "mission-p7-second", TeamInstanceID: "team-p7-related", Status: "blocked",
	})
	if _, err := selectPhase7GovernanceAnchor(snapshot, "", ""); err == nil {
		t.Fatal("ambiguous P7 governance anchors were accepted")
	}
	anchor, err = selectPhase7GovernanceAnchor(
		snapshot, blockedMission.MissionID, blockedMission.TeamInstanceID,
	)
	if err != nil || anchor.missionID != blockedMission.MissionID {
		t.Fatalf("explicit P7 governance anchor = %#v, error = %v", anchor, err)
	}
}

func TestSelectPhase7RoundTableActionTargetEnforcesCurrentActionState(t *testing.T) {
	binding := func(digest string) *roundtable.FrozenSeatBinding {
		return &roundtable.FrozenSeatBinding{
			MembershipRevision: 1, BindingDigest: digest,
		}
	}
	view := roundtable.View{
		Session: roundtable.Session{
			ID: "session-current", ModeratorSeat: "seat-moderator",
			Context: &roundtable.SessionContext{
				MissionID: "mission-current", TeamID: "team-current",
			},
		},
		Seats: map[string]roundtable.Seat{
			"seat-main": {
				ID: "seat-main", Available: true, Binding: binding("binding-main"),
			},
		},
		Rounds: []roundtable.Round{{ID: "round-current", Sequence: 1}},
		Attempts: map[string]roundtable.SeatAttempt{
			"attempt-main": {
				AttemptID: "attempt-main", RoundID: "round-current", SeatID: "seat-main",
				MembershipRevision: 1, SeatBindingDigest: "binding-main",
				Status: roundtable.SeatAttemptRunning,
			},
		},
		Interventions: map[string]roundtable.Intervention{},
	}
	if _, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesPausePreview,
	); err != nil {
		t.Fatalf("current open Round was rejected for pause: %v", err)
	}
	if target, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesSteerPreview,
	); err != nil || target.attemptID != "attempt-main" {
		t.Fatalf("running Attempt steer target = %#v, error = %v", target, err)
	}
	if _, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesRetryPreview,
	); err == nil {
		t.Fatal("running Attempt was accepted for retry")
	}
	if _, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesSkipPreview,
	); err == nil {
		t.Fatal("Seat with a running Attempt was accepted for skip")
	}

	attempt := view.Attempts["attempt-main"]
	attempt.Status = roundtable.SeatAttemptCancelled
	view.Attempts[attempt.AttemptID] = attempt
	for _, toolID := range []controltool.ToolID{
		controltool.ToolRoundtablesRetryPreview,
		controltool.ToolRoundtablesSkipPreview,
		controltool.ToolRoundtablesReplacePreview,
	} {
		if _, err := selectPhase7RoundTableActionTarget(view, toolID); err != nil {
			t.Fatalf("cancelled Attempt target rejected for %s: %v", toolID, err)
		}
	}

	view.Session.Concluded = true
	if _, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesPausePreview,
	); err == nil {
		t.Fatal("concluded Session was accepted for pause")
	}
	view.Session.Concluded = false
	view.Interventions["skip-main"] = roundtable.Intervention{
		ID: "skip-main", Kind: roundtable.InterventionSkipSeat,
		RoundID: "round-current", SeatID: "seat-main",
	}
	if _, err := selectPhase7RoundTableActionTarget(
		view, controltool.ToolRoundtablesReplacePreview,
	); err == nil {
		t.Fatal("already skipped Seat was accepted for replacement")
	}
}

func TestLoadPhase7RoundtableRegistryRejectsUnsafeOrAmbiguousFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "roundtable-sessions.json")
	payload := []byte(`{"schemaVersion":1,"sessions":[{"missionID":"mission-1","sessionID":"session-1"}]}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := loadPhase7RoundtableRegistry(path)
	if err != nil || registry["mission-1"] != "session-1" {
		t.Fatalf("registry = %#v, error = %v", registry, err)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPhase7RoundtableRegistry(path); err == nil {
		t.Fatal("world-readable RoundTable registry was accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, "roundtable-link.json")
	if err := os.Symlink(path, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPhase7RoundtableRegistry(symlink); err == nil {
		t.Fatal("RoundTable registry symlink was accepted")
	}
	if err := os.WriteFile(
		path,
		[]byte(`{"schemaVersion":1,"schemaVersion":1,"sessions":[]}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPhase7RoundtableRegistry(path); err == nil {
		t.Fatal("duplicate RoundTable registry key was accepted")
	}
}

func TestPhase7LiveThreadFailureUsesSafeStructuredFields(t *testing.T) {
	thread := api.LocalProductChatThread{
		AvailabilityFailure: &api.LocalProductChatAvailabilityFailure{
			Code: "provider_unavailable", Stage: "conversation_dispatch",
			IncidentID: "incident-1", Retryable: true,
		},
	}
	if got := phase7LiveThreadFailure(thread); got !=
		"code=provider_unavailable stage=conversation_dispatch incident=incident-1 retryable=true" {
		t.Fatalf("failure = %q", got)
	}
	if !phase7LiveThreadFailureRetryable(thread) {
		t.Fatal("retryable availability failure was not classified for bounded retry")
	}
	thread.AvailabilityFailure = nil
	thread.Attempts = []api.LocalProductConversationAttempt{{
		Status: "failed", FailureCode: "timeout", FailureStage: "provider_connect",
		IncidentID: "incident-2", Retryable: true,
	}}
	if got := phase7LiveThreadFailure(thread); got !=
		"code=timeout stage=provider_connect provider_code= incident=incident-2 retryable=true" {
		t.Fatalf("attempt failure = %q", got)
	}
	if !phase7LiveThreadFailureRetryable(thread) {
		t.Fatal("retryable Attempt failure was not classified for bounded retry")
	}
	thread.Attempts[0].Retryable = false
	if phase7LiveThreadFailureRetryable(thread) {
		t.Fatal("terminal Attempt failure was classified for retry")
	}
}

func TestValidatePhase7CompletedToolsRequiresExactTargetSet(t *testing.T) {
	threadWith := func(calls ...controltool.CompletedCall) api.LocalProductChatThread {
		return api.LocalProductChatThread{Attempts: []api.LocalProductConversationAttempt{{
			Status: "succeeded", CompletedControlTools: calls,
		}}}
	}
	status := controltool.CompletedCall{
		ToolID: controltool.ToolMissionsStatus, ToolVersion: 1,
		Effect: controltool.EffectRead,
	}
	continued := controltool.CompletedCall{
		ToolID: controltool.ToolMissionsContinuePreview, ToolVersion: 2,
		Effect: controltool.EffectProposal,
	}
	created := controltool.CompletedCall{
		ToolID: controltool.ToolMissionsCreatePreview, ToolVersion: 2,
		Effect: controltool.EffectProposal,
	}
	for name, thread := range map[string]api.LocalProductChatThread{
		"missing target":        threadWith(status),
		"duplicate target":      threadWith(continued, continued),
		"unexpected read":       threadWith(status, continued),
		"unexpected mutation":   threadWith(status, created, continued),
		"no completed attempts": {},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validatePhase7CompletedTools(
				thread, controltool.ToolMissionsContinuePreview,
			); err == nil {
				t.Fatal("invalid Tool audit accepted")
			}
		})
	}
}

func TestPhase7ExpectedInstalledIdentityRequiresCanonicalDigests(t *testing.T) {
	appDigest := strings.Repeat("a", 64)
	daemonDigest := strings.Repeat("b", 64)
	identity, err := newPhase7ExpectedInstalledIdentity(
		"264", appDigest, daemonDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Build != "264" || identity.AppExecutableSHA256 != appDigest ||
		identity.DaemonSHA256 != daemonDigest {
		t.Fatalf("identity = %#v", identity)
	}

	for _, testCase := range []struct {
		name   string
		build  string
		app    string
		daemon string
	}{
		{name: "missing build", app: appDigest, daemon: daemonDigest},
		{name: "short app digest", build: "264", app: strings.Repeat("a", 63), daemon: daemonDigest},
		{name: "uppercase app digest", build: "264", app: strings.Repeat("A", 64), daemon: daemonDigest},
		{name: "non hex daemon digest", build: "264", app: appDigest, daemon: strings.Repeat("z", 64)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := newPhase7ExpectedInstalledIdentity(
				testCase.build, testCase.app, testCase.daemon,
			); err == nil {
				t.Fatal("invalid installed identity was accepted")
			}
		})
	}
}

func TestPhase7InstalledFileDigestRejectsMutationAndSymlink(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "LoomLocalApp")
	if err := os.WriteFile(filePath, []byte("phase7-app"), 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := phase7InstalledFileSHA256(filePath)
	if err != nil || len(digest) != 64 {
		t.Fatalf("digest = %q, error = %v", digest, err)
	}
	if err := requirePhase7InstalledFileDigest(filePath, digest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte("changed"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := requirePhase7InstalledFileDigest(filePath, digest); err == nil {
		t.Fatal("changed installed executable digest was accepted")
	}

	symlinkPath := filepath.Join(root, "loomd")
	if err := os.Symlink(filePath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := phase7InstalledFileSHA256(symlinkPath); err == nil {
		t.Fatal("installed executable symlink was accepted")
	}
}

func TestPhase7InstalledDaemonPIDParserHandlesSpacesAndDuplicates(t *testing.T) {
	helper := "/tmp/Loom Candidate.app/Contents/Library/Helpers/loomd"
	socket := "/tmp/Application Support/Loom/run/loomd.sock"
	line := "  101 " + helper + " --state /tmp/state --socket " + socket +
		" --managed-parent-pid 100"
	output := "  98 " + helper + " --not--socket " + socket + "\n" +
		"  99 " + helper + "-old --socket " + socket + "\n" + line + "\n"
	pid, err := phase7InstalledDaemonPIDFromPS(output, helper, socket)
	if err != nil || pid != "101" {
		t.Fatalf("pid = %q, error = %v", pid, err)
	}
	if _, err := phase7InstalledDaemonPIDFromPS(
		output+line+"\n", helper, socket,
	); err == nil {
		t.Fatal("duplicate installed managed daemons were accepted")
	}
}

func TestPhase7LiveClientConstructionCanWaitForRunDirectory(t *testing.T) {
	tempRoot, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(tempRoot, "loom-p7-client-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	missingSocket := filepath.Join(root, "missing", "loomd.sock")
	if _, err := phase7LiveClient(missingSocket); err == nil {
		t.Fatal("missing private run directory was accepted")
	}
	runDir := filepath.Join(root, "run")
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := phase7LiveClient(filepath.Join(runDir, "loomd.sock")); err != nil {
		t.Fatalf("private run directory should be retryable before socket creation: %v", err)
	}
}
