package work

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidProviderModelRateCard         = errors.New("invalid Provider model Rate Card")
	ErrProviderModelRateCardConflict        = errors.New("Provider model Rate Card conflict")
	ErrProviderModelRateCardNotFound        = errors.New("Provider model Rate Card not found")
	ErrInvalidProviderModelRateCardEstimate = errors.New("invalid Provider model Rate Card estimate")
)

const (
	providerModelRateCardVersion      = 1
	maximumRateMicrounitsPerMillion   = int64(1_000_000_000_000)
	rateCardTokenScale                = int64(1_000_000)
	RateCardInputIncludesCache        = "input_includes_cache"
	RateCardInputExcludesCache        = "input_excludes_cache"
	RateCardRoundingCeilingPerAttempt = "ceiling_per_attempt"
	CostSourceRateCardEstimate        = "rate_card_estimate"
	frozenRateCardConfigured          = "configured"
	frozenRateCardNotConfigured       = "not_configured"
)

type ProviderModelRateCardCommand struct {
	CommandID                      string
	ProviderID                     string
	ProviderAccountID              string
	ModelID                        string
	ExpectedRevision               int64
	Currency                       string
	InputTokenBasis                string
	InputMicrounitsPerMillion      int64
	OutputMicrounitsPerMillion     int64
	CacheReadMicrounitsPerMillion  int64
	CacheWriteMicrounitsPerMillion int64
	RoundingMode                   string
	CorrelationID                  string
}

type ProviderModelRateCardInput struct {
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
}

type ProviderModelRateCard struct {
	version                        int
	providerID                     string
	providerAccountID              string
	modelID                        string
	revision                       int64
	currency                       string
	inputTokenBasis                string
	inputMicrounitsPerMillion      int64
	outputMicrounitsPerMillion     int64
	cacheReadMicrounitsPerMillion  int64
	cacheWriteMicrounitsPerMillion int64
	roundingMode                   string
	configuredAt                   time.Time
	digest                         string
}

type providerModelRateCardState struct {
	rateCard  ProviderModelRateCard
	commandID string
	eventID   string
}

type providerModelRateCardPayload struct {
	CommandID                      string `json:"command_id"`
	RateCardVersion                int    `json:"rate_card_version"`
	ProviderID                     string `json:"provider_id"`
	ProviderAccountID              string `json:"provider_account_id"`
	ModelID                        string `json:"model_id"`
	Revision                       int64  `json:"revision"`
	Currency                       string `json:"currency"`
	InputTokenBasis                string `json:"input_token_basis"`
	InputMicrounitsPerMillion      int64  `json:"input_microunits_per_million"`
	OutputMicrounitsPerMillion     int64  `json:"output_microunits_per_million"`
	CacheReadMicrounitsPerMillion  int64  `json:"cache_read_microunits_per_million"`
	CacheWriteMicrounitsPerMillion int64  `json:"cache_write_microunits_per_million"`
	RoundingMode                   string `json:"rounding_mode"`
	ConfiguredAt                   string `json:"configured_at"`
	RateCardDigest                 string `json:"rate_card_digest"`
}

type frozenProviderModelRateCardPayload struct {
	RateCardVersion                int    `json:"rate_card_version"`
	ProviderID                     string `json:"provider_id"`
	ProviderAccountID              string `json:"provider_account_id"`
	ModelID                        string `json:"model_id"`
	Revision                       int64  `json:"revision"`
	Currency                       string `json:"currency"`
	InputTokenBasis                string `json:"input_token_basis"`
	InputMicrounitsPerMillion      int64  `json:"input_microunits_per_million"`
	OutputMicrounitsPerMillion     int64  `json:"output_microunits_per_million"`
	CacheReadMicrounitsPerMillion  int64  `json:"cache_read_microunits_per_million"`
	CacheWriteMicrounitsPerMillion int64  `json:"cache_write_microunits_per_million"`
	RoundingMode                   string `json:"rounding_mode"`
	ConfiguredAt                   string `json:"configured_at"`
	RateCardDigest                 string `json:"rate_card_digest"`
}

