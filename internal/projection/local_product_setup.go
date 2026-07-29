package projection

import (
	"fmt"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type TeamDefinitionRoleRecord struct {
	Kind              string
	AgentDefinitionID string
	RuntimeProfileID  string
	Responsibility    string
}

type TeamConfigurationSkillRevision struct {
	ID       string
	Revision int
	Digest   string
}

type TeamConfigurationRoleBinding struct {
	Kind              string
	AgentDefinitionID string
	RuntimeProfileID  string
	RuntimeInstanceID string
	ModelID           string
	SkillRevisions    []TeamConfigurationSkillRevision
	PermissionIDs     []string
	ResourceIDs       []string
}

type TeamConfigurationRecord struct {
	RequestedConcurrency int
	MaximumBudgetCredits int
	RoleBindings         []TeamConfigurationRoleBinding
}

type TeamDefinitionRecord struct {
	ID               string
	Version          int
	Scope            string
	ScopeIdentity    ScopeIdentity
	Name             string
	Status           string
	Roles            []TeamDefinitionRoleRecord
	DefinitionDigest string
	DraftID          string
	DraftRevision    int
	CatalogDigest    string
	ContentDigest    string
	BindingDigest    string
	Configuration    TeamConfigurationRecord
}

type ProviderCredentialRecord struct {
	ProviderID          string
	CredentialReference string
	AuthMode            string
	Revision            int64
	Status              string
	Reason              string
	OccurredAt          time.Time
}

type localProductSetupSnapshotFields struct {
	TeamDefinitions     map[string]TeamDefinitionRecord
	ProviderCredentials map[string]ProviderCredentialRecord
}

type localSetupScopeIdentity struct {
	ProjectID    string `json:"project_id"`
	GenerationID string `json:"generation_id"`
}

type localSetupTeamRole struct {
	Kind              string `json:"kind"`
	AgentDefinitionID string `json:"agent_definition_id"`
	RuntimeProfileID  string `json:"runtime_profile_id"`
	Responsibility    string `json:"responsibility"`
}

type localSetupTeamDefinition struct {
	ID            string                  `json:"id"`
	Version       int                     `json:"version"`
	Scope         string                  `json:"scope"`
	ScopeIdentity localSetupScopeIdentity `json:"scope_identity"`
	Name          string                  `json:"name"`
	Status        string                  `json:"status"`
	Roles         []localSetupTeamRole    `json:"roles"`
	Digest        string                  `json:"digest"`
}

type localSetupSkillRevision struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
	Digest   string `json:"digest"`
}

type localSetupRoleBinding struct {
	Kind              string                    `json:"kind"`
	AgentDefinitionID string                    `json:"agent_definition_id"`
	RuntimeProfileID  string                    `json:"runtime_profile_id"`
	RuntimeInstanceID string                    `json:"runtime_instance_id"`
	ModelID           string                    `json:"model_id"`
	SkillRevisions    []localSetupSkillRevision `json:"skill_revisions"`
	PermissionIDs     []string                  `json:"permission_ids"`
	ResourceIDs       []string                  `json:"resource_ids"`
}

type localSetupConfiguration struct {
	RequestedConcurrency int                     `json:"requested_concurrency"`
	MaximumBudgetCredits int                     `json:"maximum_budget_credits"`
	RoleBindings         []localSetupRoleBinding `json:"role_bindings"`
}

type localSetupTeamSavedPayload struct {
	Definition    localSetupTeamDefinition `json:"definition"`
	DraftID       string                   `json:"draft_id"`
	DraftRevision int                      `json:"draft_revision"`
	CatalogDigest string                   `json:"catalog_digest"`
	ContentDigest string                   `json:"content_digest"`
	BindingDigest string                   `json:"binding_digest"`
	Configuration localSetupConfiguration  `json:"configuration"`
}

type localSetupTeamStatusPayload struct {
	DefinitionID     string `json:"definition_id"`
	DefinitionDigest string `json:"definition_digest"`
	Status           string `json:"status"`
}

type localSetupCredentialPayload struct {
	ProviderID          string `json:"provider_id"`
	CredentialReference string `json:"credential_reference"`
	AuthMode            string `json:"auth_mode"`
	Revision            int64  `json:"revision"`
	Status              string `json:"status"`
	Reason              string `json:"reason"`
	OccurredAtUnixMilli int64  `json:"occurred_at_unix_milli"`
}

