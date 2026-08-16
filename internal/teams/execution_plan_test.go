package teams

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/assets"
)

func TestExecutionPlanSupportsGovernedTeamLimit(t *testing.T) {
	nodes := make([]ExecutionNodeInput, 0, MaxTeamAgentCount)
	dependencies := make([]string, 0, MaxTeamAgentCount-1)
	for index := 1; index < MaxTeamAgentCount; index++ {
		logicalID := fmt.Sprintf("sub-%d", index)
		dependencies = append(dependencies, logicalID)
		nodes = append(nodes, ExecutionNodeInput{
			LogicalNodeID: logicalID, Title: fmt.Sprintf("Sub %d", index),
			AgentInstanceID:   fmt.Sprintf("agent-%d", index),
			RuntimeInstanceID: fmt.Sprintf("runtime-%d", index),
			Role:              ExecutionRoleSubAgent, MaxAttempts: 1,
		})
	}
	nodes = append(nodes, ExecutionNodeInput{
		LogicalNodeID: "main", Title: "Main", AgentInstanceID: "agent-main",
		RuntimeInstanceID: "runtime-main", Role: ExecutionRoleMain,
		DependsOn: dependencies, MaxAttempts: 1,
	})
	if _, err := BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-nine", Nodes: nodes,
	}); err != nil {
		t.Fatalf("nine-Agent plan error = %v", err)
	}
	tooMany := append([]ExecutionNodeInput(nil), nodes...)
	tooMany = append(tooMany, ExecutionNodeInput{
		LogicalNodeID: "sub-9", Title: "Sub 9", AgentInstanceID: "agent-9",
		RuntimeInstanceID: "runtime-9", Role: ExecutionRoleSubAgent, MaxAttempts: 1,
	})
	if _, err := BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-ten", Nodes: tooMany,
	}); !errors.Is(err, ErrInvalidExecutionPlan) {
		t.Fatalf("ten-Agent plan error = %v", err)
	}
}

func TestExecutionPlanFreezesParallelRouteSiblingsAndAggregation(t *testing.T) {
	input := ExecutionPlanInput{
		TeamInstanceID: "team-parallel-route",
		Nodes: []ExecutionNodeInput{
			{
				LogicalNodeID: "route-openai", Title: "Explore with OpenAI",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-openai",
				Role: ExecutionRoleMain, Kind: ExecutionNodeRouteSibling,
				RouteGroupID: "route-group-main", MaxAttempts: 1,
			},
			{
				LogicalNodeID: "route-deepseek", Title: "Explore with DeepSeek",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-deepseek",
				Role: ExecutionRoleMain, Kind: ExecutionNodeRouteSibling,
				RouteGroupID: "route-group-main", MaxAttempts: 1,
			},
			{
				LogicalNodeID: "main", Title: "Aggregate",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-aggregate",
				Role: ExecutionRoleMain, Kind: ExecutionNodeAggregation,
				RouteGroupID: "route-group-main",
				DependsOn:    []string{"route-openai", "route-deepseek"}, MaxAttempts: 1,
			},
		},
	}
	plan, err := BuildExecutionPlan(input)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	input.Nodes[0].RouteGroupID = "mutated"
	nodes := plan.Nodes()
	if len(nodes) != 3 || nodes[0].LogicalNodeID() != "main" ||
		nodes[0].Kind() != ExecutionNodeAggregation ||
		nodes[0].RouteGroupID() != "route-group-main" ||
		nodes[1].Kind() != ExecutionNodeRouteSibling ||
		nodes[2].Kind() != ExecutionNodeRouteSibling {
		t.Fatalf("parallel route nodes = %#v", nodes)
	}
	if plan.AgentCount() != 1 {
		t.Fatalf("AgentCount() = %d, want 1", plan.AgentCount())
	}
	now := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC)
	states := []ExecutionNodeState{
		{LogicalNodeID: "main", Status: "pending"},
		{LogicalNodeID: "route-deepseek", Status: "pending"},
		{LogicalNodeID: "route-openai", Status: "pending"},
	}
	capacities := []RuntimeCapacityState{
		{RuntimeInstanceID: "runtime-aggregate", Capacity: 1},
		{RuntimeInstanceID: "runtime-deepseek", Capacity: 1},
		{RuntimeInstanceID: "runtime-openai", Capacity: 1},
	}
	ready, err := ReadyExecutionNodes(plan, states, capacities, now)
	if err != nil || len(ready) != 2 || ready[0].LogicalNodeID() != "route-deepseek" ||
		ready[1].LogicalNodeID() != "route-openai" {
		t.Fatalf("initial parallel ready set = %#v, %v", ready, err)
	}
	states[1] = ExecutionNodeState{LogicalNodeID: "route-deepseek", Status: "failed", CurrentAttempt: 1}
	states[2] = ExecutionNodeState{LogicalNodeID: "route-openai", Status: "succeeded", CurrentAttempt: 1}
	ready, err = ReadyExecutionNodes(plan, states, capacities, now)
	if err != nil || len(ready) != 0 {
		t.Fatalf("failed sibling must isolate and hold aggregation, ready=%#v error=%v", ready, err)
	}
	states[1] = ExecutionNodeState{LogicalNodeID: "route-deepseek", Status: "succeeded", CurrentAttempt: 1}
	ready, err = ReadyExecutionNodes(plan, states, capacities, now)
	if err != nil || len(ready) != 1 || ready[0].LogicalNodeID() != "main" {
		t.Fatalf("successful sibling set must release aggregation, ready=%#v error=%v", ready, err)
	}
	second, err := BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-parallel-route",
		Nodes: []ExecutionNodeInput{input.Nodes[2], input.Nodes[1], {
			LogicalNodeID: "route-openai", Title: "Explore with OpenAI",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-openai",
			Role: ExecutionRoleMain, Kind: ExecutionNodeRouteSibling,
			RouteGroupID: "route-group-main", MaxAttempts: 1,
		}},
	})
	if err != nil || second.Digest() != plan.Digest() {
		t.Fatalf("reordered plan = %v digest=%q want=%q", err, second.Digest(), plan.Digest())
	}
}

