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
	"loom-pi-rebuild/internal/rules"

	_ "modernc.org/sqlite"
)

func TestWRulesHandlerWiredThroughComposition(t *testing.T) {
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/wr.db?%s", t.TempDir(), values.Encode()))
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
	customerService, err := app.NewLocalCustomerRuleService(store, now, func() string { return "v1" }, port.authority)
	if err != nil {
		t.Fatal(err)
	}
	customerRuleAPI, err := api.NewLocalCustomerRuleAPI(customerService)
	if err != nil {
		t.Fatal(err)
	}
	handler := newProductRouteHandler(productRouteServices{customerRule: customerRuleAPI})
	journey := "66666666-6666-4666-8666-666666666666"

	defineParams := mustMarshalJSON(map[string]any{
		"operation_id": "op-define", "action": "define",
		"input": rules.CustomerRule{
			RuleID: "rule-wire", Scope: rules.CustomerScopeProject, ScopeID: "project-1",
			Action: "publish", Risk: "high", Effect: rules.EffectRequireApproval,
			ApproverRefs: []string{"approver:permission-owner"},
			Timeout:      time.Hour, OnTimeout: "reject",
			BudgetUnit: "calls", BudgetLimit: 3,
		},
	})
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-wr-1", JourneyID: journey,
		Method: "customer_rule_command", Params: defineParams,
	})
	if !response.OK {
		t.Fatalf("define failed: %+v", response.Error)
	}

	snapshotResponse := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-wr-2", JourneyID: journey,
		Method: "customer_rule_snapshot", Params: []byte(`{}`),
	})
	if !snapshotResponse.OK {
		t.Fatalf("snapshot failed: %+v", snapshotResponse.Error)
	}
	var snapshot app.CustomerRuleSnapshot
	if err := json.Unmarshal(snapshotResponse.Result, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rules) != 1 || snapshot.Rules[0].Rule.RuleID != "rule-wire" {
		t.Fatalf("snapshot = %+v", snapshot.Rules)
	}

	evalParams := mustMarshalJSON(map[string]any{
		"operation_id": "op-eval", "action": "evaluate",
		"input": map[string]any{"action": map[string]any{
			"ProjectID": "project-1", "TeamInstanceID": "team-1",
			"WorkPackageID": "wp-1", "WorkItemID": "work-1",
			"RunID": "run-1", "AgentInstanceID": "agent-1",
			"LogicalNodeID": "node-1", "AttemptNumber": 1,
			"Action": "publish", "Risk": "high",
			"ContractDigest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}},
	})
	response = handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "req-wr-3", JourneyID: journey,
		Method: "customer_rule_command", Params: evalParams,
	})
	if !response.OK {
		t.Fatalf("evaluate failed: %+v", response.Error)
	}
	var result app.CustomerRuleCommandResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result.Decision.Effect != rules.EffectRequireApproval || result.Decision.RuleID != "rule-wire" {
		t.Fatalf("decision = %+v", result.Decision)
	}
}
