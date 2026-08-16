package vault

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"loom-pi-rebuild/internal/toolproposal"
)

func TestToolProposalStoreEncryptsAndRestoresExactBinding(t *testing.T) {
	store, keyPath, databasePath := newConversationDocumentStoreFixture(t)
	record := testToolProposal("approval-1", "run-proposal-1", `{"command":"printf proposal-secret-marker","path":""}`)
	if err := store.PutToolProposal(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	database, err := os.ReadFile(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(database, record.Content) || bytes.Contains(database, []byte("proposal-secret-marker")) {
		t.Fatal("tool proposal persisted plaintext")
	}
	record.Close()
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
	expected := testToolProposal("approval-1", "run-proposal-1", `{"command":"printf proposal-secret-marker","path":""}`)
	defer expected.Close()
	stored, err := reopened.ReadToolProposal(context.Background(), expected.Binding)
	if err != nil || !bytes.Equal(stored.Content, expected.Content) || stored.Binding != expected.Binding {
		stored.Close()
		t.Fatalf("stored proposal = %#v, %v", stored, err)
	}
	stored.Close()
	lookedUp, err := reopened.LookupToolProposal(context.Background(), toolproposal.ApprovalLookup{
		ApprovalID: expected.Binding.ApprovalID, ApprovalDigest: expected.Binding.ApprovalDigest,
		WorkItemID: expected.Binding.WorkItemID, CallDigest: expected.Binding.CallDigest,
	})
	if err != nil || !bytes.Equal(lookedUp.Content, expected.Content) || lookedUp.Binding != expected.Binding {
		lookedUp.Close()
		t.Fatalf("approval lookup = %#v, %v", lookedUp, err)
	}
	lookedUp.Close()
}

func TestToolProposalStoreRejectsSubstitutionTamperAndConflict(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	record := testToolProposal("approval-2", "run-proposal-2", `{"command":"go test ./...","path":""}`)
	defer record.Close()
	if err := store.PutToolProposal(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := store.PutToolProposal(context.Background(), record); err != nil {
		t.Fatalf("idempotent put = %v", err)
	}
	conflict := testToolProposal("approval-2", "run-proposal-2", `{"command":"rm -rf /","path":""}`)
	defer conflict.Close()
	if err := store.PutToolProposal(context.Background(), conflict); !errors.Is(err, ErrToolProposalConflict) {
		t.Fatalf("proposal conflict = %v", err)
	}
	for name, mutate := range map[string]func(*toolproposal.Binding){
		"generation": func(binding *toolproposal.Binding) { binding.ClaimGeneration++ },
		"agent":      func(binding *toolproposal.Binding) { binding.AgentInstanceID = "agent-other" },
		"binding": func(binding *toolproposal.Binding) {
			binding.ExecutionBindingDigest = testDigest("binding-other")
		},
		"approval": func(binding *toolproposal.Binding) { binding.ApprovalDigest = testDigest("approval-other") },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := record.Binding
			mutate(&candidate)
			if _, err := store.ReadToolProposal(context.Background(), candidate); !errors.Is(err, ErrToolProposalBinding) {
				t.Fatalf("substitution read = %v", err)
			}
		})
	}
	if _, err := store.database.Exec(
		`UPDATE encrypted_tool_proposals SET call_digest = ? WHERE proposal_id = ?`,
		testDigest("tampered-call"), record.Binding.ProposalID,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadToolProposal(context.Background(), record.Binding); !errors.Is(err, ErrToolProposalAuthentication) {
		t.Fatalf("tampered proposal read = %v", err)
	}
}

func TestToolProposalStoreSurvivesRotationAndDeletesExactRecord(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	record := testToolProposal("approval-3", "run-proposal-3", `{"command":"git status","path":""}`)
	defer record.Close()
	if err := store.PutToolProposal(context.Background(), record); err != nil {
		t.Fatal(err)
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
	stored, err := store.ReadToolProposal(context.Background(), record.Binding)
	if err != nil || !bytes.Equal(stored.Content, record.Content) {
		stored.Close()
		t.Fatalf("rotated proposal = %#v, %v", stored, err)
	}
	stored.Close()
	if err := store.DeleteToolProposal(context.Background(), record.Binding); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadToolProposal(context.Background(), record.Binding); !errors.Is(err, ErrToolProposalNotFound) {
		t.Fatalf("deleted proposal read = %v", err)
	}
}

func TestToolProposalStoreConcurrentPutHasOneImmutableWinner(t *testing.T) {
	store, _, _ := newConversationDocumentStoreFixture(t)
	defer store.Close()
	records := []toolproposal.Record{
		testToolProposal("approval-race", "run-proposal-race", `{"command":"first","path":""}`),
		testToolProposal("approval-race", "run-proposal-race", `{"command":"second","path":""}`),
	}
	defer records[0].Close()
	defer records[1].Close()
	errorsByWrite := make([]error, len(records))
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range records {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			errorsByWrite[index] = store.PutToolProposal(context.Background(), records[index])
		}()
	}
	close(start)
	wait.Wait()
	succeeded, conflicted := 0, 0
	for _, err := range errorsByWrite {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrToolProposalConflict):
			conflicted++
		default:
			t.Fatalf("concurrent put error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent puts succeeded=%d conflicted=%d errors=%v", succeeded, conflicted, errorsByWrite)
	}
	var count int
	if err := store.database.QueryRow(
		`SELECT COUNT(*) FROM encrypted_tool_proposals WHERE proposal_id = ?`,
		"approval-race",
	).Scan(&count); err != nil || count != 1 {
		t.Fatalf("stored proposal count=%d, %v", count, err)
	}
}

func testToolProposal(approvalID, runID, content string) toolproposal.Record {
	body := []byte(content)
	return toolproposal.Record{
		Binding: toolproposal.Binding{
			ProposalID: approvalID, ConversationID: "conversation-tool-proposal",
			WorkItemID: "work-tool-proposal", RunID: runID, ClaimGeneration: 3,
			RuntimeInstanceID: "runtime-tool-proposal", AgentInstanceID: "agent-tool-proposal",
			ExecutionBindingDigest: testDigest("binding-" + runID),
			CapsuleDigest:          testDigest("capsule-" + runID), CallID: "call-" + runID,
			CallDigest: testDigest("call-" + runID), ApprovalID: approvalID,
			ApprovalDigest: testDigest("approval-" + approvalID), Tool: "Bash",
			OperationID: "operation-" + approvalID, IncidentID: "incident-" + approvalID,
			ContentDigest: toolproposal.ContentDigest(body),
		},
		Content: body,
	}
}
