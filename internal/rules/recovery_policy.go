package rules

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/verification"
)

var (
	ErrInvalidRecoveryPolicy = errors.New("invalid recovery policy")
	ErrInvalidRecoveryInput  = errors.New("invalid recovery input")
)

type RecoveryAction string

const (
	RecoveryNone          RecoveryAction = "none"
	RecoveryRetry         RecoveryAction = "retry"
	RecoveryFallback      RecoveryAction = "fallback"
	RecoveryDegraded      RecoveryAction = "degraded"
	RecoveryBlocked       RecoveryAction = "blocked"
	RecoveryHumanRequired RecoveryAction = "human_required"
)

type ExhaustionAction string

const (
	ExhaustionDegraded      ExhaustionAction = "degraded"
	ExhaustionBlocked       ExhaustionAction = "blocked"
	ExhaustionHumanRequired ExhaustionAction = "human_required"
)

type RecoveryPolicyInput struct {
	Version                  int
	RetryDelay               time.Duration
	AttemptCredits           int
	ExhaustionAction         ExhaustionAction
	RetryInvalid             bool
	WorkflowFallbackKey      string
	RecoveryApprovalRequired bool
}

type RecoveryPolicy struct {
	version                  int
	retryDelay               time.Duration
	attemptCredits           int
	exhaustionAction         ExhaustionAction
	retryInvalid             bool
	workflowFallbackKey      string
	recoveryApprovalRequired bool
	digest                   string
}

type RecoveryInput struct {
	TeamInstanceID       string
	PlanDigest           string
	LogicalNodeID        string
	AttemptNumber        int
	MaxAttempts          int
	AgentInstanceID      string
	RuntimeInstanceID    string
	EvidenceID           string
	EvidenceDigest       string
	OutputSummaryDigest  string
	Classification       verification.Classification
	PriorClassifications []verification.OutputClassification
	RemainingCredits     int
	FallbackConsumed     bool
	DecisionTime         time.Time
}

type RecoveryDecision struct {
	teamInstanceID           string
	planDigest               string
	logicalNodeID            string
	attemptNumber            int
	maxAttempts              int
	agentInstanceID          string
	runtimeInstanceID        string
	evidenceID               string
	evidenceDigest           string
	outputSummaryDigest      string
	classification           verification.OutputClassification
	classificationDigest     string
	priorClassifications     []verification.OutputClassification
	remainingCredits         int
	fallbackConsumed         bool
	decisionTime             time.Time
	action                   RecoveryAction
	nextAttemptNumber        int
	nextAgentInstanceID      string
	nextRuntimeInstanceID    string
	workflowFallbackKey      string
	creditsBefore            int
	creditsAfter             int
	retryAt                  time.Time
	policyVersion            int
	policyDigest             string
	recoveryApprovalRequired bool
	digest                   string
}

func NewRecoveryPolicy(input RecoveryPolicyInput) (RecoveryPolicy, error) {
	if input.Version < 1 || input.Version > 1_000_000 ||
		input.RetryDelay < 0 || input.RetryDelay > 24*time.Hour ||
		input.AttemptCredits < 0 || input.AttemptCredits > 2 ||
		!validExhaustionAction(input.ExhaustionAction) ||
		input.WorkflowFallbackKey != "" &&
			!validRecoveryID(input.WorkflowFallbackKey) {
		return RecoveryPolicy{}, ErrInvalidRecoveryPolicy
	}
	policy := RecoveryPolicy{
		version:                  input.Version,
		retryDelay:               input.RetryDelay,
		attemptCredits:           input.AttemptCredits,
		exhaustionAction:         input.ExhaustionAction,
		retryInvalid:             input.RetryInvalid,
		workflowFallbackKey:      input.WorkflowFallbackKey,
		recoveryApprovalRequired: input.RecoveryApprovalRequired,
	}
	policy.digest = canonicalRecoveryDigest(
		"loom.recovery-policy.v1",
		strconv.Itoa(policy.version),
		strconv.FormatInt(int64(policy.retryDelay), 10),
		strconv.Itoa(policy.attemptCredits),
		string(policy.exhaustionAction),
		strconv.FormatBool(policy.retryInvalid),
		policy.workflowFallbackKey,
		strconv.FormatBool(policy.recoveryApprovalRequired),
	)
	return policy, nil
}

