package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/production"
)

var ErrInvalidLocalProductionAPI = errors.New("invalid local production API")

type ProductionSnapshotRequest = app.ProductionSnapshotRequest
type ProductionSnapshot = production.ProductionSnapshot
type ProductionCommandRequest = app.ProductionCommandRequest
type ProductionCommandResult = app.ProductionCommandResult

type ProductionService interface {
	ProductionSnapshot(context.Context, app.ProductionSnapshotRequest) (production.ProductionSnapshot, error)
	ProductionCommand(context.Context, app.ProductionCommandRequest) (app.ProductionCommandResult, error)
	Degraded(context.Context) bool
}

type LocalProductionAPI struct{ service ProductionService }

func NewLocalProductionAPI(service ProductionService) (*LocalProductionAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalProductionAPI
	}
	return &LocalProductionAPI{service: service}, nil
}

func (api *LocalProductionAPI) ProductionSnapshot(
	ctx context.Context,
	request ProductionSnapshotRequest,
) (ProductionSnapshot, error) {
	if api == nil || api.service == nil {
		return ProductionSnapshot{}, ErrInvalidLocalProductionAPI
	}
	return api.service.ProductionSnapshot(ctx, request)
}

func (api *LocalProductionAPI) ProductionCommand(
	ctx context.Context,
	request ProductionCommandRequest,
) (ProductionCommandResult, error) {
	if api == nil || api.service == nil {
		return ProductionCommandResult{}, ErrInvalidLocalProductionAPI
	}
	return api.service.ProductionCommand(ctx, request)
}

func (api *LocalProductionAPI) Degraded(ctx context.Context) bool {
	if api == nil || api.service == nil {
		return true
	}
	return api.service.Degraded(ctx)
}
