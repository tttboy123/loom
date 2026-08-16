package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/toolproposal"
)

type productCredentialVaultRuntime struct {
	mu           sync.Mutex
	store        *credentialvault.VaultStore
	leasing      *credentialvault.CredentialLeaseManager
	access       productCredentialLeaseAccess
	coordinator  *credentialvault.CredentialVaultCoordinator
	databasePath string
	keyPath      string
	pendingPath  string
	journalStore *journal.Store
	readModel    *projection.Projection
	failure      error
	locked       bool
	closed       bool
}

var _ app.MissionContextCapsuleStore = (*productCredentialVaultRuntime)(nil)
var _ api.LocalProductConversationContextCapsuleStore = (*productCredentialVaultRuntime)(nil)
var _ contextcapsule.RetrievalStore = (*productCredentialVaultRuntime)(nil)
var _ attemptpayload.Store = (*productCredentialVaultRuntime)(nil)
var _ agentinbox.Store = (*productCredentialVaultRuntime)(nil)
var _ agentcheckpoint.Store = (*productCredentialVaultRuntime)(nil)
var _ api.LocalProductChatDocumentStore = (*productCredentialVaultRuntime)(nil)
var _ productExternalSessionHandleAccess = (*productCredentialVaultRuntime)(nil)

type productCredentialVaultController interface {
	RotateCredentialVault(context.Context) error
	LockCredentialVault(context.Context) error
	UnlockCredentialVault(context.Context) error
	ResetCredentialVault(context.Context, string) error
	ExportCredentialVault(context.Context, []byte, string) (productCredentialVaultExportResult, error)
}

const credentialVaultResetConfirmation = "reset_recovery_vault"

type productCredentialVaultExportResult struct {
	SchemaVersion   int    `json:"schema_version"`
	FilePath        string `json:"file_path"`
	Digest          string `json:"digest"`
	CredentialCount int    `json:"credential_count"`
}

func newProductCredentialVaultRuntime(
	ctx context.Context,
	statePath string,
	journalStore *journal.Store,
	readModel *projection.Projection,
) (*productCredentialVaultRuntime, error) {
	if ctx == nil || ctx.Err() != nil || journalStore == nil || readModel == nil ||
		!filepath.IsAbs(statePath) || filepath.Clean(statePath) != statePath {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	databasePath, keyPath, pendingKeyPath, err := productCredentialVaultPaths(statePath)
	if err != nil {
		return nil, err
	}
	privateDirectory := filepath.Dir(keyPath)
	if err := ensureProductCredentialVaultDirectory(privateDirectory); err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen, err,
		)
	}
	if err := credentialvault.RecoverLocalKeyRotation(
		ctx, databasePath, keyPath, pendingKeyPath,
	); err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultRotation,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(ctx)
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultKeyLoad,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	vaultStore, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath,
		KeyMaterial:  material,
	})
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	leasing, err := credentialvault.NewCredentialLeaseManager(vaultStore)
	if err != nil {
		_ = vaultStore.Close()
		return nil, err
	}
	access, err := newProductVaultCredentialLeaseAccess(leasing)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return nil, err
	}
	writer, err := state.NewLocalProductSetupWriter(journalStore)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return nil, err
	}
	verifier, err := provider.NewSystemCatalogCredentialVerifier(5*time.Second, 64*1024)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return nil, err
	}
	coordinator, err := credentialvault.NewCredentialVaultCoordinator(
		credentialvault.CredentialVaultCoordinatorConfig{
			Store: vaultStore, Leases: leasing, Verifier: verifier, Committer: writer,
		},
	)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return nil, err
	}
	runtime := &productCredentialVaultRuntime{
		store: vaultStore, leasing: leasing, access: access, coordinator: coordinator,
		databasePath: databasePath, keyPath: keyPath, pendingPath: pendingKeyPath,
		journalStore: journalStore, readModel: readModel,
	}
	if err := runtime.reconcilePending(ctx, readModel); err != nil {
		_ = runtime.Close()
		return nil, err
	}
	return runtime, nil
}

