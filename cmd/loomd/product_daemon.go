package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"

	_ "modernc.org/sqlite"
)

const localProductBuildID = "loom-phase2a-w1"

type productSetupRuntimeConfig struct {
	CodexExecutable string
}

type productDaemonFailure struct {
	code string
	err  error
}

func (failure *productDaemonFailure) Error() string {
	return "product daemon failed: " + failure.code
}

func (failure *productDaemonFailure) Unwrap() error {
	return failure.err
}

func (failure *productDaemonFailure) DaemonFailureCode() string {
	return failure.code
}

func classifyProductDaemonFailure(code string, err error) error {
	if err == nil {
		err = errors.New("product daemon lifecycle failure")
	}
	return &productDaemonFailure{code: code, err: err}
}

type productDaemonRunner struct {
	observer daemonRunner
	server   *localipc.Server
	database *sql.DB
	setup    *api.LocalProductSetupAPI

	mu      sync.Mutex
	running bool
	closed  bool
}

func newProductDaemonRunner(
	observer daemonRunner,
	statePath,
	socketPath string,
	setupConfigs ...productSetupRuntimeConfig,
) (_ *productDaemonRunner, resultErr error) {
	if observer == nil {
		return nil, errors.New("invalid product daemon")
	}
	database, err := openProductReadDatabase(statePath)
	if err != nil {
		_ = observer.Close()
		return nil, err
	}
	var setupService *api.LocalProductSetupAPI
	defer func() {
		if resultErr != nil {
			if setupService != nil {
				_ = setupService.Close()
			}
			_ = database.Close()
			_ = observer.Close()
		}
	}()
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		return nil, err
	}
	store := journal.NewStore(database)
	service, err := api.NewLocalProductReadService(api.LocalProductReadConfig{
		Journal:    store,
		Projection: readModel,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}
	setupConfig := productSetupRuntimeConfig{}
	if len(setupConfigs) == 1 {
		setupConfig = setupConfigs[0]
	}
	setupService, err = buildProductSetupService(
		database,
		store,
		readModel,
		setupConfig,
	)
	if err != nil {
		return nil, err
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      localProductBuildID,
		Handler: localipc.HandlerFunc(
			localProductHandler(service, setupService),
		),
	})
	if err != nil {
		return nil, err
	}
	return &productDaemonRunner{
		observer: observer,
		server:   server,
		database: database,
		setup:    setupService,
	}, nil
}

type productSetupProjectionProbe struct {
	observations []loomruntime.RuntimeObservation
}

func productSetupRuntimeStatus(
	status string,
) (loomruntime.RuntimeStatus, bool) {
	switch status {
	case "online":
		return loomruntime.RuntimeOnline, true
	case "offline":
		return loomruntime.RuntimeOffline, true
	case "incompatible":
		return loomruntime.RuntimeIncompatible, true
	case "disabled":
		return loomruntime.RuntimeDisabled, true
	default:
		return "", false
	}
}

func (probe productSetupProjectionProbe) ID() string {
	return "loom-product-projection"
}

func (probe productSetupProjectionProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	return append([]loomruntime.RuntimeObservation{}, probe.observations...), nil
}

type productSetupIdentity struct{}

func (productSetupIdentity) NextSetupID(kind string) (string, error) {
	if kind == "" {
		return "", app.ErrInvalidLocalProductSetup
	}
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", app.ErrInvalidLocalProductSetup
	}
	return kind + "-" + hex.EncodeToString(value[:]), nil
}

type productNativeAuthObserver struct {
	observer *provider.CodexNativeAuthObserver
}

func (observer productNativeAuthObserver) ObserveNativeAuth(
	ctx context.Context,
) (app.NativeAuthObservation, error) {
	if observer.observer == nil {
		return app.NativeAuthObservation{
			Status:   "unavailable",
			AuthMode: "native_auth",
			Reason:   "unavailable",
		}, nil
	}
	observation, err := observer.observer.Observe(ctx)
	if err != nil {
		return app.NativeAuthObservation{}, err
	}
	return app.NativeAuthObservation{
		Status:   string(observation.Status),
		AuthMode: observation.AuthMode,
		Reason:   string(observation.Reason),
	}, nil
}

