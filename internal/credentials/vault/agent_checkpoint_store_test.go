package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/agentcheckpoint"
)

func TestAgentCheckpointStoreEncryptsPersistsAndReadsExactCheckpoint(t *testing.T) {
	store, keyPath, databasePath := newConversationDocumentStoreFixture(t)
	payload := testAgentCheckpointPayload("checkpoint-1", 1, 1, "private model checkpoint body")
	if err := store.PutAgentCheckpoint(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, payload.Content) {
		t.Fatal("Agent checkpoint persisted plaintext")
	}
	payload.Close()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(StoreConfig{DatabasePath: databasePath, KeyMaterial: material})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	binding := testAgentCheckpointPayload("checkpoint-1", 1, 1, "private model checkpoint body").Binding
	stored, err := reopened.ReadAgentCheckpoint(context.Background(), binding)
	if err != nil || string(stored.Content) != "private model checkpoint body" {
		stored.Close()
		t.Fatalf("checkpoint = %#v, %v", stored, err)
	}
	stored.Close()
}

func TestAgentCheckpointStoreRejectsBindingDriftTamperAndPositionReuse(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testAgentCheckpointPayload("checkpoint-1", 1, 1, "first checkpoint")
	defer first.Close()
	if err := store.PutAgentCheckpoint(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.PutAgentCheckpoint(context.Background(), first); err != nil {
		t.Fatalf("idempotent put = %v", err)
	}
	duplicatePosition := testAgentCheckpointPayload("checkpoint-2", 1, 1, "second checkpoint")
	defer duplicatePosition.Close()
	if err := store.PutAgentCheckpoint(context.Background(), duplicatePosition); !errors.Is(err, ErrAgentCheckpointConflict) {
		t.Fatalf("position reuse = %v", err)
	}
	changed := first.Binding
	changed.SegmentID = "segment-substituted"
	if _, err := store.ReadAgentCheckpoint(context.Background(), changed); !errors.Is(err, ErrAgentCheckpointBinding) {
		t.Fatalf("segment substitution = %v", err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_agent_checkpoints SET runtime_instance_id = ? WHERE checkpoint_id = ?`,
		"runtime-substituted", first.Binding.CheckpointID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAgentCheckpoint(context.Background(), first.Binding); !errors.Is(err, ErrAgentCheckpointAuthentication) {
		t.Fatalf("AAD tamper = %v", err)
	}
}

func TestAgentCheckpointStoreDeleteAndConversationCryptoErase(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testAgentCheckpointPayload("checkpoint-delete", 1, 1, "delete me")
	defer first.Close()
	if err := store.PutAgentCheckpoint(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAgentCheckpoint(context.Background(), first.Binding); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAgentCheckpoint(context.Background(), first.Binding); !errors.Is(err, ErrAgentCheckpointNotFound) {
		t.Fatalf("deleted read = %v", err)
	}
	second := testAgentCheckpointPayload("checkpoint-erased", 2, 1, "erase with conversation")
	defer second.Close()
	if err := store.PutAgentCheckpoint(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteContextConversation(context.Background(), second.Binding.ConversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAgentCheckpoint(context.Background(), second.Binding); !errors.Is(err, ErrAgentCheckpointNotFound) {
		t.Fatalf("crypto-erased read = %v", err)
	}
}

func TestAgentCheckpointStoreResolvesExactlyOneFrozenRouteAndRejectsAmbiguity(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testAgentCheckpointPayload("checkpoint-resolve-1", 1, 1, "repeated output")
	defer first.Close()
	if err := store.PutAgentCheckpoint(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	query := agentCheckpointQuery(first.Binding)
	resolved, err := store.ResolveAgentCheckpoint(context.Background(), query)
	if err != nil || resolved.Binding != first.Binding || string(resolved.Content) != "repeated output" {
		resolved.Close()
		t.Fatalf("resolved checkpoint = %#v, %v", resolved, err)
	}
	resolved.Close()
	substituted := query
	substituted.SegmentID = "segment-substituted"
	if _, err := store.ResolveAgentCheckpoint(context.Background(), substituted); !errors.Is(err, ErrAgentCheckpointNotFound) {
		t.Fatalf("substituted route = %v", err)
	}
	second := testAgentCheckpointPayload("checkpoint-resolve-2", 1, 2, "repeated output")
	defer second.Close()
	if err := store.PutAgentCheckpoint(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveAgentCheckpoint(context.Background(), query); !errors.Is(err, ErrAgentCheckpointConflict) {
		t.Fatalf("ambiguous checkpoint = %v", err)
	}
}

func agentCheckpointQuery(binding agentcheckpoint.Binding) agentcheckpoint.Query {
	return agentcheckpoint.Query{
		ConversationID: binding.ConversationID, SegmentID: binding.SegmentID,
		AttemptID: binding.AttemptID, AgentInstanceID: binding.AgentInstanceID,
		WorkItemID: binding.WorkItemID, RunID: binding.RunID,
		ClaimGeneration:        binding.ClaimGeneration,
		RuntimeInstanceID:      binding.RuntimeInstanceID,
		ExecutionBindingDigest: binding.ExecutionBindingDigest,
		CapsuleDigest:          binding.CapsuleDigest, ContentDigest: binding.ContentDigest,
	}
}

func testAgentCheckpointPayload(checkpointID string, turn, step int, content string) agentcheckpoint.Payload {
	digest := sha256.Sum256([]byte(content))
	return agentcheckpoint.Payload{
		Binding: agentcheckpoint.Binding{
			CheckpointID: checkpointID, ConversationID: "mission:team-1",
			SegmentID: "segment-1", AttemptID: "attempt-checkpoint",
			AgentInstanceID: "agent-main", WorkItemID: "work-checkpoint",
			RunID: "run-checkpoint", ClaimGeneration: 5,
			RuntimeInstanceID:      "runtime-a",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			CapsuleDigest:          strings.Repeat("b", 64),
			TurnID:                 "turn-checkpoint", TurnSequence: turn,
			StepID: "step-checkpoint", StepSequence: step,
			ContentType:   agentcheckpoint.ContentTypeTextUTF8,
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Content: []byte(content),
	}
}
