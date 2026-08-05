package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/rules"
)

type CustomerRuleClient interface {
	CustomerRuleSnapshot(context.Context, app.CustomerRuleSnapshotRequest) (app.CustomerRuleSnapshot, error)
	CustomerRuleCommand(context.Context, app.CustomerRuleCommandRequest) (app.CustomerRuleCommandResult, error)
}

func customerRuleClientFrom(client ReadClient) CustomerRuleClient {
	customerRuleClient, _ := client.(CustomerRuleClient)
	return customerRuleClient
}

func (client *DaemonReadClient) CustomerRuleSnapshot(
	ctx context.Context,
	request app.CustomerRuleSnapshotRequest,
) (app.CustomerRuleSnapshot, error) {
	var snapshot app.CustomerRuleSnapshot
	err := client.client.CallJourney(
		ctx, request.JourneyID, "customer_rule_snapshot", request, &snapshot,
	)
	return snapshot, err
}

func (client *DaemonReadClient) CustomerRuleCommand(
	ctx context.Context,
	request app.CustomerRuleCommandRequest,
) (app.CustomerRuleCommandResult, error) {
	var result app.CustomerRuleCommandResult
	err := client.client.CallJourney(
		ctx, request.JourneyID, "customer_rule_command", request, &result,
	)
	return result, err
}

type customerRuleLoadedMsg struct {
	snapshot app.CustomerRuleSnapshot
}

type customerRuleFailedMsg struct {
	err error
}

type customerRuleCommandDoneMsg struct {
	err  error
	note string
}

func (model Model) loadCustomerRules() tea.Cmd {
	client, ctx, journeyID := model.customerRuleClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return customerRuleFailedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		snapshot, err := client.CustomerRuleSnapshot(ctx, app.CustomerRuleSnapshotRequest{JourneyID: journeyID})
		if err != nil {
			return customerRuleFailedMsg{err: err}
		}
		return customerRuleLoadedMsg{snapshot: snapshot}
	}
}

func (model Model) customerRuleImport(content string) tea.Cmd {
	client, ctx, journeyID := model.customerRuleClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return customerRuleCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		input, _ := json.Marshal(map[string]any{"content": content})
		_, err := client.CustomerRuleCommand(ctx, app.CustomerRuleCommandRequest{
			JourneyID: journeyID, OperationID: "tui-rule-import",
			Action: "import", Input: input,
		})
		return customerRuleCommandDoneMsg{err: err, note: "rules imported"}
	}
}

func (model Model) customerRuleEvaluate() tea.Cmd {
	client, ctx, journeyID := model.customerRuleClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return customerRuleCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		input, _ := json.Marshal(map[string]any{"action": map[string]any{
			"ProjectID": "project-1", "TeamInstanceID": "team-1",
			"WorkPackageID": "wp-1", "WorkItemID": "work-1",
			"RunID": "run-1", "AgentInstanceID": "agent-1",
			"LogicalNodeID": "node-1", "AttemptNumber": 1,
			"Action": "publish", "Risk": "high",
			"ContractDigest": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}})
		result, err := client.CustomerRuleCommand(ctx, app.CustomerRuleCommandRequest{
			JourneyID: journeyID, OperationID: "tui-rule-eval",
			Action: "evaluate", Input: input,
		})
		if err != nil {
			return customerRuleCommandDoneMsg{err: err}
		}
		return customerRuleCommandDoneMsg{note: fmt.Sprintf(
			"rule %s effect %s", result.Decision.RuleID, result.Decision.Effect)}
	}
}

// customerRuleDefineBuiltin defines the frozen journey rule without typed
// entry, exercising the define command end to end.
func (model Model) customerRuleDefineBuiltin() tea.Cmd {
	client, ctx, journeyID := model.customerRuleClient, model.ctx, model.evolutionJourneyID
	return func() tea.Msg {
		if client == nil {
			return customerRuleCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		input, _ := json.Marshal(rules.CustomerRule{
			RuleID: "rule-journey", Scope: rules.CustomerScopeProject, ScopeID: "project-1",
			Action: "publish", Risk: "high", Effect: rules.EffectReportOnly,
		})
		_, err := client.CustomerRuleCommand(ctx, app.CustomerRuleCommandRequest{
			JourneyID: journeyID, OperationID: "tui-rule-define",
			Action: "define", Input: input,
		})
		return customerRuleCommandDoneMsg{err: err, note: "rule defined"}
	}
}

func (model Model) renderCustomerRulesView() string {
	lines := []string{
		"Customer Rules · standing policy",
		"i import toml · e evaluate · d detail · r refresh · q quit",
	}
	if model.customerRuleClient == nil {
		lines = append(lines, "Customer rule service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if len(model.customerRuleSnapshot.Rules) == 0 {
		lines = append(lines, "No customer rules. Press i to import .loom/permissions.toml.")
	}
	for index, view := range model.customerRuleSnapshot.Rules {
		marker := " "
		if index == model.selected {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf(
			"%s Rule %s · %s · %s · budget %d",
			marker,
			sanitizeCell(view.Rule.RuleID, 24),
			humanizeStatus(view.Status),
			humanizeStatus(string(view.Rule.Effect)),
			view.Consumed,
		))
		if model.customerRuleDetail && index == model.selected {
			lines = append(lines, fmt.Sprintf(
				"  scope %s/%s · action %s · risk %s · limit %d",
				sanitizeCell(view.Rule.Scope, 14), sanitizeCell(view.Rule.ScopeID, 20),
				sanitizeCell(view.Rule.Action, 16), sanitizeCell(view.Rule.Risk, 8),
				view.Rule.BudgetLimit,
			))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
