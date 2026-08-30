package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"
)

const (
	harnessControlCallFailureInputAdmission     = "input_admission"
	harnessControlCallFailureRequestEncoding    = "request_encoding"
	harnessControlCallFailureTransport          = "transport"
	harnessControlCallFailureInvalidRequest     = "invalid_request"
	harnessControlCallFailureTurnUnavailable    = "turn_unavailable"
	harnessControlCallFailureCallLimit          = "call_limit"
	harnessControlCallFailureToolUnavailable    = "tool_unavailable"
	harnessControlCallFailureResponseValidation = "response_validation"
)

type harnessControlCallFailure struct {
	code string
}

func (failure *harnessControlCallFailure) Error() string {
	return ErrHarnessControlMCP.Error()
}

func (failure *harnessControlCallFailure) Unwrap() error {
	return ErrHarnessControlMCP
}

func (failure *harnessControlCallFailure) ConversationControlFailureCode() string {
	if failure == nil || !validHarnessControlCallFailureCode(failure.code) {
		return ""
	}
	return failure.code
}

func newHarnessControlCallFailure(code string) error {
	if !validHarnessControlCallFailureCode(code) {
		return ErrHarnessControlMCP
	}
	return &harnessControlCallFailure{code: code}
}

func HarnessControlCallFailureCode(err error) string {
	var failure *harnessControlCallFailure
	if !errors.As(err, &failure) || failure == nil ||
		!validHarnessControlCallFailureCode(failure.code) {
		return ""
	}
	return failure.code
}

func validHarnessControlCallFailureCode(code string) bool {
	switch code {
	case harnessControlCallFailureInputAdmission,
		harnessControlCallFailureRequestEncoding,
		harnessControlCallFailureTransport,
		harnessControlCallFailureInvalidRequest,
		harnessControlCallFailureTurnUnavailable,
		harnessControlCallFailureCallLimit,
		harnessControlCallFailureToolUnavailable,
		harnessControlCallFailureResponseValidation:
		return true
	default:
		return false
	}
}

