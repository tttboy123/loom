package provider

import (
	"strconv"
	"strings"

	"loom-pi-rebuild/internal/credentials"
)

const (
	CodexConversationProfileID      = "conversation-openai-codex-default-v1"
	ClaudeCodeConversationProfileID = "conversation-anthropic-claude-code-default-v1"
	AnthropicConversationModelID    = "claude-sonnet-5"
	DeepSeekConversationModelID     = "deepseek-chat"
	KimiConversationModelID         = "kimi-k2.6"
	MiniMaxConversationModelID      = "MiniMax-M3"
	OpenCodeConversationProfileID   = "conversation-opencode-default-v1"
	PiConversationProfileID         = "conversation-loom-local-pi-default-v1"
	PiConversationModelID           = "qwen2.5-coder-1.5b-instruct-q4-k-m"
)

func AnthropicConversationProfileID(revision int64) string {
	return AnthropicConversationAccountProfileID("anthropic.primary", revision)
}

func AnthropicConversationAccountProfileID(accountID string, revision int64) string {
	return conversationAccountProfileID(
		"anthropic", accountID, "conversation-anthropic-claude-sonnet-5", revision,
	)
}

func DeepSeekConversationProfileID(revision int64) string {
	return DeepSeekConversationAccountProfileID("deepseek.primary", revision)
}

func DeepSeekConversationAccountProfileID(accountID string, revision int64) string {
	return conversationAccountProfileID(
		"deepseek", accountID, "conversation-deepseek-deepseek-chat", revision,
	)
}

func KimiConversationProfileID(revision int64) string {
	return KimiConversationAccountProfileID("kimi.primary", revision)
}

func KimiConversationAccountProfileID(accountID string, revision int64) string {
	return conversationAccountProfileID(
		"kimi", accountID, "conversation-kimi-kimi-k2.6", revision,
	)
}

func MiniMaxConversationProfileID(revision int64) string {
	return MiniMaxConversationAccountProfileID("minimax.primary", revision)
}

func MiniMaxConversationAccountProfileID(accountID string, revision int64) string {
	return conversationAccountProfileID(
		"minimax", accountID, "conversation-minimax-minimax-m3", revision,
	)
}

func OpenCodeConversationAccountProfileID(
	providerID string,
	accountID string,
	revision int64,
) string {
	return conversationAccountProfileID(
		providerID,
		accountID,
		"conversation-opencode-"+providerID,
		revision,
	)
}

func conversationAccountProfileID(
	providerID,
	accountID,
	prefix string,
	revision int64,
) string {
	if revision <= 0 || prefix == "" ||
		!credentials.ValidProviderAccountIdentifier(providerID, accountID) {
		return ""
	}
	suffix := strings.TrimPrefix(accountID, providerID+".")
	if suffix == "primary" {
		return prefix + "-r" + strconv.FormatInt(revision, 10)
	}
	return prefix + "-account-" + suffix + "-r" + strconv.FormatInt(revision, 10)
}

// Descriptor describes a Provider capability without carrying credentials or
// making an availability claim. Connection state is resolved by the product
// service at runtime.
type Descriptor struct {
	ID                     string
	DisplayName            string
	Category               string
	Protocol               string
	AuthMode               string
	ConnectionKind         string
	SupportsModelDiscovery bool
}

var providerCatalog = []Descriptor{
	{ID: "openai", DisplayName: "OpenAI", Category: "official", Protocol: "openai_responses", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "anthropic", DisplayName: "Anthropic", Category: "official", Protocol: "anthropic_messages", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "google-gemini", DisplayName: "Google Gemini", Category: "official", Protocol: "gemini_generate_content", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "deepseek", DisplayName: "DeepSeek", Category: "official", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "kimi", DisplayName: "Kimi", Category: "official", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "minimax", DisplayName: "MiniMax", Category: "official", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "xai", DisplayName: "xAI", Category: "official", Protocol: "openai_responses", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "zhipu", DisplayName: "Zhipu GLM", Category: "official", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "alibaba-bailian", DisplayName: "Alibaba Bailian", Category: "cloud", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "tencent-hunyuan", DisplayName: "Tencent Hunyuan", Category: "cloud", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "baidu-qianfan", DisplayName: "Baidu Qianfan", Category: "cloud", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "stepfun", DisplayName: "StepFun", Category: "official", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "modelscope", DisplayName: "ModelScope", Category: "cloud", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "openrouter", DisplayName: "OpenRouter", Category: "gateway", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "siliconflow", DisplayName: "SiliconFlow", Category: "gateway", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "nvidia-nim", DisplayName: "NVIDIA NIM", Category: "cloud", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "novita", DisplayName: "Novita AI", Category: "gateway", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "api_key", SupportsModelDiscovery: true},
	{ID: "azure-openai", DisplayName: "Azure OpenAI", Category: "cloud", Protocol: "openai_compatible", AuthMode: "provider_ephemeral", ConnectionKind: "managed_cloud", SupportsModelDiscovery: true},
	{ID: "aws-bedrock", DisplayName: "AWS Bedrock", Category: "cloud", Protocol: "bedrock_converse", AuthMode: "provider_ephemeral", ConnectionKind: "managed_cloud", SupportsModelDiscovery: true},
	{ID: "google-vertex", DisplayName: "Google Vertex AI", Category: "cloud", Protocol: "vertex_generate_content", AuthMode: "provider_ephemeral", ConnectionKind: "managed_cloud", SupportsModelDiscovery: true},
	{ID: "ollama", DisplayName: "Ollama", Category: "local", Protocol: "ollama", AuthMode: "native_auth", ConnectionKind: "local_runtime", SupportsModelDiscovery: true},
	{ID: "lm-studio", DisplayName: "LM Studio", Category: "local", Protocol: "openai_compatible", AuthMode: "native_auth", ConnectionKind: "local_runtime", SupportsModelDiscovery: true},
	{ID: "custom-openai", DisplayName: "Custom OpenAI-compatible", Category: "custom", Protocol: "openai_compatible", AuthMode: "brokered", ConnectionKind: "custom_endpoint", SupportsModelDiscovery: true},
	{ID: "opencode", DisplayName: "OpenCode", Category: "official", Protocol: "opencode_agent", AuthMode: "native_auth", ConnectionKind: "native_runtime", SupportsModelDiscovery: true},
	{ID: "custom-anthropic", DisplayName: "Custom Anthropic-compatible", Category: "custom", Protocol: "anthropic_messages", AuthMode: "brokered", ConnectionKind: "custom_endpoint", SupportsModelDiscovery: true},
}

func Catalog() []Descriptor {
	return append([]Descriptor(nil), providerCatalog...)
}

// ModelCatalog returns only actual model service Providers. Harnesses such as
// OpenCode are discovered through the Runtime inventory and must never be
// projected as Provider Accounts or credential destinations.
func ModelCatalog() []Descriptor {
	result := make([]Descriptor, 0, len(providerCatalog))
	for _, descriptor := range providerCatalog {
		if descriptor.ConnectionKind == "native_runtime" {
			continue
		}
		result = append(result, descriptor)
	}
	return result
}

func DescriptorByID(id string) (Descriptor, bool) {
	for _, descriptor := range providerCatalog {
		if descriptor.ID == id {
			return descriptor, true
		}
	}
	return Descriptor{}, false
}
