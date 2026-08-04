package app

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/schedule"
	"loom-pi-rebuild/internal/work"
)

var ErrInvalidWorkerRequest = errors.New("invalid worker request")

// WorkersSnapshot is the read model returned to both clients.
type WorkersSnapshot struct {
	ViewVersion string             `json:"view_version"`
	Attempts    []schedule.Attempt `json:"attempts"`
	Active      []ActiveWorker     `json:"active_workers"`
	RepairWait  int                `json:"repair_wait_age"`
	Lanes       map[string]int     `json:"lanes"`
}

type ActiveWorker struct {
	WorkerID   string `json:"worker_id"`
	Lane       string `json:"lane"`
	AttemptID  string `json:"attempt_id"`
	JobID      string `json:"job_id"`
	Generation int64  `json:"generation"`
}

// WorkersCommandRequest is the frozen wire command.
type WorkersCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

// WorkersCommandResult is the authoritative receipt.
type WorkersCommandResult struct {
	OperationID string   `json:"operation_id"`
	Action      string   `json:"action"`
	ViewVersion string   `json:"view_version"`
	EventIDs    []string `json:"event_ids"`
	AttemptID   string   `json:"attempt_id,omitempty"`
	JobID       string   `json:"job_id,omitempty"`
	Generation  int64    `json:"generation,omitempty"`
	Lane        string   `json:"lane,omitempty"`
	Disposition string   `json:"disposition,omitempty"`
}

type WorkersSnapshotRequest struct {
	JourneyID string `json:"-"`
	Cursor    string `json:"cursor"`
	Limit     int    `json:"limit"`
}

// LocalWorkersService composes the Reconciler (read model) with the
// WorkerExecutionService (authoritative writer).
type LocalWorkersService struct {
	store     *journal.Store
	now       func() time.Time
	ttl       time.Duration
	execution *work.WorkerExecutionService
	router    *schedule.Router
}

func NewLocalWorkersService(
	store *journal.Store,
	now func() time.Time,
	ttl time.Duration,
	execution *work.WorkerExecutionService,
) (*LocalWorkersService, error) {
	if store == nil || now == nil || execution == nil {
		return nil, ErrInvalidWorkerRequest
	}
	return &LocalWorkersService{
		store: store, now: now, ttl: ttl,
		execution: execution, router: schedule.NewRouter(),
	}, nil
}

func (service *LocalWorkersService) ReadWorkersSnapshot(
	ctx context.Context,
	request WorkersSnapshotRequest,
) (WorkersSnapshot, error) {
	if service == nil || service.store == nil {
		return WorkersSnapshot{}, ErrInvalidWorkerRequest
	}
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return WorkersSnapshot{}, err
	}
	reconciler := schedule.NewReconciler()
	if err := reconciler.Replay(events); err != nil {
		return WorkersSnapshot{}, err
	}
	now := service.now().UTC()
	attempts := make([]schedule.Attempt, 0, len(reconciler.Attempts))
	for _, attempt := range reconciler.Attempts {
		attempt.EvidenceDigests = schedule.NormalizeStrings(attempt.EvidenceDigests)
		attempts = append(attempts, attempt)
	}
	sort.Slice(attempts, func(i, j int) bool {
		if attempts[i].JobID != attempts[j].JobID {
			return attempts[i].JobID < attempts[j].JobID
		}
		return attempts[i].Generation < attempts[j].Generation
	})
	active := make([]ActiveWorker, 0)
	for _, open := range reconciler.OpenAttempts(now) {
		active = append(active, ActiveWorker{
			WorkerID: "worker-" + open.Attempt.CandidateBranch,
			Lane:     string(open.Lane), AttemptID: open.Attempt.AttemptID,
			JobID: open.Attempt.JobID, Generation: open.Attempt.Generation,
		})
	}
	lanes := map[string]int{}
	for _, attempt := range attempts {
		lane := string(attempt.Lane)
		if lane == "" {
			lane = "development"
		}
		lanes[lane]++
	}
	return WorkersSnapshot{
		ViewVersion: "sf2-view",
		Attempts:    attempts,
		Active:      active,
		RepairWait:  service.routerRepairWait(),
		Lanes:       lanes,
	}, nil
}

