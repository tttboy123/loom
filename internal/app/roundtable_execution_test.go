package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func roundtableMissionContextFixture(missionID string) RoundtableMissionContext {
	teamID := strings.TrimPrefix(missionID, "mission/")
	return RoundtableMissionContext{
		MissionID: missionID, TeamInstanceID: teamID,
		Objective: "Each Agent independently proposes one practical Mission improvement.",
		Status:    "blocked", PlanDigest: strings.Repeat("9", 64),
	}
}

type roundtableContextStoreFixture struct {
	authorities []contextcapsule.AuthorityRecord
	capsules    []contextcapsule.RoleContextCapsule
	payloads    [][]byte
}

func (store *roundtableContextStoreFixture) PutRoleContextCapsule(
	_ context.Context,
	capsule contextcapsule.RoleContextCapsule,
	payload []byte,
) error {
	store.authorities = append(store.authorities, capsule.AuthorityRecord())
	store.capsules = append(store.capsules, capsule)
	store.payloads = append(store.payloads, append([]byte(nil), payload...))
	return nil
}

func TestCompileRoundtableExecutionBuildsIndependentGovernedSeatAttempts(t *testing.T) {
	t.Parallel()

	sourcePath := t.TempDir()
	store := &roundtableContextStoreFixture{}
	now := time.Date(2026, 8, 26, 4, 0, 0, 0, time.UTC)
	mainBinding := roundtableExecutionBindingFixture(t, "profile-main", "runtime-main", "openai", "openai.primary", "gpt-5.6")
	peerBinding := roundtableExecutionBindingFixture(t, "profile-peer", "runtime-peer", "minimax", "minimax.primary", "MiniMax-M3")

	compilation, err := CompileRoundtableExecution(context.Background(), RoundtableExecutionInput{
		SessionID: "session-roundtable-1", RoundID: "round-1",
		ConversationID: "conversation-1", MissionID: "mission/team-1",
		MissionContext: roundtableMissionContextFixture("mission/team-1"),
		Title:          "Architecture review", Prompt: "Compare the two implementation strategies.",
		CorrelationID: "11111111-1111-4111-8111-111111111111",
		SourcePath:    sourcePath, AuthoritativeTime: now, ContextCapsules: store,
		Seats: []RoundtableSeatExecutionInput{
			{SeatID: "seat-main", DisplayName: "Planner", TeamRoleKind: "main", AgentDefinitionID: "agent-main", MembershipRevision: 1, SeatBindingDigest: strings.Repeat("a", 64), ExecutionBinding: mainBinding},
			{SeatID: "seat-peer", DisplayName: "Reviewer", TeamRoleKind: "subagent", AgentDefinitionID: "agent-peer", MembershipRevision: 1, SeatBindingDigest: strings.Repeat("b", 64), ExecutionBinding: peerBinding},
		},
	})
	if err != nil {
		t.Fatalf("CompileRoundtableExecution() error = %v", err)
	}
	if compilation.ExecutionTeamID == "team-1" || compilation.ExecutionTeamID == "" {
		t.Fatalf("ExecutionTeamID = %q, want isolated RoundTable execution identity", compilation.ExecutionTeamID)
	}
	if len(compilation.Attempts) != 2 || len(compilation.Request.Nodes) != 2 || len(store.authorities) != 2 {
		t.Fatalf("attempts/nodes/capsules = %d/%d/%d, want 2/2/2", len(compilation.Attempts), len(compilation.Request.Nodes), len(store.authorities))
	}
	if compilation.Request.ExecutionGenerationID != compilation.Request.CorrelationID {
		t.Fatalf(
			"ExecutionGenerationID = %q, want RoundTable correlation %q",
			compilation.Request.ExecutionGenerationID,
			compilation.Request.CorrelationID,
		)
	}
	if compilation.Attempts[0].SeatID != "seat-main" || compilation.Attempts[1].SeatID != "seat-peer" {
		t.Fatalf("attempt seat order = %#v", compilation.Attempts)
	}
	for index, attempt := range compilation.Attempts {
		if attempt.AttemptID == "" || attempt.WorkItemID == "" || attempt.RunID == "" ||
			attempt.SegmentID == "" || attempt.ContextCapsuleDigest == "" ||
			attempt.ExecutionBindingDigest == "" ||
			attempt.ClaimGeneration != 1 {
			t.Fatalf("attempt[%d] missing frozen identity: %#v", index, attempt)
		}
		if store.authorities[index].CapsuleDigest != attempt.ContextCapsuleDigest {
			t.Fatalf("attempt[%d] capsule digest drift", index)
		}
		wantWorkItemID := appTeamAttemptIdentity(
			"work", compilation.Request.Plan, attempt.SeatID, 1,
			appTeamAttemptIdentitySalt(compilation.Request)...,
		)
		wantRunID := appTeamAttemptIdentity(
			"run", compilation.Request.Plan, attempt.SeatID, 1,
			appTeamAttemptIdentitySalt(compilation.Request)...,
		)
		if attempt.WorkItemID != wantWorkItemID || attempt.RunID != wantRunID {
			t.Fatalf(
				"attempt[%d] lineage = %q/%q, coordinator will dispatch %q/%q",
				index, attempt.WorkItemID, attempt.RunID, wantWorkItemID, wantRunID,
			)
		}
		if strings.Contains(string(store.payloads[index]), mainBinding.CredentialReference) ||
			strings.Contains(string(store.payloads[index]), peerBinding.CredentialReference) {
			t.Fatalf("attempt[%d] payload disclosed credential reference", index)
		}
	}
	var missionItem contextcapsule.DisclosedItem
	for _, item := range store.capsules[0].Disclosed() {
		if item.ItemID == "roundtable-mission" {
			missionItem = item
			break
		}
	}
	if missionItem.Kind != contextcapsule.KindCurrentTaskState ||
		missionItem.Trust != contextcapsule.TrustAuthoritative ||
		missionItem.Scope != contextcapsule.ScopeTeamShared || !missionItem.Required ||
		missionItem.SourceRef != "mission-execution:team-1:"+strings.Repeat("9", 64) {
		t.Fatalf("Mission Capsule item = %#v", missionItem)
	}
	var missionContext RoundtableMissionContext
	if err := json.Unmarshal(missionItem.Content, &missionContext); err != nil ||
		missionContext != roundtableMissionContextFixture("mission/team-1") {
		t.Fatalf("Mission Capsule content = %#v, error = %v", missionContext, err)
	}
	if _, err := validateTeamExecutionRequest(context.Background(), compilation.Request); err != nil {
		t.Fatalf("compiled request is not accepted by TeamCoordinator: %v", err)
	}
}

