package runtime

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestDiscoverRuntimePrevalidatesProbeSetBeforeInvocation(t *testing.T) {
	valid := &recordingRuntimeProbe{
		id: "zeta",
		observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.zeta", func(i *RuntimeInstance) {
				i.DeviceID = "device.zeta"
			}),
		}},
	}

	tests := []struct {
		name   string
		probes []RuntimeProbe
		wantIs error
	}{
		{name: "nil probe", probes: []RuntimeProbe{nil, valid}, wantIs: ErrInvalidRuntimeProbe},
		{name: "empty probe id", probes: []RuntimeProbe{&recordingRuntimeProbe{id: ""}, valid}, wantIs: ErrInvalidRuntimeProbe},
		{name: "duplicate probe id", probes: []RuntimeProbe{&recordingRuntimeProbe{id: "same"}, &recordingRuntimeProbe{id: "same"}, valid}, wantIs: ErrDuplicateRuntimeProbe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, probe := range tt.probes {
				if probe, ok := probe.(*recordingRuntimeProbe); ok {
					probe.reset()
				}
			}

			snapshot, err := DiscoverRuntime(context.Background(), tt.probes)
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("DiscoverRuntime() error = %v, want errors.Is(%v)", err, tt.wantIs)
			}
			assertZeroRuntimeDiscoverySnapshot(t, snapshot)
			for _, probe := range tt.probes {
				if probe == nil {
					continue
				}
				recording, ok := probe.(*recordingRuntimeProbe)
				if !ok {
					t.Fatalf("test probe %T is not inspectable", probe)
				}
				if got := recording.callCount(); got != 0 {
					t.Fatalf("probe ID %q was invoked %d times during prevalidation failure", recording.id, got)
				}
			}
		})
	}
}

func TestDiscoverRuntimeEmptyProbeSetReturnsDeterministicEmptySnapshot(t *testing.T) {
	first, err := DiscoverRuntime(context.Background(), []RuntimeProbe{})
	if err != nil {
		t.Fatalf("DiscoverRuntime(empty) error = %v", err)
	}
	if observations := first.Observations(); len(observations) != 0 {
		t.Fatalf("empty probe set observations = %#v, want empty", observations)
	}
	if digest := first.Digest(); !isLowercaseSHA256Digest(digest) {
		t.Fatalf("empty probe set digest %q is not lowercase SHA-256 hex", digest)
	}

	second, err := DiscoverRuntime(context.Background(), []RuntimeProbe{})
	if err != nil {
		t.Fatalf("DiscoverRuntime(empty repeat) error = %v", err)
	}
	if first.Digest() != second.Digest() {
		t.Fatalf("empty probe set digest changed: first %q second %q", first.Digest(), second.Digest())
	}
}

func TestDiscoverRuntimeDeterministicProbeInvocationAndSnapshotOrder(t *testing.T) {
	var trace []string
	alpha := &recordingRuntimeProbe{id: "alpha", trace: &trace, observations: []RuntimeObservation{{
		Instance: discoveryTestInstance("runtime.alpha", func(i *RuntimeInstance) {
			i.DeviceID = "device.alpha"
			i.DisplayName = "Alpha Runtime"
			i.ObservedCapabilities = []string{"shell", "apply_patch"}
		}),
		ModelIDs: []string{"model.z", "model.a"},
	}}}
	bravo := &recordingRuntimeProbe{id: "bravo", trace: &trace, observations: []RuntimeObservation{{
		Instance: discoveryTestInstance("runtime.bravo", func(i *RuntimeInstance) {
			i.DeviceID = "device.bravo"
			i.DisplayName = "Bravo Runtime"
			i.ObservedCapabilities = []string{"go_test", "shell"}
		}),
		ModelIDs: []string{"model.c", "model.b"},
	}}}

	first, err := DiscoverRuntime(context.Background(), []RuntimeProbe{bravo, alpha})
	if err != nil {
		t.Fatalf("DiscoverRuntime(first) error = %v", err)
	}
	if got, want := trace, []string{"alpha", "bravo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("probe invocation order = %#v, want %#v", got, want)
	}
	assertRuntimeProbeIDCalls(t, alpha, 1)
	assertRuntimeProbeIDCalls(t, bravo, 1)
	assertDiscoveryObservationOrder(t, first, []string{"runtime.alpha", "runtime.bravo"})
	firstObservations := first.Observations()
	assertDiscoveryObservation(t, firstObservations[0], "alpha", []string{"apply_patch", "shell"}, []string{"model.a", "model.z"})
	assertDiscoveryObservation(t, firstObservations[1], "bravo", []string{"go_test", "shell"}, []string{"model.b", "model.c"})

	trace = nil
	alpha.reset()
	bravo.reset()
	second, err := DiscoverRuntime(context.Background(), []RuntimeProbe{alpha, bravo})
	if err != nil {
		t.Fatalf("DiscoverRuntime(second) error = %v", err)
	}
	if got, want := trace, []string{"alpha", "bravo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("probe invocation order after reorder = %#v, want %#v", got, want)
	}
	assertRuntimeProbeIDCalls(t, alpha, 1)
	assertRuntimeProbeIDCalls(t, bravo, 1)
	if first.Digest() != second.Digest() {
		t.Fatalf("digest differs under probe reordering: first %q second %q", first.Digest(), second.Digest())
	}
	if !reflect.DeepEqual(first.Observations(), second.Observations()) {
		t.Fatalf("snapshot differs under probe reordering:\nfirst: %#v\nsecond: %#v", first.Observations(), second.Observations())
	}
}

