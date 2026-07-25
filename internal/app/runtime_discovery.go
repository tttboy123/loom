package app

import (
	"context"
	"errors"
	"reflect"
	"strings"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

var (
	ErrInvalidRuntimeDiscoveryRun           = errors.New("invalid runtime discovery run")
	ErrRuntimeDiscoveryCommitResultMismatch = errors.New(
		"runtime discovery commit result mismatch",
	)
)

type RuntimeDiscoveryCommitter interface {
	CommitRuntimeDiscovery(
		context.Context,
		loomruntime.RuntimeDiscoverySnapshot,
	) (state.RuntimeDiscoveryCommitCandidate, error)
}

func RunConfiguredRuntimeDiscoveryOnce(
	ctx context.Context,
	factories []discoveryscan.ProbeFactory,
	committer RuntimeDiscoveryCommitter,
) (
	loomruntime.RuntimeDiscoverySnapshot,
	state.RuntimeDiscoveryCommitCandidate,
	error,
) {
	if ctx == nil || nilAppInterface(committer) {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			ErrInvalidRuntimeDiscoveryRun
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			err
	}

	snapshot, err := discoveryscan.DiscoverConfiguredRuntimes(ctx, factories)
	if err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			err
	}
	if len(snapshot.Observations()) == 0 {
		return snapshot, state.RuntimeDiscoveryCommitCandidate{}, nil
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			err
	}

	candidate, err := committer.CommitRuntimeDiscovery(ctx, snapshot)
	if err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			err
	}
	if err := ctx.Err(); err != nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			err
	}
	if !validRuntimeDiscoveryCommitResult(snapshot, candidate) {
		return loomruntime.RuntimeDiscoverySnapshot{},
			state.RuntimeDiscoveryCommitCandidate{},
			ErrRuntimeDiscoveryCommitResultMismatch
	}

	return snapshot, candidate, nil
}

func validRuntimeDiscoveryCommitResult(
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	candidate state.RuntimeDiscoveryCommitCandidate,
) bool {
	observationCount := len(snapshot.Observations())
	return candidate.Committed() &&
		candidate.SourceDiscoveryDigest() == snapshot.Digest() &&
		candidate.EventCount() == observationCount &&
		len(candidate.Events()) == observationCount &&
		validAppDiscoveryDigest(candidate.CommitDigest())
}

func validAppDiscoveryDigest(value string) bool {
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

func nilAppInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