func TestCompileRoundtableExecutionBuildsFreshSingleSeatRetry(t *testing.T) {
	t.Parallel()

	store := &roundtableContextStoreFixture{}
	binding := roundtableExecutionBindingFixture(
		t, "profile-peer", "runtime-peer", "minimax", "minimax.primary", "MiniMax-M3",
	)
	base := RoundtableExecutionInput{
		Mode:      RoundtableExecutionInitial,
		SessionID: "session-roundtable-retry", RoundID: "round-1",
		ConversationID: "conversation-retry", MissionID: "mission/team-retry",
		MissionContext: roundtableMissionContextFixture("mission/team-retry"),
		Title:          "Retry review", Prompt: "Review the proposed change.\nFocus on lifecycle safety.",
		CorrelationID:     "33333333-3333-4333-8333-333333333333",
		SourcePath:        t.TempDir(),
		AuthoritativeTime: time.Date(2026, 8, 26, 4, 10, 0, 0, time.UTC),
		ContextCapsules:   store,
	}
	main := RoundtableSeatExecutionInput{
		SeatID: "seat-main", DisplayName: "Planner", TeamRoleKind: "main",
		AgentDefinitionID: "agent-main", MembershipRevision: 1,
		SeatBindingDigest: strings.Repeat("a", 64), ExecutionBinding: binding,
		AttemptNumber: 1,
	}
	peer := RoundtableSeatExecutionInput{
		SeatID: "seat-peer", DisplayName: "Reviewer", TeamRoleKind: "subagent",
		AgentDefinitionID: "agent-peer", MembershipRevision: 1,
		SeatBindingDigest: strings.Repeat("b", 64), ExecutionBinding: binding,
		AttemptNumber: 1,
	}
	base.Seats = []RoundtableSeatExecutionInput{main, peer}
	initial, err := CompileRoundtableExecution(context.Background(), base)
	if err != nil {
		t.Fatalf("initial CompileRoundtableExecution() error = %v", err)
	}

	base.Mode = RoundtableExecutionRetry
	base.RetryGuidance = "Return a concise plain-text review."
	base.RetryInterventionID = "intervention-retry-peer-2"
	base.CorrelationID = "44444444-4444-4444-8444-444444444444"
	peer.AttemptNumber = 2
	base.Seats = []RoundtableSeatExecutionInput{peer}
	retry, err := CompileRoundtableExecution(context.Background(), base)
	if err != nil {
		t.Fatalf("retry CompileRoundtableExecution() error = %v", err)
	}
	if len(retry.Attempts) != 1 || retry.Attempts[0].AttemptNumber != 2 {
		t.Fatalf("retry Attempts = %#v, want one attempt number 2", retry.Attempts)
	}
	firstPeer := initial.Attempts[1]
	retried := retry.Attempts[0]
	if retried.AttemptID == firstPeer.AttemptID || retried.WorkItemID == firstPeer.WorkItemID ||
		retried.RunID == firstPeer.RunID || retried.SegmentID == firstPeer.SegmentID {
		t.Fatalf("retry reused prior execution identity: first=%#v retry=%#v", firstPeer, retried)
	}
	if retried.ExecutionBindingDigest != binding.BindingDigest ||
		retried.SeatBindingDigest != peer.SeatBindingDigest ||
		retried.MembershipRevision != peer.MembershipRevision {
		t.Fatalf("retry changed frozen binding: %#v", retried)
	}
	if len(retry.Request.Nodes) != 1 || retry.Request.Nodes[0].LogicalNodeID != peer.SeatID {
		t.Fatalf("retry nodes = %#v", retry.Request.Nodes)
	}
	retryItems := make(map[string]contextcapsule.DisclosedItem)
	for _, item := range store.capsules[len(store.capsules)-1].Disclosed() {
		retryItems[item.ItemID] = item
	}
	promptItem, promptFound := retryItems["roundtable-prompt"]
	guidanceItem, guidanceFound := retryItems["roundtable-retry-guidance"]
	if !promptFound || string(promptItem.Content) != base.Prompt ||
		promptItem.Kind != contextcapsule.KindConversationGoal ||
		promptItem.Trust != contextcapsule.TrustAuthoritative {
		t.Fatalf("retry original prompt authority = %#v", promptItem)
	}
	if !guidanceFound || string(guidanceItem.Content) != base.RetryGuidance ||
		guidanceItem.Kind != contextcapsule.KindConfirmedConstraint ||
		guidanceItem.Trust != contextcapsule.TrustAuthoritative ||
		guidanceItem.Scope != contextcapsule.ScopeRoleRestricted ||
		guidanceItem.AllowedRoleID != peer.SeatID ||
		guidanceItem.SourceRef != "roundtable-intervention:"+base.RetryInterventionID {
		t.Fatalf("retry guidance authority = %#v", guidanceItem)
	}
	for _, item := range retryItems {
		if item.Kind == contextcapsule.KindPriorModelOutput ||
			item.SourceType == contextcapsule.SourceModelOutput {
			t.Fatalf("retry promoted prior model output = %#v", item)
		}
	}
	if _, err := validateTeamExecutionRequest(context.Background(), retry.Request); err != nil {
		t.Fatalf("retry request is not accepted by TeamCoordinator: %v", err)
	}
}

