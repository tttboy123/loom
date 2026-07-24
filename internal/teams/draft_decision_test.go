package teams

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

func TestDecideStructuredTeamDraftAccept(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.decision", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)
	command := decisionCommand(proposed, TeamDraftDecisionAccepted)

	decided, err := DecideStructuredTeamDraft(proposed, 2, catalog, command)
	if err != nil {
		t.Fatalf("DecideStructuredTeamDraft(accepted) error = %v", err)
	}
	assertDecidedIdentity(t, decided, TeamDraftDecisionAccepted, "draft.decision", 2, 3, catalog.Digest(), content.Digest(), proposed.BindingDigest())
	assertSHA256Digest(t, decided.DecisionDigest())
	if !reflect.DeepEqual(decided.Command(), command) {
		t.Fatalf("Command() = %#v, want %#v", decided.Command(), command)
	}
	if !reflect.DeepEqual(decided.References(), proposed.References()) ||
		decided.Content().Digest() != proposed.Content().Digest() {
		t.Fatal("terminal decision did not preserve exact source references/content")
	}

	candidate, err := ValidateDecidedTeamDraft(decided, catalog)
	if err != nil {
		t.Fatalf("ValidateDecidedTeamDraft() error = %v", err)
	}
	want := DecidedTeamDraftValidationCandidate{
		Valid:          true,
		Kind:           TeamDraftDecisionAccepted,
		DraftID:        "draft.decision",
		SourceRevision: 2,
		Revision:       3,
		CatalogDigest:  catalog.Digest(),
		ContentDigest:  content.Digest(),
		BindingDigest:  proposed.BindingDigest(),
		DecisionDigest: decided.DecisionDigest(),
	}
	if !reflect.DeepEqual(candidate, want) {
		t.Fatalf("validation Candidate = %#v, want %#v", candidate, want)
	}

	command.ActorID = "mutated"
	sourceContent := decided.Content()
	sourceContent.tasks[0].AcceptanceCriteria[0] = "mutated"
	refs := decided.References()
	refs.SkillIDs[0] = "mutated"
	if decided.Command().ActorID != "user.owner" ||
		decided.Content().Tasks()[0].AcceptanceCriteria[0] == "mutated" ||
		decided.References().SkillIDs[0] == "mutated" {
		t.Fatal("input/accessor mutation changed terminal decision")
	}
	assertStructuredIdentity(t, proposed, "draft.decision", 2, TeamDraftStateProposed, catalog.Digest(), content.Digest())
}

func TestDecideStructuredTeamDraftAcceptFailures(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.decision", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)
	gapped := structuredGappedContent(t, catalog, input)
	gappedBase := mustNewStructuredTeamDraft(t, "draft.gapped", catalog, gapped)
	awaiting := mustPresentStructuredTeamDraft(t, gappedBase, catalog, &DraftQuestion{ID: "question.gap", Prompt: "Resolve?"})

	tamperedBinding := cloneStructuredTeamDraft(proposed)
	tamperedBinding.bindingDigest = "tampered"
	tamperedContent := cloneStructuredTeamDraft(proposed)
	tamperedContent.content.tasks[0].AcceptanceCriteria[0] = "tampered"
	mismatchedReferences := cloneStructuredTeamDraft(proposed)
	mismatchedReferences.draft.references.SkillIDs = []string{"skill.a"}
	mismatchedReferences.bindingDigest, _ = digestStructuredTeamDraft(mismatchedReferences)

	tests := []struct {
		name     string
		current  StructuredTeamDraft
		revision int
		catalog  TeamDraftCatalogSnapshot
		mutate   func(*TeamDraftDecisionCommand)
		want     error
	}{
		{name: "draft source", current: base, revision: 1, catalog: catalog, want: ErrIneligibleTeamDraftAcceptance},
		{name: "awaiting source", current: awaiting, revision: 2, catalog: catalog, want: ErrIneligibleTeamDraftAcceptance},
		{name: "stale source", current: proposed, revision: 1, catalog: catalog, want: ErrStaleTeamDraftRevision},
		{name: "catalog mismatch", current: proposed, revision: 2, catalog: draftCatalogFixture(t, []string{"skill.extra"}), want: ErrTeamDraftCatalogMismatch},
		{name: "binding tamper", current: tamperedBinding, revision: 2, catalog: catalog, want: ErrStructuredTeamDraftBindingDigestMismatch},
		{name: "content tamper", current: tamperedContent, revision: 2, catalog: catalog, want: ErrTeamDraftContentDigestMismatch},
		{name: "reference mismatch", current: mismatchedReferences, revision: 2, catalog: catalog, want: ErrStructuredTeamDraftReferenceMismatch},
		{name: "empty decision id", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.DecisionID = "" }, want: ErrInvalidTeamDraftDecisionCommand},
		{name: "system actor", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.ActorKind = TeamDraftDecisionActorSystem }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "empty actor", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.ActorID = "" }, want: ErrInvalidTeamDraftDecisionCommand},
		{name: "wrong action", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.Action = TeamDraftDecisionActionReject }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "accept reason", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.Reason = "unexpected" }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "zero timestamp", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.DecidedAtUnixMillis = 0 }, want: ErrInvalidTeamDraftDecisionCommand},
		{name: "draft id mismatch", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.DraftID = "draft.other" }, want: ErrTeamDraftDecisionSourceMismatch},
		{name: "revision mismatch", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.Revision = 1 }, want: ErrTeamDraftDecisionSourceMismatch},
		{name: "catalog digest mismatch", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.CatalogDigest = "mismatch" }, want: ErrTeamDraftDecisionSourceMismatch},
		{name: "content digest mismatch", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.ContentDigest = "mismatch" }, want: ErrTeamDraftDecisionSourceMismatch},
		{name: "binding digest mismatch", current: proposed, revision: 2, catalog: catalog, mutate: func(c *TeamDraftDecisionCommand) { c.BindingDigest = "mismatch" }, want: ErrTeamDraftDecisionSourceMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := decisionCommand(tt.current, TeamDraftDecisionAccepted)
			if tt.mutate != nil {
				tt.mutate(&command)
			}
			got, err := DecideStructuredTeamDraft(tt.current, tt.revision, tt.catalog, command)
			if !errors.Is(err, tt.want) {
				t.Fatalf("DecideStructuredTeamDraft() error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroDecidedTeamDraft(t, got)
		})
	}
}