func (rateCard ProviderModelRateCard) Version() int              { return rateCard.version }
func (rateCard ProviderModelRateCard) ProviderID() string        { return rateCard.providerID }
func (rateCard ProviderModelRateCard) ProviderAccountID() string { return rateCard.providerAccountID }
func (rateCard ProviderModelRateCard) ModelID() string           { return rateCard.modelID }
func (rateCard ProviderModelRateCard) Revision() int64           { return rateCard.revision }
func (rateCard ProviderModelRateCard) Currency() string          { return rateCard.currency }
func (rateCard ProviderModelRateCard) InputTokenBasis() string   { return rateCard.inputTokenBasis }
func (rateCard ProviderModelRateCard) InputMicrounitsPerMillion() int64 {
	return rateCard.inputMicrounitsPerMillion
}
func (rateCard ProviderModelRateCard) OutputMicrounitsPerMillion() int64 {
	return rateCard.outputMicrounitsPerMillion
}
func (rateCard ProviderModelRateCard) CacheReadMicrounitsPerMillion() int64 {
	return rateCard.cacheReadMicrounitsPerMillion
}
func (rateCard ProviderModelRateCard) CacheWriteMicrounitsPerMillion() int64 {
	return rateCard.cacheWriteMicrounitsPerMillion
}
func (rateCard ProviderModelRateCard) RoundingMode() string    { return rateCard.roundingMode }
func (rateCard ProviderModelRateCard) ConfiguredAt() time.Time { return rateCard.configuredAt }
func (rateCard ProviderModelRateCard) Digest() string          { return rateCard.digest }

func NewProviderModelRateCard(input ProviderModelRateCardInput) (ProviderModelRateCard, error) {
	rateCard := ProviderModelRateCard{
		version: input.Version, providerID: input.ProviderID,
		providerAccountID: input.ProviderAccountID, modelID: input.ModelID,
		revision: input.Revision, currency: input.Currency,
		inputTokenBasis:                input.InputTokenBasis,
		inputMicrounitsPerMillion:      input.InputMicrounitsPerMillion,
		outputMicrounitsPerMillion:     input.OutputMicrounitsPerMillion,
		cacheReadMicrounitsPerMillion:  input.CacheReadMicrounitsPerMillion,
		cacheWriteMicrounitsPerMillion: input.CacheWriteMicrounitsPerMillion,
		roundingMode:                   input.RoundingMode, configuredAt: input.ConfiguredAt,
	}
	if !validProviderModelRateCardShape(rateCard) {
		return ProviderModelRateCard{}, ErrInvalidProviderModelRateCard
	}
	rateCard.digest = providerModelRateCardDigest(rateCard)
	return rateCard, nil
}

func (rateCard ProviderModelRateCard) Valid() bool {
	return validProviderModelRateCardShape(rateCard) &&
		rateCard.digest == providerModelRateCardDigest(rateCard)
}

