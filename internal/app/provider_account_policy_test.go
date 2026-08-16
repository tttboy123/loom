package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/work"
)

func TestLocalProductSetupConfiguresProviderAccountPolicyAndRefreshesProjection(t *testing.T) {
	service, _, store, _ := newSetupFixtureService(t)
	now := time.Unix(3_001, 0).UTC()
	authority, err := work.NewAuthority(
		store,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x81}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	service.providerAccountPolicies = authority
	if _, err := service.writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:  "configure-deepseek-work-credential",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			CredentialReference: "credential-ref-deepseek-work",
			ExpectedRevision:    0, OccurredAt: time.Unix(3_000, 0).UTC(),
			Status: credentials.CredentialConfigured,
		},
	); err != nil {
		t.Fatal(err)
	}

	result, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 0, MaximumConcurrentAttempts: 3,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 12,
			MaximumAssignedBudgetUnits: 8_000,
			TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
			DataRegion:    "apac",
			OperationID:   "configure-deepseek-work-v1",
			CorrelationID: "loom-policy-test-1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.PolicyAvailable || result.ProviderID != "deepseek" ||
		result.ProviderAccountID != "deepseek.work" || result.Revision != 1 ||
		result.MaximumConcurrentAttempts != 3 ||
		result.DispatchWindowSeconds != 60 ||
		result.MaximumDispatchStarts != 12 ||
		result.MaximumAssignedBudgetUnits != 8_000 ||
		result.PolicyVersion != 2 || result.TrustDomain != "external_provider" ||
		result.RetentionMode != "zero_data_retention" || result.DataRegion != "apac" ||
		len(result.PolicyDigest) != 64 || result.ConfiguredAt != now.Format(time.RFC3339) {
		t.Fatalf("policy result = %#v", result)
	}
	projected, ok := service.projection.GlobalReadView().ProviderAccountPolicy(
		"deepseek", "deepseek.work",
	)
	if !ok || projected.Revision() != result.Revision ||
		projected.Digest() != result.PolicyDigest {
		t.Fatalf("projected policy = %#v, ok=%t", projected, ok)
	}
	snapshot, err := service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var account ProviderAccountDirectoryEntry
	for _, candidate := range snapshot.ProviderAccounts {
		if candidate.ProviderAccountID == "deepseek.work" {
			account = candidate
			break
		}
	}
	if !account.PolicyAvailable || account.PolicyRevision != 1 ||
		len(account.PolicyDigest) != 64 ||
		account.MaximumConcurrentAttempts != 3 ||
		account.DispatchWindowSeconds != 60 ||
		account.MaximumDispatchStarts != 12 ||
		account.MaximumAssignedBudgetUnits != 8_000 {
		// Disclosure fields are checked separately to keep the limit assertion readable.
		t.Fatalf("setup account policy = %#v", account)
	}
	if account.PolicyVersion != 2 || account.TrustDomain != "external_provider" ||
		account.RetentionMode != "zero_data_retention" || account.DataRegion != "apac" {
		t.Fatalf("setup account disclosure policy = %#v", account)
	}

	retried, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 0, MaximumConcurrentAttempts: 3,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 12,
			MaximumAssignedBudgetUnits: 8_000,
			TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
			DataRegion:    "apac",
			OperationID:   "configure-deepseek-work-v1",
			CorrelationID: "loom-policy-test-retry",
		},
	)
	if err != nil || retried != result {
		t.Fatalf("idempotent retry = %#v, err=%v", retried, err)
	}
}

func TestLocalProductSetupProviderAccountPolicyFailsClosedWithoutAuthority(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	_, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 0, MaximumConcurrentAttempts: 1,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 1,
			OperationID:   "configure-deepseek-work-v1",
			CorrelationID: "loom-policy-test-unavailable",
		},
	)
	if err != ErrProviderAccountPolicyUnavailable {
		t.Fatalf("missing policy authority error = %v", err)
	}
}
