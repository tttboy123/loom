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
				TeamInstanceID: teamInstanceID,
				PlanDigest:     payload.PlanDigest,
				Status:         "pending",
				Nodes:          make([]TeamExecutionNode, len(payload.Nodes)),
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
				maxAttempts[node.LogicalNodeID] = node.MaxAttempts
			}
			sort.Slice(record.Nodes, func(i, j int) bool {
				return record.Nodes[i].LogicalNodeID < record.Nodes[j].LogicalNodeID
			})
		case "TeamNodeAttemptScheduled":
			var payload struct {
				LogicalNodeID     string `json:"logical_node_id"`
				AttemptNumber     int    `json:"attempt_number"`
				WorkItemID        string `json:"work_item_id"`
				RunID             string `json:"run_id"`
				RuntimeInstanceID string `json:"runtime_instance_id"`
				AgentInstanceID   string `json:"agent_instance_id"`
				RetryAt           string `json:"retry_at"`
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
			node.CurrentAttempt = payload.AttemptNumber
			node.RetryAt = retryAt
			if payload.AttemptNumber > 1 {
				node.Status = "retry_scheduled"
			}
			node.Attempts = append(node.Attempts, TeamExecutionAttempt{
				AttemptNumber:     payload.AttemptNumber,
				WorkItemID:        payload.WorkItemID,
				RunID:             payload.RunID,
				RuntimeInstanceID: payload.RuntimeInstanceID,
				AgentInstanceID:   payload.AgentInstanceID,
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
				LogicalNodeID     string `json:"logical_node_id"`
				AttemptNumber     int    `json:"attempt_number"`
				WorkItemID        string `json:"work_item_id"`
				RunID             string `json:"run_id"`
				ClaimID           string `json:"claim_id"`
				ClaimGeneration   int64  `json:"claim_generation"`
				RuntimeInstanceID string `json:"runtime_instance_id"`
				AgentInstanceID   string `json:"agent_instance_id"`
				Status            string `json:"status"`
				EvidenceID        string `json:"evidence_id"`
				EvidenceDigest    string `json:"evidence_digest"`
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
			attempt.Status = payload.Status
			attempt.EvidenceID = payload.EvidenceID
			attempt.EvidenceDigest = payload.EvidenceDigest
			if payload.Status == "succeeded" {
				node.Status = "succeeded"
				node.DependencySatisfied = true
			} else if payload.AttemptNumber >= maxAttempts[payload.LogicalNodeID] {
				node.Status = payload.Status
				node.DependencySatisfied = false
			} else {
				node.Status = "awaiting_recovery"
				node.DependencySatisfied = false
				record.Status = "awaiting_recovery"
			}
		case "TeamNodeRecoveryRecorded":
			var payload struct {
				LogicalNodeID         string `json:"logical_node_id"`
				AttemptNumber         int    `json:"attempt_number"`
				Action                string `json:"action"`
				RetryAt               string `json:"retry_at"`
				NextAgentInstanceID   string `json:"next_agent_instance_id"`
				NextRuntimeInstanceID string `json:"next_runtime_instance_id"`
				DependencySatisfied   bool   `json:"dependency_satisfied"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			if node == nil || node.CurrentAttempt != payload.AttemptNumber {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node.DependencySatisfied = payload.DependencySatisfied
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
