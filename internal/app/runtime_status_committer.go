package app

import (
	"context"
	"errors"
	"fmt"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var (
	ErrInvalidPreparedRuntimeStatusCommitter = errors.New(
		"invalid prepared runtime status committer",
	)
	ErrRuntimeStatusCommitInputFailed = errors.New(
		"runtime status commit input failed",
	)
)

type RuntimeStatusCommitInputProvider interface {
	PrepareRuntimeStatusCommit(
		context.Context,
		loomruntime.RuntimeStatusReconciliationCandidate,
	) (state.RuntimeStatusCommitInput, error)
}

type PreparedRuntimeStatusCommitter struct {
	appender state.EventBatchAppender
	provider RuntimeStatusCommitInputProvider
}

var _ RuntimeStatusCommitter = (*PreparedRuntimeStatusCommitter)(nil)

func NewPreparedRuntimeStatusCommitter(
	appender state.EventBatchAppender,
	provider RuntimeStatusCommitInputProvider,
) (*PreparedRuntimeStatusCommitter, error) {
	if nilAppInterface(appender) || nilAppInterface(provider) {
		return nil, ErrInvalidPreparedRuntimeStatusCommitter
	}
	return &PreparedRuntimeStatusCommitter{
		appender: appender,
		provider: provider,
	}, nil
}

func (c *PreparedRuntimeStatusCommitter) CommitRuntimeStatus(
	ctx context.Context,
	candidate loomruntime.RuntimeStatusReconciliationCandidate,
) (state.RuntimeStatusCommitCandidate, error) {
	if c == nil ||
		nilAppInterface(c.appender) ||
		nilAppInterface(c.provider) ||
		ctx == nil {
		return state.RuntimeStatusCommitCandidate{},
			ErrInvalidPreparedRuntimeStatusCommitter
	}
	if err := ctx.Err(); err != nil {
		return state.RuntimeStatusCommitCandidate{}, err
	}

	input, err := c.provider.PrepareRuntimeStatusCommit(ctx, candidate)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return state.RuntimeStatusCommitCandidate{}, context.Canceled
		case errors.Is(err, context.DeadlineExceeded):
			return state.RuntimeStatusCommitCandidate{}, context.DeadlineExceeded
		default:
			return state.RuntimeStatusCommitCandidate{},
				fmt.Errorf("%w: %w", ErrRuntimeStatusCommitInputFailed, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return state.RuntimeStatusCommitCandidate{}, err
	}

	return state.CommitRuntimeStatusTransitions(
		ctx, c.appender, candidate, input,
	)
}
