package api

import (
	"context"

	"loom-pi-rebuild/internal/app"
)

// LocalIntegrationAPI is the thin IPC boundary over the integration service.
type LocalIntegrationAPI struct {
	service *app.LocalIntegrationService
}

func NewLocalIntegrationAPI(service *app.LocalIntegrationService) (*LocalIntegrationAPI, error) {
	if service == nil {
		return nil, app.ErrInvalidIntegrationRequest
	}
	return &LocalIntegrationAPI{service: service}, nil
}

func (api *LocalIntegrationAPI) Snapshot(ctx context.Context) (app.IntegrationSnapshot, error) {
	return api.service.ReadSnapshot(ctx)
}

func (api *LocalIntegrationAPI) Command(
	ctx context.Context,
	request app.IntegrationCommandRequest,
) (app.IntegrationCommandResult, error) {
	return api.service.CommitCommand(ctx, request)
}
