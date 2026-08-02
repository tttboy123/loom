package work

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/verification"
)

var (
	ErrInvalidWorkItemAcceptance       = errors.New("invalid WorkItem acceptance")
	ErrWorkItemAcceptanceConflict      = errors.New("WorkItem acceptance conflict")
	ErrWorkItemAlreadyAccepted         = errors.New("WorkItem already accepted")
	ErrIndependentVerificationRequired = errors.New("independent verification required")
	ErrVerifierLineageMismatch         = errors.New("verifier lineage mismatch")
)

type TeamNodeAcceptanceInput struct {
	TeamInstanceID      string
	PlanDigest          string
	LogicalNodeID       string
	AttemptNumber       int
	SourceReceipt       evidence.AttemptReceipt
	AcceptanceContract  verification.AcceptanceContract
	DeterministicResult verification.DeterministicVerificationResult
	VerifierCandidate   verification.VerifierCandidate
	VerifierReceipt     evidence.AttemptReceipt
	Decision            verification.AcceptanceDecision
	RecoveryPolicy      AcceptanceRecoveryPolicy
	MaxAttempts         int
	CreditsBefore       int
	CorrelationID       string
}

type AcceptanceRecoveryPolicy interface {
	Valid() bool
	Version() int
	Digest() string
	AttemptCredits() int
}

type VerifierEvidenceInput struct {
	WorkItemID        string
	RunID             string
	ClaimID           string
	ClaimGeneration   int64
	RuntimeInstanceID string
	AgentInstanceID   string
	Receipt           evidence.AttemptReceipt
	CorrelationID     string
}

type workItemVerificationPayload struct {
	TeamInstanceID              string `json:"team_instance_id"`
	PlanDigest                  string `json:"plan_digest"`
	LogicalNodeID               string `json:"logical_node_id"`
	AttemptNumber               int    `json:"attempt_number"`
	WorkItemID                  string `json:"work_item_id"`
	RunID                       string `json:"run_id"`
	ClaimID                     string `json:"claim_id"`
	ClaimGeneration             int64  `json:"claim_generation"`
	SourceEvidenceID            string `json:"source_evidence_id"`
	SourceEvidenceDigest        string `json:"source_evidence_digest"`
	OutputSummaryDigest         string `json:"output_summary_digest"`
	OutputContractVersion       int    `json:"output_contract_version"`
	OutputContractDigest        string `json:"output_contract_digest"`
	OutputClassification        string `json:"output_classification"`
	OutputClassificationDigest  string `json:"output_classification_digest"`
	AcceptanceContractVersion   int    `json:"acceptance_contract_version"`
	AcceptanceContractDigest    string `json:"acceptance_contract_digest"`
	Risk                        string `json:"risk"`
	DeterministicResultDigest   string `json:"deterministic_result_digest"`
	VerifierRequired            bool   `json:"verifier_required"`
	VerifierWorkItemID          string `json:"verifier_work_item_id"`
	VerifierRunID               string `json:"verifier_run_id"`
	VerifierClaimID             string `json:"verifier_claim_id"`
	VerifierClaimGeneration     int64  `json:"verifier_claim_generation"`
	VerifierRuntimeInstanceID   string `json:"verifier_runtime_instance_id"`
	VerifierAgentInstanceID     string `json:"verifier_agent_instance_id"`
	VerifierGrantID             string `json:"verifier_grant_id"`
	VerifierEvidenceID          string `json:"verifier_evidence_id"`
	VerifierEvidenceDigest      string `json:"verifier_evidence_digest"`
	VerifierOutputSummaryDigest string `json:"verifier_output_summary_digest"`
	VerifierCandidateKind       string `json:"verifier_candidate_kind"`
	VerifierReasonCode          string `json:"verifier_reason_code"`
	VerifierCandidateDigest     string `json:"verifier_candidate_digest"`
	AcceptanceDecisionKind      string `json:"acceptance_decision_kind"`
	AcceptanceDecisionDigest    string `json:"acceptance_decision_digest"`
	DecidedAt                   string `json:"decided_at"`
}

type workItemAcceptanceOutcomePayload struct {
	WorkItemID               string `json:"work_item_id"`
	RunID                    string `json:"run_id"`
	ClaimGeneration          int64  `json:"claim_generation"`
	Status                   string `json:"status"`
	VerificationEventID      string `json:"verification_event_id"`
	AcceptanceDecisionDigest string `json:"acceptance_decision_digest"`
	SourceEvidenceID         string `json:"source_evidence_id,omitempty"`
	SourceEvidenceDigest     string `json:"source_evidence_digest,omitempty"`
	VerifierEvidenceID       string `json:"verifier_evidence_id,omitempty"`
	VerifierEvidenceDigest   string `json:"verifier_evidence_digest,omitempty"`
	RecoveryTrigger          string `json:"recovery_trigger,omitempty"`
	RecoveryPolicyVersion    int    `json:"recovery_policy_version,omitempty"`
	RecoveryPolicyDigest     string `json:"recovery_policy_digest,omitempty"`
	AttemptNumber            int    `json:"attempt_number,omitempty"`
	MaxAttempts              int    `json:"max_attempts,omitempty"`
	CreditsBefore            int    `json:"credits_before,omitempty"`
}

func (payload workItemAcceptanceOutcomePayload) MarshalJSON() ([]byte, error) {
	if payload.Status == "done" {
		return json.Marshal(struct {
			WorkItemID               string `json:"work_item_id"`
			RunID                    string `json:"run_id"`
			ClaimGeneration          int64  `json:"claim_generation"`
			Status                   string `json:"status"`
			VerificationEventID      string `json:"verification_event_id"`
			AcceptanceDecisionDigest string `json:"acceptance_decision_digest"`
			SourceEvidenceID         string `json:"source_evidence_id"`
			SourceEvidenceDigest     string `json:"source_evidence_digest"`
			VerifierEvidenceID       string `json:"verifier_evidence_id"`
			VerifierEvidenceDigest   string `json:"verifier_evidence_digest"`
		}{
			payload.WorkItemID,
			payload.RunID,
			payload.ClaimGeneration,
			payload.Status,
			payload.VerificationEventID,
			payload.AcceptanceDecisionDigest,
			payload.SourceEvidenceID,
			payload.SourceEvidenceDigest,
			payload.VerifierEvidenceID,
			payload.VerifierEvidenceDigest,
		})
	}
	return json.Marshal(struct {
		WorkItemID               string `json:"work_item_id"`
		RunID                    string `json:"run_id"`
		ClaimGeneration          int64  `json:"claim_generation"`
		Status                   string `json:"status"`
		VerificationEventID      string `json:"verification_event_id"`
		AcceptanceDecisionDigest string `json:"acceptance_decision_digest"`
		RecoveryTrigger          string `json:"recovery_trigger"`
		RecoveryPolicyVersion    int    `json:"recovery_policy_version"`
		RecoveryPolicyDigest     string `json:"recovery_policy_digest"`
		AttemptNumber            int    `json:"attempt_number"`
		MaxAttempts              int    `json:"max_attempts"`
		CreditsBefore            int    `json:"credits_before"`
	}{
		payload.WorkItemID,
		payload.RunID,
		payload.ClaimGeneration,
		payload.Status,
		payload.VerificationEventID,
		payload.AcceptanceDecisionDigest,
		payload.RecoveryTrigger,
		payload.RecoveryPolicyVersion,
		payload.RecoveryPolicyDigest,
		payload.AttemptNumber,
		payload.MaxAttempts,
		payload.CreditsBefore,
	})
}

type teamNodeAcceptancePayload struct {
	TeamInstanceID            string `json:"team_instance_id"`
	PlanDigest                string `json:"plan_digest"`
	LogicalNodeID             string `json:"logical_node_id"`
	AttemptNumber             int    `json:"attempt_number"`
	WorkItemID                string `json:"work_item_id"`
	WorkOutcomeEventID        string `json:"work_outcome_event_id"`
	AcceptanceContractVersion int    `json:"acceptance_contract_version"`
	AcceptanceContractDigest  string `json:"acceptance_contract_digest"`
	AcceptanceDecisionKind    string `json:"acceptance_decision_kind"`
	AcceptanceDecisionDigest  string `json:"acceptance_decision_digest"`
	DecidedAt                 string `json:"decided_at"`
	NodeStatus                string `json:"node_status"`
	DependencySatisfied       bool   `json:"dependency_satisfied"`
	RecoveryTrigger           string `json:"recovery_trigger"`
	RecoveryPolicyVersion     int    `json:"recovery_policy_version"`
	RecoveryPolicyDigest      string `json:"recovery_policy_digest"`
	CreditsBefore             int    `json:"credits_before"`
}

