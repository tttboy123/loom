package agents

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidAgentDefinition   = errors.New("invalid agent definition")
	ErrAgentDefinitionNotFound  = errors.New("agent definition not found")
	ErrDuplicateAgentDefinition = errors.New("duplicate agent definition")
)

type Scope string

const (
	ScopeProject   Scope = "project"
	ScopeReusable  Scope = "reusable"
	ScopeTransient Scope = "transient"
)

type DefinitionStatus string

const (
	DefinitionActive   DefinitionStatus = "active"
	DefinitionArchived DefinitionStatus = "archived"
)

type ScopeIdentity struct {
	ProjectID    string
	GenerationID string
}

type AgentDefinition struct {
	ID            string
	Version       int
	Scope         Scope
	ScopeIdentity ScopeIdentity
	Name          string
	RoleSpec      string
	Status        DefinitionStatus
}

type ResolutionContext struct {
	DefinitionID string
	ProjectID    string
	GenerationID string
}

func NewAgentDefinition(input AgentDefinition) (AgentDefinition, error) {
	if err := validateAgentDefinition(input); err != nil {
		return AgentDefinition{}, err
	}
	return input, nil
}

func ResolveDefinition(inputs []AgentDefinition, context ResolutionContext) (AgentDefinition, error) {
	if context.DefinitionID == "" {
		return AgentDefinition{}, ErrAgentDefinitionNotFound
	}

	seen := make(map[definitionKey]struct{}, len(inputs))
	for _, input := range inputs {
		if err := validateAgentDefinition(input); err != nil {
			return AgentDefinition{}, err
		}
		key := keyForDefinition(input)
		if _, ok := seen[key]; ok {
			return AgentDefinition{}, fmt.Errorf("%w: %s", ErrDuplicateAgentDefinition, input.ID)
		}
		seen[key] = struct{}{}
	}

	for _, scope := range []Scope{ScopeProject, ScopeReusable, ScopeTransient} {
		var selected AgentDefinition
		found := false
		for _, input := range inputs {
			if input.Status == DefinitionArchived || !matchesDefinitionID(input, context.DefinitionID) || !eligibleInScope(input, context, scope) {
				continue
			}
			if !found || input.Version > selected.Version {
				selected = input
				found = true
			}
		}
		if found {
			return selected, nil
		}
	}

	return AgentDefinition{}, ErrAgentDefinitionNotFound
}

type definitionKey struct {
	id           string
	version      int
	scope        Scope
	projectID    string
	generationID string
}

func keyForDefinition(input AgentDefinition) definitionKey {
	return definitionKey{
		id:           input.ID,
		version:      input.Version,
		scope:        input.Scope,
		projectID:    input.ScopeIdentity.ProjectID,
		generationID: input.ScopeIdentity.GenerationID,
	}
}

func validateAgentDefinition(input AgentDefinition) error {
	if input.ID == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidAgentDefinition)
	}
	if input.Version <= 0 {
		return fmt.Errorf("%w: nonpositive version", ErrInvalidAgentDefinition)
	}
	if input.Name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidAgentDefinition)
	}
	if input.RoleSpec == "" {
		return fmt.Errorf("%w: empty role specification", ErrInvalidAgentDefinition)
	}
	switch input.Status {
	case DefinitionActive, DefinitionArchived:
	default:
		return fmt.Errorf("%w: invalid status %q", ErrInvalidAgentDefinition, input.Status)
	}

	switch input.Scope {
	case ScopeProject:
		if input.ScopeIdentity.ProjectID == "" || input.ScopeIdentity.GenerationID != "" {
			return fmt.Errorf("%w: invalid project scope identity", ErrInvalidAgentDefinition)
		}
	case ScopeReusable:
		if input.ScopeIdentity.ProjectID != "" || input.ScopeIdentity.GenerationID != "" {
			return fmt.Errorf("%w: invalid reusable scope identity", ErrInvalidAgentDefinition)
		}
	case ScopeTransient:
		if input.ScopeIdentity.ProjectID != "" || input.ScopeIdentity.GenerationID == "" {
			return fmt.Errorf("%w: invalid transient scope identity", ErrInvalidAgentDefinition)
		}
	default:
		return fmt.Errorf("%w: invalid scope %q", ErrInvalidAgentDefinition, input.Scope)
	}

	return nil
}

func matchesDefinitionID(input AgentDefinition, id string) bool {
	return input.ID == id
}

func eligibleInScope(input AgentDefinition, context ResolutionContext, scope Scope) bool {
	if input.Scope != scope {
		return false
	}

	switch scope {
	case ScopeProject:
		return context.ProjectID != "" && input.ScopeIdentity.ProjectID == context.ProjectID
	case ScopeReusable:
		return true
	case ScopeTransient:
		return context.GenerationID != "" && input.ScopeIdentity.GenerationID == context.GenerationID
	default:
		return false
	}
}
