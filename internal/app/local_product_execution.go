package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const MissionExecutionSchemaVersion = 1

const (
	maxMissionExecutionTextBytes   = 4096
	maxMissionExecutionListItems   = 64
	maxMissionExecutionNodeCount   = 16
	maxAuthoritativeMissionFlights = 64
	missionExecutionPreflightTTL   = 5 * time.Minute
	missionExecutionPreflight      = "preflight"
	missionExecutionStart          = "start"
	missionExecutionControl        = "control"
	localPiProviderID              = "loom-local"
	localPiAdapterModelID          = "qwen2.5-coder-1.5b-instruct-q4-k-m"
)

var ErrInvalidMissionExecution = errors.New("invalid mission execution")

var ErrMissionExecutionConflict = errors.New("mission execution conflict")

var ErrMissionExecutionBusy = errors.New("mission execution busy")

type MissionExecutionCommand struct {
	SchemaVersion       int    `json:"schema_version"`
	Operation           string `json:"operation"`
	MissionID           string `json:"mission_id"`
	TeamInstanceID      string `json:"team_instance_id"`
	WorkPackageID       string `json:"work_package_id,omitempty"`
	WorkPackageDigest   string `json:"work_package_digest,omitempty"`
	Objective           string `json:"objective,omitempty"`
	ExpectedViewVersion string `json:"expected_view_version"`
	PreflightDigest     string `json:"preflight_digest,omitempty"`
	ControlAction       string `json:"control_action,omitempty"`
	ExecutionDigest     string `json:"execution_digest,omitempty"`
	LogicalNodeID       string `json:"logical_node_id,omitempty"`
	AttemptNumber       int    `json:"attempt_number,omitempty"`
	ClaimGeneration     int64  `json:"claim_generation,omitempty"`
	CorrelationID       string `json:"correlation_id"`
}

type MissionExecutionNodePreview struct {
	LogicalNodeID string   `json:"logical_node_id"`
	Title         string   `json:"title"`
	Role          string   `json:"role"`
	DependsOn     []string `json:"depends_on"`
	MaxAttempts   int      `json:"max_attempts"`
}

type MissionExecutionPreflight struct {
	SchemaVersion     int                           `json:"schema_version"`
	MissionID         string                        `json:"mission_id"`
	TeamInstanceID    string                        `json:"team_instance_id"`
	WorkPackageID     string                        `json:"work_package_id"`
	WorkPackageDigest string                        `json:"work_package_digest"`
	ViewVersion       string                        `json:"view_version"`
	PlanDigest        string                        `json:"plan_digest"`
	PreflightDigest   string                        `json:"preflight_digest"`
	ExpiresAt         string                        `json:"expires_at"`
	RuntimeInstanceID string                        `json:"runtime_instance_id"`
	RuntimeProfileID  string                        `json:"runtime_profile_id"`
	ModelID           string                        `json:"model_id"`
	AuthMode          string                        `json:"auth_mode"`
	CapacityAvailable int                           `json:"capacity_available"`
	BudgetStatus      string                        `json:"budget_status"`
	SideEffects       []string                      `json:"side_effects"`
	PermissionScopes  []string                      `json:"permission_scopes"`
	ApprovalPoints    []string                      `json:"approval_points"`
	Nodes             []MissionExecutionNodePreview `json:"nodes"`
}

type MissionExecutionResult struct {
	SchemaVersion   int    `json:"schema_version"`
	MissionID       string `json:"mission_id"`
	TeamInstanceID  string `json:"team_instance_id"`
	Status          string `json:"status"`
	ViewVersion     string `json:"view_version"`
	ExecutionDigest string `json:"execution_digest"`
}

type MissionExecutionBackend interface {
	PreflightMission(
		context.Context,
		MissionExecutionCommand,
	) (MissionExecutionPreflight, error)
	StartMission(
		context.Context,
		MissionExecutionCommand,
	) (MissionExecutionResult, error)
	ControlMission(
		context.Context,
		MissionExecutionCommand,
	) (MissionExecutionResult, error)
}

type MissionExecutionCompilation struct {
	Plan      teams.ExecutionPlan
	Preflight MissionExecutionPreflight
	Request   TeamExecutionRequest
}

type MissionExecutionCompiler interface {
	CompileMissionExecution(
		context.Context,
		MissionExecutionCommand,
	) (MissionExecutionCompilation, error)
}

type MissionExecutionReconstructor interface {
	ReconstructMissionExecution(
		context.Context,
		projection.TeamExecution,
	) (TeamExecutionRequest, error)
}

type MissionExecutionBinding struct {
	ViewVersion       string
	TeamInstanceID    string
	AgentInstanceID   string
	Profile           loomruntime.RuntimeProfile
	Instance          loomruntime.RuntimeInstance
	CapacityAvailable int
}

type MissionExecutionBindingSource interface {
	ResolveMissionExecutionBinding(
		context.Context,
		string,
	) (MissionExecutionBinding, error)
}

type missionExecutionRecoveryBindingSource interface {
	ResolveMissionExecutionRecoveryBinding(
		context.Context,
		string,
	) (MissionExecutionBinding, error)
}

type ProjectionMissionExecutionBindingSource struct {
	projection *projection.Projection
}

func NewProjectionMissionExecutionBindingSource(
	readModel *projection.Projection,
) (*ProjectionMissionExecutionBindingSource, error) {
	if readModel == nil {
		return nil, ErrInvalidMissionExecution
	}
	return &ProjectionMissionExecutionBindingSource{projection: readModel}, nil
}

func (source *ProjectionMissionExecutionBindingSource) ResolveMissionExecutionBinding(
	ctx context.Context,
	teamInstanceID string,
) (MissionExecutionBinding, error) {
	return source.resolveMissionExecutionBinding(ctx, teamInstanceID, false)
}

func (source *ProjectionMissionExecutionBindingSource) ResolveMissionExecutionRecoveryBinding(
	ctx context.Context,
	teamInstanceID string,
) (MissionExecutionBinding, error) {
	return source.resolveMissionExecutionBinding(ctx, teamInstanceID, true)
}

