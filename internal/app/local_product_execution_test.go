package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
)

type recordingMissionExecutionBackend struct {
	preflight MissionExecutionPreflight
	result    MissionExecutionResult
	err       error
	commands  []MissionExecutionCommand
}

type controlledMissionExecutionState struct {
	mu         sync.Mutex
	version    string
	execution  projection.TeamExecution
	byTeam     map[string]projection.TeamExecution
	executions []projection.TeamExecution
	runs       map[string]projection.Run
	visible    bool
	refreshes  int
}

func (state *controlledMissionExecutionState) TeamExecutions(
	string,
	int,
) ([]projection.TeamExecution, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return append([]projection.TeamExecution(nil), state.executions...), false
}

func (state *controlledMissionExecutionState) Run(
	runID string,
) (projection.Run, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	run, ok := state.runs[runID]
	return run, ok
}

func (state *controlledMissionExecutionState) Refresh(context.Context) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.refreshes++
	return nil
}
func (state *controlledMissionExecutionState) Version() string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.version
}
func (state *controlledMissionExecutionState) TeamExecution(
	teamID string,
) (projection.TeamExecution, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.byTeam != nil {
		execution, ok := state.byTeam[teamID]
		return execution, ok
	}
	return state.execution, state.visible && state.execution.TeamInstanceID == teamID
}
func (state *controlledMissionExecutionState) Stats() (bool, int) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.visible, state.refreshes
}

type controlledMissionExecutionCompiler struct {
	mu              sync.Mutex
	compilation     MissionExecutionCompilation
	recoveryRequest TeamExecutionRequest
	recoveryErr     error
	calls           int
}

func (compiler *controlledMissionExecutionCompiler) ReconstructMissionExecution(
	context.Context,
	projection.TeamExecution,
) (TeamExecutionRequest, error) {
	compiler.mu.Lock()
	defer compiler.mu.Unlock()
	return compiler.recoveryRequest, compiler.recoveryErr
}

func (compiler *controlledMissionExecutionCompiler) CompileMissionExecution(
	context.Context,
	MissionExecutionCommand,
) (MissionExecutionCompilation, error) {
	compiler.mu.Lock()
	defer compiler.mu.Unlock()
	compiler.calls++
	return compiler.compilation, nil
}

type controlledTeamExecutionRunner struct {
	mu    sync.Mutex
	state *controlledMissionExecutionState
	calls int
}

type cancellationMissionExecutionRunner struct {
	state   *controlledMissionExecutionState
	plan    teams.ExecutionPlan
	started chan struct{}
}

func (runner *cancellationMissionExecutionRunner) Run(
	ctx context.Context,
	_ TeamExecutionRequest,
) (TeamExecutionResult, error) {
	runner.state.mu.Lock()
	runner.state.visible = true
	runner.state.execution = projection.TeamExecution{
		TeamInstanceID: runner.plan.TeamInstanceID(),
		PlanDigest:     runner.plan.Digest(),
		Status:         "running",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID:  "main",
			CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber:   1,
				ClaimGeneration: 1,
				Status:          "running",
			}},
		}},
	}
	runner.state.mu.Unlock()
	close(runner.started)
	<-ctx.Done()
	runner.state.mu.Lock()
	runner.state.execution.Status = "cancelled"
	runner.state.execution.Nodes[0].Status = "cancelled"
	runner.state.execution.Nodes[0].Attempts[0].Status = "cancelled"
	runner.state.mu.Unlock()
	return TeamExecutionResult{}, ctx.Err()
}

type incompleteMissionExecutionRunner struct{}

func (incompleteMissionExecutionRunner) Run(
	context.Context,
	TeamExecutionRequest,
) (TeamExecutionResult, error) {
	return TeamExecutionResult{}, ErrTeamExecutionIncomplete
}

type controlledMissionExecutionDecisionRouter struct {
	command MissionExecutionCommand
	calls   int
	err     error
}

type recordingMissionExecutionDecisionSource struct {
	commands []MissionDecisionCommand
	query    MissionDecisionCommandQuery
	calls    int
	err      error
}

func (source *recordingMissionExecutionDecisionSource) ListMissionDecisionCommands(
	_ context.Context,
	query MissionDecisionCommandQuery,
) ([]MissionDecisionCommand, error) {
	source.calls++
	source.query = query
	return append([]MissionDecisionCommand(nil), source.commands...), source.err
}

type recordingMissionExecutionDecisionService struct {
	command MissionDecisionCommand
	result  MissionDecisionResult
	calls   int
	err     error
}

func (service *recordingMissionExecutionDecisionService) DecideMission(
	_ context.Context,
	command MissionDecisionCommand,
) (MissionDecisionResult, error) {
	service.calls++
	service.command = command
	return service.result, service.err
}

func (router *controlledMissionExecutionDecisionRouter) RouteMissionExecutionControl(
	_ context.Context,
	command MissionExecutionCommand,
) error {
	router.calls++
	router.command = command
	return router.err
}

type controlledMissionExecutionBindingSource struct {
	binding MissionExecutionBinding
	err     error
	calls   int
}

func (source *controlledMissionExecutionBindingSource) ResolveMissionExecutionRecoveryBinding(
	ctx context.Context,
	teamID string,
) (MissionExecutionBinding, error) {
	return source.ResolveMissionExecutionBinding(ctx, teamID)
}

type controlledMissionExecutionOutputObserver struct{}

func (controlledMissionExecutionOutputObserver) ObserveNodeOutput(
	context.Context,
	NodeOutput,
) error {
	return nil
}

