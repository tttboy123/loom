package api_test

import (
	"context"
	"errors"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
)

func TestLocalQueueAPIRejectsNilService(t *testing.T) {
	if _, err := api.NewLocalQueueAPI(nil); !errors.Is(err, api.ErrInvalidLocalQueueAPI) {
		t.Fatalf("NewLocalQueueAPI(nil) = %v, want ErrInvalidLocalQueueAPI", err)
	}
}

type stubQueueService struct {
	snapshot app.QueueSnapshot
	result   app.QueueCommandResult
}

func (service *stubQueueService) ReadQueueSnapshot(
	context.Context,
	app.QueueSnapshotRequest,
) (app.QueueSnapshot, error) {
	return service.snapshot, nil
}

func (service *stubQueueService) CommitQueueCommand(
	context.Context,
	app.QueueCommandRequest,
) (app.QueueCommandResult, error) {
	return service.result, nil
}

func TestLocalQueueAPIPassesThrough(t *testing.T) {
	stub := &stubQueueService{
		snapshot: app.QueueSnapshot{ViewVersion: "v1"},
		result:   app.QueueCommandResult{OperationID: "op-1", Action: "create_job"},
	}
	queueAPI, err := api.NewLocalQueueAPI(stub)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := queueAPI.QueueSnapshot(context.Background(), app.QueueSnapshotRequest{})
	if err != nil || snapshot.ViewVersion != "v1" {
		t.Fatalf("snapshot = %+v, %v", snapshot, err)
	}
	result, err := queueAPI.QueueCommand(context.Background(), app.QueueCommandRequest{OperationID: "op-1"})
	if err != nil || result.Action != "create_job" {
		t.Fatalf("result = %+v, %v", result, err)
	}
}
