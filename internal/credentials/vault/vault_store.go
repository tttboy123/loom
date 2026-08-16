package vault

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"loom-pi-rebuild/internal/credentials"

	_ "modernc.org/sqlite"
)

const (
	vaultDatabaseSchema            = 1
	vaultFaultRotationBeforeCommit = "rotation_before_commit"
)

type IdentityResolver func(
	context.Context,
	string,
) (CredentialIdentity, error)

type StoreConfig struct {
	DatabasePath     string
	KeyMaterial      *KeyMaterial
	IdentityResolver IdentityResolver
	Random           io.Reader
	Now              func() time.Time
}

type VaultStore struct {
	mu               sync.Mutex
	database         *sql.DB
	databasePath     string
	databaseIdentity privateFileIdentity
	keyMaterial      *KeyMaterial
	cipher           *EnvelopeCipher
	identityResolver IdentityResolver
	now              func() time.Time
	fault            func(string) error
	closed           bool
}

func OpenStore(config StoreConfig) (*VaultStore, error) {
	if !validVaultDatabasePath(config.DatabasePath) ||
		config.KeyMaterial == nil || config.KeyMaterial.KeyVersion() == 0 {
		if config.KeyMaterial != nil {
			config.KeyMaterial.Close()
		}
		return nil, ErrInvalidVaultInput
	}
	parent := filepath.Dir(config.DatabasePath)
	if err := validatePrivateDirectory(parent); err != nil {
		config.KeyMaterial.Close()
		return nil, err
	}
	if err := ensurePrivateDatabaseFile(config.DatabasePath); err != nil {
		config.KeyMaterial.Close()
		return nil, err
	}
	databaseIdentity, err := capturePrivateFileIdentity(config.DatabasePath)
	if err != nil {
		config.KeyMaterial.Close()
		return nil, err
	}
	cipher, err := NewEnvelopeCipher(config.Random)
	if err != nil {
		config.KeyMaterial.Close()
		return nil, err
	}
	database, err := sql.Open("sqlite", "file:"+config.DatabasePath)
	if err != nil {
		config.KeyMaterial.Close()
		return nil, ErrVaultUnavailable
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	store := &VaultStore{
		database: database, databasePath: config.DatabasePath,
		databaseIdentity: databaseIdentity,
		keyMaterial:      config.KeyMaterial, cipher: cipher,
		identityResolver: config.IdentityResolver, now: config.Now,
	}
	if store.now == nil {
		store.now = time.Now
	}
	if err := store.initialize(context.Background()); err != nil {
		store.Close()
		return nil, err
	}
	if err := store.validateDatabaseFile(); err != nil {
		store.Close()
		return nil, ErrVaultUnavailable
	}
	return store, nil
}

func (store *VaultStore) Put(
	ctx context.Context,
	reference string,
	secret []byte,
) error {
	identity, err := store.resolveIdentity(ctx, reference)
	if err != nil {
		return err
	}
	return store.PutCredential(ctx, identity, secret)
}

func (store *VaultStore) Read(
	ctx context.Context,
	reference string,
) ([]byte, error) {
	identity, err := store.resolveIdentity(ctx, reference)
	if err != nil {
		return nil, err
	}
	return store.ReadCredential(ctx, identity)
}

func (store *VaultStore) Delete(
	ctx context.Context,
	reference string,
) error {
	identity, err := store.resolveIdentity(ctx, reference)
	if err != nil {
		return err
	}
	return store.DeleteCredential(ctx, identity)
}

func (store *VaultStore) PutCredential(
	ctx context.Context,
	identity CredentialIdentity,
	secret []byte,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialIdentity(identity) || len(secret) == 0 ||
		len(secret) > MaximumSecretBytes {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return credentials.ErrCredentialStoreUnavailable
	}
	if err := store.validateDatabaseFile(); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(vmk)
	record, err := store.cipher.Encrypt(
		vmk, store.keyMaterial.KeyVersion(), identity, secret,
	)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer transaction.Rollback()
	var existingProvider, existingAccount string
	var existingRevision int64
	queryErr := transaction.QueryRowContext(
		ctx,
		`SELECT provider_id, provider_account_id, credential_revision
		   FROM encrypted_credentials WHERE credential_reference = ?`,
		identity.CredentialReference,
	).Scan(&existingProvider, &existingAccount, &existingRevision)
	if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
		return credentials.ErrCredentialStoreUnavailable
	}
	if queryErr == nil && (existingProvider != identity.ProviderID ||
		existingAccount != identity.ProviderAccountID ||
		existingRevision >= identity.CredentialRevision) {
		return credentials.ErrCredentialMetadataConflict
	}
	_, err = transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_credentials (
			credential_reference, provider_id, provider_account_id,
			credential_revision, schema_version, cipher_version, key_version,
			wrapped_dek, wrap_nonce, ciphertext, data_nonce, aad_digest,
			status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)
		ON CONFLICT(credential_reference) DO UPDATE SET
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
		identity.CredentialReference, identity.ProviderID,
		identity.ProviderAccountID, identity.CredentialRevision,
		record.SchemaVersion, record.CipherVersion, record.KeyVersion,
		record.WrappedDEK, record.WrapNonce, record.Ciphertext, record.DataNonce,
		record.AADDigest, now, now,
	)
	if err != nil || transaction.Commit() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (store *VaultStore) ReadCredential(
	ctx context.Context,
	identity CredentialIdentity,
) ([]byte, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialIdentity(identity) {
		return nil, ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	if err := store.validateDatabaseFile(); err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	record, err := store.readRecord(ctx, identity)
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultOpen,
			err,
		)
	}
	if record.KeyVersion != store.keyMaterial.KeyVersion() {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultKeyLoad,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return nil, credentials.WithCredentialFailureStage(
			credentials.CredentialStageVaultKeyLoad,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	defer clearBytes(vmk)
	plaintext, err := store.cipher.Decrypt(vmk, record)
	if err != nil {
		stage := credentials.CredentialFailureStage(err)
		if stage == "" {
			stage = credentials.CredentialStageVaultDecrypt
		}
		return nil, credentials.WithCredentialFailureStage(
			stage,
			credentials.ErrCredentialStoreUnavailable,
		)
	}
	return plaintext, nil
}

// HasCredential checks the exact active encrypted record without decrypting it.
func (store *VaultStore) HasCredential(
	ctx context.Context,
	identity CredentialIdentity,
) (bool, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialIdentity(identity) {
		return false, ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return false, credentials.ErrCredentialStoreUnavailable
	}
	if err := store.validateRuntime(); err != nil {
		return false, credentials.ErrCredentialStoreUnavailable
	}
	var marker int
	err := store.database.QueryRowContext(
		ctx,
		`SELECT 1 FROM encrypted_credentials
		  WHERE credential_reference = ? AND provider_id = ?
		    AND provider_account_id = ? AND credential_revision = ?
		    AND status = 'active'`,
		identity.CredentialReference, identity.ProviderID,
		identity.ProviderAccountID, identity.CredentialRevision,
	).Scan(&marker)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil || marker != 1 {
		return false, credentials.ErrCredentialStoreUnavailable
	}
	return true, nil
}

func (store *VaultStore) DeleteCredential(
	ctx context.Context,
	identity CredentialIdentity,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validCredentialIdentity(identity) {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return credentials.ErrCredentialStoreUnavailable
	}
	if err := store.validateRuntime(); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	result, err := store.database.ExecContext(
		ctx,
		`DELETE FROM encrypted_credentials
		 WHERE credential_reference = ? AND provider_id = ?
		   AND provider_account_id = ? AND credential_revision = ?`,
		identity.CredentialReference, identity.ProviderID,
		identity.ProviderAccountID, identity.CredentialRevision,
	)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	if count != 1 {
		return credentials.ErrCredentialNotFound
	}
	return nil
}

// Health validates the live key and database identities without decrypting a
// credential or exposing key material to the caller.
func (store *VaultStore) Health(ctx context.Context) error {
	if store == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (store *VaultStore) KeyIdentity(
	ctx context.Context,
) (uint32, [localKeyIDBytes]byte, error) {
	if store == nil || ctx == nil || ctx.Err() != nil {
		return 0, [localKeyIDBytes]byte{}, ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return 0, [localKeyIDBytes]byte{}, credentials.ErrCredentialStoreUnavailable
	}
	return store.keyMaterial.KeyVersion(), store.keyMaterial.KeyID(), nil
}

// AdoptRotatedKeyMaterial rebinds an already committed Store to the promoted
// canonical key file. Ownership transfers only after all identities agree.
func (store *VaultStore) AdoptRotatedKeyMaterial(
	ctx context.Context,
	material *KeyMaterial,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || material == nil {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateDatabaseFile() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	version := material.KeyVersion()
	keyID := material.KeyID()
	if version == 0 || version != store.keyMaterial.KeyVersion() ||
		keyID != store.keyMaterial.KeyID() {
		return ErrInvalidVaultInput
	}
	key, err := material.keyCopy()
	clearBytes(key)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	var storedVersion uint32
	var storedKeyID []byte
	if err := store.database.QueryRowContext(
		ctx,
		`SELECT key_version, key_id FROM vault_metadata WHERE singleton = 1`,
	).Scan(&storedVersion, &storedKeyID); err != nil ||
		storedVersion != version || !bytes.Equal(storedKeyID, keyID[:]) {
		return credentials.ErrCredentialStoreUnavailable
	}
	oldMaterial := store.keyMaterial
	store.keyMaterial = material
	_ = oldMaterial.Close()
	return nil
}

// RotateWrappingKey atomically rewraps every active credential and advances
// the Vault metadata to newMaterial. Ownership transfers only after commit.
func (store *VaultStore) RotateWrappingKey(
	ctx context.Context,
	newMaterial *KeyMaterial,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || newMaterial == nil {
		return ErrInvalidVaultInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	oldVersion := store.keyMaterial.KeyVersion()
	newVersion := newMaterial.KeyVersion()
	oldKeyID := store.keyMaterial.KeyID()
	newKeyID := newMaterial.KeyID()
	if newVersion <= oldVersion || oldKeyID == newKeyID {
		return ErrInvalidVaultInput
	}
	oldVMK, err := store.keyMaterial.keyCopy()
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(oldVMK)
	newVMK, err := newMaterial.keyCopy()
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer clearBytes(newVMK)

	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	defer transaction.Rollback()
	var pendingCount int
	if err := transaction.QueryRowContext(
		ctx, `SELECT COUNT(*) FROM pending_credential_mutations`,
	).Scan(&pendingCount); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	if pendingCount != 0 {
		return credentials.ErrCredentialMetadataConflict
	}
	records, err := readAllCredentialRecordsTx(ctx, transaction)
	if err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	for _, record := range records {
		rotated, rotateErr := store.cipher.RotateWrappingKey(
			oldVMK, newVMK, newVersion, record,
		)
		if rotateErr != nil {
			return credentials.ErrCredentialStoreUnavailable
		}
		result, updateErr := transaction.ExecContext(
			ctx,
			`UPDATE encrypted_credentials SET
				key_version = ?, wrapped_dek = ?, wrap_nonce = ?,
				ciphertext = ?, data_nonce = ?, aad_digest = ?, updated_at = ?
			 WHERE credential_reference = ? AND provider_id = ?
			   AND provider_account_id = ? AND credential_revision = ?
			   AND key_version = ? AND status = 'active'`,
			rotated.KeyVersion, rotated.WrappedDEK, rotated.WrapNonce,
			rotated.Ciphertext, rotated.DataNonce, rotated.AADDigest, now,
			record.Identity.CredentialReference, record.Identity.ProviderID,
			record.Identity.ProviderAccountID,
			record.Identity.CredentialRevision, record.KeyVersion,
		)
		if updateErr != nil || !exactlyOneRow(result) {
			return credentials.ErrCredentialStoreUnavailable
		}
	}
	if err := store.rotateConversationKeysTx(
		ctx, transaction, oldVMK, newVMK, oldVersion, newVersion,
	); err != nil {
		return err
	}
	result, err := transaction.ExecContext(
		ctx,
		`UPDATE vault_metadata SET key_version = ?, key_id = ?
		  WHERE singleton = 1 AND schema_version = ?
		    AND key_version = ? AND key_id = ?`,
		newVersion, newKeyID[:], vaultDatabaseSchema, oldVersion, oldKeyID[:],
	)
	if err != nil || !exactlyOneRow(result) {
		return credentials.ErrCredentialStoreUnavailable
	}
	if err := store.injectMutationFault(vaultFaultRotationBeforeCommit); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return credentials.ErrCredentialStoreUnavailable
	}
	oldMaterial := store.keyMaterial
	store.keyMaterial = newMaterial
	_ = oldMaterial.Close()
	return nil
}

func (store *VaultStore) Close() error {
	if store == nil {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed {
		return nil
	}
	store.closed = true
	var result error
	if store.database != nil {
		result = store.database.Close()
	}
	if store.keyMaterial != nil {
		result = errors.Join(result, store.keyMaterial.Close())
	}
	return result
}

func (store *VaultStore) initialize(ctx context.Context) error {
	for _, statement := range []string{
		`PRAGMA journal_mode=DELETE`,
		`PRAGMA synchronous=FULL`,
		`PRAGMA secure_delete=ON`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS vault_metadata (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			schema_version INTEGER NOT NULL,
			key_version INTEGER NOT NULL,
			key_id BLOB NOT NULL
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_credentials (
			credential_reference TEXT PRIMARY KEY,
			provider_id TEXT NOT NULL,
			provider_account_id TEXT NOT NULL,
			credential_revision INTEGER NOT NULL CHECK (credential_revision > 0),
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			key_version INTEGER NOT NULL,
			wrapped_dek BLOB NOT NULL,
			wrap_nonce BLOB NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('active')),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS pending_credential_mutations (
			mutation_id TEXT PRIMARY KEY,
			mutation_kind TEXT NOT NULL CHECK (
				mutation_kind IN ('configure', 'import', 'rebind', 'replace', 'revoke')
			),
			credential_reference TEXT NOT NULL UNIQUE,
			provider_id TEXT NOT NULL,
			provider_account_id TEXT NOT NULL,
			current_revision INTEGER NOT NULL CHECK (current_revision >= 0),
			candidate_revision INTEGER NOT NULL CHECK (candidate_revision > 0),
			schema_version INTEGER,
			cipher_version INTEGER,
			key_version INTEGER,
			wrapped_dek BLOB,
			wrap_nonce BLOB,
			ciphertext BLOB,
			data_nonce BLOB,
			aad_digest BLOB,
			status TEXT NOT NULL CHECK (status = 'pending'),
			created_at TEXT NOT NULL,
			CHECK (
				(mutation_kind = 'revoke' AND schema_version IS NULL AND
				 cipher_version IS NULL AND key_version IS NULL AND
				 wrapped_dek IS NULL AND wrap_nonce IS NULL AND
				 ciphertext IS NULL AND data_nonce IS NULL AND aad_digest IS NULL)
				OR
				(mutation_kind != 'revoke' AND schema_version IS NOT NULL AND
				 cipher_version IS NOT NULL AND key_version IS NOT NULL AND
				 wrapped_dek IS NOT NULL AND wrap_nonce IS NOT NULL AND
				 ciphertext IS NOT NULL AND data_nonce IS NOT NULL AND
				 aad_digest IS NOT NULL)
			)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_conversation_keys (
			conversation_id TEXT PRIMARY KEY,
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			key_version INTEGER NOT NULL,
			wrapped_dek BLOB NOT NULL,
			wrap_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_context_capsules (
			capsule_digest TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			disclosure_receipt_digest TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			authority_record BLOB NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_conversation_documents (
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			document_kind TEXT NOT NULL,
			document_revision INTEGER NOT NULL CHECK (document_revision > 0),
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (conversation_id, document_kind)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_external_session_handles (
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			segment_id TEXT NOT NULL,
			handle_kind TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			provider_account_id TEXT NOT NULL,
			model_id TEXT NOT NULL,
			auth_mode TEXT NOT NULL,
			credential_reference TEXT NOT NULL,
			credential_revision INTEGER NOT NULL CHECK (credential_revision >= 0),
			handle_revision INTEGER NOT NULL CHECK (handle_revision > 0),
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (conversation_id, segment_id, handle_kind)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_attempt_payloads (
			payload_id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			work_item_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			claim_generation INTEGER NOT NULL CHECK (claim_generation > 0),
			runtime_instance_id TEXT NOT NULL,
			execution_binding_digest TEXT NOT NULL,
			capsule_digest TEXT NOT NULL,
			call_id TEXT NOT NULL,
			sequence INTEGER NOT NULL CHECK (sequence > 0),
			content_type TEXT NOT NULL,
			content_digest TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'delivered')),
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (run_id, claim_generation, sequence)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_agent_inputs (
			payload_id TEXT PRIMARY KEY,
			input_id TEXT NOT NULL UNIQUE,
			mode TEXT NOT NULL CHECK (mode IN ('queue', 'steer', 'inject')),
			context_scope TEXT NOT NULL,
			scope_target_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			segment_id TEXT NOT NULL,
			agent_instance_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			claim_generation INTEGER NOT NULL CHECK (claim_generation > 0),
			runtime_instance_id TEXT NOT NULL,
			execution_binding_digest TEXT NOT NULL,
			capsule_digest TEXT NOT NULL,
			order_key INTEGER NOT NULL CHECK (order_key > 0),
			target_turn_id TEXT NOT NULL,
			target_turn_sequence INTEGER NOT NULL CHECK (target_turn_sequence >= 0),
			target_step_id TEXT NOT NULL,
			target_step_sequence INTEGER NOT NULL CHECK (target_step_sequence >= 0),
			content_type TEXT NOT NULL,
			content_digest TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'consumed')),
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (run_id, claim_generation, order_key)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_agent_checkpoints (
			checkpoint_id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			segment_id TEXT NOT NULL,
			attempt_id TEXT NOT NULL,
			agent_instance_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			claim_generation INTEGER NOT NULL CHECK (claim_generation > 0),
			runtime_instance_id TEXT NOT NULL,
			execution_binding_digest TEXT NOT NULL,
			capsule_digest TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			turn_sequence INTEGER NOT NULL CHECK (turn_sequence > 0),
			step_id TEXT NOT NULL,
			step_sequence INTEGER NOT NULL CHECK (step_sequence > 0),
			content_type TEXT NOT NULL,
			content_digest TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (attempt_id, claim_generation, turn_sequence, step_sequence)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_tool_proposals (
			proposal_id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			work_item_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			claim_generation INTEGER NOT NULL CHECK (claim_generation > 0),
			runtime_instance_id TEXT NOT NULL,
			agent_instance_id TEXT NOT NULL,
			execution_binding_digest TEXT NOT NULL,
			capsule_digest TEXT NOT NULL,
			call_id TEXT NOT NULL,
			call_digest TEXT NOT NULL,
			approval_id TEXT NOT NULL UNIQUE,
			approval_digest TEXT NOT NULL,
			tool TEXT NOT NULL,
			operation_id TEXT NOT NULL,
			incident_id TEXT NOT NULL,
			content_digest TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			cipher_version INTEGER NOT NULL,
			ciphertext BLOB NOT NULL,
			data_nonce BLOB NOT NULL,
			aad_digest BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE (run_id, claim_generation, call_id)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS encrypted_conversation_data_nonces (
			conversation_id TEXT NOT NULL REFERENCES encrypted_conversation_keys(
				conversation_id
			) ON DELETE CASCADE,
			data_nonce BLOB NOT NULL,
			purpose TEXT NOT NULL,
			record_id TEXT NOT NULL,
			PRIMARY KEY (conversation_id, data_nonce)
		) STRICT`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'context-capsule', capsule_digest
		    FROM encrypted_context_capsules`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'document',
		         document_kind || ':' || document_revision
		    FROM encrypted_conversation_documents`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'external-session-handle',
		         segment_id || ':' || handle_kind || ':' || handle_revision
		    FROM encrypted_external_session_handles`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'attempt-payload',
		         payload_id || ':' || status
		    FROM encrypted_attempt_payloads`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'agent-input',
		         payload_id || ':' || status
		    FROM encrypted_agent_inputs`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'agent-checkpoint', checkpoint_id
		    FROM encrypted_agent_checkpoints`,
		`INSERT OR IGNORE INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) SELECT conversation_id, data_nonce, 'tool-proposal', proposal_id
		    FROM encrypted_tool_proposals`,
		`CREATE UNIQUE INDEX IF NOT EXISTS encrypted_conversation_wrap_nonce
			ON encrypted_conversation_keys(wrap_nonce)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS encrypted_context_capsule_data_nonce
			ON encrypted_context_capsules(conversation_id, data_nonce)`,
	} {
		if _, err := store.database.ExecContext(ctx, statement); err != nil {
			return credentials.ErrCredentialStoreUnavailable
		}
	}
	keyID := store.keyMaterial.KeyID()
	var schema int
	var keyVersion uint32
	var storedKeyID []byte
	err := store.database.QueryRowContext(
		ctx,
		`SELECT schema_version, key_version, key_id
		   FROM vault_metadata WHERE singleton = 1`,
	).Scan(&schema, &keyVersion, &storedKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = store.database.ExecContext(
			ctx,
			`INSERT INTO vault_metadata
			 (singleton, schema_version, key_version, key_id)
			 VALUES (1, ?, ?, ?)`,
			vaultDatabaseSchema, store.keyMaterial.KeyVersion(), keyID[:],
		)
		if err != nil {
			return credentials.ErrCredentialStoreUnavailable
		}
		return nil
	}
	if err != nil || schema != vaultDatabaseSchema ||
		keyVersion != store.keyMaterial.KeyVersion() ||
		!bytes.Equal(storedKeyID, keyID[:]) {
		return credentials.ErrCredentialStoreUnavailable
	}
	return nil
}

func (store *VaultStore) readRecord(
	ctx context.Context,
	identity CredentialIdentity,
) (EncryptedCredential, error) {
	record := EncryptedCredential{Identity: identity}
	var schemaVersion, cipherVersion int64
	var keyVersion int64
	err := store.database.QueryRowContext(
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

func readAllCredentialRecordsTx(
	ctx context.Context,
	transaction *sql.Tx,
) ([]EncryptedCredential, error) {
	rows, err := transaction.QueryContext(
		ctx,
		`SELECT credential_reference, provider_id, provider_account_id,
		        credential_revision, schema_version, cipher_version, key_version,
		        wrapped_dek, wrap_nonce, ciphertext, data_nonce, aad_digest
		   FROM encrypted_credentials WHERE status = 'active'
		  ORDER BY credential_reference`,
	)
	if err != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	defer rows.Close()
	records := make([]EncryptedCredential, 0)
	for rows.Next() {
		var record EncryptedCredential
		var schemaVersion, cipherVersion, keyVersion int64
		if err := rows.Scan(
			&record.Identity.CredentialReference, &record.Identity.ProviderID,
			&record.Identity.ProviderAccountID,
			&record.Identity.CredentialRevision, &schemaVersion, &cipherVersion,
			&keyVersion, &record.WrappedDEK, &record.WrapNonce,
			&record.Ciphertext, &record.DataNonce, &record.AADDigest,
		); err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
			cipherVersion < 0 || cipherVersion > 1<<16-1 ||
			keyVersion < 0 || keyVersion > 1<<32-1 {
			return nil, credentials.ErrCredentialStoreUnavailable
		}
		record.SchemaVersion = uint16(schemaVersion)
		record.CipherVersion = uint16(cipherVersion)
		record.KeyVersion = uint32(keyVersion)
		if !validEncryptedCredential(record) {
			return nil, credentials.ErrCredentialStoreUnavailable
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, credentials.ErrCredentialStoreUnavailable
	}
	return records, nil
}

func exactlyOneRow(result sql.Result) bool {
	if result == nil {
		return false
	}
	count, err := result.RowsAffected()
	return err == nil && count == 1
}

func (store *VaultStore) resolveIdentity(
	ctx context.Context,
	reference string,
) (CredentialIdentity, error) {
	if store == nil || store.identityResolver == nil {
		return CredentialIdentity{}, credentials.ErrCredentialStoreUnavailable
	}
	identity, err := store.identityResolver(ctx, reference)
	if err != nil {
		if errors.Is(err, credentials.ErrCredentialNotFound) {
			return CredentialIdentity{}, credentials.ErrCredentialNotFound
		}
		return CredentialIdentity{}, credentials.ErrCredentialStoreUnavailable
	}
	if identity.CredentialReference != reference ||
		!validCredentialIdentity(identity) {
		return CredentialIdentity{}, credentials.ErrCredentialStoreUnavailable
	}
	return identity, nil
}

func validVaultDatabasePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path &&
		!strings.ContainsAny(path, "\x00\n\r?#") && len(path) <= 1_024
}

func ensurePrivateDatabaseFile(path string) error {
	info, err := os.Lstat(path)
	if err == nil {
		return validatePrivateRegularFile(info)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return ErrVaultUnavailable
	}
	file, err := os.OpenFile(
		path,
		os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW,
		0o600,
	)
	if err != nil {
		return ErrVaultUnavailable
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return ErrVaultUnavailable
	}
	if err := file.Close(); err != nil {
		return ErrVaultUnavailable
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return ErrVaultUnavailable
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return ErrVaultUnavailable
	}
	info, err = os.Lstat(path)
	if err != nil || validatePrivateRegularFile(info) != nil {
		return ErrVaultUnavailable
	}
	return nil
}

type privateFileIdentity struct {
	device uint64
	inode  uint64
}

func capturePrivateFileIdentity(path string) (privateFileIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil || validatePrivateRegularFile(info) != nil {
		return privateFileIdentity{}, ErrVaultUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return privateFileIdentity{}, ErrVaultUnavailable
	}
	return privateFileIdentity{
		device: uint64(stat.Dev),
		inode:  uint64(stat.Ino),
	}, nil
}

func (store *VaultStore) validateDatabaseFile() error {
	if store == nil {
		return ErrVaultUnavailable
	}
	identity, err := capturePrivateFileIdentity(store.databasePath)
	if err != nil || identity != store.databaseIdentity {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) validateRuntime() error {
	if store == nil || store.validateDatabaseFile() != nil ||
		store.keyMaterial == nil {
		return ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	clearBytes(vmk)
	if err != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) String() string {
	return fmt.Sprintf("CredentialVault(%s)", filepath.Base(store.databasePath))
}