func (source *ProjectionMissionExecutionBindingSource) resolveMissionExecutionBinding(
	ctx context.Context,
	teamInstanceID string,
	recovery bool,
) (MissionExecutionBinding, error) {
	if source == nil || source.projection == nil || ctx == nil ||
		!validMissionExecutionText(teamInstanceID, 128) {
		return MissionExecutionBinding{}, ErrInvalidMissionExecution
	}
	if err := ctx.Err(); err != nil {
		return MissionExecutionBinding{}, err
	}
	view := source.projection.GlobalReadView()
	anchor, ok := view.TeamTimelineAnchor(teamInstanceID)
	if !ok || anchor.Kind != "saved_team" || !anchor.Confirmed ||
		!anchor.Executable || anchor.ReadOnly {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	team, ok := view.Team(teamInstanceID)
	if !ok || team.ID != teamInstanceID || team.SourceKind != "saved_team" ||
		team.State != "created" || team.TeamInstanceCount != 1 ||
		team.AgentInstanceCount != 1 || team.ActiveSubAgentCount != 0 ||
		!validSHA256(team.SourcePlanDigest) ||
		!validSHA256(team.SourceRecordSetDigest) {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	snapshot := source.projection.Snapshot()
	mainAgents := make([]projection.AgentInstance, 0, 1)
	for _, candidate := range snapshot.AgentInstances {
		if candidate.TeamInstanceID == teamInstanceID && candidate.IsMain {
			mainAgents = append(mainAgents, candidate)
		}
	}
	if len(mainAgents) != 1 {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	agent := mainAgents[0]
	if agent.ID == "" || agent.State != "created" ||
		agent.RuntimeProfileID != "loom-main-native" ||
		agent.RuntimeInstanceID == "" ||
		!agent.RuntimeBinding.Accepted ||
		agent.RuntimeBinding.ProfileID != agent.RuntimeProfileID ||
		agent.RuntimeBinding.InstanceID != agent.RuntimeInstanceID ||
		agent.SourcePlanDigest != team.SourcePlanDigest ||
		agent.SourceRecordSetDigest != team.SourceRecordSetDigest ||
		!validSHA256(agent.RuntimeDiscoveryDigest) {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	projectedRuntime, ok := view.RuntimeInstance(agent.RuntimeInstanceID)
	if !ok || projectedRuntime.ID != agent.RuntimeInstanceID ||
		projectedRuntime.AdapterType != "pi-cli" ||
		projectedRuntime.Status != string(loomruntime.RuntimeOnline) ||
		projectedRuntime.Capacity < 1 ||
		projectedRuntime.DiscoveryDigest != agent.RuntimeDiscoveryDigest ||
		len(projectedRuntime.ModelIDs) != 1 {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	providerID, adapterModelID, ok := parseLockedLocalPiModelIdentity(
		projectedRuntime.ModelIDs[0],
	)
	if !ok {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                   agent.RuntimeProfileID,
		AdapterType:          projectedRuntime.AdapterType,
		ProviderID:           providerID,
		ModelID:              adapterModelID,
		AuthMode:             loomruntime.AuthNative,
		RequiredCapabilities: []string{},
		Timeout:              5 * time.Minute,
	})
	if err != nil {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID:                projectedRuntime.ID,
		DeviceID:          projectedRuntime.DeviceID,
		AdapterType:       projectedRuntime.AdapterType,
		DisplayName:       projectedRuntime.DisplayName,
		ExecutableVersion: projectedRuntime.ExecutableVersion,
		Status:            loomruntime.RuntimeStatus(projectedRuntime.Status),
		ObservedCapabilities: append(
			[]string{},
			projectedRuntime.ObservedCapabilities...,
		),
		Capacity: projectedRuntime.Capacity,
	})
	if err != nil {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	available := instance.Capacity - view.ActiveRunCount(instance.ID)
	if available < 1 {
		if !recovery || !missionExecutionOwnsActiveCapacity(
			view,
			teamInstanceID,
			instance.ID,
		) {
			return MissionExecutionBinding{}, ErrMissionExecutionBusy
		}
		available = 1
	}
	return MissionExecutionBinding{
		ViewVersion: view.Version(), TeamInstanceID: teamInstanceID,
		AgentInstanceID: agent.ID, Profile: profile, Instance: instance,
		CapacityAvailable: available,
	}, nil
}

func parseLockedLocalPiModelIdentity(value string) (string, string, bool) {
	if strings.Count(value, "/") != 1 {
		return "", "", false
	}
	providerID, modelID, found := strings.Cut(value, "/")
	if !found || providerID != localPiProviderID ||
		modelID != localPiAdapterModelID {
		return "", "", false
	}
	return providerID, modelID, true
}

func missionExecutionOwnsActiveCapacity(
	view projection.GlobalReadView,
	teamInstanceID, runtimeInstanceID string,
) bool {
	execution, ok := view.TeamExecution(teamInstanceID)
	if !ok || appTerminalTeamStatus(execution.Status) {
		return false
	}
	for _, node := range execution.Nodes {
		for _, attempt := range node.Attempts {
			if attempt.RuntimeInstanceID == runtimeInstanceID &&
				attempt.ClaimGeneration > 0 &&
				attempt.Status != "succeeded" && attempt.Status != "failed" &&
				attempt.Status != "cancelled" {
				return true
			}
		}
	}
	return false
}

type BuiltInMissionExecutionCompilerConfig struct {
	Bindings        MissionExecutionBindingSource
	SourcePath      string
	OutputObserver  NodeOutputObserver
	ObserverFactory MissionExecutionObserverFactory
	Now             func() time.Time
}

type MissionExecutionObserverFactory interface {
	MissionExecutionObserver(
		context.Context,
		string,
	) (NodeOutputObserver, error)
}

type BuiltInMissionExecutionCompiler struct {
	bindings        MissionExecutionBindingSource
	sourcePath      string
	outputObserver  NodeOutputObserver
	observerFactory MissionExecutionObserverFactory
	now             func() time.Time
}

type compileOnlyMissionExecutor struct{}

func (compileOnlyMissionExecutor) Execute(
	context.Context,
	supervisor.ExecuteInput,
) (supervisor.Outcome, error) {
	return supervisor.Outcome{}, ErrInvalidTeamCoordinator
}

func NewBuiltInMissionExecutionCompiler(
	config BuiltInMissionExecutionCompilerConfig,
) (*BuiltInMissionExecutionCompiler, error) {
	if nilMissionExecutionInterface(config.Bindings) ||
		config.Now == nil ||
		config.OutputObserver != nil &&
			nilMissionExecutionInterface(config.OutputObserver) ||
		config.ObserverFactory != nil &&
			nilMissionExecutionInterface(config.ObserverFactory) ||
		config.OutputObserver != nil && config.ObserverFactory != nil ||
		!validMissionExecutionSourcePath(config.SourcePath) {
		return nil, ErrInvalidMissionExecution
	}
	now := config.Now()
	if now.IsZero() || now.Location() != time.UTC {
		return nil, ErrInvalidMissionExecution
	}
	return &BuiltInMissionExecutionCompiler{
		bindings: config.Bindings, sourcePath: config.SourcePath,
		outputObserver:  config.OutputObserver,
		observerFactory: config.ObserverFactory,
		now:             config.Now,
	}, nil
}

func (compiler *BuiltInMissionExecutionCompiler) CompileMissionExecution(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionCompilation, error) {
	if compiler == nil || ctx == nil ||
		(command.Operation != missionExecutionPreflight &&
			command.Operation != missionExecutionStart) ||
		!validMissionExecutionCommand(command, command.Operation) {
		return MissionExecutionCompilation{}, ErrInvalidMissionExecution
	}
	if err := ctx.Err(); err != nil {
		return MissionExecutionCompilation{}, err
	}
	binding, err := compiler.bindings.ResolveMissionExecutionBinding(
		ctx,
		command.TeamInstanceID,
	)
	if err != nil {
		return MissionExecutionCompilation{}, err
	}
	return compiler.compileMissionExecution(ctx, command, binding)
}

func (compiler *BuiltInMissionExecutionCompiler) compileMissionExecution(
	ctx context.Context,
	command MissionExecutionCommand,
	binding MissionExecutionBinding,
) (MissionExecutionCompilation, error) {
	if binding.ViewVersion != command.ExpectedViewVersion ||
		binding.TeamInstanceID != command.TeamInstanceID ||
		binding.AgentInstanceID == "" ||
		binding.CapacityAvailable < 1 ||
		binding.CapacityAvailable > binding.Instance.Capacity {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	validated, err := loomruntime.ValidateBinding(
		binding.Profile,
		binding.Instance,
	)
	if err != nil || !validated.Accepted ||
		validated.ProfileID != binding.Profile.ID ||
		validated.InstanceID != binding.Instance.ID {
		return MissionExecutionCompilation{}, errors.Join(
			ErrMissionExecutionConflict,
			err,
		)
	}
	workPackage, err := missionExecutionWorkPackage(command.WorkPackageID)
	if err != nil || workPackage.Digest() != command.WorkPackageDigest {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID:     "main",
			Title:             command.Objective,
			AgentInstanceID:   binding.AgentInstanceID,
			RuntimeInstanceID: binding.Instance.ID,
			Role:              teams.ExecutionRoleMain,
			DependsOn:         []string{},
			MaxAttempts:       2,
		}},
	})
	if err != nil {
		return MissionExecutionCompilation{}, errors.Join(
			ErrInvalidMissionExecution,
			err,
		)
	}
	outputContract, err := verification.NewOutputContract(
		1,
		verification.EmptyOutputTransient,
	)
	if err != nil {
		return MissionExecutionCompilation{}, err
	}
	recoveryPolicy, err := rules.NewRecoveryPolicy(rules.RecoveryPolicyInput{
		Version: 1, RetryDelay: 0, AttemptCredits: 1,
		ExhaustionAction: rules.ExhaustionBlocked,
		RetryInvalid:     true,
	})
	if err != nil {
		return MissionExecutionCompilation{}, err
	}
	acceptanceContract, err := verification.NewAcceptanceContract(
		1,
		[]string{
			"authorized output is non-empty",
			"result satisfies the confirmed Mission objective",
		},
		verification.AcceptanceRiskMedium,
	)
	if err != nil {
		return MissionExecutionCompilation{}, err
	}
	now := compiler.now()
	if now.IsZero() || now.Location() != time.UTC {
		return MissionExecutionCompilation{}, ErrInvalidMissionExecution
	}
	executor := compileOnlyMissionExecutor{}
	executions := make([]TeamNodeExecution, 0, 2)
	for attemptNumber := 1; attemptNumber <= 2; attemptNumber++ {
		payload, marshalErr := json.Marshal(struct {
			SchemaVersion int    `json:"schema_version"`
			Kind          string `json:"kind"`
			Prompt        string `json:"prompt"`
		}{1, "pi_rpc_prompt", command.Objective})
		if marshalErr != nil {
			return MissionExecutionCompilation{}, marshalErr
		}
		workItemID := appTeamAttemptIdentity(
			"work",
			plan,
			"main",
			attemptNumber,
		)
		runID := appTeamAttemptIdentity(
			"run",
			plan,
			"main",
			attemptNumber,
		)
		dispatch, frameErr := bridgev1.NewFrame(bridgev1.FrameInput{
			MessageID: appVerifierUUID(
				"mission-dispatch",
				plan.TeamInstanceID(),
				plan.Digest(),
				strconv.Itoa(attemptNumber),
			),
			CorrelationID: command.CorrelationID,
			WorkItemID:    workItemID, RunID: runID,
			ClaimGeneration:       1,
			RuntimeInstanceID:     binding.Instance.ID,
			SenderAgentInstanceID: binding.AgentInstanceID,
			Sequence:              1, Type: bridgev1.MessageDispatch,
			EmittedAt: now, Payload: payload,
		})
		if frameErr != nil {
			return MissionExecutionCompilation{}, errors.Join(
				ErrInvalidMissionExecution,
				frameErr,
			)
		}
		executions = append(executions, TeamNodeExecution{
			LogicalNodeID: "main", AttemptNumber: attemptNumber,
			WorkflowPath: "builtin/mission-primary-v1",
			SourcePath:   compiler.sourcePath,
			Profile:      binding.Profile, Instance: binding.Instance,
			Dispatch: dispatch, Executor: executor,
		})
	}
	verifierAgentID := appVerifierIdentity(
		"mission-verifier-agent",
		plan.TeamInstanceID(),
		plan.Digest(),
	)
	permissionScopes := workPackage.ToolCategories()
	approvalPoints := workPackage.DefaultCustomerRuleTemplateIDs()
	sideEffects := []string{}
	for _, scope := range permissionScopes {
		if scope == "workspace.edit" {
			sideEffects = append(sideEffects, "workspace_write")
		}
	}
	preflight := MissionExecutionPreflight{
		SchemaVersion:     MissionExecutionSchemaVersion,
		MissionID:         command.MissionID,
		TeamInstanceID:    command.TeamInstanceID,
		WorkPackageID:     workPackage.ID(),
		WorkPackageDigest: workPackage.Digest(),
		ViewVersion:       binding.ViewVersion,
		PlanDigest:        plan.Digest(),
		RuntimeInstanceID: binding.Instance.ID,
		RuntimeProfileID:  binding.Profile.ID,
		ModelID:           binding.Profile.ModelID,
		AuthMode:          string(binding.Profile.AuthMode),
		CapacityAvailable: binding.CapacityAvailable,
		BudgetStatus:      "unavailable",
		SideEffects:       sideEffects,
		PermissionScopes:  permissionScopes,
		ApprovalPoints:    approvalPoints,
		Nodes: []MissionExecutionNodePreview{{
			LogicalNodeID: "main", Title: command.Objective,
			Role: "main", DependsOn: []string{}, MaxAttempts: 2,
		}},
	}
	outputObserver := compiler.outputObserver
	if command.Operation == missionExecutionStart &&
		compiler.observerFactory != nil {
		outputObserver, err = compiler.observerFactory.MissionExecutionObserver(
			ctx,
			command.TeamInstanceID,
		)
		if err != nil || nilMissionExecutionInterface(outputObserver) {
			return MissionExecutionCompilation{}, errors.Join(
				ErrInvalidMissionExecution,
				err,
			)
		}
	}
	request := TeamExecutionRequest{
		Plan: plan, Nodes: executions,
		Semantics: []TeamNodeSemantics{{
			LogicalNodeID:             "main",
			OutputContract:            outputContract,
			RecoveryPolicy:            recoveryPolicy,
			AcceptanceContract:        acceptanceContract,
			PrimaryWorkflowPath:       "builtin/mission-primary-v1",
			VerifierAgentInstanceID:   verifierAgentID,
			VerifierRuntimeInstanceID: binding.Instance.ID,
			VerifierWorkflowPath:      "builtin/mission-verifier-v1",
			VerifierExecution: &TeamVerifierExecution{
				SourcePath: compiler.sourcePath,
				Profile:    binding.Profile, Instance: binding.Instance,
				Executor: executor,
			},
		}},
		AuthoritativeTime:    now,
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        2 * time.Minute,
		CorrelationID:        command.CorrelationID,
		OutputObserver:       outputObserver,
	}
	return MissionExecutionCompilation{
		Plan: plan, Preflight: preflight, Request: request,
	}, nil
}

func (compiler *BuiltInMissionExecutionCompiler) ReconstructMissionExecution(
	ctx context.Context,
	projected projection.TeamExecution,
) (TeamExecutionRequest, error) {
	if compiler == nil || ctx == nil ||
		projected.TeamInstanceID == "" || !validSHA256(projected.PlanDigest) ||
		appTerminalTeamStatus(projected.Status) || len(projected.Nodes) != 1 {
		return TeamExecutionRequest{}, ErrMissionExecutionConflict
	}
	recoveryBindings, ok := compiler.bindings.(missionExecutionRecoveryBindingSource)
	if !ok || nilMissionExecutionInterface(recoveryBindings) {
		return TeamExecutionRequest{}, ErrMissionExecutionConflict
	}
	binding, err := recoveryBindings.ResolveMissionExecutionRecoveryBinding(
		ctx,
		projected.TeamInstanceID,
	)
	if err != nil {
		return TeamExecutionRequest{}, err
	}
	node := projected.Nodes[0]
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		return TeamExecutionRequest{}, err
	}
	command := MissionExecutionCommand{
		SchemaVersion:       MissionExecutionSchemaVersion,
		Operation:           missionExecutionStart,
		MissionID:           "mission/" + projected.TeamInstanceID,
		TeamInstanceID:      projected.TeamInstanceID,
		WorkPackageID:       workPackage.ID(),
		WorkPackageDigest:   workPackage.Digest(),
		Objective:           node.Title,
		ExpectedViewVersion: binding.ViewVersion,
		PreflightDigest:     strings.Repeat("0", 64),
		CorrelationID: appVerifierUUID(
			"mission-recovery-correlation",
			projected.TeamInstanceID,
			projected.PlanDigest,
		),
	}
	if !validMissionExecutionCommand(command, missionExecutionStart) {
		return TeamExecutionRequest{}, ErrMissionExecutionConflict
	}
	compilation, err := compiler.compileMissionExecution(ctx, command, binding)
	if err != nil {
		return TeamExecutionRequest{}, errors.Join(
			ErrMissionExecutionConflict,
			err,
		)
	}
	semanticErr := appValidateProjectedSemantics(compilation.Request, projected)
	if compilation.Plan.Digest() != projected.PlanDigest ||
		compilation.Plan.TeamInstanceID() != projected.TeamInstanceID ||
		semanticErr != nil {
		return TeamExecutionRequest{}, errors.Join(
			ErrMissionExecutionConflict,
			semanticErr,
		)
	}
	return compilation.Request, nil
}

func validMissionExecutionSourcePath(path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

type MissionExecutionState interface {
	Refresh(context.Context) error
	Version() string
	TeamExecution(string) (projection.TeamExecution, bool)
}

type MissionExecutionRunner interface {
	Run(context.Context, TeamExecutionRequest) (TeamExecutionResult, error)
}

type MissionExecutionDecisionRouter interface {
	RouteMissionExecutionControl(
		context.Context,
		MissionExecutionCommand,
	) error
}

type missionExecutionDecisionCommandSource interface {
	ListMissionDecisionCommands(
		context.Context,
		MissionDecisionCommandQuery,
	) ([]MissionDecisionCommand, error)
}

type missionExecutionDecisionService interface {
	DecideMission(
		context.Context,
		MissionDecisionCommand,
	) (MissionDecisionResult, error)
}

type missionExecutionPreparedControl struct {
	kind           string
	controlAction  string
	decisionAction string
	mutates        bool
}

type PreparedMissionExecutionDecisionRouter struct {
	commands missionExecutionDecisionCommandSource
	service  missionExecutionDecisionService
	controls map[string][]missionExecutionPreparedControl
}

func NewPreparedMissionExecutionDecisionRouter(
	commands missionExecutionDecisionCommandSource,
	service missionExecutionDecisionService,
	prepared PreparedMissionDecisions,
) (*PreparedMissionExecutionDecisionRouter, error) {
	if nilMissionExecutionInterface(commands) ||
		nilMissionExecutionInterface(service) {
		return nil, ErrInvalidMissionExecution
	}
	router := &PreparedMissionExecutionDecisionRouter{
		commands: commands,
		service:  service,
		controls: make(map[string][]missionExecutionPreparedControl),
	}
	add := func(sheet MissionDecisionSheet, controls ...missionExecutionPreparedControl) error {
		if sheet.DecisionID == "" || sheet.Kind == "" || len(controls) == 0 ||
			len(router.controls[sheet.DecisionID]) != 0 {
			return ErrInvalidMissionExecution
		}
		for _, control := range controls {
			if control.kind != sheet.Kind ||
				!validMissionExecutionPreparedControl(control) {
				return ErrInvalidMissionExecution
			}
		}
		router.controls[sheet.DecisionID] = append(
			[]missionExecutionPreparedControl(nil),
			controls...,
		)
		return nil
	}
	for _, candidate := range prepared.Authorizations {
		controls := []missionExecutionPreparedControl{
			{candidate.Sheet.Kind, "pause_at_gate", "", false},
			{candidate.Sheet.Kind, "not_now", "not_now", false},
			{candidate.Sheet.Kind, "edit_scope", "edit_scope", false},
		}
		if candidate.AllowOnce.ApprovalRequestID() != "" {
			controls = append(controls, missionExecutionPreparedControl{
				candidate.Sheet.Kind, "approve", "allow_once", true,
			})
		}
		if candidate.Deny.ApprovalRequestID() != "" {
			controls = append(controls, missionExecutionPreparedControl{
				candidate.Sheet.Kind, "reject", "deny", true,
			})
		}
		if err := add(candidate.Sheet, controls...); err != nil {
			return nil, err
		}
	}
	for _, candidate := range prepared.Reviews {
		controls := []missionExecutionPreparedControl{
			{candidate.Sheet.Kind, "not_now", "not_now", false},
		}
		if candidate.AcceptResult != nil {
			controls = append(controls, missionExecutionPreparedControl{
				candidate.Sheet.Kind, "approve", "accept_result", true,
			})
		}
		if candidate.RequestChanges != nil {
			controls = append(controls, missionExecutionPreparedControl{
				candidate.Sheet.Kind, "reject", "request_changes", true,
			})
		}
		if err := add(candidate.Sheet, controls...); err != nil {
			return nil, err
		}
	}
	for _, candidate := range prepared.Recoveries {
		controls := []missionExecutionPreparedControl{
			{candidate.Sheet.Kind, "not_now", "not_now", false},
			{candidate.Sheet.Kind, "edit_scope", "edit_scope", false},
		}
		if candidate.StartNewAttempt != nil &&
			candidate.StartNewAttempt.Decision != nil {
			action := candidate.StartNewAttempt.Decision.ActionValue()
			controls = append(controls, missionExecutionPreparedControl{
				candidate.Sheet.Kind, action, "start_new_attempt", true,
			})
			if action == "retry" {
				controls = append(controls, missionExecutionPreparedControl{
					candidate.Sheet.Kind, "restart", "start_new_attempt", true,
				})
			}
		}
		if candidate.StopMission != nil && candidate.StopMission.Decision != nil {
			controls = append(controls, missionExecutionPreparedControl{
				candidate.Sheet.Kind,
				candidate.StopMission.Decision.ActionValue(),
				"stop_mission",
				true,
			})
		}
		if err := add(candidate.Sheet, controls...); err != nil {
			return nil, err
		}
	}
	return router, nil
}

func validMissionExecutionPreparedControl(
	control missionExecutionPreparedControl,
) bool {
	if control.kind != "authorization" && control.kind != "review" &&
		control.kind != "recovery" {
		return false
	}
	switch control.controlAction {
	case "pause_at_gate", "approve", "reject", "not_now", "edit_scope",
		"retry", "fallback", "degraded", "blocked", "human_required",
		"restart":
	default:
		return false
	}
	if control.controlAction == "pause_at_gate" {
		return control.kind == "authorization" &&
			control.decisionAction == "" && !control.mutates
	}
	if control.controlAction == "not_now" || control.controlAction == "edit_scope" {
		return control.decisionAction == control.controlAction && !control.mutates
	}
	return control.decisionAction != "" && control.mutates
}

func (router *PreparedMissionExecutionDecisionRouter) RouteMissionExecutionControl(
	ctx context.Context,
	command MissionExecutionCommand,
) error {
	if router == nil || nilMissionExecutionInterface(router.commands) ||
		nilMissionExecutionInterface(router.service) || ctx == nil ||
		!validMissionExecutionCommand(command, missionExecutionControl) ||
		command.ControlAction == "cancel" {
		return ErrInvalidMissionExecution
	}
	commands, err := router.commands.ListMissionDecisionCommands(
		ctx,
		MissionDecisionCommandQuery{
			ViewVersion: command.ExpectedViewVersion,
			Mode:        MissionDecisionCommandRefreshCurrent,
		},
	)
	if err != nil {
		return err
	}
	type match struct {
		command MissionDecisionCommand
		control missionExecutionPreparedControl
	}
	matches := make([]match, 0, 1)
	for _, candidate := range commands {
		if candidate.TeamInstanceID != command.TeamInstanceID ||
			candidate.LogicalNodeID != command.LogicalNodeID ||
			candidate.AttemptNumber != command.AttemptNumber ||
			candidate.ClaimGeneration != command.ClaimGeneration {
			continue
		}
		for _, control := range router.controls[candidate.DecisionID] {
			if control.kind == candidate.Kind &&
				control.controlAction == command.ControlAction {
				matches = append(matches, match{candidate, control})
			}
		}
	}
	if len(matches) != 1 {
		return ErrMissionDecisionConflict
	}
	selected := matches[0]
	if selected.control.controlAction == "pause_at_gate" {
		return nil
	}
	decision := selected.command
	decision.CorrelationID = command.CorrelationID
	decision.Action = selected.control.decisionAction
	decision.Operation = "defer"
	if selected.control.mutates {
		decision.Operation = "submit"
	}
	result, err := router.service.DecideMission(ctx, decision)
	if err != nil {
		return err
	}
	if result.MissionID != decision.MissionID ||
		result.DecisionID != decision.DecisionID ||
		result.ViewVersion == "" ||
		result.Authoritative != selected.control.mutates {
		return ErrMissionDecisionConflict
	}
	return nil
}

type ProjectionMissionExecutionState struct {
	projection *projection.Projection
}

func NewProjectionMissionExecutionState(
	readModel *projection.Projection,
) (*ProjectionMissionExecutionState, error) {
	if readModel == nil {
		return nil, ErrInvalidMissionExecution
	}
	return &ProjectionMissionExecutionState{projection: readModel}, nil
}

func (state *ProjectionMissionExecutionState) Refresh(ctx context.Context) error {
	if state == nil || state.projection == nil {
		return ErrInvalidMissionExecution
	}
	return state.projection.Rebuild(ctx)
}

func (state *ProjectionMissionExecutionState) Version() string {
	if state == nil || state.projection == nil {
		return ""
	}
	return state.projection.GlobalReadView().Version()
}

func (state *ProjectionMissionExecutionState) TeamExecution(
	teamID string,
) (projection.TeamExecution, bool) {
	if state == nil || state.projection == nil {
		return projection.TeamExecution{}, false
	}
	return state.projection.GlobalReadView().TeamExecution(teamID)
}

func (state *ProjectionMissionExecutionState) TeamExecutions(
	afterTeamID string,
	limit int,
) ([]projection.TeamExecution, bool) {
	if state == nil || state.projection == nil {
		return []projection.TeamExecution{}, false
	}
	return state.projection.GlobalReadView().TeamExecutions(afterTeamID, limit)
}

func (state *ProjectionMissionExecutionState) Run(
	runID string,
) (projection.Run, bool) {
	if state == nil || state.projection == nil {
		return projection.Run{}, false
	}
	return state.projection.GlobalReadView().Run(runID)
}

type AuthoritativeMissionExecutionConfig struct {
	State             MissionExecutionState
	Compiler          MissionExecutionCompiler
	Runner            MissionExecutionRunner
	Decisions         MissionExecutionDecisionRouter
	VisibilityTimeout time.Duration
	Now               func() time.Time
}

type missionExecutionRecoveryState interface {
	TeamExecutions(string, int) ([]projection.TeamExecution, bool)
	Run(string) (projection.Run, bool)
}

type authoritativeMissionFlight struct {
	planDigest      string
	executionDigest string
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	errMu           sync.Mutex
	err             error
}

type missionExecutionPreflightLease struct {
	missionID      string
	teamInstanceID string
	viewVersion    string
	expiresAt      time.Time
}

type AuthoritativeMissionExecutionBackend struct {
	state             MissionExecutionState
	compiler          MissionExecutionCompiler
	runner            MissionExecutionRunner
	decisions         MissionExecutionDecisionRouter
	visibilityTimeout time.Duration
	now               func() time.Time
	ctx               context.Context
	cancel            context.CancelFunc

	mu         sync.Mutex
	closed     bool
	flights    map[string]*authoritativeMissionFlight
	preflights map[string]missionExecutionPreflightLease
}

func NewAuthoritativeMissionExecutionBackend(
	config AuthoritativeMissionExecutionConfig,
) (*AuthoritativeMissionExecutionBackend, error) {
	if nilMissionExecutionInterface(config.State) ||
		nilMissionExecutionInterface(config.Compiler) ||
		nilMissionExecutionInterface(config.Runner) ||
		config.Decisions != nil && nilMissionExecutionInterface(config.Decisions) ||
		config.VisibilityTimeout <= 0 ||
		config.VisibilityTimeout > 10*time.Second {
		return nil, ErrInvalidMissionExecution
	}
	ctx, cancel := context.WithCancel(context.Background())
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if current := now(); current.IsZero() || current.Location() != time.UTC {
		cancel()
		return nil, ErrInvalidMissionExecution
	}
	return &AuthoritativeMissionExecutionBackend{
		state: config.State, compiler: config.Compiler, runner: config.Runner,
		decisions:         config.Decisions,
		visibilityTimeout: config.VisibilityTimeout, now: now,
		ctx: ctx, cancel: cancel,
		flights:    make(map[string]*authoritativeMissionFlight),
		preflights: make(map[string]missionExecutionPreflightLease),
	}, nil
}

func (backend *AuthoritativeMissionExecutionBackend) ResumeProjectedMissions(
	ctx context.Context,
) error {
	if backend == nil || ctx == nil {
		return ErrInvalidMissionExecution
	}
	recoveryState, ok := backend.state.(missionExecutionRecoveryState)
	if !ok || nilMissionExecutionInterface(recoveryState) {
		return ErrMissionExecutionConflict
	}
	reconstructor, ok := backend.compiler.(MissionExecutionReconstructor)
	if !ok || nilMissionExecutionInterface(reconstructor) {
		return ErrMissionExecutionConflict
	}
	if err := backend.state.Refresh(ctx); err != nil {
		return err
	}
	executions, hasMore := recoveryState.TeamExecutions(
		"",
		maxAuthoritativeMissionFlights,
	)
	if hasMore {
		return ErrMissionExecutionBusy
	}
	for _, projected := range executions {
		if appTerminalTeamStatus(projected.Status) {
			continue
		}
		if projected.Status != "running" &&
			projected.Status != "awaiting_recovery" {
			return ErrMissionExecutionConflict
		}
		request, err := reconstructor.ReconstructMissionExecution(ctx, projected)
		if err != nil || request.Plan.Digest() != projected.PlanDigest {
			return errors.Join(ErrMissionExecutionConflict, err)
		}
		delay, err := missionExecutionRecoveryDelay(
			recoveryState,
			projected,
			backend.now(),
		)
		if err != nil {
			return err
		}
		executionDigest := missionExecutionDigest(
			projected.TeamInstanceID,
			projected.PlanDigest,
			"restart",
		)
		flight, launch, err := backend.flight(
			projected.TeamInstanceID,
			projected.PlanDigest,
			executionDigest,
		)
		if err != nil {
			return err
		}
		if launch {
			backend.launchRecovered(flight, request, delay)
		}
	}
	return nil
}

func missionExecutionRecoveryDelay(
	state missionExecutionRecoveryState,
	projected projection.TeamExecution,
	now time.Time,
) (time.Duration, error) {
	if now.IsZero() || now.Location() != time.UTC {
		return 0, ErrInvalidMissionExecution
	}
	var latest time.Time
	for _, node := range projected.Nodes {
		for _, attempt := range node.Attempts {
			if attempt.Status == "succeeded" || attempt.Status == "failed" ||
				attempt.Status == "cancelled" || attempt.RunID == "" {
				continue
			}
			run, ok := state.Run(attempt.RunID)
			if !ok || run.ID != attempt.RunID ||
				run.ClaimGeneration != attempt.ClaimGeneration {
				return 0, ErrMissionExecutionConflict
			}
			if run.PrepareLeaseExpiresAt.After(latest) {
				latest = run.PrepareLeaseExpiresAt
			}
		}
	}
	if latest.After(now) {
		return latest.Sub(now), nil
	}
	return 0, nil
}

func (backend *AuthoritativeMissionExecutionBackend) launchRecovered(
	flight *authoritativeMissionFlight,
	request TeamExecutionRequest,
	delay time.Duration,
) {
	go func() {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-flight.ctx.Done():
				close(flight.done)
				return
			case <-timer.C:
			}
		}
		request.AuthoritativeTime = backend.now()
		_, err := backend.runner.Run(flight.ctx, request)
		flight.errMu.Lock()
		flight.err = err
		flight.errMu.Unlock()
		close(flight.done)
	}()
}

