package app

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
	"loom-pi-rebuild/internal/rules"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
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
	WorkflowPath  string
	SourcePath    string
	Profile       loomruntime.RuntimeProfile
	Instance      loomruntime.RuntimeInstance
	Dispatch      bridgev1.Frame
	Executor      ManagedNodeExecutor
}

type TeamVerifierExecution struct {
	SourcePath string
	Profile    loomruntime.RuntimeProfile
	Instance   loomruntime.RuntimeInstance
	Executor   ManagedNodeExecutor
}

type TeamNodeSemantics struct {
	LogicalNodeID             string
	OutputContract            verification.OutputContract
	RecoveryPolicy            rules.RecoveryPolicy
	AcceptanceContract        verification.AcceptanceContract
	PrimaryWorkflowPath       string
	VerifierAgentInstanceID   string
	VerifierRuntimeInstanceID string
	VerifierWorkflowPath      string
	VerifierExecution         *TeamVerifierExecution
}

type TeamExecutionRequest struct {
	Plan                 teams.ExecutionPlan
	Nodes                []TeamNodeExecution
	Semantics            []TeamNodeSemantics
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
	for wave := 0; wave < 9; wave++ {
		if err := coordinator.projection.Rebuild(ctx); err != nil {
			return TeamExecutionResult{}, fmt.Errorf(
				"Team projection rebuild: %w",
				err,
			)
		}
		view := coordinator.projection.GlobalReadView()
		if projected, ok := view.TeamExecution(
			request.Plan.TeamInstanceID(),
		); ok {
			if err := appValidateProjectedSemantics(
				request,
				projected,
			); err != nil {
				return TeamExecutionResult{}, err
			}
			if appTerminalTeamStatus(projected.Status) {
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
			expectedWorkflowPath, err := appExpectedWorkflowPath(
				request,
				view,
				node.LogicalNodeID(),
				attemptNumber,
			)
			if err != nil ||
				execution.WorkflowPath != expectedWorkflowPath {
				return TeamExecutionResult{}, errors.Join(
					ErrInvalidTeamCoordinator,
					err,
				)
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
				SemanticBindings:     appSemanticBindings(request),
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

func appValidateProjectedSemantics(
	request TeamExecutionRequest,
	projected projection.TeamExecution,
) error {
	if projected.LegacySemanticUnbound {
		return work.ErrTeamAttemptRecoveryRequired
	}
	if projected.PlanDigest != request.Plan.Digest() ||
		len(projected.Nodes) != len(request.Semantics) {
		return work.ErrTeamExecutionConflict
	}
	for _, semantics := range request.Semantics {
		var node *projection.TeamExecutionNode
		for index := range projected.Nodes {
			if projected.Nodes[index].LogicalNodeID ==
				semantics.LogicalNodeID {
				node = &projected.Nodes[index]
				break
			}
		}
		if node == nil ||
			node.OutputContractVersion !=
				semantics.OutputContract.Version() ||
			node.OutputContractDigest !=
				semantics.OutputContract.Digest() ||
			node.RecoveryPolicyVersion !=
				semantics.RecoveryPolicy.Version() ||
			node.RecoveryPolicyDigest !=
				semantics.RecoveryPolicy.Digest() ||
			node.AttemptCredits !=
				semantics.RecoveryPolicy.AttemptCredits() ||
			node.PrimaryWorkflowPath !=
				semantics.PrimaryWorkflowPath ||
			node.WorkflowFallbackKey !=
				semantics.RecoveryPolicy.WorkflowFallbackKey() ||
			node.RecoveryApprovalRequired !=
				semantics.RecoveryPolicy.RecoveryApprovalRequired() ||
			node.AcceptanceContractVersion !=
				semantics.AcceptanceContract.Version() ||
			node.AcceptanceContractDigest !=
				semantics.AcceptanceContract.Digest() ||
			node.AcceptanceRisk !=
				string(semantics.AcceptanceContract.Risk()) ||
			node.IndependentVerifierRequired !=
				semantics.AcceptanceContract.
					IndependentVerifierRequired() ||
			node.VerifierAgentInstanceID !=
				semantics.VerifierAgentInstanceID ||
			node.VerifierRuntimeInstanceID !=
				semantics.VerifierRuntimeInstanceID ||
			node.VerifierWorkflowPath !=
				semantics.VerifierWorkflowPath {
			return work.ErrTeamExecutionConflict
		}
	}
	return nil
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
	semantics, ok := appNodeSemantics(request, logicalNodeID)
	if !ok {
		return ErrInvalidTeamCoordinator
	}
	summary := receipt.OutputSummary()
	observation, err := verification.NewOutputObservation(
		receipt.EvidenceID(),
		receipt.Digest(),
		summary.Digest(),
		summary.AuthorizedFrameCount(),
		summary.OutputFrameCount(),
		summary.OutputPayloadBytes(),
		summary.ResultObserved(),
		summary.TerminalStatus(),
	)
	if err != nil {
		return err
	}
	classification, err := verification.Classify(
		semantics.OutputContract,
		observation,
	)
	if err != nil {
		return err
	}
	team, err := coordinator.workAuthority.CommitTeamAttemptEvidence(
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
			Receipt:           receipt,
			Classification:    classification,
			CorrelationID:     request.CorrelationID,
		},
	)
	if err != nil {
		return err
	}
	var current work.TeamNodeRecord
	found := false
	for _, node := range team.Nodes() {
		if node.LogicalNodeID() == logicalNodeID {
			current = node
			found = true
			break
		}
	}
	if !found || current.Status() != "awaiting_recovery" {
		if !found || current.Status() != "ready_for_review" {
			return nil
		}
		result, err := verification.VerifyDeterministic(
			semantics.AcceptanceContract,
			verification.DeterministicVerificationInput{
				TeamInstanceID:             request.Plan.TeamInstanceID(),
				PlanDigest:                 request.Plan.Digest(),
				LogicalNodeID:              logicalNodeID,
				AttemptNumber:              attemptNumber,
				WorkItemID:                 generation.WorkItemID,
				RunID:                      generation.RunID,
				ClaimID:                    generation.ClaimID,
				ClaimGeneration:            generation.ClaimGeneration,
				SourceEvidenceID:           receipt.EvidenceID(),
				SourceEvidenceDigest:       receipt.Digest(),
				OutputSummaryDigest:        summary.Digest(),
				OutputContractVersion:      semantics.OutputContract.Version(),
				OutputContractDigest:       semantics.OutputContract.Digest(),
				OutputClassification:       classification.Kind(),
				OutputClassificationDigest: classification.Digest(),
				AcceptanceContractDigest:   semantics.AcceptanceContract.Digest(),
				TerminalStatus:             summary.TerminalStatus(),
			},
		)
		if err != nil {
			return err
		}
		var verifierCandidate verification.VerifierCandidate
		var verifierReceipt evidence.AttemptReceipt
		if result.Kind() ==
			verification.DeterministicNeedsIndependentVerifier {
			verifierCandidate, verifierReceipt, err =
				coordinator.runIndependentVerifier(
					ctx,
					request,
					semantics,
					result,
					receipt,
				)
			if err != nil {
				return err
			}
		}
		decision, err := verification.DecideAcceptance(
			verification.AcceptanceDecisionInput{
				Contract:            semantics.AcceptanceContract,
				DeterministicResult: result,
				VerifierCandidate:   verifierCandidate,
				DecisionTime:        request.AuthoritativeTime,
			},
		)
		if err != nil {
			return err
		}
		planNode := appPlanNode(request.Plan, logicalNodeID)
		if planNode.LogicalNodeID() == "" {
			return ErrInvalidTeamCoordinator
		}
		remainingCredits := semantics.RecoveryPolicy.AttemptCredits() -
			(attemptNumber - 1)
		if remainingCredits < 0 {
			remainingCredits = 0
		}
		acceptedTeam, err := coordinator.workAuthority.CommitTeamNodeAcceptance(
			ctx,
			work.TeamNodeAcceptanceInput{
				TeamInstanceID:      request.Plan.TeamInstanceID(),
				PlanDigest:          request.Plan.Digest(),
				LogicalNodeID:       logicalNodeID,
				AttemptNumber:       attemptNumber,
				SourceReceipt:       receipt,
				AcceptanceContract:  semantics.AcceptanceContract,
				DeterministicResult: result,
				VerifierCandidate:   verifierCandidate,
				VerifierReceipt:     verifierReceipt,
				Decision:            decision,
				RecoveryPolicy:      semantics.RecoveryPolicy,
				MaxAttempts:         planNode.MaxAttempts(),
				CreditsBefore:       remainingCredits,
				CorrelationID:       request.CorrelationID,
			},
		)
		if err != nil {
			return err
		}
		if decision.Kind() == verification.AcceptanceAccepted {
			return nil
		}
		return coordinator.scheduleTeamRecoveryDecision(
			ctx,
			request,
			semantics,
			acceptedTeam,
			logicalNodeID,
			attemptNumber,
			generation,
			receipt,
			classification,
			rules.RecoveryTriggerVerificationRejected,
			decision.Digest(),
		)
	}
	recoveryTrigger := rules.RecoveryTriggerOutput
	acceptanceDecisionDigest := ""
	if current.RecoveryTrigger() ==
		string(rules.RecoveryTriggerVerificationRejected) {
		recoveryTrigger = rules.RecoveryTriggerVerificationRejected
		acceptanceDecisionDigest =
			current.AcceptanceDecisionDigest()
	}
	return coordinator.scheduleTeamRecoveryDecision(
		ctx,
		request,
		semantics,
		team,
		logicalNodeID,
		attemptNumber,
		generation,
		receipt,
		classification,
		recoveryTrigger,
		acceptanceDecisionDigest,
	)
}

func (coordinator *TeamCoordinator) scheduleTeamRecoveryDecision(
	ctx context.Context,
	request TeamExecutionRequest,
	semantics TeamNodeSemantics,
	team work.TeamExecutionRecord,
	logicalNodeID string,
	attemptNumber int,
	generation work.RunGenerationInput,
	receipt evidence.AttemptReceipt,
	classification verification.Classification,
	trigger rules.RecoveryTrigger,
	acceptanceDecisionDigest string,
) error {
	planNode := appPlanNode(request.Plan, logicalNodeID)
	if planNode.LogicalNodeID() == "" {
		return ErrInvalidTeamCoordinator
	}
	var current work.TeamNodeRecord
	found := false
	for _, node := range team.Nodes() {
		if node.LogicalNodeID() == logicalNodeID {
			current = node
			found = true
			break
		}
	}
	if !found || current.Status() != "awaiting_recovery" {
		return work.ErrTeamAttemptRecoveryRequired
	}
	prior := make([]verification.OutputClassification, 0, attemptNumber-1)
	fallbackConsumed := false
	for _, attempt := range current.Attempts() {
		if attempt.AttemptNumber() < attemptNumber {
			prior = append(prior, attempt.OutputClassification())
		}
		if semantics.RecoveryPolicy.WorkflowFallbackKey() != "" &&
			attempt.WorkflowPath() ==
				semantics.RecoveryPolicy.WorkflowFallbackKey() {
			fallbackConsumed = true
		}
	}
	remainingCredits := semantics.RecoveryPolicy.AttemptCredits() -
		(attemptNumber - 1)
	if remainingCredits < 0 {
		remainingCredits = 0
	}
	decision, err := rules.DecideRecovery(
		semantics.RecoveryPolicy,
		rules.RecoveryInput{
			TeamInstanceID:           request.Plan.TeamInstanceID(),
			PlanDigest:               request.Plan.Digest(),
			LogicalNodeID:            logicalNodeID,
			AttemptNumber:            attemptNumber,
			MaxAttempts:              planNode.MaxAttempts(),
			AgentInstanceID:          generation.AgentInstanceID,
			RuntimeInstanceID:        generation.RuntimeInstanceID,
			EvidenceID:               receipt.EvidenceID(),
			EvidenceDigest:           receipt.Digest(),
			OutputSummaryDigest:      receipt.OutputSummary().Digest(),
			Classification:           classification,
			PriorClassifications:     prior,
			RemainingCredits:         remainingCredits,
			FallbackConsumed:         fallbackConsumed,
			DecisionTime:             request.AuthoritativeTime,
			Trigger:                  trigger,
			AcceptanceDecisionDigest: acceptanceDecisionDigest,
		},
	)
	if err != nil {
		return err
	}
	if decision.Action() == rules.RecoveryNone {
		return ErrTeamExecutionIncomplete
	}
	_, err = coordinator.workAuthority.ScheduleTeamNodeRecovery(
		ctx,
		work.TeamRecoveryInput{
			Decision:      decision,
			CorrelationID: request.CorrelationID,
		},
	)
	return err
}

func (coordinator *TeamCoordinator) runIndependentVerifier(
	ctx context.Context,
	request TeamExecutionRequest,
	semantics TeamNodeSemantics,
	result verification.DeterministicVerificationResult,
	sourceReceipt evidence.AttemptReceipt,
) (
	verification.VerifierCandidate,
	evidence.AttemptReceipt,
	error,
) {
	execution := semantics.VerifierExecution
	if execution == nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			work.ErrIndependentVerificationRequired
	}
	source := result.Input()
	workItemID := appVerifierIdentity(
		"verifier-work",
		source.TeamInstanceID,
		source.PlanDigest,
		source.LogicalNodeID,
		fmt.Sprint(source.AttemptNumber),
		source.WorkItemID,
		source.RunID,
		source.SourceEvidenceDigest,
		semantics.AcceptanceContract.Digest(),
	)
	runID := appVerifierIdentity(
		"verifier-run",
		workItemID,
		semantics.VerifierAgentInstanceID,
		semantics.VerifierRuntimeInstanceID,
		semantics.VerifierWorkflowPath,
	)
	evidenceID := appVerifierIdentity(
		"verifier-evidence",
		workItemID,
		runID,
		source.SourceEvidenceDigest,
		semantics.AcceptanceContract.Digest(),
	)
	verifierLogicalNodeID := appVerifierIdentity(
		"verifier-node",
		source.TeamInstanceID,
		source.LogicalNodeID,
		fmt.Sprint(source.AttemptNumber),
	)[:64]
	if err := coordinator.projection.Rebuild(ctx); err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	view := coordinator.projection.GlobalReadView()
	var verifierRun work.RunRecord
	hasVerifierRun := false
	if projectedWork, exists := view.WorkItem(workItemID); exists {
		if projectedWork.RunID != runID ||
			projectedWork.AgentInstanceID !=
				semantics.VerifierAgentInstanceID {
			return verification.VerifierCandidate{},
				evidence.AttemptReceipt{},
				work.ErrVerifierLineageMismatch
		}
		projectedRun, ok := view.Run(runID)
		if !ok ||
			projectedRun.RuntimeInstanceID !=
				semantics.VerifierRuntimeInstanceID ||
			projectedRun.AgentInstanceID !=
				semantics.VerifierAgentInstanceID {
			return verification.VerifierCandidate{},
				evidence.AttemptReceipt{},
				work.ErrVerifierLineageMismatch
		}
		if projectedRun.Phase == "claimed" {
			if request.AuthoritativeTime.Before(
				projectedRun.PrepareLeaseExpiresAt,
			) {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					work.ErrTeamAttemptRecoveryRequired
			}
			if receipt, found, receiptErr :=
				coordinator.evidenceStore.AttemptReceipt(
					ctx,
					evidenceID,
				); receiptErr != nil || found ||
				receipt.EvidenceID() != "" {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					errors.Join(
						work.ErrTeamAttemptRecoveryRequired,
						receiptErr,
					)
			}
			capture, captureFound, captureErr :=
				coordinator.evidenceStore.AttemptCapture(
					ctx,
					evidenceID,
				)
			if captureErr != nil ||
				captureFound && capture.FrameCount() != 0 {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					errors.Join(
						work.ErrTeamAttemptRecoveryRequired,
						captureErr,
					)
			}
			if previousGrant, found :=
				view.LatestAgentGrantForRun(runID); found {
				switch previousGrant.RevocationReason {
				case "":
					if _, revokeErr :=
						coordinator.grantAuthority.Revoke(
							ctx,
							authorization.RevokeInput{
								GrantID:       previousGrant.ID,
								Reason:        authorization.RevocationOperator,
								CorrelationID: request.CorrelationID,
							},
						); revokeErr != nil {
						return verification.VerifierCandidate{},
							evidence.AttemptReceipt{},
							revokeErr
					}
				case string(authorization.RevocationOperator):
				default:
					return verification.VerifierCandidate{},
						evidence.AttemptReceipt{},
						work.ErrTeamAttemptRecoveryRequired
				}
			}
			_, reclaimed, reclaimErr :=
				coordinator.workAuthority.Claim(
					ctx,
					work.RunClaimInput{
						WorkItemID:           workItemID,
						RunID:                runID,
						RuntimeInstanceID:    semantics.VerifierRuntimeInstanceID,
						AgentInstanceID:      semantics.VerifierAgentInstanceID,
						PrepareLeaseDuration: request.PrepareLeaseDuration,
						CorrelationID:        request.CorrelationID,
					},
				)
			if reclaimErr != nil {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					reclaimErr
			}
			if captureFound {
				if rebindErr :=
					coordinator.evidenceStore.RebindAttemptCapture(
						ctx,
						evidence.AttemptCaptureInput{
							EvidenceID:        evidenceID,
							TeamInstanceID:    source.TeamInstanceID,
							PlanDigest:        source.PlanDigest,
							LogicalNodeID:     verifierLogicalNodeID,
							AttemptNumber:     1,
							WorkItemID:        workItemID,
							RunID:             runID,
							ClaimID:           reclaimed.ClaimID(),
							ClaimGeneration:   reclaimed.ClaimGeneration(),
							RuntimeInstanceID: reclaimed.RuntimeInstanceID(),
							AgentInstanceID:   reclaimed.AgentInstanceID(),
						},
					); rebindErr != nil {
					return verification.VerifierCandidate{},
						evidence.AttemptReceipt{},
						rebindErr
				}
			}
			verifierRun = reclaimed
			hasVerifierRun = true
		} else if projectedRun.Phase != "terminal" {
			return verification.VerifierCandidate{},
				evidence.AttemptReceipt{},
				work.ErrTeamAttemptRecoveryRequired
		}
		if !hasVerifierRun {
			receipt, found, err := coordinator.evidenceStore.AttemptReceipt(
				ctx,
				evidenceID,
			)
			if err != nil || !found {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					errors.Join(work.ErrTeamAttemptRecoveryRequired, err)
			}
			grant, found := view.LatestAgentGrantForRun(runID)
			if !found {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					work.ErrVerifierLineageMismatch
			}
			if err := coordinator.workAuthority.CommitVerifierEvidence(
				ctx,
				work.VerifierEvidenceInput{
					WorkItemID:        workItemID,
					RunID:             runID,
					ClaimID:           projectedRun.ClaimID,
					ClaimGeneration:   projectedRun.ClaimGeneration,
					RuntimeInstanceID: projectedRun.RuntimeInstanceID,
					AgentInstanceID:   projectedRun.AgentInstanceID,
					Receipt:           receipt,
					CorrelationID:     request.CorrelationID,
				},
			); err != nil {
				return verification.VerifierCandidate{},
					evidence.AttemptReceipt{},
					err
			}
			candidate, err := verification.VerifierCandidateFromTerminal(
				verification.VerifierTerminalInput{
					WorkItemID:          workItemID,
					RunID:               runID,
					ClaimID:             projectedRun.ClaimID,
					ClaimGeneration:     projectedRun.ClaimGeneration,
					RuntimeInstanceID:   projectedRun.RuntimeInstanceID,
					AgentInstanceID:     projectedRun.AgentInstanceID,
					GrantID:             grant.ID,
					EvidenceID:          receipt.EvidenceID(),
					EvidenceDigest:      receipt.Digest(),
					OutputSummaryDigest: receipt.OutputSummary().Digest(),
					TerminalStatus:      projectedRun.TerminalStatus,
					TerminalReason:      projectedRun.TerminalReason,
				},
			)
			return candidate, receipt, err
		}
	}
	if !hasVerifierRun {
		_, _, err := coordinator.workAuthority.CreateAndAssign(
			ctx,
			work.WorkItemAssignmentInput{
				WorkItemID:      workItemID,
				Title:           "Independent verification",
				RunID:           runID,
				AgentInstanceID: semantics.VerifierAgentInstanceID,
				CorrelationID:   request.CorrelationID,
			},
		)
		if err != nil {
			return verification.VerifierCandidate{},
				evidence.AttemptReceipt{},
				err
		}
		_, verifierRun, err = coordinator.workAuthority.Claim(
			ctx,
			work.RunClaimInput{
				WorkItemID:           workItemID,
				RunID:                runID,
				RuntimeInstanceID:    semantics.VerifierRuntimeInstanceID,
				AgentInstanceID:      semantics.VerifierAgentInstanceID,
				PrepareLeaseDuration: request.PrepareLeaseDuration,
				CorrelationID:        request.CorrelationID,
			},
		)
		if err != nil {
			return verification.VerifierCandidate{},
				evidence.AttemptReceipt{},
				err
		}
	}
	generation := work.RunGenerationInput{
		WorkItemID:        workItemID,
		RunID:             runID,
		ClaimID:           verifierRun.ClaimID(),
		ClaimGeneration:   verifierRun.ClaimGeneration(),
		RuntimeInstanceID: semantics.VerifierRuntimeInstanceID,
		AgentInstanceID:   semantics.VerifierAgentInstanceID,
		CorrelationID:     request.CorrelationID,
	}
	if err := coordinator.evidenceStore.BeginAttemptCapture(
		ctx,
		evidence.AttemptCaptureInput{
			EvidenceID:        evidenceID,
			TeamInstanceID:    source.TeamInstanceID,
			PlanDigest:        source.PlanDigest,
			LogicalNodeID:     verifierLogicalNodeID,
			AttemptNumber:     1,
			WorkItemID:        workItemID,
			RunID:             runID,
			ClaimID:           generation.ClaimID,
			ClaimGeneration:   generation.ClaimGeneration,
			RuntimeInstanceID: generation.RuntimeInstanceID,
			AgentInstanceID:   generation.AgentInstanceID,
		},
	); err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	grant, err := coordinator.issueTeamGrant(ctx, request, generation)
	if err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	payload, err := json.Marshal(struct {
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
	}{
		source.TeamInstanceID,
		source.PlanDigest,
		source.LogicalNodeID,
		source.AttemptNumber,
		source.WorkItemID,
		source.RunID,
		sourceReceipt.Digest(),
		sourceReceipt.OutputSummary().Digest(),
		semantics.AcceptanceContract.Digest(),
		string(semantics.AcceptanceContract.Risk()),
		semantics.AcceptanceContract.Criteria(),
		[]string{
			string(verification.VerifierReasonCriteriaSatisfied),
			string(verification.VerifierReasonCriteriaNotSatisfied),
			string(verification.VerifierReasonInsufficientEvidence),
		},
	})
	if err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: appVerifierUUID(
			"verifier-dispatch",
			workItemID,
			runID,
		),
		CorrelationID:         request.CorrelationID,
		WorkItemID:            workItemID,
		RunID:                 runID,
		ClaimGeneration:       generation.ClaimGeneration,
		RuntimeInstanceID:     generation.RuntimeInstanceID,
		SenderAgentInstanceID: generation.AgentInstanceID,
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             request.AuthoritativeTime,
		Payload:               payload,
	})
	if err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	collector := &teamFrameCollector{
		logicalNodeID: verifierLogicalNodeID,
		attemptNumber: 1,
		evidenceStore: coordinator.evidenceStore,
		evidenceID:    evidenceID,
	}
	task := teamExecutionTask{
		logicalNodeID: verifierLogicalNodeID,
		attemptNumber: 1,
		execution: TeamNodeExecution{
			LogicalNodeID: verifierLogicalNodeID,
			AttemptNumber: 1,
			WorkflowPath:  semantics.VerifierWorkflowPath,
			SourcePath:    execution.SourcePath,
			Profile:       execution.Profile,
			Instance:      execution.Instance,
			Dispatch:      dispatch,
			Executor:      execution.Executor,
		},
		generation: generation,
		grant:      grant,
		evidenceID: evidenceID,
		collector:  collector,
	}
	outcomes := executeTeamTasks(ctx, []teamExecutionTask{task})
	if len(outcomes) != 1 {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			work.ErrTeamAttemptRecoveryRequired
	}
	if outcomes[0].outcome.Run().TerminalStatus() == "" {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			errors.Join(work.ErrTeamAttemptRecoveryRequired, outcomes[0].err)
	}
	terminal := outcomes[0].outcome.Run()
	receipt, err := coordinator.evidenceStore.FinalizeAttemptCapture(
		ctx,
		evidenceID,
		evidence.AttemptTerminal{
			Status: terminal.TerminalStatus(),
			Reason: terminal.TerminalReason(),
		},
	)
	if err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	if err := coordinator.workAuthority.CommitVerifierEvidence(
		ctx,
		work.VerifierEvidenceInput{
			WorkItemID:        workItemID,
			RunID:             runID,
			ClaimID:           terminal.ClaimID(),
			ClaimGeneration:   terminal.ClaimGeneration(),
			RuntimeInstanceID: terminal.RuntimeInstanceID(),
			AgentInstanceID:   terminal.AgentInstanceID(),
			Receipt:           receipt,
			CorrelationID:     request.CorrelationID,
		},
	); err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
	}
	candidate, err := verification.VerifierCandidateFromTerminal(
		verification.VerifierTerminalInput{
			WorkItemID:          workItemID,
			RunID:               runID,
			ClaimID:             terminal.ClaimID(),
			ClaimGeneration:     terminal.ClaimGeneration(),
			RuntimeInstanceID:   terminal.RuntimeInstanceID(),
			AgentInstanceID:     terminal.AgentInstanceID(),
			GrantID:             grant.Record().ID(),
			EvidenceID:          receipt.EvidenceID(),
			EvidenceDigest:      receipt.Digest(),
			OutputSummaryDigest: receipt.OutputSummary().Digest(),
			TerminalStatus:      terminal.TerminalStatus(),
			TerminalReason:      terminal.TerminalReason(),
		},
	)
	return candidate, receipt, err
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
		if node.Status == "awaiting_recovery" {
			attempt, ok := appProjectedCurrentAttempt(node)
			if !ok || attempt.EvidenceID == "" {
				return nil, changed, work.ErrTeamAttemptRecoveryRequired
			}
			receipt, found, err := coordinator.evidenceStore.AttemptReceipt(
				ctx,
				attempt.EvidenceID,
			)
			if err != nil || !found {
				return nil, changed, errors.Join(
					work.ErrTeamAttemptRecoveryRequired,
					err,
				)
			}
			run, ok := view.Run(attempt.RunID)
			if !ok {
				return nil, changed, work.ErrTeamAttemptRecoveryRequired
			}
			if err := coordinator.commitTeamAttemptReceipt(
				ctx,
				request,
				node.LogicalNodeID,
				attempt.AttemptNumber,
				appProjectionRunGeneration(run, request.CorrelationID),
				receipt,
			); err != nil {
				return nil, changed, err
			}
			changed = true
			continue
		}
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
		len(request.Semantics) != len(nodes) ||
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
	semanticNodes := make(map[string]TeamNodeSemantics, len(request.Semantics))
	for index, semantics := range request.Semantics {
		node, exists := planNodes[semantics.LogicalNodeID]
		if !exists ||
			index > 0 &&
				request.Semantics[index-1].LogicalNodeID >=
					semantics.LogicalNodeID ||
			!semantics.OutputContract.Valid() ||
			!semantics.RecoveryPolicy.Valid() ||
			!semantics.AcceptanceContract.Valid() ||
			semantics.RecoveryPolicy.AttemptCredits() >
				node.MaxAttempts()-1 ||
			!validAppWorkflowPath(semantics.PrimaryWorkflowPath) ||
			semantics.RecoveryPolicy.WorkflowFallbackKey() != "" &&
				(!validAppWorkflowPath(
					semantics.RecoveryPolicy.WorkflowFallbackKey(),
				) ||
					semantics.RecoveryPolicy.WorkflowFallbackKey() ==
						semantics.PrimaryWorkflowPath) ||
			!validAppVerifierBinding(semantics, node.AgentInstanceID()) {
			return nil, ErrInvalidTeamCoordinator
		}
		semanticNodes[semantics.LogicalNodeID] = semantics
	}
	executions := make(map[string]TeamNodeExecution, len(request.Nodes))
	for _, execution := range request.Nodes {
		node, exists := planNodes[execution.LogicalNodeID]
		if !exists ||
			execution.AttemptNumber < 1 ||
			execution.AttemptNumber > node.MaxAttempts() ||
			!validAppWorkflowPath(execution.WorkflowPath) ||
			execution.SourcePath == "" ||
			nilAppInterface(execution.Executor) {
			return nil, ErrInvalidTeamCoordinator
		}
		semantics := semanticNodes[execution.LogicalNodeID]
		if execution.AttemptNumber == 1 &&
			execution.WorkflowPath != semantics.PrimaryWorkflowPath ||
			execution.WorkflowPath != semantics.PrimaryWorkflowPath &&
				execution.WorkflowPath !=
					semantics.RecoveryPolicy.WorkflowFallbackKey() {
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
	for _, node := range nodes {
		for attemptNumber := 1; attemptNumber <= node.MaxAttempts(); attemptNumber++ {
			if _, ok := executions[appExecutionKey(
				node.LogicalNodeID(),
				attemptNumber,
			)]; !ok {
				return nil, ErrTeamExecutionIncomplete
			}
		}
	}
	return executions, nil
}

func appNodeSemantics(
	request TeamExecutionRequest,
	logicalNodeID string,
) (TeamNodeSemantics, bool) {
	for _, semantics := range request.Semantics {
		if semantics.LogicalNodeID == logicalNodeID {
			return semantics, true
		}
	}
	return TeamNodeSemantics{}, false
}

func appPlanNode(
	plan teams.ExecutionPlan,
	logicalNodeID string,
) teams.ExecutionNode {
	for _, node := range plan.Nodes() {
		if node.LogicalNodeID() == logicalNodeID {
			return node
		}
	}
	return teams.ExecutionNode{}
}

func appSemanticBindings(
	request TeamExecutionRequest,
) []work.TeamNodeSemanticBinding {
	result := make(
		[]work.TeamNodeSemanticBinding,
		0,
		len(request.Semantics),
	)
	for _, semantics := range request.Semantics {
		result = append(result, work.TeamNodeSemanticBinding{
			LogicalNodeID:               semantics.LogicalNodeID,
			OutputContractVersion:       semantics.OutputContract.Version(),
			OutputContractDigest:        semantics.OutputContract.Digest(),
			RecoveryPolicyVersion:       semantics.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:        semantics.RecoveryPolicy.Digest(),
			AttemptCredits:              semantics.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:         semantics.PrimaryWorkflowPath,
			WorkflowFallbackKey:         semantics.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired:    semantics.RecoveryPolicy.RecoveryApprovalRequired(),
			AcceptanceContractVersion:   semantics.AcceptanceContract.Version(),
			AcceptanceContractDigest:    semantics.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(semantics.AcceptanceContract.Risk()),
			IndependentVerifierRequired: semantics.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     semantics.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   semantics.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        semantics.VerifierWorkflowPath,
		})
	}
	return result
}

func validAppVerifierBinding(
	semantics TeamNodeSemantics,
	sourceAgentInstanceID string,
) bool {
	required := semantics.AcceptanceContract.IndependentVerifierRequired()
	if !required {
		return semantics.VerifierAgentInstanceID == "" &&
			semantics.VerifierRuntimeInstanceID == "" &&
			semantics.VerifierWorkflowPath == "" &&
			semantics.VerifierExecution == nil
	}
	return validAppWorkflowPath(semantics.VerifierAgentInstanceID) &&
		semantics.VerifierAgentInstanceID != sourceAgentInstanceID &&
		validAppWorkflowPath(semantics.VerifierRuntimeInstanceID) &&
		validAppWorkflowPath(semantics.VerifierWorkflowPath) &&
		semantics.VerifierExecution != nil &&
		semantics.VerifierExecution.SourcePath != "" &&
		!nilAppInterface(semantics.VerifierExecution.Executor) &&
		semantics.VerifierExecution.Instance.ID ==
			semantics.VerifierRuntimeInstanceID
}

func validAppWorkflowPath(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if current < 0x21 || current == 0x7f {
			return false
		}
	}
	return true
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

func appExpectedWorkflowPath(
	request TeamExecutionRequest,
	view projection.GlobalReadView,
	logicalNodeID string,
	attemptNumber int,
) (string, error) {
	if projected, ok := view.TeamExecution(
		request.Plan.TeamInstanceID(),
	); ok {
		for _, node := range projected.Nodes {
			if node.LogicalNodeID != logicalNodeID {
				continue
			}
			for _, attempt := range node.Attempts {
				if attempt.AttemptNumber == attemptNumber &&
					attempt.WorkflowPath != "" {
					return attempt.WorkflowPath, nil
				}
			}
		}
	}
	semantics, ok := appNodeSemantics(request, logicalNodeID)
	if !ok || attemptNumber != 1 {
		return "", ErrInvalidTeamCoordinator
	}
	return semantics.PrimaryWorkflowPath, nil
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

func appVerifierIdentity(label string, fields ...string) string {
	hash := sha256.New()
	writeVerifierIdentityField(hash, "loom."+label+".v1")
	for _, field := range fields {
		writeVerifierIdentityField(hash, field)
	}
	return label + "-" + hex.EncodeToString(hash.Sum(nil))
}

func appVerifierUUID(label string, fields ...string) string {
	hash := sha256.New()
	writeVerifierIdentityField(hash, "loom."+label+".v1")
	for _, field := range fields {
		writeVerifierIdentityField(hash, field)
	}
	value := hash.Sum(nil)[:16]
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value)
	return encoded[:8] + "-" + encoded[8:12] + "-" +
		encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func writeVerifierIdentityField(hash interface{ Write([]byte) (int, error) }, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hash.Write(length[:])
	_, _ = hash.Write([]byte(value))
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
