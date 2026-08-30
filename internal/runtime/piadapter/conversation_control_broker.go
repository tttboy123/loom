//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/prompting"
)

const piConversationControlMCPMaximumBytes = 64 << 10

func (adapter *PiRPCConversationAdapter) resolveConversationControlTool(
	content string,
) (PiRPCConversationControlTool, error) {
	if adapter == nil {
		return PiRPCConversationControlTool{}, ErrPiConversationControl
	}
	return adapter.resolveConversationControlSelection(content, adapter.control)
}

func (adapter *PiRPCConversationAdapter) resolveConversationControlSelection(
	content string,
	config *PiRPCConversationControlConfig,
) (PiRPCConversationControlTool, error) {
	if adapter == nil || adapter.bridge == nil || config == nil ||
		!validPiConversationSelectionConfig(*config) ||
		content == "" || len(content) > adapter.bridge.maxAssistantBytes {
		return PiRPCConversationControlTool{}, ErrPiConversationControl
	}
	fields, err := piRPCObject(json.RawMessage(content))
	if err != nil || !piRPCExactKeys(fields, "tool_name") {
		return PiRPCConversationControlTool{}, piConversationControlSelectionFailure(
			"envelope", "", 0, 0,
		)
	}
	toolName, nameOK := piRPCString(fields, "tool_name")
	if !nameOK {
		return PiRPCConversationControlTool{}, piConversationControlSelectionFailure(
			"fields", toolName, 0, 0,
		)
	}
	if tool, ok := piConversationControlTool(config, toolName); ok {
		return tool, nil
	}
	return PiRPCConversationControlTool{}, piConversationControlSelectionFailure(
		"tool", toolName, 0, 0,
	)
}

func (adapter *PiRPCConversationAdapter) conversationControlTool(
	name string,
) (PiRPCConversationControlTool, bool) {
	if adapter == nil || adapter.control == nil {
		return PiRPCConversationControlTool{}, false
	}
	return piConversationControlTool(adapter.control, name)
}

func piConversationControlTool(
	config *PiRPCConversationControlConfig,
	name string,
) (PiRPCConversationControlTool, bool) {
	if config == nil {
		return PiRPCConversationControlTool{}, false
	}
	for _, tool := range config.Tools {
		if tool.Name == name {
			tool.InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
			return tool, true
		}
	}
	return PiRPCConversationControlTool{}, false
}

func (adapter *PiRPCConversationAdapter) resolveConversationControlArguments(
	ctx context.Context,
	tool PiRPCConversationControlTool,
	content string,
	explicitTargets piConversationExplicitArgumentTargets,
) (PiRPCConversationResponse, error) {
	if adapter == nil || adapter.bridge == nil || adapter.control == nil || ctx == nil ||
		tool.Name == prompting.ControlToolConversationReply ||
		content == "" || len(content) > piConversationControlMaxSchemaBytes {
		return PiRPCConversationResponse{}, ErrPiConversationControl
	}
	frozenTool, ok := adapter.conversationControlTool(tool.Name)
	if !ok || frozenTool.Name != tool.Name ||
		!bytes.Equal(frozenTool.InputSchema, tool.InputSchema) {
		return PiRPCConversationResponse{}, ErrPiConversationControl
	}
	arguments, err := piRPCObject(json.RawMessage(content))
	if err != nil {
		return PiRPCConversationResponse{}, piConversationControlSelectionFailure(
			"arguments", tool.Name, 0, 0,
		)
	}
	if err := validatePiConversationExplicitArgumentTargets(
		arguments, explicitTargets,
	); err != nil {
		return PiRPCConversationResponse{}, err
	}
	canonicalArguments, err := json.Marshal(arguments)
	if err != nil || len(canonicalArguments) == 0 ||
		len(canonicalArguments) > piConversationControlMaxSchemaBytes {
		return PiRPCConversationResponse{}, ErrPiConversationControl
	}
	defer zeroPiRPCBytes(canonicalArguments)
	result, err := adapter.callConversationControlMCP(
		ctx, tool.Name, canonicalArguments,
	)
	if err != nil {
		return PiRPCConversationResponse{}, err
	}
	defer zeroPiRPCBytes(result)
	if strings.HasSuffix(tool.Name, "_preview") {
		return PiRPCConversationResponse{
			Content: "I prepared a Loom proposal for your review.",
		}, nil
	}
	text, err := piConversationControlResultText(
		result, adapter.bridge.maxAssistantBytes,
	)
	if err != nil {
		return PiRPCConversationResponse{}, err
	}
	return PiRPCConversationResponse{Content: text}, nil
}

