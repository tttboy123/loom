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
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/toolproposal"
)

const (
	toolProposalSchemaVersion = uint16(1)
	toolProposalCipherVersion = uint16(1)
	maxToolProposalBytes      = 32 << 10
)

var (
	ErrInvalidToolProposal        = errors.New("invalid encrypted Tool Proposal")
	ErrToolProposalNotFound       = errors.New("encrypted Tool Proposal not found")
	ErrToolProposalBinding        = errors.New("encrypted Tool Proposal binding mismatch")
	ErrToolProposalConflict       = errors.New("encrypted Tool Proposal conflict")
	ErrToolProposalAuthentication = errors.New("encrypted Tool Proposal authentication failed")
)

type encryptedToolProposal struct {
	Binding       toolproposal.Binding
	SchemaVersion uint16
	CipherVersion uint16
	Ciphertext    []byte
	DataNonce     []byte
	AADDigest     []byte
}

func (store *VaultStore) PutToolProposal(
	ctx context.Context,
	record toolproposal.Record,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || !validToolProposal(record) {
		return ErrInvalidToolProposal
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
		ctx, transaction, vmk, record.Binding.ConversationID,
	)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	existing, found, err := readEncryptedToolProposal(ctx, transaction, record.Binding.ProposalID)
	if err != nil {
		return err
	}
	if found {
		plaintext, decryptErr := decryptToolProposal(dek, existing)
		if decryptErr != nil {
			return decryptErr
		}
		defer clearBytes(plaintext)
		if existing.Binding != record.Binding {
			if existing.Binding.ProposalID == record.Binding.ProposalID {
				return ErrToolProposalConflict
			}
			return ErrToolProposalBinding
		}
		if !bytes.Equal(plaintext, record.Content) {
			return ErrToolProposalConflict
		}
		return nil
	}
	encrypted, err := store.encryptToolProposal(record, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, transaction, encrypted.Binding.ConversationID, encrypted.DataNonce,
		"tool-proposal", encrypted.Binding.ProposalID,
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	_, err = transaction.ExecContext(
		ctx,
		`INSERT INTO encrypted_tool_proposals (
			proposal_id, conversation_id, work_item_id, run_id, claim_generation,
			runtime_instance_id, agent_instance_id, execution_binding_digest,
			capsule_digest, call_id, call_digest, approval_id, approval_digest,
			tool, operation_id, incident_id, content_digest, schema_version,
			cipher_version, ciphertext, data_nonce, aad_digest, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		encrypted.Binding.ProposalID, encrypted.Binding.ConversationID,
		encrypted.Binding.WorkItemID, encrypted.Binding.RunID,
		encrypted.Binding.ClaimGeneration, encrypted.Binding.RuntimeInstanceID,
		encrypted.Binding.AgentInstanceID, encrypted.Binding.ExecutionBindingDigest,
		encrypted.Binding.CapsuleDigest, encrypted.Binding.CallID,
		encrypted.Binding.CallDigest, encrypted.Binding.ApprovalID,
		encrypted.Binding.ApprovalDigest, encrypted.Binding.Tool,
		encrypted.Binding.OperationID, encrypted.Binding.IncidentID,
		encrypted.Binding.ContentDigest, encrypted.SchemaVersion,
		encrypted.CipherVersion, encrypted.Ciphertext, encrypted.DataNonce,
		encrypted.AADDigest, now, now,
	)
	if err != nil || transaction.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ReadToolProposal(
	ctx context.Context,
	binding toolproposal.Binding,
) (toolproposal.Record, error) {
	if store == nil || ctx == nil || ctx.Err() != nil || !validToolProposalBinding(binding) {
		return toolproposal.Record{}, ErrInvalidToolProposal
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return toolproposal.Record{}, ErrVaultUnavailable
	}
	record, found, err := readEncryptedToolProposal(ctx, store.database, binding.ProposalID)
	if err != nil {
		return toolproposal.Record{}, err
	}
	if !found {
		return toolproposal.Record{}, ErrToolProposalNotFound
	}
	content, err := store.decryptToolProposalRecord(ctx, record)
	if err != nil {
		return toolproposal.Record{}, err
	}
	if record.Binding != binding {
		clearBytes(content)
		return toolproposal.Record{}, ErrToolProposalBinding
	}
	return toolproposal.Record{Binding: binding, Content: content}, nil
}

func (store *VaultStore) LookupToolProposal(
	ctx context.Context,
	lookup toolproposal.ApprovalLookup,
) (toolproposal.Record, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validContextCapsuleIdentifier(lookup.ApprovalID) ||
		!validContextCapsuleDigest(lookup.ApprovalDigest) ||
		!validContextCapsuleIdentifier(lookup.WorkItemID) ||
		!validContextCapsuleDigest(lookup.CallDigest) {
		return toolproposal.Record{}, ErrInvalidToolProposal
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return toolproposal.Record{}, ErrVaultUnavailable
	}
	record, found, err := readEncryptedToolProposal(ctx, store.database, lookup.ApprovalID)
	if err != nil {
		return toolproposal.Record{}, err
	}
	if !found {
		return toolproposal.Record{}, ErrToolProposalNotFound
	}
	content, err := store.decryptToolProposalRecord(ctx, record)
	if err != nil {
		return toolproposal.Record{}, err
	}
	if record.Binding.ApprovalID != lookup.ApprovalID ||
		record.Binding.ApprovalDigest != lookup.ApprovalDigest ||
		record.Binding.WorkItemID != lookup.WorkItemID ||
		record.Binding.CallDigest != lookup.CallDigest {
		clearBytes(content)
		return toolproposal.Record{}, ErrToolProposalBinding
	}
	return toolproposal.Record{Binding: record.Binding, Content: content}, nil
}

func (store *VaultStore) DeleteToolProposal(
	ctx context.Context,
	binding toolproposal.Binding,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || !validToolProposalBinding(binding) {
		return ErrInvalidToolProposal
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return ErrVaultUnavailable
	}
	record, found, err := readEncryptedToolProposal(ctx, store.database, binding.ProposalID)
	if err != nil {
		return err
	}
	if !found {
		return ErrToolProposalNotFound
	}
	plaintext, err := store.decryptToolProposalRecord(ctx, record)
	if err != nil {
		return err
	}
	defer clearBytes(plaintext)
	if record.Binding != binding {
		return ErrToolProposalBinding
	}
	result, err := store.database.ExecContext(
		ctx, `DELETE FROM encrypted_tool_proposals WHERE proposal_id = ?`,
		binding.ProposalID,
	)
	if err != nil || !exactlyOneRow(result) {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) encryptToolProposal(
	record toolproposal.Record,
	dek []byte,
) (encryptedToolProposal, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes || !validToolProposal(record) {
		return encryptedToolProposal{}, ErrInvalidToolProposal
	}
	encrypted := encryptedToolProposal{
		Binding: record.Binding, SchemaVersion: toolProposalSchemaVersion,
		CipherVersion: toolProposalCipherVersion,
		DataNonce:     make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, encrypted.DataNonce); err != nil {
		return encryptedToolProposal{}, ErrVaultUnavailable
	}
	aad, err := canonicalToolProposalAAD(encrypted)
	if err != nil {
		return encryptedToolProposal{}, err
	}
	digest := sha256.Sum256(aad)
	encrypted.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedToolProposal{}, ErrVaultUnavailable
	}
	encrypted.Ciphertext = aead.Seal(nil, encrypted.DataNonce, record.Content, aad)
	return encrypted, nil
}

func (store *VaultStore) decryptToolProposalRecord(
	ctx context.Context,
	record encryptedToolProposal,
) ([]byte, error) {
	conversationKey, err := readEncryptedConversationKey(
		ctx, store.database, record.Binding.ConversationID,
	)
	if err != nil || conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return nil, ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return nil, err
	}
	defer clearBytes(dek)
	return decryptToolProposal(dek, record)
}

func decryptToolProposal(dek []byte, record encryptedToolProposal) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedToolProposal(record) {
		return nil, ErrToolProposalAuthentication
	}
	aad, err := canonicalToolProposalAAD(record)
	if err != nil {
		return nil, ErrToolProposalAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrToolProposalAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxToolProposalBytes ||
		toolproposal.ContentDigest(plaintext) != record.Binding.ContentDigest {
		clearBytes(plaintext)
		return nil, ErrToolProposalAuthentication
	}
	return plaintext, nil
}

func readEncryptedToolProposal(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	proposalID string,
) (encryptedToolProposal, bool, error) {
	record := encryptedToolProposal{}
	record.Binding.ProposalID = proposalID
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(
		ctx,
		`SELECT conversation_id, work_item_id, run_id, claim_generation,
		        runtime_instance_id, agent_instance_id, execution_binding_digest,
		        capsule_digest, call_id, call_digest, approval_id, approval_digest,
		        tool, operation_id, incident_id, content_digest, schema_version,
		        cipher_version, ciphertext, data_nonce, aad_digest
		   FROM encrypted_tool_proposals WHERE proposal_id = ?`,
		proposalID,
	).Scan(
		&record.Binding.ConversationID, &record.Binding.WorkItemID,
		&record.Binding.RunID, &record.Binding.ClaimGeneration,
		&record.Binding.RuntimeInstanceID, &record.Binding.AgentInstanceID,
		&record.Binding.ExecutionBindingDigest, &record.Binding.CapsuleDigest,
		&record.Binding.CallID, &record.Binding.CallDigest,
		&record.Binding.ApprovalID, &record.Binding.ApprovalDigest,
		&record.Binding.Tool, &record.Binding.OperationID,
		&record.Binding.IncidentID, &record.Binding.ContentDigest,
		&schemaVersion, &cipherVersion, &record.Ciphertext,
		&record.DataNonce, &record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedToolProposal{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedToolProposal{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion = uint16(schemaVersion)
	record.CipherVersion = uint16(cipherVersion)
	if !validEncryptedToolProposal(record) {
		return encryptedToolProposal{}, false, ErrToolProposalAuthentication
	}
	return record, true, nil
}

func canonicalToolProposalAAD(record encryptedToolProposal) ([]byte, error) {
	if record.SchemaVersion != toolProposalSchemaVersion ||
		record.CipherVersion != toolProposalCipherVersion ||
		!validToolProposalBinding(record.Binding) {
		return nil, ErrInvalidToolProposal
	}
	var body bytes.Buffer
	body.WriteString("LOOM-TOOL-PROPOSAL-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	writeCanonicalString(&body, record.Binding.ProposalID)
	writeCanonicalString(&body, record.Binding.ConversationID)
	writeCanonicalString(&body, record.Binding.WorkItemID)
	writeCanonicalString(&body, record.Binding.RunID)
	_ = binary.Write(&body, binary.BigEndian, record.Binding.ClaimGeneration)
	writeCanonicalString(&body, record.Binding.RuntimeInstanceID)
	writeCanonicalString(&body, record.Binding.AgentInstanceID)
	writeCanonicalString(&body, record.Binding.ExecutionBindingDigest)
	writeCanonicalString(&body, record.Binding.CapsuleDigest)
	writeCanonicalString(&body, record.Binding.CallID)
	writeCanonicalString(&body, record.Binding.CallDigest)
	writeCanonicalString(&body, record.Binding.ApprovalID)
	writeCanonicalString(&body, record.Binding.ApprovalDigest)
	writeCanonicalString(&body, record.Binding.Tool)
	writeCanonicalString(&body, record.Binding.OperationID)
	writeCanonicalString(&body, record.Binding.IncidentID)
	writeCanonicalString(&body, record.Binding.ContentDigest)
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func validToolProposal(record toolproposal.Record) bool {
	return validToolProposalBinding(record.Binding) && len(record.Content) > 0 &&
		len(record.Content) <= maxToolProposalBytes && utf8.Valid(record.Content) &&
		toolproposal.ContentDigest(record.Content) == record.Binding.ContentDigest
}

func validEncryptedToolProposal(record encryptedToolProposal) bool {
	return record.SchemaVersion == toolProposalSchemaVersion &&
		record.CipherVersion == toolProposalCipherVersion &&
		validToolProposalBinding(record.Binding) && len(record.Ciphertext) > 16 &&
		len(record.Ciphertext) <= maxToolProposalBytes+16 &&
		len(record.DataNonce) == vaultNonceBytes &&
		len(record.AADDigest) == vaultAADDigestBytes
}

func validToolProposalBinding(binding toolproposal.Binding) bool {
	return validContextCapsuleIdentifier(binding.ProposalID) &&
		validContextCapsuleIdentifier(binding.ConversationID) &&
		validContextCapsuleIdentifier(binding.WorkItemID) &&
		validContextCapsuleIdentifier(binding.RunID) && binding.ClaimGeneration > 0 &&
		validContextCapsuleIdentifier(binding.RuntimeInstanceID) &&
		validContextCapsuleIdentifier(binding.AgentInstanceID) &&
		validContextCapsuleDigest(binding.ExecutionBindingDigest) &&
		validContextCapsuleDigest(binding.CapsuleDigest) &&
		validContextCapsuleIdentifier(binding.CallID) &&
		validContextCapsuleDigest(binding.CallDigest) &&
		validContextCapsuleIdentifier(binding.ApprovalID) &&
		validContextCapsuleDigest(binding.ApprovalDigest) &&
		permissions.ValidToolKind(binding.Tool) &&
		validContextCapsuleIdentifier(binding.OperationID) &&
		validContextCapsuleIdentifier(binding.IncidentID) &&
		validContextCapsuleDigest(binding.ContentDigest)
}
