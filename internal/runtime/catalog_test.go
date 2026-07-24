package runtime

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRuntimeProfileAndRuntimeInstanceValidateFrozenContractsAndCopyMutableInputs(t *testing.T) {
	budget := int64(500)
	profileInput := RuntimeProfile{
		ID:                   "runtime.profile.local",
		AdapterType:          "local-shell",
		ProviderID:           "provider.local",
		ModelID:              "model.local",
		AuthMode:             AuthBrokered,
		RequiredCapabilities: []string{"apply_patch", "go_test"},
		Timeout:              30 * time.Second,
		Budget:               &budget,
	}
	profile, err := NewRuntimeProfile(profileInput)
	if err != nil {
		t.Fatalf("NewRuntimeProfile(valid) error = %v", err)
	}
	profileInput.RequiredCapabilities[0] = "mutated"
	budget = 900
	if got := profile.RequiredCapabilities; !reflect.DeepEqual(got, []string{"apply_patch", "go_test"}) {
		t.Fatalf("RuntimeProfile capabilities = %#v, want immutable copy", got)
	}
	if profile.Budget == nil || *profile.Budget != 500 {
		t.Fatalf("RuntimeProfile budget = %#v, want immutable copy of 500", profile.Budget)
	}

	instanceInput := RuntimeInstance{
		ID:                   "runtime.instance.mac",
		DeviceID:             "device.mac",
		AdapterType:          "local-shell",
		DisplayName:          "Local Mac",
		ExecutableVersion:    "1.0.0",
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{"go_test", "apply_patch", "extra"},
		Capacity:             1,
	}
	instance, err := NewRuntimeInstance(instanceInput)
	if err != nil {
		t.Fatalf("NewRuntimeInstance(valid) error = %v", err)
	}
	instanceInput.ObservedCapabilities[0] = "mutated"
	if got := instance.ObservedCapabilities; !reflect.DeepEqual(got, []string{"apply_patch", "extra", "go_test"}) {
		t.Fatalf("RuntimeInstance observed capabilities = %#v, want deterministic immutable copy", got)
	}

	profileInvalids := []struct {
		name  string
		input RuntimeProfile
	}{
		{name: "empty stable id", input: withRuntimeProfile(profile, func(p *RuntimeProfile) { p.ID = "" })},
		{name: "empty adapter type", input: withRuntimeProfile(profile, func(p *RuntimeProfile) { p.AdapterType = "" })},
		{name: "invalid auth mode", input: withRuntimeProfile(profile, func(p *RuntimeProfile) { p.AuthMode = "raw_key" })},
		{name: "nonpositive timeout", input: withRuntimeProfile(profile, func(p *RuntimeProfile) { p.Timeout = 0 })},
		{name: "negative budget", input: withRuntimeProfile(profile, func(p *RuntimeProfile) {
			negative := int64(-1)
			p.Budget = &negative
		})},
	}
	for _, tt := range profileInvalids {
		t.Run("profile "+tt.name, func(t *testing.T) {
			_, err := NewRuntimeProfile(tt.input)
			if !errors.Is(err, ErrInvalidRuntimeProfile) {
				t.Fatalf("NewRuntimeProfile() error = %v, want ErrInvalidRuntimeProfile", err)
			}
		})
	}

	instanceInvalids := []struct {
		name  string
		input RuntimeInstance
	}{
		{name: "empty stable id", input: withRuntimeInstance(instance, func(i *RuntimeInstance) { i.ID = "" })},
		{name: "empty device id", input: withRuntimeInstance(instance, func(i *RuntimeInstance) { i.DeviceID = "" })},
		{name: "empty adapter type", input: withRuntimeInstance(instance, func(i *RuntimeInstance) { i.AdapterType = "" })},
		{name: "empty display name", input: withRuntimeInstance(instance, func(i *RuntimeInstance) { i.DisplayName = "" })},
		{name: "invalid status", input: withRuntimeInstance(instance, func(i *RuntimeInstance) { i.Status = "busy" })},
		{name: "zero capacity", input: withRuntimeInstance(instance, func(i *RuntimeInstance) { i.Capacity = 0 })},
	}
	for _, tt := range instanceInvalids {
		t.Run("instance "+tt.name, func(t *testing.T) {
			_, err := NewRuntimeInstance(tt.input)
			if !errors.Is(err, ErrInvalidRuntimeInstance) {
				t.Fatalf("NewRuntimeInstance() error = %v, want ErrInvalidRuntimeInstance", err)
			}
		})
	}

	forbiddenProfileFields := []string{"instance", "device", "executable", "online", "capacity", "raw", "refresh", "cli", "credential", "grant", "token", "run"}
	assertNoFieldsContaining(t, profile, forbiddenProfileFields)

	forbiddenInstanceFields := []string{"role", "modelpolicy", "budgetpolicy", "timeoutpolicy", "credential", "grant", "run"}
	assertNoFieldsContaining(t, instance, forbiddenInstanceFields)
}

