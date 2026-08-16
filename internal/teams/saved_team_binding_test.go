package teams

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/agents"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestBuildSavedTeamRuntimeBinding(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	team := mustBuildTeamDefinition(t, input, definitions, profiles)
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	selections := savedTeamBindingSelections()

	got, err := BuildSavedTeamRuntimeBinding(
		[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, profiles, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	if !got.Ready() || got.TeamDefinitionID() != input.ID ||
		got.TeamDefinitionVersion() != 1 ||
		got.TeamDefinitionDigest() != team.Digest() ||
		got.RuntimeDiscoveryDigest() != discovery.Digest() ||
		got.RoleCount() != 3 {
		t.Fatalf("binding Candidate = %#v", got)
	}
	main := got.MainBinding()
	if main.Kind != TeamDefinitionRoleMain ||
		main.AgentDefinitionID != "agent.main" ||
		main.RuntimeProfileID != "profile.main" ||
		main.RuntimeInstanceID != "runtime.shared" ||
		!main.Binding.Accepted {
		t.Fatalf("MainBinding() = %#v", main)
	}
	if bindingAgentIDs(got.SubAgentBindings()) != "agent.sub.one,agent.sub.two" {
		t.Fatalf("SubAgentBindings() = %#v", got.SubAgentBindings())
	}
	assertSHA256Digest(t, got.BindingDigest())

	validated, err := ValidateSavedTeamRuntimeBinding(
		got, []TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, profiles, discovery, selections,
	)
	if err != nil {
		t.Fatalf("ValidateSavedTeamRuntimeBinding() error = %v", err)
	}
	if validated != (SavedTeamRuntimeBindingValidationCandidate{
		Valid:                  true,
		TeamDefinitionID:       input.ID,
		TeamDefinitionDigest:   team.Digest(),
		RuntimeDiscoveryDigest: discovery.Digest(),
		BindingDigest:          got.BindingDigest(),
		MainAgentDefinitionID:  "agent.main",
		RoleCount:              3,
	}) {
		t.Fatalf("validation Candidate = %#v", validated)
	}

	selections[0].RuntimeInstanceID = "mutated"
	returned := got.SubAgentBindings()
	returned[0].RuntimeInstanceID = "mutated"
	returned[0].ExecutionBinding.Capabilities[0] = "mutated"
	*returned[0].ExecutionBinding.Budget = 999
	returnedMain := got.MainBinding()
	returnedMain.ExecutionBinding.Capabilities[0] = "mutated"
	*returnedMain.ExecutionBinding.Budget = 999
	if got.MainBinding().RuntimeInstanceID != "runtime.shared" ||
		got.SubAgentBindings()[0].RuntimeInstanceID != "runtime.shared" ||
		got.MainBinding().ExecutionBinding.Capabilities[0] != "text" ||
		*got.MainBinding().ExecutionBinding.Budget != 100 ||
		got.SubAgentBindings()[0].ExecutionBinding.Capabilities[0] != "text" ||
		*got.SubAgentBindings()[0].ExecutionBinding.Budget != 100 {
		t.Fatal("input/accessor mutation changed binding Candidate")
	}
}

func TestBuildSavedTeamRuntimeBindingFreezesProviderAccountPerRole(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	for index := range profiles {
		profiles[index].ProviderID = "anthropic"
		profiles[index].ProviderAccountID = "anthropic.account." + profiles[index].ID
		profiles[index].ModelID = "model.test"
		profiles[index].EndpointFingerprint = strings.Repeat(string(rune('a'+index)), 64)
		profiles[index].CredentialReference = "credential-ref-" + strings.ReplaceAll(profiles[index].ID, ".", "-")
		profiles[index].CredentialRevision = int64(index + 1)
	}
	team := mustBuildTeamDefinition(t, input, definitions, profiles)
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})

	got, err := BuildSavedTeamRuntimeBinding(
		[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, profiles, discovery, savedTeamBindingSelections(),
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	bindings := append([]SavedTeamRuntimeRoleBinding{got.MainBinding()}, got.SubAgentBindings()...)
	accounts := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		frozen := binding.ExecutionBinding
		if frozen.ProfileID != binding.RuntimeProfileID ||
			frozen.RuntimeInstanceID != binding.RuntimeInstanceID ||
			frozen.ProviderID != "anthropic" ||
			frozen.ProviderAccountID == "" || frozen.CredentialReference == "" ||
			frozen.CredentialRevision <= 0 || frozen.BindingDigest == "" {
			t.Fatalf("role binding did not freeze account identity: %#v", binding)
		}
		accounts[frozen.ProviderAccountID] = struct{}{}
	}
	if len(accounts) != len(bindings) {
		t.Fatalf("Provider Accounts = %#v, want one independent account per role", accounts)
	}
}