func (authority *Authority) CommitVerifierEvidence(
	ctx context.Context,
	input VerifierEvidenceInput,
) error {
	if ctx == nil {
		return ErrInvalidWorkItemAcceptance
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	summary := input.Receipt.OutputSummary()
	if !validOpaqueID(input.WorkItemID) ||
		!validOpaqueID(input.RunID) ||
		!validCanonicalUUID(input.ClaimID) ||
		input.ClaimGeneration < 1 ||
		!validOpaqueID(input.RuntimeInstanceID) ||
		!validOpaqueID(input.AgentInstanceID) ||
		!validOpaqueID(input.Receipt.EvidenceID()) ||
		!validSHA256Hex(input.Receipt.Digest()) ||
		!validSHA256Hex(summary.Digest()) ||
		input.Receipt.EvidenceID() != summary.EvidenceID() ||
		input.Receipt.Digest() != summary.EvidenceDigest() ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidWorkItemAcceptance
	}
	streamIDs := []string{
		workItemStream(input.WorkItemID),
		runStream(input.RunID),
		"evidence/" + input.Receipt.EvidenceID(),
		runtimeStatusStream(input.RuntimeInstanceID),
		runtimeCapacityStream(input.RuntimeInstanceID),
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return err
	}
	evidenceHead := snapshotHead(
		snapshot,
		"evidence/"+input.Receipt.EvidenceID(),
	)
	if evidenceHead.Sequence > 0 {
		if exactSubmittedEvidence(
			snapshot.Events(),
			input.WorkItemID,
			input.RunID,
			input.ClaimGeneration,
			input.CorrelationID,
			input.Receipt,
		) {
			return nil
		}
		return ErrVerifierLineageMismatch
	}
	state, err := replayAuthorityEventsSelective(ctx, snapshot.Events())
	if err != nil {
		return ErrVerifierLineageMismatch
	}
	workItem := state.workItems[input.WorkItemID]
	run := state.runs[input.RunID]
	if workItem.id != input.WorkItemID ||
		run.id != input.RunID ||
		run.workItemID != input.WorkItemID ||
		run.claimID != input.ClaimID ||
		run.claimGeneration != input.ClaimGeneration ||
		run.runtimeInstanceID != input.RuntimeInstanceID ||
		run.agentInstanceID != input.AgentInstanceID ||
		run.phase != "terminal" ||
		summary.TerminalStatus() != run.terminalStatus ||
		(run.terminalStatus == "succeeded" &&
			workItem.status != "ready_for_review") ||
		(run.terminalStatus != "succeeded" &&
			workItem.status != run.terminalStatus) {
		return ErrVerifierLineageMismatch
	}
	now, err := authority.operationTime()
	if err != nil {
		return err
	}
	eventID := deterministicEventID(
		"EvidenceSubmitted",
		input.Receipt.EvidenceID(),
		input.Receipt.Digest(),
		input.RunID,
		fmt.Sprint(input.ClaimGeneration),
	)
	event := newEvent(
		eventID,
		"evidence/"+input.Receipt.EvidenceID(),
		1,
		"EvidenceSubmitted",
		now,
		input.CorrelationID,
		"",
		struct {
			EvidenceID string `json:"evidence_id"`
			WorkItemID string `json:"work_item_id"`
			Digest     string `json:"digest"`
		}{
			input.Receipt.EvidenceID(),
			input.WorkItemID,
			input.Receipt.Digest(),
		},
	)
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID,
			Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectations,
		[]journal.Event{event},
	); err != nil {
		return ErrWorkItemAcceptanceConflict
	}
	return nil
}

