package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

var (
	ErrInvalidTeamCoordinator  = errors.New("invalid Team coordinator")
	ErrTeamExecutionIncomplete = errors.New("Team execution incomplete")
)

type ManagedNodeExecutor interface {
	Execute(context.Context, supervisor.ExecuteInput) (supervisor.Outcome, error)
}

type NodeOutput struct {
	logicalNodeID   string
	attemptNumber   int
	authorizedFrame supervisor.AuthorizedFrame
}

func (output NodeOutput) LogicalNodeID() string { return output.logicalNodeID }
func (output NodeOutput) AttemptNumber() int    { return output.attemptNumber }
func (output NodeOutput) AuthorizedFrame() supervisor.AuthorizedFrame {
	return output.authorizedFrame
}
func (NodeOutput) Tentative() bool { return true }

type NodeOutputObserver interface {
	ObserveNodeOutput(context.Context, NodeOutput) error
}

type TeamNodeExecution struct {
	LogicalNodeID string
	AttemptNumber int
	SourcePath    string
	Profile       loomruntime.RuntimeProfile
	Instance      loomruntime.RuntimeInstance
	Dispatch      bridgev1.Frame
	Executor      ManagedNodeExecutor
}

type TeamExecutionRequest struct {
	Plan                 teams.ExecutionPlan
	Nodes                []TeamNodeExecution
	AuthoritativeTime    time.Time
	PrepareLeaseDuration time.Duration
	GrantLifetime        time.Duration
	CorrelationID        string
	OutputObserver       NodeOutputObserver
}

type TeamExecutionResult struct {
	team            work.TeamExecutionRecord
	executedNodeIDs []string
}

func (result TeamExecutionResult) Team() work.TeamExecutionRecord { return result.team }
func (result TeamExecutionResult) ExecutedNodeIDs() []string {
	return append([]string(nil), result.executedNodeIDs...)
}

type TeamCoordinator struct {
	workAuthority  *work.Authority
	grantAuthority *authorization.Authority
	projection     *projection.Projection
	evidenceStore  *evidence.Store
}

func NewTeamCoordinator(
	workAuthority *work.Authority,
	grantAuthority *authorization.Authority,
	readModel *projection.Projection,
	evidenceStore *evidence.Store,
) (*TeamCoordinator, error) {
	if workAuthority == nil || grantAuthority == nil ||
		readModel == nil || evidenceStore == nil {
		return nil, ErrInvalidTeamCoordinator
	}
	return &TeamCoordinator{
		workAuthority:  workAuthority,
		grantAuthority: grantAuthority,
		projection:     readModel,
		evidenceStore:  evidenceStore,
	}, nil
}

