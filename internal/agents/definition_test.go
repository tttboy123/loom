package agents

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestAgentDefinitionValidatesFrozenContractAndSeparatesRuntimeProfile(t *testing.T) {
	valid := AgentDefinition{
		ID:      "agent.review",
		Version: 1,
		Scope:   ScopeProject,
		ScopeIdentity: ScopeIdentity{
			ProjectID: "project.alpha",
		},
		Name:     "Reviewer",
		RoleSpec: "Review candidate changes against the frozen contract.",
		Status:   DefinitionActive,
	}

	definition, err := NewAgentDefinition(valid)
	if err != nil {
		t.Fatalf("NewAgentDefinition(valid) error = %v", err)
	}
	assertAgentDefinition(t, definition, valid)

	forbiddenFields := []string{
		"runtime", "profile", "adapter", "provider", "model", "auth",
		"executable", "device", "capacity", "credential", "secret", "run",
	}
	definitionType := reflect.TypeOf(definition)
	for i := 0; i < definitionType.NumField(); i++ {
		fieldName := strings.ToLower(definitionType.Field(i).Name)
		for _, forbidden := range forbiddenFields {
			if strings.Contains(fieldName, forbidden) {
				t.Fatalf("AgentDefinition field %q embeds forbidden RuntimeProfile or execution authority %q", definitionType.Field(i).Name, forbidden)
			}
		}
	}

	type runtimeSelection struct {
		Definition       AgentDefinition
		RuntimeProfileID string
	}
	firstSelection := runtimeSelection{
		Definition:       definition,
		RuntimeProfileID: "runtime.profile.fastcontext",
	}
	secondSelection := runtimeSelection{
		Definition:       definition,
		RuntimeProfileID: "runtime.profile.local-shell",
	}
	if firstSelection.RuntimeProfileID == secondSelection.RuntimeProfileID {
		t.Fatalf("test setup must use distinct profile identifiers")
	}
	if !reflect.DeepEqual(firstSelection.Definition, secondSelection.Definition) {
		t.Fatalf("runtime profile switch changed AgentDefinition: first=%#v second=%#v", firstSelection.Definition, secondSelection.Definition)
	}
	if !reflect.DeepEqual(firstSelection.Definition, definition) || !reflect.DeepEqual(secondSelection.Definition, definition) {
		t.Fatalf("runtime profile selection mutated validated AgentDefinition: first=%#v second=%#v want %#v", firstSelection.Definition, secondSelection.Definition, definition)
	}

	tests := []struct {
		name   string
		input  AgentDefinition
		wantIs error
	}{
		{name: "empty stable id", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.ID = "" }), wantIs: ErrInvalidAgentDefinition},
		{name: "nonpositive version", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.Version = 0 }), wantIs: ErrInvalidAgentDefinition},
		{name: "invalid scope", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.Scope = "global" }), wantIs: ErrInvalidAgentDefinition},
		{name: "project scope missing project id", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.ScopeIdentity.ProjectID = "" }), wantIs: ErrInvalidAgentDefinition},
		{name: "project scope with generation id", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.ScopeIdentity.GenerationID = "gen-1" }), wantIs: ErrInvalidAgentDefinition},
		{name: "reusable scope with project id", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.Scope = ScopeReusable }), wantIs: ErrInvalidAgentDefinition},
		{name: "transient scope missing generation id", input: withAgentDefinition(valid, func(d *AgentDefinition) {
			d.Scope = ScopeTransient
			d.ScopeIdentity = ScopeIdentity{}
		}), wantIs: ErrInvalidAgentDefinition},
		{name: "transient scope with project id", input: withAgentDefinition(valid, func(d *AgentDefinition) {
			d.Scope = ScopeTransient
			d.ScopeIdentity = ScopeIdentity{ProjectID: "project.alpha", GenerationID: "gen-1"}
		}), wantIs: ErrInvalidAgentDefinition},
		{name: "empty name", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.Name = "" }), wantIs: ErrInvalidAgentDefinition},
		{name: "empty role specification", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.RoleSpec = "" }), wantIs: ErrInvalidAgentDefinition},
		{name: "invalid status", input: withAgentDefinition(valid, func(d *AgentDefinition) { d.Status = "draft" }), wantIs: ErrInvalidAgentDefinition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAgentDefinition(tt.input)
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("NewAgentDefinition() error = %v, want errors.Is(%v)", err, tt.wantIs)
			}
		})
	}
}

