package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/toolproposal"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
)

// bridgeExecutionHook 把桥接信封接到 B-W1 执行适配器。桥接不含执行决策：
// Execute 内部完成 Evaluate/ask/deny 与 Journal 事实（不变量 3）。
type productToolExecutionPort interface {
	Execute(context.Context, execution.Proposal) (execution.ExecutionResult, error)
	RemoteToolKinds() []permissions.ToolKind
}

type bridgeExecutionHook struct {
	adapter     productToolExecutionPort
	proposals   toolproposal.Store
	loops       *work.AttemptLoopAuthority
	payloads    attemptpayload.Store
	diagnostics productAttemptToolDiagnosticRecorder
}

func (hook *bridgeExecutionHook) AllowedToolCalls() []permissions.ToolKind {
	allowed := []permissions.ToolKind{
		permissions.ToolBash, permissions.ToolEdit,
		permissions.ToolRead, permissions.ToolGrep,
	}
	if hook != nil && hook.adapter != nil {
		allowed = append(allowed, hook.adapter.RemoteToolKinds()...)
	}
	return allowed
}

func newBridgeExecutionHook(
	adapter productToolExecutionPort,
	proposals toolproposal.Store,
	loops *work.AttemptLoopAuthority,
	payloads attemptpayload.Store,
	diagnostics productAttemptToolDiagnosticRecorder,
) (*bridgeExecutionHook, error) {
	if nilProductAssetPort(adapter) || (loops == nil) != (payloads == nil) ||
		(diagnostics != nil && loops == nil) {
		return nil, execution.ErrInvalidExecutionInput
	}
	return &bridgeExecutionHook{
		adapter: adapter, proposals: proposals, loops: loops, payloads: payloads,
		diagnostics: diagnostics,
	}, nil
}

