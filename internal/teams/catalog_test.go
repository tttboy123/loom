package teams

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/runtime"
)

func TestTeamDraftCatalog(t *testing.T) {
	projectReviewer := mustCatalogAgent(t, agents.AgentDefinition{
		ID:      "agent.review",
		Version: 2,
		Scope:   agents.ScopeProject,
		ScopeIdentity: agents.ScopeIdentity{
			ProjectID: "project.alpha",
		},
		Name:     "Project Reviewer",
		RoleSpec: "Review candidate changes against the frozen contract.",
		Status:   agents.DefinitionActive,
	})
	reusableReviewer := mustCatalogAgent(t, agents.AgentDefinition{
		ID:       "agent.review",
		Version:  9,
		Scope:    agents.ScopeReusable,
		Name:     "Reusable Reviewer",
		RoleSpec: "Reusable fallback reviewer.",
		Status:   agents.DefinitionActive,
	})
	transientBuilder := mustCatalogAgent(t, agents.AgentDefinition{
		ID:      "agent.build",
		Version: 1,
		Scope:   agents.ScopeTransient,
		ScopeIdentity: agents.ScopeIdentity{
			GenerationID: "generation.alpha",
		},
		Name:     "Transient Builder",
		RoleSpec: "Build the candidate after review.",
		Status:   agents.DefinitionActive,
	})
	archivedBuilderV9 := mustCatalogAgent(t, withCatalogAgent(transientBuilder, func(d *agents.AgentDefinition) {
		d.Version = 9
		d.Status = agents.DefinitionArchived
		d.Name = "Archived Builder"
	}))
	otherProjectAuditor := mustCatalogAgent(t, agents.AgentDefinition{
		ID:      "agent.audit",
		Version: 4,
		Scope:   agents.ScopeProject,
		ScopeIdentity: agents.ScopeIdentity{
			ProjectID: "project.beta",
		},
		Name:     "Other Project Auditor",
		RoleSpec: "Audit another project.",
		Status:   agents.DefinitionActive,
	})
	reusableAuditor := mustCatalogAgent(t, agents.AgentDefinition{
		ID:       "agent.audit",
		Version:  1,
		Scope:    agents.ScopeReusable,
		Name:     "Reusable Auditor",
		RoleSpec: "Audit any project.",
		Status:   agents.DefinitionActive,
	})

	discovery := mustCatalogRuntimeDiscovery(t, []runtime.RuntimeProbe{
		catalogRuntimeProbe("probe.beta", runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.beta", func(i *runtime.RuntimeInstance) {
				i.DeviceID = "device.beta"
				i.DisplayName = "Beta Runtime"
				i.ObservedCapabilities = []string{"shell", "go_test"}
				i.Capacity = 2
			}),
			ModelIDs: []string{"model.shared", "model.beta"},
		}),
		catalogRuntimeProbe("probe.offline", runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.offline", func(i *runtime.RuntimeInstance) {
				i.Status = runtime.RuntimeOffline
			}),
			ModelIDs: []string{"model.offline"},
		}),
		catalogRuntimeProbe("probe.alpha", runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.alpha", func(i *runtime.RuntimeInstance) {
				i.DeviceID = "device.alpha"
				i.DisplayName = "Alpha Runtime"
				i.ObservedCapabilities = []string{"apply_patch", "go_test"}
				i.Capacity = 3
			}),
			ModelIDs: []string{"model.shared", "model.alpha"},
		}),
	})

	baseline := TeamDraftCatalogInput{
		AgentDefinitions: []agents.AgentDefinition{
			archivedBuilderV9,
			reusableReviewer,
			otherProjectAuditor,
			transientBuilder,
			projectReviewer,
			reusableAuditor,
		},
		ResolutionContext: agents.ResolutionContext{
			ProjectID:    "project.alpha",
			GenerationID: "generation.alpha",
		},
		RuntimeDiscovery:   discovery,
		SkillIDs:           []string{"skill.test", "skill.patch"},
		MemberIDs:          []string{"member.reviewer", "member.owner"},
		PermissionIDs:      []string{"permission.read", "permission.write"},
		BudgetCeiling:      100,
		ConcurrencyCeiling: 3,
		MaxCounts: TeamDraftCatalogMaxCounts{
			Agents:      3,
			Runtimes:    2,
			Models:      4,
			Skills:      2,
			Members:     2,
			Permissions: 2,
		},
	}

	t.Run("builds bounded immutable normalized snapshot", func(t *testing.T) {
		snapshot := mustTeamDraftCatalog(t, baseline)
		assertLowercaseSHA256Digest(t, snapshot.Digest())
		if snapshot.RuntimeDiscoveryDigest() != discovery.Digest() {
			t.Fatalf("RuntimeDiscoveryDigest() = %q, want %q", snapshot.RuntimeDiscoveryDigest(), discovery.Digest())
		}
		if got := snapshot.BudgetCeiling(); got != 100 {
			t.Fatalf("BudgetCeiling() = %d, want 100", got)
		}
		if got := snapshot.ConcurrencyCeiling(); got != 3 {
			t.Fatalf("ConcurrencyCeiling() = %d, want 3", got)
		}
		if got := snapshot.MaxCounts(); got != baseline.MaxCounts {
			t.Fatalf("MaxCounts() = %#v, want %#v", got, baseline.MaxCounts)
		}

		agentEntries := snapshot.Agents()
		assertAgentCatalogOrder(t, agentEntries, []string{"agent.audit", "agent.build", "agent.review"})
		assertAgentCatalogEntry(t, agentEntries[0], reusableAuditor)
		assertAgentCatalogEntry(t, agentEntries[1], transientBuilder)
		assertAgentCatalogEntry(t, agentEntries[2], projectReviewer)
		if agentEntries[2].Name == reusableReviewer.Name {
			t.Fatalf("project scoped definition did not take precedence over reusable version")
		}
		for _, entry := range agentEntries {
			if entry.Status != agents.DefinitionActive {
				t.Fatalf("catalog included non-active agent entry: %#v", entry)
			}
			if entry.ID == otherProjectAuditor.ID && entry.Name == otherProjectAuditor.Name {
				t.Fatalf("catalog included context-ineligible project definition: %#v", entry)
			}
			if entry.ID == archivedBuilderV9.ID && entry.Name == archivedBuilderV9.Name {
				t.Fatalf("catalog included archived definition: %#v", entry)
			}
		}

		runtimes := snapshot.Runtimes()
		assertRuntimeCatalogOrder(t, runtimes, []string{"runtime.alpha", "runtime.beta"})
		assertRuntimeCatalogEntry(t, runtimes[0], "probe.alpha", []string{"apply_patch", "go_test"}, []string{"model.alpha", "model.shared"})
		assertRuntimeCatalogEntry(t, runtimes[1], "probe.beta", []string{"go_test", "shell"}, []string{"model.beta", "model.shared"})
		if containsRuntimeCatalogID(runtimes, "runtime.offline") {
			t.Fatalf("catalog included offline runtime: %#v", runtimes)
		}

		assertStringSet(t, "Skills", snapshot.Skills(), []string{"skill.patch", "skill.test"})
		assertStringSet(t, "Members", snapshot.Members(), []string{"member.owner", "member.reviewer"})
		assertStringSet(t, "Permissions", snapshot.Permissions(), []string{"permission.read", "permission.write"})
	})

	t.Run("accepts successful empty discovery but rejects zero value discovery", func(t *testing.T) {
		empty := withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
			input.RuntimeDiscovery = mustCatalogRuntimeDiscovery(t, nil)
			input.MaxCounts.Runtimes = 1
			input.MaxCounts.Models = 1
		})
		snapshot := mustTeamDraftCatalog(t, empty)
		if got := snapshot.Runtimes(); len(got) != 0 {
			t.Fatalf("empty successful discovery produced runtimes %#v, want none", got)
		}

		_, err := BuildTeamDraftCatalog(withCatalogInput(empty, func(input *TeamDraftCatalogInput) {
			input.RuntimeDiscovery = runtime.RuntimeDiscoverySnapshot{}
		}))
		if !errors.Is(err, ErrInvalidTeamDraftCatalog) {
			t.Fatalf("BuildTeamDraftCatalog(zero discovery) error = %v, want ErrInvalidTeamDraftCatalog", err)
		}
	})

	t.Run("rejects invalid ceilings and workspace ids before normalization", func(t *testing.T) {
		tests := []struct {
			name   string
			input  TeamDraftCatalogInput
			wantIs error
		}{
			{name: "negative budget", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.BudgetCeiling = -1 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero concurrency", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.ConcurrencyCeiling = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero max agents", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Agents = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero max runtimes", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Runtimes = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero max models", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Models = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero max skills", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Skills = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero max members", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Members = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "zero max permissions", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Permissions = 0 }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "empty skill id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.SkillIDs = []string{"skill.test", ""} }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "duplicate skill id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.SkillIDs = []string{"skill.test", "skill.test"} }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "empty member id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MemberIDs = []string{"member.owner", ""} }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "duplicate member id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MemberIDs = []string{"member.owner", "member.owner"} }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "empty permission id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.PermissionIDs = []string{"permission.read", ""} }), wantIs: ErrInvalidTeamDraftCatalog},
			{name: "duplicate permission id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.PermissionIDs = []string{"permission.read", "permission.read"}
			}), wantIs: ErrInvalidTeamDraftCatalog},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				snapshot, err := BuildTeamDraftCatalog(tt.input)
				if !errors.Is(err, tt.wantIs) {
					t.Fatalf("BuildTeamDraftCatalog() error = %v, want errors.Is(%v)", err, tt.wantIs)
				}
				assertZeroTeamDraftCatalog(t, snapshot)
			})
		}
	})

	t.Run("enforces exact max and max plus one without truncation", func(t *testing.T) {
		mustTeamDraftCatalog(t, baseline)
		tests := []struct {
			name  string
			input TeamDraftCatalogInput
		}{
			{name: "selected agents max plus one", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Agents = 2 })},
			{name: "online runtimes max plus one", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Runtimes = 1 })},
			{name: "runtime scoped models max plus one", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Models = 3 })},
			{name: "skills max plus one", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Skills = 1 })},
			{name: "members max plus one", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Members = 1 })},
			{name: "permissions max plus one", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Permissions = 1 })},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				snapshot, err := BuildTeamDraftCatalog(tt.input)
				if !errors.Is(err, ErrTeamDraftCatalogLimitExceeded) {
					t.Fatalf("BuildTeamDraftCatalog() error = %v, want ErrTeamDraftCatalogLimitExceeded", err)
				}
				assertZeroTeamDraftCatalog(t, snapshot)
			})
		}
	})

	t.Run("revalidates every agent and fails duplicate accepted identity", func(t *testing.T) {
		_, invalidErr := BuildTeamDraftCatalog(withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
			input.AgentDefinitions = append(slices.Clone(input.AgentDefinitions), agents.AgentDefinition{
				ID:      "agent.invalid",
				Version: 1,
				Scope:   agents.ScopeProject,
				ScopeIdentity: agents.ScopeIdentity{
					ProjectID: "project.alpha",
				},
				Name:   "Invalid",
				Status: agents.DefinitionActive,
			})
		}))
		if !errors.Is(invalidErr, agents.ErrInvalidAgentDefinition) {
			t.Fatalf("BuildTeamDraftCatalog(invalid agent) error = %v, want ErrInvalidAgentDefinition", invalidErr)
		}

		_, duplicateErr := BuildTeamDraftCatalog(withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
			input.AgentDefinitions = append(slices.Clone(input.AgentDefinitions), projectReviewer)
		}))
		if !errors.Is(duplicateErr, agents.ErrDuplicateAgentDefinition) {
			t.Fatalf("BuildTeamDraftCatalog(duplicate agent identity) error = %v, want ErrDuplicateAgentDefinition", duplicateErr)
		}
	})

	t.Run("digest is reorder stable and sensitive to every included field class", func(t *testing.T) {
		first := mustTeamDraftCatalog(t, baseline)
		reordered := mustTeamDraftCatalog(t, withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
			slices.Reverse(input.AgentDefinitions)
			slices.Reverse(input.SkillIDs)
			slices.Reverse(input.MemberIDs)
			slices.Reverse(input.PermissionIDs)
			input.RuntimeDiscovery = mustCatalogRuntimeDiscovery(t, []runtime.RuntimeProbe{
				catalogRuntimeProbe("probe.alpha", runtime.RuntimeObservation{
					Instance: catalogRuntimeInstance("runtime.alpha", func(i *runtime.RuntimeInstance) {
						i.DeviceID = "device.alpha"
						i.DisplayName = "Alpha Runtime"
						i.ObservedCapabilities = []string{"go_test", "apply_patch"}
						i.Capacity = 3
					}),
					ModelIDs: []string{"model.shared", "model.alpha"},
				}),
				catalogRuntimeProbe("probe.offline", runtime.RuntimeObservation{
					Instance: catalogRuntimeInstance("runtime.offline", func(i *runtime.RuntimeInstance) {
						i.Status = runtime.RuntimeOffline
					}),
					ModelIDs: []string{"model.offline"},
				}),
				catalogRuntimeProbe("probe.beta", runtime.RuntimeObservation{
					Instance: catalogRuntimeInstance("runtime.beta", func(i *runtime.RuntimeInstance) {
						i.DeviceID = "device.beta"
						i.DisplayName = "Beta Runtime"
						i.ObservedCapabilities = []string{"go_test", "shell"}
						i.Capacity = 2
					}),
					ModelIDs: []string{"model.beta", "model.shared"},
				}),
			})
		}))
		if first.Digest() != reordered.Digest() {
			t.Fatalf("digest differs under canonical reordering: first %q reordered %q", first.Digest(), reordered.Digest())
		}

		tests := []struct {
			name   string
			input  TeamDraftCatalogInput
			verify func(*testing.T, TeamDraftCatalogSnapshot)
		}{
			{name: "agent stable id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.AgentDefinitions[4].ID = "agent.review.changed"
				input.MaxCounts.Agents = 4
			})},
			{name: "agent version", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.AgentDefinitions[4].Version = 3 })},
			{name: "agent scope", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.AgentDefinitions[4].Scope = agents.ScopeTransient
				input.AgentDefinitions[4].ScopeIdentity = agents.ScopeIdentity{GenerationID: "generation.alpha"}
			})},
			{name: "agent scope identity", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.ResolutionContext.ProjectID = "project.gamma" })},
			{name: "agent name", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.AgentDefinitions[4].Name = "Changed Reviewer" })},
			{name: "agent role spec", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.AgentDefinitions[4].RoleSpec = "Changed role." })},
			{name: "runtime source probe id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaProbeID = "probe.alpha.changed"
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				assertRuntimeCatalogEntry(t, snapshot.Runtimes()[0], "probe.alpha.changed", []string{"apply_patch", "go_test"}, []string{"model.alpha", "model.shared"})
			}},
			{name: "runtime id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.ID = "runtime.alpha.changed"
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				assertRuntimeCatalogOrder(t, snapshot.Runtimes(), []string{"runtime.alpha.changed", "runtime.beta"})
			}},
			{name: "runtime device id", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.DeviceID = "device.alpha.changed"
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				if got := snapshot.Runtimes()[0].DeviceID; got != "device.alpha.changed" {
					t.Fatalf("DeviceID = %q, want changed runtime device id", got)
				}
			}},
			{name: "runtime adapter type", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.AdapterType = "adapter.changed"
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				if got := snapshot.Runtimes()[0].AdapterType; got != "adapter.changed" {
					t.Fatalf("AdapterType = %q, want changed runtime adapter type", got)
				}
			}},
			{name: "runtime display name", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.DisplayName = "Alpha Runtime Changed"
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				if got := snapshot.Runtimes()[0].DisplayName; got != "Alpha Runtime Changed" {
					t.Fatalf("DisplayName = %q, want changed runtime display name", got)
				}
			}},
			{name: "runtime executable version", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.ExecutableVersion = "1.0.1"
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				if got := snapshot.Runtimes()[0].ExecutableVersion; got != "1.0.1" {
					t.Fatalf("ExecutableVersion = %q, want changed runtime executable version", got)
				}
			}},
			{name: "runtime status filtering", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.Status = runtime.RuntimeDisabled
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				assertRuntimeCatalogOrder(t, snapshot.Runtimes(), []string{"runtime.beta"})
			}},
			{name: "runtime capabilities", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.ObservedCapabilities = []string{"apply_patch", "shell"}
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				assertRuntimeCatalogEntry(t, snapshot.Runtimes()[0], "probe.alpha", []string{"apply_patch", "shell"}, []string{"model.alpha", "model.shared"})
			}},
			{name: "runtime capacity", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.Instance.Capacity = 7
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				if got := snapshot.Runtimes()[0].Capacity; got != 7 {
					t.Fatalf("Capacity = %d, want changed runtime capacity", got)
				}
			}},
			{name: "runtime model ids", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {
				input.RuntimeDiscovery = catalogComparableDiscovery(t, func(discovery *catalogComparableDiscoveryInput) {
					discovery.alphaObservation.ModelIDs = []string{"model.alpha", "model.changed"}
				})
			}), verify: func(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
				assertRuntimeCatalogEntry(t, snapshot.Runtimes()[0], "probe.alpha", []string{"apply_patch", "go_test"}, []string{"model.alpha", "model.changed"})
			}},
			{name: "skill set", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.SkillIDs[0] = "skill.changed" })},
			{name: "member set", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MemberIDs[0] = "member.changed" })},
			{name: "permission set", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.PermissionIDs[0] = "permission.changed" })},
			{name: "max counts", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.MaxCounts.Models = 5 })},
			{name: "budget ceiling", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.BudgetCeiling = 101 })},
			{name: "concurrency ceiling", input: withCatalogInput(baseline, func(input *TeamDraftCatalogInput) { input.ConcurrencyCeiling = 4 })},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				changed := mustTeamDraftCatalog(t, tt.input)
				if changed.Digest() == first.Digest() {
					t.Fatalf("digest did not change when included field class %q changed", tt.name)
				}
				if tt.verify != nil {
					tt.verify(t, changed)
				}
			})
		}
	})

	t.Run("accessors return deep copies for all nested collections", func(t *testing.T) {
		snapshot := mustTeamDraftCatalog(t, baseline)
		originalDigest := snapshot.Digest()

		agents := snapshot.Agents()
		agents[0].ID = "agent.mutated"
		agents[0].ScopeIdentity.ProjectID = "project.mutated"

		runtimes := snapshot.Runtimes()
		runtimes[0].ID = "runtime.mutated"
		runtimes[0].ObservedCapabilities[0] = "capability.mutated"
		runtimes[0].ModelIDs[0] = "model.mutated"

		skills := snapshot.Skills()
		members := snapshot.Members()
		permissions := snapshot.Permissions()
		skills[0] = "skill.mutated"
		members[0] = "member.mutated"
		permissions[0] = "permission.mutated"

		if snapshot.Digest() != originalDigest {
			t.Fatalf("digest changed after mutating returned collections: got %q want %q", snapshot.Digest(), originalDigest)
		}
		assertAgentCatalogOrder(t, snapshot.Agents(), []string{"agent.audit", "agent.build", "agent.review"})
		assertRuntimeCatalogEntry(t, snapshot.Runtimes()[0], "probe.alpha", []string{"apply_patch", "go_test"}, []string{"model.alpha", "model.shared"})
		assertStringSet(t, "Skills", snapshot.Skills(), []string{"skill.patch", "skill.test"})
		assertStringSet(t, "Members", snapshot.Members(), []string{"member.owner", "member.reviewer"})
		assertStringSet(t, "Permissions", snapshot.Permissions(), []string{"permission.read", "permission.write"})
	})

	t.Run("caller input aliases cannot mutate snapshot", func(t *testing.T) {
		callerInput := withCatalogInput(baseline, func(input *TeamDraftCatalogInput) {})
		snapshot := mustTeamDraftCatalog(t, callerInput)
		originalDigest := snapshot.Digest()

		callerInput.AgentDefinitions[0].ID = "agent.alias.mutated"
		callerInput.AgentDefinitions[0].ScopeIdentity.GenerationID = "generation.mutated"
		callerInput.SkillIDs[0] = "skill.alias.mutated"
		callerInput.MemberIDs[0] = "member.alias.mutated"
		callerInput.PermissionIDs[0] = "permission.alias.mutated"

		if snapshot.Digest() != originalDigest {
			t.Fatalf("digest changed after mutating caller-owned input slices: got %q want %q", snapshot.Digest(), originalDigest)
		}
		assertAgentCatalogOrder(t, snapshot.Agents(), []string{"agent.audit", "agent.build", "agent.review"})
		assertStringSet(t, "Skills", snapshot.Skills(), []string{"skill.patch", "skill.test"})
		assertStringSet(t, "Members", snapshot.Members(), []string{"member.owner", "member.reviewer"})
		assertStringSet(t, "Permissions", snapshot.Permissions(), []string{"permission.read", "permission.write"})
	})

	t.Run("production import boundary stays pure domain", func(t *testing.T) {
		assertTeamDraftCatalogProductionImports(t, "catalog.go")
	})
}

