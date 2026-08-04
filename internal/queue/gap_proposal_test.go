package queue

import (
	"errors"
	"testing"
)

func TestDuplicateGapObservationsConvergeOnOneGapID(t *testing.T) {
	first := GapProposalSubmission{
		SourceType:         "run_failure",
		SourceIDs:          []string{"run-1"},
		SourceDigests:      []string{"abc123"},
		AffectedCapability: "scheduling",
		ObservedBehavior:   "queue stalls",
		ExpectedBehavior:   "queue drains",
		UserImpact:         "delayed work",
		Confidence:         "high",
		Reproducibility:    "always",
		PrivacyClass:       "none",
		ProposedScope:      "SF-W1 admission",
		RiskClass:          "low",
	}
	proposal, err := CompileGap(first)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := Replay(nil)
	if err != nil {
		t.Fatal(err)
	}
	projection.Gaps[proposal.GapID] = proposal
	second := first
	second.SourceIDs = []string{"run-1", "run-2"} // same digest-bound source, extra id
	duplicate, err := CompileGap(second)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.GapID != proposal.GapID {
		t.Fatalf("duplicate observation converged on %s, want %s", duplicate.GapID, proposal.GapID)
	}
}

func TestStaleUnauthorizedEvidenceCannotCreateSuccessor(t *testing.T) {
	proposal := GapProposal{
		GapID:              "gap-1",
		Source:             GapSource{SourceDigests: []string{"authoritative"}},
		AffectedCapability: "scheduling",
		Disposition:        "propose_successor",
		OwnedPathClaims:    []string{"internal/queue/admission.go"},
		ResourceClaims:     ResourceClaims{Runtime: "pi", Slots: 1, Model: "m"},
	}
	request := SuccessorCompileRequest{
		GapID:                 "gap-1",
		ExpectedSourceDigests: []string{"stale"},
		Submission: JobSubmission{
			JobID:                "job-s",
			Source:               "gap_proposal",
			DAGNodeID:            "node-s",
			OwnedPaths:           []string{"internal/queue/admission.go"},
			MaxAttempts:          3,
			CapabilityKind:       "feature",
			ExitConditions:       []string{"focused tests pass"},
			VerificationStrategy: "focused + race",
			IntegrationStrategy:  "single-integrator",
			EligibilityAuthority: "user",
		},
	}
	if _, err := CompileSuccessor(proposal, request); !errors.Is(err, ErrDenied) {
		t.Fatalf("stale evidence successor = %v, want ErrDenied", err)
	}
}

func TestSuccessorCompilationHasNoWorkItemSideEffect(t *testing.T) {
	proposal := GapProposal{
		GapID:              "gap-1",
		Source:             GapSource{SourceDigests: []string{"authoritative"}},
		AffectedCapability: "scheduling",
		Disposition:        "propose_successor",
		OwnedPathClaims:    []string{"internal/queue/admission.go"},
		ResourceClaims:     ResourceClaims{Runtime: "pi", Slots: 1, Model: "m"},
	}
	request := SuccessorCompileRequest{
		GapID:                 "gap-1",
		ExpectedSourceDigests: []string{"authoritative"},
		Submission: JobSubmission{
			JobID: "job-s", Source: "gap_proposal", DAGNodeID: "node-s",
			OwnedPaths:  []string{"internal/queue/admission.go"},
			MaxAttempts: 3, CapabilityKind: "feature",
			ExitConditions:       []string{"focused tests pass"},
			VerificationStrategy: "focused + race",
			IntegrationStrategy:  "single-integrator",
			EligibilityAuthority: "user",
		},
	}
	successor, err := CompileSuccessor(proposal, request)
	if err != nil {
		t.Fatal(err)
	}
	if successor.GapID != "gap-1" || successor.SuccessorProposalID == "" {
		t.Fatalf("successor binding mismatch: %+v", successor)
	}
}
