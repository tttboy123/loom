package main

import (
	"context"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

type productLocalIPCHandlerDecorator func(localipc.Handler) localipc.Handler

type productLocalIPCHandlerSlot struct {
	mu      sync.RWMutex
	handler localipc.Handler
	bound   bool
	closed  bool
}

func (slot *productLocalIPCHandlerSlot) Bind(handler localipc.Handler) error {
	if slot == nil || nilProductAssetPort(handler) {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed || slot.bound {
		return composition.ErrInvalidComposition
	}
	slot.handler = handler
	slot.bound = true
	return nil
}

func (slot *productLocalIPCHandlerSlot) Handle(
	ctx context.Context,
	request localipc.Request,
) localipc.Response {
	if slot == nil {
		return productLocalIPCUnavailableResponse(request)
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || nilProductAssetPort(slot.handler) {
		return productLocalIPCUnavailableResponse(request)
	}
	return slot.handler.Handle(ctx, request)
}

func (slot *productLocalIPCHandlerSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return !slot.closed && slot.bound && !nilProductAssetPort(slot.handler)
}

func (slot *productLocalIPCHandlerSlot) Close(context.Context) error {
	if slot == nil {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	slot.closed = true
	slot.bound = false
	slot.handler = nil
	return nil
}

func productLocalIPCUnavailableResponse(request localipc.Request) localipc.Response {
	response := productErrorResponse(
		"state_unavailable", composition.ErrCompositionNotReady,
	)
	response.Version = request.Version
	response.RequestID = request.RequestID
	response.JourneyID = request.JourneyID
	if response.Error != nil {
		response.Error.Stage = productDiagnosticStageDaemonAdmission
		response.Error.Recoverable = true
	}
	return response
}

func newProductLocalIPCHandlerFactory(
	services productRouteServices,
	decorators ...productLocalIPCHandlerDecorator,
) func(context.Context) (localipc.Handler, error) {
	return func(ctx context.Context) (localipc.Handler, error) {
		if ctx == nil {
			return nil, composition.ErrInvalidComposition
		}
		handler := localipc.Handler(localipc.HandlerFunc(newProductRouteHandler(services)))
		for _, decorate := range decorators {
			if decorate == nil {
				return nil, composition.ErrInvalidComposition
			}
			handler = decorate(handler)
			if nilProductAssetPort(handler) {
				return nil, composition.ErrCapabilityUnavailable
			}
		}
		return handler, nil
	}
}

func (construction productCompatibilityConstruction) startLocalIPC(
	ctx context.Context,
	capabilities *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.localIPCSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	var setupEffect composition.Effect
	if construction.setupSlot != nil {
		var err error
		setupEffect, err = construction.startSetup(ctx, capabilities)
		if err != nil {
			_ = construction.localIPCSlot.Close(ctx)
			return nil, err
		}
	}
	rollback := func(cause error) error {
		closeErr := construction.localIPCSlot.Close(ctx)
		if setupEffect != nil {
			closeErr = errors.Join(closeErr, setupEffect.Close(ctx))
		}
		return errors.Join(cause, closeErr)
	}
	handler, err := construction.localIPCFactory(ctx)
	if err != nil || nilProductAssetPort(handler) {
		return nil, rollback(errors.Join(composition.ErrCapabilityUnavailable, err))
	}
	if err := construction.localIPCSlot.Bind(handler); err != nil {
		return nil, rollback(err)
	}
	return composition.NewEffect(func(closeContext context.Context) error {
		closeErr := construction.localIPCSlot.Close(closeContext)
		if setupEffect != nil {
			closeErr = errors.Join(closeErr, setupEffect.Close(closeContext))
		}
		return closeErr
	}), nil
}

func (construction productCompatibilityConstruction) localIPCReady(
	ctx context.Context,
	capabilities *composition.BundleContext,
) error {
	if !construction.valid() || construction.localIPCSlot == nil ||
		!construction.localIPCSlot.Ready() {
		return composition.ErrCapabilityUnavailable
	}
	if construction.setupSlot != nil {
		return construction.setupReady(ctx, capabilities)
	}
	return nil
}

var _ localipc.Handler = (*productLocalIPCHandlerSlot)(nil)
