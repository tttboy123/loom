package teams

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

func TestStructuredTeamDraft(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)

	t.Run("constructs immutable content-bound revision one", func(t *testing.T) {
		structured, err := NewStructuredTeamDraft("draft.structured", catalog, content)
		if err != nil {
			t.Fatalf("NewStructuredTeamDraft() error = %v", err)
		}
		assertStructuredIdentity(t, structured, "draft.structured", 1, TeamDraftStateDraft, catalog.Digest(), content.Digest())
		assertSHA256Digest(t, structured.BindingDigest())
		if !reflect.DeepEqual(structured.References(), content.References()) {
			t.Fatalf("References() = %#v, want content references %#v", structured.References(), content.References())
		}

		returned := structured.Content()
		returned.roles[0].AgentDefinitionID = "mutated"
		returned.tasks[0].AcceptanceCriteria[0] = "mutated"
		*returned.roles[0].RuntimeProfile.Budget = 999
		if structured.Content().Digest() != content.Digest() ||
			structured.Content().Roles()[0].AgentDefinitionID == "mutated" {
			t.Fatal("Content() accessor mutation changed structured Draft")
		}
	})

	t.Run("construction fails closed", func(t *testing.T) {
		tampered := tamperTeamDraftContent(content, func(snapshot *TeamDraftContentSnapshot) {
			snapshot.tasks[0].AcceptanceCriteria[0] = "tampered"
		})
		tests := []struct {
			name    string
			catalog TeamDraftCatalogSnapshot
			content TeamDraftContentSnapshot
			want    error
		}{
			{name: "zero content", catalog: catalog, want: ErrInvalidTeamDraftContent},
			{name: "tampered content", catalog: catalog, content: tampered, want: ErrTeamDraftContentDigestMismatch},
			{name: "catalog mismatch", catalog: draftCatalogFixture(t, []string{"skill.extra"}), content: content, want: ErrTeamDraftCatalogMismatch},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := NewStructuredTeamDraft("draft.structured", tt.catalog, tt.content)
				if !errors.Is(err, tt.want) {
					t.Fatalf("NewStructuredTeamDraft() error = %v, want errors.Is(%v)", err, tt.want)
				}
				assertZeroStructuredTeamDraft(t, got)
			})
		}
	})

	t.Run("presentation preserves content and enforces gap question", func(t *testing.T) {
		base := mustNewStructuredTeamDraft(t, "draft.structured", catalog, content)
		proposed, err := PresentStructuredTeamDraft(base, 1, catalog, nil)
		if err != nil {
			t.Fatalf("PresentStructuredTeamDraft() error = %v", err)
		}
		assertStructuredIdentity(t, proposed, "draft.structured", 2, TeamDraftStateProposed, catalog.Digest(), content.Digest())
		if proposed.BindingDigest() == base.BindingDigest() {
			t.Fatal("presentation did not change binding digest")
		}

		awaiting, err := PresentStructuredTeamDraft(base, 1, catalog, &DraftQuestion{ID: "question.scope", Prompt: "Which scope?"})
		if err != nil {
			t.Fatalf("PresentStructuredTeamDraft(question) error = %v", err)
		}
		assertStructuredIdentity(t, awaiting, "draft.structured", 2, TeamDraftStateAwaitingAnswer, catalog.Digest(), content.Digest())
		assertStructuredQuestion(t, awaiting, "question.scope", "Which scope?")

		gappedContent := structuredGappedContent(t, catalog, input)
		gappedBase := mustNewStructuredTeamDraft(t, "draft.gapped", catalog, gappedContent)
		next, err := PresentStructuredTeamDraft(gappedBase, 1, catalog, nil)
		if !errors.Is(err, ErrStructuredTeamDraftGapRequiresQuestion) {
			t.Fatalf("PresentStructuredTeamDraft(gap, no question) error = %v", err)
		}
		assertZeroStructuredTeamDraft(t, next)
		gappedAwaiting, err := PresentStructuredTeamDraft(gappedBase, 1, catalog, &DraftQuestion{ID: "question.gap", Prompt: "Resolve gap?"})
		if err != nil {
			t.Fatalf("PresentStructuredTeamDraft(gap, question) error = %v", err)
		}
		assertStructuredIdentity(t, gappedAwaiting, "draft.gapped", 2, TeamDraftStateAwaitingAnswer, catalog.Digest(), gappedContent.Digest())

		for _, tt := range []struct {
			name     string
			current  StructuredTeamDraft
			revision int
			catalog  TeamDraftCatalogSnapshot
			want     error
		}{
			{name: "stale", current: base, revision: 2, catalog: catalog, want: ErrStaleTeamDraftRevision},
			{name: "catalog mismatch", current: base, revision: 1, catalog: draftCatalogFixture(t, []string{"skill.extra"}), want: ErrTeamDraftCatalogMismatch},
			{name: "wrong state", current: proposed, revision: 2, catalog: catalog, want: ErrInvalidTeamDraftStateTransition},
		} {
			t.Run(tt.name, func(t *testing.T) {
				got, err := PresentStructuredTeamDraft(tt.current, tt.revision, tt.catalog, nil)
				if !errors.Is(err, tt.want) {
					t.Fatalf("PresentStructuredTeamDraft() error = %v, want %v", err, tt.want)
				}
				assertZeroStructuredTeamDraft(t, got)
			})
		}
		assertStructuredIdentity(t, base, "draft.structured", 1, TeamDraftStateDraft, catalog.Digest(), content.Digest())
	})
}

