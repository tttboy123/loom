package main

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

const productAttemptRecoveryCompletionTimeout = 5 * time.Second

var (
	errProductInvalidAttemptRecoveryCompletion = errors.New("invalid Agent Attempt recovery completion")
	errProductAttemptRecoveryGrantConflict     = errors.New("Agent Attempt recovery grant conflict")
)

type productRecoveryRunTerminalAuthority interface {
	ResolveRecoveryAccounting(
		context.Context,
		work.AgentAttemptRestartOutcome,
		*work.RunAccounting,
	) (*work.RunAccounting, error)
	CommitTerminal(
		context.Context,
		work.RunTerminalInput,
	) (work.WorkItemRecord, work.RunRecord, error)
}

type productWorkRecoveryTerminalAuthority struct {
	authority *work.Authority
}

type productRecoveryGrantClosureAuthority interface {
	ResolveOriginalGrant(
		context.Context,
		work.AgentAttemptRestartOutcome,
	) (supervisor.RecoveryGrantBinding, error)
	RevokeOriginalGrant(
		context.Context,
		supervisor.RecoveryGrantBinding,
		authorization.RevocationReason,
		string,
	) error
}

type productRecoveryFrameObserverFactory interface {
	RecoveryFrameObserver(
		context.Context,
		work.AgentAttemptRestartOutcome,
	) (supervisor.AuthorizedFrameObserver, error)
}

type productAgentAttemptRecoveryCompletion struct {
	runtime   *productAgentAttemptRecoveryRuntime
	runs      productRecoveryRunTerminalAuthority
	grants    productRecoveryGrantClosureAuthority
	observer  supervisor.AuthorizedFrameObserver
	observers productRecoveryFrameObserverFactory
}

type productAuthorizationRecoveryGrantClosure struct {
	authority *authorization.Authority
}

