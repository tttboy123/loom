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
//
// Model lists and reasoning efforts are grounded in each Provider's official
// documentation and the installed runtimes (verified 2026-08-16):
//   - openai/Codex: installed Codex CLI model capabilities with per-model
//     reasoning levels. Third-party cc-switch aliases are excluded until the
//     current runtime explicitly discovers or verifies them.
//   - deepseek: api-docs.deepseek.com current models deepseek-v4-flash /
//     deepseek-v4-pro with reasoning_effort low/high/max (medium/xhigh map to
//     high); deepseek-chat / deepseek-reasoner remain as legacy aliases that
//     map to v4-flash non-thinking / thinking modes.
//   - kimi: platform.kimi.com kimi-k3 with reasoning_effort low/high/max;
//     kimi-k2.6 is a thinking-toggle model with no effort parameter.
//   - minimax: platform.minimax.io MiniMax-M3 reasoning toggle (effort values
//     are accepted for compatibility but do not tune depth -> no picker).
//   - anthropic: claude-sonnet-5 adaptive-only always-on thinking.
//   - opencode: installed OpenCode CLI 1.18.3 model catalog (models.dev
//     cache). Provider prefixes follow the CLI: deepseek/*, minimax/*, zai/*
//     (GLM with ZHIPU_API_KEY), and the hosted opencode/* free tier.
//     `openai/*` is intentionally not listed because the Codex profile already
//     covers OpenAI models and the OpenCode CLI does not expose an openai
//     provider in this environment.

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
		// `codex-default` is the Loom alias for the profile default (no --model
		// override). Every explicit model below passed the installed live gate;
		// third-party runtime aliases require separate capability discovery.
		return []ConversationModel{
			{ID: "codex-default", DisplayName: "Codex default"},
			{ID: "gpt-5.6-sol", DisplayName: "GPT-5.6 Sol", ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max", "ultra"}},
			{ID: "gpt-5.6-terra", DisplayName: "GPT-5.6 Terra", ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max", "ultra"}},
			{ID: "gpt-5.6-luna", DisplayName: "GPT-5.6 Luna", ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max"}},
			{ID: "gpt-5.5", DisplayName: "GPT-5.5", ReasoningEfforts: []string{"low", "medium", "high", "xhigh"}},
			{ID: "gpt-5.4", DisplayName: "GPT-5.4", ReasoningEfforts: []string{"low", "medium", "high", "xhigh"}},
			{ID: "gpt-5.4-mini", DisplayName: "GPT-5.4 Mini", ReasoningEfforts: []string{"low", "medium", "high", "xhigh"}},
			{ID: "gpt-5.3-codex-spark", DisplayName: "GPT-5.3 Codex Spark", ReasoningEfforts: []string{"low", "medium", "high", "xhigh"}},
			{ID: "codex-auto-review", DisplayName: "Codex Auto Review", ReasoningEfforts: []string{"low", "medium", "high", "xhigh", "max"}},
		}
	case "deepseek":
		return []ConversationModel{
			{ID: "deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", ReasoningEfforts: []string{"low", "high", "max"}},
			{ID: "deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", ReasoningEfforts: []string{"low", "high", "max"}},
			{ID: "deepseek-chat", DisplayName: "DeepSeek Chat"},
			{ID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner"},
		}
	case "kimi":
		return []ConversationModel{
			{ID: "kimi-k3", DisplayName: "Kimi K3", ReasoningEfforts: []string{"low", "high", "max"}},
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
		// (`opencode models` + models.dev cache). Reasoning efforts follow each
		// model's real capability: v4-flash -> low/high/max, v4-pro ->
		// high/max, glm-5.2 -> high/max, toggle-only models -> none.
		return []ConversationModel{
			{ID: "deepseek/deepseek-chat", DisplayName: "DeepSeek Chat"},
			{ID: "deepseek/deepseek-reasoner", DisplayName: "DeepSeek Reasoner"},
			{ID: "deepseek/deepseek-v4-flash", DisplayName: "DeepSeek V4 Flash", ReasoningEfforts: []string{"low", "high", "max"}},
			{ID: "deepseek/deepseek-v4-pro", DisplayName: "DeepSeek V4 Pro", ReasoningEfforts: []string{"high", "max"}},
			{ID: "minimax-cn/MiniMax-M2.7", DisplayName: "MiniMax M2.7"},
			{ID: "minimax-cn/MiniMax-M3", DisplayName: "MiniMax M3"},
			{ID: "zai/glm-4.5", DisplayName: "Zhipu GLM-4.5"},
			{ID: "zai/glm-5.2", DisplayName: "Zhipu GLM-5.2", ReasoningEfforts: []string{"high", "max"}},
			{ID: OpenCodeConversationDefaultModel, DisplayName: "Big Pickle (OpenCode)"},
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
