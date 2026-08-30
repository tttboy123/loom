package controltool

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestBuiltinRegistryExposesOnlyGovernedLoomTools(t *testing.T) {
	registry, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry() error = %v", err)
	}
	definitions := registry.Definitions()
	if len(definitions) != 28 {
		t.Fatalf("definition count = %d, want 28", len(definitions))
	}
	want := []struct {
		id           ToolID
		name         string
		version      int
		effect       Effect
		confirmation ConfirmationPolicy
	}{
		{ToolSessionsSearch, "loom_sessions_search", 1, EffectRead, ConfirmationNever},
		{ToolSessionsAlignPreview, "loom_sessions_align_preview", 2, EffectProposal, ConfirmationUser},
		{ToolMissionsCreatePreview, "loom_missions_create_preview", 2, EffectProposal, ConfirmationUser},
		{ToolMissionsContinuePreview, "loom_missions_continue_preview", 2, EffectProposal, ConfirmationUser},
		{ToolTeamsCreatePreview, "loom_teams_create_preview", 2, EffectProposal, ConfirmationUser},
		{ToolRoundtablesOpenPreview, "loom_roundtables_open_preview", 2, EffectProposal, ConfirmationUser},
		{ToolMissionsSearch, "loom_missions_search", 1, EffectRead, ConfirmationNever},
		{ToolMissionsStatus, "loom_missions_status", 1, EffectRead, ConfirmationNever},
		{ToolTeamsSearch, "loom_teams_search", 1, EffectRead, ConfirmationNever},
		{ToolTeamsStatus, "loom_teams_status", 1, EffectRead, ConfirmationNever},
		{ToolRoundtablesStatus, "loom_roundtables_status", 1, EffectRead, ConfirmationNever},
		{ToolGovernanceNeedsYou, "loom_governance_needs_you", 1, EffectRead, ConfirmationNever},
		{ToolRuntimesStatus, "loom_runtimes_status", 1, EffectRead, ConfirmationNever},
		{ToolProvidersStatus, "loom_providers_status", 1, EffectRead, ConfirmationNever},
		{ToolDiagnosticsIncident, "loom_diagnostics_incident", 1, EffectRead, ConfirmationNever},
		{ToolWorkspaceStatus, "loom_workspace_status", 1, EffectRead, ConfirmationNever},
		{ToolConversationRouteStatus, "loom_conversation_route_status", 1, EffectRead, ConfirmationNever},
		{ToolLibrarySearch, "loom_library_search", 1, EffectRead, ConfirmationNever},
		{ToolConversationRouteChangePreview, "loom_conversation_route_change_preview", 2, EffectProposal, ConfirmationUser},
		{ToolConversationModelChangePreview, "loom_conversation_model_change_preview", 2, EffectProposal, ConfirmationUser},
		{ToolConversationReasoningChangePreview, "loom_conversation_reasoning_change_preview", 2, EffectProposal, ConfirmationUser},
		{ToolWorkspaceChoosePreview, "loom_workspace_choose_preview", 2, EffectProposal, ConfirmationUser},
		{ToolTeamsEditPreview, "loom_teams_edit_preview", 2, EffectProposal, ConfirmationUser},
		{ToolRoundtablesPausePreview, "loom_roundtables_pause_preview", 2, EffectProposal, ConfirmationUser},
		{ToolRoundtablesSteerPreview, "loom_roundtables_steer_preview", 2, EffectProposal, ConfirmationUser},
		{ToolRoundtablesRetryPreview, "loom_roundtables_retry_preview", 2, EffectProposal, ConfirmationUser},
		{ToolRoundtablesSkipPreview, "loom_roundtables_skip_preview", 2, EffectProposal, ConfirmationUser},
		{ToolRoundtablesReplacePreview, "loom_roundtables_replace_preview", 2, EffectProposal, ConfirmationUser},
	}
	for index, expected := range want {
		definition := definitions[index]
		if definition.ID != expected.id || definition.MCPName != expected.name ||
			definition.Version != expected.version || definition.Effect != expected.effect ||
			definition.Confirmation != expected.confirmation {
			t.Fatalf("definition[%d] = %#v, want %#v", index, definition, expected)
		}
		var schema map[string]any
		if json.Unmarshal(definition.InputSchema, &schema) != nil ||
			schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("definition[%d] schema is not strict: %s", index, definition.InputSchema)
		}
	}
	digest := registry.Digest()
	if len(digest) != 64 {
		t.Fatalf("registry digest length = %d, want 64", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Fatalf("registry digest is not hex: %v", err)
	}
	second, err := NewBuiltinRegistry()
	if err != nil || second.Digest() != digest {
		t.Fatalf("builtin registry is not deterministic: %v %q", err, second.Digest())
	}
}