func TestResolveDefinitionAppliesScopePrecedenceVersionReorderDuplicateArchivedAndNotFoundRules(t *testing.T) {
	reusableV1 := mustAgentDefinition(t, AgentDefinition{
		ID:       "agent.review",
		Version:  1,
		Scope:    ScopeReusable,
		Name:     "Reusable Reviewer v1",
		RoleSpec: "Reusable reviewer.",
		Status:   DefinitionActive,
	})
	reusableV2 := mustAgentDefinition(t, withAgentDefinition(reusableV1, func(d *AgentDefinition) {
		d.Version = 2
		d.Name = "Reusable Reviewer v2"
	}))
	projectV1 := mustAgentDefinition(t, AgentDefinition{
		ID:      "agent.review",
		Version: 1,
		Scope:   ScopeProject,
		ScopeIdentity: ScopeIdentity{
			ProjectID: "project.alpha",
		},
		Name:     "Project Reviewer v1",
		RoleSpec: "Project reviewer.",
		Status:   DefinitionActive,
	})
	projectV2 := mustAgentDefinition(t, withAgentDefinition(projectV1, func(d *AgentDefinition) {
		d.Version = 2
		d.Name = "Project Reviewer v2"
	}))
	otherProject := mustAgentDefinition(t, withAgentDefinition(projectV2, func(d *AgentDefinition) {
		d.ScopeIdentity.ProjectID = "project.beta"
		d.Name = "Other Project Reviewer"
	}))
	transient := mustAgentDefinition(t, AgentDefinition{
		ID:      "agent.review",
		Version: 9,
		Scope:   ScopeTransient,
		ScopeIdentity: ScopeIdentity{
			GenerationID: "generation.alpha",
		},
		Name:     "Transient Reviewer",
		RoleSpec: "Generated reviewer.",
		Status:   DefinitionActive,
	})
	archivedProjectV3 := mustAgentDefinition(t, withAgentDefinition(projectV2, func(d *AgentDefinition) {
		d.Version = 3
		d.Status = DefinitionArchived
		d.Name = "Archived Project Reviewer v3"
	}))

	otherStableIDHigherVersion := mustAgentDefinition(t, AgentDefinition{
		ID:      "agent.deploy",
		Version: 99,
		Scope:   ScopeProject,
		ScopeIdentity: ScopeIdentity{
			ProjectID: "project.alpha",
		},
		Name:     "Project Deployer v99",
		RoleSpec: "Deploy reviewed changes.",
		Status:   DefinitionActive,
	})

	inputs := []AgentDefinition{transient, reusableV1, projectV1, archivedProjectV3, otherProject, otherStableIDHigherVersion, reusableV2, projectV2}
	context := ResolutionContext{DefinitionID: "agent.review", ProjectID: "project.alpha", GenerationID: "generation.alpha"}

	got, err := ResolveDefinition(inputs, context)
	if err != nil {
		t.Fatalf("ResolveDefinition(project context) error = %v", err)
	}
	if got.Name != "Project Reviewer v2" {
		t.Fatalf("ResolveDefinition(project context) selected %q, want Project Reviewer v2", got.Name)
	}

	reordered := slices.Clone(inputs)
	slices.Reverse(reordered)
	reorderedGot, err := ResolveDefinition(reordered, context)
	if err != nil {
		t.Fatalf("ResolveDefinition(reordered project context) error = %v", err)
	}
	if !reflect.DeepEqual(got, reorderedGot) {
		t.Fatalf("ResolveDefinition reordered result = %#v, want %#v", reorderedGot, got)
	}

	reusableGot, err := ResolveDefinition(inputs, ResolutionContext{DefinitionID: "agent.review", ProjectID: "project.gamma", GenerationID: "generation.alpha"})
	if err != nil {
		t.Fatalf("ResolveDefinition(reusable fallback) error = %v", err)
	}
	if reusableGot.Name != "Reusable Reviewer v2" {
		t.Fatalf("ResolveDefinition(reusable fallback) selected %q, want Reusable Reviewer v2", reusableGot.Name)
	}

	transientGot, err := ResolveDefinition([]AgentDefinition{transient}, ResolutionContext{DefinitionID: "agent.review", GenerationID: "generation.alpha"})
	if err != nil {
		t.Fatalf("ResolveDefinition(transient only) error = %v", err)
	}
	if transientGot.Name != "Transient Reviewer" {
		t.Fatalf("ResolveDefinition(transient only) selected %q, want Transient Reviewer", transientGot.Name)
	}

	if _, err := ResolveDefinition([]AgentDefinition{transient}, ResolutionContext{}); !errors.Is(err, ErrAgentDefinitionNotFound) {
		t.Fatalf("ResolveDefinition(transient without generation) error = %v, want ErrAgentDefinitionNotFound", err)
	}
	if _, err := ResolveDefinition(inputs, ResolutionContext{ProjectID: "project.alpha", GenerationID: "generation.alpha"}); !errors.Is(err, ErrAgentDefinitionNotFound) {
		t.Fatalf("ResolveDefinition(empty definition id) error = %v, want ErrAgentDefinitionNotFound", err)
	}
	if _, err := ResolveDefinition([]AgentDefinition{withAgentDefinition(projectV2, func(d *AgentDefinition) { d.Status = DefinitionArchived })}, context); !errors.Is(err, ErrAgentDefinitionNotFound) {
		t.Fatalf("ResolveDefinition(archived only) error = %v, want ErrAgentDefinitionNotFound", err)
	}
	if _, err := ResolveDefinition([]AgentDefinition{projectV2, projectV2}, context); !errors.Is(err, ErrDuplicateAgentDefinition) {
		t.Fatalf("ResolveDefinition(duplicate) error = %v, want ErrDuplicateAgentDefinition", err)
	}

	mutableInputs := []AgentDefinition{projectV2}
	resolved, err := ResolveDefinition(mutableInputs, context)
	if err != nil {
		t.Fatalf("ResolveDefinition(mutable input) error = %v", err)
	}
	mutableInputs[0].Name = "mutated caller-owned name"
	if resolved.Name != "Project Reviewer v2" {
		t.Fatalf("resolved definition exposed caller-owned mutation: got %q", resolved.Name)
	}
}