type controlledMissionExecutionObserverFactory struct {
	observer NodeOutputObserver
	calls    int
	teamID   string
}

func (factory *controlledMissionExecutionObserverFactory) MissionExecutionObserver(
	_ context.Context,
	teamInstanceID string,
) (NodeOutputObserver, error) {
	factory.calls++
	factory.teamID = teamInstanceID
	return factory.observer, nil
}

func (source *controlledMissionExecutionBindingSource) ResolveMissionExecutionBinding(
	context.Context,
	string,
) (MissionExecutionBinding, error) {
	source.calls++
	return source.binding, source.err
}

func (runner *controlledTeamExecutionRunner) Run(
	context.Context,
	TeamExecutionRequest,
) (TeamExecutionResult, error) {
	runner.mu.Lock()
	runner.calls++
	runner.mu.Unlock()
	runner.state.mu.Lock()
	runner.state.visible = true
	runner.state.mu.Unlock()
	return TeamExecutionResult{}, ErrTeamExecutionIncomplete
}

func (runner *controlledTeamExecutionRunner) Calls() int {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.calls
}

func TestAuthoritativeMissionExecutionStartReturnsOnlyAfterProjectedDispatch(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: command.ExpectedViewVersion,
		execution: projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID,
			PlanDigest:     plan.Digest(), Status: "running",
		},
	}
	compiler := &controlledMissionExecutionCompiler{
		compilation: MissionExecutionCompilation{
			Plan: plan,
			Preflight: MissionExecutionPreflight{
				SchemaVersion: MissionExecutionSchemaVersion,
				MissionID:     command.MissionID, TeamInstanceID: command.TeamInstanceID,
				WorkPackageID:     command.WorkPackageID,
				WorkPackageDigest: command.WorkPackageDigest,
				ViewVersion:       command.ExpectedViewVersion,
				PlanDigest:        plan.Digest(), RuntimeInstanceID: "runtime-pi",
				RuntimeProfileID: "pi-default", ModelID: "qwen",
				AuthMode: "brokered", CapacityAvailable: 1,
				BudgetStatus: "unavailable", SideEffects: []string{},
				PermissionScopes: []string{"workspace"},
				ApprovalPoints:   []string{"before_start"},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", DependsOn: []string{}, MaxAttempts: 2,
				}},
			},
			Request: TeamExecutionRequest{Plan: plan},
		},
	}
	runner := &controlledTeamExecutionRunner{state: state}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler, Runner: runner,
			VisibilityTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()

	preflight, err := backend.PreflightMission(context.Background(), command)
	if err != nil || preflight.PreflightDigest == "" {
		t.Fatalf("preflight = %#v, %v", preflight, err)
	}
	start := command
	start.Operation = "start"
	start.CorrelationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	start.PreflightDigest = preflight.PreflightDigest
	result, err := backend.StartMission(context.Background(), start)
	visible, refreshes := state.Stats()
	if err != nil || result.Status != "running" || runner.Calls() != 1 ||
		!visible || refreshes < 2 {
		t.Fatalf("start = %#v, err=%v calls=%d refreshes=%d", result, err, runner.Calls(), refreshes)
	}
}

func TestAuthoritativeMissionExecutionConcurrentStartHasOneRunner(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: command.ExpectedViewVersion,
		execution: projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID,
			PlanDigest:     plan.Digest(), Status: "running",
		},
	}
	compiler := &controlledMissionExecutionCompiler{
		compilation: MissionExecutionCompilation{
			Plan: plan, Request: TeamExecutionRequest{Plan: plan},
			Preflight: MissionExecutionPreflight{
				SchemaVersion: 1, MissionID: command.MissionID,
				TeamInstanceID:    command.TeamInstanceID,
				WorkPackageID:     command.WorkPackageID,
				WorkPackageDigest: command.WorkPackageDigest,
				ViewVersion:       command.ExpectedViewVersion,
				PlanDigest:        plan.Digest(), RuntimeInstanceID: "runtime-pi",
				RuntimeProfileID: "pi-default", ModelID: "qwen",
				AuthMode: "brokered", CapacityAvailable: 1,
				BudgetStatus: "unavailable", SideEffects: []string{},
				PermissionScopes: []string{"workspace"},
				ApprovalPoints:   []string{"before_start"},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", DependsOn: []string{}, MaxAttempts: 1,
				}},
			},
		},
	}
	runner := &controlledTeamExecutionRunner{state: state}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler, Runner: runner,
			VisibilityTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	preflight, err := backend.PreflightMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	start := command
	start.Operation = "start"
	start.PreflightDigest = preflight.PreflightDigest

	const count = 16
	errs := make(chan error, count)
	var wait sync.WaitGroup
	for range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, startErr := backend.StartMission(context.Background(), start)
			if startErr == nil && result.Status != "running" {
				startErr = fmt.Errorf("status %q", result.Status)
			}
			errs <- startErr
		}()
	}
	wait.Wait()
	close(errs)
	for startErr := range errs {
		if startErr != nil {
			t.Fatal(startErr)
		}
	}
	if runner.Calls() != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.Calls())
	}
}

