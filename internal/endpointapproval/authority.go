package endpointapproval

// Apply performs one pure, fail-closed authority transition.
func Apply(
	candidate ReviewCandidate,
	current *ApprovalRecord,
	command ApprovalCommand,
) (ApprovalRecord, error) {
	if !validCandidate(candidate) {
		return ApprovalRecord{}, ErrInvalidCandidate
	}
	if !validCommand(command) {
		return ApprovalRecord{}, ErrInvalidCommand
	}
	if !commandMatchesCandidate(command, candidate) {
		return ApprovalRecord{}, ErrForeignCandidate
	}
	if current == nil {
		return applyRequest(candidate, command)
	}
	if !validRecord(*current) {
		return ApprovalRecord{}, ErrInvalidRecord
	}
	if !recordMatchesCandidate(*current, candidate) {
		return ApprovalRecord{}, ErrForeignCandidate
	}

	if command.operationID == current.terminalOperationID {
		if command.digest == current.terminalCommandDigest {
			return *current, nil
		}
		return ApprovalRecord{}, ErrOperationConflict
	}
	if command.operationID == current.requestOperationID {
		if current.revision == 1 && command.digest == current.requestCommandDigest {
			return *current, nil
		}
		if command.digest == current.requestCommandDigest {
			return ApprovalRecord{}, ErrOperationReplay
		}
		return ApprovalRecord{}, ErrOperationConflict
	}
	if command.expectedRevision < current.revision {
		return ApprovalRecord{}, ErrRevisionReplay
	}
	if command.expectedRevision > current.revision {
		return ApprovalRecord{}, ErrFutureRevision
	}
	if command.expectedRecordDigest != current.digest {
		return ApprovalRecord{}, ErrDigestMismatch
	}
	if current.status != StatusRequested {
		return ApprovalRecord{}, ErrTerminalApproval
	}

	return applyTerminal(*current, command)
}

func applyRequest(
	candidate ReviewCandidate,
	command ApprovalCommand,
) (ApprovalRecord, error) {
	if command.kind != CommandRequest {
		return ApprovalRecord{}, ErrInvalidTransition
	}
	if command.expectedRevision > 0 {
		return ApprovalRecord{}, ErrFutureRevision
	}
	if command.expectedRecordDigest != "" {
		return ApprovalRecord{}, ErrDigestMismatch
	}
	record := ApprovalRecord{
		candidateDigest:      candidate.digest,
		endpointFingerprint:  candidate.endpointFingerprint,
		providerID:           candidate.providerID,
		accountID:            candidate.accountID,
		protocol:             candidate.protocol,
		modelDigest:          candidate.modelDigest,
		reviewPolicyVersion:  candidate.reviewPolicyVersion,
		reviewPolicyDigest:   candidate.reviewPolicyDigest,
		status:               StatusRequested,
		revision:             1,
		requestedBy:          command.actor,
		requestOperationID:   command.operationID,
		requestCommandDigest: command.digest,
		requestedAt:          command.occurredAt,
		expiresAt:            command.expiresAt,
	}
	record.digest = digestRecord(record)
	return record, nil
}

func applyTerminal(
	current ApprovalRecord,
	command ApprovalCommand,
) (ApprovalRecord, error) {
	switch command.kind {
	case CommandApprove, CommandReject:
		if command.occurredAt.Before(current.requestedAt) {
			return ApprovalRecord{}, ErrInvalidCommand
		}
		if !command.occurredAt.Before(current.expiresAt) {
			return ApprovalRecord{}, ErrApprovalExpired
		}
	case CommandExpire:
		if command.occurredAt.Before(current.expiresAt) {
			return ApprovalRecord{}, ErrApprovalNotExpired
		}
	case CommandRequest:
		return ApprovalRecord{}, ErrInvalidTransition
	default:
		return ApprovalRecord{}, ErrInvalidCommand
	}

	next := current
	switch command.kind {
	case CommandApprove:
		next.status = StatusApproved
	case CommandReject:
		next.status = StatusRejected
	case CommandExpire:
		next.status = StatusExpired
	}
	next.revision++
	next.decidedBy = command.actor
	next.terminalOperationID = command.operationID
	next.terminalCommandDigest = command.digest
	next.decidedAt = command.occurredAt
	next.digest = digestRecord(next)
	return next, nil
}
