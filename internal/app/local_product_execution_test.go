package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/supervisor"
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
	grants     map[string]projection.AgentGrant
	visible    bool
	refreshes  int
}

func (state *controlledMissionExecutionState) TeamExecutions(
	afterTeamID string,
	limit int,
) ([]projection.TeamExecution, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	start := 0
	for start < len(state.executions) &&
		state.executions[start].TeamInstanceID <= afterTeamID {
		start++
	}
	end := min(start+limit, len(state.executions))
	executions := make([]projection.TeamExecution, end-start)
	for index := range executions {
		executions[index] = cloneControlledMissionExecution(
			state.executions[start+index],
		)
	}
	return executions, end < len(state.executions)
}

func (state *controlledMissionExecutionState) Run(
	runID string,
) (projection.Run, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	run, ok := state.runs[runID]
	return run, ok
}

func (state *controlledMissionExecutionState) LatestAgentGrantForRun(
	runID string,
) (projection.AgentGrant, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	grant, ok := state.grants[runID]
	return grant, ok
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
		return cloneControlledMissionExecution(execution), ok
	}
	return cloneControlledMissionExecution(state.execution),
		state.visible && state.execution.TeamInstanceID == teamID
}

func cloneControlledMissionExecution(
	execution projection.TeamExecution,
) projection.TeamExecution {
	clone := execution
	clone.Nodes = append([]projection.TeamExecutionNode(nil), execution.Nodes...)
	for index := range clone.Nodes {
		source := execution.Nodes[index]
		clone.Nodes[index].DependsOn = append([]string(nil), source.DependsOn...)
		clone.Nodes[index].PriorClassifications = append(
			[]string(nil), source.PriorClassifications...,
		)
		clone.Nodes[index].InitialCapabilities = append(
			[]string(nil), source.InitialCapabilities...,
		)
		if source.InitialBudget != nil {
			budget := *source.InitialBudget
			clone.Nodes[index].InitialBudget = &budget
		}
		clone.Nodes[index].AssetRevisionBindings = append(
			[]assets.ExactAssetRevisionBinding(nil),
			source.AssetRevisionBindings...,
		)
		clone.Nodes[index].Attempts = append(
			[]projection.TeamExecutionAttempt(nil), source.Attempts...,
		)
		for attemptIndex := range clone.Nodes[index].Attempts {
			attemptSource := source.Attempts[attemptIndex]
			clone.Nodes[index].Attempts[attemptIndex].AssetRevisionBindings = append(
				[]assets.ExactAssetRevisionBinding(nil),
				attemptSource.AssetRevisionBindings...,
			)
			binding := attemptSource.ExecutionBinding
			binding.Capabilities = append([]string(nil), binding.Capabilities...)
			if binding.Budget != nil {
				budget := *binding.Budget
				binding.Budget = &budget
			}
			clone.Nodes[index].Attempts[attemptIndex].ExecutionBinding = binding
		}
	}
	return clone
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

type recordingMissionFallbackDecisionPreparer struct {
	mu         sync.Mutex
	teamID     string
	view       string
	candidates []MissionFallbackDecisionCandidate
	calls      int
	err        error
}

type unavailableMissionFallbackDecisionPreparer struct {
	calls int
}

func (preparer *unavailableMissionFallbackDecisionPreparer) PrepareMissionFallbackDecisions(
	context.Context,
	string,
	string,
	[]MissionFallbackDecisionCandidate,
) error {
	preparer.calls++
	return ErrInvalidMissionDecision
}

func (preparer *recordingMissionFallbackDecisionPreparer) PrepareMissionFallbackDecisions(
	_ context.Context,
	teamID string,
	viewVersion string,
	candidates []MissionFallbackDecisionCandidate,
) error {
	preparer.mu.Lock()
	defer preparer.mu.Unlock()
	preparer.calls++
	preparer.teamID = teamID
	preparer.view = viewVersion
	preparer.candidates = append(
		[]MissionFallbackDecisionCandidate(nil), candidates...,
	)
	return preparer.err
}

type controlledMissionFallbackApprovalSource struct {
	now                 time.Time
	targetBindingDigest string
	queries             []MissionFallbackApprovalQuery
}

func (source *controlledMissionFallbackApprovalSource) ResolveMissionFallbackApproval(
	_ context.Context,
	query MissionFallbackApprovalQuery,
) (work.TeamFallbackApproval, bool, error) {
	source.queries = append(source.queries, query)
	targetBindingDigest := query.TargetBindingDigest
	if source.targetBindingDigest != "" {
		targetBindingDigest = source.targetBindingDigest
	}
	approval, err := work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
		Version: 1, ApprovalID: "fallback-approval-main-v1",
		ActorRef: "user:local-owner", ApprovedAt: source.now,
		SourceBindingDigest: query.SourceBindingDigest,
		TargetBindingDigest: targetBindingDigest,
	})
	return approval, err == nil, err
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
	_ context.Context,
	_ MissionExecutionCommand,
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
			Status:         "running",
			CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber:     1,
				WorkItemID:        "work-main-1",
				RunID:             "run-main-1",
				ClaimID:           "claim-main-1",
				ClaimGeneration:   1,
				RuntimeInstanceID: "runtime-pi",
				AgentInstanceID:   "agent-main",
				Status:            "dispatched",
			}},
		}},
	}
	if runner.state.runs == nil {
		runner.state.runs = make(map[string]projection.Run)
	}
	runner.state.runs["run-main-1"] = projection.Run{
		ID: "run-main-1", WorkItemID: "work-main-1", Phase: "running",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
	}
	if runner.state.grants == nil {
		runner.state.grants = make(map[string]projection.AgentGrant)
	}
	runner.state.grants["run-main-1"] = projection.AgentGrant{
		ID: "grant-main-1", WorkItemID: "work-main-1", RunID: "run-main-1",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
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

type continuationMissionExecutionRunner struct {
	state *controlledMissionExecutionState
	calls int
}

func (runner *continuationMissionExecutionRunner) Run(
	context.Context,
	TeamExecutionRequest,
) (TeamExecutionResult, error) {
	runner.calls++
	runner.state.mu.Lock()
	defer runner.state.mu.Unlock()
	runner.state.execution.Nodes[0].Status = "running"
	runner.state.execution.Nodes[0].CurrentAttempt = 1
	runner.state.execution.Nodes[0].Attempts = []projection.TeamExecutionAttempt{{
		AttemptNumber: 1, WorkItemID: "work-main-1", RunID: "run-main-1",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
		Status: "dispatched",
	}}
	if runner.state.runs == nil {
		runner.state.runs = make(map[string]projection.Run)
	}
	runner.state.runs["run-main-1"] = projection.Run{
		ID: "run-main-1", WorkItemID: "work-main-1", Phase: "running",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
	}
	if runner.state.grants == nil {
		runner.state.grants = make(map[string]projection.AgentGrant)
	}
	runner.state.grants["run-main-1"] = projection.AgentGrant{
		ID: "grant-main-1", WorkItemID: "work-main-1", RunID: "run-main-1",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
	}
	return TeamExecutionResult{}, ErrTeamExecutionIncomplete
}

type preProjectionCancellationRunner struct {
	entered chan struct{}
	exited  chan struct{}
}

type stagedStartLineageRunner struct {
	state      *controlledMissionExecutionState
	plan       teams.ExecutionPlan
	dispatched chan struct{}
	release    chan struct{}
}

func (runner *stagedStartLineageRunner) Run(
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
			LogicalNodeID: "main", Status: "running", CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, WorkItemID: "work-main-1", RunID: "run-main-1",
				ClaimID: "claim-main-1", ClaimGeneration: 1,
				RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
				Status: "dispatched",
			}},
		}},
	}
	runner.state.mu.Unlock()
	close(runner.dispatched)
	select {
	case <-ctx.Done():
		return TeamExecutionResult{}, ctx.Err()
	case <-runner.release:
	}
	runner.state.mu.Lock()
	runner.state.runs = map[string]projection.Run{
		"run-main-1": {
			ID: "run-main-1", WorkItemID: "work-main-1", Phase: "running",
			ClaimID: "claim-main-1", ClaimGeneration: 1,
		},
	}
	runner.state.grants = map[string]projection.AgentGrant{
		"run-main-1": {
			ID: "grant-main-1", WorkItemID: "work-main-1", RunID: "run-main-1",
			ClaimID: "claim-main-1", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
		},
	}
	runner.state.mu.Unlock()
	return TeamExecutionResult{}, ErrTeamExecutionIncomplete
}

func (runner *preProjectionCancellationRunner) Run(
	ctx context.Context,
	_ TeamExecutionRequest,
) (TeamExecutionResult, error) {
	close(runner.entered)
	<-ctx.Done()
	close(runner.exited)
	return TeamExecutionResult{}, ctx.Err()
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

type recordedMissionContextCapsule struct {
	authority contextcapsule.AuthorityRecord
	capsule   contextcapsule.RoleContextCapsule
	payload   []byte
}

type controlledMissionContextCapsuleStore struct {
	records []recordedMissionContextCapsule
	err     error
}

func (store *controlledMissionContextCapsuleStore) PutRoleContextCapsule(
	_ context.Context,
	capsule contextcapsule.RoleContextCapsule,
	payload []byte,
) error {
	if store.err != nil {
		return store.err
	}
	store.records = append(store.records, recordedMissionContextCapsule{
		authority: capsule.AuthorityRecord(),
		capsule:   capsule,
		payload:   append([]byte(nil), payload...),
	})
	return nil
}

func (store *controlledMissionContextCapsuleStore) ReadRoleContextCapsule(
	_ context.Context,
	authority contextcapsule.AuthorityRecord,
) (contextcapsule.RoleContextCapsule, []byte, error) {
	if store.err != nil {
		return contextcapsule.RoleContextCapsule{}, nil, store.err
	}
	for _, record := range store.records {
		if record.authority == authority {
			return record.capsule, append([]byte(nil), record.payload...), nil
		}
	}
	return contextcapsule.RoleContextCapsule{}, nil, errors.New("capsule not found")
}

func (store *controlledMissionContextCapsuleStore) ListRoleContextCapsuleAuthorities(
	_ context.Context,
	conversationID string,
) ([]contextcapsule.AuthorityRecord, error) {
	if store.err != nil {
		return nil, store.err
	}
	records := make([]contextcapsule.AuthorityRecord, 0, len(store.records))
	seen := make(map[contextcapsule.AuthorityRecord]struct{}, len(store.records))
	for _, stored := range store.records {
		if stored.authority.ConversationID != conversationID {
			continue
		}
		if _, duplicate := seen[stored.authority]; duplicate {
			continue
		}
		seen[stored.authority] = struct{}{}
		records = append(records, stored.authority)
	}
	return records, nil
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
	if len(runner.state.execution.Nodes) == 0 {
		runner.state.execution.Nodes = []projection.TeamExecutionNode{{
			LogicalNodeID: "main", Status: "running", CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, WorkItemID: "work-main-1", RunID: "run-main-1",
				ClaimID: "claim-main-1", ClaimGeneration: 1,
				RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
				Status: "dispatched",
			}},
		}}
	}
	if runner.state.runs == nil {
		runner.state.runs = make(map[string]projection.Run)
	}
	runner.state.runs["run-main-1"] = projection.Run{
		ID: "run-main-1", WorkItemID: "work-main-1", Phase: "running",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
	}
	if runner.state.grants == nil {
		runner.state.grants = make(map[string]projection.AgentGrant)
	}
	runner.state.grants["run-main-1"] = projection.AgentGrant{
		ID: "grant-main-1", WorkItemID: "work-main-1", RunID: "run-main-1",
		ClaimID: "claim-main-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-pi", AgentInstanceID: "agent-main",
	}
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
			FallbackDecisions: []MissionFallbackDecisionCandidate{{
				MissionID: command.MissionID,
				Scope: mustMissionFallbackDecisionScope(t,
					command.TeamInstanceID, plan.Digest(), "main",
					strings.Repeat("1", 64), strings.Repeat("2", 64),
				),
				AgentTitle: "Main Agent", HarnessAdapter: "loom-native",
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				ModelID: "deepseek-chat", CredentialRevision: 6,
			}},
		},
	}
	decisionPreparer := &recordingMissionFallbackDecisionPreparer{}
	runner := &controlledTeamExecutionRunner{state: state}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler, Runner: runner,
			FallbackDecisions: decisionPreparer,
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
	if decisionPreparer.calls != 1 ||
		decisionPreparer.teamID != command.TeamInstanceID ||
		decisionPreparer.view != command.ExpectedViewVersion ||
		len(decisionPreparer.candidates) != 1 ||
		decisionPreparer.candidates[0].Scope.PlanDigest() != plan.Digest() {
		t.Fatalf("fallback decision preparation = %#v", decisionPreparer)
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

func TestAuthoritativeMissionPreflightSkipsFallbackGovernanceWithoutCandidates(
	t *testing.T,
) {
	command := missionExecutionTestCommand(missionExecutionPreflight)
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
	compiler := &controlledMissionExecutionCompiler{
		compilation: MissionExecutionCompilation{
			Plan:    plan,
			Request: TeamExecutionRequest{Plan: plan},
			Preflight: MissionExecutionPreflight{
				SchemaVersion: MissionExecutionSchemaVersion,
				MissionID:     command.MissionID, TeamInstanceID: command.TeamInstanceID,
				WorkPackageID:     command.WorkPackageID,
				WorkPackageDigest: command.WorkPackageDigest,
				ViewVersion:       command.ExpectedViewVersion, PlanDigest: plan.Digest(),
				RuntimeInstanceID: "runtime-pi", RuntimeProfileID: "pi-default",
				ModelID: "qwen", AuthMode: "native", CapacityAvailable: 1,
				BudgetStatus: "unavailable", SideEffects: []string{},
				PermissionScopes: []string{"workspace"},
				ApprovalPoints:   []string{"before_start"},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", DependsOn: []string{}, MaxAttempts: 2,
				}},
			},
		},
	}
	preparer := &unavailableMissionFallbackDecisionPreparer{}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler, Runner: incompleteMissionExecutionRunner{},
			FallbackDecisions: preparer, VisibilityTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if _, err := backend.PreflightMission(context.Background(), command); err != nil {
		t.Fatalf("preflight without fallback candidates = %v", err)
	}
	if preparer.calls != 0 {
		t.Fatalf("fallback governance called without candidates: %d", preparer.calls)
	}
}

