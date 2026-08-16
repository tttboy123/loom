package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/teams"
)

var (
	ErrInvalidLocalProductSetupWrite = errors.New("invalid local product setup write")
	ErrTeamDefinitionSetupNotFound   = errors.New("team definition setup not found")
)

type TeamConfigurationSkillRevision struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
}

type TeamConfigurationExecutionProfile struct {
	Version                    int                  `json:"version"`
	ID                         string               `json:"id"`
	HarnessAdapter             string               `json:"harness_adapter"`
	ProviderID                 string               `json:"provider_id"`
	ProviderAccountID          string               `json:"provider_account_id"`
	ModelID                    string               `json:"model_id"`
	AuthMode                   loomruntime.AuthMode `json:"auth_mode"`
	EndpointFingerprint        string               `json:"endpoint_fingerprint"`
	CredentialReference        string               `json:"credential_reference"`
	CredentialRevision         int64                `json:"credential_revision"`
	ReasoningEffort            string               `json:"reasoning_effort"`
	TimeoutNanoseconds         int64                `json:"timeout_nanoseconds"`
	Budget                     *int64               `json:"budget"`
	RequiredCapabilities       []string             `json:"required_capabilities"`
	RemoteToolEnrollmentID     string               `json:"remote_tool_enrollment_id,omitempty"`
	RemoteToolEnrollmentDigest string               `json:"remote_tool_enrollment_digest,omitempty"`
}

type TeamConfigurationFallbackRoute struct {
	Version           int                                `json:"version"`
	RuntimeProfileID  string                             `json:"runtime_profile_id"`
	RuntimeInstanceID string                             `json:"runtime_instance_id"`
	ModelID           string                             `json:"model_id"`
	ExecutionProfile  *TeamConfigurationExecutionProfile `json:"execution_profile"`
	ApprovalRequired  bool                               `json:"approval_required"`
}

type TeamConfigurationExecutionRoute struct {
	Version           int                                `json:"version"`
	RuntimeProfileID  string                             `json:"runtime_profile_id"`
	RuntimeInstanceID string                             `json:"runtime_instance_id"`
	ModelID           string                             `json:"model_id"`
	ExecutionProfile  *TeamConfigurationExecutionProfile `json:"execution_profile"`
}

type TeamConfigurationParallelRouteSet struct {
	Version          int                               `json:"version"`
	AdditionalRoutes []TeamConfigurationExecutionRoute `json:"additional_routes"`
	SynthesisRoute   TeamConfigurationExecutionRoute   `json:"synthesis_route"`
}

type TeamConfigurationRoleBinding struct {
	Kind              string                             `json:"kind"`
	AgentDefinitionID string                             `json:"agent_definition_id"`
	RuntimeProfileID  string                             `json:"runtime_profile_id"`
	RuntimeInstanceID string                             `json:"runtime_instance_id"`
	ModelID           string                             `json:"model_id"`
	ExecutionProfile  *TeamConfigurationExecutionProfile `json:"execution_profile,omitempty"`
	FallbackRoute     *TeamConfigurationFallbackRoute    `json:"fallback_route,omitempty"`
	ParallelRouteSet  *TeamConfigurationParallelRouteSet `json:"parallel_route_set,omitempty"`
	SkillRevisions    []TeamConfigurationSkillRevision   `json:"skill_revisions"`
	PermissionIDs     []string                           `json:"permission_ids"`
	ResourceIDs       []string                           `json:"resource_ids"`
}

type TeamConfigurationSnapshot struct {
	RequestedConcurrency int                            `json:"requested_concurrency"`
	MaximumBudgetCredits int                            `json:"maximum_budget_credits"`
	RoleBindings         []TeamConfigurationRoleBinding `json:"role_bindings"`
}

type TeamDefinitionSaveCommand struct {
	CommandID       string
	ExpectedHead    int64
	OccurredAt      time.Time
	Definition      teams.TeamDefinition
	Definitions     []agents.AgentDefinition
	RuntimeProfiles []loomruntime.RuntimeProfile
	DraftID         string
	DraftRevision   int
	CatalogDigest   string
	ContentDigest   string
	BindingDigest   string
	Configuration   TeamConfigurationSnapshot
}

