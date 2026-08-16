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

func TestBuildSavedTeamInstanceRecordSet(t *testing.T) {
	intent, context, catalog, discovery, selections, binding, plan :=
		savedTeamInstanceRecordSetFixture(t)
	identity := validSavedTeamInstanceIdentity()

	got, err := BuildSavedTeamInstanceRecordSet(
		plan, intent, context, catalog, binding, discovery, selections, identity,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet() error = %v", err)
	}
	if !got.Ready() || got.SourcePlanDigest() != plan.PlanDigest() ||
		got.TeamInstanceCount() != 1 || got.AgentInstanceCount() != 1 ||
		got.ActiveSubAgentCount() != 0 || got.WorkItemCount() != 0 {
		t.Fatalf("record set = %#v", got)
	}
	team := got.Team()
	if team.ID != identity.TeamInstanceID ||
		team.WorkRequestID != identity.WorkRequestID ||
		team.SourceKind != SavedTeamInstanceSourceSavedTeam ||
		team.TeamDefinitionID != plan.TeamDefinitionID() ||
		team.TeamDefinitionVersion != plan.TeamDefinitionVersion() ||
		team.TeamDefinitionScope != plan.TeamDefinitionScope() ||
		team.ScopeIdentity != plan.ScopeIdentity() ||
		team.TeamDefinitionDigest != plan.TeamDefinitionDigest() ||
		team.SourcePlanDigest != plan.PlanDigest() ||
		team.State != SavedTeamInstanceStateCreated ||
		team.CreatedAt != identity.CreatedAt {
		t.Fatalf("Team() = %#v", team)
	}
	main := got.MainAgent()
	if main.ID != identity.MainAgentInstanceID ||
		main.TeamInstanceID != identity.TeamInstanceID ||
		main.AgentDefinitionID != "agent.main" ||
		main.AgentDefinitionVersion != 1 ||
		main.AgentDefinitionScope != agents.ScopeProject ||
		main.ScopeIdentity != context ||
		main.RuntimeProfileID != "profile.main" ||
		main.RuntimeInstanceID != "runtime.shared" ||
		!main.IsMain ||
		main.State != SavedTeamAgentInstanceStateCreated ||
		!main.Binding.Accepted {
		t.Fatalf("MainAgent() = %#v", main)
	}
	dormant := got.DormantSubAgents()
	if dormantAgentRecordIDs(dormant) != "agent.sub.one,agent.sub.two" {
		t.Fatalf("DormantSubAgents() = %#v", dormant)
	}
	assertSHA256Digest(t, got.RecordSetDigest())

	validated, err := ValidateSavedTeamInstanceRecordSet(
		got, plan, intent, context, catalog, binding, discovery, selections, identity,
	)
	if err != nil {
		t.Fatalf("ValidateSavedTeamInstanceRecordSet() error = %v", err)
	}
	if validated != (SavedTeamInstanceRecordSetValidationCandidate{
		Valid:                true,
		TeamInstanceID:       identity.TeamInstanceID,
		MainAgentInstanceID:  identity.MainAgentInstanceID,
		TeamDefinitionID:     plan.TeamDefinitionID(),
		TeamDefinitionDigest: plan.TeamDefinitionDigest(),
		SourcePlanDigest:     plan.PlanDigest(),
		RecordSetDigest:      got.RecordSetDigest(),
		TeamInstanceCount:    1,
		AgentInstanceCount:   1,
		ActiveSubAgentCount:  0,
		WorkItemCount:        0,
	}) {
		t.Fatalf("validation Candidate = %#v", validated)
	}
}

