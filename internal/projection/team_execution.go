package projection

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

func projectTeamExecutions(
	events []journal.Event,
) (map[string]TeamExecution, error) {
	byStream := make(map[string][]journal.Event)
	runClaimReferences := make(map[string]teamExecutionRunClaimReference)
	workOutcomeReferences := make(map[string]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "team-execution/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
		if strings.HasPrefix(event.StreamID, "run/") &&
			event.Type == "RunClaimed" {
			runClaimReferences[teamExecutionRunReferenceKey(
				event.StreamID,
				event.Seq,
			)] = teamExecutionRunClaimReference{eventID: event.ID}
		}
		if strings.HasPrefix(event.StreamID, "work-item/") &&
			(event.Type == "WorkItemDone" ||
				event.Type == "WorkItemRejected") {
			workOutcomeReferences[event.ID] = event
		}
	}
	projected := make(map[string]TeamExecution, len(byStream))
	for streamID, streamEvents := range byStream {
		sort.Slice(streamEvents, func(i, j int) bool {
			if streamEvents[i].Seq != streamEvents[j].Seq {
				return streamEvents[i].Seq < streamEvents[j].Seq
			}
			return streamEvents[i].ID < streamEvents[j].ID
		})
		teamInstanceID := strings.TrimPrefix(streamID, "team-execution/")
		record, err := projectTeamExecutionStream(
			teamInstanceID,
			streamEvents,
			runClaimReferences,
			workOutcomeReferences,
		)
		if err != nil {
			return nil, err
		}
		projected[teamInstanceID] = record
	}
	return projected, nil
}

