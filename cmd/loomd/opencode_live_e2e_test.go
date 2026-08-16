package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

// TestLiveOpenCodeConversationE2E drives a real OpenCode conversation through
// the installed App daemon (UDS socket) with a real Provider credential leased
// from the Loom Credential Vault. It is a paid, network-enabled live gate and
// is skipped unless LOOM_LIVE_OPENCODE_E2E=1 is set.
//
// Prerequisites:
//   - The installed App is running and its daemon socket is reachable.
//   - The Credential Vault is unlocked and the deepseek account is verified
//     (the model below selects DEEPSEEK_API_KEY from the vault).
func TestLiveOpenCodeConversationE2E(t *testing.T) {
	if os.Getenv("LOOM_LIVE_OPENCODE_E2E") != "1" {
		t.Skip("live OpenCode E2E gate requires LOOM_LIVE_OPENCODE_E2E=1")
	}
	socketPath := os.Getenv("LOOM_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    60 * time.Second,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. Vault status.
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

	// 2. Unlock when the vault reports locked.
	if vaultStatus == "locked" {
		if err := client.Call(ctx, "credential_vault_unlock", struct{}{}, &struct{}{}); err != nil {
			t.Fatalf("unlock error: %v", err)
		}
		t.Log("vault unlocked via route")
	}

	// 3. Send a real OpenCode conversation on a dedicated E2E thread.
	threadID := "thread-live-opencode-e2e-" + time.Now().UTC().Format("20060102T150405Z")
	var result json.RawMessage
	sendCtx, sendCancel := context.WithTimeout(ctx, 60*time.Second)
	defer sendCancel()
	err = client.Call(sendCtx, "chat_message", api.LocalProductChatMessageRequest{
		ThreadID:    threadID,
		Content:     "Reply with exactly: E2E-OK",
		ProfileID:   "conversation-opencode-default-v1",
		ModelID:     "deepseek/deepseek-chat",
		ContextMode: api.ContextModeStartClean,
		ExpectedExecutionBinding: &api.LocalProductConversationExecutionBinding{
			SchemaVersion: 3, ProviderID: "opencode",
		},
		IncidentID: "loom-chat-live-e2e-" + time.Now().UTC().Format("20060102T150405Z"),
	}, &result)
	if err != nil {
		var remote *localipc.RemoteError
		if errors.As(err, &remote) {
			t.Fatalf("opencode chat error: code=%s stage=%s recoverable=%v",
				remote.Code, remote.Stage, remote.Recoverable)
		}
		t.Fatalf("opencode chat error: %+v", err)
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
		t.Fatal("no messages returned")
	}
	last := thread.Messages[len(thread.Messages)-1]
	t.Logf("last message role=%s content=%q", last.Role, last.Content)
	if last.Role != "loom" || last.Content != "E2E-OK" {
		t.Fatalf("expected loom reply E2E-OK, got role=%s content=%q", last.Role, last.Content)
	}
}
