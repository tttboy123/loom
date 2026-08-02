package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

type testRecoveryPolicyPort struct {
	policy rules.RecoveryPolicy
}

func (port testRecoveryPolicyPort) Valid() bool    { return port.policy.Valid() }
func (port testRecoveryPolicyPort) Version() int   { return port.policy.Version() }
func (port testRecoveryPolicyPort) Digest() string { return port.policy.Digest() }
func (port testRecoveryPolicyPort) AttemptCredits() int {
	return port.policy.AttemptCredits()
}
func (port testRecoveryPolicyPort) RetryDelay() time.Duration {
	return port.policy.RetryDelay()
}
func (port testRecoveryPolicyPort) RecoveryApprovalRequired() bool {
	return port.policy.RecoveryApprovalRequired()
}
func (port testRecoveryPolicyPort) Decide(
	request TeamRecoveryDecisionRequest,
) (TeamRecoveryDecision, error) {
	return rules.DecideRecovery(
		port.policy,
		rules.RecoveryInput{
			TeamInstanceID:           request.TeamInstanceID,
			PlanDigest:               request.PlanDigest,
			LogicalNodeID:            request.LogicalNodeID,
			AttemptNumber:            request.AttemptNumber,
			MaxAttempts:              request.MaxAttempts,
			AgentInstanceID:          request.AgentInstanceID,
			RuntimeInstanceID:        request.RuntimeInstanceID,
			EvidenceID:               request.EvidenceID,
			EvidenceDigest:           request.EvidenceDigest,
			OutputSummaryDigest:      request.OutputSummaryDigest,
			Classification:           request.Classification,
			PriorClassifications:     request.PriorClassifications,
			RemainingCredits:         request.RemainingCredits,
			FallbackConsumed:         request.FallbackConsumed,
			DecisionTime:             request.DecisionTime,
			Trigger:                  rules.RecoveryTrigger(request.Trigger),
			AcceptanceDecisionDigest: request.AcceptanceDecisionDigest,
		},
	)
}

