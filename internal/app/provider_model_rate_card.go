package app

import (
	"context"
	"errors"

	"loom-pi-rebuild/internal/work"
)

var ErrProviderModelRateCardUnavailable = errors.New(
	"Provider model Rate Card unavailable",
)

type ProviderModelRateCardAuthority interface {
	ConfigureProviderModelRateCard(
		context.Context,
		work.ProviderModelRateCardCommand,
	) (work.ProviderModelRateCard, error)
}

type ProviderModelRateCardCommand struct {
	ProviderID                     string `json:"provider_id"`
	ProviderAccountID              string `json:"provider_account_id"`
	ModelID                        string `json:"model_id"`
	ExpectedRevision               int64  `json:"expected_revision"`
	Currency                       string `json:"currency"`
	InputTokenBasis                string `json:"input_token_basis"`
	InputMicrounitsPerMillion      int64  `json:"input_microunits_per_million"`
	OutputMicrounitsPerMillion     int64  `json:"output_microunits_per_million"`
	CacheReadMicrounitsPerMillion  int64  `json:"cache_read_microunits_per_million"`
	CacheWriteMicrounitsPerMillion int64  `json:"cache_write_microunits_per_million"`
	RoundingMode                   string `json:"rounding_mode"`
	OperationID                    string `json:"operation_id"`
	CorrelationID                  string `json:"-"`
}

type ProviderModelRateCardResult struct {
	ProviderID                     string `json:"provider_id"`
	ProviderAccountID              string `json:"provider_account_id"`
	ModelID                        string `json:"model_id"`
	Revision                       int64  `json:"revision"`
	RateCardDigest                 string `json:"rate_card_digest"`
	Currency                       string `json:"currency"`
	InputTokenBasis                string `json:"input_token_basis"`
	InputMicrounitsPerMillion      int64  `json:"input_microunits_per_million"`
	OutputMicrounitsPerMillion     int64  `json:"output_microunits_per_million"`
	CacheReadMicrounitsPerMillion  int64  `json:"cache_read_microunits_per_million"`
	CacheWriteMicrounitsPerMillion int64  `json:"cache_write_microunits_per_million"`
	RoundingMode                   string `json:"rounding_mode"`
	ConfiguredAt                   string `json:"configured_at"`
}

func (service *LocalProductSetupService) ConfigureProviderModelRateCard(
	ctx context.Context,
	command ProviderModelRateCardCommand,
) (ProviderModelRateCardResult, error) {
	if service == nil || ctx == nil || service.providerModelRateCards == nil {
		return ProviderModelRateCardResult{}, ErrProviderModelRateCardUnavailable
	}
	rateCard, err := service.providerModelRateCards.ConfigureProviderModelRateCard(
		ctx,
		work.ProviderModelRateCardCommand{
			CommandID: command.OperationID, ProviderID: command.ProviderID,
			ProviderAccountID: command.ProviderAccountID, ModelID: command.ModelID,
			ExpectedRevision: command.ExpectedRevision, Currency: command.Currency,
			InputTokenBasis:                command.InputTokenBasis,
			InputMicrounitsPerMillion:      command.InputMicrounitsPerMillion,
			OutputMicrounitsPerMillion:     command.OutputMicrounitsPerMillion,
			CacheReadMicrounitsPerMillion:  command.CacheReadMicrounitsPerMillion,
			CacheWriteMicrounitsPerMillion: command.CacheWriteMicrounitsPerMillion,
			RoundingMode:                   command.RoundingMode, CorrelationID: command.CorrelationID,
		},
	)
	if err != nil {
		return ProviderModelRateCardResult{}, err
	}
	if err := service.projection.Rebuild(ctx); err != nil {
		return ProviderModelRateCardResult{}, ErrProviderModelRateCardUnavailable
	}
	var projected work.ProviderModelRateCard
	found := false
	for _, candidate := range service.projection.GlobalReadView().ProviderModelRateCards(
		rateCard.ProviderID(), rateCard.ProviderAccountID(),
	) {
		if candidate.ModelID() == rateCard.ModelID() {
			projected, found = candidate, true
			break
		}
	}
	if !found || projected.Revision() != rateCard.Revision() ||
		projected.Digest() != rateCard.Digest() {
		return ProviderModelRateCardResult{}, ErrProviderModelRateCardUnavailable
	}
	return providerModelRateCardResult(rateCard), nil
}

func providerModelRateCardResult(
	rateCard work.ProviderModelRateCard,
) ProviderModelRateCardResult {
	return ProviderModelRateCardResult{
		ProviderID: rateCard.ProviderID(), ProviderAccountID: rateCard.ProviderAccountID(),
		ModelID: rateCard.ModelID(), Revision: rateCard.Revision(),
		RateCardDigest: rateCard.Digest(), Currency: rateCard.Currency(),
		InputTokenBasis:                rateCard.InputTokenBasis(),
		InputMicrounitsPerMillion:      rateCard.InputMicrounitsPerMillion(),
		OutputMicrounitsPerMillion:     rateCard.OutputMicrounitsPerMillion(),
		CacheReadMicrounitsPerMillion:  rateCard.CacheReadMicrounitsPerMillion(),
		CacheWriteMicrounitsPerMillion: rateCard.CacheWriteMicrounitsPerMillion(),
		RoundingMode:                   rateCard.RoundingMode(),
		ConfiguredAt:                   rateCard.ConfiguredAt().Format("2006-01-02T15:04:05.999999999Z"),
	}
}

func providerModelRateCardDirectoryEntry(
	rateCard work.ProviderModelRateCard,
) ProviderModelRateCardDirectoryEntry {
	result := providerModelRateCardResult(rateCard)
	return ProviderModelRateCardDirectoryEntry{
		ModelID: result.ModelID, Revision: result.Revision,
		RateCardDigest: result.RateCardDigest, Currency: result.Currency,
		InputTokenBasis:                result.InputTokenBasis,
		InputMicrounitsPerMillion:      result.InputMicrounitsPerMillion,
		OutputMicrounitsPerMillion:     result.OutputMicrounitsPerMillion,
		CacheReadMicrounitsPerMillion:  result.CacheReadMicrounitsPerMillion,
		CacheWriteMicrounitsPerMillion: result.CacheWriteMicrounitsPerMillion,
		RoundingMode:                   result.RoundingMode, ConfiguredAt: result.ConfiguredAt,
	}
}
