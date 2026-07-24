package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"

	"loom-pi-rebuild/internal/agents"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidSavedTeamRuntimeBinding        = errors.New("invalid saved team runtime binding")
	ErrInvalidSavedTeamRuntimeDiscovery      = errors.New("invalid saved team runtime discovery")
	ErrInvalidSavedTeamRuntimeSelection      = errors.New("invalid saved team runtime selection")
	ErrIncompleteSavedTeamRuntimeSelection   = errors.New("incomplete saved team runtime selection")
	ErrDuplicateSavedTeamRuntimeSelection    = errors.New("duplicate saved team runtime selection")
	ErrExtraSavedTeamRuntimeSelection        = errors.New("extra saved team runtime selection")
	ErrSavedTeamRuntimeInstanceNotFound      = errors.New("saved team runtime instance not found")
	ErrSavedTeamRuntimeInstanceAmbiguous     = errors.New("saved team runtime instance ambiguous")
	ErrSavedTeamRuntimeModelUnavailable      = errors.New("saved team runtime model unavailable")
	ErrSavedTeamRuntimeCapacityExceeded      = errors.New("saved team runtime capacity exceeded")
	ErrSavedTeamRuntimeBindingSourceMismatch = errors.New("saved team runtime binding source mismatch")
	ErrSavedTeamRuntimeBindingDigestMismatch = errors.New("saved team runtime binding digest mismatch")
)

type SavedTeamRuntimeSelection struct {
	AgentDefinitionID string
	RuntimeInstanceID string
}

type SavedTeamRuntimeRoleBinding struct {
	Kind              TeamDefinitionRoleKind
	AgentDefinitionID string
	RuntimeProfileID  string
	RuntimeInstanceID string
	Binding           loomruntime.BindingCandidate
}

type SavedTeamRuntimeBindingCandidate struct {
	ready                  bool
	teamDefinitionID       string
	teamDefinitionVersion  int
	teamDefinitionScope    TeamDefinitionScope
	scopeIdentity          agents.ScopeIdentity
	teamDefinitionDigest   string
	runtimeDiscoveryDigest string
	mainBinding            SavedTeamRuntimeRoleBinding
	subAgentBindings       []SavedTeamRuntimeRoleBinding
	roleCount              int
	bindingDigest          string
}

type SavedTeamRuntimeBindingValidationCandidate struct {
	Valid                  bool
	TeamDefinitionID       string
	TeamDefinitionDigest   string
	RuntimeDiscoveryDigest string
	BindingDigest          string
	MainAgentDefinitionID  string
	RoleCount              int
}

