package main

import (
	"context"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

const productCompatibilityDispatchRoute composition.RouteMethod = "legacy.product.dispatch"

type productCompatibilityPort interface {
	CompatibilityBundleID() string
}

type productCompatibilityMarker struct{ bundleID string }

func (marker *productCompatibilityMarker) CompatibilityBundleID() string {
	if marker == nil {
		return ""
	}
	return marker.bundleID
}

var productCompatibilityKeys = map[composition.CapabilityID]composition.CapabilityKey[productCompatibilityPort]{
	composition.CapabilityCoreReadiness:          composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityCoreReadiness),
	composition.CapabilityObservabilityRecorder:  composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityObservabilityRecorder),
	composition.CapabilityAssetsReader:           composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityAssetsReader),
	composition.CapabilityCredentialLeaseIssuer:  composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityCredentialLeaseIssuer),
	composition.CapabilityConversationRouter:     composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityConversationRouter),
	composition.CapabilityRuntimeDispatcher:      composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityRuntimeDispatcher),
	composition.CapabilityScopedContextRetriever: composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityScopedContextRetriever),
	composition.CapabilityApprovedToolInvoker:    composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityApprovedToolInvoker),
	composition.CapabilityWorkCoordinator:        composition.MustCapabilityKey[productCompatibilityPort](composition.CapabilityWorkCoordinator),
}

var productCompatibilityHandlerKey = composition.MustCapabilityKey[localipc.Handler](composition.CapabilityLocalIPCHandler)

type productCompatibilityBundle struct {
	descriptor composition.BundleDescriptor
	handler    localipc.Handler
	start      func(context.Context, *composition.BundleContext) (composition.Effect, error)
	ready      func(context.Context, *composition.BundleContext) error
}

func (bundle *productCompatibilityBundle) Descriptor() composition.BundleDescriptor {
	return cloneProductCompatibilityDescriptor(bundle.descriptor)
}

func (bundle *productCompatibilityBundle) Register(
	_ context.Context,
	registration *composition.Registration,
) (composition.Effect, error) {
	for _, ref := range bundle.descriptor.Requires {
		key, ok := productCompatibilityKeys[ref.ID]
		if !ok {
			return nil, composition.ErrCapabilityUnavailable
		}
		if _, err := composition.Require(registration, key); err != nil {
			return nil, err
		}
	}
	for _, ref := range bundle.descriptor.Provides {
		if ref.ID == composition.CapabilityLocalIPCHandler {
			if bundle.handler == nil {
				return nil, composition.ErrCapabilityUnavailable
			}
			if err := composition.Provide(
				registration, productCompatibilityHandlerKey, bundle.handler,
			); err != nil {
				return nil, err
			}
			continue
		}
		key, ok := productCompatibilityKeys[ref.ID]
		if !ok {
			return nil, composition.ErrCapabilityUnavailable
		}
		marker := productCompatibilityPort(&productCompatibilityMarker{bundleID: bundle.descriptor.ID})
		if err := composition.Provide(registration, key, marker); err != nil {
			return nil, err
		}
	}
	return composition.NewEffect(func(context.Context) error { return nil }), nil
}

func (bundle *productCompatibilityBundle) Start(
	ctx context.Context,
	capabilities *composition.BundleContext,
) (composition.Effect, error) {
	if bundle.start != nil {
		return bundle.start(ctx, capabilities)
	}
	return composition.NewEffect(func(context.Context) error { return nil }), nil
}

func (bundle *productCompatibilityBundle) Ready(
	ctx context.Context,
	capabilities *composition.BundleContext,
) error {
	if bundle.ready != nil {
		return bundle.ready(ctx, capabilities)
	}
	return nil
}

func (*productCompatibilityBundle) Stop(context.Context, *composition.BundleContext) error {
	return nil
}

type productCompatibilityComposition struct {
	activation   *composition.Activation
	productScope *composition.CapabilityContext
	snapshot     composition.Snapshot
	handler      localipc.Handler
	recorder     composition.DiagnosticRecorder

	mu       sync.RWMutex
	closed   bool
	closeErr error
}

