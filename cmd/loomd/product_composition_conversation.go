package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
	"loom-pi-rebuild/internal/work"
)

var (
	errProductConversationRouteUnavailable       = errors.New("product Conversation route unavailable")
	errProductConversationNativeAuthConstruction = errors.New("product Conversation native auth construction failed")
	errProductConversationProviderConstruction   = errors.New("product Conversation Provider construction failed")
)

type productConversationRoutes struct {
	route      api.LocalProductChatSource
	localModel productLocalModelRuntime
	close      func() error
}

func (slot *productConversationRouteSlot) LocalModelRuntime() (
	productLocalModelRuntime,
	error,
) {
	if slot == nil {
		return nil, errProductConversationRouteUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() ||
		nilProductAssetPort(slot.routes.localModel) {
		return nil, errProductConversationRouteUnavailable
	}
	return slot.routes.localModel, nil
}

func (routes productConversationRoutes) valid() bool {
	return !nilProductAssetPort(routes.route) && routes.close != nil
}

type productConversationRouteSlot struct {
	mu     sync.RWMutex
	routes productConversationRoutes
	bound  bool
	closed bool
}

func (slot *productConversationRouteSlot) Bind(
	routes productConversationRoutes,
) error {
	if slot == nil || !routes.valid() {
		return api.ErrLocalProductChatUnavailable
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

func (slot *productConversationRouteSlot) Ready() bool {
	if slot == nil {
		return false
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	return slot.bound && !slot.closed && slot.routes.valid()
}

func (slot *productConversationRouteSlot) ChatThread(
	ctx context.Context,
	threadID string,
) (api.LocalProductChatThread, error) {
	if slot == nil {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	return slot.routes.route.ChatThread(ctx, threadID)
}

func (slot *productConversationRouteSlot) InspectChatContextDisclosure(
	ctx context.Context,
	request api.LocalProductChatContextDisclosureRequest,
) (api.LocalProductChatContextDisclosure, error) {
	if slot == nil {
		return api.LocalProductChatContextDisclosure{}, api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductChatContextDisclosure{}, api.ErrLocalProductChatUnavailable
	}
	inspector, ok := slot.routes.route.(api.LocalProductChatContextDisclosureInspector)
	if !ok || inspector == nil {
		return api.LocalProductChatContextDisclosure{}, api.ErrLocalProductChatUnavailable
	}
	return inspector.InspectChatContextDisclosure(ctx, request)
}

func (slot *productConversationRouteSlot) SendMessage(
	ctx context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	if slot == nil {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.LocalProductChatThread{}, api.ErrLocalProductChatUnavailable
	}
	return slot.routes.route.SendMessage(ctx, request)
}

func (slot *productConversationRouteSlot) CancelChatResponse(
	ctx context.Context,
	request api.LocalProductChatResponseCancelRequest,
) error {
	if slot == nil {
		return api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.ErrLocalProductChatUnavailable
	}
	canceller, ok := slot.routes.route.(api.LocalProductConversationResponseCanceller)
	if !ok || canceller == nil {
		return api.ErrLocalProductChatUnavailable
	}
	return canceller.CancelChatResponse(ctx, request)
}

func (slot *productConversationRouteSlot) DeleteThread(
	ctx context.Context,
	threadID string,
) error {
	if slot == nil {
		return api.ErrLocalProductChatUnavailable
	}
	slot.mu.RLock()
	defer slot.mu.RUnlock()
	if slot.closed || !slot.bound || !slot.routes.valid() {
		return api.ErrLocalProductChatUnavailable
	}
	return slot.routes.route.DeleteThread(ctx, threadID)
}

func (slot *productConversationRouteSlot) Close(context.Context) error {
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
	slot.routes = productConversationRoutes{}
	if closeRoute == nil {
		return nil
	}
	return closeRoute()
}

func newProductConversationRouteFactory(
	route api.LocalProductChatSource,
) func(context.Context) (productConversationRoutes, error) {
	return func(ctx context.Context) (productConversationRoutes, error) {
		if ctx == nil || ctx.Err() != nil || nilProductAssetPort(route) {
			return productConversationRoutes{}, errProductConversationRouteUnavailable
		}
		return productConversationRoutes{route: route, close: func() error { return nil }}, nil
	}
}

func newProductConversationConstructionFactory(
	statePath string,
	setup productSetupRuntimeConfig,
	readModel *projection.Projection,
	vault *productVaultRouteSlot,
	diagnostics productOperationalDiagnosticSink,
) func(context.Context) (productConversationRoutes, error) {
	return func(ctx context.Context) (_ productConversationRoutes, resultErr error) {
		if ctx == nil || ctx.Err() != nil || readModel == nil || nilProductAssetPort(diagnostics) ||
			!filepath.IsAbs(statePath) || filepath.Clean(statePath) != statePath {
			return productConversationRoutes{}, errProductConversationRouteUnavailable
		}
		conversationResponder := setup.ConversationResponder
		var leases productCredentialLeaseAccess
		var localModel productLocalModelRuntime
		var harnessGateway *productHarnessGatewayConversationResponder
		defer func() {
			if resultErr != nil && harnessGateway != nil {
				closeContext, cancel := context.WithTimeout(
					context.Background(), 5*time.Second,
				)
				resultErr = errors.Join(resultErr, harnessGateway.Close(closeContext))
				cancel()
			}
			if resultErr != nil && !nilProductAssetPort(localModel) {
				resultErr = errors.Join(resultErr, localModel.Close())
			}
		}()
		if setup.Execution != nil && setup.Execution.LocalModelCatalog != nil {
			localModel = setup.Execution.LocalModelRuntime
			if nilProductAssetPort(localModel) {
				var err error
				localModel, err = newProductSharedLocalModel(
					*setup.Execution.LocalModelCatalog,
				)
				if err != nil {
					return productConversationRoutes{}, errors.Join(
						errProductConversationProviderConstruction, err,
					)
				}
			}
			if conversationResponder == nil {
				conversationResponder = &productPiConversationResponder{
					runtime:            localModel,
					runtimeSearchPaths: append([]string(nil), setup.Execution.RuntimeSearchPaths...),
					runtimeInstanceID:  setup.Execution.RuntimeInstanceID,
					privateRoot: filepath.Join(
						filepath.Dir(statePath), "conversation-runtime",
					),
					now: setup.Execution.Now, random: setup.Execution.Random,
				}
			}
		}
		var openCodeClient *provider.OpenCodeConversationClient
		if setup.OpenCodeExecutable != "" {
			resolved, err := provider.ResolveOpenCodeNativeExecutable(
				setup.OpenCodeExecutable,
			)
			if err != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, err,
				)
			}
			homePath, err := os.UserHomeDir()
			if err != nil || !filepath.IsAbs(homePath) {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction,
					provider.ErrInvalidOpenCodeConversationConfig,
				)
			}
			modelID := provider.OpenCodeConversationDefaultModel
			if runtime, found := readModel.GlobalReadView().RuntimeInstance(
				productOpenCodeRuntimeInstanceID,
			); found {
				if selected, available := provider.SelectOpenCodeNativeModel(
					runtime.ModelIDs,
				); available {
					modelID = selected
				}
			}
			client, err := provider.NewOpenCodeConversationClient(
				provider.OpenCodeConversationConfig{
					ExecutablePath: resolved, HomePath: homePath,
					PrivateRoot: filepath.Join(filepath.Dir(statePath), "conversation-opencode"),
					ModelID:     modelID,
					Timeout:     2 * time.Minute, MaxOutputBytes: 32 * 1024,
					Runner: provider.NewSystemOpenCodeConversationRunner(),
				},
			)
			if err != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, err,
				)
			}
			openCodeClient = client
		}
		var codexClient *provider.CodexConversationClient
		var codexSegment *productCodexSegmentBackendConfig
		if setup.CodexExecutable != "" {
			resolved, err := provider.ResolveCodexNativeExecutable(setup.CodexExecutable)
			if err != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, err,
				)
			}
			homePath, err := os.UserHomeDir()
			if err != nil || !filepath.IsAbs(homePath) {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction,
					provider.ErrInvalidCodexConversationConfig,
				)
			}
			privateRoot := filepath.Join(filepath.Dir(statePath), "conversation-codex")
			client, err := provider.NewCodexConversationClient(
				provider.CodexConversationConfig{
					ExecutablePath: resolved, HomePath: homePath,
					PrivateRoot: privateRoot,
					Timeout:     2 * time.Minute, MaxOutputBytes: 32 * 1024,
					Runner: provider.NewSystemCodexConversationRunner(),
				},
			)
			if err != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, err,
				)
			}
			codexClient = client
			segmentPrivateRoot := filepath.Join(
				filepath.Dir(statePath), "conversation-codex-segments",
			)
			if err := prepareProductHarnessGatewayWorkspace(segmentPrivateRoot); err != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, err,
				)
			}
			codexSegment = &productCodexSegmentBackendConfig{
				ExecutablePath: resolved, HomePath: homePath,
				PrivateRoot: segmentPrivateRoot,
				Timeout:     8 * time.Hour, MaxOutputBytes: 64 << 10,
				Sessions: harnessadapter.NewSystemHarnessSessionRunner(),
			}
		}
		var claudeCodeRunner harnessadapter.HarnessProcessRunner
		var claudeCodeHomePath string
		if setup.ClaudeExecutable != "" {
			var err error
			claudeCodeHomePath, err = os.UserHomeDir()
			if err != nil || !filepath.IsAbs(claudeCodeHomePath) {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction,
					harnessadapter.ErrInvalidClaudeCodeAdapter,
				)
			}
			claudeCodeRunner, err = harnessadapter.NewClaudeCodeProcessRunner(
				harnessadapter.ClaudeCodeProcessRunnerConfig{
					Commands: harnessadapter.NewSystemHarnessCommandRunner(),
				},
			)
			if err != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, err,
				)
			}
		}
		deepSeekClient, err := provider.NewSystemDeepSeekConversationClient(45*time.Second, 256*1024)
		if err != nil {
			return productConversationRoutes{}, errors.Join(errProductConversationProviderConstruction, err)
		}
		kimiClient, err := provider.NewSystemKimiConversationClient(45*time.Second, 256*1024)
		if err != nil {
			return productConversationRoutes{}, errors.Join(errProductConversationProviderConstruction, err)
		}
		miniMaxClient, err := provider.NewSystemMiniMaxConversationClient(45*time.Second, 256*1024)
		if err != nil {
			return productConversationRoutes{}, errors.Join(errProductConversationProviderConstruction, err)
		}
		anthropicClient, err := provider.NewSystemAnthropicConversationClient(45*time.Second, 256*1024)
		if err != nil {
			return productConversationRoutes{}, errors.Join(errProductConversationProviderConstruction, err)
		}
		leases = setup.CredentialLeases
		documents := setup.ConversationDocuments
		contextCapsules := setup.ConversationContextCapsules
		if setup.UseCredentialVault {
			if vault == nil || !vault.Ready() {
				return productConversationRoutes{}, errors.Join(
					errProductConversationRouteUnavailable,
					credentials.ErrCredentialStoreUnavailable,
				)
			}
			leases = vault
			documents = vault
			contextCapsules = vault
		}
		if leases == nil {
			return productConversationRoutes{}, errProductConversationRouteUnavailable
		}
		gatewayWorkspacePath := filepath.Join(
			filepath.Dir(statePath), "conversation-workspace",
		)
		if err := prepareProductHarnessGatewayWorkspace(gatewayWorkspacePath); err != nil {
			return productConversationRoutes{}, errors.Join(
				errProductConversationRouteUnavailable, err,
			)
		}
		var codexResponder api.LocalProductConversationResponder
		if codexClient != nil {
			codexResponder = &productCodexConversationResponder{client: codexClient}
		}
		var openCodeResponder api.LocalProductConversationResponder
		if openCodeClient != nil {
			// Native OpenCode runs without a Loom credential. Account-scoped
			// OpenCode profiles use this same responder's exact bound lease path.
			openCodeResponder = &productOpenCodeConversationResponder{
				client: openCodeClient,
				leases: leases,
			}
		}
		var claudeCodeResponder api.LocalProductConversationResponder
		var claudeCodeSegment *productClaudeCodeSegmentBackendConfig
		if claudeCodeRunner != nil {
			configuredClaudeCodeResponder, responderErr := newProductClaudeCodeConversationResponder(
				productClaudeCodeConversationConfig{
					ExecutablePath: setup.ClaudeExecutable,
					HomePath:       claudeCodeHomePath,
					WorkspacePath:  gatewayWorkspacePath,
					PrivateRoot: filepath.Join(
						filepath.Dir(statePath), "conversation-claude-code",
					),
					Runner: claudeCodeRunner, Timeout: 2 * time.Minute,
					MaxOutputBytes: 32 * 1024,
				},
			)
			if responderErr != nil {
				return productConversationRoutes{}, errors.Join(
					errProductConversationNativeAuthConstruction, responderErr,
				)
			}
			claudeCodeResponder = configuredClaudeCodeResponder
			claudeCodeSegment = &productClaudeCodeSegmentBackendConfig{
				Responder: configuredClaudeCodeResponder,
			}
		}
		if conversationResponder == nil && openCodeResponder != nil {
			conversationResponder = openCodeResponder
		}
		if conversationResponder == nil && codexResponder != nil {
			conversationResponder = codexResponder
		}
		if conversationResponder == nil && claudeCodeResponder != nil {
			conversationResponder = claudeCodeResponder
		}
		router, err := newProductConversationProfileRouterWithPolicy(
			conversationResponder,
			func(providerID string) []projection.ProviderCredentialRecord {
				return readModel.GlobalReadView().ProviderAccountCredentials(providerID)
			},
			func(providerID, accountID string) (work.ProviderAccountPolicy, bool) {
				return readModel.GlobalReadView().ProviderAccountPolicy(providerID, accountID)
			},
			leases, deepSeekClient, true, openCodeResponder, codexResponder,
			productConversationProviderRoute{
				providerID: "anthropic", profileID: provider.AnthropicConversationAccountProfileID,
				client: anthropicClient,
			},
			productConversationProviderRoute{
				providerID: "kimi", profileID: provider.KimiConversationAccountProfileID,
				client: kimiClient,
			},
			productConversationProviderRoute{
				providerID: "minimax", profileID: provider.MiniMaxConversationAccountProfileID,
				client: miniMaxClient,
			},
		)
		if err != nil {
			return productConversationRoutes{}, errors.Join(errProductConversationProviderConstruction, err)
		}
		router.claudeCodeResponder = claudeCodeResponder
		harnessGateway, err = newProductHarnessGatewayConversationResponder(
			productHarnessGatewayConversationConfig{
				Executor: router, WorkspacePath: gatewayWorkspacePath,
				CodexSegment: codexSegment, ClaudeCodeSegment: claudeCodeSegment,
				Events: &productHarnessGatewayOperationalEventSink{
					diagnostics: diagnostics,
				},
				Now: func() time.Time { return time.Now().UTC() },
			},
		)
		if err != nil {
			return productConversationRoutes{}, errors.Join(
				errProductConversationRouteUnavailable, err,
			)
		}
		migration := &productChatMigrationDiagnosticRecorder{
			store: diagnostics,
			incidentID: "loom-migration-" + productDeterministicUUID(
				"conversation-migration", statePath,
				time.Now().UTC().Format(time.RFC3339Nano), fmt.Sprint(os.Getpid()),
			),
		}
		storePath := filepath.Join(filepath.Dir(statePath), "chat-threads.json")
		var chat *api.LocalProductChatAPI
		switch {
		case documents != nil:
			chat, err = api.NewEncryptedPersistentLocalProductChatAPI(
				ctx, storePath, documents, func() time.Time { return time.Now().UTC() },
				harnessGateway, migration,
			)
		case setup.UseCredentialVault:
			chat = api.NewUnavailableLocalProductChatAPI(func() time.Time { return time.Now().UTC() })
		default:
			chat, err = api.NewPersistentLocalProductChatAPI(
				storePath, func() time.Time { return time.Now().UTC() }, harnessGateway,
			)
		}
		if err != nil {
			failure, found := migration.availabilityFailure()
			if !found {
				failure = api.LocalProductChatAvailabilityFailure{
					Code: "state_unavailable", Stage: "migration_read",
					IncidentID: migration.incidentID, Retryable: false,
				}
			}
			chat = api.NewUnavailableLocalProductChatAPIWithFailure(
				func() time.Time { return time.Now().UTC() }, failure,
			)
		}
		if err := chat.SetConversationExecutionBindingResolver(router); err != nil {
			return productConversationRoutes{}, err
		}
		if setup.ConversationScopes != nil {
			if err := chat.SetConversationScopeManager(setup.ConversationScopes); err != nil {
				return productConversationRoutes{}, err
			}
		}
		if contextCapsules != nil {
			if err := chat.SetConversationContextCapsuleRuntime(
				router, contextCapsules,
			); err != nil {
				return productConversationRoutes{}, err
			}
		}
		closeRoute := func() error {
			closeContext, cancel := context.WithTimeout(
				context.Background(), 5*time.Second,
			)
			defer cancel()
			result := harnessGateway.Close(closeContext)
			if !nilProductAssetPort(localModel) {
				result = errors.Join(result, localModel.Close())
			}
			return result
		}
		return productConversationRoutes{
			route: chat, localModel: localModel, close: closeRoute,
		}, nil
	}
}

