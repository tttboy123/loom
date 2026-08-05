package tui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
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

// createQueueJob queues a new user Job with the given owned source path.
// It uses only the production queue_command create_job action.
func (model Model) createQueueJob(sourcePath string) tea.Cmd {
	client, ctx, journeyID := model.queueClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return queueCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if sourcePath == "" {
			return queueCommandDoneMsg{err: app.ErrInvalidQueueRequest}
		}
		var idBytes [16]byte
		if _, err := rand.Read(idBytes[:]); err != nil {
			return queueCommandDoneMsg{err: err}
		}
		idBytes[6] = (idBytes[6] & 0x0f) | 0x40
		idBytes[8] = (idBytes[8] & 0x3f) | 0x80
		encoded := hex.EncodeToString(idBytes[:])
		jobID := encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
		input, err := json.Marshal(map[string]any{
			"job_id": jobID, "source": "user_queued",
			"dag_node_id":  strings.TrimSuffix(strings.ReplaceAll(sourcePath, "/", "_"), "_"),
			"dependencies": []string{},
			"owned_paths":  []string{sourcePath},
			"mutex_keys":   []string{},
			"resource_claims": map[string]any{
				"runtime": "runtime.pi.earendil-works.0.82.1",
				"slots":   1,
				"model":   "loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
			},
			"max_attempts":              3,
			"capability_kind":           "feature",
			"exit_conditions":           []string{"focused green"},
			"verification_strategy":     "full_matrix",
			"integration_strategy":      "single_integrator",
			"protected_authority_paths": []string{},
			"eligibility_authority":     "user",
		})
		if err != nil {
			return queueCommandDoneMsg{err: err}
		}
		if _, err := client.QueueCommand(ctx, api.QueueCommandRequest{
			JourneyID: journeyID, OperationID: "tui-create-" + jobID,
			Action: "create_job", Input: input,
		}); err != nil {
			return queueCommandDoneMsg{err: err}
		}
		return queueCommandDoneMsg{}
	}
}
