package app

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/teams"

	_ "modernc.org/sqlite"
)

type setupFixtureProbe struct {
	observations []loomruntime.RuntimeObservation
}

func (probe setupFixtureProbe) ID() string { return "setup-fixture-probe" }

func (probe setupFixtureProbe) ObserveRuntime(
	context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	return append([]loomruntime.RuntimeObservation(nil), probe.observations...), nil
}

type setupFixtureIdentity struct {
	next int
}

func (source *setupFixtureIdentity) NextSetupID(kind string) (string, error) {
	source.next++
	return kind + "-" + setupDecimal(source.next), nil
}

type setupFixtureNativeAuth struct {
	result NativeAuthObservation
}

func (observer setupFixtureNativeAuth) ObserveNativeAuth(
	context.Context,
) (NativeAuthObservation, error) {
	return observer.result, nil
}

type setupFixtureNativeAuthConnector struct {
	starts int
	closed bool
	err    error
}

func (connector *setupFixtureNativeAuthConnector) StartNativeAuth(
	context.Context,
) error {
	connector.starts++
	return connector.err
}

func (connector *setupFixtureNativeAuthConnector) Close() error {
	connector.closed = true
	return connector.err
}

type setupFixtureBroker struct {
	status credentials.MetadataResult
}

func (broker setupFixtureBroker) CredentialStatus(
	context.Context,
	string,
) (credentials.MetadataResult, error) {
	return broker.status, nil
}

type setupFixtureMutator struct {
	called *bool
}

type setupCapturingCredentialMutator struct {
	command credentials.CredentialCommand
}

func (mutator *setupCapturingCredentialMutator) Configure(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, errors.New("unexpected configure")
}

func (mutator *setupCapturingCredentialMutator) Verify(
	_ context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	mutator.command = command
	return credentials.MetadataResult{
		ProviderID: command.ProviderID, CredentialReference: command.CredentialReference,
		Revision: command.ExpectedRevision + 1, Status: credentials.CredentialVerified,
	}, nil
}

func (mutator *setupCapturingCredentialMutator) Replace(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, errors.New("unexpected replace")
}

func (mutator *setupCapturingCredentialMutator) Revoke(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	return credentials.MetadataResult{}, errors.New("unexpected revoke")
}

func (mutator setupFixtureMutator) Configure(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	*mutator.called = true
	return credentials.MetadataResult{}, nil
}

func (mutator setupFixtureMutator) Verify(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	*mutator.called = true
	return credentials.MetadataResult{}, nil
}

func (mutator setupFixtureMutator) Replace(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	*mutator.called = true
	return credentials.MetadataResult{}, nil
}

func (mutator setupFixtureMutator) Revoke(
	context.Context,
	credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	*mutator.called = true
	return credentials.MetadataResult{}, nil
}

type setupFixtureCatalogSource struct {
	catalog LocalProductSetupCatalog
	err     error
	calls   int
}

func (source *setupFixtureCatalogSource) CatalogForView(
	context.Context,
	projection.GlobalReadView,
) (LocalProductSetupCatalog, error) {
	source.calls++
	return cloneSetupCatalog(source.catalog), source.err
}

