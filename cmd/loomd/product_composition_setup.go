package main

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

var errProductSetupRouteUnavailable = errors.New("product Setup route unavailable")

type productSetupRoute interface {
	SetupSnapshot(context.Context) (app.SetupSnapshot, error)
	ConnectCodex(context.Context) (app.ProviderConnectResult, error)
	ConnectClaudeCode(context.Context) (app.ProviderConnectResult, error)
	CancelClaudeCode(context.Context) (app.ProviderConnectResult, error)
	ConfigureProviderAccountPolicy(context.Context, app.ProviderAccountPolicyCommand) (app.ProviderAccountPolicyResult, error)
	ConfigureProviderModelRateCard(context.Context, app.ProviderModelRateCardCommand) (app.ProviderModelRateCardResult, error)
	ConfigureRemoteToolBackendEnrollment(context.Context, app.RemoteToolBackendEnrollmentCommand) (app.RemoteToolBackendEnrollmentResult, error)
	RevokeRemoteToolBackendEnrollment(context.Context, app.RemoteToolBackendEnrollmentRevokeCommand) (app.RemoteToolBackendEnrollmentResult, error)
	StartBuilder(context.Context, app.BuilderStartCommand) (app.BuilderSessionView, error)
	AnswerBuilder(context.Context, app.BuilderAnswerCommand) (app.BuilderSessionView, error)
	EditBuilder(context.Context, app.BuilderEditCommand) (app.BuilderSessionView, error)
	ValidateBuilder(context.Context, app.BuilderValidateCommand) (app.BuilderSessionView, error)
	ConfirmBuilder(context.Context, app.BuilderConfirmCommand) (app.BuilderConfirmation, error)
	ArchiveTeam(context.Context, app.TeamStatusCommand) (app.SetupSavedTeamPreview, error)
	RestoreTeam(context.Context, app.TeamStatusCommand) (app.SetupSavedTeamPreview, error)
	ConfigureCredential(context.Context, app.CredentialSetupCommand) (app.CredentialSetupResult, error)
	ImportCredentialCandidate(context.Context, app.CredentialImportCommand) (app.CredentialSetupResult, error)
	ApproveEndpointCandidate(context.Context, app.EndpointReviewCommand) (app.EndpointReviewResult, error)
	VerifyCredential(context.Context, app.CredentialSetupCommand) (app.CredentialSetupResult, error)
	ReplaceCredential(context.Context, app.CredentialSetupCommand) (app.CredentialSetupResult, error)
	RevokeCredential(context.Context, app.CredentialSetupCommand) (app.CredentialSetupResult, error)
}

type productSetupRoutes struct {
	route productSetupRoute
	close func() error
}

func (routes productSetupRoutes) valid() bool {
	return !nilProductAssetPort(routes.route) && routes.close != nil
}

type productSetupRouteSlot struct {
	mu     sync.RWMutex
	routes productSetupRoutes
	bound  bool
	closed bool
}

func (slot *productSetupRouteSlot) Bind(routes productSetupRoutes) error {
	if slot == nil || !routes.valid() {
		return api.ErrInvalidLocalProductSetupAPI
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

func (slot *productSetupRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productSetupRouteSlot) Close() error {
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
	slot.routes = productSetupRoutes{}
	if closeRoute == nil {
		return nil
	}
	return closeRoute()
}

func newProductSetupRouteFactory(
	database *sql.DB,
	store *journal.Store,
	readModel *projection.Projection,
	config productSetupRuntimeConfig,
) func(context.Context) (productSetupRoutes, error) {
	return func(ctx context.Context) (productSetupRoutes, error) {
		if ctx == nil || ctx.Err() != nil {
			return productSetupRoutes{}, errProductSetupRouteUnavailable
		}
		route, err := buildProductSetupService(database, store, readModel, config)
		if err != nil {
			return productSetupRoutes{}, err
		}
		return productSetupRoutes{route: route, close: route.Close}, nil
	}
}

func newProductSetupRouteFactoryFromCore(
	core *productCoreRouteSlot,
	config productSetupRuntimeConfig,
) func(context.Context) (productSetupRoutes, error) {
	return func(ctx context.Context) (productSetupRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productSetupRoutes{}, err
		}
		return newProductSetupRouteFactory(
			resources.database, resources.store, resources.readModel, config,
		)(ctx)
	}
}

func (construction productCompatibilityConstruction) startSetup(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.setupSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.setupFactory(ctx)
	if err != nil || !routes.valid() {
		if routes.close != nil {
			_ = routes.close()
		}
		return nil, errors.Join(errProductSetupRouteUnavailable, err)
	}
	if err := construction.setupSlot.Bind(routes); err != nil {
		_ = routes.close()
		return nil, err
	}
	return composition.NewEffect(func(context.Context) error {
		return construction.setupSlot.Close()
	}), nil
}

func (construction productCompatibilityConstruction) setupReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.setupSlot == nil ||
		!construction.setupSlot.Ready() {
		return errProductSetupRouteUnavailable
	}
	return nil
}