func applyLocalProductSetupProjection(
	snapshot Snapshot,
	event journal.Event,
) error {
	ensureLocalProductSetupSnapshot(&snapshot)
	switch event.Type {
	case "TeamDefinitionSaved":
		return applyTeamDefinitionSaved(snapshot, event)
	case "TeamDefinitionArchived", "TeamDefinitionRestored":
		return applyTeamDefinitionStatus(snapshot, event)
	case "ProviderCredentialConfigured",
		"ProviderCredentialVerified",
		"ProviderCredentialRevoked":
		return applyProviderCredential(snapshot, event)
	default:
		return nil
	}
}

func isLocalProductSetupProjectionEvent(event journal.Event) bool {
	switch event.Type {
	case "TeamDefinitionSaved",
		"TeamDefinitionArchived",
		"TeamDefinitionRestored",
		"ProviderCredentialConfigured",
		"ProviderCredentialVerified",
		"ProviderCredentialRevoked":
		return true
	default:
		return false
	}
}

func ensureLocalProductSetupSnapshot(snapshot *Snapshot) {
	if snapshot.TeamDefinitions == nil {
		snapshot.TeamDefinitions = make(map[string]TeamDefinitionRecord)
	}
	if snapshot.ProviderCredentials == nil {
		snapshot.ProviderCredentials = make(
			map[string]ProviderCredentialRecord,
		)
	}
}

func cloneLocalProductSetupSnapshot(source Snapshot, target *Snapshot) {
	if source.TeamDefinitions != nil {
		target.TeamDefinitions = make(
			map[string]TeamDefinitionRecord,
			len(source.TeamDefinitions),
		)
		for id, definition := range source.TeamDefinitions {
			target.TeamDefinitions[id] = cloneTeamDefinitionRecord(definition)
		}
	}
	if source.ProviderCredentials != nil {
		target.ProviderCredentials = make(
			map[string]ProviderCredentialRecord,
			len(source.ProviderCredentials),
		)
		for id, credential := range source.ProviderCredentials {
			target.ProviderCredentials[id] = credential
		}
	}
}

