// Command loom-net-probe tests the installed Loom daemon's network access:
//  1. local IPC reachability,
//  2. daemon -> model Provider network egress via a real chat_message,
//  3. whether Chat exposes web/tool capabilities (expected: no tools).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"loom-pi-rebuild/internal/localipc"
)

type profile struct {
	ProfileID          string `json:"profile_id"`
	HarnessAdapter     string `json:"harness_adapter"`
	ProviderID         string `json:"provider_id"`
	ProviderAccountID  string `json:"provider_account_id"`
	CredentialRevision int64  `json:"credential_revision"`
	ModelID            string `json:"model_id"`
	AuthMode           string `json:"auth_mode"`
	PolicyVersion      int    `json:"policy_version"`
	PolicyRevision     int64  `json:"policy_revision"`
	PolicyDigest       string `json:"policy_digest"`
	TrustDomain        string `json:"trust_domain"`
	RetentionMode      string `json:"retention_mode"`
	DataRegion         string `json:"data_region"`
}

type chatReply struct {
	Content            string
	ContextTokenBudget int
	ContextTokenCount  int
}

type chatThread struct {
	ThreadID  string `json:"thread_id"`
	ProfileID string `json:"profile_id"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Attempts []struct {
		ContextTokenBudget int `json:"context_token_budget"`
		ContextTokenCount  int `json:"context_token_count"`
	} `json:"attempts"`
}

type probeOptions struct {
	setupOnly bool
}

func parseProbeArgs(args []string, output io.Writer) (probeOptions, error) {
	options := probeOptions{}
	flags := flag.NewFlagSet("loom-net-probe", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(
		&options.setupOnly,
		"setup-only",
		false,
		"inspect installed Provider, Runtime, Profile, and import catalogs without a Provider request",
	)
	if err := flags.Parse(args); err != nil {
		return probeOptions{}, err
	}
	if flags.NArg() != 0 {
		return probeOptions{}, fmt.Errorf("unexpected arguments")
	}
	return options, nil
}

func socketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(home, "/") {
		return "", fmt.Errorf("invalid home")
	}
	return home + "/Library/Application Support/Loom/run/loomd.sock", nil
}

func main() {
	options, err := parseProbeArgs(os.Args[1:], os.Stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return
		}
		fatal(err)
	}
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
		Providers            []struct {
			ProviderID string `json:"provider_id"`
			Status     string `json:"status"`
		} `json:"providers"`
		Runtimes []struct {
			RuntimeInstanceID string `json:"runtime_instance_id"`
			Status            string `json:"status"`
		} `json:"runtimes"`
		CredentialImports []struct {
			SourceApplication string   `json:"source_application"`
			TargetProviderID  string   `json:"target_provider_id"`
			ModelIDs          []string `json:"model_ids"`
			ImportMode        string   `json:"import_mode"`
		} `json:"credential_import_candidates"`
	}
	setupStarted := time.Now()
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &setup); err != nil {
		fatal(fmt.Errorf("setup_snapshot (local IPC): %w", err))
	}
	fmt.Printf("[1] local IPC reachable: ok (%d conversation profiles, setup=%s)\n",
		len(setup.ConversationProfiles), time.Since(setupStarted).Round(time.Millisecond))
	for _, p := range setup.ConversationProfiles {
		fmt.Printf("    profile=%s harness=%s provider=%s account=%q revision=%d model=%s auth=%s\n",
			p.ProfileID, p.HarnessAdapter, p.ProviderID, p.ProviderAccountID,
			p.CredentialRevision, p.ModelID, p.AuthMode)
	}
	fmt.Printf(
		"    setup providers=%d runtimes=%d credential_import_candidates=%d\n",
		len(setup.Providers), len(setup.Runtimes), len(setup.CredentialImports),
	)
	for _, p := range setup.Providers {
		fmt.Printf("    provider=%s status=%s\n", p.ProviderID, p.Status)
	}
	for _, runtime := range setup.Runtimes {
		fmt.Printf("    runtime=%s status=%s\n", runtime.RuntimeInstanceID, runtime.Status)
	}
	for _, candidate := range setup.CredentialImports {
		fmt.Printf(
			"    import_source=%q target_provider=%s mode=%s models=%d\n",
			candidate.SourceApplication, candidate.TargetProviderID,
			candidate.ImportMode, len(candidate.ModelIDs),
		)
	}
	if options.setupOnly {
		return
	}

	// Prefer an account-scoped OpenCode route so the installed probe verifies
	// exact Harness + Provider Account + credential revision + model binding.
	target := profile{}
	for _, p := range setup.ConversationProfiles {
		if p.HarnessAdapter == "opencode" && p.AuthMode == "brokered" &&
			p.ProviderAccountID != "" {
			target = p
			break
		}
	}
	if target.ProfileID == "" {
		for _, p := range setup.ConversationProfiles {
			if p.AuthMode == "brokered" && p.ProviderAccountID != "" {
				target = p
				break
			}
		}
	}
	if target.ProfileID == "" && len(setup.ConversationProfiles) > 0 {
		target = setup.ConversationProfiles[0]
	}
	if target.ProfileID == "" {
		fatal(fmt.Errorf("no conversation profile available"))
	}
	binding := map[string]any{
		"schema_version":                   4,
		"harness_adapter":                  target.HarnessAdapter,
		"provider_id":                      target.ProviderID,
		"provider_account_id":              target.ProviderAccountID,
		"credential_revision":              target.CredentialRevision,
		"model_id":                         target.ModelID,
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
		fmt.Printf("[2] chat (network egress): got reply len=%d egress_ok=%t context=%d/%d estimated tokens\n",
			len(reply.Content), ok, reply.ContextTokenCount, reply.ContextTokenBudget)
		if !ok {
			fmt.Printf("    reply: %s\n", truncate(reply.Content, 240))
		}
	}

	// 3. A successful reply passed the strict conversation decoder. Any
	// OpenCode tool event would have failed the request instead of being hidden
	// behind model-authored self-report text.
	if err == nil {
		fmt.Println("[3] chat tool boundary: tool_event_observed=false (strict text-only event stream)")
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
	if len(threadResult.Attempts) == 0 {
		return chatReply{}, fmt.Errorf("chat response missing frozen attempt")
	}
	latest := threadResult.Attempts[len(threadResult.Attempts)-1]
	if !validContextTokenProjection(
		latest.ContextTokenBudget,
		latest.ContextTokenCount,
	) {
		return chatReply{}, fmt.Errorf("chat response missing valid context token projection")
	}
	return chatReply{
		Content:            lastContent,
		ContextTokenBudget: latest.ContextTokenBudget,
		ContextTokenCount:  latest.ContextTokenCount,
	}, nil
}

func validContextTokenProjection(budget int, count int) bool {
	return budget > 0 && count > 0 && count <= budget
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