func (backend *AuthoritativeMissionExecutionBackend) PreflightMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionPreflight, error) {
	compilation, err := backend.compileCurrent(ctx, command)
	if err != nil {
		return MissionExecutionPreflight{}, err
	}
	preflight := cloneMissionExecutionPreflight(compilation.Preflight)
	expiresAt := backend.now().Add(missionExecutionPreflightTTL)
	if expiresAt.IsZero() || expiresAt.Location() != time.UTC {
		return MissionExecutionPreflight{}, ErrInvalidMissionExecution
	}
	preflight.ExpiresAt = expiresAt.Format(time.RFC3339Nano)
	preflight.PreflightDigest, err = missionExecutionPreflightDigest(
		command,
		preflight,
	)
	if err != nil {
		return MissionExecutionPreflight{}, err
	}
	if err := backend.rememberPreflight(
		preflight.PreflightDigest,
		command,
		expiresAt,
	); err != nil {
		return MissionExecutionPreflight{}, err
	}
	return preflight, nil
}

func (backend *AuthoritativeMissionExecutionBackend) StartMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionResult, error) {
	compilation, err := backend.compileCurrent(ctx, command)
	if err != nil {
		return MissionExecutionResult{}, err
	}
	preflight := cloneMissionExecutionPreflight(compilation.Preflight)
	lease, ok := backend.currentPreflight(command, backend.now())
	if !ok {
		return MissionExecutionResult{}, ErrMissionExecutionConflict
	}
	preflight.ExpiresAt = lease.expiresAt.Format(time.RFC3339Nano)
	preflight.PreflightDigest, err = missionExecutionPreflightDigest(
		command,
		preflight,
	)
	if err != nil || preflight.PreflightDigest != command.PreflightDigest {
		return MissionExecutionResult{}, ErrMissionExecutionConflict
	}
	executionDigest := missionExecutionDigest(
		command.TeamInstanceID,
		compilation.Plan.Digest(),
		preflight.PreflightDigest,
	)
	if projected, ok := backend.state.TeamExecution(command.TeamInstanceID); ok {
		return backend.projectedResult(command, compilation.Plan, executionDigest, projected)
	}
	flight, launch, err := backend.flight(
		command.TeamInstanceID,
		compilation.Plan.Digest(),
		executionDigest,
	)
	if err != nil {
		return MissionExecutionResult{}, err
	}
	if launch {
		backend.launch(flight, compilation.Request)
	}
	return backend.waitForProjectedDispatch(ctx, command, compilation.Plan, flight)
}

