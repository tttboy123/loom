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

type TeamConfigurationRoleBinding struct {
	Kind              string                           `json:"kind"`
	AgentDefinitionID string                           `json:"agent_definition_id"`
	RuntimeProfileID  string                           `json:"runtime_profile_id"`
	RuntimeInstanceID string                           `json:"runtime_instance_id"`
	ModelID           string                           `json:"model_id"`
	SkillRevisions    []TeamConfigurationSkillRevision `json:"skill_revisions"`
	PermissionIDs     []string                         `json:"permission_ids"`
	ResourceIDs       []string                         `json:"resource_ids"`
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
		command.ProviderID != "minimax" ||
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
	payload := credentialMetadataPayload{
		ProviderID:          command.ProviderID,
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
			SkillRevisions:    skills,
			PermissionIDs:     permissions,
			ResourceIDs:       resources,
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