// CallHarnessControlTool invokes one exact tool through the private turn-scoped
// MCP lease. It intentionally follows the same authenticated server path used
// by subprocess harnesses so call limits and Proposal capture cannot diverge.
func CallHarnessControlTool(
	ctx context.Context,
	lease HarnessControlMCPLease,
	name string,
	arguments json.RawMessage,
) (json.RawMessage, error) {
	if ctx == nil || ctx.Err() != nil || !validHarnessControlMCPLease(lease) ||
		!validHarnessControlToolName(name) || !harnessControlLeaseContains(lease, name) ||
		!validHarnessControlArguments(arguments) {
		return nil, newHarnessControlCallFailure(harnessControlCallFailureInputAdmission)
	}
	payload, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"params"`
	}{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}{Name: name, Arguments: arguments},
	})
	if err != nil || len(payload) > harnessControlMCPMaxRequest {
		zeroHarnessBytes(payload)
		return nil, newHarnessControlCallFailure(harnessControlCallFailureRequestEncoding)
	}
	defer zeroHarnessBytes(payload)
	body, err := performHarnessControlRPC(ctx, lease, payload)
	if err != nil {
		return nil, err
	}
	defer zeroHarnessBytes(body)
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent json.RawMessage `json:"structuredContent"`
			IsError           bool            `json:"isError"`
		} `json:"result,omitempty"`
		Error json.RawMessage `json:"error,omitempty"`
	}
	if decoder.Decode(&envelope) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		envelope.JSONRPC != "2.0" || string(bytes.TrimSpace(envelope.ID)) != "1" {
		return nil, newHarnessControlCallFailure(harnessControlCallFailureResponseValidation)
	}
	if len(envelope.Error) != 0 {
		if envelope.Result != nil {
			return nil, newHarnessControlCallFailure(harnessControlCallFailureResponseValidation)
		}
		var rpcError struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		errorDecoder := json.NewDecoder(bytes.NewReader(envelope.Error))
		errorDecoder.DisallowUnknownFields()
		if errorDecoder.Decode(&rpcError) != nil ||
			errorDecoder.Decode(&struct{}{}) != io.EOF {
			return nil, newHarnessControlCallFailure(harnessControlCallFailureResponseValidation)
		}
		for _, candidate := range []struct {
			code    int
			message string
			failure string
		}{
			{-32602, "invalid_request", harnessControlCallFailureInvalidRequest},
			{-32001, "turn_unavailable", harnessControlCallFailureTurnUnavailable},
			{-32002, "call_limit", harnessControlCallFailureCallLimit},
			{-32003, "tool_unavailable", harnessControlCallFailureToolUnavailable},
		} {
			if rpcError.Code == candidate.code && rpcError.Message == candidate.message {
				return nil, newHarnessControlCallFailure(candidate.failure)
			}
		}
		return nil, newHarnessControlCallFailure(harnessControlCallFailureResponseValidation)
	}
	if envelope.Result == nil || envelope.Result.IsError ||
		len(envelope.Result.Content) != 1 || envelope.Result.Content[0].Type != "text" ||
		!validHarnessControlResult(envelope.Result.Content[0].Text, envelope.Result.StructuredContent) {
		return nil, newHarnessControlCallFailure(harnessControlCallFailureResponseValidation)
	}
	return append(json.RawMessage(nil), envelope.Result.StructuredContent...), nil
}

// VerifyHarnessControlToolCompletions requires the authenticated turn-scoped
// server to corroborate the exact successful direct tool calls reported by a
// subprocess. Process stdout alone is never execution evidence.
func VerifyHarnessControlToolCompletions(
	ctx context.Context,
	lease HarnessControlMCPLease,
	expected []string,
) error {
	if ctx == nil || ctx.Err() != nil || !validHarnessControlMCPLease(lease) ||
		len(expected) == 0 || len(expected) > harnessControlMCPMaxCalls {
		return ErrHarnessControlMCP
	}
	for _, name := range expected {
		if !validHarnessControlToolName(name) || !harnessControlLeaseContains(lease, name) {
			return ErrHarnessControlMCP
		}
	}
	payload, err := json.Marshal(struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      int            `json:"id"`
		Method  string         `json:"method"`
		Params  map[string]any `json:"params"`
	}{JSONRPC: "2.0", ID: 2, Method: harnessControlMCPCompletedMethod, Params: map[string]any{}})
	if err != nil || len(payload) > harnessControlMCPMaxRequest {
		zeroHarnessBytes(payload)
		return ErrHarnessControlMCP
	}
	defer zeroHarnessBytes(payload)
	body, err := performHarnessControlRPC(ctx, lease, payload)
	if err != nil {
		return err
	}
	defer zeroHarnessBytes(body)
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  *struct {
			ToolNames []string `json:"tool_names"`
		} `json:"result,omitempty"`
		Error json.RawMessage `json:"error,omitempty"`
	}
	if decoder.Decode(&envelope) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		envelope.JSONRPC != "2.0" || string(bytes.TrimSpace(envelope.ID)) != "2" ||
		envelope.Result == nil || len(envelope.Error) != 0 ||
		len(envelope.Result.ToolNames) != len(expected) {
		return ErrHarnessControlMCP
	}
	for index, name := range expected {
		if envelope.Result.ToolNames[index] != name {
			return ErrHarnessControlMCP
		}
	}
	return nil
}

// AcknowledgeHarnessControlToolCompletion tells the turn-scoped server that a
// Harness has consumed the preceding Tool response. It carries no arguments or
// result content. A true result means the completed Tool was a terminal
// proposal and the Harness may be stopped after this acknowledgement.
func AcknowledgeHarnessControlToolCompletion(
	ctx context.Context,
	lease HarnessControlMCPLease,
) (bool, error) {
	if ctx == nil || ctx.Err() != nil || !validHarnessControlMCPLease(lease) {
		return false, ErrHarnessControlMCP
	}
	payload, err := json.Marshal(struct {
		JSONRPC string         `json:"jsonrpc"`
		ID      int            `json:"id"`
		Method  string         `json:"method"`
		Params  map[string]any `json:"params"`
	}{JSONRPC: "2.0", ID: 3, Method: harnessControlMCPAckMethod, Params: map[string]any{}})
	if err != nil || len(payload) > harnessControlMCPMaxRequest {
		zeroHarnessBytes(payload)
		return false, ErrHarnessControlMCP
	}
	defer zeroHarnessBytes(payload)
	body, err := performHarnessControlRPC(ctx, lease, payload)
	if err != nil {
		return false, err
	}
	defer zeroHarnessBytes(body)
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  *struct {
			Terminal *bool `json:"terminal"`
		} `json:"result,omitempty"`
		Error json.RawMessage `json:"error,omitempty"`
	}
	if decoder.Decode(&envelope) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		envelope.JSONRPC != "2.0" || string(bytes.TrimSpace(envelope.ID)) != "3" ||
		envelope.Result == nil || envelope.Result.Terminal == nil || len(envelope.Error) != 0 {
		return false, ErrHarnessControlMCP
	}
	return *envelope.Result.Terminal, nil
}

func performHarnessControlRPC(
	ctx context.Context,
	lease HarnessControlMCPLease,
	payload []byte,
) ([]byte, error) {
	endpoint, err := url.Parse(lease.URL)
	if err != nil || !validHarnessControlLoopbackURL(endpoint) {
		return nil, newHarnessControlCallFailure(harnessControlCallFailureTransport)
	}
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext, http.MethodPost, endpoint.String(), bytes.NewReader(payload),
	)
	if err != nil {
		return nil, newHarnessControlCallFailure(harnessControlCallFailureTransport)
	}
	request.Header.Set("Authorization", "Bearer "+lease.Token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true,
		ForceAttemptHTTP2: false,
		DialContext: func(dialContext context.Context, _ string, address string) (net.Conn, error) {
			if address != endpoint.Host {
				return nil, newHarnessControlCallFailure(harnessControlCallFailureTransport)
			}
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(
				dialContext, "tcp4", endpoint.Host,
			)
		},
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	request.Header.Del("Authorization")
	transport.CloseIdleConnections()
	if err != nil || response == nil || response.Body == nil {
		return nil, newHarnessControlCallFailure(harnessControlCallFailureTransport)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, harnessControlMCPMaxResponse+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK ||
		len(body) == 0 || len(body) > harnessControlMCPMaxResponse ||
		!utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 ||
		rejectHarnessDuplicateJSONKeys(body) {
		zeroHarnessBytes(body)
		return nil, newHarnessControlCallFailure(harnessControlCallFailureTransport)
	}
	return body, nil
}

func harnessControlLeaseContains(lease HarnessControlMCPLease, name string) bool {
	for _, allowed := range lease.ToolNames {
		if allowed == name {
			return true
		}
	}
	return false
}

func validHarnessControlArguments(arguments json.RawMessage) bool {
	if len(arguments) == 0 || len(arguments) > harnessControlMCPMaxRequest ||
		!utf8.Valid(arguments) || bytes.IndexByte(arguments, 0) >= 0 ||
		rejectHarnessDuplicateJSONKeys(arguments) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.UseNumber()
	var object map[string]any
	return decoder.Decode(&object) == nil && object != nil &&
		decoder.Decode(&struct{}{}) == io.EOF
}

func validHarnessControlResult(text string, structured json.RawMessage) bool {
	if text == "" || len(text) > harnessControlMCPMaxResponse ||
		len(structured) == 0 || len(structured) > harnessControlMCPMaxResponse ||
		!utf8.ValidString(text) || !utf8.Valid(structured) ||
		bytes.IndexByte([]byte(text), 0) >= 0 || bytes.IndexByte(structured, 0) >= 0 ||
		rejectHarnessDuplicateJSONKeys([]byte(text)) ||
		rejectHarnessDuplicateJSONKeys(structured) {
		return false
	}
	var textValue any
	var structuredValue any
	textDecoder := json.NewDecoder(bytes.NewBufferString(text))
	textDecoder.UseNumber()
	structuredDecoder := json.NewDecoder(bytes.NewReader(structured))
	structuredDecoder.UseNumber()
	if textDecoder.Decode(&textValue) != nil || textDecoder.Decode(&struct{}{}) != io.EOF ||
		structuredDecoder.Decode(&structuredValue) != nil ||
		structuredDecoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	textCanonical, textErr := json.Marshal(textValue)
	structuredCanonical, structuredErr := json.Marshal(structuredValue)
	return textErr == nil && structuredErr == nil &&
		bytes.Equal(textCanonical, structuredCanonical)
}

func validHarnessControlLoopbackURL(endpoint *url.URL) bool {
	if endpoint == nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" ||
		endpoint.Path != "/mcp" || endpoint.RawPath != "" || endpoint.RawQuery != "" ||
		endpoint.Fragment != "" || endpoint.User != nil || endpoint.Opaque != "" {
		return false
	}
	port, err := strconv.Atoi(endpoint.Port())
	return err == nil && port > 0 && port <= 65535
}
