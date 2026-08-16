package teams

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

func TestBuildAcceptedDraftInstantiationPlan(t *testing.T) {
	catalog, decided := acceptedInstantiationFixture(t, false, 50)

	got, err := BuildAcceptedDraftInstantiationPlan(decided, catalog)
	if err != nil {
		t.Fatalf("BuildAcceptedDraftInstantiationPlan() error = %v", err)
	}
	if !got.Ready() || got.SourceKind() != TeamInstantiationSourceAcceptedDraft ||
		got.DraftID() != decided.DraftID() ||
		got.SourceRevision() != decided.SourceRevision() ||
		got.TerminalRevision() != decided.Revision() ||
		got.CatalogDigest() != decided.CatalogDigest() ||
		got.ContentDigest() != decided.ContentDigest() ||
		got.BindingDigest() != decided.BindingDigest() ||
		got.DecisionDigest() != decided.DecisionDigest() ||
		got.RequestedBudget() != 50 || got.RequestedConcurrency() != 2 ||
		got.BudgetCeiling() != 100 || got.ConcurrencyCeiling() != 2 {
		t.Fatalf("instantiation plan identity = %#v", got)
	}
	assertSHA256Digest(t, got.PlanDigest())

	main := got.MainRole()
	if main.Kind != TeamInstantiationRoleMain ||
		main.AgentDefinitionID != "agent.main" ||
		main.RuntimeProfile.ID != "profile.main" ||
		main.RuntimeProfile.ModelID != "model.alpha" ||
		main.RuntimeInstanceID != "runtime.local" ||
		!reflect.DeepEqual(main.SkillIDs, []string{"skill.a"}) ||
		!reflect.DeepEqual(main.MemberIDs, []string{"member.a"}) ||
		!reflect.DeepEqual(main.PermissionIDs, []string{"permission.read"}) {
		t.Fatalf("MainRole() = %#v", main)
	}
	subAgents := got.SubAgentRoles()
	if roleSeedIDs(subAgents) != "agent.sub.a,agent.sub.b" {
		t.Fatalf("SubAgentRoles() = %#v", subAgents)
	}
	assertInstantiationRoleSeed(
		t, subAgents[0], "agent.sub.a", "profile.sub.a", "model.beta",
		[]string{"skill.z"}, []string{"member.z"}, []string{"permission.write"},
	)
	assertInstantiationRoleSeed(
		t, subAgents[1], "agent.sub.b", "profile.sub.b", "model.alpha",
		[]string{"skill.a", "skill.z"}, []string{"member.a"}, []string{"permission.read"},
	)
	tasks := got.WorkItems()
	if workItemSeedIDs(tasks) != "task.alpha,task.beta,task.gamma" ||
		tasks[1].OwnerAgentDefinitionID != "agent.sub.b" ||
		!reflect.DeepEqual(tasks[2].DependencyTaskIDs, []string{"task.alpha", "task.beta"}) {
		t.Fatalf("WorkItems() = %#v", tasks)
	}
	if got.CustomerRuleSummary() == "" || len(got.ApprovalMarkers()) != 2 ||
		len(got.CapabilityGaps()) != 0 {
		t.Fatalf("annotations = (%q,%#v,%#v)", got.CustomerRuleSummary(), got.ApprovalMarkers(), got.CapabilityGaps())
	}

	validated, err := ValidateAcceptedDraftInstantiationPlan(got, decided, catalog)
	if err != nil {
		t.Fatalf("ValidateAcceptedDraftInstantiationPlan() error = %v", err)
	}
	want := TeamInstantiationPlanValidationCandidate{
		Valid:                 true,
		DraftID:               decided.DraftID(),
		DecisionDigest:        decided.DecisionDigest(),
		PlanDigest:            got.PlanDigest(),
		MainAgentDefinitionID: "agent.main",
		RoleCount:             3,
		TaskCount:             3,
	}
	if !reflect.DeepEqual(validated, want) {
		t.Fatalf("validation Candidate = %#v, want %#v", validated, want)
	}
}

