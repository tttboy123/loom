package work

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

func TestProviderAccountPolicyAuthorityConfiguresVersionsAndReplays(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x91)

	input := providerAccountPolicyTestInput("deepseek", "deepseek.work", 0)
	first, err := authority.ConfigureProviderAccountPolicy(ctx, input)
	if err != nil || !first.Valid() || first.Version() != 1 ||
		first.ProviderID() != input.ProviderID ||
		first.ProviderAccountID() != input.ProviderAccountID ||
		first.Revision() != 1 || first.MaximumConcurrentAttempts() != 2 ||
		first.DispatchWindow() != time.Minute ||
		first.MaximumDispatchStarts() != 12 ||
		first.MaximumAssignedBudgetUnits() != 5_000 ||
		first.ConfiguredAt() != testNow || len(first.Digest()) != 64 {
		t.Fatalf("first policy = %#v, err=%v", first, err)
	}

	replayed, err := authority.ProviderAccountPolicy(
		ctx, input.ProviderID, input.ProviderAccountID,
	)
	if err != nil || replayed != first {
		t.Fatalf("replayed policy = %#v, err=%v", replayed, err)
	}

	clock.Set(testNow.Add(time.Minute))
	updatedInput := input
	updatedInput.CommandID = "configure-deepseek-work-v2"
	updatedInput.ExpectedRevision = 1
	updatedInput.MaximumConcurrentAttempts = 3
	updatedInput.MaximumAssignedBudgetUnits = 8_000
	updated, err := authority.ConfigureProviderAccountPolicy(ctx, updatedInput)
	if err != nil || updated.Revision() != 2 ||
		updated.MaximumConcurrentAttempts() != 3 ||
		updated.MaximumAssignedBudgetUnits() != 8_000 ||
		updated.Digest() == first.Digest() ||
		updated.ConfiguredAt() != testNow.Add(time.Minute) {
		t.Fatalf("updated policy = %#v, err=%v", updated, err)
	}

	retried, err := authority.ConfigureProviderAccountPolicy(ctx, updatedInput)
	if err != nil || retried != updated {
		t.Fatalf("idempotent retry = %#v, err=%v", retried, err)
	}
	events, err := store.ReadStream(
		ctx, providerAccountPolicyStream("deepseek.work"),
	)
	if err != nil || len(events) != 2 {
		t.Fatalf("policy Events = %d, err=%v", len(events), err)
	}
	for _, event := range events {
		text := string(event.PayloadJSON)
		for _, forbidden := range []string{
			"credential-ref-", "api_key", "authorization", "secret", "prompt",
		} {
			if strings.Contains(strings.ToLower(text), forbidden) {
				t.Fatalf("policy payload contains %q: %s", forbidden, text)
			}
		}
	}
}

func TestProviderAccountPolicyRejectsNonAdvancingRevisionTime(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x92)
	input := providerAccountPolicyTestInput("deepseek", "deepseek.work", 0)
	if _, err := authority.ConfigureProviderAccountPolicy(ctx, input); err != nil {
		t.Fatal(err)
	}
	updated := input
	updated.CommandID = "configure-deepseek-work-same-time-v2"
	updated.ExpectedRevision = 1
	updated.MaximumConcurrentAttempts = 3
	if _, err := authority.ConfigureProviderAccountPolicy(
		ctx, updated,
	); !errors.Is(err, ErrProviderAccountPolicyConflict) {
		t.Fatalf("same-time policy revision error = %v", err)
	}
}

func TestProviderAccountPolicyValueIsCanonicalAndRejectsInvalidInput(t *testing.T) {
	input := ProviderAccountPolicyInput{
		Version: 1, ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		Revision: 3, MaximumConcurrentAttempts: 4,
		DispatchWindow: time.Minute, MaximumDispatchStarts: 20,
		MaximumAssignedBudgetUnits: 8_000, ConfiguredAt: testNow,
	}
	first, err := NewProviderAccountPolicy(input)
	if err != nil || !first.Valid() {
		t.Fatalf("NewProviderAccountPolicy() = %#v, %v", first, err)
	}
	second, err := NewProviderAccountPolicy(input)
	if err != nil || second != first || second.Digest() != first.Digest() {
		t.Fatalf("canonical policy = %#v, %v", second, err)
	}

	invalid := input
	invalid.ProviderAccountID = "anthropic.work"
	if _, err := NewProviderAccountPolicy(invalid); !errors.Is(err, ErrInvalidProviderAccountPolicy) {
		t.Fatalf("foreign account error = %v", err)
	}
	invalid = input
	invalid.ConfiguredAt = testNow.In(time.FixedZone("non-utc", 3600))
	if _, err := NewProviderAccountPolicy(invalid); !errors.Is(err, ErrInvalidProviderAccountPolicy) {
		t.Fatalf("non-UTC time error = %v", err)
	}
}

