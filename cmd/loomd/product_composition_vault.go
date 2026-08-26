package main

import (
	"context"
	"errors"
	"io"
	"sync"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/toolproposal"
)

var errProductVaultConstruction = errors.New("product Vault construction failed")

type productVaultRoutes struct {
	owner                io.Closer
	controller           productCredentialVaultController
	leases               productCredentialLeaseAccess
	mutator              app.CredentialMutator
	availability         productCredentialAvailability
	contextCapsules      app.MissionContextCapsuleStore
	conversationCapsules api.LocalProductConversationContextCapsuleStore
	documents            api.LocalProductChatDocumentStore
	retrieval            contextcapsule.RetrievalStore
	attemptPayloads      attemptpayload.Store
	agentInputs          agentinbox.Store
	agentCheckpoints     agentcheckpoint.Store
	toolProposals        toolproposal.Store
	externalSessions     productExternalSessionHandleAccess
}

func (routes productVaultRoutes) valid() bool {
	return !nilProductAssetPort(routes.owner) &&
		!nilProductAssetPort(routes.controller) &&
		!nilProductAssetPort(routes.leases) &&
		!nilProductAssetPort(routes.mutator) &&
		!nilProductAssetPort(routes.availability)
}

type productVaultRouteSlot struct {
	mu     sync.RWMutex
	routes productVaultRoutes
	bound  bool
	closed bool
}

func (slot *productVaultRouteSlot) Bind(routes productVaultRoutes) error {
	if slot == nil || !routes.valid() {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.bound || slot.closed {
		return composition.ErrCompositionConflict
	}
	slot.routes = routes
	slot.bound = true
	return nil
}

func (slot *productVaultRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productVaultRouteSlot) Close() error {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	owner := slot.routes.owner
	slot.routes = productVaultRoutes{}
	if owner == nil {
		return nil
	}
	return owner.Close()
}

func newProductVaultRouteFactory(
	statePath string,
	store *journal.Store,
	readModel *projection.Projection,
) func(context.Context) (productVaultRoutes, error) {
	return func(ctx context.Context) (productVaultRoutes, error) {
		runtime, err := newProductCredentialVaultRuntime(
			ctx, statePath, store, readModel,
		)
		if err != nil {
			stage := credentials.CredentialFailureStage(err)
			if stage != credentials.CredentialStageVaultKeyLoad &&
				stage != credentials.CredentialStageVaultOpen &&
				stage != credentials.CredentialStageVaultRotation {
				return productVaultRoutes{}, errors.Join(errProductVaultConstruction, err)
			}
			recovery := newProductCredentialVaultRecoveryRuntime(
				err, statePath, store, readModel,
			)
			return productVaultRoutes{
				owner: recovery, controller: recovery, leases: recovery,
				mutator: recovery, availability: recovery,
				contextCapsules: recovery, conversationCapsules: recovery,
				documents: recovery, retrieval: recovery, attemptPayloads: recovery,
				agentInputs: recovery, agentCheckpoints: recovery,
				toolProposals: recovery, externalSessions: recovery,
			}, nil
		}
		return productVaultRoutes{
			owner: runtime, controller: runtime, leases: runtime,
			mutator: runtime, availability: runtime,
			contextCapsules: runtime, conversationCapsules: runtime,
			documents: runtime, retrieval: runtime, attemptPayloads: runtime,
			agentInputs: runtime, agentCheckpoints: runtime,
			toolProposals: runtime.store, externalSessions: runtime,
		}, nil
	}
}

func newProductVaultRouteFactoryFromCore(
	statePath string,
	core *productCoreRouteSlot,
) func(context.Context) (productVaultRoutes, error) {
	return func(ctx context.Context) (productVaultRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productVaultRoutes{}, err
		}
		return newProductVaultRouteFactory(
			statePath, resources.store, resources.readModel,
		)(ctx)
	}
}

func newProductLegacyCredentialLeaseFactory(
	store credentials.SecretStore,
) func(context.Context) (productCredentialLeaseAccess, error) {
	return func(ctx context.Context) (productCredentialLeaseAccess, error) {
		if ctx == nil || ctx.Err() != nil {
			return nil, credentials.ErrCredentialStoreUnavailable
		}
		return newProductLegacyCredentialLeaseAccess(store)
	}
}