func TestValidateTeamDraftCatalog(t *testing.T) {
	snapshot := mustTeamDraftCatalog(t, TeamDraftCatalogInput{
		AgentDefinitions: []agents.AgentDefinition{
			mustCatalogAgent(t, agents.AgentDefinition{
				ID:       "agent.main",
				Version:  1,
				Scope:    agents.ScopeReusable,
				Name:     "Main Agent",
				RoleSpec: "Coordinate the draft.",
				Status:   agents.DefinitionActive,
			}),
			mustCatalogAgent(t, agents.AgentDefinition{
				ID:       "agent.sub",
				Version:  1,
				Scope:    agents.ScopeReusable,
				Name:     "Sub Agent",
				RoleSpec: "Assist the main agent.",
				Status:   agents.DefinitionActive,
			}),
		},
		RuntimeDiscovery: mustCatalogRuntimeDiscovery(t, []runtime.RuntimeProbe{
			catalogRuntimeProbe("probe.one", runtime.RuntimeObservation{
				Instance: catalogRuntimeInstance("runtime.one", nil),
				ModelIDs: []string{"model.a", "model.shared"},
			}),
			catalogRuntimeProbe("probe.two", runtime.RuntimeObservation{
				Instance: catalogRuntimeInstance("runtime.two", func(i *runtime.RuntimeInstance) {
					i.DeviceID = "device.two"
					i.DisplayName = "Second Runtime"
				}),
				ModelIDs: []string{"model.b", "model.shared"},
			}),
		}),
		SkillIDs:           []string{"skill.patch", "skill.test"},
		MemberIDs:          []string{"member.owner", "member.reviewer"},
		PermissionIDs:      []string{"permission.read", "permission.write"},
		BudgetCeiling:      25,
		ConcurrencyCeiling: 2,
		MaxCounts: TeamDraftCatalogMaxCounts{
			Agents:      2,
			Runtimes:    2,
			Models:      4,
			Skills:      2,
			Members:     2,
			Permissions: 2,
		},
	})

	valid := TeamDraftReferences{
		MainAgentDefinitionID: "agent.main",
		SubAgentDefinitionIDs: []string{"agent.sub"},
		RuntimeInstanceIDs:    []string{"runtime.one", "runtime.two"},
		RuntimeModels: []RuntimeModelReference{
			{RuntimeInstanceID: "runtime.one", ModelID: "model.a"},
			{RuntimeInstanceID: "runtime.two", ModelID: "model.shared"},
		},
		SkillIDs:             []string{"skill.patch"},
		MemberIDs:            []string{"member.owner"},
		PermissionIDs:        []string{"permission.read"},
		RequestedBudget:      25,
		RequestedConcurrency: 2,
	}

	t.Run("returns successful candidate without accepting a draft", func(t *testing.T) {
		candidate, err := ValidateTeamDraftCatalog(snapshot, valid)
		if err != nil {
			t.Fatalf("ValidateTeamDraftCatalog(valid) error = %v", err)
		}
		if !candidate.Accepted {
			t.Fatalf("validation candidate Accepted = false, want true")
		}
		if candidate.CatalogDigest != snapshot.Digest() {
			t.Fatalf("validation candidate CatalogDigest = %q, want %q", candidate.CatalogDigest, snapshot.Digest())
		}
		if candidate.MainAgentDefinitionID != valid.MainAgentDefinitionID {
			t.Fatalf("validation candidate MainAgentDefinitionID = %q, want %q", candidate.MainAgentDefinitionID, valid.MainAgentDefinitionID)
		}
	})

	tests := []struct {
		name   string
		refs   TeamDraftReferences
		wantIs error
	}{
		{name: "empty main agent", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.MainAgentDefinitionID = "" }), wantIs: ErrInvalidTeamDraftReference},
		{name: "invented main agent", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.MainAgentDefinitionID = "agent.invented"
		}), wantIs: ErrInventedTeamDraftReference},
		{name: "empty sub agent", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SubAgentDefinitionIDs = []string{"agent.sub", ""} }), wantIs: ErrInvalidTeamDraftReference},
		{name: "duplicate sub agent", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SubAgentDefinitionIDs = []string{"agent.sub", "agent.sub"} }), wantIs: ErrDuplicateTeamDraftReference},
		{name: "main repeated as sub", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SubAgentDefinitionIDs = []string{"agent.main"} }), wantIs: ErrDuplicateTeamDraftReference},
		{name: "invented sub agent", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SubAgentDefinitionIDs = []string{"agent.invented"} }), wantIs: ErrInventedTeamDraftReference},
		{name: "empty runtime id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RuntimeInstanceIDs = []string{"runtime.one", ""} }), wantIs: ErrInvalidTeamDraftReference},
		{name: "duplicate runtime id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RuntimeInstanceIDs = []string{"runtime.one", "runtime.one"} }), wantIs: ErrDuplicateTeamDraftReference},
		{name: "invented runtime id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RuntimeInstanceIDs = []string{"runtime.invented"} }), wantIs: ErrInventedTeamDraftReference},
		{name: "empty model runtime id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "", ModelID: "model.a"}}
		}), wantIs: ErrInvalidTeamDraftReference},
		{name: "empty model id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "runtime.one", ModelID: ""}}
		}), wantIs: ErrInvalidTeamDraftReference},
		{name: "duplicate runtime model pair", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "runtime.one", ModelID: "model.a"}, {RuntimeInstanceID: "runtime.one", ModelID: "model.a"}}
		}), wantIs: ErrDuplicateTeamDraftReference},
		{name: "model runtime not referenced", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.RuntimeInstanceIDs = []string{"runtime.one"}
			refs.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "runtime.two", ModelID: "model.b"}}
		}), wantIs: ErrInventedTeamDraftReference},
		{name: "invented model id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "runtime.one", ModelID: "model.invented"}}
		}), wantIs: ErrInventedTeamDraftReference},
		{name: "cross paired model", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) {
			refs.RuntimeModels = []RuntimeModelReference{{RuntimeInstanceID: "runtime.one", ModelID: "model.b"}}
		}), wantIs: ErrRuntimeModelPairMismatch},
		{name: "empty skill id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SkillIDs = []string{"skill.patch", ""} }), wantIs: ErrInvalidTeamDraftReference},
		{name: "duplicate skill id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SkillIDs = []string{"skill.patch", "skill.patch"} }), wantIs: ErrDuplicateTeamDraftReference},
		{name: "invented skill id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.SkillIDs = []string{"skill.invented"} }), wantIs: ErrInventedTeamDraftReference},
		{name: "empty member id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.MemberIDs = []string{"member.owner", ""} }), wantIs: ErrInvalidTeamDraftReference},
		{name: "duplicate member id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.MemberIDs = []string{"member.owner", "member.owner"} }), wantIs: ErrDuplicateTeamDraftReference},
		{name: "invented member id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.MemberIDs = []string{"member.invented"} }), wantIs: ErrInventedTeamDraftReference},
		{name: "empty permission id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.PermissionIDs = []string{"permission.read", ""} }), wantIs: ErrInvalidTeamDraftReference},
		{name: "duplicate permission id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.PermissionIDs = []string{"permission.read", "permission.read"} }), wantIs: ErrDuplicateTeamDraftReference},
		{name: "invented permission id", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.PermissionIDs = []string{"permission.invented"} }), wantIs: ErrInventedTeamDraftReference},
		{name: "negative requested budget", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RequestedBudget = -1 }), wantIs: ErrInvalidTeamDraftReference},
		{name: "budget overflow", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RequestedBudget = 26 }), wantIs: ErrBudgetCeilingExceeded},
		{name: "zero requested concurrency", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RequestedConcurrency = 0 }), wantIs: ErrInvalidTeamDraftReference},
		{name: "concurrency overflow", refs: withTeamDraftReferences(valid, func(refs *TeamDraftReferences) { refs.RequestedConcurrency = 3 }), wantIs: ErrConcurrencyCeilingExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, err := ValidateTeamDraftCatalog(snapshot, tt.refs)
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("ValidateTeamDraftCatalog() error = %v, want errors.Is(%v)", err, tt.wantIs)
			}
			if candidate.Accepted || candidate.CatalogDigest != "" {
				t.Fatalf("failed validation returned usable candidate: %#v", candidate)
			}
		})
	}
}

