package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/contextcapsule"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
)

const missionRoleContextTokenBudget = 2048

type missionWorkPackagePolicyContext struct {
	SchemaVersion                  int                    `json:"schema_version"`
	WorkPackageID                  string                 `json:"work_package_id"`
	WorkPackageVersion             int                    `json:"work_package_version"`
	WorkPackageDigest              string                 `json:"work_package_digest"`
	DomainKind                     work.WorkPackageDomain `json:"domain_kind"`
	ToolCategories                 []string               `json:"tool_categories"`
	DefaultVerifierKey             string                 `json:"default_verifier_key"`
	EvidenceTypes                  []string               `json:"evidence_types"`
	DefaultCustomerRuleTemplateIDs []string               `json:"default_customer_rule_template_ids"`
}

type missionCurrentTaskContext struct {
	SchemaVersion     int    `json:"schema_version"`
	MissionID         string `json:"mission_id"`
	TeamInstanceID    string `json:"team_instance_id"`
	PlanDigest        string `json:"plan_digest"`
	WorkPackageID     string `json:"work_package_id"`
	WorkPackageDigest string `json:"work_package_digest"`
	Operation         string `json:"operation"`
	LogicalNodeID     string `json:"logical_node_id"`
}

type missionRoleGovernanceContext struct {
	SchemaVersion       int                 `json:"schema_version"`
	LogicalNodeID       string              `json:"logical_node_id"`
	Title               string              `json:"title"`
	Role                teams.ExecutionRole `json:"role"`
	DependsOn           []string            `json:"depends_on"`
	MaxAttempts         int                 `json:"max_attempts"`
	ExecutionPlanDigest string              `json:"execution_plan_digest"`
}

type missionFallbackApprovalContext struct {
	SchemaVersion       int    `json:"schema_version"`
	ApprovalID          string `json:"approval_id"`
	ApprovalVersion     int    `json:"approval_version"`
	ApprovedAt          string `json:"approved_at"`
	ApprovalDigest      string `json:"approval_digest"`
	SourceBindingDigest string `json:"source_binding_digest"`
	TargetBindingDigest string `json:"target_binding_digest"`
}

type missionWorkspaceSnapshotContext struct {
	SchemaVersion  int    `json:"schema_version"`
	SnapshotKind   string `json:"snapshot_kind"`
	TreeDigest     string `json:"tree_digest"`
	EntryCount     int    `json:"entry_count"`
	FileCount      int    `json:"file_count"`
	DirectoryCount int    `json:"directory_count"`
	TotalBytes     int64  `json:"total_bytes"`
}

