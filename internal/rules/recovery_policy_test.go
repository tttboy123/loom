package rules

import (
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/verification"
)

var recoveryDecisionTime = time.Date(2026, 7, 26, 15, 0, 0, 0, time.UTC)

func TestRecoveryPolicyProducesBoundedExactDecisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		classification   verification.OutputClassification
		policy           RecoveryPolicyInput
		attempt          int
		credits          int
		fallbackConsumed bool
		wantAction       RecoveryAction
		wantNextAttempt  int
		wantFallback     string
		wantCreditsAfter int
		wantRetryAt      time.Time
	}{
		{
			name:           "valid output has no recovery",
			classification: verification.OutputValidNonEmpty,
			policy:         testRecoveryPolicyInput(),
			attempt:        1, credits: 2, wantAction: RecoveryNone,
			wantCreditsAfter: 2,
		},
		{
			name:           "transient empty retries",
			classification: verification.OutputTransientEmpty,
			policy:         testRecoveryPolicyInput(),
			attempt:        1, credits: 2, wantAction: RecoveryRetry,
			wantNextAttempt: 2, wantCreditsAfter: 1,
			wantRetryAt: recoveryDecisionTime.Add(5 * time.Second),
		},
		{
			name:           "invalid uses explicit workflow fallback",
			classification: verification.OutputInvalid,
			policy:         testRecoveryPolicyInput(),
			attempt:        1, credits: 2, wantAction: RecoveryFallback,
			wantNextAttempt: 2, wantFallback: "cached-source",
			wantCreditsAfter: 1,
			wantRetryAt:      recoveryDecisionTime.Add(5 * time.Second),
		},
		{
			name:           "consumed fallback exhausts to blocked",
			classification: verification.OutputInvalid,
			policy:         testRecoveryPolicyInput(),
			attempt:        2, credits: 0, fallbackConsumed: true,
			wantAction: RecoveryBlocked, wantCreditsAfter: 0,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			policy, err := NewRecoveryPolicy(test.policy)
			if err != nil {
				t.Fatal(err)
			}
			classification := testRecoveryClassification(
				t,
				test.classification,
			)
			input := testRecoveryInput(
				classification,
				test.attempt,
				test.credits,
			)
			input.FallbackConsumed = test.fallbackConsumed
			first, err := DecideRecovery(policy, input)
			if err != nil {
				t.Fatal(err)
			}
			second, err := DecideRecovery(policy, input)
			if err != nil {
				t.Fatal(err)
			}
			if first.Action() != test.wantAction ||
				first.NextAttemptNumber() != test.wantNextAttempt ||
				first.WorkflowFallbackKey() != test.wantFallback ||
				first.CreditsBefore() != test.credits ||
				first.CreditsAfter() != test.wantCreditsAfter ||
				!first.RetryAt().Equal(test.wantRetryAt) ||
				first.Digest() == "" ||
				first.Digest() != second.Digest() ||
				first.PolicyDigest() != policy.Digest() ||
				first.ClassificationDigest() != classification.Digest() {
				t.Fatalf("decision = %#v", first)
			}
			if test.wantAction == RecoveryRetry ||
				test.wantAction == RecoveryFallback {
				if first.NextAgentInstanceID() != "agent-main" ||
					first.NextRuntimeInstanceID() != "runtime-a" {
					t.Fatalf("next binding = %#v", first)
				}
			}
		})
	}
}

