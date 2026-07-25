package projection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"loom-pi-rebuild/internal/journal"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

const runtimeStatusProjectionEventType = "RuntimeInstanceStatusChanged"

type runtimeStatusProjectionPayload struct {
	ReconciliationDigest  *string                    `json:"reconciliation_digest"`
	BaselineDigest        *string                    `json:"baseline_digest"`
	SourceDiscoveryDigest *string                    `json:"source_discovery_digest"`
	SourceProbeID         *string                    `json:"source_probe_id"`
	RuntimeInstanceID     *string                    `json:"runtime_instance_id"`
	DeviceID              *string                    `json:"device_id"`
	AdapterType           *string                    `json:"adapter_type"`
	FromStatus            *loomruntime.RuntimeStatus `json:"from_status"`
	ToStatus              *loomruntime.RuntimeStatus `json:"to_status"`
	PreviousEventID       *string                    `json:"previous_event_id"`
	PreviousSequence      *int64                     `json:"previous_sequence"`
}

func projectRuntimeStatusEvent(
	event journal.Event,
	existing RuntimeInstance,
) (RuntimeInstance, error) {
	if event.Type != runtimeStatusProjectionEventType ||
		event.SchemaVersion != 1 ||
		event.ID == "" ||
		event.IdempotencyKey == "" ||
		event.CorrelationID == "" ||
		event.CausationID == "" ||
		event.Seq <= 0 ||
		event.EmittedAt.IsZero() ||
		event.EmittedAt.Location() != time.UTC {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceStatusChanged envelope",
			ErrInvalidProjectionEvent,
		)
	}
	if err := rejectDuplicateRuntimeStatusPayloadFields(event.PayloadJSON); err != nil {
		return RuntimeInstance{}, err
	}

	var payload runtimeStatusProjectionPayload
	if err := decodeExactProjectionPayload(event, &payload); err != nil {
		return RuntimeInstance{}, err
	}
	if payload.ReconciliationDigest == nil ||
		payload.BaselineDigest == nil ||
		payload.SourceDiscoveryDigest == nil ||
		payload.SourceProbeID == nil ||
		payload.RuntimeInstanceID == nil ||
		payload.DeviceID == nil ||
		payload.AdapterType == nil ||
		payload.FromStatus == nil ||
		payload.ToStatus == nil ||
		payload.PreviousEventID == nil ||
		payload.PreviousSequence == nil {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: incomplete RuntimeInstanceStatusChanged payload",
			ErrInvalidProjectionEvent,
		)
	}
	if !validSHA256Digest(*payload.ReconciliationDigest) ||
		!validSHA256Digest(*payload.BaselineDigest) ||
		!validSHA256Digest(*payload.SourceDiscoveryDigest) ||
		*payload.SourceProbeID == "" ||
		*payload.RuntimeInstanceID == "" ||
		*payload.DeviceID == "" ||
		*payload.AdapterType == "" ||
		!validRuntimeProjectionStatus(*payload.FromStatus) ||
		!validRuntimeProjectionStatus(*payload.ToStatus) ||
		*payload.FromStatus == *payload.ToStatus ||
		*payload.PreviousEventID == "" ||
		*payload.PreviousSequence <= 0 ||
		*payload.PreviousSequence == math.MaxInt64 ||
		event.Seq != *payload.PreviousSequence+1 ||
		event.CausationID != *payload.PreviousEventID {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: invalid RuntimeInstanceStatusChanged facts",
			ErrInvalidProjectionEvent,
		)
	}
	if existing.ID == "" ||
		event.StreamID != "runtime_instance:"+*payload.RuntimeInstanceID ||
		existing.ID != *payload.RuntimeInstanceID ||
		existing.DeviceID != *payload.DeviceID ||
		existing.AdapterType != *payload.AdapterType ||
		existing.Status != string(*payload.FromStatus) {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: RuntimeInstanceStatusChanged state mismatch",
			ErrInvalidProjectionEvent,
		)
	}

	previousEventID, previousSequence, ok := runtimeStatusBearingFact(existing)
	if !ok ||
		previousEventID != *payload.PreviousEventID ||
		previousSequence != *payload.PreviousSequence {
		return RuntimeInstance{}, fmt.Errorf(
			"%w: RuntimeInstanceStatusChanged stale provenance",
			ErrInvalidProjectionEvent,
		)
	}

	result := cloneProjectedRuntimeInstance(existing)
	result.Status = string(*payload.ToStatus)
	result.StatusReconciliationID = event.CorrelationID
	result.StatusReconciliationDigest = *payload.ReconciliationDigest
	result.StatusBaselineDigest = *payload.BaselineDigest
	result.StatusDiscoveryDigest = *payload.SourceDiscoveryDigest
	result.StatusSourceProbeID = *payload.SourceProbeID
	result.StatusChangedAt = event.EmittedAt
	result.StatusEventID = event.ID
	result.StatusSequence = event.Seq
	result.StatusPreviousEventID = *payload.PreviousEventID
	result.StatusPreviousSequence = *payload.PreviousSequence
	return result, nil
}

func runtimeStatusBearingFact(
	instance RuntimeInstance,
) (string, int64, bool) {
	if instance.StatusEventID != "" || instance.StatusSequence != 0 {
		if instance.StatusEventID == "" || instance.StatusSequence <= 0 {
			return "", 0, false
		}
		return instance.StatusEventID, instance.StatusSequence, true
	}
	if instance.DiscoveryEventID == "" || instance.DiscoverySequence <= 0 {
		return "", 0, false
	}
	return instance.DiscoveryEventID, instance.DiscoverySequence, true
}

func runtimeInstanceIDFromStream(streamID string) string {
	const prefix = "runtime_instance:"
	if len(streamID) <= len(prefix) || streamID[:len(prefix)] != prefix {
		return ""
	}
	return streamID[len(prefix):]
}

func validRuntimeProjectionStatus(status loomruntime.RuntimeStatus) bool {
	switch status {
	case loomruntime.RuntimeOnline,
		loomruntime.RuntimeOffline,
		loomruntime.RuntimeIncompatible,
		loomruntime.RuntimeDisabled:
		return true
	default:
		return false
	}
}

func rejectDuplicateRuntimeStatusPayloadFields(payload []byte) error {
	if !json.Valid(payload) {
		return fmt.Errorf("%w: malformed payload", ErrInvalidProjectionEvent)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf(
			"%w: invalid RuntimeInstanceStatusChanged payload",
			ErrInvalidProjectionEvent,
		)
	}
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidProjectionEvent, err)
		}
		field, ok := token.(string)
		if !ok {
			return fmt.Errorf(
				"%w: invalid RuntimeInstanceStatusChanged field",
				ErrInvalidProjectionEvent,
			)
		}
		if _, exists := seen[field]; exists {
			return fmt.Errorf(
				"%w: duplicate RuntimeInstanceStatusChanged field %s",
				ErrInvalidProjectionEvent,
				field,
			)
		}
		seen[field] = struct{}{}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidProjectionEvent, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProjectionEvent, err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf(
			"%w: trailing RuntimeInstanceStatusChanged payload",
			ErrInvalidProjectionEvent,
		)
	}
	return nil
}
