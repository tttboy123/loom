package tui

import (
	"context"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/rules"
)

type stubCustomerRuleClient struct {
	snapshot app.CustomerRuleSnapshot
	result   app.CustomerRuleCommandResult
}

func (client *stubCustomerRuleClient) CustomerRuleSnapshot(
	context.Context,
	app.CustomerRuleSnapshotRequest,
) (app.CustomerRuleSnapshot, error) {
	return client.snapshot, nil
}

func (client *stubCustomerRuleClient) CustomerRuleCommand(
	context.Context,
	app.CustomerRuleCommandRequest,
) (app.CustomerRuleCommandResult, error) {
	return client.result, nil
}

type stubCustomerRuleReadClient struct{}

func (client *stubCustomerRuleReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return api.LocalProductSnapshot{}, nil
}

func (client *stubCustomerRuleReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, nil
}

func TestCustomerRulesScreenRendersRulesAndImportKey(t *testing.T) {
	ruleClient := &stubCustomerRuleClient{
		snapshot: app.CustomerRuleSnapshot{
			ViewVersion: "v1",
			Rules: []app.CustomerRuleView{{
				Rule: rules.CustomerRule{
					RuleID: "rule-a", Scope: rules.CustomerScopeProject,
					ScopeID: "project-1", Effect: rules.EffectRequireApproval,
					BudgetLimit: 5,
				},
				Status: "active", Consumed: 2,
			}},
		},
	}
	model, err := newModelWithContext(context.Background(), &stubCustomerRuleReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	model.customerRuleClient = ruleClient
	model.customerRuleSnapshot = ruleClient.snapshot
	model.screenIndex = indexOfScreen(ScreenCustomerRules)
	body := model.renderCustomerRulesView()
	for _, want := range []string{"Customer Rules", "rule-a", "Require approval", "budget 2"} {
		if !strings.Contains(body, want) {
			t.Fatalf("customer rules view missing %q:\n%s", want, body)
		}
	}
	updated, _ := model.Update(teaKeyString("i"))
	if updated.(Model).entryMode != entryCustomerRuleImport {
		t.Fatalf("i must open import entry mode")
	}
}
