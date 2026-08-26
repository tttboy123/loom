package vault

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"loom-pi-rebuild/internal/credentials"
)

const credentialCoordinatorLeaseTTL = 30 * time.Second

type CredentialMutationStore interface {
	PrepareCredentialMutation(
		context.Context,
		CredentialMutation,
	) (CredentialMutationReceipt, error)
	CommitCredentialMutation(context.Context, CredentialMutationReceipt) error
	RollbackCredentialMutation(context.Context, CredentialMutationReceipt) error
}

type CredentialVaultCoordinatorConfig struct {
	Store     CredentialMutationStore
	Leases    *CredentialLeaseManager
	Verifier  credentials.CredentialVerifier
	Committer credentials.MetadataCommitter
}

type CredentialVaultCoordinator struct {
	mu        sync.Mutex
	store     CredentialMutationStore
	leases    *CredentialLeaseManager
	verifier  credentials.CredentialVerifier
	committer credentials.MetadataCommitter
}

func NewCredentialVaultCoordinator(
	config CredentialVaultCoordinatorConfig,
) (*CredentialVaultCoordinator, error) {
	if config.Store == nil || config.Leases == nil ||
		config.Verifier == nil || config.Committer == nil {
		return nil, credentials.ErrInvalidCredentialCommand
	}
	return &CredentialVaultCoordinator{
		store: config.Store, leases: config.Leases,
		verifier: config.Verifier, committer: config.Committer,
	}, nil
}

func (coordinator *CredentialVaultCoordinator) Configure(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	defer clearBytes(command.Secret)
	if err := validateCoordinatorCommand(coordinator, ctx, command, true); err != nil ||
		command.ExpectedRevision != 0 {
		return credentials.MetadataResult{}, credentials.ErrInvalidCredentialCommand
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	candidate := coordinatorIdentity(command, command.ExpectedRevision+1)
	secret := append([]byte(nil), command.Secret...)
	receipt, err := coordinator.store.PrepareCredentialMutation(
		ctx,
		CredentialMutation{
			MutationID: coordinatorMutationID(command, MutationConfigure),
			Kind:       MutationConfigure, Candidate: candidate, Secret: secret,
		},
	)
	if err != nil {
		clearBytes(secret)
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultEncrypt, closeCoordinatorStoreError(err),
		)
	}
	return coordinator.commitPreparedMutation(
		ctx, command, receipt, credentials.CredentialConfigured,
		credentials.VerificationReasonNone, false,
	)
}

func (coordinator *CredentialVaultCoordinator) Verify(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if err := validateCoordinatorCommand(coordinator, ctx, command, false); err != nil ||
		command.ExpectedRevision <= 0 {
		return credentials.MetadataResult{}, credentials.ErrInvalidCredentialCommand
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	current := coordinatorIdentity(command, command.ExpectedRevision)
	lease, err := coordinator.leases.Acquire(ctx, current, credentialCoordinatorLeaseTTL)
	if err != nil {
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageLeaseIssue, closeCoordinatorStoreError(err),
		)
	}
	var verification credentials.VerificationResult
	verifyErr := lease.WithSecret(func(
		leaseContext context.Context,
		secret []byte,
	) error {
		var callErr error
		verification, callErr = credentials.VerifyCredentialBinding(
			leaseContext, coordinator.verifier,
			credentials.CredentialVerificationBinding{
				ProviderID:          command.ProviderID,
				ProviderAccountID:   command.ProviderAccountID,
				CredentialReference: command.CredentialReference,
				CredentialRevision:  command.ExpectedRevision,
			}, secret,
		)
		return callErr
	})
	_ = lease.Close()
	if verifyErr != nil {
		return credentials.MetadataResult{}, credentials.ClosedVerificationError(verifyErr)
	}
	if !credentials.ValidateVerificationResult(verification) {
		return credentials.MetadataResult{}, credentials.ErrCredentialRejected
	}
	status := credentials.CredentialVerified
	if verification.Status != credentials.VerificationValid {
		status = credentials.CredentialRejected
	}
	candidate := coordinatorIdentity(command, command.ExpectedRevision+1)
	receipt, err := coordinator.store.PrepareCredentialMutation(
		ctx,
		CredentialMutation{
			MutationID: coordinatorMutationID(command, MutationRebind),
			Kind:       MutationRebind, Current: current, Candidate: candidate,
		},
	)
	if err != nil {
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultDecrypt, closeCoordinatorStoreError(err),
		)
	}
	return coordinator.commitPreparedMutation(
		ctx, command, receipt, status, verification.Reason, true,
	)
}

