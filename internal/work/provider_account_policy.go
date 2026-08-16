package work

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/journal"
)

var (
	ErrInvalidProviderAccountPolicy  = errors.New("invalid Provider Account policy")
	ErrProviderAccountPolicyConflict = errors.New("Provider Account policy conflict")
	ErrProviderAccountPolicyNotFound = errors.New("Provider Account policy not found")
)

const (
	providerAccountPolicyLegacyVersion   = 1
	providerAccountPolicyCurrentVersion  = 2
	maximumProviderAccountConcurrency    = 64
	maximumProviderAccountDispatchStarts = 1_000_000
	maximumProviderAccountDispatchWindow = 24 * time.Hour
)

type ProviderAccountPolicyCommand struct {
	CommandID                  string
	ProviderID                 string
	ProviderAccountID          string
	ExpectedRevision           int64
	MaximumConcurrentAttempts  int
	DispatchWindow             time.Duration
	MaximumDispatchStarts      int
	MaximumAssignedBudgetUnits int64
	TrustDomain                string
	RetentionMode              string
	DataRegion                 string
	CorrelationID              string
}

type ProviderAccountPolicyInput struct {
	Version                    int
	ProviderID                 string
	ProviderAccountID          string
	Revision                   int64
	MaximumConcurrentAttempts  int
	DispatchWindow             time.Duration
	MaximumDispatchStarts      int
	MaximumAssignedBudgetUnits int64
	TrustDomain                string
	RetentionMode              string
	DataRegion                 string
	ConfiguredAt               time.Time
}

type ProviderAccountPolicy struct {
	version                    int
	providerID                 string
	providerAccountID          string
	revision                   int64
	maximumConcurrentAttempts  int
	dispatchWindow             time.Duration
	maximumDispatchStarts      int
	maximumAssignedBudgetUnits int64
	trustDomain                string
	retentionMode              string
	dataRegion                 string
	configuredAt               time.Time
	digest                     string
}

type providerAccountPolicyState struct {
	policy    ProviderAccountPolicy
	commandID string
	eventID   string
}

type providerAccountPolicyPayload struct {
	CommandID                  string `json:"command_id"`
	PolicyVersion              int    `json:"policy_version"`
	ProviderID                 string `json:"provider_id"`
	ProviderAccountID          string `json:"provider_account_id"`
	Revision                   int64  `json:"revision"`
	MaximumConcurrentAttempts  int    `json:"maximum_concurrent_attempts"`
	DispatchWindowNanoseconds  int64  `json:"dispatch_window_nanoseconds"`
	MaximumDispatchStarts      int    `json:"maximum_dispatch_starts"`
	MaximumAssignedBudgetUnits int64  `json:"maximum_assigned_budget_units"`
	TrustDomain                string `json:"trust_domain,omitempty"`
	RetentionMode              string `json:"retention_mode,omitempty"`
	DataRegion                 string `json:"data_region,omitempty"`
	ConfiguredAt               string `json:"configured_at"`
	PolicyDigest               string `json:"policy_digest"`
}

func (policy ProviderAccountPolicy) Version() int       { return policy.version }
func (policy ProviderAccountPolicy) ProviderID() string { return policy.providerID }
func (policy ProviderAccountPolicy) ProviderAccountID() string {
	return policy.providerAccountID
}
func (policy ProviderAccountPolicy) Revision() int64 { return policy.revision }
func (policy ProviderAccountPolicy) MaximumConcurrentAttempts() int {
	return policy.maximumConcurrentAttempts
}
func (policy ProviderAccountPolicy) DispatchWindow() time.Duration {
	return policy.dispatchWindow
}
func (policy ProviderAccountPolicy) MaximumDispatchStarts() int {
	return policy.maximumDispatchStarts
}
func (policy ProviderAccountPolicy) MaximumAssignedBudgetUnits() int64 {
	return policy.maximumAssignedBudgetUnits
}
func (policy ProviderAccountPolicy) TrustDomain() string     { return policy.trustDomain }
func (policy ProviderAccountPolicy) RetentionMode() string   { return policy.retentionMode }
func (policy ProviderAccountPolicy) DataRegion() string      { return policy.dataRegion }
func (policy ProviderAccountPolicy) ConfiguredAt() time.Time { return policy.configuredAt }
func (policy ProviderAccountPolicy) Digest() string          { return policy.digest }

