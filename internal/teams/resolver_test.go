package teams

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/mode"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestResolveAgentModeTeamRouterGateAndSelectedTeam(t *testing.T) {
	context, catalog, projectTeam, _ := teamResolverFixture(t)

	for _, intent := range []mode.Intent{
		{Mode: mode.ModeAgent, Trigger: mode.TriggerPlainInput, Text: "force agent"},
		{Mode: mode.ModeAgent, Trigger: mode.Trigger(""), Text: "force agent"},
		{Mode: mode.ModeAgent, Trigger: mode.Trigger("unknown"), Text: "force agent"},
	} {
		got, err := ResolveAgentModeTeam(intent, context, catalog)
		if !errors.Is(err, ErrTeamResolutionRequiresAgentMode) {
			t.Fatalf("ResolveAgentModeTeam(%#v) error = %v", intent, err)
		}
		assertZeroTeamResolutionCandidate(t, got)
	}

	intent := mode.Intent{
		Mode: mode.ModeConversation, Trigger: mode.TriggerSelectTeam,
		TargetID: "team.delivery", Text: "private text one",
	}
	got, err := ResolveAgentModeTeam(intent, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(select team) error = %v", err)
	}
	assertResolutionCandidate(t, got, TeamResolutionLoadTeam, "team.delivery", "agent.main", "", false)
	if got.Team.DefinitionDigest != projectTeam.Digest() ||
		got.Team.Scope != TeamDefinitionScopeProject {
		t.Fatalf("selected Team = %#v", got.Team)
	}
	assertSHA256Digest(t, got.Digest)

	changedText := intent
	changedText.Text = "private text two"
	again, err := ResolveAgentModeTeam(changedText, context, catalog)
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Fatalf("text-only change affected resolution: (%#v,%v)", again, err)
	}
}

func TestResolveAgentModeTeamSelectedAgent(t *testing.T) {
	context, catalog, _, _ := teamResolverFixture(t)

	main, err := ResolveAgentModeTeam(mode.Intent{
		Trigger: mode.TriggerSelectAgent, TargetID: "agent.main",
	}, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(main) error = %v", err)
	}
	assertResolutionCandidate(t, main, TeamResolutionDirectMain, "agent.main", "agent.main", "", false)

	sub, err := ResolveAgentModeTeam(mode.Intent{
		Trigger: mode.TriggerSelectAgent, TargetID: "agent.sub.one",
	}, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(subagent) error = %v", err)
	}
	assertResolutionCandidate(t, sub, TeamResolutionDraftSeed, "agent.sub.one", "agent.main", "agent.sub.one", true)
	if !reflect.DeepEqual(sub.Team, TeamDefinitionLoadCandidate{}) {
		t.Fatalf("draft seed exposed Team load %#v", sub.Team)
	}
}

func TestResolveAgentModeTeamUseAgentDefaults(t *testing.T) {
	context, catalog, projectTeam, reusableTeam := teamResolverFixture(t)
	intent := mode.Intent{Trigger: mode.TriggerUseAgent}

	project, err := ResolveAgentModeTeam(intent, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(project default) error = %v", err)
	}
	assertResolutionCandidate(t, project, TeamResolutionLoadTeam, "", "agent.main", "", false)
	if project.Team.DefinitionDigest != projectTeam.Digest() {
		t.Fatalf("project default digest = %q", project.Team.DefinitionDigest)
	}

	reusableCatalog := cloneTeamResolutionCatalogInput(catalog)
	reusableCatalog.ProjectDefaultTeamID = ""
	reusable, err := ResolveAgentModeTeam(intent, context, reusableCatalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(reusable default) error = %v", err)
	}
	if reusable.Team.DefinitionDigest != reusableTeam.Digest() ||
		reusable.Team.Scope != TeamDefinitionScopeReusable {
		t.Fatalf("reusable default = %#v", reusable.Team)
	}

	seedCatalog := cloneTeamResolutionCatalogInput(catalog)
	seedCatalog.ProjectDefaultTeamID = ""
	seedCatalog.ReusableDefaultTeamID = ""
	seed, err := ResolveAgentModeTeam(intent, context, seedCatalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(default main seed) error = %v", err)
	}
	assertResolutionCandidate(t, seed, TeamResolutionDraftSeed, "", "agent.main", "", true)
}

