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
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/mode"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/discoveryscan"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"

	_ "modernc.org/sqlite"
)

const localProductBuildID = "loom-phase2a-w1"

const productSavedTeamResolutionProjectID = "loom-local-product"

const controlledMissionFixtureManifestEnvironment = "LOOM_CONTROLLED_MISSION_FIXTURE_MANIFEST"

type controlledMissionFixtureManifest struct {
	SchemaVersion     int    `json:"schema_version"`
	Purpose           string `json:"purpose"`
	AttemptID         string `json:"attempt_id"`
	StatePath         string `json:"state_path"`
	ArtifactRoot      string `json:"artifact_root"`
	SourceCommit      string `json:"source_commit"`
	AuthoritativeTime string `json:"authoritative_time"`
	FixtureID         string `json:"fixture_id"`
}

type productSetupRuntimeConfig struct {
	CodexExecutable string
	SocketPath      string
	CredentialStore credentials.SecretStore
	Execution       *productMissionExecutionRuntimeConfig
}

type productMissionExecutionRuntimeConfig struct {
	RuntimeSearchPaths []string
	RuntimeInstanceID  string
	LocalModelCatalog  *piadapter.PiLocalModelCatalogConfig
	Now                func() time.Time
	ExecutorFactory    productMissionExecutorFactory
	Decisions          app.MissionExecutionDecisionRouter
}

type productSavedTeamMaterializer interface {
	MaterializeConfirmedTeam(
		context.Context,
		app.BuilderConfirmation,
	) (app.BuilderConfirmation, error)
}

type productSavedTeamMaterialization struct {
	store      *journal.Store
	projection *projection.Projection
	now        func() time.Time
}

type productMissionExecutorPort interface {
	Execute(context.Context, supervisor.ExecuteInput) (supervisor.Outcome, error)
	Close(context.Context) error
}

type productMissionExecutorFactory func(
	context.Context,
	string,
	*work.Authority,
	*authorization.Authority,
) (productMissionExecutorPort, error)

type productDaemonFailure struct {
	code   string
	reason string
	err    error
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

func (failure *productDaemonFailure) DaemonFailureReason() string {
	return failure.reason
}

func classifyProductDaemonFailure(code string, err error) error {
	if err == nil {
		err = errors.New("product daemon lifecycle failure")
	}
	reason := ""
	if code == "observer" {
		reason = observerFailureReason(err)
	}
	return &productDaemonFailure{code: code, reason: reason, err: err}
}

func observerFailureReason(err error) string {
	if err == nil {
		return "observer_unknown"
	}

	reasons := make(map[string]struct{}, 4)
	add := func(reason string) {
		if reason != "" && reason != "observer_unknown" {
			reasons[reason] = struct{}{}
		}
	}

	if errors.Is(err, app.ErrRuntimeObservationProjectionRefresh) {
		add("observer_projection")
	}
	if errors.Is(err, app.ErrLocalRuntimeObservationDaemonMetadata) {
		add("observer_identity_metadata")
	} else if runtimeObservationWriteFailure(err) {
		add("observer_write")
	}
	if runtimeObservationPlanFailure(err) {
		add("observer_plan")
	}
	if runtimeObservationInventoryFailure(err) {
		add("observer_inventory")
	}
	if errors.Is(err, piadapter.ErrPiMetadataBindingChanged) ||
		errors.Is(err, piadapter.ErrPiLocalRuntimeProbeBindingChanged) {
		add("observer_metadata_binding")
	}

	probeSpecific := false
	if errors.Is(err, piadapter.ErrPiLocalRuntimeCandidateInvalid) {
		add("observer_probe_candidate")
		probeSpecific = true
	}
	if errors.Is(err, piadapter.ErrPiLocalRuntimeProbeConstructionFailed) {
		add("observer_probe_construction")
		probeSpecific = true
	}
	if !probeSpecific && errors.Is(err, discoveryscan.ErrRuntimeProbeFactoryFailed) {
		add("observer_probe_factory")
	}

	command, commandFailure := uniquePiMetadataFailureCommand(err)
	if commandFailure {
		add(piMetadataObserverFailureReason(command, err))
	} else if hasPiMetadataFailureCommand(err) {
		return "observer_unknown"
	}

	if len(reasons) != 1 {
		return "observer_unknown"
	}
	for reason := range reasons {
		if observerFailureHasUnknownLeaf(err) {
			return "observer_unknown"
		}
		return reason
	}
	return "observer_unknown"
}

func observerFailureHasUnknownLeaf(err error) bool {
	unknown := false
	visitDaemonErrorLeaves(err, func(candidate error) {
		if !knownObserverFailureLeaf(candidate) {
			unknown = true
		}
	})
	return unknown
}

func visitDaemonErrorLeaves(err error, visit func(error)) {
	if err == nil {
		return
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) != 0 {
			for _, child := range children {
				visitDaemonErrorLeaves(child, visit)
			}
			return
		}
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if child := wrapped.Unwrap(); child != nil {
			visitDaemonErrorLeaves(child, visit)
			return
		}
	}
	visit(err)
}

func knownObserverFailureLeaf(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, app.ErrLocalRuntimeObservationDaemonCycle) ||
		errors.Is(err, discoveryscan.ErrRuntimeProbeFactoryFailed) ||
		errors.Is(err, piadapter.ErrPiLocalRuntimeCandidateInvalid) ||
		errors.Is(err, piadapter.ErrPiLocalRuntimeProbeConstructionFailed) ||
		errors.Is(err, piadapter.ErrPiMetadataBindingChanged) ||
		errors.Is(err, piadapter.ErrPiLocalRuntimeProbeBindingChanged) ||
		errors.Is(err, piadapter.ErrPiMetadataProcessTimeout) ||
		errors.Is(err, piadapter.ErrPiMetadataProcessOutputTooLarge) ||
		errors.Is(err, piadapter.ErrPiMetadataProcessFailed) ||
		errors.Is(err, loomruntime.ErrPiMetadataStderr) ||
		errors.Is(err, loomruntime.ErrInvalidPiMetadataOutput) ||
		errors.Is(err, loomruntime.ErrPiMetadataOutputTooLarge) ||
		errors.Is(err, loomruntime.ErrDuplicatePiRuntimeModel) ||
		errors.Is(err, loomruntime.ErrRuntimeDiscoveryFailed) ||
		runtimeObservationInventoryFailure(err) ||
		runtimeObservationPlanFailure(err) ||
		errors.Is(err, app.ErrRuntimeObservationProjectionRefresh) ||
		errors.Is(err, app.ErrLocalRuntimeObservationDaemonMetadata) ||
		runtimeObservationWriteFailure(err)
}

func hasPiMetadataFailureCommand(err error) bool {
	found := false
	visitDaemonErrors(err, func(candidate error) {
		if _, ok := candidate.(interface {
			PiMetadataFailureCommand() loomruntime.PiMetadataCommand
		}); ok {
			found = true
		}
	})
	return found
}

func uniquePiMetadataFailureCommand(
	err error,
) (loomruntime.PiMetadataCommand, bool) {
	commands := make(map[loomruntime.PiMetadataCommand]struct{}, 2)
	visitDaemonErrors(err, func(candidate error) {
		if command, ok := candidate.(interface {
			PiMetadataFailureCommand() loomruntime.PiMetadataCommand
		}); ok {
			commands[command.PiMetadataFailureCommand()] = struct{}{}
		}
	})
	if len(commands) != 1 {
		return "", false
	}
	for command := range commands {
		switch command {
		case loomruntime.PiMetadataVersion,
			loomruntime.PiMetadataListModels:
			return command, true
		default:
			return "", false
		}
	}
	return "", false
}

func visitDaemonErrors(err error, visit func(error)) {
	if err == nil {
		return
	}
	visit(err)
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			visitDaemonErrors(child, visit)
		}
		return
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		visitDaemonErrors(wrapped.Unwrap(), visit)
	}
}

func piMetadataObserverFailureReason(
	command loomruntime.PiMetadataCommand,
	err error,
) string {
	prefix := ""
	switch command {
	case loomruntime.PiMetadataVersion:
		prefix = "observer_version_"
	case loomruntime.PiMetadataListModels:
		prefix = "observer_models_"
	default:
		return "observer_unknown"
	}
	reasons := make(map[string]struct{}, 2)
	add := func(matches bool, reason string) {
		if matches {
			reasons[reason] = struct{}{}
		}
	}
	add(errors.Is(err, piadapter.ErrPiMetadataBindingChanged),
		"observer_metadata_binding")
	add(errors.Is(err, piadapter.ErrPiMetadataProcessTimeout),
		prefix+"timeout")
	add(errors.Is(err, piadapter.ErrPiMetadataProcessOutputTooLarge),
		prefix+"output_limit")
	add(errors.Is(err, piadapter.ErrPiMetadataProcessFailed),
		prefix+"process")
	add(errors.Is(err, loomruntime.ErrPiMetadataStderr), prefix+"stderr")
	add(errors.Is(err, loomruntime.ErrDuplicatePiRuntimeModel) &&
		command == loomruntime.PiMetadataListModels,
		"observer_models_duplicate")
	add(errors.Is(err, loomruntime.ErrInvalidPiMetadataOutput) ||
		errors.Is(err, loomruntime.ErrPiMetadataOutputTooLarge),
		prefix+"output")
	if len(reasons) != 1 {
		return "observer_unknown"
	}
	for reason := range reasons {
		return reason
	}
	return "observer_unknown"
}

