package work

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func mustAgentEnrollment(t *testing.T, input RemoteToolBackendEnrollmentInput) RemoteToolBackendEnrollment {
	t.Helper()
	enrollment, err := NewRemoteToolBackendEnrollment(input)
	if err != nil {
		t.Fatalf("NewRemoteToolBackendEnrollment(%#v) error = %v", input, err)
	}
	return enrollment
}

func baseAgentEnrollmentInput() RemoteToolBackendEnrollmentInput {
	return RemoteToolBackendEnrollmentInput{
		Version:                       1,
		EnrollmentID:                  "enr.search.alpha",
		BackendKind:                   RemoteToolBackendWebSearch,
		AdapterID:                     BuiltInSearchAdapterID,
		ProviderID:                    "deepseek",
		ProviderAccountID:             "deepseek.primary",
		ProviderAccountPolicyVersion:  1,
		ProviderAccountPolicyRevision: 3,
		ProviderAccountPolicyDigest:   strings.Repeat("a", 64),
		EndpointFingerprint:           strings.Repeat("b", 64),
		AllowedTools:                  nil,
		Revision:                      1,
		Status:                        RemoteToolBackendEnrollmentActive,
		MaximumConcurrentCalls:        2,
		MaximumCallsPerAttempt:        4,
		Timeout:                       30 * time.Second,
		MaximumResultBytes:            1 << 16,
		MaximumBudgetUnits:            1000,
		ConfiguredAt:                  time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
	}
}

func TestFreezeAgentRemoteToolEnrollmentBaseline(t *testing.T) {
	enrollment := mustAgentEnrollment(t, baseAgentEnrollmentInput())
	selection := AgentRemoteToolEnrollmentSelection{
		EnrollmentID:      enrollment.EnrollmentID(),
		ProviderID:        enrollment.ProviderID(),
		ProviderAccountID: enrollment.ProviderAccountID(),
		ExpectedDigest:    enrollment.Digest(),
	}
	frozen, err := FreezeAgentRemoteToolEnrollment(
		selection, enrollment, true, BuiltInRemoteToolBackendCatalog(),
	)
	if err != nil {
		t.Fatalf("FreezeAgentRemoteToolEnrollment error = %v", err)
	}
	if frozen.EnrollmentID != enrollment.EnrollmentID() ||
		frozen.EnrollmentDigest != enrollment.Digest() ||
		frozen.BackendKind != RemoteToolBackendWebSearch ||
		frozen.AdapterID != BuiltInSearchAdapterID ||
		frozen.ProviderID != "deepseek" ||
		frozen.ProviderAccountID != "deepseek.primary" ||
		frozen.PolicyVersion != 1 ||
		frozen.PolicyRevision != 3 ||
		frozen.PolicyDigest != strings.Repeat("a", 64) ||
		frozen.MaximumCallsPerAttempt != 4 ||
		frozen.Timeout != 30*time.Second ||
		frozen.MaximumResultBytes != 1<<16 {
		t.Fatalf("frozen binding fields mismatch: %#v", frozen)
	}
	if len(frozen.BindingDigest) != 64 {
		t.Fatalf("binding digest not sha256 hex: %q", frozen.BindingDigest)
	}
	recomputed, err := freezeAgentRemoteToolEnrollmentBinding(frozen)
	if err != nil || recomputed.BindingDigest != frozen.BindingDigest {
		t.Fatalf("binding digest not deterministic: %#v error=%v", recomputed, err)
	}
}

func TestFreezeAgentRemoteToolEnrollmentMCPAllowlistDigest(t *testing.T) {
	input := baseAgentEnrollmentInput()
	input.BackendKind = RemoteToolBackendMCPServer
	input.AdapterID = BuiltInMCPAdapterID
	input.MCPServerID = "issues"
	input.AllowedTools = []string{"get_issue", "list_issues"}
	enrollment := mustAgentEnrollment(t, input)
	selection := AgentRemoteToolEnrollmentSelection{
		EnrollmentID: enrollment.EnrollmentID(), ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", ExpectedDigest: enrollment.Digest(),
	}
	frozen, err := FreezeAgentRemoteToolEnrollment(
		selection, enrollment, true, BuiltInRemoteToolBackendCatalog(),
	)
	if err != nil {
		t.Fatalf("FreezeAgentRemoteToolEnrollment error = %v", err)
	}
	if frozen.AllowedToolsDigest == "" ||
		len(frozen.AllowedToolsDigest) != 64 {
		t.Fatalf("MCP allowlist digest missing: %q", frozen.AllowedToolsDigest)
	}
	reordered := mustAgentEnrollment(t, input)
	// AllowedTools are stored sorted by the authority; a different order is
	// impossible through the authority, but the digest must still be canonical.
	frozenReordered, err := FreezeAgentRemoteToolEnrollment(
		selection, reordered, true, BuiltInRemoteToolBackendCatalog(),
	)
	if err != nil {
		t.Fatalf("reordered error = %v", err)
	}
	if frozenReordered.AllowedToolsDigest != frozen.AllowedToolsDigest {
		t.Fatalf("allowlist digest not canonical: %q vs %q",
			frozen.AllowedToolsDigest, frozenReordered.AllowedToolsDigest)
	}
}