func (slot *productSetupRouteSlot) SetupSnapshot(ctx context.Context) (app.SetupSnapshot, error) {
	if slot == nil {
		return app.SetupSnapshot{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.SetupSnapshot{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.SetupSnapshot(ctx)
}

func (slot *productSetupRouteSlot) ConnectCodex(ctx context.Context) (app.ProviderConnectResult, error) {
	if slot == nil {
		return app.ProviderConnectResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ProviderConnectResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ConnectCodex(ctx)
}

func (slot *productSetupRouteSlot) ConnectClaudeCode(
	ctx context.Context,
) (app.ProviderConnectResult, error) {
	if slot == nil {
		return app.ProviderConnectResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ProviderConnectResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ConnectClaudeCode(ctx)
}

func (slot *productSetupRouteSlot) CancelClaudeCode(
	ctx context.Context,
) (app.ProviderConnectResult, error) {
	if slot == nil {
		return app.ProviderConnectResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ProviderConnectResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.CancelClaudeCode(ctx)
}

func (slot *productSetupRouteSlot) ConfigureProviderAccountPolicy(ctx context.Context, command app.ProviderAccountPolicyCommand) (app.ProviderAccountPolicyResult, error) {
	if slot == nil {
		return app.ProviderAccountPolicyResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ProviderAccountPolicyResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ConfigureProviderAccountPolicy(ctx, command)
}

func (slot *productSetupRouteSlot) ConfigureProviderModelRateCard(ctx context.Context, command app.ProviderModelRateCardCommand) (app.ProviderModelRateCardResult, error) {
	if slot == nil {
		return app.ProviderModelRateCardResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ProviderModelRateCardResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ConfigureProviderModelRateCard(ctx, command)
}

func (slot *productSetupRouteSlot) ConfigureRemoteToolBackendEnrollment(
	ctx context.Context,
	command app.RemoteToolBackendEnrollmentCommand,
) (app.RemoteToolBackendEnrollmentResult, error) {
	if slot == nil {
		return app.RemoteToolBackendEnrollmentResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.RemoteToolBackendEnrollmentResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ConfigureRemoteToolBackendEnrollment(ctx, command)
}

func (slot *productSetupRouteSlot) RevokeRemoteToolBackendEnrollment(
	ctx context.Context,
	command app.RemoteToolBackendEnrollmentRevokeCommand,
) (app.RemoteToolBackendEnrollmentResult, error) {
	if slot == nil {
		return app.RemoteToolBackendEnrollmentResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.RemoteToolBackendEnrollmentResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.RevokeRemoteToolBackendEnrollment(ctx, command)
}

func (slot *productSetupRouteSlot) StartBuilder(ctx context.Context, command app.BuilderStartCommand) (app.BuilderSessionView, error) {
	return slot.builderSession(func(route productSetupRoute) (app.BuilderSessionView, error) {
		return route.StartBuilder(ctx, command)
	})
}

func (slot *productSetupRouteSlot) AnswerBuilder(ctx context.Context, command app.BuilderAnswerCommand) (app.BuilderSessionView, error) {
	return slot.builderSession(func(route productSetupRoute) (app.BuilderSessionView, error) {
		return route.AnswerBuilder(ctx, command)
	})
}

func (slot *productSetupRouteSlot) EditBuilder(ctx context.Context, command app.BuilderEditCommand) (app.BuilderSessionView, error) {
	return slot.builderSession(func(route productSetupRoute) (app.BuilderSessionView, error) {
		return route.EditBuilder(ctx, command)
	})
}

func (slot *productSetupRouteSlot) ValidateBuilder(ctx context.Context, command app.BuilderValidateCommand) (app.BuilderSessionView, error) {
	return slot.builderSession(func(route productSetupRoute) (app.BuilderSessionView, error) {
		return route.ValidateBuilder(ctx, command)
	})
}

func (slot *productSetupRouteSlot) builderSession(
	call func(productSetupRoute) (app.BuilderSessionView, error),
) (app.BuilderSessionView, error) {
	if slot == nil || call == nil {
		return app.BuilderSessionView{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.BuilderSessionView{}, api.ErrInvalidLocalProductSetupAPI
	}
	return call(slot.routes.route)
}

func (slot *productSetupRouteSlot) ConfirmBuilder(ctx context.Context, command app.BuilderConfirmCommand) (app.BuilderConfirmation, error) {
	if slot == nil {
		return app.BuilderConfirmation{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.BuilderConfirmation{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ConfirmBuilder(ctx, command)
}

func (slot *productSetupRouteSlot) ArchiveTeam(ctx context.Context, command app.TeamStatusCommand) (app.SetupSavedTeamPreview, error) {
	return slot.teamStatus(func(route productSetupRoute) (app.SetupSavedTeamPreview, error) {
		return route.ArchiveTeam(ctx, command)
	})
}

func (slot *productSetupRouteSlot) RestoreTeam(ctx context.Context, command app.TeamStatusCommand) (app.SetupSavedTeamPreview, error) {
	return slot.teamStatus(func(route productSetupRoute) (app.SetupSavedTeamPreview, error) {
		return route.RestoreTeam(ctx, command)
	})
}

func (slot *productSetupRouteSlot) teamStatus(
	call func(productSetupRoute) (app.SetupSavedTeamPreview, error),
) (app.SetupSavedTeamPreview, error) {
	if slot == nil || call == nil {
		return app.SetupSavedTeamPreview{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.SetupSavedTeamPreview{}, api.ErrInvalidLocalProductSetupAPI
	}
	return call(slot.routes.route)
}

func (slot *productSetupRouteSlot) ConfigureCredential(ctx context.Context, command app.CredentialSetupCommand) (app.CredentialSetupResult, error) {
	return slot.credential(command, func(route productSetupRoute) (app.CredentialSetupResult, error) {
		return route.ConfigureCredential(ctx, command)
	})
}

func (slot *productSetupRouteSlot) ImportCredentialCandidate(
	ctx context.Context,
	command app.CredentialImportCommand,
) (app.CredentialSetupResult, error) {
	if slot == nil || ctx == nil {
		return app.CredentialSetupResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.CredentialSetupResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ImportCredentialCandidate(ctx, command)
}

func (slot *productSetupRouteSlot) ApproveEndpointCandidate(
	ctx context.Context,
	command app.EndpointReviewCommand,
) (app.EndpointReviewResult, error) {
	if slot == nil || ctx == nil {
		return app.EndpointReviewResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.EndpointReviewResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return slot.routes.route.ApproveEndpointCandidate(ctx, command)
}

func (slot *productSetupRouteSlot) VerifyCredential(ctx context.Context, command app.CredentialSetupCommand) (app.CredentialSetupResult, error) {
	return slot.credential(command, func(route productSetupRoute) (app.CredentialSetupResult, error) {
		return route.VerifyCredential(ctx, command)
	})
}

func (slot *productSetupRouteSlot) ReplaceCredential(ctx context.Context, command app.CredentialSetupCommand) (app.CredentialSetupResult, error) {
	return slot.credential(command, func(route productSetupRoute) (app.CredentialSetupResult, error) {
		return route.ReplaceCredential(ctx, command)
	})
}

func (slot *productSetupRouteSlot) RevokeCredential(ctx context.Context, command app.CredentialSetupCommand) (app.CredentialSetupResult, error) {
	return slot.credential(command, func(route productSetupRoute) (app.CredentialSetupResult, error) {
		return route.RevokeCredential(ctx, command)
	})
}

func (slot *productSetupRouteSlot) credential(
	command app.CredentialSetupCommand,
	call func(productSetupRoute) (app.CredentialSetupResult, error),
) (app.CredentialSetupResult, error) {
	if slot == nil || call == nil {
		clearProductCredentialBytes(command.Secret)
		return app.CredentialSetupResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		clearProductCredentialBytes(command.Secret)
		return app.CredentialSetupResult{}, api.ErrInvalidLocalProductSetupAPI
	}
	return call(slot.routes.route)
}

var _ productSetupRoute = (*api.LocalProductSetupAPI)(nil)
var _ productSetupRoute = (*productSetupRouteSlot)(nil)