func TestBuiltinRegistryInputPropertiesHaveSafeDescriptions(t *testing.T) {
	registry, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry() error = %v", err)
	}
	for _, definition := range registry.Definitions() {
		var schema struct {
			Type                 string                     `json:"type"`
			AdditionalProperties *bool                      `json:"additionalProperties"`
			Properties           map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
			t.Fatalf("%s schema error = %v", definition.ID, err)
		}
		if schema.Type != "object" || schema.AdditionalProperties == nil ||
			*schema.AdditionalProperties || schema.Properties == nil {
			t.Fatalf("%s schema is not a strict object: %s", definition.ID, definition.InputSchema)
		}
		for propertyName, rawProperty := range schema.Properties {
			var property map[string]any
			if err := json.Unmarshal(rawProperty, &property); err != nil {
				t.Fatalf("%s.%s schema error = %v", definition.ID, propertyName, err)
			}
			description, ok := property["description"].(string)
			if !ok || !validText(description, 160) || looksLikeCredential(description) {
				t.Errorf("%s.%s has unsafe description %q", definition.ID, propertyName, description)
			}
		}
	}
}

func TestCompletedCallsBindExactBuiltinDefinitionWithoutContent(t *testing.T) {
	registry, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	calls := []CompletedCall{
		{ToolID: ToolRuntimesStatus, ToolVersion: 1, Effect: EffectRead},
		{ToolID: ToolMissionsCreatePreview, ToolVersion: 2, Effect: EffectProposal},
	}
	if !registry.ValidCompletedCalls(calls, 8) {
		t.Fatalf("valid completed calls rejected: %#v", calls)
	}
	encoded, err := json.Marshal(calls[0])
	if err != nil || string(encoded) !=
		`{"tool_id":"loom.runtimes.status","tool_version":1,"effect":"read"}` {
		t.Fatalf("completed call wire projection = %s, %v", encoded, err)
	}
	for name, mutate := range map[string]func(*CompletedCall){
		"tool":    func(call *CompletedCall) { call.ToolID = ToolID("loom.unknown.status") },
		"version": func(call *CompletedCall) { call.ToolVersion = 2 },
		"effect":  func(call *CompletedCall) { call.Effect = EffectProposal },
	} {
		t.Run(name, func(t *testing.T) {
			changed := calls[0]
			mutate(&changed)
			if registry.ValidCompletedCalls([]CompletedCall{changed}, 8) {
				t.Fatalf("substituted completed call accepted: %#v", changed)
			}
		})
	}
	if registry.ValidCompletedCalls(append(calls, make([]CompletedCall, 7)...), 8) {
		t.Fatal("completed call limit was not enforced")
	}
}

