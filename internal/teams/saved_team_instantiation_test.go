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

func TestBuildSavedTeamInstantiationPlan(t *testing.T) {
	context, catalog, discovery, selections, binding := savedTeamInstantiationFixture(t)

	for _, intent := range []mode.Intent{
		{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery", Text: "private one"},
		{Trigger: mode.TriggerUseAgent, Text: "private two"},
		{Trigger: mode.TriggerAssign, TargetID: "team.delivery", Text: "private three"},
	} {
		t.Run(string(intent.Trigger), func(t *testing.T) {
			got, err := BuildSavedTeamInstantiationPlan(
				intent, context, catalog, binding, discovery, selections,
			)
			if err != nil {
				t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
			}
			if !got.Ready() || got.Trigger() != intent.Trigger ||
				got.TargetID() != intent.TargetID ||
				got.TeamDefinitionID() != "team.delivery" ||
				got.TeamDefinitionVersion() != 1 ||
				got.TeamDefinitionDigest() != binding.TeamDefinitionDigest() ||
				got.RuntimeDiscoveryDigest() != binding.RuntimeDiscoveryDigest() ||
				got.BindingDigest() != binding.BindingDigest() ||
				!got.CreateTeamInstance() || !got.CreateMainAgentInstance() ||
				got.CreateSubAgentInstances() || got.RequiresDraft() ||
				got.WorkItemCount() != 0 {
				t.Fatalf("instantiation plan = %#v", got)
			}
			main := got.MainSeed()
			if main.AgentDefinitionID != "agent.main" ||
				main.RuntimeProfileID != "profile.main" ||
				main.RuntimeInstanceID != "runtime.shared" ||
				!main.Binding.Accepted {
				t.Fatalf("MainSeed() = %#v", main)
			}
			dormant := got.DormantSubAgents()
			if dormantAgentIDs(dormant) != "agent.sub.one,agent.sub.two" {
				t.Fatalf("DormantSubAgents() = %#v", dormant)
			}
			for _, seed := range dormant {
				if !seed.Dormant || seed.AgentDefinitionID == "" ||
					seed.RuntimeProfileID == "" || seed.RuntimeInstanceID == "" {
					t.Fatalf("invalid dormant seed %#v", seed)
				}
			}
			assertSHA256Digest(t, got.PlanDigest())

			validated, err := ValidateSavedTeamInstantiationPlan(
				got, intent, context, catalog, binding, discovery, selections,
			)
			if err != nil {
				t.Fatalf("ValidateSavedTeamInstantiationPlan() error = %v", err)
			}
			if !validated.Valid ||
				validated.TeamDefinitionID != "team.delivery" ||
				validated.TeamDefinitionDigest != got.TeamDefinitionDigest() ||
				validated.ResolutionDigest != got.ResolutionDigest() ||
				validated.BindingDigest != got.BindingDigest() ||
				validated.PlanDigest != got.PlanDigest() ||
				validated.MainAgentDefinitionID != "agent.main" ||
				validated.DormantSubAgentCount != 2 ||
				validated.WorkItemCount != 0 {
				t.Fatalf("validation Candidate = %#v", validated)
			}
		})
	}
}

func TestBuildSavedTeamInstantiationPlanCardinality(t *testing.T) {
	definitions, profiles, baseInput := teamDefinitionFixture()
	for roleCount := 1; roleCount <= 3; roleCount++ {
		input := baseInput
		input.ID = "team.cardinality"
		input.Roles = append([]TeamDefinitionRole(nil), baseInput.Roles[:roleCount]...)
		team := mustBuildTeamDefinition(t, input, definitions, profiles)
		catalog := TeamResolutionCatalogInput{
			AgentDefinitions:       definitions,
			RuntimeProfiles:        profiles,
			TeamDefinitions:        []TeamDefinition{team},
			MainAgentDefinitionIDs: []string{"agent.main"},
			DefaultMainAgentID:     "agent.main",
			ProjectDefaultTeamID:   input.ID,
		}
		discovery := savedTeamBindingDiscovery(t, "online", roleCount, []string{"model.test"})
		selections := []SavedTeamRuntimeSelection{
			{AgentDefinitionID: "agent.main", RuntimeInstanceID: "runtime.shared"},
			{AgentDefinitionID: "agent.sub.one", RuntimeInstanceID: "runtime.shared"},
			{AgentDefinitionID: "agent.sub.two", RuntimeInstanceID: "runtime.shared"},
		}[:roleCount]
		binding, err := BuildSavedTeamRuntimeBinding(
			[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
			definitions, profiles, discovery, selections,
		)
		if err != nil {
			t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
		}
		got, err := BuildSavedTeamInstantiationPlan(
			mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: input.ID},
			input.ScopeIdentity, catalog, binding, discovery, selections,
		)
		if err != nil {
			t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
		}
		if len(got.DormantSubAgents()) != roleCount-1 ||
			got.CreateSubAgentInstances() || got.WorkItemCount() != 0 {
			t.Fatalf("role-count %d plan = %#v", roleCount, got)
		}
	}
}

func TestBuildSavedTeamInstantiationPlanReusableDefault(t *testing.T) {
	context, catalog, _, reusableTeam := teamResolverFixture(t)
	catalog.ProjectDefaultTeamID = ""
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	selections := savedTeamBindingSelections()
	binding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, reusableTeam.ID(), reusableTeam.ScopeIdentity(),
		catalog.AgentDefinitions, catalog.RuntimeProfiles,
		discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	got, err := BuildSavedTeamInstantiationPlan(
		mode.Intent{Trigger: mode.TriggerUseAgent},
		context, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	if got.TeamDefinitionScope() != TeamDefinitionScopeReusable ||
		got.ScopeIdentity() != (agents.ScopeIdentity{}) ||
		got.TeamDefinitionDigest() != reusableTeam.Digest() {
		t.Fatalf("reusable default plan = %#v", got)
	}
}

func TestBuildSavedTeamInstantiationPlanRejectsNonLoadAndMismatch(t *testing.T) {
	context, catalog, discovery, selections, binding := savedTeamInstantiationFixture(t)
	seedCatalog := cloneTeamResolutionCatalogInput(catalog)
	seedCatalog.ProjectDefaultTeamID = ""
	seedCatalog.ReusableDefaultTeamID = ""

	tests := []struct {
		name    string
		intent  mode.Intent
		catalog TeamResolutionCatalogInput
		want    error
	}{
		{name: "plain", intent: mode.Intent{Mode: mode.ModeAgent, Trigger: mode.TriggerPlainInput}, catalog: catalog, want: ErrTeamResolutionRequiresAgentMode},
		{name: "selected main", intent: mode.Intent{Trigger: mode.TriggerSelectAgent, TargetID: "agent.main"}, catalog: catalog, want: ErrSavedTeamInstantiationRequiresLoadTeam},
		{name: "selected subagent", intent: mode.Intent{Trigger: mode.TriggerSelectAgent, TargetID: "agent.sub.one"}, catalog: catalog, want: ErrSavedTeamInstantiationRequiresLoadTeam},
		{name: "default main seed", intent: mode.Intent{Trigger: mode.TriggerUseAgent}, catalog: seedCatalog, want: ErrSavedTeamInstantiationRequiresLoadTeam},
		{name: "agent assign", intent: mode.Intent{Trigger: mode.TriggerAssign, TargetID: "agent.sub.one"}, catalog: catalog, want: ErrSavedTeamInstantiationRequiresLoadTeam},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildSavedTeamInstantiationPlan(
				tt.intent, context, tt.catalog, binding, discovery, selections,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v, want %v", err, tt.want)
			}
			assertZeroSavedTeamInstantiationPlan(t, got)
		})
	}

	for _, tt := range []struct {
		name   string
		intent mode.Intent
		want   error
	}{
		{name: "empty trigger", intent: mode.Intent{}, want: ErrTeamResolutionRequiresAgentMode},
		{name: "unknown trigger", intent: mode.Intent{Trigger: mode.Trigger("unknown")}, want: ErrTeamResolutionRequiresAgentMode},
		{name: "empty team target", intent: mode.Intent{Trigger: mode.TriggerSelectTeam}, want: ErrInvalidTeamResolutionTarget},
		{name: "missing team", intent: mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.missing"}, want: ErrTeamResolutionTeamNotFound},
		{name: "empty agent target", intent: mode.Intent{Trigger: mode.TriggerSelectAgent}, want: ErrInvalidTeamResolutionTarget},
		{name: "missing agent", intent: mode.Intent{Trigger: mode.TriggerSelectAgent, TargetID: "agent.missing"}, want: ErrTeamResolutionAgentNotFound},
		{name: "use agent target", intent: mode.Intent{Trigger: mode.TriggerUseAgent, TargetID: "unexpected"}, want: ErrInvalidTeamResolutionTarget},
		{name: "empty assign target", intent: mode.Intent{Trigger: mode.TriggerAssign}, want: ErrInvalidTeamResolutionTarget},
		{name: "missing assign target", intent: mode.Intent{Trigger: mode.TriggerAssign, TargetID: "missing"}, want: ErrTeamResolutionTargetNotFound},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildSavedTeamInstantiationPlan(
				tt.intent, context, catalog, binding, discovery, selections,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v, want %v", err, tt.want)
			}
			assertZeroSavedTeamInstantiationPlan(t, got)
		})
	}

	otherCatalog := cloneTeamResolutionCatalogInput(catalog)
	otherInput := teamResolverProjectInput()
	otherInput.ID = "team.other"
	otherTeam := mustBuildTeamDefinition(t, otherInput, otherCatalog.AgentDefinitions, otherCatalog.RuntimeProfiles)
	otherCatalog.TeamDefinitions = append(otherCatalog.TeamDefinitions, otherTeam)
	otherSelections := append([]SavedTeamRuntimeSelection(nil), selections...)
	otherBinding, err := BuildSavedTeamRuntimeBinding(
		otherCatalog.TeamDefinitions, otherInput.ID, context,
		otherCatalog.AgentDefinitions, otherCatalog.RuntimeProfiles,
		discovery, otherSelections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding(other) error = %v", err)
	}
	got, err := BuildSavedTeamInstantiationPlan(
		mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"},
		context, otherCatalog, otherBinding, discovery, otherSelections,
	)
	if !errors.Is(err, ErrSavedTeamInstantiationSourceMismatch) {
		t.Fatalf("mismatched binding error = %v", err)
	}
	assertZeroSavedTeamInstantiationPlan(t, got)

	ambiguousCatalog := cloneTeamResolutionCatalogInput(catalog)
	ambiguousInput := teamResolverProjectInput()
	ambiguousInput.ID = "agent.sub.one"
	ambiguousCatalog.TeamDefinitions = append(
		ambiguousCatalog.TeamDefinitions,
		mustBuildTeamDefinition(
			t, ambiguousInput,
			ambiguousCatalog.AgentDefinitions, ambiguousCatalog.RuntimeProfiles,
		),
	)
	got, err = BuildSavedTeamInstantiationPlan(
		mode.Intent{Trigger: mode.TriggerAssign, TargetID: "agent.sub.one"},
		context, ambiguousCatalog, binding, discovery, selections,
	)
	if !errors.Is(err, ErrAmbiguousTeamResolutionTarget) {
		t.Fatalf("ambiguous route error = %v", err)
	}
	assertZeroSavedTeamInstantiationPlan(t, got)
}

