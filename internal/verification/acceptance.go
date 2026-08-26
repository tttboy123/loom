package verification

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxAcceptanceCriteria       = 16
	maxAcceptanceCriterionBytes = 256
	maxAcceptanceCriteriaBytes  = 2048
)

var (
	ErrInvalidAcceptanceContract        = errors.New("invalid acceptance contract")
	ErrAcceptanceContractDigestMismatch = errors.New("acceptance contract digest mismatch")
	ErrInvalidDeterministicVerification = errors.New("invalid deterministic verification")
	ErrInvalidVerifierCandidate         = errors.New("invalid verifier candidate")
	ErrInvalidAcceptanceDecision        = errors.New("invalid acceptance decision")
)

type AcceptanceRisk string

const (
	AcceptanceRiskLow    AcceptanceRisk = "low"
	AcceptanceRiskMedium AcceptanceRisk = "medium"
	AcceptanceRiskHigh   AcceptanceRisk = "high"
)

type DeterministicVerificationKind string

const (
	DeterministicAccepted                 DeterministicVerificationKind = "accepted"
	DeterministicNeedsIndependentVerifier DeterministicVerificationKind = "needs_independent_verifier"
	DeterministicRejected                 DeterministicVerificationKind = "rejected"
)

type VerifierCandidateKind string

const (
	VerifierAccepted VerifierCandidateKind = "accepted"
	VerifierRejected VerifierCandidateKind = "rejected"
)

type VerifierReasonCode string

const (
	VerifierReasonCriteriaSatisfied    VerifierReasonCode = "criteria_satisfied"
	VerifierReasonCriteriaNotSatisfied VerifierReasonCode = "criteria_not_satisfied"
	VerifierReasonInsufficientEvidence VerifierReasonCode = "insufficient_evidence"
)

type AcceptanceDecisionKind string

const (
	AcceptanceAccepted AcceptanceDecisionKind = "accepted"
	AcceptanceRejected AcceptanceDecisionKind = "rejected"
)

type AcceptanceContract struct {
	version  int
	criteria []string
	risk     AcceptanceRisk
	digest   string
}

type DeterministicVerificationInput struct {
	TeamInstanceID             string
	PlanDigest                 string
	LogicalNodeID              string
	AttemptNumber              int
	WorkItemID                 string
	RunID                      string
	ClaimID                    string
	ClaimGeneration            int64
	SourceEvidenceID           string
	SourceEvidenceDigest       string
	OutputSummaryDigest        string
	OutputContractVersion      int
	OutputContractDigest       string
	OutputClassification       OutputClassification
	OutputClassificationDigest string
	AcceptanceContractDigest   string
	TerminalStatus             string
}

type DeterministicVerificationResult struct {
	kind   DeterministicVerificationKind
	input  DeterministicVerificationInput
	risk   AcceptanceRisk
	digest string
}

type VerifierTerminalInput struct {
	WorkItemID          string
	RunID               string
	ClaimID             string
	ClaimGeneration     int64
	RuntimeInstanceID   string
	AgentInstanceID     string
	GrantID             string
	EvidenceID          string
	EvidenceDigest      string
	OutputSummaryDigest string
	TerminalStatus      string
	TerminalReason      string
	OutputReasonCode    VerifierReasonCode
}

type VerifierCandidate struct {
	kind       VerifierCandidateKind
	reasonCode VerifierReasonCode
	input      VerifierTerminalInput
	digest     string
}

type AcceptanceDecisionInput struct {
	Contract            AcceptanceContract
	DeterministicResult DeterministicVerificationResult
	VerifierCandidate   VerifierCandidate
	DecisionTime        time.Time
}

type AcceptanceDecision struct {
	kind                AcceptanceDecisionKind
	contractVersion     int
	contractDigest      string
	risk                AcceptanceRisk
	deterministicDigest string
	verifierDigest      string
	reasonCode          VerifierReasonCode
	decisionTime        time.Time
	digest              string
}

