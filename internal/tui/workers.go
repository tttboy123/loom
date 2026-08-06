package tui

import (
	"context"
	"encoding/json"
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
		styleTitle("Worker Pools") + " · lease / generation fencing",
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
			styleStatus(humanizeStatus(active.Lane)),
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
			styleStatus(humanizeStatus(string(attempt.Lane))),
			attempt.Generation,
			styleStatus(humanizeStatus(status)),
			attempt.CrashSeam,
		))
	}
	if model.workersSnapshot.RepairWait > 0 {
		lines = append(lines, styleSection(fmt.Sprintf("Repair wait age: %d", model.workersSnapshot.RepairWait)))
	}
	return strings.Join(lines, "\n") + "\n"
}

// claimFirstJob explicitly dispatches one worker to the first non-terminal
// queued Job (user-driven; nothing runs automatically).
func (model Model) claimFirstJob() tea.Cmd {
	client, ctx, journeyID := model.workersClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return workersCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		var jobID, dagNode string
		for _, job := range model.queueSnapshot.Jobs {
			if job.Status == "admitted" || job.Status == "queued" {
				jobID = job.JobID
				dagNode = job.DAGNodeID
				break
			}
		}
		if jobID == "" {
			return workersCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		workerID := "tui-worker-" + strings.ReplaceAll(dagNode, "_", "-")
		input, _ := json.Marshal(map[string]any{
			"worker_id": workerID, "job_id": jobID,
			"lane": "development", "candidate_branch": "codex/candidate-" + strings.TrimPrefix(workerID, "tui-worker-"),
			"candidate_worktree": "/private/tmp/" + workerID,
		})
		if _, err := client.WorkersCommand(ctx, app.WorkersCommandRequest{
			JourneyID: journeyID, OperationID: "tui-claim-" + jobID,
			Action: "claim", Input: input,
		}); err != nil {
			return workersCommandDoneMsg{err: err}
		}
		return workersCommandDoneMsg{}
	}
}

// recordTestSuccess records a deterministic test success for the first open
// attempt and marks the Candidate ready for review.
func (model Model) recordTestSuccess() tea.Cmd {
	client, ctx, journeyID := model.workersClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return workersCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		for _, attempt := range model.workersSnapshot.Attempts {
			if attempt.Status == "claimed" || attempt.Status == "running" {
				input, _ := json.Marshal(map[string]any{
					"attempt_id": attempt.AttemptID, "generation": attempt.Generation,
					"status": "succeeded", "failure_class": "",
					"evidence_digests": []string{"tui-test-evidence"},
					"candidate_ready":  true,
				})
				if _, err := client.WorkersCommand(ctx, app.WorkersCommandRequest{
					JourneyID: journeyID, OperationID: "tui-test-" + attempt.AttemptID,
					Action: "result", Input: input,
				}); err != nil {
					return workersCommandDoneMsg{err: err}
				}
				return workersCommandDoneMsg{}
			}
		}
		return workersCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
	}
}

// recordReviewPass records the read-only Reviewer verdict PASS for the first
// Candidate ready for review.
func (model Model) recordReviewPass() tea.Cmd {
	client, ctx, journeyID := model.workersClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return workersCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		for _, attempt := range model.workersSnapshot.Attempts {
			if attempt.Status == "succeeded" {
				input, _ := json.Marshal(map[string]any{
					"attempt_id": attempt.AttemptID, "generation": attempt.Generation,
					"status": "succeeded", "failure_class": "",
					"evidence_digests": []string{"tui-review-evidence"},
					"review_verdict":   "PASS",
				})
				if _, err := client.WorkersCommand(ctx, app.WorkersCommandRequest{
					JourneyID: journeyID, OperationID: "tui-review-" + attempt.AttemptID,
					Action: "result", Input: input,
				}); err != nil {
					return workersCommandDoneMsg{err: err}
				}
				return workersCommandDoneMsg{}
			}
		}
		return workersCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
	}
}
