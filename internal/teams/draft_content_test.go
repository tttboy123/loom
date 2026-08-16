package teams

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/runtime"
)

func TestTeamDraftContent(t *testing.T) {
	catalog, input := draftContentFixture(t)

	t.Run("builds immutable normalized gap-free content", func(t *testing.T) {
		mutableInput := cloneTeamDraftContentInput(input)
		original := cloneTeamDraftContentInput(mutableInput)
		snapshot, err := BuildTeamDraftContent(catalog, mutableInput)
		if err != nil {
			t.Fatalf("BuildTeamDraftContent() error = %v", err)
		}
		assertSHA256Digest(t, snapshot.Digest())
		if snapshot.CatalogDigest() != catalog.Digest() {
			t.Fatalf("CatalogDigest() = %q, want %q", snapshot.CatalogDigest(), catalog.Digest())
		}
		assertNormalizedDraftContent(t, snapshot)
		if !reflect.DeepEqual(mutableInput, original) {
			t.Fatalf("BuildTeamDraftContent() mutated input:\n got %#v\nwant %#v", mutableInput, original)
		}

		mutateDraftContentInput(&mutableInput)
		assertNormalizedDraftContent(t, snapshot)

		roles := snapshot.Roles()
		*roles[0].RuntimeProfile.Budget = 999
		roles[0].RuntimeProfile.RequiredCapabilities[0] = "mutated"
		roles[0].SkillIDs[0] = "mutated"
		roles[0].MemberIDs[0] = "mutated"
		roles[0].PermissionIDs[0] = "mutated"
		tasks := snapshot.Tasks()
		tasks[0].DependencyTaskIDs = append(tasks[0].DependencyTaskIDs, "mutated")
		tasks[0].AcceptanceCriteria[0] = "mutated"
		markers := snapshot.ApprovalMarkers()
		markers[0].Reason = "mutated"
		assertNormalizedDraftContent(t, snapshot)
	})

	t.Run("digest ignores set ordering and changes with semantic content", func(t *testing.T) {
		baseline := mustTeamDraftContent(t, catalog, input)
		reordered := cloneTeamDraftContentInput(input)
		slices.Reverse(reordered.Roles)
		slices.Reverse(reordered.Tasks)
		slices.Reverse(reordered.Tasks[2].DependencyTaskIDs)
		slices.Reverse(reordered.Tasks[2].AcceptanceCriteria)
		slices.Reverse(reordered.ApprovalMarkers)
		slices.Reverse(reordered.References.SubAgentDefinitionIDs)
		slices.Reverse(reordered.References.RuntimeModels)
		slices.Reverse(reordered.References.SkillIDs)
		if got := mustTeamDraftContent(t, catalog, reordered).Digest(); got != baseline.Digest() {
			t.Fatalf("reordered digest = %q, want %q", got, baseline.Digest())
		}

		changes := []struct {
			name   string
			change func(*TeamDraftContentInput)
		}{
			{name: "references", change: func(in *TeamDraftContentInput) { in.References.RequestedBudget++ }},
			{name: "runtime profile", change: func(in *TeamDraftContentInput) { in.Roles[0].RuntimeProfile.ProviderID = "provider.changed" }},
			{name: "task dependency", change: func(in *TeamDraftContentInput) {
				in.Tasks[2].DependencyTaskIDs = []string{"task.beta"}
			}},
			{name: "task criterion", change: func(in *TeamDraftContentInput) { in.Tasks[0].AcceptanceCriteria[0] = "changed criterion" }},
			{name: "rule summary", change: func(in *TeamDraftContentInput) { in.CustomerRuleSummary = "Changed rules." }},
			{name: "approval marker", change: func(in *TeamDraftContentInput) { in.ApprovalMarkers[0].Reason = "changed reason" }},
			{name: "limit", change: func(in *TeamDraftContentInput) { in.Limits.MaxTasks++ }},
		}
		for _, tt := range changes {
			t.Run(tt.name, func(t *testing.T) {
				changed := cloneTeamDraftContentInput(input)
				tt.change(&changed)
				got := mustTeamDraftContent(t, catalog, changed)
				if got.Digest() == baseline.Digest() {
					t.Fatalf("semantic %s change did not change digest", tt.name)
				}
			})
		}

		withGap := cloneTeamDraftContentInput(input)
		withGap.CapabilityGaps = []TeamDraftCapabilityGap{{Capability: "gpu", Reason: "not discovered"}}
		gapped := mustTeamDraftContent(t, catalog, withGap)
		if gapped.Digest() == baseline.Digest() {
			t.Fatal("capability gap did not change digest")
		}
		candidate, err := ValidateTeamDraftContent(gapped, catalog)
		if err != nil {
			t.Fatalf("ValidateTeamDraftContent(gapped) error = %v", err)
		}
		if !candidate.Valid || candidate.AcceptanceReady {
			t.Fatalf("gapped Candidate = %#v, want Valid and not AcceptanceReady", candidate)
		}
		returnedGaps := gapped.CapabilityGaps()
		returnedGaps[0].Reason = "mutated"
		if got := gapped.CapabilityGaps()[0].Reason; got != "not discovered" {
			t.Fatalf("CapabilityGaps accessor mutation changed snapshot: %q", got)
		}

		twoGaps := cloneTeamDraftContentInput(input)
		twoGaps.CapabilityGaps = []TeamDraftCapabilityGap{
			{Capability: "gpu", Reason: "not discovered"},
			{Capability: "network", Reason: "not permitted"},
		}
		firstGapOrder := mustTeamDraftContent(t, catalog, twoGaps)
		slices.Reverse(twoGaps.CapabilityGaps)
		if got := mustTeamDraftContent(t, catalog, twoGaps).Digest(); got != firstGapOrder.Digest() {
			t.Fatalf("reordered capability gaps changed digest: got %q want %q", got, firstGapOrder.Digest())
		}
	})
}

