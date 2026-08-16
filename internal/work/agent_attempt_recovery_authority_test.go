package work

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/journal"
)

func TestAgentAttemptRecoveryAuthorityApprovesExactPreModelCandidateContentFree(t *testing.T) {
	ctx, journalStore, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-exact",
	)
	authority, err := NewAgentAttemptRecoveryAuthority(
		coordinator,
		agentAttemptRestartCapabilityResolverFixture{capability: agentAttemptRestartCapabilityFixture(outcome)},
		func() time.Time { return time.Date(2026, 8, 14, 18, 0, 0, 0, time.UTC) },
		cryptorand.Reader,
	)
	if err != nil {
		t.Fatal(err)
	}
	capability := agentAttemptRestartCapabilityFixture(outcome)
	input := AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		DecisionID:       "11111111-1111-4111-8111-111111111111",
		CorrelationID:    "22222222-2222-4222-8222-222222222222",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	}

	decision, err := authority.Authorize(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.SchemaVersion != 1 || decision.DecisionID != input.DecisionID ||
		decision.Action != input.Action || decision.CandidateDigest != input.CandidateDigest ||
		decision.CapabilityDigest != input.CapabilityDigest || decision.EventID == "" {
		t.Fatalf("decision = %#v", decision)
	}
	replayed, err := authority.Authorize(ctx, input)
	if err != nil || replayed != decision {
		t.Fatalf("idempotent replay = %#v, %v", replayed, err)
	}

	events, err := journalStore.ReadStream(ctx, decision.StreamID)
	if err != nil || len(events) != 1 || events[0].Type != "AgentAttemptRecoveryAuthorized" ||
		events[0].CorrelationID != input.CorrelationID ||
		events[0].CausationID == "" ||
		events[0].CausationID == outcome.Binding.PayloadAuthority.IncidentID {
		t.Fatalf("events = %#v, %v", events, err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"private restart authority input", "Authorization", "api_key", "provider_response",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("recovery authority leaked content %q: %s", forbidden, encoded)
		}
	}
}

func TestAgentAttemptRecoveryAuthorityPreviewsAvailableAuthorizedAndConsumedCandidate(t *testing.T) {
	ctx, _, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-preview",
	)
	capability := agentAttemptRestartCapabilityFixture(outcome)
	authority, err := NewAgentAttemptRecoveryAuthority(
		coordinator,
		agentAttemptRestartCapabilityResolverFixture{capability: capability},
		func() time.Time { return time.Date(2026, 8, 14, 19, 0, 0, 0, time.UTC) },
		cryptorand.Reader,
	)
	if err != nil {
		t.Fatal(err)
	}

	assertPreview := func(status AgentAttemptRecoveryCandidateStatus, decisionID string) {
		t.Helper()
		preview, previewErr := authority.Preview(ctx)
		if previewErr != nil {
			t.Fatal(previewErr)
		}
		if preview.SchemaVersion != 1 || len(preview.Candidates) != 1 {
			t.Fatalf("preview = %#v", preview)
		}
		candidate := preview.Candidates[0]
		binding := outcome.Binding.PayloadAuthority
		if candidate.SchemaVersion != 1 || candidate.Status != status ||
			candidate.Action != AgentAttemptRecoveryResumePreModel ||
			candidate.DecisionID != decisionID ||
			candidate.CandidateDigest != AgentAttemptRestartCandidateDigest(outcome) ||
			candidate.CapabilityDigest != capability.CapabilityDigest ||
			candidate.AttemptID != outcome.Binding.AttemptID ||
			candidate.TeamInstanceID != outcome.Binding.TeamInstanceID ||
			candidate.SegmentID != outcome.SegmentID ||
			candidate.WorkItemID != binding.WorkItemID || candidate.RunID != binding.RunID ||
			candidate.ClaimGeneration != binding.ClaimGeneration ||
			candidate.RuntimeInstanceID != binding.RuntimeInstanceID ||
			candidate.AgentInstanceID != binding.AgentInstanceID ||
			candidate.HarnessAdapter != outcome.ExecutionBinding.HarnessAdapter ||
			candidate.ProviderID != outcome.ExecutionBinding.ProviderID ||
			candidate.ProviderAccountID != outcome.ExecutionBinding.ProviderAccountID ||
			candidate.ModelID != outcome.ExecutionBinding.ModelID ||
			candidate.CredentialRevision != outcome.ExecutionBinding.CredentialRevision {
			t.Fatalf("candidate = %#v", candidate)
		}
	}

	assertPreview(AgentAttemptRecoveryCandidateAvailable, "")
	decisionInput := AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		DecisionID:       "13131313-1313-4313-8313-131313131313",
		CorrelationID:    "14141414-1414-4414-8414-141414141414",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	}
	decision, err := authority.Authorize(ctx, decisionInput)
	if err != nil {
		t.Fatal(err)
	}
	assertPreview(AgentAttemptRecoveryCandidateAuthorized, decision.DecisionID)
	if _, err := authority.Consume(ctx, AgentAttemptRecoveryConsumeInput{
		SchemaVersion: 1,
		CorrelationID: "15151515-1515-4515-8515-151515151515",
		DecisionID:    decision.DecisionID, CandidateDigest: decision.CandidateDigest,
		CapabilityDigest: decision.CapabilityDigest,
	}); err != nil {
		t.Fatal(err)
	}
	assertPreview(AgentAttemptRecoveryCandidateConsumed, decision.DecisionID)
}

