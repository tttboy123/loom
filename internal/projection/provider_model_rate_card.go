package projection

import (
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

type ProviderModelRateCardRecord struct {
	RateCard    work.ProviderModelRateCard
	LastEventID string
}

type ProjectedProviderModelRateCard struct {
	Version                        int
	ProviderID                     string
	ProviderAccountID              string
	ModelID                        string
	Revision                       int64
	Currency                       string
	InputTokenBasis                string
	InputMicrounitsPerMillion      int64
	OutputMicrounitsPerMillion     int64
	CacheReadMicrounitsPerMillion  int64
	CacheWriteMicrounitsPerMillion int64
	RoundingMode                   string
	ConfiguredAt                   time.Time
	Digest                         string
}

func projectedProviderModelRateCard(
	rateCard work.ProviderModelRateCard,
) ProjectedProviderModelRateCard {
	return ProjectedProviderModelRateCard{
		Version: rateCard.Version(), ProviderID: rateCard.ProviderID(),
		ProviderAccountID: rateCard.ProviderAccountID(), ModelID: rateCard.ModelID(),
		Revision: rateCard.Revision(), Currency: rateCard.Currency(),
		InputTokenBasis:                rateCard.InputTokenBasis(),
		InputMicrounitsPerMillion:      rateCard.InputMicrounitsPerMillion(),
		OutputMicrounitsPerMillion:     rateCard.OutputMicrounitsPerMillion(),
		CacheReadMicrounitsPerMillion:  rateCard.CacheReadMicrounitsPerMillion(),
		CacheWriteMicrounitsPerMillion: rateCard.CacheWriteMicrounitsPerMillion(),
		RoundingMode:                   rateCard.RoundingMode(), ConfiguredAt: rateCard.ConfiguredAt(),
		Digest: rateCard.Digest(),
	}
}

func applyProviderModelRateCardProjection(
	snapshot *Snapshot,
	event journal.Event,
) error {
	if snapshot == nil || event.Type != "ProviderModelRateCardConfigured" {
		return fmt.Errorf("%w: Provider model Rate Card snapshot", ErrInvalidProjectionEvent)
	}
	if snapshot.ProviderModelRateCards == nil {
		snapshot.ProviderModelRateCards = make(map[string]ProviderModelRateCardRecord)
	}
	identity := event.StreamID
	existing, found := snapshot.ProviderModelRateCards[identity]
	previousEventID := ""
	if found {
		previousEventID = existing.LastEventID
	}
	rateCard, _, err := work.DecodeProviderModelRateCardConfiguredEvent(event, previousEventID)
	if err != nil || !rateCard.Valid() ||
		(found && (rateCard.ProviderID() != existing.RateCard.ProviderID() ||
			rateCard.ProviderAccountID() != existing.RateCard.ProviderAccountID() ||
			rateCard.ModelID() != existing.RateCard.ModelID() ||
			rateCard.Revision() != existing.RateCard.Revision()+1 ||
			!rateCard.ConfiguredAt().After(existing.RateCard.ConfiguredAt()))) ||
		(!found && rateCard.Revision() != 1) {
		return fmt.Errorf("%w: Provider model Rate Card fact", ErrInvalidProjectionEvent)
	}
	snapshot.ProviderModelRateCards[identity] = ProviderModelRateCardRecord{
		RateCard: rateCard, LastEventID: event.ID,
	}
	return nil
}