func TestDiscoverRuntimeRevalidatesRuntimeInstanceAndDuplicateIDs(t *testing.T) {
	t.Run("invalid instance", func(t *testing.T) {
		probe := &recordingRuntimeProbe{id: "probe", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.bad", func(i *RuntimeInstance) {
				i.DeviceID = ""
			}),
		}}}

		snapshot, err := DiscoverRuntime(context.Background(), []RuntimeProbe{probe})
		if !errors.Is(err, ErrInvalidRuntimeInstance) {
			t.Fatalf("DiscoverRuntime() error = %v, want ErrInvalidRuntimeInstance", err)
		}
		assertZeroRuntimeDiscoverySnapshot(t, snapshot)
	})

	t.Run("duplicate instance id", func(t *testing.T) {
		probes := []RuntimeProbe{
			&recordingRuntimeProbe{id: "alpha", observations: []RuntimeObservation{{
				Instance: discoveryTestInstance("runtime.same", func(i *RuntimeInstance) {
					i.DeviceID = "device.alpha"
				}),
			}}},
			&recordingRuntimeProbe{id: "bravo", observations: []RuntimeObservation{{
				Instance: discoveryTestInstance("runtime.same", func(i *RuntimeInstance) {
					i.DeviceID = "device.bravo"
				}),
			}}},
		}

		snapshot, err := DiscoverRuntime(context.Background(), probes)
		if !errors.Is(err, ErrDuplicateRuntimeInstance) {
			t.Fatalf("DiscoverRuntime() error = %v, want ErrDuplicateRuntimeInstance", err)
		}
		assertZeroRuntimeDiscoverySnapshot(t, snapshot)
	})
}

func TestDiscoverRuntimeModelInventoryNormalizationAndValidation(t *testing.T) {
	t.Run("normalizes model inventory", func(t *testing.T) {
		probe := &recordingRuntimeProbe{id: "probe", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.models", nil),
			ModelIDs: []string{"model.z", "model.a", "model.m"},
		}}}

		snapshot, err := DiscoverRuntime(context.Background(), []RuntimeProbe{probe})
		if err != nil {
			t.Fatalf("DiscoverRuntime() error = %v", err)
		}
		assertRuntimeProbeIDCalls(t, probe, 1)
		assertDiscoveryObservation(t, snapshot.Observations()[0], "probe", []string{"apply_patch", "go_test"}, []string{"model.a", "model.m", "model.z"})
	})

	tests := []struct {
		name   string
		models []string
		wantIs error
	}{
		{name: "empty model id", models: []string{"model.a", ""}, wantIs: ErrInvalidRuntimeModel},
		{name: "duplicate model id", models: []string{"model.a", "model.a"}, wantIs: ErrDuplicateRuntimeModel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := &recordingRuntimeProbe{id: "probe", observations: []RuntimeObservation{{
				Instance: discoveryTestInstance("runtime.models", nil),
				ModelIDs: tt.models,
			}}}

			snapshot, err := DiscoverRuntime(context.Background(), []RuntimeProbe{probe})
			if !errors.Is(err, tt.wantIs) {
				t.Fatalf("DiscoverRuntime() error = %v, want errors.Is(%v)", err, tt.wantIs)
			}
			assertZeroRuntimeDiscoverySnapshot(t, snapshot)
		})
	}
}

