package harnessgateway

import (
	"strings"
	"testing"
)

func TestSegmentSessionBindingFreezesReasoningEffort(t *testing.T) {
	binding := SegmentSessionBinding{
		SchemaVersion:       SegmentSessionBindingSchemaVersion,
		ConfiguredHarnessID: HarnessCodex, ConfiguredHarnessVersion: 1,
		BackendID: "backend.codex.app-server", BackendVersion: 1,
		ConversationID: "conversation-1", SegmentID: "segment-1",
		WorkspaceID: "workspace-1", WorkspaceDigest: strings.Repeat("1", 64),
		ExecutionBindingDigest: strings.Repeat("2", 64),
		ProviderID:             "openai", ProviderAccountID: "openai.native", CredentialRevision: 1,
		ModelID: "gpt-5.6-sol", ReasoningEffort: "high",
		SegmentContextCapsuleDigest: strings.Repeat("3", 64),
		GovernancePolicyDigest:      strings.Repeat("4", 64),
	}
	if !binding.valid() || binding.SessionID() == "" {
		t.Fatalf("valid Codex binding rejected: %#v", binding)
	}
	different := binding
	different.ReasoningEffort = "medium"
	if different.SessionID() == binding.SessionID() {
		t.Fatal("reasoning effort did not change the immutable Session identity")
	}
	authority := ResponseAuthority{
		SchemaVersion: ResponseAuthoritySchemaVersion,
		ResponseID:    "attempt-1", IncidentID: "incident-1",
		ExecutionBindingDigest:      binding.ExecutionBindingDigest,
		ContextCapsuleDigest:        binding.SegmentContextCapsuleDigest,
		SegmentContextCapsuleDigest: binding.SegmentContextCapsuleDigest,
		GovernancePolicyDigest:      binding.GovernancePolicyDigest,
		ProviderID:                  binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		CredentialRevision: binding.CredentialRevision, ModelID: binding.ModelID,
		ReasoningEffort: "medium",
	}
	if authority.validFor(binding) {
		t.Fatal("response authority with substituted reasoning effort was accepted")
	}
	authority.ReasoningEffort = binding.ReasoningEffort
	if !authority.validFor(binding) {
		t.Fatal("exact response authority was rejected")
	}
}

func TestNativeSegmentSessionBindingPreservesAbsentCredentialAuthority(t *testing.T) {
	binding := SegmentSessionBinding{
		SchemaVersion:       SegmentSessionBindingSchemaVersion,
		ConfiguredHarnessID: HarnessCodex, ConfiguredHarnessVersion: 1,
		BackendID: "backend.codex.app-server", BackendVersion: 1,
		ConversationID: "conversation-native", SegmentID: "segment-native",
		WorkspaceID: "workspace-native", WorkspaceDigest: strings.Repeat("1", 64),
		ExecutionBindingDigest: strings.Repeat("2", 64),
		ProviderID:             "openai",
		ModelID:                "gpt-5.6-sol", ReasoningEffort: "high",
		SegmentContextCapsuleDigest: strings.Repeat("3", 64),
	}
	if !binding.valid() || binding.SessionID() == "" {
		t.Fatalf("native binding rejected: %#v", binding)
	}
	authority := ResponseAuthority{
		SchemaVersion: ResponseAuthoritySchemaVersion,
		ResponseID:    "attempt-native", IncidentID: "incident-native",
		ExecutionBindingDigest:      binding.ExecutionBindingDigest,
		ContextCapsuleDigest:        binding.SegmentContextCapsuleDigest,
		SegmentContextCapsuleDigest: binding.SegmentContextCapsuleDigest,
		ProviderID:                  binding.ProviderID,
		ModelID:                     binding.ModelID,
		ReasoningEffort:             binding.ReasoningEffort,
	}
	if !authority.validFor(binding) {
		t.Fatal("exact native response authority was rejected")
	}
	synthetic := binding
	synthetic.ProviderAccountID = "openai.native"
	synthetic.CredentialRevision = 1
	if synthetic.SessionID() == binding.SessionID() {
		t.Fatal("synthetic credential authority reused the native Session identity")
	}
}

func TestSegmentSessionBindingFreezesRouteTransitionReview(t *testing.T) {
	binding := segmentBindingFixture(HarnessLoomNative, "conversation-reviewed", "segment-reviewed", "workspace-reviewed")
	binding.RouteTransitionReviewDigest = strings.Repeat("7", 64)
	if !binding.valid() || binding.SessionID() == "" {
		t.Fatal("expected reviewed route transition to produce a valid session binding")
	}
	authority := responseFixture(binding, "response-reviewed").Authority
	authority.RouteTransitionReviewDigest = binding.RouteTransitionReviewDigest
	if !authority.validFor(binding) {
		t.Fatal("expected response authority to freeze the reviewed route transition")
	}
	authority.RouteTransitionReviewDigest = strings.Repeat("8", 64)
	if authority.validFor(binding) {
		t.Fatal("expected route transition review substitution to fail closed")
	}

	invalid := binding
	invalid.RouteTransitionReviewDigest = "not-a-digest"
	if invalid.valid() || invalid.SessionID() != "" {
		t.Fatal("expected invalid route transition review digest to fail closed")
	}
}
