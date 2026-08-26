package ccswitch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const fixtureSecret = "fixture-provider-secret-must-never-escape"

func TestSourceDiscoversNonSecretProviderCandidates(t *testing.T) {
	path := createCCSwitchFixture(t, []ccSwitchFixtureProvider{
		{
			id: "deepseek", name: "DeepSeek", current: true,
			baseURL: "https://api.deepseek.com", model: "deepseek-v4-flash",
		},
		{
			id: "kimi-coding", name: "Kimi For Coding",
			baseURL: "https://api.kimi.com/coding/v1", model: "kimi-for-coding",
		},
		{
			id: "oauth", name: "OpenAI Official", oauthOnly: true,
			baseURL: "https://api.openai.com/v1", model: "gpt-5.5",
		},
	})
	source, err := NewSource(path)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %#v", candidates)
	}
	deepseek := candidates[0]
	if deepseek.DisplayName != "DeepSeek" || deepseek.TargetProviderID != "deepseek" ||
		deepseek.Protocol != ProtocolOpenAIResponses || deepseek.ImportMode != ImportExactProvider ||
		deepseek.Endpoint != "https://api.deepseek.com/" ||
		len(deepseek.ModelIDs) != 1 || deepseek.ModelIDs[0] != "deepseek-v4-flash" ||
		!deepseek.Current || deepseek.CandidateID == "" || deepseek.CandidateDigest == "" ||
		deepseek.EndpointFingerprint == "" || deepseek.ReviewPolicyVersion != 1 ||
		deepseek.ReviewPolicyDigest == "" || !deepseek.CredentialAvailable {
		t.Fatalf("deepseek candidate = %#v", deepseek)
	}
	kimi := candidates[1]
	if kimi.TargetProviderID != "custom-openai" ||
		kimi.ImportMode != ImportCustomEndpointReview ||
		kimi.Endpoint != "https://api.kimi.com/coding/v1" ||
		kimi.CandidateDigest == "" || kimi.EndpointFingerprint == "" {
		t.Fatalf("kimi candidate = %#v", kimi)
	}
	encoded, err := json.Marshal(candidates)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{fixtureSecret, "access_token", "refresh_token", "settings_config"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("candidate output contains forbidden value %q", forbidden)
		}
	}
}

func TestSourceUsesSelectedAPIKeyOnceAndZeroizesLease(t *testing.T) {
	path := createCCSwitchFixture(t, []ccSwitchFixtureProvider{{
		id: "minimax", name: "MiniMax", baseURL: "https://api.minimaxi.com/v1",
		model: "MiniMax-M3",
	}})
	source, err := NewSource(path)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := source.Discover(context.Background())
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
	}
	var leased []byte
	err = source.UseAPIKey(
		context.Background(), candidates[0].CandidateID,
		func(_ context.Context, secret []byte) error {
			if string(secret) != fixtureSecret {
				t.Fatalf("secret length=%d", len(secret))
			}
			leased = secret
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for index, value := range leased {
		if value != 0 {
			t.Fatalf("leased secret byte %d was not zeroized", index)
		}
	}
	if err := source.UseAPIKey(
		context.Background(), strings.Repeat("a", 64),
		func(context.Context, []byte) error { return nil },
	); !errors.Is(err, ErrCandidateUnavailable) {
		t.Fatalf("unknown candidate error = %v", err)
	}
}

func TestSourceAllowsOwnedInPlaceDatabaseGrowth(t *testing.T) {
	path := createCCSwitchFixture(t, []ccSwitchFixtureProvider{{
		id: "deepseek", name: "DeepSeek", baseURL: "https://api.deepseek.com",
		model: "deepseek-chat",
	}})
	source, err := NewSource(path)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`UPDATE providers SET settings_config =
json_set(settings_config, '$.loom_test_padding', hex(randomblob(32768)))`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, err := source.Discover(context.Background())
	if err != nil || len(candidates) != 1 || candidates[0].DisplayName != "DeepSeek" {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
	}
}

func TestSourceRejectsUnsafeFilesystemAndEndpoints(t *testing.T) {
	path := createCCSwitchFixture(t, []ccSwitchFixtureProvider{{
		id: "unsafe", name: "Unsafe", baseURL: "http://127.0.0.1:8080/v1",
		model: "unsafe-model",
	}})
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSource(path); !errors.Is(err, ErrUnsafeSource) {
		t.Fatalf("world-readable source error = %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := NewSource(path)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := source.Discover(context.Background())
	if err != nil || len(candidates) != 0 {
		t.Fatalf("unsafe endpoint candidates=%#v err=%v", candidates, err)
	}
	symlink := filepath.Join(filepath.Dir(path), "cc-switch-link.db")
	if err := os.Symlink(path, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSource(symlink); !errors.Is(err, ErrUnsafeSource) {
		t.Fatalf("symlink source error = %v", err)
	}
	hardlink := filepath.Join(filepath.Dir(path), "cc-switch-hardlink.db")
	if err := os.Link(path, hardlink); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSource(path); !errors.Is(err, ErrUnsafeSource) {
		t.Fatalf("hardlinked source error = %v", err)
	}
}

func TestLiveInstalledSourceDiscovery(t *testing.T) {
	path := os.Getenv("LOOM_CC_SWITCH_SOURCE")
	if path == "" {
		t.Skip("installed CC Switch source gate requires LOOM_CC_SWITCH_SOURCE")
	}
	source, err := NewSource(path)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := source.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("installed CC Switch source produced no safe API-key candidates")
	}
	for _, candidate := range candidates {
		if !candidate.CredentialAvailable || candidate.CandidateID == "" ||
			candidate.Endpoint == "" || len(candidate.ModelIDs) == 0 {
			t.Fatalf("invalid installed candidate = %#v", candidate)
		}
		t.Logf(
			"candidate=%q target=%s protocol=%s mode=%s models=%d",
			candidate.DisplayName, candidate.TargetProviderID,
			candidate.Protocol, candidate.ImportMode, len(candidate.ModelIDs),
		)
	}
}

type ccSwitchFixtureProvider struct {
	id, name, baseURL, model string
	current                  bool
	oauthOnly                bool
}

func createCCSwitchFixture(t *testing.T, providers []ccSwitchFixtureProvider) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "cc-switch.db")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE providers (
id TEXT NOT NULL, app_type TEXT NOT NULL, name TEXT NOT NULL,
settings_config TEXT NOT NULL, is_current BOOLEAN NOT NULL DEFAULT 0,
PRIMARY KEY (id, app_type))`)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range providers {
		auth := map[string]any{"OPENAI_API_KEY": fixtureSecret}
		if item.oauthOnly {
			auth = map[string]any{"auth_mode": "chatgpt", "tokens": map[string]any{
				"access_token": "fixture-oauth-token", "refresh_token": "fixture-refresh-token",
			}}
		}
		settings, err := json.Marshal(map[string]any{
			"auth": auth,
			"config": "model_provider = \"custom\"\nmodel = \"" + item.model +
				"\"\n[model_providers.custom]\nbase_url = \"" + item.baseURL +
				"\"\nwire_api = \"responses\"\nrequires_openai_auth = true\n",
			"modelCatalog": map[string]any{"models": []map[string]any{{
				"model": item.model, "displayName": item.name, "contextWindow": 128000,
			}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := database.Exec(
			`INSERT INTO providers(id, app_type, name, settings_config, is_current)
VALUES(?, 'codex', ?, ?, ?)`,
			item.id, item.name, string(settings), item.current,
		); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