func TestAuthoritativeMissionExecutionStartRejectsChangedObjective(t *testing.T) {
	// The preflight digest freezes the full Mission Context command, including
	// the objective. Start must replay the exact command that was preflighted;
	// a client that swaps the objective between preflight and start must get a
	// conflict instead of launching against a stale digest. This guards the
	// live web-mission regression where preflight used one objective and start
	// another (plan digest drifted -> mission_execution start conflict).
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
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler,
			Runner: &controlledTeamExecutionRunner{state: state},
			// No fallback preparer: keep the view stable so the only drift is
			// the command objective.
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

	drifted := command
	drifted.Operation = "start"
	drifted.Objective = "A different objective than the one preflighted"
	drifted.CorrelationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	drifted.PreflightDigest = preflight.PreflightDigest
	if _, err := backend.StartMission(context.Background(), drifted); !errors.Is(
		err, ErrMissionExecutionConflict,
	) {
		t.Fatalf("start with drifted objective: err=%v want ErrMissionExecutionConflict", err)
	}

	// Control: replaying the exact preflighted command starts successfully.
	replay := command
	replay.Operation = "start"
	replay.CorrelationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	replay.PreflightDigest = preflight.PreflightDigest
	result, err := backend.StartMission(context.Background(), replay)
	if err != nil || result.Status != "running" {
		t.Fatalf("start replay = %#v, err=%v want running", result, err)
	}
}

func TestAuthoritativeMissionExecutionCallerCancellationJoinsUnreportedFlight(
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
	runner := &preProjectionCancellationRunner{
		entered: make(chan struct{}),
		exited:  make(chan struct{}),
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
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, startErr := backend.StartMission(ctx, start)
		result <- startErr
	}()
	<-runner.entered
	cancel()
	if startErr := <-result; !errors.Is(startErr, context.Canceled) {
		t.Fatalf("canceled start error = %v", startErr)
	}
	select {
	case <-runner.exited:
	default:
		t.Fatal("canceled start returned before its unreported flight joined")
	}
	if _, visible := state.TeamExecution(command.TeamInstanceID); visible {
		t.Fatal("canceled pre-projection start manufactured authority")
	}
	backend.mu.Lock()
	flight := backend.flights[command.TeamInstanceID]
	backend.mu.Unlock()
	if flight != nil {
		t.Fatal("joined zero-authority flight still blocks an exact retry")
	}
}

func TestAuthoritativeMissionExecutionDoesNotReturnFromDispatchOnlyProjection(
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
	runner := &stagedStartLineageRunner{
		state: state, plan: plan,
		dispatched: make(chan struct{}), release: make(chan struct{}),
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
	result := make(chan struct {
		value MissionExecutionResult
		err   error
	}, 1)
	go func() {
		value, startErr := backend.StartMission(context.Background(), start)
		result <- struct {
			value MissionExecutionResult
			err   error
		}{value: value, err: startErr}
	}()
	<-runner.dispatched
	select {
	case early := <-result:
		t.Fatalf("dispatch-only projection returned early: %#v, %v", early.value, early.err)
	case <-time.After(30 * time.Millisecond):
	}
	close(runner.release)
	select {
	case outcome := <-result:
		if outcome.err != nil || outcome.value.Status != "running" {
			t.Fatalf("complete start lineage result = %#v, %v", outcome.value, outcome.err)
		}
	case <-time.After(time.Second):
		t.Fatal("complete start lineage did not become visible")
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
	if err := other.ResumeProjectedMissions(context.Background()); err != nil {
		t.Fatalf("unknown restart status must not brick startup: %v", err)
	}
	if runner.Calls() != 1 {
		t.Fatalf("unknown restart status runner calls = %d", runner.Calls())
	}
}

func TestAuthoritativeMissionRestartPagesPastTerminalHistory(t *testing.T) {
	now := time.Date(2026, 8, 21, 13, 20, 0, 0, time.UTC)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-zzz-running",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Resume after terminal history",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	executions := make([]projection.TeamExecution, 0, maxAuthoritativeMissionFlights+1)
	for index := 0; index < maxAuthoritativeMissionFlights; index++ {
		executions = append(executions, projection.TeamExecution{
			TeamInstanceID: fmt.Sprintf("team-%03d-terminal", index),
			PlanDigest:     strings.Repeat("a", 64), Status: "succeeded",
		})
	}
	executions = append(executions, projection.TeamExecution{
		TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(), Status: "running",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "main", CurrentAttempt: 1,
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, RunID: "run-paged-restart",
				ClaimGeneration: 1, Status: "running",
			}},
		}},
	})
	state := &controlledMissionExecutionState{
		version: strings.Repeat("b", 64), executions: executions,
		runs: map[string]projection.Run{
			"run-paged-restart": {
				ID: "run-paged-restart", ClaimGeneration: 1,
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
		t.Fatalf("paged restart runner calls = %d", runner.Calls())
	}
}

func TestAuthoritativeMissionRestartSkipsUnresumableProjection(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-unresumable",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Bounded task",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 2,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: strings.Repeat("a", 64),
		executions: []projection.TeamExecution{{
			TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(),
			Status: "running",
			Nodes: []projection.TeamExecutionNode{{
				LogicalNodeID: "main", Status: "running", CurrentAttempt: 1,
			}},
		}},
	}
	runner := &controlledTeamExecutionRunner{state: state}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state,
			Compiler: &controlledMissionExecutionCompiler{
				recoveryErr: work.ErrTeamExecutionConflict,
			},
			Runner: runner, VisibilityTimeout: time.Second,
			Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	// A projected Mission that cannot be reconstructed (for example its
	// execution binding drifted after the App was killed) must be skipped,
	// not treated as a fatal startup error.
	if err := backend.ResumeProjectedMissions(context.Background()); err != nil {
		t.Fatalf("unresumable projection must not brick startup: %v", err)
	}
	if runner.Calls() != 0 {
		t.Fatalf("unresumable projection runner calls = %d", runner.Calls())
	}
}