func TestRecoveryApprovalRequirementFailsClosedWithoutBorrowedApproval(t *testing.T) {
	t.Parallel()
	input := testRecoveryPolicyInput()
	input.RecoveryApprovalRequired = true
	policy, err := NewRecoveryPolicy(input)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := DecideRecovery(
		policy,
		testRecoveryInput(
			testRecoveryClassification(t, verification.OutputTransientEmpty),
			1,
			2,
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action() != RecoveryHumanRequired ||
		decision.NextAttemptNumber() != 0 ||
		!decision.RetryAt().IsZero() ||
		decision.WorkflowFallbackKey() != "" ||
		decision.CreditsAfter() != 2 {
		t.Fatalf("approval-required decision = %#v", decision)
	}
}

func TestRecoveryPolicyRejectsInvalidBudgetTimeAndMutation(t *testing.T) {
	t.Parallel()
	policyInput := testRecoveryPolicyInput()
	policyInput.AttemptCredits = 3
	if _, err := NewRecoveryPolicy(policyInput); !errors.Is(
		err,
		ErrInvalidRecoveryPolicy,
	) {
		t.Fatalf("credits error = %v", err)
	}
	policy, err := NewRecoveryPolicy(testRecoveryPolicyInput())
	if err != nil {
		t.Fatal(err)
	}
	classification := testRecoveryClassification(
		t,
		verification.OutputTransientEmpty,
	)
	input := testRecoveryInput(classification, 1, 2)
	input.DecisionTime = input.DecisionTime.In(time.FixedZone("offset", 3600))
	if _, err := DecideRecovery(policy, input); !errors.Is(
		err,
		ErrInvalidRecoveryInput,
	) {
		t.Fatalf("time error = %v", err)
	}
	prior := []verification.OutputClassification{verification.OutputInvalid}
	input = testRecoveryInput(classification, 2, 1)
	input.PriorClassifications = prior
	decision, err := DecideRecovery(policy, input)
	if err != nil {
		t.Fatal(err)
	}
	prior[0] = verification.OutputValidNonEmpty
	if decision.PriorClassifications()[0] != verification.OutputInvalid {
		t.Fatal("decision aliases caller prior classifications")
	}
}

func TestVerificationRejectedRecoveryIsBoundedAndForbidsFallback(t *testing.T) {
	policy, err := NewRecoveryPolicy(RecoveryPolicyInput{
		Version:             1,
		RetryDelay:          2 * time.Minute,
		AttemptCredits:      1,
		ExhaustionAction:    ExhaustionBlocked,
		WorkflowFallbackKey: "workflow-fallback",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 26, 18, 30, 0, 0, time.UTC)
	classification := testRecoveryClassification(
		t,
		verification.OutputValidNonEmpty,
	)
	input := RecoveryInput{
		TeamInstanceID:           "team-1",
		PlanDigest:               strings.Repeat("a", 64),
		LogicalNodeID:            "main",
		AttemptNumber:            1,
		MaxAttempts:              2,
		AgentInstanceID:          "agent-1",
		RuntimeInstanceID:        "runtime-1",
		EvidenceID:               classification.EvidenceID(),
		EvidenceDigest:           classification.EvidenceDigest(),
		OutputSummaryDigest:      classification.SummaryDigest(),
		Classification:           classification,
		RemainingCredits:         1,
		DecisionTime:             now,
		Trigger:                  RecoveryTriggerVerificationRejected,
		AcceptanceDecisionDigest: strings.Repeat("d", 64),
	}
	decision, err := DecideRecovery(policy, input)
	if err != nil {
		t.Fatalf("DecideRecovery() error = %v", err)
	}
	if decision.Action() != RecoveryRetry ||
		decision.Trigger() != RecoveryTriggerVerificationRejected ||
		decision.AcceptanceDecisionDigest() != input.AcceptanceDecisionDigest ||
		!decision.RetryAt().Equal(now.Add(2*time.Minute)) ||
		decision.WorkflowFallbackKey() != "" ||
		decision.CreditsAfter() != 0 ||
		!decision.Valid() {
		t.Fatalf("verification rejection decision = %#v", decision)
	}

	input.AttemptNumber = 2
	input.MaxAttempts = 2
	input.PriorClassifications = []verification.OutputClassification{
		verification.OutputInvalid,
	}
	input.RemainingCredits = 0
	exhausted, err := DecideRecovery(policy, input)
	if err != nil {
		t.Fatalf("DecideRecovery(exhausted) error = %v", err)
	}
	if exhausted.Action() != RecoveryBlocked ||
		exhausted.Trigger() != RecoveryTriggerVerificationRejected ||
		!exhausted.RetryAt().IsZero() {
		t.Fatalf("exhausted = %#v", exhausted)
	}

	input.AttemptNumber = 1
	input.MaxAttempts = 2
	input.PriorClassifications = nil
	input.RemainingCredits = 1
	input.AcceptanceDecisionDigest = ""
	if _, err := DecideRecovery(
		policy,
		input,
	); !errors.Is(err, ErrInvalidRecoveryInput) {
		t.Fatalf("missing acceptance digest error = %v", err)
	}
}

func testRecoveryPolicyInput() RecoveryPolicyInput {
	return RecoveryPolicyInput{
		Version:             2,
		RetryDelay:          5 * time.Second,
		AttemptCredits:      2,
		ExhaustionAction:    ExhaustionBlocked,
		RetryInvalid:        false,
		WorkflowFallbackKey: "cached-source",
	}
}

func testRecoveryInput(
	classification verification.Classification,
	attempt int,
	credits int,
) RecoveryInput {
	prior := make([]verification.OutputClassification, attempt-1)
	for index := range prior {
		prior[index] = verification.OutputInvalid
	}
	return RecoveryInput{
		TeamInstanceID:       "team-1",
		PlanDigest:           strings.Repeat("1", 64),
		LogicalNodeID:        "main",
		AttemptNumber:        attempt,
		MaxAttempts:          3,
		AgentInstanceID:      "agent-main",
		RuntimeInstanceID:    "runtime-a",
		EvidenceID:           "team-evidence-1234567890abcdef1234567890abcdef",
		EvidenceDigest:       strings.Repeat("2", 64),
		OutputSummaryDigest:  strings.Repeat("3", 64),
		Classification:       classification,
		PriorClassifications: prior,
		RemainingCredits:     credits,
		DecisionTime:         recoveryDecisionTime,
	}
}

func testRecoveryClassification(
	t testing.TB,
	want verification.OutputClassification,
) verification.Classification {
	t.Helper()
	empty := verification.EmptyOutputInvalid
	outputFrames := 0
	outputBytes := 0
	result := true
	terminal := "succeeded"
	switch want {
	case verification.OutputValidNonEmpty:
		outputFrames, outputBytes = 1, 24
	case verification.OutputValidEmpty:
		empty = verification.EmptyOutputValid
	case verification.OutputTransientEmpty:
		empty = verification.EmptyOutputTransient
	case verification.OutputInvalid:
		terminal = "failed"
	}
	contract, err := verification.NewOutputContract(1, empty)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := verification.NewOutputObservation(
		"team-evidence-1234567890abcdef1234567890abcdef",
		strings.Repeat("2", 64),
		strings.Repeat("3", 64),
		outputFrames+2,
		outputFrames,
		outputBytes,
		result,
		terminal,
	)
	if err != nil {
		t.Fatal(err)
	}
	classification, err := verification.Classify(contract, observation)
	if err != nil || classification.Kind() != want {
		t.Fatalf("classification = %#v, %v", classification, err)
	}
	return classification
}