type TeamDefinitionStatusCommand struct {
	CommandID    string
	DefinitionID string
	ExpectedHead int64
	OccurredAt   time.Time
	Status       string
}

type LocalProductSetupCommitResult struct {
	StreamID  string
	Sequence  int64
	EventID   string
	EventType string
}

type LocalProductSetupWriter struct {
	store *journal.Store
}

func NewLocalProductSetupWriter(
	store *journal.Store,
) (*LocalProductSetupWriter, error) {
	if store == nil {
		return nil, ErrInvalidLocalProductSetupWrite
	}
	return &LocalProductSetupWriter{store: store}, nil
}

func (writer *LocalProductSetupWriter) SaveTeamDefinition(
	ctx context.Context,
	command TeamDefinitionSaveCommand,
) (LocalProductSetupCommitResult, error) {
	if writer == nil || writer.store == nil || ctx == nil {
		return LocalProductSetupCommitResult{}, ErrInvalidLocalProductSetupWrite
	}
	validated, err := teams.ValidateTeamDefinition(
		command.Definition,
		command.Definitions,
		command.RuntimeProfiles,
	)
	if err != nil ||
		!validated.Valid ||
		command.Definition.Status() != teams.TeamDefinitionActive ||
		!validSetupIdentifier(command.CommandID, 128) ||
		command.ExpectedHead != 0 ||
		command.Definition.Version() != 1 ||
		!validSetupTime(command.OccurredAt) ||
		!validSetupIdentifier(command.DraftID, 128) ||
		command.DraftRevision <= 0 ||
		!validSetupDigest(command.CatalogDigest) ||
		!validSetupDigest(command.ContentDigest) ||
		!validSetupDigest(command.BindingDigest) {
		return LocalProductSetupCommitResult{}, ErrInvalidLocalProductSetupWrite
	}
	configuration, err := normalizeTeamConfiguration(
		command.Configuration,
		command.Definition,
		command.RuntimeProfiles,
	)
	if err != nil {
		return LocalProductSetupCommitResult{}, err
	}
	streamID := "team-definition/" + command.Definition.ID()
	payload := teamDefinitionSavedPayload{
		Definition: teamDefinitionFact{
			ID:      command.Definition.ID(),
			Version: command.Definition.Version(),
			Scope:   string(command.Definition.Scope()),
			ScopeIdentity: setupScopeIdentity{
				ProjectID:    command.Definition.ScopeIdentity().ProjectID,
				GenerationID: command.Definition.ScopeIdentity().GenerationID,
			},
			Name:   command.Definition.Name(),
			Status: string(command.Definition.Status()),
			Roles:  teamDefinitionRoleFacts(command.Definition.Roles()),
			Digest: command.Definition.Digest(),
		},
		DraftID:       command.DraftID,
		DraftRevision: command.DraftRevision,
		CatalogDigest: command.CatalogDigest,
		ContentDigest: command.ContentDigest,
		BindingDigest: command.BindingDigest,
		Configuration: configuration,
	}
	event, err := setupEvent(
		command.CommandID,
		streamID,
		command.ExpectedHead+1,
		"TeamDefinitionSaved",
		command.OccurredAt,
		command.DraftID,
		"",
		payload,
	)
	if err != nil {
		return LocalProductSetupCommitResult{}, err
	}
	committed, err := writer.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: streamID,
			Sequence: command.ExpectedHead,
		}},
		[]journal.Event{event},
	)
	if err != nil {
		return LocalProductSetupCommitResult{}, err
	}
	return setupCommitResult(committed[0]), nil
}

