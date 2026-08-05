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
	if event.SchemaVersion != 1 {
		return nil, fmt.Errorf("%w: schema_version=%d", ErrInvalidExecutionEvent, event.SchemaVersion)
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

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func applyProposed(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	var payload proposedPayload
	if err := decodeExecutionPayload(event, &payload); err != nil {
		return nil, err
	}
	expected := executionStreamID(payload.JobID, payload.ExecutionID)
	if event.StreamID != expected {
		return nil, fmt.Errorf("%w: stream mismatch", ErrInvalidExecutionEvent)
	}
	if payload.CallDigest == "" || payload.Tool == "" || payload.ProposedAt == "" {
		return nil, fmt.Errorf("%w: invalid proposed payload", ErrInvalidExecutionEvent)
	}
	record := getOrCreate(records, order, payload.ExecutionID, payload.JobID)
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
	record := getOrCreate(records, order, payload.ExecutionID, "")
	if payload.AllowedAt == "" {
		return nil, fmt.Errorf("%w: invalid allowed payload", ErrInvalidExecutionEvent)
	}
	record.AllowedAt = payload.AllowedAt
	record.Status = "allowed"
	return record, nil
}

func applyDenied(records map[string]*ExecutionRecord, order []string, event journal.Event) (*ExecutionRecord, error) {
	var payload deniedPayload
	if err := decodeExecutionPayload(event, &payload); err != nil {
		return nil, err
	}
	record := getOrCreate(records, order, payload.ExecutionID, "")
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
	record := getOrCreate(records, order, payload.ExecutionID, "")
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
	var payload failedPayload
	if err := decodeExecutionPayload(event, &payload); err != nil {
		return nil, err
	}
	record := getOrCreate(records, order, payload.ExecutionID, "")
	record.FailedAt = payload.FailedAt
	record.FailureReason = payload.Reason
	record.ErrorCode = payload.ErrorCode
	record.Status = "failed"
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

type allowedPayload struct {
	ExecutionID string `json:"execution_id"`
	AllowedAt   string `json:"allowed_at"`
}

type deniedPayload struct {
	ExecutionID string             `json:"execution_id"`
	Denial      permissions.Denial `json:"denial"`
	DeniedAt    string             `json:"denied_at"`
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

var _ = sort.Strings
