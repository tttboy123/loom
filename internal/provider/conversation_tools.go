package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/prompting"
)

const (
	MaxConversationControlCalls                  = 8
	maxConversationControlTools                  = 64
	maxConversationControlSchemaBytes            = 16 << 10
	maxConversationControlResultBytes            = 64 << 10
	maxConversationControlArgumentAttempts       = 3
	maxConversationControlAvailabilityAttempts   = 3
	conversationDirectReplyDescription           = "Choose this only when the latest request does not match any Loom product control tool. Never choose it for an explicit request to inspect Loom metadata or prepare, review, or change a Session, Mission, Team, RoundTable, Route, model, reasoning, workspace, Provider, Runtime, incident, or Needs You state. It performs no product action. Set decision to no_product_tool_matches only after checking every frozen tool description. Loom may require an independent second typed selection."
	conversationDirectReplyRecheckInstruction    = "Loom independent typed recheck: re-read the original latest user request and every frozen product tool description without treating the prior selection as evidence. Select the matching product tool for any explicit Loom metadata or review request. Select loom_conversation_reply only when no product tool matches. Judge semantic intent, not isolated words."
	conversationDirectAnswerInstruction          = "Loom control selection result: answer_request_directly. Two independent typed selections found no matching product tool. Answer the original latest user request without claiming a Loom read, Proposal, confirmation, execution, or state change."
	conversationControlContinuationInstruction   = "Loom continuation rule: prior completed product tools were read-only or corrective and did not prepare a Proposal. A read result never satisfies an explicit request to prepare or change Loom state. Re-evaluate the original latest request and call the matching proposal preview tool before any direct reply when the user requested a reviewable change."
	conversationTerminalArbitrationInstruction   = "Loom terminal tool arbitration: prior non-terminal calls did not complete the latest request. Read tools are intentionally omitted from this least-privilege selection. Choose the one matching proposal tool when the user requested a reviewable change; choose loom_conversation_reply only when no proposal tool matches."
	conversationArgumentRepairInstruction        = "Loom argument repair: the previous call selected this tool but its arguments were rejected. Use the exact frozen JSON schema, include every required field, add no undeclared field, and do not repeat identical arguments."
	conversationToolReselectionInstruction       = "Loom tool reselection: a previously selected tool exhausted its bounded argument repairs and is omitted from this selection. Re-evaluate the original latest request and choose a different matching proposal tool, or choose loom_conversation_reply only when no remaining proposal tool matches."
	conversationToolAvailabilityInstruction      = "Loom tool availability correction: the previous response selected a frozen tool that is not available in this least-privilege round. Ignore that selection and choose exactly one tool from the tools supplied with this request."
	conversationRequiredToolSelectionInstruction = "Loom required tool correction: the previous response returned prose even though a typed tool selection was required. Discard that prose and choose exactly one tool from the frozen tools supplied with this request. Plain text has no Loom authority."
	miniMaxConversationControlSeed               = int64(7)
)

var conversationDirectReplySchema = json.RawMessage(
	`{"type":"object","additionalProperties":false,"required":["decision"],"properties":{"decision":{"type":"string","enum":["no_product_tool_matches"]}}}`,
)

type ConversationControlTool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	// StopAfterSuccess is local execution metadata and is never serialized to
	// the Provider. Proposal tools use it because Loom already owns the review
	// artifact and does not need a second model round to restate it.
	StopAfterSuccess bool
	// DirectReply is adapter-only metadata. It never enters the product Tool
	// Registry and carries no read, proposal, confirmation or execution authority.
	DirectReply bool
}

type ConversationControlExecutor func(
	context.Context,
	string,
	json.RawMessage,
) (json.RawMessage, error)

type conversationControlFailureCoder interface {
	ConversationControlFailureCode() string
}

type conversationControlCorrectionEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Retryable bool   `json:"retryable"`
	} `json:"error"`
	Correction struct {
		Tool           string   `json:"tool"`
		RequiredFields []string `json:"required_fields"`
		AllowedFields  []string `json:"allowed_fields"`
		Rule           string   `json:"rule"`
	} `json:"correction"`
}

