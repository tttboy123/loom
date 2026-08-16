// Package execution implements the B-W1 bounded execution adapter: a model
// ToolProposal is evaluated by the permissions pipeline, and only an allow
// verdict is executed by the daemon-side deterministic executor. Every step
// is an Event Journal fact and evidence is an artifact digest, never raw
// output or secrets.
package execution

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"
)

var (
	ErrInvalidExecutionInput   = errors.New("invalid execution input")
	ErrUnknownExecutionEvent   = errors.New("unknown execution event")
	ErrInvalidExecutionEvent   = errors.New("invalid execution event")
	ErrExecutionInFlight       = errors.New("execution already in flight")
	ErrExecutionInterrupted    = errors.New("execution interrupted before terminal fact; terminalize before replay")
	ErrApprovalConsumed        = errors.New("approval was already consumed by another execution")
	ErrExecutionPathOutside    = errors.New("execution path outside worktree")
	ErrExecutionLimit          = errors.New("execution limit exceeded")
	ErrExecutionTimedOut       = errors.New("execution timed out")
	ErrExecutionContent        = errors.New("execution content unavailable")
	ErrUnsupportedTool         = errors.New("tool is not executable by the adapter")
	ErrInvalidToolRecovery     = errors.New("invalid ToolCall recovery decision")
	ErrToolRecoveryConflict    = errors.New("ToolCall recovery decision conflict")
	ErrToolRecoveryUnavailable = errors.New("ToolCall recovery action unavailable")
	ErrToolRecoveryEvidence    = errors.New("ToolCall recovery evidence is invalid")
)

const (
	EventToolProposed         = "ToolExecutionProposed"
	EventToolAllowed          = "ToolExecutionAllowed"
	EventToolDenied           = "ToolExecutionDenied"
	EventToolCompleted        = "ToolExecutionCompleted"
	EventToolFailed           = "ToolExecutionFailed"
	EventToolRecoveryRequired = "ToolExecutionRecoveryRequired"
	EventToolRecoveryResolved = "ToolExecutionRecoveryResolved"
)

// Proposal is the adapter's frozen input. The model bridge (or a
// deterministic probe) proposes one tool call for one Queue Job.
type Proposal struct {
	JobID            string                          `json:"job_id"`
	Call             permissions.ProposedCall        `json:"call"`
	OperationID      string                          `json:"operation_id"`
	JourneyID        string                          `json:"journey_id"`
	DispatchGate     ExecutionDispatchGate           `json:"-"`
	ResultCommitGate ExecutionResultCommitGate       `json:"-"`
	Diagnostics      ToolExecutionDiagnosticRecorder `json:"-"`
}

// ExecutionDispatchInput is the content-free identity committed immediately
// before an allowed executor call. Tool arguments remain in the Proposal and
// are never copied into this authority boundary.
type ExecutionDispatchInput struct {
	JobID          string
	ExecutionID    string
	CallDigest     string
	Tool           permissions.ToolKind
	OperationID    string
	CorrelationID  string
	ApprovalID     string
	ApprovalDigest string
}

type ExecutionDispatchGate interface {
	CommitExecutionDispatch(context.Context, ExecutionDispatchInput) error
}

// RemoteToolExecutor is an injected Loom-owned Web/MCP broker. Validation is
// required before dispatch; execution returns an owned mutable content buffer.
type RemoteToolExecutor interface {
	AllowedRemoteTools() []permissions.ToolKind
	ValidateProposal(permissions.ProposedCall) error
	ExecuteProposalContent(context.Context, permissions.ProposedCall) ([]byte, error)
}

// ExecutionResultCommitInput is the exact content-bearing result boundary used
// before a remote execution terminal fact. Implementations must synchronously
// encrypt or otherwise durably own Content and must not retain this slice.
type ExecutionResultCommitInput struct {
	JobID         string
	ExecutionID   string
	CallDigest    string
	Tool          permissions.ToolKind
	OperationID   string
	CorrelationID string
	OutputDigest  string
	DurationMS    int64
	Content       []byte
}

type ExecutionResultCommitGate interface {
	CommitExecutionResult(context.Context, ExecutionResultCommitInput) error
}

type ToolExecutionStage string