func TestResolveAgentModeTeamAssign(t *testing.T) {
	context, catalog, _, _ := teamResolverFixture(t)

	team, err := ResolveAgentModeTeam(mode.Intent{
		Trigger: mode.TriggerAssign, TargetID: "team.delivery",
	}, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(assign team) error = %v", err)
	}
	assertResolutionCandidate(t, team, TeamResolutionLoadTeam, "team.delivery", "agent.main", "", false)

	agent, err := ResolveAgentModeTeam(mode.Intent{
		Trigger: mode.TriggerAssign, TargetID: "agent.sub.one",
	}, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam(assign agent) error = %v", err)
	}
	assertResolutionCandidate(t, agent, TeamResolutionDraftSeed, "agent.sub.one", "agent.main", "agent.sub.one", true)

	missing, err := ResolveAgentModeTeam(mode.Intent{
		Trigger: mode.TriggerAssign, TargetID: "missing",
	}, context, catalog)
	if !errors.Is(err, ErrTeamResolutionTargetNotFound) {
		t.Fatalf("ResolveAgentModeTeam(assign missing) error = %v", err)
	}
	assertZeroTeamResolutionCandidate(t, missing)

	ambiguousCatalog := cloneTeamResolutionCatalogInput(catalog)
	ambiguousInput := teamResolverProjectInput()
	ambiguousInput.ID = "agent.sub.one"
	ambiguousTeam := mustBuildTeamDefinition(t, ambiguousInput, ambiguousCatalog.AgentDefinitions, ambiguousCatalog.RuntimeProfiles)
	ambiguousCatalog.TeamDefinitions = append(ambiguousCatalog.TeamDefinitions, ambiguousTeam)
	ambiguous, err := ResolveAgentModeTeam(mode.Intent{
		Trigger: mode.TriggerAssign, TargetID: "agent.sub.one",
	}, context, ambiguousCatalog)
	if !errors.Is(err, ErrAmbiguousTeamResolutionTarget) {
		t.Fatalf("ResolveAgentModeTeam(assign ambiguous) error = %v", err)
	}
	assertZeroTeamResolutionCandidate(t, ambiguous)
}

func TestResolveAgentModeTeamFailures(t *testing.T) {
	context, catalog, _, _ := teamResolverFixture(t)
	invalidAgent := catalog.AgentDefinitions[0]
	invalidAgent.ID = ""
	invalidProfile := catalog.RuntimeProfiles[0]
	invalidProfile.ID = ""
	tamperedTeam := cloneTeamDefinition(catalog.TeamDefinitions[0])
	tamperedTeam.digest = "tampered"

	tests := []struct {
		name    string
		intent  mode.Intent
		context agents.ScopeIdentity
		mutate  func(*TeamResolutionCatalogInput)
		want    error
	}{
		{name: "invalid context", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: agents.ScopeIdentity{ProjectID: "project.one", GenerationID: "forbidden"}, want: ErrInvalidTeamResolutionCatalog},
		{name: "select team empty target", intent: mode.Intent{Trigger: mode.TriggerSelectTeam}, context: context, want: ErrInvalidTeamResolutionTarget},
		{name: "select team not found", intent: mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.missing"}, context: context, want: ErrTeamResolutionTeamNotFound},
		{name: "select agent empty target", intent: mode.Intent{Trigger: mode.TriggerSelectAgent}, context: context, want: ErrInvalidTeamResolutionTarget},
		{name: "select agent not found", intent: mode.Intent{Trigger: mode.TriggerSelectAgent, TargetID: "agent.missing"}, context: context, want: ErrTeamResolutionAgentNotFound},
		{name: "use agent target", intent: mode.Intent{Trigger: mode.TriggerUseAgent, TargetID: "unexpected"}, context: context, want: ErrInvalidTeamResolutionTarget},
		{name: "missing default main", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) { c.DefaultMainAgentID = "" }, want: ErrInvalidTeamResolutionMainCatalog},
		{name: "default omitted from main ids", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) { c.MainAgentDefinitionIDs = []string{"agent.sub.one"} }, want: ErrInvalidTeamResolutionMainCatalog},
		{name: "duplicate main id", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) { c.MainAgentDefinitionIDs = []string{"agent.main", "agent.main"} }, want: ErrInvalidTeamResolutionMainCatalog},
		{name: "unresolved main", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) {
			c.MainAgentDefinitionIDs = []string{"agent.main", "agent.missing"}
		}, want: ErrInvalidTeamResolutionMainCatalog},
		{name: "invalid agent", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) {
			c.AgentDefinitions = append(c.AgentDefinitions, invalidAgent)
		}, want: ErrInvalidTeamResolutionCatalog},
		{name: "duplicate agent", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) {
			c.AgentDefinitions = append(c.AgentDefinitions, c.AgentDefinitions[0])
		}, want: ErrInvalidTeamResolutionCatalog},
		{name: "invalid profile", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) { c.RuntimeProfiles = append(c.RuntimeProfiles, invalidProfile) }, want: ErrInvalidTeamResolutionCatalog},
		{name: "duplicate profile", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) {
			c.RuntimeProfiles = append(c.RuntimeProfiles, c.RuntimeProfiles[0])
		}, want: ErrInvalidTeamResolutionCatalog},
		{name: "project default has reusable scope", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) {
			input := teamResolverProjectInput()
			input.ID = "team.reusable.only"
			input.Scope = TeamDefinitionScopeReusable
			input.ScopeIdentity = agents.ScopeIdentity{}
			c.TeamDefinitions = append(c.TeamDefinitions, mustBuildTeamDefinition(t, input, c.AgentDefinitions, c.RuntimeProfiles))
			c.ProjectDefaultTeamID = input.ID
		}, want: ErrInvalidTeamResolutionDefaultTeam},
		{name: "reusable default has project scope", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) {
			input := teamResolverProjectInput()
			input.ID = "team.project.only"
			c.TeamDefinitions = append(c.TeamDefinitions, mustBuildTeamDefinition(t, input, c.AgentDefinitions, c.RuntimeProfiles))
			c.ReusableDefaultTeamID = input.ID
		}, want: ErrInvalidTeamResolutionDefaultTeam},
		{name: "invalid team", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, context: context, mutate: func(c *TeamResolutionCatalogInput) { c.TeamDefinitions = append(c.TeamDefinitions, tamperedTeam) }, want: ErrTeamDefinitionDigestMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := cloneTeamResolutionCatalogInput(catalog)
			if tt.mutate != nil {
				tt.mutate(&current)
			}
			got, err := ResolveAgentModeTeam(tt.intent, tt.context, current)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ResolveAgentModeTeam() error = %v, want %v", err, tt.want)
			}
			assertZeroTeamResolutionCandidate(t, got)
		})
	}
}

