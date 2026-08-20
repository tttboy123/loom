package api

import (
	"sort"
	"strings"
	"time"

	"loom-pi-rebuild/internal/projection"
)

type MissionLane string

const (
	MissionLaneProposed      MissionLane = "Proposed"
	MissionLaneReady         MissionLane = "Ready"
	MissionLaneOrchestrating MissionLane = "Orchestrating"
	MissionLaneReview        MissionLane = "Review"
	MissionLaneComplete      MissionLane = "Complete"
)

type LocalProductMissionPulse struct {
	AgentInstanceID   string `json:"agent_instance_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	Role              string `json:"role"`
	State             string `json:"state"`
	NodeID            string `json:"node_id"`
	AttemptNumber     int    `json:"attempt_number"`
}

type LocalProductMissionNode struct {
	LogicalNodeID     string   `json:"logical_node_id"`
	Title             string   `json:"title"`
	AgentInstanceID   string   `json:"agent_instance_id"`
	RuntimeInstanceID string   `json:"runtime_instance_id"`
	Role              string   `json:"role"`
	DependsOn         []string `json:"depends_on"`
	MaxAttempts       int      `json:"max_attempts"`
	Status            string   `json:"status"`
	AttemptNumber     int      `json:"attempt_number"`
	Ready             bool     `json:"ready"`
}

type LocalProductMissionSummary struct {
	SchemaVersion      int                        `json:"schema_version"`
	MissionID          string                     `json:"mission_id"`
	TeamInstanceID     string                     `json:"team_instance_id"`
	Title              string                     `json:"title"`
	SourceKind         string                     `json:"source_kind"`
	Lane               MissionLane                `json:"lane"`
	Status             string                     `json:"status"`
	Priority           string                     `json:"priority"`
	PlanDigest         string                     `json:"plan_digest"`
	Simple             bool                       `json:"simple"`
	NodeCount          int                        `json:"node_count"`
	CompletedNodeCount int                        `json:"completed_node_count"`
	ActiveNodeCount    int                        `json:"active_node_count"`
	ReviewNodeCount    int                        `json:"review_node_count"`
	AttentionCount     int                        `json:"attention_count"`
	CurrentNodeID      string                     `json:"current_node_id"`
	LastMilestone      string                     `json:"last_milestone"`
	BlockReason        string                     `json:"block_reason,omitempty"`
	TeamPulse          []LocalProductMissionPulse `json:"team_pulse"`
	Topology           []LocalProductMissionNode  `json:"topology"`
}

func buildLocalProductMissionPage(
	view projection.GlobalReadView,
	executions []projection.TeamExecution,
	limit int,
	sourceHasMore bool,
) ([]LocalProductMissionSummary, LocalProductPageCursor) {
	ordered := append([]projection.TeamExecution(nil), executions...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].TeamInstanceID < ordered[j].TeamInstanceID
	})
	hasMore := sourceHasMore || len(ordered) > limit
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	missions := make([]LocalProductMissionSummary, 0, len(ordered))
	for _, execution := range ordered {
		missions = append(missions, buildLocalProductMission(view, execution))
	}
	return missions, localProductPageCursor(
		missions,
		hasMore,
		func(record LocalProductMissionSummary) string {
			return record.MissionID
		},
	)
}

func buildLocalProductMission(
	view projection.GlobalReadView,
	execution projection.TeamExecution,
) LocalProductMissionSummary {
	status := reconciledMissionStatus(view, execution)
	lane := missionLaneForStatus(status, execution.Status != "")
	title := "Historical mission"
	sourceKind := "historical_execution_only"
	if team, ok := view.Team(execution.TeamInstanceID); ok {
		sourceKind = team.SourceKind
		title = localProductSavedTeamDisplayName(view, team)
	}
	_, attention := deriveBoardAndAttention(view, execution.TeamInstanceID)
	mission := LocalProductMissionSummary{
		SchemaVersion:  1,
		MissionID:      "mission/" + execution.TeamInstanceID,
		TeamInstanceID: execution.TeamInstanceID,
		Title:          title,
		SourceKind:     sourceKind,
		Lane:           lane,
		Status:         status,
		Priority:       "normal",
		PlanDigest:     execution.PlanDigest,
		Simple:         len(execution.Nodes) == 1,
		NodeCount:      len(execution.Nodes),
		AttentionCount: len(attention),
		TeamPulse:      make([]LocalProductMissionPulse, 0, len(execution.Nodes)),
		Topology:       make([]LocalProductMissionNode, 0, len(execution.Nodes)),
		LastMilestone:  humanMissionMilestone(status),
		BlockReason:    localProductMissionBlockReason(view, execution),
	}
	for _, node := range execution.Nodes {
		state := missionPulseState(node.Status)
		ready := node.Status == "pending" && missionDependenciesReady(
			execution.Nodes,
			node.DependsOn,
		)
		mission.TeamPulse = append(mission.TeamPulse, LocalProductMissionPulse{
			AgentInstanceID:   node.AgentInstanceID,
			RuntimeInstanceID: node.RuntimeInstanceID,
			Role:              node.Role,
			State:             state,
			NodeID:            node.LogicalNodeID,
			AttemptNumber:     node.CurrentAttempt,
		})
		mission.Topology = append(mission.Topology, LocalProductMissionNode{
			LogicalNodeID:     node.LogicalNodeID,
			Title:             node.Title,
			AgentInstanceID:   node.AgentInstanceID,
			RuntimeInstanceID: node.RuntimeInstanceID,
			Role:              node.Role,
			DependsOn: append(
				make([]string, 0, len(node.DependsOn)),
				node.DependsOn...,
			),
			MaxAttempts:   node.MaxAttempts,
			Status:        node.Status,
			AttemptNumber: node.CurrentAttempt,
			Ready:         ready,
		})
		switch {
		case missionNodeComplete(node.Status):
			mission.CompletedNodeCount++
		case node.Status == "ready_for_review":
			mission.ReviewNodeCount++
		case missionNodeActive(node.Status):
			mission.ActiveNodeCount++
		}
		if mission.CurrentNodeID == "" &&
			!missionNodeComplete(node.Status) {
			mission.CurrentNodeID = node.LogicalNodeID
		}
	}
	sort.Slice(mission.TeamPulse, func(i, j int) bool {
		return mission.TeamPulse[i].NodeID < mission.TeamPulse[j].NodeID
	})
	sort.Slice(mission.Topology, func(i, j int) bool {
		return mission.Topology[i].LogicalNodeID <
			mission.Topology[j].LogicalNodeID
	})
	return mission
}

// localProductMissionBlockReason surfaces why a Mission is blocked so the
// board card can tell the user the concrete reason (for example
// context_retrieval_denied) instead of only "Blocked". It prefers the
// blocked node's initial block reason, then the current attempt's terminal
// reason, then any terminal attempt reason, then the terminal Run reason
// recorded in the projection (read-model fallback for a node the coordinator
// never marked terminal).
func localProductMissionBlockReason(
	view projection.GlobalReadView,
	execution projection.TeamExecution,
) string {
	for _, node := range execution.Nodes {
		if node.Status != "blocked" {
			continue
		}
		if node.InitialBlockReason != "" {
			return node.InitialBlockReason
		}
		if reason := projectedNodeTerminalReason(view, execution, node); reason != "" {
			return reason
		}
	}
	// A node the projection still marks "running" or "awaiting_recovery" whose
	// current attempt Run is already terminal (reconciled as failed/cancelled)
	// also surfaces its terminal reason so the board card can tell the user
	// what happened.
	for _, node := range execution.Nodes {
		if node.Status != "running" && node.Status != "awaiting_recovery" {
			continue
		}
		if reason := projectedNodeTerminalReason(view, execution, node); reason != "" {
			return reason
		}
	}
	return ""
}

func projectedNodeTerminalReason(
	view projection.GlobalReadView,
	execution projection.TeamExecution,
	node projection.TeamExecutionNode,
) string {
	if attempt, ok := findProjectedAttempt(
		execution, node.LogicalNodeID, node.CurrentAttempt,
	); ok {
		if attempt.TerminalReason != "" {
			return attempt.TerminalReason
		}
		if run, runOK := view.Run(attempt.RunID); runOK &&
			run.TerminalReason != "" {
			return run.TerminalReason
		}
	}
	for _, attempt := range node.Attempts {
		if attempt.TerminalReason != "" {
			return attempt.TerminalReason
		}
		if run, runOK := view.Run(attempt.RunID); runOK &&
			run.TerminalReason != "" {
			return run.TerminalReason
		}
	}
	return ""
}

// reconciledMissionStatus is a read-model-only correction for a Mission the
// projection still marks "running" even though every running node's current
// attempt Run is already terminal (failed/cancelled) and no retry is
// scheduled. This can happen when the daemon is interrupted between committing
// a Run terminal and the coordinator recording TeamNodeAttemptTerminal. It
// never writes the journal; it reflects terminal Run facts already in the
// projection. A terminal SUCCEEDED run without the coordinator record is kept
// "running" because the verification/acceptance chain has not completed.
func reconciledMissionStatus(
	view projection.GlobalReadView,
	execution projection.TeamExecution,
) string {
	status := missionStatus(execution)
	if status != "running" && status != "awaiting_recovery" {
		return status
	}
	terminal := ""
	for _, node := range execution.Nodes {
		// A node the projection still marks "running" or "awaiting_recovery"
		// whose current attempt Run is terminal, with no future retry
		// scheduled (and no retry that is still due), reflects the terminal
		// Run outcome. This covers interrupted daemons (Run terminal before
		// TeamNodeAttemptTerminal) and pre-fix recovery records whose retry
		// time passed without a new attempt.
		if node.Status != "running" && node.Status != "awaiting_recovery" {
			continue
		}
		if node.CurrentAttempt <= 0 {
			return status
		}
		if !node.RetryAt.IsZero() && node.RetryAt.After(time.Now()) {
			return status
		}
		attempt, ok := findProjectedAttempt(
			execution, node.LogicalNodeID, node.CurrentAttempt,
		)
		if !ok {
			return status
		}
		run, runOK := view.Run(attempt.RunID)
		if !runOK || run.TerminalStatus == "" {
			return status
		}
		if run.TerminalStatus == "succeeded" {
			return status
		}
		if terminal == "" {
			terminal = run.TerminalStatus
		}
	}
	if terminal != "" {
		return terminal
	}
	return status
}

func missionStatus(execution projection.TeamExecution) string {
	if execution.Status == "pending" {
		return "planned"
	}
	for _, preferred := range []string{
		"human_required",
		"blocked",
		"ready_for_review",
		"retry_scheduled",
		"fallback_scheduled",
		"running",
	} {
		for _, node := range execution.Nodes {
			if node.Status == preferred {
				if preferred == "retry_scheduled" ||
					preferred == "fallback_scheduled" {
					return "retrying"
				}
				return preferred
			}
		}
	}
	if execution.Status != "" {
		return execution.Status
	}
	return "planned"
}

func missionLaneForStatus(status string, exists bool) MissionLane {
	if !exists {
		return MissionLaneProposed
	}
	switch status {
	case "planned", "pending", "proposed":
		return MissionLaneProposed
	case "ready":
		return MissionLaneReady
	case "ready_for_review", "awaiting_acceptance":
		return MissionLaneReview
	case "succeeded", "failed", "cancelled", "degraded", "complete":
		return MissionLaneComplete
	default:
		return MissionLaneOrchestrating
	}
}

func validMissionLane(lane MissionLane) bool {
	switch lane {
	case MissionLaneProposed, MissionLaneReady, MissionLaneOrchestrating,
		MissionLaneReview, MissionLaneComplete:
		return true
	default:
		return false
	}
}

func missionPulseState(status string) string {
	switch status {
	case "pending":
		return "ready"
	case "running", "retry_scheduled", "fallback_scheduled":
		return "working"
	case "ready_for_review":
		return "review"
	case "blocked", "human_required":
		return "blocked"
	case "succeeded", "failed", "cancelled", "degraded":
		return "terminal"
	default:
		return "waiting"
	}
}

func missionNodeComplete(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled", "degraded":
		return true
	default:
		return false
	}
}

func missionNodeActive(status string) bool {
	switch status {
	case "running", "retry_scheduled", "fallback_scheduled",
		"awaiting_recovery", "blocked", "human_required":
		return true
	default:
		return false
	}
}

func missionDependenciesReady(
	nodes []projection.TeamExecutionNode,
	dependencies []string,
) bool {
	for _, dependency := range dependencies {
		ready := false
		for _, node := range nodes {
			if node.LogicalNodeID == dependency {
				ready = node.DependencySatisfied
				break
			}
		}
		if !ready {
			return false
		}
	}
	return true
}

func humanMissionMilestone(status string) string {
	switch status {
	case "planned":
		return "Plan ready"
	case "human_required":
		return "Human decision required"
	case "ready_for_review":
		return "Ready for review"
	case "retrying":
		return "Recovery scheduled"
	case "succeeded":
		return "Mission completed"
	default:
		return strings.ReplaceAll(status, "_", " ")
	}
}
