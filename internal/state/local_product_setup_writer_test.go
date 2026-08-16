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
	budget := int64(64)
	profiles[0].ProviderID = "deepseek"
	profiles[0].ProviderAccountID = "deepseek.primary"
	profiles[0].AuthMode = loomruntime.AuthBrokered
	profiles[0].EndpointFingerprint = localProductSetupDigest("endpoint")
	profiles[0].CredentialReference = "credential-ref-deepseek-primary"
	profiles[0].CredentialRevision = 4
	profiles[0].Budget = &budget
	profileSnapshot := &TeamConfigurationExecutionProfile{
		Version: 1, ID: profiles[0].ID,
		HarnessAdapter: profiles[0].AdapterType, ProviderID: profiles[0].ProviderID,
		ProviderAccountID: profiles[0].ProviderAccountID, ModelID: profiles[0].ModelID,
		AuthMode:            profiles[0].AuthMode,
		EndpointFingerprint: profiles[0].EndpointFingerprint,
		CredentialReference: profiles[0].CredentialReference,
		CredentialRevision:  profiles[0].CredentialRevision,
		TimeoutNanoseconds:  int64(profiles[0].Timeout), Budget: &budget,
		RequiredCapabilities: append([]string(nil), profiles[0].RequiredCapabilities...),
	}
	fallbackSnapshot := &TeamConfigurationExecutionProfile{
		Version: 1, ID: profiles[1].ID,
		HarnessAdapter: profiles[1].AdapterType, ProviderID: profiles[1].ProviderID,
		ProviderAccountID: profiles[1].ProviderAccountID, ModelID: profiles[1].ModelID,
		AuthMode:             profiles[1].AuthMode,
		EndpointFingerprint:  profiles[1].EndpointFingerprint,
		CredentialReference:  profiles[1].CredentialReference,
		CredentialRevision:   profiles[1].CredentialRevision,
		TimeoutNanoseconds:   int64(profiles[1].Timeout),
		RequiredCapabilities: append([]string(nil), profiles[1].RequiredCapabilities...),
	}
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
					ExecutionProfile:  profileSnapshot,
					FallbackRoute: &TeamConfigurationFallbackRoute{
						Version: 1, RuntimeProfileID: profiles[1].ID,
						RuntimeInstanceID: "runtime-pi", ModelID: profiles[1].ModelID,
						ExecutionProfile: fallbackSnapshot, ApprovalRequired: true,
					},
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
	command.Configuration.RoleBindings[0].ExecutionProfile.CredentialRevision++
	if _, err := writer.SaveTeamDefinition(
		context.Background(),
		command,
	); !errors.Is(err, ErrInvalidLocalProductSetupWrite) {
		t.Fatalf("SaveTeamDefinition(profile drift) error = %v", err)
	}
	command.Configuration.RoleBindings[0].ExecutionProfile.CredentialRevision--
	command.Configuration.RoleBindings[0].FallbackRoute.ApprovalRequired = false
	if _, err := writer.SaveTeamDefinition(
		context.Background(),
		command,
	); !errors.Is(err, ErrInvalidLocalProductSetupWrite) {
		t.Fatalf("SaveTeamDefinition(unapproved fallback) error = %v", err)
	}
	command.Configuration.RoleBindings[0].FallbackRoute.ApprovalRequired = true
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
	if !bytes.Contains(
		events[0].PayloadJSON,
		[]byte(`"credential_reference":"credential-ref-deepseek-primary"`),
	) {
		t.Fatalf("missing frozen credential reference: %s", events[0].PayloadJSON)
	}
	if !bytes.Contains(events[0].PayloadJSON, []byte(`"fallback_route"`)) ||
		!bytes.Contains(events[0].PayloadJSON, []byte(`"approval_required":true`)) {
		t.Fatalf("missing governed fallback route: %s", events[0].PayloadJSON)
	}
	for _, event := range events {
		for _, forbidden := range [][]byte{
			[]byte("deepseek-live-secret"),
			[]byte("Authorization"),
			[]byte("api_key"),
			[]byte("secret_body"),
		} {
			if bytes.Contains(event.PayloadJSON, forbidden) {
				t.Fatalf("unexpected secret material in team event: %s", event.PayloadJSON)
			}
		}
	}
}