func TestAuthoritativeMissionCancelRejectsStaleViewAndGeneration(t *testing.T) {
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{version: command.ExpectedViewVersion}
	compiler := &controlledMissionExecutionCompiler{compilation: MissionExecutionCompilation{
		Plan: plan, Request: TeamExecutionRequest{Plan: plan},
		Preflight: MissionExecutionPreflight{
			SchemaVersion: 1, MissionID: command.MissionID,
			TeamInstanceID:    command.TeamInstanceID,
			WorkPackageID:     command.WorkPackageID,
			WorkPackageDigest: command.WorkPackageDigest,
			ViewVersion:       command.ExpectedViewVersion,
			PlanDigest:        plan.Digest(), RuntimeInstanceID: "runtime-pi",
			RuntimeProfileID: "pi-default", ModelID: "qwen",
			AuthMode: "brokered", CapacityAvailable: 1,
			BudgetStatus: "unavailable", SideEffects: []string{},
			PermissionScopes: []string{"workspace"},
			ApprovalPoints:   []string{"before_start"},
			Nodes: []MissionExecutionNodePreview{{
				LogicalNodeID: "main", Title: command.Objective,
				Role: "main", DependsOn: []string{}, MaxAttempts: 2,
			}},
		},
	}}
	runner := &cancellationMissionExecutionRunner{
		state: state, plan: plan, started: make(chan struct{}),
	}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler, Runner: runner,
			VisibilityTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	preflight, err := backend.PreflightMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	start := command
	start.Operation = "start"
	start.PreflightDigest = preflight.PreflightDigest
	started, err := backend.StartMission(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	control := MissionExecutionCommand{
		SchemaVersion: 1, Operation: "control",
		MissionID: command.MissionID, TeamInstanceID: command.TeamInstanceID,
		ExpectedViewVersion: strings.Repeat("9", 64),
		ControlAction:       "cancel", ExecutionDigest: started.ExecutionDigest,
		LogicalNodeID: "main", AttemptNumber: 1, ClaimGeneration: 1,
		CorrelationID: "99999999-9999-4999-8999-999999999999",
	}
	if _, err := backend.ControlMission(
		context.Background(), control,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("stale view cancel error = %v", err)
	}
	control.ExpectedViewVersion = command.ExpectedViewVersion
	control.ClaimGeneration = 2
	if _, err := backend.ControlMission(
		context.Background(), control,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("stale generation cancel error = %v", err)
	}
	control.ClaimGeneration = 1
	result, err := backend.ControlMission(context.Background(), control)
	if err != nil || result.Status != "cancelled" {
		t.Fatalf("cancel result = %#v, %v", result, err)
	}
}

func TestAuthoritativeMissionPreparedControlRoutesExactCurrentLineage(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: command.ExpectedViewVersion,
		visible: true,
		execution: projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID,
			PlanDigest:     plan.Digest(),
			Status:         "awaiting_recovery",
			Nodes: []projection.TeamExecutionNode{{
				LogicalNodeID: "main", CurrentAttempt: 1,
				Attempts: []projection.TeamExecutionAttempt{{
					AttemptNumber: 1, ClaimGeneration: 1,
					Status: "awaiting_recovery",
				}},
			}},
		},
	}
	router := &controlledMissionExecutionDecisionRouter{}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state,
			Compiler: &controlledMissionExecutionCompiler{
				compilation: MissionExecutionCompilation{Plan: plan},
			},
			Runner:            incompleteMissionExecutionRunner{},
			Decisions:         router,
			VisibilityTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	flight, launch, err := backend.flight(
		command.TeamInstanceID,
		plan.Digest(),
		executionTestDigest("execution"),
	)
	if err != nil || !launch {
		t.Fatalf("flight = %#v, %v, %v", flight, launch, err)
	}
	close(flight.done)
	control := MissionExecutionCommand{
		SchemaVersion: 1, Operation: "control",
		MissionID: command.MissionID, TeamInstanceID: command.TeamInstanceID,
		ExpectedViewVersion: command.ExpectedViewVersion,
		ControlAction:       "retry",
		ExecutionDigest:     executionTestDigest("execution"),
		LogicalNodeID:       "main", AttemptNumber: 1, ClaimGeneration: 1,
		CorrelationID: "99999999-9999-4999-8999-999999999999",
	}
	drifted := control
	drifted.MissionID = "mission/team-other"
	if _, err := backend.ControlMission(
		context.Background(), drifted,
	); !errors.Is(err, ErrInvalidMissionExecution) || router.calls != 0 {
		t.Fatalf("mission identity drift error/calls = %v, %d", err, router.calls)
	}
	result, err := backend.ControlMission(context.Background(), control)
	if err != nil || router.calls != 1 ||
		!reflect.DeepEqual(router.command, control) ||
		result.Status != "awaiting_recovery" {
		t.Fatalf("prepared control = %#v, router=%#v, %v", result, router, err)
	}
}

