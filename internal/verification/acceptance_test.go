package verification

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAcceptanceContractRiskDigestAndIsolation(t *testing.T) {
	criteria := []string{"  tests pass  ", "artifact digest matches"}
	contract, err := NewAcceptanceContract(1, criteria, AcceptanceRiskHigh)
	if err != nil {
		t.Fatalf("NewAcceptanceContract() error = %v", err)
	}
	if contract.Version() != 1 ||
		contract.Risk() != AcceptanceRiskHigh ||
		!contract.IndependentVerifierRequired() ||
		len(contract.Digest()) != 64 {
		t.Fatalf("contract = %#v", contract)
	}
	want := []string{"artifact digest matches", "tests pass"}
	if got := contract.Criteria(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Criteria() = %#v, want %#v", got, want)
	}
	criteria[0] = "mutated"
	copied := contract.Criteria()
	copied[0] = "mutated"
	if got := contract.Criteria(); !reflect.DeepEqual(got, want) {
		t.Fatalf("contract mutated through input/accessor: %#v", got)
	}
	again, err := NewAcceptanceContract(
		1,
		[]string{"tests pass", "artifact digest matches"},
		AcceptanceRiskHigh,
	)
	if err != nil || again.Digest() != contract.Digest() {
		t.Fatalf("equal semantics changed digest: %#v %v", again, err)
	}

	for _, test := range []struct {
		name     string
		version  int
		criteria []string
		risk     AcceptanceRisk
	}{
		{"zero version", 0, want, AcceptanceRiskLow},
		{"empty criteria", 1, nil, AcceptanceRiskLow},
		{"duplicate", 1, []string{"same", " same "}, AcceptanceRiskLow},
		{"control", 1, []string{"bad\ncriterion"}, AcceptanceRiskLow},
		{"long", 1, []string{strings.Repeat("x", 257)}, AcceptanceRiskLow},
		{"risk", 1, want, AcceptanceRisk("critical")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewAcceptanceContract(
				test.version,
				test.criteria,
				test.risk,
			); !errors.Is(err, ErrInvalidAcceptanceContract) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDeterministicAndIndependentAcceptance(t *testing.T) {
	low, err := NewAcceptanceContract(
		1,
		[]string{"tests pass"},
		AcceptanceRiskLow,
	)
	if err != nil {
		t.Fatal(err)
	}
	high, err := NewAcceptanceContract(
		1,
		[]string{"tests pass"},
		AcceptanceRiskHigh,
	)
	if err != nil {
		t.Fatal(err)
	}
	base := deterministicAcceptanceInput(low)
	deterministic, err := VerifyDeterministic(low, base)
	if err != nil ||
		deterministic.Kind() != DeterministicAccepted ||
		len(deterministic.Digest()) != 64 {
		t.Fatalf("low deterministic = %#v, %v", deterministic, err)
	}
	decisionTime := time.Date(2026, 7, 26, 18, 0, 0, 0, time.UTC)
	accepted, err := DecideAcceptance(AcceptanceDecisionInput{
		Contract:            low,
		DeterministicResult: deterministic,
		DecisionTime:        decisionTime,
	})
	if err != nil ||
		accepted.Kind() != AcceptanceAccepted ||
		!accepted.DecisionTime().Equal(decisionTime) ||
		len(accepted.Digest()) != 64 {
		t.Fatalf("low decision = %#v, %v", accepted, err)
	}

	highInput := base
	highInput.AcceptanceContractDigest = high.Digest()
	needsVerifier, err := VerifyDeterministic(high, highInput)
	if err != nil ||
		needsVerifier.Kind() != DeterministicNeedsIndependentVerifier {
		t.Fatalf("high deterministic = %#v, %v", needsVerifier, err)
	}
	if _, err := DecideAcceptance(AcceptanceDecisionInput{
		Contract:            high,
		DeterministicResult: needsVerifier,
		DecisionTime:        decisionTime,
	}); !errors.Is(err, ErrInvalidAcceptanceDecision) {
		t.Fatalf("missing verifier error = %v", err)
	}

	candidate, err := VerifierCandidateFromTerminal(VerifierTerminalInput{
		WorkItemID:          "verify-work-1",
		RunID:               "verify-run-1",
		ClaimID:             "00000000-0000-4000-8000-000000000041",
		ClaimGeneration:     1,
		RuntimeInstanceID:   "runtime-verify",
		AgentInstanceID:     "agent-verify",
		GrantID:             "00000000-0000-4000-8000-000000000042",
		EvidenceID:          "verify-evidence-1",
		EvidenceDigest:      digestOf("verifier-evidence"),
		OutputSummaryDigest: digestOf("verifier-summary"),
		TerminalStatus:      "succeeded",
		TerminalReason:      "",
		OutputReasonCode:    VerifierReasonCriteriaSatisfied,
	})
	if err != nil ||
		candidate.Kind() != VerifierAccepted ||
		candidate.ReasonCode() != VerifierReasonCriteriaSatisfied {
		t.Fatalf("candidate = %#v, %v", candidate, err)
	}
	accepted, err = DecideAcceptance(AcceptanceDecisionInput{
		Contract:            high,
		DeterministicResult: needsVerifier,
		VerifierCandidate:   candidate,
		DecisionTime:        decisionTime,
	})
	if err != nil || accepted.Kind() != AcceptanceAccepted {
		t.Fatalf("verified accepted = %#v, %v", accepted, err)
	}

	rejectedCandidate, err := VerifierCandidateFromTerminal(VerifierTerminalInput{
		WorkItemID:          "verify-work-1",
		RunID:               "verify-run-1",
		ClaimID:             "00000000-0000-4000-8000-000000000041",
		ClaimGeneration:     1,
		RuntimeInstanceID:   "runtime-verify",
		AgentInstanceID:     "agent-verify",
		GrantID:             "00000000-0000-4000-8000-000000000042",
		EvidenceID:          "verify-evidence-1",
		EvidenceDigest:      digestOf("verifier-evidence"),
		OutputSummaryDigest: digestOf("verifier-summary"),
		TerminalStatus:      "failed",
		TerminalReason:      string(VerifierReasonCriteriaNotSatisfied),
	})
	if err != nil || rejectedCandidate.Kind() != VerifierRejected {
		t.Fatalf("rejected candidate = %#v, %v", rejectedCandidate, err)
	}
	rejected, err := DecideAcceptance(AcceptanceDecisionInput{
		Contract:            high,
		DeterministicResult: needsVerifier,
		VerifierCandidate:   rejectedCandidate,
		DecisionTime:        decisionTime,
	})
	if err != nil || rejected.Kind() != AcceptanceRejected {
		t.Fatalf("verified rejected = %#v, %v", rejected, err)
	}
}

func TestVerifierCandidateUsesSuccessfulOutputVerdict(t *testing.T) {
	input := VerifierTerminalInput{
		WorkItemID:          "verify-work-1",
		RunID:               "verify-run-1",
		ClaimID:             "00000000-0000-4000-8000-000000000041",
		ClaimGeneration:     1,
		RuntimeInstanceID:   "runtime-verify",
		AgentInstanceID:     "agent-verify",
		GrantID:             "00000000-0000-4000-8000-000000000042",
		EvidenceID:          "verify-evidence-1",
		EvidenceDigest:      digestOf("verifier-evidence"),
		OutputSummaryDigest: digestOf("verifier-summary"),
		TerminalStatus:      "succeeded",
		OutputReasonCode:    VerifierReasonCriteriaNotSatisfied,
	}
	candidate, err := VerifierCandidateFromTerminal(input)
	if err != nil || candidate.Kind() != VerifierRejected ||
		candidate.ReasonCode() != VerifierReasonCriteriaNotSatisfied ||
		candidate.Binding().TerminalStatus != "succeeded" ||
		candidate.Binding().TerminalReason != "" {
		t.Fatalf("successful negative verdict = %#v, %v", candidate, err)
	}

	input.OutputReasonCode = ""
	if _, err := VerifierCandidateFromTerminal(input); !errors.Is(
		err, ErrInvalidVerifierCandidate,
	) {
		t.Fatalf("missing output verdict error = %v", err)
	}
}

func TestVerifierCandidateMapsOperationalFailureToInsufficientEvidence(t *testing.T) {
	input := VerifierTerminalInput{
		WorkItemID:          "verify-work-1",
		RunID:               "verify-run-1",
		ClaimID:             "00000000-0000-4000-8000-000000000041",
		ClaimGeneration:     1,
		RuntimeInstanceID:   "runtime-verify",
		AgentInstanceID:     "agent-verify",
		GrantID:             "00000000-0000-4000-8000-000000000042",
		EvidenceID:          "verify-evidence-1",
		EvidenceDigest:      digestOf("verifier-evidence"),
		OutputSummaryDigest: digestOf("verifier-summary"),
		TerminalStatus:      "failed",
		TerminalReason:      "runtime_process_failed",
	}
	candidate, err := VerifierCandidateFromTerminal(input)
	if err != nil || candidate.Kind() != VerifierRejected ||
		candidate.ReasonCode() != VerifierReasonInsufficientEvidence ||
		candidate.Binding().TerminalStatus != "failed" ||
		candidate.Binding().TerminalReason != "runtime_process_failed" ||
		!candidate.Valid() {
		t.Fatalf("operational failure candidate = %#v, %v", candidate, err)
	}
	firstDigest := candidate.Digest()
	input.TerminalReason = "runtime_timeout"
	timedOut, err := VerifierCandidateFromTerminal(input)
	if err != nil || timedOut.ReasonCode() != VerifierReasonInsufficientEvidence ||
		timedOut.Digest() == firstDigest {
		t.Fatalf("timeout candidate = %#v, %v", timedOut, err)
	}

	input.TerminalStatus = "cancelled"
	input.TerminalReason = "operator_cancelled"
	cancelled, err := VerifierCandidateFromTerminal(input)
	if err != nil || cancelled.ReasonCode() != VerifierReasonInsufficientEvidence ||
		cancelled.Binding().TerminalReason != "operator_cancelled" {
		t.Fatalf("cancelled candidate = %#v, %v", cancelled, err)
	}
}

func TestAcceptanceMutationAndMismatchFailClosed(t *testing.T) {
	contract, err := NewAcceptanceContract(
		1,
		[]string{"tests pass"},
		AcceptanceRiskLow,
	)
	if err != nil {
		t.Fatal(err)
	}
	valid := deterministicAcceptanceInput(contract)
	mutations := []struct {
		name   string
		mutate func(*DeterministicVerificationInput)
	}{
		{"team", func(input *DeterministicVerificationInput) { input.TeamInstanceID = "" }},
		{"plan", func(input *DeterministicVerificationInput) { input.PlanDigest = "not-a-digest" }},
		{"attempt", func(input *DeterministicVerificationInput) { input.AttemptNumber = 0 }},
		{"generation", func(input *DeterministicVerificationInput) { input.ClaimGeneration = 0 }},
		{"evidence", func(input *DeterministicVerificationInput) { input.SourceEvidenceDigest = "" }},
		{"summary", func(input *DeterministicVerificationInput) { input.OutputSummaryDigest = "" }},
		{"contract", func(input *DeterministicVerificationInput) { input.AcceptanceContractDigest = digestOf("other") }},
		{"classification", func(input *DeterministicVerificationInput) { input.OutputClassification = OutputInvalid }},
		{"terminal", func(input *DeterministicVerificationInput) { input.TerminalStatus = "failed" }},
	}
	for _, test := range mutations {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if _, err := VerifyDeterministic(
				contract,
				input,
			); !errors.Is(err, ErrInvalidDeterministicVerification) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func deterministicAcceptanceInput(
	contract AcceptanceContract,
) DeterministicVerificationInput {
	return DeterministicVerificationInput{
		TeamInstanceID:             "team-1",
		PlanDigest:                 digestOf("plan"),
		LogicalNodeID:              "main",
		AttemptNumber:              1,
		WorkItemID:                 "work-1",
		RunID:                      "run-1",
		ClaimID:                    "00000000-0000-4000-8000-000000000031",
		ClaimGeneration:            1,
		SourceEvidenceID:           "evidence-1",
		SourceEvidenceDigest:       digestOf("evidence"),
		OutputSummaryDigest:        digestOf("summary"),
		OutputContractVersion:      1,
		OutputContractDigest:       digestOf("output-contract"),
		OutputClassification:       OutputValidNonEmpty,
		OutputClassificationDigest: digestOf("classification"),
		AcceptanceContractDigest:   contract.Digest(),
		TerminalStatus:             "succeeded",
	}
}

func digestOf(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
