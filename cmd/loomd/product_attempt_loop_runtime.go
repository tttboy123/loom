package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"loom-pi-rebuild/internal/agentcheckpoint"
	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const (
	productAttemptLoopPermissionProfileID = "system.context-retrieval.v1"
	productAttemptLoopMaxResultBytes      = int64(16 * 1024 * 1024)
	productAttemptLoopMaxToolCalls        = 4
	productAttemptLoopMaxInputTurns       = 8
	productAttemptLoopMaxInputSteps       = 16
)

type productAttemptLoopRuntimeAdapter struct {
	delegate       supervisor.RuntimeAdapter
	loops          *work.AttemptLoopAuthority
	payloadStore   attemptpayload.Store
	active         *productActiveAttemptRegistry
	inbox          *work.AgentInboxCoordinator
	checkpoints    agentcheckpoint.Store
	contextStore   contextcapsule.RetrievalStore
	contextAuditor contextcapsule.RetrievalAuditor
}

type productAttemptLoopFactAuthority struct {
	loops        *work.AttemptLoopAuthority
	binding      work.AttemptLoopBinding
	budget       work.AttemptLoopBudget
	payloadStore attemptpayload.Store
}

type productAttemptLoopInvocation struct {
	Binding          work.AttemptLoopBinding
	Budget           work.AttemptLoopBudget
	ExecutionBinding work.FrozenExecutionBinding
	SegmentID        string
	TurnID           string
	StepID           string
}

type productAttemptLoopInvocationContextKey struct{}

func productAttemptLoopInvocationFromContext(
	ctx context.Context,
) (productAttemptLoopInvocation, bool) {
	if ctx == nil {
		return productAttemptLoopInvocation{}, false
	}
	invocation, ok := ctx.Value(productAttemptLoopInvocationContextKey{}).(productAttemptLoopInvocation)
	return invocation, ok
}

type productAttemptLoopContextDelivery struct {
	delegate         contextcapsule.DeliveryBroker
	loops            *work.AttemptLoopAuthority
	binding          work.AttemptLoopBinding
	cursor           *productAttemptLoopCursor
	toolSchemaDigest string
	executionMode    work.ToolExecutionMode
}

type productAttemptLoopCursor struct {
	mu           sync.Mutex
	turnID       string
	turnSequence int
	stepID       string
	stepSequence int
}

type productAttemptLoopAgentInputSource struct {
	mu          sync.Mutex
	loops       *work.AttemptLoopAuthority
	inbox       *work.AgentInboxCoordinator
	checkpoints agentcheckpoint.Store
	binding     work.AttemptLoopBinding
	segmentID   string
	cursor      *productAttemptLoopCursor
}

func newProductAttemptLoopRuntimeAdapter(
	delegate supervisor.RuntimeAdapter,
	loops *work.AttemptLoopAuthority,
	payloadStore attemptpayload.Store,
) (*productAttemptLoopRuntimeAdapter, error) {
	return newProductAttemptLoopRuntimeAdapterWithRegistry(
		delegate, loops, payloadStore, nil,
	)
}

func newProductAttemptLoopRuntimeAdapterWithRegistry(
	delegate supervisor.RuntimeAdapter,
	loops *work.AttemptLoopAuthority,
	payloadStore attemptpayload.Store,
	active *productActiveAttemptRegistry,
) (*productAttemptLoopRuntimeAdapter, error) {
	return newProductAttemptLoopRuntimeAdapterWithGovernance(
		delegate, loops, payloadStore, active, nil, nil, nil, nil,
	)
}

func newProductAttemptLoopRuntimeAdapterWithGovernance(
	delegate supervisor.RuntimeAdapter,
	loops *work.AttemptLoopAuthority,
	payloadStore attemptpayload.Store,
	active *productActiveAttemptRegistry,
	inbox *work.AgentInboxCoordinator,
	checkpoints agentcheckpoint.Store,
	contextStore contextcapsule.RetrievalStore,
	contextAuditor contextcapsule.RetrievalAuditor,
) (*productAttemptLoopRuntimeAdapter, error) {
	checkpointAvailable := checkpoints != nil && !nilProductAgentInterface(checkpoints)
	_, restartConformant := delegate.(loomruntime.AgentAttemptRestartConformance)
	if nilProductAgentInterface(delegate) || loops == nil ||
		nilProductAgentInterface(payloadStore) ||
		(checkpoints != nil && !checkpointAvailable) ||
		(checkpointAvailable && inbox == nil) ||
		(restartConformant && inbox != nil && !checkpointAvailable) {
		return nil, app.ErrInvalidMissionExecution
	}
	return &productAttemptLoopRuntimeAdapter{
		delegate: delegate, loops: loops, payloadStore: payloadStore,
		active: active, inbox: inbox, checkpoints: checkpoints,
		contextStore: contextStore, contextAuditor: contextAuditor,
	}, nil
}

func (adapter *productAttemptLoopRuntimeAdapter) AdapterType() string {
	if adapter == nil || nilProductAgentInterface(adapter.delegate) {
		return ""
	}
	return adapter.delegate.AdapterType()
}

func (adapter *productAttemptLoopRuntimeAdapter) RuntimeInstanceID() string {
	if adapter == nil || nilProductAgentInterface(adapter.delegate) {
		return ""
	}
	return adapter.delegate.RuntimeInstanceID()
}

func (adapter *productAttemptLoopRuntimeAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || nilProductAgentInterface(adapter.delegate) || adapter.loops == nil ||
		nilProductAgentInterface(adapter.payloadStore) || ctx == nil {
		return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
	}
	if request.ContextCapsule == (contextcapsule.AuthorityRecord{}) {
		return adapter.delegate.Execute(ctx, request)
	}
	if request.AgentInputs != nil {
		return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
	}
	if request.ExecutionBinding.HarnessAdapter != adapter.AdapterType() ||
		request.Binding.RuntimeInstanceID != adapter.RuntimeInstanceID() {
		return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
	}
	binding, budget, err := productAttemptLoopBinding(request)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	inputCapable := adapter.inbox != nil && productRuntimeAcceptsAgentInputs(adapter.delegate)
	if inputCapable {
		budget.MaxTurns = productAttemptLoopMaxInputTurns
		budget.MaxStepsPerTurn = productAttemptLoopMaxInputSteps
	}
	turnID, stepID, requestID := productAttemptLoopIDs(binding)
	cursor := &productAttemptLoopCursor{
		turnID: turnID, turnSequence: 1, stepID: stepID, stepSequence: 1,
	}
	turnDigest := productAttemptLoopBytesDigest(
		"loom/product-attempt-loop/turn-input/v1", request.Dispatch.Payload(),
	)
	modelDigest := productAttemptLoopDigest(struct {
		Domain          string `json:"domain"`
		TurnDigest      string `json:"turn_digest"`
		CapsuleDigest   string `json:"capsule_digest"`
		BindingDigest   string `json:"binding_digest"`
		RuntimeInstance string `json:"runtime_instance"`
	}{
		Domain: "loom/product-attempt-loop/model-input/v1", TurnDigest: turnDigest,
		CapsuleDigest:   request.ContextCapsule.CapsuleDigest,
		BindingDigest:   request.ExecutionBinding.BindingDigest,
		RuntimeInstance: request.Binding.RuntimeInstanceID,
	})
	if _, err := adapter.loops.StartTurn(ctx, binding, budget, work.AttemptLoopTurnInput{
		TurnID: turnID, Sequence: 1, InputDigest: turnDigest,
	}); err != nil {
		return supervisor.AdapterResult{}, err
	}
	if _, err := adapter.loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
		TurnID: turnID, StepID: stepID, Sequence: 1, ModelInputDigest: modelDigest,
	}); err != nil {
		return supervisor.AdapterResult{}, err
	}
	if _, err := adapter.loops.AdmitModelRequest(
		ctx, binding, work.AttemptLoopModelRequestInput{
			TurnID: turnID, StepID: stepID, RequestID: requestID,
			RequestDigest: productAttemptLoopDigest(struct {
				Domain      string `json:"domain"`
				RequestID   string `json:"request_id"`
				ModelDigest string `json:"model_digest"`
			}{"loom/product-attempt-loop/model-request/v1", requestID, modelDigest}),
		},
	); err != nil {
		return supervisor.AdapterResult{}, err
	}
	segmentID := request.RouteSegment.SegmentID
	if segmentID == "" {
		return supervisor.AdapterResult{}, errors.Join(
			app.ErrInvalidMissionExecution,
			adapter.endFailed(ctx, binding, turnID, stepID),
		)
	}
	if inputCapable {
		request.AgentInputs = &productAttemptLoopAgentInputSource{
			loops: adapter.loops, inbox: adapter.inbox, checkpoints: adapter.checkpoints,
			binding: binding, segmentID: segmentID, cursor: cursor,
		}
	}
	if adapter.active != nil {
		registration, _, registerErr := adapter.active.Register(request, binding, budget)
		if registerErr != nil {
			return supervisor.AdapterResult{}, errors.Join(
				app.ErrInvalidMissionExecution,
				registerErr,
				adapter.endFailed(ctx, binding, turnID, stepID),
			)
		}
		defer registration.Close()
	}

	capable := productAgentContainsString(
		request.ExecutionBinding.Capabilities,
		loomruntime.CapabilityContextRetrieval,
	)
	if capable {
		if nilProductAgentInterface(request.ContextRetriever) {
			if adapter.contextStore == nil || adapter.contextAuditor == nil {
				return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
			}
			retriever, retrievalErr := contextcapsule.NewScopedRetriever(
				request.ContextCapsule,
				contextcapsule.AttemptIdentity{
					WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
					ClaimID: request.ClaimID, ClaimGeneration: request.Binding.ClaimGeneration,
					RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
					ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
					IncidentID:             request.IncidentID,
				},
				adapter.contextStore, adapter.contextAuditor,
			)
			if retrievalErr != nil {
				return supervisor.AdapterResult{}, errors.Join(
					app.ErrInvalidMissionExecution, retrievalErr,
				)
			}
			request.ContextRetriever = retriever
		}
		if !nilProductAgentInterface(request.ContextDelivery) {
			return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
		}
		facts := &productAttemptLoopFactAuthority{
			loops: adapter.loops, binding: binding, budget: budget,
			payloadStore: adapter.payloadStore,
		}
		delivery, deliveryErr := contextcapsule.NewDeliveryCoordinator(
			request.ContextCapsule,
			contextcapsule.AttemptIdentity{
				WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
				ClaimID: request.ClaimID, ClaimGeneration: request.Binding.ClaimGeneration,
				RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
				ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
				IncidentID:             request.IncidentID,
			},
			request.ContextRetriever, adapter.payloadStore, facts,
		)
		if deliveryErr != nil {
			return supervisor.AdapterResult{}, errors.Join(
				app.ErrInvalidMissionExecution, deliveryErr,
			)
		}
		request.ContextDelivery = &productAttemptLoopContextDelivery{
			delegate: delivery, loops: adapter.loops, binding: binding,
			cursor:           cursor,
			toolSchemaDigest: productAttemptLoopContextToolSchemaDigest(),
			executionMode: productAttemptLoopContextExecutionMode(
				request.ExecutionBinding.HarnessAdapter,
			),
		}
	} else if !nilProductAgentInterface(request.ContextRetriever) ||
		!nilProductAgentInterface(request.ContextDelivery) {
		return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
	}
	ctx = context.WithValue(
		ctx,
		productAttemptLoopInvocationContextKey{},
		productAttemptLoopInvocation{
			Binding: binding, Budget: budget, ExecutionBinding: request.ExecutionBinding,
			SegmentID: segmentID, TurnID: turnID, StepID: stepID,
		},
	)
	result, executeErr := adapter.delegate.Execute(ctx, request)
	currentTurnID, _, currentStepID, _ := cursor.Current()
	if executeErr != nil {
		return result, errors.Join(
			executeErr, adapter.endFailed(ctx, binding, currentTurnID, currentStepID),
		)
	}
	if result.ExitCode() != 0 || !result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() {
		return result, errors.Join(
			app.ErrMissionExecutionConflict,
			adapter.endFailed(ctx, binding, currentTurnID, currentStepID),
		)
	}
	if status, available := productAttemptLoopTerminalStatus(result); available &&
		status != "succeeded" {
		return result, adapter.endFailed(ctx, binding, currentTurnID, currentStepID)
	}
	outputDigest := productAttemptLoopResultDigest(result)
	if _, err := adapter.loops.EndStep(ctx, binding, work.AttemptLoopStepEndInput{
		TurnID: currentTurnID, StepID: currentStepID, Outcome: work.AttemptStepFinal,
		OutputDigest: outputDigest,
	}); err != nil {
		return result, err
	}
	if _, err := adapter.loops.EndTurn(ctx, binding, work.AttemptLoopTurnEndInput{
		TurnID: currentTurnID, Outcome: work.AttemptTurnSucceeded,
	}); err != nil {
		return result, err
	}
	return result, nil
}