func TestBuildSavedTeamInstanceRecordSetReusableShadowAndCardinality(t *testing.T) {
	context, catalog, _, reusableTeam := teamResolverFixture(t)
	catalog.ProjectDefaultTeamID = ""
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	selections := savedTeamBindingSelections()
	binding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, reusableTeam.ID(), reusableTeam.ScopeIdentity(),
		catalog.AgentDefinitions, catalog.RuntimeProfiles, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	intent := mode.Intent{Trigger: mode.TriggerUseAgent}
	plan, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	got, err := BuildSavedTeamInstanceRecordSet(
		plan, intent, context, catalog, binding, discovery, selections,
		validSavedTeamInstanceIdentity(),
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet() error = %v", err)
	}
	main := got.MainAgent()
	if got.Team().TeamDefinitionScope != TeamDefinitionScopeReusable ||
		got.Team().ScopeIdentity != (agents.ScopeIdentity{}) ||
		main.AgentDefinitionScope != agents.ScopeReusable ||
		main.ScopeIdentity != (agents.ScopeIdentity{}) ||
		main.AgentDefinitionVersion != 1 {
		t.Fatalf("reusable-shadow record set = %#v", got)
	}

	definitions, profiles, baseInput := fourRoleTeamDefinitionFixture()
	for roleCount := 1; roleCount <= 4; roleCount++ {
		input := baseInput
		input.ID = "team.cardinality"
		input.Roles = append([]TeamDefinitionRole(nil), baseInput.Roles[:roleCount]...)
		team := mustBuildTeamDefinition(t, input, definitions, profiles)
		currentCatalog := TeamResolutionCatalogInput{
			AgentDefinitions:       definitions,
			RuntimeProfiles:        profiles,
			TeamDefinitions:        []TeamDefinition{team},
			MainAgentDefinitionIDs: []string{"agent.main"},
			DefaultMainAgentID:     "agent.main",
			ProjectDefaultTeamID:   input.ID,
		}
		currentDiscovery := savedTeamBindingDiscovery(
			t, loomruntime.RuntimeOnline, roleCount, []string{"model.test"},
		)
		currentSelections := []SavedTeamRuntimeSelection{
			{AgentDefinitionID: "agent.main", RuntimeInstanceID: "runtime.shared"},
			{AgentDefinitionID: "agent.sub.one", RuntimeInstanceID: "runtime.shared"},
			{AgentDefinitionID: "agent.sub.two", RuntimeInstanceID: "runtime.shared"},
			{AgentDefinitionID: "agent.sub.three", RuntimeInstanceID: "runtime.shared"},
		}[:roleCount]
		currentBinding, err := BuildSavedTeamRuntimeBinding(
			currentCatalog.TeamDefinitions, input.ID, input.ScopeIdentity,
			definitions, profiles, currentDiscovery, currentSelections,
		)
		if err != nil {
			t.Fatalf("BuildSavedTeamRuntimeBinding(%d) error = %v", roleCount, err)
		}
		currentIntent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: input.ID}
		currentPlan, err := BuildSavedTeamInstantiationPlan(
			currentIntent, input.ScopeIdentity, currentCatalog, currentBinding,
			currentDiscovery, currentSelections,
		)
		if err != nil {
			t.Fatalf("BuildSavedTeamInstantiationPlan(%d) error = %v", roleCount, err)
		}
		current, err := BuildSavedTeamInstanceRecordSet(
			currentPlan, currentIntent, input.ScopeIdentity, currentCatalog,
			currentBinding, currentDiscovery, currentSelections,
			validSavedTeamInstanceIdentity(),
		)
		if err != nil {
			t.Fatalf("BuildSavedTeamInstanceRecordSet(%d) error = %v", roleCount, err)
		}
		if len(current.DormantSubAgents()) != roleCount-1 ||
			current.AgentInstanceCount() != 1 ||
			current.ActiveSubAgentCount() != 0 ||
			current.WorkItemCount() != 0 {
			t.Fatalf("role-count %d record set = %#v", roleCount, current)
		}
	}
}

func TestBuildSavedTeamInstanceRecordSetProjectTeamReusableAgentFallback(t *testing.T) {
	context, catalog, projectTeam, _ := teamResolverFixture(t)
	reusableDefinitions := make([]agents.AgentDefinition, 0, 3)
	for _, definition := range catalog.AgentDefinitions {
		if definition.Scope == agents.ScopeReusable {
			reusableDefinitions = append(reusableDefinitions, definition)
		}
	}
	catalog.AgentDefinitions = reusableDefinitions
	catalog.TeamDefinitions = []TeamDefinition{projectTeam}
	catalog.ReusableDefaultTeamID = ""
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	selections := savedTeamBindingSelections()
	binding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, projectTeam.ID(), context,
		catalog.AgentDefinitions, catalog.RuntimeProfiles,
		discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: projectTeam.ID()}
	plan, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	got, err := BuildSavedTeamInstanceRecordSet(
		plan, intent, context, catalog, binding, discovery, selections,
		validSavedTeamInstanceIdentity(),
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet() error = %v", err)
	}
	if got.Team().TeamDefinitionScope != TeamDefinitionScopeProject ||
		got.MainAgent().AgentDefinitionScope != agents.ScopeReusable ||
		got.MainAgent().ScopeIdentity != (agents.ScopeIdentity{}) {
		t.Fatalf("project-Team reusable-Agent fallback = %#v", got)
	}
}

