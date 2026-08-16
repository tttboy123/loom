package projection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

func TestProviderAccountPolicyProjectionRebuildsExactAccountPolicies(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x72}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, input := range []work.ProviderAccountPolicyCommand{
		projectionProviderAccountPolicyCommand("deepseek", "deepseek.work", 0),
		projectionProviderAccountPolicyCommand("deepseek", "deepseek.review", 0),
		projectionProviderAccountPolicyCommand("openai", "openai.primary", 0),
	} {
		if _, err := authority.ConfigureProviderAccountPolicy(ctx, input); err != nil {
			t.Fatal(err)
		}
	}

	readModel := New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()
	workPolicy, ok := view.ProviderAccountPolicy("deepseek", "deepseek.work")
	if !ok || workPolicy.Revision() != 1 ||
		workPolicy.MaximumConcurrentAttempts() != 2 ||
		workPolicy.MaximumAssignedBudgetUnits() != 5_000 {
		t.Fatalf("work policy = %#v, ok=%t", workPolicy, ok)
	}
	if _, ok := view.ProviderAccountPolicy("openai", "deepseek.work"); ok {
		t.Fatal("cross-Provider policy lookup succeeded")
	}
	policies := view.ProviderAccountPolicies("deepseek")
	if len(policies) != 2 ||
		policies[0].ProviderAccountID() != "deepseek.review" ||
		policies[1].ProviderAccountID() != "deepseek.work" {
		t.Fatalf("DeepSeek policies = %#v", policies)
	}

	now = now.Add(time.Minute)
	updated := projectionProviderAccountPolicyCommand(
		"deepseek", "deepseek.work", 1,
	)
	updated.CommandID = "update-deepseek-work-v2"
	updated.MaximumConcurrentAttempts = 3
	updated.MaximumAssignedBudgetUnits = 8_000
	if _, err := authority.ConfigureProviderAccountPolicy(ctx, updated); err != nil {
		t.Fatal(err)
	}
	restarted := New(database)
	if err := restarted.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	updatedPolicy, ok := restarted.GlobalReadView().ProviderAccountPolicy(
		"deepseek", "deepseek.work",
	)
	if !ok || updatedPolicy.Revision() != 2 ||
		updatedPolicy.MaximumConcurrentAttempts() != 3 ||
		updatedPolicy.MaximumAssignedBudgetUnits() != 8_000 {
		t.Fatalf("updated policy = %#v, ok=%t", updatedPolicy, ok)
	}
	peer, ok := restarted.GlobalReadView().ProviderAccountPolicy(
		"deepseek", "deepseek.review",
	)
	if !ok || peer.Revision() != 1 {
		t.Fatalf("peer policy = %#v, ok=%t", peer, ok)
	}
}

func TestProviderAccountPolicyProjectionFailurePreservesPriorView(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x73}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	input := projectionProviderAccountPolicyCommand(
		"deepseek", "deepseek.work", 0,
	)
	if _, err := authority.ConfigureProviderAccountPolicy(ctx, input); err != nil {
		t.Fatal(err)
	}
	readModel := New(database)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	accepted := readModel.GlobalReadView()
	before, ok := accepted.ProviderAccountPolicy("deepseek", "deepseek.work")
	if !ok || before.Revision() != 1 {
		t.Fatalf("accepted policy = %#v, ok=%t", before, ok)
	}

	events, err := store.ReadStream(ctx, "provider-account-policy/deepseek.work")
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d, err=%v", len(events), err)
	}
	badPayload, err := json.Marshal(map[string]any{
		"command_id": "malformed-policy-v2", "policy_version": 1,
		"provider_id": "deepseek", "provider_account_id": "deepseek.work",
		"revision": 2, "maximum_concurrent_attempts": 4,
		"dispatch_window_nanoseconds": int64(time.Minute),
		"maximum_dispatch_starts":     20, "maximum_assigned_budget_units": int64(9_000),
		"configured_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"policy_digest": strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(ctx, journal.Event{
		ID:             "malformed-provider-account-policy-v2",
		IdempotencyKey: "malformed-provider-account-policy-v2",
		StreamID:       "provider-account-policy/deepseek.work", Seq: 2,
		Type: "ProviderAccountPolicyConfigured", SchemaVersion: 1,
		EmittedAt: now.Add(time.Minute), CorrelationID: "policy-projection-bad",
		CausationID: events[0].ID, PayloadJSON: badPayload,
	}); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(ctx); !errors.Is(err, ErrInvalidProjectionEvent) {
		t.Fatalf("malformed rebuild error = %v", err)
	}
	after, ok := readModel.GlobalReadView().ProviderAccountPolicy(
		"deepseek", "deepseek.work",
	)
	if !ok || after != before {
		t.Fatalf("prior view changed: before=%#v after=%#v ok=%t", before, after, ok)
	}
}

func projectionProviderAccountPolicyCommand(
	providerID string,
	accountID string,
	expectedRevision int64,
) work.ProviderAccountPolicyCommand {
	return work.ProviderAccountPolicyCommand{
		CommandID:  "configure-" + strings.ReplaceAll(accountID, ".", "-"),
		ProviderID: providerID, ProviderAccountID: accountID,
		ExpectedRevision:          expectedRevision,
		MaximumConcurrentAttempts: 2, DispatchWindow: time.Minute,
		MaximumDispatchStarts: 12, MaximumAssignedBudgetUnits: 5_000,
		CorrelationID: "provider-account-policy-projection",
	}
}
