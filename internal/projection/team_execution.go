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
) (TeamExecution, error) {
	var record TeamExecution
	var sequence int64
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
					LogicalNodeID            string `json:"logical_node_id"`
					OutputContractVersion    int    `json:"output_contract_version"`
					OutputContractDigest     string `json:"output_contract_digest"`
					RecoveryPolicyVersion    int    `json:"recovery_policy_version"`
					RecoveryPolicyDigest     string `json:"recovery_policy_digest"`
					AttemptCredits           int    `json:"attempt_credits"`
					PrimaryWorkflowPath      string `json:"primary_workflow_path"`
					WorkflowFallbackKey      string `json:"workflow_fallback_key"`
					RecoveryApprovalRequired bool   `json:"recovery_approval_required"`
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
				node.Status = "succeeded"
				node.DependencySatisfied = true
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
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				!validProjectedTeamTerminal(payload.Status) {
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
	}
	return cloneGlobalTeamExecution(record), nil
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
