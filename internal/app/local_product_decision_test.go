package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
)

type preparedApprovalFixture struct {
	backend    *PreparedMissionDecisionBackend
	service    *LocalProductDecisionService
	authority  *rules.Authority
	work       *work.Authority
	projection *projection.Projection
	pending    rules.ApprovalRequestRecord
	context    rules.ActionContext
	decision   rules.Decision
	approved   rules.ApprovalDecisionRequest
	denied     rules.ApprovalDecisionRequest
	sheet      MissionDecisionSheet
}

type blockingApprovalAuthority struct {
	inner   ApprovalDecisionAuthority
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

type missionDecisionTestClock struct {
	now time.Time
}

func (clock *missionDecisionTestClock) Now() time.Time { return clock.now }

type missionDecisionTestAuthorizer struct {
	now time.Time
}

func (authorizer *missionDecisionTestAuthorizer) AuthorizeRuleSet(
	ctx context.Context,
	request rules.RuleSetActivationRequest,
) (rules.AuthorizedRuleSetActivation, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedRuleSetActivation{}, err
	}
	return rules.NewAuthorizedRuleSetActivation(
		request,
		"local-owner",
		missionDecisionTestDigest("activate", request.RuleSet().Digest()),
		authorizer.now,
		authorizer.now.Add(time.Minute),
	)
}

func (authorizer *missionDecisionTestAuthorizer) AuthorizeApprovalDecision(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
) (rules.AuthorizedApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return rules.AuthorizedApprovalDecision{}, err
	}
	return rules.NewAuthorizedApprovalDecision(
		request,
		"local-owner",
		missionDecisionTestDigest(
			"decision",
			request.ApprovalRequestDigest(),
			request.Decision(),
		),
		authorizer.now,
		authorizer.now.Add(time.Minute),
	)
}

func missionDecisionTestDigest(parts ...string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "\x00"))))
}

func (authority *blockingApprovalAuthority) DecideApproval(
	ctx context.Context,
	request rules.ApprovalDecisionRequest,
	correlationID string,
) (rules.ApprovalRequestRecord, error) {
	authority.calls.Add(1)
	close(authority.started)
	select {
	case <-authority.release:
	case <-ctx.Done():
		return rules.ApprovalRequestRecord{}, ctx.Err()
	}
	return authority.inner.DecideApproval(ctx, request, correlationID)
}

func TestMissionDecisionReadsExactPreparedSheetWithoutWriting(t *testing.T) {
	fixture := newPreparedApprovalFixture(t, nil)
	command := missionDecisionCommandFromSheet(fixture.sheet, "read", "read")
	before, ok := fixture.projection.GlobalReadView().ApprovalRequest(
		fixture.pending.ID(),
	)
	if !ok {
		t.Fatal("missing pending approval")
	}
	sheet, err := fixture.service.ReadMissionDecision(
		context.Background(),
		command,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sheet, fixture.sheet) {
		t.Fatalf("sheet = %#v", sheet)
	}
	after, ok := fixture.projection.GlobalReadView().ApprovalRequest(
		fixture.pending.ID(),
	)
	if !ok || !reflect.DeepEqual(after, before) {
		t.Fatalf("read changed approval: before=%#v after=%#v found=%v", before, after, ok)
	}
}

