package teams

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/mode"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

var (
	ErrInvalidTeamResolutionCatalog     = errors.New("invalid team resolution catalog")
	ErrTeamResolutionRequiresAgentMode  = errors.New("team resolution requires agent mode")
	ErrInvalidTeamResolutionTarget      = errors.New("invalid team resolution target")
	ErrTeamResolutionAgentNotFound      = errors.New("team resolution agent not found")
	ErrTeamResolutionTeamNotFound       = errors.New("team resolution team not found")
	ErrTeamResolutionTargetNotFound     = errors.New("team resolution target not found")
	ErrAmbiguousTeamResolutionTarget    = errors.New("ambiguous team resolution target")
	ErrInvalidTeamResolutionMainCatalog = errors.New("invalid team resolution main catalog")
	ErrInvalidTeamResolutionDefaultTeam = errors.New("invalid team resolution default team")
)

type TeamResolutionKind string

const (
	TeamResolutionLoadTeam   TeamResolutionKind = "load_team"
	TeamResolutionDirectMain TeamResolutionKind = "direct_main"
	TeamResolutionDraftSeed  TeamResolutionKind = "draft_seed"
)

type TeamResolutionCatalogInput struct {
	AgentDefinitions       []agents.AgentDefinition
	RuntimeProfiles        []loomruntime.RuntimeProfile
	TeamDefinitions        []TeamDefinition
	MainAgentDefinitionIDs []string
	DefaultMainAgentID     string
	ProjectDefaultTeamID   string
	ReusableDefaultTeamID  string
}

type TeamResolutionCandidate struct {
	Resolved                     bool
	Mode                         mode.Mode
	Trigger                      mode.Trigger
	TargetID                     string
	Kind                         TeamResolutionKind
	Team                         TeamDefinitionLoadCandidate
	MainAgentDefinitionID        string
	SelectedSubAgentDefinitionID string
	RequiresDraft                bool
	Digest                       string
}

func ResolveAgentModeTeam(
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
) (TeamResolutionCandidate, error) {
	if mode.Route(intent).Mode != mode.ModeAgent {
		return TeamResolutionCandidate{}, ErrTeamResolutionRequiresAgentMode
	}
	copied, mainIDs, err := validateAndCopyTeamResolutionCatalog(context, catalog)
	if err != nil {
		return TeamResolutionCandidate{}, err
	}

	switch intent.Trigger {
	case mode.TriggerSelectTeam:
		if intent.TargetID == "" {
			return TeamResolutionCandidate{}, ErrInvalidTeamResolutionTarget
		}
		team, err := resolveRequiredTeam(copied.TeamDefinitions, intent.TargetID, context)
		if err != nil {
			return TeamResolutionCandidate{}, err
		}
		return buildTeamResolutionCandidate(intent, TeamResolutionLoadTeam, team, team.MainAgentDefinitionID, "", false)
	case mode.TriggerSelectAgent:
		if intent.TargetID == "" {
			return TeamResolutionCandidate{}, ErrInvalidTeamResolutionTarget
		}
		return resolveSelectedAgent(intent, context, copied, mainIDs)
	case mode.TriggerUseAgent:
		if intent.TargetID != "" {
			return TeamResolutionCandidate{}, ErrInvalidTeamResolutionTarget
		}
		if copied.ProjectDefaultTeamID != "" {
			team, err := resolveRequiredTeam(copied.TeamDefinitions, copied.ProjectDefaultTeamID, context)
			if err != nil {
				return TeamResolutionCandidate{}, err
			}
			return buildTeamResolutionCandidate(intent, TeamResolutionLoadTeam, team, team.MainAgentDefinitionID, "", false)
		}
		if copied.ReusableDefaultTeamID != "" {
			team, err := resolveRequiredTeam(copied.TeamDefinitions, copied.ReusableDefaultTeamID, agents.ScopeIdentity{})
			if err != nil {
				return TeamResolutionCandidate{}, err
			}
			return buildTeamResolutionCandidate(intent, TeamResolutionLoadTeam, team, team.MainAgentDefinitionID, "", false)
		}
		return buildTeamResolutionCandidate(intent, TeamResolutionDraftSeed, TeamDefinitionLoadCandidate{}, copied.DefaultMainAgentID, "", true)
	case mode.TriggerAssign:
		if intent.TargetID == "" {
			return TeamResolutionCandidate{}, ErrInvalidTeamResolutionTarget
		}
		team, teamFound, err := tryResolveTeam(copied.TeamDefinitions, intent.TargetID, context)
		if err != nil {
			return TeamResolutionCandidate{}, err
		}
		_, agentFound, err := tryResolveAgent(copied.AgentDefinitions, intent.TargetID, context)
		if err != nil {
			return TeamResolutionCandidate{}, err
		}
		if teamFound && agentFound {
			return TeamResolutionCandidate{}, ErrAmbiguousTeamResolutionTarget
		}
		if teamFound {
			return buildTeamResolutionCandidate(intent, TeamResolutionLoadTeam, team, team.MainAgentDefinitionID, "", false)
		}
		if agentFound {
			return resolveSelectedAgent(intent, context, copied, mainIDs)
		}
		return TeamResolutionCandidate{}, ErrTeamResolutionTargetNotFound
	default:
		return TeamResolutionCandidate{}, ErrTeamResolutionRequiresAgentMode
	}
}