func TestTeamDraftContentLimits(t *testing.T) {
	catalog, input := draftContentFixture(t)
	mustTeamDraftContent(t, catalog, input)

	t.Run("rejects zero catalog and invalid limits", func(t *testing.T) {
		snapshot, err := BuildTeamDraftContent(TeamDraftCatalogSnapshot{}, input)
		if !errors.Is(err, ErrInvalidTeamDraftCatalog) {
			t.Fatalf("BuildTeamDraftContent(zero catalog) error = %v, want ErrInvalidTeamDraftCatalog", err)
		}
		assertZeroTeamDraftContent(t, snapshot)

		tests := []struct {
			name   string
			change func(*TeamDraftContentLimits)
		}{
			{name: "tasks", change: func(l *TeamDraftContentLimits) { l.MaxTasks = 0 }},
			{name: "dependencies", change: func(l *TeamDraftContentLimits) { l.MaxDependenciesPerTask = 0 }},
			{name: "criteria", change: func(l *TeamDraftContentLimits) { l.MaxAcceptanceCriteriaPerTask = 0 }},
			{name: "markers", change: func(l *TeamDraftContentLimits) { l.MaxApprovalMarkers = 0 }},
			{name: "gaps", change: func(l *TeamDraftContentLimits) { l.MaxCapabilityGaps = 0 }},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				changed := cloneTeamDraftContentInput(input)
				tt.change(&changed.Limits)
				got, err := BuildTeamDraftContent(catalog, changed)
				if !errors.Is(err, ErrInvalidTeamDraftContent) {
					t.Fatalf("BuildTeamDraftContent() error = %v, want ErrInvalidTeamDraftContent", err)
				}
				assertZeroTeamDraftContent(t, got)
			})
		}
	})

	t.Run("accepts exact maxima and rejects max plus one", func(t *testing.T) {
		exact := cloneTeamDraftContentInput(input)
		exact.CapabilityGaps = []TeamDraftCapabilityGap{
			{Capability: "cap.a", Reason: "missing a"},
			{Capability: "cap.b", Reason: "missing b"},
		}
		mustTeamDraftContent(t, catalog, exact)

		tests := []struct {
			name   string
			change func(*TeamDraftContentInput)
		}{
			{name: "tasks", change: func(in *TeamDraftContentInput) {
				in.Tasks = append(in.Tasks, TeamDraftTaskCandidate{
					ID: "task.delta", OwnerAgentDefinitionID: "agent.sub.a",
					AcceptanceCriteria: []string{"delta passes"},
				})
			}},
			{name: "dependencies", change: func(in *TeamDraftContentInput) {
				in.Limits.MaxTasks = 4
				in.Tasks = append(in.Tasks, TeamDraftTaskCandidate{
					ID: "task.delta", OwnerAgentDefinitionID: "agent.sub.a",
					AcceptanceCriteria: []string{"delta passes"},
				})
				in.Tasks[2].DependencyTaskIDs = []string{"task.alpha", "task.beta", "task.delta"}
			}},
			{name: "criteria", change: func(in *TeamDraftContentInput) {
				in.Tasks[2].AcceptanceCriteria = []string{"one", "two", "three"}
			}},
			{name: "markers", change: func(in *TeamDraftContentInput) {
				in.ApprovalMarkers = append(in.ApprovalMarkers, TeamDraftApprovalMarker{ID: "approval.three", Reason: "third"})
			}},
			{name: "gaps", change: func(in *TeamDraftContentInput) {
				in.CapabilityGaps = []TeamDraftCapabilityGap{
					{Capability: "cap.a", Reason: "missing a"},
					{Capability: "cap.b", Reason: "missing b"},
					{Capability: "cap.c", Reason: "missing c"},
				}
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				changed := cloneTeamDraftContentInput(input)
				tt.change(&changed)
				got, err := BuildTeamDraftContent(catalog, changed)
				if !errors.Is(err, ErrTeamDraftContentLimitExceeded) {
					t.Fatalf("BuildTeamDraftContent() error = %v, want ErrTeamDraftContentLimitExceeded", err)
				}
				assertZeroTeamDraftContent(t, got)
			})
		}
	})
}

