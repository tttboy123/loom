package app_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/production"
)

func TestLocalProductionServicePreviewConfirmSnapshot(t *testing.T) {
	store := openExecutionAppStore(t)
	root := executionAppTempDir(t)
	appSupport := filepath.Join(root, "Application Support", "Loom")
	launchAgents := filepath.Join(root, "LaunchAgents")
	if err := os.MkdirAll(appSupport, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(launchAgents, 0o700); err != nil {
		t.Fatal(err)
	}
	core, err := production.NewService(
		store, production.Paths{
			AppSupport: appSupport, LaunchAgents: launchAgents, DaemonPath: "/usr/local/bin/loomd",
		},
		func() time.Time { return time.Date(2026, 8, 5, 16, 0, 0, 0, time.UTC) },
		func(context.Context) (bool, error) { return false, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewLocalProductionService(
		store, func() time.Time { return time.Now().UTC() },
		func() string { return "v1" }, core,
	)
	if err != nil {
		t.Fatal(err)
	}
	journey := "66666666-6666-4666-8666-666666666666"
	preview, err := service.ProductionCommand(context.Background(), app.ProductionCommandRequest{
		JourneyID: journey, OperationID: "op-preview", Action: "activation_preview",
		Input: mustRawJSON(t, map[string]any{
			"operation": "activation_preview", "target_mode": "default",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Result.Preview.Digest == "" {
		t.Fatalf("preview = %+v", preview)
	}
	confirm, err := service.ProductionCommand(context.Background(), app.ProductionCommandRequest{
		JourneyID: journey, OperationID: "op-confirm", Action: "activation_confirm",
		Input: mustRawJSON(t, map[string]any{
			"operation": "activation_confirm", "target_mode": "default",
			"preview_digest": preview.Result.Preview.Digest, "authorized_by": "user-1",
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirm.Result.Activated {
		t.Fatalf("confirm = %+v", confirm)
	}
	snapshot, err := service.ProductionSnapshot(context.Background(), app.ProductionSnapshotRequest{JourneyID: journey})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Activated || snapshot.Recovery.Degraded {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if service.Degraded(context.Background()) {
		t.Fatal("fresh activation must not be degraded")
	}
}

func mustRawJSON(t testing.TB, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
