package tui

import (
	"context"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
	"strings"
)

func workersClientFrom(client ReadClient) WorkersClient {
	workersClient, _ := client.(WorkersClient)
	return workersClient
}

func (client *DaemonReadClient) WorkersSnapshot(
	ctx context.Context,
	request app.WorkersSnapshotRequest,
) (app.WorkersSnapshot, error) {
	var snapshot app.WorkersSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "workers_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) WorkersCommand(
	ctx context.Context,
	request app.WorkersCommandRequest,
) (app.WorkersCommandResult, error) {
	var result app.WorkersCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "workers_command", request, &result,
	)
	return result, err
}

func (model Model) loadWorkers() tea.Cmd {
	client, ctx, journeyID := model.workersClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return workersFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.WorkersSnapshot(ctx, app.WorkersSnapshotRequest{
			JourneyID: journeyID, Limit: 64,
		})
		if err != nil {
			return workersFailedMsg{err: err}
		}
		return workersLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) renderWorkersView() string {
	lines := []string{
		"Worker Pools · lease / generation fencing",
		"r refresh · q quit",
	}
	if model.workersClient == nil {
		lines = append(lines, "Workers service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if len(model.workersSnapshot.Active) == 0 && len(model.workersSnapshot.Attempts) == 0 {
		lines = append(lines, "No worker attempts yet.")
	}
	for _, active := range model.workersSnapshot.Active {
		lines = append(lines, fmt.Sprintf(
			"› worker %s · lane %s · attempt %s · job %s · gen %d",
			sanitizeCell(active.WorkerID, 24),
			humanizeStatus(active.Lane),
			sanitizeCell(active.AttemptID, 32),
			sanitizeCell(active.JobID, 20),
			active.Generation,
		))
	}
	for _, attempt := range model.workersSnapshot.Attempts {
		status := string(attempt.Status)
		if attempt.Status == "" {
			status = "claimed"
		}
		lines = append(lines, fmt.Sprintf(
			"  %s · lane %s · gen %d · %s · seam %s",
			sanitizeCell(attempt.AttemptID, 32),
			humanizeStatus(string(attempt.Lane)),
			attempt.Generation,
			humanizeStatus(status),
			attempt.CrashSeam,
		))
	}
	if model.workersSnapshot.RepairWait > 0 {
		lines = append(lines, fmt.Sprintf("Repair wait age: %d", model.workersSnapshot.RepairWait))
	}
	return strings.Join(lines, "\n") + "\n"
}
