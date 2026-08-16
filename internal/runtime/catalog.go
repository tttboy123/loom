package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidRuntimeProfile   = errors.New("invalid runtime profile")
	ErrInvalidRuntimeInstance  = errors.New("invalid runtime instance")
	ErrRuntimeOffline          = errors.New("runtime instance offline")
	ErrRuntimeIncompatible     = errors.New("runtime instance incompatible")
	ErrRuntimeDisabled         = errors.New("runtime instance disabled")
	ErrAdapterMismatch         = errors.New("runtime adapter mismatch")
	ErrMissingCapability       = errors.New("runtime capability missing")
	ErrInvalidCapacity         = errors.New("runtime capacity invalid")
	ErrInvalidExecutionProfile = errors.New("invalid execution profile")
)

type AuthMode string

const (
	AuthBrokered          AuthMode = "brokered"
	AuthProviderEphemeral AuthMode = "provider_ephemeral"
	AuthNative            AuthMode = "native_auth"
)

const (
	CapabilityContextRetrieval = "context_retrieval"
	CapabilityGovernedToolLoop = "governed_tool_loop"
	CapabilityReasoningEffort  = "reasoning_effort"
)

type RuntimeStatus string

const (
	RuntimeOnline       RuntimeStatus = "online"
	RuntimeOffline      RuntimeStatus = "offline"
	RuntimeIncompatible RuntimeStatus = "incompatible"
	RuntimeDisabled     RuntimeStatus = "disabled"
)

type RuntimeProfile struct {
	ID                   string
	AdapterType          string
	ProviderID           string
	ProviderAccountID    string
	ModelID              string
	AuthMode             AuthMode
	EndpointFingerprint  string
	CredentialReference  string
	CredentialRevision   int64
	ReasoningEffort      string
	RequiredCapabilities []string
	Timeout              time.Duration
	Budget               *int64
	// RemoteToolEnrollmentID and RemoteToolEnrollmentDigest are the optional
	// both-or-neither per-Agent remote tool Enrollment selection. They bind
	// the exact authoritative Enrollment digest into the frozen binding.
	RemoteToolEnrollmentID     string
	RemoteToolEnrollmentDigest string
}

// ExecutionProfile is the product name for the existing versioned
// RuntimeProfile contract during the v0.5.x schema migration.
type ExecutionProfile = RuntimeProfile

type RuntimeInstance struct {
	ID                   string
	DeviceID             string
	AdapterType          string
	DisplayName          string
	ExecutableVersion    string
	Status               RuntimeStatus
	ObservedCapabilities []string
	Capacity             int
}

type BindingCandidate struct {
	Accepted   bool
	ProfileID  string
	InstanceID string
}

type FrozenExecutionBinding struct {
	ProfileID                  string
	HarnessAdapter             string
	RuntimeInstanceID          string
	ProviderID                 string
	ProviderAccountID          string
	ModelID                    string
	AuthMode                   AuthMode
	EndpointFingerprint        string
	CredentialReference        string
	CredentialRevision         int64
	ReasoningEffort            string
	Timeout                    time.Duration
	Budget                     *int64
	Capabilities               []string
	RemoteToolEnrollmentID     string
	RemoteToolEnrollmentDigest string
	BindingDigest              string
}

func NewRuntimeProfile(input RuntimeProfile) (RuntimeProfile, error) {
	if err := validateRuntimeProfile(input); err != nil {
		return RuntimeProfile{}, err
	}
	return copyRuntimeProfile(input), nil
}

func ValidateExecutionProfile(input RuntimeProfile) (RuntimeProfile, error) {
	profile, err := NewRuntimeProfile(input)
	if err != nil {
		return RuntimeProfile{}, err
	}
	validationInstance := RuntimeInstance{
		ID:                   "execution-profile-validation",
		DeviceID:             "loom-authority",
		AdapterType:          profile.AdapterType,
		DisplayName:          "Execution Profile Validation",
		Status:               RuntimeOnline,
		ObservedCapabilities: append([]string(nil), profile.RequiredCapabilities...),
		Capacity:             1,
	}
	if _, err := FreezeExecutionBinding(profile, validationInstance); err != nil {
		return RuntimeProfile{}, err
	}
	return profile, nil
}