func (coordinator *TeamCoordinator) Run(
	ctx context.Context,
	request TeamExecutionRequest,
) (TeamExecutionResult, error) {
	executions, err := validateTeamExecutionRequest(ctx, request)
	if err != nil {
		return TeamExecutionResult{}, err
	}
	executed := make([]string, 0, len(request.Nodes))
	for wave := 0; wave < 3; wave++ {
		if err := coordinator.projection.Rebuild(ctx); err != nil {
			return TeamExecutionResult{}, fmt.Errorf(
				"Team projection rebuild: %w",
				err,
			)
		}
		view := coordinator.projection.GlobalReadView()
		if projected, ok := view.TeamExecution(
			request.Plan.TeamInstanceID(),
		); ok && appTerminalTeamStatus(projected.Status) {
			team, err := coordinator.workAuthority.TeamExecution(
				ctx,
				request.Plan.TeamInstanceID(),
			)
			if err != nil {
				return TeamExecutionResult{}, err
			}
			sort.Strings(executed)
			return TeamExecutionResult{
				team:            team,
				executedNodeIDs: executed,
			}, nil
		}
		recoveryTasks, recoveryChanged, err := coordinator.recoverTeamAttempts(
			ctx,
			request,
			executions,
			view,
		)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		if len(recoveryTasks) > 0 {
			outcomes := executeTeamTasks(ctx, recoveryTasks)
			for _, outcome := range outcomes {
				executed = append(executed, outcome.task.logicalNodeID)
			}
			if err := coordinator.commitTeamTaskOutcomes(
				ctx,
				request,
				outcomes,
			); err != nil {
				return TeamExecutionResult{}, err
			}
			continue
		}
		if recoveryChanged {
			continue
		}
		states := appExecutionStates(request.Plan, view)
		planningPlan, err := appPlanningPlan(request.Plan, view)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		capacities, err := appRuntimeCapacities(planningPlan, view)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		ready, err := teams.ReadyExecutionNodes(
			planningPlan,
			states,
			capacities,
			request.AuthoritativeTime,
		)
		if err != nil {
			return TeamExecutionResult{}, fmt.Errorf(
				"Team ready-set dispatch: %w",
				err,
			)
		}
		if len(ready) == 0 {
			break
		}
		selections := make([]work.TeamAttemptSelection, 0, len(ready))
		selectedExecutions := make([]TeamNodeExecution, 0, len(ready))
		for _, node := range ready {
			attemptNumber := appReadyAttemptNumber(states, node.LogicalNodeID())
			execution, exists := executions[appExecutionKey(
				node.LogicalNodeID(),
				attemptNumber,
			)]
			if !exists {
				return TeamExecutionResult{}, ErrTeamExecutionIncomplete
			}
			selections = append(selections, work.TeamAttemptSelection{
				LogicalNodeID: node.LogicalNodeID(),
				AttemptNumber: attemptNumber,
			})
			selectedExecutions = append(selectedExecutions, execution)
		}
		expectedHeads := appDispatchHeads(
			view,
			request.Plan,
			selections,
			ready,
		)
		dispatched, err := coordinator.workAuthority.DispatchTeamReadySet(
			ctx,
			work.TeamDispatchInput{
				Plan:                 request.Plan,
				ReadyAttempts:        selections,
				ViewVersion:          view.Version(),
				ExpectedHeads:        expectedHeads,
				AuthoritativeTime:    request.AuthoritativeTime,
				PrepareLeaseDuration: request.PrepareLeaseDuration,
				CorrelationID:        request.CorrelationID,
			},
		)
		if err != nil {
			return TeamExecutionResult{}, fmt.Errorf(
				"Team ready-set dispatch: %w",
				err,
			)
		}
		tasks, err := coordinator.prepareTeamTasks(
			ctx,
			request,
			dispatched,
			selectedExecutions,
		)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		outcomes := executeTeamTasks(ctx, tasks)
		for _, outcome := range outcomes {
			executed = append(executed, outcome.task.logicalNodeID)
		}
		if err := coordinator.commitTeamTaskOutcomes(
			ctx,
			request,
			outcomes,
		); err != nil {
			return TeamExecutionResult{}, err
		}
	}
	if err := coordinator.projection.Rebuild(ctx); err != nil {
		return TeamExecutionResult{}, fmt.Errorf(
			"Team projection rebuild: %w",
			err,
		)
	}
	team, err := coordinator.workAuthority.TeamExecution(
		ctx,
		request.Plan.TeamInstanceID(),
	)
	if err != nil {
		return TeamExecutionResult{}, err
	}
	sort.Strings(executed)
	if appTerminalTeamStatus(team.Status()) {
		return TeamExecutionResult{
			team:            team,
			executedNodeIDs: executed,
		}, nil
	}
	return TeamExecutionResult{
		team:            team,
		executedNodeIDs: executed,
	}, ErrTeamExecutionIncomplete
}

func (coordinator *TeamCoordinator) commitTeamTaskOutcomes(
	ctx context.Context,
	request TeamExecutionRequest,
	outcomes []teamTaskOutcome,
) error {
	for _, outcome := range outcomes {
		terminalStatus := outcome.outcome.Run().TerminalStatus()
		if terminalStatus == "" {
			return errors.Join(ErrTeamExecutionIncomplete, outcome.err)
		}
		receipt, err := coordinator.evidenceStore.FinalizeAttemptCapture(
			ctx,
			outcome.task.evidenceID,
			evidence.AttemptTerminal{
				Status: terminalStatus,
				Reason: outcome.outcome.Run().TerminalReason(),
			},
		)
		if err != nil {
			return fmt.Errorf(
				"Team attempt capture finalize %s/%d: %w",
				outcome.task.logicalNodeID,
				outcome.task.attemptNumber,
				errors.Join(err, outcome.err),
			)
		}
		if err := coordinator.commitTeamAttemptReceipt(
			ctx,
			request,
			outcome.task.logicalNodeID,
			outcome.task.attemptNumber,
			outcome.task.generation,
			receipt,
		); err != nil {
			return fmt.Errorf(
				"Team attempt evidence commit %s/%d: %w",
				outcome.task.logicalNodeID,
				outcome.task.attemptNumber,
				errors.Join(err, outcome.err),
			)
		}
	}
	return nil
}

func (coordinator *TeamCoordinator) commitTeamAttemptReceipt(
	ctx context.Context,
	request TeamExecutionRequest,
	logicalNodeID string,
	attemptNumber int,
	generation work.RunGenerationInput,
	receipt evidence.AttemptReceipt,
) error {
	_, err := coordinator.workAuthority.CommitTeamAttemptEvidence(
		ctx,
		work.TeamAttemptEvidenceInput{
			TeamInstanceID:    request.Plan.TeamInstanceID(),
			PlanDigest:        request.Plan.Digest(),
			LogicalNodeID:     logicalNodeID,
			AttemptNumber:     attemptNumber,
			WorkItemID:        generation.WorkItemID,
			RunID:             generation.RunID,
			ClaimID:           generation.ClaimID,
			ClaimGeneration:   generation.ClaimGeneration,
			RuntimeInstanceID: generation.RuntimeInstanceID,
			AgentInstanceID:   generation.AgentInstanceID,
			EvidenceID:        receipt.EvidenceID(),
			EvidenceDigest:    receipt.Digest(),
			CorrelationID:     request.CorrelationID,
		},
	)
	return err
}

