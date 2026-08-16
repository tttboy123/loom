package vault

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExternalSessionHandleStoreRoundTripRestartAndNoPlaintextAtRest(t *testing.T) {
	store, keyPath, databasePath := newConversationDocumentStoreFixture(t)
	handle := testExternalSessionHandle("conversation-alpha", "segment-1")
	handle.Value = []byte("provider-native-handle-must-remain-encrypted")
	if err := store.PutExternalSessionHandle(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadExternalSessionHandle(
		context.Background(), handle.Binding, handle.Kind,
	)
	if err != nil || !sameExternalSessionHandle(got, handle) {
		t.Fatalf("handle = %#v, %v", got, err)
	}
	clearBytes(got.Value)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, handle.Value) ||
		bytes.Contains(database, []byte("provider-native-handle")) {
		t.Fatal("Vault database contains plaintext Provider-native handle")
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
	got, err = reopened.ReadExternalSessionHandle(
		context.Background(), handle.Binding, handle.Kind,
	)
	if err != nil || !sameExternalSessionHandle(got, handle) {
		t.Fatalf("restart handle = %#v, %v", got, err)
	}
	clearBytes(got.Value)
}

func TestExternalSessionHandleStoreFreezesRouteBindingAndMonotonicRevision(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	handle := testExternalSessionHandle("conversation-alpha", "segment-1")
	if err := store.PutExternalSessionHandle(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if err := store.PutExternalSessionHandle(context.Background(), handle); err != nil {
		t.Fatalf("idempotent write = %v", err)
	}
	drift := handle
	drift.Value = []byte("same-revision-different-handle")
	if err := store.PutExternalSessionHandle(
		context.Background(), drift,
	); !errors.Is(err, ErrExternalSessionHandleConflict) {
		t.Fatalf("same-revision drift = %v", err)
	}
	second := handle
	second.Revision = 2
	second.Value = []byte("next-provider-native-handle")
	if err := store.PutExternalSessionHandle(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if err := store.PutExternalSessionHandle(
		context.Background(), handle,
	); !errors.Is(err, ErrExternalSessionHandleConflict) {
		t.Fatalf("stale revision = %v", err)
	}
	got, err := store.ReadExternalSessionHandle(
		context.Background(), second.Binding, second.Kind,
	)
	if err != nil || !sameExternalSessionHandle(got, second) {
		t.Fatalf("latest handle = %#v, %v", got, err)
	}
	clearBytes(got.Value)
	wrongRoute := second
	wrongRoute.Revision = 3
	wrongRoute.Binding.ModelID = "deepseek-reasoner"
	wrongRoute.Value = []byte("must-not-cross-route-binding")
	if err := store.PutExternalSessionHandle(
		context.Background(), wrongRoute,
	); !errors.Is(err, ErrExternalSessionHandleBinding) {
		t.Fatalf("cross-route update = %v", err)
	}

	for name, mutate := range map[string]func(*ExternalSessionHandleBinding){
		"provider": func(binding *ExternalSessionHandleBinding) {
			binding.ProviderID = "anthropic"
			binding.ProviderAccountID = "anthropic.primary"
		},
		"account": func(binding *ExternalSessionHandleBinding) {
			binding.ProviderAccountID = "deepseek.backup"
		},
		"model": func(binding *ExternalSessionHandleBinding) {
			binding.ModelID = "deepseek-reasoner"
		},
		"auth-mode": func(binding *ExternalSessionHandleBinding) {
			binding.AuthMode = "provider_ephemeral"
		},
		"segment": func(binding *ExternalSessionHandleBinding) {
			binding.SegmentID = "segment-2"
		},
		"credential-reference": func(binding *ExternalSessionHandleBinding) {
			binding.CredentialReference = "credential-ref-deepseek-backup"
		},
		"credential-revision": func(binding *ExternalSessionHandleBinding) {
			binding.CredentialRevision++
		},
	} {
		t.Run(name, func(t *testing.T) {
			binding := second.Binding
			mutate(&binding)
			got, err := store.ReadExternalSessionHandle(
				context.Background(), binding, second.Kind,
			)
			clearBytes(got.Value)
			if !errors.Is(err, ErrExternalSessionHandleBinding) &&
				!errors.Is(err, ErrExternalSessionHandleNotFound) {
				t.Fatalf("binding substitution = %v", err)
			}
		})
	}
}

func TestExternalSessionHandleStoreSupportsNativeAuthWithoutInventingCredential(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	handle := ExternalSessionHandle{
		Binding: ExternalSessionHandleBinding{
			ConversationID: "conversation-codex", SegmentID: "segment-1",
			ProviderID: "openai", ModelID: "codex-5.5", AuthMode: "native_auth",
		},
		Kind: ExternalSessionHandleConversationID, Revision: 1,
		Value: []byte("native-runtime-session-handle"),
	}
	if err := store.PutExternalSessionHandle(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadExternalSessionHandle(
		context.Background(), handle.Binding, handle.Kind,
	)
	if err != nil || !sameExternalSessionHandle(got, handle) {
		t.Fatalf("native handle = %#v, %v", got, err)
	}
	clearBytes(got.Value)
	invalid := handle
	invalid.Binding.ProviderAccountID = "openai.primary"
	if err := store.PutExternalSessionHandle(
		context.Background(), invalid,
	); !errors.Is(err, ErrInvalidExternalSessionHandle) {
		t.Fatalf("native invented account = %v", err)
	}
}

func TestExternalSessionHandleStoreRejectsMetadataAndCiphertextTampering(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	handle := testExternalSessionHandle("conversation-alpha", "segment-1")
	if err := store.PutExternalSessionHandle(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_external_session_handles
		    SET model_id = 'deepseek-reasoner'
		  WHERE conversation_id = ? AND segment_id = ? AND handle_kind = ?`,
		handle.Binding.ConversationID, handle.Binding.SegmentID, handle.Kind,
	); err != nil {
		t.Fatal(err)
	}
	tamperedBinding := handle.Binding
	tamperedBinding.ModelID = "deepseek-reasoner"
	got, err := store.ReadExternalSessionHandle(
		context.Background(), tamperedBinding, handle.Kind,
	)
	clearBytes(got.Value)
	if !errors.Is(err, ErrExternalSessionHandleAuthentication) {
		t.Fatalf("metadata substitution = %v", err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_external_session_handles
		    SET model_id = ?, ciphertext = zeroblob(length(ciphertext))
		  WHERE conversation_id = ? AND segment_id = ? AND handle_kind = ?`,
		handle.Binding.ModelID, handle.Binding.ConversationID,
		handle.Binding.SegmentID, handle.Kind,
	); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadExternalSessionHandle(
		context.Background(), handle.Binding, handle.Kind,
	)
	clearBytes(got.Value)
	if !errors.Is(err, ErrExternalSessionHandleAuthentication) {
		t.Fatalf("ciphertext substitution = %v", err)
	}
}

func TestExternalSessionHandleStoreSurvivesRotationAndConversationErasure(t *testing.T) {
	store, keyPath, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	first := testExternalSessionHandle("conversation-alpha", "segment-1")
	second := testExternalSessionHandle("conversation-beta", "segment-1")
	second.Value = []byte("independent-provider-native-handle")
	for _, handle := range []ExternalSessionHandle{first, second} {
		if err := store.PutExternalSessionHandle(context.Background(), handle); err != nil {
			t.Fatal(err)
		}
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
	got, err := store.ReadExternalSessionHandle(
		context.Background(), first.Binding, first.Kind,
	)
	if err != nil || !sameExternalSessionHandle(got, first) {
		t.Fatalf("rotated handle = %#v, %v", got, err)
	}
	clearBytes(got.Value)
	if err := store.DeleteContextConversation(
		context.Background(), first.Binding.ConversationID,
	); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadExternalSessionHandle(
		context.Background(), first.Binding, first.Kind,
	)
	clearBytes(got.Value)
	if !errors.Is(err, ErrExternalSessionHandleNotFound) {
		t.Fatalf("crypto-erased handle = %v", err)
	}
	got, err = store.ReadExternalSessionHandle(
		context.Background(), second.Binding, second.Kind,
	)
	if err != nil || !sameExternalSessionHandle(got, second) {
		t.Fatalf("isolated handle = %#v, %v", got, err)
	}
	clearBytes(got.Value)
}

func TestExternalSessionHandleStoreRejectsNonceReusedByConversationDocument(t *testing.T) {
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
	document := ConversationDocument{
		ConversationID: "conversation-alpha", Kind: "loom.chat-thread.v1",
		Revision: 1, Payload: []byte("encrypted-thread-state"),
	}
	if err := store.PutConversationDocument(context.Background(), document); err != nil {
		t.Fatal(err)
	}
	handle := testExternalSessionHandle("conversation-alpha", "segment-1")
	if err := store.PutExternalSessionHandle(
		context.Background(), handle,
	); !errors.Is(err, ErrVaultUnavailable) {
		t.Fatalf("cross-channel nonce reuse = %v", err)
	}
	documents, err := store.ConversationDocuments(context.Background(), document.Kind)
	if err != nil || len(documents) != 1 ||
		!sameConversationDocument(documents[0], document) {
		t.Fatalf("document after rejected handle = %#v, %v", documents, err)
	}
	clearConversationDocuments(documents)
}

func testExternalSessionHandle(
	conversationID string,
	segmentID string,
) ExternalSessionHandle {
	return ExternalSessionHandle{
		Binding: ExternalSessionHandleBinding{
			ConversationID:      conversationID,
			SegmentID:           segmentID,
			ProviderID:          "deepseek",
			ProviderAccountID:   "deepseek.primary",
			ModelID:             "deepseek-chat",
			AuthMode:            "brokered",
			CredentialReference: "credential-ref-deepseek-primary",
			CredentialRevision:  7,
		},
		Kind:     ExternalSessionHandleConversationID,
		Revision: 1,
		Value:    []byte("opaque-provider-native-handle"),
	}
}

func sameExternalSessionHandle(left, right ExternalSessionHandle) bool {
	return left.Binding == right.Binding && left.Kind == right.Kind &&
		left.Revision == right.Revision && bytes.Equal(left.Value, right.Value)
}
