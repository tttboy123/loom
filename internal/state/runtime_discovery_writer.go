package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

const (
	runtimeInstanceDiscoveredEventType  = "RuntimeInstanceDiscovered"
	runtimeDiscoveryEventSchemaVersion  = 1
	maxRuntimeDiscoveryCommitEvents     = 32
	runtimeDiscoveryCommitDigestVersion = 1
)

var (
	ErrInvalidRuntimeDiscoveryCommitInput   = errors.New("invalid runtime discovery commit input")
	ErrInvalidRuntimeDiscoveryCommitSource  = errors.New("invalid runtime discovery commit source")
	ErrRuntimeDiscoveryCommitResultMismatch = errors.New("runtime discovery commit result mismatch")
	ErrRuntimeDiscoveryCommitDigestMismatch = errors.New("runtime discovery commit digest mismatch")
	ErrEmptyRuntimeDiscoveryCommit          = errors.New("empty runtime discovery commit")
)

type RuntimeDiscoveryEventInput struct {
	RuntimeInstanceID string
	EventID           string
	IdempotencyKey    string
	Seq               int64
}

type RuntimeDiscoveryCommitInput struct {
	DiscoveryID string
	EmittedAt   time.Time
	Events      []RuntimeDiscoveryEventInput
}

type RuntimeDiscoveryCommitCandidate struct {
	committed             bool
	sourceDiscoveryDigest string
	events                []journal.Event
	eventCount            int
	commitDigest          string
}

func CommitRuntimeDiscoverySnapshot(
	ctx context.Context,
	appender EventBatchAppender,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
	input RuntimeDiscoveryCommitInput,
) (RuntimeDiscoveryCommitCandidate, error) {
	if ctx == nil || isNilEventBatchAppender(appender) {
		return RuntimeDiscoveryCommitCandidate{}, ErrInvalidRuntimeDiscoveryCommitInput
	}
	if err := ctx.Err(); err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}

	observations, err := validateRuntimeDiscoveryCommitSource(snapshot)
	if err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}
	normalizedInput, metadataByInstance, err := validateRuntimeDiscoveryCommitInput(input, observations)
	if err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}
	requested, err := buildRuntimeDiscoveryCommitEvents(
		snapshot.Digest(),
		observations,
		normalizedInput,
		metadataByInstance,
	)
	if err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}

	committed, err := appender.AppendBatch(ctx, cloneRuntimeDiscoveryCommitEvents(requested))
	if err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}
	if len(committed) != len(requested) {
		return RuntimeDiscoveryCommitCandidate{}, ErrRuntimeDiscoveryCommitResultMismatch
	}
	for index := range requested {
		if !sameRuntimeDiscoveryCommitEvent(requested[index], committed[index]) {
			return RuntimeDiscoveryCommitCandidate{}, ErrRuntimeDiscoveryCommitResultMismatch
		}
	}

	candidate := RuntimeDiscoveryCommitCandidate{
		committed:             true,
		sourceDiscoveryDigest: snapshot.Digest(),
		events:                cloneRuntimeDiscoveryCommitEvents(committed),
		eventCount:            len(committed),
	}
	candidate.commitDigest, err = digestRuntimeDiscoveryCommitCandidate(candidate)
	if err != nil {
		return RuntimeDiscoveryCommitCandidate{}, ErrRuntimeDiscoveryCommitDigestMismatch
	}
	if err := validateRuntimeDiscoveryCommitCandidate(candidate); err != nil {
		return RuntimeDiscoveryCommitCandidate{}, err
	}
	return candidate, nil
}

func (c RuntimeDiscoveryCommitCandidate) Committed() bool {
	return c.committed
}

func (c RuntimeDiscoveryCommitCandidate) SourceDiscoveryDigest() string {
	return c.sourceDiscoveryDigest
}

func (c RuntimeDiscoveryCommitCandidate) Events() []journal.Event {
	return cloneRuntimeDiscoveryCommitEvents(c.events)
}

func (c RuntimeDiscoveryCommitCandidate) EventCount() int {
	return c.eventCount
}

func (c RuntimeDiscoveryCommitCandidate) CommitDigest() string {
	return c.commitDigest
}

