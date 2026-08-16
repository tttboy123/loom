package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestAgentInboxCoordinatorRollsBackUnadmittedCiphertext(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-rollback")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, err := NewAgentInboxCoordinator(authority, store)
	if err != nil {
		t.Fatal(err)
	}
	payload := agentInboxPayloadWithContent(binding, "input-invalid", 1, "secret invalid input")
	payload.Binding.Mode = agentinbox.ModeQueue
	payload.Binding.ContextScope = agentinbox.ScopeConversationShared
	payload.Binding.TargetTurnID, payload.Binding.TargetTurnSequence = "turn-wrong", 9
	if _, err := coordinator.Admit(ctx, binding, payload); !errors.Is(err, ErrAgentInboxConflict) {
		t.Fatalf("invalid target = %v", err)
	}
	if store.count() != 0 {
		t.Fatalf("orphan ciphertext count = %d", store.count())
	}
}

func TestAgentInboxCoordinatorReconcilesCommittedConsumptionAfterRestart(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-reconcile")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	payload := agentInboxPayloadWithContent(binding, "input-queue", 1, "secret queued input")
	payload.Binding.Mode = agentinbox.ModeQueue
	payload.Binding.ContextScope = agentinbox.ScopeConversationShared
	payload.Binding.TargetTurnID, payload.Binding.TargetTurnSequence = "turn-2", 2
	frozen := payload.Binding
	if _, err := coordinator.Admit(ctx, binding, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.startQueuedTurn(ctx, binding, frozen, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: frozen.ContentDigest,
	}); err != nil {
		t.Fatal(err)
	}
	stored, err := store.ReadAgentInput(ctx, frozen)
	if err != nil || stored.Status != agentinbox.StatusPending {
		stored.Close()
		t.Fatalf("pre-reconcile = %#v, %v", stored, err)
	}
	stored.Close()
	if err := coordinator.Reconcile(ctx, binding); err != nil {
		t.Fatal(err)
	}
	stored, err = store.ReadAgentInput(ctx, frozen)
	if err != nil || stored.Status != agentinbox.StatusConsumed ||
		string(stored.Content) != "secret queued input" {
		stored.Close()
		t.Fatalf("post-reconcile = %#v, %v", stored, err)
	}
	stored.Close()
}

func TestAgentInboxCoordinatorRecoversConsumedStepBeforeModelRequest(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-recover-step")
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
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	steer := agentInboxPayloadWithContent(binding, "input-recover-step", 1, "recover exact step input")
	steer.Binding.Mode = agentinbox.ModeSteer
	steer.Binding.ContextScope = agentinbox.ScopeAgentPrivate
	steer.Binding.TargetStepID, steer.Binding.TargetStepSequence = "step-2", 2
	if _, err := coordinator.Admit(ctx, binding, steer); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepContinue,
		OutputDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatal(err)
	}
	step := AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-2", Sequence: 2,
		ModelInputDigest: strings.Repeat("5", 64),
	}
	if _, err := authority.startTargetStep(ctx, binding, []agentinbox.Binding{steer.Binding}, step); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Reconcile(ctx, binding); err != nil {
		t.Fatal(err)
	}

	payloads, _, err := coordinator.ConsumeTargetStep(
		ctx, binding, []agentinbox.Binding{steer.Binding}, step,
	)
	if err != nil || len(payloads) != 1 || payloads[0].Status != agentinbox.StatusConsumed ||
		string(payloads[0].Content) != "recover exact step input" {
		closeAgentInboxPayloads(payloads)
		t.Fatalf("recovered step payloads = %#v, %v", payloads, err)
	}
	content := payloads[0].Content
	closeAgentInboxPayloads(payloads)
	if !allAgentInboxBytesZero(content) {
		t.Fatal("recovered step plaintext was not zeroized")
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-2", RequestID: "request-2",
		RequestDigest: strings.Repeat("6", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if payloads, _, err := coordinator.ConsumeTargetStep(
		ctx, binding, []agentinbox.Binding{steer.Binding}, step,
	); !errors.Is(err, ErrAgentInboxConflict) {
		closeAgentInboxPayloads(payloads)
		t.Fatalf("post-model-request recovery = %#v, %v", payloads, err)
	}
}

func TestAgentInboxCoordinatorRecoversConsumedQueueBeforeModelRequest(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-recover-queue")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	queued := agentInboxPayloadWithContent(binding, "input-recover-queue", 1, "recover exact queued input")
	queued.Binding.Mode = agentinbox.ModeQueue
	queued.Binding.ContextScope = agentinbox.ScopeConversationShared
	queued.Binding.TargetTurnID, queued.Binding.TargetTurnSequence = "turn-2", 2
	if _, err := coordinator.Admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}
	turn := AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: queued.Binding.ContentDigest,
	}
	if _, err := authority.startQueuedTurn(ctx, binding, queued.Binding, turn); err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Reconcile(ctx, binding); err != nil {
		t.Fatal(err)
	}

	payload, _, err := coordinator.ConsumeQueuedTurn(ctx, binding, queued.Binding)
	if err != nil || payload.Status != agentinbox.StatusConsumed ||
		string(payload.Content) != "recover exact queued input" {
		payload.Close()
		t.Fatalf("recovered queue payload = %#v, %v", payload, err)
	}
	content := payload.Content
	payload.Close()
	if !allAgentInboxBytesZero(content) {
		t.Fatal("recovered queue plaintext was not zeroized")
	}
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-2", StepID: "step-2-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("7", 64),
	}); err != nil {
		t.Fatal(err)
	}
	payload, _, err = coordinator.ConsumeQueuedTurn(ctx, binding, queued.Binding)
	if err != nil || string(payload.Content) != "recover exact queued input" {
		payload.Close()
		t.Fatalf("post-step queue recovery = %#v, %v", payload, err)
	}
	payload.Close()
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-2", StepID: "step-2-1", RequestID: "request-2-1",
		RequestDigest: strings.Repeat("8", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if payload, _, err := coordinator.ConsumeQueuedTurn(
		ctx, binding, queued.Binding,
	); !errors.Is(err, ErrAgentInboxConflict) {
		payload.Close()
		t.Fatalf("post-model-request queue recovery = %#v, %v", payload, err)
	}
}

