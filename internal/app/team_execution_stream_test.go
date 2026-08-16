package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

var _ app.NodeOutputObserver = (*api.TeamExecutionStream)(nil)

type streamTestClock struct {
	now time.Time
}

func (clock *streamTestClock) Now() time.Time { return clock.now }

type streamTestAdapter struct {
	runtimeID string
	delta     string
	calls     atomic.Int32
	errors    chan error
}

func (adapter *streamTestAdapter) AdapterType() string { return "pi" }
func (adapter *streamTestAdapter) RuntimeInstanceID() string {
	return adapter.runtimeID
}

func (adapter *streamTestAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	adapter.calls.Add(1)
	frames := []bridgev1.Frame{
		streamTestInboundFrame(
			request,
			2,
			bridgev1.MessageAck,
			streamTestJSON(map[string]string{
				"message_id": request.Dispatch.MessageID(),
			}),
		),
		streamTestInboundFrame(
			request,
			3,
			bridgev1.MessageEvent,
			streamTestJSON(map[string]string{"delta": adapter.delta}),
		),
		streamTestInboundFrame(
			request,
			4,
			bridgev1.MessageResult,
			streamTestJSON(map[string]string{
				"status": "succeeded",
				"reason": "",
			}),
		),
	}
	for _, frame := range frames {
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			select {
			case adapter.errors <- err:
			default:
			}
			return supervisor.AdapterResult{}, err
		}
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames:        frames,
		ExitCode:             0,
		DispatchAcknowledged: true,
		ResultAcknowledged:   true,
	})
}

type captureCheckingStreamObserver struct {
	store         *evidence.Store
	evidenceID    string
	stream        *api.TeamExecutionStream
	observed      atomic.Bool
	staleStream   *api.TeamExecutionStream
	staleRejected atomic.Bool
}

func (observer *captureCheckingStreamObserver) ObserveNodeOutput(
	ctx context.Context,
	output app.NodeOutput,
) error {
	if output.AuthorizedFrame().Frame().Type() == bridgev1.MessageEvent {
		capture, found, err := observer.store.AttemptCapture(
			ctx,
			observer.evidenceID,
		)
		if err != nil || !found || capture.FrameCount() < 2 {
			return fmt.Errorf("attempt capture not durable before output: %w", err)
		}
		observer.observed.Store(true)
		if observer.staleStream != nil {
			if err := observer.staleStream.ObserveNodeOutput(
				ctx,
				output,
			); !errors.Is(err, api.ErrInvalidNodeOutput) {
				return fmt.Errorf("stale generation observer error = %v", err)
			}
			observer.staleRejected.Store(true)
		}
	}
	return observer.stream.ObserveNodeOutput(ctx, output)
}

