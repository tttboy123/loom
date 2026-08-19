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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/assets"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
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

const MissionContextVersion = 1

const (
	maxMissionExecutionTextBytes          = 4096
	maxMissionExecutionListItems          = 64
	maxMissionExecutionNodeCount          = 16
	maxAuthoritativeMissionFlights        = 64
	missionExecutionPreflightTTL          = 5 * time.Minute
	missionExecutionPreflight             = "preflight"
	missionExecutionStart                 = "start"
	missionExecutionControl               = "control"
	localPiProviderID                     = "loom-local"
	localPiAdapterModelID                 = "qwen2.5-coder-1.5b-instruct-q4-k-m"
	missionSkillMaterializationCapability = "loom.skill-materialization.pi.v1"
)

var ErrInvalidMissionExecution = errors.New("invalid mission execution")

var ErrMissionExecutionConflict = errors.New("mission execution conflict")

var ErrMissionExecutionBusy = errors.New("mission execution busy")

type MissionExecutionCommand struct {
	SchemaVersion        int      `json:"schema_version"`
	Operation            string   `json:"operation"`
	MissionID            string   `json:"mission_id"`
	TeamInstanceID       string   `json:"team_instance_id"`
	WorkPackageID        string   `json:"work_package_id,omitempty"`
	WorkPackageDigest    string   `json:"work_package_digest,omitempty"`
	Objective            string   `json:"objective,omitempty"`
	ContextVersion       int      `json:"context_version,omitempty"`
	ConfirmedConstraints []string `json:"confirmed_constraints,omitempty"`
	AcceptedDecisions    []string `json:"accepted_decisions,omitempty"`
	ExpectedViewVersion  string   `json:"expected_view_version"`
	PreflightDigest      string   `json:"preflight_digest,omitempty"`
	ControlAction        string   `json:"control_action,omitempty"`
	ExecutionDigest      string   `json:"execution_digest,omitempty"`
	LogicalNodeID        string   `json:"logical_node_id,omitempty"`
	AttemptNumber        int      `json:"attempt_number,omitempty"`
	ClaimGeneration      int64    `json:"claim_generation,omitempty"`
	CorrelationID        string   `json:"correlation_id"`
}

type MissionExecutionNodePreview struct {
	LogicalNodeID                 string   `json:"logical_node_id"`
	Title                         string   `json:"title"`
	Role                          string   `json:"role"`
	Kind                          string   `json:"kind,omitempty"`
	RouteGroupID                  string   `json:"route_group_id,omitempty"`
	DependsOn                     []string `json:"depends_on"`
	MaxAttempts                   int      `json:"max_attempts"`
	HarnessAdapter                string   `json:"harness_adapter"`
	ProviderID                    string   `json:"provider_id"`
	ProviderAccountID             string   `json:"provider_account_id"`
	ModelID                       string   `json:"model_id"`
	AuthMode                      string   `json:"auth_mode"`
	CredentialRevision            int64    `json:"credential_revision"`
	ReasoningEffort               string   `json:"reasoning_effort"`
	TimeoutSeconds                int64    `json:"timeout_seconds"`
	BudgetCredits                 *int64   `json:"budget_credits"`
	Capabilities                  []string `json:"capabilities"`
	Status                        string   `json:"status"`
	BlockReason                   string   `json:"block_reason"`
	FallbackConfigured            bool     `json:"fallback_configured"`
	FallbackHarnessAdapter        string   `json:"fallback_harness_adapter"`
	FallbackProviderID            string   `json:"fallback_provider_id"`
	FallbackProviderAccountID     string   `json:"fallback_provider_account_id"`
	FallbackModelID               string   `json:"fallback_model_id"`
	FallbackAuthMode              string   `json:"fallback_auth_mode"`
	FallbackCredentialRevision    int64    `json:"fallback_credential_revision"`
	FallbackReasoningEffort       string   `json:"fallback_reasoning_effort"`
	FallbackTimeoutSeconds        int64    `json:"fallback_timeout_seconds"`
	FallbackBudgetCredits         *int64   `json:"fallback_budget_credits"`
	FallbackCapabilities          []string `json:"fallback_capabilities"`
	FallbackStatus                string   `json:"fallback_status"`
	FallbackBlockReason           string   `json:"fallback_block_reason"`
	FallbackApprovalRequired      bool     `json:"fallback_approval_required"`
	FallbackApprovalAvailable     bool     `json:"fallback_approval_available"`
	FallbackApprovalVersion       int      `json:"fallback_approval_version"`
	RemoteToolEnrollmentAvailable bool     `json:"remote_tool_enrollment_available,omitempty"`
	RemoteToolEnrollmentID        string   `json:"remote_tool_enrollment_id,omitempty"`
	RemoteToolBackendKind         string   `json:"remote_tool_backend_kind,omitempty"`
	RemoteToolBindingDigest       string   `json:"remote_tool_binding_digest,omitempty"`
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
	Plan              teams.ExecutionPlan
	Preflight         MissionExecutionPreflight
	Request           TeamExecutionRequest
	FallbackDecisions []MissionFallbackDecisionCandidate
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

// MissionContextCapsuleStore persists encrypted dispatch context without
// becoming an execution authority. A failed write prevents dispatch assembly.
type MissionContextCapsuleStore interface {
	PutRoleContextCapsule(
		context.Context,
		contextcapsule.RoleContextCapsule,
		[]byte,
	) error
	ReadRoleContextCapsule(
		context.Context,
		contextcapsule.AuthorityRecord,
	) (contextcapsule.RoleContextCapsule, []byte, error)
	ListRoleContextCapsuleAuthorities(
		context.Context,
		string,
	) ([]contextcapsule.AuthorityRecord, error)
}

type MissionExecutionBinding struct {
	ViewVersion                  string
	TeamInstanceID               string
	Roles                        []MissionExecutionRoleBinding
	AgentInstanceID              string
	Profile                      loomruntime.RuntimeProfile
	Instance                     loomruntime.RuntimeInstance
	CapacityAvailable            int
	SavedTeamAssetBindings       []assets.ExactAssetRevisionBinding
	TeamDefinitionAssetBindings  []assets.ExactAssetRevisionBinding
	AgentDefinitionAssetBindings []assets.ExactAssetRevisionBinding
	AssetBindingRecords          []assets.EvolutionAssetBindingRecord
	AssetSourceStreamIDs         []string
}

type MissionExecutionRoleBinding struct {
	LogicalNodeID                 string
	Title                         string
	Role                          teams.ExecutionRole
	Kind                          teams.ExecutionNodeKind
	RouteGroupID                  string
	Aggregation                   bool
	DependsOn                     []string
	AgentInstanceID               string
	Profile                       loomruntime.RuntimeProfile
	Instance                      loomruntime.RuntimeInstance
	CapacityAvailable             int
	SavedTeamAssetBindings        []assets.ExactAssetRevisionBinding
	TeamDefinitionAssetBindings   []assets.ExactAssetRevisionBinding
	AgentDefinitionAssetBindings  []assets.ExactAssetRevisionBinding
	AssetBindingRecords           []assets.EvolutionAssetBindingRecord
	AssetSourceStreamIDs          []string
	Status                        string
	BlockReason                   string
	BlockCode                     string
	BlockStage                    string
	BlockRetryable                bool
	FallbackConfigured            bool
	FallbackProfile               loomruntime.RuntimeProfile
	FallbackInstance              loomruntime.RuntimeInstance
	FallbackCapacityAvailable     int
	FallbackStatus                string
	FallbackBlockReason           string
	FallbackApprovalRequired      bool
	RemoteToolEnrollmentAvailable bool
	RemoteToolEnrollmentID        string
	RemoteToolEnrollmentDigest    string
	RemoteToolBackendKind         string
	RemoteToolBindingDigest       string
}

type MissionExecutionBindingSource interface {
	ResolveMissionExecutionBinding(
		context.Context,
		string,
	) (MissionExecutionBinding, error)
}

type MissionFallbackApprovalQuery struct {
	TeamInstanceID      string
	PlanDigest          string
	LogicalNodeID       string
	SourceBindingDigest string
	TargetBindingDigest string
}

type MissionFallbackApprovalSource interface {
	ResolveMissionFallbackApproval(
		context.Context,
		MissionFallbackApprovalQuery,
	) (work.TeamFallbackApproval, bool, error)
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

func (source *ProjectionMissionExecutionBindingSource) ResolveMissionFallbackApproval(
	ctx context.Context,
	query MissionFallbackApprovalQuery,
) (work.TeamFallbackApproval, bool, error) {
	if source == nil || source.projection == nil || ctx == nil {
		return work.TeamFallbackApproval{}, false, ErrInvalidMissionExecution
	}
	if err := ctx.Err(); err != nil {
		return work.TeamFallbackApproval{}, false, err
	}
	scope, err := work.NewTeamFallbackDecisionScope(
		work.TeamFallbackDecisionScopeInput{
			Version: 1, TeamInstanceID: query.TeamInstanceID,
			PlanDigest: query.PlanDigest, LogicalNodeID: query.LogicalNodeID,
			SourceBindingDigest: query.SourceBindingDigest,
			TargetBindingDigest: query.TargetBindingDigest,
		},
	)
	if err != nil {
		return work.TeamFallbackApproval{}, false, ErrInvalidMissionExecution
	}
	record, ok := source.projection.MissionFallbackDecision(scope)
	if !ok || record.Decision != "approved" {
		return work.TeamFallbackApproval{}, false, nil
	}
	if !record.Approval.Valid() ||
		record.Approval.SourceBindingDigest() != query.SourceBindingDigest ||
		record.Approval.TargetBindingDigest() != query.TargetBindingDigest {
		return work.TeamFallbackApproval{}, false, ErrMissionExecutionConflict
	}
	return record.Approval, true, nil
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
	definition, definitionAvailable := view.TeamDefinition(team.TeamDefinitionID)
	exactDefinition := definitionAvailable &&
		definition.Version == team.TeamDefinitionVersion &&
		definition.Scope == team.TeamDefinitionScope &&
		definition.DefinitionDigest == team.TeamDefinitionDigest
	if exactDefinition {
		if missionExecutionDefinitionHasCompleteProfiles(definition) {
			return resolveConfiguredMissionExecutionBinding(
				view, team, agent, definition, recovery,
			)
		}
		if len(definition.Configuration.RoleBindings) > 1 ||
			len(team.DormantSubAgents) > 0 {
			return MissionExecutionBinding{}, ErrMissionExecutionConflict
		}
	}
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
	assetBindings, err := resolveMissionExecutionAssetBindings(
		view, team, agent,
	)
	if err != nil {
		return MissionExecutionBinding{}, err
	}
	return MissionExecutionBinding{
		ViewVersion: view.Version(), TeamInstanceID: teamInstanceID,
		AgentInstanceID: agent.ID, Profile: profile, Instance: instance,
		CapacityAvailable:            available,
		SavedTeamAssetBindings:       assetBindings.savedTeam,
		TeamDefinitionAssetBindings:  assetBindings.teamDefinition,
		AgentDefinitionAssetBindings: assetBindings.agentDefinition,
		AssetBindingRecords:          assetBindings.records,
		AssetSourceStreamIDs:         assetBindings.sourceStreams,
	}, nil
}

func missionExecutionDefinitionHasCompleteProfiles(
	definition projection.TeamDefinitionRecord,
) bool {
	if len(definition.Configuration.RoleBindings) == 0 ||
		len(definition.Configuration.RoleBindings) > teams.MaxTeamAgentCount {
		return false
	}
	for _, role := range definition.Configuration.RoleBindings {
		if !role.ExecutionProfileAvailable {
			return false
		}
	}
	return true
}

type missionExecutionProfileResolution struct {
	profile              loomruntime.RuntimeProfile
	instance             loomruntime.RuntimeInstance
	capacityAvailable    int
	status               string
	blockReason          string
	blockCode            string
	blockStage           string
	blockRetryable       bool
	discoveryDigest      string
	remoteToolEnrollment missionExecutionEnrollmentResolution
}

type missionExecutionEnrollmentResolution struct {
	available        bool
	enrollmentID     string
	enrollmentDigest string
	backendKind      string
	bindingDigest    string
	blocked          bool
	blockReason      string
	blockCode        string
	blockStage       string
	blockRetryable   bool
}

func resolveMissionExecutionProfile(
	view projection.GlobalReadView,
	teamInstanceID string,
	profileRecord projection.TeamExecutionProfileRecord,
	runtimeInstanceID string,
	recovery bool,
) (missionExecutionProfileResolution, error) {
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: profileRecord.ID, AdapterType: profileRecord.HarnessAdapter,
		ProviderID:        profileRecord.ProviderID,
		ProviderAccountID: profileRecord.ProviderAccountID,
		ModelID:           profileRecord.ModelID, AuthMode: profileRecord.AuthMode,
		EndpointFingerprint: profileRecord.EndpointFingerprint,
		CredentialReference: profileRecord.CredentialReference,
		CredentialRevision:  profileRecord.CredentialRevision,
		ReasoningEffort:     profileRecord.ReasoningEffort,
		RequiredCapabilities: append(
			[]string{}, profileRecord.RequiredCapabilities...,
		),
		Timeout:                    profileRecord.Timeout,
		Budget:                     profileRecord.Budget,
		RemoteToolEnrollmentID:     profileRecord.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: profileRecord.RemoteToolEnrollmentDigest,
	})
	if err != nil {
		return missionExecutionProfileResolution{}, ErrMissionExecutionConflict
	}
	status := "ready"
	blockReason := ""
	blockCode := ""
	blockStage := ""
	blockRetryable := false
	if profile.AuthMode == loomruntime.AuthBrokered {
		credential, found := view.ProviderAccountCredential(
			profile.ProviderID, profile.ProviderAccountID,
		)
		switch {
		case !found:
			status = "blocked"
			blockReason = "Credential is not configured. Reconnect this Provider Account."
			blockCode, blockStage, blockRetryable = "credential_unavailable", "credential_lease_issue", true
		case credential.Status != string(credentials.CredentialVerified):
			status = "blocked"
			blockReason = "Credential is not verified. Reconnect this Provider Account."
			blockCode, blockStage, blockRetryable = "credential_unavailable", "credential_lease_issue", true
		case credential.CredentialReference != profile.CredentialReference ||
			credential.Revision != profile.CredentialRevision:
			status = "blocked"
			blockReason = "Credential changed after this Agent was configured. Review its binding."
			blockCode, blockStage = "credential_revision_conflict", "agent_attempt_dispatch"
		}
	}
	projectedRuntime, found := view.RuntimeInstance(runtimeInstanceID)
	if !found || projectedRuntime.ID != runtimeInstanceID ||
		projectedRuntime.AdapterType != profile.AdapterType ||
		projectedRuntime.Capacity < 1 {
		return missionExecutionProfileResolution{}, ErrMissionExecutionConflict
	}
	if projectedRuntime.Status != string(loomruntime.RuntimeOnline) && status == "ready" {
		status = "blocked"
		blockReason = "Harness runtime is not online. Restart it, then retry preflight."
		blockCode, blockStage, blockRetryable = "runtime_unavailable", "agent_attempt_dispatch", true
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: projectedRuntime.ID, DeviceID: projectedRuntime.DeviceID,
		AdapterType:       projectedRuntime.AdapterType,
		DisplayName:       projectedRuntime.DisplayName,
		ExecutableVersion: projectedRuntime.ExecutableVersion,
		Status:            loomruntime.RuntimeStatus(projectedRuntime.Status),
		ObservedCapabilities: append(
			[]string{}, projectedRuntime.ObservedCapabilities...,
		),
		Capacity: projectedRuntime.Capacity,
	})
	if err != nil {
		return missionExecutionProfileResolution{}, ErrMissionExecutionConflict
	}
	if candidate, validateErr := loomruntime.ValidateBinding(
		profile,
		instance,
	); (validateErr != nil || !candidate.Accepted) && status == "ready" {
		status = "blocked"
		blockReason = "Harness capabilities do not satisfy this Agent binding."
		blockCode, blockStage = "incompatible_binding", "agent_attempt_dispatch"
	}
	enrollment := resolveMissionExecutionEnrollment(view, profile)
	if enrollment.blocked && status == "ready" {
		status = "blocked"
		blockReason = enrollment.blockReason
		blockCode, blockStage, blockRetryable = enrollment.blockCode, enrollment.blockStage, enrollment.blockRetryable
	}
	available := instance.Capacity - view.ActiveRunCount(instance.ID)
	if available < 1 {
		if recovery && missionExecutionOwnsActiveCapacity(
			view, teamInstanceID, instance.ID,
		) {
			available = 1
		} else if status == "ready" {
			status = "blocked"
			blockReason = "Harness capacity is currently full. Retry when an Attempt finishes."
			blockCode, blockStage, blockRetryable = "capacity_unavailable", "agent_attempt_dispatch", true
		}
		if available < 0 {
			available = 0
		}
	}
	return missionExecutionProfileResolution{
		profile: profile, instance: instance, capacityAvailable: available,
		status: status, blockReason: blockReason,
		blockCode: blockCode, blockStage: blockStage, blockRetryable: blockRetryable,
		discoveryDigest:      projectedRuntime.DiscoveryDigest,
		remoteToolEnrollment: enrollment,
	}, nil
}