func TestTeamDispatchCASHasOneWinnerAndIndependentAttemptLineage(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x61)
	seedRuntime(t, store, "runtime-a", "online", 1)
	seedRuntime(t, store, "runtime-b", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-instance-1",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleMain, DependsOn: []string{"sub-a", "sub-b"},
				MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub-a", Title: "Build A",
				AgentInstanceID: "agent-a", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 3,
			},
			{
				LogicalNodeID: "sub-b", Title: "Build B",
				AgentInstanceID: "agent-b", RuntimeInstanceID: "runtime-b",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 3,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mainSelection := []TeamAttemptSelection{{
		LogicalNodeID: "main",
		AttemptNumber: 1,
	}}
	mainSnapshot, err := store.ReadStreamSet(
		context.Background(),
		teamDispatchStreams(
			plan,
			plan.Nodes(),
			mainSelection,
			TeamExecutionRecord{},
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.DispatchTeamReadySet(
		context.Background(),
		TeamDispatchInput{
			Plan: plan, ReadyAttempts: mainSelection,
			SemanticBindings:     testTeamSemanticBindings(plan),
			ViewVersion:          strings.Repeat("9", 64),
			ExpectedHeads:        mainSnapshot.Heads(),
			AuthoritativeTime:    testNow,
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        testCorrelation,
		},
	); !errors.Is(err, ErrInvalidTeamAttempt) {
		t.Fatalf("dependency-blocked Main dispatch error = %v", err)
	}
	streamIDs := []string{
		"team-execution/team-instance-1",
		runIdentityStreamID,
		workItemStream(teamAttemptIdentity("work", plan, "sub-a", 1)),
		runStream(teamAttemptIdentity("run", plan, "sub-a", 1)),
		runtimeStatusStream("runtime-a"),
		runtimeCapacityStream("runtime-a"),
		workItemStream(teamAttemptIdentity("work", plan, "sub-b", 1)),
		runStream(teamAttemptIdentity("run", plan, "sub-b", 1)),
		runtimeStatusStream("runtime-b"),
		runtimeCapacityStream("runtime-b"),
	}
	snapshot, err := store.ReadStreamSet(context.Background(), streamIDs)
	if err != nil {
		t.Fatal(err)
	}
	input := TeamDispatchInput{
		Plan:             plan,
		SemanticBindings: testTeamSemanticBindings(plan),
		ReadyAttempts: []TeamAttemptSelection{
			{LogicalNodeID: "sub-a", AttemptNumber: 1},
			{LogicalNodeID: "sub-b", AttemptNumber: 1},
		},
		ViewVersion:          strings.Repeat("a", 64),
		ExpectedHeads:        snapshot.Heads(),
		AuthoritativeTime:    testNow,
		PrepareLeaseDuration: time.Minute,
		CorrelationID:        testCorrelation,
	}
	var wait sync.WaitGroup
	wait.Add(2)
	results := make(chan TeamDispatchResult, 2)
	failures := make(chan error, 2)
	for range 2 {
		go func() {
			defer wait.Done()
			result, dispatchErr := authority.DispatchTeamReadySet(
				context.Background(),
				input,
			)
			if dispatchErr != nil {
				failures <- dispatchErr
				return
			}
			results <- result
		}()
	}
	wait.Wait()
	close(results)
	close(failures)
	if len(results) != 1 || len(failures) != 1 {
		t.Fatalf("successes=%d failures=%d", len(results), len(failures))
	}
	if conflict := <-failures; !errors.Is(conflict, ErrTeamExecutionConflict) &&
		!errors.Is(conflict, ErrStaleGlobalReadView) {
		t.Fatalf("loser error = %v", conflict)
	}
	result := <-results
	nodes := result.Nodes()
	if len(nodes) != 2 {
		t.Fatalf("dispatched nodes = %d", len(nodes))
	}
	for _, node := range nodes {
		attempt := node.Attempt()
		if attempt.AttemptNumber() != 1 ||
			!strings.HasPrefix(attempt.WorkItemID(), "team-work-") ||
			!strings.HasPrefix(attempt.RunID(), "team-run-") ||
			attempt.ClaimGeneration() != 1 {
			t.Fatalf("attempt lineage = %#v", attempt)
		}
	}
	if events, err := store.ReadStream(
		context.Background(),
		"team-execution/team-instance-1",
	); err != nil || len(events) != 4 {
		t.Fatalf("team events = %d, %v", len(events), err)
	}
}

func TestTeamRecoveryIsExplicitTimeBoundedAndStopsAtMaxAttempts(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x71)
	seedRuntime(t, store, "runtime-a", "online", 1)
	seedRuntime(t, store, "runtime-b", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-recovery",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Main",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleMain, MaxAttempts: 2,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	selection := func(attempt int) []TeamAttemptSelection {
		return []TeamAttemptSelection{{
			LogicalNodeID: "main",
			AttemptNumber: attempt,
		}}
	}
	dispatchInput := func(attempt int, at time.Time) TeamDispatchInput {
		selected := selection(attempt)
		teamEvents, readErr := store.ReadStream(
			context.Background(),
			teamExecutionStream(plan.TeamInstanceID()),
		)
		if readErr != nil {
			t.Fatal(readErr)
		}
		candidateTeam, replayErr := replayTeamExecution(
			plan.TeamInstanceID(),
			teamEvents,
		)
		if replayErr != nil {
			t.Fatal(replayErr)
		}
		snapshot, readErr := store.ReadStreamSet(
			context.Background(),
			teamDispatchStreams(
				plan,
				plan.Nodes(),
				selected,
				candidateTeam,
			),
		)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return TeamDispatchInput{
			Plan: plan, ReadyAttempts: selected,
			SemanticBindings:     testTeamSemanticBindings(plan),
			ViewVersion:          strings.Repeat("b", 64),
			ExpectedHeads:        snapshot.Heads(),
			AuthoritativeTime:    at,
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        testCorrelation,
		}
	}
	commitFailedAttempt := func(
		dispatched TeamDispatchedNode,
		attemptNumber int,
	) (TeamExecutionRecord, verification.Classification) {
		t.Helper()
		run := dispatched.Run()
		generation := RunGenerationInput{
			WorkItemID: run.WorkItemID(), RunID: run.ID(),
			ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(),
			AgentInstanceID:   run.AgentInstanceID(),
			CorrelationID:     testCorrelation,
		}
		if _, _, startErr := authority.Start(
			context.Background(),
			generation,
		); startErr != nil {
			t.Fatal(startErr)
		}
		if _, _, terminalErr := authority.CommitTerminal(
			context.Background(),
			RunTerminalInput{
				RunGenerationInput: generation,
				Status:             "failed",
				Reason:             "fixture_failed",
			},
		); terminalErr != nil {
			t.Fatal(terminalErr)
		}
		receipt, classification := testTeamAttemptReceiptAndClassification(
			t,
			plan,
			"main",
			attemptNumber,
			generation,
			"failed",
		)
		record, evidenceErr := authority.CommitTeamAttemptEvidence(
			context.Background(),
			TeamAttemptEvidenceInput{
				TeamInstanceID:    plan.TeamInstanceID(),
				PlanDigest:        plan.Digest(),
				LogicalNodeID:     "main",
				AttemptNumber:     attemptNumber,
				WorkItemID:        generation.WorkItemID,
				RunID:             generation.RunID,
				ClaimID:           generation.ClaimID,
				ClaimGeneration:   generation.ClaimGeneration,
				RuntimeInstanceID: generation.RuntimeInstanceID,
				AgentInstanceID:   generation.AgentInstanceID,
				Receipt:           receipt,
				Classification:    classification,
				CorrelationID:     testCorrelation,
			},
		)
		if evidenceErr != nil {
			t.Fatal(evidenceErr)
		}
		return record, classification
	}

	first, err := authority.DispatchTeamReadySet(
		context.Background(),
		dispatchInput(1, testNow),
	)
	if err != nil {
		t.Fatal(err)
	}
	firstAttempt := first.Nodes()[0]
	team, firstClassification := commitFailedAttempt(firstAttempt, 1)
	if len(team.Nodes()) != 1 ||
		team.Nodes()[0].Status() != "awaiting_recovery" {
		t.Fatalf("failed attempt state = %#v", team.Nodes())
	}
	recoveryAuthorityTime := testNow.Add(30 * time.Second)
	retryAt := recoveryAuthorityTime.Add(time.Minute)
	recoveryInput := testTeamRecoveryInput(
		t,
		plan,
		team,
		firstClassification,
		testNow,
	)
	clock.Set(recoveryAuthorityTime)
	recoveryInput.Decision = nil
	var recoveryWait sync.WaitGroup
	recoveryWait.Add(2)
	recoveryResults := make(chan error, 2)
	for range 2 {
		go func() {
			defer recoveryWait.Done()
			_, scheduleErr := authority.ScheduleTeamNodeRecovery(
				context.Background(),
				recoveryInput,
			)
			recoveryResults <- scheduleErr
		}()
	}
	recoveryWait.Wait()
	close(recoveryResults)
	successfulRecoveryCalls := 0
	for scheduleErr := range recoveryResults {
		if scheduleErr == nil {
			successfulRecoveryCalls++
			continue
		}
		if !errors.Is(scheduleErr, ErrTeamExecutionConflict) {
			t.Fatalf("concurrent recovery error = %v", scheduleErr)
		}
	}
	if successfulRecoveryCalls == 0 {
		t.Fatal("concurrent recovery had no successful scheduler")
	}
	recoveryEvents, err := store.ReadStream(
		context.Background(),
		teamExecutionStream(plan.TeamInstanceID()),
	)
	if err != nil {
		t.Fatal(err)
	}
	recoveryEventCount := 0
	for _, event := range recoveryEvents {
		if event.Type == "TeamNodeRecoveryRecorded" {
			recoveryEventCount++
		}
	}
	if recoveryEventCount != 1 {
		t.Fatalf("concurrent recovery Events = %d", recoveryEventCount)
	}
	team, err = authority.TeamExecution(context.Background(), plan.TeamInstanceID())
	if err != nil {
		t.Fatal(err)
	}
	node := team.Nodes()[0]
	if node.Status() != "retry_scheduled" ||
		node.CurrentAttempt() != 2 ||
		!node.RetryAt().Equal(retryAt) {
		t.Fatalf("scheduled retry = %#v", node)
	}
	states := []teams.ExecutionNodeState{{
		LogicalNodeID: "main", Status: node.Status(),
		CurrentAttempt: node.CurrentAttempt(), RetryAt: node.RetryAt(),
	}}
	capacity := []teams.RuntimeCapacityState{{
		RuntimeInstanceID: "runtime-a", Capacity: 1,
	}}
	if ready, readyErr := teams.ReadyExecutionNodes(
		plan, states, capacity, retryAt.Add(-time.Nanosecond),
	); readyErr != nil || len(ready) != 0 {
		t.Fatalf("ready before retry_at = %#v, %v", ready, readyErr)
	}
	if ready, readyErr := teams.ReadyExecutionNodes(
		plan, states, capacity, retryAt,
	); readyErr != nil || len(ready) != 1 {
		t.Fatalf("ready at retry_at = %#v, %v", ready, readyErr)
	}
	if _, err := authority.DispatchTeamReadySet(
		context.Background(),
		dispatchInput(2, retryAt.Add(-time.Nanosecond)),
	); !errors.Is(err, ErrInvalidTeamAttempt) {
		t.Fatalf("dispatch before retry_at error = %v", err)
	}

	clock.Set(retryAt)
	retryInput := dispatchInput(2, retryAt)
	var wait sync.WaitGroup
	wait.Add(2)
	successes := make(chan TeamDispatchResult, 2)
	conflicts := make(chan error, 2)
	for range 2 {
		go func() {
			defer wait.Done()
			result, dispatchErr := authority.DispatchTeamReadySet(
				context.Background(),
				retryInput,
			)
			if dispatchErr != nil {
				conflicts <- dispatchErr
				return
			}
			successes <- result
		}()
	}
	wait.Wait()
	close(successes)
	close(conflicts)
	if len(successes) != 1 || len(conflicts) != 1 {
		t.Fatalf("retry successes=%d conflicts=%d", len(successes), len(conflicts))
	}
	if conflict := <-conflicts; !errors.Is(conflict, ErrTeamExecutionConflict) &&
		!errors.Is(conflict, ErrStaleGlobalReadView) {
		t.Fatalf("retry loser error = %v", conflict)
	}
	secondAttempt := (<-successes).Nodes()[0]
	if secondAttempt.Attempt().WorkItemID() == firstAttempt.Attempt().WorkItemID() ||
		secondAttempt.Attempt().RunID() == firstAttempt.Attempt().RunID() {
		t.Fatal("retry reused WorkItem or Run identity")
	}
	team, secondClassification := commitFailedAttempt(secondAttempt, 2)
	if team.Nodes()[0].Status() != "awaiting_recovery" {
		t.Fatalf("second failed attempt = %#v", team.Nodes()[0])
	}
	secondRecovery := testTeamRecoveryInput(
		t,
		plan,
		team,
		secondClassification,
		retryAt,
	)
	clock.Set(retryAt.Add(30 * time.Second))
	team, err = authority.ScheduleTeamNodeRecovery(
		context.Background(),
		secondRecovery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if team.Status() != "blocked" ||
		team.Nodes()[0].Status() != "blocked" ||
		len(team.Nodes()[0].Attempts()) != 2 {
		t.Fatalf("bounded recovery terminal = %#v", team)
	}
	clock.Set(retryAt.Add(time.Minute))
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		secondRecovery,
	); err != nil {
		t.Fatalf("exact terminal recovery retry error = %v", err)
	}
	replayed, err := authority.TeamExecution(
		context.Background(),
		plan.TeamInstanceID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := replayed.Nodes()[0].Attempts(); len(got) != 2 ||
		got[0].RunID() != firstAttempt.Attempt().RunID() ||
		got[1].RunID() != secondAttempt.Attempt().RunID() {
		t.Fatalf("replayed attempts = %#v", got)
	}
}

func TestTeamTerminalRecoveryIsTerminalOnceAndExactRetryIsIdempotent(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x81)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-degraded",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	failedTeam, classification := commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"f",
	)
	recovery := testTeamRecoveryInput(
		t,
		plan,
		failedTeam,
		classification,
		testNow,
	)
	team, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if team.Status() != "blocked" ||
		team.Nodes()[0].Status() != "blocked" ||
		team.Nodes()[0].DependencySatisfied() {
		t.Fatalf("blocked terminal = %#v", team)
	}
	retried, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	)
	if err != nil {
		t.Fatalf("exact recovery retry error = %v", err)
	}
	if retried.Status() != "blocked" {
		t.Fatalf("exact recovery retry = %#v", retried)
	}
	events, err := store.ReadStream(
		context.Background(),
		teamExecutionStream(plan.TeamInstanceID()),
	)
	if err != nil {
		t.Fatal(err)
	}
	var recoveryEvents, terminalEvents int
	for _, event := range events {
		switch event.Type {
		case "TeamNodeRecoveryRecorded":
			recoveryEvents++
		case "TeamExecutionTerminal":
			terminalEvents++
		}
	}
	if recoveryEvents != 1 || terminalEvents != 1 {
		t.Fatalf(
			"recovery events=%d terminal events=%d",
			recoveryEvents,
			terminalEvents,
		)
	}
	malformed := append([]journal.Event(nil), events...)
	for index := range malformed {
		if malformed[index].Type != "TeamNodeAttemptTerminal" {
			continue
		}
		var payload teamAttemptTerminalPayload
		if err := json.Unmarshal(malformed[index].PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		payload.ClaimGeneration++
		malformed[index].PayloadJSON, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if _, err := replayTeamExecution(
		plan.TeamInstanceID(),
		malformed,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("stale terminal binding replay error = %v", err)
	}
	conflict := recovery
	conflict.CorrelationID = "22222222-2222-4222-8222-222222222222"
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		conflict,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("conflicting recovery retry error = %v", err)
	}
}

func TestTeamRecoveryExactReplayRejectsMissingScheduledAttempt(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x82)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-partial-retry",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	failedTeam, classification := commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"partial-retry",
	)
	recovery := testTeamRecoveryInput(
		t,
		plan,
		failedTeam,
		classification,
		testNow,
	)
	if recovery.Decision.ActionValue() != "retry" {
		t.Fatalf("recovery action = %q", recovery.Decision.ActionValue())
	}
	appendRecoveryEventWithoutDownstreamForTest(
		t,
		store,
		failedTeam,
		recovery,
	)

	clock.Set(testNow.Add(time.Hour))
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("partial retry replay error = %v, want conflict", err)
	}
}

