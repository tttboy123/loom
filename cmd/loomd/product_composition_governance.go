package main

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/toolproposal"
	"loom-pi-rebuild/internal/work"
)

var (
	errProductGovernancePermissionConstruction     = errors.New("product Governance Permission construction failed")
	errProductGovernanceCustomerRuleConstruction   = errors.New("product Governance Customer Rule construction failed")
	errProductGovernanceStandingOrderConstruction  = errors.New("product Governance Standing Order construction failed")
	errProductGovernanceDecisionConstruction       = errors.New("product Governance Decision construction failed")
	errProductGovernanceProviderPolicyConstruction = errors.New("product Governance Provider policy construction failed")
)

type productPermissionRoute interface {
	PermissionSnapshot(context.Context, app.PermissionSnapshotRequest) (app.PermissionSnapshot, error)
	PermissionAttention(context.Context, app.PermissionAttentionRequest) (app.PermissionAttention, error)
	PermissionCommand(context.Context, app.PermissionCommandRequest) (app.PermissionCommandResult, error)
}

type productCustomerRuleRoute interface {
	Snapshot(context.Context, app.CustomerRuleSnapshotRequest) (app.CustomerRuleSnapshot, error)
	Command(context.Context, app.CustomerRuleCommandRequest) (app.CustomerRuleCommandResult, error)
}

type productStandingOrderRoute interface {
	Snapshot(context.Context, app.StandingOrderSnapshotRequest) (app.StandingOrderSnapshot, error)
	Command(context.Context, app.StandingOrderCommandRequest) (app.StandingOrderCommandResult, error)
}

type productGovernanceApprovalPort interface {
	RequestPermissionApproval(context.Context, rules.PermissionApprovalInput) (rules.ApprovalRequestRecord, error)
	ConsumePermissionApproval(context.Context, rules.PermissionApprovalConsumptionInput) (rules.ApprovalRequestRecord, error)
}

type productDecisionRoute interface {
	ReadMissionDecision(context.Context, app.MissionDecisionCommand) (app.MissionDecisionSheet, error)
	DecideMission(context.Context, app.MissionDecisionCommand) (app.MissionDecisionResult, error)
}

type productGovernanceRoutes struct {
	permission                   productPermissionRoute
	customerRule                 productCustomerRuleRoute
	standingOrder                productStandingOrderRoute
	approvals                    productGovernanceApprovalPort
	decision                     productDecisionRoute
	decisionCommands             api.MissionDecisionCommandSource
	executionDecisions           app.MissionExecutionDecisionRouter
	fallbackDecisions            app.MissionFallbackDecisionPreparer
	providerAccountPolicies      app.ProviderAccountPolicyAuthority
	providerModelRateCards       app.ProviderModelRateCardAuthority
	remoteToolBackendEnrollments app.RemoteToolBackendEnrollmentAuthority
	close                        func() error
}

func (routes productGovernanceRoutes) valid() bool {
	return !nilProductAssetPort(routes.permission) &&
		!nilProductAssetPort(routes.customerRule) &&
		!nilProductAssetPort(routes.standingOrder) &&
		!nilProductAssetPort(routes.approvals) &&
		!nilProductAssetPort(routes.decision) &&
		!nilProductAssetPort(routes.decisionCommands) &&
		!nilProductAssetPort(routes.executionDecisions) &&
		!nilProductAssetPort(routes.fallbackDecisions) &&
		!nilProductAssetPort(routes.providerAccountPolicies) &&
		!nilProductAssetPort(routes.providerModelRateCards) &&
		!nilProductAssetPort(routes.remoteToolBackendEnrollments) && routes.close != nil
}

type productGovernanceRouteSlot struct {
	mu     sync.RWMutex
	routes productGovernanceRoutes
	bound  bool
	closed bool
}

