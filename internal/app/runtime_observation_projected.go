package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidProjectedConfiguredRuntimeObservationRun = errors.New(
	"invalid projected configured runtime observation run",
)

func RunProjectedConfiguredRuntimeObservationOnce(
	ctx context.Context,
	factories []discoveryscan.ProbeFactory,
	readModel *projection.Projection,
	discoveryCommitter RuntimeDiscoveryCommitter,
	statusCommitter RuntimeStatusCommitter,
) (
	loomruntime.RuntimeDiscoverySnapshot,
	RuntimeObservationWritePlanCandidate,
	state.RuntimeDiscoveryCommitCandidate,
	loomruntime.RuntimeStatusReconciliationCandidate,
	state.RuntimeStatusCommitCandidate,
	error,
) {
	if ctx == nil || readModel == nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidProjectedConfiguredRuntimeObservationRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	projected := readModel.Snapshot()
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	snapshot, plan, discovery, reconciliation, status, err :=
		RunConfiguredRuntimeObservationOnce(
			ctx,
			factories,
			projected,
			discoveryCommitter,
			statusCommitter,
		)
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
