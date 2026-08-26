package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/observability"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/production"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/sandbox"
	"loom-pi-rebuild/internal/work"
)

var (
	errProductWorkQueueConstruction       = errors.New("product Work Queue construction failed")
	errProductWorkWorkersConstruction     = errors.New("product Work Workers construction failed")
	errProductWorkIntegrationConstruction = errors.New("product Work Integration construction failed")
	errProductWorkExecutionConstruction   = errors.New("product Work Execution construction failed")
	errProductWorkExecutionRecovery       = errors.New("product Work Execution recovery failed")
	errProductWorkProductionConstruction  = errors.New("product Work Production construction failed")
)

func productCompatibilityBuildFailureReason(err error) string {
	var buildFailure *daemonBuildFailure
	if errors.As(err, &buildFailure) &&
		validDaemonBuildFailureReason(buildFailure.reason) {
		return buildFailure.reason
	}
	switch {
	case errors.Is(err, errProductObservabilityUnavailable):
		return "build_diagnostics"
	case errors.Is(err, errProductConversationNativeAuthConstruction):
		return "build_setup_native_auth"
	case errors.Is(err, errProductConversationProviderConstruction):
		return "build_setup_provider"
	case errors.Is(err, errProductConversationRouteUnavailable):
		return "build_setup_provider"
	case errors.Is(err, errProductVaultConstruction):
		return "build_setup_credential"
	case errors.Is(err, errProductSetupRouteUnavailable):
		return "build_setup_runtime"
	case errors.Is(err, errProductReadRouteUnavailable):
		return "build_state"
	case errors.Is(err, api.ErrInvalidLocalProductExecutionAPI):
		return "build_execution"
	case errors.Is(err, api.ErrInvalidLocalProductAssetAPI):
		return "build_assets"
	case errors.Is(err, errProductWorkWorkersConstruction):
		return "build_workers"
	case errors.Is(err, errProductWorkIntegrationConstruction):
		return "build_integration"
	case errors.Is(err, errProductWorkExecutionRecovery):
		return "build_execution_recovery"
	case errors.Is(err, errProductWorkExecutionConstruction):
		return "build_execution"
	case errors.Is(err, errProductWorkProductionConstruction),
		errors.Is(err, api.ErrInvalidLocalProductionAPI):
		return "build_production"
	case errors.Is(err, errProductGovernancePermissionConstruction),
		errors.Is(err, api.ErrInvalidLocalPermissionAPI):
		return "build_permissions"
	case errors.Is(err, errProductGovernanceCustomerRuleConstruction),
		errors.Is(err, api.ErrInvalidLocalCustomerRuleAPI):
		return "build_customer_rule"
	case errors.Is(err, errProductGovernanceStandingOrderConstruction),
		errors.Is(err, api.ErrInvalidLocalStandingOrderAPI):
		return "build_standing_order"
	case errors.Is(err, errProductGovernanceDecisionConstruction),
		errors.Is(err, api.ErrInvalidLocalProductDecisionAPI):
		return "build_decision"
	case errors.Is(err, errProductGovernanceProviderPolicyConstruction):
		return "build_setup_policy"
	case errors.Is(err, errProductWorkQueueConstruction),
		errors.Is(err, api.ErrInvalidLocalQueueAPI):
		return "build_queue"
	default:
		return "build_ipc"
	}
}

type productQueueRoute interface {
	QueueSnapshot(context.Context, api.QueueSnapshotRequest) (api.QueueSnapshot, error)
	QueueCommand(context.Context, api.QueueCommandRequest) (api.QueueCommandResult, error)
}

type productWorkersRoute interface {
	WorkersSnapshot(context.Context, app.WorkersSnapshotRequest) (app.WorkersSnapshot, error)
	WorkersCommand(context.Context, app.WorkersCommandRequest) (app.WorkersCommandResult, error)
}

type productIntegrationRoute interface {
	Snapshot(context.Context) (app.IntegrationSnapshot, error)
	Command(context.Context, app.IntegrationCommandRequest) (app.IntegrationCommandResult, error)
}

type productExecutionRoute interface {
	ExecutionSnapshot(context.Context, app.ExecutionSnapshotRequest) (app.ExecutionSnapshot, error)
	ExecutionCommand(context.Context, app.ExecutionCommandRequest) (app.ExecutionCommandResult, error)
}

type productProductionRoute interface {
	ProductionSnapshot(context.Context, app.ProductionSnapshotRequest) (api.ProductionSnapshot, error)
	ProductionCommand(context.Context, app.ProductionCommandRequest) (app.ProductionCommandResult, error)
	Degraded(context.Context) bool
}

