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

type providerModelRateCardWireBackend struct {
	command app.ProviderModelRateCardCommand
	result  app.ProviderModelRateCardResult
}

func (backend *providerModelRateCardWireBackend) SetupSnapshot(
	context.Context,
) (app.SetupSnapshot, error) {
	return app.SetupSnapshot{}, nil
}

func (backend *providerModelRateCardWireBackend) StartBuilder(
	context.Context,
	app.BuilderStartCommand,
) (app.BuilderSessionView, error) {
	return app.BuilderSessionView{}, nil
}

func (backend *providerModelRateCardWireBackend) ConfigureProviderModelRateCard(
	_ context.Context,
	command app.ProviderModelRateCardCommand,
) (app.ProviderModelRateCardResult, error) {
	backend.command = command
	return backend.result, nil
}

func TestProviderModelRateCardConfigureWireInjectsTrustedCorrelation(t *testing.T) {
	backend := &providerModelRateCardWireBackend{
		result: app.ProviderModelRateCardResult{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			ModelID: "deepseek-chat", Revision: 2,
			RateCardDigest: strings.Repeat("b", 64), Currency: "USD",
			InputTokenBasis:           "input_includes_cache",
			InputMicrounitsPerMillion: 270_000, OutputMicrounitsPerMillion: 1_100_000,
			CacheReadMicrounitsPerMillion: 70_000,
			RoundingMode:                  "ceiling_per_attempt", ConfiguredAt: "2026-08-12T10:00:00Z",
		},
	}
	setup, err := api.NewLocalProductSetupAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	handler := localProductHandler(nil, setup)
	request := localipc.Request{
		Version: 1, RequestID: "loom-rate-card-wire-incident-1",
		Method: "provider_model_rate_card_configure",
		Params: json.RawMessage(`{
          "provider_id":"deepseek",
          "provider_account_id":"deepseek.work",
          "model_id":"deepseek-chat",
          "expected_revision":1,
          "currency":"USD",
          "input_token_basis":"input_includes_cache",
          "input_microunits_per_million":270000,
          "output_microunits_per_million":1100000,
          "cache_read_microunits_per_million":70000,
          "cache_write_microunits_per_million":0,
          "rounding_mode":"ceiling_per_attempt",
          "operation_id":"configure-deepseek-work-chat-v2"
        }`),
	}
	response := handler(context.Background(), request)
	if !response.OK || response.Error != nil {
		t.Fatalf("Rate Card response = %#v", response)
	}
	if backend.command.CorrelationID != request.RequestID ||
		backend.command.ProviderID != "deepseek" ||
		backend.command.ProviderAccountID != "deepseek.work" ||
		backend.command.ModelID != "deepseek-chat" ||
		backend.command.ExpectedRevision != 1 {
		t.Fatalf("Rate Card command = %#v", backend.command)
	}
	var result app.ProviderModelRateCardResult
	if err := json.Unmarshal(response.Result, &result); err != nil || result != backend.result {
		t.Fatalf("wire result = %#v, err=%v", result, err)
	}

	request.RequestID = "loom-rate-card-wire-incident-2"
	request.Params = json.RawMessage(`{
      "provider_id":"deepseek",
      "provider_account_id":"deepseek.work",
      "model_id":"deepseek-chat",
      "expected_revision":1,
      "currency":"USD",
      "input_token_basis":"input_includes_cache",
      "input_microunits_per_million":270000,
      "output_microunits_per_million":1100000,
      "cache_read_microunits_per_million":70000,
      "cache_write_microunits_per_million":0,
      "rounding_mode":"ceiling_per_attempt",
      "operation_id":"configure-deepseek-work-chat-v2",
      "secret":"must-not-be-accepted"
    }`)
	rejected := handler(context.Background(), request)
	if rejected.OK || rejected.Error == nil || rejected.Error.Code != "invalid_request" {
		t.Fatalf("unknown Rate Card field response = %#v", rejected)
	}
}