func BuildSavedTeamRuntimeBinding(
	teamDefinitions []TeamDefinition,
	teamID string,
	context agents.ScopeIdentity,
	agentDefinitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
) (SavedTeamRuntimeBindingCandidate, error) {
	load, err := ResolveTeamDefinition(teamDefinitions, teamID, context)
	if err != nil {
		return SavedTeamRuntimeBindingCandidate{}, err
	}
	selected, found := exactResolvedTeamDefinition(teamDefinitions, load)
	if !found {
		return SavedTeamRuntimeBindingCandidate{}, ErrSavedTeamRuntimeBindingSourceMismatch
	}
	if _, err := ValidateTeamDefinition(selected, agentDefinitions, profiles); err != nil {
		return SavedTeamRuntimeBindingCandidate{}, err
	}
	if discovery.Digest() == "" {
		return SavedTeamRuntimeBindingCandidate{}, ErrInvalidSavedTeamRuntimeBinding
	}

	selectionByAgent, err := validateSavedTeamRuntimeSelections(load.Roles, selections)
	if err != nil {
		return SavedTeamRuntimeBindingCandidate{}, err
	}
	profileByID := make(map[string]loomruntime.RuntimeProfile, len(profiles))
	for _, profile := range profiles {
		if profile.ID == "" {
			return SavedTeamRuntimeBindingCandidate{}, ErrInvalidTeamDefinitionCatalog
		}
		if _, exists := profileByID[profile.ID]; exists {
			return SavedTeamRuntimeBindingCandidate{}, ErrInvalidTeamDefinitionCatalog
		}
		profileByID[profile.ID] = copyTeamDraftRuntimeProfile(profile)
	}
	observations := discovery.Observations()
	observationByID := make(map[string]loomruntime.RuntimeObservation, len(observations))
	for _, observation := range observations {
		validatedInstance, err := loomruntime.NewRuntimeInstance(observation.Instance)
		if err != nil {
			return SavedTeamRuntimeBindingCandidate{}, err
		}
		if !reflect.DeepEqual(validatedInstance, observation.Instance) ||
			!validSavedTeamRuntimeModels(observation.ModelIDs) {
			return SavedTeamRuntimeBindingCandidate{}, ErrInvalidSavedTeamRuntimeDiscovery
		}
		id := observation.Instance.ID
		if _, exists := observationByID[id]; exists {
			return SavedTeamRuntimeBindingCandidate{}, ErrSavedTeamRuntimeInstanceAmbiguous
		}
		observationByID[id] = observation
	}

	bindings := make([]SavedTeamRuntimeRoleBinding, 0, len(load.Roles))
	usage := make(map[string]int)
	for _, role := range load.Roles {
		selection := selectionByAgent[role.AgentDefinitionID]
		profile, ok := profileByID[role.RuntimeProfileID]
		if !ok {
			return SavedTeamRuntimeBindingCandidate{}, ErrInventedTeamDefinitionReference
		}
		observation, ok := observationByID[selection.RuntimeInstanceID]
		if !ok {
			return SavedTeamRuntimeBindingCandidate{}, ErrSavedTeamRuntimeInstanceNotFound
		}
		if !containsSavedTeamModel(observation.ModelIDs, profile.ModelID) {
			return SavedTeamRuntimeBindingCandidate{}, ErrSavedTeamRuntimeModelUnavailable
		}
		binding, err := loomruntime.ValidateBinding(profile, observation.Instance)
		if err != nil {
			return SavedTeamRuntimeBindingCandidate{}, err
		}
		usage[selection.RuntimeInstanceID]++
		bindings = append(bindings, SavedTeamRuntimeRoleBinding{
			Kind:              role.Kind,
			AgentDefinitionID: role.AgentDefinitionID,
			RuntimeProfileID:  role.RuntimeProfileID,
			RuntimeInstanceID: selection.RuntimeInstanceID,
			Binding:           binding,
		})
	}
	for runtimeID, count := range usage {
		if observationByID[runtimeID].Instance.Capacity < count {
			return SavedTeamRuntimeBindingCandidate{}, ErrSavedTeamRuntimeCapacityExceeded
		}
	}

	main, subAgents, err := normalizeSavedTeamRuntimeBindings(bindings)
	if err != nil {
		return SavedTeamRuntimeBindingCandidate{}, err
	}
	candidate := SavedTeamRuntimeBindingCandidate{
		ready:                  true,
		teamDefinitionID:       load.ID,
		teamDefinitionVersion:  load.Version,
		teamDefinitionScope:    load.Scope,
		scopeIdentity:          load.ScopeIdentity,
		teamDefinitionDigest:   load.DefinitionDigest,
		runtimeDiscoveryDigest: discovery.Digest(),
		mainBinding:            main,
		subAgentBindings:       append([]SavedTeamRuntimeRoleBinding(nil), subAgents...),
		roleCount:              1 + len(subAgents),
	}
	candidate.bindingDigest, err = digestSavedTeamRuntimeBinding(candidate)
	if err != nil {
		return SavedTeamRuntimeBindingCandidate{}, err
	}
	return candidate, nil
}

