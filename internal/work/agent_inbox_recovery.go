package work

import (
	"context"
	"errors"
	"sort"
	"strings"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/journal"
)

type AgentAttemptRestartDisposition string

const (
	AgentAttemptRestartPreModelResume           AgentAttemptRestartDisposition = "pre_model_resume"
	AgentAttemptRestartProviderOutcomeUncertain AgentAttemptRestartDisposition = "provider_outcome_uncertain"
	AgentAttemptRestartRecoveryBlocked          AgentAttemptRestartDisposition = "recovery_blocked"
)

// AgentAttemptRestartOutcome is a content-free governance projection. It never
// authorizes execution or reconstructs an in-process Runtime.
type AgentAttemptRestartOutcome struct {
	Binding          AttemptLoopBinding
	Budget           AttemptLoopBudget
	ExecutionBinding FrozenExecutionBinding
	Disposition      AgentAttemptRestartDisposition
	SegmentID        string
	TurnID           string
	TurnSequence     int
	StepID           string
	StepSequence     int
	CheckpointDigest string
	InputIDs         []string
	ModelRequestID   string
	ErrorCode        string
	Retryable        bool
}

type AgentAttemptRestartReport struct {
	Outcomes []AgentAttemptRestartOutcome
}

type restartAttemptLoopSnapshot struct {
	loop      AttemptLoopSnapshot
	execution FrozenExecutionBinding
}

// RecoverAfterRestart aligns encrypted Inbox storage with Journal authority and
// reconstructs only safe recovery metadata. A caller must use a separate,
// explicit resume authority before any Runtime or Provider dispatch.
func (coordinator *AgentInboxCoordinator) RecoverAfterRestart(
	ctx context.Context,
) (AgentAttemptRestartReport, error) {
	if coordinator == nil || coordinator.authority == nil || ctx == nil || ctx.Err() != nil {
		return AgentAttemptRestartReport{}, ErrInvalidAgentInboxCoordinator
	}
	snapshots, err := coordinator.authority.loops.restartSnapshots(ctx)
	if err != nil {
		return AgentAttemptRestartReport{}, err
	}
	report := AgentAttemptRestartReport{
		Outcomes: make([]AgentAttemptRestartOutcome, 0, len(snapshots)),
	}
	for _, recovered := range snapshots {
		binding := recovered.loop.Binding
		if _, err := coordinator.Snapshot(ctx, binding); err != nil {
			// Authority replay is protected core and cannot be isolated as a
			// storage fault.
			return AgentAttemptRestartReport{}, err
		}
		if err := coordinator.Reconcile(ctx, binding); err != nil {
			if ctx.Err() != nil ||
				!errors.Is(err, ErrAgentInboxPersistence) &&
					!errors.Is(err, ErrAgentInboxConflict) {
				return AgentAttemptRestartReport{}, err
			}
			errorCode := "agent_input_recovery_unavailable"
			if errors.Is(err, ErrAgentInboxConflict) {
				errorCode = "agent_input_recovery_conflict"
			}
			report.Outcomes = append(report.Outcomes, agentAttemptRestartBlocked(
				recovered, errorCode,
			))
			continue
		}
		state, err := coordinator.Snapshot(ctx, binding)
		if err != nil {
			return AgentAttemptRestartReport{}, err
		}
		outcome, found, err := classifyAgentAttemptRestart(state, recovered.execution)
		if err != nil {
			return AgentAttemptRestartReport{}, err
		}
		if found {
			report.Outcomes = append(report.Outcomes, outcome)
		}
	}
	sort.Slice(report.Outcomes, func(i, j int) bool {
		left, right := report.Outcomes[i], report.Outcomes[j]
		if left.Binding.PayloadAuthority.RunID != right.Binding.PayloadAuthority.RunID {
			return left.Binding.PayloadAuthority.RunID < right.Binding.PayloadAuthority.RunID
		}
		return left.Disposition < right.Disposition
	})
	return report, nil
}

