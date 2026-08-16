package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestProductAttemptLoopContextToolPolicySeparatesHarnessAndNativeProofs(t *testing.T) {
	tests := []struct {
		adapterType string
		parallel    int
		mode        work.ToolExecutionMode
	}{
		{harnessadapter.CodexAdapterType, 4, work.ToolExecutionParallel},
		{harnessadapter.ClaudeCodeAdapterType, 4, work.ToolExecutionParallel},
		{nativeadapter.LoomNativeAgentAdapterType, 1, work.ToolExecutionExclusive},
		{"pi", 1, work.ToolExecutionExclusive},
		{"pi-cli", 1, work.ToolExecutionExclusive},
		{"future-runtime", 1, work.ToolExecutionExclusive},
	}
	for _, test := range tests {
		t.Run(test.adapterType, func(t *testing.T) {
			parallel, mode := productAttemptLoopContextToolPolicy(test.adapterType)
			if parallel != test.parallel || mode != test.mode {
				t.Fatalf("policy = %d/%q, want %d/%q", parallel, mode, test.parallel, test.mode)
			}
		})
	}
}

func TestProductAttemptLoopKeepsHarnessContextPendingUntilFinalOutput(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, _ :=
		productAttemptLoopFixtureForAdapter(t, harnessadapter.CodexAdapterType)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	pendingObserved := false
	delegate := &productAttemptLoopAdapterFixture{
		adapterType:                 harnessadapter.CodexAdapterType,
		contextCalls:                2,
		deferContextAcknowledgement: true,
		beforeContextAcknowledgement: func(
			ctx context.Context,
			invocation productAttemptLoopInvocation,
		) error {
			snapshot, snapshotErr := loops.Snapshot(ctx, invocation.Binding)
			if snapshotErr != nil {
				return snapshotErr
			}
			if len(snapshot.Turns) != 1 || len(snapshot.Turns[0].Steps) != 1 ||
				len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 {
				return fmt.Errorf("pending Harness Context snapshot = %#v", snapshot)
			}
			for _, call := range snapshot.Turns[0].Steps[0].ToolCalls {
				if !call.Dispatched || call.ResultStatus != attemptpayload.FactAccepted {
					return fmt.Errorf("Harness Context call was not pending: %#v", call)
				}
			}
			pendingObserved = true
			return nil
		},
	}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	request.ContextRetriever = productAttemptLoopRetrieverFixture{
		content: []byte("Harness Context must remain outside authority facts"),
	}
	if _, err := adapter.Execute(ctx, request); err != nil {
		t.Fatal(err)
	}
	if !pendingObserved {
		t.Fatal("two pending Harness Context results were not observed before final output")
	}
	binding, budget, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	if budget.MaxParallelToolCalls != 4 {
		t.Fatalf("Harness parallel Context budget = %d", budget.MaxParallelToolCalls)
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range snapshot.Turns[0].Steps[0].ToolCalls {
		if call.ResultStatus != attemptpayload.FactDelivered ||
			call.DeliveryProof != attemptpayload.ProofHarnessFinalOutput {
			t.Fatalf("final Harness Context call = %#v", call)
		}
	}
}

func TestProductAttemptLoopRuntimeGovernsContextDelivery(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	delegate := &productAttemptLoopAdapterFixture{}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	secretContent := []byte("context-content-that-must-not-enter-the-journal")
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: secretContent}

	result, err := adapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode() != 0 || delegate.calls != 1 || delegate.claimID != run.ClaimID() ||
		delegate.incidentID != request.Dispatch.CorrelationID() || !delegate.invocationFound ||
		delegate.invocation.Binding.AttemptID != run.ID() ||
		delegate.invocation.TurnID == "" || delegate.invocation.StepID == "" {
		t.Fatalf("adapter result/calls/identity = %#v / %d / %q / %q", result, delegate.calls, delegate.claimID, delegate.incidentID)
	}
	binding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 || snapshot.Turns[0].Status != work.AttemptTurnSucceeded ||
		len(snapshot.Turns[0].Steps) != 1 ||
		snapshot.Turns[0].Steps[0].Outcome != work.AttemptStepFinal ||
		len(snapshot.Turns[0].Steps[0].ToolCalls) != 1 ||
		snapshot.Turns[0].Steps[0].ToolCalls[0].ResultStatus != attemptpayload.FactDelivered {
		t.Fatalf("Attempt loop snapshot = %#v", snapshot)
	}

	events, err := journalStore.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := []string{
		"AttemptLoopStarted", "TurnStarted", "StepStarted", "ModelRequestAdmitted",
		"ToolCallAdmitted", "ToolDispatchCommitted", "StepEnded", "TurnEnded",
	}
	gotTypes := make([]string, 0, len(wantTypes))
	acceptedFacts, deliveredFacts := 0, 0
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, secretContent) {
			t.Fatalf("Journal event %s contains disclosed Context content", event.Type)
		}
		for _, wanted := range wantTypes {
			if event.Type == wanted {
				gotTypes = append(gotTypes, event.Type)
				break
			}
		}
		if event.Type == "ToolResultAccepted" {
			acceptedFacts++
		}
		if event.Type == "ToolResultDelivered" {
			deliveredFacts++
		}
	}
	if !equalProductAttemptLoopStrings(gotTypes, wantTypes) ||
		acceptedFacts != 1 || deliveredFacts != 1 {
		t.Fatalf("Attempt loop event order = %#v, want %#v", gotTypes, wantTypes)
	}

	substitutedProfile := productAttemptLoopProfile("account.other")
	substitutedBinding, err := runtime.FreezeExecutionBinding(
		substitutedProfile,
		productAttemptLoopInstance(),
	)
	if err != nil {
		t.Fatal(err)
	}
	substituted := request
	substituted.ExecutionBinding = substitutedBinding
	substituted.ContextCapsule = productAttemptLoopCapsule(t, substitutedProfile)
	substituted.RouteSegment = productAttemptLoopRouteSegment(
		t, run, substitutedBinding, substituted.ContextCapsule,
	)
	if _, err := adapter.Execute(ctx, substituted); !errors.Is(err, work.ErrAttemptLoopAuthority) {
		t.Fatalf("Execution Binding substitution error = %v", err)
	}
	if delegate.calls != 1 {
		t.Fatalf("substituted request reached runtime: calls=%d", delegate.calls)
	}
}

func TestProductAttemptLoopRuntimeGovernsSequentialContextDelivery(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	delegate := &productAttemptLoopAdapterFixture{contextCalls: 2}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	secretContent := []byte("sequential-context-content-must-not-enter-authority-facts")
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: secretContent}

	result, err := adapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode() != 0 || delegate.calls != 1 {
		t.Fatalf("adapter result/calls = %#v / %d", result, delegate.calls)
	}
	binding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 || snapshot.Turns[0].Status != work.AttemptTurnSucceeded ||
		len(snapshot.Turns[0].Steps) != 1 ||
		snapshot.Turns[0].Steps[0].Outcome != work.AttemptStepFinal ||
		len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 {
		t.Fatalf("sequential Attempt loop snapshot = %#v", snapshot)
	}
	for index, call := range snapshot.Turns[0].Steps[0].ToolCalls {
		if call.Sequence != int64(index+1) || !call.Dispatched ||
			call.ResultStatus != attemptpayload.FactDelivered ||
			call.DeliveryProof != attemptpayload.ProofHarnessFinalOutput {
			t.Fatalf("sequential ToolCall %d = %#v", index, call)
		}
	}
	if snapshot.Turns[0].Steps[0].ToolCalls[0].CallID ==
		snapshot.Turns[0].Steps[0].ToolCalls[1].CallID {
		t.Fatal("sequential Context deliveries reused one ToolCall identity")
	}

	events, err := journalStore.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := []string{
		"ToolCallAdmitted", "ToolDispatchCommitted",
		"ToolCallAdmitted", "ToolDispatchCommitted",
		"StepEnded", "TurnEnded",
	}
	gotTypes := make([]string, 0, len(wantTypes))
	factTypes := make(map[string][]string)
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, secretContent) {
			t.Fatalf("Journal event %s contains sequential Context content", event.Type)
		}
		for _, wanted := range []string{
			"ToolCallAdmitted", "ToolDispatchCommitted", "StepEnded", "TurnEnded",
		} {
			if event.Type == wanted {
				gotTypes = append(gotTypes, event.Type)
				break
			}
		}
		if event.Type == "ToolResultAccepted" || event.Type == "ToolResultDelivered" {
			factTypes[event.StreamID] = append(factTypes[event.StreamID], event.Type)
		}
	}
	if !equalProductAttemptLoopStrings(gotTypes, wantTypes) {
		t.Fatalf("sequential loop event order = %#v, want %#v", gotTypes, wantTypes)
	}
	if len(factTypes) != 2 {
		t.Fatalf("sequential fact stream count = %d", len(factTypes))
	}
	for streamID, types := range factTypes {
		if !equalProductAttemptLoopStrings(
			types, []string{"ToolResultAccepted", "ToolResultDelivered"},
		) {
			t.Fatalf("sequential fact stream %s = %#v", streamID, types)
		}
	}

	payloadStore.mu.Lock()
	defer payloadStore.mu.Unlock()
	if len(payloadStore.payloads) != 2 {
		t.Fatalf("sequential payload count = %d", len(payloadStore.payloads))
	}
	for _, payload := range payloadStore.payloads {
		if payload.Status != attemptpayload.StatusDelivered {
			t.Fatalf("sequential payload = %#v", payload.Binding)
		}
	}
}