func activateProductCompatibilityComposition(
	ctx context.Context,
	profileID composition.ProfileID,
	handler localipc.Handler,
	incidentID string,
	recorder composition.DiagnosticRecorder,
	constructions ...productCompatibilityConstruction,
) (*productCompatibilityComposition, error) {
	if ctx == nil || len(constructions) > 1 {
		return nil, composition.ErrInvalidComposition
	}
	construction := productCompatibilityConstruction{}
	if len(constructions) == 1 {
		construction = constructions[0]
		if !construction.valid() {
			return nil, composition.ErrInvalidComposition
		}
	}
	directHandler := !nilProductAssetPort(handler)
	constructedHandler := construction.localIPCSlot != nil
	if directHandler == constructedHandler {
		return nil, composition.ErrInvalidComposition
	}
	profile, err := composition.BuiltInLaunchProfile(profileID)
	if err != nil {
		return nil, err
	}
	routes := productRouteManifest()
	profile.RequiredRoutes = productRouteMethods(routes)
	bundles := productCompatibilityBundles(handler, construction)
	plan, err := composition.Compile(composition.CompileInput{
		Profile: profile, Bundles: bundles, Context: ctx,
		IncidentID: incidentID, Recorder: recorder,
	})
	if err != nil {
		return nil, err
	}
	if err := productCompositionDiagnosticFailure(recorder); err != nil {
		return nil, err
	}
	activation, err := plan.Activate(ctx, incidentID, recorder)
	if err != nil {
		return nil, err
	}
	if err := productCompositionDiagnosticFailure(recorder); err != nil {
		_ = activation.Close(ctx)
		return nil, err
	}
	productScope, err := activation.Root().OpenChild(
		ctx, composition.ScopeProduct,
		composition.ScopeOptions{ID: "loom-product"},
	)
	if err != nil {
		_ = activation.Close(ctx)
		return nil, err
	}
	if err := productCompositionDiagnosticFailure(recorder); err != nil {
		_ = productScope.Close(ctx)
		_ = activation.Close(ctx)
		return nil, err
	}
	if construction.scopeSlot != nil {
		scopeManager, scopeErr := newProductCapabilityScopeManager(
			productScope, plan.Snapshot().Digest,
		)
		if scopeErr == nil {
			scopeErr = construction.scopeSlot.Bind(scopeManager)
		}
		if scopeErr == nil {
			scopeErr = productScope.Own(composition.NewEffect(
				func(context.Context) error { return construction.scopeSlot.Close() },
			))
		}
		if scopeErr != nil {
			_ = construction.scopeSlot.Close()
			_ = productScope.Close(ctx)
			_ = activation.Close(ctx)
			return nil, scopeErr
		}
	}
	admittedHandler, err := composition.Lookup(activation.Root(), productCompatibilityHandlerKey)
	if err != nil || admittedHandler == nil {
		_ = productScope.Close(ctx)
		_ = activation.Close(ctx)
		return nil, errors.Join(composition.ErrCapabilityUnavailable, err)
	}
	facade := &productCompatibilityComposition{
		activation: activation, productScope: productScope,
		snapshot: plan.Snapshot(), handler: admittedHandler,
		recorder: recorder,
	}
	facade.handler = localipc.HandlerFunc(func(
		requestContext context.Context,
		request localipc.Request,
	) localipc.Response {
		facade.mu.RLock()
		defer facade.mu.RUnlock()
		if facade.closed || facade.activation == nil || !facade.activation.Ready() {
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
		return admittedHandler.Handle(requestContext, request)
	})
	return facade, nil
}

func (facade *productCompatibilityComposition) Snapshot() composition.Snapshot {
	if facade == nil {
		return composition.Snapshot{}
	}
	return facade.snapshot.Clone()
}

func (facade *productCompatibilityComposition) Handler() localipc.Handler {
	if facade == nil {
		return nil
	}
	facade.mu.RLock()
	defer facade.mu.RUnlock()
	if facade.closed || facade.activation == nil || !facade.activation.Ready() {
		return nil
	}
	return facade.handler
}

func (facade *productCompatibilityComposition) Close() error {
	if facade == nil {
		return composition.ErrInvalidComposition
	}
	facade.mu.Lock()
	defer facade.mu.Unlock()
	if facade.closed {
		return facade.closeErr
	}
	facade.closed = true
	if facade.activation == nil || facade.productScope == nil {
		facade.closeErr = composition.ErrInvalidComposition
		return facade.closeErr
	}
	facade.closeErr = errors.Join(
		facade.productScope.Close(context.Background()),
		facade.activation.Close(context.Background()),
		productCompositionDiagnosticFailure(facade.recorder),
	)
	facade.productScope = nil
	facade.handler = nil
	return facade.closeErr
}

func productCompositionDiagnosticFailure(recorder composition.DiagnosticRecorder) error {
	source, ok := recorder.(interface{ CompositionDiagnosticError() error })
	if !ok {
		return nil
	}
	return source.CompositionDiagnosticError()
}

func productCompatibilityBundles(
	handler localipc.Handler,
	constructions ...productCompatibilityConstruction,
) []composition.Bundle {
	core := composition.CapabilityCoreReadiness.Ref()
	observability := composition.CapabilityObservabilityRecorder.Ref()
	assets := composition.CapabilityAssetsReader.Ref()
	vault := composition.CapabilityCredentialLeaseIssuer.Ref()
	conversation := composition.CapabilityConversationRouter.Ref()
	runtime := composition.CapabilityRuntimeDispatcher.Ref()
	contextRetriever := composition.CapabilityScopedContextRetriever.Ref()
	toolInvoker := composition.CapabilityApprovedToolInvoker.Ref()
	work := composition.CapabilityWorkCoordinator.Ref()
	ipc := composition.CapabilityLocalIPCHandler.Ref()
	descriptors := []composition.BundleDescriptor{
		compatibilityDescriptor("loom-core", true, nil, []composition.CapabilityRef{core}),
		compatibilityDescriptor("loom-assets", false, []composition.CapabilityRef{core}, []composition.CapabilityRef{assets}),
		compatibilityDescriptor("loom-observability", false, []composition.CapabilityRef{core}, []composition.CapabilityRef{observability}),
		compatibilityDescriptor("loom-governance", false, []composition.CapabilityRef{core, observability, vault}, []composition.CapabilityRef{contextRetriever, toolInvoker}),
		compatibilityDescriptor("loom-vault", false, []composition.CapabilityRef{core, observability}, []composition.CapabilityRef{vault}),
		compatibilityDescriptor("loom-conversation", false, []composition.CapabilityRef{core, observability, vault}, []composition.CapabilityRef{conversation}),
		compatibilityDescriptor("loom-work", false, []composition.CapabilityRef{core, assets, contextRetriever}, []composition.CapabilityRef{work}),
		compatibilityDescriptor("loom-agent-runtime", false, []composition.CapabilityRef{core, vault, conversation, toolInvoker, work}, []composition.CapabilityRef{runtime}),
		compatibilityDescriptor("loom-local-ipc", false, []composition.CapabilityRef{core, observability, assets, vault, conversation, contextRetriever, toolInvoker, work, runtime}, []composition.CapabilityRef{ipc}),
	}
	for _, route := range productRouteManifest() {
		for index := range descriptors {
			if descriptors[index].ID == route.OwnerBundle {
				descriptors[index].Routes = append(descriptors[index].Routes, route)
				break
			}
		}
	}
	result := make([]composition.Bundle, len(descriptors))
	for index := range descriptors {
		result[index] = &productCompatibilityBundle{descriptor: descriptors[index]}
	}
	if len(constructions) == 1 && constructions[0].coreSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-core" {
				bundle.start = constructions[0].startCore
				bundle.ready = constructions[0].coreReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].assetSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-assets" {
				bundle.start = constructions[0].startAssets
				bundle.ready = constructions[0].assetsReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].observabilitySlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-observability" {
				bundle.start = constructions[0].startObservability
				bundle.ready = constructions[0].observabilityReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].conversationSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-conversation" {
				bundle.start = constructions[0].startConversation
				bundle.ready = constructions[0].conversationReady
				break
			}
		}
	}
	if len(constructions) == 1 && (constructions[0].vaultSlot != nil ||
		constructions[0].legacyCredentialSlot != nil) {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-vault" {
				bundle.start = constructions[0].startVault
				bundle.ready = constructions[0].vaultReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].governanceSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-governance" {
				bundle.start = constructions[0].startGovernance
				bundle.ready = constructions[0].governanceReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].agentRuntimeSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-agent-runtime" {
				bundle.start = constructions[0].startAgentRuntime
				bundle.ready = constructions[0].agentRuntimeReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].workSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-work" {
				bundle.start = constructions[0].startWork
				bundle.ready = constructions[0].workReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].localIPCSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-local-ipc" {
				bundle.start = constructions[0].startLocalIPC
				bundle.ready = constructions[0].localIPCReady
				break
			}
		}
	} else if len(constructions) == 1 && constructions[0].setupSlot != nil {
		for index := range result {
			bundle := result[index].(*productCompatibilityBundle)
			if bundle.descriptor.ID == "loom-local-ipc" {
				bundle.start = constructions[0].startSetup
				bundle.ready = constructions[0].setupReady
				break
			}
		}
	}
	if len(constructions) == 1 && constructions[0].localIPCSlot != nil {
		handler = constructions[0].localIPCSlot
	}
	result[len(result)-1].(*productCompatibilityBundle).handler = handler
	return result
}

