package endpointapproval

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	maxIdentityLength  = 256
	maxProtocolLength  = 64
	maxOperationLength = 128
)

func digestCandidate(candidate ReviewCandidate) string {
	return digestFields(
		"loom.endpoint-review-candidate.v1",
		candidate.endpointFingerprint,
		candidate.providerID,
		candidate.accountID,
		candidate.protocol,
		candidate.modelDigest,
		fmt.Sprint(candidate.reviewPolicyVersion),
		candidate.reviewPolicyDigest,
	)
}

func digestCommand(command ApprovalCommand) string {
	return digestFields(
		"loom.endpoint-approval-command.v1",
		string(command.kind),
		command.candidateDigest,
		command.endpointFingerprint,
		command.providerID,
		command.accountID,
		command.protocol,
		command.modelDigest,
		fmt.Sprint(command.reviewPolicyVersion),
		command.reviewPolicyDigest,
		command.actor,
		command.operationID,
		fmt.Sprint(command.expectedRevision),
		command.expectedRecordDigest,
		canonicalTime(command.occurredAt),
		canonicalTime(command.expiresAt),
	)
}

func digestRecord(record ApprovalRecord) string {
	return digestFields(
		"loom.endpoint-approval-record.v1",
		record.candidateDigest,
		record.endpointFingerprint,
		record.providerID,
		record.accountID,
		record.protocol,
		record.modelDigest,
		fmt.Sprint(record.reviewPolicyVersion),
		record.reviewPolicyDigest,
		string(record.status),
		fmt.Sprint(record.revision),
		record.requestedBy,
		record.requestOperationID,
		record.requestCommandDigest,
		canonicalTime(record.requestedAt),
		canonicalTime(record.expiresAt),
		record.decidedBy,
		record.terminalOperationID,
		record.terminalCommandDigest,
		canonicalTime(record.decidedAt),
	)
}

func digestFields(fields ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func validCandidate(candidate ReviewCandidate) bool {
	return validCandidateFields(candidate) &&
		validSHA256(candidate.digest) &&
		candidate.digest == digestCandidate(candidate)
}

func validCandidateFields(candidate ReviewCandidate) bool {
	return validSHA256(candidate.endpointFingerprint) &&
		validIdentity(candidate.providerID, maxIdentityLength) &&
		validIdentity(candidate.accountID, maxIdentityLength) &&
		validProtocol(candidate.protocol) &&
		validSHA256(candidate.modelDigest) &&
		candidate.reviewPolicyVersion > 0 &&
		validSHA256(candidate.reviewPolicyDigest)
}

func validCommand(command ApprovalCommand) bool {
	return validCommandFields(command) &&
		validSHA256(command.digest) &&
		command.digest == digestCommand(command)
}

func validCommandFields(command ApprovalCommand) bool {
	if !validCommandKind(command.kind) ||
		!validSHA256(command.candidateDigest) ||
		!validSHA256(command.endpointFingerprint) ||
		!validIdentity(command.providerID, maxIdentityLength) ||
		!validIdentity(command.accountID, maxIdentityLength) ||
		!validProtocol(command.protocol) ||
		!validSHA256(command.modelDigest) ||
		command.reviewPolicyVersion == 0 ||
		!validSHA256(command.reviewPolicyDigest) ||
		!validIdentity(command.actor, maxIdentityLength) ||
		!validIdentity(command.operationID, maxOperationLength) ||
		!validUTC(command.occurredAt) {
		return false
	}
	if command.kind == CommandRequest {
		return command.expectedRevision == 0 &&
			command.expectedRecordDigest == "" &&
			validUTC(command.expiresAt) &&
			command.occurredAt.Before(command.expiresAt)
	}
	return validSHA256(command.expectedRecordDigest) && command.expiresAt.IsZero()
}

func validRecord(record ApprovalRecord) bool {
	if !validSHA256(record.candidateDigest) ||
		!validSHA256(record.endpointFingerprint) ||
		!validIdentity(record.providerID, maxIdentityLength) ||
		!validIdentity(record.accountID, maxIdentityLength) ||
		!validProtocol(record.protocol) ||
		!validSHA256(record.modelDigest) ||
		record.reviewPolicyVersion == 0 ||
		!validSHA256(record.reviewPolicyDigest) ||
		!validApprovalStatus(record.status) ||
		record.revision == 0 || record.revision > 2 ||
		!validIdentity(record.requestedBy, maxIdentityLength) ||
		!validIdentity(record.requestOperationID, maxOperationLength) ||
		!validSHA256(record.requestCommandDigest) ||
		!validUTC(record.requestedAt) || !validUTC(record.expiresAt) ||
		!record.requestedAt.Before(record.expiresAt) ||
		!validSHA256(record.digest) || record.digest != digestRecord(record) {
		return false
	}
	if record.status == StatusRequested {
		return record.revision == 1 && record.decidedBy == "" &&
			record.terminalOperationID == "" &&
			record.terminalCommandDigest == "" && record.decidedAt.IsZero()
	}
	return record.revision == 2 &&
		validIdentity(record.decidedBy, maxIdentityLength) &&
		validIdentity(record.terminalOperationID, maxOperationLength) &&
		validSHA256(record.terminalCommandDigest) && validUTC(record.decidedAt)
}

func commandMatchesCandidate(command ApprovalCommand, candidate ReviewCandidate) bool {
	return command.candidateDigest == candidate.digest &&
		command.endpointFingerprint == candidate.endpointFingerprint &&
		command.providerID == candidate.providerID &&
		command.accountID == candidate.accountID &&
		command.protocol == candidate.protocol &&
		command.modelDigest == candidate.modelDigest &&
		command.reviewPolicyVersion == candidate.reviewPolicyVersion &&
		command.reviewPolicyDigest == candidate.reviewPolicyDigest
}

func recordMatchesCandidate(record ApprovalRecord, candidate ReviewCandidate) bool {
	return record.candidateDigest == candidate.digest &&
		record.endpointFingerprint == candidate.endpointFingerprint &&
		record.providerID == candidate.providerID &&
		record.accountID == candidate.accountID &&
		record.protocol == candidate.protocol &&
		record.modelDigest == candidate.modelDigest &&
		record.reviewPolicyVersion == candidate.reviewPolicyVersion &&
		record.reviewPolicyDigest == candidate.reviewPolicyDigest
}

func validCommandKind(kind CommandKind) bool {
	return kind == CommandRequest || kind == CommandApprove ||
		kind == CommandReject || kind == CommandExpire
}

func validApprovalStatus(status ApprovalStatus) bool {
	return status == StatusRequested || status == StatusApproved ||
		status == StatusRejected || status == StatusExpired
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func validIdentity(value string, maxLength int) bool {
	if value == "" || len(value) > maxLength || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}

func validProtocol(value string) bool {
	if value == "" || len(value) > maxProtocolLength {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' && char != '.' && char != '+' {
			return false
		}
	}
	return true
}

func validUTC(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC
}

func canonicalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
