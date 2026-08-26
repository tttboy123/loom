package app

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/verification"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const maxTeamAggregationSourceBytes = int64(1 << 20)

type TeamAggregationExecution struct {
	MaxSourceArtifactBytes int64
	CapacityAuthority      contextcapsule.CapacityAuthority
	TokenCounter           contextcapsule.TokenCounter
}

type TeamAggregationSource struct {
	LogicalNodeID              string
	AttemptNumber              int
	WorkItemID                 string
	RunID                      string
	ClaimID                    string
	ClaimGeneration            int64
	RuntimeInstanceID          string
	AgentInstanceID            string
	EvidenceID                 string
	EvidenceDigest             string
	OutputSummaryDigest        string
	TerminalStatus             string
	OutputContractVersion      int
	OutputContractDigest       string
	OutputClassification       string
	OutputClassificationDigest string
	AcceptanceDecisionKind     string
	AcceptanceDecisionDigest   string
	AcceptanceDecisionTime     time.Time
	GovernedTestReports        []verification.GovernedTestReport
	Content                    []byte
}

type teamAggregationSourceAuthority struct {
	SchemaVersion       int    `json:"schema_version"`
	Scope               string `json:"scope"`
	LogicalNodeID       string `json:"logical_node_id"`
	AttemptNumber       int    `json:"attempt_number"`
	WorkItemID          string `json:"work_item_id"`
	RunID               string `json:"run_id"`
	ClaimID             string `json:"claim_id"`
	ClaimGeneration     int64  `json:"claim_generation"`
	RuntimeInstanceID   string `json:"runtime_instance_id"`
	AgentInstanceID     string `json:"agent_instance_id"`
	EvidenceID          string `json:"evidence_id"`
	EvidenceDigest      string `json:"evidence_digest"`
	OutputSummaryDigest string `json:"output_summary_digest"`
}

type teamObservedExecutionState struct {
	SchemaVersion              int                      `json:"schema_version"`
	LogicalNodeID              string                   `json:"logical_node_id"`
	AttemptNumber              int                      `json:"attempt_number"`
	TerminalStatus             string                   `json:"terminal_status"`
	OutputClassification       string                   `json:"output_classification"`
	OutputClassificationDigest string                   `json:"output_classification_digest"`
	AcceptanceDecisionKind     string                   `json:"acceptance_decision_kind"`
	AcceptanceDecisionDigest   string                   `json:"acceptance_decision_digest"`
	AcceptedAt                 string                   `json:"accepted_at"`
	TestReportCount            int                      `json:"test_report_count,omitempty"`
	TestReportSetDigest        string                   `json:"test_report_set_digest,omitempty"`
	TestReports                []teamObservedTestReport `json:"test_reports,omitempty"`
}

type teamObservedTestReport struct {
	Runner       string `json:"runner"`
	Scope        string `json:"scope"`
	Outcome      string `json:"outcome"`
	CallSequence int64  `json:"call_sequence"`
	ReportDigest string `json:"report_digest"`
}

func buildTeamAggregationContextCapsule(
	plan teams.ExecutionPlan,
	node teams.ExecutionNode,
	base contextcapsule.RoleContextCapsule,
	sources []TeamAggregationSource,
	capacityAuthority contextcapsule.CapacityAuthority,
	tokenCounter contextcapsule.TokenCounter,
) (contextcapsule.RoleContextCapsule, error) {
	plannedNode, found := missionContextPlanNode(plan, node.LogicalNodeID())
	if !found || !sameTeamAggregationNode(plannedNode, node) ||
		node.Kind() != teams.ExecutionNodeAggregation ||
		node.RouteGroupID() == "" || !base.Valid() ||
		base.Target().TeamID != plan.TeamInstanceID() ||
		base.Target().AgentID != node.AgentInstanceID() ||
		base.Target().RoleID != node.LogicalNodeID() ||
		len(sources) != len(node.DependsOn()) || len(sources) < 2 {
		return contextcapsule.RoleContextCapsule{}, ErrInvalidTeamCoordinator
	}
	items, err := buildTeamSourceContextItems(plan, node, sources, true)
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, err
	}
	capsule, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base, items, capacityAuthority, tokenCounter,
	)
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: %w", ErrInvalidTeamCoordinator, err,
		)
	}
	return capsule, nil
}