// RestoreAgentAttemptInputs releases the exact consumed Inbox payloads for a
// still-current pre-model recovery candidate. The caller must already hold the
// separate consumed recovery dispatch grant and owns the returned plaintext.
func (coordinator *AgentInboxCoordinator) RestoreAgentAttemptInputs(
	ctx context.Context,
	outcome AgentAttemptRestartOutcome,
) ([]agentinbox.Payload, error) {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		!validAgentAttemptRestartPreModelOutcome(outcome) {
		return nil, ErrInvalidAgentInboxCoordinator
	}
	state, err := coordinator.Snapshot(ctx, outcome.Binding)
	if err != nil {
		return nil, err
	}
	current, found, err := classifyAgentAttemptRestart(state, outcome.ExecutionBinding)
	if err != nil || !found || current.Disposition != AgentAttemptRestartPreModelResume ||
		AgentAttemptRestartCandidateDigest(current) != AgentAttemptRestartCandidateDigest(outcome) {
		return nil, errors.Join(ErrAgentInboxConflict, err)
	}
	wanted := make(map[string]struct{}, len(outcome.InputIDs))
	for _, inputID := range outcome.InputIDs {
		if _, duplicate := wanted[inputID]; duplicate {
			return nil, ErrAgentInboxConflict
		}
		wanted[inputID] = struct{}{}
	}
	records := make([]AgentInboxInputRecord, 0, len(wanted))
	for _, record := range state.Inputs {
		if _, ok := wanted[record.InputID]; !ok {
			continue
		}
		if record.Status != agentinbox.StatusConsumed ||
			record.SegmentID != outcome.SegmentID {
			return nil, ErrAgentInboxConflict
		}
		records = append(records, record)
	}
	if len(records) != len(wanted) {
		return nil, ErrAgentInboxConflict
	}
	payloads, err := coordinator.readConsumedPayloads(ctx, records)
	if err != nil {
		return nil, err
	}
	for index := range payloads {
		if payloads[index].Binding.SegmentID != outcome.SegmentID {
			closeAgentInboxPayloads(payloads)
			return nil, ErrAgentInboxConflict
		}
	}
	return payloads, nil
}

func agentAttemptRestartBlocked(
	recovered restartAttemptLoopSnapshot,
	errorCode string,
) AgentAttemptRestartOutcome {
	loop := recovered.loop
	outcome := AgentAttemptRestartOutcome{
		Binding: loop.Binding, Budget: loop.Budget,
		ExecutionBinding: recovered.execution,
		Disposition:      AgentAttemptRestartRecoveryBlocked,
		ErrorCode:        errorCode,
	}
	if len(loop.Turns) == 0 {
		return outcome
	}
	turn := loop.Turns[len(loop.Turns)-1]
	outcome.TurnID, outcome.TurnSequence = turn.TurnID, turn.Sequence
	if len(turn.Steps) != 0 {
		step := turn.Steps[len(turn.Steps)-1]
		outcome.StepID, outcome.StepSequence = step.StepID, step.Sequence
		outcome.ModelRequestID = step.ModelRequestID
	}
	return outcome
}

func (authority *AttemptLoopAuthority) restartSnapshots(
	ctx context.Context,
) ([]restartAttemptLoopSnapshot, error) {
	if authority == nil || authority.runs == nil || authority.payloads == nil ||
		ctx == nil || ctx.Err() != nil {
		return nil, ErrInvalidAttemptLoop
	}
	events, err := authority.runs.store.ReadAll(ctx)
	if err != nil {
		return nil, errors.Join(ErrAttemptLoopAuthority, err)
	}
	runSnapshot, err := authority.runs.Snapshot(ctx)
	if err != nil {
		return nil, errors.Join(ErrAttemptLoopAuthority, err)
	}
	currentRuns := make(map[string]RunRecord, len(runSnapshot.Runs()))
	for _, run := range runSnapshot.Runs() {
		currentRuns[run.ID()] = run
	}
	streams := make(map[string][]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "attempt-loop/") {
			streams[event.StreamID] = append(streams[event.StreamID], event)
		}
	}
	streamIDs := make([]string, 0, len(streams))
	for streamID := range streams {
		streamIDs = append(streamIDs, streamID)
	}
	sort.Strings(streamIDs)
	results := make([]restartAttemptLoopSnapshot, 0, len(streamIDs))
	for _, streamID := range streamIDs {
		loop, err := replayFrozenAttemptLoop(streams[streamID])
		if err != nil {
			return nil, err
		}
		frozen := loop.Binding.PayloadAuthority
		current, found := currentRuns[frozen.RunID]
		if !found {
			return nil, ErrAttemptLoopConflict
		}
		if current.ClaimGeneration() > frozen.ClaimGeneration {
			// A previous generation remains immutable history, not a resumable
			// process in the new daemon generation.
			continue
		}
		if current.ClaimGeneration() != frozen.ClaimGeneration ||
			current.ClaimID() != frozen.ClaimID ||
			current.RuntimeInstanceID() != frozen.RuntimeInstanceID ||
			current.AgentInstanceID() != frozen.AgentInstanceID ||
			current.ExecutionBinding().BindingDigest != frozen.ExecutionBindingDigest {
			return nil, ErrAttemptLoopConflict
		}
		run, err := authority.payloads.exactRun(ctx, loop.Binding.PayloadAuthority)
		if err != nil {
			return nil, err
		}
		if err := validateAttemptLoopBindingRun(loop.Binding, run); err != nil {
			return nil, err
		}
		if current.Phase() != "running" {
			continue
		}
		results = append(results, restartAttemptLoopSnapshot{
			loop: loop, execution: run.ExecutionBinding(),
		})
	}
	return results, nil
}

