package api

import (
	"context"
	"errors"
	"reflect"

	"loom-pi-rebuild/internal/app"
)

var ErrInvalidLocalProductExecutionAPI = errors.New(
	"invalid local product execution API",
)

type MissionExecutionService interface {
	PreflightMission(
		context.Context,
		app.MissionExecutionCommand,
	) (app.MissionExecutionPreflight, error)
	StartMission(
		context.Context,
		app.MissionExecutionCommand,
	) (app.MissionExecutionResult, error)
	ControlMission(
		context.Context,
		app.MissionExecutionCommand,
	) (app.MissionExecutionResult, error)
}

type MissionExecutionEnvelope struct {
	SchemaVersion int                            `json:"schema_version"`
	Operation     string                         `json:"operation"`
	Preflight     *app.MissionExecutionPreflight `json:"preflight,omitempty"`
	Result        *app.MissionExecutionResult    `json:"result,omitempty"`
}

type LocalProductExecutionAPI struct {
	service MissionExecutionService
}

func NewLocalProductExecutionAPI(
	service MissionExecutionService,
) (*LocalProductExecutionAPI, error) {
	if nilMissionExecutionService(service) {
		return nil, ErrInvalidLocalProductExecutionAPI
	}
	return &LocalProductExecutionAPI{service: service}, nil
}

func (api *LocalProductExecutionAPI) ExecuteMission(
	ctx context.Context,
	command app.MissionExecutionCommand,
) (MissionExecutionEnvelope, error) {
	if api == nil || nilMissionExecutionService(api.service) {
		return MissionExecutionEnvelope{}, ErrInvalidLocalProductExecutionAPI
	}
	if err := ctx.Err(); err != nil {
		return MissionExecutionEnvelope{}, err
	}
	envelope := MissionExecutionEnvelope{
		SchemaVersion: app.MissionExecutionSchemaVersion,
		Operation:     command.Operation,
	}
	switch command.Operation {
	case "preflight":
		preflight, err := api.service.PreflightMission(ctx, command)
		if err != nil {
			return MissionExecutionEnvelope{}, err
		}
		envelope.Preflight = &preflight
		return envelope, nil
	case "start":
		result, err := api.service.StartMission(ctx, command)
		if err != nil {
			return MissionExecutionEnvelope{}, err
		}
		envelope.Result = &result
		return envelope, nil
	case "control":
		result, err := api.service.ControlMission(ctx, command)
		if err != nil {
			return MissionExecutionEnvelope{}, err
		}
		envelope.Result = &result
		return envelope, nil
	default:
		return MissionExecutionEnvelope{}, app.ErrInvalidMissionExecution
	}
}

func nilMissionExecutionService(value MissionExecutionService) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
