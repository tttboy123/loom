package state

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/teams"

	_ "modernc.org/sqlite"
)

func TestLocalProductSetupWriterSavesArchivesAndRestoresTeamDefinition(
	t *testing.T,
) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatalf("NewLocalProductSetupWriter() error = %v", err)
	}
	definition, definitions, profiles := localProductSetupDefinition(t)
	command := TeamDefinitionSaveCommand{
		CommandID:       "save-command-1",
		ExpectedHead:    0,
		OccurredAt:      time.Unix(1000, 0).UTC(),
		Definition:      definition,
		Definitions:     definitions,
		RuntimeProfiles: profiles,
		DraftID:         "draft-1",
		DraftRevision:   7,
		CatalogDigest:   localProductSetupDigest("catalog"),
		ContentDigest:   localProductSetupDigest("content"),
		BindingDigest:   localProductSetupDigest("binding"),
		Configuration: TeamConfigurationSnapshot{
			RequestedConcurrency: 2,
			MaximumBudgetCredits: 100,
			RoleBindings: []TeamConfigurationRoleBinding{
				{
					Kind:              "main",
					AgentDefinitionID: "agent-main",
					RuntimeProfileID:  "profile-main",
					RuntimeInstanceID: "runtime-pi",
					ModelID:           "model-a",
					SkillRevisions: []TeamConfigurationSkillRevision{
						{
							ID:       "skill-review",
							Revision: 2,
							Digest:   localProductSetupDigest("skill"),
						},
					},
					PermissionIDs: []string{"permission-read"},
					ResourceIDs:   []string{"resource-repo"},
				},
				{
					Kind:              "subagent",
					AgentDefinitionID: "agent-sub",
					RuntimeProfileID:  "profile-sub",
					RuntimeInstanceID: "runtime-pi",
					ModelID:           "model-a",
					SkillRevisions:    []TeamConfigurationSkillRevision{},
					PermissionIDs:     []string{"permission-read"},
					ResourceIDs:       []string{"resource-repo"},
				},
			},
		},
	}
	saved, err := writer.SaveTeamDefinition(context.Background(), command)
	if err != nil {
		t.Fatalf("SaveTeamDefinition() error = %v", err)
	}
	if saved.EventType != "TeamDefinitionSaved" ||
		saved.StreamID != "team-definition/team-1" ||
		saved.Sequence != 1 {
		t.Fatalf("SaveTeamDefinition() = %#v", saved)
	}

	archived, err := writer.SetTeamDefinitionStatus(
		context.Background(),
		TeamDefinitionStatusCommand{
			CommandID:    "archive-command-1",
			DefinitionID: "team-1",
			ExpectedHead: 1,
			OccurredAt:   time.Unix(1001, 0).UTC(),
			Status:       "archived",
		},
	)
	if err != nil {
		t.Fatalf("archive error = %v", err)
	}
	restored, err := writer.SetTeamDefinitionStatus(
		context.Background(),
		TeamDefinitionStatusCommand{
			CommandID:    "restore-command-1",
			DefinitionID: "team-1",
			ExpectedHead: 2,
			OccurredAt:   time.Unix(1002, 0).UTC(),
			Status:       "active",
		},
	)
	if err != nil {
		t.Fatalf("restore error = %v", err)
	}
	if archived.EventType != "TeamDefinitionArchived" ||
		restored.EventType != "TeamDefinitionRestored" ||
		restored.Sequence != 3 {
		t.Fatalf("status results = %#v %#v", archived, restored)
	}
	events, err := store.ReadStream(
		context.Background(),
		"team-definition/team-1",
	)
	if err != nil {
		t.Fatalf("ReadStream() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d", len(events))
	}
	for _, event := range events {
		if bytes.Contains(event.PayloadJSON, []byte("credential")) ||
			bytes.Contains(event.PayloadJSON, []byte("secret")) {
			t.Fatalf("unexpected credential material in team event: %s", event.PayloadJSON)
		}
	}
}