func TestPreparedMissionExecutionDecisionRouterMapsExactRecoveryCommand(
	t *testing.T,
) {
	viewVersion := executionTestDigest("view")
	decision := MissionDecisionCommand{
		SchemaVersion: 1, Operation: "read", Kind: "recovery", Action: "read",
		MissionID: "mission/team-confirmed", TeamInstanceID: "team-confirmed",
		ViewVersion: viewVersion, DecisionID: "decision-recovery",
		DecisionDigest: executionTestDigest("decision"), LogicalNodeID: "main",
		AttemptNumber: 1, ClaimGeneration: 2,
		CorrelationID: "00000000-0000-4000-8000-000000000000",
	}
	source := &recordingMissionExecutionDecisionSource{
		commands: []MissionDecisionCommand{decision},
	}
	service := &recordingMissionExecutionDecisionService{
		result: MissionDecisionResult{
			SchemaVersion: 1, MissionID: decision.MissionID,
			DecisionID: decision.DecisionID, Status: "retry_scheduled",
			Authoritative: true, ViewVersion: executionTestDigest("next-view"),
		},
	}
	router := &PreparedMissionExecutionDecisionRouter{
		commands: source,
		service:  service,
		controls: map[string][]missionExecutionPreparedControl{
			decision.DecisionID: {{
				kind: "recovery", controlAction: "retry",
				decisionAction: "start_new_attempt", mutates: true,
			}},
		},
	}
	command := MissionExecutionCommand{
		SchemaVersion: 1, Operation: "control",
		MissionID: decision.MissionID, TeamInstanceID: decision.TeamInstanceID,
		ExpectedViewVersion: viewVersion, ControlAction: "retry",
		ExecutionDigest: executionTestDigest("execution"),
		LogicalNodeID:   decision.LogicalNodeID, AttemptNumber: decision.AttemptNumber,
		ClaimGeneration: decision.ClaimGeneration,
		CorrelationID:   "99999999-9999-4999-8999-999999999999",
	}
	if err := router.RouteMissionExecutionControl(
		context.Background(), command,
	); err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 || source.query != (MissionDecisionCommandQuery{
		ViewVersion: viewVersion, Mode: MissionDecisionCommandRefreshCurrent,
	}) {
		t.Fatalf("source calls/query = %d, %#v", source.calls, source.query)
	}
	want := decision
	want.Operation = "submit"
	want.Action = "start_new_attempt"
	want.CorrelationID = command.CorrelationID
	if service.calls != 1 || !reflect.DeepEqual(service.command, want) {
		t.Fatalf("service calls/command = %d, %#v; want %#v", service.calls, service.command, want)
	}
}

func TestAuthoritativeMissionPreflightExpiresWithoutViewChange(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{version: command.ExpectedViewVersion}
	compiler := &controlledMissionExecutionCompiler{
		compilation: MissionExecutionCompilation{
			Plan: plan, Request: TeamExecutionRequest{Plan: plan},
			Preflight: MissionExecutionPreflight{
				SchemaVersion: 1, MissionID: command.MissionID,
				TeamInstanceID:    command.TeamInstanceID,
				WorkPackageID:     command.WorkPackageID,
				WorkPackageDigest: command.WorkPackageDigest,
				ViewVersion:       command.ExpectedViewVersion,
				PlanDigest:        plan.Digest(), RuntimeInstanceID: "runtime-pi",
				RuntimeProfileID: "pi-default", ModelID: "qwen",
				AuthMode: "native", CapacityAvailable: 1,
				BudgetStatus: "unavailable", SideEffects: []string{},
				PermissionScopes: []string{}, ApprovalPoints: []string{},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", DependsOn: []string{}, MaxAttempts: 1,
				}},
			},
		},
	}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler,
			Runner:            incompleteMissionExecutionRunner{},
			VisibilityTimeout: time.Second,
			Now:               func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	preflight, err := backend.PreflightMission(context.Background(), command)
	if err != nil || preflight.ExpiresAt == "" {
		t.Fatalf("preflight = %#v, %v", preflight, err)
	}
	now = now.Add(6 * time.Minute)
	start := command
	start.Operation = "start"
	start.PreflightDigest = preflight.PreflightDigest
	if _, err := backend.StartMission(
		context.Background(),
		start,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("expired preflight start error = %v", err)
	}
}

func TestAuthoritativeMissionRestartResumesOneExactExpiredLineage(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-restart",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Resume exact work",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	projected := projection.TeamExecution{
		TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(),
		Status: "running",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "main", CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, RunID: "run-restart",
				ClaimGeneration: 1, Status: "running",
			}},
		}},
	}
	state := &controlledMissionExecutionState{
		version:    strings.Repeat("a", 64),
		executions: []projection.TeamExecution{projected},
		runs: map[string]projection.Run{
			"run-restart": {
				ID: "run-restart", ClaimGeneration: 1,
				PrepareLeaseExpiresAt: now.Add(-time.Second),
			},
		},
	}
	runner := &controlledTeamExecutionRunner{state: state}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state,
			Compiler: &controlledMissionExecutionCompiler{
				recoveryRequest: TeamExecutionRequest{Plan: plan},
			},
			Runner: runner, VisibilityTimeout: time.Second,
			Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.ResumeProjectedMissions(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for runner.Calls() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runner.Calls() != 1 {
		t.Fatalf("restart runner calls = %d", runner.Calls())
	}
	if err := backend.ResumeProjectedMissions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.Calls() != 1 {
		t.Fatalf("repeated reconciliation calls = %d", runner.Calls())
	}

	state.mu.Lock()
	state.executions[0].Status = "unknown_nonterminal"
	state.mu.Unlock()
	other, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state,
			Compiler: &controlledMissionExecutionCompiler{
				recoveryRequest: TeamExecutionRequest{Plan: plan},
			},
			Runner: runner, VisibilityTimeout: time.Second,
			Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := other.ResumeProjectedMissions(
		context.Background(),
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("unknown restart status error = %v", err)
	}
}

