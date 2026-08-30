package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"loom-pi-rebuild/internal/prompting"
)

type openAIConversationControlFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIConversationControlCall struct {
	ID       string                            `json:"id"`
	Type     string                            `json:"type"`
	Function openAIConversationControlFunction `json:"function"`
}

type openAIConversationControlMessage struct {
	Role       string                          `json:"role"`
	Content    any                             `json:"content"`
	ToolCalls  []openAIConversationControlCall `json:"tool_calls,omitempty"`
	ToolCallID string                          `json:"tool_call_id,omitempty"`
}

type openAIConversationControlTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func (client *DeepSeekConversationClient) RespondConfiguredWithTools(
	ctx context.Context,
	messages []ConversationMessage,
	secret []byte,
	modelID string,
	reasoningEffort string,
	tools []ConversationControlTool,
	execute ConversationControlExecutor,
) (string, error) {
	if client == nil || ctx == nil || execute == nil || len(messages) == 0 ||
		len(messages) > 64 || len(secret) == 0 || len(secret) > 8_192 {
		return "", ErrInvalidOpenAICompatibleConversation
	}
	byName, toolNames, preparedTools, err := prepareConversationControlTools(tools)
	if err != nil {
		return "", ErrInvalidOpenAICompatibleConversation
	}
	if modelID == "" {
		modelID = client.modelID
	}
	model, err := ValidateConversationModel(client.providerID, modelID)
	if err != nil {
		return "", ErrInvalidOpenAICompatibleConversation
	}
	if reasoningEffort != "" {
		if err := ValidateConversationReasoningEffort(model, reasoningEffort); err != nil {
			return "", ErrInvalidOpenAICompatibleConversation
		}
	}
	systemPrompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeConversation, ProviderID: client.providerID,
		ModelID: modelID, HarnessAdapter: "loom-native", ControlTools: toolNames,
	})
	if err != nil {
		return "", ErrInvalidOpenAICompatibleConversation
	}
	systemPrompt, messages, err = conversationSystemContext(systemPrompt, messages)
	if err != nil {
		return "", ErrInvalidOpenAICompatibleConversation
	}
	wireMessages := []openAIConversationControlMessage{{
		Role: "system", Content: systemPrompt,
	}}
	totalBytes := len(systemPrompt)
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if (message.Role != "user" && message.Role != "assistant") ||
			content == "" || len(content) > maxConversationContentBytes ||
			!utf8.ValidString(content) || strings.IndexByte(content, 0) >= 0 {
			return "", ErrInvalidOpenAICompatibleConversation
		}
		totalBytes += len(content)
		if totalBytes > 96*1024 {
			return "", ErrInvalidOpenAICompatibleConversation
		}
		wireMessages = append(wireMessages, openAIConversationControlMessage{
			Role: message.Role, Content: content,
		})
	}
	baseWireMessages := append([]openAIConversationControlMessage(nil), wireMessages...)
	wireTools := make([]openAIConversationControlTool, len(preparedTools))
	wireTerminalTools := make([]openAIConversationControlTool, 0, len(preparedTools))
	wireToolByName := make(map[string]openAIConversationControlTool, len(preparedTools))
	for index, tool := range preparedTools {
		wireTools[index].Type = "function"
		wireTools[index].Function.Name = tool.Name
		wireTools[index].Function.Description = tool.Description
		wireTools[index].Function.Parameters = append(json.RawMessage(nil), tool.InputSchema...)
		if tool.StopAfterSuccess || tool.DirectReply {
			wireTerminalTools = append(wireTerminalTools, wireTools[index])
		}
		wireToolByName[tool.Name] = wireTools[index]
	}
	loopTimeout := client.timeout
	if client.providerID == "minimax" {
		// MiniMax may ignore the first required Tool selection. Keep each HTTP
		// round bounded while reserving exactly one additional round budget for
		// the typed correction; the caller's context remains the outer limit.
		loopTimeout += client.timeout
	}
	requestContext, cancel := context.WithTimeout(ctx, loopTimeout)
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
	requiredSelectionCorrectionPending := false
	for round := 0; round <= MaxConversationControlCalls; round++ {
		roundTools := wireTools
		roundMessages := wireMessages
		roundSystemPrompt := systemPrompt
		argumentRepair := repairToolName != ""
		toolReselection := len(rejectedRepairTools) != 0
		availabilityCorrection := availabilityCorrectionPending
		availabilityCorrectionPending = false
		requiredSelectionCorrection := requiredSelectionCorrectionPending
		requiredSelectionCorrectionPending = false
		if toolReselection || (availabilityCorrection || requiredSelectionCorrection) && !argumentRepair {
			roundMessages = baseWireMessages
		}
		terminalArbitration := !argumentRepair && (successfulNonTerminalCalls >= 2 ||
			directReplySelections == 1 && productCalls > 0 || toolReselection)
		toolChoice := "auto"
		if round == 0 {
			toolChoice = "required"
		}
		if directReplySelections == 1 {
			roundTools = wireTools
			toolChoice = "required"
		}
		if terminalArbitration {
			roundTools = make([]openAIConversationControlTool, 0, len(wireTerminalTools))
			for _, tool := range wireTerminalTools {
				if _, rejected := rejectedRepairTools[tool.Function.Name]; !rejected {
					roundTools = append(roundTools, tool)
				}
			}
			toolChoice = "required"
		}
		if argumentRepair {
			repairTool, found := wireToolByName[repairToolName]
			if !found {
				return "", ErrInvalidOpenAICompatibleConversation
			}
			roundTools = []openAIConversationControlTool{repairTool}
			toolChoice = "required"
		}
		if requiredSelectionCorrection {
			toolChoice = "required"
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
			if requiredSelectionCorrection {
				roundSystemPrompt += "\n\n" +
					conversationRequiredToolSelectionInstruction
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
		if roundSystemPrompt != systemPrompt {
			roundMessages = append([]openAIConversationControlMessage(nil), roundMessages...)
			roundMessages[0].Content = roundSystemPrompt
		}
		roundAllowedTools := make(map[string]struct{}, len(roundTools))
		for _, tool := range roundTools {
			roundAllowedTools[tool.Function.Name] = struct{}{}
		}
		roundContext, cancelRound := context.WithTimeout(requestContext, client.timeout)
		response, err := client.openAIConversationControlRound(
			roundContext, roundMessages, roundTools, toolChoice,
			secret, modelID, reasoningEffort,
		)
		cancelRound()
		if err != nil {
			return "", err
		}
		if len(response.ToolCalls) == 0 {
			if toolChoice == "required" {
				if round == 0 {
					requiredSelectionCorrectionPending = true
					continue
				}
				return "", invalidConversationHTTPResponse(
					http.StatusOK, "response_required_tool_choice",
				)
			}
			rawContent := strings.TrimSpace(response.Content)
			if rawContent == "" || len(rawContent) > maxConversationContentBytes ||
				!utf8.ValidString(rawContent) || strings.IndexByte(rawContent, 0) >= 0 {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_content")
			}
			content, visible := NormalizeVisibleResponseContent(client.providerID, rawContent)
			if !visible || content == "" {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_content")
			}
			return content, nil
		}
		if directAnswerSelected {
			return "", invalidConversationHTTPResponse(
				http.StatusOK, "response_direct_reply_continuation",
			)
		}
		if response.FinishReason != "tool_calls" {
			return "", invalidConversationHTTPResponse(
				http.StatusOK, "response_tool_calls_finish_reason",
			)
		}
		response.ToolCalls, err = selectOpenAITerminalConversationControlCall(
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
			if _, allowed := roundAllowedTools[call.Function.Name]; allowed {
				continue
			}
			if _, frozen := byName[call.Function.Name]; !frozen {
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
			tool, allowed := byName[call.Function.Name]
			if allowed && tool.DirectReply {
				if call.Type != "function" || !validConversationControlCallID(call.ID) {
					return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
				}
				if _, duplicate := seenCallIDs[call.ID]; duplicate {
					return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
				}
				arguments, err := validateConversationControlArguments(
					json.RawMessage(call.Function.Arguments),
				)
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
		response.Content = ""
		wireMessages = append(wireMessages, openAIConversationControlMessage{
			Role: "assistant", Content: nil, ToolCalls: response.ToolCalls,
		})
		for _, call := range response.ToolCalls {
			if call.Type != "function" || !validConversationControlCallID(call.ID) {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
			}
			if _, duplicate := seenCallIDs[call.ID]; duplicate {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_call")
			}
			seenCallIDs[call.ID] = struct{}{}
			tool, allowed := byName[call.Function.Name]
			if !allowed {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_name")
			}
			arguments, err := validateConversationControlArguments(
				json.RawMessage(call.Function.Arguments),
			)
			if err != nil {
				return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_arguments")
			}
			if tool.DirectReply {
				zeroConversationBytes(arguments)
				return "", invalidConversationHTTPResponse(
					http.StatusOK, "response_direct_reply_tool_calls",
				)
			}
			result, callErr := execute(requestContext, call.Function.Name, arguments)
			zeroConversationBytes(arguments)
			toolSucceeded := callErr == nil
			if callErr != nil {
				var recoverable bool
				result, recoverable = conversationControlCorrectionResult(callErr, tool)
				if !recoverable {
					return "", conversationControlExecutionFailure(callErr)
				}
				if repairToolName != "" && repairToolName != call.Function.Name {
					zeroConversationBytes(result)
					return "", conversationControlArgumentRepairLimitFailure()
				}
				repairToolName = call.Function.Name
				argumentRepairAttempts++
				if argumentRepairAttempts >= maxConversationControlArgumentAttempts {
					rejectedRepairTools[call.Function.Name] = struct{}{}
					repairToolName = ""
					argumentRepairAttempts = 0
				}
			} else {
				result, err = validateConversationControlResult(result)
				if err != nil {
					return "", conversationControlResultFailure()
				}
				if repairToolName == call.Function.Name {
					repairToolName = ""
					argumentRepairAttempts = 0
				}
			}
			if toolSucceeded && !byName[call.Function.Name].StopAfterSuccess {
				successfulNonTerminalCalls++
			}
			if toolSucceeded && byName[call.Function.Name].StopAfterSuccess {
				zeroConversationBytes(result)
				return "", nil
			}
			wireMessages = append(wireMessages, openAIConversationControlMessage{
				Role: "tool", Content: string(result), ToolCallID: call.ID,
			})
			zeroConversationBytes(result)
			totalCalls++
			productCalls++
		}
	}
	return "", invalidConversationHTTPResponse(http.StatusOK, "response_tool_limit")
}

func selectOpenAITerminalConversationControlCall(
	calls []openAIConversationControlCall,
	byName map[string]ConversationControlTool,
) ([]openAIConversationControlCall, error) {
	terminal := -1
	for index, call := range calls {
		tool, found := byName[call.Function.Name]
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
			if tool, found := byName[call.Function.Name]; found && tool.DirectReply {
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
	return []openAIConversationControlCall{calls[terminal]}, nil
}

type openAIConversationControlRound struct {
	Content      string
	ToolCalls    []openAIConversationControlCall
	FinishReason string
}

func (client *DeepSeekConversationClient) openAIConversationControlRound(
	ctx context.Context,
	messages []openAIConversationControlMessage,
	tools []openAIConversationControlTool,
	toolChoice string,
	secret []byte,
	modelID string,
	reasoningEffort string,
) (openAIConversationControlRound, error) {
	requestBody := struct {
		Model               string                             `json:"model"`
		Messages            []openAIConversationControlMessage `json:"messages"`
		Tools               []openAIConversationControlTool    `json:"tools,omitempty"`
		ToolChoice          string                             `json:"tool_choice,omitempty"`
		Stream              bool                               `json:"stream"`
		ReasoningEffort     string                             `json:"reasoning_effort,omitempty"`
		ReasoningSplit      bool                               `json:"reasoning_split,omitempty"`
		MaxTokens           int                                `json:"max_tokens,omitempty"`
		MaxCompletionTokens int                                `json:"max_completion_tokens,omitempty"`
		Temperature         *float64                           `json:"temperature,omitempty"`
		Seed                *int64                             `json:"seed,omitempty"`
	}{
		Model: modelID, Messages: messages, Tools: tools, ToolChoice: toolChoice,
		Stream: false, ReasoningEffort: reasoningEffort,
		ReasoningSplit: client.providerID == "minimax",
	}
	if client.providerID == "minimax" && toolChoice != "" {
		temperature := 0.0
		seed := miniMaxConversationControlSeed
		requestBody.Temperature = &temperature
		requestBody.Seed = &seed
	}
	if toolChoice != "" && toolChoice != "auto" && toolChoice != "required" ||
		toolChoice == "required" && len(tools) == 0 ||
		toolChoice == "" && len(tools) != 0 {
		return openAIConversationControlRound{}, ErrInvalidOpenAICompatibleConversation
	}
	completionTokens := defaultConversationCompletionTokens
	if reasoningEffort != "" || client.providerID == "minimax" {
		completionTokens = reasoningConversationCompletionTokens
	}
	if client.completionTokenField == "max_tokens" {
		requestBody.MaxTokens = completionTokens
	} else {
		requestBody.MaxCompletionTokens = completionTokens
	}
	payload, err := json.Marshal(requestBody)
	if err != nil || len(payload) > 256*1024 {
		return openAIConversationControlRound{}, ErrInvalidOpenAICompatibleConversation
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, client.endpoint, bytes.NewReader(payload),
	)
	if err != nil || !exactConversationURL(request.URL, client.endpoint) {
		return openAIConversationControlRound{}, ErrInvalidOpenAICompatibleConversation
	}
	request.Header.Set("Authorization", "Bearer "+string(secret))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, requestErr := client.client.Do(request)
	request.Header.Del("Authorization")
	if requestErr != nil {
		return openAIConversationControlRound{}, classifyConversationTransportFailure(requestErr)
	}
	if response == nil || response.Body == nil {
		return openAIConversationControlRound{}, newConversationFailure(
			"provider_http", "state_unavailable", true,
		)
	}
	if response.Request != nil && !exactConversationURL(response.Request.URL, client.endpoint) {
		_ = response.Body.Close()
		return openAIConversationControlRound{}, newConversationFailure(
			"provider_http", "provider_rejected", false,
		)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, client.maxResponseBytes+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || int64(len(body)) > client.maxResponseBytes {
		if errors.Is(readErr, context.DeadlineExceeded) || errors.Is(readErr, context.Canceled) {
			return openAIConversationControlRound{}, newConversationFailure(
				"provider_http", "timeout", true,
			)
		}
		return openAIConversationControlRound{}, newConversationFailure(
			"provider_http", "state_unavailable", true,
		)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return openAIConversationControlRound{}, classifyConversationHTTPFailure(
			response.StatusCode, response.Header, body,
		)
	}
	var decoded struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role      string                          `json:"role"`
				Content   json.RawMessage                 `json:"content"`
				ToolCalls []openAIConversationControlCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if rejectConversationDuplicateJSONKeys(body) ||
		json.Unmarshal(body, &decoded) != nil || !validConversationResponseModel(decoded.Model) ||
		len(decoded.Choices) != 1 || decoded.Choices[0].Message.Role != "assistant" {
		return openAIConversationControlRound{}, invalidConversationHTTPResponse(
			response.StatusCode, "response_json",
		)
	}
	content := ""
	rawContent := bytes.TrimSpace(decoded.Choices[0].Message.Content)
	if len(rawContent) != 0 && !bytes.Equal(rawContent, []byte("null")) {
		if json.Unmarshal(rawContent, &content) != nil {
			return openAIConversationControlRound{}, invalidConversationHTTPResponse(
				response.StatusCode, "response_content",
			)
		}
		content = strings.TrimSpace(content)
	}
	return openAIConversationControlRound{
		Content: content, ToolCalls: decoded.Choices[0].Message.ToolCalls,
		FinishReason: decoded.Choices[0].FinishReason,
	}, nil
}

func validConversationControlCallID(value string) bool {
	if value == "" || len(value) > 512 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '_' || character == '-' || character == '.' || character == ':' {
			continue
		}
		return false
	}
	return true
}

func zeroConversationBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
