package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidProjectedRuntimeStatusRun = errors.New(
	"invalid projected runtime status run",
)

func RunProjectedRuntimeStatusReconciliationOnce(
	ctx context.Context,
	projected projection.Snapshot,
	current loomruntime.RuntimeDiscoverySnapshot,
	committer RuntimeStatusCommitter,
) (
	loomruntime.RuntimeStatusReconciliationCandidate,
	state.RuntimeStatusCommitCandidate,
	error,
) {
	if ctx == nil || nilAppInterface(committer) {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidProjectedRuntimeStatusRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	baseline, err := projection.BuildRuntimeStatusBaselines(projected)
	if err != nil {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	return RunObservedRuntimeStatusReconciliationOnce(
		ctx, baseline, current, committer,
	)
}