func (coordinator *TeamCoordinator) recoverTeamAttempts(
	ctx context.Context,
	request TeamExecutionRequest,
	executions map[string]TeamNodeExecution,
	view projection.GlobalReadView,
) ([]teamExecutionTask, bool, error) {
	projected, exists := view.TeamExecution(request.Plan.TeamInstanceID())
	if !exists {
		return nil, false, nil
	}
	var tasks []teamExecutionTask
	changed := false
	for _, node := range projected.Nodes {
		if node.Status != "running" {
			continue
		}
		attempt, ok := appProjectedCurrentAttempt(node)
		if !ok || attempt.Status != "dispatched" {
			return nil, changed, work.ErrTeamAttemptRecoveryRequired
		}
		run, ok := view.Run(attempt.RunID)
		if !ok ||
			run.WorkItemID != attempt.WorkItemID ||
			run.RuntimeInstanceID != attempt.RuntimeInstanceID ||
			run.AgentInstanceID != attempt.AgentInstanceID {
			return nil, changed, work.ErrTeamAttemptRecoveryRequired
		}
		evidenceID := appTeamAttemptIdentity(
			"evidence",
			request.Plan,
			node.LogicalNodeID,
			attempt.AttemptNumber,
		)
		binding := appAttemptCaptureBinding(
			request.Plan,
			node.LogicalNodeID,
			attempt,
			evidenceID,
		)
		receipt, receiptFound, err := coordinator.evidenceStore.AttemptReceipt(
			ctx,
			evidenceID,
		)
		if err != nil {
			return nil, changed, err
		}
		capture, captureFound, err := coordinator.evidenceStore.AttemptCapture(
			ctx,
			evidenceID,
		)
		if err != nil {
			return nil, changed, err
		}
		switch run.Phase {
		case "terminal":
			if run.ClaimID != attempt.ClaimID ||
				run.ClaimGeneration != attempt.ClaimGeneration ||
				!captureFound ||
				capture.Binding() != binding {
				return nil, changed, work.ErrTeamAttemptRecoveryRequired
			}
			if !receiptFound {
				receipt, err = coordinator.evidenceStore.FinalizeAttemptCapture(
					ctx,
					evidenceID,
					evidence.AttemptTerminal{
						Status: run.TerminalStatus,
						Reason: run.TerminalReason,
					},
				)
				if err != nil {
					return nil, changed, err
				}
			}
			generation := appProjectionRunGeneration(run, request.CorrelationID)
			if err := coordinator.commitTeamAttemptReceipt(
				ctx,
				request,
				node.LogicalNodeID,
				attempt.AttemptNumber,
				generation,
				receipt,
			); err != nil {
				return nil, changed, err
			}
			changed = true
		case "running":
			return nil, changed, work.ErrTeamAttemptRecoveryRequired
		case "claimed":
			if receiptFound ||
				run.ClaimGeneration < attempt.ClaimGeneration ||
				run.ClaimGeneration > attempt.ClaimGeneration+1 {
				return nil, changed, work.ErrTeamAttemptRecoveryRequired
			}
			latestGrant, grantFound := view.LatestAgentGrantForRun(run.ID)
			if !captureFound {
				runHead, headFound := view.Head("run/" + run.ID)
				if grantFound ||
					!headFound ||
					runHead.Sequence != 1 ||
					run.ClaimGeneration != attempt.ClaimGeneration ||
					run.ClaimID != attempt.ClaimID {
					return nil, changed, work.ErrTeamAttemptRecoveryRequired
				}
				if err := coordinator.evidenceStore.BeginAttemptCapture(
					ctx,
					binding,
				); err != nil {
					return nil, changed, err
				}
				capture, captureFound, err = coordinator.evidenceStore.AttemptCapture(
					ctx,
					evidenceID,
				)
				if err != nil || !captureFound {
					return nil, changed, errors.Join(
						work.ErrTeamAttemptRecoveryRequired,
						err,
					)
				}
				changed = true
			}
			runBinding := binding
			runBinding.ClaimID = run.ClaimID
			runBinding.ClaimGeneration = run.ClaimGeneration
			captureBinding := capture.Binding()
			if captureBinding != binding &&
				(run.ClaimGeneration != attempt.ClaimGeneration+1 ||
					captureBinding != runBinding) {
				return nil, changed, work.ErrTeamAttemptRecoveryRequired
			}
			if run.ClaimGeneration == attempt.ClaimGeneration &&
				request.AuthoritativeTime.Before(run.PrepareLeaseExpiresAt) {
				continue
			}
			if capture.FrameCount() != 0 {
				return nil, changed, work.ErrTeamAttemptRecoveryRequired
			}
			if grantFound {
				if latestGrant.ClaimGeneration != attempt.ClaimGeneration ||
					latestGrant.ClaimID != attempt.ClaimID ||
					latestGrant.RunID != attempt.RunID {
					return nil, changed, work.ErrTeamAttemptRecoveryRequired
				}
				switch latestGrant.RevocationReason {
				case "":
					if _, err := coordinator.grantAuthority.Revoke(
						ctx,
						authorization.RevokeInput{
							GrantID:       latestGrant.ID,
							Reason:        authorization.RevocationOperator,
							CorrelationID: request.CorrelationID,
						},
					); err != nil {
						return nil, changed, err
					}
				case string(authorization.RevocationOperator):
				default:
					return nil, changed, work.ErrTeamAttemptRecoveryRequired
				}
			}
			reclaimed := run
			if run.ClaimGeneration == attempt.ClaimGeneration {
				_, record, err := coordinator.workAuthority.Claim(
					ctx,
					work.RunClaimInput{
						WorkItemID:           attempt.WorkItemID,
						RunID:                attempt.RunID,
						RuntimeInstanceID:    attempt.RuntimeInstanceID,
						AgentInstanceID:      attempt.AgentInstanceID,
						PrepareLeaseDuration: request.PrepareLeaseDuration,
						CorrelationID:        request.CorrelationID,
					},
				)
				if err != nil {
					return nil, changed, err
				}
				reclaimed = projection.Run{
					ID:                    record.ID(),
					WorkItemID:            record.WorkItemID(),
					Phase:                 record.Phase(),
					ClaimID:               record.ClaimID(),
					ClaimGeneration:       record.ClaimGeneration(),
					RuntimeInstanceID:     record.RuntimeInstanceID(),
					AgentInstanceID:       record.AgentInstanceID(),
					PrepareLeaseExpiresAt: record.PrepareLeaseExpiresAt(),
				}
			}
			reboundBinding := binding
			reboundBinding.ClaimID = reclaimed.ClaimID
			reboundBinding.ClaimGeneration = reclaimed.ClaimGeneration
			if err := coordinator.evidenceStore.RebindAttemptCapture(
				ctx,
				reboundBinding,
			); err != nil {
				return nil, changed, fmt.Errorf(
					"Team attempt capture rebind %s/%d: %w",
					node.LogicalNodeID,
					attempt.AttemptNumber,
					err,
				)
			}
			if _, err := coordinator.workAuthority.RebindTeamAttempt(
				ctx,
				work.TeamAttemptRebindInput{
					TeamInstanceID:          request.Plan.TeamInstanceID(),
					PlanDigest:              request.Plan.Digest(),
					LogicalNodeID:           node.LogicalNodeID,
					AttemptNumber:           attempt.AttemptNumber,
					WorkItemID:              attempt.WorkItemID,
					RunID:                   attempt.RunID,
					PreviousClaimID:         attempt.ClaimID,
					PreviousClaimGeneration: attempt.ClaimGeneration,
					ClaimID:                 reclaimed.ClaimID,
					ClaimGeneration:         reclaimed.ClaimGeneration,
					RuntimeInstanceID:       attempt.RuntimeInstanceID,
					AgentInstanceID:         attempt.AgentInstanceID,
					CorrelationID:           request.CorrelationID,
				},
			); err != nil {
				return nil, changed, err
			}
			execution, ok := executions[appExecutionKey(
				node.LogicalNodeID,
				attempt.AttemptNumber,
			)]
			if !ok {
				return nil, changed, ErrTeamExecutionIncomplete
			}
			task, err := coordinator.prepareRecoveredTeamTask(
				ctx,
				request,
				execution,
				reclaimed,
				evidenceID,
			)
			if err != nil {
				return nil, changed, err
			}
			tasks = append(tasks, task)
			changed = true
		default:
			return nil, changed, work.ErrTeamAttemptRecoveryRequired
		}
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].logicalNodeID < tasks[j].logicalNodeID
	})
	return tasks, changed, nil
}

