package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"

	"loom-pi-rebuild/internal/projection"
	loomruntime "loom-pi-rebuild/internal/runtime"
)

type RuntimeObservationWriteKind string

const (
	RuntimeObservationWriteNone      RuntimeObservationWriteKind = "none"
	RuntimeObservationWriteDiscovery RuntimeObservationWriteKind = "discovery"
	RuntimeObservationWriteStatus    RuntimeObservationWriteKind = "status"
)

var ErrInvalidRuntimeObservationWritePlan = errors.New(
	"invalid runtime observation write plan",
)

type RuntimeObservationWritePlanCandidate struct {
	planned               bool
	kind                  RuntimeObservationWriteKind
	sourceDiscoveryDigest string
	projectedRuntimeCount int
	observedRuntimeCount  int
	inventoryChangeCount  int
	statusTransitionCount int
	candidateDigest       string
}

func PlanRuntimeObservationWrite(
	ctx context.Context,
	projected projection.Snapshot,
	current loomruntime.RuntimeDiscoverySnapshot,
) (RuntimeObservationWritePlanCandidate, error) {
	if ctx == nil {
		return RuntimeObservationWritePlanCandidate{},
			ErrInvalidRuntimeObservationWritePlan
	}
	if err := ctx.Err(); err != nil {
		return RuntimeObservationWritePlanCandidate{}, err
	}

	baseline, err := projection.BuildRuntimeStatusBaselines(projected)
	if err != nil {
		return RuntimeObservationWritePlanCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeObservationWritePlanCandidate{}, err
	}
	reconciliation, err := loomruntime.ReconcileObservedRuntimeStatuses(
		ctx, baseline, current,
	)
	if err != nil {
		return RuntimeObservationWritePlanCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeObservationWritePlanCandidate{}, err
	}

	observations := current.Observations()
	inventoryChangeCount := 0
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return RuntimeObservationWritePlanCandidate{}, err
		}
		projectedRuntime, exists :=
			projected.RuntimeInstances[observation.Instance.ID]
		if !exists ||
			!sameRuntimeObservationInventory(projectedRuntime, observation) {
			inventoryChangeCount++
		}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeObservationWritePlanCandidate{}, err
	}

	kind := RuntimeObservationWriteNone
	if inventoryChangeCount > 0 {
		kind = RuntimeObservationWriteDiscovery
	} else if reconciliation.TransitionCount() > 0 {
		kind = RuntimeObservationWriteStatus
	}
	candidate := RuntimeObservationWritePlanCandidate{
		planned:               true,
		kind:                  kind,
		sourceDiscoveryDigest: current.Digest(),
		projectedRuntimeCount: len(projected.RuntimeInstances),
		observedRuntimeCount:  len(observations),
		inventoryChangeCount:  inventoryChangeCount,
		statusTransitionCount: reconciliation.TransitionCount(),
	}
	candidate.candidateDigest, err =
		digestRuntimeObservationWritePlan(candidate)
	if err != nil || !validRuntimeObservationWritePlan(candidate) {
		return RuntimeObservationWritePlanCandidate{},
			ErrInvalidRuntimeObservationWritePlan
	}
	return candidate, nil
}

func (c RuntimeObservationWritePlanCandidate) Planned() bool {
	return c.planned
}

func (c RuntimeObservationWritePlanCandidate) Kind() RuntimeObservationWriteKind {
	return c.kind
}

func (c RuntimeObservationWritePlanCandidate) SourceDiscoveryDigest() string {
	return c.sourceDiscoveryDigest
}

func (c RuntimeObservationWritePlanCandidate) ProjectedRuntimeCount() int {
	return c.projectedRuntimeCount
}

func (c RuntimeObservationWritePlanCandidate) ObservedRuntimeCount() int {
	return c.observedRuntimeCount
}

func (c RuntimeObservationWritePlanCandidate) InventoryChangeCount() int {
	return c.inventoryChangeCount
}