func TestAgentDefinitionImportBoundaryStaysPureDomain(t *testing.T) {
	forbidden := map[string]bool{
		"database/sql": true,
		"net":          true,
		"net/http":     true,
		"os/exec":      true,

		"loom-pi-rebuild/internal/evidence":   true,
		"loom-pi-rebuild/internal/journal":    true,
		"loom-pi-rebuild/internal/mode":       true,
		"loom-pi-rebuild/internal/projection": true,
		"loom-pi-rebuild/internal/runtime":    true,
	}

	assertNoForbiddenProductionImports(t, ".", forbidden)
}

func mustAgentDefinition(t *testing.T, input AgentDefinition) AgentDefinition {
	t.Helper()

	definition, err := NewAgentDefinition(input)
	if err != nil {
		t.Fatalf("NewAgentDefinition(%#v) error = %v", input, err)
	}
	return definition
}

func withAgentDefinition(input AgentDefinition, change func(*AgentDefinition)) AgentDefinition {
	changed := input
	change(&changed)
	return changed
}

func assertAgentDefinition(t *testing.T, got, want AgentDefinition) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AgentDefinition = %#v, want %#v", got, want)
	}
}

func assertNoForbiddenProductionImports(t *testing.T, dir string, forbidden map[string]bool) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v", path, err)
		}
		for _, imported := range file.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			if forbidden[importPath] {
				t.Fatalf("%s imports forbidden boundary package %q", path, importPath)
			}
			if strings.HasPrefix(importPath, "loom-pi-rebuild/internal/") && forbidden[importPath] {
				t.Fatalf("%s imports forbidden internal package %q", path, importPath)
			}
		}
	}
}
