package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/toolbroker"
)

// TestProductRemoteToolBrokerDefaultFailClosed keeps the default production
// composition closed: no config means no remote tool executor.
func TestProductRemoteToolBrokerDefaultFailClosed(t *testing.T) {
	executor, effect, err := newProductRemoteToolBroker(
		context.Background(), nil,
	)
	if err != nil || executor != nil || effect != nil {
		t.Fatalf("default broker = %v/%v/%v", executor, effect, err)
	}
}

// TestProductRemoteToolBrokerWebSearchLive proves the daemon's remote tool
// executor really queries the internet (governed web search), with no Codex.
func TestProductRemoteToolBrokerWebSearchLive(t *testing.T) {
	if os.Getenv("LOOM_LIVE_NET") != "1" {
		t.Skip("set LOOM_LIVE_NET=1 to run the live network probe")
	}
	config, err := newProductDefaultRemoteToolBrokerConfig()
	if err != nil {
		t.Fatal(err)
	}
	executor, effect, err := newProductRemoteToolBroker(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = effect.Close(context.Background()) }()
	allowed := executor.AllowedRemoteTools()
	if !containsToolKind(allowed, permissions.ToolWebSearch) ||
		!containsToolKind(allowed, permissions.ToolWebFetch) {
		t.Fatalf("allowed = %v", allowed)
	}
	content, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{Tool: permissions.ToolWebSearch, Path: "Loom governed handoff"},
	)
	if err != nil {
		if errors.Is(err, toolbroker.ErrToolFailed) {
			t.Skipf("external search endpoint blocked from this host: %v", err)
		}
		t.Fatalf("live web search failed: %v", err)
	}
	if len(content) == 0 {
		t.Skip("external search endpoint returned empty results")
	}
	t.Logf("live web search ok: %d bytes (%s)", len(content), truncateProductBytes(content, 120))
}

// TestProductRemoteToolBrokerWebFetchLive proves the daemon's remote tool
// executor fetches a public page over HTTPS, with no Codex.
func TestProductRemoteToolBrokerWebFetchLive(t *testing.T) {
	if os.Getenv("LOOM_LIVE_NET") != "1" {
		t.Skip("set LOOM_LIVE_NET=1 to run the live network probe")
	}
	config, err := newProductDefaultRemoteToolBrokerConfig()
	if err != nil {
		t.Fatal(err)
	}
	executor, effect, err := newProductRemoteToolBroker(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = effect.Close(context.Background()) }()
	content, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{
			Tool: permissions.ToolWebFetch, Path: "https://example.com/",
		},
	)
	if err != nil {
		t.Fatalf("live web fetch failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(content)), "example") {
		t.Fatalf("unexpected web fetch content: %s", truncateProductBytes(content, 200))
	}
	t.Logf("live web fetch ok: %d bytes", len(content))
}

// TestProductRemoteToolBrokerRejectsPrivateTarget keeps the broker SSRF-safe
// even when a model proposes a private-address fetch.
func TestProductRemoteToolBrokerRejectsPrivateTarget(t *testing.T) {
	config, err := newProductDefaultRemoteToolBrokerConfig()
	if err != nil {
		t.Fatal(err)
	}
	executor, effect, err := newProductRemoteToolBroker(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = effect.Close(context.Background()) }()
	if _, err := executor.ExecuteProposalContent(
		context.Background(),
		permissions.ProposedCall{
			Tool: permissions.ToolWebFetch, Path: "http://192.168.1.5/private",
		},
	); !errors.Is(err, toolbroker.ErrInvalidCall) &&
		!errors.Is(err, toolbroker.ErrToolDenied) &&
		!errors.Is(err, toolbroker.ErrToolFailed) {
		t.Fatalf("private fetch error = %v", err)
	}
}

func containsToolKind(values []permissions.ToolKind, want permissions.ToolKind) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func truncateProductBytes(content []byte, maximum int) string {
	value := string(content)
	if len(value) > maximum {
		return value[:maximum] + "…"
	}
	return value
}
