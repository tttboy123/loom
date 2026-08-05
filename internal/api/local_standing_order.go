package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalStandingOrderAPI = errors.New("invalid local standing order API")

type StandingOrderSnapshotRequest = app.StandingOrderSnapshotRequest
type StandingOrderSnapshot = app.StandingOrderSnapshot
type StandingOrderCommandRequest = app.StandingOrderCommandRequest
type StandingOrderCommandResult = app.StandingOrderCommandResult

type StandingOrderService interface {
	Snapshot(context.Context, app.StandingOrderSnapshotRequest) (app.StandingOrderSnapshot, error)
	Command(context.Context, app.StandingOrderCommandRequest) (app.StandingOrderCommandResult, error)
}

type LocalStandingOrderAPI struct {
	service StandingOrderService
}

func NewLocalStandingOrderAPI(service StandingOrderService) (*LocalStandingOrderAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalStandingOrderAPI
	}
	return &LocalStandingOrderAPI{service: service}, nil
}

func (api *LocalStandingOrderAPI) Snapshot(
	ctx context.Context,
	request StandingOrderSnapshotRequest,
) (StandingOrderSnapshot, error) {
	if api == nil || api.service == nil {
		return StandingOrderSnapshot{}, ErrInvalidLocalStandingOrderAPI
	}
	return api.service.Snapshot(ctx, request)
}

func (api *LocalStandingOrderAPI) Command(
	ctx context.Context,
	request StandingOrderCommandRequest,
) (StandingOrderCommandResult, error) {
	if api == nil || api.service == nil {
		return StandingOrderCommandResult{}, ErrInvalidLocalStandingOrderAPI
	}
	return api.service.Command(ctx, request)
}