func TestBuildSavedTeamInstantiationPlanTextDigestIsolationAndValidationFailures(t *testing.T) {
	context, catalog, discovery, selections, binding := savedTeamInstantiationFixture(t)
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery", Text: "private one"}
	valid, err := BuildSavedTeamInstantiationPlan(intent, context, catalog, binding, discovery, selections)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	changedText := intent
	changedText.Text = "private two"
	again, err := BuildSavedTeamInstantiationPlan(changedText, context, catalog, binding, discovery, selections)
	if err != nil || !reflect.DeepEqual(again, valid) {
		t.Fatalf("text-only change = (%#v,%v), want %#v", again, err, valid)
	}

	dormant := valid.DormantSubAgents()
	dormant[0].RuntimeInstanceID = "mutated"
	if valid.DormantSubAgents()[0].RuntimeInstanceID == "mutated" {
		t.Fatal("accessor mutation changed plan")
	}

	tamperedDigest := cloneSavedTeamInstantiationPlan(valid)
	tamperedDigest.planDigest = "tampered"
	activeDormant := cloneSavedTeamInstantiationPlan(valid)
	activeDormant.dormantSubAgents[0].Dormant = false
	activeDormant.planDigest, _ = digestSavedTeamInstantiationPlan(activeDormant)
	workItems := cloneSavedTeamInstantiationPlan(valid)
	workItems.workItemCount = 1
	workItems.planDigest, _ = digestSavedTeamInstantiationPlan(workItems)

	for _, tt := range []struct {
		name string
		plan SavedTeamInstantiationPlanCandidate
		want error
	}{
		{name: "zero", plan: SavedTeamInstantiationPlanCandidate{}, want: ErrInvalidSavedTeamInstantiationPlan},
		{name: "digest", plan: tamperedDigest, want: ErrSavedTeamInstantiationPlanDigestMismatch},
		{name: "active dormant", plan: activeDormant, want: ErrInvalidSavedTeamDormantState},
		{name: "work items", plan: workItems, want: ErrInvalidSavedTeamInstantiationPlan},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateSavedTeamInstantiationPlan(
				tt.plan, intent, context, catalog, binding, discovery, selections,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSavedTeamInstantiationPlan() error = %v, want %v", err, tt.want)
			}
			if got != (SavedTeamInstantiationPlanValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", got)
			}
		})
	}
}