func (authority *Authority) ConfigureProviderModelRateCard(
	ctx context.Context,
	command ProviderModelRateCardCommand,
) (ProviderModelRateCard, error) {
	if authority == nil || ctx == nil || !validProviderModelRateCardCommand(command) {
		return ProviderModelRateCard{}, ErrInvalidProviderModelRateCard
	}
	if err := ctx.Err(); err != nil {
		return ProviderModelRateCard{}, err
	}
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return ProviderModelRateCard{}, ErrInvalidProviderModelRateCard
	}
	streamID := providerModelRateCardStream(command.ProviderID, command.ProviderAccountID, command.ModelID)
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return ProviderModelRateCard{}, err
	}
	current, found, err := replayProviderModelRateCard(
		command.ProviderID, command.ProviderAccountID, command.ModelID, events,
	)
	if err != nil {
		return ProviderModelRateCard{}, err
	}
	if found && current.commandID == command.CommandID {
		if current.rateCard.revision == command.ExpectedRevision+1 &&
			providerModelRateCardMatchesCommand(current.rateCard, command) {
			return current.rateCard, nil
		}
		return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
	}
	actualRevision := int64(0)
	causationID := ""
	if found {
		actualRevision = current.rateCard.revision
		causationID = current.eventID
		if !now.After(current.rateCard.configuredAt) {
			return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
		}
	}
	if actualRevision != command.ExpectedRevision {
		return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
	}
	rateCard, err := NewProviderModelRateCard(ProviderModelRateCardInput{
		Version: providerModelRateCardVersion, ProviderID: command.ProviderID,
		ProviderAccountID: command.ProviderAccountID, ModelID: command.ModelID,
		Revision: actualRevision + 1, Currency: command.Currency,
		InputTokenBasis:                command.InputTokenBasis,
		InputMicrounitsPerMillion:      command.InputMicrounitsPerMillion,
		OutputMicrounitsPerMillion:     command.OutputMicrounitsPerMillion,
		CacheReadMicrounitsPerMillion:  command.CacheReadMicrounitsPerMillion,
		CacheWriteMicrounitsPerMillion: command.CacheWriteMicrounitsPerMillion,
		RoundingMode:                   command.RoundingMode, ConfiguredAt: now,
	})
	if err != nil {
		return ProviderModelRateCard{}, err
	}
	eventID := deterministicEventID(
		"ProviderModelRateCardConfigured", streamID,
		strconv.FormatInt(rateCard.revision, 10), command.CommandID, rateCard.digest,
	)
	event := newEvent(
		eventID, streamID, rateCard.revision, "ProviderModelRateCardConfigured",
		now, command.CorrelationID, causationID,
		providerModelRateCardPayloadFrom(rateCard, command.CommandID),
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{StreamID: streamID, Sequence: command.ExpectedRevision}},
		[]journal.Event{event},
	); err != nil {
		return ProviderModelRateCard{}, mapProviderModelRateCardWriteError(err)
	}
	return rateCard, nil
}

func (authority *Authority) ProviderModelRateCard(
	ctx context.Context,
	providerID, providerAccountID, modelID string,
) (ProviderModelRateCard, error) {
	if authority == nil || ctx == nil ||
		!validProviderModelRateCardIdentity(providerID, providerAccountID, modelID) {
		return ProviderModelRateCard{}, ErrInvalidProviderModelRateCard
	}
	if err := ctx.Err(); err != nil {
		return ProviderModelRateCard{}, err
	}
	events, err := authority.store.ReadStream(
		ctx, providerModelRateCardStream(providerID, providerAccountID, modelID),
	)
	if err != nil {
		return ProviderModelRateCard{}, err
	}
	current, found, err := replayProviderModelRateCard(providerID, providerAccountID, modelID, events)
	if err != nil {
		return ProviderModelRateCard{}, err
	}
	if !found {
		return ProviderModelRateCard{}, ErrProviderModelRateCardNotFound
	}
	return current.rateCard, nil
}

func DecodeProviderModelRateCardConfiguredEvent(
	event journal.Event,
	previousEventID string,
) (ProviderModelRateCard, string, error) {
	if event.SchemaVersion != 1 || event.Type != "ProviderModelRateCardConfigured" ||
		!validOpaqueID(event.CorrelationID) || event.Seq <= 0 ||
		!strings.HasPrefix(event.StreamID, "provider-model-rate-card/") {
		return ProviderModelRateCard{}, "", ErrProviderModelRateCardConflict
	}
	var payload providerModelRateCardPayload
	if decodeExactPayload(event.PayloadJSON, &payload) != nil {
		return ProviderModelRateCard{}, "", ErrProviderModelRateCardConflict
	}
	configuredAt, err := parseUTC(payload.ConfiguredAt)
	if err != nil || !configuredAt.Equal(event.EmittedAt) ||
		payload.Revision != event.Seq || !validOpaqueID(payload.CommandID) ||
		providerModelRateCardStream(payload.ProviderID, payload.ProviderAccountID, payload.ModelID) != event.StreamID ||
		(event.Seq == 1 && (previousEventID != "" || event.CausationID != "")) ||
		(event.Seq > 1 && event.CausationID != previousEventID) {
		return ProviderModelRateCard{}, "", ErrProviderModelRateCardConflict
	}
	rateCard, err := NewProviderModelRateCard(ProviderModelRateCardInput{
		Version: payload.RateCardVersion, ProviderID: payload.ProviderID,
		ProviderAccountID: payload.ProviderAccountID, ModelID: payload.ModelID,
		Revision: payload.Revision, Currency: payload.Currency,
		InputTokenBasis:                payload.InputTokenBasis,
		InputMicrounitsPerMillion:      payload.InputMicrounitsPerMillion,
		OutputMicrounitsPerMillion:     payload.OutputMicrounitsPerMillion,
		CacheReadMicrounitsPerMillion:  payload.CacheReadMicrounitsPerMillion,
		CacheWriteMicrounitsPerMillion: payload.CacheWriteMicrounitsPerMillion,
		RoundingMode:                   payload.RoundingMode, ConfiguredAt: configuredAt,
	})
	if err != nil || rateCard.digest != payload.RateCardDigest {
		return ProviderModelRateCard{}, "", ErrProviderModelRateCardConflict
	}
	expectedID := deterministicEventID(
		"ProviderModelRateCardConfigured", event.StreamID,
		strconv.FormatInt(rateCard.revision, 10), payload.CommandID, rateCard.digest,
	)
	if event.ID != expectedID || event.IdempotencyKey != expectedID {
		return ProviderModelRateCard{}, "", ErrProviderModelRateCardConflict
	}
	return rateCard, payload.CommandID, nil
}

