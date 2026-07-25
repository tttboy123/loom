package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidConfiguredRuntimeObservationRun = errors.New(
	"invalid configured runtime observation run",
)

func RunConfiguredRuntimeObservationOnce(
	ctx context.Context,
	factories []discoveryscan.ProbeFactory,
	projected projection.Snapshot,
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
	if ctx == nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidConfiguredRuntimeObservationRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	snapshot, err := discoveryscan.DiscoverConfiguredRuntimes(ctx, factories)
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

	plan, discovery, reconciliation, status, err :=
		RunRuntimeObservationWriteOnce(
			ctx, projected, snapshot, discoveryCommitter, statusCommitter,
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