func TestProductLoomNativeGovernsSequentialProviderContextContinuations(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	capsuleValue := productAttemptLoopCapsuleValue(
		t, productAttemptLoopProfile("account.primary"),
	)
	if capsuleValue.AuthorityRecord() != capsule || len(capsuleValue.Omitted()) != 1 {
		t.Fatal("native sequential Capsule fixture drifted")
	}
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(capsuleValue)
	if err != nil {
		t.Fatal(err)
	}
	request.Dispatch, err = bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: request.Dispatch.MessageID(), CorrelationID: request.Dispatch.CorrelationID(),
		WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC), Payload: dispatchPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	omitted := capsuleValue.Omitted()[0]
	toolResponse := func(id string, inputTokens int, outputTokens int) string {
		return fmt.Sprintf(
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"%s","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"%s\",\"content_digest\":\"%s\",\"artifact_ref\":\"%s\"}"}}]}}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}`,
			id, omitted.ItemID, omitted.ContentDigest, omitted.ArtifactRef,
			inputTokens, outputTokens, inputTokens+outputTokens,
		)
	}
	doer := &productNativeAgentInputDoer{responses: []string{
		toolResponse("provider-context-1", 10, 2),
		toolResponse("provider-context-2", 15, 3),
		`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Sequential context complete"}}],"usage":{"prompt_tokens":30,"completion_tokens":5,"total_tokens":35}}`,
	}}
	credential := &productNativeAgentInputCredential{secret: []byte("private-native-key")}
	native, err := nativeadapter.NewDeepSeekAgentAdapter(nativeadapter.DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: executionBinding.RuntimeInstanceID,
		CredentialAccess:  credential,
		Diagnostics:       productNativeAgentInputDiagnostics{},
		Client:            doer,
		Now: func() time.Time {
			return time.Date(2026, 8, 14, 21, 0, 0, 0, time.UTC)
		},
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := newProductAttemptLoopRuntimeAdapter(native, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	sink := &productNativeAgentInputFrameSink{}
	request.FrameSink = sink
	privateContext := []byte("context-content-that-must-not-enter-the-journal")
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: privateContext}
	result, err := adapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	accounting, accountingAvailable := result.Accounting()
	if credential.uses != 1 || doer.calls != 3 || len(doer.bodies) != 3 ||
		len(doer.authorizations) != 3 || len(sink.frames) != 3 ||
		string(sink.frames[1].Payload()) != `{"delta":"Sequential context complete"}` ||
		!accountingAvailable || accounting.InputTokens != 55 ||
		accounting.OutputTokens != 10 || accounting.TotalTokens != 65 {
		t.Fatalf("native sequential result = credential=%d calls=%d accounting=%#v/%t frames=%#v", credential.uses, doer.calls, accounting, accountingAvailable, sink.frames)
	}
	for index, authorization := range doer.authorizations {
		if authorization != "Bearer private-native-key" ||
			bytes.Contains(doer.bodies[index], []byte("private-native-key")) {
			t.Fatalf("native Provider request %d retained credential", index)
		}
	}
	for _, frame := range sink.frames {
		if bytes.Contains(frame.Payload(), privateContext) {
			t.Fatalf("native sequential Context leaked to Bridge: %s", frame.Payload())
		}
	}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := loops.Snapshot(ctx, loopBinding)
	if err != nil || len(snapshot.Turns) != 1 || len(snapshot.Turns[0].Steps) != 1 ||
		len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 ||
		snapshot.Turns[0].Status != work.AttemptTurnSucceeded {
		t.Fatalf("native sequential Attempt loop = %#v, %v", snapshot, err)
	}
	for index, call := range snapshot.Turns[0].Steps[0].ToolCalls {
		if call.Sequence != int64(index+1) ||
			call.ResultStatus != attemptpayload.FactDelivered ||
			call.DeliveryProof != attemptpayload.ProofProviderContinuation {
			t.Fatalf("native sequential ToolCall %d = %#v", index, call)
		}
	}
	payloadStore.mu.Lock()
	if len(payloadStore.payloads) != 2 {
		payloadStore.mu.Unlock()
		t.Fatalf("native sequential payload count = %d", len(payloadStore.payloads))
	}
	for _, payload := range payloadStore.payloads {
		if payload.Status != attemptpayload.StatusDelivered {
			payloadStore.mu.Unlock()
			t.Fatalf("native sequential payload = %#v", payload.Binding)
		}
	}
	payloadStore.mu.Unlock()
	events, err := journalStore.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, privateContext) ||
		bytes.Contains(encoded, []byte("private-native-key")) {
		t.Fatal("native sequential Context or credential entered Journal")
	}
}