func TestAuthoritativeMissionExecutionCompletedFlightWaitIsTickerBounded(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: command.ExpectedViewVersion,
	}
	compiler := &controlledMissionExecutionCompiler{
		compilation: MissionExecutionCompilation{
			Plan: plan, Request: TeamExecutionRequest{Plan: plan},
			Preflight: MissionExecutionPreflight{
				SchemaVersion: 1, MissionID: command.MissionID,
				TeamInstanceID:    command.TeamInstanceID,
				WorkPackageID:     command.WorkPackageID,
				WorkPackageDigest: command.WorkPackageDigest,
				ViewVersion:       command.ExpectedViewVersion,
				PlanDigest:        plan.Digest(), RuntimeInstanceID: "runtime-pi",
				RuntimeProfileID: "pi-default", ModelID: "qwen",
				AuthMode: "brokered", CapacityAvailable: 1,
				BudgetStatus: "unavailable", SideEffects: []string{},
				PermissionScopes: []string{"workspace"},
				ApprovalPoints:   []string{"before_start"},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", DependsOn: []string{}, MaxAttempts: 1,
				}},
			},
		},
	}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler,
			Runner:            incompleteMissionExecutionRunner{},
			VisibilityTimeout: 40 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	preflight, err := backend.PreflightMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	start := command
	start.Operation = "start"
	start.PreflightDigest = preflight.PreflightDigest
	if _, err := backend.StartMission(
		context.Background(),
		start,
	); !errors.Is(err, ErrTeamExecutionIncomplete) {
		t.Fatalf("start error = %v", err)
	}
	_, refreshes := state.Stats()
	if refreshes > 20 {
		t.Fatalf("refreshes = %d, completed flight busy-spun", refreshes)
	}
}

func TestAuthoritativeMissionExecutionFlightRegistryIsBounded(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: command.Objective,
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: command.ExpectedViewVersion,
		byTeam:  make(map[string]projection.TeamExecution),
	}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state,
			Compiler: &controlledMissionExecutionCompiler{
				compilation: MissionExecutionCompilation{
					Plan: plan, Request: TeamExecutionRequest{Plan: plan},
					Preflight: MissionExecutionPreflight{
						PlanDigest: plan.Digest(),
					},
				},
			},
			Runner:            incompleteMissionExecutionRunner{},
			VisibilityTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for index := range maxAuthoritativeMissionFlights {
		flight, launch, flightErr := backend.flight(
			fmt.Sprintf("team-%03d", index),
			fmt.Sprintf("plan-%03d", index),
			fmt.Sprintf("execution-%03d", index),
		)
		if flightErr != nil || !launch || flight == nil {
			t.Fatalf("flight %d = %#v, %v, %v", index, flight, launch, flightErr)
		}
		close(flight.done)
		state.byTeam[fmt.Sprintf("team-%03d", index)] = projection.TeamExecution{
			TeamInstanceID: fmt.Sprintf("team-%03d", index),
			PlanDigest:     fmt.Sprintf("plan-%03d", index),
			Status:         "succeeded",
		}
	}
	if flight, launch, err := backend.flight(
		"team-overflow",
		"plan-overflow",
		"execution-overflow",
	); err != nil || !launch || flight == nil {
		t.Fatalf("terminal flight reaping = %#v, %v, %v", flight, launch, err)
	} else {
		close(flight.done)
	}
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltInMissionExecutionCompilerBindsExactRecipeAndVerifier(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	now := time.Date(2026, 8, 1, 9, 30, 0, 0, time.UTC)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "loom-main-native", AdapterType: "pi-cli",
		ProviderID:           "local",
		ModelID:              "qwen2.5-coder-1.5b-instruct-q4-k-m",
		AuthMode:             loomruntime.AuthNative,
		RequiredCapabilities: []string{"models"},
		Timeout:              5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-pi", DeviceID: "device-local",
		AdapterType: "pi-cli", DisplayName: "Local Pi",
		ExecutableVersion: "0.82.1", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"models"}, Capacity: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := &controlledMissionExecutionBindingSource{
		binding: MissionExecutionBinding{
			ViewVersion:     command.ExpectedViewVersion,
			TeamInstanceID:  command.TeamInstanceID,
			AgentInstanceID: "agent-main",
			Profile:         profile, Instance: instance,
			CapacityAvailable: 2,
		},
	}
	observer := controlledMissionExecutionOutputObserver{}
	factory := &controlledMissionExecutionObserverFactory{observer: observer}
	compiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings:        source,
			SourcePath:      t.TempDir(),
			ObserverFactory: factory,
			Now:             func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(
		context.Background(),
		command,
	)
	if err != nil {
		t.Fatal(err)
	}
	if source.calls != 1 ||
		compilation.Plan.TeamInstanceID() != command.TeamInstanceID ||
		compilation.Plan.Digest() == "" ||
		compilation.Preflight.PlanDigest != compilation.Plan.Digest() ||
		compilation.Preflight.ViewVersion != command.ExpectedViewVersion ||
		compilation.Preflight.RuntimeInstanceID != instance.ID ||
		compilation.Preflight.RuntimeProfileID != profile.ID ||
		compilation.Preflight.ModelID != profile.ModelID ||
		compilation.Preflight.AuthMode != string(profile.AuthMode) ||
		compilation.Preflight.CapacityAvailable != 2 ||
		compilation.Preflight.PreflightDigest != "" {
		t.Fatalf("compilation = %#v", compilation)
	}
	preflightNodes := compilation.Preflight.Nodes
	if len(preflightNodes) != 1 ||
		preflightNodes[0].LogicalNodeID != "main" ||
		preflightNodes[0].Title != command.Objective ||
		preflightNodes[0].Role != "main" ||
		preflightNodes[0].MaxAttempts != 2 ||
		preflightNodes[0].DependsOn == nil {
		t.Fatalf("preflight nodes = %#v", preflightNodes)
	}
	request := compilation.Request
	if request.Plan.Digest() != compilation.Plan.Digest() ||
		!request.AuthoritativeTime.Equal(now) ||
		request.CorrelationID != command.CorrelationID ||
		len(request.Nodes) != 2 || len(request.Semantics) != 1 {
		t.Fatalf("request = %#v", request)
	}
	semantics := request.Semantics[0]
	if semantics.LogicalNodeID != "main" ||
		semantics.OutputContract.Digest() == "" ||
		semantics.RecoveryPolicy.AttemptCredits() != 1 ||
		semantics.AcceptanceContract.Risk() != "medium" ||
		!semantics.AcceptanceContract.IndependentVerifierRequired() ||
		semantics.VerifierAgentInstanceID == "" ||
		semantics.VerifierAgentInstanceID == source.binding.AgentInstanceID ||
		semantics.VerifierRuntimeInstanceID != instance.ID ||
		semantics.VerifierExecution == nil {
		t.Fatalf("semantics = %#v", semantics)
	}
	for index, execution := range request.Nodes {
		if execution.LogicalNodeID != "main" ||
			execution.AttemptNumber != index+1 ||
			execution.Profile.ID != profile.ID ||
			execution.Instance.ID != instance.ID ||
			execution.Dispatch.WorkItemID() != appTeamAttemptIdentity(
				"work", compilation.Plan, "main", index+1,
			) ||
			execution.Dispatch.RunID() != appTeamAttemptIdentity(
				"run", compilation.Plan, "main", index+1,
			) ||
			execution.Dispatch.ClaimGeneration() != 1 ||
			execution.Dispatch.Payload() == nil || execution.Executor == nil {
			t.Fatalf("execution %d = %#v", index, execution)
		}
	}
	if compilation.Preflight.PermissionScopes == nil ||
		compilation.Preflight.SideEffects == nil ||
		compilation.Preflight.ApprovalPoints == nil {
		t.Fatalf("nil preflight collections = %#v", compilation.Preflight)
	}
	if factory.calls != 0 || compilation.Request.OutputObserver != nil {
		t.Fatalf("preflight created output observer: calls=%d", factory.calls)
	}
	start := command
	start.Operation = "start"
	start.PreflightDigest = strings.Repeat("e", 64)
	started, err := compiler.CompileMissionExecution(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	if factory.calls != 1 || factory.teamID != command.TeamInstanceID ||
		started.Request.OutputObserver != observer {
		t.Fatalf(
			"start observer = %#v calls=%d team=%q",
			started.Request.OutputObserver,
			factory.calls,
			factory.teamID,
		)
	}
	projectedSemantics := started.Request.Semantics[0]
	projected := projection.TeamExecution{
		TeamInstanceID: command.TeamInstanceID,
		PlanDigest:     started.Plan.Digest(),
		Status:         "running",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "main",
			Title:         command.Objective, AgentInstanceID: source.binding.AgentInstanceID,
			RuntimeInstanceID: source.binding.Instance.ID, Role: "main",
			DependsOn: []string{}, MaxAttempts: 2, Status: "running",
			OutputContractVersion:       projectedSemantics.OutputContract.Version(),
			OutputContractDigest:        projectedSemantics.OutputContract.Digest(),
			RecoveryPolicyVersion:       projectedSemantics.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:        projectedSemantics.RecoveryPolicy.Digest(),
			AttemptCredits:              projectedSemantics.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         projectedSemantics.PrimaryWorkflowPath,
			WorkflowFallbackKey:         projectedSemantics.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    projectedSemantics.RecoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   projectedSemantics.AcceptanceContract.Version(),
			AcceptanceContractDigest:    projectedSemantics.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(projectedSemantics.AcceptanceContract.Risk()),
			IndependentVerifierRequired: projectedSemantics.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     projectedSemantics.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   projectedSemantics.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        projectedSemantics.VerifierWorkflowPath,
		}},
	}
	recovered, err := compiler.ReconstructMissionExecution(
		context.Background(),
		projected,
	)
	if err != nil || recovered.Plan.Digest() != projected.PlanDigest ||
		recovered.OutputObserver != observer || factory.calls != 2 {
		t.Fatalf("recovered request = %#v, err=%v calls=%d", recovered, err, factory.calls)
	}
	projected.Nodes[0].PrimaryWorkflowPath = "unknown/recipe"
	if _, err := compiler.ReconstructMissionExecution(
		context.Background(),
		projected,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("unknown recipe recovery error = %v", err)
	}
}

