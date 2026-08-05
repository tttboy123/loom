package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

var (
	ErrInvalidExecutionRequest = errors.New("invalid local execution request")
	ErrExecutionUnavailable    = errors.New("execution service unavailable")
)

type ExecutionSnapshotRequest struct {
	JourneyID string `json:"-"`
}

type ExecutionSnapshot struct {
	ViewVersion string                    `json:"view_version"`
	Records     []execution.ExecutionRecord `json:"records"`
}

type ExecutionCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

type ExecutionCommandResult struct {
	OperationID string                   `json:"operation_id"`
	Action      string                   `json:"action"`
	ViewVersion string                   `json:"view_version"`
	Result      execution.ExecutionResult `json:"result,omitempty"`
	EventIDs    []string                 `json:"event_ids,omitempty"`
	Note        string                   `json:"note,omitempty"`
}

// LocalExecutionService is the product service for the bounded execution
// adapter. It owns the WorktreeResolver (from AttemptClaimed facts) and
// forwards proposals to the adapter; authorization facts are never written
// here.
type LocalExecutionService struct {
	store       *journal.Store
	now         func() time.Time
	viewVersion func() string
	adapter     *execution.Adapter
	resolver    *journalWorktreeResolver
}

func NewLocalExecutionService(
	store *journal.Store,
	now func() time.Time,
	viewVersion func() string,
	adapter *execution.Adapter,
) (*LocalExecutionService, error) {
	if store == nil || now == nil || viewVersion == nil || adapter == nil {
		return nil, ErrInvalidExecutionRequest
	}
	return &LocalExecutionService{
		store: store, now: now, viewVersion: viewVersion,
		adapter: adapter, resolver: &journalWorktreeResolver{store: store},
	}, nil
}

func (service *LocalExecutionService) ExecutionSnapshot(
	ctx context.Context,
	request ExecutionSnapshotRequest,
) (ExecutionSnapshot, error) {
	if service == nil || service.store == nil {
		return ExecutionSnapshot{}, ErrInvalidExecutionRequest
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return ExecutionSnapshot{}, err
	}
	snapshot, err := execution.ReplaySnapshot(events)
	if err != nil {
		return ExecutionSnapshot{}, err
	}
	records := append([]execution.ExecutionRecord(nil), snapshot.Records...)
	sort.Slice(records, func(left, right int) bool {
		return records[left].ProposedAt < records[right].ProposedAt
	})
	return ExecutionSnapshot{
		ViewVersion: service.viewVersion(),
		Records:     records,
	}, nil
}

func (service *LocalExecutionService) ExecutionCommand(
	ctx context.Context,
	request ExecutionCommandRequest,
) (ExecutionCommandResult, error) {
	if service == nil || service.adapter == nil {
		return ExecutionCommandResult{}, ErrExecutionUnavailable
	}
	if request.OperationID == "" || request.Action == "" {
		return ExecutionCommandResult{}, ErrInvalidExecutionRequest
	}
	switch request.Action {
	case "propose":
		var input struct {
			JobID string                   `json:"job_id"`
			Call  permissions.ProposedCall `json:"call"`
		}
		if err := decodeExactExecutionParams(request.Input, &input); err != nil {
			return ExecutionCommandResult{}, ErrInvalidExecutionRequest
		}
		result, err := service.adapter.Execute(ctx, execution.Proposal{
			JobID: input.JobID, Call: input.Call,
			OperationID: request.OperationID, JourneyID: request.JourneyID,
		})
		if err != nil {
			return ExecutionCommandResult{}, err
		}
		return ExecutionCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			ViewVersion: service.viewVersion(), Result: result,
			Note: result.Note,
		}, nil
	default:
		return ExecutionCommandResult{}, fmt.Errorf("%w: unknown action %q", ErrInvalidExecutionRequest, request.Action)
	}
}

// journalWorktreeResolver resolves a Queue Job to the candidate worktree of
// its most recently claimed attempt (from AttemptClaimed facts).
type journalWorktreeResolver struct {
	store *journal.Store
}

func (r *journalWorktreeResolver) Resolve(ctx context.Context, jobID string) (string, error) {
	if r == nil || r.store == nil || jobID == "" {
		return "", ErrInvalidExecutionRequest
	}
	events, err := r.store.ReadAll(ctx)
	if err != nil {
		return "", err
	}
	type claim struct {
		jobID     string
		worktree  string
		sequence  int64
	}
	var latest claim
	found := false
	for _, event := range events {
		if event.Type != "AttemptClaimed" {
			continue
		}
		var payload struct {
			JobID             string `json:"job_id"`
			CandidateWorktree string `json:"candidate_worktree"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil || payload.JobID != jobID {
			continue
		}
		if !found || event.Seq > latest.sequence {
			latest = claim{jobID: payload.JobID, worktree: payload.CandidateWorktree, sequence: event.Seq}
			found = true
		}
	}
	if !found || latest.worktree == "" {
		return "", fmt.Errorf("%w: no claimed worktree for job %s", ErrInvalidExecutionRequest, jobID)
	}
	return latest.worktree, nil
}

func decodeExactExecutionParams(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
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