type teamExecutionTask struct {
	logicalNodeID string
	attemptNumber int
	execution     TeamNodeExecution
	generation    work.RunGenerationInput
	grant         authorization.IssuedGrant
	evidenceID    string
	collector     *teamFrameCollector
}

type teamTaskOutcome struct {
	task    teamExecutionTask
	outcome supervisor.Outcome
	err     error
}

func (coordinator *TeamCoordinator) prepareTeamTasks(
	ctx context.Context,
	request TeamExecutionRequest,
	dispatched work.TeamDispatchResult,
	executions []TeamNodeExecution,
) ([]teamExecutionTask, error) {
	executionByKey := make(map[string]TeamNodeExecution, len(executions))
	for _, execution := range executions {
		executionByKey[appExecutionKey(
			execution.LogicalNodeID,
			execution.AttemptNumber,
		)] = execution
	}
	nodes := dispatched.Nodes()
	tasks := make([]teamExecutionTask, 0, len(nodes))
	for _, node := range nodes {
		attempt := node.Attempt()
		execution, exists := executionByKey[appExecutionKey(
			node.LogicalNode().LogicalNodeID(),
			attempt.AttemptNumber(),
		)]
		if !exists {
			return nil, ErrTeamExecutionIncomplete
		}
		run := node.Run()
		generation := work.RunGenerationInput{
			WorkItemID:        run.WorkItemID(),
			RunID:             run.ID(),
			ClaimID:           run.ClaimID(),
			ClaimGeneration:   run.ClaimGeneration(),
			RuntimeInstanceID: run.RuntimeInstanceID(),
			AgentInstanceID:   run.AgentInstanceID(),
			CorrelationID:     request.CorrelationID,
		}
		if !appDispatchMatches(execution.Dispatch, generation) {
			return nil, ErrInvalidTeamCoordinator
		}
		evidenceID := appTeamAttemptIdentity(
			"evidence",
			request.Plan,
			execution.LogicalNodeID,
			execution.AttemptNumber,
		)
		if err := coordinator.evidenceStore.BeginAttemptCapture(
			ctx,
			evidence.AttemptCaptureInput{
				EvidenceID:        evidenceID,
				TeamInstanceID:    request.Plan.TeamInstanceID(),
				PlanDigest:        request.Plan.Digest(),
				LogicalNodeID:     execution.LogicalNodeID,
				AttemptNumber:     execution.AttemptNumber,
				WorkItemID:        generation.WorkItemID,
				RunID:             generation.RunID,
				ClaimID:           generation.ClaimID,
				ClaimGeneration:   generation.ClaimGeneration,
				RuntimeInstanceID: generation.RuntimeInstanceID,
				AgentInstanceID:   generation.AgentInstanceID,
			},
		); err != nil {
			return nil, err
		}
		grant, err := coordinator.grantAuthority.Issue(
			ctx,
			authorization.IssueInput{
				WorkItemID:        generation.WorkItemID,
				RunID:             generation.RunID,
				ClaimID:           generation.ClaimID,
				ClaimGeneration:   generation.ClaimGeneration,
				RuntimeInstanceID: generation.RuntimeInstanceID,
				AgentInstanceID:   generation.AgentInstanceID,
				AllowedOperations: []authorization.Operation{
					authorization.OperationBridgeAck,
					authorization.OperationBridgeEvent,
					authorization.OperationBridgeEvidence,
					authorization.OperationBridgeHeartbeat,
					authorization.OperationBridgeResult,
				},
				Lifetime:      request.GrantLifetime,
				CorrelationID: request.CorrelationID,
			},
		)
		if err != nil {
			return nil, err
		}
		collector := &teamFrameCollector{
			logicalNodeID: execution.LogicalNodeID,
			attemptNumber: execution.AttemptNumber,
			observer:      request.OutputObserver,
			evidenceStore: coordinator.evidenceStore,
			evidenceID:    evidenceID,
		}
		tasks = append(tasks, teamExecutionTask{
			logicalNodeID: execution.LogicalNodeID,
			attemptNumber: execution.AttemptNumber,
			execution:     execution,
			generation:    generation,
			grant:         grant,
			evidenceID:    evidenceID,
			collector:     collector,
		})
	}
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].logicalNodeID < tasks[j].logicalNodeID
	})
	return tasks, nil
}

