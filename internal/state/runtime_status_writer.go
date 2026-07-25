package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

const (
	runtimeInstanceStatusChangedEventType    = "RuntimeInstanceStatusChanged"
	runtimeStatusEventSchemaVersion          = 1
	maxRuntimeStatusCommitEvents             = 32
	runtimeStatusReconciliationDigestVersion = 1
	runtimeStatusCommitDigestVersion         = 1
)

var (
	ErrInvalidRuntimeStatusCommitInput   = errors.New("invalid runtime status commit input")
	ErrInvalidRuntimeStatusCommitSource  = errors.New("invalid runtime status commit source")
	ErrRuntimeStatusCommitResultMismatch = errors.New("runtime status commit result mismatch")
	ErrRuntimeStatusCommitDigestMismatch = errors.New("runtime status commit digest mismatch")
	ErrEmptyRuntimeStatusCommit          = errors.New("empty runtime status commit")
)

type RuntimeStatusEventInput struct {
	RuntimeInstanceID string
	EventID           string
	IdempotencyKey    string
	Seq               int64
}

type RuntimeStatusCommitInput struct {
	ReconciliationID string
	EmittedAt        time.Time
	Events           []RuntimeStatusEventInput
}

type RuntimeStatusCommitCandidate struct {
	committed                  bool
	sourceReconciliationDigest string
	baselineDigest             string
	sourceDiscoveryDigest      string
	events                     []journal.Event
	eventCount                 int
	commitDigest               string
}

func CommitRuntimeStatusTransitions(
	ctx context.Context,
	appender EventBatchAppender,
	source loomruntime.RuntimeStatusReconciliationCandidate,
	input RuntimeStatusCommitInput,
) (RuntimeStatusCommitCandidate, error) {
	if ctx == nil || isNilEventBatchAppender(appender) {
		return RuntimeStatusCommitCandidate{}, ErrInvalidRuntimeStatusCommitInput
	}
	if err := ctx.Err(); err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}

	transitions, err := validateRuntimeStatusCommitSource(source)
	if err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}
	normalizedInput, metadataByRuntime, err := validateRuntimeStatusCommitInput(
		input, transitions,
	)
	if err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}
	requested, err := buildRuntimeStatusCommitEvents(
		source, transitions, normalizedInput, metadataByRuntime,
	)
	if err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}

	committed, err := appender.AppendBatch(
		ctx, cloneRuntimeStatusCommitEvents(requested),
	)
	if err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}
	if len(committed) != len(requested) {
		return RuntimeStatusCommitCandidate{}, ErrRuntimeStatusCommitResultMismatch
	}
	for index := range requested {
		if !sameRuntimeStatusCommitEvent(requested[index], committed[index]) {
			return RuntimeStatusCommitCandidate{}, ErrRuntimeStatusCommitResultMismatch
		}
	}

	candidate := RuntimeStatusCommitCandidate{
		committed:                  true,
		sourceReconciliationDigest: source.CandidateDigest(),
		baselineDigest:             source.BaselineDigest(),
		sourceDiscoveryDigest:      source.SourceDiscoveryDigest(),
		events:                     cloneRuntimeStatusCommitEvents(committed),
		eventCount:                 len(committed),
	}
	candidate.commitDigest, err = digestRuntimeStatusCommitCandidate(candidate)
	if err != nil {
		return RuntimeStatusCommitCandidate{}, ErrRuntimeStatusCommitDigestMismatch
	}
	if err := validateRuntimeStatusCommitCandidate(candidate); err != nil {
		return RuntimeStatusCommitCandidate{}, err
	}
	return candidate, nil
}

func (c RuntimeStatusCommitCandidate) Committed() bool {
	return c.committed
}

func (c RuntimeStatusCommitCandidate) SourceReconciliationDigest() string {
	return c.sourceReconciliationDigest
}

func (c RuntimeStatusCommitCandidate) BaselineDigest() string {
	return c.baselineDigest
}

func (c RuntimeStatusCommitCandidate) SourceDiscoveryDigest() string {
	return c.sourceDiscoveryDigest
}

