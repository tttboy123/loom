package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"loom-pi-rebuild/internal/prompting"
)

type anthropicConversationControlTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicConversationControlCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

type anthropicConversationControlMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicConversationControlRound struct {
	Content    []json.RawMessage
	Text       []string
	ToolCalls  []anthropicConversationControlCall
	StopReason string
}

type anthropicConversationControlChoice struct {
	Type string `json:"type"`
}

func (client *AnthropicConversationClient) RespondConfiguredWithTools(
	ctx context.Context,
	messages []ConversationMessage,
	secret []byte,
	modelID string,
	reasoningEffort string,
	tools []ConversationControlTool,
	execute ConversationControlExecutor,
) (string, error) {
	if client == nil || ctx == nil || execute == nil || len(messages) == 0 ||
		len(messages) > 64 || len(secret) == 0 || len(secret) > 8_192 ||
		reasoningEffort != "" {
		return "", ErrInvalidAnthropicConversation
	}
	byName, toolNames, preparedTools, err := prepareConversationControlTools(tools)
	if err != nil {
		return "", ErrInvalidAnthropicConversation
	}
	if modelID == "" {
		modelID = AnthropicConversationModelID
	}
	if _, err := ValidateConversationModel("anthropic", modelID); err != nil {
		return "", ErrInvalidAnthropicConversation
	}
	systemPrompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeConversation, ProviderID: "anthropic",
		ModelID: modelID, HarnessAdapter: "loom-native", ControlTools: toolNames,
	})
	if err != nil {
		return "", ErrInvalidAnthropicConversation
	}
	systemPrompt, messages, err = conversationSystemContext(systemPrompt, messages)
	if err != nil {
		return "", ErrInvalidAnthropicConversation
	}
	wireMessages := make([]anthropicConversationControlMessage, 0, len(messages)+4)
	totalBytes := len(systemPrompt)
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if (message.Role != "user" && message.Role != "assistant") ||
			content == "" || len(content) > maxConversationContentBytes ||
			!utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
			return "", ErrInvalidAnthropicConversation
		}
		totalBytes += len(content)
		if totalBytes > 96*1024 {
			return "", ErrInvalidAnthropicConversation
		}
		wireMessages = append(wireMessages, anthropicConversationControlMessage{
			Role: message.Role, Content: content,
		})
	}
	baseWireMessages := append([]anthropicConversationControlMessage(nil), wireMessages...)
	wireTools := make([]anthropicConversationControlTool, len(preparedTools))
	wireTerminalTools := make([]anthropicConversationControlTool, 0, len(preparedTools))
	wireToolByName := make(map[string]anthropicConversationControlTool, len(preparedTools))
	for index, tool := range preparedTools {
		wireTools[index] = anthropicConversationControlTool{
			Name: tool.Name, Description: tool.Description,
			InputSchema: append(json.RawMessage(nil), tool.InputSchema...),
		}
		if tool.StopAfterSuccess || tool.DirectReply {
			wireTerminalTools = append(wireTerminalTools, wireTools[index])
		}
		wireToolByName[tool.Name] = wireTools[index]
	}
	requestContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	totalCalls := 0
	seenCallIDs := make(map[string]struct{})
	productCalls := 0
	successfulNonTerminalCalls := 0
	directReplySelections := 0
	directAnswerSelected := false
	repairToolName := ""
	argumentRepairAttempts := 0
	rejectedRepairTools := make(map[string]struct{})
	availabilityCorrectionAttempts := 0
	availabilityCorrectionPending := false
	for round := 0; round <= MaxConversationControlCalls; round++ {
		roundTools := wireTools
		roundMessages := wireMessages
		roundSystemPrompt := systemPrompt
		argumentRepair := repairToolName != ""
		toolReselection := len(rejectedRepairTools) != 0
		availabilityCorrection := availabilityCorrectionPending
		availabilityCorrectionPending = false
		if toolReselection || availabilityCorrection && !argumentRepair {
			roundMessages = baseWireMessages
		}
		terminalArbitration := !argumentRepair && (successfulNonTerminalCalls >= 2 ||
			directReplySelections == 1 && productCalls > 0 || toolReselection)
		toolChoice := "auto"
		if round == 0 {
			toolChoice = "any"
		}
		if directReplySelections == 1 {
			roundTools = wireTools
			toolChoice = "any"
		}
		if terminalArbitration {
			roundTools = make([]anthropicConversationControlTool, 0, len(wireTerminalTools))
			for _, tool := range wireTerminalTools {
				if _, rejected := rejectedRepairTools[tool.Name]; !rejected {
					roundTools = append(roundTools, tool)
				}
			}
			toolChoice = "any"
		}
		if argumentRepair {
			repairTool, found := wireToolByName[repairToolName]
			if !found {
				return "", ErrInvalidAnthropicConversation
			}
			roundTools = []anthropicConversationControlTool{repairTool}
			toolChoice = "any"
		}
		if directAnswerSelected {
			roundTools = nil
			toolChoice = ""
			roundSystemPrompt += "\n\n" + conversationDirectAnswerInstruction
		} else {
			if productCalls > 0 {
				roundSystemPrompt += "\n\n" +
					conversationControlContinuationInstruction
			}
			if terminalArbitration {
				roundSystemPrompt += "\n\n" +
					conversationTerminalArbitrationInstruction
			}
			if toolReselection {
				roundSystemPrompt += "\n\n" +
					conversationToolReselectionInstruction
			}
			if availabilityCorrection {
				roundSystemPrompt += "\n\n" +
					conversationToolAvailabilityInstruction
			}
			if argumentRepair {
				roundSystemPrompt += "\n\n" +
					conversationArgumentRepairInstruction
			}
			if directReplySelections == 1 {
				roundSystemPrompt += "\n\n" +
					conversationDirectReplyRecheckInstruction
			}
		}
		roundAllowedTools := make(map[string]struct{}, len(roundTools))
		for _, tool := range roundTools {
			roundAllowedTools[tool.Name] = struct{}{}
		}
		response, err := client.anthropicConversationControlRound(
			requestContext, roundSystemPrompt, roundMessages, roundTools, toolChoice,
			secret, modelID,
		)
		if err != nil {
			return "", err
		}
		if len(response.ToolCalls) == 0 {
			if toolChoice == "any" {
				return "", invalidConversationHTTPResponse(
					http.StatusOK, "response_required_tool_choice",
				)
			}
			if response.StopReason != "end_turn" && response.StopReason != "max_tokens" &&
				response.StopReason != "stop_sequence" || len(response.Text) == 0 {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_content")
			}
			content := strings.TrimSpace(strings.Join(response.Text, "\n\n"))
			if content == "" || len(content) > maxConversationContentBytes {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_content")
			}
			return content, nil
		}
		if directAnswerSelected {
			return "", invalidConversationHTTPResponse(
				http.StatusOK, "response_direct_reply_continuation",
			)
		}
		if response.StopReason != "tool_use" ||
			len(response.ToolCalls) == 0 {
			return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_calls")
		}
		response.ToolCalls, err = selectAnthropicTerminalConversationControlCall(
			response.ToolCalls, byName,
		)
		if err != nil {
			return "", err
		}
		if totalCalls+len(response.ToolCalls) > MaxConversationControlCalls {
			return "", invalidConversationHTTPResponse(
				http.StatusOK, "response_tool_calls_limit",
			)
		}
		omittedFrozenTool := false
		for _, call := range response.ToolCalls {
			if _, allowed := roundAllowedTools[call.Name]; allowed {
				continue
			}
			if _, frozen := byName[call.Name]; !frozen {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_name")
			}
			omittedFrozenTool = true
		}
		if omittedFrozenTool {
			availabilityCorrectionAttempts++
			if availabilityCorrectionAttempts >= maxConversationControlAvailabilityAttempts {
				return "", conversationControlAvailabilityLimitFailure()
			}
			availabilityCorrectionPending = true
			continue
		}
		if len(response.ToolCalls) == 1 {
			call := response.ToolCalls[0]
			tool, allowed := byName[call.Name]
			if allowed && tool.DirectReply {
				if !validConversationControlCallID(call.ID) {
					return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
				}
				if _, duplicate := seenCallIDs[call.ID]; duplicate {
					return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
				}
				arguments, err := validateConversationControlArguments(call.Input)
				if err != nil || !validConversationDirectReplyArguments(arguments) {
					zeroConversationBytes(arguments)
					return "", invalidConversationHTTPResponse(
						http.StatusOK, "response_direct_reply_arguments",
					)
				}
				zeroConversationBytes(arguments)
				seenCallIDs[call.ID] = struct{}{}
				directReplySelections++
				totalCalls++
				if directReplySelections >= 2 {
					directAnswerSelected = true
				}
				continue
			}
		}
		wireMessages = append(wireMessages, anthropicConversationControlMessage{
			Role: "assistant", Content: response.Content,
		})
		resultBlocks := make([]map[string]any, 0, len(response.ToolCalls))
		for _, call := range response.ToolCalls {
			if !validConversationControlCallID(call.ID) {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
			}
			if _, duplicate := seenCallIDs[call.ID]; duplicate {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
			}
			seenCallIDs[call.ID] = struct{}{}
			tool, allowed := byName[call.Name]
			if !allowed {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_name")
			}
			arguments, err := validateConversationControlArguments(call.Input)
			if err != nil {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_arguments")
			}
			if tool.DirectReply {
				zeroConversationBytes(arguments)
				return "", invalidConversationHTTPResponse(
					http.StatusOK, "response_direct_reply_tool_calls",
				)
			}
			result, callErr := execute(requestContext, call.Name, arguments)
			zeroConversationBytes(arguments)
			isError := false
			toolSucceeded := callErr == nil
			if callErr != nil {
				var recoverable bool
				result, recoverable = conversationControlCorrectionResult(callErr, tool)
				if !recoverable {
					return "", conversationControlExecutionFailure(callErr)
				}
				if repairToolName != "" && repairToolName != call.Name {
					zeroConversationBytes(result)
					return "", conversationControlArgumentRepairLimitFailure()
				}
				repairToolName = call.Name
				argumentRepairAttempts++
				if argumentRepairAttempts >= maxConversationControlArgumentAttempts {
					rejectedRepairTools[call.Name] = struct{}{}
					repairToolName = ""
					argumentRepairAttempts = 0
				}
				isError = true
			} else {
				result, err = validateConversationControlResult(result)
				if err != nil {
					return "", conversationControlResultFailure()
				}
				if repairToolName == call.Name {
					repairToolName = ""
					argumentRepairAttempts = 0
				}
			}
			if toolSucceeded && !byName[call.Name].StopAfterSuccess {
				successfulNonTerminalCalls++
			}
			if toolSucceeded && byName[call.Name].StopAfterSuccess {
				zeroConversationBytes(result)
				return "", nil
			}
			resultBlock := map[string]any{
				"type": "tool_result", "tool_use_id": call.ID,
				"content": string(result),
			}
			if isError {
				resultBlock["is_error"] = true
			}
			resultBlocks = append(resultBlocks, resultBlock)
			zeroConversationBytes(result)
			totalCalls++
			productCalls++
		}
		wireMessages = append(wireMessages, anthropicConversationControlMessage{
			Role: "user", Content: resultBlocks,
		})
	}
	return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_limit")
}

