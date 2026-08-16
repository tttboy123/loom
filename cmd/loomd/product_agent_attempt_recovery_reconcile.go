package main

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/work"
)

type productAttemptRecoveryTerminalReconciler struct {
	runs   *work.Authority
	grants *authorization.Authority
}

type productAttemptRecoveryTerminalReport struct {
	TerminalRuns  int
	RevokedGrants int
	Outcomes      []productAttemptRecoveryTerminalOutcome
}

type productAttemptRecoveryTerminalOutcome struct {
	WorkItemID             string
	RunID                  string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	AgentInstanceID        string
	ProviderID             string
	ProviderAccountID      string
	ModelID                string
	ExecutionBindingDigest string
}

func newProductAttemptRecoveryTerminalReconciler(
	runs *work.Authority,
	grants *authorization.Authority,
) (*productAttemptRecoveryTerminalReconciler, error) {
	if runs == nil || grants == nil {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	return &productAttemptRecoveryTerminalReconciler{runs: runs, grants: grants}, nil
}

// Reconcile closes only stale execution authorization. A terminal Run is the
// authority, so this path never owns a Runtime or Provider dispatch operation.
func (reconciler *productAttemptRecoveryTerminalReconciler) Reconcile(
	ctx context.Context,
	correlationID string,
) (productAttemptRecoveryTerminalReport, error) {
	if reconciler == nil || reconciler.runs == nil || reconciler.grants == nil ||
		ctx == nil || ctx.Err() != nil || correlationID == "" {
		return productAttemptRecoveryTerminalReport{},
			errProductInvalidAttemptRecoveryCompletion
	}
	runSnapshot, err := reconciler.runs.Snapshot(ctx)
	if err != nil {
		return productAttemptRecoveryTerminalReport{}, err
	}
	grantSnapshot, err := reconciler.grants.Snapshot(ctx)
	if err != nil {
		return productAttemptRecoveryTerminalReport{}, err
	}
	terminal := make(map[string]work.RunRecord)
	report := productAttemptRecoveryTerminalReport{}
	for _, run := range runSnapshot.Runs() {
		if run.Phase() != "terminal" {
			continue
		}
		terminal[run.ID()] = run
		report.TerminalRuns++
	}
	pending := make(map[string]authorization.GrantRecord)
	for _, grant := range grantSnapshot.Grants() {
		if !grant.RevokedAt().IsZero() {
			continue
		}
		run, found := terminal[grant.RunID()]
		if !found {
			continue
		}
		if grant.WorkItemID() != run.WorkItemID() || grant.ClaimID() != run.ClaimID() ||
			grant.ClaimGeneration() != run.ClaimGeneration() ||
			grant.RuntimeInstanceID() != run.RuntimeInstanceID() ||
			grant.AgentInstanceID() != run.AgentInstanceID() {
			return productAttemptRecoveryTerminalReport{},
				errProductAttemptRecoveryGrantConflict
		}
		if _, duplicate := pending[run.ID()]; duplicate {
			return productAttemptRecoveryTerminalReport{},
				errProductAttemptRecoveryGrantConflict
		}
		pending[run.ID()] = grant
	}
	for _, run := range runSnapshot.Runs() {
		grant, found := pending[run.ID()]
		if !found {
			continue
		}
		reason := authorization.RevocationTerminal
		if run.TerminalStatus() == "cancelled" {
			reason = authorization.RevocationCancelled
		}
		if _, err := reconciler.grants.Revoke(ctx, authorization.RevokeInput{
			GrantID: grant.ID(), Reason: reason, CorrelationID: correlationID,
		}); err != nil {
			return productAttemptRecoveryTerminalReport{}, errors.Join(
				errProductAttemptRecoveryGrantConflict, err,
			)
		}
		report.RevokedGrants++
		binding := run.ExecutionBinding()
		report.Outcomes = append(report.Outcomes, productAttemptRecoveryTerminalOutcome{
			WorkItemID: run.WorkItemID(), RunID: run.ID(),
			ClaimGeneration:        run.ClaimGeneration(),
			RuntimeInstanceID:      run.RuntimeInstanceID(),
			AgentInstanceID:        run.AgentInstanceID(),
			ProviderID:             binding.ProviderID,
			ProviderAccountID:      binding.ProviderAccountID,
			ModelID:                binding.ModelID,
			ExecutionBindingDigest: binding.BindingDigest,
		})
	}
	return report, nil
}

func recordProductAttemptRecoveryTerminalReconciliation(
	ctx context.Context,
	diagnostics nativeadapter.AgentAttemptDiagnosticRecorder,
	now func() time.Time,
	incidentID string,
	report productAttemptRecoveryTerminalReport,
) error {
	if ctx == nil || ctx.Err() != nil || now == nil ||
		report.TerminalRuns < 0 || report.RevokedGrants < 0 ||
		report.RevokedGrants != len(report.Outcomes) {
		return errProductInvalidAttemptRecoveryCompletion
	}
	if len(report.Outcomes) == 0 {
		return nil
	}
	sink, ok := diagnostics.(productOperationalDiagnosticSink)
	if !ok || nilProductAgentInterface(sink) {
		return errProductInvalidAttemptRecoveryCompletion
	}
	for _, outcome := range report.Outcomes {
		occurredAt := now()
		if occurredAt.IsZero() || occurredAt.Location() != time.UTC {
			return errProductInvalidAttemptRecoveryCompletion
		}
		if err := sink.append(productOperationalDiagnosticRecord{
			SchemaVersion: 1, OccurredAt: occurredAt.Format(time.RFC3339Nano),
			IncidentID: incidentID, Operation: "authorization_reconcile",
			CredentialRuntime: sink.credentialRuntimeValue(),
			ProviderID:        outcome.ProviderID, ProviderAccountID: outcome.ProviderAccountID,
			ModelID: outcome.ModelID, WorkItemID: outcome.WorkItemID,
			RunID: outcome.RunID, ClaimGeneration: outcome.ClaimGeneration,
			RuntimeInstanceID: outcome.RuntimeInstanceID, AgentID: outcome.AgentInstanceID,
			ExecutionBindingDigest: outcome.ExecutionBindingDigest,
			Stage:                  "agent_attempt_reconcile", Result: "succeeded",
		}); err != nil {
			return err
		}
	}
	return nil
}
