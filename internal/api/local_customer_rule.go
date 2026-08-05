package api

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalCustomerRuleAPI = errors.New("invalid local customer rule API")

type CustomerRuleSnapshotRequest = app.CustomerRuleSnapshotRequest
type CustomerRuleSnapshot = app.CustomerRuleSnapshot
type CustomerRuleCommandRequest = app.CustomerRuleCommandRequest
type CustomerRuleCommandResult = app.CustomerRuleCommandResult

type CustomerRuleService interface {
	Snapshot(context.Context, app.CustomerRuleSnapshotRequest) (app.CustomerRuleSnapshot, error)
	Command(context.Context, app.CustomerRuleCommandRequest) (app.CustomerRuleCommandResult, error)
}

type LocalCustomerRuleAPI struct{ service CustomerRuleService }

func NewLocalCustomerRuleAPI(service CustomerRuleService) (*LocalCustomerRuleAPI, error) {
	if service == nil {
		return nil, ErrInvalidLocalCustomerRuleAPI
	}
	return &LocalCustomerRuleAPI{service: service}, nil
}

func (api *LocalCustomerRuleAPI) Snapshot(
	ctx context.Context,
	request CustomerRuleSnapshotRequest,
) (CustomerRuleSnapshot, error) {
	if api == nil || api.service == nil {
		return CustomerRuleSnapshot{}, ErrInvalidLocalCustomerRuleAPI
	}
	return api.service.Snapshot(ctx, request)
}

func (api *LocalCustomerRuleAPI) Command(
	ctx context.Context,
	request CustomerRuleCommandRequest,
) (CustomerRuleCommandResult, error) {
	if api == nil || api.service == nil {
		return CustomerRuleCommandResult{}, ErrInvalidLocalCustomerRuleAPI
	}
	return api.service.Command(ctx, request)
}
