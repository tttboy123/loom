package nativeadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/prompting"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

// LoomNativeAgentContinuationRequest contains one already-authorized recovery
// continuation. The caller retains ownership of all mutable plaintext fields.
type LoomNativeAgentContinuationRequest struct {
	DispatchMessageID string
	IncidentID        string
	ClaimID           string
	WorkItemID        string
	RunID             string
	ClaimGeneration   int64
	AgentInstanceID   string
	SegmentID         string
	CheckpointDigest  string
	ExecutionBinding  loomruntime.FrozenExecutionBinding
	ContextCapsule    contextcapsule.AuthorityRecord
	ContextPayload    []byte
	PreviousOutput    []byte
	CurrentInputs     loomruntime.AgentInputBatch
	NextInputs        loomruntime.AgentInputSource
	ContextDelivery   contextcapsule.DeliveryBroker
	FrameSink         supervisor.FrameSink
}

type LoomNativeAgentContinuationAdapter interface {
	supervisor.RuntimeAdapter
	ResumeAgentAttempt(
		context.Context,
		LoomNativeAgentContinuationRequest,
	) (supervisor.AdapterResult, error)
}

var _ LoomNativeAgentContinuationAdapter = (*deepSeekAgentAdapter)(nil)

func (adapter *deepSeekAgentAdapter) ResumeAgentAttempt(
	ctx context.Context,
	continuation LoomNativeAgentContinuationRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || ctx == nil || ctx.Err() != nil {
		return supervisor.AdapterResult{}, ErrInvalidDeepSeekAgentAdapter
	}
	dispatch, prompt, err := adapter.validateContinuation(continuation)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	request := supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: continuation.WorkItemID, RunID: continuation.RunID,
			ClaimGeneration:       continuation.ClaimGeneration,
			RuntimeInstanceID:     continuation.ExecutionBinding.RuntimeInstanceID,
			SenderAgentInstanceID: continuation.AgentInstanceID,
		},
		ClaimID: continuation.ClaimID, IncidentID: continuation.IncidentID,
		ExecutionBinding: continuation.ExecutionBinding,
		Dispatch:         dispatch, FrameSink: continuation.FrameSink,
		ContextCapsule: continuation.ContextCapsule,
	}
	started := time.Now()
	var response deepSeekAgentResponse
	credentialErr := adapter.credentialAccess.UseCredential(
		ctx,
		continuation.ExecutionBinding,
		func(leaseContext context.Context, secret []byte) error {
			if len(secret) == 0 || len(secret) > 8192 {
				return ErrAgentCredentialUnavailable
			}
			candidate, callErr := adapter.callProviderContinuation(
				leaseContext, prompt, continuation, secret,
			)
			if callErr == nil {
				response = candidate
			}
			return callErr
		},
	)
	if credentialErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = adapter.recordDiagnostic(
				context.WithoutCancel(ctx), request, started,
				"agent_attempt_dispatch", "failed", "timeout", true,
			)
			return supervisor.AdapterResult{}, ctxErr
		}
		reason := "credential_unavailable"
		var providerFailure *deepSeekAgentProviderFailure
		if errors.As(credentialErr, &providerFailure) {
			reason = providerFailure.reason
		}
		stage, retryable := deepSeekAgentDiagnosticFailure(reason, credentialErr)
		if providerFailure != nil && providerFailure.stage != "" {
			stage = providerFailure.stage
		}
		if err := adapter.recordDiagnostic(
			ctx, request, started, stage, "failed", reason, retryable,
		); err != nil {
			return supervisor.AdapterResult{}, err
		}
		return adapter.publish(ctx, request, "", "failed", reason, nil)
	}
	if err := adapter.recordDiagnostic(
		ctx, request, started, "agent_attempt_dispatch", "succeeded", "", false,
	); err != nil {
		return supervisor.AdapterResult{}, err
	}
	return adapter.publish(
		ctx, request, response.content, "succeeded", "", response.accounting,
	)
}

