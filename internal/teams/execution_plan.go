package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidExecutionPlan     = errors.New("invalid Team execution plan")
	ErrExecutionDependencyCycle = errors.New("Team execution dependency cycle")
	ErrInvalidExecutionState    = errors.New("invalid Team execution state")
)

type ExecutionRole string

const (
	ExecutionRoleMain     ExecutionRole = "main"
	ExecutionRoleSubAgent ExecutionRole = "subagent"
)

type ExecutionNodeInput struct {
	LogicalNodeID     string
	Title             string
	AgentInstanceID   string
	RuntimeInstanceID string
	Role              ExecutionRole
	DependsOn         []string
	MaxAttempts       int
}

type ExecutionPlanInput struct {
	TeamInstanceID string
	Nodes          []ExecutionNodeInput
}

type ExecutionNode struct {
	logicalNodeID     string
	title             string
	agentInstanceID   string
	runtimeInstanceID string
	role              ExecutionRole
	dependsOn         []string
	maxAttempts       int
}

type ExecutionPlan struct {
	teamInstanceID string
	nodes          []ExecutionNode
	digest         string
}

type ExecutionNodeState struct {
	LogicalNodeID  string
	Status         string
	CurrentAttempt int
	RetryAt        time.Time
}

type RuntimeCapacityState struct {
	RuntimeInstanceID string
	Capacity          int
	Active            int
}

func BuildExecutionPlan(input ExecutionPlanInput) (ExecutionPlan, error) {
	if !validExecutionID(input.TeamInstanceID) ||
		len(input.Nodes) == 0 || len(input.Nodes) > 3 {
		return ExecutionPlan{}, ErrInvalidExecutionPlan
	}
	nodes := make([]ExecutionNode, len(input.Nodes))
	nodeIDs := make(map[string]struct{}, len(input.Nodes))
	agentIDs := make(map[string]struct{}, len(input.Nodes))
	mainCount := 0
	for index, raw := range input.Nodes {
		if !validExecutionID(raw.LogicalNodeID) ||
			!validExecutionText(raw.Title) ||
			!validExecutionID(raw.AgentInstanceID) ||
			!validExecutionID(raw.RuntimeInstanceID) ||
			raw.MaxAttempts < 1 || raw.MaxAttempts > 3 ||
			raw.Role != ExecutionRoleMain && raw.Role != ExecutionRoleSubAgent {
			return ExecutionPlan{}, ErrInvalidExecutionPlan
		}
		if _, exists := nodeIDs[raw.LogicalNodeID]; exists {
			return ExecutionPlan{}, ErrInvalidExecutionPlan
		}
		if _, exists := agentIDs[raw.AgentInstanceID]; exists {
			return ExecutionPlan{}, ErrInvalidExecutionPlan
		}
		nodeIDs[raw.LogicalNodeID] = struct{}{}
		agentIDs[raw.AgentInstanceID] = struct{}{}
		if raw.Role == ExecutionRoleMain {
			mainCount++
		}
		dependencies := append([]string(nil), raw.DependsOn...)
		sort.Strings(dependencies)
		for dependencyIndex, dependency := range dependencies {
			if !validExecutionID(dependency) ||
				dependency == raw.LogicalNodeID ||
				dependencyIndex > 0 && dependency == dependencies[dependencyIndex-1] {
				return ExecutionPlan{}, ErrInvalidExecutionPlan
			}
		}
		nodes[index] = ExecutionNode{
			logicalNodeID:     raw.LogicalNodeID,
			title:             raw.Title,
			agentInstanceID:   raw.AgentInstanceID,
			runtimeInstanceID: raw.RuntimeInstanceID,
			role:              raw.Role,
			dependsOn:         dependencies,
			maxAttempts:       raw.MaxAttempts,
		}
	}
	if mainCount != 1 {
		return ExecutionPlan{}, ErrInvalidExecutionPlan
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].logicalNodeID < nodes[j].logicalNodeID
	})
	for _, node := range nodes {
		for _, dependency := range node.dependsOn {
			if _, exists := nodeIDs[dependency]; !exists {
				return ExecutionPlan{}, ErrInvalidExecutionPlan
			}
		}
	}
	if executionPlanHasCycle(nodes) {
		return ExecutionPlan{}, ErrExecutionDependencyCycle
	}
	encoded, err := json.Marshal(struct {
		TeamInstanceID string              `json:"team_instance_id"`
		Nodes          []executionNodeJSON `json:"nodes"`
	}{
		TeamInstanceID: input.TeamInstanceID,
		Nodes:          executionNodesJSON(nodes),
	})
	if err != nil {
		return ExecutionPlan{}, ErrInvalidExecutionPlan
	}
	digest := sha256.Sum256(encoded)
	return ExecutionPlan{
		teamInstanceID: input.TeamInstanceID,
		nodes:          cloneExecutionNodes(nodes),
		digest:         hex.EncodeToString(digest[:]),
	}, nil
}

func (plan ExecutionPlan) TeamInstanceID() string { return plan.teamInstanceID }
func (plan ExecutionPlan) Nodes() []ExecutionNode { return cloneExecutionNodes(plan.nodes) }
func (plan ExecutionPlan) Digest() string         { return plan.digest }

func (node ExecutionNode) LogicalNodeID() string     { return node.logicalNodeID }
func (node ExecutionNode) Title() string             { return node.title }
func (node ExecutionNode) AgentInstanceID() string   { return node.agentInstanceID }
func (node ExecutionNode) RuntimeInstanceID() string { return node.runtimeInstanceID }
func (node ExecutionNode) Role() ExecutionRole       { return node.role }
func (node ExecutionNode) DependsOn() []string       { return append([]string(nil), node.dependsOn...) }
func (node ExecutionNode) MaxAttempts() int          { return node.maxAttempts }

