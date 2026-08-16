package app

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
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
	status   credentials.MetadataResult
	statuses map[string]credentials.MetadataResult
}

func (broker setupFixtureBroker) CredentialStatus(
	_ context.Context,
	providerID string,
) (credentials.MetadataResult, error) {
	if status, ok := broker.statuses[providerID]; ok {
		return status, nil
	}
	if broker.status.ProviderID == providerID {
		return broker.status, nil
	}
	return credentials.MetadataResult{}, credentials.ErrCredentialNotFound
}

type setupFixtureMutator struct {
	called *bool
}

type setupCapturingCredentialMutator struct {
	command credentials.CredentialCommand
	err     error
}

func (mutator *setupCapturingCredentialMutator) Configure(
	_ context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	mutator.command = command
	if mutator.err != nil {
		return credentials.MetadataResult{}, mutator.err
	}
	return credentials.MetadataResult{
		ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		Revision:            1, Status: credentials.CredentialConfigured,
	}, nil
}

func (mutator *setupCapturingCredentialMutator) Verify(
	_ context.Context,
	command credentials.CredentialCommand,
) (credentials.MetadataResult, error) {
	mutator.command = command
	return credentials.MetadataResult{
		ProviderID: command.ProviderID, ProviderAccountID: command.ProviderAccountID,
		CredentialReference: command.CredentialReference,
		Revision:            command.ExpectedRevision + 1, Status: credentials.CredentialVerified,
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
	for index := range changed.RuntimeProfiles {
		changed.RuntimeProfiles[index].ModelID = "model-b"
	}
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
		session.Preview.Roles[0].AuthMode != "brokered" ||
		session.Preview.Roles[0].HarnessAdapter != "pi" ||
		session.Preview.Roles[0].ProviderID != "openai" ||
		session.Preview.Roles[0].ProviderAccountID != "openai.primary" ||
		session.Preview.Roles[0].CredentialRevision != 3 ||
		session.Preview.Roles[0].ReasoningEffort != "high" ||
		session.Preview.Roles[0].TimeoutMilliseconds != 60_000 ||
		!session.Preview.Roles[0].BudgetAvailable ||
		session.Preview.Roles[0].BudgetUnits != 80 ||
		!reflect.DeepEqual(
			session.Preview.Roles[0].RequiredCapabilities,
			[]string{loomruntime.CapabilityReasoningEffort, "rpc"},
		) ||
		session.Preview.Roles[1].ProviderID != "anthropic" ||
		session.Preview.Roles[1].ProviderAccountID != "anthropic.review" ||
		session.Preview.Roles[1].CredentialRevision != 7 ||
		session.Preview.Roles[1].ReasoningEffort != "low" ||
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
		len(snapshot.Runtimes[0].ModelIDs) != 1 ||
		len(snapshot.RoleOptions) != 3 ||
		snapshot.RoleOptions[0].ProviderID != "openai" ||
		snapshot.RoleOptions[0].ProviderAccountID != "openai.primary" ||
		snapshot.RoleOptions[0].CredentialRevision != 3 ||
		snapshot.RoleOptions[0].ReasoningEffort != "high" ||
		snapshot.RoleOptions[1].ProviderID != "anthropic" ||
		snapshot.RoleOptions[1].ProviderAccountID != "anthropic.review" ||
		snapshot.RoleOptions[1].ReasoningEffort != "low" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if len(snapshot.Providers) < 20 {
		t.Fatalf("provider directory entries = %d", len(snapshot.Providers))
	}
	providers := make(map[string]ProviderDirectoryEntry, len(snapshot.Providers))
	for _, entry := range snapshot.Providers {
		providers[entry.ProviderID] = entry
	}
	if providers["minimax"].Status != "configured" ||
		providers["minimax"].CredentialReference != "credential-ref-1" ||
		providers["openai"].Status != "available" ||
		providers["openai"].AuthMode != "native_auth" ||
		providers["deepseek"].Status != "unconfigured" {
		t.Fatalf("provider directory = %#v", providers)
	}
}

func TestPhase2DBuilderEditsOneAgentExecutionProfileWithoutOverwritingPeers(
	t *testing.T,
) {
	service, _, _, catalog := newSetupFixtureService(t)
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil || len(session.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder() = %#v, %v", session, err)
	}
	beforeDigest := session.BindingDigest
	beforeSub := session.Preview.Roles[1]
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
			Field: "main_role", Value: "role-main-backup",
		},
	)
	if err != nil || edited.BindingDigest == beforeDigest ||
		len(edited.Preview.Roles) != 2 ||
		edited.Preview.Roles[0].ProviderID != "deepseek" ||
		edited.Preview.Roles[0].ProviderAccountID != "deepseek.backup" ||
		edited.Preview.Roles[0].CredentialRevision != 9 ||
		edited.Preview.Roles[0].ReasoningEffort != "medium" ||
		edited.Preview.Roles[1].RuntimeProfileID != beforeSub.RuntimeProfileID ||
		edited.Preview.Roles[1].ProviderAccountID != beforeSub.ProviderAccountID ||
		edited.Preview.Roles[1].CredentialRevision != beforeSub.CredentialRevision ||
		edited.Preview.Roles[1].ReasoningEffort != beforeSub.ReasoningEffort {
		t.Fatalf("independent Agent edit = %#v, %v", edited, err)
	}
}

