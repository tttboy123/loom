package harnessadapter

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"loom-pi-rebuild/internal/runtime/nativeadapter"
)

type AttemptGatewayConfig struct {
	Client           nativeadapter.HTTPDoer
	Random           io.Reader
	MaxRequestBytes  int64
	MaxResponseBytes int64
	MaxRequests      int64
}

type attemptGatewayProvider struct {
	providerID string
	modelID    string
	endpoint   string
	path       string
	authHeader string
	authPrefix string
	invalidErr error
}

type attemptCredentialGateway struct {
	provider         attemptGatewayProvider
	client           nativeadapter.HTTPDoer
	random           io.Reader
	maxRequestBytes  int64
	maxResponseBytes int64
	maxRequests      int64
	randomMu         sync.Mutex
}

type gatewayAttempt struct {
	provider         attemptGatewayProvider
	client           nativeadapter.HTTPDoer
	token            []byte
	secret           []byte
	allowedLoomTools map[string]struct{}
	maxRequestBytes  int64
	maxResponseBytes int64
	maxRequests      int64
	requests         atomic.Int64

	failureMu sync.Mutex
	failure   error
}

func NewAnthropicAttemptGateway(
	config AttemptGatewayConfig,
) (AttemptCredentialGateway, error) {
	return newAttemptCredentialGateway(config, attemptGatewayProvider{
		providerID: ClaudeCodeProviderID,
		modelID:    ClaudeCodeModelID,
		endpoint:   ClaudeCodeEndpoint,
		path:       "/v1/messages",
		authHeader: "x-api-key",
		invalidErr: ErrInvalidClaudeCodeAdapter,
	})
}

func NewOpenAIAttemptGateway(
	config AttemptGatewayConfig,
) (AttemptCredentialGateway, error) {
	return newAttemptCredentialGateway(config, attemptGatewayProvider{
		providerID: CodexProviderID,
		modelID:    CodexModelID,
		endpoint:   CodexEndpoint,
		path:       "/v1/responses",
		authHeader: "Authorization",
		authPrefix: "Bearer ",
		invalidErr: ErrInvalidCodexAdapter,
	})
}

func newAttemptCredentialGateway(
	config AttemptGatewayConfig,
	provider attemptGatewayProvider,
) (AttemptCredentialGateway, error) {
	endpoint, err := url.Parse(provider.endpoint)
	if nilHarnessInterface(config.Client) || nilHarnessInterface(config.Random) ||
		config.MaxRequestBytes < 1024 || config.MaxRequestBytes > 16<<20 ||
		config.MaxResponseBytes < 1024 || config.MaxResponseBytes > 32<<20 ||
		config.MaxRequests < 1 || config.MaxRequests > 512 ||
		!validHarnessID(provider.providerID) || provider.modelID == "" ||
		provider.invalidErr == nil ||
		endpoint == nil || err != nil || endpoint.Scheme != "https" ||
		endpoint.RawQuery != "" || endpoint.Fragment != "" ||
		endpoint.Path != provider.path || provider.authHeader == "" {
		return nil, provider.invalidErr
	}
	return &attemptCredentialGateway{
		provider: provider, client: config.Client, random: config.Random,
		maxRequestBytes:  config.MaxRequestBytes,
		maxResponseBytes: config.MaxResponseBytes,
		maxRequests:      config.MaxRequests,
	}, nil
}

func (gateway *attemptCredentialGateway) WithCredential(
	ctx context.Context,
	providerID string,
	modelID string,
	secret []byte,
	policy AttemptProviderToolPolicy,
	use func(AttemptGatewayLease) error,
) error {
	if gateway == nil {
		return ErrHarnessProcessUnavailable
	}
	if ctx == nil || use == nil ||
		providerID != gateway.provider.providerID || modelID != gateway.provider.modelID ||
		len(secret) == 0 || len(secret) > 8192 {
		return gateway.provider.invalidErr
	}
	allowedLoomTools, validPolicy := normalizeAttemptProviderToolPolicy(policy)
	if !validPolicy {
		return gateway.provider.invalidErr
	}
	rawToken := make([]byte, 32)
	gateway.randomMu.Lock()
	_, randomErr := io.ReadFull(gateway.random, rawToken)
	gateway.randomMu.Unlock()
	if randomErr != nil {
		clearBytes(rawToken)
		return ErrHarnessProcessUnavailable
	}
	token := []byte(hex.EncodeToString(rawToken))
	clearBytes(rawToken)
	secretCopy := append([]byte(nil), secret...)
	attempt := &gatewayAttempt{
		provider: gateway.provider, client: gateway.client,
		token: token, secret: secretCopy, allowedLoomTools: allowedLoomTools,
		maxRequestBytes:  gateway.maxRequestBytes,
		maxResponseBytes: gateway.maxResponseBytes,
		maxRequests:      gateway.maxRequests,
	}
	defer func() {
		clearBytes(attempt.token)
		clearBytes(attempt.secret)
	}()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return ErrHarnessProcessUnavailable
	}
	server := &http.Server{
		Handler:           http.HandlerFunc(attempt.serveHTTP),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       10 * time.Second,
		MaxHeaderBytes:    32 << 10,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	serveDone := make(chan error, 1)
	go func() {
		serveErr := server.Serve(listener)
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		serveDone <- serveErr
	}()
	useDone := make(chan struct{})
	cancelWatchDone := make(chan struct{})
	go func() {
		defer close(cancelWatchDone)
		select {
		case <-ctx.Done():
			_ = server.Close()
		case <-useDone:
		}
	}()

	useErr := use(AttemptGatewayLease{
		BaseURL: "http://" + listener.Addr().String(),
		Token:   string(token),
	})
	close(useDone)
	<-cancelWatchDone
	shutdownContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	shutdownErr := server.Shutdown(shutdownContext)
	cancel()
	serveErr := <-serveDone
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if failure := attempt.currentFailure(); failure != nil {
		return failure
	}
	if useErr != nil {
		return useErr
	}
	if shutdownErr != nil || serveErr != nil {
		return ErrHarnessProcessUnavailable
	}
	return nil
}

