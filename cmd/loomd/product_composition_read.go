package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

var errProductReadRouteUnavailable = errors.New("product Read route unavailable")

type productReadRoute interface {
	ReadLocalProductSnapshot(context.Context, api.LocalProductSnapshotRequest) (api.LocalProductSnapshot, error)
	ReadLocalProductTimeline(context.Context, api.LocalProductTimelineRequest) (api.LocalProductTimelinePage, error)
	ReadChatThread(context.Context, string) (api.LocalProductChatThread, error)
	SendChatMessage(context.Context, api.LocalProductChatMessageRequest) (api.LocalProductChatThread, error)
	SetAgentAttemptDiagnosticSource(api.AgentAttemptDiagnosticSource) error
	SetGovernedTestReportSource(api.GovernedTestReportSource) error
	SetSideTaskSnapshotSource(api.SideTaskSnapshotSource) error
	MissionExecutionObserver(context.Context, string) (app.NodeOutputObserver, error)
	CloseMissionExecutionObservers() error
}

type productReadRoutes struct {
	route productReadRoute
	close func() error
}

func (routes productReadRoutes) valid() bool {
	return !nilProductAssetPort(routes.route) && routes.close != nil
}

type productReadRouteSlot struct {
	mu     sync.RWMutex
	routes productReadRoutes
	bound  bool
	closed bool
}

func (slot *productReadRouteSlot) Bind(routes productReadRoutes) error {
	if slot == nil || !routes.valid() {
		return api.ErrInvalidLocalProductRequest
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

func (slot *productReadRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productReadRouteSlot) Close() error {
	if slot == nil {
		return nil
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	closeRoute := slot.routes.close
	slot.routes = productReadRoutes{}
	if closeRoute == nil {
		return nil
	}
	return closeRoute()
}

func newProductReadRouteFactory(
	store *journal.Store,
	readModel *projection.Projection,
	decisions api.MissionDecisionCommandSource,
	runtimeHealth api.RuntimeObservationHealthSource,
	chat api.LocalProductChatSource,
	diagnostics api.AgentAttemptDiagnosticSource,
) func(context.Context) (productReadRoutes, error) {
	return func(ctx context.Context) (productReadRoutes, error) {
		if ctx == nil || ctx.Err() != nil || store == nil || readModel == nil ||
			nilProductAssetPort(decisions) || nilProductAssetPort(runtimeHealth) ||
			nilProductAssetPort(chat) || nilProductAssetPort(diagnostics) {
			return productReadRoutes{}, errProductReadRouteUnavailable
		}
		route, err := api.NewLocalProductReadService(api.LocalProductReadConfig{
			Journal: store, Projection: readModel,
			Now:           func() time.Time { return time.Now().UTC() },
			Decisions:     decisions,
			RuntimeHealth: runtimeHealth,
			Diagnostics:   diagnostics,
			Chat:          chat,
		})
		if err != nil {
			return productReadRoutes{}, errors.Join(errProductReadRouteUnavailable, err)
		}
		return productReadRoutes{
			route: route, close: route.CloseMissionExecutionObservers,
		}, nil
	}
}

func newProductReadRouteFactoryFromCore(
	core *productCoreRouteSlot,
	decisions api.MissionDecisionCommandSource,
	runtimeHealth api.RuntimeObservationHealthSource,
	chat api.LocalProductChatSource,
	diagnostics api.AgentAttemptDiagnosticSource,
) func(context.Context) (productReadRoutes, error) {
	return func(ctx context.Context) (productReadRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productReadRoutes{}, err
		}
		return newProductReadRouteFactory(
			resources.store, resources.readModel, decisions, runtimeHealth,
			chat, diagnostics,
		)(ctx)
	}
}

func (slot *productReadRouteSlot) ReadLocalProductSnapshot(
	ctx context.Context, request api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	if slot == nil {
		return api.LocalProductSnapshot{}, api.ErrLocalProductStateUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductSnapshot{}, api.ErrLocalProductStateUnavailable
	}
	return slot.routes.route.ReadLocalProductSnapshot(ctx, request)
}

func (slot *productReadRouteSlot) ReadLocalProductTimeline(
	ctx context.Context, request api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	if slot == nil {
		return api.LocalProductTimelinePage{}, api.ErrLocalProductStateUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductTimelinePage{}, api.ErrLocalProductStateUnavailable
	}
	return slot.routes.route.ReadLocalProductTimeline(ctx, request)
}

func (slot *productReadRouteSlot) ReadChatThread(
	ctx context.Context, threadID string,
) (api.LocalProductChatThread, error) {
	if slot == nil {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	return slot.routes.route.ReadChatThread(ctx, threadID)
}

func (slot *productReadRouteSlot) SendChatMessage(
	ctx context.Context, request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	if slot == nil {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	return slot.routes.route.SendChatMessage(ctx, request)
}

func (slot *productReadRouteSlot) SetAgentAttemptDiagnosticSource(
	source api.AgentAttemptDiagnosticSource,
) error {
	return slot.configure(func(route productReadRoute) error {
		return route.SetAgentAttemptDiagnosticSource(source)
	})
}

func (slot *productReadRouteSlot) SetGovernedTestReportSource(
	source api.GovernedTestReportSource,
) error {
	return slot.configure(func(route productReadRoute) error {
		return route.SetGovernedTestReportSource(source)
	})
}

func (slot *productReadRouteSlot) SetSideTaskSnapshotSource(
	source api.SideTaskSnapshotSource,
) error {
	return slot.configure(func(route productReadRoute) error {
		return route.SetSideTaskSnapshotSource(source)
	})
}

func (slot *productReadRouteSlot) configure(call func(productReadRoute) error) error {
	if slot == nil || call == nil {
		return api.ErrInvalidLocalProductRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.ErrInvalidLocalProductRequest
	}
	return call(slot.routes.route)
}

func (slot *productReadRouteSlot) MissionExecutionObserver(
	ctx context.Context, teamInstanceID string,
) (app.NodeOutputObserver, error) {
	if slot == nil {
		return nil, api.ErrInvalidLocalProductRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return nil, api.ErrInvalidLocalProductRequest
	}
	return slot.routes.route.MissionExecutionObserver(ctx, teamInstanceID)
}

func (slot *productReadRouteSlot) CloseMissionExecutionObservers() error {
	if slot == nil {
		return nil
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return nil
	}
	return slot.routes.route.CloseMissionExecutionObservers()
}

var _ productReadRoute = (*api.LocalProductReadService)(nil)
var _ productReadRoute = (*productReadRouteSlot)(nil)
var _ app.MissionExecutionObserverFactory = (*productReadRouteSlot)(nil)
