// Package enrollment materializes a persisted, trusted Remote Tool Backend
// Enrollment into a typed Loom-owned remote tool executor. It is the only
// boundary that may construct a Work Bundle client from an Enrollment record;
// authorization and execution remain outside this package.
package enrollment

import (
	"errors"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/toolbroker"
	"loom-pi-rebuild/internal/work"
)

var (
	ErrToolEnrollmentInvalid            = errors.New("invalid tool enrollment")
	ErrToolEnrollmentRevoked            = errors.New("tool enrollment revoked")
	ErrToolEnrollmentPolicyDrift        = errors.New("tool enrollment policy drift")
	ErrToolEnrollmentAdapterUnsupported = errors.New("tool enrollment adapter unsupported")
	ErrToolEnrollmentPortUnavailable    = errors.New("tool enrollment port unavailable")
)

// MaterializeDeps are the injected typed ports. Default production provides
// none, so no Enrollment can materialize network access without an explicit
// trusted transport.
type MaterializeDeps struct {
	Search     toolbroker.SearchBackend
	MCPClients map[string]toolbroker.MCPToolClient
}

// Materialize validates the Enrollment state (valid, active, policy-current,
// adapter in the trusted catalog, kind/port agreement) and returns a typed
// executor carrying the exact allowlist and bounded limits. Every failure is
// closed and returns a sentinel error.
func Materialize(
	enrollment work.RemoteToolBackendEnrollment,
	policyCurrent bool,
	deps MaterializeDeps,
) (execution.RemoteToolExecutor, error) {
	if !enrollment.Valid() {
		return nil, ErrToolEnrollmentInvalid
	}
	if enrollment.Status() != work.RemoteToolBackendEnrollmentActive {
		return nil, ErrToolEnrollmentRevoked
	}
	if !policyCurrent {
		return nil, ErrToolEnrollmentPolicyDrift
	}
	catalog := work.BuiltInRemoteToolBackendCatalog()
	adapterSupported := false
	for _, descriptor := range catalog.RemoteToolBackendDescriptors() {
		if descriptor.AdapterID == enrollment.AdapterID() &&
			descriptor.BackendKind == enrollment.BackendKind() {
			adapterSupported = true
			break
		}
	}
	if !adapterSupported {
		return nil, ErrToolEnrollmentAdapterUnsupported
	}

	config := toolbroker.Config{
		Timeout:        enrollment.Timeout(),
		MaxResultBytes: enrollment.MaximumResultBytes(),
	}
	switch enrollment.BackendKind() {
	case work.RemoteToolBackendWebSearch:
		if deps.Search == nil {
			return nil, ErrToolEnrollmentPortUnavailable
		}
		config.Search = deps.Search
	case work.RemoteToolBackendMCPServer:
		serverID := enrollment.MCPServerID()
		client, found := deps.MCPClients[serverID]
		if !found || client == nil {
			return nil, ErrToolEnrollmentPortUnavailable
		}
		registry, err := toolbroker.NewMCPRegistry(
			map[string]toolbroker.MCPToolClient{serverID: client},
		)
		if err != nil {
			return nil, ErrToolEnrollmentInvalid
		}
		config.MCP = registry
		config.MCPAllowlist = map[string][]string{
			serverID: append([]string(nil), enrollment.AllowedTools()...),
		}
	default:
		return nil, ErrToolEnrollmentInvalid
	}
	broker, err := toolbroker.New(config)
	if err != nil {
		return nil, ErrToolEnrollmentInvalid
	}
	return broker, nil
}