type productNativeAuthConnector struct {
	controller provider.CodexLoginController
}

func (connector productNativeAuthConnector) StartNativeAuth(
	ctx context.Context,
) error {
	if connector.controller == nil {
		return app.ErrNativeAuthConnectUnavailable
	}
	result, err := connector.controller.Start(ctx)
	switch {
	case errors.Is(err, provider.ErrCodexLoginBusy):
		return app.ErrNativeAuthConnectBusy
	case errors.Is(err, provider.ErrCodexLoginUnavailable),
		errors.Is(err, provider.ErrCodexExecutableIdentityChanged),
		errors.Is(err, provider.ErrInvalidCodexLoginController):
		return app.ErrNativeAuthConnectUnavailable
	case err == nil && result.Status != provider.CodexLoginStarted:
		return app.ErrNativeAuthConnectUnavailable
	default:
		return err
	}
}

func (connector productNativeAuthConnector) Close() error {
	if connector.controller == nil {
		return nil
	}
	return connector.controller.Close()
}

type productCredentialStatusSource struct {
	projection *projection.Projection
}

func (source productCredentialStatusSource) CredentialStatus(
	ctx context.Context,
	providerID string,
) (credentials.MetadataResult, error) {
	if source.projection == nil || ctx == nil || providerID != "minimax" {
		return credentials.MetadataResult{},
			credentials.ErrInvalidCredentialCommand
	}
	if err := source.projection.Rebuild(ctx); err != nil {
		return credentials.MetadataResult{},
			credentials.ErrCredentialStoreUnavailable
	}
	record, ok := source.projection.GlobalReadView().ProviderCredential(
		providerID,
	)
	if !ok {
		return credentials.MetadataResult{},
			credentials.ErrCredentialNotFound
	}
	return credentials.MetadataResult{
		ProviderID:          record.ProviderID,
		CredentialReference: record.CredentialReference,
		Revision:            record.Revision,
		Status:              credentials.CredentialStatus(record.Status),
		Reason:              credentials.VerificationReason(record.Reason),
	}, nil
}