func (backend *AuthoritativeMissionExecutionBackend) ControlMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionResult, error) {
	if backend == nil || ctx == nil ||
		!validMissionExecutionCommand(command, missionExecutionControl) {
		return MissionExecutionResult{}, ErrInvalidMissionExecution
	}
	if err := backend.state.Refresh(ctx); err != nil {
		return MissionExecutionResult{}, err
	}
	if backend.state.Version() != command.ExpectedViewVersion {
		return MissionExecutionResult{}, ErrMissionExecutionConflict
	}
	projected, ok := backend.state.TeamExecution(command.TeamInstanceID)
	if !ok || !validProjectedMissionControl(command, projected) {
		return MissionExecutionResult{}, ErrMissionExecutionConflict
	}
	backend.mu.Lock()
	flight := backend.flights[command.TeamInstanceID]
	backend.mu.Unlock()
	if flight == nil || flight.executionDigest != command.ExecutionDigest {
		return MissionExecutionResult{}, ErrMissionExecutionConflict
	}
	if command.ControlAction == "cancel" {
		flight.cancel()
		select {
		case <-flight.done:
		case <-ctx.Done():
			return MissionExecutionResult{}, ctx.Err()
		}
	} else {
		if nilMissionExecutionInterface(backend.decisions) {
			return MissionExecutionResult{}, ErrMissionExecutionConflict
		}
		if err := backend.decisions.RouteMissionExecutionControl(
			ctx,
			command,
		); err != nil {
			return MissionExecutionResult{}, errors.Join(
				ErrMissionExecutionConflict,
				err,
			)
		}
	}
	if err := backend.state.Refresh(ctx); err != nil {
		return MissionExecutionResult{}, err
	}
	projected, ok = backend.state.TeamExecution(command.TeamInstanceID)
	if !ok || command.ControlAction == "cancel" && projected.Status != "cancelled" {
		return MissionExecutionResult{}, ErrTeamExecutionIncomplete
	}
	status, ok := missionExecutionProjectedStatus(projected.Status)
	if !ok {
		return MissionExecutionResult{}, ErrTeamExecutionIncomplete
	}
	return MissionExecutionResult{
		SchemaVersion: MissionExecutionSchemaVersion,
		MissionID:     command.MissionID, TeamInstanceID: command.TeamInstanceID,
		Status: status, ViewVersion: backend.state.Version(),
		ExecutionDigest: flight.executionDigest,
	}, nil
}