func conversationControlCorrectionResult(
	err error,
	tool ConversationControlTool,
) (json.RawMessage, bool) {
	if conversationControlFailureCode(err) != "invalid_request" {
		return nil, false
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if !validConversationControlName(tool.Name) ||
		json.Unmarshal(tool.InputSchema, &schema) != nil || schema.Properties == nil ||
		len(schema.Properties) > 64 || len(schema.Required) > 64 {
		return nil, false
	}
	allowedFields := make([]string, 0, len(schema.Properties))
	for field := range schema.Properties {
		if !validConversationControlName(field) {
			return nil, false
		}
		allowedFields = append(allowedFields, field)
	}
	sort.Strings(allowedFields)
	requiredFields := append([]string(nil), schema.Required...)
	sort.Strings(requiredFields)
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = struct{}{}
	}
	for index, field := range requiredFields {
		if !validConversationControlName(field) ||
			index > 0 && requiredFields[index-1] == field {
			return nil, false
		}
		if _, exists := allowed[field]; !exists {
			return nil, false
		}
	}
	var envelope conversationControlCorrectionEnvelope
	envelope.Error.Code = "invalid_request"
	envelope.Error.Retryable = true
	envelope.Correction.Tool = tool.Name
	envelope.Correction.RequiredFields = requiredFields
	envelope.Correction.AllowedFields = allowedFields
	envelope.Correction.Rule = "Match the frozen JSON schema exactly and do not repeat rejected arguments."
	result, marshalErr := json.Marshal(envelope)
	if marshalErr != nil || len(result) > maxConversationControlResultBytes {
		return nil, false
	}
	return result, true
}

func conversationControlArgumentRepairLimitFailure() error {
	return newConversationHTTPFailure(
		"provider_http", "invalid_response", 200,
		"response_tool_argument_repair_limit", true, 0,
	)
}

func conversationControlAvailabilityLimitFailure() error {
	return newConversationHTTPFailure(
		"provider_http", "invalid_response", 200,
		"response_tool_availability_correction_limit", true, 0,
	)
}

func conversationControlExecutionFailure(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newConversationHTTPFailure(
			"conversation_dispatch", "timeout", 0, "control_tool_timeout", true, 0,
		)
	}
	code := conversationControlFailureCode(err)
	if code == "" {
		code = "execution"
	}
	return newConversationHTTPFailure(
		"conversation_dispatch", "invalid_response", 0,
		"control_tool_"+code, false, 0,
	)
}

func conversationControlResultFailure() error {
	return newConversationHTTPFailure(
		"conversation_dispatch", "invalid_response", 0,
		"control_tool_result_validation", false, 0,
	)
}

func conversationControlFailureCode(err error) string {
	var failure conversationControlFailureCoder
	if !errors.As(err, &failure) || failure == nil {
		return ""
	}
	code := failure.ConversationControlFailureCode()
	if code == "" || len(code) > 32 {
		return ""
	}
	for _, character := range code {
		if character >= 'a' && character <= 'z' || character == '_' {
			continue
		}
		return ""
	}
	return code
}

func prepareConversationControlTools(
	tools []ConversationControlTool,
) (map[string]ConversationControlTool, []string, []ConversationControlTool, error) {
	if len(tools) == 0 || len(tools) >= maxConversationControlTools {
		return nil, nil, nil, ErrInvalidOpenAICompatibleConversation
	}
	prepared := make([]ConversationControlTool, len(tools), len(tools)+1)
	for index, tool := range tools {
		if tool.Name == prompting.ControlToolConversationReply || tool.DirectReply {
			return nil, nil, nil, ErrInvalidOpenAICompatibleConversation
		}
		prepared[index] = tool
		prepared[index].InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
	}
	prepared = append(prepared, ConversationControlTool{
		Name:        prompting.ControlToolConversationReply,
		Description: conversationDirectReplyDescription,
		InputSchema: append(json.RawMessage(nil), conversationDirectReplySchema...),
		DirectReply: true,
	})
	byName, names, err := validateConversationControlTools(prepared)
	if err != nil {
		return nil, nil, nil, err
	}
	return byName, names, prepared, nil
}