// resolveMissionExecutionEnrollment validates the Agent's selected Remote Tool
// Enrollment against the authoritative read view. Failures block only this
// role with a closed safe diagnostic; they never mark a Team or peer Agent
// offline and never expose Enrollment input or tool names.
func resolveMissionExecutionEnrollment(
	view projection.GlobalReadView,
	profile loomruntime.RuntimeProfile,
) missionExecutionEnrollmentResolution {
	if profile.RemoteToolEnrollmentID == "" && profile.RemoteToolEnrollmentDigest == "" {
		return missionExecutionEnrollmentResolution{}
	}
	if profile.RemoteToolEnrollmentID == "" || profile.RemoteToolEnrollmentDigest == "" {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment binding is incomplete. Review this Agent's binding.",
			blockCode:   "tool_enrollment_incomplete", blockStage: "agent_attempt_dispatch",
		}
	}
	enrollment, found := missionRemoteToolEnrollment(
		view, profile.ProviderID, profile.ProviderAccountID,
		profile.RemoteToolEnrollmentID,
	)
	if !found {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment is not available. Reconnect or re-select it for this Agent.",
			blockCode:   "tool_enrollment_unavailable", blockStage: "agent_attempt_dispatch",
			blockRetryable: true,
		}
	}
	if enrollment.Status() != work.RemoteToolBackendEnrollmentActive {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment was revoked. Restore or re-select it for this Agent.",
			blockCode:   "tool_enrollment_revoked", blockStage: "agent_attempt_dispatch",
		}
	}
	policy, policyOK := view.ProviderAccountPolicy(
		profile.ProviderID, profile.ProviderAccountID,
	)
	if !policyOK ||
		!remoteToolBackendEnrollmentUsesPolicy(enrollment, policy) {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment policy changed. Review and rebind this Agent.",
			blockCode:   "tool_enrollment_policy_drift", blockStage: "agent_attempt_dispatch",
			blockRetryable: true,
		}
	}
	if enrollment.Digest() != profile.RemoteToolEnrollmentDigest {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment changed after this Agent was configured. Review its binding.",
			blockCode:   "tool_enrollment_revision_conflict", blockStage: "agent_attempt_dispatch",
		}
	}
	catalog := work.BuiltInRemoteToolBackendCatalog()
	if !catalog.RemoteToolBackendAdapterSupported(enrollment.AdapterID()) {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment backend is not supported by this Loom build.",
			blockCode:   "tool_enrollment_adapter_unsupported", blockStage: "agent_attempt_dispatch",
		}
	}
	frozen, err := work.FreezeAgentRemoteToolEnrollment(
		work.AgentRemoteToolEnrollmentSelection{
			EnrollmentID:      profile.RemoteToolEnrollmentID,
			ProviderID:        profile.ProviderID,
			ProviderAccountID: profile.ProviderAccountID,
			ExpectedDigest:    profile.RemoteToolEnrollmentDigest,
		},
		enrollment,
		policyOK && remoteToolBackendEnrollmentUsesPolicy(enrollment, policy),
		catalog,
	)
	if err != nil {
		return missionExecutionEnrollmentResolution{
			blocked:     true,
			blockReason: "Remote tool enrollment could not be frozen. Review this Agent's binding.",
			blockCode:   "tool_enrollment_unavailable", blockStage: "agent_attempt_dispatch",
		}
	}
	return missionExecutionEnrollmentResolution{
		available: true, enrollmentID: frozen.EnrollmentID,
		enrollmentDigest: frozen.EnrollmentDigest,
		backendKind:      frozen.BackendKind, bindingDigest: frozen.BindingDigest,
	}
}

func missionRemoteToolEnrollment(
	view projection.GlobalReadView,
	providerID string,
	providerAccountID string,
	enrollmentID string,
) (work.RemoteToolBackendEnrollment, bool) {
	for _, enrollment := range view.RemoteToolBackendEnrollments(
		providerID, providerAccountID,
	) {
		if enrollment.EnrollmentID() == enrollmentID && enrollment.Valid() {
			return enrollment, true
		}
	}
	return work.RemoteToolBackendEnrollment{}, false
}

