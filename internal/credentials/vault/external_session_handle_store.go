package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/credentials"
)

const (
	externalSessionHandleSchemaVersion = uint16(1)
	externalSessionHandleCipherVersion = uint16(1)
	maxExternalSessionHandlePayload    = 16 << 10

	ExternalSessionHandleResponseID     = "response_id"
	ExternalSessionHandleConversationID = "conversation_id"
	ExternalSessionHandlePromptCacheID  = "prompt_cache_id"
)

var (
	ErrInvalidExternalSessionHandle        = errors.New("invalid encrypted ExternalSessionHandle")
	ErrExternalSessionHandleNotFound       = errors.New("encrypted ExternalSessionHandle not found")
	ErrExternalSessionHandleBinding        = errors.New("encrypted ExternalSessionHandle binding mismatch")
	ErrExternalSessionHandleConflict       = errors.New("encrypted ExternalSessionHandle revision conflict")
	ErrExternalSessionHandleAuthentication = errors.New("encrypted ExternalSessionHandle authentication failed")
)

// ExternalSessionHandleBinding freezes the only route allowed to consume one
// Provider-native handle. The opaque value is never part of this metadata.
type ExternalSessionHandleBinding struct {
	ConversationID      string
	SegmentID           string
	ProviderID          string
	ProviderAccountID   string
	ModelID             string
	AuthMode            string
	CredentialReference string
	CredentialRevision  int64
}

type ExternalSessionHandle struct {
	Binding  ExternalSessionHandleBinding
	Kind     string
	Revision int64
	Value    []byte
}

type encryptedExternalSessionHandle struct {
	ExternalSessionHandle
	SchemaVersion uint16
	CipherVersion uint16
	Ciphertext    []byte
	DataNonce     []byte
	AADDigest     []byte
}