func TestProviderAccountPolicyV2FreezesDisclosurePolicyAndPreservesV1Replay(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x9a)

	legacy := providerAccountPolicyTestInput("deepseek", "deepseek.work", 0)
	first, err := authority.ConfigureProviderAccountPolicy(ctx, legacy)
	if err != nil || first.Version() != 1 || first.TrustDomain() != "" ||
		first.RetentionMode() != "" || first.DataRegion() != "" {
		t.Fatalf("legacy policy = %#v, err=%v", first, err)
	}

	clock.Set(testNow.Add(time.Minute))
	configured := legacy
	configured.CommandID = "configure-deepseek-work-disclosure-v2"
	configured.ExpectedRevision = 1
	configured.TrustDomain = "external_provider"
	configured.RetentionMode = "zero_data_retention"
	configured.DataRegion = "apac"
	second, err := authority.ConfigureProviderAccountPolicy(ctx, configured)
	if err != nil || second.Version() != 2 ||
		second.TrustDomain() != configured.TrustDomain ||
		second.RetentionMode() != configured.RetentionMode ||
		second.DataRegion() != configured.DataRegion ||
		second.Digest() == first.Digest() {
		t.Fatalf("v2 policy = %#v, err=%v", second, err)
	}

	replayed, err := authority.ProviderAccountPolicy(
		ctx, configured.ProviderID, configured.ProviderAccountID,
	)
	if err != nil || replayed != second {
		t.Fatalf("replayed v2 policy = %#v, err=%v", replayed, err)
	}
	events, err := store.ReadStream(ctx, providerAccountPolicyStream("deepseek.work"))
	if err != nil || len(events) != 2 {
		t.Fatalf("policy events = %d, err=%v", len(events), err)
	}
	legacyDecoded, _, err := DecodeProviderAccountPolicyConfiguredEvent(events[0], "")
	if err != nil || legacyDecoded != first {
		t.Fatalf("legacy replay = %#v, err=%v", legacyDecoded, err)
	}
}

func TestProviderAccountPolicyV2RejectsUnknownDisclosureValues(t *testing.T) {
	input := ProviderAccountPolicyInput{
		Version: 2, ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		Revision: 3, MaximumConcurrentAttempts: 4,
		DispatchWindow: time.Minute, MaximumDispatchStarts: 20,
		MaximumAssignedBudgetUnits: 8_000, ConfiguredAt: testNow,
		TrustDomain: "external_provider", RetentionMode: "provider_default",
		DataRegion: "global",
	}
	for name, mutate := range map[string]func(*ProviderAccountPolicyInput){
		"trust domain": func(value *ProviderAccountPolicyInput) { value.TrustDomain = "custom" },
		"retention":    func(value *ProviderAccountPolicyInput) { value.RetentionMode = "forever" },
		"region":       func(value *ProviderAccountPolicyInput) { value.DataRegion = "unknown-region" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := input
			mutate(&candidate)
			if _, err := NewProviderAccountPolicy(candidate); !errors.Is(err, ErrInvalidProviderAccountPolicy) {
				t.Fatalf("NewProviderAccountPolicy() error = %v", err)
			}
		})
	}
}

func TestProviderAccountPolicyAuthorityRejectsInvalidAndStaleCommands(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0x92)
	valid := providerAccountPolicyTestInput("openai", "openai.primary", 0)

	tests := map[string]func(*ProviderAccountPolicyCommand){
		"command":         func(input *ProviderAccountPolicyCommand) { input.CommandID = "" },
		"provider":        func(input *ProviderAccountPolicyCommand) { input.ProviderID = "OpenAI" },
		"foreign account": func(input *ProviderAccountPolicyCommand) { input.ProviderAccountID = "anthropic.primary" },
		"revision":        func(input *ProviderAccountPolicyCommand) { input.ExpectedRevision = -1 },
		"concurrency":     func(input *ProviderAccountPolicyCommand) { input.MaximumConcurrentAttempts = 0 },
		"window":          func(input *ProviderAccountPolicyCommand) { input.DispatchWindow = 0 },
		"dispatch starts": func(input *ProviderAccountPolicyCommand) { input.MaximumDispatchStarts = 0 },
		"budget":          func(input *ProviderAccountPolicyCommand) { input.MaximumAssignedBudgetUnits = -1 },
		"correlation":     func(input *ProviderAccountPolicyCommand) { input.CorrelationID = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			if _, err := authority.ConfigureProviderAccountPolicy(ctx, input); !errors.Is(err, ErrInvalidProviderAccountPolicy) {
				t.Fatalf("ConfigureProviderAccountPolicy() error = %v", err)
			}
		})
	}

	if _, err := authority.ConfigureProviderAccountPolicy(ctx, valid); err != nil {
		t.Fatal(err)
	}
	stale := valid
	stale.CommandID = "stale-openai-policy"
	stale.MaximumConcurrentAttempts = 4
	if _, err := authority.ConfigureProviderAccountPolicy(ctx, stale); !errors.Is(err, ErrProviderAccountPolicyConflict) {
		t.Fatalf("stale policy error = %v", err)
	}
	conflictingRetry := valid
	conflictingRetry.MaximumConcurrentAttempts = 5
	if _, err := authority.ConfigureProviderAccountPolicy(ctx, conflictingRetry); !errors.Is(err, ErrProviderAccountPolicyConflict) {
		t.Fatalf("conflicting idempotency error = %v", err)
	}
	if _, err := authority.ProviderAccountPolicy(ctx, "openai", "openai.missing"); !errors.Is(err, ErrProviderAccountPolicyNotFound) {
		t.Fatalf("missing policy error = %v", err)
	}
}