func TestAgentAttemptRecoveryAuthorityRejectsUncertainProviderAndCapabilityDrift(t *testing.T) {
	ctx, _, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-reject",
	)
	capability := agentAttemptRestartCapabilityFixture(outcome)
	input := AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		DecisionID:       "33333333-3333-4333-8333-333333333333",
		CorrelationID:    "44444444-4444-4444-8444-444444444444",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	}

	drifted := capability
	drifted.SessionBindingDigest = strings.Repeat("9", 64)
	driftedAuthority, _ := NewAgentAttemptRecoveryAuthority(
		coordinator, agentAttemptRestartCapabilityResolverFixture{capability: drifted}, time.Now,
		cryptorand.Reader,
	)
	if _, err := driftedAuthority.Authorize(ctx, input); !errors.Is(err, ErrAgentAttemptRecoveryCapability) {
		t.Fatalf("capability drift error = %v", err)
	}

	uncertainCtx, _, uncertainCoordinator, uncertain := agentAttemptUncertainRecoveryCandidateFixture(
		t, "recovery-authority-uncertain",
	)
	uncertainCapability := agentAttemptRestartCapabilityFixture(uncertain)
	uncertainAuthority, _ := NewAgentAttemptRecoveryAuthority(
		uncertainCoordinator,
		agentAttemptRestartCapabilityResolverFixture{capability: uncertainCapability},
		time.Now,
		cryptorand.Reader,
	)
	input.CandidateDigest = AgentAttemptRestartCandidateDigest(uncertain)
	input.CapabilityDigest = uncertainCapability.CapabilityDigest
	if _, err := uncertainAuthority.Authorize(uncertainCtx, input); !errors.Is(err, ErrAgentAttemptRecoveryUnsafe) {
		t.Fatalf("uncertain Provider error = %v", err)
	}
}

