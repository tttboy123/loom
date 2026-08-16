package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"loom-pi-rebuild/internal/agents"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidTeamDefinition            = errors.New("invalid team definition")
	ErrInvalidTeamDefinitionRole        = errors.New("invalid team definition role")
	ErrInvalidTeamDefinitionCatalog     = errors.New("invalid team definition catalog")
	ErrInventedTeamDefinitionReference  = errors.New("invented team definition reference")
	ErrDuplicateTeamDefinitionReference = errors.New("duplicate team definition reference")
	ErrDuplicateTeamDefinition          = errors.New("duplicate team definition")
	ErrTeamDefinitionNotFound           = errors.New("team definition not found")
	ErrTeamDefinitionDigestMismatch     = errors.New("team definition digest mismatch")
)

type TeamDefinitionScope string

const (
	TeamDefinitionScopeProject  TeamDefinitionScope = "project"
	TeamDefinitionScopeReusable TeamDefinitionScope = "reusable"
)

type TeamDefinitionStatus string

const (
	TeamDefinitionActive   TeamDefinitionStatus = "active"
	TeamDefinitionArchived TeamDefinitionStatus = "archived"
)

type TeamDefinitionRoleKind string

const (
	TeamDefinitionRoleMain     TeamDefinitionRoleKind = "main"
	TeamDefinitionRoleSubAgent TeamDefinitionRoleKind = "subagent"
)

type TeamDefinitionRole struct {
	Kind              TeamDefinitionRoleKind
	AgentDefinitionID string
	RuntimeProfileID  string
	Responsibility    string
}

type TeamDefinitionInput struct {
	ID            string
	Version       int
	Scope         TeamDefinitionScope
	ScopeIdentity agents.ScopeIdentity
	Name          string
	Status        TeamDefinitionStatus
	Roles         []TeamDefinitionRole
}

type TeamDefinition struct {
	id            string
	version       int
	scope         TeamDefinitionScope
	scopeIdentity agents.ScopeIdentity
	name          string
	status        TeamDefinitionStatus
	roles         []TeamDefinitionRole
	digest        string
}

type TeamDefinitionValidationCandidate struct {
	Valid                 bool
	ID                    string
	Version               int
	Scope                 TeamDefinitionScope
	ScopeIdentity         agents.ScopeIdentity
	MainAgentDefinitionID string
	SubAgentDefinitionIDs []string
	Roles                 []TeamDefinitionRole
	DefinitionDigest      string
}

type TeamDefinitionLoadCandidate struct {
	Found                 bool
	ID                    string
	Version               int
	Scope                 TeamDefinitionScope
	ScopeIdentity         agents.ScopeIdentity
	MainAgentDefinitionID string
	SubAgentDefinitionIDs []string
	Roles                 []TeamDefinitionRole
	DefinitionDigest      string
}

func BuildTeamDefinition(
	input TeamDefinitionInput,
	definitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
) (TeamDefinition, error) {
	if err := validateTeamDefinitionInput(input); err != nil {
		return TeamDefinition{}, err
	}
	if err := validateTeamDefinitionCatalogs(definitions, profiles); err != nil {
		return TeamDefinition{}, err
	}
	normalized, err := validateAndNormalizeTeamDefinitionRoles(input, definitions, profiles)
	if err != nil {
		return TeamDefinition{}, err
	}
	definition := TeamDefinition{
		id:            input.ID,
		version:       input.Version,
		scope:         input.Scope,
		scopeIdentity: input.ScopeIdentity,
		name:          input.Name,
		status:        input.Status,
		roles:         normalized,
	}
	digest, err := digestTeamDefinition(definition)
	if err != nil {
		return TeamDefinition{}, err
	}
	definition.digest = digest
	return definition, nil
}

func ValidateTeamDefinition(
	current TeamDefinition,
	definitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
) (TeamDefinitionValidationCandidate, error) {
	if current.digest == "" {
		return TeamDefinitionValidationCandidate{}, ErrInvalidTeamDefinition
	}
	digest, err := digestTeamDefinition(current)
	if err != nil {
		return TeamDefinitionValidationCandidate{}, err
	}
	if digest != current.digest {
		return TeamDefinitionValidationCandidate{}, ErrTeamDefinitionDigestMismatch
	}
	rebuilt, err := BuildTeamDefinition(TeamDefinitionInput{
		ID:            current.id,
		Version:       current.version,
		Scope:         current.scope,
		ScopeIdentity: current.scopeIdentity,
		Name:          current.name,
		Status:        current.status,
		Roles:         current.Roles(),
	}, definitions, profiles)
	if err != nil {
		return TeamDefinitionValidationCandidate{}, err
	}
	if rebuilt.digest != current.digest {
		return TeamDefinitionValidationCandidate{}, ErrTeamDefinitionDigestMismatch
	}
	return validationCandidateForTeamDefinition(rebuilt), nil
}