func TestPhase2DBuilderSwitchesRouteWithoutChangingAgentIdentity(
	t *testing.T,
) {
	service, database, _, catalog := newSetupFixtureService(t)
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil || len(session.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder() = %#v, %v", session, err)
	}
	mainBefore := session.Preview.Roles[0]
	peerBefore := session.Preview.Roles[1]
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
			Field: "main_provider_account_route", Value: "role-main-backup",
		},
	)
	if err != nil || len(edited.Preview.Roles) != 2 {
		t.Fatalf("EditBuilder(provider account route) = %#v, %v", edited, err)
	}
	mainAfter := edited.Preview.Roles[0]
	if mainAfter.AgentDefinitionID != mainBefore.AgentDefinitionID ||
		mainAfter.AgentVersion != mainBefore.AgentVersion ||
		mainAfter.AgentScope != mainBefore.AgentScope ||
		mainAfter.DisplayName != mainBefore.DisplayName ||
		mainAfter.Responsibility != mainBefore.Responsibility ||
		!reflect.DeepEqual(mainAfter.Skills, mainBefore.Skills) ||
		!reflect.DeepEqual(mainAfter.PermissionIDs, mainBefore.PermissionIDs) ||
		!reflect.DeepEqual(mainAfter.ResourceIDs, mainBefore.ResourceIDs) ||
		mainAfter.ProviderID != "deepseek" ||
		mainAfter.ProviderAccountID != "deepseek.backup" ||
		mainAfter.CredentialRevision != 9 ||
		mainAfter.RuntimeProfileID == mainBefore.RuntimeProfileID ||
		mainAfter.RuntimeProfileID == "profile-main-backup" ||
		mainAfter.Runtime.RuntimeInstanceID != "runtime-pi" ||
		mainAfter.HarnessAdapter != mainBefore.HarnessAdapter ||
		mainAfter.ModelID != mainBefore.ModelID ||
		mainAfter.ReasoningEffort != mainBefore.ReasoningEffort ||
		mainAfter.TimeoutMilliseconds != mainBefore.TimeoutMilliseconds ||
		mainAfter.BudgetAvailable != mainBefore.BudgetAvailable ||
		mainAfter.BudgetUnits != mainBefore.BudgetUnits ||
		!reflect.DeepEqual(
			mainAfter.RequiredCapabilities,
			mainBefore.RequiredCapabilities,
		) ||
		!reflect.DeepEqual(edited.Preview.Roles[1], peerBefore) {
		t.Fatalf("stable Agent route edit = %#v; peer = %#v", mainAfter, edited.Preview.Roles[1])
	}
	internalSession := service.sessions[edited.DraftID]
	scoped := service.withSessionCatalog(internalSession)
	profile, ok := scoped.setupRuntimeProfile(mainAfter.RuntimeProfileID)
	if !ok || profile.CredentialReference != "credential-ref-deepseek-backup" ||
		profile.CredentialRevision != 9 || profile.ProviderAccountID != "deepseek.backup" {
		t.Fatalf("route Profile lost exact credential identity: %#v, %t", profile, ok)
	}
	if profile.AdapterType != mainBefore.HarnessAdapter ||
		profile.ModelID != mainBefore.ModelID ||
		profile.ReasoningEffort != mainBefore.ReasoningEffort ||
		profile.Timeout != time.Duration(mainBefore.TimeoutMilliseconds)*time.Millisecond ||
		profile.Budget == nil || *profile.Budget != mainBefore.BudgetUnits ||
		!reflect.DeepEqual(profile.RequiredCapabilities, mainBefore.RequiredCapabilities) {
		t.Fatalf("account route changed an unrelated execution dimension: %#v", profile)
	}
	beforeRejectedRevision := internalSession.view.Revision
	beforeRejectedDigest := internalSession.view.BindingDigest
	_, err = service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: edited.ViewVersion,
			Field: "main_harness_route", Value: "role-sub",
		},
	)
	if !errors.Is(err, ErrInvalidLocalProductSetup) ||
		internalSession.view.Revision != beforeRejectedRevision ||
		internalSession.view.BindingDigest != beforeRejectedDigest {
		t.Fatalf("cross-Agent route edit mutated Draft: revision=%d digest=%q err=%v",
			internalSession.view.Revision, internalSession.view.BindingDigest, err)
	}
	confirmation, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: edited.CatalogDigest, ViewVersion: edited.ViewVersion,
			BindingDigest: edited.BindingDigest, DefinitionID: "team-route-switch",
			Scope: "reusable", Confirm: true,
		},
	)
	if err != nil || confirmation.TeamDefinitionID != "team-route-switch" {
		t.Fatalf("ConfirmBuilder(route switch) = %#v, %v", confirmation, err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, ok := readModel.GlobalReadView().TeamDefinition("team-route-switch")
	if !ok || len(record.Configuration.RoleBindings) != 2 {
		t.Fatalf("saved route-switched Team = %#v, %t", record, ok)
	}
	mainBinding := record.Configuration.RoleBindings[0]
	if mainBinding.AgentDefinitionID != mainBefore.AgentDefinitionID ||
		!mainBinding.ExecutionProfileAvailable ||
		mainBinding.ExecutionProfile.ProviderID != "deepseek" ||
		mainBinding.ExecutionProfile.ProviderAccountID != "deepseek.backup" ||
		mainBinding.ExecutionProfile.CredentialReference !=
			"credential-ref-deepseek-backup" ||
		mainBinding.ExecutionProfile.CredentialRevision != 9 ||
		mainBinding.ExecutionProfile.HarnessAdapter != mainBefore.HarnessAdapter ||
		mainBinding.ExecutionProfile.ModelID != mainBefore.ModelID ||
		mainBinding.ExecutionProfile.ReasoningEffort != mainBefore.ReasoningEffort ||
		mainBinding.ExecutionProfile.Timeout !=
			time.Duration(mainBefore.TimeoutMilliseconds)*time.Millisecond ||
		mainBinding.ExecutionProfile.Budget == nil ||
		*mainBinding.ExecutionProfile.Budget != mainBefore.BudgetUnits {
		t.Fatalf("saved route binding = %#v", mainBinding)
	}
}

func TestPhase2DBuilderSwitchesHarnessWithoutChangingProviderOrLimits(
	t *testing.T,
) {
	service, _, _, catalog := newSetupFixtureService(t)
	observations := catalog.RuntimeDiscovery.Observations()
	nativeObservation := observations[0]
	nativeObservation.Instance.ID = "runtime-loom-native"
	nativeObservation.Instance.AdapterType = "loom-native"
	nativeObservation.Instance.DisplayName = "Loom Native"
	nativeObservation.ModelIDs = []string{"model-a"}
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{setupFixtureProbe{
			observations: append(observations, nativeObservation),
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	catalog.RuntimeDiscovery = discovery
	var sourceProfile loomruntime.RuntimeProfile
	for _, profile := range catalog.RuntimeProfiles {
		if profile.ID == "profile-main" {
			sourceProfile = profile
			break
		}
	}
	sourceProfile.ID = "profile-main-native-source"
	sourceProfile.AdapterType = "loom-native"
	catalog.RuntimeProfiles = append(catalog.RuntimeProfiles, sourceProfile)
	for index := range catalog.SkillRevisions {
		if catalog.SkillRevisions[index].ID == "skill-review" {
			catalog.SkillRevisions[index].CompatibleRuntimes = append(
				catalog.SkillRevisions[index].CompatibleRuntimes,
				"loom-native",
			)
		}
	}
	catalog.RoleOptions = append(catalog.RoleOptions, SetupRoleOption{
		ID:                "role-main-native-source",
		Kind:              "main",
		AgentDefinitionID: "agent-main",
		RuntimeProfileID:  sourceProfile.ID,
		RuntimeInstanceID: nativeObservation.Instance.ID,
		SkillRevisionIDs:  []string{"skill-review@2"},
		PermissionIDs:     []string{"permission-read"},
		ResourceIDs:       []string{"resource-repo"},
		Responsibility:    "Coordinate through Loom Native",
	})
	if err := validateSetupCatalog(catalog); err != nil {
		t.Fatal(err)
	}
	service.catalog = cloneSetupCatalog(catalog)
	service.domainCatalog, err = buildSetupDomainCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}

	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil || len(session.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder() = %#v, %v", session, err)
	}
	before := session.Preview.Roles[0]
	internalSession := service.sessions[session.DraftID]
	for _, incompatible := range []struct {
		field string
		value string
	}{
		{"main_provider_account_route", "role-main-native-source"},
		{"main_harness_route", "role-main-backup"},
	} {
		beforeRevision := internalSession.view.Revision
		beforeDigest := internalSession.view.BindingDigest
		_, editErr := service.EditBuilder(
			context.Background(),
			BuilderEditCommand{
				DraftID: session.DraftID, ExpectedRevision: session.Revision,
				CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
				Field: incompatible.field, Value: incompatible.value,
			},
		)
		if !errors.Is(editErr, ErrBuilderIncompatible) ||
			internalSession.view.Revision != beforeRevision ||
			internalSession.view.BindingDigest != beforeDigest {
			t.Fatalf("incompatible route %s/%s mutated Draft: %v",
				incompatible.field, incompatible.value, editErr)
		}
	}
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
			Field: "main_harness_route", Value: "role-main-native-source",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	after := edited.Preview.Roles[0]
	if after.AgentDefinitionID != before.AgentDefinitionID ||
		after.HarnessAdapter != "loom-native" ||
		after.Runtime.RuntimeInstanceID != "runtime-loom-native" ||
		after.RuntimeProfileID == before.RuntimeProfileID ||
		after.RuntimeProfileID == sourceProfile.ID ||
		after.ProviderID != before.ProviderID ||
		after.ProviderAccountID != before.ProviderAccountID ||
		after.CredentialRevision != before.CredentialRevision ||
		after.ModelID != before.ModelID ||
		after.ReasoningEffort != before.ReasoningEffort ||
		after.TimeoutMilliseconds != before.TimeoutMilliseconds ||
		after.BudgetAvailable != before.BudgetAvailable ||
		after.BudgetUnits != before.BudgetUnits ||
		!reflect.DeepEqual(after.RequiredCapabilities, before.RequiredCapabilities) {
		t.Fatalf("independent Harness route edit = %#v; before = %#v", after, before)
	}
}

func TestPhase2DBuilderPersistsCustomAgentExecutionProfile(t *testing.T) {
	service, database, store, catalog := newSetupFixtureService(t)
	observations := catalog.RuntimeDiscovery.Observations()
	observations[0].ModelIDs = []string{"model-a", "model-b"}
	discovery, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{setupFixtureProbe{observations: observations}},
	)
	if err != nil {
		t.Fatal(err)
	}
	catalog.RuntimeDiscovery = discovery
	service.catalog = cloneSetupCatalog(catalog)
	service.domainCatalog, err = buildSetupDomainCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil || len(session.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder() = %#v, %v", session, err)
	}
	originalMain := session.Preview.Roles[0]
	originalPeer := session.Preview.Roles[1]
	edited := session
	for _, change := range []struct {
		field string
		value string
	}{
		{"main_model", "model-b"},
		{"main_reasoning_effort", "medium"},
		{"main_timeout_seconds", "90"},
		{"main_budget_units", "120"},
		{"main_fallback_role", "role-main-backup"},
	} {
		edited, err = service.EditBuilder(
			context.Background(),
			BuilderEditCommand{
				DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
				CatalogDigest: catalog.CatalogDigest, ViewVersion: edited.ViewVersion,
				Field: change.field, Value: change.value,
			},
		)
		if err != nil {
			t.Fatalf("EditBuilder(%s) error = %v", change.field, err)
		}
	}
	if err != nil || edited.Preview.Roles[0].ReasoningEffort != "medium" ||
		edited.Preview.Roles[0].ModelID != "model-b" ||
		edited.Preview.Roles[0].TimeoutMilliseconds != 90_000 ||
		!edited.Preview.Roles[0].BudgetAvailable ||
		edited.Preview.Roles[0].BudgetUnits != 120 ||
		!edited.Preview.Roles[0].FallbackConfigured ||
		edited.Preview.Roles[0].FallbackProviderAccountID != "deepseek.backup" ||
		edited.Preview.Roles[0].FallbackCredentialRevision != 9 ||
		edited.Preview.Roles[0].FallbackAuthMode != string(loomruntime.AuthBrokered) ||
		edited.Preview.Roles[0].FallbackReasoningEffort != "medium" ||
		edited.Preview.Roles[0].FallbackTimeoutMilliseconds != 60_000 ||
		!edited.Preview.Roles[0].FallbackBudgetAvailable ||
		edited.Preview.Roles[0].FallbackBudgetUnits != 25 ||
		!slices.Equal(
			edited.Preview.Roles[0].FallbackRequiredCapabilities,
			[]string{"reasoning_effort", "rpc"},
		) ||
		!edited.Preview.Roles[0].FallbackApprovalRequired ||
		edited.Preview.Roles[0].RuntimeProfileID == originalMain.RuntimeProfileID ||
		edited.Preview.Roles[1].RuntimeProfileID != originalPeer.RuntimeProfileID ||
		edited.Preview.Roles[1].ReasoningEffort != originalPeer.ReasoningEffort {
		t.Fatalf("custom Agent Profile edit = %#v, %v", edited, err)
	}
	confirmation, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: edited.CatalogDigest, ViewVersion: edited.ViewVersion,
			BindingDigest: edited.BindingDigest, DefinitionID: "team-custom-profile",
			Scope: "reusable", Confirm: true,
		},
	)
	if err != nil || confirmation.TeamDefinitionID != "team-custom-profile" {
		t.Fatalf("ConfirmBuilder() = %#v, %v", confirmation, err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, ok := readModel.GlobalReadView().TeamDefinition("team-custom-profile")
	if !ok || len(record.Configuration.RoleBindings) != 2 {
		t.Fatalf("saved Team = %#v, %v", record, ok)
	}
	main := record.Configuration.RoleBindings[0]
	if !main.ExecutionProfileAvailable ||
		main.ExecutionProfile.Version != 1 ||
		main.ExecutionProfile.ID != edited.Preview.Roles[0].RuntimeProfileID ||
		main.ExecutionProfile.ProviderAccountID != "openai.primary" ||
		main.ExecutionProfile.CredentialRevision != 3 ||
		main.ExecutionProfile.ModelID != "model-b" ||
		main.ExecutionProfile.ReasoningEffort != "medium" ||
		main.ExecutionProfile.Timeout != 90*time.Second ||
		main.ExecutionProfile.Budget == nil ||
		*main.ExecutionProfile.Budget != 120 ||
		!main.FallbackRouteAvailable ||
		main.FallbackRoute.RuntimeProfileID != "profile-main-backup" ||
		main.FallbackRoute.ExecutionProfile.ProviderAccountID != "deepseek.backup" ||
		main.FallbackRoute.ExecutionProfile.CredentialRevision != 9 ||
		!main.FallbackRoute.ApprovalRequired {
		t.Fatalf("persisted custom Profile = %#v", main)
	}

	restartedProjection := projection.New(database)
	restartedWriter, err := state.NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := NewLocalProductSetupService(LocalProductSetupConfig{
		Journal: store, Projection: restartedProjection, Writer: restartedWriter,
		Catalog: catalog, Identity: &setupFixtureIdentity{},
		Now: func() time.Time { return time.Unix(3100, 0).UTC() },
		NativeAuth: setupFixtureNativeAuth{result: NativeAuthObservation{
			Status: "available", AuthMode: "native_auth",
		}},
		Credentials: setupFixtureBroker{},
	})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := restarted.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceSavedTeam, SourceID: confirmation.TeamDefinitionID,
			SourceVersion: confirmation.TeamDefinitionVersion,
			SourceDigest:  confirmation.TeamDefinitionDigest,
		},
	)
	if err != nil || len(reopened.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder(saved after restart) = %#v, %v", reopened, err)
	}
	reopenedMain := reopened.Preview.Roles[0]
	if reopenedMain.RuntimeProfileID != main.ExecutionProfile.ID ||
		reopenedMain.ProviderAccountID != main.ExecutionProfile.ProviderAccountID ||
		reopenedMain.CredentialRevision != main.ExecutionProfile.CredentialRevision ||
		reopenedMain.ModelID != main.ExecutionProfile.ModelID ||
		reopenedMain.ReasoningEffort != main.ExecutionProfile.ReasoningEffort ||
		reopenedMain.TimeoutMilliseconds != 90_000 ||
		!reopenedMain.BudgetAvailable || reopenedMain.BudgetUnits != 120 ||
		!reopenedMain.FallbackConfigured ||
		reopenedMain.FallbackRuntimeProfileID != "profile-main-backup" ||
		reopenedMain.FallbackProviderAccountID != "deepseek.backup" ||
		reopenedMain.FallbackCredentialRevision != 9 ||
		reopenedMain.FallbackAuthMode != string(loomruntime.AuthBrokered) ||
		reopenedMain.FallbackReasoningEffort != "medium" ||
		reopenedMain.FallbackTimeoutMilliseconds != 60_000 ||
		!reopenedMain.FallbackBudgetAvailable || reopenedMain.FallbackBudgetUnits != 25 ||
		!slices.Equal(
			reopenedMain.FallbackRequiredCapabilities,
			[]string{"reasoning_effort", "rpc"},
		) ||
		!reopenedMain.FallbackApprovalRequired {
		t.Fatalf("reopened custom Profile = %#v", reopenedMain)
	}
}

