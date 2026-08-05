package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/observability"
)

var ErrInvalidIntegrationRequest = errors.New("invalid integration request")

// IntegrationSnapshot is the read model for both clients.
type IntegrationSnapshot struct {
	ViewVersion string                         `json:"view_version"`
	Releases    []integration.ReleaseCandidate `json:"releases"`
	Canaries    []integration.CanaryRun        `json:"canaries"`
	Timeline    []integration.NodeOutputFrame  `json:"timeline"`
	Attention   []observability.AttentionItem  `json:"attention"`
}

type IntegrationCommandRequest struct {
	JourneyID   string          `json:"-"`
	OperationID string          `json:"operation_id"`
	Action      string          `json:"action"`
	Input       json.RawMessage `json:"input"`
}

type IntegrationCommandResult struct {
	OperationID string   `json:"operation_id"`
	Action      string   `json:"action"`
	EventIDs    []string `json:"event_ids"`
	ReleaseID   string   `json:"release_id,omitempty"`
	CanaryID    string   `json:"canary_id,omitempty"`
	Disposition string   `json:"disposition"`
}

// LocalIntegrationService composes the single-writer Integrator, the canary
// and the observability projections.
type LocalIntegrationService struct {
	store         *journal.Store
	integration   *integration.IntegrationService
	observability *observability.ObservabilityService
	mu            sync.Mutex
	lastSnapshot  IntegrationSnapshot
	rebuildFault  bool
}

func NewLocalIntegrationService(
	store *journal.Store,
	integrationService *integration.IntegrationService,
	observabilityService *observability.ObservabilityService,
) (*LocalIntegrationService, error) {
	if store == nil || integrationService == nil || observabilityService == nil {
		return nil, ErrInvalidIntegrationRequest
	}
	return &LocalIntegrationService{
		store: store, integration: integrationService,
		observability: observabilityService,
	}, nil
}

func (service *LocalIntegrationService) ReadSnapshot(ctx context.Context) (IntegrationSnapshot, error) {
	releases, err := service.integration.ReleaseProjection(ctx)
	if err != nil {
		return service.preservedView(err)
	}
	canaries, err := service.integration.CanaryProjection(ctx)
	if err != nil {
		return service.preservedView(err)
	}
	timeline, err := observability.BuildTimeline(ctx, service.store)
	if err != nil {
		return service.preservedView(err)
	}
	attention, err := observability.BuildAttention(ctx, service.store)
	if err != nil {
		return service.preservedView(err)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.rebuildFault {
		return service.lastSnapshot, nil
	}
	snapshot := IntegrationSnapshot{
		ViewVersion: "sf3-view",
		Releases:    normalizeSnapshotReleases(releases),
		Canaries:    normalizeSnapshotCanaries(canaries),
		Timeline:    normalizeSnapshotFrames(timeline.Frames),
		Attention:   normalizeSnapshotAttention(attention),
	}
	service.lastSnapshot = snapshot
	return snapshot, nil
}

func normalizeSnapshotReleases(values []integration.ReleaseCandidate) []integration.ReleaseCandidate {
	if values == nil {
		return []integration.ReleaseCandidate{}
	}
	return values
}

func normalizeSnapshotCanaries(values []integration.CanaryRun) []integration.CanaryRun {
	if values == nil {
		return []integration.CanaryRun{}
	}
	return values
}

func normalizeSnapshotFrames(values []integration.NodeOutputFrame) []integration.NodeOutputFrame {
	if values == nil {
		return []integration.NodeOutputFrame{}
	}
	return values
}

func normalizeSnapshotAttention(values []observability.AttentionItem) []observability.AttentionItem {
	if values == nil {
		return []observability.AttentionItem{}
	}
	return values
}

// preservedView returns the last-good view (or the rebuild error when no
// view has ever been built) so a failed projection refresh never discards
// the old view.
func (service *LocalIntegrationService) preservedView(rebuildErr error) (IntegrationSnapshot, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.lastSnapshot.ViewVersion != "" {
		return service.lastSnapshot, nil
	}
	return IntegrationSnapshot{}, rebuildErr
}

// SetProjectionFault toggles the controlled projection-failure seam used by
// the canary journey: while active, the old view is preserved.
func (service *LocalIntegrationService) SetProjectionFault(active bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.rebuildFault = active
}

func (service *LocalIntegrationService) CommitCommand(
	ctx context.Context,
	request IntegrationCommandRequest,
) (IntegrationCommandResult, error) {
	if service == nil {
		return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
	}
	switch request.Action {
	case "integrate":
		var input integration.IntegrateInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		result, err := service.integration.Integrate(ctx, input)
		if err != nil {
			return IntegrationCommandResult{}, err
		}
		return IntegrationCommandResult{
			OperationID: result.OperationID, Action: request.Action,
			EventIDs: result.EventIDs, ReleaseID: result.ReleaseID,
			Disposition: result.Disposition,
		}, nil
	case "start_canary":
		var input integration.CanaryInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		result, err := service.integration.StartCanary(ctx, input)
		if err != nil {
			return IntegrationCommandResult{}, err
		}
		return IntegrationCommandResult{
			OperationID: result.OperationID, Action: request.Action,
			EventIDs: result.EventIDs, CanaryID: result.CanaryID,
			Disposition: result.Disposition,
		}, nil
	case "adopt":
		var input integration.AdoptInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		result, err := service.integration.AdoptByLaterRun(ctx, input)
		if err != nil {
			return IntegrationCommandResult{}, err
		}
		return IntegrationCommandResult{
			OperationID: result.OperationID, Action: request.Action,
			EventIDs: result.EventIDs, ReleaseID: result.ReleaseID,
			Disposition: result.Disposition,
		}, nil
	case "rollback":
		var input integration.RollbackInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		result, err := service.integration.Rollback(ctx, input)
		if err != nil {
			return IntegrationCommandResult{}, err
		}
		return IntegrationCommandResult{
			OperationID: result.OperationID, Action: request.Action,
			EventIDs: result.EventIDs, ReleaseID: result.ReleaseID,
			Disposition: result.Disposition,
		}, nil
	case "publish_frame":
		var input observability.FrameInput
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
		}
		input.JourneyID = request.JourneyID
		input.OperationID = request.OperationID
		ids, err := service.observability.PublishFrame(ctx, input)
		if err != nil {
			return IntegrationCommandResult{}, err
		}
		return IntegrationCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			EventIDs: ids, Disposition: "published",
		}, nil
	case "projection_failure_test":
		var input struct {
			Active bool `json:"active"`
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
		}
		service.SetProjectionFault(input.Active)
		return IntegrationCommandResult{
			OperationID: request.OperationID, Action: request.Action,
			EventIDs: []string{}, Disposition: "projection_fault_set",
		}, nil
	default:
		return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
	}
}
