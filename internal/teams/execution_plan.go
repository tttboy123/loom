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

	"loom-pi-rebuild/internal/assets"
)

var (
	ErrInvalidExecutionPlan     = errors.New("invalid Team execution plan")
	ErrExecutionDependencyCycle = errors.New("Team execution dependency cycle")
	ErrInvalidExecutionState    = errors.New("invalid Team execution state")
)

type ExecutionRole string

type ExecutionNodeKind string

const (
	ExecutionRoleMain         ExecutionRole     = "main"
	ExecutionRoleSubAgent     ExecutionRole     = "subagent"
	ExecutionNodeAgent        ExecutionNodeKind = ""
	ExecutionNodeRouteSibling ExecutionNodeKind = "route_sibling"
	ExecutionNodeAggregation  ExecutionNodeKind = "aggregation"
	MaxTeamAgentCount                           = 9
	MaxExecutionNodeCount                       = MaxTeamAgentCount * 3
)

type ExecutionNodeInput struct {
	LogicalNodeID          string
	Title                  string
	AgentInstanceID        string
	RuntimeInstanceID      string
	Role                   ExecutionRole
	Kind                   ExecutionNodeKind
	RouteGroupID           string
	DependsOn              []string
	MaxAttempts            int
	AssetRevisionBindings  []assets.ExactAssetRevisionBinding
	AssetRevisionSetDigest string
}

type ExecutionPlanInput struct {
	TeamInstanceID string
	Nodes          []ExecutionNodeInput
}

type ExecutionNode struct {
	logicalNodeID          string
	title                  string
	agentInstanceID        string
	runtimeInstanceID      string
	role                   ExecutionRole
	kind                   ExecutionNodeKind
	routeGroupID           string
	dependsOn              []string
	maxAttempts            int
	assetRevisionBindings  []assets.ExactAssetRevisionBinding
	assetRevisionSetDigest string
}

type ExecutionPlan struct {
	teamInstanceID string
	nodes          []ExecutionNode
	agentCount     int
	digest         string
}

type ExecutionNodeState struct {
	LogicalNodeID  string
	Status         string
	CurrentAttempt int
	RetryAt        time.Time
}

type InitialExecutionBlock struct {
	LogicalNodeID       string
	Code                string
	Stage               string
	Reason              string
	Retryable           bool
	SourceLogicalNodeID string
}

type RuntimeCapacityState struct {
	RuntimeInstanceID string
	Capacity          int
	Active            int
}