func TestPhase2DBuilderPersistsPerAgentParallelRouteSet(t *testing.T) {
	service, database, _, catalog := newSetupFixtureService(t)
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
			Field: "main_parallel_route_add", Value: "role-main-backup",
		},
	)
	if err != nil {
		t.Fatalf("EditBuilder(main_parallel_route_add) error = %v", err)
	}
	edited, err = service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: edited.CatalogDigest, ViewVersion: edited.ViewVersion,
			Field: "main_timeout_seconds", Value: "90",
		},
	)
	if err != nil {
		t.Fatalf("EditBuilder(main_timeout_seconds after parallel) error = %v", err)
	}
	main := edited.Preview.Roles[0]
	if main.ParallelRouteSetVersion != 1 || len(main.ParallelRoutes) != 2 ||
		main.ParallelRoutes[0].RuntimeProfileID != main.RuntimeProfileID ||
		main.ParallelRoutes[1].ProviderAccountID != "deepseek.backup" ||
		main.ParallelRoutes[1].CredentialRevision != 9 ||
		main.SynthesisRoute.RuntimeProfileID == main.RuntimeProfileID ||
		main.TimeoutMilliseconds != 90_000 {
		t.Fatalf("parallel RouteSet preview = %#v", main)
	}
	if _, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: edited.CatalogDigest, ViewVersion: edited.ViewVersion,
			Field: "main_fallback_role", Value: "role-main-backup",
		},
	); !errors.Is(err, ErrBuilderIncompatible) {
		t.Fatalf("parallel plus fallback error = %v", err)
	}
	confirmation, err := service.ConfirmBuilder(
		context.Background(),
		BuilderConfirmCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: edited.CatalogDigest, ViewVersion: edited.ViewVersion,
			BindingDigest: edited.BindingDigest, DefinitionID: "team-parallel-routes",
			Scope: "reusable", Confirm: true,
		},
	)
	if err != nil {
		t.Fatalf("ConfirmBuilder() error = %v", err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, ok := readModel.GlobalReadView().TeamDefinition(
		confirmation.TeamDefinitionID,
	)
	if !ok || !record.Configuration.RoleBindings[0].ParallelRouteSetAvailable ||
		len(record.Configuration.RoleBindings[0].ParallelRouteSet.AdditionalRoutes) != 1 ||
		record.Configuration.RoleBindings[0].ParallelRouteSet.AdditionalRoutes[0].
			ExecutionProfile.ProviderAccountID != "deepseek.backup" ||
		record.Configuration.RoleBindings[0].ParallelRouteSet.SynthesisRoute.
			ExecutionProfile.ID == record.Configuration.RoleBindings[0].ExecutionProfile.ID ||
		record.Configuration.RoleBindings[0].ExecutionProfile.Timeout != 90*time.Second {
		t.Fatalf("persisted parallel RouteSet = %#v", record.Configuration.RoleBindings)
	}
	reopened, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceSavedTeam, SourceID: confirmation.TeamDefinitionID,
			SourceVersion: confirmation.TeamDefinitionVersion,
			SourceDigest:  confirmation.TeamDefinitionDigest,
		},
	)
	if err != nil || len(reopened.Preview.Roles) != 2 ||
		len(reopened.Preview.Roles[0].ParallelRoutes) != 2 ||
		reopened.Preview.Roles[0].ParallelRoutes[1].ProviderAccountID != "deepseek.backup" {
		t.Fatalf("reopened parallel RouteSet = %#v, %v", reopened, err)
	}
}