func projectTeamExecutionStream(
	teamInstanceID string,
	events []journal.Event,
	runClaimReferences map[string]teamExecutionRunClaimReference,
	workOutcomeReferences map[string]journal.Event,
) (TeamExecution, error) {
	var record TeamExecution
	var sequence int64
	var lastEventID string
	maxAttempts := make(map[string]int)
	for _, event := range events {
		if event.Seq != sequence+1 || event.SchemaVersion != 1 {
			return TeamExecution{}, ErrInvalidProjectionEvent
		}
		sequence = event.Seq
		switch event.Type {
		case "TeamExecutionPlanned":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				ViewVersion    string `json:"view_version"`
				Nodes          []struct {
					LogicalNodeID     string   `json:"logical_node_id"`
					Title             string   `json:"title"`
					AgentInstanceID   string   `json:"agent_instance_id"`
					RuntimeInstanceID string   `json:"runtime_instance_id"`
					Role              string   `json:"role"`
					DependsOn         []string `json:"depends_on"`
					MaxAttempts       int      `json:"max_attempts"`
				} `json:"nodes"`
				SemanticBindings *[]struct {
					LogicalNodeID               string  `json:"logical_node_id"`
					OutputContractVersion       int     `json:"output_contract_version"`
					OutputContractDigest        string  `json:"output_contract_digest"`
					RecoveryPolicyVersion       int     `json:"recovery_policy_version"`
					RecoveryPolicyDigest        string  `json:"recovery_policy_digest"`
					AttemptCredits              int     `json:"attempt_credits"`
					PrimaryWorkflowPath         string  `json:"primary_workflow_path"`
					WorkflowFallbackKey         string  `json:"workflow_fallback_key"`
					RecoveryApprovalRequired    bool    `json:"recovery_approval_required"`
					AcceptanceContractVersion   *int    `json:"acceptance_contract_version,omitempty"`
					AcceptanceContractDigest    *string `json:"acceptance_contract_digest,omitempty"`
					AcceptanceRisk              *string `json:"risk,omitempty"`
					IndependentVerifierRequired *bool   `json:"independent_verifier_required,omitempty"`
					VerifierAgentInstanceID     *string `json:"verifier_agent_instance_id,omitempty"`
					VerifierRuntimeInstanceID   *string `json:"verifier_runtime_instance_id,omitempty"`
					VerifierWorkflowPath        *string `json:"verifier_workflow_path,omitempty"`
				} `json:"semantic_bindings"`
			}
			if record.TeamInstanceID != "" ||
				decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != teamInstanceID ||
				!validSHA256Digest(payload.PlanDigest) ||
				!validSHA256Digest(payload.ViewVersion) ||
				len(payload.Nodes) == 0 || len(payload.Nodes) > 3 {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			record = TeamExecution{
				TeamInstanceID:        teamInstanceID,
				PlanDigest:            payload.PlanDigest,
				Status:                "pending",
				Nodes:                 make([]TeamExecutionNode, len(payload.Nodes)),
				LegacySemanticUnbound: payload.SemanticBindings == nil,
			}
			semanticIndexes := make(map[string]int)
			if payload.SemanticBindings != nil {
				if len(*payload.SemanticBindings) != len(payload.Nodes) {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				for index, binding := range *payload.SemanticBindings {
					acceptancePresent, complete := completeProjectedAcceptanceBinding(
						binding.AcceptanceContractVersion,
						binding.AcceptanceContractDigest,
						binding.AcceptanceRisk,
						binding.IndependentVerifierRequired,
						binding.VerifierAgentInstanceID,
						binding.VerifierRuntimeInstanceID,
						binding.VerifierWorkflowPath,
					)
					if binding.LogicalNodeID == "" ||
						index > 0 &&
							(*payload.SemanticBindings)[index-1].
								LogicalNodeID >= binding.LogicalNodeID ||
						binding.OutputContractVersion < 1 ||
						!validSHA256Digest(binding.OutputContractDigest) ||
						binding.RecoveryPolicyVersion < 1 ||
						!validSHA256Digest(binding.RecoveryPolicyDigest) ||
						binding.AttemptCredits < 0 ||
						binding.AttemptCredits > 2 ||
						binding.PrimaryWorkflowPath == "" ||
						binding.WorkflowFallbackKey ==
							binding.PrimaryWorkflowPath {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					if !complete ||
						index > 0 &&
							record.LegacyAcceptanceUnbound == acceptancePresent {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					if index == 0 {
						record.LegacyAcceptanceUnbound = !acceptancePresent
					}
					semanticIndexes[binding.LogicalNodeID] = index
				}
			}
			for index, node := range payload.Nodes {
				if node.LogicalNodeID == "" ||
					node.MaxAttempts < 1 || node.MaxAttempts > 3 {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				record.Nodes[index] = TeamExecutionNode{
					LogicalNodeID: node.LogicalNodeID,
					Status:        "pending",
					Attempts:      []TeamExecutionAttempt{},
				}
				if payload.SemanticBindings != nil {
					bindingIndex, ok := semanticIndexes[node.LogicalNodeID]
					if !ok {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					binding := (*payload.SemanticBindings)[bindingIndex]
					if binding.AttemptCredits > node.MaxAttempts-1 {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					record.Nodes[index].OutputContractVersion =
						binding.OutputContractVersion
					record.Nodes[index].OutputContractDigest =
						binding.OutputContractDigest
					record.Nodes[index].RecoveryPolicyVersion =
						binding.RecoveryPolicyVersion
					record.Nodes[index].RecoveryPolicyDigest =
						binding.RecoveryPolicyDigest
					record.Nodes[index].AttemptCredits =
						binding.AttemptCredits
					record.Nodes[index].PrimaryWorkflowPath =
						binding.PrimaryWorkflowPath
					record.Nodes[index].WorkflowFallbackKey =
						binding.WorkflowFallbackKey
					record.Nodes[index].RecoveryApprovalRequired =
						binding.RecoveryApprovalRequired
					if !record.LegacyAcceptanceUnbound {
						record.Nodes[index].AcceptanceContractVersion =
							*binding.AcceptanceContractVersion
						record.Nodes[index].AcceptanceContractDigest =
							*binding.AcceptanceContractDigest
						record.Nodes[index].AcceptanceRisk =
							*binding.AcceptanceRisk
						record.Nodes[index].IndependentVerifierRequired =
							*binding.IndependentVerifierRequired
						record.Nodes[index].VerifierAgentInstanceID =
							*binding.VerifierAgentInstanceID
						record.Nodes[index].VerifierRuntimeInstanceID =
							*binding.VerifierRuntimeInstanceID
						record.Nodes[index].VerifierWorkflowPath =
							*binding.VerifierWorkflowPath
						if !validProjectedAcceptanceBinding(
							record.Nodes[index],
							node.AgentInstanceID,
						) {
							return TeamExecution{}, ErrInvalidProjectionEvent
						}
					}
				}
				maxAttempts[node.LogicalNodeID] = node.MaxAttempts
			}
			sort.Slice(record.Nodes, func(i, j int) bool {
				return record.Nodes[i].LogicalNodeID < record.Nodes[j].LogicalNodeID
			})
		case "TeamNodeAttemptScheduled":
			var payload struct {
				LogicalNodeID     string  `json:"logical_node_id"`
				AttemptNumber     int     `json:"attempt_number"`
				WorkItemID        string  `json:"work_item_id"`
				RunID             string  `json:"run_id"`
				RuntimeInstanceID string  `json:"runtime_instance_id"`
				AgentInstanceID   string  `json:"agent_instance_id"`
				WorkflowPath      *string `json:"workflow_path"`
				RetryAt           string  `json:"retry_at"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			retryAt, err := parseProjectedRetryAt(payload.RetryAt)
			if node == nil || err != nil ||
				payload.AttemptNumber != node.CurrentAttempt+1 ||
				payload.AttemptNumber < 1 || payload.AttemptNumber > 3 ||
				payload.WorkItemID == "" || payload.RunID == "" ||
				payload.RuntimeInstanceID == "" ||
				payload.AgentInstanceID == "" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			workflowPath := ""
			if record.LegacySemanticUnbound {
				if payload.WorkflowPath != nil {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			} else {
				if payload.WorkflowPath == nil ||
					*payload.WorkflowPath == "" {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				workflowPath = *payload.WorkflowPath
				if payload.AttemptNumber == 1 {
					if workflowPath != node.PrimaryWorkflowPath ||
						!retryAt.IsZero() {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
				} else {
					expectedWorkflow := ""
					switch node.RecoveryAction {
					case "retry":
						previous := projectedTeamAttempt(
							node,
							payload.AttemptNumber-1,
						)
						if previous != nil {
							expectedWorkflow = previous.WorkflowPath
						}
					case "fallback":
						expectedWorkflow = node.WorkflowFallbackKey
					}
					if expectedWorkflow == "" ||
						workflowPath != expectedWorkflow ||
						!retryAt.Equal(node.RetryAt) {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
				}
			}
			node.CurrentAttempt = payload.AttemptNumber
			node.RetryAt = retryAt
			if payload.AttemptNumber > 1 {
				if node.RecoveryAction == "fallback" {
					node.Status = "fallback_scheduled"
				} else {
					node.Status = "retry_scheduled"
				}
			}
			node.Attempts = append(node.Attempts, TeamExecutionAttempt{
				AttemptNumber:     payload.AttemptNumber,
				WorkItemID:        payload.WorkItemID,
				RunID:             payload.RunID,
				RuntimeInstanceID: payload.RuntimeInstanceID,
				AgentInstanceID:   payload.AgentInstanceID,
				WorkflowPath:      workflowPath,
				Status:            "scheduled",
			})
		case "TeamReadySetDispatched":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				ViewVersion    string `json:"view_version"`
				Attempts       []struct {
					LogicalNodeID     string `json:"logical_node_id"`
					AttemptNumber     int    `json:"attempt_number"`
					WorkItemID        string `json:"work_item_id"`
					RunID             string `json:"run_id"`
					ClaimID           string `json:"claim_id"`
					ClaimGeneration   int64  `json:"claim_generation"`
					RuntimeInstanceID string `json:"runtime_instance_id"`
					AgentInstanceID   string `json:"agent_instance_id"`
				} `json:"attempts"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				!validSHA256Digest(payload.ViewVersion) ||
				len(payload.Attempts) == 0 {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			for _, dispatched := range payload.Attempts {
				node := projectedTeamNode(&record, dispatched.LogicalNodeID)
				attempt := projectedTeamAttempt(node, dispatched.AttemptNumber)
				if attempt == nil ||
					attempt.WorkItemID != dispatched.WorkItemID ||
					attempt.RunID != dispatched.RunID ||
					attempt.RuntimeInstanceID != dispatched.RuntimeInstanceID ||
					attempt.AgentInstanceID != dispatched.AgentInstanceID ||
					dispatched.ClaimID == "" ||
					dispatched.ClaimGeneration <= 0 {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				attempt.ClaimID = dispatched.ClaimID
				attempt.ClaimGeneration = dispatched.ClaimGeneration
				attempt.Status = "dispatched"
				node.Status = "running"
				node.RetryAt = time.Time{}
			}
			record.Status = "running"
		case "TeamNodeAttemptRebound":
			var payload struct {
				TeamInstanceID          string `json:"team_instance_id"`
				PlanDigest              string `json:"plan_digest"`
				LogicalNodeID           string `json:"logical_node_id"`
				AttemptNumber           int    `json:"attempt_number"`
				WorkItemID              string `json:"work_item_id"`
				RunID                   string `json:"run_id"`
				PreviousClaimID         string `json:"previous_claim_id"`
				PreviousClaimGeneration int64  `json:"previous_claim_generation"`
				ClaimID                 string `json:"claim_id"`
				ClaimGeneration         int64  `json:"claim_generation"`
				RuntimeInstanceID       string `json:"runtime_instance_id"`
				AgentInstanceID         string `json:"agent_instance_id"`
				RunStream               string `json:"run_stream"`
				RunSequence             int64  `json:"run_sequence"`
				RunEventID              string `json:"run_event_id"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				payload.RunStream != "run/"+payload.RunID ||
				payload.PreviousClaimID == "" ||
				payload.ClaimID == "" ||
				payload.ClaimID == payload.PreviousClaimID ||
				payload.PreviousClaimGeneration <= 0 ||
				payload.ClaimGeneration != payload.PreviousClaimGeneration+1 {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			reference, exists := runClaimReferences[teamExecutionRunReferenceKey(
				payload.RunStream,
				payload.RunSequence,
			)]
			if !exists ||
				reference.eventID != payload.RunEventID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			attempt := projectedTeamAttempt(node, payload.AttemptNumber)
			if node == nil ||
				attempt == nil ||
				node.Status != "running" ||
				node.CurrentAttempt != payload.AttemptNumber ||
				attempt.Status != "dispatched" ||
				attempt.WorkItemID != payload.WorkItemID ||
				attempt.RunID != payload.RunID ||
				attempt.ClaimID != payload.PreviousClaimID ||
				attempt.ClaimGeneration != payload.PreviousClaimGeneration ||
				attempt.RuntimeInstanceID != payload.RuntimeInstanceID ||
				attempt.AgentInstanceID != payload.AgentInstanceID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			attempt.ClaimID = payload.ClaimID
			attempt.ClaimGeneration = payload.ClaimGeneration
		case "TeamNodeAttemptTerminal":
			var payload struct {
				LogicalNodeID              string `json:"logical_node_id"`
				AttemptNumber              int    `json:"attempt_number"`
				WorkItemID                 string `json:"work_item_id"`
				RunID                      string `json:"run_id"`
				ClaimID                    string `json:"claim_id"`
				ClaimGeneration            int64  `json:"claim_generation"`
				RuntimeInstanceID          string `json:"runtime_instance_id"`
				AgentInstanceID            string `json:"agent_instance_id"`
				Status                     string `json:"status"`
				EvidenceID                 string `json:"evidence_id"`
				EvidenceDigest             string `json:"evidence_digest"`
				OutputContractVersion      int    `json:"output_contract_version"`
				OutputContractDigest       string `json:"output_contract_digest"`
				OutputClassification       string `json:"output_classification"`
				OutputClassificationDigest string `json:"output_classification_digest"`
				OutputSummaryDigest        string `json:"output_summary_digest"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				!validSHA256Digest(payload.EvidenceDigest) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			attempt := projectedTeamAttempt(node, payload.AttemptNumber)
			if attempt == nil ||
				attempt.WorkItemID != payload.WorkItemID ||
				attempt.RunID != payload.RunID ||
				attempt.ClaimID != payload.ClaimID ||
				attempt.ClaimGeneration != payload.ClaimGeneration ||
				attempt.RuntimeInstanceID != payload.RuntimeInstanceID ||
				attempt.AgentInstanceID != payload.AgentInstanceID ||
				payload.EvidenceID == "" ||
				payload.Status != "succeeded" &&
					payload.Status != "failed" &&
					payload.Status != "cancelled" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			if !record.LegacySemanticUnbound &&
				(payload.OutputContractVersion !=
					node.OutputContractVersion ||
					payload.OutputContractDigest !=
						node.OutputContractDigest ||
					!validProjectedOutputClassification(
						payload.OutputClassification,
					) ||
					!validSHA256Digest(
						payload.OutputClassificationDigest,
					) ||
					!validSHA256Digest(payload.OutputSummaryDigest)) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			attempt.Status = payload.Status
			attempt.EvidenceID = payload.EvidenceID
			attempt.EvidenceDigest = payload.EvidenceDigest
			attempt.OutputContractVersion = payload.OutputContractVersion
			attempt.OutputContractDigest = payload.OutputContractDigest
			attempt.OutputClassification = payload.OutputClassification
			attempt.OutputClassificationDigest =
				payload.OutputClassificationDigest
			attempt.OutputSummaryDigest = payload.OutputSummaryDigest
			if !record.LegacySemanticUnbound &&
				(payload.OutputClassification == "valid_nonempty" ||
					payload.OutputClassification == "valid_empty") {
				if record.LegacyAcceptanceUnbound {
					node.Status = "succeeded"
					node.DependencySatisfied = true
				} else {
					node.Status = "ready_for_review"
					node.DependencySatisfied = false
				}
			} else if record.LegacySemanticUnbound &&
				payload.Status == "succeeded" {
				node.Status = "succeeded"
				node.DependencySatisfied = true
			} else if record.LegacySemanticUnbound &&
				payload.AttemptNumber >= maxAttempts[payload.LogicalNodeID] {
				node.Status = payload.Status
				node.DependencySatisfied = false
			} else {
				node.Status = "awaiting_recovery"
				node.DependencySatisfied = false
				record.Status = "awaiting_recovery"
			}
		case "TeamNodeAcceptanceCommitted":
			var payload struct {
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
			if decodeExactProjectionPayload(event, &payload) != nil ||
				record.LegacyAcceptanceUnbound ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				event.ID != projectionDeterministicEventID(
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
				) ||
				event.CausationID != payload.WorkOutcomeEventID ||
				!validSHA256Digest(payload.AcceptanceDecisionDigest) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			attempt := projectedTeamAttempt(node, payload.AttemptNumber)
			decidedAt, decidedErr := parseProjectedRetryAt(payload.DecidedAt)
			outcome, outcomeExists :=
				workOutcomeReferences[payload.WorkOutcomeEventID]
			if node == nil ||
				attempt == nil ||
				node.Status != "ready_for_review" ||
				node.CurrentAttempt != payload.AttemptNumber ||
				attempt.WorkItemID != payload.WorkItemID ||
				payload.AcceptanceContractVersion !=
					node.AcceptanceContractVersion ||
				payload.AcceptanceContractDigest !=
					node.AcceptanceContractDigest ||
				decidedErr != nil ||
				decidedAt.IsZero() ||
				!outcomeExists ||
				outcome.StreamID != "work-item/"+payload.WorkItemID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			var outcomePayload runProjectionAcceptanceOutcomePayload
			if decodeExactProjectionPayload(outcome, &outcomePayload) != nil ||
				outcomePayload.WorkItemID != payload.WorkItemID ||
				outcomePayload.RunID != attempt.RunID ||
				outcomePayload.ClaimGeneration !=
					attempt.ClaimGeneration ||
				outcomePayload.AcceptanceDecisionDigest !=
					payload.AcceptanceDecisionDigest ||
				outcome.ID != projectionAcceptanceOutcomeEventID(
					outcome.Type,
					outcomePayload,
				) ||
				outcome.CausationID !=
					outcomePayload.VerificationEventID ||
				outcomePayload.VerificationEventID == "" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			switch payload.AcceptanceDecisionKind {
			case "accepted":
				if outcome.Type != "WorkItemDone" ||
					outcomePayload.Status != "done" ||
					payload.NodeStatus != "succeeded" ||
					!payload.DependencySatisfied ||
					payload.RecoveryTrigger != "" ||
					payload.RecoveryPolicyVersion != 0 ||
					payload.RecoveryPolicyDigest != "" ||
					payload.CreditsBefore != 0 {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "rejected":
				expectedCredits := node.AttemptCredits -
					(payload.AttemptNumber - 1)
				if expectedCredits < 0 {
					expectedCredits = 0
				}
				if outcome.Type != "WorkItemRejected" ||
					outcomePayload.Status != "rejected" ||
					payload.NodeStatus != "awaiting_recovery" ||
					payload.DependencySatisfied ||
					payload.RecoveryTrigger !=
						"verification_rejected" ||
					payload.RecoveryPolicyVersion !=
						node.RecoveryPolicyVersion ||
					payload.RecoveryPolicyDigest !=
						node.RecoveryPolicyDigest ||
					payload.CreditsBefore != expectedCredits {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			default:
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node.Status = payload.NodeStatus
			node.DependencySatisfied = payload.DependencySatisfied
			node.AcceptanceDecisionKind =
				payload.AcceptanceDecisionKind
			node.AcceptanceDecisionDigest =
				payload.AcceptanceDecisionDigest
			node.AcceptanceDecisionTime = decidedAt
			node.RecoveryTrigger = payload.RecoveryTrigger
			node.CreditsBefore = payload.CreditsBefore
			if payload.NodeStatus == "awaiting_recovery" {
				record.Status = "awaiting_recovery"
			}
		case "TeamNodeRecoveryRecorded":
			var payload struct {
				LogicalNodeID            string   `json:"logical_node_id"`
				AttemptNumber            int      `json:"attempt_number"`
				Action                   string   `json:"action"`
				DecisionTime             string   `json:"decision_time"`
				RetryAt                  string   `json:"retry_at"`
				NextAttemptNumber        int      `json:"next_attempt_number"`
				NextAgentInstanceID      string   `json:"next_agent_instance_id"`
				NextRuntimeInstanceID    string   `json:"next_runtime_instance_id"`
				WorkflowFallbackKey      string   `json:"workflow_fallback_key"`
				RecoveryPolicyVersion    int      `json:"recovery_policy_version"`
				RecoveryPolicyDigest     string   `json:"recovery_policy_digest"`
				RecoveryDecisionDigest   string   `json:"recovery_decision_digest"`
				ClassificationDigest     string   `json:"classification_digest"`
				PriorClassifications     []string `json:"prior_classifications"`
				CreditsBefore            int      `json:"credits_before"`
				CreditsAfter             int      `json:"credits_after"`
				FallbackConsumed         bool     `json:"fallback_consumed"`
				RecoveryApprovalRequired bool     `json:"recovery_approval_required"`
				DependencySatisfied      bool     `json:"dependency_satisfied"`
				RecoveryTrigger          string   `json:"recovery_trigger"`
				AcceptanceDecisionDigest string   `json:"acceptance_decision_digest"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			decisionTime, decisionErr := parseProjectedRetryAt(
				payload.DecisionTime,
			)
			retryAt, retryErr := parseProjectedRetryAt(payload.RetryAt)
			currentAttempt := projectedTeamAttempt(
				node,
				payload.AttemptNumber,
			)
			expectedCredits := 0
			if node != nil {
				expectedCredits = node.AttemptCredits -
					(payload.AttemptNumber - 1)
				if expectedCredits < 0 {
					expectedCredits = 0
				}
			}
			if node == nil ||
				record.LegacySemanticUnbound ||
				node.CurrentAttempt != payload.AttemptNumber ||
				currentAttempt == nil ||
				decisionErr != nil || decisionTime.IsZero() ||
				retryErr != nil ||
				payload.RecoveryPolicyVersion !=
					node.RecoveryPolicyVersion ||
				payload.RecoveryPolicyDigest !=
					node.RecoveryPolicyDigest ||
				!validSHA256Digest(payload.RecoveryDecisionDigest) ||
				!validSHA256Digest(payload.ClassificationDigest) ||
				payload.CreditsBefore < 0 ||
				payload.CreditsBefore > node.AttemptCredits ||
				payload.CreditsBefore != expectedCredits ||
				payload.CreditsAfter < 0 ||
				payload.CreditsAfter > payload.CreditsBefore ||
				len(payload.PriorClassifications) > 2 ||
				len(payload.PriorClassifications) !=
					payload.AttemptNumber-1 ||
				payload.ClassificationDigest !=
					currentAttempt.OutputClassificationDigest ||
				payload.RecoveryApprovalRequired !=
					node.RecoveryApprovalRequired {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			expectedTrigger := node.RecoveryTrigger
			if expectedTrigger == "" {
				expectedTrigger = "output"
			}
			payloadTrigger := payload.RecoveryTrigger
			if payloadTrigger == "" &&
				payload.AcceptanceDecisionDigest == "" {
				payloadTrigger = "output"
			}
			if payloadTrigger != expectedTrigger ||
				payload.AcceptanceDecisionDigest !=
					node.AcceptanceDecisionDigest ||
				payloadTrigger == "verification_rejected" &&
					payload.Action == "fallback" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			for _, classification := range payload.PriorClassifications {
				if !validProjectedOutputClassification(classification) {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			}
			for index, classification := range payload.PriorClassifications {
				if index >= len(node.Attempts) ||
					node.Attempts[index].AttemptNumber != index+1 ||
					node.Attempts[index].OutputClassification !=
						classification {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			}
			fallbackBefore := projectedTeamFallbackConsumed(*node)
			switch payload.Action {
			case "retry":
				if payload.NextAttemptNumber != payload.AttemptNumber+1 ||
					payload.NextAgentInstanceID !=
						currentAttempt.AgentInstanceID ||
					payload.NextRuntimeInstanceID !=
						currentAttempt.RuntimeInstanceID ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore-1 ||
					retryAt.IsZero() ||
					payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "fallback":
				if payload.NextAttemptNumber != payload.AttemptNumber+1 ||
					payload.NextAgentInstanceID !=
						currentAttempt.AgentInstanceID ||
					payload.NextRuntimeInstanceID !=
						currentAttempt.RuntimeInstanceID ||
					payload.WorkflowFallbackKey == "" ||
					payload.WorkflowFallbackKey !=
						node.WorkflowFallbackKey ||
					payload.CreditsAfter != payload.CreditsBefore-1 ||
					retryAt.IsZero() ||
					payload.DependencySatisfied ||
					fallbackBefore ||
					!payload.FallbackConsumed {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "degraded":
				if payload.NextAttemptNumber != 0 ||
					payload.NextAgentInstanceID != "" ||
					payload.NextRuntimeInstanceID != "" ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore ||
					!retryAt.IsZero() ||
					!payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "blocked", "human_required":
				if payload.NextAttemptNumber != 0 ||
					payload.NextAgentInstanceID != "" ||
					payload.NextRuntimeInstanceID != "" ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore ||
					!retryAt.IsZero() ||
					payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			default:
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node.DependencySatisfied = payload.DependencySatisfied
			node.RecoveryTrigger = payloadTrigger
			node.RecoveryAction = payload.Action
			node.RecoveryDecisionDigest = payload.RecoveryDecisionDigest
			node.RecoveryDecisionTime = decisionTime
			node.RetryAt = retryAt
			node.CreditsBefore = payload.CreditsBefore
			node.CreditsAfter = payload.CreditsAfter
			node.FallbackConsumed = payload.FallbackConsumed
			node.PriorClassifications = append(
				[]string(nil),
				payload.PriorClassifications...,
			)
			switch payload.Action {
			case "retry":
				node.Status = "retry_scheduled"
			case "fallback":
				node.Status = "fallback_scheduled"
			case "degraded":
				node.Status = "degraded"
			case "blocked":
				node.Status = "blocked"
			case "human_required":
				node.Status = "human_required"
			default:
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
		case "TeamExecutionTerminal":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				Status         string `json:"status"`
				Reason         string `json:"reason"`
			}
			expectedStatus, expectedReason :=
				projectedTeamTerminal(record)
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				!validProjectedTeamTerminal(payload.Status) ||
				expectedStatus == "" ||
				payload.Status != expectedStatus ||
				payload.Reason != expectedReason ||
				event.CausationID != lastEventID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			record.Status = payload.Status
		default:
			return TeamExecution{}, fmt.Errorf(
				"%w: unknown Team execution event %s",
				ErrInvalidProjectionEvent,
				event.Type,
			)
		}
		lastEventID = event.ID
	}
	return cloneGlobalTeamExecution(record), nil
}

func projectedTeamTerminal(record TeamExecution) (string, string) {
	if len(record.Nodes) == 0 {
		return "", ""
	}
	for _, node := range record.Nodes {
		switch node.Status {
		case "succeeded", "failed", "cancelled", "degraded", "blocked",
			"human_required":
		default:
			return "", ""
		}
	}
	for _, status := range []string{
		"human_required",
		"blocked",
		"failed",
		"cancelled",
		"degraded",
	} {
		for _, node := range record.Nodes {
			if node.Status == status {
				return status,
					"node_" + node.LogicalNodeID + "_" + status
			}
		}
	}
	for _, node := range record.Nodes {
		if node.Status != "succeeded" {
			return "failed",
				"node_" + node.LogicalNodeID + "_" + node.Status
		}
	}
	return "succeeded", ""
}

func completeProjectedAcceptanceBinding(
	version *int,
	digest *string,
	risk *string,
	required *bool,
	agentID *string,
	runtimeID *string,
	workflowPath *string,
) (bool, bool) {
	present := 0
	for _, exists := range []bool{
		version != nil,
		digest != nil,
		risk != nil,
		required != nil,
		agentID != nil,
		runtimeID != nil,
		workflowPath != nil,
	} {
		if exists {
			present++
		}
	}
	return present == 7, present == 0 || present == 7
}

func validProjectedAcceptanceBinding(
	node TeamExecutionNode,
	sourceAgentInstanceID string,
) bool {
	if node.AcceptanceContractVersion < 1 ||
		!validSHA256Digest(node.AcceptanceContractDigest) ||
		(node.AcceptanceRisk != "low" &&
			node.AcceptanceRisk != "medium" &&
			node.AcceptanceRisk != "high") {
		return false
	}
	required := node.AcceptanceRisk == "medium" ||
		node.AcceptanceRisk == "high"
	if node.IndependentVerifierRequired != required {
		return false
	}
	if !required {
		return node.VerifierAgentInstanceID == "" &&
			node.VerifierRuntimeInstanceID == "" &&
			node.VerifierWorkflowPath == ""
	}
	return node.VerifierAgentInstanceID != "" &&
		node.VerifierAgentInstanceID != sourceAgentInstanceID &&
		node.VerifierRuntimeInstanceID != "" &&
		node.VerifierWorkflowPath != ""
}

type teamExecutionRunClaimReference struct {
	eventID string
}

func teamExecutionRunReferenceKey(streamID string, sequence int64) string {
	return streamID + "\x00" + fmt.Sprint(sequence)
}

func projectedTeamNode(
	record *TeamExecution,
	logicalNodeID string,
) *TeamExecutionNode {
	for index := range record.Nodes {
		if record.Nodes[index].LogicalNodeID == logicalNodeID {
			return &record.Nodes[index]
		}
	}
	return nil
}

func projectedTeamAttempt(
	node *TeamExecutionNode,
	attemptNumber int,
) *TeamExecutionAttempt {
	if node == nil {
		return nil
	}
	for index := range node.Attempts {
		if node.Attempts[index].AttemptNumber == attemptNumber {
			return &node.Attempts[index]
		}
	}
	return nil
}

func projectedTeamFallbackConsumed(node TeamExecutionNode) bool {
	if node.WorkflowFallbackKey == "" {
		return false
	}
	for _, attempt := range node.Attempts {
		if attempt.WorkflowPath == node.WorkflowFallbackKey {
			return true
		}
	}
	return false
}

func parseProjectedRetryAt(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidProjectionEvent
	}
	return parsed, nil
}

func validProjectedTeamTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "degraded", "blocked", "human_required", "cancelled":
		return true
	default:
		return false
	}
}

func validProjectedOutputClassification(value string) bool {
	switch value {
	case "valid_nonempty", "valid_empty", "transient_empty", "invalid":
		return true
	default:
		return false
	}
}