func resolveConfiguredMissionExecutionBinding(
	view projection.GlobalReadView,
	team projection.TeamInstance,
	mainAgent projection.AgentInstance,
	definition projection.TeamDefinitionRecord,
	recovery bool,
) (MissionExecutionBinding, error) {
	if definition.ID != team.TeamDefinitionID ||
		definition.Version != team.TeamDefinitionVersion ||
		definition.Scope != team.TeamDefinitionScope ||
		definition.DefinitionDigest != team.TeamDefinitionDigest ||
		len(definition.Configuration.RoleBindings) !=
			len(definition.Roles) {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	if mainAgent.ID == "" || mainAgent.State != "created" ||
		mainAgent.SourcePlanDigest != team.SourcePlanDigest ||
		mainAgent.SourceRecordSetDigest != team.SourceRecordSetDigest ||
		!validSHA256(mainAgent.RuntimeDiscoveryDigest) {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	dormantByDefinition := make(map[string]projection.DormantSubAgent)
	for _, dormant := range team.DormantSubAgents {
		dormantByDefinition[dormant.AgentDefinitionID] = dormant
	}
	roles := make([]MissionExecutionRoleBinding, 0, len(definition.Configuration.RoleBindings))
	subNodeIDs := make([]string, 0, len(definition.Configuration.RoleBindings)-1)
	mainCount := 0
	for _, configured := range definition.Configuration.RoleBindings {
		roleKind := teams.ExecutionRole(configured.Kind)
		if roleKind != teams.ExecutionRoleMain &&
			roleKind != teams.ExecutionRoleSubAgent {
			return MissionExecutionBinding{}, ErrMissionExecutionConflict
		}
		if configured.RuntimeProfileID != configured.ExecutionProfile.ID ||
			configured.ModelID != configured.ExecutionProfile.ModelID ||
			configured.RuntimeInstanceID == "" {
			return MissionExecutionBinding{}, ErrMissionExecutionConflict
		}
		resolved, err := resolveMissionExecutionProfile(
			view,
			team.ID,
			configured.ExecutionProfile,
			configured.RuntimeInstanceID,
			recovery,
		)
		if err != nil {
			return MissionExecutionBinding{}, err
		}
		profile := resolved.profile
		instance := resolved.instance
		available := resolved.capacityAvailable
		status := resolved.status
		blockReason := resolved.blockReason
		fallback := missionExecutionProfileResolution{}
		fallbackConfigured := configured.FallbackRouteAvailable
		if fallbackConfigured {
			route := configured.FallbackRoute
			if route.Version != 1 || !route.ApprovalRequired ||
				route.RuntimeProfileID != route.ExecutionProfile.ID ||
				route.ModelID != route.ExecutionProfile.ModelID ||
				route.RuntimeProfileID == configured.RuntimeProfileID ||
				route.RuntimeInstanceID == "" {
				return MissionExecutionBinding{}, ErrMissionExecutionConflict
			}
			fallback, err = resolveMissionExecutionProfile(
				view,
				team.ID,
				route.ExecutionProfile,
				route.RuntimeInstanceID,
				recovery,
			)
			if err != nil {
				return MissionExecutionBinding{}, err
			}
		}
		logicalNodeID := "main"
		agentInstanceID := mainAgent.ID
		var assetAgent *projection.AgentInstance
		if roleKind == teams.ExecutionRoleMain {
			mainCount++
			if mainAgent.AgentDefinitionID != configured.AgentDefinitionID ||
				mainAgent.RuntimeProfileID != configured.RuntimeProfileID ||
				mainAgent.RuntimeInstanceID != configured.RuntimeInstanceID ||
				!mainAgent.RuntimeBinding.Accepted ||
				mainAgent.RuntimeBinding.ProfileID != configured.RuntimeProfileID ||
				mainAgent.RuntimeBinding.InstanceID != configured.RuntimeInstanceID ||
				resolved.discoveryDigest != mainAgent.RuntimeDiscoveryDigest {
				return MissionExecutionBinding{}, ErrMissionExecutionConflict
			}
			assetAgent = &mainAgent
		} else {
			dormant, found := dormantByDefinition[configured.AgentDefinitionID]
			if !found || !dormant.Dormant ||
				dormant.RuntimeProfileID != configured.RuntimeProfileID ||
				dormant.RuntimeInstanceID != configured.RuntimeInstanceID {
				return MissionExecutionBinding{}, ErrMissionExecutionConflict
			}
			logicalNodeID = appVerifierIdentity(
				"mission-role", team.ID, configured.AgentDefinitionID,
			)
			agentInstanceID = appVerifierIdentity(
				"mission-agent", team.ID, configured.AgentDefinitionID,
				configured.RuntimeProfileID,
			)
			subNodeIDs = append(subNodeIDs, logicalNodeID)
		}
		assetBindings, err := resolveMissionExecutionRoleAssetBindings(
			view, team, configured, assetAgent,
		)
		if err != nil {
			return MissionExecutionBinding{}, err
		}
		baseRole := MissionExecutionRoleBinding{
			LogicalNodeID: logicalNodeID, Title: missionExecutionRoleTitle(
				definition, configured.AgentDefinitionID,
			),
			Role: roleKind, DependsOn: []string{},
			AgentInstanceID: agentInstanceID, Profile: profile, Instance: instance,
			CapacityAvailable:             available,
			SavedTeamAssetBindings:        assetBindings.savedTeam,
			TeamDefinitionAssetBindings:   assetBindings.teamDefinition,
			AgentDefinitionAssetBindings:  assetBindings.agentDefinition,
			AssetBindingRecords:           assetBindings.records,
			AssetSourceStreamIDs:          assetBindings.sourceStreams,
			Status:                        status,
			BlockReason:                   blockReason,
			BlockCode:                     resolved.blockCode,
			BlockStage:                    resolved.blockStage,
			BlockRetryable:                resolved.blockRetryable,
			FallbackConfigured:            fallbackConfigured,
			FallbackProfile:               fallback.profile,
			FallbackInstance:              fallback.instance,
			FallbackCapacityAvailable:     fallback.capacityAvailable,
			FallbackStatus:                fallback.status,
			FallbackBlockReason:           fallback.blockReason,
			FallbackApprovalRequired:      fallbackConfigured,
			RemoteToolEnrollmentAvailable: resolved.remoteToolEnrollment.available,
			RemoteToolEnrollmentID:        resolved.remoteToolEnrollment.enrollmentID,
			RemoteToolEnrollmentDigest:    resolved.remoteToolEnrollment.enrollmentDigest,
			RemoteToolBackendKind:         resolved.remoteToolEnrollment.backendKind,
			RemoteToolBindingDigest:       resolved.remoteToolEnrollment.bindingDigest,
		}
		expanded, err := expandConfiguredParallelRouteSet(
			view, team, configured, baseRole, recovery,
		)
		if err != nil {
			return MissionExecutionBinding{}, err
		}
		roles = append(roles, expanded...)
	}
	if mainCount != 1 || len(roles) < 1+len(team.DormantSubAgents) ||
		len(roles) > teams.MaxExecutionNodeCount {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	sort.Strings(subNodeIDs)
	mainIndex := -1
	for index := range roles {
		if roles[index].Role == teams.ExecutionRoleMain &&
			roles[index].Kind != teams.ExecutionNodeAggregation {
			roles[index].DependsOn = append([]string{}, subNodeIDs...)
		}
		if roles[index].Role == teams.ExecutionRoleMain &&
			roles[index].Kind != teams.ExecutionNodeRouteSibling {
			mainIndex = index
		}
	}
	if mainIndex < 0 {
		return MissionExecutionBinding{}, ErrMissionExecutionConflict
	}
	main := roles[mainIndex]
	return MissionExecutionBinding{
		ViewVersion: view.Version(), TeamInstanceID: team.ID, Roles: roles,
		AgentInstanceID: main.AgentInstanceID, Profile: main.Profile,
		Instance: main.Instance, CapacityAvailable: main.CapacityAvailable,
		SavedTeamAssetBindings:       main.SavedTeamAssetBindings,
		TeamDefinitionAssetBindings:  main.TeamDefinitionAssetBindings,
		AgentDefinitionAssetBindings: main.AgentDefinitionAssetBindings,
		AssetBindingRecords:          main.AssetBindingRecords,
		AssetSourceStreamIDs:         main.AssetSourceStreamIDs,
	}, nil
}

func expandConfiguredParallelRouteSet(
	view projection.GlobalReadView,
	team projection.TeamInstance,
	configured projection.TeamConfigurationRoleBinding,
	base MissionExecutionRoleBinding,
	recovery bool,
) ([]MissionExecutionRoleBinding, error) {
	if !configured.ParallelRouteSetAvailable {
		return []MissionExecutionRoleBinding{base}, nil
	}
	routeSet := configured.ParallelRouteSet
	if configured.FallbackRouteAvailable || routeSet.Version != 1 ||
		len(routeSet.AdditionalRoutes) < 1 || len(routeSet.AdditionalRoutes) > 2 {
		return nil, ErrMissionExecutionConflict
	}
	groupID := appVerifierIdentity(
		"mission-route-group", team.ID, configured.AgentDefinitionID,
		strconv.Itoa(routeSet.Version),
	)
	siblings := make([]MissionExecutionRoleBinding, 0, len(routeSet.AdditionalRoutes)+1)
	appendSibling := func(
		profile loomruntime.RuntimeProfile,
		instance loomruntime.RuntimeInstance,
		resolved missionExecutionProfileResolution,
	) {
		sibling := base
		sibling.LogicalNodeID = appVerifierIdentity(
			"mission-route", team.ID, configured.AgentDefinitionID,
			profile.ID,
		)
		sibling.Kind = teams.ExecutionNodeRouteSibling
		sibling.RouteGroupID = groupID
		sibling.Aggregation = false
		sibling.DependsOn = []string{}
		sibling.Profile = profile
		sibling.Instance = instance
		sibling.CapacityAvailable = resolved.capacityAvailable
		sibling.Status = resolved.status
		sibling.BlockReason = resolved.blockReason
		sibling.BlockCode = resolved.blockCode
		sibling.BlockStage = resolved.blockStage
		sibling.BlockRetryable = resolved.blockRetryable
		sibling.FallbackConfigured = false
		sibling.FallbackProfile = loomruntime.RuntimeProfile{}
		sibling.FallbackInstance = loomruntime.RuntimeInstance{}
		sibling.FallbackCapacityAvailable = 0
		sibling.FallbackStatus = ""
		sibling.FallbackBlockReason = ""
		sibling.FallbackApprovalRequired = false
		siblings = append(siblings, sibling)
	}
	primaryResolution := missionExecutionProfileResolution{
		profile: base.Profile, instance: base.Instance,
		capacityAvailable: base.CapacityAvailable,
		status:            base.Status, blockReason: base.BlockReason,
		blockCode: base.BlockCode, blockStage: base.BlockStage,
		blockRetryable: base.BlockRetryable,
	}
	appendSibling(base.Profile, base.Instance, primaryResolution)
	for _, route := range routeSet.AdditionalRoutes {
		if route.Version != 1 || route.RuntimeProfileID != route.ExecutionProfile.ID ||
			route.ModelID != route.ExecutionProfile.ModelID ||
			route.RuntimeProfileID == configured.RuntimeProfileID ||
			route.RuntimeInstanceID == "" {
			return nil, ErrMissionExecutionConflict
		}
		resolved, err := resolveMissionExecutionProfile(
			view, team.ID, route.ExecutionProfile,
			route.RuntimeInstanceID, recovery,
		)
		if err != nil {
			return nil, err
		}
		appendSibling(resolved.profile, resolved.instance, resolved)
	}
	synthesisRoute := routeSet.SynthesisRoute
	if synthesisRoute.Version != 1 ||
		synthesisRoute.RuntimeProfileID != synthesisRoute.ExecutionProfile.ID ||
		synthesisRoute.ModelID != synthesisRoute.ExecutionProfile.ModelID ||
		synthesisRoute.RuntimeInstanceID == "" {
		return nil, ErrMissionExecutionConflict
	}
	synthesis, err := resolveMissionExecutionProfile(
		view, team.ID, synthesisRoute.ExecutionProfile,
		synthesisRoute.RuntimeInstanceID, recovery,
	)
	if err != nil {
		return nil, err
	}
	aggregation := base
	aggregation.Kind = teams.ExecutionNodeAggregation
	aggregation.RouteGroupID = groupID
	aggregation.Aggregation = true
	aggregation.Profile = synthesis.profile
	aggregation.Instance = synthesis.instance
	aggregation.CapacityAvailable = synthesis.capacityAvailable
	aggregation.Status = synthesis.status
	aggregation.BlockReason = synthesis.blockReason
	aggregation.BlockCode = synthesis.blockCode
	aggregation.BlockStage = synthesis.blockStage
	aggregation.BlockRetryable = synthesis.blockRetryable
	aggregation.FallbackConfigured = false
	aggregation.FallbackProfile = loomruntime.RuntimeProfile{}
	aggregation.FallbackInstance = loomruntime.RuntimeInstance{}
	aggregation.FallbackCapacityAvailable = 0
	aggregation.FallbackStatus = ""
	aggregation.FallbackBlockReason = ""
	aggregation.FallbackApprovalRequired = false
	aggregation.DependsOn = make([]string, len(siblings))
	for index := range siblings {
		aggregation.DependsOn[index] = siblings[index].LogicalNodeID
	}
	sort.Strings(aggregation.DependsOn)
	return append(siblings, aggregation), nil
}

func missionExecutionRoleTitle(
	definition projection.TeamDefinitionRecord,
	agentDefinitionID string,
) string {
	for _, role := range definition.Roles {
		if role.AgentDefinitionID == agentDefinitionID {
			return role.Responsibility
		}
	}
	return ""
}

type missionExecutionAssetBindingContext struct {
	savedTeam       []assets.ExactAssetRevisionBinding
	teamDefinition  []assets.ExactAssetRevisionBinding
	agentDefinition []assets.ExactAssetRevisionBinding
	records         []assets.EvolutionAssetBindingRecord
	sourceStreams   []string
}

func resolveMissionExecutionAssetBindings(
	view projection.GlobalReadView,
	team projection.TeamInstance,
	agent projection.AgentInstance,
) (missionExecutionAssetBindingContext, error) {
	configured := projection.TeamConfigurationRoleBinding{
		Kind: "main", AgentDefinitionID: agent.AgentDefinitionID,
	}
	if definition, ok := view.TeamDefinition(team.TeamDefinitionID); ok {
		for _, candidate := range definition.Configuration.RoleBindings {
			if candidate.Kind == "main" &&
				candidate.AgentDefinitionID == agent.AgentDefinitionID {
				configured = candidate
				break
			}
		}
	}
	return resolveMissionExecutionRoleAssetBindings(
		view, team, configured, &agent,
	)
}

func resolveMissionExecutionRoleAssetBindings(
	view projection.GlobalReadView,
	team projection.TeamInstance,
	configured projection.TeamConfigurationRoleBinding,
	agent *projection.AgentInstance,
) (missionExecutionAssetBindingContext, error) {
	context := missionExecutionAssetBindingContext{
		savedTeam:       []assets.ExactAssetRevisionBinding{},
		teamDefinition:  []assets.ExactAssetRevisionBinding{},
		agentDefinition: []assets.ExactAssetRevisionBinding{},
		records:         []assets.EvolutionAssetBindingRecord{},
		sourceStreams:   []string{"team-definition/" + team.TeamDefinitionID},
	}
	definition, ok := view.TeamDefinition(team.TeamDefinitionID)
	if ok && (definition.Version != team.TeamDefinitionVersion ||
		definition.DefinitionDigest != team.TeamDefinitionDigest ||
		definition.Scope != team.TeamDefinitionScope) {
		return missionExecutionAssetBindingContext{}, ErrMissionExecutionConflict
	}
	for _, skill := range configured.SkillRevisions {
		revisionID := strconv.Itoa(skill.Revision)
		assetDefinition, found := view.EvolutionAssetDefinition(
			"skill/" + skill.ID,
		)
		revision, revisionFound := view.EvolutionAssetRevision(
			"skill/" + skill.ID + "/" + revisionID,
		)
		if !found || !revisionFound ||
			assetDefinition.ActiveRevisionID != revisionID ||
			revision.ArtifactDigest != skill.Digest {
			return missionExecutionAssetBindingContext{}, ErrMissionExecutionConflict
		}
		context.savedTeam = append(context.savedTeam, assets.ExactAssetRevisionBinding{
			AssetKind: assets.AssetKindSkill, DefinitionID: skill.ID,
			RevisionID: revisionID, SHA256Digest: skill.Digest,
			SourceScope: revision.SourceScope,
		})
	}
	records, _ := view.EvolutionAssetBindings("", 64)
	context.records = append(context.records, records...)
	for _, record := range records {
		switch {
		case record.SubjectKind == "team_definition" &&
			record.SubjectID == team.TeamDefinitionID &&
			record.SubjectVersion == int64(team.TeamDefinitionVersion) &&
			record.SubjectDigest == team.TeamDefinitionDigest &&
			record.SubjectScope == team.TeamDefinitionScope:
			context.teamDefinition = append(
				context.teamDefinition, record.Bindings...,
			)
			context.sourceStreams = append(context.sourceStreams,
				"evolution-asset-binding/team_definition/"+record.SubjectIdentityDigest)
		case agent != nil && record.SubjectKind == "agent_definition" &&
			record.SubjectID == agent.AgentDefinitionID &&
			record.SubjectVersion == int64(agent.AgentDefinitionVersion) &&
			record.SubjectScope == agent.AgentDefinitionScope &&
			record.SubjectProjectID == agent.ScopeIdentity.ProjectID &&
			record.SubjectGenerationID == agent.ScopeIdentity.GenerationID:
			context.agentDefinition = append(
				context.agentDefinition, record.Bindings...,
			)
			context.sourceStreams = append(context.sourceStreams,
				"evolution-asset-binding/agent_definition/"+record.SubjectIdentityDigest)
		}
	}
	sort.Strings(context.sourceStreams)
	return context, nil
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
	Bindings          MissionExecutionBindingSource
	FallbackApprovals MissionFallbackApprovalSource
	ContextCapsules   MissionContextCapsuleStore
	SourcePath        string
	OutputObserver    NodeOutputObserver
	ObserverFactory   MissionExecutionObserverFactory
	Now               func() time.Time
}

type MissionExecutionObserverFactory interface {
	MissionExecutionObserver(
		context.Context,
		string,
	) (NodeOutputObserver, error)
}

type BuiltInMissionExecutionCompiler struct {
	bindings          MissionExecutionBindingSource
	fallbackApprovals MissionFallbackApprovalSource
	contextCapsules   MissionContextCapsuleStore
	sourcePath        string
	outputObserver    NodeOutputObserver
	observerFactory   MissionExecutionObserverFactory
	now               func() time.Time
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
		config.FallbackApprovals != nil &&
			nilMissionExecutionInterface(config.FallbackApprovals) ||
		config.ContextCapsules != nil &&
			nilMissionExecutionInterface(config.ContextCapsules) ||
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
		fallbackApprovals: config.FallbackApprovals,
		contextCapsules:   config.ContextCapsules,
		outputObserver:    config.OutputObserver,
		observerFactory:   config.ObserverFactory,
		now:               config.Now,
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
	return compiler.compileMissionExecution(
		ctx, command, binding, command.Operation == missionExecutionStart,
	)
}

func (compiler *BuiltInMissionExecutionCompiler) compileMissionExecution(
	ctx context.Context,
	command MissionExecutionCommand,
	binding MissionExecutionBinding,
	persistContextCapsules bool,
) (MissionExecutionCompilation, error) {
	if binding.ViewVersion != command.ExpectedViewVersion ||
		binding.TeamInstanceID != command.TeamInstanceID {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	legacySingleRole := len(binding.Roles) == 0
	roles := missionExecutionRoleBindings(binding, command.Objective)
	if len(roles) == 0 || len(roles) > teams.MaxExecutionNodeCount {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	mainRoleIndex := -1
	for index := range roles {
		role := roles[index]
		if role.Kind == teams.ExecutionNodeAgent &&
			(role.RouteGroupID != "" || role.Aggregation) ||
			role.Kind == teams.ExecutionNodeRouteSibling &&
				(role.RouteGroupID == "" || role.Aggregation || role.FallbackConfigured) ||
			role.Kind == teams.ExecutionNodeAggregation &&
				(role.RouteGroupID == "" || !role.Aggregation || role.FallbackConfigured) {
			return MissionExecutionCompilation{}, ErrMissionExecutionConflict
		}
		if role.Status == "blocked" && role.BlockCode == "" {
			role.BlockCode = "agent_unavailable"
			role.BlockStage = "agent_attempt_dispatch"
			role.BlockRetryable = true
			roles[index] = role
		}
		if role.AgentInstanceID == "" ||
			(role.Status != "ready" && role.Status != "blocked") ||
			(role.Status == "ready" && role.BlockReason != "") ||
			(role.Status == "blocked" && role.BlockReason == "") ||
			role.CapacityAvailable < 0 ||
			role.CapacityAvailable > role.Instance.Capacity ||
			role.Status == "ready" && role.CapacityAvailable < 1 {
			return MissionExecutionCompilation{}, ErrMissionExecutionConflict
		}
		validated, validateErr := loomruntime.ValidateBinding(role.Profile, role.Instance)
		if role.Status == "ready" && (validateErr != nil || !validated.Accepted ||
			validated.ProfileID != role.Profile.ID ||
			validated.InstanceID != role.Instance.ID) {
			return MissionExecutionCompilation{}, errors.Join(
				ErrMissionExecutionConflict,
				validateErr,
			)
		}
		if role.Status == "ready" &&
			(role.BlockCode != "" || role.BlockStage != "" || role.BlockRetryable) ||
			role.Status == "blocked" &&
				(role.BlockCode == "" || role.BlockStage == "") {
			return MissionExecutionCompilation{}, ErrMissionExecutionConflict
		}
		if role.FallbackConfigured {
			if !role.FallbackApprovalRequired ||
				role.FallbackProfile.ID == role.Profile.ID ||
				(role.FallbackStatus != "ready" && role.FallbackStatus != "blocked") ||
				(role.FallbackStatus == "ready" && role.FallbackBlockReason != "") ||
				(role.FallbackStatus == "blocked" && role.FallbackBlockReason == "") ||
				role.FallbackCapacityAvailable < 0 ||
				role.FallbackCapacityAvailable > role.FallbackInstance.Capacity ||
				role.FallbackStatus == "ready" && role.FallbackCapacityAvailable < 1 {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			fallbackBinding, fallbackErr := loomruntime.ValidateBinding(
				role.FallbackProfile,
				role.FallbackInstance,
			)
			if role.FallbackStatus == "ready" &&
				(fallbackErr != nil || !fallbackBinding.Accepted ||
					fallbackBinding.ProfileID != role.FallbackProfile.ID ||
					fallbackBinding.InstanceID != role.FallbackInstance.ID) {
				return MissionExecutionCompilation{}, errors.Join(
					ErrMissionExecutionConflict,
					fallbackErr,
				)
			}
		} else if role.FallbackApprovalRequired ||
			role.FallbackStatus != "" || role.FallbackBlockReason != "" {
			return MissionExecutionCompilation{}, ErrMissionExecutionConflict
		}
		if role.Role == teams.ExecutionRoleMain &&
			role.Kind != teams.ExecutionNodeRouteSibling {
			if mainRoleIndex >= 0 {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			mainRoleIndex = index
		}
	}
	if mainRoleIndex < 0 {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	workPackage, err := missionExecutionWorkPackage(command.WorkPackageID)
	if err != nil || workPackage.Digest() != command.WorkPackageDigest {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	assetSourceStreamSet := make(map[string]struct{})
	planNodes := make([]teams.ExecutionNodeInput, 0, len(roles))
	for index := range roles {
		role := &roles[index]
		workPackageBindings := []assets.ExactAssetRevisionBinding{}
		for _, streamID := range role.AssetSourceStreamIDs {
			assetSourceStreamSet[streamID] = struct{}{}
		}
		for _, record := range role.AssetBindingRecords {
			if record.SubjectKind != "work_package" ||
				record.SubjectID != workPackage.ID() ||
				record.SubjectVersion != int64(workPackage.Version()) ||
				record.SubjectDigest != workPackage.Digest() ||
				record.SubjectScope != "builtin" {
				continue
			}
			if len(workPackageBindings) != 0 {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			workPackageBindings = append(workPackageBindings, record.Bindings...)
			assetSourceStreamSet["evolution-asset-binding/work_package/"+record.SubjectIdentityDigest] = struct{}{}
		}
		mergedAssetBindings, mergedAssetDigest, mergeErr := teams.MergeExecutionAssetBindings(
			role.SavedTeamAssetBindings,
			role.TeamDefinitionAssetBindings,
			role.AgentDefinitionAssetBindings,
			workPackageBindings,
		)
		if mergeErr != nil {
			return MissionExecutionCompilation{}, ErrMissionExecutionConflict
		}
		if len(mergedAssetBindings) > 0 {
			profileInput := role.Profile
			profileInput.RequiredCapabilities = append(
				[]string{}, profileInput.RequiredCapabilities...,
			)
			profileInput.RequiredCapabilities = append(
				profileInput.RequiredCapabilities,
				missionSkillMaterializationCapability,
			)
			profile, profileErr := loomruntime.NewRuntimeProfile(profileInput)
			if profileErr != nil {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			if validated, validateErr := loomruntime.ValidateBinding(profile, role.Instance); role.Status == "ready" && (validateErr != nil || !validated.Accepted) {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			role.Profile = profile
			if role.FallbackConfigured {
				fallbackInput := role.FallbackProfile
				fallbackInput.RequiredCapabilities = append(
					[]string{}, fallbackInput.RequiredCapabilities...,
				)
				fallbackInput.RequiredCapabilities = append(
					fallbackInput.RequiredCapabilities,
					missionSkillMaterializationCapability,
				)
				fallbackProfile, fallbackErr := loomruntime.NewRuntimeProfile(
					fallbackInput,
				)
				if fallbackErr != nil {
					return MissionExecutionCompilation{}, ErrMissionExecutionConflict
				}
				if validated, validateErr := loomruntime.ValidateBinding(
					fallbackProfile,
					role.FallbackInstance,
				); role.FallbackStatus == "ready" &&
					(validateErr != nil || !validated.Accepted) {
					return MissionExecutionCompilation{}, ErrMissionExecutionConflict
				}
				role.FallbackProfile = fallbackProfile
			}
		}
		planNodes = append(planNodes, teams.ExecutionNodeInput{
			LogicalNodeID: role.LogicalNodeID, Title: role.Title,
			AgentInstanceID: role.AgentInstanceID, RuntimeInstanceID: role.Instance.ID,
			Role: role.Role, Kind: role.Kind, RouteGroupID: role.RouteGroupID,
			DependsOn:   append([]string{}, role.DependsOn...),
			MaxAttempts: 2, AssetRevisionBindings: mergedAssetBindings,
			AssetRevisionSetDigest: mergedAssetDigest,
		})
	}
	assetSourceStreams := make([]string, 0, len(assetSourceStreamSet))
	for streamID := range assetSourceStreamSet {
		assetSourceStreams = append(assetSourceStreams, streamID)
	}
	sort.Strings(assetSourceStreams)
	plan, err := teams.BuildExecutionPlan(teams.ExecutionPlanInput{
		TeamInstanceID: command.TeamInstanceID,
		Nodes:          planNodes,
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
	workspaceSnapshot, err := supervisor.ObserveSourceSnapshot(compiler.sourcePath)
	if err != nil {
		return MissionExecutionCompilation{}, errors.Join(
			ErrInvalidMissionExecution,
			err,
		)
	}
	executions := make([]TeamNodeExecution, 0, len(roles)*2)
	semantics := make([]TeamNodeSemantics, 0, len(roles))
	preflightNodes := make([]MissionExecutionNodePreview, 0, len(roles))
	fallbackDecisions := make(
		[]MissionFallbackDecisionCandidate,
		0,
		len(roles),
	)
	for roleIndex := range roles {
		role := &roles[roleIndex]
		attemptTwoProfile := role.Profile
		attemptTwoInstance := role.Instance
		attemptTwoWorkflowPath := "builtin/mission-primary-v1"
		roleRecoveryPolicy := recoveryPolicy
		fallbackApproval := work.TeamFallbackApproval{}
		if role.FallbackConfigured {
			sourceBinding, freezeErr := loomruntime.FreezeExecutionBinding(
				role.Profile,
				role.Instance,
			)
			if freezeErr != nil {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			targetBinding, freezeErr := loomruntime.FreezeExecutionBinding(
				role.FallbackProfile,
				role.FallbackInstance,
			)
			if freezeErr != nil {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			fallbackScope, scopeErr := work.NewTeamFallbackDecisionScope(
				work.TeamFallbackDecisionScopeInput{
					Version: 1, TeamInstanceID: plan.TeamInstanceID(),
					PlanDigest: plan.Digest(), LogicalNodeID: role.LogicalNodeID,
					SourceBindingDigest: sourceBinding.BindingDigest,
					TargetBindingDigest: targetBinding.BindingDigest,
				},
			)
			if scopeErr != nil {
				return MissionExecutionCompilation{}, ErrMissionExecutionConflict
			}
			fallbackDecisions = append(
				fallbackDecisions,
				MissionFallbackDecisionCandidate{
					MissionID: command.MissionID, Scope: fallbackScope,
					AgentTitle:         role.Title,
					HarnessAdapter:     role.FallbackProfile.AdapterType,
					ProviderID:         role.FallbackProfile.ProviderID,
					ProviderAccountID:  role.FallbackProfile.ProviderAccountID,
					ModelID:            role.FallbackProfile.ModelID,
					CredentialRevision: role.FallbackProfile.CredentialRevision,
				},
			)
			if compiler.fallbackApprovals != nil {
				query := MissionFallbackApprovalQuery{
					TeamInstanceID:      fallbackScope.TeamInstanceID(),
					PlanDigest:          fallbackScope.PlanDigest(),
					LogicalNodeID:       fallbackScope.LogicalNodeID(),
					SourceBindingDigest: fallbackScope.SourceBindingDigest(),
					TargetBindingDigest: fallbackScope.TargetBindingDigest(),
				}
				approval, found, approvalErr := compiler.fallbackApprovals.
					ResolveMissionFallbackApproval(ctx, query)
				if approvalErr != nil {
					return MissionExecutionCompilation{}, approvalErr
				}
				if found {
					if !approval.Valid() ||
						approval.SourceBindingDigest() != query.SourceBindingDigest ||
						approval.TargetBindingDigest() != query.TargetBindingDigest ||
						approval.ApprovedAt().After(now) {
						return MissionExecutionCompilation{}, ErrMissionExecutionConflict
					}
					if role.FallbackStatus != "ready" {
						role.Status = "blocked"
						role.BlockReason = "Approved fallback is unavailable. " +
							role.FallbackBlockReason
						role.BlockCode = "fallback_unavailable"
						role.BlockStage = "agent_attempt_dispatch"
						role.BlockRetryable = true
					} else {
						roleRecoveryPolicy, approvalErr = rules.NewRecoveryPolicy(
							rules.RecoveryPolicyInput{
								Version: 2, RetryDelay: 0, AttemptCredits: 1,
								ExhaustionAction:    rules.ExhaustionBlocked,
								RetryInvalid:        true,
								WorkflowFallbackKey: "builtin/mission-fallback-v1",
							},
						)
						if approvalErr != nil {
							return MissionExecutionCompilation{}, approvalErr
						}
						fallbackApproval = approval
						attemptTwoProfile = role.FallbackProfile
						attemptTwoInstance = role.FallbackInstance
						attemptTwoWorkflowPath = "builtin/mission-fallback-v1"
					}
				}
			}
		}
		for attemptNumber := 1; attemptNumber <= 2; attemptNumber++ {
			executionProfile := role.Profile
			executionInstance := role.Instance
			workflowPath := "builtin/mission-primary-v1"
			if attemptNumber == 2 {
				executionProfile = attemptTwoProfile
				executionInstance = attemptTwoInstance
				workflowPath = attemptTwoWorkflowPath
			}
			capsule, capsuleErr := buildMissionRoleContextCapsule(
				command, plan, workPackage, *role, executionProfile,
				attemptNumber, fallbackApproval, workspaceSnapshot,
			)
			if capsuleErr != nil {
				return MissionExecutionCompilation{}, errors.Join(
					ErrInvalidMissionExecution,
					capsuleErr,
				)
			}
			payload, renderErr := contextcapsule.RenderDispatchPayload(capsule)
			if renderErr != nil {
				return MissionExecutionCompilation{}, errors.Join(
					ErrInvalidMissionExecution,
					renderErr,
				)
			}
			if persistContextCapsules && compiler.contextCapsules != nil {
				if storeErr := compiler.contextCapsules.PutRoleContextCapsule(
					ctx, capsule, payload,
				); storeErr != nil {
					return MissionExecutionCompilation{}, errors.Join(
						ErrInvalidMissionExecution,
						storeErr,
					)
				}
			}
			workItemID := appTeamAttemptIdentity(
				"work", plan, role.LogicalNodeID, attemptNumber,
			)
			runID := appTeamAttemptIdentity(
				"run", plan, role.LogicalNodeID, attemptNumber,
			)
			dispatch, frameErr := bridgev1.NewFrame(bridgev1.FrameInput{
				MessageID: appVerifierUUID(
					"mission-dispatch", plan.TeamInstanceID(), plan.Digest(),
					role.LogicalNodeID, strconv.Itoa(attemptNumber),
				),
				CorrelationID: command.CorrelationID,
				WorkItemID:    workItemID, RunID: runID, ClaimGeneration: 1,
				RuntimeInstanceID:     executionInstance.ID,
				SenderAgentInstanceID: role.AgentInstanceID,
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
				LogicalNodeID: role.LogicalNodeID, AttemptNumber: attemptNumber,
				WorkflowPath:         workflowPath,
				SourcePath:           compiler.sourcePath,
				SourceSnapshotDigest: workspaceSnapshot.TreeDigest(),
				Profile:              executionProfile,
				Instance:             executionInstance, Dispatch: dispatch, Executor: executor,
				ContextCapsule: capsule,
			})
			if role.Aggregation {
				executions[len(executions)-1].Aggregation = &TeamAggregationExecution{
					MaxSourceArtifactBytes: maxTeamAggregationSourceBytes,
				}
			}
		}
		verifierIdentityFields := []string{plan.TeamInstanceID(), plan.Digest()}
		if !legacySingleRole {
			verifierIdentityFields = append(verifierIdentityFields, role.LogicalNodeID)
		}
		verifierAgentID := appVerifierIdentity(
			"mission-verifier-agent", verifierIdentityFields...,
		)
		semantics = append(semantics, TeamNodeSemantics{
			LogicalNodeID:  role.LogicalNodeID,
			OutputContract: outputContract, RecoveryPolicy: roleRecoveryPolicy,
			AcceptanceContract:        acceptanceContract,
			PrimaryWorkflowPath:       "builtin/mission-primary-v1",
			FallbackApproval:          fallbackApproval,
			VerifierAgentInstanceID:   verifierAgentID,
			VerifierRuntimeInstanceID: role.Instance.ID,
			VerifierWorkflowPath:      "builtin/mission-verifier-v1",
			VerifierExecution: &TeamVerifierExecution{
				SourcePath:           compiler.sourcePath,
				SourceSnapshotDigest: workspaceSnapshot.TreeDigest(),
				Profile:              role.Profile,
				Instance:             role.Instance, Executor: executor,
			},
		})
		preflightNodes = append(preflightNodes, MissionExecutionNodePreview{
			LogicalNodeID: role.LogicalNodeID, Title: role.Title,
			Role: string(role.Role), Kind: string(role.Kind),
			RouteGroupID: role.RouteGroupID,
			DependsOn:    append([]string{}, role.DependsOn...),
			MaxAttempts:  2, HarnessAdapter: role.Profile.AdapterType,
			ProviderID:         role.Profile.ProviderID,
			ProviderAccountID:  role.Profile.ProviderAccountID,
			ModelID:            role.Profile.ModelID,
			AuthMode:           string(role.Profile.AuthMode),
			CredentialRevision: role.Profile.CredentialRevision,
			ReasoningEffort:    role.Profile.ReasoningEffort,
			TimeoutSeconds:     int64(role.Profile.Timeout / time.Second),
			BudgetCredits:      role.Profile.Budget,
			Capabilities:       append([]string{}, role.Profile.RequiredCapabilities...),
			Status:             role.Status, BlockReason: role.BlockReason,
			FallbackConfigured:         role.FallbackConfigured,
			FallbackHarnessAdapter:     role.FallbackProfile.AdapterType,
			FallbackProviderID:         role.FallbackProfile.ProviderID,
			FallbackProviderAccountID:  role.FallbackProfile.ProviderAccountID,
			FallbackModelID:            role.FallbackProfile.ModelID,
			FallbackAuthMode:           string(role.FallbackProfile.AuthMode),
			FallbackCredentialRevision: role.FallbackProfile.CredentialRevision,
			FallbackReasoningEffort:    role.FallbackProfile.ReasoningEffort,
			FallbackTimeoutSeconds: int64(
				role.FallbackProfile.Timeout / time.Second,
			),
			FallbackBudgetCredits: role.FallbackProfile.Budget,
			FallbackCapabilities: append(
				[]string{}, role.FallbackProfile.RequiredCapabilities...,
			),
			FallbackStatus:                role.FallbackStatus,
			FallbackBlockReason:           role.FallbackBlockReason,
			FallbackApprovalRequired:      role.FallbackApprovalRequired,
			FallbackApprovalAvailable:     fallbackApproval.Valid(),
			FallbackApprovalVersion:       fallbackApproval.Version(),
			RemoteToolEnrollmentAvailable: role.RemoteToolEnrollmentAvailable,
			RemoteToolEnrollmentID:        role.RemoteToolEnrollmentID,
			RemoteToolBackendKind:         role.RemoteToolBackendKind,
			RemoteToolBindingDigest:       role.RemoteToolBindingDigest,
		})
	}
	directBlocks := make([]teams.InitialExecutionBlock, 0, len(roles))
	for _, role := range roles {
		if role.Status != "blocked" {
			continue
		}
		directBlocks = append(directBlocks, teams.InitialExecutionBlock{
			LogicalNodeID: role.LogicalNodeID, Code: role.BlockCode,
			Stage: role.BlockStage, Reason: role.BlockReason,
			Retryable: role.BlockRetryable,
		})
	}
	initialBlocks, err := teams.PropagateInitialExecutionBlocks(plan, directBlocks)
	if err != nil {
		return MissionExecutionCompilation{}, ErrMissionExecutionConflict
	}
	routeSummaries := make([]work.TeamNodeRouteSummary, 0, len(roles))
	for _, role := range roles {
		routeSummaries = append(routeSummaries, missionTeamRouteSummary(role))
	}
	sort.Slice(routeSummaries, func(i, j int) bool {
		return routeSummaries[i].LogicalNodeID < routeSummaries[j].LogicalNodeID
	})
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
		RuntimeInstanceID: roles[mainRoleIndex].Instance.ID,
		RuntimeProfileID:  roles[mainRoleIndex].Profile.ID,
		ModelID:           roles[mainRoleIndex].Profile.ModelID,
		AuthMode:          string(roles[mainRoleIndex].Profile.AuthMode),
		CapacityAvailable: roles[mainRoleIndex].CapacityAvailable,
		BudgetStatus:      "unavailable",
		SideEffects:       sideEffects,
		PermissionScopes:  permissionScopes,
		ApprovalPoints:    approvalPoints,
		Nodes:             preflightNodes,
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
		Semantics:            semantics,
		InitialBlocks:        initialBlocks,
		RouteSummaries:       routeSummaries,
		AuthoritativeTime:    now,
		PrepareLeaseDuration: time.Minute,
		GrantLifetime:        2 * time.Minute,
		CorrelationID:        command.CorrelationID,
		OutputObserver:       outputObserver,
		ContextCapsules:      compiler.contextCapsules,
		AssetSourceStreamIDs: assetSourceStreams,
		Objective:            command.Objective,
	}
	return MissionExecutionCompilation{
		Plan: plan, Preflight: preflight, Request: request,
		FallbackDecisions: fallbackDecisions,
	}, nil
}

func missionTeamRouteSummary(role MissionExecutionRoleBinding) work.TeamNodeRouteSummary {
	return work.TeamNodeRouteSummary{
		LogicalNodeID: role.LogicalNodeID, HarnessAdapter: role.Profile.AdapterType,
		ProviderID: role.Profile.ProviderID, ProviderAccountID: role.Profile.ProviderAccountID,
		ModelID: role.Profile.ModelID, ReasoningEffort: role.Profile.ReasoningEffort,
		TimeoutNanoseconds: int64(role.Profile.Timeout), Budget: role.Profile.Budget,
		Capabilities:       append([]string(nil), role.Profile.RequiredCapabilities...),
		CredentialRevision: role.Profile.CredentialRevision,
	}
}

func missionExecutionRoleBindings(
	binding MissionExecutionBinding,
	objective string,
) []MissionExecutionRoleBinding {
	if len(binding.Roles) == 0 {
		return []MissionExecutionRoleBinding{{
			LogicalNodeID: "main", Title: objective, Role: teams.ExecutionRoleMain,
			DependsOn: []string{}, AgentInstanceID: binding.AgentInstanceID,
			Profile: binding.Profile, Instance: binding.Instance,
			CapacityAvailable: binding.CapacityAvailable,
			SavedTeamAssetBindings: append(
				[]assets.ExactAssetRevisionBinding{}, binding.SavedTeamAssetBindings...,
			),
			TeamDefinitionAssetBindings: append(
				[]assets.ExactAssetRevisionBinding{}, binding.TeamDefinitionAssetBindings...,
			),
			AgentDefinitionAssetBindings: append(
				[]assets.ExactAssetRevisionBinding{}, binding.AgentDefinitionAssetBindings...,
			),
			AssetBindingRecords: append(
				[]assets.EvolutionAssetBindingRecord{}, binding.AssetBindingRecords...,
			),
			AssetSourceStreamIDs: append([]string{}, binding.AssetSourceStreamIDs...),
			Status:               "ready",
		}}
	}
	roles := append([]MissionExecutionRoleBinding{}, binding.Roles...)
	for index := range roles {
		roles[index].DependsOn = append([]string{}, roles[index].DependsOn...)
		roles[index].SavedTeamAssetBindings = append(
			[]assets.ExactAssetRevisionBinding{}, roles[index].SavedTeamAssetBindings...,
		)
		roles[index].TeamDefinitionAssetBindings = append(
			[]assets.ExactAssetRevisionBinding{}, roles[index].TeamDefinitionAssetBindings...,
		)
		roles[index].AgentDefinitionAssetBindings = append(
			[]assets.ExactAssetRevisionBinding{}, roles[index].AgentDefinitionAssetBindings...,
		)
		roles[index].AssetBindingRecords = append(
			[]assets.EvolutionAssetBindingRecord{}, roles[index].AssetBindingRecords...,
		)
		roles[index].AssetSourceStreamIDs = append(
			[]string{}, roles[index].AssetSourceStreamIDs...,
		)
		roles[index].FallbackProfile.RequiredCapabilities = append(
			[]string{}, roles[index].FallbackProfile.RequiredCapabilities...,
		)
		if roles[index].Status == "" {
			roles[index].Status = "ready"
		}
		if roles[index].Role == teams.ExecutionRoleMain {
			roles[index].Title = objective
		}
	}
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].LogicalNodeID < roles[j].LogicalNodeID
	})
	return roles
}

func (compiler *BuiltInMissionExecutionCompiler) ReconstructMissionExecution(
	ctx context.Context,
	projected projection.TeamExecution,
) (TeamExecutionRequest, error) {
	if compiler == nil || ctx == nil ||
		projected.TeamInstanceID == "" || !validSHA256(projected.PlanDigest) ||
		appTerminalTeamStatus(projected.Status) || len(projected.Nodes) == 0 ||
		len(projected.Nodes) > teams.MaxExecutionNodeCount {
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
	objective := ""
	for _, node := range projected.Nodes {
		if node.Role == string(teams.ExecutionRoleMain) &&
			node.Kind != string(teams.ExecutionNodeRouteSibling) {
			if objective != "" {
				return TeamExecutionRequest{}, ErrMissionExecutionConflict
			}
			objective = node.Title
		}
	}
	if objective == "" {
		return TeamExecutionRequest{}, ErrMissionExecutionConflict
	}
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
		Objective:           objective,
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
	compilation, err := compiler.compileMissionExecution(ctx, command, binding, false)
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
	if err := compiler.restoreProjectedContextCapsules(
		ctx, &compilation.Request, projected,
	); err != nil {
		return TeamExecutionRequest{}, errors.Join(
			ErrMissionExecutionConflict,
			err,
		)
	}
	if _, err := validateTeamExecutionRequest(ctx, compilation.Request); err != nil {
		return TeamExecutionRequest{}, errors.Join(
			ErrMissionExecutionConflict,
			err,
		)
	}
	return compilation.Request, nil
}

func (compiler *BuiltInMissionExecutionCompiler) restoreProjectedContextCapsules(
	ctx context.Context,
	request *TeamExecutionRequest,
	projected projection.TeamExecution,
) error {
	projectedAuthorities := make(map[string]contextcapsule.AuthorityRecord)
	conversationID := ""
	for _, node := range projected.Nodes {
		for _, attempt := range node.Attempts {
			if !attempt.ContextCapsuleAvailable {
				continue
			}
			if _, err := contextcapsule.ValidateAuthorityRecord(
				attempt.ContextCapsule,
			); err != nil || conversationID != "" &&
				attempt.ContextCapsule.ConversationID != conversationID {
				return errors.Join(ErrMissionExecutionConflict, err)
			}
			conversationID = attempt.ContextCapsule.ConversationID
			projectedAuthorities[appExecutionKey(
				node.LogicalNodeID, attempt.AttemptNumber,
			)] = attempt.ContextCapsule
		}
	}
	if len(projectedAuthorities) == 0 {
		return nil
	}
	if compiler.contextCapsules == nil || conversationID == "" {
		return ErrMissionExecutionConflict
	}
	manifest, err := compiler.contextCapsules.ListRoleContextCapsuleAuthorities(
		ctx, conversationID,
	)
	if err != nil || len(manifest) == 0 {
		return errors.Join(ErrMissionExecutionConflict, err)
	}
	for index := range request.Nodes {
		execution := request.Nodes[index]
		key := appExecutionKey(execution.LogicalNodeID, execution.AttemptNumber)
		authority, exact := projectedAuthorities[key]
		if exact {
			if err := missionProjectedAttemptBindingMatches(
				projected, execution,
			); err != nil {
				return err
			}
		} else {
			matches := make([]contextcapsule.AuthorityRecord, 0, 1)
			for _, candidate := range manifest {
				if missionCapsuleAuthorityMatchesExecution(
					candidate, projected.TeamInstanceID, execution,
				) {
					matches = append(matches, candidate)
				}
			}
			if len(matches) != 1 {
				return ErrMissionExecutionConflict
			}
			authority = matches[0]
		}
		if !missionCapsuleAuthorityMatchesExecution(
			authority, projected.TeamInstanceID, execution,
		) {
			return ErrMissionExecutionConflict
		}
		capsule, payload, err := compiler.contextCapsules.ReadRoleContextCapsule(
			ctx, authority,
		)
		if err != nil || capsule.AuthorityRecord() != authority {
			clearMissionContextPayload(payload)
			return errors.Join(ErrMissionExecutionConflict, err)
		}
		sourceDigest, available, err := appContextSourceSnapshotDigest(capsule)
		if err != nil || !available {
			clearMissionContextPayload(payload)
			return errors.Join(ErrMissionExecutionConflict, err)
		}
		frame, err := missionFrameWithPayload(execution.Dispatch, payload)
		clearMissionContextPayload(payload)
		if err != nil {
			return errors.Join(ErrMissionExecutionConflict, err)
		}
		execution.ContextCapsule = capsule
		execution.SourceSnapshotDigest = sourceDigest
		execution.Dispatch = frame
		request.Nodes[index] = execution
	}
	return nil
}

func missionCapsuleAuthorityMatchesExecution(
	authority contextcapsule.AuthorityRecord,
	teamInstanceID string,
	execution TeamNodeExecution,
) bool {
	return authority.TeamID == teamInstanceID &&
		authority.AgentID == execution.Dispatch.SenderAgentInstanceID() &&
		authority.RoleID == execution.LogicalNodeID &&
		authority.ProviderID == execution.Profile.ProviderID &&
		authority.ProviderAccountID == execution.Profile.ProviderAccountID &&
		authority.ModelID == execution.Profile.ModelID &&
		authority.AuthMode == string(execution.Profile.AuthMode) &&
		authority.ContextAdapterID == "context:"+execution.Profile.AdapterType+":v1"
}

func missionProjectedAttemptBindingMatches(
	projected projection.TeamExecution,
	execution TeamNodeExecution,
) error {
	for _, node := range projected.Nodes {
		if node.LogicalNodeID != execution.LogicalNodeID {
			continue
		}
		for _, attempt := range node.Attempts {
			if attempt.AttemptNumber != execution.AttemptNumber {
				continue
			}
			projectedBinding, err := loomruntime.ValidateFrozenExecutionBinding(
				attempt.ExecutionBinding,
			)
			currentBinding, currentErr := loomruntime.FreezeExecutionBinding(
				execution.Profile, execution.Instance,
			)
			if !attempt.ExecutionBindingAvailable || err != nil || currentErr != nil ||
				projectedBinding.BindingDigest != currentBinding.BindingDigest {
				return errors.Join(ErrMissionExecutionConflict, err, currentErr)
			}
			return nil
		}
	}
	return ErrMissionExecutionConflict
}

func missionFrameWithPayload(
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

func clearMissionContextPayload(payload []byte) {
	for index := range payload {
		payload[index] = 0
	}
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
	for _, candidate := range prepared.Fallbacks {
		controls := []missionExecutionPreparedControl{
			{candidate.Sheet.Kind, "not_now", "not_now", false},
			{candidate.Sheet.Kind, "approve", "approve_fallback", true},
			{candidate.Sheet.Kind, "reject", "reject_fallback", true},
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
		control.kind != "recovery" && control.kind != "fallback" {
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
		controls := router.controls[candidate.DecisionID]
		if len(controls) == 0 && candidate.Kind == "fallback" {
			controls = []missionExecutionPreparedControl{
				{candidate.Kind, "not_now", "not_now", false},
				{candidate.Kind, "approve", "approve_fallback", true},
				{candidate.Kind, "reject", "reject_fallback", true},
			}
		}
		for _, control := range controls {
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

func (state *ProjectionMissionExecutionState) LatestAgentGrantForRun(
	runID string,
) (projection.AgentGrant, bool) {
	if state == nil || state.projection == nil {
		return projection.AgentGrant{}, false
	}
	return state.projection.GlobalReadView().LatestAgentGrantForRun(runID)
}

type AuthoritativeMissionExecutionConfig struct {
	State             MissionExecutionState
	Compiler          MissionExecutionCompiler
	Runner            MissionExecutionRunner
	Decisions         MissionExecutionDecisionRouter
	FallbackDecisions MissionFallbackDecisionPreparer
	ParentGate        ParentContinuationGate
	VisibilityTimeout time.Duration
	Now               func() time.Time
}

type ParentContinuationGate interface {
	AllowParentContinuation(context.Context, string, int64) error
}

type missionExecutionRecoveryState interface {
	TeamExecutions(string, int) ([]projection.TeamExecution, bool)
	Run(string) (projection.Run, bool)
}

type missionExecutionStartLineageState interface {
	Run(string) (projection.Run, bool)
	LatestAgentGrantForRun(string) (projection.AgentGrant, bool)
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
	fallbackDecisions MissionFallbackDecisionPreparer
	parentGate        ParentContinuationGate
	visibilityTimeout time.Duration
	now               func() time.Time
	ctx               context.Context
	cancel            context.CancelFunc

	mu         sync.Mutex
	closed     bool
	flights    map[string]*authoritativeMissionFlight
	preflights map[string]missionExecutionPreflightLease
}

func (backend *AuthoritativeMissionExecutionBackend) ValidateParentExecutionDigest(
	ctx context.Context,
	teamInstanceID string,
	executionDigest string,
) error {
	if backend == nil || ctx == nil || teamInstanceID == "" ||
		!validSHA256(executionDigest) {
		return ErrInvalidSideTaskProduct
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.closed {
		return ErrSideTaskProductUnavailable
	}
	flight := backend.flights[teamInstanceID]
	if flight == nil || flight.executionDigest != executionDigest {
		return ErrSideTaskProductDigestMismatch
	}
	return nil
}

func NewAuthoritativeMissionExecutionBackend(
	config AuthoritativeMissionExecutionConfig,
) (*AuthoritativeMissionExecutionBackend, error) {
	if nilMissionExecutionInterface(config.State) ||
		nilMissionExecutionInterface(config.Compiler) ||
		nilMissionExecutionInterface(config.Runner) ||
		config.Decisions != nil && nilMissionExecutionInterface(config.Decisions) ||
		config.FallbackDecisions != nil &&
			nilMissionExecutionInterface(config.FallbackDecisions) ||
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
		fallbackDecisions: config.FallbackDecisions,
		parentGate:        config.ParentGate,
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
		if missionExecutionAwaitingHumanReview(projected) {
			continue
		}
		if backend.parentGate != nil {
			generation := int64(0)
			for _, node := range projected.Nodes {
				for _, attempt := range node.Attempts {
					if attempt.ClaimGeneration > generation {
						generation = attempt.ClaimGeneration
					}
				}
			}
			if err := backend.parentGate.AllowParentContinuation(ctx, projected.TeamInstanceID, generation); err != nil {
				if errors.Is(err, ErrSideTaskProductConflict) {
					continue
				}
				return err
			}
		}
		if projected.Status != "running" &&
			projected.Status != "awaiting_recovery" {
			// A projected Mission in an unrecognized non-terminal state must
			// not brick daemon startup. Leave it untouched (it stays visible
			// on the Mission Board) instead of failing the whole service.
			continue
		}
		request, err := reconstructor.ReconstructMissionExecution(ctx, projected)
		if err != nil || request.Plan.Digest() != projected.PlanDigest {
			// An unresumable projected Mission (for example its execution
			// binding drifted after the App was killed) must not prevent the
			// daemon from serving. Skip it so the rest of the product starts;
			// the Mission remains visible for governance attention.
			continue
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

func missionExecutionAwaitingHumanReview(
	projected projection.TeamExecution,
) bool {
	if projected.Status != "running" || len(projected.Nodes) == 0 {
		return false
	}
	awaitingReview := false
	for _, node := range projected.Nodes {
		switch node.Status {
		case "ready_for_review":
			awaitingReview = true
		case "succeeded":
		default:
			return false
		}
	}
	return awaitingReview
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
	if backend.fallbackDecisions != nil {
		if err := backend.fallbackDecisions.PrepareMissionFallbackDecisions(
			ctx,
			command.TeamInstanceID,
			preflight.ViewVersion,
			compilation.FallbackDecisions,
		); err != nil {
			return MissionExecutionPreflight{}, err
		}
	}
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
	if backend.parentGate != nil {
		if err := backend.parentGate.AllowParentContinuation(ctx, command.TeamInstanceID, 0); err != nil {
			return MissionExecutionResult{}, err
		}
	}
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
		result, projectedErr := backend.projectedResult(
			command, compilation.Plan, executionDigest, projected,
		)
		if !errors.Is(projectedErr, ErrTeamExecutionIncomplete) {
			return result, projectedErr
		}
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

func (backend *AuthoritativeMissionExecutionBackend) CancelRecoveredOrActiveMissionExecution(
	ctx context.Context,
	input ParentMissionCancellation,
) (MissionExecutionResult, error) {
	if backend == nil || ctx == nil || input.MissionID == "" ||
		input.TeamInstanceID == "" || !validSHA256(input.ExpectedViewVersion) ||
		!validSHA256(input.ExecutionDigest) || input.LogicalNodeID == "" ||
		input.AttemptNumber <= 0 || input.ClaimGeneration <= 0 ||
		input.CorrelationID == "" {
		return MissionExecutionResult{}, ErrInvalidMissionExecution
	}
	return backend.ControlMission(ctx, MissionExecutionCommand{
		SchemaVersion: MissionExecutionSchemaVersion,
		Operation:     missionExecutionControl,
		MissionID:     input.MissionID, TeamInstanceID: input.TeamInstanceID,
		ExpectedViewVersion: input.ExpectedViewVersion,
		ControlAction:       "cancel", ExecutionDigest: input.ExecutionDigest,
		LogicalNodeID: input.LogicalNodeID, AttemptNumber: input.AttemptNumber,
		ClaimGeneration: input.ClaimGeneration, CorrelationID: input.CorrelationID,
	})
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
			return backend.resolveInterruptedStart(
				ctx, command, plan, flight, err,
			)
		}
		if projected, ok := backend.state.TeamExecution(command.TeamInstanceID); ok {
			result, projectedErr := backend.projectedResult(
				command, plan, flight.executionDigest, projected,
			)
			if !errors.Is(projectedErr, ErrTeamExecutionIncomplete) {
				return result, projectedErr
			}
		}
		select {
		case <-ctx.Done():
			return backend.resolveInterruptedStart(
				ctx, command, plan, flight, ctx.Err(),
			)
		case <-deadline.C:
			return backend.resolveInterruptedStart(
				ctx, command, plan, flight, ErrTeamExecutionIncomplete,
			)
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

func (backend *AuthoritativeMissionExecutionBackend) resolveInterruptedStart(
	ctx context.Context,
	command MissionExecutionCommand,
	plan teams.ExecutionPlan,
	flight *authoritativeMissionFlight,
	cause error,
) (MissionExecutionResult, error) {
	flight.cancel()
	<-flight.done
	refreshContext, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		backend.visibilityTimeout,
	)
	defer cancel()
	if err := backend.state.Refresh(refreshContext); err != nil {
		return MissionExecutionResult{}, errors.Join(cause, err)
	}
	if projected, ok := backend.state.TeamExecution(command.TeamInstanceID); ok {
		return backend.projectedResult(
			command,
			plan,
			flight.executionDigest,
			projected,
		)
	}
	backend.mu.Lock()
	if backend.flights[command.TeamInstanceID] == flight {
		delete(backend.flights, command.TeamInstanceID)
	}
	backend.mu.Unlock()
	return MissionExecutionResult{}, cause
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
	if projected.Status == "running" &&
		!backend.completeProjectedStartLineage(projected) {
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
		ExecutionDigest: executionDigest,
	}, nil
}

func (backend *AuthoritativeMissionExecutionBackend) completeProjectedStartLineage(
	projected projection.TeamExecution,
) bool {
	state, ok := backend.state.(missionExecutionStartLineageState)
	if !ok {
		return false
	}
	active := 0
	for _, node := range projected.Nodes {
		if node.CurrentAttempt <= 0 {
			continue
		}
		var current *projection.TeamExecutionAttempt
		for index := range node.Attempts {
			if node.Attempts[index].AttemptNumber == node.CurrentAttempt {
				current = &node.Attempts[index]
				break
			}
		}
		if current == nil {
			return false
		}
		if current.Status != "dispatched" {
			continue
		}
		if current.WorkItemID == "" || current.RunID == "" ||
			current.ClaimID == "" || current.ClaimGeneration <= 0 {
			return false
		}
		run, exists := state.Run(current.RunID)
		if !exists || run.ID != current.RunID ||
			run.WorkItemID != current.WorkItemID || run.Phase != "running" ||
			run.ClaimID != current.ClaimID ||
			run.ClaimGeneration != current.ClaimGeneration {
			return false
		}
		grant, exists := state.LatestAgentGrantForRun(current.RunID)
		if !exists || grant.WorkItemID != current.WorkItemID ||
			grant.RunID != current.RunID || grant.ClaimID != current.ClaimID ||
			grant.ClaimGeneration != current.ClaimGeneration ||
			grant.RuntimeInstanceID != current.RuntimeInstanceID ||
			grant.AgentInstanceID != current.AgentInstanceID ||
			!grant.RevokedAt.IsZero() {
			return false
		}
		active++
	}
	return active > 0
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
			command.ContextVersion == 0 && command.ConfirmedConstraints == nil &&
			command.AcceptedDecisions == nil &&
			command.PreflightDigest == "" &&
			validMissionExecutionControl(command)
	default:
		return false
	}
}

func validMissionExecutionPreStart(command MissionExecutionCommand) bool {
	workPackage, err := missionExecutionWorkPackage(command.WorkPackageID)
	return err == nil && command.WorkPackageDigest == workPackage.Digest() &&
		validMissionExecutionText(command.Objective, maxMissionExecutionTextBytes) &&
		validMissionContext(command)
}

func validMissionContext(command MissionExecutionCommand) bool {
	if command.ContextVersion == 0 {
		return command.ConfirmedConstraints == nil && command.AcceptedDecisions == nil
	}
	if command.ContextVersion != MissionContextVersion ||
		command.ConfirmedConstraints == nil || command.AcceptedDecisions == nil ||
		len(command.ConfirmedConstraints) > maxMissionExecutionListItems ||
		len(command.AcceptedDecisions) > maxMissionExecutionListItems {
		return false
	}
	seen := make(map[string]struct{}, len(command.ConfirmedConstraints)+len(command.AcceptedDecisions))
	for _, values := range [][]string{command.ConfirmedConstraints, command.AcceptedDecisions} {
		for _, value := range values {
			if strings.TrimSpace(value) != value ||
				!validMissionExecutionText(value, maxMissionExecutionTextBytes) {
				return false
			}
			if _, duplicate := seen[value]; duplicate {
				return false
			}
			seen[value] = struct{}{}
		}
	}
	return true
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
	claimGenerationValid := command.ClaimGeneration > 0
	if command.ClaimGeneration == 0 && command.AttemptNumber == 2 {
		switch command.ControlAction {
		case "approve", "reject", "not_now":
			claimGenerationValid = true
		}
	}
	return validSHA256(command.ExecutionDigest) &&
		validMissionExecutionText(command.LogicalNodeID, 128) &&
		command.AttemptNumber > 0 && command.AttemptNumber <= 16 &&
		claimGenerationValid
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
			(node.Role != "main" && node.Role != "subagent") ||
			node.MaxAttempts <= 0 || node.MaxAttempts > 16 ||
			!validMissionExecutionStrings(node.DependsOn) ||
			!validMissionExecutionRoute(
				node.HarnessAdapter,
				node.ProviderID,
				node.ProviderAccountID,
				node.ModelID,
				node.AuthMode,
				node.CredentialRevision,
				node.ReasoningEffort,
				node.TimeoutSeconds,
				node.BudgetCredits,
				node.Capabilities,
				node.Status,
				node.BlockReason,
			) ||
			!validMissionExecutionFallback(node) ||
			!validMissionExecutionFallbackApproval(node) {
			return false
		}
	}
	return true
}

func validMissionExecutionFallback(node MissionExecutionNodePreview) bool {
	if !node.FallbackConfigured {
		return node.FallbackHarnessAdapter == "" &&
			node.FallbackProviderID == "" &&
			node.FallbackProviderAccountID == "" &&
			node.FallbackModelID == "" &&
			node.FallbackAuthMode == "" &&
			node.FallbackCredentialRevision == 0 &&
			node.FallbackReasoningEffort == "" &&
			node.FallbackTimeoutSeconds == 0 &&
			node.FallbackBudgetCredits == nil &&
			len(node.FallbackCapabilities) == 0 &&
			node.FallbackStatus == "" && node.FallbackBlockReason == ""
	}
	return validMissionExecutionRoute(
		node.FallbackHarnessAdapter,
		node.FallbackProviderID,
		node.FallbackProviderAccountID,
		node.FallbackModelID,
		node.FallbackAuthMode,
		node.FallbackCredentialRevision,
		node.FallbackReasoningEffort,
		node.FallbackTimeoutSeconds,
		node.FallbackBudgetCredits,
		node.FallbackCapabilities,
		node.FallbackStatus,
		node.FallbackBlockReason,
	)
}

func validMissionExecutionRoute(
	harnessAdapter string,
	providerID string,
	providerAccountID string,
	modelID string,
	authMode string,
	credentialRevision int64,
	reasoningEffort string,
	timeoutSeconds int64,
	budgetCredits *int64,
	capabilities []string,
	status string,
	blockReason string,
) bool {
	if !validMissionExecutionIdentifier(harnessAdapter, 256) ||
		!credentials.ValidProviderIdentifier(providerID) ||
		!validMissionExecutionProfileText(modelID, 256) || timeoutSeconds <= 0 ||
		(budgetCredits != nil && *budgetCredits < 0) ||
		!validMissionExecutionCapabilities(capabilities) ||
		!validMissionExecutionRouteStatus(status, blockReason) {
		return false
	}
	switch loomruntime.AuthMode(authMode) {
	case loomruntime.AuthNative:
		if providerAccountID != "" || credentialRevision != 0 {
			return false
		}
	case loomruntime.AuthBrokered, loomruntime.AuthProviderEphemeral:
		if !credentials.ValidProviderAccountIdentifier(providerID, providerAccountID) ||
			credentialRevision <= 0 {
			return false
		}
	default:
		return false
	}
	if reasoningEffort == "" {
		return true
	}
	if len(reasoningEffort) > 32 ||
		!containsMissionExecutionCapability(
			capabilities, loomruntime.CapabilityReasoningEffort,
		) {
		return false
	}
	for _, character := range reasoningEffort {
		if character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' ||
			character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validMissionExecutionCapabilities(values []string) bool {
	if values == nil || len(values) > maxMissionExecutionListItems ||
		!sort.StringsAreSorted(values) {
		return false
	}
	for index, value := range values {
		if !validMissionExecutionIdentifier(value, 256) ||
			(index > 0 && value == values[index-1]) {
			return false
		}
	}
	return true
}

func validMissionExecutionIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == ':' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}

func validMissionExecutionProfileText(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e ||
			strings.ContainsRune("\\\"'`", character) {
			return false
		}
	}
	return true
}

func containsMissionExecutionCapability(values []string, target string) bool {
	index := sort.SearchStrings(values, target)
	return index < len(values) && values[index] == target
}

func validMissionExecutionRouteStatus(status, blockReason string) bool {
	switch status {
	case "ready":
		return blockReason == ""
	case "blocked":
		return validMissionExecutionText(blockReason, 512)
	default:
		return false
	}
}

func validMissionExecutionFallbackApproval(
	node MissionExecutionNodePreview,
) bool {
	return node.FallbackApprovalAvailable ==
		(node.FallbackApprovalVersion > 0) &&
		(!node.FallbackApprovalAvailable || node.FallbackConfigured) &&
		(!node.FallbackConfigured || node.FallbackApprovalRequired) &&
		(node.FallbackConfigured ||
			(!node.FallbackApprovalRequired &&
				node.FallbackApprovalVersion == 0))
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
		if node.BudgetCredits != nil {
			budget := *node.BudgetCredits
			cloned.Nodes[index].BudgetCredits = &budget
		}
		if node.FallbackBudgetCredits != nil {
			budget := *node.FallbackBudgetCredits
			cloned.Nodes[index].FallbackBudgetCredits = &budget
		}
		cloned.Nodes[index].DependsOn = cloneMissionExecutionStrings(node.DependsOn)
		if node.Capabilities != nil {
			cloned.Nodes[index].Capabilities = cloneMissionExecutionStrings(
				node.Capabilities,
			)
		}
		if node.FallbackCapabilities != nil {
			cloned.Nodes[index].FallbackCapabilities = cloneMissionExecutionStrings(
				node.FallbackCapabilities,
			)
		}
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