func (construction productCompatibilityConstruction) startVault(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	if construction.legacyCredentialSlot != nil {
		access, err := construction.legacyCredentialFactory(ctx)
		if err != nil || nilProductAssetPort(access) {
			return nil, errors.Join(errProductVaultConstruction, err)
		}
		if err := construction.legacyCredentialSlot.Bind(access); err != nil {
			return nil, err
		}
		return composition.NewEffect(func(context.Context) error {
			return construction.legacyCredentialSlot.Close()
		}), nil
	}
	if construction.vaultSlot == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.vaultFactory(ctx)
	if err != nil || !routes.valid() {
		if routes.owner != nil {
			_ = routes.owner.Close()
		}
		return nil, errors.Join(errProductVaultConstruction, err)
	}
	if err := construction.vaultSlot.Bind(routes); err != nil {
		_ = routes.owner.Close()
		return nil, err
	}
	return composition.NewEffect(func(context.Context) error {
		return construction.vaultSlot.Close()
	}), nil
}

func (construction productCompatibilityConstruction) vaultReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() {
		return credentials.ErrCredentialStoreUnavailable
	}
	if construction.legacyCredentialSlot != nil {
		if construction.legacyCredentialSlot.Ready() {
			return nil
		}
		return credentials.ErrCredentialStoreUnavailable
	}
	if construction.vaultSlot == nil || !construction.vaultSlot.Ready() {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (slot *productVaultRouteSlot) unavailable() error {
	return credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultOpen,
		credentials.ErrCredentialStoreUnavailable,
	)
}

func (slot *productVaultRouteSlot) CredentialVaultStatus(
	ctx context.Context,
) (app.CredentialVaultStatus, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.CredentialVaultStatus{}, slot.unavailable()
	}
	status, ok := slot.routes.availability.(app.CredentialVaultStatusSource)
	if !ok {
		return app.CredentialVaultStatus{}, slot.unavailable()
	}
	return status.CredentialVaultStatus(ctx)
}

func (slot *productVaultRouteSlot) CredentialAvailable(
	ctx context.Context, providerID, providerAccountID, reference string, revision int64,
) (bool, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return false, slot.unavailable()
	}
	return slot.routes.availability.CredentialAvailable(
		ctx, providerID, providerAccountID, reference, revision,
	)
}

func (slot *productVaultRouteSlot) UseCredential(
	ctx context.Context, identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return slot.unavailable()
	}
	return slot.routes.leases.UseCredential(ctx, identity, use)
}

func (slot *productVaultRouteSlot) Configure(
	ctx context.Context, command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return slot.mutate(ctx, command, func(
		mutator app.CredentialMutator,
	) (credentials.MetadataResult, error) {
		return mutator.Configure(ctx, command)
	})
}

func (slot *productVaultRouteSlot) Verify(
	ctx context.Context, command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return slot.mutate(ctx, command, func(
		mutator app.CredentialMutator,
	) (credentials.MetadataResult, error) {
		return mutator.Verify(ctx, command)
	})
}

func (slot *productVaultRouteSlot) Replace(
	ctx context.Context, command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return slot.mutate(ctx, command, func(
		mutator app.CredentialMutator,
	) (credentials.MetadataResult, error) {
		return mutator.Replace(ctx, command)
	})
}

func (slot *productVaultRouteSlot) Revoke(
	ctx context.Context, command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return slot.mutate(ctx, command, func(
		mutator app.CredentialMutator,
	) (credentials.MetadataResult, error) {
		return mutator.Revoke(ctx, command)
	})
}

func (slot *productVaultRouteSlot) mutate(
	_ context.Context, command credentials.CredentialCommand,
	call func(app.CredentialMutator) (credentials.MetadataResult, error),
) (credentials.MetadataResult, error) {
	if slot == nil || call == nil {
		clearProductCredentialBytes(command.Secret)
		return credentials.MetadataResult{}, credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		clearProductCredentialBytes(command.Secret)
		return credentials.MetadataResult{}, slot.unavailable()
	}
	return call(slot.routes.mutator)
}