func TestLocalProductSetupWriterPersistsVersionedParallelRouteSet(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	definition, definitions, profiles := localProductSetupDefinition(t)
	primary := profiles[0]
	alternate := primary
	alternate.ID = "profile-main-parallel"
	alternate.ProviderID = "deepseek"
	alternate.ProviderAccountID = "deepseek.primary"
	alternate.ModelID = "deepseek-chat"
	alternate.AuthMode = loomruntime.AuthBrokered
	alternate.EndpointFingerprint = localProductSetupDigest("parallel-endpoint")
	alternate.CredentialReference = "credential-ref-deepseek-parallel"
	alternate.CredentialRevision = 7
	profiles = append(profiles, alternate)
	snapshot := func(profile loomruntime.RuntimeProfile) *TeamConfigurationExecutionProfile {
		return &TeamConfigurationExecutionProfile{
			Version: 1, ID: profile.ID, HarnessAdapter: profile.AdapterType,
			ProviderID: profile.ProviderID, ProviderAccountID: profile.ProviderAccountID,
			ModelID: profile.ModelID, AuthMode: profile.AuthMode,
			EndpointFingerprint:  profile.EndpointFingerprint,
			CredentialReference:  profile.CredentialReference,
			CredentialRevision:   profile.CredentialRevision,
			TimeoutNanoseconds:   int64(profile.Timeout),
			RequiredCapabilities: append([]string(nil), profile.RequiredCapabilities...),
		}
	}
	route := func(profile loomruntime.RuntimeProfile) TeamConfigurationExecutionRoute {
		return TeamConfigurationExecutionRoute{
			Version: 1, RuntimeProfileID: profile.ID,
			RuntimeInstanceID: "runtime-pi", ModelID: profile.ModelID,
			ExecutionProfile: snapshot(profile),
		}
	}
	command := TeamDefinitionSaveCommand{
		CommandID: "save-parallel-route-set", OccurredAt: time.Unix(1050, 0).UTC(),
		Definition: definition, Definitions: definitions, RuntimeProfiles: profiles,
		DraftID: "draft-parallel", DraftRevision: 1,
		CatalogDigest: localProductSetupDigest("parallel-catalog"),
		ContentDigest: localProductSetupDigest("parallel-content"),
		BindingDigest: localProductSetupDigest("parallel-binding"),
		Configuration: TeamConfigurationSnapshot{
			RequestedConcurrency: 2, MaximumBudgetCredits: 100,
			RoleBindings: []TeamConfigurationRoleBinding{
				{
					Kind: "main", AgentDefinitionID: "agent-main",
					RuntimeProfileID: primary.ID, RuntimeInstanceID: "runtime-pi",
					ModelID: primary.ModelID, ExecutionProfile: snapshot(primary),
					ParallelRouteSet: &TeamConfigurationParallelRouteSet{
						Version:          1,
						AdditionalRoutes: []TeamConfigurationExecutionRoute{route(alternate)},
						SynthesisRoute:   route(primary),
					},
					SkillRevisions: []TeamConfigurationSkillRevision{},
					PermissionIDs:  []string{}, ResourceIDs: []string{},
				},
				{
					Kind: "subagent", AgentDefinitionID: "agent-sub",
					RuntimeProfileID: profiles[1].ID, RuntimeInstanceID: "runtime-pi",
					ModelID:        profiles[1].ModelID,
					SkillRevisions: []TeamConfigurationSkillRevision{},
					PermissionIDs:  []string{}, ResourceIDs: []string{},
				},
			},
		},
	}
	command.Configuration.RoleBindings[0].ParallelRouteSet.AdditionalRoutes[0].
		ExecutionProfile.CredentialRevision++
	if _, err := writer.SaveTeamDefinition(context.Background(), command); !errors.Is(err, ErrInvalidLocalProductSetupWrite) {
		t.Fatalf("SaveTeamDefinition(route substitution) error = %v", err)
	}
	command.Configuration.RoleBindings[0].ParallelRouteSet.AdditionalRoutes[0].
		ExecutionProfile.CredentialRevision--
	command.Configuration.RoleBindings[0].FallbackRoute = &TeamConfigurationFallbackRoute{
		Version: 1, RuntimeProfileID: alternate.ID, RuntimeInstanceID: "runtime-pi",
		ModelID: alternate.ModelID, ExecutionProfile: snapshot(alternate),
		ApprovalRequired: true,
	}
	if _, err := writer.SaveTeamDefinition(context.Background(), command); !errors.Is(err, ErrInvalidLocalProductSetupWrite) {
		t.Fatalf("SaveTeamDefinition(parallel plus fallback) error = %v", err)
	}
	command.Configuration.RoleBindings[0].FallbackRoute = nil
	if _, err := writer.SaveTeamDefinition(context.Background(), command); err != nil {
		t.Fatalf("SaveTeamDefinition(parallel route set) error = %v", err)
	}
	events, err := store.ReadStream(context.Background(), "team-definition/team-1")
	if err != nil || len(events) != 1 {
		t.Fatalf("ReadStream() = %d, %v", len(events), err)
	}
	for _, required := range [][]byte{
		[]byte(`"parallel_route_set"`),
		[]byte(`"additional_routes"`),
		[]byte(`"synthesis_route"`),
		[]byte(`"provider_account_id":"deepseek.primary"`),
		[]byte(`"credential_revision":7`),
	} {
		if !bytes.Contains(events[0].PayloadJSON, required) {
			t.Fatalf("missing parallel route authority %q: %s", required, events[0].PayloadJSON)
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

func TestLocalProductSetupWriterCommitsCatalogProviderMetadata(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	result, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "credential-deepseek-1",
			ProviderID:          "deepseek",
			CredentialReference: "credential-ref-deepseek-1",
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(1250, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	)
	if err != nil {
		t.Fatalf("CommitCredentialMetadata() error = %v", err)
	}
	if result.ProviderID != "deepseek" || result.Revision != 1 {
		t.Fatalf("result = %#v", result)
	}
	events, err := store.ReadStream(
		context.Background(), "provider-credential/deepseek",
	)
	if err != nil || len(events) != 1 ||
		events[0].Type != "ProviderCredentialConfigured" {
		t.Fatalf("events = %#v, error = %v", events, err)
	}
}

func TestLocalProductSetupWriterCommitsIndependentProviderAccountStream(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	result, err := writer.CommitCredentialMetadata(
		context.Background(), credentials.MetadataCommand{
			CommandID: "credential-deepseek-work-1", ProviderID: "deepseek",
			ProviderAccountID:   "deepseek.work",
			CredentialReference: "credential-ref-deepseek-work-1",
			ExpectedRevision:    0, OccurredAt: time.Unix(1251, 0).UTC(),
			Status: credentials.CredentialConfigured,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderAccountID != "deepseek.work" || result.Revision != 1 {
		t.Fatalf("result = %#v", result)
	}
	events, err := store.ReadStream(
		context.Background(), "provider-account-credential/deepseek.work",
	)
	if err != nil || len(events) != 1 ||
		events[0].Type != "ProviderAccountCredentialConfigured" ||
		!bytes.Contains(events[0].PayloadJSON, []byte(`"provider_account_id":"deepseek.work"`)) ||
		bytes.Contains(events[0].PayloadJSON, []byte("private-key")) {
		t.Fatalf("events = %#v, error = %v", events, err)
	}
}

func TestLocalProductSetupWriterKeepsPrimaryAccountOnLegacyStream(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	result, err := writer.CommitCredentialMetadata(
		context.Background(), credentials.MetadataCommand{
			CommandID: "credential-deepseek-primary-1", ProviderID: "deepseek",
			ProviderAccountID:   "deepseek.primary",
			CredentialReference: "credential-ref-deepseek-primary-1",
			ExpectedRevision:    0, OccurredAt: time.Unix(1252, 0).UTC(),
			Status: credentials.CredentialConfigured,
		},
	)
	if err != nil || result.ProviderAccountID != "deepseek.primary" {
		t.Fatalf("result=%#v error=%v", result, err)
	}
	events, err := store.ReadStream(context.Background(), "provider-credential/deepseek")
	if err != nil || len(events) != 1 || events[0].Type != "ProviderCredentialConfigured" ||
		bytes.Contains(events[0].PayloadJSON, []byte("provider_account_id")) {
		t.Fatalf("events=%#v error=%v", events, err)
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