type staticCatalogRuntimeProbe struct {
	id          string
	observation runtime.RuntimeObservation
}

type catalogComparableDiscoveryInput struct {
	alphaProbeID       string
	betaProbeID        string
	offlineProbeID     string
	alphaObservation   runtime.RuntimeObservation
	betaObservation    runtime.RuntimeObservation
	offlineObservation runtime.RuntimeObservation
}

func catalogComparableDiscovery(t *testing.T, configure func(*catalogComparableDiscoveryInput)) runtime.RuntimeDiscoverySnapshot {
	t.Helper()

	input := catalogComparableDiscoveryInput{
		alphaProbeID:   "probe.alpha",
		betaProbeID:    "probe.beta",
		offlineProbeID: "probe.offline",
		alphaObservation: runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.alpha", func(i *runtime.RuntimeInstance) {
				i.DeviceID = "device.alpha"
				i.DisplayName = "Alpha Runtime"
				i.ObservedCapabilities = []string{"apply_patch", "go_test"}
				i.Capacity = 3
			}),
			ModelIDs: []string{"model.alpha", "model.shared"},
		},
		betaObservation: runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.beta", func(i *runtime.RuntimeInstance) {
				i.DeviceID = "device.beta"
				i.DisplayName = "Beta Runtime"
				i.ObservedCapabilities = []string{"shell", "go_test"}
				i.Capacity = 2
			}),
			ModelIDs: []string{"model.beta", "model.shared"},
		},
		offlineObservation: runtime.RuntimeObservation{
			Instance: catalogRuntimeInstance("runtime.offline", func(i *runtime.RuntimeInstance) {
				i.Status = runtime.RuntimeOffline
			}),
			ModelIDs: []string{"model.offline"},
		},
	}
	if configure != nil {
		configure(&input)
	}

	return mustCatalogRuntimeDiscovery(t, []runtime.RuntimeProbe{
		catalogRuntimeProbe(input.betaProbeID, input.betaObservation),
		catalogRuntimeProbe(input.offlineProbeID, input.offlineObservation),
		catalogRuntimeProbe(input.alphaProbeID, input.alphaObservation),
	})
}

