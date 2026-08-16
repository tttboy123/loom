package provider

import (
	"errors"
	"testing"
)

func TestConversationModelCatalogLayers(t *testing.T) {
	// Provider -> Model dependency
	deepSeekModels := ProviderConversationModels("deepseek")
	ids := map[string]bool{}
	for _, model := range deepSeekModels {
		ids[model.ID] = true
	}
	if !ids["deepseek-chat"] || !ids["deepseek-reasoner"] {
		t.Fatalf("deepseek models = %#v", deepSeekModels)
	}
	openAIModels := ProviderConversationModels("openai")
	openAIIndex := map[string]bool{}
	for _, model := range openAIModels {
		openAIIndex[model.ID] = true
	}
	for _, want := range []string{
		"codex-default", "gpt-5.5", "gpt-5.5-pro", "gpt-5.4",
		"gpt-5.4-mini", "gpt-5.2", "gpt-5.1-codex-max",
		"deepseek-v4-flash", "deepseek-v4-pro",
	} {
		if !openAIIndex[want] {
			t.Fatalf("openai model %q missing: %#v", want, openAIModels)
		}
	}
	// Model -> Reasoning dependency
	for _, model := range deepSeekModels {
		if len(model.ReasoningEfforts) != 0 {
			t.Fatalf("deepseek model %q should declare no reasoning effort", model.ID)
		}
	}
	opencode := ProviderConversationModels("opencode")
	opencodeModels := map[string]bool{}
	efforts := map[string][]string{}
	for _, model := range opencode {
		opencodeModels[model.ID] = true
		efforts[model.ID] = append([]string(nil), model.ReasoningEfforts...)
	}
	for _, required := range []string{
		"deepseek/deepseek-chat", "deepseek/deepseek-v4-flash",
		"minimax/MiniMax-M3", "zai/glm-4.5", "opencode/deepseek-v4-flash-free",
	} {
		if !opencodeModels[required] {
			t.Fatalf("opencode models missing %q: %#v", required, opencodeModels)
		}
	}
	// Reasoning efforts follow the OpenCode CLI 1.18.3 per-model capability:
	// effort-capable models declare their values, toggle-only models declare
	// none, and the invalid zhipu/ prefix is not used (zai is the CLI prefix).
	if opencodeModels["zhipu/glm-4.5"] {
		t.Fatalf("zhipu/ prefix must not be used; use zai/glm-4.5")
	}
	if len(efforts["deepseek/deepseek-v4-flash"]) == 0 ||
		len(efforts["deepseek/deepseek-chat"]) != 0 {
		t.Fatalf("reasoning efforts = %#v", efforts)
	}
}

func TestValidateConversationModelAndReasoning(t *testing.T) {
	model, err := ValidateConversationModel("deepseek", "deepseek-chat")
	if err != nil || model.ID != "deepseek-chat" {
		t.Fatalf("model=%#v error=%v", model, err)
	}
	if _, err := ValidateConversationModel("deepseek", "gpt-5"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("cross model error = %v", err)
	}
	opencodeModel, err := ValidateConversationModel("opencode", "zai/glm-4.5")
	if err != nil {
		t.Fatalf("opencode model error = %v", err)
	}
	// Toggle-only GLM-4.5 has no effort values: empty effort is valid and a
	// named effort is rejected.
	if err := ValidateConversationReasoningEffort(opencodeModel, ""); err != nil {
		t.Fatalf("opencode reasoning error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(opencodeModel, "high"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("unsupported reasoning error = %v", err)
	}
	flash, err := ValidateConversationModel("opencode", "deepseek/deepseek-v4-flash")
	if err != nil || flash.ID != "deepseek/deepseek-v4-flash" {
		t.Fatalf("flash model=%#v error=%v", flash, err)
	}
	if err := ValidateConversationReasoningEffort(flash, "max"); err != nil {
		t.Fatalf("flash max effort error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(opencodeModel, ""); err != nil {
		t.Fatalf("empty reasoning error = %v", err)
	}
	// DeepSeek chat declares no reasoning: non-empty must fail closed.
	deepSeekChat, err := ValidateConversationModel("deepseek", "deepseek-chat")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateConversationReasoningEffort(deepSeekChat, "high"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("deepseek reasoning error = %v", err)
	}
}
