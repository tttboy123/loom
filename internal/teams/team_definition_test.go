package teams

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestTeamDefinitionBuildAndValidate(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()

	for _, count := range []int{1, 2, 3} {
		t.Run(string(rune('0'+count))+"_roles", func(t *testing.T) {
			current := input
			current.Roles = append([]TeamDefinitionRole(nil), input.Roles[:count]...)
			got, err := BuildTeamDefinition(current, definitions, profiles)
			if err != nil {
				t.Fatalf("BuildTeamDefinition() error = %v", err)
			}
			assertTeamDefinitionIdentity(t, got, input.ID, input.Version, TeamDefinitionScopeProject, "project.one")
			if got.MainAgentDefinitionID() != "agent.main" {
				t.Fatalf("MainAgentDefinitionID() = %q", got.MainAgentDefinitionID())
			}
			if len(got.SubAgentDefinitionIDs()) != count-1 {
				t.Fatalf("SubAgentDefinitionIDs() count = %d, want %d", len(got.SubAgentDefinitionIDs()), count-1)
			}
			assertSHA256Digest(t, got.Digest())

			candidate, err := ValidateTeamDefinition(got, definitions, profiles)
			if err != nil {
				t.Fatalf("ValidateTeamDefinition() error = %v", err)
			}
			if !candidate.Valid || candidate.DefinitionDigest != got.Digest() ||
				candidate.MainAgentDefinitionID != "agent.main" ||
				len(candidate.Roles) != count {
				t.Fatalf("validation Candidate = %#v", candidate)
			}
		})
	}

	t.Run("normalizes role order and copies mutable inputs", func(t *testing.T) {
		reordered := input
		reordered.Roles = []TeamDefinitionRole{input.Roles[2], input.Roles[0], input.Roles[1]}
		first := mustBuildTeamDefinition(t, input, definitions, profiles)
		second := mustBuildTeamDefinition(t, reordered, definitions, profiles)
		if first.Digest() != second.Digest() || !reflect.DeepEqual(first.Roles(), second.Roles()) {
			t.Fatal("semantic role reorder changed normalized definition")
		}
		reordered.Roles[0].Responsibility = "mutated"
		returned := second.Roles()
		returned[0].Responsibility = "mutated"
		if second.Roles()[0].Responsibility == "mutated" {
			t.Fatal("input/accessor mutation changed TeamDefinition")
		}
	})

	t.Run("switches profile without rewriting agent definition", func(t *testing.T) {
		alternate := profiles[0]
		alternate.ID = "profile.main.alternate"
		alternate.ModelID = "model.alternate"
		changed := input
		changed.Roles = append([]TeamDefinitionRole(nil), input.Roles...)
		changed.Roles[0].RuntimeProfileID = alternate.ID
		got := mustBuildTeamDefinition(t, changed, definitions, append(profiles, alternate))
		if got.Roles()[0].AgentDefinitionID != "agent.main" ||
			got.Roles()[0].RuntimeProfileID != alternate.ID {
			t.Fatalf("profile switch rewrote role identity: %#v", got.Roles()[0])
		}
	})
}