func (p staticCatalogRuntimeProbe) ID() string {
	return p.id
}

func (p staticCatalogRuntimeProbe) ObserveRuntime(context.Context) ([]runtime.RuntimeObservation, error) {
	return []runtime.RuntimeObservation{p.observation}, nil
}

func catalogRuntimeProbe(id string, observation runtime.RuntimeObservation) runtime.RuntimeProbe {
	return staticCatalogRuntimeProbe{id: id, observation: observation}
}

func catalogRuntimeInstance(id string, change func(*runtime.RuntimeInstance)) runtime.RuntimeInstance {
	instance := runtime.RuntimeInstance{
		ID:                   id,
		DeviceID:             "device.local",
		AdapterType:          "adapter.local",
		DisplayName:          "Local Runtime",
		ExecutableVersion:    "1.0.0",
		Status:               runtime.RuntimeOnline,
		ObservedCapabilities: []string{"go_test", "apply_patch"},
		Capacity:             1,
	}
	if change != nil {
		change(&instance)
	}
	return instance
}

func mustCatalogRuntimeDiscovery(t *testing.T, probes []runtime.RuntimeProbe) runtime.RuntimeDiscoverySnapshot {
	t.Helper()

	snapshot, err := runtime.DiscoverRuntime(context.Background(), probes)
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	return snapshot
}