func validateRuntimeDiscoveryCommitSource(
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) ([]loomruntime.RuntimeObservation, error) {
	if !validRuntimeDiscoveryCommitDigest(snapshot.Digest()) {
		return nil, ErrInvalidRuntimeDiscoveryCommitSource
	}
	observations := snapshot.Observations()
	if len(observations) == 0 {
		return nil, ErrEmptyRuntimeDiscoveryCommit
	}
	if len(observations) > maxRuntimeDiscoveryCommitEvents {
		return nil, ErrInvalidRuntimeDiscoveryCommitSource
	}

	seenInstances := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		if observation.SourceProbeID == "" {
			return nil, ErrInvalidRuntimeDiscoveryCommitSource
		}
		normalized, err := loomruntime.NewRuntimeInstance(observation.Instance)
		if err != nil || !reflect.DeepEqual(normalized, observation.Instance) {
			return nil, ErrInvalidRuntimeDiscoveryCommitSource
		}
		if _, exists := seenInstances[observation.Instance.ID]; exists {
			return nil, ErrInvalidRuntimeDiscoveryCommitSource
		}
		seenInstances[observation.Instance.ID] = struct{}{}
		if !validCanonicalRuntimeDiscoveryModels(observation.ModelIDs) {
			return nil, ErrInvalidRuntimeDiscoveryCommitSource
		}
	}
	return cloneRuntimeDiscoveryObservations(observations), nil
}

func validateRuntimeDiscoveryCommitInput(
	input RuntimeDiscoveryCommitInput,
	observations []loomruntime.RuntimeObservation,
) (
	RuntimeDiscoveryCommitInput,
	map[string]RuntimeDiscoveryEventInput,
	error,
) {
	if input.DiscoveryID == "" ||
		input.EmittedAt.IsZero() ||
		len(input.Events) == 0 ||
		len(input.Events) > maxRuntimeDiscoveryCommitEvents ||
		len(input.Events) != len(observations) {
		return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
	}
	input.EmittedAt = input.EmittedAt.UTC()
	input.Events = append([]RuntimeDiscoveryEventInput(nil), input.Events...)

	knownInstances := make(map[string]struct{}, len(observations))
	for _, observation := range observations {
		knownInstances[observation.Instance.ID] = struct{}{}
	}
	metadataByInstance := make(map[string]RuntimeDiscoveryEventInput, len(input.Events))
	eventIDs := make(map[string]struct{}, len(input.Events))
	idempotencyKeys := make(map[string]struct{}, len(input.Events))
	for _, metadata := range input.Events {
		if metadata.RuntimeInstanceID == "" ||
			metadata.EventID == "" ||
			metadata.IdempotencyKey == "" ||
			metadata.Seq <= 0 {
			return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
		}
		if _, exists := knownInstances[metadata.RuntimeInstanceID]; !exists {
			return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
		}
		if _, exists := metadataByInstance[metadata.RuntimeInstanceID]; exists {
			return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
		}
		if _, exists := eventIDs[metadata.EventID]; exists {
			return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
		}
		if _, exists := idempotencyKeys[metadata.IdempotencyKey]; exists {
			return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
		}
		metadataByInstance[metadata.RuntimeInstanceID] = metadata
		eventIDs[metadata.EventID] = struct{}{}
		idempotencyKeys[metadata.IdempotencyKey] = struct{}{}
	}
	if len(metadataByInstance) != len(knownInstances) {
		return RuntimeDiscoveryCommitInput{}, nil, ErrInvalidRuntimeDiscoveryCommitInput
	}
	return input, metadataByInstance, nil
}

func buildRuntimeDiscoveryCommitEvents(
	discoveryDigest string,
	observations []loomruntime.RuntimeObservation,
	input RuntimeDiscoveryCommitInput,
	metadataByInstance map[string]RuntimeDiscoveryEventInput,
) ([]journal.Event, error) {
	events := make([]journal.Event, len(observations))
	for index, observation := range observations {
		payload, err := json.Marshal(runtimeDiscoveryCommittedPayload{
			DiscoveryDigest: discoveryDigest,
			SourceProbeID:   observation.SourceProbeID,
			Instance:        canonicalRuntimeDiscoveryInstance(observation.Instance),
			ModelIDs:        append([]string(nil), observation.ModelIDs...),
		})
		if err != nil {
			return nil, ErrInvalidRuntimeDiscoveryCommitSource
		}
		metadata := metadataByInstance[observation.Instance.ID]
		events[index] = journal.Event{
			ID:             metadata.EventID,
			StreamID:       "runtime_instance:" + observation.Instance.ID,
			Seq:            metadata.Seq,
			IdempotencyKey: metadata.IdempotencyKey,
			Type:           runtimeInstanceDiscoveredEventType,
			SchemaVersion:  runtimeDiscoveryEventSchemaVersion,
			EmittedAt:      input.EmittedAt,
			CorrelationID:  input.DiscoveryID,
			CausationID:    "",
			PayloadJSON:    append([]byte(nil), payload...),
		}
	}
	return events, nil
}