func TestTeamDefinitionBuildFailures(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	archived := definitions[1]
	archived.Status = agents.DefinitionArchived
	invalidDefinition := definitions[0]
	invalidDefinition.ID = ""
	invalidProfile := profiles[0]
	invalidProfile.ID = ""
	duplicateProfile := append(append([]loomruntime.RuntimeProfile(nil), profiles...), profiles[0])

	tests := []struct {
		name        string
		input       TeamDefinitionInput
		definitions []agents.AgentDefinition
		profiles    []loomruntime.RuntimeProfile
		want        error
	}{
		{name: "empty id", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.ID = "" }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "zero version", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Version = 0 }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "empty name", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Name = "" }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "invalid scope", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Scope = TeamDefinitionScope("invalid") }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "invalid project identity", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.ScopeIdentity.ProjectID = "" }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "invalid reusable identity", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Scope = TeamDefinitionScopeReusable }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "invalid status", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Status = TeamDefinitionStatus("invalid") }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "zero roles", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles = nil }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinitionRole},
		{name: "zero main", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles[0].Kind = TeamDefinitionRoleSubAgent }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinitionRole},
		{name: "two main", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles[1].Kind = TeamDefinitionRoleMain }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinitionRole},
		{name: "three subagents", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) {
			i.Roles = append(i.Roles, TeamDefinitionRole{Kind: TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent.sub.three", RuntimeProfileID: "profile.sub.three", Responsibility: "third"})
		}), definitions: append(definitions, projectAgent("agent.sub.three")), profiles: append(profiles, runtimeProfile("profile.sub.three")), want: ErrInvalidTeamDefinitionRole},
		{name: "duplicate agent", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles[1].AgentDefinitionID = "agent.main" }), definitions: definitions, profiles: profiles, want: ErrDuplicateTeamDefinitionReference},
		{name: "empty responsibility", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles[0].Responsibility = "" }), definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinitionRole},
		{name: "invented agent", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles[0].AgentDefinitionID = "agent.invented" }), definitions: definitions, profiles: profiles, want: ErrInventedTeamDefinitionReference},
		{name: "archived agent", input: input, definitions: []agents.AgentDefinition{definitions[0], archived, definitions[2]}, profiles: profiles, want: ErrInventedTeamDefinitionReference},
		{name: "invented profile", input: withTeamDefinitionInput(input, func(i *TeamDefinitionInput) { i.Roles[0].RuntimeProfileID = "profile.invented" }), definitions: definitions, profiles: profiles, want: ErrInventedTeamDefinitionReference},
		{name: "invalid definition catalog", input: input, definitions: append(definitions, invalidDefinition), profiles: profiles, want: ErrInvalidTeamDefinitionCatalog},
		{name: "invalid profile catalog", input: input, definitions: definitions, profiles: append(profiles, invalidProfile), want: ErrInvalidTeamDefinitionCatalog},
		{name: "duplicate profile catalog", input: input, definitions: definitions, profiles: duplicateProfile, want: ErrDuplicateTeamDefinitionReference},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildTeamDefinition(tt.input, tt.definitions, tt.profiles)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildTeamDefinition() error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroTeamDefinition(t, got)
		})
	}
}

