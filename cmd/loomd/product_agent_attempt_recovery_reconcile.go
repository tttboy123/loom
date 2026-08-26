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
	now    func() time.Time
}

type productAttemptRecoveryTerminalReport struct {
	TerminalRuns     int
	RecoveredRuns    int
	RevokedGrants    int
	RecoveryOutcomes []productAttemptRecoveryTerminalOutcome
	Outcomes         []productAttemptRecoveryTerminalOutcome
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
	return &productAttemptRecoveryTerminalReconciler{
		runs: runs, grants: grants, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// Reconcile closes only stale execution authorization. A terminal Run is the
// authority, so this path never owns a Runtime or Provider dispatch operation.
func (reconciler *productAttemptRecoveryTerminalReconciler) Reconcile(
	ctx context.Context,
	correlationID string,
) (productAttemptRecoveryTerminalReport, error) {
	if reconciler == nil || reconciler.runs == nil || reconciler.grants == nil ||
		reconciler.now == nil || ctx == nil || ctx.Err() != nil || correlationID == "" {
		return productAttemptRecoveryTerminalReport{},
			errProductInvalidAttemptRecoveryCompletion
	}
	now := reconciler.now()
	if now.IsZero() || now.Location() != time.UTC {
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
		switch run.Phase() {
		case "terminal":
			terminal[run.ID()] = run
			report.TerminalRuns++
		case "claimed":
			// An expired prepare lease is an abandoned attempt, not evidence of
			// provider work. Close only this run so an explicit New Attempt can
			// safely reopen the Team without fabricating agent output.
			if run.PrepareLeaseExpiresAt().IsZero() || now.Before(run.PrepareLeaseExpiresAt()) {
				continue
			}
			_, closed, err := reconciler.runs.CommitTerminal(ctx, work.RunTerminalInput{
				RunGenerationInput: work.RunGenerationInput{
					WorkItemID: run.WorkItemID(), RunID: run.ID(),
					ClaimID: run.ClaimID(), ClaimGeneration: run.ClaimGeneration(),
					RuntimeInstanceID: run.RuntimeInstanceID(), AgentInstanceID: run.AgentInstanceID(),
					CorrelationID: correlationID,
				},
				Status: "failed", Reason: "agent_attempt_recovery_required",
			})
			if err != nil {
				return productAttemptRecoveryTerminalReport{}, err
			}
			terminal[closed.ID()] = closed
			report.RecoveredRuns++
			report.RecoveryOutcomes = append(report.RecoveryOutcomes, recoveryOutcome(closed))
		}
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

func recoveryOutcome(run work.RunRecord) productAttemptRecoveryTerminalOutcome {
	binding := run.ExecutionBinding()
	return productAttemptRecoveryTerminalOutcome{
		WorkItemID: run.WorkItemID(), RunID: run.ID(),
		ClaimGeneration: run.ClaimGeneration(), RuntimeInstanceID: run.RuntimeInstanceID(),
		AgentInstanceID: run.AgentInstanceID(), ProviderID: binding.ProviderID,
		ProviderAccountID: binding.ProviderAccountID, ModelID: binding.ModelID,
		ExecutionBindingDigest: binding.BindingDigest,
	}
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
		report.RecoveredRuns < 0 ||
		report.RevokedGrants != len(report.Outcomes) {
		return errProductInvalidAttemptRecoveryCompletion
	}
	if len(report.Outcomes) == 0 && len(report.RecoveryOutcomes) == 0 {
		return nil
	}
	sink, ok := diagnostics.(productOperationalDiagnosticSink)
	if !ok || nilProductAgentInterface(sink) {
		return errProductInvalidAttemptRecoveryCompletion
	}
	record := func(outcome productAttemptRecoveryTerminalOutcome, result string) error {
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
			Stage:                  "agent_attempt_reconcile", Result: result,
		}); err != nil {
			return err
		}
		return nil
	}
	for _, outcome := range report.RecoveryOutcomes {
		if err := record(outcome, "recovered"); err != nil {
			return err
		}
	}
	for _, outcome := range report.Outcomes {
		if err := record(outcome, "succeeded"); err != nil {
			return err
		}
	}
	return nil
}
