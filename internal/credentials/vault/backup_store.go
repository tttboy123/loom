package vault

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"loom-pi-rebuild/internal/credentials"
)

func (store *VaultStore) ExportEncryptedBackup(
	ctx context.Context,
	passphrase []byte,
) (BackupArtifact, error) {
	defer clearBytes(passphrase)
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validBackupPassphrase(passphrase) {
		return BackupArtifact{}, ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return BackupArtifact{}, credentials.ErrCredentialStoreUnavailable
	}
	transaction, err := store.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return BackupArtifact{}, credentials.ErrCredentialStoreUnavailable
	}
	defer transaction.Rollback()
	var pending int
	if err := transaction.QueryRowContext(
		ctx, `SELECT COUNT(*) FROM pending_credential_mutations`,
	).Scan(&pending); err != nil || pending != 0 {
		return BackupArtifact{}, credentials.ErrCredentialMetadataConflict
	}
	records, err := readAllCredentialRecordsTx(ctx, transaction)
	if err != nil || len(records) > maximumBackupRows {
		return BackupArtifact{}, credentials.ErrCredentialStoreUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return BackupArtifact{}, credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(vmk)
	artifact, err := buildEncryptedBackup(passphrase, vmk, records, store.cipher.random)
	if err != nil {
		return BackupArtifact{}, credentials.ErrCredentialStoreUnavailable
	}
	digest := sha256.Sum256(artifact.Data)
	artifact.Digest = hex.EncodeToString(digest[:])
	return artifact, nil
}
