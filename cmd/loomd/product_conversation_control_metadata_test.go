package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/roundtable"
)

type productConversationControlReadFixture struct {
	snapshot api.LocalProductSnapshot
	err      error
}

func (fixture productConversationControlReadFixture) ReadLocalProductSnapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return fixture.snapshot, fixture.err
}

type productConversationControlSetupFixture struct {
	snapshot app.SetupSnapshot
	err      error
}

func (fixture productConversationControlSetupFixture) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return fixture.snapshot, fixture.err
}

type productConversationControlRoundtableFixture struct {
	view roundtable.View
	err  error
}

func (fixture productConversationControlRoundtableFixture) ReadView(
	context.Context,
	string,
) (roundtable.View, error) {
	return fixture.view, fixture.err
}

type productConversationControlIncidentFixture struct {
	events []productIncidentDiagnosticEvent
	err    error
}

func (fixture productConversationControlIncidentFixture) IncidentDiagnostics(
	context.Context,
	string,
) ([]productIncidentDiagnosticEvent, error) {
	return append([]productIncidentDiagnosticEvent(nil), fixture.events...), fixture.err
}

func TestProductConversationControlMetadataPublishesBoundedSafeProjection(t *testing.T) {
	digest := strings.Repeat("a", 64)
	read := productConversationControlReadFixture{snapshot: api.LocalProductSnapshot{
		SchemaVersion: 3,
		ViewVersion:   "view-7",
		Missions: []api.LocalProductMissionSummary{{
			SchemaVersion:      1,
			MissionID:          "mission/team-alpha",
			TeamInstanceID:     "team-alpha",
			Title:              "Alpha delivery",
			Lane:               api.MissionLaneOrchestrating,
			Status:             "blocked",
			NodeCount:          3,
			CompletedNodeCount: 1,
			ActiveNodeCount:    1,
			AttentionCount:     1,
			CurrentNodeID:      "node-build",
			LastMilestone:      "Implementation started",
			BlockReason:        "Provider account requires attention",
		}},
		Teams: []api.LocalProductTeamSummary{{
			TeamInstanceID: "team-alpha", TeamDefinitionID: "team-def-alpha",
			TeamDefinitionVersion: 3, DisplayName: "Alpha Team", State: "active",
			Confirmed: true, Executable: true,
			Agents: []api.LocalProductTeamAgentSummary{{
				RoleKind: "main", AgentDefinitionID: "agent-planner",
				RuntimeProfileID: "profile-planner", BindingStatus: "ready",
				HarnessAdapter: "codex", ProviderID: "openai",
				ProviderAccountID: "openai.primary", ModelID: "gpt-5.6-sol",
				CredentialRevision: 7,
			}},
		}},
		Runtimes: []api.LocalProductRuntimeSummary{{
			RuntimeInstanceID: "runtime-codex", DisplayName: "Codex",
			AdapterType: "codex", ExecutableVersion: "1.2.3", Status: "online",
			Capacity: 2, ModelIDs: []string{"gpt-5.6-sol"},
			ObservedCapabilities: []string{"conversation", "control_tools"},
		}},
		Evidence: []api.LocalProductEvidenceSummary{{
			EvidenceID: "evidence-alpha", WorkItemID: "work-alpha", Digest: digest,
		}},
		Attention: []api.AttentionItem{{
			SchemaVersion: 1, AttentionID: "attention-alpha", Kind: "provider_account",
			Severity: "blocking", TeamInstanceID: "team-alpha", Status: "open",
			ActionRequired: "Review the Provider account",
		}},
	}}
	setup := productConversationControlSetupFixture{snapshot: app.SetupSnapshot{
		SchemaVersion: 1, ViewVersion: "setup-9",
		Providers: []app.ProviderDirectoryEntry{{
			ProviderID: "deepseek", DisplayName: "DeepSeek", Protocol: "openai_compatible",
			CredentialReference: "vault/provider-secret", Revision: 4, Status: "verified",
		}},
		ProviderAccounts: []app.ProviderAccountDirectoryEntry{{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			CredentialReference: "vault/account-secret", Revision: 4, Status: "verified",
			PolicyAvailable: true, PolicyVersion: 1, PolicyRevision: 3,
			PolicyDigest: digest, MaximumConcurrentAttempts: 2,
			MaximumAssignedBudgetUnits: 50, TrustDomain: "external",
			RetentionMode: "provider_default", DataRegion: "global",
		}},
		ConversationProfiles: []app.ConversationProviderProfile{{
			ProfileID: "conversation-deepseek-r4", HarnessAdapter: "loom-native",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			DisplayName: "DeepSeek Chat", ModelID: "deepseek-chat",
			CredentialRevision: 4, PolicyVersion: 1, PolicyRevision: 3,
			PolicyDigest: digest, TrustDomain: "external",
			RetentionMode: "provider_default", DataRegion: "global",
		}},
	}}
	roundtableSource := productConversationControlRoundtableFixture{view: roundtable.View{
		Session: roundtable.Session{
			ID: "roundtable-alpha", ModeratorSeat: "moderator", Title: "Alpha review",
			Context: &roundtable.SessionContext{
				ConversationID: "conversation-alpha", MissionID: "mission/team-alpha",
				TeamID: "team-alpha", TeamVersion: 3, WorkspaceID: "workspace-alpha",
			},
		},
		Seats: map[string]roundtable.Seat{
			"planner": {
				ID: "planner", DisplayName: "Planner", Available: true,
				Binding: &roundtable.FrozenSeatBinding{
					AgentDefinitionID: "agent-planner", TeamRoleKind: "main",
					RuntimeProfileID: "profile-planner", MembershipRevision: 2,
					BindingDigest: digest,
					ExecutionBinding: roundtable.SeatExecutionBinding{
						HarnessAdapter: "codex", ProviderID: "openai",
						ProviderAccountID: "openai.primary", ModelID: "gpt-5.6-sol",
						CredentialReference: "vault/roundtable-secret",
						CredentialRevision:  7, EndpointFingerprint: "private-endpoint",
						ReasoningEffort: "max", BindingDigest: digest,
					},
				},
			},
		},
		Rounds: []roundtable.Round{{ID: "round-1", Sequence: 1, MessageCount: 1}},
		Messages: map[string]roundtable.Message{
			"message-private": {ID: "message-private", Body: "PRIVATE_DELIBERATION_BODY"},
		},
		Attempts: map[string]roundtable.SeatAttempt{
			"attempt-planner": {
				AttemptID: "attempt-planner", SeatID: "planner", RoundID: "round-1",
				Status: roundtable.SeatAttemptRunning, IncidentID: "incident-alpha",
			},
		},
		Deliveries: map[string]roundtable.SeatDelivery{
			"planner": {AttemptID: "attempt-planner", SeatID: "planner", Body: "PRIVATE_AGENT_OUTPUT"},
		},
		Digest: digest,
	}}
	incidents := productConversationControlIncidentFixture{events: []productIncidentDiagnosticEvent{{
		OccurredAt: "2026-08-28T12:00:00Z", IncidentID: "incident-alpha",
		Operation: "agent_attempt", ProviderID: "openai",
		ProviderAccountID: "openai.primary", ModelID: "gpt-5.6-sol",
		HarnessID: "codex", Stage: "provider_http", ElapsedMS: 91,
		Result: "failed", ErrorCode: "rate_limited", Retryable: true,
	}}}

	gateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{
			Read: read, Setup: setup, Roundtable: roundtableSource, Incidents: incidents,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	turn := productConversationControlMetadataTurn(digest)
	calls := []controltool.Call{
		{ToolID: controltool.ToolMissionsSearch, Arguments: json.RawMessage(`{"query":"alpha","limit":8}`)},
		{ToolID: controltool.ToolMissionsStatus, Arguments: json.RawMessage(`{"mission_id":"mission/team-alpha"}`)},
		{ToolID: controltool.ToolTeamsSearch, Arguments: json.RawMessage(`{"query":"alpha","limit":8}`)},
		{ToolID: controltool.ToolTeamsStatus, Arguments: json.RawMessage(`{"team_instance_id":"team-alpha"}`)},
		{ToolID: controltool.ToolRoundtablesStatus, Arguments: json.RawMessage(`{"session_id":"roundtable-alpha"}`)},
		{ToolID: controltool.ToolGovernanceNeedsYou, Arguments: json.RawMessage(`{"limit":8}`)},
		{ToolID: controltool.ToolRuntimesStatus, Arguments: json.RawMessage(`{"adapter_type":"codex","limit":8}`)},
		{ToolID: controltool.ToolProvidersStatus, Arguments: json.RawMessage(`{"provider_id":"deepseek","provider_account_id":"deepseek.primary","limit":8}`)},
		{ToolID: controltool.ToolDiagnosticsIncident, Arguments: json.RawMessage(`{"incident_id":"incident-alpha"}`)},
		{ToolID: controltool.ToolLibrarySearch, Arguments: json.RawMessage(`{"query":"alpha","limit":8}`)},
	}
	combined := make([]byte, 0, 16<<10)
	for _, call := range calls {
		result, callErr := gateway.Call(context.Background(), turn, call)
		if callErr != nil {
			t.Fatalf("%s: %v", call.ToolID, callErr)
		}
		if len(result.Content) == 0 || !json.Valid(result.Content) ||
			result.Proposal != nil || result.ActionProposal != nil {
			t.Fatalf("invalid read result for %s: %#v", call.ToolID, result)
		}
		combined = append(combined, result.Content...)
	}
	output := string(combined)
	for _, expected := range []string{
		"mission/team-alpha", "team-alpha", "runtime-codex", "deepseek.primary",
		"roundtable-alpha", "incident-alpha", "evidence-alpha",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q from metadata output: %s", expected, output)
		}
	}
	for _, forbidden := range []string{
		"credential_reference", "vault/provider-secret", "vault/account-secret",
		"vault/roundtable-secret", "endpoint_fingerprint", "private-endpoint",
		"PRIVATE_DELIBERATION_BODY", "PRIVATE_AGENT_OUTPUT",
	} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("metadata output disclosed %q: %s", forbidden, output)
		}
	}
}