func TestAgentAttemptRecoveryAuthorityCASAllowsOneDecision(t *testing.T) {
	ctx, journalStore, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-cas",
	)
	capability := agentAttemptRestartCapabilityFixture(outcome)
	authority, _ := NewAgentAttemptRecoveryAuthority(
		coordinator, agentAttemptRestartCapabilityResolverFixture{capability: capability}, time.Now,
		cryptorand.Reader,
	)
	base := AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		CorrelationID:    "55555555-5555-4555-8555-555555555555",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	}
	inputs := []AgentAttemptRecoveryDecisionInput{base, base}
	inputs[0].DecisionID = "66666666-6666-4666-8666-666666666666"
	inputs[1].DecisionID = "77777777-7777-4777-8777-777777777777"

	start := make(chan struct{})
	errs := make([]error, len(inputs))
	var decisions [2]AgentAttemptRecoveryDecision
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			decisions[index], errs[index] = authority.Authorize(ctx, inputs[index])
		}(index)
	}
	close(start)
	wait.Wait()
	succeeded, conflicted := 0, 0
	var streamID string
	for index, err := range errs {
		switch {
		case err == nil:
			succeeded++
			streamID = decisions[index].StreamID
		case errors.Is(err, ErrAgentAttemptRecoveryConflict):
			conflicted++
		default:
			t.Fatalf("decision %d error = %v", index, err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("succeeded=%d conflicted=%d errors=%#v", succeeded, conflicted, errs)
	}
	events, err := journalStore.ReadStream(ctx, streamID)
	if err != nil || len(events) != 1 {
		t.Fatalf("CAS events = %#v, %v", events, err)
	}
}

func TestAgentAttemptRecoveryAuthorityConsumesDecisionExactlyOnce(t *testing.T) {
	ctx, journalStore, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-consume",
	)
	capability := agentAttemptRestartCapabilityFixture(outcome)
	authority, err := NewAgentAttemptRecoveryAuthority(
		coordinator, agentAttemptRestartCapabilityResolverFixture{capability: capability},
		func() time.Time { return time.Date(2026, 8, 14, 18, 15, 0, 0, time.UTC) },
		cryptorand.Reader,
	)
	if err != nil {
		t.Fatal(err)
	}
	decisionInput := AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		DecisionID:       "88888888-8888-4888-8888-888888888888",
		CorrelationID:    "99999999-9999-4999-8999-999999999999",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	}
	decision, err := authority.Authorize(ctx, decisionInput)
	if err != nil {
		t.Fatal(err)
	}
	consumeInput := AgentAttemptRecoveryConsumeInput{
		SchemaVersion: 1,
		CorrelationID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		DecisionID:    decision.DecisionID, CandidateDigest: decision.CandidateDigest,
		CapabilityDigest: decision.CapabilityDigest,
	}
	lease, err := authority.Consume(ctx, consumeInput)
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil || lease.LeaseID() == "" || lease.DecisionID() != decision.DecisionID ||
		lease.CandidateDigest() != decision.CandidateDigest ||
		lease.CapabilityDigest() != decision.CapabilityDigest ||
		lease.AttemptID() != outcome.Binding.AttemptID ||
		lease.RuntimeInstanceID() != outcome.Binding.PayloadAuthority.RuntimeInstanceID ||
		lease.CheckpointDigest() != outcome.CheckpointDigest {
		t.Fatalf("lease = %#v", lease)
	}
	grant, err := lease.Take()
	if err != nil || grant.LeaseID() != lease.LeaseID() ||
		grant.DecisionID() != lease.DecisionID() ||
		grant.IncidentID() != consumeInput.CorrelationID ||
		grant.Outcome().CheckpointDigest != outcome.CheckpointDigest ||
		grant.Outcome().SegmentID != outcome.SegmentID ||
		grant.Capability().CapabilityDigest != capability.CapabilityDigest {
		t.Fatalf("grant = %#v, %v", grant, err)
	}
	validatedOutcome, validatedCapability, err := ValidateAgentAttemptRecoveryDispatchGrant(grant)
	if err != nil || validatedOutcome.SegmentID != outcome.SegmentID ||
		validatedCapability != capability {
		t.Fatalf("validated grant = %#v / %#v, %v", validatedOutcome, validatedCapability, err)
	}
	drifted := grant
	drifted.incidentID = ""
	if _, _, err := ValidateAgentAttemptRecoveryDispatchGrant(drifted); !errors.Is(err, ErrInvalidAgentAttemptRecovery) {
		t.Fatalf("drifted grant error = %v", err)
	}
	if _, err := lease.Take(); !errors.Is(err, ErrAgentAttemptRecoveryConsumed) {
		t.Fatalf("second lease take error = %v", err)
	}
	if _, err := authority.Consume(ctx, consumeInput); !errors.Is(err, ErrAgentAttemptRecoveryConsumed) {
		t.Fatalf("second consumption error = %v", err)
	}
	events, err := journalStore.ReadStream(ctx, decision.StreamID)
	if err != nil || len(events) != 2 ||
		events[1].Type != "AgentAttemptRecoveryConsumed" ||
		events[1].CausationID != events[0].ID ||
		events[1].CorrelationID != consumeInput.CorrelationID {
		t.Fatalf("consumption events = %#v, %v", events, err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private restart authority input") {
		t.Fatalf("consumption leaked input: %s", encoded)
	}
}