func TestLocalProductSetupRejectsChangedCatalogWithoutRebindingSession(
	t *testing.T,
) {
	service, _, _, catalog := newSetupFixtureService(t)
	source := &setupFixtureCatalogSource{catalog: catalog}
	service.catalogSource = source
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{Source: BuilderSourceBlank},
	)
	if err != nil {
		t.Fatal(err)
	}
	changed := cloneSetupCatalog(catalog)
	changed.CatalogDigest = setupDigest("changed-catalog")
	changed.RuntimeProfiles[0].ModelID = "model-b"
	changed.RuntimeProfiles[1].ModelID = "model-b"
	observations := changed.RuntimeDiscovery.Observations()
	observations[0].ModelIDs = []string{"model-b"}
	changed.RuntimeDiscovery, err = loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{setupFixtureProbe{
			observations: observations,
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	source.catalog = changed

	_, err = service.AnswerBuilder(
		context.Background(),
		BuilderAnswerCommand{
			DraftID:          session.DraftID,
			ExpectedRevision: session.Revision,
			CatalogDigest:    session.CatalogDigest,
			ViewVersion:      session.ViewVersion,
			QuestionID:       session.Question.ID,
			Answer:           "Must remain bound to model-a",
		},
	)
	if !errors.Is(err, ErrBuilderConflict) {
		t.Fatalf("AnswerBuilder(changed catalog) error = %v", err)
	}
	stored := service.sessions[session.DraftID]
	if stored == nil ||
		stored.view.Revision != session.Revision ||
		stored.catalog.RuntimeProfiles[0].ModelID != "model-a" ||
		source.calls != 2 {
		t.Fatalf("stored session=%#v source_calls=%d", stored, source.calls)
	}
}

func TestLocalProductSetupBuilderBlankFlowRequiresExplicitConfirmation(
	t *testing.T,
) {
	service, database, store, catalog := newSetupFixtureService(t)

	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{Source: BuilderSourceBlank},
	)
	if err != nil {
		t.Fatalf("StartBuilder() error = %v", err)
	}
	if session.DraftID == "" ||
		session.Revision != 1 ||
		session.Question.ID != "team_name" ||
		session.CanConfirm {
		t.Fatalf("initial session = %#v", session)
	}

	answers := []struct {
		questionID string
		answer     string
	}{
		{"team_name", "Local Review Team"},
		{"purpose", "Review one bounded change"},
		{"main_role", "role-main"},
		{"subagent_role", "role-sub"},
	}
	for _, answer := range answers {
		session, err = service.AnswerBuilder(
			context.Background(),
			BuilderAnswerCommand{
				DraftID:          session.DraftID,
				ExpectedRevision: session.Revision,
				CatalogDigest:    catalog.CatalogDigest,
				ViewVersion:      session.ViewVersion,
				QuestionID:       answer.questionID,
				Answer:           answer.answer,
			},
		)
		if err != nil {
			t.Fatalf("AnswerBuilder(%s) error = %v", answer.questionID, err)
		}
	}
	if session.Question.ID != "" ||
		!session.CanConfirm ||
		session.BindingDigest == "" ||
		len(session.Preview.Roles) != 2 ||
		session.Preview.Roles[0].Runtime.ExecutableVersion != "0.82.1" ||
		session.Preview.Roles[0].Runtime.ModelID != "model-a" ||
		session.Preview.Roles[0].AuthMode != "native_auth" ||
		session.Preview.MaximumBudgetCredits != 100 ||
		len(session.Preview.Roles[0].Skills) != 1 ||
		session.Preview.Roles[0].Skills[0].Revision != 2 ||
		session.Preview.Roles[0].Skills[0].Digest == "" {
		t.Fatalf("proposed session = %#v", session)
	}

	before, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if containsSetupExecutionFact(before) {
		t.Fatalf("pre-confirm execution fact = %#v", before)
	}
	if _, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID:          session.DraftID,
			ExpectedRevision: session.Revision,
			CatalogDigest:    catalog.CatalogDigest,
			ViewVersion:      session.ViewVersion,
			BindingDigest:    session.BindingDigest,
			DefinitionID:     "team-review",
			Scope:            "reusable",
			Confirm:          false,
		},
	); !errors.Is(err, ErrBuilderConfirmationRequired) {
		t.Fatalf("ConfirmBuilder(false) error = %v", err)
	}

	confirmed, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID:          session.DraftID,
			ExpectedRevision: session.Revision,
			CatalogDigest:    catalog.CatalogDigest,
			ViewVersion:      session.ViewVersion,
			BindingDigest:    session.BindingDigest,
			DefinitionID:     "team-review",
			Scope:            "reusable",
			Confirm:          true,
		},
	)
	if err != nil {
		t.Fatalf("ConfirmBuilder() error = %v", err)
	}
	if confirmed.TeamDefinitionID != "team-review" ||
		confirmed.Status != "active" ||
		confirmed.TeamInstanceCreated ||
		confirmed.RunCreated {
		t.Fatalf("confirmation = %#v", confirmed)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, ok := readModel.GlobalReadView().TeamDefinition("team-review")
	if !ok || saved.Status != "active" {
		t.Fatalf("saved = %#v, ok = %v", saved, ok)
	}
	after, err := store.ReadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if containsSetupExecutionFact(after) {
		t.Fatalf("confirmation created execution fact = %#v", after)
	}
	second, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source:        BuilderSourceTemplate,
			SourceID:      "template-review",
			SourceVersion: 3,
			SourceDigest:  setupDigest("template"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID:          second.DraftID,
			ExpectedRevision: second.Revision,
			CatalogDigest:    second.CatalogDigest,
			ViewVersion:      second.ViewVersion,
			BindingDigest:    second.BindingDigest,
			DefinitionID:     "team-review",
			Scope:            "reusable",
			Confirm:          true,
		},
	); !errors.Is(err, ErrBuilderConflict) {
		t.Fatalf("duplicate TeamDefinition confirmation error = %v", err)
	}
}

