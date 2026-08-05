package tui

import (
	"context"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/production"
)

type stubProductionClient struct {
	snapshot production.ProductionSnapshot
	result   app.ProductionCommandResult
}

func (client *stubProductionClient) ProductionSnapshot(
	context.Context,
	app.ProductionSnapshotRequest,
) (production.ProductionSnapshot, error) {
	return client.snapshot, nil
}

func (client *stubProductionClient) ProductionCommand(
	context.Context,
	app.ProductionCommandRequest,
) (app.ProductionCommandResult, error) {
	return client.result, nil
}

type stubProductionReadClient struct {
	production ProductionClient
}

func (client *stubProductionReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return api.LocalProductSnapshot{}, nil
}

func (client *stubProductionReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, nil
}

var _ ReadClient = (*stubProductionReadClient)(nil)

func TestProductionScreenRendersActivationAndDegraded(t *testing.T) {
	productionClient := &stubProductionClient{
		snapshot: production.ProductionSnapshot{
			ViewVersion: "v1", Activated: true, TargetMode: "default",
			Recovery: production.RecoveryStatus{
				Degraded: true, LastActivated: true, ConfigDigestMismatch: true,
				Reason: "config mismatch",
			},
		},
	}
	model, err := newModelWithContext(context.Background(), &stubProductionReadClient{production: productionClient})
	if err != nil {
		t.Fatal(err)
	}
	model.productionClient = productionClient
	model.productionSnapshot = productionClient.snapshot
	model.screenIndex = indexOfScreen(ScreenProduction)
	body := model.renderProductionView()
	for _, want := range []string{"Production", "activated", "DEGRADED", "config mismatch"} {
		if !strings.Contains(body, want) {
			t.Fatalf("production view missing %q:\n%s", want, body)
		}
	}
}
