package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
)

func TestCOMP2CVaultSlotClearsSensitiveInputsWhenUnavailable(t *testing.T) {
	for _, fixture := range []struct {
		name string
		slot *productVaultRouteSlot
	}{
		{name: "unbound", slot: &productVaultRouteSlot{}},
		{name: "nil"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			contextPayload := []byte("private context")
			if err := fixture.slot.PutRoleContextCapsule(
				context.Background(), contextcapsule.RoleContextCapsule{}, contextPayload,
			); err == nil {
				t.Fatal("unavailable Vault slot accepted a Context Capsule")
			}
			assertProductBytesCleared(t, "Context Capsule", contextPayload)

			documentPayload := []byte("private transcript")
			if err := fixture.slot.PutConversationDocument(context.Background(), api.LocalProductChatDocument{
				Payload: documentPayload,
			}); err == nil {
				t.Fatal("unavailable Vault slot accepted a Conversation document")
			}
			assertProductBytesCleared(t, "Conversation document", documentPayload)

			attemptContent := []byte("private tool result")
			if err := fixture.slot.PutAttemptPayload(context.Background(), attemptpayload.Payload{
				Content: attemptContent,
			}); err == nil {
				t.Fatal("unavailable Vault slot accepted an Attempt payload")
			}
			assertProductBytesCleared(t, "Attempt payload", attemptContent)

			agentInput := []byte("private Agent input")
			if err := fixture.slot.PutAgentInput(context.Background(), agentinbox.Payload{
				Content: agentInput,
			}); err == nil {
				t.Fatal("unavailable Vault slot accepted an Agent input")
			}
			assertProductBytesCleared(t, "Agent input", agentInput)
		})
	}
}

func assertProductBytesCleared(t *testing.T, name string, value []byte) {
	t.Helper()
	for index, current := range value {
		if current != 0 {
			t.Fatalf("%s byte %d was not cleared", name, index)
		}
	}
}