func TestBuildAcceptedDraftInstantiationPlanOneSubAgentAndZeroBudget(t *testing.T) {
	catalog, decided := acceptedInstantiationFixture(t, true, 0)
	got, err := BuildAcceptedDraftInstantiationPlan(decided, catalog)
	if err != nil {
		t.Fatalf("BuildAcceptedDraftInstantiationPlan() error = %v", err)
	}
	if got.RequestedBudget() != 0 || got.RequestedConcurrency() != 2 ||
		len(got.SubAgentRoles()) != 1 || len(got.WorkItems()) != 1 {
		t.Fatalf("zero-budget one-SubAgent plan = %#v", got)
	}
	if _, err := ValidateAcceptedDraftInstantiationPlan(got, decided, catalog); err != nil {
		t.Fatalf("ValidateAcceptedDraftInstantiationPlan() error = %v", err)
	}
}

func TestBuildAcceptedDraftInstantiationPlanFourAgentTeam(t *testing.T) {
	catalog, input := fourRoleDraftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.four-agent", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)
	decided := mustDecideStructuredTeamDraft(
		t, proposed, catalog, decisionCommand(proposed, TeamDraftDecisionAccepted),
	)

	got, err := BuildAcceptedDraftInstantiationPlan(decided, catalog)
	if err != nil {
		t.Fatalf("BuildAcceptedDraftInstantiationPlan(4 agents) error = %v", err)
	}
	if !got.Ready() || len(got.SubAgentRoles()) != 3 {
		t.Fatalf("four-agent instantiation plan = %#v", got)
	}
	validated, err := ValidateAcceptedDraftInstantiationPlan(got, decided, catalog)
	if err != nil || !validated.Valid || validated.RoleCount != 4 {
		t.Fatalf("ValidateAcceptedDraftInstantiationPlan(4 agents) = (%#v, %v)", validated, err)
	}
}

func TestBuildAcceptedDraftInstantiationPlanFailures(t *testing.T) {
	catalog, accepted := acceptedInstantiationFixture(t, false, 50)
	proposed := accepted.Source()
	rejected := mustDecideStructuredTeamDraft(t, proposed, catalog, decisionCommand(proposed, TeamDraftDecisionRejected))
	expired := mustDecideStructuredTeamDraft(t, proposed, catalog, decisionCommand(proposed, TeamDraftDecisionExpired))
	tamperedDecision := cloneDecidedTeamDraft(accepted)
	tamperedDecision.decisionDigest = "tampered"
	negativeBudget := mutateAcceptedInstantiationReferences(t, accepted, func(refs *TeamDraftReferences) {
		refs.RequestedBudget = -1
	})
	budgetOverflow := mutateAcceptedInstantiationReferences(t, accepted, func(refs *TeamDraftReferences) {
		refs.RequestedBudget = 101
	})
	concurrencyOverflow := mutateAcceptedInstantiationReferences(t, accepted, func(refs *TeamDraftReferences) {
		refs.RequestedConcurrency = 4
	})
	tamperedContent := cloneDecidedTeamDraft(accepted)
	tamperedContent.source.content.tasks[0].AcceptanceCriteria[0] = "tampered"
	tamperedBinding := cloneDecidedTeamDraft(accepted)
	tamperedBinding.source.bindingDigest = "tampered"
	referenceMismatch := cloneDecidedTeamDraft(accepted)
	referenceMismatch.source.draft.references.SkillIDs = []string{"skill.a"}
	referenceMismatch.source.bindingDigest, _ = digestStructuredTeamDraft(referenceMismatch.source)
	referenceMismatch.command.BindingDigest = referenceMismatch.source.BindingDigest()
	referenceMismatch.decisionDigest, _ = digestDecidedTeamDraft(referenceMismatch)

	tests := []struct {
		name    string
		decided DecidedTeamDraft
		catalog TeamDraftCatalogSnapshot
		want    error
	}{
		{name: "zero decision", decided: DecidedTeamDraft{}, catalog: catalog, want: ErrInvalidDecidedTeamDraft},
		{name: "rejected", decided: rejected, catalog: catalog, want: ErrTeamInstantiationRequiresAcceptedDraft},
		{name: "expired", decided: expired, catalog: catalog, want: ErrTeamInstantiationRequiresAcceptedDraft},
		{name: "decision digest", decided: tamperedDecision, catalog: catalog, want: ErrTeamDraftDecisionDigestMismatch},
		{name: "catalog mismatch", decided: accepted, catalog: draftCatalogFixture(t, []string{"skill.extra"}), want: ErrTeamDraftCatalogMismatch},
		{name: "negative budget", decided: negativeBudget, catalog: catalog, want: ErrInvalidTeamDraftReference},
		{name: "budget overflow", decided: budgetOverflow, catalog: catalog, want: ErrBudgetCeilingExceeded},
		{name: "concurrency overflow", decided: concurrencyOverflow, catalog: catalog, want: ErrConcurrencyCeilingExceeded},
		{name: "content tamper", decided: tamperedContent, catalog: catalog, want: ErrTeamDraftContentDigestMismatch},
		{name: "reference mismatch", decided: referenceMismatch, catalog: catalog, want: ErrStructuredTeamDraftReferenceMismatch},
		{name: "binding tamper", decided: tamperedBinding, catalog: catalog, want: ErrStructuredTeamDraftBindingDigestMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildAcceptedDraftInstantiationPlan(tt.decided, tt.catalog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildAcceptedDraftInstantiationPlan() error = %v, want %v", err, tt.want)
			}
			assertZeroTeamInstantiationPlan(t, got)
		})
	}
}