func runtimeObservationWriteFailure(err error) bool {
	return errors.Is(err, app.ErrRuntimeDiscoveryCommitInputFailed) ||
		errors.Is(err, app.ErrRuntimeDiscoveryCommitResultMismatch) ||
		errors.Is(err, app.ErrRuntimeStatusCommitInputFailed) ||
		errors.Is(err, app.ErrRuntimeStatusCommitResultMismatch) ||
		errors.Is(err, app.ErrInvalidRuntimeObservationWriteRun) ||
		errors.Is(err, state.ErrInvalidRuntimeDiscoveryCommitInput) ||
		errors.Is(err, state.ErrInvalidRuntimeDiscoveryCommitSource) ||
		errors.Is(err, state.ErrRuntimeDiscoveryCommitResultMismatch) ||
		errors.Is(err, state.ErrRuntimeDiscoveryCommitDigestMismatch) ||
		errors.Is(err, state.ErrEmptyRuntimeDiscoveryCommit) ||
		errors.Is(err, state.ErrInvalidRuntimeStatusCommitInput) ||
		errors.Is(err, state.ErrInvalidRuntimeStatusCommitSource) ||
		errors.Is(err, state.ErrRuntimeStatusCommitResultMismatch) ||
		errors.Is(err, state.ErrRuntimeStatusCommitDigestMismatch) ||
		errors.Is(err, state.ErrEmptyRuntimeStatusCommit) ||
		runtimeObservationJournalWriteFailure(err)
}

func runtimeObservationJournalWriteFailure(err error) bool {
	return errors.Is(err, journal.ErrInvalidEvent) ||
		errors.Is(err, journal.ErrUnsupportedVersion) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrInvalidEventBatch) ||
		errors.Is(err, journal.ErrEventBatchTooLarge) ||
		errors.Is(err, journal.ErrDuplicateBatchIdempotencyKey) ||
		errors.Is(err, journal.ErrDuplicateBatchStreamSequence) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) ||
		errors.Is(err, journal.ErrStreamHeadConflict)
}

func runtimeObservationPlanFailure(err error) bool {
	return errors.Is(err, app.ErrInvalidRuntimeObservationWritePlan) ||
		errors.Is(err, app.ErrInvalidRuntimeObservationWriteRun) ||
		errors.Is(err, app.ErrInvalidConfiguredRuntimeObservationRun) ||
		errors.Is(err, app.ErrInvalidProjectedConfiguredRuntimeObservationRun) ||
		errors.Is(err, app.ErrInvalidProjectionSynchronizedRuntimeObservationRun) ||
		errors.Is(err, app.ErrInvalidTriggeredPreparedRuntimeObservationRun) ||
		errors.Is(err, app.ErrInvalidPreparedProjectedRuntimeObserver) ||
		errors.Is(err, app.ErrInvalidProjectedRuntimeStatusRun) ||
		errors.Is(err, app.ErrInvalidRuntimeStatusRun)
}

func runtimeObservationInventoryFailure(err error) bool {
	return errors.Is(err, loomruntime.ErrInvalidRuntimeProbe) ||
		errors.Is(err, loomruntime.ErrDuplicateRuntimeProbe) ||
		errors.Is(err, loomruntime.ErrInvalidRuntimeModel) ||
		errors.Is(err, loomruntime.ErrDuplicateRuntimeModel) ||
		errors.Is(err, loomruntime.ErrDuplicateRuntimeInstance) ||
		errors.Is(err, loomruntime.ErrInvalidRuntimeInstance) ||
		errors.Is(err, loomruntime.ErrInvalidRuntimeStatusReconciliation) ||
		errors.Is(err, loomruntime.ErrInvalidRuntimeStatusBaseline) ||
		errors.Is(err, loomruntime.ErrInvalidRuntimeStatusSource) ||
		errors.Is(err, loomruntime.ErrRuntimeStatusIdentityDrift) ||
		errors.Is(err, loomruntime.ErrRuntimeStatusCandidateDigest)
}

func containableProductObserverTimeout(err error) bool {
	if !errors.Is(err, piadapter.ErrPiMetadataProcessTimeout) {
		return false
	}
	_, ok := uniquePiMetadataFailureCommand(err)
	return ok && daemonErrorLeavesMatch(
		err,
		piadapter.ErrPiMetadataProcessTimeout,
		app.ErrLocalRuntimeObservationDaemonCycle,
		loomruntime.ErrRuntimeDiscoveryFailed,
	)
}

func daemonErrorLeavesMatch(err error, allowedTargets ...error) bool {
	if err == nil || len(allowedTargets) == 0 {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return daemonErrorMatchesAny(err, allowedTargets)
		}
		for _, child := range children {
			if !daemonErrorLeavesMatch(child, allowedTargets...) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		child := wrapped.Unwrap()
		if child != nil {
			return daemonErrorLeavesMatch(child, allowedTargets...)
		}
	}
	return daemonErrorMatchesAny(err, allowedTargets)
}

func daemonErrorMatchesAny(err error, allowedTargets []error) bool {
	for _, target := range allowedTargets {
		if target != nil && errors.Is(err, target) {
			return true
		}
	}
	return false
}

func joinProductDaemonErrors(values ...error) error {
	nonNil := make([]error, 0, len(values))
	for _, value := range values {
		if value != nil {
			nonNil = append(nonNil, value)
		}
	}
	switch len(nonNil) {
	case 0:
		return nil
	case 1:
		return nonNil[0]
	default:
		return errors.Join(nonNil...)
	}
}

type productDaemonRunner struct {
	observer  daemonRunner
	server    productIPCServer
	database  io.Closer
	setup     io.Closer
	execution io.Closer
	health    *productRuntimeObservationHealth

	mu      sync.Mutex
	running bool
	closed  bool
}

type productRuntimeObservationHealth struct {
	mu     sync.RWMutex
	reason string
}

func (health *productRuntimeObservationHealth) RuntimeObservationHealth() (
	string,
	bool,
) {
	if health == nil {
		return "", false
	}
	health.mu.RLock()
	defer health.mu.RUnlock()
	return health.reason, health.reason != ""
}

func (health *productRuntimeObservationHealth) recordTimeout(err error) {
	if health == nil {
		return
	}
	reason := observerFailureReason(err)
	if reason != "observer_version_timeout" &&
		reason != "observer_models_timeout" {
		return
	}
	health.mu.Lock()
	health.reason = reason
	health.mu.Unlock()
}

type productIPCServer interface {
	Serve(context.Context) error
	Ready() <-chan struct{}
	Close() error
}

type productShutdownError struct {
	stage string
	err   error
}

func (failure *productShutdownError) Error() string {
	return "product daemon shutdown failed: " + failure.stage
}

func (failure *productShutdownError) Unwrap() error {
	return failure.err
}

func newProductDaemonRunner(
	observer daemonRunner,
	statePath,
	socketPath string,
	setupConfigs ...productSetupRuntimeConfig,
) (*productDaemonRunner, error) {
	return newProductDaemonRunnerWithPreparedDecisions(
		observer,
		statePath,
		socketPath,
		app.PreparedMissionDecisions{},
		setupConfigs...,
	)
}

func newProductDaemonRunnerWithPreparedDecisions(
	observer daemonRunner,
	statePath,
	socketPath string,
	prepared app.PreparedMissionDecisions,
	setupConfigs ...productSetupRuntimeConfig,
) (_ *productDaemonRunner, resultErr error) {
	if observer == nil {
		return nil, newDaemonBuildFailure("build_observer", errors.New("invalid product daemon"))
	}
	database, err := openProductReadDatabase(statePath)
	if err != nil {
		_ = observer.Close()
		return nil, newDaemonBuildFailure("build_state", err)
	}
	var setupService *api.LocalProductSetupAPI
	var executionBundle io.Closer
	defer func() {
		if resultErr != nil {
			if executionBundle != nil {
				_ = executionBundle.Close()
			}
			if setupService != nil {
				_ = setupService.Close()
			}
			_ = database.Close()
			_ = observer.Close()
		}
	}()
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		return nil, newDaemonBuildFailure("build_state", err)
	}
	store := journal.NewStore(database)
	prepared, err = controlledMissionFixtureFromEnvironment(
		context.Background(),
		database,
		statePath,
		prepared,
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_state", err)
	}
	decisionBackend, err := app.NewPreparedMissionDecisionBackend(prepared)
	if err != nil {
		return nil, newDaemonBuildFailure("build_decision", err)
	}
	runtimeHealth := &productRuntimeObservationHealth{}
	service, err := api.NewLocalProductReadService(api.LocalProductReadConfig{
		Journal:    store,
		Projection: readModel,
		Now: func() time.Time {
			return time.Now().UTC()
		},
		Decisions:     decisionBackend,
		RuntimeHealth: runtimeHealth,
	})
	if err != nil {
		return nil, newDaemonBuildFailure("build_state", err)
	}
	setupConfig := productSetupRuntimeConfig{}
	if len(setupConfigs) == 1 {
		setupConfig = setupConfigs[0]
	}
	setupConfig.SocketPath = socketPath
	setupService, err = buildProductSetupService(
		database,
		store,
		readModel,
		setupConfig,
	)
	if err != nil {
		return nil, err
	}
	decisionService, err := app.NewLocalProductDecisionService(
		app.MissionDecisionConfig{Backend: decisionBackend},
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_decision", err)
	}
	decisionRouter, err := app.NewPreparedMissionExecutionDecisionRouter(
		decisionBackend,
		decisionService,
		prepared,
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_decision", err)
	}
	decisionAPI, err := api.NewLocalProductDecisionAPI(decisionService)
	if err != nil {
		return nil, newDaemonBuildFailure("build_ipc", err)
	}
	var executionAPI *api.LocalProductExecutionAPI
	var savedTeamMaterializer productSavedTeamMaterializer
	if setupConfig.Execution != nil {
		setupConfig.Execution.Decisions = decisionRouter
		executionAPI, executionBundle, err = buildProductMissionExecutionAPI(
			context.Background(),
			store,
			readModel,
			service,
			statePath,
			*setupConfig.Execution,
		)
		if err != nil {
			return nil, newDaemonBuildFailure("build_execution", err)
		}
		savedTeamMaterializer = &productSavedTeamMaterialization{
			store:      store,
			projection: readModel,
			now:        setupConfig.Execution.Now,
		}
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath:   socketPath,
		EffectiveUID: os.Geteuid(),
		BuildID:      localProductBuildID,
		Handler: localipc.HandlerFunc(
			localProductHandlerWithComposition(
				service,
				setupService,
				decisionAPI,
				executionAPI,
				savedTeamMaterializer,
			),
		),
	})
	if err != nil {
		return nil, newDaemonBuildFailure("build_ipc", err)
	}
	return &productDaemonRunner{
		observer:  observer,
		server:    server,
		database:  database,
		setup:     setupService,
		execution: executionBundle,
		health:    runtimeHealth,
	}, nil
}

