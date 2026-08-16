package projection

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"

	_ "modernc.org/sqlite"
)

func TestLocalProductSetupProjectionRebuildsSavedTeamAndCredentialMetadata(
	t *testing.T,
) {
	database, store := openLocalProductSetupProjectionStore(t)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	definition, definitions, profiles := projectionSetupDefinition(t)
	budget := int64(32)
	profiles[0].Budget = &budget
	fallbackProfile := profiles[1]
	parallelProfile := profiles[1]
	parallelProfile.ID = "profile-sub-parallel"
	parallelProfile.ProviderID = "deepseek"
	parallelProfile.ProviderAccountID = "deepseek.primary"
	parallelProfile.ModelID = "deepseek-chat"
	parallelProfile.AuthMode = loomruntime.AuthBrokered
	parallelProfile.EndpointFingerprint = projectionSetupDigest("parallel-endpoint")
	parallelProfile.CredentialReference = "credential-ref-deepseek-parallel"
	parallelProfile.CredentialRevision = 5
	profiles = append(profiles, parallelProfile)
	if _, err := writer.SaveTeamDefinition(
		context.Background(),
		state.TeamDefinitionSaveCommand{
			CommandID:       "save-team-1",
			ExpectedHead:    0,
			OccurredAt:      time.Unix(2000, 0).UTC(),
			Definition:      definition,
			Definitions:     definitions,
			RuntimeProfiles: profiles,
			DraftID:         "draft-1",
			DraftRevision:   4,
			CatalogDigest:   projectionSetupDigest("catalog"),
			ContentDigest:   projectionSetupDigest("content"),
			BindingDigest:   projectionSetupDigest("binding"),
			Configuration: state.TeamConfigurationSnapshot{
				RequestedConcurrency: 1,
				MaximumBudgetCredits: 25,
				RoleBindings: []state.TeamConfigurationRoleBinding{
					{
						Kind:              "main",
						AgentDefinitionID: "agent-main",
						RuntimeProfileID:  "profile-main",
						RuntimeInstanceID: "runtime-pi",
						ModelID:           "model-a",
						ExecutionProfile: &state.TeamConfigurationExecutionProfile{
							Version: 1, ID: profiles[0].ID,
							HarnessAdapter: profiles[0].AdapterType,
							ProviderID:     profiles[0].ProviderID, ModelID: profiles[0].ModelID,
							AuthMode:           profiles[0].AuthMode,
							TimeoutNanoseconds: int64(profiles[0].Timeout), Budget: &budget,
							RequiredCapabilities: append(
								[]string(nil), profiles[0].RequiredCapabilities...,
							),
						},
						FallbackRoute: &state.TeamConfigurationFallbackRoute{
							Version: 1, RuntimeProfileID: fallbackProfile.ID,
							RuntimeInstanceID: "runtime-pi", ModelID: fallbackProfile.ModelID,
							ExecutionProfile: &state.TeamConfigurationExecutionProfile{
								Version: 1, ID: fallbackProfile.ID,
								HarnessAdapter:    fallbackProfile.AdapterType,
								ProviderID:        fallbackProfile.ProviderID,
								ProviderAccountID: fallbackProfile.ProviderAccountID,
								ModelID:           fallbackProfile.ModelID, AuthMode: fallbackProfile.AuthMode,
								TimeoutNanoseconds: int64(fallbackProfile.Timeout),
								RequiredCapabilities: append(
									[]string(nil), fallbackProfile.RequiredCapabilities...,
								),
							},
							ApprovalRequired: true,
						},
						SkillRevisions: []state.TeamConfigurationSkillRevision{},
						PermissionIDs:  []string{},
						ResourceIDs:    []string{},
					},
					{
						Kind:              "subagent",
						AgentDefinitionID: "agent-sub",
						RuntimeProfileID:  "profile-sub",
						RuntimeInstanceID: "runtime-pi",
						ModelID:           "model-a",
						ExecutionProfile: &state.TeamConfigurationExecutionProfile{
							Version: 1, ID: profiles[1].ID,
							HarnessAdapter: profiles[1].AdapterType,
							ProviderID:     profiles[1].ProviderID, ModelID: profiles[1].ModelID,
							AuthMode:           profiles[1].AuthMode,
							TimeoutNanoseconds: int64(profiles[1].Timeout),
							RequiredCapabilities: append(
								[]string(nil), profiles[1].RequiredCapabilities...,
							),
						},
						ParallelRouteSet: &state.TeamConfigurationParallelRouteSet{
							Version: 1,
							AdditionalRoutes: []state.TeamConfigurationExecutionRoute{{
								Version: 1, RuntimeProfileID: parallelProfile.ID,
								RuntimeInstanceID: "runtime-pi", ModelID: parallelProfile.ModelID,
								ExecutionProfile: &state.TeamConfigurationExecutionProfile{
									Version: 1, ID: parallelProfile.ID,
									HarnessAdapter:    parallelProfile.AdapterType,
									ProviderID:        parallelProfile.ProviderID,
									ProviderAccountID: parallelProfile.ProviderAccountID,
									ModelID:           parallelProfile.ModelID, AuthMode: parallelProfile.AuthMode,
									EndpointFingerprint: parallelProfile.EndpointFingerprint,
									CredentialReference: parallelProfile.CredentialReference,
									CredentialRevision:  parallelProfile.CredentialRevision,
									TimeoutNanoseconds:  int64(parallelProfile.Timeout),
									RequiredCapabilities: append(
										[]string(nil), parallelProfile.RequiredCapabilities...,
									),
								},
							}},
							SynthesisRoute: state.TeamConfigurationExecutionRoute{
								Version: 1, RuntimeProfileID: profiles[1].ID,
								RuntimeInstanceID: "runtime-pi", ModelID: profiles[1].ModelID,
								ExecutionProfile: &state.TeamConfigurationExecutionProfile{
									Version: 1, ID: profiles[1].ID,
									HarnessAdapter: profiles[1].AdapterType,
									ProviderID:     profiles[1].ProviderID, ModelID: profiles[1].ModelID,
									AuthMode:           profiles[1].AuthMode,
									TimeoutNanoseconds: int64(profiles[1].Timeout),
									RequiredCapabilities: append(
										[]string(nil), profiles[1].RequiredCapabilities...,
									),
								},
							},
						},
						SkillRevisions: []state.TeamConfigurationSkillRevision{},
						PermissionIDs:  []string{},
						ResourceIDs:    []string{},
					},
				},
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "configure-minimax-1",
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-1",
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(2001, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	); err != nil {
		t.Fatal(err)
	}

	readModel := New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	view := readModel.GlobalReadView()
	team, ok := view.TeamDefinition("team-1")
	if !ok ||
		team.ID != "team-1" ||
		team.Status != "active" ||
		team.DefinitionDigest != definition.Digest() ||
		len(team.Configuration.RoleBindings) != 2 {
		t.Fatalf("team = %#v, ok = %v", team, ok)
	}
	mainBinding := team.Configuration.RoleBindings[0]
	if !mainBinding.ExecutionProfileAvailable ||
		mainBinding.ExecutionProfile.Version != 1 ||
		mainBinding.ExecutionProfile.ID != "profile-main" ||
		mainBinding.ExecutionProfile.ProviderID != "local" ||
		mainBinding.ExecutionProfile.Timeout != time.Minute ||
		mainBinding.ExecutionProfile.Budget == nil ||
		*mainBinding.ExecutionProfile.Budget != 32 ||
		len(mainBinding.ExecutionProfile.RequiredCapabilities) != 1 ||
		!mainBinding.FallbackRouteAvailable ||
		mainBinding.FallbackRoute.Version != 1 ||
		!mainBinding.FallbackRoute.ApprovalRequired ||
		mainBinding.FallbackRoute.ExecutionProfile.ID != fallbackProfile.ID ||
		!team.Configuration.RoleBindings[1].ExecutionProfileAvailable ||
		!team.Configuration.RoleBindings[1].ParallelRouteSetAvailable ||
		team.Configuration.RoleBindings[1].ParallelRouteSet.Version != 1 ||
		len(team.Configuration.RoleBindings[1].ParallelRouteSet.AdditionalRoutes) != 1 ||
		team.Configuration.RoleBindings[1].ParallelRouteSet.AdditionalRoutes[0].
			ExecutionProfile.ProviderAccountID != "deepseek.primary" ||
		team.Configuration.RoleBindings[1].ParallelRouteSet.SynthesisRoute.
			ExecutionProfile.ID != profiles[1].ID {
		t.Fatalf("execution Profile projection = %#v", team.Configuration.RoleBindings)
	}
	credential, ok := view.ProviderCredential("minimax")
	if !ok ||
		credential.ProviderID != "minimax" ||
		credential.AuthMode != "brokered" ||
		credential.Status != "configured" ||
		credential.CredentialReference != "credential-ref-1" {
		t.Fatalf("credential = %#v, ok = %v", credential, ok)
	}

	team.Configuration.RoleBindings[0].PermissionIDs = append(
		team.Configuration.RoleBindings[0].PermissionIDs,
		"mutated",
	)
	*team.Configuration.RoleBindings[0].ExecutionProfile.Budget = 999
	team.Configuration.RoleBindings[0].ExecutionProfile.RequiredCapabilities[0] = "mutated"
	team.Configuration.RoleBindings[0].FallbackRoute.ExecutionProfile.RequiredCapabilities[0] = "mutated"
	team.Configuration.RoleBindings[1].ParallelRouteSet.AdditionalRoutes[0].
		ExecutionProfile.RequiredCapabilities[0] = "mutated"
	again, _ := view.TeamDefinition("team-1")
	if len(again.Configuration.RoleBindings[0].PermissionIDs) != 0 ||
		*again.Configuration.RoleBindings[0].ExecutionProfile.Budget != 32 ||
		again.Configuration.RoleBindings[0].ExecutionProfile.RequiredCapabilities[0] != "rpc" ||
		again.Configuration.RoleBindings[0].FallbackRoute.ExecutionProfile.RequiredCapabilities[0] != "rpc" ||
		again.Configuration.RoleBindings[1].ParallelRouteSet.AdditionalRoutes[0].
			ExecutionProfile.RequiredCapabilities[0] != "rpc" {
		t.Fatal("GlobalReadView returned mutable saved Team configuration")
	}
}

func TestLocalProductSetupProjectionArchiveRestoreAndFailurePreservation(
	t *testing.T,
) {
	database, store := openLocalProductSetupProjectionStore(t)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	definition, definitions, profiles := projectionSetupDefinition(t)
	save := state.TeamDefinitionSaveCommand{
		CommandID:       "save-team-1",
		OccurredAt:      time.Unix(2100, 0).UTC(),
		Definition:      definition,
		Definitions:     definitions,
		RuntimeProfiles: profiles,
		DraftID:         "draft-1",
		DraftRevision:   4,
		CatalogDigest:   projectionSetupDigest("catalog"),
		ContentDigest:   projectionSetupDigest("content"),
		BindingDigest:   projectionSetupDigest("binding"),
		Configuration: state.TeamConfigurationSnapshot{
			RequestedConcurrency: 1,
			MaximumBudgetCredits: 25,
			RoleBindings: []state.TeamConfigurationRoleBinding{
				{
					Kind:              "main",
					AgentDefinitionID: "agent-main",
					RuntimeProfileID:  "profile-main",
					RuntimeInstanceID: "runtime-pi",
					ModelID:           "model-a",
					SkillRevisions:    []state.TeamConfigurationSkillRevision{},
					PermissionIDs:     []string{},
					ResourceIDs:       []string{},
				},
				{
					Kind:              "subagent",
					AgentDefinitionID: "agent-sub",
					RuntimeProfileID:  "profile-sub",
					RuntimeInstanceID: "runtime-pi",
					ModelID:           "model-a",
					SkillRevisions:    []state.TeamConfigurationSkillRevision{},
					PermissionIDs:     []string{},
					ResourceIDs:       []string{},
				},
			},
		},
	}
	if _, err := writer.SaveTeamDefinition(context.Background(), save); err != nil {
		t.Fatal(err)
	}
	readModel := New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := readModel.GlobalReadView()
	if _, err := writer.SetTeamDefinitionStatus(
		context.Background(),
		state.TeamDefinitionStatusCommand{
			CommandID:    "archive-team-1",
			DefinitionID: "team-1",
			ExpectedHead: 1,
			OccurredAt:   time.Unix(2101, 0).UTC(),
			Status:       "archived",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	archived, _ := readModel.GlobalReadView().TeamDefinition("team-1")
	if archived.Status != "archived" {
		t.Fatalf("archived = %#v", archived)
	}
	if _, err := writer.SetTeamDefinitionStatus(
		context.Background(),
		state.TeamDefinitionStatusCommand{
			CommandID:    "restore-team-1",
			DefinitionID: "team-1",
			ExpectedHead: 2,
			OccurredAt:   time.Unix(2102, 0).UTC(),
			Status:       "active",
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	restored, _ := readModel.GlobalReadView().TeamDefinition("team-1")
	if restored.Status != "active" ||
		restored.DefinitionDigest != archived.DefinitionDigest ||
		before.Version() == readModel.GlobalReadView().Version() {
		t.Fatalf("restored = %#v", restored)
	}
}

func TestLocalProductSetupProjectionRebuildsCatalogProviderCredential(
	t *testing.T,
) {
	database, store := openLocalProductSetupProjectionStore(t)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "configure-deepseek-1",
			ProviderID:          "deepseek",
			CredentialReference: "credential-ref-deepseek-1",
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(2100, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	); err != nil {
		t.Fatalf("CommitCredentialMetadata() error = %v", err)
	}
	readModel := New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	credential, ok := readModel.GlobalReadView().ProviderCredential("deepseek")
	if !ok || credential.ProviderID != "deepseek" ||
		credential.ProviderAccountID != "deepseek.primary" ||
		credential.CredentialReference != "credential-ref-deepseek-1" ||
		credential.Status != "configured" || credential.Revision != 1 {
		t.Fatalf("credential = %#v, ok = %v", credential, ok)
	}
	account, accountOK := readModel.GlobalReadView().ProviderAccountCredential(
		"deepseek", "deepseek.primary",
	)
	if !accountOK || account != credential {
		t.Fatalf("account = %#v, ok = %v", account, accountOK)
	}
}

func TestLocalProductSetupProjectionKeepsSameProviderAccountsIndependent(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	for index, accountID := range []string{"deepseek.work", "deepseek.personal"} {
		_, err := writer.CommitCredentialMetadata(
			context.Background(), credentials.MetadataCommand{
				CommandID: "configure-" + accountID, ProviderID: "deepseek",
				ProviderAccountID:   accountID,
				CredentialReference: "credential-ref-deepseek-account-" + string(rune('a'+index)),
				ExpectedRevision:    0,
				OccurredAt:          time.Unix(2200+int64(index), 0).UTC(),
				Status:              credentials.CredentialConfigured,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	readModel := New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	view := readModel.GlobalReadView()
	work, workOK := view.ProviderAccountCredential("deepseek", "deepseek.work")
	personal, personalOK := view.ProviderAccountCredential("deepseek", "deepseek.personal")
	if !workOK || !personalOK || work.ProviderAccountID != "deepseek.work" ||
		personal.ProviderAccountID != "deepseek.personal" ||
		work.CredentialReference == personal.CredentialReference ||
		work.Revision != 1 || personal.Revision != 1 {
		t.Fatalf("work=%#v/%t personal=%#v/%t", work, workOK, personal, personalOK)
	}
	if _, ok := view.ProviderAccountCredential("openai", "deepseek.work"); ok {
		t.Fatal("account lookup crossed Provider boundary")
	}
	accounts := view.ProviderAccountCredentials("deepseek")
	if len(accounts) != 2 ||
		accounts[0].ProviderAccountID != "deepseek.personal" ||
		accounts[1].ProviderAccountID != "deepseek.work" {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func openLocalProductSetupProjectionStore(
	t *testing.T,
) (*sql.DB, *journal.Store) {
	t.Helper()
	database, err := sql.Open(
		"sqlite",
		filepath.Join(t.TempDir(), "state.sqlite"),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	return database, journal.NewStore(database)
}

func projectionSetupDefinition(
	t *testing.T,
) (teams.TeamDefinition, []agents.AgentDefinition, []loomruntime.RuntimeProfile) {
	t.Helper()
	definitions := []agents.AgentDefinition{
		{
			ID:       "agent-main",
			Version:  1,
			Scope:    agents.ScopeReusable,
			Name:     "Main",
			RoleSpec: "Coordinate",
			Status:   agents.DefinitionActive,
		},
		{
			ID:       "agent-sub",
			Version:  1,
			Scope:    agents.ScopeReusable,
			Name:     "Sub",
			RoleSpec: "Implement",
			Status:   agents.DefinitionActive,
		},
	}
	profiles := []loomruntime.RuntimeProfile{
		{
			ID:                   "profile-main",
			AdapterType:          "pi",
			ProviderID:           "local",
			ModelID:              "model-a",
			AuthMode:             loomruntime.AuthNative,
			RequiredCapabilities: []string{"rpc"},
			Timeout:              time.Minute,
		},
		{
			ID:                   "profile-sub",
			AdapterType:          "pi",
			ProviderID:           "local",
			ModelID:              "model-a",
			AuthMode:             loomruntime.AuthNative,
			RequiredCapabilities: []string{"rpc"},
			Timeout:              time.Minute,
		},
	}
	definition, err := teams.BuildTeamDefinition(
		teams.TeamDefinitionInput{
			ID:      "team-1",
			Version: 1,
			Scope:   teams.TeamDefinitionScopeReusable,
			Name:    "Fixture Team",
			Status:  teams.TeamDefinitionActive,
			Roles: []teams.TeamDefinitionRole{
				{
					Kind:              teams.TeamDefinitionRoleMain,
					AgentDefinitionID: "agent-main",
					RuntimeProfileID:  "profile-main",
					Responsibility:    "Coordinate",
				},
				{
					Kind:              teams.TeamDefinitionRoleSubAgent,
					AgentDefinitionID: "agent-sub",
					RuntimeProfileID:  "profile-sub",
					Responsibility:    "Implement",
				},
			},
		},
		definitions,
		profiles,
	)
	if err != nil {
		t.Fatal(err)
	}
	return definition, definitions, profiles
}

func projectionSetupDigest(seed string) string {
	const hexadecimal = "0123456789abcdef"
	output := make([]byte, 64)
	for index := range output {
		output[index] = hexadecimal[(index+len(seed))%len(hexadecimal)]
	}
	return string(output)
}