func TestCOMP2CVaultConstructsInsideBundleAndRevokesBoundedPorts(t *testing.T) {
	root, statePath := productDaemonFailureState(t)
	database, err := openProductReadDatabase(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	slot := &productVaultRouteSlot{}
	constructed := 0
	factory := newProductVaultRouteFactory(statePath, store, readModel)
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(
			context.Context, localipc.Request,
		) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"66666666-6666-4666-8666-666666666666",
		nil,
		productCompatibilityConstruction{
			vaultSlot: slot,
			vaultFactory: func(ctx context.Context) (productVaultRoutes, error) {
				constructed++
				return factory(ctx)
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if constructed != 1 || !slot.Ready() {
		t.Fatalf("constructed=%d ready=%t", constructed, slot.Ready())
	}
	status, err := slot.CredentialVaultStatus(context.Background())
	if err != nil || status.Status != "unlocked" || status.StorageMode != "local_key_file" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	if documents, err := slot.ConversationDocuments(
		context.Background(), "loom.chat-thread.v1",
	); err != nil || len(documents) != 0 {
		t.Fatalf("documents=%#v err=%v", documents, err)
	}
	attemptResult := productAttemptPayloadVaultPayload(
		"payload-vault-rollback", "private encrypted rollback payload",
	)
	frozenAttemptBinding := attemptResult.Binding
	defer attemptResult.Close()
	if err := slot.PutAttemptPayload(context.Background(), attemptResult); err != nil {
		t.Fatal(err)
	}
	if err := slot.DeleteAttemptPayload(context.Background(), frozenAttemptBinding); err != nil {
		t.Fatal(err)
	}
	if _, err := slot.ReadAttemptPayload(
		context.Background(), frozenAttemptBinding,
	); !errors.Is(err, attemptpayload.ErrPayloadNotFound) {
		t.Fatalf("deleted Attempt payload read=%v", err)
	}
	agentInput := productAgentInboxVaultPayload("agent-input-1", "private queued input")
	frozenAgentInput := agentInput.Binding
	if err := slot.PutAgentInput(context.Background(), agentInput); err != nil {
		t.Fatal(err)
	}
	storedAgentInput, err := slot.ReadAgentInput(context.Background(), frozenAgentInput)
	if err != nil || storedAgentInput.Status != agentinbox.StatusPending ||
		string(storedAgentInput.Content) != "private queued input" {
		storedAgentInput.Close()
		t.Fatalf("Agent input=%#v err=%v", storedAgentInput, err)
	}
	storedAgentInput.Close()
	if err := slot.MarkAgentInputConsumed(context.Background(), frozenAgentInput); err != nil {
		t.Fatal(err)
	}
	checkpoint := productAgentCheckpointVaultPayload("agent-checkpoint-1", "private checkpoint")
	frozenCheckpoint := checkpoint.Binding
	if err := slot.PutAgentCheckpoint(context.Background(), checkpoint); err != nil {
		t.Fatal(err)
	}
	storedCheckpoint, err := slot.ReadAgentCheckpoint(context.Background(), frozenCheckpoint)
	if err != nil || string(storedCheckpoint.Content) != "private checkpoint" {
		storedCheckpoint.Close()
		t.Fatalf("Agent checkpoint=%#v err=%v", storedCheckpoint, err)
	}
	storedCheckpoint.Close()
	for path, mode := range map[string]os.FileMode{
		filepath.Join(root, "credential-vault.db"):  0o600,
		filepath.Join(root, "private"):              0o700,
		filepath.Join(root, "private", "vault.key"): 0o600,
	} {
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode().Perm() != mode {
			t.Fatalf("path=%s mode=%v err=%v", path, info.Mode().Perm(), statErr)
		}
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("Vault slot remained ready after Composition close")
	}
	if _, err := slot.CredentialVaultStatus(context.Background()); err == nil {
		t.Fatal("closed Vault slot returned status")
	}
	if _, err := slot.ReadAgentInput(context.Background(), frozenAgentInput); err == nil {
		t.Fatal("closed Vault slot returned Agent input")
	}
	if _, err := slot.ReadAgentCheckpoint(context.Background(), frozenCheckpoint); err == nil {
		t.Fatal("closed Vault slot returned Agent checkpoint")
	}
}

func productAgentCheckpointVaultPayload(
	checkpointID, content string,
) agentcheckpoint.Payload {
	digest := sha256.Sum256([]byte(content))
	return agentcheckpoint.Payload{
		Binding: agentcheckpoint.Binding{
			CheckpointID: checkpointID, ConversationID: "mission:team-vault",
			SegmentID: "segment-vault", AttemptID: "attempt-vault",
			AgentInstanceID: "agent-vault", WorkItemID: "work-vault",
			RunID: "run-vault", ClaimGeneration: 1,
			RuntimeInstanceID:      "runtime-vault",
			ExecutionBindingDigest: productAgentInboxTestDigest("checkpoint-binding"),
			CapsuleDigest:          productAgentInboxTestDigest("checkpoint-capsule"),
			TurnID:                 "turn-vault", TurnSequence: 1,
			StepID: "step-vault", StepSequence: 1,
			ContentType:   agentcheckpoint.ContentTypeTextUTF8,
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Content: []byte(content),
	}
}

func productAttemptPayloadVaultPayload(payloadID, content string) attemptpayload.Payload {
	digest := sha256.Sum256([]byte(content))
	return attemptpayload.Payload{
		Binding: attemptpayload.Binding{
			PayloadID: payloadID,
			Scope: attemptpayload.Scope{
				ConversationID: "mission:team-vault", WorkItemID: "work-vault",
				RunID: "run-vault", ClaimGeneration: 1,
				RuntimeInstanceID:      "runtime-vault",
				ExecutionBindingDigest: productAgentInboxTestDigest("attempt-binding"),
				CapsuleDigest:          productAgentInboxTestDigest("attempt-capsule"),
			},
			CallID: "call-vault", Sequence: 1,
			ContentType:   attemptpayload.ContentTypeTextUTF8,
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status:  attemptpayload.StatusPending,
		Content: []byte(content),
	}
}

func productAgentInboxVaultPayload(inputID, content string) agentinbox.Payload {
	digest := sha256.Sum256([]byte(content))
	return agentinbox.Payload{
		Binding: agentinbox.Binding{
			PayloadID: "payload-" + inputID, InputID: inputID,
			Mode: agentinbox.ModeQueue, ContextScope: agentinbox.ScopeConversationShared,
			ConversationID: "mission:team-vault", SegmentID: "segment-vault",
			AgentInstanceID: "agent-vault", WorkItemID: "work-vault",
			RunID: "run-vault", ClaimGeneration: 1,
			RuntimeInstanceID:      "runtime-vault",
			ExecutionBindingDigest: productAgentInboxTestDigest("binding"),
			CapsuleDigest:          productAgentInboxTestDigest("capsule"),
			OrderKey:               1, TargetTurnID: "turn-2", TargetTurnSequence: 2,
			ContentType: "text/plain", ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: agentinbox.StatusPending, Content: []byte(content),
	}
}

func productAgentInboxTestDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func TestCOMP2CLegacyCredentialLeaseConstructsInsideVaultBundleAndRevokes(t *testing.T) {
	slot := &productCredentialLeaseRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2c-legacy-credential", nil,
		productCompatibilityConstruction{
			legacyCredentialSlot: slot,
			legacyCredentialFactory: newProductLegacyCredentialLeaseFactory(
				&productCredentialTestStore{},
			),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slot.Ready() {
		t.Fatal("loom-vault did not publish the explicit legacy lease port")
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("closed loom-vault retained the explicit legacy lease port")
	}
}

func TestCOMP2CVaultStartsBeforeConversationAndClosesAfterIt(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := openProductReadDatabase(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	events := make([]string, 0, 2)
	vaultSlot := &productVaultRouteSlot{}
	conversationSlot := &productConversationRouteSlot{}
	vaultFactory := newProductVaultRouteFactory(statePath, store, readModel)
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(
			context.Context, localipc.Request,
		) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"77777777-7777-4777-8777-777777777777",
		nil,
		productCompatibilityConstruction{
			vaultSlot: vaultSlot,
			vaultFactory: func(ctx context.Context) (productVaultRoutes, error) {
				events = append(events, "vault")
				return vaultFactory(ctx)
			},
			conversationSlot: conversationSlot,
			conversationFactory: func(
				context.Context,
			) (productConversationRoutes, error) {
				if !vaultSlot.Ready() {
					return productConversationRoutes{}, errors.New("Vault was not ready")
				}
				events = append(events, "conversation")
				return productConversationRoutes{
					route: productConversationRouteFixture{},
					close: func() error { return nil },
				}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0] != "vault" || events[1] != "conversation" {
		t.Fatalf("events=%v", events)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if vaultSlot.Ready() || conversationSlot.Ready() {
		t.Fatalf("vaultReady=%t conversationReady=%t", vaultSlot.Ready(), conversationSlot.Ready())
	}
}

func TestCOMP2CProductionDoesNotConstructVaultOutsideBundle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundSlot := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok {
				name := ""
				switch target := call.Fun.(type) {
				case *ast.Ident:
					name = target.Name
				case *ast.SelectorExpr:
					name = target.Sel.Name
				}
				if name == "newProductCredentialVaultRuntime" ||
					name == "newProductCredentialVaultRecoveryRuntime" {
					t.Errorf("production constructs %s outside loom-vault Bundle", name)
				}
			}
			field, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := field.Key.(*ast.Ident)
			if ok && key.Name == "vaultSlot" && identifierNamed(field.Value, "vaultRouteSlot") {
				foundSlot = true
			}
			return true
		})
	}
	if !foundSlot {
		t.Fatal("production Composition does not bind the Vault route slot")
	}
}
