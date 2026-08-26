package work

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/verification"
)

func TestAttemptLoopAuthorityAllowsFailedStepWithUndeliveredToolCall(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xd3)
	assignmentInput := assignment("work-loop-fail", "run-loop-fail")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.loop", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 7,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(ctx, claim("work-loop-fail", "run-loop-fail", "runtime-a"))
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	binding := attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding)
	budget := AttemptLoopBudget{
		MaxTurns: 2, MaxStepsPerTurn: 3, MaxToolCalls: 4,
		MaxParallelToolCalls: 1, MaxResultBytes: 32_768,
		ToolTimeoutMillis: 15_000,
	}
	if _, err := loops.StartTurn(ctx, binding, budget, AttemptLoopTurnInput{
		TurnID: "turn-1", Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1", RequestID: "request-1",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	call := AttemptLoopToolCallInput{
		TurnID: "turn-1", StepID: "step-1", CallID: "call-context",
		Sequence: 1, Tool: permissions.ToolMCPTool, TargetID: "context.retrieval",
		ProposalDigest:      strings.Repeat("4", 64),
		ArgumentsDigest:     strings.Repeat("5", 64),
		ToolSchemaDigest:    strings.Repeat("6", 64),
		ExecutionMode:       ToolExecutionExclusive,
		ConflictScopeDigest: strings.Repeat("7", 64),
	}
	if _, err := loops.AdmitToolCall(ctx, binding, call); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.CommitToolDispatch(ctx, binding, AttemptLoopToolDispatchInput{
		TurnID: "turn-1", StepID: "step-1", CallID: "call-context",
		AuthorizationDigest: strings.Repeat("8", 64),
		DispatchDigest:      strings.Repeat("9", 64),
	}); err != nil {
		t.Fatal(err)
	}
	// The tool was dispatched but its result was never delivered (for example
	// the provider call after the dispatch failed). A step that ends in
	// FAILURE must still finalize so the attempt can terminate instead of
	// remaining stuck on an open, undeliverable tool call.
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepFailed,
		OutputDigest: strings.Repeat("a", 64), ErrorCode: "runtime_adapter_failed",
	}); err != nil {
		t.Fatalf("failed step with undelivered tool call must finalize: %v", err)
	}
	if _, err := loops.EndTurn(ctx, binding, AttemptLoopTurnEndInput{
		TurnID: "turn-1", Outcome: AttemptTurnFailed,
	}); err != nil {
		t.Fatalf("failed turn must finalize: %v", err)
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Turns) != 1 || snapshot.Turns[0].Status != AttemptTurnFailed {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestAttemptLoopAuthorityReplaysParallelToolLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xb1)
	assignmentInput := assignment("work-loop", "run-loop")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.loop", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 7,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(
		ctx, claim("work-loop", "run-loop", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	binding := attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding)
	budget := AttemptLoopBudget{
		MaxTurns: 2, MaxStepsPerTurn: 3, MaxToolCalls: 4,
		MaxParallelToolCalls: 2, MaxResultBytes: 32_768,
		ToolTimeoutMillis: 15_000,
	}
	turn := AttemptLoopTurnInput{
		TurnID: "turn-1", Sequence: 1,
		InputDigest: strings.Repeat("1", 64),
	}
	if _, err := loops.StartTurn(ctx, binding, budget, turn); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartTurn(ctx, binding, budget, turn); err != nil {
		t.Fatalf("idempotent StartTurn error = %v", err)
	}
	step1 := AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}
	if _, err := loops.StartStep(ctx, binding, step1); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1", RequestID: "request-1",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	firstCall := AttemptLoopToolCallInput{
		TurnID: "turn-1", StepID: "step-1", CallID: "call-search",
		Sequence: 1, Tool: permissions.ToolWebSearch, TargetID: "web-search.default",
		ProposalDigest:      strings.Repeat("4", 64),
		ArgumentsDigest:     strings.Repeat("5", 64),
		ToolSchemaDigest:    strings.Repeat("6", 64),
		ExecutionMode:       ToolExecutionParallel,
		ConflictScopeDigest: strings.Repeat("7", 64),
	}
	if _, err := loops.AdmitToolCall(ctx, binding, firstCall); err != nil {
		t.Fatal(err)
	}
	secondCall := firstCall
	secondCall.CallID = "call-fetch"
	secondCall.Sequence = 2
	secondCall.Tool = permissions.ToolWebFetch
	secondCall.TargetID = "web-fetch.allowlisted"
	secondCall.ProposalDigest = strings.Repeat("8", 64)
	secondCall.ArgumentsDigest = strings.Repeat("9", 64)
	secondCall.ConflictScopeDigest = strings.Repeat("a", 64)
	if _, err := loops.AdmitToolCall(ctx, binding, secondCall); err != nil {
		t.Fatal(err)
	}
	exclusive := secondCall
	exclusive.CallID = "call-exclusive"
	exclusive.Sequence = 3
	exclusive.ExecutionMode = ToolExecutionExclusive
	exclusive.ConflictScopeDigest = strings.Repeat("b", 64)
	if _, err := loops.AdmitToolCall(
		ctx, binding, exclusive,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("exclusive overlap = %v", err)
	}
	conflicting := secondCall
	conflicting.CallID = "call-conflicting"
	conflicting.Sequence = 3
	conflicting.ConflictScopeDigest = firstCall.ConflictScopeDigest
	if _, err := loops.AdmitToolCall(
		ctx, binding, conflicting,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("parallel scope overlap = %v", err)
	}
	for index, call := range []AttemptLoopToolCallInput{firstCall, secondCall} {
		if _, err := loops.CommitToolDispatch(ctx, binding, AttemptLoopToolDispatchInput{
			TurnID: call.TurnID, StepID: call.StepID, CallID: call.CallID,
			AuthorizationDigest: strings.Repeat(string(rune('c'+index)), 64),
			DispatchDigest:      strings.Repeat(string(rune('e'+index)), 64),
		}); err != nil {
			t.Fatalf("CommitToolDispatch(%s) error = %v", call.CallID, err)
		}
		payloadBinding := attemptLoopPayloadBinding(binding, call)
		if _, err := loops.AcceptToolResult(ctx, binding, payloadBinding); err != nil {
			t.Fatalf("Accept(%s) error = %v", call.CallID, err)
		}
		if _, err := loops.DeliverToolResult(
			ctx, binding, payloadBinding,
			attemptpayload.ProofProviderContinuation,
		); err != nil {
			t.Fatalf("Deliver(%s) error = %v", call.CallID, err)
		}
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepContinue,
		OutputDigest: strings.Repeat("f", 64),
	}); err != nil {
		t.Fatal(err)
	}
	step2 := AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-2", Sequence: 2,
		ModelInputDigest: strings.Repeat("0", 64),
	}
	if _, err := loops.StartStep(ctx, binding, step2); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-2", RequestID: "request-2",
		RequestDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-2", Outcome: AttemptStepFinal,
		OutputDigest: strings.Repeat("b", 64),
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := loops.EndTurn(ctx, binding, AttemptLoopTurnEndInput{
		TurnID: "turn-1", Outcome: AttemptTurnSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertAttemptLoopSnapshot(t, snapshot)
	if _, err := loops.EndTurn(ctx, binding, AttemptLoopTurnEndInput{
		TurnID: turn.TurnID, Outcome: AttemptTurnCancelled,
	}); !errors.Is(err, ErrInvalidAttemptLoop) {
		t.Fatalf("unimplemented cancellation transition = %v", err)
	}

	rebuilt, err := loops.Snapshot(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	assertAttemptLoopSnapshot(t, rebuilt)
	rebuilt.Turns[0].Steps[0].ToolCalls[0].TargetID = "mutated"
	again, err := loops.Snapshot(ctx, binding)
	if err != nil || again.Turns[0].Steps[0].ToolCalls[0].TargetID != firstCall.TargetID {
		t.Fatalf("snapshot alias = %#v, %v", again, err)
	}
	loopEvents, err := store.ReadStream(ctx, attemptLoopStream(binding))
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]journal.Event(nil), loopEvents...)
	var tamperedPayload map[string]any
	if err := json.Unmarshal(tampered[4].PayloadJSON, &tamperedPayload); err != nil {
		t.Fatal(err)
	}
	tamperedPayload["prompt"] = "secret prompt body"
	tampered[4].PayloadJSON, err = json.Marshal(tamperedPayload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replayAttemptLoop(binding, tampered); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("unknown replay field = %v", err)
	}
	if _, _, err := runs.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(run), Status: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
	restartedRuns := newAuthority(t, store, &mutableClock{now: testNow}, 0xb3)
	restartedPayloads, err := NewAttemptPayloadAuthority(restartedRuns)
	if err != nil {
		t.Fatal(err)
	}
	restartedLoops, err := NewAttemptLoopAuthority(restartedRuns, restartedPayloads)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := restartedLoops.Snapshot(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	assertAttemptLoopSnapshot(t, restarted)

	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"secret prompt body", "Authorization: Bearer", "tool result body",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("Journal leaked %q", forbidden)
		}
	}
}

