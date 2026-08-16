package work

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/journal"
)

func TestAgentInboxQueueConsumesAtomicallyIntoNextTurn(t *testing.T) {
	ctx, store, loops, inbox, binding, budget := agentInboxFixture(t, "queue")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)

	queued := agentInboxPayloadFixture(binding, "segment-1", "input-queue-1", 1)
	queued.Mode = agentinbox.ModeQueue
	queued.ContextScope = agentinbox.ScopeConversationShared
	queued.TargetTurnID = "turn-2"
	queued.TargetTurnSequence = 2
	if _, err := inbox.admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}
	if state, err := inbox.Snapshot(ctx, binding); err != nil ||
		len(state.Loop.Turns) != 1 || state.Loop.Turns[0].Status != "" {
		t.Fatalf("Queue woke active Turn = %#v, %v", state, err)
	}
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	state, err := inbox.startQueuedTurn(ctx, binding, queued, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: queued.ContentDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Inputs) != 1 || state.Inputs[0].Status != agentinbox.StatusConsumed ||
		len(state.Loop.Turns) != 2 || state.Loop.Turns[1].TurnID != "turn-2" {
		t.Fatalf("queue state = %#v", state)
	}
	if _, err := inbox.startQueuedTurn(ctx, binding, queued, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: queued.ContentDigest,
	}); err != nil {
		t.Fatalf("idempotent queue consume = %v", err)
	}

	restartedPayloads, _ := NewAttemptPayloadAuthority(loops.runs)
	restartedLoops, _ := NewAttemptLoopAuthority(loops.runs, restartedPayloads)
	restartedInbox, _ := NewAgentInboxAuthority(restartedLoops)
	rebuilt, err := restartedInbox.Snapshot(ctx, binding)
	if err != nil || len(rebuilt.Inputs) != 1 ||
		rebuilt.Inputs[0].Status != agentinbox.StatusConsumed || len(rebuilt.Loop.Turns) != 2 {
		t.Fatalf("rebuilt queue state = %#v, %v", rebuilt, err)
	}

	events, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(events)
	for _, forbidden := range []string{"queued secret body", "prompt body", "Authorization: Bearer"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("Journal leaked %q", forbidden)
		}
	}
}

func TestAgentInboxSteerAndInjectFreezeAndConsumeNextStepInOrder(t *testing.T) {
	ctx, _, loops, inbox, binding, budget := agentInboxFixture(t, "step")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
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

	steer := agentInboxPayloadFixture(binding, "segment-1", "input-steer-1", 1)
	steer.Mode = agentinbox.ModeSteer
	steer.ContextScope = agentinbox.ScopeAgentPrivate
	steer.TargetStepID, steer.TargetStepSequence = "step-2", 2
	inject := agentInboxPayloadFixture(binding, "segment-1", "input-inject-1", 2)
	inject.Mode = agentinbox.ModeInject
	inject.ContextScope = agentinbox.ScopeArtifact
	inject.ScopeTargetID = "artifact-diff-1"
	inject.TargetStepID, inject.TargetStepSequence = "step-2", 2
	if _, err := inbox.admit(ctx, binding, steer); err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.admit(ctx, binding, inject); err != nil {
		t.Fatal(err)
	}
	if state, err := inbox.Snapshot(ctx, binding); err != nil ||
		len(state.Loop.Turns[0].Steps) != 1 ||
		state.Loop.Turns[0].Steps[0].ModelRequestID == "" {
		t.Fatalf("Steer/Inject woke a Step = %#v, %v", state, err)
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepContinue,
		OutputDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatal(err)
	}

	state, err := inbox.startTargetStep(ctx, binding, []agentinbox.Binding{steer, inject}, AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-2", Sequence: 2,
		ModelInputDigest: strings.Repeat("5", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Inputs) != 2 || state.Inputs[0].InputID != steer.InputID ||
		state.Inputs[1].InputID != inject.InputID ||
		state.Inputs[0].Status != agentinbox.StatusConsumed ||
		state.Inputs[1].Status != agentinbox.StatusConsumed ||
		len(state.Loop.Turns[0].Steps) != 2 {
		t.Fatalf("step state = %#v", state)
	}
}

func TestAgentInboxQueueOrderingCannotSkipEarlierInput(t *testing.T) {
	ctx, _, loops, inbox, binding, budget := agentInboxFixture(t, "queue-order")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	first := agentInboxPayloadFixture(binding, "segment-1", "input-queue-1", 1)
	first.Mode, first.ContextScope = agentinbox.ModeQueue, agentinbox.ScopeConversationShared
	first.TargetTurnID, first.TargetTurnSequence = "turn-2", 2
	second := agentInboxPayloadFixture(binding, "segment-1", "input-queue-2", 2)
	second.Mode, second.ContextScope = agentinbox.ModeQueue, agentinbox.ScopeConversationShared
	second.TargetTurnID, second.TargetTurnSequence = "turn-3", 3
	if _, err := inbox.admit(ctx, binding, first); err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.admit(ctx, binding, second); err != nil {
		t.Fatal(err)
	}
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	if _, err := inbox.startQueuedTurn(ctx, binding, second, AttemptLoopTurnInput{
		TurnID: "turn-3", Sequence: 3, InputDigest: second.ContentDigest,
	}); !errors.Is(err, ErrAgentInboxConflict) {
		t.Fatalf("skipped Queue input = %v", err)
	}
	state, err := inbox.startQueuedTurn(ctx, binding, first, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: first.ContentDigest,
	})
	if err != nil || state.Inputs[0].Status != agentinbox.StatusConsumed ||
		state.Inputs[1].Status != agentinbox.StatusPending {
		t.Fatalf("ordered Queue state = %#v, %v", state, err)
	}
}