func TestProjectionMissionExecutionBindingSourceUsesConfirmedExactPiTeam(
	t *testing.T,
) {
	database := openTeamCanaryDB(t)
	store := journal.NewStore(database)
	now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	digestA := fmt.Sprintf("%064x", 1)
	digestB := fmt.Sprintf("%064x", 2)
	digestC := fmt.Sprintf("%064x", 3)
	correlation := "11111111-1111-4111-8111-111111111111"
	appendEvent := func(event journal.Event) {
		t.Helper()
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	encode := func(value any) []byte {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	appendEvent(journal.Event{
		ID: "runtime-discovered", StreamID: "runtime_instance:runtime-pi",
		Seq: 1, IdempotencyKey: "idem-runtime-discovered",
		Type: "RuntimeInstanceDiscovered", SchemaVersion: 1,
		EmittedAt: now, CorrelationID: correlation,
		PayloadJSON: encode(map[string]any{
			"discovery_digest": digestA,
			"source_probe_id":  "probe-pi",
			"instance": map[string]any{
				"id": "runtime-pi", "device_id": "device-local",
				"adapter_type": "pi-cli", "display_name": "Local Pi",
				"executable_version": "0.82.1", "status": "online",
				"observed_capabilities": []string{
					"pi.metadata.models", "pi.metadata.version",
				},
				"capacity": 1,
			},
			"model_ids": []string{
				"loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m",
			},
		}),
	})
	appendEvent(journal.Event{
		ID: "team-created", StreamID: "team_instance:team-confirmed",
		Seq: 1, IdempotencyKey: "idem-team-created",
		Type: "TeamInstanceCreated", SchemaVersion: 1,
		EmittedAt: now.Add(time.Second), CorrelationID: correlation,
		PayloadJSON: encode(map[string]any{
			"team": map[string]any{
				"id": "team-confirmed", "work_request_id": correlation,
				"source_kind":             "saved_team",
				"team_definition_id":      "team.delivery",
				"team_definition_version": 1,
				"team_definition_scope":   "project",
				"scope_identity": map[string]any{
					"project_id": "project-one", "generation_id": "",
				},
				"team_definition_digest": digestB,
				"source_plan_digest":     digestC,
				"state":                  "created", "created_at": int64(1_722_508_400),
			},
			"dormant_sub_agents":       []any{},
			"source_plan_digest":       digestC,
			"source_record_set_digest": digestB,
			"team_instance_count":      1,
			"agent_instance_count":     1,
			"active_sub_agent_count":   0,
			"work_item_count":          0,
		}),
	})
	appendEvent(journal.Event{
		ID: "agent-created", StreamID: "agent_instance:agent-main",
		Seq: 1, IdempotencyKey: "idem-agent-created",
		Type: "AgentInstanceCreated", SchemaVersion: 1,
		EmittedAt: now.Add(2 * time.Second), CorrelationID: correlation,
		CausationID: "team-created",
		PayloadJSON: encode(map[string]any{
			"main_agent": map[string]any{
				"id": "agent-main", "team_instance_id": "team-confirmed",
				"agent_definition_id":      "loom-main-coordinator",
				"agent_definition_version": 1,
				"agent_definition_scope":   "project",
				"scope_identity": map[string]any{
					"project_id": "project-one", "generation_id": "",
				},
				"runtime_profile_id":  "loom-main-native",
				"runtime_instance_id": "runtime-pi",
				"is_main":             true, "state": "created",
			},
			"runtime_binding": map[string]any{
				"accepted": true, "profile_id": "loom-main-native",
				"instance_id": "runtime-pi",
			},
			"source_plan_digest":       digestC,
			"source_record_set_digest": digestB,
			"team_created_at":          int64(1_722_508_400),
			"binding_digest":           digestC,
			"runtime_discovery_digest": digestA,
		}),
	})
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	source, err := NewProjectionMissionExecutionBindingSource(readModel)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := source.ResolveMissionExecutionBinding(
		context.Background(),
		"team-confirmed",
	)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ViewVersion != readModel.GlobalReadView().Version() ||
		binding.TeamInstanceID != "team-confirmed" ||
		binding.AgentInstanceID != "agent-main" ||
		binding.Profile.ID != "loom-main-native" ||
		binding.Profile.ProviderID != "loom-local" ||
		binding.Profile.ModelID !=
			"qwen2.5-coder-1.5b-instruct-q4-k-m" ||
		binding.Profile.AuthMode != loomruntime.AuthNative ||
		binding.Instance.ID != "runtime-pi" ||
		binding.CapacityAvailable != 1 {
		t.Fatalf("binding = %#v", binding)
	}
}

func TestLockedLocalPiModelIdentityRejectsAliasesAndAmbiguity(t *testing.T) {
	valid := "loom-local/qwen2.5-coder-1.5b-instruct-q4-k-m"
	provider, model, ok := parseLockedLocalPiModelIdentity(valid)
	if !ok || provider != "loom-local" ||
		model != "qwen2.5-coder-1.5b-instruct-q4-k-m" {
		t.Fatalf("valid identity = %q %q %v", provider, model, ok)
	}
	for _, invalid := range []string{
		"", "loom-local", "/qwen2.5-coder-1.5b-instruct-q4-k-m",
		"loom-local/", "local/qwen2.5-coder-1.5b-instruct-q4-k-m",
		"loom-local/other", "prefix/" + valid, valid + "/suffix",
	} {
		if provider, model, ok := parseLockedLocalPiModelIdentity(invalid); ok ||
			provider != "" || model != "" {
			t.Fatalf("invalid identity %q = %q %q %v", invalid, provider, model, ok)
		}
	}
}

func (backend *recordingMissionExecutionBackend) PreflightMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionPreflight, error) {
	if err := ctx.Err(); err != nil {
		return MissionExecutionPreflight{}, err
	}
	backend.commands = append(backend.commands, command)
	return backend.preflight, backend.err
}

func (backend *recordingMissionExecutionBackend) StartMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return MissionExecutionResult{}, err
	}
	backend.commands = append(backend.commands, command)
	return backend.result, backend.err
}