func applyTeamDefinitionSaved(snapshot Snapshot, event journal.Event) error {
	var payload localSetupTeamSavedPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return err
	}
	if event.Seq != 1 ||
		event.StreamID != "team-definition/"+payload.Definition.ID ||
		!validLocalSetupIdentifier(payload.Definition.ID, 128) ||
		payload.Definition.Version <= 0 ||
		(payload.Definition.Scope != "reusable" &&
			payload.Definition.Scope != "project") ||
		!validLocalSetupIdentifier(payload.Definition.Name, 256) ||
		payload.Definition.Status != "active" ||
		len(payload.Definition.Roles) == 0 ||
		payload.Definition.Roles == nil ||
		!validSHA256Digest(payload.Definition.Digest) ||
		!validLocalSetupIdentifier(payload.DraftID, 128) ||
		payload.DraftRevision <= 0 ||
		!validSHA256Digest(payload.CatalogDigest) ||
		!validSHA256Digest(payload.ContentDigest) ||
		!validSHA256Digest(payload.BindingDigest) ||
		payload.Configuration.RequestedConcurrency <= 0 ||
		payload.Configuration.RequestedConcurrency > 3 ||
		payload.Configuration.MaximumBudgetCredits < 0 ||
		payload.Configuration.RoleBindings == nil ||
		len(payload.Configuration.RoleBindings) != len(payload.Definition.Roles) {
		return fmt.Errorf(
			"%w: invalid saved TeamDefinition",
			ErrInvalidProjectionEvent,
		)
	}
	if payload.Definition.Scope == "project" &&
		(!validLocalSetupIdentifier(
			payload.Definition.ScopeIdentity.ProjectID,
			128,
		) ||
			!validLocalSetupIdentifier(
				payload.Definition.ScopeIdentity.GenerationID,
				128,
			)) {
		return fmt.Errorf(
			"%w: invalid TeamDefinition scope identity",
			ErrInvalidProjectionEvent,
		)
	}
	if payload.Definition.Scope == "reusable" &&
		(payload.Definition.ScopeIdentity.ProjectID != "" ||
			payload.Definition.ScopeIdentity.GenerationID != "") {
		return fmt.Errorf(
			"%w: unexpected TeamDefinition scope identity",
			ErrInvalidProjectionEvent,
		)
	}
	if _, exists := snapshot.TeamDefinitions[payload.Definition.ID]; exists {
		return fmt.Errorf(
			"%w: duplicate TeamDefinition save",
			ErrInvalidProjectionEvent,
		)
	}

	roles := make([]TeamDefinitionRoleRecord, len(payload.Definition.Roles))
	roleKinds := make(map[string]string, len(payload.Definition.Roles))
	for index, role := range payload.Definition.Roles {
		if (role.Kind != "main" && role.Kind != "subagent") ||
			!validLocalSetupIdentifier(role.AgentDefinitionID, 128) ||
			!validLocalSetupIdentifier(role.RuntimeProfileID, 128) ||
			!validLocalSetupIdentifier(role.Responsibility, 2048) {
			return fmt.Errorf(
				"%w: invalid TeamDefinition role",
				ErrInvalidProjectionEvent,
			)
		}
		if _, duplicate := roleKinds[role.AgentDefinitionID]; duplicate {
			return fmt.Errorf(
				"%w: duplicate TeamDefinition role",
				ErrInvalidProjectionEvent,
			)
		}
		roleKinds[role.AgentDefinitionID] = role.Kind
		roles[index] = TeamDefinitionRoleRecord{
			Kind:              role.Kind,
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfileID:  role.RuntimeProfileID,
			Responsibility:    role.Responsibility,
		}
	}

	bindings := make(
		[]TeamConfigurationRoleBinding,
		len(payload.Configuration.RoleBindings),
	)
	seenBindings := make(map[string]struct{}, len(bindings))
	for index, binding := range payload.Configuration.RoleBindings {
		expectedKind, exists := roleKinds[binding.AgentDefinitionID]
		if !exists ||
			binding.Kind != expectedKind ||
			!validLocalSetupIdentifier(binding.RuntimeProfileID, 128) ||
			!validLocalSetupIdentifier(binding.RuntimeInstanceID, 128) ||
			!validLocalSetupIdentifier(binding.ModelID, 256) ||
			binding.SkillRevisions == nil ||
			binding.PermissionIDs == nil ||
			binding.ResourceIDs == nil {
			return fmt.Errorf(
				"%w: invalid TeamDefinition binding",
				ErrInvalidProjectionEvent,
			)
		}
		if _, duplicate := seenBindings[binding.AgentDefinitionID]; duplicate {
			return fmt.Errorf(
				"%w: duplicate TeamDefinition binding",
				ErrInvalidProjectionEvent,
			)
		}
		seenBindings[binding.AgentDefinitionID] = struct{}{}
		skills := make(
			[]TeamConfigurationSkillRevision,
			len(binding.SkillRevisions),
		)
		seenSkills := make(map[string]struct{}, len(skills))
		for skillIndex, skill := range binding.SkillRevisions {
			key := fmt.Sprintf("%s@%d", skill.ID, skill.Revision)
			if !validLocalSetupIdentifier(skill.ID, 128) ||
				skill.Revision <= 0 ||
				!validSHA256Digest(skill.Digest) {
				return fmt.Errorf(
					"%w: invalid Skill revision",
					ErrInvalidProjectionEvent,
				)
			}
			if _, duplicate := seenSkills[key]; duplicate {
				return fmt.Errorf(
					"%w: duplicate Skill revision",
					ErrInvalidProjectionEvent,
				)
			}
			seenSkills[key] = struct{}{}
			skills[skillIndex] = TeamConfigurationSkillRevision{
				ID:       skill.ID,
				Revision: skill.Revision,
				Digest:   skill.Digest,
			}
		}
		if !validLocalSetupIDs(binding.PermissionIDs) ||
			!validLocalSetupIDs(binding.ResourceIDs) {
			return fmt.Errorf(
				"%w: invalid binding references",
				ErrInvalidProjectionEvent,
			)
		}
		bindings[index] = TeamConfigurationRoleBinding{
			Kind:              binding.Kind,
			AgentDefinitionID: binding.AgentDefinitionID,
			RuntimeProfileID:  binding.RuntimeProfileID,
			RuntimeInstanceID: binding.RuntimeInstanceID,
			ModelID:           binding.ModelID,
			SkillRevisions:    skills,
			PermissionIDs: append(
				[]string{},
				binding.PermissionIDs...,
			),
			ResourceIDs: append(
				[]string{},
				binding.ResourceIDs...,
			),
		}
	}

	snapshot.TeamDefinitions[payload.Definition.ID] = TeamDefinitionRecord{
		ID:      payload.Definition.ID,
		Version: payload.Definition.Version,
		Scope:   payload.Definition.Scope,
		ScopeIdentity: ScopeIdentity{
			ProjectID:    payload.Definition.ScopeIdentity.ProjectID,
			GenerationID: payload.Definition.ScopeIdentity.GenerationID,
		},
		Name:             payload.Definition.Name,
		Status:           payload.Definition.Status,
		Roles:            roles,
		DefinitionDigest: payload.Definition.Digest,
		DraftID:          payload.DraftID,
		DraftRevision:    payload.DraftRevision,
		CatalogDigest:    payload.CatalogDigest,
		ContentDigest:    payload.ContentDigest,
		BindingDigest:    payload.BindingDigest,
		Configuration: TeamConfigurationRecord{
			RequestedConcurrency: payload.Configuration.RequestedConcurrency,
			MaximumBudgetCredits: payload.Configuration.MaximumBudgetCredits,
			RoleBindings:         bindings,
		},
	}
	return nil
}