func ResolveTeamDefinition(
	definitions []TeamDefinition,
	id string,
	context agents.ScopeIdentity,
) (TeamDefinitionLoadCandidate, error) {
	if id == "" {
		return TeamDefinitionLoadCandidate{}, ErrTeamDefinitionNotFound
	}
	if context.GenerationID != "" {
		return TeamDefinitionLoadCandidate{}, ErrInvalidTeamDefinition
	}
	for _, definition := range definitions {
		if err := validateStoredTeamDefinition(definition); err != nil {
			return TeamDefinitionLoadCandidate{}, err
		}
	}

	for _, scope := range []TeamDefinitionScope{TeamDefinitionScopeProject, TeamDefinitionScopeReusable} {
		var selected TeamDefinition
		found := false
		winnerCount := 0
		for _, definition := range definitions {
			if definition.status == TeamDefinitionArchived ||
				definition.id != id ||
				!teamDefinitionEligibleInScope(definition, context, scope) {
				continue
			}
			if !found || definition.version > selected.version {
				selected = definition
				found = true
				winnerCount = 1
			} else if definition.version == selected.version {
				winnerCount++
			}
		}
		if found {
			if winnerCount > 1 {
				return TeamDefinitionLoadCandidate{}, fmt.Errorf("%w: %s", ErrDuplicateTeamDefinition, id)
			}
			return loadCandidateForTeamDefinition(selected), nil
		}
	}
	return TeamDefinitionLoadCandidate{}, ErrTeamDefinitionNotFound
}

func (d TeamDefinition) ID() string {
	return d.id
}

func (d TeamDefinition) Version() int {
	return d.version
}

func (d TeamDefinition) Scope() TeamDefinitionScope {
	return d.scope
}

func (d TeamDefinition) ScopeIdentity() agents.ScopeIdentity {
	return d.scopeIdentity
}

func (d TeamDefinition) Name() string {
	return d.name
}

func (d TeamDefinition) Status() TeamDefinitionStatus {
	return d.status
}

func (d TeamDefinition) Roles() []TeamDefinitionRole {
	return copyTeamDefinitionRoles(d.roles)
}

func (d TeamDefinition) MainAgentDefinitionID() string {
	for _, role := range d.roles {
		if role.Kind == TeamDefinitionRoleMain {
			return role.AgentDefinitionID
		}
	}
	return ""
}

func (d TeamDefinition) SubAgentDefinitionIDs() []string {
	capacity := len(d.roles) - 1
	if capacity < 0 {
		capacity = 0
	}
	ids := make([]string, 0, capacity)
	for _, role := range d.roles {
		if role.Kind == TeamDefinitionRoleSubAgent {
			ids = append(ids, role.AgentDefinitionID)
		}
	}
	return ids
}

func (d TeamDefinition) Digest() string {
	return d.digest
}

func validateTeamDefinitionInput(input TeamDefinitionInput) error {
	if input.ID == "" || input.Version <= 0 || input.Name == "" {
		return ErrInvalidTeamDefinition
	}
	switch input.Status {
	case TeamDefinitionActive, TeamDefinitionArchived:
	default:
		return ErrInvalidTeamDefinition
	}
	switch input.Scope {
	case TeamDefinitionScopeProject:
		if input.ScopeIdentity.ProjectID == "" || input.ScopeIdentity.GenerationID != "" {
			return ErrInvalidTeamDefinition
		}
	case TeamDefinitionScopeReusable:
		if input.ScopeIdentity != (agents.ScopeIdentity{}) {
			return ErrInvalidTeamDefinition
		}
	default:
		return ErrInvalidTeamDefinition
	}
	return nil
}