func TestValidateAcceptedDraftInstantiationPlanFailures(t *testing.T) {
	catalog, decided := acceptedInstantiationFixture(t, false, 50)
	valid := mustAcceptedDraftInstantiationPlan(t, decided, catalog)
	_, otherDecision := acceptedInstantiationFixture(t, false, 40)
	mismatchedCatalog := draftCatalogFixture(t, []string{"skill.extra"})

	tamperedDigest := cloneTeamInstantiationPlan(valid)
	tamperedDigest.planDigest = "tampered"
	requestMismatch := cloneTeamInstantiationPlan(valid)
	requestMismatch.requestedBudget++
	requestMismatch.planDigest, _ = digestTeamInstantiationPlan(requestMismatch)
	roleMismatch := cloneTeamInstantiationPlan(valid)
	roleMismatch.subAgentRoles[0].RuntimeInstanceID = "runtime.other"
	roleMismatch.planDigest, _ = digestTeamInstantiationPlan(roleMismatch)
	taskMismatch := cloneTeamInstantiationPlan(valid)
	taskMismatch.workItems[0].OwnerAgentDefinitionID = "agent.sub.b"
	taskMismatch.planDigest, _ = digestTeamInstantiationPlan(taskMismatch)
	ceilingOverflow := cloneTeamInstantiationPlan(valid)
	ceilingOverflow.budgetCeiling = 49
	ceilingOverflow.planDigest, _ = digestTeamInstantiationPlan(ceilingOverflow)

	tests := []struct {
		name    string
		plan    TeamInstantiationPlanCandidate
		decided DecidedTeamDraft
		catalog TeamDraftCatalogSnapshot
		want    error
	}{
		{name: "zero plan", plan: TeamInstantiationPlanCandidate{}, decided: decided, catalog: catalog, want: ErrInvalidTeamInstantiationPlan},
		{name: "digest tamper", plan: tamperedDigest, decided: decided, catalog: catalog, want: ErrTeamInstantiationPlanDigestMismatch},
		{name: "request mismatch", plan: requestMismatch, decided: decided, catalog: catalog, want: ErrTeamInstantiationPlanSourceMismatch},
		{name: "role mismatch", plan: roleMismatch, decided: decided, catalog: catalog, want: ErrTeamInstantiationPlanSourceMismatch},
		{name: "task mismatch", plan: taskMismatch, decided: decided, catalog: catalog, want: ErrTeamInstantiationPlanSourceMismatch},
		{name: "ceiling overflow", plan: ceilingOverflow, decided: decided, catalog: catalog, want: ErrInvalidTeamInstantiationPlan},
		{name: "decision mismatch", plan: valid, decided: otherDecision, catalog: catalog, want: ErrTeamInstantiationPlanSourceMismatch},
		{name: "catalog mismatch", plan: valid, decided: decided, catalog: mismatchedCatalog, want: ErrTeamDraftCatalogMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAcceptedDraftInstantiationPlan(tt.plan, tt.decided, tt.catalog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateAcceptedDraftInstantiationPlan() error = %v, want %v", err, tt.want)
			}
			if got != (TeamInstantiationPlanValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", got)
			}
		})
	}
}

