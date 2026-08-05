package tui

import (
	"context"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/rules"
)

type stubAutonomyClient struct {
	snapshot app.StandingOrderSnapshot
	commands []app.StandingOrderCommandRequest
}

func (client *stubAutonomyClient) StandingOrderSnapshot(
	ctx context.Context,
	request app.StandingOrderSnapshotRequest,
) (app.StandingOrderSnapshot, error) {
	return client.snapshot, nil
}

func (client *stubAutonomyClient) StandingOrderCommand(
	ctx context.Context,
	request app.StandingOrderCommandRequest,
) (app.StandingOrderCommandResult, error) {
	client.commands = append(client.commands, request)
	return app.StandingOrderCommandResult{
		OperationID: request.OperationID, Action: request.Action,
		ViewVersion: "v1", Note: "ok",
	}, nil
}

func TestAutonomyScreenRendersOrdersAndKeys(t *testing.T) {
	client := &stubAutonomyClient{
		snapshot: app.StandingOrderSnapshot{
			ViewVersion: "v1",
			Orders: []app.StandingOrderView{{
				Order: rules.StandingOrder{
					OrderID: "order-a", Scope: rules.StandingScopeJob,
					ScopeID: "job-1", Tool: "Bash", Active: true,
				},
				Dispatches: 3,
			}},
		},
	}
	model, err := newModelWithContext(context.Background(), &stubCustomerRuleReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	model.standingOrderClient = client
	model.standingOrderSnapshot = client.snapshot
	model.screenIndex = indexOfScreen(ScreenAutonomy)
	body := model.renderAutonomyView()
	for _, want := range []string{"Autopilot", "order-a", "Active", "dispatches 3", "default off"} {
		if !strings.Contains(body, want) {
			t.Fatalf("autonomy view missing %q:\n%s", want, body)
		}
	}
	// K activates the selected order; L revokes it.
	updated, cmd := model.Update(teaKeyString("K"))
	if _, ok := updated.(Model); !ok {
		t.Fatal("K must stay in model")
	}
	if cmd != nil {
		_ = cmd() // execute the activate command synchronously
	}
	updated, cmd = model.Update(teaKeyString("L"))
	model = updated.(Model)
	if cmd != nil {
		_ = cmd() // execute the revoke command synchronously
	}
	if len(client.commands) != 2 {
		t.Fatalf("expected 2 commands (activate+revoke), got %d", len(client.commands))
	}
	if client.commands[0].Action != "activate" || client.commands[1].Action != "revoke" {
		t.Fatalf("commands = %+v", client.commands)
	}
}