func TestProductMissionExecutorComposesAgentInboxWithAttemptAuthority(t *testing.T) {
	ctx := context.Background()
	runs, _, _, _, payloadStore, journalStore := productAttemptLoopFixture(t)
	grantAuthority, err := authorization.NewAuthority(
		journalStore, runs,
		func() time.Time { return time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC) },
		bytes.NewReader(bytes.Repeat([]byte{0x71}, 256)),
	)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pi"), []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	inboxStore := &productAgentInboxStoreFixture{}
	config := productMissionExecutionRuntimeConfig{
		RuntimeSearchPaths: []string{root}, RuntimeInstanceID: "runtime-a",
		LocalModelCatalog: &piadapter.PiLocalModelCatalogConfig{
			PrivateRoot:    filepath.Join(root, "model"),
			ExecutablePath: filepath.Join(root, "llama-server"),
			ModelPath:      filepath.Join(root, "model.gguf"),
		},
		ContextRetrievalStore:   productAgentInboxRetrievalStoreFixture{},
		ContextRetrievalAuditor: productAgentInboxRetrievalAuditorFixture{},
		AttemptPayloadStore:     payloadStore,
		AgentInboxStore:         inboxStore,
		AgentCheckpointStore:    &productAgentCheckpointStoreFixture{},
	}
	executor, err := newProductMissionExecutor(
		ctx, config, root, runs, grantAuthority,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer executor.Close(ctx)
	if executor.agentInbox == nil || executor.runtime.config.AgentInbox == nil ||
		executor.runtime.config.AgentInbox != executor.agentInbox {
		t.Fatalf("Agent Inbox composition = %#v", executor)
	}
	if executor.activeAttempts == nil || executor.runtime.config.ActiveAttempts == nil ||
		executor.runtime.config.ActiveAttempts != executor.activeAttempts {
		t.Fatalf("active Attempt composition = %#v", executor)
	}
	sharedLoops := executor.runtime.config.AttemptLoops
	sharedInbox := executor.runtime.config.AgentInbox
	sharedActive := executor.runtime.config.ActiveAttempts
	sharedConfig := config
	sharedConfig.AttemptLoops = sharedLoops
	sharedConfig.AgentInbox = sharedInbox
	sharedConfig.ActiveAttempts = sharedActive
	sharedExecutor, err := newProductMissionExecutor(
		ctx, sharedConfig, root, runs, grantAuthority,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sharedExecutor.Close(ctx)
	if sharedExecutor.runtime.config.AttemptLoops != sharedLoops ||
		sharedExecutor.agentInbox != sharedInbox ||
		sharedExecutor.activeAttempts != sharedActive {
		t.Fatalf("executor detached shared Attempt governance = %#v", sharedExecutor)
	}

	config.AttemptPayloadStore = nil
	config.ContextRetrievalStore = nil
	config.ContextRetrievalAuditor = nil
	if _, err := newProductMissionExecutor(
		ctx, config, root, runs, grantAuthority,
	); !errors.Is(err, app.ErrInvalidMissionExecution) {
		t.Fatalf("Inbox without Attempt authority = %v", err)
	}
}

func TestProductAttemptLoopRegistersExactActiveAttemptOnlyDuringExecution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, _ := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	registry := newProductActiveAttemptRegistry()
	delegate := &productBlockingAttemptLoopAdapterFixture{
		entered: make(chan struct{}), release: make(chan struct{}),
	}
	adapter, err := newProductAttemptLoopRuntimeAdapterWithRegistry(
		delegate, loops, payloadStore, registry,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: []byte("unused")}
	done := make(chan error, 1)
	go func() {
		_, executeErr := adapter.Execute(ctx, request)
		done <- executeErr
	}()
	select {
	case <-delegate.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("delegate did not enter")
	}

	query := productActiveAttemptQuery{
		ConversationID:  capsule.ConversationID,
		SegmentID:       request.RouteSegment.SegmentID,
		AgentInstanceID: run.AgentInstanceID(), WorkItemID: run.WorkItemID(),
		RunID: run.ID(), ClaimGeneration: run.ClaimGeneration(),
	}
	active, err := registry.Resolve(query)
	if err != nil {
		t.Fatal(err)
	}
	if active.Identity != query ||
		active.AttemptLoopBinding.PayloadAuthority.ExecutionBindingDigest != executionBinding.BindingDigest ||
		active.ExecutionBinding.ProviderAccountID != executionBinding.ProviderAccountID ||
		active.CapsuleDigest != capsule.CapsuleDigest {
		t.Fatalf("active = %#v", active)
	}
	active.ExecutionBinding.Capabilities[0] = "mutated"
	again, err := registry.Resolve(query)
	if err != nil {
		t.Fatal(err)
	}
	if again.ExecutionBinding.Capabilities[0] == "mutated" {
		t.Fatal("resolved capability mutation escaped registry clone")
	}
	stale := query
	stale.ClaimGeneration++
	if _, err := registry.Resolve(stale); !errors.Is(err, errProductActiveAttemptNotFound) {
		t.Fatalf("stale resolve error = %v", err)
	}

	close(delegate.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Resolve(query); !errors.Is(err, errProductActiveAttemptNotFound) {
		t.Fatalf("resolve after completion error = %v", err)
	}
}

func TestProductAttemptLoopRuntimeConsumesSteerThenQueueWithoutDuplicateTerminal(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, _ := work.NewAttemptPayloadAuthority(runs)
	loops, _ := work.NewAttemptLoopAuthority(runs, payloads)
	inboxAuthority, _ := work.NewAgentInboxAuthority(loops)
	inboxStore := newProductMemoryAgentInboxStore()
	inbox, _ := work.NewAgentInboxCoordinator(inboxAuthority, inboxStore)
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	capsuleValue := productAttemptLoopCapsuleValue(t, productAttemptLoopProfile("account.primary"))
	if capsuleValue.AuthorityRecord() != capsule {
		t.Fatal("native Capsule fixture drifted")
	}
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(capsuleValue)
	if err != nil {
		t.Fatal(err)
	}
	request.Dispatch, err = bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     request.Dispatch.MessageID(),
		CorrelationID: request.Dispatch.CorrelationID(),
		WorkItemID:    request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
		Payload:   dispatchPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	delegate := &productAgentInputAttemptLoopAdapterFixture{
		admit: func(ctx context.Context) error {
			steer := productAgentInputPayload(
				loopBinding, request.RouteSegment.SegmentID, "input-steer", 1,
				agentinbox.ModeSteer, "step-"+productDeterministicUUID(
					"attempt-loop-step", loopBinding.AttemptID, "1", "2",
				), 2, "narrow the implementation boundary",
			)
			queue := productAgentInputPayload(
				loopBinding, request.RouteSegment.SegmentID, "input-queue", 2,
				agentinbox.ModeQueue, "", 0, "continue with the accepted tests",
			)
			queue.Binding.TargetTurnID = "turn-queued-2"
			queue.Binding.TargetTurnSequence = 2
			for _, payload := range []agentinbox.Payload{steer, queue} {
				if _, admitErr := inbox.Admit(ctx, loopBinding, payload); admitErr != nil {
					return admitErr
				}
			}
			return nil
		},
	}
	adapter, err := newProductAttemptLoopRuntimeAdapterWithGovernance(
		delegate, loops, payloadStore, newProductActiveAttemptRegistry(), inbox, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: []byte("unused")}
	result, err := adapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode() != 0 || delegate.calls != 1 || delegate.inputBatches != 2 ||
		delegate.terminalResults != 1 || !delegate.zeroized {
		t.Fatalf("result/delegate = %#v / %#v", result, delegate)
	}
	snapshot, err := loops.Snapshot(ctx, loopBinding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 2 || len(snapshot.Turns[0].Steps) != 2 ||
		len(snapshot.Turns[1].Steps) != 1 ||
		snapshot.Turns[0].Steps[0].Outcome != work.AttemptStepContinue ||
		snapshot.Turns[0].Status != work.AttemptTurnSucceeded ||
		snapshot.Turns[1].Status != work.AttemptTurnSucceeded {
		t.Fatalf("multi-Step snapshot = %#v", snapshot)
	}
	events, err := journalStore.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(events)
	for _, forbidden := range [][]byte{
		[]byte("narrow the implementation boundary"),
		[]byte("continue with the accepted tests"),
	} {
		if bytes.Contains(encoded, forbidden) {
			t.Fatal("Agent input plaintext entered the Journal")
		}
	}
}

func TestProductLoomNativeConsumesAuthoritativeInboxAcrossProviderRounds(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, _ := work.NewAttemptPayloadAuthority(runs)
	loops, _ := work.NewAttemptLoopAuthority(runs, payloads)
	inboxAuthority, _ := work.NewAgentInboxAuthority(loops)
	inboxStore := newProductMemoryAgentInboxStore()
	inbox, _ := work.NewAgentInboxCoordinator(inboxAuthority, inboxStore)
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	capsuleValue := productAttemptLoopCapsuleValue(t, productAttemptLoopProfile("account.primary"))
	if capsuleValue.AuthorityRecord() != capsule {
		t.Fatal("native Capsule fixture drifted")
	}
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(capsuleValue)
	if err != nil {
		t.Fatal(err)
	}
	request.Dispatch, err = bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     request.Dispatch.MessageID(),
		CorrelationID: request.Dispatch.CorrelationID(),
		WorkItemID:    request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
		Payload:   dispatchPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	admit := func(ctx context.Context) error {
		steer := productAgentInputPayload(
			loopBinding, request.RouteSegment.SegmentID, "native-steer", 1,
			agentinbox.ModeSteer, productAttemptLoopStepID(loopBinding, 1, 2), 2,
			"use the exact frozen route",
		)
		queue := productAgentInputPayload(
			loopBinding, request.RouteSegment.SegmentID, "native-queue", 2,
			agentinbox.ModeQueue, "", 0, "run the accepted verification",
		)
		queue.Binding.TargetTurnID, queue.Binding.TargetTurnSequence = "turn-native-2", 2
		for _, payload := range []agentinbox.Payload{steer, queue} {
			if _, err := inbox.Admit(ctx, loopBinding, payload); err != nil {
				return err
			}
		}
		return nil
	}
	doer := &productNativeAgentInputDoer{
		admit: admit,
		responses: []string{
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"First native answer"}}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Second native answer"}}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`,
			`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"Final native answer"}}],"usage":{"prompt_tokens":14,"completion_tokens":4,"total_tokens":18}}`,
		},
	}
	credential := &productNativeAgentInputCredential{secret: []byte("private-native-key")}
	checkpointStore := &productAgentCheckpointStoreFixture{
		beforePut: func() error {
			snapshot, snapshotErr := inbox.Snapshot(ctx, loopBinding)
			if snapshotErr != nil {
				return snapshotErr
			}
			pending, consumed := 0, 0
			for _, input := range snapshot.Inputs {
				if input.Status == agentinbox.StatusPending {
					pending++
				}
				if input.Status == agentinbox.StatusConsumed {
					consumed++
				}
			}
			if pending != 2 || consumed != 0 {
				return errors.New("checkpoint did not precede Agent input consumption")
			}
			return nil
		},
	}
	native, err := nativeadapter.NewDeepSeekAgentAdapter(nativeadapter.DeepSeekAgentAdapterConfig{
		RuntimeInstanceID: executionBinding.RuntimeInstanceID,
		CredentialAccess:  credential,
		Diagnostics:       productNativeAgentInputDiagnostics{},
		Client:            doer,
		Now: func() time.Time {
			return time.Date(2026, 8, 14, 19, 0, 0, 0, time.UTC)
		},
		MaxResponseBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := newProductAttemptLoopRuntimeAdapterWithGovernance(
		native, loops, payloadStore, newProductActiveAttemptRegistry(), inbox,
		checkpointStore,
	)
	if err != nil {
		t.Fatal(err)
	}
	sink := &productNativeAgentInputFrameSink{}
	request.FrameSink = sink
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: []byte("unused")}
	result, err := adapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if credential.uses != 1 || doer.calls != 3 || len(doer.bodies) != 3 ||
		!bytes.Contains(doer.bodies[1], []byte("use the exact frozen route")) ||
		!bytes.Contains(doer.bodies[2], []byte("run the accepted verification")) ||
		!result.ResultAcknowledged() || len(sink.frames) != 3 ||
		string(sink.frames[1].Payload()) != `{"delta":"Final native answer"}` {
		t.Fatalf("native execution = credential=%d calls=%d bodies=%q result=%#v", credential.uses, doer.calls, doer.bodies, result)
	}
	if len(checkpointStore.payloads) != 2 ||
		string(checkpointStore.payloads[0].Content) != "First native answer" ||
		checkpointStore.payloads[0].Binding.SegmentID != request.RouteSegment.SegmentID ||
		checkpointStore.payloads[0].Binding.AttemptID != loopBinding.AttemptID ||
		checkpointStore.payloads[0].Binding.AgentInstanceID != "agent-main" ||
		checkpointStore.payloads[0].Binding.RuntimeInstanceID != "runtime-1" ||
		checkpointStore.payloads[0].Binding.ExecutionBindingDigest != executionBinding.BindingDigest ||
		checkpointStore.payloads[0].Binding.CapsuleDigest != capsule.CapsuleDigest {
		t.Fatalf("durable checkpoint bindings = %#v", checkpointStore.payloads)
	}
	snapshot, err := loops.Snapshot(ctx, loopBinding)
	if err != nil || len(snapshot.Turns) != 2 || len(snapshot.Turns[0].Steps) != 2 ||
		len(snapshot.Turns[1].Steps) != 1 || snapshot.Turns[1].Status != work.AttemptTurnSucceeded {
		t.Fatalf("native Attempt loop = %#v, %v", snapshot, err)
	}
	events, err := journalStore.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(events)
	for _, forbidden := range [][]byte{
		[]byte("use the exact frozen route"), []byte("run the accepted verification"),
		[]byte("private-native-key"),
	} {
		if bytes.Contains(encoded, forbidden) {
			t.Fatalf("Journal contains private native input %q", forbidden)
		}
	}
}

type productAgentCheckpointStoreFixture struct {
	payloads  []agentcheckpoint.Payload
	beforePut func() error
}

func (store *productAgentCheckpointStoreFixture) PutAgentCheckpoint(
	_ context.Context,
	payload agentcheckpoint.Payload,
) error {
	if store.beforePut != nil && len(store.payloads) == 0 {
		if err := store.beforePut(); err != nil {
			return err
		}
	}
	store.payloads = append(store.payloads, agentcheckpoint.Payload{
		Binding: payload.Binding,
		Content: bytes.Clone(payload.Content),
	})
	return nil
}

func (store *productAgentCheckpointStoreFixture) ReadAgentCheckpoint(
	_ context.Context,
	binding agentcheckpoint.Binding,
) (agentcheckpoint.Payload, error) {
	for index := range store.payloads {
		if store.payloads[index].Binding == binding {
			return agentcheckpoint.Payload{
				Binding: binding, Content: bytes.Clone(store.payloads[index].Content),
			}, nil
		}
	}
	return agentcheckpoint.Payload{}, errors.New("checkpoint not found")
}

func (store *productAgentCheckpointStoreFixture) ResolveAgentCheckpoint(
	_ context.Context,
	query agentcheckpoint.Query,
) (agentcheckpoint.Payload, error) {
	var matched *agentcheckpoint.Payload
	for index := range store.payloads {
		binding := store.payloads[index].Binding
		if binding.ConversationID != query.ConversationID ||
			binding.SegmentID != query.SegmentID || binding.AttemptID != query.AttemptID ||
			binding.AgentInstanceID != query.AgentInstanceID ||
			binding.WorkItemID != query.WorkItemID || binding.RunID != query.RunID ||
			binding.ClaimGeneration != query.ClaimGeneration ||
			binding.RuntimeInstanceID != query.RuntimeInstanceID ||
			binding.ExecutionBindingDigest != query.ExecutionBindingDigest ||
			binding.CapsuleDigest != query.CapsuleDigest ||
			binding.ContentDigest != query.ContentDigest {
			continue
		}
		if matched != nil {
			return agentcheckpoint.Payload{}, errors.New("checkpoint conflict")
		}
		matched = &store.payloads[index]
	}
	if matched == nil {
		return agentcheckpoint.Payload{}, errors.New("checkpoint not found")
	}
	return agentcheckpoint.Payload{
		Binding: matched.Binding, Content: bytes.Clone(matched.Content),
	}, nil
}

func (store *productAgentCheckpointStoreFixture) DeleteAgentCheckpoint(
	_ context.Context,
	binding agentcheckpoint.Binding,
) error {
	for index := range store.payloads {
		if store.payloads[index].Binding == binding {
			store.payloads[index].Close()
			store.payloads = append(store.payloads[:index], store.payloads[index+1:]...)
			return nil
		}
	}
	return errors.New("checkpoint not found")
}

func TestProductAgentInputSourceRecoversCommittedStepBeforeModelRequest(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, _, _ := productAttemptLoopFixture(t)
	payloads, _ := work.NewAttemptPayloadAuthority(runs)
	loops, _ := work.NewAttemptLoopAuthority(runs, payloads)
	inboxAuthority, _ := work.NewAgentInboxAuthority(loops)
	store := &productFailingMarkAgentInboxStore{
		productMemoryAgentInboxStore: newProductMemoryAgentInboxStore(),
		remainingFailures:            1,
	}
	inbox, _ := work.NewAgentInboxCoordinator(inboxAuthority, store)
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	binding, budget, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	budget.MaxTurns = productAttemptLoopMaxInputTurns
	budget.MaxStepsPerTurn = productAttemptLoopMaxInputSteps
	turnID, stepID, requestID := productAttemptLoopIDs(binding)
	if _, err := loops.StartTurn(ctx, binding, budget, work.AttemptLoopTurnInput{
		TurnID: turnID, Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
		TurnID: turnID, StepID: stepID, Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, work.AttemptLoopModelRequestInput{
		TurnID: turnID, StepID: stepID, RequestID: requestID,
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	nextStepID := productAttemptLoopStepID(binding, 1, 2)
	steer := productAgentInputPayload(
		binding, request.RouteSegment.SegmentID, "recover-source-step", 1,
		agentinbox.ModeSteer, nextStepID, 2, "recover committed source step",
	)
	if _, err := inbox.Admit(ctx, binding, steer); err != nil {
		t.Fatal(err)
	}
	cursor := &productAttemptLoopCursor{
		turnID: turnID, turnSequence: 1, stepID: stepID, stepSequence: 1,
	}
	source := &productAttemptLoopAgentInputSource{
		loops: loops, inbox: inbox, binding: binding, cursor: cursor,
	}
	checkpoint := runtime.AgentInputCheckpoint{OutputDigest: strings.Repeat("4", 64)}
	if batch, ok, err := source.NextAgentInput(ctx, checkpoint); err == nil || ok {
		batch.Close()
		t.Fatalf("first step delivery = %#v, %v, %v", batch, ok, err)
	}
	wrongCheckpoint := runtime.AgentInputCheckpoint{OutputDigest: strings.Repeat("5", 64)}
	if batch, ok, err := source.NextAgentInput(ctx, wrongCheckpoint); err == nil || ok {
		batch.Close()
		t.Fatalf("wrong-checkpoint recovery = %#v, %v, %v", batch, ok, err)
	}
	batch, ok, err := source.NextAgentInput(ctx, checkpoint)
	if err != nil || !ok || len(batch.Inputs) != 1 ||
		string(batch.Inputs[0].Content) != "recover committed source step" ||
		batch.StepID != nextStepID || batch.StepSequence != 2 {
		batch.Close()
		t.Fatalf("recovered step delivery = %#v, %v, %v", batch, ok, err)
	}
	content := batch.Inputs[0].Content
	batch.Close()
	if !allProductAgentInputBytesZero(content) {
		t.Fatal("recovered source step plaintext was not zeroized")
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil || len(snapshot.Turns[0].Steps) != 2 ||
		snapshot.Turns[0].Steps[1].ModelRequestID == "" {
		t.Fatalf("recovered source step authority = %#v, %v", snapshot, err)
	}
}

func TestProductAgentInputSourceRecoversCommittedQueueBeforeModelRequest(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, _, _ := productAttemptLoopFixture(t)
	payloads, _ := work.NewAttemptPayloadAuthority(runs)
	loops, _ := work.NewAttemptLoopAuthority(runs, payloads)
	inboxAuthority, _ := work.NewAgentInboxAuthority(loops)
	store := &productFailingMarkAgentInboxStore{
		productMemoryAgentInboxStore: newProductMemoryAgentInboxStore(),
		remainingFailures:            1,
	}
	inbox, _ := work.NewAgentInboxCoordinator(inboxAuthority, store)
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	binding, budget, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	budget.MaxTurns = productAttemptLoopMaxInputTurns
	budget.MaxStepsPerTurn = productAttemptLoopMaxInputSteps
	turnID, stepID, requestID := productAttemptLoopIDs(binding)
	if _, err := loops.StartTurn(ctx, binding, budget, work.AttemptLoopTurnInput{
		TurnID: turnID, Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
		TurnID: turnID, StepID: stepID, Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, work.AttemptLoopModelRequestInput{
		TurnID: turnID, StepID: stepID, RequestID: requestID,
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	queued := productAgentInputPayload(
		binding, request.RouteSegment.SegmentID, "recover-source-queue", 1,
		agentinbox.ModeQueue, "", 0, "recover committed source queue",
	)
	queued.Binding.TargetTurnID, queued.Binding.TargetTurnSequence = "turn-recovered-2", 2
	if _, err := inbox.Admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}
	cursor := &productAttemptLoopCursor{
		turnID: turnID, turnSequence: 1, stepID: stepID, stepSequence: 1,
	}
	source := &productAttemptLoopAgentInputSource{
		loops: loops, inbox: inbox, binding: binding, cursor: cursor,
	}
	checkpoint := runtime.AgentInputCheckpoint{OutputDigest: strings.Repeat("4", 64)}
	if batch, ok, err := source.NextAgentInput(ctx, checkpoint); err == nil || ok {
		batch.Close()
		t.Fatalf("first queue delivery = %#v, %v, %v", batch, ok, err)
	}
	queuedStepID := productAttemptLoopStepID(binding, 2, 1)
	queuedModelDigest := productAgentInputModelDigest([]agentinbox.Binding{queued.Binding})
	if _, err := loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
		TurnID: queued.Binding.TargetTurnID, StepID: queuedStepID, Sequence: 1,
		ModelInputDigest: queuedModelDigest,
	}); err != nil {
		t.Fatal(err)
	}
	batch, ok, err := source.NextAgentInput(ctx, checkpoint)
	if err != nil || !ok || len(batch.Inputs) != 1 ||
		string(batch.Inputs[0].Content) != "recover committed source queue" ||
		batch.TurnID != queued.Binding.TargetTurnID || batch.TurnSequence != 2 ||
		batch.StepID != queuedStepID || batch.StepSequence != 1 {
		batch.Close()
		t.Fatalf("recovered queue delivery = %#v, %v, %v", batch, ok, err)
	}
	content := batch.Inputs[0].Content
	batch.Close()
	if !allProductAgentInputBytesZero(content) {
		t.Fatal("recovered source queue plaintext was not zeroized")
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil || len(snapshot.Turns) != 2 || len(snapshot.Turns[1].Steps) != 1 ||
		snapshot.Turns[1].Steps[0].ModelRequestID == "" {
		t.Fatalf("recovered source queue authority = %#v, %v", snapshot, err)
	}
}

type productAgentInboxStoreFixture struct{}

type productNativeAgentInputCredential struct {
	secret []byte
	uses   int
}

func (credential *productNativeAgentInputCredential) UseCredential(
	ctx context.Context,
	_ runtime.FrozenExecutionBinding,
	use func(context.Context, []byte) error,
) error {
	credential.uses++
	secret := bytes.Clone(credential.secret)
	defer func() {
		for index := range secret {
			secret[index] = 0
		}
	}()
	return use(ctx, secret)
}

type productNativeAgentInputDiagnostics struct{}

func (productNativeAgentInputDiagnostics) RecordAgentAttemptDiagnostic(
	context.Context,
	nativeadapter.AgentAttemptDiagnostic,
) error {
	return nil
}

type productNativeAgentInputDoer struct {
	admit          func(context.Context) error
	responses      []string
	bodies         [][]byte
	authorizations []string
	calls          int
}

func (doer *productNativeAgentInputDoer) Do(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	doer.calls++
	doer.bodies = append(doer.bodies, body)
	doer.authorizations = append(doer.authorizations, request.Header.Get("Authorization"))
	request.Header.Del("Authorization")
	if doer.calls == 1 && doer.admit != nil {
		if err := doer.admit(request.Context()); err != nil {
			return nil, err
		}
	}
	if len(doer.responses) == 0 {
		return nil, errors.New("unexpected native Provider request")
	}
	responseBody := doer.responses[0]
	doer.responses = doer.responses[1:]
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Request:    request,
	}, nil
}

type productNativeAgentInputFrameSink struct {
	frames []bridgev1.Frame
}

func (sink *productNativeAgentInputFrameSink) AcceptFrame(
	_ context.Context,
	frame bridgev1.Frame,
) error {
	sink.frames = append(sink.frames, frame)
	return nil
}

type productMemoryAgentInboxStore struct {
	mu       sync.Mutex
	payloads map[string]agentinbox.Payload
}

type productFailingMarkAgentInboxStore struct {
	*productMemoryAgentInboxStore
	remainingFailures int
}

func (store *productFailingMarkAgentInboxStore) MarkAgentInputConsumed(
	ctx context.Context,
	binding agentinbox.Binding,
) error {
	store.mu.Lock()
	if store.remainingFailures > 0 {
		store.remainingFailures--
		store.mu.Unlock()
		return errors.New("injected Agent input status failure")
	}
	store.mu.Unlock()
	return store.productMemoryAgentInboxStore.MarkAgentInputConsumed(ctx, binding)
}

func newProductMemoryAgentInboxStore() *productMemoryAgentInboxStore {
	return &productMemoryAgentInboxStore{payloads: make(map[string]agentinbox.Payload)}
}

func (store *productMemoryAgentInboxStore) PutAgentInput(
	_ context.Context,
	payload agentinbox.Payload,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.payloads[payload.Binding.PayloadID]; exists {
		return errors.New("duplicate Agent input")
	}
	payload.Content = bytes.Clone(payload.Content)
	store.payloads[payload.Binding.PayloadID] = payload
	return nil
}

func (store *productMemoryAgentInboxStore) ReadAgentInput(
	_ context.Context,
	binding agentinbox.Binding,
) (agentinbox.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return agentinbox.Payload{}, errors.New("Agent input unavailable")
	}
	payload.Content = bytes.Clone(payload.Content)
	return payload, nil
}

func (store *productMemoryAgentInboxStore) ListPendingAgentInputs(
	_ context.Context,
	conversationID string,
	runID string,
	agentInstanceID string,
	claimGeneration int64,
) ([]agentinbox.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var payloads []agentinbox.Payload
	for _, payload := range store.payloads {
		binding := payload.Binding
		if payload.Status == agentinbox.StatusPending &&
			binding.ConversationID == conversationID && binding.RunID == runID &&
			binding.AgentInstanceID == agentInstanceID &&
			binding.ClaimGeneration == claimGeneration {
			payload.Content = bytes.Clone(payload.Content)
			payloads = append(payloads, payload)
		}
	}
	return payloads, nil
}

func (store *productMemoryAgentInboxStore) MarkAgentInputConsumed(
	_ context.Context,
	binding agentinbox.Binding,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return errors.New("Agent input unavailable")
	}
	payload.Status = agentinbox.StatusConsumed
	store.payloads[binding.PayloadID] = payload
	return nil
}

func (store *productMemoryAgentInboxStore) DeleteAgentInput(
	_ context.Context,
	binding agentinbox.Binding,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return errors.New("Agent input unavailable")
	}
	payload.Close()
	delete(store.payloads, binding.PayloadID)
	return nil
}

type productBlockingAttemptLoopAdapterFixture struct {
	entered chan struct{}
	release chan struct{}
}

type productAgentInputAttemptLoopAdapterFixture struct {
	admit           func(context.Context) error
	calls           int
	inputBatches    int
	terminalResults int
	zeroized        bool
}

func (*productAgentInputAttemptLoopAdapterFixture) AdapterType() string       { return "loom-native" }
func (*productAgentInputAttemptLoopAdapterFixture) RuntimeInstanceID() string { return "runtime-1" }
func (*productAgentInputAttemptLoopAdapterFixture) AcceptsAgentInputs() bool  { return true }

func (fixture *productAgentInputAttemptLoopAdapterFixture) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	fixture.calls++
	if request.AgentInputs == nil || fixture.admit == nil {
		return supervisor.AdapterResult{}, errors.New("Agent input source unavailable")
	}
	if err := fixture.admit(ctx); err != nil {
		return supervisor.AdapterResult{}, err
	}
	_, found := productAttemptLoopInvocationFromContext(ctx)
	if !found {
		return supervisor.AdapterResult{}, errors.New("Attempt invocation unavailable")
	}
	stepBatch, ok, err := request.AgentInputs.NextAgentInput(ctx, runtime.AgentInputCheckpoint{
		OutputDigest: strings.Repeat("8", 64),
	})
	if err != nil || !ok || len(stepBatch.Inputs) != 1 ||
		string(stepBatch.Inputs[0].Content) != "narrow the implementation boundary" {
		stepBatch.Close()
		return supervisor.AdapterResult{}, errors.Join(errors.New("Step input unavailable"), err)
	}
	fixture.inputBatches++
	stepContent := stepBatch.Inputs[0].Content
	stepBatch.Close()
	queueBatch, ok, err := request.AgentInputs.NextAgentInput(ctx, runtime.AgentInputCheckpoint{
		OutputDigest: strings.Repeat("9", 64),
	})
	if err != nil || !ok || len(queueBatch.Inputs) != 1 ||
		string(queueBatch.Inputs[0].Content) != "continue with the accepted tests" {
		queueBatch.Close()
		return supervisor.AdapterResult{}, errors.Join(errors.New("Queue input unavailable"), err)
	}
	fixture.inputBatches++
	queueContent := queueBatch.Inputs[0].Content
	queueBatch.Close()
	fixture.zeroized = allProductAgentInputBytesZero(stepContent) &&
		allProductAgentInputBytesZero(queueContent)
	fixture.terminalResults++
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		ExitCode: 0, DispatchAcknowledged: true, ResultAcknowledged: true,
	})
}