func validProjectedMissionControl(
	command MissionExecutionCommand,
	projected projection.TeamExecution,
) bool {
	if projected.TeamInstanceID != command.TeamInstanceID ||
		appTerminalTeamStatus(projected.Status) {
		return false
	}
	for _, node := range projected.Nodes {
		if node.LogicalNodeID != command.LogicalNodeID ||
			node.CurrentAttempt != command.AttemptNumber {
			continue
		}
		for _, attempt := range node.Attempts {
			if attempt.AttemptNumber == command.AttemptNumber &&
				attempt.ClaimGeneration == command.ClaimGeneration &&
				attempt.Status != "cancelled" &&
				attempt.Status != "succeeded" && attempt.Status != "failed" {
				return true
			}
		}
	}
	return false
}

func (backend *AuthoritativeMissionExecutionBackend) Close() error {
	if backend == nil {
		return nil
	}
	backend.mu.Lock()
	if backend.closed {
		backend.mu.Unlock()
		return nil
	}
	backend.closed = true
	backend.cancel()
	flights := make([]*authoritativeMissionFlight, 0, len(backend.flights))
	for _, flight := range backend.flights {
		flight.cancel()
		flights = append(flights, flight)
	}
	backend.mu.Unlock()
	for _, flight := range flights {
		<-flight.done
	}
	return nil
}

