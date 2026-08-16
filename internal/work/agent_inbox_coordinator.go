package work

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"

	"loom-pi-rebuild/internal/agentinbox"
)

var (
	ErrInvalidAgentInboxCoordinator = errors.New("invalid Agent inbox coordinator")
	ErrAgentInboxPersistence        = errors.New("Agent inbox encrypted payload persistence failed")
)

// AgentInboxCoordinator is the only public mutation surface. It keeps content
// in the encrypted store and commits only immutable metadata to the Journal.
type AgentInboxCoordinator struct {
	authority *AgentInboxAuthority
	store     agentinbox.Store
}

func NewAgentInboxCoordinator(
	authority *AgentInboxAuthority,
	store agentinbox.Store,
) (*AgentInboxCoordinator, error) {
	if authority == nil || authority.loops == nil || store == nil {
		return nil, ErrInvalidAgentInboxCoordinator
	}
	return &AgentInboxCoordinator{authority: authority, store: store}, nil
}

func (coordinator *AgentInboxCoordinator) Admit(
	ctx context.Context,
	binding AttemptLoopBinding,
	payload agentinbox.Payload,
) (AgentInboxSnapshot, error) {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		payload.Status != agentinbox.StatusPending || len(payload.Content) == 0 {
		payload.Close()
		return AgentInboxSnapshot{}, ErrInvalidAgentInboxCoordinator
	}
	defer payload.Close()
	if err := coordinator.store.PutAgentInput(ctx, payload); err != nil {
		return AgentInboxSnapshot{}, errors.Join(ErrAgentInboxPersistence, err)
	}
	state, err := coordinator.authority.admit(ctx, binding, payload.Binding)
	if err == nil {
		return state, nil
	}
	committed, snapshotErr := coordinator.authority.Snapshot(ctx, binding)
	if snapshotErr != nil {
		// The commit outcome is unknown. Keep pending ciphertext so a later
		// reconciliation can compare it with authoritative facts.
		return AgentInboxSnapshot{}, errors.Join(
			ErrAgentInboxPersistence, err, snapshotErr,
		)
	}
	if record, found := agentInboxInputByID(committed, payload.Binding.InputID); found && record.Binding == payload.Binding {
		return committed, nil
	}
	deleteErr := coordinator.store.DeleteAgentInput(ctx, payload.Binding)
	if deleteErr != nil {
		return AgentInboxSnapshot{}, errors.Join(
			ErrAgentInboxPersistence, err, snapshotErr, deleteErr,
		)
	}
	return AgentInboxSnapshot{}, err
}

func (coordinator *AgentInboxCoordinator) StartQueuedTurn(
	ctx context.Context,
	binding AttemptLoopBinding,
	input agentinbox.Binding,
	turn AttemptLoopTurnInput,
) (AgentInboxSnapshot, error) {
	if coordinator == nil {
		return AgentInboxSnapshot{}, ErrInvalidAgentInboxCoordinator
	}
	state, err := coordinator.authority.startQueuedTurn(ctx, binding, input, turn)
	if err != nil {
		return AgentInboxSnapshot{}, err
	}
	if err := coordinator.store.MarkAgentInputConsumed(ctx, input); err != nil {
		return state, errors.Join(ErrAgentInboxPersistence, err)
	}
	return state, nil
}

func (coordinator *AgentInboxCoordinator) StartTargetStep(
	ctx context.Context,
	binding AttemptLoopBinding,
	inputs []agentinbox.Binding,
	step AttemptLoopStepInput,
) (AgentInboxSnapshot, error) {
	if coordinator == nil {
		return AgentInboxSnapshot{}, ErrInvalidAgentInboxCoordinator
	}
	state, err := coordinator.authority.startTargetStep(ctx, binding, inputs, step)
	if err != nil {
		return AgentInboxSnapshot{}, err
	}
	for _, input := range inputs {
		if err := coordinator.store.MarkAgentInputConsumed(ctx, input); err != nil {
			return state, errors.Join(ErrAgentInboxPersistence, err)
		}
	}
	return state, nil
}

// Snapshot returns only immutable Inbox and Attempt-loop metadata. Encrypted
// content remains behind the coordinator's consumption boundary.
func (coordinator *AgentInboxCoordinator) Snapshot(
	ctx context.Context,
	binding AttemptLoopBinding,
) (AgentInboxSnapshot, error) {
	if coordinator == nil || ctx == nil || ctx.Err() != nil {
		return AgentInboxSnapshot{}, ErrInvalidAgentInboxCoordinator
	}
	return coordinator.authority.Snapshot(ctx, binding)
}