func (slot *productGovernanceRouteSlot) Bind(routes productGovernanceRoutes) error {
	if slot == nil || !routes.valid() {
		return api.ErrInvalidLocalPermissionAPI
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

func (slot *productGovernanceRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productGovernanceRouteSlot) PermissionSnapshot(
	ctx context.Context,
	request app.PermissionSnapshotRequest,
) (app.PermissionSnapshot, error) {
	if slot == nil {
		return app.PermissionSnapshot{}, api.ErrInvalidLocalPermissionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.PermissionSnapshot{}, api.ErrInvalidLocalPermissionAPI
	}
	return slot.routes.permission.PermissionSnapshot(ctx, request)
}

func (slot *productGovernanceRouteSlot) PermissionAttention(
	ctx context.Context,
	request app.PermissionAttentionRequest,
) (app.PermissionAttention, error) {
	if slot == nil {
		return app.PermissionAttention{}, api.ErrInvalidLocalPermissionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.PermissionAttention{}, api.ErrInvalidLocalPermissionAPI
	}
	return slot.routes.permission.PermissionAttention(ctx, request)
}

func (slot *productGovernanceRouteSlot) PermissionCommand(
	ctx context.Context,
	request app.PermissionCommandRequest,
) (app.PermissionCommandResult, error) {
	if slot == nil {
		return app.PermissionCommandResult{}, api.ErrInvalidLocalPermissionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.PermissionCommandResult{}, api.ErrInvalidLocalPermissionAPI
	}
	return slot.routes.permission.PermissionCommand(ctx, request)
}

func (slot *productGovernanceRouteSlot) customerRuleSnapshot(
	ctx context.Context,
	request app.CustomerRuleSnapshotRequest,
) (app.CustomerRuleSnapshot, error) {
	if slot == nil {
		return app.CustomerRuleSnapshot{}, api.ErrInvalidLocalCustomerRuleAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.CustomerRuleSnapshot{}, api.ErrInvalidLocalCustomerRuleAPI
	}
	return slot.routes.customerRule.Snapshot(ctx, request)
}

func (slot *productGovernanceRouteSlot) customerRuleCommand(
	ctx context.Context,
	request app.CustomerRuleCommandRequest,
) (app.CustomerRuleCommandResult, error) {
	if slot == nil {
		return app.CustomerRuleCommandResult{}, api.ErrInvalidLocalCustomerRuleAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.CustomerRuleCommandResult{}, api.ErrInvalidLocalCustomerRuleAPI
	}
	return slot.routes.customerRule.Command(ctx, request)
}

func (slot *productGovernanceRouteSlot) standingOrderSnapshot(
	ctx context.Context,
	request app.StandingOrderSnapshotRequest,
) (app.StandingOrderSnapshot, error) {
	if slot == nil {
		return app.StandingOrderSnapshot{}, api.ErrInvalidLocalStandingOrderAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.StandingOrderSnapshot{}, api.ErrInvalidLocalStandingOrderAPI
	}
	return slot.routes.standingOrder.Snapshot(ctx, request)
}

func (slot *productGovernanceRouteSlot) standingOrderCommand(
	ctx context.Context,
	request app.StandingOrderCommandRequest,
) (app.StandingOrderCommandResult, error) {
	if slot == nil {
		return app.StandingOrderCommandResult{}, api.ErrInvalidLocalStandingOrderAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.StandingOrderCommandResult{}, api.ErrInvalidLocalStandingOrderAPI
	}
	return slot.routes.standingOrder.Command(ctx, request)
}

func (slot *productGovernanceRouteSlot) Approvals() (productGovernanceApprovalPort, error) {
	if slot == nil {
		return nil, api.ErrInvalidLocalPermissionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return nil, api.ErrInvalidLocalPermissionAPI
	}
	return slot.routes.approvals, nil
}

func (slot *productGovernanceRouteSlot) ReadMissionDecision(
	ctx context.Context, command app.MissionDecisionCommand,
) (app.MissionDecisionSheet, error) {
	if slot == nil {
		return app.MissionDecisionSheet{}, api.ErrInvalidLocalProductDecisionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.MissionDecisionSheet{}, api.ErrInvalidLocalProductDecisionAPI
	}
	return slot.routes.decision.ReadMissionDecision(ctx, command)
}

func (slot *productGovernanceRouteSlot) DecideMission(
	ctx context.Context, command app.MissionDecisionCommand,
) (app.MissionDecisionResult, error) {
	if slot == nil {
		return app.MissionDecisionResult{}, api.ErrInvalidLocalProductDecisionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.MissionDecisionResult{}, api.ErrInvalidLocalProductDecisionAPI
	}
	return slot.routes.decision.DecideMission(ctx, command)
}

func (slot *productGovernanceRouteSlot) ListMissionDecisionCommands(
	ctx context.Context, query app.MissionDecisionCommandQuery,
) ([]app.MissionDecisionCommand, error) {
	if slot == nil {
		return nil, api.ErrInvalidLocalProductDecisionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return nil, api.ErrInvalidLocalProductDecisionAPI
	}
	return slot.routes.decisionCommands.ListMissionDecisionCommands(ctx, query)
}

func (slot *productGovernanceRouteSlot) RouteMissionExecutionControl(
	ctx context.Context, command app.MissionExecutionCommand,
) error {
	if slot == nil {
		return api.ErrInvalidLocalProductDecisionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.ErrInvalidLocalProductDecisionAPI
	}
	return slot.routes.executionDecisions.RouteMissionExecutionControl(ctx, command)
}

func (slot *productGovernanceRouteSlot) PrepareMissionFallbackDecisions(
	ctx context.Context,
	teamInstanceID string,
	viewVersion string,
	candidates []app.MissionFallbackDecisionCandidate,
) error {
	if slot == nil {
		return api.ErrInvalidLocalProductDecisionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.ErrInvalidLocalProductDecisionAPI
	}
	return slot.routes.fallbackDecisions.PrepareMissionFallbackDecisions(
		ctx, teamInstanceID, viewVersion, candidates,
	)
}

func (slot *productGovernanceRouteSlot) ConfigureProviderAccountPolicy(
	ctx context.Context,
	command work.ProviderAccountPolicyCommand,
) (work.ProviderAccountPolicy, error) {
	if slot == nil {
		return work.ProviderAccountPolicy{}, app.ErrProviderAccountPolicyUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return work.ProviderAccountPolicy{}, app.ErrProviderAccountPolicyUnavailable
	}
	return slot.routes.providerAccountPolicies.ConfigureProviderAccountPolicy(ctx, command)
}

func (slot *productGovernanceRouteSlot) ConfigureProviderModelRateCard(
	ctx context.Context,
	command work.ProviderModelRateCardCommand,
) (work.ProviderModelRateCard, error) {
	if slot == nil {
		return work.ProviderModelRateCard{}, app.ErrProviderModelRateCardUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return work.ProviderModelRateCard{}, app.ErrProviderModelRateCardUnavailable
	}
	return slot.routes.providerModelRateCards.ConfigureProviderModelRateCard(ctx, command)
}

func (slot *productGovernanceRouteSlot) ConfigureRemoteToolBackendEnrollment(
	ctx context.Context,
	command work.RemoteToolBackendEnrollmentCommand,
) (work.RemoteToolBackendEnrollment, error) {
	if slot == nil {
		return work.RemoteToolBackendEnrollment{}, app.ErrRemoteToolBackendEnrollmentUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return work.RemoteToolBackendEnrollment{}, app.ErrRemoteToolBackendEnrollmentUnavailable
	}
	return slot.routes.remoteToolBackendEnrollments.ConfigureRemoteToolBackendEnrollment(ctx, command)
}

func (slot *productGovernanceRouteSlot) RevokeRemoteToolBackendEnrollment(
	ctx context.Context,
	command work.RemoteToolBackendEnrollmentRevokeCommand,
) (work.RemoteToolBackendEnrollment, error) {
	if slot == nil {
		return work.RemoteToolBackendEnrollment{}, app.ErrRemoteToolBackendEnrollmentUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return work.RemoteToolBackendEnrollment{}, app.ErrRemoteToolBackendEnrollmentUnavailable
	}
	return slot.routes.remoteToolBackendEnrollments.RevokeRemoteToolBackendEnrollment(ctx, command)
}

func (slot *productGovernanceRouteSlot) Close(context.Context) error {
	if slot == nil {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	closeRoutes := slot.routes.close
	slot.routes = productGovernanceRoutes{}
	if closeRoutes == nil {
		return nil
	}
	return closeRoutes()
}

type productCustomerRuleRouteProxy struct{ slot *productGovernanceRouteSlot }

func (proxy productCustomerRuleRouteProxy) Snapshot(
	ctx context.Context, request app.CustomerRuleSnapshotRequest,
) (app.CustomerRuleSnapshot, error) {
	return proxy.slot.customerRuleSnapshot(ctx, request)
}
func (proxy productCustomerRuleRouteProxy) Command(
	ctx context.Context, request app.CustomerRuleCommandRequest,
) (app.CustomerRuleCommandResult, error) {
	return proxy.slot.customerRuleCommand(ctx, request)
}

type productStandingOrderRouteProxy struct{ slot *productGovernanceRouteSlot }

func (proxy productStandingOrderRouteProxy) Snapshot(
	ctx context.Context, request app.StandingOrderSnapshotRequest,
) (app.StandingOrderSnapshot, error) {
	return proxy.slot.standingOrderSnapshot(ctx, request)
}
func (proxy productStandingOrderRouteProxy) Command(
	ctx context.Context, request app.StandingOrderCommandRequest,
) (app.StandingOrderCommandResult, error) {
	return proxy.slot.standingOrderCommand(ctx, request)
}

func newProductGovernanceRouteFactory(
	store *journal.Store,
	readModel *projection.Projection,
	proposalStore toolproposal.Store,
	prepared app.PreparedMissionDecisions,
) func(context.Context) (productGovernanceRoutes, error) {
	return func(ctx context.Context) (productGovernanceRoutes, error) {
		if ctx == nil || store == nil || readModel == nil {
			return productGovernanceRoutes{}, api.ErrInvalidLocalPermissionAPI
		}
		approvalPort, err := newPermissionApprovalPort(
			store, func() time.Time { return time.Now().UTC() },
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernancePermissionConstruction, err)
		}
		permissionService, err := app.NewLocalPermissionService(
			store,
			func() time.Time { return time.Now().UTC() },
			func() string { return readModel.GlobalReadView().Version() },
			approvalPort,
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernancePermissionConstruction, err)
		}
		if proposalStore != nil {
			if err := permissionService.SetToolProposalStore(proposalStore); err != nil {
				return productGovernanceRoutes{}, errors.Join(errProductGovernancePermissionConstruction, err)
			}
		}
		permissionRoute, err := api.NewLocalPermissionAPI(permissionService)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernancePermissionConstruction, err)
		}
		customerRuleService, err := app.NewLocalCustomerRuleService(
			store,
			func() time.Time { return time.Now().UTC() },
			func() string { return readModel.GlobalReadView().Version() },
			approvalPort.authority,
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceCustomerRuleConstruction, err)
		}
		customerRuleRoute, err := api.NewLocalCustomerRuleAPI(customerRuleService)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceCustomerRuleConstruction, err)
		}
		standingOrderService, err := app.NewLocalStandingOrderService(
			store,
			func() time.Time { return time.Now().UTC() },
			func() string { return readModel.GlobalReadView().Version() },
			rules.NewStandingOrderAuthority(
				store, approvalPort.authority,
				func() time.Time { return time.Now().UTC() },
			),
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceStandingOrderConstruction, err)
		}
		standingOrderRoute, err := api.NewLocalStandingOrderAPI(standingOrderService)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceStandingOrderConstruction, err)
		}
		decisionBackend, err := app.NewPreparedMissionDecisionBackend(prepared)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceDecisionConstruction, err)
		}
		fallbackDecisionWriter, err := state.NewLocalProductSetupWriter(store)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceDecisionConstruction, err)
		}
		fallbackDecisionPreparer, err := app.NewProjectionMissionFallbackDecisionPreparer(
			app.ProjectionMissionFallbackDecisionPreparerConfig{
				Backend: decisionBackend, Projection: readModel,
				Authority: fallbackDecisionWriter, ActorRef: "user:local-owner",
				Now: func() time.Time { return time.Now().UTC() },
			},
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceDecisionConstruction, err)
		}
		decisionService, err := app.NewLocalProductDecisionService(
			app.MissionDecisionConfig{Backend: decisionBackend},
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceDecisionConstruction, err)
		}
		decisionRouter, err := app.NewPreparedMissionExecutionDecisionRouter(
			decisionBackend, decisionService, prepared,
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceDecisionConstruction, err)
		}
		decisionRoute, err := api.NewLocalProductDecisionAPI(decisionService)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceDecisionConstruction, err)
		}
		providerPolicies, err := work.NewAuthority(
			store, func() time.Time { return time.Now().UTC() }, rand.Reader,
		)
		if err != nil {
			return productGovernanceRoutes{}, errors.Join(errProductGovernanceProviderPolicyConstruction, err)
		}
		return productGovernanceRoutes{
			permission: permissionRoute, customerRule: customerRuleRoute,
			standingOrder: standingOrderRoute, approvals: approvalPort,
			decision: decisionRoute, decisionCommands: decisionBackend,
			executionDecisions: decisionRouter, fallbackDecisions: fallbackDecisionPreparer,
			providerAccountPolicies:      providerPolicies,
			providerModelRateCards:       providerPolicies,
			remoteToolBackendEnrollments: providerPolicies,
			close:                        func() error { return nil },
		}, nil
	}
}