func (authority *Authority) CommitTeamNodeAcceptance(
	ctx context.Context,
	input TeamNodeAcceptanceInput,
) (TeamExecutionRecord, error) {
	if err := validateTeamNodeAcceptanceInput(ctx, input); err != nil {
		return TeamExecutionRecord{}, err
	}
	resultInput := input.DeterministicResult.Input()
	streamIDs := []string{
		teamExecutionStream(input.TeamInstanceID),
		workItemStream(resultInput.WorkItemID),
		runStream(resultInput.RunID),
		"evidence/" + input.SourceReceipt.EvidenceID(),
	}
	if input.AcceptanceContract.IndependentVerifierRequired() {
		binding := input.VerifierCandidate.Binding()
		streamIDs = append(streamIDs,
			workItemStream(binding.WorkItemID),
			runStream(binding.RunID),
			"agent-grant/"+binding.RunID,
			"evidence/"+input.VerifierReceipt.EvidenceID(),
			runtimeStatusStream(binding.RuntimeInstanceID),
			runtimeCapacityStream(binding.RuntimeInstanceID),
		)
	}
	snapshot, err := authority.store.ReadStreamSet(ctx, streamIDs)
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	team, err := replayTeamExecution(
		input.TeamInstanceID,
		filterTeamEvents(
			snapshot.Events(),
			teamExecutionStream(input.TeamInstanceID),
		),
	)
	if err != nil || team.status == "" ||
		team.planDigest != input.PlanDigest {
		return TeamExecutionRecord{}, ErrWorkItemAcceptanceConflict
	}
	if team.legacySemanticUnbound || team.legacyAcceptanceUnbound {
		return TeamExecutionRecord{}, ErrLegacyAcceptanceUnbound
	}
	node := teamNodeByID(&team, input.LogicalNodeID)
	attempt := teamAttemptByNumber(node, input.AttemptNumber)
	if node == nil || attempt == nil ||
		node.currentAttempt != input.AttemptNumber ||
		attempt.workItemID != resultInput.WorkItemID ||
		attempt.runID != resultInput.RunID ||
		attempt.claimID != resultInput.ClaimID ||
		attempt.claimGeneration != resultInput.ClaimGeneration ||
		attempt.evidenceID != input.SourceReceipt.EvidenceID() ||
		attempt.evidenceDigest != input.SourceReceipt.Digest() ||
		attempt.outputSummaryDigest !=
			input.SourceReceipt.OutputSummary().Digest() ||
		attempt.outputContractVersion != resultInput.OutputContractVersion ||
		attempt.outputContractDigest != resultInput.OutputContractDigest ||
		string(attempt.outputClassification) !=
			string(resultInput.OutputClassification) ||
		attempt.outputClassificationDigest !=
			resultInput.OutputClassificationDigest ||
		node.semanticBinding.AcceptanceContractVersion !=
			input.AcceptanceContract.Version() ||
		node.semanticBinding.AcceptanceContractDigest !=
			input.AcceptanceContract.Digest() ||
		node.semanticBinding.AcceptanceRisk !=
			string(input.AcceptanceContract.Risk()) ||
		node.semanticBinding.RecoveryPolicyVersion !=
			input.RecoveryPolicy.Version() ||
		node.semanticBinding.RecoveryPolicyDigest !=
			input.RecoveryPolicy.Digest() ||
		input.MaxAttempts != node.maxAttempts ||
		input.CreditsBefore !=
			node.semanticBinding.AttemptCredits-(input.AttemptNumber-1) {
		return TeamExecutionRecord{}, ErrWorkItemAcceptanceConflict
	}
	if !exactSubmittedEvidence(
		snapshot.Events(),
		resultInput.WorkItemID,
		resultInput.RunID,
		resultInput.ClaimGeneration,
		input.CorrelationID,
		input.SourceReceipt,
	) {
		return TeamExecutionRecord{}, ErrWorkItemAcceptanceConflict
	}
	sourceWork, sourceRun, err := replayAcceptanceSourceLineage(
		snapshot.Events(),
		resultInput,
	)
	if err != nil {
		return TeamExecutionRecord{}, ErrWorkItemAcceptanceConflict
	}
	if err := validateVerifierAcceptanceLineage(
		ctx,
		snapshot,
		node,
		input,
	); err != nil {
		return TeamExecutionRecord{}, err
	}
	matched, existing := matchExistingTeamNodeAcceptance(
		snapshot.Events(),
		input,
	)
	if matched {
		return cloneTeamExecutionRecord(team), nil
	}
	if existing || sourceWork.status == "done" {
		return TeamExecutionRecord{}, ErrWorkItemAlreadyAccepted
	}
	if node.status != "ready_for_review" ||
		sourceWork.status != "ready_for_review" ||
		sourceRun.phase != "terminal" ||
		sourceRun.terminalStatus != "succeeded" ||
		sourceRun.claimID != resultInput.ClaimID ||
		sourceRun.claimGeneration != resultInput.ClaimGeneration {
		return TeamExecutionRecord{}, ErrWorkItemAcceptanceConflict
	}
	now, err := authority.operationTime()
	if err != nil {
		return TeamExecutionRecord{}, err
	}
	decision, err := verification.DecideAcceptance(
		verification.AcceptanceDecisionInput{
			Contract:            input.AcceptanceContract,
			DeterministicResult: input.DeterministicResult,
			VerifierCandidate:   input.VerifierCandidate,
			DecisionTime:        now,
		},
	)
	if err != nil {
		return TeamExecutionRecord{}, ErrInvalidWorkItemAcceptance
	}
	input.Decision = decision
	verificationPayload := newWorkItemVerificationPayload(input)
	verificationID := deterministicEventID(
		"WorkItemVerificationCommitted",
		input.TeamInstanceID,
		input.PlanDigest,
		input.LogicalNodeID,
		fmt.Sprint(input.AttemptNumber),
		resultInput.WorkItemID,
		resultInput.RunID,
		fmt.Sprint(resultInput.ClaimGeneration),
		input.SourceReceipt.EvidenceID(),
		input.SourceReceipt.Digest(),
		input.AcceptanceContract.Digest(),
		input.DeterministicResult.Digest(),
		input.VerifierCandidate.Digest(),
		input.Decision.Digest(),
	)
	workHead := snapshotHead(snapshot, workItemStream(resultInput.WorkItemID))
	verificationEvent := newEvent(
		verificationID,
		workItemStream(resultInput.WorkItemID),
		workHead.Sequence+1,
		"WorkItemVerificationCommitted",
		now,
		input.CorrelationID,
		sourceWork.lastEventID,
		verificationPayload,
	)
	outcomeType := "WorkItemDone"
	outcomeStatus := "done"
	if input.Decision.Kind() == verification.AcceptanceRejected {
		outcomeType = "WorkItemRejected"
		outcomeStatus = "rejected"
	}
	outcomePayload := newWorkItemAcceptanceOutcomePayload(
		input,
		verificationID,
		outcomeStatus,
	)
	outcomeID := acceptanceOutcomeEventID(
		input,
		verificationID,
		outcomeStatus,
	)
	outcomeEvent := newEvent(
		outcomeID,
		workItemStream(resultInput.WorkItemID),
		workHead.Sequence+2,
		outcomeType,
		now,
		input.CorrelationID,
		verificationID,
		outcomePayload,
	)
	nodeStatus := "succeeded"
	dependencySatisfied := true
	recoveryTrigger := ""
	recoveryVersion := 0
	recoveryDigest := ""
	creditsBefore := 0
	if input.Decision.Kind() == verification.AcceptanceRejected {
		nodeStatus = "awaiting_recovery"
		dependencySatisfied = false
		recoveryTrigger = teamRecoveryTriggerVerificationRejected
		recoveryVersion = input.RecoveryPolicy.Version()
		recoveryDigest = input.RecoveryPolicy.Digest()
		creditsBefore = input.CreditsBefore
	}
	teamPayload := teamNodeAcceptancePayload{
		TeamInstanceID:            input.TeamInstanceID,
		PlanDigest:                input.PlanDigest,
		LogicalNodeID:             input.LogicalNodeID,
		AttemptNumber:             input.AttemptNumber,
		WorkItemID:                resultInput.WorkItemID,
		WorkOutcomeEventID:        outcomeID,
		AcceptanceContractVersion: input.AcceptanceContract.Version(),
		AcceptanceContractDigest:  input.AcceptanceContract.Digest(),
		AcceptanceDecisionKind:    string(input.Decision.Kind()),
		AcceptanceDecisionDigest:  input.Decision.Digest(),
		DecidedAt:                 now.Format(time.RFC3339Nano),
		NodeStatus:                nodeStatus,
		DependencySatisfied:       dependencySatisfied,
		RecoveryTrigger:           recoveryTrigger,
		RecoveryPolicyVersion:     recoveryVersion,
		RecoveryPolicyDigest:      recoveryDigest,
		CreditsBefore:             creditsBefore,
	}
	teamHead := snapshotHead(
		snapshot,
		teamExecutionStream(input.TeamInstanceID),
	)
	teamEventID := teamAcceptanceEventID(
		input,
		outcomeID,
		nodeStatus,
		dependencySatisfied,
		recoveryTrigger,
		recoveryVersion,
		recoveryDigest,
		creditsBefore,
	)
	teamEvent := newEvent(
		teamEventID,
		teamExecutionStream(input.TeamInstanceID),
		teamHead.Sequence+1,
		"TeamNodeAcceptanceCommitted",
		now,
		input.CorrelationID,
		outcomeID,
		teamPayload,
	)
	events := []journal.Event{verificationEvent, outcomeEvent, teamEvent}
	applyTeamNodeAcceptance(
		&team,
		input.LogicalNodeID,
		input.AttemptNumber,
		input.Decision,
		input.RecoveryPolicy,
		input.CreditsBefore,
	)
	if teamIsTerminal(team) {
		status, reason := aggregateTeamTerminal(team)
		terminalID := deterministicEventID(
			"TeamExecutionTerminal",
			input.TeamInstanceID,
			input.PlanDigest,
			status,
			reason,
		)
		events = append(events, newEvent(
			terminalID,
			teamExecutionStream(input.TeamInstanceID),
			teamHead.Sequence+2,
			"TeamExecutionTerminal",
			now,
			input.CorrelationID,
			teamEventID,
			struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				Status         string `json:"status"`
				Reason         string `json:"reason"`
			}{input.TeamInstanceID, input.PlanDigest, status, reason},
		))
	}
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID,
			Sequence: head.Sequence,
		})
	}
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		expectations,
		events,
	); err != nil {
		return TeamExecutionRecord{}, ErrWorkItemAcceptanceConflict
	}
	return authority.TeamExecution(ctx, input.TeamInstanceID)
}

