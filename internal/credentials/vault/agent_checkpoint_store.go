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

	"loom-pi-rebuild/internal/agentcheckpoint"
)

const (
	agentCheckpointSchemaVersion = uint16(1)
	agentCheckpointCipherVersion = uint16(1)
	maxAgentCheckpointBytes      = 64 << 10
)

var (
	ErrInvalidAgentCheckpoint        = errors.New("invalid encrypted Agent checkpoint")
	ErrAgentCheckpointNotFound       = errors.New("encrypted Agent checkpoint not found")
	ErrAgentCheckpointBinding        = errors.New("encrypted Agent checkpoint binding mismatch")
	ErrAgentCheckpointConflict       = errors.New("encrypted Agent checkpoint conflict")
	ErrAgentCheckpointAuthentication = errors.New("encrypted Agent checkpoint authentication failed")
)

type encryptedAgentCheckpoint struct {
	agentcheckpoint.Payload
	SchemaVersion uint16
	CipherVersion uint16
	Ciphertext    []byte
	DataNonce     []byte
	AADDigest     []byte
}

var _ agentcheckpoint.Store = (*VaultStore)(nil)

func (store *VaultStore) PutAgentCheckpoint(
	ctx context.Context,
	payload agentcheckpoint.Payload,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentCheckpointPayload(payload) {
		return ErrInvalidAgentCheckpoint
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
	_, dek, err := store.readOrCreateConversationDEKTx(
		ctx, tx, vmk, payload.Binding.ConversationID,
	)
	if err != nil {
		return err
	}
	defer clearBytes(dek)
	existing, found, err := readEncryptedAgentCheckpoint(ctx, tx, payload.Binding.CheckpointID)
	if err != nil {
		return err
	}
	if found {
		plaintext, decryptErr := decryptAgentCheckpoint(dek, existing)
		if decryptErr != nil {
			return decryptErr
		}
		defer clearBytes(plaintext)
		if existing.Binding != payload.Binding {
			return ErrAgentCheckpointBinding
		}
		if !bytes.Equal(plaintext, payload.Content) {
			return ErrAgentCheckpointConflict
		}
		return nil
	}
	if reused, reuseErr := agentCheckpointPositionExists(ctx, tx, payload.Binding); reuseErr != nil {
		return reuseErr
	} else if reused {
		return ErrAgentCheckpointConflict
	}
	record, err := store.encryptAgentCheckpoint(payload, dek)
	if err != nil {
		return err
	}
	if err := reserveConversationDataNonceTx(
		ctx, tx, record.Binding.ConversationID, record.DataNonce,
		"agent-checkpoint", record.Binding.CheckpointID,
	); err != nil {
		return err
	}
	now := store.now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO encrypted_agent_checkpoints (
		checkpoint_id, conversation_id, segment_id, attempt_id, agent_instance_id,
		work_item_id, run_id, claim_generation, runtime_instance_id,
		execution_binding_digest, capsule_digest, turn_id, turn_sequence,
		step_id, step_sequence, content_type, content_digest, schema_version,
		cipher_version, ciphertext, data_nonce, aad_digest, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Binding.CheckpointID, record.Binding.ConversationID,
		record.Binding.SegmentID, record.Binding.AttemptID,
		record.Binding.AgentInstanceID, record.Binding.WorkItemID,
		record.Binding.RunID, record.Binding.ClaimGeneration,
		record.Binding.RuntimeInstanceID, record.Binding.ExecutionBindingDigest,
		record.Binding.CapsuleDigest, record.Binding.TurnID,
		record.Binding.TurnSequence, record.Binding.StepID,
		record.Binding.StepSequence, record.Binding.ContentType,
		record.Binding.ContentDigest, record.SchemaVersion, record.CipherVersion,
		record.Ciphertext, record.DataNonce, record.AADDigest, now, now,
	)
	if err != nil || tx.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) ReadAgentCheckpoint(
	ctx context.Context,
	binding agentcheckpoint.Binding,
) (agentcheckpoint.Payload, error) {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentCheckpointBinding(binding) {
		return agentcheckpoint.Payload{}, ErrInvalidAgentCheckpoint
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return agentcheckpoint.Payload{}, ErrVaultUnavailable
	}
	record, found, err := readEncryptedAgentCheckpoint(ctx, store.database, binding.CheckpointID)
	if err != nil {
		return agentcheckpoint.Payload{}, err
	}
	if !found {
		return agentcheckpoint.Payload{}, ErrAgentCheckpointNotFound
	}
	plaintext, err := store.decryptAgentCheckpointRecord(ctx, record)
	if err != nil {
		return agentcheckpoint.Payload{}, err
	}
	if record.Binding != binding {
		clearBytes(plaintext)
		return agentcheckpoint.Payload{}, ErrAgentCheckpointBinding
	}
	return agentcheckpoint.Payload{Binding: record.Binding, Content: plaintext}, nil
}

func (store *VaultStore) ResolveAgentCheckpoint(
	ctx context.Context,
	query agentcheckpoint.Query,
) (agentcheckpoint.Payload, error) {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentCheckpointQuery(query) {
		return agentcheckpoint.Payload{}, ErrInvalidAgentCheckpoint
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.validateRuntime() != nil {
		return agentcheckpoint.Payload{}, ErrVaultUnavailable
	}
	rows, err := store.database.QueryContext(ctx, `SELECT checkpoint_id
		FROM encrypted_agent_checkpoints
		WHERE conversation_id = ? AND segment_id = ? AND attempt_id = ?
		  AND agent_instance_id = ? AND work_item_id = ? AND run_id = ?
		  AND claim_generation = ? AND runtime_instance_id = ?
		  AND execution_binding_digest = ? AND capsule_digest = ?
		  AND content_digest = ?
		ORDER BY checkpoint_id`,
		query.ConversationID, query.SegmentID, query.AttemptID,
		query.AgentInstanceID, query.WorkItemID, query.RunID,
		query.ClaimGeneration, query.RuntimeInstanceID,
		query.ExecutionBindingDigest, query.CapsuleDigest, query.ContentDigest,
	)
	if err != nil {
		return agentcheckpoint.Payload{}, ErrVaultUnavailable
	}
	defer rows.Close()
	ids := make([]string, 0, 2)
	for rows.Next() {
		var checkpointID string
		if rows.Scan(&checkpointID) != nil || !validContextCapsuleIdentifier(checkpointID) {
			return agentcheckpoint.Payload{}, ErrAgentCheckpointAuthentication
		}
		ids = append(ids, checkpointID)
		if len(ids) > 1 {
			return agentcheckpoint.Payload{}, ErrAgentCheckpointConflict
		}
	}
	if rows.Err() != nil {
		return agentcheckpoint.Payload{}, ErrVaultUnavailable
	}
	if len(ids) == 0 {
		return agentcheckpoint.Payload{}, ErrAgentCheckpointNotFound
	}
	record, found, err := readEncryptedAgentCheckpoint(ctx, store.database, ids[0])
	if err != nil {
		return agentcheckpoint.Payload{}, err
	}
	if !found || !agentCheckpointMatchesQuery(record.Binding, query) {
		return agentcheckpoint.Payload{}, ErrAgentCheckpointAuthentication
	}
	plaintext, err := store.decryptAgentCheckpointRecord(ctx, record)
	if err != nil {
		return agentcheckpoint.Payload{}, err
	}
	return agentcheckpoint.Payload{Binding: record.Binding, Content: plaintext}, nil
}

func (store *VaultStore) DeleteAgentCheckpoint(
	ctx context.Context,
	binding agentcheckpoint.Binding,
) error {
	if store == nil || ctx == nil || ctx.Err() != nil || !validAgentCheckpointBinding(binding) {
		return ErrInvalidAgentCheckpoint
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
	record, found, err := readEncryptedAgentCheckpoint(ctx, tx, binding.CheckpointID)
	if err != nil {
		return err
	}
	if !found {
		return ErrAgentCheckpointNotFound
	}
	if record.Binding != binding {
		return ErrAgentCheckpointBinding
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
	plaintext, err := decryptAgentCheckpoint(dek, record)
	clearBytes(plaintext)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(
		ctx, `DELETE FROM encrypted_agent_checkpoints WHERE checkpoint_id = ?`,
		binding.CheckpointID,
	)
	if err != nil || !exactlyOneRow(result) || tx.Commit() != nil {
		return ErrVaultUnavailable
	}
	return nil
}

func (store *VaultStore) decryptAgentCheckpointRecord(
	ctx context.Context,
	record encryptedAgentCheckpoint,
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
	return decryptAgentCheckpoint(dek, record)
}

func (store *VaultStore) encryptAgentCheckpoint(
	payload agentcheckpoint.Payload,
	dek []byte,
) (encryptedAgentCheckpoint, error) {
	if store == nil || store.cipher == nil || store.cipher.random == nil ||
		len(dek) != vaultKeyBytes || !validAgentCheckpointPayload(payload) {
		return encryptedAgentCheckpoint{}, ErrInvalidAgentCheckpoint
	}
	record := encryptedAgentCheckpoint{
		Payload: payload, SchemaVersion: agentCheckpointSchemaVersion,
		CipherVersion: agentCheckpointCipherVersion,
		DataNonce:     make([]byte, vaultNonceBytes),
	}
	if _, err := io.ReadFull(store.cipher.random, record.DataNonce); err != nil {
		return encryptedAgentCheckpoint{}, ErrVaultUnavailable
	}
	aad, err := canonicalAgentCheckpointAAD(record)
	if err != nil {
		return encryptedAgentCheckpoint{}, err
	}
	digest := sha256.Sum256(aad)
	record.AADDigest = append([]byte(nil), digest[:]...)
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return encryptedAgentCheckpoint{}, ErrVaultUnavailable
	}
	record.Ciphertext = aead.Seal(nil, record.DataNonce, payload.Content, aad)
	record.Content = nil
	return record, nil
}

func decryptAgentCheckpoint(dek []byte, record encryptedAgentCheckpoint) ([]byte, error) {
	if len(dek) != vaultKeyBytes || !validEncryptedAgentCheckpoint(record) {
		return nil, ErrAgentCheckpointAuthentication
	}
	aad, err := canonicalAgentCheckpointAAD(record)
	if err != nil {
		return nil, ErrAgentCheckpointAuthentication
	}
	digest := sha256.Sum256(aad)
	if subtle.ConstantTimeCompare(record.AADDigest, digest[:]) != 1 {
		return nil, ErrAgentCheckpointAuthentication
	}
	aead, err := newVaultAEAD(dek)
	if err != nil {
		return nil, ErrVaultUnavailable
	}
	plaintext, err := aead.Open(nil, record.DataNonce, record.Ciphertext, aad)
	if err != nil || len(plaintext) == 0 || len(plaintext) > maxAgentCheckpointBytes ||
		agentCheckpointDigest(plaintext) != record.Binding.ContentDigest {
		clearBytes(plaintext)
		return nil, ErrAgentCheckpointAuthentication
	}
	return plaintext, nil
}

func canonicalAgentCheckpointAAD(record encryptedAgentCheckpoint) ([]byte, error) {
	if record.SchemaVersion != agentCheckpointSchemaVersion ||
		record.CipherVersion != agentCheckpointCipherVersion ||
		!validAgentCheckpointBinding(record.Binding) {
		return nil, ErrInvalidAgentCheckpoint
	}
	var body bytes.Buffer
	body.WriteString("LOOM-AGENT-CHECKPOINT-AAD")
	_ = binary.Write(&body, binary.BigEndian, record.SchemaVersion)
	binding := record.Binding
	for _, value := range []string{
		binding.CheckpointID, binding.ConversationID, binding.SegmentID,
		binding.AttemptID, binding.AgentInstanceID, binding.WorkItemID, binding.RunID,
	} {
		writeCanonicalString(&body, value)
	}
	_ = binary.Write(&body, binary.BigEndian, binding.ClaimGeneration)
	for _, value := range []string{
		binding.RuntimeInstanceID, binding.ExecutionBindingDigest, binding.CapsuleDigest,
		binding.TurnID,
	} {
		writeCanonicalString(&body, value)
	}
	_ = binary.Write(&body, binary.BigEndian, int64(binding.TurnSequence))
	writeCanonicalString(&body, binding.StepID)
	_ = binary.Write(&body, binary.BigEndian, int64(binding.StepSequence))
	writeCanonicalString(&body, binding.ContentType)
	writeCanonicalString(&body, binding.ContentDigest)
	_ = binary.Write(&body, binary.BigEndian, record.CipherVersion)
	return body.Bytes(), nil
}

func readEncryptedAgentCheckpoint(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	checkpointID string,
) (encryptedAgentCheckpoint, bool, error) {
	record := encryptedAgentCheckpoint{}
	record.Binding.CheckpointID = checkpointID
	var schemaVersion, cipherVersion int64
	err := querier.QueryRowContext(ctx, `SELECT
		conversation_id, segment_id, attempt_id, agent_instance_id, work_item_id,
		run_id, claim_generation, runtime_instance_id, execution_binding_digest,
		capsule_digest, turn_id, turn_sequence, step_id, step_sequence,
		content_type, content_digest, schema_version, cipher_version,
		ciphertext, data_nonce, aad_digest
		FROM encrypted_agent_checkpoints WHERE checkpoint_id = ?`, checkpointID).Scan(
		&record.Binding.ConversationID, &record.Binding.SegmentID,
		&record.Binding.AttemptID, &record.Binding.AgentInstanceID,
		&record.Binding.WorkItemID, &record.Binding.RunID,
		&record.Binding.ClaimGeneration, &record.Binding.RuntimeInstanceID,
		&record.Binding.ExecutionBindingDigest, &record.Binding.CapsuleDigest,
		&record.Binding.TurnID, &record.Binding.TurnSequence,
		&record.Binding.StepID, &record.Binding.StepSequence,
		&record.Binding.ContentType, &record.Binding.ContentDigest,
		&schemaVersion, &cipherVersion, &record.Ciphertext,
		&record.DataNonce, &record.AADDigest,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return encryptedAgentCheckpoint{}, false, nil
	}
	if err != nil || schemaVersion < 0 || schemaVersion > 1<<16-1 ||
		cipherVersion < 0 || cipherVersion > 1<<16-1 {
		return encryptedAgentCheckpoint{}, false, ErrVaultUnavailable
	}
	record.SchemaVersion, record.CipherVersion = uint16(schemaVersion), uint16(cipherVersion)
	if !validEncryptedAgentCheckpoint(record) {
		return encryptedAgentCheckpoint{}, false, ErrAgentCheckpointAuthentication
	}
	return record, true, nil
}

func agentCheckpointPositionExists(
	ctx context.Context,
	querier interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	binding agentcheckpoint.Binding,
) (bool, error) {
	var checkpointID string
	err := querier.QueryRowContext(ctx, `SELECT checkpoint_id
		FROM encrypted_agent_checkpoints
		WHERE attempt_id = ? AND claim_generation = ?
		  AND turn_sequence = ? AND step_sequence = ?`,
		binding.AttemptID, binding.ClaimGeneration,
		binding.TurnSequence, binding.StepSequence,
	).Scan(&checkpointID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil || !validContextCapsuleIdentifier(checkpointID) {
		return false, ErrVaultUnavailable
	}
	return true, nil
}

func validAgentCheckpointPayload(payload agentcheckpoint.Payload) bool {
	return validAgentCheckpointBinding(payload.Binding) &&
		len(payload.Content) > 0 && len(payload.Content) <= maxAgentCheckpointBytes &&
		utf8.Valid(payload.Content) &&
		agentCheckpointDigest(payload.Content) == payload.Binding.ContentDigest
}

func validEncryptedAgentCheckpoint(record encryptedAgentCheckpoint) bool {
	return record.SchemaVersion == agentCheckpointSchemaVersion &&
		record.CipherVersion == agentCheckpointCipherVersion &&
		validAgentCheckpointBinding(record.Binding) &&
		len(record.Ciphertext) > 16 && len(record.Ciphertext) <= maxAgentCheckpointBytes+16 &&
		len(record.DataNonce) == vaultNonceBytes && len(record.AADDigest) == vaultAADDigestBytes
}

func validAgentCheckpointBinding(binding agentcheckpoint.Binding) bool {
	return validContextCapsuleIdentifier(binding.CheckpointID) &&
		validContextCapsuleIdentifier(binding.ConversationID) &&
		validContextCapsuleIdentifier(binding.SegmentID) &&
		validContextCapsuleIdentifier(binding.AttemptID) &&
		validContextCapsuleIdentifier(binding.AgentInstanceID) &&
		validContextCapsuleIdentifier(binding.WorkItemID) &&
		validContextCapsuleIdentifier(binding.RunID) && binding.ClaimGeneration > 0 &&
		validContextCapsuleIdentifier(binding.RuntimeInstanceID) &&
		validContextCapsuleDigest(binding.ExecutionBindingDigest) &&
		validContextCapsuleDigest(binding.CapsuleDigest) &&
		validContextCapsuleIdentifier(binding.TurnID) && binding.TurnSequence > 0 &&
		validContextCapsuleIdentifier(binding.StepID) && binding.StepSequence > 0 &&
		binding.ContentType == agentcheckpoint.ContentTypeTextUTF8 &&
		validContextCapsuleDigest(binding.ContentDigest)
}

func validAgentCheckpointQuery(query agentcheckpoint.Query) bool {
	return validContextCapsuleIdentifier(query.ConversationID) &&
		validContextCapsuleIdentifier(query.SegmentID) &&
		validContextCapsuleIdentifier(query.AttemptID) &&
		validContextCapsuleIdentifier(query.AgentInstanceID) &&
		validContextCapsuleIdentifier(query.WorkItemID) &&
		validContextCapsuleIdentifier(query.RunID) && query.ClaimGeneration > 0 &&
		validContextCapsuleIdentifier(query.RuntimeInstanceID) &&
		validContextCapsuleDigest(query.ExecutionBindingDigest) &&
		validContextCapsuleDigest(query.CapsuleDigest) &&
		validContextCapsuleDigest(query.ContentDigest)
}

func agentCheckpointMatchesQuery(
	binding agentcheckpoint.Binding,
	query agentcheckpoint.Query,
) bool {
	return binding.ConversationID == query.ConversationID &&
		binding.SegmentID == query.SegmentID && binding.AttemptID == query.AttemptID &&
		binding.AgentInstanceID == query.AgentInstanceID &&
		binding.WorkItemID == query.WorkItemID && binding.RunID == query.RunID &&
		binding.ClaimGeneration == query.ClaimGeneration &&
		binding.RuntimeInstanceID == query.RuntimeInstanceID &&
		binding.ExecutionBindingDigest == query.ExecutionBindingDigest &&
		binding.CapsuleDigest == query.CapsuleDigest &&
		binding.ContentDigest == query.ContentDigest
}

func agentCheckpointDigest(content []byte) string {
	digest := sha256.Sum256(content)
	const alphabet = "0123456789abcdef"
	result := make([]byte, sha256.Size*2)
	for index, value := range digest {
		result[index*2], result[index*2+1] = alphabet[value>>4], alphabet[value&0x0f]
	}
	return string(result)
}
