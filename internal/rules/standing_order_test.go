package rules

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
)

const soTestCorrelation = "33333333-3333-4333-8333-333333333333"

func mustStandingFixture(t testing.TB) (*StandingOrderAuthority, *Authority, *journal.Store) {
	t.Helper()
	store := openRulesStore(t)
	clock := &rulesTestClock{now: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)}
	authorizer := &rulesTestAuthorizer{actor: "approver:permission-owner", now: clock.now}
	customers := mustRulesAuthority(t, store, authorizer, clock)
	standings := NewStandingOrderAuthority(store, customers, clock.Now)
	return standings, customers, store
}

func seedStandingRules(t testing.TB, customers *Authority) {
	t.Helper()
	trigger := CustomerRule{
		RuleID: "trigger-1", Scope: CustomerScopeProject, ScopeID: "project-1",
		Action: "publish", Risk: "high", Effect: EffectReportOnly,
		BudgetUnit: "calls", BudgetLimit: 10,
	}
	budget := CustomerRule{
		RuleID: "budget-1", Scope: CustomerScopeProject, ScopeID: "project-1",
		Action: "publish", Risk: "high", Effect: EffectReportOnly,
		BudgetUnit: "calls", BudgetLimit: 2,
	}
	for _, rule := range []CustomerRule{trigger, budget} {
		if _, err := customers.DefineCustomerRule(context.Background(), rule,
			"owner-1", "op-seed-"+rule.RuleID, soTestCorrelation); err != nil {
			t.Fatal(err)
		}
	}
}

func baseStandingOrder() StandingOrder {
	return StandingOrder{
		OrderID: "order-1", Scope: StandingScopeJob, ScopeID: "job-bridge-allow",
		TriggerRuleID: "trigger-1", Tool: permissions.ToolBash, Pattern: "printf",
		BudgetRuleID: "budget-1", MaxIterations: 5,
	}
}

func standingBinding(command string) StandingOrderBinding {
	return StandingOrderBinding{
		JobID: "job-bridge-allow", ProjectID: "project-1", Generation: 1,
		Call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: command},
	}
}

func mustDefineOrder(
	t testing.TB,
	standings *StandingOrderAuthority,
	order StandingOrder,
) StandingOrder {
	t.Helper()
	defined, err := standings.DefineStandingOrder(context.Background(), order, "op-define-1", soTestCorrelation)
	if err != nil {
		t.Fatal(err)
	}
	return defined
}

func mustActivateOrder(
	t testing.TB,
	standings *StandingOrderAuthority,
	orderID string,
) StandingOrder {
	t.Helper()
	activated, err := standings.ActivateStandingOrder(
		context.Background(), orderID, "human:project-owner", "op-activate-1", soTestCorrelation)
	if err != nil {
		t.Fatal(err)
	}
	return activated
}

// TestRedSA1_DefineDefaultsInactive covers RED 1: definition is inactive and
// never dispatches until explicitly activated.
func TestRedSA1_DefineDefaultsInactive(t *testing.T) {
	standings, customers, store := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustDefineOrder(t, standings, baseStandingOrder())
	if order.Active {
		t.Fatal("defined order must default inactive")
	}
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("printf x")); !errors.Is(err, ErrStandingOrderInactive) {
		t.Fatalf("inactive order must not dispatch: %v", err)
	}
	found := false
	for _, event := range mustRulesReadAll(t, store) {
		if event.Type == StandingEventDefined {
			found = true
		}
	}
	if !found {
		t.Fatal("missing StandingOrderDefined fact")
	}
}

// TestRedSA2_ActivateRequiresHuman covers RED 2: activation needs an explicit
// human actor; after activation the gate opens.
func TestRedSA2_ActivateRequiresHuman(t *testing.T) {
	standings, customers, _ := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustDefineOrder(t, standings, baseStandingOrder())
	if _, err := standings.ActivateStandingOrder(
		context.Background(), order.OrderID, "", "op-act-0", soTestCorrelation); err == nil {
		t.Fatal("activation without human actor must fail")
	}
	activated := mustActivateOrder(t, standings, order.OrderID)
	if !activated.Active || activated.AuthorizedBy != "human:project-owner" {
		t.Fatalf("activation state wrong: %+v", activated)
	}
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("printf x")); err != nil {
		t.Fatalf("active order must dispatch: %v", err)
	}
}

// TestRedSA3_FailClosedGate covers RED 3: every condition must hold.
func TestRedSA3_FailClosedGate(t *testing.T) {
	standings, customers, _ := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustActivateOrder(t, standings, mustDefineOrder(t, standings, baseStandingOrder()).OrderID)

	// Scope mismatch.
	mismatch := standingBinding("printf x")
	mismatch.JobID = "job-other"
	if err := standings.CheckDispatch(context.Background(), order.OrderID, mismatch); !errors.Is(err, ErrStandingOrderDenied) {
		t.Fatalf("scope mismatch must deny: %v", err)
	}
	// Trigger mismatch.
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("curl x")); !errors.Is(err, ErrStandingOrderDenied) {
		t.Fatalf("trigger mismatch must deny: %v", err)
	}
	// Trigger rule revoked.
	if _, err := customers.RevokeCustomerRule(context.Background(), "trigger-1", "owner-1", "op-rev-trigger", soTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("printf x")); !errors.Is(err, ErrStandingOrderDenied) {
		t.Fatalf("revoked trigger must deny: %v", err)
	}
}

