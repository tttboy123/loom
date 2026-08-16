package vault

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"loom-pi-rebuild/internal/credentials"
)

type CredentialMutationKind string

const (
	MutationConfigure CredentialMutationKind = "configure"
	MutationImport    CredentialMutationKind = "import"
	MutationRebind    CredentialMutationKind = "rebind"
	MutationReplace   CredentialMutationKind = "replace"
	MutationRevoke    CredentialMutationKind = "revoke"

	vaultFaultPrepareBeforeCommit  = "prepare_before_commit"
	vaultFaultFinalizeBeforeCommit = "finalize_before_commit"
	vaultFaultRollbackBeforeCommit = "rollback_before_commit"
)

type CredentialMutation struct {
	MutationID string
	Kind       CredentialMutationKind
	Current    CredentialIdentity
	Candidate  CredentialIdentity
	Secret     []byte
}

type CredentialMutationReceipt struct {
	MutationID string
	Kind       CredentialMutationKind
	Current    CredentialIdentity
	Candidate  CredentialIdentity
}

func (store *VaultStore) PrepareCredentialMutation(
	ctx context.Context,
	mutation CredentialMutation,
) (CredentialMutationReceipt, error) {
	defer clearBytes(mutation.Secret)
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialMutation(mutation) {
		return CredentialMutationReceipt{}, ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return CredentialMutationReceipt{}, credentials.ErrCredentialStoreUnavailable
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return CredentialMutationReceipt{}, credentials.ErrCredentialStoreUnavailable
	}
	defer transaction.Rollback()
	if err := ensureNoPendingMutation(ctx, transaction, mutation.Candidate); err != nil {
		return CredentialMutationReceipt{}, err
	}

	var candidate EncryptedCredential
	switch mutation.Kind {
	case MutationConfigure:
		if err := ensureNoActiveCredential(
			ctx, transaction, mutation.Candidate.CredentialReference,
		); err != nil {
			return CredentialMutationReceipt{}, err
		}
		candidate, err = store.encryptMutationSecret(mutation.Candidate, mutation.Secret)
	case MutationImport:
		if err := ensureNoActiveCredential(
			ctx, transaction, mutation.Candidate.CredentialReference,
		); err != nil {
			return CredentialMutationReceipt{}, err
		}
		candidate, err = store.encryptMutationSecret(mutation.Candidate, mutation.Secret)
	case MutationReplace:
		if _, readErr := readCredentialRecordTx(ctx, transaction, mutation.Current); readErr != nil {
			return CredentialMutationReceipt{}, readErr
		}
		candidate, err = store.encryptMutationSecret(mutation.Candidate, mutation.Secret)
	case MutationRebind:
		current, readErr := readCredentialRecordTx(ctx, transaction, mutation.Current)
		if readErr != nil {
			return CredentialMutationReceipt{}, readErr
		}
		candidate, err = store.rebindCredential(current, mutation.Candidate)
	case MutationRevoke:
		_, err = readCredentialRecordTx(ctx, transaction, mutation.Current)
	default:
		err = ErrInvalidVaultInput
	}
	if err != nil {
		return CredentialMutationReceipt{}, err
	}
	if err := insertPendingMutation(ctx, transaction, mutation, candidate, store.now); err != nil {
		return CredentialMutationReceipt{}, err
	}
	if err := store.injectMutationFault(vaultFaultPrepareBeforeCommit); err != nil {
		return CredentialMutationReceipt{}, err
	}
	if err := transaction.Commit(); err != nil {
		return CredentialMutationReceipt{}, credentials.ErrCredentialStoreUnavailable
	}
	return receiptForMutation(mutation), nil
}

func (store *VaultStore) PendingCredentialMutations(
	ctx context.Context,
) ([]CredentialMutationReceipt, error) {
	if store == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	rows, err := store.database.QueryContext(
		ctx,
		`SELECT mutation_id, mutation_kind, credential_reference, provider_id,
		        provider_account_id, current_revision, candidate_revision
		   FROM pending_credential_mutations
		  WHERE status = 'pending' ORDER BY mutation_id`,
	)
	if err != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	defer rows.Close()
	receipts := make([]CredentialMutationReceipt, 0)
	for rows.Next() {
		var receipt CredentialMutationReceipt
		var kind string
		var currentRevision int64
		if err := rows.Scan(
			&receipt.MutationID, &kind, &receipt.Candidate.CredentialReference,
			&receipt.Candidate.ProviderID, &receipt.Candidate.ProviderAccountID,
			&currentRevision, &receipt.Candidate.CredentialRevision,
		); err != nil {
			return nil, credentials.ErrCredentialStoreUnavailable
		}
		receipt.Kind = CredentialMutationKind(kind)
		if currentRevision > 0 {
			receipt.Current = receipt.Candidate
			receipt.Current.CredentialRevision = currentRevision
		}
		if !validCredentialMutationReceipt(receipt) {
			return nil, credentials.ErrCredentialStoreUnavailable
		}
		receipts = append(receipts, receipt)
	}
	if err := rows.Err(); err != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	return receipts, nil
}