const (
	ToolStageAuthorization     ToolExecutionStage = "tool_authorization"
	ToolStageApprovalWait      ToolExecutionStage = "tool_approval_wait"
	ToolStageSandboxPrepare    ToolExecutionStage = "tool_sandbox_prepare"
	ToolStageBindingValidation ToolExecutionStage = "tool_binding_validation"
	ToolStageDispatch          ToolExecutionStage = "tool_dispatch"
	ToolStageResultValidation  ToolExecutionStage = "tool_result_validation"
	ToolStageResultCommit      ToolExecutionStage = "tool_result_commit"
	ToolStagePayloadCommit     ToolExecutionStage = "tool_payload_commit"
	ToolStageResultDelivery    ToolExecutionStage = "tool_result_delivery"
	ToolStageRecovery          ToolExecutionStage = "tool_recovery"
)

const (
	ToolDiagnosticSucceeded = "succeeded"
	ToolDiagnosticFailed    = "failed"
)

// ToolExecutionDiagnostic is deliberately content-free. It carries only the
// frozen execution identity, safe stage/result enums, elapsed time, and a
// controlled error code; command, path, output, Prompt, and Provider content
// have no representation at this boundary.
type ToolExecutionDiagnostic struct {
	ExecutionID   string               `json:"execution_id,omitempty"`
	JobID         string               `json:"job_id"`
	CallDigest    string               `json:"call_digest"`
	Tool          permissions.ToolKind `json:"tool"`
	OperationID   string               `json:"operation_id"`
	CorrelationID string               `json:"correlation_id"`
	Stage         ToolExecutionStage   `json:"stage"`
	Elapsed       time.Duration        `json:"-"`
	Result        string               `json:"result"`
	ErrorCode     string               `json:"error_code,omitempty"`
	Retryable     bool                 `json:"retryable"`
}

// ToolExecutionDiagnosticRecorder observes execution; it is never an
// authorization authority. Recorder errors are intentionally non-fatal.
type ToolExecutionDiagnosticRecorder interface {
	RecordToolExecutionDiagnostic(context.Context, ToolExecutionDiagnostic) error
}

// ExecutionResult is the typed outcome of Execute. Verdict is allow when the
// tool executed, ask when an approval was created or is pending, and deny
// when the call was refused without side effects.
type ExecutionResult struct {
	ExecutionID        string              `json:"execution_id"`
	JobID              string              `json:"job_id"`
	Verdict            permissions.Verdict `json:"verdict"`
	Denial             permissions.Denial  `json:"denial,omitempty"`
	ApprovalID         string              `json:"approval_id,omitempty"`
	ApprovalDigest     string              `json:"approval_digest,omitempty"`
	ExitCode           int                 `json:"exit_code,omitempty"`
	OutputDigest       string              `json:"output_digest,omitempty"`
	ChangedFilesDigest string              `json:"changed_files_digest,omitempty"`
	EvidenceID         string              `json:"evidence_id,omitempty"`
	DurationMS         int64               `json:"duration_ms,omitempty"`
	Note               string              `json:"note,omitempty"`
	ErrorCode          string              `json:"error_code,omitempty"`
	RecoveryRequired   bool                `json:"recovery_required,omitempty"`
	RecoveryCode       string              `json:"recovery_code,omitempty"`
	RecoveryAction     string              `json:"recovery_action,omitempty"`
	Retryable          bool                `json:"retryable"`
	Content            []byte              `json:"-"`
}

func (result *ExecutionResult) Close() {
	if result == nil {
		return
	}
	for index := range result.Content {
		result.Content[index] = 0
	}
	result.Content = nil
}

// EditRequest and RunRequest are the frozen executor inputs. Worktree is the
// candidate worktree root resolved by WorktreeResolver.
type EditRequest struct {
	Worktree         string
	RelativePath     string
	NewContentDigest string
	NewContent       string
	NewContentBytes  []byte
}

type EditResult struct {
	ChangedFilesDigest string
	RelativePath       string
}

type RunRequest struct {
	Worktree    string
	Command     string
	Timeout     time.Duration
	OutputLimit int64
}

type RunResult struct {
	ExitCode           int
	OutputDigest       string
	ChangedFilesDigest string
	DurationMS         int64
}

type ReadRequest struct {
	Worktree     string
	RelativePath string
	OutputLimit  int64
}

type ReadResult struct {
	Content       []byte
	ContentDigest string
	OutputDigest  string
	Truncated     bool
	DurationMS    int64
}

func (result *ReadResult) Close() {
	if result == nil {
		return
	}
	for index := range result.Content {
		result.Content[index] = 0
	}
	result.Content = nil
}