func TestTeamRecoveryExactReplayRejectsMissingTeamTerminal(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x83)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-partial-terminal",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	failedTeam, classification := commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"partial-terminal",
	)
	recovery := testTeamRecoveryInput(
		t,
		plan,
		failedTeam,
		classification,
		testNow,
	)
	if recovery.Decision.ActionValue() != "blocked" {
		t.Fatalf("recovery action = %q", recovery.Decision.ActionValue())
	}
	appendRecoveryEventWithoutDownstreamForTest(
		t,
		store,
		failedTeam,
		recovery,
	)

	clock.Set(testNow.Add(time.Hour))
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("partial terminal replay error = %v, want conflict", err)
	}
}

func TestTeamRecoveryExactReplayRejectsMismatchedDownstreamFact(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x84)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-mismatched-retry",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	failedTeam, classification := commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"mismatched-retry",
	)
	recovery := testTeamRecoveryInput(
		t,
		plan,
		failedTeam,
		classification,
		testNow,
	)
	transaction, err := buildTeamRecoveryTransaction(
		failedTeam,
		recovery,
		recovery.Decision,
	)
	if err != nil || len(transaction) != 2 {
		t.Fatalf("recovery transaction = %#v, %v", transaction, err)
	}
	transaction[1].CausationID = "mismatched-recovery-causation"
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(),
		[]journal.StreamHeadExpectation{{
			StreamID: teamExecutionStream(plan.TeamInstanceID()),
			Sequence: failedTeam.streamSequence,
		}},
		transaction,
	); err != nil {
		t.Fatal(err)
	}

	clock.Set(testNow.Add(time.Hour))
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("mismatched retry replay error = %v, want conflict", err)
	}
}