func validateTeamNodeAcceptanceInput(
	ctx context.Context,
	input TeamNodeAcceptanceInput,
) error {
	if ctx == nil {
		return ErrInvalidWorkItemAcceptance
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validOpaqueID(input.TeamInstanceID) ||
		!validSHA256Hex(input.PlanDigest) ||
		!validOpaqueID(input.LogicalNodeID) ||
		input.AttemptNumber < 1 || input.AttemptNumber > 3 ||
		!input.AcceptanceContract.Valid() ||
		!input.DeterministicResult.Valid() ||
		nilInterface(input.RecoveryPolicy) ||
		!input.RecoveryPolicy.Valid() ||
		input.MaxAttempts < input.AttemptNumber ||
		input.MaxAttempts > 3 ||
		input.CreditsBefore < 0 ||
		input.CreditsBefore > input.RecoveryPolicy.AttemptCredits() ||
		input.SourceReceipt.EvidenceID() == "" ||
		input.SourceReceipt.Digest() == "" ||
		input.SourceReceipt.OutputSummary().Digest() == "" ||
		!validCanonicalUUID(input.CorrelationID) {
		return ErrInvalidWorkItemAcceptance
	}
	resultInput := input.DeterministicResult.Input()
	if resultInput.TeamInstanceID != input.TeamInstanceID ||
		resultInput.PlanDigest != input.PlanDigest ||
		resultInput.LogicalNodeID != input.LogicalNodeID ||
		resultInput.AttemptNumber != input.AttemptNumber ||
		resultInput.SourceEvidenceID != input.SourceReceipt.EvidenceID() ||
		resultInput.SourceEvidenceDigest != input.SourceReceipt.Digest() ||
		resultInput.OutputSummaryDigest !=
			input.SourceReceipt.OutputSummary().Digest() ||
		input.DeterministicResult.Risk() !=
			input.AcceptanceContract.Risk() {
		return ErrInvalidWorkItemAcceptance
	}
	if input.AcceptanceContract.IndependentVerifierRequired() {
		if !input.VerifierCandidate.Valid() ||
			input.VerifierReceipt.EvidenceID() == "" ||
			input.VerifierReceipt.EvidenceID() !=
				input.VerifierCandidate.Binding().EvidenceID ||
			input.VerifierReceipt.Digest() !=
				input.VerifierCandidate.Binding().EvidenceDigest ||
			input.VerifierReceipt.OutputSummary().Digest() !=
				input.VerifierCandidate.Binding().OutputSummaryDigest {
			return ErrIndependentVerificationRequired
		}
	} else if input.VerifierCandidate.Digest() != "" ||
		input.VerifierReceipt.EvidenceID() != "" {
		return ErrVerifierLineageMismatch
	}
	return nil
}

func validateVerifierAcceptanceLineage(
	ctx context.Context,
	snapshot journal.StreamSetSnapshot,
	node *TeamNodeRecord,
	input TeamNodeAcceptanceInput,
) error {
	if !input.AcceptanceContract.IndependentVerifierRequired() {
		return nil
	}
	binding := input.VerifierCandidate.Binding()
	source := input.DeterministicResult.Input()
	expectedWorkItemID := verifierLineageIdentity(
		"verifier-work",
		source.TeamInstanceID,
		source.PlanDigest,
		source.LogicalNodeID,
		fmt.Sprint(source.AttemptNumber),
		source.WorkItemID,
		source.RunID,
		source.SourceEvidenceDigest,
		input.AcceptanceContract.Digest(),
	)
	expectedRunID := verifierLineageIdentity(
		"verifier-run",
		expectedWorkItemID,
		node.semanticBinding.VerifierAgentInstanceID,
		node.semanticBinding.VerifierRuntimeInstanceID,
		node.semanticBinding.VerifierWorkflowPath,
	)
	expectedEvidenceID := verifierLineageIdentity(
		"verifier-evidence",
		expectedWorkItemID,
		expectedRunID,
		source.SourceEvidenceDigest,
		input.AcceptanceContract.Digest(),
	)
	if binding.AgentInstanceID !=
		node.semanticBinding.VerifierAgentInstanceID ||
		binding.RuntimeInstanceID !=
			node.semanticBinding.VerifierRuntimeInstanceID ||
		binding.AgentInstanceID ==
			teamAttemptByNumber(node, input.AttemptNumber).agentInstanceID ||
		binding.WorkItemID != expectedWorkItemID ||
		binding.RunID != expectedRunID ||
		binding.EvidenceID != expectedEvidenceID {
		return ErrVerifierLineageMismatch
	}
	state, err := replayAuthorityEventsSelective(
		ctx,
		filterAcceptanceEvents(
			snapshot.Events(),
			[]string{
				workItemStream(binding.WorkItemID),
				runStream(binding.RunID),
				runtimeStatusStream(binding.RuntimeInstanceID),
				runtimeCapacityStream(binding.RuntimeInstanceID),
			},
		),
	)
	if err != nil {
		return ErrVerifierLineageMismatch
	}
	workRecord := state.workItems[binding.WorkItemID]
	runRecord := state.runs[binding.RunID]
	if workRecord.id != binding.WorkItemID ||
		runRecord.id != binding.RunID ||
		runRecord.workItemID != binding.WorkItemID ||
		runRecord.claimID != binding.ClaimID ||
		runRecord.claimGeneration != binding.ClaimGeneration ||
		runRecord.runtimeInstanceID != binding.RuntimeInstanceID ||
		runRecord.agentInstanceID != binding.AgentInstanceID ||
		runRecord.phase != "terminal" ||
		runRecord.terminalStatus != binding.TerminalStatus ||
		runRecord.terminalReason != binding.TerminalReason ||
		snapshotHead(
			snapshot,
			"evidence/"+binding.EvidenceID,
		).Sequence != 1 {
		return ErrVerifierLineageMismatch
	}
	if !exactSubmittedEvidence(
		snapshot.Events(),
		binding.WorkItemID,
		binding.RunID,
		binding.ClaimGeneration,
		input.CorrelationID,
		input.VerifierReceipt,
	) {
		return ErrVerifierLineageMismatch
	}
	if !exactVerifierGrantAuthorization(
		snapshot.Events(),
		binding,
	) {
		return ErrVerifierLineageMismatch
	}
	return nil
}

func verifierLineageIdentity(label string, fields ...string) string {
	hash := sha256.New()
	writeVerifierLineageField(hash, "loom."+label+".v1")
	for _, field := range fields {
		writeVerifierLineageField(hash, field)
	}
	return label + "-" + hex.EncodeToString(hash.Sum(nil))
}

func writeVerifierLineageField(
	hash interface{ Write([]byte) (int, error) },
	value string,
) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hash.Write(length[:])
	_, _ = hash.Write([]byte(value))
}

func matchExistingTeamNodeAcceptance(
	events []journal.Event,
	input TeamNodeAcceptanceInput,
) (bool, bool) {
	source := input.DeterministicResult.Input()
	existing := false
	for _, event := range events {
		if event.Type != "WorkItemVerificationCommitted" {
			continue
		}
		var payload workItemVerificationPayload
		if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.WorkItemID != source.WorkItemID ||
			payload.TeamInstanceID != input.TeamInstanceID ||
			payload.LogicalNodeID != input.LogicalNodeID ||
			payload.AttemptNumber != input.AttemptNumber {
			continue
		}
		existing = true
		decidedAt, err := time.Parse(time.RFC3339Nano, payload.DecidedAt)
		if err != nil || decidedAt.Location() != time.UTC {
			continue
		}
		decision, err := verification.DecideAcceptance(
			verification.AcceptanceDecisionInput{
				Contract:            input.AcceptanceContract,
				DeterministicResult: input.DeterministicResult,
				VerifierCandidate:   input.VerifierCandidate,
				DecisionTime:        decidedAt,
			},
		)
		if err != nil {
			continue
		}
		resolved := input
		resolved.Decision = decision
		matched, _ := matchExistingTeamNodeAcceptanceDecision(
			events,
			resolved,
		)
		if matched {
			return true, true
		}
	}
	return false, existing
}