func NewAcceptanceContract(
	version int,
	criteria []string,
	risk AcceptanceRisk,
) (AcceptanceContract, error) {
	if version < 1 || version > 1_000_000 || !validAcceptanceRisk(risk) {
		return AcceptanceContract{}, ErrInvalidAcceptanceContract
	}
	normalized, ok := normalizeAcceptanceCriteria(criteria)
	if !ok {
		return AcceptanceContract{}, ErrInvalidAcceptanceContract
	}
	fields := make([]string, 0, 4+len(normalized))
	fields = append(
		fields,
		"loom.acceptance-contract.v1",
		strconv.Itoa(version),
		string(risk),
		strconv.Itoa(len(normalized)),
	)
	fields = append(fields, normalized...)
	return AcceptanceContract{
		version:  version,
		criteria: normalized,
		risk:     risk,
		digest:   canonicalOutputDigest(fields[0], fields[1:]...),
	}, nil
}

func VerifyDeterministic(
	contract AcceptanceContract,
	input DeterministicVerificationInput,
) (DeterministicVerificationResult, error) {
	if !contract.Valid() {
		return DeterministicVerificationResult{}, ErrAcceptanceContractDigestMismatch
	}
	if !validDeterministicVerificationInput(contract, input) {
		return DeterministicVerificationResult{}, ErrInvalidDeterministicVerification
	}
	kind := DeterministicAccepted
	if contract.IndependentVerifierRequired() {
		kind = DeterministicNeedsIndependentVerifier
	}
	result := DeterministicVerificationResult{
		kind:  kind,
		input: input,
		risk:  contract.risk,
	}
	result.digest = deterministicVerificationDigest(result)
	return result, nil
}

func VerifierCandidateFromTerminal(
	input VerifierTerminalInput,
) (VerifierCandidate, error) {
	if !validVerifierTerminalBinding(input) {
		return VerifierCandidate{}, ErrInvalidVerifierCandidate
	}
	var kind VerifierCandidateKind
	var reason VerifierReasonCode
	switch {
	case input.TerminalStatus == "succeeded" &&
		input.TerminalReason == "" &&
		validVerifierReasonCode(input.OutputReasonCode):
		reason = input.OutputReasonCode
		if reason == VerifierReasonCriteriaSatisfied {
			kind = VerifierAccepted
		} else {
			kind = VerifierRejected
		}
	case input.TerminalStatus == "failed" &&
		(input.TerminalReason == string(VerifierReasonCriteriaNotSatisfied) ||
			input.TerminalReason == string(VerifierReasonInsufficientEvidence)):
		kind = VerifierRejected
		reason = VerifierReasonCode(input.TerminalReason)
	case (input.TerminalStatus == "failed" ||
		input.TerminalStatus == "cancelled") &&
		input.TerminalReason != "" && input.OutputReasonCode == "":
		kind = VerifierRejected
		reason = VerifierReasonInsufficientEvidence
	default:
		return VerifierCandidate{}, ErrInvalidVerifierCandidate
	}
	candidate := VerifierCandidate{
		kind:       kind,
		reasonCode: reason,
		input:      input,
	}
	candidate.digest = verifierCandidateDigest(candidate)
	return candidate, nil
}