func DecideRecovery(
	policy RecoveryPolicy,
	input RecoveryInput,
) (RecoveryDecision, error) {
	if !policy.Valid() {
		return RecoveryDecision{}, ErrInvalidRecoveryPolicy
	}
	if !validRecoveryInput(policy, input) {
		return RecoveryDecision{}, ErrInvalidRecoveryInput
	}
	action := RecoveryNone
	switch input.Classification.Kind() {
	case verification.OutputValidNonEmpty, verification.OutputValidEmpty:
		action = RecoveryNone
	case verification.OutputTransientEmpty:
		action = recoveryAttemptAction(policy, input, false)
	case verification.OutputInvalid:
		action = recoveryAttemptAction(policy, input, true)
	default:
		return RecoveryDecision{}, ErrInvalidRecoveryInput
	}
	decision := RecoveryDecision{
		teamInstanceID:       input.TeamInstanceID,
		planDigest:           input.PlanDigest,
		logicalNodeID:        input.LogicalNodeID,
		attemptNumber:        input.AttemptNumber,
		maxAttempts:          input.MaxAttempts,
		agentInstanceID:      input.AgentInstanceID,
		runtimeInstanceID:    input.RuntimeInstanceID,
		evidenceID:           input.EvidenceID,
		evidenceDigest:       input.EvidenceDigest,
		outputSummaryDigest:  input.OutputSummaryDigest,
		classification:       input.Classification.Kind(),
		classificationDigest: input.Classification.Digest(),
		priorClassifications: append(
			[]verification.OutputClassification(nil),
			input.PriorClassifications...,
		),
		remainingCredits:         input.RemainingCredits,
		fallbackConsumed:         input.FallbackConsumed,
		decisionTime:             input.DecisionTime,
		action:                   action,
		creditsBefore:            input.RemainingCredits,
		creditsAfter:             input.RemainingCredits,
		policyVersion:            policy.version,
		policyDigest:             policy.digest,
		recoveryApprovalRequired: policy.recoveryApprovalRequired,
	}
	if action == RecoveryRetry || action == RecoveryFallback {
		decision.nextAttemptNumber = input.AttemptNumber + 1
		decision.nextAgentInstanceID = input.AgentInstanceID
		decision.nextRuntimeInstanceID = input.RuntimeInstanceID
		decision.creditsAfter--
		decision.retryAt = input.DecisionTime.Add(policy.retryDelay)
		if action == RecoveryFallback {
			decision.workflowFallbackKey = policy.workflowFallbackKey
		}
	}
	decision.digest = recoveryDecisionDigest(decision)
	return decision, nil
}

func recoveryDecisionDigest(decision RecoveryDecision) string {
	prior := make([]string, len(decision.priorClassifications))
	for index, classification := range decision.priorClassifications {
		prior[index] = string(classification)
	}
	fields := []string{
		decision.teamInstanceID,
		decision.planDigest,
		decision.logicalNodeID,
		strconv.Itoa(decision.attemptNumber),
		strconv.Itoa(decision.maxAttempts),
		decision.agentInstanceID,
		decision.runtimeInstanceID,
		decision.evidenceID,
		decision.evidenceDigest,
		decision.outputSummaryDigest,
		string(decision.classification),
		decision.classificationDigest,
		strconv.Itoa(len(prior)),
	}
	fields = append(fields, prior...)
	fields = append(fields,
		strconv.Itoa(decision.remainingCredits),
		strconv.FormatBool(decision.fallbackConsumed),
		decision.decisionTime.Format(time.RFC3339Nano),
		string(decision.action),
		strconv.Itoa(decision.nextAttemptNumber),
		decision.nextAgentInstanceID,
		decision.nextRuntimeInstanceID,
		decision.workflowFallbackKey,
		strconv.Itoa(decision.creditsBefore),
		strconv.Itoa(decision.creditsAfter),
		recoveryTimeString(decision.retryAt),
		strconv.Itoa(decision.policyVersion),
		decision.policyDigest,
		strconv.FormatBool(decision.recoveryApprovalRequired),
	)
	return canonicalRecoveryDigest(
		"loom.recovery-decision.v1",
		fields...,
	)
}