func mustCatalogAgent(t *testing.T, input agents.AgentDefinition) agents.AgentDefinition {
	t.Helper()

	definition, err := agents.NewAgentDefinition(input)
	if err != nil {
		t.Fatalf("NewAgentDefinition(%#v) error = %v", input, err)
	}
	return definition
}

func mustTeamDraftCatalog(t *testing.T, input TeamDraftCatalogInput) TeamDraftCatalogSnapshot {
	t.Helper()

	snapshot, err := BuildTeamDraftCatalog(input)
	if err != nil {
		t.Fatalf("BuildTeamDraftCatalog() error = %v", err)
	}
	return snapshot
}

func withCatalogAgent(input agents.AgentDefinition, change func(*agents.AgentDefinition)) agents.AgentDefinition {
	changed := input
	change(&changed)
	return changed
}

func withCatalogInput(input TeamDraftCatalogInput, change func(*TeamDraftCatalogInput)) TeamDraftCatalogInput {
	changed := input
	changed.AgentDefinitions = slices.Clone(input.AgentDefinitions)
	changed.SkillIDs = slices.Clone(input.SkillIDs)
	changed.MemberIDs = slices.Clone(input.MemberIDs)
	changed.PermissionIDs = slices.Clone(input.PermissionIDs)
	change(&changed)
	return changed
}

