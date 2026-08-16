package runtime

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/permissions"
)

var (
	ErrInvalidToolCallSequence    = errors.New("invalid ToolCall sequence")
	ErrInvalidToolCallEnvelope    = errors.New("invalid tool call envelope")
	ErrToolCallGatewayUnavailable = errors.New("tool call hook unavailable")
)

const MaxSequentialToolCalls = 4

type toolCallSequenceContextKey struct{}

func BindToolCallSequence(ctx context.Context, sequence int64) (context.Context, error) {
	if ctx == nil || sequence < 1 || sequence > MaxSequentialToolCalls {
		return nil, ErrInvalidToolCallSequence
	}
	return context.WithValue(ctx, toolCallSequenceContextKey{}, sequence), nil
}

func ToolCallSequence(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	sequence, ok := ctx.Value(toolCallSequenceContextKey{}).(int64)
	return sequence, ok && sequence >= 1 && sequence <= MaxSequentialToolCalls
}

// ToolCallEnvelope is a Runtime-neutral model proposal. It carries no
// execution authority; the daemon-owned AttemptToolGateway resolves all
// frozen identity and policy from ToolCallBinding.
type ToolCallEnvelope struct {
	JobID string                   `json:"job_id"`
	Call  permissions.ProposedCall `json:"call"`
}

type ToolCallBinding struct {
	ConversationID         string
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	AgentInstanceID        string
	ExecutionBindingDigest string
	CapsuleDigest          string
	ClaimID                string
	IncidentID             string
	JourneyID              string
}

type ToolCallResult struct {
	Verdict            permissions.Verdict `json:"verdict"`
	ExecutionID        string              `json:"execution_id,omitempty"`
	ApprovalID         string              `json:"approval_id,omitempty"`
	ApprovalDigest     string              `json:"approval_digest,omitempty"`
	ResultNote         string              `json:"result_note,omitempty"`
	ErrorCode          string              `json:"error_code,omitempty"`
	DenialReason       string              `json:"denial_reason,omitempty"`
	AuthorizationPath  string              `json:"authorization_path,omitempty"`
	ExitCode           int                 `json:"exit_code,omitempty"`
	OutputDigest       string              `json:"output_digest,omitempty"`
	ChangedFilesDigest string              `json:"changed_files_digest,omitempty"`
	EvidenceID         string              `json:"evidence_id,omitempty"`
	DurationMS         int64               `json:"duration_ms,omitempty"`
	ContentDigest      string              `json:"-"`
	Delivery           *ToolCallDelivery   `json:"-"`
}

type ToolCallDelivery struct {
	Binding     attemptpayload.Binding
	CallDigest  string
	Tool        permissions.ToolKind
	OperationID string
}

type AttemptToolGateway interface {
	ExecuteToolCall(context.Context, ToolCallEnvelope, ToolCallBinding) (ToolCallResult, error)
}

type ToolCallCapabilityProvider interface {
	AllowedToolCalls() []permissions.ToolKind
}

type ToolCallResultAcknowledger interface {
	AcknowledgeToolCallResult(context.Context, ToolCallBinding, ToolCallResult) error
}

type ToolCallResultProofAcknowledger interface {
	AcknowledgeToolCallResultWithProof(
		context.Context,
		ToolCallBinding,
		ToolCallResult,
		attemptpayload.DeliveryProof,
	) error
}

type ToolCallResultContentReader interface {
	ReadToolCallResultContent(
		context.Context,
		ToolCallBinding,
		ToolCallResult,
	) ([]byte, error)
}