func (adapter *productAttemptLoopRuntimeAdapter) endFailed(
	ctx context.Context,
	binding work.AttemptLoopBinding,
	turnID string,
	stepID string,
) error {
	failureDigest := productAttemptLoopDigest(struct {
		Domain string `json:"domain"`
		Code   string `json:"code"`
	}{"loom/product-attempt-loop/failure/v1", "runtime_adapter_failed"})
	snapshot, err := adapter.loops.Snapshot(ctx, binding)
	if err != nil {
		return err
	}
	if snapshot.Status != work.AttemptLoopRunning || len(snapshot.Turns) == 0 {
		return nil
	}
	turn := snapshot.Turns[len(snapshot.Turns)-1]
	if turn.Status != "" {
		return nil
	}
	turnID = turn.TurnID
	if len(turn.Steps) == 0 || turn.Steps[len(turn.Steps)-1].Outcome != "" {
		sequence := len(turn.Steps) + 1
		stepID = productAttemptLoopStepID(binding, turn.Sequence, sequence)
		modelDigest := productAttemptLoopDigest(struct {
			Domain string `json:"domain"`
			Code   string `json:"code"`
		}{"loom/product-attempt-loop/failure-model/v1", "runtime_adapter_failed"})
		if _, err := adapter.loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
			TurnID: turnID, StepID: stepID, Sequence: sequence,
			ModelInputDigest: modelDigest,
		}); err != nil {
			return err
		}
		requestID := productAttemptLoopRequestID(binding, turn.Sequence, sequence)
		if _, err := adapter.loops.AdmitModelRequest(
			ctx, binding, work.AttemptLoopModelRequestInput{
				TurnID: turnID, StepID: stepID, RequestID: requestID,
				RequestDigest: productAttemptLoopDigest(struct {
					Domain string `json:"domain"`
					Code   string `json:"code"`
				}{"loom/product-attempt-loop/failure-request/v1", "runtime_adapter_failed"}),
			},
		); err != nil {
			return err
		}
	} else {
		stepID = turn.Steps[len(turn.Steps)-1].StepID
	}
	_, stepErr := adapter.loops.EndStep(ctx, binding, work.AttemptLoopStepEndInput{
		TurnID: turnID, StepID: stepID, Outcome: work.AttemptStepFailed,
		OutputDigest: failureDigest, ErrorCode: "runtime_adapter_failed",
	})
	if stepErr != nil {
		return stepErr
	}
	_, turnErr := adapter.loops.EndTurn(ctx, binding, work.AttemptLoopTurnEndInput{
		TurnID: turnID, Outcome: work.AttemptTurnFailed,
	})
	return turnErr
}

func productAttemptLoopBinding(
	request supervisor.AdapterRequest,
) (work.AttemptLoopBinding, work.AttemptLoopBudget, error) {
	capsule, err := contextcapsule.ValidateAuthorityRecord(request.ContextCapsule)
	segment, segmentErr := contextcapsule.ValidateRouteSegmentBinding(request.RouteSegment)
	if err != nil || request.ClaimID == "" || request.IncidentID == "" ||
		request.Binding.WorkItemID == "" || request.Binding.RunID == "" ||
		request.Binding.ClaimGeneration < 1 || request.Binding.RuntimeInstanceID == "" ||
		request.Binding.SenderAgentInstanceID == "" ||
		capsule.AgentID != request.Binding.SenderAgentInstanceID ||
		capsule.ProviderID != request.ExecutionBinding.ProviderID ||
		capsule.ProviderAccountID != request.ExecutionBinding.ProviderAccountID ||
		capsule.ModelID != request.ExecutionBinding.ModelID ||
		capsule.AuthMode != string(request.ExecutionBinding.AuthMode) ||
		segmentErr != nil || segment.ConversationID != capsule.ConversationID ||
		segment.TeamID != capsule.TeamID || segment.AgentID != capsule.AgentID ||
		segment.RoleID != capsule.RoleID || segment.CapsuleDigest != capsule.CapsuleDigest ||
		segment.ExecutionBindingDigest != request.ExecutionBinding.BindingDigest ||
		request.ExecutionBinding.RuntimeInstanceID != request.Binding.RuntimeInstanceID ||
		request.Dispatch.CorrelationID() != request.IncidentID ||
		request.Dispatch.WorkItemID() != request.Binding.WorkItemID ||
		request.Dispatch.RunID() != request.Binding.RunID ||
		request.Dispatch.ClaimGeneration() != request.Binding.ClaimGeneration ||
		request.Dispatch.RuntimeInstanceID() != request.Binding.RuntimeInstanceID ||
		request.Dispatch.SenderAgentInstanceID() != request.Binding.SenderAgentInstanceID {
		return work.AttemptLoopBinding{}, work.AttemptLoopBudget{},
			app.ErrInvalidMissionExecution
	}
	permissionDigest := productAttemptLoopDigest(struct {
		Domain     string `json:"domain"`
		ProfileID  string `json:"profile_id"`
		Generation int64  `json:"generation"`
		Capability string `json:"capability"`
	}{
		"loom/product-attempt-loop/system-permission/v1",
		productAttemptLoopPermissionProfileID, 1, loomruntime.CapabilityContextRetrieval,
	})
	toolSchemaSetDigest := productAttemptLoopDigest(struct {
		Domain  string   `json:"domain"`
		Schemas []string `json:"schemas"`
	}{
		"loom/product-attempt-loop/tool-schema-set/v1",
		[]string{productAttemptLoopContextToolSchemaDigest()},
	})
	binding := work.AttemptLoopBinding{
		SchemaVersion: 1, AttemptID: request.Binding.RunID, TeamInstanceID: capsule.TeamID,
		PayloadAuthority: attemptpayload.Authority{
			Scope: attemptpayload.Scope{
				ConversationID: capsule.ConversationID,
				WorkItemID:     request.Binding.WorkItemID, RunID: request.Binding.RunID,
				ClaimGeneration:        request.Binding.ClaimGeneration,
				RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
				ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
				CapsuleDigest:          capsule.CapsuleDigest,
			},
			ClaimID: request.ClaimID, AgentInstanceID: request.Binding.SenderAgentInstanceID,
			IncidentID: request.IncidentID,
		},
		PermissionProfileID:         productAttemptLoopPermissionProfileID,
		PermissionProfileGeneration: 1, PermissionProfileDigest: permissionDigest,
		CapabilitySetDigest: work.AttemptLoopCapabilitySetDigest(
			request.ExecutionBinding.Capabilities,
		),
		ToolSchemaSetDigest: toolSchemaSetDigest, BudgetPolicyVersion: 1,
	}
	timeout := request.ExecutionBinding.Timeout
	if timeout <= 0 {
		return work.AttemptLoopBinding{}, work.AttemptLoopBudget{},
			app.ErrInvalidMissionExecution
	}
	if timeout > time.Hour {
		timeout = time.Hour
	}
	maxParallel, _ := productAttemptLoopContextToolPolicy(
		request.ExecutionBinding.HarnessAdapter,
	)
	budget := work.AttemptLoopBudget{
		MaxTurns: 1, MaxStepsPerTurn: 1, MaxToolCalls: productAttemptLoopMaxToolCalls,
		MaxParallelToolCalls: maxParallel, MaxResultBytes: productAttemptLoopMaxResultBytes,
		ToolTimeoutMillis: timeout.Milliseconds(),
	}
	return binding, budget, nil
}