func TestBuildAcceptedDraftInstantiationPlanDeterminismDigestAndIsolation(t *testing.T) {
	catalog, decided := acceptedInstantiationFixture(t, false, 50)
	first := mustAcceptedDraftInstantiationPlan(t, decided, catalog)
	second := mustAcceptedDraftInstantiationPlan(t, decided, catalog)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("equal accepted semantics produced different plans")
	}

	mutations := []struct {
		name   string
		mutate func(*TeamInstantiationPlanCandidate)
	}{
		{name: "source revision", mutate: func(p *TeamInstantiationPlanCandidate) { p.sourceRevision++ }},
		{name: "terminal revision", mutate: func(p *TeamInstantiationPlanCandidate) { p.terminalRevision++ }},
		{name: "catalog digest", mutate: func(p *TeamInstantiationPlanCandidate) { p.catalogDigest = "changed" }},
		{name: "content digest", mutate: func(p *TeamInstantiationPlanCandidate) { p.contentDigest = "changed" }},
		{name: "binding digest", mutate: func(p *TeamInstantiationPlanCandidate) { p.bindingDigest = "changed" }},
		{name: "decision digest", mutate: func(p *TeamInstantiationPlanCandidate) { p.decisionDigest = "changed" }},
		{name: "main definition", mutate: func(p *TeamInstantiationPlanCandidate) { p.mainRole.AgentDefinitionID = "changed" }},
		{name: "main profile", mutate: func(p *TeamInstantiationPlanCandidate) { p.mainRole.RuntimeProfile.ModelID = "changed" }},
		{name: "main runtime", mutate: func(p *TeamInstantiationPlanCandidate) { p.mainRole.RuntimeInstanceID = "changed" }},
		{name: "main skill", mutate: func(p *TeamInstantiationPlanCandidate) { p.mainRole.SkillIDs[0] = "changed" }},
		{name: "main member", mutate: func(p *TeamInstantiationPlanCandidate) { p.mainRole.MemberIDs[0] = "changed" }},
		{name: "subagent permission", mutate: func(p *TeamInstantiationPlanCandidate) { p.subAgentRoles[0].PermissionIDs[0] = "changed" }},
		{name: "work item id", mutate: func(p *TeamInstantiationPlanCandidate) { p.workItems[0].ID = "changed" }},
		{name: "work item owner", mutate: func(p *TeamInstantiationPlanCandidate) { p.workItems[0].OwnerAgentDefinitionID = "changed" }},
		{name: "task dependency", mutate: func(p *TeamInstantiationPlanCandidate) { p.workItems[2].DependencyTaskIDs = []string{"task.beta"} }},
		{name: "task acceptance", mutate: func(p *TeamInstantiationPlanCandidate) { p.workItems[0].AcceptanceCriteria[0] = "changed" }},
		{name: "rule", mutate: func(p *TeamInstantiationPlanCandidate) { p.customerRuleSummary = "changed" }},
		{name: "approval", mutate: func(p *TeamInstantiationPlanCandidate) { p.approvalMarkers[0].Reason = "changed" }},
		{name: "requested budget", mutate: func(p *TeamInstantiationPlanCandidate) { p.requestedBudget++ }},
		{name: "requested concurrency", mutate: func(p *TeamInstantiationPlanCandidate) { p.requestedConcurrency++ }},
		{name: "budget ceiling", mutate: func(p *TeamInstantiationPlanCandidate) { p.budgetCeiling++ }},
		{name: "concurrency ceiling", mutate: func(p *TeamInstantiationPlanCandidate) { p.concurrencyCeiling++ }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneTeamInstantiationPlan(first)
			tt.mutate(&changed)
			digest, err := digestTeamInstantiationPlan(changed)
			if err != nil {
				t.Fatalf("digestTeamInstantiationPlan() error = %v", err)
			}
			if digest == first.PlanDigest() {
				t.Fatal("semantic change did not change plan digest")
			}
		})
	}

	pristine := cloneTeamInstantiationPlan(first)
	main := first.MainRole()
	*main.RuntimeProfile.Budget = 999
	main.RuntimeProfile.RequiredCapabilities[0] = "mutated"
	main.SkillIDs[0] = "mutated"
	subs := first.SubAgentRoles()
	subs[0].PermissionIDs[0] = "mutated"
	tasks := first.WorkItems()
	tasks[2].DependencyTaskIDs[0] = "mutated"
	markers := first.ApprovalMarkers()
	markers[0].Reason = "mutated"
	if !reflect.DeepEqual(first, pristine) {
		t.Fatal("returned accessor mutation changed stored plan")
	}
}

