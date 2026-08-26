package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
)

// Harness execution is bounded to 15 minutes. The lease still closes as soon
// as its parent request finishes, so shorter conversations and Agent Attempts
// do not retain plaintext until this upper bound.
const productCredentialLeaseTTL = 15 * time.Minute

type productCredentialLeaseAccess interface {
	UseCredential(
		context.Context,
		credentialvault.CredentialIdentity,
		func(context.Context, []byte) error,
	) error
}

type productCredentialLeaseRouteSlot struct {
	mu     sync.RWMutex
	access productCredentialLeaseAccess
	bound  bool
	closed bool
}

func (slot *productCredentialLeaseRouteSlot) Bind(
	access productCredentialLeaseAccess,
) error {
	if slot == nil || nilProductAssetPort(access) {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.bound || slot.closed {
		return composition.ErrCompositionConflict
	}
	slot.access = access
	slot.bound = true
	return nil
}

func (slot *productCredentialLeaseRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && !nilProductAssetPort(slot.access)
}

func (slot *productCredentialLeaseRouteSlot) Close() error {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	slot.closed = true
	slot.access = nil
	return nil
}

func (slot *productCredentialLeaseRouteSlot) UseCredential(
	ctx context.Context,
	identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	if slot == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if !slot.bound || slot.closed || nilProductAssetPort(slot.access) {
		return credentials.ErrCredentialStoreUnavailable
	}
	return slot.access.UseCredential(ctx, identity, use)
}

type productVaultCredentialLeaseAccess struct {
	manager *credentialvault.CredentialLeaseManager
}

func newProductVaultCredentialLeaseAccess(
	manager *credentialvault.CredentialLeaseManager,
) (*productVaultCredentialLeaseAccess, error) {
	if manager == nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	return &productVaultCredentialLeaseAccess{manager: manager}, nil
}

func (access *productVaultCredentialLeaseAccess) UseCredential(
	ctx context.Context,
	identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	if access == nil || access.manager == nil || ctx == nil || use == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	lease, err := access.manager.Acquire(ctx, identity, productCredentialLeaseTTL)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return credentials.WithCredentialFailureStage(
			productCredentialLeaseFailureStage(err),
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	defer lease.Close()
	callbackRan := false
	err = lease.WithSecret(func(leaseContext context.Context, secret []byte) error {
		callbackRan = true
		return use(leaseContext, secret)
	})
	if err == nil || callbackRan {
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return credentials.WithCredentialFailureStage(
		productCredentialLeaseFailureStage(err),
		credentials.ErrCredentialStoreUnavailable,
	)
}

type productLegacyCredentialLeaseAccess struct {
	store credentials.SecretStore
}

func newProductLegacyCredentialLeaseAccess(
	store credentials.SecretStore,
) (*productLegacyCredentialLeaseAccess, error) {
	if store == nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	return &productLegacyCredentialLeaseAccess{store: store}, nil
}

func (access *productLegacyCredentialLeaseAccess) UseCredential(
	ctx context.Context,
	identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	if access == nil || access.store == nil || ctx == nil || use == nil ||
		identity.CredentialReference == "" || identity.CredentialRevision <= 0 ||
		!credentials.ValidProviderAccountIdentifier(
			identity.ProviderID, identity.ProviderAccountID,
		) {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageLeaseIssue,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	secret, err := access.store.Read(ctx, identity.CredentialReference)
	if err != nil || len(secret) == 0 || len(secret) > 8192 {
		clearProductCredentialLeaseSecret(secret)
		stage := credentials.CredentialFailureStage(err)
		if stage == "" {
			stage = credentials.CredentialStageLeaseIssue
		}
		return credentials.WithCredentialFailureStage(
			stage,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	defer clearProductCredentialLeaseSecret(secret)
	return use(ctx, secret)
}

func productCredentialLeaseFailureStage(err error) string {
	if stage := credentials.CredentialFailureStage(err); stage != "" {
		return stage
	}
	switch {
	case errors.Is(err, credentialvault.ErrCredentialLeaseRevoked):
		return credentials.CredentialStageLeaseRevoke
	case errors.Is(err, credentialvault.ErrCredentialLeaseExpired):
		return credentials.CredentialStageLeaseExpire
	default:
		return credentials.CredentialStageLeaseIssue
	}
}

func clearProductCredentialLeaseSecret(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}