func TestValidateBindingAcceptsCompatibleOnlineCandidateWithoutSideEffects(t *testing.T) {
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID:                   "runtime.profile.local",
		AdapterType:          "local-shell",
		AuthMode:             AuthBrokered,
		RequiredCapabilities: []string{"go_test", "apply_patch"},
		Timeout:              30 * time.Second,
	})
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID:                   "runtime.instance.mac",
		DeviceID:             "device.mac",
		AdapterType:          "local-shell",
		DisplayName:          "Local Mac",
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{"apply_patch", "go_test", "extra"},
		Capacity:             2,
	})
	beforeProfile := cloneRuntimeProfile(profile)
	beforeInstance := cloneRuntimeInstance(instance)

	candidate, err := ValidateBinding(profile, instance)
	if err != nil {
		t.Fatalf("ValidateBinding(compatible) error = %v", err)
	}
	if !candidate.Accepted {
		t.Fatalf("ValidateBinding(compatible) candidate.Accepted = false, want true")
	}
	if candidate.ProfileID != profile.ID || candidate.InstanceID != instance.ID {
		t.Fatalf("ValidateBinding candidate = %#v, want profile %q and instance %q", candidate, profile.ID, instance.ID)
	}
	if !reflect.DeepEqual(profile, beforeProfile) {
		t.Fatalf("ValidateBinding mutated profile: got %#v want %#v", profile, beforeProfile)
	}
	if !reflect.DeepEqual(instance, beforeInstance) {
		t.Fatalf("ValidateBinding mutated instance: got %#v want %#v", instance, beforeInstance)
	}
}

func TestValidateBindingRejectsFrozenFailureClassesWithTypedErrors(t *testing.T) {
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID:                   "runtime.profile.local",
		AdapterType:          "local-shell",
		AuthMode:             AuthBrokered,
		RequiredCapabilities: []string{"go_test", "apply_patch"},
		Timeout:              30 * time.Second,
	})
	online := mustRuntimeInstance(t, RuntimeInstance{
		ID:                   "runtime.instance.mac",
		DeviceID:             "device.mac",
		AdapterType:          "local-shell",
		DisplayName:          "Local Mac",
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{"apply_patch", "go_test"},
		Capacity:             1,
	})

	tests := []struct {
		name     string
		profile  RuntimeProfile
		instance RuntimeInstance
		wantIs   error
	}{
		{name: "offline", profile: profile, instance: withRuntimeInstance(online, func(i *RuntimeInstance) { i.Status = RuntimeOffline }), wantIs: ErrRuntimeOffline},
		{name: "incompatible", profile: profile, instance: withRuntimeInstance(online, func(i *RuntimeInstance) { i.Status = RuntimeIncompatible }), wantIs: ErrRuntimeIncompatible},
		{name: "disabled", profile: profile, instance: withRuntimeInstance(online, func(i *RuntimeInstance) { i.Status = RuntimeDisabled }), wantIs: ErrRuntimeDisabled},
		{name: "adapter mismatch", profile: profile, instance: withRuntimeInstance(online, func(i *RuntimeInstance) { i.AdapterType = "remote-http" }), wantIs: ErrAdapterMismatch},
		{name: "missing capability", profile: profile, instance: withRuntimeInstance(online, func(i *RuntimeInstance) { i.ObservedCapabilities = []string{"go_test"} }), wantIs: ErrMissingCapability},
		{name: "invalid capacity", profile: profile, instance: withRuntimeInstance(online, func(i *RuntimeInstance) { i.Capacity = 0 }), wantIs: ErrInvalidCapacity},
		{name: "invalid profile", profile: withRuntimeProfile(profile, func(p *RuntimeProfile) { p.Timeout = 0 }), instance: online, wantIs: ErrInvalidRuntimeProfile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, err := ValidateBinding(tt.profile, tt.instance)
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("ValidateBinding() error = %v, want errors.Is(%v)", err, tt.wantIs)
			}
			if candidate.Accepted {
				t.Fatalf("ValidateBinding() accepted candidate for %s failure", tt.name)
			}
		})
	}
}

