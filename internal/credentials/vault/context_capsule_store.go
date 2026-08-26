package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
)

const (
	contextCapsuleSchemaVersion = uint16(1)
	contextCapsuleCipherVersion = uint16(1)
	contextCapsuleEnvelopeV1    = 1
	contextCapsuleEnvelopeV2    = 2
	maxContextCapsulePayload    = 256 << 10
)

var (
	ErrInvalidContextCapsuleStore   = errors.New("invalid encrypted Context Capsule")
	ErrContextCapsuleNotFound       = errors.New("encrypted Context Capsule not found")
	ErrContextCapsuleAuthentication = errors.New("encrypted Context Capsule authentication failed")
)

type StoredContextCapsule struct {
	Authority        contextcapsule.AuthorityRecord
	Capsule          contextcapsule.RoleContextCapsule
	CapsuleAvailable bool
	EnvelopeVersion  int
	DispatchPayload  []byte
}

type contextCapsuleEnvelope struct {
	SchemaVersion      int             `json:"schema_version"`
	CapsuleBody        json.RawMessage `json:"capsule_body"`
	RetrievableContext json.RawMessage `json:"retrievable_context,omitempty"`
	DispatchPayload    json.RawMessage `json:"dispatch_payload"`
}

type encryptedConversationKey struct {
	ConversationID string
	SchemaVersion  uint16
	CipherVersion  uint16
	KeyVersion     uint32
	WrappedDEK     []byte
	WrapNonce      []byte
	AADDigest      []byte
}

type encryptedContextCapsule struct {
	ConversationID          string
	CapsuleDigest           string
	DisclosureReceiptDigest string
	SchemaVersion           uint16
	CipherVersion           uint16
	Authority               contextcapsule.AuthorityRecord
	AuthorityJSON           []byte
	Ciphertext              []byte
	DataNonce               []byte
	AADDigest               []byte
}

func (store *VaultStore) PutContextCapsule(
	ctx context.Context,
	authority contextcapsule.AuthorityRecord,
	dispatchPayload []byte,
) error {
	validated, authorityJSON, err := validateContextCapsuleInput(authority, dispatchPayload)
	if err != nil || store == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidContextCapsuleStore
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

	conversationKey, dek, err := store.readOrCreateConversationDEKTx(
		ctx, transaction, vmk, validated.ConversationID,
	)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	if conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return ErrVaultUnavailable
	}

	existing, found, err := readEncryptedContextCapsuleTx(
		ctx, transaction, validated.ConversationID, validated.CapsuleDigest,
	)
	if err != nil {
		return err
	}
	if found {
		plaintext, decryptErr := decryptContextCapsulePayload(dek, existing)
		if decryptErr != nil {
			return decryptErr
		}
		same := bytes.Equal(plaintext, dispatchPayload) &&
			bytes.Equal(existing.AuthorityJSON, authorityJSON)
		existingStored, existingDecodeErr := decodeStoredContextCapsule(
			existing.Authority, plaintext,
		)
		incomingStored, incomingDecodeErr := decodeStoredContextCapsule(
			validated, dispatchPayload,
		)
		clearBytes(plaintext)
		defer clearBytes(existingStored.DispatchPayload)
		defer clearBytes(incomingStored.DispatchPayload)
		if !same {
			if existingDecodeErr != nil || incomingDecodeErr != nil ||
				existingStored.Authority != incomingStored.Authority ||
				!validContextCapsuleEnvelopeUpgrade(existingStored, incomingStored) ||
				!bytes.Equal(
					existingStored.DispatchPayload,
					incomingStored.DispatchPayload,
				) {
				return ErrContextCapsuleAuthentication
			}
			upgraded, encryptErr := store.encryptContextCapsule(
				validated, authorityJSON, dispatchPayload, dek,
			)
			if encryptErr != nil {
				return encryptErr
			}
			if err := reserveConversationDataNonceTx(
				ctx, transaction, upgraded.ConversationID, upgraded.DataNonce,
				"context-capsule", upgraded.CapsuleDigest,
			); err != nil {
				return err
			}
			result, updateErr := transaction.ExecContext(
				ctx,
				`UPDATE encrypted_context_capsules
				    SET ciphertext = ?, data_nonce = ?, aad_digest = ?
				  WHERE capsule_digest = ? AND conversation_id = ?
				    AND data_nonce = ?`,
				upgraded.Ciphertext, upgraded.DataNonce, upgraded.AADDigest,
				existing.CapsuleDigest, existing.ConversationID, existing.DataNonce,
			)
			if updateErr != nil || !exactlyOneRow(result) || transaction.Commit() != nil {
				return ErrVaultUnavailable
			}
			return nil
		}
		return nil
	}

	record, err := store.encryptContextCapsule(
		validated, authorityJSON, dispatchPayload, dek,
	)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, transaction, record.ConversationID, record.DataNonce,
		"context-capsule", record.CapsuleDigest,
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	_, err = transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_context_capsules (
			capsule_digest, conversation_id, disclosure_receipt_digest,
			schema_version, cipher_version, authority_record,
			ciphertext, data_nonce, aad_digest, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.CapsuleDigest, record.ConversationID,
		record.DisclosureReceiptDigest, record.SchemaVersion,
		record.CipherVersion, record.AuthorityJSON, record.Ciphertext,
		record.DataNonce, record.AADDigest, now,
	)
	if err != nil || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func validContextCapsuleEnvelopeUpgrade(
	existing StoredContextCapsule,
	incoming StoredContextCapsule,
) bool {
	if !incoming.CapsuleAvailable || incoming.EnvelopeVersion != contextCapsuleEnvelopeV2 {
		return false
	}
	return !existing.CapsuleAvailable && existing.EnvelopeVersion == 0 ||
		existing.CapsuleAvailable && existing.EnvelopeVersion == contextCapsuleEnvelopeV1
}