func (coordinator *CredentialVaultCoordinator) Replace(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	defer clearBytes(command.Secret)
	if err := validateCoordinatorCommand(coordinator, ctx, command, true); err != nil ||
		command.ExpectedRevision <= 0 {
		return credentials.MetadataResult{}, credentials.ErrInvalidCredentialCommand
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	current := coordinatorIdentity(command, command.ExpectedRevision)
	candidate := coordinatorIdentity(command, command.ExpectedRevision+1)
	secret := append([]byte(nil), command.Secret...)
	receipt, err := coordinator.store.PrepareCredentialMutation(
		ctx,
		CredentialMutation{
			MutationID: coordinatorMutationID(command, MutationReplace),
			Kind:       MutationReplace, Current: current, Candidate: candidate,
			Secret: secret,
		},
	)
	if errors.Is(err, credentials.ErrCredentialNotFound) {
		secret = append([]byte(nil), command.Secret...)
		receipt, err = coordinator.store.PrepareCredentialMutation(
			ctx,
			CredentialMutation{
				MutationID: coordinatorMutationID(command, MutationImport),
				Kind:       MutationImport, Current: current, Candidate: candidate,
				Secret: secret,
			},
		)
	}
	if err != nil {
		clearBytes(secret)
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultEncrypt, closeCoordinatorStoreError(err),
		)
	}
	return coordinator.commitPreparedMutation(
		ctx, command, receipt, credentials.CredentialConfigured,
		credentials.VerificationReasonNone, false,
	)
}

func (coordinator *CredentialVaultCoordinator) Revoke(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if err := validateCoordinatorCommand(coordinator, ctx, command, false); err != nil ||
		command.ExpectedRevision <= 0 {
		return credentials.MetadataResult{}, credentials.ErrInvalidCredentialCommand
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	current := coordinatorIdentity(command, command.ExpectedRevision)
	candidate := coordinatorIdentity(command, command.ExpectedRevision+1)
	receipt, err := coordinator.store.PrepareCredentialMutation(
		ctx,
		CredentialMutation{
			MutationID: coordinatorMutationID(command, MutationRevoke),
			Kind:       MutationRevoke, Current: current, Candidate: candidate,
		},
	)
	if err != nil {
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultCommit, closeCoordinatorStoreError(err),
		)
	}
	return coordinator.commitPreparedMutation(
		ctx, command, receipt, credentials.CredentialRevoked,
		credentials.VerificationReasonNone, false,
	)
}

func (coordinator *CredentialVaultCoordinator) ReconcileCredentialMutation(
	ctx context.Context,
	receipt CredentialMutationReceipt,
	metadata credentials.MetadataResult,
	found bool,
) error {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialMutationReceipt(receipt) {
		return credentials.ErrInvalidCredentialCommand
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if found && metadata.ProviderID == receipt.Candidate.ProviderID &&
		metadata.ProviderAccountID == receipt.Candidate.ProviderAccountID &&
		metadata.CredentialReference == receipt.Candidate.CredentialReference &&
		metadata.Revision == receipt.Candidate.CredentialRevision &&
		validReconciliationCandidate(receipt.Kind, metadata) {
		if receipt.Current != (CredentialIdentity{}) {
			coordinator.leases.Revoke(receipt.Current)
		}
		if err := coordinator.store.CommitCredentialMutation(ctx, receipt); err != nil {
			return credentials.WithCredentialFailureStage(
				credentials.CredentialStageVaultCommit,
				closeCoordinatorStoreError(err),
			)
		}
		return nil
	}
	if !found && receipt.Kind == MutationConfigure ||
		found && receipt.Current != (CredentialIdentity{}) &&
			metadata.ProviderID == receipt.Current.ProviderID &&
			metadata.ProviderAccountID == receipt.Current.ProviderAccountID &&
			metadata.CredentialReference == receipt.Current.CredentialReference &&
			metadata.Revision == receipt.Current.CredentialRevision {
		if err := coordinator.store.RollbackCredentialMutation(ctx, receipt); err != nil {
			return credentials.WithCredentialFailureStage(
				credentials.CredentialStageVaultCommit,
				closeCoordinatorStoreError(err),
			)
		}
		return nil
	}
	return credentials.ErrCredentialMetadataConflict
}

func (coordinator *CredentialVaultCoordinator) commitPreparedMutation(
	ctx context.Context,
	command credentials.CredentialCommand,
	receipt CredentialMutationReceipt,
	status credentials.CredentialStatus,
	reason credentials.VerificationReason,
	observed bool,
) (credentials.MetadataResult, error) {
	commitContext := ctx
	var cancel context.CancelFunc
	if observed {
		commitContext, cancel = context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
	}
	result, err := coordinator.committer.CommitCredentialMetadata(
		commitContext,
		credentials.MetadataCommand{
			CommandID: command.CommandID, ProviderID: command.ProviderID,
			ProviderAccountID:   command.ProviderAccountID,
			CredentialReference: command.CredentialReference,
			ExpectedRevision:    command.ExpectedRevision, OccurredAt: command.OccurredAt,
			Status: status, Reason: reason,
		},
	)
	if err == nil && !validCoordinatorMetadataResult(result, receipt.Candidate, status, reason) {
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageMetadataCommit,
			credentials.ErrCredentialMetadataConflict,
		)
	}
	if err != nil {
		metadataErr := credentials.WithCredentialFailureStage(
			credentials.CredentialStageMetadataCommit, err,
		)
		if !errors.Is(err, credentials.ErrCredentialMetadataConflict) {
			return credentials.MetadataResult{}, metadataErr
		}
		rollbackContext, cancelRollback := context.WithTimeout(
			context.WithoutCancel(ctx), time.Second,
		)
		defer cancelRollback()
		if rollbackErr := coordinator.store.RollbackCredentialMutation(
			rollbackContext, receipt,
		); rollbackErr != nil {
			return credentials.MetadataResult{}, errors.Join(
				metadataErr, credentials.ErrCredentialRollbackFailed,
			)
		}
		return credentials.MetadataResult{}, metadataErr
	}
	if receipt.Current != (CredentialIdentity{}) {
		coordinator.leases.Revoke(receipt.Current)
	}
	finalizeContext, cancelFinalize := context.WithTimeout(
		context.WithoutCancel(ctx), 2*time.Second,
	)
	defer cancelFinalize()
	if err := coordinator.store.CommitCredentialMutation(
		finalizeContext, receipt,
	); err != nil {
		return credentials.MetadataResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultCommit, closeCoordinatorStoreError(err),
		)
	}
	return result, nil
}

