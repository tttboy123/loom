package app

import (
	"context"
	"errors"
	"fmt"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
)

var (
	ErrInvalidPreparedRuntimeDiscoveryCommitter = errors.New(
		"invalid prepared runtime discovery committer",
	)
	ErrRuntimeDiscoveryCommitInputFailed = errors.New(
		"runtime discovery commit input failed",
	)
)

type RuntimeDiscoveryCommitInputProvider interface {
	PrepareRuntimeDiscoveryCommit(
		context.Context,
		loomruntime.RuntimeDiscoverySnapshot,
	) (state.RuntimeDiscoveryCommitInput, error)
}

type PreparedRuntimeDiscoveryCommitter struct {
	appender state.EventBatchAppender
	provider RuntimeDiscoveryCommitInputProvider
}

var _ RuntimeDiscoveryCommitter = (*PreparedRuntimeDiscoveryCommitter)(nil)

func NewPreparedRuntimeDiscoveryCommitter(
	appender state.EventBatchAppender,
	provider RuntimeDiscoveryCommitInputProvider,
) (*PreparedRuntimeDiscoveryCommitter, error) {
	if nilAppInterface(appender) || nilAppInterface(provider) {
		return nil, ErrInvalidPreparedRuntimeDiscoveryCommitter
	}
	return &PreparedRuntimeDiscoveryCommitter{
		appender: appender,
		provider: provider,
	}, nil
}

func (c *PreparedRuntimeDiscoveryCommitter) CommitRuntimeDiscovery(
	ctx context.Context,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) (state.RuntimeDiscoveryCommitCandidate, error) {
	if c == nil ||
		nilAppInterface(c.appender) ||
		nilAppInterface(c.provider) ||
		ctx == nil {
		return state.RuntimeDiscoveryCommitCandidate{},
			ErrInvalidPreparedRuntimeDiscoveryCommitter
	}
	if err := ctx.Err(); err != nil {
		return state.RuntimeDiscoveryCommitCandidate{}, err
	}

	input, err := c.provider.PrepareRuntimeDiscoveryCommit(ctx, snapshot)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return state.RuntimeDiscoveryCommitCandidate{}, context.Canceled
		case errors.Is(err, context.DeadlineExceeded):
			return state.RuntimeDiscoveryCommitCandidate{}, context.DeadlineExceeded
		default:
			return state.RuntimeDiscoveryCommitCandidate{},
				fmt.Errorf("%w: %w", ErrRuntimeDiscoveryCommitInputFailed, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return state.RuntimeDiscoveryCommitCandidate{}, err
	}

	return state.CommitRuntimeDiscoverySnapshot(ctx, c.appender, snapshot, input)
}