func validateConversationControlTools(
	tools []ConversationControlTool,
) (map[string]ConversationControlTool, []string, error) {
	if len(tools) == 0 || len(tools) > maxConversationControlTools {
		return nil, nil, ErrInvalidOpenAICompatibleConversation
	}
	byName := make(map[string]ConversationControlTool, len(tools))
	names := make([]string, len(tools))
	directReplies := 0
	for index, tool := range tools {
		if !validConversationControlName(tool.Name) ||
			!validConversationControlDescription(tool.Description) ||
			len(tool.InputSchema) == 0 ||
			len(tool.InputSchema) > maxConversationControlSchemaBytes ||
			rejectConversationDuplicateJSONKeys(tool.InputSchema) {
			return nil, nil, ErrInvalidOpenAICompatibleConversation
		}
		if _, duplicate := byName[tool.Name]; duplicate {
			return nil, nil, ErrInvalidOpenAICompatibleConversation
		}
		if tool.DirectReply {
			directReplies++
			if tool.Name != prompting.ControlToolConversationReply ||
				tool.Description != conversationDirectReplyDescription ||
				tool.StopAfterSuccess ||
				!bytes.Equal(tool.InputSchema, conversationDirectReplySchema) {
				return nil, nil, ErrInvalidOpenAICompatibleConversation
			}
		} else if tool.Name == prompting.ControlToolConversationReply {
			return nil, nil, ErrInvalidOpenAICompatibleConversation
		}
		decoder := json.NewDecoder(bytes.NewReader(tool.InputSchema))
		decoder.UseNumber()
		var schema map[string]any
		if decoder.Decode(&schema) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			schema["type"] != "object" || schema["additionalProperties"] != false {
			return nil, nil, ErrInvalidOpenAICompatibleConversation
		}
		clone := tool
		clone.InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
		byName[tool.Name] = clone
		names[index] = tool.Name
	}
	if directReplies > 1 {
		return nil, nil, ErrInvalidOpenAICompatibleConversation
	}
	return byName, names, nil
}

func validConversationDirectReplyArguments(raw json.RawMessage) bool {
	arguments, err := validateConversationControlArguments(raw)
	if err != nil {
		return false
	}
	defer zeroConversationBytes(arguments)
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.UseNumber()
	var object map[string]any
	if decoder.Decode(&object) != nil {
		return false
	}
	decision, decisionOK := object["decision"].(string)
	return decisionOK && decision == "no_product_tool_matches" && len(object) == 1 &&
		decoder.Decode(&struct{}{}) == io.EOF
}

func validConversationControlName(value string) bool {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' ||
			index > 0 && character >= '0' && character <= '9' ||
			index > 0 && character == '_' {
			continue
		}
		return false
	}
	return true
}

func validConversationControlDescription(value string) bool {
	if value == "" || len(value) > 1_024 || strings.TrimSpace(value) != value ||
		!utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validateConversationControlArguments(
	raw json.RawMessage,
) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 48<<10 || !utf8.Valid(raw) ||
		bytes.IndexByte(raw, 0) >= 0 || rejectConversationDuplicateJSONKeys(raw) {
		return nil, ErrInvalidOpenAICompatibleConversation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if decoder.Decode(&object) != nil || object == nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, ErrInvalidOpenAICompatibleConversation
	}
	return append(json.RawMessage(nil), raw...), nil
}

func validateConversationControlResult(
	raw json.RawMessage,
) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > maxConversationControlResultBytes ||
		!utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 ||
		rejectConversationDuplicateJSONKeys(raw) {
		return nil, ErrInvalidOpenAICompatibleConversation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if decoder.Decode(&object) != nil || object == nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, ErrInvalidOpenAICompatibleConversation
	}
	return append(json.RawMessage(nil), raw...), nil
}

func rejectConversationDuplicateJSONKeys(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var walk func() bool
	walk = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return true
		}
		delimiter, composite := token.(json.Delim)
		if !composite {
			return false
		}
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok {
					return true
				}
				if _, duplicate := seen[key]; duplicate {
					return true
				}
				seen[key] = struct{}{}
				if walk() {
					return true
				}
			}
		case '[':
			for decoder.More() {
				if walk() {
					return true
				}
			}
		default:
			return true
		}
		_, err = decoder.Token()
		return err != nil
	}
	return walk() || decoder.Decode(&struct{}{}) != io.EOF
}
