package provider

import "testing"

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
}

func TestProviderCatalogReturnsAnIndependentCopy(t *testing.T) {
	first := Catalog()
	first[0].DisplayName = "changed"
	second := Catalog()
	if second[0].DisplayName == "changed" {
		t.Fatal("Catalog returned shared mutable storage")
	}
}

func TestConversationProfileIDsFreezeExactProviderAccount(t *testing.T) {
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
