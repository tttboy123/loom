package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/work"
)

func TestLocalProductSetupConfiguresProviderModelRateCardAndRefreshesProjection(t *testing.T) {
	service, _, store, _ := newSetupFixtureService(t)
	now := time.Unix(4_001, 0).UTC()
	authority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x82}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	service.providerModelRateCards = authority
	if _, err := service.writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:  "configure-deepseek-work-rate-card-credential",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			CredentialReference: "credential-ref-deepseek-work-rate-card",
			ExpectedRevision:    0, OccurredAt: time.Unix(4_000, 0).UTC(),
			Status: credentials.CredentialConfigured,
		},
	); err != nil {
		t.Fatal(err)
	}

	command := ProviderModelRateCardCommand{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ModelID: "deepseek-chat", ExpectedRevision: 0, Currency: "USD",
		InputTokenBasis:           work.RateCardInputIncludesCache,
		InputMicrounitsPerMillion: 270_000, OutputMicrounitsPerMillion: 1_100_000,
		CacheReadMicrounitsPerMillion: 70_000, CacheWriteMicrounitsPerMillion: 0,
		RoundingMode:  work.RateCardRoundingCeilingPerAttempt,
		OperationID:   "configure-deepseek-work-chat-v1",
		CorrelationID: "loom-rate-card-test-1",
	}
	result, err := service.ConfigureProviderModelRateCard(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != command.ProviderID ||
		result.ProviderAccountID != command.ProviderAccountID ||
		result.ModelID != command.ModelID || result.Revision != 1 ||
		result.Currency != "USD" || len(result.RateCardDigest) != 64 ||
		result.ConfiguredAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("rate card result = %#v", result)
	}

	snapshot, err := service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var account ProviderAccountDirectoryEntry
	for _, candidate := range snapshot.ProviderAccounts {
		if candidate.ProviderAccountID == command.ProviderAccountID {
			account = candidate
			break
		}
	}
	if len(account.RateCards) != 1 ||
		account.RateCards[0].ModelID != command.ModelID ||
		account.RateCards[0].RateCardDigest != result.RateCardDigest {
		t.Fatalf("setup account rate cards = %#v", account.RateCards)
	}

	retried, err := service.ConfigureProviderModelRateCard(context.Background(), command)
	if err != nil || retried != result {
		t.Fatalf("idempotent retry = %#v, err=%v", retried, err)
	}
}

func TestLocalProductSetupProviderModelRateCardFailsClosedWithoutAuthority(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	_, err := service.ConfigureProviderModelRateCard(
		context.Background(),
		ProviderModelRateCardCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ModelID: "deepseek-chat", ExpectedRevision: 0, Currency: "USD",
			InputTokenBasis: work.RateCardInputExcludesCache,
			RoundingMode:    work.RateCardRoundingCeilingPerAttempt,
			OperationID:     "configure-deepseek-work-chat-v1",
			CorrelationID:   "loom-rate-card-test-unavailable",
		},
	)
	if err != ErrProviderModelRateCardUnavailable {
		t.Fatalf("missing Rate Card authority error = %v", err)
	}
}
