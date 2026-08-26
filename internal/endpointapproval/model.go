package endpointapproval

import (
	"errors"
	"time"
)

var (
	ErrInvalidCandidate   = errors.New("endpointapproval: invalid review candidate")
	ErrInvalidCommand     = errors.New("endpointapproval: invalid approval command")
	ErrInvalidRecord      = errors.New("endpointapproval: invalid approval record")
	ErrForeignCandidate   = errors.New("endpointapproval: foreign review candidate")
	ErrDigestMismatch     = errors.New("endpointapproval: digest mismatch")
	ErrRevisionReplay     = errors.New("endpointapproval: replayed revision")
	ErrFutureRevision     = errors.New("endpointapproval: future revision")
	ErrOperationConflict  = errors.New("endpointapproval: operation conflict")
	ErrOperationReplay    = errors.New("endpointapproval: replayed operation")
	ErrInvalidTransition  = errors.New("endpointapproval: invalid transition")
	ErrTerminalApproval   = errors.New("endpointapproval: approval is terminal")
	ErrApprovalExpired    = errors.New("endpointapproval: approval has expired")
	ErrApprovalNotExpired = errors.New("endpointapproval: approval has not expired")
)

type ApprovalStatus string

const (
	StatusRequested ApprovalStatus = "requested"
	StatusApproved  ApprovalStatus = "approved"
	StatusRejected  ApprovalStatus = "rejected"
	StatusExpired   ApprovalStatus = "expired"
)

type CommandKind string

const (
	CommandRequest CommandKind = "request"
	CommandApprove CommandKind = "approve"
	CommandReject  CommandKind = "reject"
	CommandExpire  CommandKind = "expire"
)

// ReviewCandidateInput contains only non-secret endpoint identity metadata.
type ReviewCandidateInput struct {
	EndpointFingerprint string
	ProviderID          string
	AccountID           string
	Protocol            string
	ModelDigest         string
	ReviewPolicyVersion uint64
	ReviewPolicyDigest  string
}

// ReviewCandidate is the immutable subject presented for endpoint review.
type ReviewCandidate struct {
	endpointFingerprint string
	providerID          string
	accountID           string
	protocol            string
	modelDigest         string
	reviewPolicyVersion uint64
	reviewPolicyDigest  string
	digest              string
}

func NewReviewCandidate(input ReviewCandidateInput) (ReviewCandidate, error) {
	candidate := ReviewCandidate{
		endpointFingerprint: input.EndpointFingerprint,
		providerID:          input.ProviderID,
		accountID:           input.AccountID,
		protocol:            input.Protocol,
		modelDigest:         input.ModelDigest,
		reviewPolicyVersion: input.ReviewPolicyVersion,
		reviewPolicyDigest:  input.ReviewPolicyDigest,
	}
	if !validCandidateFields(candidate) {
		return ReviewCandidate{}, ErrInvalidCandidate
	}
	candidate.digest = digestCandidate(candidate)
	return candidate, nil
}

func (candidate ReviewCandidate) EndpointFingerprint() string { return candidate.endpointFingerprint }
func (candidate ReviewCandidate) ProviderID() string          { return candidate.providerID }
func (candidate ReviewCandidate) AccountID() string           { return candidate.accountID }
func (candidate ReviewCandidate) Protocol() string            { return candidate.protocol }
func (candidate ReviewCandidate) ModelDigest() string         { return candidate.modelDigest }
func (candidate ReviewCandidate) ReviewPolicyVersion() uint64 { return candidate.reviewPolicyVersion }
func (candidate ReviewCandidate) ReviewPolicyDigest() string  { return candidate.reviewPolicyDigest }
func (candidate ReviewCandidate) Digest() string              { return candidate.digest }

type ApprovalCommandInput struct {
	Kind                 CommandKind
	Actor                string
	OperationID          string
	ExpectedRevision     uint64
	ExpectedRecordDigest string
	OccurredAt           time.Time
	ExpiresAt            time.Time
}

// ApprovalCommand freezes both the candidate binding and the requested CAS.
type ApprovalCommand struct {
	kind                 CommandKind
	candidateDigest      string
	endpointFingerprint  string
	providerID           string
	accountID            string
	protocol             string
	modelDigest          string
	reviewPolicyVersion  uint64
	reviewPolicyDigest   string
	actor                string
	operationID          string
	expectedRevision     uint64
	expectedRecordDigest string
	occurredAt           time.Time
	expiresAt            time.Time
	digest               string
}