func TestStructuredTeamDraftAnswerAndEdit(t *testing.T) {
	catalog, input := draftContentFixture(t)
	readyContent := mustTeamDraftContent(t, catalog, input)
	gappedContent := structuredGappedContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.structured", catalog, gappedContent)
	awaiting := mustPresentStructuredTeamDraft(t, base, catalog, &DraftQuestion{ID: "question.gap", Prompt: "Resolve gap?"})

	t.Run("answer resolves gap and attaches next content once", func(t *testing.T) {
		proposed, err := AnswerStructuredTeamDraft(awaiting, 2, catalog, "question.gap", "Use available capability", readyContent, nil)
		if err != nil {
			t.Fatalf("AnswerStructuredTeamDraft() error = %v", err)
		}
		assertStructuredIdentity(t, proposed, "draft.structured", 3, TeamDraftStateProposed, catalog.Digest(), readyContent.Digest())
		if proposed.LastAnsweredQuestionID() != "question.gap" || proposed.LastAnswerText() != "Use available capability" {
			t.Fatalf("answer metadata = (%q,%q)", proposed.LastAnsweredQuestionID(), proposed.LastAnswerText())
		}
		if proposed.BindingDigest() == awaiting.BindingDigest() {
			t.Fatal("answer did not change binding digest")
		}
		assertStructuredIdentity(t, awaiting, "draft.structured", 2, TeamDraftStateAwaitingAnswer, catalog.Digest(), gappedContent.Digest())
	})

	t.Run("gap-bearing next content requires next question", func(t *testing.T) {
		got, err := AnswerStructuredTeamDraft(awaiting, 2, catalog, "question.gap", "Still blocked", gappedContent, nil)
		if !errors.Is(err, ErrStructuredTeamDraftGapRequiresQuestion) {
			t.Fatalf("AnswerStructuredTeamDraft(gap) error = %v", err)
		}
		assertZeroStructuredTeamDraft(t, got)

		next, err := AnswerStructuredTeamDraft(awaiting, 2, catalog, "question.gap", "Still blocked", gappedContent,
			&DraftQuestion{ID: "question.next", Prompt: "Choose fallback?"})
		if err != nil {
			t.Fatalf("AnswerStructuredTeamDraft(next question) error = %v", err)
		}
		assertStructuredIdentity(t, next, "draft.structured", 3, TeamDraftStateAwaitingAnswer, catalog.Digest(), gappedContent.Digest())
		assertStructuredQuestion(t, next, "question.next", "Choose fallback?")
	})

	t.Run("answer wraps core failures", func(t *testing.T) {
		invalidCurrent := cloneStructuredTeamDraft(awaiting)
		invalidCurrent.draft.id = ""
		invalidCurrent.bindingDigest, _ = digestStructuredTeamDraft(invalidCurrent)
		wrongState := base
		invalidReferences := cloneTeamDraftContentSnapshot(readyContent)
		invalidReferences.references.SkillIDs = append(invalidReferences.references.SkillIDs, "skill.invented")
		invalidReferences.digest, _ = digestTeamDraftContent(invalidReferences)

		tests := []struct {
			name       string
			current    StructuredTeamDraft
			revision   int
			catalog    TeamDraftCatalogSnapshot
			questionID string
			answer     string
			content    TeamDraftContentSnapshot
			next       *DraftQuestion
			want       error
		}{
			{name: "invalid current", current: invalidCurrent, revision: 2, catalog: catalog, questionID: "question.gap", answer: "answer", content: readyContent, want: ErrInvalidTeamDraft},
			{name: "stale", current: awaiting, revision: 1, catalog: catalog, questionID: "question.gap", answer: "answer", content: readyContent, want: ErrStaleTeamDraftRevision},
			{name: "catalog mismatch", current: awaiting, revision: 2, catalog: draftCatalogFixture(t, []string{"skill.extra"}), questionID: "question.gap", answer: "answer", content: readyContent, want: ErrTeamDraftCatalogMismatch},
			{name: "wrong state", current: wrongState, revision: 1, catalog: catalog, questionID: "question.gap", answer: "answer", content: readyContent, want: ErrInvalidTeamDraftStateTransition},
			{name: "wrong question", current: awaiting, revision: 2, catalog: catalog, questionID: "wrong", answer: "answer", content: readyContent, want: ErrTeamDraftQuestionMismatch},
			{name: "empty answer", current: awaiting, revision: 2, catalog: catalog, questionID: "question.gap", content: readyContent, want: ErrInvalidDraftAnswer},
			{name: "invalid next question", current: awaiting, revision: 2, catalog: catalog, questionID: "question.gap", answer: "answer", content: readyContent, next: &DraftQuestion{Prompt: "Missing ID"}, want: ErrInvalidDraftQuestion},
			{name: "invalid next references", current: awaiting, revision: 2, catalog: catalog, questionID: "question.gap", answer: "answer", content: invalidReferences, want: ErrInventedTeamDraftReference},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := AnswerStructuredTeamDraft(tt.current, tt.revision, tt.catalog, tt.questionID, tt.answer, tt.content, tt.next)
				if !errors.Is(err, tt.want) {
					t.Fatalf("AnswerStructuredTeamDraft() error = %v, want %v", err, tt.want)
				}
				assertZeroStructuredTeamDraft(t, got)
			})
		}
	})

	t.Run("edit attaches content and preserves answer metadata", func(t *testing.T) {
		proposed, err := AnswerStructuredTeamDraft(awaiting, 2, catalog, "question.gap", "resolved", readyContent, nil)
		if err != nil {
			t.Fatalf("AnswerStructuredTeamDraft() error = %v", err)
		}
		reopened, err := EditStructuredTeamDraft(proposed, 3, catalog, gappedContent,
			&DraftQuestion{ID: "question.edit", Prompt: "Resolve edited gap?"})
		if err != nil {
			t.Fatalf("EditStructuredTeamDraft() error = %v", err)
		}
		assertStructuredIdentity(t, reopened, "draft.structured", 4, TeamDraftStateAwaitingAnswer, catalog.Digest(), gappedContent.Digest())
		if reopened.LastAnsweredQuestionID() != "question.gap" || reopened.LastAnswerText() != "resolved" {
			t.Fatal("edit did not preserve answer metadata")
		}
		got, err := EditStructuredTeamDraft(proposed, 3, catalog, gappedContent, nil)
		if !errors.Is(err, ErrStructuredTeamDraftGapRequiresQuestion) {
			t.Fatalf("EditStructuredTeamDraft(gap) error = %v", err)
		}
		assertZeroStructuredTeamDraft(t, got)
	})

	t.Run("edit returns typed failures with no usable revision", func(t *testing.T) {
		proposed, err := AnswerStructuredTeamDraft(awaiting, 2, catalog, "question.gap", "resolved", readyContent, nil)
		if err != nil {
			t.Fatalf("AnswerStructuredTeamDraft() error = %v", err)
		}
		invalidCurrent := cloneStructuredTeamDraft(proposed)
		invalidCurrent.draft.id = ""
		invalidCurrent.bindingDigest, _ = digestStructuredTeamDraft(invalidCurrent)
		invalidReferences := cloneTeamDraftContentSnapshot(readyContent)
		invalidReferences.references.SkillIDs = append(invalidReferences.references.SkillIDs, "skill.invented")
		invalidReferences.digest, _ = digestTeamDraftContent(invalidReferences)

		tests := []struct {
			name     string
			current  StructuredTeamDraft
			revision int
			catalog  TeamDraftCatalogSnapshot
			content  TeamDraftContentSnapshot
			question *DraftQuestion
			want     error
		}{
			{name: "invalid current", current: invalidCurrent, revision: 3, catalog: catalog, content: readyContent, want: ErrInvalidTeamDraft},
			{name: "stale", current: proposed, revision: 2, catalog: catalog, content: readyContent, want: ErrStaleTeamDraftRevision},
			{name: "catalog mismatch", current: proposed, revision: 3, catalog: draftCatalogFixture(t, []string{"skill.extra"}), content: readyContent, want: ErrTeamDraftCatalogMismatch},
			{name: "wrong state", current: base, revision: 1, catalog: catalog, content: readyContent, want: ErrInvalidTeamDraftStateTransition},
			{name: "invalid question", current: proposed, revision: 3, catalog: catalog, content: readyContent, question: &DraftQuestion{ID: "question.edit"}, want: ErrInvalidDraftQuestion},
			{name: "invalid references", current: proposed, revision: 3, catalog: catalog, content: invalidReferences, want: ErrInventedTeamDraftReference},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := EditStructuredTeamDraft(tt.current, tt.revision, tt.catalog, tt.content, tt.question)
				if !errors.Is(err, tt.want) {
					t.Fatalf("EditStructuredTeamDraft() error = %v, want %v", err, tt.want)
				}
				assertZeroStructuredTeamDraft(t, got)
			})
		}
	})
}