func (*productBlockingAttemptLoopAdapterFixture) AdapterType() string { return "loom-native" }

func (*productBlockingAttemptLoopAdapterFixture) RuntimeInstanceID() string { return "runtime-1" }

func (fixture *productBlockingAttemptLoopAdapterFixture) Execute(
	ctx context.Context,
	_ supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	close(fixture.entered)
	select {
	case <-fixture.release:
		return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
			ExitCode: 0, DispatchAcknowledged: true, ResultAcknowledged: true,
		})
	case <-ctx.Done():
		return supervisor.AdapterResult{}, ctx.Err()
	}
}

func (*productAgentInboxStoreFixture) PutAgentInput(context.Context, agentinbox.Payload) error {
	return nil
}
func (*productAgentInboxStoreFixture) ReadAgentInput(context.Context, agentinbox.Binding) (agentinbox.Payload, error) {
	return agentinbox.Payload{}, errors.New("not found")
}
func (*productAgentInboxStoreFixture) ListPendingAgentInputs(context.Context, string, string, string, int64) ([]agentinbox.Payload, error) {
	return nil, nil
}
func (*productAgentInboxStoreFixture) MarkAgentInputConsumed(context.Context, agentinbox.Binding) error {
	return nil
}
func (*productAgentInboxStoreFixture) DeleteAgentInput(context.Context, agentinbox.Binding) error {
	return nil
}