func (hook *bridgeExecutionHook) ExecuteToolCall(
	ctx context.Context,
	envelope loomruntime.ToolCallEnvelope,
	binding loomruntime.ToolCallBinding,
) (loomruntime.ToolCallResult, error) {
	if hook == nil || hook.adapter == nil {
		return loomruntime.ToolCallResult{}, loomruntime.ErrToolCallGatewayUnavailable
	}
	if hook.proposals != nil && !validBridgeProposalBinding(binding) {
		return loomruntime.ToolCallResult{}, loomruntime.ErrInvalidToolCallEnvelope
	}
	sequence, sequenceBound := loomruntime.ToolCallSequence(ctx)
	if !sequenceBound {
		sequence = 1
	}
	callDigest := permissions.ProposedCallDigest(envelope.Call)
	operationID := productDeterministicUUID(
		"wbridge",
		envelope.JobID,
		binding.RunID,
		callDigest,
		strconv.FormatInt(sequence, 10),
	)
	var dispatchGate execution.ExecutionDispatchGate
	var diagnosticRecorder execution.ToolExecutionDiagnosticRecorder
	var gateway *productAttemptToolDispatchGate
	if hook.loops != nil {
		invocation, found := productAttemptLoopInvocationFromContext(ctx)
		if !found || !validBridgeProposalBinding(binding) ||
			!sameProductAttemptToolBinding(invocation, binding, envelope.JobID) ||
			sequence > int64(invocation.Budget.MaxToolCalls) {
			return loomruntime.ToolCallResult{}, loomruntime.ErrInvalidToolCallEnvelope
		}
		gateway = newProductAttemptToolDispatchGate(
			hook.loops, hook.payloads, invocation, envelope.Call, callDigest, operationID,
			sequence,
		)
		dispatchGate = gateway
		if hook.diagnostics != nil {
			diagnosticRecorder = &productAttemptToolDiagnosticObserver{
				recorder: hook.diagnostics, invocation: invocation,
				callID: gateway.callInput.CallID,
			}
		}
	}
	result, err := hook.adapter.Execute(ctx, execution.Proposal{
		JobID:            envelope.JobID,
		Call:             envelope.Call,
		OperationID:      operationID,
		JourneyID:        binding.JourneyID,
		DispatchGate:     dispatchGate,
		ResultCommitGate: gateway,
		Diagnostics:      diagnosticRecorder,
	})
	if err != nil {
		return loomruntime.ToolCallResult{}, err
	}
	defer result.Close()
	outcome := loomruntime.ToolCallResult{
		Verdict:            result.Verdict,
		ExecutionID:        result.ExecutionID,
		ApprovalID:         result.ApprovalID,
		ApprovalDigest:     result.ApprovalDigest,
		ResultNote:         result.Note,
		ErrorCode:          result.ErrorCode,
		DenialReason:       result.Denial.Reason,
		AuthorizationPath:  result.Denial.AuthorizationPath,
		ExitCode:           result.ExitCode,
		OutputDigest:       result.OutputDigest,
		ChangedFilesDigest: result.ChangedFilesDigest,
		EvidenceID:         result.EvidenceID,
		DurationMS:         result.DurationMS,
		ContentDigest:      result.OutputDigest,
	}
	if outcome.Verdict == permissions.VerdictAsk && outcome.ApprovalID != "" &&
		hook.proposals != nil {
		content, marshalErr := json.Marshal(envelope.Call)
		if marshalErr != nil {
			return loomruntime.ToolCallResult{}, marshalErr
		}
		defer clearProductBytes(content)
		proposalID := outcome.ApprovalID
		callID := "call-" + productDeterministicUUID(
			"tool-proposal", binding.RunID, callDigest,
		)
		if putErr := hook.proposals.PutToolProposal(ctx, toolproposal.Record{
			Binding: toolproposal.Binding{
				ProposalID: proposalID, ConversationID: binding.ConversationID,
				WorkItemID: binding.WorkItemID, RunID: binding.RunID,
				ClaimGeneration:        binding.ClaimGeneration,
				RuntimeInstanceID:      binding.RuntimeInstanceID,
				AgentInstanceID:        binding.AgentInstanceID,
				ExecutionBindingDigest: binding.ExecutionBindingDigest,
				CapsuleDigest:          binding.CapsuleDigest, CallID: callID,
				CallDigest: callDigest, ApprovalID: outcome.ApprovalID,
				ApprovalDigest: outcome.ApprovalDigest,
				Tool:           string(envelope.Call.Tool), OperationID: operationID,
				IncidentID:    binding.IncidentID,
				ContentDigest: toolproposal.ContentDigest(content),
			},
			Content: content,
		}); putErr != nil {
			return loomruntime.ToolCallResult{}, putErr
		}
	}
	if outcome.Verdict == permissions.VerdictAllow && gateway != nil {
		var delivery attemptpayload.Binding
		var deliveryErr error
		if productAttemptRemoteContentTool(gateway.call.Tool) {
			delivery, deliveryErr = gateway.acceptedRemoteResultBinding(ctx, outcome)
		} else {
			delivery, deliveryErr = hook.persistAttemptToolResult(
				ctx, gateway, outcome, result.Content,
			)
		}
		if deliveryErr != nil {
			hook.recordAttemptToolStage(
				ctx, gateway.invocation, gateway.callInput.CallID,
				outcome.ExecutionID, gateway.callDigest, gateway.call.Tool,
				gateway.operationID,
				execution.ToolStagePayloadCommit, execution.ToolDiagnosticFailed,
				"tool_payload_commit_failed", true,
			)
			return loomruntime.ToolCallResult{}, deliveryErr
		}
		hook.recordAttemptToolStage(
			ctx, gateway.invocation, gateway.callInput.CallID,
			outcome.ExecutionID, gateway.callDigest, gateway.call.Tool,
			gateway.operationID,
			execution.ToolStagePayloadCommit, execution.ToolDiagnosticSucceeded,
			"", false,
		)
		outcome.Delivery = &loomruntime.ToolCallDelivery{
			Binding: delivery, CallDigest: gateway.callDigest,
			Tool: gateway.call.Tool, OperationID: gateway.operationID,
		}
	}
	if outcome.Verdict == permissions.VerdictAllow &&
		outcome.ExecutionID == "" {
		return loomruntime.ToolCallResult{}, errors.New("allow verdict without execution id")
	}
	if err := advanceProductAttemptTurnAfterToolResult(
		ctx, sequence, outcome,
	); err != nil {
		return loomruntime.ToolCallResult{}, err
	}
	return outcome, nil
}

