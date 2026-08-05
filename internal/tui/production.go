package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/production"
)

func productionClientFrom(client ReadClient) ProductionClient {
	productionClient, _ := client.(ProductionClient)
	return productionClient
}

func (client *DaemonReadClient) ProductionSnapshot(
	ctx context.Context,
	request app.ProductionSnapshotRequest,
) (production.ProductionSnapshot, error) {
	var snapshot production.ProductionSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "production_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) ProductionCommand(
	ctx context.Context,
	request app.ProductionCommandRequest,
) (app.ProductionCommandResult, error) {
	var result app.ProductionCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "production_command", request, &result,
	)
	return result, err
}

type productionLoadedMsg struct {
	snapshot production.ProductionSnapshot
}

type productionFailedMsg struct {
	err error
}

type productionCommandDoneMsg struct {
	err  error
	note string
}

func (model Model) loadProduction() tea.Cmd {
	client, ctx, journeyID := model.productionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return productionFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.ProductionSnapshot(ctx, app.ProductionSnapshotRequest{
			JourneyID: journeyID,
		})
		if err != nil {
			return productionFailedMsg{err: err}
		}
		return productionLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) productionPreviewActivation() tea.Cmd {
	return model.productionCommand("activation_preview", map[string]any{
		"operation": "activation_preview", "target_mode": "default",
	})
}

func (model Model) productionConfirmActivation() tea.Cmd {
	return model.productionCommand("activation_confirm", map[string]any{
		"operation": "activation_confirm", "target_mode": "default",
		"preview_digest": model.productionPreview.Digest, "authorized_by": "tui-user",
	})
}

func (model Model) productionPreviewDeactivation() tea.Cmd {
	return model.productionCommand("deactivation_preview", map[string]any{
		"operation": "deactivation_preview",
	})
}

func (model Model) productionConfirmDeactivation() tea.Cmd {
	return model.productionCommand("deactivation_confirm", map[string]any{
		"operation": "deactivation_confirm",
		"preview_digest": model.productionPreview.Digest, "authorized_by": "tui-user",
	})
}

func (model Model) productionCommand(action string, input map[string]any) tea.Cmd {
	client, ctx, journeyID := model.productionClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return productionCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		encoded, _ := json.Marshal(input)
		result, err := client.ProductionCommand(ctx, app.ProductionCommandRequest{
			JourneyID: journeyID, OperationID: "tui-prod-" + action,
			Action: action, Input: encoded,
		})
		if err != nil {
			return productionCommandDoneMsg{err: err}
		}
		model.productionPreview = result.Result.Preview
		model.productionPending = action
		return productionCommandDoneMsg{note: result.Note}
	}
}

func (model Model) renderProductionView() string {
	lines := []string{
		"Production · resident daemon activation",
		"p preview activation · c confirm · d deactivation preview · x confirm deactivation · r refresh · q quit",
	}
	if model.productionClient == nil {
		lines = append(lines, "Production service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	status := "inactive"
	if model.productionSnapshot.Activated {
		status = "activated · " + model.productionSnapshot.TargetMode
	}
	lines = append(lines, "Status: "+status)
	if model.productionSnapshot.Recovery.Degraded {
		lines = append(lines, "DEGRADED read-only: "+model.productionSnapshot.Recovery.Reason)
	}
	if model.productionPreview.Digest != "" {
		lines = append(lines, fmt.Sprintf(
			"Preview %s · digest %s",
			model.productionPreview.Operation,
			sanitizeCell(model.productionPreview.Digest, 16),
		))
		for _, change := range model.productionPreview.Files {
			lines = append(lines, fmt.Sprintf(
				"  %s %s",
				sanitizeCell(change.Action, 8),
				sanitizeCell(change.Path, 64),
			))
		}
		for _, item := range model.productionPreview.LaunchdItems {
			lines = append(lines, fmt.Sprintf(
				"  launchd %s %s",
				sanitizeCell(item.Action, 8),
				sanitizeCell(item.Label, 32),
			))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
