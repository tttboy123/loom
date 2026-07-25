package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
)

const (
	maxRuntimeStatusReconciliationEntries = 32
	runtimeStatusBaselineDigestVersion    = 2
	runtimeStatusCandidateDigestVersion   = 1
)

var (
	ErrInvalidRuntimeStatusReconciliation = errors.New("invalid runtime status reconciliation")
	ErrInvalidRuntimeStatusBaseline       = errors.New("invalid runtime status baseline")
	ErrInvalidRuntimeStatusSource         = errors.New("invalid runtime status source")
	ErrRuntimeStatusIdentityDrift         = errors.New("runtime status identity drift")
	ErrRuntimeStatusCandidateDigest       = errors.New("runtime status candidate digest mismatch")
)

type RuntimeStatusBaseline struct {
	Instance         RuntimeInstance
	PreviousEventID  string
	PreviousSequence int64
}

type RuntimeStatusTransition struct {
	RuntimeInstanceID     string
	DeviceID              string
	AdapterType           string
	FromStatus            RuntimeStatus
	ToStatus              RuntimeStatus
	SourceProbeID         string
	SourceDiscoveryDigest string
	PreviousEventID       string
	PreviousSequence      int64
}

type RuntimeStatusReconciliationCandidate struct {
	reconciled            bool
	baselineDigest        string
	sourceDiscoveryDigest string
	transitions           []RuntimeStatusTransition
	transitionCount       int
	candidateDigest       string
}

func ReconcileObservedRuntimeStatuses(
	ctx context.Context,
	baseline []RuntimeStatusBaseline,
	current RuntimeDiscoverySnapshot,
) (RuntimeStatusReconciliationCandidate, error) {
	if ctx == nil {
		return RuntimeStatusReconciliationCandidate{}, ErrInvalidRuntimeStatusReconciliation
	}
	if err := ctx.Err(); err != nil {
		return RuntimeStatusReconciliationCandidate{}, err
	}

	orderedBaseline, err := validateRuntimeStatusBaseline(baseline)
	if err != nil {
		return RuntimeStatusReconciliationCandidate{}, err
	}
	observations, err := validateRuntimeStatusSource(current)
	if err != nil {
		return RuntimeStatusReconciliationCandidate{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeStatusReconciliationCandidate{}, err
	}

	baselineDigest, err := digestRuntimeStatusBaseline(orderedBaseline)
	if err != nil {
		return RuntimeStatusReconciliationCandidate{}, ErrRuntimeStatusCandidateDigest
	}
	baselineByID := make(map[string]RuntimeStatusBaseline, len(orderedBaseline))
	for _, entry := range orderedBaseline {
		baselineByID[entry.Instance.ID] = entry
	}

	transitions := make([]RuntimeStatusTransition, 0)
	for _, observation := range observations {
		if err := ctx.Err(); err != nil {
			return RuntimeStatusReconciliationCandidate{}, err
		}
		previous, exists := baselineByID[observation.Instance.ID]
		if !exists {
			continue
		}
		if previous.Instance.DeviceID != observation.Instance.DeviceID ||
			previous.Instance.AdapterType != observation.Instance.AdapterType {
			return RuntimeStatusReconciliationCandidate{}, ErrRuntimeStatusIdentityDrift
		}
		if previous.Instance.Status == observation.Instance.Status {
			continue
		}
		transitions = append(transitions, RuntimeStatusTransition{
			RuntimeInstanceID:     observation.Instance.ID,
			DeviceID:              observation.Instance.DeviceID,
			AdapterType:           observation.Instance.AdapterType,
			FromStatus:            previous.Instance.Status,
			ToStatus:              observation.Instance.Status,
			SourceProbeID:         observation.SourceProbeID,
			SourceDiscoveryDigest: current.Digest(),
			PreviousEventID:       previous.PreviousEventID,
			PreviousSequence:      previous.PreviousSequence,
		})
	}

	candidate := RuntimeStatusReconciliationCandidate{
		reconciled:            true,
		baselineDigest:        baselineDigest,
		sourceDiscoveryDigest: current.Digest(),
		transitions:           cloneRuntimeStatusTransitions(transitions),
		transitionCount:       len(transitions),
	}
	candidate.candidateDigest, err = digestRuntimeStatusCandidate(candidate)
	if err != nil {
		return RuntimeStatusReconciliationCandidate{}, ErrRuntimeStatusCandidateDigest
	}
	if err := validateRuntimeStatusCandidate(candidate); err != nil {
		return RuntimeStatusReconciliationCandidate{}, err
	}
	return candidate, nil
}

func (c RuntimeStatusReconciliationCandidate) Reconciled() bool {
	return c.reconciled
}

func (c RuntimeStatusReconciliationCandidate) BaselineDigest() string {
	return c.baselineDigest
}

func (c RuntimeStatusReconciliationCandidate) SourceDiscoveryDigest() string {
	return c.sourceDiscoveryDigest
}

func (c RuntimeStatusReconciliationCandidate) Transitions() []RuntimeStatusTransition {
	return cloneRuntimeStatusTransitions(c.transitions)
}

func (c RuntimeStatusReconciliationCandidate) TransitionCount() int {
	return c.transitionCount
}

func (c RuntimeStatusReconciliationCandidate) CandidateDigest() string {
	return c.candidateDigest
}

func validateRuntimeStatusBaseline(
	input []RuntimeStatusBaseline,
) ([]RuntimeStatusBaseline, error) {
	if len(input) > maxRuntimeStatusReconciliationEntries {
		return nil, ErrInvalidRuntimeStatusBaseline
	}
	ordered := cloneRuntimeStatusBaselines(input)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].Instance.ID < ordered[right].Instance.ID
	})
	for index, entry := range ordered {
		normalized, err := NewRuntimeInstance(entry.Instance)
		if err != nil ||
			!reflect.DeepEqual(normalized, entry.Instance) ||
			entry.PreviousEventID == "" ||
			entry.PreviousSequence <= 0 ||
			index > 0 && entry.Instance.ID == ordered[index-1].Instance.ID {
			return nil, ErrInvalidRuntimeStatusBaseline
		}
	}
	return ordered, nil
}