func TestLocalProductSetupConnectCodexDelegatesOnlyWhenDisconnected(
	t *testing.T,
) {
	service, _, _, _ := newSetupFixtureService(t)
	connector := &setupFixtureNativeAuthConnector{}
	service.nativeAuthConnector = connector

	connected, err := service.ConnectCodex(context.Background())
	if err != nil {
		t.Fatalf("ConnectCodex() connected error = %v", err)
	}
	if connected.ProviderID != "codex" ||
		connected.AuthMode != "native_auth" ||
		connected.Status != "already_connected" ||
		connector.starts != 0 {
		t.Fatalf("connected result = %#v, starts = %d", connected, connector.starts)
	}

	service.nativeAuth = setupFixtureNativeAuth{result: NativeAuthObservation{
		Status:   "not_logged_in",
		AuthMode: "native_auth",
		Reason:   "not_logged_in",
	}}
	started, err := service.ConnectCodex(context.Background())
	if err != nil {
		t.Fatalf("ConnectCodex() disconnected error = %v", err)
	}
	if started.ProviderID != "codex" ||
		started.AuthMode != "native_auth" ||
		started.Status != "started" ||
		connector.starts != 1 {
		t.Fatalf("started result = %#v, starts = %d", started, connector.starts)
	}
	service.nativeAuth = setupFixtureNativeAuth{result: NativeAuthObservation{
		Status:   "unsupported",
		AuthMode: "native_auth",
		Reason:   "unknown_output",
	}}
	if _, err := service.ConnectCodex(context.Background()); !errors.Is(
		err,
		ErrNativeAuthConnectUnavailable,
	) {
		t.Fatalf("ConnectCodex() unsupported error = %v", err)
	}
	if connector.starts != 1 {
		t.Fatalf("unsupported status launched login; starts = %d", connector.starts)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !connector.closed {
		t.Fatal("Close() did not join the native auth connector")
	}
}

func TestLocalProductSetupBuilderStaleEditTemplateAndSavedSources(t *testing.T) {
	service, _, _, catalog := newSetupFixtureService(t)
	template, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source:        BuilderSourceTemplate,
			SourceID:      "template-review",
			SourceVersion: 3,
			SourceDigest:  setupDigest("template"),
		},
	)
	if err != nil {
		t.Fatalf("StartBuilder(template) error = %v", err)
	}
	if !template.CanConfirm || template.Source != BuilderSourceTemplate {
		t.Fatalf("template session = %#v", template)
	}
	if _, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID:          template.DraftID,
			ExpectedRevision: template.Revision - 1,
			CatalogDigest:    catalog.CatalogDigest,
			ViewVersion:      template.ViewVersion,
			Field:            "purpose",
			Value:            "Updated bounded purpose",
		},
	); !errors.Is(err, ErrBuilderConflict) {
		t.Fatalf("stale EditBuilder() error = %v", err)
	}
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID:          template.DraftID,
			ExpectedRevision: template.Revision,
			CatalogDigest:    catalog.CatalogDigest,
			ViewVersion:      template.ViewVersion,
			Field:            "purpose",
			Value:            "Updated bounded purpose",
		},
	)
	if err != nil || edited.Revision != template.Revision+1 {
		t.Fatalf("EditBuilder() = %#v, %v", edited, err)
	}
	confirmed, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID:          edited.DraftID,
			ExpectedRevision: edited.Revision,
			CatalogDigest:    catalog.CatalogDigest,
			ViewVersion:      edited.ViewVersion,
			BindingDigest:    edited.BindingDigest,
			DefinitionID:     "team-template",
			Scope:            "reusable",
			Confirm:          true,
		},
	)
	if err != nil {
		t.Fatalf("ConfirmBuilder(template) error = %v", err)
	}
	saved, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source:        BuilderSourceSavedTeam,
			SourceID:      confirmed.TeamDefinitionID,
			SourceVersion: confirmed.TeamDefinitionVersion,
			SourceDigest:  confirmed.TeamDefinitionDigest,
		},
	)
	if err != nil {
		t.Fatalf("StartBuilder(saved) error = %v", err)
	}
	if saved.Source != BuilderSourceSavedTeam ||
		!saved.CanConfirm ||
		saved.Preview.Name != edited.Preview.Name {
		t.Fatalf("saved session = %#v", saved)
	}
}

