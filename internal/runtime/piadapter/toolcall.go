package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/prompting"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidToolCallEnvelope = loomruntime.ErrInvalidToolCallEnvelope
	ErrToolCallUnknownTool     = errors.New("unknown tool call tool")
	ErrToolCallHookUnavailable = loomruntime.ErrToolCallGatewayUnavailable
)

const (
	toolCallMaxEnvelopeBytes = 4096
	piMaxSequentialToolCalls = loomruntime.MaxSequentialToolCalls
)

func BindToolCallSequence(ctx context.Context, sequence int64) (context.Context, error) {
	bound, err := loomruntime.BindToolCallSequence(ctx, sequence)
	if err != nil {
		return nil, ErrInvalidToolCallEnvelope
	}
	return bound, nil
}

func ToolCallSequence(ctx context.Context) (int64, bool) {
	return loomruntime.ToolCallSequence(ctx)
}

// bridgeAllowedTools 是 W-BRIDGE 冻结的本地工具白名单。网络/远端/第三方
// 工具（WebFetch/WebSearch/MCPTool）不在本契约范围，严格拒绝。
var bridgeAllowedTools = map[permissions.ToolKind]struct{}{
	permissions.ToolBash: {},
	permissions.ToolEdit: {},
	permissions.ToolRead: {},
	permissions.ToolGrep: {},
}

var governedToolKinds = map[permissions.ToolKind]struct{}{
	permissions.ToolBash: {}, permissions.ToolEdit: {},
	permissions.ToolRead: {}, permissions.ToolGrep: {},
	permissions.ToolWebSearch: {}, permissions.ToolWebFetch: {},
	permissions.ToolMCPTool: {},
}

// Compatibility aliases preserve Pi's public protocol while the authority
// contract is shared by every Runtime transport.
type ToolCallEnvelope = loomruntime.ToolCallEnvelope
type ToolCallBinding = loomruntime.ToolCallBinding
type ToolCallResult = loomruntime.ToolCallResult
type ToolCallDelivery = loomruntime.ToolCallDelivery
type ToolCallHook = loomruntime.AttemptToolGateway
type ToolCallCapabilityProvider = loomruntime.ToolCallCapabilityProvider
type ToolCallResultAcknowledger = loomruntime.ToolCallResultAcknowledger
type ToolCallResultProofAcknowledger = loomruntime.ToolCallResultProofAcknowledger
type ToolCallResultContentReader = loomruntime.ToolCallResultContentReader

func marshalToolCallResultPayload(
	envelope ToolCallEnvelope,
	result ToolCallResult,
) ([]byte, error) {
	return json.Marshal(struct {
		CallDigest string         `json:"call_digest"`
		Tool       string         `json:"tool"`
		Result     ToolCallResult `json:"result"`
	}{
		CallDigest: permissions.ProposedCallDigest(envelope.Call),
		Tool:       string(envelope.Call.Tool),
		Result:     result,
	})
}

// DecodeToolCallEnvelope 严格解码：拒绝未知键、重复键、尾随值、空 job_id
// 与白名单之外的 tool；任何解析失败返回错误，绝不进入执行裁决。
func DecodeToolCallEnvelope(line []byte) (ToolCallEnvelope, error) {
	return decodeToolCallEnvelope(line, bridgeAllowedTools)
}