func TestCheckStructuredTeamDraftAcceptable(t *testing.T) {
	catalog, input := draftContentFixture(t)
	content := mustTeamDraftContent(t, catalog, input)
	base := mustNewStructuredTeamDraft(t, "draft.structured", catalog, content)
	proposed := mustPresentStructuredTeamDraft(t, base, catalog, nil)

	candidate, err := CheckStructuredTeamDraftAcceptable(proposed, 2, catalog)
	if err != nil {
		t.Fatalf("CheckStructuredTeamDraftAcceptable() error = %v", err)
	}
	want := StructuredTeamDraftAcceptanceCandidate{
		Eligible:              true,
		DraftID:               "draft.structured",
		Revision:              2,
		CatalogDigest:         catalog.Digest(),
		ContentDigest:         content.Digest(),
		BindingDigest:         proposed.BindingDigest(),
		MainAgentDefinitionID: "agent.main",
		RoleCount:             3,
		TaskCount:             3,
	}
	if !reflect.DeepEqual(candidate, want) {
		t.Fatalf("Candidate = %#v, want %#v", candidate, want)
	}

	gappedContent := structuredGappedContent(t, catalog, input)
	gappedBase := mustNewStructuredTeamDraft(t, "draft.gapped", catalog, gappedContent)
	gappedAwaiting := mustPresentStructuredTeamDraft(t, gappedBase, catalog, &DraftQuestion{ID: "question.gap", Prompt: "Resolve?"})
	tamperedBinding := cloneStructuredTeamDraft(proposed)
	tamperedBinding.bindingDigest = "tampered"
	tamperedContent := cloneStructuredTeamDraft(proposed)
	tamperedContent.content.tasks[0].AcceptanceCriteria[0] = "tampered"
	mismatchedRefs := cloneStructuredTeamDraft(proposed)
	mismatchedRefs.draft.references.SkillIDs = []string{"skill.a"}
	mismatchedRefs.bindingDigest, _ = digestStructuredTeamDraft(mismatchedRefs)

	tests := []struct {
		name     string
		current  StructuredTeamDraft
		revision int
		catalog  TeamDraftCatalogSnapshot
		want     error
	}{
		{name: "draft state", current: base, revision: 1, catalog: catalog, want: ErrInvalidTeamDraftStateTransition},
		{name: "unresolved gap", current: gappedAwaiting, revision: 2, catalog: catalog, want: ErrStructuredTeamDraftContentNotReady},
		{name: "stale", current: proposed, revision: 1, catalog: catalog, want: ErrStaleTeamDraftRevision},
		{name: "catalog mismatch", current: proposed, revision: 2, catalog: draftCatalogFixture(t, []string{"skill.extra"}), want: ErrTeamDraftCatalogMismatch},
		{name: "binding tamper", current: tamperedBinding, revision: 2, catalog: catalog, want: ErrStructuredTeamDraftBindingDigestMismatch},
		{name: "content tamper", current: tamperedContent, revision: 2, catalog: catalog, want: ErrTeamDraftContentDigestMismatch},
		{name: "reference mismatch", current: mismatchedRefs, revision: 2, catalog: catalog, want: ErrStructuredTeamDraftReferenceMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckStructuredTeamDraftAcceptable(tt.current, tt.revision, tt.catalog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("CheckStructuredTeamDraftAcceptable() error = %v, want %v", err, tt.want)
			}
			if got != (StructuredTeamDraftAcceptanceCandidate{}) {
				t.Fatalf("failed eligibility returned Candidate %#v", got)
			}
		})
	}
}