func DecideAcceptance(
	input AcceptanceDecisionInput,
) (AcceptanceDecision, error) {
	if !input.Contract.Valid() ||
		!input.DeterministicResult.Valid() ||
		input.DeterministicResult.input.AcceptanceContractDigest !=
			input.Contract.digest ||
		input.DeterministicResult.risk != input.Contract.risk ||
		input.DecisionTime.IsZero() ||
		input.DecisionTime.Location() != time.UTC {
		return AcceptanceDecision{}, ErrInvalidAcceptanceDecision
	}
	var kind AcceptanceDecisionKind
	var verifierDigest string
	var reason VerifierReasonCode
	switch input.DeterministicResult.kind {
	case DeterministicAccepted:
		if input.Contract.IndependentVerifierRequired() ||
			!zeroVerifierCandidate(input.VerifierCandidate) {
			return AcceptanceDecision{}, ErrInvalidAcceptanceDecision
		}
		kind = AcceptanceAccepted
	case DeterministicNeedsIndependentVerifier:
		if !input.Contract.IndependentVerifierRequired() ||
			!input.VerifierCandidate.Valid() {
			return AcceptanceDecision{}, ErrInvalidAcceptanceDecision
		}
		verifierDigest = input.VerifierCandidate.digest
		reason = input.VerifierCandidate.reasonCode
		if input.VerifierCandidate.kind == VerifierAccepted {
			kind = AcceptanceAccepted
		} else {
			kind = AcceptanceRejected
		}
	default:
		return AcceptanceDecision{}, ErrInvalidAcceptanceDecision
	}
	decision := AcceptanceDecision{
		kind:                kind,
		contractVersion:     input.Contract.version,
		contractDigest:      input.Contract.digest,
		risk:                input.Contract.risk,
		deterministicDigest: input.DeterministicResult.digest,
		verifierDigest:      verifierDigest,
		reasonCode:          reason,
		decisionTime:        input.DecisionTime,
	}
	decision.digest = acceptanceDecisionDigest(decision)
	return decision, nil
}

func (contract AcceptanceContract) Version() int { return contract.version }
func (contract AcceptanceContract) Criteria() []string {
	return append([]string(nil), contract.criteria...)
}
func (contract AcceptanceContract) Risk() AcceptanceRisk { return contract.risk }
func (contract AcceptanceContract) IndependentVerifierRequired() bool {
	return contract.risk == AcceptanceRiskMedium ||
		contract.risk == AcceptanceRiskHigh
}
func (contract AcceptanceContract) Digest() string { return contract.digest }
func (contract AcceptanceContract) Valid() bool {
	rebuilt, err := NewAcceptanceContract(
		contract.version,
		contract.criteria,
		contract.risk,
	)
	return err == nil && rebuilt.digest == contract.digest
}

func (result DeterministicVerificationResult) Kind() DeterministicVerificationKind {
	return result.kind
}
func (result DeterministicVerificationResult) Input() DeterministicVerificationInput {
	return result.input
}
func (result DeterministicVerificationResult) Risk() AcceptanceRisk {
	return result.risk
}
func (result DeterministicVerificationResult) Digest() string { return result.digest }
func (result DeterministicVerificationResult) Valid() bool {
	return validDeterministicVerificationKind(result.kind) &&
		validAcceptanceRisk(result.risk) &&
		validDeterministicVerificationInputShape(result.input) &&
		result.digest == deterministicVerificationDigest(result)
}

func (candidate VerifierCandidate) Kind() VerifierCandidateKind {
	return candidate.kind
}
func (candidate VerifierCandidate) ReasonCode() VerifierReasonCode {
	return candidate.reasonCode
}
func (candidate VerifierCandidate) Binding() VerifierTerminalInput {
	return candidate.input
}
func (candidate VerifierCandidate) Digest() string { return candidate.digest }
func (candidate VerifierCandidate) Valid() bool {
	if !validVerifierTerminalBinding(candidate.input) {
		return false
	}
	switch candidate.kind {
	case VerifierAccepted:
		if candidate.reasonCode != VerifierReasonCriteriaSatisfied ||
			candidate.input.TerminalStatus != "succeeded" ||
			candidate.input.TerminalReason != "" ||
			candidate.input.OutputReasonCode != candidate.reasonCode {
			return false
		}
	case VerifierRejected:
		if candidate.reasonCode != VerifierReasonCriteriaNotSatisfied &&
			candidate.reasonCode != VerifierReasonInsufficientEvidence {
			return false
		}
		switch candidate.input.TerminalStatus {
		case "succeeded":
			if candidate.input.TerminalReason != "" ||
				candidate.input.OutputReasonCode != candidate.reasonCode {
				return false
			}
		case "failed":
			if candidate.input.TerminalReason == "" ||
				candidate.input.OutputReasonCode != "" ||
				(candidate.input.TerminalReason ==
					string(VerifierReasonCriteriaNotSatisfied) &&
					candidate.reasonCode !=
						VerifierReasonCriteriaNotSatisfied) ||
				(candidate.input.TerminalReason !=
					string(VerifierReasonCriteriaNotSatisfied) &&
					candidate.reasonCode !=
						VerifierReasonInsufficientEvidence) {
				return false
			}
		case "cancelled":
			if candidate.input.TerminalReason == "" ||
				candidate.input.OutputReasonCode != "" ||
				candidate.reasonCode !=
					VerifierReasonInsufficientEvidence {
				return false
			}
		default:
			return false
		}
	default:
		return false
	}
	return candidate.digest == verifierCandidateDigest(candidate)
}

