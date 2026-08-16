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
		ProviderAccountID:    "provider-account.local.primary",
		ModelID:              "model.local",
		AuthMode:             AuthBrokered,
		EndpointFingerprint:  strings.Repeat("a", 64),
		CredentialReference:  "credential-ref-local-primary",
		CredentialRevision:   7,
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

	forbiddenProfileFields := []string{"instance", "device", "executable", "online", "capacity", "secret", "rawkey", "apikey", "grant", "token", "run"}
	assertNoFieldsContaining(t, profile, forbiddenProfileFields)

	forbiddenInstanceFields := []string{"role", "modelpolicy", "budgetpolicy", "timeoutpolicy", "credential", "grant", "run"}
	assertNoFieldsContaining(t, instance, forbiddenInstanceFields)
}

func TestFreezeExecutionBindingCapturesProviderAccountWithoutSecretBody(t *testing.T) {
	budget := int64(250)
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID: "profile.claude.primary", AdapterType: "claude-code",
		ProviderID: "anthropic", ProviderAccountID: "anthropic.work",
		ModelID: "claude-sonnet-4-5", AuthMode: AuthBrokered,
		EndpointFingerprint:  strings.Repeat("b", 64),
		CredentialReference:  "credential-ref-anthropic-work",
		CredentialRevision:   11,
		RequiredCapabilities: []string{"edit", "terminal"},
		Timeout:              2 * time.Minute, Budget: &budget,
	})
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID: "runtime.claude-code.local", DeviceID: "device.mac",
		AdapterType: "claude-code", DisplayName: "Claude Code",
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{"terminal", "edit", "review"},
		Capacity:             2,
	})

	binding, err := FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatalf("FreezeExecutionBinding() error = %v", err)
	}
	if binding.ProfileID != profile.ID ||
		binding.HarnessAdapter != "claude-code" ||
		binding.RuntimeInstanceID != instance.ID ||
		binding.ProviderID != "anthropic" ||
		binding.ProviderAccountID != "anthropic.work" ||
		binding.ModelID != "claude-sonnet-4-5" ||
		binding.EndpointFingerprint != strings.Repeat("b", 64) ||
		binding.CredentialReference != "credential-ref-anthropic-work" ||
		binding.CredentialRevision != 11 ||
		binding.Timeout != 2*time.Minute || binding.Budget == nil ||
		*binding.Budget != 250 ||
		!reflect.DeepEqual(binding.Capabilities, []string{"edit", "terminal"}) ||
		len(binding.BindingDigest) != 64 {
		t.Fatalf("frozen binding = %#v", binding)
	}
	encoded := strings.ToLower(binding.BindingDigest + binding.CredentialReference)
	if strings.Contains(encoded, "sk-ant-secret-body") {
		t.Fatal("frozen binding contains secret body")
	}

	second := profile
	second.ProviderAccountID = "anthropic.personal"
	second.CredentialReference = "credential-ref-anthropic-personal"
	second.CredentialRevision = 3
	secondBinding, err := FreezeExecutionBinding(second, instance)
	if err != nil {
		t.Fatalf("FreezeExecutionBinding(second account) error = %v", err)
	}
	if secondBinding.BindingDigest == binding.BindingDigest {
		t.Fatal("different Provider Accounts produced the same binding digest")
	}
}

func TestFreezeExecutionBindingPreservesLegacyDigest(t *testing.T) {
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID: "profile.legacy.v1", AdapterType: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", AuthMode: AuthBrokered,
		EndpointFingerprint:  strings.Repeat("a", 64),
		CredentialReference:  "credential-ref-deepseek-primary",
		CredentialRevision:   2,
		RequiredCapabilities: []string{"chat"},
		Timeout:              45 * time.Second,
	})
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID: "runtime.loom-native", DeviceID: "device.mac",
		AdapterType: "loom-native", DisplayName: "Loom Native",
		Status: RuntimeOnline, ObservedCapabilities: []string{"chat"},
		Capacity: 1,
	})

	binding, err := FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	const publishedDigest = "88e49a903c70d6dce01d466d00699eb5a33b4c5251b63c6cecc8d4fda8e68ffe"
	if binding.BindingDigest != publishedDigest {
		t.Fatalf("legacy binding digest = %q, want %q", binding.BindingDigest, publishedDigest)
	}
}

