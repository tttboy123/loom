package app

import (
	"context"
	"encoding/json"
	"errors"

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
		return IntegrationSnapshot{}, err
	}
	canaries, err := service.integration.CanaryProjection(ctx)
	if err != nil {
		return IntegrationSnapshot{}, err
	}
	timeline, err := observability.BuildTimeline(ctx, service.store)
	if err != nil {
		return IntegrationSnapshot{}, err
	}
	attention, err := observability.BuildAttention(ctx, service.store)
	if err != nil {
		return IntegrationSnapshot{}, err
	}
	return IntegrationSnapshot{
		ViewVersion: "sf3-view",
		Releases:    releases, Canaries: canaries,
		Timeline: timeline.Frames, Attention: attention,
	}, nil
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
	default:
		return IntegrationCommandResult{}, ErrInvalidIntegrationRequest
	}
}