func productAttemptLoopContextToolPolicy(
	adapterType string,
) (int, work.ToolExecutionMode) {
	switch adapterType {
	case harnessadapter.CodexAdapterType, harnessadapter.ClaudeCodeAdapterType:
		return productAttemptLoopMaxToolCalls, work.ToolExecutionParallel
	default:
		return 1, work.ToolExecutionExclusive
	}
}

func productAttemptLoopContextExecutionMode(adapterType string) work.ToolExecutionMode {
	_, mode := productAttemptLoopContextToolPolicy(adapterType)
	return mode
}

func productAttemptLoopIDs(
	binding work.AttemptLoopBinding,
) (turnID string, stepID string, requestID string) {
	turnID = "turn-" + productDeterministicUUID("attempt-loop-turn", binding.AttemptID)
	stepID = "step-" + productDeterministicUUID("attempt-loop-step", binding.AttemptID, "1")
	requestID = "request-" + productDeterministicUUID("attempt-loop-request", binding.AttemptID, "1")
	return turnID, stepID, requestID
}

func productRuntimeAcceptsAgentInputs(adapter supervisor.RuntimeAdapter) bool {
	consumer, ok := adapter.(loomruntime.AgentInputConsumer)
	return ok && consumer.AcceptsAgentInputs()
}

func (cursor *productAttemptLoopCursor) Current() (string, int, string, int) {
	if cursor == nil {
		return "", 0, "", 0
	}
	cursor.mu.Lock()
	defer cursor.mu.Unlock()
	return cursor.turnID, cursor.turnSequence, cursor.stepID, cursor.stepSequence
}

func (cursor *productAttemptLoopCursor) Move(
	turnID string,
	turnSequence int,
	stepID string,
	stepSequence int,
) {
	cursor.mu.Lock()
	defer cursor.mu.Unlock()
	cursor.turnID, cursor.turnSequence = turnID, turnSequence
	cursor.stepID, cursor.stepSequence = stepID, stepSequence
}

