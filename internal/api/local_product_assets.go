package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalProductAssetAPI = errors.New("invalid local product asset API")

type EvolutionAssetSnapshotRequest = app.EvolutionAssetSnapshotRequest
type EvolutionAssetSnapshot = app.EvolutionAssetSnapshot
type EvolutionAssetDiffRequest = app.EvolutionAssetDiffRequest
type EvolutionAssetDiff = app.EvolutionAssetDiff
type EvolutionAssetCommandRequest = app.EvolutionAssetCommandRequest
type EvolutionAssetCommandResult = app.EvolutionAssetCommandResult

type EvolutionAssetService interface {
	ReadEvolutionAssetSnapshot(context.Context, app.EvolutionAssetSnapshotRequest) (app.EvolutionAssetSnapshot, error)
	DiffEvolutionAssetRevisions(context.Context, app.EvolutionAssetDiffRequest) (app.EvolutionAssetDiff, error)
	CommitEvolutionAssetCommand(context.Context, app.EvolutionAssetCommandRequest) (app.EvolutionAssetCommandResult, error)
}
type LocalProductAssetAPI struct{ service EvolutionAssetService }

func NewLocalProductAssetAPI(service EvolutionAssetService) (*LocalProductAssetAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalProductAssetAPI
	}
	return &LocalProductAssetAPI{service: service}, nil
}
func (api *LocalProductAssetAPI) EvolutionAssetSnapshot(ctx context.Context, request EvolutionAssetSnapshotRequest) (EvolutionAssetSnapshot, error) {
	if api == nil || api.service == nil {
		return EvolutionAssetSnapshot{}, ErrInvalidLocalProductAssetAPI
	}
	return api.service.ReadEvolutionAssetSnapshot(ctx, request)
}
func (api *LocalProductAssetAPI) EvolutionAssetDiff(ctx context.Context, request EvolutionAssetDiffRequest) (EvolutionAssetDiff, error) {
	if api == nil || api.service == nil {
		return EvolutionAssetDiff{}, ErrInvalidLocalProductAssetAPI
	}
	return api.service.DiffEvolutionAssetRevisions(ctx, request)
}
func (api *LocalProductAssetAPI) EvolutionAssetCommand(ctx context.Context, request EvolutionAssetCommandRequest) (EvolutionAssetCommandResult, error) {
	if api == nil || api.service == nil {
		return EvolutionAssetCommandResult{}, ErrInvalidLocalProductAssetAPI
	}
	return api.service.CommitEvolutionAssetCommand(ctx, request)
}
