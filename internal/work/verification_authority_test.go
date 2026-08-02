package work

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
)

func TestCommitTeamNodeAcceptanceLowRiskIsTerminalOnce(t *testing.T) {
	store := openAuthorityStore(t)
	clock := &mutableClock{now: testNow}
	authority := newAuthority(t, store, clock, 0xa1)
	seedRuntime(t, store, "runtime-a", "online", 1)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-acceptance-low",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Accept controlled output",
			AgentInstanceID:   "agent-main",
			RuntimeInstanceID: "runtime-a",
			Role:              teams.ExecutionRoleMain,
			MaxAttempts:       2,
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
		WorkItemID:        run.WorkItemID(),
		RunID:             run.ID(),
		ClaimID:           run.ClaimID(),
		ClaimGeneration:   run.ClaimGeneration(),
		RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID:   run.AgentInstanceID(),
		CorrelationID:     testCorrelation,
	}
	if _, _, err := authority.Start(context.Background(), generation); err != nil {
		t.Fatal(err)
	}
	if _, _, err := authority.CommitTerminal(
		context.Background(),
		RunTerminalInput{
			RunGenerationInput: generation,
			Status:             "succeeded",
		},
	); err != nil {
		t.Fatal(err)
	}
	receipt, classification := testTeamAttemptReceiptAndClassification(
		t,
		plan,
		"main",
		1,
		generation,
		"succeeded",
	)
	team, err := authority.CommitTeamAttemptEvidence(
		context.Background(),
		TeamAttemptEvidenceInput{
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
			Receipt:           receipt,
			Classification:    classification,
			CorrelationID:     testCorrelation,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := team.Nodes()[0]; got.Status() != "ready_for_review" ||
		got.DependencySatisfied() {
		t.Fatalf("pre-acceptance node = %#v", got)
	}
	contract, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		verification.AcceptanceRiskLow,
	)
	if err != nil {
		t.Fatal(err)
	}
	outputContract, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputInvalid,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := verification.VerifyDeterministic(
		contract,
		verification.DeterministicVerificationInput{
			TeamInstanceID:             plan.TeamInstanceID(),
			PlanDigest:                 plan.Digest(),
			LogicalNodeID:              "main",
			AttemptNumber:              1,
			WorkItemID:                 generation.WorkItemID,
			RunID:                      generation.RunID,
			ClaimID:                    generation.ClaimID,
			ClaimGeneration:            generation.ClaimGeneration,
			SourceEvidenceID:           receipt.EvidenceID(),
			SourceEvidenceDigest:       receipt.Digest(),
			OutputSummaryDigest:        receipt.OutputSummary().Digest(),
			OutputContractVersion:      outputContract.Version(),
			OutputContractDigest:       outputContract.Digest(),
			OutputClassification:       classification.Kind(),
			OutputClassificationDigest: classification.Digest(),
			AcceptanceContractDigest:   contract.Digest(),
			TerminalStatus:             "succeeded",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := verification.DecideAcceptance(
		verification.AcceptanceDecisionInput{
			Contract:            contract,
			DeterministicResult: result,
			DecisionTime:        testNow,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:          1,
		RetryDelay:       time.Minute,
		AttemptCredits:   1,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	input := TeamNodeAcceptanceInput{
		TeamInstanceID:      plan.TeamInstanceID(),
		PlanDigest:          plan.Digest(),
		LogicalNodeID:       "main",
		AttemptNumber:       1,
		SourceReceipt:       receipt,
		AcceptanceContract:  contract,
		DeterministicResult: result,
		Decision:            decision,
		RecoveryPolicy:      policy,
		MaxAttempts:         2,
		CreditsBefore:       1,
		CorrelationID:       testCorrelation,
	}
	authorityNow := testNow.Add(2 * time.Minute)
	clock.Set(authorityNow)
	authoritativeDecision, err := verification.DecideAcceptance(
		verification.AcceptanceDecisionInput{
			Contract:            contract,
			DeterministicResult: result,
			DecisionTime:        authorityNow,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	wait.Add(2)
	results := make(chan TeamExecutionRecord, 2)
	failures := make(chan error, 2)
	for range 2 {
		go func() {
			defer wait.Done()
			accepted, acceptErr :=
				authority.CommitTeamNodeAcceptance(
					context.Background(),
					input,
				)
			if acceptErr != nil {
				failures <- acceptErr
				return
			}
			results <- accepted
		}()
	}
	wait.Wait()
	close(results)
	close(failures)
	if len(results) == 0 {
		state, stateErr := authority.readStateFor(
			context.Background(),
			[]string{
				workItemStream(generation.WorkItemID),
				runStream(generation.RunID),
			},
		)
		t.Fatalf(
			"acceptance errors=%v state_error=%v work=%#v run=%#v node=%#v",
			len(failures),
			stateErr,
			state.workItems[generation.WorkItemID],
			state.runs[generation.RunID],
			team.Nodes()[0],
		)
	}
	for failure := range failures {
		if !errors.Is(failure, ErrWorkItemAcceptanceConflict) {
			t.Fatalf("concurrent acceptance error = %v", failure)
		}
	}
	accepted := <-results
	if accepted.Status() != "succeeded" ||
		accepted.Nodes()[0].Status() != "succeeded" ||
		!accepted.Nodes()[0].DependencySatisfied() {
		t.Fatalf("accepted Team = %#v", accepted)
	}
	before, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	clock.Set(authorityNow.Add(time.Minute))
	input.Decision = verification.AcceptanceDecision{}
	exact, err := authority.CommitTeamNodeAcceptance(
		context.Background(),
		input,
	)
	if err != nil || exact.Status() != "succeeded" {
		t.Fatalf("exact acceptance replay = %#v, %v", exact, err)
	}
	after, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("exact replay appended %d Events", len(after)-len(before))
	}
	var verificationFacts, doneFacts int
	for _, event := range after {
		switch event.Type {
		case "WorkItemVerificationCommitted":
			verificationFacts++
			assertExactJSON(t, event.PayloadJSON, map[string]any{
				"team_instance_id":               plan.TeamInstanceID(),
				"plan_digest":                    plan.Digest(),
				"logical_node_id":                "main",
				"attempt_number":                 1,
				"work_item_id":                   generation.WorkItemID,
				"run_id":                         generation.RunID,
				"claim_id":                       generation.ClaimID,
				"claim_generation":               generation.ClaimGeneration,
				"source_evidence_id":             receipt.EvidenceID(),
				"source_evidence_digest":         receipt.Digest(),
				"output_summary_digest":          receipt.OutputSummary().Digest(),
				"output_contract_version":        outputContract.Version(),
				"output_contract_digest":         outputContract.Digest(),
				"output_classification":          string(classification.Kind()),
				"output_classification_digest":   classification.Digest(),
				"acceptance_contract_version":    contract.Version(),
				"acceptance_contract_digest":     contract.Digest(),
				"risk":                           string(contract.Risk()),
				"deterministic_result_digest":    result.Digest(),
				"verifier_required":              false,
				"verifier_work_item_id":          "",
				"verifier_run_id":                "",
				"verifier_claim_id":              "",
				"verifier_claim_generation":      0,
				"verifier_runtime_instance_id":   "",
				"verifier_agent_instance_id":     "",
				"verifier_grant_id":              "",
				"verifier_evidence_id":           "",
				"verifier_evidence_digest":       "",
				"verifier_output_summary_digest": "",
				"verifier_candidate_kind":        "",
				"verifier_reason_code":           "",
				"verifier_candidate_digest":      "",
				"acceptance_decision_kind":       string(authoritativeDecision.Kind()),
				"acceptance_decision_digest":     authoritativeDecision.Digest(),
				"decided_at":                     authorityNow.Format(time.RFC3339Nano),
			})
		case "WorkItemDone":
			doneFacts++
			assertExactJSON(t, event.PayloadJSON, map[string]any{
				"work_item_id":               generation.WorkItemID,
				"run_id":                     generation.RunID,
				"claim_generation":           generation.ClaimGeneration,
				"status":                     "done",
				"verification_event_id":      event.CausationID,
				"acceptance_decision_digest": authoritativeDecision.Digest(),
				"source_evidence_id":         receipt.EvidenceID(),
				"source_evidence_digest":     receipt.Digest(),
				"verifier_evidence_id":       "",
				"verifier_evidence_digest":   "",
			})
		case "TeamNodeAcceptanceCommitted":
			assertExactJSON(t, event.PayloadJSON, map[string]any{
				"team_instance_id":            plan.TeamInstanceID(),
				"plan_digest":                 plan.Digest(),
				"logical_node_id":             "main",
				"attempt_number":              1,
				"work_item_id":                generation.WorkItemID,
				"work_outcome_event_id":       event.CausationID,
				"acceptance_contract_version": contract.Version(),
				"acceptance_contract_digest":  contract.Digest(),
				"acceptance_decision_kind":    string(authoritativeDecision.Kind()),
				"acceptance_decision_digest":  authoritativeDecision.Digest(),
				"decided_at":                  authorityNow.Format(time.RFC3339Nano),
				"node_status":                 "succeeded",
				"dependency_satisfied":        true,
				"recovery_trigger":            "",
				"recovery_policy_version":     0,
				"recovery_policy_digest":      "",
				"credits_before":              0,
			})
		}
		if strings.Contains(string(event.PayloadJSON), "authorized output") {
			t.Fatal("raw output entered Journal")
		}
	}
	if verificationFacts != 1 || doneFacts != 1 {
		t.Fatalf(
			"verification facts=%d done facts=%d",
			verificationFacts,
			doneFacts,
		)
	}
}