func TestResolveAgentModeTeamDeterminismAndIsolation(t *testing.T) {
	context, catalog, _, _ := teamResolverFixture(t)
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"}
	first, err := ResolveAgentModeTeam(intent, context, catalog)
	if err != nil {
		t.Fatalf("ResolveAgentModeTeam() error = %v", err)
	}
	reordered := cloneTeamResolutionCatalogInput(catalog)
	reverseAgentDefinitions(reordered.AgentDefinitions)
	reverseRuntimeProfiles(reordered.RuntimeProfiles)
	reverseTeamDefinitions(reordered.TeamDefinitions)
	reordered.MainAgentDefinitionIDs = []string{"agent.main"}
	second, err := ResolveAgentModeTeam(intent, context, reordered)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("reordered resolution = (%#v,%v), want %#v", second, err, first)
	}

	pristine := cloneTeamResolutionCandidate(first)
	catalog.AgentDefinitions[0].ID = "mutated"
	catalog.RuntimeProfiles[0].RequiredCapabilities[0] = "mutated"
	returnedRoles := first.Team.Roles
	returnedRoles[0].Responsibility = "mutated"
	third, err := ResolveAgentModeTeam(intent, context, reordered)
	if err != nil || !reflect.DeepEqual(third, pristine) {
		t.Fatalf("input/result mutation changed later resolution: (%#v,%v)", third, err)
	}

	mutations := []struct {
		name   string
		mutate func(*TeamResolutionCandidate)
	}{
		{name: "trigger", mutate: func(c *TeamResolutionCandidate) { c.Trigger = mode.TriggerAssign }},
		{name: "target", mutate: func(c *TeamResolutionCandidate) { c.TargetID = "changed" }},
		{name: "kind", mutate: func(c *TeamResolutionCandidate) { c.Kind = TeamResolutionDirectMain }},
		{name: "team", mutate: func(c *TeamResolutionCandidate) { c.Team.DefinitionDigest = "changed" }},
		{name: "main", mutate: func(c *TeamResolutionCandidate) { c.MainAgentDefinitionID = "changed" }},
		{name: "subagent", mutate: func(c *TeamResolutionCandidate) { c.SelectedSubAgentDefinitionID = "changed" }},
		{name: "requires draft", mutate: func(c *TeamResolutionCandidate) { c.RequiresDraft = true }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneTeamResolutionCandidate(pristine)
			tt.mutate(&changed)
			digest, err := digestTeamResolutionCandidate(changed)
			if err != nil {
				t.Fatalf("digestTeamResolutionCandidate() error = %v", err)
			}
			if digest == pristine.Digest {
				t.Fatal("semantic change did not change resolution digest")
			}
		})
	}
}

