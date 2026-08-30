package provider

import (
	"errors"
	"strings"
)

// OpenCode multi-model adaptation.
//
// OpenCode identifies a model as "provider/model" (for example
// "deepseek/deepseek-chat") and reads provider credentials from well-known
// environment variables. These helpers map a Loom Provider + Model pair onto
// that contract so one OpenCode runtime can serve many Providers and models
// without hardcoding a single backend.

var (
	ErrOpenCodeModelAdaptation = errors.New("invalid OpenCode model adaptation")
)

// OpenCodeCredentialEnv returns the environment variable OpenCode reads for a
// Provider's API key. The names are grounded in the OpenCode 1.18 binary. A
// Provider with no known env credential returns ok=false (OpenCode native auth
// may still cover it).
func OpenCodeCredentialEnv(providerID string) (string, bool) {
	switch providerID {
	case "openai":
		return "OPENAI_API_KEY", true
	case "anthropic":
		return "ANTHROPIC_API_KEY", true
	case "deepseek":
		return "DEEPSEEK_API_KEY", true
	case "kimi":
		return "MOONSHOT_API_KEY", true
	case "minimax":
		return "MINIMAX_API_KEY", true
	case "minimax-cn":
		return "MINIMAX_API_KEY", true
	case "xai":
		return "XAI_API_KEY", true
	case "zhipu", "zai":
		// OpenCode identifies GLM models under the `zai` provider prefix and
		// reads the Zhipu key from ZHIPU_API_KEY.
		return "ZHIPU_API_KEY", true
	case "opencode":
		// The OpenCode provider identity is the harness itself; its hosted
		// free-tier models run under OpenCode's own native auth and Loom
		// manages no "opencode" account, so there is no key to inject.
		return "", false
	case "stepfun":
		return "STEPFUN_API_KEY", true
	case "openrouter":
		return "OPENROUTER_API_KEY", true
	case "google-gemini":
		return "GOOGLE_GENERATIVE_AI_API_KEY", true
	case "alibaba-bailian":
		return "DASHSCOPE_API_KEY", true
	case "ollama":
		return "OLLAMA_API_KEY", true
	case "lm-studio":
		return "LMSTUDIO_API_KEY", true
	default:
		return "", false
	}
}

// OpenCodeRuntimeProviderID maps a Loom-owned Provider identity to the
// provider prefix implemented by OpenCode. Loom's MiniMax account is verified
// against the minimaxi.com region, whose OpenCode identity is `minimax-cn`.
// Credential ownership remains with the Loom Provider returned by
// OpenCodeLoomProviderID.
func OpenCodeRuntimeProviderID(providerID string) (string, bool) {
	providerID = strings.TrimSpace(providerID)
	if !validOpenCodeProviderPart(providerID) {
		return "", false
	}
	switch providerID {
	case "minimax":
		return "minimax-cn", true
	case "zhipu":
		return "zai", true
	default:
		return providerID, true
	}
}

// OpenCodeLoomProviderID resolves an OpenCode provider prefix back to the
// Loom Provider Account namespace used by the Credential Vault.
func OpenCodeLoomProviderID(runtimeProviderID string) (string, bool) {
	runtimeProviderID = strings.TrimSpace(runtimeProviderID)
	if !validOpenCodeProviderPart(runtimeProviderID) {
		return "", false
	}
	switch runtimeProviderID {
	case "minimax-cn":
		return "minimax", true
	case "zai":
		return "zhipu", true
	default:
		return runtimeProviderID, true
	}
}

// OpenCodeBrokeredModelSupported is the single compatibility gate for a
// Loom-owned Provider Account routed through OpenCode. MiniMax is intentionally
// excluded: current OpenCode releases can complete HTTP 200 while dropping
// MiniMax-M3 text and tool parts, so advertising that route would be a false
// executable capability. MiniMax remains available through Loom Native.
func OpenCodeBrokeredModelSupported(
	loomProviderID string,
	modelIdentity string,
) bool {
	if loomProviderID == "" || loomProviderID == "opencode" ||
		loomProviderID == "minimax" || !ValidOpenCodeModelIdentity(modelIdentity) {
		return false
	}
	runtimeProviderID, ok := OpenCodeRuntimeProviderID(loomProviderID)
	if !ok {
		return false
	}
	separator := strings.IndexByte(modelIdentity, '/')
	if separator <= 0 || modelIdentity[:separator] != runtimeProviderID {
		return false
	}
	_, credentialKnown := OpenCodeCredentialEnv(runtimeProviderID)
	return credentialKnown
}

// OpenCodeModelIdentity maps a Loom Provider + Model pair to OpenCode's
// "provider/model" identity.
//
//   - If model already contains a "/", it must be "provider/model" whose
//     provider part equals providerID (no cross-Provider substitution).
//   - Otherwise the identity is providerID + "/" + model.
//
// Empty inputs and malformed identities fail closed.
func OpenCodeModelIdentity(providerID string, modelID string) (string, error) {
	providerID = strings.TrimSpace(providerID)
	modelID = strings.TrimSpace(modelID)
	if providerID == "" || modelID == "" ||
		len(providerID) > 128 || len(modelID) > 256 ||
		!validOpenCodeProviderPart(providerID) {
		return "", ErrOpenCodeModelAdaptation
	}
	runtimeProviderID, ok := OpenCodeRuntimeProviderID(providerID)
	if !ok {
		return "", ErrOpenCodeModelAdaptation
	}
	if strings.Contains(modelID, "/") {
		// Gateway Providers (for example openrouter) accept a qualified model
		// naming the upstream Provider: openrouter + deepseek/deepseek-chat ->
		// openrouter/deepseek/deepseek-chat. Non-gateway Providers must not be
		// silently pointed at another Provider's model.
		if runtimeProviderID == "openrouter" {
			identity := runtimeProviderID + "/" + modelID
			if !validOpenCodeModelID(identity) {
				return "", ErrOpenCodeModelAdaptation
			}
			return identity, nil
		}
		parts := strings.SplitN(modelID, "/", 2)
		if len(parts) != 2 || parts[0] != runtimeProviderID ||
			!validOpenCodeProviderPart(parts[0]) ||
			!validOpenCodeModelPart(parts[1]) {
			return "", ErrOpenCodeModelAdaptation
		}
		identity := runtimeProviderID + "/" + parts[1]
		if !validOpenCodeModelID(identity) {
			return "", ErrOpenCodeModelAdaptation
		}
		return identity, nil
	}
	if !validOpenCodeModelPart(modelID) {
		return "", ErrOpenCodeModelAdaptation
	}
	identity := runtimeProviderID + "/" + modelID
	if !validOpenCodeModelID(identity) {
		return "", ErrOpenCodeModelAdaptation
	}
	return identity, nil
}

func validOpenCodeProviderPart(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r == '-' || r == '.' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validOpenCodeModelPart(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			strings.ContainsRune(".-_:", r) {
			continue
		}
		return false
	}
	return true
}

// ValidOpenCodeModelIdentity validates an already-qualified "provider/model"
// identity (for example "deepseek/deepseek-chat" or the OpenRouter
// "openrouter/deepseek/deepseek-chat").
func ValidOpenCodeModelIdentity(identity string) bool {
	if !validOpenCodeModelID(identity) {
		return false
	}
	parts := strings.SplitN(identity, "/", 2)
	return len(parts) == 2 &&
		validOpenCodeProviderPart(parts[0]) &&
		validOpenCodeModelPart(parts[1])
}