func validateTeamDefinitionCatalogs(
	definitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
) error {
	definitionKeys := make(map[teamAgentDefinitionKey]struct{}, len(definitions))
	for _, definition := range definitions {
		validated, err := agents.NewAgentDefinition(definition)
		if err != nil {
			return fmt.Errorf("%w: agent definition: %v", ErrInvalidTeamDefinitionCatalog, err)
		}
		key := teamAgentDefinitionKey{
			id:           validated.ID,
			version:      validated.Version,
			scope:        validated.Scope,
			projectID:    validated.ScopeIdentity.ProjectID,
			generationID: validated.ScopeIdentity.GenerationID,
		}
		if _, ok := definitionKeys[key]; ok {
			return fmt.Errorf("%w: agent definition %s", ErrDuplicateTeamDefinitionReference, validated.ID)
		}
		definitionKeys[key] = struct{}{}
	}
	profileIDs := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		validated, err := loomruntime.NewRuntimeProfile(profile)
		if err != nil {
			return fmt.Errorf("%w: runtime profile: %v", ErrInvalidTeamDefinitionCatalog, err)
		}
		if _, ok := profileIDs[validated.ID]; ok {
			return fmt.Errorf("%w: runtime profile %s", ErrDuplicateTeamDefinitionReference, validated.ID)
		}
		profileIDs[validated.ID] = struct{}{}
	}
	return nil
}

func validateAndNormalizeTeamDefinitionRoles(
	input TeamDefinitionInput,
	definitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
) ([]TeamDefinitionRole, error) {
	if len(input.Roles) == 0 || len(input.Roles) > MaxTeamAgentCount {
		return nil, ErrInvalidTeamDefinitionRole
	}
	profileIDs := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		profileIDs[profile.ID] = struct{}{}
	}
	seenAgents := make(map[string]struct{}, len(input.Roles))
	mainCount := 0
	subAgentCount := 0
	roles := copyTeamDefinitionRoles(input.Roles)
	for _, role := range roles {
		if role.AgentDefinitionID == "" || role.RuntimeProfileID == "" || role.Responsibility == "" {
			return nil, ErrInvalidTeamDefinitionRole
		}
		switch role.Kind {
		case TeamDefinitionRoleMain:
			mainCount++
		case TeamDefinitionRoleSubAgent:
			subAgentCount++
		default:
			return nil, ErrInvalidTeamDefinitionRole
		}
		if _, ok := seenAgents[role.AgentDefinitionID]; ok {
			return nil, fmt.Errorf("%w: agent definition %s", ErrDuplicateTeamDefinitionReference, role.AgentDefinitionID)
		}
		seenAgents[role.AgentDefinitionID] = struct{}{}
		if _, ok := profileIDs[role.RuntimeProfileID]; !ok {
			return nil, fmt.Errorf("%w: runtime profile %s", ErrInventedTeamDefinitionReference, role.RuntimeProfileID)
		}
		context := agents.ResolutionContext{DefinitionID: role.AgentDefinitionID}
		if input.Scope == TeamDefinitionScopeProject {
			context.ProjectID = input.ScopeIdentity.ProjectID
		}
		if _, err := agents.ResolveDefinition(definitions, context); err != nil {
			if errors.Is(err, agents.ErrAgentDefinitionNotFound) {
				return nil, fmt.Errorf("%w: agent definition %s", ErrInventedTeamDefinitionReference, role.AgentDefinitionID)
			}
			return nil, fmt.Errorf("%w: agent definition: %v", ErrInvalidTeamDefinitionCatalog, err)
		}
	}
	if mainCount != 1 || subAgentCount > MaxTeamAgentCount-1 {
		return nil, ErrInvalidTeamDefinitionRole
	}
	normalizeTeamDefinitionRoles(roles)
	return roles, nil
}

func validateStoredTeamDefinition(current TeamDefinition) error {
	if current.digest == "" {
		return ErrInvalidTeamDefinition
	}
	if err := validateTeamDefinitionInput(TeamDefinitionInput{
		ID:            current.id,
		Version:       current.version,
		Scope:         current.scope,
		ScopeIdentity: current.scopeIdentity,
		Name:          current.name,
		Status:        current.status,
		Roles:         current.roles,
	}); err != nil {
		return err
	}
	normalized, err := validateStoredTeamDefinitionRoles(current.roles)
	if err != nil {
		return err
	}
	if !equalTeamDefinitionRoles(current.roles, normalized) {
		return ErrInvalidTeamDefinitionRole
	}
	digest, err := digestTeamDefinition(current)
	if err != nil {
		return err
	}
	if digest != current.digest {
		return ErrTeamDefinitionDigestMismatch
	}
	return nil
}