func TestPhase2DBuilderEditsMultipleSubAgentsByStableDefinitionID(t *testing.T) {
	service, _, _, catalog := newSetupFixtureService(t)
	catalog.AgentDefinitions = append(catalog.AgentDefinitions, agents.AgentDefinition{
		ID:       "agent-peer",
		Version:  1,
		Scope:    agents.ScopeReusable,
		Name:     "Researcher",
		RoleSpec: "Research bounded work",
		Status:   agents.DefinitionActive,
	})
	catalog.RoleOptions = append(catalog.RoleOptions,
		SetupRoleOption{
			ID:                "role-peer",
			Kind:              "subagent",
			AgentDefinitionID: "agent-peer",
			RuntimeProfileID:  "profile-sub",
			RuntimeInstanceID: "runtime-pi",
			SkillRevisionIDs:  []string{},
			PermissionIDs:     []string{"permission-read"},
			ResourceIDs:       []string{"resource-repo"},
			Responsibility:    "Research",
		},
		SetupRoleOption{
			ID:                "role-sub-backup",
			Kind:              "subagent",
			AgentDefinitionID: "agent-sub",
			RuntimeProfileID:  "profile-main-backup",
			RuntimeInstanceID: "runtime-pi",
			SkillRevisionIDs:  []string{},
			PermissionIDs:     []string{"permission-read"},
			ResourceIDs:       []string{"resource-repo"},
			Responsibility:    "Review with backup account",
		},
	)
	if err := validateSetupCatalog(catalog); err != nil {
		t.Fatal(err)
	}
	service.catalog = cloneSetupCatalog(catalog)
	var err error
	service.domainCatalog, err = buildSetupDomainCatalog(catalog)
	if err != nil {
		t.Fatal(err)
	}

	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil || len(session.Preview.Roles) != 2 {
		t.Fatalf("StartBuilder() = %#v, %v", session, err)
	}
	added, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: session.DraftID, ExpectedRevision: session.Revision,
			CatalogDigest: session.CatalogDigest, ViewVersion: session.ViewVersion,
			Field: "subagent_add", Value: "role-peer",
		},
	)
	if err != nil || len(added.Preview.Roles) != 3 {
		t.Fatalf("EditBuilder(add peer) = %#v, %v", added, err)
	}
	peerBefore := added.Preview.Roles[2]
	internalSession := service.sessions[added.DraftID]
	beforeRevision := internalSession.view.Revision
	beforeDigest := internalSession.view.BindingDigest
	_, err = service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: added.DraftID, ExpectedRevision: added.Revision,
			CatalogDigest: added.CatalogDigest, ViewVersion: added.ViewVersion,
			Field: "subagent_provider_account_route", Value: "role-sub-backup",
		},
	)
	if !errors.Is(err, ErrInvalidLocalProductSetup) ||
		internalSession.view.Revision != beforeRevision ||
		internalSession.view.BindingDigest != beforeDigest {
		t.Fatalf("ambiguous subagent edit mutated Draft: revision=%d digest=%q err=%v",
			internalSession.view.Revision, internalSession.view.BindingDigest, err)
	}
	edited, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: added.DraftID, ExpectedRevision: added.Revision,
			CatalogDigest: added.CatalogDigest, ViewVersion: added.ViewVersion,
			Field: "subagent_provider_account_route", Value: "role-sub-backup",
			RoleAgentDefinitionID: "agent-sub",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(edited.Preview.Roles) != 3 ||
		edited.Preview.Roles[1].AgentDefinitionID != "agent-sub" ||
		edited.Preview.Roles[1].ProviderID != "deepseek" ||
		edited.Preview.Roles[1].ProviderAccountID != "deepseek.backup" ||
		edited.Preview.Roles[1].CredentialRevision != 9 ||
		!reflect.DeepEqual(edited.Preview.Roles[2], peerBefore) {
		t.Fatalf("targeted subagent edit = %#v", edited.Preview.Roles)
	}
	editedAgain, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: edited.DraftID, ExpectedRevision: edited.Revision,
			CatalogDigest: edited.CatalogDigest, ViewVersion: edited.ViewVersion,
			Field: "subagent_timeout_seconds", Value: "135",
			RoleAgentDefinitionID: "agent-sub",
		},
	)
	if err != nil || editedAgain.Preview.Roles[1].TimeoutMilliseconds != 135_000 ||
		editedAgain.Preview.Roles[1].ProviderAccountID != "deepseek.backup" ||
		!reflect.DeepEqual(editedAgain.Preview.Roles[2], peerBefore) {
		t.Fatalf("successive targeted subagent edit = %#v, %v", editedAgain.Preview.Roles, err)
	}
	removed, err := service.EditBuilder(
		context.Background(),
		BuilderEditCommand{
			DraftID: editedAgain.DraftID, ExpectedRevision: editedAgain.Revision,
			CatalogDigest: editedAgain.CatalogDigest, ViewVersion: editedAgain.ViewVersion,
			Field: "subagent_remove", Value: "remove",
			RoleAgentDefinitionID: "agent-peer",
		},
	)
	if err != nil || len(removed.Preview.Roles) != 2 ||
		removed.Preview.Roles[1].AgentDefinitionID != "agent-sub" {
		t.Fatalf("EditBuilder(remove peer) = %#v, %v", removed, err)
	}
}