func newProductAgentAttemptRecoveryCompletionWithObserverFactory(
	runtime *productAgentAttemptRecoveryRuntime,
	runs productRecoveryRunTerminalAuthority,
	grants productRecoveryGrantClosureAuthority,
	observers productRecoveryFrameObserverFactory,
) (*productAgentAttemptRecoveryCompletion, error) {
	if runtime == nil || nilProductAgentInterface(runs) ||
		nilProductAgentInterface(grants) || nilProductAgentInterface(observers) {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	return &productAgentAttemptRecoveryCompletion{
		runtime: runtime, runs: runs, grants: grants, observers: observers,
	}, nil
}

func newProductAgentAttemptRecoveryCompletion(
	runtime *productAgentAttemptRecoveryRuntime,
	runs productRecoveryRunTerminalAuthority,
	grants productRecoveryGrantClosureAuthority,
	observer supervisor.AuthorizedFrameObserver,
) (*productAgentAttemptRecoveryCompletion, error) {
	if runtime == nil || nilProductAgentInterface(runs) ||
		nilProductAgentInterface(grants) ||
		observer != nil && nilProductAgentInterface(observer) {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	return &productAgentAttemptRecoveryCompletion{
		runtime: runtime, runs: runs, grants: grants, observer: observer,
	}, nil
}

func newProductAuthorizationRecoveryGrantClosure(
	authority *authorization.Authority,
) (*productAuthorizationRecoveryGrantClosure, error) {
	if authority == nil {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	return &productAuthorizationRecoveryGrantClosure{authority: authority}, nil
}

func newProductWorkRecoveryTerminalAuthority(
	authority *work.Authority,
) (*productWorkRecoveryTerminalAuthority, error) {
	if authority == nil {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	return &productWorkRecoveryTerminalAuthority{authority: authority}, nil
}

func (completion *productAgentAttemptRecoveryCompletion) Resume(
	ctx context.Context,
	lease *work.AgentAttemptRecoveryDispatchLease,
) (result supervisor.AdapterResult, resultErr error) {
	if completion == nil || completion.runtime == nil || ctx == nil ||
		ctx.Err() != nil || lease == nil || nilProductAgentInterface(completion.runs) ||
		nilProductAgentInterface(completion.grants) {
		return supervisor.AdapterResult{}, errProductInvalidAttemptRecoveryCompletion
	}
	grant, err := lease.Take()
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(
			errProductInvalidAttemptRecoveryCompletion, err,
		)
	}
	outcome, _, err := work.ValidateAgentAttemptRecoveryDispatchGrant(grant)
	if err != nil {
		return supervisor.AdapterResult{}, errors.Join(
			errProductInvalidAttemptRecoveryCompletion, err,
		)
	}
	original, err := completion.grants.ResolveOriginalGrant(ctx, outcome)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	dispatchMessageID := productDeterministicUUID(
		"agent-attempt-recovery-dispatch", grant.LeaseID(),
	)
	observer := completion.observer
	if completion.observers != nil {
		observer, err = completion.observers.RecoveryFrameObserver(
			ctx, outcome,
		)
		if err != nil || nilProductAgentInterface(observer) {
			return supervisor.AdapterResult{}, errors.Join(
				errProductInvalidAttemptRecoveryCompletion, err,
			)
		}
	}
	frames, err := supervisor.NewRecoveryFrameAuthority(
		grant, original, dispatchMessageID, observer,
	)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	attachment, _, err := completion.runtime.attachGrant(ctx, grant)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, attachment.Close())
	}()
	continuation, ok := attachment.session.(productAgentAttemptRecoveryContinuationSession)
	if !ok || nilProductAgentInterface(continuation) {
		return supervisor.AdapterResult{}, errProductInvalidAttemptRecoveryAttachment
	}
	var continueErr error
	result, continueErr = continuation.ContinueAgentAttempt(ctx, frames)
	if continueErr != nil {
		return result, continueErr
	}
	_, status, _, err := frames.ValidateAdapterResult(ctx, result)
	if err != nil {
		return result, err
	}
	if result.ExitCode() != 0 {
		return result, supervisor.ErrRuntimeAdapter
	}
	reason := ""
	if status == "failed" {
		reason = "agent_attempt_recovery_failed"
	}
	accounting, accountingAvailable := result.Accounting()
	var accountingInput *work.RunAccounting
	if accountingAvailable {
		accountingInput = &accounting
	}
	terminalContext, cancelTerminal := context.WithTimeout(
		context.WithoutCancel(ctx), productAttemptRecoveryCompletionTimeout,
	)
	defer cancelTerminal()
	accountingInput, err = completion.runs.ResolveRecoveryAccounting(
		terminalContext, outcome, accountingInput,
	)
	if err != nil {
		return result, err
	}
	authority := outcome.Binding.PayloadAuthority
	_, _, terminalErr := completion.runs.CommitTerminal(
		terminalContext,
		work.RunTerminalInput{
			RunGenerationInput: work.RunGenerationInput{
				WorkItemID: authority.WorkItemID, RunID: authority.RunID,
				ClaimID: authority.ClaimID, ClaimGeneration: authority.ClaimGeneration,
				RuntimeInstanceID: authority.RuntimeInstanceID,
				AgentInstanceID:   authority.AgentInstanceID,
				CorrelationID:     grant.IncidentID(),
			},
			Status: status, Reason: reason, Accounting: accountingInput,
		},
	)
	if terminalErr != nil {
		return result, terminalErr
	}
	revokeContext, cancelRevoke := context.WithTimeout(
		context.WithoutCancel(ctx), productAttemptRecoveryCompletionTimeout,
	)
	revokeErr := completion.grants.RevokeOriginalGrant(
		revokeContext, original, authorization.RevocationTerminal, grant.IncidentID(),
	)
	cancelRevoke()
	return result, revokeErr
}

func (authority *productWorkRecoveryTerminalAuthority) ResolveRecoveryAccounting(
	ctx context.Context,
	outcome work.AgentAttemptRestartOutcome,
	accounting *work.RunAccounting,
) (*work.RunAccounting, error) {
	if authority == nil || authority.authority == nil || ctx == nil || ctx.Err() != nil {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	validated, err := work.ValidateAgentAttemptRestartOutcome(outcome)
	if err != nil {
		return nil, errors.Join(errProductInvalidAttemptRecoveryCompletion, err)
	}
	if accounting != nil {
		if err := work.ValidateRunAccounting(*accounting); err != nil {
			return nil, errors.Join(errProductInvalidAttemptRecoveryCompletion, err)
		}
	}
	snapshot, err := authority.authority.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	payload := validated.Binding.PayloadAuthority
	for _, run := range snapshot.Runs() {
		if run.ID() != payload.RunID {
			continue
		}
		if run.WorkItemID() != payload.WorkItemID || run.Phase() != "running" ||
			run.ClaimID() != payload.ClaimID ||
			run.ClaimGeneration() != payload.ClaimGeneration ||
			run.RuntimeInstanceID() != payload.RuntimeInstanceID ||
			run.AgentInstanceID() != payload.AgentInstanceID ||
			run.ExecutionBinding().BindingDigest !=
				validated.ExecutionBinding.BindingDigest {
			return nil, errProductInvalidAttemptRecoveryCompletion
		}
		if accounting == nil {
			return nil, nil
		}
		resolved := *accounting
		if rateCard, available := run.ProviderModelRateCard(); available && !resolved.CostObserved {
			resolved, err = work.EstimateRunAccounting(resolved, rateCard)
			if err != nil {
				return nil, err
			}
		}
		return &resolved, nil
	}
	return nil, errProductInvalidAttemptRecoveryCompletion
}

func (authority *productWorkRecoveryTerminalAuthority) CommitTerminal(
	ctx context.Context,
	input work.RunTerminalInput,
) (work.WorkItemRecord, work.RunRecord, error) {
	if authority == nil || authority.authority == nil {
		return work.WorkItemRecord{}, work.RunRecord{},
			errProductInvalidAttemptRecoveryCompletion
	}
	return authority.authority.CommitTerminal(ctx, input)
}

func (closure *productAuthorizationRecoveryGrantClosure) ResolveOriginalGrant(
	ctx context.Context,
	outcome work.AgentAttemptRestartOutcome,
) (supervisor.RecoveryGrantBinding, error) {
	if closure == nil || closure.authority == nil || ctx == nil || ctx.Err() != nil {
		return supervisor.RecoveryGrantBinding{}, errProductAttemptRecoveryGrantConflict
	}
	validated, err := work.ValidateAgentAttemptRestartOutcome(outcome)
	if err != nil {
		return supervisor.RecoveryGrantBinding{}, errors.Join(
			errProductAttemptRecoveryGrantConflict, err,
		)
	}
	snapshot, err := closure.authority.Snapshot(ctx)
	if err != nil {
		return supervisor.RecoveryGrantBinding{}, err
	}
	authority := validated.Binding.PayloadAuthority
	var matched *authorization.GrantRecord
	for _, record := range snapshot.Grants() {
		if record.WorkItemID() != authority.WorkItemID || record.RunID() != authority.RunID ||
			record.ClaimID() != authority.ClaimID ||
			record.ClaimGeneration() != authority.ClaimGeneration ||
			record.RuntimeInstanceID() != authority.RuntimeInstanceID ||
			record.AgentInstanceID() != authority.AgentInstanceID ||
			!record.RevokedAt().IsZero() {
			continue
		}
		if matched != nil {
			return supervisor.RecoveryGrantBinding{}, errProductAttemptRecoveryGrantConflict
		}
		candidate := record
		matched = &candidate
	}
	if matched == nil {
		return supervisor.RecoveryGrantBinding{}, errProductAttemptRecoveryGrantConflict
	}
	return supervisor.RecoveryGrantBinding{
		GrantID: matched.ID(), WorkItemID: matched.WorkItemID(), RunID: matched.RunID(),
		ClaimID: matched.ClaimID(), ClaimGeneration: matched.ClaimGeneration(),
		RuntimeInstanceID: matched.RuntimeInstanceID(),
		AgentInstanceID:   matched.AgentInstanceID(),
		AllowedOperations: matched.AllowedOperations(),
	}, nil
}

func (closure *productAuthorizationRecoveryGrantClosure) RevokeOriginalGrant(
	ctx context.Context,
	binding supervisor.RecoveryGrantBinding,
	reason authorization.RevocationReason,
	correlationID string,
) error {
	if closure == nil || closure.authority == nil || binding.GrantID == "" {
		return errProductAttemptRecoveryGrantConflict
	}
	_, err := closure.authority.Revoke(ctx, authorization.RevokeInput{
		GrantID: binding.GrantID, Reason: reason, CorrelationID: correlationID,
	})
	return err
}
