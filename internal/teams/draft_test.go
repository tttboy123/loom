package teams

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/runtime"
)

func TestTeamDraftRevision(t *testing.T) {
	catalog := draftCatalogFixture(t, nil)
	otherCatalog := draftCatalogFixture(t, []string{"skill.extra"})
	references := draftReferencesFixture()

	t.Run("constructs immutable normalized revision one", func(t *testing.T) {
		input := cloneDraftReferences(references)
		draft, err := NewTeamDraft("draft.alpha", catalog, input)
		if err != nil {
			t.Fatalf("NewTeamDraft() error = %v", err)
		}
		assertDraftIdentity(t, draft, "draft.alpha", 1, TeamDraftStateDraft, catalog.Digest())
		if _, ok := draft.Question(); ok {
			t.Fatal("Question() present on a new draft")
		}
		if draft.LastAnsweredQuestionID() != "" || draft.LastAnswerText() != "" {
			t.Fatalf("new draft has answer metadata: question=%q answer=%q", draft.LastAnsweredQuestionID(), draft.LastAnswerText())
		}
		assertNormalizedDraftReferences(t, draft.References())

		input.SubAgentDefinitionIDs[0] = "agent.mutated"
		input.RuntimeInstanceIDs[0] = "runtime.mutated"
		input.RuntimeModels[0].ModelID = "model.mutated"
		input.SkillIDs[0] = "skill.mutated"
		input.MemberIDs[0] = "member.mutated"
		input.PermissionIDs[0] = "permission.mutated"
		assertNormalizedDraftReferences(t, draft.References())

		returned := draft.References()
		returned.SubAgentDefinitionIDs[0] = "agent.returned-mutated"
		returned.RuntimeInstanceIDs[0] = "runtime.returned-mutated"
		returned.RuntimeModels[0].ModelID = "model.returned-mutated"
		returned.SkillIDs[0] = "skill.returned-mutated"
		returned.MemberIDs[0] = "member.returned-mutated"
		returned.PermissionIDs[0] = "permission.returned-mutated"
		assertNormalizedDraftReferences(t, draft.References())
	})

	t.Run("rejects invalid construction and wraps every catalog reference class", func(t *testing.T) {
		tests := []struct {
			name    string
			id      string
			catalog TeamDraftCatalogSnapshot
			refs    TeamDraftReferences
			want    error
		}{
			{name: "empty id", catalog: catalog, refs: references, want: ErrInvalidTeamDraft},
			{name: "zero catalog", id: "draft.alpha", refs: references, want: ErrInvalidTeamDraftCatalog},
			{name: "missing main agent", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.MainAgentDefinitionID = ""
			}), want: ErrInvalidTeamDraftReference},
			{name: "invented main agent", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.MainAgentDefinitionID = "agent.invented"
			}), want: ErrInventedTeamDraftReference},
			{name: "duplicate subagent", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.SubAgentDefinitionIDs = []string{"agent.sub.a", "agent.sub.a"}
			}), want: ErrDuplicateTeamDraftReference},
			{name: "invented runtime", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.RuntimeInstanceIDs = []string{"runtime.invented"}
			}), want: ErrInventedTeamDraftReference},
			{name: "duplicate runtime", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.RuntimeInstanceIDs = []string{"runtime.local", "runtime.local"}
			}), want: ErrDuplicateTeamDraftReference},
			{name: "runtime model mismatch", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.RuntimeInstanceIDs = append(refs.RuntimeInstanceIDs, "runtime.other")
				refs.RuntimeModels[0] = RuntimeModelReference{RuntimeInstanceID: "runtime.other", ModelID: "model.alpha"}
			}), want: ErrRuntimeModelPairMismatch},
			{name: "duplicate skill", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.SkillIDs = []string{"skill.a", "skill.a"}
			}), want: ErrDuplicateTeamDraftReference},
			{name: "invented skill", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.SkillIDs = []string{"skill.invented"}
			}), want: ErrInventedTeamDraftReference},
			{name: "duplicate member", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.MemberIDs = []string{"member.a", "member.a"}
			}), want: ErrDuplicateTeamDraftReference},
			{name: "invented member", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.MemberIDs = []string{"member.invented"}
			}), want: ErrInventedTeamDraftReference},
			{name: "duplicate permission", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.PermissionIDs = []string{"permission.read", "permission.read"}
			}), want: ErrDuplicateTeamDraftReference},
			{name: "invented permission", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.PermissionIDs = []string{"permission.invented"}
			}), want: ErrInventedTeamDraftReference},
			{name: "budget ceiling", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.RequestedBudget = 101
			}), want: ErrBudgetCeilingExceeded},
			{name: "concurrency ceiling", id: "draft.alpha", catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.RequestedConcurrency = 3
			}), want: ErrConcurrencyCeilingExceeded},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				draft, err := NewTeamDraft(tt.id, tt.catalog, tt.refs)
				if !errors.Is(err, tt.want) {
					t.Fatalf("NewTeamDraft() error = %v, want errors.Is(%v)", err, tt.want)
				}
				assertZeroTeamDraft(t, draft)
			})
		}
	})

	t.Run("presents draft once with zero or one question", func(t *testing.T) {
		base := mustNewTeamDraft(t, "draft.alpha", catalog, references)

		proposed, err := PresentTeamDraft(base, 1, catalog, nil)
		if err != nil {
			t.Fatalf("PresentTeamDraft(no question) error = %v", err)
		}
		assertDraftIdentity(t, proposed, "draft.alpha", 2, TeamDraftStateProposed, catalog.Digest())
		if _, ok := proposed.Question(); ok {
			t.Fatal("proposed draft retained a question")
		}
		assertNormalizedDraftReferences(t, proposed.References())

		questionInput := &DraftQuestion{ID: "question.scope", Prompt: "Which scope?"}
		awaiting, err := PresentTeamDraft(base, 1, catalog, questionInput)
		if err != nil {
			t.Fatalf("PresentTeamDraft(question) error = %v", err)
		}
		assertDraftIdentity(t, awaiting, "draft.alpha", 2, TeamDraftStateAwaitingAnswer, catalog.Digest())
		assertDraftQuestion(t, awaiting, "question.scope", "Which scope?")
		questionInput.ID = "question.mutated"
		questionInput.Prompt = "mutated"
		assertDraftQuestion(t, awaiting, "question.scope", "Which scope?")

		returned, ok := awaiting.Question()
		if !ok {
			t.Fatal("Question() missing")
		}
		returned.ID = "question.returned-mutated"
		returned.Prompt = "returned mutated"
		assertDraftQuestion(t, awaiting, "question.scope", "Which scope?")

		for _, tt := range []struct {
			name     string
			current  TeamDraft
			revision int
			catalog  TeamDraftCatalogSnapshot
			question *DraftQuestion
			want     error
		}{
			{name: "stale", current: base, revision: 2, catalog: catalog, want: ErrStaleTeamDraftRevision},
			{name: "catalog mismatch", current: base, revision: 1, catalog: otherCatalog, want: ErrTeamDraftCatalogMismatch},
			{name: "invalid question id", current: base, revision: 1, catalog: catalog, question: &DraftQuestion{Prompt: "prompt"}, want: ErrInvalidDraftQuestion},
			{name: "invalid question prompt", current: base, revision: 1, catalog: catalog, question: &DraftQuestion{ID: "question.scope"}, want: ErrInvalidDraftQuestion},
			{name: "wrong state", current: proposed, revision: 2, catalog: catalog, want: ErrInvalidTeamDraftStateTransition},
		} {
			t.Run(tt.name, func(t *testing.T) {
				next, err := PresentTeamDraft(tt.current, tt.revision, tt.catalog, tt.question)
				if !errors.Is(err, tt.want) {
					t.Fatalf("PresentTeamDraft() error = %v, want errors.Is(%v)", err, tt.want)
				}
				assertZeroTeamDraft(t, next)
			})
		}
		assertDraftIdentity(t, base, "draft.alpha", 1, TeamDraftStateDraft, catalog.Digest())
	})

	t.Run("answers exact question and preserves prior revisions", func(t *testing.T) {
		base := mustNewTeamDraft(t, "draft.alpha", catalog, references)
		awaiting := mustPresentTeamDraft(t, base, catalog, &DraftQuestion{ID: "question.scope", Prompt: "Which scope?"})
		revised := mutateDraftReferences(references, func(refs *TeamDraftReferences) {
			refs.SubAgentDefinitionIDs = []string{"agent.sub.a"}
			refs.SkillIDs = []string{"skill.z", "skill.a"}
		})

		proposed, err := AnswerTeamDraft(awaiting, 2, catalog, "question.scope", "Project only", revised, nil)
		if err != nil {
			t.Fatalf("AnswerTeamDraft() error = %v", err)
		}
		assertDraftIdentity(t, proposed, "draft.alpha", 3, TeamDraftStateProposed, catalog.Digest())
		if proposed.LastAnsweredQuestionID() != "question.scope" || proposed.LastAnswerText() != "Project only" {
			t.Fatalf("answer metadata = (%q, %q), want exact answer", proposed.LastAnsweredQuestionID(), proposed.LastAnswerText())
		}
		if _, ok := proposed.Question(); ok {
			t.Fatal("proposed answer retained unresolved question")
		}
		if got := proposed.References().SubAgentDefinitionIDs; !reflect.DeepEqual(got, []string{"agent.sub.a"}) {
			t.Fatalf("SubAgentDefinitionIDs = %#v, want revised references", got)
		}

		nextQuestion := &DraftQuestion{ID: "question.budget", Prompt: "Confirm budget?"}
		next, err := AnswerTeamDraft(awaiting, 2, catalog, "question.scope", "Project only", revised, nextQuestion)
		if err != nil {
			t.Fatalf("AnswerTeamDraft(next question) error = %v", err)
		}
		assertDraftIdentity(t, next, "draft.alpha", 3, TeamDraftStateAwaitingAnswer, catalog.Digest())
		assertDraftQuestion(t, next, "question.budget", "Confirm budget?")
		nextQuestion.ID = "question.mutated"
		nextQuestion.Prompt = "mutated"
		assertDraftQuestion(t, next, "question.budget", "Confirm budget?")

		assertDraftIdentity(t, awaiting, "draft.alpha", 2, TeamDraftStateAwaitingAnswer, catalog.Digest())
		assertDraftQuestion(t, awaiting, "question.scope", "Which scope?")
		if awaiting.LastAnsweredQuestionID() != "" || awaiting.LastAnswerText() != "" {
			t.Fatal("prior revision was mutated with answer metadata")
		}
	})

	t.Run("answer failures are typed and return no revision", func(t *testing.T) {
		base := mustNewTeamDraft(t, "draft.alpha", catalog, references)
		awaiting := mustPresentTeamDraft(t, base, catalog, &DraftQuestion{ID: "question.scope", Prompt: "Which scope?"})
		proposed := mustPresentTeamDraft(t, base, catalog, nil)

		tests := []struct {
			name       string
			current    TeamDraft
			revision   int
			catalog    TeamDraftCatalogSnapshot
			questionID string
			answer     string
			refs       TeamDraftReferences
			next       *DraftQuestion
			want       error
		}{
			{name: "stale", current: awaiting, revision: 1, catalog: catalog, questionID: "question.scope", answer: "answer", refs: references, want: ErrStaleTeamDraftRevision},
			{name: "catalog mismatch", current: awaiting, revision: 2, catalog: otherCatalog, questionID: "question.scope", answer: "answer", refs: references, want: ErrTeamDraftCatalogMismatch},
			{name: "wrong question", current: awaiting, revision: 2, catalog: catalog, questionID: "question.other", answer: "answer", refs: references, want: ErrTeamDraftQuestionMismatch},
			{name: "empty answer", current: awaiting, revision: 2, catalog: catalog, questionID: "question.scope", refs: references, want: ErrInvalidDraftAnswer},
			{name: "invalid next question", current: awaiting, revision: 2, catalog: catalog, questionID: "question.scope", answer: "answer", refs: references, next: &DraftQuestion{ID: "question.next"}, want: ErrInvalidDraftQuestion},
			{name: "invalid references", current: awaiting, revision: 2, catalog: catalog, questionID: "question.scope", answer: "answer", refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.SkillIDs = []string{"skill.invented"}
			}), want: ErrInventedTeamDraftReference},
			{name: "wrong draft state", current: base, revision: 1, catalog: catalog, questionID: "question.scope", answer: "answer", refs: references, want: ErrInvalidTeamDraftStateTransition},
			{name: "wrong proposed state", current: proposed, revision: 2, catalog: catalog, questionID: "question.scope", answer: "answer", refs: references, want: ErrInvalidTeamDraftStateTransition},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				next, err := AnswerTeamDraft(tt.current, tt.revision, tt.catalog, tt.questionID, tt.answer, tt.refs, tt.next)
				if !errors.Is(err, tt.want) {
					t.Fatalf("AnswerTeamDraft() error = %v, want errors.Is(%v)", err, tt.want)
				}
				assertZeroTeamDraft(t, next)
			})
		}
		assertDraftIdentity(t, awaiting, "draft.alpha", 2, TeamDraftStateAwaitingAnswer, catalog.Digest())
		assertDraftQuestion(t, awaiting, "question.scope", "Which scope?")
	})

	t.Run("direct edits normalize references and preserve prior answer metadata", func(t *testing.T) {
		base := mustNewTeamDraft(t, "draft.alpha", catalog, references)
		first := mustPresentTeamDraft(t, base, catalog, &DraftQuestion{ID: "question.scope", Prompt: "Which scope?"})
		answered, err := AnswerTeamDraft(first, 2, catalog, "question.scope", "Project only", references, &DraftQuestion{
			ID: "question.budget", Prompt: "Confirm budget?",
		})
		if err != nil {
			t.Fatalf("AnswerTeamDraft() error = %v", err)
		}
		reordered := mutateDraftReferences(references, func(refs *TeamDraftReferences) {
			refs.SubAgentDefinitionIDs = []string{"agent.sub.a", "agent.sub.b"}
			refs.SkillIDs = []string{"skill.a", "skill.z"}
			refs.MemberIDs = []string{"member.a", "member.z"}
			refs.PermissionIDs = []string{"permission.read", "permission.write"}
		})
		edited, err := EditTeamDraft(answered, 3, catalog, reordered, nil)
		if err != nil {
			t.Fatalf("EditTeamDraft() error = %v", err)
		}
		assertDraftIdentity(t, edited, "draft.alpha", 4, TeamDraftStateProposed, catalog.Digest())
		assertNormalizedDraftReferences(t, edited.References())
		if edited.LastAnsweredQuestionID() != "question.scope" || edited.LastAnswerText() != "Project only" {
			t.Fatalf("edit changed answer metadata: (%q, %q)", edited.LastAnsweredQuestionID(), edited.LastAnswerText())
		}

		question := &DraftQuestion{ID: "question.final", Prompt: "Final choice?"}
		reopened, err := EditTeamDraft(edited, 4, catalog, references, question)
		if err != nil {
			t.Fatalf("EditTeamDraft(question) error = %v", err)
		}
		assertDraftIdentity(t, reopened, "draft.alpha", 5, TeamDraftStateAwaitingAnswer, catalog.Digest())
		assertDraftQuestion(t, reopened, "question.final", "Final choice?")
		if reopened.LastAnsweredQuestionID() != "question.scope" || reopened.LastAnswerText() != "Project only" {
			t.Fatal("edit with question changed prior answer metadata")
		}

		for _, tt := range []struct {
			name     string
			current  TeamDraft
			revision int
			catalog  TeamDraftCatalogSnapshot
			refs     TeamDraftReferences
			question *DraftQuestion
			want     error
		}{
			{name: "draft state", current: base, revision: 1, catalog: catalog, refs: references, want: ErrInvalidTeamDraftStateTransition},
			{name: "stale", current: edited, revision: 3, catalog: catalog, refs: references, want: ErrStaleTeamDraftRevision},
			{name: "catalog mismatch", current: edited, revision: 4, catalog: otherCatalog, refs: references, want: ErrTeamDraftCatalogMismatch},
			{name: "invalid references", current: edited, revision: 4, catalog: catalog, refs: mutateDraftReferences(references, func(refs *TeamDraftReferences) {
				refs.MemberIDs = []string{"member.invented"}
			}), want: ErrInventedTeamDraftReference},
			{name: "invalid question", current: edited, revision: 4, catalog: catalog, refs: references, question: &DraftQuestion{Prompt: "missing id"}, want: ErrInvalidDraftQuestion},
		} {
			t.Run(tt.name, func(t *testing.T) {
				next, err := EditTeamDraft(tt.current, tt.revision, tt.catalog, tt.refs, tt.question)
				if !errors.Is(err, tt.want) {
					t.Fatalf("EditTeamDraft() error = %v, want errors.Is(%v)", err, tt.want)
				}
				assertZeroTeamDraft(t, next)
			})
		}
	})
}