type productMissionExecutionBundle struct {
	backend   *app.AuthoritativeMissionExecutionBackend
	evidence  *evidence.Store
	observers *api.LocalProductReadService

	mu     sync.Mutex
	closed bool
}

func (bundle *productMissionExecutionBundle) Close() error {
	if bundle == nil {
		return nil
	}
	bundle.mu.Lock()
	if bundle.closed {
		bundle.mu.Unlock()
		return nil
	}
	bundle.closed = true
	bundle.mu.Unlock()
	var backendErr error
	if bundle.backend != nil {
		backendErr = bundle.backend.Close()
	}
	var evidenceErr error
	if bundle.evidence != nil {
		evidenceErr = bundle.evidence.Close()
	}
	var observerErr error
	if bundle.observers != nil {
		observerErr = bundle.observers.CloseMissionExecutionObservers()
	}
	return errors.Join(backendErr, observerErr, evidenceErr)
}

type productMissionExecutionRunner struct {
	coordinator    *app.TeamCoordinator
	workAuthority  *work.Authority
	grantAuthority *authorization.Authority
	config         productMissionExecutionRuntimeConfig
	workspaceRoot  string
}

func (runner *productMissionExecutionRunner) Run(
	ctx context.Context,
	request app.TeamExecutionRequest,
) (result app.TeamExecutionResult, resultErr error) {
	if runner == nil || runner.coordinator == nil || ctx == nil {
		return app.TeamExecutionResult{}, app.ErrInvalidMissionExecution
	}
	factory := runner.config.ExecutorFactory
	if factory == nil {
		factory = func(
			factoryContext context.Context,
			workspaceRoot string,
			workAuthority *work.Authority,
			grantAuthority *authorization.Authority,
		) (productMissionExecutorPort, error) {
			return newProductMissionExecutor(
				factoryContext,
				runner.config,
				workspaceRoot,
				workAuthority,
				grantAuthority,
			)
		}
	}
	executor, err := factory(
		ctx,
		runner.workspaceRoot,
		runner.workAuthority,
		runner.grantAuthority,
	)
	if err != nil {
		return app.TeamExecutionResult{}, err
	}
	defer func() {
		closeContext, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()
		resultErr = errors.Join(resultErr, executor.Close(closeContext))
	}()
	prepared := request
	prepared.Nodes = append([]app.TeamNodeExecution{}, request.Nodes...)
	for index := range prepared.Nodes {
		prepared.Nodes[index].Executor = executor
	}
	prepared.Semantics = append([]app.TeamNodeSemantics{}, request.Semantics...)
	for index := range prepared.Semantics {
		if request.Semantics[index].VerifierExecution == nil {
			continue
		}
		verifier := *request.Semantics[index].VerifierExecution
		verifier.Executor = executor
		prepared.Semantics[index].VerifierExecution = &verifier
	}
	return runner.coordinator.Run(ctx, prepared)
}

type productMissionExecutor struct {
	supervisor *supervisor.Supervisor
	server     piadapter.PiLocalModelServer
}

const (
	productPiSourceOutputLimit = 4096
	productPiSourceRunLimit    = 16
)

type productPiRuntimeAdapter struct {
	delegate supervisor.RuntimeAdapter

	mu           sync.Mutex
	sourceOutput map[string][]byte
}

type productPiPromptDispatch struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          string `json:"kind"`
	Prompt        string `json:"prompt"`
}

type productPiVerifierDispatch struct {
	TeamInstanceID            string   `json:"team_instance_id"`
	PlanDigest                string   `json:"plan_digest"`
	LogicalNodeID             string   `json:"logical_node_id"`
	SourceAttemptNumber       int      `json:"source_attempt_number"`
	SourceWorkItemID          string   `json:"source_work_item_id"`
	SourceRunID               string   `json:"source_run_id"`
	SourceEvidenceDigest      string   `json:"source_evidence_digest"`
	SourceOutputSummaryDigest string   `json:"source_output_summary_digest"`
	AcceptanceContractDigest  string   `json:"acceptance_contract_digest"`
	Risk                      string   `json:"risk"`
	Criteria                  []string `json:"criteria"`
	AllowedReasonCodes        []string `json:"allowed_reason_codes"`
}

func newProductPiRuntimeAdapter(
	delegate supervisor.RuntimeAdapter,
) (*productPiRuntimeAdapter, error) {
	if delegate == nil || delegate.AdapterType() != "pi-cli" ||
		delegate.RuntimeInstanceID() == "" {
		return nil, app.ErrInvalidMissionExecution
	}
	return &productPiRuntimeAdapter{
		delegate: delegate, sourceOutput: make(map[string][]byte),
	}, nil
}

func (adapter *productPiRuntimeAdapter) AdapterType() string {
	if adapter == nil || adapter.delegate == nil {
		return ""
	}
	return adapter.delegate.AdapterType()
}

func (adapter *productPiRuntimeAdapter) RuntimeInstanceID() string {
	if adapter == nil || adapter.delegate == nil {
		return ""
	}
	return adapter.delegate.RuntimeInstanceID()
}

func (adapter *productPiRuntimeAdapter) Execute(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	if adapter == nil || adapter.delegate == nil || ctx == nil ||
		request.FrameSink == nil {
		return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
	}
	if _, ok := decodeProductPiPromptDispatch(request.Dispatch.Payload()); ok {
		return adapter.executeSource(ctx, request)
	}
	verifier, ok := decodeProductPiVerifierDispatch(request.Dispatch.Payload())
	if !ok {
		return supervisor.AdapterResult{}, app.ErrInvalidMissionExecution
	}
	return adapter.executeVerifier(ctx, request, verifier)
}

