package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidRuntimeObservationWriteRun = errors.New(
	"invalid runtime observation write run",
)

func RunRuntimeObservationWriteOnce(
	ctx context.Context,
	projected projection.Snapshot,
	current loomruntime.RuntimeDiscoverySnapshot,
	discoveryCommitter RuntimeDiscoveryCommitter,
	statusCommitter RuntimeStatusCommitter,
) (
	RuntimeObservationWritePlanCandidate,
	state.RuntimeDiscoveryCommitCandidate,
	loomruntime.RuntimeStatusReconciliationCandidate,
	state.RuntimeStatusCommitCandidate,
	error,
) {
	if ctx == nil {
		return RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidRuntimeObservationWriteRun
	}
	if err := ctx.Err(); err != nil {
		return RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	plan, err := PlanRuntimeObservationWrite(ctx, projected, current)
	if err != nil {
		return RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	switch plan.Kind() {
	case RuntimeObservationWriteNone:
		if err := ctx.Err(); err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		return plan,
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			nil

	case RuntimeObservationWriteDiscovery:
		if nilAppInterface(discoveryCommitter) {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				ErrInvalidRuntimeObservationWriteRun
		}
		if err := ctx.Err(); err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		commit, err := discoveryCommitter.CommitRuntimeDiscovery(ctx, current)
		if err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		if err := ctx.Err(); err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		if !validRuntimeDiscoveryCommitResult(current, commit) {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				ErrRuntimeDiscoveryCommitResultMismatch
		}
		return plan,
			commit,
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			nil

	case RuntimeObservationWriteStatus:
		if nilAppInterface(statusCommitter) {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				ErrInvalidRuntimeObservationWriteRun
		}
		if err := ctx.Err(); err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		reconciliation, commit, err :=
			RunProjectedRuntimeStatusReconciliationOnce(
				ctx, projected, current, statusCommitter,
			)
		if err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		if err := ctx.Err(); err != nil {
			return RuntimeObservationWritePlanCandidate{},
				state.RuntimeDiscoveryCommitCandidate{},
				loomruntime.RuntimeStatusReconciliationCandidate{},
				state.RuntimeStatusCommitCandidate{},
				err
		}
		return plan,
			state.RuntimeDiscoveryCommitCandidate{},
			reconciliation,
			commit,
			nil

	default:
		return RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidRuntimeObservationWriteRun
	}
}