func (coordinator *TeamCoordinator) prepareRecoveredTeamTask(
	ctx context.Context,
	request TeamExecutionRequest,
	execution TeamNodeExecution,
	run projection.Run,
	evidenceID string,
) (teamExecutionTask, error) {
	generation := appProjectionRunGeneration(run, request.CorrelationID)
	original := execution.Dispatch
	if original.WorkItemID() != generation.WorkItemID ||
		original.RunID() != generation.RunID ||
		original.RuntimeInstanceID() != generation.RuntimeInstanceID ||
		original.SenderAgentInstanceID() != generation.AgentInstanceID ||
		original.CorrelationID() != generation.CorrelationID ||
		original.Sequence() != 1 ||
		original.Type() != bridgev1.MessageDispatch {
		return teamExecutionTask{}, ErrInvalidTeamCoordinator
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             original.MessageID(),
		CorrelationID:         original.CorrelationID(),
		WorkItemID:            original.WorkItemID(),
		RunID:                 original.RunID(),
		ClaimGeneration:       generation.ClaimGeneration,
		RuntimeInstanceID:     original.RuntimeInstanceID(),
		SenderAgentInstanceID: original.SenderAgentInstanceID(),
		Sequence:              original.Sequence(),
		Type:                  original.Type(),
		EmittedAt:             request.AuthoritativeTime,
		Payload:               original.Payload(),
	})
	if err != nil {
		return teamExecutionTask{}, err
	}
	execution.Dispatch = dispatch
	grant, err := coordinator.issueTeamGrant(ctx, request, generation)
	if err != nil {
		return teamExecutionTask{}, err
	}
	collector := &teamFrameCollector{
		logicalNodeID: execution.LogicalNodeID,
		attemptNumber: execution.AttemptNumber,
		observer:      request.OutputObserver,
		evidenceStore: coordinator.evidenceStore,
		evidenceID:    evidenceID,
	}
	return teamExecutionTask{
		logicalNodeID: execution.LogicalNodeID,
		attemptNumber: execution.AttemptNumber,
		execution:     execution,
		generation:    generation,
		grant:         grant,
		evidenceID:    evidenceID,
		collector:     collector,
	}, nil
}