func TestExecutionPlanRejectsInvalidParallelRouteGroupShapes(t *testing.T) {
	valid := []ExecutionNodeInput{
		{
			LogicalNodeID: "route-a", Title: "Route A",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-a",
			Role: ExecutionRoleMain, Kind: ExecutionNodeRouteSibling,
			RouteGroupID: "route-group-main", MaxAttempts: 1,
		},
		{
			LogicalNodeID: "route-b", Title: "Route B",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-b",
			Role: ExecutionRoleMain, Kind: ExecutionNodeRouteSibling,
			RouteGroupID: "route-group-main", MaxAttempts: 1,
		},
		{
			LogicalNodeID: "main", Title: "Aggregate",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main",
			Role: ExecutionRoleMain, Kind: ExecutionNodeAggregation,
			RouteGroupID: "route-group-main",
			DependsOn:    []string{"route-a", "route-b"}, MaxAttempts: 1,
		},
	}
	tests := map[string]func([]ExecutionNodeInput) []ExecutionNodeInput{
		"one sibling":         func(nodes []ExecutionNodeInput) []ExecutionNodeInput { return nodes[1:] },
		"missing aggregation": func(nodes []ExecutionNodeInput) []ExecutionNodeInput { return nodes[:2] },
		"mismatched Agent": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes[1].AgentInstanceID = "agent-other"
			return nodes
		},
		"mismatched role": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes[1].Role = ExecutionRoleSubAgent
			return nodes
		},
		"mismatched group": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes[1].RouteGroupID = "route-group-other"
			return nodes
		},
		"missing source": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes[2].DependsOn = []string{"route-a"}
			return nodes
		},
		"extra source": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes = append(nodes, ExecutionNodeInput{
				LogicalNodeID: "other", Title: "Other", AgentInstanceID: "agent-other",
				RuntimeInstanceID: "runtime-other", Role: ExecutionRoleSubAgent, MaxAttempts: 1,
			})
			nodes[2].DependsOn = append(nodes[2].DependsOn, "other")
			return nodes
		},
		"external sibling consumer": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes = append(nodes, ExecutionNodeInput{
				LogicalNodeID: "other", Title: "Other", AgentInstanceID: "agent-other",
				RuntimeInstanceID: "runtime-other", Role: ExecutionRoleSubAgent,
				DependsOn: []string{"route-a"}, MaxAttempts: 1,
			})
			return nodes
		},
		"standard duplicate Agent": func(nodes []ExecutionNodeInput) []ExecutionNodeInput {
			nodes[0].Kind, nodes[0].RouteGroupID = ExecutionNodeAgent, ""
			return nodes
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			nodes := append([]ExecutionNodeInput(nil), valid...)
			for index := range nodes {
				nodes[index].DependsOn = append([]string(nil), nodes[index].DependsOn...)
			}
			nodes = mutate(nodes)
			if _, err := BuildExecutionPlan(ExecutionPlanInput{
				TeamInstanceID: "team-invalid-route", Nodes: nodes,
			}); !errors.Is(err, ErrInvalidExecutionPlan) {
				t.Fatalf("BuildExecutionPlan() error = %v", err)
			}
		})
	}
}

