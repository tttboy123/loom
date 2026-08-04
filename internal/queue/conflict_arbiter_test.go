package queue

import "testing"

func TestConflictArbiterSerializesSharedOwnedPath(t *testing.T) {
	left := QueueJob{JobID: "left", Status: StatusAdmitted,
		OwnedPaths: []string{"internal/queue/model.go"}}
	right := QueueJob{JobID: "right", Status: StatusAdmitted,
		OwnedPaths: []string{"internal/queue/model.go", "internal/tui/model.go"}}
	arbiter := NewConflictArbiter()
	if parallel, _ := arbiter.CanRunInParallel(left, right); parallel {
		t.Fatal("jobs sharing an owned path must not run in parallel")
	}
}

func TestConflictArbiterSerializesSharedMutex(t *testing.T) {
	left := QueueJob{JobID: "left", Status: StatusAdmitted,
		MutexKeys: []string{"protocol.go"}}
	right := QueueJob{JobID: "right", Status: StatusAdmitted,
		MutexKeys: []string{"protocol.go"}}
	arbiter := NewConflictArbiter()
	if parallel, _ := arbiter.CanRunInParallel(left, right); parallel {
		t.Fatal("jobs sharing a mutex must not run in parallel")
	}
}

func TestConflictArbiterAllowsDisjointWork(t *testing.T) {
	left := QueueJob{JobID: "left", Status: StatusAdmitted,
		OwnedPaths: []string{"internal/a"}, MutexKeys: []string{"a"}}
	right := QueueJob{JobID: "right", Status: StatusAdmitted,
		OwnedPaths: []string{"internal/b"}, MutexKeys: []string{"b"}}
	arbiter := NewConflictArbiter()
	if parallel, reason := arbiter.CanRunInParallel(left, right); !parallel {
		t.Fatalf("disjoint jobs should run in parallel, reason = %q", reason)
	}
}

func TestConflictArbiterRejectsResourceOversell(t *testing.T) {
	left := QueueJob{JobID: "left", Status: StatusAdmitted,
		ResourceClaims: ResourceClaims{Runtime: "pi", Slots: 2, Model: "m"}}
	right := QueueJob{JobID: "right", Status: StatusAdmitted,
		ResourceClaims: ResourceClaims{Runtime: "pi", Slots: 2, Model: "m"}}
	arbiter := NewConflictArbiter()
	if parallel, _ := arbiter.CanRunInParallel(left, right); parallel {
		t.Fatal("slot oversell must not run in parallel")
	}
}