func productAgentInputPayload(
	binding work.AttemptLoopBinding,
	segmentID string,
	inputID string,
	order int64,
	mode agentinbox.Mode,
	targetStepID string,
	targetStepSequence int,
	content string,
) agentinbox.Payload {
	digest := sha256.Sum256([]byte(content))
	return agentinbox.Payload{
		Binding: agentinbox.Binding{
			PayloadID: "payload-" + inputID, InputID: inputID, Mode: mode,
			ContextScope:   agentinbox.ScopeAgentPrivate,
			ConversationID: binding.PayloadAuthority.ConversationID,
			SegmentID:      segmentID, AgentInstanceID: binding.PayloadAuthority.AgentInstanceID,
			WorkItemID:             binding.PayloadAuthority.WorkItemID,
			RunID:                  binding.PayloadAuthority.RunID,
			ClaimGeneration:        binding.PayloadAuthority.ClaimGeneration,
			RuntimeInstanceID:      binding.PayloadAuthority.RuntimeInstanceID,
			ExecutionBindingDigest: binding.PayloadAuthority.ExecutionBindingDigest,
			CapsuleDigest:          binding.PayloadAuthority.CapsuleDigest,
			OrderKey:               order, TargetStepID: targetStepID,
			TargetStepSequence: targetStepSequence, ContentType: "text/plain",
			ContentDigest: hex.EncodeToString(digest[:]),
		},
		Status: agentinbox.StatusPending, Content: []byte(content),
	}
}

func allProductAgentInputBytesZero(content []byte) bool {
	for _, value := range content {
		if value != 0 {
			return false
		}
	}
	return true
}

type productAgentInboxRetrievalStoreFixture struct{}

func (productAgentInboxRetrievalStoreFixture) RetrieveContextItem(
	context.Context,
	contextcapsule.RetrievalRequest,
) (contextcapsule.RetrievedItem, error) {
	return contextcapsule.RetrievedItem{}, errors.New("not found")
}

type productAgentInboxRetrievalAuditorFixture struct{}

func (productAgentInboxRetrievalAuditorFixture) RecordContextRetrieval(
	context.Context,
	contextcapsule.RetrievalAudit,
) error {
	return nil
}