func TestBuildSavedTeamInstanceRecordSetFailuresAndIsolation(t *testing.T) {
	intent, context, catalog, discovery, selections, binding, plan :=
		savedTeamInstanceRecordSetFixture(t)
	validIdentity := validSavedTeamInstanceIdentity()

	for _, tt := range []struct {
		name     string
		identity SavedTeamInstanceIdentityInput
	}{
		{name: "empty work request", identity: mutateSavedTeamInstanceIdentity(validIdentity, func(input *SavedTeamInstanceIdentityInput) { input.WorkRequestID = "" })},
		{name: "empty team", identity: mutateSavedTeamInstanceIdentity(validIdentity, func(input *SavedTeamInstanceIdentityInput) { input.TeamInstanceID = "" })},
		{name: "empty main", identity: mutateSavedTeamInstanceIdentity(validIdentity, func(input *SavedTeamInstanceIdentityInput) { input.MainAgentInstanceID = "" })},
		{name: "zero created at", identity: mutateSavedTeamInstanceIdentity(validIdentity, func(input *SavedTeamInstanceIdentityInput) { input.CreatedAt = 0 })},
		{name: "negative created at", identity: mutateSavedTeamInstanceIdentity(validIdentity, func(input *SavedTeamInstanceIdentityInput) { input.CreatedAt = -1 })},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildSavedTeamInstanceRecordSet(
				plan, intent, context, catalog, binding, discovery, selections, tt.identity,
			)
			if !errors.Is(err, ErrInvalidSavedTeamInstanceIdentity) {
				t.Fatalf("BuildSavedTeamInstanceRecordSet() error = %v", err)
			}
			assertZeroSavedTeamInstanceRecordSet(t, got)
		})
	}

	changedIntent := mode.Intent{Trigger: mode.TriggerAssign, TargetID: "team.delivery"}
	if got, err := BuildSavedTeamInstanceRecordSet(
		plan, changedIntent, context, catalog, binding, discovery, selections, validIdentity,
	); !errors.Is(err, ErrSavedTeamInstantiationPlanSourceMismatch) ||
		!reflect.DeepEqual(got, SavedTeamInstanceRecordSetCandidate{}) {
		t.Fatalf("changed route = (%#v,%v)", got, err)
	}

	pristine := cloneSavedTeamInstanceRecordSetMust(t, plan, intent, context, catalog, binding, discovery, selections, validIdentity)
	expected := cloneSavedTeamInstanceRecordSet(pristine)
	catalog.AgentDefinitions[0].ID = "mutated"
	catalog.RuntimeProfiles[0].ID = "mutated"
	catalog.TeamDefinitions[0] = TeamDefinition{}
	selections[0].RuntimeInstanceID = "mutated"
	dormant := pristine.DormantSubAgents()
	dormant[0].AgentDefinitionID = "mutated"
	if pristine.DormantSubAgents()[0].AgentDefinitionID == "mutated" ||
		!reflect.DeepEqual(pristine, expected) ||
		pristine.RecordSetDigest() != expected.RecordSetDigest() {
		t.Fatal("source/accessor mutation changed record set")
	}
}

