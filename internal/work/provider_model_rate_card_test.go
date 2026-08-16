package work

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

func TestProviderModelRateCardAuthorityConfiguresVersionsAndIsolatesModels(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa1)

	input := providerModelRateCardTestCommand("deepseek", "deepseek.work", "deepseek-chat", 0)
	first, err := authority.ConfigureProviderModelRateCard(ctx, input)
	if err != nil || !first.Valid() || first.Version() != 1 ||
		first.ProviderID() != input.ProviderID ||
		first.ProviderAccountID() != input.ProviderAccountID ||
		first.ModelID() != input.ModelID || first.Revision() != 1 ||
		first.Currency() != "USD" ||
		first.InputTokenBasis() != RateCardInputIncludesCache ||
		first.RoundingMode() != RateCardRoundingCeilingPerAttempt ||
		first.ConfiguredAt() != testNow || len(first.Digest()) != 64 {
		t.Fatalf("first rate card = %#v, err=%v", first, err)
	}

	replayed, err := authority.ProviderModelRateCard(
		ctx, input.ProviderID, input.ProviderAccountID, input.ModelID,
	)
	if err != nil || replayed != first {
		t.Fatalf("replayed rate card = %#v, err=%v", replayed, err)
	}

	peer := providerModelRateCardTestCommand(
		"deepseek", "deepseek.work", "deepseek-reasoner", 0,
	)
	peer.CommandID = "configure-deepseek-reasoner-v1"
	peer.InputMicrounitsPerMillion = 2_000_000
	if _, err := authority.ConfigureProviderModelRateCard(ctx, peer); err != nil {
		t.Fatal(err)
	}

	clock.Set(testNow.Add(time.Minute))
	updatedInput := input
	updatedInput.CommandID = "configure-deepseek-chat-v2"
	updatedInput.ExpectedRevision = 1
	updatedInput.OutputMicrounitsPerMillion = 3_000_000
	updated, err := authority.ConfigureProviderModelRateCard(ctx, updatedInput)
	if err != nil || updated.Revision() != 2 ||
		updated.OutputMicrounitsPerMillion() != 3_000_000 ||
		updated.Digest() == first.Digest() ||
		updated.ConfiguredAt() != testNow.Add(time.Minute) {
		t.Fatalf("updated rate card = %#v, err=%v", updated, err)
	}
	if retried, err := authority.ConfigureProviderModelRateCard(ctx, updatedInput); err != nil || retried != updated {
		t.Fatalf("idempotent retry = %#v, err=%v", retried, err)
	}

	peerCard, err := authority.ProviderModelRateCard(
		ctx, "deepseek", "deepseek.work", "deepseek-reasoner",
	)
	if err != nil || peerCard.Revision() != 1 ||
		peerCard.InputMicrounitsPerMillion() != 2_000_000 {
		t.Fatalf("peer rate card = %#v, err=%v", peerCard, err)
	}

	events, err := store.ReadStream(ctx, providerModelRateCardStream(
		"deepseek", "deepseek.work", "deepseek-chat",
	))
	if err != nil || len(events) != 2 {
		t.Fatalf("rate card Events = %d, err=%v", len(events), err)
	}
	for _, event := range events {
		text := strings.ToLower(string(event.PayloadJSON))
		for _, forbidden := range []string{"credential-ref-", "api_key", "authorization", "secret", "prompt"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("rate card payload contains %q: %s", forbidden, text)
			}
		}
	}
}