func TestProviderAccountPolicyAuthorityCASIsAccountLocal(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x93)
	for _, account := range []string{"deepseek.work", "deepseek.review"} {
		input := providerAccountPolicyTestInput("deepseek", account, 0)
		input.CommandID = "configure-" + strings.ReplaceAll(account, ".", "-")
		if _, err := authority.ConfigureProviderAccountPolicy(ctx, input); err != nil {
			t.Fatal(err)
		}
	}

	clock.Set(testNow.Add(time.Second))
	base := providerAccountPolicyTestInput("deepseek", "deepseek.work", 1)
	base.MaximumConcurrentAttempts = 3
	var wait sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for _, commandID := range []string{"update-work-a", "update-work-b"} {
		wait.Add(1)
		go func(commandID string) {
			defer wait.Done()
			input := base
			input.CommandID = commandID
			_, err := authority.ConfigureProviderAccountPolicy(ctx, input)
			errorsSeen <- err
		}(commandID)
	}
	wait.Wait()
	close(errorsSeen)
	succeeded := 0
	conflicted := 0
	for err := range errorsSeen {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrProviderAccountPolicyConflict):
			conflicted++
		default:
			t.Fatalf("concurrent policy error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("CAS results succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	review, err := authority.ProviderAccountPolicy(ctx, "deepseek", "deepseek.review")
	if err != nil || review.Revision() != 1 || review.MaximumConcurrentAttempts() != 2 {
		t.Fatalf("peer account policy = %#v, err=%v", review, err)
	}
}

func TestProviderAccountPolicyReplayRejectsMalformedAuthorityFact(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0x94)
	payload, err := json.Marshal(map[string]any{
		"command_id": "malformed-policy", "policy_version": 1,
		"provider_id": "deepseek", "provider_account_id": "deepseek.work",
		"revision": 1, "maximum_concurrent_attempts": 2,
		"dispatch_window_nanoseconds": int64(time.Minute),
		"maximum_dispatch_starts":     12, "maximum_assigned_budget_units": 5_000,
		"configured_at": testNow.Format(time.RFC3339Nano),
		"policy_digest": strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	event := journal.Event{
		ID: "malformed-provider-account-policy", IdempotencyKey: "malformed-provider-account-policy",
		StreamID: providerAccountPolicyStream("deepseek.work"), Seq: 1,
		Type: "ProviderAccountPolicyConfigured", SchemaVersion: 1,
		EmittedAt: testNow, CorrelationID: testCorrelation, PayloadJSON: payload,
	}
	if _, err := store.Append(ctx, event); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ProviderAccountPolicy(ctx, "deepseek", "deepseek.work"); !errors.Is(err, ErrProviderAccountPolicyConflict) {
		t.Fatalf("malformed replay error = %v", err)
	}
}

func providerAccountPolicyTestInput(
	providerID string,
	accountID string,
	expectedRevision int64,
) ProviderAccountPolicyCommand {
	return ProviderAccountPolicyCommand{
		CommandID:  "configure-" + strings.ReplaceAll(accountID, ".", "-"),
		ProviderID: providerID, ProviderAccountID: accountID,
		ExpectedRevision:          expectedRevision,
		MaximumConcurrentAttempts: 2,
		DispatchWindow:            time.Minute, MaximumDispatchStarts: 12,
		MaximumAssignedBudgetUnits: 5_000,
		CorrelationID:              testCorrelation,
	}
}