func TestDecideStructuredTeamDraftRejectAndExpire(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	draft := mustNewStructuredTeamDraft(t, "draft.terminal", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, draft, catalog, nil)
	gapped := structuredGappedContent(t, catalog, input)
	gappedDraft := mustNewStructuredTeamDraft(t, "draft.awaiting", catalog, gapped)
	awaiting := mustPresentStructuredTeamDraft(t, gappedDraft, catalog, &DraftQuestion{ID: "question.gap", Prompt: "Resolve?"})

	for _, source := range []StructuredTeamDraft{draft, awaiting, proposed} {
		t.Run("reject_"+string(source.State()), func(t *testing.T) {
			command := decisionCommand(source, TeamDraftDecisionRejected)
			got, err := DecideStructuredTeamDraft(source, source.Revision(), catalog, command)
			if err != nil {
				t.Fatalf("DecideStructuredTeamDraft(rejected) error = %v", err)
			}
			assertDecidedIdentity(t, got, TeamDraftDecisionRejected, source.ID(), source.Revision(), source.Revision()+1, catalog.Digest(), source.ContentDigest(), source.BindingDigest())
		})
		t.Run("expire_"+string(source.State()), func(t *testing.T) {
			command := decisionCommand(source, TeamDraftDecisionExpired)
			got, err := DecideStructuredTeamDraft(source, source.Revision(), catalog, command)
			if err != nil {
				t.Fatalf("DecideStructuredTeamDraft(expired) error = %v", err)
			}
			assertDecidedIdentity(t, got, TeamDraftDecisionExpired, source.ID(), source.Revision(), source.Revision()+1, catalog.Digest(), source.ContentDigest(), source.BindingDigest())
		})
	}

	tests := []struct {
		name   string
		kind   TeamDraftDecisionKind
		mutate func(*TeamDraftDecisionCommand)
		want   error
	}{
		{name: "reject system actor", kind: TeamDraftDecisionRejected, mutate: func(c *TeamDraftDecisionCommand) { c.ActorKind = TeamDraftDecisionActorSystem }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "reject wrong action", kind: TeamDraftDecisionRejected, mutate: func(c *TeamDraftDecisionCommand) { c.Action = TeamDraftDecisionActionExpire }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "reject empty reason", kind: TeamDraftDecisionRejected, mutate: func(c *TeamDraftDecisionCommand) { c.Reason = "" }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "expire user actor", kind: TeamDraftDecisionExpired, mutate: func(c *TeamDraftDecisionCommand) { c.ActorKind = TeamDraftDecisionActorUser }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "expire wrong action", kind: TeamDraftDecisionExpired, mutate: func(c *TeamDraftDecisionCommand) { c.Action = TeamDraftDecisionActionReject }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "expire empty reason", kind: TeamDraftDecisionExpired, mutate: func(c *TeamDraftDecisionCommand) { c.Reason = "" }, want: ErrInvalidTeamDraftDecisionSemantics},
		{name: "unknown kind", kind: TeamDraftDecisionAccepted, mutate: func(c *TeamDraftDecisionCommand) { c.Kind = TeamDraftDecisionKind("unknown") }, want: ErrInvalidTeamDraftDecisionCommand},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := decisionCommand(proposed, tt.kind)
			tt.mutate(&command)
			got, err := DecideStructuredTeamDraft(proposed, 2, catalog, command)
			if !errors.Is(err, tt.want) {
				t.Fatalf("DecideStructuredTeamDraft() error = %v, want %v", err, tt.want)
			}
			assertZeroDecidedTeamDraft(t, got)
		})
	}
}

