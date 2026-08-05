package tui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestCustomerRuleImportEntryCarriesContent(t *testing.T) {
	var received string
	client := &stubCustomerRuleClient{}
	_ = client
	model, err := newModelWithContext(context.Background(), &stubCustomerRuleReadClient{})
	if err != nil {
		t.Fatal(err)
	}
	// stub client records import content
	recordClient := &recordingCustomerRuleClient{onImport: func(content string) { received = content }}
	model.customerRuleClient = recordClient
	model.screenIndex = indexOfScreen(ScreenCustomerRules)
	updated, _ := model.Update(teaKeyString("i"))
	model = updated.(Model)
	if model.entryMode != entryCustomerRuleImport {
		t.Fatalf("entryMode = %q", model.entryMode)
	}
	text := []rune(`rule "rule-journey" project "project-1" action "publish" effect "require_approval" approver "x" timeout 3600`)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: text})
	model = updated.(Model)
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter must produce an import command")
	}
	_ = cmd() // execute the command synchronously
	if received == "" || !strings.Contains(received, "rule-journey") {
		t.Fatalf("import content lost: %q", received)
	}
}

type recordingCustomerRuleClient struct {
	onImport func(string)
}

func (client *recordingCustomerRuleClient) CustomerRuleSnapshot(
	context.Context,
	app.CustomerRuleSnapshotRequest,
) (app.CustomerRuleSnapshot, error) {
	return app.CustomerRuleSnapshot{}, nil
}

func (client *recordingCustomerRuleClient) CustomerRuleCommand(
	ctx context.Context,
	request app.CustomerRuleCommandRequest,
) (app.CustomerRuleCommandResult, error) {
	if request.Action == "import" {
		var input struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(request.Input, &input)
		client.onImport(input.Content)
	}
	return app.CustomerRuleCommandResult{}, nil
}
