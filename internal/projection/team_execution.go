package projection

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/teams"
)

const maxProjectedTeamAgentCount = teams.MaxExecutionNodeCount

type projectedFrozenExecutionBinding = loomruntime.FrozenExecutionBinding

type projectedTeamRouteSummary struct {
	LogicalNodeID      string   `json:"logical_node_id"`
	HarnessAdapter     string   `json:"harness_adapter"`
	ProviderID         string   `json:"provider_id"`
	ProviderAccountID  string   `json:"provider_account_id"`
	ModelID            string   `json:"model_id"`
	ReasoningEffort    string   `json:"reasoning_effort,omitempty"`
	TimeoutNanoseconds int64    `json:"timeout_nanoseconds"`
	Budget             *int64   `json:"budget"`
	Capabilities       []string `json:"capabilities"`
	CredentialRevision int64    `json:"credential_revision"`
}

type projectedExecutionBindingPayload struct {
	ProfileID                  string   `json:"profile_id"`
	HarnessAdapter             string   `json:"harness_adapter"`
	RuntimeInstanceID          string   `json:"runtime_instance_id"`
	ProviderID                 string   `json:"provider_id"`
	ProviderAccountID          string   `json:"provider_account_id"`
	ModelID                    string   `json:"model_id"`
	AuthMode                   string   `json:"auth_mode"`
	EndpointFingerprint        string   `json:"endpoint_fingerprint"`
	CredentialReference        string   `json:"credential_reference"`
	CredentialRevision         int64    `json:"credential_revision"`
	ReasoningEffort            string   `json:"reasoning_effort,omitempty"`
	TimeoutNanoseconds         int64    `json:"timeout_nanoseconds"`
	Budget                     *int64   `json:"budget"`
	Capabilities               []string `json:"capabilities"`
	BindingDigest              string   `json:"binding_digest"`
	RemoteToolEnrollmentID     string   `json:"remote_tool_enrollment_id,omitempty"`
	RemoteToolEnrollmentDigest string   `json:"remote_tool_enrollment_digest,omitempty"`
}

type projectedTeamFallbackApprovalPayload struct {
	Version             int    `json:"version"`
	ApprovalID          string `json:"approval_id"`
	ActorRef            string `json:"actor_ref"`
	ApprovedAt          string `json:"approved_at"`
	SourceBindingDigest string `json:"source_binding_digest"`
	TargetBindingDigest string `json:"target_binding_digest"`
	Digest              string `json:"digest"`
}

func projectTeamExecutions(
	events []journal.Event,
) (map[string]TeamExecution, error) {
	byStream := make(map[string][]journal.Event)
	runClaimReferences := make(map[string]teamExecutionRunClaimReference)
	workOutcomeReferences := make(map[string]journal.Event)
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, "team-execution/") {
			byStream[event.StreamID] = append(byStream[event.StreamID], event)
		}
		if strings.HasPrefix(event.StreamID, "run/") &&
			event.Type == "RunClaimed" {
			runClaimReferences[teamExecutionRunReferenceKey(
				event.StreamID,
				event.Seq,
			)] = teamExecutionRunClaimReference{eventID: event.ID}
		}
		if strings.HasPrefix(event.StreamID, "work-item/") &&
			(event.Type == "WorkItemDone" ||
				event.Type == "WorkItemRejected") {
			workOutcomeReferences[event.ID] = event
		}
	}
	projected := make(map[string]TeamExecution, len(byStream))
	for streamID, streamEvents := range byStream {
		sort.Slice(streamEvents, func(i, j int) bool {
			if streamEvents[i].Seq != streamEvents[j].Seq {
				return streamEvents[i].Seq < streamEvents[j].Seq
			}
			return streamEvents[i].ID < streamEvents[j].ID
		})
		teamInstanceID := strings.TrimPrefix(streamID, "team-execution/")
		record, err := projectTeamExecutionStream(
			teamInstanceID,
			streamEvents,
			runClaimReferences,
			workOutcomeReferences,
		)
		if err != nil {
			return nil, err
		}
		projected[teamInstanceID] = record
	}
	return projected, nil
}