func NewProviderAccountPolicy(input ProviderAccountPolicyInput) (ProviderAccountPolicy, error) {
	policy := ProviderAccountPolicy{
		version:                    input.Version,
		providerID:                 input.ProviderID,
		providerAccountID:          input.ProviderAccountID,
		revision:                   input.Revision,
		maximumConcurrentAttempts:  input.MaximumConcurrentAttempts,
		dispatchWindow:             input.DispatchWindow,
		maximumDispatchStarts:      input.MaximumDispatchStarts,
		maximumAssignedBudgetUnits: input.MaximumAssignedBudgetUnits,
		trustDomain:                input.TrustDomain,
		retentionMode:              input.RetentionMode,
		dataRegion:                 input.DataRegion,
		configuredAt:               input.ConfiguredAt,
	}
	if !validProviderAccountPolicyShape(policy) {
		return ProviderAccountPolicy{}, ErrInvalidProviderAccountPolicy
	}
	policy.digest = providerAccountPolicyDigest(policy)
	return policy, nil
}

func DecodeProviderAccountPolicyConfiguredEvent(
	event journal.Event,
	previousEventID string,
) (ProviderAccountPolicy, string, error) {
	if event.SchemaVersion != 1 ||
		event.Type != "ProviderAccountPolicyConfigured" ||
		!validOpaqueID(event.CorrelationID) ||
		!strings.HasPrefix(event.StreamID, "provider-account-policy/") ||
		event.Seq <= 0 {
		return ProviderAccountPolicy{}, "", ErrProviderAccountPolicyConflict
	}
	var payload providerAccountPolicyPayload
	if decodeExactPayload(event.PayloadJSON, &payload) != nil {
		return ProviderAccountPolicy{}, "", ErrProviderAccountPolicyConflict
	}
	configuredAt, err := parseUTC(payload.ConfiguredAt)
	providerAccountID := strings.TrimPrefix(
		event.StreamID, "provider-account-policy/",
	)
	if err != nil || !configuredAt.Equal(event.EmittedAt) ||
		payload.ProviderAccountID != providerAccountID ||
		payload.Revision != event.Seq ||
		(event.Seq == 1 && previousEventID != "") ||
		(event.Seq > 1 && event.CausationID != previousEventID) ||
		(event.Seq == 1 && event.CausationID != "") ||
		!validOpaqueID(payload.CommandID) {
		return ProviderAccountPolicy{}, "", ErrProviderAccountPolicyConflict
	}
	policy, err := NewProviderAccountPolicy(ProviderAccountPolicyInput{
		Version:                    payload.PolicyVersion,
		ProviderID:                 payload.ProviderID,
		ProviderAccountID:          payload.ProviderAccountID,
		Revision:                   payload.Revision,
		MaximumConcurrentAttempts:  payload.MaximumConcurrentAttempts,
		DispatchWindow:             time.Duration(payload.DispatchWindowNanoseconds),
		MaximumDispatchStarts:      payload.MaximumDispatchStarts,
		MaximumAssignedBudgetUnits: payload.MaximumAssignedBudgetUnits,
		TrustDomain:                payload.TrustDomain,
		RetentionMode:              payload.RetentionMode,
		DataRegion:                 payload.DataRegion,
		ConfiguredAt:               configuredAt,
	})
	if err != nil || policy.digest != payload.PolicyDigest {
		return ProviderAccountPolicy{}, "", ErrProviderAccountPolicyConflict
	}
	expectedEventID := deterministicEventID(
		"ProviderAccountPolicyConfigured", event.StreamID,
		strconv.FormatInt(policy.revision, 10), payload.CommandID, policy.digest,
	)
	if event.ID != expectedEventID || event.IdempotencyKey != event.ID {
		return ProviderAccountPolicy{}, "", ErrProviderAccountPolicyConflict
	}
	return policy, payload.CommandID, nil
}

func (policy ProviderAccountPolicy) Valid() bool {
	if !validProviderAccountPolicyShape(policy) ||
		policy.digest != providerAccountPolicyDigest(policy) {
		return false
	}
	return true
}

