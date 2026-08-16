package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/contextcapsule"
)

func TestContextCapsuleStoreRoundTripRestartAndNoPlaintextAtRest(t *testing.T) {
	store, keyPath, databasePath := newContextCapsuleStoreFixture(t)
	capsule, payload := testStoredContextCapsule(t, "conversation-alpha", "alpha-sensitive-context")
	record := capsule.AuthorityRecord()
	if err := store.PutContextCapsule(context.Background(), record, payload); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !reflect.DeepEqual(stored.Authority, record) ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("stored capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, []byte("alpha-sensitive-context")) ||
		bytes.Contains(database, payload) {
		t.Fatal("Vault database contains plaintext Context Capsule payload")
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
	stored, err = reopened.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !reflect.DeepEqual(stored.Authority, record) ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("restarted capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
}

func TestContextCapsuleStoreRestoresCanonicalRoleCapsuleAfterRestart(t *testing.T) {
	store, keyPath, databasePath := newContextCapsuleStoreFixture(t)
	capsule, payload := testStoredContextCapsule(
		t, "conversation-authority", "authoritative-goal-sensitive-context",
	)
	record := capsule.AuthorityRecord()
	if err := store.PutRoleContextCapsule(
		context.Background(), capsule, payload,
	); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !stored.CapsuleAvailable ||
		!reflect.DeepEqual(stored.Authority, record) ||
		stored.Capsule.AuthorityRecord() != record ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("stored role capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, []byte("authoritative-goal-sensitive-context")) ||
		bytes.Contains(database, payload) {
		t.Fatal("Vault database contains plaintext canonical Role Context Capsule")
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
	stored, err = reopened.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !stored.CapsuleAvailable ||
		stored.Capsule.AuthorityRecord() != record ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("restarted role capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
}

func TestContextCapsuleStoreRestoresStructuredConversationCapsuleAfterRestart(t *testing.T) {
	store, keyPath, databasePath := newContextCapsuleStoreFixture(t)
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-route", TeamID: "conversation:conversation-route",
			AgentID: "conversation-agent", RoleID: "segment-2",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ModelID: "deepseek-chat", AuthMode: "brokered",
			ContextAdapterID:        "context:loom-native:v1",
			DisclosurePolicyID:      "provider-account-policy:" + strings.Repeat("a", 64),
			DisclosurePolicyVersion: 2, TokenBudget: 2_048,
		},
		[]contextcapsule.ItemInput{
			{
				ItemID: "message-msg-1", Kind: contextcapsule.KindRecentUserTurn,
				Trust:    contextcapsule.TrustAuthoritative,
				Scope:    contextcapsule.ScopeConversationShared,
				Priority: contextcapsule.PriorityWorkspace, TokenCount: 7,
				Content:    []byte("private confirmed user context"),
				SourceType: contextcapsule.SourceAuthority,
				SourceRef:  "conversation-message:msg-1",
			},
			{
				ItemID: "message-msg-2", Kind: contextcapsule.KindPriorModelOutput,
				Trust:    contextcapsule.TrustUntrusted,
				Scope:    contextcapsule.ScopeConversationShared,
				Priority: contextcapsule.PriorityHistory, TokenCount: 6,
				Content:    []byte("untrusted provider output"),
				SourceType: contextcapsule.SourceModelOutput,
				SourceRef:  "conversation-message:msg-2", PolicyFiltered: true,
			},
			{
				ItemID: "message-msg-3", Kind: contextcapsule.KindRecentUserTurn,
				Trust:    contextcapsule.TrustAuthoritative,
				Scope:    contextcapsule.ScopeConversationShared,
				Priority: contextcapsule.PriorityConfirmed, TokenCount: 4, Required: true,
				Content: []byte("continue safely"), SourceType: contextcapsule.SourceAuthority,
				SourceRef: "conversation-message:msg-3",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutRoleContextCapsule(context.Background(), capsule, payload); err != nil {
		t.Fatal(err)
	}
	record := capsule.AuthorityRecord()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{
		[]byte("private confirmed user context"),
		[]byte("untrusted provider output"),
		[]byte("continue safely"),
	} {
		if bytes.Contains(database, forbidden) {
			t.Fatalf("Vault database contains conversation plaintext %q", forbidden)
		}
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
	stored, err := reopened.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !stored.CapsuleAvailable ||
		stored.Capsule.AuthorityRecord() != record ||
		len(stored.Capsule.Omitted()) != 1 ||
		stored.Capsule.Omitted()[0].Reason != contextcapsule.OmissionPolicyFiltered ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("stored=%#v error=%v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
}

func TestContextCapsuleStoreDeletesOnlyExactConversationAuthority(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	capsule, payload := testStoredContextCapsule(
		t, "conversation-delete", "conversation-delete-sensitive-context",
	)
	record := capsule.AuthorityRecord()
	if err := store.PutRoleContextCapsule(
		context.Background(), capsule, payload,
	); err != nil {
		t.Fatal(err)
	}
	forged := record
	forged.ModelID = "substituted-model"
	if err := store.DeleteRoleContextCapsule(
		context.Background(), forged,
	); !errors.Is(err, ErrInvalidContextCapsuleStore) {
		t.Fatalf("forged authority delete error = %v", err)
	}
	if _, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	); err != nil {
		t.Fatalf("forged delete removed record: %v", err)
	}
	if err := store.DeleteRoleContextCapsule(
		context.Background(), record,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	); !errors.Is(err, ErrContextCapsuleNotFound) {
		t.Fatalf("deleted record read error = %v", err)
	}
	if err := store.DeleteRoleContextCapsule(
		context.Background(), record,
	); err != nil {
		t.Fatalf("idempotent delete = %v", err)
	}
}

func TestContextCapsuleStoreRetrievesScopedBudgetOmissionAfterRestart(t *testing.T) {
	store, keyPath, databasePath := newContextCapsuleStoreFixture(t)
	target := contextcapsule.Target{
		ConversationID: "conversation-retrieval", TeamID: "team-retrieval",
		AgentID: "agent-reviewer", RoleID: "reviewer",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ModelID: "deepseek-chat", AuthMode: "brokered",
		ContextAdapterID:   "context:loom-native:v1",
		DisclosurePolicyID: "loom.local-team-disclosure", DisclosurePolicyVersion: 1,
		ArtifactRefs: []string{"artifact:diff-1"}, TokenBudget: 4,
	}
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		target,
		[]contextcapsule.ItemInput{
			{
				ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
				Content:    []byte("Review the encrypted artifact."),
				SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:vault-retrieval",
			},
			{
				ItemID: "diff-detail", Kind: contextcapsule.KindArtifactReference,
				Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
				Priority: contextcapsule.PriorityRetrievable, TokenCount: 8,
				Content:    []byte("encrypted scoped retrieval body"),
				SourceType: contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
				ArtifactRef: "artifact:diff-1",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutRoleContextCapsule(context.Background(), capsule, payload); err != nil {
		t.Fatal(err)
	}
	omitted := capsule.Omitted()[0]
	request := contextcapsule.RetrievalRequest{
		Authority: capsule.AuthorityRecord(), ItemID: omitted.ItemID,
		ContentDigest:    omitted.ContentDigest,
		RequesterAgentID: target.AgentID, RequesterRoleID: target.RoleID,
		ArtifactRef: "artifact:diff-1",
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, []byte("encrypted scoped retrieval body")) {
		t.Fatal("Vault database contains plaintext retrievable context")
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
	item, err := reopened.RetrieveContextItem(context.Background(), request)
	if err != nil || string(item.Content) != "encrypted scoped retrieval body" {
		t.Fatalf("retrieved = %#v, %v", item, err)
	}
	clearBytes(item.Content)

	forged := request
	forged.RequesterRoleID = "coder"
	if _, err := reopened.RetrieveContextItem(context.Background(), forged); !errors.Is(err, contextcapsule.ErrContextRetrievalDenied) {
		t.Fatalf("cross-role retrieval error = %v", err)
	}
}

func TestContextCapsuleStoreMarksLegacyDispatchOnlyRecords(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	capsule, payload := testStoredContextCapsule(
		t, "conversation-legacy", "legacy-sensitive-context",
	)
	record := capsule.AuthorityRecord()
	if err := store.PutContextCapsule(
		context.Background(), record, payload,
	); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || stored.CapsuleAvailable || stored.Capsule.Valid() ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("legacy capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
}

func TestContextCapsuleStoreUpgradesMatchingLegacyRecordAtomically(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	capsule, payload := testStoredContextCapsule(
		t, "conversation-upgrade", "upgrade-sensitive-context",
	)
	record := capsule.AuthorityRecord()
	if err := store.PutContextCapsule(
		context.Background(), record, payload,
	); err != nil {
		t.Fatal(err)
	}
	if err := store.PutRoleContextCapsule(
		context.Background(), capsule, payload,
	); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !stored.CapsuleAvailable ||
		stored.Capsule.AuthorityRecord() != record ||
		!bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("upgraded capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)

	if err := store.PutContextCapsule(
		context.Background(), record, payload,
	); !errors.Is(err, ErrContextCapsuleAuthentication) {
		t.Fatalf("legacy downgrade error = %v", err)
	}
}

func TestContextCapsuleStoreUpgradesStructuredV1ToRetrievableV2(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	target := contextcapsule.Target{
		ConversationID: "conversation-v1-v2", TeamID: "team-v1-v2",
		AgentID: "agent-reviewer", RoleID: "reviewer",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ModelID: "deepseek-chat", AuthMode: "brokered",
		ContextAdapterID:   "context:loom-native:v1",
		DisclosurePolicyID: "loom.local-team-disclosure", DisclosurePolicyVersion: 1,
		TokenBudget: 3,
	}
	capsule, err := contextcapsule.BuildRoleContextCapsule(target, []contextcapsule.ItemInput{
		{
			ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 3, Required: true,
			Content: []byte("Review safely."), SourceType: contextcapsule.SourceAuthority,
			SourceRef: "goal:v1-v2",
		},
		{
			ItemID: "history", Kind: contextcapsule.KindPriorModelOutput,
			Trust: contextcapsule.TrustUntrusted, Scope: contextcapsule.ScopeConversationShared,
			Priority: contextcapsule.PriorityRetrievable, TokenCount: 4,
			Content: []byte("retrievable prior output"), SourceType: contextcapsule.SourceModelOutput,
			SourceRef: "attempt:old-v1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	body, err := contextcapsule.MarshalCanonicalRoleContextCapsule(capsule)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := json.Marshal(contextCapsuleEnvelope{
		SchemaVersion:   contextCapsuleEnvelopeV1,
		CapsuleBody:     append(json.RawMessage(nil), body...),
		DispatchPayload: append(json.RawMessage(nil), payload...),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutContextCapsule(context.Background(), capsule.AuthorityRecord(), v1); err != nil {
		t.Fatal(err)
	}
	if err := store.PutRoleContextCapsule(context.Background(), capsule, payload); err != nil {
		t.Fatalf("v1 to v2 upgrade: %v", err)
	}
	omitted := capsule.Omitted()[0]
	item, err := store.RetrieveContextItem(context.Background(), contextcapsule.RetrievalRequest{
		Authority: capsule.AuthorityRecord(), ItemID: omitted.ItemID,
		ContentDigest:    omitted.ContentDigest,
		RequesterAgentID: target.AgentID, RequesterRoleID: target.RoleID,
	})
	if err != nil || string(item.Content) != "retrievable prior output" {
		t.Fatalf("retrieved = %#v, %v", item, err)
	}
	item.Close()
	if err := store.PutContextCapsule(
		context.Background(), capsule.AuthorityRecord(), v1,
	); !errors.Is(err, ErrContextCapsuleAuthentication) {
		t.Fatalf("v2 downgrade error = %v", err)
	}
}

func TestContextCapsuleStoreListsValidatedConversationAuthorityManifest(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	first, firstPayload := testStoredContextCapsule(
		t, "conversation-manifest", "first-sensitive-context",
	)
	second, secondPayload := testStoredContextCapsule(
		t, "conversation-manifest", "second-sensitive-context",
	)
	other, otherPayload := testStoredContextCapsule(
		t, "conversation-other", "other-sensitive-context",
	)
	for _, candidate := range []struct {
		capsule contextcapsule.RoleContextCapsule
		payload []byte
	}{{first, firstPayload}, {second, secondPayload}, {other, otherPayload}} {
		if err := store.PutRoleContextCapsule(
			context.Background(), candidate.capsule, candidate.payload,
		); err != nil {
			t.Fatal(err)
		}
	}
	records, err := store.ListContextCapsuleAuthorities(
		context.Background(), "conversation-manifest",
	)
	if err != nil || len(records) != 2 {
		t.Fatalf("authority manifest = %#v, %v", records, err)
	}
	want := map[contextcapsule.AuthorityRecord]bool{
		first.AuthorityRecord():  true,
		second.AuthorityRecord(): true,
	}
	for _, record := range records {
		if !want[record] {
			t.Fatalf("unexpected authority = %#v", record)
		}
	}
}

func TestContextCapsuleStoreRejectsIdentityAndCiphertextSubstitution(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	capsule, payload := testStoredContextCapsule(t, "conversation-alpha", "alpha-context")
	record := capsule.AuthorityRecord()
	if err := store.PutContextCapsule(context.Background(), record, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadContextCapsule(
		context.Background(), "conversation-beta", record.CapsuleDigest,
	); !errors.Is(err, ErrContextCapsuleNotFound) {
		t.Fatalf("cross-conversation read error = %v", err)
	}
	if _, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, strings.Repeat("a", 64),
	); !errors.Is(err, ErrContextCapsuleNotFound) {
		t.Fatalf("digest substitution read error = %v", err)
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_context_capsules
		    SET ciphertext = zeroblob(length(ciphertext))
		  WHERE capsule_digest = ?`,
		record.CapsuleDigest,
	); err != nil {
		t.Fatal(err)
	}
	if stored, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	); !errors.Is(err, ErrContextCapsuleAuthentication) {
		clearBytes(stored.DispatchPayload)
		t.Fatalf("tampered ciphertext error = %v", err)
	}
}

func TestContextCapsuleStoreCryptoErasureIsConversationLocal(t *testing.T) {
	store, _, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	first, firstPayload := testStoredContextCapsule(t, "conversation-alpha", "alpha-context")
	second, secondPayload := testStoredContextCapsule(t, "conversation-beta", "beta-context")
	for _, candidate := range []struct {
		record  contextcapsule.AuthorityRecord
		payload []byte
	}{
		{first.AuthorityRecord(), firstPayload},
		{second.AuthorityRecord(), secondPayload},
	} {
		if err := store.PutContextCapsule(
			context.Background(), candidate.record, candidate.payload,
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteContextConversation(
		context.Background(), first.Target().ConversationID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadContextCapsule(
		context.Background(), first.Target().ConversationID, first.Digest(),
	); !errors.Is(err, ErrContextCapsuleNotFound) {
		t.Fatalf("crypto-erased capsule error = %v", err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), second.Target().ConversationID, second.Digest(),
	)
	if err != nil || !bytes.Equal(stored.DispatchPayload, secondPayload) {
		t.Fatalf("isolated capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
	var keys, capsules int
	if err := store.database.QueryRow(
		`SELECT COUNT(*) FROM encrypted_conversation_keys`,
	).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	if err := store.database.QueryRow(
		`SELECT COUNT(*) FROM encrypted_context_capsules`,
	).Scan(&capsules); err != nil {
		t.Fatal(err)
	}
	if keys != 1 || capsules != 1 {
		t.Fatalf("remaining keys/capsules = %d/%d", keys, capsules)
	}
}

func TestContextCapsuleStoreRewrapsConversationDEKsOnVaultRotation(t *testing.T) {
	store, keyPath, _ := newContextCapsuleStoreFixture(t)
	defer store.Close()
	capsule, payload := testStoredContextCapsule(t, "conversation-alpha", "rotation-context")
	record := capsule.AuthorityRecord()
	if err := store.PutContextCapsule(context.Background(), record, payload); err != nil {
		t.Fatal(err)
	}
	newKeyPath := keyPath + ".next"
	newMaterial, err := (LocalKeyFile{Path: newKeyPath}).Create(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RotateWrappingKey(context.Background(), newMaterial); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), record.ConversationID, record.CapsuleDigest,
	)
	if err != nil || !bytes.Equal(stored.DispatchPayload, payload) {
		t.Fatalf("rotated capsule = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
	var keyVersion uint32
	if err := store.database.QueryRow(
		`SELECT key_version FROM encrypted_conversation_keys
		  WHERE conversation_id = ?`,
		record.ConversationID,
	).Scan(&keyVersion); err != nil {
		t.Fatal(err)
	}
	if keyVersion != 2 {
		t.Fatalf("conversation key version = %d", keyVersion)
	}
}

func TestContextCapsuleStoreRejectsRepeatedDataNonce(t *testing.T) {
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
	dataNonce := bytes.Repeat([]byte{3}, vaultNonceBytes)
	random := bytes.NewReader(bytes.Join([][]byte{
		dek, wrapNonce, dataNonce, dataNonce,
	}, nil))
	store, err := OpenStore(StoreConfig{
		DatabasePath: filepath.Join(stateDir, "credential-vault.db"),
		KeyMaterial:  material,
		Random:       random,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, firstPayload := testStoredContextCapsule(
		t, "conversation-alpha", "first-context",
	)
	second, secondPayload := testStoredContextCapsule(
		t, "conversation-alpha", "second-context",
	)
	if err := store.PutContextCapsule(
		context.Background(), first.AuthorityRecord(), firstPayload,
	); err != nil {
		t.Fatal(err)
	}
	if err := store.PutContextCapsule(
		context.Background(), second.AuthorityRecord(), secondPayload,
	); !errors.Is(err, ErrVaultUnavailable) {
		t.Fatalf("repeated data nonce error = %v", err)
	}
	stored, err := store.ReadContextCapsule(
		context.Background(), first.Target().ConversationID, first.Digest(),
	)
	if err != nil || !bytes.Equal(stored.DispatchPayload, firstPayload) {
		t.Fatalf("first capsule after rejected nonce = %#v, %v", stored, err)
	}
	clearBytes(stored.DispatchPayload)
}

func newContextCapsuleStoreFixture(t *testing.T) (*VaultStore, string, string) {
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

func testStoredContextCapsule(
	t *testing.T,
	conversationID string,
	content string,
) (contextcapsule.RoleContextCapsule, []byte) {
	t.Helper()
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID:          conversationID,
			TeamID:                  "team-alpha",
			AgentID:                 "agent-alpha",
			RoleID:                  "role-alpha",
			ProviderID:              "deepseek",
			ProviderAccountID:       "deepseek.primary",
			ModelID:                 "deepseek-chat",
			AuthMode:                "api_key",
			ContextAdapterID:        "context:loom-native:v1",
			DisclosurePolicyID:      "loom.local-team-disclosure",
			DisclosurePolicyVersion: 1,
			TokenBudget:             128,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
			Trust:      contextcapsule.TrustAuthoritative,
			Scope:      contextcapsule.ScopeConversationShared,
			Priority:   contextcapsule.PrioritySystem,
			TokenCount: 4, Required: true, Content: []byte(content),
			SourceType: contextcapsule.SourceAuthority,
			SourceRef:  "goal:phase-2d",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	return capsule, payload
}