func NewRuntimeInstance(input RuntimeInstance) (RuntimeInstance, error) {
	if err := validateRuntimeInstance(input); err != nil {
		return RuntimeInstance{}, err
	}
	return copyRuntimeInstance(input), nil
}

func ValidateBinding(profile RuntimeProfile, instance RuntimeInstance) (BindingCandidate, error) {
	if err := validateRuntimeProfile(profile); err != nil {
		return BindingCandidate{}, err
	}
	if err := validateRuntimeInstanceShape(instance); err != nil {
		return BindingCandidate{}, err
	}
	if instance.Capacity <= 0 {
		return BindingCandidate{}, fmt.Errorf("%w: %w", ErrInvalidRuntimeInstance, ErrInvalidCapacity)
	}

	switch instance.Status {
	case RuntimeOnline:
	case RuntimeOffline:
		return BindingCandidate{}, ErrRuntimeOffline
	case RuntimeIncompatible:
		return BindingCandidate{}, ErrRuntimeIncompatible
	case RuntimeDisabled:
		return BindingCandidate{}, ErrRuntimeDisabled
	default:
		return BindingCandidate{}, fmt.Errorf("%w: invalid status %q", ErrInvalidRuntimeInstance, instance.Status)
	}

	if profile.AdapterType != instance.AdapterType {
		return BindingCandidate{}, ErrAdapterMismatch
	}

	observed := make(map[string]struct{}, len(instance.ObservedCapabilities))
	for _, capability := range instance.ObservedCapabilities {
		observed[capability] = struct{}{}
	}
	for _, capability := range profile.RequiredCapabilities {
		if _, ok := observed[capability]; !ok {
			return BindingCandidate{}, fmt.Errorf("%w: %s", ErrMissingCapability, capability)
		}
	}

	return BindingCandidate{
		Accepted:   true,
		ProfileID:  profile.ID,
		InstanceID: instance.ID,
	}, nil
}

func FreezeExecutionBinding(
	profile RuntimeProfile,
	instance RuntimeInstance,
) (FrozenExecutionBinding, error) {
	if _, err := ValidateBinding(profile, instance); err != nil {
		return FrozenExecutionBinding{}, err
	}
	if !validExecutionProfileText(profile.ProviderID, 128) ||
		!validExecutionProfileText(profile.ModelID, 256) {
		return FrozenExecutionBinding{}, ErrInvalidExecutionProfile
	}
	if profile.AuthMode == AuthNative {
		if profile.ProviderAccountID != "" || profile.EndpointFingerprint != "" ||
			profile.CredentialReference != "" || profile.CredentialRevision != 0 {
			return FrozenExecutionBinding{}, ErrInvalidExecutionProfile
		}
	} else if !validExecutionProfileText(profile.ProviderAccountID, 128) ||
		!validSHA256Fingerprint(profile.EndpointFingerprint) ||
		!validExecutionCredentialReference(profile.CredentialReference) ||
		profile.CredentialRevision <= 0 {
		return FrozenExecutionBinding{}, ErrInvalidExecutionProfile
	}
	if !validExecutionProfileEnrollment(profile) {
		return FrozenExecutionBinding{}, ErrInvalidExecutionProfile
	}

	binding := FrozenExecutionBinding{
		ProfileID:                  profile.ID,
		HarnessAdapter:             profile.AdapterType,
		RuntimeInstanceID:          instance.ID,
		ProviderID:                 profile.ProviderID,
		ProviderAccountID:          profile.ProviderAccountID,
		ModelID:                    profile.ModelID,
		AuthMode:                   profile.AuthMode,
		EndpointFingerprint:        profile.EndpointFingerprint,
		CredentialReference:        profile.CredentialReference,
		CredentialRevision:         profile.CredentialRevision,
		ReasoningEffort:            profile.ReasoningEffort,
		Timeout:                    profile.Timeout,
		Capabilities:               normalizeCapabilities(profile.RequiredCapabilities),
		RemoteToolEnrollmentID:     profile.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: profile.RemoteToolEnrollmentDigest,
	}
	if profile.Budget != nil {
		budget := *profile.Budget
		binding.Budget = &budget
	}
	digest, err := digestFrozenExecutionBinding(binding)
	if err != nil {
		return FrozenExecutionBinding{}, ErrInvalidExecutionProfile
	}
	binding.BindingDigest = digest
	return binding, nil
}