func TestBuildSavedTeamInstantiationPlanReorderAndSourceIsolation(t *testing.T) {
	context, catalog, _, _ := teamResolverFixture(t)
	firstDiscovery, secondDiscovery := savedTeamInstantiationReorderedDiscovery(t)
	selections := savedTeamBindingSelections()
	firstBinding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, "team.delivery", context,
		catalog.AgentDefinitions, catalog.RuntimeProfiles,
		firstDiscovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding(first) error = %v", err)
	}
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"}
	first, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, firstBinding, firstDiscovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan(first) error = %v", err)
	}

	reordered := cloneTeamResolutionCatalogInput(catalog)
	reverseAgentDefinitions(reordered.AgentDefinitions)
	reverseRuntimeProfiles(reordered.RuntimeProfiles)
	reverseTeamDefinitions(reordered.TeamDefinitions)
	reverseStrings(reordered.MainAgentDefinitionIDs)
	reverseSavedTeamSelections(selections)
	secondBinding, err := BuildSavedTeamRuntimeBinding(
		reordered.TeamDefinitions, "team.delivery", context,
		reordered.AgentDefinitions, reordered.RuntimeProfiles,
		secondDiscovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding(second) error = %v", err)
	}
	second, err := BuildSavedTeamInstantiationPlan(
		intent, context, reordered, secondBinding, secondDiscovery, selections,
	)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("reordered plan = (%#v,%v), want %#v", second, err, first)
	}

	reordered.AgentDefinitions[0].ID = "mutated"
	reordered.RuntimeProfiles[0].ID = "mutated"
	reordered.TeamDefinitions[0] = TeamDefinition{}
	reordered.MainAgentDefinitionIDs[0] = "mutated"
	selections[0].RuntimeInstanceID = "mutated"
	observations := secondDiscovery.Observations()
	observations[0].Instance.ID = "mutated"
	observations[0].ModelIDs[0] = "mutated"
	subBindings := secondBinding.SubAgentBindings()
	subBindings[0].AgentDefinitionID = "mutated"
	if !reflect.DeepEqual(second, first) || second.PlanDigest() != first.PlanDigest() {
		t.Fatal("source mutation changed stored instantiation plan")
	}
}

