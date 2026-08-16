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
		defer func() {
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
			client, err := provider.NewOpenCodeConversationClient(
				provider.OpenCodeConversationConfig{
					ExecutablePath: resolved, HomePath: homePath,
					PrivateRoot: filepath.Join(filepath.Dir(statePath), "conversation-opencode"),
					ModelID:     provider.OpenCodeConversationDefaultModel,
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
			client, err := provider.NewCodexConversationClient(
				provider.CodexConversationConfig{
					ExecutablePath: resolved, HomePath: homePath,
					PrivateRoot: filepath.Join(filepath.Dir(statePath), "conversation-codex"),
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
		var codexResponder api.LocalProductConversationResponder
		if codexClient != nil {
			codexResponder = &productCodexConversationResponder{client: codexClient}
		}
		if conversationResponder == nil && openCodeClient != nil {
			// OpenCode is a native conversation profile that needs the Loom
			// Vault lease access to inject the bound model's Provider
			// credential (for example DEEPSEEK_API_KEY) into the OpenCode
			// process environment. Bind it only after leases is resolved so the
			// responder never captures a nil lease access.
			conversationResponder = &productOpenCodeConversationResponder{
				client: openCodeClient,
				leases: leases,
				credentials: func(
					ctx context.Context,
					providerID string,
				) []projection.ProviderCredentialRecord {
					return readModel.GlobalReadView().ProviderAccountCredentials(
						providerID,
					)
				},
			}
		}
		if conversationResponder == nil && codexResponder != nil {
			conversationResponder = codexResponder
		}
		router, err := newProductConversationProfileRouterWithPolicy(
			conversationResponder,
			func(providerID string) []projection.ProviderCredentialRecord {
				return readModel.GlobalReadView().ProviderAccountCredentials(providerID)
			},
			func(providerID, accountID string) (work.ProviderAccountPolicy, bool) {
				return readModel.GlobalReadView().ProviderAccountPolicy(providerID, accountID)
			},
			leases, deepSeekClient, true, codexResponder,
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
				router, migration,
			)
		case setup.UseCredentialVault:
			chat = api.NewUnavailableLocalProductChatAPI(func() time.Time { return time.Now().UTC() })
		default:
			chat, err = api.NewPersistentLocalProductChatAPI(
				storePath, func() time.Time { return time.Now().UTC() }, router,
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
		closeRoute := func() error { return nil }
		if !nilProductAssetPort(localModel) {
			closeRoute = localModel.Close
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
