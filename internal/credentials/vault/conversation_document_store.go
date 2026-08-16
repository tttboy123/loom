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
)

const (
	conversationDocumentSchemaVersion = uint16(1)
	conversationDocumentCipherVersion = uint16(1)
	maxConversationDocumentPayload    = 4 << 20
)

var (
	ErrInvalidConversationDocument        = errors.New("invalid encrypted Conversation document")
	ErrConversationDocumentConflict       = errors.New("encrypted Conversation document conflict")
	ErrConversationDocumentNotFound       = errors.New("encrypted Conversation document not found")
	ErrConversationDocumentAuthentication = errors.New("encrypted Conversation document authentication failed")
)

type ConversationDocument struct {
	ConversationID string
	Kind           string
	Revision       int64
	Payload        []byte
}

type encryptedConversationDocument struct {
	ConversationID string
	Kind           string
	Revision       int64
	SchemaVersion  uint16
	CipherVersion  uint16
	Ciphertext     []byte
	DataNonce      []byte
	AADDigest      []byte
}

type encryptedConversationDocumentWithKey struct {
	document encryptedConversationDocument
	key      encryptedConversationKey
}

func (store *VaultStore) PutConversationDocument(
	ctx context.Context,
	document ConversationDocument,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validConversationDocument(document) {
		return ErrInvalidConversationDocument
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
		ctx, transaction, vmk, document.ConversationID,
	)
	if err != nil {
		return err
	}
	defer clearBytes(dek)

	existing, found, err := readEncryptedConversationDocumentTx(
		ctx, transaction, document.ConversationID, document.Kind,
	)
	if err != nil {
		return err
	}
	if found {
		if document.Revision < existing.Revision {
			return ErrConversationDocumentConflict
		}
		if document.Revision == existing.Revision {
			plaintext, decryptErr := decryptConversationDocument(dek, existing)
			if decryptErr != nil {
				return decryptErr
			}
			same := bytes.Equal(plaintext, document.Payload)
			clearBytes(plaintext)
			if !same {
				return ErrConversationDocumentConflict
			}
			return nil
		}
	}

	record, err := store.encryptConversationDocument(document, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, transaction, record.ConversationID, record.DataNonce,
		"document", record.Kind+":"+decimalRevision(record.Revision),
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	result, err := transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_conversation_documents (
			conversation_id, document_kind, document_revision,
			schema_version, cipher_version, ciphertext, data_nonce,
			aad_digest, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(conversation_id, document_kind) DO UPDATE SET
			document_revision=excluded.document_revision,
			schema_version=excluded.schema_version,
			cipher_version=excluded.cipher_version,
			ciphertext=excluded.ciphertext,
			data_nonce=excluded.data_nonce,
			aad_digest=excluded.aad_digest,
			updated_at=excluded.updated_at
		WHERE encrypted_conversation_documents.document_revision <
		      excluded.document_revision`,
		record.ConversationID, record.Kind, record.Revision,
		record.SchemaVersion, record.CipherVersion, record.Ciphertext,
		record.DataNonce, record.AADDigest, now, now,
	)
	if err != nil || !exactlyOneRow(result) || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ConversationDocuments(
	ctx context.Context,
	kind string,
) ([]ConversationDocument, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validConversationDocumentKind(kind) {
		return nil, ErrInvalidConversationDocument
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return nil, ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	records, err := readAllEncryptedConversationDocuments(
		ctx, store.database, kind,
	)
	if err != nil {
		return nil, err
	}
	documents := make([]ConversationDocument, 0, len(records))
	for _, record := range records {
		if record.key.KeyVersion != store.keyMaterial.KeyVersion() {
			clearConversationDocumentPayloads(documents)
			return nil, ErrVaultUnavailable
		}
		dek, unwrapErr := unwrapConversationDEK(vmk, record.key)
		if unwrapErr != nil {
			clearConversationDocumentPayloads(documents)
			return nil, unwrapErr
		}
		plaintext, decryptErr := decryptConversationDocument(
			dek, record.document,
		)
		clearBytes(dek)
		if decryptErr != nil {
			clearConversationDocumentPayloads(documents)
			return nil, decryptErr
		}
		documents = append(documents, ConversationDocument{
			ConversationID: record.document.ConversationID,
			Kind:           record.document.Kind,
			Revision:       record.document.Revision,
			Payload:        plaintext,
		})
	}
	return documents, nil
}

func (store *VaultStore) encryptConversationDocument(
	document ConversationDocument,
	dek []byte,
) (encryptedConversationDocument, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes || !validConversationDocument(document) {
		return encryptedConversationDocument{}, ErrInvalidConversationDocument
	}
	record := encryptedConversationDocument{
		ConversationID: document.ConversationID,
		Kind:           document.Kind,
		Revision:       document.Revision,
		SchemaVersion:  conversationDocumentSchemaVersion,
		CipherVersion:  conversationDocumentCipherVersion,
		DataNonce:      make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, record.DataNonce); err != nil {
		return encryptedConversationDocument{}, ErrVaultUnavailable
	}
	aad, err := canonicalConversationDocumentAAD(record)
	if err != nil {
		return encryptedConversationDocument{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedConversationDocument{}, ErrVaultUnavailable
	}
	record.Ciphertext = aead.Seal(nil, record.DataNonce, document.Payload, aad)
	return record, nil
}

func decryptConversationDocument(
	dek []byte,
	record encryptedConversationDocument,
) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedConversationDocument(record) {
		return nil, ErrConversationDocumentAuthentication
	}
	aad, err := canonicalConversationDocumentAAD(record)
	if err != nil {
		return nil, ErrConversationDocumentAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrConversationDocumentAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 ||
		len(plaintext) > maxConversationDocumentPayload {
		clearBytes(plaintext)
		return nil, ErrConversationDocumentAuthentication
	}
	return plaintext, nil
}

func readEncryptedConversationDocumentTx(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	conversationID string,
	kind string,
) (encryptedConversationDocument, bool, error) {
	record := encryptedConversationDocument{
		ConversationID: conversationID,
		Kind:           kind,
	}
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(
		ctx,
		`SELECT document_revision, schema_version, cipher_version,
		        ciphertext, data_nonce, aad_digest
		   FROM encrypted_conversation_documents
		  WHERE conversation_id = ? AND document_kind = ?`,
		conversationID, kind,
	).Scan(
		&record.Revision, &schemaVersion, &cipherVersion,
		&record.Ciphertext, &record.DataNonce, &record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedConversationDocument{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedConversationDocument{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	if !validEncryptedConversationDocument(record) {
		return encryptedConversationDocument{}, false,
			ErrConversationDocumentAuthentication
	}
	return record, true, nil
}

func readAllEncryptedConversationDocuments(
	ctx context.Context,
	database *sql.DB,
	kind string,
) ([]encryptedConversationDocumentWithKey, error) {
	rows, err := database.QueryContext(
		ctx,
		`SELECT d.conversation_id, d.document_revision,
		        d.schema_version, d.cipher_version, d.ciphertext,
		        d.data_nonce, d.aad_digest,
		        k.schema_version, k.cipher_version, k.key_version,
		        k.wrapped_dek, k.wrap_nonce, k.aad_digest
		   FROM encrypted_conversation_documents AS d
		   JOIN encrypted_conversation_keys AS k
		     ON k.conversation_id = d.conversation_id
		  WHERE d.document_kind = ?
		  ORDER BY d.conversation_id`,
		kind,
	)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer rows.Close()
	records := make([]encryptedConversationDocumentWithKey, 0)
	for rows.Next() {
		var record encryptedConversationDocumentWithKey
		var documentSchema, documentCipher int64
		var keySchema, keyCipher, keyVersion int64
		record.document.Kind = kind
		if err := rows.Scan(
			&record.document.ConversationID, &record.document.Revision,
			&documentSchema, &documentCipher, &record.document.Ciphertext,
			&record.document.DataNonce, &record.document.AADDigest,
			&keySchema, &keyCipher, &keyVersion, &record.key.WrappedDEK,
			&record.key.WrapNonce, &record.key.AADDigest,
		); err != nil || documentSchema < 0 || documentSchema > 1<<16-1 ||
			documentCipher < 0 || documentCipher > 1<<16-1 ||
			keySchema < 0 || keySchema > 1<<16-1 ||
			keyCipher < 0 || keyCipher > 1<<16-1 ||
			keyVersion < 0 || keyVersion > 1<<32-1 {
			return nil, ErrVaultUnavailable
		}
		record.document.SchemaVersion = uint16(documentSchema)
		record.document.CipherVersion = uint16(documentCipher)
		record.key = encryptedConversationKey{
			ConversationID: record.document.ConversationID,
			SchemaVersion:  uint16(keySchema),
			CipherVersion:  uint16(keyCipher),
			KeyVersion:     uint32(keyVersion),
			WrappedDEK:     record.key.WrappedDEK,
			WrapNonce:      record.key.WrapNonce,
			AADDigest:      record.key.AADDigest,
		}
		if !validEncryptedConversationDocument(record.document) ||
			!validEncryptedConversationKey(record.key) {
			return nil, ErrConversationDocumentAuthentication
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, ErrVaultUnavailable
	}
	return records, nil
}

func reserveConversationDataNonceTx(
	ctx context.Context,
	transaction *sql.Tx,
	conversationID string,
	nonce []byte,
	purpose string,
	recordID string,
) error {
	if transaction == nil || !validContextCapsuleIdentifier(conversationID) ||
		len(nonce) != vaultNonceBytes ||
		!validConversationDocumentKind(purpose) ||
		!validContextCapsuleIdentifier(recordID) {
		return ErrInvalidConversationDocument
	}
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_conversation_data_nonces (
			conversation_id, data_nonce, purpose, record_id
		) VALUES (?, ?, ?, ?)`,
		conversationID, nonce, purpose, recordID,
	)
	if err != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func canonicalConversationDocumentAAD(
	record encryptedConversationDocument,
) ([]byte, error) {
	if record.SchemaVersion != conversationDocumentSchemaVersion ||
		record.CipherVersion != conversationDocumentCipherVersion ||
		!validContextCapsuleIdentifier(record.ConversationID) ||
		!validConversationDocumentKind(record.Kind) || record.Revision <= 0 {
		return nil, ErrInvalidConversationDocument
	}
	var body bytes.Buffer
	body.WriteString("LOOM-CONVERSATION-DOCUMENT-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.ConversationID)
	writeCanonicalString(&body, record.Kind)
	_ = binary.Write(&body, binary.BigEndian, record.Revision)
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func validConversationDocumentIdentity(conversationID string, kind string) bool {
	return validContextCapsuleIdentifier(conversationID) &&
		validConversationDocumentKind(kind)
}

func validConversationDocument(document ConversationDocument) bool {
	return validContextCapsuleIdentifier(document.ConversationID) &&
		validConversationDocumentKind(document.Kind) && document.Revision > 0 &&
		len(document.Payload) > 0 && len(document.Payload) <= maxConversationDocumentPayload
}

func validEncryptedConversationDocument(record encryptedConversationDocument) bool {
	return record.SchemaVersion == conversationDocumentSchemaVersion &&
		record.CipherVersion == conversationDocumentCipherVersion &&
		validContextCapsuleIdentifier(record.ConversationID) &&
		validConversationDocumentKind(record.Kind) && record.Revision > 0 &&
		len(record.Ciphertext) > 16 &&
		len(record.Ciphertext) <= maxConversationDocumentPayload+16 &&
		len(record.DataNonce) == vaultNonceBytes &&
		len(record.AADDigest) == vaultAADDigestBytes
}

func validConversationDocumentKind(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func decimalRevision(value int64) string {
	if value <= 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}

func clearConversationDocumentPayloads(documents []ConversationDocument) {
	for index := range documents {
		clearBytes(documents[index].Payload)
	}
}

// DeleteConversationDocument removes the encrypted conversation document and
// its associated data nonce for a conversation. It returns
// ErrConversationDocumentNotFound when no document exists. Deleting is
// idempotent-safe for the caller only when the document exists.
func (store *VaultStore) DeleteConversationDocument(
	ctx context.Context,
	conversationID string,
	kind string,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validConversationDocumentIdentity(conversationID, kind) {
		return ErrInvalidConversationDocument
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
	existing, found, err := readEncryptedConversationDocumentTx(
		ctx, transaction, conversationID, kind,
	)
	if err != nil {
		return err
	}
	if !found {
		return ErrConversationDocumentNotFound
	}
	if _, err := transaction.ExecContext(
		ctx,
		`DELETE FROM encrypted_conversation_documents
		  WHERE conversation_id = ? AND document_kind = ?`,
		conversationID, kind,
	); err != nil {
		return ErrVaultUnavailable
	}
	if _, err := transaction.ExecContext(
		ctx,
		`DELETE FROM encrypted_conversation_data_nonces
		  WHERE conversation_id = ?`,
		conversationID,
	); err != nil {
		return ErrVaultUnavailable
	}
	_ = existing
	if transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}