func productCredentialVaultPaths(statePath string) (string, string, string, error) {
	if !filepath.IsAbs(statePath) || filepath.Clean(statePath) != statePath {
		return "", "", "", credentials.ErrCredentialStoreUnavailable
	}
	stateDirectory := filepath.Dir(statePath)
	if validateProductCredentialVaultDirectory(stateDirectory) != nil {
		return "", "", "", credentials.ErrCredentialStoreUnavailable
	}
	privateDirectory := filepath.Join(stateDirectory, "private")
	if filepath.Base(stateDirectory) == "state" {
		privateDirectory = filepath.Join(filepath.Dir(stateDirectory), "private")
	}
	return filepath.Join(stateDirectory, "credential-vault.db"),
		filepath.Join(privateDirectory, "vault.key"),
		filepath.Join(privateDirectory, "vault.key.rotation-pending"), nil
}

func (runtime *productCredentialVaultRuntime) reconcilePending(
	ctx context.Context,
	readModel *projection.Projection,
) error {
	if runtime == nil || runtime.store == nil || runtime.coordinator == nil ||
		readModel == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	pending, err := runtime.store.PendingCredentialMutations(ctx)
	if err != nil {
		return err
	}
	for _, receipt := range pending {
		record, found := readModel.GlobalReadView().ProviderAccountCredential(
			receipt.Candidate.ProviderID, receipt.Candidate.ProviderAccountID,
		)
		metadata := credentials.MetadataResult{}
		if found {
			metadata = credentials.MetadataResult{
				ProviderID: record.ProviderID, ProviderAccountID: record.ProviderAccountID,
				CredentialReference: record.CredentialReference,
				Revision:            record.Revision, Status: credentials.CredentialStatus(record.Status),
				Reason: credentials.VerificationReason(record.Reason),
			}
		}
		if err := runtime.coordinator.ReconcileCredentialMutation(
			ctx, receipt, metadata, found,
		); err != nil && !errors.Is(err, credentials.ErrCredentialMetadataConflict) {
			return err
		}
	}
	return nil
}

func (runtime *productCredentialVaultRuntime) Close() error {
	if runtime == nil {
		return nil
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.closed = true
	runtime.locked = true
	return runtime.closeComponentsLocked()
}

func (runtime *productCredentialVaultRuntime) closeComponentsLocked() error {
	if runtime == nil {
		return nil
	}
	var result error
	if runtime.leasing != nil {
		result = errors.Join(result, runtime.leasing.Close())
		runtime.leasing = nil
	}
	runtime.access = nil
	runtime.coordinator = nil
	if runtime.store != nil {
		result = errors.Join(result, runtime.store.Close())
		runtime.store = nil
	}
	return result
}

func (runtime *productCredentialVaultRuntime) CredentialAvailable(
	ctx context.Context,
	providerID,
	providerAccountID,
	credentialReference string,
	credentialRevision int64,
) (bool, error) {
	if runtime == nil {
		return false, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.store == nil {
		return false, runtime.safeFailure()
	}
	return runtime.store.HasCredential(ctx, credentialvault.CredentialIdentity{
		ProviderID: providerID, ProviderAccountID: providerAccountID,
		CredentialReference: credentialReference,
		CredentialRevision:  credentialRevision,
	})
}

func (runtime *productCredentialVaultRuntime) PutRoleContextCapsule(
	ctx context.Context,
	capsule contextcapsule.RoleContextCapsule,
	dispatchPayload []byte,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.PutRoleContextCapsule(ctx, capsule, dispatchPayload)
}

func (runtime *productCredentialVaultRuntime) DeleteRoleContextCapsule(
	ctx context.Context,
	authority contextcapsule.AuthorityRecord,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.DeleteRoleContextCapsule(ctx, authority)
}

func (runtime *productCredentialVaultRuntime) ReadRoleContextCapsule(
	ctx context.Context,
	authority contextcapsule.AuthorityRecord,
) (contextcapsule.RoleContextCapsule, []byte, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return contextcapsule.RoleContextCapsule{}, nil,
			credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return contextcapsule.RoleContextCapsule{}, nil, runtime.safeFailure()
	}
	stored, err := runtime.store.ReadContextCapsule(
		ctx, authority.ConversationID, authority.CapsuleDigest,
	)
	if err != nil || !stored.CapsuleAvailable || stored.Authority != authority {
		return contextcapsule.RoleContextCapsule{}, nil,
			errors.Join(credentials.ErrCredentialStoreUnavailable, err)
	}
	return stored.Capsule, stored.DispatchPayload, nil
}

func (runtime *productCredentialVaultRuntime) ListRoleContextCapsuleAuthorities(
	ctx context.Context,
	conversationID string,
) ([]contextcapsule.AuthorityRecord, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return nil, runtime.safeFailure()
	}
	return runtime.store.ListContextCapsuleAuthorities(ctx, conversationID)
}

