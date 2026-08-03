package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalProductHandoffAPI = errors.New("invalid local product handoff API")

type SideTaskHandoffService interface {
	ProposeSideTask(context.Context, app.SideTaskProposalRequest) (app.SideTaskProposalResult, error)
	CreateSideTask(context.Context, app.SideTaskCreateRequest) (app.SideTaskCreateResult, error)
	ReadSideTask(context.Context, app.SideTaskReadRequest) (app.SideTaskReadResult, error)
	DecideSideTask(context.Context, app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error)
}

type LocalProductHandoffAPI struct{ service SideTaskHandoffService }

func NewLocalProductHandoffAPI(service SideTaskHandoffService) (*LocalProductHandoffAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalProductHandoffAPI
	}
	return &LocalProductHandoffAPI{service: service}, nil
}

func (api *LocalProductHandoffAPI) ProposeSideTask(ctx context.Context, request app.SideTaskProposalRequest) (app.SideTaskProposalResult, error) {
	if api == nil || api.service == nil {
		return app.SideTaskProposalResult{}, ErrInvalidLocalProductHandoffAPI
	}
	return api.service.ProposeSideTask(ctx, request)
}

func (api *LocalProductHandoffAPI) CreateSideTask(ctx context.Context, request app.SideTaskCreateRequest) (app.SideTaskCreateResult, error) {
	if api == nil || api.service == nil {
		return app.SideTaskCreateResult{}, ErrInvalidLocalProductHandoffAPI
	}
	return api.service.CreateSideTask(ctx, request)
}

func (api *LocalProductHandoffAPI) ReadSideTask(ctx context.Context, request app.SideTaskReadRequest) (app.SideTaskReadResult, error) {
	if api == nil || api.service == nil {
		return app.SideTaskReadResult{}, ErrInvalidLocalProductHandoffAPI
	}
	return api.service.ReadSideTask(ctx, request)
}

func (api *LocalProductHandoffAPI) DecideSideTask(ctx context.Context, request app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error) {
	if api == nil || api.service == nil {
		return app.SideTaskDecisionResult{}, ErrInvalidLocalProductHandoffAPI
	}
	return api.service.DecideSideTask(ctx, request)
}
