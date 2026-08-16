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
	for _, model := range opencode {
		opencodeModels[model.ID] = true
		if len(model.ReasoningEfforts) == 0 {
			t.Fatalf("opencode model %q should declare reasoning efforts", model.ID)
		}
	}
	if !opencodeModels["deepseek/deepseek-chat"] ||
		!opencodeModels["minimax/MiniMax-M3"] ||
		!opencodeModels["zhipu/glm-4.5"] {
		t.Fatalf("opencode models = %#v", opencodeModels)
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
	opencodeModel, err := ValidateConversationModel("opencode", "zhipu/glm-4.5")
	if err != nil {
		t.Fatalf("opencode model error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(opencodeModel, "high"); err != nil {
		t.Fatalf("opencode reasoning error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(opencodeModel, "ultra"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("unsupported reasoning error = %v", err)
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
