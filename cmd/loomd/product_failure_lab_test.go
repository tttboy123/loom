package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/provider/failurelab"
)

func TestLiveInstalledEndpointFailureLabAndCatalog(t *testing.T) {
	if os.Getenv("LOOM_LIVE_FAILURE_LAB_E2E") != "1" {
		t.Skip("installed Failure Lab gate requires LOOM_LIVE_FAILURE_LAB_E2E=1")
	}
	socketPath := os.Getenv("LOOM_LIVE_SOCKET")
	if socketPath == "" {
		socketPath = "/Users/lune/Library/Application Support/Loom/run/loomd.sock"
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 5 * time.Second, ExtendedTimeout: 15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var setup struct {
		Providers []struct {
			ProviderID string `json:"provider_id"`
		} `json:"providers"`
		Runtimes []struct {
			RuntimeInstanceID string `json:"runtime_instance_id"`
		} `json:"runtimes"`
		ConversationProfiles []struct {
			ProfileID string `json:"profile_id"`
		} `json:"conversation_profiles"`
		CredentialImportCandidates []struct {
			CandidateDigest     string `json:"candidate_digest"`
			EndpointFingerprint string `json:"endpoint_fingerprint"`
			ReviewPolicyVersion int    `json:"review_policy_version"`
			ReviewPolicyDigest  string `json:"review_policy_digest"`
		} `json:"credential_import_candidates"`
	}
	if err := client.Call(ctx, "setup_snapshot", struct{}{}, &setup); err != nil {
		t.Fatalf("setup snapshot: %v", err)
	}
	if len(setup.Providers) != 25 || len(setup.Runtimes) != 7 ||
		len(setup.ConversationProfiles) == 0 {
		t.Fatalf("catalog providers=%d runtimes=%d profiles=%d", len(setup.Providers),
			len(setup.Runtimes), len(setup.ConversationProfiles))
	}
	for index, candidate := range setup.CredentialImportCandidates {
		if candidate.CandidateDigest == "" || candidate.EndpointFingerprint == "" ||
			candidate.ReviewPolicyVersion <= 0 || candidate.ReviewPolicyDigest == "" {
			t.Fatalf("candidate %d missing review binding", index)
		}
	}

	for _, scenario := range []string{
		"auth", "rate_limit", "timeout", "insufficient_balance",
		"corrupt_vault_record", "revision_conflict",
	} {
		var result productFailureLabResult
		err := client.Call(ctx, "provider_failure_lab_run", productFailureLabRequest{
			Scenario: scenario, ProviderAccountID: "failure-lab.target",
			HealthyPeerAccountID: "failure-lab.healthy-peer",
		}, &result)
		if err != nil {
			t.Fatalf("scenario %s: %v", scenario, err)
		}
		if result.SchemaVersion != 1 || result.Scenario != scenario ||
			result.IncidentID == "" || len(result.Agents) != 2 ||
			result.Agents[0].Status != "failed" || result.Agents[0].Stage == "" ||
			result.Agents[0].Code == "" || result.Agents[1].Status != "succeeded" ||
			result.Agents[1].Stage != "" || result.Agents[1].Code != "" {
			t.Fatalf("scenario %s result=%#v", scenario, result)
		}
	}
}

func TestProductFailureLabRunsAllSixCellsWithHealthyPeerIsolation(t *testing.T) {
	want := map[string]struct {
		stage, code string
		retryable   bool
	}{
		"auth":                 {"provider_auth", "provider_auth", false},
		"rate_limit":           {"provider_rate_limit", "provider_rate_limit", true},
		"timeout":              {"provider_connect", "timeout", true},
		"insufficient_balance": {"provider_http", "provider_insufficient_balance", false},
		"corrupt_vault_record": {"vault_aad_validation", "corrupt_vault_record", false},
		"revision_conflict":    {"agent_attempt_dispatch", "credential_revision_conflict", false},
	}
	for scenario, expected := range want {
		t.Run(scenario, func(t *testing.T) {
			result, err := runProductFailureLab(
				context.Background(), failurelab.NewRunner(failurelab.NewTransport()),
				"incident-failure-lab-"+scenario,
				productFailureLabRequest{
					Scenario: scenario, ProviderAccountID: legacyFailureLabTargetAccount,
					HealthyPeerAccountID: legacyFailureLabPeerAccount,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.SchemaVersion != 1 || len(result.Agents) != 2 ||
				result.Agents[0].Status != "failed" ||
				result.Agents[0].Stage != expected.stage ||
				result.Agents[0].Code != expected.code ||
				result.Agents[0].Retryable != expected.retryable ||
				result.Agents[0].ElapsedMillis <= 0 ||
				result.Agents[1].Status != "succeeded" ||
				result.Agents[1].Stage != "" || result.Agents[1].Code != "" ||
				result.Agents[1].ElapsedMillis <= 0 {
				t.Fatalf("result=%#v", result)
			}
			if !strings.HasPrefix(result.Agents[0].ProviderAccountID, "failurelab_tmp_") ||
				!strings.HasPrefix(result.Agents[1].ProviderAccountID, "failurelab_tmp_") ||
				result.Agents[0].ProviderAccountID == result.Agents[1].ProviderAccountID ||
				result.Agents[0].ProviderAccountID == legacyFailureLabTargetAccount ||
				result.Agents[1].ProviderAccountID == legacyFailureLabPeerAccount {
				t.Fatalf("accounts are not isolated server-owned temporaries: %#v", result.Agents)
			}
		})
	}
}

func TestProductFailureLabRouteReturnsOnlySafeStructuredIsolationResult(t *testing.T) {
	handler := newProductRouteHandler(productRouteServices{
		failureLab: failurelab.NewRunner(failurelab.NewTransport()),
	})
	params, err := json.Marshal(productFailureLabRequest{
		Scenario: "auth", ProviderAccountID: "failure-lab.target",
		HealthyPeerAccountID: "failure-lab.healthy-peer",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "incident-failure-lab-route",
		Method: "provider_failure_lab_run", Params: params,
	})
	if !response.OK || response.Error != nil {
		t.Fatalf("response=%#v", response)
	}
	encoded := string(response.Result)
	for _, forbidden := range []string{
		"api_key", "authorization", "credential", "endpoint", "prompt",
		"provider_body", "provider_response",
	} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("response contains forbidden field %q: %s", forbidden, encoded)
		}
	}
}

func TestProductFailureLabRouteOwnsTemporaryAccounts(t *testing.T) {
	handler := newProductRouteHandler(productRouteServices{
		failureLab: failurelab.NewRunner(failurelab.NewTransport()),
	})
	params, err := json.Marshal(productFailureLabRequest{Scenario: "auth"})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "incident-failure-lab-temporary-accounts",
		Method: "provider_failure_lab_run", Params: params,
	})
	if !response.OK || response.Error != nil {
		t.Fatalf("response=%#v", response)
	}
	var result productFailureLabResult
	if err := json.Unmarshal(response.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Agents) != 2 ||
		!strings.HasPrefix(result.Agents[0].ProviderAccountID, "failurelab_tmp_") ||
		!strings.HasPrefix(result.Agents[1].ProviderAccountID, "failurelab_tmp_") ||
		result.Agents[0].ProviderAccountID == result.Agents[1].ProviderAccountID {
		t.Fatalf("result=%#v", result)
	}
}