func (store *VaultStore) CommitCredentialMutation(
	ctx context.Context,
	receipt CredentialMutationReceipt,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialMutationReceipt(receipt) {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer transaction.Rollback()
	pending, candidate, err := readPendingMutation(ctx, transaction, receipt.MutationID)
	if err != nil {
		return err
	}
	if receiptForMutation(pending) != receipt {
		return credentials.ErrCredentialMetadataConflict
	}
	switch pending.Kind {
	case MutationConfigure, MutationImport:
		if err := ensureNoActiveCredential(
			ctx, transaction, pending.Candidate.CredentialReference,
		); err != nil {
			return err
		}
		if err := activateCredentialRecord(ctx, transaction, candidate, store.now); err != nil {
			return err
		}
	case MutationRebind, MutationReplace:
		if _, err := readCredentialRecordTx(ctx, transaction, pending.Current); err != nil {
			return err
		}
		if err := activateCredentialRecord(ctx, transaction, candidate, store.now); err != nil {
			return err
		}
	case MutationRevoke:
		result, err := transaction.ExecContext(
			ctx,
			`DELETE FROM encrypted_credentials
			  WHERE credential_reference = ? AND provider_id = ?
			    AND provider_account_id = ? AND credential_revision = ?`,
			pending.Current.CredentialReference, pending.Current.ProviderID,
			pending.Current.ProviderAccountID, pending.Current.CredentialRevision,
		)
		if err != nil {
			return credentials.ErrCredentialStoreUnavailable
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			return credentials.ErrCredentialMetadataConflict
		}
	default:
		return ErrInvalidVaultInput
	}
	if _, err := transaction.ExecContext(
		ctx, `DELETE FROM pending_credential_mutations WHERE mutation_id = ?`,
		receipt.MutationID,
	); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	if err := store.injectMutationFault(vaultFaultFinalizeBeforeCommit); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (store *VaultStore) RollbackCredentialMutation(
	ctx context.Context,
	receipt CredentialMutationReceipt,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialMutationReceipt(receipt) {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer transaction.Rollback()
	pending, _, err := readPendingMutation(ctx, transaction, receipt.MutationID)
	if err != nil {
		return err
	}
	if receiptForMutation(pending) != receipt {
		return credentials.ErrCredentialMetadataConflict
	}
	result, err := transaction.ExecContext(
		ctx, `DELETE FROM pending_credential_mutations WHERE mutation_id = ?`,
		receipt.MutationID,
	)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return credentials.ErrCredentialMetadataConflict
	}
	if err := store.injectMutationFault(vaultFaultRollbackBeforeCommit); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (store *VaultStore) injectMutationFault(stage string) error {
	if store != nil && store.fault != nil && store.fault(stage) != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (store *VaultStore) encryptMutationSecret(
	identity CredentialIdentity,
	secret []byte,
) (EncryptedCredential, error) {
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(vmk)
	record, err := store.cipher.Encrypt(
		vmk, store.keyMaterial.KeyVersion(), identity, secret,
	)
	if err != nil {
		return EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	return record, nil
}

func (store *VaultStore) rebindCredential(
	current EncryptedCredential,
	candidate CredentialIdentity,
) (EncryptedCredential, error) {
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(vmk)
	secret, err := store.cipher.Decrypt(vmk, current)
	if err != nil {
		return EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(secret)
	record, err := store.cipher.Encrypt(
		vmk, store.keyMaterial.KeyVersion(), candidate, secret,
	)
	if err != nil {
		return EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	return record, nil
}

func ensureNoPendingMutation(
	ctx context.Context,
	transaction *sql.Tx,
	identity CredentialIdentity,
) error {
	var mutationID string
	err := transaction.QueryRowContext(
		ctx,
		`SELECT mutation_id FROM pending_credential_mutations
		  WHERE credential_reference = ?`,
		identity.CredentialReference,
	).Scan(&mutationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return credentials.ErrCredentialMetadataConflict
}

func ensureNoActiveCredential(
	ctx context.Context,
	transaction *sql.Tx,
	reference string,
) error {
	var found string
	err := transaction.QueryRowContext(
		ctx,
		`SELECT credential_reference FROM encrypted_credentials
		  WHERE credential_reference = ?`,
		reference,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return credentials.ErrCredentialMetadataConflict
}

func readCredentialRecordTx(
	ctx context.Context,
	transaction *sql.Tx,
	identity CredentialIdentity,
) (EncryptedCredential, error) {
	record := EncryptedCredential{Identity: identity}
	var schemaVersion, cipherVersion, keyVersion int64
	err := transaction.QueryRowContext(
		ctx,
		`SELECT schema_version, cipher_version, key_version, wrapped_dek,
		        wrap_nonce, ciphertext, data_nonce, aad_digest
		   FROM encrypted_credentials
		  WHERE credential_reference = ? AND provider_id = ?
		    AND provider_account_id = ? AND credential_revision = ?
		    AND status = 'active'`,
		identity.CredentialReference, identity.ProviderID,
		identity.ProviderAccountID, identity.CredentialRevision,
	).Scan(
		&schemaVersion, &cipherVersion, &keyVersion, &record.WrappedDEK,
		&record.WrapNonce, &record.Ciphertext, &record.DataNonce,
		&record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return EncryptedCredential{}, credentials.ErrCredentialNotFound
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 ||
		keyVersion < 0 || keyVersion > 1<<32-1 {
		return EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	record.KeyVersion = uint32(keyVersion)
	return record, nil
}

func insertPendingMutation(
	ctx context.Context,
	transaction *sql.Tx,
	mutation CredentialMutation,
	record EncryptedCredential,
	now func() time.Time,
) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO pending_credential_mutations (
			mutation_id, mutation_kind, credential_reference, provider_id,
			provider_account_id, current_revision, candidate_revision,
			schema_version, cipher_version, key_version, wrapped_dek,
			wrap_nonce, ciphertext, data_nonce, aad_digest, status, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		mutation.MutationID, mutation.Kind, mutation.Candidate.CredentialReference,
		mutation.Candidate.ProviderID, mutation.Candidate.ProviderAccountID,
		mutation.Current.CredentialRevision, mutation.Candidate.CredentialRevision,
		nullableVersion(record.SchemaVersion, mutation.Kind),
		nullableVersion(record.CipherVersion, mutation.Kind),
		nullableKeyVersion(record.KeyVersion, mutation.Kind),
		nullableBytes(record.WrappedDEK), nullableBytes(record.WrapNonce),
		nullableBytes(record.Ciphertext), nullableBytes(record.DataNonce),
		nullableBytes(record.AADDigest), now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func readPendingMutation(
	ctx context.Context,
	transaction *sql.Tx,
	mutationID string,
) (CredentialMutation, EncryptedCredential, error) {
	var mutation CredentialMutation
	var kind string
	var currentRevision, candidateRevision int64
	var schemaVersion, cipherVersion, keyVersion sql.NullInt64
	var wrappedDEK, wrapNonce, ciphertext, dataNonce, aadDigest []byte
	err := transaction.QueryRowContext(
		ctx,
		`SELECT mutation_id, mutation_kind, credential_reference, provider_id,
		        provider_account_id, current_revision, candidate_revision,
		        schema_version, cipher_version, key_version, wrapped_dek,
		        wrap_nonce, ciphertext, data_nonce, aad_digest
		   FROM pending_credential_mutations
		  WHERE mutation_id = ? AND status = 'pending'`,
		mutationID,
	).Scan(
		&mutation.MutationID, &kind, &mutation.Candidate.CredentialReference,
		&mutation.Candidate.ProviderID, &mutation.Candidate.ProviderAccountID,
		&currentRevision, &candidateRevision, &schemaVersion, &cipherVersion,
		&keyVersion, &wrappedDEK, &wrapNonce, &ciphertext, &dataNonce, &aadDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CredentialMutation{}, EncryptedCredential{}, credentials.ErrCredentialNotFound
	}
	if err != nil || currentRevision < 0 || candidateRevision <= 0 {
		return CredentialMutation{}, EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	mutation.Kind = CredentialMutationKind(kind)
	mutation.Candidate.CredentialRevision = candidateRevision
	if currentRevision > 0 {
		mutation.Current = mutation.Candidate
		mutation.Current.CredentialRevision = currentRevision
	}
	record := EncryptedCredential{
		Identity: mutation.Candidate, WrappedDEK: wrappedDEK, WrapNonce: wrapNonce,
		Ciphertext: ciphertext, DataNonce: dataNonce, AADDigest: aadDigest,
	}
	if mutation.Kind != MutationRevoke {
		if !schemaVersion.Valid || !cipherVersion.Valid || !keyVersion.Valid ||
			schemaVersion.Int64 < 0 || schemaVersion.Int64 > 1<<16-1 ||
			cipherVersion.Int64 < 0 || cipherVersion.Int64 > 1<<16-1 ||
			keyVersion.Int64 <= 0 || keyVersion.Int64 > 1<<32-1 {
			return CredentialMutation{}, EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
		}
		record.SchemaVersion = uint16(schemaVersion.Int64)
		record.CipherVersion = uint16(cipherVersion.Int64)
		record.KeyVersion = uint32(keyVersion.Int64)
	}
	if !validCredentialMutation(CredentialMutation{
		MutationID: mutation.MutationID, Kind: mutation.Kind,
		Current: mutation.Current, Candidate: mutation.Candidate,
		Secret: mutationSecretFixture(mutation.Kind),
	}) {
		return CredentialMutation{}, EncryptedCredential{}, credentials.ErrCredentialStoreUnavailable
	}
	return mutation, record, nil
}

func activateCredentialRecord(
	ctx context.Context,
	transaction *sql.Tx,
	record EncryptedCredential,
	now func() time.Time,
) error {
	timestamp := now().UTC().Format(time.RFC3339Nano)
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_credentials (
			credential_reference, provider_id, provider_account_id,
			credential_revision, schema_version, cipher_version, key_version,
			wrapped_dek, wrap_nonce, ciphertext, data_nonce, aad_digest,
			status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
		ON CONFLICT(credential_reference) DO UPDATE SET
			provider_id=excluded.provider_id,
			provider_account_id=excluded.provider_account_id,
			credential_revision=excluded.credential_revision,
			schema_version=excluded.schema_version,
			cipher_version=excluded.cipher_version,
			key_version=excluded.key_version,
			wrapped_dek=excluded.wrapped_dek,
			wrap_nonce=excluded.wrap_nonce,
			ciphertext=excluded.ciphertext,
			data_nonce=excluded.data_nonce,
			aad_digest=excluded.aad_digest,
			status='active', updated_at=excluded.updated_at`,
		record.Identity.CredentialReference, record.Identity.ProviderID,
		record.Identity.ProviderAccountID, record.Identity.CredentialRevision,
		record.SchemaVersion, record.CipherVersion, record.KeyVersion,
		record.WrappedDEK, record.WrapNonce, record.Ciphertext, record.DataNonce,
		record.AADDigest, timestamp, timestamp,
	)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func validCredentialMutation(mutation CredentialMutation) bool {
	if !validMutationID(mutation.MutationID) ||
		!validCredentialIdentity(mutation.Candidate) {
		return false
	}
	switch mutation.Kind {
	case MutationConfigure:
		return mutation.Current == (CredentialIdentity{}) &&
			len(mutation.Secret) > 0 && len(mutation.Secret) <= MaximumSecretBytes
	case MutationImport, MutationReplace:
		return matchingMutationIdentities(mutation.Current, mutation.Candidate) &&
			len(mutation.Secret) > 0 && len(mutation.Secret) <= MaximumSecretBytes
	case MutationRebind, MutationRevoke:
		return matchingMutationIdentities(mutation.Current, mutation.Candidate) &&
			len(mutation.Secret) == 0
	default:
		return false
	}
}

func validCredentialMutationReceipt(receipt CredentialMutationReceipt) bool {
	return validCredentialMutation(CredentialMutation{
		MutationID: receipt.MutationID, Kind: receipt.Kind,
		Current: receipt.Current, Candidate: receipt.Candidate,
		Secret: mutationSecretFixture(receipt.Kind),
	})
}

func matchingMutationIdentities(current, candidate CredentialIdentity) bool {
	return validCredentialIdentity(current) &&
		current.CredentialReference == candidate.CredentialReference &&
		current.ProviderID == candidate.ProviderID &&
		current.ProviderAccountID == candidate.ProviderAccountID &&
		candidate.CredentialRevision == current.CredentialRevision+1
}

func validMutationID(value string) bool {
	const prefix = "credential-mutation-"
	if !strings.HasPrefix(value, prefix) || len(value) <= len(prefix) || len(value) > 160 {
		return false
	}
	for _, character := range value[len(prefix):] {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func receiptForMutation(mutation CredentialMutation) CredentialMutationReceipt {
	return CredentialMutationReceipt{
		MutationID: mutation.MutationID, Kind: mutation.Kind,
		Current: mutation.Current, Candidate: mutation.Candidate,
	}
}

func mutationSecretFixture(kind CredentialMutationKind) []byte {
	if kind == MutationConfigure || kind == MutationImport || kind == MutationReplace {
		return []byte{1}
	}
	return nil
}

func nullableVersion(value uint16, kind CredentialMutationKind) any {
	if kind == MutationRevoke {
		return nil
	}
	return int64(value)
}

func nullableKeyVersion(value uint32, kind CredentialMutationKind) any {
	if kind == MutationRevoke {
		return nil
	}
	return int64(value)
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
