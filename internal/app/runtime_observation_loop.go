package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidProjectionSynchronizedRuntimeObservationRun = errors.New(
	"invalid projection-synchronized runtime observation run",
)

type projectionSynchronizedRuntimeObservationTrigger struct {
	trigger   RuntimeObservationTrigger
	readModel *projection.Projection
}

func (synchronized projectionSynchronizedRuntimeObservationTrigger) AwaitRuntimeObservation(
	ctx context.Context,
) error {
	trigger := synchronized.trigger
	if err := trigger.AwaitRuntimeObservation(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	readModel := synchronized.readModel
	return readModel.Rebuild(ctx)
}

func RunProjectionSynchronizedRuntimeObservationOnce(
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
	if ctx == nil ||
		nilAppInterface(trigger) ||
		observer == nil ||
		observer.readModel == nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidProjectionSynchronizedRuntimeObservationRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	readModel := observer.readModel
	synchronizedTrigger := projectionSynchronizedRuntimeObservationTrigger{
		trigger:   trigger,
		readModel: readModel,
	}
	snapshot, plan, discovery, reconciliation, status, err :=
		RunTriggeredPreparedRuntimeObservationOnce(
			ctx,
			synchronizedTrigger,
			observer,
		)
	if err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return snapshot, plan, discovery, reconciliation, status, err
	}
	return snapshot, plan, discovery, reconciliation, status, nil
}
