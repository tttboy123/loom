package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/execution"
)

type stubBoundedExecutionClient struct {
	snapshot app.ExecutionSnapshot
	result   app.ExecutionCommandResult
}

func (client *stubBoundedExecutionClient) ExecutionSnapshot(
	context.Context,
	app.ExecutionSnapshotRequest,
) (app.ExecutionSnapshot, error) {
	return client.snapshot, nil
}

func (client *stubBoundedExecutionClient) ExecutionCommand(
	context.Context,
	app.ExecutionCommandRequest,
) (app.ExecutionCommandResult, error) {
	return client.result, nil
}

type stubExecutionReadClient struct {
	execution BoundedExecutionClient
}

func (client *stubExecutionReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return api.LocalProductSnapshot{}, nil
}

func (client *stubExecutionReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, nil
}

var _ ReadClient = (*stubExecutionReadClient)(nil)

func TestExecutionScreenRendersRecords(t *testing.T) {
	executionClient := &stubBoundedExecutionClient{
		snapshot: app.ExecutionSnapshot{
			ViewVersion: "v1",
			Records: []execution.ExecutionRecord{
				{
					ExecutionID: "exec-1", JobID: "job-a", Tool: "Bash",
					Command: "printf hi", Status: "completed", ProposedAt: "2026-08-05T12:00:00Z",
				},
			},
		},
	}
	model, err := newModelWithContext(context.Background(), &stubExecutionReadClient{execution: executionClient})
	if err != nil {
		t.Fatal(err)
	}
	model.boundedExecutionClient = executionClient
	model.executionSnapshot = executionClient.snapshot
	model.screenIndex = indexOfScreen(ScreenExecution)
	body := model.renderExecutionsView()
	for _, want := range []string{"Execution", "exec-1", "job-a", "Completed"} {
		if !strings.Contains(body, want) {
			t.Fatalf("execution view missing %q:\n%s", want, body)
		}
	}
}

var _ = tea.Quit