func TestTeamDraftContentRoles(t *testing.T) {
	catalog, input := draftContentFixture(t)

	t.Run("accepts minimum one subagent with one task", func(t *testing.T) {
		minimum := cloneTeamDraftContentInput(input)
		minimum.References.SubAgentDefinitionIDs = []string{"agent.sub.a"}
		minimum.Roles = minimum.Roles[:2]
		minimum.Tasks = []TeamDraftTaskCandidate{{
			ID: "task.alpha", OwnerAgentDefinitionID: "agent.sub.a",
			AcceptanceCriteria: []string{"alpha passes"},
		}}
		snapshot := mustTeamDraftContent(t, catalog, minimum)
		candidate, err := ValidateTeamDraftContent(snapshot, catalog)
		if err != nil {
			t.Fatalf("ValidateTeamDraftContent(minimum) error = %v", err)
		}
		if !candidate.Valid || !candidate.AcceptanceReady ||
			candidate.RoleCount != 2 || candidate.TaskCount != 1 {
			t.Fatalf("minimum Candidate = %#v, want valid ready roles=2 tasks=1", candidate)
		}
	})

	tests := []struct {
		name    string
		catalog TeamDraftCatalogSnapshot
		change  func(*TeamDraftContentInput)
		want    error
	}{
		{name: "main only", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.References.SubAgentDefinitionIDs = nil
			in.References.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "runtime.local", ModelID: "model.alpha"}}
			in.Roles = in.Roles[:1]
			in.Tasks = nil
		}, want: ErrMissingTeamDraftRole},
		{name: "duplicate role", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[2] = cloneTeamDraftRoleSelection(in.Roles[1])
		}, want: ErrDuplicateTeamDraftRole},
		{name: "missing role", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles = in.Roles[:2]
		}, want: ErrMissingTeamDraftRole},
		{name: "extra role", catalog: draftContentCatalog(t, []string{"agent.main", "agent.sub.a", "agent.sub.b", "agent.extra"}), change: func(in *TeamDraftContentInput) {
			in.Roles = append(in.Roles, draftContentRole("agent.extra", "profile.extra", "model.alpha"))
		}, want: ErrExtraTeamDraftRole},
		{name: "invalid profile", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].RuntimeProfile.Timeout = 0
		}, want: runtime.ErrInvalidRuntimeProfile},
		{name: "adapter mismatch", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].RuntimeProfile.AdapterType = "adapter.other"
		}, want: runtime.ErrAdapterMismatch},
		{name: "capability mismatch", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].RuntimeProfile.RequiredCapabilities = []string{"capability.missing"}
		}, want: runtime.ErrMissingCapability},
		{name: "runtime not selected", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].RuntimeInstanceID = "runtime.other"
		}, want: ErrTeamDraftRuntimeCoverageMismatch},
		{name: "model mismatch", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].RuntimeProfile.ModelID = "model.gamma"
		}, want: ErrTeamDraftRuntimeProfileModelMismatch},
		{name: "duplicate role skill", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].SkillIDs = []string{"skill.a", "skill.a"}
		}, want: ErrInvalidTeamDraftRole},
		{name: "role permission outside references", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[0].PermissionIDs = []string{"permission.invented"}
		}, want: ErrInvalidTeamDraftRole},
		{name: "invented reference", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.References.SkillIDs = []string{"skill.invented"}
		}, want: ErrInventedTeamDraftReference},
		{name: "incomplete runtime model coverage", catalog: catalog, change: func(in *TeamDraftContentInput) {
			in.Roles[1].RuntimeProfile.ModelID = "model.alpha"
		}, want: ErrTeamDraftRuntimeCoverageMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneTeamDraftContentInput(input)
			tt.change(&changed)
			got, err := BuildTeamDraftContent(tt.catalog, changed)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildTeamDraftContent() error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroTeamDraftContent(t, got)
		})
	}

	t.Run("accepts four-agent Phase 2D team", func(t *testing.T) {
		largeCatalog := draftContentCatalog(t, []string{"agent.main", "agent.sub.a", "agent.sub.b", "agent.sub.c"})
		changed := cloneTeamDraftContentInput(input)
		changed.References.SubAgentDefinitionIDs = append(changed.References.SubAgentDefinitionIDs, "agent.sub.c")
		changed.Roles = append(changed.Roles, draftContentRole("agent.sub.c", "profile.sub.c", "model.alpha"))
		changed.Tasks = append(changed.Tasks, TeamDraftTaskCandidate{
			ID: "task.sub.c", OwnerAgentDefinitionID: "agent.sub.c",
			AcceptanceCriteria: []string{"sub c passes"},
		})
		changed.Limits.MaxTasks = 4
		got := mustTeamDraftContent(t, largeCatalog, changed)
		validated, err := ValidateTeamDraftContent(got, largeCatalog)
		if err != nil || !validated.Valid || validated.RoleCount != 4 {
			t.Fatalf("ValidateTeamDraftContent(4 agents) = (%#v, %v)", validated, err)
		}
	})

	t.Run("rejects max team agents plus one", func(t *testing.T) {
		changed := cloneTeamDraftContentInput(input)
		agentIDs := []string{"agent.main", "agent.sub.a", "agent.sub.b"}
		for index := len(agentIDs); index <= MaxTeamAgentCount; index++ {
			agentID := fmt.Sprintf("agent.sub.%d", index)
			agentIDs = append(agentIDs, agentID)
			changed.References.SubAgentDefinitionIDs = append(
				changed.References.SubAgentDefinitionIDs, agentID,
			)
		}
		got, err := BuildTeamDraftContent(draftContentCatalog(t, agentIDs), changed)
		if !errors.Is(err, ErrTeamDraftContentLimitExceeded) {
			t.Fatalf("BuildTeamDraftContent(max+1) error = %v, want ErrTeamDraftContentLimitExceeded", err)
		}
		assertZeroTeamDraftContent(t, got)
	})
}