func withTeamDraftReferences(input TeamDraftReferences, change func(*TeamDraftReferences)) TeamDraftReferences {
	changed := input
	changed.SubAgentDefinitionIDs = slices.Clone(input.SubAgentDefinitionIDs)
	changed.RuntimeInstanceIDs = slices.Clone(input.RuntimeInstanceIDs)
	changed.RuntimeModels = slices.Clone(input.RuntimeModels)
	changed.SkillIDs = slices.Clone(input.SkillIDs)
	changed.MemberIDs = slices.Clone(input.MemberIDs)
	changed.PermissionIDs = slices.Clone(input.PermissionIDs)
	change(&changed)
	return changed
}

func assertAgentCatalogOrder(t *testing.T, entries []AgentCatalogEntry, wantIDs []string) {
	t.Helper()

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.ID)
	}
	if !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("agent catalog order = %#v, want %#v", got, wantIDs)
	}
}

func assertAgentCatalogEntry(t *testing.T, got AgentCatalogEntry, want agents.AgentDefinition) {
	t.Helper()

	if got.ID != want.ID ||
		got.Version != want.Version ||
		got.Scope != want.Scope ||
		got.ScopeIdentity != want.ScopeIdentity ||
		got.Name != want.Name ||
		got.RoleSpec != want.RoleSpec ||
		got.Status != want.Status {
		t.Fatalf("agent catalog entry = %#v, want bound fields from %#v", got, want)
	}
}