func ReadyExecutionNodes(
	plan ExecutionPlan,
	states []ExecutionNodeState,
	capacities []RuntimeCapacityState,
	now time.Time,
) ([]ExecutionNode, error) {
	if !validExecutionID(plan.teamInstanceID) ||
		len(plan.nodes) == 0 ||
		len(plan.digest) != 64 ||
		now.IsZero() || now.Location() != time.UTC {
		return nil, ErrInvalidExecutionState
	}
	stateByNode := make(map[string]ExecutionNodeState, len(states))
	for _, state := range states {
		if !validExecutionID(state.LogicalNodeID) ||
			state.CurrentAttempt < 0 || state.CurrentAttempt > 3 ||
			!validExecutionNodeStatus(state.Status) {
			return nil, ErrInvalidExecutionState
		}
		if !state.RetryAt.IsZero() && state.RetryAt.Location() != time.UTC {
			return nil, ErrInvalidExecutionState
		}
		if (state.Status == "retry_scheduled" || state.Status == "fallback_scheduled") != !state.RetryAt.IsZero() {
			return nil, ErrInvalidExecutionState
		}
		if _, exists := stateByNode[state.LogicalNodeID]; exists {
			return nil, ErrInvalidExecutionState
		}
		stateByNode[state.LogicalNodeID] = state
	}
	if len(stateByNode) != len(plan.nodes) {
		return nil, ErrInvalidExecutionState
	}
	remaining := make(map[string]int, len(capacities))
	for _, capacity := range capacities {
		if !validExecutionID(capacity.RuntimeInstanceID) ||
			capacity.Capacity < 0 || capacity.Active < 0 ||
			capacity.Active > capacity.Capacity {
			return nil, ErrInvalidExecutionState
		}
		if _, exists := remaining[capacity.RuntimeInstanceID]; exists {
			return nil, ErrInvalidExecutionState
		}
		remaining[capacity.RuntimeInstanceID] = capacity.Capacity - capacity.Active
	}
	ready := make([]ExecutionNode, 0, len(plan.nodes))
	for _, node := range plan.nodes {
		state, exists := stateByNode[node.logicalNodeID]
		if !exists || state.CurrentAttempt > node.maxAttempts {
			continue
		}
		switch state.Status {
		case "pending":
		case "retry_scheduled", "fallback_scheduled":
			if now.Before(state.RetryAt) {
				continue
			}
		default:
			continue
		}
		dependenciesReady := true
		for _, dependency := range node.dependsOn {
			dependencyState, exists := stateByNode[dependency]
			if !exists ||
				dependencyState.Status != "succeeded" &&
					dependencyState.Status != "degraded" {
				dependenciesReady = false
				break
			}
		}
		if !dependenciesReady || remaining[node.runtimeInstanceID] <= 0 {
			continue
		}
		remaining[node.runtimeInstanceID]--
		ready = append(ready, cloneExecutionNode(node))
	}
	return ready, nil
}

type executionNodeJSON struct {
	LogicalNodeID     string        `json:"logical_node_id"`
	Title             string        `json:"title"`
	AgentInstanceID   string        `json:"agent_instance_id"`
	RuntimeInstanceID string        `json:"runtime_instance_id"`
	Role              ExecutionRole `json:"role"`
	DependsOn         []string      `json:"depends_on"`
	MaxAttempts       int           `json:"max_attempts"`
}

func executionNodesJSON(nodes []ExecutionNode) []executionNodeJSON {
	encoded := make([]executionNodeJSON, len(nodes))
	for index, node := range nodes {
		encoded[index] = executionNodeJSON{
			LogicalNodeID: node.logicalNodeID, Title: node.title,
			AgentInstanceID: node.agentInstanceID, RuntimeInstanceID: node.runtimeInstanceID,
			Role: node.role, DependsOn: append([]string(nil), node.dependsOn...),
			MaxAttempts: node.maxAttempts,
		}
	}
	return encoded
}

func executionPlanHasCycle(nodes []ExecutionNode) bool {
	dependencies := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		dependencies[node.logicalNodeID] = node.dependsOn
	}
	state := make(map[string]uint8, len(nodes))
	var visit func(string) bool
	visit = func(nodeID string) bool {
		switch state[nodeID] {
		case 1:
			return true
		case 2:
			return false
		}
		state[nodeID] = 1
		for _, dependency := range dependencies[nodeID] {
			if visit(dependency) {
				return true
			}
		}
		state[nodeID] = 2
		return false
	}
	for _, node := range nodes {
		if visit(node.logicalNodeID) {
			return true
		}
	}
	return false
}

func validExecutionNodeStatus(status string) bool {
	switch status {
	case "pending", "running", "awaiting_recovery", "retry_scheduled",
		"fallback_scheduled", "degraded", "blocked", "human_required",
		"succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func validExecutionID(value string) bool {
	return value != "" && len(value) <= 128 && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value
}

func validExecutionText(value string) bool {
	return value != "" && len(value) <= 4096 && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value
}

func cloneExecutionNodes(nodes []ExecutionNode) []ExecutionNode {
	cloned := make([]ExecutionNode, len(nodes))
	for index, node := range nodes {
		cloned[index] = cloneExecutionNode(node)
	}
	return cloned
}

func cloneExecutionNode(node ExecutionNode) ExecutionNode {
	node.dependsOn = append([]string(nil), node.dependsOn...)
	return node
}