func (coordinator *TeamCoordinator) issueTeamGrant(
	ctx context.Context,
	request TeamExecutionRequest,
	generation work.RunGenerationInput,
) (authorization.IssuedGrant, error) {
	return coordinator.grantAuthority.Issue(
		ctx,
		authorization.IssueInput{
			WorkItemID:        generation.WorkItemID,
			RunID:             generation.RunID,
			ClaimID:           generation.ClaimID,
			ClaimGeneration:   generation.ClaimGeneration,
			RuntimeInstanceID: generation.RuntimeInstanceID,
			AgentInstanceID:   generation.AgentInstanceID,
			AllowedOperations: []authorization.Operation{
				authorization.OperationBridgeAck,
				authorization.OperationBridgeEvent,
				authorization.OperationBridgeEvidence,
				authorization.OperationBridgeHeartbeat,
				authorization.OperationBridgeResult,
			},
			Lifetime:      request.GrantLifetime,
			CorrelationID: request.CorrelationID,
		},
	)
}

func executeTeamTasks(
	ctx context.Context,
	tasks []teamExecutionTask,
) []teamTaskOutcome {
	outcomes := make([]teamTaskOutcome, len(tasks))
	var wait sync.WaitGroup
	wait.Add(len(tasks))
	for index := range tasks {
		index := index
		go func() {
			defer wait.Done()
			task := tasks[index]
			outcome, err := task.execution.Executor.Execute(
				ctx,
				supervisor.ExecuteInput{
					SourcePath:    task.execution.SourcePath,
					Profile:       task.execution.Profile,
					Instance:      task.execution.Instance,
					Generation:    task.generation,
					Grant:         task.grant,
					Dispatch:      task.execution.Dispatch,
					FrameObserver: task.collector,
				},
			)
			outcomes[index] = teamTaskOutcome{
				task:    task,
				outcome: outcome,
				err:     err,
			}
		}()
	}
	wait.Wait()
	return outcomes
}

func appProjectedCurrentAttempt(
	node projection.TeamExecutionNode,
) (projection.TeamExecutionAttempt, bool) {
	for _, attempt := range node.Attempts {
		if attempt.AttemptNumber == node.CurrentAttempt {
			return attempt, true
		}
	}
	return projection.TeamExecutionAttempt{}, false
}

func appAttemptCaptureBinding(
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attempt projection.TeamExecutionAttempt,
	evidenceID string,
) evidence.AttemptCaptureInput {
	return evidence.AttemptCaptureInput{
		EvidenceID:        evidenceID,
		TeamInstanceID:    plan.TeamInstanceID(),
		PlanDigest:        plan.Digest(),
		LogicalNodeID:     logicalNodeID,
		AttemptNumber:     attempt.AttemptNumber,
		WorkItemID:        attempt.WorkItemID,
		RunID:             attempt.RunID,
		ClaimID:           attempt.ClaimID,
		ClaimGeneration:   attempt.ClaimGeneration,
		RuntimeInstanceID: attempt.RuntimeInstanceID,
		AgentInstanceID:   attempt.AgentInstanceID,
	}
}

func appProjectionRunGeneration(
	run projection.Run,
	correlationID string,
) work.RunGenerationInput {
	return work.RunGenerationInput{
		WorkItemID:        run.WorkItemID,
		RunID:             run.ID,
		ClaimID:           run.ClaimID,
		ClaimGeneration:   run.ClaimGeneration,
		RuntimeInstanceID: run.RuntimeInstanceID,
		AgentInstanceID:   run.AgentInstanceID,
		CorrelationID:     correlationID,
	}
}