func (writer *LocalProductSetupWriter) SetTeamDefinitionStatus(
	ctx context.Context,
	command TeamDefinitionStatusCommand,
) (LocalProductSetupCommitResult, error) {
	if writer == nil || writer.store == nil || ctx == nil ||
		!validSetupIdentifier(command.CommandID, 128) ||
		!validSetupIdentifier(command.DefinitionID, 128) ||
		command.ExpectedHead <= 0 ||
		!validSetupTime(command.OccurredAt) ||
		(command.Status != "active" && command.Status != "archived") {
		return LocalProductSetupCommitResult{}, ErrInvalidLocalProductSetupWrite
	}
	streamID := "team-definition/" + command.DefinitionID
	events, err := writer.store.ReadStream(ctx, streamID)
	if err != nil {
		return LocalProductSetupCommitResult{}, err
	}
	if len(events) == 0 {
		return LocalProductSetupCommitResult{}, ErrTeamDefinitionSetupNotFound
	}
	currentStatus, definitionDigest, err := replayTeamDefinitionStatus(events)
	if err != nil ||
		int64(len(events)) != command.ExpectedHead ||
		currentStatus == command.Status {
		return LocalProductSetupCommitResult{}, ErrInvalidLocalProductSetupWrite
	}
	eventType := "TeamDefinitionArchived"
	if command.Status == "active" {
		eventType = "TeamDefinitionRestored"
	}
	payload := teamDefinitionStatusPayload{
		DefinitionID:     command.DefinitionID,
		DefinitionDigest: definitionDigest,
		Status:           command.Status,
	}
	event, err := setupEvent(
		command.CommandID,
		streamID,
		command.ExpectedHead+1,
		eventType,
		command.OccurredAt,
		command.DefinitionID,
		events[len(events)-1].ID,
		payload,
	)
	if err != nil {
		return LocalProductSetupCommitResult{}, err
	}
	committed, err := writer.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: streamID,
			Sequence: command.ExpectedHead,
		}},
		[]journal.Event{event},
	)
	if err != nil {
		return LocalProductSetupCommitResult{}, err
	}
	return setupCommitResult(committed[0]), nil
}

func (writer *LocalProductSetupWriter) CommitCredentialMetadata(
	ctx context.Context,
	command credentials.MetadataCommand,
) (credentials.MetadataResult, error) {
	if writer == nil || writer.store == nil || ctx == nil ||
		!validSetupIdentifier(command.CommandID, 128) ||
		!credentials.ValidProviderIdentifier(command.ProviderID) ||
		command.ProviderAccountID != "" &&
			!credentials.ValidProviderAccountIdentifier(
				command.ProviderID, command.ProviderAccountID,
			) ||
		!validSetupCredentialReference(command.CredentialReference) ||
		command.ExpectedRevision < 0 ||
		!validSetupTime(command.OccurredAt) ||
		!validCredentialMetadataStatus(command.Status, command.Reason) {
		return credentials.MetadataResult{}, ErrInvalidLocalProductSetupWrite
	}
	eventType := "ProviderCredentialConfigured"
	switch command.Status {
	case credentials.CredentialVerified, credentials.CredentialRejected:
		eventType = "ProviderCredentialVerified"
	case credentials.CredentialRevoked:
		eventType = "ProviderCredentialRevoked"
	}
	streamID := "provider-credential/" + command.ProviderID
	providerAccountID := command.ProviderAccountID
	if providerAccountID != "" &&
		providerAccountID != command.ProviderID+".primary" {
		streamID = "provider-account-credential/" + providerAccountID
		switch command.Status {
		case credentials.CredentialConfigured:
			eventType = "ProviderAccountCredentialConfigured"
		case credentials.CredentialVerified, credentials.CredentialRejected:
			eventType = "ProviderAccountCredentialVerified"
		case credentials.CredentialRevoked:
			eventType = "ProviderAccountCredentialRevoked"
		}
	} else {
		providerAccountID = ""
	}
	payload := credentialMetadataPayload{
		ProviderID:          command.ProviderID,
		ProviderAccountID:   providerAccountID,
		CredentialReference: command.CredentialReference,
		AuthMode:            "brokered",
		Revision:            command.ExpectedRevision + 1,
		Status:              string(command.Status),
		Reason:              string(command.Reason),
		OccurredAtUnixMilli: command.OccurredAt.UnixMilli(),
	}
	event, err := setupEvent(
		command.CommandID,
		streamID,
		command.ExpectedRevision+1,
		eventType,
		command.OccurredAt,
		command.ProviderID,
		"",
		payload,
	)
	if err != nil {
		return credentials.MetadataResult{}, err
	}
	_, err = writer.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: streamID,
			Sequence: command.ExpectedRevision,
		}},
		[]journal.Event{event},
	)
	if err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) {
			return credentials.MetadataResult{},
				credentials.ErrCredentialMetadataConflict
		}
		return credentials.MetadataResult{}, err
	}
	return credentials.MetadataResult{
		ProviderID:          command.ProviderID,
		ProviderAccountID:   command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		Revision:            command.ExpectedRevision + 1,
		Status:              command.Status,
		Reason:              command.Reason,
	}, nil
}

