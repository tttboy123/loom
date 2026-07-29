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
	again, _ := view.TeamDefinition("team-1")
	if len(again.Configuration.RoleBindings[0].PermissionIDs) != 0 {
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
