package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"loom-pi-rebuild/internal/attemptpayload"
	"os"
	"path/filepath"
	"testing"
)

func TestAttemptPayloadStorePersistsPendingDeliveryAcrossRestart(t *testing.T) {
	store, keyPath, databasePath := newConversationDocumentStoreFixture(t)
	payload := testAttemptPayload("payload-1", "run-1", 1, "tool body never plaintext")
	if err := store.PutAttemptPayload(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, payload.Content) {
		t.Fatal("attempt payload persisted plaintext")
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
	pending, err := reopened.ListPendingAttemptPayloads(
		context.Background(), testAttemptPayloadScope("run-1", 1),
	)
	if err != nil || len(pending) != 1 || string(pending[0].Content) != "tool body never plaintext" ||
		pending[0].Status != AttemptPayloadPending {
		clearAttemptPayloads(pending)
		t.Fatalf("pending = %#v, %v", pending, err)
	}
	clearAttemptPayloads(pending)
	identity := testAttemptPayload("payload-1", "run-1", 1, "tool body never plaintext")
	defer identity.Close()
	if err := reopened.MarkAttemptPayloadDelivered(context.Background(), identity.Binding); err != nil {
		t.Fatal(err)
	}
	if err := reopened.MarkAttemptPayloadDelivered(context.Background(), identity.Binding); err != nil {
		t.Fatalf("idempotent delivered mark: %v", err)
	}
	stored, err := reopened.ReadAttemptPayload(context.Background(), identity.Binding)
	if err != nil || stored.Status != AttemptPayloadDelivered ||
		string(stored.Content) != "tool body never plaintext" {
		stored.Close()
		t.Fatalf("delivered = %#v, %v", stored, err)
	}
	stored.Close()
	if pending, err := reopened.ListPendingAttemptPayloads(
		context.Background(), testAttemptPayloadScope("run-1", 1),
	); err != nil || len(pending) != 0 {
		clearAttemptPayloads(pending)
		t.Fatalf("delivered payload remained pending: %#v, %v", pending, err)
	}
}

func TestAttemptPayloadStoreRejectsIdentityDriftConflictAndTamper(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	payload := testAttemptPayload("payload-2", "run-2", 3, "bounded result")
	defer payload.Close()
	if err := store.PutAttemptPayload(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if err := store.PutAttemptPayload(context.Background(), payload); err != nil {
		t.Fatalf("idempotent put: %v", err)
	}
	conflict := testAttemptPayload("payload-2", "run-2", 3, "different result")
	defer conflict.Close()
	if err := store.PutAttemptPayload(context.Background(), conflict); !errors.Is(err, ErrAttemptPayloadConflict) {
		t.Fatalf("content conflict = %v", err)
	}
	for name, mutate := range map[string]func(*AttemptPayloadBinding){
		"generation": func(binding *AttemptPayloadBinding) { binding.ClaimGeneration++ },
		"binding":    func(binding *AttemptPayloadBinding) { binding.ExecutionBindingDigest = testDigest("other-binding") },
		"capsule":    func(binding *AttemptPayloadBinding) { binding.CapsuleDigest = testDigest("other-capsule") },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := payload.Binding
			mutate(&candidate)
			if _, err := store.ReadAttemptPayload(context.Background(), candidate); !errors.Is(err, ErrAttemptPayloadBinding) {
				t.Fatalf("drift read = %v", err)
			}
		})
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_attempt_payloads SET content_digest = ? WHERE payload_id = ?`,
		testDigest("tampered"), payload.Binding.PayloadID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAttemptPayload(context.Background(), payload.Binding); !errors.Is(err, ErrAttemptPayloadAuthentication) {
		t.Fatalf("tampered read = %v", err)
	}
}

func TestAttemptPayloadStoreAuthenticatesDeliveryStatus(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	payload := testAttemptPayload("payload-status", "run-status", 1, "pending body")
	defer payload.Close()
	if err := store.PutAttemptPayload(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_attempt_payloads SET status = 'delivered' WHERE payload_id = ?`,
		payload.Binding.PayloadID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAttemptPayload(context.Background(), payload.Binding); !errors.Is(err, ErrAttemptPayloadAuthentication) {
		t.Fatalf("status tamper read = %v", err)
	}
	if _, err := store.ListPendingAttemptPayloads(
		context.Background(), payload.Binding.Scope,
	); !errors.Is(err, ErrAttemptPayloadAuthentication) {
		t.Fatalf("status tamper recovery = %v", err)
	}
}

func TestAttemptPayloadStoreIsolatesAttemptsAndSurvivesVaultRotation(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testAttemptPayload("payload-3", "run-3", 1, "first result")
	second := testAttemptPayload("payload-4", "run-4", 1, "second result")
	defer first.Close()
	defer second.Close()
	for _, payload := range []AttemptPayload{first, second} {
		if err := store.PutAttemptPayload(context.Background(), payload); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := store.ListPendingAttemptPayloads(
		context.Background(), testAttemptPayloadScope("run-3", 1),
	)
	if err != nil || len(pending) != 1 || pending[0].Binding.PayloadID != "payload-3" {
		clearAttemptPayloads(pending)
		t.Fatalf("isolated pending = %#v, %v", pending, err)
	}
	clearAttemptPayloads(pending)
	stale := testAttemptPayloadScope("run-3", 2)
	if _, err := store.ListPendingAttemptPayloads(context.Background(), stale); !errors.Is(err, ErrAttemptPayloadBinding) {
		t.Fatalf("stale recovery scope = %v", err)
	}

	pendingKeyPath := filepath.Join(
		filepath.Dir(filepath.Dir(store.databasePath)),
		"private", "vault.key.rotation-pending",
	)
	newMaterial, err := (LocalKeyFile{Path: pendingKeyPath}).Create(
		context.Background(), store.keyMaterial.KeyVersion()+1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RotateWrappingKey(context.Background(), newMaterial); err != nil {
		newMaterial.Close()
		t.Fatal(err)
	}
	stored, err := store.ReadAttemptPayload(context.Background(), first.Binding)
	if err != nil || string(stored.Content) != "first result" {
		stored.Close()
		t.Fatalf("rotated payload = %#v, %v", stored, err)
	}
	stored.Close()
	if err := store.DeleteAttemptPayload(context.Background(), first.Binding); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAttemptPayload(context.Background(), first.Binding); !errors.Is(err, ErrAttemptPayloadNotFound) {
		t.Fatalf("deleted payload read = %v", err)
	}
	peer, err := store.ReadAttemptPayload(context.Background(), second.Binding)
	if err != nil || string(peer.Content) != "second result" {
		peer.Close()
		t.Fatalf("peer payload = %#v, %v", peer, err)
	}
	peer.Close()
}

func TestAttemptPayloadStoreRejectsSequenceReuseAndConversationEraseIsLocal(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testAttemptPayload("payload-5", "run-5", 1, "first sequence result")
	duplicate := testAttemptPayload("payload-6", "run-5", 1, "different sequence result")
	peer := testAttemptPayload("payload-7", "run-7", 1, "peer conversation result")
	peer.Binding.ConversationID = "mission:team-2"
	defer first.Close()
	defer duplicate.Close()
	defer peer.Close()
	if err := store.PutAttemptPayload(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.PutAttemptPayload(context.Background(), duplicate); !errors.Is(err, ErrAttemptPayloadConflict) {
		t.Fatalf("sequence reuse = %v", err)
	}
	if err := store.PutAttemptPayload(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteContextConversation(
		context.Background(), first.Binding.ConversationID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadAttemptPayload(context.Background(), first.Binding); !errors.Is(err, ErrAttemptPayloadNotFound) {
		t.Fatalf("erased conversation payload = %v", err)
	}
	storedPeer, err := store.ReadAttemptPayload(context.Background(), peer.Binding)
	if err != nil || string(storedPeer.Content) != "peer conversation result" {
		storedPeer.Close()
		t.Fatalf("peer after erase = %#v, %v", storedPeer, err)
	}
	storedPeer.Close()
}

func testAttemptPayload(payloadID, runID string, generation int64, content string) AttemptPayload {
	body := []byte(content)
	digest := sha256.Sum256(body)
	return AttemptPayload{
		Binding: AttemptPayloadBinding{
			PayloadID: payloadID,
			Scope: AttemptPayloadScope{
				ConversationID: "mission:team-1", WorkItemID: "work-1",
				RunID: runID, ClaimGeneration: generation,
				RuntimeInstanceID:      "runtime-1",
				ExecutionBindingDigest: testDigest("binding-" + runID),
				CapsuleDigest:          testDigest("capsule-" + runID),
			},
			CallID: "call-1", Sequence: 1, ContentType: "application/json",
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: AttemptPayloadPending, Content: body,
	}
}

func testAttemptPayloadScope(runID string, generation int64) AttemptPayloadScope {
	return AttemptPayloadScope{
		ConversationID: "mission:team-1", WorkItemID: "work-1", RunID: runID,
		ClaimGeneration: generation, RuntimeInstanceID: "runtime-1",
		ExecutionBindingDigest: testDigest("binding-" + runID),
		CapsuleDigest:          testDigest("capsule-" + runID),
	}
}

func testDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func clearAttemptPayloads(payloads []AttemptPayload) {
	for index := range payloads {
		payloads[index].Close()
	}
}

func TestValidAttemptPayloadContentTypeAcceptsCanonicalMIME(t *testing.T) {
	// The vault content-type gate must accept the canonical Attempt payload
	// MIME types, including the standard "text/plain; charset=utf-8" form
	// used by governed remote-tool results. Rejecting it bricked web tool
	// result persistence in Missions (result_persistence_failed).
	if !validAttemptPayloadContentType(attemptpayload.ContentTypeJSON) ||
		!validAttemptPayloadContentType(attemptpayload.ContentTypeTextUTF8) ||
		validAttemptPayloadContentType("") ||
		validAttemptPayloadContentType("text/html") {
		t.Fatal("vault attempt payload content-type validation incorrect")
	}
}