func TestDiscoverRuntimeCancellationAndProbeFailureReturnNoPartialSnapshot(t *testing.T) {
	t.Run("context canceled before invocation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		probe := &recordingRuntimeProbe{id: "probe", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.canceled", nil),
		}}}

		snapshot, err := DiscoverRuntime(ctx, []RuntimeProbe{probe})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DiscoverRuntime() error = %v, want context.Canceled", err)
		}
		assertZeroRuntimeDiscoverySnapshot(t, snapshot)
		assertRuntimeProbeIDCalls(t, probe, 1)
		if got := probe.callCount(); got != 0 {
			t.Fatalf("probe call count = %d, want 0 after pre-canceled context", got)
		}
	})

	t.Run("context canceled during invocation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		alpha := &recordingRuntimeProbe{id: "alpha", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.alpha", nil),
		}}}
		bravo := &recordingRuntimeProbe{id: "bravo", observe: func(ctx context.Context) ([]RuntimeObservation, error) {
			cancel()
			return nil, ctx.Err()
		}}
		charlie := &recordingRuntimeProbe{id: "charlie", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.charlie", nil),
		}}}

		snapshot, err := DiscoverRuntime(ctx, []RuntimeProbe{charlie, bravo, alpha})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DiscoverRuntime() error = %v, want context.Canceled", err)
		}
		assertZeroRuntimeDiscoverySnapshot(t, snapshot)
		assertRuntimeProbeIDCalls(t, alpha, 1)
		assertRuntimeProbeIDCalls(t, bravo, 1)
		assertRuntimeProbeIDCalls(t, charlie, 1)
		if got := charlie.callCount(); got != 0 {
			t.Fatalf("later probe call count = %d, want 0 after cancellation", got)
		}
	})

	t.Run("probe failure", func(t *testing.T) {
		probeErr := errors.New("probe failed")
		alpha := &recordingRuntimeProbe{id: "alpha", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.alpha", nil),
		}}}
		bravo := &recordingRuntimeProbe{id: "bravo", err: probeErr}
		charlie := &recordingRuntimeProbe{id: "charlie", observations: []RuntimeObservation{{
			Instance: discoveryTestInstance("runtime.charlie", nil),
		}}}

		snapshot, err := DiscoverRuntime(context.Background(), []RuntimeProbe{charlie, bravo, alpha})
		if !errors.Is(err, ErrRuntimeDiscoveryFailed) || !errors.Is(err, probeErr) {
			t.Fatalf("DiscoverRuntime() error = %v, want ErrRuntimeDiscoveryFailed wrapping probe error", err)
		}
		if !strings.Contains(err.Error(), `probe "bravo"`) {
			t.Fatalf("DiscoverRuntime() error = %q, want cached probe ID label %q", err.Error(), `probe "bravo"`)
		}
		assertZeroRuntimeDiscoverySnapshot(t, snapshot)
		assertRuntimeProbeIDCalls(t, alpha, 1)
		assertRuntimeProbeIDCalls(t, bravo, 1)
		assertRuntimeProbeIDCalls(t, charlie, 1)
		if got := charlie.callCount(); got != 0 {
			t.Fatalf("later probe call count = %d, want 0 after probe failure", got)
		}
	})
}

func TestRuntimeDiscoveryDigestStableUnderReorderingAndSensitiveToIncludedFields(t *testing.T) {
	baseline := []RuntimeProbe{
		discoveryDigestProbe("bravo", "runtime.bravo", func(i *RuntimeInstance) {
			i.DeviceID = "device.bravo"
			i.AdapterType = "adapter.bravo"
			i.DisplayName = "Bravo"
			i.ExecutableVersion = "2.0.0"
			i.Status = RuntimeOffline
			i.ObservedCapabilities = []string{"shell", "go_test"}
			i.Capacity = 2
		}, []string{"model.b", "model.a"}),
		discoveryDigestProbe("alpha", "runtime.alpha", nil, []string{"model.y", "model.x"}),
	}

	first := mustDiscoverRuntimeSnapshot(t, baseline)
	reordered := mustDiscoverRuntimeSnapshot(t, []RuntimeProbe{baseline[1], baseline[0]})
	if first.Digest() != reordered.Digest() {
		t.Fatalf("digest differs under input reordering: first %q reordered %q", first.Digest(), reordered.Digest())
	}
	if !isLowercaseSHA256Digest(first.Digest()) {
		t.Fatalf("digest %q is not lowercase SHA-256 hex", first.Digest())
	}

	tests := []struct {
		name   string
		probes []RuntimeProbe
	}{
		{name: "source probe id", probes: []RuntimeProbe{
			discoveryDigestProbe("changed", "runtime.alpha", nil, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "runtime id", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.changed", nil, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "device id", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.DeviceID = "device.changed" }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "adapter type", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.AdapterType = "adapter.changed" }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "display name", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.DisplayName = "Changed" }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "executable version", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.ExecutableVersion = "9.9.9" }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "status", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.Status = RuntimeDisabled }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "capabilities", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.ObservedCapabilities = []string{"apply_patch", "shell"} }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "capacity", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", func(i *RuntimeInstance) { i.Capacity = 3 }, []string{"model.y", "model.x"}),
			baseline[0],
		}},
		{name: "model ids", probes: []RuntimeProbe{
			discoveryDigestProbe("alpha", "runtime.alpha", nil, []string{"model.y", "model.changed"}),
			baseline[0],
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := mustDiscoverRuntimeSnapshot(t, tt.probes)
			if changed.Digest() == first.Digest() {
				t.Fatalf("digest did not change when included field class %q changed", tt.name)
			}
		})
	}
}

