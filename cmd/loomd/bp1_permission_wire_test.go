package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"

	_ "modernc.org/sqlite"
)

func TestBp1PermissionHandlerWiredThroughComposition(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/dbg.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	service, err := app.NewLocalPermissionService(store, func() time.Time { return time.Now().UTC() },
		func() string { return "v1" })
	if err != nil {
		t.Fatal(err)
	}
	permissionAPI, err := api.NewLocalPermissionAPI(service)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithComposition(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, permissionAPI)
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-1", JourneyID: "11111111-1111-4111-8111-111111111111",
		Method: "permissions_snapshot", Params: []byte(`{}`),
	})
	t.Logf("snapshot ok=%v err=%+v", response.OK, response.Error)
	if !response.OK {
		t.Fatalf("permissions_snapshot failed: %+v", response.Error)
	}
}