func TestResolveAgentModeTeamImportBoundary(t *testing.T) {
	source, err := os.ReadFile("resolver.go")
	if err != nil {
		t.Fatalf("ReadFile(resolver.go): %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "resolver.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(resolver.go): %v", err)
	}
	allowed := map[string]bool{
		`"crypto/sha256"`:                    true,
		`"encoding/hex"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"fmt"`:                              true,
		`"loom-pi-rebuild/internal/agents"`:  true,
		`"loom-pi-rebuild/internal/mode"`:    true,
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("resolver.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func teamResolverFixture(t *testing.T) (
	agents.ScopeIdentity,
	TeamResolutionCatalogInput,
	TeamDefinition,
	TeamDefinition,
) {
	t.Helper()
	projectDefinitions, profiles, projectInput := teamDefinitionFixture()
	reusableDefinitions := []agents.AgentDefinition{
		reusableAgent("agent.main"),
		reusableAgent("agent.sub.one"),
		reusableAgent("agent.sub.two"),
	}
	definitions := append(append([]agents.AgentDefinition(nil), projectDefinitions...), reusableDefinitions...)
	projectTeam := mustBuildTeamDefinition(t, projectInput, definitions, profiles)
	reusableInput := projectInput
	reusableInput.Scope = TeamDefinitionScopeReusable
	reusableInput.ScopeIdentity = agents.ScopeIdentity{}
	reusableTeam := mustBuildTeamDefinition(t, reusableInput, definitions, profiles)
	return agents.ScopeIdentity{ProjectID: "project.one"}, TeamResolutionCatalogInput{
		AgentDefinitions:       definitions,
		RuntimeProfiles:        profiles,
		TeamDefinitions:        []TeamDefinition{reusableTeam, projectTeam},
		MainAgentDefinitionIDs: []string{"agent.main"},
		DefaultMainAgentID:     "agent.main",
		ProjectDefaultTeamID:   "team.delivery",
		ReusableDefaultTeamID:  "team.delivery",
	}, projectTeam, reusableTeam
}

func teamResolverProjectInput() TeamDefinitionInput {
	_, _, input := teamDefinitionFixture()
	return input
}

func cloneTeamResolutionCatalogInput(input TeamResolutionCatalogInput) TeamResolutionCatalogInput {
	input.AgentDefinitions = append([]agents.AgentDefinition(nil), input.AgentDefinitions...)
	input.RuntimeProfiles = append([]loomruntime.RuntimeProfile(nil), input.RuntimeProfiles...)
	for index := range input.RuntimeProfiles {
		input.RuntimeProfiles[index].RequiredCapabilities = append(
			[]string(nil), input.RuntimeProfiles[index].RequiredCapabilities...,
		)
		if input.RuntimeProfiles[index].Budget != nil {
			budget := *input.RuntimeProfiles[index].Budget
			input.RuntimeProfiles[index].Budget = &budget
		}
	}
	input.TeamDefinitions = append([]TeamDefinition(nil), input.TeamDefinitions...)
	input.MainAgentDefinitionIDs = append([]string(nil), input.MainAgentDefinitionIDs...)
	return input
}

func reverseAgentDefinitions(input []agents.AgentDefinition) {
	for left, right := 0, len(input)-1; left < right; left, right = left+1, right-1 {
		input[left], input[right] = input[right], input[left]
	}
}

func reverseRuntimeProfiles(input []loomruntime.RuntimeProfile) {
	for left, right := 0, len(input)-1; left < right; left, right = left+1, right-1 {
		input[left], input[right] = input[right], input[left]
	}
}

func reverseTeamDefinitions(input []TeamDefinition) {
	for left, right := 0, len(input)-1; left < right; left, right = left+1, right-1 {
		input[left], input[right] = input[right], input[left]
	}
}

func assertResolutionCandidate(
	t *testing.T,
	got TeamResolutionCandidate,
	kind TeamResolutionKind,
	targetID, mainID, subAgentID string,
	requiresDraft bool,
) {
	t.Helper()
	if !got.Resolved || got.Mode != mode.ModeAgent || got.Kind != kind ||
		got.TargetID != targetID || got.MainAgentDefinitionID != mainID ||
		got.SelectedSubAgentDefinitionID != subAgentID ||
		got.RequiresDraft != requiresDraft {
		t.Fatalf("resolution Candidate = %#v", got)
	}
}

func assertZeroTeamResolutionCandidate(t *testing.T, got TeamResolutionCandidate) {
	t.Helper()
	if !reflect.DeepEqual(got, TeamResolutionCandidate{}) {
		t.Fatalf("failed resolution returned Candidate %#v", got)
	}
}
