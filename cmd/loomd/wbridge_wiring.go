package main

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

// bridgeExecutionHook 把桥接信封接到 B-W1 执行适配器。桥接不含执行决策：
// Execute 内部完成 Evaluate/ask/deny 与 Journal 事实（不变量 3）。
type bridgeExecutionHook struct {
	adapter *execution.Adapter
}

func newBridgeExecutionHook(adapter *execution.Adapter) (*bridgeExecutionHook, error) {
	if adapter == nil {
		return nil, execution.ErrInvalidExecutionInput
	}
	return &bridgeExecutionHook{adapter: adapter}, nil
}

func (hook *bridgeExecutionHook) ExecuteToolCall(
	ctx context.Context,
	envelope piadapter.ToolCallEnvelope,
	binding piadapter.ToolCallBinding,
) (piadapter.ToolCallResult, error) {
	if hook == nil || hook.adapter == nil {
		return piadapter.ToolCallResult{}, piadapter.ErrToolCallHookUnavailable
	}
	operationID := productDeterministicUUID(
		"wbridge",
		envelope.JobID,
		binding.RunID,
		string(envelope.Call.Tool),
		envelope.Call.Command,
		envelope.Call.Path,
	)
	result, err := hook.adapter.Execute(ctx, execution.Proposal{
		JobID:       envelope.JobID,
		Call:        envelope.Call,
		OperationID: operationID,
		JourneyID:   binding.JourneyID,
	})
	if err != nil {
		return piadapter.ToolCallResult{}, err
	}
	outcome := piadapter.ToolCallResult{
		Verdict:           result.Verdict,
		ExecutionID:       result.ExecutionID,
		ApprovalID:        result.ApprovalID,
		ApprovalDigest:    result.ApprovalDigest,
		ResultNote:        result.Note,
		DenialReason:      result.Denial.Reason,
		AuthorizationPath: result.Denial.AuthorizationPath,
	}
	if outcome.Verdict == permissions.VerdictAllow &&
		outcome.ExecutionID == "" {
		return piadapter.ToolCallResult{}, errors.New("allow verdict without execution id")
	}
	return outcome, nil
}