type teamFrameCollector struct {
	logicalNodeID string
	attemptNumber int
	observer      NodeOutputObserver
	evidenceStore *evidence.Store
	evidenceID    string
}

func (collector *teamFrameCollector) ObserveAuthorizedFrame(
	ctx context.Context,
	frame supervisor.AuthorizedFrame,
) error {
	line, err := bridgev1.EncodeLine(frame.Frame())
	if err != nil {
		return err
	}
	if err := collector.evidenceStore.AppendAttemptFrame(
		ctx,
		collector.evidenceID,
		line,
	); err != nil {
		return fmt.Errorf("Team attempt capture append: %w", err)
	}
	if collector.observer == nil {
		return nil
	}
	return collector.observer.ObserveNodeOutput(ctx, NodeOutput{
		logicalNodeID:   collector.logicalNodeID,
		attemptNumber:   collector.attemptNumber,
		authorizedFrame: frame,
	})
}

func validateTeamExecutionRequest(
	ctx context.Context,
	request TeamExecutionRequest,
) (map[string]TeamNodeExecution, error) {
	if ctx == nil {
		return nil, ErrInvalidTeamCoordinator
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	nodes := request.Plan.Nodes()
	if len(nodes) == 0 || len(nodes) > 3 ||
		len(request.Nodes) == 0 || len(request.Nodes) > 9 ||
		request.AuthoritativeTime.IsZero() ||
		request.AuthoritativeTime.Location() != time.UTC ||
		request.PrepareLeaseDuration <= 0 ||
		request.PrepareLeaseDuration > 5*time.Minute ||
		request.GrantLifetime <= 0 ||
		request.GrantLifetime > time.Hour ||
		request.CorrelationID == "" ||
		request.OutputObserver != nil &&
			nilAppInterface(request.OutputObserver) {
		return nil, ErrInvalidTeamCoordinator
	}
	planNodes := make(map[string]teams.ExecutionNode, len(nodes))
	for _, node := range nodes {
		planNodes[node.LogicalNodeID()] = node
	}
	executions := make(map[string]TeamNodeExecution, len(request.Nodes))
	for _, execution := range request.Nodes {
		node, exists := planNodes[execution.LogicalNodeID]
		if !exists ||
			execution.AttemptNumber < 1 ||
			execution.AttemptNumber > node.MaxAttempts() ||
			execution.SourcePath == "" ||
			nilAppInterface(execution.Executor) {
			return nil, ErrInvalidTeamCoordinator
		}
		key := appExecutionKey(
			execution.LogicalNodeID,
			execution.AttemptNumber,
		)
		if _, duplicate := executions[key]; duplicate {
			return nil, ErrInvalidTeamCoordinator
		}
		executions[key] = execution
	}
	return executions, nil
}

func appExecutionStates(
	plan teams.ExecutionPlan,
	view projection.GlobalReadView,
) []teams.ExecutionNodeState {
	projected, exists := view.TeamExecution(plan.TeamInstanceID())
	if !exists {
		states := make([]teams.ExecutionNodeState, 0, len(plan.Nodes()))
		for _, node := range plan.Nodes() {
			states = append(states, teams.ExecutionNodeState{
				LogicalNodeID: node.LogicalNodeID(),
				Status:        "pending",
			})
		}
		return states
	}
	states := make([]teams.ExecutionNodeState, len(projected.Nodes))
	for index, node := range projected.Nodes {
		states[index] = teams.ExecutionNodeState{
			LogicalNodeID:  node.LogicalNodeID,
			Status:         node.Status,
			CurrentAttempt: node.CurrentAttempt,
			RetryAt:        node.RetryAt,
		}
	}
	return states
}

func appPlanningPlan(
	plan teams.ExecutionPlan,
	view projection.GlobalReadView,
) (teams.ExecutionPlan, error) {
	projected, exists := view.TeamExecution(plan.TeamInstanceID())
	if !exists {
		return plan, nil
	}
	projectedNodes := make(map[string]projection.TeamExecutionNode, len(projected.Nodes))
	for _, node := range projected.Nodes {
		projectedNodes[node.LogicalNodeID] = node
	}
	inputs := make([]teams.ExecutionNodeInput, 0, len(plan.Nodes()))
	for _, node := range plan.Nodes() {
		agentInstanceID := node.AgentInstanceID()
		runtimeInstanceID := node.RuntimeInstanceID()
		if projectedNode, ok := projectedNodes[node.LogicalNodeID()]; ok {
			for _, attempt := range projectedNode.Attempts {
				if attempt.AttemptNumber != projectedNode.CurrentAttempt {
					continue
				}
				agentInstanceID = attempt.AgentInstanceID
				runtimeInstanceID = attempt.RuntimeInstanceID
				break
			}
		}
		inputs = append(inputs, teams.ExecutionNodeInput{
			LogicalNodeID:     node.LogicalNodeID(),
			Title:             node.Title(),
			AgentInstanceID:   agentInstanceID,
			RuntimeInstanceID: runtimeInstanceID,
			Role:              node.Role(),
			DependsOn:         node.DependsOn(),
			MaxAttempts:       node.MaxAttempts(),
		})
	}
	planningPlan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: plan.TeamInstanceID(),
		Nodes:          inputs,
	})
	if err != nil {
		return teams.ExecutionPlan{}, ErrInvalidTeamCoordinator
	}
	return planningPlan, nil
}

