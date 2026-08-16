package provider

import (
	"errors"
	"sort"
	"strings"
)

// Conversation model catalog for the three-layer client selection:
// Provider -> Model -> Reasoning Effort. The catalog is the single source of
// truth for which models a Provider supports and which reasoning efforts a
// model supports, so the client can render dependent pickers and the router
// can fail closed on unsupported combinations.

var (
	ErrInvalidConversationModel = errors.New("invalid conversation model")
)

// ConversationModel describes one selectable conversation model. ReasoningEfforts
// lists the reasoning intensities the model supports; empty means the model does
// not accept a reasoning-effort parameter.
type ConversationModel struct {
	ID               string
	DisplayName      string
	ReasoningEfforts []string
}

// ProviderConversationModels returns the selectable conversation models for a
// Provider. OpenCode is dynamic: it accepts any "provider/model" identity, and
// its reasoning intensities map to the OpenCode `--variant` values.
func ProviderConversationModels(providerID string) []ConversationModel {
	switch providerID {
	case "openai":
		// Model list grounded in the Codex CLI 0.144.1 binary catalog
		// (gpt-5.5 / gpt-5.5-pro / gpt-5.4 / gpt-5.4-mini / gpt-5.2 /
		// gpt-5.1-codex-max / gpt-5.6-* / o3). `codex-default` is the Loom
		// alias for the profile default (no --model override).
		return []ConversationModel{
			{ID: "codex-default", DisplayName: "GPT-5.5 (Codex default)"},
			{ID: "gpt-5.5", DisplayName: "GPT-5.5"},
			{ID: "gpt-5.5-pro", DisplayName: "GPT-5.5 Pro"},
			{ID: "gpt-5.4", DisplayName: "GPT-5.4"},
			{ID: "gpt-5.4-mini", DisplayName: "GPT-5.4 Mini"},
			{ID: "gpt-5.2", DisplayName: "GPT-5.2"},
			{ID: "gpt-5.1-codex-max", DisplayName: "GPT-5.1 Codex Max"},
			{ID: "gpt-5.6-terra", DisplayName: "GPT-5.6 Terra"},
			{ID: "gpt-5.6-sol", DisplayName: "GPT-5.6 Sol"},
			{ID: "gpt-5.6-luna", DisplayName: "GPT-5.6 Luna"},
			{ID: "o3", DisplayName: "o3"},
			{ID: "deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", ReasoningEfforts: []string{"none", "high"}},
			{ID: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", ReasoningEfforts: []string{"none", "high"}},
		}
	case "deepseek":
		return []ConversationModel{
			{ID: "deepseek-chat", DisplayName: "DeepSeek Chat"},
			{ID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner"},
		}
	case "kimi":
		return []ConversationModel{
			{ID: KimiConversationModelID, DisplayName: "Kimi K2.6"},
		}
	case "minimax":
		return []ConversationModel{
			{ID: MiniMaxConversationModelID, DisplayName: "MiniMax M3"},
		}
	case "anthropic":
		return []ConversationModel{
			{ID: AnthropicConversationModelID, DisplayName: "Claude Sonnet 5"},
		}
	case "opencode":
		// Grounded in the installed OpenCode CLI 1.18.3 model catalog
		// (`opencode models` + models.dev cache). Provider prefixes and
		// reasoning efforts follow the CLI: deepseek/*, minimax/*, zai/* (GLM
		// uses the `zai` provider prefix with ZHIPU_API_KEY), and the hosted
		// opencode/* free tier. `openai/*` is intentionally not listed because
		// the Codex profile already covers OpenAI models and the OpenCode CLI
		// does not expose an openai provider in this environment.
		return []ConversationModel{
			{ID: "deepseek/deepseek-chat", DisplayName: "DeepSeek Chat"},
			{ID: "deepseek/deepseek-reasoner", DisplayName: "DeepSeek Reasoner"},
			{ID: "deepseek/deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", ReasoningEfforts: []string{"low", "high", "max"}},
			{ID: "deepseek/deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", ReasoningEfforts: []string{"low", "high", "max"}},
			{ID: "minimax/MiniMax-M2.7", DisplayName: "MiniMax M2.7"},
			{ID: "minimax/MiniMax-M3", DisplayName: "MiniMax M3"},
			{ID: "zai/glm-4.5", DisplayName: "Zhipu GLM-4.5"},
			{ID: "zai/glm-5.2", DisplayName: "Zhipu GLM-5.2", ReasoningEfforts: []string{"high", "max"}},
			{ID: "opencode/deepseek-v4-flash-free", DisplayName: "DeepSeek V4 Flash (OpenCode)", ReasoningEfforts: []string{"low", "high", "max"}},
		}
	default:
		return nil
	}
}

// ValidateConversationModel checks that modelID is a selectable model for the
// Provider and returns the normalized model.
func ValidateConversationModel(providerID, modelID string) (ConversationModel, error) {
	models := ProviderConversationModels(providerID)
	for _, model := range models {
		if model.ID == modelID {
			return model, nil
		}
	}
	if providerID == "opencode" {
		// OpenCode is dynamic: any valid "provider/model" identity is allowed,
		// using the catalog entry when present for display and reasoning.
		if !ValidOpenCodeModelIdentity(modelID) {
			return ConversationModel{}, ErrInvalidConversationModel
		}
		return ConversationModel{ID: modelID, DisplayName: modelID}, nil
	}
	return ConversationModel{}, ErrInvalidConversationModel
}

// SupportedConversationReasoningEfforts returns the sorted reasoning efforts a
// model supports (empty means none).
func SupportedConversationReasoningEfforts(model ConversationModel) []string {
	efforts := append([]string(nil), model.ReasoningEfforts...)
	sort.Strings(efforts)
	return efforts
}

// ValidateConversationReasoningEffort fails closed when the model does not
// support the requested reasoning effort.
func ValidateConversationReasoningEffort(
	model ConversationModel,
	reasoningEffort string,
) error {
	reasoningEffort = strings.TrimSpace(reasoningEffort)
	if reasoningEffort == "" {
		return nil
	}
	for _, effort := range model.ReasoningEfforts {
		if effort == reasoningEffort {
			return nil
		}
	}
	return ErrInvalidConversationModel
}