func EstimateRunAccounting(
	accounting RunAccounting,
	rateCard ProviderModelRateCard,
) (RunAccounting, error) {
	if !rateCard.Valid() || !validRunUsageAccounting(accounting) {
		return RunAccounting{}, ErrInvalidProviderModelRateCardEstimate
	}
	if accounting.CostObserved {
		if !validRunAccounting(accounting) || accounting.CostSource == CostSourceRateCardEstimate {
			return RunAccounting{}, ErrInvalidProviderModelRateCardEstimate
		}
		return accounting, nil
	}
	if accounting.CostMicrounits != 0 || accounting.CostCurrency != "" || accounting.CostSource != "" {
		return RunAccounting{}, ErrInvalidProviderModelRateCardEstimate
	}
	if !accounting.UsageObserved {
		return accounting, nil
	}
	inputTokens := accounting.InputTokens
	if rateCard.inputTokenBasis == RateCardInputIncludesCache {
		if accounting.CacheReadTokens > inputTokens {
			return RunAccounting{}, ErrInvalidProviderModelRateCardEstimate
		}
		inputTokens -= accounting.CacheReadTokens
	}
	pairs := [][2]int64{
		{inputTokens, rateCard.inputMicrounitsPerMillion},
		{accounting.OutputTokens, rateCard.outputMicrounitsPerMillion},
		{accounting.CacheReadTokens, rateCard.cacheReadMicrounitsPerMillion},
		{accounting.CacheWriteTokens, rateCard.cacheWriteMicrounitsPerMillion},
	}
	numerator := new(big.Int)
	for _, pair := range pairs {
		term := new(big.Int).Mul(big.NewInt(pair[0]), big.NewInt(pair[1]))
		numerator.Add(numerator, term)
	}
	if numerator.Sign() < 0 {
		return RunAccounting{}, ErrInvalidProviderModelRateCardEstimate
	}
	divisor := big.NewInt(rateCardTokenScale)
	numerator.Add(numerator, new(big.Int).Sub(divisor, big.NewInt(1)))
	numerator.Div(numerator, divisor)
	if !numerator.IsInt64() {
		return RunAccounting{}, ErrInvalidProviderModelRateCardEstimate
	}
	estimated := accounting
	estimated.CostObserved = true
	estimated.CostMicrounits = numerator.Int64()
	estimated.CostCurrency = rateCard.currency
	estimated.CostSource = CostSourceRateCardEstimate
	return estimated, nil
}

