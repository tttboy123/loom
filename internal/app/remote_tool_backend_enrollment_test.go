package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/work"
)

func TestLocalProductSetupConfiguresAndRevokesRemoteToolBackendEnrollment(t *testing.T) {
	service, _, store, _ := newSetupFixtureService(t)
	now := time.Unix(3_101, 0).UTC()
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xc2}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	service.providerAccountPolicies = authority
	service.remoteToolBackendEnrollments = authority
	if _, err := service.writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:  "configure-deepseek-work-enrollment-credential",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			CredentialReference: "credential-ref-deepseek-work-enrollment",
			ExpectedRevision:    0, OccurredAt: time.Unix(3_100, 0).UTC(),
			Status: credentials.CredentialConfigured,
		},
	); err != nil {
		t.Fatal(err)
	}
	policyResult, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 0, MaximumConcurrentAttempts: 4,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 20,
			MaximumAssignedBudgetUnits: 10_000,
			TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
			DataRegion: "apac", OperationID: "configure-deepseek-work-policy-enrollment",
			CorrelationID: "app-enrollment-policy",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	command := RemoteToolBackendEnrollmentCommand{
		EnrollmentID: "search-deepseek-work", BackendKind: "web_search",
		AdapterID: "builtin.search.v1", ProviderID: "deepseek",
		ProviderAccountID:             "deepseek.work",
		ProviderAccountPolicyVersion:  policyResult.PolicyVersion,
		ProviderAccountPolicyRevision: policyResult.Revision,
		ProviderAccountPolicyDigest:   policyResult.PolicyDigest,
		EndpointFingerprint:           strings.Repeat("e", 64), ExpectedRevision: 0,
		MaximumConcurrentCalls: 2, MaximumCallsPerAttempt: 4,
		TimeoutSeconds: 30, MaximumResultBytes: 32 * 1024,
		MaximumBudgetUnits: 2_000, OperationID: "configure-search-deepseek-work",
		CorrelationID: "app-enrollment-configure",
	}
	result, err := service.ConfigureRemoteToolBackendEnrollment(
		context.Background(), command,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.EnrollmentAvailable || !result.PolicyCurrent ||
		result.EnrollmentID != command.EnrollmentID ||
		result.BackendKind != "web_search" || result.Status != "active" ||
		result.Revision != 1 || result.ProviderAccountPolicyDigest != policyResult.PolicyDigest ||
		result.TimeoutSeconds != 30 || result.MaximumResultBytes != 32*1024 ||
		len(result.AllowedTools) != 0 || len(result.EnrollmentDigest) != 64 {
		t.Fatalf("enrollment result = %#v", result)
	}
	snapshot, err := service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	account := setupAccountByID(snapshot, "deepseek.work")
	if len(account.RemoteToolBackends) != 1 ||
		account.RemoteToolBackends[0].EnrollmentDigest != result.EnrollmentDigest ||
		account.RemoteToolBackends[0].Status != "active" ||
		!account.RemoteToolBackends[0].PolicyCurrent {
		t.Fatalf("setup enrollment = %#v", account.RemoteToolBackends)
	}

	now = now.Add(time.Minute)
	if _, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ExpectedRevision: 1, MaximumConcurrentAttempts: 4,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 20,
			MaximumAssignedBudgetUnits: 12_000,
			TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
			DataRegion: "apac", OperationID: "update-deepseek-work-policy-enrollment",
			CorrelationID: "app-enrollment-policy-update",
		},
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	account = setupAccountByID(snapshot, "deepseek.work")
	if len(account.RemoteToolBackends) != 1 ||
		account.RemoteToolBackends[0].PolicyCurrent {
		t.Fatalf("stale-policy setup enrollment = %#v", account.RemoteToolBackends)
	}

	now = now.Add(time.Minute)
	revoked, err := service.RevokeRemoteToolBackendEnrollment(
		context.Background(),
		RemoteToolBackendEnrollmentRevokeCommand{
			EnrollmentID: command.EnrollmentID, ProviderID: "deepseek",
			ProviderAccountID: "deepseek.work", ExpectedRevision: 1,
			OperationID:   "revoke-search-deepseek-work",
			CorrelationID: "app-enrollment-revoke",
		},
	)
	if err != nil || revoked.Status != "revoked" || revoked.Revision != 2 {
		t.Fatalf("revoked result = %#v, err=%v", revoked, err)
	}
	snapshot, err = service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	account = setupAccountByID(snapshot, "deepseek.work")
	if len(account.RemoteToolBackends) != 1 ||
		account.RemoteToolBackends[0].Status != "revoked" ||
		account.RemoteToolBackends[0].Revision != 2 {
		t.Fatalf("revoked setup enrollment = %#v", account.RemoteToolBackends)
	}
}

func TestLocalProductSetupRemoteToolBackendEnrollmentFailsClosedWithoutAuthority(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	_, err := service.ConfigureRemoteToolBackendEnrollment(
		context.Background(), RemoteToolBackendEnrollmentCommand{},
	)
	if err != ErrRemoteToolBackendEnrollmentUnavailable {
		t.Fatalf("missing enrollment authority error = %v", err)
	}
}

func TestLocalProductSetupRemoteToolBackendEnrollmentRejectsOrphanAccount(t *testing.T) {
	service, _, store, _ := newSetupFixtureService(t)
	now := time.Unix(3_200, 0).UTC()
	authority, err := work.NewAuthority(
		store, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0xc3}, 4_096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	service.providerAccountPolicies = authority
	service.remoteToolBackendEnrollments = authority
	policy, err := service.ConfigureProviderAccountPolicy(
		context.Background(), ProviderAccountPolicyCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.orphan",
			ExpectedRevision: 0, MaximumConcurrentAttempts: 2,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 10,
			MaximumAssignedBudgetUnits: 5_000,
			TrustDomain:                "external_provider", RetentionMode: "provider_default",
			DataRegion: "global", OperationID: "configure-deepseek-orphan-policy",
			CorrelationID: "app-enrollment-orphan-policy",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfigureRemoteToolBackendEnrollment(
		context.Background(), RemoteToolBackendEnrollmentCommand{
			EnrollmentID: "search-deepseek-orphan", BackendKind: "web_search",
			AdapterID: "builtin.search.v1", ProviderID: "deepseek",
			ProviderAccountID:             "deepseek.orphan",
			ProviderAccountPolicyVersion:  policy.PolicyVersion,
			ProviderAccountPolicyRevision: policy.Revision,
			ProviderAccountPolicyDigest:   policy.PolicyDigest,
			EndpointFingerprint:           strings.Repeat("f", 64), ExpectedRevision: 0,
			MaximumConcurrentCalls: 1, MaximumCallsPerAttempt: 2,
			TimeoutSeconds: 30, MaximumResultBytes: 4096,
			MaximumBudgetUnits: 1_000, OperationID: "configure-search-deepseek-orphan",
			CorrelationID: "app-enrollment-orphan-configure",
		},
	)
	if err != work.ErrInvalidRemoteToolBackendEnrollment {
		t.Fatalf("orphan account error = %v", err)
	}
}

func setupAccountByID(
	snapshot SetupSnapshot,
	accountID string,
) ProviderAccountDirectoryEntry {
	for _, account := range snapshot.ProviderAccounts {
		if account.ProviderAccountID == accountID {
			return account
		}
	}
	return ProviderAccountDirectoryEntry{}
}