func TestProviderModelRateCardEstimateUsesFrozenTokenBasis(t *testing.T) {
	included, err := NewProviderModelRateCard(ProviderModelRateCardInput{
		Version: 1, ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ModelID: "deepseek-chat", Revision: 1, Currency: "USD",
		InputTokenBasis:           RateCardInputIncludesCache,
		InputMicrounitsPerMillion: 1_000_000, OutputMicrounitsPerMillion: 2_000_000,
		CacheReadMicrounitsPerMillion: 100_000, CacheWriteMicrounitsPerMillion: 1_500_000,
		RoundingMode: RateCardRoundingCeilingPerAttempt, ConfiguredAt: testNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	usage := RunAccounting{
		UsageObserved: true, InputTokens: 1_000_000, OutputTokens: 500_000,
		CacheReadTokens: 200_000, CacheWriteTokens: 100_000, TotalTokens: 1_500_000,
	}
	estimated, err := EstimateRunAccounting(usage, included)
	if err != nil || !estimated.CostObserved || estimated.CostMicrounits != 1_970_000 ||
		estimated.CostCurrency != "USD" || estimated.CostSource != CostSourceRateCardEstimate {
		t.Fatalf("included-cache estimate = %#v, err=%v", estimated, err)
	}

	excludedInput := ProviderModelRateCardInput{
		Version: 1, ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ModelID: "deepseek-chat", Revision: 1, Currency: "USD",
		InputTokenBasis:           RateCardInputExcludesCache,
		InputMicrounitsPerMillion: 1_000_000, OutputMicrounitsPerMillion: 2_000_000,
		CacheReadMicrounitsPerMillion: 100_000, CacheWriteMicrounitsPerMillion: 1_500_000,
		RoundingMode: RateCardRoundingCeilingPerAttempt, ConfiguredAt: testNow,
	}
	excluded, err := NewProviderModelRateCard(excludedInput)
	if err != nil {
		t.Fatal(err)
	}
	estimated, err = EstimateRunAccounting(usage, excluded)
	if err != nil || estimated.CostMicrounits != 2_170_000 {
		t.Fatalf("excluded-cache estimate = %#v, err=%v", estimated, err)
	}

	badCache := usage
	badCache.CacheReadTokens = badCache.InputTokens + 1
	if _, err := EstimateRunAccounting(badCache, included); !errors.Is(err, ErrInvalidProviderModelRateCardEstimate) {
		t.Fatalf("cache substitution error = %v", err)
	}
	overflow := usage
	overflow.InputTokens = math.MaxInt64
	overflow.OutputTokens = 0
	overflow.TotalTokens = math.MaxInt64
	overflowInput := excludedInput
	overflowInput.InputMicrounitsPerMillion = maximumRateMicrounitsPerMillion
	overflowCard, err := NewProviderModelRateCard(overflowInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EstimateRunAccounting(overflow, overflowCard); !errors.Is(err, ErrInvalidProviderModelRateCardEstimate) {
		t.Fatalf("overflow estimate error = %v", err)
	}
	reported := usage
	reported.CostObserved = true
	reported.CostMicrounits = 7
	reported.CostCurrency = "USD"
	reported.CostSource = CostSourceProviderReported
	if got, err := EstimateRunAccounting(reported, included); err != nil || got != reported {
		t.Fatalf("reported accounting changed = %#v, err=%v", got, err)
	}
}

func TestProviderModelRateCardRejectsMalformedAuthorityFact(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0xa2)
	command := providerModelRateCardTestCommand("deepseek", "deepseek.work", "deepseek-chat", 0)
	payload, err := json.Marshal(map[string]any{
		"command_id": command.CommandID, "rate_card_version": 1,
		"provider_id": command.ProviderID, "provider_account_id": command.ProviderAccountID,
		"model_id": command.ModelID, "revision": 1, "currency": "USD",
		"input_token_basis":                  RateCardInputIncludesCache,
		"input_microunits_per_million":       int64(1_000_000),
		"output_microunits_per_million":      int64(2_000_000),
		"cache_read_microunits_per_million":  int64(100_000),
		"cache_write_microunits_per_million": int64(1_500_000),
		"rounding_mode":                      RateCardRoundingCeilingPerAttempt,
		"configured_at":                      testNow.Format(time.RFC3339Nano),
		"rate_card_digest":                   strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	streamID := providerModelRateCardStream(command.ProviderID, command.ProviderAccountID, command.ModelID)
	if _, err := store.Append(ctx, journal.Event{
		ID: "malformed-rate-card", IdempotencyKey: "malformed-rate-card",
		StreamID: streamID, Seq: 1, Type: "ProviderModelRateCardConfigured",
		SchemaVersion: 1, EmittedAt: testNow, CorrelationID: testCorrelation,
		PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ProviderModelRateCard(
		ctx, command.ProviderID, command.ProviderAccountID, command.ModelID,
	); !errors.Is(err, ErrProviderModelRateCardConflict) {
		t.Fatalf("malformed replay error = %v", err)
	}
}

func TestProviderModelRateCardClaimFreezesRevisionAndPriceDriftAffectsOnlyFutureRuns(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 4)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa3)
	command := providerModelRateCardTestCommand(
		"deepseek", "deepseek.work", "deepseek-chat", 0,
	)
	firstCard, err := authority.ConfigureProviderModelRateCard(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	firstInput := providerAccountCapacityAssignment(
		t, "work-rate-card-1", "run-rate-card-1", "deepseek.work", 0,
	)
	if _, _, err := authority.CreateAndAssign(ctx, firstInput); err != nil {
		t.Fatal(err)
	}
	_, firstRun, err := authority.Claim(
		ctx, claim(firstInput.WorkItemID, firstInput.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	frozen, ok := firstRun.ProviderModelRateCard()
	if !ok || frozen != firstCard {
		t.Fatalf("first frozen rate card = %#v, ok=%t", frozen, ok)
	}
	generation := generationInput(firstRun)
	if _, started, err := authority.Start(ctx, generation); err != nil {
		t.Fatalf("Start with frozen Rate Card error = %v", err)
	} else if startedCard, available := started.ProviderModelRateCard(); !available || startedCard != firstCard {
		t.Fatalf("started frozen Rate Card = %#v, available=%t", startedCard, available)
	}
	terminalCommand := RunTerminalInput{
		RunGenerationInput: generation,
		Status:             "failed", Reason: "fixture_failed",
		Accounting: &RunAccounting{
			UsageObserved: true, InputTokens: 1, TotalTokens: 1,
			CostObserved: true, CostMicrounits: 1, CostCurrency: "USD",
			CostSource: CostSourceRateCardEstimate,
		},
	}
	if _, terminal, err := authority.CommitTerminal(ctx, terminalCommand); err != nil {
		t.Fatalf("CommitTerminal with frozen Rate Card error = %v", err)
	} else if terminalCard, available := terminal.ProviderModelRateCard(); !available || terminalCard != firstCard {
		t.Fatalf("terminal frozen Rate Card = %#v, available=%t", terminalCard, available)
	}

	clock.Set(testNow.Add(time.Minute))
	updatedCommand := command
	updatedCommand.CommandID = "configure-deepseek-chat-v2-after-claim"
	updatedCommand.ExpectedRevision = 1
	updatedCommand.InputMicrounitsPerMillion = 9_000_000
	updatedCard, err := authority.ConfigureProviderModelRateCard(ctx, updatedCommand)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := authority.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range snapshot.Runs() {
		if run.ID() != firstRun.ID() {
			continue
		}
		replayed, available := run.ProviderModelRateCard()
		if !available || replayed != firstCard || replayed == updatedCard {
			t.Fatalf("replayed frozen rate card = %#v, available=%t", replayed, available)
		}
	}

	secondInput := providerAccountCapacityAssignment(
		t, "work-rate-card-2", "run-rate-card-2", "deepseek.work", 0,
	)
	if _, _, err := authority.CreateAndAssign(ctx, secondInput); err != nil {
		t.Fatal(err)
	}
	_, secondRun, err := authority.Claim(
		ctx, claim(secondInput.WorkItemID, secondInput.RunID, "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondFrozen, ok := secondRun.ProviderModelRateCard()
	if !ok || secondFrozen != updatedCard {
		t.Fatalf("future frozen rate card = %#v, ok=%t", secondFrozen, ok)
	}

	events, err := store.ReadStream(ctx, runStream(firstRun.ID()))
	if err != nil || len(events) != 3 || events[0].Type != "RunClaimed" ||
		events[1].Type != "RunStarted" || events[2].Type != "RunTerminalCommitted" ||
		!strings.Contains(string(events[0].PayloadJSON), `"claim_contract_version":2`) ||
		!strings.Contains(string(events[0].PayloadJSON), firstCard.Digest()) {
		t.Fatalf("frozen Run lifecycle Events = %#v, err=%v", events, err)
	}
}

func providerModelRateCardTestCommand(
	providerID, accountID, modelID string,
	expectedRevision int64,
) ProviderModelRateCardCommand {
	return ProviderModelRateCardCommand{
		CommandID:  "configure-" + strings.ReplaceAll(accountID, ".", "-") + "-" + modelID,
		ProviderID: providerID, ProviderAccountID: accountID, ModelID: modelID,
		ExpectedRevision: expectedRevision, Currency: "USD",
		InputTokenBasis:                RateCardInputIncludesCache,
		InputMicrounitsPerMillion:      1_000_000,
		OutputMicrounitsPerMillion:     2_000_000,
		CacheReadMicrounitsPerMillion:  100_000,
		CacheWriteMicrounitsPerMillion: 1_500_000,
		RoundingMode:                   RateCardRoundingCeilingPerAttempt,
		CorrelationID:                  testCorrelation,
	}
}