func TestAgentInboxCoordinatorReconstructsRestartCheckpointsWithoutReplayingProvider(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-restart-checkpoint")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	queued := agentInboxPayloadWithContent(
		binding, "input-restart-queue", 1, "private restart checkpoint input",
	)
	queued.Binding.Mode = agentinbox.ModeQueue
	queued.Binding.ContextScope = agentinbox.ScopeConversationShared
	queued.Binding.TargetTurnID, queued.Binding.TargetTurnSequence = "turn-2", 2
	if _, err := coordinator.Admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.startQueuedTurn(ctx, binding, queued.Binding, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: queued.Binding.ContentDigest,
	}); err != nil {
		t.Fatal(err)
	}

	restartedPayloads, _ := NewAttemptPayloadAuthority(loops.runs)
	restartedLoops, _ := NewAttemptLoopAuthority(loops.runs, restartedPayloads)
	restartedAuthority, _ := NewAgentInboxAuthority(restartedLoops)
	restarted, _ := NewAgentInboxCoordinator(restartedAuthority, store)
	report, err := restarted.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("restart report = %#v, %v", report, err)
	}
	outcome := report.Outcomes[0]
	if outcome.Disposition != AgentAttemptRestartPreModelResume ||
		outcome.Binding != binding || outcome.Budget != budget ||
		outcome.SegmentID != queued.Binding.SegmentID ||
		outcome.TurnID != "turn-2" || outcome.TurnSequence != 2 ||
		outcome.StepID != "" || outcome.StepSequence != 0 ||
		outcome.CheckpointDigest != strings.Repeat("4", 64) ||
		len(outcome.InputIDs) != 1 || outcome.InputIDs[0] != queued.Binding.InputID ||
		outcome.ModelRequestID != "" ||
		outcome.ExecutionBinding.BindingDigest != binding.PayloadAuthority.ExecutionBindingDigest {
		t.Fatalf("restart outcome = %#v", outcome)
	}
	stored, err := store.ReadAgentInput(ctx, queued.Binding)
	if err != nil || stored.Status != agentinbox.StatusConsumed {
		stored.Close()
		t.Fatalf("reconciled payload = %#v, %v", stored, err)
	}
	stored.Close()
	restored, err := restarted.RestoreAgentAttemptInputs(ctx, outcome)
	if err != nil || len(restored) != 1 ||
		string(restored[0].Content) != "private restart checkpoint input" {
		closeAgentInboxPayloads(restored)
		t.Fatalf("restored queue input = %#v, %v", restored, err)
	}
	closeAgentInboxPayloads(restored)
	substituted := outcome
	substituted.InputIDs = []string{"input-substituted"}
	if restored, err := restarted.RestoreAgentAttemptInputs(ctx, substituted); !errors.Is(err, ErrInvalidAgentInboxCoordinator) && !errors.Is(err, ErrAgentInboxConflict) {
		closeAgentInboxPayloads(restored)
		t.Fatalf("substituted recovery input = %#v, %v", restored, err)
	}
}