func (store *VaultStore) PutRoleContextCapsule(
	ctx context.Context,
	capsule contextcapsule.RoleContextCapsule,
	dispatchPayload []byte,
) error {
	if !capsule.Valid() {
		return ErrInvalidContextCapsuleStore
	}
	body, err := contextcapsule.MarshalCanonicalRoleContextCapsule(capsule)
	if err != nil {
		return ErrInvalidContextCapsuleStore
	}
	defer clearBytes(body)
	retrievable, err := contextcapsule.MarshalCanonicalRetrievableContext(capsule)
	if err != nil {
		return ErrInvalidContextCapsuleStore
	}
	defer clearBytes(retrievable)
	if _, err := contextcapsule.ValidateDispatchPayload(
		capsule, dispatchPayload,
	); err != nil {
		return ErrInvalidContextCapsuleStore
	}
	envelope, err := json.Marshal(contextCapsuleEnvelope{
		SchemaVersion:      contextCapsuleEnvelopeV2,
		CapsuleBody:        append(json.RawMessage(nil), body...),
		RetrievableContext: append(json.RawMessage(nil), retrievable...),
		DispatchPayload:    append(json.RawMessage(nil), dispatchPayload...),
	})
	if err != nil || len(envelope) == 0 || len(envelope) > maxContextCapsulePayload {
		clearBytes(envelope)
		return ErrInvalidContextCapsuleStore
	}
	defer clearBytes(envelope)
	return store.PutContextCapsule(
		ctx, capsule.AuthorityRecord(), envelope,
	)
}

