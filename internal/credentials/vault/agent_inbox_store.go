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

	"loom-pi-rebuild/internal/agentinbox"
)

const (
	agentInputSchemaVersion = uint16(1)
	agentInputCipherVersion = uint16(1)
	maxAgentInputBytes      = 64 << 10
)

var (
	ErrInvalidAgentInput        = errors.New("invalid encrypted Agent input")
	ErrAgentInputNotFound       = errors.New("encrypted Agent input not found")
	ErrAgentInputBinding        = errors.New("encrypted Agent input binding mismatch")
	ErrAgentInputConflict       = errors.New("encrypted Agent input conflict")
	ErrAgentInputAuthentication = errors.New("encrypted Agent input authentication failed")
)

type encryptedAgentInput struct {
	agentinbox.Payload
	SchemaVersion uint16
	CipherVersion uint16
	Ciphertext    []byte
	DataNonce     []byte
	AADDigest     []byte
}

var _ agentinbox.Store = (*VaultStore)(nil)

func (store *VaultStore) PutAgentInput(ctx context.Context, payload agentinbox.Payload) error {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		payload.Status != agentinbox.StatusPending || !validAgentInputPayload(payload) {
		return ErrInvalidAgentInput
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
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer tx.Rollback()
	_, dek, err := store.readOrCreateConversationDEKTx(ctx, tx, vmk, payload.Binding.ConversationID)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	existing, found, err := readEncryptedAgentInput(ctx, tx, payload.Binding.PayloadID)
	if err != nil {
		return err
	}
	if found {
		plaintext, decryptErr := decryptAgentInput(dek, existing)
		if decryptErr != nil {
			return decryptErr
		}
		defer clearBytes(plaintext)
		if existing.Binding != payload.Binding {
			return ErrAgentInputBinding
		}
		if !bytes.Equal(plaintext, payload.Content) {
			return ErrAgentInputConflict
		}
		return nil
	}
	if reused, reuseErr := agentInputOrderExists(ctx, tx, payload.Binding); reuseErr != nil {
		return reuseErr
	} else if reused {
		return ErrAgentInputConflict
	}
	record, err := store.encryptAgentInput(payload, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, tx, record.Binding.ConversationID, record.DataNonce,
		"agent-input", record.Binding.PayloadID+":"+string(record.Status),
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO encrypted_agent_inputs (
		payload_id, input_id, mode, context_scope, scope_target_id,
		conversation_id, segment_id,
		agent_instance_id, work_item_id, run_id, claim_generation,
		runtime_instance_id, execution_binding_digest, capsule_digest, order_key,
		target_turn_id, target_turn_sequence, target_step_id, target_step_sequence,
		content_type, content_digest, status, schema_version, cipher_version,
		ciphertext, data_nonce, aad_digest, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Binding.PayloadID, record.Binding.InputID, record.Binding.Mode,
		record.Binding.ContextScope, record.Binding.ScopeTargetID,
		record.Binding.ConversationID,
		record.Binding.SegmentID, record.Binding.AgentInstanceID,
		record.Binding.WorkItemID, record.Binding.RunID, record.Binding.ClaimGeneration,
		record.Binding.RuntimeInstanceID, record.Binding.ExecutionBindingDigest,
		record.Binding.CapsuleDigest, record.Binding.OrderKey,
		record.Binding.TargetTurnID, record.Binding.TargetTurnSequence,
		record.Binding.TargetStepID, record.Binding.TargetStepSequence,
		record.Binding.ContentType, record.Binding.ContentDigest, record.Status,
		record.SchemaVersion, record.CipherVersion, record.Ciphertext,
		record.DataNonce, record.AADDigest, now, now,
	)
	if err != nil || tx.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ReadAgentInput(
	ctx context.Context,
	binding agentinbox.Binding,
) (agentinbox.Payload, error) {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentInputBinding(binding) {
		return agentinbox.Payload{}, ErrInvalidAgentInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return agentinbox.Payload{}, ErrVaultUnavailable
	}
	record, found, err := readEncryptedAgentInput(ctx, store.database, binding.PayloadID)
	if err != nil {
		return agentinbox.Payload{}, err
	}
	if !found {
		return agentinbox.Payload{}, ErrAgentInputNotFound
	}
	payload, err := store.decryptAgentInputRecord(ctx, record)
	if err != nil {
		return agentinbox.Payload{}, err
	}
	if record.Binding != binding {
		payload.Close()
		return agentinbox.Payload{}, ErrAgentInputBinding
	}
	return payload, nil
}

func (store *VaultStore) ListPendingAgentInputs(
	ctx context.Context,
	conversationID, runID, agentInstanceID string,
	claimGeneration int64,
) ([]agentinbox.Payload, error) {
	if store == nil || ctx == nil || ctx.Err() != nil ||
		!validContextCapsuleIdentifier(conversationID) ||
		!validContextCapsuleIdentifier(runID) ||
		!validContextCapsuleIdentifier(agentInstanceID) || claimGeneration <= 0 {
		return nil, ErrInvalidAgentInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return nil, ErrVaultUnavailable
	}
	rows, err := store.database.QueryContext(ctx, `SELECT payload_id
		FROM encrypted_agent_inputs
		WHERE conversation_id = ? AND run_id = ? AND agent_instance_id = ?
		  AND claim_generation = ? AND status = 'pending'
		ORDER BY order_key, payload_id`,
		conversationID, runID, agentInstanceID, claimGeneration,
	)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if rows.Scan(&id) != nil || !validContextCapsuleIdentifier(id) {
			return nil, ErrAgentInputAuthentication
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, ErrVaultUnavailable
	}
	if err := rows.Close(); err != nil {
		return nil, ErrVaultUnavailable
	}
	results := make([]agentinbox.Payload, 0, len(ids))
	for _, id := range ids {
		record, found, readErr := readEncryptedAgentInput(ctx, store.database, id)
		if readErr != nil || !found {
			clearAgentInputPayloads(results)
			return nil, errors.Join(ErrAgentInputAuthentication, readErr)
		}
		payload, decryptErr := store.decryptAgentInputRecord(ctx, record)
		if decryptErr != nil || payload.Status != agentinbox.StatusPending {
			payload.Close()
			clearAgentInputPayloads(results)
			return nil, errors.Join(ErrAgentInputAuthentication, decryptErr)
		}
		results = append(results, payload)
	}
	return results, nil
}

func (store *VaultStore) MarkAgentInputConsumed(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentInputBinding(binding) {
		return ErrInvalidAgentInput
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
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer tx.Rollback()
	record, found, err := readEncryptedAgentInput(ctx, tx, binding.PayloadID)
	if err != nil {
		return err
	}
	if !found {
		return ErrAgentInputNotFound
	}
	conversationKey, err := readEncryptedConversationKey(ctx, tx, record.Binding.ConversationID)
	if err != nil || conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return ErrVaultUnavailable
	}
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	plaintext, err := decryptAgentInput(dek, record)
	if err != nil {
		return err
	}
	defer clearBytes(plaintext)
	if record.Binding != binding {
		return ErrAgentInputBinding
	}
	if record.Status == agentinbox.StatusConsumed {
		return nil
	}
	updated, err := store.encryptAgentInput(agentinbox.Payload{
		Binding: record.Binding, Status: agentinbox.StatusConsumed, Content: plaintext,
	}, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, tx, binding.ConversationID, updated.DataNonce,
		"agent-input", binding.PayloadID+":"+string(updated.Status),
	); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE encrypted_agent_inputs SET
		status = ?, schema_version = ?, cipher_version = ?, ciphertext = ?,
		data_nonce = ?, aad_digest = ?, updated_at = ?
		WHERE payload_id = ? AND status = 'pending' AND data_nonce = ?`,
		updated.Status, updated.SchemaVersion, updated.CipherVersion,
		updated.Ciphertext, updated.DataNonce, updated.AADDigest,
		store.now().UTC().Format(time.RFC3339Nano), binding.PayloadID, record.DataNonce,
	)
	if err != nil || !exactlyOneRow(result) || tx.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) DeleteAgentInput(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentInputBinding(binding) {
		return ErrInvalidAgentInput
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return ErrVaultUnavailable
	}
	tx, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return ErrVaultUnavailable
	}
	defer tx.Rollback()
	record, found, err := readEncryptedAgentInput(ctx, tx, binding.PayloadID)
	if err != nil {
		return err
	}
	if !found {
		return ErrAgentInputNotFound
	}
	if record.Binding != binding {
		return ErrAgentInputBinding
	}
	conversationKey, err := readEncryptedConversationKey(ctx, tx, record.Binding.ConversationID)
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
	plaintext, err := decryptAgentInput(dek, record)
	clearBytes(plaintext)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM encrypted_agent_inputs WHERE payload_id = ?`, binding.PayloadID)
	if err != nil || !exactlyOneRow(result) || tx.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) decryptAgentInputRecord(
	ctx context.Context,
	record encryptedAgentInput,
) (agentinbox.Payload, error) {
	conversationKey, err := readEncryptedConversationKey(ctx, store.database, record.Binding.ConversationID)
	if err != nil || conversationKey.KeyVersion != store.keyMaterial.KeyVersion() {
		return agentinbox.Payload{}, ErrVaultUnavailable
	}
	vmk, err := store.keyMaterial.keyCopy()
	if err != nil {
		return agentinbox.Payload{}, ErrVaultUnavailable
	}
	defer clearBytes(vmk)
	dek, err := unwrapConversationDEK(vmk, conversationKey)
	if err != nil {
		return agentinbox.Payload{}, err
	}
	defer clearBytes(dek)
	plaintext, err := decryptAgentInput(dek, record)
	if err != nil {
		return agentinbox.Payload{}, err
	}
	return agentinbox.Payload{Binding: record.Binding, Status: record.Status, Content: plaintext}, nil
}

func (store *VaultStore) encryptAgentInput(
	payload agentinbox.Payload,
	dek []byte,
) (encryptedAgentInput, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes || !validAgentInputPayload(payload) {
		return encryptedAgentInput{}, ErrInvalidAgentInput
	}
	record := encryptedAgentInput{
		Payload: payload, SchemaVersion: agentInputSchemaVersion,
		CipherVersion: agentInputCipherVersion, DataNonce: make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, record.DataNonce); err != nil {
		return encryptedAgentInput{}, ErrVaultUnavailable
	}
	aad, err := canonicalAgentInputAAD(record)
	if err != nil {
		return encryptedAgentInput{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedAgentInput{}, ErrVaultUnavailable
	}
	record.Ciphertext = aead.Seal(nil, record.DataNonce, payload.Content, aad)
	record.Content = nil
	return record, nil
}

func decryptAgentInput(dek []byte, record encryptedAgentInput) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedAgentInput(record) {
		return nil, ErrAgentInputAuthentication
	}
	aad, err := canonicalAgentInputAAD(record)
	if err != nil {
		return nil, ErrAgentInputAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrAgentInputAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxAgentInputBytes ||
		agentInputDigest(plaintext) != record.Binding.ContentDigest {
		clearBytes(plaintext)
		return nil, ErrAgentInputAuthentication
	}
	return plaintext, nil
}

func canonicalAgentInputAAD(record encryptedAgentInput) ([]byte, error) {
	if record.SchemaVersion != agentInputSchemaVersion ||
		record.CipherVersion != agentInputCipherVersion ||
		!validAgentInputBinding(record.Binding) || !validAgentInputStatus(record.Status) {
		return nil, ErrInvalidAgentInput
	}
	var body bytes.Buffer
	body.WriteString("LOOM-AGENT-INPUT-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	binding := record.Binding
	for _, value := range []string{
		binding.PayloadID, binding.InputID, string(binding.Mode), string(binding.ContextScope),
		binding.ScopeTargetID,
		binding.ConversationID, binding.SegmentID, binding.AgentInstanceID,
		binding.WorkItemID, binding.RunID,
	} {
		writeCanonicalString(&body, value)
	}
	_ = binary.Write(&body, binary.BigEndian, binding.ClaimGeneration)
	for _, value := range []string{
		binding.RuntimeInstanceID, binding.ExecutionBindingDigest, binding.CapsuleDigest,
	} {
		writeCanonicalString(&body, value)
	}
	_ = binary.Write(&body, binary.BigEndian, binding.OrderKey)
	writeCanonicalString(&body, binding.TargetTurnID)
	_ = binary.Write(&body, binary.BigEndian, int64(binding.TargetTurnSequence))
	writeCanonicalString(&body, binding.TargetStepID)
	_ = binary.Write(&body, binary.BigEndian, int64(binding.TargetStepSequence))
	writeCanonicalString(&body, binding.ContentType)
	writeCanonicalString(&body, binding.ContentDigest)
	writeCanonicalString(&body, string(record.Status))
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func readEncryptedAgentInput(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	payloadID string,
) (encryptedAgentInput, bool, error) {
	record := encryptedAgentInput{}
	record.Binding.PayloadID = payloadID
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(ctx, `SELECT
		input_id, mode, context_scope, scope_target_id, conversation_id, segment_id,
		agent_instance_id, work_item_id, run_id, claim_generation,
		runtime_instance_id, execution_binding_digest, capsule_digest, order_key,
		target_turn_id, target_turn_sequence, target_step_id, target_step_sequence,
		content_type, content_digest, status, schema_version, cipher_version,
		ciphertext, data_nonce, aad_digest
		FROM encrypted_agent_inputs WHERE payload_id = ?`, payloadID).Scan(
		&record.Binding.InputID, &record.Binding.Mode, &record.Binding.ContextScope,
		&record.Binding.ScopeTargetID,
		&record.Binding.ConversationID, &record.Binding.SegmentID,
		&record.Binding.AgentInstanceID, &record.Binding.WorkItemID,
		&record.Binding.RunID, &record.Binding.ClaimGeneration,
		&record.Binding.RuntimeInstanceID, &record.Binding.ExecutionBindingDigest,
		&record.Binding.CapsuleDigest, &record.Binding.OrderKey,
		&record.Binding.TargetTurnID, &record.Binding.TargetTurnSequence,
		&record.Binding.TargetStepID, &record.Binding.TargetStepSequence,
		&record.Binding.ContentType, &record.Binding.ContentDigest, &record.Status,
		&schemaVersion, &cipherVersion, &record.Ciphertext, &record.DataNonce,
		&record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedAgentInput{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedAgentInput{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion, record.CipherVersion = uint16(schemaVersion), uint16(cipherVersion)
	if !validEncryptedAgentInput(record) {
		return encryptedAgentInput{}, false, ErrAgentInputAuthentication
	}
	return record, true, nil
}

func agentInputOrderExists(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	binding agentinbox.Binding,
) (bool, error) {
	var payloadID string
	err := querier.QueryRowContext(ctx, `SELECT payload_id FROM encrypted_agent_inputs
		WHERE run_id = ? AND claim_generation = ? AND order_key = ?`,
		binding.RunID, binding.ClaimGeneration, binding.OrderKey,
	).Scan(&payloadID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil || !validContextCapsuleIdentifier(payloadID) {
		return false, ErrVaultUnavailable
	}
	return true, nil
}

func validAgentInputPayload(payload agentinbox.Payload) bool {
	return validAgentInputBinding(payload.Binding) && validAgentInputStatus(payload.Status) &&
		len(payload.Content) > 0 && len(payload.Content) <= maxAgentInputBytes &&
		utf8.Valid(payload.Content) && agentInputDigest(payload.Content) == payload.Binding.ContentDigest
}

func validEncryptedAgentInput(record encryptedAgentInput) bool {
	return record.SchemaVersion == agentInputSchemaVersion &&
		record.CipherVersion == agentInputCipherVersion &&
		validAgentInputBinding(record.Binding) && validAgentInputStatus(record.Status) &&
		len(record.Ciphertext) > 16 && len(record.Ciphertext) <= maxAgentInputBytes+16 &&
		len(record.DataNonce) == vaultNonceBytes && len(record.AADDigest) == vaultAADDigestBytes
}

func validAgentInputBinding(binding agentinbox.Binding) bool {
	if !validContextCapsuleIdentifier(binding.PayloadID) ||
		!validContextCapsuleIdentifier(binding.InputID) ||
		!validContextCapsuleIdentifier(binding.ConversationID) ||
		!validContextCapsuleIdentifier(binding.SegmentID) ||
		!validContextCapsuleIdentifier(binding.AgentInstanceID) ||
		!validContextCapsuleIdentifier(binding.WorkItemID) ||
		!validContextCapsuleIdentifier(binding.RunID) || binding.ClaimGeneration <= 0 ||
		!validContextCapsuleIdentifier(binding.RuntimeInstanceID) ||
		!validContextCapsuleDigest(binding.ExecutionBindingDigest) ||
		!validContextCapsuleDigest(binding.CapsuleDigest) || binding.OrderKey <= 0 ||
		binding.ContentType != "text/plain" ||
		!validContextCapsuleDigest(binding.ContentDigest) || !validAgentInputMode(binding.Mode) ||
		!validAgentInputScope(binding.ContextScope, binding.ScopeTargetID) {
		return false
	}
	if binding.Mode == agentinbox.ModeQueue {
		return validContextCapsuleIdentifier(binding.TargetTurnID) &&
			binding.TargetTurnSequence > 0 && binding.TargetStepID == "" &&
			binding.TargetStepSequence == 0
	}
	return binding.TargetTurnID == "" && binding.TargetTurnSequence == 0 &&
		validContextCapsuleIdentifier(binding.TargetStepID) && binding.TargetStepSequence > 0
}

func validAgentInputMode(mode agentinbox.Mode) bool {
	return mode == agentinbox.ModeQueue || mode == agentinbox.ModeSteer || mode == agentinbox.ModeInject
}

func validAgentInputScope(scope agentinbox.ContextScope, targetID string) bool {
	switch scope {
	case agentinbox.ScopeConversationShared, agentinbox.ScopeTeamShared,
		agentinbox.ScopeAgentPrivate:
		return targetID == ""
	case agentinbox.ScopeRoleRestricted, agentinbox.ScopeArtifact,
		agentinbox.ScopeSecretReference:
		return validContextCapsuleIdentifier(targetID)
	default:
		return false
	}
}

func validAgentInputStatus(status agentinbox.Status) bool {
	return status == agentinbox.StatusPending || status == agentinbox.StatusConsumed
}

func agentInputDigest(content []byte) string {
	digest := sha256.Sum256(content)
	const alphabet = "0123456789abcdef"
	result := make([]byte, sha256.Size*2)
	for index, value := range digest {
		result[index*2], result[index*2+1] = alphabet[value>>4], alphabet[value&0x0f]
	}
	return string(result)
}

func clearAgentInputPayloads(payloads []agentinbox.Payload) {
	for index := range payloads {
		payloads[index].Close()
	}
}