func ValidateSavedTeamRuntimeBinding(
	current SavedTeamRuntimeBindingCandidate,
	teamDefinitions []TeamDefinition,
	teamID string,
	context agents.ScopeIdentity,
	agentDefinitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
) (SavedTeamRuntimeBindingValidationCandidate, error) {
	if !current.ready || current.teamDefinitionID == "" ||
		current.teamDefinitionVersion <= 0 || current.teamDefinitionDigest == "" ||
		current.runtimeDiscoveryDigest == "" || current.bindingDigest == "" ||
		current.mainBinding.Kind != TeamDefinitionRoleMain ||
		current.roleCount != 1+len(current.subAgentBindings) {
		return SavedTeamRuntimeBindingValidationCandidate{}, ErrInvalidSavedTeamRuntimeBinding
	}
	digest, err := digestSavedTeamRuntimeBinding(current)
	if err != nil {
		return SavedTeamRuntimeBindingValidationCandidate{}, err
	}
	if digest != current.bindingDigest {
		return SavedTeamRuntimeBindingValidationCandidate{}, ErrSavedTeamRuntimeBindingDigestMismatch
	}
	expected, err := BuildSavedTeamRuntimeBinding(
		teamDefinitions, teamID, context, agentDefinitions, profiles, discovery, selections,
	)
	if err != nil {
		return SavedTeamRuntimeBindingValidationCandidate{}, err
	}
	if !reflect.DeepEqual(current, expected) {
		return SavedTeamRuntimeBindingValidationCandidate{}, ErrSavedTeamRuntimeBindingSourceMismatch
	}
	return SavedTeamRuntimeBindingValidationCandidate{
		Valid:                  true,
		TeamDefinitionID:       current.teamDefinitionID,
		TeamDefinitionDigest:   current.teamDefinitionDigest,
		RuntimeDiscoveryDigest: current.runtimeDiscoveryDigest,
		BindingDigest:          current.bindingDigest,
		MainAgentDefinitionID:  current.mainBinding.AgentDefinitionID,
		RoleCount:              current.roleCount,
	}, nil
}

func exactResolvedTeamDefinition(
	definitions []TeamDefinition,
	load TeamDefinitionLoadCandidate,
) (TeamDefinition, bool) {
	for _, definition := range definitions {
		if definition.Digest() == load.DefinitionDigest &&
			definition.ID() == load.ID &&
			definition.Version() == load.Version &&
			definition.Scope() == load.Scope &&
			definition.ScopeIdentity() == load.ScopeIdentity {
			return cloneTeamDefinition(definition), true
		}
	}
	return TeamDefinition{}, false
}

func validateSavedTeamRuntimeSelections(
	roles []TeamDefinitionRole,
	selections []SavedTeamRuntimeSelection,
) (map[string]SavedTeamRuntimeSelection, error) {
	if len(selections) == 0 {
		return nil, ErrIncompleteSavedTeamRuntimeSelection
	}
	roleIDs := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		roleIDs[role.AgentDefinitionID] = struct{}{}
	}
	byAgent := make(map[string]SavedTeamRuntimeSelection, len(selections))
	for _, selection := range selections {
		if selection.AgentDefinitionID == "" || selection.RuntimeInstanceID == "" {
			return nil, ErrInvalidSavedTeamRuntimeSelection
		}
		if _, exists := byAgent[selection.AgentDefinitionID]; exists {
			return nil, ErrDuplicateSavedTeamRuntimeSelection
		}
		if _, exists := roleIDs[selection.AgentDefinitionID]; !exists {
			return nil, ErrExtraSavedTeamRuntimeSelection
		}
		byAgent[selection.AgentDefinitionID] = selection
	}
	if len(byAgent) != len(roleIDs) {
		return nil, ErrIncompleteSavedTeamRuntimeSelection
	}
	return byAgent, nil
}

func normalizeSavedTeamRuntimeBindings(
	input []SavedTeamRuntimeRoleBinding,
) (SavedTeamRuntimeRoleBinding, []SavedTeamRuntimeRoleBinding, error) {
	var main SavedTeamRuntimeRoleBinding
	subAgents := make([]SavedTeamRuntimeRoleBinding, 0, len(input)-1)
	for _, binding := range input {
		switch binding.Kind {
		case TeamDefinitionRoleMain:
			if main.AgentDefinitionID != "" {
				return SavedTeamRuntimeRoleBinding{}, nil, ErrInvalidSavedTeamRuntimeBinding
			}
			main = binding
		case TeamDefinitionRoleSubAgent:
			subAgents = append(subAgents, binding)
		default:
			return SavedTeamRuntimeRoleBinding{}, nil, ErrInvalidSavedTeamRuntimeBinding
		}
	}
	if main.AgentDefinitionID == "" || len(subAgents) > 2 {
		return SavedTeamRuntimeRoleBinding{}, nil, ErrInvalidSavedTeamRuntimeBinding
	}
	sort.Slice(subAgents, func(i, j int) bool {
		if subAgents[i].AgentDefinitionID != subAgents[j].AgentDefinitionID {
			return subAgents[i].AgentDefinitionID < subAgents[j].AgentDefinitionID
		}
		if subAgents[i].RuntimeProfileID != subAgents[j].RuntimeProfileID {
			return subAgents[i].RuntimeProfileID < subAgents[j].RuntimeProfileID
		}
		return subAgents[i].RuntimeInstanceID < subAgents[j].RuntimeInstanceID
	})
	return main, subAgents, nil
}