func TestBuildSavedTeamRuntimeBindingCardinalityDormantCapacityResolutionAndReorder(t *testing.T) {
	definitions, profiles, baseInput := fourRoleTeamDefinitionFixture()
	for _, roleCount := range []int{1, 2, 3, 4} {
		t.Run(string(rune('0'+roleCount))+"_roles", func(t *testing.T) {
			input := baseInput
			input.ID = "team.cardinality"
			input.Roles = append([]TeamDefinitionRole(nil), baseInput.Roles[:roleCount]...)
			team := mustBuildTeamDefinition(t, input, definitions, profiles)
			selections := []SavedTeamRuntimeSelection{
				{AgentDefinitionID: "agent.main", RuntimeInstanceID: "runtime.shared"},
				{AgentDefinitionID: "agent.sub.one", RuntimeInstanceID: "runtime.shared"},
				{AgentDefinitionID: "agent.sub.two", RuntimeInstanceID: "runtime.shared"},
				{AgentDefinitionID: "agent.sub.three", RuntimeInstanceID: "runtime.shared"},
			}
			selections = selections[:roleCount]
			discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 1, []string{"model.test"})
			got, err := BuildSavedTeamRuntimeBinding(
				[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
				definitions, profiles, discovery, selections,
			)
			if err != nil {
				t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
			}
			if got.RoleCount() != roleCount || len(got.SubAgentBindings()) != roleCount-1 {
				t.Fatalf("cardinality Candidate = %#v", got)
			}
			if got.MainBinding().Kind != TeamDefinitionRoleMain {
				t.Fatalf("MainBinding().Kind = %q", got.MainBinding().Kind)
			}
			for _, subAgent := range got.SubAgentBindings() {
				if subAgent.Kind != TeamDefinitionRoleSubAgent ||
					subAgent.RuntimeInstanceID != "runtime.shared" {
					t.Fatalf("dormant SubAgent binding = %#v", subAgent)
				}
			}
			validated, err := ValidateSavedTeamRuntimeBinding(
				got, []TeamDefinition{team}, input.ID, input.ScopeIdentity,
				definitions, profiles, discovery, selections,
			)
			if err != nil || !validated.Valid || validated.RoleCount != roleCount {
				t.Fatalf("ValidateSavedTeamRuntimeBinding() = (%#v,%v)", validated, err)
			}
			if discovery.Observations()[0].Instance.Capacity != 1 {
				t.Fatal("binding mutated or reserved Runtime capacity")
			}
		})
	}

	context, catalog, projectTeam, reusableTeam := teamResolverFixture(t)
	newerInput := teamResolverProjectInput()
	newerInput.Version = 2
	newerProject := mustBuildTeamDefinition(t, newerInput, catalog.AgentDefinitions, catalog.RuntimeProfiles)
	archivedInput := teamResolverProjectInput()
	archivedInput.Version = 3
	archivedInput.Status = TeamDefinitionArchived
	archivedProject := mustBuildTeamDefinition(t, archivedInput, catalog.AgentDefinitions, catalog.RuntimeProfiles)
	teams := []TeamDefinition{archivedProject, reusableTeam, projectTeam, newerProject}
	selections := savedTeamBindingSelections()
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	first, err := BuildSavedTeamRuntimeBinding(
		teams, "team.delivery", context, catalog.AgentDefinitions,
		catalog.RuntimeProfiles, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding(precedence) error = %v", err)
	}
	if first.TeamDefinitionScope() != TeamDefinitionScopeProject ||
		first.TeamDefinitionVersion() != 2 ||
		first.TeamDefinitionDigest() != newerProject.Digest() {
		t.Fatalf("resolution Candidate = %#v", first)
	}
	reverseTeamDefinitions(teams)
	reverseAgentDefinitions(catalog.AgentDefinitions)
	reverseRuntimeProfiles(catalog.RuntimeProfiles)
	for left, right := 0, len(selections)-1; left < right; left, right = left+1, right-1 {
		selections[left], selections[right] = selections[right], selections[left]
	}
	second, err := BuildSavedTeamRuntimeBinding(
		teams, "team.delivery", context, catalog.AgentDefinitions,
		catalog.RuntimeProfiles, discovery, selections,
	)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("reordered build = (%#v,%v), want %#v", second, err, first)
	}
}