func ValidateFrozenExecutionBinding(
	input FrozenExecutionBinding,
) (FrozenExecutionBinding, error) {
	profile := RuntimeProfile{
		ID:                         input.ProfileID,
		AdapterType:                input.HarnessAdapter,
		ProviderID:                 input.ProviderID,
		ProviderAccountID:          input.ProviderAccountID,
		ModelID:                    input.ModelID,
		AuthMode:                   input.AuthMode,
		EndpointFingerprint:        input.EndpointFingerprint,
		CredentialReference:        input.CredentialReference,
		CredentialRevision:         input.CredentialRevision,
		ReasoningEffort:            input.ReasoningEffort,
		RequiredCapabilities:       append([]string(nil), input.Capabilities...),
		Timeout:                    input.Timeout,
		RemoteToolEnrollmentID:     input.RemoteToolEnrollmentID,
		RemoteToolEnrollmentDigest: input.RemoteToolEnrollmentDigest,
	}
	if input.Budget != nil {
		budget := *input.Budget
		profile.Budget = &budget
	}
	instance := RuntimeInstance{
		ID:                   input.RuntimeInstanceID,
		DeviceID:             "frozen-binding-validation",
		AdapterType:          input.HarnessAdapter,
		DisplayName:          "Frozen binding validation",
		Status:               RuntimeOnline,
		ObservedCapabilities: append([]string(nil), input.Capabilities...),
		Capacity:             1,
	}
	expected, err := FreezeExecutionBinding(profile, instance)
	if err != nil || input.BindingDigest == "" ||
		!reflect.DeepEqual(input, expected) {
		return FrozenExecutionBinding{}, ErrInvalidExecutionProfile
	}
	return expected, nil
}

func validateRuntimeProfile(input RuntimeProfile) error {
	if input.ID == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidRuntimeProfile)
	}
	if input.AdapterType == "" {
		return fmt.Errorf("%w: empty adapter type", ErrInvalidRuntimeProfile)
	}
	switch input.AuthMode {
	case AuthBrokered, AuthProviderEphemeral, AuthNative:
	default:
		return fmt.Errorf("%w: invalid auth mode %q", ErrInvalidRuntimeProfile, input.AuthMode)
	}
	if err := validateCapabilitySet(input.RequiredCapabilities, ErrInvalidRuntimeProfile); err != nil {
		return err
	}
	if input.ReasoningEffort != "" {
		if !validReasoningEffort(input.ReasoningEffort) ||
			!containsCapability(input.RequiredCapabilities, CapabilityReasoningEffort) {
			return fmt.Errorf("%w: invalid reasoning effort", ErrInvalidRuntimeProfile)
		}
	}
	if input.Timeout <= 0 {
		return fmt.Errorf("%w: nonpositive timeout", ErrInvalidRuntimeProfile)
	}
	if input.Budget != nil && *input.Budget < 0 {
		return fmt.Errorf("%w: negative budget", ErrInvalidRuntimeProfile)
	}
	return nil
}

func validateRuntimeInstance(input RuntimeInstance) error {
	if err := validateRuntimeInstanceShape(input); err != nil {
		return err
	}
	if input.Capacity <= 0 {
		return fmt.Errorf("%w: %w", ErrInvalidRuntimeInstance, ErrInvalidCapacity)
	}
	return nil
}

func validateRuntimeInstanceShape(input RuntimeInstance) error {
	if input.ID == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidRuntimeInstance)
	}
	if input.DeviceID == "" {
		return fmt.Errorf("%w: empty device id", ErrInvalidRuntimeInstance)
	}
	if input.AdapterType == "" {
		return fmt.Errorf("%w: empty adapter type", ErrInvalidRuntimeInstance)
	}
	if input.DisplayName == "" {
		return fmt.Errorf("%w: empty display name", ErrInvalidRuntimeInstance)
	}
	switch input.Status {
	case RuntimeOnline, RuntimeOffline, RuntimeIncompatible, RuntimeDisabled:
	default:
		return fmt.Errorf("%w: invalid status %q", ErrInvalidRuntimeInstance, input.Status)
	}
	if err := validateCapabilitySet(input.ObservedCapabilities, ErrInvalidRuntimeInstance); err != nil {
		return err
	}
	return nil
}

