package main

import (
	"context"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
)

var errProductObservabilityUnavailable = errors.New("product observability unavailable")

type productIPCOperationalDiagnosticRecorder interface {
	wrap(localipc.Handler) localipc.Handler
}

type productObservabilityRoutes struct {
	contextRetrieval contextcapsule.RetrievalAuditor
	agentAttempts    nativeadapter.AgentAttemptDiagnosticRecorder
	attemptSource    api.AgentAttemptDiagnosticSource
	toolAttempts     productAttemptToolDiagnosticRecorder
	ipc              productIPCOperationalDiagnosticRecorder
}

func (routes productObservabilityRoutes) valid() bool {
	return !nilProductAssetPort(routes.contextRetrieval) &&
		!nilProductAssetPort(routes.agentAttempts) &&
		!nilProductAssetPort(routes.attemptSource) &&
		!nilProductAssetPort(routes.toolAttempts) &&
		!nilProductAssetPort(routes.ipc)
}

type productObservabilityRouteSlot struct {
	mu     sync.RWMutex
	routes productObservabilityRoutes
	bound  bool
	closed bool
}

func (slot *productObservabilityRouteSlot) Bind(
	routes productObservabilityRoutes,
) error {
	if slot == nil || !routes.valid() {
		return errProductObservabilityUnavailable
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.bound || slot.closed {
		return composition.ErrCompositionConflict
	}
	slot.routes = routes
	slot.bound = true
	return nil
}

func (slot *productObservabilityRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productObservabilityRouteSlot) Close() error {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	slot.routes = productObservabilityRoutes{}
	return nil
}

func (slot *productObservabilityRouteSlot) RecordContextRetrieval(
	ctx context.Context,
	audit contextcapsule.RetrievalAudit,
) error {
	if slot == nil {
		return errProductObservabilityUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return errProductObservabilityUnavailable
	}
	return slot.routes.contextRetrieval.RecordContextRetrieval(ctx, audit)
}

func (slot *productObservabilityRouteSlot) RecordAgentAttemptDiagnostic(
	ctx context.Context,
	diagnostic nativeadapter.AgentAttemptDiagnostic,
) error {
	if slot == nil {
		return errProductObservabilityUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return errProductObservabilityUnavailable
	}
	return slot.routes.agentAttempts.RecordAgentAttemptDiagnostic(ctx, diagnostic)
}

func (slot *productObservabilityRouteSlot) RecordAttemptToolDiagnostic(
	ctx context.Context,
	diagnostic productAttemptToolDiagnostic,
) error {
	if slot == nil {
		return errProductObservabilityUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return errProductObservabilityUnavailable
	}
	return slot.routes.toolAttempts.RecordAttemptToolDiagnostic(ctx, diagnostic)
}

func (slot *productObservabilityRouteSlot) AgentAttemptDiagnostics(
	ctx context.Context,
	queries []api.AgentAttemptDiagnosticQuery,
) ([]api.AgentAttemptDiagnosticSummary, error) {
	if slot == nil {
		return nil, errProductObservabilityUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return nil, errProductObservabilityUnavailable
	}
	return slot.routes.attemptSource.AgentAttemptDiagnostics(ctx, queries)
}

func (slot *productObservabilityRouteSlot) wrap(
	next localipc.Handler,
) localipc.Handler {
	return localipc.HandlerFunc(func(
		ctx context.Context,
		request localipc.Request,
	) localipc.Response {
		if slot == nil {
			return next.Handle(ctx, request)
		}
		slot.mu.RLock()
		if slot.closed || !slot.bound || !slot.routes.valid() {
			slot.mu.RUnlock()
			return next.Handle(ctx, request)
		}
		recorder := slot.routes.ipc
		slot.mu.RUnlock()
		return recorder.wrap(next).Handle(ctx, request)
	})
}

func newProductObservabilityRouteFactory(
	store *productOperationalDiagnosticStore,
) func(context.Context) (productObservabilityRoutes, error) {
	return func(ctx context.Context) (productObservabilityRoutes, error) {
		if ctx == nil || ctx.Err() != nil || store == nil {
			return productObservabilityRoutes{}, errProductObservabilityUnavailable
		}
		return productObservabilityRoutes{
			contextRetrieval: store,
			agentAttempts:    store,
			attemptSource:    store,
			toolAttempts:     store,
			ipc:              store,
		}, nil
	}
}

func newProductObservabilityConstructionFactory(
	bootstrap *productOperationalDiagnosticBootstrap,
) func(context.Context) (productObservabilityRoutes, error) {
	return func(ctx context.Context) (productObservabilityRoutes, error) {
		if ctx == nil || ctx.Err() != nil || bootstrap == nil {
			return productObservabilityRoutes{}, errProductObservabilityUnavailable
		}
		store, err := newProductOperationalDiagnosticStore(
			bootstrap.statePath, bootstrap.maximum, bootstrap.now,
		)
		if err != nil {
			return productObservabilityRoutes{}, errors.Join(
				errProductObservabilityUnavailable, err,
			)
		}
		if err := bootstrap.bind(store); err != nil {
			return productObservabilityRoutes{}, errors.Join(
				errProductObservabilityUnavailable, err,
			)
		}
		return newProductObservabilityRouteFactory(store)(ctx)
	}
}

func (construction productCompatibilityConstruction) startObservability(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.observabilitySlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.observabilityFactory(ctx)
	if err != nil || !routes.valid() {
		return nil, errors.Join(errProductObservabilityUnavailable, err)
	}
	if err := construction.observabilitySlot.Bind(routes); err != nil {
		return nil, err
	}
	return composition.NewEffect(func(context.Context) error {
		return construction.observabilitySlot.Close()
	}), nil
}

func (construction productCompatibilityConstruction) observabilityReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.observabilitySlot == nil ||
		!construction.observabilitySlot.Ready() {
		return errProductObservabilityUnavailable
	}
	return nil
}

var _ contextcapsule.RetrievalAuditor = (*productObservabilityRouteSlot)(nil)
var _ nativeadapter.AgentAttemptDiagnosticRecorder = (*productObservabilityRouteSlot)(nil)
var _ api.AgentAttemptDiagnosticSource = (*productObservabilityRouteSlot)(nil)
var _ productAttemptToolDiagnosticRecorder = (*productObservabilityRouteSlot)(nil)