func TestProductFailureLabRouteRejectsProductionAccountHints(t *testing.T) {
	handler := newProductRouteHandler(productRouteServices{
		failureLab: failurelab.NewRunner(failurelab.NewTransport()),
	})
	params, err := json.Marshal(productFailureLabRequest{
		Scenario: "auth", ProviderAccountID: "deepseek.production",
		HealthyPeerAccountID: "minimax.production",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "incident-failure-lab-production-account",
		Method: "provider_failure_lab_run", Params: params,
	})
	if response.OK || response.Error == nil || response.Error.Code != "invalid_request" {
		t.Fatalf("response=%#v", response)
	}
}

func TestProductFailureLabRejectsUnknownScenarioAndSharedAccount(t *testing.T) {
	for _, request := range []productFailureLabRequest{
		{Scenario: "unknown", ProviderAccountID: "one", HealthyPeerAccountID: "two"},
		{Scenario: "auth", ProviderAccountID: "same", HealthyPeerAccountID: "same"},
		{Scenario: "auth", ProviderAccountID: "deepseek.production", HealthyPeerAccountID: "minimax.production"},
	} {
		if _, err := runProductFailureLab(
			context.Background(), failurelab.NewRunner(failurelab.NewTransport()),
			"incident-invalid", request,
		); err == nil {
			t.Fatalf("request=%#v accepted", request)
		}
	}
}