func TestCheckTeamDraftAcceptable(t *testing.T) {
	catalog := draftCatalogFixture(t, nil)
	otherCatalog := draftCatalogFixture(t, []string{"skill.extra"})
	references := draftReferencesFixture()
	base := mustNewTeamDraft(t, "draft.alpha", catalog, references)
	proposed := mustPresentTeamDraft(t, base, catalog, nil)
	awaiting := mustPresentTeamDraft(t, base, catalog, &DraftQuestion{ID: "question.scope", Prompt: "Which scope?"})

	candidate, err := CheckTeamDraftAcceptable(proposed, 2, catalog)
	if err != nil {
		t.Fatalf("CheckTeamDraftAcceptable() error = %v", err)
	}
	want := TeamDraftAcceptanceCandidate{
		Eligible:      true,
		DraftID:       "draft.alpha",
		Revision:      2,
		CatalogDigest: catalog.Digest(),
	}
	if !reflect.DeepEqual(candidate, want) {
		t.Fatalf("CheckTeamDraftAcceptable() = %#v, want %#v", candidate, want)
	}
	assertDraftIdentity(t, proposed, "draft.alpha", 2, TeamDraftStateProposed, catalog.Digest())

	tests := []struct {
		name     string
		draft    TeamDraft
		revision int
		catalog  TeamDraftCatalogSnapshot
		want     error
	}{
		{name: "stale", draft: proposed, revision: 1, catalog: catalog, want: ErrStaleTeamDraftRevision},
		{name: "catalog mismatch", draft: proposed, revision: 2, catalog: otherCatalog, want: ErrTeamDraftCatalogMismatch},
		{name: "draft state", draft: base, revision: 1, catalog: catalog, want: ErrInvalidTeamDraftStateTransition},
		{name: "awaiting unresolved question", draft: awaiting, revision: 2, catalog: catalog, want: ErrInvalidTeamDraftStateTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckTeamDraftAcceptable(tt.draft, tt.revision, tt.catalog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("CheckTeamDraftAcceptable() error = %v, want errors.Is(%v)", err, tt.want)
			}
			if got != (TeamDraftAcceptanceCandidate{}) {
				t.Fatalf("failed CheckTeamDraftAcceptable() returned usable Candidate %#v", got)
			}
		})
	}
}