func assertRuntimeCatalogOrder(t *testing.T, entries []RuntimeCatalogEntry, wantIDs []string) {
	t.Helper()

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.ID)
	}
	if !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("runtime catalog order = %#v, want %#v", got, wantIDs)
	}
}

func assertRuntimeCatalogEntry(t *testing.T, got RuntimeCatalogEntry, wantProbeID string, wantCapabilities []string, wantModels []string) {
	t.Helper()

	if got.SourceProbeID != wantProbeID {
		t.Fatalf("SourceProbeID = %q, want %q", got.SourceProbeID, wantProbeID)
	}
	if got.Status != runtime.RuntimeOnline {
		t.Fatalf("runtime status = %q, want online", got.Status)
	}
	if !reflect.DeepEqual(got.ObservedCapabilities, wantCapabilities) {
		t.Fatalf("ObservedCapabilities = %#v, want %#v", got.ObservedCapabilities, wantCapabilities)
	}
	if !reflect.DeepEqual(got.ModelIDs, wantModels) {
		t.Fatalf("ModelIDs = %#v, want %#v", got.ModelIDs, wantModels)
	}
}

func containsRuntimeCatalogID(entries []RuntimeCatalogEntry, id string) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func assertStringSet(t *testing.T, label string, got []string, want []string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func assertZeroTeamDraftCatalog(t *testing.T, snapshot TeamDraftCatalogSnapshot) {
	t.Helper()

	if snapshot.Digest() != "" ||
		snapshot.RuntimeDiscoveryDigest() != "" ||
		len(snapshot.Agents()) != 0 ||
		len(snapshot.Runtimes()) != 0 ||
		len(snapshot.Skills()) != 0 ||
		len(snapshot.Members()) != 0 ||
		len(snapshot.Permissions()) != 0 ||
		snapshot.BudgetCeiling() != 0 ||
		snapshot.ConcurrencyCeiling() != 0 ||
		snapshot.MaxCounts() != (TeamDraftCatalogMaxCounts{}) {
		t.Fatalf("snapshot = %#v, want no usable partial catalog", snapshot)
	}
}

func assertLowercaseSHA256Digest(t *testing.T, digest string) {
	t.Helper()

	if len(digest) != 64 {
		t.Fatalf("digest %q length = %d, want 64", digest, len(digest))
	}
	for _, r := range digest {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Fatalf("digest %q contains non-lowercase-hex rune %q", digest, r)
		}
	}
}