func (runtime *productCredentialVaultRuntime) RetrieveContextItem(
	ctx context.Context,
	request contextcapsule.RetrievalRequest,
) (contextcapsule.RetrievedItem, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return contextcapsule.RetrievedItem{}, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return contextcapsule.RetrievedItem{}, runtime.safeFailure()
	}
	return runtime.store.RetrieveContextItem(ctx, request)
}

func (runtime *productCredentialVaultRuntime) PutAttemptPayload(
	ctx context.Context,
	payload attemptpayload.Payload,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.PutAttemptPayload(ctx, payload)
}

func (runtime *productCredentialVaultRuntime) ReadAttemptPayload(
	ctx context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return attemptpayload.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return attemptpayload.Payload{}, runtime.safeFailure()
	}
	return runtime.store.ReadAttemptPayload(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) ListPendingAttemptPayloads(
	ctx context.Context,
	scope attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return nil, runtime.safeFailure()
	}
	return runtime.store.ListPendingAttemptPayloads(ctx, scope)
}

func (runtime *productCredentialVaultRuntime) MarkAttemptPayloadDelivered(
	ctx context.Context,
	binding attemptpayload.Binding,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.MarkAttemptPayloadDelivered(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) PutAgentInput(
	ctx context.Context,
	payload agentinbox.Payload,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		payload.Close()
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		payload.Close()
		return runtime.safeFailure()
	}
	return runtime.store.PutAgentInput(ctx, payload)
}

func (runtime *productCredentialVaultRuntime) ReadAgentInput(
	ctx context.Context,
	binding agentinbox.Binding,
) (agentinbox.Payload, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return agentinbox.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return agentinbox.Payload{}, runtime.safeFailure()
	}
	return runtime.store.ReadAgentInput(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) ListPendingAgentInputs(
	ctx context.Context,
	conversationID, runID, agentInstanceID string,
	claimGeneration int64,
) ([]agentinbox.Payload, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return nil, runtime.safeFailure()
	}
	return runtime.store.ListPendingAgentInputs(
		ctx, conversationID, runID, agentInstanceID, claimGeneration,
	)
}

func (runtime *productCredentialVaultRuntime) MarkAgentInputConsumed(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.MarkAgentInputConsumed(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) DeleteAgentInput(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.DeleteAgentInput(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) PutAgentCheckpoint(
	ctx context.Context,
	payload agentcheckpoint.Payload,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		payload.Close()
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		payload.Close()
		return runtime.safeFailure()
	}
	return runtime.store.PutAgentCheckpoint(ctx, payload)
}

func (runtime *productCredentialVaultRuntime) ReadAgentCheckpoint(
	ctx context.Context,
	binding agentcheckpoint.Binding,
) (agentcheckpoint.Payload, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return agentcheckpoint.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return agentcheckpoint.Payload{}, runtime.safeFailure()
	}
	return runtime.store.ReadAgentCheckpoint(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) ResolveAgentCheckpoint(
	ctx context.Context,
	query agentcheckpoint.Query,
) (agentcheckpoint.Payload, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return agentcheckpoint.Payload{}, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return agentcheckpoint.Payload{}, runtime.safeFailure()
	}
	return runtime.store.ResolveAgentCheckpoint(ctx, query)
}

func (runtime *productCredentialVaultRuntime) DeleteAgentCheckpoint(
	ctx context.Context,
	binding agentcheckpoint.Binding,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed || runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.DeleteAgentCheckpoint(ctx, binding)
}

func (runtime *productCredentialVaultRuntime) ConversationDocuments(
	ctx context.Context,
	kind string,
) ([]api.LocalProductChatDocument, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return nil, runtime.safeFailure()
	}
	documents, err := runtime.store.ConversationDocuments(ctx, kind)
	if err != nil {
		return nil, err
	}
	result := make([]api.LocalProductChatDocument, 0, len(documents))
	for index := range documents {
		result = append(result, api.LocalProductChatDocument{
			ConversationID: documents[index].ConversationID,
			Kind:           documents[index].Kind,
			Revision:       documents[index].Revision,
			Payload:        append([]byte(nil), documents[index].Payload...),
		})
		clearProductCredentialBytes(documents[index].Payload)
	}
	return result, nil
}