func TestBuildSavedTeamRuntimeBindingFailures(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	team := mustBuildTeamDefinition(t, input, definitions, profiles)
	validDiscovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})

	tests := []struct {
		name       string
		discovery  loomruntime.RuntimeDiscoverySnapshot
		selections []SavedTeamRuntimeSelection
		profiles   []loomruntime.RuntimeProfile
		want       error
	}{
		{name: "missing role", discovery: validDiscovery, selections: savedTeamBindingSelections()[:2], profiles: profiles, want: ErrIncompleteSavedTeamRuntimeSelection},
		{name: "empty selection", discovery: validDiscovery, selections: mutateSavedTeamSelection(savedTeamBindingSelections(), 0, ""), profiles: profiles, want: ErrInvalidSavedTeamRuntimeSelection},
		{name: "duplicate role", discovery: validDiscovery, selections: append(savedTeamBindingSelections(), SavedTeamRuntimeSelection{AgentDefinitionID: "agent.main", RuntimeInstanceID: "runtime.shared"}), profiles: profiles, want: ErrDuplicateSavedTeamRuntimeSelection},
		{name: "extra role", discovery: validDiscovery, selections: append(savedTeamBindingSelections(), SavedTeamRuntimeSelection{AgentDefinitionID: "agent.extra", RuntimeInstanceID: "runtime.shared"}), profiles: profiles, want: ErrExtraSavedTeamRuntimeSelection},
		{name: "missing runtime", discovery: validDiscovery, selections: mutateSavedTeamSelection(savedTeamBindingSelections(), 0, "runtime.missing"), profiles: profiles, want: ErrSavedTeamRuntimeInstanceNotFound},
		{name: "offline", discovery: savedTeamBindingDiscovery(t, loomruntime.RuntimeOffline, 3, []string{"model.test"}), selections: savedTeamBindingSelections(), profiles: profiles, want: loomruntime.ErrRuntimeOffline},
		{name: "incompatible", discovery: savedTeamBindingDiscovery(t, loomruntime.RuntimeIncompatible, 3, []string{"model.test"}), selections: savedTeamBindingSelections(), profiles: profiles, want: loomruntime.ErrRuntimeIncompatible},
		{name: "disabled", discovery: savedTeamBindingDiscovery(t, loomruntime.RuntimeDisabled, 3, []string{"model.test"}), selections: savedTeamBindingSelections(), profiles: profiles, want: loomruntime.ErrRuntimeDisabled},
		{name: "model missing", discovery: savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.other"}), selections: savedTeamBindingSelections(), profiles: profiles, want: ErrSavedTeamRuntimeModelUnavailable},
		{name: "adapter mismatch", discovery: savedTeamBindingDiscoveryMutated(t, func(instance *loomruntime.RuntimeInstance) { instance.AdapterType = "other" }), selections: savedTeamBindingSelections(), profiles: profiles, want: loomruntime.ErrAdapterMismatch},
		{name: "capability missing", discovery: savedTeamBindingDiscoveryMutated(t, func(instance *loomruntime.RuntimeInstance) { instance.ObservedCapabilities = nil }), selections: savedTeamBindingSelections(), profiles: profiles, want: loomruntime.ErrMissingCapability},
		{name: "invalid profile", discovery: validDiscovery, selections: savedTeamBindingSelections(), profiles: mutateSavedTeamProfiles(profiles, func(profile *loomruntime.RuntimeProfile) { profile.ID = "" }), want: ErrInvalidTeamDefinitionCatalog},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildSavedTeamRuntimeBinding(
				[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
				definitions, tt.profiles, tt.discovery, tt.selections,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(got, SavedTeamRuntimeBindingCandidate{}) {
				t.Fatalf("failed build returned Candidate %#v", got)
			}
		})
	}
	if validDiscovery.Observations()[0].Instance.Capacity != 3 {
		t.Fatal("binding checks mutated discovery capacity")
	}

	duplicateDefinitions := append([]agents.AgentDefinition(nil), definitions...)
	duplicateDefinitions = append(duplicateDefinitions, definitions[0])
	if got, err := BuildSavedTeamRuntimeBinding(
		[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
		duplicateDefinitions, profiles, validDiscovery, savedTeamBindingSelections(),
	); !errors.Is(err, ErrDuplicateTeamDefinitionReference) ||
		!reflect.DeepEqual(got, SavedTeamRuntimeBindingCandidate{}) {
		t.Fatalf("duplicate Agent catalog = (%#v,%v)", got, err)
	}
	duplicateProfiles := append([]loomruntime.RuntimeProfile(nil), profiles...)
	duplicateProfiles = append(duplicateProfiles, profiles[0])
	if got, err := BuildSavedTeamRuntimeBinding(
		[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, duplicateProfiles, validDiscovery, savedTeamBindingSelections(),
	); !errors.Is(err, ErrDuplicateTeamDefinitionReference) ||
		!reflect.DeepEqual(got, SavedTeamRuntimeBindingCandidate{}) {
		t.Fatalf("duplicate Profile catalog = (%#v,%v)", got, err)
	}

	duplicateDiscovery, err := loomruntime.DiscoverRuntime(context.Background(), []loomruntime.RuntimeProbe{
		catalogRuntimeProbe("probe.one", loomruntime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.duplicate", nil), ModelIDs: []string{"model.test"},
		}),
		catalogRuntimeProbe("probe.two", loomruntime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.duplicate", nil), ModelIDs: []string{"model.test"},
		}),
	})
	if !errors.Is(err, loomruntime.ErrDuplicateRuntimeInstance) ||
		duplicateDiscovery.Digest() != "" {
		t.Fatalf("duplicate discovery = (%#v,%v)", duplicateDiscovery, err)
	}
}