func frozenProviderModelRateCardPayloadFrom(
	rateCard ProviderModelRateCard,
) frozenProviderModelRateCardPayload {
	return frozenProviderModelRateCardPayload{
		RateCardVersion: rateCard.version, ProviderID: rateCard.providerID,
		ProviderAccountID: rateCard.providerAccountID, ModelID: rateCard.modelID,
		Revision: rateCard.revision, Currency: rateCard.currency,
		InputTokenBasis:                rateCard.inputTokenBasis,
		InputMicrounitsPerMillion:      rateCard.inputMicrounitsPerMillion,
		OutputMicrounitsPerMillion:     rateCard.outputMicrounitsPerMillion,
		CacheReadMicrounitsPerMillion:  rateCard.cacheReadMicrounitsPerMillion,
		CacheWriteMicrounitsPerMillion: rateCard.cacheWriteMicrounitsPerMillion,
		RoundingMode:                   rateCard.roundingMode,
		ConfiguredAt:                   rateCard.configuredAt.Format(time.RFC3339Nano),
		RateCardDigest:                 rateCard.digest,
	}
}

func providerModelRateCardFromFrozenPayload(
	payload frozenProviderModelRateCardPayload,
) (ProviderModelRateCard, error) {
	configuredAt, err := parseUTC(payload.ConfiguredAt)
	if err != nil {
		return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
	}
	rateCard, err := NewProviderModelRateCard(ProviderModelRateCardInput{
		Version: payload.RateCardVersion, ProviderID: payload.ProviderID,
		ProviderAccountID: payload.ProviderAccountID, ModelID: payload.ModelID,
		Revision: payload.Revision, Currency: payload.Currency,
		InputTokenBasis:                payload.InputTokenBasis,
		InputMicrounitsPerMillion:      payload.InputMicrounitsPerMillion,
		OutputMicrounitsPerMillion:     payload.OutputMicrounitsPerMillion,
		CacheReadMicrounitsPerMillion:  payload.CacheReadMicrounitsPerMillion,
		CacheWriteMicrounitsPerMillion: payload.CacheWriteMicrounitsPerMillion,
		RoundingMode:                   payload.RoundingMode, ConfiguredAt: configuredAt,
	})
	if err != nil || rateCard.digest != payload.RateCardDigest {
		return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
	}
	return rateCard, nil
}

// DecodeFrozenProviderModelRateCard validates the exact non-secret snapshot
// carried by a v2 Run claim. Callers must separately validate claim status.
func DecodeFrozenProviderModelRateCard(
	payload []byte,
) (ProviderModelRateCard, error) {
	if len(payload) == 0 || bytes.Equal(bytes.TrimSpace(payload), []byte("null")) ||
		hasDuplicateJSONKeys(payload) {
		return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
	}
	var frozen frozenProviderModelRateCardPayload
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&frozen) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return ProviderModelRateCard{}, ErrProviderModelRateCardConflict
	}
	return providerModelRateCardFromFrozenPayload(frozen)
}

func replayProviderModelRateCard(
	providerID, providerAccountID, modelID string,
	events []journal.Event,
) (providerModelRateCardState, bool, error) {
	if !validProviderModelRateCardIdentity(providerID, providerAccountID, modelID) {
		return providerModelRateCardState{}, false, ErrInvalidProviderModelRateCard
	}
	streamID := providerModelRateCardStream(providerID, providerAccountID, modelID)
	var current providerModelRateCardState
	for index, event := range events {
		if event.StreamID != streamID || event.Seq != int64(index+1) {
			return providerModelRateCardState{}, false, ErrProviderModelRateCardConflict
		}
		candidate, commandID, err := DecodeProviderModelRateCardConfiguredEvent(event, current.eventID)
		if err != nil || candidate.providerID != providerID ||
			candidate.providerAccountID != providerAccountID || candidate.modelID != modelID {
			return providerModelRateCardState{}, false, ErrProviderModelRateCardConflict
		}
		current = providerModelRateCardState{rateCard: candidate, commandID: commandID, eventID: event.ID}
	}
	return current, len(events) > 0, nil
}

