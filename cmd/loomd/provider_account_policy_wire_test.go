package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/localipc"
)

type providerAccountPolicyWireBackend struct {
	command app.ProviderAccountPolicyCommand
	result  app.ProviderAccountPolicyResult
}

func (backend *providerAccountPolicyWireBackend) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return app.SetupSnapshot{}, nil
}

func (backend *providerAccountPolicyWireBackend) StartBuilder(
	context.Context,
	app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	return app.BuilderSessionView{}, nil
}

func (backend *providerAccountPolicyWireBackend) ConfigureProviderAccountPolicy(
	_ context.Context,
	command app.ProviderAccountPolicyCommand,
) (app.ProviderAccountPolicyResult, error) {
	backend.command = command
	return backend.result, nil
}

func TestProviderAccountPolicyConfigureWireInjectsTrustedCorrelation(t *testing.T) {
	backend := &providerAccountPolicyWireBackend{
		result: app.ProviderAccountPolicyResult{
			PolicyAvailable: true, PolicyVersion: 2, ProviderID: "deepseek",
			ProviderAccountID: "deepseek.work", Revision: 2,
			PolicyDigest:              strings.Repeat("a", 64),
			MaximumConcurrentAttempts: 3, DispatchWindowSeconds: 60,
			MaximumDispatchStarts: 12, MaximumAssignedBudgetUnits: 8_000,
			TrustDomain: "external_provider", RetentionMode: "zero_data_retention",
			DataRegion:   "apac",
			ConfiguredAt: "2026-08-11T10:00:00Z",
		},
	}
	setup, err := api.NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandler(nil, setup)
	request := localipc.Request{
		Version: 1, RequestID: "loom-policy-wire-incident-1",
		Method: "provider_account_policy_configure",
		Params: json.RawMessage(`{
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "expected_revision":1,
          "maximum_concurrent_attempts":3,
          "dispatch_window_seconds":60,
          "maximum_dispatch_starts":12,
          "maximum_assigned_budget_units":8000,
          "trust_domain":"external_provider",
          "retention_mode":"zero_data_retention",
          "data_region":"apac",
          "operation_id":"configure-deepseek-work-v2"
        }`),
	}
	response := handler(context.Background(), request)
	if !response.OK || response.Error != nil {
		t.Fatalf("policy response = %#v", response)
	}
	if backend.command.CorrelationID != request.RequestID ||
		backend.command.ProviderID != "deepseek" ||
		backend.command.ProviderAccountID != "deepseek.work" ||
		backend.command.ExpectedRevision != 1 ||
		backend.command.OperationID != "configure-deepseek-work-v2" {
		t.Fatalf("policy command = %#v", backend.command)
	}
	if backend.command.TrustDomain != "external_provider" ||
		backend.command.RetentionMode != "zero_data_retention" ||
		backend.command.DataRegion != "apac" {
		t.Fatalf("policy disclosure command = %#v", backend.command)
	}
	var result app.ProviderAccountPolicyResult
	if err := json.Unmarshal(response.Result, &result); err != nil ||
		result != backend.result {
		t.Fatalf("wire result = %#v, err=%v", result, err)
	}

	request.RequestID = "loom-policy-wire-incident-2"
	request.Params = json.RawMessage(`{
      "provider_id":"deepseek",
      "provider_account_id":"deepseek.work",
      "expected_revision":1,
      "maximum_concurrent_attempts":3,
      "dispatch_window_seconds":60,
      "maximum_dispatch_starts":12,
      "maximum_assigned_budget_units":8000,
      "trust_domain":"external_provider",
      "retention_mode":"zero_data_retention",
      "data_region":"apac",
      "operation_id":"configure-deepseek-work-v2",
      "secret":"must-not-be-accepted"
    }`)
	rejected := handler(context.Background(), request)
	if rejected.OK || rejected.Error == nil || rejected.Error.Code != "invalid_request" {
		t.Fatalf("unknown policy field response = %#v", rejected)
	}
}