func containsSavedTeamModel(models []string, wanted string) bool {
	for _, model := range models {
		if model == wanted {
			return true
		}
	}
	return false
}

func validSavedTeamRuntimeModels(models []string) bool {
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		if model == "" {
			return false
		}
		if _, exists := seen[model]; exists {
			return false
		}
		seen[model] = struct{}{}
	}
	return true
}

func digestSavedTeamRuntimeBinding(input SavedTeamRuntimeBindingCandidate) (string, error) {
	canonical := struct {
		Ready                  bool
		TeamDefinitionID       string
		TeamDefinitionVersion  int
		TeamDefinitionScope    TeamDefinitionScope
		ScopeIdentity          agents.ScopeIdentity
		TeamDefinitionDigest   string
		RuntimeDiscoveryDigest string
		MainBinding            SavedTeamRuntimeRoleBinding
		SubAgentBindings       []SavedTeamRuntimeRoleBinding
		RoleCount              int
	}{
		Ready:                  input.ready,
		TeamDefinitionID:       input.teamDefinitionID,
		TeamDefinitionVersion:  input.teamDefinitionVersion,
		TeamDefinitionScope:    input.teamDefinitionScope,
		ScopeIdentity:          input.scopeIdentity,
		TeamDefinitionDigest:   input.teamDefinitionDigest,
		RuntimeDiscoveryDigest: input.runtimeDiscoveryDigest,
		MainBinding:            input.mainBinding,
		SubAgentBindings:       append([]SavedTeamRuntimeRoleBinding(nil), input.subAgentBindings...),
		RoleCount:              input.roleCount,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func cloneSavedTeamRuntimeBinding(input SavedTeamRuntimeBindingCandidate) SavedTeamRuntimeBindingCandidate {
	input.subAgentBindings = append([]SavedTeamRuntimeRoleBinding(nil), input.subAgentBindings...)
	return input
}

func (c SavedTeamRuntimeBindingCandidate) Ready() bool {
	return c.ready
}

func (c SavedTeamRuntimeBindingCandidate) TeamDefinitionID() string {
	return c.teamDefinitionID
}

func (c SavedTeamRuntimeBindingCandidate) TeamDefinitionVersion() int {
	return c.teamDefinitionVersion
}

func (c SavedTeamRuntimeBindingCandidate) TeamDefinitionScope() TeamDefinitionScope {
	return c.teamDefinitionScope
}

func (c SavedTeamRuntimeBindingCandidate) ScopeIdentity() agents.ScopeIdentity {
	return c.scopeIdentity
}

func (c SavedTeamRuntimeBindingCandidate) TeamDefinitionDigest() string {
	return c.teamDefinitionDigest
}

func (c SavedTeamRuntimeBindingCandidate) RuntimeDiscoveryDigest() string {
	return c.runtimeDiscoveryDigest
}

func (c SavedTeamRuntimeBindingCandidate) MainBinding() SavedTeamRuntimeRoleBinding {
	return c.mainBinding
}

func (c SavedTeamRuntimeBindingCandidate) SubAgentBindings() []SavedTeamRuntimeRoleBinding {
	return append([]SavedTeamRuntimeRoleBinding(nil), c.subAgentBindings...)
}

func (c SavedTeamRuntimeBindingCandidate) RoleCount() int {
	return c.roleCount
}

func (c SavedTeamRuntimeBindingCandidate) BindingDigest() string {
	return c.bindingDigest
}