func buildMissionRoleContextCapsule(
	command MissionExecutionCommand,
	plan teams.ExecutionPlan,
	workPackage work.WorkPackage,
	role MissionExecutionRoleBinding,
	profile loomruntime.RuntimeProfile,
	attemptNumber int,
	fallbackApproval work.TeamFallbackApproval,
	workspaceSnapshot supervisor.SourceSnapshot,
) (contextcapsule.RoleContextCapsule, error) {
	node, found := missionContextPlanNode(plan, role.LogicalNodeID)
	if !found || attemptNumber < 1 || attemptNumber > node.MaxAttempts() ||
		node.AgentInstanceID() != role.AgentInstanceID ||
		node.Title() != role.Title || node.Role() != role.Role ||
		!slices.Equal(node.DependsOn(), role.DependsOn) ||
		workPackage.ID() != command.WorkPackageID || !workspaceSnapshot.Valid() ||
		workPackage.Digest() != command.WorkPackageDigest {
		return contextcapsule.RoleContextCapsule{}, ErrInvalidMissionExecution
	}

	items := make([]contextcapsule.ItemInput, 0,
		5+len(command.ConfirmedConstraints)+len(command.AcceptedDecisions)+
			len(node.AssetRevisionBindings()))
	items = append(items, contextcapsule.ItemInput{
		ItemID: "mission-objective", Kind: contextcapsule.KindConversationGoal,
		Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
		Priority: contextcapsule.PrioritySystem, Required: true,
		Content: []byte(command.Objective), TokenCount: missionContextTokenCount([]byte(command.Objective)),
		SourceType: contextcapsule.SourceAuthority, SourceRef: "mission:" + command.MissionID,
	})
	for index, constraint := range command.ConfirmedConstraints {
		items = append(items, missionAuthorityContextItem(
			fmt.Sprintf("confirmed-constraint-%03d", index),
			contextcapsule.KindConfirmedConstraint,
			contextcapsule.ScopeTeamShared,
			contextcapsule.PriorityConfirmed,
			[]byte(constraint),
			fmt.Sprintf("mission-context:%s:constraint:%03d", command.MissionID, index),
		))
	}
	for index, decision := range command.AcceptedDecisions {
		items = append(items, missionAuthorityContextItem(
			fmt.Sprintf("accepted-decision-%03d", index),
			contextcapsule.KindAcceptedDecision,
			contextcapsule.ScopeTeamShared,
			contextcapsule.PriorityConfirmed,
			[]byte(decision),
			fmt.Sprintf("mission-context:%s:decision:%03d", command.MissionID, index),
		))
	}

	policy, err := missionContextJSON(missionWorkPackagePolicyContext{
		SchemaVersion: workPackage.SchemaVersion(), WorkPackageID: workPackage.ID(),
		WorkPackageVersion: workPackage.Version(), WorkPackageDigest: workPackage.Digest(),
		DomainKind: workPackage.DomainKind(), ToolCategories: workPackage.ToolCategories(),
		DefaultVerifierKey: workPackage.DefaultVerifierKey(), EvidenceTypes: workPackage.EvidenceTypes(),
		DefaultCustomerRuleTemplateIDs: workPackage.DefaultCustomerRuleTemplateIDs(),
	})
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, err
	}
	items = append(items, missionAuthorityContextItem(
		"work-package-policy", contextcapsule.KindSystemPolicy,
		contextcapsule.ScopeTeamShared, contextcapsule.PrioritySystem, policy,
		fmt.Sprintf("work-package:%s:%d:%s", workPackage.ID(), workPackage.Version(), workPackage.Digest()),
	))

	currentTask, err := missionContextJSON(missionCurrentTaskContext{
		SchemaVersion: 1, MissionID: command.MissionID, TeamInstanceID: plan.TeamInstanceID(),
		PlanDigest: plan.Digest(), WorkPackageID: workPackage.ID(),
		WorkPackageDigest: workPackage.Digest(), Operation: command.Operation,
		LogicalNodeID: node.LogicalNodeID(),
	})
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, err
	}
	items = append(items, missionAuthorityContextItem(
		"current-task-state", contextcapsule.KindCurrentTaskState,
		contextcapsule.ScopeRoleRestricted, contextcapsule.PriorityConfirmed, currentTask,
		"execution-plan:"+plan.Digest()+":"+node.LogicalNodeID(),
	))
	items[len(items)-1].AllowedRoleID = role.LogicalNodeID

	workspace, err := missionContextJSON(missionWorkspaceSnapshotContext{
		SchemaVersion: 1, SnapshotKind: "managed_source_baseline",
		TreeDigest: workspaceSnapshot.TreeDigest(),
		EntryCount: workspaceSnapshot.EntryCount(), FileCount: workspaceSnapshot.FileCount(),
		DirectoryCount: workspaceSnapshot.DirectoryCount(), TotalBytes: workspaceSnapshot.TotalBytes(),
	})
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, err
	}
	items = append(items, contextcapsule.ItemInput{
		ItemID: "workspace-snapshot", Kind: contextcapsule.KindWorkspaceSnapshot,
		Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeTeamShared,
		Priority: contextcapsule.PriorityWorkspace, Required: true,
		TokenCount: missionContextTokenCount(workspace), Content: workspace,
		SourceType: contextcapsule.SourceObservation,
		SourceRef:  "managed-source:" + workspaceSnapshot.TreeDigest(),
	})

	governance, err := missionContextJSON(missionRoleGovernanceContext{
		SchemaVersion: 1, LogicalNodeID: node.LogicalNodeID(), Title: node.Title(),
		Role: node.Role(), DependsOn: nonNilMissionContextStrings(node.DependsOn()),
		MaxAttempts: node.MaxAttempts(), ExecutionPlanDigest: plan.Digest(),
	})
	if err != nil {
		return contextcapsule.RoleContextCapsule{}, err
	}
	items = append(items, missionAuthorityContextItem(
		"role-governance", contextcapsule.KindGovernanceState,
		contextcapsule.ScopeRoleRestricted, contextcapsule.PriorityConfirmed, governance,
		"execution-plan:"+plan.Digest()+":"+node.LogicalNodeID(),
	))
	items[len(items)-1].AllowedRoleID = role.LogicalNodeID

	artifactRefs := make([]string, 0, len(node.AssetRevisionBindings()))
	for index, binding := range node.AssetRevisionBindings() {
		content, marshalErr := missionContextJSON(binding)
		if marshalErr != nil {
			return contextcapsule.RoleContextCapsule{}, marshalErr
		}
		artifactRef := missionContextArtifactRef(binding)
		artifactRefs = append(artifactRefs, artifactRef)
		items = append(items, contextcapsule.ItemInput{
			ItemID: fmt.Sprintf("artifact-%03d", index), Kind: contextcapsule.KindArtifactReference,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeArtifactScoped,
			Priority: contextcapsule.PriorityWorkspace, TokenCount: missionContextTokenCount(content),
			Content: content, SourceType: contextcapsule.SourceAuthority,
			SourceRef: "asset-revision:" + binding.SHA256Digest, ArtifactRef: artifactRef,
		})
	}

	if attemptNumber == 2 && fallbackApproval.Valid() {
		approval, marshalErr := missionContextJSON(missionFallbackApprovalContext{
			SchemaVersion: 1, ApprovalID: fallbackApproval.ApprovalID(),
			ApprovalVersion:     fallbackApproval.Version(),
			ApprovedAt:          fallbackApproval.ApprovedAt().Format(time.RFC3339Nano),
			ApprovalDigest:      fallbackApproval.Digest(),
			SourceBindingDigest: fallbackApproval.SourceBindingDigest(),
			TargetBindingDigest: fallbackApproval.TargetBindingDigest(),
		})
		if marshalErr != nil {
			return contextcapsule.RoleContextCapsule{}, marshalErr
		}
		items = append(items, missionAuthorityContextItem(
			"fallback-route-approval", contextcapsule.KindAcceptedDecision,
			contextcapsule.ScopeRoleRestricted, contextcapsule.PriorityConfirmed, approval,
			"fallback-approval:"+fallbackApproval.ApprovalID()+":"+fallbackApproval.Digest(),
		))
		items[len(items)-1].AllowedRoleID = role.LogicalNodeID
	}

	return contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "mission:" + command.MissionID,
			TeamID:         plan.TeamInstanceID(), AgentID: role.AgentInstanceID,
			RoleID: role.LogicalNodeID, ProviderID: profile.ProviderID,
			ProviderAccountID: profile.ProviderAccountID, ModelID: profile.ModelID,
			AuthMode: string(profile.AuthMode), ContextAdapterID: "context:" + profile.AdapterType + ":v1",
			DisclosurePolicyID: "loom.local-team-disclosure", DisclosurePolicyVersion: 1,
			ArtifactRefs: artifactRefs, TokenBudget: missionRoleContextTokenBudget,
		},
		items,
	)
}