func TestBuildSavedTeamInstanceRecordSetReorderAndLatestDefinitionVersion(t *testing.T) {
	context, catalog, _, _ := teamResolverFixture(t)
	firstDiscovery, secondDiscovery := savedTeamInstantiationReorderedDiscovery(t)
	selections := savedTeamBindingSelections()
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"}
	identity := validSavedTeamInstanceIdentity()

	firstBinding, err := BuildSavedTeamRuntimeBinding(
		catalog.TeamDefinitions, "team.delivery", context,
		catalog.AgentDefinitions, catalog.RuntimeProfiles,
		firstDiscovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding(first) error = %v", err)
	}
	firstPlan, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, firstBinding, firstDiscovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan(first) error = %v", err)
	}
	first, err := BuildSavedTeamInstanceRecordSet(
		firstPlan, intent, context, catalog, firstBinding,
		firstDiscovery, selections, identity,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet(first) error = %v", err)
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
	secondPlan, err := BuildSavedTeamInstantiationPlan(
		intent, context, reordered, secondBinding, secondDiscovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan(second) error = %v", err)
	}
	second, err := BuildSavedTeamInstanceRecordSet(
		secondPlan, intent, context, reordered, secondBinding,
		secondDiscovery, selections, identity,
	)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("reordered record set = (%#v,%v), want %#v", second, err, first)
	}

	versioned := cloneTeamResolutionCatalogInput(catalog)
	var newer agents.AgentDefinition
	for _, definition := range versioned.AgentDefinitions {
		if definition.ID == "agent.main" && definition.Scope == agents.ScopeProject {
			newer = definition
			break
		}
	}
	if newer.ID == "" {
		t.Fatal("project Main AgentDefinition missing")
	}
	newer.Version++
	newer.RoleSpec = "newer main role"
	versioned.AgentDefinitions = append(versioned.AgentDefinitions, newer)
	versionedRecordSet, err := BuildSavedTeamInstanceRecordSet(
		firstPlan, intent, context, versioned, firstBinding,
		firstDiscovery, savedTeamBindingSelections(), identity,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet(versioned) error = %v", err)
	}
	if versionedRecordSet.MainAgent().AgentDefinitionVersion != newer.Version {
		t.Fatalf("Main AgentDefinition version = %d, want %d",
			versionedRecordSet.MainAgent().AgentDefinitionVersion, newer.Version)
	}
}

