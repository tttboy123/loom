package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalProductDecisionAPI = errors.New(
	"invalid local product decision API",
)

type MissionDecisionService interface {
	ReadMissionDecision(
		context.Context,
		app.MissionDecisionCommand,
	) (app.MissionDecisionSheet, error)
	DecideMission(
		context.Context,
		app.MissionDecisionCommand,
	) (app.MissionDecisionResult, error)
}

func (api *LocalProductDecisionAPI) ReadMissionDecision(
	ctx context.Context,
	command app.MissionDecisionCommand,
) (app.MissionDecisionSheet, error) {
	if api == nil || api.service == nil {
		return app.MissionDecisionSheet{}, ErrInvalidLocalProductDecisionAPI
	}
	return api.service.ReadMissionDecision(ctx, command)
}

type LocalProductDecisionAPI struct {
	service MissionDecisionService
}

func NewLocalProductDecisionAPI(
	service MissionDecisionService,
) (*LocalProductDecisionAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalProductDecisionAPI
	}
	return &LocalProductDecisionAPI{service: service}, nil
}

func (api *LocalProductDecisionAPI) DecideMission(
	ctx context.Context,
	command app.MissionDecisionCommand,
) (app.MissionDecisionResult, error) {
	if api == nil || api.service == nil {
		return app.MissionDecisionResult{}, ErrInvalidLocalProductDecisionAPI
	}
	return api.service.DecideMission(ctx, command)
}