func validateCapabilitySet(capabilities []string, sentinel error) error {
	seen := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if capability == "" {
			return fmt.Errorf("%w: empty capability", sentinel)
		}
		if _, ok := seen[capability]; ok {
			return fmt.Errorf("%w: duplicate capability %q", sentinel, capability)
		}
		seen[capability] = struct{}{}
	}
	return nil
}

func copyRuntimeProfile(input RuntimeProfile) RuntimeProfile {
	clone := input
	clone.RequiredCapabilities = normalizeCapabilities(input.RequiredCapabilities)
	if input.Budget != nil {
		budget := *input.Budget
		clone.Budget = &budget
	}
	return clone
}

func copyRuntimeInstance(input RuntimeInstance) RuntimeInstance {
	clone := input
	clone.ObservedCapabilities = normalizeCapabilities(input.ObservedCapabilities)
	return clone
}

func normalizeCapabilities(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	clone := append([]string(nil), input...)
	sort.Strings(clone)
	return clone
}

func digestFrozenExecutionBinding(input FrozenExecutionBinding) (string, error) {
	input.BindingDigest = ""
	var value any = legacyFrozenExecutionBindingDigestValue(input)
	if input.ReasoningEffort != "" || input.RemoteToolEnrollmentID != "" {
		domain := "loom.frozen-execution-binding.v2"
		if input.RemoteToolEnrollmentID != "" {
			domain = "loom.frozen-execution-binding.v3"
		}
		value = struct {
			DigestDomain string
			Binding      FrozenExecutionBinding
		}{
			DigestDomain: domain,
			Binding:      input,
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func legacyFrozenExecutionBindingDigestValue(input FrozenExecutionBinding) any {
	return struct {
		ProfileID           string
		HarnessAdapter      string
		RuntimeInstanceID   string
		ProviderID          string
		ProviderAccountID   string
		ModelID             string
		AuthMode            AuthMode
		EndpointFingerprint string
		CredentialReference string
		CredentialRevision  int64
		Timeout             time.Duration
		Budget              *int64
		Capabilities        []string
		BindingDigest       string
	}{
		ProfileID:           input.ProfileID,
		HarnessAdapter:      input.HarnessAdapter,
		RuntimeInstanceID:   input.RuntimeInstanceID,
		ProviderID:          input.ProviderID,
		ProviderAccountID:   input.ProviderAccountID,
		ModelID:             input.ModelID,
		AuthMode:            input.AuthMode,
		EndpointFingerprint: input.EndpointFingerprint,
		CredentialReference: input.CredentialReference,
		CredentialRevision:  input.CredentialRevision,
		Timeout:             input.Timeout,
		Budget:              input.Budget,
		Capabilities:        input.Capabilities,
		BindingDigest:       input.BindingDigest,
	}
}

func containsCapability(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validReasoningEffort(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validExecutionProfileText(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e || strings.ContainsRune("\\\"'`", r) {
			return false
		}
	}
	return true
}

func validSHA256Fingerprint(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validExecutionProfileEnrollment(profile RuntimeProfile) bool {
	if profile.RemoteToolEnrollmentID == "" && profile.RemoteToolEnrollmentDigest == "" {
		return true
	}
	return validExecutionProfileText(profile.RemoteToolEnrollmentID, 128) &&
		validSHA256Fingerprint(profile.RemoteToolEnrollmentDigest)
}

func validExecutionCredentialReference(value string) bool {
	const prefix = "credential-ref-"
	if !strings.HasPrefix(value, prefix) ||
		len(value) <= len(prefix) || len(value) > 128 {
		return false
	}
	for _, r := range value[len(prefix):] {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || r == '-' {
			continue
		}
		return false
	}
	return true
}