func TestValidateSavedTeamInstanceRecordSetFailuresAndDigestSensitivity(t *testing.T) {
	intent, context, catalog, discovery, selections, binding, plan :=
		savedTeamInstanceRecordSetFixture(t)
	identity := validSavedTeamInstanceIdentity()
	valid := cloneSavedTeamInstanceRecordSetMust(
		t, plan, intent, context, catalog, binding, discovery, selections, identity,
	)

	tamperedDigest := cloneSavedTeamInstanceRecordSet(valid)
	tamperedDigest.recordSetDigest = "tampered"
	activeDormant := cloneSavedTeamInstanceRecordSet(valid)
	activeDormant.dormantSubAgents[0].Dormant = false
	activeDormant.recordSetDigest, _ = digestSavedTeamInstanceRecordSet(activeDormant)
	badCount := cloneSavedTeamInstanceRecordSet(valid)
	badCount.agentInstanceCount = 2
	badCount.recordSetDigest, _ = digestSavedTeamInstanceRecordSet(badCount)
	badState := cloneSavedTeamInstanceRecordSet(valid)
	badState.team.State = SavedTeamInstanceState("running")
	badState.recordSetDigest, _ = digestSavedTeamInstanceRecordSet(badState)
	sourceMismatch := cloneSavedTeamInstanceRecordSet(valid)
	sourceMismatch.team.WorkRequestID = "other"
	sourceMismatch.recordSetDigest, _ = digestSavedTeamInstanceRecordSet(sourceMismatch)

	for _, tt := range []struct {
		name string
		got  SavedTeamInstanceRecordSetCandidate
		want error
	}{
		{name: "zero", got: SavedTeamInstanceRecordSetCandidate{}, want: ErrInvalidSavedTeamInstanceRecordSet},
		{name: "digest", got: tamperedDigest, want: ErrSavedTeamInstanceRecordSetDigestMismatch},
		{name: "dormant active", got: activeDormant, want: ErrInvalidSavedTeamInstanceDormantState},
		{name: "count", got: badCount, want: ErrInvalidSavedTeamInstanceRecordSet},
		{name: "state", got: badState, want: ErrInvalidSavedTeamInstanceRecordSet},
		{name: "source", got: sourceMismatch, want: ErrSavedTeamInstanceRecordSourceMismatch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateSavedTeamInstanceRecordSet(
				tt.got, plan, intent, context, catalog, binding, discovery,
				selections, identity,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSavedTeamInstanceRecordSet() error = %v, want %v", err, tt.want)
			}
			if got != (SavedTeamInstanceRecordSetValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", got)
			}
		})
	}

	for _, tt := range []struct {
		name   string
		mutate func(*SavedTeamInstanceRecordSetCandidate)
	}{
		{name: "ready", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.ready = false }},
		{name: "team id", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.ID = "other" }},
		{name: "work request", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.WorkRequestID = "other" }},
		{name: "source kind", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.SourceKind = "other" }},
		{name: "team definition", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.TeamDefinitionID = "other" }},
		{name: "team version", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.TeamDefinitionVersion++ }},
		{name: "team scope", mutate: func(got *SavedTeamInstanceRecordSetCandidate) {
			got.team.TeamDefinitionScope = TeamDefinitionScopeReusable
		}},
		{name: "team scope identity", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.ScopeIdentity.ProjectID = "other" }},
		{name: "team digest", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.TeamDefinitionDigest = "other" }},
		{name: "team plan", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.SourcePlanDigest = "other" }},
		{name: "team state", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.State = "other" }},
		{name: "created at", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.team.CreatedAt++ }},
		{name: "main id", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.ID = "other" }},
		{name: "main team id", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.TeamInstanceID = "other" }},
		{name: "main definition", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.AgentDefinitionID = "other" }},
		{name: "main version", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.AgentDefinitionVersion++ }},
		{name: "main scope", mutate: func(got *SavedTeamInstanceRecordSetCandidate) {
			got.mainAgent.AgentDefinitionScope = agents.ScopeReusable
		}},
		{name: "main scope identity", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.ScopeIdentity.ProjectID = "other" }},
		{name: "main profile", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.RuntimeProfileID = "other" }},
		{name: "main runtime", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.RuntimeInstanceID = "other" }},
		{name: "main flag", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.IsMain = false }},
		{name: "main state", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.State = "other" }},
		{name: "main binding", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.Binding.Accepted = false }},
		{name: "main binding profile", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.Binding.ProfileID = "other" }},
		{name: "main binding instance", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.mainAgent.Binding.InstanceID = "other" }},
		{name: "dormant state", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.dormantSubAgents[0].Dormant = false }},
		{name: "dormant agent", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.dormantSubAgents[0].AgentDefinitionID = "other" }},
		{name: "dormant profile", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.dormantSubAgents[0].RuntimeProfileID = "other" }},
		{name: "dormant", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.dormantSubAgents[0].RuntimeInstanceID = "other" }},
		{name: "team count", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.teamInstanceCount++ }},
		{name: "agent count", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.agentInstanceCount++ }},
		{name: "active subagent count", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.activeSubAgentCount++ }},
		{name: "work item count", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.workItemCount++ }},
		{name: "source plan digest", mutate: func(got *SavedTeamInstanceRecordSetCandidate) { got.sourcePlanDigest = "other" }},
	} {
		t.Run("digest_"+tt.name, func(t *testing.T) {
			changed := cloneSavedTeamInstanceRecordSet(valid)
			tt.mutate(&changed)
			digest, err := digestSavedTeamInstanceRecordSet(changed)
			if err != nil {
				t.Fatalf("digestSavedTeamInstanceRecordSet() error = %v", err)
			}
			if digest == valid.RecordSetDigest() {
				t.Fatalf("%s did not change digest", tt.name)
			}
		})
	}
}

func TestValidateSavedTeamInstanceRecordSetRejectsChangedSources(t *testing.T) {
	intent, context, catalog, discovery, selections, binding, plan :=
		savedTeamInstanceRecordSetFixture(t)
	identity := validSavedTeamInstanceIdentity()
	valid := cloneSavedTeamInstanceRecordSetMust(
		t, plan, intent, context, catalog, binding, discovery, selections, identity,
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
	changedPlan, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, changedBinding, changedDiscovery, changedSelections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan(changed) error = %v", err)
	}
	assertSavedTeamInstanceRecordSetValidationError(
		t, valid, changedPlan, intent, context, catalog, changedBinding,
		changedDiscovery, changedSelections, identity,
		ErrSavedTeamInstanceRecordSourceMismatch,
	)

	versioned := cloneTeamResolutionCatalogInput(catalog)
	for _, definition := range catalog.AgentDefinitions {
		if definition.ID == "agent.main" && definition.Scope == agents.ScopeProject {
			newer := definition
			newer.Version++
			versioned.AgentDefinitions = append(versioned.AgentDefinitions, newer)
			break
		}
	}
	assertSavedTeamInstanceRecordSetValidationError(
		t, valid, plan, intent, context, versioned, binding,
		discovery, selections, identity,
		ErrSavedTeamInstanceRecordSourceMismatch,
	)

	tamperedPlan := cloneSavedTeamInstantiationPlan(plan)
	tamperedPlan.planDigest = "tampered"
	assertSavedTeamInstanceRecordSetValidationError(
		t, valid, tamperedPlan, intent, context, catalog, binding,
		discovery, selections, identity,
		ErrSavedTeamInstantiationPlanDigestMismatch,
	)
}

