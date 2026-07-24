package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
)

var (
	ErrInvalidRuntimeProbe      = errors.New("invalid runtime probe")
	ErrDuplicateRuntimeProbe    = errors.New("duplicate runtime probe")
	ErrRuntimeDiscoveryFailed   = errors.New("runtime discovery failed")
	ErrInvalidRuntimeModel      = errors.New("invalid runtime model")
	ErrDuplicateRuntimeModel    = errors.New("duplicate runtime model")
	ErrDuplicateRuntimeInstance = errors.New("duplicate runtime instance")
)

type RuntimeProbe interface {
	ID() string
	ObserveRuntime(context.Context) ([]RuntimeObservation, error)
}

type RuntimeObservation struct {
	SourceProbeID string
	Instance      RuntimeInstance
	ModelIDs      []string
}

type RuntimeDiscoverySnapshot struct {
	digest       string
	observations []RuntimeObservation
}

func (s RuntimeDiscoverySnapshot) Digest() string {
	return s.digest
}

func (s RuntimeDiscoverySnapshot) Observations() []RuntimeObservation {
	return copyRuntimeObservations(s.observations)
}

func DiscoverRuntime(ctx context.Context, probes []RuntimeProbe) (RuntimeDiscoverySnapshot, error) {
	ordered, err := validateRuntimeProbes(probes)
	if err != nil {
		return RuntimeDiscoverySnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return RuntimeDiscoverySnapshot{}, err
	}

	observations := make([]RuntimeObservation, 0)
	for _, probe := range ordered {
		if err := ctx.Err(); err != nil {
			return RuntimeDiscoverySnapshot{}, err
		}

		probeObservations, err := probe.probe.ObserveRuntime(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return RuntimeDiscoverySnapshot{}, err
			}
			return RuntimeDiscoverySnapshot{}, fmt.Errorf("%w: probe %q: %w", ErrRuntimeDiscoveryFailed, probe.id, err)
		}
		if err := ctx.Err(); err != nil {
			return RuntimeDiscoverySnapshot{}, err
		}

		for _, observation := range probeObservations {
			normalized, err := normalizeRuntimeObservation(probe.id, observation)
			if err != nil {
				return RuntimeDiscoverySnapshot{}, err
			}
			observations = append(observations, normalized)
		}
	}

	sort.Slice(observations, func(i, j int) bool {
		return observations[i].Instance.ID < observations[j].Instance.ID
	})
	if err := validateUniqueRuntimeInstances(observations); err != nil {
		return RuntimeDiscoverySnapshot{}, err
	}

	digest, err := digestRuntimeDiscovery(observations)
	if err != nil {
		return RuntimeDiscoverySnapshot{}, err
	}

	return RuntimeDiscoverySnapshot{
		digest:       digest,
		observations: copyRuntimeObservations(observations),
	}, nil
}

type runtimeProbeCandidate struct {
	id    string
	probe RuntimeProbe
}

func validateRuntimeProbes(probes []RuntimeProbe) ([]runtimeProbeCandidate, error) {
	ordered := make([]runtimeProbeCandidate, 0, len(probes))
	seen := make(map[string]struct{}, len(probes))
	for _, probe := range probes {
		if isNilRuntimeProbe(probe) {
			return nil, ErrInvalidRuntimeProbe
		}
		id := probe.ID()
		if id == "" {
			return nil, fmt.Errorf("%w: empty id", ErrInvalidRuntimeProbe)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateRuntimeProbe, id)
		}
		seen[id] = struct{}{}
		ordered = append(ordered, runtimeProbeCandidate{
			id:    id,
			probe: probe,
		})
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].id < ordered[j].id
	})
	return ordered, nil
}

func isNilRuntimeProbe(probe RuntimeProbe) bool {
	if probe == nil {
		return true
	}
	value := reflect.ValueOf(probe)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func normalizeRuntimeObservation(probeID string, observation RuntimeObservation) (RuntimeObservation, error) {
	instance, err := NewRuntimeInstance(observation.Instance)
	if err != nil {
		return RuntimeObservation{}, err
	}
	models, err := normalizeRuntimeModels(observation.ModelIDs)
	if err != nil {
		return RuntimeObservation{}, err
	}
	return RuntimeObservation{
		SourceProbeID: probeID,
		Instance:      instance,
		ModelIDs:      models,
	}, nil
}

func normalizeRuntimeModels(modelIDs []string) ([]string, error) {
	if len(modelIDs) == 0 {
		return nil, nil
	}

	models := append([]string(nil), modelIDs...)
	sort.Strings(models)
	for i, modelID := range models {
		if modelID == "" {
			return nil, fmt.Errorf("%w: empty id", ErrInvalidRuntimeModel)
		}
		if i > 0 && modelID == models[i-1] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateRuntimeModel, modelID)
		}
	}
	return models, nil
}

func validateUniqueRuntimeInstances(observations []RuntimeObservation) error {
	for i := 1; i < len(observations); i++ {
		if observations[i].Instance.ID == observations[i-1].Instance.ID {
			return fmt.Errorf("%w: %s", ErrDuplicateRuntimeInstance, observations[i].Instance.ID)
		}
	}
	return nil
}

func digestRuntimeDiscovery(observations []RuntimeObservation) (string, error) {
	canonical := runtimeDiscoveryCanonical{
		Observations: make([]runtimeDiscoveryCanonicalObservation, 0, len(observations)),
	}
	for _, observation := range observations {
		canonical.Observations = append(canonical.Observations, runtimeDiscoveryCanonicalObservation{
			SourceProbeID:     observation.SourceProbeID,
			RuntimeInstanceID: observation.Instance.ID,
			DeviceID:          observation.Instance.DeviceID,
			AdapterType:       observation.Instance.AdapterType,
			DisplayName:       observation.Instance.DisplayName,
			ExecutableVersion: observation.Instance.ExecutableVersion,
			Status:            string(observation.Instance.Status),
			Capabilities:      append([]string(nil), observation.Instance.ObservedCapabilities...),
			Capacity:          observation.Instance.Capacity,
			ModelIDs:          append([]string(nil), observation.ModelIDs...),
		})
	}

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("%w: canonical encoding: %w", ErrRuntimeDiscoveryFailed, err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

type runtimeDiscoveryCanonical struct {
	Observations []runtimeDiscoveryCanonicalObservation `json:"observations"`
}

type runtimeDiscoveryCanonicalObservation struct {
	SourceProbeID     string   `json:"source_probe_id"`
	RuntimeInstanceID string   `json:"runtime_instance_id"`
	DeviceID          string   `json:"device_id"`
	AdapterType       string   `json:"adapter_type"`
	DisplayName       string   `json:"display_name"`
	ExecutableVersion string   `json:"executable_version"`
	Status            string   `json:"status"`
	Capabilities      []string `json:"capabilities"`
	Capacity          int      `json:"capacity"`
	ModelIDs          []string `json:"model_ids"`
}

func copyRuntimeObservations(observations []RuntimeObservation) []RuntimeObservation {
	if len(observations) == 0 {
		return []RuntimeObservation{}
	}

	copied := make([]RuntimeObservation, len(observations))
	for i, observation := range observations {
		copied[i] = RuntimeObservation{
			SourceProbeID: observation.SourceProbeID,
			Instance:      copyRuntimeInstance(observation.Instance),
			ModelIDs:      append([]string(nil), observation.ModelIDs...),
		}
	}
	return copied
}
