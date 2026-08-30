package main

import (
	"strings"
	"testing"

	"loom-pi-rebuild/internal/controltool"
)

func TestProductConversationControlSystemPromptFreezesRegistryToolSelection(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := productConversationControlSystemPrompt(
		"minimax", "minimax-cn/MiniMax-M3", "opencode", registry,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"provider=minimax", "model=minimax-cn/MiniMax-M3", "harness=opencode",
		"loom_missions_create_preview", "loom_teams_create_preview",
		"loom_sessions_align_preview", "loom_roundtables_open_preview",
		"prose-only", "user must confirm",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("system prompt missing %q: %s", required, prompt)
		}
	}
	if len(prompt) > 4096 {
		t.Fatalf("full Registry system prompt is %d bytes, want at most 4096", len(prompt))
	}
}

func TestProductConversationControlSystemPromptWithoutRegistryAdvertisesNoTools(t *testing.T) {
	prompt, err := productConversationControlSystemPrompt(
		"openai", "codex-default", "codex", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Tools are unavailable") ||
		strings.Contains(prompt, "loom_missions_create_preview") {
		t.Fatalf("system prompt = %q", prompt)
	}
}
