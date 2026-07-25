package projection

import (
	"errors"
	"math"
	"reflect"
	"sort"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const maxRuntimeStatusBaselineProjectionEntries = 32

var ErrInvalidRuntimeStatusBaselineProjection = errors.New(
	"invalid runtime status baseline projection",
)

func BuildRuntimeStatusBaselines(
	snapshot Snapshot,
) ([]loomruntime.RuntimeStatusBaseline, error) {
	if len(snapshot.RuntimeInstances) > maxRuntimeStatusBaselineProjectionEntries {
		return nil, ErrInvalidRuntimeStatusBaselineProjection
	}

	result := make([]loomruntime.RuntimeStatusBaseline, 0, len(snapshot.RuntimeInstances))
	eventOwners := make(map[string]string, len(snapshot.RuntimeInstances)*3)
	for key, projected := range snapshot.RuntimeInstances {
		if key == "" || key != projected.ID {
			return nil, ErrInvalidRuntimeStatusBaselineProjection
		}
		instance := loomruntime.RuntimeInstance{
			ID:                   projected.ID,
			DeviceID:             projected.DeviceID,
			AdapterType:          projected.AdapterType,
			DisplayName:          projected.DisplayName,
			ExecutableVersion:    projected.ExecutableVersion,
			Status:               loomruntime.RuntimeStatus(projected.Status),
			ObservedCapabilities: append([]string(nil), projected.ObservedCapabilities...),
			Capacity:             projected.Capacity,
		}
		normalized, err := loomruntime.NewRuntimeInstance(instance)
		if err != nil || !reflect.DeepEqual(normalized, instance) ||
			!validRuntimeProjectionModels(projected.ModelIDs) ||
			!validRuntimeStatusBaselineDiscovery(projected) {
			return nil, ErrInvalidRuntimeStatusBaselineProjection
		}

		previousEventID, previousSequence, ok :=
			runtimeStatusBaselinePreviousFact(projected)
		if !ok {
			return nil, ErrInvalidRuntimeStatusBaselineProjection
		}
		if !claimRuntimeStatusBaselineEvent(
			eventOwners,
			projected.DiscoveryEventID,
			projected.ID,
		) {
			return nil, ErrInvalidRuntimeStatusBaselineProjection
		}
		if !runtimeStatusBaselineStatusFieldsZero(projected) &&
			(!claimRuntimeStatusBaselineEvent(
				eventOwners,
				projected.StatusEventID,
				projected.ID,
			) ||
				!claimRuntimeStatusBaselineEvent(
					eventOwners,
					projected.StatusPreviousEventID,
					projected.ID,
				)) {
			return nil, ErrInvalidRuntimeStatusBaselineProjection
		}
		result = append(result, loomruntime.RuntimeStatusBaseline{
			Instance:         normalized,
			PreviousEventID:  previousEventID,
			PreviousSequence: previousSequence,
		})
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Instance.ID < result[right].Instance.ID
	})
	return result, nil
}

func validRuntimeStatusBaselineDiscovery(input RuntimeInstance) bool {
	return validSHA256Digest(input.DiscoveryDigest) &&
		input.SourceProbeID != "" &&
		input.DiscoveryID != "" &&
		!input.DiscoveredAt.IsZero() &&
		input.DiscoveredAt.Location() == time.UTC &&
		input.DiscoveryEventID != "" &&
		input.DiscoverySequence > 0
}

func runtimeStatusBaselinePreviousFact(
	input RuntimeInstance,
) (string, int64, bool) {
	if runtimeStatusBaselineStatusFieldsZero(input) {
		return input.DiscoveryEventID, input.DiscoverySequence, true
	}
	if input.StatusReconciliationID == "" ||
		!validSHA256Digest(input.StatusReconciliationDigest) ||
		!validSHA256Digest(input.StatusBaselineDigest) ||
		!validSHA256Digest(input.StatusDiscoveryDigest) ||
		input.StatusSourceProbeID == "" ||
		input.StatusChangedAt.IsZero() ||
		input.StatusChangedAt.Location() != time.UTC ||
		input.StatusEventID == "" ||
		input.StatusSequence <= 0 ||
		input.StatusPreviousEventID == "" ||
		input.StatusPreviousSequence <= 0 ||
		input.StatusPreviousSequence == math.MaxInt64 ||
		input.StatusSequence != input.StatusPreviousSequence+1 ||
		input.StatusPreviousSequence < input.DiscoverySequence ||
		(input.StatusPreviousEventID == input.DiscoveryEventID) !=
			(input.StatusPreviousSequence == input.DiscoverySequence) ||
		input.StatusEventID == input.StatusPreviousEventID ||
		input.StatusEventID == input.DiscoveryEventID {
		return "", 0, false
	}
	return input.StatusEventID, input.StatusSequence, true
}

func runtimeStatusBaselineStatusFieldsZero(input RuntimeInstance) bool {
	return input.StatusReconciliationID == "" &&
		input.StatusReconciliationDigest == "" &&
		input.StatusBaselineDigest == "" &&
		input.StatusDiscoveryDigest == "" &&
		input.StatusSourceProbeID == "" &&
		input.StatusChangedAt.IsZero() &&
		input.StatusEventID == "" &&
		input.StatusSequence == 0 &&
		input.StatusPreviousEventID == "" &&
		input.StatusPreviousSequence == 0
}

func claimRuntimeStatusBaselineEvent(
	owners map[string]string,
	eventID string,
	runtimeID string,
) bool {
	owner, exists := owners[eventID]
	if exists && owner != runtimeID {
		return false
	}
	owners[eventID] = runtimeID
	return true
}