func TestTeamRecoveryExactReplayRejectsDuplicateScheduledAttempt(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x85)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-duplicate-retry",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	failedTeam, classification := commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"duplicate-retry",
	)
	recovery := testTeamRecoveryInput(
		t,
		plan,
		failedTeam,
		classification,
		testNow,
	)
	transaction, err := buildTeamRecoveryTransaction(
		failedTeam,
		recovery,
		recovery.Decision,
	)
	if err != nil || len(transaction) != 2 {
		t.Fatalf("recovery transaction = %#v, %v", transaction, err)
	}
	duplicate := transaction[1]
	duplicate.ID = deterministicEventID(
		"DuplicateTeamNodeAttemptScheduled",
		plan.TeamInstanceID(),
		"main",
		"2",
	)
	duplicate.IdempotencyKey = duplicate.ID
	duplicate.Seq++
	duplicate.CausationID = transaction[1].ID
	transaction = append(transaction, duplicate)
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(),
		[]journal.StreamHeadExpectation{{
			StreamID: teamExecutionStream(plan.TeamInstanceID()),
			Sequence: failedTeam.streamSequence,
		}},
		transaction,
	); err != nil {
		t.Fatal(err)
	}

	clock.Set(testNow.Add(time.Hour))
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("duplicate retry replay error = %v, want conflict", err)
	}
}

func TestTeamRecoveryExactReplayRejectsDuplicateTeamTerminal(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x86)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-duplicate-terminal",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	failedTeam, classification := commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"duplicate-terminal",
	)
	recovery := testTeamRecoveryInput(
		t,
		plan,
		failedTeam,
		classification,
		testNow,
	)
	transaction, err := buildTeamRecoveryTransaction(
		failedTeam,
		recovery,
		recovery.Decision,
	)
	if err != nil || len(transaction) != 2 {
		t.Fatalf("recovery transaction = %#v, %v", transaction, err)
	}
	duplicate := transaction[1]
	duplicate.ID = deterministicEventID(
		"DuplicateTeamExecutionTerminal",
		plan.TeamInstanceID(),
		plan.Digest(),
	)
	duplicate.IdempotencyKey = duplicate.ID
	duplicate.Seq++
	duplicate.CausationID = transaction[1].ID
	transaction = append(transaction, duplicate)
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(),
		[]journal.StreamHeadExpectation{{
			StreamID: teamExecutionStream(plan.TeamInstanceID()),
			Sequence: failedTeam.streamSequence,
		}},
		transaction,
	); err != nil {
		t.Fatal(err)
	}

	clock.Set(testNow.Add(time.Hour))
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("duplicate terminal replay error = %v, want conflict", err)
	}
}