func recoveryAttemptAction(
	policy RecoveryPolicy,
	input RecoveryInput,
	invalid bool,
) RecoveryAction {
	if policy.recoveryApprovalRequired {
		return RecoveryHumanRequired
	}
	if input.AttemptNumber >= input.MaxAttempts ||
		input.RemainingCredits == 0 {
		return recoveryExhaustionAction(policy.exhaustionAction)
	}
	if invalid {
		if policy.workflowFallbackKey != "" && !input.FallbackConsumed {
			return RecoveryFallback
		}
		if !policy.retryInvalid {
			return recoveryExhaustionAction(policy.exhaustionAction)
		}
	}
	return RecoveryRetry
}

func recoveryExhaustionAction(action ExhaustionAction) RecoveryAction {
	switch action {
	case ExhaustionDegraded:
		return RecoveryDegraded
	case ExhaustionBlocked:
		return RecoveryBlocked
	case ExhaustionHumanRequired:
		return RecoveryHumanRequired
	default:
		return ""
	}
}

func (policy RecoveryPolicy) Version() int              { return policy.version }
func (policy RecoveryPolicy) RetryDelay() time.Duration { return policy.retryDelay }
func (policy RecoveryPolicy) AttemptCredits() int       { return policy.attemptCredits }
func (policy RecoveryPolicy) ExhaustionAction() ExhaustionAction {
	return policy.exhaustionAction
}
func (policy RecoveryPolicy) RetryInvalid() bool { return policy.retryInvalid }
func (policy RecoveryPolicy) WorkflowFallbackKey() string {
	return policy.workflowFallbackKey
}
func (policy RecoveryPolicy) RecoveryApprovalRequired() bool {
	return policy.recoveryApprovalRequired
}
func (policy RecoveryPolicy) Digest() string { return policy.digest }
func (policy RecoveryPolicy) Valid() bool {
	reconstructed, err := NewRecoveryPolicy(RecoveryPolicyInput{
		Version:                  policy.version,
		RetryDelay:               policy.retryDelay,
		AttemptCredits:           policy.attemptCredits,
		ExhaustionAction:         policy.exhaustionAction,
		RetryInvalid:             policy.retryInvalid,
		WorkflowFallbackKey:      policy.workflowFallbackKey,
		RecoveryApprovalRequired: policy.recoveryApprovalRequired,
	})
	return err == nil && reconstructed.digest == policy.digest
}

