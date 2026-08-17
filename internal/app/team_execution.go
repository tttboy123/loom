package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/authorization"
	"loom-pi-rebuild/internal/contextcapsule"
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

const teamAttemptCommitTimeout = 15 * time.Second

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
	LogicalNodeID        string
	AttemptNumber        int
	WorkflowPath         string
	SourcePath           string
	SourceSnapshotDigest string
	Profile              loomruntime.RuntimeProfile
	Instance             loomruntime.RuntimeInstance
	Dispatch             bridgev1.Frame
	Executor             ManagedNodeExecutor
	ContextCapsule       contextcapsule.RoleContextCapsule
	Aggregation          *TeamAggregationExecution
	executionBinding     loomruntime.FrozenExecutionBinding
	routeSegment         contextcapsule.RouteSegmentBinding
}

type TeamContextCapsuleStore interface {
	PutRoleContextCapsule(
		context.Context,
		contextcapsule.RoleContextCapsule,
		[]byte,
	) error
}

type TeamVerifierExecution struct {
	SourcePath           string
	SourceSnapshotDigest string
	Profile              loomruntime.RuntimeProfile
	Instance             loomruntime.RuntimeInstance
	Executor             ManagedNodeExecutor
}

type TeamNodeSemantics struct {
	LogicalNodeID             string
	OutputContract            verification.OutputContract
	RecoveryPolicy            rules.RecoveryPolicy
	AcceptanceContract        verification.AcceptanceContract
	PrimaryWorkflowPath       string
	FallbackApproval          work.TeamFallbackApproval
	VerifierAgentInstanceID   string
	VerifierRuntimeInstanceID string
	VerifierWorkflowPath      string
	VerifierExecution         *TeamVerifierExecution
}

type TeamExecutionRequest struct {
	Plan                 teams.ExecutionPlan
	Nodes                []TeamNodeExecution
	Semantics            []TeamNodeSemantics
	InitialBlocks        []teams.InitialExecutionBlock
	RouteSummaries       []work.TeamNodeRouteSummary
	AuthoritativeTime    time.Time
	PrepareLeaseDuration time.Duration
	GrantLifetime        time.Duration
	CorrelationID        string
	OutputObserver       NodeOutputObserver
	ContextCapsules      TeamContextCapsuleStore
	AssetSourceStreamIDs []string
}

type TeamAssetMaterializationRequest struct {
	TeamExecutionID   string
	LogicalNodeID     string
	RunID             string
	AttemptNumber     int
	Generation        int64
	JourneyID         string
	ExpectedView      string
	Profile           loomruntime.RuntimeProfile
	Instance          loomruntime.RuntimeInstance
	Bindings          []assets.ExactAssetRevisionBinding
	RevisionSetDigest string
}

type TeamAssetMaterialization struct {
	SourcePath     string
	AttemptLineage work.TeamAttemptMaterialization
	RunID          string
	AttemptNumber  int
	Generation     int64
	JourneyID      string
	ManifestDigest string
	RootDigest     string
	Authoritative  bool
}

type TeamAssetMaterializer interface {
	PrepareTeamAttemptMaterialization(
		context.Context,
		TeamAssetMaterializationRequest,
	) (TeamAssetMaterialization, error)
	CleanupTeamAttemptMaterialization(
		context.Context,
		TeamAssetMaterialization,
	) error
}

type TeamGovernedTestReportSource interface {
	GovernedTestReportsForAttempt(
		context.Context,
		work.AttemptReportQuery,
	) ([]verification.GovernedTestReport, error)
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
	workAuthority     *work.Authority
	grantAuthority    *authorization.Authority
	projection        *projection.Projection
	evidenceStore     *evidence.Store
	assetMaterializer TeamAssetMaterializer
	testReportSource  TeamGovernedTestReportSource
}

func (coordinator *TeamCoordinator) SetAssetMaterializer(
	materializer TeamAssetMaterializer,
) error {
	if coordinator == nil || materializer == nil || coordinator.assetMaterializer != nil {
		return ErrInvalidTeamCoordinator
	}
	coordinator.assetMaterializer = materializer
	return nil
}

func (coordinator *TeamCoordinator) SetGovernedTestReportSource(
	source TeamGovernedTestReportSource,
) error {
	if coordinator == nil || nilAppInterface(source) || coordinator.testReportSource != nil {
		return ErrInvalidTeamCoordinator
	}
	coordinator.testReportSource = source
	return nil
}

type teamRecoveryPolicyPort struct {
	policy rules.RecoveryPolicy
}

func newTeamRecoveryPolicyPort(
	policy rules.RecoveryPolicy,
) teamRecoveryPolicyPort {
	return teamRecoveryPolicyPort{policy: policy}
}

