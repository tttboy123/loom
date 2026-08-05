package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/rules"

	_ "modernc.org/sqlite"
)

func TestWAutonomyHandlerWiredThroughComposition(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/wa.db?%s", t.TempDir(), values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	store := journal.NewStore(db)
	now := func() time.Time { return time.Now().UTC() }
	port, err := newPermissionApprovalPort(store, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []rules.CustomerRule{
		{
			RuleID: "wa-trigger", Scope: rules.CustomerScopeProject, ScopeID: "project-1",
			Action: "publish", Risk: "high", Effect: rules.EffectReportOnly,
			BudgetUnit: "calls", BudgetLimit: 10,
		},
		{
			RuleID: "wa-budget", Scope: rules.CustomerScopeProject, ScopeID: "project-1",
			Action: "publish", Risk: "high", Effect: rules.EffectReportOnly,
			BudgetUnit: "calls", BudgetLimit: 2,
		},
	} {
		if _, err := port.authority.DefineCustomerRule(context.Background(), rule,
			"owner-1", "op-seed-"+rule.RuleID, "88888888-8888-4888-8888-888888888888"); err != nil {
			t.Fatal(err)
		}
	}
	standingService, err := app.NewLocalStandingOrderService(
		store, now, func() string { return "v1" },
		rules.NewStandingOrderAuthority(store, port.authority, now),
	)
	if err != nil {
		t.Fatal(err)
	}
	standingOrderAPI, err := api.NewLocalStandingOrderAPI(standingService)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandlerWithComposition(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, standingOrderAPI,
	)
	journey := "88888888-8888-4888-8888-888888888888"

	define := func(action string, input any) localipc.Response {
		return handler(context.Background(), localipc.Request{
			Version: 1, RequestID: "req-wa-" + fmt.Sprint(time.Now().UnixNano()),
			JourneyID: journey, Method: "standing_order_command",
			Params: mustMarshalJSON(map[string]any{
				"operation_id": "op-" + action, "action": action, "input": input,
			}),
		})
	}
	response := define("define", map[string]any{
		"order_id": "order-wire", "scope": "job", "scope_id": "job-wa",
		"trigger_rule_id": "wa-trigger", "tool": "Bash", "pattern": "printf",
		"budget_rule_id": "wa-budget", "max_iterations": 5,
	})
	if !response.OK {
		t.Fatalf("define failed: %+v", response.Error)
	}
	snapshot := func() app.StandingOrderSnapshot {
		response := handler(context.Background(), localipc.Request{
			Version: 1, RequestID: "req-wa-snap", JourneyID: journey,
			Method: "standing_order_snapshot", Params: []byte(`{}`),
		})
		if !response.OK {
			t.Fatalf("snapshot failed: %+v", response.Error)
		}
		var snapshot app.StandingOrderSnapshot
		if err := json.Unmarshal(response.Result, &snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	initial := snapshot()
	if len(initial.Orders) != 1 || initial.Orders[0].Order.Active {
		t.Fatalf("defined order must be inactive: %+v", initial.Orders)
	}
	response = define("activate", map[string]any{
		"order_id": "order-wire", "authorized_by": "human:project-owner",
	})
	if !response.OK {
		t.Fatalf("activate failed: %+v", response.Error)
	}
	activated := snapshot()
	if len(activated.Orders) != 1 || !activated.Orders[0].Order.Active {
		t.Fatalf("order must be active: %+v", activated.Orders)
	}
	response = define("revoke", map[string]any{
		"order_id": "order-wire", "authorized_by": "human:project-owner",
	})
	if !response.OK {
		t.Fatalf("revoke failed: %+v", response.Error)
	}
	revoked := snapshot()
	if len(revoked.Orders) != 1 || revoked.Orders[0].Order.RevokedAt == "" {
		t.Fatalf("order must be revoked: %+v", revoked.Orders)
	}
	// Model/agent must never be a valid activator: empty actor is rejected.
	response = define("activate", map[string]any{
		"order_id": "order-wire", "authorized_by": "",
	})
	if response.OK {
		t.Fatal("activation without human actor must fail")
	}
	_ = permissions.ToolBash
}