func (decision AcceptanceDecision) Kind() AcceptanceDecisionKind {
	return decision.kind
}
func (decision AcceptanceDecision) ContractVersion() int {
	return decision.contractVersion
}
func (decision AcceptanceDecision) ContractDigest() string {
	return decision.contractDigest
}
func (decision AcceptanceDecision) Risk() AcceptanceRisk { return decision.risk }
func (decision AcceptanceDecision) DeterministicResultDigest() string {
	return decision.deterministicDigest
}
func (decision AcceptanceDecision) VerifierCandidateDigest() string {
	return decision.verifierDigest
}
func (decision AcceptanceDecision) ReasonCode() VerifierReasonCode {
	return decision.reasonCode
}
func (decision AcceptanceDecision) DecisionTime() time.Time {
	return decision.decisionTime
}
func (decision AcceptanceDecision) Digest() string { return decision.digest }
func (decision AcceptanceDecision) Valid() bool {
	if (decision.kind != AcceptanceAccepted &&
		decision.kind != AcceptanceRejected) ||
		decision.contractVersion < 1 ||
		!validDigest(decision.contractDigest) ||
		!validAcceptanceRisk(decision.risk) ||
		!validDigest(decision.deterministicDigest) ||
		decision.decisionTime.IsZero() ||
		decision.decisionTime.Location() != time.UTC {
		return false
	}
	if decision.risk == AcceptanceRiskLow {
		if decision.verifierDigest != "" || decision.reasonCode != "" ||
			decision.kind != AcceptanceAccepted {
			return false
		}
	} else if !validDigest(decision.verifierDigest) ||
		!validVerifierReasonCode(decision.reasonCode) {
		return false
	}
	return decision.digest == acceptanceDecisionDigest(decision)
}

func validAcceptanceRisk(risk AcceptanceRisk) bool {
	return risk == AcceptanceRiskLow ||
		risk == AcceptanceRiskMedium ||
		risk == AcceptanceRiskHigh
}

func normalizeAcceptanceCriteria(criteria []string) ([]string, bool) {
	if len(criteria) == 0 || len(criteria) > maxAcceptanceCriteria {
		return nil, false
	}
	normalized := make([]string, len(criteria))
	total := 0
	for index, raw := range criteria {
		value := strings.TrimSpace(raw)
		if value == "" || len(value) > maxAcceptanceCriterionBytes ||
			!utf8.ValidString(value) {
			return nil, false
		}
		for _, current := range value {
			if current < 0x20 || current == 0x7f {
				return nil, false
			}
		}
		total += len(value)
		if total > maxAcceptanceCriteriaBytes {
			return nil, false
		}
		normalized[index] = value
	}
	sort.Strings(normalized)
	for index := 1; index < len(normalized); index++ {
		if normalized[index] == normalized[index-1] {
			return nil, false
		}
	}
	return normalized, true
}

func validDeterministicVerificationInput(
	contract AcceptanceContract,
	input DeterministicVerificationInput,
) bool {
	return validDeterministicVerificationInputShape(input) &&
		input.AcceptanceContractDigest == contract.digest
}