func sameTeamDependencyNode(left, right teams.ExecutionNode) bool {
	return left.LogicalNodeID() == right.LogicalNodeID() &&
		left.Title() == right.Title() &&
		left.AgentInstanceID() == right.AgentInstanceID() &&
		left.Role() == right.Role() && left.Kind() == right.Kind() &&
		left.RouteGroupID() == right.RouteGroupID() &&
		left.MaxAttempts() == right.MaxAttempts() &&
		reflect.DeepEqual(left.DependsOn(), right.DependsOn()) &&
		reflect.DeepEqual(left.AssetRevisionBindings(), right.AssetRevisionBindings()) &&
		left.AssetRevisionSetDigest() == right.AssetRevisionSetDigest()
}

func buildTeamDependencyContextCapsule(
	plan teams.ExecutionPlan,
	node teams.ExecutionNode,
	base contextcapsule.RoleContextCapsule,
	sources []TeamAggregationSource,
	capacityAuthority contextcapsule.CapacityAuthority,
	tokenCounter contextcapsule.TokenCounter,
) (contextcapsule.RoleContextCapsule, error) {
	plannedNode, found := missionContextPlanNode(plan, node.LogicalNodeID())
	if !found || !sameTeamDependencyNode(plannedNode, node) {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: dependency plan node mismatch", ErrInvalidTeamCoordinator,
		)
	}
	if node.Kind() == teams.ExecutionNodeAggregation || len(node.DependsOn()) == 0 {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: invalid dependency topology", ErrInvalidTeamCoordinator,
		)
	}
	if !base.Valid() || base.Target().TeamID != plan.TeamInstanceID() ||
		base.Target().AgentID != node.AgentInstanceID() ||
		base.Target().RoleID != node.LogicalNodeID() {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: dependency Capsule target mismatch", ErrInvalidTeamCoordinator,
		)
	}
	if len(sources) != len(node.DependsOn()) {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: dependency source count mismatch", ErrInvalidTeamCoordinator,
		)
	}
	items, err := buildTeamSourceContextItems(plan, node, sources, false)
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, err
	}
	if _, capacityBound := base.CapacityProjection(); !capacityBound {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: dependency Capsule has no capacity authority", ErrInvalidTeamCoordinator,
		)
	}
	capsule, err := contextcapsule.ExtendRoleContextCapsuleWithCapacity(
		base, items, capacityAuthority, tokenCounter,
	)
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, fmt.Errorf(
			"%w: %w", ErrInvalidTeamCoordinator, err,
		)
	}
	return capsule, nil
}

