package work

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/teams"
)

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
		Plan: plan,
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
				Role: teams.ExecutionRoleMain, MaxAttempts: 3,
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
		digestByte string,
	) TeamExecutionRecord {
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
				EvidenceID: teamAttemptIdentity(
					"evidence", plan, "main", attemptNumber,
				),
				EvidenceDigest: strings.Repeat(digestByte, 64),
				CorrelationID:  testCorrelation,
			},
		)
		if evidenceErr != nil {
			t.Fatal(evidenceErr)
		}
		return record
	}

	first, err := authority.DispatchTeamReadySet(
		context.Background(),
		dispatchInput(1, testNow),
	)
	if err != nil {
		t.Fatal(err)
	}
	firstAttempt := first.Nodes()[0]
	team := commitFailedAttempt(firstAttempt, 1, "c")
	if len(team.Nodes()) != 1 ||
		team.Nodes()[0].Status() != "awaiting_recovery" {
		t.Fatalf("failed attempt state = %#v", team.Nodes())
	}
	retryAt := testNow.Add(time.Minute)
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		TeamRecoveryInput{
			TeamInstanceID:        "team-recovery",
			PlanDigest:            plan.Digest(),
			LogicalNodeID:         "main",
			AttemptNumber:         1,
			Action:                TeamRecoveryRetry,
			RetryAt:               retryAt,
			NextAgentInstanceID:   "agent-main",
			NextRuntimeInstanceID: "runtime-a",
			CorrelationID:         testCorrelation,
		},
	); err != nil {
		t.Fatal(err)
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

	clock.now = retryAt
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
	team = commitFailedAttempt(secondAttempt, 2, "d")
	if team.Nodes()[0].Status() != "awaiting_recovery" {
		t.Fatalf("second failed attempt = %#v", team.Nodes()[0])
	}
	secondRetryAt := retryAt.Add(time.Minute)
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		TeamRecoveryInput{
			TeamInstanceID:        plan.TeamInstanceID(),
			PlanDigest:            plan.Digest(),
			LogicalNodeID:         "main",
			AttemptNumber:         2,
			Action:                TeamRecoveryFallback,
			RetryAt:               secondRetryAt,
			NextAgentInstanceID:   "agent-fallback",
			NextRuntimeInstanceID: "runtime-b",
			CorrelationID:         testCorrelation,
		},
	); err != nil {
		t.Fatal(err)
	}
	clock.now = secondRetryAt
	third, err := authority.DispatchTeamReadySet(
		context.Background(),
		dispatchInput(3, secondRetryAt),
	)
	if err != nil {
		t.Fatal(err)
	}
	thirdAttempt := third.Nodes()[0]
	if thirdAttempt.Attempt().WorkItemID() == secondAttempt.Attempt().WorkItemID() ||
		thirdAttempt.Attempt().RunID() == secondAttempt.Attempt().RunID() ||
		thirdAttempt.Attempt().RuntimeInstanceID() != "runtime-b" ||
		thirdAttempt.Attempt().AgentInstanceID() != "agent-fallback" {
		t.Fatal("third attempt reused prior lineage")
	}
	team = commitFailedAttempt(thirdAttempt, 3, "e")
	if team.Status() != "failed" || team.Nodes()[0].Status() != "failed" ||
		len(team.Nodes()[0].Attempts()) != 3 {
		t.Fatalf("max-attempt terminal = %#v", team)
	}
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		TeamRecoveryInput{
			TeamInstanceID:        plan.TeamInstanceID(),
			PlanDigest:            plan.Digest(),
			LogicalNodeID:         "main",
			AttemptNumber:         3,
			Action:                TeamRecoveryRetry,
			RetryAt:               secondRetryAt.Add(time.Minute),
			NextAgentInstanceID:   "agent-main",
			NextRuntimeInstanceID: "runtime-a",
			CorrelationID:         testCorrelation,
		},
	); !errors.Is(err, ErrTeamExecutionAlreadyTerminal) {
		t.Fatalf("recovery beyond max error = %v", err)
	}
	replayed, err := authority.TeamExecution(
		context.Background(),
		plan.TeamInstanceID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := replayed.Nodes()[0].Attempts(); len(got) != 3 ||
		got[0].RunID() != firstAttempt.Attempt().RunID() ||
		got[1].RunID() != secondAttempt.Attempt().RunID() ||
		got[2].RunID() != thirdAttempt.Attempt().RunID() {
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
	commitTeamAttemptTerminalForTest(
		t,
		authority,
		plan,
		dispatched,
		1,
		"failed",
		"f",
	)
	recovery := TeamRecoveryInput{
		TeamInstanceID:      plan.TeamInstanceID(),
		PlanDigest:          plan.Digest(),
		LogicalNodeID:       "main",
		AttemptNumber:       1,
		Action:              TeamRecoveryDegraded,
		DependencySatisfied: true,
		CorrelationID:       testCorrelation,
	}
	team, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	)
	if err != nil {
		t.Fatal(err)
	}
	if team.Status() != "degraded" ||
		team.Nodes()[0].Status() != "degraded" ||
		!team.Nodes()[0].DependencySatisfied() {
		t.Fatalf("degraded terminal = %#v", team)
	}
	retried, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		recovery,
	)
	if err != nil {
		t.Fatalf("exact recovery retry error = %v", err)
	}
	if retried.Status() != "degraded" {
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
	conflict.DependencySatisfied = false
	if _, err := authority.ScheduleTeamNodeRecovery(
		context.Background(),
		conflict,
	); !errors.Is(err, ErrTeamExecutionConflict) {
		t.Fatalf("conflicting recovery retry error = %v", err)
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
) TeamExecutionRecord {
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
			EvidenceID: teamAttemptIdentity(
				"evidence", plan, "main", attemptNumber,
			),
			EvidenceDigest: strings.Repeat(digestByte, 64),
			CorrelationID:  testCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return team
}