func (backend *AuthoritativeMissionExecutionBackend) compileCurrent(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionCompilation, error) {
	if backend == nil || ctx == nil {
		return MissionExecutionCompilation{}, ErrInvalidMissionExecution
	}
	backend.mu.Lock()
	closed := backend.closed
	backend.mu.Unlock()
	if closed {
		return MissionExecutionCompilation{}, ErrInvalidMissionExecution
	}
	if err := backend.state.Refresh(ctx); err != nil {
		return MissionExecutionCompilation{}, err
	}
	if backend.state.Version() != command.ExpectedViewVersion {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	compilation, err := backend.compiler.CompileMissionExecution(ctx, command)
	if err != nil {
		return MissionExecutionCompilation{}, err
	}
	if compilation.Plan.TeamInstanceID() != command.TeamInstanceID ||
		compilation.Plan.Digest() == "" ||
		compilation.Request.Plan.Digest() != compilation.Plan.Digest() ||
		compilation.Preflight.PlanDigest != compilation.Plan.Digest() ||
		compilation.Preflight.PreflightDigest != "" {
		return MissionExecutionCompilation{}, ErrInvalidMissionExecution
	}
	return compilation, nil
}

func (backend *AuthoritativeMissionExecutionBackend) flight(
	teamID, planDigest, executionDigest string,
) (*authoritativeMissionFlight, bool, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.closed {
		return nil, false, ErrInvalidMissionExecution
	}
	backend.reapTerminalFlightsLocked()
	if current := backend.flights[teamID]; current != nil {
		if current.planDigest != planDigest ||
			current.executionDigest != executionDigest {
			return nil, false, ErrMissionExecutionConflict
		}
		return current, false, nil
	}
	if len(backend.flights) >= maxAuthoritativeMissionFlights {
		return nil, false, ErrMissionExecutionBusy
	}
	ctx, cancel := context.WithCancel(backend.ctx)
	flight := &authoritativeMissionFlight{
		planDigest: planDigest, executionDigest: executionDigest,
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
	}
	backend.flights[teamID] = flight
	return flight, true, nil
}

func (backend *AuthoritativeMissionExecutionBackend) reapTerminalFlightsLocked() {
	for teamID, flight := range backend.flights {
		select {
		case <-flight.done:
			projected, ok := backend.state.TeamExecution(teamID)
			if ok && appTerminalTeamStatus(projected.Status) {
				delete(backend.flights, teamID)
			}
		default:
		}
	}
}

func (backend *AuthoritativeMissionExecutionBackend) launch(
	flight *authoritativeMissionFlight,
	request TeamExecutionRequest,
) {
	go func() {
		_, err := backend.runner.Run(flight.ctx, request)
		flight.errMu.Lock()
		flight.err = err
		flight.errMu.Unlock()
		close(flight.done)
	}()
}

func (backend *AuthoritativeMissionExecutionBackend) waitForProjectedDispatch(
	ctx context.Context,
	command MissionExecutionCommand,
	plan teams.ExecutionPlan,
	flight *authoritativeMissionFlight,
) (MissionExecutionResult, error) {
	deadline := time.NewTimer(backend.visibilityTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	done := flight.done
	for {
		if err := backend.state.Refresh(ctx); err != nil {
			return MissionExecutionResult{}, err
		}
		if projected, ok := backend.state.TeamExecution(command.TeamInstanceID); ok {
			return backend.projectedResult(
				command, plan, flight.executionDigest, projected,
			)
		}
		select {
		case <-ctx.Done():
			return MissionExecutionResult{}, ctx.Err()
		case <-deadline.C:
			return MissionExecutionResult{}, ErrTeamExecutionIncomplete
		case <-done:
			done = nil
			flight.errMu.Lock()
			err := flight.err
			flight.errMu.Unlock()
			if err != nil && !errors.Is(err, ErrTeamExecutionIncomplete) {
				return MissionExecutionResult{}, err
			}
		case <-ticker.C:
		}
	}
}

func (backend *AuthoritativeMissionExecutionBackend) projectedResult(
	command MissionExecutionCommand,
	plan teams.ExecutionPlan,
	executionDigest string,
	projected projection.TeamExecution,
) (MissionExecutionResult, error) {
	if projected.PlanDigest != plan.Digest() {
		return MissionExecutionResult{}, ErrMissionExecutionConflict
	}
	status, ok := missionExecutionProjectedStatus(projected.Status)
	if !ok {
		return MissionExecutionResult{}, ErrTeamExecutionIncomplete
	}
	return MissionExecutionResult{
		SchemaVersion: MissionExecutionSchemaVersion,
		MissionID:     command.MissionID, TeamInstanceID: command.TeamInstanceID,
		Status: status, ViewVersion: backend.state.Version(),
		ExecutionDigest: executionDigest,
	}, nil
}

func missionExecutionProjectedStatus(status string) (string, bool) {
	switch status {
	case "running", "awaiting_recovery", "cancelled", "degraded", "blocked",
		"human_required", "succeeded", "failed":
		return status, true
	default:
		return "", false
	}
}

func (backend *AuthoritativeMissionExecutionBackend) rememberPreflight(
	digest string,
	command MissionExecutionCommand,
	expiresAt time.Time,
) error {
	if !validSHA256(digest) || expiresAt.IsZero() ||
		expiresAt.Location() != time.UTC {
		return ErrInvalidMissionExecution
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.closed {
		return ErrInvalidMissionExecution
	}
	backend.reapExpiredPreflightsLocked(backend.now())
	if len(backend.preflights) >= maxAuthoritativeMissionFlights {
		return ErrMissionExecutionBusy
	}
	backend.preflights[digest] = missionExecutionPreflightLease{
		missionID: command.MissionID, teamInstanceID: command.TeamInstanceID,
		viewVersion: command.ExpectedViewVersion, expiresAt: expiresAt,
	}
	return nil
}

func (backend *AuthoritativeMissionExecutionBackend) currentPreflight(
	command MissionExecutionCommand,
	now time.Time,
) (missionExecutionPreflightLease, bool) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.reapExpiredPreflightsLocked(now)
	lease, ok := backend.preflights[command.PreflightDigest]
	if !ok || lease.missionID != command.MissionID ||
		lease.teamInstanceID != command.TeamInstanceID ||
		lease.viewVersion != command.ExpectedViewVersion ||
		!now.Before(lease.expiresAt) {
		return missionExecutionPreflightLease{}, false
	}
	return lease, true
}

func (backend *AuthoritativeMissionExecutionBackend) reapExpiredPreflightsLocked(
	now time.Time,
) {
	for digest, lease := range backend.preflights {
		if !now.Before(lease.expiresAt) {
			delete(backend.preflights, digest)
		}
	}
}

func missionExecutionPreflightDigest(
	command MissionExecutionCommand,
	preflight MissionExecutionPreflight,
) (string, error) {
	preflight.PreflightDigest = ""
	command.PreflightDigest = ""
	command.Operation = missionExecutionPreflight
	command.CorrelationID = ""
	encoded, err := json.Marshal(struct {
		SchemaVersion int                       `json:"schema_version"`
		Command       MissionExecutionCommand   `json:"command"`
		Preflight     MissionExecutionPreflight `json:"preflight"`
	}{MissionExecutionSchemaVersion, command, preflight})
	if err != nil {
		return "", fmt.Errorf("%w: preflight encoding", ErrInvalidMissionExecution)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func missionExecutionDigest(fields ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(fields, "\x00")))
	return hex.EncodeToString(digest[:])
}

func nilMissionExecutionInterface(value any) bool {
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

type LocalProductExecutionConfig struct {
	Backend MissionExecutionBackend
}

type LocalProductExecutionService struct {
	backend MissionExecutionBackend
}

func NewLocalProductExecutionService(
	config LocalProductExecutionConfig,
) (*LocalProductExecutionService, error) {
	if nilMissionExecutionBackend(config.Backend) {
		return nil, ErrInvalidMissionExecution
	}
	return &LocalProductExecutionService{backend: config.Backend}, nil
}

func (service *LocalProductExecutionService) PreflightMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionPreflight, error) {
	if service == nil || nilMissionExecutionBackend(service.backend) ||
		!validMissionExecutionCommand(command, missionExecutionPreflight) {
		return MissionExecutionPreflight{}, ErrInvalidMissionExecution
	}
	if err := ctx.Err(); err != nil {
		return MissionExecutionPreflight{}, err
	}
	result, err := service.backend.PreflightMission(ctx, command)
	if err != nil {
		return MissionExecutionPreflight{}, err
	}
	if !validMissionExecutionPreflight(command, result) {
		return MissionExecutionPreflight{}, ErrInvalidMissionExecution
	}
	return cloneMissionExecutionPreflight(result), nil
}

func (service *LocalProductExecutionService) StartMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionResult, error) {
	return service.execute(ctx, command, missionExecutionStart)
}

func (service *LocalProductExecutionService) ControlMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionResult, error) {
	return service.execute(ctx, command, missionExecutionControl)
}