func replayFrozenAttemptLoop(events []journal.Event) (AttemptLoopSnapshot, error) {
	if len(events) == 0 || events[0].Type != "AttemptLoopStarted" ||
		events[0].Seq != 1 || events[0].SchemaVersion != 1 {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	var payload attemptLoopEventPayload
	if decodeExactPayload(events[0].PayloadJSON, &payload) != nil || payload.Budget == nil {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	binding := AttemptLoopBinding{
		SchemaVersion:  payload.SchemaVersion,
		AttemptID:      payload.AttemptID,
		TeamInstanceID: payload.TeamInstanceID,
		PayloadAuthority: attemptpayload.Authority{
			Scope: attemptpayload.Scope{
				ConversationID: payload.ConversationID,
				WorkItemID:     payload.WorkItemID, RunID: payload.RunID,
				ClaimGeneration:        payload.ClaimGeneration,
				RuntimeInstanceID:      payload.RuntimeInstanceID,
				ExecutionBindingDigest: payload.ExecutionBindingDigest,
				CapsuleDigest:          payload.ContextCapsuleDigest,
			},
			ClaimID: payload.ClaimID, AgentInstanceID: payload.AgentInstanceID,
			IncidentID: events[0].CorrelationID,
		},
		PermissionProfileID:         payload.PermissionProfileID,
		PermissionProfileGeneration: payload.PermissionProfileGeneration,
		PermissionProfileDigest:     payload.PermissionProfileDigest,
		CapabilitySetDigest:         payload.CapabilitySetDigest,
		ToolSchemaSetDigest:         payload.ToolSchemaSetDigest,
		BudgetPolicyVersion:         payload.BudgetPolicyVersion,
	}
	if !validAttemptLoopBinding(binding) || !validAttemptLoopBudget(*payload.Budget) ||
		events[0].StreamID != attemptLoopStream(binding) {
		return AttemptLoopSnapshot{}, ErrAttemptLoopConflict
	}
	return replayAttemptLoop(binding, events)
}

func classifyAgentAttemptRestart(
	state AgentInboxSnapshot,
	execution FrozenExecutionBinding,
) (AgentAttemptRestartOutcome, bool, error) {
	loop := state.Loop
	if loop.Status != AttemptLoopRunning || len(loop.Turns) == 0 {
		return AgentAttemptRestartOutcome{}, false, nil
	}
	turn := loop.Turns[len(loop.Turns)-1]
	if turn.Status != "" {
		return AgentAttemptRestartOutcome{}, false, nil
	}
	base := AgentAttemptRestartOutcome{
		Binding: loop.Binding, Budget: loop.Budget, ExecutionBinding: execution,
		TurnID: turn.TurnID, TurnSequence: turn.Sequence,
	}
	if len(turn.Steps) == 0 {
		inputIDs, ok := restartConsumedQueueIDs(state, turn)
		checkpoint, checkpointOK := restartPreviousTurnCheckpoint(loop, turn.Sequence)
		if !ok || !checkpointOK {
			return AgentAttemptRestartOutcome{}, false, nil
		}
		segmentID, segmentOK := restartInputSegmentID(state, inputIDs)
		if !segmentOK {
			return agentAttemptRestartSegmentBlocked(base), true, nil
		}
		base.Disposition = AgentAttemptRestartPreModelResume
		base.SegmentID = segmentID
		base.CheckpointDigest, base.InputIDs = checkpoint, inputIDs
		return base, true, nil
	}
	step := turn.Steps[len(turn.Steps)-1]
	base.StepID, base.StepSequence = step.StepID, step.Sequence
	if step.ModelRequestID != "" && step.Outcome == "" {
		base.Disposition = AgentAttemptRestartProviderOutcomeUncertain
		base.ModelRequestID = step.ModelRequestID
		base.InputIDs = restartCurrentInputIDs(state, turn, step)
		return base, true, nil
	}
	if step.ModelRequestID != "" || step.Outcome != "" {
		return AgentAttemptRestartOutcome{}, false, nil
	}
	var inputIDs []string
	var checkpoint string
	var ok bool
	if step.Sequence == 1 {
		inputIDs, ok = restartConsumedQueueIDs(state, turn)
		checkpoint, _ = restartPreviousTurnCheckpoint(loop, turn.Sequence)
	} else {
		inputIDs, ok = restartConsumedStepIDs(state, step)
		checkpoint, _ = restartPreviousStepCheckpoint(turn, step.Sequence)
	}
	if !ok || !validSHA256Hex(checkpoint) {
		return AgentAttemptRestartOutcome{}, false, nil
	}
	segmentID, segmentOK := restartInputSegmentID(state, inputIDs)
	if !segmentOK {
		return agentAttemptRestartSegmentBlocked(base), true, nil
	}
	base.Disposition = AgentAttemptRestartPreModelResume
	base.SegmentID = segmentID
	base.CheckpointDigest, base.InputIDs = checkpoint, inputIDs
	return base, true, nil
}

func restartInputSegmentID(state AgentInboxSnapshot, inputIDs []string) (string, bool) {
	if len(inputIDs) == 0 {
		return "", false
	}
	wanted := make(map[string]struct{}, len(inputIDs))
	for _, inputID := range inputIDs {
		wanted[inputID] = struct{}{}
	}
	segmentID := ""
	found := make(map[string]struct{}, len(inputIDs))
	for _, record := range state.Inputs {
		if _, ok := wanted[record.InputID]; !ok {
			continue
		}
		if !validOpaqueID(record.SegmentID) || segmentID != "" && record.SegmentID != segmentID {
			return "", false
		}
		segmentID = record.SegmentID
		found[record.InputID] = struct{}{}
	}
	return segmentID, segmentID != "" && len(found) == len(wanted)
}

func agentAttemptRestartSegmentBlocked(base AgentAttemptRestartOutcome) AgentAttemptRestartOutcome {
	base.Disposition = AgentAttemptRestartRecoveryBlocked
	base.ErrorCode = "agent_input_recovery_conflict"
	base.CheckpointDigest = ""
	base.InputIDs = nil
	return base
}

func restartCurrentInputIDs(
	state AgentInboxSnapshot,
	turn AttemptLoopTurnRecord,
	step AttemptLoopStepRecord,
) []string {
	if step.Sequence == 1 {
		ids, _ := restartConsumedQueueIDs(state, turn)
		return ids
	}
	ids, _ := restartConsumedStepIDs(state, step)
	return ids
}

func restartConsumedQueueIDs(
	state AgentInboxSnapshot,
	turn AttemptLoopTurnRecord,
) ([]string, bool) {
	var ids []string
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusConsumed && record.Mode == agentinbox.ModeQueue &&
			record.TargetTurnID == turn.TurnID && record.TargetTurnSequence == turn.Sequence {
			ids = append(ids, record.InputID)
		}
	}
	return ids, len(ids) == 1
}

func restartConsumedStepIDs(
	state AgentInboxSnapshot,
	step AttemptLoopStepRecord,
) ([]string, bool) {
	var ids []string
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusConsumed &&
			(record.Mode == agentinbox.ModeSteer || record.Mode == agentinbox.ModeInject) &&
			record.TargetStepID == step.StepID && record.TargetStepSequence == step.Sequence {
			ids = append(ids, record.InputID)
		}
	}
	return ids, len(ids) != 0
}

func restartPreviousTurnCheckpoint(
	loop AttemptLoopSnapshot,
	turnSequence int,
) (string, bool) {
	for _, turn := range loop.Turns {
		if turn.Sequence != turnSequence-1 || turn.Status != AttemptTurnSucceeded ||
			len(turn.Steps) == 0 {
			continue
		}
		step := turn.Steps[len(turn.Steps)-1]
		return step.OutputDigest, step.Outcome == AttemptStepFinal &&
			validSHA256Hex(step.OutputDigest)
	}
	return "", false
}

func restartPreviousStepCheckpoint(
	turn AttemptLoopTurnRecord,
	stepSequence int,
) (string, bool) {
	for _, step := range turn.Steps {
		if step.Sequence == stepSequence-1 {
			return step.OutputDigest, step.Outcome == AttemptStepContinue &&
				validSHA256Hex(step.OutputDigest)
		}
	}
	return "", false
}