func TestAgentInboxCoordinatorReconstructsConsumedStepCheckpointAfterRestart(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-restart-step")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1",
		RequestID: "request-1", RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	steer := agentInboxPayloadWithContent(
		binding, "input-restart-step", 1, "private restart step input",
	)
	steer.Binding.Mode = agentinbox.ModeSteer
	steer.Binding.ContextScope = agentinbox.ScopeAgentPrivate
	steer.Binding.TargetStepID, steer.Binding.TargetStepSequence = "step-2", 2
	if _, err := coordinator.Admit(ctx, binding, steer); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepContinue,
		OutputDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.startTargetStep(
		ctx, binding, []agentinbox.Binding{steer.Binding}, AttemptLoopStepInput{
			TurnID: "turn-1", StepID: "step-2", Sequence: 2,
			ModelInputDigest: strings.Repeat("5", 64),
		},
	); err != nil {
		t.Fatal(err)
	}

	restartedPayloads, _ := NewAttemptPayloadAuthority(loops.runs)
	restartedLoops, _ := NewAttemptLoopAuthority(loops.runs, restartedPayloads)
	restartedAuthority, _ := NewAgentInboxAuthority(restartedLoops)
	restarted, _ := NewAgentInboxCoordinator(restartedAuthority, store)
	report, err := restarted.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("step restart report = %#v, %v", report, err)
	}
	outcome := report.Outcomes[0]
	if outcome.Disposition != AgentAttemptRestartPreModelResume ||
		outcome.SegmentID != steer.Binding.SegmentID ||
		outcome.TurnID != "turn-1" || outcome.TurnSequence != 1 ||
		outcome.StepID != "step-2" || outcome.StepSequence != 2 ||
		outcome.CheckpointDigest != strings.Repeat("4", 64) ||
		len(outcome.InputIDs) != 1 || outcome.InputIDs[0] != steer.Binding.InputID {
		t.Fatalf("step restart outcome = %#v", outcome)
	}
	restored, err := restarted.RestoreAgentAttemptInputs(ctx, outcome)
	if err != nil || len(restored) != 1 ||
		string(restored[0].Content) != "private restart step input" {
		closeAgentInboxPayloads(restored)
		t.Fatalf("restored step input = %#v, %v", restored, err)
	}
	closeAgentInboxPayloads(restored)
}

func TestAgentInboxCoordinatorBlocksRestartAcrossRouteSegments(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-restart-segment-conflict")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1",
		RequestID: "request-1", RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	steer := agentInboxPayloadWithContent(binding, "input-segment-a", 1, "segment a input")
	steer.Binding.Mode = agentinbox.ModeSteer
	steer.Binding.ContextScope = agentinbox.ScopeAgentPrivate
	steer.Binding.TargetStepID, steer.Binding.TargetStepSequence = "step-2", 2
	inject := agentInboxPayloadWithContent(binding, "input-segment-b", 2, "segment b input")
	inject.Binding.Mode = agentinbox.ModeInject
	inject.Binding.ContextScope = agentinbox.ScopeAgentPrivate
	inject.Binding.SegmentID = "segment-2"
	inject.Binding.TargetStepID, inject.Binding.TargetStepSequence = "step-2", 2
	for _, payload := range []agentinbox.Payload{steer, inject} {
		if _, err := coordinator.Admit(ctx, binding, payload); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepContinue,
		OutputDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.startTargetStep(
		ctx, binding, []agentinbox.Binding{steer.Binding, inject.Binding}, AttemptLoopStepInput{
			TurnID: "turn-1", StepID: "step-2", Sequence: 2,
			ModelInputDigest: strings.Repeat("5", 64),
		},
	); err != nil {
		t.Fatal(err)
	}

	restartedPayloads, _ := NewAttemptPayloadAuthority(loops.runs)
	restartedLoops, _ := NewAttemptLoopAuthority(loops.runs, restartedPayloads)
	restartedAuthority, _ := NewAgentInboxAuthority(restartedLoops)
	restarted, _ := NewAgentInboxCoordinator(restartedAuthority, store)
	report, err := restarted.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("restart report = %#v, %v", report, err)
	}
	outcome := report.Outcomes[0]
	if outcome.Disposition != AgentAttemptRestartRecoveryBlocked ||
		outcome.ErrorCode != "agent_input_recovery_conflict" || outcome.SegmentID != "" ||
		outcome.CheckpointDigest != "" || len(outcome.InputIDs) != 0 || outcome.Retryable {
		t.Fatalf("segment conflict outcome = %#v", outcome)
	}
}