func TestFreezeAgentRemoteToolEnrollmentFailsClosed(t *testing.T) {
	enrollment := mustAgentEnrollment(t, baseAgentEnrollmentInput())
	selection := AgentRemoteToolEnrollmentSelection{
		EnrollmentID:      enrollment.EnrollmentID(),
		ProviderID:        enrollment.ProviderID(),
		ProviderAccountID: enrollment.ProviderAccountID(),
		ExpectedDigest:    enrollment.Digest(),
	}
	catalog := BuiltInRemoteToolBackendCatalog()

	t.Run("revoked", func(t *testing.T) {
		revoked := mustAgentEnrollment(t, func() RemoteToolBackendEnrollmentInput {
			input := baseAgentEnrollmentInput()
			input.Revision = 2
			input.Status = RemoteToolBackendEnrollmentRevoked
			return input
		}())
		if _, err := FreezeAgentRemoteToolEnrollment(
			selection, revoked, true, catalog,
		); !errors.Is(err, ErrAgentRemoteToolEnrollmentRevoked) {
			t.Fatalf("revoked error = %v", err)
		}
	})

	t.Run("policy drift", func(t *testing.T) {
		if _, err := FreezeAgentRemoteToolEnrollment(
			selection, enrollment, false, catalog,
		); !errors.Is(err, ErrAgentRemoteToolBackendPolicyDrift) {
			t.Fatalf("drift error = %v", err)
		}
	})

	t.Run("account mismatch", func(t *testing.T) {
		mismatch := selection
		mismatch.ProviderAccountID = "deepseek.secondary"
		if _, err := FreezeAgentRemoteToolEnrollment(
			mismatch, enrollment, true, catalog,
		); !errors.Is(err, ErrAgentRemoteToolEnrollmentAccountMismatch) {
			t.Fatalf("account mismatch error = %v", err)
		}
	})

	t.Run("digest conflict", func(t *testing.T) {
		conflict := selection
		conflict.ExpectedDigest = strings.Repeat("c", 64)
		if _, err := FreezeAgentRemoteToolEnrollment(
			conflict, enrollment, true, catalog,
		); !errors.Is(err, ErrAgentRemoteToolEnrollmentDigestConflict) {
			t.Fatalf("digest conflict error = %v", err)
		}
	})

	t.Run("unsupported adapter", func(t *testing.T) {
		unsupported := mustAgentEnrollment(t, func() RemoteToolBackendEnrollmentInput {
			input := baseAgentEnrollmentInput()
			input.AdapterID = "arbitrary.adapter.v9"
			return input
		}())
		unsupportedSelection := selection
		unsupportedSelection.ExpectedDigest = unsupported.Digest()
		if _, err := FreezeAgentRemoteToolEnrollment(
			unsupportedSelection, unsupported, true, catalog,
		); !errors.Is(err, ErrAgentRemoteToolEnrollmentAdapterUnsupported) {
			t.Fatalf("unsupported adapter error = %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		missing := selection
		missing.EnrollmentID = "enr.search.missing"
		if _, err := FreezeAgentRemoteToolEnrollment(
			missing, enrollment, true, catalog,
		); !errors.Is(err, ErrAgentRemoteToolEnrollmentNotFound) {
			t.Fatalf("not found error = %v", err)
		}
	})

	t.Run("invalid selection", func(t *testing.T) {
		invalid := selection
		invalid.ExpectedDigest = "not-a-digest"
		if _, err := FreezeAgentRemoteToolEnrollment(
			invalid, enrollment, true, catalog,
		); !errors.Is(err, ErrInvalidAgentRemoteToolEnrollment) {
			t.Fatalf("invalid selection error = %v", err)
		}
	})

	t.Run("invalid enrollment", func(t *testing.T) {
		if _, err := FreezeAgentRemoteToolEnrollment(
			selection, RemoteToolBackendEnrollment{}, true, catalog,
		); !errors.Is(err, ErrInvalidAgentRemoteToolEnrollment) {
			t.Fatalf("invalid enrollment error = %v", err)
		}
	})
}

func TestBuiltInRemoteToolBackendCatalog(t *testing.T) {
	catalog := BuiltInRemoteToolBackendCatalog()
	if catalog == nil {
		t.Fatal("nil built-in catalog")
	}
	if !catalog.RemoteToolBackendAdapterSupported(BuiltInSearchAdapterID) ||
		!catalog.RemoteToolBackendAdapterSupported(BuiltInMCPAdapterID) {
		t.Fatal("built-in adapters not supported")
	}
	if catalog.RemoteToolBackendAdapterSupported("arbitrary.adapter.v9") {
		t.Fatal("arbitrary adapter reported supported")
	}
	descriptors := catalog.RemoteToolBackendDescriptors()
	if len(descriptors) != 2 {
		t.Fatalf("descriptors = %#v", descriptors)
	}
	seen := map[string]bool{}
	for _, descriptor := range descriptors {
		if seen[descriptor.AdapterID] {
			t.Fatalf("duplicate adapter %q", descriptor.AdapterID)
		}
		seen[descriptor.AdapterID] = true
		if descriptor.BackendKind == "" {
			t.Fatalf("empty backend kind for %q", descriptor.AdapterID)
		}
	}
}
