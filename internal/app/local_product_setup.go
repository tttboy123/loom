package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"
	"loom-pi-rebuild/internal/work"
)

const localProductSetupSchemaVersion = 1

var (
	ErrInvalidLocalProductSetup              = errors.New("invalid local product setup")
	ErrBuilderNotFound                       = errors.New("builder session not found")
	ErrBuilderConflict                       = errors.New("builder session conflict")
	ErrBuilderIncompatible                   = errors.New("builder configuration incompatible")
	ErrRemoteToolEnrollmentSelectionRejected = errors.New("remote tool enrollment selection rejected")
	ErrBuilderConfirmationRequired           = errors.New("builder confirmation required")
	ErrCredentialSetupUnavailable            = errors.New("credential setup unavailable")
	ErrNativeAuthConnectUnavailable          = errors.New("native auth connection unavailable")
	ErrNativeAuthConnectBusy                 = errors.New("native auth connection busy")
)

type BuilderSource string

const (
	BuilderSourceBlank     BuilderSource = "blank"
	BuilderSourceSavedTeam BuilderSource = "saved_team"
	BuilderSourceTemplate  BuilderSource = "template"
)

type SetupIdentitySource interface {
	NextSetupID(string) (string, error)
}

type NativeAuthObservation struct {
	Status   string `json:"status"`
	AuthMode string `json:"auth_mode"`
	Reason   string `json:"reason"`
}

type NativeAuthObserver interface {
	ObserveNativeAuth(context.Context) (NativeAuthObservation, error)
}

type NativeAuthConnector interface {
	StartNativeAuth(context.Context) error
	Close() error
}

type CredentialStatusSource interface {
	CredentialStatus(
		context.Context,
		string,
	) (credentials.MetadataResult, error)
}

type CredentialAccountStatusSource interface {
	CredentialAccountStatus(
		context.Context,
		string,
		string,
	) (credentials.MetadataResult, error)
}

type CredentialVaultStatusSource interface {
	CredentialVaultStatus(context.Context) (CredentialVaultStatus, error)
}

type CredentialMutator interface {
	Configure(
		context.Context,
		credentials.CredentialCommand,
	) (credentials.MetadataResult, error)
	Verify(
		context.Context,
		credentials.CredentialCommand,
	) (credentials.MetadataResult, error)
	Replace(
		context.Context,
		credentials.CredentialCommand,
	) (credentials.MetadataResult, error)
	Revoke(
		context.Context,
		credentials.CredentialCommand,
	) (credentials.MetadataResult, error)
}

type SetupCatalogSource interface {
	CatalogForView(
		context.Context,
		projection.GlobalReadView,
	) (LocalProductSetupCatalog, error)
}

type SetupSkillRevision struct {
	ID                 string   `json:"id"`
	Revision           int      `json:"revision"`
	Digest             string   `json:"digest"`
	SourceScope        string   `json:"source_scope"`
	Risk               string   `json:"risk"`
	CompatibleRuntimes []string `json:"compatible_runtimes"`
}

type SetupResourcePointer struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type SetupRoleOption struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	AgentDefinitionID string   `json:"agent_definition_id"`
	RuntimeProfileID  string   `json:"runtime_profile_id"`
	RuntimeInstanceID string   `json:"runtime_instance_id"`
	SkillRevisionIDs  []string `json:"skill_revision_ids"`
	PermissionIDs     []string `json:"permission_ids"`
	ResourceIDs       []string `json:"resource_ids"`
	Responsibility    string   `json:"responsibility"`
}

type SetupTeamTemplate struct {
	ID                   string   `json:"id"`
	Version              int      `json:"version"`
	Digest               string   `json:"digest"`
	Name                 string   `json:"name"`
	Purpose              string   `json:"purpose"`
	MainRoleID           string   `json:"main_role_id"`
	SubagentRoleIDs      []string `json:"subagent_role_ids"`
	RequestedConcurrency int      `json:"requested_concurrency"`
	MaximumBudgetCredits int      `json:"maximum_budget_credits"`
}

type LocalProductSetupCatalog struct {
	CatalogDigest      string
	AgentDefinitions   []agents.AgentDefinition
	RuntimeProfiles    []loomruntime.RuntimeProfile
	RuntimeDiscovery   loomruntime.RuntimeDiscoverySnapshot
	SkillRevisions     []SetupSkillRevision
	Permissions        []string
	Resources          []SetupResourcePointer
	RoleOptions        []SetupRoleOption
	Templates          []SetupTeamTemplate
	BudgetCeiling      int
	ConcurrencyCeiling int
}

type LocalProductSetupConfig struct {
	Journal    *journal.Store
	Projection interface {
		Rebuild(context.Context) error
		GlobalReadView() projection.GlobalReadView
	}
	Writer                       *state.LocalProductSetupWriter
	Catalog                      LocalProductSetupCatalog
	CatalogSource                SetupCatalogSource
	Identity                     SetupIdentitySource
	Now                          func() time.Time
	NativeAuth                   NativeAuthObserver
	NativeAuthConnector          NativeAuthConnector
	Credentials                  CredentialStatusSource
	CredentialMutator            CredentialMutator
	CredentialVault              CredentialVaultStatusSource
	ProviderAccountPolicies      ProviderAccountPolicyAuthority
	ProviderModelRateCards       ProviderModelRateCardAuthority
	RemoteToolBackendEnrollments RemoteToolBackendEnrollmentAuthority
}

type ProviderSetupStatus struct {
	ProviderID          string `json:"provider_id"`
	AuthMode            string `json:"auth_mode"`
	CredentialReference string `json:"credential_reference"`
	Revision            int64  `json:"revision"`
	Status              string `json:"status"`
	Reason              string `json:"reason"`
}

type ProviderDirectoryEntry struct {
	ProviderID             string `json:"provider_id"`
	DisplayName            string `json:"display_name"`
	Category               string `json:"category"`
	Protocol               string `json:"protocol"`
	AuthMode               string `json:"auth_mode"`
	ConnectionKind         string `json:"connection_kind"`
	CredentialReference    string `json:"credential_reference"`
	Revision               int64  `json:"revision"`
	Status                 string `json:"status"`
	Reason                 string `json:"reason"`
	SupportsModelDiscovery bool   `json:"supports_model_discovery"`
}

type ProviderAccountDirectoryEntry struct {
	ProviderID                 string                                `json:"provider_id"`
	ProviderAccountID          string                                `json:"provider_account_id"`
	AuthMode                   string                                `json:"auth_mode"`
	CredentialReference        string                                `json:"credential_reference"`
	Revision                   int64                                 `json:"revision"`
	Status                     string                                `json:"status"`
	Reason                     string                                `json:"reason"`
	PolicyAvailable            bool                                  `json:"policy_available"`
	PolicyVersion              int                                   `json:"policy_version"`
	PolicyRevision             int64                                 `json:"policy_revision"`
	PolicyDigest               string                                `json:"policy_digest"`
	MaximumConcurrentAttempts  int                                   `json:"maximum_concurrent_attempts"`
	DispatchWindowSeconds      int64                                 `json:"dispatch_window_seconds"`
	MaximumDispatchStarts      int                                   `json:"maximum_dispatch_starts"`
	MaximumAssignedBudgetUnits int64                                 `json:"maximum_assigned_budget_units"`
	TrustDomain                string                                `json:"trust_domain"`
	RetentionMode              string                                `json:"retention_mode"`
	DataRegion                 string                                `json:"data_region"`
	RateCards                  []ProviderModelRateCardDirectoryEntry `json:"rate_cards"`
	RemoteToolBackends         []RemoteToolBackendDirectoryEntry     `json:"remote_tool_backends"`
}

type RemoteToolBackendDirectoryEntry struct {
	EnrollmentVersion             int      `json:"enrollment_version"`
	EnrollmentID                  string   `json:"enrollment_id"`
	BackendKind                   string   `json:"backend_kind"`
	AdapterID                     string   `json:"adapter_id"`
	ProviderAccountPolicyVersion  int      `json:"provider_account_policy_version"`
	ProviderAccountPolicyRevision int64    `json:"provider_account_policy_revision"`
	ProviderAccountPolicyDigest   string   `json:"provider_account_policy_digest"`
	PolicyCurrent                 bool     `json:"policy_current"`
	EndpointFingerprint           string   `json:"endpoint_fingerprint"`
	MCPServerID                   string   `json:"mcp_server_id"`
	AllowedTools                  []string `json:"allowed_tools"`
	Revision                      int64    `json:"revision"`
	Status                        string   `json:"status"`
	MaximumConcurrentCalls        int      `json:"maximum_concurrent_calls"`
	MaximumCallsPerAttempt        int      `json:"maximum_calls_per_attempt"`
	TimeoutSeconds                int64    `json:"timeout_seconds"`
	MaximumResultBytes            int      `json:"maximum_result_bytes"`
	MaximumBudgetUnits            int64    `json:"maximum_budget_units"`
	ConfiguredAt                  string   `json:"configured_at"`
	EnrollmentDigest              string   `json:"enrollment_digest"`
}

type ProviderModelRateCardDirectoryEntry struct {
	ModelID                        string `json:"model_id"`
	Revision                       int64  `json:"revision"`
	RateCardDigest                 string `json:"rate_card_digest"`
	Currency                       string `json:"currency"`
	InputTokenBasis                string `json:"input_token_basis"`
	InputMicrounitsPerMillion      int64  `json:"input_microunits_per_million"`
	OutputMicrounitsPerMillion     int64  `json:"output_microunits_per_million"`
	CacheReadMicrounitsPerMillion  int64  `json:"cache_read_microunits_per_million"`
	CacheWriteMicrounitsPerMillion int64  `json:"cache_write_microunits_per_million"`
	RoundingMode                   string `json:"rounding_mode"`
	ConfiguredAt                   string `json:"configured_at"`
}

type CredentialVaultStatus struct {
	SchemaVersion             int    `json:"schema_version"`
	Status                    string `json:"status"`
	StorageMode               string `json:"storage_mode"`
	MigrationRequiredAccounts int    `json:"migration_required_accounts"`
	RecoveryRequiredAccounts  int    `json:"recovery_required_accounts"`
}

type ConversationProviderProfile struct {
	ProfileID          string `json:"profile_id"`
	HarnessAdapter     string `json:"harness_adapter"`
	ProviderID         string `json:"provider_id"`
	ProviderAccountID  string `json:"provider_account_id"`
	DisplayName        string `json:"display_name"`
	Protocol           string `json:"protocol"`
	ModelID            string `json:"model_id"`
	AuthMode           string `json:"auth_mode"`
	CredentialRevision int64  `json:"credential_revision"`
	PolicyVersion      int    `json:"policy_version"`
	PolicyRevision     int64  `json:"policy_revision"`
	PolicyDigest       string `json:"policy_digest"`
	TrustDomain        string `json:"trust_domain"`
	RetentionMode      string `json:"retention_mode"`
	DataRegion         string `json:"data_region"`
}

type SetupRuntimePreview struct {
	RuntimeInstanceID    string   `json:"runtime_instance_id"`
	DisplayName          string   `json:"display_name"`
	AdapterType          string   `json:"adapter_type"`
	ExecutableVersion    string   `json:"executable_version"`
	Status               string   `json:"status"`
	Capacity             int      `json:"capacity"`
	ModelID              string   `json:"model_id,omitempty"`
	ModelIDs             []string `json:"model_ids"`
	ObservedCapabilities []string `json:"observed_capabilities"`
	SourceProbeID        string   `json:"source_probe_id"`
}