func newProductGovernanceRouteFactoryFromCore(
	core *productCoreRouteSlot,
	proposalStore toolproposal.Store,
) func(context.Context) (productGovernanceRoutes, error) {
	return func(ctx context.Context) (productGovernanceRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productGovernanceRoutes{}, err
		}
		return newProductGovernanceRouteFactory(
			resources.store, resources.readModel, proposalStore, resources.prepared,
		)(ctx)
	}
}

func (construction productCompatibilityConstruction) startGovernance(
	ctx context.Context, _ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.governanceSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.governanceFactory(ctx)
	if err != nil {
		return nil, err
	}
	if !routes.valid() {
		if routes.close != nil {
			_ = routes.close()
		}
		return nil, api.ErrInvalidLocalPermissionAPI
	}
	if err := construction.governanceSlot.Bind(routes); err != nil {
		_ = routes.close()
		return nil, err
	}
	if construction.readSlot != nil {
		readRoutes, readErr := construction.readFactory(ctx)
		if readErr != nil || !readRoutes.valid() {
			if readRoutes.close != nil {
				_ = readRoutes.close()
			}
			_ = construction.governanceSlot.Close(ctx)
			return nil, errors.Join(errProductReadRouteUnavailable, readErr)
		}
		if bindErr := construction.readSlot.Bind(readRoutes); bindErr != nil {
			_ = readRoutes.close()
			_ = construction.governanceSlot.Close(ctx)
			return nil, bindErr
		}
	}
	return composition.NewEffect(func(closeContext context.Context) error {
		var readErr error
		if construction.readSlot != nil {
			readErr = construction.readSlot.Close()
		}
		return errors.Join(
			readErr,
			construction.governanceSlot.Close(closeContext),
		)
	}), nil
}

