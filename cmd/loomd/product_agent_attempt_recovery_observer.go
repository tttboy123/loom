package main

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
)

type productRecoveryTeamFrameObserverFactory struct {
	projection *projection.Projection
	read       productReadRoute
	evidence   *evidence.Store
}

func newProductRecoveryTeamFrameObserverFactory(
	readModel *projection.Projection,
	read productReadRoute,
	evidenceStore *evidence.Store,
) (*productRecoveryTeamFrameObserverFactory, error) {
	if readModel == nil || nilProductAssetPort(read) || evidenceStore == nil {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	return &productRecoveryTeamFrameObserverFactory{
		projection: readModel, read: read, evidence: evidenceStore,
	}, nil
}

func (factory *productRecoveryTeamFrameObserverFactory) RecoveryFrameObserver(
	ctx context.Context,
	outcome work.AgentAttemptRestartOutcome,
) (supervisor.AuthorizedFrameObserver, error) {
	if factory == nil || factory.projection == nil ||
		nilProductAssetPort(factory.read) || factory.evidence == nil ||
		ctx == nil || ctx.Err() != nil {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	validated, err := work.ValidateAgentAttemptRestartOutcome(outcome)
	if err != nil {
		return nil, errors.Join(errProductInvalidAttemptRecoveryCompletion, err)
	}
	if err := factory.projection.Rebuild(ctx); err != nil {
		return nil, err
	}
	teamID := validated.Binding.TeamInstanceID
	team, found := factory.projection.GlobalReadView().TeamExecution(teamID)
	if !found || team.TeamInstanceID != teamID {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	authority := validated.Binding.PayloadAuthority
	logicalNodeID := ""
	attemptNumber := 0
	evidenceID := ""
	for _, node := range team.Nodes {
		for _, attempt := range node.Attempts {
			if attempt.WorkItemID != authority.WorkItemID ||
				attempt.RunID != authority.RunID || attempt.ClaimID != authority.ClaimID ||
				attempt.ClaimGeneration != authority.ClaimGeneration ||
				attempt.RuntimeInstanceID != authority.RuntimeInstanceID ||
				attempt.AgentInstanceID != authority.AgentInstanceID {
				continue
			}
			if logicalNodeID != "" || node.LogicalNodeID == "" ||
				attempt.AttemptNumber <= 0 || attempt.EvidenceID == "" {
				return nil, errProductInvalidAttemptRecoveryCompletion
			}
			logicalNodeID = node.LogicalNodeID
			attemptNumber = attempt.AttemptNumber
			evidenceID = attempt.EvidenceID
		}
	}
	if logicalNodeID == "" {
		return nil, errProductInvalidAttemptRecoveryCompletion
	}
	observer, err := factory.read.MissionExecutionObserver(ctx, teamID)
	if err != nil || nilProductAgentInterface(observer) {
		return nil, errors.Join(errProductInvalidAttemptRecoveryCompletion, err)
	}
	return app.NewRecoveredTeamFrameObserver(app.RecoveredTeamFrameObserverInput{
		LogicalNodeID: logicalNodeID, AttemptNumber: attemptNumber,
		Observer: observer, EvidenceStore: factory.evidence, EvidenceID: evidenceID,
	})
}