func TestTeamDraftRevisionImportBoundary(t *testing.T) {
	source, err := os.ReadFile("draft.go")
	if err != nil {
		t.Fatalf("ReadFile(draft.go): %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "draft.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(draft.go): %v", err)
	}
	allowed := map[string]bool{
		`"errors"`: true,
		`"fmt"`:    true,
		`"sort"`:   true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("draft.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func draftCatalogFixture(t *testing.T, extraSkills []string) TeamDraftCatalogSnapshot {
	t.Helper()
	definitions := []agents.AgentDefinition{
		mustCatalogAgent(t, agents.AgentDefinition{
			ID: "agent.main", Version: 1, Scope: agents.ScopeReusable,
			Name: "Main", RoleSpec: "Coordinate the proposed team.", Status: agents.DefinitionActive,
		}),
		mustCatalogAgent(t, agents.AgentDefinition{
			ID: "agent.sub.a", Version: 1, Scope: agents.ScopeReusable,
			Name: "Sub A", RoleSpec: "Perform bounded task A.", Status: agents.DefinitionActive,
		}),
		mustCatalogAgent(t, agents.AgentDefinition{
			ID: "agent.sub.b", Version: 1, Scope: agents.ScopeReusable,
			Name: "Sub B", RoleSpec: "Perform bounded task B.", Status: agents.DefinitionActive,
		}),
	}
	discovery := mustCatalogRuntimeDiscovery(t, []runtime.RuntimeProbe{
		catalogRuntimeProbe("probe.local", runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.local", func(instance *runtime.RuntimeInstance) {
				instance.ObservedCapabilities = []string{"apply_patch", "go_test"}
				instance.Capacity = 2
			}),
			ModelIDs: []string{"model.beta", "model.alpha"},
		}),
		catalogRuntimeProbe("probe.other", runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.other", func(instance *runtime.RuntimeInstance) {
				instance.ObservedCapabilities = []string{"go_test"}
				instance.Capacity = 1
			}),
			ModelIDs: []string{"model.gamma"},
		}),
	})
	skills := append([]string{"skill.z", "skill.a"}, extraSkills...)
	return mustTeamDraftCatalog(t, TeamDraftCatalogInput{
		AgentDefinitions:   definitions,
		RuntimeDiscovery:   discovery,
		SkillIDs:           skills,
		MemberIDs:          []string{"member.z", "member.a"},
		PermissionIDs:      []string{"permission.write", "permission.read"},
		BudgetCeiling:      100,
		ConcurrencyCeiling: 2,
		MaxCounts: TeamDraftCatalogMaxCounts{
			Agents: 3, Runtimes: 2, Models: 3,
			Skills: 3, Members: 2, Permissions: 2,
		},
	})
}