func (slot *productVaultRouteSlot) RotateCredentialVault(ctx context.Context) error {
	return slot.control(func(controller productCredentialVaultController) error {
		return controller.RotateCredentialVault(ctx)
	})
}
func (slot *productVaultRouteSlot) LockCredentialVault(ctx context.Context) error {
	return slot.control(func(controller productCredentialVaultController) error {
		return controller.LockCredentialVault(ctx)
	})
}
func (slot *productVaultRouteSlot) UnlockCredentialVault(ctx context.Context) error {
	return slot.control(func(controller productCredentialVaultController) error {
		return controller.UnlockCredentialVault(ctx)
	})
}
func (slot *productVaultRouteSlot) ResetCredentialVault(ctx context.Context, confirmation string) error {
	return slot.control(func(controller productCredentialVaultController) error {
		return controller.ResetCredentialVault(ctx, confirmation)
	})
}
func (slot *productVaultRouteSlot) ExportCredentialVault(
	ctx context.Context, passphrase []byte, destination string,
) (productCredentialVaultExportResult, error) {
	if slot == nil {
		clearProductCredentialBytes(passphrase)
		return productCredentialVaultExportResult{}, credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		clearProductCredentialBytes(passphrase)
		return productCredentialVaultExportResult{}, slot.unavailable()
	}
	return slot.routes.controller.ExportCredentialVault(ctx, passphrase, destination)
}
func (slot *productVaultRouteSlot) control(call func(productCredentialVaultController) error) error {
	if slot == nil || call == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return slot.unavailable()
	}
	return call(slot.routes.controller)
}