func TestTeamDraftContentTasks(t *testing.T) {
	catalog, input := draftContentFixture(t)
	tests := []struct {
		name   string
		change func(*TeamDraftContentInput)
		want   error
	}{
		{name: "zero tasks", change: func(in *TeamDraftContentInput) { in.Tasks = nil }, want: ErrInvalidTeamDraftTask},
		{name: "empty task id", change: func(in *TeamDraftContentInput) { in.Tasks[0].ID = "" }, want: ErrInvalidTeamDraftTask},
		{name: "duplicate task", change: func(in *TeamDraftContentInput) { in.Tasks[1].ID = in.Tasks[0].ID }, want: ErrDuplicateTeamDraftTask},
		{name: "unknown owner", change: func(in *TeamDraftContentInput) { in.Tasks[0].OwnerAgentDefinitionID = "agent.unknown" }, want: ErrInvalidTeamDraftTask},
		{name: "main owns delivery", change: func(in *TeamDraftContentInput) { in.Tasks[0].OwnerAgentDefinitionID = "agent.main" }, want: ErrMainAgentDeliveryAssignment},
		{name: "unassigned subagent", change: func(in *TeamDraftContentInput) {
			for i := range in.Tasks {
				in.Tasks[i].OwnerAgentDefinitionID = "agent.sub.a"
			}
		}, want: ErrUnassignedTeamDraftSubAgent},
		{name: "missing criteria", change: func(in *TeamDraftContentInput) { in.Tasks[0].AcceptanceCriteria = nil }, want: ErrInvalidTeamDraftTask},
		{name: "duplicate criterion", change: func(in *TeamDraftContentInput) {
			in.Tasks[0].AcceptanceCriteria = []string{"same", "same"}
		}, want: ErrInvalidTeamDraftTask},
		{name: "unknown dependency", change: func(in *TeamDraftContentInput) {
			in.Tasks[1].DependencyTaskIDs = []string{"task.unknown"}
		}, want: ErrUnknownTeamDraftTaskDependency},
		{name: "self dependency", change: func(in *TeamDraftContentInput) {
			in.Tasks[1].DependencyTaskIDs = []string{in.Tasks[1].ID}
		}, want: ErrTeamDraftTaskSelfDependency},
		{name: "duplicate dependency", change: func(in *TeamDraftContentInput) {
			in.Tasks[2].DependencyTaskIDs = []string{"task.alpha", "task.alpha"}
		}, want: ErrInvalidTeamDraftTask},
		{name: "cycle", change: func(in *TeamDraftContentInput) {
			in.Tasks[0].DependencyTaskIDs = []string{"task.gamma"}
		}, want: ErrCyclicTeamDraftTaskGraph},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneTeamDraftContentInput(input)
			tt.change(&changed)
			got, err := BuildTeamDraftContent(catalog, changed)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildTeamDraftContent() error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroTeamDraftContent(t, got)
		})
	}
}