func replayProviderModelRateCardStreams(
	state *authorityState,
	byStream map[string][]journal.Event,
) error {
	for streamID, events := range byStream {
		if !strings.HasPrefix(streamID, "provider-model-rate-card/") {
			continue
		}
		var previousEventID string
		for _, event := range events {
			rateCard, commandID, err := DecodeProviderModelRateCardConfiguredEvent(event, previousEventID)
			if err != nil || providerModelRateCardStream(
				rateCard.ProviderID(), rateCard.ProviderAccountID(), rateCard.ModelID(),
			) != streamID {
				return ErrRunAuthorityConflict
			}
			identity := providerModelRateCardIdentityDigest(
				rateCard.ProviderID(), rateCard.ProviderAccountID(), rateCard.ModelID(),
			)
			history := state.providerModelRateCards[identity]
			if len(history) > 0 && !rateCard.ConfiguredAt().After(
				history[len(history)-1].rateCard.ConfiguredAt(),
			) {
				return ErrRunAuthorityConflict
			}
			state.providerModelRateCards[identity] = append(history, providerModelRateCardState{
				rateCard: rateCard, commandID: commandID, eventID: event.ID,
			})
			previousEventID = event.ID
		}
	}
	return nil
}

func (state authorityState) providerModelRateCardAt(
	providerID, accountID, modelID string,
	at time.Time,
) (ProviderModelRateCard, bool) {
	identity := providerModelRateCardIdentityDigest(providerID, accountID, modelID)
	history := state.providerModelRateCards[identity]
	for index := len(history) - 1; index >= 0; index-- {
		rateCard := history[index].rateCard
		if rateCard.ProviderID() == providerID &&
			rateCard.ProviderAccountID() == accountID &&
			rateCard.ModelID() == modelID && !rateCard.ConfiguredAt().After(at) {
			return rateCard, true
		}
	}
	return ProviderModelRateCard{}, false
}

func providerModelRateCardPayloadFrom(rateCard ProviderModelRateCard, commandID string) providerModelRateCardPayload {
	return providerModelRateCardPayload{
		CommandID: commandID, RateCardVersion: rateCard.version,
		ProviderID: rateCard.providerID, ProviderAccountID: rateCard.providerAccountID,
		ModelID: rateCard.modelID, Revision: rateCard.revision, Currency: rateCard.currency,
		InputTokenBasis:                rateCard.inputTokenBasis,
		InputMicrounitsPerMillion:      rateCard.inputMicrounitsPerMillion,
		OutputMicrounitsPerMillion:     rateCard.outputMicrounitsPerMillion,
		CacheReadMicrounitsPerMillion:  rateCard.cacheReadMicrounitsPerMillion,
		CacheWriteMicrounitsPerMillion: rateCard.cacheWriteMicrounitsPerMillion,
		RoundingMode:                   rateCard.roundingMode,
		ConfiguredAt:                   rateCard.configuredAt.Format(time.RFC3339Nano),
		RateCardDigest:                 rateCard.digest,
	}
}

func validProviderModelRateCardCommand(command ProviderModelRateCardCommand) bool {
	return validOpaqueID(command.CommandID) && command.ExpectedRevision >= 0 &&
		command.ExpectedRevision < int64(^uint32(0)) && validOpaqueID(command.CorrelationID) &&
		validProviderModelRateCardIdentity(command.ProviderID, command.ProviderAccountID, command.ModelID) &&
		validCurrency(command.Currency) && validRateCardTokenBasis(command.InputTokenBasis) &&
		validRateCardRates(command.InputMicrounitsPerMillion, command.OutputMicrounitsPerMillion,
			command.CacheReadMicrounitsPerMillion, command.CacheWriteMicrounitsPerMillion) &&
		command.RoundingMode == RateCardRoundingCeilingPerAttempt
}

func validProviderModelRateCardShape(rateCard ProviderModelRateCard) bool {
	return rateCard.version == providerModelRateCardVersion &&
		validProviderModelRateCardIdentity(rateCard.providerID, rateCard.providerAccountID, rateCard.modelID) &&
		rateCard.revision > 0 && validCurrency(rateCard.currency) &&
		validRateCardTokenBasis(rateCard.inputTokenBasis) &&
		validRateCardRates(rateCard.inputMicrounitsPerMillion, rateCard.outputMicrounitsPerMillion,
			rateCard.cacheReadMicrounitsPerMillion, rateCard.cacheWriteMicrounitsPerMillion) &&
		rateCard.roundingMode == RateCardRoundingCeilingPerAttempt &&
		!rateCard.configuredAt.IsZero() && rateCard.configuredAt.Location() == time.UTC
}

