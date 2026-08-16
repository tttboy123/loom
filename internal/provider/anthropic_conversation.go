package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/prompting"
)

const AnthropicConversationEndpoint = "https://api.anthropic.com/v1/messages"

var (
	ErrInvalidAnthropicConversation     = errors.New("invalid Anthropic conversation")
	ErrAnthropicConversationUnavailable = ErrProviderConversationUnavailable
)

type AnthropicConversationConfig struct {
	Client           HTTPDoer
	Timeout          time.Duration
	MaxResponseBytes int64
}

type AnthropicConversationClient struct {
	client           HTTPDoer
	timeout          time.Duration
	maxResponseBytes int64
}

func NewAnthropicConversationClient(
	config AnthropicConversationConfig,
) (*AnthropicConversationClient, error) {
	if interfaceIsNil(config.Client) || config.Timeout <= 0 ||
		config.Timeout > 2*time.Minute || config.MaxResponseBytes < 256 ||
		config.MaxResponseBytes > 1<<20 ||
		!exactConversationEndpoint(AnthropicConversationEndpoint) {
		return nil, ErrInvalidAnthropicConversation
	}
	return &AnthropicConversationClient{
		client: config.Client, timeout: config.Timeout,
		maxResponseBytes: config.MaxResponseBytes,
	}, nil
}

func NewSystemAnthropicConversationClient(
	timeout time.Duration,
	maxResponseBytes int64,
) (*AnthropicConversationClient, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidAnthropicConversation
	}
	privateTransport := transport.Clone()
	privateTransport.Proxy = nil
	privateTransport.DisableCompression = true
	client := &http.Client{
		Transport: privateTransport,
		Timeout:   timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return NewAnthropicConversationClient(AnthropicConversationConfig{
		Client: client, Timeout: timeout, MaxResponseBytes: maxResponseBytes,
	})
}

func (client *AnthropicConversationClient) Respond(
	ctx context.Context,
	messages []ConversationMessage,
	secret []byte,
) (string, error) {
	return client.RespondConfigured(ctx, messages, secret, "", "")
}

// RespondConfigured runs one conversation turn with an explicit model and
// optional reasoning effort. Anthropic conversation models currently declare
// no reasoning-effort parameter, so a non-empty reasoning effort fails closed.
func (client *AnthropicConversationClient) RespondConfigured(
	ctx context.Context,
	messages []ConversationMessage,
	secret []byte,
	modelID string,
	reasoningEffort string,
) (string, error) {
	if client == nil || ctx == nil || len(messages) == 0 || len(messages) > 64 ||
		len(secret) == 0 || len(secret) > 8192 {
		return "", ErrInvalidAnthropicConversation
	}
	if modelID == "" {
		modelID = AnthropicConversationModelID
	}
	model, err := ValidateConversationModel("anthropic", modelID)
	if err != nil {
		return "", ErrInvalidAnthropicConversation
	}
	if reasoningEffort != "" {
		if err := ValidateConversationReasoningEffort(model, reasoningEffort); err != nil {
			return "", ErrInvalidAnthropicConversation
		}
	}
	type wireMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	systemPrompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeConversation, ProviderID: "anthropic",
		ModelID: modelID, HarnessAdapter: "loom-native",
	})
	if err != nil {
		return "", ErrInvalidAnthropicConversation
	}
	wireMessages := make([]wireMessage, 0, len(messages))
	totalBytes := 0
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if (message.Role != "user" && message.Role != "assistant") ||
			content == "" || len(content) > maxConversationContentBytes ||
			!utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
			return "", ErrInvalidAnthropicConversation
		}
		totalBytes += len(content)
		if totalBytes > 64*1024 {
			return "", ErrInvalidAnthropicConversation
		}
		wireMessages = append(wireMessages, wireMessage{
			Role: message.Role, Content: content,
		})
	}
	requestBody := struct {
		Model     string        `json:"model"`
		System    string        `json:"system"`
		Messages  []wireMessage `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
	}{
		Model:    modelID,
		System:   systemPrompt,
		Messages: wireMessages, MaxTokens: 2048,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil || len(payload) > 96*1024 {
		return "", ErrInvalidAnthropicConversation
	}
	requestContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext, http.MethodPost, AnthropicConversationEndpoint,
		bytes.NewReader(payload),
	)
	if err != nil || !exactConversationURL(request.URL, AnthropicConversationEndpoint) {
		return "", ErrInvalidAnthropicConversation
	}
	request.Header.Set("x-api-key", string(secret))
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, requestErr := client.client.Do(request)
	request.Header.Del("x-api-key")
	if requestErr != nil {
		return "", classifyConversationTransportFailure(requestErr)
	}
	if response == nil || response.Body == nil {
		return "", newConversationFailure("provider_http", "state_unavailable", true)
	}
	defer response.Body.Close()
	if response.Request != nil &&
		!exactConversationURL(response.Request.URL, AnthropicConversationEndpoint) {
		return "", newConversationFailure("provider_http", "provider_rejected", false)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, client.maxResponseBytes+1))
	if readErr != nil || int64(len(body)) > client.maxResponseBytes {
		return "", newConversationFailure("provider_http", "state_unavailable", true)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", classifyConversationHTTPFailure(response.StatusCode, response.Header, body)
	}
	var decoded struct {
		Type       string `json:"type"`
		Role       string `json:"role"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(body, &decoded) != nil || decoded.Type != "message" ||
		decoded.Role != "assistant" || decoded.Model != AnthropicConversationModelID ||
		len(decoded.Content) == 0 ||
		(decoded.StopReason != "end_turn" && decoded.StopReason != "max_tokens" &&
			decoded.StopReason != "stop_sequence") {
		return "", newConversationFailure("provider_http", "invalid_response", false)
	}
	parts := make([]string, 0, len(decoded.Content))
	contentBytes := 0
	for _, block := range decoded.Content {
		text := strings.TrimSpace(block.Text)
		if block.Type != "text" || text == "" || !utf8.ValidString(text) ||
			strings.IndexByte(text, 0) >= 0 {
			return "", newConversationFailure("provider_http", "invalid_response", false)
		}
		contentBytes += len(text)
		if contentBytes > maxConversationContentBytes {
			return "", newConversationFailure("provider_http", "invalid_response", false)
		}
		parts = append(parts, text)
	}
	content := strings.Join(parts, "\n\n")
	if len(content) > maxConversationContentBytes {
		return "", newConversationFailure("provider_http", "invalid_response", false)
	}
	return content, nil
}