func TestValidateSavedTeamRuntimeBindingFailures(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	team := mustBuildTeamDefinition(t, input, definitions, profiles)
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	selections := savedTeamBindingSelections()
	valid, err := BuildSavedTeamRuntimeBinding(
		[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, profiles, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	tampered := cloneSavedTeamRuntimeBinding(valid)
	tampered.bindingDigest = "tampered"

	for _, tt := range []struct {
		name       string
		got        SavedTeamRuntimeBindingCandidate
		discovery  loomruntime.RuntimeDiscoverySnapshot
		selections []SavedTeamRuntimeSelection
		want       error
	}{
		{name: "zero", got: SavedTeamRuntimeBindingCandidate{}, discovery: discovery, selections: selections, want: ErrInvalidSavedTeamRuntimeBinding},
		{name: "digest", got: tampered, discovery: discovery, selections: selections, want: ErrSavedTeamRuntimeBindingDigestMismatch},
		{name: "changed selections", got: valid, discovery: discovery, selections: selections[:2], want: ErrIncompleteSavedTeamRuntimeSelection},
		{name: "changed discovery", got: valid, discovery: savedTeamBindingDiscoveryWithID(t, "runtime.other", 3), selections: selections, want: ErrSavedTeamRuntimeInstanceNotFound},
		{name: "capacity source", got: valid, discovery: savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 2, []string{"model.test"}), selections: selections, want: ErrSavedTeamRuntimeBindingSourceMismatch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candidate, err := ValidateSavedTeamRuntimeBinding(
				tt.got, []TeamDefinition{team}, input.ID, input.ScopeIdentity,
				definitions, profiles, tt.discovery, tt.selections,
			)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSavedTeamRuntimeBinding() error = %v, want %v", err, tt.want)
			}
			if candidate != (SavedTeamRuntimeBindingValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", candidate)
			}
		})
	}
}

func TestBuildSavedTeamRuntimeBindingDigestSensitivityAndValidationSourceMismatch(t *testing.T) {
	definitions, profiles, input := teamDefinitionFixture()
	team := mustBuildTeamDefinition(t, input, definitions, profiles)
	discovery := savedTeamBindingDiscovery(t, loomruntime.RuntimeOnline, 3, []string{"model.test"})
	selections := savedTeamBindingSelections()
	valid, err := BuildSavedTeamRuntimeBinding(
		[]TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, profiles, discovery, selections,
	)
	if err != nil {
		t.Fatalf("BuildSavedTeamRuntimeBinding() error = %v", err)
	}
	mutations := []struct {
		name   string
		mutate func(*SavedTeamRuntimeBindingCandidate)
	}{
		{name: "team id", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.teamDefinitionID = "changed" }},
		{name: "version", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.teamDefinitionVersion++ }},
		{name: "scope", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.teamDefinitionScope = TeamDefinitionScopeReusable }},
		{name: "scope identity", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.scopeIdentity.ProjectID = "changed" }},
		{name: "team digest", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.teamDefinitionDigest = "changed" }},
		{name: "discovery digest", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.runtimeDiscoveryDigest = "changed" }},
		{name: "main kind", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.mainBinding.Kind = TeamDefinitionRoleSubAgent }},
		{name: "main agent", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.mainBinding.AgentDefinitionID = "changed" }},
		{name: "profile", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.subAgentBindings[0].RuntimeProfileID = "changed" }},
		{name: "runtime", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.subAgentBindings[0].RuntimeInstanceID = "changed" }},
		{name: "binding", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.subAgentBindings[0].Binding.InstanceID = "changed" }},
		{name: "role count", mutate: func(c *SavedTeamRuntimeBindingCandidate) { c.roleCount++ }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneSavedTeamRuntimeBinding(valid)
			tt.mutate(&changed)
			digest, err := digestSavedTeamRuntimeBinding(changed)
			if err != nil {
				t.Fatalf("digestSavedTeamRuntimeBinding() error = %v", err)
			}
			if digest == valid.BindingDigest() {
				t.Fatal("semantic change did not change binding digest")
			}
		})
	}

	sourceMismatch := cloneSavedTeamRuntimeBinding(valid)
	sourceMismatch.subAgentBindings[0].RuntimeInstanceID = "runtime.other"
	sourceMismatch.bindingDigest, _ = digestSavedTeamRuntimeBinding(sourceMismatch)
	got, err := ValidateSavedTeamRuntimeBinding(
		sourceMismatch, []TeamDefinition{team}, input.ID, input.ScopeIdentity,
		definitions, profiles, discovery, selections,
	)
	if !errors.Is(err, ErrSavedTeamRuntimeBindingSourceMismatch) ||
		got != (SavedTeamRuntimeBindingValidationCandidate{}) {
		t.Fatalf("source mismatch validation = (%#v,%v)", got, err)
	}
}