func (c RuntimeStatusCommitCandidate) Events() []journal.Event {
	return cloneRuntimeStatusCommitEvents(c.events)
}

func (c RuntimeStatusCommitCandidate) EventCount() int {
	return c.eventCount
}

func (c RuntimeStatusCommitCandidate) CommitDigest() string {
	return c.commitDigest
}

func validateRuntimeStatusCommitSource(
	source loomruntime.RuntimeStatusReconciliationCandidate,
) ([]loomruntime.RuntimeStatusTransition, error) {
	if !source.Reconciled() {
		return nil, ErrInvalidRuntimeStatusCommitSource
	}
	transitions := source.Transitions()
	if len(transitions) == 0 && source.TransitionCount() == 0 {
		return nil, ErrEmptyRuntimeStatusCommit
	}
	if len(transitions) == 0 ||
		len(transitions) > maxRuntimeStatusCommitEvents ||
		source.TransitionCount() != len(transitions) ||
		!validRuntimeStatusCommitDigest(source.BaselineDigest()) ||
		!validRuntimeStatusCommitDigest(source.SourceDiscoveryDigest()) ||
		!validRuntimeStatusCommitDigest(source.CandidateDigest()) {
		return nil, ErrInvalidRuntimeStatusCommitSource
	}
	for index, transition := range transitions {
		if transition.RuntimeInstanceID == "" ||
			transition.DeviceID == "" ||
			transition.AdapterType == "" ||
			!validRuntimeStatusCommitStatus(transition.FromStatus) ||
			!validRuntimeStatusCommitStatus(transition.ToStatus) ||
			transition.FromStatus == transition.ToStatus ||
			transition.SourceProbeID == "" ||
			transition.SourceDiscoveryDigest != source.SourceDiscoveryDigest() ||
			transition.PreviousEventID == "" ||
			transition.PreviousSequence <= 0 ||
			index > 0 &&
				transition.RuntimeInstanceID <= transitions[index-1].RuntimeInstanceID {
			return nil, ErrInvalidRuntimeStatusCommitSource
		}
	}
	digest, err := digestRuntimeStatusReconciliationSource(
		source.BaselineDigest(),
		source.SourceDiscoveryDigest(),
		transitions,
		source.TransitionCount(),
	)
	if err != nil || digest != source.CandidateDigest() {
		return nil, ErrInvalidRuntimeStatusCommitSource
	}
	return cloneRuntimeStatusCommitTransitions(transitions), nil
}

func validateRuntimeStatusCommitInput(
	input RuntimeStatusCommitInput,
	transitions []loomruntime.RuntimeStatusTransition,
) (
	RuntimeStatusCommitInput,
	map[string]RuntimeStatusEventInput,
	error,
) {
	if input.ReconciliationID == "" ||
		input.EmittedAt.IsZero() ||
		len(input.Events) == 0 ||
		len(input.Events) > maxRuntimeStatusCommitEvents ||
		len(input.Events) != len(transitions) {
		return RuntimeStatusCommitInput{}, nil, ErrInvalidRuntimeStatusCommitInput
	}
	input.EmittedAt = input.EmittedAt.UTC()
	input.Events = append([]RuntimeStatusEventInput(nil), input.Events...)

	transitionByRuntime := make(
		map[string]loomruntime.RuntimeStatusTransition, len(transitions),
	)
	for _, transition := range transitions {
		transitionByRuntime[transition.RuntimeInstanceID] = transition
	}
	metadataByRuntime := make(
		map[string]RuntimeStatusEventInput, len(input.Events),
	)
	eventIDs := make(map[string]struct{}, len(input.Events))
	idempotencyKeys := make(map[string]struct{}, len(input.Events))
	for _, metadata := range input.Events {
		transition, exists := transitionByRuntime[metadata.RuntimeInstanceID]
		if !exists ||
			metadata.RuntimeInstanceID == "" ||
			metadata.EventID == "" ||
			metadata.IdempotencyKey == "" ||
			metadata.Seq <= 0 ||
			transition.PreviousSequence == math.MaxInt64 ||
			metadata.Seq != transition.PreviousSequence+1 {
			return RuntimeStatusCommitInput{}, nil, ErrInvalidRuntimeStatusCommitInput
		}
		if _, exists := metadataByRuntime[metadata.RuntimeInstanceID]; exists {
			return RuntimeStatusCommitInput{}, nil, ErrInvalidRuntimeStatusCommitInput
		}
		if _, exists := eventIDs[metadata.EventID]; exists {
			return RuntimeStatusCommitInput{}, nil, ErrInvalidRuntimeStatusCommitInput
		}
		if _, exists := idempotencyKeys[metadata.IdempotencyKey]; exists {
			return RuntimeStatusCommitInput{}, nil, ErrInvalidRuntimeStatusCommitInput
		}
		metadataByRuntime[metadata.RuntimeInstanceID] = metadata
		eventIDs[metadata.EventID] = struct{}{}
		idempotencyKeys[metadata.IdempotencyKey] = struct{}{}
	}
	if len(metadataByRuntime) != len(transitionByRuntime) {
		return RuntimeStatusCommitInput{}, nil, ErrInvalidRuntimeStatusCommitInput
	}
	return input, metadataByRuntime, nil
}