func newProductConversationConstructionFactoryFromCore(
	statePath string,
	setup productSetupRuntimeConfig,
	core *productCoreRouteSlot,
	vault *productVaultRouteSlot,
	diagnostics productOperationalDiagnosticSink,
) func(context.Context) (productConversationRoutes, error) {
	return func(ctx context.Context) (productConversationRoutes, error) {
		resources, err := core.Resources()
		if err != nil {
			return productConversationRoutes{}, err
		}
		return newProductConversationConstructionFactory(
			statePath, setup, resources.readModel, vault, diagnostics,
		)(ctx)
	}
}

func (construction productCompatibilityConstruction) startConversation(
	ctx context.Context,
	_ *composition.BundleContext,
) (composition.Effect, error) {
	if !construction.valid() || construction.conversationSlot == nil || ctx == nil {
		return nil, composition.ErrInvalidComposition
	}
	routes, err := construction.conversationFactory(ctx)
	if err != nil || !routes.valid() {
		if routes.close != nil {
			_ = routes.close()
		}
		return nil, errors.Join(errProductConversationRouteUnavailable, err)
	}
	if err := construction.conversationSlot.Bind(routes); err != nil {
		_ = routes.close()
		return nil, err
	}
	return composition.NewEffect(construction.conversationSlot.Close), nil
}

func (construction productCompatibilityConstruction) conversationReady(
	context.Context,
	*composition.BundleContext,
) error {
	if !construction.valid() || construction.conversationSlot == nil ||
		!construction.conversationSlot.Ready() {
		return errProductConversationRouteUnavailable
	}
	return nil
}

var _ api.LocalProductChatSource = (*api.LocalProductChatAPI)(nil)
var _ api.LocalProductChatSource = (*productConversationRouteSlot)(nil)
var _ api.LocalProductChatContextDisclosureInspector = (*productConversationRouteSlot)(nil)
var _ api.LocalProductConversationResponseCanceller = (*productConversationRouteSlot)(nil)