func advanceProductAttemptTurnAfterToolResult(
	ctx context.Context,
	sequence int64,
	result loomruntime.ToolCallResult,
) error {
	if ctx == nil || sequence < 1 {
		return loomruntime.ErrInvalidToolCallEnvelope
	}
	if result.Verdict == permissions.VerdictAsk {
		return nil
	}
	if result.Verdict != permissions.VerdictAllow &&
		result.Verdict != permissions.VerdictDeny {
		return loomruntime.ErrInvalidToolCallEnvelope
	}
	controller, found := productAttemptTurnScopeFromContext(ctx)
	if !found {
		return nil
	}
	return controller.AdvanceTurn(ctx, sequence)
}

type productAttemptToolDispatchGate struct {
	loops       *work.AttemptLoopAuthority
	payloads    attemptpayload.Store
	invocation  productAttemptLoopInvocation
	call        permissions.ProposedCall
	callDigest  string
	operationID string
	callInput   work.AttemptLoopToolCallInput
}

type productAttemptToolDiagnosticObserver struct {
	recorder   productAttemptToolDiagnosticRecorder
	invocation productAttemptLoopInvocation
	callID     string
}

func (observer *productAttemptToolDiagnosticObserver) RecordToolExecutionDiagnostic(
	ctx context.Context,
	diagnostic execution.ToolExecutionDiagnostic,
) error {
	if observer == nil || observer.recorder == nil {
		return nil
	}
	return observer.recorder.RecordAttemptToolDiagnostic(
		ctx,
		productAttemptToolDiagnosticFromExecution(
			observer.invocation, observer.callID, diagnostic,
		),
	)
}

func newProductAttemptToolDispatchGate(
	loops *work.AttemptLoopAuthority,
	payloads attemptpayload.Store,
	invocation productAttemptLoopInvocation,
	call permissions.ProposedCall,
	callDigest string,
	operationID string,
	sequence int64,
) *productAttemptToolDispatchGate {
	callID := "call-" + productDeterministicUUID(
		"attempt-tool-call", invocation.Binding.AttemptID, operationID, callDigest,
	)
	executionMode := work.ToolExecutionExclusive
	conflictScopeDigest := productAttemptLoopDigest(struct {
		Domain     string `json:"domain"`
		AttemptID  string `json:"attempt_id"`
		WorkItemID string `json:"work_item_id"`
	}{
		"loom/attempt-tool/conflict/v1", invocation.Binding.AttemptID,
		invocation.Binding.PayloadAuthority.WorkItemID,
	})
	if call.Tool == permissions.ToolRead || call.Tool == permissions.ToolGrep {
		executionMode = work.ToolExecutionParallel
		conflictScopeDigest = productAttemptLoopDigest(struct {
			Domain    string `json:"domain"`
			AttemptID string `json:"attempt_id"`
			Path      string `json:"path"`
		}{
			"loom/attempt-tool/read-conflict/v1", invocation.Binding.AttemptID,
			call.Path,
		})
	}
	return &productAttemptToolDispatchGate{
		loops: loops, payloads: payloads, invocation: invocation, call: call,
		callDigest: callDigest, operationID: operationID,
		callInput: work.AttemptLoopToolCallInput{
			TurnID: invocation.TurnID, StepID: invocation.StepID,
			CallID: callID, Sequence: sequence, Tool: call.Tool,
			TargetID: productAttemptToolTargetID(call.Tool),
			ProposalDigest: productAttemptLoopDigest(struct {
				Domain     string `json:"domain"`
				CallDigest string `json:"call_digest"`
			}{"loom/attempt-tool/proposal/v1", callDigest}),
			ArgumentsDigest: productAttemptLoopDigest(struct {
				Domain     string `json:"domain"`
				CallDigest string `json:"call_digest"`
			}{"loom/attempt-tool/arguments/v1", callDigest}),
			ToolSchemaDigest: productAttemptLoopLocalToolSchemaDigest(call.Tool),
			ExecutionMode:    executionMode, ConflictScopeDigest: conflictScopeDigest,
		},
	}
}