func TestResolveTeamDefinition(t *testing.T) {
	projectDefinitions, projectProfiles, projectInput := teamDefinitionFixture()
	projectV1 := mustBuildTeamDefinition(t, projectInput, projectDefinitions, projectProfiles)
	projectV2Input := projectInput
	projectV2Input.Version = 2
	projectV2 := mustBuildTeamDefinition(t, projectV2Input, projectDefinitions, projectProfiles)
	archivedInput := projectInput
	archivedInput.Version = 3
	archivedInput.Status = TeamDefinitionArchived
	archived := mustBuildTeamDefinition(t, archivedInput, projectDefinitions, projectProfiles)

	reusableDefinitions := []agents.AgentDefinition{
		reusableAgent("agent.main"),
		reusableAgent("agent.sub.one"),
		reusableAgent("agent.sub.two"),
	}
	reusableInput := projectInput
	reusableInput.Scope = TeamDefinitionScopeReusable
	reusableInput.ScopeIdentity = agents.ScopeIdentity{}
	reusableV1 := mustBuildTeamDefinition(t, reusableInput, reusableDefinitions, projectProfiles)
	reusableV2Input := reusableInput
	reusableV2Input.Version = 2
	reusableV2 := mustBuildTeamDefinition(t, reusableV2Input, reusableDefinitions, projectProfiles)

	candidates := []TeamDefinition{reusableV2, archived, projectV1, reusableV1, projectV2}
	project, err := ResolveTeamDefinition(candidates, "team.delivery", agents.ScopeIdentity{ProjectID: "project.one"})
	if err != nil {
		t.Fatalf("ResolveTeamDefinition(project) error = %v", err)
	}
	assertLoadCandidate(t, project, TeamDefinitionScopeProject, "project.one", 2, projectV2.Digest())

	fallback, err := ResolveTeamDefinition(candidates, "team.delivery", agents.ScopeIdentity{ProjectID: "project.other"})
	if err != nil {
		t.Fatalf("ResolveTeamDefinition(reusable) error = %v", err)
	}
	assertLoadCandidate(t, fallback, TeamDefinitionScopeReusable, "", 2, reusableV2.Digest())

	reordered := []TeamDefinition{projectV2, reusableV1, projectV1, archived, reusableV2}
	again, err := ResolveTeamDefinition(reordered, "team.delivery", agents.ScopeIdentity{ProjectID: "project.one"})
	if err != nil || !reflect.DeepEqual(again, project) {
		t.Fatalf("reordered ResolveTeamDefinition() = (%#v,%v), want %#v", again, err, project)
	}

	tampered := cloneTeamDefinition(projectV1)
	tampered.digest = "tampered"
	nonCanonical := cloneTeamDefinition(projectV1)
	nonCanonical.roles[0], nonCanonical.roles[2] = nonCanonical.roles[2], nonCanonical.roles[0]
	nonCanonical.digest, _ = digestTeamDefinition(nonCanonical)
	unrelated := cloneTeamDefinition(projectV2)
	unrelated.id = "team.unrelated"
	unrelated.digest, _ = digestTeamDefinition(unrelated)
	withNonWinners := append(append([]TeamDefinition(nil), candidates...), archived, unrelated, unrelated)
	unblocked, err := ResolveTeamDefinition(withNonWinners, "team.delivery", agents.ScopeIdentity{ProjectID: "project.one"})
	if err != nil || !reflect.DeepEqual(unblocked, project) {
		t.Fatalf("non-winning duplicates blocked resolution: (%#v,%v)", unblocked, err)
	}
	tests := []struct {
		name       string
		candidates []TeamDefinition
		id         string
		context    agents.ScopeIdentity
		want       error
	}{
		{name: "not found", candidates: candidates, id: "team.missing", context: agents.ScopeIdentity{ProjectID: "project.one"}, want: ErrTeamDefinitionNotFound},
		{name: "empty id", candidates: candidates, context: agents.ScopeIdentity{ProjectID: "project.one"}, want: ErrTeamDefinitionNotFound},
		{name: "invalid context", candidates: candidates, id: "team.delivery", context: agents.ScopeIdentity{GenerationID: "generation.forbidden"}, want: ErrInvalidTeamDefinition},
		{name: "duplicate winner", candidates: append(candidates, projectV2), id: "team.delivery", context: agents.ScopeIdentity{ProjectID: "project.one"}, want: ErrDuplicateTeamDefinition},
		{name: "invalid candidate", candidates: append(candidates, tampered), id: "team.delivery", context: agents.ScopeIdentity{ProjectID: "project.one"}, want: ErrTeamDefinitionDigestMismatch},
		{name: "noncanonical candidate", candidates: append(candidates, nonCanonical), id: "team.delivery", context: agents.ScopeIdentity{ProjectID: "project.one"}, want: ErrInvalidTeamDefinitionRole},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTeamDefinition(tt.candidates, tt.id, tt.context)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ResolveTeamDefinition() error = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(got, TeamDefinitionLoadCandidate{}) {
				t.Fatalf("failed resolution returned Candidate %#v", got)
			}
		})
	}
}

