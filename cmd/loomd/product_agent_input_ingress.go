package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/work"
)

const (
	productAgentInputSchemaVersion = 1
	productAgentInputMaxBytes      = 32 * 1024
	productAgentInputStage         = "agent_input_admission"
)

var (
	errProductInvalidAgentInput  = errors.New("invalid Agent input")
	errProductAgentInputConflict = errors.New("Agent input conflict")
)

type productAgentInputRequest struct {
	SchemaVersion   int                     `json:"schema_version"`
	SegmentID       string                  `json:"segment_id"`
	AgentInstanceID string                  `json:"agent_instance_id"`
	WorkItemID      string                  `json:"work_item_id"`
	RunID           string                  `json:"run_id"`
	ClaimGeneration int64                   `json:"claim_generation"`
	Mode            agentinbox.Mode         `json:"mode"`
	ContextScope    agentinbox.ContextScope `json:"context_scope"`
	ScopeTargetID   string                  `json:"scope_target_id,omitempty"`
	Content         []byte                  `json:"content"`
	IncidentID      string                  `json:"-"`
}

type productAgentInputReceipt struct {
	SchemaVersion      int             `json:"schema_version"`
	IncidentID         string          `json:"incident_id"`
	InputID            string          `json:"input_id"`
	Mode               agentinbox.Mode `json:"mode"`
	OrderKey           int64           `json:"order_key"`
	TargetTurnID       string          `json:"target_turn_id,omitempty"`
	TargetTurnSequence int             `json:"target_turn_sequence,omitempty"`
	TargetStepID       string          `json:"target_step_id,omitempty"`
	TargetStepSequence int             `json:"target_step_sequence,omitempty"`
}

type productAgentInputRoute interface {
	AdmitAgentInput(context.Context, productAgentInputRequest) (productAgentInputReceipt, error)
}

type productAgentInputIngress struct {
	mu          sync.Mutex
	active      *productActiveAttemptRegistry
	inbox       *work.AgentInboxCoordinator
	diagnostics nativeadapter.AgentAttemptDiagnosticRecorder
	now         func() time.Time
}

func newProductAgentInputIngress(
	active *productActiveAttemptRegistry,
	inbox *work.AgentInboxCoordinator,
	diagnostics nativeadapter.AgentAttemptDiagnosticRecorder,
	now func() time.Time,
) (*productAgentInputIngress, error) {
	if active == nil || inbox == nil || nilProductAgentInterface(diagnostics) || now == nil {
		return nil, errProductInvalidAgentInput
	}
	current := now()
	if current.IsZero() || current.Location() != time.UTC {
		return nil, errProductInvalidAgentInput
	}
	return &productAgentInputIngress{
		active: active, inbox: inbox, diagnostics: diagnostics, now: now,
	}, nil
}

func (ingress *productAgentInputIngress) AdmitAgentInput(
	ctx context.Context,
	request productAgentInputRequest,
) (receipt productAgentInputReceipt, resultErr error) {
	startedAt := time.Time{}
	if ingress != nil && ingress.now != nil {
		startedAt = ingress.now()
	}
	defer clearProductAgentInput(request.Content)
	if ingress == nil || ctx == nil || ctx.Err() != nil ||
		!validProductAgentInputRequest(request) || startedAt.IsZero() ||
		startedAt.Location() != time.UTC {
		return productAgentInputReceipt{}, errProductInvalidAgentInput
	}

	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	query := productActiveAttemptInputQuery{
		SegmentID: request.SegmentID, AgentInstanceID: request.AgentInstanceID,
		WorkItemID: request.WorkItemID, RunID: request.RunID,
		ClaimGeneration: request.ClaimGeneration,
	}
	active, err := ingress.active.ResolveAgentInput(query)
	if err != nil {
		ingress.record(ctx, startedAt, request, productActiveAttempt{}, "failed", agentInputErrorCode(err), agentInputRetryable(err))
		return productAgentInputReceipt{}, err
	}
	state, err := ingress.inbox.Snapshot(ctx, active.AttemptLoopBinding)
	if err != nil {
		ingress.record(ctx, startedAt, request, active, "failed", "state_unavailable", true)
		return productAgentInputReceipt{}, err
	}
	contentDigest := sha256.Sum256(request.Content)
	digest := hex.EncodeToString(contentDigest[:])
	inputID := "input-" + productDeterministicUUID(
		"agent-input", request.IncidentID, request.SegmentID,
		request.AgentInstanceID, request.WorkItemID, request.RunID,
		strconv.FormatInt(request.ClaimGeneration, 10),
	)
	if existing, found := productAgentInputByID(state, inputID); found {
		if !productAgentInputMatchesRequest(existing.Binding, request, digest) {
			ingress.record(ctx, startedAt, request, active, "failed", "conflict", true)
			return productAgentInputReceipt{}, errProductAgentInputConflict
		}
		receipt = productAgentInputReceiptFor(request.IncidentID, existing.Binding)
		ingress.record(ctx, startedAt, request, active, "succeeded", "", false)
		return receipt, nil
	}

	binding, err := freezeProductAgentInput(active, state, request, inputID, digest)
	if err != nil {
		ingress.record(ctx, startedAt, request, active, "failed", agentInputErrorCode(err), agentInputRetryable(err))
		return productAgentInputReceipt{}, err
	}
	payload := agentinbox.Payload{
		Binding: binding, Status: agentinbox.StatusPending, Content: request.Content,
	}
	if _, err := ingress.inbox.Admit(ctx, active.AttemptLoopBinding, payload); err != nil {
		mapped := err
		if errors.Is(err, work.ErrAgentInboxConflict) {
			mapped = errors.Join(errProductAgentInputConflict, err)
		}
		ingress.record(ctx, startedAt, request, active, "failed", agentInputErrorCode(mapped), agentInputRetryable(mapped))
		return productAgentInputReceipt{}, mapped
	}
	receipt = productAgentInputReceiptFor(request.IncidentID, binding)
	if err := ingress.record(ctx, startedAt, request, active, "succeeded", "", false); err != nil {
		return productAgentInputReceipt{}, err
	}
	return receipt, nil
}