func TestCompileRoundtableExecutionCarriesPriorContributionsAsUntrustedContext(t *testing.T) {
	t.Parallel()

	store := &roundtableContextStoreFixture{}
	binding := roundtableExecutionBindingFixture(
		t, "profile-main", "runtime-main", "minimax", "minimax.primary", "MiniMax-M3",
	)
	content := []byte("The participant recommends preserving conversation continuity.")
	digest := sha256.Sum256(content)
	input := RoundtableExecutionInput{
		SessionID: "session-roundtable-synthesis", RoundID: "round-2",
		ConversationID: "conversation-synthesis", MissionID: "mission/team-synthesis",
		MissionContext: roundtableMissionContextFixture("mission/team-synthesis"),
		Title:          "Synthesis", Prompt: "Synthesize the prior Agent contributions.",
		CorrelationID:     "55555555-5555-4555-8555-555555555555",
		SourcePath:        t.TempDir(),
		AuthoritativeTime: time.Date(2026, 8, 27, 1, 0, 0, 0, time.UTC),
		ContextCapsules:   store,
		PriorContributions: []RoundtablePriorContribution{{
			AttemptID: "attempt-participant-round-1", RoundID: "round-1",
			SeatID: "seat-participant", OutputDigest: hex.EncodeToString(digest[:]),
			Content: content,
		}},
		Seats: []RoundtableSeatExecutionInput{
			{
				SeatID: "seat-main", DisplayName: "Lead", TeamRoleKind: "main",
				AgentDefinitionID: "agent-main", MembershipRevision: 1,
				SeatBindingDigest: strings.Repeat("a", 64), ExecutionBinding: binding,
			},
			{
				SeatID: "seat-participant", DisplayName: "Participant",
				TeamRoleKind: "subagent", AgentDefinitionID: "agent-participant",
				MembershipRevision: 1, SeatBindingDigest: strings.Repeat("b", 64),
				ExecutionBinding: binding,
			},
		},
	}
	compilation, err := CompileRoundtableExecution(context.Background(), input)
	if err != nil {
		t.Fatalf("CompileRoundtableExecution() error = %v", err)
	}
	if len(compilation.Attempts) != 2 || len(store.capsules) != 2 {
		t.Fatalf("synthesis attempts/capsules = %d/%d", len(compilation.Attempts), len(store.capsules))
	}
	for index, capsule := range store.capsules {
		var prior contextcapsule.DisclosedItem
		found := false
		for _, item := range capsule.Disclosed() {
			if item.Kind == contextcapsule.KindPriorModelOutput {
				if found {
					t.Fatalf("capsule[%d] disclosed duplicate prior output", index)
				}
				prior, found = item, true
			}
		}
		if !found || string(prior.Content) != string(content) ||
			prior.Trust != contextcapsule.TrustUntrusted ||
			prior.Scope != contextcapsule.ScopeTeamShared ||
			prior.Priority != contextcapsule.PriorityHistory || prior.Required ||
			prior.SourceType != contextcapsule.SourceModelOutput ||
			prior.SourceRef != "roundtable-attempt:attempt-participant-round-1:"+
				hex.EncodeToString(digest[:]) {
			t.Fatalf("capsule[%d] prior contribution = %#v", index, prior)
		}
	}

	input.ContextCapsules = &roundtableContextStoreFixture{}
	input.PriorContributions[0].OutputDigest = strings.Repeat("f", 64)
	if _, err := CompileRoundtableExecution(context.Background(), input); err == nil {
		t.Fatal("CompileRoundtableExecution() accepted a substituted prior output digest")
	}
	input.PriorContributions[0].OutputDigest = hex.EncodeToString(digest[:])
	input.PriorContributions = append(input.PriorContributions, input.PriorContributions[0])
	if _, err := CompileRoundtableExecution(context.Background(), input); err == nil {
		t.Fatal("CompileRoundtableExecution() accepted duplicate prior Attempt output")
	}

	unsafe := []byte("TOKEN=untrusted-model-claim")
	unsafeDigest := sha256.Sum256(unsafe)
	unsafeStore := &roundtableContextStoreFixture{}
	input.ContextCapsules = unsafeStore
	input.PriorContributions = []RoundtablePriorContribution{{
		AttemptID: "attempt-unsafe-round-1", RoundID: "round-1",
		SeatID: "seat-participant", OutputDigest: hex.EncodeToString(unsafeDigest[:]),
		Content: unsafe,
	}}
	if _, err := CompileRoundtableExecution(context.Background(), input); err != nil {
		t.Fatalf("untrusted unsafe prior output was not policy-filtered: %v", err)
	}
	for index, capsule := range unsafeStore.capsules {
		priorDisclosed := 0
		for _, item := range capsule.Disclosed() {
			if item.Kind == contextcapsule.KindPriorModelOutput {
				priorDisclosed++
			}
		}
		priorOmitted := 0
		for _, item := range capsule.Omitted() {
			if item.Kind == contextcapsule.KindPriorModelOutput &&
				item.Reason == contextcapsule.OmissionPolicyFiltered {
				priorOmitted++
			}
		}
		if priorDisclosed != 0 || priorOmitted != 1 {
			t.Fatalf(
				"unsafe capsule[%d] prior disclosed/omitted = %d/%d",
				index, priorDisclosed, priorOmitted,
			)
		}
	}
}