func TestConversationActionProposalBindsExactToolActionArgumentAndTurn(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	proposal, err := NewConversationActionProposal(ConversationActionProposalInput{
		ProposalID:           "proposal-mission-1",
		ToolID:               ToolMissionsCreatePreview,
		TargetConversationID: "conversation-1",
		TargetContentDigest:  digestOf("target"),
		Argument:             "Ship the governed release",
		Route:                phase7FrozenRoute(),
		Workspace:            phase7FrozenWorkspace(),
		RegistryDigest:       digestOf("registry"),
		IncidentID:           "incident-mission-1",
		SegmentID:            "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("NewConversationActionProposal() error = %v", err)
	}
	if proposal.SchemaVersion != 2 || proposal.ToolVersion != 2 ||
		proposal.Action != ConversationActionMission ||
		proposal.ToolID != ToolMissionsCreatePreview ||
		proposal.Status != ProposalPending || !proposal.Valid() {
		t.Fatalf("proposal = %#v", proposal)
	}
	changed := proposal
	changed.Argument = "Run a different release"
	if changed.Valid() {
		t.Fatal("proposal remained valid after argument substitution")
	}
	changed = proposal
	changed.Action = ConversationActionTeam
	if changed.Valid() {
		t.Fatal("proposal remained valid after action substitution")
	}
	changed = proposal
	changed.AttemptID = "attempt-2"
	if changed.Valid() {
		t.Fatal("proposal remained valid after turn substitution")
	}
	changed = proposal.Clone()
	changed.Route.ModelID = "different-model"
	if changed.Valid() {
		t.Fatal("proposal remained valid after frozen Route substitution")
	}
	changed = proposal.Clone()
	changed.Workspace.WorkspaceDigest = digestOf("different-workspace")
	if changed.Valid() {
		t.Fatal("proposal remained valid after frozen Workspace substitution")
	}
}

func TestProposalDecisionReceiptBindsExactProposalTurnAndDecision(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	proposal, err := NewConversationActionProposal(ConversationActionProposalInput{
		ProposalID:           "proposal-mission-receipt-1",
		ToolID:               ToolMissionsCreatePreview,
		TargetConversationID: "conversation-1",
		TargetContentDigest:  digestOf("target"),
		Argument:             "Ship the governed release",
		Route:                phase7FrozenRoute(),
		Workspace:            phase7FrozenWorkspace(),
		RegistryDigest:       digestOf("registry"),
		IncidentID:           "incident-mission-1",
		SegmentID:            "segment-1",
		AttemptID:            "attempt-1",
		CreatedAt:            createdAt,
		ExpiresAt:            createdAt.Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := NewConversationActionDecisionReceipt(
		proposal,
		ProposalDecisionConfirm,
		"incident-confirm-1",
		createdAt.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewConversationActionDecisionReceipt() error = %v", err)
	}
	if !receipt.Valid() || !receipt.MatchesConversationActionProposal(proposal) ||
		receipt.Decision != ProposalDecisionConfirm ||
		receipt.DecisionIncidentID != "incident-confirm-1" ||
		receipt.RegistryDigest != proposal.RegistryDigest ||
		receipt.WorkspaceDigest != proposal.Workspace.WorkspaceDigest ||
		receipt.ExecutionBindingDigest != proposal.Route.ExecutionBindingDigest ||
		receipt.ContextCapsuleDigest != proposal.Route.ContextCapsuleDigest {
		t.Fatalf("decision receipt = %#v", receipt)
	}

	changed := receipt
	changed.Decision = ProposalDecisionCancel
	if changed.Valid() {
		t.Fatal("receipt remained valid after decision substitution")
	}
	changed = receipt
	changed.ProposalDigest = digestOf("different-proposal")
	if changed.Valid() || changed.MatchesConversationActionProposal(proposal) {
		t.Fatal("receipt remained valid after Proposal substitution")
	}
	changed = receipt
	changed.ExecutionBindingDigest = digestOf("different-binding")
	if changed.Valid() {
		t.Fatal("receipt remained valid after execution binding substitution")
	}
	if _, err := NewConversationActionDecisionReceipt(
		proposal,
		ProposalDecisionExpire,
		"incident-expire-too-soon",
		createdAt.Add(time.Minute),
	); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("early expiry receipt error = %v", err)
	}
}

func TestConversationActionProposalRejectsCredentialMaterialAndToolActionMismatch(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	input := ConversationActionProposalInput{
		ProposalID: "proposal-team-1", ToolID: ToolTeamsCreatePreview,
		TargetConversationID: "conversation-1",
		TargetContentDigest:  digestOf("target"),
		Argument:             "Build a team with sk-secret-123456789012345678901234567890",
		Route:                phase7FrozenRoute(),
		Workspace:            phase7FrozenWorkspace(),
		RegistryDigest:       digestOf("registry"),
		IncidentID:           "incident-team-1",
		SegmentID:            "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	}
	if _, err := NewConversationActionProposal(input); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("credential-shaped argument error = %v", err)
	}
	input.Argument = "Review the architecture"
	input.ToolID = ToolSessionsAlignPreview
	if _, err := NewConversationActionProposal(input); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("non-action tool error = %v", err)
	}
	input.ToolID = ToolRoundtablesOpenPreview
	input.Argument = ""
	input.Payload = &ConversationActionPayload{MissionID: "mission-1"}
	proposal, err := NewConversationActionProposal(input)
	if err != nil || proposal.Action != ConversationActionRoundTable || !proposal.Valid() {
		t.Fatalf("roundtable proposal = %#v, error = %v", proposal, err)
	}
}

func TestConversationActionProposalBindsTypedGovernancePayload(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	proposal, err := NewConversationActionProposal(ConversationActionProposalInput{
		ProposalID: "proposal-route-1", ToolID: ToolConversationRouteChangePreview,
		TargetConversationID: "conversation-1",
		TargetContentDigest:  digestOf("target"),
		Payload: &ConversationActionPayload{
			ProfileID: "conversation-deepseek-r4",
		},
		Route: phase7FrozenRoute(), Workspace: phase7FrozenWorkspace(),
		RegistryDigest: digestOf("registry"), IncidentID: "incident-route-1",
		SegmentID: "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	})
	if err != nil || proposal.Action != ConversationActionRoute ||
		proposal.Payload == nil || proposal.Payload.ProfileID != "conversation-deepseek-r4" ||
		!proposal.Valid() {
		t.Fatalf("route proposal = %#v, error = %v", proposal, err)
	}
	changed := proposal
	changed.Payload = cloneConversationActionPayload(proposal.Payload)
	changed.Payload.ProfileID = "conversation-other-r1"
	if changed.Valid() {
		t.Fatal("proposal remained valid after Route payload substitution")
	}

	_, err = NewConversationActionProposal(ConversationActionProposalInput{
		ProposalID: "proposal-steer-1", ToolID: ToolRoundtablesSteerPreview,
		TargetConversationID: "conversation-1",
		TargetContentDigest:  digestOf("target"),
		Payload: &ConversationActionPayload{
			SessionID: "roundtable-1", RoundID: "round-1", SeatID: "seat-reviewer",
			AttemptID: "attempt-reviewer-1",
			Guidance:  "Use sk-secret-123456789012345678901234567890",
		},
		Route: phase7FrozenRoute(), Workspace: phase7FrozenWorkspace(),
		RegistryDigest: digestOf("registry"), IncidentID: "incident-steer-1",
		SegmentID: "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	})
	if !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("credential-shaped intervention error = %v", err)
	}
}

func TestConversationRoundTableSeatProposalBindsMembershipAndSeatBinding(t *testing.T) {
	createdAt := time.Date(2026, 8, 29, 15, 30, 0, 0, time.UTC)
	proposal, err := NewConversationActionProposal(ConversationActionProposalInput{
		ProposalID: "proposal-skip-bound-1", ToolID: ToolRoundtablesSkipPreview,
		TargetConversationID: "conversation-1", TargetContentDigest: digestOf("target"),
		Payload: &ConversationActionPayload{
			SessionID: "roundtable-1", RoundID: "round-1", SeatID: "seat-coder",
			MembershipRevision: 7, SeatBindingDigest: digestOf("seat-binding-7"),
		},
		Route: phase7FrozenRoute(), Workspace: phase7FrozenWorkspace(),
		RegistryDigest: digestOf("registry"), IncidentID: "incident-skip-bound-1",
		SegmentID: "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	})
	if err != nil || !proposal.Valid() || proposal.Payload == nil ||
		!proposal.Payload.ValidFrozenRoundTableSeatBinding() || !proposal.Confirmable() {
		t.Fatalf("bound skip proposal = %#v, error = %v", proposal, err)
	}
	changed := proposal.Clone()
	changed.Payload.MembershipRevision++
	if changed.Valid() {
		t.Fatal("Proposal remained valid after membership revision substitution")
	}
	changed = proposal.Clone()
	changed.Payload.SeatBindingDigest = digestOf("other-seat-binding")
	if changed.Valid() {
		t.Fatal("Proposal remained valid after seat binding substitution")
	}
	legacy := proposal.Clone()
	legacy.Payload.MembershipRevision = 0
	legacy.Payload.SeatBindingDigest = ""
	legacy.ProposalDigest = legacy.payloadDigest()
	if !legacy.Valid() || legacy.Payload.ValidFrozenRoundTableSeatBinding() {
		t.Fatal("legacy v2 RoundTable proposal is not readable as an unbound migration record")
	}
	if legacy.Confirmable() {
		t.Fatal("legacy unbound v2 RoundTable proposal remained confirmable")
	}
	if _, err := NewConversationActionDecisionReceipt(
		legacy, ProposalDecisionConfirm, "incident-confirm-legacy", createdAt.Add(time.Minute),
	); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("legacy unbound confirmation receipt error = %v", err)
	}
	if _, err := NewConversationActionDecisionReceipt(
		legacy, ProposalDecisionCancel, "incident-cancel-legacy", createdAt.Add(time.Minute),
	); err != nil {
		t.Fatalf("legacy unbound cancellation receipt error = %v", err)
	}
	localTarget := proposal.Clone()
	localTarget.Payload.SessionID = "session+本地"
	localTarget.Payload.RoundID = "round#评审"
	localTarget.Payload.SeatID = "seat+c++/评审"
	localTarget.ProposalDigest = localTarget.payloadDigest()
	if !localTarget.Valid() {
		t.Fatalf("authority-valid local RoundTable identifiers were rejected: %#v", localTarget.Payload)
	}
	secretShaped := proposal.Clone()
	secretShaped.Payload.SeatID = "sk-private-seat"
	secretShaped.ProposalDigest = secretShaped.payloadDigest()
	if secretShaped.Valid() {
		t.Fatal("credential-shaped RoundTable identifier was accepted")
	}
}

func TestConversationRoundTableProposalAcceptsOpaqueAuthorityIdentifiers(t *testing.T) {
	createdAt := time.Date(2026, 8, 29, 16, 0, 0, 0, time.UTC)
	input := ConversationActionProposalInput{
		ProposalID:           "proposal-real-authority-id-1",
		ToolID:               ToolRoundtablesSteerPreview,
		TargetConversationID: "conversation-1",
		TargetContentDigest:  digestOf("target"),
		Payload: &ConversationActionPayload{
			SessionID: "rt-20260827-0953", RoundID: "round-3",
			SeatID:    "agent-subagent-loom-bounded-worker-loom-minimax-subagent-r23",
			AttemptID: "attempt-04255270-c624-4b10-a85a-9576449bee27",
			Guidance:  "Continue the bounded review.", MembershipRevision: 7,
			SeatBindingDigest: digestOf("seat-binding-7"),
		},
		Route: phase7FrozenRoute(), Workspace: phase7FrozenWorkspace(),
		RegistryDigest: digestOf("registry"), IncidentID: "incident-real-id-1",
		SegmentID: "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	}
	proposal, err := NewConversationActionProposal(input)
	if err != nil || !proposal.Valid() || !proposal.Confirmable() {
		t.Fatalf("opaque authority identifiers were rejected: %#v, %v", proposal, err)
	}
	input.Payload = cloneConversationActionPayload(input.Payload)
	input.Payload.SeatID = "sk-secret-authority-identifier"
	if _, err := NewConversationActionProposal(input); !errors.Is(err, ErrInvalidCall) {
		t.Fatalf("explicit credential marker in authority identifier error = %v", err)
	}
}

func TestConversationActionProposalV2BindsExactMissionTargetsAndLegacyV1RemainsReadable(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 12, 30, 0, 0, time.UTC)
	proposal, err := NewConversationActionProposal(ConversationActionProposalInput{
		ProposalID: "proposal-continue-1", ToolID: ToolMissionsContinuePreview,
		TargetConversationID: "conversation-1", TargetContentDigest: digestOf("target"),
		Argument: "Retry only the failed verification step",
		Payload:  &ConversationActionPayload{MissionID: "mission-release-7"},
		Route:    phase7FrozenRoute(), Workspace: phase7FrozenWorkspace(),
		RegistryDigest: digestOf("registry"), IncidentID: "incident-continue-1",
		SegmentID: "segment-1", AttemptID: "attempt-1",
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	})
	if err != nil || !proposal.Valid() || proposal.Payload == nil ||
		proposal.Payload.MissionID != "mission-release-7" {
		t.Fatalf("continue proposal = %#v, error = %v", proposal, err)
	}
	changed := proposal.Clone()
	changed.Payload.MissionID = "mission-other"
	if changed.Valid() {
		t.Fatal("proposal remained valid after Mission target substitution")
	}

	legacy := ConversationActionProposal{
		SchemaVersion: 1, ProposalID: "proposal-legacy-1",
		ToolID: ToolMissionsCreatePreview, ToolVersion: 1,
		Confirmation: ConfirmationUser, Action: ConversationActionMission,
		Argument: "Read a legacy Mission proposal", TargetConversationID: "conversation-1",
		TargetContentDigest: digestOf("target"), SegmentID: "segment-1",
		AttemptID: "attempt-1", Status: ProposalConfirmed,
		CreatedAt: createdAt, ExpiresAt: createdAt.Add(5 * time.Minute),
	}
	legacy.ProposalDigest = legacy.payloadDigest()
	if !legacy.Valid() || legacy.Route != nil || legacy.Workspace != nil {
		t.Fatalf("legacy proposal is no longer readable: %#v", legacy)
	}
}

func phase7FrozenRoute() *FrozenRouteReference {
	return &FrozenRouteReference{
		HarnessAdapter: "codex", ProviderID: "openai",
		ProviderAccountID: "openai.primary", CredentialRevision: 4,
		ModelID: "gpt-5.6-sol", ReasoningEffort: "max",
		ExecutionBindingDigest: digestOf("binding"),
		ContextCapsuleDigest:   digestOf("capsule"),
	}
}

func phase7FrozenWorkspace() *FrozenWorkspaceReference {
	return &FrozenWorkspaceReference{
		WorkspaceID: "workspace-primary", WorkspaceDigest: digestOf("workspace"),
	}
}

func TestRegistryRejectsDuplicateIdentityAndModelName(t *testing.T) {
	builtin, err := NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definition := builtin.Definitions()[0]
	if _, err := NewRegistry([]Definition{definition, definition}); !errors.Is(err, ErrInvalidRegistry) {
		t.Fatalf("duplicate identity error = %v", err)
	}
	second := definition
	second.ID = "loom.sessions.other"
	if _, err := NewRegistry([]Definition{definition, second}); !errors.Is(err, ErrInvalidRegistry) {
		t.Fatalf("duplicate MCP name error = %v", err)
	}
}

func TestSessionAlignmentProposalDigestBindsFrozenSourcesAndExpiry(t *testing.T) {
	expires := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	proposal, err := NewSessionAlignmentProposal(SessionAlignmentProposalInput{
		ProposalID: "proposal-1", TargetConversationID: "s3",
		TargetContentDigest: digestOf("target"), ContextMode: ContextModeSummaryOnly,
		Sources: []SessionSource{
			{ConversationID: "s2", Title: "Second", ContentDigest: digestOf("second"), MessageCount: 4},
			{ConversationID: "s1", Title: "First", ContentDigest: digestOf("first"), MessageCount: 2},
		},
		CatalogDigest: digestOf("catalog"), CreatedAt: expires.Add(-5 * time.Minute),
		ExpiresAt: expires, AttemptID: "attempt-2", SegmentID: "segment-1",
		Route: phase7FrozenRoute(), Workspace: phase7FrozenWorkspace(),
		RegistryDigest: digestOf("registry"), IncidentID: "incident-align-1",
	})
	if err != nil {
		t.Fatalf("NewSessionAlignmentProposal() error = %v", err)
	}
	if proposal.Sources[0].ConversationID != "s1" ||
		proposal.Sources[1].ConversationID != "s2" {
		t.Fatalf("sources are not canonical: %#v", proposal.Sources)
	}
	if proposal.Status != ProposalPending || proposal.Confirmation != ConfirmationUser ||
		proposal.SchemaVersion != 2 || proposal.ToolID != ToolSessionsAlignPreview ||
		proposal.ToolVersion != 2 || len(proposal.ProposalDigest) != 64 ||
		!proposal.Valid() || !proposal.Confirmable() {
		t.Fatalf("proposal = %#v", proposal)
	}
	changed := proposal
	changed.Sources = append([]SessionSource(nil), proposal.Sources...)
	changed.Sources[0].ContentDigest = digestOf("drifted")
	if changed.Valid() {
		t.Fatal("proposal remained valid after source digest substitution")
	}
	changed = proposal
	changed.ExpiresAt = changed.ExpiresAt.Add(time.Second)
	if changed.Valid() {
		t.Fatal("proposal remained valid after expiry substitution")
	}
	changed = proposal.Clone()
	changed.Route.ContextCapsuleDigest = digestOf("different-capsule")
	if changed.Valid() {
		t.Fatal("alignment remained valid after frozen Context Capsule substitution")
	}
	changed = proposal.Clone()
	changed.RegistryDigest = digestOf("different-registry")
	if changed.Valid() {
		t.Fatal("alignment remained valid after Tool Registry substitution")
	}
}

func TestLegacySessionAlignmentProposalV1RemainsReadable(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	legacy := SessionAlignmentProposal{
		SchemaVersion: 1, ProposalID: "proposal-align-legacy-1",
		ToolID: ToolSessionsAlignPreview, ToolVersion: 1,
		Confirmation: ConfirmationUser, TargetConversationID: "s3",
		TargetContentDigest: digestOf("target"),
		Sources: []SessionSource{{
			ConversationID: "s1", Title: "First",
			ContentDigest: digestOf("first"), MessageCount: 2,
		}},
		ContextMode: ContextModeSummaryOnly, CatalogDigest: digestOf("catalog"),
		SegmentID: "segment-1", AttemptID: "attempt-1",
		Status: ProposalPending, CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(5 * time.Minute),
	}
	legacy.ProposalDigest = legacy.payloadDigest()
	if !legacy.Valid() || legacy.Route != nil || legacy.Workspace != nil {
		t.Fatalf("legacy alignment is no longer readable: %#v", legacy)
	}
	if legacy.Confirmable() {
		t.Fatal("legacy alignment remained confirmable")
	}
}

func TestGatewaySlotBindsOnceAndFailsClosed(t *testing.T) {
	slot := NewGatewaySlot()
	turn := TurnContext{ConversationID: "s3", SegmentID: "segment-1", AttemptID: "attempt-1"}
	if _, err := slot.Call(context.Background(), turn, Call{ToolID: ToolSessionsSearch}); !errors.Is(err, ErrGatewayUnavailable) {
		t.Fatalf("unbound Call() error = %v", err)
	}
	gateway := &recordingGateway{}
	if err := slot.Bind(gateway); err != nil {
		t.Fatalf("Bind() error = %v", err)
	}
	if err := slot.Bind(&recordingGateway{}); !errors.Is(err, ErrGatewayConflict) {
		t.Fatalf("second Bind() error = %v", err)
	}
	if _, err := slot.Call(context.Background(), turn, Call{ToolID: ToolSessionsSearch}); err != nil {
		t.Fatalf("bound Call() error = %v", err)
	}
	if gateway.calls != 1 {
		t.Fatalf("gateway calls = %d, want 1", gateway.calls)
	}
}

type recordingGateway struct{ calls int }

func (gateway *recordingGateway) Call(
	context.Context,
	TurnContext,
	Call,
) (Result, error) {
	gateway.calls++
	return Result{Content: json.RawMessage(`{"sessions":[]}`)}, nil
}

func digestOf(value string) string {
	return DigestBytes([]byte(value))
}
