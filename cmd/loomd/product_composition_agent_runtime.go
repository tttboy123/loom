package main

import (
	"context"
	"errors"
	"reflect"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

type productMissionExecutionRoute interface {
	ExecuteMission(context.Context, app.MissionExecutionCommand) (api.MissionExecutionEnvelope, error)
}

type productHandoffRoute interface {
	ProposeSideTask(context.Context, app.SideTaskProposalRequest) (app.SideTaskProposalResult, error)
	CreateSideTask(context.Context, app.SideTaskCreateRequest) (app.SideTaskCreateResult, error)
	ReadSideTask(context.Context, app.SideTaskReadRequest) (app.SideTaskReadResult, error)
	DecideSideTask(context.Context, app.SideTaskDecisionRequest) (app.SideTaskDecisionResult, error)
}

type productAgentRuntimeRoutes struct {
	mission      productMissionExecutionRoute
	recovery     productAgentAttemptRecoveryRoute
	agentInput   productAgentInputRoute
	handoff      productHandoffRoute
	materializer productSavedTeamMaterializer
	close        func() error
}

func (routes productAgentRuntimeRoutes) valid() bool {
	return !nilProductAgentRuntimePort(routes.mission) &&
		!nilProductAgentRuntimePort(routes.handoff) &&
		!nilProductAgentRuntimePort(routes.materializer) && routes.close != nil &&
		(routes.agentInput == nil) == (routes.recovery == nil)
}

type productAgentRuntimeRouteSlot struct {
	mu     sync.RWMutex
	routes productAgentRuntimeRoutes
	bound  bool
	closed bool
}

func (slot *productAgentRuntimeRouteSlot) Bind(routes productAgentRuntimeRoutes) error {
	if slot == nil || !routes.valid() {
		return api.ErrInvalidLocalProductExecutionAPI
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

func (slot *productAgentRuntimeRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productAgentRuntimeRouteSlot) ExecuteMission(
	ctx context.Context,
	command app.MissionExecutionCommand,
) (api.MissionExecutionEnvelope, error) {
	if slot == nil {
		return api.MissionExecutionEnvelope{}, api.ErrInvalidLocalProductExecutionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.MissionExecutionEnvelope{}, api.ErrInvalidLocalProductExecutionAPI
	}
	return slot.routes.mission.ExecuteMission(ctx, command)
}

func (slot *productAgentRuntimeRouteSlot) AdmitAgentInput(
	ctx context.Context,
	request productAgentInputRequest,
) (productAgentInputReceipt, error) {
	if slot == nil {
		return productAgentInputReceipt{}, errProductInvalidAgentInput
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() || slot.routes.agentInput == nil {
		return productAgentInputReceipt{}, errProductInvalidAgentInput
	}
	return slot.routes.agentInput.AdmitAgentInput(ctx, request)
}

func (slot *productAgentRuntimeRouteSlot) RecoverAgentAttempt(
	ctx context.Context,
	request productAgentAttemptRecoveryRequest,
) (productAgentAttemptRecoveryResponse, error) {
	if slot == nil {
		return productAgentAttemptRecoveryResponse{}, errProductInvalidAttemptRecoveryRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() || slot.routes.recovery == nil {
		return productAgentAttemptRecoveryResponse{}, errProductInvalidAttemptRecoveryRequest
	}
	return slot.routes.recovery.RecoverAgentAttempt(ctx, request)
}

func (slot *productAgentRuntimeRouteSlot) ProposeSideTask(
	ctx context.Context, request app.SideTaskProposalRequest,
) (app.SideTaskProposalResult, error) {
	if slot == nil {
		return app.SideTaskProposalResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.SideTaskProposalResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	return slot.routes.handoff.ProposeSideTask(ctx, request)
}

func (slot *productAgentRuntimeRouteSlot) CreateSideTask(
	ctx context.Context, request app.SideTaskCreateRequest,
) (app.SideTaskCreateResult, error) {
	if slot == nil {
		return app.SideTaskCreateResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.SideTaskCreateResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	return slot.routes.handoff.CreateSideTask(ctx, request)
}

func (slot *productAgentRuntimeRouteSlot) ReadSideTask(
	ctx context.Context, request app.SideTaskReadRequest,
) (app.SideTaskReadResult, error) {
	if slot == nil {
		return app.SideTaskReadResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.SideTaskReadResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	return slot.routes.handoff.ReadSideTask(ctx, request)
}

func (slot *productAgentRuntimeRouteSlot) DecideSideTask(
	ctx context.Context, request app.SideTaskDecisionRequest,
) (app.SideTaskDecisionResult, error) {
	if slot == nil {
		return app.SideTaskDecisionResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.SideTaskDecisionResult{}, api.ErrInvalidLocalProductHandoffAPI
	}
	return slot.routes.handoff.DecideSideTask(ctx, request)
}

func (slot *productAgentRuntimeRouteSlot) MaterializeConfirmedTeam(
	ctx context.Context,
	confirmation app.BuilderConfirmation,
) (app.BuilderConfirmation, error) {
	if slot == nil {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	return slot.routes.materializer.MaterializeConfirmedTeam(ctx, confirmation)
}

func (slot *productAgentRuntimeRouteSlot) Close(context.Context) error {
	if slot == nil {
		return composition.ErrInvalidComposition
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	if slot.closed {
		return nil
	}
	slot.closed = true
	closeRuntime := slot.routes.close
	slot.routes = productAgentRuntimeRoutes{}
	if closeRuntime == nil {
		return nil
	}
	return closeRuntime()
}

func nilProductAgentRuntimePort(value any) bool {
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

func newProductAgentRuntimeFactory(
	store *journal.Store,
	readModel *projection.Projection,
	readService productReadRoute,
	statePath string,
	config productMissionExecutionRuntimeConfig,
	assetSlot *productAssetRouteSlot,
	workSlot *productWorkRouteSlot,
	conversationSlot *productConversationRouteSlot,
) func(context.Context) (productAgentRuntimeRoutes, error) {
	return func(ctx context.Context) (productAgentRuntimeRoutes, error) {
		materializer, err := assetSlot.TeamAssetMaterializer()
		if err != nil {
			return productAgentRuntimeRoutes{}, err
		}
		toolExecution, err := workSlot.ToolExecution()
		if err != nil {
			return productAgentRuntimeRoutes{}, err
		}
		config.ToolExecution = toolExecution
		if config.LocalModelCatalog != nil {
			localModel, modelErr := conversationSlot.LocalModelRuntime()
			if modelErr != nil {
				return productAgentRuntimeRoutes{}, modelErr
			}
			config.LocalModelRuntime = localModel
		}
		mission, closer, err := buildProductMissionExecutionAPI(
			ctx, store, readModel, readService, statePath, config,
			productMissionAssetExecutionConfig{Materializer: materializer},
		)
		if err != nil {
			return productAgentRuntimeRoutes{}, err
		}
		bundle, ok := closer.(*productMissionExecutionBundle)
		if !ok || bundle.handoff == nil ||
			(config.AgentInboxStore != nil &&
				(bundle.agentInput == nil || bundle.agentRecovery == nil)) {
			_ = closer.Close()
			return productAgentRuntimeRoutes{}, api.ErrInvalidLocalProductExecutionAPI
		}
		return productAgentRuntimeRoutes{
			mission: mission, recovery: bundle.agentRecovery,
			agentInput: bundle.agentInput, handoff: bundle.handoff,
			materializer: &productSavedTeamMaterialization{
				store: store, projection: readModel, now: config.Now,
			},
			close: closer.Close,
		}, nil
	}
}

func newProductAgentRuntimeFactoryFromCore(
	core *productCoreRouteSlot,
	readService productReadRoute,
	statePath string,
	config productMissionExecutionRuntimeConfig,
	assetSlot *productAssetRouteSlot,
	workSlot *productWorkRouteSlot,
	conversationSlot *productConversationRouteSlot,
) func(context.Context) (productAgentRuntimeRoutes, error) {
	return func(ctx context.Context) (productAgentRuntimeRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productAgentRuntimeRoutes{}, err
		}
		return newProductAgentRuntimeFactory(
			resources.store, resources.readModel, readService, statePath, config,
			assetSlot, workSlot, conversationSlot,
		)(ctx)
	}
}

func (construction productCompatibilityConstruction) startAgentRuntime(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.agentRuntimeSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.agentRuntimeFactory(ctx)
	if err != nil || !routes.valid() {
		if routes.close != nil {
			_ = routes.close()
		}
		return nil, errors.Join(api.ErrInvalidLocalProductExecutionAPI, err)
	}
	if err := construction.agentRuntimeSlot.Bind(routes); err != nil {
		if routes.close != nil {
			_ = routes.close()
		}
		return nil, err
	}
	return composition.NewEffect(construction.agentRuntimeSlot.Close), nil
}

func (construction productCompatibilityConstruction) agentRuntimeReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.agentRuntimeSlot == nil ||
		!construction.agentRuntimeSlot.Ready() {
		return api.ErrInvalidLocalProductExecutionAPI
	}
	return nil
}

var _ productMissionExecutionRoute = (*api.LocalProductExecutionAPI)(nil)
var _ productHandoffRoute = (*api.LocalProductHandoffAPI)(nil)
var _ productMissionExecutionRoute = (*productAgentRuntimeRouteSlot)(nil)
var _ productAgentAttemptRecoveryRoute = (*productAgentRuntimeRouteSlot)(nil)
var _ productAgentInputRoute = (*productAgentRuntimeRouteSlot)(nil)
var _ productHandoffRoute = (*productAgentRuntimeRouteSlot)(nil)
var _ productSavedTeamMaterializer = (*productAgentRuntimeRouteSlot)(nil)