func TestTeamDraftContentAnnotations(t *testing.T) {
	catalog, input := draftContentFixture(t)
	tests := []struct {
		name   string
		change func(*TeamDraftContentInput)
		want   error
	}{
		{name: "empty rule summary", change: func(in *TeamDraftContentInput) { in.CustomerRuleSummary = "" }, want: ErrInvalidTeamDraftContent},
		{name: "empty marker id", change: func(in *TeamDraftContentInput) { in.ApprovalMarkers[0].ID = "" }, want: ErrInvalidTeamDraftApprovalMarker},
		{name: "empty marker reason", change: func(in *TeamDraftContentInput) { in.ApprovalMarkers[0].Reason = "" }, want: ErrInvalidTeamDraftApprovalMarker},
		{name: "duplicate marker", change: func(in *TeamDraftContentInput) { in.ApprovalMarkers[1].ID = in.ApprovalMarkers[0].ID }, want: ErrDuplicateTeamDraftApprovalMarker},
		{name: "empty gap capability", change: func(in *TeamDraftContentInput) {
			in.CapabilityGaps = []TeamDraftCapabilityGap{{Reason: "missing"}}
		}, want: ErrInvalidTeamDraftCapabilityGap},
		{name: "empty gap reason", change: func(in *TeamDraftContentInput) {
			in.CapabilityGaps = []TeamDraftCapabilityGap{{Capability: "gpu"}}
		}, want: ErrInvalidTeamDraftCapabilityGap},
		{name: "duplicate gap", change: func(in *TeamDraftContentInput) {
			in.CapabilityGaps = []TeamDraftCapabilityGap{
				{Capability: "gpu", Reason: "missing"},
				{Capability: "gpu", Reason: "missing"},
			}
		}, want: ErrDuplicateTeamDraftCapabilityGap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := cloneTeamDraftContentInput(input)
			tt.change(&changed)
			got, err := BuildTeamDraftContent(catalog, changed)
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildTeamDraftContent() error = %v, want errors.Is(%v)", err, tt.want)
			}
			assertZeroTeamDraftContent(t, got)
		})
	}
}

func TestValidateTeamDraftContent(t *testing.T) {
	catalog, input := draftContentFixture(t)
	snapshot := mustTeamDraftContent(t, catalog, input)
	candidate, err := ValidateTeamDraftContent(snapshot, catalog)
	if err != nil {
		t.Fatalf("ValidateTeamDraftContent() error = %v", err)
	}
	want := TeamDraftContentValidationCandidate{
		Valid:                 true,
		AcceptanceReady:       true,
		ContentDigest:         snapshot.Digest(),
		CatalogDigest:         catalog.Digest(),
		MainAgentDefinitionID: "agent.main",
		RoleCount:             3,
		TaskCount:             3,
	}
	if !reflect.DeepEqual(candidate, want) {
		t.Fatalf("ValidateTeamDraftContent() = %#v, want %#v", candidate, want)
	}

	tests := []struct {
		name     string
		snapshot TeamDraftContentSnapshot
		catalog  TeamDraftCatalogSnapshot
		want     error
	}{
		{name: "zero snapshot", catalog: catalog, want: ErrInvalidTeamDraftContent},
		{name: "tampered digest", snapshot: tamperTeamDraftContent(snapshot, func(s *TeamDraftContentSnapshot) {
			s.digest = "tampered"
		}), catalog: catalog, want: ErrTeamDraftContentDigestMismatch},
		{name: "tampered task", snapshot: tamperTeamDraftContent(snapshot, func(s *TeamDraftContentSnapshot) {
			s.tasks[0].AcceptanceCriteria[0] = "tampered"
		}), catalog: catalog, want: ErrTeamDraftContentDigestMismatch},
		{name: "catalog mismatch", snapshot: snapshot, catalog: draftCatalogFixture(t, []string{"skill.extra"}), want: ErrTeamDraftCatalogMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateTeamDraftContent(tt.snapshot, tt.catalog)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateTeamDraftContent() error = %v, want errors.Is(%v)", err, tt.want)
			}
			if got != (TeamDraftContentValidationCandidate{}) {
				t.Fatalf("failed validation returned Candidate %#v", got)
			}
		})
	}
}