func matchExistingTeamNodeAcceptanceDecision(
	events []journal.Event,
	input TeamNodeAcceptanceInput,
) (bool, bool) {
	source := input.DeterministicResult.Input()
	verificationPayload := newWorkItemVerificationPayload(input)
	verificationID := deterministicEventID(
		"WorkItemVerificationCommitted",
		input.TeamInstanceID,
		input.PlanDigest,
		input.LogicalNodeID,
		fmt.Sprint(input.AttemptNumber),
		source.WorkItemID,
		source.RunID,
		fmt.Sprint(source.ClaimGeneration),
		input.SourceReceipt.EvidenceID(),
		input.SourceReceipt.Digest(),
		input.AcceptanceContract.Digest(),
		input.DeterministicResult.Digest(),
		input.VerifierCandidate.Digest(),
		input.Decision.Digest(),
	)
	outcomeType := "WorkItemDone"
	outcomeStatus := "done"
	nodeStatus := "succeeded"
	dependencySatisfied := true
	recoveryTrigger := ""
	recoveryVersion := 0
	recoveryDigest := ""
	creditsBefore := 0
	if input.Decision.Kind() == verification.AcceptanceRejected {
		outcomeType = "WorkItemRejected"
		outcomeStatus = "rejected"
		nodeStatus = "awaiting_recovery"
		dependencySatisfied = false
		recoveryTrigger = teamRecoveryTriggerVerificationRejected
		recoveryVersion = input.RecoveryPolicy.Version()
		recoveryDigest = input.RecoveryPolicy.Digest()
		creditsBefore = input.CreditsBefore
	}
	outcomePayload := newWorkItemAcceptanceOutcomePayload(
		input,
		verificationID,
		outcomeStatus,
	)
	outcomeID := acceptanceOutcomeEventID(
		input,
		verificationID,
		outcomeStatus,
	)
	teamPayload := teamNodeAcceptancePayload{
		TeamInstanceID:            input.TeamInstanceID,
		PlanDigest:                input.PlanDigest,
		LogicalNodeID:             input.LogicalNodeID,
		AttemptNumber:             input.AttemptNumber,
		WorkItemID:                source.WorkItemID,
		WorkOutcomeEventID:        outcomeID,
		AcceptanceContractVersion: input.AcceptanceContract.Version(),
		AcceptanceContractDigest:  input.AcceptanceContract.Digest(),
		AcceptanceDecisionKind:    string(input.Decision.Kind()),
		AcceptanceDecisionDigest:  input.Decision.Digest(),
		DecidedAt:                 input.Decision.DecisionTime().Format(time.RFC3339Nano),
		NodeStatus:                nodeStatus,
		DependencySatisfied:       dependencySatisfied,
		RecoveryTrigger:           recoveryTrigger,
		RecoveryPolicyVersion:     recoveryVersion,
		RecoveryPolicyDigest:      recoveryDigest,
		CreditsBefore:             creditsBefore,
	}
	teamID := teamAcceptanceEventID(
		input,
		outcomeID,
		nodeStatus,
		dependencySatisfied,
		recoveryTrigger,
		recoveryVersion,
		recoveryDigest,
		creditsBefore,
	)
	foundVerification := false
	foundOutcome := false
	foundTeam := false
	existing := false
	for _, event := range events {
		switch event.Type {
		case "WorkItemVerificationCommitted":
			var payload workItemVerificationPayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil {
				continue
			}
			if payload.WorkItemID != source.WorkItemID ||
				payload.TeamInstanceID != input.TeamInstanceID ||
				payload.LogicalNodeID != input.LogicalNodeID ||
				payload.AttemptNumber != input.AttemptNumber {
				continue
			}
			existing = true
			if event.ID == verificationID &&
				event.CorrelationID == input.CorrelationID &&
				reflect.DeepEqual(payload, verificationPayload) {
				foundVerification = true
			}
		case "WorkItemDone", "WorkItemRejected":
			var payload workItemAcceptanceOutcomePayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.WorkItemID != source.WorkItemID {
				continue
			}
			if payload.VerificationEventID != verificationID {
				continue
			}
			existing = true
			if event.Type == outcomeType &&
				event.ID == outcomeID &&
				event.CorrelationID == input.CorrelationID &&
				event.CausationID == verificationID &&
				reflect.DeepEqual(payload, outcomePayload) {
				foundOutcome = true
			}
		case "TeamNodeAcceptanceCommitted":
			var payload teamNodeAcceptancePayload
			if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.TeamInstanceID != input.TeamInstanceID ||
				payload.LogicalNodeID != input.LogicalNodeID ||
				payload.AttemptNumber != input.AttemptNumber {
				continue
			}
			existing = true
			if event.ID == teamID &&
				event.CorrelationID == input.CorrelationID &&
				event.CausationID == outcomeID &&
				reflect.DeepEqual(payload, teamPayload) {
				foundTeam = true
			}
		}
	}
	return foundVerification && foundOutcome && foundTeam, existing
}

func acceptanceOutcomeEventID(
	input TeamNodeAcceptanceInput,
	verificationEventID string,
	status string,
) string {
	source := input.DeterministicResult.Input()
	if status == "done" {
		return deterministicEventID(
			"WorkItemDone",
			source.WorkItemID,
			verificationEventID,
			input.Decision.Digest(),
			input.SourceReceipt.Digest(),
			input.VerifierReceipt.Digest(),
		)
	}
	return deterministicEventID(
		"WorkItemRejected",
		source.WorkItemID,
		source.RunID,
		fmt.Sprint(source.ClaimGeneration),
		status,
		verificationEventID,
		input.Decision.Digest(),
		teamRecoveryTriggerVerificationRejected,
		fmt.Sprint(input.RecoveryPolicy.Version()),
		input.RecoveryPolicy.Digest(),
		fmt.Sprint(input.AttemptNumber),
		fmt.Sprint(input.MaxAttempts),
		fmt.Sprint(input.CreditsBefore),
	)
}

func workItemVerificationPayloadEventID(
	payload workItemVerificationPayload,
) string {
	return deterministicEventID(
		"WorkItemVerificationCommitted",
		payload.TeamInstanceID,
		payload.PlanDigest,
		payload.LogicalNodeID,
		fmt.Sprint(payload.AttemptNumber),
		payload.WorkItemID,
		payload.RunID,
		fmt.Sprint(payload.ClaimGeneration),
		payload.SourceEvidenceID,
		payload.SourceEvidenceDigest,
		payload.AcceptanceContractDigest,
		payload.DeterministicResultDigest,
		payload.VerifierCandidateDigest,
		payload.AcceptanceDecisionDigest,
	)
}

func acceptanceOutcomePayloadEventID(
	payload workItemAcceptanceOutcomePayload,
	eventType string,
) string {
	if eventType == "WorkItemDone" {
		return deterministicEventID(
			eventType,
			payload.WorkItemID,
			payload.VerificationEventID,
			payload.AcceptanceDecisionDigest,
			payload.SourceEvidenceDigest,
			payload.VerifierEvidenceDigest,
		)
	}
	return deterministicEventID(
		eventType,
		payload.WorkItemID,
		payload.RunID,
		fmt.Sprint(payload.ClaimGeneration),
		payload.Status,
		payload.VerificationEventID,
		payload.AcceptanceDecisionDigest,
		payload.RecoveryTrigger,
		fmt.Sprint(payload.RecoveryPolicyVersion),
		payload.RecoveryPolicyDigest,
		fmt.Sprint(payload.AttemptNumber),
		fmt.Sprint(payload.MaxAttempts),
		fmt.Sprint(payload.CreditsBefore),
	)
}

func teamAcceptanceEventID(
	input TeamNodeAcceptanceInput,
	outcomeEventID string,
	nodeStatus string,
	dependencySatisfied bool,
	recoveryTrigger string,
	recoveryPolicyVersion int,
	recoveryPolicyDigest string,
	creditsBefore int,
) string {
	source := input.DeterministicResult.Input()
	return deterministicEventID(
		"TeamNodeAcceptanceCommitted",
		input.TeamInstanceID,
		input.PlanDigest,
		input.LogicalNodeID,
		fmt.Sprint(input.AttemptNumber),
		source.WorkItemID,
		outcomeEventID,
		fmt.Sprint(input.AcceptanceContract.Version()),
		input.AcceptanceContract.Digest(),
		string(input.Decision.Kind()),
		input.Decision.Digest(),
		input.Decision.DecisionTime().Format(time.RFC3339Nano),
		nodeStatus,
		fmt.Sprint(dependencySatisfied),
		recoveryTrigger,
		fmt.Sprint(recoveryPolicyVersion),
		recoveryPolicyDigest,
		fmt.Sprint(creditsBefore),
	)
}

func teamAcceptancePayloadEventID(
	payload teamNodeAcceptancePayload,
) string {
	return deterministicEventID(
		"TeamNodeAcceptanceCommitted",
		payload.TeamInstanceID,
		payload.PlanDigest,
		payload.LogicalNodeID,
		fmt.Sprint(payload.AttemptNumber),
		payload.WorkItemID,
		payload.WorkOutcomeEventID,
		fmt.Sprint(payload.AcceptanceContractVersion),
		payload.AcceptanceContractDigest,
		payload.AcceptanceDecisionKind,
		payload.AcceptanceDecisionDigest,
		payload.DecidedAt,
		payload.NodeStatus,
		fmt.Sprint(payload.DependencySatisfied),
		payload.RecoveryTrigger,
		fmt.Sprint(payload.RecoveryPolicyVersion),
		payload.RecoveryPolicyDigest,
		fmt.Sprint(payload.CreditsBefore),
	)
}

