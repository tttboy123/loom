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

	"loom-pi-rebuild/internal/agentinbox"
)

func TestAgentInboxStoreEncryptsPersistsAndConsumesExactInput(t *testing.T) {
	store, keyPath, databasePath := newConversationDocumentStoreFixture(t)
	payload := testAgentInboxPayload("input-1", 1, "private queued input body")
	if err := store.PutAgentInput(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, payload.Content) {
		t.Fatal("Agent inbox payload persisted plaintext")
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
	pending, err := reopened.ListPendingAgentInputs(
		context.Background(), "mission:team-1", "run-agent-inbox", "agent-main", 5,
	)
	if err != nil || len(pending) != 1 || string(pending[0].Content) != "private queued input body" ||
		pending[0].Status != agentinbox.StatusPending {
		clearAgentInboxPayloads(pending)
		t.Fatalf("pending = %#v, %v", pending, err)
	}
	binding := pending[0].Binding
	clearAgentInboxPayloads(pending)
	if err := reopened.MarkAgentInputConsumed(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	if err := reopened.MarkAgentInputConsumed(context.Background(), binding); err != nil {
		t.Fatalf("idempotent consume = %v", err)
	}
	stored, err := reopened.ReadAgentInput(context.Background(), binding)
	if err != nil || stored.Status != agentinbox.StatusConsumed ||
		string(stored.Content) != "private queued input body" {
		stored.Close()
		t.Fatalf("consumed = %#v, %v", stored, err)
	}
	stored.Close()
}

func TestAgentInboxStoreRejectsBindingDriftTamperAndOrderReuse(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testAgentInboxPayload("input-1", 1, "first input")
	defer first.Close()
	if err := store.PutAgentInput(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.PutAgentInput(context.Background(), first); err != nil {
		t.Fatalf("idempotent put = %v", err)
	}
	duplicateOrder := testAgentInboxPayload("input-2", 1, "second input")
	defer duplicateOrder.Close()
	if err := store.PutAgentInput(context.Background(), duplicateOrder); !errors.Is(err, ErrAgentInputConflict) {
		t.Fatalf("order reuse = %v", err)
	}
	changed := first.Binding
	changed.TargetTurnID = "turn-substituted"
	if _, err := store.ReadAgentInput(context.Background(), changed); !errors.Is(err, ErrAgentInputBinding) {
		t.Fatalf("target substitution = %v", err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_agent_inputs SET segment_id = ? WHERE payload_id = ?`,
		"segment-substituted", first.Binding.PayloadID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAgentInput(context.Background(), first.Binding); !errors.Is(err, ErrAgentInputAuthentication) {
		t.Fatalf("AAD tamper = %v", err)
	}
}

func testAgentInboxPayload(inputID string, order int64, content string) agentinbox.Payload {
	digest := sha256.Sum256([]byte(content))
	return agentinbox.Payload{
		Binding: agentinbox.Binding{
			PayloadID: "agent-input-payload-" + inputID, InputID: inputID,
			Mode: agentinbox.ModeQueue, ContextScope: agentinbox.ScopeConversationShared,
			ConversationID: "mission:team-1", SegmentID: "segment-1",
			AgentInstanceID: "agent-main", WorkItemID: "work-agent-inbox",
			RunID: "run-agent-inbox", ClaimGeneration: 5,
			RuntimeInstanceID:      "runtime-a",
			ExecutionBindingDigest: strings.Repeat("a", 64),
			CapsuleDigest:          strings.Repeat("b", 64), OrderKey: order,
			TargetTurnID: "turn-" + inputID, TargetTurnSequence: int(order + 1),
			ContentType: "text/plain", ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: agentinbox.StatusPending, Content: []byte(content),
	}
}

func clearAgentInboxPayloads(payloads []agentinbox.Payload) {
	for index := range payloads {
		payloads[index].Close()
	}
}