func (slot *productVaultRouteSlot) PutRoleContextCapsule(ctx context.Context, capsule contextcapsule.RoleContextCapsule, payload []byte) error {
	if slot == nil {
		clearProductCredentialBytes(payload)
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.contextCapsules) {
		clearProductCredentialBytes(payload)
		return slot.unavailable()
	}
	return slot.routes.contextCapsules.PutRoleContextCapsule(ctx, capsule, payload)
}
func (slot *productVaultRouteSlot) ReadRoleContextCapsule(ctx context.Context, authority contextcapsule.AuthorityRecord) (contextcapsule.RoleContextCapsule, []byte, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.contextCapsules) {
		return contextcapsule.RoleContextCapsule{}, nil, slot.unavailable()
	}
	return slot.routes.contextCapsules.ReadRoleContextCapsule(ctx, authority)
}
func (slot *productVaultRouteSlot) ListRoleContextCapsuleAuthorities(ctx context.Context, conversationID string) ([]contextcapsule.AuthorityRecord, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.contextCapsules) {
		return nil, slot.unavailable()
	}
	return slot.routes.contextCapsules.ListRoleContextCapsuleAuthorities(ctx, conversationID)
}
func (slot *productVaultRouteSlot) DeleteRoleContextCapsule(ctx context.Context, authority contextcapsule.AuthorityRecord) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.conversationCapsules) {
		return slot.unavailable()
	}
	return slot.routes.conversationCapsules.DeleteRoleContextCapsule(ctx, authority)
}
func (slot *productVaultRouteSlot) DeleteContextConversation(ctx context.Context, conversationID string) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.conversationCapsules) {
		return slot.unavailable()
	}
	return slot.routes.conversationCapsules.DeleteContextConversation(ctx, conversationID)
}
func (slot *productVaultRouteSlot) ConversationDocuments(ctx context.Context, kind string) ([]api.LocalProductChatDocument, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.documents) {
		return nil, slot.unavailable()
	}
	return slot.routes.documents.ConversationDocuments(ctx, kind)
}
func (slot *productVaultRouteSlot) PutConversationDocument(ctx context.Context, document api.LocalProductChatDocument) error {
	if slot == nil {
		clearProductCredentialBytes(document.Payload)
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.documents) {
		clearProductCredentialBytes(document.Payload)
		return slot.unavailable()
	}
	return slot.routes.documents.PutConversationDocument(ctx, document)
}
func (slot *productVaultRouteSlot) DeleteConversationDocument(ctx context.Context, conversationID string, kind string) error {
	if slot == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.documents) {
		return slot.unavailable()
	}
	return slot.routes.documents.DeleteConversationDocument(ctx, conversationID, kind)
}
func (slot *productVaultRouteSlot) RetrieveContextItem(ctx context.Context, request contextcapsule.RetrievalRequest) (contextcapsule.RetrievedItem, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.retrieval) {
		return contextcapsule.RetrievedItem{}, slot.unavailable()
	}
	return slot.routes.retrieval.RetrieveContextItem(ctx, request)
}
func (slot *productVaultRouteSlot) PutAttemptPayload(ctx context.Context, payload attemptpayload.Payload) error {
	if slot == nil {
		payload.Close()
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.attemptPayloads) {
		payload.Close()
		return slot.unavailable()
	}
	return slot.routes.attemptPayloads.PutAttemptPayload(ctx, payload)
}
func (slot *productVaultRouteSlot) ReadAttemptPayload(ctx context.Context, binding attemptpayload.Binding) (attemptpayload.Payload, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.attemptPayloads) {
		return attemptpayload.Payload{}, slot.unavailable()
	}
	return slot.routes.attemptPayloads.ReadAttemptPayload(ctx, binding)
}
func (slot *productVaultRouteSlot) ListPendingAttemptPayloads(ctx context.Context, scope attemptpayload.Scope) ([]attemptpayload.Payload, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.attemptPayloads) {
		return nil, slot.unavailable()
	}
	return slot.routes.attemptPayloads.ListPendingAttemptPayloads(ctx, scope)
}
func (slot *productVaultRouteSlot) MarkAttemptPayloadDelivered(ctx context.Context, binding attemptpayload.Binding) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.attemptPayloads) {
		return slot.unavailable()
	}
	return slot.routes.attemptPayloads.MarkAttemptPayloadDelivered(ctx, binding)
}
func (slot *productVaultRouteSlot) DeleteAttemptPayload(ctx context.Context, binding attemptpayload.Binding) error {
	if slot == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.attemptPayloads) {
		return slot.unavailable()
	}
	rollback, ok := slot.routes.attemptPayloads.(attemptpayload.RollbackStore)
	if !ok || nilProductAssetPort(rollback) {
		return credentials.ErrCredentialStoreUnavailable
	}
	return rollback.DeleteAttemptPayload(ctx, binding)
}
func (slot *productVaultRouteSlot) PutAgentInput(ctx context.Context, payload agentinbox.Payload) error {
	if slot == nil {
		payload.Close()
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentInputs) {
		payload.Close()
		return slot.unavailable()
	}
	return slot.routes.agentInputs.PutAgentInput(ctx, payload)
}
func (slot *productVaultRouteSlot) ReadAgentInput(ctx context.Context, binding agentinbox.Binding) (agentinbox.Payload, error) {
	if slot == nil {
		return agentinbox.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentInputs) {
		return agentinbox.Payload{}, slot.unavailable()
	}
	return slot.routes.agentInputs.ReadAgentInput(ctx, binding)
}
func (slot *productVaultRouteSlot) ListPendingAgentInputs(ctx context.Context, conversationID, runID, agentInstanceID string, claimGeneration int64) ([]agentinbox.Payload, error) {
	if slot == nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentInputs) {
		return nil, slot.unavailable()
	}
	return slot.routes.agentInputs.ListPendingAgentInputs(ctx, conversationID, runID, agentInstanceID, claimGeneration)
}
func (slot *productVaultRouteSlot) MarkAgentInputConsumed(ctx context.Context, binding agentinbox.Binding) error {
	if slot == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentInputs) {
		return slot.unavailable()
	}
	return slot.routes.agentInputs.MarkAgentInputConsumed(ctx, binding)
}
func (slot *productVaultRouteSlot) DeleteAgentInput(ctx context.Context, binding agentinbox.Binding) error {
	if slot == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentInputs) {
		return slot.unavailable()
	}
	return slot.routes.agentInputs.DeleteAgentInput(ctx, binding)
}
func (slot *productVaultRouteSlot) PutAgentCheckpoint(ctx context.Context, payload agentcheckpoint.Payload) error {
	if slot == nil {
		payload.Close()
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentCheckpoints) {
		payload.Close()
		return slot.unavailable()
	}
	return slot.routes.agentCheckpoints.PutAgentCheckpoint(ctx, payload)
}
func (slot *productVaultRouteSlot) ReadAgentCheckpoint(ctx context.Context, binding agentcheckpoint.Binding) (agentcheckpoint.Payload, error) {
	if slot == nil {
		return agentcheckpoint.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentCheckpoints) {
		return agentcheckpoint.Payload{}, slot.unavailable()
	}
	return slot.routes.agentCheckpoints.ReadAgentCheckpoint(ctx, binding)
}
func (slot *productVaultRouteSlot) ResolveAgentCheckpoint(ctx context.Context, query agentcheckpoint.Query) (agentcheckpoint.Payload, error) {
	if slot == nil {
		return agentcheckpoint.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentCheckpoints) {
		return agentcheckpoint.Payload{}, slot.unavailable()
	}
	return slot.routes.agentCheckpoints.ResolveAgentCheckpoint(ctx, query)
}
func (slot *productVaultRouteSlot) DeleteAgentCheckpoint(ctx context.Context, binding agentcheckpoint.Binding) error {
	if slot == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.agentCheckpoints) {
		return slot.unavailable()
	}
	return slot.routes.agentCheckpoints.DeleteAgentCheckpoint(ctx, binding)
}
func (slot *productVaultRouteSlot) PutToolProposal(ctx context.Context, record toolproposal.Record) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.toolProposals) {
		record.Close()
		return slot.unavailable()
	}
	return slot.routes.toolProposals.PutToolProposal(ctx, record)
}
func (slot *productVaultRouteSlot) ReadToolProposal(ctx context.Context, binding toolproposal.Binding) (toolproposal.Record, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.toolProposals) {
		return toolproposal.Record{}, slot.unavailable()
	}
	return slot.routes.toolProposals.ReadToolProposal(ctx, binding)
}
func (slot *productVaultRouteSlot) LookupToolProposal(ctx context.Context, lookup toolproposal.ApprovalLookup) (toolproposal.Record, error) {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.toolProposals) {
		return toolproposal.Record{}, slot.unavailable()
	}
	return slot.routes.toolProposals.LookupToolProposal(ctx, lookup)
}
func (slot *productVaultRouteSlot) DeleteToolProposal(ctx context.Context, binding toolproposal.Binding) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.toolProposals) {
		return slot.unavailable()
	}
	return slot.routes.toolProposals.DeleteToolProposal(ctx, binding)
}
func (slot *productVaultRouteSlot) PutExternalSessionHandle(ctx context.Context, handle credentialvault.ExternalSessionHandle) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.externalSessions) {
		clearProductCredentialBytes(handle.Value)
		return slot.unavailable()
	}
	return slot.routes.externalSessions.PutExternalSessionHandle(ctx, handle)
}
func (slot *productVaultRouteSlot) UseExternalSessionHandle(ctx context.Context, binding credentialvault.ExternalSessionHandleBinding, kind string, use func(context.Context, []byte, int64) error) error {
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.routes.externalSessions) {
		return slot.unavailable()
	}
	return slot.routes.externalSessions.UseExternalSessionHandle(ctx, binding, kind, use)
}

var _ productCredentialVaultController = (*productVaultRouteSlot)(nil)
var _ productCredentialLeaseAccess = (*productVaultRouteSlot)(nil)
var _ productCredentialAvailability = (*productVaultRouteSlot)(nil)
var _ app.CredentialMutator = (*productVaultRouteSlot)(nil)
var _ app.CredentialVaultStatusSource = (*productVaultRouteSlot)(nil)
var _ app.MissionContextCapsuleStore = (*productVaultRouteSlot)(nil)
var _ api.LocalProductConversationContextCapsuleStore = (*productVaultRouteSlot)(nil)
var _ api.LocalProductChatDocumentStore = (*productVaultRouteSlot)(nil)
var _ contextcapsule.RetrievalStore = (*productVaultRouteSlot)(nil)
var _ attemptpayload.Store = (*productVaultRouteSlot)(nil)
var _ toolproposal.Store = (*productVaultRouteSlot)(nil)
var _ productExternalSessionHandleAccess = (*productVaultRouteSlot)(nil)