func decodeToolCallEnvelope(
	line []byte,
	allowed map[permissions.ToolKind]struct{},
) (ToolCallEnvelope, error) {
	if len(line) == 0 || len(line) > toolCallMaxEnvelopeBytes {
		return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
	}
	if err := rejectPiRPCDuplicateKeys(line); err != nil {
		return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	var envelope ToolCallEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
	}
	if token, err := decoder.Token(); !errors.Is(err, io.EOF) || token != nil {
		return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
	}
	if envelope.JobID == "" {
		return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
	}
	if _, ok := allowed[envelope.Call.Tool]; !ok {
		return ToolCallEnvelope{}, ErrToolCallUnknownTool
	}
	switch envelope.Call.Tool {
	case permissions.ToolBash:
		if envelope.Call.Command == "" || envelope.Call.Path != "" || envelope.Call.Pattern != "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	case permissions.ToolRead:
		if envelope.Call.Path == "" || envelope.Call.Command != "" || envelope.Call.Pattern != "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	case permissions.ToolGrep:
		if envelope.Call.Path == "" || envelope.Call.Pattern == "" || envelope.Call.Command != "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	case permissions.ToolEdit:
		if envelope.Call.Path == "" || envelope.Call.Command == "" || envelope.Call.Pattern != "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	case permissions.ToolWebSearch, permissions.ToolWebFetch:
		if envelope.Call.Path == "" || envelope.Call.Command != "" || envelope.Call.Pattern != "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	case permissions.ToolMCPTool:
		if envelope.Call.Path == "" || envelope.Call.Command == "" || envelope.Call.Pattern != "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	}
	return envelope, nil
}

func allowedPiToolCalls(hook ToolCallHook) map[permissions.ToolKind]struct{} {
	allowed := make(map[permissions.ToolKind]struct{}, len(bridgeAllowedTools))
	for tool := range bridgeAllowedTools {
		allowed[tool] = struct{}{}
	}
	provider, ok := hook.(ToolCallCapabilityProvider)
	if !ok {
		return allowed
	}
	candidate := provider.AllowedToolCalls()
	if len(candidate) == 0 {
		return allowed
	}
	allowed = make(map[permissions.ToolKind]struct{}, len(candidate))
	for _, tool := range candidate {
		if _, valid := governedToolKinds[tool]; !valid {
			return map[permissions.ToolKind]struct{}{}
		}
		allowed[tool] = struct{}{}
	}
	return allowed
}

// ToolCallSystemPrompt 是启用工具面后的冻结系统提示：唯一信封协议、禁止
// 隐藏推理/凭据/路径、禁止自证执行。RED 5 直接断言此常量。
func ToolCallSystemPrompt() string {
	return toolCallSystemPrompt(bridgeAllowedTools, false)
}

func toolCallSystemPrompt(
	allowed map[permissions.ToolKind]struct{},
	withContext bool,
) string {
	tools, names, ok := piPromptTools(allowed)
	if !ok {
		return "No tools are available. Return a proposal without claiming execution."
	}
	if withContext {
		tools = append(tools, prompting.ToolContextRead)
	}
	base, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeAgent, ProviderID: piRPCProviderID,
		ModelID: piRPCModelID, HarnessAdapter: "pi",
		Tools: tools,
	})
	if err != nil {
		return "No tools are available. Return a proposal without claiming execution."
	}
	toolList := strings.Join(names, "|")
	if withContext {
		return base + "\nFor this governed Attempt, use exactly one Loom-owned tool in the first turn. " +
			"Use loom_read_context only for an exact omitted Context Capsule item. " +
			"Use loom_tool for exactly one {\"job_id\":\"...\",\"call\":{\"tool\":\"" + toolList + "\",...}} envelope. " +
			"Never include credentials or hidden reasoning. Continue from the native toolResult in the next turn."
	}
	return base + "\nTool protocol: propose exactly one tool call per assistant message as one JSON envelope " +
		"{\"job_id\":\"...\",\"call\":{\"tool\":\"" + toolList + "\",\"command\":\"...\",\"path\":\"...\"}} " +
		"through one toolcall event. Include only arguments required by that tool and never include credentials or hidden reasoning."
}

func piPromptTools(
	allowed map[permissions.ToolKind]struct{},
) ([]prompting.ToolCapability, []string, bool) {
	if len(allowed) == 0 {
		return nil, nil, false
	}
	names := make([]string, 0, len(allowed))
	tools := make([]prompting.ToolCapability, 0, len(allowed))
	for tool := range allowed {
		var capability prompting.ToolCapability
		switch tool {
		case permissions.ToolBash:
			capability = prompting.ToolBash
		case permissions.ToolEdit:
			capability = prompting.ToolEdit
		case permissions.ToolRead:
			capability = prompting.ToolRead
		case permissions.ToolGrep:
			capability = prompting.ToolGrep
		case permissions.ToolWebSearch:
			capability = prompting.ToolWebSearch
		case permissions.ToolWebFetch:
			capability = prompting.ToolWebFetch
		case permissions.ToolMCPTool:
			capability = prompting.ToolMCP
		default:
			return nil, nil, false
		}
		tools = append(tools, capability)
		names = append(names, string(tool))
	}
	sort.Strings(names)
	return tools, names, true
}

func ContextReadSystemPrompt() string {
	base, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeAgent, ProviderID: piRPCProviderID,
		ModelID: piRPCModelID, HarnessAdapter: "pi",
		Tools: []prompting.ToolCapability{prompting.ToolContextRead},
	})
	if err != nil {
		return "No tools are available. Return a proposal without claiming execution."
	}
	return base + "\nContext retrieval protocol: call loom_read_context at most once with exactly " +
		"item_id, content_digest, and optional artifact_ref from the omitted Context Capsule manifest. " +
		"Treat the returned trust and source metadata as binding; never request credentials, hidden reasoning, or undeclared context."
}

func ContextAndToolSystemPrompt() string {
	return toolCallSystemPrompt(bridgeAllowedTools, true)
}