func (runtime *productCredentialVaultRuntime) PutConversationDocument(
	ctx context.Context,
	document api.LocalProductChatDocument,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.PutConversationDocument(
		ctx,
		credentialvault.ConversationDocument{
			ConversationID: document.ConversationID,
			Kind:           document.Kind,
			Revision:       document.Revision,
			Payload:        document.Payload,
		},
	)
}

func (runtime *productCredentialVaultRuntime) PutExternalSessionHandle(
	ctx context.Context,
	handle credentialvault.ExternalSessionHandle,
) error {
	defer clearProductCredentialBytes(handle.Value)
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		return runtime.safeFailure()
	}
	return runtime.store.PutExternalSessionHandle(ctx, handle)
}

func (runtime *productCredentialVaultRuntime) UseExternalSessionHandle(
	ctx context.Context,
	binding credentialvault.ExternalSessionHandleBinding,
	kind string,
	use func(context.Context, []byte, int64) error,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil || use == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	if runtime.failure != nil || runtime.locked || runtime.closed ||
		runtime.store == nil {
		err := runtime.safeFailure()
		runtime.mu.Unlock()
		return err
	}
	handle, err := runtime.store.ReadExternalSessionHandle(ctx, binding, kind)
	runtime.mu.Unlock()
	if err != nil {
		return err
	}
	defer clearProductCredentialBytes(handle.Value)
	return use(ctx, handle.Value, handle.Revision)
}

func clearProductCredentialBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func (runtime *productCredentialVaultRuntime) CredentialVaultStatus(
	ctx context.Context,
) (app.CredentialVaultStatus, error) {
	if runtime != nil {
		runtime.mu.Lock()
		defer runtime.mu.Unlock()
	}
	status := "unlocked"
	if runtime != nil && runtime.locked && runtime.failure == nil {
		status = "locked"
	} else if runtime == nil || runtime.failure != nil || runtime.store == nil ||
		runtime.store.Health(ctx) != nil {
		status = "recovery_required"
	}
	return app.CredentialVaultStatus{
		SchemaVersion: 1,
		Status:        status,
		StorageMode:   "local_key_file",
	}, nil
}

func (runtime *productCredentialVaultRuntime) LockCredentialVault(
	ctx context.Context,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.closed {
		return runtime.safeFailure()
	}
	if runtime.locked {
		return nil
	}
	if err := runtime.closeComponentsLocked(); err != nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.locked = true
	return nil
}