func TestProductAttemptLoopRuntimeGovernsLocalToolDispatchAndDelivery(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	permissionAuthority, err := permissions.NewAuthority(
		journalStore,
		func() time.Time { return time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.DefineProfile(ctx, permissions.ProfileInput{
		ProfileID: "attempt-tool-auto", Mode: permissions.ModeAuto,
	}, "op-attempt-tool-profile", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.BindJob(
		ctx, run.WorkItemID(), "attempt-tool-auto", "op-attempt-tool-bind",
		"11111111-1111-4111-8111-111111111111",
	); err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	request.ContextRetriever = productAttemptLoopRetrieverFixture{
		content: []byte("unused-context-for-local-tool-attempt"),
	}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	executor := &productAttemptLoopExecutionFixture{
		loops: loops, binding: loopBinding,
		runOutputDigest: "sha256:" + strings.Repeat("8", 64),
	}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionAdapter, err := execution.NewAdapter(
		journalStore, evidenceStore, executor,
		productAttemptLoopResolver{root: t.TempDir()}, nil, nil,
		func() time.Time { return time.Date(2026, 8, 14, 10, 1, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := &productAttemptToolDiagnosticFixture{}
	hook, err := newBridgeExecutionHook(
		executionAdapter, nil, loops, payloadStore, diagnostics,
	)
	if err != nil {
		t.Fatal(err)
	}
	testCommand := "go test ./... -count=1"
	delegate := &productAttemptLoopToolAdapterFixture{
		hook: hook,
		call: permissions.ProposedCall{Tool: permissions.ToolBash, Command: testCommand},
	}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode() != 0 || executor.calls != 1 || !executor.dispatchObserved ||
		delegate.result.Verdict != permissions.VerdictAllow ||
		delegate.proof != attemptpayload.ProofHarnessFinalOutput {
		t.Fatalf("result=%#v executor=%#v tool=%#v", result, executor, delegate.result)
	}
	snapshot, err := loops.Snapshot(ctx, loopBinding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 || len(snapshot.Turns[0].Steps) != 1 ||
		len(snapshot.Turns[0].Steps[0].ToolCalls) != 1 ||
		!snapshot.Turns[0].Steps[0].ToolCalls[0].Dispatched ||
		snapshot.Turns[0].Steps[0].ToolCalls[0].ResultStatus != attemptpayload.FactDelivered ||
		len(snapshot.GovernedTestReports) != 1 ||
		snapshot.GovernedTestReports[0].Report.Outcome() != verification.TestOutcomePassed ||
		snapshot.GovernedTestReports[0].Report.Runner() != verification.TestRunnerGo {
		t.Fatalf("local tool loop snapshot=%#v", snapshot)
	}
	for _, event := range mustProductAttemptLoopEvents(t, journalStore) {
		if bytes.Contains(event.PayloadJSON, []byte(testCommand)) ||
			bytes.Contains(event.PayloadJSON, []byte("./...")) {
			t.Fatalf("Journal event %s persisted local tool arguments", event.Type)
		}
	}
	wantStages := []execution.ToolExecutionStage{
		execution.ToolStageAuthorization,
		execution.ToolStageSandboxPrepare,
		execution.ToolStageBindingValidation,
		execution.ToolStageDispatch,
		execution.ToolStageResultValidation,
		execution.ToolStageResultCommit,
		execution.ToolStagePayloadCommit,
		execution.ToolStageResultDelivery,
	}
	if len(diagnostics.records) != len(wantStages) {
		t.Fatalf("tool diagnostics=%#v", diagnostics.records)
	}
	for index, diagnostic := range diagnostics.records {
		if diagnostic.IncidentID != request.IncidentID ||
			diagnostic.ProviderID != executionBinding.ProviderID ||
			diagnostic.ProviderAccountID != executionBinding.ProviderAccountID ||
			diagnostic.ModelID != executionBinding.ModelID ||
			diagnostic.WorkItemID != run.WorkItemID() ||
			diagnostic.RunID != run.ID() ||
			diagnostic.AgentInstanceID != run.AgentInstanceID() ||
			diagnostic.ExecutionBindingDigest != executionBinding.BindingDigest ||
			diagnostic.CapsuleDigest != capsule.CapsuleDigest ||
			diagnostic.Execution.Stage != wantStages[index] ||
			diagnostic.Execution.Result != execution.ToolDiagnosticSucceeded {
			t.Fatalf("tool diagnostic[%d]=%#v", index, diagnostic)
		}
	}
}

func TestProductAttemptLoopRuntimeDoesNotReportOrdinaryBashAsTests(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	permissionAuthority, err := permissions.NewAuthority(
		journalStore,
		func() time.Time { return time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.DefineProfile(ctx, permissions.ProfileInput{
		ProfileID: "attempt-tool-auto", Mode: permissions.ModeAuto,
	}, "op-attempt-tool-profile", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.BindJob(
		ctx, run.WorkItemID(), "attempt-tool-auto", "op-attempt-tool-bind",
		"11111111-1111-4111-8111-111111111111",
	); err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	request.ContextRetriever = productAttemptLoopRetrieverFixture{
		content: []byte("unused-context-for-ordinary-bash"),
	}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	executor := &productAttemptLoopExecutionFixture{loops: loops, binding: loopBinding}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionAdapter, err := execution.NewAdapter(
		journalStore, evidenceStore, executor,
		productAttemptLoopResolver{root: t.TempDir()}, nil, nil,
		func() time.Time { return time.Date(2026, 8, 14, 10, 1, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := newBridgeExecutionHook(
		executionAdapter, nil, loops, payloadStore, &productAttemptToolDiagnosticFixture{},
	)
	if err != nil {
		t.Fatal(err)
	}
	delegate := &productAttemptLoopToolAdapterFixture{
		hook: hook,
		call: permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: "printf ordinary-command",
		},
	}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(ctx, request); err != nil {
		t.Fatal(err)
	}
	snapshot, err := loops.Snapshot(ctx, loopBinding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.GovernedTestReports) != 0 || executor.calls != 1 ||
		delegate.result.Verdict != permissions.VerdictAllow {
		t.Fatalf("ordinary Bash report/result = %#v / %#v", snapshot.GovernedTestReports, delegate.result)
	}
}

func TestProductAttemptLoopRuntimeEncryptsReadAndGrepContentUntilHarnessDelivery(t *testing.T) {
	for _, test := range []struct {
		name string
		call permissions.ProposedCall
	}{
		{
			name: "Read",
			call: permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
		},
		{
			name: "Grep",
			call: permissions.ProposedCall{
				Tool: permissions.ToolGrep, Path: "src/main.go", Pattern: "private",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			testProductAttemptLoopEncryptedReadOnlyContent(t, test.call)
		})
	}
}

func TestCodexHarnessMCPUsesProductAuthorityAndDeliversAfterFinalOutput(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore :=
		productAttemptLoopFixtureForAdapter(t, harnessadapter.CodexAdapterType)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	permissionAuthority, err := permissions.NewAuthority(
		journalStore,
		func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.DefineProfile(ctx, permissions.ProfileInput{
		ProfileID: "attempt-harness-auto", Mode: permissions.ModeAuto,
	}, "op-attempt-harness-profile", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.BindJob(
		ctx, run.WorkItemID(), "attempt-harness-auto", "op-attempt-harness-bind",
		"11111111-1111-4111-8111-111111111111",
	); err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	root := t.TempDir()
	request.WorkspacePath, request.HomePath, request.TempPath = root, root, root
	request.FrameSink = &productNativeAgentInputFrameSink{}
	request.ContextRetriever = productAttemptLoopRetrieverFixture{
		content: []byte("unused-context-for-harness-tools"),
	}
	capsuleValue := productAttemptLoopCapsuleValue(t, productAttemptLoopCodexProfile())
	if capsuleValue.Digest() != capsule.CapsuleDigest {
		t.Fatalf("Capsule digest drift = %s / %s", capsuleValue.Digest(), capsule.CapsuleDigest)
	}
	dispatchPayload, err := contextcapsule.RenderDispatchPayload(capsuleValue)
	if err != nil {
		t.Fatal(err)
	}
	request.Dispatch, err = bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: request.Dispatch.MessageID(), CorrelationID: request.Dispatch.CorrelationID(),
		WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch, EmittedAt: request.Dispatch.EmittedAt(),
		Payload: dispatchPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	executor := &productAttemptLoopExecutionFixture{
		loops: loops, binding: loopBinding,
		readContent: []byte("private governed workspace result"),
	}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionAdapter, err := execution.NewAdapter(
		journalStore, evidenceStore, executor,
		productAttemptLoopResolver{root: root}, nil, nil,
		func() time.Time { return time.Date(2026, 8, 15, 9, 1, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	toolDiagnostics := &productAttemptToolDiagnosticFixture{}
	hook, err := newBridgeExecutionHook(
		executionAdapter, nil, loops, payloadStore, toolDiagnostics,
	)
	if err != nil {
		t.Fatal(err)
	}
	recordingGateway := &productRecordingToolGateway{delegate: hook}
	processRunner := &productHarnessProcessRunnerFixture{hook: func(
		ctx context.Context,
		processRequest harnessadapter.HarnessProcessRequest,
	) error {
		calls := []map[string]any{
			{"name": "loom_read_file", "arguments": map[string]any{"path": "src/main.go"}},
			{"name": "loom_grep_files", "arguments": map[string]any{"path": "src", "pattern": "governed"}},
		}
		for index, call := range calls {
			payload, marshalErr := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": index + 1, "method": "tools/call", "params": call,
			})
			if marshalErr != nil {
				return marshalErr
			}
			httpRequest, requestErr := http.NewRequestWithContext(
				ctx, http.MethodPost, processRequest.ContextMCP.URL, bytes.NewReader(payload),
			)
			if requestErr != nil {
				return requestErr
			}
			httpRequest.Header.Set("Content-Type", "application/json")
			httpRequest.Header.Set("Authorization", "Bearer "+processRequest.ContextMCP.Token)
			response, callErr := http.DefaultClient.Do(httpRequest)
			if callErr != nil {
				return callErr
			}
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr != nil || response.StatusCode != http.StatusOK ||
				!bytes.Contains(body, []byte(`"verdict":"allow"`)) ||
				!bytes.Contains(body, []byte("private governed workspace result")) {
				return fmt.Errorf("Harness MCP status=%d body=%s read=%v", response.StatusCode, body, readErr)
			}
		}
		snapshot, snapshotErr := loops.Snapshot(ctx, loopBinding)
		if snapshotErr != nil {
			return snapshotErr
		}
		if len(snapshot.Turns) != 1 || len(snapshot.Turns[0].Steps) != 1 ||
			len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 {
			return fmt.Errorf("pending Harness calls = %#v", snapshot)
		}
		for _, call := range snapshot.Turns[0].Steps[0].ToolCalls {
			if call.ResultStatus != attemptpayload.FactAccepted {
				return fmt.Errorf("Harness call acknowledged before final output: %#v", call)
			}
		}
		return nil
	}}
	delegate, err := harnessadapter.NewCodexAdapter(harnessadapter.CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex",
		CredentialAccess:  &productHarnessCredentialFixture{},
		Diagnostics:       &productHarnessAgentDiagnosticFixture{}, Runner: processRunner,
		Now:            func() time.Time { return time.Date(2026, 8, 15, 9, 2, 0, 0, time.UTC) },
		MaxOutputBytes: 4096, ContextConformance: func(string, string) bool { return true },
		ToolGateway: recordingGateway, ToolConformance: func(string, string) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Execute(ctx, request)
	if err != nil {
		snapshot, _ := loops.Snapshot(ctx, loopBinding)
		t.Fatalf("Harness Execute() error=%v snapshot=%#v runner=%v gateway=%v tool_result=%#v", err, snapshot, processRunner.err, recordingGateway.err, recordingGateway.result)
	}
	if result.ExitCode() != 0 || executor.readCalls != 1 || executor.grepCalls != 1 {
		t.Fatalf("result=%#v executor=%#v runner=%v gateway=%v envelope=%#v binding=%#v invocation=%#v/%t diagnostics=%#v", result, executor, processRunner.err, recordingGateway.err, recordingGateway.envelope, recordingGateway.binding, recordingGateway.invocation, recordingGateway.invocationFound, toolDiagnostics.records)
	}
	snapshot, err := loops.Snapshot(ctx, loopBinding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 || len(snapshot.Turns[0].Steps) != 1 ||
		len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 {
		t.Fatalf("Harness tool snapshot = %#v", snapshot)
	}
	for _, call := range snapshot.Turns[0].Steps[0].ToolCalls {
		if call.ResultStatus != attemptpayload.FactDelivered {
			t.Fatalf("Harness call not delivered after final output: %#v", call)
		}
	}
	for _, event := range mustProductAttemptLoopEvents(t, journalStore) {
		for _, private := range [][]byte{
			[]byte("src/main.go"), []byte(`"pattern":"governed"`),
			[]byte("private governed workspace result"),
		} {
			if bytes.Contains(event.PayloadJSON, private) {
				t.Fatalf("Journal event %s persisted private Harness data", event.Type)
			}
		}
	}
}

func testProductAttemptLoopEncryptedReadOnlyContent(
	t *testing.T,
	call permissions.ProposedCall,
) {
	t.Helper()
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	permissionAuthority, err := permissions.NewAuthority(
		journalStore,
		func() time.Time { return time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.DefineProfile(ctx, permissions.ProfileInput{
		ProfileID: "attempt-read-default", Mode: permissions.ModeDefault,
	}, "op-attempt-read-profile", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.BindJob(
		ctx, run.WorkItemID(), "attempt-read-default", "op-attempt-read-bind",
		"11111111-1111-4111-8111-111111111111",
	); err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: []byte("unused")}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	privateContent := []byte("private source returned only to the bound Pi child\n")
	executor := &productAttemptLoopExecutionFixture{
		loops: loops, binding: loopBinding, readContent: privateContent,
	}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionAdapter, err := execution.NewAdapter(
		journalStore, evidenceStore, executor,
		productAttemptLoopResolver{root: t.TempDir()}, nil, nil,
		func() time.Time { return time.Date(2026, 8, 14, 11, 1, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := newBridgeExecutionHook(
		executionAdapter, nil, loops, payloadStore, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	delegate := &productAttemptLoopToolAdapterFixture{
		hook: hook, call: call,
	}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(ctx, request); err != nil {
		t.Fatal(err)
	}
	defer clearProductBytes(delegate.content)
	if executor.readCalls+executor.grepCalls != 1 || !executor.dispatchObserved ||
		!bytes.Equal(delegate.content, privateContent) ||
		delegate.result.ContentDigest != delegate.result.OutputDigest ||
		delegate.proof != attemptpayload.ProofHarnessFinalOutput {
		t.Fatalf("read-only executor=%#v result=%#v content=%q", executor, delegate.result, delegate.content)
	}
	for _, event := range mustProductAttemptLoopEvents(t, journalStore) {
		if bytes.Contains(event.PayloadJSON, privateContent) ||
			bytes.Contains(event.PayloadJSON, []byte(call.Path)) ||
			(call.Pattern != "" && bytes.Contains(event.PayloadJSON, []byte(call.Pattern))) {
			t.Fatalf("Journal event %s persisted read-only content/arguments", event.Type)
		}
	}
	payloadStore.mu.Lock()
	defer payloadStore.mu.Unlock()
	if len(payloadStore.payloads) != 1 {
		t.Fatalf("encrypted payload count=%d", len(payloadStore.payloads))
	}
	for _, payload := range payloadStore.payloads {
		if payload.Status != attemptpayload.StatusDelivered ||
			payload.Binding.ContentType != attemptpayload.ContentTypeTextUTF8 ||
			!bytes.Equal(payload.Content, privateContent) {
			t.Fatalf("read-only payload=%#v", payload)
		}
	}
}

func TestProductAttemptLoopPersistsRemoteResultBeforeExecutionTerminal(t *testing.T) {
	ctx := context.Background()
	runs, run, executionBinding, capsule, payloadStore, journalStore := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	permissionAuthority, err := permissions.NewAuthority(
		journalStore,
		func() time.Time { return time.Date(2026, 8, 14, 12, 20, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.DefineProfile(ctx, permissions.ProfileInput{
		ProfileID: "attempt-web-default", Mode: permissions.ModeDefault,
	}, "op-attempt-web-profile", "11111111-1111-4111-8111-111111111111"); err != nil {
		t.Fatal(err)
	}
	if _, err := permissionAuthority.BindJob(
		ctx, run.WorkItemID(), "attempt-web-default", "op-attempt-web-bind",
		"11111111-1111-4111-8111-111111111111",
	); err != nil {
		t.Fatal(err)
	}
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	request.ContextRetriever = productAttemptLoopRetrieverFixture{content: []byte("unused")}
	loopBinding, _, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	privateContent := []byte("bounded private WebSearch result\n")
	remote := &productAttemptRemoteExecutorFixture{
		loops: loops, binding: loopBinding, content: privateContent,
	}
	evidenceRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	executionAdapter, err := execution.NewAdapter(
		journalStore, evidenceStore, &productAttemptLoopExecutionFixture{},
		productAttemptLoopResolver{root: t.TempDir()}, nil, nil,
		func() time.Time { return time.Date(2026, 8, 14, 12, 21, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	executionAdapter = executionAdapter.WithRemoteToolExecutor(remote)
	hook, err := newBridgeExecutionHook(executionAdapter, nil, loops, payloadStore, nil)
	if err != nil {
		t.Fatal(err)
	}
	call := permissions.ProposedCall{
		Tool: permissions.ToolWebSearch, Path: "Loom governance status",
	}
	delegate := &productAttemptLoopToolAdapterFixture{hook: hook, call: call}
	adapter, err := newProductAttemptLoopRuntimeAdapter(delegate, loops, payloadStore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(ctx, request); err != nil {
		t.Fatal(err)
	}
	defer clearProductBytes(delegate.content)
	if remote.calls != 1 || !remote.dispatchObserved ||
		!bytes.Equal(delegate.content, privateContent) ||
		delegate.proof != attemptpayload.ProofHarnessFinalOutput {
		t.Fatalf("remote=%#v content=%q proof=%q", remote, delegate.content, delegate.proof)
	}
	events := mustProductAttemptLoopEvents(t, journalStore)
	acceptedIndex, completedIndex := -1, -1
	for index, event := range events {
		if bytes.Contains(event.PayloadJSON, privateContent) ||
			bytes.Contains(event.PayloadJSON, []byte(call.Path)) {
			t.Fatalf("Journal event %s persisted Web content/query", event.Type)
		}
		if event.Type == "ToolResultAccepted" {
			acceptedIndex = index
		}
		if event.Type == execution.EventToolCompleted {
			completedIndex = index
		}
	}
	if acceptedIndex < 0 || completedIndex < 0 || acceptedIndex >= completedIndex {
		t.Fatalf("result ordering accepted=%d completed=%d", acceptedIndex, completedIndex)
	}
	payloadStore.mu.Lock()
	defer payloadStore.mu.Unlock()
	if len(payloadStore.payloads) != 1 {
		t.Fatalf("encrypted remote payload count=%d", len(payloadStore.payloads))
	}
	for _, payload := range payloadStore.payloads {
		if payload.Status != attemptpayload.StatusDelivered ||
			payload.Binding.ContentType != attemptpayload.ContentTypeTextUTF8 ||
			!bytes.Equal(payload.Content, privateContent) {
			t.Fatalf("remote payload=%#v", payload)
		}
	}
}

type productAttemptToolDiagnosticFixture struct {
	records []productAttemptToolDiagnostic
}

func (fixture *productAttemptToolDiagnosticFixture) RecordAttemptToolDiagnostic(
	_ context.Context,
	diagnostic productAttemptToolDiagnostic,
) error {
	fixture.records = append(fixture.records, diagnostic)
	return nil
}

type productAttemptLoopResolver struct{ root string }

func (resolver productAttemptLoopResolver) Resolve(context.Context, string) (string, error) {
	return resolver.root, nil
}

type productAttemptLoopExecutionFixture struct {
	loops            *work.AttemptLoopAuthority
	binding          work.AttemptLoopBinding
	calls            int
	dispatchObserved bool
	readContent      []byte
	readCalls        int
	grepCalls        int
	runOutputDigest  string
}

type productAttemptRemoteExecutorFixture struct {
	loops            *work.AttemptLoopAuthority
	binding          work.AttemptLoopBinding
	content          []byte
	calls            int
	dispatchObserved bool
}

func (*productAttemptRemoteExecutorFixture) AllowedRemoteTools() []permissions.ToolKind {
	return []permissions.ToolKind{permissions.ToolWebSearch}
}

func (*productAttemptRemoteExecutorFixture) ValidateProposal(
	call permissions.ProposedCall,
) error {
	if call.Tool != permissions.ToolWebSearch || call.Path == "" || call.Command != "" {
		return errors.New("invalid remote proposal")
	}
	return nil
}

func (fixture *productAttemptRemoteExecutorFixture) ExecuteProposalContent(
	ctx context.Context,
	_ permissions.ProposedCall,
) ([]byte, error) {
	fixture.calls++
	snapshot, err := fixture.loops.Snapshot(ctx, fixture.binding)
	if err != nil {
		return nil, err
	}
	fixture.dispatchObserved = len(snapshot.Turns) == 1 &&
		len(snapshot.Turns[0].Steps) == 1 &&
		len(snapshot.Turns[0].Steps[0].ToolCalls) == 1 &&
		snapshot.Turns[0].Steps[0].ToolCalls[0].Dispatched
	if !fixture.dispatchObserved {
		return nil, errors.New("remote call preceded ToolDispatchCommitted")
	}
	return bytes.Clone(fixture.content), nil
}

func (fixture *productAttemptLoopExecutionFixture) Read(
	ctx context.Context,
	_ execution.ReadRequest,
) (execution.ReadResult, error) {
	fixture.readCalls++
	snapshot, err := fixture.loops.Snapshot(ctx, fixture.binding)
	if err != nil {
		return execution.ReadResult{}, err
	}
	fixture.dispatchObserved = productAttemptLastToolCallDispatched(snapshot)
	if !fixture.dispatchObserved {
		return execution.ReadResult{}, errors.New("read preceded ToolDispatchCommitted")
	}
	content := bytes.Clone(fixture.readContent)
	return execution.ReadResult{
		Content: content, ContentDigest: "sha256:" + productJourneySHA256Hex(content),
		OutputDigest: "sha256:" + productJourneySHA256Hex(content), DurationMS: 2,
	}, nil
}

func (fixture *productAttemptLoopExecutionFixture) Grep(
	ctx context.Context,
	_ execution.GrepRequest,
) (execution.GrepResult, error) {
	fixture.grepCalls++
	snapshot, err := fixture.loops.Snapshot(ctx, fixture.binding)
	if err != nil {
		return execution.GrepResult{}, err
	}
	fixture.dispatchObserved = productAttemptLastToolCallDispatched(snapshot)
	if !fixture.dispatchObserved {
		return execution.GrepResult{}, errors.New("grep preceded ToolDispatchCommitted")
	}
	content := bytes.Clone(fixture.readContent)
	return execution.GrepResult{
		Content: content, ContentDigest: "sha256:" + productJourneySHA256Hex(content),
		OutputDigest: "sha256:" + productJourneySHA256Hex(content), DurationMS: 2,
	}, nil
}

func productAttemptLastToolCallDispatched(snapshot work.AttemptLoopSnapshot) bool {
	if len(snapshot.Turns) != 1 || len(snapshot.Turns[0].Steps) != 1 {
		return false
	}
	calls := snapshot.Turns[0].Steps[0].ToolCalls
	return len(calls) > 0 && calls[len(calls)-1].Dispatched
}

func (*productAttemptLoopExecutionFixture) Edit(
	context.Context,
	execution.EditRequest,
) (execution.EditResult, error) {
	return execution.EditResult{}, errors.New("unexpected edit")
}

func (fixture *productAttemptLoopExecutionFixture) Run(
	ctx context.Context,
	_ execution.RunRequest,
) (execution.RunResult, error) {
	fixture.calls++
	snapshot, err := fixture.loops.Snapshot(ctx, fixture.binding)
	if err != nil {
		return execution.RunResult{}, err
	}
	fixture.dispatchObserved = len(snapshot.Turns) == 1 &&
		len(snapshot.Turns[0].Steps) == 1 &&
		len(snapshot.Turns[0].Steps[0].ToolCalls) == 1 &&
		snapshot.Turns[0].Steps[0].ToolCalls[0].Dispatched
	if !fixture.dispatchObserved {
		return execution.RunResult{}, errors.New("side effect preceded ToolDispatchCommitted")
	}
	outputDigest := fixture.runOutputDigest
	if outputDigest == "" {
		outputDigest = strings.Repeat("8", 64)
	}
	return execution.RunResult{
		ExitCode: 0, OutputDigest: outputDigest,
		ChangedFilesDigest: strings.Repeat("9", 64), DurationMS: 4,
	}, nil
}

type productAttemptLoopToolAdapterFixture struct {
	hook    *bridgeExecutionHook
	result  piadapter.ToolCallResult
	proof   attemptpayload.DeliveryProof
	call    permissions.ProposedCall
	content []byte
}

type productHarnessCredentialFixture struct{}

func (*productHarnessCredentialFixture) UseCredential(
	ctx context.Context,
	_ runtime.FrozenExecutionBinding,
	use func(context.Context, []byte) error,
) error {
	secret := []byte("private-provider-key")
	defer clearProductBytes(secret)
	return use(ctx, secret)
}

type productHarnessAgentDiagnosticFixture struct{}

func (*productHarnessAgentDiagnosticFixture) RecordAgentAttemptDiagnostic(
	context.Context,
	nativeadapter.AgentAttemptDiagnostic,
) error {
	return nil
}

type productHarnessProcessRunnerFixture struct {
	hook func(context.Context, harnessadapter.HarnessProcessRequest) error
	err  error
}

type productRecordingToolGateway struct {
	delegate        *bridgeExecutionHook
	err             error
	envelope        runtime.ToolCallEnvelope
	binding         runtime.ToolCallBinding
	result          runtime.ToolCallResult
	invocation      productAttemptLoopInvocation
	invocationFound bool
}

func (gateway *productRecordingToolGateway) AllowedToolCalls() []permissions.ToolKind {
	return gateway.delegate.AllowedToolCalls()
}

func (gateway *productRecordingToolGateway) ExecuteToolCall(
	ctx context.Context,
	envelope runtime.ToolCallEnvelope,
	binding runtime.ToolCallBinding,
) (runtime.ToolCallResult, error) {
	gateway.envelope = envelope
	gateway.binding = binding
	gateway.invocation, gateway.invocationFound = productAttemptLoopInvocationFromContext(ctx)
	result, err := gateway.delegate.ExecuteToolCall(ctx, envelope, binding)
	gateway.result = result
	gateway.err = err
	return result, err
}

func (gateway *productRecordingToolGateway) ReadToolCallResultContent(
	ctx context.Context,
	binding runtime.ToolCallBinding,
	result runtime.ToolCallResult,
) ([]byte, error) {
	return gateway.delegate.ReadToolCallResultContent(ctx, binding, result)
}

func (gateway *productRecordingToolGateway) AcknowledgeToolCallResultWithProof(
	ctx context.Context,
	binding runtime.ToolCallBinding,
	result runtime.ToolCallResult,
	proof attemptpayload.DeliveryProof,
) error {
	return gateway.delegate.AcknowledgeToolCallResultWithProof(ctx, binding, result, proof)
}

func (runner *productHarnessProcessRunnerFixture) RunHarness(
	ctx context.Context,
	request harnessadapter.HarnessProcessRequest,
	_ []byte,
) (harnessadapter.HarnessProcessResult, error) {
	if runner == nil || runner.hook == nil {
		return harnessadapter.HarnessProcessResult{}, errors.New("missing Harness process hook")
	}
	if err := runner.hook(ctx, request); err != nil {
		runner.err = err
		return harnessadapter.HarnessProcessResult{}, err
	}
	return harnessadapter.HarnessProcessResult{Content: "validated Harness final output"}, nil
}

func (*productAttemptLoopToolAdapterFixture) AdapterType() string { return "loom-native" }

func (*productAttemptLoopToolAdapterFixture) RuntimeInstanceID() string { return "runtime-1" }

func (adapter *productAttemptLoopToolAdapterFixture) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	binding := piadapter.ToolCallBinding{
		ConversationID: request.ContextCapsule.ConversationID,
		WorkItemID:     request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:        request.Binding.ClaimGeneration,
		RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
		AgentInstanceID:        request.Binding.SenderAgentInstanceID,
		ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
		CapsuleDigest:          request.ContextCapsule.CapsuleDigest,
		ClaimID:                request.ClaimID, IncidentID: request.IncidentID,
		JourneyID: request.Dispatch.CorrelationID(),
	}
	call := adapter.call
	if call.Tool == "" {
		call = permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: "printf private-attempt-tool",
		}
	}
	result, err := adapter.hook.ExecuteToolCall(
		ctx,
		piadapter.ToolCallEnvelope{
			JobID: request.Binding.WorkItemID,
			Call:  call,
		},
		binding,
	)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	adapter.result = result
	if productAttemptContentTool(call.Tool) {
		reader, ok := any(adapter.hook).(piadapter.ToolCallResultContentReader)
		if !ok {
			return supervisor.AdapterResult{}, errors.New("tool result content reader unavailable")
		}
		adapter.content, err = reader.ReadToolCallResultContent(ctx, binding, result)
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	acknowledger, ok := any(adapter.hook).(piadapter.ToolCallResultProofAcknowledger)
	if !ok {
		return supervisor.AdapterResult{}, errors.New("tool result acknowledger unavailable")
	}
	adapter.proof = attemptpayload.ProofHarnessFinalOutput
	if err := acknowledger.AcknowledgeToolCallResultWithProof(
		ctx, binding, result, adapter.proof,
	); err != nil {
		return supervisor.AdapterResult{}, err
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		ExitCode: 0, DispatchAcknowledged: true, ResultAcknowledged: true,
	})
}

func mustProductAttemptLoopEvents(t *testing.T, store *journal.Store) []journal.Event {
	t.Helper()
	events, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return events
}

type productAttemptLoopAdapterFixture struct {
	calls                        int
	contextCalls                 int
	adapterType                  string
	deferContextAcknowledgement  bool
	beforeContextAcknowledgement func(context.Context, productAttemptLoopInvocation) error
	claimID                      string
	incidentID                   string
	invocation                   productAttemptLoopInvocation
	invocationFound              bool
}

func (adapter *productAttemptLoopAdapterFixture) AdapterType() string {
	if adapter.adapterType != "" {
		return adapter.adapterType
	}
	return "loom-native"
}

func (*productAttemptLoopAdapterFixture) RuntimeInstanceID() string { return "runtime-1" }

func (adapter *productAttemptLoopAdapterFixture) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	adapter.calls++
	adapter.claimID = request.ClaimID
	adapter.incidentID = request.IncidentID
	adapter.invocation, adapter.invocationFound = productAttemptLoopInvocationFromContext(ctx)
	contextCalls := adapter.contextCalls
	if contextCalls == 0 {
		contextCalls = 1
	}
	pending := make([]attemptpayload.Payload, 0, contextCalls)
	defer func() {
		for index := range pending {
			pending[index].Close()
		}
	}()
	for sequence := 1; sequence <= contextCalls; sequence++ {
		itemID := "workspace-snapshot"
		artifactRef := "artifact-workspace"
		if adapter.deferContextAcknowledgement {
			itemID += "-" + strconv.Itoa(sequence)
			artifactRef += "-" + strconv.Itoa(sequence)
		}
		proposal := contextcapsule.RetrievalProposal{
			ItemID: itemID, ContentDigest: strings.Repeat(strconv.Itoa(sequence+6), 64),
			ArtifactRef: artifactRef,
		}
		payload, err := request.ContextDelivery.Prepare(
			ctx,
			proposal,
			contextcapsule.DeliveryRequest{
				Sequence: int64(sequence), ContentType: "application/json",
			},
			func(_ contextcapsule.RetrievalProposal, item contextcapsule.RetrievedItem) ([]byte, error) {
				return json.Marshal(struct {
					Content string `json:"content"`
				}{Content: string(item.Content)})
			},
		)
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		if adapter.deferContextAcknowledgement {
			pending = append(pending, payload)
			continue
		}
		if err := request.ContextDelivery.Acknowledge(
			ctx, payload.Binding, attemptpayload.ProofHarnessFinalOutput,
		); err != nil {
			payload.Close()
			return supervisor.AdapterResult{}, err
		}
		payload.Close()
	}
	if adapter.beforeContextAcknowledgement != nil {
		if err := adapter.beforeContextAcknowledgement(ctx, adapter.invocation); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	for index := range pending {
		if err := request.ContextDelivery.Acknowledge(
			ctx, pending[index].Binding, attemptpayload.ProofHarnessFinalOutput,
		); err != nil {
			return supervisor.AdapterResult{}, err
		}
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		ExitCode: 0, DispatchAcknowledged: true, ResultAcknowledged: true,
	})
}

type productAttemptLoopRetrieverFixture struct{ content []byte }

func (fixture productAttemptLoopRetrieverFixture) Retrieve(
	_ context.Context,
	proposal contextcapsule.RetrievalProposal,
) (contextcapsule.RetrievedItem, error) {
	return contextcapsule.RetrievedItem{
		ItemID: proposal.ItemID, Kind: contextcapsule.KindWorkspaceSnapshot,
		Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
		Priority: contextcapsule.PriorityWorkspace, TokenCount: 8,
		Content: bytes.Clone(fixture.content), ContentDigest: proposal.ContentDigest,
		SourceType: contextcapsule.SourceObservation, SourceRef: "workspace:fixture",
		ArtifactRef: proposal.ArtifactRef,
	}, nil
}

type productAttemptLoopPayloadStore struct {
	mu       sync.Mutex
	payloads map[attemptpayload.Binding]attemptpayload.Payload
}

func newProductAttemptLoopPayloadStore() *productAttemptLoopPayloadStore {
	return &productAttemptLoopPayloadStore{payloads: make(map[attemptpayload.Binding]attemptpayload.Payload)}
}

func (store *productAttemptLoopPayloadStore) PutAttemptPayload(
	_ context.Context,
	payload attemptpayload.Payload,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.payloads[payload.Binding]; exists {
		return errors.New("duplicate Attempt payload")
	}
	store.payloads[payload.Binding] = cloneProductAttemptLoopPayload(payload)
	return nil
}

func (store *productAttemptLoopPayloadStore) ReadAttemptPayload(
	_ context.Context,
	binding attemptpayload.Binding,
) (attemptpayload.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, ok := store.payloads[binding]
	if !ok {
		return attemptpayload.Payload{}, attemptpayload.ErrPayloadNotFound
	}
	return cloneProductAttemptLoopPayload(payload), nil
}

func (store *productAttemptLoopPayloadStore) ListPendingAttemptPayloads(
	_ context.Context,
	scope attemptpayload.Scope,
) ([]attemptpayload.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	values := make([]attemptpayload.Payload, 0, len(store.payloads))
	for _, payload := range store.payloads {
		if payload.Binding.Scope == scope && payload.Status == attemptpayload.StatusPending {
			values = append(values, cloneProductAttemptLoopPayload(payload))
		}
	}
	return values, nil
}

func (store *productAttemptLoopPayloadStore) MarkAttemptPayloadDelivered(
	_ context.Context,
	binding attemptpayload.Binding,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, ok := store.payloads[binding]
	if !ok {
		return attemptpayload.ErrPayloadNotFound
	}
	payload.Status = attemptpayload.StatusDelivered
	store.payloads[binding] = payload
	return nil
}

func cloneProductAttemptLoopPayload(payload attemptpayload.Payload) attemptpayload.Payload {
	payload.Content = bytes.Clone(payload.Content)
	return payload
}

func productAttemptLoopFixture(
	t *testing.T,
) (*work.Authority, work.RunRecord, runtime.FrozenExecutionBinding, contextcapsule.AuthorityRecord, *productAttemptLoopPayloadStore, *journal.Store) {
	return productAttemptLoopFixtureForAdapter(t, nativeadapter.LoomNativeAgentAdapterType)
}

func productAttemptLoopFixtureForAdapter(
	t *testing.T,
	adapterType string,
) (*work.Authority, work.RunRecord, runtime.FrozenExecutionBinding, contextcapsule.AuthorityRecord, *productAttemptLoopPayloadStore, *journal.Store) {
	t.Helper()
	_, statePath := productDaemonFailureState(t)
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	appendProductExecutionRuntimeFixture(t, database)
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	journalStore := journal.NewStore(database)
	runs, err := work.NewAuthority(
		journalStore, func() time.Time { return now },
		bytes.NewReader(bytes.Repeat([]byte{0x52}, 4096)),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := runs.InitializeRunIdentityIndex(ctx); err != nil {
		t.Fatal(err)
	}
	profile := productAttemptLoopProfile("account.primary")
	instance := productAttemptLoopInstance()
	if adapterType == harnessadapter.CodexAdapterType {
		profile = productAttemptLoopCodexProfile()
		instance.AdapterType = harnessadapter.CodexAdapterType
		instance.DisplayName = "Codex"
		instance.ObservedCapabilities = append(
			[]string(nil), profile.RequiredCapabilities...,
		)
	}
	executionBinding, err := runtime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := runs.CreateAndAssign(ctx, work.WorkItemAssignmentInput{
		WorkItemID: "work-attempt-loop", Title: "Govern one Context retrieval",
		RunID: "run-attempt-loop", AgentInstanceID: "agent-main",
		ExecutionBinding: executionBinding,
		CorrelationID:    "11111111-1111-4111-8111-111111111111",
	}); err != nil {
		t.Fatal(err)
	}
	_, claimed, err := runs.Claim(ctx, work.RunClaimInput{
		WorkItemID: "work-attempt-loop", RunID: "run-attempt-loop",
		RuntimeInstanceID: "runtime-1", AgentInstanceID: "agent-main",
		PrepareLeaseDuration: time.Minute,
		CorrelationID:        "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, running, err := runs.Start(ctx, work.RunGenerationInput{
		WorkItemID: claimed.WorkItemID(), RunID: claimed.ID(), ClaimID: claimed.ClaimID(),
		ClaimGeneration: claimed.ClaimGeneration(), RuntimeInstanceID: claimed.RuntimeInstanceID(),
		AgentInstanceID: claimed.AgentInstanceID(),
		CorrelationID:   "11111111-1111-4111-8111-111111111111",
	})
	if err != nil {
		t.Fatal(err)
	}
	capsule := productAttemptLoopCapsuleValue(t, profile)
	return runs, running, executionBinding, capsule.AuthorityRecord(), newProductAttemptLoopPayloadStore(), journalStore
}

func productAttemptLoopCapsule(
	t *testing.T,
	profile runtime.RuntimeProfile,
) contextcapsule.AuthorityRecord {
	t.Helper()
	return productAttemptLoopCapsuleValue(t, profile).AuthorityRecord()
}

func productAttemptLoopCapsuleValue(
	t *testing.T,
	profile runtime.RuntimeProfile,
) contextcapsule.RoleContextCapsule {
	t.Helper()
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "mission-team-loop", TeamID: "team-loop",
			AgentID: "agent-main", RoleID: "main", ProviderID: profile.ProviderID,
			ProviderAccountID: profile.ProviderAccountID, ModelID: profile.ModelID,
			AuthMode: string(profile.AuthMode), ContextAdapterID: "context:" + profile.AdapterType + ":v1",
			DisclosurePolicyID: "loom.local-team-disclosure", DisclosurePolicyVersion: 1,
			TokenBudget: 4, ArtifactRefs: []string{"artifact-workspace"},
		},
		[]contextcapsule.ItemInput{
			{
				ItemID: "mission-objective", Kind: contextcapsule.KindConversationGoal,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, TokenCount: 2, Required: true,
				Content: []byte("Govern retrieval"), SourceType: contextcapsule.SourceAuthority,
				SourceRef: "mission:team-loop",
			},
			{
				ItemID: "workspace-snapshot", Kind: contextcapsule.KindWorkspaceSnapshot,
				Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
				Priority: contextcapsule.PriorityWorkspace, TokenCount: 8,
				Content:    []byte("context-content-that-must-not-enter-the-journal"),
				SourceType: contextcapsule.SourceObservation, SourceRef: "workspace:fixture",
				ArtifactRef: "artifact-workspace",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return capsule
}

func productAttemptLoopProfile(accountID string) runtime.RuntimeProfile {
	credentialAccountID := strings.ReplaceAll(accountID, ".", "-")
	return runtime.RuntimeProfile{
		ID: "profile-" + accountID, AdapterType: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: accountID, ModelID: "deepseek-chat", AuthMode: runtime.AuthBrokered,
		EndpointFingerprint: nativeadapter.DeepSeekAgentEndpointFingerprint,
		CredentialReference: "credential-ref-" + credentialAccountID,
		CredentialRevision:  4, RequiredCapabilities: []string{runtime.CapabilityContextRetrieval},
		Timeout: 30 * time.Second,
	}
}

func productAttemptLoopCodexProfile() runtime.RuntimeProfile {
	profile := productAttemptLoopProfile("openai.primary")
	profile.ID = "profile-openai-codex"
	profile.AdapterType = harnessadapter.CodexAdapterType
	profile.ProviderID = harnessadapter.CodexProviderID
	profile.ProviderAccountID = "openai.primary"
	profile.ModelID = harnessadapter.CodexModelID
	profile.EndpointFingerprint = harnessadapter.CodexEndpointFingerprint
	profile.CredentialReference = "credential-ref-openai-primary"
	profile.ReasoningEffort = "high"
	profile.RequiredCapabilities = []string{
		runtime.CapabilityContextRetrieval,
		runtime.CapabilityGovernedToolLoop,
		"reasoning_effort",
		"workspace_edit",
	}
	return profile
}

func productAttemptLoopInstance() runtime.RuntimeInstance {
	return runtime.RuntimeInstance{
		ID: "runtime-1", DeviceID: "device-1", AdapterType: "loom-native",
		DisplayName: "Loom Native", ExecutableVersion: "v1", Status: runtime.RuntimeOnline,
		ObservedCapabilities: []string{runtime.CapabilityContextRetrieval}, Capacity: 1,
	}
}

func productAttemptLoopRequest(
	t *testing.T,
	run work.RunRecord,
	executionBinding runtime.FrozenExecutionBinding,
	capsule contextcapsule.AuthorityRecord,
) supervisor.AdapterRequest {
	t.Helper()
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: "22222222-2222-4222-8222-222222222222", CorrelationID: "11111111-1111-4111-8111-111111111111",
		WorkItemID: run.WorkItemID(), RunID: run.ID(), ClaimGeneration: run.ClaimGeneration(),
		RuntimeInstanceID: run.RuntimeInstanceID(), SenderAgentInstanceID: run.AgentInstanceID(),
		Sequence: 1, Type: bridgev1.MessageDispatch, EmittedAt: time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC),
		Payload: []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"private user prompt"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: run.WorkItemID(), RunID: run.ID(), ClaimGeneration: run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(), SenderAgentInstanceID: run.AgentInstanceID(),
		},
		ClaimID: run.ClaimID(), IncidentID: dispatch.CorrelationID(),
		ExecutionBinding: executionBinding, Dispatch: dispatch, ContextCapsule: capsule,
		RouteSegment: productAttemptLoopRouteSegment(t, run, executionBinding, capsule),
	}
}

func productAttemptLoopRouteSegment(
	t *testing.T,
	run work.RunRecord,
	executionBinding runtime.FrozenExecutionBinding,
	capsule contextcapsule.AuthorityRecord,
) contextcapsule.RouteSegmentBinding {
	t.Helper()
	segment, err := contextcapsule.NewRouteSegmentBinding(
		contextcapsule.RouteSegmentBindingInput{
			SegmentID: "segment-" + run.ID(), ConversationID: capsule.ConversationID,
			TeamID: capsule.TeamID, AgentID: capsule.AgentID, RoleID: capsule.RoleID,
			AttemptNumber: 1, CapsuleDigest: capsule.CapsuleDigest,
			ExecutionBindingDigest: executionBinding.BindingDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return segment
}

func equalProductAttemptLoopStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
