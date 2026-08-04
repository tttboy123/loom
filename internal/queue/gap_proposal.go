package queue

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type GapSource struct {
	SourceType    string   `json:"source_type"`
	SourceIDs     []string `json:"source_ids"`
	SourceDigests []string `json:"source_digests"`
}

type GapProposal struct {
	GapID              string         `json:"gap_id"`
	Source             GapSource      `json:"source"`
	AffectedCapability string         `json:"affected_capability"`
	ObservedBehavior   string         `json:"observed_behavior"`
	ExpectedBehavior   string         `json:"expected_behavior"`
	UserImpact         string         `json:"user_impact"`
	Confidence         string         `json:"confidence"`
	Uncertainty        string         `json:"uncertainty"`
	Reproducibility    string         `json:"reproducibility"`
	PrivacyClass       string         `json:"privacy_classification"`
	ProposedScope      string         `json:"proposed_scope"`
	OwnedPathClaims    []string       `json:"owned_path_claims"`
	ResourceClaims     ResourceClaims `json:"resource_claims"`
	RiskClass          string         `json:"risk_class"`
	RollbackIdea       string         `json:"rollback_idea"`
	DuplicateOf        string         `json:"duplicate_of"`
	Supersedes         []string       `json:"supersedes"`
	Disposition        string         `json:"disposition"`
	CreatedAt          string         `json:"created_at"`
	CorrelationID      string         `json:"correlation_id"`
}

type GapProposalSubmission struct {
	SourceType         string         `json:"source_type"`
	SourceIDs          []string       `json:"source_ids"`
	SourceDigests      []string       `json:"source_digests"`
	AffectedCapability string         `json:"affected_capability"`
	ObservedBehavior   string         `json:"observed_behavior"`
	ExpectedBehavior   string         `json:"expected_behavior"`
	UserImpact         string         `json:"user_impact"`
	Confidence         string         `json:"confidence"`
	Uncertainty        string         `json:"uncertainty"`
	Reproducibility    string         `json:"reproducibility"`
	PrivacyClass       string         `json:"privacy_classification"`
	ProposedScope      string         `json:"proposed_scope"`
	OwnedPathClaims    []string       `json:"owned_path_claims"`
	ResourceClaims     ResourceClaims `json:"resource_claims"`
	RiskClass          string         `json:"risk_class"`
	RollbackIdea       string         `json:"rollback_idea"`
	Disposition        string         `json:"disposition,omitempty"`
}

type SuccessorCompileRequest struct {
	GapID                 string        `json:"gap_id"`
	ExpectedSourceDigests []string      `json:"expected_source_digests"`
	Submission            JobSubmission `json:"submission"`
}

type SuccessorProposal struct {
	SuccessorProposalID   string        `json:"successor_proposal_id"`
	GapID                 string        `json:"gap_id"`
	JobSubmission         JobSubmission `json:"job_submission"`
	SourceEvidenceDigests []string      `json:"source_evidence_digests"`
	Disposition           string        `json:"disposition"`
	CreatedAt             string        `json:"created_at"`
	CorrelationID         string        `json:"correlation_id"`
}