type runtimeDiscoveryCommittedPayload struct {
	DiscoveryDigest string                            `json:"discovery_digest"`
	SourceProbeID   string                            `json:"source_probe_id"`
	Instance        runtimeDiscoveryCommittedInstance `json:"instance"`
	ModelIDs        []string                          `json:"model_ids"`
}

type runtimeDiscoveryCommittedInstance struct {
	ID                   string                    `json:"id"`
	DeviceID             string                    `json:"device_id"`
	AdapterType          string                    `json:"adapter_type"`
	DisplayName          string                    `json:"display_name"`
	ExecutableVersion    string                    `json:"executable_version"`
	Status               loomruntime.RuntimeStatus `json:"status"`
	ObservedCapabilities []string                  `json:"observed_capabilities"`
	Capacity             int                       `json:"capacity"`
}

func canonicalRuntimeDiscoveryInstance(
	instance loomruntime.RuntimeInstance,
) runtimeDiscoveryCommittedInstance {
	return runtimeDiscoveryCommittedInstance{
		ID:                   instance.ID,
		DeviceID:             instance.DeviceID,
		AdapterType:          instance.AdapterType,
		DisplayName:          instance.DisplayName,
		ExecutableVersion:    instance.ExecutableVersion,
		Status:               instance.Status,
		ObservedCapabilities: append([]string(nil), instance.ObservedCapabilities...),
		Capacity:             instance.Capacity,
	}
}

func validCanonicalRuntimeDiscoveryModels(models []string) bool {
	for index, modelID := range models {
		if modelID == "" || index > 0 && modelID <= models[index-1] {
			return false
		}
	}
	return true
}

func validRuntimeDiscoveryCommitDigest(digest string) bool {
	if len(digest) != sha256.Size*2 {
		return false
	}
	for _, character := range digest {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func validateRuntimeDiscoveryCommitCandidate(
	candidate RuntimeDiscoveryCommitCandidate,
) error {
	if !candidate.committed ||
		!validRuntimeDiscoveryCommitDigest(candidate.sourceDiscoveryDigest) ||
		candidate.eventCount == 0 ||
		candidate.eventCount != len(candidate.events) ||
		!validRuntimeDiscoveryCommitDigest(candidate.commitDigest) {
		return ErrRuntimeDiscoveryCommitDigestMismatch
	}
	digest, err := digestRuntimeDiscoveryCommitCandidate(candidate)
	if err != nil || digest != candidate.commitDigest {
		return ErrRuntimeDiscoveryCommitDigestMismatch
	}
	return nil
}

type runtimeDiscoveryCommitDigestPayload struct {
	Version               int             `json:"version"`
	SourceDiscoveryDigest string          `json:"source_discovery_digest"`
	Events                []journal.Event `json:"events"`
}

func digestRuntimeDiscoveryCommitCandidate(
	candidate RuntimeDiscoveryCommitCandidate,
) (string, error) {
	encoded, err := json.Marshal(runtimeDiscoveryCommitDigestPayload{
		Version:               runtimeDiscoveryCommitDigestVersion,
		SourceDiscoveryDigest: candidate.sourceDiscoveryDigest,
		Events:                cloneRuntimeDiscoveryCommitEvents(candidate.events),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func sameRuntimeDiscoveryCommitEvent(left, right journal.Event) bool {
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

func cloneRuntimeDiscoveryCommitEvents(events []journal.Event) []journal.Event {
	result := make([]journal.Event, len(events))
	for index, event := range events {
		event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
		result[index] = event
	}
	return result
}

func cloneRuntimeDiscoveryObservations(
	observations []loomruntime.RuntimeObservation,
) []loomruntime.RuntimeObservation {
	result := make([]loomruntime.RuntimeObservation, len(observations))
	for index, observation := range observations {
		result[index] = loomruntime.RuntimeObservation{
			SourceProbeID: observation.SourceProbeID,
			Instance:      observation.Instance,
			ModelIDs:      append([]string(nil), observation.ModelIDs...),
		}
		result[index].Instance.ObservedCapabilities = append(
			[]string(nil),
			observation.Instance.ObservedCapabilities...,
		)
	}
	return result
}
