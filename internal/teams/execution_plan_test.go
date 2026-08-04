package teams

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/assets"
)

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