func buildTeamSourceContextItems(
	plan teams.ExecutionPlan,
	node teams.ExecutionNode,
	sources []TeamAggregationSource,
	aggregation bool,
) ([]contextcapsule.ItemInput, error) {
	items := make([]contextcapsule.ItemInput, 0, len(sources)*3)
	ordered := append([]TeamAggregationSource(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].LogicalNodeID < ordered[j].LogicalNodeID })
	dependencies := node.DependsOn()
	for index, source := range ordered {
		testReportSetDigest, reportErr := verification.GovernedTestReportSetDigest(
			source.GovernedTestReports,
		)
		sourceNode, found := missionContextPlanNode(plan, source.LogicalNodeID)
		if reportErr != nil || !found || dependencies[index] != source.LogicalNodeID ||
			aggregation && (sourceNode.Kind() != teams.ExecutionNodeRouteSibling ||
				sourceNode.RouteGroupID() != node.RouteGroupID() ||
				sourceNode.AgentInstanceID() != node.AgentInstanceID()) ||
			source.AttemptNumber < 1 || source.AttemptNumber > sourceNode.MaxAttempts() ||
			source.AgentInstanceID != sourceNode.AgentInstanceID() ||
			!validMissionExecutionText(source.WorkItemID, 128) ||
			!validMissionExecutionText(source.RunID, 128) ||
			!validMissionExecutionUUID(source.ClaimID) || source.ClaimGeneration < 1 ||
			source.RuntimeInstanceID == "" || source.EvidenceID == "" ||
			!validSHA256(source.EvidenceDigest) || !validSHA256(source.OutputSummaryDigest) ||
			source.TerminalStatus != "succeeded" || source.OutputContractVersion < 1 ||
			!validSHA256(source.OutputContractDigest) ||
			(source.OutputClassification != "valid_nonempty" &&
				source.OutputClassification != "valid_empty") ||
			!validSHA256(source.OutputClassificationDigest) ||
			source.AcceptanceDecisionKind != "accepted" ||
			!validSHA256(source.AcceptanceDecisionDigest) ||
			source.AcceptanceDecisionTime.IsZero() ||
			source.AcceptanceDecisionTime.Location() != time.UTC ||
			len(source.Content) == 0 || int64(len(source.Content)) > maxTeamAggregationSourceBytes {
			return nil, ErrInvalidTeamCoordinator
		}
		authorityContent, err := json.Marshal(teamAggregationSourceAuthority{
			SchemaVersion: 1, Scope: evidence.AggregationScopeAuthorizedOutputEvents,
			LogicalNodeID: source.LogicalNodeID, AttemptNumber: source.AttemptNumber,
			WorkItemID: source.WorkItemID, RunID: source.RunID, ClaimID: source.ClaimID,
			ClaimGeneration: source.ClaimGeneration, RuntimeInstanceID: source.RuntimeInstanceID,
			AgentInstanceID: source.AgentInstanceID, EvidenceID: source.EvidenceID,
			EvidenceDigest: source.EvidenceDigest, OutputSummaryDigest: source.OutputSummaryDigest,
		})
		if err != nil {
			return nil, ErrInvalidTeamCoordinator
		}
		testReports := make([]teamObservedTestReport, len(source.GovernedTestReports))
		orderedReports := append(
			[]verification.GovernedTestReport(nil),
			source.GovernedTestReports...,
		)
		sort.Slice(orderedReports, func(i, j int) bool {
			return orderedReports[i].CallSequence() < orderedReports[j].CallSequence()
		})
		for reportIndex, report := range orderedReports {
			testReports[reportIndex] = teamObservedTestReport{
				Runner: string(report.Runner()), Scope: string(report.Scope()),
				Outcome: string(report.Outcome()), CallSequence: report.CallSequence(),
				ReportDigest: report.Digest(),
			}
		}
		observationContent, err := json.Marshal(teamObservedExecutionState{
			SchemaVersion: 1, LogicalNodeID: source.LogicalNodeID,
			AttemptNumber: source.AttemptNumber, TerminalStatus: source.TerminalStatus,
			OutputClassification:       source.OutputClassification,
			OutputClassificationDigest: source.OutputClassificationDigest,
			AcceptanceDecisionKind:     source.AcceptanceDecisionKind,
			AcceptanceDecisionDigest:   source.AcceptanceDecisionDigest,
			AcceptedAt:                 source.AcceptanceDecisionTime.Format(time.RFC3339Nano),
			TestReportCount:            len(testReports),
			TestReportSetDigest:        testReportSetDigest,
			TestReports:                testReports,
		})
		if err != nil {
			return nil, ErrInvalidTeamCoordinator
		}
		prefixLabel := "dependency-source"
		authorityKind := contextcapsule.KindDependencySource
		if aggregation {
			prefixLabel = "aggregation-source"
			authorityKind = contextcapsule.KindAggregationSource
		}
		prefix := fmt.Sprintf("%s-%03d", prefixLabel, index)
		items = append(items,
			contextcapsule.ItemInput{
				ItemID: prefix + "-authority", Kind: authorityKind,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeRoleRestricted,
				Priority:   contextcapsule.PriorityConfirmed,
				TokenCount: missionContextTokenCount(authorityContent), Required: true,
				Content: authorityContent, SourceType: contextcapsule.SourceAuthority,
				SourceRef:     "attempt-summary:" + source.OutputSummaryDigest,
				AllowedRoleID: node.LogicalNodeID(),
			},
			contextcapsule.ItemInput{
				ItemID: prefix + "-observation", Kind: contextcapsule.KindObservedExecutionState,
				Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeRoleRestricted,
				Priority:   contextcapsule.PriorityWorkspace,
				TokenCount: missionContextTokenCount(observationContent), Content: observationContent,
				SourceType:    contextcapsule.SourceObservation,
				SourceRef:     "acceptance:" + source.AcceptanceDecisionDigest,
				AllowedRoleID: node.LogicalNodeID(),
			},
			contextcapsule.ItemInput{
				ItemID: prefix + "-output", Kind: contextcapsule.KindPriorModelOutput,
				Trust: contextcapsule.TrustUntrusted, Scope: contextcapsule.ScopeRoleRestricted,
				Priority:   contextcapsule.PriorityHistory,
				TokenCount: missionContextTokenCount(source.Content), Content: append([]byte(nil), source.Content...),
				SourceType:     contextcapsule.SourceModelOutput,
				SourceRef:      "attempt-output:" + source.EvidenceDigest,
				AllowedRoleID:  node.LogicalNodeID(),
				PolicyFiltered: !contextcapsule.ValidDispatchText(source.Content),
			},
		)
	}
	return items, nil
}