func (decision RecoveryDecision) TeamInstanceID() string { return decision.teamInstanceID }
func (decision RecoveryDecision) PlanDigest() string     { return decision.planDigest }
func (decision RecoveryDecision) LogicalNodeID() string  { return decision.logicalNodeID }
func (decision RecoveryDecision) AttemptNumber() int     { return decision.attemptNumber }
func (decision RecoveryDecision) MaxAttempts() int       { return decision.maxAttempts }
func (decision RecoveryDecision) AgentInstanceID() string {
	return decision.agentInstanceID
}
func (decision RecoveryDecision) RuntimeInstanceID() string {
	return decision.runtimeInstanceID
}
func (decision RecoveryDecision) EvidenceID() string     { return decision.evidenceID }
func (decision RecoveryDecision) EvidenceDigest() string { return decision.evidenceDigest }
func (decision RecoveryDecision) OutputSummaryDigest() string {
	return decision.outputSummaryDigest
}
func (decision RecoveryDecision) Classification() verification.OutputClassification {
	return decision.classification
}
func (decision RecoveryDecision) ClassificationValue() string {
	return string(decision.classification)
}
func (decision RecoveryDecision) ClassificationDigest() string {
	return decision.classificationDigest
}
func (decision RecoveryDecision) PriorClassifications() []verification.OutputClassification {
	return append(
		[]verification.OutputClassification(nil),
		decision.priorClassifications...,
	)
}
func (decision RecoveryDecision) PriorClassificationValues() []string {
	result := make([]string, len(decision.priorClassifications))
	for index, classification := range decision.priorClassifications {
		result[index] = string(classification)
	}
	return result
}
func (decision RecoveryDecision) RemainingCredits() int {
	return decision.remainingCredits
}
func (decision RecoveryDecision) FallbackConsumed() bool {
	return decision.fallbackConsumed
}
func (decision RecoveryDecision) DecisionTime() time.Time { return decision.decisionTime }
func (decision RecoveryDecision) Action() RecoveryAction  { return decision.action }
func (decision RecoveryDecision) ActionValue() string     { return string(decision.action) }
func (decision RecoveryDecision) NextAttemptNumber() int {
	return decision.nextAttemptNumber
}
func (decision RecoveryDecision) NextAgentInstanceID() string {
	return decision.nextAgentInstanceID
}
func (decision RecoveryDecision) NextRuntimeInstanceID() string {
	return decision.nextRuntimeInstanceID
}
func (decision RecoveryDecision) WorkflowFallbackKey() string {
	return decision.workflowFallbackKey
}
func (decision RecoveryDecision) CreditsBefore() int   { return decision.creditsBefore }
func (decision RecoveryDecision) CreditsAfter() int    { return decision.creditsAfter }
func (decision RecoveryDecision) RetryAt() time.Time   { return decision.retryAt }
func (decision RecoveryDecision) PolicyVersion() int   { return decision.policyVersion }
func (decision RecoveryDecision) PolicyDigest() string { return decision.policyDigest }
func (decision RecoveryDecision) RecoveryApprovalRequired() bool {
	return decision.recoveryApprovalRequired
}
func (decision RecoveryDecision) Digest() string { return decision.digest }
func (decision RecoveryDecision) Valid() bool {
	if decision.digest == "" ||
		!validRecoveryAction(decision.action) ||
		!validRecoveryID(decision.teamInstanceID) ||
		!validRecoveryDigest(decision.planDigest) ||
		!validRecoveryID(decision.logicalNodeID) ||
		decision.attemptNumber < 1 ||
		decision.maxAttempts < decision.attemptNumber ||
		decision.maxAttempts > 3 ||
		!validRecoveryID(decision.agentInstanceID) ||
		!validRecoveryID(decision.runtimeInstanceID) ||
		!validRecoveryID(decision.evidenceID) ||
		!validRecoveryDigest(decision.evidenceDigest) ||
		!validRecoveryDigest(decision.outputSummaryDigest) ||
		!validRecoveryClassification(decision.classification) ||
		!validRecoveryDigest(decision.policyDigest) ||
		decision.policyVersion < 1 ||
		!validRecoveryDigest(decision.classificationDigest) ||
		len(decision.priorClassifications) > 2 ||
		len(decision.priorClassifications) != decision.attemptNumber-1 ||
		decision.remainingCredits < 0 ||
		decision.remainingCredits > 2 ||
		decision.creditsBefore != decision.remainingCredits ||
		decision.creditsAfter < 0 ||
		decision.creditsAfter > decision.creditsBefore ||
		decision.decisionTime.IsZero() ||
		decision.decisionTime.Location() != time.UTC {
		return false
	}
	for _, classification := range decision.priorClassifications {
		if !validRecoveryClassification(classification) {
			return false
		}
	}
	switch decision.action {
	case RecoveryRetry:
		if decision.nextAttemptNumber != decision.attemptNumber+1 ||
			decision.nextAgentInstanceID != decision.agentInstanceID ||
			decision.nextRuntimeInstanceID != decision.runtimeInstanceID ||
			decision.workflowFallbackKey != "" ||
			decision.creditsAfter != decision.creditsBefore-1 ||
			decision.retryAt.IsZero() ||
			decision.retryAt.Location() != time.UTC {
			return false
		}
	case RecoveryFallback:
		if decision.nextAttemptNumber != decision.attemptNumber+1 ||
			decision.nextAgentInstanceID != decision.agentInstanceID ||
			decision.nextRuntimeInstanceID != decision.runtimeInstanceID ||
			!validRecoveryID(decision.workflowFallbackKey) ||
			decision.creditsAfter != decision.creditsBefore-1 ||
			decision.retryAt.IsZero() ||
			decision.retryAt.Location() != time.UTC {
			return false
		}
	case RecoveryNone, RecoveryDegraded, RecoveryBlocked,
		RecoveryHumanRequired:
		if decision.nextAttemptNumber != 0 ||
			decision.nextAgentInstanceID != "" ||
			decision.nextRuntimeInstanceID != "" ||
			decision.workflowFallbackKey != "" ||
			decision.creditsAfter != decision.creditsBefore ||
			!decision.retryAt.IsZero() {
			return false
		}
	default:
		return false
	}
	return decision.digest == recoveryDecisionDigest(decision)
}