func selectAnthropicTerminalConversationControlCall(
	calls []anthropicConversationControlCall,
	byName map[string]ConversationControlTool,
) ([]anthropicConversationControlCall, error) {
	terminal := -1
	for index, call := range calls {
		tool, found := byName[call.Name]
		if !found || !tool.StopAfterSuccess {
			continue
		}
		if terminal >= 0 {
			return nil, invalidConversationHTTPResponse(
				http.StatusOK, "response_terminal_tool_calls",
			)
		}
		terminal = index
	}
	if terminal < 0 {
		directReplies := 0
		for _, call := range calls {
			if tool, found := byName[call.Name]; found && tool.DirectReply {
				directReplies++
			}
		}
		if directReplies > 0 && (directReplies != 1 || len(calls) != 1) {
			return nil, invalidConversationHTTPResponse(
				http.StatusOK, "response_direct_reply_tool_calls",
			)
		}
		return calls, nil
	}
	return []anthropicConversationControlCall{calls[terminal]}, nil
}

func (client *AnthropicConversationClient) anthropicConversationControlRound(
	ctx context.Context,
	systemPrompt string,
	messages []anthropicConversationControlMessage,
	tools []anthropicConversationControlTool,
	toolChoice string,
	secret []byte,
	modelID string,
) (anthropicConversationControlRound, error) {
	var choice *anthropicConversationControlChoice
	if toolChoice != "" {
		choice = &anthropicConversationControlChoice{Type: toolChoice}
	}
	if toolChoice != "" && toolChoice != "auto" && toolChoice != "any" ||
		toolChoice == "any" && len(tools) == 0 ||
		toolChoice == "" && len(tools) != 0 {
		return anthropicConversationControlRound{}, ErrInvalidAnthropicConversation
	}
	payload, err := json.Marshal(struct {
		Model      string                                `json:"model"`
		System     string                                `json:"system"`
		Messages   []anthropicConversationControlMessage `json:"messages"`
		Tools      []anthropicConversationControlTool    `json:"tools,omitempty"`
		ToolChoice *anthropicConversationControlChoice   `json:"tool_choice,omitempty"`
		MaxTokens  int                                   `json:"max_tokens"`
	}{
		Model: modelID, System: systemPrompt, Messages: messages,
		Tools: tools, ToolChoice: choice, MaxTokens: 2_048,
	})
	if err != nil || len(payload) > 256*1024 {
		return anthropicConversationControlRound{}, ErrInvalidAnthropicConversation
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, AnthropicConversationEndpoint, bytes.NewReader(payload),
	)
	if err != nil || !exactConversationURL(request.URL, AnthropicConversationEndpoint) {
		return anthropicConversationControlRound{}, ErrInvalidAnthropicConversation
	}
	request.Header.Set("x-api-key", string(secret))
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, requestErr := client.client.Do(request)
	request.Header.Del("x-api-key")
	if requestErr != nil {
		return anthropicConversationControlRound{}, classifyConversationTransportFailure(requestErr)
	}
	if response == nil || response.Body == nil {
		return anthropicConversationControlRound{}, newConversationFailure(
			"provider_http", "state_unavailable", true,
		)
	}
	if response.Request != nil &&
		!exactConversationURL(response.Request.URL, AnthropicConversationEndpoint) {
		_ = response.Body.Close()
		return anthropicConversationControlRound{}, newConversationFailure(
			"provider_http", "provider_rejected", false,
		)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, client.maxResponseBytes+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || int64(len(body)) > client.maxResponseBytes {
		return anthropicConversationControlRound{}, newConversationFailure(
			"provider_http", "state_unavailable", true,
		)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return anthropicConversationControlRound{}, classifyConversationHTTPFailure(
			response.StatusCode, response.Header, body,
		)
	}
	var decoded struct {
		Type       string            `json:"type"`
		Role       string            `json:"role"`
		Model      string            `json:"model"`
		StopReason string            `json:"stop_reason"`
		Content    []json.RawMessage `json:"content"`
	}
	if rejectConversationDuplicateJSONKeys(body) ||
		json.Unmarshal(body, &decoded) != nil || decoded.Type != "message" ||
		decoded.Role != "assistant" || decoded.Model != modelID || len(decoded.Content) == 0 {
		return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
			response.StatusCode, "response_json",
		)
	}
	result := anthropicConversationControlRound{
		Content:    make([]json.RawMessage, 0, len(decoded.Content)),
		StopReason: decoded.StopReason,
	}
	for _, raw := range decoded.Content {
		if rejectConversationDuplicateJSONKeys(raw) {
			return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
				response.StatusCode, "response_content",
			)
		}
		var block struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if json.Unmarshal(raw, &block) != nil {
			return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
				response.StatusCode, "response_content",
			)
		}
		switch block.Type {
		case "text":
			text := strings.TrimSpace(block.Text)
			if text == "" || len(text) > maxConversationContentBytes ||
				!utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 ||
				block.ID != "" || block.Name != "" || len(block.Input) != 0 {
				return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
					response.StatusCode, "response_content",
				)
			}
			result.Text = append(result.Text, text)
		case "tool_use":
			if block.Text != "" || !validConversationControlCallID(block.ID) ||
				!validConversationControlName(block.Name) || len(block.Input) == 0 {
				return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
					response.StatusCode, "response_tool_call",
				)
			}
			result.ToolCalls = append(result.ToolCalls, anthropicConversationControlCall{
				ID: block.ID, Name: block.Name, Input: append(json.RawMessage(nil), block.Input...),
			})
		default:
			return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
				response.StatusCode, "response_content",
			)
		}
		result.Content = append(result.Content, append(json.RawMessage(nil), raw...))
	}
	if len(result.ToolCalls) != 0 && len(result.Text) != 0 {
		return anthropicConversationControlRound{}, invalidConversationHTTPResponse(
			response.StatusCode, "response_content",
		)
	}
	return result, nil
}