func sameTeamAggregationNode(left, right teams.ExecutionNode) bool {
	return left.LogicalNodeID() == right.LogicalNodeID() &&
		left.Title() == right.Title() &&
		left.AgentInstanceID() == right.AgentInstanceID() &&
		left.RuntimeInstanceID() == right.RuntimeInstanceID() &&
		left.Role() == right.Role() && left.Kind() == right.Kind() &&
		left.RouteGroupID() == right.RouteGroupID() &&
		left.MaxAttempts() == right.MaxAttempts() &&
		reflect.DeepEqual(left.DependsOn(), right.DependsOn()) &&
		reflect.DeepEqual(left.AssetRevisionBindings(), right.AssetRevisionBindings()) &&
		left.AssetRevisionSetDigest() == right.AssetRevisionSetDigest()
}

func (coordinator *TeamCoordinator) prepareAggregationExecution(
	ctx context.Context,
	request TeamExecutionRequest,
	view projection.GlobalReadView,
	node teams.ExecutionNode,
	execution TeamNodeExecution,
) (TeamNodeExecution, error) {
	if coordinator == nil || execution.Aggregation == nil ||
		execution.Aggregation.MaxSourceArtifactBytes <= 0 ||
		execution.Aggregation.MaxSourceArtifactBytes > maxTeamAggregationSourceBytes {
		return TeamNodeExecution{}, fmt.Errorf("%w: invalid aggregation execution", ErrInvalidTeamCoordinator)
	}
	sources, err := coordinator.readTeamDependencySources(
		ctx, request, view, node, execution.Aggregation.MaxSourceArtifactBytes,
	)
	if err != nil {
		return TeamNodeExecution{}, err
	}
	defer zeroTeamDependencySources(sources)
	capsule, err := buildTeamAggregationContextCapsule(
		request.Plan, node, execution.ContextCapsule, sources,
		execution.Aggregation.CapacityAuthority,
		execution.Aggregation.TokenCounter,
	)
	if err != nil {
		return TeamNodeExecution{}, fmt.Errorf(
			"%w: build aggregation Capsule: %w",
			ErrInvalidTeamCoordinator,
			err,
		)
	}
	return coordinator.rebuildTeamContextExecution(ctx, request, execution, capsule)
}

func (coordinator *TeamCoordinator) prepareDependencyContextExecution(
	ctx context.Context,
	request TeamExecutionRequest,
	view projection.GlobalReadView,
	node teams.ExecutionNode,
	execution TeamNodeExecution,
) (TeamNodeExecution, error) {
	sources, err := coordinator.readTeamDependencySources(
		ctx, request, view, node, maxTeamAggregationSourceBytes,
	)
	if err != nil {
		return TeamNodeExecution{}, err
	}
	defer zeroTeamDependencySources(sources)
	capsule, err := buildTeamDependencyContextCapsule(
		request.Plan, node, execution.ContextCapsule, sources,
		execution.ContextCapacityAuthority, execution.ContextTokenCounter,
	)
	if err != nil {
		return TeamNodeExecution{}, fmt.Errorf("%w: build dependency Capsule: %v", ErrInvalidTeamCoordinator, err)
	}
	return coordinator.rebuildTeamContextExecution(ctx, request, execution, capsule)
}