func validateAndCopyTeamResolutionCatalog(
	context agents.ScopeIdentity,
	input TeamResolutionCatalogInput,
) (TeamResolutionCatalogInput, map[string]struct{}, error) {
	if context.ProjectID == "" || context.GenerationID != "" {
		return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionCatalog
	}
	copied := copyTeamResolutionCatalogInput(input)
	definitionKeys := make(map[resolverAgentDefinitionKey]struct{}, len(copied.AgentDefinitions))
	for _, definition := range copied.AgentDefinitions {
		validated, err := agents.NewAgentDefinition(definition)
		if err != nil {
			return TeamResolutionCatalogInput{}, nil, fmt.Errorf("%w: agent definition: %v", ErrInvalidTeamResolutionCatalog, err)
		}
		key := resolverAgentDefinitionKey{
			id: validated.ID, version: validated.Version, scope: validated.Scope,
			projectID: validated.ScopeIdentity.ProjectID, generationID: validated.ScopeIdentity.GenerationID,
		}
		if _, ok := definitionKeys[key]; ok {
			return TeamResolutionCatalogInput{}, nil, fmt.Errorf("%w: duplicate agent definition %s", ErrInvalidTeamResolutionCatalog, validated.ID)
		}
		definitionKeys[key] = struct{}{}
	}
	profileIDs := make(map[string]struct{}, len(copied.RuntimeProfiles))
	for _, profile := range copied.RuntimeProfiles {
		validated, err := loomruntime.NewRuntimeProfile(profile)
		if err != nil {
			return TeamResolutionCatalogInput{}, nil, fmt.Errorf("%w: runtime profile: %v", ErrInvalidTeamResolutionCatalog, err)
		}
		if _, ok := profileIDs[validated.ID]; ok {
			return TeamResolutionCatalogInput{}, nil, fmt.Errorf("%w: duplicate runtime profile %s", ErrInvalidTeamResolutionCatalog, validated.ID)
		}
		profileIDs[validated.ID] = struct{}{}
	}
	for _, definition := range copied.TeamDefinitions {
		if _, err := ValidateTeamDefinition(definition, copied.AgentDefinitions, copied.RuntimeProfiles); err != nil {
			return TeamResolutionCatalogInput{}, nil, err
		}
	}

	if copied.DefaultMainAgentID == "" || len(copied.MainAgentDefinitionIDs) == 0 {
		return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionMainCatalog
	}
	mainIDs := make(map[string]struct{}, len(copied.MainAgentDefinitionIDs))
	for _, id := range copied.MainAgentDefinitionIDs {
		if id == "" {
			return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionMainCatalog
		}
		if _, ok := mainIDs[id]; ok {
			return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionMainCatalog
		}
		mainIDs[id] = struct{}{}
		if _, found, err := tryResolveAgent(copied.AgentDefinitions, id, context); err != nil || !found {
			return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionMainCatalog
		}
	}
	if _, ok := mainIDs[copied.DefaultMainAgentID]; !ok {
		return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionMainCatalog
	}

	if copied.ProjectDefaultTeamID != "" {
		team, err := ResolveTeamDefinition(copied.TeamDefinitions, copied.ProjectDefaultTeamID, context)
		if err != nil || team.Scope != TeamDefinitionScopeProject {
			return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionDefaultTeam
		}
	}
	if copied.ReusableDefaultTeamID != "" {
		team, err := ResolveTeamDefinition(copied.TeamDefinitions, copied.ReusableDefaultTeamID, agents.ScopeIdentity{})
		if err != nil || team.Scope != TeamDefinitionScopeReusable {
			return TeamResolutionCatalogInput{}, nil, ErrInvalidTeamResolutionDefaultTeam
		}
	}
	return copied, mainIDs, nil
}

func resolveSelectedAgent(
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	mainIDs map[string]struct{},
) (TeamResolutionCandidate, error) {
	definition, found, err := tryResolveAgent(catalog.AgentDefinitions, intent.TargetID, context)
	if err != nil {
		return TeamResolutionCandidate{}, err
	}
	if !found {
		return TeamResolutionCandidate{}, ErrTeamResolutionAgentNotFound
	}
	if _, ok := mainIDs[definition.ID]; ok {
		return buildTeamResolutionCandidate(intent, TeamResolutionDirectMain, TeamDefinitionLoadCandidate{}, definition.ID, "", false)
	}
	return buildTeamResolutionCandidate(
		intent,
		TeamResolutionDraftSeed,
		TeamDefinitionLoadCandidate{},
		catalog.DefaultMainAgentID,
		definition.ID,
		true,
	)
}