func TestPropagateInitialExecutionBlocksKeepsHealthySiblingsRunnable(t *testing.T) {
	plan, err := BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-isolation",
		Nodes: []ExecutionNodeInput{
			{
				LogicalNodeID: "main", Title: "Integrate",
				AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main",
				Role: ExecutionRoleMain, DependsOn: []string{"blocked-worker", "healthy-worker"}, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "blocked-worker", Title: "Blocked worker",
				AgentInstanceID: "agent-blocked", RuntimeInstanceID: "runtime-blocked",
				Role: ExecutionRoleSubAgent, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "healthy-worker", Title: "Healthy worker",
				AgentInstanceID: "agent-healthy", RuntimeInstanceID: "runtime-healthy",
				Role: ExecutionRoleSubAgent, MaxAttempts: 1,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := PropagateInitialExecutionBlocks(plan, []InitialExecutionBlock{{
		LogicalNodeID: "blocked-worker", Code: "credential_unavailable",
		Stage: "credential_lease_issue", Reason: "Credential is not verified.", Retryable: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].LogicalNodeID != "blocked-worker" ||
		blocks[1].LogicalNodeID != "main" || blocks[1].Code != "dependency_blocked" ||
		blocks[1].SourceLogicalNodeID != "blocked-worker" {
		t.Fatalf("propagated blocks = %#v", blocks)
	}
	states := InitialExecutionNodeStates(plan, blocks)
	ready, err := ReadyExecutionNodes(plan, states, []RuntimeCapacityState{
		{RuntimeInstanceID: "runtime-main", Capacity: 1},
		{RuntimeInstanceID: "runtime-blocked", Capacity: 1},
		{RuntimeInstanceID: "runtime-healthy", Capacity: 1},
	}, time.Date(2026, 8, 12, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].LogicalNodeID() != "healthy-worker" {
		t.Fatalf("ready nodes = %#v", ready)
	}
}

func TestP3AExecutionPlanInputFreezesExactAssetRevisionSet(t *testing.T) {
	inputType := reflect.TypeOf(ExecutionNodeInput{})
	bindings, found := inputType.FieldByName("AssetRevisionBindings")
	if !found || bindings.Type.Kind() != reflect.Slice {
		t.Fatalf("ExecutionNodeInput AssetRevisionBindings = %#v, %v", bindings, found)
	}
	digest, found := inputType.FieldByName("AssetRevisionSetDigest")
	if !found || digest.Type.Kind() != reflect.String {
		t.Fatalf("ExecutionNodeInput AssetRevisionSetDigest = %#v, %v", digest, found)
	}
	nodeType := reflect.TypeOf(ExecutionNode{})
	if _, found := nodeType.MethodByName("AssetRevisionBindings"); !found {
		t.Fatal("ExecutionNode.AssetRevisionBindings method is missing")
	}
	if _, found := nodeType.MethodByName("AssetRevisionSetDigest"); !found {
		t.Fatal("ExecutionNode.AssetRevisionSetDigest method is missing")
	}
}

func TestP3AExecutionPlanFreezesBindingBytesAcrossLaterInputMutation(t *testing.T) {
	binding := assets.ExactAssetRevisionBinding{
		AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
		RevisionID:   "revision-1",
		SHA256Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SourceScope:  assets.SourceScopeLocal,
	}
	bindingDigest, err := assets.CanonicalAssetRevisionSetDigest([]assets.ExactAssetRevisionBinding{binding})
	if err != nil {
		t.Fatal(err)
	}
	input := ExecutionPlanInput{
		TeamInstanceID: "team-assets",
		Nodes: []ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Main", AgentInstanceID: "agent-main",
			RuntimeInstanceID: "runtime-main", Role: ExecutionRoleMain, MaxAttempts: 1,
			AssetRevisionBindings:  []assets.ExactAssetRevisionBinding{binding},
			AssetRevisionSetDigest: bindingDigest,
		}},
	}
	plan, err := BuildExecutionPlan(input)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	input.Nodes[0].AssetRevisionBindings[0].RevisionID = "revision-later"
	node := plan.Nodes()[0]
	got := node.AssetRevisionBindings()
	if len(got) != 1 || got[0] != binding {
		t.Fatalf("frozen bindings = %#v, want %#v", got, binding)
	}
	if node.AssetRevisionSetDigest() != bindingDigest {
		t.Fatalf("binding digest = %q", node.AssetRevisionSetDigest())
	}
	got[0].RevisionID = "mutated"
	if again := node.AssetRevisionBindings(); len(again) != 1 || again[0] != binding {
		t.Fatalf("accessor mutation escaped: %#v", again)
	}
}

func TestP3AExecutionBindingMergeUsesFrozenPrecedenceAndConflictsWithinLevel(t *testing.T) {
	base := assets.ExactAssetRevisionBinding{
		AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
		RevisionID: "revision-base", SHA256Digest: strings.Repeat("a", 64),
		SourceScope: assets.SourceScopeLocal,
	}
	team := base
	team.RevisionID, team.SHA256Digest = "revision-team", strings.Repeat("b", 64)
	agent := team
	agent.RevisionID, agent.SHA256Digest = "revision-agent", strings.Repeat("c", 64)
	workPackage := agent
	workPackage.RevisionID, workPackage.SHA256Digest = "revision-work", strings.Repeat("d", 64)
	merged, digest, err := MergeExecutionAssetBindings(
		[]assets.ExactAssetRevisionBinding{base, base},
		[]assets.ExactAssetRevisionBinding{team},
		[]assets.ExactAssetRevisionBinding{agent},
		[]assets.ExactAssetRevisionBinding{workPackage},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(merged) != 1 || merged[0] != workPackage || len(digest) != 64 {
		t.Fatalf("merged = %#v digest=%q", merged, digest)
	}
	if _, _, err := MergeExecutionAssetBindings(
		[]assets.ExactAssetRevisionBinding{base, team},
	); !errors.Is(err, ErrInvalidExecutionPlan) {
		t.Fatalf("same-level conflict error = %v", err)
	}
}

func TestExecutionPlanIsImmutableDeterministicAndValidatesGraph(t *testing.T) {
	input := ExecutionPlanInput{
		TeamInstanceID: "team-instance-1",
		Nodes: []ExecutionNodeInput{
			{
				LogicalNodeID:     "main",
				Title:             "Integrate",
				AgentInstanceID:   "agent-main",
				RuntimeInstanceID: "runtime-main",
				Role:              ExecutionRoleMain,
				DependsOn:         []string{"sub-b", "sub-a"},
				MaxAttempts:       1,
			},
			{
				LogicalNodeID:     "sub-b",
				Title:             "Review",
				AgentInstanceID:   "agent-b",
				RuntimeInstanceID: "runtime-b",
				Role:              ExecutionRoleSubAgent,
				MaxAttempts:       3,
			},
			{
				LogicalNodeID:     "sub-a",
				Title:             "Build",
				AgentInstanceID:   "agent-a",
				RuntimeInstanceID: "runtime-a",
				Role:              ExecutionRoleSubAgent,
				MaxAttempts:       2,
			},
		},
	}
	first, err := BuildExecutionPlan(input)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	input.Nodes[0].DependsOn[0] = "mutated"
	second, err := BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-instance-1",
		Nodes: []ExecutionNodeInput{
			{
				LogicalNodeID: "sub-a", Title: "Build", AgentInstanceID: "agent-a",
				RuntimeInstanceID: "runtime-a", Role: ExecutionRoleSubAgent, MaxAttempts: 2,
			},
			{
				LogicalNodeID: "main", Title: "Integrate", AgentInstanceID: "agent-main",
				RuntimeInstanceID: "runtime-main", Role: ExecutionRoleMain,
				DependsOn: []string{"sub-a", "sub-b"}, MaxAttempts: 1,
			},
			{
				LogicalNodeID: "sub-b", Title: "Review", AgentInstanceID: "agent-b",
				RuntimeInstanceID: "runtime-b", Role: ExecutionRoleSubAgent, MaxAttempts: 3,
			},
		},
	})
	if err != nil {
		t.Fatalf("second BuildExecutionPlan() error = %v", err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("digest depends on input order: %s != %s", first.Digest(), second.Digest())
	}
	nodes := first.Nodes()
	if got, want := []string{nodes[0].LogicalNodeID(), nodes[1].LogicalNodeID(), nodes[2].LogicalNodeID()},
		[]string{"main", "sub-a", "sub-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("node order = %v, want %v", got, want)
	}
	dependencies := nodes[0].DependsOn()
	dependencies[0] = "mutated"
	if got := first.Nodes()[0].DependsOn(); !reflect.DeepEqual(got, []string{"sub-a", "sub-b"}) {
		t.Fatalf("dependency mutation escaped plan: %v", got)
	}

	_, err = BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-cycle",
		Nodes: []ExecutionNodeInput{
			{LogicalNodeID: "main", Title: "Main", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main", Role: ExecutionRoleMain, DependsOn: []string{"sub"}, MaxAttempts: 1},
			{LogicalNodeID: "sub", Title: "Sub", AgentInstanceID: "agent-sub", RuntimeInstanceID: "runtime-sub", Role: ExecutionRoleSubAgent, DependsOn: []string{"main"}, MaxAttempts: 1},
		},
	})
	if !errors.Is(err, ErrExecutionDependencyCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestReadyExecutionNodesHonorsDependenciesRetryTimeAndCapacity(t *testing.T) {
	plan, err := BuildExecutionPlan(ExecutionPlanInput{
		TeamInstanceID: "team-ready",
		Nodes: []ExecutionNodeInput{
			{LogicalNodeID: "main", Title: "Main", AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-main", Role: ExecutionRoleMain, DependsOn: []string{"sub-a", "sub-b"}, MaxAttempts: 1},
			{LogicalNodeID: "sub-a", Title: "A", AgentInstanceID: "agent-a", RuntimeInstanceID: "runtime-shared", Role: ExecutionRoleSubAgent, MaxAttempts: 3},
			{LogicalNodeID: "sub-b", Title: "B", AgentInstanceID: "agent-b", RuntimeInstanceID: "runtime-shared", Role: ExecutionRoleSubAgent, MaxAttempts: 3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	states := []ExecutionNodeState{
		{LogicalNodeID: "main", Status: "pending"},
		{LogicalNodeID: "sub-a", Status: "retry_scheduled", CurrentAttempt: 1, RetryAt: now},
		{LogicalNodeID: "sub-b", Status: "retry_scheduled", CurrentAttempt: 1, RetryAt: now.Add(time.Second)},
	}
	ready, err := ReadyExecutionNodes(
		plan,
		states,
		[]RuntimeCapacityState{
			{RuntimeInstanceID: "runtime-main", Capacity: 1, Active: 0},
			{RuntimeInstanceID: "runtime-shared", Capacity: 1, Active: 0},
		},
		now,
	)
	if err != nil {
		t.Fatalf("ReadyExecutionNodes() error = %v", err)
	}
	if len(ready) != 1 || ready[0].LogicalNodeID() != "sub-a" {
		t.Fatalf("ready = %#v, want only sub-a", ready)
	}

	states[1] = ExecutionNodeState{LogicalNodeID: "sub-a", Status: "succeeded", CurrentAttempt: 2}
	states[2] = ExecutionNodeState{LogicalNodeID: "sub-b", Status: "succeeded", CurrentAttempt: 2}
	ready, err = ReadyExecutionNodes(
		plan,
		states,
		[]RuntimeCapacityState{
			{RuntimeInstanceID: "runtime-main", Capacity: 1, Active: 0},
			{RuntimeInstanceID: "runtime-shared", Capacity: 1, Active: 0},
		},
		now,
	)
	if err != nil {
		t.Fatalf("dependency-ready error = %v", err)
	}
	if len(ready) != 1 || ready[0].LogicalNodeID() != "main" {
		t.Fatalf("ready = %#v, want only main", ready)
	}
}
