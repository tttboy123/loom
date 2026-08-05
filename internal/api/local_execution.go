package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalExecutionAPI = errors.New("invalid local execution API")

type ExecutionSnapshotRequest = app.ExecutionSnapshotRequest
type ExecutionSnapshot = app.ExecutionSnapshot
type ExecutionCommandRequest = app.ExecutionCommandRequest
type ExecutionCommandResult = app.ExecutionCommandResult

type ExecutionService interface {
	ExecutionSnapshot(context.Context, app.ExecutionSnapshotRequest) (app.ExecutionSnapshot, error)
	ExecutionCommand(context.Context, app.ExecutionCommandRequest) (app.ExecutionCommandResult, error)
}

type LocalExecutionAPI struct{ service ExecutionService }

func NewLocalExecutionAPI(service ExecutionService) (*LocalExecutionAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalExecutionAPI
	}
	return &LocalExecutionAPI{service: service}, nil
}

func (api *LocalExecutionAPI) ExecutionSnapshot(
	ctx context.Context,
	request ExecutionSnapshotRequest,
) (ExecutionSnapshot, error) {
	if api == nil || api.service == nil {
		return ExecutionSnapshot{}, ErrInvalidLocalExecutionAPI
	}
	return api.service.ExecutionSnapshot(ctx, request)
}

func (api *LocalExecutionAPI) ExecutionCommand(
	ctx context.Context,
	request ExecutionCommandRequest,
) (ExecutionCommandResult, error) {
	if api == nil || api.service == nil {
		return ExecutionCommandResult{}, ErrInvalidLocalExecutionAPI
	}
	return api.service.ExecutionCommand(ctx, request)
}