func (coordinator *TeamCoordinator) readTeamDependencySources(
	ctx context.Context,
	request TeamExecutionRequest,
	view projection.GlobalReadView,
	node teams.ExecutionNode,
	maximumBytes int64,
) ([]TeamAggregationSource, error) {
	projected, ok := view.TeamExecution(request.Plan.TeamInstanceID())
	if !ok || projected.PlanDigest != request.Plan.Digest() {
		return nil, fmt.Errorf("%w: dependency Team projection", ErrTeamExecutionIncomplete)
	}
	sources := make([]TeamAggregationSource, 0, len(node.DependsOn()))
	failed := true
	defer func() {
		if failed {
			zeroTeamDependencySources(sources)
		}
	}()
	for _, sourceID := range node.DependsOn() {
		sourceNode := projectedTeamExecutionNode(projected, sourceID)
		if sourceNode == nil || sourceNode.Status != "succeeded" {
			return nil, fmt.Errorf("%w: dependency source %s not succeeded", ErrTeamExecutionIncomplete, sourceID)
		}
		attempt, found := appProjectedCurrentAttempt(*sourceNode)
		if !found || attempt.Status != "succeeded" || attempt.EvidenceID == "" ||
			attempt.EvidenceDigest == "" || attempt.OutputSummaryDigest == "" {
			return nil, fmt.Errorf("%w: dependency source %s has no exact Attempt", ErrTeamExecutionIncomplete, sourceID)
		}
		receipt, found, err := coordinator.evidenceStore.AttemptReceipt(ctx, attempt.EvidenceID)
		if err != nil || !found || receipt.Digest() != attempt.EvidenceDigest ||
			receipt.OutputSummary().Digest() != attempt.OutputSummaryDigest {
			return nil, fmt.Errorf("%w: dependency receipt %s: %v", ErrInvalidTeamCoordinator, sourceID, err)
		}
		output, err := coordinator.evidenceStore.ReadAggregationOutput(ctx, receipt, maximumBytes)
		if err != nil {
			return nil, fmt.Errorf("%w: dependency output %s: %v", ErrInvalidTeamCoordinator, sourceID, err)
		}
		binding := output.Binding()
		if binding.TeamInstanceID != request.Plan.TeamInstanceID() ||
			binding.PlanDigest != request.Plan.Digest() || binding.LogicalNodeID != sourceID ||
			binding.AttemptNumber != attempt.AttemptNumber || binding.WorkItemID != attempt.WorkItemID ||
			binding.RunID != attempt.RunID || binding.ClaimID != attempt.ClaimID ||
			binding.ClaimGeneration != attempt.ClaimGeneration ||
			binding.RuntimeInstanceID != attempt.RuntimeInstanceID ||
			binding.AgentInstanceID != attempt.AgentInstanceID {
			output.Close()
			return nil, fmt.Errorf("%w: dependency lineage %s", ErrInvalidTeamCoordinator, sourceID)
		}
		content := output.Content()
		var testReports []verification.GovernedTestReport
		if coordinator.testReportSource != nil {
			if !attempt.ExecutionBindingAvailable || !attempt.ContextCapsuleAvailable ||
				attempt.ExecutionBinding.BindingDigest == "" ||
				attempt.ContextCapsule.CapsuleDigest == "" ||
				attempt.ContextCapsule.ConversationID == "" || attempt.IncidentID == "" {
				output.Close()
				return nil, fmt.Errorf(
					"%w: dependency report authority %s",
					ErrInvalidTeamCoordinator,
					sourceID,
				)
			}
			testReports, err = coordinator.testReportSource.GovernedTestReportsForAttempt(
				ctx,
				work.AttemptReportQuery{
					TeamInstanceID: request.Plan.TeamInstanceID(),
					Authority: attemptpayload.Authority{
						Scope: attemptpayload.Scope{
							ConversationID: attempt.ContextCapsule.ConversationID,
							WorkItemID:     attempt.WorkItemID, RunID: attempt.RunID,
							ClaimGeneration:        attempt.ClaimGeneration,
							RuntimeInstanceID:      attempt.RuntimeInstanceID,
							ExecutionBindingDigest: attempt.ExecutionBinding.BindingDigest,
							CapsuleDigest:          attempt.ContextCapsule.CapsuleDigest,
						},
						ClaimID: attempt.ClaimID, AgentInstanceID: attempt.AgentInstanceID,
						IncidentID: attempt.IncidentID,
					},
				},
			)
			if err != nil {
				output.Close()
				return nil, fmt.Errorf(
					"%w: dependency test reports %s: %v",
					ErrInvalidTeamCoordinator,
					sourceID,
					err,
				)
			}
		}
		sources = append(sources, TeamAggregationSource{
			LogicalNodeID: sourceID, AttemptNumber: binding.AttemptNumber,
			WorkItemID: binding.WorkItemID, RunID: binding.RunID,
			ClaimID: binding.ClaimID, ClaimGeneration: binding.ClaimGeneration,
			RuntimeInstanceID: binding.RuntimeInstanceID, AgentInstanceID: binding.AgentInstanceID,
			EvidenceID: binding.EvidenceID, EvidenceDigest: output.EvidenceDigest(),
			OutputSummaryDigest:        output.OutputSummaryDigest(),
			TerminalStatus:             attempt.Status,
			OutputContractVersion:      attempt.OutputContractVersion,
			OutputContractDigest:       attempt.OutputContractDigest,
			OutputClassification:       attempt.OutputClassification,
			OutputClassificationDigest: attempt.OutputClassificationDigest,
			AcceptanceDecisionKind:     sourceNode.AcceptanceDecisionKind,
			AcceptanceDecisionDigest:   sourceNode.AcceptanceDecisionDigest,
			AcceptanceDecisionTime:     sourceNode.AcceptanceDecisionTime,
			GovernedTestReports:        testReports,
			Content:                    content,
		})
		output.Close()
	}
	failed = false
	return sources, nil
}