func validateStoredTeamDefinitionRoles(roles []TeamDefinitionRole) ([]TeamDefinitionRole, error) {
	if len(roles) == 0 || len(roles) > MaxTeamAgentCount {
		return nil, ErrInvalidTeamDefinitionRole
	}
	mainCount := 0
	subCount := 0
	seen := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		if role.AgentDefinitionID == "" || role.RuntimeProfileID == "" || role.Responsibility == "" {
			return nil, ErrInvalidTeamDefinitionRole
		}
		switch role.Kind {
		case TeamDefinitionRoleMain:
			mainCount++
		case TeamDefinitionRoleSubAgent:
			subCount++
		default:
			return nil, ErrInvalidTeamDefinitionRole
		}
		if _, ok := seen[role.AgentDefinitionID]; ok {
			return nil, ErrDuplicateTeamDefinitionReference
		}
		seen[role.AgentDefinitionID] = struct{}{}
	}
	if mainCount != 1 || subCount > MaxTeamAgentCount-1 {
		return nil, ErrInvalidTeamDefinitionRole
	}
	normalized := copyTeamDefinitionRoles(roles)
	normalizeTeamDefinitionRoles(normalized)
	return normalized, nil
}

func validationCandidateForTeamDefinition(definition TeamDefinition) TeamDefinitionValidationCandidate {
	return TeamDefinitionValidationCandidate{
		Valid:                 true,
		ID:                    definition.id,
		Version:               definition.version,
		Scope:                 definition.scope,
		ScopeIdentity:         definition.scopeIdentity,
		MainAgentDefinitionID: definition.MainAgentDefinitionID(),
		SubAgentDefinitionIDs: definition.SubAgentDefinitionIDs(),
		Roles:                 definition.Roles(),
		DefinitionDigest:      definition.digest,
	}
}

func loadCandidateForTeamDefinition(definition TeamDefinition) TeamDefinitionLoadCandidate {
	return TeamDefinitionLoadCandidate{
		Found:                 true,
		ID:                    definition.id,
		Version:               definition.version,
		Scope:                 definition.scope,
		ScopeIdentity:         definition.scopeIdentity,
		MainAgentDefinitionID: definition.MainAgentDefinitionID(),
		SubAgentDefinitionIDs: definition.SubAgentDefinitionIDs(),
		Roles:                 definition.Roles(),
		DefinitionDigest:      definition.digest,
	}
}

func teamDefinitionEligibleInScope(
	definition TeamDefinition,
	context agents.ScopeIdentity,
	scope TeamDefinitionScope,
) bool {
	if definition.scope != scope {
		return false
	}
	switch scope {
	case TeamDefinitionScopeProject:
		return context.ProjectID != "" && definition.scopeIdentity.ProjectID == context.ProjectID
	case TeamDefinitionScopeReusable:
		return definition.scopeIdentity == (agents.ScopeIdentity{})
	default:
		return false
	}
}

func digestTeamDefinition(input TeamDefinition) (string, error) {
	canonical := struct {
		ID            string
		Version       int
		Scope         TeamDefinitionScope
		ScopeIdentity agents.ScopeIdentity
		Name          string
		Status        TeamDefinitionStatus
		Roles         []TeamDefinitionRole
	}{
		ID:            input.id,
		Version:       input.version,
		Scope:         input.scope,
		ScopeIdentity: input.scopeIdentity,
		Name:          input.name,
		Status:        input.status,
		Roles:         input.roles,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: digest: %v", ErrInvalidTeamDefinition, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

type teamAgentDefinitionKey struct {
	id           string
	version      int
	scope        agents.Scope
	projectID    string
	generationID string
}

func copyTeamDefinitionRoles(input []TeamDefinitionRole) []TeamDefinitionRole {
	return append([]TeamDefinitionRole(nil), input...)
}

func normalizeTeamDefinitionRoles(roles []TeamDefinitionRole) {
	sort.Slice(roles, func(i, j int) bool {
		if roles[i].Kind != roles[j].Kind {
			return roles[i].Kind == TeamDefinitionRoleMain
		}
		if roles[i].AgentDefinitionID != roles[j].AgentDefinitionID {
			return roles[i].AgentDefinitionID < roles[j].AgentDefinitionID
		}
		return roles[i].RuntimeProfileID < roles[j].RuntimeProfileID
	})
}

func equalTeamDefinitionRoles(left, right []TeamDefinitionRole) bool {
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

func cloneTeamDefinition(input TeamDefinition) TeamDefinition {
	input.roles = copyTeamDefinitionRoles(input.roles)
	return input
}