func (attempt *gatewayAttempt) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	if attempt == nil || request == nil {
		writeGatewayError(writer, http.StatusUnauthorized)
		return
	}
	if anthropicGatewayHealthProbe(attempt.provider.providerID, request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if attempt.requests.Add(1) > attempt.maxRequests ||
		request.Method != http.MethodPost || request.URL == nil ||
		request.URL.Path != attempt.provider.path ||
		!validAttemptGatewayQuery(attempt.provider.providerID, request.URL) ||
		request.URL.Fragment != "" ||
		!constantTimeEqualWithPrefix(
			request.Header.Get(attempt.provider.authHeader),
			attempt.provider.authPrefix,
			attempt.token,
		) ||
		!strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/json") {
		attempt.setFailure(ErrHarnessProtocol)
		writeGatewayError(writer, http.StatusUnauthorized)
		return
	}
	request.Header.Del(attempt.provider.authHeader)
	body, err := io.ReadAll(io.LimitReader(request.Body, attempt.maxRequestBytes+1))
	if err != nil || int64(len(body)) > attempt.maxRequestBytes ||
		!gatewayBodyUsesFrozenTools(
			attempt.provider.providerID,
			body,
			attempt.provider.modelID,
			attempt.allowedLoomTools,
		) {
		attempt.setFailure(ErrHarnessProtocol)
		writeGatewayError(writer, http.StatusBadRequest)
		return
	}
	upstreamURL := attempt.provider.endpoint
	if request.URL.RawQuery != "" {
		upstreamURL += "?" + request.URL.RawQuery
	}
	upstream, err := http.NewRequestWithContext(
		request.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body),
	)
	if err != nil || upstream.URL.String() != upstreamURL {
		attempt.setFailure(ErrHarnessProtocol)
		writeGatewayError(writer, http.StatusBadGateway)
		return
	}
	copyGatewayRequestHeader(upstream.Header, request.Header, "Content-Type")
	copyGatewayRequestHeader(upstream.Header, request.Header, "Accept")
	copyGatewayRequestHeader(upstream.Header, request.Header, "anthropic-version")
	copyGatewayRequestHeader(upstream.Header, request.Header, "anthropic-beta")
	copyGatewayRequestHeader(upstream.Header, request.Header, "OpenAI-Beta")
	upstream.Header.Set(
		attempt.provider.authHeader,
		attempt.provider.authPrefix+string(attempt.secret),
	)
	response, requestErr := attempt.client.Do(upstream)
	upstream.Header.Del(attempt.provider.authHeader)
	if requestErr != nil {
		if harnessProviderTimeout(requestErr) {
			attempt.setFailure(ErrHarnessProviderTimeout)
		} else {
			attempt.setFailure(ErrHarnessProviderUnavailable)
		}
		writeGatewayError(writer, http.StatusBadGateway)
		return
	}
	if response == nil || response.Body == nil {
		attempt.setFailure(ErrHarnessProviderUnavailable)
		writeGatewayError(writer, http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(
		io.LimitReader(response.Body, attempt.maxResponseBytes+1),
	)
	if readErr != nil {
		if harnessProviderTimeout(readErr) {
			attempt.setFailure(ErrHarnessProviderResponseTimeout)
		} else {
			attempt.setFailure(ErrHarnessProviderUnavailable)
		}
		writeGatewayError(writer, http.StatusBadGateway)
		return
	}
	if int64(len(responseBody)) > attempt.maxResponseBytes {
		attempt.setFailure(ErrHarnessProviderUnavailable)
		writeGatewayError(writer, http.StatusBadGateway)
		return
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 &&
		!gatewayProviderResponseUsesFrozenTools(
			attempt.provider.providerID,
			response.Header.Get("Content-Type"),
			responseBody,
			attempt.allowedLoomTools,
		) {
		attempt.setFailure(ErrHarnessProtocol)
		writeGatewayError(writer, http.StatusBadGateway)
		return
	}
	if failure := gatewayProviderFailure(response.StatusCode); failure != nil {
		attempt.setFailure(failure)
	}
	for _, name := range []string{
		"Content-Type", "request-id", "anthropic-ratelimit-requests-limit",
		"anthropic-ratelimit-requests-remaining", "anthropic-ratelimit-requests-reset",
		"anthropic-ratelimit-tokens-limit", "anthropic-ratelimit-tokens-remaining",
		"anthropic-ratelimit-tokens-reset", "retry-after",
		"x-request-id", "x-ratelimit-limit-requests",
		"x-ratelimit-remaining-requests", "x-ratelimit-reset-requests",
		"x-ratelimit-limit-tokens", "x-ratelimit-remaining-tokens",
		"x-ratelimit-reset-tokens",
	} {
		copyGatewayResponseHeader(writer.Header(), response.Header, name)
	}
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(responseBody)
}

func anthropicGatewayHealthProbe(providerID string, request *http.Request) bool {
	return providerID == ClaudeCodeProviderID && request != nil && request.URL != nil &&
		request.Method == http.MethodHead && request.URL.Path == "/" &&
		request.URL.RawQuery == "" && request.URL.Fragment == ""
}

func validAttemptGatewayQuery(providerID string, requestURL *url.URL) bool {
	if requestURL == nil {
		return false
	}
	if requestURL.RawQuery == "" {
		return true
	}
	if providerID != ClaudeCodeProviderID || requestURL.RawQuery != "beta=true" {
		return false
	}
	values, err := url.ParseQuery(requestURL.RawQuery)
	return err == nil && len(values) == 1 && len(values["beta"]) == 1 &&
		values["beta"][0] == "true"
}

func normalizeAttemptProviderToolPolicy(
	policy AttemptProviderToolPolicy,
) (map[string]struct{}, bool) {
	if len(policy.AllowedLoomTools) > 16 {
		return nil, false
	}
	allowed := make(map[string]struct{}, len(policy.AllowedLoomTools))
	for _, name := range policy.AllowedLoomTools {
		switch name {
		case "loom_read_context", "loom_read_file", "loom_grep_files":
		default:
			return nil, false
		}
		if _, duplicate := allowed[name]; duplicate {
			return nil, false
		}
		allowed[name] = struct{}{}
	}
	return allowed, true
}

func gatewayBodyUsesFrozenTools(
	providerID string,
	body []byte,
	expectedModel string,
	allowed map[string]struct{},
) bool {
	var envelope struct {
		Model string          `json:"model"`
		Tools json.RawMessage `json:"tools"`
	}
	if len(body) == 0 || !gatewayJSONHasUniqueKeys(body) ||
		json.Unmarshal(body, &envelope) != nil ||
		envelope.Model != expectedModel {
		return false
	}
	if len(envelope.Tools) == 0 || bytes.Equal(envelope.Tools, []byte("null")) {
		return true
	}
	var tools []json.RawMessage
	if json.Unmarshal(envelope.Tools, &tools) != nil {
		return false
	}
	seen := make(map[string]struct{}, len(tools))
	for _, raw := range tools {
		var tool struct {
			Type      string `json:"type"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		}
		if json.Unmarshal(raw, &tool) != nil {
			return false
		}
		identity := tool.Type + "\x00" + tool.Namespace + "\x00" + tool.Name
		if _, duplicate := seen[identity]; duplicate {
			return false
		}
		seen[identity] = struct{}{}
		switch providerID {
		case CodexProviderID:
			if !validOpenAIAttemptTool(tool.Type, tool.Namespace, tool.Name, allowed) {
				return false
			}
		case ClaudeCodeProviderID:
			if !validAnthropicAttemptTool(tool.Name, allowed) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func gatewayJSONHasUniqueKeys(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if !consumeGatewayJSONValue(decoder) {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}

func consumeGatewayJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return true
	}
	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			key, ok := keyToken.(string)
			if keyErr != nil || !ok {
				return false
			}
			if _, duplicate := keys[key]; duplicate {
				return false
			}
			keys[key] = struct{}{}
			if !consumeGatewayJSONValue(decoder) {
				return false
			}
		}
		closing, closeErr := decoder.Token()
		return closeErr == nil && closing == json.Delim('}')
	case '[':
		for decoder.More() {
			if !consumeGatewayJSONValue(decoder) {
				return false
			}
		}
		closing, closeErr := decoder.Token()
		return closeErr == nil && closing == json.Delim(']')
	default:
		return false
	}
}

func validOpenAIAttemptTool(
	toolType string,
	namespace string,
	name string,
	allowed map[string]struct{},
) bool {
	if toolType != "function" {
		return false
	}
	return validOpenAIFunction(namespace, name, allowed)
}

func validOpenAIFunction(
	namespace string,
	name string,
	allowed map[string]struct{},
) bool {
	if namespace == "mcp__loom_context" {
		_, ok := allowed[name]
		return ok
	}
	if namespace != "" {
		return false
	}
	if _, ok := allowed[name]; ok {
		return true
	}
	const prefix = "mcp__loom_context__"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	_, ok := allowed[strings.TrimPrefix(name, prefix)]
	return ok
}

func validAnthropicAttemptTool(name string, allowed map[string]struct{}) bool {
	const prefix = "mcp__loom_context__"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	_, ok := allowed[strings.TrimPrefix(name, prefix)]
	return ok
}

func gatewayProviderResponseUsesFrozenTools(
	providerID string,
	contentType string,
	body []byte,
	allowed map[string]struct{},
) bool {
	if len(body) == 0 {
		return true
	}
	if strings.HasPrefix(strings.ToLower(contentType), "text/event-stream") {
		foundData := false
		for _, line := range bytes.Split(body, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			foundData = true
			payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if bytes.Equal(payload, []byte("[DONE]")) {
				continue
			}
			if !gatewayProviderJSONUsesFrozenTools(providerID, payload, allowed) {
				return false
			}
		}
		return foundData
	}
	return gatewayProviderJSONUsesFrozenTools(providerID, body, allowed)
}

func gatewayProviderJSONUsesFrozenTools(
	providerID string,
	body []byte,
	allowed map[string]struct{},
) bool {
	if !gatewayJSONHasUniqueKeys(body) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	return gatewayProviderValueUsesFrozenTools(providerID, value, allowed)
}

func gatewayProviderValueUsesFrozenTools(
	providerID string,
	value any,
	allowed map[string]struct{},
) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if !gatewayProviderValueUsesFrozenTools(providerID, item, allowed) {
				return false
			}
		}
	case map[string]any:
		toolType, _ := typed["type"].(string)
		name, _ := typed["name"].(string)
		namespace, _ := typed["namespace"].(string)
		switch providerID {
		case CodexProviderID:
			if toolType == "function_call" {
				if !validOpenAIFunction(namespace, name, allowed) {
					return false
				}
			} else if strings.HasSuffix(toolType, "_call") {
				return false
			}
		case ClaudeCodeProviderID:
			if toolType == "tool_use" {
				if !validAnthropicAttemptTool(name, allowed) {
					return false
				}
			} else if strings.HasSuffix(toolType, "tool_use") {
				return false
			}
		default:
			return false
		}
		for _, item := range typed {
			if !gatewayProviderValueUsesFrozenTools(providerID, item, allowed) {
				return false
			}
		}
	}
	return true
}

func constantTimeEqual(candidate string, expected []byte) bool {
	return len(candidate) == len(expected) &&
		subtle.ConstantTimeCompare([]byte(candidate), expected) == 1
}

func constantTimeEqualWithPrefix(candidate, prefix string, expected []byte) bool {
	return strings.HasPrefix(candidate, prefix) &&
		constantTimeEqual(candidate[len(prefix):], expected)
}

func gatewayProviderFailure(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrHarnessProviderAuth
	case http.StatusTooManyRequests:
		return ErrHarnessProviderRateLimit
	default:
		if status >= 500 {
			return ErrHarnessProviderUnavailable
		}
		if status < 200 || status >= 300 {
			return ErrHarnessProviderRejected
		}
		return nil
	}
}

func harnessProviderTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.Is(err, context.DeadlineExceeded) ||
		errors.As(err, &timeout) && timeout.Timeout()
}

func (attempt *gatewayAttempt) setFailure(err error) {
	attempt.failureMu.Lock()
	defer attempt.failureMu.Unlock()
	if attempt.failure == nil {
		attempt.failure = err
	}
}

func (attempt *gatewayAttempt) currentFailure() error {
	attempt.failureMu.Lock()
	defer attempt.failureMu.Unlock()
	return attempt.failure
}

func copyGatewayRequestHeader(target, source http.Header, name string) {
	if value := source.Get(name); value != "" && !strings.ContainsAny(value, "\r\n\x00") {
		target.Set(name, value)
	}
}

func copyGatewayResponseHeader(target, source http.Header, name string) {
	if value := source.Get(name); value != "" && !strings.ContainsAny(value, "\r\n\x00") {
		target.Set(name, value)
	}
}

func writeGatewayError(writer http.ResponseWriter, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(`{"error":{"type":"loom_gateway_error"}}`))
}