type productWorkRoutes struct {
	queue         productQueueRoute
	workers       productWorkersRoute
	integration   productIntegrationRoute
	execution     productExecutionRoute
	toolExecution productToolExecutionPort
	toolRecovery  productToolRecoveryRoute
	production    productProductionRoute
	close         func() error
}

func (routes productWorkRoutes) valid() bool {
	return !nilProductAssetPort(routes.queue) &&
		!nilProductAssetPort(routes.workers) &&
		!nilProductAssetPort(routes.integration) &&
		!nilProductAssetPort(routes.execution) &&
		!nilProductAssetPort(routes.toolExecution) &&
		!nilProductAssetPort(routes.toolRecovery) &&
		!nilProductAssetPort(routes.production) && routes.close != nil
}

type productWorkRouteSlot struct {
	mu     sync.RWMutex
	routes productWorkRoutes
	bound  bool
	closed bool
}

func (slot *productWorkRouteSlot) Bind(routes productWorkRoutes) error {
	if slot == nil || !routes.valid() {
		return api.ErrInvalidLocalQueueAPI
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

func (slot *productWorkRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productWorkRouteSlot) QueueSnapshot(
	ctx context.Context,
	request api.QueueSnapshotRequest,
) (api.QueueSnapshot, error) {
	if slot == nil {
		return api.QueueSnapshot{}, api.ErrInvalidLocalQueueAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.QueueSnapshot{}, api.ErrInvalidLocalQueueAPI
	}
	return slot.routes.queue.QueueSnapshot(ctx, request)
}

func (slot *productWorkRouteSlot) QueueCommand(
	ctx context.Context,
	request api.QueueCommandRequest,
) (api.QueueCommandResult, error) {
	if slot == nil {
		return api.QueueCommandResult{}, api.ErrInvalidLocalQueueAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.QueueCommandResult{}, api.ErrInvalidLocalQueueAPI
	}
	return slot.routes.queue.QueueCommand(ctx, request)
}

func (slot *productWorkRouteSlot) WorkersSnapshot(
	ctx context.Context,
	request app.WorkersSnapshotRequest,
) (app.WorkersSnapshot, error) {
	if slot == nil {
		return app.WorkersSnapshot{}, app.ErrInvalidWorkerRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.WorkersSnapshot{}, app.ErrInvalidWorkerRequest
	}
	return slot.routes.workers.WorkersSnapshot(ctx, request)
}

func (slot *productWorkRouteSlot) WorkersCommand(
	ctx context.Context,
	request app.WorkersCommandRequest,
) (app.WorkersCommandResult, error) {
	if slot == nil {
		return app.WorkersCommandResult{}, app.ErrInvalidWorkerRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.WorkersCommandResult{}, app.ErrInvalidWorkerRequest
	}
	return slot.routes.workers.WorkersCommand(ctx, request)
}

func (slot *productWorkRouteSlot) Snapshot(
	ctx context.Context,
) (app.IntegrationSnapshot, error) {
	if slot == nil {
		return app.IntegrationSnapshot{}, app.ErrInvalidIntegrationRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.IntegrationSnapshot{}, app.ErrInvalidIntegrationRequest
	}
	return slot.routes.integration.Snapshot(ctx)
}

func (slot *productWorkRouteSlot) Command(
	ctx context.Context,
	request app.IntegrationCommandRequest,
) (app.IntegrationCommandResult, error) {
	if slot == nil {
		return app.IntegrationCommandResult{}, app.ErrInvalidIntegrationRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.IntegrationCommandResult{}, app.ErrInvalidIntegrationRequest
	}
	return slot.routes.integration.Command(ctx, request)
}

func (slot *productWorkRouteSlot) ExecutionSnapshot(
	ctx context.Context,
	request app.ExecutionSnapshotRequest,
) (app.ExecutionSnapshot, error) {
	if slot == nil {
		return app.ExecutionSnapshot{}, api.ErrInvalidLocalExecutionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ExecutionSnapshot{}, api.ErrInvalidLocalExecutionAPI
	}
	return slot.routes.execution.ExecutionSnapshot(ctx, request)
}

func (slot *productWorkRouteSlot) ExecutionCommand(
	ctx context.Context,
	request app.ExecutionCommandRequest,
) (app.ExecutionCommandResult, error) {
	if slot == nil {
		return app.ExecutionCommandResult{}, api.ErrInvalidLocalExecutionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ExecutionCommandResult{}, api.ErrInvalidLocalExecutionAPI
	}
	return slot.routes.execution.ExecutionCommand(ctx, request)
}

func (slot *productWorkRouteSlot) ToolExecution() (productToolExecutionPort, error) {
	if slot == nil {
		return nil, api.ErrInvalidLocalExecutionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return nil, api.ErrInvalidLocalExecutionAPI
	}
	return slot.routes.toolExecution, nil
}

func (slot *productWorkRouteSlot) RecoverToolCall(
	ctx context.Context,
	request productToolRecoveryRequest,
) (productToolRecoveryResponse, error) {
	if slot == nil {
		return productToolRecoveryResponse{}, errProductInvalidToolRecoveryRequest
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return productToolRecoveryResponse{}, errProductInvalidToolRecoveryRequest
	}
	return slot.routes.toolRecovery.RecoverToolCall(ctx, request)
}

func (slot *productWorkRouteSlot) ProductionSnapshot(
	ctx context.Context,
	request app.ProductionSnapshotRequest,
) (api.ProductionSnapshot, error) {
	if slot == nil {
		return api.ProductionSnapshot{}, api.ErrInvalidLocalProductionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.ProductionSnapshot{}, api.ErrInvalidLocalProductionAPI
	}
	return slot.routes.production.ProductionSnapshot(ctx, request)
}

func (slot *productWorkRouteSlot) ProductionCommand(
	ctx context.Context,
	request app.ProductionCommandRequest,
) (app.ProductionCommandResult, error) {
	if slot == nil {
		return app.ProductionCommandResult{}, api.ErrInvalidLocalProductionAPI
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return app.ProductionCommandResult{}, api.ErrInvalidLocalProductionAPI
	}
	return slot.routes.production.ProductionCommand(ctx, request)
}

func (slot *productWorkRouteSlot) Degraded(ctx context.Context) bool {
	if slot == nil {
		return true
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return true
	}
	return slot.routes.production.Degraded(ctx)
}

func (slot *productWorkRouteSlot) Close(context.Context) error {
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
	slot.routes = productWorkRoutes{}
	if closeRoutes == nil {
		return nil
	}
	return closeRoutes()
}

func newProductWorkRouteFactory(
	store *journal.Store,
	readModel *projection.Projection,
	statePath string,
	governance *productGovernanceRouteSlot,
	remoteConfig *productRemoteToolBrokerConfig,
) func(context.Context) (productWorkRoutes, error) {
	return func(ctx context.Context) (productWorkRoutes, error) {
		if ctx == nil || store == nil || readModel == nil ||
			!filepath.IsAbs(statePath) || governance == nil {
			return productWorkRoutes{}, api.ErrInvalidLocalQueueAPI
		}
		approvals, err := governance.Approvals()
		if err != nil {
			return productWorkRoutes{}, err
		}
		service, err := app.NewLocalQueueService(
			store,
			func() time.Time { return time.Now().UTC() },
			func() string { return readModel.GlobalReadView().Version() },
		)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkQueueConstruction, err)
		}
		queue, err := api.NewLocalQueueAPI(service)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkQueueConstruction, err)
		}
		workerExecution, err := work.NewWorkerExecutionService(
			store,
			func() time.Time { return time.Now().UTC() },
			10*time.Second,
		)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkWorkersConstruction, err)
		}
		workerService, err := app.NewLocalWorkersService(
			store,
			func() time.Time { return time.Now().UTC() },
			10*time.Second,
			workerExecution,
		)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkWorkersConstruction, err)
		}
		workers, err := api.NewLocalWorkersAPI(workerService)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkWorkersConstruction, err)
		}
		integrationService, err := integration.NewIntegrationService(
			store,
			func() time.Time { return time.Now().UTC() },
		)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkIntegrationConstruction, err)
		}
		observabilityService, err := observability.NewObservabilityService(
			store,
			func() time.Time { return time.Now().UTC() },
		)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkIntegrationConstruction, err)
		}
		localIntegrationService, err := app.NewLocalIntegrationService(
			store, integrationService, observabilityService,
		)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkIntegrationConstruction, err)
		}
		integrationRoute, err := api.NewLocalIntegrationAPI(localIntegrationService)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkIntegrationConstruction, err)
		}
		executionEvidenceRoot := filepath.Join(filepath.Dir(statePath), "execution-evidence")
		if err := ensureProductExecutionDirectory(executionEvidenceRoot); err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkExecutionConstruction, err)
		}
		executionEvidence, err := evidence.NewStore(executionEvidenceRoot)
		if err != nil {
			return productWorkRoutes{}, errors.Join(errProductWorkExecutionConstruction, err)
		}
		var remoteEffect composition.Effect
		closeOwned := func() error {
			var remoteErr error
			if remoteEffect != nil {
				remoteErr = remoteEffect.Close(context.Background())
			}
			return errors.Join(remoteErr, executionEvidence.Close())
		}
		closeOnFailure := func(category, cause error) (productWorkRoutes, error) {
			return productWorkRoutes{}, errors.Join(category, cause, closeOwned())
		}
		remote, effect, err := newProductRemoteToolBroker(ctx, remoteConfig)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionConstruction, err)
		}
		remoteEffect = effect
		adapterRemote := remote
		enrollmentRemote, enrollmentErr := newProductRemoteToolExecutorsFromEnrollments(
			func() projection.GlobalReadView { return readModel.GlobalReadView() },
			productRemoteToolEnrollmentDeps(remoteConfig),
		)
		if enrollmentErr != nil {
			return closeOnFailure(errProductWorkExecutionConstruction, enrollmentErr)
		}
		if enrollmentRemote != nil {
			enrollmentEffect := composition.NewEffect(func(context.Context) error {
				if closer, ok := enrollmentRemote.(interface{ Close() error }); ok {
					return closer.Close()
				}
				return nil
			})
			switch {
			case adapterRemote == nil:
				adapterRemote = enrollmentRemote
				remoteEffect = enrollmentEffect
			default:
				combined, combineErr := newProductCompositeRemoteToolExecutor(
					[]execution.RemoteToolExecutor{adapterRemote, enrollmentRemote},
				)
				if combineErr != nil {
					_ = enrollmentEffect.Close(context.Background())
					return closeOnFailure(errProductWorkExecutionConstruction, combineErr)
				}
				adapterRemote = combined
				combinedEffect := composition.NewEffect(func(context.Context) error {
					return combined.Close()
				})
				previousRemoteEffect := remoteEffect
				remoteEffect = composition.NewEffect(func(closeCtx context.Context) error {
					var closeErr error
					if combinedEffect != nil {
						closeErr = errors.Join(closeErr, combinedEffect.Close(closeCtx))
					}
					if enrollmentEffect != nil {
						closeErr = errors.Join(closeErr, enrollmentEffect.Close(closeCtx))
					}
					if previousRemoteEffect != nil {
						closeErr = errors.Join(closeErr, previousRemoteEffect.Close(closeCtx))
					}
					return closeErr
				})
			}
		}
		decisionRecorder, err := newExecutionDecisionRecorder(
			store,
			func() time.Time { return time.Now().UTC() },
		)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionConstruction, err)
		}
		var sandboxGate execution.SandboxGate
		if sandboxBackendName := os.Getenv("LOOM_SANDBOX_BACKEND"); sandboxBackendName != "" {
			if sandboxBackendName != "loopback" {
				return closeOnFailure(
					errProductWorkExecutionConstruction,
					errors.New("unsupported sandbox backend: "+sandboxBackendName),
				)
			}
			loopback, loopbackErr := sandbox.NewLoopbackBackend("", 0)
			if loopbackErr != nil {
				return closeOnFailure(errProductWorkExecutionConstruction, loopbackErr)
			}
			sandboxRequired := os.Getenv("LOOM_SANDBOX_REQUIRED") == "1"
			sandboxGate = execution.NewBackendGate(
				loopback,
				func(context.Context, string) (execution.SandboxPolicy, error) {
					return execution.SandboxPolicy{
						Required: sandboxRequired, Backend: "loopback",
					}, nil
				},
			)
		}
		executionAdapter, err := execution.NewAdapter(
			store,
			executionEvidence,
			execution.NewSandboxExecutor(),
			&productWorktreeResolver{store: store},
			approvals,
			decisionRecorder,
			func() time.Time { return time.Now().UTC() },
		)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionConstruction, err)
		}
		if sandboxGate != nil {
			executionAdapter = executionAdapter.WithSandboxGate(sandboxGate)
		}
		if adapterRemote != nil {
			executionAdapter = executionAdapter.WithRemoteToolExecutor(adapterRemote)
		}
		if err := executionAdapter.ReplayPending(ctx); err != nil {
			return closeOnFailure(errProductWorkExecutionRecovery, err)
		}
		toolRecoveryAuthority, err := execution.NewToolRecoveryAuthority(
			store, nil, nil, func() time.Time { return time.Now().UTC() },
		)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionRecovery, err)
		}
		toolRecoveryRoute, err := newProductToolRecoveryRoute(toolRecoveryAuthority)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionRecovery, err)
		}
		executionService, err := app.NewLocalExecutionService(
			store,
			func() time.Time { return time.Now().UTC() },
			func() string { return readModel.GlobalReadView().Version() },
			executionAdapter,
		)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionConstruction, err)
		}
		executionRoute, err := api.NewLocalExecutionAPI(executionService)
		if err != nil {
			return closeOnFailure(errProductWorkExecutionConstruction, err)
		}
		productionPaths, err := productWorkProductionPaths()
		if err != nil {
			return closeOnFailure(errProductWorkProductionConstruction, err)
		}
		productionCore, err := production.NewService(
			store,
			productionPaths,
			func() time.Time { return time.Now().UTC() },
			func(ctx context.Context) (bool, error) {
				events, readErr := store.ReadAll(ctx)
				if readErr != nil {
					return false, readErr
				}
				permissionProjection, replayErr := permissions.Replay(events)
				if replayErr != nil {
					return false, replayErr
				}
				return permissionProjection.AdminLock, nil
			},
		)
		if err != nil {
			return closeOnFailure(errProductWorkProductionConstruction, err)
		}
		productionService, err := app.NewLocalProductionService(
			store,
			func() time.Time { return time.Now().UTC() },
			func() string { return readModel.GlobalReadView().Version() },
			productionCore,
		)
		if err != nil {
			return closeOnFailure(errProductWorkProductionConstruction, err)
		}
		productionRoute, err := api.NewLocalProductionAPI(productionService)
		if err != nil {
			return closeOnFailure(errProductWorkProductionConstruction, err)
		}
		return productWorkRoutes{
			queue: queue, workers: workers, integration: integrationRoute,
			execution: executionRoute, toolExecution: executionAdapter,
			toolRecovery: toolRecoveryRoute,
			production:   productionRoute,
			close:        closeOwned,
		}, nil
	}
}