func buildProductSetupService(
	database *sql.DB,
	store *journal.Store,
	readModel *projection.Projection,
	config productSetupRuntimeConfig,
) (*api.LocalProductSetupAPI, error) {
	if database == nil || store == nil || readModel == nil {
		return nil, errors.New("setup state unavailable")
	}
	runtimes, _ := readModel.GlobalReadView().RuntimeInstances("", 64)
	observations := make([]loomruntime.RuntimeObservation, 0, len(runtimes))
	for _, runtime := range runtimes {
		status, ok := productSetupRuntimeStatus(runtime.Status)
		if !ok {
			continue
		}
		observations = append(observations, loomruntime.RuntimeObservation{
			Instance: loomruntime.RuntimeInstance{
				ID:                runtime.ID,
				DeviceID:          runtime.DeviceID,
				AdapterType:       runtime.AdapterType,
				DisplayName:       runtime.DisplayName,
				ExecutableVersion: runtime.ExecutableVersion,
				Status:            status,
				ObservedCapabilities: append(
					[]string{},
					runtime.ObservedCapabilities...,
				),
				Capacity: runtime.Capacity,
			},
			ModelIDs: append([]string{}, runtime.ModelIDs...),
		})
	}
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{
			productSetupProjectionProbe{observations: observations},
		},
	)
	if err != nil {
		return nil, errors.New("setup runtime unavailable")
	}
	definitions := []agents.AgentDefinition{}
	profiles := []loomruntime.RuntimeProfile{}
	roleOptions := []app.SetupRoleOption{}
	concurrencyCeiling := 1
	var selected *loomruntime.RuntimeObservation
	for _, observation := range discovery.Observations() {
		if observation.Instance.Status == loomruntime.RuntimeOnline &&
			observation.Instance.Capacity > 0 &&
			len(observation.ModelIDs) > 0 {
			copy := observation
			selected = &copy
			break
		}
	}
	if selected != nil {
		runtime := *selected
		definitions = []agents.AgentDefinition{
			{
				ID:       "loom-main-coordinator",
				Version:  1,
				Scope:    agents.ScopeReusable,
				Name:     "Coordinator",
				RoleSpec: "Coordinate bounded work and review",
				Status:   agents.DefinitionActive,
			},
			{
				ID:       "loom-bounded-worker",
				Version:  1,
				Scope:    agents.ScopeReusable,
				Name:     "Bounded Worker",
				RoleSpec: "Deliver one bounded task for review",
				Status:   agents.DefinitionActive,
			},
		}
		mainProfile := loomruntime.RuntimeProfile{
			ID:                   "loom-main-native",
			AdapterType:          runtime.Instance.AdapterType,
			ProviderID:           "local",
			ModelID:              runtime.ModelIDs[0],
			AuthMode:             loomruntime.AuthNative,
			RequiredCapabilities: []string{},
			Timeout:              5 * time.Minute,
		}
		subProfile := mainProfile
		subProfile.ID = "loom-subagent-native"
		profiles = []loomruntime.RuntimeProfile{mainProfile, subProfile}
		roleOptions = []app.SetupRoleOption{
			{
				ID:                "coordinator",
				Kind:              "main",
				AgentDefinitionID: definitions[0].ID,
				RuntimeProfileID:  mainProfile.ID,
				RuntimeInstanceID: runtime.Instance.ID,
				SkillRevisionIDs:  []string{},
				PermissionIDs:     []string{},
				ResourceIDs:       []string{},
				Responsibility:    "Coordinate bounded work and review",
			},
			{
				ID:                "bounded-worker",
				Kind:              "subagent",
				AgentDefinitionID: definitions[1].ID,
				RuntimeProfileID:  subProfile.ID,
				RuntimeInstanceID: runtime.Instance.ID,
				SkillRevisionIDs:  []string{},
				PermissionIDs:     []string{},
				ResourceIDs:       []string{},
				Responsibility:    "Deliver one bounded task for review",
			},
		}
		concurrencyCeiling = min(runtime.Instance.Capacity, 2)
	}
	sum := sha256.Sum256([]byte(
		"loom-product-setup-v1\x00" + discovery.Digest(),
	))
	catalog := app.LocalProductSetupCatalog{
		CatalogDigest:      hex.EncodeToString(sum[:]),
		AgentDefinitions:   definitions,
		RuntimeProfiles:    profiles,
		RuntimeDiscovery:   discovery,
		SkillRevisions:     []app.SetupSkillRevision{},
		Permissions:        []string{},
		Resources:          []app.SetupResourcePointer{},
		RoleOptions:        roleOptions,
		Templates:          []app.SetupTeamTemplate{},
		BudgetCeiling:      100,
		ConcurrencyCeiling: concurrencyCeiling,
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		return nil, errors.New("setup state unavailable")
	}
	keychain, err := credentials.NewKeychainStore(
		credentials.KeychainStoreConfig{},
	)
	if err != nil {
		return nil, nil
	}
	verifier, err := provider.NewSystemMiniMaxCredentialVerifier(
		5*time.Second,
		64*1024,
	)
	if err != nil {
		return nil, errors.New("setup Provider unavailable")
	}
	broker, err := credentials.NewCredentialBroker(
		credentials.CredentialBrokerConfig{
			Store:     keychain,
			Verifier:  verifier,
			Committer: writer,
		},
	)
	if err != nil {
		return nil, errors.New("setup credential boundary unavailable")
	}
	var (
		native          *provider.CodexNativeAuthObserver
		nativeConnector app.NativeAuthConnector
	)
	if config.CodexExecutable != "" {
		native, err = provider.NewCodexNativeAuthObserver(
			provider.CodexNativeAuthConfig{
				ExecutablePath: config.CodexExecutable,
				Timeout:        5 * time.Second,
				MaxOutputBytes: 4096,
				Runner:         provider.NewSystemCodexStatusRunner(),
			},
		)
		if err != nil {
			return nil, errors.New("setup native auth unavailable")
		}
		controller, controllerErr := provider.NewSystemCodexLoginController(
			provider.CodexLoginControllerConfig{
				ExecutablePath: config.CodexExecutable,
				Timeout:        10 * time.Minute,
			},
		)
		if controllerErr != nil {
			return nil, errors.New("setup native auth unavailable")
		}
		nativeConnector = productNativeAuthConnector{
			controller: controller,
		}
	}
	setup, err := app.NewLocalProductSetupService(
		app.LocalProductSetupConfig{
			Journal:             store,
			Projection:          readModel,
			Writer:              writer,
			Catalog:             catalog,
			Identity:            productSetupIdentity{},
			Now:                 func() time.Time { return time.Now().UTC() },
			NativeAuth:          productNativeAuthObserver{observer: native},
			NativeAuthConnector: nativeConnector,
			Credentials:         productCredentialStatusSource{projection: readModel},
			CredentialMutator:   broker,
		},
	)
	if err != nil {
		if nativeConnector != nil {
			_ = nativeConnector.Close()
		}
		return nil, errors.New("setup service unavailable")
	}
	setupAPI, err := api.NewLocalProductSetupAPI(setup)
	if err != nil {
		_ = setup.Close()
		return nil, errors.New("setup API unavailable")
	}
	return setupAPI, nil
}