func (runtime *productCredentialVaultRuntime) UnlockCredentialVault(
	ctx context.Context,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.closed {
		return runtime.safeFailure()
	}
	if !runtime.locked {
		return nil
	}
	if err := credentialvault.RecoverLocalKeyRotation(
		ctx, runtime.databasePath, runtime.keyPath, runtime.pendingPath,
	); err != nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultRotation,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	material, err := (credentialvault.LocalKeyFile{
		Path: runtime.keyPath,
	}).LoadOrCreate(ctx)
	if err != nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultKeyLoad,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	vaultStore, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: runtime.databasePath,
		KeyMaterial:  material,
	})
	if err != nil {
		material.Close()
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	leasing, err := credentialvault.NewCredentialLeaseManager(vaultStore)
	if err != nil {
		_ = vaultStore.Close()
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	access, err := newProductVaultCredentialLeaseAccess(leasing)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	writer, err := state.NewLocalProductSetupWriter(runtime.journalStore)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	verifier, err := provider.NewSystemCatalogCredentialVerifier(
		5*time.Second, 64*1024,
	)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	coordinator, err := credentialvault.NewCredentialVaultCoordinator(
		credentialvault.CredentialVaultCoordinatorConfig{
			Store: vaultStore, Leases: leasing, Verifier: verifier, Committer: writer,
		},
	)
	if err != nil {
		_ = leasing.Close()
		_ = vaultStore.Close()
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.store = vaultStore
	runtime.leasing = leasing
	runtime.access = access
	runtime.coordinator = coordinator
	if err := runtime.reconcilePending(ctx, runtime.readModel); err != nil {
		_ = runtime.closeComponentsLocked()
		return err
	}
	runtime.locked = false
	return nil
}

func (runtime *productCredentialVaultRuntime) RotateCredentialVault(
	ctx context.Context,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultRotation,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.store == nil ||
		runtime.leasing == nil {
		return runtime.safeFailure()
	}
	version, _, err := runtime.store.KeyIdentity(ctx)
	if err != nil || version == ^uint32(0) {
		return runtime.recordRotationFailure(err)
	}
	material, err := (credentialvault.LocalKeyFile{
		Path: runtime.pendingPath,
	}).Create(ctx, version+1)
	if err != nil {
		return runtime.recordRotationFailure(err)
	}
	transferred := false
	err = runtime.leasing.WithRotationBarrier(ctx, func() error {
		if rotateErr := runtime.store.RotateWrappingKey(ctx, material); rotateErr != nil {
			return rotateErr
		}
		transferred = true
		if recoverErr := credentialvault.RecoverLocalKeyRotation(
			ctx, runtime.databasePath, runtime.keyPath, runtime.pendingPath,
		); recoverErr != nil {
			return recoverErr
		}
		canonical, loadErr := (credentialvault.LocalKeyFile{
			Path: runtime.keyPath,
		}).LoadOrCreate(ctx)
		if loadErr != nil {
			return loadErr
		}
		if adoptErr := runtime.store.AdoptRotatedKeyMaterial(
			ctx, canonical,
		); adoptErr != nil {
			canonical.Close()
			return adoptErr
		}
		return nil
	})
	if err == nil {
		return nil
	}
	if !transferred {
		material.Close()
		cleanupContext, cancelCleanup := context.WithTimeout(
			context.WithoutCancel(ctx), 2*time.Second,
		)
		defer cancelCleanup()
		_ = credentialvault.RecoverLocalKeyRotation(
			cleanupContext, runtime.databasePath,
			runtime.keyPath, runtime.pendingPath,
		)
	}
	return runtime.recordRotationFailure(err)
}

func (*productCredentialVaultRuntime) ResetCredentialVault(
	context.Context,
	string,
) error {
	return credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultRecovery,
		credentials.ErrCredentialStoreDenied,
	)
}

func (runtime *productCredentialVaultRuntime) ExportCredentialVault(
	ctx context.Context,
	passphrase []byte,
	destination string,
) (productCredentialVaultExportResult, error) {
	if runtime == nil || ctx == nil || ctx.Err() != nil {
		clearProductCredentialLeaseSecret(passphrase)
		return productCredentialVaultExportResult{}, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultExport,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.store == nil {
		clearProductCredentialLeaseSecret(passphrase)
		return productCredentialVaultExportResult{}, runtime.safeFailure()
	}
	artifact, err := runtime.store.ExportEncryptedBackup(ctx, passphrase)
	if err != nil {
		return productCredentialVaultExportResult{}, runtime.exportFailure(err)
	}
	defer clearProductCredentialLeaseSecret(artifact.Data)
	if err := credentialvault.PublishEncryptedBackup(
		ctx, destination, artifact.Data,
	); err != nil {
		return productCredentialVaultExportResult{}, runtime.exportFailure(err)
	}
	return productCredentialVaultExportResult{
		SchemaVersion: 1, FilePath: destination, Digest: artifact.Digest,
		CredentialCount: artifact.CredentialCount,
	}, nil
}

func (runtime *productCredentialVaultRuntime) UseCredential(
	ctx context.Context,
	identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	if runtime == nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	if runtime.failure != nil || runtime.locked || runtime.access == nil {
		err := runtime.safeFailure()
		runtime.mu.Unlock()
		return err
	}
	access := runtime.access
	runtime.mu.Unlock()
	return access.UseCredential(ctx, identity, use)
}

func (runtime *productCredentialVaultRuntime) Configure(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return runtime.mutate(ctx, command, "configure")
}

func (runtime *productCredentialVaultRuntime) Verify(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return runtime.mutate(ctx, command, "verify")
}

func (runtime *productCredentialVaultRuntime) Replace(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return runtime.mutate(ctx, command, "replace")
}

func (runtime *productCredentialVaultRuntime) Revoke(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return runtime.mutate(ctx, command, "revoke")
}

func (runtime *productCredentialVaultRuntime) mutate(
	ctx context.Context,
	command credentials.CredentialCommand,
	operation string,
) (credentials.MetadataResult, error) {
	if runtime == nil {
		clearProductCredentialLeaseSecret(command.Secret)
		return credentials.MetadataResult{}, credentials.ErrCredentialStoreUnavailable
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.failure != nil || runtime.locked || runtime.coordinator == nil {
		clearProductCredentialLeaseSecret(command.Secret)
		return credentials.MetadataResult{}, runtime.safeFailure()
	}
	switch operation {
	case "configure":
		return runtime.coordinator.Configure(ctx, command)
	case "verify":
		return runtime.coordinator.Verify(ctx, command)
	case "replace":
		return runtime.coordinator.Replace(ctx, command)
	case "revoke":
		return runtime.coordinator.Revoke(ctx, command)
	default:
		clearProductCredentialLeaseSecret(command.Secret)
		return credentials.MetadataResult{}, credentials.ErrInvalidCredentialCommand
	}
}

func (runtime *productCredentialVaultRuntime) recordRotationFailure(err error) error {
	failure := credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultRotation,
		credentials.ErrCredentialStoreUnavailable,
	)
	runtime.failure = failure
	return failure
}

func (runtime *productCredentialVaultRuntime) safeFailure() error {
	if runtime != nil && runtime.failure != nil {
		return runtime.failure
	}
	return credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultOpen,
		credentials.ErrCredentialStoreUnavailable,
	)
}

func (*productCredentialVaultRuntime) exportFailure(error) error {
	return credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultExport,
		credentials.ErrCredentialStoreUnavailable,
	)
}