func TestAuthoritativeMissionRestartKeepsReadyForReviewQuiescent(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: "team-review-restart",
		Nodes: []teams.ExecutionNodeInput{{
			LogicalNodeID: "main", Title: "Await human review",
			AgentInstanceID: "agent-main", RuntimeInstanceID: "runtime-pi",
			Role: teams.ExecutionRoleMain, MaxAttempts: 1,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	state := &controlledMissionExecutionState{
		version: strings.Repeat("a", 64),
		executions: []projection.TeamExecution{{
			TeamInstanceID: plan.TeamInstanceID(), PlanDigest: plan.Digest(),
			Status: "running",
			Nodes: []projection.TeamExecutionNode{{
				LogicalNodeID: "main", Status: "ready_for_review",
			}},
		}},
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
	if runner.Calls() != 0 {
		t.Fatalf("ready-for-review restart runner calls = %d", runner.Calls())
	}
}

func TestMissionExecutionAwaitingHumanReview(t *testing.T) {
	tests := []struct {
		name      string
		execution projection.TeamExecution
		want      bool
	}{
		{
			name: "single ready node",
			execution: projection.TeamExecution{
				Status: "running",
				Nodes:  []projection.TeamExecutionNode{{Status: "ready_for_review"}},
			},
			want: true,
		},
		{
			name: "succeeded and ready nodes",
			execution: projection.TeamExecution{
				Status: "running",
				Nodes: []projection.TeamExecutionNode{
					{Status: "succeeded"},
					{Status: "ready_for_review"},
				},
			},
			want: true,
		},
		{
			name: "ready and pending nodes",
			execution: projection.TeamExecution{
				Status: "running",
				Nodes: []projection.TeamExecutionNode{
					{Status: "ready_for_review"},
					{Status: "pending"},
				},
			},
			want: false,
		},
		{
			name: "recovery aggregate",
			execution: projection.TeamExecution{
				Status: "awaiting_recovery",
				Nodes:  []projection.TeamExecutionNode{{Status: "ready_for_review"}},
			},
			want: false,
		},
		{
			name:      "empty running execution",
			execution: projection.TeamExecution{Status: "running"},
			want:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := missionExecutionAwaitingHumanReview(test.execution); got != test.want {
				t.Fatalf("missionExecutionAwaitingHumanReview() = %t, want %t", got, test.want)
			}
		})
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

func TestAuthoritativeMissionExecutionContinuesPendingNodeAfterCompletedFlight(
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
		version: command.ExpectedViewVersion, visible: true,
		execution: projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID, PlanDigest: plan.Digest(),
			Status: "running",
			Nodes: []projection.TeamExecutionNode{{
				LogicalNodeID: "main", Status: "pending",
			}},
		},
	}
	runner := &continuationMissionExecutionRunner{state: state}
	compiler := &controlledMissionExecutionCompiler{
		compilation: MissionExecutionCompilation{
			Plan: plan, Request: TeamExecutionRequest{Plan: plan},
			Preflight: MissionExecutionPreflight{
				SchemaVersion: 1, MissionID: command.MissionID,
				TeamInstanceID:    command.TeamInstanceID,
				WorkPackageID:     command.WorkPackageID,
				WorkPackageDigest: command.WorkPackageDigest,
				ViewVersion:       command.ExpectedViewVersion, PlanDigest: plan.Digest(),
				RuntimeInstanceID: "runtime-pi", RuntimeProfileID: "pi-default",
				ModelID: "qwen", AuthMode: "brokered", CapacityAvailable: 1,
				BudgetStatus: "unavailable", PermissionScopes: []string{"workspace"},
				ApprovalPoints: []string{"before_start"},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", MaxAttempts: 2,
				}},
			},
		},
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
	executionDigest := missionExecutionDigest(
		command.TeamInstanceID, plan.Digest(), preflight.PreflightDigest,
	)
	previous, launch, err := backend.flight(
		command.TeamInstanceID, plan.Digest(), executionDigest,
	)
	if err != nil || !launch {
		t.Fatalf("previous flight = %#v, launch=%t, err=%v", previous, launch, err)
	}
	close(previous.done)
	start := command
	start.Operation = "start"
	start.PreflightDigest = preflight.PreflightDigest
	result, err := backend.StartMission(context.Background(), start)
	if err != nil || result.Status != "running" || runner.calls != 1 {
		t.Fatalf("continued result = %#v, calls=%d, err=%v", result, runner.calls, err)
	}
}

func TestAuthoritativeMissionExecutionSurfacesCompletedTerminalFlightError(
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
		version: command.ExpectedViewVersion, visible: true,
		execution: projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID, PlanDigest: plan.Digest(),
			Status: "succeeded",
			Nodes: []projection.TeamExecutionNode{{
				LogicalNodeID: "main", Status: "succeeded",
			}},
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
				ViewVersion:       command.ExpectedViewVersion, PlanDigest: plan.Digest(),
				RuntimeInstanceID: "runtime-pi", RuntimeProfileID: "pi-default",
				ModelID: "qwen", AuthMode: "brokered", CapacityAvailable: 1,
				BudgetStatus: "unavailable", PermissionScopes: []string{"workspace"},
				ApprovalPoints: []string{"before_start"},
				Nodes: []MissionExecutionNodePreview{{
					LogicalNodeID: "main", Title: command.Objective,
					Role: "main", MaxAttempts: 1,
				}},
			},
		},
	}
	backend, err := NewAuthoritativeMissionExecutionBackend(
		AuthoritativeMissionExecutionConfig{
			State: state, Compiler: compiler,
			Runner: incompleteMissionExecutionRunner{}, VisibilityTimeout: time.Second,
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
	executionDigest := missionExecutionDigest(
		command.TeamInstanceID, plan.Digest(), preflight.PreflightDigest,
	)
	flight, launch, err := backend.flight(
		command.TeamInstanceID, plan.Digest(), executionDigest,
	)
	if err != nil || !launch {
		t.Fatalf("flight = %#v, launch=%t, err=%v", flight, launch, err)
	}
	flight.errMu.Lock()
	flight.err = errors.Join(ErrTeamWorkspacePublish, teamWorkspacePublishStageError("final_digest"))
	flight.errMu.Unlock()
	close(flight.done)

	start := command
	start.Operation = "start"
	start.PreflightDigest = preflight.PreflightDigest
	if _, err := backend.StartMission(context.Background(), start); !errors.Is(
		err, ErrTeamWorkspacePublish,
	) {
		t.Fatalf("terminal publication error = %v", err)
	} else if stage, ok := TeamWorkspacePublishStage(err); !ok || stage != "final_digest" {
		t.Fatalf("terminal publication stage = %q, %t", stage, ok)
	}
}

func TestMissionExecutionErrorTypesReportsTypesWithoutMessages(t *testing.T) {
	err := errors.Join(
		fmt.Errorf("sensitive detail: %w", context.Canceled),
		teamWorkspacePublishStageError("main_candidate_missing"),
	)
	got := missionExecutionErrorTypes(err)
	if !strings.Contains(got, "*errors.joinError") ||
		!strings.Contains(got, "*fmt.wrapError") ||
		!strings.Contains(got, "app.teamWorkspacePublishStageError") {
		t.Fatalf("error types = %q", got)
	}
	if strings.Contains(got, "sensitive detail") || strings.Contains(got, "main_candidate_missing") {
		t.Fatalf("error messages leaked into type-only diagnostic: %q", got)
	}
}

func TestMissionExecutionNeedsCoordinatorContinuationAfterFailedAttempt(t *testing.T) {
	projected := projection.TeamExecution{
		Status: "awaiting_recovery",
		Nodes: []projection.TeamExecutionNode{{
			LogicalNodeID: "main", Status: "awaiting_recovery",
			Attempts: []projection.TeamExecutionAttempt{{
				AttemptNumber: 1, Status: "failed",
			}},
		}},
	}
	if !missionExecutionNeedsCoordinatorContinuation(projected) {
		t.Fatal("failed durable attempt must wake the coordinator recovery path")
	}
	projected.Nodes[0].Attempts[0].Status = "dispatched"
	if missionExecutionNeedsCoordinatorContinuation(projected) {
		t.Fatal("a still-dispatched attempt must retain its active flight")
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

func TestBuiltInMissionExecutionCompilerPreservesRoleSpecificExecutionBindings(
	t *testing.T,
) {
	command := missionExecutionTestCommand("preflight")
	now := time.Date(2026, 8, 11, 6, 0, 0, 0, time.UTC)
	profile := func(
		id, adapter, provider, account, model, reference string,
		revision int64,
	) loomruntime.RuntimeProfile {
		t.Helper()
		value, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
			ID: id, AdapterType: adapter, ProviderID: provider,
			ProviderAccountID: account, ModelID: model,
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("a", 64),
			CredentialReference: reference, CredentialRevision: revision,
			Timeout: time.Minute,
		})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	instance := func(id, adapter string) loomruntime.RuntimeInstance {
		t.Helper()
		value, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
			ID: id, DeviceID: "device-local", AdapterType: adapter,
			DisplayName: id, ExecutableVersion: "1.0.0",
			Status: loomruntime.RuntimeOnline, Capacity: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	mainProfile := profile(
		"profile-codex", "codex", "openai", "openai.primary",
		"gpt-5.5-codex", "credential-ref-openai", 3,
	)
	mainInstance := instance("runtime-codex", "codex")
	subProfile := profile(
		"profile-deepseek", "loom-native", "deepseek", "deepseek.primary",
		"deepseek-chat", "credential-ref-deepseek", 7,
	)
	subInstance := instance("runtime-loom", "loom-native")
	fallbackProfile := profile(
		"profile-anthropic", "claude-code", "anthropic", "anthropic.backup",
		"claude-sonnet", "credential-ref-anthropic", 5,
	)
	fallbackInstance := instance("runtime-claude", "claude-code")
	source := &controlledMissionExecutionBindingSource{binding: MissionExecutionBinding{
		ViewVersion: command.ExpectedViewVersion, TeamInstanceID: command.TeamInstanceID,
		AgentInstanceID: "agent-main", Profile: mainProfile, Instance: mainInstance,
		CapacityAvailable: 1,
		Roles: []MissionExecutionRoleBinding{
			{
				LogicalNodeID: "main", Title: "Coordinate delivery",
				Role: teams.ExecutionRoleMain, DependsOn: []string{"review"},
				AgentInstanceID: "agent-main", Profile: mainProfile,
				Instance: mainInstance, CapacityAvailable: 1,
				FallbackConfigured: true, FallbackProfile: fallbackProfile,
				FallbackInstance: fallbackInstance, FallbackCapacityAvailable: 1,
				FallbackStatus: "ready", FallbackApprovalRequired: true,
			},
			{
				LogicalNodeID: "review", Title: "Review the implementation",
				Role: teams.ExecutionRoleSubAgent, DependsOn: []string{},
				AgentInstanceID: "agent-review", Profile: subProfile,
				Instance: subInstance, CapacityAvailable: 1,
			},
		},
	}}
	capsuleStore := &controlledMissionContextCapsuleStore{}
	compiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: source, SourcePath: t.TempDir(),
			ContextCapsules: capsuleStore,
			Now:             func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if len(compilation.Plan.Nodes()) != 2 ||
		len(compilation.Preflight.Nodes) != 2 ||
		len(compilation.Request.Semantics) != 2 ||
		len(compilation.Request.Nodes) != 4 {
		t.Fatalf("multi-role compilation = %#v", compilation)
	}
	if len(capsuleStore.records) != 0 {
		t.Fatalf("preflight persisted encrypted capsules = %#v", capsuleStore.records)
	}
	startForPersistence := command
	startForPersistence.Operation = missionExecutionStart
	startForPersistence.PreflightDigest = strings.Repeat("e", 64)
	startedForPersistence, err := compiler.CompileMissionExecution(
		context.Background(), startForPersistence,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(capsuleStore.records) != len(startedForPersistence.Request.Nodes) {
		t.Fatalf("stored start capsules = %#v", capsuleStore.records)
	}
	for index, execution := range startedForPersistence.Request.Nodes {
		stored := capsuleStore.records[index]
		if stored.authority != execution.ContextCapsule.AuthorityRecord() ||
			!bytes.Equal(stored.payload, execution.Dispatch.Payload()) {
			t.Fatalf("stored start capsule %d = %#v", index, stored)
		}
	}
	capsuleStore.err = errors.New("injected encrypted capsule failure")
	if _, err := compiler.CompileMissionExecution(
		context.Background(), startForPersistence,
	); !errors.Is(err, ErrInvalidMissionExecution) {
		t.Fatalf("capsule persistence failure = %v", err)
	}
	capsuleStore.err = nil
	if len(compilation.FallbackDecisions) != 1 {
		t.Fatalf("fallback decision candidates = %#v", compilation.FallbackDecisions)
	}
	decisionCandidate := compilation.FallbackDecisions[0]
	if decisionCandidate.MissionID != command.MissionID ||
		decisionCandidate.Scope.TeamInstanceID() != command.TeamInstanceID ||
		decisionCandidate.Scope.PlanDigest() != compilation.Plan.Digest() ||
		decisionCandidate.Scope.LogicalNodeID() != "main" ||
		decisionCandidate.AgentTitle != command.Objective ||
		decisionCandidate.HarnessAdapter != "claude-code" ||
		decisionCandidate.ProviderAccountID != "anthropic.backup" ||
		decisionCandidate.ModelID != "claude-sonnet" ||
		decisionCandidate.CredentialRevision != 5 {
		t.Fatalf("fallback decision candidate = %#v", decisionCandidate)
	}
	previewByNode := make(map[string]MissionExecutionNodePreview)
	for _, preview := range compilation.Preflight.Nodes {
		previewByNode[preview.LogicalNodeID] = preview
	}
	if got := previewByNode["main"]; got.HarnessAdapter != "codex" ||
		got.ProviderID != "openai" || got.ProviderAccountID != "openai.primary" ||
		got.AuthMode != string(loomruntime.AuthBrokered) ||
		got.ModelID != "gpt-5.5-codex" || got.CredentialRevision != 3 ||
		got.Status != "ready" || got.BlockReason != "" ||
		got.Title != command.Objective {
		t.Fatalf("main preflight preview = %#v", got)
	}
	if got := previewByNode["main"]; !got.FallbackConfigured ||
		got.FallbackHarnessAdapter != "claude-code" ||
		got.FallbackProviderID != "anthropic" ||
		got.FallbackProviderAccountID != "anthropic.backup" ||
		got.FallbackAuthMode != string(loomruntime.AuthBrokered) ||
		got.FallbackModelID != "claude-sonnet" ||
		got.FallbackCredentialRevision != 5 || got.FallbackTimeoutSeconds != 60 ||
		got.FallbackBudgetCredits != nil || got.FallbackStatus != "ready" ||
		got.FallbackBlockReason != "" || !got.FallbackApprovalRequired {
		t.Fatalf("main fallback preflight = %#v", got)
	}
	if encoded, err := json.Marshal(compilation.Preflight); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(encoded), "credential-openai") ||
		strings.Contains(string(encoded), strings.Repeat("a", 64)) {
		t.Fatalf("preflight exposed credential metadata: %s", encoded)
	}
	executionProfiles := map[string]loomruntime.RuntimeProfile{}
	for _, execution := range compilation.Request.Nodes {
		executionProfiles[execution.LogicalNodeID] = execution.Profile
	}
	if got := executionProfiles["main"]; got.ProviderAccountID != "openai.primary" ||
		got.CredentialRevision != 3 || got.ModelID != "gpt-5.5-codex" {
		t.Fatalf("main execution profile = %#v", got)
	}
	if got := executionProfiles["review"]; got.ProviderAccountID != "deepseek.primary" ||
		got.CredentialRevision != 7 || got.ModelID != "deepseek-chat" {
		t.Fatalf("review execution profile = %#v", got)
	}
	var ordinaryRetry TeamNodeExecution
	var primaryAttempt TeamNodeExecution
	for _, execution := range compilation.Request.Nodes {
		if execution.LogicalNodeID == "main" && execution.AttemptNumber == 1 {
			primaryAttempt = execution
		}
		if execution.LogicalNodeID == "main" && execution.AttemptNumber == 2 &&
			(execution.Profile.ID != mainProfile.ID ||
				execution.WorkflowPath != "builtin/mission-primary-v1") {
			t.Fatalf("unapproved fallback materialized = %#v", execution)
		}
		if execution.LogicalNodeID == "main" && execution.AttemptNumber == 2 {
			ordinaryRetry = execution
		}
	}
	ordinaryDispatch, err := contextcapsule.ValidateDispatchPayload(
		ordinaryRetry.ContextCapsule, ordinaryRetry.Dispatch.Payload(),
	)
	if err != nil || ordinaryRetry.ContextCapsule.Digest() != primaryAttempt.ContextCapsule.Digest() ||
		strings.Contains(ordinaryDispatch.Prompt, "fallback-route-approval") {
		t.Fatalf("ordinary retry disclosed fallback authority = %#v, err=%v", ordinaryDispatch, err)
	}
	semanticsByNode := make(map[string]TeamNodeSemantics)
	for _, semantic := range compilation.Request.Semantics {
		semanticsByNode[semantic.LogicalNodeID] = semantic
	}
	projectedNodes := make([]projection.TeamExecutionNode, 0, 2)
	for _, node := range compilation.Plan.Nodes() {
		semantic := semanticsByNode[node.LogicalNodeID()]
		projectedNodes = append(projectedNodes, projection.TeamExecutionNode{
			LogicalNodeID: node.LogicalNodeID(), Title: node.Title(),
			AgentInstanceID:   node.AgentInstanceID(),
			RuntimeInstanceID: node.RuntimeInstanceID(), Role: string(node.Role()),
			DependsOn: node.DependsOn(), MaxAttempts: node.MaxAttempts(),
			Status:                      "pending",
			OutputContractVersion:       semantic.OutputContract.Version(),
			OutputContractDigest:        semantic.OutputContract.Digest(),
			RecoveryPolicyVersion:       semantic.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:        semantic.RecoveryPolicy.Digest(),
			AttemptCredits:              semantic.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         semantic.PrimaryWorkflowPath,
			WorkflowFallbackKey:         semantic.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    semantic.RecoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   semantic.AcceptanceContract.Version(),
			AcceptanceContractDigest:    semantic.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(semantic.AcceptanceContract.Risk()),
			IndependentVerifierRequired: semantic.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     semantic.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   semantic.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        semantic.VerifierWorkflowPath,
		})
	}
	recovered, err := compiler.ReconstructMissionExecution(
		context.Background(),
		projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID,
			PlanDigest:     compilation.Plan.Digest(), Status: "running",
			Nodes: projectedNodes,
		},
	)
	if err != nil || recovered.Plan.Digest() != compilation.Plan.Digest() ||
		len(recovered.Nodes) != 4 || len(recovered.Semantics) != 2 {
		t.Fatalf("multi-role recovery = %#v, err=%v", recovered, err)
	}
	approvalSource := &controlledMissionFallbackApprovalSource{now: now}
	if _, freezeErr := loomruntime.FreezeExecutionBinding(
		source.binding.Roles[0].Profile,
		source.binding.Roles[0].Instance,
	); freezeErr != nil {
		t.Fatalf("primary approval binding: %v", freezeErr)
	}
	if _, freezeErr := loomruntime.FreezeExecutionBinding(
		source.binding.Roles[0].FallbackProfile,
		source.binding.Roles[0].FallbackInstance,
	); freezeErr != nil {
		t.Fatalf("fallback approval binding: %v", freezeErr)
	}
	approvedCompiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: source, FallbackApprovals: approvalSource,
			SourcePath: t.TempDir(), Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := approvedCompiler.CompileMissionExecution(
		context.Background(), command,
	)
	if err != nil || len(approvalSource.queries) != 1 {
		t.Fatalf("approved fallback compilation = %#v, queries=%#v, %v", approved, approvalSource.queries, err)
	}
	var approvedAttempt TeamNodeExecution
	for _, execution := range approved.Request.Nodes {
		if execution.LogicalNodeID == "main" && execution.AttemptNumber == 2 {
			approvedAttempt = execution
		}
	}
	approvedSemantic := TeamNodeSemantics{}
	for _, semantic := range approved.Request.Semantics {
		if semantic.LogicalNodeID == "main" {
			approvedSemantic = semantic
		}
	}
	if approvedAttempt.Profile.ID != fallbackProfile.ID ||
		approvedAttempt.Instance.ID != fallbackInstance.ID ||
		approvedAttempt.WorkflowPath != "builtin/mission-fallback-v1" ||
		approvedSemantic.RecoveryPolicy.WorkflowFallbackKey() !=
			"builtin/mission-fallback-v1" ||
		!approvedSemantic.FallbackApproval.Valid() ||
		approvedSemantic.FallbackApproval.SourceBindingDigest() !=
			approvalSource.queries[0].SourceBindingDigest ||
		approvedSemantic.FallbackApproval.TargetBindingDigest() !=
			approvalSource.queries[0].TargetBindingDigest {
		t.Fatalf("approved fallback attempt/semantics = %#v / %#v", approvedAttempt, approvedSemantic)
	}
	approvedDispatch, err := contextcapsule.ValidateDispatchPayload(
		approvedAttempt.ContextCapsule, approvedAttempt.Dispatch.Payload(),
	)
	if err != nil ||
		!strings.Contains(approvedDispatch.Prompt, `"item_id":"fallback-route-approval"`) ||
		!strings.Contains(approvedDispatch.Prompt, approvedSemantic.FallbackApproval.Digest()) ||
		!strings.Contains(approvedDispatch.Prompt, approvalSource.queries[0].SourceBindingDigest) ||
		!strings.Contains(approvedDispatch.Prompt, approvalSource.queries[0].TargetBindingDigest) ||
		strings.Contains(approvedDispatch.Prompt, approvedSemantic.FallbackApproval.ActorRef()) ||
		strings.Contains(approvedDispatch.Prompt, fallbackProfile.CredentialReference) {
		t.Fatalf("approved fallback context dispatch = %#v, err=%v", approvedDispatch, err)
	}
	for _, preview := range approved.Preflight.Nodes {
		if preview.LogicalNodeID == "main" &&
			(!preview.FallbackApprovalAvailable ||
				preview.FallbackApprovalVersion != 1) {
			t.Fatalf("approved fallback preflight = %#v", preview)
		}
	}
	if !validMissionExecutionFallbackApproval(approved.Preflight.Nodes[0]) {
		t.Fatal("approved fallback preview should satisfy the closed contract")
	}
	contradictory := cloneMissionExecutionPreflight(approved.Preflight)
	contradictory.Nodes[0].FallbackApprovalAvailable = false
	if validMissionExecutionFallbackApproval(contradictory.Nodes[0]) {
		t.Fatal("approval version without an available approval should fail closed")
	}
	contradictory = cloneMissionExecutionPreflight(approved.Preflight)
	contradictory.Nodes[0].FallbackApprovalRequired = false
	if validMissionExecutionFallbackApproval(contradictory.Nodes[0]) {
		t.Fatal("configured fallback without required approval should fail closed")
	}
	withCapabilities := cloneMissionExecutionPreflight(approved.Preflight)
	withCapabilities.Nodes[0].FallbackCapabilities = []string{"fallback-capability"}
	cloned := cloneMissionExecutionPreflight(withCapabilities)
	cloned.Nodes[0].FallbackCapabilities[0] = "mutated-capability"
	if withCapabilities.Nodes[0].FallbackCapabilities[0] == "mutated-capability" {
		t.Fatal("preflight clone aliases fallback capabilities")
	}
	forgedApprovalSource := &controlledMissionFallbackApprovalSource{
		now: now, targetBindingDigest: strings.Repeat("f", 64),
	}
	forgedCompiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: source, FallbackApprovals: forgedApprovalSource,
			SourcePath: t.TempDir(), Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := forgedCompiler.CompileMissionExecution(
		context.Background(), command,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("forged fallback approval error = %v", err)
	}
	source.binding.Roles[0].FallbackStatus = "blocked"
	source.binding.Roles[0].FallbackBlockReason =
		"Credential was revoked for this Provider Account."
	approvedBlocked, err := approvedCompiler.CompileMissionExecution(
		context.Background(), command,
	)
	if err != nil || approvedBlocked.Preflight.Nodes[0].Status != "blocked" ||
		approvedBlocked.Preflight.Nodes[0].BlockReason == "" ||
		approvedBlocked.Preflight.Nodes[1].Status != "ready" {
		t.Fatalf("approved blocked fallback preflight = %#v, err=%v", approvedBlocked.Preflight, err)
	}
	approvedBlockedStart := command
	approvedBlockedStart.Operation = missionExecutionStart
	approvedBlockedStart.PreflightDigest = strings.Repeat("e", 64)
	approvedIsolated, err := approvedCompiler.CompileMissionExecution(
		context.Background(), approvedBlockedStart,
	)
	if err != nil || len(approvedIsolated.Request.InitialBlocks) != 1 ||
		approvedIsolated.Request.InitialBlocks[0].Code != "fallback_unavailable" {
		t.Fatalf("approved blocked fallback start error = %v", err)
	}
	source.binding.Roles[0].FallbackStatus = "ready"
	source.binding.Roles[0].FallbackBlockReason = ""
	source.binding.Roles[1].Status = "blocked"
	source.binding.Roles[1].BlockReason = "Credential is not verified. Reconnect this Provider Account."
	blocked, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil || len(blocked.Preflight.Nodes) != 2 ||
		blocked.Preflight.Nodes[1].Status != "blocked" ||
		blocked.Preflight.Nodes[1].BlockReason == "" ||
		blocked.Preflight.Nodes[0].Status != "ready" {
		t.Fatalf("blocked preflight = %#v, err=%v", blocked.Preflight, err)
	}
	start := command
	start.Operation = missionExecutionStart
	start.PreflightDigest = strings.Repeat("f", 64)
	started, err := compiler.CompileMissionExecution(context.Background(), start)
	if err != nil {
		t.Fatalf("blocked start error = %v", err)
	}
	blocksByNode := make(map[string]teams.InitialExecutionBlock)
	for _, block := range started.Request.InitialBlocks {
		blocksByNode[block.LogicalNodeID] = block
	}
	if len(blocksByNode) != 2 ||
		blocksByNode[source.binding.Roles[1].LogicalNodeID].Code != "agent_unavailable" ||
		blocksByNode["main"].Code != "dependency_blocked" ||
		blocksByNode["main"].SourceLogicalNodeID != source.binding.Roles[1].LogicalNodeID {
		t.Fatalf("initial blocks = %#v", started.Request.InitialBlocks)
	}
}

func TestMissionExecutionGrantLifetimeCoversEveryFrozenRuntime(t *testing.T) {
	roles := []MissionExecutionRoleBinding{
		{
			Profile:            loomruntime.RuntimeProfile{Timeout: 10 * time.Minute},
			FallbackConfigured: true,
			FallbackProfile:    loomruntime.RuntimeProfile{Timeout: 12 * time.Minute},
		},
		{Profile: loomruntime.RuntimeProfile{Timeout: 3 * time.Minute}},
	}

	lifetime, err := missionExecutionGrantLifetime(roles)
	if err != nil {
		t.Fatal(err)
	}
	if lifetime != 13*time.Minute {
		t.Fatalf("grant lifetime = %s, want 13m", lifetime)
	}

	roles[0].FallbackProfile.Timeout = time.Hour
	if _, err := missionExecutionGrantLifetime(roles); !errors.Is(err, ErrInvalidMissionExecution) {
		t.Fatalf("unsafe grant lifetime error = %v, want invalid mission execution", err)
	}
}

func TestBuiltInMissionExecutionCompilerUsesExactCommandWorkspacePath(t *testing.T) {
	command := missionExecutionTestCommand(missionExecutionPreflight)
	configuredSource := t.TempDir()
	requestedSource := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(configuredSource, "configured.txt"),
		[]byte("configured\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(requestedSource, "requested.txt"),
		[]byte("requested\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile-workspace", AdapterType: "pi-cli", ProviderID: "local",
		ModelID: "qwen2.5-coder-1.5b-instruct-q4-k-m", AuthMode: loomruntime.AuthNative,
		RequiredCapabilities: []string{}, Timeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-workspace", DeviceID: "device-local", AdapterType: "pi-cli",
		DisplayName: "Local Pi", ExecutableVersion: "0.82.1",
		Status: loomruntime.RuntimeOnline, ObservedCapabilities: []string{}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := &controlledMissionExecutionBindingSource{binding: MissionExecutionBinding{
		ViewVersion: command.ExpectedViewVersion, TeamInstanceID: command.TeamInstanceID,
		AgentInstanceID: "agent-main", Profile: profile, Instance: instance,
		CapacityAvailable: 1,
	}}
	compiler, err := NewBuiltInMissionExecutionCompiler(BuiltInMissionExecutionCompilerConfig{
		Bindings: source, SourcePath: configuredSource,
		Now: func() time.Time { return time.Date(2026, 8, 25, 4, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}

	command.WorkspacePath = requestedSource
	requested, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if requested.Request.WorkspacePublisher == nil {
		t.Fatal("selected workspace did not receive accepted-change publisher")
	}
	requestedSnapshot, err := supervisor.ObserveSourceSnapshot(requestedSource)
	if err != nil {
		t.Fatal(err)
	}
	for _, execution := range requested.Request.Nodes {
		if execution.SourcePath != requestedSource ||
			execution.SourceSnapshotDigest != requestedSnapshot.TreeDigest() {
			t.Fatalf("requested execution source = %q/%q, want %q/%q",
				execution.SourcePath, execution.SourceSnapshotDigest,
				requestedSource, requestedSnapshot.TreeDigest())
		}
	}
	for _, semantic := range requested.Request.Semantics {
		if semantic.VerifierExecution == nil ||
			semantic.VerifierExecution.SourcePath != requestedSource ||
			semantic.VerifierExecution.SourceSnapshotDigest != requestedSnapshot.TreeDigest() {
			t.Fatalf("requested verifier source = %#v", semantic.VerifierExecution)
		}
	}

	command.WorkspacePath = ""
	compatible, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if compatible.Request.WorkspacePublisher != nil {
		t.Fatal("implicit daemon source unexpectedly received workspace publisher")
	}
	configuredSnapshot, err := supervisor.ObserveSourceSnapshot(configuredSource)
	if err != nil {
		t.Fatal(err)
	}
	for _, execution := range compatible.Request.Nodes {
		if execution.SourcePath != configuredSource ||
			execution.SourceSnapshotDigest != configuredSnapshot.TreeDigest() {
			t.Fatalf("compatible execution source = %q/%q, want %q/%q",
				execution.SourcePath, execution.SourceSnapshotDigest,
				configuredSource, configuredSnapshot.TreeDigest())
		}
	}
}

func TestMissionExecutionRoleBindingsBoundLongObjectiveOnlyInPreviewTitle(t *testing.T) {
	objective := strings.Repeat("界", 200)
	roles := missionExecutionRoleBindings(MissionExecutionBinding{
		Roles: []MissionExecutionRoleBinding{
			{
				LogicalNodeID: "main", Title: "Coordinator",
				Role: teams.ExecutionRoleMain,
			},
			{
				LogicalNodeID: "worker", Title: "Bounded worker",
				Role: teams.ExecutionRoleSubAgent,
			},
		},
	}, objective)
	if len(roles) != 2 {
		t.Fatalf("roles = %#v", roles)
	}
	var mainTitle, workerTitle string
	for _, role := range roles {
		switch role.LogicalNodeID {
		case "main":
			mainTitle = role.Title
		case "worker":
			workerTitle = role.Title
		}
	}
	if mainTitle == objective || len(mainTitle) > maxMissionExecutionNodeTitleBytes ||
		!utf8.ValidString(mainTitle) || !strings.HasPrefix(objective, mainTitle) {
		t.Fatalf("bounded main title bytes=%d valid=%t", len(mainTitle), utf8.ValidString(mainTitle))
	}
	if workerTitle != "Bounded worker" {
		t.Fatalf("worker title = %q", workerTitle)
	}
	if short := missionExecutionObjectiveTitle("Short objective"); short != "Short objective" {
		t.Fatalf("short objective title = %q", short)
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
	verifierProfile := semantics.VerifierExecution.Profile
	if verifierProfile.ID == profile.ID ||
		verifierProfile.AdapterType != profile.AdapterType ||
		verifierProfile.ProviderID != profile.ProviderID ||
		verifierProfile.ModelID != profile.ModelID ||
		verifierProfile.AuthMode != profile.AuthMode ||
		len(verifierProfile.RequiredCapabilities) != 0 ||
		verifierProfile.RemoteToolEnrollmentID != "" ||
		verifierProfile.RemoteToolEnrollmentDigest != "" {
		t.Fatalf("verifier profile retained mutable/tool authority = %#v", verifierProfile)
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
		TeamInstanceID:        command.TeamInstanceID,
		PlanDigest:            started.Plan.Digest(),
		ExecutionGenerationID: "22222222-2222-4222-8222-222222222222",
		Status:                "running",
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
	if recovered.ExecutionGenerationID != projected.ExecutionGenerationID {
		t.Fatalf(
			"recovered execution generation = %q, want %q",
			recovered.ExecutionGenerationID,
			projected.ExecutionGenerationID,
		)
	}
	for _, execution := range recovered.Nodes {
		wantWorkID := appTeamAttemptIdentity(
			"work", recovered.Plan, execution.LogicalNodeID,
			execution.AttemptNumber, projected.ExecutionGenerationID,
		)
		wantRunID := appTeamAttemptIdentity(
			"run", recovered.Plan, execution.LogicalNodeID,
			execution.AttemptNumber, projected.ExecutionGenerationID,
		)
		if execution.Dispatch.WorkItemID() != wantWorkID ||
			execution.Dispatch.RunID() != wantRunID {
			t.Fatalf(
				"recovered dispatch identity = (%q, %q), want (%q, %q)",
				execution.Dispatch.WorkItemID(), execution.Dispatch.RunID(),
				wantWorkID, wantRunID,
			)
		}
	}
	continued, err := compiler.compileMissionExecutionGeneration(
		context.Background(),
		command,
		projected,
	)
	if err != nil || continued.Request.ExecutionGenerationID !=
		projected.ExecutionGenerationID {
		t.Fatalf("continued request = %#v, err=%v", continued.Request, err)
	}
	for _, execution := range continued.Request.Nodes {
		wantWorkID := appTeamAttemptIdentity(
			"work", continued.Plan, execution.LogicalNodeID,
			execution.AttemptNumber, projected.ExecutionGenerationID,
		)
		wantRunID := appTeamAttemptIdentity(
			"run", continued.Plan, execution.LogicalNodeID,
			execution.AttemptNumber, projected.ExecutionGenerationID,
		)
		if execution.Dispatch.WorkItemID() != wantWorkID ||
			execution.Dispatch.RunID() != wantRunID {
			t.Fatalf(
				"continued dispatch identity = (%q, %q), want (%q, %q)",
				execution.Dispatch.WorkItemID(), execution.Dispatch.RunID(),
				wantWorkID, wantRunID,
			)
		}
	}
	projected.Nodes[0].PrimaryWorkflowPath = "unknown/recipe"
	if _, err := compiler.ReconstructMissionExecution(
		context.Background(),
		projected,
	); !errors.Is(err, ErrMissionExecutionConflict) {
		t.Fatalf("unknown recipe recovery error = %v", err)
	}
}

func TestBuiltInMissionExecutionCompilerSaltsNewAttemptDispatchIdentities(
	t *testing.T,
) {
	command := missionExecutionTestCommand(missionExecutionStart)
	command.PreflightDigest = strings.Repeat("e", 64)
	command.NewAttempt = true
	now := time.Date(2026, 8, 24, 9, 30, 0, 0, time.UTC)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "loom-main-native", AdapterType: "pi-cli",
		ProviderID: "local", ModelID: "qwen-local",
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
	compiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: &controlledMissionExecutionBindingSource{
				binding: MissionExecutionBinding{
					ViewVersion:     command.ExpectedViewVersion,
					TeamInstanceID:  command.TeamInstanceID,
					AgentInstanceID: "agent-main", Profile: profile,
					Instance: instance, CapacityAvailable: 2,
				},
			},
			SourcePath: t.TempDir(), Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(
		context.Background(), command,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !compilation.Request.RestartTerminal {
		t.Fatal("new-attempt compilation did not request a terminal restart")
	}
	for _, execution := range compilation.Request.Nodes {
		wantWorkItemID := appTeamAttemptIdentity(
			"work", compilation.Plan, execution.LogicalNodeID,
			execution.AttemptNumber, command.CorrelationID,
		)
		wantRunID := appTeamAttemptIdentity(
			"run", compilation.Plan, execution.LogicalNodeID,
			execution.AttemptNumber, command.CorrelationID,
		)
		if execution.Dispatch.WorkItemID() != wantWorkItemID ||
			execution.Dispatch.RunID() != wantRunID {
			t.Fatalf(
				"attempt %d dispatch identities = (%q, %q), want (%q, %q)",
				execution.AttemptNumber,
				execution.Dispatch.WorkItemID(), execution.Dispatch.RunID(),
				wantWorkItemID, wantRunID,
			)
		}
		if execution.Dispatch.WorkItemID() == appTeamAttemptIdentity(
			"work", compilation.Plan, execution.LogicalNodeID,
			execution.AttemptNumber,
		) || execution.Dispatch.RunID() == appTeamAttemptIdentity(
			"run", compilation.Plan, execution.LogicalNodeID,
			execution.AttemptNumber,
		) {
			t.Fatalf("attempt %d retained unsalted dispatch identities", execution.AttemptNumber)
		}
	}
}

func TestMissionVerifierRuntimeProfileRemovesToolAuthority(t *testing.T) {
	profile, err := missionVerifierRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "opencode-main", AdapterType: "opencode",
		ProviderID: "opencode", ModelID: "opencode/big-pickle",
		AuthMode: loomruntime.AuthNative, ReasoningEffort: "medium",
		RequiredCapabilities: []string{
			loomruntime.CapabilityContextRetrieval,
			loomruntime.CapabilityGovernedToolLoop,
			loomruntime.CapabilityReasoningEffort,
			"workspace_edit",
		},
		Timeout:                    10 * time.Minute,
		RemoteToolEnrollmentID:     "remote-enrollment",
		RemoteToolEnrollmentDigest: strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	wantCapabilities := []string{
		loomruntime.CapabilityReasoningEffort,
		"workspace_edit",
	}
	if profile.ID != "opencode-main:independent-verifier:v1" ||
		!reflect.DeepEqual(profile.RequiredCapabilities, wantCapabilities) ||
		profile.RemoteToolEnrollmentID != "" ||
		profile.RemoteToolEnrollmentDigest != "" {
		t.Fatalf("verifier profile = %#v", profile)
	}
}

func TestP3AMissionCompilerMergesExactBindingsAndRequiresMaterializationCapability(t *testing.T) {
	command := missionExecutionTestCommand("preflight")
	now := time.Date(2026, 8, 3, 7, 0, 0, 0, time.UTC)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "loom-main-native", AdapterType: "pi-cli", ProviderID: "local",
		ModelID:  "qwen2.5-coder-1.5b-instruct-q4-k-m",
		AuthMode: loomruntime.AuthNative, RequiredCapabilities: []string{"models"},
		Timeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime-pi", DeviceID: "device-local", AdapterType: "pi-cli",
		DisplayName: "Local Pi", ExecutableVersion: "0.82.1",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"models", missionSkillMaterializationCapability},
		Capacity:             1,
	})
	if err != nil {
		t.Fatal(err)
	}
	bindingAt := func(revision, digit string) assets.ExactAssetRevisionBinding {
		return assets.ExactAssetRevisionBinding{
			AssetKind: assets.AssetKindSkill, DefinitionID: "skill-1",
			RevisionID: revision, SHA256Digest: strings.Repeat(digit, 64),
			SourceScope: assets.SourceScopeLocal,
		}
	}
	workPackageBinding := bindingAt("work-package", "d")
	source := &controlledMissionExecutionBindingSource{binding: MissionExecutionBinding{
		ViewVersion: command.ExpectedViewVersion, TeamInstanceID: command.TeamInstanceID,
		AgentInstanceID: "agent-main", Profile: profile, Instance: instance,
		CapacityAvailable:            1,
		SavedTeamAssetBindings:       []assets.ExactAssetRevisionBinding{bindingAt("saved", "a")},
		TeamDefinitionAssetBindings:  []assets.ExactAssetRevisionBinding{bindingAt("team", "b")},
		AgentDefinitionAssetBindings: []assets.ExactAssetRevisionBinding{bindingAt("agent", "c")},
		AssetBindingRecords: []assets.EvolutionAssetBindingRecord{{
			SubjectKind: "work_package", SubjectID: command.WorkPackageID,
			SubjectVersion: 1, SubjectDigest: command.WorkPackageDigest,
			SubjectScope: "builtin", SubjectIdentityDigest: strings.Repeat("e", 64),
			Bindings: []assets.ExactAssetRevisionBinding{workPackageBinding},
		}},
		AssetSourceStreamIDs: []string{"team-definition/team-1"},
	}}
	compiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: source, SourcePath: t.TempDir(),
			Now: func() time.Time { return now },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	node := compilation.Plan.Nodes()[0]
	if got := node.AssetRevisionBindings(); len(got) != 1 || got[0] != workPackageBinding {
		t.Fatalf("merged plan bindings = %#v", got)
	}
	if node.AssetRevisionSetDigest() == "" ||
		!reflect.DeepEqual(compilation.Request.AssetSourceStreamIDs, []string{
			"evolution-asset-binding/work_package/" + strings.Repeat("e", 64),
			"team-definition/team-1",
		}) {
		t.Fatalf("lineage request = %#v", compilation.Request)
	}
	if got := compilation.Request.Nodes[0].Profile.RequiredCapabilities; !reflect.DeepEqual(got, []string{missionSkillMaterializationCapability, "models"}) {
		t.Fatalf("required capabilities = %#v", got)
	}
	dispatch, err := contextcapsule.ValidateDispatchPayload(
		compilation.Request.Nodes[0].ContextCapsule,
		compilation.Request.Nodes[0].Dispatch.Payload(),
	)
	var rolePrompt struct {
		Items []struct {
			ItemID  string `json:"item_id"`
			Content string `json:"content"`
		} `json:"items"`
	}
	if err == nil {
		err = json.Unmarshal([]byte(dispatch.Prompt), &rolePrompt)
	}
	var disclosedBinding assets.ExactAssetRevisionBinding
	for _, item := range rolePrompt.Items {
		if item.ItemID == "artifact-000" {
			err = json.Unmarshal([]byte(item.Content), &disclosedBinding)
		}
	}
	if err != nil || disclosedBinding != workPackageBinding ||
		len(compilation.Request.Nodes[0].ContextCapsule.Target().ArtifactRefs) != 1 {
		t.Fatalf("exact asset context dispatch = %#v, err=%v", dispatch, err)
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

func TestProjectionMissionExecutionBindingSourceResolvesEverySavedTeamRole(
	t *testing.T,
) {
	database := openTeamCanaryDB(t)
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	readModel := projection.New(database)
	rebuild := func(stage string) {
		t.Helper()
		if err := readModel.Rebuild(context.Background()); err != nil {
			t.Fatalf("%s projection replay: %v", stage, err)
		}
	}
	now := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)
	digest := func(seed int) string { return fmt.Sprintf("%064x", seed) }
	definitions := []agents.AgentDefinition{
		{
			ID: "agent-main", Version: 1, Scope: agents.ScopeReusable,
			Name: "Coordinator", RoleSpec: "Coordinate delivery",
			Status: agents.DefinitionActive,
		},
		{
			ID: "agent-claude", Version: 1, Scope: agents.ScopeReusable,
			Name: "Claude reviewer", RoleSpec: "Review implementation",
			Status: agents.DefinitionActive,
		},
		{
			ID: "agent-kimi", Version: 1, Scope: agents.ScopeReusable,
			Name: "Kimi researcher", RoleSpec: "Research dependencies",
			Status: agents.DefinitionActive,
		},
		{
			ID: "agent-minimax", Version: 1, Scope: agents.ScopeReusable,
			Name: "MiniMax verifier", RoleSpec: "Verify the result",
			Status: agents.DefinitionActive,
		},
	}
	profiles := []loomruntime.RuntimeProfile{
		{
			ID: "profile-codex", AdapterType: "codex", ProviderID: "openai",
			ProviderAccountID: "openai.primary", ModelID: "gpt-5.5-codex",
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("a", 64),
			CredentialReference: "credential-ref-openai-primary",
			CredentialRevision:  2, RequiredCapabilities: []string{"chat"},
			Timeout: time.Minute,
		},
		{
			ID: "profile-claude", AdapterType: "claude-code", ProviderID: "anthropic",
			ProviderAccountID: "anthropic.primary", ModelID: "claude-sonnet-4",
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("b", 64),
			CredentialReference: "credential-ref-anthropic-primary",
			CredentialRevision:  2, RequiredCapabilities: []string{"chat"},
			Timeout: time.Minute,
		},
		{
			ID: "profile-kimi", AdapterType: "loom-native", ProviderID: "kimi",
			ProviderAccountID: "kimi.primary", ModelID: "kimi-k2.5",
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("c", 64),
			CredentialReference: "credential-ref-kimi-primary",
			CredentialRevision:  2, RequiredCapabilities: []string{"chat"},
			Timeout: 2 * time.Minute,
		},
		{
			ID: "profile-minimax", AdapterType: "loom-native", ProviderID: "minimax",
			ProviderAccountID: "minimax.primary", ModelID: "MiniMax-M2.1",
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("d", 64),
			CredentialReference: "credential-ref-minimax-primary",
			CredentialRevision:  2, RequiredCapabilities: []string{"chat"},
			Timeout: 3 * time.Minute,
		},
	}
	fallbackProfile := loomruntime.RuntimeProfile{
		ID: "profile-deepseek", AdapterType: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.backup", ModelID: "deepseek-chat",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: strings.Repeat("e", 64),
		CredentialReference: "credential-ref-deepseek-backup",
		CredentialRevision:  2, RequiredCapabilities: []string{"chat"},
		Timeout: time.Minute,
	}
	runtimeProfiles := append(append([]loomruntime.RuntimeProfile{}, profiles...), fallbackProfile)
	definition, err := teams.BuildTeamDefinition(teams.TeamDefinitionInput{
		ID: "team.mixed", Version: 1, Scope: teams.TeamDefinitionScopeReusable,
		Name: "Mixed Team", Status: teams.TeamDefinitionActive,
		Roles: []teams.TeamDefinitionRole{
			{
				Kind: teams.TeamDefinitionRoleMain, AgentDefinitionID: "agent-main",
				RuntimeProfileID: "profile-codex", Responsibility: "Coordinate delivery",
			},
			{
				Kind: teams.TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent-claude",
				RuntimeProfileID: "profile-claude", Responsibility: "Review implementation",
			},
			{
				Kind: teams.TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent-kimi",
				RuntimeProfileID: "profile-kimi", Responsibility: "Research dependencies",
			},
			{
				Kind: teams.TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent-minimax",
				RuntimeProfileID: "profile-minimax", Responsibility: "Verify the result",
			},
		},
	}, definitions, runtimeProfiles)
	if err != nil {
		t.Fatal(err)
	}
	configuration := state.TeamConfigurationSnapshot{
		RequestedConcurrency: 3, MaximumBudgetCredits: 40,
		RoleBindings: make([]state.TeamConfigurationRoleBinding, 0, 4),
	}
	runtimeIDs := []string{
		"runtime-codex", "runtime-claude", "runtime-kimi", "runtime-minimax",
	}
	for index, profile := range profiles {
		kind := "main"
		agentID := "agent-main"
		if index > 0 {
			kind = "subagent"
			agentID = definitions[index].ID
		}
		configuration.RoleBindings = append(configuration.RoleBindings,
			state.TeamConfigurationRoleBinding{
				Kind: kind, AgentDefinitionID: agentID,
				RuntimeProfileID: profile.ID, RuntimeInstanceID: runtimeIDs[index],
				ModelID: profile.ModelID,
				ExecutionProfile: &state.TeamConfigurationExecutionProfile{
					Version: 1, ID: profile.ID, HarnessAdapter: profile.AdapterType,
					ProviderID:        profile.ProviderID,
					ProviderAccountID: profile.ProviderAccountID,
					ModelID:           profile.ModelID, AuthMode: profile.AuthMode,
					EndpointFingerprint: profile.EndpointFingerprint,
					CredentialReference: profile.CredentialReference,
					CredentialRevision:  profile.CredentialRevision,
					TimeoutNanoseconds:  int64(profile.Timeout),
					RequiredCapabilities: append(
						[]string{}, profile.RequiredCapabilities...,
					),
				},
				SkillRevisions: []state.TeamConfigurationSkillRevision{},
				PermissionIDs:  []string{}, ResourceIDs: []string{},
			},
		)
	}
	configuration.RoleBindings[0].FallbackRoute = &state.TeamConfigurationFallbackRoute{
		Version: 1, RuntimeProfileID: fallbackProfile.ID,
		RuntimeInstanceID: "runtime-deepseek", ModelID: fallbackProfile.ModelID,
		ExecutionProfile: &state.TeamConfigurationExecutionProfile{
			Version: 1, ID: fallbackProfile.ID,
			HarnessAdapter:    fallbackProfile.AdapterType,
			ProviderID:        fallbackProfile.ProviderID,
			ProviderAccountID: fallbackProfile.ProviderAccountID,
			ModelID:           fallbackProfile.ModelID, AuthMode: fallbackProfile.AuthMode,
			EndpointFingerprint: fallbackProfile.EndpointFingerprint,
			CredentialReference: fallbackProfile.CredentialReference,
			CredentialRevision:  fallbackProfile.CredentialRevision,
			TimeoutNanoseconds:  int64(fallbackProfile.Timeout),
			RequiredCapabilities: append(
				[]string{}, fallbackProfile.RequiredCapabilities...,
			),
		},
		ApprovalRequired: true,
	}
	configuration.RoleBindings[2].ParallelRouteSet = &state.TeamConfigurationParallelRouteSet{
		Version: 1,
		AdditionalRoutes: []state.TeamConfigurationExecutionRoute{{
			Version: 1, RuntimeProfileID: fallbackProfile.ID,
			RuntimeInstanceID: "runtime-deepseek", ModelID: fallbackProfile.ModelID,
			ExecutionProfile: &state.TeamConfigurationExecutionProfile{
				Version: 1, ID: fallbackProfile.ID,
				HarnessAdapter:    fallbackProfile.AdapterType,
				ProviderID:        fallbackProfile.ProviderID,
				ProviderAccountID: fallbackProfile.ProviderAccountID,
				ModelID:           fallbackProfile.ModelID, AuthMode: fallbackProfile.AuthMode,
				EndpointFingerprint: fallbackProfile.EndpointFingerprint,
				CredentialReference: fallbackProfile.CredentialReference,
				CredentialRevision:  fallbackProfile.CredentialRevision,
				TimeoutNanoseconds:  int64(fallbackProfile.Timeout),
				RequiredCapabilities: append(
					[]string{}, fallbackProfile.RequiredCapabilities...,
				),
			},
		}},
		SynthesisRoute: state.TeamConfigurationExecutionRoute{
			Version: 1, RuntimeProfileID: profiles[2].ID,
			RuntimeInstanceID: "runtime-kimi", ModelID: profiles[2].ModelID,
			ExecutionProfile: configuration.RoleBindings[2].ExecutionProfile,
		},
	}
	if _, err := writer.SaveTeamDefinition(context.Background(), state.TeamDefinitionSaveCommand{
		CommandID: "save-team-mixed", ExpectedHead: 0, OccurredAt: now,
		Definition: definition, Definitions: definitions, RuntimeProfiles: runtimeProfiles,
		DraftID: "draft-mixed", DraftRevision: 1,
		CatalogDigest: digest(10), ContentDigest: digest(11), BindingDigest: digest(12),
		Configuration: configuration,
	}); err != nil {
		t.Fatal(err)
	}
	rebuild("definition")
	for index, profile := range runtimeProfiles {
		for revision, status := range []credentials.CredentialStatus{
			credentials.CredentialConfigured, credentials.CredentialVerified,
		} {
			if _, err := writer.CommitCredentialMetadata(
				context.Background(), credentials.MetadataCommand{
					CommandID:           fmt.Sprintf("credential-%d-%d", index, revision),
					ProviderID:          profile.ProviderID,
					ProviderAccountID:   profile.ProviderAccountID,
					CredentialReference: profile.CredentialReference,
					ExpectedRevision:    int64(revision),
					OccurredAt:          now.Add(time.Duration(index*2+revision+1) * time.Second),
					Status:              status,
				},
			); err != nil {
				t.Fatal(err)
			}
		}
	}
	rebuild("credentials")
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
	for index, runtime := range []struct{ id, adapter string }{
		{"runtime-codex", "codex"},
		{"runtime-claude", "claude-code"},
		{"runtime-kimi", "loom-native"},
		{"runtime-minimax", "loom-native"},
		{"runtime-deepseek", "loom-native"},
	} {
		appendEvent(journal.Event{
			ID: fmt.Sprintf("runtime-%d", index), StreamID: "runtime_instance:" + runtime.id,
			Seq: 1, IdempotencyKey: fmt.Sprintf("runtime-idem-%d", index),
			Type: "RuntimeInstanceDiscovered", SchemaVersion: 1,
			EmittedAt:     now.Add(time.Duration(10+index) * time.Second),
			CorrelationID: "11111111-1111-4111-8111-111111111111",
			PayloadJSON: encode(map[string]any{
				"discovery_digest": digest(20 + index), "source_probe_id": "probe-" + runtime.id,
				"instance": map[string]any{
					"id": runtime.id, "device_id": "device-local",
					"adapter_type": runtime.adapter, "display_name": runtime.id,
					"executable_version": "1.0.0", "status": "online",
					"observed_capabilities": []string{"chat"}, "capacity": 1,
				},
				"model_ids": []string{runtimeProfiles[index].ProviderID + "/" + runtimeProfiles[index].ModelID},
			}),
		})
		rebuild("runtime " + runtime.id)
	}
	appendEvent(journal.Event{
		ID: "team-mixed-created", StreamID: "team_instance:team-mixed-instance",
		Seq: 1, IdempotencyKey: "team-mixed-created-idem",
		Type: "TeamInstanceCreated", SchemaVersion: 1,
		EmittedAt:     now.Add(20 * time.Second),
		CorrelationID: "11111111-1111-4111-8111-111111111111",
		PayloadJSON: encode(map[string]any{
			"team": map[string]any{
				"id":              "team-mixed-instance",
				"work_request_id": "11111111-1111-4111-8111-111111111111",
				"source_kind":     "saved_team", "team_definition_id": definition.ID(),
				"team_definition_version": definition.Version(),
				"team_definition_scope":   string(definition.Scope()),
				"scope_identity":          map[string]any{"project_id": "", "generation_id": ""},
				"team_definition_digest":  definition.Digest(), "source_plan_digest": digest(30),
				"state": "created", "created_at": int64(1_755_000_000),
			},
			"dormant_sub_agents": []any{
				map[string]any{
					"dormant": true, "agent_definition_id": "agent-claude",
					"runtime_profile_id": "profile-claude", "runtime_instance_id": "runtime-claude",
				},
				map[string]any{
					"dormant": true, "agent_definition_id": "agent-kimi",
					"runtime_profile_id": "profile-kimi", "runtime_instance_id": "runtime-kimi",
				},
				map[string]any{
					"dormant": true, "agent_definition_id": "agent-minimax",
					"runtime_profile_id": "profile-minimax", "runtime_instance_id": "runtime-minimax",
				},
			},
			"source_plan_digest": digest(30), "source_record_set_digest": digest(31),
			"team_instance_count": 1, "agent_instance_count": 1,
			"active_sub_agent_count": 0, "work_item_count": 0,
		}),
	})
	appendEvent(journal.Event{
		ID: "agent-mixed-created", StreamID: "agent_instance:agent-main-instance",
		Seq: 1, IdempotencyKey: "agent-mixed-created-idem",
		Type: "AgentInstanceCreated", SchemaVersion: 1,
		EmittedAt:     now.Add(21 * time.Second),
		CorrelationID: "11111111-1111-4111-8111-111111111111",
		CausationID:   "team-mixed-created",
		PayloadJSON: encode(map[string]any{
			"main_agent": map[string]any{
				"id": "agent-main-instance", "team_instance_id": "team-mixed-instance",
				"agent_definition_id": "agent-main", "agent_definition_version": 1,
				"agent_definition_scope": "reusable",
				"scope_identity":         map[string]any{"project_id": "", "generation_id": ""},
				"runtime_profile_id":     "profile-codex", "runtime_instance_id": "runtime-codex",
				"is_main": true, "state": "created",
			},
			"runtime_binding": map[string]any{
				"accepted": true, "profile_id": "profile-codex", "instance_id": "runtime-codex",
			},
			"source_plan_digest": digest(30), "source_record_set_digest": digest(31),
			"team_created_at": int64(1_755_000_000), "binding_digest": digest(32),
			"runtime_discovery_digest": digest(20),
		}),
	})
	rebuild("agent")
	source, err := NewProjectionMissionExecutionBindingSource(readModel)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := source.ResolveMissionExecutionBinding(
		context.Background(), "team-mixed-instance",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(binding.Roles) != 6 {
		t.Fatalf("roles = %#v", binding.Roles)
	}
	roleByProvider := make(map[string]MissionExecutionRoleBinding)
	seenAgentInstances := make(map[string]struct{})
	subNodeIDs := make([]string, 0, 3)
	routeSiblingCount := 0
	for _, role := range binding.Roles {
		if role.Kind == teams.ExecutionNodeRouteSibling {
			routeSiblingCount++
			continue
		}
		if _, duplicate := roleByProvider[role.Profile.ProviderID]; duplicate {
			t.Fatalf("duplicate Provider role = %#v", binding.Roles)
		}
		if _, duplicate := seenAgentInstances[role.AgentInstanceID]; duplicate {
			t.Fatalf("duplicate Agent instance = %#v", binding.Roles)
		}
		roleByProvider[role.Profile.ProviderID] = role
		seenAgentInstances[role.AgentInstanceID] = struct{}{}
		if role.Role == teams.ExecutionRoleSubAgent {
			subNodeIDs = append(subNodeIDs, role.LogicalNodeID)
		}
	}
	if routeSiblingCount != 2 ||
		roleByProvider["kimi"].Kind != teams.ExecutionNodeAggregation ||
		!roleByProvider["kimi"].Aggregation ||
		len(roleByProvider["kimi"].DependsOn) != 2 {
		t.Fatalf("Kimi parallel route binding = %#v", binding.Roles)
	}
	slices.Sort(subNodeIDs)
	main := roleByProvider["openai"]
	if main.Profile.ProviderAccountID != "openai.primary" ||
		main.Profile.CredentialReference != "credential-ref-openai-primary" ||
		main.Profile.CredentialRevision != 2 ||
		!slices.Equal(main.DependsOn, subNodeIDs) ||
		!main.FallbackConfigured ||
		main.FallbackProfile.ProviderAccountID != "deepseek.backup" ||
		main.FallbackProfile.CredentialReference != "credential-ref-deepseek-backup" ||
		main.FallbackProfile.CredentialRevision != 2 ||
		main.FallbackInstance.ID != "runtime-deepseek" ||
		main.FallbackStatus != "ready" || main.FallbackBlockReason != "" ||
		!main.FallbackApprovalRequired {
		t.Fatalf("binding = %#v", binding)
	}
	wantBindings := map[string]struct {
		adapter, account, reference, model string
		timeout                            time.Duration
	}{
		"openai":    {"codex", "openai.primary", "credential-ref-openai-primary", "gpt-5.5-codex", time.Minute},
		"anthropic": {"claude-code", "anthropic.primary", "credential-ref-anthropic-primary", "claude-sonnet-4", time.Minute},
		"kimi":      {"loom-native", "kimi.primary", "credential-ref-kimi-primary", "kimi-k2.5", 2 * time.Minute},
		"minimax":   {"loom-native", "minimax.primary", "credential-ref-minimax-primary", "MiniMax-M2.1", 3 * time.Minute},
	}
	for providerID, want := range wantBindings {
		role, found := roleByProvider[providerID]
		if !found || role.Profile.AdapterType != want.adapter ||
			role.Profile.ProviderAccountID != want.account ||
			role.Profile.CredentialReference != want.reference ||
			role.Profile.CredentialRevision != 2 ||
			role.Profile.ModelID != want.model || role.Profile.Timeout != want.timeout ||
			!slices.Equal(role.Profile.RequiredCapabilities, []string{"chat"}) ||
			role.Status != "ready" || role.BlockReason != "" ||
			role.AgentInstanceID == "" {
			t.Fatalf("%s role binding = %#v", providerID, role)
		}
	}
	command := missionExecutionTestCommand("preflight")
	command.MissionID = "mission/team-mixed-instance"
	command.TeamInstanceID = "team-mixed-instance"
	command.ExpectedViewVersion = binding.ViewVersion
	compiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: source, SourcePath: t.TempDir(),
			Now: func() time.Time { return now.Add(time.Minute) },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	compilation, err := compiler.CompileMissionExecution(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if len(compilation.Plan.Nodes()) != 6 || len(compilation.Preflight.Nodes) != 6 ||
		len(compilation.Request.Semantics) != 6 || len(compilation.Request.Nodes) != 12 {
		t.Fatalf("four-role compilation = %#v", compilation)
	}
	compiledSiblingCount := 0
	compiledAggregationCount := 0
	for _, node := range compilation.Plan.Nodes() {
		switch node.Kind() {
		case teams.ExecutionNodeRouteSibling:
			compiledSiblingCount++
		case teams.ExecutionNodeAggregation:
			compiledAggregationCount++
			if len(node.DependsOn()) != 2 || node.RouteGroupID() == "" {
				t.Fatalf("aggregation node = %#v", node)
			}
		}
	}
	if compiledSiblingCount != 2 || compiledAggregationCount != 1 {
		t.Fatalf("parallel plan topology = %#v", compilation.Plan.Nodes())
	}
	for _, execution := range compilation.Request.Nodes {
		planned, _ := missionContextPlanNode(compilation.Plan, execution.LogicalNodeID)
		if (planned.Kind() == teams.ExecutionNodeAggregation) !=
			(execution.Aggregation != nil) {
			t.Fatalf("aggregation execution marker = %#v", execution)
		}
	}
	semanticsByLogicalNode := make(map[string]TeamNodeSemantics)
	for _, semantic := range compilation.Request.Semantics {
		semanticsByLogicalNode[semantic.LogicalNodeID] = semantic
	}
	projectedRouteNodes := make([]projection.TeamExecutionNode, 0, 6)
	for _, node := range compilation.Plan.Nodes() {
		semantic := semanticsByLogicalNode[node.LogicalNodeID()]
		projectedRouteNodes = append(projectedRouteNodes, projection.TeamExecutionNode{
			LogicalNodeID: node.LogicalNodeID(), Title: node.Title(),
			AgentInstanceID: node.AgentInstanceID(), RuntimeInstanceID: node.RuntimeInstanceID(),
			Role: string(node.Role()), Kind: string(node.Kind()), RouteGroupID: node.RouteGroupID(),
			DependsOn: node.DependsOn(), MaxAttempts: node.MaxAttempts(), Status: "pending",
			OutputContractVersion:       semantic.OutputContract.Version(),
			OutputContractDigest:        semantic.OutputContract.Digest(),
			RecoveryPolicyVersion:       semantic.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:        semantic.RecoveryPolicy.Digest(),
			AttemptCredits:              semantic.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         semantic.PrimaryWorkflowPath,
			WorkflowFallbackKey:         semantic.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    semantic.RecoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   semantic.AcceptanceContract.Version(),
			AcceptanceContractDigest:    semantic.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(semantic.AcceptanceContract.Risk()),
			IndependentVerifierRequired: semantic.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     semantic.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   semantic.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        semantic.VerifierWorkflowPath,
		})
	}
	recoveredRoutes, err := compiler.ReconstructMissionExecution(
		context.Background(),
		projection.TeamExecution{
			TeamInstanceID: command.TeamInstanceID, PlanDigest: compilation.Plan.Digest(),
			Status: "running", Nodes: projectedRouteNodes,
		},
	)
	if err != nil || recoveredRoutes.Plan.Digest() != compilation.Plan.Digest() ||
		len(recoveredRoutes.Nodes) != 12 ||
		len(recoveredRoutes.Semantics) != 6 {
		t.Fatalf("parallel route recovery = %#v, err=%v", recoveredRoutes, err)
	}
	preflightByProvider := make(map[string]MissionExecutionNodePreview)
	for _, preview := range compilation.Preflight.Nodes {
		if preview.Kind == string(teams.ExecutionNodeRouteSibling) {
			continue
		}
		preflightByProvider[preview.ProviderID] = preview
	}
	for providerID, want := range wantBindings {
		preview, found := preflightByProvider[providerID]
		if !found || preview.HarnessAdapter != want.adapter ||
			preview.ProviderAccountID != want.account || preview.ModelID != want.model ||
			preview.CredentialRevision != 2 ||
			preview.TimeoutSeconds != int64(want.timeout/time.Second) ||
			!slices.Equal(preview.Capabilities, []string{"chat"}) ||
			preview.Status != "ready" || preview.BlockReason != "" {
			t.Fatalf("%s preflight = %#v", providerID, preview)
		}
	}
	primaryByProvider := make(map[string]TeamNodeExecution)
	for _, execution := range compilation.Request.Nodes {
		planned, _ := missionContextPlanNode(compilation.Plan, execution.LogicalNodeID)
		if execution.AttemptNumber == 1 &&
			planned.Kind() != teams.ExecutionNodeRouteSibling {
			primaryByProvider[execution.Profile.ProviderID] = execution
		}
	}
	for providerID, want := range wantBindings {
		execution, found := primaryByProvider[providerID]
		if !found || execution.Profile.AdapterType != want.adapter ||
			execution.Profile.ProviderAccountID != want.account ||
			execution.Profile.CredentialReference != want.reference ||
			execution.Profile.CredentialRevision != 2 ||
			execution.Profile.ModelID != want.model {
			t.Fatalf("%s primary execution = %#v", providerID, execution)
		}
		dispatch, dispatchErr := contextcapsule.ValidateDispatchPayload(
			execution.ContextCapsule,
			execution.Dispatch.Payload(),
		)
		role := roleByProvider[providerID]
		expectedTitle := role.Title
		if role.Role == teams.ExecutionRoleMain {
			expectedTitle = command.Objective
		}
		var rolePrompt struct {
			Items []struct {
				ItemID  string `json:"item_id"`
				Content string `json:"content"`
			} `json:"items"`
		}
		if dispatchErr == nil {
			dispatchErr = json.Unmarshal([]byte(dispatch.Prompt), &rolePrompt)
		}
		itemContent := make(map[string]string, len(rolePrompt.Items))
		for _, item := range rolePrompt.Items {
			itemContent[item.ItemID] = item.Content
		}
		var policy struct {
			DefaultVerifierKey string `json:"default_verifier_key"`
		}
		var task struct {
			LogicalNodeID string `json:"logical_node_id"`
		}
		var governance struct {
			Title string `json:"title"`
		}
		if dispatchErr == nil {
			dispatchErr = json.Unmarshal([]byte(itemContent["work-package-policy"]), &policy)
		}
		if dispatchErr == nil {
			dispatchErr = json.Unmarshal([]byte(itemContent["current-task-state"]), &task)
		}
		if dispatchErr == nil {
			dispatchErr = json.Unmarshal([]byte(itemContent["role-governance"]), &governance)
		}
		if dispatchErr != nil || dispatch.CapsuleDigest != execution.ContextCapsule.Digest() ||
			dispatch.DisclosureReceiptDigest !=
				execution.ContextCapsule.DisclosureReceiptDigest() ||
			dispatch.Prompt == command.Objective ||
			strings.Contains(dispatch.Prompt, want.account) ||
			strings.Contains(dispatch.Prompt, want.reference) ||
			!strings.Contains(dispatch.Prompt, command.Objective) ||
			policy.DefaultVerifierKey != "verifier.code-review.v1" ||
			task.LogicalNodeID != role.LogicalNodeID || governance.Title != expectedTitle {
			t.Fatalf("%s context dispatch = %#v, %v", providerID, dispatch, dispatchErr)
		}
		for peerProviderID, peerRole := range roleByProvider {
			if peerProviderID != providerID && peerRole.Title != command.Objective &&
				strings.Contains(dispatch.Prompt, peerRole.Title) {
				t.Fatalf(
					"%s context dispatch disclosed %s role title: %s",
					providerID, peerProviderID, dispatch.Prompt,
				)
			}
		}
	}
	approvalSource := &controlledMissionFallbackApprovalSource{now: now}
	approvedCompiler, err := NewBuiltInMissionExecutionCompiler(
		BuiltInMissionExecutionCompilerConfig{
			Bindings: source, FallbackApprovals: approvalSource,
			SourcePath: t.TempDir(), Now: func() time.Time { return now.Add(time.Minute) },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	approvedCompilation, err := approvedCompiler.CompileMissionExecution(
		context.Background(), command,
	)
	if err != nil {
		t.Fatalf("approved mixed-Team DeepSeek fallback compilation: %v", err)
	}
	var approvedFallback TeamNodeExecution
	for _, execution := range approvedCompilation.Request.Nodes {
		if execution.LogicalNodeID == "main" && execution.AttemptNumber == 2 {
			approvedFallback = execution
		}
	}
	approvedDispatch, err := contextcapsule.ValidateDispatchPayload(
		approvedFallback.ContextCapsule, approvedFallback.Dispatch.Payload(),
	)
	if err != nil || approvedFallback.Profile.ProviderID != "deepseek" ||
		!strings.Contains(approvedDispatch.Prompt, "fallback-route-approval") {
		t.Fatalf("approved mixed-Team DeepSeek fallback = %#v, dispatch=%#v, err=%v", approvedFallback, approvedDispatch, err)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(), credentials.MetadataCommand{
			CommandID:  "credential-kimi-revoke",
			ProviderID: "kimi", ProviderAccountID: "kimi.primary",
			CredentialReference: "credential-ref-kimi-primary",
			ExpectedRevision:    2, OccurredAt: now.Add(30 * time.Second),
			Status: credentials.CredentialRevoked,
		},
	); err != nil {
		t.Fatal(err)
	}
	rebuild("revoked credential")
	blockedBinding, err := source.ResolveMissionExecutionBinding(
		context.Background(), "team-mixed-instance",
	)
	if err != nil {
		t.Fatal(err)
	}
	statusByProvider := make(map[string]MissionExecutionRoleBinding)
	blockedRouteStatuses := make(map[string]string)
	for _, role := range blockedBinding.Roles {
		statusByProvider[role.Profile.ProviderID] = role
		if role.Kind == teams.ExecutionNodeRouteSibling {
			blockedRouteStatuses[role.Profile.ProviderID] = role.Status
		}
	}
	for _, providerID := range []string{"openai", "anthropic", "minimax"} {
		if statusByProvider[providerID].Status != "ready" ||
			statusByProvider[providerID].BlockReason != "" {
			t.Fatalf("%s peer credential health = %#v", providerID, statusByProvider)
		}
	}
	if statusByProvider["kimi"].Status != "blocked" ||
		!strings.Contains(statusByProvider["kimi"].BlockReason, "Credential") {
		t.Fatalf("isolated Kimi credential health = %#v", statusByProvider)
	}
	if blockedRouteStatuses["kimi"] != "blocked" ||
		blockedRouteStatuses["deepseek"] != "ready" {
		t.Fatalf("parallel route failure isolation = %#v", blockedRouteStatuses)
	}
	blockedCommand := command
	blockedCommand.ExpectedViewVersion = blockedBinding.ViewVersion
	blockedCompilation, err := compiler.CompileMissionExecution(
		context.Background(), blockedCommand,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(blockedCompilation.Preflight.Nodes) != 6 {
		t.Fatalf("blocked preflight = %#v", blockedCompilation.Preflight)
	}
	blockedPreflightByProvider := make(map[string]MissionExecutionNodePreview)
	for _, preview := range blockedCompilation.Preflight.Nodes {
		if preview.Kind == string(teams.ExecutionNodeRouteSibling) {
			continue
		}
		blockedPreflightByProvider[preview.ProviderID] = preview
	}
	for _, providerID := range []string{"openai", "anthropic", "minimax"} {
		if blockedPreflightByProvider[providerID].Status != "ready" {
			t.Fatalf("%s blocked with Kimi = %#v", providerID, blockedPreflightByProvider)
		}
	}
	if blockedPreflightByProvider["kimi"].Status != "blocked" ||
		!strings.Contains(blockedPreflightByProvider["kimi"].BlockReason, "Credential") {
		t.Fatalf("Kimi preflight = %#v", blockedPreflightByProvider["kimi"])
	}
	blockedStart := blockedCommand
	blockedStart.Operation = "start"
	blockedStart.PreflightDigest = strings.Repeat("f", 64)
	isolatedStart, err := compiler.CompileMissionExecution(
		context.Background(), blockedStart,
	)
	if err != nil {
		t.Fatalf("start with blocked Kimi error = %v", err)
	}
	if len(isolatedStart.Request.InitialBlocks) != 3 {
		t.Fatalf("Kimi isolated initial blocks = %#v", isolatedStart.Request.InitialBlocks)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(), credentials.MetadataCommand{
			CommandID:  "credential-deepseek-revoke",
			ProviderID: "deepseek", ProviderAccountID: "deepseek.backup",
			CredentialReference: "credential-ref-deepseek-backup",
			ExpectedRevision:    2, OccurredAt: now.Add(31 * time.Second),
			Status: credentials.CredentialRevoked,
		},
	); err != nil {
		t.Fatal(err)
	}
	rebuild("revoked fallback credential")
	fallbackBlocked, err := source.ResolveMissionExecutionBinding(
		context.Background(), "team-mixed-instance",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range fallbackBlocked.Roles {
		if role.Role != teams.ExecutionRoleMain {
			continue
		}
		if role.Status != "ready" || role.BlockReason != "" ||
			role.FallbackStatus != "blocked" ||
			!strings.Contains(role.FallbackBlockReason, "Credential") {
			t.Fatalf("isolated fallback health = %#v", role)
		}
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
			LogicalNodeID:  "main",
			Title:          command.Objective,
			Role:           "main",
			DependsOn:      []string{},
			MaxAttempts:    2,
			HarnessAdapter: "pi",
			ProviderID:     "loom-local",
			ModelID:        "qwen2.5-coder-1.5b",
			AuthMode:       string(loomruntime.AuthNative),
			TimeoutSeconds: 60,
			Capabilities:   []string{},
			Status:         "ready",
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

func TestMissionExecutionRoutePreviewRejectsCredentialAndFallbackDrift(t *testing.T) {
	node := MissionExecutionNodePreview{
		HarnessAdapter:             "codex",
		ProviderID:                 "openai",
		ProviderAccountID:          "openai.primary",
		ModelID:                    "gpt-5.5-codex",
		AuthMode:                   string(loomruntime.AuthBrokered),
		CredentialRevision:         3,
		TimeoutSeconds:             120,
		Capabilities:               []string{},
		Status:                     "ready",
		FallbackConfigured:         true,
		FallbackHarnessAdapter:     "claude-code",
		FallbackProviderID:         "anthropic",
		FallbackProviderAccountID:  "anthropic.backup",
		FallbackModelID:            "claude-sonnet",
		FallbackAuthMode:           string(loomruntime.AuthBrokered),
		FallbackCredentialRevision: 5,
		FallbackTimeoutSeconds:     90,
		FallbackCapabilities:       []string{},
		FallbackStatus:             "ready",
		FallbackApprovalRequired:   true,
	}
	validPrimary := func(candidate MissionExecutionNodePreview) bool {
		return validMissionExecutionRoute(
			candidate.HarnessAdapter,
			candidate.ProviderID,
			candidate.ProviderAccountID,
			candidate.ModelID,
			candidate.AuthMode,
			candidate.CredentialRevision,
			candidate.ReasoningEffort,
			candidate.TimeoutSeconds,
			candidate.BudgetCredits,
			candidate.Capabilities,
			candidate.Status,
			candidate.BlockReason,
		)
	}
	if !validPrimary(node) || !validMissionExecutionFallback(node) {
		t.Fatalf("valid route preview rejected: %#v", node)
	}
	for name, mutate := range map[string]func(*MissionExecutionNodePreview){
		"brokered account missing": func(candidate *MissionExecutionNodePreview) {
			candidate.ProviderAccountID = ""
		},
		"brokered revision missing": func(candidate *MissionExecutionNodePreview) {
			candidate.CredentialRevision = 0
		},
		"native account retained": func(candidate *MissionExecutionNodePreview) {
			candidate.AuthMode = string(loomruntime.AuthNative)
		},
		"timeout missing": func(candidate *MissionExecutionNodePreview) {
			candidate.TimeoutSeconds = 0
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := node
			mutate(&candidate)
			if validPrimary(candidate) {
				t.Fatalf("credential drift accepted: %#v", candidate)
			}
		})
	}
	missingFallbackRevision := node
	missingFallbackRevision.FallbackCredentialRevision = 0
	if validMissionExecutionFallback(missingFallbackRevision) {
		t.Fatal("brokered fallback revision drift accepted")
	}
	nativeFallback := node
	nativeFallback.FallbackAuthMode = string(loomruntime.AuthNative)
	nativeFallback.FallbackProviderAccountID = ""
	nativeFallback.FallbackCredentialRevision = 0
	if !validMissionExecutionFallback(nativeFallback) {
		t.Fatalf("legal native fallback rejected: %#v", nativeFallback)
	}
	hiddenFallback := MissionExecutionNodePreview{
		FallbackProviderID: "deepseek",
	}
	if validMissionExecutionFallback(hiddenFallback) {
		t.Fatal("unconfigured fallback carried hidden route fields")
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
	control.ContextVersion = 0
	control.ConfirmedConstraints = nil
	control.AcceptedDecisions = nil
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
	command := MissionExecutionCommand{
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
	if operation != missionExecutionControl {
		command.ContextVersion = MissionContextVersion
		command.ConfirmedConstraints = []string{}
		command.AcceptedDecisions = []string{}
	}
	return command
}

func TestMissionExecutionAcceptsMultilineObjectiveAndBuildsSingleLineTitle(t *testing.T) {
	command := missionExecutionTestCommand(missionExecutionPreflight)
	command.Objective = "Build the game.\n\nRequirements:\n1. Keyboard controls\n2. Mobile controls"
	if !validMissionExecutionCommand(command, missionExecutionPreflight) {
		t.Fatal("multiline Mission objective was rejected")
	}
	title := missionExecutionObjectiveTitle(command.Objective)
	if title != "Build the game. Requirements: 1. Keyboard controls 2. Mobile controls" ||
		strings.ContainsAny(title, "\r\n") {
		t.Fatalf("single-line Mission title = %q", title)
	}

	command.Objective = "Build the game.\rHidden"
	if validMissionExecutionCommand(command, missionExecutionPreflight) {
		t.Fatal("carriage return must remain rejected")
	}
	command.Objective = "Build the game.\x00Hidden"
	if validMissionExecutionCommand(command, missionExecutionPreflight) {
		t.Fatal("NUL must remain rejected")
	}
}

func TestMissionContextValidationAndPreflightDigestFreezeConfirmedAuthority(t *testing.T) {
	command := missionExecutionTestCommand(missionExecutionPreflight)
	command.ConfirmedConstraints = []string{"Do not change public APIs"}
	command.AcceptedDecisions = []string{"Use the existing execution adapter"}
	if !validMissionExecutionCommand(command, missionExecutionPreflight) {
		t.Fatal("canonical Mission Context v1 command was rejected")
	}
	preflight := MissionExecutionPreflight{
		SchemaVersion: MissionExecutionSchemaVersion,
		MissionID:     command.MissionID, TeamInstanceID: command.TeamInstanceID,
		WorkPackageID: command.WorkPackageID, WorkPackageDigest: command.WorkPackageDigest,
		ViewVersion: command.ExpectedViewVersion, PlanDigest: executionTestDigest("plan"),
		RuntimeInstanceID: "runtime-pi", RuntimeProfileID: "profile-pi",
		ModelID: "model-pi", AuthMode: "native_auth", CapacityAvailable: 1,
		BudgetStatus: "unavailable", SideEffects: []string{}, PermissionScopes: []string{},
		ApprovalPoints: []string{}, Nodes: []MissionExecutionNodePreview{},
	}
	first, err := missionExecutionPreflightDigest(command, preflight)
	if err != nil {
		t.Fatal(err)
	}
	drifted := command
	drifted.AcceptedDecisions = []string{"Use a replacement execution adapter"}
	second, err := missionExecutionPreflightDigest(drifted, preflight)
	if err != nil || first == second {
		t.Fatalf("Mission Context digest drift = %q/%q, %v", first, second, err)
	}
	for _, invalid := range []MissionExecutionCommand{
		func() MissionExecutionCommand { value := command; value.ContextVersion = 2; return value }(),
		func() MissionExecutionCommand { value := command; value.ConfirmedConstraints = nil; return value }(),
		func() MissionExecutionCommand {
			value := command
			value.ConfirmedConstraints = []string{" padded "}
			return value
		}(),
		func() MissionExecutionCommand {
			value := command
			value.AcceptedDecisions = []string{"Do not change public APIs"}
			return value
		}(),
	} {
		if validMissionExecutionCommand(invalid, missionExecutionPreflight) {
			t.Fatalf("invalid Mission Context accepted = %#v", invalid)
		}
	}
}

func TestMissionWorkspacePathValidationAndPreflightDigestFreeze(t *testing.T) {
	workspace := t.TempDir()
	command := missionExecutionTestCommand(missionExecutionPreflight)
	command.WorkspacePath = workspace
	if !validMissionExecutionCommand(command, missionExecutionPreflight) {
		t.Fatal("valid Mission workspace path was rejected")
	}

	preflight := MissionExecutionPreflight{
		SchemaVersion: MissionExecutionSchemaVersion,
		MissionID:     command.MissionID, TeamInstanceID: command.TeamInstanceID,
		WorkPackageID: command.WorkPackageID, WorkPackageDigest: command.WorkPackageDigest,
		ViewVersion: command.ExpectedViewVersion, PlanDigest: executionTestDigest("plan"),
		RuntimeInstanceID: "runtime-pi", RuntimeProfileID: "profile-pi",
		ModelID: "model-pi", AuthMode: "native_auth", CapacityAvailable: 1,
		BudgetStatus: "unavailable", SideEffects: []string{}, PermissionScopes: []string{},
		ApprovalPoints: []string{}, Nodes: []MissionExecutionNodePreview{},
	}
	first, err := missionExecutionPreflightDigest(command, preflight)
	if err != nil {
		t.Fatal(err)
	}
	drifted := command
	drifted.WorkspacePath = t.TempDir()
	second, err := missionExecutionPreflightDigest(drifted, preflight)
	if err != nil || first == second {
		t.Fatalf("workspace digest drift = %q/%q, %v", first, second, err)
	}

	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(workspace, symlinkPath); err != nil {
		t.Fatal(err)
	}
	inaccessible := t.TempDir()
	if err := os.Chmod(inaccessible, 0o077); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inaccessible, 0o700) })
	for name, path := range map[string]string{
		"relative":           "relative/workspace",
		"unclean":            workspace + string(filepath.Separator) + ".",
		"file":               filePath,
		"symlink":            symlinkPath,
		"owner inaccessible": inaccessible,
	} {
		t.Run(name, func(t *testing.T) {
			invalid := command
			invalid.WorkspacePath = path
			if validMissionExecutionCommand(invalid, missionExecutionPreflight) {
				t.Fatalf("invalid workspace path accepted: %q", path)
			}
		})
	}

	start := command
	start.Operation = missionExecutionStart
	start.PreflightDigest = first
	if !validMissionExecutionCommand(start, missionExecutionStart) {
		t.Fatal("valid start workspace path was rejected")
	}
	control := missionExecutionTestCommand(missionExecutionControl)
	control.WorkPackageID = ""
	control.WorkPackageDigest = ""
	control.Objective = ""
	control.ContextVersion = 0
	control.ConfirmedConstraints = nil
	control.AcceptedDecisions = nil
	control.ControlAction = "cancel"
	control.ExecutionDigest = executionTestDigest("execution")
	control.LogicalNodeID = "main"
	control.AttemptNumber = 1
	control.ClaimGeneration = 1
	control.WorkspacePath = workspace
	if validMissionExecutionCommand(control, missionExecutionControl) {
		t.Fatal("control command accepted workspace_path")
	}
}

func mustMissionFallbackDecisionScope(
	t *testing.T,
	teamInstanceID string,
	planDigest string,
	logicalNodeID string,
	sourceBindingDigest string,
	targetBindingDigest string,
) work.TeamFallbackDecisionScope {
	t.Helper()
	scope, err := work.NewTeamFallbackDecisionScope(
		work.TeamFallbackDecisionScopeInput{
			Version: 1, TeamInstanceID: teamInstanceID,
			PlanDigest: planDigest, LogicalNodeID: logicalNodeID,
			SourceBindingDigest: sourceBindingDigest,
			TargetBindingDigest: targetBindingDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func executionTestDigest(label string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(label)))
}

func TestResolveMissionExecutionProfileCarriesRemoteToolEnrollment(t *testing.T) {
	database := openTeamCanaryDB(t)
	store := journal.NewStore(database)
	now := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	digest := func(seed int) string { return fmt.Sprintf("%064x", seed) }
	correlation := "11111111-1111-4111-8111-111111111111"
	appendEvent := func(event journal.Event) {
		t.Helper()
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	payload, err := json.Marshal(map[string]any{
		"discovery_digest": digest(7), "source_probe_id": "probe-loom",
		"instance": map[string]any{
			"id": "runtime.loom-native.local", "device_id": "device-local",
			"adapter_type": "loom-native", "display_name": "Loom Native",
			"executable_version": "v1", "status": "online",
			"observed_capabilities": []string{"context_retrieval"}, "capacity": 3,
		},
		"model_ids": []string{"deepseek/deepseek-chat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	appendEvent(journal.Event{
		ID: "runtime-loom", StreamID: "runtime_instance:runtime.loom-native.local",
		Seq: 1, IdempotencyKey: "runtime-loom-idem",
		Type: "RuntimeInstanceDiscovered", SchemaVersion: 1,
		EmittedAt: now, CorrelationID: correlation, PayloadJSON: payload,
	})
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	enrollmentDigest := digest(9)
	resolved, err := resolveMissionExecutionProfile(
		readModel.GlobalReadView(),
		"team-enroll",
		projection.TeamExecutionProfileRecord{
			Version: 1, ID: "profile-enroll",
			HarnessAdapter:      "loom-native",
			ProviderID:          "deepseek",
			ProviderAccountID:   "deepseek.primary",
			ModelID:             "deepseek-chat",
			AuthMode:            loomruntime.AuthBrokered,
			EndpointFingerprint: strings.Repeat("f", 64),
			CredentialReference: "credential-ref-deepseek", CredentialRevision: 1,
			Timeout:                    time.Minute,
			RemoteToolEnrollmentID:     "enroll-web-live-1",
			RemoteToolEnrollmentDigest: enrollmentDigest,
		},
		"runtime.loom-native.local",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.profile.RemoteToolEnrollmentID != "enroll-web-live-1" ||
		resolved.profile.RemoteToolEnrollmentDigest != enrollmentDigest {
		t.Fatalf("preflight resolution dropped the Enrollment pair: %#v", resolved.profile)
	}
}