func TestRuntimeDiscoveryMutationIsolation(t *testing.T) {
	probe := &recordingRuntimeProbe{id: "probe", observations: []RuntimeObservation{{
		SourceProbeID: "untrusted-probe-id",
		Instance: discoveryTestInstance("runtime.mutable", func(i *RuntimeInstance) {
			i.ObservedCapabilities = []string{"z_cap", "a_cap"}
		}),
		ModelIDs: []string{"model.z", "model.a"},
	}}}

	snapshot := mustDiscoverRuntimeSnapshot(t, []RuntimeProbe{probe})
	probe.observations[0].Instance.ID = "runtime.mutated"
	probe.observations[0].Instance.ObservedCapabilities[0] = "mutated_cap"
	probe.observations[0].ModelIDs[0] = "model.mutated"

	assertDiscoveryObservation(t, snapshot.Observations()[0], "probe", []string{"a_cap", "z_cap"}, []string{"model.a", "model.z"})

	originalDigest := snapshot.Digest()
	returned := snapshot.Observations()
	returned[0].Instance.ID = "runtime.changed"
	returned[0].Instance.ObservedCapabilities[0] = "changed_cap"
	returned[0].ModelIDs[0] = "model.changed"

	if snapshot.Digest() != originalDigest {
		t.Fatalf("snapshot digest changed after mutating returned observations: got %q want %q", snapshot.Digest(), originalDigest)
	}
	assertDiscoveryObservation(t, snapshot.Observations()[0], "probe", []string{"a_cap", "z_cap"}, []string{"model.a", "model.z"})

	freshProbe := &recordingRuntimeProbe{id: "probe", observations: []RuntimeObservation{{
		SourceProbeID: "untrusted-probe-id",
		Instance: discoveryTestInstance("runtime.mutable", func(i *RuntimeInstance) {
			i.ObservedCapabilities = []string{"z_cap", "a_cap"}
		}),
		ModelIDs: []string{"model.z", "model.a"},
	}}}
	fresh := mustDiscoverRuntimeSnapshot(t, []RuntimeProbe{freshProbe})
	if fresh.Digest() != originalDigest {
		t.Fatalf("fresh digest = %q after returned snapshot mutation, want original %q", fresh.Digest(), originalDigest)
	}
	assertDiscoveryObservation(t, fresh.Observations()[0], "probe", []string{"a_cap", "z_cap"}, []string{"model.a", "model.z"})
}

func TestRuntimeDiscoveryProductionImportBoundary(t *testing.T) {
	forbidden := map[string]bool{
		"database/sql":  true,
		"net":           true,
		"net/http":      true,
		"os":            true,
		"os/exec":       true,
		"path/filepath": true,
		"syscall":       true,

		"loom-pi-rebuild/internal/agents":      true,
		"loom-pi-rebuild/internal/api":         true,
		"loom-pi-rebuild/internal/app":         true,
		"loom-pi-rebuild/internal/config":      true,
		"loom-pi-rebuild/internal/credentials": true,
		"loom-pi-rebuild/internal/evidence":    true,
		"loom-pi-rebuild/internal/journal":     true,
		"loom-pi-rebuild/internal/mode":        true,
		"loom-pi-rebuild/internal/projection":  true,
		"loom-pi-rebuild/internal/supervisor":  true,
	}

	assertRuntimeDiscoveryProductionImports(t, "discovery.go", forbidden)
}

type recordingRuntimeProbe struct {
	id           string
	observations []RuntimeObservation
	err          error
	observe      func(context.Context) ([]RuntimeObservation, error)
	calls        []string
	trace        *[]string
	idCalls      int
}

func (p *recordingRuntimeProbe) ID() string {
	p.idCalls++
	return p.id
}

