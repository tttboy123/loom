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

// integrationClientFrom returns the read client's IntegrationClient surface.
func integrationClientFrom(client ReadClient) IntegrationClient {
	integrationClient, _ := client.(IntegrationClient)
	return integrationClient
}

func (client *DaemonReadClient) IntegrationSnapshot(
	ctx context.Context,
	journeyID string,
) (app.IntegrationSnapshot, error) {
	var snapshot app.IntegrationSnapshot
	err := client.client.CallJourney(
		ctx, journeyID,
		"integration_snapshot", struct{}{}, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) IntegrationCommand(
	ctx context.Context,
	request app.IntegrationCommandRequest,
) (app.IntegrationCommandResult, error) {
	var result app.IntegrationCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "integration_command", request, &result,
	)
	return result, err
}

func (model Model) loadIntegration() tea.Cmd {
	client, ctx, journeyID := model.integrationClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return integrationFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.IntegrationSnapshot(ctx, journeyID)
		if err != nil {
			return integrationFailedMsg{err: err}
		}
		return integrationLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) renderIntegrationView() string {
	lines := []string{
		"Integration · single writer · Timeline / Attention",
		"r refresh · q quit",
	}
	if model.integrationClient == nil {
		lines = append(lines, "Integration service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	for _, release := range model.integrationSnapshot.Releases {
		lines = append(lines, fmt.Sprintf(
			"Release %s · %s · %s · %s",
			sanitizeCell(release.ReleaseID, 28),
			sanitizeCell(release.TargetBranch, 16),
			humanizeStatus(release.Status),
			sanitizeCell(release.CandidateID, 24),
		))
	}
	for _, canary := range model.integrationSnapshot.Canaries {
		lines = append(lines, fmt.Sprintf(
			"Canary %s · %s · %s",
			sanitizeCell(canary.CanaryID, 28),
			sanitizeCell(canary.RuntimeInstanceID, 32),
			humanizeStatus(canary.Status),
		))
	}
	for _, frame := range model.integrationSnapshot.Timeline {
		lines = append(lines, fmt.Sprintf(
			"Frame %s · attempt %s · gen %d · %s",
			sanitizeCell(frame.FrameID, 24),
			sanitizeCell(frame.AttemptID, 28),
			frame.Generation,
			sanitizeCell(frame.Kind, 20),
		))
	}
	for _, item := range model.integrationSnapshot.Attention {
		lines = append(lines, fmt.Sprintf(
			"Attention %s · %s · %s",
			sanitizeCell(item.SourceID, 28),
			humanizeStatus(item.Lane),
			sanitizeCell(item.Summary, 40),
		))
	}
	return strings.Join(lines, "\n") + "\n"
}

// integrateFirstCandidate integrates the first reviewed Candidate through
// the single Integrator (user-driven).
func (model Model) integrateFirstCandidate() tea.Cmd {
	client, ctx, journeyID := model.integrationClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return integrationCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		var candidateID string
		for _, attempt := range model.workersSnapshot.Attempts {
			if attempt.Status == "succeeded" && attempt.CandidateBranch != "" {
				candidateID = strings.TrimPrefix(attempt.CandidateBranch, "codex/candidate-")
				break
			}
		}
		if candidateID == "" {
			return integrationCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		input, _ := json.Marshal(map[string]any{
			"candidate_id": candidateID, "target_branch": "main",
			"base_commit": "base-1", "source_digest": "src-" + candidateID,
			"evidence_digest":    "ev-" + candidateID,
			"dependency_digests": []string{},
		})
		if _, err := client.IntegrationCommand(ctx, app.IntegrationCommandRequest{
			JourneyID: journeyID, OperationID: "tui-integrate-" + candidateID,
			Action: "integrate", Input: input,
		}); err != nil {
			return integrationCommandDoneMsg{err: err}
		}
		return integrationCommandDoneMsg{}
	}
}

// adoptFirstRelease binds the first published release to a later Run.
func (model Model) adoptFirstRelease() tea.Cmd {
	client, ctx, journeyID := model.integrationClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return integrationCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		for _, release := range model.integrationSnapshot.Releases {
			if release.Status != "rolled_back" {
				input, _ := json.Marshal(map[string]any{
					"release_id": release.ReleaseID, "run_id": "run-tui-later",
				})
				if _, err := client.IntegrationCommand(ctx, app.IntegrationCommandRequest{
					JourneyID: journeyID, OperationID: "tui-adopt-" + release.ReleaseID,
					Action: "adopt", Input: input,
				}); err != nil {
					return integrationCommandDoneMsg{err: err}
				}
				return integrationCommandDoneMsg{}
			}
		}
		return integrationCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
	}
}

// rollbackFirstRelease restores the prior eligible version for the first
// published release.
func (model Model) rollbackFirstRelease() tea.Cmd {
	client, ctx, journeyID := model.integrationClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return integrationCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		for _, release := range model.integrationSnapshot.Releases {
			if release.Status != "rolled_back" {
				input, _ := json.Marshal(map[string]any{
					"release_id": release.ReleaseID, "rollback_to_id": "release-prior",
				})
				if _, err := client.IntegrationCommand(ctx, app.IntegrationCommandRequest{
					JourneyID: journeyID, OperationID: "tui-rollback-" + release.ReleaseID,
					Action: "rollback", Input: input,
				}); err != nil {
					return integrationCommandDoneMsg{err: err}
				}
				return integrationCommandDoneMsg{}
			}
		}
		return integrationCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
	}
}