type SetupSavedTeamPreview struct {
	ID               string `json:"id"`
	Version          int    `json:"version"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	DefinitionDigest string `json:"definition_digest"`
	StreamHead       int64  `json:"stream_head"`
}

type SetupTeamTemplatePreview = SetupTeamTemplate

type SetupRoleOptionPreview struct {
	ID                         string   `json:"id"`
	Kind                       string   `json:"kind"`
	AgentDefinitionID          string   `json:"agent_definition_id"`
	RuntimeProfileID           string   `json:"runtime_profile_id"`
	RuntimeInstanceID          string   `json:"runtime_instance_id"`
	HarnessAdapter             string   `json:"harness_adapter"`
	ProviderID                 string   `json:"provider_id"`
	ProviderAccountID          string   `json:"provider_account_id"`
	ModelID                    string   `json:"model_id"`
	AuthMode                   string   `json:"auth_mode"`
	CredentialRevision         int64    `json:"credential_revision"`
	ReasoningEffort            string   `json:"reasoning_effort"`
	TimeoutMilliseconds        int64    `json:"timeout_milliseconds"`
	BudgetAvailable            bool     `json:"budget_available"`
	BudgetUnits                int64    `json:"budget_units"`
	RequiredCapabilities       []string `json:"required_capabilities"`
	RemoteToolEnrollmentID     string   `json:"remote_tool_enrollment_id,omitempty"`
	RemoteToolEnrollmentDigest string   `json:"remote_tool_enrollment_digest,omitempty"`
	SkillRevisionIDs           []string `json:"skill_revision_ids"`
	PermissionIDs              []string `json:"permission_ids"`
	ResourceIDs                []string `json:"resource_ids"`
	Responsibility             string   `json:"responsibility"`
}

type SetupSnapshot struct {
	SchemaVersion        int                             `json:"schema_version"`
	ViewVersion          string                          `json:"view_version"`
	Codex                ProviderSetupStatus             `json:"codex"`
	MiniMax              ProviderSetupStatus             `json:"minimax"`
	Providers            []ProviderDirectoryEntry        `json:"providers"`
	ProviderAccounts     []ProviderAccountDirectoryEntry `json:"provider_accounts"`
	CredentialVault      *CredentialVaultStatus          `json:"credential_vault,omitempty"`
	ConversationProfiles []ConversationProviderProfile   `json:"conversation_profiles"`
	Runtimes             []SetupRuntimePreview           `json:"runtimes"`
	SavedTeams           []SetupSavedTeamPreview         `json:"saved_teams"`
	Templates            []SetupTeamTemplatePreview      `json:"templates"`
	RoleOptions          []SetupRoleOptionPreview        `json:"role_options"`
	Skills               []SetupSkillRevision            `json:"skills"`
	Permissions          []string                        `json:"permissions"`
	Resources            []SetupResourcePointer          `json:"resources"`
}

type BuilderQuestionOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type BuilderQuestion struct {
	ID      string                  `json:"id"`
	Prompt  string                  `json:"prompt"`
	Options []BuilderQuestionOption `json:"options"`
}

type BuilderExecutionRoutePreview struct {
	RuntimeProfileID           string   `json:"runtime_profile_id"`
	HarnessAdapter             string   `json:"harness_adapter"`
	ProviderID                 string   `json:"provider_id"`
	ProviderAccountID          string   `json:"provider_account_id"`
	ModelID                    string   `json:"model_id"`
	AuthMode                   string   `json:"auth_mode"`
	CredentialRevision         int64    `json:"credential_revision"`
	ReasoningEffort            string   `json:"reasoning_effort"`
	TimeoutMilliseconds        int64    `json:"timeout_milliseconds"`
	BudgetAvailable            bool     `json:"budget_available"`
	BudgetUnits                int64    `json:"budget_units"`
	RequiredCapabilities       []string `json:"required_capabilities"`
	RemoteToolEnrollmentID     string   `json:"remote_tool_enrollment_id,omitempty"`
	RemoteToolEnrollmentDigest string   `json:"remote_tool_enrollment_digest,omitempty"`
}

type BuilderRolePreview struct {
	Kind                         string                         `json:"kind"`
	AgentDefinitionID            string                         `json:"agent_definition_id"`
	AgentVersion                 int                            `json:"agent_version"`
	AgentScope                   string                         `json:"agent_scope"`
	DisplayName                  string                         `json:"display_name"`
	Responsibility               string                         `json:"responsibility"`
	Runtime                      SetupRuntimePreview            `json:"runtime"`
	RuntimeProfileID             string                         `json:"runtime_profile_id"`
	HarnessAdapter               string                         `json:"harness_adapter"`
	ProviderID                   string                         `json:"provider_id"`
	ProviderAccountID            string                         `json:"provider_account_id"`
	ModelID                      string                         `json:"model_id"`
	AuthMode                     string                         `json:"auth_mode"`
	CredentialRevision           int64                          `json:"credential_revision"`
	ReasoningEffort              string                         `json:"reasoning_effort"`
	TimeoutMilliseconds          int64                          `json:"timeout_milliseconds"`
	BudgetAvailable              bool                           `json:"budget_available"`
	BudgetUnits                  int64                          `json:"budget_units"`
	RequiredCapabilities         []string                       `json:"required_capabilities"`
	RemoteToolEnrollmentID       string                         `json:"remote_tool_enrollment_id,omitempty"`
	RemoteToolEnrollmentDigest   string                         `json:"remote_tool_enrollment_digest,omitempty"`
	FallbackConfigured           bool                           `json:"fallback_configured"`
	FallbackRuntimeProfileID     string                         `json:"fallback_runtime_profile_id"`
	FallbackHarnessAdapter       string                         `json:"fallback_harness_adapter"`
	FallbackProviderID           string                         `json:"fallback_provider_id"`
	FallbackProviderAccountID    string                         `json:"fallback_provider_account_id"`
	FallbackModelID              string                         `json:"fallback_model_id"`
	FallbackAuthMode             string                         `json:"fallback_auth_mode"`
	FallbackCredentialRevision   int64                          `json:"fallback_credential_revision"`
	FallbackReasoningEffort      string                         `json:"fallback_reasoning_effort"`
	FallbackTimeoutMilliseconds  int64                          `json:"fallback_timeout_milliseconds"`
	FallbackBudgetAvailable      bool                           `json:"fallback_budget_available"`
	FallbackBudgetUnits          int64                          `json:"fallback_budget_units"`
	FallbackRequiredCapabilities []string                       `json:"fallback_required_capabilities"`
	FallbackApprovalRequired     bool                           `json:"fallback_approval_required"`
	ParallelRouteSetVersion      int                            `json:"parallel_route_set_version"`
	ParallelRoutes               []BuilderExecutionRoutePreview `json:"parallel_routes"`
	SynthesisRoute               BuilderExecutionRoutePreview   `json:"synthesis_route"`
	Skills                       []SetupSkillRevision           `json:"skills"`
	PermissionIDs                []string                       `json:"permission_ids"`
	ResourceIDs                  []string                       `json:"resource_ids"`
	Compatible                   bool                           `json:"compatible"`
	CompatibilityReason          string                         `json:"compatibility_reason"`
}

type BuilderPreview struct {
	Name                 string               `json:"name"`
	Purpose              string               `json:"purpose"`
	Roles                []BuilderRolePreview `json:"roles"`
	Permissions          []string             `json:"permissions"`
	Resources            []string             `json:"resources"`
	CompatibilityGaps    []string             `json:"compatibility_gaps"`
	RequestedConcurrency int                  `json:"requested_concurrency"`
	MaximumBudgetCredits int                  `json:"maximum_budget_credits"`
	EstimatedMaximumCost string               `json:"estimated_maximum_cost"`
}

type BuilderSessionView struct {
	SchemaVersion int             `json:"schema_version"`
	DraftID       string          `json:"draft_id"`
	Revision      int             `json:"revision"`
	Source        BuilderSource   `json:"source"`
	CatalogDigest string          `json:"catalog_digest"`
	ViewVersion   string          `json:"view_version"`
	ContentDigest string          `json:"content_digest"`
	BindingDigest string          `json:"binding_digest"`
	Question      BuilderQuestion `json:"question"`
	Preview       BuilderPreview  `json:"preview"`
	CanConfirm    bool            `json:"can_confirm"`
}

type BuilderStartCommand struct {
	Source        BuilderSource `json:"source"`
	SourceID      string        `json:"source_id"`
	SourceVersion int           `json:"source_version"`
	SourceDigest  string        `json:"source_digest"`
}

type BuilderAnswerCommand struct {
	DraftID          string `json:"draft_id"`
	ExpectedRevision int    `json:"expected_revision"`
	CatalogDigest    string `json:"catalog_digest"`
	ViewVersion      string `json:"view_version"`
	QuestionID       string `json:"question_id"`
	Answer           string `json:"answer"`
}

type BuilderEditCommand struct {
	DraftID               string `json:"draft_id"`
	ExpectedRevision      int    `json:"expected_revision"`
	CatalogDigest         string `json:"catalog_digest"`
	ViewVersion           string `json:"view_version"`
	Field                 string `json:"field"`
	Value                 string `json:"value"`
	RoleAgentDefinitionID string `json:"role_agent_definition_id,omitempty"`
}

type BuilderValidateCommand struct {
	DraftID          string `json:"draft_id"`
	ExpectedRevision int    `json:"expected_revision"`
	CatalogDigest    string `json:"catalog_digest"`
	ViewVersion      string `json:"view_version"`
}

type BuilderConfirmCommand struct {
	DraftID          string `json:"draft_id"`
	ExpectedRevision int    `json:"expected_revision"`
	CatalogDigest    string `json:"catalog_digest"`
	ViewVersion      string `json:"view_version"`
	BindingDigest    string `json:"binding_digest"`
	DefinitionID     string `json:"definition_id"`
	Scope            string `json:"scope"`
	ProjectID        string `json:"project_id"`
	Confirm          bool   `json:"confirm"`
}

type BuilderConfirmation struct {
	TeamDefinitionID      string `json:"team_definition_id"`
	TeamDefinitionVersion int    `json:"team_definition_version"`
	TeamDefinitionDigest  string `json:"team_definition_digest"`
	Status                string `json:"status"`
	TeamInstanceCreated   bool   `json:"team_instance_created"`
	RunCreated            bool   `json:"run_created"`
}

type TeamStatusCommand struct {
	DefinitionID string `json:"definition_id"`
	ExpectedHead int64  `json:"expected_head"`
}

type CredentialSetupCommand struct {
	ProviderID          string `json:"provider_id"`
	ProviderAccountID   string `json:"provider_account_id,omitempty"`
	CredentialReference string `json:"credential_reference"`
	ExpectedRevision    int64  `json:"expected_revision"`
	OperationID         string `json:"operation_id,omitempty"`
	Secret              []byte `json:"secret"`
}

type CredentialSetupResult struct {
	ProviderID string `json:"provider_id"`
	Revision   int64  `json:"revision"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

type ProviderConnectResult struct {
	ProviderID string `json:"provider_id"`
	AuthMode   string `json:"auth_mode"`
	Status     string `json:"status"`
}

type localProductBuilderSession struct {
	view                     BuilderSessionView
	catalog                  LocalProductSetupCatalog
	domainCatalog            teams.TeamDraftCatalogSnapshot
	name                     string
	purpose                  string
	mainRoleID               string
	subRoleIDs               []string
	mainFallbackRoleID       string
	subagentFallbackRoleIDs  map[string]string
	mainParallelRoleIDs      []string
	subagentParallelRoleIDs  map[string][]string
	mainSynthesisRoleID      string
	subagentSynthesisRoleIDs map[string]string
	structured               teams.StructuredTeamDraft
	hasStructured            bool
	sourceID                 string
	sourceVersion            int
	sourceDigest             string
}

type LocalProductSetupService struct {
	journal    *journal.Store
	projection interface {
		Rebuild(context.Context) error
		GlobalReadView() projection.GlobalReadView
	}
	writer                       *state.LocalProductSetupWriter
	catalog                      LocalProductSetupCatalog
	domainCatalog                teams.TeamDraftCatalogSnapshot
	catalogSource                SetupCatalogSource
	identity                     SetupIdentitySource
	now                          func() time.Time
	nativeAuth                   NativeAuthObserver
	nativeAuthConnector          NativeAuthConnector
	credentials                  CredentialStatusSource
	credentialMutator            CredentialMutator
	credentialVault              CredentialVaultStatusSource
	providerAccountPolicies      ProviderAccountPolicyAuthority
	providerModelRateCards       ProviderModelRateCardAuthority
	remoteToolBackendEnrollments RemoteToolBackendEnrollmentAuthority

	mu       sync.Mutex
	sessions map[string]*localProductBuilderSession
}

func NewLocalProductSetupService(
	config LocalProductSetupConfig,
) (*LocalProductSetupService, error) {
	if config.Journal == nil ||
		config.Projection == nil ||
		config.Writer == nil ||
		config.Identity == nil ||
		config.Now == nil ||
		config.NativeAuth == nil ||
		config.Credentials == nil ||
		!validSetupDigest(config.Catalog.CatalogDigest) {
		return nil, ErrInvalidLocalProductSetup
	}
	domainCatalog, err := buildSetupDomainCatalog(config.Catalog)
	if err != nil {
		return nil, ErrInvalidLocalProductSetup
	}
	if err := validateSetupCatalog(config.Catalog); err != nil {
		return nil, err
	}
	return &LocalProductSetupService{
		journal:                      config.Journal,
		projection:                   config.Projection,
		writer:                       config.Writer,
		catalog:                      cloneSetupCatalog(config.Catalog),
		domainCatalog:                domainCatalog,
		catalogSource:                config.CatalogSource,
		identity:                     config.Identity,
		now:                          config.Now,
		nativeAuth:                   config.NativeAuth,
		nativeAuthConnector:          config.NativeAuthConnector,
		credentials:                  config.Credentials,
		credentialMutator:            config.CredentialMutator,
		credentialVault:              config.CredentialVault,
		providerAccountPolicies:      config.ProviderAccountPolicies,
		providerModelRateCards:       config.ProviderModelRateCards,
		remoteToolBackendEnrollments: config.RemoteToolBackendEnrollments,
		sessions:                     make(map[string]*localProductBuilderSession),
	}, nil
}

func (service *LocalProductSetupService) ConnectCodex(
	ctx context.Context,
) (ProviderConnectResult, error) {
	if service == nil || ctx == nil {
		return ProviderConnectResult{}, ErrInvalidLocalProductSetup
	}
	observation, err := service.nativeAuth.ObserveNativeAuth(ctx)
	if err != nil || observation.AuthMode != "native_auth" {
		return ProviderConnectResult{}, ErrNativeAuthConnectUnavailable
	}
	if observation.Status == "available" {
		return ProviderConnectResult{
			ProviderID: "codex",
			AuthMode:   "native_auth",
			Status:     "already_connected",
		}, nil
	}
	if observation.Status != "not_logged_in" {
		return ProviderConnectResult{}, ErrNativeAuthConnectUnavailable
	}
	if service.nativeAuthConnector == nil {
		return ProviderConnectResult{}, ErrNativeAuthConnectUnavailable
	}
	if err := service.nativeAuthConnector.StartNativeAuth(ctx); err != nil {
		return ProviderConnectResult{}, err
	}
	return ProviderConnectResult{
		ProviderID: "codex",
		AuthMode:   "native_auth",
		Status:     "started",
	}, nil
}

func (service *LocalProductSetupService) Close() error {
	if service == nil {
		return ErrInvalidLocalProductSetup
	}
	if service.nativeAuthConnector != nil {
		return service.nativeAuthConnector.Close()
	}
	return nil
}

func (service *LocalProductSetupService) currentCatalog(
	ctx context.Context,
) (
	LocalProductSetupCatalog,
	teams.TeamDraftCatalogSnapshot,
	projection.GlobalReadView,
	error,
) {
	if err := service.projection.Rebuild(ctx); err != nil {
		return LocalProductSetupCatalog{},
			teams.TeamDraftCatalogSnapshot{},
			projection.GlobalReadView{},
			ErrInvalidLocalProductSetup
	}
	view := service.projection.GlobalReadView()
	catalog := cloneSetupCatalog(service.catalog)
	if service.catalogSource != nil {
		refreshed, err := service.catalogSource.CatalogForView(ctx, view)
		if err != nil {
			return LocalProductSetupCatalog{},
				teams.TeamDraftCatalogSnapshot{},
				projection.GlobalReadView{},
				ErrInvalidLocalProductSetup
		}
		catalog = cloneSetupCatalog(refreshed)
	}
	if err := validateSetupCatalog(catalog); err != nil {
		return LocalProductSetupCatalog{},
			teams.TeamDraftCatalogSnapshot{},
			projection.GlobalReadView{},
			err
	}
	domainCatalog, err := buildSetupDomainCatalog(catalog)
	if err != nil {
		return LocalProductSetupCatalog{},
			teams.TeamDraftCatalogSnapshot{},
			projection.GlobalReadView{},
			ErrInvalidLocalProductSetup
	}
	return catalog, domainCatalog, view, nil
}

func (service *LocalProductSetupService) SetupSnapshot(
	ctx context.Context,
) (SetupSnapshot, error) {
	if service == nil || ctx == nil {
		return SetupSnapshot{}, ErrInvalidLocalProductSetup
	}
	catalog, _, view, err := service.currentCatalog(ctx)
	if err != nil {
		return SetupSnapshot{}, fmt.Errorf(
			"%w: projection unavailable",
			ErrInvalidLocalProductSetup,
		)
	}
	auth, err := service.nativeAuth.ObserveNativeAuth(ctx)
	if err != nil {
		auth = NativeAuthObservation{
			Status:   "unavailable",
			AuthMode: "native_auth",
			Reason:   "unavailable",
		}
	}
	if auth.AuthMode != "native_auth" ||
		!closedNativeAuthStatus(auth.Status, auth.Reason) {
		auth = NativeAuthObservation{
			Status:   "unsupported",
			AuthMode: "native_auth",
			Reason:   "unknown_output",
		}
	}
	miniMax := ProviderSetupStatus{
		ProviderID: "minimax",
		AuthMode:   "brokered",
		Status:     "unconfigured",
	}
	if status, statusErr := service.credentials.CredentialStatus(
		ctx,
		"minimax",
	); statusErr == nil &&
		status.ProviderID == "minimax" &&
		closedCredentialStatus(string(status.Status), string(status.Reason)) {
		miniMax.Status = string(status.Status)
		miniMax.Reason = string(status.Reason)
		miniMax.CredentialReference = status.CredentialReference
		miniMax.Revision = status.Revision
	}
	savedRecords, _ := view.TeamDefinitions("", 64)
	saved := make([]SetupSavedTeamPreview, 0, len(savedRecords))
	for _, record := range savedRecords {
		streamHead, ok := view.Head("team-definition/" + record.ID)
		if !ok || streamHead.Sequence <= 0 {
			return SetupSnapshot{}, ErrInvalidLocalProductSetup
		}
		saved = append(saved, SetupSavedTeamPreview{
			ID:               record.ID,
			Version:          record.Version,
			Name:             record.Name,
			Status:           record.Status,
			DefinitionDigest: record.DefinitionDigest,
			StreamHead:       streamHead.Sequence,
		})
	}
	providers := service.providerDirectory(ctx, auth)
	providerAccounts := setupProviderAccountDirectory(
		ctx, view, service.credentials,
	)
	var credentialVault *CredentialVaultStatus
	if service.credentialVault != nil {
		vaultStatus, statusErr := service.credentialVault.CredentialVaultStatus(ctx)
		if statusErr != nil || !closedCredentialVaultStatus(vaultStatus) {
			vaultStatus = CredentialVaultStatus{
				SchemaVersion: 1,
				Status:        "recovery_required",
				StorageMode:   "local_key_file",
			}
		}
		vaultStatus = setupCredentialVaultStatus(vaultStatus, providerAccounts)
		credentialVault = &vaultStatus
	}
	return cloneSetupSnapshot(SetupSnapshot{
		SchemaVersion: localProductSetupSchemaVersion,
		ViewVersion:   view.Version(),
		Codex: ProviderSetupStatus{
			ProviderID: "codex",
			AuthMode:   "native_auth",
			Status:     auth.Status,
			Reason:     auth.Reason,
		},
		MiniMax:              miniMax,
		Providers:            providers,
		ProviderAccounts:     providerAccounts,
		CredentialVault:      credentialVault,
		ConversationProfiles: setupConversationProfiles(auth, providers, providerAccounts),
		Runtimes:             setupRuntimePreviews(catalog),
		SavedTeams:           saved,
		Templates:            append([]SetupTeamTemplate{}, catalog.Templates...),
		RoleOptions:          setupRoleOptionPreviews(catalog),
		Skills:               append([]SetupSkillRevision{}, catalog.SkillRevisions...),
		Permissions:          append([]string{}, catalog.Permissions...),
		Resources:            append([]SetupResourcePointer{}, catalog.Resources...),
	}), nil
}

func setupCredentialVaultStatus(
	base CredentialVaultStatus,
	accounts []ProviderAccountDirectoryEntry,
) CredentialVaultStatus {
	status := base
	status.MigrationRequiredAccounts = 0
	status.RecoveryRequiredAccounts = 0
	for _, account := range accounts {
		switch account.Status {
		case "migration_required":
			status.MigrationRequiredAccounts++
		case "recovery_required":
			status.RecoveryRequiredAccounts++
		}
	}
	if base.Status == "locked" {
		status.Status = "locked"
	} else if base.Status == "recovery_required" ||
		status.RecoveryRequiredAccounts > 0 {
		status.Status = "recovery_required"
	} else if base.Status == "migration_required" ||
		status.MigrationRequiredAccounts > 0 {
		status.Status = "migration_required"
	} else {
		status.Status = "unlocked"
	}
	return status
}

func closedCredentialVaultStatus(status CredentialVaultStatus) bool {
	if status.SchemaVersion != 1 ||
		status.MigrationRequiredAccounts < 0 ||
		status.RecoveryRequiredAccounts < 0 {
		return false
	}
	switch status.StorageMode {
	case "local_key_file", "passphrase", "external":
	default:
		return false
	}
	switch status.Status {
	case "unlocked", "locked", "migration_required", "recovery_required":
		return true
	default:
		return false
	}
}

func setupProviderAccountDirectory(
	ctx context.Context,
	view projection.GlobalReadView,
	statusSource CredentialStatusSource,
) []ProviderAccountDirectoryEntry {
	result := make([]ProviderAccountDirectoryEntry, 0)
	accountStatus, hasAccountStatus := statusSource.(CredentialAccountStatusSource)
	for _, descriptor := range provider.Catalog() {
		for _, record := range view.ProviderAccountCredentials(descriptor.ID) {
			if hasAccountStatus {
				status, err := accountStatus.CredentialAccountStatus(
					ctx, record.ProviderID, record.ProviderAccountID,
				)
				if err == nil &&
					status.CredentialReference == record.CredentialReference &&
					status.Revision == record.Revision &&
					closedCredentialStatus(
						string(status.Status), string(status.Reason),
					) {
					record.Status = string(status.Status)
					record.Reason = string(status.Reason)
				}
			}
			entry := ProviderAccountDirectoryEntry{
				ProviderID: record.ProviderID, ProviderAccountID: record.ProviderAccountID,
				AuthMode: record.AuthMode, CredentialReference: record.CredentialReference,
				Revision: record.Revision, Status: record.Status, Reason: record.Reason,
			}
			if policy, ok := view.ProviderAccountPolicy(
				record.ProviderID, record.ProviderAccountID,
			); ok {
				entry.PolicyAvailable = true
				entry.PolicyVersion = policy.Version()
				entry.PolicyRevision = policy.Revision()
				entry.PolicyDigest = policy.Digest()
				entry.MaximumConcurrentAttempts = policy.MaximumConcurrentAttempts()
				entry.DispatchWindowSeconds = int64(policy.DispatchWindow() / time.Second)
				entry.MaximumDispatchStarts = policy.MaximumDispatchStarts()
				entry.MaximumAssignedBudgetUnits = policy.MaximumAssignedBudgetUnits()
				entry.TrustDomain = policy.TrustDomain()
				entry.RetentionMode = policy.RetentionMode()
				entry.DataRegion = policy.DataRegion()
			}
			for _, rateCard := range view.ProviderModelRateCards(
				record.ProviderID, record.ProviderAccountID,
			) {
				entry.RateCards = append(entry.RateCards, providerModelRateCardDirectoryEntry(rateCard))
			}
			if entry.RateCards == nil {
				entry.RateCards = []ProviderModelRateCardDirectoryEntry{}
			}
			for _, enrollment := range view.RemoteToolBackendEnrollments(
				record.ProviderID, record.ProviderAccountID,
			) {
				policy, policyAvailable := view.ProviderAccountPolicy(
					record.ProviderID, record.ProviderAccountID,
				)
				entry.RemoteToolBackends = append(
					entry.RemoteToolBackends,
					remoteToolBackendDirectoryEntry(
						enrollment,
						policyAvailable && remoteToolBackendEnrollmentUsesPolicy(enrollment, policy),
					),
				)
			}
			if entry.RemoteToolBackends == nil {
				entry.RemoteToolBackends = []RemoteToolBackendDirectoryEntry{}
			}
			result = append(result, entry)
		}
	}
	return result
}

// openCodeConversationDefaultModel returns the OpenCode profile's default
// model: OpenCode's own hosted free-tier model. The OpenCode profile must not
// silently default to a DeepSeek/MiniMax model; those Providers have their own
// conversation profiles, and the client's Model layer gates cross-Provider
// OpenCode models by verified accounts so they are only used when the user
// explicitly selects them.
func openCodeConversationDefaultModel() string {
	return provider.OpenCodeConversationDefaultModel
}

func setupConversationProfiles(
	auth NativeAuthObservation,
	providers []ProviderDirectoryEntry,
	providerAccounts []ProviderAccountDirectoryEntry,
) []ConversationProviderProfile {
	accounts := make(map[string]ProviderAccountDirectoryEntry, len(providerAccounts))
	for _, entry := range providerAccounts {
		if entry.Status != "verified" || entry.Reason != "" || entry.Revision <= 0 ||
			entry.AuthMode != "brokered" ||
			!credentials.ValidProviderAccountIdentifier(
				entry.ProviderID, entry.ProviderAccountID,
			) {
			continue
		}
		accounts[entry.ProviderAccountID] = entry
	}
	for _, entry := range providers {
		if entry.Status != "verified" || entry.Reason != "" || entry.Revision <= 0 {
			continue
		}
		accountID := entry.ProviderID + ".primary"
		if _, exists := accounts[accountID]; !exists {
			accounts[accountID] = ProviderAccountDirectoryEntry{
				ProviderID: entry.ProviderID, ProviderAccountID: accountID,
				AuthMode: entry.AuthMode, Revision: entry.Revision,
				Status: entry.Status, Reason: entry.Reason,
			}
		}
	}
	profiles := make([]ConversationProviderProfile, 0, 1+len(providerAccounts))
	// User-configured verified brokered accounts come first: they carry an
	// explicit API key the user added and verified, so they are the most likely
	// to work out of the box (the default conversation profile is the first
	// entry). Native auth profiles go last, with OpenCode before Codex, because
	// the Codex official CLI account is frequently quota-limited while OpenCode
	// runs against the user's own configured providers.
	ordered := make([]ProviderAccountDirectoryEntry, 0, len(accounts))
	for _, entry := range accounts {
		ordered = append(ordered, entry)
	}
	providerOrder := make(map[string]int)
	for index, descriptor := range provider.Catalog() {
		providerOrder[descriptor.ID] = index
	}
	sort.Slice(ordered, func(left, right int) bool {
		leftOrder, leftKnown := providerOrder[ordered[left].ProviderID]
		rightOrder, rightKnown := providerOrder[ordered[right].ProviderID]
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return ordered[left].ProviderAccountID < ordered[right].ProviderAccountID
	})
	for _, entry := range ordered {
		profile := ConversationProviderProfile{
			HarnessAdapter: "loom-native", ProviderID: entry.ProviderID,
			ProviderAccountID: entry.ProviderAccountID,
			Protocol:          "openai_compatible",
			AuthMode:          "brokered", CredentialRevision: entry.Revision,
			PolicyVersion: entry.PolicyVersion, PolicyRevision: entry.PolicyRevision,
			PolicyDigest: entry.PolicyDigest, TrustDomain: entry.TrustDomain,
			RetentionMode: entry.RetentionMode, DataRegion: entry.DataRegion,
		}
		switch entry.ProviderID {
		case "anthropic":
			profile.ProfileID = provider.AnthropicConversationAccountProfileID(
				entry.ProviderAccountID, entry.Revision,
			)
			profile.DisplayName = "Anthropic"
			profile.Protocol = "anthropic_messages"
			profile.ModelID = provider.AnthropicConversationModelID
		case "deepseek":
			profile.ProfileID = provider.DeepSeekConversationAccountProfileID(
				entry.ProviderAccountID, entry.Revision,
			)
			profile.DisplayName = "DeepSeek"
			profile.ModelID = provider.DeepSeekConversationModelID
		case "kimi":
			profile.ProfileID = provider.KimiConversationAccountProfileID(
				entry.ProviderAccountID, entry.Revision,
			)
			profile.DisplayName = "Kimi"
			profile.ModelID = provider.KimiConversationModelID
		case "minimax":
			profile.ProfileID = provider.MiniMaxConversationAccountProfileID(
				entry.ProviderAccountID, entry.Revision,
			)
			profile.DisplayName = "MiniMax"
			profile.ModelID = provider.MiniMaxConversationModelID
		default:
			continue
		}
		if profile.ProfileID == "" {
			continue
		}
		profiles = append(profiles, ConversationProviderProfile{
			ProfileID: profile.ProfileID, HarnessAdapter: profile.HarnessAdapter,
			ProviderID: profile.ProviderID, ProviderAccountID: profile.ProviderAccountID,
			DisplayName: profile.DisplayName, Protocol: profile.Protocol,
			ModelID: profile.ModelID, AuthMode: profile.AuthMode,
			CredentialRevision: profile.CredentialRevision,
			PolicyVersion:      profile.PolicyVersion, PolicyRevision: profile.PolicyRevision,
			PolicyDigest: profile.PolicyDigest, TrustDomain: profile.TrustDomain,
			RetentionMode: profile.RetentionMode, DataRegion: profile.DataRegion,
		})
	}
	if auth.Status == "available" && auth.AuthMode == "native_auth" {
		profiles = append(profiles, ConversationProviderProfile{
			ProfileID:      provider.OpenCodeConversationProfileID,
			HarnessAdapter: "opencode", ProviderID: "opencode", DisplayName: "OpenCode",
			Protocol: "opencode_agent", ModelID: openCodeConversationDefaultModel(),
			AuthMode: "native_auth",
		})
		profiles = append(profiles, ConversationProviderProfile{
			ProfileID:      provider.CodexConversationProfileID,
			HarnessAdapter: "codex", ProviderID: "openai", DisplayName: "Codex",
			Protocol: "openai_responses", ModelID: "codex-default",
			AuthMode: "native_auth",
		})
	}
	return profiles
}

func setupRoleOptionPreviews(
	catalog LocalProductSetupCatalog,
) []SetupRoleOptionPreview {
	profiles := make(map[string]loomruntime.RuntimeProfile, len(catalog.RuntimeProfiles))
	for _, profile := range catalog.RuntimeProfiles {
		profiles[profile.ID] = profile
	}
	result := make([]SetupRoleOptionPreview, 0, len(catalog.RoleOptions))
	for _, option := range catalog.RoleOptions {
		profile, ok := profiles[option.RuntimeProfileID]
		if !ok {
			continue
		}
		budgetAvailable := profile.Budget != nil
		budgetUnits := int64(0)
		if profile.Budget != nil {
			budgetUnits = *profile.Budget
		}
		result = append(result, SetupRoleOptionPreview{
			ID: option.ID, Kind: option.Kind,
			AgentDefinitionID:          option.AgentDefinitionID,
			RuntimeProfileID:           option.RuntimeProfileID,
			RuntimeInstanceID:          option.RuntimeInstanceID,
			HarnessAdapter:             profile.AdapterType,
			ProviderID:                 profile.ProviderID,
			ProviderAccountID:          profile.ProviderAccountID,
			ModelID:                    profile.ModelID,
			AuthMode:                   string(profile.AuthMode),
			CredentialRevision:         profile.CredentialRevision,
			ReasoningEffort:            profile.ReasoningEffort,
			TimeoutMilliseconds:        profile.Timeout.Milliseconds(),
			BudgetAvailable:            budgetAvailable,
			BudgetUnits:                budgetUnits,
			RequiredCapabilities:       append([]string{}, profile.RequiredCapabilities...),
			RemoteToolEnrollmentID:     profile.RemoteToolEnrollmentID,
			RemoteToolEnrollmentDigest: profile.RemoteToolEnrollmentDigest,
			SkillRevisionIDs:           append([]string{}, option.SkillRevisionIDs...),
			PermissionIDs:              append([]string{}, option.PermissionIDs...),
			ResourceIDs:                append([]string{}, option.ResourceIDs...),
			Responsibility:             option.Responsibility,
		})
	}
	return result
}

func (service *LocalProductSetupService) providerDirectory(
	ctx context.Context,
	auth NativeAuthObservation,
) []ProviderDirectoryEntry {
	catalog := provider.Catalog()
	entries := make([]ProviderDirectoryEntry, 0, len(catalog))
	for _, descriptor := range catalog {
		entry := ProviderDirectoryEntry{
			ProviderID:             descriptor.ID,
			DisplayName:            descriptor.DisplayName,
			Category:               descriptor.Category,
			Protocol:               descriptor.Protocol,
			AuthMode:               descriptor.AuthMode,
			ConnectionKind:         descriptor.ConnectionKind,
			Status:                 "unconfigured",
			SupportsModelDiscovery: descriptor.SupportsModelDiscovery,
		}
		switch descriptor.ConnectionKind {
		case "native_runtime":
			entry.Status = auth.Status
			entry.Reason = auth.Reason
		case "managed_cloud":
			entry.Status = "external_setup_required"
		case "local_runtime":
			entry.Status = "not_detected"
		case "custom_endpoint":
			entry.Status = "endpoint_required"
		default:
			status, err := service.credentials.CredentialStatus(ctx, descriptor.ID)
			if err == nil && status.ProviderID == descriptor.ID &&
				closedCredentialStatus(string(status.Status), string(status.Reason)) {
				entry.CredentialReference = status.CredentialReference
				entry.Revision = status.Revision
				entry.Status = string(status.Status)
				entry.Reason = string(status.Reason)
			}
		}
		entries = append(entries, entry)
	}
	return entries
}

func (service *LocalProductSetupService) StartBuilder(
	ctx context.Context,
	command BuilderStartCommand,
) (BuilderSessionView, error) {
	if service == nil || ctx == nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	catalog, domainCatalog, view, err := service.currentCatalog(ctx)
	if err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	if len(catalog.RoleOptions) < 2 {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	draftID, err := service.identity.NextSetupID("draft")
	if err != nil || !validSetupText(draftID, 128) {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	session := &localProductBuilderSession{
		view: BuilderSessionView{
			SchemaVersion: localProductSetupSchemaVersion,
			DraftID:       draftID,
			Revision:      1,
			Source:        command.Source,
			CatalogDigest: catalog.CatalogDigest,
			ViewVersion:   view.Version(),
			Question:      emptyBuilderQuestion(),
			Preview:       emptyBuilderPreview(),
		},
		catalog:                  catalog,
		domainCatalog:            domainCatalog,
		subagentFallbackRoleIDs:  make(map[string]string),
		subagentParallelRoleIDs:  make(map[string][]string),
		subagentSynthesisRoleIDs: make(map[string]string),
	}
	scoped := service.withSessionCatalog(session)
	switch command.Source {
	case BuilderSourceBlank:
		session.mainRoleID, session.subRoleIDs, err =
			scoped.defaultSetupRoles()
		if err != nil {
			return BuilderSessionView{}, err
		}
		// Form-first: present the full editable draft immediately instead of
		// a sequential Q&A gauntlet. The user fills name/purpose inline;
		// confirmation stays explicit (CanConfirm requires name + purpose and
		// a compatible, gap-free preview), matching the template/saved-team
		// review-before-confirm contract.
		if err := scoped.initializeStructuredSession(session); err != nil {
			return BuilderSessionView{}, err
		}
		if err := scoped.makeSessionProposed(session); err != nil {
			return BuilderSessionView{}, err
		}
	case BuilderSourceTemplate:
		template, ok := scoped.setupTemplate(
			command.SourceID,
			command.SourceVersion,
			command.SourceDigest,
		)
		if !ok {
			return BuilderSessionView{}, ErrBuilderNotFound
		}
		session.name = template.Name
		session.purpose = template.Purpose
		session.mainRoleID = template.MainRoleID
		session.subRoleIDs = append([]string{}, template.SubagentRoleIDs...)
		session.sourceID = template.ID
		session.sourceVersion = template.Version
		session.sourceDigest = template.Digest
		if err := scoped.initializeStructuredSession(session); err != nil {
			return BuilderSessionView{}, err
		}
		if err := scoped.makeSessionProposed(session); err != nil {
			return BuilderSessionView{}, err
		}
	case BuilderSourceSavedTeam:
		if err := scoped.loadSavedTeamSession(session, command); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
		if err := scoped.initializeStructuredSession(session); err != nil {
			return BuilderSessionView{}, err
		}
		if err := scoped.makeSessionProposed(session); err != nil {
			return BuilderSessionView{}, err
		}
	default:
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	scoped.refreshBuilderView(session)
	service.mu.Lock()
	defer service.mu.Unlock()
	if _, exists := service.sessions[draftID]; exists {
		return BuilderSessionView{}, ErrBuilderConflict
	}
	service.sessions[draftID] = session
	return cloneBuilderSessionView(session.view), nil
}

func (service *LocalProductSetupService) AnswerBuilder(
	ctx context.Context,
	command BuilderAnswerCommand,
) (BuilderSessionView, error) {
	if service == nil || ctx == nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	if err := ctx.Err(); err != nil {
		return BuilderSessionView{}, err
	}
	currentCatalog, _, currentView, err := service.currentCatalog(ctx)
	if err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
		currentCatalog.CatalogDigest,
		currentView.Version(),
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	scoped := service.withSessionCatalog(session)
	if session.view.Question.ID == "" ||
		command.QuestionID != session.view.Question.ID ||
		!validSetupText(command.Answer, 2048) {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	nextQuestionID, err := scoped.applyBuilderAnswer(
		session,
		command.QuestionID,
		command.Answer,
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	content, err := scoped.buildSessionContent(session)
	if err != nil {
		return BuilderSessionView{}, err
	}
	var nextQuestion *teams.DraftQuestion
	if nextQuestionID != "" {
		question := scoped.domainQuestion(nextQuestionID)
		nextQuestion = &question
	}
	structured, err := teams.AnswerStructuredTeamDraft(
		session.structured,
		session.structured.Revision(),
		session.domainCatalog,
		command.QuestionID,
		command.Answer,
		content,
		nextQuestion,
	)
	if err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	session.structured = structured
	session.view.Revision++
	session.view.Question = scoped.builderQuestion(nextQuestionID)
	scoped.refreshBuilderView(session)
	return cloneBuilderSessionView(session.view), nil
}

func (service *LocalProductSetupService) EditBuilder(
	ctx context.Context,
	command BuilderEditCommand,
) (BuilderSessionView, error) {
	if service == nil || ctx == nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	if err := ctx.Err(); err != nil {
		return BuilderSessionView{}, err
	}
	currentCatalog, _, currentView, err := service.currentCatalog(ctx)
	if err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
		currentCatalog.CatalogDigest,
		currentView.Version(),
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	scoped := service.withSessionCatalog(session)
	if !validSetupText(command.Value, 2048) {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	switch command.Field {
	case "team_name":
		if command.RoleAgentDefinitionID != "" {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		session.name = command.Value
	case "purpose":
		if command.RoleAgentDefinitionID != "" {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		session.purpose = command.Value
	case "main_role":
		if command.RoleAgentDefinitionID != "" {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		if !scoped.validRoleChoice(command.Value, "main") {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		session.mainRoleID = command.Value
		session.mainParallelRoleIDs = nil
		session.mainSynthesisRoleID = ""
		if !scoped.validFallbackRoleChoice(
			session, "main", "", session.mainFallbackRoleID,
		) {
			session.mainFallbackRoleID = ""
		}
	case "subagent_role":
		if !scoped.validRoleChoice(command.Value, "subagent") {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		_, index, err := scoped.builderRoleSelection(
			session, "subagent", command.RoleAgentDefinitionID,
		)
		if err != nil {
			return BuilderSessionView{}, err
		}
		choice, _ := scoped.setupRole(command.Value)
		for peerIndex, roleID := range session.subRoleIDs {
			if peerIndex == index {
				continue
			}
			peer, ok := scoped.setupRole(roleID)
			if ok && peer.AgentDefinitionID == choice.AgentDefinitionID {
				return BuilderSessionView{}, ErrInvalidLocalProductSetup
			}
		}
		delete(session.subagentFallbackRoleIDs, command.RoleAgentDefinitionID)
		delete(session.subagentParallelRoleIDs, command.RoleAgentDefinitionID)
		delete(session.subagentSynthesisRoleIDs, command.RoleAgentDefinitionID)
		session.subRoleIDs[index] = command.Value
	case "subagent_add":
		if command.RoleAgentDefinitionID != "" ||
			len(session.subRoleIDs) >= teams.MaxTeamAgentCount-1 ||
			!scoped.validRoleChoice(command.Value, "subagent") {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		choice, _ := scoped.setupRole(command.Value)
		for _, roleID := range session.subRoleIDs {
			current, ok := scoped.setupRole(roleID)
			if ok && current.AgentDefinitionID == choice.AgentDefinitionID {
				return BuilderSessionView{}, ErrInvalidLocalProductSetup
			}
		}
		session.subRoleIDs = append(session.subRoleIDs, command.Value)
	case "subagent_remove":
		if len(session.subRoleIDs) <= 1 || command.RoleAgentDefinitionID == "" {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		_, index, err := scoped.builderRoleSelection(
			session, "subagent", command.RoleAgentDefinitionID,
		)
		if err != nil {
			return BuilderSessionView{}, err
		}
		delete(session.subagentFallbackRoleIDs, command.RoleAgentDefinitionID)
		delete(session.subagentParallelRoleIDs, command.RoleAgentDefinitionID)
		delete(session.subagentSynthesisRoleIDs, command.RoleAgentDefinitionID)
		session.subRoleIDs = append(
			session.subRoleIDs[:index], session.subRoleIDs[index+1:]...,
		)
	case "main_fallback_role", "subagent_fallback_role":
		kind := strings.TrimSuffix(command.Field, "_fallback_role")
		if len(service.builderParallelRoleIDs(
			session, kind, command.RoleAgentDefinitionID,
		)) != 0 {
			return BuilderSessionView{}, ErrBuilderIncompatible
		}
		if command.Value == "none" {
			service.setBuilderFallbackRole(
				session, kind, command.RoleAgentDefinitionID, "",
			)
			break
		}
		if !scoped.validFallbackRoleChoice(
			session, kind, command.RoleAgentDefinitionID, command.Value,
		) {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		service.setBuilderFallbackRole(
			session, kind, command.RoleAgentDefinitionID, command.Value,
		)
	case "main_parallel_route_add", "subagent_parallel_route_add":
		kind := strings.TrimSuffix(command.Field, "_parallel_route_add")
		if service.builderFallbackRoleID(
			session, kind, command.RoleAgentDefinitionID,
		) != "" {
			return BuilderSessionView{}, ErrBuilderIncompatible
		}
		if err := scoped.addBuilderParallelRole(
			session, kind, command.RoleAgentDefinitionID, command.Value,
		); err != nil {
			return BuilderSessionView{}, err
		}
	case "main_parallel_route_remove", "subagent_parallel_route_remove":
		kind := strings.TrimSuffix(command.Field, "_parallel_route_remove")
		if err := scoped.removeBuilderParallelRole(
			session, kind, command.RoleAgentDefinitionID, command.Value,
		); err != nil {
			return BuilderSessionView{}, err
		}
	case "main_harness_route", "subagent_harness_route",
		"main_provider_account_route", "subagent_provider_account_route":
		kind := "main"
		if strings.HasPrefix(command.Field, "subagent_") {
			kind = "subagent"
		}
		if err := service.adoptBuilderRoleExecutionRoute(
			session, kind, command.RoleAgentDefinitionID,
			command.Field, command.Value,
		); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
		fallbackRoleID := service.builderFallbackRoleID(
			session, kind, command.RoleAgentDefinitionID,
		)
		if !scoped.validFallbackRoleChoice(
			session, kind, command.RoleAgentDefinitionID, fallbackRoleID,
		) {
			service.setBuilderFallbackRole(
				session, kind, command.RoleAgentDefinitionID, "",
			)
		}
	case "main_reasoning_effort", "subagent_reasoning_effort":
		kind := strings.TrimSuffix(command.Field, "_reasoning_effort")
		effort := command.Value
		if effort == "provider_default" {
			effort = ""
		}
		if err := service.customizeBuilderRoleProfile(
			session,
			kind,
			command.RoleAgentDefinitionID,
			func(profile *loomruntime.RuntimeProfile) error {
				profile.ReasoningEffort = effort
				return nil
			},
		); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
	case "main_model", "subagent_model":
		kind := strings.TrimSuffix(command.Field, "_model")
		if err := service.customizeBuilderRoleProfile(
			session,
			kind,
			command.RoleAgentDefinitionID,
			func(profile *loomruntime.RuntimeProfile) error {
				profile.ModelID = command.Value
				return nil
			},
		); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
	case "main_timeout_seconds", "subagent_timeout_seconds":
		kind := strings.TrimSuffix(command.Field, "_timeout_seconds")
		seconds, parseErr := strconv.ParseInt(command.Value, 10, 64)
		if parseErr != nil || seconds < 1 || seconds > 3_600 {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		if err := service.customizeBuilderRoleProfile(
			session,
			kind,
			command.RoleAgentDefinitionID,
			func(profile *loomruntime.RuntimeProfile) error {
				profile.Timeout = time.Duration(seconds) * time.Second
				return nil
			},
		); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
	case "main_budget_units", "subagent_budget_units":
		kind := strings.TrimSuffix(command.Field, "_budget_units")
		if err := service.customizeBuilderRoleProfile(
			session,
			kind,
			command.RoleAgentDefinitionID,
			func(profile *loomruntime.RuntimeProfile) error {
				if command.Value == "none" {
					profile.Budget = nil
					return nil
				}
				units, parseErr := strconv.ParseInt(command.Value, 10, 64)
				if parseErr != nil || units < 0 || units > 1_000_000_000 {
					return ErrInvalidLocalProductSetup
				}
				profile.Budget = &units
				return nil
			},
		); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
	case "main_remote_tool_enrollment", "subagent_remote_tool_enrollment":
		kind := strings.TrimSuffix(command.Field, "_remote_tool_enrollment")
		if err := service.selectBuilderRoleRemoteToolEnrollment(
			session,
			kind,
			command.RoleAgentDefinitionID,
			command.Value,
		); err != nil {
			return BuilderSessionView{}, err
		}
		scoped = service.withSessionCatalog(session)
	default:
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	content, err := scoped.buildSessionContent(session)
	if err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	structured, err := teams.EditStructuredTeamDraft(
		session.structured,
		session.structured.Revision(),
		session.domainCatalog,
		content,
		nil,
	)
	if err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	session.structured = structured
	session.view.Revision++
	session.view.Question = emptyBuilderQuestion()
	scoped.refreshBuilderView(session)
	return cloneBuilderSessionView(session.view), nil
}

func (service *LocalProductSetupService) adoptBuilderRoleExecutionRoute(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	field string,
	sourceRoleID string,
) error {
	if session == nil || (kind != "main" && kind != "subagent") {
		return ErrInvalidLocalProductSetup
	}
	isHarnessRoute := field == kind+"_harness_route"
	isProviderAccountRoute := field == kind+"_provider_account_route"
	if !isHarnessRoute && !isProviderAccountRoute {
		return ErrInvalidLocalProductSetup
	}
	currentRoleID, roleIndex, err := service.builderRoleSelection(
		session, kind, roleAgentDefinitionID,
	)
	if err != nil {
		return err
	}
	scoped := service.withSessionCatalog(session)
	current, currentOK := scoped.setupRole(currentRoleID)
	source, sourceOK := scoped.setupRole(sourceRoleID)
	if !currentOK || !sourceOK || source.Kind != kind ||
		current.Kind != kind ||
		source.AgentDefinitionID != current.AgentDefinitionID {
		return ErrInvalidLocalProductSetup
	}
	currentProfile, currentProfileOK := scoped.setupRuntimeProfile(
		current.RuntimeProfileID,
	)
	sourceProfile, sourceProfileOK := scoped.setupRuntimeProfile(
		source.RuntimeProfileID,
	)
	sourceObservation, sourceObservationOK := scoped.setupRuntimeObservation(
		source.RuntimeInstanceID,
	)
	if !currentProfileOK || !sourceProfileOK || !sourceObservationOK ||
		!containsSetupString(sourceObservation.ModelIDs, sourceProfile.ModelID) {
		return ErrBuilderIncompatible
	}
	if _, err := loomruntime.FreezeExecutionBinding(
		sourceProfile, sourceObservation.Instance,
	); err != nil {
		return ErrBuilderIncompatible
	}
	if isProviderAccountRoute &&
		(sourceProfile.AdapterType != currentProfile.AdapterType ||
			source.RuntimeInstanceID != current.RuntimeInstanceID) {
		return ErrBuilderIncompatible
	}
	if isHarnessRoute &&
		(sourceProfile.ProviderID != currentProfile.ProviderID ||
			sourceProfile.ProviderAccountID != currentProfile.ProviderAccountID ||
			sourceProfile.AuthMode != currentProfile.AuthMode ||
			sourceProfile.EndpointFingerprint != currentProfile.EndpointFingerprint ||
			sourceProfile.CredentialReference != currentProfile.CredentialReference ||
			sourceProfile.CredentialRevision != currentProfile.CredentialRevision ||
			sourceProfile.ModelID != currentProfile.ModelID) {
		return ErrBuilderIncompatible
	}

	profile := currentProfile
	runtimeInstanceID := current.RuntimeInstanceID
	if isHarnessRoute {
		profile.AdapterType = sourceProfile.AdapterType
		runtimeInstanceID = source.RuntimeInstanceID
	} else {
		profile.ProviderID = sourceProfile.ProviderID
		profile.ProviderAccountID = sourceProfile.ProviderAccountID
		profile.AuthMode = sourceProfile.AuthMode
		profile.EndpointFingerprint = sourceProfile.EndpointFingerprint
		profile.CredentialReference = sourceProfile.CredentialReference
		profile.CredentialRevision = sourceProfile.CredentialRevision
		profile.ModelID = sourceProfile.ModelID
		// A Provider Account route change invalidates any Enrollment bound
		// to the previous account; the selection must be made again.
		profile.RemoteToolEnrollmentID = ""
		profile.RemoteToolEnrollmentDigest = ""
	}
	targetObservation, targetObservationOK := scoped.setupRuntimeObservation(
		runtimeInstanceID,
	)
	if !targetObservationOK ||
		!containsSetupString(targetObservation.ModelIDs, profile.ModelID) {
		return ErrBuilderIncompatible
	}
	profile.ID = ""
	profileIdentity, err := json.Marshal(struct {
		Domain            string
		Kind              string
		AgentDefinitionID string
		RuntimeInstanceID string
		Profile           loomruntime.RuntimeProfile
	}{
		Domain: "loom.builder-agent-route.v2", Kind: kind,
		AgentDefinitionID: current.AgentDefinitionID,
		RuntimeInstanceID: runtimeInstanceID,
		Profile:           profile,
	})
	if err != nil {
		return ErrInvalidLocalProductSetup
	}
	profileDigest := sha256.Sum256(profileIdentity)
	suffix := hex.EncodeToString(profileDigest[:12])
	profile.ID = "loom-profile-" + suffix
	profile, err = loomruntime.NewRuntimeProfile(profile)
	if err != nil {
		return ErrBuilderIncompatible
	}
	if _, err := loomruntime.FreezeExecutionBinding(
		profile, targetObservation.Instance,
	); err != nil {
		return ErrBuilderIncompatible
	}

	custom := cloneSetupRoleOption(current)
	custom.ID = "loom-role-" + kind + "-" + suffix
	custom.RuntimeProfileID = profile.ID
	custom.RuntimeInstanceID = runtimeInstanceID

	candidate := cloneSetupCatalog(session.catalog)
	removeProfileID := current.RuntimeProfileID
	if !strings.HasPrefix(removeProfileID, "loom-profile-") {
		removeProfileID = ""
	}
	profiles := make(
		[]loomruntime.RuntimeProfile,
		0,
		len(candidate.RuntimeProfiles)+1,
	)
	for _, existing := range candidate.RuntimeProfiles {
		if existing.ID != removeProfileID && existing.ID != profile.ID {
			profiles = append(profiles, existing)
		}
	}
	candidate.RuntimeProfiles = append(profiles, profile)
	options := make([]SetupRoleOption, 0, len(candidate.RoleOptions)+1)
	for _, option := range candidate.RoleOptions {
		if option.ID != custom.ID &&
			(option.ID != current.ID ||
				!strings.HasPrefix(current.ID, "loom-role-"+kind+"-")) {
			options = append(options, option)
		}
	}
	candidate.RoleOptions = append(options, custom)
	if err := validateSetupCatalog(candidate); err != nil {
		return ErrBuilderIncompatible
	}
	domainCatalog, err := buildSetupDomainCatalog(candidate)
	if err != nil {
		return ErrBuilderIncompatible
	}
	session.catalog = candidate
	session.domainCatalog = domainCatalog
	if kind == "main" {
		session.mainRoleID = custom.ID
	} else {
		session.subRoleIDs[roleIndex] = custom.ID
	}
	return nil
}

func (service *LocalProductSetupService) selectBuilderRoleRemoteToolEnrollment(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	value string,
) error {
	if session == nil || (kind != "main" && kind != "subagent") ||
		!validSetupText(value, 512) {
		return ErrInvalidLocalProductSetup
	}
	roleID, _, err := service.builderRoleSelection(
		session, kind, roleAgentDefinitionID,
	)
	if err != nil {
		return err
	}
	scoped := service.withSessionCatalog(session)
	option, ok := scoped.setupRole(roleID)
	if !ok {
		return ErrBuilderIncompatible
	}
	profile, ok := scoped.setupRuntimeProfile(option.RuntimeProfileID)
	if !ok {
		return ErrBuilderIncompatible
	}
	if value == "none" {
		if profile.RemoteToolEnrollmentID == "" {
			return nil
		}
		return service.customizeBuilderRoleProfile(
			session, kind, roleAgentDefinitionID,
			func(candidate *loomruntime.RuntimeProfile) error {
				candidate.RemoteToolEnrollmentID = ""
				candidate.RemoteToolEnrollmentDigest = ""
				return nil
			},
		)
	}
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 || !validRemoteToolBackendEnrollmentSelectionSyntax(
		parts[0], parts[1],
	) {
		return ErrInvalidLocalProductSetup
	}
	enrollment, ok := service.authoritativeRemoteToolEnrollment(
		profile.ProviderID, profile.ProviderAccountID, parts[0],
	)
	if !ok {
		return ErrRemoteToolEnrollmentSelectionRejected
	}
	policy, policyOK := service.projection.GlobalReadView().ProviderAccountPolicy(
		profile.ProviderID, profile.ProviderAccountID,
	)
	if enrollment.Status() != work.RemoteToolBackendEnrollmentActive ||
		!policyOK ||
		!remoteToolBackendEnrollmentUsesPolicy(enrollment, policy) ||
		enrollment.Digest() != parts[1] ||
		!work.BuiltInRemoteToolBackendCatalog().RemoteToolBackendAdapterSupported(
			enrollment.AdapterID(),
		) {
		return ErrRemoteToolEnrollmentSelectionRejected
	}
	return service.customizeBuilderRoleProfile(
		session, kind, roleAgentDefinitionID,
		func(candidate *loomruntime.RuntimeProfile) error {
			candidate.RemoteToolEnrollmentID = enrollment.EnrollmentID()
			candidate.RemoteToolEnrollmentDigest = enrollment.Digest()
			return nil
		},
	)
}

func (service *LocalProductSetupService) authoritativeRemoteToolEnrollment(
	providerID string,
	providerAccountID string,
	enrollmentID string,
) (work.RemoteToolBackendEnrollment, bool) {
	if service == nil || service.projection == nil {
		return work.RemoteToolBackendEnrollment{}, false
	}
	for _, enrollment := range service.projection.GlobalReadView().RemoteToolBackendEnrollments(
		providerID, providerAccountID,
	) {
		if enrollment.EnrollmentID() == enrollmentID && enrollment.Valid() {
			return enrollment, true
		}
	}
	return work.RemoteToolBackendEnrollment{}, false
}

func validRemoteToolBackendEnrollmentSelectionSyntax(enrollmentID string, digest string) bool {
	return validSetupText(enrollmentID, 128) &&
		len(digest) == 64 &&
		digest == strings.ToLower(digest)
}

func (service *LocalProductSetupService) customizeBuilderRoleProfile(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	mutate func(*loomruntime.RuntimeProfile) error,
) error {
	if session == nil || mutate == nil ||
		(kind != "main" && kind != "subagent") {
		return ErrInvalidLocalProductSetup
	}
	roleID, roleIndex, err := service.builderRoleSelection(
		session, kind, roleAgentDefinitionID,
	)
	if err != nil {
		return err
	}
	scoped := service.withSessionCatalog(session)
	option, ok := scoped.setupRole(roleID)
	if !ok {
		return ErrBuilderIncompatible
	}
	profile, ok := scoped.setupRuntimeProfile(option.RuntimeProfileID)
	if !ok || mutate(&profile) != nil {
		return ErrInvalidLocalProductSetup
	}
	observation, ok := scoped.setupRuntimeObservation(option.RuntimeInstanceID)
	if !ok || !containsSetupString(observation.ModelIDs, profile.ModelID) {
		return ErrBuilderIncompatible
	}
	profile.ID = ""
	digestInput, err := json.Marshal(struct {
		Kind              string
		AgentDefinitionID string
		RuntimeInstanceID string
		Profile           loomruntime.RuntimeProfile
	}{kind, option.AgentDefinitionID, option.RuntimeInstanceID, profile})
	if err != nil {
		return ErrInvalidLocalProductSetup
	}
	digest := sha256.Sum256(digestInput)
	suffix := hex.EncodeToString(digest[:12])
	profile.ID = "loom-profile-" + suffix
	profile, err = loomruntime.NewRuntimeProfile(profile)
	if err != nil {
		return ErrBuilderIncompatible
	}
	if _, err := loomruntime.FreezeExecutionBinding(profile, observation.Instance); err != nil {
		return ErrBuilderIncompatible
	}
	customOption := cloneSetupRoleOption(option)
	customOption.ID = "loom-role-" + kind + "-" + suffix
	customOption.RuntimeProfileID = profile.ID
	removeProfileID := option.RuntimeProfileID
	if !strings.HasPrefix(removeProfileID, "loom-profile-") {
		removeProfileID = ""
	}
	removeOptionID := option.ID
	if !strings.HasPrefix(removeOptionID, "loom-role-"+kind+"-") {
		removeOptionID = ""
	}
	profiles := make([]loomruntime.RuntimeProfile, 0, len(session.catalog.RuntimeProfiles)+1)
	for _, current := range session.catalog.RuntimeProfiles {
		if current.ID != removeProfileID && current.ID != profile.ID {
			profiles = append(profiles, current)
		}
	}
	session.catalog.RuntimeProfiles = append(profiles, profile)
	options := make([]SetupRoleOption, 0, len(session.catalog.RoleOptions)+1)
	for _, current := range session.catalog.RoleOptions {
		if current.ID != removeOptionID && current.ID != customOption.ID {
			options = append(options, current)
		}
	}
	session.catalog.RoleOptions = append(options, customOption)
	if kind == "main" {
		session.mainRoleID = customOption.ID
	} else {
		session.subRoleIDs[roleIndex] = customOption.ID
	}
	return nil
}

func (service *LocalProductSetupService) builderRoleSelection(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
) (string, int, error) {
	if session == nil {
		return "", -1, ErrInvalidLocalProductSetup
	}
	if kind == "main" {
		if roleAgentDefinitionID != "" {
			return "", -1, ErrInvalidLocalProductSetup
		}
		return session.mainRoleID, -1, nil
	}
	if kind != "subagent" || roleAgentDefinitionID == "" {
		return "", -1, ErrInvalidLocalProductSetup
	}
	matchedIndex := -1
	scoped := service.withSessionCatalog(session)
	for index, roleID := range session.subRoleIDs {
		option, ok := scoped.setupRole(roleID)
		if !ok || option.AgentDefinitionID != roleAgentDefinitionID {
			continue
		}
		if matchedIndex >= 0 {
			return "", -1, ErrBuilderIncompatible
		}
		matchedIndex = index
	}
	if matchedIndex < 0 {
		return "", -1, ErrInvalidLocalProductSetup
	}
	return session.subRoleIDs[matchedIndex], matchedIndex, nil
}

func (service *LocalProductSetupService) setBuilderFallbackRole(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	roleID string,
) {
	if kind == "main" {
		if roleAgentDefinitionID != "" {
			return
		}
		session.mainFallbackRoleID = roleID
	} else if kind == "subagent" && roleAgentDefinitionID != "" {
		if session.subagentFallbackRoleIDs == nil {
			session.subagentFallbackRoleIDs = make(map[string]string)
		}
		if roleID == "" {
			delete(session.subagentFallbackRoleIDs, roleAgentDefinitionID)
		} else {
			session.subagentFallbackRoleIDs[roleAgentDefinitionID] = roleID
		}
	}
}

func (service *LocalProductSetupService) builderFallbackRoleID(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
) string {
	if session == nil {
		return ""
	}
	if kind == "main" {
		if roleAgentDefinitionID != "" {
			return ""
		}
		return session.mainFallbackRoleID
	}
	if kind == "subagent" {
		return session.subagentFallbackRoleIDs[roleAgentDefinitionID]
	}
	return ""
}

func (service *LocalProductSetupService) builderParallelRoleIDs(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
) []string {
	if session == nil {
		return nil
	}
	if kind == "main" && roleAgentDefinitionID == "" {
		return append([]string(nil), session.mainParallelRoleIDs...)
	}
	if kind == "subagent" && roleAgentDefinitionID != "" {
		return append(
			[]string(nil),
			session.subagentParallelRoleIDs[roleAgentDefinitionID]...,
		)
	}
	return nil
}

func (service *LocalProductSetupService) builderSynthesisRoleID(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
) string {
	if session == nil {
		return ""
	}
	if kind == "main" && roleAgentDefinitionID == "" {
		return session.mainSynthesisRoleID
	}
	if kind == "subagent" && roleAgentDefinitionID != "" {
		return session.subagentSynthesisRoleIDs[roleAgentDefinitionID]
	}
	return ""
}

func (service *LocalProductSetupService) setBuilderParallelRoles(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	roleIDs []string,
	synthesisRoleID string,
) {
	roleIDs = append([]string(nil), roleIDs...)
	sort.Strings(roleIDs)
	if kind == "main" && roleAgentDefinitionID == "" {
		session.mainParallelRoleIDs = roleIDs
		session.mainSynthesisRoleID = synthesisRoleID
		return
	}
	if kind != "subagent" || roleAgentDefinitionID == "" {
		return
	}
	if session.subagentParallelRoleIDs == nil {
		session.subagentParallelRoleIDs = make(map[string][]string)
	}
	if session.subagentSynthesisRoleIDs == nil {
		session.subagentSynthesisRoleIDs = make(map[string]string)
	}
	if len(roleIDs) == 0 {
		delete(session.subagentParallelRoleIDs, roleAgentDefinitionID)
		delete(session.subagentSynthesisRoleIDs, roleAgentDefinitionID)
		return
	}
	session.subagentParallelRoleIDs[roleAgentDefinitionID] = roleIDs
	session.subagentSynthesisRoleIDs[roleAgentDefinitionID] = synthesisRoleID
}

func (service *LocalProductSetupService) addBuilderParallelRole(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	roleID string,
) error {
	if !service.validFallbackRoleChoice(
		session, kind, roleAgentDefinitionID, roleID,
	) {
		return ErrBuilderIncompatible
	}
	roleIDs := service.builderParallelRoleIDs(
		session, kind, roleAgentDefinitionID,
	)
	if len(roleIDs) >= 2 || containsSetupString(roleIDs, roleID) {
		return ErrInvalidLocalProductSetup
	}
	primaryRoleID, _, err := service.builderRoleSelection(
		session, kind, roleAgentDefinitionID,
	)
	if err != nil {
		return err
	}
	roleIDs = append(roleIDs, roleID)
	service.setBuilderParallelRoles(
		session, kind, roleAgentDefinitionID, roleIDs, primaryRoleID,
	)
	return nil
}

func (service *LocalProductSetupService) removeBuilderParallelRole(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	roleID string,
) error {
	roleIDs := service.builderParallelRoleIDs(
		session, kind, roleAgentDefinitionID,
	)
	index := -1
	for currentIndex, current := range roleIDs {
		if current == roleID {
			index = currentIndex
			break
		}
	}
	if index < 0 {
		return ErrInvalidLocalProductSetup
	}
	roleIDs = append(roleIDs[:index], roleIDs[index+1:]...)
	synthesisRoleID := service.builderSynthesisRoleID(
		session, kind, roleAgentDefinitionID,
	)
	if len(roleIDs) == 0 {
		synthesisRoleID = ""
	}
	service.setBuilderParallelRoles(
		session, kind, roleAgentDefinitionID, roleIDs, synthesisRoleID,
	)
	return nil
}

func (service *LocalProductSetupService) validFallbackRoleChoice(
	session *localProductBuilderSession,
	kind string,
	roleAgentDefinitionID string,
	fallbackRoleID string,
) bool {
	if fallbackRoleID == "" {
		return true
	}
	primaryRoleID, _, err := service.builderRoleSelection(
		session, kind, roleAgentDefinitionID,
	)
	if err != nil {
		return false
	}
	primary, primaryOK := service.setupRole(primaryRoleID)
	fallback, fallbackOK := service.setupRole(fallbackRoleID)
	if !primaryOK || !fallbackOK || primary.ID == fallback.ID ||
		primary.Kind != kind || fallback.Kind != kind ||
		primary.AgentDefinitionID != fallback.AgentDefinitionID ||
		primary.RuntimeProfileID == fallback.RuntimeProfileID {
		return false
	}
	profile, profileOK := service.setupRuntimeProfile(fallback.RuntimeProfileID)
	observation, observationOK := service.setupRuntimeObservation(fallback.RuntimeInstanceID)
	if !profileOK || !observationOK ||
		!containsSetupString(observation.ModelIDs, profile.ModelID) {
		return false
	}
	_, err = loomruntime.FreezeExecutionBinding(profile, observation.Instance)
	return err == nil
}

func (service *LocalProductSetupService) ValidateBuilder(
	ctx context.Context,
	command BuilderValidateCommand,
) (BuilderSessionView, error) {
	if service == nil || ctx == nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	if err := ctx.Err(); err != nil {
		return BuilderSessionView{}, err
	}
	currentCatalog, _, currentView, err := service.currentCatalog(ctx)
	if err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
		currentCatalog.CatalogDigest,
		currentView.Version(),
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	scoped := service.withSessionCatalog(session)
	if session.view.Question.ID != "" {
		return cloneBuilderSessionView(session.view), nil
	}
	if _, err := teams.CheckStructuredTeamDraftAcceptable(
		session.structured,
		session.structured.Revision(),
		session.domainCatalog,
	); err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	scoped.refreshBuilderView(session)
	return cloneBuilderSessionView(session.view), nil
}

func (service *LocalProductSetupService) ConfirmBuilder(
	ctx context.Context,
	command BuilderConfirmCommand,
) (BuilderConfirmation, error) {
	if service == nil || ctx == nil {
		return BuilderConfirmation{}, ErrInvalidLocalProductSetup
	}
	if !command.Confirm {
		return BuilderConfirmation{}, ErrBuilderConfirmationRequired
	}
	if err := ctx.Err(); err != nil {
		return BuilderConfirmation{}, err
	}
	currentCatalog, _, currentView, err := service.currentCatalog(ctx)
	if err != nil {
		return BuilderConfirmation{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
		currentCatalog.CatalogDigest,
		currentView.Version(),
	)
	if err != nil {
		return BuilderConfirmation{}, err
	}
	scoped := service.withSessionCatalog(session)
	if !session.view.CanConfirm ||
		command.BindingDigest != session.view.BindingDigest ||
		!validSetupText(command.DefinitionID, 128) {
		return BuilderConfirmation{}, ErrBuilderConflict
	}
	if _, err := teams.CheckStructuredTeamDraftAcceptable(
		session.structured,
		session.structured.Revision(),
		session.domainCatalog,
	); err != nil {
		return BuilderConfirmation{}, ErrBuilderIncompatible
	}
	scope := teams.TeamDefinitionScope(command.Scope)
	scopeIdentity := agents.ScopeIdentity{}
	if scope == teams.TeamDefinitionScopeProject {
		if !validSetupText(command.ProjectID, 128) {
			return BuilderConfirmation{}, ErrInvalidLocalProductSetup
		}
		scopeIdentity.ProjectID = command.ProjectID
	}
	roles, configuration, err := scoped.definitionInputs(session)
	if err != nil {
		return BuilderConfirmation{}, ErrBuilderIncompatible
	}
	definition, err := teams.BuildTeamDefinition(
		teams.TeamDefinitionInput{
			ID:            command.DefinitionID,
			Version:       1,
			Scope:         scope,
			ScopeIdentity: scopeIdentity,
			Name:          session.name,
			Status:        teams.TeamDefinitionActive,
			Roles:         roles,
		},
		session.catalog.AgentDefinitions,
		session.catalog.RuntimeProfiles,
	)
	if err != nil {
		return BuilderConfirmation{}, ErrBuilderIncompatible
	}
	view := service.projection.GlobalReadView()
	if _, exists := view.Head(
		"team-definition/" + command.DefinitionID,
	); exists {
		return BuilderConfirmation{}, ErrBuilderConflict
	}
	commandID, err := service.identity.NextSetupID("save-team")
	if err != nil {
		return BuilderConfirmation{}, ErrInvalidLocalProductSetup
	}
	_, err = service.writer.SaveTeamDefinition(
		ctx,
		state.TeamDefinitionSaveCommand{
			CommandID:       commandID,
			ExpectedHead:    0,
			OccurredAt:      service.now().UTC(),
			Definition:      definition,
			Definitions:     session.catalog.AgentDefinitions,
			RuntimeProfiles: session.catalog.RuntimeProfiles,
			DraftID:         session.view.DraftID,
			DraftRevision:   session.view.Revision,
			CatalogDigest:   session.view.CatalogDigest,
			ContentDigest:   session.view.ContentDigest,
			BindingDigest:   session.view.BindingDigest,
			Configuration:   configuration,
		},
	)
	if err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) {
			return BuilderConfirmation{}, ErrBuilderConflict
		}
		return BuilderConfirmation{}, ErrInvalidLocalProductSetup
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return BuilderConfirmation{}, ErrInvalidLocalProductSetup
	}
	delete(service.sessions, session.view.DraftID)
	return BuilderConfirmation{
		TeamDefinitionID:      definition.ID(),
		TeamDefinitionVersion: definition.Version(),
		TeamDefinitionDigest:  definition.Digest(),
		Status:                "active",
		TeamInstanceCreated:   false,
		RunCreated:            false,
	}, nil
}

func (service *LocalProductSetupService) ArchiveTeam(
	ctx context.Context,
	command TeamStatusCommand,
) (SetupSavedTeamPreview, error) {
	return service.setTeamStatus(ctx, command, "archived")
}

func (service *LocalProductSetupService) RestoreTeam(
	ctx context.Context,
	command TeamStatusCommand,
) (SetupSavedTeamPreview, error) {
	return service.setTeamStatus(ctx, command, "active")
}

func (service *LocalProductSetupService) ConfigureCredential(
	ctx context.Context,
	command CredentialSetupCommand,
) (CredentialSetupResult, error) {
	defer clearSetupBytes(command.Secret)
	providerAccountID := setupProviderAccountID(
		command.ProviderID, command.ProviderAccountID,
	)
	if service == nil ||
		ctx == nil ||
		!provider.SupportsBrokeredCredential(command.ProviderID) ||
		!credentials.ValidProviderAccountIdentifier(
			command.ProviderID, providerAccountID,
		) ||
		command.CredentialReference != "" ||
		command.OperationID != "" ||
		len(command.Secret) == 0 ||
		len(command.Secret) > 8192 ||
		service.credentialMutator == nil {
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	reference, err := service.identity.NextSetupID("credential-ref")
	if err != nil {
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	commandID, err := service.identity.NextSetupID("configure-credential")
	if err != nil {
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	result, err := service.credentialMutator.Configure(
		ctx,
		credentials.CredentialCommand{
			CommandID:           commandID,
			ProviderID:          command.ProviderID,
			ProviderAccountID:   providerAccountID,
			CredentialReference: reference,
			ExpectedRevision:    command.ExpectedRevision,
			OccurredAt:          service.now().UTC(),
			Secret:              command.Secret,
		},
	)
	return setupCredentialResult(result, err)
}

func (service *LocalProductSetupService) VerifyCredential(
	ctx context.Context,
	command CredentialSetupCommand,
) (CredentialSetupResult, error) {
	return service.mutateCredential(ctx, command, "verify")
}

func (service *LocalProductSetupService) ReplaceCredential(
	ctx context.Context,
	command CredentialSetupCommand,
) (CredentialSetupResult, error) {
	defer clearSetupBytes(command.Secret)
	return service.mutateCredential(ctx, command, "replace")
}

func (service *LocalProductSetupService) RevokeCredential(
	ctx context.Context,
	command CredentialSetupCommand,
) (CredentialSetupResult, error) {
	return service.mutateCredential(ctx, command, "revoke")
}

func (service *LocalProductSetupService) mutateCredential(
	ctx context.Context,
	command CredentialSetupCommand,
	action string,
) (CredentialSetupResult, error) {
	providerAccountID := setupProviderAccountID(
		command.ProviderID, command.ProviderAccountID,
	)
	if service == nil ||
		ctx == nil ||
		!provider.SupportsBrokeredCredential(command.ProviderID) ||
		!credentials.ValidProviderAccountIdentifier(
			command.ProviderID, providerAccountID,
		) ||
		!validSetupCredentialReference(command.CredentialReference) ||
		command.ExpectedRevision <= 0 ||
		service.credentialMutator == nil {
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	if action != "replace" && len(command.Secret) != 0 ||
		action == "replace" &&
			(len(command.Secret) == 0 || len(command.Secret) > 8192) {
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	commandID := ""
	var err error
	if action == "verify" {
		if !validSetupOperationID(command.OperationID) {
			return CredentialSetupResult{}, ErrCredentialSetupUnavailable
		}
		commandID = "verify-credential-" + command.OperationID
	} else {
		if command.OperationID != "" {
			return CredentialSetupResult{}, ErrCredentialSetupUnavailable
		}
		commandID, err = service.identity.NextSetupID(action + "-credential")
		if err != nil {
			return CredentialSetupResult{}, ErrCredentialSetupUnavailable
		}
	}
	brokerCommand := credentials.CredentialCommand{
		CommandID:           commandID,
		ProviderID:          command.ProviderID,
		ProviderAccountID:   providerAccountID,
		CredentialReference: command.CredentialReference,
		ExpectedRevision:    command.ExpectedRevision,
		OccurredAt:          service.now().UTC(),
		Secret:              command.Secret,
	}
	var result credentials.MetadataResult
	switch action {
	case "verify":
		result, err = service.credentialMutator.Verify(ctx, brokerCommand)
	case "replace":
		result, err = service.credentialMutator.Replace(ctx, brokerCommand)
	case "revoke":
		result, err = service.credentialMutator.Revoke(ctx, brokerCommand)
	default:
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	return setupCredentialResult(result, err)
}

func setupProviderAccountID(providerID, providerAccountID string) string {
	if providerAccountID == "" && providerID != "" {
		return providerID + ".primary"
	}
	return providerAccountID
}

func (service *LocalProductSetupService) setTeamStatus(
	ctx context.Context,
	command TeamStatusCommand,
	status string,
) (SetupSavedTeamPreview, error) {
	if service == nil ||
		ctx == nil ||
		!validSetupText(command.DefinitionID, 128) ||
		command.ExpectedHead <= 0 {
		return SetupSavedTeamPreview{}, ErrInvalidLocalProductSetup
	}
	commandID, err := service.identity.NextSetupID(status + "-team")
	if err != nil {
		return SetupSavedTeamPreview{}, ErrInvalidLocalProductSetup
	}
	if _, err := service.writer.SetTeamDefinitionStatus(
		ctx,
		state.TeamDefinitionStatusCommand{
			CommandID:    commandID,
			DefinitionID: command.DefinitionID,
			ExpectedHead: command.ExpectedHead,
			OccurredAt:   service.now().UTC(),
			Status:       status,
		},
	); err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) {
			return SetupSavedTeamPreview{}, ErrBuilderConflict
		}
		return SetupSavedTeamPreview{}, ErrInvalidLocalProductSetup
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return SetupSavedTeamPreview{}, ErrInvalidLocalProductSetup
	}
	record, ok := service.projection.GlobalReadView().TeamDefinition(
		command.DefinitionID,
	)
	if !ok {
		return SetupSavedTeamPreview{}, ErrBuilderNotFound
	}
	head, ok := service.projection.GlobalReadView().Head(
		"team-definition/" + command.DefinitionID,
	)
	if !ok || head.Sequence <= 0 {
		return SetupSavedTeamPreview{}, ErrInvalidLocalProductSetup
	}
	return SetupSavedTeamPreview{
		ID:               record.ID,
		Version:          record.Version,
		Name:             record.Name,
		Status:           record.Status,
		DefinitionDigest: record.DefinitionDigest,
		StreamHead:       head.Sequence,
	}, nil
}

func buildSetupDomainCatalog(
	catalog LocalProductSetupCatalog,
) (teams.TeamDraftCatalogSnapshot, error) {
	skillIDs := make([]string, len(catalog.SkillRevisions))
	for index, skill := range catalog.SkillRevisions {
		skillIDs[index] = setupSkillKey(skill.ID, skill.Revision)
	}
	resourceIDs := make([]string, len(catalog.Resources))
	for index, resource := range catalog.Resources {
		resourceIDs[index] = resource.ID
	}
	maximum := func(value int) int {
		if value < 1 {
			return 1
		}
		return value
	}
	modelCount := 0
	for _, observation := range catalog.RuntimeDiscovery.Observations() {
		modelCount += len(observation.ModelIDs)
	}
	return teams.BuildTeamDraftCatalog(teams.TeamDraftCatalogInput{
		AgentDefinitions:   catalog.AgentDefinitions,
		RuntimeDiscovery:   catalog.RuntimeDiscovery,
		SkillIDs:           skillIDs,
		MemberIDs:          resourceIDs,
		PermissionIDs:      catalog.Permissions,
		BudgetCeiling:      catalog.BudgetCeiling,
		ConcurrencyCeiling: catalog.ConcurrencyCeiling,
		MaxCounts: teams.TeamDraftCatalogMaxCounts{
			Agents:      maximum(len(catalog.AgentDefinitions)),
			Runtimes:    maximum(len(catalog.RuntimeDiscovery.Observations())),
			Models:      maximum(modelCount),
			Skills:      maximum(len(skillIDs)),
			Members:     maximum(len(resourceIDs)),
			Permissions: maximum(len(catalog.Permissions)),
		},
	})
}

func validateSetupCatalog(catalog LocalProductSetupCatalog) error {
	if catalog.BudgetCeiling < 0 ||
		catalog.ConcurrencyCeiling <= 0 {
		return ErrInvalidLocalProductSetup
	}
	if len(catalog.RoleOptions) == 0 {
		if len(catalog.AgentDefinitions) != 0 ||
			len(catalog.RuntimeProfiles) != 0 ||
			len(catalog.Templates) != 0 {
			return ErrInvalidLocalProductSetup
		}
		return nil
	}
	if len(catalog.RoleOptions) < 2 {
		return ErrInvalidLocalProductSetup
	}
	agentsByID := make(map[string]agents.AgentDefinition)
	for _, definition := range catalog.AgentDefinitions {
		if !validSetupText(definition.ID, 128) {
			return ErrInvalidLocalProductSetup
		}
		if _, duplicate := agentsByID[definition.ID]; duplicate {
			return ErrInvalidLocalProductSetup
		}
		agentsByID[definition.ID] = definition
	}
	profilesByID := make(map[string]loomruntime.RuntimeProfile)
	for _, profile := range catalog.RuntimeProfiles {
		if !validSetupText(profile.ID, 128) {
			return ErrInvalidLocalProductSetup
		}
		if _, duplicate := profilesByID[profile.ID]; duplicate {
			return ErrInvalidLocalProductSetup
		}
		profilesByID[profile.ID] = profile
	}
	observationsByID := make(map[string]loomruntime.RuntimeObservation)
	for _, observation := range catalog.RuntimeDiscovery.Observations() {
		if !validSetupText(observation.Instance.ID, 128) {
			return ErrInvalidLocalProductSetup
		}
		if _, duplicate := observationsByID[observation.Instance.ID]; duplicate {
			return ErrInvalidLocalProductSetup
		}
		observationsByID[observation.Instance.ID] = observation
	}
	skills := make(map[string]SetupSkillRevision)
	for _, skill := range catalog.SkillRevisions {
		key := setupSkillKey(skill.ID, skill.Revision)
		if !validSetupText(skill.ID, 128) ||
			skill.Revision <= 0 ||
			!validSetupDigest(skill.Digest) ||
			!validSetupText(skill.SourceScope, 128) ||
			!validSetupText(skill.Risk, 64) ||
			skill.CompatibleRuntimes == nil {
			return ErrInvalidLocalProductSetup
		}
		if _, duplicate := skills[key]; duplicate {
			return ErrInvalidLocalProductSetup
		}
		skills[key] = skill
	}
	permissions := make(map[string]struct{})
	for _, permission := range catalog.Permissions {
		if !validSetupText(permission, 128) {
			return ErrInvalidLocalProductSetup
		}
		if _, duplicate := permissions[permission]; duplicate {
			return ErrInvalidLocalProductSetup
		}
		permissions[permission] = struct{}{}
	}
	resources := make(map[string]struct{})
	for _, resource := range catalog.Resources {
		if !validSetupText(resource.ID, 128) ||
			!validSetupText(resource.Kind, 64) {
			return ErrInvalidLocalProductSetup
		}
		if _, duplicate := resources[resource.ID]; duplicate {
			return ErrInvalidLocalProductSetup
		}
		resources[resource.ID] = struct{}{}
	}
	roleIDs := make(map[string]struct{}, len(catalog.RoleOptions))
	roleKinds := make(map[string]string, len(catalog.RoleOptions))
	for _, option := range catalog.RoleOptions {
		if !validSetupText(option.ID, 128) ||
			(option.Kind != "main" && option.Kind != "subagent") ||
			!validSetupText(option.AgentDefinitionID, 128) ||
			!validSetupText(option.RuntimeProfileID, 128) ||
			!validSetupText(option.RuntimeInstanceID, 128) ||
			!validSetupText(option.Responsibility, 2048) ||
			option.SkillRevisionIDs == nil ||
			option.PermissionIDs == nil ||
			option.ResourceIDs == nil {
			return ErrInvalidLocalProductSetup
		}
		if _, exists := roleIDs[option.ID]; exists {
			return ErrInvalidLocalProductSetup
		}
		roleIDs[option.ID] = struct{}{}
		roleKinds[option.ID] = option.Kind
		definition, definitionOK := agentsByID[option.AgentDefinitionID]
		profile, profileOK := profilesByID[option.RuntimeProfileID]
		observation, observationOK := observationsByID[option.RuntimeInstanceID]
		if !definitionOK ||
			definition.Status != agents.DefinitionActive ||
			!profileOK ||
			!observationOK {
			return ErrInvalidLocalProductSetup
		}
		if _, err := loomruntime.FreezeExecutionBinding(
			profile,
			observation.Instance,
		); err != nil ||
			!containsSetupString(observation.ModelIDs, profile.ModelID) {
			return ErrInvalidLocalProductSetup
		}
		if setupStringsHaveDuplicates(option.SkillRevisionIDs) ||
			setupStringsHaveDuplicates(option.PermissionIDs) ||
			setupStringsHaveDuplicates(option.ResourceIDs) {
			return ErrInvalidLocalProductSetup
		}
		for _, skillID := range option.SkillRevisionIDs {
			skill, ok := skills[skillID]
			if !ok ||
				!containsSetupString(
					skill.CompatibleRuntimes,
					observation.Instance.AdapterType,
				) {
				return ErrInvalidLocalProductSetup
			}
		}
		for _, permission := range option.PermissionIDs {
			if _, ok := permissions[permission]; !ok {
				return ErrInvalidLocalProductSetup
			}
		}
		for _, resource := range option.ResourceIDs {
			if _, ok := resources[resource]; !ok {
				return ErrInvalidLocalProductSetup
			}
		}
	}
	templateIDs := make(map[string]struct{})
	for _, template := range catalog.Templates {
		if !validSetupText(template.ID, 128) ||
			template.Version <= 0 ||
			!validSetupDigest(template.Digest) ||
			!validSetupText(template.Name, 256) ||
			!validSetupText(template.Purpose, 2048) ||
			roleKinds[template.MainRoleID] != "main" ||
			template.SubagentRoleIDs == nil ||
			len(template.SubagentRoleIDs) == 0 ||
			template.RequestedConcurrency <= 0 ||
			template.RequestedConcurrency > catalog.ConcurrencyCeiling ||
			template.MaximumBudgetCredits < 0 ||
			template.MaximumBudgetCredits > catalog.BudgetCeiling {
			return ErrInvalidLocalProductSetup
		}
		templateKey := fmt.Sprintf("%s@%d", template.ID, template.Version)
		if _, duplicate := templateIDs[templateKey]; duplicate ||
			setupStringsHaveDuplicates(template.SubagentRoleIDs) {
			return ErrInvalidLocalProductSetup
		}
		templateIDs[templateKey] = struct{}{}
		for _, roleID := range template.SubagentRoleIDs {
			if roleKinds[roleID] != "subagent" {
				return ErrInvalidLocalProductSetup
			}
		}
	}
	return nil
}

func setupStringsHaveDuplicates(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func (service *LocalProductSetupService) initializeStructuredSession(
	session *localProductBuilderSession,
) error {
	content, err := service.buildSessionContent(session)
	if err != nil {
		return ErrBuilderIncompatible
	}
	structured, err := teams.NewStructuredTeamDraft(
		session.view.DraftID,
		service.domainCatalog,
		content,
	)
	if err != nil {
		return ErrBuilderIncompatible
	}
	if session.view.Question.ID != "" {
		question := service.domainQuestion(session.view.Question.ID)
		structured, err = teams.PresentStructuredTeamDraft(
			structured,
			structured.Revision(),
			service.domainCatalog,
			&question,
		)
		if err != nil {
			return ErrBuilderIncompatible
		}
	}
	session.structured = structured
	session.hasStructured = true
	return nil
}

func (service *LocalProductSetupService) makeSessionProposed(
	session *localProductBuilderSession,
) error {
	structured, err := teams.PresentStructuredTeamDraft(
		session.structured,
		session.structured.Revision(),
		service.domainCatalog,
		nil,
	)
	if err != nil {
		return ErrBuilderIncompatible
	}
	session.structured = structured
	session.view.Question = emptyBuilderQuestion()
	return nil
}

func (service *LocalProductSetupService) buildSessionContent(
	session *localProductBuilderSession,
) (teams.TeamDraftContentSnapshot, error) {
	roleIDs := append([]string{session.mainRoleID}, session.subRoleIDs...)
	options := make([]SetupRoleOption, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		option, ok := service.setupRole(roleID)
		if !ok {
			return teams.TeamDraftContentSnapshot{}, ErrBuilderIncompatible
		}
		options = append(options, option)
	}
	main := options[0]
	subagents := options[1:]
	references := teams.TeamDraftReferences{
		MainAgentDefinitionID: main.AgentDefinitionID,
		RequestedBudget:       service.catalog.BudgetCeiling,
		RequestedConcurrency:  1,
	}
	runtimeIDs := make(map[string]struct{})
	runtimeModels := make(map[string]teams.RuntimeModelReference)
	skills := make(map[string]struct{})
	resources := make(map[string]struct{})
	permissions := make(map[string]struct{})
	roles := make([]teams.TeamDraftRoleSelection, 0, len(options))
	for _, option := range options {
		if option.Kind == "subagent" {
			references.SubAgentDefinitionIDs = append(
				references.SubAgentDefinitionIDs,
				option.AgentDefinitionID,
			)
		}
		profile, ok := service.setupRuntimeProfile(option.RuntimeProfileID)
		if !ok {
			return teams.TeamDraftContentSnapshot{}, ErrBuilderIncompatible
		}
		runtimeIDs[option.RuntimeInstanceID] = struct{}{}
		runtimeModels[option.RuntimeInstanceID+"\x00"+profile.ModelID] =
			teams.RuntimeModelReference{
				RuntimeInstanceID: option.RuntimeInstanceID,
				ModelID:           profile.ModelID,
			}
		for _, id := range option.SkillRevisionIDs {
			skills[id] = struct{}{}
		}
		for _, id := range option.ResourceIDs {
			resources[id] = struct{}{}
		}
		for _, id := range option.PermissionIDs {
			permissions[id] = struct{}{}
		}
		roles = append(roles, teams.TeamDraftRoleSelection{
			AgentDefinitionID: option.AgentDefinitionID,
			RuntimeProfile:    profile,
			RuntimeInstanceID: option.RuntimeInstanceID,
			SkillIDs:          append([]string{}, option.SkillRevisionIDs...),
			MemberIDs:         append([]string{}, option.ResourceIDs...),
			PermissionIDs:     append([]string{}, option.PermissionIDs...),
		})
	}
	references.RuntimeInstanceIDs = sortedSetupKeys(runtimeIDs)
	references.SkillIDs = sortedSetupKeys(skills)
	references.MemberIDs = sortedSetupKeys(resources)
	references.PermissionIDs = sortedSetupKeys(permissions)
	for _, pair := range runtimeModels {
		references.RuntimeModels = append(references.RuntimeModels, pair)
	}
	sort.Slice(references.RuntimeModels, func(i, j int) bool {
		if references.RuntimeModels[i].RuntimeInstanceID !=
			references.RuntimeModels[j].RuntimeInstanceID {
			return references.RuntimeModels[i].RuntimeInstanceID <
				references.RuntimeModels[j].RuntimeInstanceID
		}
		return references.RuntimeModels[i].ModelID <
			references.RuntimeModels[j].ModelID
	})
	tasks := make([]teams.TeamDraftTaskCandidate, len(subagents))
	for index, option := range subagents {
		tasks[index] = teams.TeamDraftTaskCandidate{
			ID:                     fmt.Sprintf("task-%d", index+1),
			OwnerAgentDefinitionID: option.AgentDefinitionID,
			DependencyTaskIDs:      []string{},
			AcceptanceCriteria:     []string{"bounded result ready for review"},
		}
	}
	purpose := session.purpose
	if purpose == "" {
		purpose = "Pending bounded purpose confirmation"
	}
	return teams.BuildTeamDraftContent(
		service.domainCatalog,
		teams.TeamDraftContentInput{
			References:          references,
			Roles:               roles,
			Tasks:               tasks,
			CustomerRuleSummary: purpose,
			ApprovalMarkers:     []teams.TeamDraftApprovalMarker{},
			CapabilityGaps:      []teams.TeamDraftCapabilityGap{},
			Limits: teams.TeamDraftContentLimits{
				MaxTasks:                     3,
				MaxDependenciesPerTask:       3,
				MaxAcceptanceCriteriaPerTask: 8,
				MaxApprovalMarkers:           8,
				MaxCapabilityGaps:            8,
			},
		},
	)
}

func (service *LocalProductSetupService) refreshBuilderView(
	session *localProductBuilderSession,
) {
	session.view.Preview = service.buildBuilderPreview(session)
	session.view.CanConfirm = session.view.Question.ID == "" &&
		session.name != "" &&
		session.purpose != "" &&
		session.hasStructured &&
		len(session.view.Preview.CompatibilityGaps) == 0
	if session.hasStructured {
		session.view.ContentDigest = session.structured.ContentDigest()
		session.view.BindingDigest = service.builderBindingDigest(session)
	}
	if session.view.Question.Options == nil {
		session.view.Question.Options = []BuilderQuestionOption{}
	}
}

func (service *LocalProductSetupService) builderBindingDigest(
	session *localProductBuilderSession,
) string {
	base := session.structured.BindingDigest()
	if session.mainFallbackRoleID == "" && len(session.subagentFallbackRoleIDs) == 0 &&
		len(session.mainParallelRoleIDs) == 0 &&
		len(session.subagentParallelRoleIDs) == 0 {
		return base
	}
	type route struct {
		Kind               string   `json:"kind"`
		PrimaryRoleID      string   `json:"primary_role_id"`
		FallbackRoleID     string   `json:"fallback_role_id,omitempty"`
		Approval           bool     `json:"approval_required,omitempty"`
		ParallelRoleIDs    []string `json:"parallel_role_ids,omitempty"`
		SynthesisRoleID    string   `json:"synthesis_role_id,omitempty"`
		ParallelSetVersion int      `json:"parallel_set_version,omitempty"`
	}
	routes := make([]route, 0, 1+len(session.subRoleIDs))
	if session.mainFallbackRoleID != "" {
		routes = append(routes, route{
			Kind: "main", PrimaryRoleID: session.mainRoleID,
			FallbackRoleID: session.mainFallbackRoleID, Approval: true,
		})
	} else if len(session.mainParallelRoleIDs) > 0 {
		routes = append(routes, route{
			Kind: "main", PrimaryRoleID: session.mainRoleID,
			ParallelRoleIDs: append([]string(nil), session.mainParallelRoleIDs...),
			SynthesisRoleID: session.mainSynthesisRoleID, ParallelSetVersion: 1,
		})
	}
	for _, primaryRoleID := range session.subRoleIDs {
		primary, ok := service.setupRole(primaryRoleID)
		if !ok {
			continue
		}
		if fallbackRoleID := session.subagentFallbackRoleIDs[primary.AgentDefinitionID]; fallbackRoleID != "" {
			routes = append(routes, route{
				Kind: "subagent", PrimaryRoleID: primaryRoleID,
				FallbackRoleID: fallbackRoleID, Approval: true,
			})
		} else if parallelRoleIDs := session.subagentParallelRoleIDs[primary.AgentDefinitionID]; len(parallelRoleIDs) > 0 {
			routes = append(routes, route{
				Kind: "subagent", PrimaryRoleID: primaryRoleID,
				ParallelRoleIDs:    append([]string(nil), parallelRoleIDs...),
				SynthesisRoleID:    session.subagentSynthesisRoleIDs[primary.AgentDefinitionID],
				ParallelSetVersion: 1,
			})
		}
	}
	encoded, err := json.Marshal(struct {
		Domain string  `json:"domain"`
		Base   string  `json:"base_binding_digest"`
		Routes []route `json:"routes"`
	}{"loom.builder-route-set.v2", base, routes})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (service *LocalProductSetupService) buildBuilderPreview(
	session *localProductBuilderSession,
) BuilderPreview {
	preview := emptyBuilderPreview()
	preview.Name = session.name
	preview.Purpose = session.purpose
	preview.RequestedConcurrency = 1
	preview.MaximumBudgetCredits = service.catalog.BudgetCeiling
	preview.EstimatedMaximumCost = fmt.Sprintf(
		"Non-authoritative estimate: up to %d credits",
		service.catalog.BudgetCeiling,
	)
	roleIDs := append([]string{session.mainRoleID}, session.subRoleIDs...)
	permissionSet := make(map[string]struct{})
	resourceSet := make(map[string]struct{})
	for _, roleID := range roleIDs {
		option, ok := service.setupRole(roleID)
		if !ok {
			preview.CompatibilityGaps = append(
				preview.CompatibilityGaps,
				"role_not_found",
			)
			continue
		}
		definition, definitionOK := service.setupAgent(option.AgentDefinitionID)
		profile, profileOK := service.setupRuntimeProfile(option.RuntimeProfileID)
		runtimePreview, runtimeOK := service.setupRuntime(option.RuntimeInstanceID)
		observation, observationOK := service.setupRuntimeObservation(
			option.RuntimeInstanceID,
		)
		compatible := definitionOK &&
			profileOK &&
			runtimeOK &&
			observationOK
		reason := ""
		if compatible {
			if _, err := loomruntime.FreezeExecutionBinding(
				profile,
				observation.Instance,
			); err != nil {
				compatible = false
				reason = "runtime_incompatible"
			} else if !containsSetupString(
				observation.ModelIDs,
				profile.ModelID,
			) {
				compatible = false
				reason = "model_unavailable"
			}
		}
		if !compatible {
			if reason == "" {
				reason = "incompatible_binding"
			}
			preview.CompatibilityGaps = append(
				preview.CompatibilityGaps,
				reason,
			)
		}
		skills := make([]SetupSkillRevision, 0, len(option.SkillRevisionIDs))
		for _, skillID := range option.SkillRevisionIDs {
			if skill, found := service.setupSkill(skillID); found {
				skills = append(skills, skill)
			}
		}
		authMode := ""
		modelID := ""
		harnessAdapter := ""
		providerID := ""
		providerAccountID := ""
		credentialRevision := int64(0)
		reasoningEffort := ""
		timeoutMilliseconds := int64(0)
		budgetAvailable := false
		budgetUnits := int64(0)
		requiredCapabilities := []string{}
		if profileOK {
			authMode = string(profile.AuthMode)
			modelID = profile.ModelID
			harnessAdapter = profile.AdapterType
			providerID = profile.ProviderID
			providerAccountID = profile.ProviderAccountID
			credentialRevision = profile.CredentialRevision
			reasoningEffort = profile.ReasoningEffort
			timeoutMilliseconds = profile.Timeout.Milliseconds()
			budgetAvailable = profile.Budget != nil
			if profile.Budget != nil {
				budgetUnits = *profile.Budget
			}
			requiredCapabilities = sortedSetupStrings(profile.RequiredCapabilities)
			runtimePreview.ModelID = modelID
		}
		remoteToolEnrollmentID := ""
		remoteToolEnrollmentDigest := ""
		if profileOK && profile.RemoteToolEnrollmentID != "" {
			remoteToolEnrollmentID = profile.RemoteToolEnrollmentID
			remoteToolEnrollmentDigest = profile.RemoteToolEnrollmentDigest
		}
		roleTargetID := ""
		if option.Kind == "subagent" {
			roleTargetID = option.AgentDefinitionID
		}
		fallbackRoleID := service.builderFallbackRoleID(
			session, option.Kind, roleTargetID,
		)
		fallback, fallbackOK := service.setupRole(fallbackRoleID)
		fallbackProfile, fallbackProfileOK := service.setupRuntimeProfile(fallback.RuntimeProfileID)
		fallbackConfigured := fallbackRoleID != "" && fallbackOK && fallbackProfileOK
		if fallbackRoleID != "" && !service.validFallbackRoleChoice(
			session, option.Kind, roleTargetID, fallbackRoleID,
		) {
			fallbackConfigured = false
			compatible = false
			reason = "fallback_incompatible"
			preview.CompatibilityGaps = append(preview.CompatibilityGaps, reason)
		}
		role := BuilderRolePreview{
			Kind:                       option.Kind,
			AgentDefinitionID:          option.AgentDefinitionID,
			Responsibility:             option.Responsibility,
			Runtime:                    runtimePreview,
			RuntimeProfileID:           option.RuntimeProfileID,
			HarnessAdapter:             harnessAdapter,
			ProviderID:                 providerID,
			ProviderAccountID:          providerAccountID,
			ModelID:                    modelID,
			AuthMode:                   authMode,
			CredentialRevision:         credentialRevision,
			ReasoningEffort:            reasoningEffort,
			TimeoutMilliseconds:        timeoutMilliseconds,
			BudgetAvailable:            budgetAvailable,
			BudgetUnits:                budgetUnits,
			RequiredCapabilities:       requiredCapabilities,
			RemoteToolEnrollmentID:     remoteToolEnrollmentID,
			RemoteToolEnrollmentDigest: remoteToolEnrollmentDigest,
			FallbackConfigured:         fallbackConfigured,
			FallbackApprovalRequired:   fallbackConfigured,
			ParallelRoutes:             []BuilderExecutionRoutePreview{},
			Skills:                     skills,
			PermissionIDs:              append([]string{}, option.PermissionIDs...),
			ResourceIDs:                append([]string{}, option.ResourceIDs...),
			Compatible:                 compatible,
			CompatibilityReason:        reason,
		}
		if fallbackConfigured {
			role.FallbackRuntimeProfileID = fallback.RuntimeProfileID
			role.FallbackHarnessAdapter = fallbackProfile.AdapterType
			role.FallbackProviderID = fallbackProfile.ProviderID
			role.FallbackProviderAccountID = fallbackProfile.ProviderAccountID
			role.FallbackModelID = fallbackProfile.ModelID
			role.FallbackAuthMode = string(fallbackProfile.AuthMode)
			role.FallbackCredentialRevision = fallbackProfile.CredentialRevision
			role.FallbackReasoningEffort = fallbackProfile.ReasoningEffort
			role.FallbackTimeoutMilliseconds = fallbackProfile.Timeout.Milliseconds()
			role.FallbackBudgetAvailable = fallbackProfile.Budget != nil
			if fallbackProfile.Budget != nil {
				role.FallbackBudgetUnits = *fallbackProfile.Budget
			}
			role.FallbackRequiredCapabilities = sortedSetupStrings(
				fallbackProfile.RequiredCapabilities,
			)
		}
		parallelRoleIDs := service.builderParallelRoleIDs(
			session, option.Kind, roleTargetID,
		)
		if len(parallelRoleIDs) > 0 {
			if fallbackConfigured {
				compatible = false
				reason = "parallel_fallback_conflict"
				preview.CompatibilityGaps = append(preview.CompatibilityGaps, reason)
			}
			role.ParallelRouteSetVersion = 1
			role.ParallelRoutes = append(
				role.ParallelRoutes,
				builderExecutionRoutePreview(profile),
			)
			for _, parallelRoleID := range parallelRoleIDs {
				parallelOption, optionOK := service.setupRole(parallelRoleID)
				parallelProfile, routeProfileOK := service.setupRuntimeProfile(
					parallelOption.RuntimeProfileID,
				)
				if !optionOK || !routeProfileOK ||
					!service.validFallbackRoleChoice(
						session, option.Kind, roleTargetID, parallelRoleID,
					) {
					compatible = false
					reason = "parallel_route_incompatible"
					preview.CompatibilityGaps = append(preview.CompatibilityGaps, reason)
					continue
				}
				role.ParallelRoutes = append(
					role.ParallelRoutes,
					builderExecutionRoutePreview(parallelProfile),
				)
			}
			synthesisRoleID := service.builderSynthesisRoleID(
				session, option.Kind, roleTargetID,
			)
			synthesisOption, optionOK := service.setupRole(synthesisRoleID)
			synthesisProfile, synthesisOK := service.setupRuntimeProfile(
				synthesisOption.RuntimeProfileID,
			)
			if !optionOK || !synthesisOK {
				compatible = false
				reason = "synthesis_route_incompatible"
				preview.CompatibilityGaps = append(preview.CompatibilityGaps, reason)
			} else {
				role.SynthesisRoute = builderExecutionRoutePreview(synthesisProfile)
			}
		}
		role.Compatible = compatible
		role.CompatibilityReason = reason
		if definitionOK {
			role.AgentVersion = definition.Version
			role.AgentScope = string(definition.Scope)
			role.DisplayName = definition.Name
		}
		preview.Roles = append(preview.Roles, role)
		for _, id := range option.PermissionIDs {
			permissionSet[id] = struct{}{}
		}
		for _, id := range option.ResourceIDs {
			resourceSet[id] = struct{}{}
		}
	}
	preview.Permissions = sortedSetupKeys(permissionSet)
	preview.Resources = sortedSetupKeys(resourceSet)
	sort.Strings(preview.CompatibilityGaps)
	return preview
}

func builderExecutionRoutePreview(
	profile loomruntime.RuntimeProfile,
) BuilderExecutionRoutePreview {
	result := BuilderExecutionRoutePreview{
		RuntimeProfileID: profile.ID, HarnessAdapter: profile.AdapterType,
		ProviderID: profile.ProviderID, ProviderAccountID: profile.ProviderAccountID,
		ModelID: profile.ModelID, AuthMode: string(profile.AuthMode),
		CredentialRevision:         profile.CredentialRevision,
		ReasoningEffort:            profile.ReasoningEffort,
		TimeoutMilliseconds:        profile.Timeout.Milliseconds(),
		BudgetAvailable:            profile.Budget != nil,
		RequiredCapabilities:       sortedSetupStrings(profile.RequiredCapabilities),
		RemoteToolEnrollmentID:     profile.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: profile.RemoteToolEnrollmentDigest,
	}
	if profile.Budget != nil {
		result.BudgetUnits = *profile.Budget
	}
	return result
}

func (service *LocalProductSetupService) definitionInputs(
	session *localProductBuilderSession,
) ([]teams.TeamDefinitionRole, state.TeamConfigurationSnapshot, error) {
	roleIDs := append([]string{session.mainRoleID}, session.subRoleIDs...)
	roles := make([]teams.TeamDefinitionRole, 0, len(roleIDs))
	configuration := state.TeamConfigurationSnapshot{
		RequestedConcurrency: 1,
		MaximumBudgetCredits: service.catalog.BudgetCeiling,
		RoleBindings:         make([]state.TeamConfigurationRoleBinding, 0, len(roleIDs)),
	}
	for _, roleID := range roleIDs {
		option, ok := service.setupRole(roleID)
		if !ok {
			return nil, state.TeamConfigurationSnapshot{}, ErrBuilderIncompatible
		}
		profile, ok := service.setupRuntimeProfile(option.RuntimeProfileID)
		if !ok {
			return nil, state.TeamConfigurationSnapshot{}, ErrBuilderIncompatible
		}
		observation, ok := service.setupRuntimeObservation(
			option.RuntimeInstanceID,
		)
		if !ok {
			return nil, state.TeamConfigurationSnapshot{},
				ErrBuilderIncompatible
		}
		if _, err := loomruntime.FreezeExecutionBinding(
			profile,
			observation.Instance,
		); err != nil ||
			!containsSetupString(observation.ModelIDs, profile.ModelID) {
			return nil, state.TeamConfigurationSnapshot{},
				ErrBuilderIncompatible
		}
		kind := teams.TeamDefinitionRoleSubAgent
		if option.Kind == "main" {
			kind = teams.TeamDefinitionRoleMain
		}
		roles = append(roles, teams.TeamDefinitionRole{
			Kind:              kind,
			AgentDefinitionID: option.AgentDefinitionID,
			RuntimeProfileID:  option.RuntimeProfileID,
			Responsibility:    option.Responsibility,
		})
		skills := make(
			[]state.TeamConfigurationSkillRevision,
			0,
			len(option.SkillRevisionIDs),
		)
		for _, id := range option.SkillRevisionIDs {
			skill, found := service.setupSkill(id)
			if !found {
				return nil, state.TeamConfigurationSnapshot{},
					ErrBuilderIncompatible
			}
			skills = append(skills, state.TeamConfigurationSkillRevision{
				ID:       skill.ID,
				Revision: skill.Revision,
				Digest:   skill.Digest,
			})
		}
		configuration.RoleBindings = append(
			configuration.RoleBindings,
			state.TeamConfigurationRoleBinding{
				Kind:              option.Kind,
				AgentDefinitionID: option.AgentDefinitionID,
				RuntimeProfileID:  option.RuntimeProfileID,
				RuntimeInstanceID: option.RuntimeInstanceID,
				ModelID:           profile.ModelID,
				ExecutionProfile:  setupExecutionProfileSnapshot(profile),
				SkillRevisions:    skills,
				PermissionIDs: append(
					[]string{},
					option.PermissionIDs...,
				),
				ResourceIDs: append(
					[]string{},
					option.ResourceIDs...,
				),
			},
		)
		roleTargetID := ""
		if option.Kind == "subagent" {
			roleTargetID = option.AgentDefinitionID
		}
		if fallbackRoleID := service.builderFallbackRoleID(
			session, option.Kind, roleTargetID,
		); fallbackRoleID != "" {
			fallback, found := service.setupRole(fallbackRoleID)
			fallbackProfile, profileFound := service.setupRuntimeProfile(fallback.RuntimeProfileID)
			fallbackObservation, observationFound := service.setupRuntimeObservation(
				fallback.RuntimeInstanceID,
			)
			if !found || !profileFound || !observationFound ||
				!service.validFallbackRoleChoice(
					session, option.Kind, roleTargetID, fallbackRoleID,
				) {
				return nil, state.TeamConfigurationSnapshot{}, ErrBuilderIncompatible
			}
			if _, err := loomruntime.FreezeExecutionBinding(
				fallbackProfile, fallbackObservation.Instance,
			); err != nil {
				return nil, state.TeamConfigurationSnapshot{}, ErrBuilderIncompatible
			}
			binding := &configuration.RoleBindings[len(configuration.RoleBindings)-1]
			binding.FallbackRoute = &state.TeamConfigurationFallbackRoute{
				Version: 1, RuntimeProfileID: fallback.RuntimeProfileID,
				RuntimeInstanceID: fallback.RuntimeInstanceID,
				ModelID:           fallbackProfile.ModelID,
				ExecutionProfile:  setupExecutionProfileSnapshot(fallbackProfile),
				ApprovalRequired:  true,
			}
		} else if parallelRoleIDs := service.builderParallelRoleIDs(
			session, option.Kind, roleTargetID,
		); len(parallelRoleIDs) > 0 {
			additional := make(
				[]state.TeamConfigurationExecutionRoute,
				0,
				len(parallelRoleIDs),
			)
			for _, parallelRoleID := range parallelRoleIDs {
				route, routeErr := service.builderExecutionRoute(
					session, option, parallelRoleID,
				)
				if routeErr != nil {
					return nil, state.TeamConfigurationSnapshot{}, routeErr
				}
				additional = append(additional, route)
			}
			synthesis, routeErr := service.builderExecutionRoute(
				session,
				option,
				service.builderSynthesisRoleID(session, option.Kind, roleTargetID),
			)
			if routeErr != nil {
				return nil, state.TeamConfigurationSnapshot{}, routeErr
			}
			binding := &configuration.RoleBindings[len(configuration.RoleBindings)-1]
			binding.ParallelRouteSet = &state.TeamConfigurationParallelRouteSet{
				Version: 1, AdditionalRoutes: additional,
				SynthesisRoute: synthesis,
			}
		}
	}
	return roles, configuration, nil
}

func (service *LocalProductSetupService) builderExecutionRoute(
	session *localProductBuilderSession,
	primary SetupRoleOption,
	routeRoleID string,
) (state.TeamConfigurationExecutionRoute, error) {
	scoped := service.withSessionCatalog(session)
	option, ok := scoped.setupRole(routeRoleID)
	if !ok || option.Kind != primary.Kind ||
		option.AgentDefinitionID != primary.AgentDefinitionID {
		return state.TeamConfigurationExecutionRoute{}, ErrBuilderIncompatible
	}
	profile, ok := scoped.setupRuntimeProfile(option.RuntimeProfileID)
	observation, observationOK := scoped.setupRuntimeObservation(
		option.RuntimeInstanceID,
	)
	if !ok || !observationOK ||
		!containsSetupString(observation.ModelIDs, profile.ModelID) {
		return state.TeamConfigurationExecutionRoute{}, ErrBuilderIncompatible
	}
	if _, err := loomruntime.FreezeExecutionBinding(
		profile, observation.Instance,
	); err != nil {
		return state.TeamConfigurationExecutionRoute{}, ErrBuilderIncompatible
	}
	return state.TeamConfigurationExecutionRoute{
		Version: 1, RuntimeProfileID: option.RuntimeProfileID,
		RuntimeInstanceID: option.RuntimeInstanceID, ModelID: profile.ModelID,
		ExecutionProfile: setupExecutionProfileSnapshot(profile),
	}, nil
}

func setupExecutionProfileSnapshot(
	profile loomruntime.RuntimeProfile,
) *state.TeamConfigurationExecutionProfile {
	snapshot := &state.TeamConfigurationExecutionProfile{
		Version:                    1,
		ID:                         profile.ID,
		HarnessAdapter:             profile.AdapterType,
		ProviderID:                 profile.ProviderID,
		ProviderAccountID:          profile.ProviderAccountID,
		ModelID:                    profile.ModelID,
		AuthMode:                   profile.AuthMode,
		EndpointFingerprint:        profile.EndpointFingerprint,
		CredentialReference:        profile.CredentialReference,
		CredentialRevision:         profile.CredentialRevision,
		ReasoningEffort:            profile.ReasoningEffort,
		TimeoutNanoseconds:         int64(profile.Timeout),
		RequiredCapabilities:       append([]string(nil), profile.RequiredCapabilities...),
		RemoteToolEnrollmentID:     profile.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: profile.RemoteToolEnrollmentDigest,
	}
	if profile.Budget != nil {
		budget := *profile.Budget
		snapshot.Budget = &budget
	}
	return snapshot
}

func (service *LocalProductSetupService) currentSession(
	draftID string,
	expectedRevision int,
	catalogDigest string,
	viewVersion string,
	currentCatalogDigest string,
	currentViewVersion string,
) (*localProductBuilderSession, error) {
	session, ok := service.sessions[draftID]
	if !ok {
		return nil, ErrBuilderNotFound
	}
	if expectedRevision != session.view.Revision ||
		catalogDigest != session.view.CatalogDigest ||
		viewVersion != session.view.ViewVersion ||
		currentCatalogDigest != session.view.CatalogDigest ||
		currentViewVersion != session.view.ViewVersion {
		return nil, ErrBuilderConflict
	}
	return session, nil
}

func (service *LocalProductSetupService) withSessionCatalog(
	session *localProductBuilderSession,
) *LocalProductSetupService {
	return &LocalProductSetupService{
		journal:             service.journal,
		projection:          service.projection,
		writer:              service.writer,
		catalog:             cloneSetupCatalog(session.catalog),
		domainCatalog:       session.domainCatalog,
		identity:            service.identity,
		now:                 service.now,
		nativeAuth:          service.nativeAuth,
		nativeAuthConnector: service.nativeAuthConnector,
		credentials:         service.credentials,
		credentialMutator:   service.credentialMutator,
	}
}

func (service *LocalProductSetupService) applyBuilderAnswer(
	session *localProductBuilderSession,
	questionID,
	answer string,
) (string, error) {
	switch questionID {
	case "team_name":
		session.name = answer
		return "purpose", nil
	case "purpose":
		session.purpose = answer
		return "main_role", nil
	case "main_role":
		if !service.validRoleChoice(answer, "main") {
			return "", ErrInvalidLocalProductSetup
		}
		session.mainRoleID = answer
		return "subagent_role", nil
	case "subagent_role":
		if !service.validRoleChoice(answer, "subagent") {
			return "", ErrInvalidLocalProductSetup
		}
		session.subRoleIDs = []string{answer}
		return "", nil
	default:
		return "", ErrInvalidLocalProductSetup
	}
}

func (service *LocalProductSetupService) builderQuestion(
	id string,
) BuilderQuestion {
	if id == "" {
		return emptyBuilderQuestion()
	}
	question := BuilderQuestion{
		ID:      id,
		Options: []BuilderQuestionOption{},
	}
	switch id {
	case "team_name":
		question.Prompt = "Name this team"
	case "purpose":
		question.Prompt = "What bounded outcome should this team deliver?"
	case "main_role":
		question.Prompt = "Choose the Main Agent role"
		question.Options = service.roleQuestionOptions("main")
	case "subagent_role":
		question.Prompt = "Choose a SubAgent role"
		question.Options = service.roleQuestionOptions("subagent")
	}
	return question
}

func (service *LocalProductSetupService) domainQuestion(
	id string,
) teams.DraftQuestion {
	question := service.builderQuestion(id)
	return teams.DraftQuestion{ID: question.ID, Prompt: question.Prompt}
}

func (service *LocalProductSetupService) roleQuestionOptions(
	kind string,
) []BuilderQuestionOption {
	options := make([]BuilderQuestionOption, 0)
	for _, role := range service.catalog.RoleOptions {
		if role.Kind == kind {
			options = append(options, BuilderQuestionOption{
				ID:    role.ID,
				Label: role.Responsibility,
			})
		}
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].ID < options[j].ID
	})
	return options
}

func (service *LocalProductSetupService) defaultSetupRoles() (string, []string, error) {
	main := ""
	subs := make([]string, 0)
	for _, option := range service.catalog.RoleOptions {
		switch option.Kind {
		case "main":
			if main == "" || option.ID < main {
				main = option.ID
			}
		case "subagent":
			subs = append(subs, option.ID)
		}
	}
	sort.Strings(subs)
	if main == "" || len(subs) == 0 {
		return "", nil, ErrBuilderIncompatible
	}
	return main, []string{subs[0]}, nil
}

func (service *LocalProductSetupService) loadSavedTeamSession(
	session *localProductBuilderSession,
	command BuilderStartCommand,
) error {
	record, ok := service.projection.GlobalReadView().TeamDefinition(
		command.SourceID,
	)
	if !ok ||
		record.Status != "active" ||
		record.Version != command.SourceVersion ||
		record.DefinitionDigest != command.SourceDigest {
		return ErrBuilderNotFound
	}
	session.name = record.Name
	session.purpose = "Continue from accepted saved TeamDefinition"
	session.sourceID = record.ID
	session.sourceVersion = record.Version
	session.sourceDigest = record.DefinitionDigest
	for _, binding := range record.Configuration.RoleBindings {
		scoped := service.withSessionCatalog(session)
		optionID := scoped.findRoleOption(binding)
		if optionID == "" && binding.ExecutionProfileAvailable {
			if _, exists := scoped.setupRuntimeProfile(binding.RuntimeProfileID); exists {
				return ErrBuilderIncompatible
			}
			if err := service.restoreSavedExecutionProfile(
				session,
				record,
				binding,
			); err != nil {
				return err
			}
			scoped = service.withSessionCatalog(session)
			optionID = scoped.findRoleOption(binding)
		}
		if optionID == "" {
			return ErrBuilderIncompatible
		}
		if binding.Kind == "main" {
			session.mainRoleID = optionID
		} else {
			session.subRoleIDs = append(session.subRoleIDs, optionID)
		}
		if binding.FallbackRouteAvailable {
			fallbackOptionID := scoped.findFallbackRoleOption(binding)
			if fallbackOptionID == "" {
				if err := service.restoreSavedFallbackRoute(
					session,
					record,
					binding,
				); err != nil {
					return err
				}
				scoped = service.withSessionCatalog(session)
				fallbackOptionID = scoped.findFallbackRoleOption(binding)
			}
			roleTargetID := ""
			if binding.Kind == "subagent" {
				roleTargetID = binding.AgentDefinitionID
			}
			if fallbackOptionID == "" || !scoped.validFallbackRoleChoice(
				session,
				binding.Kind,
				roleTargetID,
				fallbackOptionID,
			) {
				return ErrBuilderIncompatible
			}
			service.setBuilderFallbackRole(
				session,
				binding.Kind,
				roleTargetID,
				fallbackOptionID,
			)
		}
		if binding.ParallelRouteSetAvailable {
			routeSet := binding.ParallelRouteSet
			if routeSet.Version != 1 || len(routeSet.AdditionalRoutes) < 1 ||
				len(routeSet.AdditionalRoutes) > 2 || binding.FallbackRouteAvailable {
				return ErrBuilderIncompatible
			}
			roleTargetID := ""
			if binding.Kind == "subagent" {
				roleTargetID = binding.AgentDefinitionID
			}
			parallelRoleIDs := make([]string, 0, len(routeSet.AdditionalRoutes))
			for _, route := range routeSet.AdditionalRoutes {
				routeOptionID := scoped.findExecutionRouteOption(binding, route)
				if routeOptionID == "" || !scoped.validFallbackRoleChoice(
					session, binding.Kind, roleTargetID, routeOptionID,
				) {
					return ErrBuilderIncompatible
				}
				parallelRoleIDs = append(parallelRoleIDs, routeOptionID)
			}
			synthesisRoleID := scoped.findExecutionRouteOption(
				binding, routeSet.SynthesisRoute,
			)
			if synthesisRoleID == "" {
				return ErrBuilderIncompatible
			}
			service.setBuilderParallelRoles(
				session, binding.Kind, roleTargetID,
				parallelRoleIDs, synthesisRoleID,
			)
		}
	}
	if session.mainRoleID == "" || len(session.subRoleIDs) == 0 {
		return ErrBuilderIncompatible
	}
	return nil
}

func (service *LocalProductSetupService) restoreSavedFallbackRoute(
	session *localProductBuilderSession,
	record projection.TeamDefinitionRecord,
	binding projection.TeamConfigurationRoleBinding,
) error {
	if session == nil || !binding.FallbackRouteAvailable {
		return ErrBuilderIncompatible
	}
	route := binding.FallbackRoute
	profileRecord := route.ExecutionProfile
	if route.Version != 1 || !route.ApprovalRequired ||
		profileRecord.Version != 1 ||
		profileRecord.ID != route.RuntimeProfileID ||
		profileRecord.ModelID != route.ModelID ||
		route.RuntimeProfileID == binding.RuntimeProfileID {
		return ErrBuilderIncompatible
	}

	candidate := cloneSetupCatalog(session.catalog)
	scoped := service.withSessionCatalog(session)
	profile, profileExists := scoped.setupRuntimeProfile(route.RuntimeProfileID)
	if !profileExists {
		var err error
		profile, err = runtimeProfileFromSavedRecord(profileRecord)
		if err != nil {
			return err
		}
		candidate.RuntimeProfiles = append(candidate.RuntimeProfiles, profile)
	}
	candidateSession := *session
	candidateSession.catalog = candidate
	scoped = service.withSessionCatalog(&candidateSession)
	observation, ok := scoped.setupRuntimeObservation(route.RuntimeInstanceID)
	if !ok || !containsSetupString(observation.ModelIDs, profile.ModelID) {
		return ErrBuilderIncompatible
	}
	if _, err := loomruntime.FreezeExecutionBinding(
		profile,
		observation.Instance,
	); err != nil {
		return ErrBuilderIncompatible
	}

	responsibility := ""
	for _, role := range record.Roles {
		if role.Kind == binding.Kind &&
			role.AgentDefinitionID == binding.AgentDefinitionID &&
			role.RuntimeProfileID == binding.RuntimeProfileID {
			responsibility = role.Responsibility
			break
		}
	}
	if responsibility == "" {
		return ErrBuilderIncompatible
	}
	skillIDs := make([]string, 0, len(binding.SkillRevisions))
	for _, revision := range binding.SkillRevisions {
		key := setupSkillKey(revision.ID, revision.Revision)
		skill, found := scoped.setupSkill(key)
		if !found || skill.Digest != revision.Digest {
			return ErrBuilderIncompatible
		}
		skillIDs = append(skillIDs, key)
	}
	identityInput, err := json.Marshal(struct {
		Kind              string
		AgentDefinitionID string
		RuntimeProfileID  string
	}{binding.Kind, binding.AgentDefinitionID, route.RuntimeProfileID})
	if err != nil {
		return ErrBuilderIncompatible
	}
	identityDigest := sha256.Sum256(identityInput)
	candidate.RoleOptions = append(candidate.RoleOptions, SetupRoleOption{
		ID: "saved-fallback-role-" + binding.Kind + "-" +
			hex.EncodeToString(identityDigest[:12]),
		Kind:              binding.Kind,
		AgentDefinitionID: binding.AgentDefinitionID,
		RuntimeProfileID:  route.RuntimeProfileID,
		RuntimeInstanceID: route.RuntimeInstanceID,
		SkillRevisionIDs:  skillIDs,
		PermissionIDs:     append([]string(nil), binding.PermissionIDs...),
		ResourceIDs:       append([]string(nil), binding.ResourceIDs...),
		Responsibility:    responsibility,
	})
	if err := validateSetupCatalog(candidate); err != nil {
		return ErrBuilderIncompatible
	}
	domainCatalog, err := buildSetupDomainCatalog(candidate)
	if err != nil {
		return ErrBuilderIncompatible
	}
	session.catalog = candidate
	session.domainCatalog = domainCatalog
	return nil
}

func runtimeProfileFromSavedRecord(
	profileRecord projection.TeamExecutionProfileRecord,
) (loomruntime.RuntimeProfile, error) {
	profileBudget := profileRecord.Budget
	if profileBudget != nil {
		budget := *profileBudget
		profileBudget = &budget
	}
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                   profileRecord.ID,
		AdapterType:          profileRecord.HarnessAdapter,
		ProviderID:           profileRecord.ProviderID,
		ProviderAccountID:    profileRecord.ProviderAccountID,
		ModelID:              profileRecord.ModelID,
		AuthMode:             profileRecord.AuthMode,
		EndpointFingerprint:  profileRecord.EndpointFingerprint,
		CredentialReference:  profileRecord.CredentialReference,
		CredentialRevision:   profileRecord.CredentialRevision,
		ReasoningEffort:      profileRecord.ReasoningEffort,
		RequiredCapabilities: append([]string(nil), profileRecord.RequiredCapabilities...),
		Timeout:              profileRecord.Timeout,
		Budget:               profileBudget,
	})
	if err != nil {
		return loomruntime.RuntimeProfile{}, ErrBuilderIncompatible
	}
	return profile, nil
}

func (service *LocalProductSetupService) restoreSavedExecutionProfile(
	session *localProductBuilderSession,
	record projection.TeamDefinitionRecord,
	binding projection.TeamConfigurationRoleBinding,
) error {
	if session == nil || !binding.ExecutionProfileAvailable {
		return ErrBuilderIncompatible
	}
	profileRecord := binding.ExecutionProfile
	if profileRecord.Version != 1 ||
		profileRecord.ID != binding.RuntimeProfileID ||
		profileRecord.ModelID != binding.ModelID {
		return ErrBuilderIncompatible
	}
	profile, err := runtimeProfileFromSavedRecord(profileRecord)
	if err != nil {
		return err
	}

	candidate := cloneSetupCatalog(session.catalog)
	candidate.RuntimeProfiles = append(candidate.RuntimeProfiles, profile)
	candidateSession := *session
	candidateSession.catalog = candidate
	scoped := service.withSessionCatalog(&candidateSession)
	if _, ok := scoped.setupAgent(binding.AgentDefinitionID); !ok {
		return ErrBuilderIncompatible
	}
	observation, ok := scoped.setupRuntimeObservation(binding.RuntimeInstanceID)
	if !ok || !containsSetupString(observation.ModelIDs, profile.ModelID) {
		return ErrBuilderIncompatible
	}
	if _, err := loomruntime.FreezeExecutionBinding(profile, observation.Instance); err != nil {
		return ErrBuilderIncompatible
	}
	responsibility := ""
	for _, role := range record.Roles {
		if role.Kind == binding.Kind &&
			role.AgentDefinitionID == binding.AgentDefinitionID &&
			role.RuntimeProfileID == binding.RuntimeProfileID {
			responsibility = role.Responsibility
			break
		}
	}
	if responsibility == "" {
		return ErrBuilderIncompatible
	}
	skillIDs := make([]string, 0, len(binding.SkillRevisions))
	for _, revision := range binding.SkillRevisions {
		key := setupSkillKey(revision.ID, revision.Revision)
		skill, found := scoped.setupSkill(key)
		if !found || skill.Digest != revision.Digest {
			return ErrBuilderIncompatible
		}
		skillIDs = append(skillIDs, key)
	}
	for _, permissionID := range binding.PermissionIDs {
		if !containsSetupString(candidate.Permissions, permissionID) {
			return ErrBuilderIncompatible
		}
	}
	for _, resourceID := range binding.ResourceIDs {
		found := false
		for _, resource := range candidate.Resources {
			if resource.ID == resourceID {
				found = true
				break
			}
		}
		if !found {
			return ErrBuilderIncompatible
		}
	}
	identityInput, err := json.Marshal(struct {
		Kind              string
		AgentDefinitionID string
		RuntimeProfileID  string
	}{binding.Kind, binding.AgentDefinitionID, binding.RuntimeProfileID})
	if err != nil {
		return ErrBuilderIncompatible
	}
	identityDigest := sha256.Sum256(identityInput)
	option := SetupRoleOption{
		ID:                "saved-role-" + binding.Kind + "-" + hex.EncodeToString(identityDigest[:12]),
		Kind:              binding.Kind,
		AgentDefinitionID: binding.AgentDefinitionID,
		RuntimeProfileID:  binding.RuntimeProfileID,
		RuntimeInstanceID: binding.RuntimeInstanceID,
		SkillRevisionIDs:  skillIDs,
		PermissionIDs:     append([]string(nil), binding.PermissionIDs...),
		ResourceIDs:       append([]string(nil), binding.ResourceIDs...),
		Responsibility:    responsibility,
	}
	candidate.RoleOptions = append(candidate.RoleOptions, option)
	if err := validateSetupCatalog(candidate); err != nil {
		return ErrBuilderIncompatible
	}
	domainCatalog, err := buildSetupDomainCatalog(candidate)
	if err != nil {
		return ErrBuilderIncompatible
	}
	session.catalog = candidate
	session.domainCatalog = domainCatalog
	return nil
}

func (service *LocalProductSetupService) findRoleOption(
	binding projection.TeamConfigurationRoleBinding,
) string {
	for _, option := range service.catalog.RoleOptions {
		if service.roleOptionMatchesSavedBinding(option, binding) {
			return option.ID
		}
	}
	return ""
}

func (service *LocalProductSetupService) findFallbackRoleOption(
	binding projection.TeamConfigurationRoleBinding,
) string {
	if !binding.FallbackRouteAvailable {
		return ""
	}
	route := binding.FallbackRoute
	for _, option := range service.catalog.RoleOptions {
		if option.Kind != binding.Kind ||
			option.AgentDefinitionID != binding.AgentDefinitionID ||
			option.RuntimeProfileID != route.RuntimeProfileID ||
			option.RuntimeInstanceID != route.RuntimeInstanceID {
			continue
		}
		profile, ok := service.setupRuntimeProfile(option.RuntimeProfileID)
		if ok && profile.ModelID == route.ModelID {
			return option.ID
		}
	}
	return ""
}

func (service *LocalProductSetupService) findExecutionRouteOption(
	binding projection.TeamConfigurationRoleBinding,
	route projection.TeamConfigurationExecutionRouteRecord,
) string {
	for _, option := range service.catalog.RoleOptions {
		if option.Kind != binding.Kind ||
			option.AgentDefinitionID != binding.AgentDefinitionID ||
			option.RuntimeProfileID != route.RuntimeProfileID ||
			option.RuntimeInstanceID != route.RuntimeInstanceID {
			continue
		}
		profile, ok := service.setupRuntimeProfile(option.RuntimeProfileID)
		if ok && profile.ModelID == route.ModelID {
			return option.ID
		}
	}
	return ""
}

func (service *LocalProductSetupService) roleOptionMatchesSavedBinding(
	option SetupRoleOption,
	binding projection.TeamConfigurationRoleBinding,
) bool {
	if option.Kind != binding.Kind ||
		option.AgentDefinitionID != binding.AgentDefinitionID ||
		option.RuntimeProfileID != binding.RuntimeProfileID ||
		option.RuntimeInstanceID != binding.RuntimeInstanceID ||
		!sameSetupStringSet(option.PermissionIDs, binding.PermissionIDs) ||
		!sameSetupStringSet(option.ResourceIDs, binding.ResourceIDs) {
		return false
	}
	profile, ok := service.setupRuntimeProfile(option.RuntimeProfileID)
	if !ok || profile.ModelID != binding.ModelID ||
		len(option.SkillRevisionIDs) != len(binding.SkillRevisions) {
		return false
	}
	skills := make(map[string]string, len(binding.SkillRevisions))
	for _, skill := range binding.SkillRevisions {
		skills[setupSkillKey(skill.ID, skill.Revision)] = skill.Digest
	}
	for _, key := range option.SkillRevisionIDs {
		skill, found := service.setupSkill(key)
		if !found || skills[key] != skill.Digest {
			return false
		}
		delete(skills, key)
	}
	return len(skills) == 0
}

func sameSetupStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[string]int, len(left))
	for _, value := range left {
		values[value]++
	}
	for _, value := range right {
		values[value]--
		if values[value] < 0 {
			return false
		}
	}
	for _, count := range values {
		if count != 0 {
			return false
		}
	}
	return true
}

func (service *LocalProductSetupService) setupTemplate(
	id string,
	version int,
	digest string,
) (SetupTeamTemplate, bool) {
	for _, template := range service.catalog.Templates {
		if template.ID == id &&
			template.Version == version &&
			template.Digest == digest {
			return template, true
		}
	}
	return SetupTeamTemplate{}, false
}

func (service *LocalProductSetupService) setupRole(
	id string,
) (SetupRoleOption, bool) {
	for _, role := range service.catalog.RoleOptions {
		if role.ID == id {
			return cloneSetupRoleOption(role), true
		}
	}
	return SetupRoleOption{}, false
}

func (service *LocalProductSetupService) validRoleChoice(
	id,
	kind string,
) bool {
	role, ok := service.setupRole(id)
	return ok && role.Kind == kind
}

func (service *LocalProductSetupService) setupAgent(
	id string,
) (agents.AgentDefinition, bool) {
	for _, definition := range service.catalog.AgentDefinitions {
		if definition.ID == id &&
			definition.Status == agents.DefinitionActive {
			return definition, true
		}
	}
	return agents.AgentDefinition{}, false
}

func (service *LocalProductSetupService) setupRuntimeProfile(
	id string,
) (loomruntime.RuntimeProfile, bool) {
	for _, profile := range service.catalog.RuntimeProfiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return loomruntime.RuntimeProfile{}, false
}

func (service *LocalProductSetupService) setupRuntime(
	id string,
) (SetupRuntimePreview, bool) {
	for _, runtime := range setupRuntimePreviews(service.catalog) {
		if runtime.RuntimeInstanceID == id {
			return runtime, true
		}
	}
	return SetupRuntimePreview{
		ModelIDs:             []string{},
		ObservedCapabilities: []string{},
	}, false
}

func (service *LocalProductSetupService) setupRuntimeObservation(
	id string,
) (loomruntime.RuntimeObservation, bool) {
	for _, observation := range service.catalog.RuntimeDiscovery.Observations() {
		if observation.Instance.ID == id {
			return observation, true
		}
	}
	return loomruntime.RuntimeObservation{}, false
}

func containsSetupString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validSetupCredentialReference(value string) bool {
	const prefix = "credential-ref-"
	if !strings.HasPrefix(value, prefix) ||
		len(value) <= len(prefix) ||
		len(value) > 128 {
		return false
	}
	for _, character := range value[len(prefix):] {
		if character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '-' {
			continue
		}
		return false
	}
	return true
}

func (service *LocalProductSetupService) setupSkill(
	key string,
) (SetupSkillRevision, bool) {
	for _, skill := range service.catalog.SkillRevisions {
		if setupSkillKey(skill.ID, skill.Revision) == key {
			return cloneSetupSkill(skill), true
		}
	}
	return SetupSkillRevision{}, false
}

func setupRuntimePreviews(
	catalog LocalProductSetupCatalog,
) []SetupRuntimePreview {
	observations := catalog.RuntimeDiscovery.Observations()
	output := make([]SetupRuntimePreview, len(observations))
	for index, observation := range observations {
		output[index] = SetupRuntimePreview{
			RuntimeInstanceID: observation.Instance.ID,
			DisplayName:       observation.Instance.DisplayName,
			AdapterType:       observation.Instance.AdapterType,
			ExecutableVersion: observation.Instance.ExecutableVersion,
			Status:            string(observation.Instance.Status),
			Capacity:          observation.Instance.Capacity,
			ModelIDs:          append([]string{}, observation.ModelIDs...),
			ObservedCapabilities: append(
				[]string{},
				observation.Instance.ObservedCapabilities...,
			),
			SourceProbeID: observation.SourceProbeID,
		}
	}
	return output
}

func setupCredentialResult(
	result credentials.MetadataResult,
	err error,
) (CredentialSetupResult, error) {
	if err != nil {
		if credentials.CredentialFailureStage(err) != "" {
			return CredentialSetupResult{}, err
		}
		switch {
		case errors.Is(err, credentials.ErrCredentialStoreDenied):
			return CredentialSetupResult{}, credentials.ErrCredentialStoreDenied
		case errors.Is(err, credentials.ErrCredentialStoreUnavailable),
			errors.Is(err, credentials.ErrCredentialNotFound):
			return CredentialSetupResult{},
				credentials.ErrCredentialStoreUnavailable
		case errors.Is(err, credentials.ErrCredentialMetadataConflict):
			return CredentialSetupResult{}, ErrBuilderConflict
		case errors.Is(err, credentials.ErrCredentialRollbackFailed):
			return CredentialSetupResult{},
				credentials.ErrCredentialRollbackFailed
		case errors.Is(err, credentials.ErrCredentialRejected):
			return CredentialSetupResult{}, credentials.ErrCredentialRejected
		default:
			return CredentialSetupResult{}, ErrCredentialSetupUnavailable
		}
	}
	return CredentialSetupResult{
		ProviderID: result.ProviderID,
		Revision:   result.Revision,
		Status:     string(result.Status),
		Reason:     string(result.Reason),
	}, nil
}

func cloneSetupCatalog(input LocalProductSetupCatalog) LocalProductSetupCatalog {
	output := input
	output.AgentDefinitions = append([]agents.AgentDefinition{}, input.AgentDefinitions...)
	output.RuntimeProfiles = append([]loomruntime.RuntimeProfile{}, input.RuntimeProfiles...)
	for index := range output.RuntimeProfiles {
		output.RuntimeProfiles[index] = cloneSetupRuntimeProfile(
			output.RuntimeProfiles[index],
		)
	}
	output.SkillRevisions = append([]SetupSkillRevision{}, input.SkillRevisions...)
	for index := range output.SkillRevisions {
		output.SkillRevisions[index] = cloneSetupSkill(output.SkillRevisions[index])
	}
	output.Permissions = append([]string{}, input.Permissions...)
	output.Resources = append([]SetupResourcePointer{}, input.Resources...)
	output.RoleOptions = append([]SetupRoleOption{}, input.RoleOptions...)
	for index := range output.RoleOptions {
		output.RoleOptions[index] = cloneSetupRoleOption(output.RoleOptions[index])
	}
	output.Templates = append([]SetupTeamTemplate{}, input.Templates...)
	for index := range output.Templates {
		output.Templates[index].SubagentRoleIDs = append(
			[]string{},
			output.Templates[index].SubagentRoleIDs...,
		)
	}
	return output
}

func cloneSetupRuntimeProfile(
	input loomruntime.RuntimeProfile,
) loomruntime.RuntimeProfile {
	input.RequiredCapabilities = append(
		[]string(nil),
		input.RequiredCapabilities...,
	)
	if input.Budget != nil {
		budget := *input.Budget
		input.Budget = &budget
	}
	return input
}

func cloneSetupSnapshot(input SetupSnapshot) SetupSnapshot {
	if input.CredentialVault != nil {
		vault := *input.CredentialVault
		input.CredentialVault = &vault
	}
	input.Providers = append([]ProviderDirectoryEntry{}, input.Providers...)
	input.ProviderAccounts = append(
		[]ProviderAccountDirectoryEntry{}, input.ProviderAccounts...,
	)
	for index := range input.ProviderAccounts {
		input.ProviderAccounts[index].RateCards = append(
			[]ProviderModelRateCardDirectoryEntry{},
			input.ProviderAccounts[index].RateCards...,
		)
		input.ProviderAccounts[index].RemoteToolBackends = append(
			[]RemoteToolBackendDirectoryEntry{},
			input.ProviderAccounts[index].RemoteToolBackends...,
		)
		for backendIndex := range input.ProviderAccounts[index].RemoteToolBackends {
			input.ProviderAccounts[index].RemoteToolBackends[backendIndex].AllowedTools = append(
				[]string{},
				input.ProviderAccounts[index].RemoteToolBackends[backendIndex].AllowedTools...,
			)
		}
	}
	input.ConversationProfiles = append(
		[]ConversationProviderProfile{}, input.ConversationProfiles...,
	)
	input.Runtimes = append([]SetupRuntimePreview{}, input.Runtimes...)
	for index := range input.Runtimes {
		input.Runtimes[index].ModelIDs = append(
			[]string{},
			input.Runtimes[index].ModelIDs...,
		)
		input.Runtimes[index].ObservedCapabilities = append(
			[]string{},
			input.Runtimes[index].ObservedCapabilities...,
		)
	}
	input.SavedTeams = append([]SetupSavedTeamPreview{}, input.SavedTeams...)
	input.Templates = append([]SetupTeamTemplate{}, input.Templates...)
	for index := range input.Templates {
		input.Templates[index].SubagentRoleIDs = append(
			[]string{},
			input.Templates[index].SubagentRoleIDs...,
		)
	}
	input.RoleOptions = append([]SetupRoleOptionPreview{}, input.RoleOptions...)
	for index := range input.RoleOptions {
		role := &input.RoleOptions[index]
		role.RequiredCapabilities = append(
			[]string{},
			role.RequiredCapabilities...,
		)
		role.SkillRevisionIDs = append([]string{}, role.SkillRevisionIDs...)
		role.PermissionIDs = append([]string{}, role.PermissionIDs...)
		role.ResourceIDs = append([]string{}, role.ResourceIDs...)
	}
	input.Skills = append([]SetupSkillRevision{}, input.Skills...)
	for index := range input.Skills {
		input.Skills[index] = cloneSetupSkill(input.Skills[index])
	}
	input.Permissions = append([]string{}, input.Permissions...)
	input.Resources = append([]SetupResourcePointer{}, input.Resources...)
	return input
}

func cloneBuilderSessionView(input BuilderSessionView) BuilderSessionView {
	input.Question.Options = append(
		[]BuilderQuestionOption{},
		input.Question.Options...,
	)
	input.Preview.Roles = append(
		[]BuilderRolePreview{},
		input.Preview.Roles...,
	)
	for index := range input.Preview.Roles {
		role := &input.Preview.Roles[index]
		role.Runtime.ModelIDs = append([]string{}, role.Runtime.ModelIDs...)
		role.Runtime.ObservedCapabilities = append(
			[]string{},
			role.Runtime.ObservedCapabilities...,
		)
		role.Skills = append([]SetupSkillRevision{}, role.Skills...)
		for skillIndex := range role.Skills {
			role.Skills[skillIndex] = cloneSetupSkill(
				role.Skills[skillIndex],
			)
		}
		role.PermissionIDs = append([]string{}, role.PermissionIDs...)
		role.ResourceIDs = append([]string{}, role.ResourceIDs...)
		role.RequiredCapabilities = append(
			[]string{},
			role.RequiredCapabilities...,
		)
		role.ParallelRoutes = append(
			[]BuilderExecutionRoutePreview{},
			role.ParallelRoutes...,
		)
		for routeIndex := range role.ParallelRoutes {
			role.ParallelRoutes[routeIndex].RequiredCapabilities = append(
				[]string{},
				role.ParallelRoutes[routeIndex].RequiredCapabilities...,
			)
		}
		role.SynthesisRoute.RequiredCapabilities = append(
			[]string{}, role.SynthesisRoute.RequiredCapabilities...,
		)
	}
	input.Preview.Permissions = append(
		[]string{},
		input.Preview.Permissions...,
	)
	input.Preview.Resources = append([]string{}, input.Preview.Resources...)
	input.Preview.CompatibilityGaps = append(
		[]string{},
		input.Preview.CompatibilityGaps...,
	)
	return input
}

func cloneSetupRoleOption(input SetupRoleOption) SetupRoleOption {
	input.SkillRevisionIDs = append([]string{}, input.SkillRevisionIDs...)
	input.PermissionIDs = append([]string{}, input.PermissionIDs...)
	input.ResourceIDs = append([]string{}, input.ResourceIDs...)
	return input
}

func cloneSetupSkill(input SetupSkillRevision) SetupSkillRevision {
	input.CompatibleRuntimes = append([]string{}, input.CompatibleRuntimes...)
	return input
}

func emptyBuilderQuestion() BuilderQuestion {
	return BuilderQuestion{Options: []BuilderQuestionOption{}}
}

func emptyBuilderPreview() BuilderPreview {
	return BuilderPreview{
		Roles:             []BuilderRolePreview{},
		Permissions:       []string{},
		Resources:         []string{},
		CompatibilityGaps: []string{},
	}
}

func sortedSetupKeys(values map[string]struct{}) []string {
	output := make([]string, 0, len(values))
	for value := range values {
		output = append(output, value)
	}
	sort.Strings(output)
	return output
}

func sortedSetupStrings(values []string) []string {
	output := append([]string{}, values...)
	sort.Strings(output)
	return output
}

func setupSkillKey(id string, revision int) string {
	return fmt.Sprintf("%s@%d", id, revision)
}

func closedNativeAuthStatus(status, reason string) bool {
	switch status {
	case "available":
		return reason == ""
	case "not_logged_in":
		return reason == "not_logged_in"
	case "unsupported":
		return reason == "unknown_output"
	case "unavailable":
		return reason == "timeout" ||
			reason == "identity_changed" ||
			reason == "unavailable"
	default:
		return false
	}
}

func closedCredentialStatus(status, reason string) bool {
	switch status {
	case "configured", "verified", "revoked":
		return reason == ""
	case "rejected":
		return reason == "provider_rejected" ||
			reason == "unavailable" ||
			reason == "timeout"
	case "migration_required":
		return reason == "vault_entry_missing"
	case "recovery_required":
		return reason == "vault_unavailable"
	default:
		return false
	}
}

func validSetupDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validSetupText(value string, maximum int) bool {
	if value == "" ||
		len(value) > maximum ||
		!utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validSetupOperationID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' || value[14] != '4' ||
		!strings.Contains("89ab", string(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func clearSetupBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
