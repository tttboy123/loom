package app

import (
	"context"
	"errors"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidTriggeredPreparedRuntimeObservationRun = errors.New(
	"invalid triggered prepared runtime observation run",
)

type RuntimeObservationTrigger interface {
	AwaitRuntimeObservation(context.Context) error
}

func RunTriggeredPreparedRuntimeObservationOnce(
	ctx context.Context,
	trigger RuntimeObservationTrigger,
	observer *PreparedProjectedRuntimeObserver,
) (
	loomruntime.RuntimeDiscoverySnapshot,
	RuntimeObservationWritePlanCandidate,
	state.RuntimeDiscoveryCommitCandidate,
	loomruntime.RuntimeStatusReconciliationCandidate,
	state.RuntimeStatusCommitCandidate,
	error,
) {
	if ctx == nil || nilAppInterface(trigger) || observer == nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidTriggeredPreparedRuntimeObservationRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	if err := trigger.AwaitRuntimeObservation(ctx); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	snapshot, plan, discovery, reconciliation, status, err :=
		observer.RunOnce(ctx)
	if err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	return snapshot, plan, discovery, reconciliation, status, nil
}