func TestStructuredTeamDraftBindingDigestDeterminism(t *testing.T) {
	catalog, input := draftContentFixture(t)
	first := mustTeamDraftContent(t, catalog, input)
	reordered := cloneTeamDraftContentInput(input)
	for left, right := 0, len(reordered.Roles)-1; left < right; left, right = left+1, right-1 {
		reordered.Roles[left], reordered.Roles[right] = reordered.Roles[right], reordered.Roles[left]
	}
	second := mustTeamDraftContent(t, catalog, reordered)
	a := mustNewStructuredTeamDraft(t, "draft.same", catalog, first)
	b := mustNewStructuredTeamDraft(t, "draft.same", catalog, second)
	if a.BindingDigest() != b.BindingDigest() {
		t.Fatalf("semantic reorder binding digests differ: %q vs %q", a.BindingDigest(), b.BindingDigest())
	}
	presented := mustPresentStructuredTeamDraft(t, a, catalog, nil)
	if presented.BindingDigest() == a.BindingDigest() {
		t.Fatal("revision/state change did not change binding digest")
	}
	questioned := mustPresentStructuredTeamDraft(t, b, catalog, &DraftQuestion{ID: "question", Prompt: "Prompt?"})
	if questioned.BindingDigest() == presented.BindingDigest() {
		t.Fatal("question semantic change did not change binding digest")
	}
	answerA, err := AnswerStructuredTeamDraft(questioned, 2, catalog, "question", "answer a", first, nil)
	if err != nil {
		t.Fatalf("AnswerStructuredTeamDraft(answer a) error = %v", err)
	}
	answerB, err := AnswerStructuredTeamDraft(questioned, 2, catalog, "question", "answer b", first, nil)
	if err != nil {
		t.Fatalf("AnswerStructuredTeamDraft(answer b) error = %v", err)
	}
	if answerA.BindingDigest() == answerB.BindingDigest() {
		t.Fatal("answer text change did not change binding digest")
	}
	changedInput := cloneTeamDraftContentInput(input)
	changedInput.CustomerRuleSummary = "Changed rules."
	changed := mustNewStructuredTeamDraft(t, "draft.same", catalog, mustTeamDraftContent(t, catalog, changedInput))
	if changed.BindingDigest() == a.BindingDigest() {
		t.Fatal("content digest change did not change binding digest")
	}
}