func (runner *productDaemonRunner) Run(
	ctx context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	if runner == nil || ctx == nil {
		return app.LocalRuntimeObservationDaemonResult{},
			errors.New("invalid product daemon")
	}
	runner.mu.Lock()
	if runner.running || runner.closed {
		runner.mu.Unlock()
		return app.LocalRuntimeObservationDaemonResult{},
			errors.New("product daemon unavailable")
	}
	runner.running = true
	runner.mu.Unlock()
	defer func() {
		runner.mu.Lock()
		runner.running = false
		runner.mu.Unlock()
	}()

	runContext, cancel := context.WithCancel(ctx)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- runner.server.Serve(runContext)
	}()
	select {
	case <-runner.server.Ready():
	case serverErr := <-serverDone:
		return app.LocalRuntimeObservationDaemonResult{},
			classifyProductDaemonFailure("local_ipc", serverErr)
	case <-ctx.Done():
		closeErr := runner.server.Close()
		serverErr := <-serverDone
		if closeErr != nil || serverErr != nil {
			return app.LocalRuntimeObservationDaemonResult{},
				classifyProductDaemonFailure(
					"shutdown",
					errors.Join(serverErr, closeErr),
				)
		}
		return app.LocalRuntimeObservationDaemonResult{}, ctx.Err()
	}

	type observerOutcome struct {
		result app.LocalRuntimeObservationDaemonResult
		err    error
	}
	observerDone := make(chan observerOutcome, 1)
	go func() {
		result, err := runner.observer.Run(runContext)
		observerDone <- observerOutcome{result: result, err: err}
	}()

	select {
	case outcome := <-observerDone:
		cancel()
		closeErr := runner.server.Close()
		serverErr := <-serverDone
		if outcome.err != nil &&
			!errors.Is(outcome.err, context.Canceled) &&
			!errors.Is(outcome.err, context.DeadlineExceeded) {
			return outcome.result, classifyProductDaemonFailure(
				"observer",
				errors.Join(outcome.err, serverErr, closeErr),
			)
		}
		if serverErr != nil || closeErr != nil {
			return outcome.result, classifyProductDaemonFailure(
				"shutdown",
				errors.Join(serverErr, closeErr),
			)
		}
		return outcome.result, outcome.err
	case serverErr := <-serverDone:
		cancel()
		outcome := <-observerDone
		return outcome.result, classifyProductDaemonFailure(
			"local_ipc",
			errors.Join(serverErr, outcome.err),
		)
	case <-ctx.Done():
		cancel()
		closeErr := runner.server.Close()
		outcome := <-observerDone
		serverErr := <-serverDone
		if outcome.err != nil &&
			!errors.Is(outcome.err, context.Canceled) &&
			!errors.Is(outcome.err, context.DeadlineExceeded) {
			return outcome.result, classifyProductDaemonFailure(
				"observer",
				errors.Join(outcome.err, serverErr, closeErr),
			)
		}
		if serverErr != nil || closeErr != nil {
			return outcome.result, classifyProductDaemonFailure(
				"shutdown",
				errors.Join(serverErr, closeErr),
			)
		}
		return outcome.result, ctx.Err()
	}
}