func freezeProductAgentInput(
	active productActiveAttempt,
	state work.AgentInboxSnapshot,
	request productAgentInputRequest,
	inputID string,
	contentDigest string,
) (agentinbox.Binding, error) {
	loop := state.Loop
	if loop.Status != work.AttemptLoopRunning || loop.Binding != active.AttemptLoopBinding ||
		len(loop.Turns) == 0 {
		return agentinbox.Binding{}, errProductAgentInputConflict
	}
	binding := agentinbox.Binding{
		PayloadID: "payload-" + productDeterministicUUID("agent-input-payload", inputID),
		InputID:   inputID, Mode: request.Mode, ContextScope: request.ContextScope,
		ScopeTargetID:  request.ScopeTargetID,
		ConversationID: active.Identity.ConversationID,
		SegmentID:      request.SegmentID, AgentInstanceID: request.AgentInstanceID,
		WorkItemID: request.WorkItemID, RunID: request.RunID,
		ClaimGeneration:        request.ClaimGeneration,
		RuntimeInstanceID:      active.AttemptLoopBinding.PayloadAuthority.RuntimeInstanceID,
		ExecutionBindingDigest: active.AttemptLoopBinding.PayloadAuthority.ExecutionBindingDigest,
		CapsuleDigest:          active.CapsuleDigest, OrderKey: int64(len(state.Inputs) + 1),
		ContentType: "text/plain", ContentDigest: contentDigest,
	}
	switch request.Mode {
	case agentinbox.ModeQueue:
		pending := 0
		for _, input := range state.Inputs {
			if input.Status == agentinbox.StatusPending && input.Mode == agentinbox.ModeQueue {
				pending++
			}
		}
		binding.TargetTurnSequence = len(loop.Turns) + pending + 1
		if binding.TargetTurnSequence > loop.Budget.MaxTurns {
			return agentinbox.Binding{}, errProductAgentInputConflict
		}
		binding.TargetTurnID = "turn-" + productDeterministicUUID(
			"agent-input-turn", active.AttemptLoopBinding.AttemptID,
			strconv.Itoa(binding.TargetTurnSequence),
		)
	case agentinbox.ModeSteer, agentinbox.ModeInject:
		turn := loop.Turns[len(loop.Turns)-1]
		if turn.Status != "" || len(turn.Steps) == 0 {
			return agentinbox.Binding{}, errProductAgentInputConflict
		}
		lastStep := turn.Steps[len(turn.Steps)-1]
		if lastStep.Outcome != "" && lastStep.Outcome != work.AttemptStepContinue {
			return agentinbox.Binding{}, errProductAgentInputConflict
		}
		binding.TargetStepSequence = len(turn.Steps) + 1
		if binding.TargetStepSequence > loop.Budget.MaxStepsPerTurn {
			return agentinbox.Binding{}, errProductAgentInputConflict
		}
		for _, input := range state.Inputs {
			if input.Status == agentinbox.StatusPending &&
				(input.Mode == agentinbox.ModeSteer || input.Mode == agentinbox.ModeInject) &&
				input.TargetStepSequence == binding.TargetStepSequence {
				binding.TargetStepID = input.TargetStepID
				break
			}
		}
		if binding.TargetStepID == "" {
			binding.TargetStepID = productAttemptLoopStepID(
				active.AttemptLoopBinding, turn.Sequence, binding.TargetStepSequence,
			)
		}
	default:
		return agentinbox.Binding{}, errProductInvalidAgentInput
	}
	return binding, nil
}