type setupScopeIdentity struct {
	ProjectID    string `json:"project_id"`
	GenerationID string `json:"generation_id"`
}

type teamDefinitionRoleFact struct {
	Kind              string `json:"kind"`
	AgentDefinitionID string `json:"agent_definition_id"`
	RuntimeProfileID  string `json:"runtime_profile_id"`
	Responsibility    string `json:"responsibility"`
}

type teamDefinitionFact struct {
	ID            string                   `json:"id"`
	Version       int                      `json:"version"`
	Scope         string                   `json:"scope"`
	ScopeIdentity setupScopeIdentity       `json:"scope_identity"`
	Name          string                   `json:"name"`
	Status        string                   `json:"status"`
	Roles         []teamDefinitionRoleFact `json:"roles"`
	Digest        string                   `json:"digest"`
}

type teamDefinitionSavedPayload struct {
	Definition    teamDefinitionFact        `json:"definition"`
	DraftID       string                    `json:"draft_id"`
	DraftRevision int                       `json:"draft_revision"`
	CatalogDigest string                    `json:"catalog_digest"`
	ContentDigest string                    `json:"content_digest"`
	BindingDigest string                    `json:"binding_digest"`
	Configuration TeamConfigurationSnapshot `json:"configuration"`
}

type teamDefinitionStatusPayload struct {
	DefinitionID     string `json:"definition_id"`
	DefinitionDigest string `json:"definition_digest"`
	Status           string `json:"status"`
}

type credentialMetadataPayload struct {
	ProviderID          string `json:"provider_id"`
	ProviderAccountID   string `json:"provider_account_id,omitempty"`
	CredentialReference string `json:"credential_reference"`
	AuthMode            string `json:"auth_mode"`
	Revision            int64  `json:"revision"`
	Status              string `json:"status"`
	Reason              string `json:"reason"`
	OccurredAtUnixMilli int64  `json:"occurred_at_unix_milli"`
}

func setupEvent(
	commandID,
	streamID string,
	sequence int64,
	eventType string,
	occurredAt time.Time,
	correlationID,
	causationID string,
	payload any,
) (journal.Event, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return journal.Event{}, ErrInvalidLocalProductSetupWrite
	}
	sum := sha256.Sum256([]byte(
		commandID + "\x00" + streamID + "\x00" + eventType,
	))
	eventID := "setup-" + hex.EncodeToString(sum[:16])
	return journal.Event{
		ID:             eventID,
		StreamID:       streamID,
		Seq:            sequence,
		IdempotencyKey: "local-product-setup/" + commandID,
		Type:           eventType,
		SchemaVersion:  1,
		EmittedAt:      occurredAt,
		CorrelationID:  correlationID,
		CausationID:    causationID,
		PayloadJSON:    encoded,
	}, nil
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

func setupCommitResult(event journal.Event) LocalProductSetupCommitResult {
	return LocalProductSetupCommitResult{
		StreamID:  event.StreamID,
		Sequence:  event.Seq,
		EventID:   event.ID,
		EventType: event.Type,
	}
}