func validProviderModelRateCardIdentity(providerID, accountID, modelID string) bool {
	return credentials.ValidProviderAccountIdentifier(providerID, accountID) &&
		validProviderModelRateCardModelID(modelID)
}

func validProviderModelRateCardModelID(value string) bool {
	if value == "" || len(value) > 256 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validRateCardTokenBasis(value string) bool {
	return value == RateCardInputIncludesCache || value == RateCardInputExcludesCache
}

func validRateCardRates(rates ...int64) bool {
	for _, rate := range rates {
		if rate < 0 || rate > maximumRateMicrounitsPerMillion {
			return false
		}
	}
	return true
}

func providerModelRateCardMatchesCommand(rateCard ProviderModelRateCard, command ProviderModelRateCardCommand) bool {
	return rateCard.providerID == command.ProviderID && rateCard.providerAccountID == command.ProviderAccountID &&
		rateCard.modelID == command.ModelID && rateCard.currency == command.Currency &&
		rateCard.inputTokenBasis == command.InputTokenBasis &&
		rateCard.inputMicrounitsPerMillion == command.InputMicrounitsPerMillion &&
		rateCard.outputMicrounitsPerMillion == command.OutputMicrounitsPerMillion &&
		rateCard.cacheReadMicrounitsPerMillion == command.CacheReadMicrounitsPerMillion &&
		rateCard.cacheWriteMicrounitsPerMillion == command.CacheWriteMicrounitsPerMillion &&
		rateCard.roundingMode == command.RoundingMode
}

func providerModelRateCardDigest(rateCard ProviderModelRateCard) string {
	digest := sha256.New()
	for _, field := range []string{
		"loom.provider-model-rate-card.v1", strconv.Itoa(rateCard.version),
		rateCard.providerID, rateCard.providerAccountID, rateCard.modelID,
		strconv.FormatInt(rateCard.revision, 10), rateCard.currency, rateCard.inputTokenBasis,
		strconv.FormatInt(rateCard.inputMicrounitsPerMillion, 10),
		strconv.FormatInt(rateCard.outputMicrounitsPerMillion, 10),
		strconv.FormatInt(rateCard.cacheReadMicrounitsPerMillion, 10),
		strconv.FormatInt(rateCard.cacheWriteMicrounitsPerMillion, 10),
		rateCard.roundingMode, rateCard.configuredAt.Format(time.RFC3339Nano),
	} {
		writeProviderModelRateCardField(digest, field)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func writeProviderModelRateCardField(target hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = target.Write(length[:])
	_, _ = target.Write([]byte(value))
}

func providerModelRateCardIdentityDigest(providerID, accountID, modelID string) string {
	digest := sha256.New()
	for _, field := range []string{"loom.provider-model-rate-card.identity.v1", providerID, accountID, modelID} {
		writeProviderModelRateCardField(digest, field)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func providerModelRateCardStream(providerID, accountID, modelID string) string {
	return "provider-model-rate-card/" + providerModelRateCardIdentityDigest(providerID, accountID, modelID)
}

// ProviderModelRateCardStreamID exposes only the non-secret authority stream
// identity so callers can bind optimistic concurrency to the same Rate Card.
func ProviderModelRateCardStreamID(
	providerID, accountID, modelID string,
) (string, error) {
	if !validProviderModelRateCardIdentity(providerID, accountID, modelID) {
		return "", ErrInvalidProviderModelRateCard
	}
	return providerModelRateCardStream(providerID, accountID, modelID), nil
}

func mapProviderModelRateCardWriteError(err error) error {
	switch {
	case errors.Is(err, journal.ErrStreamHeadConflict),
		errors.Is(err, journal.ErrSequenceConflict),
		errors.Is(err, journal.ErrIdempotencyConflict),
		errors.Is(err, journal.ErrPartialEventBatchConflict):
		return fmt.Errorf("%w: %v", ErrProviderModelRateCardConflict, err)
	default:
		return err
	}
}
