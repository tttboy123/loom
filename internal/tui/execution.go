package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
)

type BoundedExecutionClient interface {
	ExecutionSnapshot(context.Context, app.ExecutionSnapshotRequest) (app.ExecutionSnapshot, error)
	ExecutionCommand(context.Context, app.ExecutionCommandRequest) (app.ExecutionCommandResult, error)
}

func boundedExecutionClientFrom(client ReadClient) BoundedExecutionClient {
	boundedExecutionClient, _ := client.(BoundedExecutionClient)
	return boundedExecutionClient
}

func (client *DaemonReadClient) ExecutionSnapshot(
	ctx context.Context,
	request app.ExecutionSnapshotRequest,
) (app.ExecutionSnapshot, error) {
	var snapshot app.ExecutionSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "execution_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) ExecutionCommand(
	ctx context.Context,
	request app.ExecutionCommandRequest,
) (app.ExecutionCommandResult, error) {
	var result app.ExecutionCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "execution_command", request, &result,
	)
	return result, err
}

type executionLoadedMsg struct {
	snapshot app.ExecutionSnapshot
}

type executionFailedMsg struct {
	err error
}

type executionCommandDoneMsg struct {
	err  error
	note string
}

func (model Model) loadExecutions() tea.Cmd {
	client, ctx, journeyID := model.boundedExecutionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return executionFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.ExecutionSnapshot(ctx, app.ExecutionSnapshotRequest{
			JourneyID: journeyID,
		})
		if err != nil {
			return executionFailedMsg{err: err}
		}
		return executionLoadedMsg{snapshot: snapshot}
	}
}

// proposeExecutionProbe proposes one deterministic tool call for the selected
// Queue Job (or the first job) through the bounded execution adapter.
func (model Model) proposeExecutionProbe() tea.Cmd {
	client, ctx, journeyID := model.boundedExecutionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return executionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		jobID := ""
		if model.selected < len(model.queueSnapshot.Jobs) {
			jobID = model.queueSnapshot.Jobs[model.selected].JobID
		} else if len(model.queueSnapshot.Jobs) > 0 {
			jobID = model.queueSnapshot.Jobs[0].JobID
		}
		if jobID == "" {
			return executionCommandDoneMsg{err: app.ErrInvalidExecutionRequest}
		}
		input, _ := json.Marshal(map[string]any{
			"job_id": jobID,
			"call": map[string]any{
				"tool": "Bash", "command": "printf loom-execution-probe", "path": "",
			},
		})
		result, err := client.ExecutionCommand(ctx, app.ExecutionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-exec-probe-" + jobID,
			Action: "propose", Input: input,
		})
		if err != nil {
			return executionCommandDoneMsg{err: err}
		}
		return executionCommandDoneMsg{
			note: fmt.Sprintf(
				"execution %s · verdict %s · %s",
				result.Result.ExecutionID,
				result.Result.Verdict,
				result.Note,
			),
		}
	}
}

func (model Model) renderExecutionsView() string {
	lines := []string{
		styleTitle("Execution") + " · bounded execution adapter",
		"p propose probe · d detail · r refresh · q quit",
	}
	if model.boundedExecutionClient == nil {
		lines = append(lines, "Execution service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if len(model.executionSnapshot.Records) == 0 {
		lines = append(lines, "No executions yet. Press p to propose a deterministic probe.")
	}
	for index, record := range model.executionSnapshot.Records {
		marker := " "
		if index == model.selected {
			marker = styleSelectedMarker(">")
		}
		row := fmt.Sprintf(
			"%s %s · %s · %s · %s · %s",
			marker,
			sanitizeCell(record.ExecutionID, 12),
			sanitizeCell(record.JobID, 24),
			sanitizeCell(string(record.Tool), 10),
			styleStatus(humanizeStatus(record.Status)),
			sanitizeCell(record.ProposedAt, 20),
		)
		if index == model.selected {
			row = styleSelected(row)
		}
		lines = append(lines, row)
		if model.executionDetail && index == model.selected {
			lines = append(lines, styleSection(fmt.Sprintf(
				"  command: %s",
				sanitizeCell(record.Command, 64),
			)))
			lines = append(lines, fmt.Sprintf(
				"  exit %d · output %s · changed %s",
				record.ExitCode,
				sanitizeCell(record.OutputDigest, 16),
				sanitizeCell(record.ChangedFilesDigest, 16),
			))
			if record.DenialReason != "" {
				lines = append(lines, styleError("  denial: "+sanitizeCell(record.DenialReason, 64)))
			}
			if record.FailureReason != "" {
				lines = append(lines, styleError("  failure: "+sanitizeCell(record.FailureReason, 64)))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