func (adapter *productPiRuntimeAdapter) executeSource(
	ctx context.Context,
	request supervisor.AdapterRequest,
) (supervisor.AdapterResult, error) {
	sink, err := newProductSourceCaptureSink(request)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	prepared := request
	prepared.FrameSink = sink
	result, err := adapter.delegate.Execute(ctx, prepared)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	if result.ExitCode() != 0 || !result.DispatchAcknowledged() ||
		!result.ResultAcknowledged() || !sink.succeeded ||
		len(sink.output) == 0 {
		return supervisor.AdapterResult{}, app.ErrMissionExecutionConflict
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if _, exists := adapter.sourceOutput[request.Binding.RunID]; exists ||
		len(adapter.sourceOutput) >= productPiSourceRunLimit {
		return supervisor.AdapterResult{}, app.ErrMissionExecutionBusy
	}
	adapter.sourceOutput[request.Binding.RunID] = bytes.Clone(sink.output)
	return result, nil
}

func (adapter *productPiRuntimeAdapter) executeVerifier(
	ctx context.Context,
	request supervisor.AdapterRequest,
	verifier productPiVerifierDispatch,
) (supervisor.AdapterResult, error) {
	adapter.mu.Lock()
	source := bytes.Clone(adapter.sourceOutput[verifier.SourceRunID])
	adapter.mu.Unlock()
	if len(source) == 0 {
		return supervisor.AdapterResult{}, app.ErrMissionExecutionConflict
	}
	prompt, err := productPiVerifierPrompt(verifier, source)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	payload, err := json.Marshal(productPiPromptDispatch{
		SchemaVersion: 1,
		Kind:          "pi_rpc_prompt",
		Prompt:        prompt,
	})
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	dispatch, err := productFrameWithPayload(request.Dispatch, payload)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	buffer, err := newProductVerifierBuffer(request.Binding, dispatch.MessageID())
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	prepared := request
	prepared.Dispatch = dispatch
	prepared.FrameSink = buffer
	delegateResult, err := adapter.delegate.Execute(ctx, prepared)
	if err != nil {
		return supervisor.AdapterResult{}, err
	}
	if delegateResult.ExitCode() != 0 ||
		!delegateResult.DispatchAcknowledged() ||
		!delegateResult.ResultAcknowledged() || !buffer.succeeded {
		return supervisor.AdapterResult{}, app.ErrMissionExecutionConflict
	}
	reasonCode := strings.TrimSpace(string(buffer.output))
	if !productVerifierReasonAllowed(reasonCode) {
		reasonCode = "insufficient_evidence"
	}
	return publishProductVerifierResult(
		ctx,
		request,
		reasonCode,
		delegateResult.Stderr(),
	)
}

type productSourceCaptureSink struct {
	delegate  supervisor.FrameSink
	stream    bridgev1.BoundRunStream
	dispatch  string
	output    []byte
	succeeded bool
}

func newProductSourceCaptureSink(
	request supervisor.AdapterRequest,
) (*productSourceCaptureSink, error) {
	stream, err := bridgev1.NewBoundRunStream(request.Binding)
	if err != nil {
		return nil, err
	}
	return &productSourceCaptureSink{
		delegate: request.FrameSink,
		stream:   stream,
		dispatch: request.Dispatch.MessageID(),
	}, nil
}

func (sink *productSourceCaptureSink) AcceptFrame(
	ctx context.Context,
	frame bridgev1.Frame,
) error {
	if sink == nil || sink.delegate == nil || ctx == nil || sink.succeeded {
		return app.ErrMissionExecutionConflict
	}
	candidate, err := bridgev1.AdvanceBoundRunStream(sink.stream, frame)
	if err != nil || !productPiFramePayloadValid(
		frame,
		sink.dispatch,
		&sink.output,
		&sink.succeeded,
	) {
		return errors.Join(app.ErrMissionExecutionConflict, err)
	}
	if err := sink.delegate.AcceptFrame(ctx, frame); err != nil {
		return err
	}
	sink.stream = candidate
	return nil
}

type productVerifierBuffer struct {
	stream    bridgev1.BoundRunStream
	dispatch  string
	output    []byte
	succeeded bool
}

func newProductVerifierBuffer(
	binding bridgev1.RunStreamBinding,
	dispatch string,
) (*productVerifierBuffer, error) {
	stream, err := bridgev1.NewBoundRunStream(binding)
	if err != nil {
		return nil, err
	}
	return &productVerifierBuffer{stream: stream, dispatch: dispatch}, nil
}

func (sink *productVerifierBuffer) AcceptFrame(
	ctx context.Context,
	frame bridgev1.Frame,
) error {
	if sink == nil || ctx == nil || sink.succeeded {
		return app.ErrMissionExecutionConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	candidate, err := bridgev1.AdvanceBoundRunStream(sink.stream, frame)
	if err != nil || !productPiFramePayloadValid(
		frame,
		sink.dispatch,
		&sink.output,
		&sink.succeeded,
	) {
		return errors.Join(app.ErrMissionExecutionConflict, err)
	}
	sink.stream = candidate
	return nil
}

func productPiFramePayloadValid(
	frame bridgev1.Frame,
	dispatch string,
	output *[]byte,
	succeeded *bool,
) bool {
	switch frame.Type() {
	case bridgev1.MessageAck:
		body, err := json.Marshal(struct {
			MessageID string `json:"message_id"`
		}{dispatch})
		return err == nil && bytes.Equal(body, frame.Payload())
	case bridgev1.MessageEvent:
		var event struct {
			Delta string `json:"delta"`
		}
		if !decodeProductExactJSON(frame.Payload(), &event) ||
			event.Delta == "" ||
			len(*output)+len(event.Delta) > productPiSourceOutputLimit {
			return false
		}
		*output = append(*output, event.Delta...)
		return true
	case bridgev1.MessageEvidence:
		return len(frame.Payload()) > 0
	case bridgev1.MessageResult:
		var terminal struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if !decodeProductExactJSON(frame.Payload(), &terminal) ||
			terminal.Status != "succeeded" || terminal.Reason != "" {
			return false
		}
		*succeeded = true
		return true
	default:
		return false
	}
}

func decodeProductPiPromptDispatch(
	payload []byte,
) (productPiPromptDispatch, bool) {
	var dispatch productPiPromptDispatch
	return dispatch, decodeProductExactJSON(payload, &dispatch) &&
		dispatch.SchemaVersion == 1 && dispatch.Kind == "pi_rpc_prompt" &&
		dispatch.Prompt != ""
}

func decodeProductPiVerifierDispatch(
	payload []byte,
) (productPiVerifierDispatch, bool) {
	var dispatch productPiVerifierDispatch
	if !decodeProductExactJSON(payload, &dispatch) ||
		dispatch.TeamInstanceID == "" || dispatch.LogicalNodeID == "" ||
		dispatch.SourceAttemptNumber <= 0 ||
		dispatch.SourceWorkItemID == "" || dispatch.SourceRunID == "" ||
		!validProductHex(dispatch.PlanDigest, 64) ||
		!validProductHex(dispatch.SourceEvidenceDigest, 64) ||
		!validProductHex(dispatch.SourceOutputSummaryDigest, 64) ||
		!validProductHex(dispatch.AcceptanceContractDigest, 64) ||
		(dispatch.Risk != "low" && dispatch.Risk != "medium" &&
			dispatch.Risk != "high") ||
		len(dispatch.Criteria) == 0 || len(dispatch.Criteria) > 64 ||
		len(dispatch.AllowedReasonCodes) != 3 ||
		dispatch.AllowedReasonCodes[0] != "criteria_satisfied" ||
		dispatch.AllowedReasonCodes[1] != "criteria_not_satisfied" ||
		dispatch.AllowedReasonCodes[2] != "insufficient_evidence" {
		return productPiVerifierDispatch{}, false
	}
	for _, criterion := range dispatch.Criteria {
		if criterion == "" || len(criterion) > 512 {
			return productPiVerifierDispatch{}, false
		}
	}
	return dispatch, true
}

func decodeProductExactJSON(payload []byte, destination any) bool {
	if len(payload) == 0 || destination == nil {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(destination) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	canonical, err := json.Marshal(destination)
	return err == nil && bytes.Equal(canonical, payload)
}

func productPiVerifierPrompt(
	dispatch productPiVerifierDispatch,
	source []byte,
) (string, error) {
	var prompt strings.Builder
	prompt.WriteString("Evaluate the authorized source result against every criterion.\n")
	prompt.WriteString("Return exactly one allowed verifier reason code and no other text: criteria_satisfied, criteria_not_satisfied, or insufficient_evidence.\nCriteria:\n")
	for _, criterion := range dispatch.Criteria {
		prompt.WriteString("- ")
		prompt.WriteString(criterion)
		prompt.WriteByte('\n')
	}
	prompt.WriteString("Authorized source result:\n")
	prompt.Write(source)
	result := prompt.String()
	if len(result) > 8192 {
		return "", app.ErrMissionExecutionConflict
	}
	return result, nil
}

func productFrameWithPayload(
	frame bridgev1.Frame,
	payload []byte,
) (bridgev1.Frame, error) {
	return bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: frame.MessageID(), CorrelationID: frame.CorrelationID(),
		WorkItemID: frame.WorkItemID(), RunID: frame.RunID(),
		ClaimGeneration:       frame.ClaimGeneration(),
		RuntimeInstanceID:     frame.RuntimeInstanceID(),
		SenderAgentInstanceID: frame.SenderAgentInstanceID(),
		Sequence:              frame.Sequence(), Type: frame.Type(),
		EmittedAt: frame.EmittedAt(), Payload: payload,
	})
}

func productVerifierReasonAllowed(reason string) bool {
	return reason == "criteria_satisfied" ||
		reason == "criteria_not_satisfied" ||
		reason == "insufficient_evidence"
}

func publishProductVerifierResult(
	ctx context.Context,
	request supervisor.AdapterRequest,
	reasonCode string,
	stderr []byte,
) (supervisor.AdapterResult, error) {
	status := "failed"
	reason := reasonCode
	if reasonCode == "criteria_satisfied" {
		status = "succeeded"
		reason = ""
	}
	digest := sha256.Sum256([]byte(reasonCode))
	records := []struct {
		kind    bridgev1.MessageType
		payload any
	}{
		{bridgev1.MessageAck, struct {
			MessageID string `json:"message_id"`
		}{request.Dispatch.MessageID()}},
		{bridgev1.MessageEvent, struct {
			Delta string `json:"delta"`
		}{reasonCode}},
		{bridgev1.MessageEvidence, struct {
			Kind   string `json:"kind"`
			SHA256 string `json:"sha256"`
			Bytes  int    `json:"bytes"`
		}{"verifier_reason_digest", hex.EncodeToString(digest[:]), len(reasonCode)}},
		{bridgev1.MessageResult, struct {
			Reason string `json:"reason"`
			Status string `json:"status"`
		}{reason, status}},
	}
	frames := make([]bridgev1.Frame, 0, len(records))
	for index, record := range records {
		payload, err := json.Marshal(record.payload)
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		frame, err := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID: productDeterministicUUID(
				request.Dispatch.MessageID(),
				string(record.kind),
			),
			CorrelationID:         request.Dispatch.CorrelationID(),
			WorkItemID:            request.Binding.WorkItemID,
			RunID:                 request.Binding.RunID,
			ClaimGeneration:       request.Binding.ClaimGeneration,
			RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
			SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
			Sequence:              int64(index + 2),
			Type:                  record.kind,
			EmittedAt:             request.Dispatch.EmittedAt(),
			Payload:               payload,
		})
		if err != nil {
			return supervisor.AdapterResult{}, err
		}
		if err := request.FrameSink.AcceptFrame(ctx, frame); err != nil {
			return supervisor.AdapterResult{}, err
		}
		frames = append(frames, frame)
	}
	return supervisor.NewAdapterResult(supervisor.AdapterResultInput{
		InboundFrames: frames, Stderr: stderr, ExitCode: 0,
		DispatchAcknowledged: true, ResultAcknowledged: true,
	})
}

func productDeterministicUUID(parts ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	digest[6] = digest[6]&0x0f | 0x40
	digest[8] = digest[8]&0x3f | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16],
	)
}