func (construction productCompatibilityConstruction) governanceReady(
	context.Context, *composition.BundleContext,
) error {
	if !construction.valid() || construction.governanceSlot == nil ||
		!construction.governanceSlot.Ready() {
		return api.ErrInvalidLocalPermissionAPI
	}
	if construction.readSlot != nil && !construction.readSlot.Ready() {
		return errProductReadRouteUnavailable
	}
	return nil
}

var _ productPermissionRoute = (*api.LocalPermissionAPI)(nil)
var _ productCustomerRuleRoute = (*api.LocalCustomerRuleAPI)(nil)
var _ productStandingOrderRoute = (*api.LocalStandingOrderAPI)(nil)
var _ productGovernanceApprovalPort = (*permissionApprovalPort)(nil)
var _ productDecisionRoute = (*api.LocalProductDecisionAPI)(nil)
var _ api.MissionDecisionCommandSource = (*productGovernanceRouteSlot)(nil)
var _ app.MissionExecutionDecisionRouter = (*productGovernanceRouteSlot)(nil)
var _ app.MissionFallbackDecisionPreparer = (*productGovernanceRouteSlot)(nil)
var _ app.ProviderAccountPolicyAuthority = (*productGovernanceRouteSlot)(nil)
var _ app.ProviderModelRateCardAuthority = (*productGovernanceRouteSlot)(nil)
var _ app.RemoteToolBackendEnrollmentAuthority = (*productGovernanceRouteSlot)(nil)
var _ productDecisionRoute = (*productGovernanceRouteSlot)(nil)
var _ productPermissionRoute = (*productGovernanceRouteSlot)(nil)
var _ productCustomerRuleRoute = productCustomerRuleRouteProxy{}
var _ productStandingOrderRoute = productStandingOrderRouteProxy{}