func (port teamRecoveryPolicyPort) Valid() bool {
	return port.policy.Valid()
}
func (port teamRecoveryPolicyPort) Version() int {
	return port.policy.Version()
}
func (port teamRecoveryPolicyPort) Digest() string {
	return port.policy.Digest()
}
func (port teamRecoveryPolicyPort) AttemptCredits() int {
	return port.policy.AttemptCredits()
}
func (port teamRecoveryPolicyPort) RetryDelay() time.Duration {
	return port.policy.RetryDelay()
}
func (port teamRecoveryPolicyPort) RecoveryApprovalRequired() bool {
	return port.policy.RecoveryApprovalRequired()
}
func (port teamRecoveryPolicyPort) Decide(
	request work.TeamRecoveryDecisionRequest,
) (work.TeamRecoveryDecision, error) {
	return rules.DecideRecovery(
		port.policy,
		rules.RecoveryInput{
			TeamInstanceID:            request.TeamInstanceID,
			PlanDigest:                request.PlanDigest,
			LogicalNodeID:             request.LogicalNodeID,
			AttemptNumber:             request.AttemptNumber,
			MaxAttempts:               request.MaxAttempts,
			AgentInstanceID:           request.AgentInstanceID,
			RuntimeInstanceID:         request.RuntimeInstanceID,
			FallbackRuntimeInstanceID: request.FallbackRuntimeInstanceID,
			EvidenceID:                request.EvidenceID,
			EvidenceDigest:            request.EvidenceDigest,
			OutputSummaryDigest:       request.OutputSummaryDigest,
			Classification:            request.Classification,
			PriorClassifications:      request.PriorClassifications,
			RemainingCredits:          request.RemainingCredits,
			FallbackConsumed:          request.FallbackConsumed,
			DecisionTime:              request.DecisionTime,
			Trigger:                   rules.RecoveryTrigger(request.Trigger),
			AcceptanceDecisionDigest:  request.AcceptanceDecisionDigest,
		},
	)
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
			if err := coordinator.refreshTeamObservationView(ctx); err != nil {
				return TeamExecutionResult{}, err
			}
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
		states := appExecutionStates(request, view)
		planningPlan, err := appPlanningPlan(request.Plan, view)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		capacities, err := appRuntimeCapacities(planningPlan, states, view)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		readyTime := appReadyEvaluationTime(request, states)
		ready, err := teams.ReadyExecutionNodes(
			planningPlan,
			states,
			capacities,
			readyTime,
		)
		if err != nil {
			return TeamExecutionResult{}, fmt.Errorf(
				"Team ready-set dispatch: %w",
				err,
			)
		}
		if len(ready) == 0 && len(request.InitialBlocks) != len(request.Plan.Nodes()) {
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
			if node.Kind() == teams.ExecutionNodeAggregation {
				execution, err = coordinator.prepareAggregationExecution(
					ctx, request, view, node, execution,
				)
				if err != nil {
					return TeamExecutionResult{}, err
				}
				executions[appExecutionKey(node.LogicalNodeID(), attemptNumber)] = execution
			} else if len(node.DependsOn()) > 0 {
				execution, err = coordinator.prepareDependencyContextExecution(
					ctx, request, view, node, execution,
				)
				if err != nil {
					return TeamExecutionResult{}, err
				}
				executions[appExecutionKey(node.LogicalNodeID(), attemptNumber)] = execution
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
				LogicalNodeID:    node.LogicalNodeID(),
				AttemptNumber:    attemptNumber,
				ExecutionBinding: execution.executionBinding,
				ContextCapsule:   execution.ContextCapsule,
			})
			selectedExecutions = append(selectedExecutions, execution)
		}
		selectedExecutions, materializations, cleanupMaterializations, err :=
			coordinator.prepareTeamAssetMaterializations(
				ctx, request, view, ready, selections, selectedExecutions,
			)
		if err != nil {
			return TeamExecutionResult{}, err
		}
		expectedHeads := appDispatchHeadsWithSources(
			view,
			request.Plan,
			selections,
			ready,
			request.AssetSourceStreamIDs,
			materializations,
		)
		assetSourceHeads := appAssetSourceHeads(view, request.AssetSourceStreamIDs)
		waveRequest := request
		waveRequest.AuthoritativeTime = readyTime
		dispatched, err := coordinator.workAuthority.DispatchTeamReadySet(
			ctx,
			work.TeamDispatchInput{
				Plan:                 request.Plan,
				ReadyAttempts:        selections,
				InitialBlocks:        appInitialDispatchBlocks(request, view),
				RouteSummaries:       appInitialDispatchRoutes(request, view),
				SemanticBindings:     appSemanticBindings(request),
				Materializations:     materializations,
				AssetSourceHeads:     assetSourceHeads,
				ViewVersion:          view.Version(),
				ExpectedHeads:        expectedHeads,
				AuthoritativeTime:    waveRequest.AuthoritativeTime,
				PrepareLeaseDuration: request.PrepareLeaseDuration,
				CorrelationID:        request.CorrelationID,
			},
		)
		if err != nil {
			coordinator.cleanupTeamAssetMaterializations(ctx, cleanupMaterializations)
			return TeamExecutionResult{}, fmt.Errorf(
				"Team ready-set dispatch: %w",
				err,
			)
		}
		if err := coordinator.refreshTeamObservationView(ctx); err != nil {
			return TeamExecutionResult{}, err
		}
		if err := coordinator.verifyDispatchedAssetLineage(
			request.Plan.TeamInstanceID(), materializations,
		); err != nil {
			return TeamExecutionResult{}, err
		}
		if len(dispatched.Nodes()) == 0 {
			continue
		}
		tasks, err := coordinator.prepareTeamTasks(
			ctx,
			waveRequest,
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
			waveRequest,
			outcomes,
		); err != nil {
			return TeamExecutionResult{}, err
		}
		for index := range cleanupMaterializations {
			cleanupMaterializations[index].Authoritative = true
		}
		if err := coordinator.cleanupCommittedTeamAssetMaterializations(
			ctx, cleanupMaterializations,
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

func appAssetSourceHeads(
	view projection.GlobalReadView,
	streamIDs []string,
) []journal.StreamHead {
	heads := make([]journal.StreamHead, len(streamIDs))
	for index, streamID := range streamIDs {
		head, ok := view.Head(streamID)
		if !ok {
			head = journal.StreamHead{StreamID: streamID}
		}
		heads[index] = head
	}
	return heads
}

func (coordinator *TeamCoordinator) refreshTeamObservationView(
	ctx context.Context,
) error {
	if err := coordinator.projection.Rebuild(ctx); err != nil {
		return fmt.Errorf("Team projection rebuild: %w", err)
	}
	return nil
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
			!appProjectedFallbackApprovalMatches(
				*node,
				semantics.FallbackApproval,
			) ||
			node.FallbackRuntimeInstanceID !=
				appFallbackRuntimeInstanceID(request, semantics) ||
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
		if route, ok := appRouteSummaryByNode(request.RouteSummaries, semantics.LogicalNodeID); ok &&
			!appProjectedInitialRouteMatches(*node, route) {
			return work.ErrTeamExecutionConflict
		}
	}
	return nil
}

func appProjectedInitialRouteMatches(
	node projection.TeamExecutionNode,
	route work.TeamNodeRouteSummary,
) bool {
	if !node.InitialRouteAvailable {
		return true
	}
	if node.InitialHarnessAdapter != route.HarnessAdapter ||
		node.InitialProviderID != route.ProviderID ||
		node.InitialProviderAccountID != route.ProviderAccountID ||
		node.InitialModelID != route.ModelID ||
		node.InitialReasoningEffort != route.ReasoningEffort ||
		node.InitialTimeoutNanoseconds != route.TimeoutNanoseconds ||
		node.InitialCredentialRevision != route.CredentialRevision ||
		!reflect.DeepEqual(node.InitialCapabilities, route.Capabilities) {
		return false
	}
	return reflect.DeepEqual(node.InitialBudget, route.Budget)
}