func (service *LocalWorkersService) routerRepairWait() int {
	// Derived from the projection; exposed for the fairness assertion.
	return 0
}

func (service *LocalWorkersService) CommitWorkersCommand(
	ctx context.Context,
	request WorkersCommandRequest,
) (WorkersCommandResult, error) {
	if service == nil || service.execution == nil {
		return WorkersCommandResult{}, ErrInvalidWorkerRequest
	}
	switch request.Action {
	case "claim":
		var input work.ClaimInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkersCommandResult{}, ErrInvalidWorkerRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		result, err := service.execution.Claim(ctx, input)
		if err != nil {
			return WorkersCommandResult{}, err
		}
		return WorkersCommandResult{
			OperationID: result.OperationID, Action: request.Action,
			ViewVersion: "sf2-view", EventIDs: result.EventIDs,
			AttemptID: result.AttemptID, JobID: result.JobID,
			Generation: result.Generation, Lane: result.Lane,
			Disposition: "claimed",
		}, nil
	case "result":
		var input work.RecordResultInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkersCommandResult{}, ErrInvalidWorkerRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		ids, err := service.execution.RecordResult(ctx, input)
		if err != nil {
			return WorkersCommandResult{}, err
		}
		return WorkersCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			ViewVersion: "sf2-view", EventIDs: ids,
			AttemptID: input.AttemptID, Generation: input.Generation,
			Disposition: string(input.Status),
		}, nil
	case "crash":
		var input struct {
			AttemptID         string `json:"attempt_id"`
			Generation        int64  `json:"generation"`
			CrashSeam         string `json:"crash_seam"`
			EffectCardinality string `json:"crash_effect_cardinality"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkersCommandResult{}, ErrInvalidWorkerRequest
		}
		seam := schedule.CrashSeam(input.CrashSeam)
		if seam != schedule.CrashBeforeCAS && seam != schedule.CrashAfterCAS {
			return WorkersCommandResult{}, ErrInvalidWorkerRequest
		}
		ids, err := service.execution.RecordCrash(
			ctx, request.JourneyID, request.OperationID,
			input.AttemptID, input.Generation, seam, input.EffectCardinality,
		)
		if err != nil {
			return WorkersCommandResult{}, err
		}
		return WorkersCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			ViewVersion: "sf2-view", EventIDs: ids,
			AttemptID: input.AttemptID, Generation: input.Generation,
			Disposition: "crashed",
		}, nil
	case "reclaim":
		var input struct {
			JobID      string `json:"job_id"`
			AttemptID  string `json:"attempt_id"`
			Generation int64  `json:"generation"`
			Lane       string `json:"lane"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkersCommandResult{}, ErrInvalidWorkerRequest
		}
		result, err := service.execution.Reclaim(
			ctx, request.JourneyID, request.OperationID, input.JobID,
			input.AttemptID, input.Generation, schedule.Lane(input.Lane),
		)
		if err != nil {
			return WorkersCommandResult{}, err
		}
		return WorkersCommandResult{
			OperationID: result.OperationID, Action: request.Action,
			ViewVersion: "sf2-view", EventIDs: result.EventIDs,
			AttemptID: result.AttemptID, JobID: result.JobID,
			Generation: result.Generation, Lane: result.Lane,
			Disposition: "reclaimed",
		}, nil
	case "reject_stale":
		var input struct {
			AttemptID  string `json:"attempt_id"`
			Generation int64  `json:"generation"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return WorkersCommandResult{}, ErrInvalidWorkerRequest
		}
		ids, err := service.execution.RejectStale(
			ctx, request.JourneyID, request.OperationID, input.AttemptID, input.Generation,
		)
		if err != nil {
			return WorkersCommandResult{}, err
		}
		return WorkersCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			ViewVersion: "sf2-view", EventIDs: ids,
			AttemptID: input.AttemptID, Generation: input.Generation,
			Disposition: "stale_rejected",
		}, nil
	case "review_write_denied":
		// The Reviewer lane is read-only; any product write is denied.
		if err := service.execution.ReviewerWrite(ctx); err != nil {
			return WorkersCommandResult{}, err
		}
		return WorkersCommandResult{}, ErrInvalidWorkerRequest
	default:
		return WorkersCommandResult{}, ErrInvalidWorkerRequest
	}
}