func (service *LocalProductExecutionService) execute(
	ctx context.Context,
	command MissionExecutionCommand,
	operation string,
) (MissionExecutionResult, error) {
	if service == nil || nilMissionExecutionBackend(service.backend) ||
		!validMissionExecutionCommand(command, operation) {
		return MissionExecutionResult{}, ErrInvalidMissionExecution
	}
	if err := ctx.Err(); err != nil {
		return MissionExecutionResult{}, err
	}
	var (
		result MissionExecutionResult
		err    error
	)
	if operation == missionExecutionStart {
		result, err = service.backend.StartMission(ctx, command)
	} else {
		result, err = service.backend.ControlMission(ctx, command)
	}
	if err != nil {
		return MissionExecutionResult{}, err
	}
	if !validMissionExecutionResult(command, result) {
		return MissionExecutionResult{}, ErrInvalidMissionExecution
	}
	return result, nil
}

func validMissionExecutionCommand(
	command MissionExecutionCommand,
	operation string,
) bool {
	if command.SchemaVersion != MissionExecutionSchemaVersion ||
		command.Operation != operation ||
		!validMissionExecutionText(command.MissionID, 128) ||
		!validMissionExecutionText(command.TeamInstanceID, 128) ||
		command.MissionID != "mission/"+command.TeamInstanceID ||
		!validSHA256(command.ExpectedViewVersion) ||
		!validMissionExecutionUUID(command.CorrelationID) {
		return false
	}
	switch operation {
	case missionExecutionPreflight:
		return validMissionExecutionPreStart(command) &&
			command.PreflightDigest == "" &&
			missionExecutionControlFieldsEmpty(command)
	case missionExecutionStart:
		return validMissionExecutionPreStart(command) &&
			validSHA256(command.PreflightDigest) &&
			missionExecutionControlFieldsEmpty(command)
	case missionExecutionControl:
		return command.WorkPackageID == "" &&
			command.WorkPackageDigest == "" && command.Objective == "" &&
			command.PreflightDigest == "" &&
			validMissionExecutionControl(command)
	default:
		return false
	}
}