func validDeterministicVerificationInputShape(
	input DeterministicVerificationInput,
) bool {
	return validOutputID(input.TeamInstanceID) &&
		validDigest(input.PlanDigest) &&
		validOutputID(input.LogicalNodeID) &&
		input.AttemptNumber > 0 && input.AttemptNumber <= 3 &&
		validOutputID(input.WorkItemID) &&
		validOutputID(input.RunID) &&
		validOutputID(input.ClaimID) &&
		input.ClaimGeneration > 0 &&
		validOutputID(input.SourceEvidenceID) &&
		validDigest(input.SourceEvidenceDigest) &&
		validDigest(input.OutputSummaryDigest) &&
		input.OutputContractVersion > 0 &&
		validDigest(input.OutputContractDigest) &&
		(input.OutputClassification == OutputValidNonEmpty ||
			input.OutputClassification == OutputValidEmpty) &&
		validDigest(input.OutputClassificationDigest) &&
		validDigest(input.AcceptanceContractDigest) &&
		input.TerminalStatus == "succeeded"
}

func validVerifierTerminalBinding(input VerifierTerminalInput) bool {
	return validOutputID(input.WorkItemID) &&
		validOutputID(input.RunID) &&
		validOutputID(input.ClaimID) &&
		input.ClaimGeneration > 0 &&
		validOutputID(input.RuntimeInstanceID) &&
		validOutputID(input.AgentInstanceID) &&
		validOutputID(input.GrantID) &&
		validOutputID(input.EvidenceID) &&
		validDigest(input.EvidenceDigest) &&
		validDigest(input.OutputSummaryDigest)
}

func validDeterministicVerificationKind(
	kind DeterministicVerificationKind,
) bool {
	return kind == DeterministicAccepted ||
		kind == DeterministicNeedsIndependentVerifier ||
		kind == DeterministicRejected
}

func validVerifierReasonCode(code VerifierReasonCode) bool {
	return code == VerifierReasonCriteriaSatisfied ||
		code == VerifierReasonCriteriaNotSatisfied ||
		code == VerifierReasonInsufficientEvidence
}

func deterministicVerificationDigest(
	result DeterministicVerificationResult,
) string {
	input := result.input
	return canonicalOutputDigest(
		"loom.deterministic-verification.v1",
		string(result.kind),
		string(result.risk),
		input.TeamInstanceID,
		input.PlanDigest,
		input.LogicalNodeID,
		strconv.Itoa(input.AttemptNumber),
		input.WorkItemID,
		input.RunID,
		input.ClaimID,
		strconv.FormatInt(input.ClaimGeneration, 10),
		input.SourceEvidenceID,
		input.SourceEvidenceDigest,
		input.OutputSummaryDigest,
		strconv.Itoa(input.OutputContractVersion),
		input.OutputContractDigest,
		string(input.OutputClassification),
		input.OutputClassificationDigest,
		input.AcceptanceContractDigest,
		input.TerminalStatus,
	)
}

func verifierCandidateDigest(candidate VerifierCandidate) string {
	input := candidate.input
	return canonicalOutputDigest(
		"loom.verifier-candidate.v1",
		string(candidate.kind),
		string(candidate.reasonCode),
		input.WorkItemID,
		input.RunID,
		input.ClaimID,
		strconv.FormatInt(input.ClaimGeneration, 10),
		input.RuntimeInstanceID,
		input.AgentInstanceID,
		input.GrantID,
		input.EvidenceID,
		input.EvidenceDigest,
		input.OutputSummaryDigest,
		input.TerminalStatus,
		input.TerminalReason,
		string(input.OutputReasonCode),
	)
}

func acceptanceDecisionDigest(decision AcceptanceDecision) string {
	return canonicalOutputDigest(
		"loom.acceptance-decision.v1",
		string(decision.kind),
		strconv.Itoa(decision.contractVersion),
		decision.contractDigest,
		string(decision.risk),
		decision.deterministicDigest,
		decision.verifierDigest,
		string(decision.reasonCode),
		decision.decisionTime.Format(time.RFC3339Nano),
	)
}

func zeroVerifierCandidate(candidate VerifierCandidate) bool {
	return candidate.kind == "" &&
		candidate.reasonCode == "" &&
		candidate.input == (VerifierTerminalInput{}) &&
		candidate.digest == ""
}