func TestSavedTeamRuntimeBindingImportBoundary(t *testing.T) {
	file, err := parser.ParseFile(
		token.NewFileSet(), "saved_team_binding.go",
		mustReadSavedTeamBindingFile(t), parser.ImportsOnly,
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
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("saved_team_binding.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func savedTeamBindingDiscovery(
	t *testing.T,
	status loomruntime.RuntimeStatus,
	capacity int,
	models []string,
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	return mustCatalogRuntimeDiscovery(t, []loomruntime.RuntimeProbe{
		catalogRuntimeProbe("probe.saved", loomruntime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.shared", func(instance *loomruntime.RuntimeInstance) {
				instance.AdapterType = "test"
				instance.Status = status
				instance.ObservedCapabilities = []string{"text"}
				instance.Capacity = capacity
			}),
			ModelIDs: models,
		}),
	})
}

func savedTeamBindingDiscoveryWithID(
	t *testing.T,
	runtimeID string,
	capacity int,
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	return mustCatalogRuntimeDiscovery(t, []loomruntime.RuntimeProbe{
		catalogRuntimeProbe("probe.saved", loomruntime.RuntimeObservation{
			Instance: catalogRuntimeInstance(runtimeID, func(instance *loomruntime.RuntimeInstance) {
				instance.AdapterType = "test"
				instance.ObservedCapabilities = []string{"text"}
				instance.Capacity = capacity
			}),
			ModelIDs: []string{"model.test"},
		}),
	})
}

func savedTeamBindingDiscoveryMutated(
	t *testing.T,
	mutate func(*loomruntime.RuntimeInstance),
) loomruntime.RuntimeDiscoverySnapshot {
	t.Helper()
	return mustCatalogRuntimeDiscovery(t, []loomruntime.RuntimeProbe{
		catalogRuntimeProbe("probe.saved", loomruntime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.shared", func(instance *loomruntime.RuntimeInstance) {
				instance.AdapterType = "test"
				instance.ObservedCapabilities = []string{"text"}
				instance.Capacity = 3
				mutate(instance)
			}),
			ModelIDs: []string{"model.test"},
		}),
	})
}

func mutateSavedTeamProfiles(
	input []loomruntime.RuntimeProfile,
	mutate func(*loomruntime.RuntimeProfile),
) []loomruntime.RuntimeProfile {
	copied := append([]loomruntime.RuntimeProfile(nil), input...)
	mutate(&copied[0])
	return copied
}

func savedTeamBindingSelections() []SavedTeamRuntimeSelection {
	return []SavedTeamRuntimeSelection{
		{AgentDefinitionID: "agent.sub.two", RuntimeInstanceID: "runtime.shared"},
		{AgentDefinitionID: "agent.main", RuntimeInstanceID: "runtime.shared"},
		{AgentDefinitionID: "agent.sub.one", RuntimeInstanceID: "runtime.shared"},
	}
}

func mutateSavedTeamSelection(
	input []SavedTeamRuntimeSelection,
	index int,
	runtimeID string,
) []SavedTeamRuntimeSelection {
	copied := append([]SavedTeamRuntimeSelection(nil), input...)
	copied[index].RuntimeInstanceID = runtimeID
	return copied
}

func bindingAgentIDs(input []SavedTeamRuntimeRoleBinding) string {
	result := ""
	for index, binding := range input {
		if index > 0 {
			result += ","
		}
		result += binding.AgentDefinitionID
	}
	return result
}

func mustReadSavedTeamBindingFile(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile("saved_team_binding.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return content
}
