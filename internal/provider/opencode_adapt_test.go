package provider

import (
	"errors"
	"strings"
	"testing"
)

func TestOpenCodeModelIdentityAdaptsProviderModel(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		want     string
	}{
		{"deepseek", "deepseek-chat", "deepseek/deepseek-chat"},
		{"openai", "gpt-5", "openai/gpt-5"},
		{"anthropic", "claude-sonnet-4", "anthropic/claude-sonnet-4"},
		{"kimi", "kimi-k2.6", "kimi/kimi-k2.6"},
		{"minimax", "MiniMax-M3", "minimax-cn/MiniMax-M3"},
		{"xai", "grok-4", "xai/grok-4"},
		{"zhipu", "glm-4.5", "zai/glm-4.5"},
		{"stepfun", "step-2", "stepfun/step-2"},
		{"openrouter", "deepseek/deepseek-chat", "openrouter/deepseek/deepseek-chat"},
	}
	for _, test := range cases {
		got, err := OpenCodeModelIdentity(test.provider, test.model)
		if err != nil || got != test.want {
			t.Fatalf("OpenCodeModelIdentity(%q,%q) = %q, %v; want %q",
				test.provider, test.model, got, err, test.want)
		}
	}
}

func TestOpenCodeRuntimeProviderMappingKeepsLoomAccountIdentity(t *testing.T) {
	runtimeProvider, ok := OpenCodeRuntimeProviderID("minimax")
	if !ok || runtimeProvider != "minimax-cn" {
		t.Fatalf("MiniMax runtime provider = %q, %v", runtimeProvider, ok)
	}
	loomProvider, ok := OpenCodeLoomProviderID(runtimeProvider)
	if !ok || loomProvider != "minimax" {
		t.Fatalf("MiniMax Loom provider = %q, %v", loomProvider, ok)
	}
	if runtimeProvider, ok = OpenCodeRuntimeProviderID("zhipu"); !ok || runtimeProvider != "zai" {
		t.Fatalf("Zhipu runtime provider = %q, %v", runtimeProvider, ok)
	}
	if loomProvider, ok = OpenCodeLoomProviderID("zai"); !ok || loomProvider != "zhipu" {
		t.Fatalf("Zhipu Loom provider = %q, %v", loomProvider, ok)
	}
}

func TestOpenCodeModelIdentityAlreadyQualified(t *testing.T) {
	got, err := OpenCodeModelIdentity("deepseek", "deepseek/deepseek-chat")
	if err != nil || got != "deepseek/deepseek-chat" {
		t.Fatalf("qualified = %q, %v", got, err)
	}
	got, err = OpenCodeModelIdentity("minimax", "minimax-cn/MiniMax-M3")
	if err != nil || got != "minimax-cn/MiniMax-M3" {
		t.Fatalf("qualified MiniMax = %q, %v", got, err)
	}
}

func TestOpenCodeModelIdentityFailsClosed(t *testing.T) {
	cases := []struct {
		provider string
		model    string
	}{
		{"", "deepseek-chat"},
		{"deepseek", ""},
		{"deepseek", "deepseek-chat with spaces"},
		{"deepseek", "openai/gpt-5"},           // cross-provider
		{"deep seek", "deepseek-chat"},         // invalid provider part
		{"deepseek", "deepseek-chat\x00"},      // control char
		{"deepseek", strings.Repeat("m", 300)}, // overlong
	}
	for _, test := range cases {
		if _, err := OpenCodeModelIdentity(test.provider, test.model); !errors.Is(
			err, ErrOpenCodeModelAdaptation,
		) {
			t.Fatalf("OpenCodeModelIdentity(%q,%q) error = %v", test.provider, test.model, err)
		}
	}
}

func TestOpenCodeCredentialEnvGrounding(t *testing.T) {
	cases := map[string]string{
		"openai": "OPENAI_API_KEY", "anthropic": "ANTHROPIC_API_KEY",
		"deepseek": "DEEPSEEK_API_KEY", "kimi": "MOONSHOT_API_KEY",
		"minimax": "MINIMAX_API_KEY", "minimax-cn": "MINIMAX_API_KEY",
		"xai":   "XAI_API_KEY",
		"zhipu": "ZHIPU_API_KEY", "stepfun": "STEPFUN_API_KEY",
		"openrouter": "OPENROUTER_API_KEY", "google-gemini": "GOOGLE_GENERATIVE_AI_API_KEY",
		"alibaba-bailian": "DASHSCOPE_API_KEY", "ollama": "OLLAMA_API_KEY",
		"lm-studio": "LMSTUDIO_API_KEY",
	}
	for provider, want := range cases {
		got, ok := OpenCodeCredentialEnv(provider)
		if !ok || got != want {
			t.Fatalf("OpenCodeCredentialEnv(%q) = %q, %v; want %q", provider, got, ok, want)
		}
	}
	if _, ok := OpenCodeCredentialEnv("unknown-provider"); ok {
		t.Fatal("unknown provider reported a credential env")
	}
	if got, ok := OpenCodeCredentialEnv("opencode"); ok || got != "" {
		t.Fatalf("OpenCodeCredentialEnv(\"opencode\") = %q, %v; want native auth", got, ok)
	}
}