func piConversationControlSelectionFailure(
	reason string,
	toolName string,
	argumentFields int,
	answerBytes int,
) error {
	toolNameValid := validPiConversationControlName(toolName)
	return errors.Join(
		ErrPiConversationControl,
		fmt.Errorf(
			"selection_%s: tool_name_valid=%t argument_fields=%d answer_bytes=%d",
			reason, toolNameValid, argumentFields, answerBytes,
		),
	)
}

func (adapter *PiRPCConversationAdapter) callConversationControlMCP(
	ctx context.Context,
	toolName string,
	arguments json.RawMessage,
) ([]byte, error) {
	if adapter == nil || adapter.control == nil ||
		!validPreparedPiConversationControlConfig(*adapter.control) {
		return nil, ErrPiConversationControl
	}
	endpoint, err := url.Parse(adapter.control.URL)
	if err != nil || endpoint.String() != adapter.control.URL {
		return nil, ErrPiConversationControl
	}
	requestBody, err := json.Marshal(struct {
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
		}{Name: toolName, Arguments: arguments},
	})
	if err != nil || len(requestBody) == 0 ||
		len(requestBody) > piConversationControlMCPMaximumBytes {
		return nil, ErrPiConversationControl
	}
	defer zeroPiRPCBytes(requestBody)
	requestContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(
		requestContext, http.MethodPost, endpoint.String(), bytes.NewReader(requestBody),
	)
	if err != nil {
		return nil, ErrPiConversationControl
	}
	request.Header.Set("Authorization", "Bearer "+adapter.control.Token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true,
		ForceAttemptHTTP2: false,
		DialContext: func(dialContext context.Context, _ string, address string) (net.Conn, error) {
			if address != endpoint.Host {
				return nil, ErrPiConversationControl
			}
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(
				dialContext, "tcp4", endpoint.Host,
			)
		},
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	request.Header.Del("Authorization")
	transport.CloseIdleConnections()
	if err != nil || response == nil || response.Body == nil {
		return nil, ErrPiConversationControl
	}
	body, readErr := io.ReadAll(io.LimitReader(
		response.Body, piConversationControlMCPMaximumBytes+1,
	))
	closeErr := response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK ||
		closeErr != nil || len(body) == 0 ||
		len(body) > piConversationControlMCPMaximumBytes ||
		!utf8.Valid(body) || bytes.IndexByte(body, 0) >= 0 ||
		rejectPiRPCDuplicateKeys(body) != nil {
		zeroPiRPCBytes(body)
		return nil, ErrPiConversationControl
	}
	fields, parseErr := piRPCObject(body)
	if parseErr != nil || !piRPCExactKeys(fields, "jsonrpc", "id", "result") ||
		string(fields["jsonrpc"]) != `"2.0"` || string(fields["id"]) != "1" {
		zeroPiRPCBytes(body)
		return nil, ErrPiConversationControl
	}
	result, resultErr := piRPCObject(fields["result"])
	if resultErr != nil || !piRPCExactKeys(result, "content", "structuredContent", "isError") {
		zeroPiRPCBytes(body)
		return nil, ErrPiConversationControl
	}
	isError, ok := piRPCBool(result, "isError")
	if !ok || isError {
		zeroPiRPCBytes(body)
		return nil, ErrPiConversationControl
	}
	if _, structuredErr := piRPCObject(result["structuredContent"]); structuredErr != nil {
		zeroPiRPCBytes(body)
		return nil, ErrPiConversationControl
	}
	resultBytes := bytes.Clone(fields["result"])
	zeroPiRPCBytes(body)
	return resultBytes, nil
}

func piConversationControlResultText(
	result json.RawMessage,
	maximum int,
) (string, error) {
	fields, err := piRPCObject(result)
	if err != nil || maximum < 1 ||
		!piRPCExactKeys(fields, "content", "structuredContent", "isError") {
		return "", ErrPiConversationControl
	}
	var blocks []json.RawMessage
	if json.Unmarshal(fields["content"], &blocks) != nil ||
		len(blocks) == 0 || len(blocks) > 8 {
		return "", ErrPiConversationControl
	}
	parts := make([]string, 0, len(blocks))
	length := 0
	for _, raw := range blocks {
		block, blockErr := piRPCObject(raw)
		kind, kindOK := piRPCString(block, "type")
		text, textOK := piRPCString(block, "text")
		if blockErr != nil || !piRPCExactKeys(block, "type", "text") ||
			!kindOK || !textOK || kind != "text" || text == "" ||
			!validPiRPCConversationText(text) {
			return "", ErrPiConversationControl
		}
		length += len(text)
		if len(parts) != 0 {
			length++
		}
		if length > maximum {
			return "", ErrPiConversationControl
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n"), nil
}