func (executor *productMissionExecutor) Execute(
	ctx context.Context,
	input supervisor.ExecuteInput,
) (supervisor.Outcome, error) {
	if executor == nil || executor.supervisor == nil {
		return supervisor.Outcome{}, app.ErrInvalidMissionExecution
	}
	return executor.supervisor.Execute(ctx, input)
}

func (executor *productMissionExecutor) Close(ctx context.Context) error {
	if executor == nil || executor.server == nil {
		return nil
	}
	return executor.server.Close(ctx)
}

func newProductMissionExecutor(
	ctx context.Context,
	config productMissionExecutionRuntimeConfig,
	workspaceRoot string,
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
) (*productMissionExecutor, error) {
	if ctx == nil || config.LocalModelCatalog == nil ||
		workAuthority == nil || grantAuthority == nil {
		return nil, app.ErrInvalidMissionExecution
	}
	server, err := piadapter.StartPiLocalModelServer(
		ctx,
		piadapter.PiLocalModelServerConfig{
			PrivateRoot:    config.LocalModelCatalog.PrivateRoot,
			ExecutablePath: config.LocalModelCatalog.ExecutablePath,
			ModelPath:      config.LocalModelCatalog.ModelPath,
			Host:           "127.0.0.1", Port: 18427,
			StartupTimeout: 60 * time.Second,
			CancelGrace:    3 * time.Second,
		},
	)
	if err != nil {
		return nil, err
	}
	closeServer := func(base error) (*productMissionExecutor, error) {
		closeContext, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()
		return nil, errors.Join(base, server.Close(closeContext))
	}
	piExecutable, err := resolveProductPiExecutable(config.RuntimeSearchPaths)
	if err != nil {
		return closeServer(err)
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	adapter, err := piadapter.NewPiRPCBridgeAdapter(
		piadapter.PiRPCBridgeAdapterConfig{
			Execution: piadapter.PiExecutionAdapterConfig{
				ExecutablePath:    piExecutable,
				RuntimeInstanceID: config.RuntimeInstanceID,
				RuntimeSearchPaths: append(
					[]string{},
					config.RuntimeSearchPaths...,
				),
				CancelGrace: 3 * time.Second,
				Now:         now,
				Random:      rand.Reader,
			},
			ProviderID:        "loom-local",
			ModelID:           "qwen2.5-coder-1.5b-instruct-q4-k-m",
			BaseURL:           server.BaseURL(),
			MaxAssistantBytes: 16384,
		},
	)
	if err != nil {
		return closeServer(err)
	}
	productAdapter, err := newProductPiRuntimeAdapter(adapter)
	if err != nil {
		return closeServer(err)
	}
	managed, err := supervisor.New(
		supervisor.Config{
			WorkspaceRoot:  workspaceRoot,
			CleanupTimeout: 5 * time.Second,
		},
		workAuthority,
		grantAuthority,
		productAdapter,
	)
	if err != nil {
		return closeServer(err)
	}
	return &productMissionExecutor{supervisor: managed, server: server}, nil
}

func resolveProductPiExecutable(searchPaths []string) (string, error) {
	for _, searchPath := range searchPaths {
		if !filepath.IsAbs(searchPath) || filepath.Clean(searchPath) != searchPath {
			return "", app.ErrInvalidMissionExecution
		}
		candidate := filepath.Join(searchPath, "pi")
		if _, err := os.Lstat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", app.ErrInvalidMissionExecution
}

func buildProductMissionExecutionAPI(
	ctx context.Context,
	store *journal.Store,
	readModel *projection.Projection,
	readService *api.LocalProductReadService,
	statePath string,
	config productMissionExecutionRuntimeConfig,
) (_ *api.LocalProductExecutionAPI, _ io.Closer, resultErr error) {
	if ctx == nil || store == nil || readModel == nil || readService == nil ||
		config.LocalModelCatalog == nil ||
		config.RuntimeInstanceID == "" ||
		len(config.RuntimeSearchPaths) == 0 ||
		!filepath.IsAbs(statePath) {
		return nil, nil, app.ErrInvalidMissionExecution
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if current := now(); current.IsZero() || current.Location() != time.UTC {
		return nil, nil, app.ErrInvalidMissionExecution
	}
	executionRoot := filepath.Join(filepath.Dir(statePath), "execution")
	workspaceRoot := filepath.Join(executionRoot, "workspaces")
	sourcePath := filepath.Join(executionRoot, "source")
	evidenceRoot := filepath.Join(executionRoot, "evidence")
	for _, path := range []string{
		executionRoot,
		workspaceRoot,
		sourcePath,
		evidenceRoot,
	} {
		if err := ensureProductExecutionDirectory(path); err != nil {
			return nil, nil, err
		}
	}
	evidenceStore, err := evidence.NewStore(evidenceRoot)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = evidenceStore.Close()
		}
	}()
	workAuthority, err := work.NewAuthority(store, now, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	if err := workAuthority.InitializeRunIdentityIndex(ctx); err != nil {
		return nil, nil, err
	}
	grantAuthority, err := authorization.NewAuthority(
		store,
		workAuthority,
		now,
		rand.Reader,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := grantAuthority.InitializeGrantIdentityIndex(ctx); err != nil {
		return nil, nil, err
	}
	if err := readModel.Rebuild(ctx); err != nil {
		return nil, nil, err
	}
	coordinator, err := app.NewTeamCoordinator(
		workAuthority,
		grantAuthority,
		readModel,
		evidenceStore,
	)
	if err != nil {
		return nil, nil, err
	}
	bindings, err := app.NewProjectionMissionExecutionBindingSource(readModel)
	if err != nil {
		return nil, nil, err
	}
	compiler, err := app.NewBuiltInMissionExecutionCompiler(
		app.BuiltInMissionExecutionCompilerConfig{
			Bindings: bindings, SourcePath: sourcePath,
			ObserverFactory: readService,
			Now:             now,
		},
	)
	if err != nil {
		return nil, nil, err
	}
	state, err := app.NewProjectionMissionExecutionState(readModel)
	if err != nil {
		return nil, nil, err
	}
	runner := &productMissionExecutionRunner{
		coordinator:    coordinator,
		workAuthority:  workAuthority,
		grantAuthority: grantAuthority,
		config:         config,
		workspaceRoot:  workspaceRoot,
	}
	backend, err := app.NewAuthoritativeMissionExecutionBackend(
		app.AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler, Runner: runner,
			Decisions:         config.Decisions,
			VisibilityTimeout: 5 * time.Second,
			Now:               now,
		},
	)
	if err != nil {
		return nil, nil, err
	}
	bundle := &productMissionExecutionBundle{
		backend:   backend,
		evidence:  evidenceStore,
		observers: readService,
	}
	if err := backend.ResumeProjectedMissions(ctx); err != nil {
		_ = bundle.Close()
		return nil, nil, err
	}
	service, err := app.NewLocalProductExecutionService(
		app.LocalProductExecutionConfig{Backend: backend},
	)
	if err != nil {
		_ = bundle.Close()
		return nil, nil, err
	}
	executionAPI, err := api.NewLocalProductExecutionAPI(service)
	if err != nil {
		_ = bundle.Close()
		return nil, nil, err
	}
	return executionAPI, bundle, nil
}

func ensureProductExecutionDirectory(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return app.ErrInvalidMissionExecution
	}
	if err := os.Mkdir(path, 0o700); err != nil &&
		!errors.Is(err, os.ErrExist) {
		return err
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path {
		return app.ErrInvalidMissionExecution
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return app.ErrInvalidMissionExecution
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return app.ErrInvalidMissionExecution
	}
	return nil
}

func controlledMissionFixtureFromEnvironment(
	ctx context.Context,
	database *sql.DB,
	statePath string,
	prepared app.PreparedMissionDecisions,
) (app.PreparedMissionDecisions, error) {
	manifestPath := os.Getenv(
		controlledMissionFixtureManifestEnvironment,
	)
	if manifestPath == "" {
		return prepared, nil
	}
	if len(prepared.Authorizations) != 0 ||
		len(prepared.Reviews) != 0 ||
		len(prepared.Recoveries) != 0 {
		return app.PreparedMissionDecisions{}, errors.New(
			"conflicting controlled mission fixture",
		)
	}
	manifest, authoritativeTime, err :=
		readControlledMissionFixtureManifest(
			manifestPath,
			statePath,
		)
	if err != nil {
		return app.PreparedMissionDecisions{}, err
	}
	return app.BuildControlledMissionDecisionFixture(
		ctx,
		app.ControlledMissionDecisionFixtureConfig{
			Database:          database,
			ArtifactRoot:      manifest.ArtifactRoot,
			AuthoritativeTime: authoritativeTime,
			FixtureID:         manifest.FixtureID,
		},
	)
}

func readControlledMissionFixtureManifest(
	manifestPath, statePath string,
) (controlledMissionFixtureManifest, time.Time, error) {
	if !filepath.IsAbs(manifestPath) ||
		!filepath.IsAbs(statePath) {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture path")
	}
	canonicalManifest, err := filepath.EvalSymlinks(manifestPath)
	if err != nil || canonicalManifest != manifestPath {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture manifest")
	}
	info, err := os.Lstat(manifestPath)
	if err != nil ||
		!info.Mode().IsRegular() ||
		info.Mode().Perm() != 0o600 ||
		info.Size() <= 0 ||
		info.Size() > 64<<10 {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture manifest")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture owner")
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil || len(content) == 0 || len(content) > 64<<10 {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture manifest")
	}
	var manifest controlledMissionFixtureManifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil ||
		decoder.Decode(&struct{}{}) != io.EOF {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture manifest")
	}
	canonicalState, err := filepath.EvalSymlinks(statePath)
	if err != nil || canonicalState != statePath ||
		manifest.StatePath != statePath ||
		manifest.SchemaVersion != 1 ||
		manifest.Purpose !=
			"p2a-w2-mission-workbench-controlled-live" ||
		manifest.AttemptID == "" ||
		manifest.FixtureID == "" ||
		!validProductHex(manifest.SourceCommit, 40) {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture identity")
	}
	attemptRoot := filepath.Dir(filepath.Dir(statePath))
	if filepath.Base(attemptRoot) != manifest.AttemptID ||
		manifestPath != filepath.Join(
			attemptRoot,
			"manifest",
			"mission-fixture.json",
		) ||
		manifest.ArtifactRoot != filepath.Join(
			attemptRoot,
			"artifacts",
			"mission-fixture",
		) {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture scope")
	}
	artifactParent := filepath.Dir(manifest.ArtifactRoot)
	for _, directory := range []string{
		attemptRoot,
		filepath.Dir(manifestPath),
		filepath.Dir(statePath),
		artifactParent,
	} {
		if !validControlledProductDirectory(directory) {
			return controlledMissionFixtureManifest{}, time.Time{},
				errors.New("invalid controlled mission fixture directory")
		}
	}
	if !validControlledProductFile(statePath, 0o600) {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture state")
	}
	canonicalArtifactParent, err := filepath.EvalSymlinks(artifactParent)
	if err != nil || canonicalArtifactParent != artifactParent {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission artifact root")
	}
	if _, err := os.Lstat(manifest.ArtifactRoot); !errors.Is(
		err,
		os.ErrNotExist,
	) {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("controlled mission artifact root is not fresh")
	}
	authoritativeTime, err := time.Parse(
		time.RFC3339Nano,
		manifest.AuthoritativeTime,
	)
	if err != nil ||
		authoritativeTime.Location() != time.UTC ||
		authoritativeTime.After(time.Now().UTC().Add(time.Minute)) {
		return controlledMissionFixtureManifest{}, time.Time{},
			errors.New("invalid controlled mission fixture time")
	}
	return manifest, authoritativeTime, nil
}

func validControlledProductDirectory(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func validControlledProductFile(path string, mode os.FileMode) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid()
}

func validProductHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, current := range value {
		if current < '0' || current > '9' {
			if current < 'a' || current > 'f' {
				return false
			}
		}
	}
	return true
}

type productSetupProjectionProbe struct {
	id           string
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
	return probe.id
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
	store      *journal.Store
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

type productCredentialOperationStatus struct {
	metadata  credentials.MetadataResult
	commandID string
}

type productCredentialOperationStatusSource interface {
	CredentialOperationStatus(
		context.Context,
		string,
	) (productCredentialOperationStatus, error)
}

func (source productCredentialStatusSource) CredentialOperationStatus(
	ctx context.Context,
	providerID string,
) (productCredentialOperationStatus, error) {
	metadata, err := source.CredentialStatus(ctx, providerID)
	if err != nil || source.store == nil {
		return productCredentialOperationStatus{},
			credentials.ErrCredentialStoreUnavailable
	}
	events, err := source.store.ReadStream(
		ctx,
		"provider-credential/"+providerID,
	)
	if err != nil || len(events) == 0 {
		return productCredentialOperationStatus{},
			credentials.ErrCredentialStoreUnavailable
	}
	event := events[len(events)-1]
	const prefix = "local-product-setup/"
	if event.StreamID != "provider-credential/"+providerID ||
		event.Seq != metadata.Revision ||
		!strings.HasPrefix(event.IdempotencyKey, prefix) ||
		len(event.IdempotencyKey) <= len(prefix) {
		return productCredentialOperationStatus{},
			credentials.ErrCredentialStoreUnavailable
	}
	return productCredentialOperationStatus{
		metadata: metadata,
		commandID: strings.TrimPrefix(
			event.IdempotencyKey,
			prefix,
		),
	}, nil
}

type productCredentialMutator struct {
	mu       sync.Mutex
	status   productCredentialOperationStatusSource
	delegate app.CredentialMutator
}

func (mutator *productCredentialMutator) Configure(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if mutator == nil || mutator.delegate == nil || ctx == nil {
		return credentials.MetadataResult{},
			credentials.ErrCredentialMetadataConflict
	}
	mutator.mu.Lock()
	defer mutator.mu.Unlock()
	return mutator.delegate.Configure(ctx, command)
}

func (mutator *productCredentialMutator) Verify(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if mutator == nil ||
		mutator.status == nil ||
		mutator.delegate == nil ||
		ctx == nil {
		return credentials.MetadataResult{},
			credentials.ErrCredentialMetadataConflict
	}
	mutator.mu.Lock()
	defer mutator.mu.Unlock()
	current, err := mutator.status.CredentialOperationStatus(
		ctx,
		command.ProviderID,
	)
	if err != nil ||
		current.metadata.ProviderID != command.ProviderID ||
		current.metadata.CredentialReference != command.CredentialReference {
		return credentials.MetadataResult{},
			credentials.ErrCredentialMetadataConflict
	}
	if current.metadata.Revision == command.ExpectedRevision {
		return mutator.delegate.Verify(ctx, command)
	}
	if current.metadata.Revision == command.ExpectedRevision+1 &&
		current.commandID == command.CommandID &&
		(current.metadata.Status == credentials.CredentialVerified ||
			current.metadata.Status == credentials.CredentialRejected) {
		return current.metadata, nil
	}
	return credentials.MetadataResult{},
		credentials.ErrCredentialMetadataConflict
}

func (mutator *productCredentialMutator) Replace(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if mutator == nil || mutator.delegate == nil || ctx == nil {
		return credentials.MetadataResult{},
			credentials.ErrCredentialMetadataConflict
	}
	mutator.mu.Lock()
	defer mutator.mu.Unlock()
	return mutator.delegate.Replace(ctx, command)
}

func (mutator *productCredentialMutator) Revoke(
	ctx context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	if mutator == nil || mutator.delegate == nil || ctx == nil {
		return credentials.MetadataResult{},
			credentials.ErrCredentialMetadataConflict
	}
	mutator.mu.Lock()
	defer mutator.mu.Unlock()
	return mutator.delegate.Revoke(ctx, command)
}

func buildProductSetupService(
	database *sql.DB,
	store *journal.Store,
	readModel *projection.Projection,
	config productSetupRuntimeConfig,
) (*api.LocalProductSetupAPI, error) {
	if database == nil || store == nil || readModel == nil {
		return nil, newDaemonBuildFailure("build_state", errors.New("setup state unavailable"))
	}
	catalog, err := productSetupCatalogForView(
		context.Background(),
		readModel.GlobalReadView(),
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_setup_runtime", err)
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		return nil, newDaemonBuildFailure("build_state", err)
	}
	keychain := config.CredentialStore
	if keychain == nil {
		executablePath, executableErr := os.Executable()
		if executableErr != nil || config.SocketPath == "" {
			return nil, newDaemonBuildFailure("build_setup_credential", errors.Join(executableErr, errors.New("setup credential boundary unavailable")))
		}
		keychain, err = credentials.NewProductKeychainStore(
			executablePath,
			config.SocketPath,
		)
	}
	if err != nil {
		return nil, newDaemonBuildFailure("build_setup_credential", err)
	}
	verifier, err := provider.NewSystemMiniMaxCredentialVerifier(
		5*time.Second,
		64*1024,
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_setup_provider", err)
	}
	broker, err := credentials.NewCredentialBroker(
		credentials.CredentialBrokerConfig{
			Store:     keychain,
			Verifier:  verifier,
			Committer: writer,
		},
	)
	if err != nil {
		return nil, newDaemonBuildFailure("build_setup_credential", err)
	}
	var (
		native          *provider.CodexNativeAuthObserver
		nativeConnector app.NativeAuthConnector
	)
	if config.CodexExecutable != "" {
		resolvedCodexExecutable, resolveErr := provider.ResolveCodexNativeExecutable(
			config.CodexExecutable,
		)
		if resolveErr != nil {
			return nil, newDaemonBuildFailure("build_setup_native_auth", resolveErr)
		}
		native, err = provider.NewCodexNativeAuthObserver(
			provider.CodexNativeAuthConfig{
				ExecutablePath: resolvedCodexExecutable,
				Timeout:        5 * time.Second,
				MaxOutputBytes: 4096,
				Runner:         provider.NewSystemCodexStatusRunner(),
			},
		)
		if err != nil {
			return nil, newDaemonBuildFailure("build_setup_native_auth", err)
		}
		controller, controllerErr := provider.NewSystemCodexLoginController(
			provider.CodexLoginControllerConfig{
				ExecutablePath: resolvedCodexExecutable,
				Timeout:        10 * time.Minute,
			},
		)
		if controllerErr != nil {
			return nil, newDaemonBuildFailure("build_setup_native_auth", controllerErr)
		}
		nativeConnector = productNativeAuthConnector{
			controller: controller,
		}
	}
	credentialStatus := productCredentialStatusSource{
		projection: readModel,
		store:      store,
	}
	setup, err := app.NewLocalProductSetupService(
		app.LocalProductSetupConfig{
			Journal:             store,
			Projection:          readModel,
			Writer:              writer,
			Catalog:             catalog,
			CatalogSource:       productSetupCatalogSource{},
			Identity:            productSetupIdentity{},
			Now:                 func() time.Time { return time.Now().UTC() },
			NativeAuth:          productNativeAuthObserver{observer: native},
			NativeAuthConnector: nativeConnector,
			Credentials:         credentialStatus,
			CredentialMutator: &productCredentialMutator{
				status:   credentialStatus,
				delegate: broker,
			},
		},
	)
	if err != nil {
		if nativeConnector != nil {
			_ = nativeConnector.Close()
		}
		return nil, newDaemonBuildFailure("build_setup_runtime", err)
	}
	setupAPI, err := api.NewLocalProductSetupAPI(setup)
	if err != nil {
		_ = setup.Close()
		return nil, newDaemonBuildFailure("build_setup_runtime", err)
	}
	return setupAPI, nil
}

type productSetupCatalogSource struct{}

func (productSetupCatalogSource) CatalogForView(
	ctx context.Context,
	view projection.GlobalReadView,
) (app.LocalProductSetupCatalog, error) {
	return productSetupCatalogForView(ctx, view)
}

func productSetupCatalogForView(
	ctx context.Context,
	view projection.GlobalReadView,
) (app.LocalProductSetupCatalog, error) {
	if ctx == nil {
		return app.LocalProductSetupCatalog{},
			errors.New("setup runtime unavailable")
	}
	if err := ctx.Err(); err != nil {
		return app.LocalProductSetupCatalog{}, err
	}
	runtimes, _ := view.RuntimeInstances("", 64)
	observationsByProbe := make(map[string][]loomruntime.RuntimeObservation)
	for _, runtime := range runtimes {
		status, ok := productSetupRuntimeStatus(runtime.Status)
		if !ok {
			continue
		}
		if runtime.SourceProbeID == "" {
			return app.LocalProductSetupCatalog{},
				errors.New("setup runtime unavailable")
		}
		observationsByProbe[runtime.SourceProbeID] = append(
			observationsByProbe[runtime.SourceProbeID],
			loomruntime.RuntimeObservation{
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
			},
		)
	}
	probes := make([]loomruntime.RuntimeProbe, 0, len(observationsByProbe))
	for probeID, observations := range observationsByProbe {
		probes = append(probes, productSetupProjectionProbe{
			id:           probeID,
			observations: observations,
		})
	}
	discovery, err := loomruntime.DiscoverRuntime(
		ctx,
		probes,
	)
	if err != nil {
		return app.LocalProductSetupCatalog{},
			errors.New("setup runtime unavailable")
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
	return app.LocalProductSetupCatalog{
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
	}, nil
}

func (materializer *productSavedTeamMaterialization) MaterializeConfirmedTeam(
	ctx context.Context,
	confirmation app.BuilderConfirmation,
) (app.BuilderConfirmation, error) {
	if materializer == nil || materializer.store == nil ||
		materializer.projection == nil || ctx == nil ||
		confirmation.TeamDefinitionID == "" ||
		confirmation.TeamDefinitionVersion <= 0 ||
		confirmation.TeamDefinitionDigest == "" ||
		confirmation.Status != "active" ||
		confirmation.TeamInstanceCreated || confirmation.RunCreated {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	if err := ctx.Err(); err != nil {
		return app.BuilderConfirmation{}, err
	}
	now := materializer.now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	occurredAt := now()
	if occurredAt.IsZero() || occurredAt.Location() != time.UTC ||
		occurredAt.UnixNano() <= 0 {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	if err := materializer.projection.Rebuild(ctx); err != nil {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	view := materializer.projection.GlobalReadView()
	record, ok := view.TeamDefinition(confirmation.TeamDefinitionID)
	if !ok || record.Version != confirmation.TeamDefinitionVersion ||
		record.DefinitionDigest != confirmation.TeamDefinitionDigest ||
		record.Status != confirmation.Status {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	setupCatalog, err := productSetupCatalogForView(ctx, view)
	if err != nil {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	definition, selections, err := productSavedTeamMaterializationInputs(
		record,
		setupCatalog,
	)
	if err != nil || definition.Digest() != confirmation.TeamDefinitionDigest {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	scope := agents.ScopeIdentity{ProjectID: productSavedTeamResolutionProjectID}
	if definition.Scope() == teams.TeamDefinitionScopeProject {
		scope = definition.ScopeIdentity()
	}
	binding, err := teams.BuildSavedTeamRuntimeBinding(
		[]teams.TeamDefinition{definition},
		definition.ID(),
		scope,
		setupCatalog.AgentDefinitions,
		setupCatalog.RuntimeProfiles,
		setupCatalog.RuntimeDiscovery,
		selections,
	)
	if err != nil {
		return app.BuilderConfirmation{}, fmt.Errorf(
			"%w: saved-Team binding: %v",
			app.ErrInvalidLocalProductSetup,
			err,
		)
	}
	resolutionCatalog := teams.TeamResolutionCatalogInput{
		AgentDefinitions:       append([]agents.AgentDefinition{}, setupCatalog.AgentDefinitions...),
		RuntimeProfiles:        append([]loomruntime.RuntimeProfile{}, setupCatalog.RuntimeProfiles...),
		TeamDefinitions:        []teams.TeamDefinition{definition},
		MainAgentDefinitionIDs: []string{definition.MainAgentDefinitionID()},
		DefaultMainAgentID:     definition.MainAgentDefinitionID(),
	}
	if definition.Scope() == teams.TeamDefinitionScopeProject {
		resolutionCatalog.ProjectDefaultTeamID = definition.ID()
	} else {
		resolutionCatalog.ReusableDefaultTeamID = definition.ID()
	}
	intent := mode.Intent{
		Trigger:  mode.TriggerSelectTeam,
		TargetID: definition.ID(),
		Text:     "materialize confirmed saved Team",
	}
	plan, err := teams.BuildSavedTeamInstantiationPlan(
		intent,
		scope,
		resolutionCatalog,
		binding,
		setupCatalog.RuntimeDiscovery,
		selections,
	)
	if err != nil {
		return app.BuilderConfirmation{}, fmt.Errorf(
			"%w: saved-Team plan: %v",
			app.ErrInvalidLocalProductSetup,
			err,
		)
	}
	identity, commit, err := newProductSavedTeamMaterializationIdentity(occurredAt)
	if err != nil {
		return app.BuilderConfirmation{}, err
	}
	records, err := teams.BuildSavedTeamInstanceRecordSet(
		plan,
		intent,
		scope,
		resolutionCatalog,
		binding,
		setupCatalog.RuntimeDiscovery,
		selections,
		identity,
	)
	if err != nil {
		return app.BuilderConfirmation{}, fmt.Errorf(
			"%w: saved-Team records: %v",
			app.ErrInvalidLocalProductSetup,
			err,
		)
	}
	committed, err := state.CommitSavedTeamInstanceRecordSet(
		ctx,
		materializer.store,
		records,
		plan,
		intent,
		scope,
		resolutionCatalog,
		binding,
		setupCatalog.RuntimeDiscovery,
		selections,
		identity,
		commit,
	)
	if err != nil || !committed.Committed() || committed.EventCount() != 2 {
		return app.BuilderConfirmation{}, fmt.Errorf(
			"%w: saved-Team commit: %v",
			app.ErrInvalidLocalProductSetup,
			err,
		)
	}
	if err := materializer.projection.Rebuild(ctx); err != nil {
		return app.BuilderConfirmation{}, app.ErrInvalidLocalProductSetup
	}
	confirmation.TeamInstanceCreated = true
	return confirmation, nil
}

func productSavedTeamMaterializationInputs(
	record projection.TeamDefinitionRecord,
	catalog app.LocalProductSetupCatalog,
) (teams.TeamDefinition, []teams.SavedTeamRuntimeSelection, error) {
	roles := make([]teams.TeamDefinitionRole, len(record.Roles))
	for index, role := range record.Roles {
		roles[index] = teams.TeamDefinitionRole{
			Kind:              teams.TeamDefinitionRoleKind(role.Kind),
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfileID:  role.RuntimeProfileID,
			Responsibility:    role.Responsibility,
		}
	}
	definition, err := teams.BuildTeamDefinition(
		teams.TeamDefinitionInput{
			ID:      record.ID,
			Version: record.Version,
			Scope:   teams.TeamDefinitionScope(record.Scope),
			ScopeIdentity: agents.ScopeIdentity{
				ProjectID:    record.ScopeIdentity.ProjectID,
				GenerationID: record.ScopeIdentity.GenerationID,
			},
			Name:   record.Name,
			Status: teams.TeamDefinitionStatus(record.Status),
			Roles:  roles,
		},
		catalog.AgentDefinitions,
		catalog.RuntimeProfiles,
	)
	if err != nil || definition.Digest() != record.DefinitionDigest ||
		len(record.Configuration.RoleBindings) != len(roles) {
		return teams.TeamDefinition{}, nil, app.ErrInvalidLocalProductSetup
	}
	bindings := make(map[string]projection.TeamConfigurationRoleBinding, len(roles))
	for _, binding := range record.Configuration.RoleBindings {
		if _, duplicate := bindings[binding.AgentDefinitionID]; duplicate {
			return teams.TeamDefinition{}, nil, app.ErrInvalidLocalProductSetup
		}
		bindings[binding.AgentDefinitionID] = binding
	}
	profiles := make(map[string]loomruntime.RuntimeProfile, len(catalog.RuntimeProfiles))
	for _, profile := range catalog.RuntimeProfiles {
		profiles[profile.ID] = profile
	}
	selections := make([]teams.SavedTeamRuntimeSelection, len(roles))
	for index, role := range roles {
		binding, ok := bindings[role.AgentDefinitionID]
		profile, profileOK := profiles[role.RuntimeProfileID]
		if !ok || !profileOK ||
			binding.Kind != string(role.Kind) ||
			binding.RuntimeProfileID != role.RuntimeProfileID ||
			binding.ModelID != profile.ModelID ||
			binding.RuntimeInstanceID == "" {
			return teams.TeamDefinition{}, nil, app.ErrInvalidLocalProductSetup
		}
		selections[index] = teams.SavedTeamRuntimeSelection{
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeInstanceID: binding.RuntimeInstanceID,
		}
	}
	return definition, selections, nil
}

func newProductSavedTeamMaterializationIdentity(
	occurredAt time.Time,
) (teams.SavedTeamInstanceIdentityInput, state.SavedTeamCommitInput, error) {
	identitySource := productSetupIdentity{}
	next := func(kind string) (string, error) {
		value, err := identitySource.NextSetupID(kind)
		if err != nil {
			return "", app.ErrInvalidLocalProductSetup
		}
		return value, nil
	}
	values := make([]string, 7)
	for index, kind := range []string{
		"saved-team-request",
		"team-instance",
		"agent-instance",
		"team-event",
		"team-idempotency",
		"agent-event",
		"agent-idempotency",
	} {
		value, err := next(kind)
		if err != nil {
			return teams.SavedTeamInstanceIdentityInput{}, state.SavedTeamCommitInput{}, err
		}
		values[index] = value
	}
	return teams.SavedTeamInstanceIdentityInput{
			WorkRequestID:       values[0],
			TeamInstanceID:      values[1],
			MainAgentInstanceID: values[2],
			CreatedAt:           occurredAt.UnixNano(),
		}, state.SavedTeamCommitInput{
			TeamEventID:             values[3],
			TeamIdempotencyKey:      values[4],
			MainAgentEventID:        values[5],
			MainAgentIdempotencyKey: values[6],
			EmittedAt:               occurredAt,
		}, nil
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
		if containableProductObserverTimeout(outcome.err) {
			runner.health.recordTimeout(outcome.err)
			select {
			case serverErr := <-serverDone:
				return outcome.result, classifyProductDaemonFailure(
					"local_ipc",
					serverErr,
				)
			case <-ctx.Done():
				cancel()
				closeErr := runner.server.Close()
				serverErr := <-serverDone
				if serverErr != nil || closeErr != nil {
					return outcome.result, classifyProductDaemonFailure(
						"shutdown",
						errors.Join(serverErr, closeErr),
					)
				}
				return outcome.result, ctx.Err()
			}
		}
		cancel()
		closeErr := runner.server.Close()
		serverErr := <-serverDone
		if outcome.err != nil &&
			!errors.Is(outcome.err, context.Canceled) &&
			!errors.Is(outcome.err, context.DeadlineExceeded) {
			return outcome.result, classifyProductDaemonFailure(
				"observer",
				joinProductDaemonErrors(
					outcome.err,
					serverErr,
					closeErr,
				),
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
				joinProductDaemonErrors(
					outcome.err,
					serverErr,
					closeErr,
				),
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
	var closeErr error
	closeErr = errors.Join(
		closeErr,
		closeProductDaemonStage("local_ipc", runner.server),
	)
	if runner.execution != nil {
		closeErr = errors.Join(
			closeErr,
			closeProductDaemonStage("execution", runner.execution),
		)
	}
	closeErr = errors.Join(
		closeErr,
		closeProductDaemonStage("observer", runner.observer),
	)
	closeErr = errors.Join(
		closeErr,
		closeProductDaemonStage("setup", runner.setup),
	)
	closeErr = errors.Join(
		closeErr,
		closeProductDaemonStage("database", runner.database),
	)
	return closeErr
}

func closeProductDaemonStage(stage string, closer io.Closer) error {
	if closer == nil {
		return &productShutdownError{
			stage: stage,
			err:   errors.New("missing shutdown owner"),
		}
	}
	if err := closer.Close(); err != nil {
		return &productShutdownError{stage: stage, err: err}
	}
	return nil
}

func localProductHandler(
	service *api.LocalProductReadService,
	setupServices ...*api.LocalProductSetupAPI,
) func(context.Context, localipc.Request) localipc.Response {
	var setup *api.LocalProductSetupAPI
	if len(setupServices) == 1 {
		setup = setupServices[0]
	}
	backend, err := app.NewPreparedMissionDecisionBackend(
		app.PreparedMissionDecisions{},
	)
	if err != nil {
		return localProductHandlerWithDecision(service, setup, nil)
	}
	decisionService, err := app.NewLocalProductDecisionService(
		app.MissionDecisionConfig{Backend: backend},
	)
	if err != nil {
		return localProductHandlerWithDecision(service, setup, nil)
	}
	decision, err := api.NewLocalProductDecisionAPI(decisionService)
	if err != nil {
		return localProductHandlerWithDecision(service, setup, nil)
	}
	return localProductHandlerWithDecision(service, setup, decision)
}

func localProductHandlerWithDecision(
	service *api.LocalProductReadService,
	setup *api.LocalProductSetupAPI,
	decision *api.LocalProductDecisionAPI,
	executionServices ...*api.LocalProductExecutionAPI,
) func(context.Context, localipc.Request) localipc.Response {
	var execution *api.LocalProductExecutionAPI
	if len(executionServices) == 1 {
		execution = executionServices[0]
	}
	return localProductHandlerWithComposition(
		service,
		setup,
		decision,
		execution,
		nil,
	)
}

func localProductHandlerWithComposition(
	service *api.LocalProductReadService,
	setup *api.LocalProductSetupAPI,
	decision *api.LocalProductDecisionAPI,
	execution *api.LocalProductExecutionAPI,
	savedTeamMaterializer productSavedTeamMaterializer,
) func(context.Context, localipc.Request) localipc.Response {
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
		if request.Method == "mission_decision" && decision == nil {
			return productErrorResponse(
				"state_unavailable",
				api.ErrInvalidLocalProductDecisionAPI,
			)
		}
		if request.Method == "mission_execution" && execution == nil {
			return productErrorResponse(
				"state_unavailable",
				api.ErrInvalidLocalProductExecutionAPI,
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
		case "mission_decision":
			var input app.MissionDecisionCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					app.ErrInvalidMissionDecision,
				)
			}
			if input.Operation == "read" {
				result, err := decision.ReadMissionDecision(ctx, input)
				if err != nil {
					return productServiceError(err)
				}
				return productResultResponse(result)
			}
			result, err := decision.DecideMission(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "mission_execution":
			var input app.MissionExecutionCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					app.ErrInvalidMissionExecution,
				)
			}
			result, err := execution.ExecuteMission(ctx, input)
			if err != nil {
				return productServiceError(err)
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
			if savedTeamMaterializer != nil {
				result, err = savedTeamMaterializer.MaterializeConfirmedTeam(
					ctx,
					result,
				)
				if err != nil {
					return productServiceError(err)
				}
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
			if request.Method == "credential_verify" &&
				!validProductCredentialOperationID(input.OperationID) ||
				request.Method != "credential_verify" && input.OperationID != "" {
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
				OperationID:         input.OperationID,
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
	OperationID         string `json:"operation_id,omitempty"`
	Secret              string `json:"secret"`
}

func validProductCredentialOperationID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' || value[14] != '4' ||
		!strings.Contains("89ab", string(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func clearProductSecret(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}

func decodeExactProductParams(data []byte, output any) error {
	if duplicate, err := scanProductJSONValue(
		json.NewDecoder(bytes.NewReader(data)),
	); err != nil || duplicate {
		return errors.New("invalid product params")
	}
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

func scanProductJSONValue(decoder *json.Decoder) (bool, error) {
	token, err := decoder.Token()
	if err != nil {
		return false, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false, nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return false, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return false, errors.New("invalid product params")
			}
			if _, exists := seen[key]; exists {
				return true, nil
			}
			seen[key] = struct{}{}
			duplicate, err := scanProductJSONValue(decoder)
			if err != nil || duplicate {
				return duplicate, err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return false, errors.New("invalid product params")
		}
	case '[':
		for decoder.More() {
			duplicate, err := scanProductJSONValue(decoder)
			if err != nil || duplicate {
				return duplicate, err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return false, errors.New("invalid product params")
		}
	default:
		return false, errors.New("invalid product params")
	}
	return false, nil
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
		errors.Is(err, api.ErrInvalidLocalProductDecisionAPI),
		errors.Is(err, api.ErrInvalidLocalProductExecutionAPI),
		errors.Is(err, app.ErrInvalidMissionDecision),
		errors.Is(err, app.ErrInvalidMissionExecution),
		errors.Is(err, app.ErrInvalidLocalProductSetup):
		return productErrorResponse("invalid_request", err)
	case errors.Is(err, api.ErrTeamTimelineNotFound),
		errors.Is(err, app.ErrBuilderNotFound):
		return productErrorResponse("not_found", err)
	case errors.Is(err, app.ErrBuilderConflict),
		errors.Is(err, app.ErrMissionDecisionConflict),
		errors.Is(err, app.ErrMissionExecutionConflict):
		return productErrorResponse("conflict", err)
	case errors.Is(err, app.ErrBuilderIncompatible):
		return productErrorResponse("incompatible", err)
	case errors.Is(err, app.ErrBuilderConfirmationRequired):
		return productErrorResponse("denied", err)
	case errors.Is(err, app.ErrNativeAuthConnectBusy),
		errors.Is(err, app.ErrMissionExecutionBusy):
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