func BuildExecutionPlan(input ExecutionPlanInput) (ExecutionPlan, error) {
	if !validExecutionID(input.TeamInstanceID) ||
		len(input.Nodes) == 0 || len(input.Nodes) > MaxExecutionNodeCount {
		return ExecutionPlan{}, ErrInvalidExecutionPlan
	}
	nodes := make([]ExecutionNode, len(input.Nodes))
	nodeIDs := make(map[string]struct{}, len(input.Nodes))
	agentIDs := make(map[string]struct{}, MaxTeamAgentCount)
	mainCount := 0
	for index, raw := range input.Nodes {
		if !validExecutionID(raw.LogicalNodeID) ||
			!validExecutionText(raw.Title) ||
			!validExecutionID(raw.AgentInstanceID) ||
			!validExecutionID(raw.RuntimeInstanceID) ||
			raw.MaxAttempts < 1 || raw.MaxAttempts > 3 ||
			raw.Role != ExecutionRoleMain && raw.Role != ExecutionRoleSubAgent ||
			raw.Kind != ExecutionNodeAgent &&
				raw.Kind != ExecutionNodeRouteSibling &&
				raw.Kind != ExecutionNodeAggregation ||
			raw.Kind == ExecutionNodeAgent && raw.RouteGroupID != "" ||
			raw.Kind != ExecutionNodeAgent && !validExecutionID(raw.RouteGroupID) {
			return ExecutionPlan{}, ErrInvalidExecutionPlan
		}
		if _, exists := nodeIDs[raw.LogicalNodeID]; exists {
			return ExecutionPlan{}, ErrInvalidExecutionPlan
		}
		assetBindings := append([]assets.ExactAssetRevisionBinding(nil), raw.AssetRevisionBindings...)
		if len(assetBindings) == 0 {
			if raw.AssetRevisionSetDigest != "" {
				return ExecutionPlan{}, ErrInvalidExecutionPlan
			}
		} else {
			digest, digestErr := assets.CanonicalAssetRevisionSetDigest(assetBindings)
			if digestErr != nil || digest != raw.AssetRevisionSetDigest {
				return ExecutionPlan{}, ErrInvalidExecutionPlan
			}
			canonicalJSON, _ := assets.CanonicalAssetRevisionSetJSON(assetBindings)
			_ = json.Unmarshal(canonicalJSON, &assetBindings)
		}
		nodeIDs[raw.LogicalNodeID] = struct{}{}
		agentIDs[raw.AgentInstanceID] = struct{}{}
		if len(agentIDs) > MaxTeamAgentCount {
			return ExecutionPlan{}, ErrInvalidExecutionPlan
		}
		if raw.Role == ExecutionRoleMain && raw.Kind != ExecutionNodeRouteSibling {
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
			logicalNodeID:          raw.LogicalNodeID,
			title:                  raw.Title,
			agentInstanceID:        raw.AgentInstanceID,
			runtimeInstanceID:      raw.RuntimeInstanceID,
			role:                   raw.Role,
			kind:                   raw.Kind,
			routeGroupID:           raw.RouteGroupID,
			dependsOn:              dependencies,
			maxAttempts:            raw.MaxAttempts,
			assetRevisionBindings:  assetBindings,
			assetRevisionSetDigest: raw.AssetRevisionSetDigest,
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
	if !validParallelRouteGroups(nodes) {
		return ExecutionPlan{}, ErrInvalidExecutionPlan
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
		agentCount:     len(agentIDs),
		digest:         hex.EncodeToString(digest[:]),
	}, nil
}

// MergeExecutionAssetBindings applies precedence levels from lowest to
// highest. Identical tuples deduplicate, competing tuples at one level fail,
// and a higher level may intentionally replace the same asset definition.
func MergeExecutionAssetBindings(
	levels ...[]assets.ExactAssetRevisionBinding,
) ([]assets.ExactAssetRevisionBinding, string, error) {
	merged := make(map[string]assets.ExactAssetRevisionBinding)
	for _, rawLevel := range levels {
		level := make(map[string]assets.ExactAssetRevisionBinding)
		for _, binding := range rawLevel {
			key := string(binding.AssetKind) + "\x00" + binding.DefinitionID
			if existing, found := level[key]; found {
				if existing != binding {
					return nil, "", ErrInvalidExecutionPlan
				}
				continue
			}
			level[key] = binding
		}
		validated := make([]assets.ExactAssetRevisionBinding, 0, len(level))
		for _, binding := range level {
			validated = append(validated, binding)
		}
		if _, err := assets.CanonicalAssetRevisionSetDigest(validated); err != nil {
			return nil, "", ErrInvalidExecutionPlan
		}
		for key, binding := range level {
			merged[key] = binding
		}
	}
	result := make([]assets.ExactAssetRevisionBinding, 0, len(merged))
	for _, binding := range merged {
		result = append(result, binding)
	}
	if len(result) == 0 {
		return []assets.ExactAssetRevisionBinding{}, "", nil
	}
	canonical, err := assets.CanonicalAssetRevisionSetJSON(result)
	if err != nil {
		return nil, "", ErrInvalidExecutionPlan
	}
	if err := json.Unmarshal(canonical, &result); err != nil {
		return nil, "", ErrInvalidExecutionPlan
	}
	digest, err := assets.CanonicalAssetRevisionSetDigest(result)
	if err != nil {
		return nil, "", ErrInvalidExecutionPlan
	}
	return result, digest, nil
}

func (plan ExecutionPlan) TeamInstanceID() string { return plan.teamInstanceID }
func (plan ExecutionPlan) Nodes() []ExecutionNode { return cloneExecutionNodes(plan.nodes) }
func (plan ExecutionPlan) AgentCount() int        { return plan.agentCount }
func (plan ExecutionPlan) Digest() string         { return plan.digest }

func (node ExecutionNode) LogicalNodeID() string     { return node.logicalNodeID }
func (node ExecutionNode) Title() string             { return node.title }
func (node ExecutionNode) AgentInstanceID() string   { return node.agentInstanceID }
func (node ExecutionNode) RuntimeInstanceID() string { return node.runtimeInstanceID }
func (node ExecutionNode) Role() ExecutionRole       { return node.role }
func (node ExecutionNode) Kind() ExecutionNodeKind   { return node.kind }
func (node ExecutionNode) RouteGroupID() string      { return node.routeGroupID }
func (node ExecutionNode) DependsOn() []string       { return append([]string(nil), node.dependsOn...) }
func (node ExecutionNode) MaxAttempts() int          { return node.maxAttempts }
func (node ExecutionNode) AssetRevisionBindings() []assets.ExactAssetRevisionBinding {
	if node.assetRevisionBindings == nil {
		return []assets.ExactAssetRevisionBinding{}
	}
	return append([]assets.ExactAssetRevisionBinding{}, node.assetRevisionBindings...)
}
func (node ExecutionNode) AssetRevisionSetDigest() string { return node.assetRevisionSetDigest }

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

func PropagateInitialExecutionBlocks(
	plan ExecutionPlan,
	direct []InitialExecutionBlock,
) ([]InitialExecutionBlock, error) {
	if !validExecutionID(plan.teamInstanceID) || len(plan.nodes) == 0 ||
		len(plan.digest) != 64 || len(direct) > len(plan.nodes) {
		return nil, ErrInvalidExecutionState
	}
	nodeByID := make(map[string]ExecutionNode, len(plan.nodes))
	for _, node := range plan.nodes {
		nodeByID[node.logicalNodeID] = node
	}
	initial := append([]InitialExecutionBlock(nil), direct...)
	sort.Slice(initial, func(i, j int) bool {
		return initial[i].LogicalNodeID < initial[j].LogicalNodeID
	})
	blocked := make(map[string]InitialExecutionBlock, len(plan.nodes))
	result := make([]InitialExecutionBlock, 0, len(plan.nodes))
	for index, block := range initial {
		if !validInitialExecutionBlock(block, false) ||
			index > 0 && block.LogicalNodeID == initial[index-1].LogicalNodeID {
			return nil, ErrInvalidExecutionState
		}
		if _, exists := nodeByID[block.LogicalNodeID]; !exists {
			return nil, ErrInvalidExecutionState
		}
		blocked[block.LogicalNodeID] = block
		result = append(result, block)
	}
	for {
		changed := false
		for _, node := range plan.nodes {
			if _, exists := blocked[node.logicalNodeID]; exists {
				continue
			}
			for _, dependency := range node.dependsOn {
				source, exists := blocked[dependency]
				if !exists {
					continue
				}
				block := InitialExecutionBlock{
					LogicalNodeID: node.logicalNodeID, Code: "dependency_blocked",
					Stage:     "agent_attempt_dispatch",
					Reason:    "A required Agent is blocked.",
					Retryable: source.Retryable, SourceLogicalNodeID: dependency,
				}
				blocked[node.logicalNodeID] = block
				result = append(result, block)
				changed = true
				break
			}
		}
		if !changed {
			break
		}
	}
	return result, nil
}

func ValidateInitialExecutionBlocks(
	plan ExecutionPlan,
	blocks []InitialExecutionBlock,
) ([]InitialExecutionBlock, error) {
	direct := make([]InitialExecutionBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Code != "dependency_blocked" {
			direct = append(direct, block)
		}
	}
	canonical, err := PropagateInitialExecutionBlocks(plan, direct)
	if err != nil || !equalInitialExecutionBlocks(canonical, blocks) {
		return nil, ErrInvalidExecutionState
	}
	return append([]InitialExecutionBlock(nil), canonical...), nil
}

func InitialExecutionNodeStates(
	plan ExecutionPlan,
	blocks []InitialExecutionBlock,
) []ExecutionNodeState {
	blocked := make(map[string]struct{}, len(blocks))
	for _, block := range blocks {
		blocked[block.LogicalNodeID] = struct{}{}
	}
	states := make([]ExecutionNodeState, 0, len(plan.nodes))
	for _, node := range plan.nodes {
		status := "pending"
		if _, exists := blocked[node.logicalNodeID]; exists {
			status = "blocked"
		}
		states = append(states, ExecutionNodeState{
			LogicalNodeID: node.logicalNodeID, Status: status,
		})
	}
	return states
}

type executionNodeJSON struct {
	LogicalNodeID          string                             `json:"logical_node_id"`
	Title                  string                             `json:"title"`
	AgentInstanceID        string                             `json:"agent_instance_id"`
	RuntimeInstanceID      string                             `json:"runtime_instance_id"`
	Role                   ExecutionRole                      `json:"role"`
	Kind                   ExecutionNodeKind                  `json:"kind,omitempty"`
	RouteGroupID           string                             `json:"route_group_id,omitempty"`
	DependsOn              []string                           `json:"depends_on"`
	MaxAttempts            int                                `json:"max_attempts"`
	AssetRevisionBindings  []assets.ExactAssetRevisionBinding `json:"asset_revision_bindings,omitempty"`
	AssetRevisionSetDigest string                             `json:"asset_revision_set_digest,omitempty"`
}

func executionNodesJSON(nodes []ExecutionNode) []executionNodeJSON {
	encoded := make([]executionNodeJSON, len(nodes))
	for index, node := range nodes {
		encoded[index] = executionNodeJSON{
			LogicalNodeID: node.logicalNodeID, Title: node.title,
			AgentInstanceID: node.agentInstanceID, RuntimeInstanceID: node.runtimeInstanceID,
			Role: node.role, Kind: node.kind, RouteGroupID: node.routeGroupID,
			DependsOn:              append([]string(nil), node.dependsOn...),
			MaxAttempts:            node.maxAttempts,
			AssetRevisionBindings:  append([]assets.ExactAssetRevisionBinding(nil), node.assetRevisionBindings...),
			AssetRevisionSetDigest: node.assetRevisionSetDigest,
		}
	}
	return encoded
}

type executionRouteGroup struct {
	siblings   []ExecutionNode
	aggregator *ExecutionNode
}

func validParallelRouteGroups(nodes []ExecutionNode) bool {
	groups := make(map[string]*executionRouteGroup)
	agentNodes := make(map[string][]ExecutionNode)
	siblingGroupByID := make(map[string]string)
	for index := range nodes {
		node := nodes[index]
		agentNodes[node.agentInstanceID] = append(agentNodes[node.agentInstanceID], node)
		if node.kind == ExecutionNodeAgent {
			continue
		}
		group := groups[node.routeGroupID]
		if group == nil {
			group = &executionRouteGroup{}
			groups[node.routeGroupID] = group
		}
		switch node.kind {
		case ExecutionNodeRouteSibling:
			group.siblings = append(group.siblings, node)
			siblingGroupByID[node.logicalNodeID] = node.routeGroupID
		case ExecutionNodeAggregation:
			if group.aggregator != nil {
				return false
			}
			copy := node
			group.aggregator = &copy
		default:
			return false
		}
	}
	for _, related := range agentNodes {
		if len(related) == 1 {
			continue
		}
		groupID := related[0].routeGroupID
		if groupID == "" {
			return false
		}
		for _, node := range related {
			if node.routeGroupID != groupID || node.kind == ExecutionNodeAgent {
				return false
			}
		}
	}
	for groupID, group := range groups {
		if len(group.siblings) < 2 || group.aggregator == nil {
			return false
		}
		aggregator := *group.aggregator
		siblingIDs := make([]string, 0, len(group.siblings))
		var commonDependencies []string
		for index, sibling := range group.siblings {
			if sibling.agentInstanceID != aggregator.agentInstanceID ||
				sibling.role != aggregator.role ||
				sibling.routeGroupID != groupID {
				return false
			}
			if index == 0 {
				commonDependencies = sibling.dependsOn
			} else if !equalExecutionStrings(commonDependencies, sibling.dependsOn) {
				return false
			}
			siblingIDs = append(siblingIDs, sibling.logicalNodeID)
		}
		sort.Strings(siblingIDs)
		if !equalExecutionStrings(siblingIDs, aggregator.dependsOn) {
			return false
		}
	}
	for _, node := range nodes {
		for _, dependency := range node.dependsOn {
			groupID, sibling := siblingGroupByID[dependency]
			if !sibling ||
				node.kind == ExecutionNodeAggregation && node.routeGroupID == groupID {
				continue
			}
			return false
		}
	}
	return true
}

func equalExecutionStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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

func validInitialExecutionBlock(block InitialExecutionBlock, dependency bool) bool {
	if !validExecutionID(block.LogicalNodeID) ||
		!validExecutionToken(block.Code) || !validExecutionToken(block.Stage) ||
		!validExecutionReason(block.Reason) {
		return false
	}
	if dependency {
		return block.Code == "dependency_blocked" &&
			validExecutionID(block.SourceLogicalNodeID) &&
			block.SourceLogicalNodeID != block.LogicalNodeID
	}
	return block.Code != "dependency_blocked" && block.SourceLogicalNodeID == ""
}

func equalInitialExecutionBlocks(left, right []InitialExecutionBlock) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validExecutionToken(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if current != '_' &&
			(current < 'a' || current > 'z') &&
			(current < '0' || current > '9') {
			return false
		}
	}
	return true
}

func validExecutionReason(value string) bool {
	return value != "" && len(value) <= 512 && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\x00")
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
	node.assetRevisionBindings = append([]assets.ExactAssetRevisionBinding(nil), node.assetRevisionBindings...)
	return node
}