func (gate *productAttemptToolDispatchGate) CommitExecutionResult(
	ctx context.Context,
	input execution.ExecutionResultCommitInput,
) error {
	if gate == nil || gate.loops == nil || gate.payloads == nil ||
		!productAttemptRemoteContentTool(gate.call.Tool) ||
		input.JobID != gate.invocation.Binding.PayloadAuthority.WorkItemID ||
		input.CallDigest != gate.callDigest || input.Tool != gate.call.Tool ||
		input.OperationID != gate.operationID ||
		input.CorrelationID != gate.invocation.Binding.PayloadAuthority.IncidentID ||
		input.ExecutionID == "" || len(input.Content) == 0 ||
		input.OutputDigest != "sha256:"+toolproposal.ContentDigest(input.Content) {
		return work.ErrInvalidAttemptLoop
	}
	binding, err := gate.remoteResultBinding(input.ExecutionID, input.OutputDigest)
	if err != nil {
		return err
	}
	if err := gate.payloads.PutAttemptPayload(ctx, attemptpayload.Payload{
		Binding: binding, Status: attemptpayload.StatusPending, Content: input.Content,
	}); err != nil {
		return err
	}
	_, err = gate.loops.AcceptToolResult(ctx, gate.invocation.Binding, binding)
	return err
}

func (gate *productAttemptToolDispatchGate) acceptedRemoteResultBinding(
	ctx context.Context,
	result loomruntime.ToolCallResult,
) (attemptpayload.Binding, error) {
	if gate == nil || !productAttemptRemoteContentTool(gate.call.Tool) ||
		result.Verdict != permissions.VerdictAllow {
		return attemptpayload.Binding{}, work.ErrInvalidAttemptLoop
	}
	binding, err := gate.remoteResultBinding(result.ExecutionID, result.OutputDigest)
	if err != nil {
		return attemptpayload.Binding{}, err
	}
	fact, found, err := gate.loops.LookupToolResult(
		ctx, gate.invocation.Binding, gate.callInput.CallID, gate.callInput.Sequence,
	)
	if err != nil || !found || fact.Binding != binding ||
		(fact.Status != attemptpayload.FactAccepted &&
			fact.Status != attemptpayload.FactDelivered) {
		return attemptpayload.Binding{}, work.ErrInvalidAttemptLoop
	}
	return binding, nil
}

func (gate *productAttemptToolDispatchGate) remoteResultBinding(
	executionID string,
	outputDigest string,
) (attemptpayload.Binding, error) {
	if gate == nil || executionID == "" || len(outputDigest) != len("sha256:")+64 ||
		!strings.HasPrefix(outputDigest, "sha256:") {
		return attemptpayload.Binding{}, work.ErrInvalidAttemptLoop
	}
	return attemptpayload.Binding{
		PayloadID: "payload-" + productDeterministicUUID(
			"attempt-tool-result", gate.invocation.Binding.AttemptID,
			gate.callInput.CallID, executionID,
		),
		Scope:  gate.invocation.Binding.PayloadAuthority.Scope,
		CallID: gate.callInput.CallID, Sequence: gate.callInput.Sequence,
		ContentType:   attemptpayload.ContentTypeTextUTF8,
		ContentDigest: strings.TrimPrefix(outputDigest, "sha256:"),
	}, nil
}