func (store *VaultStore) PutExternalSessionHandle(
	ctx context.Context,
	handle ExternalSessionHandle,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validExternalSessionHandle(handle) {
		return ErrInvalidExternalSessionHandle
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer transaction.Rollback()
	_, dek, err := store.readOrCreateConversationDEKTx(
		ctx, transaction, vmk, handle.Binding.ConversationID,
	)
	if err != nil {
		return err
	}
	defer clearBytes(dek)

	existing, found, err := readEncryptedExternalSessionHandleTx(
		ctx, transaction, handle.Binding.ConversationID,
		handle.Binding.SegmentID, handle.Kind,
	)
	if err != nil {
		return err
	}
	if found {
		if existing.Binding != handle.Binding {
			return ErrExternalSessionHandleBinding
		}
		if handle.Revision < existing.Revision {
			return ErrExternalSessionHandleConflict
		}
		if handle.Revision == existing.Revision {
			plaintext, decryptErr := decryptExternalSessionHandle(dek, existing)
			if decryptErr != nil {
				return decryptErr
			}
			same := bytes.Equal(plaintext, handle.Value)
			clearBytes(plaintext)
			if !same {
				return ErrExternalSessionHandleConflict
			}
			return nil
		}
	}

	record, err := store.encryptExternalSessionHandle(handle, dek)
	if err != nil {
		return err
	}
	recordID := record.Binding.SegmentID + ":" + record.Kind + ":" +
		decimalRevision(record.Revision)
	if err := reserveConversationDataNonceTx(
		ctx, transaction, record.Binding.ConversationID, record.DataNonce,
		"external-session-handle", recordID,
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	result, err := transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_external_session_handles (
			conversation_id, segment_id, handle_kind, provider_id,
			provider_account_id, model_id, auth_mode, credential_reference,
			credential_revision, handle_revision, schema_version,
			cipher_version, ciphertext, data_nonce, aad_digest,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conversation_id, segment_id, handle_kind) DO UPDATE SET
			handle_revision=excluded.handle_revision,
			schema_version=excluded.schema_version,
			cipher_version=excluded.cipher_version,
			ciphertext=excluded.ciphertext,
			data_nonce=excluded.data_nonce,
			aad_digest=excluded.aad_digest,
			updated_at=excluded.updated_at
		WHERE encrypted_external_session_handles.provider_id = excluded.provider_id
		  AND encrypted_external_session_handles.provider_account_id = excluded.provider_account_id
		  AND encrypted_external_session_handles.model_id = excluded.model_id
		  AND encrypted_external_session_handles.auth_mode = excluded.auth_mode
		  AND encrypted_external_session_handles.credential_reference = excluded.credential_reference
		  AND encrypted_external_session_handles.credential_revision = excluded.credential_revision
		  AND encrypted_external_session_handles.handle_revision < excluded.handle_revision`,
		record.Binding.ConversationID, record.Binding.SegmentID, record.Kind,
		record.Binding.ProviderID, record.Binding.ProviderAccountID,
		record.Binding.ModelID, record.Binding.AuthMode,
		record.Binding.CredentialReference,
		record.Binding.CredentialRevision, record.Revision,
		record.SchemaVersion, record.CipherVersion, record.Ciphertext,
		record.DataNonce, record.AADDigest, now, now,
	)
	if err != nil || !exactlyOneRow(result) || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ReadExternalSessionHandle(
	ctx context.Context,
	binding ExternalSessionHandleBinding,
	kind string,
) (ExternalSessionHandle, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validExternalSessionHandleBinding(binding) ||
		!validExternalSessionHandleKind(kind) {
		return ExternalSessionHandle{}, ErrInvalidExternalSessionHandle
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return ExternalSessionHandle{}, ErrVaultUnavailable
	}
	record, found, err := readEncryptedExternalSessionHandleTx(
		ctx, store.database, binding.ConversationID, binding.SegmentID, kind,
	)
	if err != nil {
		return ExternalSessionHandle{}, err
	}
	if !found {
		return ExternalSessionHandle{}, ErrExternalSessionHandleNotFound
	}
	if record.Binding != binding {
		return ExternalSessionHandle{}, ErrExternalSessionHandleBinding
	}
	conversationKey, err := readEncryptedConversationKey(
		ctx, store.database, binding.ConversationID,
	)
	if err != nil {
		if errors.Is(err, ErrContextCapsuleNotFound) {
			return ExternalSessionHandle{}, ErrExternalSessionHandleNotFound
		}
		return ExternalSessionHandle{}, err
	}
	if conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return ExternalSessionHandle{}, ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return ExternalSessionHandle{}, ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return ExternalSessionHandle{}, err
	}
	defer clearBytes(dek)
	plaintext, err := decryptExternalSessionHandle(dek, record)
	if err != nil {
		return ExternalSessionHandle{}, err
	}
	return ExternalSessionHandle{
		Binding: record.Binding, Kind: record.Kind,
		Revision: record.Revision, Value: plaintext,
	}, nil
}

func (store *VaultStore) encryptExternalSessionHandle(
	handle ExternalSessionHandle,
	dek []byte,
) (encryptedExternalSessionHandle, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes || !validExternalSessionHandle(handle) {
		return encryptedExternalSessionHandle{}, ErrInvalidExternalSessionHandle
	}
	record := encryptedExternalSessionHandle{
		ExternalSessionHandle: ExternalSessionHandle{
			Binding: handle.Binding, Kind: handle.Kind, Revision: handle.Revision,
		},
		SchemaVersion: externalSessionHandleSchemaVersion,
		CipherVersion: externalSessionHandleCipherVersion,
		DataNonce:     make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, record.DataNonce); err != nil {
		return encryptedExternalSessionHandle{}, ErrVaultUnavailable
	}
	aad, err := canonicalExternalSessionHandleAAD(record)
	if err != nil {
		return encryptedExternalSessionHandle{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedExternalSessionHandle{}, ErrVaultUnavailable
	}
	record.Ciphertext = aead.Seal(nil, record.DataNonce, handle.Value, aad)
	return record, nil
}

func decryptExternalSessionHandle(
	dek []byte,
	record encryptedExternalSessionHandle,
) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedExternalSessionHandle(record) {
		return nil, ErrExternalSessionHandleAuthentication
	}
	aad, err := canonicalExternalSessionHandleAAD(record)
	if err != nil {
		return nil, ErrExternalSessionHandleAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrExternalSessionHandleAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 ||
		len(plaintext) > maxExternalSessionHandlePayload {
		clearBytes(plaintext)
		return nil, ErrExternalSessionHandleAuthentication
	}
	return plaintext, nil
}

func readEncryptedExternalSessionHandleTx(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	conversationID string,
	segmentID string,
	kind string,
) (encryptedExternalSessionHandle, bool, error) {
	record := encryptedExternalSessionHandle{
		ExternalSessionHandle: ExternalSessionHandle{
			Binding: ExternalSessionHandleBinding{
				ConversationID: conversationID, SegmentID: segmentID,
			},
			Kind: kind,
		},
	}
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(
		ctx,
		`SELECT provider_id, provider_account_id, model_id, auth_mode,
		        credential_reference, credential_revision, handle_revision,
		        schema_version, cipher_version, ciphertext, data_nonce, aad_digest
		   FROM encrypted_external_session_handles
		  WHERE conversation_id = ? AND segment_id = ? AND handle_kind = ?`,
		conversationID, segmentID, kind,
	).Scan(
		&record.Binding.ProviderID, &record.Binding.ProviderAccountID,
		&record.Binding.ModelID, &record.Binding.AuthMode,
		&record.Binding.CredentialReference,
		&record.Binding.CredentialRevision, &record.Revision,
		&schemaVersion, &cipherVersion, &record.Ciphertext,
		&record.DataNonce, &record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedExternalSessionHandle{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedExternalSessionHandle{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	if !validEncryptedExternalSessionHandle(record) {
		return encryptedExternalSessionHandle{}, false,
			ErrExternalSessionHandleAuthentication
	}
	return record, true, nil
}

func canonicalExternalSessionHandleAAD(
	record encryptedExternalSessionHandle,
) ([]byte, error) {
	if record.SchemaVersion != externalSessionHandleSchemaVersion ||
		record.CipherVersion != externalSessionHandleCipherVersion ||
		!validExternalSessionHandleBinding(record.Binding) ||
		!validExternalSessionHandleKind(record.Kind) || record.Revision <= 0 {
		return nil, ErrInvalidExternalSessionHandle
	}
	var body bytes.Buffer
	body.WriteString("LOOM-EXTERNAL-SESSION-HANDLE-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.Binding.ConversationID)
	writeCanonicalString(&body, record.Binding.SegmentID)
	writeCanonicalString(&body, record.Binding.ProviderID)
	writeCanonicalString(&body, record.Binding.ProviderAccountID)
	writeCanonicalString(&body, record.Binding.ModelID)
	writeCanonicalString(&body, record.Binding.AuthMode)
	writeCanonicalString(&body, record.Binding.CredentialReference)
	_ = binary.Write(&body, binary.BigEndian, record.Binding.CredentialRevision)
	writeCanonicalString(&body, record.Kind)
	_ = binary.Write(&body, binary.BigEndian, record.Revision)
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func validExternalSessionHandle(handle ExternalSessionHandle) bool {
	return validExternalSessionHandleBinding(handle.Binding) &&
		validExternalSessionHandleKind(handle.Kind) && handle.Revision > 0 &&
		len(handle.Value) > 0 && len(handle.Value) <= maxExternalSessionHandlePayload
}

func validEncryptedExternalSessionHandle(record encryptedExternalSessionHandle) bool {
	return record.SchemaVersion == externalSessionHandleSchemaVersion &&
		record.CipherVersion == externalSessionHandleCipherVersion &&
		validExternalSessionHandleBinding(record.Binding) &&
		validExternalSessionHandleKind(record.Kind) && record.Revision > 0 &&
		len(record.Ciphertext) > 16 &&
		len(record.Ciphertext) <= maxExternalSessionHandlePayload+16 &&
		len(record.DataNonce) == vaultNonceBytes &&
		len(record.AADDigest) == vaultAADDigestBytes
}

func validExternalSessionHandleBinding(binding ExternalSessionHandleBinding) bool {
	if !validContextCapsuleIdentifier(binding.ConversationID) ||
		!validContextCapsuleIdentifier(binding.SegmentID) ||
		!credentials.ValidProviderIdentifier(binding.ProviderID) ||
		!validExternalSessionModelID(binding.ModelID) {
		return false
	}
	switch binding.AuthMode {
	case "native_auth":
		return binding.ProviderAccountID == "" &&
			binding.CredentialReference == "" && binding.CredentialRevision == 0
	case "brokered", "provider_ephemeral":
		return credentials.ValidProviderAccountIdentifier(
			binding.ProviderID, binding.ProviderAccountID,
		) && validCredentialIdentity(CredentialIdentity{
			ProviderID:          binding.ProviderID,
			ProviderAccountID:   binding.ProviderAccountID,
			CredentialReference: binding.CredentialReference,
			CredentialRevision:  binding.CredentialRevision,
		})
	default:
		return false
	}
}

func validExternalSessionHandleKind(kind string) bool {
	switch kind {
	case ExternalSessionHandleResponseID,
		ExternalSessionHandleConversationID,
		ExternalSessionHandlePromptCacheID:
		return true
	default:
		return false
	}
}

func validExternalSessionModelID(value string) bool {
	if value == "" || len(value) > 128 || !utf8.ValidString(value) ||
		value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '_' || character == '-' || character == '.' ||
			character == ':' || character == '/' {
			continue
		}
		return false
	}
	return true
}
