package provider

import (
	"errors"
	"testing"
)

func TestConversationModelCatalogLayers(t *testing.T) {
	// Provider -> Model dependency (grounded in official catalogs).
	deepSeekModels := ProviderConversationModels("deepseek")
	ids := map[string]bool{}
	efforts := map[string][]string{}
	for _, model := range deepSeekModels {
		ids[model.ID] = true
		efforts[model.ID] = append([]string(nil), model.ReasoningEfforts...)
	}
	for _, want := range []string{
		"deepseek-v4-flash", "deepseek-v4-pro",
		"deepseek-chat", "deepseek-reasoner",
	} {
		if !ids[want] {
			t.Fatalf("deepseek model %q missing: %#v", want, deepSeekModels)
		}
	}
	// Official DeepSeek API: v4 models accept low/high/max; legacy aliases
	// declare no effort parameter.
	if len(efforts["deepseek-v4-flash"]) != 3 ||
		len(efforts["deepseek-v4-pro"]) != 3 ||
		len(efforts["deepseek-chat"]) != 0 ||
		len(efforts["deepseek-reasoner"]) != 0 {
		t.Fatalf("deepseek efforts = %#v", efforts)
	}
	openAIModels := ProviderConversationModels("openai")
	openAIIndex := map[string]bool{}
	openAIEfforts := map[string][]string{}
	for _, model := range openAIModels {
		openAIIndex[model.ID] = true
		openAIEfforts[model.ID] = append([]string(nil), model.ReasoningEfforts...)
	}
	for _, want := range []string{
		"codex-default", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna",
		"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex-spark",
		"codex-auto-review", "deepseek-v4-flash", "deepseek-v4-pro",
	} {
		if !openAIIndex[want] {
			t.Fatalf("openai model %q missing: %#v", want, openAIModels)
		}
	}
	// Grounded in the installed Codex CLI 0.144.1 model catalog: each GPT
	// model declares its exact reasoning levels; codex-default (no override)
	// declares none.
	if len(openAIEfforts["gpt-5.6-sol"]) != 6 ||
		len(openAIEfforts["gpt-5.6-terra"]) != 6 ||
		len(openAIEfforts["gpt-5.6-luna"]) != 5 ||
		len(openAIEfforts["gpt-5.5"]) != 4 ||
		len(openAIEfforts["codex-default"]) != 0 {
		t.Fatalf("openai efforts = %#v", openAIEfforts)
	}
	// Model -> Reasoning dependency
	opencode := ProviderConversationModels("opencode")
	opencodeModels := map[string]bool{}
	opencodeEfforts := map[string][]string{}
	for _, model := range opencode {
		opencodeModels[model.ID] = true
		opencodeEfforts[model.ID] = append([]string(nil), model.ReasoningEfforts...)
	}
	for _, required := range []string{
		"deepseek/deepseek-chat", "deepseek/deepseek-v4-flash",
		"deepseek/deepseek-v4-pro", "minimax/MiniMax-M3", "zai/glm-4.5",
		"opencode/deepseek-v4-flash-free",
	} {
		if !opencodeModels[required] {
			t.Fatalf("opencode models missing %q: %#v", required, opencodeModels)
		}
	}
	// Reasoning efforts follow the OpenCode CLI 1.18.3 per-model capability:
	// v4-flash -> low/high/max, v4-pro -> high/max, glm-5.2 -> high/max,
	// toggle-only models declare none, and the invalid zhipu/ prefix is not
	// used (zai is the CLI prefix).
	if opencodeModels["zhipu/glm-4.5"] {
		t.Fatalf("zhipu/ prefix must not be used; use zai/glm-4.5")
	}
	if len(opencodeEfforts["deepseek/deepseek-v4-flash"]) == 0 ||
		len(opencodeEfforts["deepseek/deepseek-chat"]) != 0 ||
		len(opencodeEfforts["deepseek/deepseek-v4-pro"]) != 2 ||
		len(opencodeEfforts["zai/glm-5.2"]) != 2 {
		t.Fatalf("opencode reasoning efforts = %#v", opencodeEfforts)
	}
	// Kimi: kimi-k3 carries low/high/max; kimi-k2.6 is toggle-only.
	kimi := ProviderConversationModels("kimi")
	kimiIndex := map[string]bool{}
	for _, model := range kimi {
		kimiIndex[model.ID] = true
	}
	if !kimiIndex["kimi-k3"] || !kimiIndex[KimiConversationModelID] {
		t.Fatalf("kimi models = %#v", kimi)
	}
}

func TestValidateConversationModelAndReasoning(t *testing.T) {
	model, err := ValidateConversationModel("deepseek", "deepseek-v4-flash")
	if err != nil || model.ID != "deepseek-v4-flash" {
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
	// v4-pro accepts high/max but not low (OpenCode CLI 1.18.3).
	v4pro, err := ValidateConversationModel("opencode", "deepseek/deepseek-v4-pro")
	if err != nil {
		t.Fatalf("v4-pro model error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(v4pro, "high"); err != nil {
		t.Fatalf("v4-pro high effort error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(v4pro, "low"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("v4-pro low effort should fail closed = %v", err)
	}
	if err := ValidateConversationReasoningEffort(opencodeModel, ""); err != nil {
		t.Fatalf("empty reasoning error = %v", err)
	}
	// DeepSeek v4-flash declares low/high/max; a valid effort passes and an
	// unsupported value fails closed.
	deepSeekV4, err := ValidateConversationModel("deepseek", "deepseek-v4-flash")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateConversationReasoningEffort(deepSeekV4, "high"); err != nil {
		t.Fatalf("deepseek v4 reasoning error = %v", err)
	}
	if err := ValidateConversationReasoningEffort(deepSeekV4, "xhigh"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("deepseek unsupported reasoning error = %v", err)
	}
	// Legacy deepseek-chat declares no reasoning: non-empty must fail closed.
	deepSeekChat, err := ValidateConversationModel("deepseek", "deepseek-chat")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateConversationReasoningEffort(deepSeekChat, "high"); !errors.Is(
		err, ErrInvalidConversationModel,
	) {
		t.Fatalf("deepseek chat reasoning error = %v", err)
	}
}