func newProductWorkRouteFactoryFromCore(
	core *productCoreRouteSlot,
	statePath string,
	governance *productGovernanceRouteSlot,
	remoteConfig *productRemoteToolBrokerConfig,
) func(context.Context) (productWorkRoutes, error) {
	return func(ctx context.Context) (productWorkRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productWorkRoutes{}, err
		}
		return newProductWorkRouteFactory(
			resources.store, resources.readModel, statePath, governance, remoteConfig,
		)(ctx)
	}
}

func productWorkProductionPaths() (production.Paths, error) {
	appSupport := ""
	launchAgents := ""
	if sandboxRoot := os.Getenv("LOOM_PRODUCTION_SANDBOX_ROOT"); sandboxRoot != "" {
		if !filepath.IsAbs(sandboxRoot) {
			return production.Paths{}, errors.New("invalid sandbox root")
		}
		appSupport = filepath.Join(sandboxRoot, "Application Support", "Loom")
		launchAgents = filepath.Join(sandboxRoot, "LaunchAgents")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return production.Paths{}, err
		}
		appSupport = filepath.Join(home, "Library", "Application Support", "Loom")
		launchAgents = filepath.Join(home, "Library", "LaunchAgents")
	}
	daemonPath, err := os.Executable()
	if err != nil {
		return production.Paths{}, err
	}
	return production.Paths{
		AppSupport: appSupport, LaunchAgents: launchAgents, DaemonPath: daemonPath,
	}, nil
}