func validProductAgentInputRequest(request productAgentInputRequest) bool {
	if request.SchemaVersion != productAgentInputSchemaVersion ||
		!validProductAgentInputIdentity(request.SegmentID) ||
		!validProductAgentInputIdentity(request.AgentInstanceID) ||
		!validProductAgentInputIdentity(request.WorkItemID) ||
		!validProductAgentInputIdentity(request.RunID) || request.ClaimGeneration <= 0 ||
		!validProductAgentInputIncident(request.IncidentID) || len(request.Content) == 0 ||
		len(request.Content) > productAgentInputMaxBytes || !utf8.Valid(request.Content) {
		return false
	}
	for _, value := range request.Content {
		if value == 0 {
			return false
		}
	}
	switch request.Mode {
	case agentinbox.ModeQueue, agentinbox.ModeSteer, agentinbox.ModeInject:
	default:
		return false
	}
	switch request.ContextScope {
	case agentinbox.ScopeConversationShared, agentinbox.ScopeTeamShared,
		agentinbox.ScopeAgentPrivate:
		return request.ScopeTargetID == ""
	case agentinbox.ScopeRoleRestricted, agentinbox.ScopeArtifact,
		agentinbox.ScopeSecretReference:
		return validProductAgentInputIdentity(request.ScopeTargetID)
	default:
		return false
	}
}

func validProductAgentInputIdentity(value string) bool {
	return len(value) > 0 && len(value) <= 256
}

func validProductAgentInputIncident(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' || character == '.' || character == '_' ||
			character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func productAgentInputMatchesRequest(
	binding agentinbox.Binding,
	request productAgentInputRequest,
	contentDigest string,
) bool {
	return binding.SegmentID == request.SegmentID &&
		binding.AgentInstanceID == request.AgentInstanceID &&
		binding.WorkItemID == request.WorkItemID && binding.RunID == request.RunID &&
		binding.ClaimGeneration == request.ClaimGeneration && binding.Mode == request.Mode &&
		binding.ContextScope == request.ContextScope && binding.ScopeTargetID == request.ScopeTargetID &&
		binding.ContentType == "text/plain" && binding.ContentDigest == contentDigest
}

func productAgentInputByID(
	state work.AgentInboxSnapshot,
	inputID string,
) (work.AgentInboxInputRecord, bool) {
	for _, input := range state.Inputs {
		if input.InputID == inputID {
			return input, true
		}
	}
	return work.AgentInboxInputRecord{}, false
}

func productAgentInputReceiptFor(
	incidentID string,
	binding agentinbox.Binding,
) productAgentInputReceipt {
	return productAgentInputReceipt{
		SchemaVersion: productAgentInputSchemaVersion, IncidentID: incidentID,
		InputID: binding.InputID, Mode: binding.Mode, OrderKey: binding.OrderKey,
		TargetTurnID: binding.TargetTurnID, TargetTurnSequence: binding.TargetTurnSequence,
		TargetStepID: binding.TargetStepID, TargetStepSequence: binding.TargetStepSequence,
	}
}

func (ingress *productAgentInputIngress) record(
	ctx context.Context,
	startedAt time.Time,
	request productAgentInputRequest,
	active productActiveAttempt,
	result string,
	errorCode string,
	retryable bool,
) error {
	elapsed := ingress.now().Sub(startedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	binding := active.ExecutionBinding
	return ingress.diagnostics.RecordAgentAttemptDiagnostic(ctx, nativeadapter.AgentAttemptDiagnostic{
		OccurredAt: startedAt, IncidentID: request.IncidentID,
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		ModelID: binding.ModelID, WorkItemID: request.WorkItemID, RunID: request.RunID,
		ClaimGeneration:        request.ClaimGeneration,
		RuntimeInstanceID:      active.AttemptLoopBinding.PayloadAuthority.RuntimeInstanceID,
		AgentInstanceID:        request.AgentInstanceID,
		ExecutionBindingDigest: active.AttemptLoopBinding.PayloadAuthority.ExecutionBindingDigest,
		ContextCapsuleDigest:   active.CapsuleDigest, Stage: productAgentInputStage,
		Elapsed: elapsed, Result: result, ErrorCode: errorCode, Retryable: retryable,
	})
}

func agentInputErrorCode(err error) string {
	switch {
	case errors.Is(err, errProductInvalidAgentInput):
		return "invalid_request"
	case errors.Is(err, errProductActiveAttemptNotFound):
		return "stale_generation"
	case errors.Is(err, errProductAgentInputConflict), errors.Is(err, work.ErrAgentInboxConflict):
		return "conflict"
	default:
		return "state_unavailable"
	}
}

func agentInputRetryable(err error) bool {
	return errors.Is(err, errProductAgentInputConflict) ||
		errors.Is(err, work.ErrAgentInboxConflict) ||
		(!errors.Is(err, errProductInvalidAgentInput) &&
			!errors.Is(err, errProductActiveAttemptNotFound))
}

func clearProductAgentInput(content []byte) {
	for index := range content {
		content[index] = 0
	}
}