func TestTeamExecutionStreamReceivesPostCaptureAuthorizedOutput(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 18, 0, 0, 0, time.UTC)
	clock := &streamTestClock{now: now}
	db := openStreamTestDB(t)
	store := journal.NewStore(db)
	seedStreamTestRuntime(t, store, "runtime-stream-integration", now)
	seedStreamTestRuntime(t, store, "runtime-stream-verifier", now)
	seedStreamTestTeam(t, store, now)

	workRandom := make([]byte, 4096)
	for index := range workRandom {
		workRandom[index] = 0x51 + byte(index/16)
	}
	workAuthority, err := work.NewAuthority(
		store,
		clock.Now,
		bytes.NewReader(workRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	grantRandom := make([]byte, 4096)
	for index := range grantRandom {
		grantRandom[index] = 0x61 + byte(index/48)
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		clock.Now,
		bytes.NewReader(grantRandom),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(db)
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	artifactParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifactStore, err := evidence.NewStore(
		filepath.Join(artifactParent, "evidence"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = artifactStore.Close() })
	coordinator, err := app.NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		artifactStore,
	)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-stream-integration",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             "Stream integration",
			AgentInstanceID:   "agent-stream-integration",
			RuntimeInstanceID: "runtime-stream-integration",
			Role:              teams.ExecutionRoleMain,
			MaxAttempts:       1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	outputContract, recoveryPolicy, acceptanceContract :=
		streamTestSemantics(t)
	semanticBinding := work.TeamNodeSemanticBinding{
		LogicalNodeID:             "main",
		OutputContractVersion:     outputContract.Version(),
		OutputContractDigest:      outputContract.Digest(),
		RecoveryPolicyVersion:     recoveryPolicy.Version(),
		RecoveryPolicyDigest:      recoveryPolicy.Digest(),
		AttemptCredits:            recoveryPolicy.AttemptCredits(),
		PrimaryWorkflowPath:       "primary",
		WorkflowFallbackKey:       recoveryPolicy.WorkflowFallbackKey(),
		RecoveryApprovalRequired:  recoveryPolicy.RecoveryApprovalRequired(),
		AcceptanceContractVersion: acceptanceContract.Version(),
		AcceptanceContractDigest:  acceptanceContract.Digest(),
		AcceptanceRisk:            string(acceptanceContract.Risk()),
		IndependentVerifierRequired: acceptanceContract.
			IndependentVerifierRequired(),
		VerifierAgentInstanceID:   "agent-stream-verifier",
		VerifierRuntimeInstanceID: "runtime-stream-verifier",
		VerifierWorkflowPath:      "independent-verification",
	}
	view := readModel.GlobalReadView()
	executionBinding := streamTestExecutionBinding(
		t, "runtime-stream-integration",
	)
	contextCapsule := streamTestContextCapsule(t, plan, executionBinding)
	contextPayload, err := contextcapsule.RenderDispatchPayload(contextCapsule)
	if err != nil {
		t.Fatal(err)
	}
	selection := work.TeamAttemptSelection{
		LogicalNodeID: "main", AttemptNumber: 1,
		ExecutionBinding: executionBinding,
		ContextCapsule:   contextCapsule,
	}
	dispatched, err := workAuthority.DispatchTeamReadySet(
		ctx,
		work.TeamDispatchInput{
			Plan:                 plan,
			ReadyAttempts:        []work.TeamAttemptSelection{selection},
			SemanticBindings:     []work.TeamNodeSemanticBinding{semanticBinding},
			ViewVersion:          view.Version(),
			ExpectedHeads:        streamTestDispatchHeads(view, plan, executionBinding),
			AuthoritativeTime:    now,
			PrepareLeaseDuration: time.Minute,
			CorrelationID:        "11111111-1111-4111-8111-111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	staleReadModel := projection.New(db)
	if err := staleReadModel.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	stream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: plan.TeamInstanceID(),
		Journal:        store,
		Projection:     readModel,
		Now:            clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := stream.Subscribe(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = subscription.Close() })
	closedSubscription, err := stream.Subscribe(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := closedSubscription.Close(); err != nil {
		t.Fatal(err)
	}
	staleStream, err := api.NewTeamExecutionStream(api.TeamExecutionStreamConfig{
		TeamInstanceID: plan.TeamInstanceID(),
		Journal:        store,
		Projection:     staleReadModel,
		Now:            clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	staleSubscription, err := staleStream.Subscribe(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = staleSubscription.Close() })

	node := dispatched.Nodes()[0]
	attempt := node.Attempt()
	evidenceID := streamTestAttemptIdentity("evidence", plan, "main", 1)
	adapter := &streamTestAdapter{
		runtimeID: "runtime-stream-integration",
		delta:     "authorized-stream-delta",
		errors:    make(chan error, 1),
	}
	executor := newStreamTestSupervisor(
		t,
		workAuthority,
		grantAuthority,
		adapter,
	)
	dispatchFrame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		CorrelationID:         "11111111-1111-4111-8111-111111111111",
		WorkItemID:            attempt.WorkItemID(),
		RunID:                 attempt.RunID(),
		ClaimGeneration:       attempt.ClaimGeneration(),
		RuntimeInstanceID:     attempt.RuntimeInstanceID(),
		SenderAgentInstanceID: attempt.AgentInstanceID(),
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             now,
		Payload:               contextPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                  "profile-stream-integration",
		AdapterType:         "pi",
		ProviderID:          "openai",
		ProviderAccountID:   "openai.stream",
		ModelID:             "gpt-test",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("f", 64),
		CredentialReference: "credential-ref-stream-integration",
		CredentialRevision:  1,
		Timeout:             5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                "runtime-stream-integration",
		DeviceID:          "device-1",
		AdapterType:       "pi",
		DisplayName:       "runtime-stream-integration",
		ExecutableVersion: "1.0.0",
		Status:            loomruntime.RuntimeOnline,
		Capacity:          1,
	})
	if err != nil {
		t.Fatal(err)
	}
	verifierProfile, err := loomruntime.NewRuntimeProfile(
		loomruntime.RuntimeProfile{
			ID:                  "profile-stream-verifier",
			AdapterType:         "pi",
			ProviderID:          "anthropic",
			ProviderAccountID:   "anthropic.stream-verifier",
			ModelID:             "claude-sonnet",
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("a", 64),
			CredentialReference: "credential-ref-stream-verifier",
			CredentialRevision:  1,
			Timeout:             5 * time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	verifierInstance, err := loomruntime.NewRuntimeInstance(
		loomruntime.RuntimeInstance{
			ID:                "runtime-stream-verifier",
			DeviceID:          "device-1",
			AdapterType:       "pi",
			DisplayName:       "runtime-stream-verifier",
			ExecutableVersion: "1.0.0",
			Status:            loomruntime.RuntimeOnline,
			Capacity:          1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	verifierAdapter := &streamTestAdapter{
		runtimeID: "runtime-stream-verifier",
		delta:     "verifier-private-delta",
		errors:    make(chan error, 1),
	}
	verifierExecutor := newStreamTestSupervisor(
		t,
		workAuthority,
		grantAuthority,
		verifierAdapter,
	)
	observer := &captureCheckingStreamObserver{
		store:       artifactStore,
		evidenceID:  evidenceID,
		stream:      stream,
		staleStream: staleStream,
	}
	clock.now = now.Add(2 * time.Minute)
	result, err := coordinator.Run(ctx, app.TeamExecutionRequest{
		Plan: plan,
		Nodes: []app.TeamNodeExecution{{
			LogicalNodeID:  "main",
			AttemptNumber:  1,
			WorkflowPath:   "primary",
			SourcePath:     t.TempDir(),
			Profile:        profile,
			Instance:       instance,
			Dispatch:       dispatchFrame,
			Executor:       executor,
			ContextCapsule: contextCapsule,
		}},
		Semantics: []app.TeamNodeSemantics{{
			LogicalNodeID:             "main",
			OutputContract:            outputContract,
			RecoveryPolicy:            recoveryPolicy,
			AcceptanceContract:        acceptanceContract,
			PrimaryWorkflowPath:       "primary",
			VerifierAgentInstanceID:   "agent-stream-verifier",
			VerifierRuntimeInstanceID: "runtime-stream-verifier",
			VerifierWorkflowPath:      "independent-verification",
			VerifierExecution: &app.TeamVerifierExecution{
				SourcePath: t.TempDir(),
				Profile:    verifierProfile,
				Instance:   verifierInstance,
				Executor:   verifierExecutor,
			},
		}},
		AuthoritativeTime:    clock.now,
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        time.Minute,
		CorrelationID:        "11111111-1111-4111-8111-111111111111",
		OutputObserver:       observer,
	})
	if err != nil || result.Team().Status() != "succeeded" {
		select {
		case adapterErr := <-adapter.errors:
			t.Logf("adapter FrameSink error = %v", adapterErr)
		default:
		}
		t.Fatalf("Run() = %#v, %v", result, err)
	}
	if adapter.calls.Load() != 1 ||
		verifierAdapter.calls.Load() != 1 ||
		!observer.observed.Load() ||
		!observer.staleRejected.Load() {
		t.Fatalf(
			"source calls = %d, verifier calls = %d, post-capture observed = %v, stale rejected = %v",
			adapter.calls.Load(),
			verifierAdapter.calls.Load(),
			observer.observed.Load(),
			observer.staleRejected.Load(),
		)
	}
	cancelled, cancelStale := context.WithCancel(ctx)
	cancelStale()
	if _, err := staleSubscription.Next(
		cancelled,
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("stale generation published item: %v", err)
	}

	nextContext, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	item, err := subscription.Next(nextContext)
	if err != nil {
		t.Fatal(err)
	}
	delivery, ok := item.Delivery()
	if !ok {
		t.Fatal("tentative delivery missing")
	}
	encoded, err := json.Marshal(delivery)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Kind      string `json:"kind"`
		Authority string `json:"authority"`
		Payload   struct {
			TextDelta string `json:"text_delta"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Kind != "node_output_delta" ||
		wire.Authority != "tentative" ||
		wire.Payload.TextDelta != adapter.delta {
		t.Fatalf("tentative delivery = %s", encoded)
	}
	noVerifierContext, cancelVerifier := context.WithCancel(ctx)
	cancelVerifier()
	if _, err := subscription.Next(
		noVerifierContext,
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("verifier output reached source subscription: %v", err)
	}

	receipt, found, err := artifactStore.AttemptReceipt(
		ctx,
		evidenceID,
	)
	if err != nil || !found || receipt.Digest() == "" {
		t.Fatalf("AttemptReceipt() = %#v, %v, %v", receipt, found, err)
	}
	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	evidenceSubmitted := false
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte(adapter.delta)) {
			t.Fatalf("tentative delta persisted in Journal Event %s", event.ID)
		}
		if bytes.Contains(
			event.PayloadJSON,
			[]byte(verifierAdapter.delta),
		) {
			t.Fatalf("verifier delta persisted in Journal Event %s", event.ID)
		}
		if event.Type == "EvidenceSubmitted" {
			evidenceSubmitted = true
		}
	}
	if !evidenceSubmitted {
		t.Fatal("EvidenceSubmitted Event missing")
	}

	page, err := stream.ReadPage(ctx, "", journal.MaxReadPageEvents)
	if err != nil {
		t.Fatal(err)
	}
	pageJSON, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var authoritative struct {
		Records []struct {
			Kind          string `json:"kind"`
			Authority     string `json:"authority"`
			LogicalNodeID string `json:"logical_node_id"`
			AttemptNumber int    `json:"attempt_number"`
			SourceStream  string `json:"source_stream_id"`
		} `json:"records"`
		Board struct {
			Status string `json:"status"`
			Nodes  []struct {
				LogicalNodeID                  string `json:"logical_node_id"`
				Status                         string `json:"status"`
				ContextCapsuleAvailable        bool   `json:"context_capsule_available"`
				ContextCapsuleDigest           string `json:"context_capsule_digest"`
				ContextDisclosureReceiptDigest string `json:"context_disclosure_receipt_digest"`
				ContextAdapterID               string `json:"context_adapter_id"`
				DisclosurePolicyID             string `json:"disclosure_policy_id"`
				ContextOmissionCount           int    `json:"context_omission_count"`
			} `json:"nodes"`
		} `json:"board"`
		Attention []json.RawMessage `json:"attention"`
	}
	if err := json.Unmarshal(pageJSON, &authoritative); err != nil {
		t.Fatal(err)
	}
	finalWorkItem, found := readModel.GlobalReadView().WorkItem(
		attempt.WorkItemID(),
	)
	if !found ||
		finalWorkItem.SourceEvidenceID == "" ||
		finalWorkItem.VerifierWorkItemID == "" ||
		finalWorkItem.VerifierRunID == "" ||
		finalWorkItem.VerifierEvidenceID == "" {
		t.Fatalf("final high-risk WorkItem = %#v, %v", finalWorkItem, found)
	}
	kinds := make(map[string]bool)
	relatedStreams := make(map[string]bool)
	for _, record := range authoritative.Records {
		if record.Authority != "journal" {
			t.Fatalf("authoritative page record = %#v", record)
		}
		kinds[record.Kind] = true
		relatedStreams[record.SourceStream] = true
		switch record.Kind {
		case "node_scheduled", "node_rebound", "run_started",
			"run_terminal", "node_attempt_terminal",
			"ready_for_review", "node_acceptance",
			"work_item_done", "evidence_available":
			if record.LogicalNodeID != "main" ||
				record.AttemptNumber != 1 {
				t.Fatalf("resolved lineage record = %#v", record)
			}
		}
	}
	for _, streamID := range []string{
		"run/" + finalWorkItem.RunID,
		"evidence/" + finalWorkItem.SourceEvidenceID,
		"work-item/" + finalWorkItem.VerifierWorkItemID,
		"run/" + finalWorkItem.VerifierRunID,
		"evidence/" + finalWorkItem.VerifierEvidenceID,
	} {
		if !relatedStreams[streamID] {
			t.Fatalf(
				"authoritative related stream %q missing",
				streamID,
			)
		}
	}
	for _, kind := range []string{
		"team_planned",
		"node_scheduled",
		"ready_set_dispatched",
		"node_rebound",
		"run_started",
		"run_terminal",
		"node_attempt_terminal",
		"evidence_available",
		"team_terminal",
	} {
		if !kinds[kind] {
			t.Fatalf("authoritative kind %q missing: %s", kind, pageJSON)
		}
	}
	if authoritative.Board.Status != "succeeded" ||
		len(authoritative.Board.Nodes) != 1 ||
		authoritative.Board.Nodes[0].LogicalNodeID != "main" ||
		authoritative.Board.Nodes[0].Status != "succeeded" ||
		!authoritative.Board.Nodes[0].ContextCapsuleAvailable ||
		authoritative.Board.Nodes[0].ContextCapsuleDigest != contextCapsule.Digest() ||
		authoritative.Board.Nodes[0].ContextDisclosureReceiptDigest !=
			contextCapsule.DisclosureReceiptDigest() ||
		authoritative.Board.Nodes[0].ContextAdapterID != "context:pi:v1" ||
		authoritative.Board.Nodes[0].DisclosurePolicyID != "policy.stream-test" ||
		authoritative.Board.Nodes[0].ContextOmissionCount != 0 ||
		len(authoritative.Attention) != 0 {
		t.Fatalf("final board/Attention = %s", pageJSON)
	}

	if _, err := db.ExecContext(
		ctx,
		`DROP TRIGGER events_no_delete`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(
		ctx,
		`DELETE FROM events WHERE stream_id = ?`,
		"evidence/"+finalWorkItem.VerifierEvidenceID,
	); err != nil {
		t.Fatal(err)
	}
	missingEvidenceView := projection.New(db)
	if err := missingEvidenceView.Rebuild(ctx); err != nil {
		t.Fatalf("corrupt fixture Projection rebuild = %v", err)
	}
	missingEvidenceStream, err := api.NewTeamExecutionStream(
		api.TeamExecutionStreamConfig{
			TeamInstanceID: plan.TeamInstanceID(),
			Journal:        store,
			Projection:     missingEvidenceView,
			Now:            clock.Now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missingEvidenceStream.ReadPage(
		ctx,
		"",
		journal.MaxReadPageEvents,
	); !errors.Is(err, api.ErrInvalidDeliveryRecord) {
		t.Fatalf("missing verifier Evidence relation error = %v", err)
	}
}

func streamTestContextCapsule(
	t testing.TB,
	plan teams.ExecutionPlan,
	binding loomruntime.FrozenExecutionBinding,
) contextcapsule.RoleContextCapsule {
	t.Helper()
	node := plan.Nodes()[0]
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "team-conversation:" + plan.TeamInstanceID(),
			TeamID:         plan.TeamInstanceID(), AgentID: node.AgentInstanceID(), RoleID: "main",
			ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
			ModelID: binding.ModelID, AuthMode: string(binding.AuthMode),
			ContextAdapterID:   "context:pi:v1",
			DisclosurePolicyID: "policy.stream-test", DisclosurePolicyVersion: 1,
			TokenBudget: 64,
		},
		[]contextcapsule.ItemInput{{
			ItemID: "goal-1", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
			Content:    []byte("Execute the stream integration test."),
			SourceType: contextcapsule.SourceAuthority,
			SourceRef:  "team-plan:" + plan.Digest(),
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return capsule
}

func streamTestExecutionBinding(
	t testing.TB,
	runtimeInstanceID string,
) loomruntime.FrozenExecutionBinding {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                  "profile-stream-integration",
		AdapterType:         "pi",
		ProviderID:          "openai",
		ProviderAccountID:   "openai.stream",
		ModelID:             "gpt-test",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("f", 64),
		CredentialReference: "credential-ref-stream-integration",
		CredentialRevision:  1,
		Timeout:             5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: runtimeInstanceID, DeviceID: "device-1",
		AdapterType: "pi", DisplayName: runtimeInstanceID,
		ExecutableVersion: "1.0.0", Status: loomruntime.RuntimeOnline,
		Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}

func streamTestSemantics(
	t testing.TB,
) (
	verification.OutputContract,
	rules.RecoveryPolicy,
	verification.AcceptanceContract,
) {
	t.Helper()
	outputContract, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputInvalid,
	)
	if err != nil {
		t.Fatal(err)
	}
	recoveryPolicy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version:          1,
		AttemptCredits:   0,
		ExhaustionAction: rules.ExhaustionBlocked,
	})
	if err != nil {
		t.Fatal(err)
	}
	acceptanceContract, err := verification.NewAcceptanceContract(
		1,
		[]string{"controlled output is accepted"},
		verification.AcceptanceRiskHigh,
	)
	if err != nil {
		t.Fatal(err)
	}
	return outputContract, recoveryPolicy, acceptanceContract
}

func streamTestDispatchHeads(
	view projection.GlobalReadView,
	plan teams.ExecutionPlan,
	binding loomruntime.FrozenExecutionBinding,
) []journal.StreamHead {
	workItemID := streamTestAttemptIdentity("work", plan, "main", 1)
	runID := streamTestAttemptIdentity("run", plan, "main", 1)
	streamIDs := []string{
		"provider-account-capacity/" + binding.ProviderAccountID,
		"provider-account-policy/" + binding.ProviderAccountID,
		"run/" + runID,
		"runtime_capacity:runtime-stream-integration",
		"runtime_instance:runtime-stream-integration",
		"team-execution/" + plan.TeamInstanceID(),
		"work-item/" + workItemID,
		"work-run-identity/v1",
	}
	rateCardStreamID, err := work.ProviderModelRateCardStreamID(
		binding.ProviderID, binding.ProviderAccountID, binding.ModelID,
	)
	if err != nil {
		panic(err)
	}
	streamIDs = append(streamIDs, rateCardStreamID)
	heads := make([]journal.StreamHead, len(streamIDs))
	for index, streamID := range streamIDs {
		head, ok := view.Head(streamID)
		if !ok {
			head = journal.StreamHead{StreamID: streamID}
		}
		heads[index] = head
	}
	return heads
}

func streamTestAttemptIdentity(
	label string,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		label,
		plan.TeamInstanceID(),
		plan.Digest(),
		logicalNodeID,
		fmt.Sprint(attemptNumber),
	}, "\x00")))
	return "team-" + label + "-" + hex.EncodeToString(digest[:16])
}

func newStreamTestSupervisor(
	t testing.TB,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	adapter supervisor.RuntimeAdapter,
) app.ManagedNodeExecutor {
	t.Helper()
	workspaceRoot := t.TempDir()
	if err := os.Chmod(workspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := supervisor.New(
		supervisor.Config{
			WorkspaceRoot:  workspaceRoot,
			CleanupTimeout: time.Second,
		},
		workAuthority,
		grantAuthority,
		adapter,
	)
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func streamTestInboundFrame(
	request supervisor.AdapterRequest,
	sequence int64,
	messageType bridgev1.MessageType,
	payload []byte,
) bridgev1.Frame {
	material := sha256.Sum256([]byte(fmt.Sprintf(
		"%s/%d/%s",
		request.Binding.RunID,
		sequence,
		messageType,
	)))
	material[6] = material[6]&0x0f | 0x40
	material[8] = material[8]&0x3f | 0x80
	messageID := fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		material[0:4],
		material[4:6],
		material[6:8],
		material[8:10],
		material[10:16],
	)
	frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             messageID,
		CorrelationID:         request.Dispatch.CorrelationID(),
		WorkItemID:            request.Binding.WorkItemID,
		RunID:                 request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              sequence,
		Type:                  messageType,
		EmittedAt:             request.Dispatch.EmittedAt(),
		Payload:               payload,
	})
	if err != nil {
		panic(err)
	}
	return frame
}

func streamTestJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func openStreamTestDB(t testing.TB) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf(
		"file:%s?%s",
		filepath.Join(t.TempDir(), "team-stream.db"),
		values.Encode(),
	))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedStreamTestRuntime(
	t testing.TB,
	store *journal.Store,
	runtimeID string,
	now time.Time,
) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": strings.Repeat("a", 64),
		"source_probe_id":  "probe-1",
		"instance": map[string]any{
			"id":                    runtimeID,
			"device_id":             "device-1",
			"adapter_type":          "pi",
			"display_name":          runtimeID,
			"executable_version":    "1.0.0",
			"status":                "online",
			"observed_capabilities": []string{"models"},
			"capacity":              1,
		},
		"model_ids": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(context.Background(), journal.Event{
		ID:             "runtime-" + runtimeID,
		StreamID:       "runtime_instance:" + runtimeID,
		Seq:            1,
		IdempotencyKey: "runtime-" + runtimeID,
		Type:           "RuntimeInstanceDiscovered",
		SchemaVersion:  1,
		EmittedAt:      now.Add(-time.Minute),
		CorrelationID:  "22222222-2222-4222-8222-222222222222",
		PayloadJSON:    payload,
	}); err != nil {
		t.Fatal(err)
	}
}

func seedStreamTestTeam(
	t testing.TB,
	store *journal.Store,
	now time.Time,
) {
	t.Helper()
	digestA := strings.Repeat("a", 64)
	digestB := strings.Repeat("b", 64)
	teamPayload := map[string]any{
		"team": map[string]any{
			"id":                      "team-stream-integration",
			"work_request_id":         "request-stream-integration",
			"source_kind":             "saved_team",
			"team_definition_id":      "team.definition.stream",
			"team_definition_version": 1,
			"team_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project-stream-integration",
				"generation_id": "",
			},
			"team_definition_digest": digestA,
			"source_plan_digest":     digestB,
			"state":                  "created",
			"created_at":             now.Add(-time.Minute).Unix(),
		},
		"dormant_sub_agents":       []any{},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_instance_count":      1,
		"agent_instance_count":     1,
		"active_sub_agent_count":   0,
		"work_item_count":          0,
	}
	agentPayload := map[string]any{
		"main_agent": map[string]any{
			"id":                       "agent-stream-integration",
			"team_instance_id":         "team-stream-integration",
			"agent_definition_id":      "agent.definition.stream",
			"agent_definition_version": 1,
			"agent_definition_scope":   "project",
			"scope_identity": map[string]any{
				"project_id":    "project-stream-integration",
				"generation_id": "",
			},
			"runtime_profile_id":  "profile-stream-integration",
			"runtime_instance_id": "runtime-stream-integration",
			"is_main":             true,
			"state":               "created",
		},
		"runtime_binding": map[string]any{
			"accepted":    true,
			"profile_id":  "profile-stream-integration",
			"instance_id": "runtime-stream-integration",
		},
		"source_plan_digest":       digestB,
		"source_record_set_digest": digestA,
		"team_created_at":          now.Add(-time.Minute).Unix(),
		"binding_digest":           digestB,
		"runtime_discovery_digest": digestA,
	}
	for _, event := range []journal.Event{
		{
			ID:             "event-stream-team-created",
			StreamID:       "team_instance:team-stream-integration",
			Seq:            1,
			IdempotencyKey: "key-stream-team-created",
			Type:           "TeamInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      now.Add(-time.Minute),
			CorrelationID:  "request-stream-integration",
			PayloadJSON:    streamTestJSON(teamPayload),
		},
		{
			ID:             "event-stream-agent-created",
			StreamID:       "agent_instance:agent-stream-integration",
			Seq:            1,
			IdempotencyKey: "key-stream-agent-created",
			Type:           "AgentInstanceCreated",
			SchemaVersion:  1,
			EmittedAt:      now.Add(-time.Minute),
			CorrelationID:  "request-stream-integration",
			CausationID:    "event-stream-team-created",
			PayloadJSON:    streamTestJSON(agentPayload),
		},
	} {
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}