func TestLocalProductSetupSavedSourceRequiresExactBinding(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	template, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source:        BuilderSourceTemplate,
			SourceID:      "template-review",
			SourceVersion: 3,
			SourceDigest:  setupDigest("template"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID:          template.DraftID,
			ExpectedRevision: template.Revision,
			CatalogDigest:    template.CatalogDigest,
			ViewVersion:      template.ViewVersion,
			BindingDigest:    template.BindingDigest,
			DefinitionID:     "team-exact-binding",
			Scope:            "reusable",
			Confirm:          true,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	service.catalog.RoleOptions[0].PermissionIDs = []string{}
	_, err = service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source:        BuilderSourceSavedTeam,
			SourceID:      confirmed.TeamDefinitionID,
			SourceVersion: confirmed.TeamDefinitionVersion,
			SourceDigest:  confirmed.TeamDefinitionDigest,
		},
	)
	if !errors.Is(err, ErrBuilderIncompatible) {
		t.Fatalf("StartBuilder(saved exact-binding drift) error = %v", err)
	}
}

func TestLocalProductSetupBuilderRejectsStaleGlobalView(t *testing.T) {
	service, _, store, catalog := newSetupFixtureService(t)
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{Source: BuilderSourceBlank},
	)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.CommitCredentialMetadata(
		context.Background(),
		credentials.MetadataCommand{
			CommandID:           "configure-after-builder-start",
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-new-view",
			ExpectedRevision:    0,
			OccurredAt:          time.Unix(2200, 0).UTC(),
			Status:              credentials.CredentialConfigured,
			Reason:              credentials.VerificationReasonNone,
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AnswerBuilder(
		context.Background(),
		BuilderAnswerCommand{
			DraftID:          session.DraftID,
			ExpectedRevision: session.Revision,
			CatalogDigest:    catalog.CatalogDigest,
			ViewVersion:      session.ViewVersion,
			QuestionID:       session.Question.ID,
			Answer:           "Stale Candidate",
		},
	); !errors.Is(err, ErrBuilderConflict) {
		t.Fatalf("stale view AnswerBuilder() error = %v", err)
	}
}

func TestLocalProductSetupCatalogFailsClosedOnBindingAndReferenceDrift(
	t *testing.T,
) {
	_, _, _, catalog := newSetupFixtureService(t)

	modelDrift := cloneSetupCatalog(catalog)
	modelDrift.RuntimeProfiles[0].ModelID = "model-not-observed"
	if !errors.Is(
		validateSetupCatalog(modelDrift),
		ErrInvalidLocalProductSetup,
	) {
		t.Fatal("catalog accepted a Runtime model absent from observation")
	}

	duplicatePermission := cloneSetupCatalog(catalog)
	duplicatePermission.RoleOptions[0].PermissionIDs = []string{
		"permission-read",
		"permission-read",
	}
	if !errors.Is(
		validateSetupCatalog(duplicatePermission),
		ErrInvalidLocalProductSetup,
	) {
		t.Fatal("catalog accepted duplicate permission references")
	}

	incompatibleSkill := cloneSetupCatalog(catalog)
	incompatibleSkill.SkillRevisions[0].CompatibleRuntimes = []string{"other"}
	if !errors.Is(
		validateSetupCatalog(incompatibleSkill),
		ErrInvalidLocalProductSetup,
	) {
		t.Fatal("catalog accepted a Runtime-incompatible Skill revision")
	}
}

func TestLocalProductSetupSnapshotReportsTruthfulProviderAndRuntimeModes(
	t *testing.T,
) {
	service, _, _, _ := newSetupFixtureService(t)
	snapshot, err := service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatalf("SetupSnapshot() error = %v", err)
	}
	if snapshot.Codex.Status != "available" ||
		snapshot.Codex.AuthMode != "native_auth" ||
		snapshot.MiniMax.Status != "configured" ||
		snapshot.MiniMax.AuthMode != "brokered" ||
		snapshot.MiniMax.CredentialReference != "credential-ref-1" ||
		snapshot.MiniMax.Revision != 1 ||
		len(snapshot.Runtimes) != 1 ||
		snapshot.Runtimes[0].ExecutableVersion != "0.82.1" ||
		len(snapshot.Runtimes[0].ModelIDs) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func newSetupFixtureService(
	t *testing.T,
) (*LocalProductSetupService, *sql.DB, *journal.Store, LocalProductSetupCatalog) {
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
	store := journal.NewStore(database)
	writer, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{setupFixtureProbe{
			observations: []loomruntime.RuntimeObservation{{
				Instance: loomruntime.RuntimeInstance{
					ID:                   "runtime-pi",
					DeviceID:             "device-local",
					AdapterType:          "pi",
					DisplayName:          "Pi Coding Agent",
					ExecutableVersion:    "0.82.1",
					Status:               loomruntime.RuntimeOnline,
					ObservedCapabilities: []string{"rpc"},
					Capacity:             2,
				},
				ModelIDs: []string{"model-a"},
			}},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	definitions := []agents.AgentDefinition{
		{
			ID:       "agent-main",
			Version:  1,
			Scope:    agents.ScopeReusable,
			Name:     "Coordinator",
			RoleSpec: "Coordinate bounded work",
			Status:   agents.DefinitionActive,
		},
		{
			ID:       "agent-sub",
			Version:  1,
			Scope:    agents.ScopeReusable,
			Name:     "Reviewer",
			RoleSpec: "Review bounded work",
			Status:   agents.DefinitionActive,
		},
	}
	mainProfile := loomruntime.RuntimeProfile{
		ID:                   "profile-main",
		AdapterType:          "pi",
		ProviderID:           "local",
		ModelID:              "model-a",
		AuthMode:             loomruntime.AuthNative,
		RequiredCapabilities: []string{"rpc"},
		Timeout:              time.Minute,
	}
	subProfile := mainProfile
	subProfile.ID = "profile-sub"
	catalog := LocalProductSetupCatalog{
		CatalogDigest:    setupDigest("catalog"),
		AgentDefinitions: definitions,
		RuntimeProfiles:  []loomruntime.RuntimeProfile{mainProfile, subProfile},
		RuntimeDiscovery: discovery,
		SkillRevisions: []SetupSkillRevision{{
			ID:                 "skill-review",
			Revision:           2,
			Digest:             setupDigest("skill"),
			SourceScope:        "reusable",
			Risk:               "low",
			CompatibleRuntimes: []string{"pi"},
		}},
		Permissions: []string{"permission-read"},
		Resources: []SetupResourcePointer{{
			ID:   "resource-repo",
			Kind: "repository",
		}},
		RoleOptions: []SetupRoleOption{
			{
				ID:                "role-main",
				Kind:              "main",
				AgentDefinitionID: "agent-main",
				RuntimeProfileID:  "profile-main",
				RuntimeInstanceID: "runtime-pi",
				SkillRevisionIDs:  []string{"skill-review@2"},
				PermissionIDs:     []string{"permission-read"},
				ResourceIDs:       []string{"resource-repo"},
				Responsibility:    "Coordinate",
			},
			{
				ID:                "role-sub",
				Kind:              "subagent",
				AgentDefinitionID: "agent-sub",
				RuntimeProfileID:  "profile-sub",
				RuntimeInstanceID: "runtime-pi",
				SkillRevisionIDs:  []string{},
				PermissionIDs:     []string{"permission-read"},
				ResourceIDs:       []string{"resource-repo"},
				Responsibility:    "Review",
			},
		},
		Templates: []SetupTeamTemplate{{
			ID:                   "template-review",
			Version:              3,
			Digest:               setupDigest("template"),
			Name:                 "Template Review Team",
			Purpose:              "Review one bounded change",
			MainRoleID:           "role-main",
			SubagentRoleIDs:      []string{"role-sub"},
			RequestedConcurrency: 1,
			MaximumBudgetCredits: 100,
		}},
		BudgetCeiling:      100,
		ConcurrencyCeiling: 2,
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	service, err := NewLocalProductSetupService(LocalProductSetupConfig{
		Journal:    store,
		Projection: readModel,
		Writer:     writer,
		Catalog:    catalog,
		Identity:   &setupFixtureIdentity{},
		Now: func() time.Time {
			return time.Unix(3000, 0).UTC()
		},
		NativeAuth: setupFixtureNativeAuth{result: NativeAuthObservation{
			Status:   "available",
			AuthMode: "native_auth",
		}},
		Credentials: setupFixtureBroker{status: credentials.MetadataResult{
			ProviderID:          "minimax",
			CredentialReference: "credential-ref-1",
			Revision:            1,
			Status:              credentials.CredentialConfigured,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, database, store, catalog
}

func containsSetupExecutionFact(events []journal.Event) bool {
	for _, event := range events {
		switch event.Type {
		case "TeamInstanceCreated", "AgentInstanceCreated", "WorkItemCreated",
			"RunCreated", "AgentGrantIssued", "EvidenceSubmitted",
			"TeamExecutionPlanned", "TeamReadySetDispatched":
			return true
		}
	}
	return false
}

func setupDigest(seed string) string {
	const hexadecimal = "0123456789abcdef"
	output := make([]byte, 64)
	for index := range output {
		output[index] = hexadecimal[(index+len(seed))%len(hexadecimal)]
	}
	return string(output)
}

func setupDecimal(value int) string {
	if value == 0 {
		return "0"
	}
	var output [20]byte
	index := len(output)
	for value > 0 {
		index--
		output[index] = byte('0' + value%10)
		value /= 10
	}
	return string(output[index:])
}

func TestLocalProductSetupErrorsNeverContainSecretMaterial(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	_, err := service.ConfigureCredential(
		context.Background(),
		CredentialSetupCommand{
			ProviderID: "unsupported",
			Secret:     []byte{0xde, 0xad, 0xbe, 0xef},
		},
	)
	if err == nil ||
		strings.Contains(err.Error(), string([]byte{0xde, 0xad, 0xbe, 0xef})) {
		t.Fatalf("unsafe credential setup error = %v", err)
	}
}

func TestLocalProductSetupRejectsNonOpaqueCredentialReferenceBeforeMutation(
	t *testing.T,
) {
	service, _, _, _ := newSetupFixtureService(t)
	called := false
	service.credentialMutator = setupFixtureMutator{called: &called}

	_, err := service.VerifyCredential(
		context.Background(),
		CredentialSetupCommand{
			ProviderID:          "minimax",
			CredentialReference: "not-an-opaque-reference",
			ExpectedRevision:    1,
		},
	)
	if !errors.Is(err, ErrCredentialSetupUnavailable) {
		t.Fatalf("VerifyCredential(non-opaque) error = %v", err)
	}
	if called {
		t.Fatal("non-opaque credential reference reached mutator")
	}
}

func TestLocalProductSetupVerifyCredentialBindsStrictOperationIdentity(
	t *testing.T,
) {
	service, _, _, _ := newSetupFixtureService(t)
	mutator := &setupCapturingCredentialMutator{}
	service.credentialMutator = mutator
	operationID := "11111111-1111-4111-8111-111111111111"

	result, err := service.VerifyCredential(
		context.Background(),
		CredentialSetupCommand{
			ProviderID: "minimax", CredentialReference: "credential-ref-1",
			ExpectedRevision: 1, OperationID: operationID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != 2 ||
		mutator.command.CommandID != "verify-credential-"+operationID {
		t.Fatalf("result=%#v command=%#v", result, mutator.command)
	}

	for _, invalid := range []string{
		"", "11111111-1111-4111-8111-11111111111A",
		"11111111-1111-5111-8111-111111111111",
		"11111111-1111-4111-7111-111111111111",
	} {
		mutator.command = credentials.CredentialCommand{}
		_, err := service.VerifyCredential(
			context.Background(),
			CredentialSetupCommand{
				ProviderID: "minimax", CredentialReference: "credential-ref-1",
				ExpectedRevision: 1, OperationID: invalid,
			},
		)
		if !errors.Is(err, ErrCredentialSetupUnavailable) ||
			mutator.command.CommandID != "" {
			t.Fatalf("operation_id=%q error=%v command=%#v", invalid, err, mutator.command)
		}
	}
}

func TestLocalProductSetupNonVerifyCredentialRejectsOperationIdentity(
	t *testing.T,
) {
	service, _, _, _ := newSetupFixtureService(t)
	called := false
	service.credentialMutator = setupFixtureMutator{called: &called}
	_, err := service.ReplaceCredential(
		context.Background(),
		CredentialSetupCommand{
			ProviderID: "minimax", CredentialReference: "credential-ref-1",
			ExpectedRevision: 1,
			OperationID:      "11111111-1111-4111-8111-111111111111",
			Secret:           []byte("replacement"),
		},
	)
	if !errors.Is(err, ErrCredentialSetupUnavailable) || called {
		t.Fatalf("ReplaceCredential() error=%v called=%v", err, called)
	}
}

var _ = teams.TeamDefinition{}
