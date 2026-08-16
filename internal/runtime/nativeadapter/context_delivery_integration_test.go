package nativeadapter

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"

	_ "modernc.org/sqlite"
)

type integrationRetrievalAuditor struct {
	records []contextcapsule.RetrievalAudit
}

func (auditor *integrationRetrievalAuditor) RecordContextRetrieval(
	_ context.Context,
	record contextcapsule.RetrievalAudit,
) error {
	auditor.records = append(auditor.records, record)
	return nil
}

func TestLoomNativeContextDeliveryRecoversFromVaultAndJournalAcrossRestart(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	capsule := integrationRetrievalCapsule(t, request)
	if capsule.AuthorityRecord() != request.ContextCapsule {
		t.Fatalf("capsule authority = %#v, request = %#v", capsule.AuthorityRecord(), request.ContextCapsule)
	}
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}

	vaultStore, keyPath, vaultPath := openIntegrationVault(t)
	if err := vaultStore.PutRoleContextCapsule(ctx, capsule, dispatchPayload); err != nil {
		t.Fatal(err)
	}
	journalStore, journalPath := openIntegrationJournal(t)
	seedIntegrationRuntime(t, journalStore, request, now)
	workAuthority, err := work.NewAuthority(
		journalStore,
		func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x41}, 128)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	workItem, run, err := workAuthority.CreateAndAssign(ctx, work.WorkItemAssignmentInput{
		WorkItemID: request.Binding.WorkItemID, Title: "restart-safe context delivery",
		RunID: request.Binding.RunID, AgentInstanceID: request.Binding.SenderAgentInstanceID,
		ExecutionBinding: request.ExecutionBinding,
		CorrelationID:    request.Dispatch.CorrelationID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	workItem, run, err = workAuthority.Claim(ctx, work.RunClaimInput{
		WorkItemID: workItem.ID(), RunID: run.ID(),
		RuntimeInstanceID:    request.Binding.RuntimeInstanceID,
		AgentInstanceID:      request.Binding.SenderAgentInstanceID,
		PrepareLeaseDuration: 5 * time.Minute,
		CorrelationID:        request.Dispatch.CorrelationID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = workAuthority.Start(ctx, work.RunGenerationInput{
		WorkItemID: workItem.ID(), RunID: run.ID(), ClaimID: run.ClaimID(),
		ClaimGeneration: run.ClaimGeneration(), RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID: run.AgentInstanceID(), CorrelationID: request.Dispatch.CorrelationID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := work.NewAttemptPayloadAuthority(workAuthority)
	if err != nil {
		t.Fatal(err)
	}
	attempt := contextcapsule.AttemptIdentity{
		WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
		RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
		ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
		IncidentID:             request.Dispatch.CorrelationID(),
	}
	scope := attemptpayload.Scope{
		ConversationID: capsule.AuthorityRecord().ConversationID,
		WorkItemID:     attempt.WorkItemID, RunID: attempt.RunID,
		ClaimGeneration: attempt.ClaimGeneration, RuntimeInstanceID: attempt.RuntimeInstanceID,
		ExecutionBindingDigest: attempt.ExecutionBindingDigest,
		CapsuleDigest:          capsule.Digest(),
	}
	auditor := &integrationRetrievalAuditor{}
	delivery := newIntegrationDeliveryCoordinator(
		t, capsule, attempt, vaultStore, auditor, facts,
	)
	request.ContextRetriever = delivery.retriever
	request.ContextDelivery = delivery.coordinator

	toolResponse := integrationContextToolResponse(omitted, "provider-call-before-restart")
	firstDoer := &contextRetrievalHTTPDoerFixture{
		responses: []*http.Response{
			toolResponse,
			{StatusCode: http.StatusOK, Body: io.NopCloser(agentTimeoutBodyFixture{})},
		},
		beforeDo: func(round int) error {
			if round != 2 {
				return nil
			}
			pending, pendingErr := vaultStore.ListPendingAttemptPayloads(ctx, scope)
			defer closeAttemptPayloads(pending)
			if pendingErr != nil || len(pending) != 1 {
				return fmt.Errorf("payload not persisted before transport: %d, %w", len(pending), pendingErr)
			}
			return nil
		},
	}
	firstAdapter := integrationDeepSeekAdapter(t, firstDoer, now)
	if _, err := firstAdapter.Execute(ctx, request); err != nil {
		t.Fatal(err)
	}
	pending, err := vaultStore.ListPendingAttemptPayloads(ctx, scope)
	if err != nil || len(pending) != 1 || len(auditor.records) != 1 {
		closeAttemptPayloads(pending)
		t.Fatalf("after timeout pending=%#v audits=%#v error=%v", pending, auditor.records, err)
	}
	closeAttemptPayloads(pending)
	if err := vaultStore.Close(); err != nil {
		t.Fatal(err)
	}

	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: vaultPath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	delivery = newIntegrationDeliveryCoordinator(
		t, capsule, attempt, reopened, auditor, facts,
	)
	request.ContextRetriever = delivery.retriever
	request.ContextDelivery = delivery.coordinator
	request.FrameSink = &frameSinkFixture{}
	secondDoer := &contextRetrievalHTTPDoerFixture{
		responses: []*http.Response{
			integrationContextToolResponse(omitted, "provider-call-after-restart"),
			{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"resumed from persisted context"}}]}`,
				)),
			},
		},
		beforeDo: func(round int) error {
			if round != 2 {
				return nil
			}
			pending, pendingErr := reopened.ListPendingAttemptPayloads(ctx, scope)
			defer closeAttemptPayloads(pending)
			if pendingErr != nil || len(pending) != 1 {
				return fmt.Errorf("recovered payload missing before transport: %d, %w", len(pending), pendingErr)
			}
			return nil
		},
	}
	secondAdapter := integrationDeepSeekAdapter(t, secondDoer, now.Add(time.Second))
	result, err := secondAdapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode() != 0 || len(auditor.records) != 1 {
		t.Fatalf("resumed result=%#v audits=%#v", result, auditor.records)
	}
	if len(secondDoer.bodies) != 2 ||
		!bytes.Contains(secondDoer.bodies[1], []byte(`"tool_call_id":"provider-call-after-restart"`)) ||
		!bytes.Contains(secondDoer.bodies[1], []byte("scoped omitted context must only enter the second Provider request")) {
		t.Fatalf("resumed Provider bodies = %#v", secondDoer.bodies)
	}
	if pending, err := reopened.ListPendingAttemptPayloads(ctx, scope); err != nil || len(pending) != 0 {
		closeAttemptPayloads(pending)
		t.Fatalf("delivered payload remained pending: %#v, %v", pending, err)
	}

	events, err := journalStore.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	binding := integrationAttemptPayloadBinding(t, events, scope)
	stored, err := reopened.ReadAttemptPayload(ctx, binding)
	if err != nil || stored.Status != attemptpayload.StatusDelivered ||
		!bytes.Contains(stored.Content, []byte("scoped omitted context must only enter the second Provider request")) {
		stored.Close()
		t.Fatalf("delivered payload = %#v, %v", stored, err)
	}
	stored.Close()
	assertIntegrationFilesExclude(t, []string{
		vaultPath, vaultPath + "-wal", journalPath, journalPath + "-wal",
	}, "scoped omitted context must only enter the second Provider request")
}

func TestAttemptPayloadTerminalReconciliationRepairsRealVaultFromJournal(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 14, 0, 0, 0, time.UTC)
	fixture := newIntegrationAttemptAuthorityFixture(t, now)
	auditor := &integrationRetrievalAuditor{}
	delivery := newIntegrationDeliveryCoordinator(
		t, fixture.capsule, fixture.attempt, fixture.vault, auditor, fixture.facts,
	)
	payload, err := delivery.coordinator.Prepare(
		ctx,
		contextcapsule.RetrievalProposal{
			ItemID: fixture.omitted.ItemID, ContentDigest: fixture.omitted.ContentDigest,
			ArtifactRef: fixture.omitted.ArtifactRef,
		},
		contextcapsule.DeliveryRequest{Sequence: 1, ContentType: "application/json"},
		marshalContextToolResult,
	)
	if err != nil {
		t.Fatal(err)
	}
	binding := payload.Binding
	payload.Close()
	if err := fixture.facts.Deliver(
		ctx, fixture.authority, binding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.runs.CommitTerminal(ctx, work.RunTerminalInput{
		RunGenerationInput: work.RunGenerationInput{
			WorkItemID: fixture.run.WorkItemID(), RunID: fixture.run.ID(),
			ClaimID: fixture.run.ClaimID(), ClaimGeneration: fixture.run.ClaimGeneration(),
			RuntimeInstanceID: fixture.run.RuntimeInstanceID(),
			AgentInstanceID:   fixture.run.AgentInstanceID(),
			CorrelationID:     fixture.request.Dispatch.CorrelationID(),
		},
		Status: "cancelled", Reason: "operator_cancelled",
	}); err != nil {
		t.Fatal(err)
	}
	pending, err := fixture.vault.ListPendingAttemptPayloads(ctx, binding.Scope)
	if err != nil || len(pending) != 1 {
		closeAttemptPayloads(pending)
		t.Fatalf("crash-window pending=%#v err=%v", pending, err)
	}
	closeAttemptPayloads(pending)
	if err := fixture.vault.Close(); err != nil {
		t.Fatal(err)
	}

	material, err := (credentialvault.LocalKeyFile{Path: fixture.keyPath}).LoadOrCreate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: fixture.vaultPath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	report, err := fixture.facts.ReconcileDelivered(ctx, reopened)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Outcomes) != 1 ||
		report.Outcomes[0].Result != work.AttemptPayloadReconcileRepaired ||
		report.Outcomes[0].RunPhase != "terminal" ||
		report.Outcomes[0].Binding != binding {
		t.Fatalf("terminal reconciliation = %#v", report)
	}
	stored, err := reopened.ReadAttemptPayload(ctx, binding)
	if err != nil || stored.Status != attemptpayload.StatusDelivered ||
		!bytes.Contains(stored.Content, []byte(
			"scoped omitted context must only enter the second Provider request",
		)) {
		stored.Close()
		t.Fatalf("reconciled payload = %#v err=%v", stored, err)
	}
	stored.Close()
	assertIntegrationFilesExclude(t, []string{
		fixture.vaultPath, fixture.vaultPath + "-wal",
		fixture.journalPath, fixture.journalPath + "-wal",
	}, "scoped omitted context must only enter the second Provider request")
}

type integrationAttemptAuthorityFixture struct {
	request     supervisor.AdapterRequest
	omitted     contextcapsule.OmittedItem
	capsule     contextcapsule.RoleContextCapsule
	vault       *credentialvault.VaultStore
	keyPath     string
	vaultPath   string
	journalPath string
	runs        *work.Authority
	run         work.RunRecord
	facts       *work.AttemptPayloadAuthority
	attempt     contextcapsule.AttemptIdentity
	authority   attemptpayload.Authority
}

func newIntegrationAttemptAuthorityFixture(
	t testing.TB,
	now time.Time,
) integrationAttemptAuthorityFixture {
	t.Helper()
	ctx := context.Background()
	request, omitted := deepSeekRetrievalAdapterRequest(t)
	capsule := integrationRetrievalCapsule(t, request)
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	vaultStore, keyPath, vaultPath := openIntegrationVault(t)
	if err := vaultStore.PutRoleContextCapsule(ctx, capsule, dispatchPayload); err != nil {
		t.Fatal(err)
	}
	journalStore, journalPath := openIntegrationJournal(t)
	seedIntegrationRuntime(t, journalStore, request, now)
	runs, err := work.NewAuthority(
		journalStore, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x51}, 128)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runs.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	workItem, run, err := runs.CreateAndAssign(ctx, work.WorkItemAssignmentInput{
		WorkItemID: request.Binding.WorkItemID, Title: "terminal payload reconciliation",
		RunID: request.Binding.RunID, AgentInstanceID: request.Binding.SenderAgentInstanceID,
		ExecutionBinding: request.ExecutionBinding,
		CorrelationID:    request.Dispatch.CorrelationID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	workItem, run, err = runs.Claim(ctx, work.RunClaimInput{
		WorkItemID: workItem.ID(), RunID: run.ID(),
		RuntimeInstanceID:    request.Binding.RuntimeInstanceID,
		AgentInstanceID:      request.Binding.SenderAgentInstanceID,
		PrepareLeaseDuration: 5 * time.Minute,
		CorrelationID:        request.Dispatch.CorrelationID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, work.RunGenerationInput{
		WorkItemID: workItem.ID(), RunID: run.ID(), ClaimID: run.ClaimID(),
		ClaimGeneration: run.ClaimGeneration(), RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID: run.AgentInstanceID(), CorrelationID: request.Dispatch.CorrelationID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	attempt := contextcapsule.AttemptIdentity{
		WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
		RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
		ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
		IncidentID:             request.Dispatch.CorrelationID(),
	}
	return integrationAttemptAuthorityFixture{
		request: request, omitted: omitted, capsule: capsule,
		vault: vaultStore, keyPath: keyPath, vaultPath: vaultPath,
		journalPath: journalPath, runs: runs, run: run, facts: facts,
		attempt: attempt,
		authority: attemptpayload.Authority{
			Scope: attemptpayload.Scope{
				ConversationID: capsule.AuthorityRecord().ConversationID,
				WorkItemID:     attempt.WorkItemID, RunID: attempt.RunID,
				ClaimGeneration:        attempt.ClaimGeneration,
				RuntimeInstanceID:      attempt.RuntimeInstanceID,
				ExecutionBindingDigest: attempt.ExecutionBindingDigest,
				CapsuleDigest:          capsule.Digest(),
			},
			ClaimID:         attempt.ClaimID,
			AgentInstanceID: capsule.AuthorityRecord().AgentID,
			IncidentID:      attempt.IncidentID,
		},
	}
}

type integrationDelivery struct {
	retriever   *contextcapsule.ScopedRetriever
	coordinator *contextcapsule.DeliveryCoordinator
}

func newIntegrationDeliveryCoordinator(
	t testing.TB,
	capsule contextcapsule.RoleContextCapsule,
	attempt contextcapsule.AttemptIdentity,
	store *credentialvault.VaultStore,
	auditor *integrationRetrievalAuditor,
	facts *work.AttemptPayloadAuthority,
) integrationDelivery {
	t.Helper()
	retriever, err := contextcapsule.NewScopedRetriever(
		capsule.AuthorityRecord(), attempt, store, auditor,
	)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := contextcapsule.NewDeliveryCoordinator(
		capsule.AuthorityRecord(), attempt, retriever, store, facts,
	)
	if err != nil {
		t.Fatal(err)
	}
	return integrationDelivery{retriever: retriever, coordinator: coordinator}
}

func integrationRetrievalCapsule(
	t testing.TB,
	request supervisor.AdapterRequest,
) contextcapsule.RoleContextCapsule {
	t.Helper()
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-retrieval-1", TeamID: "team-retrieval-1",
			AgentID: request.Binding.SenderAgentInstanceID, RoleID: "researcher",
			ProviderID:        request.ExecutionBinding.ProviderID,
			ProviderAccountID: request.ExecutionBinding.ProviderAccountID,
			ModelID:           request.ExecutionBinding.ModelID, AuthMode: "brokered",
			ContextAdapterID:   "context:loom-native:v1",
			DisclosurePolicyID: "policy.test", DisclosurePolicyVersion: 1,
			ArtifactRefs: []string{"artifact:diff-1"}, TokenBudget: 4,
		},
		[]contextcapsule.ItemInput{
			{
				ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
				Trust:    contextcapsule.TrustAuthoritative,
				Scope:    contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
				Content:    []byte("Review the exact bounded diff."),
				SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:retrieval-1",
			},
			{
				ItemID: "diff-detail", Kind: contextcapsule.KindArtifactReference,
				Trust:    contextcapsule.TrustObserved,
				Scope:    contextcapsule.ScopeArtifactScoped,
				Priority: contextcapsule.PriorityRetrievable, TokenCount: 5,
				Content:    []byte("scoped omitted context must only enter the second Provider request"),
				SourceType: contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
				ArtifactRef: "artifact:diff-1",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return capsule
}

func openIntegrationVault(
	t testing.TB,
) (*credentialvault.VaultStore, string, string) {
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
	material, err := (credentialvault.LocalKeyFile{Path: keyPath}).LoadOrCreate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store, err := credentialvault.OpenStore(credentialvault.StoreConfig{
		DatabasePath: databasePath, KeyMaterial: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, keyPath, databasePath
}

func openIntegrationJournal(t testing.TB) (*journal.Store, string) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "journal.db")
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	database, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", databasePath, values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	return journal.NewStore(database), databasePath
}

func seedIntegrationRuntime(
	t testing.TB,
	store *journal.Store,
	request supervisor.AdapterRequest,
	now time.Time,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe-native-integration",
		"instance": map[string]any{
			"id": request.Binding.RuntimeInstanceID, "device_id": "device-local",
			"adapter_type": "loom-native", "display_name": "Loom Native",
			"executable_version": "1.0.0", "status": "online",
			"observed_capabilities": []string{"context_retrieval"}, "capacity": 1,
		},
		"model_ids": []string{"deepseek-chat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:       "runtime-discovered-native-integration",
		StreamID: "runtime_instance:" + request.Binding.RuntimeInstanceID,
		Seq:      1, IdempotencyKey: "runtime-discovered-native-integration",
		Type: "RuntimeInstanceDiscovered", SchemaVersion: 1,
		EmittedAt: now.Add(-time.Minute), CorrelationID: request.Dispatch.CorrelationID(),
		PayloadJSON: payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func integrationContextToolResponse(
	omitted contextcapsule.OmittedItem,
	providerCallID string,
) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"` + providerCallID + `","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"` + omitted.ItemID + `\",\"content_digest\":\"` + omitted.ContentDigest + `\",\"artifact_ref\":\"` + omitted.ArtifactRef + `\"}"}}]}}]}`,
		)),
	}
}

func integrationDeepSeekAdapter(
	t testing.TB,
	client *contextRetrievalHTTPDoerFixture,
	now time.Time,
) supervisor.RuntimeAdapter {
	t.Helper()
	adapter, err := NewDeepSeekAgentAdapter(DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: "loom-native-local",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-deepseek-key")},
		Diagnostics:       &agentDiagnosticRecorderFixture{}, Client: client,
		Now: func() time.Time { return now }, MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func integrationAttemptPayloadBinding(
	t testing.TB,
	events []journal.Event,
	scope attemptpayload.Scope,
) attemptpayload.Binding {
	t.Helper()
	var binding attemptpayload.Binding
	accepted := 0
	delivered := 0
	for _, event := range events {
		if event.Type != "ToolResultAccepted" && event.Type != "ToolResultDelivered" {
			continue
		}
		var payload struct {
			PayloadID              string `json:"payload_id"`
			ConversationID         string `json:"conversation_id"`
			WorkItemID             string `json:"work_item_id"`
			RunID                  string `json:"run_id"`
			ClaimGeneration        int64  `json:"claim_generation"`
			RuntimeInstanceID      string `json:"runtime_instance_id"`
			ExecutionBindingDigest string `json:"execution_binding_digest"`
			CapsuleDigest          string `json:"capsule_digest"`
			CallID                 string `json:"call_id"`
			Sequence               int64  `json:"sequence"`
			ContentType            string `json:"content_type"`
			ContentDigest          string `json:"content_digest"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		candidate := attemptpayload.Binding{
			PayloadID: payload.PayloadID,
			Scope: attemptpayload.Scope{
				ConversationID: payload.ConversationID, WorkItemID: payload.WorkItemID,
				RunID: payload.RunID, ClaimGeneration: payload.ClaimGeneration,
				RuntimeInstanceID:      payload.RuntimeInstanceID,
				ExecutionBindingDigest: payload.ExecutionBindingDigest,
				CapsuleDigest:          payload.CapsuleDigest,
			},
			CallID: payload.CallID, Sequence: payload.Sequence,
			ContentType: payload.ContentType, ContentDigest: payload.ContentDigest,
		}
		if candidate.Scope != scope || binding != (attemptpayload.Binding{}) && binding != candidate {
			t.Fatalf("Attempt payload fact drift: %#v", candidate)
		}
		binding = candidate
		if event.Type == "ToolResultAccepted" {
			accepted++
		} else {
			delivered++
		}
	}
	if accepted != 1 || delivered != 1 || binding == (attemptpayload.Binding{}) {
		t.Fatalf("Attempt payload facts accepted=%d delivered=%d binding=%#v", accepted, delivered, binding)
	}
	return binding
}

func closeAttemptPayloads(payloads []attemptpayload.Payload) {
	for index := range payloads {
		payloads[index].Close()
	}
}

func assertIntegrationFilesExclude(t testing.TB, paths []string, forbidden string) {
	t.Helper()
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatalf("plaintext Attempt payload leaked to %s", path)
		}
	}
}