func TestTeamAttemptRebindFencesExpiredGenerationAndIsIdempotent(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0x91)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-rebind",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	oldAttempt := dispatched.Attempt()
	oldRun := dispatched.Run()
	clock.now = oldRun.PrepareLeaseExpiresAt()
	_, reclaimed, err := authority.Claim(
		context.Background(),
		RunClaimInput{
			WorkItemID:           oldRun.WorkItemID(),
			RunID:                oldRun.ID(),
			RuntimeInstanceID:    oldRun.RuntimeInstanceID(),
			AgentInstanceID:      oldRun.AgentInstanceID(),
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        testCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed.ClaimGeneration() != oldAttempt.ClaimGeneration()+1 ||
		reclaimed.ClaimID() == oldAttempt.ClaimID() {
		t.Fatalf("reclaimed Run = %#v", reclaimed)
	}
	input := TeamAttemptRebindInput{
		TeamInstanceID:          plan.TeamInstanceID(),
		PlanDigest:              plan.Digest(),
		LogicalNodeID:           "main",
		AttemptNumber:           1,
		WorkItemID:              oldAttempt.WorkItemID(),
		RunID:                   oldAttempt.RunID(),
		PreviousClaimID:         oldAttempt.ClaimID(),
		PreviousClaimGeneration: oldAttempt.ClaimGeneration(),
		ClaimID:                 reclaimed.ClaimID(),
		ClaimGeneration:         reclaimed.ClaimGeneration(),
		RuntimeInstanceID:       reclaimed.RuntimeInstanceID(),
		AgentInstanceID:         reclaimed.AgentInstanceID(),
		CorrelationID:           testCorrelation,
	}
	team, err := authority.RebindTeamAttempt(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	attempt := team.Nodes()[0].Attempts()[0]
	if attempt.ClaimID() != reclaimed.ClaimID() ||
		attempt.ClaimGeneration() != reclaimed.ClaimGeneration() ||
		attempt.Status() != "dispatched" {
		t.Fatalf("rebound attempt = %#v", attempt)
	}
	retried, err := authority.RebindTeamAttempt(context.Background(), input)
	if err != nil ||
		retried.Nodes()[0].Attempts()[0].ClaimGeneration() != 2 {
		t.Fatalf("exact rebound retry = %#v, %v", retried, err)
	}
	wrongPrevious := input
	wrongPrevious.PreviousClaimID =
		"88888888-8888-4888-8888-888888888888"
	if _, err := authority.RebindTeamAttempt(
		context.Background(),
		wrongPrevious,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("wrong previous rebound retry error = %v", err)
	}
	stale := input
	stale.ClaimID = "99999999-9999-4999-8999-999999999999"
	if _, err := authority.RebindTeamAttempt(
		context.Background(),
		stale,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("stale rebound error = %v", err)
	}
	events, err := store.ReadStream(
		context.Background(),
		teamExecutionStream(plan.TeamInstanceID()),
	)
	if err != nil {
		t.Fatal(err)
	}
	var rebounds int
	for _, event := range events {
		if event.Type == "TeamNodeAttemptRebound" {
			rebounds++
		}
	}
	if rebounds != 1 {
		t.Fatalf("rebound Events = %d", rebounds)
	}
}

func dispatchTeamAttemptForTest(
	t testing.TB,
	store *journal.Store,
	authority *Authority,
	plan teams.ExecutionPlan,
	attemptNumber int,
	at time.Time,
) TeamDispatchedNode {
	t.Helper()
	selections := []TeamAttemptSelection{{
		LogicalNodeID: "main",
		AttemptNumber: attemptNumber,
	}}
	teamEvents, err := store.ReadStream(
		context.Background(),
		teamExecutionStream(plan.TeamInstanceID()),
	)
	if err != nil {
		t.Fatal(err)
	}
	team, err := replayTeamExecution(plan.TeamInstanceID(), teamEvents)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.ReadStreamSet(
		context.Background(),
		teamDispatchStreams(plan, plan.Nodes(), selections, team),
	)
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := authority.DispatchTeamReadySet(
		context.Background(),
		TeamDispatchInput{
			Plan: plan, ReadyAttempts: selections,
			SemanticBindings:     testTeamSemanticBindings(plan),
			ViewVersion:          strings.Repeat("7", 64),
			ExpectedHeads:        snapshot.Heads(),
			AuthoritativeTime:    at,
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        testCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return dispatched.Nodes()[0]
}

func commitTeamAttemptTerminalForTest(
	t testing.TB,
	authority *Authority,
	plan teams.ExecutionPlan,
	dispatched TeamDispatchedNode,
	attemptNumber int,
	status string,
	digestByte string,
) (TeamExecutionRecord, verification.Classification) {
	t.Helper()
	run := dispatched.Run()
	generation := RunGenerationInput{
		WorkItemID: run.WorkItemID(), RunID: run.ID(),
		ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
		RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID:   run.AgentInstanceID(),
		CorrelationID:     testCorrelation,
	}
	if _, _, err := authority.Start(
		context.Background(),
		generation,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.CommitTerminal(
		context.Background(),
		RunTerminalInput{
			RunGenerationInput: generation,
			Status:             status,
			Reason:             "fixture_terminal",
		},
	); err != nil {
		t.Fatal(err)
	}
	receipt, classification := testTeamAttemptReceiptAndClassification(
		t,
		plan,
		"main",
		attemptNumber,
		generation,
		status,
	)
	team, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		TeamAttemptEvidenceInput{
			TeamInstanceID:    plan.TeamInstanceID(),
			PlanDigest:        plan.Digest(),
			LogicalNodeID:     "main",
			AttemptNumber:     attemptNumber,
			WorkItemID:        generation.WorkItemID,
			RunID:             generation.RunID,
			ClaimID:           generation.ClaimID,
			ClaimGeneration:   generation.ClaimGeneration,
			RuntimeInstanceID: generation.RuntimeInstanceID,
			AgentInstanceID:   generation.AgentInstanceID,
			Receipt:           receipt,
			Classification:    classification,
			CorrelationID:     testCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = digestByte
	return team, classification
}

func TestTeamDispatchFreezesSemanticBindingsBeforeLaterWorkWrites(t *testing.T) {
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0x66)
	seedRuntime(t, store, "runtime-a", "online", 2)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-semantic-freeze",
		Nodes: []teams.ExecutionNodeInput{
			{
				LogicalNodeID: "first", Title: "First",
				AgentInstanceID: "agent-first", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleMain, MaxAttempts: 2,
			},
			{
				LogicalNodeID: "second", Title: "Second",
				AgentInstanceID: "agent-second", RuntimeInstanceID: "runtime-a",
				Role: teams.ExecutionRoleSubAgent, MaxAttempts: 2,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	bindings := testTeamSemanticBindings(plan)
	dispatch := func(
		selection TeamAttemptSelection,
		semanticBindings []TeamNodeSemanticBinding,
		at time.Time,
	) error {
		events, readErr := store.ReadStream(
			context.Background(),
			teamExecutionStream(plan.TeamInstanceID()),
		)
		if readErr != nil {
			t.Fatal(readErr)
		}
		current, replayErr := replayTeamExecution(plan.TeamInstanceID(), events)
		if replayErr != nil {
			t.Fatal(replayErr)
		}
		snapshot, readErr := store.ReadStreamSet(
			context.Background(),
			teamDispatchStreams(
				plan,
				plan.Nodes(),
				[]TeamAttemptSelection{selection},
				current,
			),
		)
		if readErr != nil {
			t.Fatal(readErr)
		}
		_, dispatchErr := authority.DispatchTeamReadySet(
			context.Background(),
			TeamDispatchInput{
				Plan:                 plan,
				ReadyAttempts:        []TeamAttemptSelection{selection},
				SemanticBindings:     semanticBindings,
				ViewVersion:          strings.Repeat("e", 64),
				ExpectedHeads:        snapshot.Heads(),
				AuthoritativeTime:    at,
				PrepareLeaseDuration: time.Minute,
				CorrelationID:        testCorrelation,
			},
		)
		return dispatchErr
	}
	if err := dispatch(
		TeamAttemptSelection{LogicalNodeID: "first", AttemptNumber: 1},
		bindings,
		testNow,
	); err != nil {
		t.Fatal(err)
	}
	changed := append([]TeamNodeSemanticBinding(nil), bindings...)
	changed[1].RecoveryPolicyDigest = strings.Repeat("f", 64)
	if err := dispatch(
		TeamAttemptSelection{LogicalNodeID: "second", AttemptNumber: 1},
		changed,
		testNow.Add(time.Second),
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("changed semantic binding error = %v", err)
	}
	events, err := store.ReadStream(
		context.Background(),
		workItemStream(teamAttemptIdentity("work", plan, "second", 1)),
	)
	if err != nil || len(events) != 0 {
		t.Fatalf("later WorkItem events = %d, %v", len(events), err)
	}
}

func TestTeamAttemptCommitRejectsZeroAndMismatchedAuthorityValues(t *testing.T) {
	store := openAuthorityStore(t)
	authority := newAuthority(t, store, &mutableClock{now: testNow}, 0x67)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-attempt-authority-values",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatched := dispatchTeamAttemptForTest(
		t,
		store,
		authority,
		plan,
		1,
		testNow,
	)
	run := dispatched.Run()
	generation := RunGenerationInput{
		WorkItemID: run.WorkItemID(), RunID: run.ID(),
		ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
		RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID:   run.AgentInstanceID(),
		CorrelationID:     testCorrelation,
	}
	if _, _, err := authority.Start(
		context.Background(),
		generation,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.CommitTerminal(
		context.Background(),
		RunTerminalInput{
			RunGenerationInput: generation,
			Status:             "failed",
			Reason:             "fixture_terminal",
		},
	); err != nil {
		t.Fatal(err)
	}
	input := TeamAttemptEvidenceInput{
		TeamInstanceID:    plan.TeamInstanceID(),
		PlanDigest:        plan.Digest(),
		LogicalNodeID:     "main",
		AttemptNumber:     1,
		WorkItemID:        generation.WorkItemID,
		RunID:             generation.RunID,
		ClaimID:           generation.ClaimID,
		ClaimGeneration:   generation.ClaimGeneration,
		RuntimeInstanceID: generation.RuntimeInstanceID,
		AgentInstanceID:   generation.AgentInstanceID,
		CorrelationID:     testCorrelation,
	}
	before, err := store.ReadStream(
		context.Background(),
		teamExecutionStream(plan.TeamInstanceID()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		input,
	); !errors.Is(err, ErrInvalidTeamExecution) {
		t.Fatalf("zero authority values error = %v", err)
	}
	receipt, classification := testTeamAttemptReceiptAndClassification(
		t,
		plan,
		"main",
		1,
		generation,
		"failed",
	)
	summary := receipt.OutputSummary()
	changedContract, err := verification.NewOutputContract(
		2,
		verification.EmptyOutputInvalid,
	)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := verification.NewOutputObservation(
		receipt.EvidenceID(),
		receipt.Digest(),
		summary.Digest(),
		summary.AuthorizedFrameCount(),
		summary.OutputFrameCount(),
		summary.OutputPayloadBytes(),
		summary.ResultObserved(),
		summary.TerminalStatus(),
	)
	if err != nil {
		t.Fatal(err)
	}
	mismatched, err := verification.Classify(changedContract, observation)
	if err != nil {
		t.Fatal(err)
	}
	input.Receipt = receipt
	input.Classification = mismatched
	if _, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		input,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("mismatched classification error = %v", err)
	}
	exactContract, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputInvalid,
	)
	if err != nil {
		t.Fatal(err)
	}
	forgedObservation, err := verification.NewOutputObservation(
		receipt.EvidenceID(),
		receipt.Digest(),
		summary.Digest(),
		1,
		1,
		1,
		false,
		summary.TerminalStatus(),
	)
	if err != nil {
		t.Fatal(err)
	}
	forgedClassification, err := verification.Classify(
		exactContract,
		forgedObservation,
	)
	if err != nil {
		t.Fatal(err)
	}
	input.Classification = forgedClassification
	if _, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		input,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("forged observation classification error = %v", err)
	}
	afterRejected, err := store.ReadStream(
		context.Background(),
		teamExecutionStream(plan.TeamInstanceID()),
	)
	if err != nil || len(afterRejected) != len(before) {
		t.Fatalf(
			"rejected commit Team Events = %d -> %d, %v",
			len(before),
			len(afterRejected),
			err,
		)
	}
	input.Classification = classification
	first, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	exact, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		input,
	)
	if err != nil || exact.Status() != first.Status() {
		t.Fatalf("exact attempt commit = %#v, %v", exact, err)
	}
}

func testTeamSemanticBindings(
	plan teams.ExecutionPlan,
) []TeamNodeSemanticBinding {
	nodes := plan.Nodes()
	bindings := make([]TeamNodeSemanticBinding, 0, len(nodes))
	for _, node := range nodes {
		credits := node.MaxAttempts() - 1
		if credits > 2 {
			credits = 2
		}
		contract, err := verification.NewOutputContract(
			1,
			verification.EmptyOutputInvalid,
		)
		if err != nil {
			panic(err)
		}
		policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
			Version:          1,
			RetryDelay:       time.Minute,
			AttemptCredits:   credits,
			ExhaustionAction: rules.ExhaustionBlocked,
			RetryInvalid:     true,
		})
		if err != nil {
			panic(err)
		}
		acceptance, err := verification.NewAcceptanceContract(
			1,
			[]string{"controlled output is accepted"},
			verification.AcceptanceRiskLow,
		)
		if err != nil {
			panic(err)
		}
		bindings = append(bindings, TeamNodeSemanticBinding{
			LogicalNodeID:             node.LogicalNodeID(),
			OutputContractVersion:     contract.Version(),
			OutputContractDigest:      contract.Digest(),
			RecoveryPolicyVersion:     policy.Version(),
			RecoveryPolicyDigest:      policy.Digest(),
			AttemptCredits:            credits,
			PrimaryWorkflowPath:       "primary",
			WorkflowFallbackKey:       "",
			RecoveryApprovalRequired:  false,
			AcceptanceContractVersion: acceptance.Version(),
			AcceptanceContractDigest:  acceptance.Digest(),
			AcceptanceRisk:            string(acceptance.Risk()),
		})
	}
	return bindings
}

func testTeamAttemptReceiptAndClassification(
	t testing.TB,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
	generation RunGenerationInput,
	status string,
) (evidence.AttemptReceipt, verification.Classification) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := evidence.NewStore(filepath.Join(parent, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	evidenceID := teamAttemptIdentity(
		"evidence",
		plan,
		logicalNodeID,
		attemptNumber,
	)
	binding := evidence.AttemptCaptureInput{
		EvidenceID:        evidenceID,
		TeamInstanceID:    plan.TeamInstanceID(),
		PlanDigest:        plan.Digest(),
		LogicalNodeID:     logicalNodeID,
		AttemptNumber:     attemptNumber,
		WorkItemID:        generation.WorkItemID,
		RunID:             generation.RunID,
		ClaimID:           generation.ClaimID,
		ClaimGeneration:   generation.ClaimGeneration,
		RuntimeInstanceID: generation.RuntimeInstanceID,
		AgentInstanceID:   generation.AgentInstanceID,
	}
	if err := store.BeginAttemptCapture(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	reason := ""
	if status != "succeeded" {
		reason = "fixture_terminal"
	} else {
		for _, frame := range []bridgev1.Frame{
			testTeamAttemptFrame(
				t,
				binding,
				2,
				bridgev1.MessageAck,
				[]byte(`{"message_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}`),
			),
			testTeamAttemptFrame(
				t,
				binding,
				3,
				bridgev1.MessageEvent,
				[]byte(`{"delta":"authorized output"}`),
			),
			testTeamAttemptFrame(
				t,
				binding,
				4,
				bridgev1.MessageResult,
				[]byte(`{"status":"succeeded","reason":""}`),
			),
		} {
			line, err := bridgev1.EncodeLine(frame)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.AppendAttemptFrame(
				context.Background(),
				evidenceID,
				line,
			); err != nil {
				t.Fatal(err)
			}
		}
	}
	receipt, err := store.FinalizeAttemptCapture(
		context.Background(),
		evidenceID,
		evidence.AttemptTerminal{Status: status, Reason: reason},
	)
	if err != nil {
		t.Fatal(err)
	}
	summary := receipt.OutputSummary()
	contract, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputInvalid,
	)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := verification.NewOutputObservation(
		receipt.EvidenceID(),
		receipt.Digest(),
		summary.Digest(),
		summary.AuthorizedFrameCount(),
		summary.OutputFrameCount(),
		summary.OutputPayloadBytes(),
		summary.ResultObserved(),
		summary.TerminalStatus(),
	)
	if err != nil {
		t.Fatal(err)
	}
	classification, err := verification.Classify(contract, observation)
	if err != nil {
		t.Fatal(err)
	}
	return receipt, classification
}

func testTeamAttemptFrame(
	t testing.TB,
	binding evidence.AttemptCaptureInput,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	t.Helper()
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: fmt.Sprintf(
			"00000000-0000-4000-8000-%012x",
			sequence,
		),
		CorrelationID:         testCorrelation,
		WorkItemID:            binding.WorkItemID,
		RunID:                 binding.RunID,
		ClaimGeneration:       binding.ClaimGeneration,
		RuntimeInstanceID:     binding.RuntimeInstanceID,
		SenderAgentInstanceID: binding.AgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             testNow,
		Payload:               payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func testTeamRecoveryInput(
	t testing.TB,
	plan teams.ExecutionPlan,
	team TeamExecutionRecord,
	classification verification.Classification,
	decisionTime time.Time,
) TeamRecoveryInput {
	t.Helper()
	node := teamNodeByID(&team, "main")
	if node == nil {
		t.Fatal("missing main node")
	}
	attempt := teamAttemptByNumber(node, node.currentAttempt)
	if attempt == nil {
		t.Fatal("missing current attempt")
	}
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:          node.semanticBinding.RecoveryPolicyVersion,
		RetryDelay:       time.Minute,
		AttemptCredits:   node.semanticBinding.AttemptCredits,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil || policy.Digest() != node.semanticBinding.RecoveryPolicyDigest {
		t.Fatalf("recovery policy = %#v, %v", policy, err)
	}
	remaining := node.semanticBinding.AttemptCredits -
		(node.currentAttempt - 1)
	decision, err := rules.DecideRecovery(
		policy,
		rules.RecoveryInput{
			TeamInstanceID:      plan.TeamInstanceID(),
			PlanDigest:          plan.Digest(),
			LogicalNodeID:       node.logicalNodeID,
			AttemptNumber:       node.currentAttempt,
			MaxAttempts:         node.maxAttempts,
			AgentInstanceID:     attempt.agentInstanceID,
			RuntimeInstanceID:   attempt.runtimeInstanceID,
			EvidenceID:          classification.EvidenceID(),
			EvidenceDigest:      classification.EvidenceDigest(),
			OutputSummaryDigest: classification.SummaryDigest(),
			Classification:      classification,
			PriorClassifications: teamPriorClassifications(
				node,
				node.currentAttempt,
			),
			RemainingCredits: remaining,
			FallbackConsumed: teamFallbackConsumed(node),
			DecisionTime:     decisionTime,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return TeamRecoveryInput{
		TeamInstanceID:      plan.TeamInstanceID(),
		PlanDigest:          plan.Digest(),
		LogicalNodeID:       node.logicalNodeID,
		AttemptNumber:       node.currentAttempt,
		MaxAttempts:         node.maxAttempts,
		AgentInstanceID:     attempt.agentInstanceID,
		RuntimeInstanceID:   attempt.runtimeInstanceID,
		EvidenceID:          classification.EvidenceID(),
		EvidenceDigest:      classification.EvidenceDigest(),
		OutputSummaryDigest: classification.SummaryDigest(),
		Classification:      classification,
		RecoveryPolicy:      testRecoveryPolicyPort{policy: policy},
		Decision:            decision,
		CorrelationID:       testCorrelation,
	}
}

func appendRecoveryEventWithoutDownstreamForTest(
	t testing.TB,
	store *journal.Store,
	team TeamExecutionRecord,
	input TeamRecoveryInput,
) {
	t.Helper()
	decision := input.Decision
	if decision == nil || !decision.Valid() {
		t.Fatal("missing valid recovery decision")
	}
	prior := decision.PriorClassificationValues()
	streamID := teamExecutionStream(input.TeamInstanceID)
	recoveryID := deterministicEventID(
		"TeamNodeRecoveryRecorded",
		decision.TeamInstanceID(),
		decision.LogicalNodeID(),
		fmt.Sprint(decision.AttemptNumber()),
		decision.Digest(),
	)
	event := newEvent(
		recoveryID,
		streamID,
		team.streamSequence+1,
		"TeamNodeRecoveryRecorded",
		decision.DecisionTime(),
		input.CorrelationID,
		team.lastEventID,
		teamRecoveryPayload{
			LogicalNodeID:          decision.LogicalNodeID(),
			AttemptNumber:          decision.AttemptNumber(),
			Action:                 decision.ActionValue(),
			DecisionTime:           decision.DecisionTime().Format(time.RFC3339Nano),
			RetryAt:                formatOptionalUTC(decision.RetryAt()),
			NextAttemptNumber:      decision.NextAttemptNumber(),
			NextAgentInstanceID:    decision.NextAgentInstanceID(),
			NextRuntimeInstanceID:  decision.NextRuntimeInstanceID(),
			WorkflowFallbackKey:    decision.WorkflowFallbackKey(),
			RecoveryPolicyVersion:  decision.PolicyVersion(),
			RecoveryPolicyDigest:   decision.PolicyDigest(),
			RecoveryDecisionDigest: decision.Digest(),
			ClassificationDigest:   decision.ClassificationDigest(),
			PriorClassifications:   prior,
			CreditsBefore:          decision.CreditsBefore(),
			CreditsAfter:           decision.CreditsAfter(),
			FallbackConsumed: decision.FallbackConsumed() ||
				decision.ActionValue() == "fallback",
			RecoveryApprovalRequired: decision.RecoveryApprovalRequired(),
			DependencySatisfied:      decision.ActionValue() == "degraded",
			RecoveryTrigger:          decision.TriggerValue(),
			AcceptanceDecisionDigest: decision.AcceptanceDecisionDigest(),
		},
	)
	if _, err := store.AppendBatchIfStreamHeads(
		context.Background(),
		[]journal.StreamHeadExpectation{{
			StreamID: streamID,
			Sequence: team.streamSequence,
		}},
		[]journal.Event{event},
	); err != nil {
		t.Fatal(err)
	}
}