func TestTeamDraftContentImportBoundary(t *testing.T) {
	source, err := os.ReadFile("draft_content.go")
	if err != nil {
		t.Fatalf("ReadFile(draft_content.go): %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "draft_content.go", source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(draft_content.go): %v", err)
	}
	allowed := map[string]bool{
		`"crypto/sha256"`:                    true,
		`"encoding/hex"`:                     true,
		`"encoding/json"`:                    true,
		`"errors"`:                           true,
		`"fmt"`:                              true,
		`"sort"`:                             true,
		`"loom-pi-rebuild/internal/runtime"`: true,
	}
	for _, imported := range file.Imports {
		if !allowed[imported.Path.Value] {
			t.Fatalf("draft_content.go imports forbidden package %s", imported.Path.Value)
		}
	}
}

func draftContentFixture(t *testing.T) (TeamDraftCatalogSnapshot, TeamDraftContentInput) {
	t.Helper()
	catalog := draftCatalogFixture(t, nil)
	mainBudget, subABudget, subBBudget := int64(20), int64(15), int64(15)
	return catalog, TeamDraftContentInput{
		References: draftReferencesFixture(),
		Roles: []TeamDraftRoleSelection{
			{
				AgentDefinitionID: "agent.main",
				RuntimeProfile: runtime.RuntimeProfile{
					ID: "profile.main", AdapterType: "adapter.local", ProviderID: "provider.local",
					ModelID: "model.alpha", AuthMode: runtime.AuthBrokered,
					RequiredCapabilities: []string{"go_test"}, Timeout: 30 * time.Second, Budget: &mainBudget,
				},
				RuntimeInstanceID: "runtime.local",
				SkillIDs:          []string{"skill.a"},
				MemberIDs:         []string{"member.a"},
				PermissionIDs:     []string{"permission.read"},
			},
			{
				AgentDefinitionID: "agent.sub.a",
				RuntimeProfile: runtime.RuntimeProfile{
					ID: "profile.sub.a", AdapterType: "adapter.local", ProviderID: "provider.local",
					ModelID: "model.beta", AuthMode: runtime.AuthBrokered,
					RequiredCapabilities: []string{"apply_patch", "go_test"}, Timeout: 30 * time.Second, Budget: &subABudget,
				},
				RuntimeInstanceID: "runtime.local",
				SkillIDs:          []string{"skill.z"},
				MemberIDs:         []string{"member.z"},
				PermissionIDs:     []string{"permission.write"},
			},
			{
				AgentDefinitionID: "agent.sub.b",
				RuntimeProfile: runtime.RuntimeProfile{
					ID: "profile.sub.b", AdapterType: "adapter.local", ProviderID: "provider.local",
					ModelID: "model.alpha", AuthMode: runtime.AuthNative,
					RequiredCapabilities: []string{"go_test"}, Timeout: 45 * time.Second, Budget: &subBBudget,
				},
				RuntimeInstanceID: "runtime.local",
				SkillIDs:          []string{"skill.a", "skill.z"},
				MemberIDs:         []string{"member.a"},
				PermissionIDs:     []string{"permission.read"},
			},
		},
		Tasks: []TeamDraftTaskCandidate{
			{
				ID: "task.alpha", OwnerAgentDefinitionID: "agent.sub.a",
				AcceptanceCriteria: []string{"alpha is reproducible"},
			},
			{
				ID: "task.beta", OwnerAgentDefinitionID: "agent.sub.b",
				DependencyTaskIDs:  []string{"task.alpha"},
				AcceptanceCriteria: []string{"beta passes"},
			},
			{
				ID: "task.gamma", OwnerAgentDefinitionID: "agent.sub.a",
				DependencyTaskIDs:  []string{"task.beta", "task.alpha"},
				AcceptanceCriteria: []string{"gamma evidence exists", "gamma passes"},
			},
		},
		CustomerRuleSummary: "No external mutations; independent review required.",
		ApprovalMarkers: []TeamDraftApprovalMarker{
			{ID: "approval.external", Reason: "external mutation"},
			{ID: "approval.cost", Reason: "budget increase"},
		},
		Limits: TeamDraftContentLimits{
			MaxTasks:                     3,
			MaxDependenciesPerTask:       2,
			MaxAcceptanceCriteriaPerTask: 2,
			MaxApprovalMarkers:           2,
			MaxCapabilityGaps:            2,
		},
	}
}