func projectTeamExecutionStream(
	teamInstanceID string,
	events []journal.Event,
	runClaimReferences map[string]teamExecutionRunClaimReference,
	workOutcomeReferences map[string]journal.Event,
) (TeamExecution, error) {
	var record TeamExecution
	var sequence int64
	var lastEventID string
	maxAttempts := make(map[string]int)
	for _, event := range events {
		if event.Seq != sequence+1 || event.SchemaVersion != 1 {
			return TeamExecution{}, ErrInvalidProjectionEvent
		}
		sequence = event.Seq
		switch event.Type {
		case "TeamExecutionPlanned":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				ViewVersion    string `json:"view_version"`
				Nodes          []struct {
					LogicalNodeID          string                              `json:"logical_node_id"`
					Title                  string                              `json:"title"`
					AgentInstanceID        string                              `json:"agent_instance_id"`
					RuntimeInstanceID      string                              `json:"runtime_instance_id"`
					Role                   string                              `json:"role"`
					Kind                   teams.ExecutionNodeKind             `json:"kind,omitempty"`
					RouteGroupID           string                              `json:"route_group_id,omitempty"`
					DependsOn              []string                            `json:"depends_on"`
					MaxAttempts            int                                 `json:"max_attempts"`
					AssetRevisionBindings  *[]assets.ExactAssetRevisionBinding `json:"asset_revision_bindings"`
					AssetRevisionSetDigest *string                             `json:"asset_revision_set_digest"`
				} `json:"nodes"`
				SemanticBindings *[]struct {
					LogicalNodeID               string                                `json:"logical_node_id"`
					OutputContractVersion       int                                   `json:"output_contract_version"`
					OutputContractDigest        string                                `json:"output_contract_digest"`
					RecoveryPolicyVersion       int                                   `json:"recovery_policy_version"`
					RecoveryPolicyDigest        string                                `json:"recovery_policy_digest"`
					AttemptCredits              int                                   `json:"attempt_credits"`
					PrimaryWorkflowPath         string                                `json:"primary_workflow_path"`
					WorkflowFallbackKey         string                                `json:"workflow_fallback_key"`
					RecoveryApprovalRequired    bool                                  `json:"recovery_approval_required"`
					FallbackApproval            *projectedTeamFallbackApprovalPayload `json:"fallback_approval,omitempty"`
					FallbackRuntimeInstanceID   string                                `json:"fallback_runtime_instance_id,omitempty"`
					AcceptanceContractVersion   *int                                  `json:"acceptance_contract_version,omitempty"`
					AcceptanceContractDigest    *string                               `json:"acceptance_contract_digest,omitempty"`
					AcceptanceRisk              *string                               `json:"risk,omitempty"`
					IndependentVerifierRequired *bool                                 `json:"independent_verifier_required,omitempty"`
					VerifierAgentInstanceID     *string                               `json:"verifier_agent_instance_id,omitempty"`
					VerifierRuntimeInstanceID   *string                               `json:"verifier_runtime_instance_id,omitempty"`
					VerifierWorkflowPath        *string                               `json:"verifier_workflow_path,omitempty"`
					AssetRevisionBindings       *[]assets.ExactAssetRevisionBinding   `json:"asset_revision_bindings"`
					AssetRevisionSetDigest      *string                               `json:"asset_revision_set_digest"`
				} `json:"semantic_bindings"`
				RouteSummaries *[]projectedTeamRouteSummary `json:"route_summaries"`
			}
			if record.TeamInstanceID != "" ||
				decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != teamInstanceID ||
				!validSHA256Digest(payload.PlanDigest) ||
				!validSHA256Digest(payload.ViewVersion) ||
				len(payload.Nodes) == 0 ||
				len(payload.Nodes) > teams.MaxExecutionNodeCount {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			planInput := teams.ExecutionPlanInput{
				TeamInstanceID: payload.TeamInstanceID,
				Nodes:          make([]teams.ExecutionNodeInput, len(payload.Nodes)),
			}
			for index, node := range payload.Nodes {
				lineage, valid := decodeProjectedAssetLineage(
					node.AssetRevisionBindings, node.AssetRevisionSetDigest, nil, nil, false,
				)
				if !valid {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				planInput.Nodes[index] = teams.ExecutionNodeInput{
					LogicalNodeID: node.LogicalNodeID, Title: node.Title,
					AgentInstanceID: node.AgentInstanceID, RuntimeInstanceID: node.RuntimeInstanceID,
					Role: teams.ExecutionRole(node.Role), Kind: node.Kind, RouteGroupID: node.RouteGroupID,
					DependsOn: append([]string(nil), node.DependsOn...), MaxAttempts: node.MaxAttempts,
					AssetRevisionBindings:  append([]assets.ExactAssetRevisionBinding(nil), lineage.bindings...),
					AssetRevisionSetDigest: lineage.setDigest,
				}
			}
			if _, planErr := teams.BuildExecutionPlan(planInput); planErr != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			record = TeamExecution{
				TeamInstanceID:        teamInstanceID,
				PlanDigest:            payload.PlanDigest,
				Status:                "pending",
				Nodes:                 make([]TeamExecutionNode, len(payload.Nodes)),
				LegacySemanticUnbound: payload.SemanticBindings == nil,
			}
			semanticIndexes := make(map[string]int)
			semanticAssetLineage := make(map[string]projectedAssetLineage)
			routeSummaries := make(map[string]projectedTeamRouteSummary)
			if payload.RouteSummaries != nil {
				if len(*payload.RouteSummaries) != len(payload.Nodes) {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				for index, route := range *payload.RouteSummaries {
					if index > 0 && (*payload.RouteSummaries)[index-1].LogicalNodeID >= route.LogicalNodeID ||
						!validProjectedTeamRouteSummary(route) {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					routeSummaries[route.LogicalNodeID] = cloneProjectedTeamRouteSummary(route)
				}
			}
			if payload.SemanticBindings != nil {
				if len(*payload.SemanticBindings) != len(payload.Nodes) {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				for index, binding := range *payload.SemanticBindings {
					assetLineage, valid := decodeProjectedAssetLineage(
						binding.AssetRevisionBindings,
						binding.AssetRevisionSetDigest,
						nil,
						nil,
						false,
					)
					acceptancePresent, complete := completeProjectedAcceptanceBinding(
						binding.AcceptanceContractVersion,
						binding.AcceptanceContractDigest,
						binding.AcceptanceRisk,
						binding.IndependentVerifierRequired,
						binding.VerifierAgentInstanceID,
						binding.VerifierRuntimeInstanceID,
						binding.VerifierWorkflowPath,
					)
					if !valid || binding.LogicalNodeID == "" ||
						index > 0 &&
							(*payload.SemanticBindings)[index-1].
								LogicalNodeID >= binding.LogicalNodeID ||
						binding.OutputContractVersion < 1 ||
						!validSHA256Digest(binding.OutputContractDigest) ||
						binding.RecoveryPolicyVersion < 1 ||
						!validSHA256Digest(binding.RecoveryPolicyDigest) ||
						binding.AttemptCredits < 0 ||
						binding.AttemptCredits > 2 ||
						binding.PrimaryWorkflowPath == "" ||
						binding.WorkflowFallbackKey ==
							binding.PrimaryWorkflowPath {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					if !validProjectedFallbackApproval(
						binding.FallbackApproval,
						binding.WorkflowFallbackKey,
					) || binding.FallbackApproval == nil &&
						binding.FallbackRuntimeInstanceID != "" ||
						binding.FallbackApproval != nil &&
							binding.FallbackRuntimeInstanceID == "" {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					if binding.FallbackApproval != nil {
						approvedAt, _ := time.Parse(
							time.RFC3339Nano,
							binding.FallbackApproval.ApprovedAt,
						)
						if approvedAt.After(event.EmittedAt) {
							return TeamExecution{}, ErrInvalidProjectionEvent
						}
					}
					if !complete ||
						index > 0 &&
							record.LegacyAcceptanceUnbound == acceptancePresent {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					if index == 0 {
						record.LegacyAcceptanceUnbound = !acceptancePresent
					}
					semanticIndexes[binding.LogicalNodeID] = index
					semanticAssetLineage[binding.LogicalNodeID] = assetLineage
				}
			}
			for index, node := range payload.Nodes {
				nodeAssetLineage, valid := decodeProjectedAssetLineage(
					node.AssetRevisionBindings,
					node.AssetRevisionSetDigest,
					nil,
					nil,
					false,
				)
				if node.LogicalNodeID == "" ||
					node.MaxAttempts < 1 || node.MaxAttempts > 3 || !valid {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				record.Nodes[index] = TeamExecutionNode{
					LogicalNodeID:          node.LogicalNodeID,
					Title:                  node.Title,
					AgentInstanceID:        node.AgentInstanceID,
					RuntimeInstanceID:      node.RuntimeInstanceID,
					Role:                   node.Role,
					Kind:                   string(node.Kind),
					RouteGroupID:           node.RouteGroupID,
					DependsOn:              append([]string(nil), node.DependsOn...),
					MaxAttempts:            node.MaxAttempts,
					Status:                 "pending",
					Attempts:               []TeamExecutionAttempt{},
					AssetLineageAvailable:  nodeAssetLineage.available,
					AssetRevisionBindings:  nodeAssetLineage.bindings,
					AssetRevisionSetDigest: nodeAssetLineage.setDigest,
				}
				if payload.RouteSummaries != nil {
					route, ok := routeSummaries[node.LogicalNodeID]
					if !ok {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					applyProjectedInitialRoute(&record.Nodes[index], route)
				}
				if index == 0 {
					record.AssetLineageAvailable = nodeAssetLineage.available
				} else if record.AssetLineageAvailable != nodeAssetLineage.available {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				if payload.SemanticBindings != nil {
					bindingIndex, ok := semanticIndexes[node.LogicalNodeID]
					if !ok {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					binding := (*payload.SemanticBindings)[bindingIndex]
					semanticLineage := semanticAssetLineage[node.LogicalNodeID]
					if semanticLineage.available != nodeAssetLineage.available ||
						semanticLineage.setDigest != nodeAssetLineage.setDigest ||
						!equalProjectedAssetBindings(semanticLineage.bindings, nodeAssetLineage.bindings) {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					if binding.AttemptCredits > node.MaxAttempts-1 {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
					record.Nodes[index].OutputContractVersion =
						binding.OutputContractVersion
					record.Nodes[index].OutputContractDigest =
						binding.OutputContractDigest
					record.Nodes[index].RecoveryPolicyVersion =
						binding.RecoveryPolicyVersion
					record.Nodes[index].RecoveryPolicyDigest =
						binding.RecoveryPolicyDigest
					record.Nodes[index].AttemptCredits =
						binding.AttemptCredits
					record.Nodes[index].PrimaryWorkflowPath =
						binding.PrimaryWorkflowPath
					record.Nodes[index].WorkflowFallbackKey =
						binding.WorkflowFallbackKey
					record.Nodes[index].RecoveryApprovalRequired =
						binding.RecoveryApprovalRequired
					record.Nodes[index].FallbackRuntimeInstanceID =
						binding.FallbackRuntimeInstanceID
					if binding.FallbackApproval != nil {
						fallback := binding.FallbackApproval
						record.Nodes[index].FallbackApprovalAvailable = true
						record.Nodes[index].FallbackApprovalVersion = fallback.Version
						record.Nodes[index].FallbackApprovalID = fallback.ApprovalID
						record.Nodes[index].FallbackApprovalActorRef = fallback.ActorRef
						record.Nodes[index].FallbackApprovedAt, _ =
							time.Parse(time.RFC3339Nano, fallback.ApprovedAt)
						record.Nodes[index].FallbackSourceBindingDigest = fallback.SourceBindingDigest
						record.Nodes[index].FallbackTargetBindingDigest = fallback.TargetBindingDigest
						record.Nodes[index].FallbackApprovalDigest = fallback.Digest
					}
					if !record.LegacyAcceptanceUnbound {
						record.Nodes[index].AcceptanceContractVersion =
							*binding.AcceptanceContractVersion
						record.Nodes[index].AcceptanceContractDigest =
							*binding.AcceptanceContractDigest
						record.Nodes[index].AcceptanceRisk =
							*binding.AcceptanceRisk
						record.Nodes[index].IndependentVerifierRequired =
							*binding.IndependentVerifierRequired
						record.Nodes[index].VerifierAgentInstanceID =
							*binding.VerifierAgentInstanceID
						record.Nodes[index].VerifierRuntimeInstanceID =
							*binding.VerifierRuntimeInstanceID
						record.Nodes[index].VerifierWorkflowPath =
							*binding.VerifierWorkflowPath
						if !validProjectedAcceptanceBinding(
							record.Nodes[index],
							node.AgentInstanceID,
						) {
							return TeamExecution{}, ErrInvalidProjectionEvent
						}
					}
				}
				maxAttempts[node.LogicalNodeID] = node.MaxAttempts
			}
			sort.Slice(record.Nodes, func(i, j int) bool {
				return record.Nodes[i].LogicalNodeID < record.Nodes[j].LogicalNodeID
			})
		case "TeamNodeInitiallyBlocked":
			var payload struct {
				LogicalNodeID       string `json:"logical_node_id"`
				Code                string `json:"code"`
				Stage               string `json:"stage"`
				Reason              string `json:"reason"`
				Retryable           bool   `json:"retryable"`
				SourceLogicalNodeID string `json:"source_logical_node_id"`
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			if decodeExactProjectionPayload(event, &payload) != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node = projectedTeamNode(&record, payload.LogicalNodeID)
			if node == nil || node.Status != "pending" || node.CurrentAttempt != 0 ||
				event.CausationID != lastEventID ||
				!validProjectedInitialBlock(record, payload.LogicalNodeID, payload.Code,
					payload.Stage, payload.Reason, payload.Retryable, payload.SourceLogicalNodeID) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node.Status = "blocked"
			node.InitialBlockIncidentID = event.CorrelationID
			node.InitialBlockCode = payload.Code
			node.InitialBlockStage = payload.Stage
			node.InitialBlockReason = payload.Reason
			node.InitialBlockRetryable = payload.Retryable
			node.InitialBlockSourceNodeID = payload.SourceLogicalNodeID
		case "TeamNodeAttemptScheduled":
			var payload struct {
				LogicalNodeID     string  `json:"logical_node_id"`
				AttemptNumber     int     `json:"attempt_number"`
				WorkItemID        string  `json:"work_item_id"`
				RunID             string  `json:"run_id"`
				RuntimeInstanceID string  `json:"runtime_instance_id"`
				AgentInstanceID   string  `json:"agent_instance_id"`
				WorkflowPath      *string `json:"workflow_path"`
				RetryAt           string  `json:"retry_at"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			retryAt, err := parseProjectedRetryAt(payload.RetryAt)
			if node == nil || err != nil ||
				payload.AttemptNumber != node.CurrentAttempt+1 ||
				payload.AttemptNumber < 1 || payload.AttemptNumber > 3 ||
				payload.WorkItemID == "" || payload.RunID == "" ||
				payload.RuntimeInstanceID == "" ||
				payload.AgentInstanceID == "" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			workflowPath := ""
			if record.LegacySemanticUnbound {
				if payload.WorkflowPath != nil {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			} else {
				if payload.WorkflowPath == nil ||
					*payload.WorkflowPath == "" {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				workflowPath = *payload.WorkflowPath
				if payload.AttemptNumber == 1 {
					if workflowPath != node.PrimaryWorkflowPath ||
						!retryAt.IsZero() {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
				} else {
					expectedWorkflow := ""
					expectedRuntimeInstanceID := ""
					switch node.RecoveryAction {
					case "retry":
						previous := projectedTeamAttempt(
							node,
							payload.AttemptNumber-1,
						)
						if previous != nil {
							expectedWorkflow = previous.WorkflowPath
							expectedRuntimeInstanceID = previous.RuntimeInstanceID
						}
					case "fallback":
						expectedWorkflow = node.WorkflowFallbackKey
						previous := projectedTeamAttempt(
							node, payload.AttemptNumber-1,
						)
						if previous != nil {
							expectedRuntimeInstanceID = previous.RuntimeInstanceID
						}
						if node.FallbackRuntimeInstanceID != "" {
							expectedRuntimeInstanceID = node.FallbackRuntimeInstanceID
						}
					}
					if expectedWorkflow == "" ||
						workflowPath != expectedWorkflow ||
						payload.RuntimeInstanceID != expectedRuntimeInstanceID ||
						!retryAt.Equal(node.RetryAt) {
						return TeamExecution{}, ErrInvalidProjectionEvent
					}
				}
			}
			node.CurrentAttempt = payload.AttemptNumber
			node.RetryAt = retryAt
			if payload.AttemptNumber > 1 {
				if node.RecoveryAction == "fallback" {
					node.Status = "fallback_scheduled"
				} else {
					node.Status = "retry_scheduled"
				}
			}
			node.Attempts = append(node.Attempts, TeamExecutionAttempt{
				AttemptNumber:     payload.AttemptNumber,
				WorkItemID:        payload.WorkItemID,
				RunID:             payload.RunID,
				RuntimeInstanceID: payload.RuntimeInstanceID,
				AgentInstanceID:   payload.AgentInstanceID,
				WorkflowPath:      workflowPath,
				Status:            "scheduled",
			})
		case "TeamReadySetDispatched":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				ViewVersion    string `json:"view_version"`
				Attempts       []struct {
					LogicalNodeID                 string                              `json:"logical_node_id"`
					AttemptNumber                 int                                 `json:"attempt_number"`
					WorkItemID                    string                              `json:"work_item_id"`
					RunID                         string                              `json:"run_id"`
					ClaimID                       string                              `json:"claim_id"`
					ClaimGeneration               int64                               `json:"claim_generation"`
					RuntimeInstanceID             string                              `json:"runtime_instance_id"`
					AgentInstanceID               string                              `json:"agent_instance_id"`
					AssetRevisionBindings         *[]assets.ExactAssetRevisionBinding `json:"asset_revision_bindings"`
					AssetRevisionSetDigest        *string                             `json:"asset_revision_set_digest"`
					MaterializationManifestDigest *string                             `json:"materialization_manifest_digest"`
					MaterializationRootDigest     *string                             `json:"materialization_root_digest"`
					ExecutionBinding              *projectedExecutionBindingPayload   `json:"execution_binding,omitempty"`
					ContextCapsule                *contextcapsule.AuthorityRecord     `json:"context_capsule,omitempty"`
					RouteSegment                  *contextcapsule.RouteSegmentBinding `json:"route_segment,omitempty"`
				} `json:"attempts"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				!validSHA256Digest(payload.ViewVersion) ||
				len(payload.Attempts) == 0 {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			for _, dispatched := range payload.Attempts {
				executionBinding, bindingErr := projectedExecutionBinding(
					dispatched.ExecutionBinding,
				)
				contextRecord := contextcapsule.AuthorityRecord{}
				var contextErr error
				if dispatched.ContextCapsule != nil {
					contextRecord, contextErr = contextcapsule.ValidateAuthorityRecord(
						*dispatched.ContextCapsule,
					)
				}
				routeSegment := contextcapsule.RouteSegmentBinding{}
				var routeSegmentErr error
				if dispatched.ContextCapsule != nil && dispatched.RouteSegment != nil {
					routeSegment, routeSegmentErr = contextcapsule.ValidateRouteSegmentBinding(
						*dispatched.RouteSegment,
					)
				} else if dispatched.ContextCapsule != nil || dispatched.RouteSegment != nil {
					routeSegmentErr = contextcapsule.ErrInvalidRouteSegmentBinding
				}
				node := projectedTeamNode(&record, dispatched.LogicalNodeID)
				attempt := projectedTeamAttempt(node, dispatched.AttemptNumber)
				dispatchedLineage, validLineage := decodeProjectedAssetLineage(
					dispatched.AssetRevisionBindings,
					dispatched.AssetRevisionSetDigest,
					dispatched.MaterializationManifestDigest,
					dispatched.MaterializationRootDigest,
					true,
				)
				if attempt == nil ||
					attempt.WorkItemID != dispatched.WorkItemID ||
					attempt.RunID != dispatched.RunID ||
					attempt.RuntimeInstanceID != dispatched.RuntimeInstanceID ||
					attempt.AgentInstanceID != dispatched.AgentInstanceID ||
					dispatched.ClaimID == "" ||
					dispatched.ClaimGeneration <= 0 || !validLineage ||
					dispatchedLineage.available != node.AssetLineageAvailable ||
					dispatchedLineage.setDigest != node.AssetRevisionSetDigest ||
					!equalProjectedAssetBindings(dispatchedLineage.bindings, node.AssetRevisionBindings) ||
					bindingErr != nil ||
					dispatched.AttemptNumber == 1 && node.InitialRouteAvailable &&
						!projectedInitialRouteMatchesBinding(*node, executionBinding) ||
					dispatched.ExecutionBinding != nil &&
						executionBinding.RuntimeInstanceID != dispatched.RuntimeInstanceID ||
					contextErr != nil || routeSegmentErr != nil ||
					dispatched.ContextCapsule != nil &&
						(routeSegment.ConversationID != contextRecord.ConversationID ||
							routeSegment.TeamID != contextRecord.TeamID ||
							routeSegment.AgentID != dispatched.AgentInstanceID ||
							routeSegment.RoleID != dispatched.LogicalNodeID ||
							routeSegment.AttemptNumber != dispatched.AttemptNumber ||
							routeSegment.CapsuleDigest != contextRecord.CapsuleDigest ||
							routeSegment.ExecutionBindingDigest != executionBinding.BindingDigest) ||
					dispatched.ContextCapsule != nil &&
						(contextRecord.TeamID != record.TeamInstanceID ||
							contextRecord.AgentID != dispatched.AgentInstanceID ||
							contextRecord.RoleID != dispatched.LogicalNodeID ||
							contextRecord.ProviderID != executionBinding.ProviderID ||
							contextRecord.ProviderAccountID != executionBinding.ProviderAccountID ||
							contextRecord.ModelID != executionBinding.ModelID ||
							contextRecord.AuthMode != string(executionBinding.AuthMode)) {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
				attempt.ClaimID = dispatched.ClaimID
				attempt.ClaimGeneration = dispatched.ClaimGeneration
				attempt.IncidentID = event.CorrelationID
				attempt.Status = "dispatched"
				attempt.AssetLineageAvailable = dispatchedLineage.available
				attempt.AssetRevisionBindings = dispatchedLineage.bindings
				attempt.AssetRevisionSetDigest = dispatchedLineage.setDigest
				attempt.MaterializationManifestDigest = dispatchedLineage.manifestDigest
				attempt.MaterializationRootDigest = dispatchedLineage.rootDigest
				attempt.ExecutionBindingAvailable = dispatched.ExecutionBinding != nil
				attempt.ExecutionBinding = executionBinding
				attempt.ContextCapsuleAvailable = dispatched.ContextCapsule != nil
				attempt.ContextCapsule = contextRecord
				attempt.RouteSegmentAvailable = dispatched.RouteSegment != nil
				attempt.RouteSegment = routeSegment
				node.Status = "running"
				node.RetryAt = time.Time{}
			}
			record.Status = "running"
		case "TeamNodeAttemptRebound":
			var payload struct {
				TeamInstanceID          string `json:"team_instance_id"`
				PlanDigest              string `json:"plan_digest"`
				LogicalNodeID           string `json:"logical_node_id"`
				AttemptNumber           int    `json:"attempt_number"`
				WorkItemID              string `json:"work_item_id"`
				RunID                   string `json:"run_id"`
				PreviousClaimID         string `json:"previous_claim_id"`
				PreviousClaimGeneration int64  `json:"previous_claim_generation"`
				ClaimID                 string `json:"claim_id"`
				ClaimGeneration         int64  `json:"claim_generation"`
				RuntimeInstanceID       string `json:"runtime_instance_id"`
				AgentInstanceID         string `json:"agent_instance_id"`
				RunStream               string `json:"run_stream"`
				RunSequence             int64  `json:"run_sequence"`
				RunEventID              string `json:"run_event_id"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				payload.RunStream != "run/"+payload.RunID ||
				payload.PreviousClaimID == "" ||
				payload.ClaimID == "" ||
				payload.ClaimID == payload.PreviousClaimID ||
				payload.PreviousClaimGeneration <= 0 ||
				payload.ClaimGeneration != payload.PreviousClaimGeneration+1 {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			reference, exists := runClaimReferences[teamExecutionRunReferenceKey(
				payload.RunStream,
				payload.RunSequence,
			)]
			if !exists ||
				reference.eventID != payload.RunEventID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			attempt := projectedTeamAttempt(node, payload.AttemptNumber)
			if node == nil ||
				attempt == nil ||
				node.Status != "running" ||
				node.CurrentAttempt != payload.AttemptNumber ||
				attempt.Status != "dispatched" ||
				attempt.WorkItemID != payload.WorkItemID ||
				attempt.RunID != payload.RunID ||
				attempt.ClaimID != payload.PreviousClaimID ||
				attempt.ClaimGeneration != payload.PreviousClaimGeneration ||
				attempt.RuntimeInstanceID != payload.RuntimeInstanceID ||
				attempt.AgentInstanceID != payload.AgentInstanceID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			attempt.ClaimID = payload.ClaimID
			attempt.ClaimGeneration = payload.ClaimGeneration
			if event.CorrelationID != "" {
				attempt.IncidentID = event.CorrelationID
			}
		case "TeamNodeAttemptTerminal":
			var payload struct {
				LogicalNodeID              string  `json:"logical_node_id"`
				AttemptNumber              int     `json:"attempt_number"`
				WorkItemID                 string  `json:"work_item_id"`
				RunID                      string  `json:"run_id"`
				ClaimID                    string  `json:"claim_id"`
				ClaimGeneration            int64   `json:"claim_generation"`
				RuntimeInstanceID          string  `json:"runtime_instance_id"`
				AgentInstanceID            string  `json:"agent_instance_id"`
				Status                     string  `json:"status"`
				Reason                     *string `json:"reason,omitempty"`
				EvidenceID                 string  `json:"evidence_id"`
				EvidenceDigest             string  `json:"evidence_digest"`
				OutputContractVersion      int     `json:"output_contract_version"`
				OutputContractDigest       string  `json:"output_contract_digest"`
				OutputClassification       string  `json:"output_classification"`
				OutputClassificationDigest string  `json:"output_classification_digest"`
				OutputSummaryDigest        string  `json:"output_summary_digest"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				!validSHA256Digest(payload.EvidenceDigest) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			attempt := projectedTeamAttempt(node, payload.AttemptNumber)
			reason := ""
			if payload.Reason != nil {
				reason = *payload.Reason
			}
			if attempt == nil ||
				attempt.WorkItemID != payload.WorkItemID ||
				attempt.RunID != payload.RunID ||
				attempt.ClaimID != payload.ClaimID ||
				attempt.ClaimGeneration != payload.ClaimGeneration ||
				attempt.RuntimeInstanceID != payload.RuntimeInstanceID ||
				attempt.AgentInstanceID != payload.AgentInstanceID ||
				payload.EvidenceID == "" ||
				payload.Status != "succeeded" &&
					payload.Status != "failed" &&
					payload.Status != "cancelled" ||
				payload.Reason != nil &&
					!validProjectedTeamTerminalReason(payload.Status, reason) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			if !record.LegacySemanticUnbound &&
				(payload.OutputContractVersion !=
					node.OutputContractVersion ||
					payload.OutputContractDigest !=
						node.OutputContractDigest ||
					!validProjectedOutputClassification(
						payload.OutputClassification,
					) ||
					!validSHA256Digest(
						payload.OutputClassificationDigest,
					) ||
					!validSHA256Digest(payload.OutputSummaryDigest)) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			attempt.Status = payload.Status
			attempt.TerminalReason = reason
			if attempt.IncidentID == "" {
				attempt.IncidentID = event.CorrelationID
			}
			attempt.EvidenceID = payload.EvidenceID
			attempt.EvidenceDigest = payload.EvidenceDigest
			attempt.OutputContractVersion = payload.OutputContractVersion
			attempt.OutputContractDigest = payload.OutputContractDigest
			attempt.OutputClassification = payload.OutputClassification
			attempt.OutputClassificationDigest =
				payload.OutputClassificationDigest
			attempt.OutputSummaryDigest = payload.OutputSummaryDigest
			if !record.LegacySemanticUnbound &&
				(payload.OutputClassification == "valid_nonempty" ||
					payload.OutputClassification == "valid_empty") {
				if record.LegacyAcceptanceUnbound {
					node.Status = "succeeded"
					node.DependencySatisfied = true
				} else {
					node.Status = "ready_for_review"
					node.DependencySatisfied = false
				}
			} else if record.LegacySemanticUnbound &&
				payload.Status == "succeeded" {
				node.Status = "succeeded"
				node.DependencySatisfied = true
			} else if record.LegacySemanticUnbound &&
				payload.AttemptNumber >= maxAttempts[payload.LogicalNodeID] {
				node.Status = payload.Status
				node.DependencySatisfied = false
			} else {
				node.Status = "awaiting_recovery"
				node.DependencySatisfied = false
				record.Status = "awaiting_recovery"
			}
		case "TeamNodeAcceptanceCommitted":
			var payload struct {
				TeamInstanceID            string `json:"team_instance_id"`
				PlanDigest                string `json:"plan_digest"`
				LogicalNodeID             string `json:"logical_node_id"`
				AttemptNumber             int    `json:"attempt_number"`
				WorkItemID                string `json:"work_item_id"`
				WorkOutcomeEventID        string `json:"work_outcome_event_id"`
				AcceptanceContractVersion int    `json:"acceptance_contract_version"`
				AcceptanceContractDigest  string `json:"acceptance_contract_digest"`
				AcceptanceDecisionKind    string `json:"acceptance_decision_kind"`
				AcceptanceDecisionDigest  string `json:"acceptance_decision_digest"`
				DecidedAt                 string `json:"decided_at"`
				NodeStatus                string `json:"node_status"`
				DependencySatisfied       bool   `json:"dependency_satisfied"`
				RecoveryTrigger           string `json:"recovery_trigger"`
				RecoveryPolicyVersion     int    `json:"recovery_policy_version"`
				RecoveryPolicyDigest      string `json:"recovery_policy_digest"`
				CreditsBefore             int    `json:"credits_before"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil ||
				record.LegacyAcceptanceUnbound ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				event.ID != projectionDeterministicEventID(
					"TeamNodeAcceptanceCommitted",
					payload.TeamInstanceID,
					payload.PlanDigest,
					payload.LogicalNodeID,
					fmt.Sprint(payload.AttemptNumber),
					payload.WorkItemID,
					payload.WorkOutcomeEventID,
					fmt.Sprint(payload.AcceptanceContractVersion),
					payload.AcceptanceContractDigest,
					payload.AcceptanceDecisionKind,
					payload.AcceptanceDecisionDigest,
					payload.DecidedAt,
					payload.NodeStatus,
					fmt.Sprint(payload.DependencySatisfied),
					payload.RecoveryTrigger,
					fmt.Sprint(payload.RecoveryPolicyVersion),
					payload.RecoveryPolicyDigest,
					fmt.Sprint(payload.CreditsBefore),
				) ||
				event.CausationID != payload.WorkOutcomeEventID ||
				!validSHA256Digest(payload.AcceptanceDecisionDigest) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			attempt := projectedTeamAttempt(node, payload.AttemptNumber)
			decidedAt, decidedErr := parseProjectedRetryAt(payload.DecidedAt)
			outcome, outcomeExists :=
				workOutcomeReferences[payload.WorkOutcomeEventID]
			if node == nil ||
				attempt == nil ||
				node.Status != "ready_for_review" ||
				node.CurrentAttempt != payload.AttemptNumber ||
				attempt.WorkItemID != payload.WorkItemID ||
				payload.AcceptanceContractVersion !=
					node.AcceptanceContractVersion ||
				payload.AcceptanceContractDigest !=
					node.AcceptanceContractDigest ||
				decidedErr != nil ||
				decidedAt.IsZero() ||
				!outcomeExists ||
				outcome.StreamID != "work-item/"+payload.WorkItemID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			var outcomePayload runProjectionAcceptanceOutcomePayload
			if decodeExactProjectionPayload(outcome, &outcomePayload) != nil ||
				outcomePayload.WorkItemID != payload.WorkItemID ||
				outcomePayload.RunID != attempt.RunID ||
				outcomePayload.ClaimGeneration !=
					attempt.ClaimGeneration ||
				outcomePayload.AcceptanceDecisionDigest !=
					payload.AcceptanceDecisionDigest ||
				outcome.ID != projectionAcceptanceOutcomeEventID(
					outcome.Type,
					outcomePayload,
				) ||
				outcome.CausationID !=
					outcomePayload.VerificationEventID ||
				outcomePayload.VerificationEventID == "" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			switch payload.AcceptanceDecisionKind {
			case "accepted":
				if outcome.Type != "WorkItemDone" ||
					outcomePayload.Status != "done" ||
					payload.NodeStatus != "succeeded" ||
					!payload.DependencySatisfied ||
					payload.RecoveryTrigger != "" ||
					payload.RecoveryPolicyVersion != 0 ||
					payload.RecoveryPolicyDigest != "" ||
					payload.CreditsBefore != 0 {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "rejected":
				expectedCredits := node.AttemptCredits -
					(payload.AttemptNumber - 1)
				if expectedCredits < 0 {
					expectedCredits = 0
				}
				if outcome.Type != "WorkItemRejected" ||
					outcomePayload.Status != "rejected" ||
					payload.NodeStatus != "awaiting_recovery" ||
					payload.DependencySatisfied ||
					payload.RecoveryTrigger !=
						"verification_rejected" ||
					payload.RecoveryPolicyVersion !=
						node.RecoveryPolicyVersion ||
					payload.RecoveryPolicyDigest !=
						node.RecoveryPolicyDigest ||
					payload.CreditsBefore != expectedCredits {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			default:
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node.Status = payload.NodeStatus
			node.DependencySatisfied = payload.DependencySatisfied
			node.AcceptanceDecisionKind =
				payload.AcceptanceDecisionKind
			node.AcceptanceDecisionDigest =
				payload.AcceptanceDecisionDigest
			node.AcceptanceDecisionTime = decidedAt
			node.RecoveryTrigger = payload.RecoveryTrigger
			node.CreditsBefore = payload.CreditsBefore
			if payload.NodeStatus == "awaiting_recovery" {
				record.Status = "awaiting_recovery"
			}
		case "TeamNodeRecoveryRecorded":
			var payload struct {
				LogicalNodeID               string   `json:"logical_node_id"`
				AttemptNumber               int      `json:"attempt_number"`
				Action                      string   `json:"action"`
				DecisionTime                string   `json:"decision_time"`
				RetryAt                     string   `json:"retry_at"`
				NextAttemptNumber           int      `json:"next_attempt_number"`
				NextAgentInstanceID         string   `json:"next_agent_instance_id"`
				NextRuntimeInstanceID       string   `json:"next_runtime_instance_id"`
				WorkflowFallbackKey         string   `json:"workflow_fallback_key"`
				RecoveryPolicyVersion       int      `json:"recovery_policy_version"`
				RecoveryPolicyDigest        string   `json:"recovery_policy_digest"`
				RecoveryDecisionDigest      string   `json:"recovery_decision_digest"`
				ClassificationDigest        string   `json:"classification_digest"`
				PriorClassifications        []string `json:"prior_classifications"`
				CreditsBefore               int      `json:"credits_before"`
				CreditsAfter                int      `json:"credits_after"`
				FallbackConsumed            bool     `json:"fallback_consumed"`
				RecoveryApprovalRequired    bool     `json:"recovery_approval_required"`
				DependencySatisfied         bool     `json:"dependency_satisfied"`
				RecoveryTrigger             string   `json:"recovery_trigger"`
				AcceptanceDecisionDigest    string   `json:"acceptance_decision_digest"`
				FallbackApprovalVersion     int      `json:"fallback_approval_version,omitempty"`
				FallbackApprovalID          string   `json:"fallback_approval_id,omitempty"`
				FallbackApprovalDigest      string   `json:"fallback_approval_digest,omitempty"`
				FallbackSourceBindingDigest string   `json:"fallback_source_binding_digest,omitempty"`
				FallbackTargetBindingDigest string   `json:"fallback_target_binding_digest,omitempty"`
			}
			if decodeExactProjectionPayload(event, &payload) != nil {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node := projectedTeamNode(&record, payload.LogicalNodeID)
			decisionTime, decisionErr := parseProjectedRetryAt(
				payload.DecisionTime,
			)
			retryAt, retryErr := parseProjectedRetryAt(payload.RetryAt)
			currentAttempt := projectedTeamAttempt(
				node,
				payload.AttemptNumber,
			)
			expectedCredits := 0
			if node != nil {
				expectedCredits = node.AttemptCredits -
					(payload.AttemptNumber - 1)
				if expectedCredits < 0 {
					expectedCredits = 0
				}
			}
			if node == nil ||
				record.LegacySemanticUnbound ||
				node.CurrentAttempt != payload.AttemptNumber ||
				currentAttempt == nil ||
				decisionErr != nil || decisionTime.IsZero() ||
				retryErr != nil ||
				payload.RecoveryPolicyVersion !=
					node.RecoveryPolicyVersion ||
				payload.RecoveryPolicyDigest !=
					node.RecoveryPolicyDigest ||
				!validSHA256Digest(payload.RecoveryDecisionDigest) ||
				!validSHA256Digest(payload.ClassificationDigest) ||
				payload.CreditsBefore < 0 ||
				payload.CreditsBefore > node.AttemptCredits ||
				payload.CreditsBefore != expectedCredits ||
				payload.CreditsAfter < 0 ||
				payload.CreditsAfter > payload.CreditsBefore ||
				len(payload.PriorClassifications) > 2 ||
				len(payload.PriorClassifications) !=
					payload.AttemptNumber-1 ||
				payload.ClassificationDigest !=
					currentAttempt.OutputClassificationDigest ||
				payload.RecoveryApprovalRequired !=
					node.RecoveryApprovalRequired ||
				!validProjectedRecoveryFallbackApproval(
					payload.Action,
					payload.FallbackApprovalVersion,
					payload.FallbackApprovalID,
					payload.FallbackApprovalDigest,
					payload.FallbackSourceBindingDigest,
					payload.FallbackTargetBindingDigest,
					node,
					currentAttempt,
				) {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			expectedTrigger := node.RecoveryTrigger
			if expectedTrigger == "" {
				expectedTrigger = "output"
			}
			payloadTrigger := payload.RecoveryTrigger
			if payloadTrigger == "" &&
				payload.AcceptanceDecisionDigest == "" {
				payloadTrigger = "output"
			}
			if payloadTrigger != expectedTrigger ||
				payload.AcceptanceDecisionDigest !=
					node.AcceptanceDecisionDigest ||
				payloadTrigger == "verification_rejected" &&
					payload.Action == "fallback" {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			for _, classification := range payload.PriorClassifications {
				if !validProjectedOutputClassification(classification) {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			}
			for index, classification := range payload.PriorClassifications {
				if index >= len(node.Attempts) ||
					node.Attempts[index].AttemptNumber != index+1 ||
					node.Attempts[index].OutputClassification !=
						classification {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			}
			fallbackBefore := projectedTeamFallbackConsumed(*node)
			switch payload.Action {
			case "retry":
				if payload.NextAttemptNumber != payload.AttemptNumber+1 ||
					payload.NextAgentInstanceID !=
						currentAttempt.AgentInstanceID ||
					payload.NextRuntimeInstanceID !=
						currentAttempt.RuntimeInstanceID ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore-1 ||
					retryAt.IsZero() ||
					payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "fallback":
				expectedRuntimeInstanceID := currentAttempt.RuntimeInstanceID
				if node.FallbackRuntimeInstanceID != "" {
					expectedRuntimeInstanceID = node.FallbackRuntimeInstanceID
				}
				if payload.NextAttemptNumber != payload.AttemptNumber+1 ||
					payload.NextAgentInstanceID !=
						currentAttempt.AgentInstanceID ||
					payload.NextRuntimeInstanceID != expectedRuntimeInstanceID ||
					payload.WorkflowFallbackKey == "" ||
					payload.WorkflowFallbackKey !=
						node.WorkflowFallbackKey ||
					payload.CreditsAfter != payload.CreditsBefore-1 ||
					retryAt.IsZero() ||
					payload.DependencySatisfied ||
					fallbackBefore ||
					!payload.FallbackConsumed {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "degraded":
				if payload.NextAttemptNumber != 0 ||
					payload.NextAgentInstanceID != "" ||
					payload.NextRuntimeInstanceID != "" ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore ||
					!retryAt.IsZero() ||
					!payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			case "blocked", "human_required":
				if payload.NextAttemptNumber != 0 ||
					payload.NextAgentInstanceID != "" ||
					payload.NextRuntimeInstanceID != "" ||
					payload.WorkflowFallbackKey != "" ||
					payload.CreditsAfter != payload.CreditsBefore ||
					!retryAt.IsZero() ||
					payload.DependencySatisfied ||
					payload.FallbackConsumed != fallbackBefore {
					return TeamExecution{}, ErrInvalidProjectionEvent
				}
			default:
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			node.DependencySatisfied = payload.DependencySatisfied
			node.RecoveryTrigger = payloadTrigger
			node.RecoveryAction = payload.Action
			node.RecoveryDecisionDigest = payload.RecoveryDecisionDigest
			node.RecoveryDecisionTime = decisionTime
			node.RetryAt = retryAt
			node.CreditsBefore = payload.CreditsBefore
			node.CreditsAfter = payload.CreditsAfter
			node.FallbackConsumed = payload.FallbackConsumed
			node.PriorClassifications = append(
				[]string(nil),
				payload.PriorClassifications...,
			)
			switch payload.Action {
			case "retry":
				node.Status = "retry_scheduled"
			case "fallback":
				node.Status = "fallback_scheduled"
			case "degraded":
				node.Status = "degraded"
			case "blocked":
				node.Status = "blocked"
			case "human_required":
				node.Status = "human_required"
			default:
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
		case "TeamExecutionTerminal":
			var payload struct {
				TeamInstanceID string `json:"team_instance_id"`
				PlanDigest     string `json:"plan_digest"`
				Status         string `json:"status"`
				Reason         string `json:"reason"`
			}
			expectedStatus, expectedReason :=
				projectedTeamTerminal(record)
			if decodeExactProjectionPayload(event, &payload) != nil ||
				payload.TeamInstanceID != record.TeamInstanceID ||
				payload.PlanDigest != record.PlanDigest ||
				!validProjectedTeamTerminal(payload.Status) ||
				expectedStatus == "" ||
				payload.Status != expectedStatus ||
				payload.Reason != expectedReason ||
				event.CausationID != lastEventID {
				return TeamExecution{}, ErrInvalidProjectionEvent
			}
			record.Status = payload.Status
		default:
			return TeamExecution{}, fmt.Errorf(
				"%w: unknown Team execution event %s",
				ErrInvalidProjectionEvent,
				event.Type,
			)
		}
		lastEventID = event.ID
	}
	return cloneGlobalTeamExecution(record), nil
}

func projectedExecutionBinding(
	input *projectedExecutionBindingPayload,
) (loomruntime.FrozenExecutionBinding, error) {
	if input == nil {
		return loomruntime.FrozenExecutionBinding{}, nil
	}
	binding := loomruntime.FrozenExecutionBinding{
		ProfileID:                  input.ProfileID,
		HarnessAdapter:             input.HarnessAdapter,
		RuntimeInstanceID:          input.RuntimeInstanceID,
		ProviderID:                 input.ProviderID,
		ProviderAccountID:          input.ProviderAccountID,
		ModelID:                    input.ModelID,
		AuthMode:                   loomruntime.AuthMode(input.AuthMode),
		EndpointFingerprint:        input.EndpointFingerprint,
		CredentialReference:        input.CredentialReference,
		CredentialRevision:         input.CredentialRevision,
		ReasoningEffort:            input.ReasoningEffort,
		Timeout:                    time.Duration(input.TimeoutNanoseconds),
		Capabilities:               append([]string(nil), input.Capabilities...),
		BindingDigest:              input.BindingDigest,
		RemoteToolEnrollmentID:     input.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: input.RemoteToolEnrollmentDigest,
	}
	if input.Budget != nil {
		budget := *input.Budget
		binding.Budget = &budget
	}
	return loomruntime.ValidateFrozenExecutionBinding(binding)
}

func validProjectedTeamTerminalReason(status, reason string) bool {
	if status == "succeeded" {
		return reason == ""
	}
	if status != "failed" && status != "cancelled" ||
		reason == "" || len(reason) > 64 {
		return false
	}
	for _, current := range reason {
		if current >= 'a' && current <= 'z' ||
			current >= '0' && current <= '9' ||
			current == '_' || current == '-' || current == '.' {
			continue
		}
		return false
	}
	return true
}

type projectedAssetLineage struct {
	available      bool
	bindings       []assets.ExactAssetRevisionBinding
	setDigest      string
	manifestDigest string
	rootDigest     string
}

func decodeProjectedAssetLineage(
	bindings *[]assets.ExactAssetRevisionBinding,
	setDigest *string,
	manifestDigest *string,
	rootDigest *string,
	requireManifestField bool,
) (projectedAssetLineage, bool) {
	present := bindings != nil || setDigest != nil ||
		manifestDigest != nil || rootDigest != nil
	if !present {
		return projectedAssetLineage{bindings: []assets.ExactAssetRevisionBinding{}}, true
	}
	if bindings == nil || setDigest == nil ||
		requireManifestField && (manifestDigest == nil || rootDigest == nil) ||
		!requireManifestField && (manifestDigest != nil || rootDigest != nil) {
		return projectedAssetLineage{}, false
	}
	manifest := ""
	root := ""
	if manifestDigest != nil {
		manifest = *manifestDigest
	}
	if rootDigest != nil {
		root = *rootDigest
	}
	if len(*bindings) == 0 {
		if *setDigest != "" || manifest != "" || root != "" {
			return projectedAssetLineage{}, false
		}
		return projectedAssetLineage{
			bindings: []assets.ExactAssetRevisionBinding{},
		}, true
	}
	digest, err := assets.CanonicalAssetRevisionSetDigest(*bindings)
	if err != nil || digest != *setDigest ||
		requireManifestField && (!validSHA256Digest(manifest) ||
			!validSHA256Digest(root)) {
		return projectedAssetLineage{}, false
	}
	return projectedAssetLineage{
		available:      true,
		bindings:       append([]assets.ExactAssetRevisionBinding{}, (*bindings)...),
		setDigest:      *setDigest,
		manifestDigest: manifest,
		rootDigest:     root,
	}, true
}

func equalProjectedAssetBindings(
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

func projectedTeamTerminal(record TeamExecution) (string, string) {
	if len(record.Nodes) == 0 {
		return "", ""
	}
	for _, node := range record.Nodes {
		switch node.Status {
		case "succeeded", "failed", "cancelled", "degraded", "blocked",
			"human_required":
		default:
			return "", ""
		}
	}
	for _, status := range []string{
		"human_required",
		"blocked",
		"failed",
		"cancelled",
		"degraded",
	} {
		for _, node := range record.Nodes {
			if node.Status == status {
				return status,
					"node_" + node.LogicalNodeID + "_" + status
			}
		}
	}
	for _, node := range record.Nodes {
		if node.Status != "succeeded" {
			return "failed",
				"node_" + node.LogicalNodeID + "_" + node.Status
		}
	}
	return "succeeded", ""
}

func validProjectedInitialBlock(
	record TeamExecution,
	logicalNodeID string,
	code string,
	stage string,
	reason string,
	retryable bool,
	sourceLogicalNodeID string,
) bool {
	if logicalNodeID == "" || code == "" || stage == "" || reason == "" ||
		len(code) > 64 || len(stage) > 64 || len(reason) > 512 ||
		!validProjectedInitialBlockToken(code) || !validProjectedInitialBlockToken(stage) ||
		!validProjectedInitialBlockReason(reason) {
		return false
	}
	node := projectedTeamNode(&record, logicalNodeID)
	if node == nil {
		return false
	}
	if code != "dependency_blocked" {
		return sourceLogicalNodeID == ""
	}
	if stage != "agent_attempt_dispatch" ||
		reason != "A required Agent is blocked." ||
		sourceLogicalNodeID == "" || sourceLogicalNodeID == logicalNodeID {
		return false
	}
	declared := false
	for _, dependency := range node.DependsOn {
		if dependency == sourceLogicalNodeID {
			declared = true
			break
		}
	}
	source := projectedTeamNode(&record, sourceLogicalNodeID)
	return declared && source != nil && source.Status == "blocked" &&
		retryable == source.InitialBlockRetryable
}

func validProjectedInitialBlockToken(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, current := range value {
		if current != '_' && (current < 'a' || current > 'z') &&
			(current < '0' || current > '9') {
			return false
		}
	}
	return true
}

func validProjectedInitialBlockReason(value string) bool {
	if value == "" || len(value) > 512 || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) {
			return false
		}
	}
	return true
}

func validProjectedTeamRouteSummary(route projectedTeamRouteSummary) bool {
	credentialValid := route.ProviderAccountID == "" && route.CredentialRevision == 0 ||
		validProjectionText(route.ProviderAccountID) && route.CredentialRevision > 0
	return validProjectionText(route.LogicalNodeID) &&
		validProjectionText(route.HarnessAdapter) &&
		validProjectionText(route.ProviderID) && credentialValid &&
		validProjectionText(route.ModelID) &&
		(route.ReasoningEffort == "" || validProjectionText(route.ReasoningEffort)) &&
		route.TimeoutNanoseconds > 0 &&
		(route.Budget == nil || *route.Budget >= 0) &&
		validSortedProjectionTexts(route.Capabilities, true)
}

func cloneProjectedTeamRouteSummary(route projectedTeamRouteSummary) projectedTeamRouteSummary {
	route.Capabilities = append([]string(nil), route.Capabilities...)
	if route.Budget != nil {
		budget := *route.Budget
		route.Budget = &budget
	}
	return route
}

func applyProjectedInitialRoute(node *TeamExecutionNode, route projectedTeamRouteSummary) {
	if node == nil {
		return
	}
	node.InitialRouteAvailable = true
	node.InitialHarnessAdapter = route.HarnessAdapter
	node.InitialProviderID = route.ProviderID
	node.InitialProviderAccountID = route.ProviderAccountID
	node.InitialModelID = route.ModelID
	node.InitialReasoningEffort = route.ReasoningEffort
	node.InitialTimeoutNanoseconds = route.TimeoutNanoseconds
	node.InitialCapabilities = append([]string(nil), route.Capabilities...)
	node.InitialCredentialRevision = route.CredentialRevision
	if route.Budget != nil {
		budget := *route.Budget
		node.InitialBudget = &budget
	}
}

func projectedInitialRouteMatchesBinding(
	node TeamExecutionNode,
	binding loomruntime.FrozenExecutionBinding,
) bool {
	if !node.InitialRouteAvailable {
		return true
	}
	if node.InitialHarnessAdapter != binding.HarnessAdapter ||
		node.InitialProviderID != binding.ProviderID ||
		node.InitialProviderAccountID != binding.ProviderAccountID ||
		node.InitialModelID != binding.ModelID ||
		node.InitialReasoningEffort != binding.ReasoningEffort ||
		node.InitialTimeoutNanoseconds != int64(binding.Timeout) ||
		node.InitialCredentialRevision != binding.CredentialRevision ||
		!equalProjectionStrings(node.InitialCapabilities, binding.Capabilities) {
		return false
	}
	if node.InitialBudget == nil || binding.Budget == nil {
		return node.InitialBudget == nil && binding.Budget == nil
	}
	return *node.InitialBudget == *binding.Budget
}

func completeProjectedAcceptanceBinding(
	version *int,
	digest *string,
	risk *string,
	required *bool,
	agentID *string,
	runtimeID *string,
	workflowPath *string,
) (bool, bool) {
	present := 0
	for _, exists := range []bool{
		version != nil,
		digest != nil,
		risk != nil,
		required != nil,
		agentID != nil,
		runtimeID != nil,
		workflowPath != nil,
	} {
		if exists {
			present++
		}
	}
	return present == 7, present == 0 || present == 7
}

func validProjectedAcceptanceBinding(
	node TeamExecutionNode,
	sourceAgentInstanceID string,
) bool {
	if node.AcceptanceContractVersion < 1 ||
		!validSHA256Digest(node.AcceptanceContractDigest) ||
		(node.AcceptanceRisk != "low" &&
			node.AcceptanceRisk != "medium" &&
			node.AcceptanceRisk != "high") {
		return false
	}
	required := node.AcceptanceRisk == "medium" ||
		node.AcceptanceRisk == "high"
	if node.IndependentVerifierRequired != required {
		return false
	}
	if !required {
		return node.VerifierAgentInstanceID == "" &&
			node.VerifierRuntimeInstanceID == "" &&
			node.VerifierWorkflowPath == ""
	}
	return node.VerifierAgentInstanceID != "" &&
		node.VerifierAgentInstanceID != sourceAgentInstanceID &&
		node.VerifierRuntimeInstanceID != "" &&
		node.VerifierWorkflowPath != ""
}

type teamExecutionRunClaimReference struct {
	eventID string
}

func teamExecutionRunReferenceKey(streamID string, sequence int64) string {
	return streamID + "\x00" + fmt.Sprint(sequence)
}

func projectedTeamNode(
	record *TeamExecution,
	logicalNodeID string,
) *TeamExecutionNode {
	for index := range record.Nodes {
		if record.Nodes[index].LogicalNodeID == logicalNodeID {
			return &record.Nodes[index]
		}
	}
	return nil
}

func projectedTeamAttempt(
	node *TeamExecutionNode,
	attemptNumber int,
) *TeamExecutionAttempt {
	if node == nil {
		return nil
	}
	for index := range node.Attempts {
		if node.Attempts[index].AttemptNumber == attemptNumber {
			return &node.Attempts[index]
		}
	}
	return nil
}

func projectedTeamFallbackConsumed(node TeamExecutionNode) bool {
	if node.WorkflowFallbackKey == "" {
		return false
	}
	for _, attempt := range node.Attempts {
		if attempt.WorkflowPath == node.WorkflowFallbackKey {
			return true
		}
	}
	return false
}

func validProjectedFallbackApproval(
	approval *projectedTeamFallbackApprovalPayload,
	workflowFallbackKey string,
) bool {
	if approval == nil {
		return true
	}
	approvedAt, err := time.Parse(time.RFC3339Nano, approval.ApprovedAt)
	if err != nil || approvedAt.IsZero() || approvedAt.Location() != time.UTC ||
		workflowFallbackKey == "" ||
		approval.Version < 1 || approval.Version > 1_000_000 ||
		!validGlobalReadPageID(approval.ApprovalID) ||
		!validGlobalReadPageID(approval.ActorRef) ||
		!validSHA256Digest(approval.SourceBindingDigest) ||
		!validSHA256Digest(approval.TargetBindingDigest) ||
		approval.SourceBindingDigest == approval.TargetBindingDigest ||
		!validSHA256Digest(approval.Digest) {
		return false
	}
	hash := sha256.New()
	fields := []string{
		"loom.team-fallback-approval.v1",
		fmt.Sprint(approval.Version),
		approval.ApprovalID,
		approval.ActorRef,
		approvedAt.Format(time.RFC3339Nano),
		approval.SourceBindingDigest,
		approval.TargetBindingDigest,
	}
	var length [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil)) == approval.Digest
}

func validProjectedRecoveryFallbackApproval(
	action string,
	version int,
	approvalID, approvalDigest, sourceDigest, targetDigest string,
	node *TeamExecutionNode,
	attempt *TeamExecutionAttempt,
) bool {
	if node == nil || attempt == nil {
		return false
	}
	if action != "fallback" || !node.FallbackApprovalAvailable {
		return version == 0 && approvalID == "" && approvalDigest == "" &&
			sourceDigest == "" && targetDigest == ""
	}
	return version == node.FallbackApprovalVersion &&
		approvalID == node.FallbackApprovalID &&
		approvalDigest == node.FallbackApprovalDigest &&
		sourceDigest == node.FallbackSourceBindingDigest &&
		targetDigest == node.FallbackTargetBindingDigest &&
		attempt.ExecutionBindingAvailable &&
		attempt.ExecutionBinding.BindingDigest == sourceDigest
}

func parseProjectedRetryAt(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Location() != time.UTC {
		return time.Time{}, ErrInvalidProjectionEvent
	}
	return parsed, nil
}

func validProjectedTeamTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "degraded", "blocked", "human_required", "cancelled":
		return true
	default:
		return false
	}
}

func validProjectedOutputClassification(value string) bool {
	switch value {
	case "valid_nonempty", "valid_empty", "transient_empty", "invalid":
		return true
	default:
		return false
	}
}