func normalizeTeamConfiguration(
	input TeamConfigurationSnapshot,
	definition teams.TeamDefinition,
	runtimeProfiles []loomruntime.RuntimeProfile,
) (TeamConfigurationSnapshot, error) {
	if input.RequestedConcurrency <= 0 ||
		input.RequestedConcurrency > 3 ||
		input.MaximumBudgetCredits < 0 ||
		len(input.RoleBindings) != len(definition.Roles()) {
		return TeamConfigurationSnapshot{}, ErrInvalidLocalProductSetupWrite
	}
	expected := make(map[string]teams.TeamDefinitionRole)
	for _, role := range definition.Roles() {
		expected[role.AgentDefinitionID] = role
	}
	output := TeamConfigurationSnapshot{
		RequestedConcurrency: input.RequestedConcurrency,
		MaximumBudgetCredits: input.MaximumBudgetCredits,
		RoleBindings: make(
			[]TeamConfigurationRoleBinding,
			len(input.RoleBindings),
		),
	}
	seen := make(map[string]struct{})
	for index, role := range input.RoleBindings {
		expectedRole, ok := expected[role.AgentDefinitionID]
		if !ok ||
			role.Kind != string(expectedRole.Kind) ||
			role.RuntimeProfileID != expectedRole.RuntimeProfileID ||
			!validSetupIdentifier(role.RuntimeInstanceID, 128) ||
			!validSetupIdentifier(role.ModelID, 256) {
			return TeamConfigurationSnapshot{}, ErrInvalidLocalProductSetupWrite
		}
		if _, duplicate := seen[role.AgentDefinitionID]; duplicate {
			return TeamConfigurationSnapshot{}, ErrInvalidLocalProductSetupWrite
		}
		seen[role.AgentDefinitionID] = struct{}{}
		skills, err := normalizeSetupSkills(role.SkillRevisions)
		if err != nil {
			return TeamConfigurationSnapshot{}, err
		}
		permissions, err := normalizeSetupIDs(role.PermissionIDs, 64)
		if err != nil {
			return TeamConfigurationSnapshot{}, err
		}
		resources, err := normalizeSetupIDs(role.ResourceIDs, 64)
		if err != nil {
			return TeamConfigurationSnapshot{}, err
		}
		output.RoleBindings[index] = TeamConfigurationRoleBinding{
			Kind:              role.Kind,
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfileID:  role.RuntimeProfileID,
			RuntimeInstanceID: role.RuntimeInstanceID,
			ModelID:           role.ModelID,
			ExecutionProfile:  nil,
			FallbackRoute:     nil,
			ParallelRouteSet:  nil,
			SkillRevisions:    skills,
			PermissionIDs:     permissions,
			ResourceIDs:       resources,
		}
		if role.ExecutionProfile != nil {
			profile, err := normalizeTeamConfigurationExecutionProfile(
				*role.ExecutionProfile,
				role.RuntimeProfileID,
				role.ModelID,
			)
			if err != nil {
				return TeamConfigurationSnapshot{}, err
			}
			expected, found := runtimeProfileByID(
				runtimeProfiles,
				role.RuntimeProfileID,
			)
			if !found || !teamExecutionProfileMatchesRuntimeProfile(
				profile,
				expected,
			) {
				return TeamConfigurationSnapshot{}, ErrInvalidLocalProductSetupWrite
			}
			output.RoleBindings[index].ExecutionProfile = &profile
		}
		if role.FallbackRoute != nil {
			if role.ParallelRouteSet != nil {
				return TeamConfigurationSnapshot{}, ErrInvalidLocalProductSetupWrite
			}
			route, err := normalizeTeamConfigurationFallbackRoute(
				*role.FallbackRoute,
				role.RuntimeProfileID,
				runtimeProfiles,
			)
			if err != nil {
				return TeamConfigurationSnapshot{}, err
			}
			output.RoleBindings[index].FallbackRoute = &route
		}
		if role.ParallelRouteSet != nil {
			if role.ExecutionProfile == nil {
				return TeamConfigurationSnapshot{}, ErrInvalidLocalProductSetupWrite
			}
			routeSet, err := normalizeTeamConfigurationParallelRouteSet(
				*role.ParallelRouteSet,
				role.RuntimeProfileID,
				runtimeProfiles,
			)
			if err != nil {
				return TeamConfigurationSnapshot{}, err
			}
			output.RoleBindings[index].ParallelRouteSet = &routeSet
		}
	}
	sort.Slice(output.RoleBindings, func(i, j int) bool {
		if output.RoleBindings[i].Kind != output.RoleBindings[j].Kind {
			return output.RoleBindings[i].Kind == "main"
		}
		return output.RoleBindings[i].AgentDefinitionID <
			output.RoleBindings[j].AgentDefinitionID
	})
	return output, nil
}