func TestCompileRoundtableExecutionRejectsBindingSubstitutionAndMissingMainSeat(t *testing.T) {
	t.Parallel()

	binding := roundtableExecutionBindingFixture(t, "profile-peer", "runtime-peer", "minimax", "minimax.primary", "MiniMax-M3")
	base := RoundtableExecutionInput{
		SessionID: "session-roundtable-2", RoundID: "round-1",
		ConversationID: "conversation-2", MissionID: "mission/team-2",
		MissionContext: roundtableMissionContextFixture("mission/team-2"),
		Title:          "Review", Prompt: "Review the plan.",
		CorrelationID: "22222222-2222-4222-8222-222222222222",
		SourcePath:    t.TempDir(), AuthoritativeTime: time.Date(2026, 8, 26, 4, 5, 0, 0, time.UTC),
		ContextCapsules: &roundtableContextStoreFixture{},
		Seats: []RoundtableSeatExecutionInput{
			{SeatID: "seat-one", DisplayName: "One", TeamRoleKind: "subagent", AgentDefinitionID: "agent-one", MembershipRevision: 1, SeatBindingDigest: strings.Repeat("c", 64), ExecutionBinding: binding},
			{SeatID: "seat-two", DisplayName: "Two", TeamRoleKind: "subagent", AgentDefinitionID: "agent-two", MembershipRevision: 1, SeatBindingDigest: strings.Repeat("d", 64), ExecutionBinding: binding},
		},
	}
	if _, err := CompileRoundtableExecution(context.Background(), base); err == nil {
		t.Fatal("CompileRoundtableExecution() accepted a RoundTable without one main seat")
	}

	base.Seats[0].TeamRoleKind = "main"
	base.Seats[0].ExecutionBinding.BindingDigest = strings.Repeat("f", 64)
	if _, err := CompileRoundtableExecution(context.Background(), base); err == nil {
		t.Fatal("CompileRoundtableExecution() accepted a substituted frozen binding")
	}

	base.MissionContext.MissionID = "mission/substituted"
	if _, err := CompileRoundtableExecution(context.Background(), base); err == nil {
		t.Fatal("CompileRoundtableExecution() accepted substituted Mission context")
	}
}

func roundtableExecutionBindingFixture(
	t *testing.T,
	profileID, runtimeID, providerID, accountID, modelID string,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: profileID, AdapterType: "loom-native", ProviderID: providerID,
		ProviderAccountID: accountID, ModelID: modelID,
		AuthMode: loomruntime.AuthBrokered, EndpointFingerprint: strings.Repeat("e", 64),
		CredentialReference: "credential-ref-" + strings.ReplaceAll(accountID, ".", "-"), CredentialRevision: 3,
		RequiredCapabilities: []string{"models"}, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewRuntimeProfile() error = %v", err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: runtimeID, DeviceID: "device-local", AdapterType: "loom-native",
		DisplayName: "Fixture", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"models"}, Capacity: 2,
	})
	if err != nil {
		t.Fatalf("NewRuntimeInstance() error = %v", err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatalf("FreezeExecutionBinding() error = %v", err)
	}
	return binding
}