func TestAgentAttemptRestartCandidateDigestFreezesRouteSegment(t *testing.T) {
	_, _, _, outcome := agentAttemptRecoveryCandidateFixture(t, "recovery-segment-digest")
	if outcome.SegmentID == "" {
		t.Fatal("recovery outcome omitted route segment")
	}
	drifted := outcome
	drifted.SegmentID = "segment-drifted"
	if AgentAttemptRestartCandidateDigest(outcome) == AgentAttemptRestartCandidateDigest(drifted) {
		t.Fatal("candidate digest did not freeze route segment")
	}
}

func TestAgentAttemptRecoveryAuthorityConcurrentConsumptionIssuesOneLease(t *testing.T) {
	ctx, _, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-consume-cas",
	)
	capability := agentAttemptRestartCapabilityFixture(outcome)
	authority, _ := NewAgentAttemptRecoveryAuthority(
		coordinator, agentAttemptRestartCapabilityResolverFixture{capability: capability},
		time.Now, cryptorand.Reader,
	)
	decision, err := authority.Authorize(ctx, AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		DecisionID:       "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		CorrelationID:    "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := AgentAttemptRecoveryConsumeInput{
		SchemaVersion: 1,
		CorrelationID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		DecisionID:    decision.DecisionID, CandidateDigest: decision.CandidateDigest,
		CapabilityDigest: decision.CapabilityDigest,
	}
	start := make(chan struct{})
	errs := make([]error, 2)
	leases := make([]*AgentAttemptRecoveryDispatchLease, 2)
	var wait sync.WaitGroup
	for index := range errs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			leases[index], errs[index] = authority.Consume(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	succeeded, rejected := 0, 0
	for index, err := range errs {
		switch {
		case err == nil && leases[index] != nil:
			succeeded++
		case errors.Is(err, ErrAgentAttemptRecoveryConsumed) ||
			errors.Is(err, ErrAgentAttemptRecoveryConflict):
			rejected++
		default:
			t.Fatalf("consume %d lease=%#v err=%v", index, leases[index], err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("succeeded=%d rejected=%d errors=%#v", succeeded, rejected, errs)
	}
}

func TestAgentAttemptRecoveryAuthorityRejectsCapabilityDriftBeforeConsumptionCommit(t *testing.T) {
	ctx, journalStore, coordinator, outcome := agentAttemptRecoveryCandidateFixture(
		t, "recovery-authority-consume-capability-drift",
	)
	capability := agentAttemptRestartCapabilityFixture(outcome)
	drifted := capability
	drifted.SessionBindingDigest = strings.Repeat("7", 64)
	drifted.CapabilityDigest = AgentAttemptRestartCapabilityDigest(drifted)
	resolver := &agentAttemptRestartCapabilitySequenceFixture{capabilities: []AgentAttemptRestartCapability{
		capability, capability, capability, drifted,
	}}
	authority, _ := NewAgentAttemptRecoveryAuthority(
		coordinator, resolver, time.Now, cryptorand.Reader,
	)
	decision, err := authority.Authorize(ctx, AgentAttemptRecoveryDecisionInput{
		SchemaVersion:    1,
		DecisionID:       "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee",
		CorrelationID:    "ffffffff-ffff-4fff-8fff-ffffffffffff",
		PrincipalID:      "local-user",
		Action:           AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Consume(ctx, AgentAttemptRecoveryConsumeInput{
		SchemaVersion: 1,
		CorrelationID: "12121212-1212-4212-8212-121212121212",
		DecisionID:    decision.DecisionID, CandidateDigest: decision.CandidateDigest,
		CapabilityDigest: decision.CapabilityDigest,
	}); !errors.Is(err, ErrAgentAttemptRecoveryCapability) {
		t.Fatalf("capability drift consume error = %v", err)
	}
	events, err := journalStore.ReadStream(ctx, decision.StreamID)
	if err != nil || len(events) != 1 || events[0].Type != "AgentAttemptRecoveryAuthorized" {
		t.Fatalf("drifted consumption events = %#v, %v", events, err)
	}
}

func agentAttemptRecoveryCandidateFixture(
	t *testing.T,
	suffix string,
) (context.Context, *journal.Store, *AgentInboxCoordinator, AgentAttemptRestartOutcome) {
	t.Helper()
	ctx, journalStore, loops, inboxAuthority, binding, budget := agentInboxFixture(t, suffix)
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(inboxAuthority, store)
	queued := agentInboxPayloadWithContent(
		binding, "input-"+suffix, 1, "private restart authority input",
	)
	queued.Binding.Mode = agentinbox.ModeQueue
	queued.Binding.ContextScope = agentinbox.ScopeConversationShared
	queued.Binding.TargetTurnID, queued.Binding.TargetTurnSequence = "turn-2", 2
	if _, err := coordinator.Admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}
	if _, err := inboxAuthority.startQueuedTurn(ctx, binding, queued.Binding, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: queued.Binding.ContentDigest,
	}); err != nil {
		t.Fatal(err)
	}
	restartedPayloads, _ := NewAttemptPayloadAuthority(loops.runs)
	restartedLoops, _ := NewAttemptLoopAuthority(loops.runs, restartedPayloads)
	restartedInbox, _ := NewAgentInboxAuthority(restartedLoops)
	restarted, _ := NewAgentInboxCoordinator(restartedInbox, store)
	report, err := restarted.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("restart report = %#v, %v", report, err)
	}
	return ctx, journalStore, restarted, report.Outcomes[0]
}

func agentAttemptRestartCapabilityFixture(
	outcome AgentAttemptRestartOutcome,
) AgentAttemptRestartCapability {
	capability := AgentAttemptRestartCapability{
		SchemaVersion:          1,
		AttemptID:              outcome.Binding.AttemptID,
		RuntimeInstanceID:      outcome.Binding.PayloadAuthority.RuntimeInstanceID,
		ExecutionBindingDigest: outcome.Binding.PayloadAuthority.ExecutionBindingDigest,
		ContextCapsuleDigest:   outcome.Binding.PayloadAuthority.CapsuleDigest,
		ResumeMode:             AgentAttemptRestartResumeFromCheckpoint,
		SessionBindingDigest:   strings.Repeat("8", 64),
	}
	capability.CapabilityDigest = AgentAttemptRestartCapabilityDigest(capability)
	return capability
}

type agentAttemptRestartCapabilityResolverFixture struct {
	capability AgentAttemptRestartCapability
	err        error
}

type agentAttemptRestartCapabilitySequenceFixture struct {
	mu           sync.Mutex
	capabilities []AgentAttemptRestartCapability
	calls        int
}

func (fixture *agentAttemptRestartCapabilitySequenceFixture) ResolveAgentAttemptRestartCapability(
	_ context.Context,
	_ AgentAttemptRestartOutcome,
) (AgentAttemptRestartCapability, error) {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.calls >= len(fixture.capabilities) {
		return AgentAttemptRestartCapability{}, errors.New("capability sequence exhausted")
	}
	capability := fixture.capabilities[fixture.calls]
	fixture.calls++
	return capability, nil
}

func (fixture agentAttemptRestartCapabilityResolverFixture) ResolveAgentAttemptRestartCapability(
	_ context.Context,
	_ AgentAttemptRestartOutcome,
) (AgentAttemptRestartCapability, error) {
	return fixture.capability, fixture.err
}

func agentAttemptUncertainRecoveryCandidateFixture(
	t *testing.T,
	suffix string,
) (context.Context, *journal.Store, *AgentInboxCoordinator, AgentAttemptRestartOutcome) {
	t.Helper()
	ctx, journalStore, loops, inboxAuthority, binding, budget := agentInboxFixture(t, suffix)
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1", RequestID: "request-uncertain",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(inboxAuthority, store)
	report, err := coordinator.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("uncertain restart report = %#v, %v", report, err)
	}
	return ctx, journalStore, coordinator, report.Outcomes[0]
}