func (source *productAttemptLoopAgentInputSource) NextAgentInput(
	ctx context.Context,
	checkpoint loomruntime.AgentInputCheckpoint,
) (loomruntime.AgentInputBatch, bool, error) {
	if source == nil || source.loops == nil || source.inbox == nil ||
		source.cursor == nil || ctx == nil || ctx.Err() != nil ||
		!loomruntime.ValidAgentInputCheckpoint(checkpoint) {
		return loomruntime.AgentInputBatch{}, false, app.ErrInvalidMissionExecution
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.nextAgentInputLocked(ctx, checkpoint)
}

func (source *productAttemptLoopAgentInputSource) NextAgentInputFromDurableCheckpoint(
	ctx context.Context,
	payload loomruntime.AgentInputCheckpointPayload,
) (loomruntime.AgentInputBatch, bool, error) {
	if source == nil || source.loops == nil || source.inbox == nil ||
		nilProductAgentInterface(source.checkpoints) || source.cursor == nil ||
		ctx == nil || ctx.Err() != nil ||
		!loomruntime.ValidAgentInputCheckpointPayload(payload) {
		return loomruntime.AgentInputBatch{}, false, app.ErrInvalidMissionExecution
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	turnID, turnSequence, stepID, stepSequence := source.cursor.Current()
	binding := source.binding
	checkpointBinding := agentcheckpoint.Binding{
		CheckpointID: "agent-checkpoint-" + productDeterministicUUID(
			"agent-checkpoint", binding.AttemptID,
			strconv.FormatInt(binding.PayloadAuthority.ClaimGeneration, 10),
			turnID, stepID, payload.Checkpoint.OutputDigest,
		),
		ConversationID:         binding.PayloadAuthority.ConversationID,
		SegmentID:              source.segmentID,
		AttemptID:              binding.AttemptID,
		AgentInstanceID:        binding.PayloadAuthority.AgentInstanceID,
		WorkItemID:             binding.PayloadAuthority.WorkItemID,
		RunID:                  binding.PayloadAuthority.RunID,
		ClaimGeneration:        binding.PayloadAuthority.ClaimGeneration,
		RuntimeInstanceID:      binding.PayloadAuthority.RuntimeInstanceID,
		ExecutionBindingDigest: binding.PayloadAuthority.ExecutionBindingDigest,
		CapsuleDigest:          binding.PayloadAuthority.CapsuleDigest,
		TurnID:                 turnID,
		TurnSequence:           turnSequence,
		StepID:                 stepID,
		StepSequence:           stepSequence,
		ContentType:            agentcheckpoint.ContentTypeTextUTF8,
		ContentDigest:          payload.Checkpoint.OutputDigest,
	}
	checkpointPayload := agentcheckpoint.Payload{
		Binding: checkpointBinding,
		Content: append([]byte(nil), payload.Content...),
	}
	err := source.checkpoints.PutAgentCheckpoint(ctx, checkpointPayload)
	checkpointPayload.Close()
	if err != nil {
		return loomruntime.AgentInputBatch{}, false,
			errors.Join(app.ErrMissionExecutionConflict, err)
	}
	batch, available, nextErr := source.nextAgentInputLocked(ctx, payload.Checkpoint)
	if nextErr == nil && !available {
		if deleteErr := source.checkpoints.DeleteAgentCheckpoint(
			ctx, checkpointBinding,
		); deleteErr != nil {
			return loomruntime.AgentInputBatch{}, false,
				errors.Join(app.ErrMissionExecutionConflict, deleteErr)
		}
	}
	return batch, available, nextErr
}

func (source *productAttemptLoopAgentInputSource) nextAgentInputLocked(
	ctx context.Context,
	checkpoint loomruntime.AgentInputCheckpoint,
) (loomruntime.AgentInputBatch, bool, error) {
	turnID, turnSequence, stepID, stepSequence := source.cursor.Current()
	state, err := source.inbox.Snapshot(ctx, source.binding)
	if err != nil {
		return loomruntime.AgentInputBatch{}, false, err
	}
	if batch, recovered, recoverErr := source.recoverCommittedAgentInput(
		ctx, state, checkpoint, turnID, turnSequence, stepID, stepSequence,
	); recoverErr != nil || recovered {
		return batch, recovered, recoverErr
	}
	stepInputs := productPendingStepInputs(state, stepSequence+1)
	if len(stepInputs) != 0 {
		if _, err := source.loops.EndStep(ctx, source.binding, work.AttemptLoopStepEndInput{
			TurnID: turnID, StepID: stepID, Outcome: work.AttemptStepContinue,
			OutputDigest: checkpoint.OutputDigest,
		}); err != nil {
			return loomruntime.AgentInputBatch{}, false, err
		}
		nextStepID := stepInputs[0].TargetStepID
		modelDigest := productAgentInputModelDigest(stepInputs)
		payloads, _, err := source.inbox.ConsumeTargetStep(
			ctx, source.binding, stepInputs, work.AttemptLoopStepInput{
				TurnID: turnID, StepID: nextStepID, Sequence: stepSequence + 1,
				ModelInputDigest: modelDigest,
			},
		)
		if err != nil {
			return loomruntime.AgentInputBatch{}, false, err
		}
		if err := source.admitModelRequest(
			ctx, turnID, turnSequence, nextStepID, stepSequence+1, modelDigest,
		); err != nil {
			closeProductAgentInputPayloads(payloads)
			return loomruntime.AgentInputBatch{}, false, err
		}
		batch, err := loomruntime.NewAgentInputBatch(
			turnID, turnSequence, nextStepID, stepSequence+1, payloads,
		)
		if err != nil {
			return loomruntime.AgentInputBatch{}, false,
				errors.Join(app.ErrMissionExecutionConflict, err)
		}
		source.cursor.Move(turnID, turnSequence, nextStepID, stepSequence+1)
		return batch, true, nil
	}
	queued, found := productNextQueuedInput(state)
	if !found {
		return loomruntime.AgentInputBatch{}, false, nil
	}
	if _, err := source.loops.EndStep(ctx, source.binding, work.AttemptLoopStepEndInput{
		TurnID: turnID, StepID: stepID, Outcome: work.AttemptStepFinal,
		OutputDigest: checkpoint.OutputDigest,
	}); err != nil {
		return loomruntime.AgentInputBatch{}, false, err
	}
	if _, err := source.loops.EndTurn(ctx, source.binding, work.AttemptLoopTurnEndInput{
		TurnID: turnID, Outcome: work.AttemptTurnSucceeded,
	}); err != nil {
		return loomruntime.AgentInputBatch{}, false, err
	}
	payload, _, err := source.inbox.ConsumeQueuedTurn(ctx, source.binding, queued)
	if err != nil {
		return loomruntime.AgentInputBatch{}, false, err
	}
	nextTurnID, nextTurnSequence := queued.TargetTurnID, queued.TargetTurnSequence
	nextStepID := productAttemptLoopStepID(source.binding, nextTurnSequence, 1)
	modelDigest := productAgentInputModelDigest([]agentinbox.Binding{queued})
	if _, err := source.loops.StartStep(ctx, source.binding, work.AttemptLoopStepInput{
		TurnID: nextTurnID, StepID: nextStepID, Sequence: 1,
		ModelInputDigest: modelDigest,
	}); err != nil {
		payload.Close()
		return loomruntime.AgentInputBatch{}, false, err
	}
	if err := source.admitModelRequest(
		ctx, nextTurnID, nextTurnSequence, nextStepID, 1, modelDigest,
	); err != nil {
		payload.Close()
		return loomruntime.AgentInputBatch{}, false, err
	}
	batch, err := loomruntime.NewAgentInputBatch(
		nextTurnID, nextTurnSequence, nextStepID, 1, []agentinbox.Payload{payload},
	)
	if err != nil {
		return loomruntime.AgentInputBatch{}, false,
			errors.Join(app.ErrMissionExecutionConflict, err)
	}
	source.cursor.Move(nextTurnID, nextTurnSequence, nextStepID, 1)
	return batch, true, nil
}

func (source *productAttemptLoopAgentInputSource) recoverCommittedAgentInput(
	ctx context.Context,
	state work.AgentInboxSnapshot,
	checkpoint loomruntime.AgentInputCheckpoint,
	cursorTurnID string,
	cursorTurnSequence int,
	cursorStepID string,
	cursorStepSequence int,
) (loomruntime.AgentInputBatch, bool, error) {
	loop := state.Loop
	if loop.Status != work.AttemptLoopRunning || len(loop.Turns) == 0 {
		return loomruntime.AgentInputBatch{}, false, nil
	}
	turn := loop.Turns[len(loop.Turns)-1]
	if turn.Status != "" {
		return loomruntime.AgentInputBatch{}, false, nil
	}
	if len(turn.Steps) == 0 {
		queued, found, err := productConsumedQueueForTurn(state, turn)
		if err != nil || !found || !productPreviousTurnMatchesCheckpoint(loop, checkpoint) {
			return loomruntime.AgentInputBatch{}, false, errors.Join(
				app.ErrMissionExecutionConflict, err,
			)
		}
		payload, _, err := source.inbox.ConsumeQueuedTurn(ctx, source.binding, queued)
		if err != nil {
			return loomruntime.AgentInputBatch{}, false, err
		}
		return source.resumeQueuedInput(
			ctx, turn, work.AttemptLoopStepRecord{}, queued, payload,
		)
	}
	step := turn.Steps[len(turn.Steps)-1]
	if step.ModelRequestID != "" {
		if turn.TurnID != cursorTurnID || turn.Sequence != cursorTurnSequence ||
			step.StepID != cursorStepID || step.Sequence != cursorStepSequence {
			return loomruntime.AgentInputBatch{}, false, app.ErrMissionExecutionConflict
		}
		return loomruntime.AgentInputBatch{}, false, nil
	}
	if step.Outcome != "" {
		return loomruntime.AgentInputBatch{}, false, app.ErrMissionExecutionConflict
	}
	if step.Sequence == 1 {
		queued, found, err := productConsumedQueueForTurn(state, turn)
		if err != nil || !found || !productPreviousTurnMatchesCheckpoint(loop, checkpoint) {
			return loomruntime.AgentInputBatch{}, false, errors.Join(
				app.ErrMissionExecutionConflict, err,
			)
		}
		payload, _, err := source.inbox.ConsumeQueuedTurn(ctx, source.binding, queued)
		if err != nil {
			return loomruntime.AgentInputBatch{}, false, err
		}
		return source.resumeQueuedInput(ctx, turn, step, queued, payload)
	}
	if len(turn.Steps) < 2 {
		return loomruntime.AgentInputBatch{}, false, app.ErrMissionExecutionConflict
	}
	previous := turn.Steps[len(turn.Steps)-2]
	if previous.Outcome != work.AttemptStepContinue ||
		previous.OutputDigest != checkpoint.OutputDigest {
		return loomruntime.AgentInputBatch{}, false, app.ErrMissionExecutionConflict
	}
	inputs := productConsumedStepInputs(state, step.StepID, step.Sequence)
	if len(inputs) == 0 || productAgentInputModelDigest(inputs) != step.ModelInputDigest {
		return loomruntime.AgentInputBatch{}, false, app.ErrMissionExecutionConflict
	}
	payloads, _, err := source.inbox.ConsumeTargetStep(ctx, source.binding, inputs, work.AttemptLoopStepInput{
		TurnID: turn.TurnID, StepID: step.StepID, Sequence: step.Sequence,
		ModelInputDigest: step.ModelInputDigest,
	})
	if err != nil {
		return loomruntime.AgentInputBatch{}, false, err
	}
	if err := source.admitModelRequest(
		ctx, turn.TurnID, turn.Sequence, step.StepID, step.Sequence,
		step.ModelInputDigest,
	); err != nil {
		closeProductAgentInputPayloads(payloads)
		return loomruntime.AgentInputBatch{}, false, err
	}
	batch, err := loomruntime.NewAgentInputBatch(
		turn.TurnID, turn.Sequence, step.StepID, step.Sequence, payloads,
	)
	if err != nil {
		return loomruntime.AgentInputBatch{}, false, errors.Join(
			app.ErrMissionExecutionConflict, err,
		)
	}
	source.cursor.Move(turn.TurnID, turn.Sequence, step.StepID, step.Sequence)
	return batch, true, nil
}

func (source *productAttemptLoopAgentInputSource) resumeQueuedInput(
	ctx context.Context,
	turn work.AttemptLoopTurnRecord,
	step work.AttemptLoopStepRecord,
	queued agentinbox.Binding,
	payload agentinbox.Payload,
) (loomruntime.AgentInputBatch, bool, error) {
	modelDigest := productAgentInputModelDigest([]agentinbox.Binding{queued})
	if step.StepID == "" {
		step = work.AttemptLoopStepRecord{
			StepID:   productAttemptLoopStepID(source.binding, turn.Sequence, 1),
			Sequence: 1, ModelInputDigest: modelDigest,
		}
		if _, err := source.loops.StartStep(ctx, source.binding, work.AttemptLoopStepInput{
			TurnID: turn.TurnID, StepID: step.StepID, Sequence: step.Sequence,
			ModelInputDigest: step.ModelInputDigest,
		}); err != nil {
			payload.Close()
			return loomruntime.AgentInputBatch{}, false, err
		}
	} else if step.Sequence != 1 || step.ModelInputDigest != modelDigest ||
		step.StepID != productAttemptLoopStepID(source.binding, turn.Sequence, 1) {
		payload.Close()
		return loomruntime.AgentInputBatch{}, false, app.ErrMissionExecutionConflict
	}
	if err := source.admitModelRequest(
		ctx, turn.TurnID, turn.Sequence, step.StepID, step.Sequence, modelDigest,
	); err != nil {
		payload.Close()
		return loomruntime.AgentInputBatch{}, false, err
	}
	batch, err := loomruntime.NewAgentInputBatch(
		turn.TurnID, turn.Sequence, step.StepID, step.Sequence,
		[]agentinbox.Payload{payload},
	)
	if err != nil {
		return loomruntime.AgentInputBatch{}, false, errors.Join(
			app.ErrMissionExecutionConflict, err,
		)
	}
	source.cursor.Move(turn.TurnID, turn.Sequence, step.StepID, step.Sequence)
	return batch, true, nil
}

func productConsumedStepInputs(
	state work.AgentInboxSnapshot,
	targetStepID string,
	targetStepSequence int,
) []agentinbox.Binding {
	var inputs []agentinbox.Binding
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusConsumed &&
			(record.Mode == agentinbox.ModeSteer || record.Mode == agentinbox.ModeInject) &&
			record.TargetStepID == targetStepID &&
			record.TargetStepSequence == targetStepSequence {
			inputs = append(inputs, record.Binding)
		}
	}
	return inputs
}

func productConsumedQueueForTurn(
	state work.AgentInboxSnapshot,
	turn work.AttemptLoopTurnRecord,
) (agentinbox.Binding, bool, error) {
	var matched agentinbox.Binding
	for _, record := range state.Inputs {
		if record.Status != agentinbox.StatusConsumed || record.Mode != agentinbox.ModeQueue ||
			record.TargetTurnID != turn.TurnID || record.TargetTurnSequence != turn.Sequence {
			continue
		}
		if matched.InputID != "" {
			return agentinbox.Binding{}, false, app.ErrMissionExecutionConflict
		}
		matched = record.Binding
	}
	return matched, matched.InputID != "", nil
}

func productPreviousTurnMatchesCheckpoint(
	loop work.AttemptLoopSnapshot,
	checkpoint loomruntime.AgentInputCheckpoint,
) bool {
	if len(loop.Turns) < 2 {
		return false
	}
	previous := loop.Turns[len(loop.Turns)-2]
	if previous.Status != work.AttemptTurnSucceeded || len(previous.Steps) == 0 {
		return false
	}
	step := previous.Steps[len(previous.Steps)-1]
	return step.Outcome == work.AttemptStepFinal &&
		step.OutputDigest == checkpoint.OutputDigest
}

func (source *productAttemptLoopAgentInputSource) admitModelRequest(
	ctx context.Context,
	turnID string,
	turnSequence int,
	stepID string,
	stepSequence int,
	modelDigest string,
) error {
	requestID := productAttemptLoopRequestID(
		source.binding, turnSequence, stepSequence,
	)
	_, err := source.loops.AdmitModelRequest(
		ctx, source.binding, work.AttemptLoopModelRequestInput{
			TurnID: turnID, StepID: stepID, RequestID: requestID,
			RequestDigest: productAttemptLoopDigest(struct {
				Domain      string `json:"domain"`
				RequestID   string `json:"request_id"`
				ModelDigest string `json:"model_digest"`
			}{"loom/product-attempt-loop/model-request/v1", requestID, modelDigest}),
		},
	)
	return err
}

func productPendingStepInputs(
	state work.AgentInboxSnapshot,
	targetSequence int,
) []agentinbox.Binding {
	var inputs []agentinbox.Binding
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusPending &&
			(record.Mode == agentinbox.ModeSteer || record.Mode == agentinbox.ModeInject) &&
			record.TargetStepSequence == targetSequence {
			inputs = append(inputs, record.Binding)
		}
	}
	return inputs
}