func TestProductConversationControlRoundTableTargetResolverFreezesAuthoritativeSeat(t *testing.T) {
	digest := strings.Repeat("a", 64)
	source := productConversationControlRoundtableFixture{view: roundtable.View{
		Session: roundtable.Session{
			ID: "session+本地", ModeratorSeat: "seat-moderator",
			Context: &roundtable.SessionContext{
				ConversationID: "conversation-1", MissionID: "mission/team-1",
				TeamID: "team-1", TeamVersion: 1, WorkspaceID: "workspace-1",
			},
		},
		Rounds: []roundtable.Round{{ID: "round#评审", Sequence: 1}},
		Seats: map[string]roundtable.Seat{
			"seat+c++/评审": {
				ID: "seat+c++/评审", DisplayName: "C++ Reviewer", Available: true,
				Binding: &roundtable.FrozenSeatBinding{
					MembershipRevision: 3, BindingDigest: digest,
				},
			},
		},
		Attempts: map[string]roundtable.SeatAttempt{
			"attempt+评审": {
				AttemptID: "attempt+评审", RoundID: "round#评审",
				SeatID: "seat+c++/评审", MembershipRevision: 3,
				SeatBindingDigest: digest, Status: roundtable.SeatAttemptFailed,
				Retryable: true,
			},
		},
	}}
	gateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{Roundtable: source},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload := controltool.ConversationActionPayload{
		SessionID: "session+本地", RoundID: "round#评审", SeatID: "seat+c++/评审",
	}
	target, err := gateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesSkipPreview, payload,
	)
	if err != nil || target.MembershipRevision != 3 ||
		target.SeatBindingDigest != digest {
		t.Fatalf("resolved target = %#v, error = %v", target, err)
	}
	payload.AttemptID = "attempt+评审"
	if _, err := gateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesRetryPreview, payload,
	); err != nil {
		t.Fatalf("exact Attempt target error = %v", err)
	}
	notRetryable := source
	notRetryable.view.Attempts = make(map[string]roundtable.SeatAttempt, len(source.view.Attempts))
	for id, attempt := range source.view.Attempts {
		attempt.Retryable = false
		notRetryable.view.Attempts[id] = attempt
	}
	notRetryableGateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{Roundtable: notRetryable},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := notRetryableGateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesRetryPreview, payload,
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("non-retryable Attempt target error = %v", err)
	}
	running := source
	running.view.Attempts = make(map[string]roundtable.SeatAttempt, len(source.view.Attempts))
	for id, attempt := range source.view.Attempts {
		attempt.Status = roundtable.SeatAttemptRunning
		attempt.Retryable = false
		running.view.Attempts[id] = attempt
	}
	runningGateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{Roundtable: running},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload.AttemptID = ""
	if _, err := runningGateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesSkipPreview, payload,
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("running seat skip target error = %v", err)
	}
	payload.AttemptID = "attempt-other"
	if _, err := gateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesRetryPreview, payload,
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("substituted Attempt target error = %v", err)
	}
	retired := source
	retired.view = roundtable.View{
		Session: source.view.Session, Rounds: source.view.Rounds,
		Seats:    make(map[string]roundtable.Seat, len(source.view.Seats)),
		Attempts: source.view.Attempts,
	}
	for id, seat := range source.view.Seats {
		retired.view.Seats[id] = seat
	}
	seat := retired.view.Seats["seat+c++/评审"]
	seat.Available = false
	retired.view.Seats[seat.ID] = seat
	retiredGateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{Roundtable: retired},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload.AttemptID = ""
	if _, err := retiredGateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesSkipPreview, payload,
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("retired seat target error = %v", err)
	}
	stale := source
	stale.view.Rounds = append(
		append([]roundtable.Round(nil), source.view.Rounds...),
		roundtable.Round{ID: "round-current", Sequence: 2},
	)
	staleGateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{Roundtable: stale},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload.RoundID = "round#评审"
	if _, err := staleGateway.ResolveRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesSkipPreview, payload,
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("historical Round target error = %v", err)
	}
	if err := staleGateway.ValidateRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesPausePreview,
		controltool.ConversationActionPayload{
			SessionID: payload.SessionID, RoundID: payload.RoundID,
		},
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("historical pause target error = %v", err)
	}
	if err := staleGateway.ValidateRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesPausePreview,
		controltool.ConversationActionPayload{
			SessionID: payload.SessionID, RoundID: "round-current",
		},
	); err != nil {
		t.Fatalf("current pause target error = %v", err)
	}
	paused := stale
	paused.view.Rounds = append([]roundtable.Round(nil), stale.view.Rounds...)
	paused.view.Rounds[len(paused.view.Rounds)-1].PauseRequested = true
	pausedGateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{Roundtable: paused},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := pausedGateway.ValidateRoundTableActionTarget(
		context.Background(), controltool.ToolRoundtablesPausePreview,
		controltool.ConversationActionPayload{
			SessionID: payload.SessionID, RoundID: "round-current",
		},
	); !errors.Is(err, controltool.ErrInvalidCall) {
		t.Fatalf("already-paused target error = %v", err)
	}
}