func missionAuthorityContextItem(
	itemID string,
	kind contextcapsule.ItemKind,
	scope contextcapsule.Scope,
	priority contextcapsule.Priority,
	content []byte,
	sourceRef string,
) contextcapsule.ItemInput {
	return contextcapsule.ItemInput{
		ItemID: itemID, Kind: kind, Trust: contextcapsule.TrustAuthoritative,
		Scope: scope, Priority: priority, TokenCount: missionContextTokenCount(content),
		Required: true, Content: content, SourceType: contextcapsule.SourceAuthority,
		SourceRef: sourceRef,
	}
}

func missionContextPlanNode(
	plan teams.ExecutionPlan,
	logicalNodeID string,
) (teams.ExecutionNode, bool) {
	for _, node := range plan.Nodes() {
		if node.LogicalNodeID() == logicalNodeID {
			return node, true
		}
	}
	return teams.ExecutionNode{}, false
}

func missionContextArtifactRef(binding assets.ExactAssetRevisionBinding) string {
	return fmt.Sprintf(
		"asset:%s:%s:%s:%s",
		binding.AssetKind, binding.DefinitionID, binding.RevisionID, binding.SHA256Digest,
	)
}

func missionContextJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) == 0 {
		return nil, ErrInvalidMissionExecution
	}
	return encoded, nil
}

func missionContextTokenCount(content []byte) int {
	count := (utf8.RuneCount(content) + 3) / 4
	if count < 1 {
		return 1
	}
	return count
}

func nonNilMissionContextStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string{}, values...)
}