func validRecoveryClassification(
	value verification.OutputClassification,
) bool {
	switch value {
	case verification.OutputValidNonEmpty,
		verification.OutputValidEmpty,
		verification.OutputTransientEmpty,
		verification.OutputInvalid:
		return true
	default:
		return false
	}
}

func validRecoveryInput(policy RecoveryPolicy, input RecoveryInput) bool {
	if !validRecoveryID(input.TeamInstanceID) ||
		!validRecoveryDigest(input.PlanDigest) ||
		!validRecoveryID(input.LogicalNodeID) ||
		input.AttemptNumber < 1 || input.AttemptNumber > 3 ||
		input.MaxAttempts < input.AttemptNumber || input.MaxAttempts > 3 ||
		!validRecoveryID(input.AgentInstanceID) ||
		!validRecoveryID(input.RuntimeInstanceID) ||
		!validRecoveryID(input.EvidenceID) ||
		!validRecoveryDigest(input.EvidenceDigest) ||
		!validRecoveryDigest(input.OutputSummaryDigest) ||
		!input.Classification.Valid() ||
		input.Classification.EvidenceID() != input.EvidenceID ||
		input.Classification.EvidenceDigest() != input.EvidenceDigest ||
		input.Classification.SummaryDigest() != input.OutputSummaryDigest ||
		len(input.PriorClassifications) > 2 ||
		len(input.PriorClassifications) != input.AttemptNumber-1 ||
		input.RemainingCredits < 0 ||
		input.RemainingCredits > policy.attemptCredits ||
		input.DecisionTime.IsZero() ||
		input.DecisionTime.Location() != time.UTC {
		return false
	}
	for _, classification := range input.PriorClassifications {
		switch classification {
		case verification.OutputValidNonEmpty,
			verification.OutputValidEmpty,
			verification.OutputTransientEmpty,
			verification.OutputInvalid:
		default:
			return false
		}
	}
	return true
}

func validExhaustionAction(value ExhaustionAction) bool {
	switch value {
	case ExhaustionDegraded, ExhaustionBlocked, ExhaustionHumanRequired:
		return true
	default:
		return false
	}
}

func validRecoveryAction(value RecoveryAction) bool {
	switch value {
	case RecoveryNone, RecoveryRetry, RecoveryFallback,
		RecoveryDegraded, RecoveryBlocked, RecoveryHumanRequired:
		return true
	default:
		return false
	}
}

func validRecoveryID(value string) bool {
	if value == "" || len(value) > 128 ||
		!utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current == 0x7f {
			return false
		}
	}
	return true
}

func validRecoveryDigest(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func recoveryTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}

func canonicalRecoveryDigest(schema string, fields ...string) string {
	hash := sha256.New()
	writeRecoveryDigestField(hash, schema)
	for _, field := range fields {
		writeRecoveryDigestField(hash, field)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type recoveryDigestWriter interface {
	Write([]byte) (int, error)
}

func writeRecoveryDigestField(writer recoveryDigestWriter, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(value))
}