func validateRuntimeStatusSource(
	input RuntimeDiscoverySnapshot,
) ([]RuntimeObservation, error) {
	if !validRuntimeStatusDigest(input.Digest()) {
		return nil, ErrInvalidRuntimeStatusSource
	}
	observations := cloneRuntimeStatusObservations(input.observations)
	if len(observations) > maxRuntimeStatusReconciliationEntries {
		return nil, ErrInvalidRuntimeStatusSource
	}
	for index, observation := range observations {
		if observation.SourceProbeID == "" {
			return nil, ErrInvalidRuntimeStatusSource
		}
		normalized, err := NewRuntimeInstance(observation.Instance)
		if err != nil || !reflect.DeepEqual(normalized, observation.Instance) {
			return nil, ErrInvalidRuntimeStatusSource
		}
		if !validRuntimeStatusModels(observation.ModelIDs) ||
			index > 0 &&
				observation.Instance.ID <= observations[index-1].Instance.ID {
			return nil, ErrInvalidRuntimeStatusSource
		}
	}
	digest, err := digestRuntimeDiscovery(observations)
	if err != nil || digest != input.Digest() {
		return nil, ErrInvalidRuntimeStatusSource
	}
	return copyRuntimeObservations(observations), nil
}

func validRuntimeStatusModels(input []string) bool {
	for index, modelID := range input {
		if modelID == "" || index > 0 && modelID <= input[index-1] {
			return false
		}
	}
	return true
}

type runtimeStatusBaselineDigestPayload struct {
	Version  int                           `json:"version"`
	Baseline []runtimeStatusBaselineRecord `json:"baseline"`
}

type runtimeStatusBaselineRecord struct {
	ID                   string        `json:"id"`
	DeviceID             string        `json:"device_id"`
	AdapterType          string        `json:"adapter_type"`
	DisplayName          string        `json:"display_name"`
	ExecutableVersion    string        `json:"executable_version"`
	Status               RuntimeStatus `json:"status"`
	ObservedCapabilities []string      `json:"observed_capabilities"`
	Capacity             int           `json:"capacity"`
	PreviousEventID      string        `json:"previous_event_id"`
	PreviousSequence     int64         `json:"previous_sequence"`
}