func TestProductConversationControlMetadataRejectsAmbiguousOrStaleCalls(t *testing.T) {
	digest := strings.Repeat("b", 64)
	gateway, err := newProductConversationControlMetadataGateway(
		productConversationControlMetadataSources{
			Read: productConversationControlReadFixture{snapshot: api.LocalProductSnapshot{
				SchemaVersion: 3,
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	turn := productConversationControlMetadataTurn(digest)
	for _, call := range []controltool.Call{
		{ToolID: controltool.ToolMissionsSearch, Arguments: json.RawMessage(`{"limit":2,"limit":3}`)},
		{ToolID: controltool.ToolMissionsSearch, Arguments: json.RawMessage(`{"unexpected":true}`)},
		{ToolID: controltool.ToolMissionsStatus, Arguments: json.RawMessage(`{"mission_id":""}`)},
		{ToolID: controltool.ToolSessionsSearch, Arguments: json.RawMessage(`{"query":"alpha"}`)},
	} {
		if _, callErr := gateway.Call(context.Background(), turn, call); callErr == nil {
			t.Fatalf("expected %s to fail closed", call.ToolID)
		}
	}

	stale := turn
	stale.Route.ExecutionBindingDigest = ""
	if _, callErr := gateway.Call(context.Background(), stale, controltool.Call{
		ToolID: controltool.ToolMissionsSearch, Arguments: json.RawMessage(`{}`),
	}); callErr == nil {
		t.Fatal("stale frozen turn should fail closed")
	}

	stale = turn
	stale.RegistryDigest = strings.Repeat("c", 64)
	if _, callErr := gateway.Call(context.Background(), stale, controltool.Call{
		ToolID: controltool.ToolMissionsSearch, Arguments: json.RawMessage(`{}`),
	}); callErr == nil {
		t.Fatal("substituted Tool Registry digest should fail closed")
	}
}

func productConversationControlMetadataTurn(digest string) controltool.TurnContext {
	registry, _ := controltool.NewBuiltinRegistry()
	return controltool.TurnContext{
		ConversationID: "conversation-alpha", SegmentID: "segment-alpha",
		AttemptID: "attempt-alpha", IncidentID: "incident-alpha",
		RegistryDigest: registry.Digest(),
		Route: controltool.FrozenRouteReference{
			HarnessAdapter: "codex", ProviderID: "openai",
			ProviderAccountID: "openai.primary", CredentialRevision: 7,
			ModelID: "gpt-5.6-sol", ReasoningEffort: "max",
			ExecutionBindingDigest: digest, ContextCapsuleDigest: digest,
		},
		Workspace: controltool.FrozenWorkspaceReference{
			WorkspaceID: "workspace-alpha", WorkspaceDigest: digest,
		},
	}
}
