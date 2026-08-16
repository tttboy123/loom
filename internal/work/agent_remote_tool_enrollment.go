package work

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidAgentRemoteToolEnrollment            = errors.New("invalid agent remote tool enrollment")
	ErrAgentRemoteToolEnrollmentNotFound           = errors.New("agent remote tool enrollment not found")
	ErrAgentRemoteToolEnrollmentRevoked            = errors.New("agent remote tool enrollment revoked")
	ErrAgentRemoteToolBackendPolicyDrift           = errors.New("agent remote tool enrollment policy drift")
	ErrAgentRemoteToolEnrollmentAccountMismatch    = errors.New("agent remote tool enrollment account mismatch")
	ErrAgentRemoteToolEnrollmentDigestConflict     = errors.New("agent remote tool enrollment digest conflict")
	ErrAgentRemoteToolEnrollmentAdapterUnsupported = errors.New("agent remote tool enrollment adapter unsupported")
)

const (
	BuiltInSearchAdapterID = "builtin.search.deepseek.v1"
	BuiltInMCPAdapterID    = "builtin.mcp.stdio.v1"
)

// RemoteToolBackendDescriptor is a content-free trusted candidate description.
// It declares which Adapter IDs are acceptable and which backend kind each one
// materializes; it never carries endpoint, credential or tool data.
type RemoteToolBackendDescriptor struct {
	AdapterID   string
	BackendKind string
}

// RemoteToolBackendCatalog is the trusted built-in backend candidate registry.
// It is the only source of acceptable Adapter IDs for Enrollment binding and
// Work Bundle materialization.
type RemoteToolBackendCatalog interface {
	RemoteToolBackendDescriptors() []RemoteToolBackendDescriptor
	RemoteToolBackendAdapterSupported(adapterID string) bool
}

type staticRemoteToolBackendCatalog struct {
	descriptors []RemoteToolBackendDescriptor
}

func (catalog staticRemoteToolBackendCatalog) RemoteToolBackendDescriptors() []RemoteToolBackendDescriptor {
	result := make([]RemoteToolBackendDescriptor, 0, len(catalog.descriptors))
	for _, descriptor := range catalog.descriptors {
		result = append(result, descriptor)
	}
	return result
}

func (catalog staticRemoteToolBackendCatalog) RemoteToolBackendAdapterSupported(adapterID string) bool {
	for _, descriptor := range catalog.descriptors {
		if descriptor.AdapterID == adapterID {
			return true
		}
	}
	return false
}

// BuiltInRemoteToolBackendCatalog returns the trusted, versioned built-in
// backend candidate set. Anything not listed here is not materializable and
// must fail closed at preflight and materialization.
func BuiltInRemoteToolBackendCatalog() RemoteToolBackendCatalog {
	return staticRemoteToolBackendCatalog{
		descriptors: []RemoteToolBackendDescriptor{
			{AdapterID: BuiltInSearchAdapterID, BackendKind: RemoteToolBackendWebSearch},
			{AdapterID: BuiltInMCPAdapterID, BackendKind: RemoteToolBackendMCPServer},
		},
	}
}

// AgentRemoteToolEnrollmentSelection is the per-Agent frozen selection: the
// exact Enrollment identity plus the digest observed at selection time. The
// Provider Account must match the Agent's ExecutionProfile account.
type AgentRemoteToolEnrollmentSelection struct {
	EnrollmentID      string
	ProviderID        string
	ProviderAccountID string
	ExpectedDigest    string
}

// FrozenAgentRemoteToolEnrollment is the content-free per-Agent Enrollment
// binding frozen into an Attempt. It carries no tool names and no enrollment
// input; the allowlist is represented only by its digest.
type FrozenAgentRemoteToolEnrollment struct {
	EnrollmentID           string
	EnrollmentDigest       string
	BackendKind            string
	AdapterID              string
	ProviderID             string
	ProviderAccountID      string
	PolicyVersion          int
	PolicyRevision         int64
	PolicyDigest           string
	AllowedToolsDigest     string
	MaximumCallsPerAttempt int
	Timeout                time.Duration
	MaximumResultBytes     int
	BindingDigest          string
}

func validAgentRemoteToolEnrollmentSelection(
	selection AgentRemoteToolEnrollmentSelection,
	enrollment RemoteToolBackendEnrollment,
) bool {
	return validRemoteToolBackendIdentifier(selection.EnrollmentID) &&
		validRemoteToolBackendIdentifier(selection.ProviderID) &&
		validRemoteToolBackendIdentifier(selection.ProviderAccountID) &&
		validRemoteToolBackendDigest(selection.ExpectedDigest) &&
		validRemoteToolBackendIdentifier(enrollment.enrollmentID) &&
		validRemoteToolBackendIdentifier(enrollment.adapterID) &&
		enrollment.Valid()
}

func validRemoteToolBackendDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size &&
		value == strings.ToLower(value)
}