func TestValidateTeamDefinitionFailures(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	valid := mustBuildTeamDefinition(t, input, definitions, profiles)
	tamperedRole := cloneTeamDefinition(valid)
	tamperedRole.roles[0].Responsibility = "tampered"
	tamperedDigest := cloneTeamDefinition(valid)
	tamperedDigest.digest = "tampered"
	missingDefinitions := definitions[1:]
	missingProfiles := profiles[1:]

	tests := []struct {
		name        string
		current     TeamDefinition
		definitions []agents.AgentDefinition
		profiles    []loomruntime.RuntimeProfile
		want        error
	}{
		{name: "zero", definitions: definitions, profiles: profiles, want: ErrInvalidTeamDefinition},
		{name: "role tamper", current: tamperedRole, definitions: definitions, profiles: profiles, want: ErrTeamDefinitionDigestMismatch},
		{name: "digest tamper", current: tamperedDigest, definitions: definitions, profiles: profiles, want: ErrTeamDefinitionDigestMismatch},
		{name: "definition removed", current: valid, definitions: missingDefinitions, profiles: profiles, want: ErrInventedTeamDefinitionReference},
		{name: "profile removed", current: valid, definitions: definitions, profiles: missingProfiles, want: ErrInventedTeamDefinitionReference},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTeamDefinition(tt.current, tt.definitions, tt.profiles)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateTeamDefinition() error = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(got, TeamDefinitionValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", got)
			}
		})
	}
}

func TestTeamDefinitionDigestDeterminismAndSensitivity(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	base := mustBuildTeamDefinition(t, input, definitions, profiles)
	reordered := input
	reordered.Roles = []TeamDefinitionRole{input.Roles[2], input.Roles[1], input.Roles[0]}
	if got := mustBuildTeamDefinition(t, reordered, definitions, profiles); got.Digest() != base.Digest() {
		t.Fatal("semantic reorder changed digest")
	}

	mutations := []struct {
		name   string
		mutate func(*TeamDefinition)
	}{
		{name: "id", mutate: func(d *TeamDefinition) { d.id = "team.changed" }},
		{name: "version", mutate: func(d *TeamDefinition) { d.version++ }},
		{name: "scope", mutate: func(d *TeamDefinition) { d.scope = TeamDefinitionScopeReusable }},
		{name: "scope identity", mutate: func(d *TeamDefinition) { d.scopeIdentity.ProjectID = "project.changed" }},
		{name: "name", mutate: func(d *TeamDefinition) { d.name = "Changed" }},
		{name: "status", mutate: func(d *TeamDefinition) { d.status = TeamDefinitionArchived }},
		{name: "agent binding", mutate: func(d *TeamDefinition) { d.roles[1].AgentDefinitionID = "agent.changed" }},
		{name: "profile binding", mutate: func(d *TeamDefinition) { d.roles[1].RuntimeProfileID = "profile.changed" }},
		{name: "role kind", mutate: func(d *TeamDefinition) { d.roles[1].Kind = TeamDefinitionRoleMain }},
		{name: "responsibility", mutate: func(d *TeamDefinition) { d.roles[1].Responsibility = "changed" }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneTeamDefinition(base)
			tt.mutate(&changed)
			digest, err := digestTeamDefinition(changed)
			if err != nil {
				t.Fatalf("digestTeamDefinition() error = %v", err)
			}
			if digest == base.Digest() {
				t.Fatal("semantic field change did not change digest")
			}
		})
	}
}

