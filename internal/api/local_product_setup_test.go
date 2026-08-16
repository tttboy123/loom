package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"loom-pi-rebuild/internal/app"
)

type setupAPIFixtureBackend struct {
	snapshot      app.SetupSnapshot
	session       app.BuilderSessionView
	connect       app.ProviderConnectResult
	policy        app.ProviderAccountPolicyResult
	policyCommand app.ProviderAccountPolicyCommand
	editCommand   app.BuilderEditCommand
	closed        bool
}

func TestLocalProductSetupAPINilReceiverFailsClosed(t *testing.T) {
	var service *LocalProductSetupAPI
	if _, err := service.AnswerBuilder(
		context.Background(),
		app.BuilderAnswerCommand{},
	); !errors.Is(err, ErrInvalidLocalProductSetupAPI) {
		t.Fatalf("AnswerBuilder() error = %v", err)
	}
	if _, err := service.ConfigureCredential(
		context.Background(),
		app.CredentialSetupCommand{},
	); !errors.Is(err, ErrInvalidLocalProductSetupAPI) {
		t.Fatalf("ConfigureCredential() error = %v", err)
	}
}

func (backend *setupAPIFixtureBackend) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return backend.snapshot, nil
}

func (backend *setupAPIFixtureBackend) StartBuilder(
	_ context.Context,
	_ app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	return backend.session, nil
}

func (backend *setupAPIFixtureBackend) EditBuilder(
	_ context.Context,
	command app.BuilderEditCommand,
) (app.BuilderSessionView, error) {
	backend.editCommand = command
	return backend.session, nil
}

func (backend *setupAPIFixtureBackend) ConnectCodex(
	context.Context,
) (app.ProviderConnectResult, error) {
	return backend.connect, nil
}

func (backend *setupAPIFixtureBackend) ConfigureProviderAccountPolicy(
	_ context.Context,
	command app.ProviderAccountPolicyCommand,
) (app.ProviderAccountPolicyResult, error) {
	backend.policyCommand = command
	return backend.policy, nil
}

func (backend *setupAPIFixtureBackend) Close() error {
	backend.closed = true
	return nil
}