func NewApprovalCommand(
	candidate ReviewCandidate,
	input ApprovalCommandInput,
) (ApprovalCommand, error) {
	if !validCandidate(candidate) {
		return ApprovalCommand{}, ErrInvalidCandidate
	}
	command := ApprovalCommand{
		kind:                 input.Kind,
		candidateDigest:      candidate.digest,
		endpointFingerprint:  candidate.endpointFingerprint,
		providerID:           candidate.providerID,
		accountID:            candidate.accountID,
		protocol:             candidate.protocol,
		modelDigest:          candidate.modelDigest,
		reviewPolicyVersion:  candidate.reviewPolicyVersion,
		reviewPolicyDigest:   candidate.reviewPolicyDigest,
		actor:                input.Actor,
		operationID:          input.OperationID,
		expectedRevision:     input.ExpectedRevision,
		expectedRecordDigest: input.ExpectedRecordDigest,
		occurredAt:           input.OccurredAt,
		expiresAt:            input.ExpiresAt,
	}
	if !validCommandFields(command) {
		return ApprovalCommand{}, ErrInvalidCommand
	}
	command.digest = digestCommand(command)
	return command, nil
}

func (command ApprovalCommand) Kind() CommandKind            { return command.kind }
func (command ApprovalCommand) CandidateDigest() string      { return command.candidateDigest }
func (command ApprovalCommand) EndpointFingerprint() string  { return command.endpointFingerprint }
func (command ApprovalCommand) ProviderID() string           { return command.providerID }
func (command ApprovalCommand) AccountID() string            { return command.accountID }
func (command ApprovalCommand) Protocol() string             { return command.protocol }
func (command ApprovalCommand) ModelDigest() string          { return command.modelDigest }
func (command ApprovalCommand) ReviewPolicyVersion() uint64  { return command.reviewPolicyVersion }
func (command ApprovalCommand) ReviewPolicyDigest() string   { return command.reviewPolicyDigest }
func (command ApprovalCommand) Actor() string                { return command.actor }
func (command ApprovalCommand) OperationID() string          { return command.operationID }
func (command ApprovalCommand) ExpectedRevision() uint64     { return command.expectedRevision }
func (command ApprovalCommand) ExpectedRecordDigest() string { return command.expectedRecordDigest }
func (command ApprovalCommand) OccurredAt() time.Time        { return command.occurredAt }
func (command ApprovalCommand) ExpiresAt() time.Time         { return command.expiresAt }
func (command ApprovalCommand) Digest() string               { return command.digest }

// ApprovalRecord is the complete authoritative state for one candidate.
type ApprovalRecord struct {
	candidateDigest       string
	endpointFingerprint   string
	providerID            string
	accountID             string
	protocol              string
	modelDigest           string
	reviewPolicyVersion   uint64
	reviewPolicyDigest    string
	status                ApprovalStatus
	revision              uint64
	requestedBy           string
	requestOperationID    string
	requestCommandDigest  string
	requestedAt           time.Time
	expiresAt             time.Time
	decidedBy             string
	terminalOperationID   string
	terminalCommandDigest string
	decidedAt             time.Time
	digest                string
}

func (record ApprovalRecord) CandidateDigest() string     { return record.candidateDigest }
func (record ApprovalRecord) EndpointFingerprint() string { return record.endpointFingerprint }
func (record ApprovalRecord) ProviderID() string          { return record.providerID }
func (record ApprovalRecord) AccountID() string           { return record.accountID }
func (record ApprovalRecord) Protocol() string            { return record.protocol }
func (record ApprovalRecord) ModelDigest() string         { return record.modelDigest }
func (record ApprovalRecord) ReviewPolicyVersion() uint64 { return record.reviewPolicyVersion }
func (record ApprovalRecord) ReviewPolicyDigest() string  { return record.reviewPolicyDigest }
func (record ApprovalRecord) Status() ApprovalStatus      { return record.status }
func (record ApprovalRecord) Revision() uint64            { return record.revision }
func (record ApprovalRecord) RequestedBy() string         { return record.requestedBy }
func (record ApprovalRecord) RequestOperationID() string  { return record.requestOperationID }
func (record ApprovalRecord) RequestedAt() time.Time      { return record.requestedAt }
func (record ApprovalRecord) ExpiresAt() time.Time        { return record.expiresAt }
func (record ApprovalRecord) DecidedBy() string           { return record.decidedBy }
func (record ApprovalRecord) TerminalOperationID() string { return record.terminalOperationID }
func (record ApprovalRecord) DecidedAt() time.Time        { return record.decidedAt }
func (record ApprovalRecord) Digest() string              { return record.digest }
