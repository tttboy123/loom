package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalQueueAPI = errors.New("invalid local queue API")

type QueueSnapshotRequest = app.QueueSnapshotRequest
type QueueSnapshot = app.QueueSnapshot
type QueueCommandRequest = app.QueueCommandRequest
type QueueCommandResult = app.QueueCommandResult

type QueueService interface {
	ReadQueueSnapshot(context.Context, app.QueueSnapshotRequest) (app.QueueSnapshot, error)
	CommitQueueCommand(context.Context, app.QueueCommandRequest) (app.QueueCommandResult, error)
}

type LocalQueueAPI struct{ service QueueService }

func NewLocalQueueAPI(service QueueService) (*LocalQueueAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalQueueAPI
	}
	return &LocalQueueAPI{service: service}, nil
}

func (api *LocalQueueAPI) QueueSnapshot(
	ctx context.Context,
	request QueueSnapshotRequest,
) (QueueSnapshot, error) {
	if api == nil || api.service == nil {
		return QueueSnapshot{}, ErrInvalidLocalQueueAPI
	}
	return api.service.ReadQueueSnapshot(ctx, request)
}

func (api *LocalQueueAPI) QueueCommand(
	ctx context.Context,
	request QueueCommandRequest,
) (QueueCommandResult, error) {
	if api == nil || api.service == nil {
		return QueueCommandResult{}, ErrInvalidLocalQueueAPI
	}
	return api.service.CommitQueueCommand(ctx, request)
}