func normalizeTeamConfigurationParallelRouteSet(
	input TeamConfigurationParallelRouteSet,
	primaryProfileID string,
	runtimeProfiles []loomruntime.RuntimeProfile,
) (TeamConfigurationParallelRouteSet, error) {
	if input.Version != 1 || len(input.AdditionalRoutes) < 1 ||
		len(input.AdditionalRoutes) > 2 {
		return TeamConfigurationParallelRouteSet{}, ErrInvalidLocalProductSetupWrite
	}
	output := TeamConfigurationParallelRouteSet{
		Version: input.Version,
		AdditionalRoutes: make(
			[]TeamConfigurationExecutionRoute,
			len(input.AdditionalRoutes),
		),
	}
	seen := map[string]struct{}{primaryProfileID: {}}
	for index, raw := range input.AdditionalRoutes {
		route, err := normalizeTeamConfigurationExecutionRoute(raw, runtimeProfiles)
		if err != nil {
			return TeamConfigurationParallelRouteSet{}, err
		}
		if _, duplicate := seen[route.RuntimeProfileID]; duplicate {
			return TeamConfigurationParallelRouteSet{}, ErrInvalidLocalProductSetupWrite
		}
		seen[route.RuntimeProfileID] = struct{}{}
		output.AdditionalRoutes[index] = route
	}
	sort.Slice(output.AdditionalRoutes, func(i, j int) bool {
		return output.AdditionalRoutes[i].RuntimeProfileID <
			output.AdditionalRoutes[j].RuntimeProfileID
	})
	synthesis, err := normalizeTeamConfigurationExecutionRoute(
		input.SynthesisRoute,
		runtimeProfiles,
	)
	if err != nil {
		return TeamConfigurationParallelRouteSet{}, err
	}
	output.SynthesisRoute = synthesis
	return output, nil
}

func normalizeTeamConfigurationExecutionRoute(
	input TeamConfigurationExecutionRoute,
	runtimeProfiles []loomruntime.RuntimeProfile,
) (TeamConfigurationExecutionRoute, error) {
	if input.Version != 1 ||
		!validSetupIdentifier(input.RuntimeProfileID, 128) ||
		!validSetupIdentifier(input.RuntimeInstanceID, 128) ||
		!validSetupIdentifier(input.ModelID, 256) ||
		input.ExecutionProfile == nil {
		return TeamConfigurationExecutionRoute{}, ErrInvalidLocalProductSetupWrite
	}
	profile, err := normalizeTeamConfigurationExecutionProfile(
		*input.ExecutionProfile,
		input.RuntimeProfileID,
		input.ModelID,
	)
	if err != nil {
		return TeamConfigurationExecutionRoute{}, err
	}
	expected, found := runtimeProfileByID(runtimeProfiles, input.RuntimeProfileID)
	if !found || !teamExecutionProfileMatchesRuntimeProfile(profile, expected) {
		return TeamConfigurationExecutionRoute{}, ErrInvalidLocalProductSetupWrite
	}
	return TeamConfigurationExecutionRoute{
		Version: input.Version, RuntimeProfileID: input.RuntimeProfileID,
		RuntimeInstanceID: input.RuntimeInstanceID, ModelID: input.ModelID,
		ExecutionProfile: &profile,
	}, nil
}