func TestLocalProductSetupAPIExposesStrictCodexConnectResult(t *testing.T) {
	backend := &setupAPIFixtureBackend{connect: app.ProviderConnectResult{
		ProviderID: "codex",
		AuthMode:   "native_auth",
		Status:     "started",
	}}
	service, err := NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.ConnectCodex(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != backend.connect {
		t.Fatalf("ConnectCodex() = %#v", got)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if !backend.closed {
		t.Fatal("Close() did not propagate to the setup backend")
	}
}

func TestLocalProductSetupAPIConfiguresExactProviderAccountPolicy(t *testing.T) {
	backend := &setupAPIFixtureBackend{policy: app.ProviderAccountPolicyResult{
		PolicyAvailable: true, PolicyVersion: 2, ProviderID: "deepseek",
		ProviderAccountID: "deepseek.work", Revision: 2,
		PolicyDigest:              setupAPIDigest("deepseek-work-policy"),
		MaximumConcurrentAttempts: 3, DispatchWindowSeconds: 60,
		MaximumDispatchStarts: 12, MaximumAssignedBudgetUnits: 8_000,
		TrustDomain: "external_provider", RetentionMode: "zero_data_retention",
		DataRegion:   "apac",
		ConfiguredAt: "2026-08-11T10:00:00Z",
	}}
	service, err := NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	command := app.ProviderAccountPolicyCommand{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		ExpectedRevision: 1, MaximumConcurrentAttempts: 3,
		DispatchWindowSeconds: 60, MaximumDispatchStarts: 12,
		MaximumAssignedBudgetUnits: 8_000,
		TrustDomain:                "external_provider", RetentionMode: "zero_data_retention",
		DataRegion:    "apac",
		OperationID:   "configure-deepseek-work-v2",
		CorrelationID: "loom-policy-api-test",
	}
	result, err := service.ConfigureProviderAccountPolicy(
		context.Background(), command,
	)
	if err != nil || result != backend.policy || backend.policyCommand != command {
		t.Fatalf("ConfigureProviderAccountPolicy() = %#v, command=%#v, err=%v", result, backend.policyCommand, err)
	}
}

func TestLocalProductSetupAPIForwardsIndependentProviderAccountRouteEdit(t *testing.T) {
	backend := &setupAPIFixtureBackend{session: app.BuilderSessionView{
		SchemaVersion: 1,
		DraftID:       "draft-route-edit",
		Revision:      5,
		Source:        app.BuilderSourceBlank,
	}}
	service, err := NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	command := app.BuilderEditCommand{
		DraftID:          "draft-route-edit",
		ExpectedRevision: 4,
		CatalogDigest:    setupAPIDigest("catalog-route-edit"),
		ViewVersion:      setupAPIDigest("view-route-edit"),
		Field:            "main_provider_account_route",
		Value:            "role-main-backup",
	}
	result, err := service.EditBuilder(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if backend.editCommand != command {
		t.Fatalf("EditBuilder() command = %#v", backend.editCommand)
	}
	if result.DraftID != backend.session.DraftID || result.Revision != backend.session.Revision {
		t.Fatalf("EditBuilder() result = %#v", result)
	}
}

func TestLocalProductSetupAPICanonicalizesCollectionsAndCopiesResults(
	t *testing.T,
) {
	backend := &setupAPIFixtureBackend{
		snapshot: app.SetupSnapshot{
			SchemaVersion: 1,
			ViewVersion:   setupAPIDigest("view"),
			Codex: app.ProviderSetupStatus{
				ProviderID: "codex",
				AuthMode:   "native_auth",
				Status:     "available",
			},
			MiniMax: app.ProviderSetupStatus{
				ProviderID: "minimax",
				AuthMode:   "brokered",
				Status:     "unconfigured",
			},
		},
		session: app.BuilderSessionView{
			SchemaVersion: 1,
			DraftID:       "draft-1",
			Revision:      1,
			Source:        app.BuilderSourceBlank,
			Question: app.BuilderQuestion{
				ID:     "team_name",
				Prompt: "Name this team",
			},
		},
	}
	service, err := NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatalf("NewLocalProductSetupAPI() error = %v", err)
	}
	snapshot, err := service.SetupSnapshot(context.Background())
	if err != nil {
		t.Fatalf("SetupSnapshot() error = %v", err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`"providers":[]`,
		`"conversation_profiles":[]`,
		`"runtimes":[]`,
		`"saved_teams":[]`,
		`"templates":[]`,
		`"role_options":[]`,
		`"skills":[]`,
		`"permissions":[]`,
		`"resources":[]`,
	} {
		if !jsonContainsSetup(encoded, required) {
			t.Fatalf("snapshot JSON %s missing %s", encoded, required)
		}
	}

	session, err := service.StartBuilder(
		context.Background(),
		app.BuilderStartCommand{Source: app.BuilderSourceBlank},
	)
	if err != nil {
		t.Fatalf("StartBuilder() error = %v", err)
	}
	encoded, err = json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`"options":[]`,
		`"roles":[]`,
		`"permissions":[]`,
		`"resources":[]`,
		`"compatibility_gaps":[]`,
	} {
		if !jsonContainsSetup(encoded, required) {
			t.Fatalf("session JSON %s missing %s", encoded, required)
		}
	}
}

func jsonContainsSetup(encoded []byte, required string) bool {
	for index := 0; index+len(required) <= len(encoded); index++ {
		if string(encoded[index:index+len(required)]) == required {
			return true
		}
	}
	return false
}

func setupAPIDigest(seed string) string {
	const hexadecimal = "0123456789abcdef"
	output := make([]byte, 64)
	for index := range output {
		output[index] = hexadecimal[(index+len(seed))%len(hexadecimal)]
	}
	return string(output)
}
