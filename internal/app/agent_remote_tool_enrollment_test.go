package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

func mustAppAccountBaseline(
	t *testing.T,
	service *LocalProductSetupService,
	authority *work.Authority,
	now *time.Time,
	accountID string,
) ProviderAccountPolicyResult {
	t.Helper()
	credentialReference := "credential-ref-" + strings.ReplaceAll(accountID, ".", "-")
	if _, err := service.writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "enrollment-test-credential-" + accountID,
			ProviderID:          "openai",
			ProviderAccountID:   accountID,
			CredentialReference: credentialReference,
			ExpectedRevision:    0,
			OccurredAt:          *now,
			Status:              credentials.CredentialConfigured,
		},
	); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	policyResult, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "openai", ProviderAccountID: accountID,
			ExpectedRevision: 0, MaximumConcurrentAttempts: 4,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 20,
			MaximumAssignedBudgetUnits: 10_000,
			TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
			DataRegion: "apac", OperationID: "enrollment-test-policy-" + accountID,
			CorrelationID: "enrollment-test-policy",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return policyResult
}

func mustAppEnrollment(
	t *testing.T,
	service *LocalProductSetupService,
	authority *work.Authority,
	now *time.Time,
	accountID string,
	policyResult ProviderAccountPolicyResult,
	enrollmentID string,
	adapterID string,
	backendKind string,
	mcpServerID string,
	allowedTools []string,
) work.RemoteToolBackendEnrollment {
	t.Helper()
	*now = now.Add(time.Minute)
	_, err := service.ConfigureRemoteToolBackendEnrollment(
		context.Background(),
		RemoteToolBackendEnrollmentCommand{
			EnrollmentID: enrollmentID, BackendKind: backendKind,
			AdapterID:                     adapterID,
			ProviderID:                    "openai",
			ProviderAccountID:             accountID,
			ProviderAccountPolicyVersion:  policyResult.PolicyVersion,
			ProviderAccountPolicyRevision: policyResult.Revision,
			ProviderAccountPolicyDigest:   policyResult.PolicyDigest,
			EndpointFingerprint:           strings.Repeat("e", 64),
			ExpectedRevision:              0,
			MCPServerID:                   mcpServerID,
			AllowedTools:                  allowedTools,
			MaximumConcurrentCalls:        2, MaximumCallsPerAttempt: 4,
			TimeoutSeconds: 30, MaximumResultBytes: 32 * 1024,
			MaximumBudgetUnits: 2_000, OperationID: "enrollment-test-" + enrollmentID,
			CorrelationID: "enrollment-test",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	enrollment, err := authority.RemoteToolBackendEnrollment(
		context.Background(), enrollmentID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return enrollment
}

func appEnrollmentProfile(
	profileID string,
	accountID string,
	enrollmentID string,
	enrollmentDigest string,
) loomruntime.RuntimeProfile {
	return loomruntime.RuntimeProfile{
		ID: profileID, AdapterType: "codex",
		ProviderID: "openai", ProviderAccountID: accountID,
		ModelID: "model-a", AuthMode: loomruntime.AuthBrokered,
		EndpointFingerprint:        strings.Repeat("f", 64),
		CredentialReference:        "credential-ref-" + strings.ReplaceAll(accountID, ".", "-"),
		CredentialRevision:         1,
		ReasoningEffort:            "high",
		RequiredCapabilities:       []string{loomruntime.CapabilityGovernedToolLoop, loomruntime.CapabilityReasoningEffort},
		Timeout:                    90 * time.Second,
		RemoteToolEnrollmentID:     enrollmentID,
		RemoteToolEnrollmentDigest: enrollmentDigest,
	}
}

func appEnrollmentHarness(
	t *testing.T,
) (
	*LocalProductSetupService,
	*projection.Projection,
	*work.Authority,
	*time.Time,
) {
	t.Helper()
	service, database, store, _ := newSetupFixtureService(t)
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
	readModel := projection.New(database)
	return service, readModel, authority, &now
}

func TestResolveMissionExecutionEnrollmentBlocksAndSucceeds(t *testing.T) {
	service, readModel, authority, now := appEnrollmentHarness(t)
	policyResult := mustAppAccountBaseline(
		t, service, authority, now, "openai.primary",
	)
	enrollment := mustAppEnrollment(
		t, service, authority, now, "openai.primary", policyResult,
		"enr-search", work.BuiltInSearchAdapterID, work.RemoteToolBackendWebSearch,
		"", nil,
	)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()

	okProfile := appEnrollmentProfile(
		"profile-enrolled", "openai.primary",
		enrollment.EnrollmentID(), enrollment.Digest(),
	)
	resolved := resolveMissionExecutionEnrollment(view, okProfile)
	if !resolved.available || resolved.blocked ||
		resolved.enrollmentID != enrollment.EnrollmentID() ||
		resolved.enrollmentDigest != enrollment.Digest() ||
		resolved.backendKind != work.RemoteToolBackendWebSearch ||
		len(resolved.bindingDigest) != 64 {
		t.Fatalf("success resolution = %#v", resolved)
	}

	t.Run("no selection", func(t *testing.T) {
		profile := okProfile
		profile.RemoteToolEnrollmentID = ""
		profile.RemoteToolEnrollmentDigest = ""
		resolved := resolveMissionExecutionEnrollment(view, profile)
		if resolved.available || resolved.blocked {
			t.Fatalf("empty selection = %#v", resolved)
		}
	})

	t.Run("incomplete", func(t *testing.T) {
		profile := okProfile
		profile.RemoteToolEnrollmentDigest = ""
		resolved := resolveMissionExecutionEnrollment(view, profile)
		if !resolved.blocked || resolved.blockCode != "tool_enrollment_incomplete" {
			t.Fatalf("incomplete = %#v", resolved)
		}
	})

	t.Run("unavailable", func(t *testing.T) {
		profile := appEnrollmentProfile(
			"profile-missing", "openai.primary",
			"enr-missing", strings.Repeat("a", 64),
		)
		resolved := resolveMissionExecutionEnrollment(view, profile)
		if !resolved.blocked || resolved.blockCode != "tool_enrollment_unavailable" ||
			!resolved.blockRetryable {
			t.Fatalf("unavailable = %#v", resolved)
		}
	})

	t.Run("digest conflict", func(t *testing.T) {
		profile := okProfile
		profile.RemoteToolEnrollmentDigest = strings.Repeat("c", 64)
		resolved := resolveMissionExecutionEnrollment(view, profile)
		if !resolved.blocked || resolved.blockCode != "tool_enrollment_revision_conflict" {
			t.Fatalf("digest conflict = %#v", resolved)
		}
	})

	t.Run("adapter unsupported", func(t *testing.T) {
		unsupported := mustAppEnrollment(
			t, service, authority, now, "openai.primary", policyResult,
			"enr-unsupported", "arbitrary.adapter.v9", work.RemoteToolBackendWebSearch,
			"", nil,
		)
		if err := readModel.Rebuild(context.Background()); err != nil {
			t.Fatal(err)
		}
		view = readModel.GlobalReadView()
		profile := appEnrollmentProfile(
			"profile-unsupported", "openai.primary",
			unsupported.EnrollmentID(), unsupported.Digest(),
		)
		resolved := resolveMissionExecutionEnrollment(view, profile)
		if !resolved.blocked || resolved.blockCode != "tool_enrollment_adapter_unsupported" {
			t.Fatalf("adapter unsupported = %#v", resolved)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		*now = now.Add(time.Minute)
		revoked, err := authority.RevokeRemoteToolBackendEnrollment(
			context.Background(),
			work.RemoteToolBackendEnrollmentRevokeCommand{
				CommandID: "revoke-enr-revoked", EnrollmentID: "enr-search",
				ProviderID: "openai", ProviderAccountID: "openai.primary",
				ExpectedRevision: 1, CorrelationID: "test-revoke",
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := readModel.Rebuild(context.Background()); err != nil {
			t.Fatal(err)
		}
		view = readModel.GlobalReadView()
		profile := appEnrollmentProfile(
			"profile-revoked", "openai.primary",
			revoked.EnrollmentID(), revoked.Digest(),
		)
		resolved := resolveMissionExecutionEnrollment(view, profile)
		if !resolved.blocked || resolved.blockCode != "tool_enrollment_revoked" {
			t.Fatalf("revoked = %#v", resolved)
		}
	})
}

func TestResolveMissionExecutionEnrollmentPolicyDrift(t *testing.T) {
	service, readModel, authority, now := appEnrollmentHarness(t)
	policyResult := mustAppAccountBaseline(
		t, service, authority, now, "openai.primary",
	)
	enrollment := mustAppEnrollment(
		t, service, authority, now, "openai.primary", policyResult,
		"enr-search", work.BuiltInSearchAdapterID, work.RemoteToolBackendWebSearch,
		"", nil,
	)
	*now = now.Add(time.Minute)
	if _, err := service.ConfigureProviderAccountPolicy(
		context.Background(),
		ProviderAccountPolicyCommand{
			ProviderID: "openai", ProviderAccountID: "openai.primary",
			ExpectedRevision: 1, MaximumConcurrentAttempts: 4,
			DispatchWindowSeconds: 60, MaximumDispatchStarts: 20,
			MaximumAssignedBudgetUnits: 10_000,
			TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
			DataRegion: "apac", OperationID: "enrollment-test-policy-drift",
			CorrelationID: "enrollment-test-policy-drift",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	profile := appEnrollmentProfile(
		"profile-drift", "openai.primary",
		enrollment.EnrollmentID(), enrollment.Digest(),
	)
	resolved := resolveMissionExecutionEnrollment(
		readModel.GlobalReadView(), profile,
	)
	if !resolved.blocked || resolved.blockCode != "tool_enrollment_policy_drift" ||
		!resolved.blockRetryable {
		t.Fatalf("policy drift = %#v", resolved)
	}
}

func TestPhase2DBuilderSelectsAgentRemoteToolEnrollment(t *testing.T) {
	service, _, authority, now := appEnrollmentHarness(t)
	policyResult := mustAppAccountBaseline(
		t, service, authority, now, "openai.primary",
	)
	enrollment := mustAppEnrollment(
		t, service, authority, now, "openai.primary", policyResult,
		"enr-search", work.BuiltInSearchAdapterID, work.RemoteToolBackendWebSearch,
		"", nil,
	)
	catalog, _, _, err := service.currentCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil || len(session.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder() = %#v, %v", session, err)
	}
	mainBefore := session.Preview.Roles[0]
	if mainBefore.RemoteToolEnrollmentID != "" {
		t.Fatalf("template main role unexpectedly enrolled: %#v", mainBefore)
	}
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
			Field: "main_remote_tool_enrollment",
			Value: enrollment.EnrollmentID() + ":" + enrollment.Digest(),
		},
	)
	if err != nil || len(edited.Preview.Roles) != 2 {
		t.Fatalf("EditBuilder(enrollment) = %#v, %v", edited, err)
	}
	mainAfter := edited.Preview.Roles[0]
	if mainAfter.RemoteToolEnrollmentID != enrollment.EnrollmentID() ||
		mainAfter.RemoteToolEnrollmentDigest != enrollment.Digest() {
		t.Fatalf("enrollment selection not projected: %#v", mainAfter)
	}
	peer := edited.Preview.Roles[1]
	if peer.RemoteToolEnrollmentID != "" ||
		peer.RemoteToolEnrollmentDigest != "" {
		t.Fatalf("peer role unexpectedly enrolled: %#v", peer)
	}
	cleared, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: edited.ViewVersion,
			Field: "main_remote_tool_enrollment", Value: "none",
		},
	)
	if err != nil || len(cleared.Preview.Roles) != 2 ||
		cleared.Preview.Roles[0].RemoteToolEnrollmentID != "" ||
		cleared.Preview.Roles[0].RemoteToolEnrollmentDigest != "" {
		t.Fatalf("enrollment clear = %#v, %v", cleared, err)
	}
}

func TestPhase2DBuilderRejectsInvalidAgentRemoteToolEnrollment(t *testing.T) {
	service, _, authority, now := appEnrollmentHarness(t)
	policyResult := mustAppAccountBaseline(
		t, service, authority, now, "openai.primary",
	)
	enrollment := mustAppEnrollment(
		t, service, authority, now, "openai.primary", policyResult,
		"enr-search", work.BuiltInSearchAdapterID, work.RemoteToolBackendWebSearch,
		"", nil,
	)
	catalog, _, _, err := service.currentCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		value string
	}{
		{"stale digest", enrollment.EnrollmentID() + ":" + strings.Repeat("a", 64)},
		{"malformed", enrollment.EnrollmentID()},
		{"unknown enrollment", "enr-missing:" + strings.Repeat("b", 64)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.EditBuilder(
				context.Background(),
				BuilderEditCommand{
					DraftID: session.DraftID, ExpectedRevision: session.Revision,
					CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
					Field: "main_remote_tool_enrollment", Value: test.value,
				},
			)
			if err == nil {
				t.Fatal("invalid enrollment accepted")
			}
			if !errors.Is(err, ErrRemoteToolEnrollmentSelectionRejected) &&
				!errors.Is(err, ErrInvalidLocalProductSetup) {
				t.Fatalf("unexpected error = %v", err)
			}
		})
	}
}
