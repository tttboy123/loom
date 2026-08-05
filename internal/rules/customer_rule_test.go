package rules

import (
	"context"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

const wrTestCorrelation = "11111111-1111-4111-8111-111111111111"

func mustWrAuthority(t testing.TB, store *journal.Store) *Authority {
	t.Helper()
	clock := &rulesTestClock{now: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	return mustRulesAuthority(t, store, authorizer, clock)
}

func baseCustomerRule() CustomerRule {
	return CustomerRule{
		RuleID: "rule-publish", Scope: CustomerScopeProject, ScopeID: "project-1",
		Action: "publish", Risk: "high",
		Effect: EffectRequireApproval, ApproverRefs: []string{"approver:permission-owner"},
		Timeout: time.Hour, OnTimeout: "reject",
		BudgetUnit: "calls", BudgetLimit: 5,
	}
}

func TestRedWR1_DefineRevokeExpireIdempotent(t *testing.T) {
	store := openRulesStore(t)
	authority := mustWrAuthority(t, store)
	if _, err := authority.DefineCustomerRule(context.Background(), baseCustomerRule(), "owner-1", "op-1", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	before := len(mustRulesReadAll(t, store))
	if _, err := authority.DefineCustomerRule(context.Background(), baseCustomerRule(), "owner-1", "op-1", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if after := len(mustRulesReadAll(t, store)); after != before {
		t.Fatalf("idempotent define duplicated facts: %d -> %d", before, after)
	}
	if _, err := authority.RevokeCustomerRule(context.Background(), "rule-publish", "owner-1", "op-2", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ExpireCustomerRule(context.Background(), "rule-publish", "op-3", wrTestCorrelation); err == nil {
		t.Fatal("expire after revoke must error")
	}
}

func TestRedWR3_ReportOnlyZeroBlock(t *testing.T) {
	store := openRulesStore(t)
	authority := mustWrAuthority(t, store)
	rule := baseCustomerRule()
	rule.Effect = EffectReportOnly
	rule.ApproverRefs = nil
	rule.Timeout = 0
	rule.OnTimeout = ""
	if _, err := authority.DefineCustomerRule(context.Background(), rule, "owner-1", "op-r1", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	decision, err := authority.EvaluateCustomerRules(context.Background(), wrTestContext(), wrTestCorrelation)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Effect != EffectReportOnly || decision.RuleID != "rule-publish" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestRedWR4_RejectBlocks(t *testing.T) {
	store := openRulesStore(t)
	authority := mustWrAuthority(t, store)
	rule := baseCustomerRule()
	rule.Effect = EffectReject
	rule.ApproverRefs = nil
	rule.Timeout = 0
	rule.OnTimeout = ""
	if _, err := authority.DefineCustomerRule(context.Background(), rule, "owner-1", "op-r2", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	decision, err := authority.EvaluateCustomerRules(context.Background(), wrTestContext(), wrTestCorrelation)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Effect != EffectReject {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestRedWR5_BudgetExhaustionFailClosed(t *testing.T) {
	store := openRulesStore(t)
	authority := mustWrAuthority(t, store)
	if _, err := authority.DefineCustomerRule(context.Background(), baseCustomerRule(), "owner-1", "op-b1", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := authority.ConsumeBudget(context.Background(), "rule-publish", 1, "op-b"+wrInt(i), wrTestCorrelation); err != nil {
			t.Fatalf("consume %d error = %v", i, err)
		}
	}
	if _, err := authority.ConsumeBudget(context.Background(), "rule-publish", 1, "op-b-over", wrTestCorrelation); err == nil {
		t.Fatal("budget over limit must fail closed")
	}
}

func TestRedWR7_ImportIsIdempotentAndNotDirectlyEvaluated(t *testing.T) {
	store := openRulesStore(t)
	authority := mustWrAuthority(t, store)
	content := []byte(`rule "rule-import" project "project-1"
action "deploy"
effect "require_approval"
approver "approver:permission-owner"
timeout 3600
budget calls 10
`)
	if _, err := authority.ImportPermissionsTOML(context.Background(), content, "owner-1", "op-imp", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	before := len(mustRulesReadAll(t, store))
	if _, err := authority.ImportPermissionsTOML(context.Background(), content, "owner-1", "op-imp", wrTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if after := len(mustRulesReadAll(t, store)); after != before {
		t.Fatalf("import not idempotent: %d -> %d", before, after)
	}
}

func wrTestContext() ActionContext {
	action, err := NewActionContext(ActionContextInput{
		ProjectID: "project-1", TeamInstanceID: "team-1", WorkPackageID: "wp-1",
		WorkItemID: "work-1", RunID: "run-1", AgentInstanceID: "agent-1",
		LogicalNodeID: "node-1", AttemptNumber: 1, Action: "publish", Risk: "high",
		ContractDigest: rulesTestContract,
	})
	if err != nil {
		panic(err)
	}
	return action
}

func wrInt(i int) string {
	return string(rune('0' + i))
}