func (runner *productDaemonRunner) Close() error {
	if runner == nil {
		return errors.New("invalid product daemon")
	}
	runner.mu.Lock()
	if runner.running {
		runner.mu.Unlock()
		return errors.New("product daemon running")
	}
	if runner.closed {
		runner.mu.Unlock()
		return nil
	}
	runner.closed = true
	runner.mu.Unlock()
	return errors.Join(
		runner.server.Close(),
		runner.setup.Close(),
		runner.database.Close(),
		runner.observer.Close(),
	)
}

func localProductHandler(
	service *api.LocalProductReadService,
	setupServices ...*api.LocalProductSetupAPI,
) func(context.Context, localipc.Request) localipc.Response {
	var setup *api.LocalProductSetupAPI
	if len(setupServices) == 1 {
		setup = setupServices[0]
	}
	return func(
		ctx context.Context,
		request localipc.Request,
	) localipc.Response {
		if productSetupMethod(request.Method) && setup == nil {
			return productErrorResponse(
				"state_unavailable",
				api.ErrInvalidLocalProductSetupAPI,
			)
		}
		switch request.Method {
		case "snapshot":
			if service == nil {
				return productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				)
			}
			var input api.LocalProductSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductRequest,
				)
			}
			result, err := service.ReadLocalProductSnapshot(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "timeline_page":
			if service == nil {
				return productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				)
			}
			var input api.LocalProductTimelineRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductRequest,
				)
			}
			result, err := service.ReadLocalProductTimeline(ctx, input)
			if err != nil {
				var gapErr *api.TimelineGapError
				if !errors.As(err, &gapErr) {
					return productServiceError(err)
				}
			}
			return productResultResponse(result)
		case "setup_snapshot":
			if decodeExactProductParams(
				request.Params,
				&struct{}{},
			) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.SetupSnapshot(ctx)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "codex_connect":
			if decodeExactProductParams(
				request.Params,
				&struct{}{},
			) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ConnectCodex(ctx)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_start":
			var input app.BuilderStartCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.StartBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_answer":
			var input app.BuilderAnswerCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.AnswerBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_edit":
			var input app.BuilderEditCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.EditBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_validate":
			var input app.BuilderValidateCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ValidateBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_confirm":
			var input app.BuilderConfirmCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ConfirmBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "team_archive", "team_restore":
			var input app.TeamStatusCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			var (
				result app.SetupSavedTeamPreview
				err    error
			)
			if request.Method == "team_archive" {
				result, err = setup.ArchiveTeam(ctx, input)
			} else {
				result, err = setup.RestoreTeam(ctx, input)
			}
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "credential_configure",
			"credential_verify",
			"credential_replace",
			"credential_revoke":
			var input productCredentialParams
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			secret := []byte(input.Secret)
			defer clearProductSecret(secret)
			command := app.CredentialSetupCommand{
				ProviderID:          input.ProviderID,
				CredentialReference: input.CredentialReference,
				ExpectedRevision:    input.ExpectedRevision,
				Secret:              secret,
			}
			var (
				result app.CredentialSetupResult
				err    error
			)
			switch request.Method {
			case "credential_configure":
				result, err = setup.ConfigureCredential(ctx, command)
			case "credential_verify":
				result, err = setup.VerifyCredential(ctx, command)
			case "credential_replace":
				result, err = setup.ReplaceCredential(ctx, command)
			case "credential_revoke":
				result, err = setup.RevokeCredential(ctx, command)
			}
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		default:
			return productErrorResponse(
				"unknown_method",
				errors.New("unknown method"),
			)
		}
	}
}

func productSetupMethod(method string) bool {
	switch method {
	case "setup_snapshot",
		"codex_connect",
		"builder_start",
		"builder_answer",
		"builder_edit",
		"builder_validate",
		"builder_confirm",
		"team_archive",
		"team_restore",
		"credential_configure",
		"credential_verify",
		"credential_replace",
		"credential_revoke":
		return true
	default:
		return false
	}
}

type productCredentialParams struct {
	ProviderID          string `json:"provider_id"`
	CredentialReference string `json:"credential_reference"`
	ExpectedRevision    int64  `json:"expected_revision"`
	Secret              string `json:"secret"`
}

func clearProductSecret(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}

func decodeExactProductParams(data []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing product params")
	}
	return nil
}

