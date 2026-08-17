// Command loom-net-probe tests the installed Loom daemon's network access:
//  1. local IPC reachability,
//  2. daemon -> model Provider network egress via a real chat_message,
//  3. whether Chat exposes web/tool capabilities (expected: no tools).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"loom-pi-rebuild/internal/localipc"
)

type profile struct {
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

type chatReply struct {
	Content string
}

type chatThread struct {
	ThreadID  string `json:"thread_id"`
	ProfileID string `json:"profile_id"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func socketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(home, "/") {
		return "", fmt.Errorf("invalid home")
	}
	return home + "/Library/Application Support/Loom/run/loomd.sock", nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	path, err := socketPath()
	if err != nil {
		fatal(err)
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: path, Timeout: 5 * time.Second, ExtendedTimeout: 55 * time.Second,
	})
	if err != nil {
		fatal(fmt.Errorf("local IPC client: %w", err))
	}
	// 1. Local IPC reachability.
	var setup struct {
		ConversationProfiles []profile `json:"conversation_profiles"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &setup); err != nil {
		fatal(fmt.Errorf("setup_snapshot (local IPC): %w", err))
	}
	fmt.Printf("[1] local IPC reachable: ok (%d conversation profiles)\n", len(setup.ConversationProfiles))
	for _, p := range setup.ConversationProfiles {
		fmt.Printf("    profile=%s provider=%s account=%q model=%s auth=%s\n",
			p.ProfileID, p.ProviderID, p.ProviderAccountID, p.ModelID, p.AuthMode)
	}

	// Pick the first verified brokered profile (network model), else first native.
	target := profile{}
	for _, p := range setup.ConversationProfiles {
		if p.AuthMode == "brokered" && p.ProviderAccountID != "" {
			target = p
			break
		}
	}
	if target.ProfileID == "" && len(setup.ConversationProfiles) > 0 {
		target = setup.ConversationProfiles[0]
	}
	if target.ProfileID == "" {
		fatal(fmt.Errorf("no conversation profile available"))
	}
	binding := map[string]any{
		"schema_version":                   3,
		"provider_id":                      target.ProviderID,
		"provider_account_id":              target.ProviderAccountID,
		"provider_account_policy_version":  target.PolicyVersion,
		"provider_account_policy_revision": target.PolicyRevision,
		"provider_account_policy_digest":   target.PolicyDigest,
		"trust_domain":                     target.TrustDomain,
		"retention_mode":                   target.RetentionMode,
		"data_region":                      target.DataRegion,
	}

	// 2. Real network egress: daemon -> provider API.
	reply, err := chat(ctx, client, target, binding, "Reply with exactly: Loom-NET-OK", "probe-net-ok")
	if err != nil {
		fmt.Printf("[2] chat (network egress) FAILED: %v\n", err)
	} else {
		ok := strings.Contains(reply.Content, "Loom-NET-OK")
		fmt.Printf("[2] chat (network egress): got reply len=%d egress_ok=%t\n",
			len(reply.Content), ok)
		if !ok {
			fmt.Printf("    reply: %s\n", truncate(reply.Content, 240))
		}
	}

	// 3. Chat tool surface (expect NO: Chat has no web/tools by design).
	toolReply, toolErr := chat(ctx, client, target, binding,
		"Do you have live web search or browsing tools available right now? Answer with exactly YES or NO.", "probe-net-tools")
	if toolErr != nil {
		fmt.Printf("[3] chat (tool probe) FAILED: %v\n", toolErr)
	} else {
		answer := strings.ToUpper(strings.TrimSpace(toolReply.Content))
		yes := strings.HasPrefix(answer, "YES")
		fmt.Printf("[3] chat tool surface: model answered %q -> tools_present=%t (expected false: Chat is a plain text responder)\n",
			truncate(toolReply.Content, 60), yes)
	}
}

func chat(ctx context.Context, client *localipc.Client, target profile, binding map[string]any, content, thread string) (chatReply, error) {
	params := map[string]any{
		"thread_id": thread, "content": content,
		"profile_id": target.ProfileID, "model_id": target.ModelID,
		"context_mode":               "start_clean",
		"expected_execution_binding": binding,
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "chat_message", params, &raw); err != nil {
		return chatReply{}, err
	}
	var threadResult chatThread
	if err := json.Unmarshal(raw, &threadResult); err != nil {
		return chatReply{}, err
	}
	lastContent := ""
	if len(threadResult.Messages) > 0 {
		lastContent = threadResult.Messages[len(threadResult.Messages)-1].Content
	}
	return chatReply{Content: lastContent}, nil
}

func truncate(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) > maximum {
		return string(runes[:maximum]) + "…"
	}
	return value
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

var _ = json.Marshal