func normalizeTeamConfigurationFallbackRoute(
	input TeamConfigurationFallbackRoute,
	primaryProfileID string,
	runtimeProfiles []loomruntime.RuntimeProfile,
) (TeamConfigurationFallbackRoute, error) {
	if input.Version != 1 || !input.ApprovalRequired ||
		input.RuntimeProfileID == primaryProfileID ||
		!validSetupIdentifier(input.RuntimeProfileID, 128) ||
		!validSetupIdentifier(input.RuntimeInstanceID, 128) ||
		!validSetupIdentifier(input.ModelID, 256) ||
		input.ExecutionProfile == nil {
		return TeamConfigurationFallbackRoute{}, ErrInvalidLocalProductSetupWrite
	}
	profile, err := normalizeTeamConfigurationExecutionProfile(
		*input.ExecutionProfile,
		input.RuntimeProfileID,
		input.ModelID,
	)
	if err != nil {
		return TeamConfigurationFallbackRoute{}, err
	}
	expected, found := runtimeProfileByID(runtimeProfiles, input.RuntimeProfileID)
	if !found || !teamExecutionProfileMatchesRuntimeProfile(profile, expected) {
		return TeamConfigurationFallbackRoute{}, ErrInvalidLocalProductSetupWrite
	}
	return TeamConfigurationFallbackRoute{
		Version: input.Version, RuntimeProfileID: input.RuntimeProfileID,
		RuntimeInstanceID: input.RuntimeInstanceID, ModelID: input.ModelID,
		ExecutionProfile: &profile, ApprovalRequired: true,
	}, nil
}

func runtimeProfileByID(
	profiles []loomruntime.RuntimeProfile,
	id string,
) (loomruntime.RuntimeProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			normalized, err := loomruntime.NewRuntimeProfile(profile)
			return normalized, err == nil
		}
	}
	return loomruntime.RuntimeProfile{}, false
}

func teamExecutionProfileMatchesRuntimeProfile(
	snapshot TeamConfigurationExecutionProfile,
	profile loomruntime.RuntimeProfile,
) bool {
	if snapshot.ID != profile.ID ||
		snapshot.HarnessAdapter != profile.AdapterType ||
		snapshot.ProviderID != profile.ProviderID ||
		snapshot.ProviderAccountID != profile.ProviderAccountID ||
		snapshot.ModelID != profile.ModelID ||
		snapshot.AuthMode != profile.AuthMode ||
		snapshot.EndpointFingerprint != profile.EndpointFingerprint ||
		snapshot.CredentialReference != profile.CredentialReference ||
		snapshot.CredentialRevision != profile.CredentialRevision ||
		snapshot.ReasoningEffort != profile.ReasoningEffort ||
		time.Duration(snapshot.TimeoutNanoseconds) != profile.Timeout ||
		snapshot.RemoteToolEnrollmentID != profile.RemoteToolEnrollmentID ||
		snapshot.RemoteToolEnrollmentDigest != profile.RemoteToolEnrollmentDigest ||
		len(snapshot.RequiredCapabilities) != len(profile.RequiredCapabilities) ||
		(snapshot.Budget == nil) != (profile.Budget == nil) {
		return false
	}
	for index := range snapshot.RequiredCapabilities {
		if snapshot.RequiredCapabilities[index] != profile.RequiredCapabilities[index] {
			return false
		}
	}
	return snapshot.Budget == nil || *snapshot.Budget == *profile.Budget
}

func normalizeTeamConfigurationExecutionProfile(
	input TeamConfigurationExecutionProfile,
	profileID string,
	modelID string,
) (TeamConfigurationExecutionProfile, error) {
	if input.Version != 1 || input.ID != profileID || input.ModelID != modelID {
		return TeamConfigurationExecutionProfile{}, ErrInvalidLocalProductSetupWrite
	}
	profile, err := loomruntime.ValidateExecutionProfile(loomruntime.RuntimeProfile{
		ID:                         input.ID,
		AdapterType:                input.HarnessAdapter,
		ProviderID:                 input.ProviderID,
		ProviderAccountID:          input.ProviderAccountID,
		ModelID:                    input.ModelID,
		AuthMode:                   input.AuthMode,
		EndpointFingerprint:        input.EndpointFingerprint,
		CredentialReference:        input.CredentialReference,
		CredentialRevision:         input.CredentialRevision,
		ReasoningEffort:            input.ReasoningEffort,
		RequiredCapabilities:       append([]string(nil), input.RequiredCapabilities...),
		Timeout:                    time.Duration(input.TimeoutNanoseconds),
		Budget:                     input.Budget,
		RemoteToolEnrollmentID:     input.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: input.RemoteToolEnrollmentDigest,
	})
	if err != nil {
		return TeamConfigurationExecutionProfile{}, ErrInvalidLocalProductSetupWrite
	}
	output := input
	output.RequiredCapabilities = append(
		[]string(nil), profile.RequiredCapabilities...,
	)
	output.RemoteToolEnrollmentID = profile.RemoteToolEnrollmentID
	output.RemoteToolEnrollmentDigest = profile.RemoteToolEnrollmentDigest
	if profile.Budget != nil {
		budget := *profile.Budget
		output.Budget = &budget
	}
	return output, nil
}

