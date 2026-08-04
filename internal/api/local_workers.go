package api

import (
	"context"

	"loom-pi-rebuild/internal/app"
)

// LocalWorkersAPI is the thin IPC boundary over the workers service. It adds
// no authority.
type LocalWorkersAPI struct {
	service *app.LocalWorkersService
}

func NewLocalWorkersAPI(service *app.LocalWorkersService) (*LocalWorkersAPI, error) {
	if service == nil {
		return nil, app.ErrInvalidWorkerRequest
	}
	return &LocalWorkersAPI{service: service}, nil
}

func (api *LocalWorkersAPI) WorkersSnapshot(
	ctx context.Context,
	request app.WorkersSnapshotRequest,
) (app.WorkersSnapshot, error) {
	return api.service.ReadWorkersSnapshot(ctx, request)
}

func (api *LocalWorkersAPI) WorkersCommand(
	ctx context.Context,
	request app.WorkersCommandRequest,
) (app.WorkersCommandResult, error) {
	return api.service.CommitWorkersCommand(ctx, request)
}