func (backend *recordingMissionExecutionBackend) ControlMission(
	ctx context.Context,
	command MissionExecutionCommand,
) (MissionExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return MissionExecutionResult{}, err
	}
	backend.commands = append(backend.commands, command)
	return backend.result, backend.err
}

func TestLocalProductExecutionPreflightIsReadOnlyBoundedAndCopied(t *testing.T) {
	command := missionExecutionTestCommand("preflight")
	want := MissionExecutionPreflight{
		SchemaVersion:     MissionExecutionSchemaVersion,
		MissionID:         command.MissionID,
		TeamInstanceID:    command.TeamInstanceID,
		WorkPackageID:     command.WorkPackageID,
		WorkPackageDigest: command.WorkPackageDigest,
		ViewVersion:       command.ExpectedViewVersion,
		PlanDigest:        executionTestDigest("plan"),
		PreflightDigest:   executionTestDigest("preflight"),
		ExpiresAt:         "2026-08-01T12:05:00Z",
		RuntimeInstanceID: "runtime-pi",
		RuntimeProfileID:  "pi-default",
		ModelID:           "qwen2.5-coder-1.5b",
		AuthMode:          "local",
		CapacityAvailable: 1,
		BudgetStatus:      "unavailable",
		SideEffects:       []string{"workspace_write"},
		PermissionScopes:  []string{"workspace"},
		ApprovalPoints:    []string{"before_workspace_write"},
		Nodes: []MissionExecutionNodePreview{{
			LogicalNodeID: "main",
			Title:         command.Objective,
			Role:          "main",
			DependsOn:     []string{},
			MaxAttempts:   2,
		}},
	}
	backend := &recordingMissionExecutionBackend{preflight: want}
	service, err := NewLocalProductExecutionService(
		LocalProductExecutionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.PreflightMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("preflight = %#v, want %#v", first, want)
	}
	if len(backend.commands) != 1 || !reflect.DeepEqual(backend.commands[0], command) {
		t.Fatalf("backend commands = %#v", backend.commands)
	}

	first.Nodes[0].Title = "mutated"
	first.SideEffects[0] = "mutated"
	second, err := service.PreflightMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("preflight alias leaked = %#v", second)
	}
}