func TestSavedTeamInstantiationPlanDigestSensitivity(t *testing.T) {
	context, catalog, discovery, selections, binding := savedTeamInstantiationFixture(t)
	baseline, err := BuildSavedTeamInstantiationPlan(
		mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"},
		context, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*SavedTeamInstantiationPlanCandidate)
	}{
		{name: "ready", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.ready = false }},
		{name: "trigger", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.trigger = mode.TriggerAssign }},
		{name: "target", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.targetID = "team.other" }},
		{name: "resolution digest", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.resolutionDigest = "other" }},
		{name: "team id", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.teamDefinitionID = "team.other" }},
		{name: "team version", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.teamDefinitionVersion++ }},
		{name: "team scope", mutate: func(plan *SavedTeamInstantiationPlanCandidate) {
			plan.teamDefinitionScope = TeamDefinitionScopeReusable
		}},
		{name: "scope identity", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.scopeIdentity.ProjectID = "other" }},
		{name: "team digest", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.teamDefinitionDigest = "other" }},
		{name: "discovery digest", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.runtimeDiscoveryDigest = "other" }},
		{name: "binding digest", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.bindingDigest = "other" }},
		{name: "main agent", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.mainSeed.AgentDefinitionID = "other" }},
		{name: "main profile", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.mainSeed.RuntimeProfileID = "other" }},
		{name: "main runtime", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.mainSeed.RuntimeInstanceID = "other" }},
		{name: "main binding", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.mainSeed.Binding.Accepted = false }},
		{name: "dormant state", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.dormantSubAgents[0].Dormant = false }},
		{name: "dormant agent", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.dormantSubAgents[0].AgentDefinitionID = "other" }},
		{name: "dormant profile", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.dormantSubAgents[0].RuntimeProfileID = "other" }},
		{name: "dormant runtime", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.dormantSubAgents[0].RuntimeInstanceID = "other" }},
		{name: "create team", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.createTeamInstance = false }},
		{name: "create main", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.createMainAgentInstance = false }},
		{name: "create subagent", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.createSubAgentInstances = true }},
		{name: "draft", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.requiresDraft = true }},
		{name: "work items", mutate: func(plan *SavedTeamInstantiationPlanCandidate) { plan.workItemCount = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneSavedTeamInstantiationPlan(baseline)
			tt.mutate(&changed)
			digest, err := digestSavedTeamInstantiationPlan(changed)
			if err != nil {
				t.Fatalf("digestSavedTeamInstantiationPlan() error = %v", err)
			}
			if digest == baseline.PlanDigest() {
				t.Fatalf("%s did not change plan digest", tt.name)
			}
		})
	}
}