type GrepRequest struct {
	Worktree     string
	RelativePath string
	Pattern      string
	OutputLimit  int64
	MatchLimit   int
}

type GrepResult = ReadResult

type ReadExecutor interface {
	Read(context.Context, ReadRequest) (ReadResult, error)
}

type GrepExecutor interface {
	Grep(context.Context, GrepRequest) (GrepResult, error)
}

// Executor is the deterministic, sandboxed side-effect engine. It never
// decides authorization; it only applies an already-allowed request.
type Executor interface {
	Edit(context.Context, EditRequest) (EditResult, error)
	Run(context.Context, RunRequest) (RunResult, error)
}

// WorktreeResolver maps a Queue Job to its latest claimed candidate
// worktree. The daemon injects the resolver; failure is fail-closed.
type WorktreeResolver interface {
	Resolve(context.Context, string) (string, error)
}

// ApprovalRequester reuses the existing internal/rules approval lifecycle for
// an ask verdict (A4). It is nil-safe: nil means forward-only (decision fact
// only, no approval request).
type ApprovalRequester interface {
	RequestPermissionApproval(context.Context, rules.PermissionApprovalInput) (rules.ApprovalRequestRecord, error)
}

type ApprovalConsumer interface {
	ConsumePermissionApproval(
		context.Context,
		rules.PermissionApprovalConsumptionInput,
	) (rules.ApprovalRequestRecord, error)
}

// DecisionRecorder records the ask decision fact with the same semantics as
// the B-P1 permission service.
type DecisionRecorder interface {
	RecordDecision(context.Context, string, permissions.ProposedCall, permissions.Verdict, permissions.Denial, string, string, string) error
}

// ExecutionRecord is one execution row in the read-only projection.
type ExecutionRecord struct {
	ExecutionID                  string               `json:"execution_id"`
	JobID                        string               `json:"job_id"`
	CallDigest                   string               `json:"call_digest"`
	Tool                         permissions.ToolKind `json:"tool"`
	Command                      string               `json:"command,omitempty"`
	Path                         string               `json:"path,omitempty"`
	Generation                   int64                `json:"generation"`
	OperationID                  string               `json:"operation_id"`
	JourneyID                    string               `json:"journey_id"`
	ProposedAt                   string               `json:"proposed_at"`
	AllowedAt                    string               `json:"allowed_at,omitempty"`
	DeniedAt                     string               `json:"denied_at,omitempty"`
	DenialReason                 string               `json:"denial_reason,omitempty"`
	ExitCode                     int                  `json:"exit_code,omitempty"`
	OutputDigest                 string               `json:"output_digest,omitempty"`
	ChangedFilesDigest           string               `json:"changed_files_digest,omitempty"`
	EvidenceID                   string               `json:"evidence_id,omitempty"`
	DurationMS                   int64                `json:"duration_ms,omitempty"`
	Status                       string               `json:"status"`
	CompletedAt                  string               `json:"completed_at,omitempty"`
	FailedAt                     string               `json:"failed_at,omitempty"`
	FailureReason                string               `json:"failure_reason,omitempty"`
	ErrorCode                    string               `json:"error_code,omitempty"`
	RecoveryRequiredAt           string               `json:"recovery_required_at,omitempty"`
	RecoveryCode                 string               `json:"recovery_code,omitempty"`
	RecoveryAction               string               `json:"recovery_action,omitempty"`
	RecoveryDecisionID           string               `json:"recovery_decision_id,omitempty"`
	RecoveryDecision             string               `json:"recovery_decision,omitempty"`
	RecoveryResolvedAt           string               `json:"recovery_resolved_at,omitempty"`
	RecoveryEvidenceID           string               `json:"recovery_evidence_id,omitempty"`
	RecoveryObservationDigest    string               `json:"recovery_observation_digest,omitempty"`
	RecoveryReplacementAttemptID string               `json:"recovery_replacement_attempt_id,omitempty"`
	RecoveryReplacementRunID     string               `json:"recovery_replacement_run_id,omitempty"`
	recoveryEventID              string               `json:"-"`
}

// ExecutionSnapshot is the deterministic read model over execution streams.
type ExecutionSnapshot struct {
	Records []ExecutionRecord `json:"records"`
}

func (snapshot ExecutionSnapshot) Record(executionID string) (ExecutionRecord, bool) {
	for _, record := range snapshot.Records {
		if record.ExecutionID == executionID {
			return record, true
		}
	}
	return ExecutionRecord{}, false
}
