package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"loom-pi-rebuild/internal/permissions"
)

var (
	ErrInvalidToolCallEnvelope = errors.New("invalid tool call envelope")
	ErrToolCallUnknownTool     = errors.New("unknown tool call tool")
	ErrToolCallHookUnavailable = errors.New("tool call hook unavailable")
)

const (
	toolCallMaxEnvelopeBytes = 4096
)

// bridgeAllowedTools 是 W-BRIDGE 冻结的本地工具白名单。网络/远端/第三方
// 工具（WebFetch/WebSearch/MCPTool）不在本契约范围，严格拒绝。
var bridgeAllowedTools = map[permissions.ToolKind]struct{}{
	permissions.ToolBash: {},
	permissions.ToolEdit: {},
	permissions.ToolRead: {},
	permissions.ToolGrep: {},
}

// ToolCallEnvelope 是模型提议工具调用的唯一信封（冻结于 W-BRIDGE 契约
// §3.1）。模型输出是 Proposal；daemon 经执行适配器裁决后才可能执行。
type ToolCallEnvelope struct {
	JobID string                  `json:"job_id"`
	Call  permissions.ProposedCall `json:"call"`
}

// ToolCallBinding 把信封绑定到 Run/Attempt lineage，供 generation 围栏与
// Journal 事实使用。
type ToolCallBinding struct {
	WorkItemID      string
	RunID           string
	ClaimGeneration int64
	JourneyID       string
}

// ToolCallResult 是 daemon hook 对一次信封的裁决结果（allow 的结果摘要 /
// ask 的批准引用 / deny 的 typed denial），桥接只透传不决策。
type ToolCallResult struct {
	Verdict           permissions.Verdict `json:"verdict"`
	ExecutionID       string              `json:"execution_id,omitempty"`
	ApprovalID        string              `json:"approval_id,omitempty"`
	ApprovalDigest    string              `json:"approval_digest,omitempty"`
	ResultNote        string              `json:"result_note,omitempty"`
	DenialReason      string              `json:"denial_reason,omitempty"`
	AuthorizationPath string              `json:"authorization_path,omitempty"`
}

// ToolCallHook 由 daemon 组装（持有 execution.Adapter + 权限投影）；桥接
// 本身不含执行决策（不变量 3）。
type ToolCallHook interface {
	ExecuteToolCall(
		ctx context.Context,
		envelope ToolCallEnvelope,
		binding ToolCallBinding,
	) (ToolCallResult, error)
}

// DecodeToolCallEnvelope 严格解码：拒绝未知键、重复键、尾随值、空 job_id
// 与白名单之外的 tool；任何解析失败返回错误，绝不进入执行裁决。
func DecodeToolCallEnvelope(line []byte) (ToolCallEnvelope, error) {
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
	if _, ok := bridgeAllowedTools[envelope.Call.Tool]; !ok {
		return ToolCallEnvelope{}, ErrToolCallUnknownTool
	}
	switch envelope.Call.Tool {
	case permissions.ToolBash:
		if envelope.Call.Command == "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	case permissions.ToolRead, permissions.ToolGrep, permissions.ToolEdit:
		if envelope.Call.Path == "" {
			return ToolCallEnvelope{}, ErrInvalidToolCallEnvelope
		}
	}
	return envelope, nil
}

// ToolCallSystemPrompt 是启用工具面后的冻结系统提示：唯一信封协议、禁止
// 隐藏推理/凭据/路径、禁止自证执行。RED 5 直接断言此常量。
func ToolCallSystemPrompt() string {
	return "Answer only the supplied bounded task. To act, propose exactly one tool call per assistant message as a single JSON envelope {\"job_id\":\"...\",\"call\":{\"tool\":\"Bash|Edit|Read|Grep\",\"command\":\"...\",\"path\":\"...\"}} via one toolcall event. Do not include hidden reasoning, credentials, or filesystem paths in the envelope. Execution and results are owned by the daemon; never claim to have executed a tool. Return concise plain text otherwise."
}
