package teams

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

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