func validReconciliationCandidate(
	kind CredentialMutationKind,
	metadata credentials.MetadataResult,
) bool {
	switch kind {
	case MutationConfigure, MutationImport, MutationReplace:
		return metadata.Status == credentials.CredentialConfigured &&
			metadata.Reason == credentials.VerificationReasonNone
	case MutationRebind:
		return (metadata.Status == credentials.CredentialVerified ||
			metadata.Status == credentials.CredentialRejected) &&
			(metadata.Reason == credentials.VerificationReasonNone ||
				metadata.Reason == credentials.VerificationReasonProviderRejected ||
				metadata.Reason == credentials.VerificationReasonUnavailable ||
				metadata.Reason == credentials.VerificationReasonTimeout)
	case MutationRevoke:
		return metadata.Status == credentials.CredentialRevoked &&
			metadata.Reason == credentials.VerificationReasonNone
	default:
		return false
	}
}

func validateCoordinatorCommand(
	coordinator *CredentialVaultCoordinator,
	ctx context.Context,
	command credentials.CredentialCommand,
	requireSecret bool,
) error {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		!credentials.ValidProviderAccountIdentifier(
			command.ProviderID, command.ProviderAccountID,
		) {
		return credentials.ErrInvalidCredentialCommand
	}
	return credentials.ValidateCredentialCommand(command, requireSecret)
}

func coordinatorIdentity(
	command credentials.CredentialCommand,
	revision int64,
) CredentialIdentity {
	return CredentialIdentity{
		ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		CredentialRevision:  revision,
	}
}

func coordinatorMutationID(
	command credentials.CredentialCommand,
	kind CredentialMutationKind,
) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"loom/credential-mutation/v1\x00%s\x00%s\x00%s\x00%s\x00%d\x00%s",
		command.CommandID, command.ProviderID, command.ProviderAccountID,
		command.CredentialReference, command.ExpectedRevision, kind,
	)))
	return "credential-mutation-" + hex.EncodeToString(digest[:16])
}

func validCoordinatorMetadataResult(
	result credentials.MetadataResult,
	candidate CredentialIdentity,
	status credentials.CredentialStatus,
	reason credentials.VerificationReason,
) bool {
	return result.ProviderID == candidate.ProviderID &&
		result.ProviderAccountID == candidate.ProviderAccountID &&
		result.CredentialReference == candidate.CredentialReference &&
		result.Revision == candidate.CredentialRevision &&
		result.Status == status && result.Reason == reason
}

func closeCoordinatorStoreError(err error) error {
	switch {
	case errors.Is(err, credentials.ErrCredentialNotFound):
		return credentials.ErrCredentialNotFound
	case errors.Is(err, credentials.ErrCredentialMetadataConflict):
		return credentials.ErrCredentialMetadataConflict
	case errors.Is(err, credentials.ErrCredentialStoreDenied):
		return credentials.ErrCredentialStoreDenied
	default:
		return credentials.ErrCredentialStoreUnavailable
	}
}