func TestLocalProductSetupWriterTeamSaveCASHasOneWinner(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	definition, definitions, profiles := localProductSetupDefinition(t)
	command := TeamDefinitionSaveCommand{
		CommandID:       "save-command-a",
		ExpectedHead:    0,
		OccurredAt:      time.Unix(1100, 0).UTC(),
		Definition:      definition,
		Definitions:     definitions,
		RuntimeProfiles: profiles,
		DraftID:         "draft-1",
		DraftRevision:   4,
		CatalogDigest:   localProductSetupDigest("catalog"),
		ContentDigest:   localProductSetupDigest("content"),
		BindingDigest:   localProductSetupDigest("binding"),
		Configuration: TeamConfigurationSnapshot{
			RequestedConcurrency: 1,
			MaximumBudgetCredits: 50,
			RoleBindings: []TeamConfigurationRoleBinding{
				{
					Kind:              "main",
					AgentDefinitionID: "agent-main",
					RuntimeProfileID:  "profile-main",
					RuntimeInstanceID: "runtime-pi",
					ModelID:           "model-a",
					SkillRevisions:    []TeamConfigurationSkillRevision{},
					PermissionIDs:     []string{},
					ResourceIDs:       []string{},
				},
				{
					Kind:              "subagent",
					AgentDefinitionID: "agent-sub",
					RuntimeProfileID:  "profile-sub",
					RuntimeInstanceID: "runtime-pi",
					ModelID:           "model-a",
					SkillRevisions:    []TeamConfigurationSkillRevision{},
					PermissionIDs:     []string{},
					ResourceIDs:       []string{},
				},
			},
		},
	}
	if _, err := writer.SaveTeamDefinition(context.Background(), command); err != nil {
		t.Fatalf("first save error = %v", err)
	}
	command.CommandID = "save-command-b"
	if _, err := writer.SaveTeamDefinition(
		context.Background(),
		command,
	); !errors.Is(err, journal.ErrStreamHeadConflict) {
		t.Fatalf("second save error = %v", err)
	}
}

func TestLocalProductSetupWriterCommitsOnlyNonSecretCredentialMetadata(
	t *testing.T,
) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	result, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "credential-command-1",
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-1",
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(1200, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	)
	if err != nil {
		t.Fatalf("CommitCredentialMetadata() error = %v", err)
	}
	if result.Revision != 1 ||
		result.Status != credentials.CredentialConfigured {
		t.Fatalf("result = %#v", result)
	}
	events, err := store.ReadStream(
		context.Background(),
		"provider-credential/minimax",
	)
	if err != nil {
		t.Fatalf("ReadStream() error = %v", err)
	}
	if len(events) != 1 ||
		events[0].Type != "ProviderCredentialConfigured" ||
		bytes.Contains(events[0].PayloadJSON, []byte("authorization")) ||
		bytes.Contains(events[0].PayloadJSON, []byte("secret")) {
		t.Fatalf("credential events = %#v", events)
	}
}

func TestLocalProductSetupWriterRejectsNonOpaqueCredentialReference(
	t *testing.T,
) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "credential-command-invalid-reference",
			ProviderID:          "minimax",
			CredentialReference: "not-an-opaque-reference",
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(1200, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	)
	if !errors.Is(err, ErrInvalidLocalProductSetupWrite) {
		t.Fatalf("CommitCredentialMetadata() error = %v", err)
	}
	events, readErr := store.ReadStream(
		context.Background(),
		"provider-credential/minimax",
	)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(events) != 0 {
		t.Fatalf("invalid reference wrote %d events", len(events))
	}
}

func openLocalProductSetupWriterStore(t *testing.T) *journal.Store {
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
	return journal.NewStore(database)
}

func localProductSetupDefinition(
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

func localProductSetupDigest(seed string) string {
	const hex = "0123456789abcdef"
	output := make([]byte, 64)
	for index := range output {
		output[index] = hex[(index+len(seed))%len(hex)]
	}
	return string(output)
}