func TestTeamDefinitionImportBoundary(t *testing.T) {
	source, err := os.ReadFile("team_definition.go")
	if err != nil {
		t.Fatalf("ReadFile(team_definition.go): %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "team_definition.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(team_definition.go): %v", err)
	}
	allowed := map[string]bool{
		`"crypto/sha256"`:                    true,
		`"encoding/hex"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"fmt"`:                              true,
		`"sort"`:                             true,
		`"loom-pi-rebuild/internal/agents"`:  true,
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("team_definition.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func teamDefinitionFixture() ([]agents.AgentDefinition, []loomruntime.RuntimeProfile, TeamDefinitionInput) {
	definitions := []agents.AgentDefinition{
		projectAgent("agent.main"),
		projectAgent("agent.sub.one"),
		projectAgent("agent.sub.two"),
	}
	profiles := []loomruntime.RuntimeProfile{
		runtimeProfile("profile.main"),
		runtimeProfile("profile.sub.one"),
		runtimeProfile("profile.sub.two"),
	}
	input := TeamDefinitionInput{
		ID:            "team.delivery",
		Version:       1,
		Scope:         TeamDefinitionScopeProject,
		ScopeIdentity: agents.ScopeIdentity{ProjectID: "project.one"},
		Name:          "Delivery Team",
		Status:        TeamDefinitionActive,
		Roles: []TeamDefinitionRole{
			{Kind: TeamDefinitionRoleMain, AgentDefinitionID: "agent.main", RuntimeProfileID: "profile.main", Responsibility: "coordinate"},
			{Kind: TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent.sub.one", RuntimeProfileID: "profile.sub.one", Responsibility: "implement"},
			{Kind: TeamDefinitionRoleSubAgent, AgentDefinitionID: "agent.sub.two", RuntimeProfileID: "profile.sub.two", Responsibility: "review"},
		},
	}
	return definitions, profiles, input
}

func projectAgent(id string) agents.AgentDefinition {
	return agents.AgentDefinition{
		ID: id, Version: 1, Scope: agents.ScopeProject,
		ScopeIdentity: agents.ScopeIdentity{ProjectID: "project.one"},
		Name:          id, RoleSpec: "bounded role", Status: agents.DefinitionActive,
	}
}

func reusableAgent(id string) agents.AgentDefinition {
	return agents.AgentDefinition{
		ID: id, Version: 1, Scope: agents.ScopeReusable,
		Name: id, RoleSpec: "bounded role", Status: agents.DefinitionActive,
	}
}

func runtimeProfile(id string) loomruntime.RuntimeProfile {
	budget := int64(100)
	return loomruntime.RuntimeProfile{
		ID: id, AdapterType: "test", ProviderID: "provider.test", ModelID: "model.test",
		AuthMode: loomruntime.AuthBrokered, RequiredCapabilities: []string{"text"},
		Timeout: time.Minute, Budget: &budget,
	}
}

func withTeamDefinitionInput(input TeamDefinitionInput, mutate func(*TeamDefinitionInput)) TeamDefinitionInput {
	input.Roles = append([]TeamDefinitionRole(nil), input.Roles...)
	mutate(&input)
	return input
}

func mustBuildTeamDefinition(
	t *testing.T,
	input TeamDefinitionInput,
	definitions []agents.AgentDefinition,
	profiles []loomruntime.RuntimeProfile,
) TeamDefinition {
	t.Helper()
	got, err := BuildTeamDefinition(input, definitions, profiles)
	if err != nil {
		t.Fatalf("BuildTeamDefinition() error = %v", err)
	}
	return got
}

func assertTeamDefinitionIdentity(
	t *testing.T,
	got TeamDefinition,
	id string,
	version int,
	scope TeamDefinitionScope,
	projectID string,
) {
	t.Helper()
	if got.ID() != id || got.Version() != version || got.Scope() != scope ||
		got.ScopeIdentity().ProjectID != projectID {
		t.Fatalf("identity = (%q,%d,%q,%#v)", got.ID(), got.Version(), got.Scope(), got.ScopeIdentity())
	}
}

func assertLoadCandidate(
	t *testing.T,
	got TeamDefinitionLoadCandidate,
	scope TeamDefinitionScope,
	projectID string,
	version int,
	digest string,
) {
	t.Helper()
	if !got.Found || got.Scope != scope || got.ScopeIdentity.ProjectID != projectID ||
		got.Version != version || got.DefinitionDigest != digest ||
		got.MainAgentDefinitionID != "agent.main" || len(got.Roles) != 3 {
		t.Fatalf("load Candidate = %#v", got)
	}
}

func assertZeroTeamDefinition(t *testing.T, got TeamDefinition) {
	t.Helper()
	if got.ID() != "" || got.Version() != 0 || got.Scope() != "" ||
		got.ScopeIdentity() != (agents.ScopeIdentity{}) || got.Name() != "" ||
		got.Status() != "" || got.Digest() != "" || got.Roles() != nil ||
		got.MainAgentDefinitionID() != "" || len(got.SubAgentDefinitionIDs()) != 0 {
		t.Fatalf("failed build returned usable TeamDefinition %#v", got)
	}
}
