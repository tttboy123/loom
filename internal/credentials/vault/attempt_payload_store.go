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

	"loom-pi-rebuild/internal/attemptpayload"
)

const (
	attemptPayloadSchemaVersion = uint16(1)
	attemptPayloadCipherVersion = uint16(1)
	maxAttemptPayloadBytes      = 32 << 10

	AttemptPayloadPending   = attemptpayload.StatusPending
	AttemptPayloadDelivered = attemptpayload.StatusDelivered
)

var (
	ErrInvalidAttemptPayload        = errors.New("invalid encrypted Attempt payload")
	ErrAttemptPayloadNotFound       = attemptpayload.ErrPayloadNotFound
	ErrAttemptPayloadBinding        = errors.New("encrypted Attempt payload binding mismatch")
	ErrAttemptPayloadConflict       = errors.New("encrypted Attempt payload conflict")
	ErrAttemptPayloadAuthentication = errors.New("encrypted Attempt payload authentication failed")
)

type AttemptPayloadStatus = attemptpayload.Status
type AttemptPayloadScope = attemptpayload.Scope
type AttemptPayloadBinding = attemptpayload.Binding
type AttemptPayload = attemptpayload.Payload

type encryptedAttemptPayload struct {
	AttemptPayload
	SchemaVersion uint16
	CipherVersion uint16
	Ciphertext    []byte
	DataNonce     []byte
	AADDigest     []byte
}