func TestValidateSavedTeamInstantiationPlanRejectsChangedRouteAndBinding(t *testing.T) {
	context, catalog, discovery, selections, binding := savedTeamInstantiationFixture(t)
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"}
	valid, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}

	changedRoute := mode.Intent{Trigger: mode.TriggerAssign, TargetID: "team.delivery"}
	assertSavedTeamInstantiationValidationError(
		t, valid, changedRoute, context, catalog, binding, discovery, selections,
		ErrSavedTeamInstantiationPlanSourceMismatch,
	)

	changedDiscovery := savedTeamBindingDiscoveryWithID(t, "runtime.other", 3)
	changedSelections := []SavedTeamRuntimeSelection{
		{AgentDefinitionID: "agent.main", RuntimeInstanceID: "runtime.other"},
		{AgentDefinitionID: "agent.sub.one", RuntimeInstanceID: "runtime.other"},
		{AgentDefinitionID: "agent.sub.two", RuntimeInstanceID: "runtime.other"},
	}
	changedBinding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, "team.delivery", context,
		catalog.AgentDefinitions, catalog.RuntimeProfiles,
		changedDiscovery, changedSelections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding(changed) error = %v", err)
	}
	assertSavedTeamInstantiationValidationError(
		t, valid, intent, context, catalog, changedBinding,
		changedDiscovery, changedSelections,
		ErrSavedTeamInstantiationPlanSourceMismatch,
	)
}