func draftReferencesFixture() TeamDraftReferences {
	return TeamDraftReferences{
		MainAgentDefinitionID: "agent.main",
		SubAgentDefinitionIDs: []string{"agent.sub.b", "agent.sub.a"},
		RuntimeInstanceIDs:    []string{"runtime.local"},
		RuntimeModels: []RuntimeModelReference{
			{RuntimeInstanceID: "runtime.local", ModelID: "model.beta"},
			{RuntimeInstanceID: "runtime.local", ModelID: "model.alpha"},
		},
		SkillIDs:             []string{"skill.z", "skill.a"},
		MemberIDs:            []string{"member.z", "member.a"},
		PermissionIDs:        []string{"permission.write", "permission.read"},
		RequestedBudget:      50,
		RequestedConcurrency: 2,
	}
}

func mutateDraftReferences(input TeamDraftReferences, change func(*TeamDraftReferences)) TeamDraftReferences {
	clone := cloneDraftReferences(input)
	change(&clone)
	return clone
}

func cloneDraftReferences(input TeamDraftReferences) TeamDraftReferences {
	input.SubAgentDefinitionIDs = append([]string(nil), input.SubAgentDefinitionIDs...)
	input.RuntimeInstanceIDs = append([]string(nil), input.RuntimeInstanceIDs...)
	input.RuntimeModels = append([]RuntimeModelReference(nil), input.RuntimeModels...)
	input.SkillIDs = append([]string(nil), input.SkillIDs...)
	input.MemberIDs = append([]string(nil), input.MemberIDs...)
	input.PermissionIDs = append([]string(nil), input.PermissionIDs...)
	return input
}

