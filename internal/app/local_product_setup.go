package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"
)

const localProductSetupSchemaVersion = 1

var (
	ErrInvalidLocalProductSetup    = errors.New("invalid local product setup")
	ErrBuilderNotFound             = errors.New("builder session not found")
	ErrBuilderConflict             = errors.New("builder session conflict")
	ErrBuilderIncompatible         = errors.New("builder configuration incompatible")
	ErrBuilderConfirmationRequired = errors.New("builder confirmation required")
	ErrCredentialSetupUnavailable  = errors.New("credential setup unavailable")
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

type CredentialStatusSource interface {
	CredentialStatus(
		context.Context,
		string,
	) (credentials.MetadataResult, error)
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
	Writer            *state.LocalProductSetupWriter
	Catalog           LocalProductSetupCatalog
	Identity          SetupIdentitySource
	Now               func() time.Time
	NativeAuth        NativeAuthObserver
	Credentials       CredentialStatusSource
	CredentialMutator CredentialMutator
}

type ProviderSetupStatus struct {
	ProviderID          string `json:"provider_id"`
	AuthMode            string `json:"auth_mode"`
	CredentialReference string `json:"credential_reference"`
	Revision            int64  `json:"revision"`
	Status              string `json:"status"`
	Reason              string `json:"reason"`
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
type SetupRoleOptionPreview = SetupRoleOption

type SetupSnapshot struct {
	SchemaVersion int                        `json:"schema_version"`
	ViewVersion   string                     `json:"view_version"`
	Codex         ProviderSetupStatus        `json:"codex"`
	MiniMax       ProviderSetupStatus        `json:"minimax"`
	Runtimes      []SetupRuntimePreview      `json:"runtimes"`
	SavedTeams    []SetupSavedTeamPreview    `json:"saved_teams"`
	Templates     []SetupTeamTemplatePreview `json:"templates"`
	RoleOptions   []SetupRoleOptionPreview   `json:"role_options"`
	Skills        []SetupSkillRevision       `json:"skills"`
	Permissions   []string                   `json:"permissions"`
	Resources     []SetupResourcePointer     `json:"resources"`
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

type BuilderRolePreview struct {
	Kind                string               `json:"kind"`
	AgentDefinitionID   string               `json:"agent_definition_id"`
	AgentVersion        int                  `json:"agent_version"`
	AgentScope          string               `json:"agent_scope"`
	DisplayName         string               `json:"display_name"`
	Responsibility      string               `json:"responsibility"`
	Runtime             SetupRuntimePreview  `json:"runtime"`
	RuntimeProfileID    string               `json:"runtime_profile_id"`
	ModelID             string               `json:"model_id"`
	AuthMode            string               `json:"auth_mode"`
	Skills              []SetupSkillRevision `json:"skills"`
	PermissionIDs       []string             `json:"permission_ids"`
	ResourceIDs         []string             `json:"resource_ids"`
	Compatible          bool                 `json:"compatible"`
	CompatibilityReason string               `json:"compatibility_reason"`
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
	DraftID          string `json:"draft_id"`
	ExpectedRevision int    `json:"expected_revision"`
	CatalogDigest    string `json:"catalog_digest"`
	ViewVersion      string `json:"view_version"`
	Field            string `json:"field"`
	Value            string `json:"value"`
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
	CredentialReference string `json:"credential_reference"`
	ExpectedRevision    int64  `json:"expected_revision"`
	Secret              []byte `json:"secret"`
}

type CredentialSetupResult struct {
	ProviderID string `json:"provider_id"`
	Revision   int64  `json:"revision"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

type localProductBuilderSession struct {
	view          BuilderSessionView
	name          string
	purpose       string
	mainRoleID    string
	subRoleIDs    []string
	structured    teams.StructuredTeamDraft
	hasStructured bool
	sourceID      string
	sourceVersion int
	sourceDigest  string
}

type LocalProductSetupService struct {
	journal    *journal.Store
	projection interface {
		Rebuild(context.Context) error
		GlobalReadView() projection.GlobalReadView
	}
	writer            *state.LocalProductSetupWriter
	catalog           LocalProductSetupCatalog
	domainCatalog     teams.TeamDraftCatalogSnapshot
	identity          SetupIdentitySource
	now               func() time.Time
	nativeAuth        NativeAuthObserver
	credentials       CredentialStatusSource
	credentialMutator CredentialMutator

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
		journal:           config.Journal,
		projection:        config.Projection,
		writer:            config.Writer,
		catalog:           cloneSetupCatalog(config.Catalog),
		domainCatalog:     domainCatalog,
		identity:          config.Identity,
		now:               config.Now,
		nativeAuth:        config.NativeAuth,
		credentials:       config.Credentials,
		credentialMutator: config.CredentialMutator,
		sessions:          make(map[string]*localProductBuilderSession),
	}, nil
}

func (service *LocalProductSetupService) SetupSnapshot(
	ctx context.Context,
) (SetupSnapshot, error) {
	if service == nil || ctx == nil {
		return SetupSnapshot{}, ErrInvalidLocalProductSetup
	}
	if err := service.projection.Rebuild(ctx); err != nil {
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
	view := service.projection.GlobalReadView()
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
	return cloneSetupSnapshot(SetupSnapshot{
		SchemaVersion: localProductSetupSchemaVersion,
		ViewVersion:   view.Version(),
		Codex: ProviderSetupStatus{
			ProviderID: "codex",
			AuthMode:   "native_auth",
			Status:     auth.Status,
			Reason:     auth.Reason,
		},
		MiniMax:     miniMax,
		Runtimes:    setupRuntimePreviews(service.catalog),
		SavedTeams:  saved,
		Templates:   append([]SetupTeamTemplate{}, service.catalog.Templates...),
		RoleOptions: append([]SetupRoleOption{}, service.catalog.RoleOptions...),
		Skills:      append([]SetupSkillRevision{}, service.catalog.SkillRevisions...),
		Permissions: append([]string{}, service.catalog.Permissions...),
		Resources:   append([]SetupResourcePointer{}, service.catalog.Resources...),
	}), nil
}

func (service *LocalProductSetupService) StartBuilder(
	ctx context.Context,
	command BuilderStartCommand,
) (BuilderSessionView, error) {
	if service == nil || ctx == nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	if len(service.catalog.RoleOptions) < 2 {
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
			CatalogDigest: service.catalog.CatalogDigest,
			ViewVersion:   service.projection.GlobalReadView().Version(),
			Question:      emptyBuilderQuestion(),
			Preview:       emptyBuilderPreview(),
		},
	}
	switch command.Source {
	case BuilderSourceBlank:
		session.mainRoleID, session.subRoleIDs, err =
			service.defaultSetupRoles()
		if err != nil {
			return BuilderSessionView{}, err
		}
		session.view.Question = service.builderQuestion("team_name")
		if err := service.initializeStructuredSession(session); err != nil {
			return BuilderSessionView{}, err
		}
	case BuilderSourceTemplate:
		template, ok := service.setupTemplate(
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
		if err := service.initializeStructuredSession(session); err != nil {
			return BuilderSessionView{}, err
		}
		if err := service.makeSessionProposed(session); err != nil {
			return BuilderSessionView{}, err
		}
	case BuilderSourceSavedTeam:
		if err := service.loadSavedTeamSession(session, command); err != nil {
			return BuilderSessionView{}, err
		}
		if err := service.initializeStructuredSession(session); err != nil {
			return BuilderSessionView{}, err
		}
		if err := service.makeSessionProposed(session); err != nil {
			return BuilderSessionView{}, err
		}
	default:
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.refreshBuilderView(session)
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
	if err := service.projection.Rebuild(ctx); err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	if session.view.Question.ID == "" ||
		command.QuestionID != session.view.Question.ID ||
		!validSetupText(command.Answer, 2048) {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	nextQuestionID, err := service.applyBuilderAnswer(
		session,
		command.QuestionID,
		command.Answer,
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	content, err := service.buildSessionContent(session)
	if err != nil {
		return BuilderSessionView{}, err
	}
	var nextQuestion *teams.DraftQuestion
	if nextQuestionID != "" {
		question := service.domainQuestion(nextQuestionID)
		nextQuestion = &question
	}
	structured, err := teams.AnswerStructuredTeamDraft(
		session.structured,
		session.structured.Revision(),
		service.domainCatalog,
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
	session.view.Question = service.builderQuestion(nextQuestionID)
	service.refreshBuilderView(session)
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
	if err := service.projection.Rebuild(ctx); err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	if !validSetupText(command.Value, 2048) {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	switch command.Field {
	case "team_name":
		session.name = command.Value
	case "purpose":
		session.purpose = command.Value
	case "main_role":
		if !service.validRoleChoice(command.Value, "main") {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		session.mainRoleID = command.Value
	case "subagent_role":
		if !service.validRoleChoice(command.Value, "subagent") {
			return BuilderSessionView{}, ErrInvalidLocalProductSetup
		}
		session.subRoleIDs = []string{command.Value}
	default:
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	content, err := service.buildSessionContent(session)
	if err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	structured, err := teams.EditStructuredTeamDraft(
		session.structured,
		session.structured.Revision(),
		service.domainCatalog,
		content,
		nil,
	)
	if err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	session.structured = structured
	session.view.Revision++
	session.view.Question = emptyBuilderQuestion()
	service.refreshBuilderView(session)
	return cloneBuilderSessionView(session.view), nil
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
	if err := service.projection.Rebuild(ctx); err != nil {
		return BuilderSessionView{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
	)
	if err != nil {
		return BuilderSessionView{}, err
	}
	if session.view.Question.ID != "" {
		return cloneBuilderSessionView(session.view), nil
	}
	if _, err := teams.CheckStructuredTeamDraftAcceptable(
		session.structured,
		session.structured.Revision(),
		service.domainCatalog,
	); err != nil {
		return BuilderSessionView{}, ErrBuilderIncompatible
	}
	service.refreshBuilderView(session)
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
	if err := service.projection.Rebuild(ctx); err != nil {
		return BuilderConfirmation{}, ErrInvalidLocalProductSetup
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	session, err := service.currentSession(
		command.DraftID,
		command.ExpectedRevision,
		command.CatalogDigest,
		command.ViewVersion,
	)
	if err != nil {
		return BuilderConfirmation{}, err
	}
	if !session.view.CanConfirm ||
		command.BindingDigest != session.view.BindingDigest ||
		!validSetupText(command.DefinitionID, 128) {
		return BuilderConfirmation{}, ErrBuilderConflict
	}
	if _, err := teams.CheckStructuredTeamDraftAcceptable(
		session.structured,
		session.structured.Revision(),
		service.domainCatalog,
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
	roles, configuration, err := service.definitionInputs(session)
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
		service.catalog.AgentDefinitions,
		service.catalog.RuntimeProfiles,
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
			Definitions:     service.catalog.AgentDefinitions,
			RuntimeProfiles: service.catalog.RuntimeProfiles,
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
	if service == nil ||
		ctx == nil ||
		command.ProviderID != "minimax" ||
		command.CredentialReference != "" ||
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
			ProviderID:          "minimax",
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
	if service == nil ||
		ctx == nil ||
		command.ProviderID != "minimax" ||
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
	commandID, err := service.identity.NextSetupID(action + "-credential")
	if err != nil {
		return CredentialSetupResult{}, ErrCredentialSetupUnavailable
	}
	brokerCommand := credentials.CredentialCommand{
		CommandID:           commandID,
		ProviderID:          "minimax",
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
		if _, err := loomruntime.ValidateBinding(
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
		session.view.BindingDigest = session.structured.BindingDigest()
	}
	if session.view.Question.Options == nil {
		session.view.Question.Options = []BuilderQuestionOption{}
	}
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
			if _, err := loomruntime.ValidateBinding(
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
		if profileOK {
			authMode = string(profile.AuthMode)
			modelID = profile.ModelID
			runtimePreview.ModelID = modelID
		}
		role := BuilderRolePreview{
			Kind:                option.Kind,
			AgentDefinitionID:   option.AgentDefinitionID,
			Responsibility:      option.Responsibility,
			Runtime:             runtimePreview,
			RuntimeProfileID:    option.RuntimeProfileID,
			ModelID:             modelID,
			AuthMode:            authMode,
			Skills:              skills,
			PermissionIDs:       append([]string{}, option.PermissionIDs...),
			ResourceIDs:         append([]string{}, option.ResourceIDs...),
			Compatible:          compatible,
			CompatibilityReason: reason,
		}
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
		if _, err := loomruntime.ValidateBinding(
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
	}
	return roles, configuration, nil
}

func (service *LocalProductSetupService) currentSession(
	draftID string,
	expectedRevision int,
	catalogDigest string,
	viewVersion string,
) (*localProductBuilderSession, error) {
	session, ok := service.sessions[draftID]
	if !ok {
		return nil, ErrBuilderNotFound
	}
	if expectedRevision != session.view.Revision ||
		catalogDigest != session.view.CatalogDigest ||
		viewVersion != session.view.ViewVersion ||
		viewVersion != service.projection.GlobalReadView().Version() {
		return nil, ErrBuilderConflict
	}
	return session, nil
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
		optionID := service.findRoleOption(binding)
		if optionID == "" {
			return ErrBuilderIncompatible
		}
		if binding.Kind == "main" {
			session.mainRoleID = optionID
		} else {
			session.subRoleIDs = append(session.subRoleIDs, optionID)
		}
	}
	if session.mainRoleID == "" || len(session.subRoleIDs) == 0 {
		return ErrBuilderIncompatible
	}
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

func cloneSetupSnapshot(input SetupSnapshot) SetupSnapshot {
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
	input.RoleOptions = append([]SetupRoleOption{}, input.RoleOptions...)
	for index := range input.RoleOptions {
		input.RoleOptions[index] = cloneSetupRoleOption(input.RoleOptions[index])
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

func clearSetupBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