func (store *VaultStore) PutAttemptPayload(
	ctx context.Context,
	payload AttemptPayload,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		payload.Status != AttemptPayloadPending || !validAttemptPayload(payload) {
		return ErrInvalidAttemptPayload
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
		ctx, transaction, vmk, payload.Binding.ConversationID,
	)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	existing, found, err := readEncryptedAttemptPayloadTx(
		ctx, transaction, payload.Binding.PayloadID,
	)
	if err != nil {
		return err
	}
	if found {
		plaintext, decryptErr := decryptAttemptPayload(dek, existing)
		if decryptErr != nil {
			return decryptErr
		}
		defer clearBytes(plaintext)
		if existing.Binding != payload.Binding {
			if sameAttemptPayloadDeliveryIdentity(existing.Binding, payload.Binding) {
				return ErrAttemptPayloadConflict
			}
			return ErrAttemptPayloadBinding
		}
		same := bytes.Equal(plaintext, payload.Content)
		if !same {
			return ErrAttemptPayloadConflict
		}
		return nil
	}
	if reused, err := attemptPayloadSequenceExists(
		ctx, transaction, payload.Binding,
	); err != nil {
		return err
	} else if reused {
		return ErrAttemptPayloadConflict
	}
	record, err := store.encryptAttemptPayload(payload, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, transaction, record.Binding.ConversationID, record.DataNonce,
		"attempt-payload", record.Binding.PayloadID+":"+string(record.Status),
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	_, err = transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_attempt_payloads (
			payload_id, conversation_id, work_item_id, run_id,
			claim_generation, runtime_instance_id, execution_binding_digest,
			capsule_digest, call_id, sequence, content_type, content_digest,
			status, schema_version, cipher_version, ciphertext, data_nonce,
			aad_digest, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Binding.PayloadID, record.Binding.ConversationID,
		record.Binding.WorkItemID, record.Binding.RunID,
		record.Binding.ClaimGeneration, record.Binding.RuntimeInstanceID,
		record.Binding.ExecutionBindingDigest, record.Binding.CapsuleDigest,
		record.Binding.CallID, record.Binding.Sequence, record.Binding.ContentType,
		record.Binding.ContentDigest, record.Status, record.SchemaVersion,
		record.CipherVersion, record.Ciphertext, record.DataNonce,
		record.AADDigest, now, now,
	)
	if err != nil || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ReadAttemptPayload(
	ctx context.Context,
	binding AttemptPayloadBinding,
) (AttemptPayload, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validAttemptPayloadBinding(binding) {
		return AttemptPayload{}, ErrInvalidAttemptPayload
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return AttemptPayload{}, ErrVaultUnavailable
	}
	record, found, err := readEncryptedAttemptPayloadTx(
		ctx, store.database, binding.PayloadID,
	)
	if err != nil {
		return AttemptPayload{}, err
	}
	if !found {
		return AttemptPayload{}, ErrAttemptPayloadNotFound
	}
	payload, err := store.decryptAttemptPayloadRecord(ctx, record)
	if err != nil {
		return AttemptPayload{}, err
	}
	if record.Binding != binding {
		payload.Close()
		return AttemptPayload{}, ErrAttemptPayloadBinding
	}
	return payload, nil
}

func (store *VaultStore) ListPendingAttemptPayloads(
	ctx context.Context,
	scope AttemptPayloadScope,
) ([]AttemptPayload, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validAttemptPayloadScope(scope) {
		return nil, ErrInvalidAttemptPayload
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return nil, ErrVaultUnavailable
	}
	records, err := readPendingAttemptPayloads(ctx, store.database, scope)
	if err != nil {
		return nil, err
	}
	results := make([]AttemptPayload, 0, len(records))
	for _, record := range records {
		payload, decryptErr := store.decryptAttemptPayloadRecord(ctx, record)
		if decryptErr != nil {
			clearAttemptPayloadResults(results)
			return nil, decryptErr
		}
		if payload.Status == AttemptPayloadPending {
			results = append(results, payload)
		} else {
			payload.Close()
		}
	}
	return results, nil
}

func (store *VaultStore) MarkAttemptPayloadDelivered(
	ctx context.Context,
	binding AttemptPayloadBinding,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validAttemptPayloadBinding(binding) {
		return ErrInvalidAttemptPayload
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
	record, found, err := readEncryptedAttemptPayloadTx(
		ctx, transaction, binding.PayloadID,
	)
	if err != nil {
		return err
	}
	if !found {
		return ErrAttemptPayloadNotFound
	}
	conversationKey, err := readEncryptedConversationKey(
		ctx, transaction, record.Binding.ConversationID,
	)
	if err != nil || conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return ErrVaultUnavailable
	}
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	plaintext, err := decryptAttemptPayload(dek, record)
	if err != nil {
		return err
	}
	defer clearBytes(plaintext)
	if record.Binding != binding {
		return ErrAttemptPayloadBinding
	}
	if record.Status == AttemptPayloadDelivered {
		return nil
	}
	delivered := AttemptPayload{
		Binding: record.Binding, Status: AttemptPayloadDelivered,
		Content: plaintext,
	}
	updated, err := store.encryptAttemptPayload(delivered, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, transaction, binding.ConversationID, updated.DataNonce,
		"attempt-payload", binding.PayloadID+":"+string(updated.Status),
	); err != nil {
		return err
	}
	result, err := transaction.ExecContext(
		ctx,
		`UPDATE encrypted_attempt_payloads SET
			status = ?, schema_version = ?, cipher_version = ?, ciphertext = ?,
			data_nonce = ?, aad_digest = ?, updated_at = ?
		 WHERE payload_id = ? AND status = 'pending' AND data_nonce = ?`,
		updated.Status, updated.SchemaVersion, updated.CipherVersion,
		updated.Ciphertext, updated.DataNonce, updated.AADDigest,
		store.now().UTC().Format(time.RFC3339Nano), binding.PayloadID,
		record.DataNonce,
	)
	if err != nil || !exactlyOneRow(result) || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) DeleteAttemptPayload(
	ctx context.Context,
	binding AttemptPayloadBinding,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validAttemptPayloadBinding(binding) {
		return ErrInvalidAttemptPayload
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return ErrVaultUnavailable
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer transaction.Rollback()
	record, found, err := readEncryptedAttemptPayloadTx(ctx, transaction, binding.PayloadID)
	if err != nil {
		return err
	}
	if !found {
		return ErrAttemptPayloadNotFound
	}
	conversationKey, err := readEncryptedConversationKey(
		ctx, transaction, record.Binding.ConversationID,
	)
	if err != nil || conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	plaintext, err := decryptAttemptPayload(dek, record)
	clearBytes(plaintext)
	if err != nil {
		return err
	}
	if record.Binding != binding {
		return ErrAttemptPayloadBinding
	}
	result, err := transaction.ExecContext(
		ctx, `DELETE FROM encrypted_attempt_payloads WHERE payload_id = ?`,
		binding.PayloadID,
	)
	if err != nil || !exactlyOneRow(result) || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) decryptAttemptPayloadRecord(
	ctx context.Context,
	record encryptedAttemptPayload,
) (AttemptPayload, error) {
	conversationKey, err := readEncryptedConversationKey(
		ctx, store.database, record.Binding.ConversationID,
	)
	if err != nil || conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return AttemptPayload{}, ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return AttemptPayload{}, ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return AttemptPayload{}, err
	}
	defer clearBytes(dek)
	plaintext, err := decryptAttemptPayload(dek, record)
	if err != nil {
		return AttemptPayload{}, err
	}
	return AttemptPayload{
		Binding: record.Binding, Status: record.Status, Content: plaintext,
	}, nil
}

func (store *VaultStore) encryptAttemptPayload(
	payload AttemptPayload,
	dek []byte,
) (encryptedAttemptPayload, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes || !validAttemptPayload(payload) {
		return encryptedAttemptPayload{}, ErrInvalidAttemptPayload
	}
	record := encryptedAttemptPayload{
		AttemptPayload: payload, SchemaVersion: attemptPayloadSchemaVersion,
		CipherVersion: attemptPayloadCipherVersion,
		DataNonce:     make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, record.DataNonce); err != nil {
		return encryptedAttemptPayload{}, ErrVaultUnavailable
	}
	aad, err := canonicalAttemptPayloadAAD(record)
	if err != nil {
		return encryptedAttemptPayload{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedAttemptPayload{}, ErrVaultUnavailable
	}
	record.Ciphertext = aead.Seal(nil, record.DataNonce, payload.Content, aad)
	record.Content = nil
	return record, nil
}

func decryptAttemptPayload(
	dek []byte,
	record encryptedAttemptPayload,
) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedAttemptPayload(record) {
		return nil, ErrAttemptPayloadAuthentication
	}
	aad, err := canonicalAttemptPayloadAAD(record)
	if err != nil {
		return nil, ErrAttemptPayloadAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrAttemptPayloadAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxAttemptPayloadBytes ||
		attemptPayloadDigest(plaintext) != record.Binding.ContentDigest {
		clearBytes(plaintext)
		return nil, ErrAttemptPayloadAuthentication
	}
	return plaintext, nil
}

func readEncryptedAttemptPayloadTx(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	payloadID string,
) (encryptedAttemptPayload, bool, error) {
	record := encryptedAttemptPayload{}
	record.Binding.PayloadID = payloadID
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(
		ctx,
		`SELECT conversation_id, work_item_id, run_id, claim_generation,
		        runtime_instance_id, execution_binding_digest, capsule_digest,
		        call_id, sequence, content_type, content_digest, status,
		        schema_version, cipher_version, ciphertext, data_nonce, aad_digest
		   FROM encrypted_attempt_payloads WHERE payload_id = ?`,
		payloadID,
	).Scan(
		&record.Binding.ConversationID, &record.Binding.WorkItemID,
		&record.Binding.RunID, &record.Binding.ClaimGeneration,
		&record.Binding.RuntimeInstanceID, &record.Binding.ExecutionBindingDigest,
		&record.Binding.CapsuleDigest, &record.Binding.CallID,
		&record.Binding.Sequence, &record.Binding.ContentType,
		&record.Binding.ContentDigest, &record.Status, &schemaVersion,
		&cipherVersion, &record.Ciphertext, &record.DataNonce, &record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedAttemptPayload{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedAttemptPayload{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	if !validEncryptedAttemptPayload(record) {
		return encryptedAttemptPayload{}, false, ErrAttemptPayloadAuthentication
	}
	return record, true, nil
}

func attemptPayloadSequenceExists(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	binding AttemptPayloadBinding,
) (bool, error) {
	var payloadID string
	err := querier.QueryRowContext(
		ctx,
		`SELECT payload_id FROM encrypted_attempt_payloads
		  WHERE run_id = ? AND claim_generation = ? AND sequence = ?`,
		binding.RunID, binding.ClaimGeneration, binding.Sequence,
	).Scan(&payloadID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil || !validContextCapsuleIdentifier(payloadID) {
		return false, ErrVaultUnavailable
	}
	return true, nil
}

func readPendingAttemptPayloads(
	ctx context.Context,
	querier interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	scope AttemptPayloadScope,
) ([]encryptedAttemptPayload, error) {
	rows, err := querier.QueryContext(
		ctx,
		`SELECT payload_id FROM encrypted_attempt_payloads
		  WHERE conversation_id = ? AND work_item_id = ? AND run_id = ?
		    AND claim_generation = ? AND runtime_instance_id = ?
		    AND execution_binding_digest = ? AND capsule_digest = ?
		  ORDER BY sequence, payload_id`,
		scope.ConversationID, scope.WorkItemID, scope.RunID,
		scope.ClaimGeneration, scope.RuntimeInstanceID,
		scope.ExecutionBindingDigest, scope.CapsuleDigest,
	)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer rows.Close()
	identifiers := make([]string, 0)
	for rows.Next() {
		var payloadID string
		if rows.Scan(&payloadID) != nil || !validContextCapsuleIdentifier(payloadID) {
			return nil, ErrAttemptPayloadAuthentication
		}
		identifiers = append(identifiers, payloadID)
	}
	if rows.Err() != nil {
		return nil, ErrVaultUnavailable
	}
	if err := rows.Close(); err != nil {
		return nil, ErrVaultUnavailable
	}
	if len(identifiers) == 0 {
		conflict, conflictErr := attemptPayloadScopeConflictExists(
			ctx, querier, scope,
		)
		if conflictErr != nil {
			return nil, conflictErr
		}
		if conflict {
			return nil, ErrAttemptPayloadBinding
		}
	}
	records := make([]encryptedAttemptPayload, 0, len(identifiers))
	for _, payloadID := range identifiers {
		record, found, readErr := readEncryptedAttemptPayloadTx(ctx, querier, payloadID)
		if readErr != nil || !found || record.Binding.Scope != scope {
			return nil, errors.Join(ErrAttemptPayloadAuthentication, readErr)
		}
		records = append(records, record)
	}
	return records, nil
}

func attemptPayloadScopeConflictExists(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	scope AttemptPayloadScope,
) (bool, error) {
	var runExists, scopeExists int
	err := querier.QueryRowContext(
		ctx,
		`SELECT
			EXISTS(SELECT 1 FROM encrypted_attempt_payloads WHERE run_id = ?),
			EXISTS(SELECT 1 FROM encrypted_attempt_payloads
			 WHERE conversation_id = ? AND work_item_id = ? AND run_id = ?
			   AND claim_generation = ? AND runtime_instance_id = ?
			   AND execution_binding_digest = ? AND capsule_digest = ?)`,
		scope.RunID, scope.ConversationID, scope.WorkItemID, scope.RunID,
		scope.ClaimGeneration, scope.RuntimeInstanceID,
		scope.ExecutionBindingDigest, scope.CapsuleDigest,
	).Scan(&runExists, &scopeExists)
	if err != nil || (runExists != 0 && runExists != 1) ||
		(scopeExists != 0 && scopeExists != 1) {
		return false, ErrVaultUnavailable
	}
	return runExists == 1 && scopeExists == 0, nil
}

func canonicalAttemptPayloadAAD(record encryptedAttemptPayload) ([]byte, error) {
	if record.SchemaVersion != attemptPayloadSchemaVersion ||
		record.CipherVersion != attemptPayloadCipherVersion ||
		!validAttemptPayloadBinding(record.Binding) || !validAttemptPayloadStatus(record.Status) {
		return nil, ErrInvalidAttemptPayload
	}
	var body bytes.Buffer
	body.WriteString("LOOM-ATTEMPT-PAYLOAD-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.Binding.PayloadID)
	writeCanonicalString(&body, record.Binding.ConversationID)
	writeCanonicalString(&body, record.Binding.WorkItemID)
	writeCanonicalString(&body, record.Binding.RunID)
	_ = binary.Write(&body, binary.BigEndian, record.Binding.ClaimGeneration)
	writeCanonicalString(&body, record.Binding.RuntimeInstanceID)
	writeCanonicalString(&body, record.Binding.ExecutionBindingDigest)
	writeCanonicalString(&body, record.Binding.CapsuleDigest)
	writeCanonicalString(&body, record.Binding.CallID)
	_ = binary.Write(&body, binary.BigEndian, record.Binding.Sequence)
	writeCanonicalString(&body, record.Binding.ContentType)
	writeCanonicalString(&body, record.Binding.ContentDigest)
	writeCanonicalString(&body, string(record.Status))
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func validAttemptPayload(payload AttemptPayload) bool {
	return validAttemptPayloadBinding(payload.Binding) &&
		validAttemptPayloadStatus(payload.Status) && len(payload.Content) > 0 &&
		len(payload.Content) <= maxAttemptPayloadBytes && utf8.Valid(payload.Content) &&
		attemptPayloadDigest(payload.Content) == payload.Binding.ContentDigest
}

func validEncryptedAttemptPayload(record encryptedAttemptPayload) bool {
	return record.SchemaVersion == attemptPayloadSchemaVersion &&
		record.CipherVersion == attemptPayloadCipherVersion &&
		validAttemptPayloadBinding(record.Binding) &&
		validAttemptPayloadStatus(record.Status) && len(record.Ciphertext) > 16 &&
		len(record.Ciphertext) <= maxAttemptPayloadBytes+16 &&
		len(record.DataNonce) == vaultNonceBytes &&
		len(record.AADDigest) == vaultAADDigestBytes
}

func validAttemptPayloadBinding(binding AttemptPayloadBinding) bool {
	return validContextCapsuleIdentifier(binding.PayloadID) &&
		validAttemptPayloadScope(binding.Scope) &&
		validContextCapsuleIdentifier(binding.CallID) && binding.Sequence > 0 &&
		validAttemptPayloadContentType(binding.ContentType) &&
		validContextCapsuleDigest(binding.ContentDigest)
}

func sameAttemptPayloadDeliveryIdentity(left, right AttemptPayloadBinding) bool {
	left.ContentDigest = ""
	right.ContentDigest = ""
	return left == right
}

func validAttemptPayloadScope(scope AttemptPayloadScope) bool {
	return validContextCapsuleIdentifier(scope.ConversationID) &&
		validContextCapsuleIdentifier(scope.WorkItemID) &&
		validContextCapsuleIdentifier(scope.RunID) && scope.ClaimGeneration > 0 &&
		validContextCapsuleIdentifier(scope.RuntimeInstanceID) &&
		validContextCapsuleDigest(scope.ExecutionBindingDigest) &&
		validContextCapsuleDigest(scope.CapsuleDigest)
}

func validAttemptPayloadStatus(status AttemptPayloadStatus) bool {
	return status == AttemptPayloadPending || status == AttemptPayloadDelivered
}

func validAttemptPayloadContentType(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '/' || character == '+' || character == '.' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func attemptPayloadDigest(content []byte) string {
	digest := sha256.Sum256(content)
	const alphabet = "0123456789abcdef"
	result := make([]byte, sha256.Size*2)
	for index, value := range digest {
		result[index*2] = alphabet[value>>4]
		result[index*2+1] = alphabet[value&0x0f]
	}
	return string(result)
}

func clearAttemptPayloadResults(payloads []AttemptPayload) {
	for index := range payloads {
		payloads[index].Close()
	}
}