func TestSavedTeamInstanceRecordSetImportBoundary(t *testing.T) {
	content, err := os.ReadFile("saved_team_instances.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "saved_team_instances.go", content, parser.ImportsOnly)
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
			t.Fatalf("saved_team_instances.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func savedTeamInstanceRecordSetFixture(
	t *testing.T,
) (
	mode.Intent,
	agents.ScopeIdentity,
	TeamResolutionCatalogInput,
	loomruntime.RuntimeDiscoverySnapshot,
	[]SavedTeamRuntimeSelection,
	SavedTeamRuntimeBindingCandidate,
	SavedTeamInstantiationPlanCandidate,
) {
	t.Helper()
	context, catalog, discovery, selections, binding := savedTeamInstantiationFixture(t)
	intent := mode.Intent{Trigger: mode.TriggerSelectTeam, TargetID: "team.delivery"}
	plan, err := BuildSavedTeamInstantiationPlan(
		intent, context, catalog, binding, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstantiationPlan() error = %v", err)
	}
	return intent, context, catalog, discovery, selections, binding, plan
}

func validSavedTeamInstanceIdentity() SavedTeamInstanceIdentityInput {
	return SavedTeamInstanceIdentityInput{
		WorkRequestID:       "request.one",
		TeamInstanceID:      "team-instance.one",
		MainAgentInstanceID: "agent-instance.main",
		CreatedAt:           1_721_865_600,
	}
}

func mutateSavedTeamInstanceIdentity(
	input SavedTeamInstanceIdentityInput,
	mutate func(*SavedTeamInstanceIdentityInput),
) SavedTeamInstanceIdentityInput {
	mutate(&input)
	return input
}

func cloneSavedTeamInstanceRecordSetMust(
	t *testing.T,
	plan SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
	identity SavedTeamInstanceIdentityInput,
) SavedTeamInstanceRecordSetCandidate {
	t.Helper()
	got, err := BuildSavedTeamInstanceRecordSet(
		plan, intent, context, catalog, binding, discovery, selections, identity,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamInstanceRecordSet() error = %v", err)
	}
	return got
}

func dormantAgentRecordIDs(input []SavedTeamDormantSubAgentRecord) string {
	result := ""
	for index, record := range input {
		if index > 0 {
			result += ","
		}
		result += record.AgentDefinitionID
	}
	return result
}

func assertZeroSavedTeamInstanceRecordSet(
	t *testing.T,
	got SavedTeamInstanceRecordSetCandidate,
) {
	t.Helper()
	if !reflect.DeepEqual(got, SavedTeamInstanceRecordSetCandidate{}) {
		t.Fatalf("failed build returned Candidate %#v", got)
	}
}

func assertSavedTeamInstanceRecordSetValidationError(
	t *testing.T,
	current SavedTeamInstanceRecordSetCandidate,
	plan SavedTeamInstantiationPlanCandidate,
	intent mode.Intent,
	context agents.ScopeIdentity,
	catalog TeamResolutionCatalogInput,
	binding SavedTeamRuntimeBindingCandidate,
	discovery loomruntime.RuntimeDiscoverySnapshot,
	selections []SavedTeamRuntimeSelection,
	identity SavedTeamInstanceIdentityInput,
	want error,
) {
	t.Helper()
	got, err := ValidateSavedTeamInstanceRecordSet(
		current, plan, intent, context, catalog, binding,
		discovery, selections, identity,
	)
	if !errors.Is(err, want) {
		t.Fatalf("ValidateSavedTeamInstanceRecordSet() error = %v, want %v", err, want)
	}
	if got != (SavedTeamInstanceRecordSetValidationCandidate{}) {
		t.Fatalf("failed validation returned Candidate %#v", got)
	}
}
