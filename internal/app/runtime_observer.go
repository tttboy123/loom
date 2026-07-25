package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/state"
)

var ErrInvalidPreparedProjectedRuntimeObserver = errors.New(
	"invalid prepared projected runtime observer",
)

type PreparedProjectedRuntimeObserver struct {
	factories          []discoveryscan.ProbeFactory
	readModel          *projection.Projection
	discoveryCommitter RuntimeDiscoveryCommitter
	statusCommitter    RuntimeStatusCommitter
}

func NewPreparedProjectedRuntimeObserver(
	factories []discoveryscan.ProbeFactory,
	readModel *projection.Projection,
	discoveryCommitter RuntimeDiscoveryCommitter,
	statusCommitter RuntimeStatusCommitter,
) (*PreparedProjectedRuntimeObserver, error) {
	if readModel == nil {
		return nil, ErrInvalidPreparedProjectedRuntimeObserver
	}

	return &PreparedProjectedRuntimeObserver{
		factories: append(
			[]discoveryscan.ProbeFactory(nil),
			factories...,
		),
		readModel:          readModel,
		discoveryCommitter: discoveryCommitter,
		statusCommitter:    statusCommitter,
	}, nil
}

func (observer *PreparedProjectedRuntimeObserver) RunOnce(
	ctx context.Context,
) (
	loomruntime.RuntimeDiscoverySnapshot,
	RuntimeObservationWritePlanCandidate,
	state.RuntimeDiscoveryCommitCandidate,
	loomruntime.RuntimeStatusReconciliationCandidate,
	state.RuntimeStatusCommitCandidate,
	error,
) {
	if observer == nil {
		return loomruntime.RuntimeDiscoverySnapshot{},
			RuntimeObservationWritePlanCandidate{},
			state.RuntimeDiscoveryCommitCandidate{},
			loomruntime.RuntimeStatusReconciliationCandidate{},
			state.RuntimeStatusCommitCandidate{},
			ErrInvalidPreparedProjectedRuntimeObserver
	}

	return RunProjectedConfiguredRuntimeObservationOnce(
		ctx,
		observer.factories,
		observer.readModel,
		observer.discoveryCommitter,
		observer.statusCommitter,
	)
}
