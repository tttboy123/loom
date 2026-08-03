package work

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
)

func TestSideTaskAdmissionIsExplicitOnlyIdempotentAndPolicyZeroWrite(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x61)
	input := sideTaskAdmissionFixture()

	unconfirmed := input
	unconfirmed.Confirmed = false
	if _, err := authority.AdmitSideTask(context.Background(), unconfirmed); !errors.Is(
		err, ErrSideTaskConfirmation,
	) {
		t.Fatalf("unconfirmed error = %v", err)
	}
	policy := input
	policy.Confirmed = false
	policy.PolicyStreamID = "rule-set/standing"
	policy.PolicyVersion = 1
	policy.PolicyDigest = strings.Repeat("f", 64)
	if _, err := authority.AdmitSideTask(context.Background(), policy); !errors.Is(
		err, ErrSideTaskCapabilityGap,
	) {
		t.Fatalf("policy error = %v", err)
	}
	before, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Type != "WorkRunIdentityIndexInitialized" {
		t.Fatalf("rejected admission wrote events: %#v", before)
	}

	first, err := authority.AdmitSideTask(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := authority.AdmitSideTask(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if first.SideTaskID != input.SideTaskID || first.Status != "admitted" ||
		first.StreamSequence != 1 || replayed.LastEventID != first.LastEventID {
		t.Fatalf("first=%#v replayed=%#v", first, replayed)
	}
	after, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	admitted := 0
	for _, event := range after {
		if event.Type == "SideTaskAdmitted" {
			admitted++
		}
	}
	if len(after) != len(before)+1 || admitted != 1 {
		t.Fatalf("admission events = %#v", after)
	}
}

func TestSideTaskDecisionCASHasOneWinnerAndAbsorbIsOneAtomicParentEffect(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x71)
	admission := sideTaskAdmissionFixture()
	parent := acceptedSideTaskHandoffFixture(t, store, authority, SideTaskAdmissionInput{
		SideExecutionTeamInstanceID: admission.ParentTeamInstanceID,
	})
	admission.ParentTaskID = parent.SourceWorkItemID
	admission.ParentRunID = parent.SourceRunID
	admission.ParentClaimGeneration = parent.SourceGeneration
	if _, err := authority.AdmitSideTask(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	handoff := acceptedSideTaskHandoffFixture(t, store, authority, admission)
	committed, err := authority.CommitSideTaskHandoff(context.Background(), handoff)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Status != "decision_required" ||
		!committed.DecisionDeadline.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("handoff = %#v", committed)
	}
	differentTimeout := handoff
	differentTimeout.DecisionTimeout = 2 * time.Hour
	if _, err := authority.CommitSideTaskHandoff(
		context.Background(), differentTimeout,
	); !errors.Is(err, ErrSideTaskConflict) {
		t.Fatalf("mismatched timeout replay error = %v", err)
	}

	absorb := sideTaskDecisionFixture(admission, handoff, "absorb")
	beforeSubstitution, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	substituted := absorb
	substituted.ParentExecutionDigest = strings.Repeat("f", 64)
	substituted.EffectDigest = sideTaskParentEffectDigest(substituted)
	if _, err := authority.DecideSideTaskHandoff(
		context.Background(), substituted,
	); !errors.Is(err, ErrSideTaskConflict) {
		t.Fatalf("substituted parent execution digest error = %v", err)
	}
	afterSubstitution, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(afterSubstitution) != len(beforeSubstitution) {
		t.Fatalf("substituted parent execution digest wrote events: before=%d after=%d",
			len(beforeSubstitution), len(afterSubstitution))
	}
	discard := sideTaskDecisionFixture(admission, handoff, "discard")
	discard.DecisionID = "decision-discard-1"
	discard.ContextPacketID = ""
	discard.ContextPacketVersion = 0
	discard.ContextPacketDigest = ""
	discard.ContinuationExecutionTeamInstanceID = ""
	discard.ContinuationPlanDigest = ""
	discard.EffectDigest = sideTaskParentEffectDigest(discard)

	start := make(chan struct{})
	errorsByDecision := make([]error, 2)
	var wait sync.WaitGroup
	for index, decision := range []SideTaskDecisionInput{absorb, discard} {
		wait.Add(1)
		go func(index int, decision SideTaskDecisionInput) {
			defer wait.Done()
			<-start
			_, errorsByDecision[index] = authority.DecideSideTaskHandoff(
				context.Background(), decision,
			)
		}(index, decision)
	}
	close(start)
	wait.Wait()
	successes := 0
	conflicts := 0
	for _, decisionErr := range errorsByDecision {
		if decisionErr == nil {
			successes++
		} else if errors.Is(decisionErr, ErrSideTaskConflict) {
			conflicts++
		} else {
			t.Fatalf("decision error = %v", decisionErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("decision results = %#v", errorsByDecision)
	}

	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	decisionCount := 0
	packetCount := 0
	continuationCount := 0
	for _, event := range events {
		switch event.Type {
		case "SideTaskDecisionCommitted":
			decisionCount++
		case "ContextPacketCommitted":
			packetCount++
		case "ParentContinuationAuthorized":
			continuationCount++
		}
	}
	if decisionCount != 1 || packetCount != continuationCount || packetCount > 1 {
		t.Fatalf("decision=%d packet=%d continuation=%d", decisionCount, packetCount, continuationCount)
	}
}

func TestParentHandoffCompletionValidatesTerminalLineageAndExactReplay(t *testing.T) {
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0x73)
	admission := sideTaskAdmissionFixture()
	parent := acceptedSideTaskHandoffFixture(t, store, authority, SideTaskAdmissionInput{
		SideExecutionTeamInstanceID: admission.ParentTeamInstanceID,
	})
	admission.ParentTaskID = parent.SourceWorkItemID
	admission.ParentRunID = parent.SourceRunID
	admission.ParentClaimGeneration = parent.SourceGeneration
	if _, err := authority.AdmitSideTask(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	handoff := acceptedSideTaskHandoffFixture(t, store, authority, admission)
	if _, err := authority.CommitSideTaskHandoff(context.Background(), handoff); err != nil {
		t.Fatal(err)
	}
	decision := sideTaskDecisionFixture(admission, handoff, "continue")
	decision.ContextPacketID = ""
	decision.ContextPacketVersion = 0
	decision.ContextPacketDigest = ""
	terminal := acceptedSideTaskHandoffFixture(t, store, authority, SideTaskAdmissionInput{
		SideExecutionTeamInstanceID: decision.ContinuationExecutionTeamInstanceID,
	})
	team, err := authority.TeamExecution(context.Background(), decision.ContinuationExecutionTeamInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	decision.ContinuationPlanDigest = team.PlanDigest()
	if _, err := authority.DecideSideTaskHandoff(context.Background(), decision); err != nil {
		t.Fatal(err)
	}
	completion := ParentHandoffEffectCompletionInput{
		DecisionID: decision.DecisionID, SideTaskID: decision.SideTaskID,
		EffectKind: "continuation", EffectDigest: decision.EffectDigest,
		ExecutionTeamInstanceID: decision.ContinuationExecutionTeamInstanceID,
		ExecutionPlanDigest:     team.PlanDigest(), TerminalStatus: "succeeded",
		TerminalEvidenceID:     terminal.SourceEvidenceID,
		TerminalEvidenceDigest: terminal.SourceEvidenceDigest,
		ExpectedViewVersion:    testDigest, CorrelationID: testCorrelation,
	}
	nonAuthorizedTerminal := acceptedSideTaskHandoffFixture(t, store, authority,
		SideTaskAdmissionInput{SideExecutionTeamInstanceID: "team-non-authorized-1"})
	nonAuthorizedTeam, err := authority.TeamExecution(
		context.Background(), "team-non-authorized-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	nonAuthorized := completion
	nonAuthorized.ExecutionTeamInstanceID = "team-non-authorized-1"
	nonAuthorized.ExecutionPlanDigest = nonAuthorizedTeam.PlanDigest()
	nonAuthorized.TerminalEvidenceID = nonAuthorizedTerminal.SourceEvidenceID
	nonAuthorized.TerminalEvidenceDigest = nonAuthorizedTerminal.SourceEvidenceDigest
	beforeRejectedCompletion, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.CompleteParentHandoffEffect(
		context.Background(), nonAuthorized,
	); !errors.Is(err, ErrSideTaskConflict) {
		t.Fatalf("non-authorized completion error = %v", err)
	}
	afterRejectedCompletion, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRejectedCompletion) != len(beforeRejectedCompletion) {
		t.Fatalf("non-authorized completion wrote events: before=%d after=%d",
			len(beforeRejectedCompletion), len(afterRejectedCompletion))
	}
	if _, err := authority.CompleteParentHandoffEffect(context.Background(), completion); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.CompleteParentHandoffEffect(context.Background(), completion); err != nil {
		t.Fatalf("exact completion replay error = %v", err)
	}
	changed := completion
	changed.TerminalEvidenceDigest = strings.Repeat("f", 64)
	if _, err := authority.CompleteParentHandoffEffect(
		context.Background(), changed,
	); !errors.Is(err, ErrSideTaskConflict) {
		t.Fatalf("different completion replay error = %v", err)
	}
}

func TestSideTaskFollowupIsProposalOnlyAndNeedsNoConsumer(t *testing.T) {
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0x72)
	admission := sideTaskAdmissionFixture()
	parent := acceptedSideTaskHandoffFixture(t, store, authority, SideTaskAdmissionInput{
		SideExecutionTeamInstanceID: admission.ParentTeamInstanceID,
	})
	admission.ParentTaskID = parent.SourceWorkItemID
	admission.ParentRunID = parent.SourceRunID
	admission.ParentClaimGeneration = parent.SourceGeneration
	if _, err := authority.AdmitSideTask(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	handoff := acceptedSideTaskHandoffFixture(t, store, authority, admission)
	if _, err := authority.CommitSideTaskHandoff(context.Background(), handoff); err != nil {
		t.Fatal(err)
	}
	decision := sideTaskDecisionFixture(admission, handoff, "request_followup")
	decision.ContextPacketID = ""
	decision.ContextPacketVersion = 0
	decision.ContextPacketDigest = ""
	decision.ContinuationExecutionTeamInstanceID = ""
	decision.ContinuationPlanDigest = ""
	decision.FollowupProposalDigest = strings.Repeat("8", 64)
	decision.EffectDigest = sideTaskParentEffectDigest(decision)
	record, err := authority.DecideSideTaskHandoff(context.Background(), decision)
	if err != nil {
		t.Fatal(err)
	}
	if record.EffectStatus != "none" {
		t.Fatalf("effect status = %q", record.EffectStatus)
	}
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	followups := 0
	completions := 0
	for _, event := range events {
		if event.Type == "ParentFollowupProposed" {
			followups++
		}
		if event.Type == "ParentHandoffEffectCompleted" {
			completions++
		}
	}
	if followups != 1 || completions != 0 {
		t.Fatalf("followups=%d completions=%d", followups, completions)
	}
}

func sideTaskAdmissionFixture() SideTaskAdmissionInput {
	return SideTaskAdmissionInput{
		SideTaskID: "side-task-1", ParentMissionID: "mission-parent-1",
		ParentTeamInstanceID: "team-parent-1", ParentTaskID: "task-parent-1",
		ParentRunID: "run-parent-1", ParentClaimGeneration: 1,
		ParentExecutionDigest:       strings.Repeat("5", 64),
		SideExecutionTeamInstanceID: "team-side-1",
		Purpose:                     "research", Mode: "decision_required", Title: "Research bounded question",
		ProposalDigest: strings.Repeat("a", 64), InputArtifactDigest: strings.Repeat("b", 64),
		ExpectedViewVersion: testDigest, PermissionScopes: []string{"read:project"},
		Confirmed: true, BudgetMicrounits: 0, BudgetCurrency: "",
		CorrelationID: testCorrelation,
	}
}

func acceptedSideTaskHandoffFixture(
	t *testing.T,
	journalStore *journal.Store,
	authority *Authority,
	admission SideTaskAdmissionInput,
) SideTaskHandoffCommitInput {
	t.Helper()
	runtimeID := "runtime-" + admission.SideExecutionTeamInstanceID
	seedRuntime(t, journalStore, runtimeID, "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: admission.SideExecutionTeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Produce bounded Side-task evidence",
			AgentInstanceID:   "agent-" + admission.SideExecutionTeamInstanceID,
			RuntimeInstanceID: runtimeID,
			Role:              teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(t, journalStore, authority, plan, 1, testNow)
	run := dispatched.Run()
	generation := RunGenerationInput{
		WorkItemID: run.WorkItemID(), RunID: run.ID(), ClaimID: run.ClaimID(),
		ClaimGeneration: run.ClaimGeneration(), RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID: run.AgentInstanceID(), CorrelationID: testCorrelation,
	}
	if _, _, err := authority.Start(context.Background(), generation); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.CommitTerminal(context.Background(), RunTerminalInput{
		RunGenerationInput: generation, Status: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
	receipt, classification := testTeamAttemptReceiptAndClassification(
		t, plan, "main", 1, generation, "succeeded",
	)
	if _, err := authority.CommitTeamAttemptEvidence(context.Background(), TeamAttemptEvidenceInput{
		TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(),
		LogicalNodeID: "main", AttemptNumber: 1,
		WorkItemID: generation.WorkItemID, RunID: generation.RunID,
		ClaimID: generation.ClaimID, ClaimGeneration: generation.ClaimGeneration,
		RuntimeInstanceID: generation.RuntimeInstanceID,
		AgentInstanceID:   generation.AgentInstanceID,
		Receipt:           receipt, Classification: classification, CorrelationID: testCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	acceptance, err := verification.NewAcceptanceContract(
		1, []string{"controlled output is accepted"}, verification.AcceptanceRiskLow,
	)
	if err != nil {
		t.Fatal(err)
	}
	output, err := verification.NewOutputContract(1, verification.EmptyOutputInvalid)
	if err != nil {
		t.Fatal(err)
	}
	result, err := verification.VerifyDeterministic(acceptance,
		verification.DeterministicVerificationInput{
			TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(),
			LogicalNodeID: "main", AttemptNumber: 1,
			WorkItemID: generation.WorkItemID, RunID: generation.RunID,
			ClaimID: generation.ClaimID, ClaimGeneration: generation.ClaimGeneration,
			SourceEvidenceID: receipt.EvidenceID(), SourceEvidenceDigest: receipt.Digest(),
			OutputSummaryDigest:   receipt.OutputSummary().Digest(),
			OutputContractVersion: output.Version(), OutputContractDigest: output.Digest(),
			OutputClassification:       classification.Kind(),
			OutputClassificationDigest: classification.Digest(),
			AcceptanceContractDigest:   acceptance.Digest(), TerminalStatus: "succeeded",
		})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version: 1, RetryDelay: time.Minute, AttemptCredits: 1,
		ExhaustionAction: rules.ExhaustionBlocked, RetryInvalid: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.CommitTeamNodeAcceptance(context.Background(), TeamNodeAcceptanceInput{
		TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(),
		LogicalNodeID: "main", AttemptNumber: 1, SourceReceipt: receipt,
		AcceptanceContract: acceptance, DeterministicResult: result,
		RecoveryPolicy: policy, MaxAttempts: 2, CreditsBefore: 1,
		CorrelationID: testCorrelation,
	}); err != nil {
		t.Fatal(err)
	}
	return SideTaskHandoffCommitInput{
		SideTaskID:                  admission.SideTaskID,
		SideExecutionTeamInstanceID: admission.SideExecutionTeamInstanceID,
		SourceWorkItemID:            generation.WorkItemID, SourceRunID: generation.RunID,
		SourceGeneration: generation.ClaimGeneration,
		SourceEvidenceID: receipt.EvidenceID(), SourceEvidenceDigest: receipt.Digest(),
		HandoffVersion: 1, HandoffDigest: strings.Repeat("3", 64),
		SummaryArtifactDigest: strings.Repeat("4", 64), DecisionTimeout: time.Hour,
		ExpectedViewVersion: testDigest, CorrelationID: testCorrelation,
	}
}

func sideTaskDecisionFixture(admission SideTaskAdmissionInput,
	handoff SideTaskHandoffCommitInput, decision string) SideTaskDecisionInput {
	input := SideTaskDecisionInput{
		DecisionID: "decision-absorb-1", SideTaskID: admission.SideTaskID,
		ParentMissionID:      admission.ParentMissionID,
		ParentTeamInstanceID: admission.ParentTeamInstanceID,
		ParentTaskID:         admission.ParentTaskID, ParentRunID: admission.ParentRunID,
		ParentLogicalNodeID: "main", ParentAttemptNumber: 1,
		ParentClaimGeneration: admission.ParentClaimGeneration,
		ParentExecutionDigest: strings.Repeat("5", 64),
		SideTaskGeneration:    handoff.SourceGeneration,
		HandoffVersion:        handoff.HandoffVersion, HandoffDigest: handoff.HandoffDigest,
		Decision:        decision,
		ContextPacketID: "context-packet-1", ContextPacketVersion: 1,
		ContextPacketDigest:                 strings.Repeat("7", 64),
		ContinuationExecutionTeamInstanceID: "team-continuation-1",
		ContinuationPlanDigest:              strings.Repeat("8", 64),
		ExpectedViewVersion:                 testDigest, CorrelationID: testCorrelation,
	}
	input.EffectDigest = sideTaskParentEffectDigest(input)
	return input
}