func productResultResponse(value any) localipc.Response {
	result, err := json.Marshal(value)
	if err != nil {
		return productErrorResponse("internal", err)
	}
	return localipc.Response{OK: true, Result: result}
}

func productServiceError(err error) localipc.Response {
	switch {
	case errors.Is(err, api.ErrInvalidLocalProductRequest),
		errors.Is(err, api.ErrInvalidTimelineRequest),
		errors.Is(err, api.ErrInvalidLocalProductSetupAPI),
		errors.Is(err, app.ErrInvalidLocalProductSetup):
		return productErrorResponse("invalid_request", err)
	case errors.Is(err, api.ErrTeamTimelineNotFound),
		errors.Is(err, app.ErrBuilderNotFound):
		return productErrorResponse("not_found", err)
	case errors.Is(err, app.ErrBuilderConflict):
		return productErrorResponse("conflict", err)
	case errors.Is(err, app.ErrBuilderIncompatible):
		return productErrorResponse("incompatible", err)
	case errors.Is(err, app.ErrBuilderConfirmationRequired):
		return productErrorResponse("denied", err)
	case errors.Is(err, app.ErrNativeAuthConnectBusy):
		return productErrorResponse("busy", err)
	case errors.Is(err, app.ErrNativeAuthConnectUnavailable):
		return productErrorResponse("state_unavailable", err)
	case errors.Is(err, credentials.ErrCredentialStoreDenied):
		return productErrorResponse("denied", err)
	case errors.Is(err, credentials.ErrCredentialStoreUnavailable),
		errors.Is(err, app.ErrCredentialSetupUnavailable):
		return productErrorResponse("credential_unavailable", err)
	case errors.Is(err, credentials.ErrCredentialRejected):
		return productErrorResponse("credential_rejected", err)
	case errors.Is(err, credentials.ErrCredentialRollbackFailed):
		return productErrorResponse("credential_rollback_failed", err)
	case errors.Is(err, api.ErrTimelineCursorConflict):
		return productErrorResponse("cursor_conflict", err)
	case errors.Is(err, api.ErrStreamGap):
		return productErrorResponse("stream_gap", err)
	case errors.Is(err, context.DeadlineExceeded):
		return productErrorResponse("timeout", err)
	default:
		return productErrorResponse("state_unavailable", err)
	}
}

func productErrorResponse(code string, cause error) localipc.Response {
	return localipc.Response{
		OK:    false,
		Error: localipcSafeError(code, cause),
	}
}

func localipcSafeError(code string, _ error) *localipc.ProtocolError {
	messages := map[string]struct {
		message     string
		recoverable bool
	}{
		"invalid_request": {"invalid request", false},
		"unknown_method":  {"unknown method", false},
		"not_found":       {"not found", false},
		"conflict":        {"conflict", true},
		"busy":            {"busy", true},
		"incompatible":    {"incompatible", false},
		"denied":          {"denied", false},
		"credential_unavailable": {
			"credential unavailable",
			true,
		},
		"credential_rejected": {
			"credential rejected",
			false,
		},
		"credential_rollback_failed": {
			"credential rollback failed",
			false,
		},
		"cursor_conflict":   {"cursor conflict", true},
		"stream_gap":        {"stream gap", true},
		"state_unavailable": {"state unavailable", true},
		"timeout":           {"request timed out", true},
		"internal":          {"internal error", true},
	}
	definition, ok := messages[code]
	if !ok {
		code = "internal"
		definition = messages[code]
	}
	return &localipc.ProtocolError{
		Code:        code,
		Message:     definition.message,
		Recoverable: definition.recoverable,
	}
}

func openProductReadDatabase(statePath string) (*sql.DB, error) {
	absolute, err := filepath.Abs(statePath)
	if err != nil || !filepath.IsAbs(absolute) {
		return nil, errors.New("state unavailable")
	}
	info, err := os.Stat(absolute)
	if err != nil || info.IsDir() {
		return nil, errors.New("state unavailable")
	}
	values := url.Values{}
	values.Add("mode", "rw")
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	uri := url.URL{Scheme: "file", Path: absolute}
	uri.RawQuery = values.Encode()
	database, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, errors.New("state unavailable")
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, errors.New("state unavailable")
	}
	return database, nil
}