func normalizeSetupSkills(
	input []TeamConfigurationSkillRevision,
) ([]TeamConfigurationSkillRevision, error) {
	output := append([]TeamConfigurationSkillRevision(nil), input...)
	seen := make(map[string]struct{})
	for _, skill := range output {
		key := fmt.Sprintf("%s@%d", skill.ID, skill.Revision)
		if !validSetupIdentifier(skill.ID, 128) ||
			skill.Revision <= 0 ||
			!validSetupDigest(skill.Digest) {
			return nil, ErrInvalidLocalProductSetupWrite
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, ErrInvalidLocalProductSetupWrite
		}
		seen[key] = struct{}{}
	}
	sort.Slice(output, func(i, j int) bool {
		if output[i].ID != output[j].ID {
			return output[i].ID < output[j].ID
		}
		return output[i].Revision < output[j].Revision
	})
	if output == nil {
		output = []TeamConfigurationSkillRevision{}
	}
	return output, nil
}

func normalizeSetupIDs(input []string, maximum int) ([]string, error) {
	if len(input) > maximum {
		return nil, ErrInvalidLocalProductSetupWrite
	}
	output := append([]string(nil), input...)
	sort.Strings(output)
	for index, value := range output {
		if !validSetupIdentifier(value, 256) ||
			index > 0 && output[index-1] == value {
			return nil, ErrInvalidLocalProductSetupWrite
		}
	}
	if output == nil {
		output = []string{}
	}
	return output, nil
}

func replayTeamDefinitionStatus(
	events []journal.Event,
) (string, string, error) {
	var saved teamDefinitionSavedPayload
	if len(events) == 0 ||
		events[0].Type != "TeamDefinitionSaved" ||
		json.Unmarshal(events[0].PayloadJSON, &saved) != nil ||
		!validSetupDigest(saved.Definition.Digest) {
		return "", "", ErrInvalidLocalProductSetupWrite
	}
	status := saved.Definition.Status
	for _, event := range events[1:] {
		var payload teamDefinitionStatusPayload
		if json.Unmarshal(event.PayloadJSON, &payload) != nil ||
			payload.DefinitionID != saved.Definition.ID ||
			payload.DefinitionDigest != saved.Definition.Digest {
			return "", "", ErrInvalidLocalProductSetupWrite
		}
		switch event.Type {
		case "TeamDefinitionArchived":
			status = "archived"
		case "TeamDefinitionRestored":
			status = "active"
		default:
			return "", "", ErrInvalidLocalProductSetupWrite
		}
	}
	return status, saved.Definition.Digest, nil
}

func teamDefinitionRoleFacts(
	input []teams.TeamDefinitionRole,
) []teamDefinitionRoleFact {
	output := make([]teamDefinitionRoleFact, len(input))
	for index, role := range input {
		output[index] = teamDefinitionRoleFact{
			Kind:              string(role.Kind),
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfileID:  role.RuntimeProfileID,
			Responsibility:    role.Responsibility,
		}
	}
	return output
}

func validCredentialMetadataStatus(
	status credentials.CredentialStatus,
	reason credentials.VerificationReason,
) bool {
	switch status {
	case credentials.CredentialConfigured, credentials.CredentialRevoked:
		return reason == credentials.VerificationReasonNone
	case credentials.CredentialVerified:
		return reason == credentials.VerificationReasonNone
	case credentials.CredentialRejected:
		return reason == credentials.VerificationReasonProviderRejected ||
			reason == credentials.VerificationReasonUnavailable ||
			reason == credentials.VerificationReasonTimeout
	default:
		return false
	}
}

func validSetupTime(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC
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

func validSetupIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum ||
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