func mustNewTeamDraft(t *testing.T, id string, catalog TeamDraftCatalogSnapshot, refs TeamDraftReferences) TeamDraft {
	t.Helper()
	draft, err := NewTeamDraft(id, catalog, refs)
	if err != nil {
		t.Fatalf("NewTeamDraft() error = %v", err)
	}
	return draft
}

func mustPresentTeamDraft(t *testing.T, current TeamDraft, catalog TeamDraftCatalogSnapshot, question *DraftQuestion) TeamDraft {
	t.Helper()
	next, err := PresentTeamDraft(current, current.Revision(), catalog, question)
	if err != nil {
		t.Fatalf("PresentTeamDraft() error = %v", err)
	}
	return next
}

func assertDraftIdentity(t *testing.T, draft TeamDraft, id string, revision int, state TeamDraftState, digest string) {
	t.Helper()
	if draft.ID() != id || draft.Revision() != revision || draft.State() != state || draft.CatalogDigest() != digest {
		t.Fatalf("draft identity = (%q, %d, %q, %q), want (%q, %d, %q, %q)",
			draft.ID(), draft.Revision(), draft.State(), draft.CatalogDigest(),
			id, revision, state, digest)
	}
}

func assertDraftQuestion(t *testing.T, draft TeamDraft, id, prompt string) {
	t.Helper()
	question, ok := draft.Question()
	if !ok {
		t.Fatal("Question() missing")
	}
	if question.ID != id || question.Prompt != prompt {
		t.Fatalf("Question() = %#v, want ID=%q Prompt=%q", question, id, prompt)
	}
}

