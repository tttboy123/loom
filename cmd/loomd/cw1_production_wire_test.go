package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/production"

	_ "modernc.org/sqlite"
)

func TestCw1ProductionHandlerWiredThroughComposition(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/prod.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
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
		func() time.Time { return time.Now().UTC() },
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
	productionAPI, err := api.NewLocalProductionAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithComposition(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, productionAPI, nil,
	)
	journey := "77777777-7777-4777-8777-777777777777"
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-prod-1", JourneyID: journey,
		Method: "production_snapshot", Params: []byte(`{}`),
	})
	if !response.OK {
		t.Fatalf("production_snapshot failed: %+v", response.Error)
	}
	previewParams := mustMarshalJSON(map[string]any{
		"operation_id": "op-preview", "action": "activation_preview",
		"input": map[string]any{"operation": "activation_preview", "target_mode": "default"},
	})
	response = handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-prod-2", JourneyID: journey,
		Method: "production_command", Params: previewParams,
	})
	if !response.OK {
		t.Fatalf("production_command preview failed: %+v", response.Error)
	}
	var preview app.ProductionCommandResult
	if err := json.Unmarshal(response.Result, &preview); err != nil {
		t.Fatal(err)
	}
	confirmParams := mustMarshalJSON(map[string]any{
		"operation_id": "op-confirm", "action": "activation_confirm",
		"input": map[string]any{
			"operation": "activation_confirm", "target_mode": "default",
			"preview_digest": preview.Result.Preview.Digest, "authorized_by": "user-1",
		},
	})
	response = handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-prod-3", JourneyID: journey,
		Method: "production_command", Params: confirmParams,
	})
	if !response.OK {
		t.Fatalf("production_command confirm failed: %+v", response.Error)
	}
}