func buildRuntimeStatusCommitEvents(
	source loomruntime.RuntimeStatusReconciliationCandidate,
	transitions []loomruntime.RuntimeStatusTransition,
	input RuntimeStatusCommitInput,
	metadataByRuntime map[string]RuntimeStatusEventInput,
) ([]journal.Event, error) {
	events := make([]journal.Event, len(transitions))
	for index, transition := range transitions {
		payload, err := json.Marshal(runtimeStatusCommittedPayload{
			ReconciliationDigest:  source.CandidateDigest(),
			BaselineDigest:        source.BaselineDigest(),
			SourceDiscoveryDigest: source.SourceDiscoveryDigest(),
			SourceProbeID:         transition.SourceProbeID,
			RuntimeInstanceID:     transition.RuntimeInstanceID,
			DeviceID:              transition.DeviceID,
			AdapterType:           transition.AdapterType,
			FromStatus:            transition.FromStatus,
			ToStatus:              transition.ToStatus,
			PreviousEventID:       transition.PreviousEventID,
			PreviousSequence:      transition.PreviousSequence,
		})
		if err != nil {
			return nil, ErrInvalidRuntimeStatusCommitSource
		}
		metadata := metadataByRuntime[transition.RuntimeInstanceID]
		events[index] = journal.Event{
			ID:             metadata.EventID,
			StreamID:       "runtime_instance:" + transition.RuntimeInstanceID,
			Seq:            metadata.Seq,
			IdempotencyKey: metadata.IdempotencyKey,
			Type:           runtimeInstanceStatusChangedEventType,
			SchemaVersion:  runtimeStatusEventSchemaVersion,
			EmittedAt:      input.EmittedAt,
			CorrelationID:  input.ReconciliationID,
			CausationID:    transition.PreviousEventID,
			PayloadJSON:    append([]byte(nil), payload...),
		}
	}
	return events, nil
}

type runtimeStatusCommittedPayload struct {
	ReconciliationDigest  string                    `json:"reconciliation_digest"`
	BaselineDigest        string                    `json:"baseline_digest"`
	SourceDiscoveryDigest string                    `json:"source_discovery_digest"`
	SourceProbeID         string                    `json:"source_probe_id"`
	RuntimeInstanceID     string                    `json:"runtime_instance_id"`
	DeviceID              string                    `json:"device_id"`
	AdapterType           string                    `json:"adapter_type"`
	FromStatus            loomruntime.RuntimeStatus `json:"from_status"`
	ToStatus              loomruntime.RuntimeStatus `json:"to_status"`
	PreviousEventID       string                    `json:"previous_event_id"`
	PreviousSequence      int64                     `json:"previous_sequence"`
}