func productNextQueuedInput(state work.AgentInboxSnapshot) (agentinbox.Binding, bool) {
	for _, record := range state.Inputs {
		if record.Status == agentinbox.StatusPending && record.Mode == agentinbox.ModeQueue {
			return record.Binding, true
		}
	}
	return agentinbox.Binding{}, false
}

func productAgentInputModelDigest(inputs []agentinbox.Binding) string {
	type digestInput struct {
		InputID       string                  `json:"input_id"`
		Mode          agentinbox.Mode         `json:"mode"`
		ContextScope  agentinbox.ContextScope `json:"context_scope"`
		ScopeTargetID string                  `json:"scope_target_id,omitempty"`
		OrderKey      int64                   `json:"order_key"`
		ContentDigest string                  `json:"content_digest"`
	}
	frozen := make([]digestInput, 0, len(inputs))
	for _, input := range inputs {
		frozen = append(frozen, digestInput{
			InputID: input.InputID, Mode: input.Mode, ContextScope: input.ContextScope,
			ScopeTargetID: input.ScopeTargetID, OrderKey: input.OrderKey,
			ContentDigest: input.ContentDigest,
		})
	}
	return productAttemptLoopDigest(struct {
		Domain string        `json:"domain"`
		Inputs []digestInput `json:"inputs"`
	}{"loom/product-attempt-loop/agent-input-model/v1", frozen})
}