func TestStructuredTeamDraftImportBoundary(t *testing.T) {
	source, err := os.ReadFile("structured_draft.go")
	if err != nil {
		t.Fatalf("ReadFile(structured_draft.go): %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "structured_draft.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(structured_draft.go): %v", err)
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
			t.Fatalf("structured_draft.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func structuredGappedContent(t *testing.T, catalog TeamDraftCatalogSnapshot, input TeamDraftContentInput) TeamDraftContentSnapshot {
	t.Helper()
	gapped := cloneTeamDraftContentInput(input)
	gapped.CapabilityGaps = []TeamDraftCapabilityGap{{Capability: "gpu", Reason: "not discovered"}}
	return mustTeamDraftContent(t, catalog, gapped)
}

func mustNewStructuredTeamDraft(t *testing.T, id string, catalog TeamDraftCatalogSnapshot, content TeamDraftContentSnapshot) StructuredTeamDraft {
	t.Helper()
	got, err := NewStructuredTeamDraft(id, catalog, content)
	if err != nil {
		t.Fatalf("NewStructuredTeamDraft() error = %v", err)
	}
	return got
}

func mustPresentStructuredTeamDraft(t *testing.T, current StructuredTeamDraft, catalog TeamDraftCatalogSnapshot, question *DraftQuestion) StructuredTeamDraft {
	t.Helper()
	got, err := PresentStructuredTeamDraft(current, current.Revision(), catalog, question)
	if err != nil {
		t.Fatalf("PresentStructuredTeamDraft() error = %v", err)
	}
	return got
}

func assertStructuredIdentity(t *testing.T, got StructuredTeamDraft, id string, revision int, state TeamDraftState, catalogDigest, contentDigest string) {
	t.Helper()
	if got.ID() != id || got.Revision() != revision || got.State() != state ||
		got.CatalogDigest() != catalogDigest || got.ContentDigest() != contentDigest {
		t.Fatalf("structured identity=(%q,%d,%q,%q,%q), want=(%q,%d,%q,%q,%q)",
			got.ID(), got.Revision(), got.State(), got.CatalogDigest(), got.ContentDigest(),
			id, revision, state, catalogDigest, contentDigest)
	}
}

func assertStructuredQuestion(t *testing.T, got StructuredTeamDraft, id, prompt string) {
	t.Helper()
	question, ok := got.Question()
	if !ok || question.ID != id || question.Prompt != prompt {
		t.Fatalf("Question()=(%#v,%v), want %q/%q", question, ok, id, prompt)
	}
}

func assertZeroStructuredTeamDraft(t *testing.T, got StructuredTeamDraft) {
	t.Helper()
	if got.ID() != "" || got.Revision() != 0 || got.State() != "" ||
		got.CatalogDigest() != "" || got.ContentDigest() != "" ||
		got.BindingDigest() != "" || !reflect.DeepEqual(got.References(), TeamDraftReferences{}) {
		t.Fatalf("failed operation returned usable structured Draft %#v", got)
	}
}
