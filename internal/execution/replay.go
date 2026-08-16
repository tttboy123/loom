package execution

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

const executionStreamPrefix = "execution/"

// ReplaySnapshot rebuilds the deterministic execution read model from the
// full Event Journal. Non-execution streams are ignored; unknown event types
// inside an execution stream are rejected (fail-closed).
func ReplaySnapshot(events []journal.Event) (ExecutionSnapshot, error) {
	records := make(map[string]*ExecutionRecord)
	var order []string
	for _, event := range events {
		if !strings.HasPrefix(event.StreamID, executionStreamPrefix) {
			continue
		}
		record, err := applyExecutionEvent(records, order, event)
		if err != nil {
			return ExecutionSnapshot{}, err
		}
		if record != nil {
			order = appendUnique(order, record.ExecutionID)
		}
	}
	snapshot := make([]ExecutionRecord, 0, len(records))
	for _, id := range order {
		snapshot = append(snapshot, *records[id])
	}
	return ExecutionSnapshot{Records: snapshot}, nil
}

func applyExecutionEvent(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	if event.SchemaVersion != 1 && event.SchemaVersion != 2 {
		return nil, fmt.Errorf("%w: schema_version=%d", ErrInvalidExecutionEvent, event.SchemaVersion)
	}
	if event.SchemaVersion == 2 && event.Type != EventToolProposed &&
		event.Type != EventToolDenied && event.Type != EventToolFailed &&
		event.Type != EventToolRecoveryRequired && event.Type != EventToolRecoveryResolved {
		return nil, fmt.Errorf("%w: schema_version=2 type=%s", ErrInvalidExecutionEvent, event.Type)
	}
	var (
		record *ExecutionRecord
		err    error
	)
	switch event.Type {
	case EventToolProposed:
		record, err = applyProposed(records, order, event)
	case EventToolAllowed:
		record, err = applyAllowed(records, order, event)
	case EventToolDenied:
		record, err = applyDenied(records, order, event)
	case EventToolCompleted:
		record, err = applyCompleted(records, order, event)
	case EventToolFailed:
		record, err = applyFailed(records, order, event)
	case EventToolRecoveryRequired:
		record, err = applyRecoveryRequired(records, order, event)
	case EventToolRecoveryResolved:
		record, err = applyRecoveryResolved(records, order, event)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownExecutionEvent, event.Type)
	}
	return record, err
}

func executionStreamID(jobID, executionID string) string {
	return executionStreamPrefix + jobID + "/" + executionID
}

func decodeExecutionPayload(event journal.Event, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(event.PayloadJSON)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing data")
	}
	return nil
}

func getOrCreate(records map[string]*ExecutionRecord, order []string, executionID, jobID string) *ExecutionRecord {
	if record, ok := records[executionID]; ok {
		return record
	}
	record := &ExecutionRecord{ExecutionID: executionID, JobID: jobID}
	records[executionID] = record
	order = appendUnique(order, executionID)
	return record
}