func TestValidateDecidedTeamDraftFailures(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.validate", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)
	valid := mustDecideStructuredTeamDraft(t, proposed, catalog, decisionCommand(proposed, TeamDraftDecisionAccepted))

	zero := DecidedTeamDraft{}
	tamperedDecision := cloneDecidedTeamDraft(valid)
	tamperedDecision.command.ActorID = "tampered"
	tamperedDigest := cloneDecidedTeamDraft(valid)
	tamperedDigest.decisionDigest = "tampered"
	tamperedSource := cloneDecidedTeamDraft(valid)
	tamperedSource.source.content.tasks[0].AcceptanceCriteria[0] = "tampered"
	tamperedBinding := cloneDecidedTeamDraft(valid)
	tamperedBinding.source.bindingDigest = "tampered"
	mismatchedReferences := cloneDecidedTeamDraft(valid)
	mismatchedReferences.source.draft.references.SkillIDs = []string{"skill.a"}
	mismatchedReferences.source.bindingDigest, _ = digestStructuredTeamDraft(mismatchedReferences.source)

	tests := []struct {
		name    string
		current DecidedTeamDraft
		catalog TeamDraftCatalogSnapshot
		want    error
	}{
		{name: "zero", current: zero, catalog: catalog, want: ErrInvalidDecidedTeamDraft},
		{name: "decision tamper", current: tamperedDecision, catalog: catalog, want: ErrTeamDraftDecisionDigestMismatch},
		{name: "digest tamper", current: tamperedDigest, catalog: catalog, want: ErrTeamDraftDecisionDigestMismatch},
		{name: "source tamper", current: tamperedSource, catalog: catalog, want: ErrTeamDraftContentDigestMismatch},
		{name: "source binding tamper", current: tamperedBinding, catalog: catalog, want: ErrStructuredTeamDraftBindingDigestMismatch},
		{name: "source reference mismatch", current: mismatchedReferences, catalog: catalog, want: ErrStructuredTeamDraftReferenceMismatch},
		{name: "catalog mismatch", current: valid, catalog: draftCatalogFixture(t, []string{"skill.extra"}), want: ErrTeamDraftCatalogMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDecidedTeamDraft(tt.current, tt.catalog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateDecidedTeamDraft() error = %v, want %v", err, tt.want)
			}
			if got != (DecidedTeamDraftValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", got)
			}
		})
	}
}

func TestDecidedTeamDraftDigestDeterminism(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.digest", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)
	command := decisionCommand(proposed, TeamDraftDecisionAccepted)
	a := mustDecideStructuredTeamDraft(t, proposed, catalog, command)
	b := mustDecideStructuredTeamDraft(t, proposed, catalog, command)
	if a.DecisionDigest() != b.DecisionDigest() {
		t.Fatal("equal semantic decisions produced different digests")
	}

	digestMutations := []struct {
		name   string
		mutate func(*DecidedTeamDraft)
	}{
		{name: "kind", mutate: func(d *DecidedTeamDraft) {
			d.kind = TeamDraftDecisionRejected
			d.command.Kind = TeamDraftDecisionRejected
		}},
		{name: "actor kind", mutate: func(d *DecidedTeamDraft) { d.command.ActorKind = TeamDraftDecisionActorSystem }},
		{name: "actor id", mutate: func(d *DecidedTeamDraft) { d.command.ActorID = "user.other" }},
		{name: "action", mutate: func(d *DecidedTeamDraft) { d.command.Action = TeamDraftDecisionActionReject }},
		{name: "timestamp", mutate: func(d *DecidedTeamDraft) { d.command.DecidedAtUnixMillis++ }},
		{name: "reason", mutate: func(d *DecidedTeamDraft) { d.command.Reason = "changed" }},
	}
	for _, tt := range digestMutations {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneDecidedTeamDraft(a)
			tt.mutate(&changed)
			digest, err := digestDecidedTeamDraft(changed)
			if err != nil {
				t.Fatalf("digestDecidedTeamDraft() error = %v", err)
			}
			if digest == a.DecisionDigest() {
				t.Fatal("semantic decision change did not change digest")
			}
		})
	}

	nextSource := mustPresentStructuredTeamDraft(t, mustNewStructuredTeamDraft(t, "draft.digest", catalog, content), catalog, &DraftQuestion{ID: "q", Prompt: "Q?"})
	nextRevision := mustDecideStructuredTeamDraft(t, nextSource, catalog, decisionCommand(nextSource, TeamDraftDecisionRejected))
	if nextRevision.DecisionDigest() == a.DecisionDigest() {
		t.Fatal("source revision change did not change decision digest")
	}

	changedInput := cloneTeamDraftContentInput(input)
	changedInput.CustomerRuleSummary = "Changed rules."
	changedContent := mustTeamDraftContent(t, catalog, changedInput)
	changedSource := mustPresentStructuredTeamDraft(t, mustNewStructuredTeamDraft(t, "draft.digest", catalog, changedContent), catalog, nil)
	changed := mustDecideStructuredTeamDraft(t, changedSource, catalog, decisionCommand(changedSource, TeamDraftDecisionAccepted))
	if changed.DecisionDigest() == a.DecisionDigest() {
		t.Fatal("source binding/content change did not change decision digest")
	}
}

