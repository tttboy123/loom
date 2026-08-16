package vault

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConversationDocumentStoreRoundTripRestartAndNoPlaintextAtRest(t *testing.T) {
	store, keyPath, databasePath := newConversationDocumentStoreFixture(t)
	document := ConversationDocument{
		ConversationID: "thread-alpha",
		Kind:           "loom.chat-thread.v1",
		Revision:       7,
		Payload:        []byte(`{"content":"alpha-sensitive-transcript"}`),
	}
	if err := store.PutConversationDocument(context.Background(), document); err != nil {
		t.Fatal(err)
	}
	got, err := store.ConversationDocuments(context.Background(), document.Kind)
	if err != nil || len(got) != 1 || !sameConversationDocument(got[0], document) {
		t.Fatalf("documents = %#v, %v", got, err)
	}
	clearConversationDocuments(got)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, document.Payload) ||
		bytes.Contains(database, []byte("alpha-sensitive-transcript")) {
		t.Fatal("Vault database contains plaintext Conversation document")
	}

	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(StoreConfig{
		DatabasePath: databasePath,
		KeyMaterial:  material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err = reopened.ConversationDocuments(context.Background(), document.Kind)
	if err != nil || len(got) != 1 || !sameConversationDocument(got[0], document) {
		t.Fatalf("restart documents = %#v, %v", got, err)
	}
	clearConversationDocuments(got)
}

func TestConversationDocumentStoreEnforcesMonotonicRevisionAndIdempotency(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := ConversationDocument{
		ConversationID: "thread-alpha", Kind: "loom.chat-thread.v1",
		Revision: 1, Payload: []byte("first-thread-state"),
	}
	if err := store.PutConversationDocument(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.PutConversationDocument(context.Background(), first); err != nil {
		t.Fatalf("idempotent write = %v", err)
	}
	drift := first
	drift.Payload = []byte("same-revision-different-state")
	if err := store.PutConversationDocument(
		context.Background(), drift,
	); !errors.Is(err, ErrConversationDocumentConflict) {
		t.Fatalf("same-revision drift = %v", err)
	}
	second := first
	second.Revision = 2
	second.Payload = []byte("second-thread-state")
	if err := store.PutConversationDocument(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := store.PutConversationDocument(
		context.Background(), first,
	); !errors.Is(err, ErrConversationDocumentConflict) {
		t.Fatalf("stale revision write = %v", err)
	}
	got, err := store.ConversationDocuments(context.Background(), first.Kind)
	if err != nil || len(got) != 1 || !sameConversationDocument(got[0], second) {
		t.Fatalf("latest document = %#v, %v", got, err)
	}
	clearConversationDocuments(got)
}

func TestConversationDocumentStoreRejectsCiphertextAndAuthoritySubstitution(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	document := ConversationDocument{
		ConversationID: "thread-alpha", Kind: "loom.chat-thread.v1",
		Revision: 1, Payload: []byte("thread-state"),
	}
	if err := store.PutConversationDocument(context.Background(), document); err != nil {
		t.Fatal(err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_conversation_documents
		    SET document_revision = 2
		  WHERE conversation_id = ? AND document_kind = ?`,
		document.ConversationID, document.Kind,
	); err != nil {
		t.Fatal(err)
	}
	got, err := store.ConversationDocuments(context.Background(), document.Kind)
	clearConversationDocuments(got)
	if !errors.Is(err, ErrConversationDocumentAuthentication) {
		t.Fatalf("revision substitution error = %v", err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_conversation_documents
		    SET document_revision = 1,
		        ciphertext = zeroblob(length(ciphertext))
		  WHERE conversation_id = ? AND document_kind = ?`,
		document.ConversationID, document.Kind,
	); err != nil {
		t.Fatal(err)
	}
	got, err = store.ConversationDocuments(context.Background(), document.Kind)
	clearConversationDocuments(got)
	if !errors.Is(err, ErrConversationDocumentAuthentication) {
		t.Fatalf("ciphertext substitution error = %v", err)
	}
}

func TestConversationDocumentStoreSurvivesVaultRotation(t *testing.T) {
	store, keyPath, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	document := ConversationDocument{
		ConversationID: "thread-alpha", Kind: "loom.chat-thread.v1",
		Revision: 1, Payload: []byte("rotation-thread-state"),
	}
	if err := store.PutConversationDocument(context.Background(), document); err != nil {
		t.Fatal(err)
	}
	newMaterial, err := (LocalKeyFile{Path: keyPath + ".next"}).Create(
		context.Background(), 2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RotateWrappingKey(context.Background(), newMaterial); err != nil {
		t.Fatal(err)
	}
	got, err := store.ConversationDocuments(context.Background(), document.Kind)
	if err != nil || len(got) != 1 || !sameConversationDocument(got[0], document) {
		t.Fatalf("rotated documents = %#v, %v", got, err)
	}
	clearConversationDocuments(got)
}

func TestConversationDocumentStoreRejectsNonceReusedByContextCapsule(t *testing.T) {
	root := t.TempDir()
	privateDir := filepath.Join(root, "private")
	stateDir := filepath.Join(root, "state")
	for _, directory := range []string{privateDir, stateDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	material, err := (LocalKeyFile{
		Path: filepath.Join(privateDir, "vault.key"),
	}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dek := bytes.Repeat([]byte{1}, vaultKeyBytes)
	wrapNonce := bytes.Repeat([]byte{2}, vaultNonceBytes)
	sharedDataNonce := bytes.Repeat([]byte{3}, vaultNonceBytes)
	store, err := OpenStore(StoreConfig{
		DatabasePath: filepath.Join(stateDir, "credential-vault.db"),
		KeyMaterial:  material,
		Random: bytes.NewReader(bytes.Join([][]byte{
			dek, wrapNonce, sharedDataNonce, sharedDataNonce,
		}, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	capsule, payload := testStoredContextCapsule(
		t, "thread-alpha", "capsule-context",
	)
	if err := store.PutContextCapsule(
		context.Background(), capsule.AuthorityRecord(), payload,
	); err != nil {
		t.Fatal(err)
	}
	document := ConversationDocument{
		ConversationID: "thread-alpha", Kind: "loom.chat-thread.v1",
		Revision: 1, Payload: []byte("thread-state"),
	}
	if err := store.PutConversationDocument(
		context.Background(), document,
	); !errors.Is(err, ErrVaultUnavailable) {
		t.Fatalf("cross-channel nonce reuse error = %v", err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), capsule.Target().ConversationID, capsule.Digest(),
	)
	if err != nil || !bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("capsule after rejected document = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
}

func newConversationDocumentStoreFixture(t *testing.T) (*VaultStore, string, string) {
	t.Helper()
	root := t.TempDir()
	privateDir := filepath.Join(root, "private")
	stateDir := filepath.Join(root, "state")
	for _, directory := range []string{privateDir, stateDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDir, "vault.key")
	databasePath := filepath.Join(stateDir, "credential-vault.db")
	material, err := (LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(StoreConfig{
		DatabasePath: databasePath,
		KeyMaterial:  material,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, keyPath, databasePath
}

func sameConversationDocument(left, right ConversationDocument) bool {
	return left.ConversationID == right.ConversationID &&
		left.Kind == right.Kind && left.Revision == right.Revision &&
		bytes.Equal(left.Payload, right.Payload)
}

func clearConversationDocuments(documents []ConversationDocument) {
	for index := range documents {
		clearBytes(documents[index].Payload)
	}
}
