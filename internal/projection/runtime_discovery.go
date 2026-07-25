package projection

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

type RuntimeInstance struct {
	ID                         string
	DeviceID                   string
	AdapterType                string
	DisplayName                string
	ExecutableVersion          string
	Status                     string
	ObservedCapabilities       []string
	Capacity                   int
	ModelIDs                   []string
	DiscoveryDigest            string
	SourceProbeID              string
	DiscoveryID                string
	DiscoveredAt               time.Time
	DiscoveryEventID           string
	DiscoverySequence          int64
	StatusReconciliationID     string
	StatusReconciliationDigest string
	StatusBaselineDigest       string
	StatusDiscoveryDigest      string
	StatusSourceProbeID        string
	StatusChangedAt            time.Time
	StatusEventID              string
	StatusSequence             int64
	StatusPreviousEventID      string
	StatusPreviousSequence     int64
}

type runtimeDiscoveryProjectionPayload struct {
	DiscoveryDigest *string                             `json:"discovery_digest"`
	SourceProbeID   *string                             `json:"source_probe_id"`
	Instance        *runtimeDiscoveryProjectionInstance `json:"instance"`
	ModelIDs        json.RawMessage                     `json:"model_ids"`
}

type runtimeDiscoveryProjectionInstance struct {
	ID                   *string                    `json:"id"`
	DeviceID             *string                    `json:"device_id"`
	AdapterType          *string                    `json:"adapter_type"`
	DisplayName          *string                    `json:"display_name"`
	ExecutableVersion    *string                    `json:"executable_version"`
	Status               *loomruntime.RuntimeStatus `json:"status"`
	ObservedCapabilities json.RawMessage            `json:"observed_capabilities"`
	Capacity             *int                       `json:"capacity"`
}

func projectRuntimeDiscoveryEvent(event journal.Event) (RuntimeInstance, error) {
	if event.ID == "" ||
		event.IdempotencyKey == "" ||
		event.CorrelationID == "" ||
		event.CausationID != "" ||
		event.Seq <= 0 ||
		event.EmittedAt.IsZero() ||
		event.EmittedAt.Location() != time.UTC {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceDiscovered envelope",
			ErrInvalidProjectionEvent,
		)
	}

	var payload runtimeDiscoveryProjectionPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return RuntimeInstance{}, err
	}
	if payload.DiscoveryDigest == nil ||
		!validSHA256Digest(*payload.DiscoveryDigest) ||
		payload.SourceProbeID == nil ||
		*payload.SourceProbeID == "" ||
		payload.Instance == nil ||
		payload.ModelIDs == nil {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceDiscovered payload",
			ErrInvalidProjectionEvent,
		)
	}

	instance, err := runtimeInstanceFromDiscoveryPayload(*payload.Instance)
	if err != nil {
		return RuntimeInstance{}, err
	}
	if event.StreamID != "runtime_instance:"+instance.ID {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: RuntimeInstanceDiscovered stream mismatch",
			ErrInvalidProjectionEvent,
		)
	}

	var modelIDs []string
	if err := json.Unmarshal(payload.ModelIDs, &modelIDs); err != nil ||
		!validRuntimeProjectionModels(modelIDs) {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceDiscovered models",
			ErrInvalidProjectionEvent,
		)
	}
	modelIDs = cloneRuntimeProjectionStrings(modelIDs)

	return RuntimeInstance{
		ID:                   instance.ID,
		DeviceID:             instance.DeviceID,
		AdapterType:          instance.AdapterType,
		DisplayName:          instance.DisplayName,
		ExecutableVersion:    instance.ExecutableVersion,
		Status:               string(instance.Status),
		ObservedCapabilities: cloneRuntimeProjectionStrings(instance.ObservedCapabilities),
		Capacity:             instance.Capacity,
		ModelIDs:             modelIDs,
		DiscoveryDigest:      *payload.DiscoveryDigest,
		SourceProbeID:        *payload.SourceProbeID,
		DiscoveryID:          event.CorrelationID,
		DiscoveredAt:         event.EmittedAt,
		DiscoveryEventID:     event.ID,
		DiscoverySequence:    event.Seq,
	}, nil
}

func runtimeInstanceFromDiscoveryPayload(
	payload runtimeDiscoveryProjectionInstance,
) (loomruntime.RuntimeInstance, error) {
	if payload.ID == nil ||
		payload.DeviceID == nil ||
		payload.AdapterType == nil ||
		payload.DisplayName == nil ||
		payload.ExecutableVersion == nil ||
		payload.Status == nil ||
		payload.ObservedCapabilities == nil ||
		payload.Capacity == nil {
		return loomruntime.RuntimeInstance{}, fmt.Errorf(
			"%w: incomplete RuntimeInstanceDiscovered instance",
			ErrInvalidProjectionEvent,
		)
	}

	var capabilities []string
	if err := json.Unmarshal(payload.ObservedCapabilities, &capabilities); err != nil {
		return loomruntime.RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceDiscovered capabilities",
			ErrInvalidProjectionEvent,
		)
	}
	input := loomruntime.RuntimeInstance{
		ID:                   *payload.ID,
		DeviceID:             *payload.DeviceID,
		AdapterType:          *payload.AdapterType,
		DisplayName:          *payload.DisplayName,
		ExecutableVersion:    *payload.ExecutableVersion,
		Status:               *payload.Status,
		ObservedCapabilities: capabilities,
		Capacity:             *payload.Capacity,
	}
	normalized, err := loomruntime.NewRuntimeInstance(input)
	if err != nil || !reflect.DeepEqual(normalized, input) {
		return loomruntime.RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceDiscovered instance",
			ErrInvalidProjectionEvent,
		)
	}
	return normalized, nil
}

func validRuntimeProjectionModels(modelIDs []string) bool {
	for index, modelID := range modelIDs {
		if modelID == "" || index > 0 && modelID <= modelIDs[index-1] {
			return false
		}
	}
	return true
}

func cloneProjectedRuntimeInstance(input RuntimeInstance) RuntimeInstance {
	input.ObservedCapabilities = cloneRuntimeProjectionStrings(input.ObservedCapabilities)
	input.ModelIDs = cloneRuntimeProjectionStrings(input.ModelIDs)
	return input
}

func cloneRuntimeProjectionStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	return append([]string(nil), input...)
}