func resolveRequiredTeam(
	definitions []TeamDefinition,
	id string,
	context agents.ScopeIdentity,
) (TeamDefinitionLoadCandidate, error) {
	team, err := ResolveTeamDefinition(definitions, id, context)
	if errors.Is(err, ErrTeamDefinitionNotFound) {
		return TeamDefinitionLoadCandidate{}, ErrTeamResolutionTeamNotFound
	}
	if err != nil {
		return TeamDefinitionLoadCandidate{}, err
	}
	return team, nil
}

func tryResolveTeam(
	definitions []TeamDefinition,
	id string,
	context agents.ScopeIdentity,
) (TeamDefinitionLoadCandidate, bool, error) {
	team, err := ResolveTeamDefinition(definitions, id, context)
	if errors.Is(err, ErrTeamDefinitionNotFound) {
		return TeamDefinitionLoadCandidate{}, false, nil
	}
	if err != nil {
		return TeamDefinitionLoadCandidate{}, false, err
	}
	return team, true, nil
}

func tryResolveAgent(
	definitions []agents.AgentDefinition,
	id string,
	context agents.ScopeIdentity,
) (agents.AgentDefinition, bool, error) {
	definition, err := agents.ResolveDefinition(definitions, agents.ResolutionContext{
		DefinitionID: id,
		ProjectID:    context.ProjectID,
		GenerationID: context.GenerationID,
	})
	if errors.Is(err, agents.ErrAgentDefinitionNotFound) {
		return agents.AgentDefinition{}, false, nil
	}
	if err != nil {
		return agents.AgentDefinition{}, false, err
	}
	return definition, true, nil
}

func buildTeamResolutionCandidate(
	intent mode.Intent,
	kind TeamResolutionKind,
	team TeamDefinitionLoadCandidate,
	mainAgentID string,
	selectedSubAgentID string,
	requiresDraft bool,
) (TeamResolutionCandidate, error) {
	candidate := TeamResolutionCandidate{
		Resolved:                     true,
		Mode:                         mode.ModeAgent,
		Trigger:                      intent.Trigger,
		TargetID:                     intent.TargetID,
		Kind:                         kind,
		Team:                         copyTeamDefinitionLoadCandidate(team),
		MainAgentDefinitionID:        mainAgentID,
		SelectedSubAgentDefinitionID: selectedSubAgentID,
		RequiresDraft:                requiresDraft,
	}
	digest, err := digestTeamResolutionCandidate(candidate)
	if err != nil {
		return TeamResolutionCandidate{}, err
	}
	candidate.Digest = digest
	return candidate, nil
}

func digestTeamResolutionCandidate(input TeamResolutionCandidate) (string, error) {
	canonical := struct {
		Mode                         mode.Mode
		Trigger                      mode.Trigger
		TargetID                     string
		Kind                         TeamResolutionKind
		TeamDefinitionDigest         string
		MainAgentDefinitionID        string
		SelectedSubAgentDefinitionID string
		RequiresDraft                bool
	}{
		Mode:                         input.Mode,
		Trigger:                      input.Trigger,
		TargetID:                     input.TargetID,
		Kind:                         input.Kind,
		TeamDefinitionDigest:         input.Team.DefinitionDigest,
		MainAgentDefinitionID:        input.MainAgentDefinitionID,
		SelectedSubAgentDefinitionID: input.SelectedSubAgentDefinitionID,
		RequiresDraft:                input.RequiresDraft,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: resolution digest: %v", ErrInvalidTeamResolutionCatalog, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func copyTeamResolutionCatalogInput(input TeamResolutionCatalogInput) TeamResolutionCatalogInput {
	input.AgentDefinitions = append([]agents.AgentDefinition(nil), input.AgentDefinitions...)
	profiles := input.RuntimeProfiles
	input.RuntimeProfiles = make([]loomruntime.RuntimeProfile, len(profiles))
	for index, profile := range profiles {
		copied := profile
		copied.RequiredCapabilities = append([]string(nil), profile.RequiredCapabilities...)
		if profile.Budget != nil {
			budget := *profile.Budget
			copied.Budget = &budget
		}
		input.RuntimeProfiles[index] = copied
	}
	definitions := input.TeamDefinitions
	input.TeamDefinitions = make([]TeamDefinition, len(definitions))
	for index, definition := range definitions {
		input.TeamDefinitions[index] = cloneTeamDefinition(definition)
	}
	input.MainAgentDefinitionIDs = append([]string(nil), input.MainAgentDefinitionIDs...)
	return input
}

func copyTeamDefinitionLoadCandidate(input TeamDefinitionLoadCandidate) TeamDefinitionLoadCandidate {
	input.SubAgentDefinitionIDs = append([]string(nil), input.SubAgentDefinitionIDs...)
	input.Roles = copyTeamDefinitionRoles(input.Roles)
	return input
}

func cloneTeamResolutionCandidate(input TeamResolutionCandidate) TeamResolutionCandidate {
	input.Team = copyTeamDefinitionLoadCandidate(input.Team)
	return input
}

type resolverAgentDefinitionKey struct {
	id           string
	version      int
	scope        agents.Scope
	projectID    string
	generationID string
}
