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

// AutonomyClient exposes the W-AUTONOMY standing-order surface (default off).
type AutonomyClient interface {
	StandingOrderSnapshot(context.Context, app.StandingOrderSnapshotRequest) (app.StandingOrderSnapshot, error)
	StandingOrderCommand(context.Context, app.StandingOrderCommandRequest) (app.StandingOrderCommandResult, error)
}

func standingOrderClientFrom(client ReadClient) AutonomyClient {
	autonomyClient, _ := client.(AutonomyClient)
	return autonomyClient
}

func (client *DaemonReadClient) StandingOrderSnapshot(
	ctx context.Context,
	request app.StandingOrderSnapshotRequest,
) (app.StandingOrderSnapshot, error) {
	var snapshot app.StandingOrderSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "standing_order_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) StandingOrderCommand(
	ctx context.Context,
	request app.StandingOrderCommandRequest,
) (app.StandingOrderCommandResult, error) {
	var result app.StandingOrderCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "standing_order_command", request, &result,
	)
	return result, err
}

type standingOrderLoadedMsg struct {
	snapshot app.StandingOrderSnapshot
}

type standingOrderFailedMsg struct {
	err error
}

type standingOrderCommandDoneMsg struct {
	err  error
	note string
}

func (model Model) loadStandingOrders() tea.Cmd {
	client, ctx, journeyID := model.standingOrderClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return standingOrderFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.StandingOrderSnapshot(
			ctx, app.StandingOrderSnapshotRequest{JourneyID: journeyID},
		)
		if err != nil {
			return standingOrderFailedMsg{err: err}
		}
		return standingOrderLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) standingOrderActivate(orderID string) tea.Cmd {
	client, ctx, journeyID := model.standingOrderClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return standingOrderCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		result, err := client.StandingOrderCommand(ctx, app.StandingOrderCommandRequest{
			JourneyID: journeyID, OperationID: "tui-activate-" + orderID,
			Action: "activate",
			Input:  mustJSON(map[string]any{"order_id": orderID, "authorized_by": "human:tui-operator"}),
		})
		if err != nil {
			return standingOrderCommandDoneMsg{err: err}
		}
		return standingOrderCommandDoneMsg{note: result.Note}
	}
}

func (model Model) standingOrderRevoke(orderID string) tea.Cmd {
	client, ctx, journeyID := model.standingOrderClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return standingOrderCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		result, err := client.StandingOrderCommand(ctx, app.StandingOrderCommandRequest{
			JourneyID: journeyID, OperationID: "tui-revoke-" + orderID,
			Action: "revoke",
			Input:  mustJSON(map[string]any{"order_id": orderID, "authorized_by": "human:tui-operator"}),
		})
		if err != nil {
			return standingOrderCommandDoneMsg{err: err}
		}
		return standingOrderCommandDoneMsg{note: result.Note}
	}
}

func (model Model) renderAutonomyView() string {
	lines := []string{
		"Autopilot · standing orders · default off",
		"K activate · L revoke · r refresh · q quit",
	}
	if model.standingOrderClient == nil {
		lines = append(lines, "Standing order service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	state := "OFF"
	if model.standingOrderSnapshot.Autopilot {
		state = "ON"
	}
	lines = append(lines, fmt.Sprintf("Autopilot: %s", state))
	if len(model.standingOrderSnapshot.Orders) == 0 {
		lines = append(lines, "No standing orders. Orders are human-defined and default inactive.")
	}
	for index, view := range model.standingOrderSnapshot.Orders {
		marker := " "
		if index == model.standingOrderSelected {
			marker = ">"
		}
		status := "inactive"
		if view.Order.RevokedAt != "" {
			status = "revoked"
		} else if view.Order.Active {
			status = "active"
		}
		lines = append(lines, fmt.Sprintf(
			"%s Order %s · %s · %s/%s · %s · dispatches %d",
			marker,
			sanitizeCell(view.Order.OrderID, 24),
			humanizeStatus(status),
			sanitizeCell(string(view.Order.Scope), 10),
			sanitizeCell(view.Order.ScopeID, 20),
			sanitizeCell(string(view.Order.Tool), 8),
			view.Dispatches,
		))
	}
	return strings.Join(lines, "\n") + "\n"
}

func mustJSON(value any) []byte {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return body
}
