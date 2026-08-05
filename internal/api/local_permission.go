package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalPermissionAPI = errors.New("invalid local permission API")

type PermissionSnapshotRequest = app.PermissionSnapshotRequest
type PermissionSnapshot = app.PermissionSnapshot
type PermissionAttentionRequest = app.PermissionAttentionRequest
type PermissionAttention = app.PermissionAttention
type PermissionCommandRequest = app.PermissionCommandRequest
type PermissionCommandResult = app.PermissionCommandResult

type PermissionService interface {
	PermissionSnapshot(context.Context, app.PermissionSnapshotRequest) (app.PermissionSnapshot, error)
	PermissionAttention(context.Context, app.PermissionAttentionRequest) (app.PermissionAttention, error)
	PermissionCommand(context.Context, app.PermissionCommandRequest) (app.PermissionCommandResult, error)
}

type LocalPermissionAPI struct{ service PermissionService }

func NewLocalPermissionAPI(service PermissionService) (*LocalPermissionAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalPermissionAPI
	}
	return &LocalPermissionAPI{service: service}, nil
}

func (api *LocalPermissionAPI) PermissionSnapshot(
	ctx context.Context,
	request PermissionSnapshotRequest,
) (PermissionSnapshot, error) {
	if api == nil || api.service == nil {
		return PermissionSnapshot{}, ErrInvalidLocalPermissionAPI
	}
	return api.service.PermissionSnapshot(ctx, request)
}

func (api *LocalPermissionAPI) PermissionAttention(
	ctx context.Context,
	request PermissionAttentionRequest,
) (PermissionAttention, error) {
	if api == nil || api.service == nil {
		return PermissionAttention{}, ErrInvalidLocalPermissionAPI
	}
	return api.service.PermissionAttention(ctx, request)
}

func (api *LocalPermissionAPI) PermissionCommand(
	ctx context.Context,
	request PermissionCommandRequest,
) (PermissionCommandResult, error) {
	if api == nil || api.service == nil {
		return PermissionCommandResult{}, ErrInvalidLocalPermissionAPI
	}
	return api.service.PermissionCommand(ctx, request)
}
