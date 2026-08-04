package queue

import (
	"errors"
	"testing"
)

func validSubmission() JobSubmission {
	return JobSubmission{
		JobID:                "123e4567-e89b-42d3-a456-426614174000",
		Source:               "user_queued",
		DAGNodeID:            "node-a",
		OwnedPaths:           []string{"internal/queue/model.go"},
		MaxAttempts:          3,
		CapabilityKind:       "feature",
		ExitConditions:       []string{"focused tests pass"},
		VerificationStrategy: "focused + race",
		IntegrationStrategy:  "single-integrator",
		EligibilityAuthority: "user",
	}
}

func TestAdmissionRejectsUnmetEligibility(t *testing.T) {
	input := validSubmission()
	input.EligibilityAuthority = ""
	if _, err := CompileSubmission(input); !errors.Is(err, ErrDenied) {
		t.Fatalf("CompileSubmission = %v, want ErrDenied", err)
	}
}

func TestAdmissionRejectsThinCapability(t *testing.T) {
	for _, kind := range []string{"", "wrapper", "adapter", "coordinator", "visual-only"} {
		input := validSubmission()
		input.CapabilityKind = kind
		if _, err := CompileSubmission(input); err == nil {
			t.Fatalf("capability kind %q admitted, want rejection", kind)
		}
	}
}

func TestAdmissionRejectsMissingExitConditions(t *testing.T) {
	input := validSubmission()
	input.ExitConditions = nil
	if _, err := CompileSubmission(input); err == nil {
		t.Fatal("job without exit conditions admitted, want compile error")
	}
}

func TestAdmissionRejectsProtectedAuthorityClaim(t *testing.T) {
	input := validSubmission()
	input.ProtectedAuthorityPaths = []string{"internal/journal/store.go"}
	if _, err := CompileSubmission(input); !errors.Is(err, ErrDenied) {
		t.Fatalf("protected claim = %v, want ErrDenied", err)
	}
}

func TestAdmissionRejectsDAGCycle(t *testing.T) {
	first := queueJobFromCompiled(compiledJobForTest("node-a", []string{"node-b"}))
	candidate := compiledJobForTest("node-b", []string{"node-a"})
	if err := ValidateDAG([]QueueJob{first}, candidate); !errors.Is(err, ErrDAGCycle) {
		t.Fatalf("ValidateDAG = %v, want ErrDAGCycle", err)
	}
}

func TestAdmissionRejectsDuplicateActiveWork(t *testing.T) {
	existing := queueJobFromCompiled(compiledJobForTest("node-a", nil))
	candidate := compiledJobForTest("node-a", nil)
	if err := ValidateDAG([]QueueJob{existing}, candidate); !errors.Is(err, ErrDuplicateWork) {
		t.Fatalf("ValidateDAG = %v, want ErrDuplicateWork", err)
	}
}

func TestAdmissionAllowsTerminalDuplicate(t *testing.T) {
	existing := QueueJob{
		JobID: "old", DAGNodeID: "node-a", Status: StatusIntegrated,
	}
	candidate := compiledJobForTest("node-a", nil)
	if err := ValidateDAG([]QueueJob{existing}, candidate); err != nil {
		t.Fatalf("terminal duplicate should admit, got %v", err)
	}
}

func compiledJobForTest(node string, dependencies []string) CompiledJob {
	job := CompiledJob{
		JobID: "job-" + node, DAGNodeID: node, Dependencies: dependencies,
		OwnedPaths: []string{"internal/queue/model.go"},
	}
	return job
}

func queueJobFromCompiled(job CompiledJob) QueueJob {
	return QueueJob{
		JobID: job.JobID, DAGNodeID: job.DAGNodeID,
		Dependencies: job.Dependencies, Status: StatusAdmitted,
	}
}