func validMissionExecutionPreStart(command MissionExecutionCommand) bool {
	workPackage, err := missionExecutionWorkPackage(command.WorkPackageID)
	return err == nil && command.WorkPackageDigest == workPackage.Digest() &&
		validMissionExecutionText(command.Objective, maxMissionExecutionTextBytes)
}

func missionExecutionWorkPackage(id string) (work.WorkPackage, error) {
	switch id {
	case "work-package.coding":
		return work.CodingWorkPackage()
	case "work-package.knowledge":
		return work.KnowledgeWorkPackage()
	default:
		return work.WorkPackage{}, work.ErrInvalidWorkPackage
	}
}

func missionExecutionControlFieldsEmpty(command MissionExecutionCommand) bool {
	return command.ControlAction == "" && command.ExecutionDigest == "" &&
		command.LogicalNodeID == "" && command.AttemptNumber == 0 &&
		command.ClaimGeneration == 0
}

func validMissionExecutionControl(command MissionExecutionCommand) bool {
	switch command.ControlAction {
	case "pause_at_gate", "approve", "reject", "not_now", "edit_scope",
		"cancel", "retry", "fallback", "degraded", "blocked",
		"human_required", "restart":
	default:
		return false
	}
	return validSHA256(command.ExecutionDigest) &&
		validMissionExecutionText(command.LogicalNodeID, 128) &&
		command.AttemptNumber > 0 && command.AttemptNumber <= 16 &&
		command.ClaimGeneration > 0
}

func validMissionExecutionPreflight(
	command MissionExecutionCommand,
	result MissionExecutionPreflight,
) bool {
	if result.SchemaVersion != MissionExecutionSchemaVersion ||
		result.MissionID != command.MissionID ||
		result.TeamInstanceID != command.TeamInstanceID ||
		result.WorkPackageID != command.WorkPackageID ||
		result.WorkPackageDigest != command.WorkPackageDigest ||
		result.ViewVersion != command.ExpectedViewVersion ||
		!validSHA256(result.PlanDigest) ||
		!validSHA256(result.PreflightDigest) ||
		!validMissionExecutionText(result.RuntimeInstanceID, 128) ||
		!validMissionExecutionText(result.RuntimeProfileID, 128) ||
		!validMissionExecutionText(result.ModelID, 256) ||
		!validMissionExecutionText(result.AuthMode, 64) ||
		result.CapacityAvailable < 0 ||
		(result.BudgetStatus != "available" && result.BudgetStatus != "unavailable") ||
		!validMissionExecutionStrings(result.SideEffects) ||
		!validMissionExecutionStrings(result.PermissionScopes) ||
		!validMissionExecutionStrings(result.ApprovalPoints) ||
		len(result.Nodes) == 0 || len(result.Nodes) > maxMissionExecutionNodeCount {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, result.ExpiresAt)
	if err != nil || expiresAt.IsZero() || expiresAt.Location() != time.UTC {
		return false
	}
	for _, node := range result.Nodes {
		if !validMissionExecutionText(node.LogicalNodeID, 128) ||
			!validMissionExecutionText(node.Title, 512) ||
			!validMissionExecutionText(node.Role, 64) ||
			node.MaxAttempts <= 0 || node.MaxAttempts > 16 ||
			!validMissionExecutionStrings(node.DependsOn) {
			return false
		}
	}
	return true
}

func validMissionExecutionResult(
	command MissionExecutionCommand,
	result MissionExecutionResult,
) bool {
	if result.SchemaVersion != MissionExecutionSchemaVersion ||
		result.MissionID != command.MissionID ||
		result.TeamInstanceID != command.TeamInstanceID ||
		!validSHA256(result.ViewVersion) ||
		!validSHA256(result.ExecutionDigest) {
		return false
	}
	switch result.Status {
	case "running", "awaiting_recovery", "cancelled", "degraded", "blocked",
		"human_required", "succeeded", "failed":
		return true
	default:
		return false
	}
}

func cloneMissionExecutionPreflight(
	value MissionExecutionPreflight,
) MissionExecutionPreflight {
	cloned := value
	cloned.SideEffects = cloneMissionExecutionStrings(value.SideEffects)
	cloned.PermissionScopes = cloneMissionExecutionStrings(value.PermissionScopes)
	cloned.ApprovalPoints = cloneMissionExecutionStrings(value.ApprovalPoints)
	cloned.Nodes = make([]MissionExecutionNodePreview, len(value.Nodes))
	for index, node := range value.Nodes {
		cloned.Nodes[index] = node
		cloned.Nodes[index].DependsOn = cloneMissionExecutionStrings(node.DependsOn)
	}
	return cloned
}

func cloneMissionExecutionStrings(values []string) []string {
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func validMissionExecutionStrings(values []string) bool {
	if values == nil || len(values) > maxMissionExecutionListItems {
		return false
	}
	for _, value := range values {
		if !validMissionExecutionText(value, 256) {
			return false
		}
	}
	return true
}

func validMissionExecutionText(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) &&
		utf8.ValidString(value) && len(value) <= maximum &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func validMissionExecutionUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' || value[14] != '4' ||
		!strings.Contains("89ab", string(value[19])) {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	_, err := hex.DecodeString(compact)
	return err == nil && strings.ToLower(value) == value
}

func nilMissionExecutionBackend(value MissionExecutionBackend) bool {
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