func TestLocalProductExecutionRejectsInvalidClosedCommandsBeforeBackend(
	t *testing.T,
) {
	backend := &recordingMissionExecutionBackend{}
	service, err := NewLocalProductExecutionService(
		LocalProductExecutionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}
	valid := missionExecutionTestCommand("preflight")
	invalid := []MissionExecutionCommand{
		{},
		func() MissionExecutionCommand { value := valid; value.SchemaVersion++; return value }(),
		func() MissionExecutionCommand { value := valid; value.Operation = "execute"; return value }(),
		func() MissionExecutionCommand { value := valid; value.MissionID = ""; return value }(),
		func() MissionExecutionCommand {
			value := valid
			value.MissionID = "mission/team-other"
			return value
		}(),
		func() MissionExecutionCommand { value := valid; value.TeamInstanceID = ""; return value }(),
		func() MissionExecutionCommand { value := valid; value.WorkPackageID = ""; return value }(),
		func() MissionExecutionCommand {
			value := valid
			value.WorkPackageDigest = executionTestDigest("wrong-work-package")
			return value
		}(),
		func() MissionExecutionCommand { value := valid; value.Objective = ""; return value }(),
		func() MissionExecutionCommand { value := valid; value.ExpectedViewVersion = ""; return value }(),
		func() MissionExecutionCommand { value := valid; value.CorrelationID = ""; return value }(),
		func() MissionExecutionCommand { value := valid; value.ControlAction = "cancel"; return value }(),
	}
	for index, command := range invalid {
		if _, err := service.PreflightMission(context.Background(), command); !errors.Is(
			err,
			ErrInvalidMissionExecution,
		) {
			t.Fatalf("invalid[%d] error = %v", index, err)
		}
	}
	if len(backend.commands) != 0 {
		t.Fatalf("invalid commands reached backend: %#v", backend.commands)
	}
}

func TestLocalProductExecutionStartAndControlRequireExactOperationShape(
	t *testing.T,
) {
	want := MissionExecutionResult{
		SchemaVersion:   MissionExecutionSchemaVersion,
		MissionID:       "mission/team-1",
		TeamInstanceID:  "team-1",
		Status:          "running",
		ViewVersion:     executionTestDigest("view-next"),
		ExecutionDigest: executionTestDigest("execution"),
	}
	backend := &recordingMissionExecutionBackend{result: want}
	service, err := NewLocalProductExecutionService(
		LocalProductExecutionConfig{Backend: backend},
	)
	if err != nil {
		t.Fatal(err)
	}

	start := missionExecutionTestCommand("start")
	start.PreflightDigest = executionTestDigest("preflight")
	started, err := service.StartMission(context.Background(), start)
	if err != nil || !reflect.DeepEqual(started, want) {
		t.Fatalf("start = %#v, %v", started, err)
	}

	control := missionExecutionTestCommand("control")
	control.WorkPackageID = ""
	control.WorkPackageDigest = ""
	control.Objective = ""
	control.PreflightDigest = ""
	control.ControlAction = "cancel"
	control.ExecutionDigest = want.ExecutionDigest
	control.LogicalNodeID = "main"
	control.AttemptNumber = 1
	control.ClaimGeneration = 1
	controlled, err := service.ControlMission(context.Background(), control)
	if err != nil || !reflect.DeepEqual(controlled, want) {
		t.Fatalf("control = %#v, %v", controlled, err)
	}

	if len(backend.commands) != 2 ||
		backend.commands[0].Operation != "start" ||
		backend.commands[1].Operation != "control" {
		t.Fatalf("backend commands = %#v", backend.commands)
	}
}

func missionExecutionTestCommand(operation string) MissionExecutionCommand {
	workPackage, err := work.CodingWorkPackage()
	if err != nil {
		panic(err)
	}
	return MissionExecutionCommand{
		SchemaVersion:       MissionExecutionSchemaVersion,
		Operation:           operation,
		MissionID:           "mission/team-1",
		TeamInstanceID:      "team-1",
		WorkPackageID:       workPackage.ID(),
		WorkPackageDigest:   workPackage.Digest(),
		Objective:           "Implement the bounded change",
		ExpectedViewVersion: executionTestDigest("view"),
		CorrelationID:       "11111111-1111-4111-8111-111111111111",
	}
}

func executionTestDigest(label string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(label)))
}