func (store *VaultStore) DeleteRoleContextCapsule(
	ctx context.Context,
	authority contextcapsule.AuthorityRecord,
) error {
	validated, err := contextcapsule.ValidateAuthorityRecord(authority)
	if err != nil || store == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidContextCapsuleStore
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
	record, found, err := readEncryptedContextCapsuleTx(
		ctx, transaction, validated.ConversationID, validated.CapsuleDigest,
	)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if record.Authority != validated ||
		record.DisclosureReceiptDigest != validated.DisclosureReceiptDigest {
		return ErrContextCapsuleAuthentication
	}
	result, err := transaction.ExecContext(
		ctx,
		`DELETE FROM encrypted_context_capsules
		  WHERE capsule_digest = ? AND conversation_id = ?
		    AND disclosure_receipt_digest = ? AND authority_record = ?`,
		validated.CapsuleDigest, validated.ConversationID,
		validated.DisclosureReceiptDigest, record.AuthorityJSON,
	)
	if err != nil || !exactlyOneRow(result) || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ReadContextCapsule(
	ctx context.Context,
	conversationID string,
	capsuleDigest string,
) (StoredContextCapsule, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validContextCapsuleIdentifier(conversationID) ||
		!validContextCapsuleDigest(capsuleDigest) {
		return StoredContextCapsule{}, ErrInvalidContextCapsuleStore
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return StoredContextCapsule{}, ErrVaultUnavailable
	}
	conversationKey, err := readEncryptedConversationKey(
		ctx, store.database, conversationID,
	)
	if err != nil {
		return StoredContextCapsule{}, err
	}
	if conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return StoredContextCapsule{}, ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return StoredContextCapsule{}, ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return StoredContextCapsule{}, err
	}
	defer clearBytes(dek)
	record, found, err := readEncryptedContextCapsule(
		ctx, store.database, conversationID, capsuleDigest,
	)
	if err != nil {
		return StoredContextCapsule{}, err
	}
	if !found {
		return StoredContextCapsule{}, ErrContextCapsuleNotFound
	}
	plaintext, err := decryptContextCapsulePayload(dek, record)
	if err != nil {
		return StoredContextCapsule{}, err
	}
	stored, err := decodeStoredContextCapsule(record.Authority, plaintext)
	clearBytes(plaintext)
	if err != nil {
		return StoredContextCapsule{}, err
	}
	return stored, nil
}

func (store *VaultStore) RetrieveContextItem(
	ctx context.Context,
	request contextcapsule.RetrievalRequest,
) (contextcapsule.RetrievedItem, error) {
	if store == nil || ctx == nil || ctx.Err() != nil {
		return contextcapsule.RetrievedItem{}, ErrInvalidContextCapsuleStore
	}
	stored, err := store.ReadContextCapsule(
		ctx, request.Authority.ConversationID, request.Authority.CapsuleDigest,
	)
	if err != nil {
		if errors.Is(err, ErrContextCapsuleNotFound) {
			return contextcapsule.RetrievedItem{}, errors.Join(
				contextcapsule.ErrContextItemNotRetrievable, err,
			)
		}
		return contextcapsule.RetrievedItem{}, err
	}
	defer clearBytes(stored.DispatchPayload)
	if !stored.CapsuleAvailable || stored.Authority != request.Authority {
		return contextcapsule.RetrievedItem{}, contextcapsule.ErrContextRetrievalDenied
	}
	return stored.Capsule.Retrieve(request)
}

func (store *VaultStore) ListContextCapsuleAuthorities(
	ctx context.Context,
	conversationID string,
) ([]contextcapsule.AuthorityRecord, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validContextCapsuleIdentifier(conversationID) {
		return nil, ErrInvalidContextCapsuleStore
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return nil, ErrVaultUnavailable
	}
	rows, err := store.database.QueryContext(
		ctx,
		`SELECT authority_record
		   FROM encrypted_context_capsules
		  WHERE conversation_id = ?
		  ORDER BY capsule_digest`,
		conversationID,
	)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer rows.Close()
	records := make([]contextcapsule.AuthorityRecord, 0)
	for rows.Next() {
		var encoded []byte
		if rows.Scan(&encoded) != nil || len(encoded) == 0 {
			return nil, ErrContextCapsuleAuthentication
		}
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		var record contextcapsule.AuthorityRecord
		if decoder.Decode(&record) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			return nil, ErrContextCapsuleAuthentication
		}
		validated, validateErr := contextcapsule.ValidateAuthorityRecord(record)
		if validateErr != nil || validated != record ||
			record.ConversationID != conversationID {
			return nil, ErrContextCapsuleAuthentication
		}
		records = append(records, record)
	}
	if rows.Err() != nil {
		return nil, ErrVaultUnavailable
	}
	return records, nil
}

