package provider

import (
	"strings"
	"testing"
)

func TestProviderCatalogCoversSupportedProtocolFamilies(t *testing.T) {
	catalog := Catalog()
	if len(catalog) < 20 {
		t.Fatalf("catalog entries = %d, want at least 20", len(catalog))
	}

	wantIDs := []string{
		"openai", "anthropic", "google-gemini", "deepseek", "kimi",
		"minimax", "openrouter", "xai", "zhipu", "alibaba-bailian",
		"tencent-hunyuan", "baidu-qianfan", "stepfun", "modelscope",
		"siliconflow", "nvidia-nim", "novita", "azure-openai",
		"aws-bedrock", "google-vertex", "ollama", "lm-studio",
		"custom-openai", "custom-anthropic", "opencode",
	}
	byID := make(map[string]Descriptor, len(catalog))
	for _, entry := range catalog {
		if _, exists := byID[entry.ID]; exists {
			t.Fatalf("duplicate provider ID %q", entry.ID)
		}
		byID[entry.ID] = entry
		if entry.DisplayName == "" || entry.Category == "" ||
			entry.Protocol == "" || entry.ConnectionKind == "" {
			t.Fatalf("incomplete provider descriptor = %#v", entry)
		}
	}
	for _, id := range wantIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("provider %q missing from catalog", id)
		}
	}
	openAI := byID["openai"]
	if openAI.AuthMode != "brokered" || openAI.ConnectionKind != "api_key" {
		t.Fatalf("OpenAI Provider Account boundary = %#v", openAI)
	}
}

func TestProviderCatalogReturnsAnIndependentCopy(t *testing.T) {
	first := Catalog()
	first[0].DisplayName = "changed"
	second := Catalog()
	if second[0].DisplayName == "changed" {
		t.Fatal("Catalog returned shared mutable storage")
	}
}

func TestModelProviderCatalogExcludesHarnessRuntimes(t *testing.T) {
	catalog := ModelCatalog()
	if len(catalog) == 0 {
		t.Fatal("model Provider catalog is empty")
	}
	for _, descriptor := range catalog {
		if descriptor.ConnectionKind == "native_runtime" {
			t.Fatalf("Harness runtime leaked into Provider catalog: %#v", descriptor)
		}
		if descriptor.ID == "opencode" {
			t.Fatalf("OpenCode Harness leaked into Provider catalog: %#v", descriptor)
		}
	}
}

func TestConversationProfileIDsFreezeExactProviderAccount(t *testing.T) {
	if ClaudeCodeConversationProfileID !=
		"conversation-anthropic-claude-code-default-v1" {
		t.Fatalf("Claude Code native profile = %q", ClaudeCodeConversationProfileID)
	}
	if got := AnthropicConversationAccountProfileID("anthropic.primary", 5); got !=
		AnthropicConversationProfileID(5) {
		t.Fatalf("Anthropic primary profile = %q", got)
	}
	if got := AnthropicConversationAccountProfileID("anthropic.work", 5); got !=
		"conversation-anthropic-claude-sonnet-5-account-work-r5" {
		t.Fatalf("Anthropic work profile = %q", got)
	}
	if got := DeepSeekConversationAccountProfileID("deepseek.primary", 7); got != DeepSeekConversationProfileID(7) {
		t.Fatalf("primary compatibility ID = %q", got)
	}
	work := DeepSeekConversationAccountProfileID("deepseek.work", 7)
	personal := DeepSeekConversationAccountProfileID("deepseek.personal", 7)
	if work != "conversation-deepseek-deepseek-chat-account-work-r7" ||
		personal != "conversation-deepseek-deepseek-chat-account-personal-r7" ||
		work == personal {
		t.Fatalf("account profile IDs = %q, %q", work, personal)
	}
	if KimiConversationAccountProfileID("kimi.review.eu", 3) !=
		"conversation-kimi-kimi-k2.6-account-review.eu-r3" ||
		MiniMaxConversationAccountProfileID("minimax.backup", 4) !=
			"conversation-minimax-minimax-m3-account-backup-r4" {
		t.Fatal("non-primary Provider profile ID is not account scoped")
	}
	for _, invalid := range []string{
		DeepSeekConversationAccountProfileID("deepseek", 1),
		DeepSeekConversationAccountProfileID("openai.work", 1),
		DeepSeekConversationAccountProfileID("deepseek.work", 0),
	} {
		if invalid != "" {
			t.Fatalf("invalid account profile ID = %q", invalid)
		}
	}
}

func TestOpenCodeConversationAccountProfileIDFreezesHarnessProviderAccountAndRevision(t *testing.T) {
	primary := OpenCodeConversationAccountProfileID("openai", "openai.primary", 7)
	secondary := OpenCodeConversationAccountProfileID("openai", "openai.team-a", 7)
	otherRevision := OpenCodeConversationAccountProfileID("openai", "openai.primary", 8)
	if primary == "" || secondary == "" || otherRevision == "" ||
		primary == secondary || primary == otherRevision ||
		!strings.HasPrefix(primary, "conversation-opencode-openai-") {
		t.Fatalf("profile ids primary=%q secondary=%q revision=%q", primary, secondary, otherRevision)
	}
	if got := OpenCodeConversationAccountProfileID("openai", "deepseek.primary", 7); got != "" {
		t.Fatalf("cross-provider profile id = %q", got)
	}
}