func remoteToolBackendAllowlistDigest(allowedTools []string) string {
	tools := append([]string(nil), allowedTools...)
	sort.Strings(tools)
	encoded, _ := json.Marshal(tools)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// FreezeAgentRemoteToolEnrollment validates the per-Agent selection against
// the authoritative Enrollment and policy currency and returns the content-
// free frozen binding. Every failure is a closed sentinel so the caller can
// map it to a safe stage and retryability without exposing any input.
func FreezeAgentRemoteToolEnrollment(
	selection AgentRemoteToolEnrollmentSelection,
	enrollment RemoteToolBackendEnrollment,
	policyCurrent bool,
	catalog RemoteToolBackendCatalog,
) (FrozenAgentRemoteToolEnrollment, error) {
	if catalog == nil {
		return FrozenAgentRemoteToolEnrollment{}, ErrInvalidAgentRemoteToolEnrollment
	}
	if !validAgentRemoteToolEnrollmentSelection(selection, enrollment) {
		return FrozenAgentRemoteToolEnrollment{}, ErrInvalidAgentRemoteToolEnrollment
	}
	if selection.EnrollmentID != enrollment.enrollmentID {
		return FrozenAgentRemoteToolEnrollment{}, ErrAgentRemoteToolEnrollmentNotFound
	}
	if enrollment.status != RemoteToolBackendEnrollmentActive {
		return FrozenAgentRemoteToolEnrollment{}, ErrAgentRemoteToolEnrollmentRevoked
	}
	if !policyCurrent {
		return FrozenAgentRemoteToolEnrollment{}, ErrAgentRemoteToolBackendPolicyDrift
	}
	if selection.ProviderID != enrollment.providerID ||
		selection.ProviderAccountID != enrollment.providerAccountID {
		return FrozenAgentRemoteToolEnrollment{}, ErrAgentRemoteToolEnrollmentAccountMismatch
	}
	if selection.ExpectedDigest != enrollment.digest {
		return FrozenAgentRemoteToolEnrollment{}, ErrAgentRemoteToolEnrollmentDigestConflict
	}
	if !catalog.RemoteToolBackendAdapterSupported(enrollment.adapterID) {
		return FrozenAgentRemoteToolEnrollment{}, ErrAgentRemoteToolEnrollmentAdapterUnsupported
	}
	frozen := FrozenAgentRemoteToolEnrollment{
		EnrollmentID:           enrollment.enrollmentID,
		EnrollmentDigest:       enrollment.digest,
		BackendKind:            enrollment.backendKind,
		AdapterID:              enrollment.adapterID,
		ProviderID:             enrollment.providerID,
		ProviderAccountID:      enrollment.providerAccountID,
		PolicyVersion:          enrollment.providerAccountPolicyVersion,
		PolicyRevision:         enrollment.providerAccountPolicyRevision,
		PolicyDigest:           enrollment.providerAccountPolicyDigest,
		AllowedToolsDigest:     remoteToolBackendAllowlistDigest(enrollment.allowedTools),
		MaximumCallsPerAttempt: enrollment.maximumCallsPerAttempt,
		Timeout:                enrollment.timeout,
		MaximumResultBytes:     enrollment.maximumResultBytes,
	}
	return freezeAgentRemoteToolEnrollmentBinding(frozen)
}

func freezeAgentRemoteToolEnrollmentBinding(
	frozen FrozenAgentRemoteToolEnrollment,
) (FrozenAgentRemoteToolEnrollment, error) {
	frozen.BindingDigest = ""
	value := struct {
		DigestDomain           string
		EnrollmentID           string
		EnrollmentDigest       string
		BackendKind            string
		AdapterID              string
		ProviderID             string
		ProviderAccountID      string
		PolicyVersion          int
		PolicyRevision         int64
		PolicyDigest           string
		AllowedToolsDigest     string
		MaximumCallsPerAttempt int
		TimeoutNanoseconds     int64
		MaximumResultBytes     int
	}{
		DigestDomain:           "loom.agent-remote-tool-enrollment.v1",
		EnrollmentID:           frozen.EnrollmentID,
		EnrollmentDigest:       frozen.EnrollmentDigest,
		BackendKind:            frozen.BackendKind,
		AdapterID:              frozen.AdapterID,
		ProviderID:             frozen.ProviderID,
		ProviderAccountID:      frozen.ProviderAccountID,
		PolicyVersion:          frozen.PolicyVersion,
		PolicyRevision:         frozen.PolicyRevision,
		PolicyDigest:           frozen.PolicyDigest,
		AllowedToolsDigest:     frozen.AllowedToolsDigest,
		MaximumCallsPerAttempt: frozen.MaximumCallsPerAttempt,
		TimeoutNanoseconds:     int64(frozen.Timeout),
		MaximumResultBytes:     frozen.MaximumResultBytes,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return FrozenAgentRemoteToolEnrollment{}, ErrInvalidAgentRemoteToolEnrollment
	}
	sum := sha256.Sum256(encoded)
	frozen.BindingDigest = hex.EncodeToString(sum[:])
	return frozen, nil
}