func applyTeamDefinitionStatus(snapshot Snapshot, event journal.Event) error {
	var payload localSetupTeamStatusPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return err
	}
	expectedStatus := "archived"
	if event.Type == "TeamDefinitionRestored" {
		expectedStatus = "active"
	}
	record, exists := snapshot.TeamDefinitions[payload.DefinitionID]
	if !exists ||
		event.StreamID != "team-definition/"+payload.DefinitionID ||
		event.Seq <= 1 ||
		payload.Status != expectedStatus ||
		payload.DefinitionDigest != record.DefinitionDigest ||
		(record.Status == "active") == (expectedStatus == "active") {
		return fmt.Errorf(
			"%w: invalid TeamDefinition status transition",
			ErrInvalidProjectionEvent,
		)
	}
	record.Status = expectedStatus
	snapshot.TeamDefinitions[record.ID] = record
	return nil
}

func applyProviderCredential(snapshot Snapshot, event journal.Event) error {
	var payload localSetupCredentialPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return err
	}
	if payload.ProviderID != "minimax" ||
		event.StreamID != "provider-credential/minimax" ||
		payload.AuthMode != "brokered" ||
		!validLocalSetupCredentialReference(payload.CredentialReference) ||
		payload.Revision != event.Seq ||
		payload.Revision <= 0 ||
		payload.OccurredAtUnixMilli <= 0 {
		return fmt.Errorf(
			"%w: invalid Provider credential metadata",
			ErrInvalidProjectionEvent,
		)
	}
	if !validLocalSetupCredentialStatus(
		event.Type,
		payload.Status,
		payload.Reason,
	) {
		return fmt.Errorf(
			"%w: invalid Provider credential status",
			ErrInvalidProjectionEvent,
		)
	}
	previous, exists := snapshot.ProviderCredentials[payload.ProviderID]
	if !exists {
		if event.Type != "ProviderCredentialConfigured" ||
			payload.Revision != 1 {
			return fmt.Errorf(
				"%w: Provider credential status before configure",
				ErrInvalidProjectionEvent,
			)
		}
	} else if payload.Revision != previous.Revision+1 {
		return fmt.Errorf(
			"%w: Provider credential revision gap",
			ErrInvalidProjectionEvent,
		)
	}
	snapshot.ProviderCredentials[payload.ProviderID] =
		ProviderCredentialRecord{
			ProviderID:          payload.ProviderID,
			CredentialReference: payload.CredentialReference,
			AuthMode:            payload.AuthMode,
			Revision:            payload.Revision,
			Status:              payload.Status,
			Reason:              payload.Reason,
			OccurredAt: time.UnixMilli(
				payload.OccurredAtUnixMilli,
			).UTC(),
		}
	return nil
}

func validLocalSetupCredentialReference(value string) bool {
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

func validLocalSetupCredentialStatus(
	eventType,
	status,
	reason string,
) bool {
	switch eventType {
	case "ProviderCredentialConfigured":
		return status == "configured" && reason == ""
	case "ProviderCredentialVerified":
		switch status {
		case "verified":
			return reason == ""
		case "rejected":
			return reason == "provider_rejected" ||
				reason == "unavailable" ||
				reason == "timeout"
		default:
			return false
		}
	case "ProviderCredentialRevoked":
		return status == "revoked" && reason == ""
	default:
		return false
	}
}

func validLocalSetupIDs(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validLocalSetupIdentifier(value, 256) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func validLocalSetupIdentifier(value string, maximum int) bool {
	return value != "" &&
		len(value) <= maximum &&
		strings.TrimSpace(value) == value
}

func cloneTeamDefinitionRecord(record TeamDefinitionRecord) TeamDefinitionRecord {
	record.Roles = append([]TeamDefinitionRoleRecord{}, record.Roles...)
	record.Configuration.RoleBindings = append(
		[]TeamConfigurationRoleBinding{},
		record.Configuration.RoleBindings...,
	)
	for index := range record.Configuration.RoleBindings {
		binding := &record.Configuration.RoleBindings[index]
		binding.SkillRevisions = append(
			[]TeamConfigurationSkillRevision{},
			binding.SkillRevisions...,
		)
		binding.PermissionIDs = append(
			[]string{},
			binding.PermissionIDs...,
		)
		binding.ResourceIDs = append(
			[]string{},
			binding.ResourceIDs...,
		)
	}
	return record
}