func TestAgentInboxCoordinatorMarksOpenModelRequestUncertainAndSkipsTerminalRun(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-restart-uncertain")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	if _, err := loops.StartStep(ctx, binding, AttemptLoopStepInput{
		TurnID: "turn-1", StepID: "step-1", Sequence: 1,
		ModelInputDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, binding, AttemptLoopModelRequestInput{
		TurnID: "turn-1", StepID: "step-1", RequestID: "request-uncertain",
		RequestDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatal(err)
	}
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	report, err := coordinator.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 1 {
		t.Fatalf("uncertain report = %#v, %v", report, err)
	}
	outcome := report.Outcomes[0]
	if outcome.Disposition != AgentAttemptRestartProviderOutcomeUncertain ||
		outcome.TurnID != "turn-1" || outcome.TurnSequence != 1 ||
		outcome.StepID != "step-1" || outcome.StepSequence != 1 ||
		outcome.ModelRequestID != "request-uncertain" || outcome.CheckpointDigest != "" ||
		len(outcome.InputIDs) != 0 {
		t.Fatalf("uncertain outcome = %#v", outcome)
	}

	run, err := authority.loops.payloads.exactRun(ctx, binding.PayloadAuthority)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loops.runs.CommitTerminal(ctx, RunTerminalInput{
		RunGenerationInput: generationInput(run), Status: "failed", Reason: "daemon_restart",
	}); err != nil {
		t.Fatal(err)
	}
	report, err = coordinator.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 0 {
		t.Fatalf("terminal restart report = %#v, %v", report, err)
	}
}