func productAttemptRemoteContentTool(tool permissions.ToolKind) bool {
	return tool == permissions.ToolWebSearch || tool == permissions.ToolWebFetch ||
		tool == permissions.ToolMCPTool
}

func productAttemptToolTargetID(tool permissions.ToolKind) string {
	switch tool {
	case permissions.ToolWebSearch, permissions.ToolWebFetch:
		return "loom-web-broker"
	case permissions.ToolMCPTool:
		return "loom-mcp-registry"
	default:
		return "local-workspace"
	}
}

func (gate *productAttemptToolDispatchGate) CommitExecutionDispatch(
	ctx context.Context,
	input execution.ExecutionDispatchInput,
) error {
	if gate == nil || gate.loops == nil || input.JobID != gate.invocation.Binding.PayloadAuthority.WorkItemID ||
		input.CallDigest != gate.callDigest || input.Tool != gate.call.Tool ||
		input.OperationID != gate.operationID ||
		input.CorrelationID != gate.invocation.Binding.PayloadAuthority.IncidentID ||
		(input.ApprovalID == "") != (input.ApprovalDigest == "") {
		return work.ErrInvalidAttemptLoop
	}
	if _, err := gate.loops.AdmitToolCall(
		ctx, gate.invocation.Binding, gate.callInput,
	); err != nil {
		return err
	}
	authorizationDigest := productAttemptLoopDigest(struct {
		Domain           string `json:"domain"`
		PermissionDigest string `json:"permission_digest"`
		ExecutionID      string `json:"execution_id"`
		CallDigest       string `json:"call_digest"`
		ApprovalID       string `json:"approval_id,omitempty"`
		ApprovalDigest   string `json:"approval_digest,omitempty"`
	}{
		"loom/attempt-tool/authorization/v1",
		gate.invocation.Binding.PermissionProfileDigest,
		input.ExecutionID, input.CallDigest, input.ApprovalID, input.ApprovalDigest,
	})
	dispatchDigest := productAttemptLoopDigest(struct {
		Domain      string `json:"domain"`
		CallID      string `json:"call_id"`
		ExecutionID string `json:"execution_id"`
		OperationID string `json:"operation_id"`
		IncidentID  string `json:"incident_id"`
	}{
		"loom/attempt-tool/dispatch/v1", gate.callInput.CallID,
		input.ExecutionID, input.OperationID, input.CorrelationID,
	})
	_, err := gate.loops.CommitToolDispatch(
		ctx,
		gate.invocation.Binding,
		work.AttemptLoopToolDispatchInput{
			TurnID: gate.invocation.TurnID, StepID: gate.invocation.StepID,
			CallID: gate.callInput.CallID, AuthorizationDigest: authorizationDigest,
			ApprovalID: input.ApprovalID, DispatchDigest: dispatchDigest,
		},
	)
	return err
}