func mustGet(records map[string]*ExecutionRecord, executionID string) (*ExecutionRecord, error) {
	record, ok := records[executionID]
	if !ok {
		return nil, fmt.Errorf("%w: %s before ToolExecutionProposed", ErrInvalidExecutionEvent, executionID)
	}
	return record, nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func applyProposed(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	payload := proposedPayload{}
	if event.SchemaVersion == 1 {
		if err := decodeExecutionPayload(event, &payload); err != nil {
			return nil, err
		}
	} else {
		var contentFree proposedPayloadV2
		if err := decodeExecutionPayload(event, &contentFree); err != nil {
			return nil, err
		}
		payload.JobID = contentFree.JobID
		payload.ExecutionID = contentFree.ExecutionID
		payload.CallDigest = contentFree.CallDigest
		payload.Tool = contentFree.Tool
		payload.ProposedAt = contentFree.ProposedAt
		payload.Generation = contentFree.Generation
		payload.OperationID = contentFree.OperationID
		payload.JourneyID = contentFree.JourneyID
	}
	expected := executionStreamID(payload.JobID, payload.ExecutionID)
	if event.StreamID != expected {
		return nil, fmt.Errorf("%w: stream mismatch", ErrInvalidExecutionEvent)
	}
	if payload.CallDigest == "" || payload.Tool == "" || payload.ProposedAt == "" {
		return nil, fmt.Errorf("%w: invalid proposed payload", ErrInvalidExecutionEvent)
	}
	record := getOrCreate(records, order, payload.ExecutionID, payload.JobID)
	if record.ProposedAt != "" {
		return nil, fmt.Errorf("%w: duplicate proposed for %s", ErrInvalidExecutionEvent, payload.ExecutionID)
	}
	record.CallDigest = payload.CallDigest
	record.Tool = permissions.ToolKind(payload.Tool)
	record.Command = payload.Command
	record.Path = payload.Path
	record.Generation = payload.Generation
	record.OperationID = payload.OperationID
	record.JourneyID = payload.JourneyID
	record.ProposedAt = payload.ProposedAt
	record.Status = "proposed"
	return record, nil
}

func applyAllowed(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	var payload allowedPayload
	if err := decodeExecutionPayload(event, &payload); err != nil {
		return nil, err
	}
	record, err := mustGet(records, payload.ExecutionID)
	if err != nil {
		return nil, err
	}
	if payload.AllowedAt == "" {
		return nil, fmt.Errorf("%w: invalid allowed payload", ErrInvalidExecutionEvent)
	}
	if record.AllowedAt != "" {
		return nil, fmt.Errorf("%w: duplicate allowed for %s", ErrInvalidExecutionEvent, payload.ExecutionID)
	}
	record.AllowedAt = payload.AllowedAt
	record.Status = "allowed"
	return record, nil
}

func applyDenied(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	payload := deniedPayload{}
	if event.SchemaVersion == 1 {
		if err := decodeExecutionPayload(event, &payload); err != nil {
			return nil, err
		}
	} else {
		var contentFree deniedPayloadV2
		if err := decodeExecutionPayload(event, &contentFree); err != nil {
			return nil, err
		}
		if contentFree.ReasonCode != "permission_denied" {
			return nil, fmt.Errorf("%w: invalid denial reason code", ErrInvalidExecutionEvent)
		}
		payload.ExecutionID = contentFree.ExecutionID
		payload.Denial.Reason = contentFree.ReasonCode
		payload.DeniedAt = contentFree.DeniedAt
	}
	record, err := mustGet(records, payload.ExecutionID)
	if err != nil {
		return nil, err
	}
	if record.DeniedAt != "" {
		return nil, fmt.Errorf("%w: duplicate denied for %s", ErrInvalidExecutionEvent, payload.ExecutionID)
	}
	record.DeniedAt = payload.DeniedAt
	record.DenialReason = payload.Denial.Reason
	record.Status = "denied"
	return record, nil
}

func applyCompleted(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	var payload completedPayload
	if err := decodeExecutionPayload(event, &payload); err != nil {
		return nil, err
	}
	record, err := mustGet(records, payload.ExecutionID)
	if err != nil {
		return nil, err
	}
	if record.AllowedAt == "" {
		return nil, fmt.Errorf("%w: completed without allowed for %s", ErrInvalidExecutionEvent, payload.ExecutionID)
	}
	if record.CompletedAt != "" {
		return nil, fmt.Errorf("%w: duplicate completed for %s", ErrInvalidExecutionEvent, payload.ExecutionID)
	}
	record.ExitCode = payload.ExitCode
	record.OutputDigest = payload.OutputDigest
	record.ChangedFilesDigest = payload.ChangedFilesDigest
	record.EvidenceID = payload.EvidenceID
	record.DurationMS = payload.DurationMS
	record.CompletedAt = payload.CompletedAt
	record.Status = "completed"
	return record, nil
}

func applyFailed(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	payload := failedPayload{}
	if event.SchemaVersion == 1 {
		if err := decodeExecutionPayload(event, &payload); err != nil {
			return nil, err
		}
	} else {
		var contentFree failedPayloadV2
		if err := decodeExecutionPayload(event, &contentFree); err != nil {
			return nil, err
		}
		if contentFree.ErrorCode == "" {
			return nil, fmt.Errorf("%w: empty failure error code", ErrInvalidExecutionEvent)
		}
		payload.ExecutionID = contentFree.ExecutionID
		payload.Reason = contentFree.ErrorCode
		payload.ErrorCode = contentFree.ErrorCode
		payload.FailedAt = contentFree.FailedAt
	}
	record, err := mustGet(records, payload.ExecutionID)
	if err != nil {
		return nil, err
	}
	if record.FailedAt != "" {
		return nil, fmt.Errorf("%w: duplicate failed for %s", ErrInvalidExecutionEvent, payload.ExecutionID)
	}
	record.FailedAt = payload.FailedAt
	record.FailureReason = payload.Reason
	record.ErrorCode = payload.ErrorCode
	record.Status = "failed"
	return record, nil
}

func applyRecoveryRequired(
	records map[string]*ExecutionRecord,
	order []string,
	event journal.Event,
) (*ExecutionRecord, error) {
	var payload recoveryRequiredPayloadV2
	if event.SchemaVersion != 2 || decodeExecutionPayload(event, &payload) != nil ||
		payload.ExecutionID == "" || payload.RecoveryCode != "side_effect_unknown" ||
		payload.RecoveryAction != "resolve_tool_recovery" ||
		payload.RequiredAt == "" {
		return nil, fmt.Errorf("%w: invalid recovery-required payload", ErrInvalidExecutionEvent)
	}
	record, err := mustGet(records, payload.ExecutionID)
	if err != nil {
		return nil, err
	}
	if record.AllowedAt == "" || record.CompletedAt != "" || record.FailedAt != "" ||
		record.DeniedAt != "" || record.RecoveryRequiredAt != "" {
		return nil, fmt.Errorf("%w: invalid recovery-required transition", ErrInvalidExecutionEvent)
	}
	record.RecoveryRequiredAt = payload.RequiredAt
	record.RecoveryCode = payload.RecoveryCode
	record.RecoveryAction = payload.RecoveryAction
	record.recoveryEventID = event.ID
	record.Status = "recovery_required"
	return record, nil
}

func applyRecoveryResolved(
	records map[string]*ExecutionRecord,
	order []string,
	event journal.Event,
) (*ExecutionRecord, error) {
	var payload toolRecoveryResolvedPayloadV2
	if event.SchemaVersion != 2 || decodeExecutionPayload(event, &payload) != nil ||
		!validToolRecoveryResolvedPayload(payload) {
		return nil, fmt.Errorf("%w: invalid recovery-resolved payload", ErrInvalidExecutionEvent)
	}
	record, err := mustGet(records, payload.ExecutionID)
	if err != nil {
		return nil, err
	}
	if record.Status != "recovery_required" || record.recoveryEventID == "" ||
		record.RecoveryDecisionID != "" || event.CausationID != record.recoveryEventID ||
		payload.CandidateDigest != toolRecoveryCandidateDigest(*record) {
		return nil, fmt.Errorf("%w: invalid recovery-resolved transition", ErrInvalidExecutionEvent)
	}
	record.RecoveryDecisionID = payload.DecisionID
	record.RecoveryDecision = string(payload.Action)
	record.RecoveryResolvedAt = payload.ResolvedAt
	record.RecoveryEvidenceID = payload.EvidenceID
	record.RecoveryObservationDigest = payload.ObservationDigest
	record.RecoveryReplacementAttemptID = payload.ReplacementAttemptID
	record.RecoveryReplacementRunID = payload.ReplacementRunID
	switch payload.Action {
	case ToolRecoveryAbortAttempt:
		record.Status = "recovery_aborted"
	case ToolRecoveryAcceptObservedEffect:
		record.Status = "recovery_effect_accepted"
	case ToolRecoveryRetryInNewAttempt:
		record.Status = "recovery_retry_authorized"
	default:
		return nil, fmt.Errorf("%w: unsupported recovery resolution", ErrInvalidExecutionEvent)
	}
	return record, nil
}

type proposedPayload struct {
	JobID       string `json:"job_id"`
	ExecutionID string `json:"execution_id"`
	CallDigest  string `json:"call_digest"`
	Tool        string `json:"tool"`
	Command     string `json:"command,omitempty"`
	Path        string `json:"path,omitempty"`
	ProposedAt  string `json:"proposed_at"`
	Generation  int64  `json:"generation"`
	OperationID string `json:"operation_id"`
	JourneyID   string `json:"journey_id"`
}

type proposedPayloadV2 struct {
	JobID       string `json:"job_id"`
	ExecutionID string `json:"execution_id"`
	CallDigest  string `json:"call_digest"`
	Tool        string `json:"tool"`
	ProposedAt  string `json:"proposed_at"`
	Generation  int64  `json:"generation"`
	OperationID string `json:"operation_id"`
	JourneyID   string `json:"journey_id"`
}

type allowedPayload struct {
	ExecutionID string `json:"execution_id"`
	AllowedAt   string `json:"allowed_at"`
}

type deniedPayload struct {
	ExecutionID string             `json:"execution_id"`
	Denial      permissions.Denial `json:"denial"`
	DeniedAt    string             `json:"denied_at"`
}

type deniedPayloadV2 struct {
	ExecutionID string `json:"execution_id"`
	ReasonCode  string `json:"reason_code"`
	DeniedAt    string `json:"denied_at"`
}

type completedPayload struct {
	ExecutionID        string `json:"execution_id"`
	ExitCode           int    `json:"exit_code"`
	OutputDigest       string `json:"output_digest"`
	ChangedFilesDigest string `json:"changed_files_digest"`
	DurationMS         int64  `json:"duration_ms"`
	EvidenceID         string `json:"evidence_id"`
	CompletedAt        string `json:"completed_at"`
}

type failedPayload struct {
	ExecutionID string `json:"execution_id"`
	Reason      string `json:"reason"`
	ErrorCode   string `json:"error_code"`
	FailedAt    string `json:"failed_at"`
}

type failedPayloadV2 struct {
	ExecutionID string `json:"execution_id"`
	ErrorCode   string `json:"error_code"`
	FailedAt    string `json:"failed_at"`
}

type recoveryRequiredPayloadV2 struct {
	ExecutionID    string `json:"execution_id"`
	RecoveryCode   string `json:"recovery_code"`
	RecoveryAction string `json:"recovery_action"`
	RequiredAt     string `json:"required_at"`
}

var _ = sort.Strings