// TestRedSA5_BudgetExhaustedStops covers RED 5: budget is append-only and
// exhaustion stops dispatch.
func TestRedSA5_BudgetExhaustedStops(t *testing.T) {
	standings, customers, store := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustActivateOrder(t, standings, mustDefineOrder(t, standings, baseStandingOrder()).OrderID)
	for i := 1; i <= 2; i++ {
		if err := standings.ConsumeDispatch(context.Background(), order.OrderID,
			standingBinding("printf x"), "op-dispatch-"+string(rune('a'+i-1)), soTestCorrelation); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("printf x")); !errors.Is(err, ErrRuleBudgetExhausted) {
		t.Fatalf("exhausted budget must stop: %v", err)
	}
	consumed := 0
	for _, event := range mustRulesReadAll(t, store) {
		if event.Type == "BudgetConsumed" {
			consumed++
		}
	}
	if consumed != 2 {
		t.Fatalf("budget consumed %d, want 2", consumed)
	}
}

// TestRedSA6_MaxIterationsStops covers RED 6: max iterations is never
// exceeded.
func TestRedSA6_MaxIterationsStops(t *testing.T) {
	standings, customers, _ := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := baseStandingOrder()
	order.MaxIterations = 2
	order = mustActivateOrder(t, standings, mustDefineOrder(t, standings, order).OrderID)
	for i := 1; i <= 2; i++ {
		if err := standings.ConsumeDispatch(context.Background(), order.OrderID,
			standingBinding("printf x"), "op-max-"+string(rune('0'+i)), soTestCorrelation); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("printf x")); !errors.Is(err, ErrStandingOrderMaxHits) {
		t.Fatalf("max iterations must stop: %v", err)
	}
}

// TestRedSA7_RevokeStops covers RED 7: revocation immediately blocks future
// dispatch.
func TestRedSA7_RevokeStops(t *testing.T) {
	standings, customers, store := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustActivateOrder(t, standings, mustDefineOrder(t, standings, baseStandingOrder()).OrderID)
	if err := standings.RevokeStandingOrder(context.Background(), order.OrderID,
		"human:project-owner", "op-revoke-1", soTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if err := standings.CheckDispatch(context.Background(), order.OrderID, standingBinding("printf x")); !errors.Is(err, ErrStandingOrderRevoked) {
		t.Fatalf("revoked order must stop: %v", err)
	}
	found := false
	for _, event := range mustRulesReadAll(t, store) {
		if event.Type == StandingEventRevoked {
			found = true
		}
	}
	if !found {
		t.Fatal("missing StandingOrderRevoked fact")
	}
}

// TestRedSA8_StaleScopeRejected covers RED 8: stale generation and scope
// mismatches are rejected.
func TestRedSA8_StaleScopeRejected(t *testing.T) {
	standings, customers, _ := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustActivateOrder(t, standings, mustDefineOrder(t, standings, baseStandingOrder()).OrderID)
	stale := standingBinding("printf x")
	stale.Generation = 0
	if err := standings.CheckDispatch(context.Background(), order.OrderID, stale); !errors.Is(err, ErrStandingOrderStale) {
		t.Fatalf("stale generation must reject: %v", err)
	}
}

// TestRedSA9_RebuildFromJournal covers RED 9: the read model reconstructs
// lifecycle state from the Journal with no second authority.
func TestRedSA9_RebuildFromJournal(t *testing.T) {
	standings, customers, _ := mustStandingFixture(t)
	seedStandingRules(t, customers)
	order := mustActivateOrder(t, standings, mustDefineOrder(t, standings, baseStandingOrder()).OrderID)
	if err := standings.ConsumeDispatch(context.Background(), order.OrderID,
		standingBinding("printf x"), "op-rebuild-1", soTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if err := standings.RevokeStandingOrder(context.Background(), order.OrderID,
		"human:project-owner", "op-rebuild-revoke", soTestCorrelation); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := NewStandingOrderAuthority(standings.store, customers, standings.now).Rebuild(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	record := rebuilt.orders[order.OrderID]
	if record.order.RevokedAt == "" || record.order.Active || record.dispatches != 1 {
		t.Fatalf("rebuild mismatch: %+v", record)
	}
}

// TestRedSA10_IsolatedAndIdempotent covers RED 10: orders are isolated and
// replay of the same operation never double-consumes or double-dispatches.
func TestRedSA10_IsolatedAndIdempotent(t *testing.T) {
	standings, customers, store := mustStandingFixture(t)
	seedStandingRules(t, customers)
	first := baseStandingOrder()
	second := baseStandingOrder()
	second.OrderID = "order-2"
	second.MaxIterations = 3
	first = mustActivateOrder(t, standings, mustDefineOrder(t, standings, first).OrderID)
	second = mustActivateOrder(t, standings, mustDefineOrder(t, standings, second).OrderID)

	if err := standings.ConsumeDispatch(context.Background(), first.OrderID,
		standingBinding("printf x"), "op-iso-1", soTestCorrelation); err != nil {
		t.Fatal(err)
	}
	// Replay the exact same operation: no double consumption/dispatch.
	if err := standings.ConsumeDispatch(context.Background(), first.OrderID,
		standingBinding("printf x"), "op-iso-1", soTestCorrelation); err != nil {
		t.Fatal(err)
	}
	if err := standings.CheckDispatch(context.Background(), second.OrderID, standingBinding("printf x")); err != nil {
		t.Fatalf("order-2 must be unaffected: %v", err)
	}
	dispatches := 0
	consumed := 0
	for _, event := range mustRulesReadAll(t, store) {
		if event.Type == StandingEventDispatch && strings.HasSuffix(event.StreamID, "/order-1") {
			dispatches++
		}
		if event.Type == "BudgetConsumed" {
			consumed++
		}
	}
	if dispatches != 1 || consumed != 1 {
		t.Fatalf("replay double-counted: dispatches=%d consumed=%d", dispatches, consumed)
	}
}