func (hook *bridgeExecutionHook) persistAttemptToolResult(
	ctx context.Context,
	gate *productAttemptToolDispatchGate,
	result loomruntime.ToolCallResult,
	resultContent []byte,
) (attemptpayload.Binding, error) {
	if hook == nil || hook.payloads == nil || hook.loops == nil || gate == nil {
		return attemptpayload.Binding{}, work.ErrInvalidAttemptLoop
	}
	var content []byte
	var err error
	contentType := attemptpayload.ContentTypeJSON
	if gate.call.Tool == permissions.ToolRead || gate.call.Tool == permissions.ToolGrep {
		if len(resultContent) == 0 ||
			result.OutputDigest != "sha256:"+toolproposal.ContentDigest(resultContent) {
			return attemptpayload.Binding{}, work.ErrInvalidAttemptLoop
		}
		content = bytes.Clone(resultContent)
		contentType = attemptpayload.ContentTypeTextUTF8
	} else {
		content, err = json.Marshal(struct {
			SchemaVersion      int    `json:"schema_version"`
			Verdict            string `json:"verdict"`
			ExecutionID        string `json:"execution_id"`
			ExitCode           int    `json:"exit_code"`
			OutputDigest       string `json:"output_digest,omitempty"`
			ChangedFilesDigest string `json:"changed_files_digest,omitempty"`
			EvidenceID         string `json:"evidence_id,omitempty"`
			DurationMS         int64  `json:"duration_ms"`
		}{
			1, string(result.Verdict), result.ExecutionID, result.ExitCode,
			result.OutputDigest, result.ChangedFilesDigest, result.EvidenceID, result.DurationMS,
		})
	}
	if err != nil || len(content) == 0 || int64(len(content)) > gate.invocation.Budget.MaxResultBytes {
		clearProductBytes(content)
		return attemptpayload.Binding{}, work.ErrInvalidAttemptLoop
	}
	defer clearProductBytes(content)
	binding := attemptpayload.Binding{
		PayloadID: "payload-" + productDeterministicUUID(
			"attempt-tool-result", gate.invocation.Binding.AttemptID,
			gate.callInput.CallID, result.ExecutionID,
		),
		Scope:  gate.invocation.Binding.PayloadAuthority.Scope,
		CallID: gate.callInput.CallID, Sequence: gate.callInput.Sequence,
		ContentType: contentType, ContentDigest: toolproposal.ContentDigest(content),
	}
	if err := hook.payloads.PutAttemptPayload(ctx, attemptpayload.Payload{
		Binding: binding, Status: attemptpayload.StatusPending, Content: content,
	}); err != nil {
		return attemptpayload.Binding{}, err
	}
	if _, err := hook.loops.AcceptToolResult(
		ctx, gate.invocation.Binding, binding,
	); err != nil {
		return attemptpayload.Binding{}, err
	}
	if gate.call.Tool == permissions.ToolBash {
		command, recognizeErr := verification.RecognizeGovernedTestCommand(
			gate.call.Command,
		)
		switch {
		case recognizeErr == nil:
			report, reportErr := verification.NewGovernedTestReport(
				command,
				verification.GovernedTestReportInput{
					CallID:          gate.callInput.CallID,
					CallSequence:    gate.callInput.Sequence,
					ArgumentsDigest: gate.callInput.ArgumentsDigest,
					ExecutionID:     result.ExecutionID, ExitCode: result.ExitCode,
					OutputDigest: result.OutputDigest, DurationMS: result.DurationMS,
				},
			)
			if reportErr != nil {
				return attemptpayload.Binding{}, reportErr
			}
			if _, reportErr = hook.loops.CommitGovernedTestReport(
				ctx, gate.invocation.Binding, binding, report,
			); reportErr != nil {
				return attemptpayload.Binding{}, reportErr
			}
		case errors.Is(recognizeErr, verification.ErrUnsupportedTestCommand):
		default:
			return attemptpayload.Binding{}, recognizeErr
		}
	}
	return binding, nil
}

func (hook *bridgeExecutionHook) ReadToolCallResultContent(
	ctx context.Context,
	binding loomruntime.ToolCallBinding,
	result loomruntime.ToolCallResult,
) ([]byte, error) {
	invocation, found := productAttemptLoopInvocationFromContext(ctx)
	if hook == nil || hook.payloads == nil || !found || result.Delivery == nil ||
		!productAttemptContentTool(result.Delivery.Tool) ||
		!sameProductAttemptToolBinding(invocation, binding, binding.WorkItemID) ||
		result.Delivery.Binding.Scope != invocation.Binding.PayloadAuthority.Scope ||
		result.ContentDigest == "" || result.ContentDigest != result.OutputDigest {
		return nil, work.ErrInvalidAttemptLoop
	}
	payload, err := hook.payloads.ReadAttemptPayload(ctx, result.Delivery.Binding)
	if err != nil {
		return nil, err
	}
	defer payload.Close()
	if payload.Status != attemptpayload.StatusPending ||
		payload.Binding != result.Delivery.Binding ||
		payload.Binding.ContentType != attemptpayload.ContentTypeTextUTF8 ||
		payload.Binding.ContentDigest != toolproposal.ContentDigest(payload.Content) ||
		result.ContentDigest != "sha256:"+payload.Binding.ContentDigest {
		return nil, work.ErrInvalidAttemptLoop
	}
	return bytes.Clone(payload.Content), nil
}