func (store *VaultStore) DeleteContextConversation(
	ctx context.Context,
	conversationID string,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validContextCapsuleIdentifier(conversationID) {
		return ErrInvalidContextCapsuleStore
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
	result, err := transaction.ExecContext(
		ctx,
		`DELETE FROM encrypted_conversation_keys WHERE conversation_id = ?`,
		conversationID,
	)
	if err != nil {
		return ErrVaultUnavailable
	}
	count, err := result.RowsAffected()
	if err != nil {
		return ErrVaultUnavailable
	}
	if count == 0 {
		return ErrContextCapsuleNotFound
	}
	if count != 1 || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) readOrCreateConversationDEKTx(
	ctx context.Context,
	transaction *sql.Tx,
	vmk []byte,
	conversationID string,
) (encryptedConversationKey, []byte, error) {
	record, err := readEncryptedConversationKey(ctx, transaction, conversationID)
	if err == nil {
		dek, unwrapErr := unwrapConversationDEK(vmk, record)
		return record, dek, unwrapErr
	}
	if !errors.Is(err, ErrContextCapsuleNotFound) {
		return encryptedConversationKey{}, nil, err
	}
	var dek []byte
	record, dek, err = store.newEncryptedConversationKey(
		vmk, conversationID, store.keyMaterial.KeyVersion(),
	)
	if err != nil {
		return encryptedConversationKey{}, nil, err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	_, err = transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_conversation_keys (
			conversation_id, schema_version, cipher_version, key_version,
			wrapped_dek, wrap_nonce, aad_digest, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ConversationID, record.SchemaVersion, record.CipherVersion,
		record.KeyVersion, record.WrappedDEK, record.WrapNonce,
		record.AADDigest, now, now,
	)
	if err != nil {
		clearBytes(dek)
		return encryptedConversationKey{}, nil, ErrVaultUnavailable
	}
	return record, dek, nil
}

func (store *VaultStore) newEncryptedConversationKey(
	vmk []byte,
	conversationID string,
	keyVersion uint32,
) (encryptedConversationKey, []byte, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(vmk) != vaultKeyBytes || keyVersion == 0 ||
		!validContextCapsuleIdentifier(conversationID) {
		return encryptedConversationKey{}, nil, ErrInvalidContextCapsuleStore
	}
	record := encryptedConversationKey{
		ConversationID: conversationID,
		SchemaVersion:  contextCapsuleSchemaVersion,
		CipherVersion:  contextCapsuleCipherVersion,
		KeyVersion:     keyVersion,
		WrapNonce:      make([]byte, vaultNonceBytes),
	}
	dek := make([]byte, vaultKeyBytes)
	if _, err := io.ReadFull(store.cipher.random, dek); err != nil {
		clearBytes(dek)
		return encryptedConversationKey{}, nil, ErrVaultUnavailable
	}
	if _, err := io.ReadFull(store.cipher.random, record.WrapNonce); err != nil {
		clearBytes(dek)
		return encryptedConversationKey{}, nil, ErrVaultUnavailable
	}
	aad, err := canonicalConversationKeyAAD(record)
	if err != nil {
		clearBytes(dek)
		return encryptedConversationKey{}, nil, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	kek, err := DeriveDomainKey(vmk, ConversationWrapDomain)
	if err != nil {
		clearBytes(dek)
		return encryptedConversationKey{}, nil, ErrVaultUnavailable
	}
	defer clearBytes(kek)
	aead, err := newVaultAEAD(kek)
	if err != nil {
		clearBytes(dek)
		return encryptedConversationKey{}, nil, ErrVaultUnavailable
	}
	record.WrappedDEK = aead.Seal(nil, record.WrapNonce, dek, aad)
	return record, dek, nil
}

func (store *VaultStore) rotateConversationKeysTx(
	ctx context.Context,
	transaction *sql.Tx,
	oldVMK []byte,
	newVMK []byte,
	oldKeyVersion uint32,
	newKeyVersion uint32,
) error {
	if store == nil || transaction == nil || store.cipher == nil ||
		store.cipher.random == nil || len(oldVMK) != vaultKeyBytes ||
		len(newVMK) != vaultKeyBytes || oldKeyVersion == 0 ||
		newKeyVersion <= oldKeyVersion {
		return ErrInvalidContextCapsuleStore
	}
	rows, err := transaction.QueryContext(
		ctx,
		`SELECT conversation_id, schema_version, cipher_version, key_version,
		        wrapped_dek, wrap_nonce, aad_digest
		   FROM encrypted_conversation_keys ORDER BY conversation_id`,
	)
	if err != nil {
		return ErrVaultUnavailable
	}
	records := make([]encryptedConversationKey, 0)
	for rows.Next() {
		var record encryptedConversationKey
		var schemaVersion, cipherVersion, keyVersion int64
		if err := rows.Scan(
			&record.ConversationID, &schemaVersion, &cipherVersion, &keyVersion,
			&record.WrappedDEK, &record.WrapNonce, &record.AADDigest,
		); err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
			cipherVersion < 0 || cipherVersion > 1<<16-1 ||
			keyVersion < 0 || keyVersion > 1<<32-1 {
			rows.Close()
			return ErrVaultUnavailable
		}
		record.SchemaVersion = uint16(schemaVersion)
		record.CipherVersion = uint16(cipherVersion)
		record.KeyVersion = uint32(keyVersion)
		if !validEncryptedConversationKey(record) ||
			record.KeyVersion != oldKeyVersion {
			rows.Close()
			return ErrContextCapsuleAuthentication
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ErrVaultUnavailable
	}
	if err := rows.Close(); err != nil {
		return ErrVaultUnavailable
	}
	newKEK, err := DeriveDomainKey(newVMK, ConversationWrapDomain)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer clearBytes(newKEK)
	newAEAD, err := newVaultAEAD(newKEK)
	if err != nil {
		return ErrVaultUnavailable
	}
	updatedAt := store.now().UTC().Format(time.RFC3339Nano)
	for _, record := range records {
		dek, err := unwrapConversationDEK(oldVMK, record)
		if err != nil {
			return err
		}
		rotated := record
		rotated.KeyVersion = newKeyVersion
		rotated.WrapNonce = make([]byte, vaultNonceBytes)
		if _, err := io.ReadFull(store.cipher.random, rotated.WrapNonce); err != nil {
			clearBytes(dek)
			return ErrVaultUnavailable
		}
		aad, err := canonicalConversationKeyAAD(rotated)
		if err != nil {
			clearBytes(dek)
			return err
		}
		digest := sha256.Sum256(aad)
		rotated.AADDigest = append([]byte(nil), digest[:]...)
		rotated.WrappedDEK = newAEAD.Seal(nil, rotated.WrapNonce, dek, aad)
		clearBytes(dek)
		result, err := transaction.ExecContext(
			ctx,
			`UPDATE encrypted_conversation_keys SET
				key_version = ?, wrapped_dek = ?, wrap_nonce = ?,
				aad_digest = ?, updated_at = ?
			  WHERE conversation_id = ? AND key_version = ?`,
			rotated.KeyVersion, rotated.WrappedDEK, rotated.WrapNonce,
			rotated.AADDigest, updatedAt, record.ConversationID,
			record.KeyVersion,
		)
		if err != nil || !exactlyOneRow(result) {
			return ErrVaultUnavailable
		}
	}
	return nil
}

func (store *VaultStore) encryptContextCapsule(
	authority contextcapsule.AuthorityRecord,
	authorityJSON []byte,
	dispatchPayload []byte,
	dek []byte,
) (encryptedContextCapsule, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes {
		return encryptedContextCapsule{}, ErrInvalidContextCapsuleStore
	}
	record := encryptedContextCapsule{
		ConversationID:          authority.ConversationID,
		CapsuleDigest:           authority.CapsuleDigest,
		DisclosureReceiptDigest: authority.DisclosureReceiptDigest,
		SchemaVersion:           contextCapsuleSchemaVersion,
		CipherVersion:           contextCapsuleCipherVersion,
		Authority:               authority,
		AuthorityJSON:           append([]byte(nil), authorityJSON...),
		DataNonce:               make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, record.DataNonce); err != nil {
		return encryptedContextCapsule{}, ErrVaultUnavailable
	}
	aad, err := canonicalContextCapsuleAAD(record)
	if err != nil {
		return encryptedContextCapsule{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedContextCapsule{}, ErrVaultUnavailable
	}
	record.Ciphertext = aead.Seal(nil, record.DataNonce, dispatchPayload, aad)
	return record, nil
}

func unwrapConversationDEK(
	vmk []byte,
	record encryptedConversationKey,
) ([]byte, error) {
	if len(vmk) != vaultKeyBytes || !validEncryptedConversationKey(record) {
		return nil, ErrContextCapsuleAuthentication
	}
	aad, err := canonicalConversationKeyAAD(record)
	if err != nil {
		return nil, ErrContextCapsuleAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrContextCapsuleAuthentication
	}
	kek, err := DeriveDomainKey(vmk, ConversationWrapDomain)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer clearBytes(kek)
	aead, err := newVaultAEAD(kek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	dek, err := aead.Open(nil, record.WrapNonce, record.WrappedDEK, aad)
	if err != nil || len(dek) != vaultKeyBytes {
		clearBytes(dek)
		return nil, ErrContextCapsuleAuthentication
	}
	return dek, nil
}

func decryptContextCapsulePayload(
	dek []byte,
	record encryptedContextCapsule,
) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedContextCapsule(record) {
		return nil, ErrContextCapsuleAuthentication
	}
	aad, err := canonicalContextCapsuleAAD(record)
	if err != nil {
		return nil, ErrContextCapsuleAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrContextCapsuleAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxContextCapsulePayload {
		clearBytes(plaintext)
		return nil, ErrContextCapsuleAuthentication
	}
	stored, err := decodeStoredContextCapsule(record.Authority, plaintext)
	if err != nil {
		clearBytes(plaintext)
		return nil, ErrContextCapsuleAuthentication
	}
	clearBytes(stored.DispatchPayload)
	return plaintext, nil
}

func validateContextCapsuleInput(
	authority contextcapsule.AuthorityRecord,
	dispatchPayload []byte,
) (contextcapsule.AuthorityRecord, []byte, error) {
	validated, err := contextcapsule.ValidateAuthorityRecord(authority)
	if err != nil || len(dispatchPayload) == 0 ||
		len(dispatchPayload) > maxContextCapsulePayload {
		return contextcapsule.AuthorityRecord{}, nil, ErrInvalidContextCapsuleStore
	}
	stored, err := decodeStoredContextCapsule(validated, dispatchPayload)
	if err != nil {
		return contextcapsule.AuthorityRecord{}, nil, ErrInvalidContextCapsuleStore
	}
	clearBytes(stored.DispatchPayload)
	authorityJSON, err := json.Marshal(validated)
	if err != nil {
		return contextcapsule.AuthorityRecord{}, nil, ErrInvalidContextCapsuleStore
	}
	return validated, authorityJSON, nil
}

func decodeStoredContextCapsule(
	authority contextcapsule.AuthorityRecord,
	plaintext []byte,
) (StoredContextCapsule, error) {
	validated, err := contextcapsule.ValidateAuthorityRecord(authority)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxContextCapsulePayload {
		return StoredContextCapsule{}, ErrContextCapsuleAuthentication
	}
	if dispatch, dispatchErr := contextcapsule.DecodeDispatchPayload(plaintext); dispatchErr == nil {
		if dispatch.CapsuleDigest != validated.CapsuleDigest ||
			dispatch.DisclosureReceiptDigest != validated.DisclosureReceiptDigest {
			return StoredContextCapsule{}, ErrContextCapsuleAuthentication
		}
		return StoredContextCapsule{
			Authority: validated, DispatchPayload: dispatch.CanonicalPayload(),
		}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	var envelope contextCapsuleEnvelope
	if decoder.Decode(&envelope) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		(envelope.SchemaVersion != contextCapsuleEnvelopeV1 &&
			envelope.SchemaVersion != contextCapsuleEnvelopeV2) ||
		len(envelope.CapsuleBody) == 0 || len(envelope.DispatchPayload) == 0 {
		return StoredContextCapsule{}, ErrContextCapsuleAuthentication
	}
	canonical, err := json.Marshal(envelope)
	if err != nil || !bytes.Equal(canonical, plaintext) {
		return StoredContextCapsule{}, ErrContextCapsuleAuthentication
	}
	capsule, err := contextcapsule.RestoreRoleContextCapsule(
		validated, envelope.CapsuleBody,
	)
	if err != nil {
		return StoredContextCapsule{}, ErrContextCapsuleAuthentication
	}
	if envelope.SchemaVersion == contextCapsuleEnvelopeV1 &&
		len(envelope.RetrievableContext) != 0 ||
		envelope.SchemaVersion == contextCapsuleEnvelopeV2 &&
			len(envelope.RetrievableContext) == 0 {
		return StoredContextCapsule{}, ErrContextCapsuleAuthentication
	}
	if envelope.SchemaVersion == contextCapsuleEnvelopeV2 {
		capsule, err = contextcapsule.RestoreRetrievableContext(
			capsule, envelope.RetrievableContext,
		)
		if err != nil {
			return StoredContextCapsule{}, ErrContextCapsuleAuthentication
		}
	}
	dispatch, err := contextcapsule.ValidateDispatchPayload(
		capsule, envelope.DispatchPayload,
	)
	if err != nil {
		return StoredContextCapsule{}, ErrContextCapsuleAuthentication
	}
	return StoredContextCapsule{
		Authority: validated, Capsule: capsule, CapsuleAvailable: true,
		EnvelopeVersion: envelope.SchemaVersion,
		DispatchPayload: dispatch.CanonicalPayload(),
	}, nil
}

func readEncryptedConversationKey(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	conversationID string,
) (encryptedConversationKey, error) {
	record := encryptedConversationKey{ConversationID: conversationID}
	var schemaVersion, cipherVersion, keyVersion int64
	err := querier.QueryRowContext(
		ctx,
		`SELECT schema_version, cipher_version, key_version,
		        wrapped_dek, wrap_nonce, aad_digest
		   FROM encrypted_conversation_keys WHERE conversation_id = ?`,
		conversationID,
	).Scan(
		&schemaVersion, &cipherVersion, &keyVersion, &record.WrappedDEK,
		&record.WrapNonce, &record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedConversationKey{}, ErrContextCapsuleNotFound
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 ||
		keyVersion < 0 || keyVersion > 1<<32-1 {
		return encryptedConversationKey{}, ErrVaultUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	record.KeyVersion = uint32(keyVersion)
	if !validEncryptedConversationKey(record) {
		return encryptedConversationKey{}, ErrContextCapsuleAuthentication
	}
	return record, nil
}

func readEncryptedContextCapsule(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	conversationID string,
	capsuleDigest string,
) (encryptedContextCapsule, bool, error) {
	return readEncryptedContextCapsuleTx(
		ctx, querier, conversationID, capsuleDigest,
	)
}

func readEncryptedContextCapsuleTx(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	conversationID string,
	capsuleDigest string,
) (encryptedContextCapsule, bool, error) {
	record := encryptedContextCapsule{
		ConversationID: conversationID, CapsuleDigest: capsuleDigest,
	}
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(
		ctx,
		`SELECT disclosure_receipt_digest, schema_version, cipher_version,
		        authority_record, ciphertext, data_nonce, aad_digest
		   FROM encrypted_context_capsules
		  WHERE conversation_id = ? AND capsule_digest = ?`,
		conversationID, capsuleDigest,
	).Scan(
		&record.DisclosureReceiptDigest, &schemaVersion, &cipherVersion,
		&record.AuthorityJSON, &record.Ciphertext, &record.DataNonce,
		&record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedContextCapsule{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedContextCapsule{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	authority, err := decodeContextCapsuleAuthority(record.AuthorityJSON)
	if err != nil {
		return encryptedContextCapsule{}, false, ErrContextCapsuleAuthentication
	}
	record.Authority = authority
	if !validEncryptedContextCapsule(record) {
		return encryptedContextCapsule{}, false, ErrContextCapsuleAuthentication
	}
	return record, true, nil
}

func decodeContextCapsuleAuthority(
	encoded []byte,
) (contextcapsule.AuthorityRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var record contextcapsule.AuthorityRecord
	if decoder.Decode(&record) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return contextcapsule.AuthorityRecord{}, ErrContextCapsuleAuthentication
	}
	validated, err := contextcapsule.ValidateAuthorityRecord(record)
	if err != nil {
		return contextcapsule.AuthorityRecord{}, ErrContextCapsuleAuthentication
	}
	canonical, err := json.Marshal(validated)
	if err != nil || !bytes.Equal(canonical, encoded) {
		return contextcapsule.AuthorityRecord{}, ErrContextCapsuleAuthentication
	}
	return validated, nil
}

func canonicalConversationKeyAAD(
	record encryptedConversationKey,
) ([]byte, error) {
	if record.SchemaVersion != contextCapsuleSchemaVersion ||
		record.CipherVersion != contextCapsuleCipherVersion ||
		record.KeyVersion == 0 ||
		!validContextCapsuleIdentifier(record.ConversationID) {
		return nil, ErrInvalidContextCapsuleStore
	}
	var body bytes.Buffer
	body.WriteString("LOOM-CONVERSATION-KEY-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.ConversationID)
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func canonicalContextCapsuleAAD(
	record encryptedContextCapsule,
) ([]byte, error) {
	if record.SchemaVersion != contextCapsuleSchemaVersion ||
		record.CipherVersion != contextCapsuleCipherVersion ||
		!validContextCapsuleIdentifier(record.ConversationID) ||
		!validContextCapsuleDigest(record.CapsuleDigest) ||
		!validContextCapsuleDigest(record.DisclosureReceiptDigest) ||
		record.Authority.ConversationID != record.ConversationID ||
		record.Authority.CapsuleDigest != record.CapsuleDigest ||
		record.Authority.DisclosureReceiptDigest != record.DisclosureReceiptDigest {
		return nil, ErrInvalidContextCapsuleStore
	}
	validated, err := contextcapsule.ValidateAuthorityRecord(record.Authority)
	if err != nil {
		return nil, ErrInvalidContextCapsuleStore
	}
	canonical, err := json.Marshal(validated)
	if err != nil || !bytes.Equal(canonical, record.AuthorityJSON) {
		return nil, ErrInvalidContextCapsuleStore
	}
	var body bytes.Buffer
	body.WriteString("LOOM-CONTEXT-CAPSULE-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.ConversationID)
	writeCanonicalString(&body, record.CapsuleDigest)
	writeCanonicalString(&body, record.DisclosureReceiptDigest)
	writeCanonicalString(&body, string(canonical))
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func validEncryptedConversationKey(record encryptedConversationKey) bool {
	return record.SchemaVersion == contextCapsuleSchemaVersion &&
		record.CipherVersion == contextCapsuleCipherVersion &&
		record.KeyVersion > 0 &&
		validContextCapsuleIdentifier(record.ConversationID) &&
		len(record.WrappedDEK) == vaultKeyBytes+16 &&
		len(record.WrapNonce) == vaultNonceBytes &&
		len(record.AADDigest) == vaultAADDigestBytes
}

func validEncryptedContextCapsule(record encryptedContextCapsule) bool {
	if record.SchemaVersion != contextCapsuleSchemaVersion ||
		record.CipherVersion != contextCapsuleCipherVersion ||
		!validContextCapsuleIdentifier(record.ConversationID) ||
		!validContextCapsuleDigest(record.CapsuleDigest) ||
		!validContextCapsuleDigest(record.DisclosureReceiptDigest) ||
		len(record.Ciphertext) <= 16 ||
		len(record.Ciphertext) > maxContextCapsulePayload+16 ||
		len(record.DataNonce) != vaultNonceBytes ||
		len(record.AADDigest) != vaultAADDigestBytes ||
		record.Authority.ConversationID != record.ConversationID ||
		record.Authority.CapsuleDigest != record.CapsuleDigest ||
		record.Authority.DisclosureReceiptDigest != record.DisclosureReceiptDigest {
		return false
	}
	validated, err := contextcapsule.ValidateAuthorityRecord(record.Authority)
	return err == nil && validated == record.Authority
}

func validContextCapsuleIdentifier(value string) bool {
	return value != "" && value == strings.TrimSpace(value) &&
		len(value) <= 512 && !strings.ContainsAny(value, "\x00\r\n")
}

func validContextCapsuleDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