type productCredentialVaultRecoveryRuntime struct {
	mu           sync.Mutex
	failure      error
	statePath    string
	journalStore *journal.Store
	readModel    *projection.Projection
	active       *productCredentialVaultRuntime
	closed       bool
}

func newProductCredentialVaultRecoveryRuntime(
	failure error,
	statePath string,
	journalStore *journal.Store,
	readModel *projection.Projection,
) *productCredentialVaultRecoveryRuntime {
	return &productCredentialVaultRecoveryRuntime{
		failure: failure, statePath: statePath,
		journalStore: journalStore, readModel: readModel,
	}
}

func (runtime *productCredentialVaultRecoveryRuntime) Close() error {
	if runtime == nil {
		return nil
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.closed = true
	if runtime.active != nil {
		return runtime.active.Close()
	}
	return nil
}

func (runtime *productCredentialVaultRecoveryRuntime) RotateCredentialVault(
	ctx context.Context,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.RotateCredentialVault(ctx)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) LockCredentialVault(
	ctx context.Context,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.LockCredentialVault(ctx)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) UnlockCredentialVault(
	ctx context.Context,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.UnlockCredentialVault(ctx)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ResetCredentialVault(
	ctx context.Context,
	confirmation string,
) error {
	if runtime == nil || ctx == nil || ctx.Err() != nil ||
		confirmation != credentialVaultResetConfirmation {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultRecovery,
			credentials.ErrCredentialStoreDenied,
		)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.closed || runtime.active != nil || runtime.failure == nil ||
		runtime.journalStore == nil || runtime.readModel == nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultRecovery,
			credentials.ErrCredentialStoreDenied,
		)
	}
	databasePath, keyPath, pendingPath, err := productCredentialVaultPaths(
		runtime.statePath,
	)
	if err != nil {
		return runtime.recoveryFailure(err)
	}
	if err := credentialvault.ResetLocalVault(
		ctx, databasePath, keyPath, pendingPath,
	); err != nil {
		return runtime.recoveryFailure(err)
	}
	active, err := newProductCredentialVaultRuntime(
		ctx, runtime.statePath, runtime.journalStore, runtime.readModel,
	)
	if err != nil {
		return runtime.recoveryFailure(err)
	}
	runtime.active = active
	runtime.failure = nil
	return nil
}

func (runtime *productCredentialVaultRecoveryRuntime) ExportCredentialVault(
	ctx context.Context,
	passphrase []byte,
	destination string,
) (productCredentialVaultExportResult, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ExportCredentialVault(ctx, passphrase, destination)
	}
	clearProductCredentialLeaseSecret(passphrase)
	return productCredentialVaultExportResult{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) CredentialVaultStatus(
	ctx context.Context,
) (app.CredentialVaultStatus, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.CredentialVaultStatus(ctx)
	}
	return app.CredentialVaultStatus{
		SchemaVersion: 1,
		Status:        "recovery_required",
		StorageMode:   "local_key_file",
	}, nil
}