func (construction productCompatibilityConstruction) startWork(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.workSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.workFactory(ctx)
	if err != nil {
		return nil, err
	}
	if !routes.valid() {
		if routes.close != nil {
			_ = routes.close()
		}
		return nil, errors.Join(
			api.ErrInvalidLocalQueueAPI,
			app.ErrInvalidWorkerRequest,
			app.ErrInvalidIntegrationRequest,
		)
	}
	if err := construction.workSlot.Bind(routes); err != nil {
		_ = routes.close()
		return nil, err
	}
	return composition.NewEffect(construction.workSlot.Close), nil
}

func (construction productCompatibilityConstruction) workReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.workSlot == nil ||
		!construction.workSlot.Ready() {
		return api.ErrInvalidLocalQueueAPI
	}
	return nil
}

var _ productQueueRoute = (*api.LocalQueueAPI)(nil)
var _ productWorkersRoute = (*api.LocalWorkersAPI)(nil)
var _ productIntegrationRoute = (*api.LocalIntegrationAPI)(nil)
var _ productExecutionRoute = (*api.LocalExecutionAPI)(nil)
var _ productProductionRoute = (*api.LocalProductionAPI)(nil)
var _ productToolExecutionPort = (*execution.Adapter)(nil)
var _ productQueueRoute = (*productWorkRouteSlot)(nil)
var _ productWorkersRoute = (*productWorkRouteSlot)(nil)
var _ productIntegrationRoute = (*productWorkRouteSlot)(nil)
var _ productExecutionRoute = (*productWorkRouteSlot)(nil)
var _ productProductionRoute = (*productWorkRouteSlot)(nil)