func TestFreezeExecutionBindingCapturesReasoningEffortAndRejectsMutation(t *testing.T) {
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID: "profile.codex.reasoning", AdapterType: "codex",
		ProviderID: "openai", ProviderAccountID: "openai.primary",
		ModelID: "gpt-5.5-codex", AuthMode: AuthBrokered,
		EndpointFingerprint:  strings.Repeat("b", 64),
		CredentialReference:  "credential-ref-openai-primary",
		CredentialRevision:   4,
		ReasoningEffort:      "high",
		RequiredCapabilities: []string{CapabilityReasoningEffort, "tools"},
		Timeout:              2 * time.Minute,
	})
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID: "runtime.codex", DeviceID: "device.mac",
		AdapterType: "codex", DisplayName: "Codex",
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{CapabilityReasoningEffort, "tools"},
		Capacity:             1,
	})

	binding, err := FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ReasoningEffort != "high" || binding.BindingDigest == "" {
		t.Fatalf("frozen binding = %#v", binding)
	}
	if _, err := ValidateFrozenExecutionBinding(binding); err != nil {
		t.Fatalf("ValidateFrozenExecutionBinding() error = %v", err)
	}

	mutated := binding
	mutated.ReasoningEffort = "low"
	if _, err := ValidateFrozenExecutionBinding(mutated); !errors.Is(err, ErrInvalidExecutionProfile) {
		t.Fatalf("reasoning mutation error = %v, want ErrInvalidExecutionProfile", err)
	}
}

func TestRuntimeProfileRejectsReasoningEffortWithoutRuntimeCapability(t *testing.T) {
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID: "profile.codex.reasoning", AdapterType: "codex",
		ProviderID: "openai", ProviderAccountID: "openai.primary",
		ModelID: "gpt-5.5-codex", AuthMode: AuthBrokered,
		EndpointFingerprint:  strings.Repeat("c", 64),
		CredentialReference:  "credential-ref-openai-primary",
		CredentialRevision:   4,
		ReasoningEffort:      "high",
		RequiredCapabilities: []string{CapabilityReasoningEffort},
		Timeout:              time.Minute,
	})
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID: "runtime.codex", DeviceID: "device.mac",
		AdapterType: "codex", DisplayName: "Codex",
		Status: RuntimeOnline, Capacity: 1,
	})

	if _, err := FreezeExecutionBinding(profile, instance); !errors.Is(err, ErrMissingCapability) {
		t.Fatalf("FreezeExecutionBinding() error = %v, want ErrMissingCapability", err)
	}
}

func TestFreezeExecutionBindingRejectsIncompleteBrokeredAccount(t *testing.T) {
	base := RuntimeProfile{
		ID: "profile.codex.primary", AdapterType: "codex",
		ProviderID: "openai", ProviderAccountID: "openai.primary",
		ModelID: "gpt-5.5-codex", AuthMode: AuthBrokered,
		EndpointFingerprint: strings.Repeat("c", 64),
		CredentialReference: "credential-ref-openai-primary",
		CredentialRevision:  4, Timeout: time.Minute,
	}
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID: "runtime.codex.local", DeviceID: "device.mac", AdapterType: "codex",
		DisplayName: "Codex", Status: RuntimeOnline, Capacity: 1,
	})
	tests := []struct {
		name   string
		mutate func(*RuntimeProfile)
	}{
		{name: "account", mutate: func(p *RuntimeProfile) { p.ProviderAccountID = "" }},
		{name: "endpoint", mutate: func(p *RuntimeProfile) { p.EndpointFingerprint = "" }},
		{name: "credential reference", mutate: func(p *RuntimeProfile) { p.CredentialReference = "" }},
		{name: "non opaque credential reference", mutate: func(p *RuntimeProfile) { p.CredentialReference = "sk-secret-body" }},
		{name: "credential revision", mutate: func(p *RuntimeProfile) { p.CredentialRevision = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := base
			tt.mutate(&profile)
			if binding, err := FreezeExecutionBinding(profile, instance); !errors.Is(err, ErrInvalidExecutionProfile) || binding.BindingDigest != "" {
				t.Fatalf("FreezeExecutionBinding() = (%#v, %v), want ErrInvalidExecutionProfile", binding, err)
			}
		})
	}
}