func assertTeamDraftCatalogProductionImports(t *testing.T, filename string) {
	t.Helper()

	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("expected production catalog file %s: %v", filename, err)
	}

	forbidden := map[string]bool{
		"database/sql":  true,
		"net":           true,
		"net/http":      true,
		"os":            true,
		"os/exec":       true,
		"path/filepath": true,
		"syscall":       true,

		"loom-pi-rebuild/internal/api":         true,
		"loom-pi-rebuild/internal/app":         true,
		"loom-pi-rebuild/internal/config":      true,
		"loom-pi-rebuild/internal/credentials": true,
		"loom-pi-rebuild/internal/evidence":    true,
		"loom-pi-rebuild/internal/journal":     true,
		"loom-pi-rebuild/internal/mode":        true,
		"loom-pi-rebuild/internal/projection":  true,
		"loom-pi-rebuild/internal/supervisor":  true,
	}

	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", filename, err)
	}
	for _, imported := range file.Imports {
		importPath := strings.Trim(imported.Path.Value, `"`)
		if forbidden[importPath] {
			t.Fatalf("%s imports forbidden boundary package %q", filename, importPath)
		}
		if strings.HasPrefix(importPath, "loom-pi-rebuild/internal/") &&
			importPath != "loom-pi-rebuild/internal/agents" &&
			importPath != "loom-pi-rebuild/internal/runtime" {
			t.Fatalf("%s imports non-domain internal package %q", filename, importPath)
		}
	}
}