func assertNormalizedDraftReferences(t *testing.T, got TeamDraftReferences) {
	t.Helper()
	want := TeamDraftReferences{
		MainAgentDefinitionID: "agent.main",
		SubAgentDefinitionIDs: []string{"agent.sub.a", "agent.sub.b"},
		RuntimeInstanceIDs:    []string{"runtime.local"},
		RuntimeModels: []RuntimeModelReference{
			{RuntimeInstanceID: "runtime.local", ModelID: "model.alpha"},
			{RuntimeInstanceID: "runtime.local", ModelID: "model.beta"},
		},
		SkillIDs:             []string{"skill.a", "skill.z"},
		MemberIDs:            []string{"member.a", "member.z"},
		PermissionIDs:        []string{"permission.read", "permission.write"},
		RequestedBudget:      50,
		RequestedConcurrency: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("References() = %#v, want normalized %#v", got, want)
	}
}

func assertZeroTeamDraft(t *testing.T, draft TeamDraft) {
	t.Helper()
	if draft.ID() != "" || draft.Revision() != 0 || draft.State() != "" ||
		draft.CatalogDigest() != "" || !reflect.DeepEqual(draft.References(), TeamDraftReferences{}) ||
		draft.LastAnsweredQuestionID() != "" || draft.LastAnswerText() != "" {
		t.Fatalf("failed operation returned usable Draft %#v", draft)
	}
	if _, ok := draft.Question(); ok {
		t.Fatalf("zero Draft returned a question")
	}
}
