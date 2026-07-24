package runtime

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

var (
	ErrInvalidRuntimeProfile  = errors.New("invalid runtime profile")
	ErrInvalidRuntimeInstance = errors.New("invalid runtime instance")
	ErrRuntimeOffline         = errors.New("runtime instance offline")
	ErrRuntimeIncompatible    = errors.New("runtime instance incompatible")
	ErrRuntimeDisabled        = errors.New("runtime instance disabled")
	ErrAdapterMismatch        = errors.New("runtime adapter mismatch")
	ErrMissingCapability      = errors.New("runtime capability missing")
	ErrInvalidCapacity        = errors.New("runtime capacity invalid")
)

type AuthMode string

const (
	AuthBrokered          AuthMode = "brokered"
	AuthProviderEphemeral AuthMode = "provider_ephemeral"
	AuthNative            AuthMode = "native_auth"
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
	ModelID              string
	AuthMode             AuthMode
	RequiredCapabilities []string
	Timeout              time.Duration
	Budget               *int64
}

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

func NewRuntimeProfile(input RuntimeProfile) (RuntimeProfile, error) {
	if err := validateRuntimeProfile(input); err != nil {
		return RuntimeProfile{}, err
	}
	return copyRuntimeProfile(input), nil
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