func (p *recordingRuntimeProbe) ObserveRuntime(ctx context.Context) ([]RuntimeObservation, error) {
	p.calls = append(p.calls, p.id)
	if p.trace != nil {
		*p.trace = append(*p.trace, p.id)
	}
	if p.observe != nil {
		return p.observe(ctx)
	}
	if p.err != nil {
		return nil, p.err
	}
	return slices.Clone(p.observations), nil
}

func (p *recordingRuntimeProbe) callCount() int {
	return len(p.calls)
}

func (p *recordingRuntimeProbe) reset() {
	p.calls = nil
	p.idCalls = 0
}

func discoveryDigestProbe(id string, instanceID string, change func(*RuntimeInstance), models []string) RuntimeProbe {
	return &recordingRuntimeProbe{id: id, observations: []RuntimeObservation{{
		Instance: discoveryTestInstance(instanceID, change),
		ModelIDs: models,
	}}}
}

func discoveryTestInstance(id string, change func(*RuntimeInstance)) RuntimeInstance {
	instance := RuntimeInstance{
		ID:                   id,
		DeviceID:             "device.local",
		AdapterType:          "adapter.local",
		DisplayName:          "Local Runtime",
		ExecutableVersion:    "1.0.0",
		Status:               RuntimeOnline,
		ObservedCapabilities: []string{"go_test", "apply_patch"},
		Capacity:             1,
	}
	if change != nil {
		change(&instance)
	}
	return instance
}

func mustDiscoverRuntimeSnapshot(t *testing.T, probes []RuntimeProbe) RuntimeDiscoverySnapshot {
	t.Helper()

	snapshot, err := DiscoverRuntime(context.Background(), probes)
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	return snapshot
}

func assertZeroRuntimeDiscoverySnapshot(t *testing.T, snapshot RuntimeDiscoverySnapshot) {
	t.Helper()

	if snapshot.Digest() != "" || len(snapshot.Observations()) != 0 {
		t.Fatalf("snapshot = %#v, want no usable partial snapshot", snapshot)
	}
}

func assertDiscoveryObservationOrder(t *testing.T, snapshot RuntimeDiscoverySnapshot, wantIDs []string) {
	t.Helper()

	observations := snapshot.Observations()
	got := make([]string, 0, len(observations))
	for _, observation := range observations {
		got = append(got, observation.Instance.ID)
	}
	if !reflect.DeepEqual(got, wantIDs) {
		t.Fatalf("observation order = %#v, want %#v", got, wantIDs)
	}
}

func assertDiscoveryObservation(t *testing.T, observation RuntimeObservation, wantProbeID string, wantCapabilities []string, wantModels []string) {
	t.Helper()

	if observation.SourceProbeID != wantProbeID {
		t.Fatalf("SourceProbeID = %q, want coordinator-assigned %q", observation.SourceProbeID, wantProbeID)
	}
	if !reflect.DeepEqual(observation.Instance.ObservedCapabilities, wantCapabilities) {
		t.Fatalf("ObservedCapabilities = %#v, want %#v", observation.Instance.ObservedCapabilities, wantCapabilities)
	}
	if !reflect.DeepEqual(observation.ModelIDs, wantModels) {
		t.Fatalf("ModelIDs = %#v, want %#v", observation.ModelIDs, wantModels)
	}
}

func assertRuntimeProbeIDCalls(t *testing.T, probe *recordingRuntimeProbe, want int) {
	t.Helper()

	if probe.idCalls != want {
		t.Fatalf("probe %q ID() call count = %d, want %d", probe.id, probe.idCalls, want)
	}
}

func isLowercaseSHA256Digest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, r := range digest {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func assertRuntimeDiscoveryProductionImports(t *testing.T, filename string, forbidden map[string]bool) {
	t.Helper()

	if _, err := os.Stat(filename); err != nil {
		t.Fatalf("expected production discovery file %s: %v", filename, err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", filename, err)
	}
	for _, imported := range file.Imports {
		importPath := strings.Trim(imported.Path.Value, `"`)
		if forbidden[importPath] {
			t.Fatalf("%s imports forbidden boundary package %q", filename, importPath)
		}
		if strings.HasPrefix(importPath, "loom-pi-rebuild/internal/runtime/") {
			t.Fatalf("%s imports concrete runtime adapter package %q", filename, importPath)
		}
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		if strings.Contains(strings.ToLower(entry.Name()), "adapter") {
			t.Fatalf("production runtime file %q appears to introduce a concrete adapter in S2-W2", entry.Name())
		}
	}
}