func fourRoleDraftContentFixture(t *testing.T) (TeamDraftCatalogSnapshot, TeamDraftContentInput) {
	_, input := draftContentFixture(t)
	catalog := draftContentCatalog(t, []string{"agent.main", "agent.sub.a", "agent.sub.b", "agent.sub.c"})
	input.References.SubAgentDefinitionIDs = append(input.References.SubAgentDefinitionIDs, "agent.sub.c")
	input.Roles = append(input.Roles, draftContentRole("agent.sub.c", "profile.sub.c", "model.alpha"))
	input.Tasks = append(input.Tasks, TeamDraftTaskCandidate{
		ID: "task.sub.c", OwnerAgentDefinitionID: "agent.sub.c",
		AcceptanceCriteria: []string{"sub c passes"},
	})
	input.Limits.MaxTasks = len(input.Tasks)
	return catalog, input
}

func draftContentCatalog(t *testing.T, agentIDs []string) TeamDraftCatalogSnapshot {
	t.Helper()
	definitions := make([]agents.AgentDefinition, 0, len(agentIDs))
	for _, id := range agentIDs {
		definitions = append(definitions, mustCatalogAgent(t, agents.AgentDefinition{
			ID: id, Version: 1, Scope: agents.ScopeReusable,
			Name: id, RoleSpec: "Bounded role " + id, Status: agents.DefinitionActive,
		}))
	}
	discovery := mustCatalogRuntimeDiscovery(t, []runtime.RuntimeProbe{
		catalogRuntimeProbe("probe.local", runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.local", func(instance *runtime.RuntimeInstance) {
				instance.Capacity = 3
			}),
			ModelIDs: []string{"model.alpha", "model.beta"},
		}),
	})
	return mustTeamDraftCatalog(t, TeamDraftCatalogInput{
		AgentDefinitions: definitions, RuntimeDiscovery: discovery,
		SkillIDs: []string{"skill.a", "skill.z"}, MemberIDs: []string{"member.a", "member.z"},
		PermissionIDs: []string{"permission.read", "permission.write"},
		BudgetCeiling: 100, ConcurrencyCeiling: 3,
		MaxCounts: TeamDraftCatalogMaxCounts{
			Agents: len(agentIDs), Runtimes: 1, Models: 2, Skills: 2, Members: 2, Permissions: 2,
		},
	})
}

func draftContentRole(agentID, profileID, modelID string) TeamDraftRoleSelection {
	budget := int64(10)
	return TeamDraftRoleSelection{
		AgentDefinitionID: agentID,
		RuntimeProfile: runtime.RuntimeProfile{
			ID: profileID, AdapterType: "adapter.local", ProviderID: "provider.local",
			ModelID: modelID, AuthMode: runtime.AuthBrokered,
			RequiredCapabilities: []string{"go_test"}, Timeout: 30 * time.Second, Budget: &budget,
		},
		RuntimeInstanceID: "runtime.local",
	}
}

func mustTeamDraftContent(t *testing.T, catalog TeamDraftCatalogSnapshot, input TeamDraftContentInput) TeamDraftContentSnapshot {
	t.Helper()
	snapshot, err := BuildTeamDraftContent(catalog, input)
	if err != nil {
		t.Fatalf("BuildTeamDraftContent() error = %v", err)
	}
	return snapshot
}

func cloneTeamDraftContentInput(input TeamDraftContentInput) TeamDraftContentInput {
	input.References = cloneDraftReferences(input.References)
	roles := input.Roles
	input.Roles = make([]TeamDraftRoleSelection, len(roles))
	for i := range roles {
		input.Roles[i] = cloneTeamDraftRoleSelection(roles[i])
	}
	tasks := input.Tasks
	input.Tasks = make([]TeamDraftTaskCandidate, len(tasks))
	for i := range tasks {
		input.Tasks[i] = cloneTeamDraftTaskCandidate(tasks[i])
	}
	input.ApprovalMarkers = append([]TeamDraftApprovalMarker(nil), input.ApprovalMarkers...)
	input.CapabilityGaps = append([]TeamDraftCapabilityGap(nil), input.CapabilityGaps...)
	return input
}