func (coordinator *TeamCoordinator) rebuildTeamContextExecution(
	ctx context.Context,
	request TeamExecutionRequest,
	execution TeamNodeExecution,
	capsule contextcapsule.RoleContextCapsule,
) (TeamNodeExecution, error) {
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		// A prior model output may be valid at rest but disallowed on the
		// provider wire. Rebuild once with the capsule's policy filter; authority
		// and observed items still fail closed in RebuildDispatchSafe.
		if safeCapsule, rebuildErr := contextcapsule.RebuildDispatchSafe(
			capsule, teamContextTokenCounter(execution, capsule),
		); rebuildErr == nil {
			capsule = safeCapsule
			payload, err = contextcapsule.RenderDispatchPayload(capsule)
		}
		if err != nil {
			return TeamNodeExecution{}, fmt.Errorf("%w: render Role Context Capsule: %v", ErrInvalidTeamCoordinator, err)
		}
	}
	if request.ContextCapsules != nil {
		if err := request.ContextCapsules.PutRoleContextCapsule(ctx, capsule, payload); err != nil {
			return TeamNodeExecution{}, fmt.Errorf("%w: persist Role Context Capsule: %v", ErrInvalidTeamCoordinator, err)
		}
	}
	frame := execution.Dispatch
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: frame.MessageID(), CorrelationID: frame.CorrelationID(),
		WorkItemID: frame.WorkItemID(), RunID: frame.RunID(), ClaimGeneration: frame.ClaimGeneration(),
		RuntimeInstanceID: frame.RuntimeInstanceID(), SenderAgentInstanceID: frame.SenderAgentInstanceID(),
		Sequence: frame.Sequence(), Type: frame.Type(), EmittedAt: frame.EmittedAt(), Payload: payload,
	})
	if err != nil || dispatch.Type() != bridgev1.MessageDispatch {
		return TeamNodeExecution{}, fmt.Errorf("%w: rebuild Role Context dispatch", ErrInvalidTeamCoordinator)
	}
	execution.ContextCapsule = capsule
	execution.Dispatch = dispatch
	routeSegment, err := work.BuildTeamRouteSegmentBinding(
		work.TeamAttemptSelection{
			LogicalNodeID:    execution.LogicalNodeID,
			AttemptNumber:    execution.AttemptNumber,
			ExecutionBinding: execution.executionBinding, ContextCapsule: capsule,
		},
	)
	if err != nil {
		return TeamNodeExecution{}, fmt.Errorf("%w: rebuild Role Context Route Segment", ErrInvalidTeamCoordinator)
	}
	execution.routeSegment = routeSegment
	return execution, nil
}

func teamContextTokenCounter(
	execution TeamNodeExecution,
	capsule contextcapsule.RoleContextCapsule,
) contextcapsule.TokenCounter {
	projection, ok := capsule.CapacityProjection()
	if !ok {
		return nil
	}
	for _, counter := range []contextcapsule.TokenCounter{
		execution.ContextTokenCounter,
		func() contextcapsule.TokenCounter {
			if execution.Aggregation == nil {
				return nil
			}
			return execution.Aggregation.TokenCounter
		}(),
	} {
		if !nilAppInterface(counter) && counter.ID() == projection.TokenCounterID &&
			counter.Version() == projection.TokenCounterVersion {
			return counter
		}
	}
	return nil
}

func zeroTeamDependencySources(sources []TeamAggregationSource) {
	for index := range sources {
		zeroTeamAggregationBytes(sources[index].Content)
		sources[index].Content = nil
	}
}

func projectedTeamExecutionNode(
	execution projection.TeamExecution,
	logicalNodeID string,
) *projection.TeamExecutionNode {
	for index := range execution.Nodes {
		if execution.Nodes[index].LogicalNodeID == logicalNodeID {
			return &execution.Nodes[index]
		}
	}
	return nil
}

func zeroTeamAggregationBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
