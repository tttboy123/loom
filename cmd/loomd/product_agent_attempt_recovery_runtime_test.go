package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

const productRecoveryCheckpointContent = "Previous encrypted Loom Native checkpoint"

func TestProductAgentAttemptRecoveryAttachesThenRegistersAndClosesInOrder(t *testing.T) {
	ctx, lease, query, capability := productAgentAttemptRecoveryLeaseFixture(t, "attach")
	registry := newProductActiveAttemptRegistry()
	session := &productRecoveredAttemptSessionFixture{
		runtimeInstanceID: capability.RuntimeInstanceID,
		sessionDigest:     capability.SessionBindingDigest,
		closeCheck: func() error {
			if _, err := registry.Resolve(query); !errors.Is(err, errProductActiveAttemptNotFound) {
				return errors.New("active registration remained during session close")
			}
			return nil
		},
	}
	recovery, err := newProductAgentAttemptRecoveryRuntime(
		registry, &productAttemptReattacherFixture{session: session},
	)
	if err != nil {
		t.Fatal(err)
	}
	attachment, active, err := recovery.Attach(ctx, lease)
	if err != nil {
		t.Fatal(err)
	}
	if active.Identity != query || active.IncidentID != "cccccccc-cccc-4ccc-8ccc-cccccccccccc" ||
		active.CapsuleDigest == "" || active.ExecutionBinding.BindingDigest == "" {
		t.Fatalf("recovered active Attempt = %#v", active)
	}
	resolved, err := registry.Resolve(query)
	if err != nil || resolved.IncidentID != active.IncidentID {
		t.Fatalf("resolved recovered Attempt = %#v, %v", resolved, err)
	}

	const closers = 8
	errs := make(chan error, closers)
	var wait sync.WaitGroup
	for range closers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs <- attachment.Close()
		}()
	}
	wait.Wait()
	close(errs)
	for closeErr := range errs {
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	if session.CloseCalls() != 1 {
		t.Fatalf("session close calls = %d", session.CloseCalls())
	}
	if _, err := registry.Resolve(query); !errors.Is(err, errProductActiveAttemptNotFound) {
		t.Fatalf("resolve after recovered close = %v", err)
	}
}

