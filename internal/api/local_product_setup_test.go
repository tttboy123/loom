package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"loom-pi-rebuild/internal/app"
)

type setupAPIFixtureBackend struct {
	snapshot app.SetupSnapshot
	session  app.BuilderSessionView
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