func TestRuntimeImportBoundaryStaysPureDomain(t *testing.T) {
	forbidden := map[string]bool{
		"database/sql": true,
		"net":          true,
		"net/http":     true,
		"os/exec":      true,

		"loom-pi-rebuild/internal/agents":     true,
		"loom-pi-rebuild/internal/evidence":   true,
		"loom-pi-rebuild/internal/journal":    true,
		"loom-pi-rebuild/internal/mode":       true,
		"loom-pi-rebuild/internal/projection": true,
	}

	assertNoForbiddenProductionImports(t, ".", forbidden)
}

func mustRuntimeProfile(t *testing.T, input RuntimeProfile) RuntimeProfile {
	t.Helper()

	profile, err := NewRuntimeProfile(input)
	if err != nil {
		t.Fatalf("NewRuntimeProfile(%#v) error = %v", input, err)
	}
	return profile
}

func mustRuntimeInstance(t *testing.T, input RuntimeInstance) RuntimeInstance {
	t.Helper()

	instance, err := NewRuntimeInstance(input)
	if err != nil {
		t.Fatalf("NewRuntimeInstance(%#v) error = %v", input, err)
	}
	return instance
}

func withRuntimeProfile(input RuntimeProfile, change func(*RuntimeProfile)) RuntimeProfile {
	changed := cloneRuntimeProfile(input)
	change(&changed)
	return changed
}

func withRuntimeInstance(input RuntimeInstance, change func(*RuntimeInstance)) RuntimeInstance {
	changed := cloneRuntimeInstance(input)
	change(&changed)
	return changed
}

func cloneRuntimeProfile(input RuntimeProfile) RuntimeProfile {
	clone := input
	clone.RequiredCapabilities = slices.Clone(input.RequiredCapabilities)
	if input.Budget != nil {
		budget := *input.Budget
		clone.Budget = &budget
	}
	return clone
}

func cloneRuntimeInstance(input RuntimeInstance) RuntimeInstance {
	clone := input
	clone.ObservedCapabilities = slices.Clone(input.ObservedCapabilities)
	return clone
}

func assertNoFieldsContaining(t *testing.T, value any, forbidden []string) {
	t.Helper()

	valueType := reflect.TypeOf(value)
	for i := 0; i < valueType.NumField(); i++ {
		fieldName := strings.ToLower(valueType.Field(i).Name)
		for _, token := range forbidden {
			if strings.Contains(fieldName, token) {
				t.Fatalf("%s field %q contains forbidden authority token %q", valueType.Name(), valueType.Field(i).Name, token)
			}
		}
	}
}

func assertNoForbiddenProductionImports(t *testing.T, dir string, forbidden map[string]bool) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v", path, err)
		}
		for _, imported := range file.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			if forbidden[importPath] {
				t.Fatalf("%s imports forbidden boundary package %q", path, importPath)
			}
		}
	}
}