func TestControlledMissionDecisionFixtureBuildsRealJournalBackedRegistry(
	t *testing.T,
) {
	db := openTeamCanaryDB(t)
	artifactParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := BuildControlledMissionDecisionFixture(
		context.Background(),
		ControlledMissionDecisionFixtureConfig{
			Database: db,
			ArtifactRoot: filepath.Join(
				artifactParent,
				"controlled-artifacts",
			),
			AuthoritativeTime: time.Date(2026, 7, 30, 18, 0, 0, 0, time.UTC),
			FixtureID:         "p2a-w2-controlled",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Authorizations) != 2 ||
		len(prepared.Reviews) != 1 ||
		len(prepared.Recoveries) != 1 {
		t.Fatalf("prepared fixture = %#v", prepared)
	}
	if prepared.Reviews[0].Sheet.NetworkAccess !=
		"Independent Reviewer: PASS" ||
		!strings.Contains(
			strings.ToLower(
				prepared.Reviews[0].Sheet.ExpectedEvidence,
			),
			"accepted evidence",
		) {
		t.Fatalf("review sheet is not semantically review-ready: %#v",
			prepared.Reviews[0].Sheet)
	}
	if prepared.Recoveries[0].Sheet.AttemptScope !=
		"Attempt 2 · fresh generation" ||
		prepared.Recoveries[0].Sheet.Target !=
			"No Team or Provider change" ||
		prepared.Recoveries[0].Sheet.PermissionScope !=
			"No permission or budget increase" {
		t.Fatalf("recovery sheet hides its boundary: %#v",
			prepared.Recoveries[0].Sheet)
	}
	backend, err := NewPreparedMissionDecisionBackend(prepared)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := backend.ListMissionDecisionCommands(
		context.Background(),
	)
	if err != nil || len(commands) != 4 {
		t.Fatalf("commands = %#v, %v", commands, err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	executions, more := readModel.GlobalReadView().TeamExecutions(
		"",
		16,
	)
	if more || len(executions) != 5 {
		t.Fatalf("Journal-backed Team executions = %d, more=%v", len(executions), more)
	}
	var deny MissionDecisionCommand
	for _, command := range commands {
		if command.TeamInstanceID == "team-auth-deny" {
			deny = command
			break
		}
	}
	if deny.DecisionID == "" {
		t.Fatal("missing prepared deny mission")
	}
	deny.Operation = "submit"
	deny.Action = "deny"
	deny.CorrelationID = "99999999-9999-4999-8999-999999999999"
	service, err := NewLocalProductDecisionService(
		MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.DecideMission(context.Background(), deny)
	if err != nil || result.Status != "denied" || !result.Authoritative {
		t.Fatalf("deny result = %#v, %v", result, err)
	}
	remaining, err := backend.ListMissionDecisionCommands(
		context.Background(),
	)
	if err != nil || len(remaining) != 3 {
		t.Fatalf("remaining commands = %#v, %v", remaining, err)
	}
	for _, command := range remaining {
		if command.ViewVersion != result.ViewVersion {
			t.Fatalf("stale remaining command = %#v", command)
		}
	}
}

func TestMissionDecisionNotNowAndEditScopeAreZeroAuthorityCalls(t *testing.T) {
	fixture := newPreparedApprovalFixture(t, nil)
	before, ok := fixture.projection.GlobalReadView().ApprovalRequest(
		fixture.pending.ID(),
	)
	if !ok {
		t.Fatal("missing pending approval")
	}
	for _, action := range []string{"not_now", "edit_scope"} {
		command := missionDecisionCommandFromSheet(
			fixture.sheet,
			"defer",
			action,
		)
		result, err := fixture.service.DecideMission(
			context.Background(),
			command,
		)
		if err != nil {
			t.Fatalf("%s error = %v", action, err)
		}
		if result.Status != "pending" ||
			result.Authoritative ||
			result.ViewVersion != fixture.sheet.ViewVersion {
			t.Fatalf("%s result = %#v", action, result)
		}
	}
	if err := fixture.projection.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, ok := fixture.projection.GlobalReadView().ApprovalRequest(
		fixture.pending.ID(),
	)
	if !ok || !reflect.DeepEqual(after, before) {
		t.Fatalf("defer changed approval: before=%#v after=%#v found=%v", before, after, ok)
	}
}

func TestPreparedAuthorizationDelegatesExactRulesAuthorityInput(t *testing.T) {
	fixture := newPreparedApprovalFixture(t, nil)
	command := missionDecisionCommandFromSheet(
		fixture.sheet,
		"submit",
		"allow_once",
	)
	result, err := fixture.service.DecideMission(
		context.Background(),
		command,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Authoritative ||
		result.Status != "approved" ||
		result.ViewVersion == fixture.sheet.ViewVersion {
		t.Fatalf("result = %#v", result)
	}
	projected, ok := fixture.projection.GlobalReadView().ApprovalRequest(
		fixture.pending.ID(),
	)
	if !ok || projected.Status != "approved" {
		t.Fatalf("projected approval = %#v, %v", projected, ok)
	}
}

func TestPreparedAuthorizationRejectsMislabeledMissionContext(t *testing.T) {
	fixture := newPreparedApprovalFixture(t, nil)
	sheet := fixture.sheet
	sheet.MissionID = "mission/team-other"
	sheet.TeamInstanceID = "team-other"
	_, err := NewPreparedMissionDecisionBackend(
		PreparedMissionDecisions{
			Authorizations: []PreparedAuthorizationDecision{{
				Sheet:     sheet,
				Pending:   fixture.pending,
				Context:   fixture.context,
				Decision:  fixture.decision,
				Authority: fixture.authority,
				Deny:      fixture.denied,
				AllowOnce: fixture.approved,
				Refresh: projectionMissionDecisionRefresh(
					fixture.projection,
				),
			}},
		},
	)
	if !errors.Is(err, ErrInvalidMissionDecision) {
		t.Fatalf("mislabeled authorization error = %v", err)
	}
}

func TestPreparedMissionDecisionRejectsStaleViewAndGenerationBeforeAuthority(
	t *testing.T,
) {
	blocking := &blockingApprovalAuthority{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	fixture := newPreparedApprovalFixture(t, blocking)
	command := missionDecisionCommandFromSheet(
		fixture.sheet,
		"submit",
		"allow_once",
	)
	staleView := command
	staleView.ViewVersion = strings.Repeat("c", 64)
	if _, err := fixture.service.DecideMission(
		context.Background(),
		staleView,
	); !errors.Is(err, ErrMissionDecisionConflict) {
		t.Fatalf("stale view error = %v", err)
	}
	staleGeneration := command
	staleGeneration.ClaimGeneration++
	if _, err := fixture.service.DecideMission(
		context.Background(),
		staleGeneration,
	); !errors.Is(err, ErrMissionDecisionConflict) {
		t.Fatalf("stale generation error = %v", err)
	}
	if blocking.calls.Load() != 0 {
		t.Fatalf("stale binding reached authority %d times", blocking.calls.Load())
	}
	if _, _, err := fixture.work.CreateAndAssign(
		context.Background(),
		work.WorkItemAssignmentInput{
			WorkItemID:      "work-stale-view",
			Title:           "Advance authoritative view",
			RunID:           "run-stale-view",
			AgentInstanceID: "agent-stale-view",
			CorrelationID:   "33333333-3333-4333-8333-333333333333",
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.DecideMission(
		context.Background(),
		command,
	); !errors.Is(err, ErrMissionDecisionConflict) {
		t.Fatalf("stale authoritative view error = %v", err)
	}
	if blocking.calls.Load() != 0 {
		t.Fatalf(
			"stale authoritative view reached authority %d times",
			blocking.calls.Load(),
		)
	}
}

func TestPreparedMissionDecisionPublishesCurrentViewAfterJournalAdvance(
	t *testing.T,
) {
	fixture := newPreparedApprovalFixture(t, nil)
	ctx := context.Background()
	before, err := fixture.backend.ListMissionDecisionCommands(ctx)
	if err != nil || len(before) != 1 {
		t.Fatalf("initial commands = %#v, %v", before, err)
	}
	oldCommand := before[0]

	if _, _, err := fixture.work.CreateAndAssign(
		ctx,
		work.WorkItemAssignmentInput{
			WorkItemID:      "work-after-prepared-snapshot",
			Title:           "Advance prepared decision view",
			RunID:           "run-after-prepared-snapshot",
			AgentInstanceID: "agent-after-prepared-snapshot",
			CorrelationID:   "44444444-4444-4444-8444-444444444444",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := fixture.projection.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	currentView := fixture.projection.GlobalReadView().Version()
	if currentView == oldCommand.ViewVersion {
		t.Fatal("Journal advance did not change the authoritative view")
	}

	current, err := fixture.backend.ListMissionDecisionCommands(ctx)
	if err != nil || len(current) != 1 {
		t.Fatalf("current commands = %#v, %v", current, err)
	}
	if current[0].ViewVersion != currentView {
		t.Fatalf(
			"prepared command view = %s, authoritative view = %s",
			current[0].ViewVersion,
			currentView,
		)
	}

	oldCommand.Operation = "submit"
	oldCommand.Action = "deny"
	oldCommand.CorrelationID = "55555555-5555-4555-8555-555555555555"
	if _, err := fixture.service.DecideMission(ctx, oldCommand); !errors.Is(
		err,
		ErrMissionDecisionConflict,
	) {
		t.Fatalf("pre-rebind command error = %v", err)
	}

	newCommand := current[0]
	newCommand.Operation = "submit"
	newCommand.Action = "deny"
	newCommand.CorrelationID = "66666666-6666-4666-8666-666666666666"
	result, err := fixture.service.DecideMission(ctx, newCommand)
	if err != nil || result.Status != "denied" || !result.Authoritative {
		t.Fatalf("current command result = %#v, %v", result, err)
	}
}

func TestPreparedMissionDecisionConcurrentSubmissionHasOneWinner(t *testing.T) {
	blocking := &blockingApprovalAuthority{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	fixture := newPreparedApprovalFixture(t, blocking)
	command := missionDecisionCommandFromSheet(
		fixture.sheet,
		"submit",
		"allow_once",
	)
	type outcome struct {
		result MissionDecisionResult
		err    error
	}
	first := make(chan outcome, 1)
	go func() {
		result, err := fixture.service.DecideMission(
			context.Background(),
			command,
		)
		first <- outcome{result: result, err: err}
	}()
	<-blocking.started
	second := command
	second.CorrelationID = "22222222-2222-4222-8222-222222222222"
	_, secondErr := fixture.service.DecideMission(
		context.Background(),
		second,
	)
	close(blocking.release)
	firstOutcome := <-first
	if firstOutcome.err != nil || !firstOutcome.result.Authoritative {
		t.Fatalf("winner = %#v", firstOutcome)
	}
	if !errors.Is(secondErr, ErrMissionDecisionConflict) {
		t.Fatalf("loser error = %v", secondErr)
	}
	if blocking.calls.Load() != 1 {
		t.Fatalf("authority calls = %d", blocking.calls.Load())
	}
}

func TestPreparedMissionDecisionRejectsReplayAfterViewAdvances(t *testing.T) {
	fixture := newPreparedApprovalFixture(t, nil)
	command := missionDecisionCommandFromSheet(
		fixture.sheet,
		"submit",
		"allow_once",
	)
	first, err := fixture.service.DecideMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.service.DecideMission(
		context.Background(),
		command,
	)
	if !errors.Is(err, ErrMissionDecisionConflict) {
		t.Fatalf(
			"stale replay after view %s error = %v",
			first.ViewVersion,
			err,
		)
	}
}

func TestPreparedReviewDelegatesExactWorkAcceptanceInput(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	input := prepareAcceptedTeamNodeInput(t, fixture)
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	sheet := missionWorkDecisionSheet(
		"review",
		fixture,
		input.DeterministicResult.Input().ClaimGeneration,
		[]string{"not_now", "request_changes", "accept_result"},
	)
	sheet.DecisionDigest = input.Decision.Digest()
	refresh := projectionMissionDecisionRefresh(fixture.readModel)
	backend, err := NewPreparedMissionDecisionBackend(
		PreparedMissionDecisions{
			Reviews: []PreparedReviewDecision{{
				Sheet:        sheet,
				Authority:    fixture.work,
				AcceptResult: &input,
				Refresh:      refresh,
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductDecisionService(
		MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	unprepared := missionDecisionCommandFromSheet(
		sheet,
		"submit",
		"request_changes",
	)
	if _, err := service.DecideMission(
		context.Background(),
		unprepared,
	); !errors.Is(err, ErrMissionDecisionConflict) {
		t.Fatalf("unprepared review action error = %v", err)
	}
	command := missionDecisionCommandFromSheet(
		sheet,
		"submit",
		"accept_result",
	)
	result, err := service.DecideMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "accepted" ||
		!result.Authoritative ||
		result.ViewVersion == sheet.ViewVersion {
		t.Fatalf("review result = %#v", result)
	}
	team, err := fixture.work.TeamExecution(
		context.Background(),
		fixture.plan.TeamInstanceID(),
	)
	if err != nil || team.Status() != "succeeded" {
		t.Fatalf("accepted Team = %#v, %v", team, err)
	}
}

func TestPreparedReviewRejectsUnboundDecisionDigest(t *testing.T) {
	fixture := newTeamRecoveryFixture(t)
	input := prepareAcceptedTeamNodeInput(t, fixture)
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	sheet := missionWorkDecisionSheet(
		"review",
		fixture,
		input.DeterministicResult.Input().ClaimGeneration,
		[]string{"not_now", "request_changes", "accept_result"},
	)
	_, err := NewPreparedMissionDecisionBackend(
		PreparedMissionDecisions{
			Reviews: []PreparedReviewDecision{{
				Sheet:        sheet,
				Authority:    fixture.work,
				AcceptResult: &input,
				Refresh: projectionMissionDecisionRefresh(
					fixture.readModel,
				),
			}},
		},
	)
	if !errors.Is(err, ErrInvalidMissionDecision) {
		t.Fatalf("unbound review digest error = %v", err)
	}
}

func TestPreparedRecoveryDelegatesExactWorkRecoveryInput(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:          1,
		AttemptCredits:   1,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	input := prepareFailedTeamRecoveryInput(t, fixture)
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	sheet := missionWorkDecisionSheet(
		"recovery",
		fixture,
		1,
		[]string{"not_now", "stop_mission", "edit_scope", "start_new_attempt"},
	)
	sheet.DecisionDigest = input.Decision.Digest()
	attempt := currentTeamAttempt(t, fixture, "main")
	refresh := projectionMissionDecisionRefresh(fixture.readModel)
	backend, err := NewPreparedMissionDecisionBackend(
		PreparedMissionDecisions{
			Recoveries: []PreparedRecoveryDecision{{
				Sheet:           sheet,
				Attempt:         attempt,
				Authority:       fixture.work,
				StartNewAttempt: &input,
				Refresh:         refresh,
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductDecisionService(
		MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	command := missionDecisionCommandFromSheet(
		sheet,
		"submit",
		"start_new_attempt",
	)
	result, err := service.DecideMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "recovery_started" ||
		!result.Authoritative ||
		result.ViewVersion == sheet.ViewVersion {
		t.Fatalf("recovery result = %#v", result)
	}
	team, err := fixture.work.TeamExecution(
		context.Background(),
		fixture.plan.TeamInstanceID(),
	)
	if err != nil ||
		team.Nodes()[0].Status() != "retry_scheduled" {
		t.Fatalf("recovered Team = %#v, %v", team, err)
	}
}

func TestPreparedRecoveryRejectsUnboundDecisionAndClaim(t *testing.T) {
	fixture := newTeamRecoveryFixtureWithMaxAttempts(t, 2)
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:          1,
		AttemptCredits:   1,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.request.Semantics[0].RecoveryPolicy = policy
	input := prepareFailedTeamRecoveryInput(t, fixture)
	if err := fixture.readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	attempt := currentTeamAttempt(t, fixture, "main")
	base := missionWorkDecisionSheet(
		"recovery",
		fixture,
		attempt.ClaimGeneration(),
		[]string{"not_now", "stop_mission", "edit_scope", "start_new_attempt"},
	)
	for _, test := range []struct {
		name   string
		mutate func(*MissionDecisionSheet)
	}{
		{
			name: "arbitrary digest",
			mutate: func(sheet *MissionDecisionSheet) {
				sheet.DecisionDigest = strings.Repeat("f", 64)
			},
		},
		{
			name: "stale claim generation",
			mutate: func(sheet *MissionDecisionSheet) {
				sheet.DecisionDigest = input.Decision.Digest()
				sheet.ClaimGeneration++
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sheet := base
			test.mutate(&sheet)
			_, err := NewPreparedMissionDecisionBackend(
				PreparedMissionDecisions{
					Recoveries: []PreparedRecoveryDecision{{
						Sheet:           sheet,
						Attempt:         attempt,
						Authority:       fixture.work,
						StartNewAttempt: &input,
						Refresh: projectionMissionDecisionRefresh(
							fixture.readModel,
						),
					}},
				},
			)
			if !errors.Is(err, ErrInvalidMissionDecision) {
				t.Fatalf("unbound recovery error = %v", err)
			}
		})
	}
}

func currentTeamAttempt(
	t testing.TB,
	fixture *teamRecoveryFixture,
	logicalNodeID string,
) work.TeamAttemptRecord {
	t.Helper()
	record, err := fixture.work.TeamExecution(
		context.Background(),
		fixture.plan.TeamInstanceID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range record.Nodes() {
		if node.LogicalNodeID() != logicalNodeID {
			continue
		}
		attempts := node.Attempts()
		if len(attempts) == 0 {
			t.Fatalf("node %q has no attempts", logicalNodeID)
		}
		return attempts[len(attempts)-1]
	}
	t.Fatalf("missing node %q", logicalNodeID)
	return work.TeamAttemptRecord{}
}

func prepareAcceptedTeamNodeInput(
	t testing.TB,
	fixture *teamRecoveryFixture,
) work.TeamNodeAcceptanceInput {
	t.Helper()
	outcome, receipt, classification := executePreparedTeamAttempt(t, fixture)
	semantics := fixture.request.Semantics[0]
	summary := receipt.OutputSummary()
	result, err := verification.VerifyDeterministic(
		semantics.AcceptanceContract,
		verification.DeterministicVerificationInput{
			TeamInstanceID:             fixture.plan.TeamInstanceID(),
			PlanDigest:                 fixture.plan.Digest(),
			LogicalNodeID:              outcome.task.logicalNodeID,
			AttemptNumber:              outcome.task.attemptNumber,
			WorkItemID:                 outcome.task.generation.WorkItemID,
			RunID:                      outcome.task.generation.RunID,
			ClaimID:                    outcome.task.generation.ClaimID,
			ClaimGeneration:            outcome.task.generation.ClaimGeneration,
			SourceEvidenceID:           receipt.EvidenceID(),
			SourceEvidenceDigest:       receipt.Digest(),
			OutputSummaryDigest:        summary.Digest(),
			OutputContractVersion:      semantics.OutputContract.Version(),
			OutputContractDigest:       semantics.OutputContract.Digest(),
			OutputClassification:       classification.Kind(),
			OutputClassificationDigest: classification.Digest(),
			AcceptanceContractDigest:   semantics.AcceptanceContract.Digest(),
			TerminalStatus:             summary.TerminalStatus(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := verification.DecideAcceptance(
		verification.AcceptanceDecisionInput{
			Contract:            semantics.AcceptanceContract,
			DeterministicResult: result,
			DecisionTime:        fixture.request.AuthoritativeTime,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return work.TeamNodeAcceptanceInput{
		TeamInstanceID:      fixture.plan.TeamInstanceID(),
		PlanDigest:          fixture.plan.Digest(),
		LogicalNodeID:       outcome.task.logicalNodeID,
		AttemptNumber:       outcome.task.attemptNumber,
		SourceReceipt:       receipt,
		AcceptanceContract:  semantics.AcceptanceContract,
		DeterministicResult: result,
		Decision:            decision,
		RecoveryPolicy:      semantics.RecoveryPolicy,
		MaxAttempts:         fixture.plan.Nodes()[0].MaxAttempts(),
		CreditsBefore:       semantics.RecoveryPolicy.AttemptCredits(),
		CorrelationID:       fixture.request.CorrelationID,
	}
}

func prepareFailedTeamRecoveryInput(
	t testing.TB,
	fixture *teamRecoveryFixture,
) work.TeamRecoveryInput {
	t.Helper()
	fixture.request.Nodes[0].Executor = newTeamCanarySupervisor(
		t,
		fixture.work,
		fixture.grants,
		&teamCanaryAdapter{
			barrier:        &teamCanaryBarrier{release: make(chan struct{})},
			runtimeID:      "runtime-recovery",
			terminalStatus: "failed",
			terminalReason: "controlled_failure",
		},
	)
	outcome, receipt, classification := executePreparedTeamAttempt(t, fixture)
	team, err := fixture.work.TeamExecution(
		context.Background(),
		fixture.plan.TeamInstanceID(),
	)
	if err != nil {
		t.Fatal(err)
	}
	node := team.Nodes()[0]
	decision, err := rules.DecideRecovery(
		fixture.request.Semantics[0].RecoveryPolicy,
		rules.RecoveryInput{
			TeamInstanceID:       fixture.plan.TeamInstanceID(),
			PlanDigest:           fixture.plan.Digest(),
			LogicalNodeID:        outcome.task.logicalNodeID,
			AttemptNumber:        outcome.task.attemptNumber,
			MaxAttempts:          fixture.plan.Nodes()[0].MaxAttempts(),
			AgentInstanceID:      outcome.task.generation.AgentInstanceID,
			RuntimeInstanceID:    outcome.task.generation.RuntimeInstanceID,
			EvidenceID:           receipt.EvidenceID(),
			EvidenceDigest:       receipt.Digest(),
			OutputSummaryDigest:  receipt.OutputSummary().Digest(),
			Classification:       classification,
			RemainingCredits:     1,
			FallbackConsumed:     false,
			DecisionTime:         fixture.request.AuthoritativeTime,
			Trigger:              rules.RecoveryTriggerOutput,
			PriorClassifications: nil,
		},
	)
	if err != nil {
		t.Fatalf("DecideRecovery() node=%#v classification=%s error=%v", node, classification.Kind(), err)
	}
	return work.TeamRecoveryInput{
		Decision:      decision,
		CorrelationID: fixture.request.CorrelationID,
	}
}

func executePreparedTeamAttempt(
	t testing.TB,
	fixture *teamRecoveryFixture,
) (teamTaskOutcome, evidence.AttemptReceipt, verification.Classification) {
	t.Helper()
	ctx := context.Background()
	tasks := fixture.dispatchAndPrepare(t)
	outcomes := executeTeamTasks(ctx, tasks)
	if len(outcomes) != 1 {
		t.Fatalf("outcomes = %d", len(outcomes))
	}
	outcome := outcomes[0]
	terminalStatus := outcome.outcome.Run().TerminalStatus()
	receipt, err := fixture.artifacts.FinalizeAttemptCapture(
		ctx,
		outcome.task.evidenceID,
		evidence.AttemptTerminal{
			Status: terminalStatus,
			Reason: outcome.outcome.Run().TerminalReason(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	semantics := fixture.request.Semantics[0]
	summary := receipt.OutputSummary()
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
	classification, err := verification.Classify(
		semantics.OutputContract,
		observation,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.work.CommitTeamAttemptEvidence(
		ctx,
		work.TeamAttemptEvidenceInput{
			TeamInstanceID:    fixture.plan.TeamInstanceID(),
			PlanDigest:        fixture.plan.Digest(),
			LogicalNodeID:     outcome.task.logicalNodeID,
			AttemptNumber:     outcome.task.attemptNumber,
			WorkItemID:        outcome.task.generation.WorkItemID,
			RunID:             outcome.task.generation.RunID,
			ClaimID:           outcome.task.generation.ClaimID,
			ClaimGeneration:   outcome.task.generation.ClaimGeneration,
			RuntimeInstanceID: outcome.task.generation.RuntimeInstanceID,
			AgentInstanceID:   outcome.task.generation.AgentInstanceID,
			Receipt:           receipt,
			Classification:    classification,
			CorrelationID:     fixture.request.CorrelationID,
		},
	); err != nil {
		t.Fatal(err)
	}
	return outcome, receipt, classification
}

func missionWorkDecisionSheet(
	kind string,
	fixture *teamRecoveryFixture,
	generation int64,
	actions []string,
) MissionDecisionSheet {
	preparedActions := []string{"accept_result"}
	if kind == "recovery" {
		preparedActions = []string{"start_new_attempt"}
	}
	return MissionDecisionSheet{
		SchemaVersion:    1,
		Kind:             kind,
		MissionID:        "mission/" + fixture.plan.TeamInstanceID(),
		TeamInstanceID:   fixture.plan.TeamInstanceID(),
		ViewVersion:      fixture.readModel.GlobalReadView().Version(),
		DecisionID:       kind + "-decision-1",
		DecisionDigest:   strings.Repeat("f", 64),
		Title:            "Mission decision required",
		Summary:          "Review the exact prepared Work authority input.",
		Requester:        "Main Agent",
		Target:           "Team node",
		CommandType:      "Journal authority",
		NetworkAccess:    "none",
		CredentialAccess: "none",
		PermissionScope:  "this Mission",
		AttemptScope:     "Attempt 1",
		ExpectedEvidence: "accepted Evidence",
		TechnicalDetails: []string{},
		Actions:          actions,
		PreparedActions:  preparedActions,
		Prepared:         true,
		LogicalNodeID:    "main",
		AttemptNumber:    1,
		ClaimGeneration:  generation,
	}
}

func projectionMissionDecisionRefresh(
	readModel *projection.Projection,
) MissionDecisionViewRefreshFunc {
	return func(ctx context.Context) (string, error) {
		if err := readModel.Rebuild(ctx); err != nil {
			return "", err
		}
		return readModel.GlobalReadView().Version(), nil
	}
}

func newPreparedApprovalFixture(
	t testing.TB,
	override ApprovalDecisionAuthority,
) *preparedApprovalFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	correlationID := "11111111-1111-4111-8111-111111111111"
	db := openTeamCanaryDB(t)
	store := journal.NewStore(db)
	clock := &missionDecisionTestClock{now: now}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(bytes.Repeat([]byte{0x55}, 1024)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := workAuthority.CreateAndAssign(
		ctx,
		work.WorkItemAssignmentInput{
			WorkItemID:      "work-approval-sheet",
			Title:           "Approval sheet fixture",
			RunID:           "run-approval-sheet",
			AgentInstanceID: "agent-approval-sheet",
			CorrelationID:   correlationID,
		},
	); err != nil {
		t.Fatal(err)
	}
	authorizer := &missionDecisionTestAuthorizer{now: now}
	authority, err := rules.NewAuthority(store, authorizer, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := rules.NewScope("work_item", "work-approval-sheet")
	if err != nil {
		t.Fatal(err)
	}
	condition, err := rules.NewCondition("start_run", "high")
	if err != nil {
		t.Fatal(err)
	}
	effect, err := rules.NewEffect(
		"require_approval",
		"",
		[]string{"local-owner"},
		5*time.Minute,
		"reject",
	)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := rules.NewRule("approve-start", condition, effect)
	if err != nil {
		t.Fatal(err)
	}
	ruleSet, err := rules.NewRuleSet(scope, 1, []rules.Rule{rule})
	if err != nil {
		t.Fatal(err)
	}
	activation, err := rules.NewRuleSetActivationRequest(
		ruleSet,
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authority.ActivateRuleSet(ctx, activation, correlationID); err != nil {
		t.Fatalf("ActivateRuleSet() error = %v", err)
	}
	action, err := rules.NewActionContext(rules.ActionContextInput{
		ProjectID:       "project.phase2a",
		TeamInstanceID:  "team-1",
		WorkPackageID:   "package-1",
		WorkItemID:      "work-approval-sheet",
		RunID:           "run-approval-sheet",
		AgentInstanceID: "agent-approval-sheet",
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		Action:          "start_run",
		Risk:            "high",
		ClaimGeneration: 0,
		ContractDigest:  strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatalf("NewActionContext() error = %v", err)
	}
	decision, err := rules.Evaluate([]rules.RuleSet{ruleSet}, action)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	pending, err := authority.RequestApproval(
		ctx,
		rules.ApprovalRequestInput{
			Context:            action,
			ContinuationDigest: strings.Repeat("e", 64),
			Decision:           decision,
			RequestedAt:        now,
			CorrelationID:      correlationID,
		},
	)
	if err != nil {
		t.Fatalf("RequestApproval() error = %v", err)
	}
	approved, err := rules.NewApprovalDecisionRequest(
		pending.ID(),
		pending.Digest(),
		"approved",
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatalf("approved NewApprovalDecisionRequest() error = %v", err)
	}
	denied, err := rules.NewApprovalDecisionRequest(
		pending.ID(),
		pending.Digest(),
		"rejected",
		[]byte("opaque-local-authorization"),
	)
	if err != nil {
		t.Fatalf("denied NewApprovalDecisionRequest() error = %v", err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	sheet := MissionDecisionSheet{
		SchemaVersion:    1,
		Kind:             "authorization",
		MissionID:        "mission/team-1",
		TeamInstanceID:   "team-1",
		ViewVersion:      readModel.GlobalReadView().Version(),
		DecisionID:       pending.ID(),
		DecisionDigest:   pending.Digest(),
		Title:            "Authorization required",
		Summary:          "Review the prepared Rules command.",
		Requester:        "Main Agent",
		Target:           "workspace",
		CommandType:      "local process",
		NetworkAccess:    "none",
		CredentialAccess: "none",
		PermissionScope:  "this Mission",
		AttemptScope:     "Attempt 1",
		ExpectedEvidence: "terminal Evidence",
		TechnicalDetails: []string{},
		Actions:          []string{"not_now", "deny", "edit_scope", "allow_once"},
		PreparedActions:  []string{"deny", "allow_once"},
		Prepared:         true,
		LogicalNodeID:    "main",
		AttemptNumber:    1,
		ClaimGeneration:  pending.ClaimGeneration(),
	}
	decisionAuthority := ApprovalDecisionAuthority(authority)
	if override != nil {
		if blocking, ok := override.(*blockingApprovalAuthority); ok {
			blocking.inner = authority
		}
		decisionAuthority = override
	}
	refresh := MissionDecisionViewRefreshFunc(func(ctx context.Context) (string, error) {
		if err := readModel.Rebuild(ctx); err != nil {
			return "", err
		}
		return readModel.GlobalReadView().Version(), nil
	})
	backend, err := NewPreparedMissionDecisionBackend(
		PreparedMissionDecisions{
			Authorizations: []PreparedAuthorizationDecision{{
				Sheet:     sheet,
				Pending:   pending,
				Context:   action,
				Decision:  decision,
				Authority: decisionAuthority,
				Deny:      denied,
				AllowOnce: approved,
				Refresh:   refresh,
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductDecisionService(
		MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	return &preparedApprovalFixture{
		backend:    backend,
		service:    service,
		authority:  authority,
		work:       workAuthority,
		projection: readModel,
		pending:    pending,
		context:    action,
		decision:   decision,
		approved:   approved,
		denied:     denied,
		sheet:      sheet,
	}
}
