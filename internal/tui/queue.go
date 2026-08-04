package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

// queueClientFrom returns the read client's QueueClient surface, or nil.
func queueClientFrom(client ReadClient) QueueClient {
	queueClient, _ := client.(QueueClient)
	return queueClient
}

func (client *DaemonReadClient) QueueSnapshot(
	ctx context.Context,
	request api.QueueSnapshotRequest,
) (api.QueueSnapshot, error) {
	var snapshot api.QueueSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "queue_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) QueueCommand(
	ctx context.Context,
	request api.QueueCommandRequest,
) (api.QueueCommandResult, error) {
	var result api.QueueCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "queue_command", request, &result,
	)
	return result, err
}

func (model Model) loadQueue() tea.Cmd {
	client, ctx, journeyID := model.queueClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return queueFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.QueueSnapshot(ctx, api.QueueSnapshotRequest{
			JourneyID: journeyID, Limit: 64,
		})
		if err != nil {
			return queueFailedMsg{err: err}
		}
		return queueLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) renderQueueView() string {
	lines := []string{
		"Development Queue · Journal-authoritative",
		"j/k select · r refresh · q quit",
	}
	if model.queueClient == nil {
		lines = append(lines, "Queue service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if len(model.queueSnapshot.Jobs) == 0 {
		lines = append(lines, "No queued jobs. Create one to begin.")
	}
	for _, job := range model.queueSnapshot.Jobs {
		lines = append(lines, fmt.Sprintf(
			"• %s · %s · lane %s · dag %s · %s",
			sanitizeCell(job.JobID, 36),
			humanizeStatus(string(job.Status)),
			humanizeStatus(string(job.Lane)),
			sanitizeCell(job.DAGNodeID, 24),
			sanitizeCell(strings.Join(job.OwnedPaths, ","), 40),
		))
	}
	if len(model.queueSnapshot.Gaps) > 0 {
		lines = append(lines, "Gap proposals:")
		for _, gap := range model.queueSnapshot.Gaps {
			lines = append(lines, fmt.Sprintf(
				"• %s · %s · %s",
				sanitizeCell(gap.GapID, 20),
				sanitizeCell(gap.AffectedCapability, 24),
				sanitizeCell(gap.Disposition, 20),
			))
		}
	}
	if len(model.queueSnapshot.Successors) > 0 {
		lines = append(lines, "Successor proposals:")
		for _, successor := range model.queueSnapshot.Successors {
			lines = append(lines, fmt.Sprintf(
				"• %s · gap %s",
				sanitizeCell(successor.SuccessorProposalID, 20),
				sanitizeCell(successor.GapID, 20),
			))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