type runtimeStatusReconciliationDigestPayload struct {
	Version               int                                   `json:"version"`
	BaselineDigest        string                                `json:"baseline_digest"`
	SourceDiscoveryDigest string                                `json:"source_discovery_digest"`
	Transitions           []loomruntime.RuntimeStatusTransition `json:"transitions"`
	TransitionCount       int                                   `json:"transition_count"`
}

func digestRuntimeStatusReconciliationSource(
	baselineDigest string,
	sourceDiscoveryDigest string,
	transitions []loomruntime.RuntimeStatusTransition,
	transitionCount int,
) (string, error) {
	encoded, err := json.Marshal(runtimeStatusReconciliationDigestPayload{
		Version:               runtimeStatusReconciliationDigestVersion,
		BaselineDigest:        baselineDigest,
		SourceDiscoveryDigest: sourceDiscoveryDigest,
		Transitions:           cloneRuntimeStatusCommitTransitions(transitions),
		TransitionCount:       transitionCount,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

type runtimeStatusCommitDigestPayload struct {
	Version                    int             `json:"version"`
	SourceReconciliationDigest string          `json:"source_reconciliation_digest"`
	BaselineDigest             string          `json:"baseline_digest"`
	SourceDiscoveryDigest      string          `json:"source_discovery_digest"`
	Events                     []journal.Event `json:"events"`
}

func digestRuntimeStatusCommitCandidate(
	candidate RuntimeStatusCommitCandidate,
) (string, error) {
	encoded, err := json.Marshal(runtimeStatusCommitDigestPayload{
		Version:                    runtimeStatusCommitDigestVersion,
		SourceReconciliationDigest: candidate.sourceReconciliationDigest,
		BaselineDigest:             candidate.baselineDigest,
		SourceDiscoveryDigest:      candidate.sourceDiscoveryDigest,
		Events:                     cloneRuntimeStatusCommitEvents(candidate.events),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateRuntimeStatusCommitCandidate(
	candidate RuntimeStatusCommitCandidate,
) error {
	if !candidate.committed ||
		!validRuntimeStatusCommitDigest(candidate.sourceReconciliationDigest) ||
		!validRuntimeStatusCommitDigest(candidate.baselineDigest) ||
		!validRuntimeStatusCommitDigest(candidate.sourceDiscoveryDigest) ||
		candidate.eventCount == 0 ||
		candidate.eventCount != len(candidate.events) ||
		!validRuntimeStatusCommitDigest(candidate.commitDigest) {
		return ErrRuntimeStatusCommitDigestMismatch
	}
	digest, err := digestRuntimeStatusCommitCandidate(candidate)
	if err != nil || digest != candidate.commitDigest {
		return ErrRuntimeStatusCommitDigestMismatch
	}
	return nil
}

func sameRuntimeStatusCommitEvent(left, right journal.Event) bool {
	return left.ID == right.ID &&
		left.StreamID == right.StreamID &&
		left.Seq == right.Seq &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.EmittedAt.Equal(right.EmittedAt) &&
		left.EmittedAt.Location() == time.UTC &&
		right.EmittedAt.Location() == time.UTC &&
		left.CorrelationID == right.CorrelationID &&
		left.CausationID == right.CausationID &&
		bytes.Equal(left.PayloadJSON, right.PayloadJSON)
}

func validRuntimeStatusCommitStatus(value loomruntime.RuntimeStatus) bool {
	switch value {
	case loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
		loomruntime.RuntimeIncompatible,
		loomruntime.RuntimeDisabled:
		return true
	default:
		return false
	}
}

func validRuntimeStatusCommitDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func cloneRuntimeStatusCommitEvents(events []journal.Event) []journal.Event {
	result := make([]journal.Event, len(events))
	for index, event := range events {
		event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
		result[index] = event
	}
	return result
}

func cloneRuntimeStatusCommitTransitions(
	transitions []loomruntime.RuntimeStatusTransition,
) []loomruntime.RuntimeStatusTransition {
	return append([]loomruntime.RuntimeStatusTransition(nil), transitions...)
}