func (authority *Authority) ConfigureProviderAccountPolicy(
	ctx context.Context,
	command ProviderAccountPolicyCommand,
) (ProviderAccountPolicy, error) {
	if authority == nil || ctx == nil || !validProviderAccountPolicyCommand(command) {
		return ProviderAccountPolicy{}, ErrInvalidProviderAccountPolicy
	}
	if err := ctx.Err(); err != nil {
		return ProviderAccountPolicy{}, err
	}
	now := authority.now()
	if now.IsZero() || now.Location() != time.UTC {
		return ProviderAccountPolicy{}, ErrInvalidProviderAccountPolicy
	}
	streamID := providerAccountPolicyStream(command.ProviderAccountID)
	events, err := authority.store.ReadStream(ctx, streamID)
	if err != nil {
		return ProviderAccountPolicy{}, err
	}
	current, found, replayErr := replayProviderAccountPolicy(
		command.ProviderID, command.ProviderAccountID, events,
	)
	if replayErr != nil {
		return ProviderAccountPolicy{}, replayErr
	}
	if found && current.commandID == command.CommandID {
		if current.policy.revision == command.ExpectedRevision+1 &&
			providerAccountPolicyMatchesCommand(current.policy, command) {
			return current.policy, nil
		}
		return ProviderAccountPolicy{}, ErrProviderAccountPolicyConflict
	}
	actualRevision := int64(0)
	causationID := ""
	if found {
		actualRevision = current.policy.revision
		causationID = current.eventID
		if !now.After(current.policy.configuredAt) {
			return ProviderAccountPolicy{}, ErrProviderAccountPolicyConflict
		}
	}
	if actualRevision != command.ExpectedRevision {
		return ProviderAccountPolicy{}, ErrProviderAccountPolicyConflict
	}
	version, ok := providerAccountPolicyVersionForDisclosure(
		command.TrustDomain, command.RetentionMode, command.DataRegion,
	)
	if !ok {
		return ProviderAccountPolicy{}, ErrInvalidProviderAccountPolicy
	}
	policy, err := NewProviderAccountPolicy(ProviderAccountPolicyInput{
		Version:                    version,
		ProviderID:                 command.ProviderID,
		ProviderAccountID:          command.ProviderAccountID,
		Revision:                   actualRevision + 1,
		MaximumConcurrentAttempts:  command.MaximumConcurrentAttempts,
		DispatchWindow:             command.DispatchWindow,
		MaximumDispatchStarts:      command.MaximumDispatchStarts,
		MaximumAssignedBudgetUnits: command.MaximumAssignedBudgetUnits,
		TrustDomain:                command.TrustDomain,
		RetentionMode:              command.RetentionMode,
		DataRegion:                 command.DataRegion,
		ConfiguredAt:               now,
	})
	if err != nil {
		return ProviderAccountPolicy{}, err
	}
	eventID := deterministicEventID(
		"ProviderAccountPolicyConfigured", streamID,
		strconv.FormatInt(policy.revision, 10), command.CommandID, policy.digest,
	)
	event := newEvent(
		eventID, streamID, policy.revision,
		"ProviderAccountPolicyConfigured", now, command.CorrelationID, causationID,
		providerAccountPolicyPayloadFrom(policy, command.CommandID),
	)
	if _, err := authority.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: streamID, Sequence: command.ExpectedRevision,
		}},
		[]journal.Event{event},
	); err != nil {
		return ProviderAccountPolicy{}, mapProviderAccountPolicyWriteError(err)
	}
	return policy, nil
}

func (authority *Authority) ProviderAccountPolicy(
	ctx context.Context,
	providerID string,
	providerAccountID string,
) (ProviderAccountPolicy, error) {
	if authority == nil || ctx == nil ||
		!credentials.ValidProviderAccountIdentifier(providerID, providerAccountID) {
		return ProviderAccountPolicy{}, ErrInvalidProviderAccountPolicy
	}
	if err := ctx.Err(); err != nil {
		return ProviderAccountPolicy{}, err
	}
	events, err := authority.store.ReadStream(
		ctx, providerAccountPolicyStream(providerAccountID),
	)
	if err != nil {
		return ProviderAccountPolicy{}, err
	}
	policy, found, err := replayProviderAccountPolicy(
		providerID, providerAccountID, events,
	)
	if err != nil {
		return ProviderAccountPolicy{}, err
	}
	if !found {
		return ProviderAccountPolicy{}, ErrProviderAccountPolicyNotFound
	}
	return policy.policy, nil
}