func TestAgentInboxRejectsStaleTargetsSubstitutionAndConcurrentConsumption(t *testing.T) {
	ctx, _, loops, inbox, binding, budget := agentInboxFixture(t, "guards")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	queued := agentInboxPayloadFixture(binding, "segment-1", "input-queue-1", 1)
	queued.Mode = agentinbox.ModeQueue
	queued.ContextScope = agentinbox.ScopeConversationShared
	queued.TargetTurnID, queued.TargetTurnSequence = "turn-2", 2
	if _, err := inbox.admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}

	changed := queued
	changed.ContentDigest = strings.Repeat("f", 64)
	if _, err := inbox.admit(ctx, binding, changed); !errors.Is(err, ErrAgentInboxConflict) {
		t.Fatalf("content substitution = %v", err)
	}
	stale := binding
	stale.PayloadAuthority.ClaimGeneration++
	stale.PayloadAuthority.Scope.ClaimGeneration++
	if _, err := inbox.admit(ctx, stale, queued); !errors.Is(err, ErrAgentInboxAuthority) {
		t.Fatalf("stale generation = %v", err)
	}

	errorsOut := make(chan error, 2)
	for range 2 {
		go func() {
			_, consumeErr := inbox.startQueuedTurn(ctx, binding, queued, AttemptLoopTurnInput{
				TurnID: "turn-2", Sequence: 2, InputDigest: queued.ContentDigest,
			})
			errorsOut <- consumeErr
		}()
	}
	for range 2 {
		if err := <-errorsOut; err != nil {
			t.Fatalf("concurrent idempotent consume = %v", err)
		}
	}
	state, err := inbox.Snapshot(ctx, binding)
	if err != nil || len(state.Loop.Turns) != 2 || len(state.Inputs) != 1 {
		t.Fatalf("concurrent state = %#v, %v", state, err)
	}
}

func agentInboxFixture(t *testing.T, suffix string) (
	context.Context, *journal.Store,
	*AttemptLoopAuthority, *AgentInboxAuthority, AttemptLoopBinding, AttemptLoopBudget,
) {
	t.Helper()
	ctx := context.Background()
	store := openAuthorityStore(t)
	seedRuntime(t, store, "runtime-a", "online", 1)
	runs := newAuthority(t, store, &mutableClock{now: testNow}, 0xc1)
	workID, runID := "work-inbox-"+suffix, "run-inbox-"+suffix
	assignmentInput := assignment(workID, runID)
	assignmentInput.ExecutionBinding = testFrozenExecutionBinding(
		t, "profile.inbox."+suffix, "deepseek", "deepseek.primary", "deepseek-chat",
		"credential-ref-deepseek-primary", 11,
	)
	if _, _, err := runs.CreateAndAssign(ctx, assignmentInput); err != nil {
		t.Fatal(err)
	}
	_, run, err := runs.Claim(ctx, claim(workID, runID, "runtime-a"))
	if err != nil {
		t.Fatal(err)
	}
	_, run, err = runs.Start(ctx, generationInput(run))
	if err != nil {
		t.Fatal(err)
	}
	payloads, _ := NewAttemptPayloadAuthority(runs)
	loops, _ := NewAttemptLoopAuthority(runs, payloads)
	inbox, err := NewAgentInboxAuthority(loops)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, store, loops, inbox,
		attemptLoopBindingFixture(run, assignmentInput.ExecutionBinding),
		AttemptLoopBudget{MaxTurns: 3, MaxStepsPerTurn: 3, MaxToolCalls: 2,
			MaxParallelToolCalls: 1, MaxResultBytes: 1024, ToolTimeoutMillis: 1000}
}

func startAgentInboxTurn(t *testing.T, ctx context.Context, loops *AttemptLoopAuthority,
	binding AttemptLoopBinding, budget AttemptLoopBudget, turnID string, sequence int,
) {
	t.Helper()
	if _, err := loops.StartTurn(ctx, binding, budget, AttemptLoopTurnInput{
		TurnID: turnID, Sequence: sequence, InputDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatal(err)
	}
}

func finishAgentInboxTurn(t *testing.T, ctx context.Context, loops *AttemptLoopAuthority,
	binding AttemptLoopBinding, turnID, stepID string,
) {
	t.Helper()
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: turnID, StepID: stepID, Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: turnID, StepID: stepID, RequestID: "request-" + stepID,
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: turnID, StepID: stepID, Outcome: AttemptStepFinal,
		OutputDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndTurn(ctx, binding, AttemptLoopTurnEndInput{
		TurnID: turnID, Outcome: AttemptTurnSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
}

func agentInboxPayloadFixture(binding AttemptLoopBinding, segmentID, inputID string, order int64) agentinbox.Binding {
	return agentinbox.Binding{
		PayloadID: "agent-input-payload-" + inputID, InputID: inputID,
		ConversationID: binding.PayloadAuthority.ConversationID, SegmentID: segmentID,
		AgentInstanceID: binding.PayloadAuthority.AgentInstanceID,
		WorkItemID:      binding.PayloadAuthority.WorkItemID, RunID: binding.PayloadAuthority.RunID,
		ClaimGeneration:        binding.PayloadAuthority.ClaimGeneration,
		RuntimeInstanceID:      binding.PayloadAuthority.RuntimeInstanceID,
		ExecutionBindingDigest: binding.PayloadAuthority.ExecutionBindingDigest,
		CapsuleDigest:          binding.PayloadAuthority.CapsuleDigest, OrderKey: order,
		ContentType: "text/plain", ContentDigest: strings.Repeat("a", 64),
	}
}
