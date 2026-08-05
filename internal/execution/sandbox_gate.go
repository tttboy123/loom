package execution

import (
	"context"

	"loom-pi-rebuild/internal/sandbox"
)

// SandboxPolicy is the per-Job execution isolation policy (Phase 3B). When
// Required is true the execution adapter must route through a sandbox backend;
// an unavailable backend fails closed (never falls back to local execution).
type SandboxPolicy struct {
	Required bool
	Backend  string
}

// SandboxGate resolves the per-Job policy and backend availability. It is
// advisory-free: nil means no sandbox requirement (default local executor).
type SandboxGate interface {
	ResolvePolicy(context.Context, string) (SandboxPolicy, error)
	BackendAvailable(context.Context, string) (bool, error)
}

// BackendGate is the default product gate: policy is resolved by a caller
// supplied function (e.g. from a Job profile or experimental config) and
// availability is decided by the backend's InspectCapabilities. A backend
// that does not advertise the requested capability is treated as unavailable,
// which makes Required policies fail closed.
type BackendGate struct {
	backend sandbox.SandboxBackend
	resolve func(context.Context, string) (SandboxPolicy, error)
	name    string
}

func NewBackendGate(
	backend sandbox.SandboxBackend,
	resolve func(context.Context, string) (SandboxPolicy, error),
) *BackendGate {
	return &BackendGate{backend: backend, resolve: resolve, name: "loopback"}
}

func (gate *BackendGate) ResolvePolicy(ctx context.Context, jobID string) (SandboxPolicy, error) {
	if gate.resolve == nil {
		return SandboxPolicy{}, nil
	}
	return gate.resolve(ctx, jobID)
}

func (gate *BackendGate) BackendAvailable(ctx context.Context, backendName string) (bool, error) {
	if gate.backend == nil {
		return false, nil
	}
	capabilities, err := gate.backend.InspectCapabilities(ctx)
	if err != nil {
		return false, err
	}
	// The policy may require a specific mounted backend by name; an unknown
	// or unmounted backend is unavailable (fail closed). The capability
	// inspection additionally proves the backend can actually execute.
	if backendName != "" && gate.name != backendName {
		return false, nil
	}
	return containsCapability(capabilities.Supported, "exec"), nil
}

func containsCapability(supported []string, name string) bool {
	for _, capability := range supported {
		if capability == name {
			return true
		}
	}
	return false
}