func appRuntimeCapacities(
	plan teams.ExecutionPlan,
	view projection.GlobalReadView,
) ([]teams.RuntimeCapacityState, error) {
	seen := make(map[string]struct{})
	capacities := make([]teams.RuntimeCapacityState, 0, len(plan.Nodes()))
	for _, node := range plan.Nodes() {
		if _, exists := seen[node.RuntimeInstanceID()]; exists {
			continue
		}
		seen[node.RuntimeInstanceID()] = struct{}{}
		instance, exists := view.RuntimeInstance(node.RuntimeInstanceID())
		if !exists || instance.Status != "online" {
			return nil, work.ErrRuntimeUnavailable
		}
		capacities = append(capacities, teams.RuntimeCapacityState{
			RuntimeInstanceID: instance.ID,
			Capacity:          instance.Capacity,
			Active:            view.ActiveRunCount(instance.ID),
		})
	}
	return capacities, nil
}

func appReadyAttemptNumber(
	states []teams.ExecutionNodeState,
	logicalNodeID string,
) int {
	for _, state := range states {
		if state.LogicalNodeID != logicalNodeID {
			continue
		}
		if state.CurrentAttempt == 0 {
			return 1
		}
		return state.CurrentAttempt
	}
	return 1
}

func appDispatchHeads(
	view projection.GlobalReadView,
	plan teams.ExecutionPlan,
	selections []work.TeamAttemptSelection,
	nodes []teams.ExecutionNode,
) []journal.StreamHead {
	streams := map[string]struct{}{
		"team-execution/" + plan.TeamInstanceID(): {},
		"work-run-identity/v1":                    {},
	}
	for _, selection := range selections {
		var runtimeInstanceID string
		for _, node := range nodes {
			if node.LogicalNodeID() == selection.LogicalNodeID {
				runtimeInstanceID = node.RuntimeInstanceID()
				break
			}
		}
		workItemID := appTeamAttemptIdentity(
			"work", plan, selection.LogicalNodeID, selection.AttemptNumber,
		)
		runID := appTeamAttemptIdentity(
			"run", plan, selection.LogicalNodeID, selection.AttemptNumber,
		)
		streams["work-item/"+workItemID] = struct{}{}
		streams["run/"+runID] = struct{}{}
		streams["runtime_instance:"+runtimeInstanceID] = struct{}{}
		streams["runtime_capacity:"+runtimeInstanceID] = struct{}{}
	}
	streamIDs := make([]string, 0, len(streams))
	for streamID := range streams {
		streamIDs = append(streamIDs, streamID)
	}
	sort.Strings(streamIDs)
	heads := make([]journal.StreamHead, len(streamIDs))
	for index, streamID := range streamIDs {
		head, exists := view.Head(streamID)
		if !exists {
			head = journal.StreamHead{StreamID: streamID}
		}
		heads[index] = head
	}
	return heads
}

func appTeamAttemptIdentity(
	label string,
	plan teams.ExecutionPlan,
	logicalNodeID string,
	attemptNumber int,
) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		label,
		plan.TeamInstanceID(),
		plan.Digest(),
		logicalNodeID,
		fmt.Sprint(attemptNumber),
	}, "\x00")))
	return "team-" + label + "-" + hex.EncodeToString(digest[:16])
}

func appExecutionKey(logicalNodeID string, attemptNumber int) string {
	return logicalNodeID + "\x00" + fmt.Sprint(attemptNumber)
}

func appDispatchMatches(
	frame bridgev1.Frame,
	generation work.RunGenerationInput,
) bool {
	return frame.WorkItemID() == generation.WorkItemID &&
		frame.RunID() == generation.RunID &&
		frame.ClaimGeneration() == generation.ClaimGeneration &&
		frame.RuntimeInstanceID() == generation.RuntimeInstanceID &&
		frame.SenderAgentInstanceID() == generation.AgentInstanceID &&
		frame.CorrelationID() == generation.CorrelationID &&
		frame.Sequence() == 1 &&
		frame.Type() == bridgev1.MessageDispatch
}

func appTerminalTeamStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "degraded", "blocked", "human_required", "cancelled":
		return true
	default:
		return false
	}
}