func cloneTeamDraftRoleSelection(input TeamDraftRoleSelection) TeamDraftRoleSelection {
	input.RuntimeProfile.RequiredCapabilities = append([]string(nil), input.RuntimeProfile.RequiredCapabilities...)
	if input.RuntimeProfile.Budget != nil {
		value := *input.RuntimeProfile.Budget
		input.RuntimeProfile.Budget = &value
	}
	input.SkillIDs = append([]string(nil), input.SkillIDs...)
	input.MemberIDs = append([]string(nil), input.MemberIDs...)
	input.PermissionIDs = append([]string(nil), input.PermissionIDs...)
	return input
}

func cloneTeamDraftTaskCandidate(input TeamDraftTaskCandidate) TeamDraftTaskCandidate {
	input.DependencyTaskIDs = append([]string(nil), input.DependencyTaskIDs...)
	input.AcceptanceCriteria = append([]string(nil), input.AcceptanceCriteria...)
	return input
}

func mutateDraftContentInput(input *TeamDraftContentInput) {
	input.References.SubAgentDefinitionIDs[0] = "mutated"
	input.Roles[0].AgentDefinitionID = "mutated"
	input.Roles[0].RuntimeProfile.RequiredCapabilities[0] = "mutated"
	*input.Roles[0].RuntimeProfile.Budget = 999
	input.Roles[0].SkillIDs[0] = "mutated"
	input.Tasks[0].AcceptanceCriteria[0] = "mutated"
	input.ApprovalMarkers[0].Reason = "mutated"
}

func tamperTeamDraftContent(input TeamDraftContentSnapshot, change func(*TeamDraftContentSnapshot)) TeamDraftContentSnapshot {
	clone := cloneTeamDraftContentSnapshot(input)
	change(&clone)
	return clone
}

func assertNormalizedDraftContent(t *testing.T, snapshot TeamDraftContentSnapshot) {
	t.Helper()
	if got := roleIDs(snapshot.Roles()); !reflect.DeepEqual(got, []string{"agent.main", "agent.sub.a", "agent.sub.b"}) {
		t.Fatalf("role IDs = %#v", got)
	}
	if got := taskIDs(snapshot.Tasks()); !reflect.DeepEqual(got, []string{"task.alpha", "task.beta", "task.gamma"}) {
		t.Fatalf("task IDs = %#v", got)
	}
	tasks := snapshot.Tasks()
	if !reflect.DeepEqual(tasks[2].DependencyTaskIDs, []string{"task.alpha", "task.beta"}) {
		t.Fatalf("normalized dependencies = %#v", tasks[2].DependencyTaskIDs)
	}
	if !reflect.DeepEqual(tasks[2].AcceptanceCriteria, []string{"gamma evidence exists", "gamma passes"}) {
		t.Fatalf("normalized criteria = %#v", tasks[2].AcceptanceCriteria)
	}
	roles := snapshot.Roles()
	if !reflect.DeepEqual(roles[2].SkillIDs, []string{"skill.a", "skill.z"}) {
		t.Fatalf("normalized role skills = %#v", roles[2].SkillIDs)
	}
	if snapshot.CustomerRuleSummary() != "No external mutations; independent review required." {
		t.Fatalf("CustomerRuleSummary() = %q", snapshot.CustomerRuleSummary())
	}
	if len(snapshot.CapabilityGaps()) != 0 {
		t.Fatalf("CapabilityGaps() = %#v, want none", snapshot.CapabilityGaps())
	}
}

func assertZeroTeamDraftContent(t *testing.T, snapshot TeamDraftContentSnapshot) {
	t.Helper()
	if snapshot.Digest() != "" || snapshot.CatalogDigest() != "" ||
		!reflect.DeepEqual(snapshot.References(), TeamDraftReferences{}) ||
		len(snapshot.Roles()) != 0 || len(snapshot.Tasks()) != 0 ||
		snapshot.CustomerRuleSummary() != "" || len(snapshot.ApprovalMarkers()) != 0 ||
		len(snapshot.CapabilityGaps()) != 0 || snapshot.Limits() != (TeamDraftContentLimits{}) {
		t.Fatalf("failed operation returned usable snapshot %#v", snapshot)
	}
}

func assertSHA256Digest(t *testing.T, digest string) {
	t.Helper()
	if len(digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(digest))
	}
	for _, char := range digest {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			t.Fatalf("digest %q is not lowercase SHA-256 hex", digest)
		}
	}
}

func roleIDs(roles []TeamDraftRoleSelection) []string {
	ids := make([]string, len(roles))
	for i := range roles {
		ids[i] = roles[i].AgentDefinitionID
	}
	return ids
}

func taskIDs(tasks []TeamDraftTaskCandidate) []string {
	ids := make([]string, len(tasks))
	for i := range tasks {
		ids[i] = tasks[i].ID
	}
	return ids
}