func replayProviderAccountPolicy(
	providerID string,
	providerAccountID string,
	events []journal.Event,
) (providerAccountPolicyState, bool, error) {
	if !credentials.ValidProviderAccountIdentifier(providerID, providerAccountID) {
		return providerAccountPolicyState{}, false, ErrInvalidProviderAccountPolicy
	}
	streamID := providerAccountPolicyStream(providerAccountID)
	var current providerAccountPolicyState
	for index, event := range events {
		if event.StreamID != streamID || event.Seq != int64(index+1) {
			return providerAccountPolicyState{}, false, ErrProviderAccountPolicyConflict
		}
		candidate, commandID, err := DecodeProviderAccountPolicyConfiguredEvent(
			event, current.eventID,
		)
		if err != nil || candidate.providerID != providerID ||
			candidate.providerAccountID != providerAccountID {
			return providerAccountPolicyState{}, false, ErrProviderAccountPolicyConflict
		}
		current = providerAccountPolicyState{
			policy: candidate, commandID: commandID, eventID: event.ID,
		}
	}
	return current, len(events) > 0, nil
}

func providerAccountPolicyPayloadFrom(
	policy ProviderAccountPolicy,
	commandID string,
) providerAccountPolicyPayload {
	return providerAccountPolicyPayload{
		CommandID:                  commandID,
		PolicyVersion:              policy.version,
		ProviderID:                 policy.providerID,
		ProviderAccountID:          policy.providerAccountID,
		Revision:                   policy.revision,
		MaximumConcurrentAttempts:  policy.maximumConcurrentAttempts,
		DispatchWindowNanoseconds:  int64(policy.dispatchWindow),
		MaximumDispatchStarts:      policy.maximumDispatchStarts,
		MaximumAssignedBudgetUnits: policy.maximumAssignedBudgetUnits,
		TrustDomain:                policy.trustDomain,
		RetentionMode:              policy.retentionMode,
		DataRegion:                 policy.dataRegion,
		ConfiguredAt:               policy.configuredAt.Format(time.RFC3339Nano),
		PolicyDigest:               policy.digest,
	}
}

func validProviderAccountPolicyCommand(command ProviderAccountPolicyCommand) bool {
	return validOpaqueID(command.CommandID) &&
		credentials.ValidProviderAccountIdentifier(
			command.ProviderID, command.ProviderAccountID,
		) &&
		command.ExpectedRevision >= 0 &&
		command.ExpectedRevision < int64(^uint32(0)) &&
		validProviderAccountPolicyLimits(
			command.MaximumConcurrentAttempts,
			command.DispatchWindow,
			command.MaximumDispatchStarts,
			command.MaximumAssignedBudgetUnits,
		) && validProviderAccountPolicyCommandDisclosure(command) &&
		validOpaqueID(command.CorrelationID)
}

func validProviderAccountPolicyCommandDisclosure(
	command ProviderAccountPolicyCommand,
) bool {
	_, ok := providerAccountPolicyVersionForDisclosure(
		command.TrustDomain, command.RetentionMode, command.DataRegion,
	)
	return ok
}

func validProviderAccountPolicyShape(policy ProviderAccountPolicy) bool {
	return (policy.version == providerAccountPolicyLegacyVersion ||
		policy.version == providerAccountPolicyCurrentVersion) &&
		credentials.ValidProviderAccountIdentifier(
			policy.providerID, policy.providerAccountID,
		) && policy.revision > 0 &&
		validProviderAccountPolicyLimits(
			policy.maximumConcurrentAttempts,
			policy.dispatchWindow,
			policy.maximumDispatchStarts,
			policy.maximumAssignedBudgetUnits,
		) && validProviderAccountPolicyVersionDisclosure(
		policy.version, policy.trustDomain, policy.retentionMode, policy.dataRegion,
	) && !policy.configuredAt.IsZero() &&
		policy.configuredAt.Location() == time.UTC
}