func productAttemptLoopStepID(
	binding work.AttemptLoopBinding,
	turnSequence int,
	stepSequence int,
) string {
	return "step-" + productDeterministicUUID(
		"attempt-loop-step", binding.AttemptID,
		strconv.Itoa(turnSequence), strconv.Itoa(stepSequence),
	)
}

func productAttemptLoopRequestID(
	binding work.AttemptLoopBinding,
	turnSequence int,
	stepSequence int,
) string {
	return "request-" + productDeterministicUUID(
		"attempt-loop-request", binding.AttemptID,
		strconv.Itoa(turnSequence), strconv.Itoa(stepSequence),
	)
}

func closeProductAgentInputPayloads(payloads []agentinbox.Payload) {
	for index := range payloads {
		payloads[index].Close()
	}
}

func (delivery *productAttemptLoopContextDelivery) Prepare(
	ctx context.Context,
	proposal contextcapsule.RetrievalProposal,
	request contextcapsule.DeliveryRequest,
	encode contextcapsule.DeliveryEncoder,
) (attemptpayload.Payload, error) {
	if delivery == nil || delivery.delegate == nil || delivery.loops == nil {
		return attemptpayload.Payload{}, contextcapsule.ErrInvalidContextDelivery
	}
	turnID, _, stepID, _ := delivery.cursor.Current()
	if turnID == "" || stepID == "" {
		return attemptpayload.Payload{}, contextcapsule.ErrInvalidContextDelivery
	}
	callID := contextcapsule.DeliveryCallID(proposal, request.Sequence)
	proposalDigest := productAttemptLoopDigest(struct {
		Domain        string `json:"domain"`
		ItemID        string `json:"item_id"`
		ContentDigest string `json:"content_digest"`
		ArtifactRef   string `json:"artifact_ref,omitempty"`
	}{
		"loom/product-attempt-loop/context-proposal/v1",
		proposal.ItemID, proposal.ContentDigest, proposal.ArtifactRef,
	})
	argumentsDigest := productAttemptLoopDigest(struct {
		Domain        string `json:"domain"`
		ItemID        string `json:"item_id"`
		ContentDigest string `json:"content_digest"`
		ArtifactRef   string `json:"artifact_ref,omitempty"`
	}{
		"loom/product-attempt-loop/context-arguments/v1",
		proposal.ItemID, proposal.ContentDigest, proposal.ArtifactRef,
	})
	conflictDigest := productAttemptLoopDigest(struct {
		Domain        string `json:"domain"`
		CapsuleDigest string `json:"capsule_digest"`
		ItemID        string `json:"item_id"`
		ArtifactRef   string `json:"artifact_ref,omitempty"`
	}{
		"loom/product-attempt-loop/context-conflict/v1",
		delivery.binding.PayloadAuthority.CapsuleDigest, proposal.ItemID, proposal.ArtifactRef,
	})
	if _, err := delivery.loops.AdmitToolCall(
		ctx, delivery.binding, work.AttemptLoopToolCallInput{
			TurnID: turnID, StepID: stepID, CallID: callID,
			Sequence: request.Sequence, Tool: permissions.ToolMCPTool,
			TargetID: proposal.ItemID, ProposalDigest: proposalDigest,
			ArgumentsDigest: argumentsDigest, ToolSchemaDigest: delivery.toolSchemaDigest,
			ExecutionMode:       delivery.executionMode,
			ConflictScopeDigest: conflictDigest,
		},
	); err != nil {
		return attemptpayload.Payload{}, errors.Join(contextcapsule.ErrInvalidContextDelivery, err)
	}
	authorizationDigest := productAttemptLoopDigest(struct {
		Domain           string `json:"domain"`
		CallID           string `json:"call_id"`
		PermissionDigest string `json:"permission_digest"`
		CapabilityDigest string `json:"capability_digest"`
	}{
		"loom/product-attempt-loop/context-authorization/v1", callID,
		delivery.binding.PermissionProfileDigest, delivery.binding.CapabilitySetDigest,
	})
	dispatchDigest := productAttemptLoopDigest(struct {
		Domain         string `json:"domain"`
		CallID         string `json:"call_id"`
		ProposalDigest string `json:"proposal_digest"`
		BindingDigest  string `json:"binding_digest"`
	}{
		"loom/product-attempt-loop/context-dispatch/v1", callID,
		proposalDigest, delivery.binding.PayloadAuthority.ExecutionBindingDigest,
	})
	if _, err := delivery.loops.CommitToolDispatch(
		ctx, delivery.binding, work.AttemptLoopToolDispatchInput{
			TurnID: turnID, StepID: stepID, CallID: callID,
			AuthorizationDigest: authorizationDigest, DispatchDigest: dispatchDigest,
		},
	); err != nil {
		return attemptpayload.Payload{}, errors.Join(contextcapsule.ErrInvalidContextDelivery, err)
	}
	return delivery.delegate.Prepare(ctx, proposal, request, encode)
}