func TestTeamDraftDecisionImportBoundary(t *testing.T) {
	source, err := os.ReadFile("draft_decision.go")
	if err != nil {
		t.Fatalf("ReadFile(draft_decision.go): %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "draft_decision.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(draft_decision.go): %v", err)
	}
	allowed := map[string]bool{
		`"crypto/sha256"`: true,
		`"encoding/hex"`:  true,
		`"encoding/json"`: true,
		`"errors"`:        true,
		`"fmt"`:           true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("draft_decision.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func decisionCommand(source StructuredTeamDraft, kind TeamDraftDecisionKind) TeamDraftDecisionCommand {
	command := TeamDraftDecisionCommand{
		DecisionID:          "decision.1",
		Kind:                kind,
		ActorKind:           TeamDraftDecisionActorUser,
		ActorID:             "user.owner",
		Action:              TeamDraftDecisionActionConfirmAndStart,
		DraftID:             source.ID(),
		Revision:            source.Revision(),
		CatalogDigest:       source.CatalogDigest(),
		ContentDigest:       source.ContentDigest(),
		BindingDigest:       source.BindingDigest(),
		DecidedAtUnixMillis: 1_753_372_800_000,
	}
	switch kind {
	case TeamDraftDecisionRejected:
		command.Action = TeamDraftDecisionActionReject
		command.Reason = "user declined"
	case TeamDraftDecisionExpired:
		command.ActorKind = TeamDraftDecisionActorSystem
		command.ActorID = "system.deadline"
		command.Action = TeamDraftDecisionActionExpire
		command.Reason = "decision deadline elapsed"
	}
	return command
}

func mustDecideStructuredTeamDraft(
	t *testing.T,
	source StructuredTeamDraft,
	catalog TeamDraftCatalogSnapshot,
	command TeamDraftDecisionCommand,
) DecidedTeamDraft {
	t.Helper()
	got, err := DecideStructuredTeamDraft(source, source.Revision(), catalog, command)
	if err != nil {
		t.Fatalf("DecideStructuredTeamDraft() error = %v", err)
	}
	return got
}

func assertDecidedIdentity(
	t *testing.T,
	got DecidedTeamDraft,
	kind TeamDraftDecisionKind,
	id string,
	sourceRevision, revision int,
	catalogDigest, contentDigest, bindingDigest string,
) {
	t.Helper()
	if got.Kind() != kind || got.DraftID() != id ||
		got.SourceRevision() != sourceRevision || got.Revision() != revision ||
		got.CatalogDigest() != catalogDigest || got.ContentDigest() != contentDigest ||
		got.BindingDigest() != bindingDigest {
		t.Fatalf("decided identity=(%q,%q,%d,%d,%q,%q,%q), want=(%q,%q,%d,%d,%q,%q,%q)",
			got.Kind(), got.DraftID(), got.SourceRevision(), got.Revision(),
			got.CatalogDigest(), got.ContentDigest(), got.BindingDigest(),
			kind, id, sourceRevision, revision, catalogDigest, contentDigest, bindingDigest)
	}
}

func assertZeroDecidedTeamDraft(t *testing.T, got DecidedTeamDraft) {
	t.Helper()
	if got.Kind() != "" || got.DraftID() != "" || got.SourceRevision() != 0 ||
		got.Revision() != 0 || got.CatalogDigest() != "" ||
		got.ContentDigest() != "" || got.BindingDigest() != "" ||
		got.DecisionDigest() != "" || got.Command() != (TeamDraftDecisionCommand{}) ||
		!reflect.DeepEqual(got.References(), TeamDraftReferences{}) {
		t.Fatalf("failed operation returned usable decided Draft %#v", got)
	}
}