func TestAttemptLoopAuthoritySerializesConcurrentToolCallSequence(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xb4)
	assignmentInput := assignment("work-loop-race", "run-loop-race")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.loop-race", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 9,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(
		ctx, claim("work-loop-race", "run-loop-race", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, _ := NewAttemptPayloadAuthority(runs)
	loops, _ := NewAttemptLoopAuthority(runs, payloads)
	binding := attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding)
	budget := AttemptLoopBudget{
		MaxTurns: 1, MaxStepsPerTurn: 1, MaxToolCalls: 2,
		MaxParallelToolCalls: 2, MaxResultBytes: 1_024, ToolTimeoutMillis: 1_000,
	}
	if _, err := loops.StartTurn(ctx, binding, budget, AttemptLoopTurnInput{
		TurnID: "turn-race", Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-race", StepID: "step-race", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-race", StepID: "step-race", RequestID: "request-race",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	calls := []AttemptLoopToolCallInput{
		{
			TurnID: "turn-race", StepID: "step-race", CallID: "call-race-a",
			Sequence: 1, Tool: permissions.ToolWebSearch, TargetID: "search.a",
			ProposalDigest: strings.Repeat("4", 64), ArgumentsDigest: strings.Repeat("5", 64),
			ToolSchemaDigest: strings.Repeat("6", 64), ExecutionMode: ToolExecutionParallel,
			ConflictScopeDigest: strings.Repeat("7", 64),
		},
		{
			TurnID: "turn-race", StepID: "step-race", CallID: "call-race-b",
			Sequence: 1, Tool: permissions.ToolWebFetch, TargetID: "fetch.b",
			ProposalDigest: strings.Repeat("8", 64), ArgumentsDigest: strings.Repeat("9", 64),
			ToolSchemaDigest: strings.Repeat("a", 64), ExecutionMode: ToolExecutionParallel,
			ConflictScopeDigest: strings.Repeat("b", 64),
		},
	}
	errorsOut := make(chan error, len(calls))
	for _, call := range calls {
		call := call
		go func() {
			_, callErr := loops.AdmitToolCall(ctx, binding, call)
			errorsOut <- callErr
		}()
	}
	var succeeded, conflicted int
	for range calls {
		callErr := <-errorsOut
		if callErr == nil {
			succeeded++
		} else if errors.Is(callErr, ErrAttemptLoopConflict) {
			conflicted++
		} else {
			t.Fatalf("concurrent call error = %v", callErr)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent admission succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil || len(snapshot.Turns[0].Steps[0].ToolCalls) != 1 {
		t.Fatalf("concurrent snapshot = %#v, %v", snapshot, err)
	}
}

func TestAttemptLoopAuthorityReleasesSequentialSlotOnlyAfterDelivery(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xb5)
	assignmentInput := assignment("work-loop-sequential", "run-loop-sequential")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.loop-sequential", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 10,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(
		ctx, claim("work-loop-sequential", "run-loop-sequential", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	binding := attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding)
	budget := AttemptLoopBudget{
		MaxTurns: 1, MaxStepsPerTurn: 1, MaxToolCalls: 2,
		MaxParallelToolCalls: 1, MaxResultBytes: 1_024, ToolTimeoutMillis: 1_000,
	}
	if _, err := loops.StartTurn(ctx, binding, budget, AttemptLoopTurnInput{
		TurnID: "turn-sequential", Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-sequential", StepID: "step-sequential", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-sequential", StepID: "step-sequential", RequestID: "request-sequential",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	first := AttemptLoopToolCallInput{
		TurnID: "turn-sequential", StepID: "step-sequential", CallID: "call-sequential-1",
		Sequence: 1, Tool: permissions.ToolMCPTool, TargetID: "context.workspace",
		ProposalDigest: strings.Repeat("4", 64), ArgumentsDigest: strings.Repeat("5", 64),
		ToolSchemaDigest: strings.Repeat("6", 64), ExecutionMode: ToolExecutionExclusive,
		ConflictScopeDigest: strings.Repeat("7", 64),
	}
	if _, err := loops.AdmitToolCall(ctx, binding, first); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.CommitToolDispatch(ctx, binding, AttemptLoopToolDispatchInput{
		TurnID: first.TurnID, StepID: first.StepID, CallID: first.CallID,
		AuthorizationDigest: strings.Repeat("8", 64),
		DispatchDigest:      strings.Repeat("9", 64),
	}); err != nil {
		t.Fatal(err)
	}
	second := first
	second.CallID = "call-sequential-2"
	second.Sequence = 2
	second.ProposalDigest = strings.Repeat("a", 64)
	second.ArgumentsDigest = strings.Repeat("b", 64)
	second.ConflictScopeDigest = strings.Repeat("c", 64)
	if _, err := loops.AdmitToolCall(
		ctx, binding, second,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("second call before result = %v", err)
	}
	firstPayload := attemptLoopPayloadBinding(binding, first)
	if _, err := loops.AcceptToolResult(ctx, binding, firstPayload); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitToolCall(
		ctx, binding, second,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("second call before delivery proof = %v", err)
	}
	if _, err := loops.DeliverToolResult(
		ctx, binding, firstPayload, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitToolCall(ctx, binding, second); err != nil {
		t.Fatalf("second call after delivery = %v", err)
	}
	snapshot, err := loops.Snapshot(ctx, binding)
	if err != nil || len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 ||
		snapshot.Turns[0].Steps[0].ToolCalls[0].ResultStatus != attemptpayload.FactDelivered ||
		snapshot.Turns[0].Steps[0].ToolCalls[1].Sequence != 2 {
		t.Fatalf("sequential snapshot = %#v, %v", snapshot, err)
	}
}

func TestAttemptLoopAuthorityRejectsGenerationAndBindingSubstitution(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xb2)
	assignmentInput := assignment("work-loop-guard", "run-loop-guard")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.loop-guard", "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 8,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(
		ctx, claim("work-loop-guard", "run-loop-guard", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, _ := NewAttemptPayloadAuthority(runs)
	loops, _ := NewAttemptLoopAuthority(runs, payloads)
	binding := attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding)
	budget := AttemptLoopBudget{
		MaxTurns: 1, MaxStepsPerTurn: 1, MaxToolCalls: 1,
		MaxParallelToolCalls: 1, MaxResultBytes: 1_024, ToolTimeoutMillis: 1_000,
	}
	turn := AttemptLoopTurnInput{
		TurnID: "turn-guard", Sequence: 1,
		InputDigest: strings.Repeat("1", 64),
	}
	stale := binding
	stale.PayloadAuthority.ClaimGeneration++
	stale.PayloadAuthority.Scope.ClaimGeneration++
	if _, err := loops.StartTurn(
		ctx, stale, budget, turn,
	); !errors.Is(err, ErrAttemptLoopAuthority) {
		t.Fatalf("stale generation = %v", err)
	}
	changedBinding := binding
	changedBinding.PayloadAuthority.ExecutionBindingDigest = strings.Repeat("d", 64)
	if _, err := loops.StartTurn(
		ctx, changedBinding, budget, turn,
	); !errors.Is(err, ErrAttemptLoopAuthority) {
		t.Fatalf("binding substitution = %v", err)
	}
	changedCapability := binding
	changedCapability.CapabilitySetDigest = strings.Repeat("e", 64)
	if _, err := loops.StartTurn(
		ctx, changedCapability, budget, turn,
	); !errors.Is(err, ErrInvalidAttemptLoop) {
		t.Fatalf("capability substitution = %v", err)
	}
	if _, err := loops.StartTurn(ctx, binding, budget, turn); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-guard", StepID: "step-guard", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-guard", StepID: "step-guard", RequestID: "request-guard",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	call := AttemptLoopToolCallInput{
		TurnID: "turn-guard", StepID: "step-guard", CallID: "call-guard",
		Sequence: 1, Tool: permissions.ToolRead, TargetID: "workspace.read",
		ProposalDigest: strings.Repeat("4", 64), ArgumentsDigest: strings.Repeat("5", 64),
		ToolSchemaDigest: strings.Repeat("6", 64), ExecutionMode: ToolExecutionExclusive,
		ConflictScopeDigest: strings.Repeat("7", 64),
	}
	if _, err := loops.AdmitToolCall(ctx, binding, call); err != nil {
		t.Fatal(err)
	}
	premature := attemptLoopPayloadBinding(binding, call)
	if err := payloads.Accept(ctx, binding.PayloadAuthority, premature); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.CommitToolDispatch(ctx, binding, AttemptLoopToolDispatchInput{
		TurnID: call.TurnID, StepID: call.StepID, CallID: call.CallID,
		AuthorizationDigest: strings.Repeat("8", 64),
		DispatchDigest:      strings.Repeat("9", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AcceptToolResult(
		ctx, binding, premature,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("pre-dispatch result lineage = %v", err)
	}
}

func TestAttemptLoopAuthorityCommitsAndQueriesExactGovernedTestReport(t *testing.T) {
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xb6)
	assignmentInput := assignment("work-loop-report", "run-loop-report")
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.loop-report", "openai", "openai.primary", "gpt-test",
		"credential-ref-openai-primary", 11,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(
		ctx, claim("work-loop-report", "run-loop-report", "runtime-a"),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, err := NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	binding := attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding)
	budget := AttemptLoopBudget{
		MaxTurns: 1, MaxStepsPerTurn: 1, MaxToolCalls: 1,
		MaxParallelToolCalls: 1, MaxResultBytes: 4_096, ToolTimeoutMillis: 10_000,
	}
	if _, err := loops.StartTurn(ctx, binding, budget, AttemptLoopTurnInput{
		TurnID: "turn-report", Sequence: 1, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-report", StepID: "step-report", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-report", StepID: "step-report", RequestID: "request-report",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	call := AttemptLoopToolCallInput{
		TurnID: "turn-report", StepID: "step-report", CallID: "call-report",
		Sequence: 1, Tool: permissions.ToolBash, TargetID: "local-workspace",
		ProposalDigest: strings.Repeat("4", 64), ArgumentsDigest: strings.Repeat("5", 64),
		ToolSchemaDigest: strings.Repeat("6", 64), ExecutionMode: ToolExecutionExclusive,
		ConflictScopeDigest: strings.Repeat("7", 64),
	}
	if _, err := loops.AdmitToolCall(ctx, binding, call); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.CommitToolDispatch(ctx, binding, AttemptLoopToolDispatchInput{
		TurnID: call.TurnID, StepID: call.StepID, CallID: call.CallID,
		AuthorizationDigest: strings.Repeat("8", 64),
		DispatchDigest:      strings.Repeat("9", 64),
	}); err != nil {
		t.Fatal(err)
	}
	payloadBinding := attemptLoopPayloadBinding(binding, call)
	if _, err := loops.AcceptToolResult(ctx, binding, payloadBinding); err != nil {
		t.Fatal(err)
	}
	command, err := verification.RecognizeGovernedTestCommand("go test ./... -count=1")
	if err != nil {
		t.Fatal(err)
	}
	report, err := verification.NewGovernedTestReport(
		command,
		verification.GovernedTestReportInput{
			CallID: call.CallID, CallSequence: call.Sequence,
			ArgumentsDigest: call.ArgumentsDigest,
			ExecutionID:     "execution-report", ExitCode: 0,
			OutputDigest: "sha256:" + strings.Repeat("a", 64), DurationMS: 12,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := loops.CommitGovernedTestReport(
		ctx, binding, payloadBinding, report,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.GovernedTestReports) != 1 ||
		snapshot.GovernedTestReports[0].Report.Digest() != report.Digest() ||
		snapshot.GovernedTestReports[0].PayloadBinding != payloadBinding {
		t.Fatalf("test reports = %#v", snapshot.GovernedTestReports)
	}
	if _, err := loops.CommitGovernedTestReport(
		ctx, binding, payloadBinding, report,
	); err != nil {
		t.Fatalf("idempotent report commit = %v", err)
	}

	changedArguments, err := verification.NewGovernedTestReport(
		command,
		verification.GovernedTestReportInput{
			CallID: call.CallID, CallSequence: call.Sequence,
			ArgumentsDigest: strings.Repeat("b", 64),
			ExecutionID:     "execution-report", ExitCode: 0,
			OutputDigest: "sha256:" + strings.Repeat("a", 64), DurationMS: 12,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loops.CommitGovernedTestReport(
		ctx, binding, payloadBinding, changedArguments,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("arguments substitution = %v", err)
	}
	changedPayload := payloadBinding
	changedPayload.ContentDigest = strings.Repeat("c", 64)
	if _, err := loops.CommitGovernedTestReport(
		ctx, binding, changedPayload, report,
	); !errors.Is(err, ErrAttemptLoopConflict) {
		t.Fatalf("payload substitution = %v", err)
	}

	if _, err := loops.DeliverToolResult(
		ctx, binding, payloadBinding, attemptpayload.ProofProviderContinuation,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: call.TurnID, StepID: call.StepID, Outcome: AttemptStepFinal,
		OutputDigest: strings.Repeat("d", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndTurn(ctx, binding, AttemptLoopTurnEndInput{
		TurnID: call.TurnID, Outcome: AttemptTurnSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runs.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(run), Status: "succeeded",
	}); err != nil {
		t.Fatal(err)
	}
	reports, err := loops.GovernedTestReportsForAttempt(
		ctx,
		AttemptReportQuery{
			TeamInstanceID: binding.TeamInstanceID,
			Authority:      binding.PayloadAuthority,
		},
	)
	if err != nil || len(reports) != 1 || reports[0].Digest() != report.Digest() {
		t.Fatalf("queried reports=%#v error=%v", reports, err)
	}
	foreign := binding.PayloadAuthority
	foreign.CapsuleDigest = strings.Repeat("e", 64)
	if _, err := loops.GovernedTestReportsForAttempt(
		ctx,
		AttemptReportQuery{TeamInstanceID: binding.TeamInstanceID, Authority: foreign},
	); !errors.Is(err, ErrAttemptLoopAuthority) {
		t.Fatalf("foreign Attempt query = %v", err)
	}

	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "go test") ||
		strings.Contains(string(encoded), "./...") {
		t.Fatalf("test command entered Journal: %s", encoded)
	}
}

func attemptLoopBindingFixture(
	run RunRecord,
	execution FrozenExecutionBinding,
) AttemptLoopBinding {
	payloadAuthority := attemptPayloadAuthorityFixture(run, execution)
	return AttemptLoopBinding{
		SchemaVersion:               1,
		AttemptID:                   run.ID(),
		TeamInstanceID:              "team-loop",
		PayloadAuthority:            payloadAuthority,
		PermissionProfileID:         "permission-profile.default",
		PermissionProfileGeneration: 3,
		PermissionProfileDigest:     strings.Repeat("8", 64),
		CapabilitySetDigest:         attemptLoopCapabilitySetDigest(execution.Capabilities),
		ToolSchemaSetDigest:         strings.Repeat("9", 64),
		BudgetPolicyVersion:         1,
	}
}

func attemptLoopPayloadBinding(
	binding AttemptLoopBinding,
	call AttemptLoopToolCallInput,
) attemptpayload.Binding {
	return attemptpayload.Binding{
		PayloadID:     "payload-" + call.CallID,
		Scope:         binding.PayloadAuthority.Scope,
		CallID:        call.CallID,
		Sequence:      call.Sequence,
		ContentType:   "application/json",
		ContentDigest: call.ProposalDigest,
	}
}

func assertAttemptLoopSnapshot(t *testing.T, snapshot AttemptLoopSnapshot) {
	t.Helper()
	if snapshot.Status != AttemptLoopRunning || len(snapshot.Turns) != 1 ||
		snapshot.Turns[0].Status != AttemptTurnSucceeded ||
		len(snapshot.Turns[0].Steps) != 2 ||
		snapshot.Turns[0].Steps[0].Outcome != AttemptStepContinue ||
		len(snapshot.Turns[0].Steps[0].ToolCalls) != 2 ||
		!snapshot.Turns[0].Steps[0].ToolCalls[0].Dispatched ||
		snapshot.Turns[0].Steps[0].ToolCalls[0].ResultStatus !=
			attemptpayload.FactDelivered ||
		snapshot.Turns[0].Steps[1].Outcome != AttemptStepFinal {
		t.Fatalf("Attempt loop snapshot = %#v", snapshot)
	}
}