func productAttemptContentTool(tool permissions.ToolKind) bool {
	return tool == permissions.ToolRead || tool == permissions.ToolGrep ||
		productAttemptRemoteContentTool(tool)
}

func (hook *bridgeExecutionHook) AcknowledgeToolCallResult(
	ctx context.Context,
	binding loomruntime.ToolCallBinding,
	result loomruntime.ToolCallResult,
) error {
	return hook.AcknowledgeToolCallResultWithProof(
		ctx, binding, result, attemptpayload.ProofRunStreamToolResult,
	)
}

func (hook *bridgeExecutionHook) AcknowledgeToolCallResultWithProof(
	ctx context.Context,
	binding loomruntime.ToolCallBinding,
	result loomruntime.ToolCallResult,
	proof attemptpayload.DeliveryProof,
) error {
	if result.Delivery == nil {
		return nil
	}
	invocation, found := productAttemptLoopInvocationFromContext(ctx)
	if hook == nil || hook.loops == nil || hook.payloads == nil || !found ||
		!validBridgeProposalBinding(binding) ||
		!sameProductAttemptToolBinding(invocation, binding, binding.WorkItemID) ||
		result.Verdict != permissions.VerdictAllow ||
		result.Delivery.Binding.Scope != invocation.Binding.PayloadAuthority.Scope ||
		(proof != attemptpayload.ProofRunStreamToolResult &&
			proof != attemptpayload.ProofHarnessFinalOutput) {
		return work.ErrInvalidAttemptLoop
	}
	if _, err := hook.loops.DeliverToolResult(
		ctx, invocation.Binding, result.Delivery.Binding,
		proof,
	); err != nil {
		hook.recordAttemptToolStage(
			ctx, invocation, result.Delivery.Binding.CallID,
			result.ExecutionID, result.Delivery.CallDigest,
			result.Delivery.Tool, result.Delivery.OperationID,
			execution.ToolStageResultDelivery, execution.ToolDiagnosticFailed,
			"tool_result_delivery_failed", true,
		)
		return err
	}
	if err := hook.payloads.MarkAttemptPayloadDelivered(ctx, result.Delivery.Binding); err != nil {
		hook.recordAttemptToolStage(
			ctx, invocation, result.Delivery.Binding.CallID,
			result.ExecutionID, result.Delivery.CallDigest,
			result.Delivery.Tool, result.Delivery.OperationID,
			execution.ToolStageResultDelivery, execution.ToolDiagnosticFailed,
			"tool_result_delivery_failed", true,
		)
		return err
	}
	hook.recordAttemptToolStage(
		ctx, invocation, result.Delivery.Binding.CallID,
		result.ExecutionID, result.Delivery.CallDigest,
		result.Delivery.Tool, result.Delivery.OperationID,
		execution.ToolStageResultDelivery, execution.ToolDiagnosticSucceeded,
		"", false,
	)
	return nil
}

