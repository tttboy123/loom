package main

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

type productAssetRoute interface {
	EvolutionAssetSnapshot(context.Context, api.EvolutionAssetSnapshotRequest) (api.EvolutionAssetSnapshot, error)
	EvolutionAssetDiff(context.Context, api.EvolutionAssetDiffRequest) (api.EvolutionAssetDiff, error)
	EvolutionAssetCommand(context.Context, api.EvolutionAssetCommandRequest) (api.EvolutionAssetCommandResult, error)
}

type productAssetBundle struct {
	route        productAssetRoute
	materializer app.TeamAssetMaterializer
	close        func() error
}

func (bundle productAssetBundle) valid() bool {
	return !nilProductAssetPort(bundle.route) &&
		!nilProductAssetPort(bundle.materializer) && bundle.close != nil
}

type productAssetRouteSlot struct {
	mu     sync.RWMutex
	bundle productAssetBundle
	bound  bool
	closed bool
}

func (slot *productAssetRouteSlot) Bind(bundle productAssetBundle) error {
	if slot == nil || !bundle.valid() {
		return api.ErrInvalidLocalProductAssetAPI
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.bound || slot.closed {
		return composition.ErrCompositionConflict
	}
	slot.bundle = bundle
	slot.bound = true
	return nil
}

func (slot *productAssetRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.bundle.valid()
}

func (slot *productAssetRouteSlot) TeamAssetMaterializer() (app.TeamAssetMaterializer, error) {
	if slot == nil {
		return nil, api.ErrInvalidLocalProductAssetAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.bundle.valid() {
		return nil, api.ErrInvalidLocalProductAssetAPI
	}
	return slot.bundle.materializer, nil
}

func (slot *productAssetRouteSlot) EvolutionAssetSnapshot(
	ctx context.Context,
	request api.EvolutionAssetSnapshotRequest,
) (api.EvolutionAssetSnapshot, error) {
	if slot == nil {
		return api.EvolutionAssetSnapshot{}, api.ErrInvalidLocalProductAssetAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.bundle.valid() {
		return api.EvolutionAssetSnapshot{}, api.ErrInvalidLocalProductAssetAPI
	}
	return slot.bundle.route.EvolutionAssetSnapshot(ctx, request)
}

func (slot *productAssetRouteSlot) EvolutionAssetDiff(
	ctx context.Context,
	request api.EvolutionAssetDiffRequest,
) (api.EvolutionAssetDiff, error) {
	if slot == nil {
		return api.EvolutionAssetDiff{}, api.ErrInvalidLocalProductAssetAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.bundle.valid() {
		return api.EvolutionAssetDiff{}, api.ErrInvalidLocalProductAssetAPI
	}
	return slot.bundle.route.EvolutionAssetDiff(ctx, request)
}

func (slot *productAssetRouteSlot) EvolutionAssetCommand(
	ctx context.Context,
	request api.EvolutionAssetCommandRequest,
) (api.EvolutionAssetCommandResult, error) {
	if slot == nil {
		return api.EvolutionAssetCommandResult{}, api.ErrInvalidLocalProductAssetAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.bundle.valid() {
		return api.EvolutionAssetCommandResult{}, api.ErrInvalidLocalProductAssetAPI
	}
	return slot.bundle.route.EvolutionAssetCommand(ctx, request)
}

func (slot *productAssetRouteSlot) Close(context.Context) error {
	if slot == nil {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	closeBundle := slot.bundle.close
	slot.bundle = productAssetBundle{}
	if closeBundle == nil {
		return nil
	}
	return closeBundle()
}

func nilProductAssetPort(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

type productCompatibilityConstruction struct {
	coreSlot                *productCoreRouteSlot
	coreFactory             func(context.Context) (productCoreResources, error)
	scopeSlot               *productCapabilityScopeSlot
	observabilitySlot       *productObservabilityRouteSlot
	observabilityFactory    func(context.Context) (productObservabilityRoutes, error)
	conversationSlot        *productConversationRouteSlot
	conversationFactory     func(context.Context) (productConversationRoutes, error)
	vaultSlot               *productVaultRouteSlot
	vaultFactory            func(context.Context) (productVaultRoutes, error)
	legacyCredentialSlot    *productCredentialLeaseRouteSlot
	legacyCredentialFactory func(context.Context) (productCredentialLeaseAccess, error)
	governanceSlot          *productGovernanceRouteSlot
	governanceFactory       func(context.Context) (productGovernanceRoutes, error)
	assetSlot               *productAssetRouteSlot
	assetFactory            func(context.Context) (productAssetBundle, error)
	agentRuntimeSlot        *productAgentRuntimeRouteSlot
	agentRuntimeFactory     func(context.Context) (productAgentRuntimeRoutes, error)
	workSlot                *productWorkRouteSlot
	workFactory             func(context.Context) (productWorkRoutes, error)
	setupSlot               *productSetupRouteSlot
	setupFactory            func(context.Context) (productSetupRoutes, error)
	readSlot                *productReadRouteSlot
	readFactory             func(context.Context) (productReadRoutes, error)
	localIPCSlot            *productLocalIPCHandlerSlot
	localIPCFactory         func(context.Context) (localipc.Handler, error)
}

func newProductAssetRouteFactory(
	store *journal.Store,
	readModel *projection.Projection,
	statePath string,
	refresh app.EvolutionAssetProjectionRefresh,
) func(context.Context) (productAssetBundle, error) {
	return func(ctx context.Context) (productAssetBundle, error) {
		if ctx == nil || store == nil || readModel == nil || !filepath.IsAbs(statePath) {
			return productAssetBundle{}, api.ErrInvalidLocalProductAssetAPI
		}
		assetEvidenceRoot := filepath.Join(filepath.Dir(statePath), "evidence")
		if err := ensureProductExecutionDirectory(assetEvidenceRoot); err != nil {
			return productAssetBundle{}, err
		}
		artifacts, err := evidence.NewStore(assetEvidenceRoot)
		if err != nil {
			return productAssetBundle{}, err
		}
		closeOnFailure := func(cause error) (productAssetBundle, error) {
			return productAssetBundle{}, errors.Join(cause, artifacts.Close())
		}
		authority, err := assets.NewAuthority(assets.AuthorityConfig{
			Store: store,
			Now:   func() time.Time { return time.Now().UTC() },
			ViewVersion: func() string {
				return readModel.GlobalReadView().Version()
			},
			Subjects: &productAssetSubjectResolver{projection: readModel},
			Promotion: &productAssetPromotionResolver{
				projection: readModel,
				artifacts:  artifacts,
			},
			TemplateArtifacts: productTemplateArtifactResolver{artifacts: artifacts},
			TemplateOutputs:   productTemplateOutputSink{},
		})
		if err != nil {
			return closeOnFailure(err)
		}
		service, err := app.NewLocalProductAssetService(
			readModel, authority, artifacts, refresh,
		)
		if err != nil {
			return closeOnFailure(err)
		}
		route, err := api.NewLocalProductAssetAPI(service)
		if err != nil {
			return closeOnFailure(err)
		}
		skillMaterializer, err := piadapter.NewSkillMaterializer(
			piadapter.MaterializationHooks{},
		)
		if err != nil {
			return closeOnFailure(err)
		}
		teamMaterializer := &productTeamAssetMaterializer{
			authority: authority, projection: readModel,
			artifacts: artifacts, materializer: skillMaterializer,
			isolatedRoot: filepath.Join(filepath.Dir(statePath), "materialization"),
			plans:        make(map[string]piadapter.MaterializationPlan),
		}
		if err := teamMaterializer.RecoverStartup(ctx); err != nil {
			return closeOnFailure(err)
		}
		return productAssetBundle{
			route: route, materializer: teamMaterializer, close: artifacts.Close,
		}, nil
	}
}

func newProductAssetRouteFactoryFromCore(
	core *productCoreRouteSlot,
	statePath string,
	refresh app.EvolutionAssetProjectionRefresh,
) func(context.Context) (productAssetBundle, error) {
	return func(ctx context.Context) (productAssetBundle, error) {
		resources, err := core.Resources()
		if err != nil {
			return productAssetBundle{}, err
		}
		return newProductAssetRouteFactory(
			resources.store, resources.readModel, statePath, refresh,
		)(ctx)
	}
}

func (construction productCompatibilityConstruction) valid() bool {
	return (construction.coreSlot == nil) == (construction.coreFactory == nil) &&
		(construction.observabilitySlot == nil) == (construction.observabilityFactory == nil) &&
		(construction.conversationSlot == nil) == (construction.conversationFactory == nil) &&
		(construction.vaultSlot == nil) == (construction.vaultFactory == nil) &&
		(construction.legacyCredentialSlot == nil) == (construction.legacyCredentialFactory == nil) &&
		(construction.vaultSlot == nil || construction.legacyCredentialSlot == nil) &&
		(construction.governanceSlot == nil) == (construction.governanceFactory == nil) &&
		(construction.assetSlot == nil) == (construction.assetFactory == nil) &&
		(construction.agentRuntimeSlot == nil) == (construction.agentRuntimeFactory == nil) &&
		(construction.workSlot == nil) == (construction.workFactory == nil) &&
		(construction.setupSlot == nil) == (construction.setupFactory == nil) &&
		(construction.readSlot == nil) == (construction.readFactory == nil) &&
		(construction.localIPCSlot == nil) == (construction.localIPCFactory == nil) &&
		(construction.agentRuntimeSlot == nil ||
			construction.assetSlot != nil && construction.workSlot != nil) &&
		(construction.readSlot == nil || construction.governanceSlot != nil)
}

func (construction productCompatibilityConstruction) startAssets(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.assetSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	bundle, err := construction.assetFactory(ctx)
	if err != nil || !bundle.valid() {
		if bundle.close != nil {
			_ = bundle.close()
		}
		return nil, errors.Join(api.ErrInvalidLocalProductAssetAPI, err)
	}
	if err := construction.assetSlot.Bind(bundle); err != nil {
		_ = bundle.close()
		return nil, err
	}
	return composition.NewEffect(construction.assetSlot.Close), nil
}

func (construction productCompatibilityConstruction) assetsReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.assetSlot == nil ||
		!construction.assetSlot.Ready() {
		return api.ErrInvalidLocalProductAssetAPI
	}
	return nil
}

var _ productAssetRoute = (*api.LocalProductAssetAPI)(nil)
var _ productAssetRoute = (*productAssetRouteSlot)(nil)