func TestSavedTeamInstantiationPlanImportBoundary(t *testing.T) {
	file, err := parser.ParseFile(
		token.NewFileSet(), "saved_team_instantiation.go",
		mustReadSavedTeamInstantiationFile(t), parser.ImportsOnly,
	)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	allowed := map[string]bool{
		`"crypto/sha256"`:                    true,
		`"encoding/hex"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"reflect"`:                          true,
		`"sort"`:                             true,
		`"loom-pi-rebuild/internal/agents"`:  true,
		`"loom-pi-rebuild/internal/mode"`:    true,
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("saved_team_instantiation.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func savedTeamInstantiationReorderedDiscovery(
	t *testing.T,
) (loomruntime.RuntimeDiscoverySnapshot, loomruntime.RuntimeDiscoverySnapshot) {
	t.Helper()
	shared := catalogRuntimeProbe("probe.shared", loomruntime.RuntimeObservation{
		Instance: catalogRuntimeInstance("runtime.shared", func(instance *loomruntime.RuntimeInstance) {
			instance.AdapterType = "test"
			instance.ObservedCapabilities = []string{"text"}
			instance.Capacity = 3
		}),
		ModelIDs: []string{"model.test"},
	})
	spare := catalogRuntimeProbe("probe.spare", loomruntime.RuntimeObservation{
		Instance: catalogRuntimeInstance("runtime.spare", func(instance *loomruntime.RuntimeInstance) {
			instance.AdapterType = "test"
			instance.ObservedCapabilities = []string{"text"}
			instance.Capacity = 1
		}),
		ModelIDs: []string{"model.test"},
	})
	return mustCatalogRuntimeDiscovery(t, []loomruntime.RuntimeProbe{shared, spare}),
		mustCatalogRuntimeDiscovery(t, []loomruntime.RuntimeProbe{spare, shared})
}

func reverseStrings(input []string) {
	for left, right := 0, len(input)-1; left < right; left, right = left+1, right-1 {
		input[left], input[right] = input[right], input[left]
	}
}

func reverseSavedTeamSelections(input []SavedTeamRuntimeSelection) {
	for left, right := 0, len(input)-1; left < right; left, right = left+1, right-1 {
		input[left], input[right] = input[right], input[left]
	}
}

func assertSavedTeamInstantiationValidationError(
	t *testing.T,
	plan SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
	want error,
) {
	t.Helper()
	got, err := ValidateSavedTeamInstantiationPlan(
		plan, intent, context, catalog, binding, discovery, selections,
	)
	if !errors.Is(err, want) {
		t.Fatalf("ValidateSavedTeamInstantiationPlan() error = %v, want %v", err, want)
	}
	if got != (SavedTeamInstantiationPlanValidationCandidate{}) {
		t.Fatalf("failed validation returned Candidate %#v", got)
	}
}

func savedTeamInstantiationFixture(
	t *testing.T,
) (
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
	binding SavedTeamRuntimeBindingCandidate,
) {
	t.Helper()
	context, catalog, _, _ = teamResolverFixture(t)
	discovery = savedTeamBindingDiscovery(t, "online", 3, []string{"model.test"})
	selections = savedTeamBindingSelections()
	binding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, "team.delivery", context,
		catalog.AgentDefinitions, catalog.RuntimeProfiles,
		discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	return
}

func dormantAgentIDs(input []SavedTeamDormantSubAgentSeed) string {
	result := ""
	for index, seed := range input {
		if index > 0 {
			result += ","
		}
		result += seed.AgentDefinitionID
	}
	return result
}

func assertZeroSavedTeamInstantiationPlan(t *testing.T, got SavedTeamInstantiationPlanCandidate) {
	t.Helper()
	if !reflect.DeepEqual(got, SavedTeamInstantiationPlanCandidate{}) {
		t.Fatalf("failed build returned plan %#v", got)
	}
}

func mustReadSavedTeamInstantiationFile(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile("saved_team_instantiation.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return content
}