func TestValidateExecutionProfileRequiresFreezeReadyAccountBinding(t *testing.T) {
	profile := RuntimeProfile{
		ID: "profile.deepseek.primary", AdapterType: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", AuthMode: AuthBrokered,
		EndpointFingerprint: strings.Repeat("d", 64),
		CredentialReference: "credential-ref-deepseek-primary",
		CredentialRevision:  7, Timeout: time.Minute,
	}
	validated, err := ValidateExecutionProfile(profile)
	if err != nil || validated.ProviderAccountID != "deepseek.primary" ||
		validated.CredentialRevision != 7 {
		t.Fatalf("ValidateExecutionProfile(valid) = (%#v, %v)", validated, err)
	}
	invalidNative := profile
	invalidNative.AuthMode = AuthNative
	if _, err := ValidateExecutionProfile(invalidNative); !errors.Is(
		err,
		ErrInvalidExecutionProfile,
	) {
		t.Fatalf("ValidateExecutionProfile(invalid native) error = %v", err)
	}
}

func TestValidateFrozenExecutionBindingRejectsAnyPostFreezeMutation(t *testing.T) {
	budget := int64(300)
	profile := mustRuntimeProfile(t, RuntimeProfile{
		ID: "profile.deepseek.primary", AdapterType: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		ModelID: "deepseek-chat", AuthMode: AuthBrokered,
		EndpointFingerprint: strings.Repeat("d", 64),
		CredentialReference: "credential-ref-deepseek-primary",
		CredentialRevision:  9, Timeout: 90 * time.Second,
		Budget: &budget, RequiredCapabilities: []string{"chat", "tools"},
	})
	instance := mustRuntimeInstance(t, RuntimeInstance{
		ID: "runtime.loom.local", DeviceID: "device.mac",
		AdapterType: "loom-native", DisplayName: "Loom Native",
		Status: RuntimeOnline, ObservedCapabilities: []string{"chat", "tools"},
		Capacity: 2,
	})
	binding, err := FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	if validated, err := ValidateFrozenExecutionBinding(binding); err != nil ||
		validated.BindingDigest != binding.BindingDigest {
		t.Fatalf("ValidateFrozenExecutionBinding() = (%#v, %v)", validated, err)
	}

	mutated := binding
	mutated.CredentialRevision++
	if validated, err := ValidateFrozenExecutionBinding(mutated); !errors.Is(err, ErrInvalidExecutionProfile) || validated.BindingDigest != "" {
		t.Fatalf("mutated binding validation = (%#v, %v)", validated, err)
	}

	mutated = binding
	mutated.Capabilities[0] = "changed-after-freeze"
	if _, err := ValidateFrozenExecutionBinding(mutated); !errors.Is(err, ErrInvalidExecutionProfile) {
		t.Fatalf("mutated capability error = %v", err)
	}
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

func baseEnrollmentProfile(t *testing.T) RuntimeProfile {
	t.Helper()
	profile := RuntimeProfile{
		ID:                         "profile-enrolled",
		AdapterType:                "codex",
		ProviderID:                 "deepseek",
		ProviderAccountID:          "deepseek.primary",
		ModelID:                    "deepseek-chat",
		AuthMode:                   AuthBrokered,
		EndpointFingerprint:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CredentialReference:        "credential-ref-enrolled-0001",
		CredentialRevision:         7,
		ReasoningEffort:            "high",
		RequiredCapabilities:       []string{CapabilityGovernedToolLoop, CapabilityReasoningEffort},
		Timeout:                    90 * time.Second,
		RemoteToolEnrollmentID:     "enr.search.alpha",
		RemoteToolEnrollmentDigest: "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
	}
	budget := int64(5000)
	profile.Budget = &budget
	return profile
}

func TestExecutionProfileFreezesRemoteToolEnrollment(t *testing.T) {
	profile := baseEnrollmentProfile(t)
	instance := RuntimeInstance{
		ID: "runtime-1", DeviceID: "device-1", AdapterType: "codex",
		DisplayName: "Codex", Status: RuntimeOnline,
		ObservedCapabilities: profile.RequiredCapabilities, Capacity: 1,
	}
	validated, err := ValidateExecutionProfile(profile)
	if err != nil {
		t.Fatalf("ValidateExecutionProfile error = %v", err)
	}
	if validated.RemoteToolEnrollmentID != "enr.search.alpha" ||
		validated.RemoteToolEnrollmentDigest != profile.RemoteToolEnrollmentDigest {
		t.Fatalf("enrollment fields lost: %#v", validated)
	}
	binding, err := FreezeExecutionBinding(validated, instance)
	if err != nil {
		t.Fatalf("FreezeExecutionBinding error = %v", err)
	}
	if binding.RemoteToolEnrollmentID != "enr.search.alpha" ||
		binding.RemoteToolEnrollmentDigest != profile.RemoteToolEnrollmentDigest {
		t.Fatalf("binding enrollment fields lost: %#v", binding)
	}
	// The enrollment must participate in the binding digest.
	drifted := validated
	drifted.RemoteToolEnrollmentDigest = strings.Repeat("f", 64)
	driftedBinding, err := FreezeExecutionBinding(drifted, instance)
	if err != nil {
		t.Fatalf("drifted FreezeExecutionBinding error = %v", err)
	}
	if driftedBinding.BindingDigest == binding.BindingDigest {
		t.Fatalf("enrollment digest drift did not change binding digest")
	}
	// Round-trip validation must accept the frozen binding.
	revalidated, err := ValidateFrozenExecutionBinding(binding)
	if err != nil {
		t.Fatalf("ValidateFrozenExecutionBinding error = %v", err)
	}
	if revalidated.BindingDigest != binding.BindingDigest {
		t.Fatalf("round-trip digest mismatch: %q vs %q",
			revalidated.BindingDigest, binding.BindingDigest)
	}
	// A mutated enrollment digest must fail validation.
	corrupt := binding
	corrupt.RemoteToolEnrollmentDigest = strings.Repeat("e", 64)
	if _, err := ValidateFrozenExecutionBinding(corrupt); err == nil {
		t.Fatal("corrupt enrollment digest accepted")
	}
}

func TestExecutionProfileEnrollmentBothOrNeither(t *testing.T) {
	instance := RuntimeInstance{
		ID: "runtime-1", DeviceID: "device-1", AdapterType: "codex",
		DisplayName: "Codex", Status: RuntimeOnline,
		ObservedCapabilities: []string{CapabilityGovernedToolLoop, CapabilityReasoningEffort}, Capacity: 1,
	}
	profile := baseEnrollmentProfile(t)
	profile.ID = "profile-id-only"
	profile.RemoteToolEnrollmentDigest = ""
	if _, err := FreezeExecutionBinding(profile, instance); err == nil {
		t.Fatal("enrollment id without digest accepted")
	}
	profile = baseEnrollmentProfile(t)
	profile.ID = "profile-digest-only"
	profile.RemoteToolEnrollmentID = ""
	if _, err := FreezeExecutionBinding(profile, instance); err == nil {
		t.Fatal("enrollment digest without id accepted")
	}
	profile = baseEnrollmentProfile(t)
	profile.ID = "profile-bad-digest"
	profile.RemoteToolEnrollmentDigest = "not-a-digest"
	if _, err := FreezeExecutionBinding(profile, instance); err == nil {
		t.Fatal("malformed enrollment digest accepted")
	}
	profile = baseEnrollmentProfile(t)
	profile.ID = "profile-no-enrollment"
	profile.RemoteToolEnrollmentID = ""
	profile.RemoteToolEnrollmentDigest = ""
	if _, err := FreezeExecutionBinding(profile, instance); err != nil {
		t.Fatalf("profile without enrollment rejected: %v", err)
	}
}