func (adapter *deepSeekAgentAdapter) validateContinuation(
	request LoomNativeAgentContinuationRequest,
) (bridgev1.Frame, string, error) {
	if adapter.ValidateAgentAttemptRestartBinding(request.ExecutionBinding) != nil ||
		nilInterface(request.FrameSink) || request.DispatchMessageID == "" ||
		request.IncidentID == "" || request.ClaimID == "" ||
		request.WorkItemID == "" || request.RunID == "" || request.ClaimGeneration <= 0 ||
		request.AgentInstanceID == "" || request.SegmentID == "" ||
		!validContinuationText(request.PreviousOutput) ||
		!loomruntime.ValidAgentInputCheckpoint(
			loomruntime.AgentInputCheckpoint{OutputDigest: request.CheckpointDigest},
		) {
		return bridgev1.Frame{}, "", ErrAgentExecutionBindingChanged
	}
	checkpoint := sha256.Sum256(request.PreviousOutput)
	if hex.EncodeToString(checkpoint[:]) != request.CheckpointDigest {
		return bridgev1.Frame{}, "", ErrAgentExecutionBindingChanged
	}
	authority, err := contextcapsule.ValidateAuthorityRecord(request.ContextCapsule)
	if err != nil || authority.ConversationID == "" ||
		authority.AgentID != request.AgentInstanceID ||
		authority.ProviderID != request.ExecutionBinding.ProviderID ||
		authority.ProviderAccountID != request.ExecutionBinding.ProviderAccountID ||
		authority.ModelID != request.ExecutionBinding.ModelID ||
		authority.AuthMode != string(request.ExecutionBinding.AuthMode) {
		return bridgev1.Frame{}, "", ErrAgentExecutionBindingChanged
	}
	prompt, err := contextcapsule.DecodeDispatchPayload(request.ContextPayload)
	if err != nil || prompt.CapsuleDigest != authority.CapsuleDigest ||
		prompt.DisclosureReceiptDigest != authority.DisclosureReceiptDigest ||
		!validContinuationBatch(request, authority) {
		return bridgev1.Frame{}, "", ErrAgentExecutionBindingChanged
	}
	now := adapter.now()
	if now.IsZero() || now.Location() != time.UTC {
		return bridgev1.Frame{}, "", ErrInvalidDeepSeekAgentAdapter
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: request.DispatchMessageID, CorrelationID: request.IncidentID,
		WorkItemID: request.WorkItemID, RunID: request.RunID,
		ClaimGeneration:       request.ClaimGeneration,
		RuntimeInstanceID:     request.ExecutionBinding.RuntimeInstanceID,
		SenderAgentInstanceID: request.AgentInstanceID, Sequence: 1,
		Type: bridgev1.MessageDispatch, EmittedAt: now,
		Payload: bytes.Clone(request.ContextPayload),
	})
	if err != nil {
		return bridgev1.Frame{}, "", errors.Join(ErrDeepSeekAgentProtocol, err)
	}
	return dispatch, prompt.Prompt, nil
}

func validContinuationBatch(
	request LoomNativeAgentContinuationRequest,
	authority contextcapsule.AuthorityRecord,
) bool {
	batch := request.CurrentInputs
	if batch.TurnID == "" || batch.TurnSequence <= 0 ||
		batch.StepID == "" || batch.StepSequence <= 0 || len(batch.Inputs) == 0 {
		return false
	}
	for index := range batch.Inputs {
		binding := batch.Inputs[index].Binding
		if binding.ConversationID != authority.ConversationID ||
			binding.SegmentID != request.SegmentID ||
			binding.AgentInstanceID != request.AgentInstanceID ||
			binding.WorkItemID != request.WorkItemID || binding.RunID != request.RunID ||
			binding.ClaimGeneration != request.ClaimGeneration ||
			binding.RuntimeInstanceID != request.ExecutionBinding.RuntimeInstanceID ||
			binding.ExecutionBindingDigest != request.ExecutionBinding.BindingDigest ||
			binding.CapsuleDigest != authority.CapsuleDigest ||
			!validContinuationText(batch.Inputs[index].Content) {
			return false
		}
	}
	return true
}

func validContinuationText(content []byte) bool {
	return len(content) > 0 && len(content) <= 64<<10 && utf8.Valid(content) &&
		bytes.IndexByte(content, 0) < 0
}

func (adapter *deepSeekAgentAdapter) callProviderContinuation(
	ctx context.Context,
	prompt string,
	request LoomNativeAgentContinuationRequest,
	secret []byte,
) (deepSeekAgentResponse, error) {
	systemPrompt, err := buildNativeContinuationSystemPrompt(
		adapter.provider.providerID, adapter.provider.modelID,
		request.ContextDelivery != nil,
	)
	if err != nil {
		return deepSeekAgentResponse{}, err
	}
	inputPrompt, err := loomruntime.RenderAgentInput(&request.CurrentInputs)
	if err != nil {
		return deepSeekAgentResponse{}, &deepSeekAgentProviderFailure{
			reason: "agent_input_unavailable", stage: "agent_attempt_dispatch",
		}
	}
	messages := []openAICompatibleMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: prompt},
		{Role: "assistant", MutableContent: bytes.Clone(request.PreviousOutput)},
		{Role: "user", MutableContent: inputPrompt},
	}
	return adapter.callProviderMessages(
		ctx, messages, secret, request.ContextDelivery, request.NextInputs,
	)
}

func buildNativeContinuationSystemPrompt(
	providerID, modelID string,
	contextDelivery bool,
) (string, error) {
	tools := []prompting.ToolCapability(nil)
	if contextDelivery {
		tools = []prompting.ToolCapability{prompting.ToolContextRead}
	}
	prompt, err := prompting.BuildSystemPrompt(prompting.Profile{
		Mode: prompting.ModeAgent, ProviderID: providerID, ModelID: modelID,
		HarnessAdapter: LoomNativeAgentAdapterType, Tools: tools,
	})
	if err != nil {
		return "", ErrDeepSeekAgentProtocol
	}
	return prompt, nil
}