func TestProductAgentAttemptRecoveryRejectsSessionIdentityAndRollsBackRegistryFailure(t *testing.T) {
	t.Run("session identity", func(t *testing.T) {
		ctx, lease, query, capability := productAgentAttemptRecoveryLeaseFixture(t, "identity")
		registry := newProductActiveAttemptRegistry()
		session := &productRecoveredAttemptSessionFixture{
			runtimeInstanceID: capability.RuntimeInstanceID,
			sessionDigest:     strings.Repeat("9", 64),
		}
		recovery, err := newProductAgentAttemptRecoveryRuntime(
			registry, &productAttemptReattacherFixture{session: session},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := recovery.Attach(ctx, lease); !errors.Is(err, errProductInvalidAttemptRecoveryAttachment) {
			t.Fatalf("identity substitution error = %v", err)
		}
		if session.CloseCalls() != 1 {
			t.Fatalf("identity session close calls = %d", session.CloseCalls())
		}
		if _, err := registry.Resolve(query); !errors.Is(err, errProductActiveAttemptNotFound) {
			t.Fatalf("identity failure registered Attempt: %v", err)
		}
	})

	t.Run("registry conflict", func(t *testing.T) {
		ctx, lease, query, capability := productAgentAttemptRecoveryLeaseFixture(t, "conflict")
		grant, err := lease.Take()
		if err != nil {
			t.Fatal(err)
		}
		registry := newProductActiveAttemptRegistry()
		registration, _, err := registry.RegisterRecovered(grant)
		if err != nil {
			t.Fatal(err)
		}
		defer registration.Close()
		session := &productRecoveredAttemptSessionFixture{
			runtimeInstanceID: capability.RuntimeInstanceID,
			sessionDigest:     capability.SessionBindingDigest,
		}
		recovery, err := newProductAgentAttemptRecoveryRuntime(
			registry, &productAttemptReattacherFixture{session: session},
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := recovery.attachGrant(ctx, grant); !errors.Is(err, errProductActiveAttemptConflict) {
			t.Fatalf("registry conflict error = %v", err)
		}
		if session.CloseCalls() != 1 {
			t.Fatalf("conflicted session close calls = %d", session.CloseCalls())
		}
		if _, err := registry.Resolve(query); err != nil {
			t.Fatalf("original registration lost = %v", err)
		}
	})
}

type productAttemptReattacherFixture struct {
	session productAgentAttemptRecoverySession
	err     error
}

func (fixture *productAttemptReattacherFixture) ReattachAgentAttempt(
	_ context.Context,
	_ work.AgentAttemptRecoveryDispatchGrant,
) (productAgentAttemptRecoverySession, error) {
	return fixture.session, fixture.err
}

type productRecoveredAttemptSessionFixture struct {
	mu                sync.Mutex
	runtimeInstanceID string
	sessionDigest     string
	closeCheck        func() error
	closeCalls        int
}

func (fixture *productRecoveredAttemptSessionFixture) RuntimeInstanceID() string {
	return fixture.runtimeInstanceID
}

func (fixture *productRecoveredAttemptSessionFixture) SessionBindingDigest() string {
	return fixture.sessionDigest
}

func (fixture *productRecoveredAttemptSessionFixture) Close() error {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.closeCalls++
	if fixture.closeCheck != nil {
		return fixture.closeCheck()
	}
	return nil
}

func (fixture *productRecoveredAttemptSessionFixture) CloseCalls() int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.closeCalls
}

type productAttemptRestartCapabilityResolverFixture struct {
	capability work.AgentAttemptRestartCapability
}

func (fixture productAttemptRestartCapabilityResolverFixture) ResolveAgentAttemptRestartCapability(
	_ context.Context,
	_ work.AgentAttemptRestartOutcome,
) (work.AgentAttemptRestartCapability, error) {
	return fixture.capability, nil
}

func productAgentAttemptRecoveryLeaseFixture(
	t *testing.T,
	suffix string,
) (context.Context, *work.AgentAttemptRecoveryDispatchLease, productActiveAttemptQuery, work.AgentAttemptRestartCapability) {
	return productAgentAttemptRecoveryLeaseFixtureWithResolver(t, suffix, nil, nil)
}

func productAgentAttemptRecoveryLeaseFixtureWithResolver(
	t *testing.T,
	suffix string,
	resolverFactory func(work.AgentAttemptRestartOutcome) work.AgentAttemptRestartCapabilityResolver,
	capture func(
		*work.AgentInboxCoordinator,
		*work.AttemptLoopAuthority,
		*journal.Store,
		work.AgentAttemptRestartOutcome,
	),
) (context.Context, *work.AgentAttemptRecoveryDispatchLease, productActiveAttemptQuery, work.AgentAttemptRestartCapability) {
	t.Helper()
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	inboxAuthority, err := work.NewAgentInboxAuthority(loops)
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := work.NewAgentInboxCoordinator(inboxAuthority, newProductMemoryAgentInboxStore())
	if err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	binding, budget, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	budget.MaxTurns = productAttemptLoopMaxInputTurns
	budget.MaxStepsPerTurn = productAttemptLoopMaxInputSteps
	if _, err := loops.StartTurn(ctx, binding, budget, work.AttemptLoopTurnInput{
		TurnID: "turn-1", Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, work.AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1", RequestID: "request-1",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	checkpoint := sha256.Sum256([]byte(productRecoveryCheckpointContent))
	if _, err := loops.EndStep(ctx, binding, work.AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: work.AttemptStepFinal,
		OutputDigest: hex.EncodeToString(checkpoint[:]),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndTurn(ctx, binding, work.AttemptLoopTurnEndInput{
		TurnID: "turn-1", Outcome: work.AttemptTurnSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
	content := []byte("private recovered input " + suffix)
	digest := sha256.Sum256(content)
	input := agentinbox.Payload{
		Binding: agentinbox.Binding{
			PayloadID: "recovery-payload-" + suffix, InputID: "recovery-input-" + suffix,
			Mode: agentinbox.ModeQueue, ContextScope: agentinbox.ScopeConversationShared,
			ConversationID: capsule.ConversationID, SegmentID: request.RouteSegment.SegmentID,
			AgentInstanceID: run.AgentInstanceID(), WorkItemID: run.WorkItemID(), RunID: run.ID(),
			ClaimGeneration: run.ClaimGeneration(), RuntimeInstanceID: run.RuntimeInstanceID(),
			ExecutionBindingDigest: executionBinding.BindingDigest,
			CapsuleDigest:          capsule.CapsuleDigest, OrderKey: 1,
			TargetTurnID: "turn-2", TargetTurnSequence: 2,
			ContentType: "text/plain", ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status:  agentinbox.StatusPending,
		Content: append([]byte(nil), content...),
	}
	if _, err := inbox.Admit(ctx, binding, input); err != nil {
		t.Fatalf("admit recovery input: %v", err)
	}
	if _, err := inbox.StartQueuedTurn(ctx, binding, input.Binding, work.AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: input.Binding.ContentDigest,
	}); err != nil {
		t.Fatalf("start recovery turn: %v", err)
	}
	report, err := inbox.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("restart report = %#v, %v", report, err)
	}
	outcome := report.Outcomes[0]
	if capture != nil {
		capture(inbox, loops, journalStore, outcome)
	}
	var resolver work.AgentAttemptRestartCapabilityResolver
	if resolverFactory != nil {
		resolver = resolverFactory(outcome)
	} else {
		capability := work.AgentAttemptRestartCapability{
			SchemaVersion: 1, AttemptID: outcome.Binding.AttemptID,
			RuntimeInstanceID:      outcome.Binding.PayloadAuthority.RuntimeInstanceID,
			ExecutionBindingDigest: outcome.Binding.PayloadAuthority.ExecutionBindingDigest,
			ContextCapsuleDigest:   outcome.Binding.PayloadAuthority.CapsuleDigest,
			ResumeMode:             work.AgentAttemptRestartResumeFromCheckpoint,
			SessionBindingDigest:   strings.Repeat("8", 64),
		}
		capability.CapabilityDigest = work.AgentAttemptRestartCapabilityDigest(capability)
		resolver = productAttemptRestartCapabilityResolverFixture{capability: capability}
	}
	capability, err := resolver.ResolveAgentAttemptRestartCapability(ctx, outcome)
	if err != nil {
		t.Fatal(err)
	}
	recoveryAuthority, err := work.NewAgentAttemptRecoveryAuthority(
		inbox, resolver,
		func() time.Time { return time.Date(2026, 8, 14, 19, 0, 0, 0, time.UTC) },
		bytes.NewReader(bytes.Repeat([]byte{0x51}, 128)),
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := recoveryAuthority.Authorize(ctx, work.AgentAttemptRecoveryDecisionInput{
		SchemaVersion: 1, DecisionID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		CorrelationID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		PrincipalID:   "local-user", Action: work.AgentAttemptRecoveryResumePreModel,
		CandidateDigest:  work.AgentAttemptRestartCandidateDigest(outcome),
		CapabilityDigest: capability.CapabilityDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := recoveryAuthority.Consume(ctx, work.AgentAttemptRecoveryConsumeInput{
		SchemaVersion: 1, CorrelationID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		DecisionID: decision.DecisionID, CandidateDigest: decision.CandidateDigest,
		CapabilityDigest: decision.CapabilityDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = payloadStore
	return ctx, lease, productActiveAttemptQuery{
		ConversationID: capsule.ConversationID, SegmentID: request.RouteSegment.SegmentID,
		AgentInstanceID: run.AgentInstanceID(), WorkItemID: run.WorkItemID(), RunID: run.ID(),
		ClaimGeneration: run.ClaimGeneration(),
	}, capability
}
