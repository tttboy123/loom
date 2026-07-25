package app

import (
	"context"
	"errors"
	"strings"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var (
	ErrInvalidRuntimeStatusRun = errors.New(
		"invalid runtime status run",
	)
	ErrRuntimeStatusCommitResultMismatch = errors.New(
		"runtime status commit result mismatch",
	)
)

type RuntimeStatusCommitter interface {
	CommitRuntimeStatus(
		context.Context,
		loomruntime.RuntimeStatusReconciliationCandidate,
	) (state.RuntimeStatusCommitCandidate, error)
}

func RunObservedRuntimeStatusReconciliationOnce(
	ctx context.Context,
	baseline []loomruntime.RuntimeStatusBaseline,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	committer RuntimeStatusCommitter,
) (
	loomruntime.RuntimeStatusReconciliationCandidate,
	state.RuntimeStatusCommitCandidate,
	error,
) {
	if ctx == nil || nilAppInterface(committer) {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidRuntimeStatusRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			err
	}

	reconciliation, err := loomruntime.ReconcileObservedRuntimeStatuses(
		ctx, baseline, snapshot,
	)
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
	if reconciliation.TransitionCount() == 0 {
		return reconciliation, state.RuntimeStatusCommitCandidate{}, nil
	}

	commit, err := committer.CommitRuntimeStatus(ctx, reconciliation)
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
	if !validRuntimeStatusCommitResult(reconciliation, commit) {
		return loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrRuntimeStatusCommitResultMismatch
	}
	return reconciliation, commit, nil
}

type runtimeStatusCommitResultFacts struct {
	committed                  bool
	sourceReconciliationDigest string
	baselineDigest             string
	sourceDiscoveryDigest      string
	eventCount                 int
	eventAccessorCount         int
	commitDigest               string
}

func validRuntimeStatusCommitResult(
	reconciliation loomruntime.RuntimeStatusReconciliationCandidate,
	commit state.RuntimeStatusCommitCandidate,
) bool {
	return validRuntimeStatusCommitResultFacts(
		reconciliation,
		runtimeStatusCommitResultFacts{
			committed:                  commit.Committed(),
			sourceReconciliationDigest: commit.SourceReconciliationDigest(),
			baselineDigest:             commit.BaselineDigest(),
			sourceDiscoveryDigest:      commit.SourceDiscoveryDigest(),
			eventCount:                 commit.EventCount(),
			eventAccessorCount:         len(commit.Events()),
			commitDigest:               commit.CommitDigest(),
		},
	)
}

func validRuntimeStatusCommitResultFacts(
	reconciliation loomruntime.RuntimeStatusReconciliationCandidate,
	facts runtimeStatusCommitResultFacts,
) bool {
	transitionCount := reconciliation.TransitionCount()
	return facts.committed &&
		facts.sourceReconciliationDigest ==
			reconciliation.CandidateDigest() &&
		facts.baselineDigest == reconciliation.BaselineDigest() &&
		facts.sourceDiscoveryDigest ==
			reconciliation.SourceDiscoveryDigest() &&
		facts.eventCount == transitionCount &&
		facts.eventAccessorCount == transitionCount &&
		validRuntimeStatusCommitDigest(facts.commitDigest)
}

func validRuntimeStatusCommitDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}