func replayAcceptanceSourceLineage(
	events []journal.Event,
	input verification.DeterministicVerificationInput,
) (WorkItemRecord, RunRecord, error) {
	workEvents := filterAcceptanceEvents(
		events,
		[]string{workItemStream(input.WorkItemID)},
	)
	runEvents := filterAcceptanceEvents(
		events,
		[]string{runStream(input.RunID)},
	)
	if len(workEvents) == 0 || len(runEvents) == 0 {
		return WorkItemRecord{}, RunRecord{}, ErrWorkItemAcceptanceConflict
	}
	state := authorityState{
		workItems: make(map[string]WorkItemRecord),
		runs:      make(map[string]RunRecord),
	}
	outcomes := make(map[string]journal.Event)
	if err := replayWorkItemStream(
		&state,
		workItemStream(input.WorkItemID),
		workEvents,
		outcomes,
	); err != nil {
		return WorkItemRecord{}, RunRecord{}, err
	}
	for index, event := range runEvents {
		if event.StreamID != runStream(input.RunID) ||
			event.Seq != int64(index+1) ||
			event.SchemaVersion != 1 ||
			event.ID == "" ||
			event.IdempotencyKey == "" ||
			event.CorrelationID == "" ||
			event.EmittedAt.IsZero() ||
			event.EmittedAt.Location() != time.UTC {
			return WorkItemRecord{}, RunRecord{}, ErrWorkItemAcceptanceConflict
		}
	}
	terminal := runEvents[len(runEvents)-1]
	if terminal.Type != "RunTerminalCommitted" {
		return WorkItemRecord{}, RunRecord{}, ErrWorkItemAcceptanceConflict
	}
	if _, ok := outcomes[terminal.ID]; !ok {
		return WorkItemRecord{}, RunRecord{}, ErrWorkItemAcceptanceConflict
	}
	var payload terminalPayload
	if decodeExactPayload(terminal.PayloadJSON, &payload) != nil ||
		payload.WorkItemID == nil ||
		payload.RunID == nil ||
		payload.ClaimID == nil ||
		payload.ClaimGeneration == nil ||
		payload.RuntimeInstanceID == nil ||
		payload.AgentInstanceID == nil ||
		payload.Status == nil ||
		payload.Reason == nil ||
		!payload.runtimeStatusReferenceFields.complete() ||
		*payload.WorkItemID != input.WorkItemID ||
		*payload.RunID != input.RunID ||
		*payload.ClaimID != input.ClaimID ||
		*payload.ClaimGeneration != input.ClaimGeneration ||
		*payload.Status != input.TerminalStatus {
		return WorkItemRecord{}, RunRecord{}, ErrWorkItemAcceptanceConflict
	}
	return state.workItems[input.WorkItemID], RunRecord{
		id:                input.RunID,
		workItemID:        input.WorkItemID,
		phase:             "terminal",
		claimID:           *payload.ClaimID,
		claimGeneration:   *payload.ClaimGeneration,
		runtimeInstanceID: *payload.RuntimeInstanceID,
		agentInstanceID:   *payload.AgentInstanceID,
		terminalStatus:    *payload.Status,
		terminalReason:    *payload.Reason,
		lastEventID:       terminal.ID,
		streamSequence:    terminal.Seq,
	}, nil
}