func compatibilityDescriptor(
	id string,
	coreProtected bool,
	requires []composition.CapabilityRef,
	provides []composition.CapabilityRef,
) composition.BundleDescriptor {
	return composition.BundleDescriptor{
		SchemaVersion: 1, ID: id, Version: "1.0.0", LifecycleID: id + "-v1",
		CoreProtected: coreProtected,
		Requires:      append([]composition.CapabilityRef(nil), requires...),
		Provides:      append([]composition.CapabilityRef(nil), provides...),
	}
}

func cloneProductCompatibilityDescriptor(
	value composition.BundleDescriptor,
) composition.BundleDescriptor {
	result := value
	result.Requires = append([]composition.CapabilityRef(nil), value.Requires...)
	result.OptionalRequires = append([]composition.CapabilityRef(nil), value.OptionalRequires...)
	result.Provides = append([]composition.CapabilityRef(nil), value.Provides...)
	result.Compatibility = append([]composition.BundleConstraint(nil), value.Compatibility...)
	result.Routes = make([]composition.RouteDescriptor, len(value.Routes))
	for index, route := range value.Routes {
		result.Routes[index] = route
		result.Routes[index].RequiredCapabilities = append(
			[]composition.CapabilityRef(nil), route.RequiredCapabilities...,
		)
	}
	return result
}
