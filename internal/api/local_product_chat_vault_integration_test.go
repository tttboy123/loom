package api

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
)

type localProductChatVaultStore struct {
	*credentialvault.VaultStore
}

var _ LocalProductChatDocumentStore = (*localProductChatVaultStore)(nil)
var _ LocalProductConversationContextCapsuleStore = (*localProductChatVaultStore)(nil)

func (store *localProductChatVaultStore) ConversationDocuments(
	ctx context.Context,
	kind string,
) ([]LocalProductChatDocument, error) {
	documents, err := store.VaultStore.ConversationDocuments(ctx, kind)
	if err != nil {
		return nil, err
	}
	converted := make([]LocalProductChatDocument, 0, len(documents))
	for index := range documents {
		document := documents[index]
		converted = append(converted, LocalProductChatDocument{
			ConversationID: document.ConversationID,
			Kind:           document.Kind,
			Revision:       document.Revision,
			Payload:        append([]byte(nil), document.Payload...),
		})
		clearLocalProductChatVaultBytes(document.Payload)
	}
	return converted, nil
}

func (store *localProductChatVaultStore) PutConversationDocument(
	ctx context.Context,
	document LocalProductChatDocument,
) error {
	return store.VaultStore.PutConversationDocument(
		ctx,
		credentialvault.ConversationDocument{
			ConversationID: document.ConversationID,
			Kind:           document.Kind,
			Revision:       document.Revision,
			Payload:        document.Payload,
		},
	)
}

