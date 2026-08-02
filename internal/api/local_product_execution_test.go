package api

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"loom-pi-rebuild/internal/app"
)

type localProductExecutionServiceStub struct {
	preflight app.MissionExecutionPreflight
	result    app.MissionExecutionResult
	err       error
	calls     []string
}

func (stub *localProductExecutionServiceStub) PreflightMission(
	ctx context.Context,
	command app.MissionExecutionCommand,
) (app.MissionExecutionPreflight, error) {
	stub.calls = append(stub.calls, command.Operation)
	return stub.preflight, stub.err
}

func (stub *localProductExecutionServiceStub) StartMission(
	ctx context.Context,
	command app.MissionExecutionCommand,
) (app.MissionExecutionResult, error) {
	stub.calls = append(stub.calls, command.Operation)
	return stub.result, stub.err
}

func (stub *localProductExecutionServiceStub) ControlMission(
	ctx context.Context,
	command app.MissionExecutionCommand,
) (app.MissionExecutionResult, error) {
	stub.calls = append(stub.calls, command.Operation)
	return stub.result, stub.err
}

func TestLocalProductExecutionAPIReturnsClosedOperationEnvelope(t *testing.T) {
	preflight := app.MissionExecutionPreflight{
		SchemaVersion: app.MissionExecutionSchemaVersion,
		MissionID:     "mission-1",
	}
	result := app.MissionExecutionResult{
		SchemaVersion: app.MissionExecutionSchemaVersion,
		MissionID:     "mission-1",
		Status:        "running",
	}
	service := &localProductExecutionServiceStub{
		preflight: preflight,
		result:    result,
	}
	productAPI, err := NewLocalProductExecutionAPI(service)
	if err != nil {
		t.Fatal(err)
	}

	preflightEnvelope, err := productAPI.ExecuteMission(
		context.Background(),
		app.MissionExecutionCommand{Operation: "preflight"},
	)
	if err != nil || preflightEnvelope.Operation != "preflight" ||
		preflightEnvelope.Preflight == nil ||
		!reflect.DeepEqual(*preflightEnvelope.Preflight, preflight) ||
		preflightEnvelope.Result != nil {
		t.Fatalf("preflight envelope = %#v, %v", preflightEnvelope, err)
	}

	for _, operation := range []string{"start", "control"} {
		envelope, executeErr := productAPI.ExecuteMission(
			context.Background(),
			app.MissionExecutionCommand{Operation: operation},
		)
		if executeErr != nil || envelope.Operation != operation ||
			envelope.Preflight != nil || envelope.Result == nil ||
			!reflect.DeepEqual(*envelope.Result, result) {
			t.Fatalf("%s envelope = %#v, %v", operation, envelope, executeErr)
		}
	}
	if !reflect.DeepEqual(service.calls, []string{"preflight", "start", "control"}) {
		t.Fatalf("calls = %#v", service.calls)
	}
}

func TestLocalProductExecutionAPIRejectsUnknownOperationAndNilService(
	t *testing.T,
) {
	if _, err := NewLocalProductExecutionAPI(nil); !errors.Is(
		err,
		ErrInvalidLocalProductExecutionAPI,
	) {
		t.Fatalf("nil service error = %v", err)
	}
	productAPI, err := NewLocalProductExecutionAPI(
		&localProductExecutionServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := productAPI.ExecuteMission(
		context.Background(),
		app.MissionExecutionCommand{Operation: "execute"},
	); !errors.Is(err, app.ErrInvalidMissionExecution) {
		t.Fatalf("unknown operation error = %v", err)
	}
	if len(productAPI.service.(*localProductExecutionServiceStub).calls) != 0 {
		t.Fatal("unknown operation reached service")
	}
}