func (delivery *productAttemptLoopContextDelivery) Acknowledge(
	ctx context.Context,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if delivery == nil || delivery.delegate == nil {
		return contextcapsule.ErrInvalidContextDelivery
	}
	return delivery.delegate.Acknowledge(ctx, binding, proof)
}

func (facts *productAttemptLoopFactAuthority) Lookup(
	ctx context.Context,
	authority attemptpayload.Authority,
	callID string,
	sequence int64,
) (attemptpayload.Fact, bool, error) {
	if facts == nil || authority != facts.binding.PayloadAuthority {
		return attemptpayload.Fact{}, false, contextcapsule.ErrInvalidContextDelivery
	}
	return facts.loops.LookupToolResult(ctx, facts.binding, callID, sequence)
}

func (facts *productAttemptLoopFactAuthority) Accept(
	ctx context.Context,
	authority attemptpayload.Authority,
	binding attemptpayload.Binding,
) error {
	if facts == nil || authority != facts.binding.PayloadAuthority {
		return contextcapsule.ErrInvalidContextDelivery
	}
	payload, err := facts.payloadStore.ReadAttemptPayload(ctx, binding)
	if err != nil {
		return err
	}
	resultBytes := int64(len(payload.Content))
	payload.Close()
	if resultBytes <= 0 || resultBytes > facts.budget.MaxResultBytes {
		return contextcapsule.ErrInvalidContextDelivery
	}
	_, err = facts.loops.AcceptToolResult(ctx, facts.binding, binding)
	return err
}

func (facts *productAttemptLoopFactAuthority) Deliver(
	ctx context.Context,
	authority attemptpayload.Authority,
	binding attemptpayload.Binding,
	proof attemptpayload.DeliveryProof,
) error {
	if facts == nil || authority != facts.binding.PayloadAuthority {
		return contextcapsule.ErrInvalidContextDelivery
	}
	_, err := facts.loops.DeliverToolResult(ctx, facts.binding, binding, proof)
	return err
}

func productAttemptLoopContextToolSchemaDigest() string {
	return productAttemptLoopDigest(struct {
		Domain     string   `json:"domain"`
		Name       string   `json:"name"`
		Version    int      `json:"version"`
		Required   []string `json:"required"`
		Optional   []string `json:"optional"`
		ResultType string   `json:"result_type"`
	}{
		"loom/product-attempt-loop/tool-schema/v1", "loom_read_context", 1,
		[]string{"item_id", "content_digest"}, []string{"artifact_ref"},
		attemptpayload.ContentTypeJSON,
	})
}

func productAttemptLoopResultDigest(result supervisor.AdapterResult) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("loom/product-attempt-loop/result/v1\x00"))
	_, _ = hash.Write([]byte(strconv.Itoa(result.ExitCode())))
	for _, frame := range result.InboundFrames() {
		_, _ = hash.Write([]byte("\x00" + string(frame.Type()) + "\x00"))
		payloadDigest := sha256.Sum256(frame.Payload())
		_, _ = hash.Write(payloadDigest[:])
	}
	stderrDigest := sha256.Sum256(result.Stderr())
	_, _ = hash.Write(stderrDigest[:])
	return hex.EncodeToString(hash.Sum(nil))
}

func productAttemptLoopTerminalStatus(
	result supervisor.AdapterResult,
) (string, bool) {
	for _, frame := range result.InboundFrames() {
		if frame.Type() != bridgev1.MessageResult {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(frame.Payload()))
		decoder.DisallowUnknownFields()
		var payload struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if decoder.Decode(&payload) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			(payload.Status != "succeeded" && payload.Status != "failed") {
			return "", false
		}
		return payload.Status, true
	}
	return "", false
}

func productAttemptLoopDigest(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("canonical Attempt loop digest: %v", err))
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func productAttemptLoopBytesDigest(domain string, value []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	return hex.EncodeToString(hash.Sum(nil))
}