func (hook *bridgeExecutionHook) recordAttemptToolStage(
	ctx context.Context,
	invocation productAttemptLoopInvocation,
	callID string,
	executionID string,
	callDigest string,
	tool permissions.ToolKind,
	operationID string,
	stage execution.ToolExecutionStage,
	diagnosticResult string,
	errorCode string,
	retryable bool,
) {
	if hook == nil || hook.diagnostics == nil {
		return
	}
	_ = hook.diagnostics.RecordAttemptToolDiagnostic(
		ctx,
		productAttemptToolDiagnosticFromExecution(
			invocation,
			callID,
			execution.ToolExecutionDiagnostic{
				ExecutionID:   executionID,
				JobID:         invocation.Binding.PayloadAuthority.WorkItemID,
				CallDigest:    callDigest,
				Tool:          tool,
				OperationID:   operationID,
				CorrelationID: invocation.Binding.PayloadAuthority.IncidentID,
				Stage:         stage, Result: diagnosticResult,
				ErrorCode: errorCode, Retryable: retryable,
			},
		),
	)
}

func productAttemptToolDiagnosticFromExecution(
	invocation productAttemptLoopInvocation,
	callID string,
	diagnostic execution.ToolExecutionDiagnostic,
) productAttemptToolDiagnostic {
	return productAttemptToolDiagnostic{
		IncidentID:             invocation.Binding.PayloadAuthority.IncidentID,
		ProviderID:             invocation.ExecutionBinding.ProviderID,
		ProviderAccountID:      invocation.ExecutionBinding.ProviderAccountID,
		ModelID:                invocation.ExecutionBinding.ModelID,
		WorkItemID:             invocation.Binding.PayloadAuthority.WorkItemID,
		RunID:                  invocation.Binding.PayloadAuthority.RunID,
		ClaimGeneration:        invocation.Binding.PayloadAuthority.ClaimGeneration,
		RuntimeInstanceID:      invocation.Binding.PayloadAuthority.RuntimeInstanceID,
		AgentInstanceID:        invocation.Binding.PayloadAuthority.AgentInstanceID,
		ExecutionBindingDigest: invocation.Binding.PayloadAuthority.ExecutionBindingDigest,
		CapsuleDigest:          invocation.Binding.PayloadAuthority.CapsuleDigest,
		CallID:                 callID, Execution: diagnostic,
	}
}

func sameProductAttemptToolBinding(
	invocation productAttemptLoopInvocation,
	binding loomruntime.ToolCallBinding,
	jobID string,
) bool {
	authority := invocation.Binding.PayloadAuthority
	return invocation.TurnID != "" && invocation.StepID != "" &&
		jobID == authority.WorkItemID && binding.ConversationID == authority.ConversationID &&
		binding.WorkItemID == authority.WorkItemID && binding.RunID == authority.RunID &&
		binding.ClaimGeneration == authority.ClaimGeneration &&
		binding.RuntimeInstanceID == authority.RuntimeInstanceID &&
		binding.AgentInstanceID == authority.AgentInstanceID &&
		binding.ExecutionBindingDigest == authority.ExecutionBindingDigest &&
		binding.CapsuleDigest == authority.CapsuleDigest && binding.ClaimID == authority.ClaimID &&
		binding.IncidentID == authority.IncidentID && binding.JourneyID == authority.IncidentID
}

func productAttemptLoopLocalToolSchemaDigest(tool permissions.ToolKind) string {
	return productAttemptLoopDigest(struct {
		Domain  string `json:"domain"`
		Name    string `json:"name"`
		Version int    `json:"version"`
	}{"loom/product-attempt-loop/local-tool-schema/v1", string(tool), 1})
}

func validBridgeProposalBinding(binding loomruntime.ToolCallBinding) bool {
	return binding.ConversationID != "" && binding.WorkItemID != "" &&
		binding.RunID != "" && binding.ClaimGeneration > 0 &&
		binding.RuntimeInstanceID != "" && binding.AgentInstanceID != "" &&
		binding.ExecutionBindingDigest != "" && binding.CapsuleDigest != "" &&
		binding.ClaimID != "" && binding.IncidentID != "" &&
		binding.JourneyID == binding.IncidentID
}

func clearProductBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