func (c RuntimeObservationWritePlanCandidate) StatusTransitionCount() int {
	return c.statusTransitionCount
}

func (c RuntimeObservationWritePlanCandidate) CandidateDigest() string {
	return c.candidateDigest
}

func sameRuntimeObservationInventory(
	projected projection.RuntimeInstance,
	current loomruntime.RuntimeObservation,
) bool {
	return projected.DisplayName == current.Instance.DisplayName &&
		projected.ExecutableVersion == current.Instance.ExecutableVersion &&
		reflect.DeepEqual(
			projected.ObservedCapabilities,
			current.Instance.ObservedCapabilities,
		) &&
		projected.Capacity == current.Instance.Capacity &&
		reflect.DeepEqual(projected.ModelIDs, current.ModelIDs) &&
		projected.SourceProbeID == current.SourceProbeID
}

type runtimeObservationWritePlanDigestPayload struct {
	Version               int                         `json:"version"`
	Planned               bool                        `json:"planned"`
	Kind                  RuntimeObservationWriteKind `json:"kind"`
	SourceDiscoveryDigest string                      `json:"source_discovery_digest"`
	ProjectedRuntimeCount int                         `json:"projected_runtime_count"`
	ObservedRuntimeCount  int                         `json:"observed_runtime_count"`
	InventoryChangeCount  int                         `json:"inventory_change_count"`
	StatusTransitionCount int                         `json:"status_transition_count"`
}

func digestRuntimeObservationWritePlan(
	candidate RuntimeObservationWritePlanCandidate,
) (string, error) {
	encoded, err := json.Marshal(runtimeObservationWritePlanDigestPayload{
		Version:               1,
		Planned:               candidate.planned,
		Kind:                  candidate.kind,
		SourceDiscoveryDigest: candidate.sourceDiscoveryDigest,
		ProjectedRuntimeCount: candidate.projectedRuntimeCount,
		ObservedRuntimeCount:  candidate.observedRuntimeCount,
		InventoryChangeCount:  candidate.inventoryChangeCount,
		StatusTransitionCount: candidate.statusTransitionCount,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validRuntimeObservationWritePlan(
	candidate RuntimeObservationWritePlanCandidate,
) bool {
	if !candidate.planned ||
		!validRuntimeObservationWriteKind(candidate.kind) ||
		!validRuntimeObservationWriteDigest(candidate.sourceDiscoveryDigest) ||
		candidate.projectedRuntimeCount < 0 ||
		candidate.projectedRuntimeCount > 32 ||
		candidate.observedRuntimeCount < 0 ||
		candidate.observedRuntimeCount > 32 ||
		candidate.inventoryChangeCount < 0 ||
		candidate.inventoryChangeCount > candidate.observedRuntimeCount ||
		candidate.statusTransitionCount < 0 ||
		candidate.statusTransitionCount > candidate.observedRuntimeCount ||
		!validRuntimeObservationWriteClassification(candidate) ||
		!validRuntimeObservationWriteDigest(candidate.candidateDigest) {
		return false
	}
	digest, err := digestRuntimeObservationWritePlan(candidate)
	return err == nil && digest == candidate.candidateDigest
}

func validRuntimeObservationWriteKind(
	kind RuntimeObservationWriteKind,
) bool {
	return kind == RuntimeObservationWriteNone ||
		kind == RuntimeObservationWriteDiscovery ||
		kind == RuntimeObservationWriteStatus
}

func validRuntimeObservationWriteClassification(
	candidate RuntimeObservationWritePlanCandidate,
) bool {
	switch {
	case candidate.inventoryChangeCount > 0:
		return candidate.kind == RuntimeObservationWriteDiscovery
	case candidate.statusTransitionCount > 0:
		return candidate.kind == RuntimeObservationWriteStatus
	default:
		return candidate.kind == RuntimeObservationWriteNone
	}
}

func validRuntimeObservationWriteDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil &&
		len(decoded) == sha256.Size &&
		hex.EncodeToString(decoded) == value
}