func TestAcceptedDraftInstantiationPlanImportBoundary(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "instantiation_plan.go", mustReadTeamFile(t, "instantiation_plan.go"), parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	allowed := map[string]bool{
		`"crypto/sha256"`:                    true,
		`"encoding/hex"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"fmt"`:                              true,
		`"reflect"`:                          true,
		`"sort"`:                             true,
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("instantiation_plan.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func acceptedInstantiationFixture(t *testing.T, oneSubAgent bool, requestedBudget int) (TeamDraftCatalogSnapshot, DecidedTeamDraft) {
	t.Helper()
	catalog, input := draftContentFixture(t)
	input.References.RequestedBudget = requestedBudget
	if oneSubAgent {
		input.References.SubAgentDefinitionIDs = []string{"agent.sub.a"}
		input.Roles = input.Roles[:2]
		input.Tasks = input.Tasks[:1]
	}
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.instantiate", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)
	return catalog, mustDecideStructuredTeamDraft(t, proposed, catalog, decisionCommand(proposed, TeamDraftDecisionAccepted))
}

func mutateAcceptedInstantiationReferences(
	t *testing.T,
	decided DecidedTeamDraft,
	mutate func(*TeamDraftReferences),
) DecidedTeamDraft {
	t.Helper()
	changed := decided
	source := cloneStructuredTeamDraft(decided.source)
	refs := source.content.References()
	mutate(&refs)
	source.content.references = copyTeamDraftReferences(refs)
	source.content.digest, _ = digestTeamDraftContent(source.content)
	source.draft.references = copyTeamDraftReferences(refs)
	source.bindingDigest, _ = digestStructuredTeamDraft(source)
	changed.source = source
	changed.command.ContentDigest = source.ContentDigest()
	changed.command.BindingDigest = source.BindingDigest()
	changed.decisionDigest, _ = digestDecidedTeamDraft(changed)
	return changed
}

func mustAcceptedDraftInstantiationPlan(
	t *testing.T,
	decided DecidedTeamDraft,
	catalog TeamDraftCatalogSnapshot,
) TeamInstantiationPlanCandidate {
	t.Helper()
	got, err := BuildAcceptedDraftInstantiationPlan(decided, catalog)
	if err != nil {
		t.Fatalf("BuildAcceptedDraftInstantiationPlan() error = %v", err)
	}
	return got
}

func assertZeroTeamInstantiationPlan(t *testing.T, got TeamInstantiationPlanCandidate) {
	t.Helper()
	if !reflect.DeepEqual(got, TeamInstantiationPlanCandidate{}) {
		t.Fatalf("failed build returned plan %#v", got)
	}
}

func assertInstantiationRoleSeed(
	t *testing.T,
	got TeamInstantiationRoleSeed,
	agentID, profileID, modelID string,
	skills, members, permissions []string,
) {
	t.Helper()
	if got.Kind != TeamInstantiationRoleSubAgent ||
		got.AgentDefinitionID != agentID ||
		got.RuntimeProfile.ID != profileID ||
		got.RuntimeProfile.ModelID != modelID ||
		got.RuntimeInstanceID != "runtime.local" ||
		!reflect.DeepEqual(got.SkillIDs, skills) ||
		!reflect.DeepEqual(got.MemberIDs, members) ||
		!reflect.DeepEqual(got.PermissionIDs, permissions) {
		t.Fatalf("role seed = %#v", got)
	}
}

func roleSeedIDs(input []TeamInstantiationRoleSeed) string {
	ids := make([]byte, 0)
	for index, role := range input {
		if index > 0 {
			ids = append(ids, ',')
		}
		ids = append(ids, role.AgentDefinitionID...)
	}
	return string(ids)
}

func workItemSeedIDs(input []TeamInstantiationWorkItemSeed) string {
	ids := make([]byte, 0)
	for index, item := range input {
		if index > 0 {
			ids = append(ids, ',')
		}
		ids = append(ids, item.ID...)
	}
	return string(ids)
}

func mustReadTeamFile(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", name, err)
	}
	return content
}