func CompileGap(input GapProposalSubmission) (GapProposal, error) {
	if input.SourceType == "" || len(input.SourceIDs) == 0 || len(input.SourceDigests) == 0 {
		return GapProposal{}, fmt.Errorf("%w: gap source type/ids/digests required", ErrInvalidInput)
	}
	if input.AffectedCapability == "" || input.ObservedBehavior == "" {
		return GapProposal{}, fmt.Errorf("%w: affected capability and observed behavior required", ErrInvalidInput)
	}
	switch input.Confidence {
	case "high", "medium", "low":
	default:
		return GapProposal{}, fmt.Errorf("%w: invalid confidence", ErrInvalidInput)
	}
	switch input.PrivacyClass {
	case "none", "pii", "credential", "other_sensitive":
	default:
		return GapProposal{}, fmt.Errorf("%w: invalid privacy classification", ErrInvalidInput)
	}
	disposition := input.Disposition
	if disposition == "" {
		disposition = "observe"
	}
	switch disposition {
	case "observe", "reject", "merge_duplicate", "human_required", "propose_successor":
	default:
		return GapProposal{}, fmt.Errorf("%w: invalid disposition", ErrInvalidInput)
	}
	source := GapSource{
		SourceType:    input.SourceType,
		SourceIDs:     append([]string(nil), input.SourceIDs...),
		SourceDigests: append([]string(nil), input.SourceDigests...),
	}
	sort.Strings(source.SourceIDs)
	sort.Strings(source.SourceDigests)
	return GapProposal{
		GapID:              GapKey(source, input.AffectedCapability),
		Source:             source,
		AffectedCapability: input.AffectedCapability,
		ObservedBehavior:   input.ObservedBehavior,
		ExpectedBehavior:   input.ExpectedBehavior,
		UserImpact:         input.UserImpact,
		Confidence:         input.Confidence,
		Uncertainty:        input.Uncertainty,
		Reproducibility:    input.Reproducibility,
		PrivacyClass:       input.PrivacyClass,
		ProposedScope:      input.ProposedScope,
		OwnedPathClaims:    append([]string(nil), input.OwnedPathClaims...),
		ResourceClaims:     input.ResourceClaims,
		RiskClass:          input.RiskClass,
		RollbackIdea:       input.RollbackIdea,
		Disposition:        disposition,
	}, nil
}

// GapKey derives exactly one digest-bound gap_id per authorized source:
// sha256 of the canonical (source_type, sorted source_digests,
// affected_capability) tuple. Source IDs are deliberately excluded so
// duplicate observations of the same digest-bound gap converge.
func GapKey(source GapSource, affectedCapability string) string {
	canonical := struct {
		Type       string   `json:"t"`
		Digests    []string `json:"d"`
		Capability string   `json:"c"`
	}{
		Type: source.SourceType, Digests: source.SourceDigests,
		Capability: affectedCapability,
	}
	data, _ := json.Marshal(canonical)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FindDuplicateGap returns the existing digest-bound gap for the candidate,
// if any. Duplicate observations converge on the existing gap_id.
func FindDuplicateGap(projection *Projection, candidate GapProposal) (GapProposal, bool) {
	for _, existing := range projection.Gaps {
		if existing.GapID == candidate.GapID {
			return existing, true
		}
	}
	return GapProposal{}, false
}

// CompileSuccessor produces a read-only SuccessorProposal bound to the gap's
// exact source digests. Stale or unauthorized evidence is rejected
// (SF-W1 RED #6); compilation creates no QueueJob/WorkItem side effect.
func CompileSuccessor(gap GapProposal, request SuccessorCompileRequest) (SuccessorProposal, error) {
	if gap.Disposition != "propose_successor" {
		return SuccessorProposal{}, fmt.Errorf("%w: gap disposition %q is not propose_successor", ErrDenied, gap.Disposition)
	}
	if !sameDigests(gap.Source.SourceDigests, request.ExpectedSourceDigests) {
		return SuccessorProposal{}, fmt.Errorf("%w: stale or unauthorized evidence", ErrDenied)
	}
	submission := request.Submission
	if submission.Source != "gap_proposal" {
		return SuccessorProposal{}, fmt.Errorf("%w: successor source must be gap_proposal", ErrDenied)
	}
	if _, err := CompileSubmission(submission); err != nil {
		return SuccessorProposal{}, err
	}
	return SuccessorProposal{
		SuccessorProposalID:   "spr-" + gap.GapID,
		GapID:                 gap.GapID,
		JobSubmission:         submission,
		SourceEvidenceDigests: append([]string(nil), gap.Source.SourceDigests...),
		Disposition:           "propose_successor",
	}, nil
}

func sameDigests(left, right []string) bool {
	leftSorted := append([]string(nil), left...)
	rightSorted := append([]string(nil), right...)
	sort.Strings(leftSorted)
	sort.Strings(rightSorted)
	if len(leftSorted) != len(rightSorted) {
		return false
	}
	for index := range leftSorted {
		if leftSorted[index] != rightSorted[index] {
			return false
		}
	}
	return true
}