func (runtime *productCredentialVaultRecoveryRuntime) CredentialAvailable(
	ctx context.Context,
	providerID string,
	providerAccountID string,
	credentialReference string,
	credentialRevision int64,
) (bool, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.CredentialAvailable(
			ctx, providerID, providerAccountID,
			credentialReference, credentialRevision,
		)
	}
	return false, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) UseCredential(
	ctx context.Context,
	identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.UseCredential(ctx, identity, use)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) Configure(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.Configure(ctx, command)
	}
	clearProductCredentialLeaseSecret(command.Secret)
	return credentials.MetadataResult{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) Verify(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.Verify(ctx, command)
	}
	clearProductCredentialLeaseSecret(command.Secret)
	return credentials.MetadataResult{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) Replace(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.Replace(ctx, command)
	}
	clearProductCredentialLeaseSecret(command.Secret)
	return credentials.MetadataResult{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) Revoke(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.Revoke(ctx, command)
	}
	clearProductCredentialLeaseSecret(command.Secret)
	return credentials.MetadataResult{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutRoleContextCapsule(
	ctx context.Context, capsule contextcapsule.RoleContextCapsule, payload []byte,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.PutRoleContextCapsule(ctx, capsule, payload)
	}
	clearProductCredentialBytes(payload)
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) DeleteRoleContextCapsule(
	ctx context.Context, authority contextcapsule.AuthorityRecord,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.DeleteRoleContextCapsule(ctx, authority)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ReadRoleContextCapsule(
	ctx context.Context, authority contextcapsule.AuthorityRecord,
) (contextcapsule.RoleContextCapsule, []byte, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ReadRoleContextCapsule(ctx, authority)
	}
	return contextcapsule.RoleContextCapsule{}, nil, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ListRoleContextCapsuleAuthorities(
	ctx context.Context, conversationID string,
) ([]contextcapsule.AuthorityRecord, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ListRoleContextCapsuleAuthorities(ctx, conversationID)
	}
	return nil, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) RetrieveContextItem(
	ctx context.Context, request contextcapsule.RetrievalRequest,
) (contextcapsule.RetrievedItem, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.RetrieveContextItem(ctx, request)
	}
	return contextcapsule.RetrievedItem{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutAttemptPayload(
	ctx context.Context, payload attemptpayload.Payload,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.PutAttemptPayload(ctx, payload)
	}
	payload.Close()
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ReadAttemptPayload(
	ctx context.Context, binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ReadAttemptPayload(ctx, binding)
	}
	return attemptpayload.Payload{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ListPendingAttemptPayloads(
	ctx context.Context, scope attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ListPendingAttemptPayloads(ctx, scope)
	}
	return nil, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) MarkAttemptPayloadDelivered(
	ctx context.Context, binding attemptpayload.Binding,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.MarkAttemptPayloadDelivered(ctx, binding)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutAgentInput(
	ctx context.Context,
	payload agentinbox.Payload,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.PutAgentInput(ctx, payload)
	}
	payload.Close()
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ReadAgentInput(
	ctx context.Context,
	binding agentinbox.Binding,
) (agentinbox.Payload, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ReadAgentInput(ctx, binding)
	}
	return agentinbox.Payload{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ListPendingAgentInputs(
	ctx context.Context,
	conversationID, runID, agentInstanceID string,
	claimGeneration int64,
) ([]agentinbox.Payload, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ListPendingAgentInputs(
			ctx, conversationID, runID, agentInstanceID, claimGeneration,
		)
	}
	return nil, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) MarkAgentInputConsumed(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.MarkAgentInputConsumed(ctx, binding)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) DeleteAgentInput(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.DeleteAgentInput(ctx, binding)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutAgentCheckpoint(
	ctx context.Context,
	payload agentcheckpoint.Payload,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.PutAgentCheckpoint(ctx, payload)
	}
	payload.Close()
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ReadAgentCheckpoint(
	ctx context.Context,
	binding agentcheckpoint.Binding,
) (agentcheckpoint.Payload, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ReadAgentCheckpoint(ctx, binding)
	}
	return agentcheckpoint.Payload{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ResolveAgentCheckpoint(
	ctx context.Context,
	query agentcheckpoint.Query,
) (agentcheckpoint.Payload, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ResolveAgentCheckpoint(ctx, query)
	}
	return agentcheckpoint.Payload{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) DeleteAgentCheckpoint(
	ctx context.Context,
	binding agentcheckpoint.Binding,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.DeleteAgentCheckpoint(ctx, binding)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ConversationDocuments(
	ctx context.Context, kind string,
) ([]api.LocalProductChatDocument, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.ConversationDocuments(ctx, kind)
	}
	return nil, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutConversationDocument(
	ctx context.Context, document api.LocalProductChatDocument,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.PutConversationDocument(ctx, document)
	}
	clearProductCredentialBytes(document.Payload)
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutExternalSessionHandle(
	ctx context.Context, handle credentialvault.ExternalSessionHandle,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.PutExternalSessionHandle(ctx, handle)
	}
	clearProductCredentialBytes(handle.Value)
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) UseExternalSessionHandle(
	ctx context.Context, binding credentialvault.ExternalSessionHandleBinding,
	kind string, use func(context.Context, []byte, int64) error,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.UseExternalSessionHandle(ctx, binding, kind, use)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) PutToolProposal(
	ctx context.Context, record toolproposal.Record,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.store.PutToolProposal(ctx, record)
	}
	record.Close()
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) ReadToolProposal(
	ctx context.Context, binding toolproposal.Binding,
) (toolproposal.Record, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.store.ReadToolProposal(ctx, binding)
	}
	return toolproposal.Record{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) LookupToolProposal(
	ctx context.Context, lookup toolproposal.ApprovalLookup,
) (toolproposal.Record, error) {
	if active := runtime.activeRuntime(); active != nil {
		return active.store.LookupToolProposal(ctx, lookup)
	}
	return toolproposal.Record{}, runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) DeleteToolProposal(
	ctx context.Context, binding toolproposal.Binding,
) error {
	if active := runtime.activeRuntime(); active != nil {
		return active.store.DeleteToolProposal(ctx, binding)
	}
	return runtime.safeFailure()
}

func (runtime *productCredentialVaultRecoveryRuntime) safeFailure() error {
	if runtime == nil {
		return credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.safeFailureLocked()
}

func (runtime *productCredentialVaultRecoveryRuntime) safeFailureLocked() error {
	if runtime != nil && runtime.failure != nil &&
		credentials.CredentialFailureStage(runtime.failure) != "" {
		return runtime.failure
	}
	return credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultOpen,
		credentials.ErrCredentialStoreUnavailable,
	)
}

func (runtime *productCredentialVaultRecoveryRuntime) activeRuntime() *productCredentialVaultRuntime {
	if runtime == nil {
		return nil
	}
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.active
}

func (runtime *productCredentialVaultRecoveryRuntime) recoveryFailure(error) error {
	failure := credentials.WithCredentialFailureStage(
		credentials.CredentialStageVaultRecovery,
		credentials.ErrCredentialStoreUnavailable,
	)
	runtime.failure = failure
	return failure
}

func ensureProductCredentialVaultDirectory(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return credentials.ErrCredentialStoreUnavailable
	}
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return credentials.ErrCredentialStoreUnavailable
	}
	return validateProductCredentialVaultDirectory(path)
}

func validateProductCredentialVaultDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() ||
		info.Mode().Perm() != 0o700 {
		return credentials.ErrCredentialStoreUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

var _ io.Closer = (*productCredentialVaultRuntime)(nil)
var _ io.Closer = (*productCredentialVaultRecoveryRuntime)(nil)
var _ app.CredentialMutator = (*productCredentialVaultRecoveryRuntime)(nil)
var _ app.CredentialMutator = (*productCredentialVaultRuntime)(nil)
var _ productCredentialLeaseAccess = (*productCredentialVaultRuntime)(nil)
var _ productCredentialLeaseAccess = (*productCredentialVaultRecoveryRuntime)(nil)
var _ productCredentialAvailability = (*productCredentialVaultRecoveryRuntime)(nil)
var _ productCredentialVaultController = (*productCredentialVaultRecoveryRuntime)(nil)
var _ productCredentialVaultController = (*productCredentialVaultRuntime)(nil)