func appProjectedFallbackApprovalMatches(
	node projection.TeamExecutionNode,
	approval work.TeamFallbackApproval,
) bool {
	if approval.Digest() == "" {
		return !node.FallbackApprovalAvailable
	}
	return approval.Valid() && node.FallbackApprovalAvailable &&
		node.FallbackApprovalVersion == approval.Version() &&
		node.FallbackApprovalID == approval.ApprovalID() &&
		node.FallbackApprovalActorRef == approval.ActorRef() &&
		node.FallbackApprovedAt.Equal(approval.ApprovedAt()) &&
		node.FallbackSourceBindingDigest == approval.SourceBindingDigest() &&
		node.FallbackTargetBindingDigest == approval.TargetBindingDigest() &&
		node.FallbackApprovalDigest == approval.Digest()
}

func (coordinator *TeamCoordinator) commitTeamTaskOutcomes(
	ctx context.Context,
	request TeamExecutionRequest,
	outcomes []teamTaskOutcome,
) error {
	var commitErr error
	for _, outcome := range outcomes {
		if err := coordinator.commitTeamTaskOutcome(ctx, request, outcome); err != nil {
			commitErr = errors.Join(commitErr, err)
		}
	}
	return commitErr
}

func (coordinator *TeamCoordinator) commitTeamTaskOutcome(
	ctx context.Context,
	request TeamExecutionRequest,
	outcome teamTaskOutcome,
) error {
	commitContext, cancelCommit := context.WithTimeout(
		context.WithoutCancel(ctx),
		teamAttemptCommitTimeout,
	)
	defer cancelCommit()
	terminalStatus := outcome.outcome.Run().TerminalStatus()
	if terminalStatus == "" {
		return errors.Join(ErrTeamExecutionIncomplete, outcome.err)
	}
	receipt, err := coordinator.evidenceStore.FinalizeAttemptCapture(
		commitContext,
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
		commitContext,
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
				RecoveryPolicy:      semantics.RecoveryPolicy,
				MaxAttempts:         planNode.MaxAttempts(),
				CreditsBefore:       remainingCredits,
				CorrelationID:       request.CorrelationID,
			},
		)
		if err != nil {
			return err
		}
		var acceptedNode work.TeamNodeRecord
		foundAcceptedNode := false
		for _, node := range acceptedTeam.Nodes() {
			if node.LogicalNodeID() == logicalNodeID {
				acceptedNode = node
				foundAcceptedNode = true
				break
			}
		}
		if !foundAcceptedNode {
			return ErrTeamExecutionIncomplete
		}
		if acceptedNode.Status() == "succeeded" {
			return nil
		}
		if acceptedNode.Status() != "awaiting_recovery" ||
			acceptedNode.AcceptanceDecisionDigest() == "" {
			return ErrTeamExecutionIncomplete
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
		)
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
	_, err := coordinator.workAuthority.ScheduleTeamNodeRecovery(
		ctx,
		work.TeamRecoveryInput{
			TeamInstanceID:      request.Plan.TeamInstanceID(),
			PlanDigest:          request.Plan.Digest(),
			LogicalNodeID:       logicalNodeID,
			AttemptNumber:       attemptNumber,
			MaxAttempts:         planNode.MaxAttempts(),
			AgentInstanceID:     generation.AgentInstanceID,
			RuntimeInstanceID:   generation.RuntimeInstanceID,
			EvidenceID:          receipt.EvidenceID(),
			EvidenceDigest:      receipt.Digest(),
			OutputSummaryDigest: receipt.OutputSummary().Digest(),
			Classification:      classification,
			RecoveryPolicy:      newTeamRecoveryPolicyPort(semantics.RecoveryPolicy),
			CorrelationID:       request.CorrelationID,
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
	verifierBinding, verifierProfile, verifierInstance, err :=
		appFreezeVerifierExecution(semantics)
	if err != nil {
		return verification.VerifierCandidate{},
			evidence.AttemptReceipt{},
			err
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
				semantics.VerifierAgentInstanceID ||
			!projectedRun.ExecutionBindingAvailable ||
			projectedRun.ExecutionBinding.BindingDigest !=
				verifierBinding.BindingDigest {
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
		_, _, err = coordinator.workAuthority.CreateAndAssign(
			ctx,
			work.WorkItemAssignmentInput{
				WorkItemID:       workItemID,
				Title:            "Independent verification",
				RunID:            runID,
				AgentInstanceID:  semantics.VerifierAgentInstanceID,
				ExecutionBinding: verifierBinding,
				CorrelationID:    request.CorrelationID,
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
			LogicalNodeID:        verifierLogicalNodeID,
			AttemptNumber:        1,
			WorkflowPath:         semantics.VerifierWorkflowPath,
			SourcePath:           execution.SourcePath,
			SourceSnapshotDigest: execution.SourceSnapshotDigest,
			Profile:              verifierProfile,
			Instance:             verifierInstance,
			Dispatch:             dispatch,
			Executor:             execution.Executor,
			executionBinding:     verifierBinding,
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
		execution, executionOK := executions[appExecutionKey(
			node.LogicalNodeID,
			attempt.AttemptNumber,
		)]
		if !executionOK || !attempt.ExecutionBindingAvailable ||
			execution.executionBinding.BindingDigest == "" ||
			execution.executionBinding.BindingDigest !=
				attempt.ExecutionBinding.BindingDigest {
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
		frozen := attempt.ExecutionBinding()
		routeSegment := attempt.RouteSegmentBinding()
		if frozen.BindingDigest == "" ||
			frozen.BindingDigest != execution.executionBinding.BindingDigest ||
			routeSegment != execution.routeSegment {
			return nil, ErrInvalidTeamCoordinator
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
					SourcePath:           task.execution.SourcePath,
					ExpectedSourceDigest: task.execution.SourceSnapshotDigest,
					Profile:              task.execution.Profile,
					Instance:             task.execution.Instance,
					Generation:           task.generation,
					Grant:                task.grant,
					Dispatch:             task.execution.Dispatch,
					FrameObserver:        task.collector,
					ContextCapsule:       task.execution.ContextCapsule.AuthorityRecord(),
					RouteSegment:         task.execution.routeSegment,
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

type RecoveredTeamFrameObserverInput struct {
	LogicalNodeID string
	AttemptNumber int
	Observer      NodeOutputObserver
	EvidenceStore *evidence.Store
	EvidenceID    string
}

// NewRecoveredTeamFrameObserver restores the same evidence and tentative Team
// output path used by an ordinary managed execution. It grants no execution
// authority and accepts no Run identity from the caller.
func NewRecoveredTeamFrameObserver(
	input RecoveredTeamFrameObserverInput,
) (supervisor.AuthorizedFrameObserver, error) {
	if input.LogicalNodeID == "" || input.AttemptNumber <= 0 ||
		nilAppInterface(input.Observer) || input.EvidenceStore == nil ||
		input.EvidenceID == "" {
		return nil, ErrInvalidTeamCoordinator
	}
	return &teamFrameCollector{
		logicalNodeID: input.LogicalNodeID,
		attemptNumber: input.AttemptNumber,
		observer:      input.Observer, evidenceStore: input.EvidenceStore,
		evidenceID: input.EvidenceID,
	}, nil
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
	initialBlocks, initialBlockErr := teams.ValidateInitialExecutionBlocks(
		request.Plan, request.InitialBlocks,
	)
	routeSummaries, routeErr := appValidateRouteSummaries(request.Plan, request.RouteSummaries)
	if len(nodes) == 0 || len(nodes) > teams.MaxExecutionNodeCount ||
		len(request.Nodes) == 0 ||
		len(request.Nodes) > teams.MaxExecutionNodeCount ||
		len(request.Semantics) != len(nodes) ||
		request.AuthoritativeTime.IsZero() ||
		request.AuthoritativeTime.Location() != time.UTC ||
		request.PrepareLeaseDuration <= 0 ||
		request.PrepareLeaseDuration > 5*time.Minute ||
		request.GrantLifetime <= 0 ||
		request.GrantLifetime > time.Hour ||
		request.CorrelationID == "" ||
		!validAppAssetSourceStreamIDs(request.AssetSourceStreamIDs) ||
		initialBlockErr != nil || len(initialBlocks) != len(request.InitialBlocks) || routeErr != nil ||
		request.OutputObserver != nil &&
			nilAppInterface(request.OutputObserver) ||
		request.ContextCapsules != nil && nilAppInterface(request.ContextCapsules) {
		return nil, ErrInvalidTeamCoordinator
	}
	request.InitialBlocks = initialBlocks
	request.RouteSummaries = routeSummaries
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
			semantics.FallbackApproval.Digest() != "" &&
				(!semantics.FallbackApproval.Valid() ||
					semantics.RecoveryPolicy.WorkflowFallbackKey() == "" ||
					semantics.FallbackApproval.ApprovedAt().After(
						request.AuthoritativeTime,
					)) ||
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
		if node.Kind() == teams.ExecutionNodeAggregation {
			if execution.Aggregation == nil ||
				execution.Aggregation.MaxSourceArtifactBytes <= 0 ||
				execution.Aggregation.MaxSourceArtifactBytes > maxTeamAggregationSourceBytes {
				return nil, ErrInvalidTeamCoordinator
			}
		} else if execution.Aggregation != nil {
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
		profile, profileErr := loomruntime.NewRuntimeProfile(execution.Profile)
		instance, instanceErr := loomruntime.NewRuntimeInstance(execution.Instance)
		binding, bindingErr := loomruntime.FreezeExecutionBinding(profile, instance)
		capsuleRecord := execution.ContextCapsule.AuthorityRecord()
		_, capsuleErr := contextcapsule.ValidateAuthorityRecord(capsuleRecord)
		_, contextDispatchErr := contextcapsule.ValidateDispatchPayload(
			execution.ContextCapsule,
			execution.Dispatch.Payload(),
		)
		contextSourceDigest, contextSourceAvailable, contextSourceErr := appContextSourceSnapshotDigest(
			execution.ContextCapsule,
		)
		contextSourceMatches := !contextSourceAvailable && execution.SourceSnapshotDigest == "" ||
			contextSourceAvailable && execution.SourceSnapshotDigest == contextSourceDigest
		routeSegment, routeSegmentErr := work.BuildTeamRouteSegmentBinding(
			work.TeamAttemptSelection{
				LogicalNodeID:    execution.LogicalNodeID,
				AttemptNumber:    execution.AttemptNumber,
				ExecutionBinding: binding, ContextCapsule: execution.ContextCapsule,
			},
		)
		if profileErr != nil || instanceErr != nil || bindingErr != nil ||
			capsuleErr != nil || contextDispatchErr != nil || contextSourceErr != nil ||
			routeSegmentErr != nil ||
			!contextSourceMatches ||
			capsuleRecord.TeamID != request.Plan.TeamInstanceID() ||
			capsuleRecord.AgentID != node.AgentInstanceID() ||
			capsuleRecord.RoleID != execution.LogicalNodeID ||
			capsuleRecord.ProviderID != binding.ProviderID ||
			capsuleRecord.ProviderAccountID != binding.ProviderAccountID ||
			capsuleRecord.ModelID != binding.ModelID ||
			capsuleRecord.AuthMode != string(binding.AuthMode) ||
			execution.AttemptNumber == 1 &&
				binding.RuntimeInstanceID != node.RuntimeInstanceID() ||
			execution.Dispatch.RuntimeInstanceID() != binding.RuntimeInstanceID {
			return nil, errors.Join(
				ErrInvalidTeamCoordinator,
				profileErr,
				instanceErr,
				bindingErr,
				contextDispatchErr,
				contextSourceErr,
				routeSegmentErr,
			)
		}
		execution.Profile = appRuntimeProfileFromFrozenBinding(binding)
		execution.Instance = instance
		execution.executionBinding = binding
		execution.routeSegment = routeSegment
		executions[key] = execution
	}
	for _, node := range nodes {
		primary, ok := executions[appExecutionKey(node.LogicalNodeID(), 1)]
		if !ok {
			return nil, ErrTeamExecutionIncomplete
		}
		if route, ok := appRouteSummaryByNode(request.RouteSummaries, node.LogicalNodeID()); ok &&
			!appRouteSummaryMatchesBinding(route, primary.executionBinding) {
			return nil, ErrInvalidTeamCoordinator
		}
		semantics := semanticNodes[node.LogicalNodeID()]
		fallbackSeen := false
		for attemptNumber := 1; attemptNumber <= node.MaxAttempts(); attemptNumber++ {
			execution, exists := executions[appExecutionKey(
				node.LogicalNodeID(),
				attemptNumber,
			)]
			if !exists {
				return nil, ErrTeamExecutionIncomplete
			}
			digest := execution.executionBinding.BindingDigest
			if !fallbackSeen && digest == primary.executionBinding.BindingDigest {
				continue
			}
			approval := semantics.FallbackApproval
			if !approval.Valid() ||
				approval.SourceBindingDigest() !=
					primary.executionBinding.BindingDigest ||
				approval.TargetBindingDigest() != digest ||
				execution.WorkflowPath !=
					semantics.RecoveryPolicy.WorkflowFallbackKey() {
				return nil, ErrInvalidTeamCoordinator
			}
			fallbackSeen = true
		}
	}
	if err := appValidateParallelRouteBindings(request.Plan, executions); err != nil {
		return nil, err
	}
	return executions, nil
}

func appValidateParallelRouteBindings(
	plan teams.ExecutionPlan,
	executions map[string]TeamNodeExecution,
) error {
	groupBindings := make(map[string]map[string]struct{})
	for _, node := range plan.Nodes() {
		if node.Kind() != teams.ExecutionNodeRouteSibling {
			continue
		}
		execution, ok := executions[appExecutionKey(node.LogicalNodeID(), 1)]
		if !ok || execution.executionBinding.BindingDigest == "" {
			return ErrTeamExecutionIncomplete
		}
		bindings := groupBindings[node.RouteGroupID()]
		if bindings == nil {
			bindings = make(map[string]struct{})
			groupBindings[node.RouteGroupID()] = bindings
		}
		if _, duplicate := bindings[execution.executionBinding.BindingDigest]; duplicate {
			return ErrInvalidTeamCoordinator
		}
		bindings[execution.executionBinding.BindingDigest] = struct{}{}
	}
	return nil
}

func appContextSourceSnapshotDigest(
	capsule contextcapsule.RoleContextCapsule,
) (string, bool, error) {
	digest := ""
	for _, item := range capsule.Disclosed() {
		if item.Kind != contextcapsule.KindWorkspaceSnapshot {
			continue
		}
		if digest != "" || item.ItemID != "workspace-snapshot" ||
			item.Trust != contextcapsule.TrustObserved ||
			item.Scope != contextcapsule.ScopeTeamShared ||
			item.SourceType != contextcapsule.SourceObservation {
			return "", false, ErrInvalidTeamCoordinator
		}
		var snapshot struct {
			SchemaVersion  int    `json:"schema_version"`
			SnapshotKind   string `json:"snapshot_kind"`
			TreeDigest     string `json:"tree_digest"`
			EntryCount     int    `json:"entry_count"`
			FileCount      int    `json:"file_count"`
			DirectoryCount int    `json:"directory_count"`
			TotalBytes     int64  `json:"total_bytes"`
		}
		decoder := json.NewDecoder(bytes.NewReader(item.Content))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&snapshot) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
			snapshot.SchemaVersion != 1 ||
			snapshot.SnapshotKind != "managed_source_baseline" ||
			snapshot.EntryCount < 0 || snapshot.FileCount < 0 ||
			snapshot.DirectoryCount < 0 || snapshot.TotalBytes < 0 ||
			snapshot.FileCount+snapshot.DirectoryCount != snapshot.EntryCount ||
			len(snapshot.TreeDigest) != sha256.Size*2 ||
			item.SourceRef != "managed-source:"+snapshot.TreeDigest {
			return "", false, ErrInvalidTeamCoordinator
		}
		if _, err := hex.DecodeString(snapshot.TreeDigest); err != nil {
			return "", false, ErrInvalidTeamCoordinator
		}
		digest = snapshot.TreeDigest
	}
	if digest == "" {
		return "", false, nil
	}
	return digest, true, nil
}

func appRuntimeProfileFromFrozenBinding(
	binding loomruntime.FrozenExecutionBinding,
) loomruntime.RuntimeProfile {
	profile := loomruntime.RuntimeProfile{
		ID:                         binding.ProfileID,
		AdapterType:                binding.HarnessAdapter,
		ProviderID:                 binding.ProviderID,
		ProviderAccountID:          binding.ProviderAccountID,
		ModelID:                    binding.ModelID,
		AuthMode:                   binding.AuthMode,
		EndpointFingerprint:        binding.EndpointFingerprint,
		CredentialReference:        binding.CredentialReference,
		CredentialRevision:         binding.CredentialRevision,
		ReasoningEffort:            binding.ReasoningEffort,
		RequiredCapabilities:       append([]string(nil), binding.Capabilities...),
		Timeout:                    binding.Timeout,
		RemoteToolEnrollmentID:     binding.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: binding.RemoteToolEnrollmentDigest,
	}
	if binding.Budget != nil {
		budget := *binding.Budget
		profile.Budget = &budget
	}
	return profile
}

func validAppAssetSourceStreamIDs(values []string) bool {
	if len(values) > 16 {
		return false
	}
	for index, value := range values {
		if value == "" || len(value) > 256 || strings.ContainsAny(value, "\r\n\x00") ||
			index > 0 && values[index-1] >= value {
			return false
		}
		if !strings.HasPrefix(value, "team-definition/") &&
			!strings.HasPrefix(value, "evolution-asset-binding/") {
			return false
		}
	}
	return true
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
		planNode := appPlanNode(request.Plan, semantics.LogicalNodeID)
		result = append(result, work.TeamNodeSemanticBinding{
			LogicalNodeID:            semantics.LogicalNodeID,
			OutputContractVersion:    semantics.OutputContract.Version(),
			OutputContractDigest:     semantics.OutputContract.Digest(),
			RecoveryPolicyVersion:    semantics.RecoveryPolicy.Version(),
			RecoveryPolicyDigest:     semantics.RecoveryPolicy.Digest(),
			AttemptCredits:           semantics.RecoveryPolicy.AttemptCredits(),
			PrimaryWorkflowPath:      semantics.PrimaryWorkflowPath,
			WorkflowFallbackKey:      semantics.RecoveryPolicy.WorkflowFallbackKey(),
			RecoveryApprovalRequired: semantics.RecoveryPolicy.RecoveryApprovalRequired(),
			FallbackApproval:         semantics.FallbackApproval,
			FallbackRuntimeInstanceID: appFallbackRuntimeInstanceID(
				request, semantics,
			),
			AcceptanceContractVersion:   semantics.AcceptanceContract.Version(),
			AcceptanceContractDigest:    semantics.AcceptanceContract.Digest(),
			AcceptanceRisk:              string(semantics.AcceptanceContract.Risk()),
			IndependentVerifierRequired: semantics.AcceptanceContract.IndependentVerifierRequired(),
			VerifierAgentInstanceID:     semantics.VerifierAgentInstanceID,
			VerifierRuntimeInstanceID:   semantics.VerifierRuntimeInstanceID,
			VerifierWorkflowPath:        semantics.VerifierWorkflowPath,
			AssetRevisionBindings:       planNode.AssetRevisionBindings(),
			AssetRevisionSetDigest:      planNode.AssetRevisionSetDigest(),
		})
	}
	return result
}

func appFallbackRuntimeInstanceID(
	request TeamExecutionRequest,
	semantics TeamNodeSemantics,
) string {
	if !semantics.FallbackApproval.Valid() ||
		semantics.RecoveryPolicy.WorkflowFallbackKey() == "" {
		return ""
	}
	for _, execution := range request.Nodes {
		if execution.LogicalNodeID == semantics.LogicalNodeID &&
			execution.AttemptNumber > 1 &&
			execution.WorkflowPath == semantics.RecoveryPolicy.WorkflowFallbackKey() {
			return execution.Instance.ID
		}
	}
	return ""
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
	_, _, _, bindingErr := appFreezeVerifierExecution(semantics)
	return validAppWorkflowPath(semantics.VerifierAgentInstanceID) &&
		semantics.VerifierAgentInstanceID != sourceAgentInstanceID &&
		validAppWorkflowPath(semantics.VerifierRuntimeInstanceID) &&
		validAppWorkflowPath(semantics.VerifierWorkflowPath) &&
		semantics.VerifierExecution != nil &&
		semantics.VerifierExecution.SourcePath != "" &&
		!nilAppInterface(semantics.VerifierExecution.Executor) &&
		semantics.VerifierExecution.Instance.ID ==
			semantics.VerifierRuntimeInstanceID &&
		bindingErr == nil
}

func appFreezeVerifierExecution(
	semantics TeamNodeSemantics,
) (
	loomruntime.FrozenExecutionBinding,
	loomruntime.RuntimeProfile,
	loomruntime.RuntimeInstance,
	error,
) {
	if semantics.VerifierExecution == nil {
		return loomruntime.FrozenExecutionBinding{},
			loomruntime.RuntimeProfile{},
			loomruntime.RuntimeInstance{},
			ErrInvalidTeamCoordinator
	}
	profile, profileErr := loomruntime.NewRuntimeProfile(
		semantics.VerifierExecution.Profile,
	)
	instance, instanceErr := loomruntime.NewRuntimeInstance(
		semantics.VerifierExecution.Instance,
	)
	binding, bindingErr := loomruntime.FreezeExecutionBinding(profile, instance)
	if profileErr != nil || instanceErr != nil || bindingErr != nil ||
		binding.RuntimeInstanceID != semantics.VerifierRuntimeInstanceID {
		return loomruntime.FrozenExecutionBinding{},
			loomruntime.RuntimeProfile{},
			loomruntime.RuntimeInstance{},
			errors.Join(
				ErrInvalidTeamCoordinator,
				profileErr,
				instanceErr,
				bindingErr,
			)
	}
	return binding, appRuntimeProfileFromFrozenBinding(binding), instance, nil
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
	request TeamExecutionRequest,
	view projection.GlobalReadView,
) []teams.ExecutionNodeState {
	plan := request.Plan
	projected, exists := view.TeamExecution(plan.TeamInstanceID())
	if !exists {
		return teams.InitialExecutionNodeStates(plan, request.InitialBlocks)
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

func appInitialDispatchBlocks(
	request TeamExecutionRequest,
	view projection.GlobalReadView,
) []teams.InitialExecutionBlock {
	if _, exists := view.TeamExecution(request.Plan.TeamInstanceID()); exists {
		return nil
	}
	return append([]teams.InitialExecutionBlock(nil), request.InitialBlocks...)
}

func appInitialDispatchRoutes(
	request TeamExecutionRequest,
	view projection.GlobalReadView,
) []work.TeamNodeRouteSummary {
	if _, exists := view.TeamExecution(request.Plan.TeamInstanceID()); exists {
		return nil
	}
	return cloneAppRouteSummaries(request.RouteSummaries)
}

func appValidateRouteSummaries(
	plan teams.ExecutionPlan,
	values []work.TeamNodeRouteSummary,
) ([]work.TeamNodeRouteSummary, error) {
	if len(values) == 0 {
		return []work.TeamNodeRouteSummary{}, nil
	}
	if len(values) != len(plan.Nodes()) {
		return nil, ErrInvalidTeamCoordinator
	}
	routes := cloneAppRouteSummaries(values)
	sort.Slice(routes, func(i, j int) bool { return routes[i].LogicalNodeID < routes[j].LogicalNodeID })
	for index, route := range routes {
		node := appPlanNode(plan, route.LogicalNodeID)
		if node.LogicalNodeID() == "" || index > 0 && route.LogicalNodeID == routes[index-1].LogicalNodeID ||
			!validAppWorkflowPath(route.HarnessAdapter) || !validAppWorkflowPath(route.ProviderID) ||
			!validAppRouteCredentialIdentity(route) || !validAppWorkflowPath(route.ModelID) ||
			route.ReasoningEffort != "" && !validAppWorkflowPath(route.ReasoningEffort) ||
			route.TimeoutNanoseconds <= 0 ||
			route.Budget != nil && *route.Budget < 0 {
			return nil, ErrInvalidTeamCoordinator
		}
		for capabilityIndex, capability := range route.Capabilities {
			if !validAppWorkflowPath(capability) ||
				capabilityIndex > 0 && route.Capabilities[capabilityIndex-1] >= capability {
				return nil, ErrInvalidTeamCoordinator
			}
		}
	}
	return routes, nil
}

func validAppRouteCredentialIdentity(route work.TeamNodeRouteSummary) bool {
	if route.ProviderAccountID == "" {
		return route.CredentialRevision == 0
	}
	return validAppWorkflowPath(route.ProviderAccountID) && route.CredentialRevision > 0
}

func appRouteSummaryByNode(
	routes []work.TeamNodeRouteSummary,
	logicalNodeID string,
) (work.TeamNodeRouteSummary, bool) {
	for _, route := range routes {
		if route.LogicalNodeID == logicalNodeID {
			return route, true
		}
	}
	return work.TeamNodeRouteSummary{}, false
}

func appRouteSummaryMatchesBinding(
	route work.TeamNodeRouteSummary,
	binding loomruntime.FrozenExecutionBinding,
) bool {
	return route.HarnessAdapter == binding.HarnessAdapter &&
		route.ProviderID == binding.ProviderID &&
		route.ProviderAccountID == binding.ProviderAccountID &&
		route.ModelID == binding.ModelID &&
		route.ReasoningEffort == binding.ReasoningEffort &&
		route.TimeoutNanoseconds == int64(binding.Timeout) &&
		route.CredentialRevision == binding.CredentialRevision &&
		reflect.DeepEqual(route.Budget, binding.Budget) &&
		reflect.DeepEqual(route.Capabilities, binding.Capabilities)
}

func cloneAppRouteSummaries(values []work.TeamNodeRouteSummary) []work.TeamNodeRouteSummary {
	result := make([]work.TeamNodeRouteSummary, len(values))
	for index, value := range values {
		result[index] = value
		result[index].Capabilities = append([]string(nil), value.Capabilities...)
		if value.Budget != nil {
			budget := *value.Budget
			result[index].Budget = &budget
		}
	}
	return result
}

func appReadyEvaluationTime(
	request TeamExecutionRequest,
	states []teams.ExecutionNodeState,
) time.Time {
	readyTime := request.AuthoritativeTime
	for _, state := range states {
		if state.Status != "retry_scheduled" &&
			state.Status != "fallback_scheduled" {
			continue
		}
		semantics, ok := appNodeSemantics(request, state.LogicalNodeID)
		if !ok || semantics.RecoveryPolicy.RetryDelay() != 0 ||
			state.RetryAt.IsZero() || !state.RetryAt.After(readyTime) {
			continue
		}
		readyTime = state.RetryAt
	}
	return readyTime
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
			LogicalNodeID:          node.LogicalNodeID(),
			Title:                  node.Title(),
			AgentInstanceID:        agentInstanceID,
			RuntimeInstanceID:      runtimeInstanceID,
			Role:                   node.Role(),
			Kind:                   node.Kind(),
			RouteGroupID:           node.RouteGroupID(),
			DependsOn:              node.DependsOn(),
			MaxAttempts:            node.MaxAttempts(),
			AssetRevisionBindings:  node.AssetRevisionBindings(),
			AssetRevisionSetDigest: node.AssetRevisionSetDigest(),
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
	states []teams.ExecutionNodeState,
	view projection.GlobalReadView,
) ([]teams.RuntimeCapacityState, error) {
	stateByNode := make(map[string]teams.ExecutionNodeState, len(states))
	for _, state := range states {
		stateByNode[state.LogicalNodeID] = state
	}
	seen := make(map[string]struct{})
	capacities := make([]teams.RuntimeCapacityState, 0, len(plan.Nodes()))
	for _, node := range plan.Nodes() {
		state, exists := stateByNode[node.LogicalNodeID()]
		if !exists || state.Status != "pending" &&
			state.Status != "retry_scheduled" &&
			state.Status != "fallback_scheduled" {
			continue
		}
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
	materializationSets ...[]work.TeamAttemptMaterialization,
) []journal.StreamHead {
	return appDispatchHeadsWithSources(
		view, plan, selections, nodes, nil, materializationSets...,
	)
}

func appDispatchHeadsWithSources(
	view projection.GlobalReadView,
	plan teams.ExecutionPlan,
	selections []work.TeamAttemptSelection,
	nodes []teams.ExecutionNode,
	assetSourceStreamIDs []string,
	materializationSets ...[]work.TeamAttemptMaterialization,
) []journal.StreamHead {
	var materializations []work.TeamAttemptMaterialization
	if len(materializationSets) == 1 {
		materializations = materializationSets[0]
	}
	streams := map[string]struct{}{
		"team-execution/" + plan.TeamInstanceID(): {},
		"work-run-identity/v1":                    {},
	}
	for _, streamID := range assetSourceStreamIDs {
		streams[streamID] = struct{}{}
	}
	for _, selection := range selections {
		if selection.ExecutionBinding.ProviderAccountID != "" {
			accountID := selection.ExecutionBinding.ProviderAccountID
			streams["provider-account-policy/"+accountID] = struct{}{}
			streams["provider-account-capacity/"+accountID] = struct{}{}
			if rateCardStreamID, err := work.ProviderModelRateCardStreamID(
				selection.ExecutionBinding.ProviderID,
				accountID,
				selection.ExecutionBinding.ModelID,
			); err == nil {
				streams[rateCardStreamID] = struct{}{}
			}
		}
		var runtimeInstanceID string
		for _, node := range nodes {
			if node.LogicalNodeID() == selection.LogicalNodeID {
				runtimeInstanceID = node.RuntimeInstanceID()
				for _, binding := range node.AssetRevisionBindings() {
					streams["evolution-asset-revision/"+string(binding.AssetKind)+"/"+
						binding.DefinitionID+"/"+binding.RevisionID] = struct{}{}
					streams["evolution-asset-activation/"+string(binding.AssetKind)+"/"+
						binding.DefinitionID] = struct{}{}
				}
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
	for _, materialization := range materializations {
		for _, head := range materialization.Prepared.Heads() {
			streams[head.StreamID] = struct{}{}
		}
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

func (coordinator *TeamCoordinator) prepareTeamAssetMaterializations(
	ctx context.Context,
	request TeamExecutionRequest,
	view projection.GlobalReadView,
	ready []teams.ExecutionNode,
	selections []work.TeamAttemptSelection,
	executions []TeamNodeExecution,
) ([]TeamNodeExecution, []work.TeamAttemptMaterialization, []TeamAssetMaterialization, error) {
	updated := append([]TeamNodeExecution(nil), executions...)
	lineage := make([]work.TeamAttemptMaterialization, 0, len(ready))
	prepared := make([]TeamAssetMaterialization, 0, len(ready))
	cleanup := func() {
		coordinator.cleanupTeamAssetMaterializations(ctx, prepared)
	}
	for index, node := range ready {
		bindings := node.AssetRevisionBindings()
		if len(bindings) == 0 {
			continue
		}
		if coordinator.assetMaterializer == nil || index >= len(selections) ||
			index >= len(updated) {
			cleanup()
			return nil, nil, nil, ErrTeamExecutionIncomplete
		}
		selection := selections[index]
		execution := updated[index]
		if selection.LogicalNodeID != node.LogicalNodeID() ||
			execution.LogicalNodeID != node.LogicalNodeID() ||
			execution.AttemptNumber != selection.AttemptNumber {
			cleanup()
			return nil, nil, nil, ErrInvalidTeamCoordinator
		}
		runID := appTeamAttemptIdentity(
			"run", request.Plan, selection.LogicalNodeID, selection.AttemptNumber,
		)
		result, err := coordinator.assetMaterializer.PrepareTeamAttemptMaterialization(
			ctx,
			TeamAssetMaterializationRequest{
				TeamExecutionID: request.Plan.TeamInstanceID(),
				LogicalNodeID:   selection.LogicalNodeID,
				RunID:           runID, AttemptNumber: selection.AttemptNumber,
				Generation: 1, JourneyID: request.CorrelationID,
				ExpectedView: view.Version(), Profile: execution.Profile,
				Instance:          execution.Instance,
				Bindings:          append([]assets.ExactAssetRevisionBinding(nil), bindings...),
				RevisionSetDigest: node.AssetRevisionSetDigest(),
			},
		)
		if err != nil {
			cleanup()
			return nil, nil, nil, err
		}
		attemptLineage := result.AttemptLineage
		if result.SourcePath == "" ||
			attemptLineage.LogicalNodeID != selection.LogicalNodeID ||
			attemptLineage.AttemptNumber != selection.AttemptNumber ||
			attemptLineage.AssetRevisionSetDigest != node.AssetRevisionSetDigest() ||
			!equalAppAssetBindings(attemptLineage.AssetRevisionBindings, bindings) {
			prepared = append(prepared, result)
			cleanup()
			return nil, nil, nil, ErrInvalidTeamCoordinator
		}
		baseline, observeErr := supervisor.ObserveSourceSnapshot(execution.SourcePath)
		if observeErr != nil || baseline.TreeDigest() != execution.SourceSnapshotDigest {
			prepared = append(prepared, result)
			cleanup()
			return nil, nil, nil, errors.Join(ErrInvalidTeamCoordinator, observeErr)
		}
		execution.SourcePath = result.SourcePath
		execution.SourceSnapshotDigest = ""
		updated[index] = execution
		lineage = append(lineage, attemptLineage)
		prepared = append(prepared, result)
	}
	return updated, lineage, prepared, nil
}

func (coordinator *TeamCoordinator) cleanupTeamAssetMaterializations(
	ctx context.Context,
	prepared []TeamAssetMaterialization,
) {
	if coordinator == nil || coordinator.assetMaterializer == nil {
		return
	}
	for index := len(prepared) - 1; index >= 0; index-- {
		if prepared[index].Authoritative {
			continue
		}
		_ = coordinator.assetMaterializer.CleanupTeamAttemptMaterialization(
			ctx, prepared[index],
		)
	}
}

func (coordinator *TeamCoordinator) cleanupCommittedTeamAssetMaterializations(
	ctx context.Context,
	prepared []TeamAssetMaterialization,
) error {
	if coordinator == nil || coordinator.assetMaterializer == nil {
		if len(prepared) == 0 {
			return nil
		}
		return ErrInvalidTeamCoordinator
	}
	for index := len(prepared) - 1; index >= 0; index-- {
		if err := coordinator.assetMaterializer.CleanupTeamAttemptMaterialization(
			ctx, prepared[index],
		); err != nil {
			return err
		}
	}
	return nil
}

func (coordinator *TeamCoordinator) verifyDispatchedAssetLineage(
	teamInstanceID string,
	materializations []work.TeamAttemptMaterialization,
) error {
	if len(materializations) == 0 {
		return nil
	}
	view := coordinator.projection.GlobalReadView()
	team, ok := view.TeamExecution(teamInstanceID)
	if !ok {
		return ErrTeamExecutionIncomplete
	}
	for _, expected := range materializations {
		found := false
		for _, node := range team.Nodes {
			if node.LogicalNodeID != expected.LogicalNodeID {
				continue
			}
			for _, attempt := range node.Attempts {
				if attempt.AttemptNumber == expected.AttemptNumber &&
					attempt.AssetLineageAvailable &&
					attempt.AssetRevisionSetDigest == expected.AssetRevisionSetDigest &&
					attempt.MaterializationManifestDigest == expected.MaterializationManifestDigest &&
					attempt.MaterializationRootDigest == expected.MaterializationRootDigest &&
					equalAppAssetBindings(attempt.AssetRevisionBindings, expected.AssetRevisionBindings) {
					found = true
				}
			}
		}
		if !found {
			return ErrTeamExecutionIncomplete
		}
	}
	return nil
}

func equalAppAssetBindings(
	left []assets.ExactAssetRevisionBinding,
	right []assets.ExactAssetRevisionBinding,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