func TestAgentInboxCoordinatorIsolatesMissingRestartPayloadToAffectedAttempt(t *testing.T) {
	ctx, journalStore, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-restart-isolation")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	queued := agentInboxPayloadWithContent(
		binding, "input-restart-missing", 1, "private missing restart input",
	)
	queued.Binding.Mode = agentinbox.ModeQueue
	queued.Binding.ContextScope = agentinbox.ScopeConversationShared
	queued.Binding.TargetTurnID, queued.Binding.TargetTurnSequence = "turn-2", 2
	if _, err := coordinator.Admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.startQueuedTurn(ctx, binding, queued.Binding, AttemptLoopTurnInput{
		TurnID: "turn-2", Sequence: 2, InputDigest: queued.Binding.ContentDigest,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAgentInput(ctx, queued.Binding); err != nil {
		t.Fatal(err)
	}
	seedRuntime(t, journalStore, "runtime-b", "online", 1)
	otherAssignment := assignment("work-restart-other", "run-restart-other")
	budgetUnits := int64(250)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile.restart.other", AdapterType: "loom-native",
		ProviderID: "openai", ProviderAccountID: "openai.primary", ModelID: "gpt-5",
		AuthMode: loomruntime.AuthBrokered, EndpointFingerprint: strings.Repeat("b", 64),
		CredentialReference: "credential-ref-openai-primary", CredentialRevision: 12,
		RequiredCapabilities: []string{"chat", "tools"}, Timeout: 90 * time.Second,
		Budget: &budgetUnits,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-b", DeviceID: "device.local", AdapterType: "loom-native",
		DisplayName: "Loom Native B", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"chat", "tools"}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherAssignment.ExecutionBinding, err = loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loops.runs.CreateAndAssign(ctx, otherAssignment); err != nil {
		t.Fatal(err)
	}
	_, otherRun, err := loops.runs.Claim(ctx, claim(
		otherAssignment.WorkItemID, otherAssignment.RunID, "runtime-b",
	))
	if err != nil {
		t.Fatal(err)
	}
	if _, otherRun, err = loops.runs.Start(ctx, generationInput(otherRun)); err != nil {
		t.Fatal(err)
	}
	otherBinding := attemptLoopBindingFixture(otherRun, otherAssignment.ExecutionBinding)
	startAgentInboxTurn(t, ctx, loops, otherBinding, budget, "turn-other", 1)
	if _, err := loops.StartStep(ctx, otherBinding, AttemptLoopStepInput{
		TurnID: "turn-other", StepID: "step-other", Sequence: 1,
		ModelInputDigest: strings.Repeat("6", 64),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.AdmitModelRequest(ctx, otherBinding, AttemptLoopModelRequestInput{
		TurnID: "turn-other", StepID: "step-other", RequestID: "request-other",
		RequestDigest: strings.Repeat("7", 64),
	}); err != nil {
		t.Fatal(err)
	}

	restartedPayloads, _ := NewAttemptPayloadAuthority(loops.runs)
	restartedLoops, _ := NewAttemptLoopAuthority(loops.runs, restartedPayloads)
	restartedAuthority, _ := NewAgentInboxAuthority(restartedLoops)
	restarted, _ := NewAgentInboxCoordinator(restartedAuthority, store)
	report, err := restarted.RecoverAfterRestart(ctx)
	if err != nil || len(report.Outcomes) != 2 {
		t.Fatalf("isolated restart report = %#v, %v", report, err)
	}
	byRun := make(map[string]AgentAttemptRestartOutcome, len(report.Outcomes))
	for _, outcome := range report.Outcomes {
		byRun[outcome.Binding.PayloadAuthority.RunID] = outcome
	}
	blocked := byRun[binding.PayloadAuthority.RunID]
	if blocked.Disposition != AgentAttemptRestartRecoveryBlocked ||
		blocked.Binding != binding || blocked.ErrorCode != "agent_input_recovery_unavailable" ||
		blocked.Retryable || blocked.CheckpointDigest != "" || len(blocked.InputIDs) != 0 {
		t.Fatalf("isolated blocked outcome = %#v", blocked)
	}
	unaffected := byRun[otherBinding.PayloadAuthority.RunID]
	if unaffected.Disposition != AgentAttemptRestartProviderOutcomeUncertain ||
		unaffected.Binding != otherBinding || unaffected.ModelRequestID != "request-other" {
		t.Fatalf("unaffected restart outcome = %#v", unaffected)
	}
}

func TestAgentInboxCoordinatorConsumesTargetStepIntoZeroizableRuntimePayloads(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-runtime-step")
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
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	steer := agentInboxPayloadWithContent(binding, "input-steer", 1, "keep the API boundary narrow")
	steer.Binding.Mode = agentinbox.ModeSteer
	steer.Binding.ContextScope = agentinbox.ScopeAgentPrivate
	steer.Binding.TargetStepID, steer.Binding.TargetStepSequence = "step-2", 2
	inject := agentInboxPayloadWithContent(binding, "input-inject", 2, "inspect artifact digest 42")
	inject.Binding.Mode = agentinbox.ModeInject
	inject.Binding.ContextScope = agentinbox.ScopeArtifact
	inject.Binding.ScopeTargetID = "artifact-diff-42"
	inject.Binding.TargetStepID, inject.Binding.TargetStepSequence = "step-2", 2
	for _, payload := range []agentinbox.Payload{steer, inject} {
		if _, err := coordinator.Admit(ctx, binding, payload); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := loops.EndStep(ctx, binding, AttemptLoopStepEndInput{
		TurnID: "turn-1", StepID: "step-1", Outcome: AttemptStepContinue,
		OutputDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatal(err)
	}

	payloads, state, err := coordinator.ConsumeTargetStep(
		ctx, binding, []agentinbox.Binding{inject.Binding, steer.Binding},
		AttemptLoopStepInput{
			TurnID: "turn-1", StepID: "step-2", Sequence: 2,
			ModelInputDigest: strings.Repeat("5", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(payloads) != 2 || payloads[0].Binding.InputID != "input-steer" ||
		payloads[1].Binding.InputID != "input-inject" ||
		string(payloads[0].Content) != "keep the API boundary narrow" ||
		string(payloads[1].Content) != "inspect artifact digest 42" ||
		payloads[0].Status != agentinbox.StatusConsumed ||
		payloads[1].Status != agentinbox.StatusConsumed ||
		len(state.Loop.Turns[0].Steps) != 2 {
		t.Fatalf("runtime payloads/state = %#v / %#v", payloads, state)
	}
	firstBytes := payloads[0].Content
	secondBytes := payloads[1].Content
	closeAgentInboxPayloads(payloads)
	if !allAgentInboxBytesZero(firstBytes) || !allAgentInboxBytesZero(secondBytes) {
		t.Fatal("runtime input plaintext was not zeroized")
	}
}

func TestAgentInboxCoordinatorConsumesNextQueueOnlyAfterTurnAuthority(t *testing.T) {
	ctx, _, loops, authority, binding, budget := agentInboxFixture(t, "coordinator-runtime-queue")
	startAgentInboxTurn(t, ctx, loops, binding, budget, "turn-1", 1)
	finishAgentInboxTurn(t, ctx, loops, binding, "turn-1", "step-1")
	store := newMemoryAgentInboxStore()
	coordinator, _ := NewAgentInboxCoordinator(authority, store)
	queued := agentInboxPayloadWithContent(binding, "input-queue", 1, "continue with the accepted plan")
	queued.Binding.Mode = agentinbox.ModeQueue
	queued.Binding.ContextScope = agentinbox.ScopeConversationShared
	queued.Binding.TargetTurnID, queued.Binding.TargetTurnSequence = "turn-2", 2
	if _, err := coordinator.Admit(ctx, binding, queued); err != nil {
		t.Fatal(err)
	}

	payload, state, err := coordinator.ConsumeQueuedTurn(ctx, binding, queued.Binding)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Status != agentinbox.StatusConsumed ||
		string(payload.Content) != "continue with the accepted plan" ||
		len(state.Loop.Turns) != 2 || state.Loop.Turns[1].TurnID != "turn-2" {
		t.Fatalf("runtime queue payload/state = %#v / %#v", payload, state)
	}
	content := payload.Content
	payload.Close()
	if !allAgentInboxBytesZero(content) {
		t.Fatal("queued runtime plaintext was not zeroized")
	}
}

func allAgentInboxBytesZero(content []byte) bool {
	for _, value := range content {
		if value != 0 {
			return false
		}
	}
	return true
}

type memoryAgentInboxStore struct {
	mu       sync.Mutex
	payloads map[string]agentinbox.Payload
}

func newMemoryAgentInboxStore() *memoryAgentInboxStore {
	return &memoryAgentInboxStore{payloads: make(map[string]agentinbox.Payload)}
}

func (store *memoryAgentInboxStore) PutAgentInput(_ context.Context, payload agentinbox.Payload) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if existing, found := store.payloads[payload.Binding.PayloadID]; found {
		if existing.Binding != payload.Binding || string(existing.Content) != string(payload.Content) {
			return errors.New("payload conflict")
		}
		return nil
	}
	payload.Content = append([]byte(nil), payload.Content...)
	store.payloads[payload.Binding.PayloadID] = payload
	return nil
}

func (store *memoryAgentInboxStore) ReadAgentInput(_ context.Context, binding agentinbox.Binding) (agentinbox.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return agentinbox.Payload{}, errors.New("payload missing")
	}
	payload.Content = append([]byte(nil), payload.Content...)
	return payload, nil
}

func (store *memoryAgentInboxStore) ListPendingAgentInputs(
	_ context.Context, conversationID, runID, agentInstanceID string, generation int64,
) ([]agentinbox.Payload, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	results := make([]agentinbox.Payload, 0)
	for _, payload := range store.payloads {
		binding := payload.Binding
		if payload.Status == agentinbox.StatusPending &&
			binding.ConversationID == conversationID && binding.RunID == runID &&
			binding.AgentInstanceID == agentInstanceID && binding.ClaimGeneration == generation {
			payload.Content = append([]byte(nil), payload.Content...)
			results = append(results, payload)
		}
	}
	return results, nil
}

func (store *memoryAgentInboxStore) MarkAgentInputConsumed(_ context.Context, binding agentinbox.Binding) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return errors.New("payload missing")
	}
	payload.Status = agentinbox.StatusConsumed
	store.payloads[binding.PayloadID] = payload
	return nil
}

func (store *memoryAgentInboxStore) DeleteAgentInput(_ context.Context, binding agentinbox.Binding) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	payload, found := store.payloads[binding.PayloadID]
	if !found || payload.Binding != binding {
		return errors.New("payload missing")
	}
	payload.Close()
	delete(store.payloads, binding.PayloadID)
	return nil
}

func (store *memoryAgentInboxStore) count() int {
	store.mu.Lock()
	defer store.mu.Unlock()
	return len(store.payloads)
}

func agentInboxPayloadWithContent(
	binding AttemptLoopBinding,
	inputID string,
	order int64,
	content string,
) agentinbox.Payload {
	frozen := agentInboxPayloadFixture(binding, "segment-1", inputID, order)
	digest := sha256.Sum256([]byte(content))
	frozen.ContentDigest = hex.EncodeToString(digest[:])
	return agentinbox.Payload{
		Binding: frozen, Status: agentinbox.StatusPending, Content: []byte(content),
	}
}