func TestLocalProductChatVaultRestartPreservesEncryptedThreadAndRetrievableCapsule(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	privateDirectory := filepath.Join(root, "private")
	stateDirectory := filepath.Join(root, "state")
	for _, directory := range []string{privateDirectory, stateDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(privateDirectory, "vault.key")
	databasePath := filepath.Join(stateDirectory, "credential-vault.db")
	legacyChatPath := filepath.Join(stateDirectory, "chat-threads.json")

	openVault := func() *localProductChatVaultStore {
		t.Helper()
		material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(ctx)
		if err != nil {
			t.Fatal(err)
		}
		store, err := credentialvault.OpenStore(credentialvault.StoreConfig{
			DatabasePath: databasePath,
			KeyMaterial:  material,
			Now:          func() time.Time { return time.Unix(1_787_342_400, 0).UTC() },
		})
		if err != nil {
			t.Fatal(err)
		}
		return &localProductChatVaultStore{VaultStore: store}
	}

	const (
		threadID           = "phase2d-local-vault-thread"
		profileID          = "conversation-local-native-v1"
		transcriptSentinel = "PHASE2D_TRANSCRIPT_SENTINEL_private_route_history_must_stay_encrypted"
		capsuleSentinel    = "PHASE2D_CAPSULE_SENTINEL_budgeted_artifact_body_must_stay_encrypted"
		artifactRef        = "artifact:phase2d-budget-report"
	)
	targetForRoute := func(segmentID string) contextcapsule.Target {
		modelID := "local-model-v1"
		tokenBudget := 256
		if segmentID == "segment-2" {
			modelID = "local-model-v2"
			tokenBudget = 8
		}
		return contextcapsule.Target{
			ConversationID:          threadID,
			TeamID:                  "conversation:" + threadID,
			AgentID:                 "conversation-agent:reviewer",
			RoleID:                  "reviewer",
			ProviderID:              "local-native",
			ModelID:                 modelID,
			AuthMode:                "native_auth",
			ContextAdapterID:        "context:loom-native:v1",
			DisclosurePolicyID:      "loom.local-conversation-disclosure",
			DisclosurePolicyVersion: 1,
			TokenBudget:             tokenBudget,
		}
	}
	targetResolver := localProductConversationContextTargetResolverFunc(func(
		_ context.Context,
		conversationID string,
		segmentID string,
		resolvedProfileID string,
	) (contextcapsule.Target, error) {
		if conversationID != threadID || resolvedProfileID != profileID {
			return contextcapsule.Target{}, ErrInvalidLocalProductChatRequest
		}
		return targetForRoute(segmentID), nil
	})
	responder := localProductConversationResponderFunc(func(
		context.Context,
		LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		return LocalProductConversationResponse{
			Content:   "local synthetic acknowledgement",
			Tentative: true,
		}, nil
	})

	firstVault := openVault()
	chat, err := NewEncryptedPersistentLocalProductChatAPI(
		ctx, legacyChatPath, firstVault, time.Now, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SetConversationContextCapsuleRuntime(targetResolver, firstVault); err != nil {
		t.Fatal(err)
	}
	if _, err := chat.SendMessage(ctx, LocalProductChatMessageRequest{
		ThreadID:    threadID,
		Content:     transcriptSentinel,
		ProfileID:   profileID,
		ModelID:     "local-model-v1",
		ContextMode: ContextModeStartClean,
	}); err != nil {
		t.Fatal(err)
	}
	thread, err := chat.SendMessage(ctx, LocalProductChatMessageRequest{
		ThreadID:    threadID,
		Content:     "inspect artifact",
		ProfileID:   profileID,
		ModelID:     "local-model-v2",
		ContextMode: ContextModeContinueWithContext,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Segments) != 2 || thread.Segments[1].OmittedContextCount == 0 {
		t.Fatalf("route did not freeze budget omission: %#v", thread.Segments)
	}

	artifactTarget := targetForRoute("segment-2")
	artifactTarget.ArtifactRefs = []string{artifactRef}
	artifactTarget.TokenBudget = 4
	artifactCapsule, err := contextcapsule.BuildRoleContextCapsule(
		artifactTarget,
		[]contextcapsule.ItemInput{
			{
				ItemID: "current-task", Kind: contextcapsule.KindCurrentTaskState,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PriorityConfirmed, TokenCount: 4, Required: true,
				Content:    []byte("inspect encrypted artifact"),
				SourceType: contextcapsule.SourceAuthority, SourceRef: "conversation-route:segment-2",
			},
			{
				ItemID: "budget-report", Kind: contextcapsule.KindArtifactReference,
				Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
				Priority: contextcapsule.PriorityRetrievable, TokenCount: 16,
				Content:    []byte(capsuleSentinel),
				SourceType: contextcapsule.SourceObservation, SourceRef: "evidence:phase2d-budget-report",
				ArtifactRef: artifactRef,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	omitted := artifactCapsule.Omitted()
	if len(omitted) != 1 || omitted[0].ItemID != "budget-report" ||
		omitted[0].Reason != contextcapsule.OmissionBudgetExceeded {
		t.Fatalf("structured capsule omission = %#v", omitted)
	}
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(artifactCapsule)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstVault.PutRoleContextCapsule(ctx, artifactCapsule, dispatchPayload); err != nil {
		t.Fatal(err)
	}
	clearLocalProductChatVaultBytes(dispatchPayload)
	if err := firstVault.Close(); err != nil {
		t.Fatal(err)
	}

	vaultFiles, err := filepath.Glob(databasePath + "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(vaultFiles) == 0 {
		t.Fatal("Vault database files are unavailable after close")
	}
	for _, vaultFile := range vaultFiles {
		databaseBytes, err := os.ReadFile(vaultFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, sentinel := range []string{transcriptSentinel, capsuleSentinel} {
			if bytes.Contains(databaseBytes, []byte(sentinel)) {
				t.Fatalf("Vault file %s contains plaintext sentinel %q", filepath.Base(vaultFile), sentinel)
			}
		}
	}

	reopenedVault := openVault()
	t.Cleanup(func() { _ = reopenedVault.Close() })
	restartedChat, err := NewEncryptedPersistentLocalProductChatAPI(
		ctx, legacyChatPath, reopenedVault, time.Now, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := restartedChat.SetConversationContextCapsuleRuntime(
		targetResolver, reopenedVault,
	); err != nil {
		t.Fatal(err)
	}
	restartedThread, err := restartedChat.ChatThread(ctx, threadID)
	if err != nil || len(restartedThread.Messages) != 4 ||
		restartedThread.Messages[0].Content != transcriptSentinel ||
		len(restartedThread.Segments) != 2 ||
		restartedThread.Segments[1].ContextCapsuleDigest == "" {
		t.Fatalf("restarted thread = %#v, %v", restartedThread, err)
	}

	routeCapsule, err := reopenedVault.ReadContextCapsule(
		ctx,
		threadID,
		restartedThread.Segments[1].ContextCapsuleDigest,
	)
	if err != nil || !routeCapsule.CapsuleAvailable ||
		routeCapsule.Capsule.AuthorityRecord().RoleID != artifactTarget.RoleID ||
		len(routeCapsule.Capsule.Omitted()) == 0 ||
		routeCapsule.Capsule.Omitted()[0].Reason != contextcapsule.OmissionBudgetExceeded {
		t.Fatalf("restarted route capsule = %#v, %v", routeCapsule, err)
	}
	clearLocalProductChatVaultBytes(routeCapsule.DispatchPayload)

	authority := artifactCapsule.AuthorityRecord()
	storedArtifactCapsule, err := reopenedVault.ReadContextCapsule(
		ctx, authority.ConversationID, authority.CapsuleDigest,
	)
	if err != nil || !storedArtifactCapsule.CapsuleAvailable ||
		storedArtifactCapsule.Capsule.AuthorityRecord() != authority ||
		len(storedArtifactCapsule.Capsule.Omitted()) != 1 ||
		storedArtifactCapsule.Capsule.Omitted()[0].Reason != contextcapsule.OmissionBudgetExceeded {
		t.Fatalf("restarted structured capsule = %#v, %v", storedArtifactCapsule, err)
	}
	clearLocalProductChatVaultBytes(storedArtifactCapsule.DispatchPayload)

	retrieval := contextcapsule.RetrievalRequest{
		Authority:        authority,
		ItemID:           omitted[0].ItemID,
		ContentDigest:    omitted[0].ContentDigest,
		RequesterAgentID: artifactTarget.AgentID,
		RequesterRoleID:  artifactTarget.RoleID,
		ArtifactRef:      artifactRef,
	}
	retrieved, err := reopenedVault.RetrieveContextItem(ctx, retrieval)
	if err != nil || string(retrieved.Content) != capsuleSentinel {
		t.Fatalf("exact authority retrieval = %#v, %v", retrieved, err)
	}
	retrieved.Close()

	for name, forge := range map[string]func(*contextcapsule.RetrievalRequest){
		"role": func(request *contextcapsule.RetrievalRequest) {
			request.RequesterRoleID = "forged-reviewer"
		},
		"agent": func(request *contextcapsule.RetrievalRequest) {
			request.RequesterAgentID = "forged-agent"
		},
	} {
		t.Run("rejects forged "+name, func(t *testing.T) {
			forged := retrieval
			forge(&forged)
			if _, err := reopenedVault.RetrieveContextItem(ctx, forged); !errors.Is(err, contextcapsule.ErrContextRetrievalDenied) {
				t.Fatalf("forged %s retrieval error = %v", name, err)
			}
		})
	}
}

func clearLocalProductChatVaultBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