func TestPhase2DBuilderRejectsInvalidCustomExecutionProfileWithoutMutation(
	t *testing.T,
) {
	service, _, _, catalog := newSetupFixtureService(t)
	session, err := service.StartBuilder(
		context.Background(),
		BuilderStartCommand{
			Source: BuilderSourceTemplate, SourceID: "template-review",
			SourceVersion: 3, SourceDigest: setupDigest("template"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	stored := service.sessions[session.DraftID]
	baselineRoleID := stored.mainRoleID
	baselineProfiles := len(stored.catalog.RuntimeProfiles)
	for _, change := range []struct {
		field string
		value string
	}{
		{"main_model", "unobserved-model"},
		{"main_reasoning_effort", "Maximum"},
		{"main_timeout_seconds", "0"},
		{"main_budget_units", "-1"},
		{"main_fallback_role", "role-sub"},
	} {
		if _, err := service.EditBuilder(
			context.Background(),
			BuilderEditCommand{
				DraftID: session.DraftID, ExpectedRevision: session.Revision,
				CatalogDigest: catalog.CatalogDigest, ViewVersion: session.ViewVersion,
				Field: change.field, Value: change.value,
			},
		); err == nil {
			t.Fatalf("EditBuilder(%s) accepted invalid value", change.field)
		}
		stored = service.sessions[session.DraftID]
		if stored.view.Revision != session.Revision ||
			stored.mainRoleID != baselineRoleID ||
			len(stored.catalog.RuntimeProfiles) != baselineProfiles {
			t.Fatalf("EditBuilder(%s) mutated session = %#v", change.field, stored)
		}
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
					ID:                "runtime-pi",
					DeviceID:          "device-local",
					AdapterType:       "pi",
					DisplayName:       "Pi Coding Agent",
					ExecutableVersion: "0.82.1",
					Status:            loomruntime.RuntimeOnline,
					ObservedCapabilities: []string{
						"rpc", loomruntime.CapabilityReasoningEffort,
					},
					Capacity: 2,
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
	mainBudget := int64(80)
	subBudget := int64(40)
	mainProfile := loomruntime.RuntimeProfile{
		ID:                  "profile-main",
		AdapterType:         "pi",
		ProviderID:          "openai",
		ProviderAccountID:   "openai.primary",
		ModelID:             "model-a",
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: setupDigest("openai-endpoint"),
		CredentialReference: "credential-ref-openai-primary",
		CredentialRevision:  3,
		ReasoningEffort:     "high",
		RequiredCapabilities: []string{
			"rpc", loomruntime.CapabilityReasoningEffort,
		},
		Timeout: time.Minute,
		Budget:  &mainBudget,
	}
	subProfile := mainProfile
	subProfile.ID = "profile-sub"
	subProfile.ProviderID = "anthropic"
	subProfile.ProviderAccountID = "anthropic.review"
	subProfile.EndpointFingerprint = setupDigest("anthropic-endpoint")
	subProfile.CredentialReference = "credential-ref-anthropic-review"
	subProfile.CredentialRevision = 7
	subProfile.ReasoningEffort = "low"
	subProfile.Budget = &subBudget
	backupBudget := int64(25)
	backupProfile := mainProfile
	backupProfile.ID = "profile-main-backup"
	backupProfile.ProviderID = "deepseek"
	backupProfile.ProviderAccountID = "deepseek.backup"
	backupProfile.EndpointFingerprint = setupDigest("deepseek-endpoint")
	backupProfile.CredentialReference = "credential-ref-deepseek-backup"
	backupProfile.CredentialRevision = 9
	backupProfile.ReasoningEffort = "medium"
	backupProfile.Budget = &backupBudget
	catalog := LocalProductSetupCatalog{
		CatalogDigest:    setupDigest("catalog"),
		AgentDefinitions: definitions,
		RuntimeProfiles: []loomruntime.RuntimeProfile{
			mainProfile, subProfile, backupProfile,
		},
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
			{
				ID:                "role-main-backup",
				Kind:              "main",
				AgentDefinitionID: "agent-main",
				RuntimeProfileID:  "profile-main-backup",
				RuntimeInstanceID: "runtime-pi",
				SkillRevisionIDs:  []string{"skill-review@2"},
				PermissionIDs:     []string{"permission-read"},
				ResourceIDs:       []string{"resource-repo"},
				Responsibility:    "Coordinate with backup account",
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

type setupCredentialStageFailure struct {
	stage string
	err   error
}

func (failure *setupCredentialStageFailure) Error() string { return failure.err.Error() }
func (failure *setupCredentialStageFailure) Unwrap() error { return failure.err }
func (failure *setupCredentialStageFailure) CredentialFailureStage() string {
	return failure.stage
}

func TestSetupCredentialResultPreservesClassifiedFailureStage(t *testing.T) {
	staged := &setupCredentialStageFailure{
		stage: credentials.CredentialStageHelperAuthorization,
		err:   credentials.ErrCredentialStoreUnavailable,
	}
	_, err := setupCredentialResult(credentials.MetadataResult{}, staged)
	if err != staged ||
		!errors.Is(err, credentials.ErrCredentialStoreUnavailable) ||
		credentials.CredentialFailureStage(err) !=
			credentials.CredentialStageHelperAuthorization {
		t.Fatalf("classified credential failure changed: %v", err)
	}
}

func TestLocalProductSetupPreservesClassifiedCredentialMutatorFailure(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	staged := &setupCredentialStageFailure{
		stage: credentials.CredentialStageHelperAuthorization,
		err:   credentials.ErrCredentialStoreUnavailable,
	}
	service.credentialMutator = &setupCapturingCredentialMutator{err: staged}
	secret := []byte("synthetic-secret-cleared-by-service")
	_, err := service.ConfigureCredential(
		context.Background(),
		CredentialSetupCommand{ProviderID: "deepseek", Secret: secret},
	)
	if err != staged ||
		credentials.CredentialFailureStage(err) !=
			credentials.CredentialStageHelperAuthorization {
		t.Fatalf("classified mutator failure changed: %v", err)
	}
	for _, value := range secret {
		if value != 0 {
			t.Fatal("credential input was not cleared")
		}
	}
}

func TestLocalProductSetupConfiguresRegisteredBrokeredProvider(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	mutator := &setupCapturingCredentialMutator{}
	service.credentialMutator = mutator
	secret := []byte("deepseek-secret")

	result, err := service.ConfigureCredential(
		context.Background(),
		CredentialSetupCommand{ProviderID: "deepseek", Secret: secret},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderID != "deepseek" ||
		mutator.command.ProviderID != "deepseek" ||
		mutator.command.ProviderAccountID != "deepseek.primary" ||
		mutator.command.CredentialReference == "" {
		t.Fatalf("result=%#v command=%#v", result, mutator.command)
	}
	if _, err := service.ConfigureCredential(
		context.Background(),
		CredentialSetupCommand{ProviderID: "aws-bedrock", Secret: []byte("x")},
	); !errors.Is(err, ErrCredentialSetupUnavailable) {
		t.Fatalf("managed cloud configure error = %v", err)
	}
}

func TestLocalProductSetupConfiguresIndependentProviderAccount(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	mutator := &setupCapturingCredentialMutator{}
	service.credentialMutator = mutator
	_, err := service.ConfigureCredential(
		context.Background(), CredentialSetupCommand{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			Secret: []byte("deepseek-work-secret"),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if mutator.command.ProviderID != "deepseek" ||
		mutator.command.ProviderAccountID != "deepseek.work" {
		t.Fatalf("command = %#v", mutator.command)
	}
	_, err = service.ConfigureCredential(
		context.Background(), CredentialSetupCommand{
			ProviderID: "deepseek", ProviderAccountID: "openai.work",
			Secret: []byte("must-not-reach-mutator"),
		},
	)
	if !errors.Is(err, ErrCredentialSetupUnavailable) {
		t.Fatalf("cross-provider account error = %v", err)
	}
}

func TestLocalProductSetupPublishesReadyConversationProfiles(t *testing.T) {
	service, _, _, _ := newSetupFixtureService(t)
	service.credentials = setupFixtureBroker{statuses: map[string]credentials.MetadataResult{
		"deepseek": {
			ProviderID: "deepseek", CredentialReference: "credential-ref-deepseek-1",
			Revision: 2, Status: credentials.CredentialVerified,
		},
		"kimi": {
			ProviderID: "kimi", CredentialReference: "credential-ref-kimi-1",
			Revision: 3, Status: credentials.CredentialVerified,
		},
		"minimax": {
			ProviderID: "minimax", CredentialReference: "credential-ref-minimax-1",
			Revision: 4, Status: credentials.CredentialVerified,
		},
	}}
	snapshot, err := service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.ConversationProfiles) != 5 {
		t.Fatalf("profiles = %#v", snapshot.ConversationProfiles)
	}
	if snapshot.ConversationProfiles[0].ProfileID !=
		"conversation-openai-codex-default-v1" ||
		snapshot.ConversationProfiles[0].ProviderID != "openai" ||
		snapshot.ConversationProfiles[1].ProfileID !=
			"conversation-opencode-default-v1" ||
		snapshot.ConversationProfiles[1].ProviderID != "opencode" ||
		snapshot.ConversationProfiles[1].HarnessAdapter != "opencode" ||
		snapshot.ConversationProfiles[1].Protocol != "opencode_agent" ||
		snapshot.ConversationProfiles[1].ModelID != "deepseek/deepseek-chat" ||
		snapshot.ConversationProfiles[2].ProfileID !=
			"conversation-deepseek-deepseek-chat-r2" ||
		snapshot.ConversationProfiles[2].ProviderID != "deepseek" ||
		snapshot.ConversationProfiles[2].ProviderAccountID != "deepseek.primary" ||
		snapshot.ConversationProfiles[2].HarnessAdapter != "loom-native" ||
		snapshot.ConversationProfiles[2].ModelID != "deepseek-chat" ||
		snapshot.ConversationProfiles[2].CredentialRevision != 2 ||
		snapshot.ConversationProfiles[3].ProfileID !=
			"conversation-kimi-kimi-k2.6-r3" ||
		snapshot.ConversationProfiles[3].ProviderID != "kimi" ||
		snapshot.ConversationProfiles[3].ModelID != "kimi-k2.6" ||
		snapshot.ConversationProfiles[3].CredentialRevision != 3 ||
		snapshot.ConversationProfiles[4].ProfileID !=
			"conversation-minimax-minimax-m3-r4" ||
		snapshot.ConversationProfiles[4].ProviderID != "minimax" ||
		snapshot.ConversationProfiles[4].ProviderAccountID != "minimax.primary" ||
		snapshot.ConversationProfiles[4].HarnessAdapter != "loom-native" ||
		snapshot.ConversationProfiles[4].ModelID != "MiniMax-M3" ||
		snapshot.ConversationProfiles[4].CredentialRevision != 4 {
		t.Fatalf("profiles = %#v", snapshot.ConversationProfiles)
	}
}

func TestSetupConversationProfilesPublishesEveryVerifiedProviderAccount(t *testing.T) {
	profiles := setupConversationProfiles(
		NativeAuthObservation{},
		[]ProviderDirectoryEntry{{
			ProviderID: "deepseek", Status: "verified", Revision: 2,
		}},
		[]ProviderAccountDirectoryEntry{
			{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				AuthMode: "brokered", Revision: 2, Status: "verified",
			},
			{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
				AuthMode: "brokered", Revision: 2, Status: "verified",
			},
			{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.revoked",
				AuthMode: "brokered", Revision: 4, Status: "revoked",
			},
		},
	)
	if len(profiles) != 2 ||
		profiles[0].ProfileID != provider.DeepSeekConversationProfileID(2) ||
		profiles[0].ProviderAccountID != "deepseek.primary" ||
		profiles[1].ProfileID != provider.DeepSeekConversationAccountProfileID(
			"deepseek.work", 2,
		) ||
		profiles[1].ProviderAccountID != "deepseek.work" ||
		profiles[1].CredentialRevision != 2 {
		t.Fatalf("profiles = %#v", profiles)
	}
}

func TestSetupConversationProfilesOpenCodeDefaultFollowsVerifiedProvider(t *testing.T) {
	// Only DeepSeek verified -> the OpenCode profile defaults to a DeepSeek
	// model instead of the catalog default (openai/gpt-5.5) which has no
	// configured credential.
	profiles := setupConversationProfiles(
		NativeAuthObservation{Status: "available", AuthMode: "native_auth"},
		nil,
		[]ProviderAccountDirectoryEntry{{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			AuthMode: "brokered", Revision: 6, Status: "verified",
		}},
	)
	if len(profiles) != 3 {
		t.Fatalf("profiles = %#v", profiles)
	}
	opencode := profiles[1]
	if opencode.ProfileID != provider.OpenCodeConversationProfileID ||
		opencode.ModelID != "deepseek/deepseek-chat" {
		t.Fatalf("opencode profile = %#v", opencode)
	}

	// No verified accounts -> fall back to the catalog default.
	profiles = setupConversationProfiles(
		NativeAuthObservation{Status: "available", AuthMode: "native_auth"},
		nil,
		nil,
	)
	if len(profiles) != 2 {
		t.Fatalf("profiles = %#v", profiles)
	}
	if profiles[1].ProfileID != provider.OpenCodeConversationProfileID ||
		profiles[1].ModelID != provider.OpenCodeConversationDefaultModel {
		t.Fatalf("opencode profile = %#v", profiles[1])
	}
}

func TestSetupConversationProfilesFreezeExactAccountDisclosurePolicy(t *testing.T) {
	digest := strings.Repeat("a", 64)
	profiles := setupConversationProfiles(
		NativeAuthObservation{},
		nil,
		[]ProviderAccountDirectoryEntry{
			{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.legacy",
				AuthMode: "brokered", Revision: 2, Status: "verified",
			},
			{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
				AuthMode: "brokered", Revision: 3, Status: "verified",
				PolicyAvailable: true, PolicyVersion: 2, PolicyRevision: 4,
				PolicyDigest: digest, TrustDomain: "enterprise_tenant",
				RetentionMode: "zero_data_retention", DataRegion: "apac",
			},
		},
	)
	if len(profiles) != 2 {
		t.Fatalf("profiles = %#v", profiles)
	}
	legacy := profiles[0]
	configured := profiles[1]
	if legacy.ProviderAccountID != "deepseek.legacy" || legacy.PolicyVersion != 0 ||
		legacy.PolicyRevision != 0 || legacy.PolicyDigest != "" ||
		legacy.TrustDomain != "" || legacy.RetentionMode != "" || legacy.DataRegion != "" {
		t.Fatalf("legacy profile = %#v", legacy)
	}
	if configured.ProviderAccountID != "deepseek.work" || configured.PolicyVersion != 2 ||
		configured.PolicyRevision != 4 || configured.PolicyDigest != digest ||
		configured.TrustDomain != "enterprise_tenant" ||
		configured.RetentionMode != "zero_data_retention" ||
		configured.DataRegion != "apac" {
		t.Fatalf("configured profile = %#v", configured)
	}
}

func TestSetupConversationProfilesPublishesAnthropicMessagesAccount(t *testing.T) {
	profiles := setupConversationProfiles(
		NativeAuthObservation{},
		nil,
		[]ProviderAccountDirectoryEntry{{
			ProviderID: "anthropic", ProviderAccountID: "anthropic.work",
			AuthMode: "brokered", Revision: 5, Status: "verified",
		}},
	)
	if len(profiles) != 1 ||
		profiles[0].ProfileID != provider.AnthropicConversationAccountProfileID(
			"anthropic.work", 5,
		) ||
		profiles[0].HarnessAdapter != "loom-native" ||
		profiles[0].ProviderID != "anthropic" ||
		profiles[0].ProviderAccountID != "anthropic.work" ||
		profiles[0].Protocol != "anthropic_messages" ||
		profiles[0].ModelID != provider.AnthropicConversationModelID ||
		profiles[0].CredentialRevision != 5 {
		t.Fatalf("profiles = %#v", profiles)
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