func digestRuntimeStatusBaseline(input []RuntimeStatusBaseline) (string, error) {
	records := make([]runtimeStatusBaselineRecord, len(input))
	for index, entry := range input {
		records[index] = runtimeStatusBaselineRecord{
			ID:                   entry.Instance.ID,
			DeviceID:             entry.Instance.DeviceID,
			AdapterType:          entry.Instance.AdapterType,
			DisplayName:          entry.Instance.DisplayName,
			ExecutableVersion:    entry.Instance.ExecutableVersion,
			Status:               entry.Instance.Status,
			ObservedCapabilities: append([]string(nil), entry.Instance.ObservedCapabilities...),
			Capacity:             entry.Instance.Capacity,
			PreviousEventID:      entry.PreviousEventID,
			PreviousSequence:     entry.PreviousSequence,
		}
	}
	encoded, err := json.Marshal(runtimeStatusBaselineDigestPayload{
		Version:  runtimeStatusBaselineDigestVersion,
		Baseline: records,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

type runtimeStatusCandidateDigestPayload struct {
	Version               int                       `json:"version"`
	BaselineDigest        string                    `json:"baseline_digest"`
	SourceDiscoveryDigest string                    `json:"source_discovery_digest"`
	Transitions           []RuntimeStatusTransition `json:"transitions"`
	TransitionCount       int                       `json:"transition_count"`
}

func digestRuntimeStatusCandidate(
	input RuntimeStatusReconciliationCandidate,
) (string, error) {
	encoded, err := json.Marshal(runtimeStatusCandidateDigestPayload{
		Version:               runtimeStatusCandidateDigestVersion,
		BaselineDigest:        input.baselineDigest,
		SourceDiscoveryDigest: input.sourceDiscoveryDigest,
		Transitions:           cloneRuntimeStatusTransitions(input.transitions),
		TransitionCount:       input.transitionCount,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateRuntimeStatusCandidate(
	input RuntimeStatusReconciliationCandidate,
) error {
	if !input.reconciled ||
		!validRuntimeStatusDigest(input.baselineDigest) ||
		!validRuntimeStatusDigest(input.sourceDiscoveryDigest) ||
		input.transitionCount != len(input.transitions) ||
		!validRuntimeStatusDigest(input.candidateDigest) {
		return ErrRuntimeStatusCandidateDigest
	}
	for index, transition := range input.transitions {
		if transition.RuntimeInstanceID == "" ||
			transition.DeviceID == "" ||
			transition.AdapterType == "" ||
			!validRuntimeStatusValue(transition.FromStatus) ||
			!validRuntimeStatusValue(transition.ToStatus) ||
			transition.FromStatus == transition.ToStatus ||
			transition.SourceProbeID == "" ||
			transition.SourceDiscoveryDigest != input.sourceDiscoveryDigest ||
			transition.PreviousEventID == "" ||
			transition.PreviousSequence <= 0 ||
			index > 0 &&
				transition.RuntimeInstanceID <= input.transitions[index-1].RuntimeInstanceID {
			return ErrRuntimeStatusCandidateDigest
		}
	}
	digest, err := digestRuntimeStatusCandidate(input)
	if err != nil || digest != input.candidateDigest {
		return ErrRuntimeStatusCandidateDigest
	}
	return nil
}

func validRuntimeStatusValue(input RuntimeStatus) bool {
	switch input {
	case RuntimeOnline, RuntimeOffline, RuntimeIncompatible, RuntimeDisabled:
		return true
	default:
		return false
	}
}

func validRuntimeStatusDigest(input string) bool {
	if len(input) != sha256.Size*2 {
		return false
	}
	for _, character := range input {
		switch {
		case character >= '0' && character <= '9':
		case character >= 'a' && character <= 'f':
		default:
			return false
		}
	}
	return true
}

func cloneRuntimeStatusBaselines(input []RuntimeStatusBaseline) []RuntimeStatusBaseline {
	result := make([]RuntimeStatusBaseline, len(input))
	for index, entry := range input {
		entry.Instance.ObservedCapabilities = append(
			[]string(nil),
			entry.Instance.ObservedCapabilities...,
		)
		result[index] = entry
	}
	return result
}

func cloneRuntimeStatusTransitions(
	input []RuntimeStatusTransition,
) []RuntimeStatusTransition {
	return append([]RuntimeStatusTransition(nil), input...)
}

func cloneRuntimeStatusObservations(input []RuntimeObservation) []RuntimeObservation {
	result := make([]RuntimeObservation, len(input))
	for index, observation := range input {
		observation.Instance.ObservedCapabilities = append(
			[]string(nil),
			observation.Instance.ObservedCapabilities...,
		)
		observation.ModelIDs = append([]string(nil), observation.ModelIDs...)
		result[index] = observation
	}
	return result
}

func cloneRuntimeStatusCandidate(
	input RuntimeStatusReconciliationCandidate,
) RuntimeStatusReconciliationCandidate {
	input.transitions = cloneRuntimeStatusTransitions(input.transitions)
	return input
}