func validProviderAccountPolicyLimits(
	maximumConcurrentAttempts int,
	dispatchWindow time.Duration,
	maximumDispatchStarts int,
	maximumAssignedBudgetUnits int64,
) bool {
	return maximumConcurrentAttempts > 0 &&
		maximumConcurrentAttempts <= maximumProviderAccountConcurrency &&
		dispatchWindow >= time.Second &&
		dispatchWindow <= maximumProviderAccountDispatchWindow &&
		maximumDispatchStarts > 0 &&
		maximumDispatchStarts <= maximumProviderAccountDispatchStarts &&
		maximumAssignedBudgetUnits >= 0
}

func providerAccountPolicyMatchesCommand(
	policy ProviderAccountPolicy,
	command ProviderAccountPolicyCommand,
) bool {
	return policy.providerID == command.ProviderID &&
		policy.providerAccountID == command.ProviderAccountID &&
		policy.maximumConcurrentAttempts == command.MaximumConcurrentAttempts &&
		policy.dispatchWindow == command.DispatchWindow &&
		policy.maximumDispatchStarts == command.MaximumDispatchStarts &&
		policy.maximumAssignedBudgetUnits == command.MaximumAssignedBudgetUnits &&
		policy.trustDomain == command.TrustDomain &&
		policy.retentionMode == command.RetentionMode &&
		policy.dataRegion == command.DataRegion
}

func providerAccountPolicyDigest(policy ProviderAccountPolicy) string {
	digest := sha256.New()
	fields := []string{
		"loom.provider-account-policy.v1",
		strconv.Itoa(policy.version),
		policy.providerID,
		policy.providerAccountID,
		strconv.FormatInt(policy.revision, 10),
		strconv.Itoa(policy.maximumConcurrentAttempts),
		strconv.FormatInt(int64(policy.dispatchWindow), 10),
		strconv.Itoa(policy.maximumDispatchStarts),
		strconv.FormatInt(policy.maximumAssignedBudgetUnits, 10),
		policy.configuredAt.Format(time.RFC3339Nano),
	}
	if policy.version == providerAccountPolicyCurrentVersion {
		fields[0] = "loom.provider-account-policy.v2"
		fields = append(fields, policy.trustDomain, policy.retentionMode, policy.dataRegion)
	}
	for _, field := range fields {
		writeProviderAccountPolicyField(digest, field)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func providerAccountPolicyVersionForDisclosure(
	trustDomain string,
	retentionMode string,
	dataRegion string,
) (int, bool) {
	if trustDomain == "" && retentionMode == "" && dataRegion == "" {
		return providerAccountPolicyLegacyVersion, true
	}
	if validProviderAccountDisclosurePolicy(trustDomain, retentionMode, dataRegion) {
		return providerAccountPolicyCurrentVersion, true
	}
	return 0, false
}

func validProviderAccountPolicyVersionDisclosure(
	version int,
	trustDomain string,
	retentionMode string,
	dataRegion string,
) bool {
	switch version {
	case providerAccountPolicyLegacyVersion:
		return trustDomain == "" && retentionMode == "" && dataRegion == ""
	case providerAccountPolicyCurrentVersion:
		return validProviderAccountDisclosurePolicy(trustDomain, retentionMode, dataRegion)
	default:
		return false
	}
}

func validProviderAccountDisclosurePolicy(
	trustDomain string,
	retentionMode string,
	dataRegion string,
) bool {
	return oneOfProviderAccountPolicyValue(trustDomain,
		"external_provider", "enterprise_tenant", "local_runtime") &&
		oneOfProviderAccountPolicyValue(retentionMode,
			"provider_default", "zero_data_retention", "limited_retention") &&
		oneOfProviderAccountPolicyValue(dataRegion,
			"global", "us", "eu", "apac", "local")
}

func oneOfProviderAccountPolicyValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func writeProviderAccountPolicyField(target hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = target.Write(length[:])
	_, _ = target.Write([]byte(value))
}

func providerAccountPolicyStream(providerAccountID string) string {
	return "provider-account-policy/" + providerAccountID
}

func mapProviderAccountPolicyWriteError(err error) error {
	switch {
	case errors.Is(err, journal.ErrStreamHeadConflict),
		errors.Is(err, journal.ErrSequenceConflict),
		errors.Is(err, journal.ErrIdempotencyConflict),
		errors.Is(err, journal.ErrPartialEventBatchConflict):
		return fmt.Errorf("%w: %v", ErrProviderAccountPolicyConflict, err)
	default:
		return err
	}
}