// ConsumeTargetStep releases ordered plaintext only after the exact Inbox
// records and Step transition are authoritative and storage status is aligned.
// The caller owns the returned payloads and must close every one.
func (coordinator *AgentInboxCoordinator) ConsumeTargetStep(
	ctx context.Context,
	binding AttemptLoopBinding,
	inputs []agentinbox.Binding,
	step AttemptLoopStepInput,
) ([]agentinbox.Payload, AgentInboxSnapshot, error) {
	if coordinator == nil || ctx == nil || ctx.Err() != nil || len(inputs) == 0 {
		return nil, AgentInboxSnapshot{}, ErrInvalidAgentInboxCoordinator
	}
	state, err := coordinator.authority.Snapshot(ctx, binding)
	if err != nil {
		return nil, AgentInboxSnapshot{}, err
	}
	records, allConsumed, err := selectAgentInboxStepInputs(state, inputs, step)
	if err != nil {
		return nil, AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	if allConsumed {
		existing, found := attemptLoopStepByID(state.Loop, step.StepID)
		if !found || existing.Sequence != step.Sequence ||
			existing.ModelInputDigest != step.ModelInputDigest ||
			existing.ModelRequestID != "" || existing.Outcome != "" {
			return nil, AgentInboxSnapshot{}, ErrAgentInboxConflict
		}
		payloads, readErr := coordinator.readConsumedPayloads(ctx, records)
		return payloads, state, readErr
	}
	payloads, err := coordinator.readPendingPayloads(ctx, records)
	if err != nil {
		return nil, AgentInboxSnapshot{}, err
	}
	consumed, err := coordinator.StartTargetStep(ctx, binding, inputs, step)
	if err != nil {
		closeAgentInboxPayloads(payloads)
		return nil, consumed, err
	}
	for index := range payloads {
		payloads[index].Status = agentinbox.StatusConsumed
	}
	return payloads, consumed, nil
}

// ConsumeQueuedTurn releases one queued plaintext only after it atomically
// starts its frozen next Turn and storage records the same consumption fact.
func (coordinator *AgentInboxCoordinator) ConsumeQueuedTurn(
	ctx context.Context,
	binding AttemptLoopBinding,
	input agentinbox.Binding,
) (agentinbox.Payload, AgentInboxSnapshot, error) {
	if coordinator == nil || ctx == nil || ctx.Err() != nil ||
		input.Mode != agentinbox.ModeQueue {
		return agentinbox.Payload{}, AgentInboxSnapshot{}, ErrInvalidAgentInboxCoordinator
	}
	state, err := coordinator.authority.Snapshot(ctx, binding)
	if err != nil {
		return agentinbox.Payload{}, AgentInboxSnapshot{}, err
	}
	record, found := agentInboxInputByID(state, input.InputID)
	if !found || record.Binding != input {
		return agentinbox.Payload{}, AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	if record.Status == agentinbox.StatusConsumed {
		turn, exists := attemptLoopTurnByID(state.Loop, input.TargetTurnID)
		if !exists || turn.Sequence != input.TargetTurnSequence ||
			turn.InputDigest != input.ContentDigest || turn.Status != "" ||
			len(turn.Steps) > 1 ||
			(len(turn.Steps) == 1 &&
				(turn.Steps[0].Sequence != 1 || turn.Steps[0].ModelRequestID != "" ||
					turn.Steps[0].Outcome != "")) {
			return agentinbox.Payload{}, AgentInboxSnapshot{}, ErrAgentInboxConflict
		}
		payloads, readErr := coordinator.readConsumedPayloads(
			ctx, []AgentInboxInputRecord{record},
		)
		if readErr != nil {
			return agentinbox.Payload{}, AgentInboxSnapshot{}, readErr
		}
		return payloads[0], state, nil
	}
	if record.Status != agentinbox.StatusPending ||
		!agentInboxQueueIsNext(state, input.InputID) {
		return agentinbox.Payload{}, AgentInboxSnapshot{}, ErrAgentInboxConflict
	}
	payloads, err := coordinator.readPendingPayloads(ctx, []AgentInboxInputRecord{record})
	if err != nil {
		return agentinbox.Payload{}, AgentInboxSnapshot{}, err
	}
	payload := payloads[0]
	consumed, err := coordinator.StartQueuedTurn(ctx, binding, input, AttemptLoopTurnInput{
		TurnID: input.TargetTurnID, Sequence: input.TargetTurnSequence,
		InputDigest: input.ContentDigest,
	})
	if err != nil {
		payload.Close()
		return agentinbox.Payload{}, consumed, err
	}
	payload.Status = agentinbox.StatusConsumed
	return payload, consumed, nil
}

func (coordinator *AgentInboxCoordinator) readPendingPayloads(
	ctx context.Context,
	records []AgentInboxInputRecord,
) ([]agentinbox.Payload, error) {
	ordered := append([]AgentInboxInputRecord(nil), records...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].OrderKey < ordered[right].OrderKey
	})
	payloads := make([]agentinbox.Payload, 0, len(ordered))
	for _, record := range ordered {
		if record.Status != agentinbox.StatusPending {
			closeAgentInboxPayloads(payloads)
			return nil, ErrAgentInboxConflict
		}
		payload, err := coordinator.store.ReadAgentInput(ctx, record.Binding)
		if err != nil {
			closeAgentInboxPayloads(payloads)
			return nil, errors.Join(ErrAgentInboxPersistence, err)
		}
		digest := sha256.Sum256(payload.Content)
		if payload.Binding != record.Binding || payload.Status != agentinbox.StatusPending ||
			len(payload.Content) == 0 ||
			hex.EncodeToString(digest[:]) != record.ContentDigest {
			payload.Close()
			closeAgentInboxPayloads(payloads)
			return nil, ErrAgentInboxConflict
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

func (coordinator *AgentInboxCoordinator) readConsumedPayloads(
	ctx context.Context,
	records []AgentInboxInputRecord,
) ([]agentinbox.Payload, error) {
	ordered := append([]AgentInboxInputRecord(nil), records...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].OrderKey < ordered[right].OrderKey
	})
	payloads := make([]agentinbox.Payload, 0, len(ordered))
	for _, record := range ordered {
		if record.Status != agentinbox.StatusConsumed {
			closeAgentInboxPayloads(payloads)
			return nil, ErrAgentInboxConflict
		}
		if err := coordinator.store.MarkAgentInputConsumed(ctx, record.Binding); err != nil {
			closeAgentInboxPayloads(payloads)
			return nil, errors.Join(ErrAgentInboxPersistence, err)
		}
		payload, err := coordinator.store.ReadAgentInput(ctx, record.Binding)
		if err != nil {
			closeAgentInboxPayloads(payloads)
			return nil, errors.Join(ErrAgentInboxPersistence, err)
		}
		digest := sha256.Sum256(payload.Content)
		if payload.Binding != record.Binding || payload.Status != agentinbox.StatusConsumed ||
			len(payload.Content) == 0 ||
			hex.EncodeToString(digest[:]) != record.ContentDigest {
			payload.Close()
			closeAgentInboxPayloads(payloads)
			return nil, ErrAgentInboxConflict
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

// Reconcile repairs only storage status. It never invents authority facts or
// changes a frozen target. Unadmitted pending ciphertext is removed.
func (coordinator *AgentInboxCoordinator) Reconcile(
	ctx context.Context,
	binding AttemptLoopBinding,
) error {
	if coordinator == nil || ctx == nil || ctx.Err() != nil {
		return ErrInvalidAgentInboxCoordinator
	}
	state, err := coordinator.authority.Snapshot(ctx, binding)
	if err != nil {
		return err
	}
	pending, err := coordinator.store.ListPendingAgentInputs(
		ctx, binding.PayloadAuthority.ConversationID,
		binding.PayloadAuthority.RunID,
		binding.PayloadAuthority.AgentInstanceID,
		binding.PayloadAuthority.ClaimGeneration,
	)
	if err != nil {
		return errors.Join(ErrAgentInboxPersistence, err)
	}
	defer closeAgentInboxPayloads(pending)
	for index := range pending {
		payload := &pending[index]
		record, found := agentInboxInputByID(state, payload.Binding.InputID)
		if !found {
			if err := coordinator.store.DeleteAgentInput(ctx, payload.Binding); err != nil {
				return errors.Join(ErrAgentInboxPersistence, err)
			}
			continue
		}
		if record.Binding != payload.Binding {
			return ErrAgentInboxConflict
		}
		if record.Status == agentinbox.StatusConsumed {
			if err := coordinator.store.MarkAgentInputConsumed(ctx, payload.Binding); err != nil {
				return errors.Join(ErrAgentInboxPersistence, err)
			}
		}
	}
	for _, record := range state.Inputs {
		payload, readErr := coordinator.store.ReadAgentInput(ctx, record.Binding)
		if readErr != nil {
			return errors.Join(ErrAgentInboxPersistence, readErr)
		}
		status := payload.Status
		payload.Close()
		if status != record.Status {
			return ErrAgentInboxConflict
		}
	}
	return nil
}

func closeAgentInboxPayloads(payloads []agentinbox.Payload) {
	for index := range payloads {
		payloads[index].Close()
	}
}
