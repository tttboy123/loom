package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestProductAttemptRecoveryCompletionCommitsValidatedResultAccountingAndClosesGrant(t *testing.T) {
	ctx, lease, query, _ := productAgentAttemptRecoveryLeaseFixture(t, "completion")
	registry := newProductActiveAttemptRegistry()
	session := &productRecoveryCompletionSessionFixture{}
	runtime, err := newProductAgentAttemptRecoveryRuntime(
		registry,
		&productRecoveryCompletionReattacherFixture{session: session},
	)
	if err != nil {
		t.Fatal(err)
	}
	runs := &productRecoveryRunTerminalFixture{}
	grants := &productRecoveryGrantClosureFixture{}
	completion, err := newProductAgentAttemptRecoveryCompletion(
		runtime, runs, grants, nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := completion.Resume(ctx, lease)
	if err != nil {
		t.Fatal(err)
	}
	accounting, available := result.Accounting()
	if !available || accounting.TotalTokens != 22 || accounting.CostSource != work.CostSourceProviderReported {
		t.Fatalf("accounting = %#v, %t", accounting, available)
	}
	if runs.calls != 1 || runs.input.Status != "succeeded" || runs.input.Reason != "" ||
		runs.input.Accounting == nil || *runs.input.Accounting != accounting {
		t.Fatalf("terminal input = %#v (calls=%d)", runs.input, runs.calls)
	}
	if grants.resolveCalls != 1 || grants.revokeCalls != 1 ||
		grants.revokeReason != authorization.RevocationTerminal ||
		grants.revokeCorrelation != "cccccccc-cccc-4ccc-8ccc-cccccccccccc" {
		t.Fatalf("grant closure = %#v", grants)
	}
	if _, err := registry.Resolve(query); !errors.Is(err, errProductActiveAttemptNotFound) {
		t.Fatalf("completed recovery remained active: %v", err)
	}
}

func TestProductAttemptRecoveryCompletionRejectsSubstitutedFrameBeforeTerminalAuthority(t *testing.T) {
	ctx, lease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "substituted-frame")
	registry := newProductActiveAttemptRegistry()
	session := &productRecoveryCompletionSessionFixture{substituteCorrelation: true}
	runtime, err := newProductAgentAttemptRecoveryRuntime(
		registry,
		&productRecoveryCompletionReattacherFixture{session: session},
	)
	if err != nil {
		t.Fatal(err)
	}
	runs := &productRecoveryRunTerminalFixture{}
	grants := &productRecoveryGrantClosureFixture{}
	completion, err := newProductAgentAttemptRecoveryCompletion(runtime, runs, grants, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := completion.Resume(ctx, lease); !errors.Is(err, supervisor.ErrBridgeSession) {
		t.Fatalf("substituted frame error = %v", err)
	}
	if runs.calls != 0 || grants.revokeCalls != 0 {
		t.Fatalf("substitution reached terminal authority: runs=%d revokes=%d", runs.calls, grants.revokeCalls)
	}
}

func TestProductAttemptRecoveryCompletionRejectsAdapterResultMismatchAndGrantAmbiguity(t *testing.T) {
	t.Run("adapter result mismatch", func(t *testing.T) {
		ctx, lease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "result-mismatch")
		runtime, err := newProductAgentAttemptRecoveryRuntime(
			newProductActiveAttemptRegistry(),
			&productRecoveryCompletionReattacherFixture{
				session: &productRecoveryCompletionSessionFixture{mismatchResult: true},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		runs := &productRecoveryRunTerminalFixture{}
		grants := &productRecoveryGrantClosureFixture{}
		completion, err := newProductAgentAttemptRecoveryCompletion(runtime, runs, grants, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := completion.Resume(ctx, lease); !errors.Is(err, supervisor.ErrBridgeSession) {
			t.Fatalf("result mismatch error = %v", err)
		}
		if runs.calls != 0 || grants.revokeCalls != 0 {
			t.Fatalf("result mismatch reached terminal authority: runs=%d revokes=%d", runs.calls, grants.revokeCalls)
		}
	})

	t.Run("observer rejection", func(t *testing.T) {
		ctx, lease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "observer-rejection")
		runtime, err := newProductAgentAttemptRecoveryRuntime(
			newProductActiveAttemptRegistry(),
			&productRecoveryCompletionReattacherFixture{
				session: &productRecoveryCompletionSessionFixture{},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		runs := &productRecoveryRunTerminalFixture{}
		grants := &productRecoveryGrantClosureFixture{}
		observer := &productRecoveryFrameObserverFixture{err: errors.New("observer rejected frame")}
		completion, err := newProductAgentAttemptRecoveryCompletion(
			runtime, runs, grants, observer,
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := completion.Resume(ctx, lease); !errors.Is(
			err, supervisor.ErrAuthorizedFrameObserver,
		) {
			t.Fatalf("observer rejection error = %v", err)
		}
		if observer.calls != 1 || runs.calls != 0 || grants.revokeCalls != 0 {
			t.Fatalf("observer rejection escaped authority: observer=%d runs=%d revokes=%d",
				observer.calls, runs.calls, grants.revokeCalls)
		}
	})

	t.Run("ambiguous original grant", func(t *testing.T) {
		ctx, lease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "grant-ambiguity")
		reattacher := &productRecoveryCompletionReattacherFixture{
			session: &productRecoveryCompletionSessionFixture{},
		}
		runtime, err := newProductAgentAttemptRecoveryRuntime(
			newProductActiveAttemptRegistry(), reattacher,
		)
		if err != nil {
			t.Fatal(err)
		}
		runs := &productRecoveryRunTerminalFixture{}
		grants := &productRecoveryGrantClosureFixture{resolveErr: errProductAttemptRecoveryGrantConflict}
		completion, err := newProductAgentAttemptRecoveryCompletion(runtime, runs, grants, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := completion.Resume(ctx, lease); !errors.Is(err, errProductAttemptRecoveryGrantConflict) {
			t.Fatalf("ambiguous grant error = %v", err)
		}
		if reattacher.calls != 0 || runs.calls != 0 || grants.revokeCalls != 0 {
			t.Fatalf("ambiguous grant escaped preflight: reattach=%d runs=%d revokes=%d",
				reattacher.calls, runs.calls, grants.revokeCalls)
		}
	})
}

type productRecoveryFrameObserverFixture struct {
	calls int
	err   error
}

func (fixture *productRecoveryFrameObserverFixture) ObserveAuthorizedFrame(
	_ context.Context,
	_ supervisor.AuthorizedFrame,
) error {
	fixture.calls++
	return fixture.err
}

func TestProductAuthorizationRecoveryGrantClosureResolvesExactUnrevokedGrant(t *testing.T) {
	ctx, lease, _, _ := productAgentAttemptRecoveryLeaseFixture(t, "grant-closure")
	recoveryGrant, err := lease.Take()
	if err != nil {
		t.Fatal(err)
	}
	outcome := recoveryGrant.Outcome()
	authority := outcome.Binding.PayloadAuthority

	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	appendProductExecutionRuntimeFixture(t, database)
	journalStore := journal.NewStore(database)
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	runs, err := work.NewAuthority(
		journalStore, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x52}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runs.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runs.CreateAndAssign(ctx, work.WorkItemAssignmentInput{
		WorkItemID: authority.WorkItemID, Title: "Resolve original recovery grant",
		RunID: authority.RunID, AgentInstanceID: authority.AgentInstanceID,
		ExecutionBinding: outcome.ExecutionBinding,
		CorrelationID:    "11111111-1111-4111-8111-111111111111",
	}); err != nil {
		t.Fatal(err)
	}
	_, claimed, err := runs.Claim(ctx, work.RunClaimInput{
		WorkItemID: authority.WorkItemID, RunID: authority.RunID,
		RuntimeInstanceID:    authority.RuntimeInstanceID,
		AgentInstanceID:      authority.AgentInstanceID,
		PrepareLeaseDuration: time.Minute,
		CorrelationID:        "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ClaimID() != authority.ClaimID ||
		claimed.ClaimGeneration() != authority.ClaimGeneration {
		t.Fatalf("claim fixture drifted: %#v", claimed)
	}
	grants, err := authorization.NewAuthority(
		journalStore, runs, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x61}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grants.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	issued, err := grants.Issue(ctx, authorization.IssueInput{
		WorkItemID: authority.WorkItemID, RunID: authority.RunID,
		ClaimID: authority.ClaimID, ClaimGeneration: authority.ClaimGeneration,
		RuntimeInstanceID: authority.RuntimeInstanceID,
		AgentInstanceID:   authority.AgentInstanceID,
		AllowedOperations: []authorization.Operation{
			authorization.OperationBridgeAck,
			authorization.OperationBridgeEvent,
			authorization.OperationBridgeResult,
		},
		Lifetime: time.Minute, CorrelationID: "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runs.Start(ctx, work.RunGenerationInput{
		WorkItemID: authority.WorkItemID, RunID: authority.RunID,
		ClaimID: authority.ClaimID, ClaimGeneration: authority.ClaimGeneration,
		RuntimeInstanceID: authority.RuntimeInstanceID,
		AgentInstanceID:   authority.AgentInstanceID,
		CorrelationID:     "11111111-1111-4111-8111-111111111111",
	}); err != nil {
		t.Fatal(err)
	}
	runTerminal, err := newProductWorkRecoveryTerminalAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	resolvedAccounting, err := runTerminal.ResolveRecoveryAccounting(
		ctx, outcome, &work.RunAccounting{
			UsageObserved: true, InputTokens: 18, OutputTokens: 4, TotalTokens: 22,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedAccounting == nil || resolvedAccounting.CostObserved ||
		resolvedAccounting.TotalTokens != 22 {
		t.Fatalf("resolved recovery accounting = %#v", resolvedAccounting)
	}
	closure, err := newProductAuthorizationRecoveryGrantClosure(grants)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := closure.ResolveOriginalGrant(ctx, outcome)
	if err != nil {
		t.Fatal(err)
	}
	if binding.GrantID != issued.Record().ID() || binding.RunID != authority.RunID ||
		binding.ClaimGeneration != authority.ClaimGeneration ||
		len(binding.AllowedOperations) != 3 {
		t.Fatalf("resolved original grant = %#v", binding)
	}
	if _, _, err := runTerminal.CommitTerminal(ctx, work.RunTerminalInput{
		RunGenerationInput: work.RunGenerationInput{
			WorkItemID: authority.WorkItemID, RunID: authority.RunID,
			ClaimID: authority.ClaimID, ClaimGeneration: authority.ClaimGeneration,
			RuntimeInstanceID: authority.RuntimeInstanceID,
			AgentInstanceID:   authority.AgentInstanceID,
			CorrelationID:     recoveryGrant.IncidentID(),
		},
		Status: "succeeded", Accounting: resolvedAccounting,
	}); err != nil {
		t.Fatal(err)
	}
	reconciler, err := newProductAttemptRecoveryTerminalReconciler(runs, grants)
	if err != nil {
		t.Fatal(err)
	}
	report, err := reconciler.Reconcile(ctx, recoveryGrant.IncidentID())
	if err != nil {
		t.Fatal(err)
	}
	if report.TerminalRuns != 1 || report.RevokedGrants != 1 || len(report.Outcomes) != 1 {
		t.Fatalf("reconciliation report = %#v", report)
	}
	closed := report.Outcomes[0]
	if closed.WorkItemID != authority.WorkItemID || closed.RunID != authority.RunID ||
		closed.ClaimGeneration != authority.ClaimGeneration ||
		closed.RuntimeInstanceID != authority.RuntimeInstanceID ||
		closed.AgentInstanceID != authority.AgentInstanceID ||
		closed.ExecutionBindingDigest != outcome.ExecutionBinding.BindingDigest ||
		closed.ProviderID != outcome.ExecutionBinding.ProviderID ||
		closed.ProviderAccountID != outcome.ExecutionBinding.ProviderAccountID ||
		closed.ModelID != outcome.ExecutionBinding.ModelID {
		t.Fatalf("reconciliation outcome = %#v", closed)
	}
	repeated, err := reconciler.Reconcile(ctx, recoveryGrant.IncidentID())
	if err != nil || repeated.TerminalRuns != 1 || repeated.RevokedGrants != 0 ||
		len(repeated.Outcomes) != 0 {
		t.Fatalf("repeated reconciliation = %#v, %v", repeated, err)
	}
	if _, err := closure.ResolveOriginalGrant(ctx, outcome); !errors.Is(
		err, errProductAttemptRecoveryGrantConflict,
	) {
		t.Fatalf("revoked grant resolution error = %v", err)
	}
}

type productRecoveryCompletionReattacherFixture struct {
	session *productRecoveryCompletionSessionFixture
	calls   int
}

func (fixture *productRecoveryCompletionReattacherFixture) ReattachAgentAttempt(
	_ context.Context,
	grant work.AgentAttemptRecoveryDispatchGrant,
) (productAgentAttemptRecoverySession, error) {
	fixture.calls++
	fixture.session.grant = grant
	fixture.session.outcome = grant.Outcome()
	fixture.session.capability = grant.Capability()
	return fixture.session, nil
}

type productRecoveryCompletionSessionFixture struct {
	grant                 work.AgentAttemptRecoveryDispatchGrant
	outcome               work.AgentAttemptRestartOutcome
	capability            work.AgentAttemptRestartCapability
	substituteCorrelation bool
	mismatchResult        bool
}

func (fixture *productRecoveryCompletionSessionFixture) RuntimeInstanceID() string {
	return fixture.capability.RuntimeInstanceID
}

func (fixture *productRecoveryCompletionSessionFixture) SessionBindingDigest() string {
	return fixture.capability.SessionBindingDigest
}

func (*productRecoveryCompletionSessionFixture) Close() error { return nil }

func (fixture *productRecoveryCompletionSessionFixture) ContinueAgentAttempt(
	ctx context.Context,
	sink supervisor.FrameSink,
) (supervisor.AdapterResult, error) {
	dispatchID := productDeterministicUUID(
		"agent-attempt-recovery-dispatch", fixture.grant.LeaseID(),
	)
	correlationID := fixture.grant.IncidentID()
	if fixture.substituteCorrelation {
		correlationID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	}
	frames := []bridgev1.Frame{
		productRecoveryCompletionFrame(fixture.outcome, correlationID,
			2, bridgev1.MessageAck, mustProductRecoveryJSON(map[string]string{"message_id": dispatchID})),
		productRecoveryCompletionFrame(fixture.outcome, correlationID,
			3, bridgev1.MessageEvent, []byte(`{"delta":"recovered"}`)),
		productRecoveryCompletionFrame(fixture.outcome, correlationID,
			4, bridgev1.MessageResult, []byte(`{"reason":"","status":"succeeded"}`)),
	}
	for _, frame := range frames {
		if err := sink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	resultFrames := frames
	if fixture.mismatchResult {
		resultFrames = frames[:2]
	}
	accounting := work.RunAccounting{
		UsageObserved: true, InputTokens: 18, OutputTokens: 4, TotalTokens: 22,
		CostObserved: true, CostMicrounits: 9, CostCurrency: "USD",
		CostSource: work.CostSourceProviderReported,
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames: resultFrames, ExitCode: 0,
		DispatchAcknowledged: true, ResultAcknowledged: true,
		Accounting: &accounting,
	})
}

func productRecoveryCompletionFrame(
	outcome work.AgentAttemptRestartOutcome,
	correlationID string,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	authority := outcome.Binding.PayloadAuthority
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: productDeterministicUUID(
			"agent-attempt-recovery-frame", string(messageType), correlationID,
		),
		CorrelationID: correlationID, WorkItemID: authority.WorkItemID,
		RunID: authority.RunID, ClaimGeneration: authority.ClaimGeneration,
		RuntimeInstanceID:     authority.RuntimeInstanceID,
		SenderAgentInstanceID: authority.AgentInstanceID,
		Sequence:              sequence, Type: messageType,
		EmittedAt: time.Date(2026, 8, 14, 21, 0, int(sequence), 0, time.UTC),
		Payload:   payload,
	})
	if err != nil {
		panic(err)
	}
	return frame
}

func mustProductRecoveryJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

type productRecoveryRunTerminalFixture struct {
	calls int
	input work.RunTerminalInput
}

func (*productRecoveryRunTerminalFixture) ResolveRecoveryAccounting(
	_ context.Context,
	_ work.AgentAttemptRestartOutcome,
	accounting *work.RunAccounting,
) (*work.RunAccounting, error) {
	if accounting == nil {
		return nil, nil
	}
	clone := *accounting
	return &clone, nil
}

func (fixture *productRecoveryRunTerminalFixture) CommitTerminal(
	_ context.Context,
	input work.RunTerminalInput,
) (work.WorkItemRecord, work.RunRecord, error) {
	fixture.calls++
	fixture.input = input
	return work.WorkItemRecord{}, work.RunRecord{}, nil
}

type productRecoveryGrantClosureFixture struct {
	resolveCalls      int
	resolveErr        error
	revokeCalls       int
	revokeReason      authorization.RevocationReason
	revokeCorrelation string
}

func (fixture *productRecoveryGrantClosureFixture) ResolveOriginalGrant(
	_ context.Context,
	outcome work.AgentAttemptRestartOutcome,
) (supervisor.RecoveryGrantBinding, error) {
	fixture.resolveCalls++
	if fixture.resolveErr != nil {
		return supervisor.RecoveryGrantBinding{}, fixture.resolveErr
	}
	authority := outcome.Binding.PayloadAuthority
	return supervisor.RecoveryGrantBinding{
		GrantID: "original-grant", WorkItemID: authority.WorkItemID,
		RunID: authority.RunID, ClaimID: authority.ClaimID,
		ClaimGeneration:   authority.ClaimGeneration,
		RuntimeInstanceID: authority.RuntimeInstanceID,
		AgentInstanceID:   authority.AgentInstanceID,
		AllowedOperations: []authorization.Operation{
			authorization.OperationBridgeAck,
			authorization.OperationBridgeEvent,
			authorization.OperationBridgeEvidence,
			authorization.OperationBridgeHeartbeat,
			authorization.OperationBridgeResult,
		},
	}, nil
}

func (fixture *productRecoveryGrantClosureFixture) RevokeOriginalGrant(
	_ context.Context,
	binding supervisor.RecoveryGrantBinding,
	reason authorization.RevocationReason,
	correlationID string,
) error {
	fixture.revokeCalls++
	fixture.revokeReason = reason
	fixture.revokeCorrelation = correlationID
	if binding.GrantID == "" {
		return errProductAttemptRecoveryGrantConflict
	}
	return nil
}