func filterAcceptanceEvents(
	events []journal.Event,
	streamIDs []string,
) []journal.Event {
	allowed := make(map[string]struct{}, len(streamIDs))
	for _, streamID := range streamIDs {
		allowed[streamID] = struct{}{}
	}
	filtered := make([]journal.Event, 0, len(events))
	for _, event := range events {
		if _, ok := allowed[event.StreamID]; ok {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func exactSubmittedEvidence(
	events []journal.Event,
	workItemID string,
	runID string,
	claimGeneration int64,
	correlationID string,
	receipt evidence.AttemptReceipt,
) bool {
	streamID := "evidence/" + receipt.EvidenceID()
	expectedEventID := deterministicEventID(
		"EvidenceSubmitted",
		receipt.EvidenceID(),
		receipt.Digest(),
		runID,
		fmt.Sprint(claimGeneration),
	)
	var matched bool
	for _, event := range events {
		if event.StreamID != streamID {
			continue
		}
		if matched || event.Seq != 1 ||
			event.Type != "EvidenceSubmitted" ||
			event.SchemaVersion != 1 ||
			event.ID != expectedEventID ||
			event.IdempotencyKey != event.ID ||
			event.CorrelationID != correlationID ||
			!validCanonicalUUID(event.CorrelationID) ||
			event.EmittedAt.IsZero() ||
			event.EmittedAt.Location() != time.UTC ||
			event.CausationID != "" {
			return false
		}
		var payload struct {
			EvidenceID string `json:"evidence_id"`
			WorkItemID string `json:"work_item_id"`
			Digest     string `json:"digest"`
		}
		if decodeExactPayload(event.PayloadJSON, &payload) != nil ||
			payload.EvidenceID != receipt.EvidenceID() ||
			payload.WorkItemID != workItemID ||
			payload.Digest != receipt.Digest() {
			return false
		}
		matched = true
	}
	return matched
}

func exactVerifierGrantAuthorization(
	events []journal.Event,
	binding verification.VerifierTerminalInput,
) bool {
	streamID := "agent-grant/" + binding.RunID
	targetIssued := false
	targetAuthorized := false
	seenGrantIDs := make(map[string]struct{})
	var currentGrantID string
	var currentWorkItemID string
	var currentClaimID string
	var currentClaimGeneration int64
	var currentRuntimeInstanceID string
	var currentAgentInstanceID string
	var currentIssuedAt time.Time
	var currentExpiresAt time.Time
	currentAllowedOperations := make(map[string]struct{})
	var previousEventID string
	var sequence int64
	for _, event := range events {
		if event.StreamID != streamID {
			continue
		}
		if event.Seq != sequence+1 ||
			event.SchemaVersion != 1 ||
			event.ID == "" ||
			event.IdempotencyKey != event.ID ||
			!validCanonicalUUID(event.CorrelationID) ||
			event.EmittedAt.IsZero() ||
			event.EmittedAt.Location() != time.UTC {
			return false
		}
		sequence = event.Seq
		switch event.Type {
		case "AgentGrantIssued":
			var payload struct {
				GrantID           string   `json:"grant_id"`
				WorkItemID        string   `json:"work_item_id"`
				RunID             string   `json:"run_id"`
				ClaimID           string   `json:"claim_id"`
				ClaimGeneration   int64    `json:"claim_generation"`
				RuntimeInstanceID string   `json:"runtime_instance_id"`
				AgentInstanceID   string   `json:"agent_instance_id"`
				AllowedOperations []string `json:"allowed_operations"`
				TokenHash         string   `json:"token_hash"`
				IssuedAt          string   `json:"issued_at"`
				ExpiresAt         string   `json:"expires_at"`
				RunStream         string   `json:"run_stream"`
				RunSequence       int64    `json:"run_sequence"`
				RunEventID        string   `json:"run_event_id"`
			}
			if currentGrantID != "" ||
				decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				!validCanonicalUUID(payload.GrantID) ||
				!validOpaqueID(payload.WorkItemID) ||
				payload.RunID != binding.RunID ||
				!validCanonicalUUID(payload.ClaimID) ||
				payload.ClaimGeneration < 1 ||
				!validOpaqueID(payload.RuntimeInstanceID) ||
				!validOpaqueID(payload.AgentInstanceID) ||
				payload.RunStream != runStream(binding.RunID) ||
				payload.RunSequence <= 0 ||
				!validOpaqueID(payload.RunEventID) ||
				!validSHA256Hex(payload.TokenHash) ||
				!validUTCString(payload.IssuedAt) ||
				!validUTCString(payload.ExpiresAt) ||
				len(payload.AllowedOperations) == 0 ||
				event.ID != deterministicGrantEventIdentity(
					"AgentGrantIssued",
					payload.GrantID,
					payload.RunID,
					fmt.Sprint(payload.ClaimGeneration),
					event.CorrelationID,
				) ||
				(previousEventID == "" &&
					event.CausationID == "" ||
					previousEventID != "" &&
						event.CausationID != previousEventID) ||
				!validOpaqueID(event.CausationID) ||
				event.EmittedAt.Format(time.RFC3339Nano) !=
					payload.IssuedAt {
				return false
			}
			if _, duplicate := seenGrantIDs[payload.GrantID]; duplicate {
				return false
			}
			issuedAt, issuedErr := time.Parse(
				time.RFC3339Nano,
				payload.IssuedAt,
			)
			expiresAt, expiresErr := time.Parse(
				time.RFC3339Nano,
				payload.ExpiresAt,
			)
			if issuedErr != nil ||
				expiresErr != nil ||
				!expiresAt.After(issuedAt) {
				return false
			}
			currentAllowedOperations =
				make(map[string]struct{}, len(payload.AllowedOperations))
			for _, operation := range payload.AllowedOperations {
				if !validOpaqueID(operation) {
					return false
				}
				if _, duplicate :=
					currentAllowedOperations[operation]; duplicate {
					return false
				}
				currentAllowedOperations[operation] = struct{}{}
			}
			seenGrantIDs[payload.GrantID] = struct{}{}
			currentGrantID = payload.GrantID
			currentWorkItemID = payload.WorkItemID
			currentClaimID = payload.ClaimID
			currentClaimGeneration = payload.ClaimGeneration
			currentRuntimeInstanceID = payload.RuntimeInstanceID
			currentAgentInstanceID = payload.AgentInstanceID
			currentIssuedAt = issuedAt
			currentExpiresAt = expiresAt
			if payload.GrantID == binding.GrantID {
				if payload.WorkItemID != binding.WorkItemID ||
					payload.ClaimID != binding.ClaimID ||
					payload.ClaimGeneration != binding.ClaimGeneration ||
					payload.RuntimeInstanceID != binding.RuntimeInstanceID ||
					payload.AgentInstanceID != binding.AgentInstanceID {
					return false
				}
				targetIssued = true
			}
		case "AgentGrantAuthorized":
			var payload struct {
				GrantID           string `json:"grant_id"`
				WorkItemID        string `json:"work_item_id"`
				RunID             string `json:"run_id"`
				ClaimID           string `json:"claim_id"`
				ClaimGeneration   int64  `json:"claim_generation"`
				RuntimeInstanceID string `json:"runtime_instance_id"`
				AgentInstanceID   string `json:"agent_instance_id"`
				Operation         string `json:"operation"`
				RequestID         string `json:"request_id"`
				AuthorizedAt      string `json:"authorized_at"`
				RunStream         string `json:"run_stream"`
				RunSequence       int64  `json:"run_sequence"`
				RunEventID        string `json:"run_event_id"`
			}
			if currentGrantID == "" ||
				decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.GrantID != currentGrantID ||
				payload.WorkItemID != currentWorkItemID ||
				payload.RunID != binding.RunID ||
				payload.ClaimID != currentClaimID ||
				payload.ClaimGeneration != currentClaimGeneration ||
				payload.RuntimeInstanceID != currentRuntimeInstanceID ||
				payload.AgentInstanceID != currentAgentInstanceID ||
				payload.RunStream != runStream(binding.RunID) ||
				payload.RunSequence <= 0 ||
				!validOpaqueID(payload.RunEventID) ||
				!validOpaqueID(payload.RequestID) ||
				!validUTCString(payload.AuthorizedAt) ||
				event.ID != deterministicGrantEventIdentity(
					"AgentGrantAuthorized",
					payload.RunID,
					payload.RequestID,
				) ||
				event.CausationID != previousEventID ||
				event.EmittedAt.Format(time.RFC3339Nano) !=
					payload.AuthorizedAt {
				return false
			}
			if _, permitted :=
				currentAllowedOperations[payload.Operation]; !permitted {
				return false
			}
			authorizedAt, authorizedErr := time.Parse(
				time.RFC3339Nano,
				payload.AuthorizedAt,
			)
			if authorizedErr != nil ||
				!authorizedAt.Before(currentExpiresAt) {
				return false
			}
			if currentGrantID == binding.GrantID &&
				(payload.Operation == "bridge.result" ||
					payload.Operation == "bridge.evidence") {
				targetAuthorized = true
			}
		case "AgentGrantRevoked":
			// Revocation after authorized execution does not erase the
			// immutable authorization fact used by the verifier Evidence.
			var payload struct {
				GrantID     string `json:"grant_id"`
				Reason      string `json:"reason"`
				RevokedAt   string `json:"revoked_at"`
				RunStream   string `json:"run_stream"`
				RunSequence int64  `json:"run_sequence"`
				RunEventID  string `json:"run_event_id"`
			}
			if currentGrantID == "" ||
				decodeExactPayload(event.PayloadJSON, &payload) != nil ||
				payload.GrantID != currentGrantID ||
				payload.Reason == "" ||
				!validUTCString(payload.RevokedAt) ||
				payload.RunStream != runStream(binding.RunID) ||
				payload.RunSequence <= 0 ||
				!validOpaqueID(payload.RunEventID) ||
				event.ID != deterministicGrantEventIdentity(
					"AgentGrantRevoked",
					payload.GrantID,
					payload.Reason,
					event.CorrelationID,
				) ||
				event.CausationID != previousEventID ||
				event.EmittedAt.Format(time.RFC3339Nano) !=
					payload.RevokedAt {
				return false
			}
			revokedAt, revokedErr := time.Parse(
				time.RFC3339Nano,
				payload.RevokedAt,
			)
			if revokedErr != nil || revokedAt.Before(currentIssuedAt) {
				return false
			}
			currentGrantID = ""
			currentWorkItemID = ""
			currentClaimID = ""
			currentClaimGeneration = 0
			currentRuntimeInstanceID = ""
			currentAgentInstanceID = ""
			currentIssuedAt = time.Time{}
			currentExpiresAt = time.Time{}
			currentAllowedOperations = make(map[string]struct{})
		default:
			return false
		}
		previousEventID = event.ID
	}
	return targetIssued && targetAuthorized
}

func deterministicGrantEventIdentity(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func newWorkItemVerificationPayload(
	input TeamNodeAcceptanceInput,
) workItemVerificationPayload {
	source := input.DeterministicResult.Input()
	payload := workItemVerificationPayload{
		TeamInstanceID:             input.TeamInstanceID,
		PlanDigest:                 input.PlanDigest,
		LogicalNodeID:              input.LogicalNodeID,
		AttemptNumber:              input.AttemptNumber,
		WorkItemID:                 source.WorkItemID,
		RunID:                      source.RunID,
		ClaimID:                    source.ClaimID,
		ClaimGeneration:            source.ClaimGeneration,
		SourceEvidenceID:           input.SourceReceipt.EvidenceID(),
		SourceEvidenceDigest:       input.SourceReceipt.Digest(),
		OutputSummaryDigest:        input.SourceReceipt.OutputSummary().Digest(),
		OutputContractVersion:      source.OutputContractVersion,
		OutputContractDigest:       source.OutputContractDigest,
		OutputClassification:       string(source.OutputClassification),
		OutputClassificationDigest: source.OutputClassificationDigest,
		AcceptanceContractVersion:  input.AcceptanceContract.Version(),
		AcceptanceContractDigest:   input.AcceptanceContract.Digest(),
		Risk:                       string(input.AcceptanceContract.Risk()),
		DeterministicResultDigest:  input.DeterministicResult.Digest(),
		VerifierRequired:           input.AcceptanceContract.IndependentVerifierRequired(),
		AcceptanceDecisionKind:     string(input.Decision.Kind()),
		AcceptanceDecisionDigest:   input.Decision.Digest(),
		DecidedAt:                  input.Decision.DecisionTime().Format(time.RFC3339Nano),
	}
	if payload.VerifierRequired {
		binding := input.VerifierCandidate.Binding()
		payload.VerifierWorkItemID = binding.WorkItemID
		payload.VerifierRunID = binding.RunID
		payload.VerifierClaimID = binding.ClaimID
		payload.VerifierClaimGeneration = binding.ClaimGeneration
		payload.VerifierRuntimeInstanceID = binding.RuntimeInstanceID
		payload.VerifierAgentInstanceID = binding.AgentInstanceID
		payload.VerifierGrantID = binding.GrantID
		payload.VerifierEvidenceID = binding.EvidenceID
		payload.VerifierEvidenceDigest = binding.EvidenceDigest
		payload.VerifierOutputSummaryDigest = binding.OutputSummaryDigest
		payload.VerifierCandidateKind = string(input.VerifierCandidate.Kind())
		payload.VerifierReasonCode = string(input.VerifierCandidate.ReasonCode())
		payload.VerifierCandidateDigest = input.VerifierCandidate.Digest()
	}
	return payload
}

func newWorkItemAcceptanceOutcomePayload(
	input TeamNodeAcceptanceInput,
	verificationEventID string,
	status string,
) workItemAcceptanceOutcomePayload {
	source := input.DeterministicResult.Input()
	payload := workItemAcceptanceOutcomePayload{
		WorkItemID:               source.WorkItemID,
		RunID:                    source.RunID,
		ClaimGeneration:          source.ClaimGeneration,
		Status:                   status,
		VerificationEventID:      verificationEventID,
		AcceptanceDecisionDigest: input.Decision.Digest(),
	}
	if status == "done" {
		payload.SourceEvidenceID = input.SourceReceipt.EvidenceID()
		payload.SourceEvidenceDigest = input.SourceReceipt.Digest()
		payload.VerifierEvidenceID = input.VerifierReceipt.EvidenceID()
		payload.VerifierEvidenceDigest = input.VerifierReceipt.Digest()
	} else {
		payload.RecoveryTrigger =
			teamRecoveryTriggerVerificationRejected
		payload.RecoveryPolicyVersion = input.RecoveryPolicy.Version()
		payload.RecoveryPolicyDigest = input.RecoveryPolicy.Digest()
		payload.AttemptNumber = input.AttemptNumber
		payload.MaxAttempts = input.MaxAttempts
		payload.CreditsBefore = input.CreditsBefore
	}
	return payload
}

func (payload workItemVerificationPayload) valid(
	record WorkItemRecord,
) bool {
	if payload.WorkItemID != record.id ||
		payload.RunID != record.runID ||
		payload.AttemptNumber <= 0 ||
		payload.ClaimGeneration <= 0 ||
		!validOpaqueID(payload.TeamInstanceID) ||
		!validSHA256Hex(payload.PlanDigest) ||
		!validOpaqueID(payload.LogicalNodeID) ||
		!validOpaqueID(payload.ClaimID) ||
		!validOpaqueID(payload.SourceEvidenceID) ||
		!validSHA256Hex(payload.SourceEvidenceDigest) ||
		!validSHA256Hex(payload.OutputSummaryDigest) ||
		payload.OutputContractVersion <= 0 ||
		!validSHA256Hex(payload.OutputContractDigest) ||
		!validOutputClassificationString(payload.OutputClassification) ||
		!validSHA256Hex(payload.OutputClassificationDigest) ||
		payload.AcceptanceContractVersion <= 0 ||
		!validSHA256Hex(payload.AcceptanceContractDigest) ||
		!validAcceptanceRiskString(payload.Risk) ||
		!validSHA256Hex(payload.DeterministicResultDigest) ||
		(payload.AcceptanceDecisionKind != "accepted" &&
			payload.AcceptanceDecisionKind != "rejected") ||
		!validSHA256Hex(payload.AcceptanceDecisionDigest) ||
		!validUTCString(payload.DecidedAt) {
		return false
	}
	if !payload.VerifierRequired {
		return payload.Risk == "low" &&
			payload.AcceptanceDecisionKind == "accepted" &&
			payload.VerifierWorkItemID == "" &&
			payload.VerifierRunID == "" &&
			payload.VerifierClaimID == "" &&
			payload.VerifierClaimGeneration == 0 &&
			payload.VerifierRuntimeInstanceID == "" &&
			payload.VerifierAgentInstanceID == "" &&
			payload.VerifierGrantID == "" &&
			payload.VerifierEvidenceID == "" &&
			payload.VerifierEvidenceDigest == "" &&
			payload.VerifierOutputSummaryDigest == "" &&
			payload.VerifierCandidateKind == "" &&
			payload.VerifierReasonCode == "" &&
			payload.VerifierCandidateDigest == ""
	}
	if payload.Risk != "medium" && payload.Risk != "high" {
		return false
	}
	if !validOpaqueID(payload.VerifierWorkItemID) ||
		!validOpaqueID(payload.VerifierRunID) ||
		!validCanonicalUUID(payload.VerifierClaimID) ||
		payload.VerifierClaimGeneration <= 0 ||
		!validOpaqueID(payload.VerifierRuntimeInstanceID) ||
		!validOpaqueID(payload.VerifierAgentInstanceID) ||
		!validOpaqueID(payload.VerifierGrantID) ||
		!validOpaqueID(payload.VerifierEvidenceID) ||
		!validSHA256Hex(payload.VerifierEvidenceDigest) ||
		!validSHA256Hex(payload.VerifierOutputSummaryDigest) ||
		!validSHA256Hex(payload.VerifierCandidateDigest) ||
		payload.VerifierWorkItemID == payload.WorkItemID ||
		payload.VerifierRunID == payload.RunID {
		return false
	}
	switch payload.VerifierCandidateKind {
	case "accepted":
		return payload.VerifierReasonCode == "criteria_satisfied" &&
			payload.AcceptanceDecisionKind == "accepted"
	case "rejected":
		return (payload.VerifierReasonCode ==
			"criteria_not_satisfied" ||
			payload.VerifierReasonCode == "insufficient_evidence") &&
			payload.AcceptanceDecisionKind == "rejected"
	default:
		return false
	}
}

func (payload workItemAcceptanceOutcomePayload) valid(
	record WorkItemRecord,
	eventType string,
) bool {
	if payload.WorkItemID != record.id ||
		payload.RunID != record.runID ||
		payload.ClaimGeneration != record.verificationClaimGeneration ||
		payload.VerificationEventID != record.verificationEventID ||
		payload.AcceptanceDecisionDigest !=
			record.acceptanceDecisionDigest {
		return false
	}
	if eventType == "WorkItemDone" {
		return payload.Status == "done" &&
			payload.SourceEvidenceID == record.sourceEvidenceID &&
			payload.SourceEvidenceDigest ==
				record.sourceEvidenceDigest &&
			payload.VerifierEvidenceID ==
				record.verifierEvidenceID &&
			payload.VerifierEvidenceDigest ==
				record.verifierEvidenceDigest &&
			(!record.verifierRequired ||
				validOpaqueID(payload.VerifierEvidenceID) &&
					validSHA256Hex(
						payload.VerifierEvidenceDigest,
					)) &&
			payload.RecoveryTrigger == "" &&
			payload.RecoveryPolicyVersion == 0 &&
			payload.RecoveryPolicyDigest == ""
	}
	return eventType == "WorkItemRejected" &&
		payload.Status == "rejected" &&
		payload.SourceEvidenceID == "" &&
		payload.SourceEvidenceDigest == "" &&
		payload.VerifierEvidenceID == "" &&
		payload.VerifierEvidenceDigest == "" &&
		payload.RecoveryTrigger ==
			teamRecoveryTriggerVerificationRejected &&
		payload.RecoveryPolicyVersion > 0 &&
		validSHA256Hex(payload.RecoveryPolicyDigest) &&
		payload.AttemptNumber > 0 &&
		payload.MaxAttempts >= payload.AttemptNumber &&
		payload.CreditsBefore >= 0
}

func applyTeamNodeAcceptance(
	team *TeamExecutionRecord,
	logicalNodeID string,
	attemptNumber int,
	decision verification.AcceptanceDecision,
	policy AcceptanceRecoveryPolicy,
	creditsBefore int,
) {
	node := teamNodeByID(team, logicalNodeID)
	if node == nil || node.currentAttempt != attemptNumber {
		return
	}
	node.acceptanceDecisionDigest = decision.Digest()
	node.acceptanceDecisionTime = decision.DecisionTime()
	if decision.Kind() == verification.AcceptanceAccepted {
		node.status = "succeeded"
		node.dependencySatisfied = true
		return
	}
	node.status = "awaiting_recovery"
	node.dependencySatisfied = false
	node.recoveryTrigger = teamRecoveryTriggerVerificationRejected
	node.recoveryPolicyVersion = policy.Version()
	node.recoveryPolicyDigest = policy.Digest()
	node.creditsBefore = creditsBefore
	team.status = "awaiting_recovery"
}

func validUTCString(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.Location() == time.UTC
}
