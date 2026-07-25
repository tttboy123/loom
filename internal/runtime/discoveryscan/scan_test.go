package discoveryscan

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

var _ ProbeFactory = (*piadapter.PiLocalRuntimeProbeFactory)(nil)

func TestDiscoverConfiguredRuntimesEmptyAndAllAbsent(t *testing.T) {
	empty, err := DiscoverConfiguredRuntimes(context.Background(), nil)
	if err != nil {
		t.Fatalf("DiscoverConfiguredRuntimes(empty) error = %v", err)
	}
	assertValidEmptyDiscoverySnapshot(t, empty)

	var calls []string
	factories := []ProbeFactory{
		&configuredDiscoveryFactory{name: "first", trace: &calls},
		&configuredDiscoveryFactory{name: "second", trace: &calls},
	}
	absent, err := DiscoverConfiguredRuntimes(context.Background(), factories)
	if err != nil {
		t.Fatalf("DiscoverConfiguredRuntimes(all absent) error = %v", err)
	}
	assertValidEmptyDiscoverySnapshot(t, absent)
	if got, want := calls, []string{"factory:first", "factory:second"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("factory call order = %#v, want %#v", got, want)
	}
	if empty.Digest() != absent.Digest() {
		t.Fatalf("empty digest = %q, all-absent digest = %q", empty.Digest(), absent.Digest())
	}
}

func TestDiscoverConfiguredRuntimesBuildsThenDelegatesDeterministically(t *testing.T) {
	var trace []string
	alpha := &configuredDiscoveryProbe{
		id:    "alpha",
		trace: &trace,
		observations: []loomruntime.RuntimeObservation{{
			Instance: configuredDiscoveryInstance("runtime.alpha", func(instance *loomruntime.RuntimeInstance) {
				instance.DeviceID = "device.alpha"
				instance.DisplayName = "Alpha Runtime"
				instance.ObservedCapabilities = []string{"shell", "apply_patch"}
			}),
			ModelIDs: []string{"model.z", "model.a"},
		}},
	}
	bravo := &configuredDiscoveryProbe{
		id:    "bravo",
		trace: &trace,
		observations: []loomruntime.RuntimeObservation{{
			Instance: configuredDiscoveryInstance("runtime.bravo", func(instance *loomruntime.RuntimeInstance) {
				instance.DeviceID = "device.bravo"
				instance.DisplayName = "Bravo Runtime"
				instance.ObservedCapabilities = []string{"go_test", "shell"}
			}),
			ModelIDs: []string{"model.c", "model.b"},
		}},
	}

	var input []ProbeFactory
	first := &configuredDiscoveryFactory{
		name:  "build-bravo",
		trace: &trace,
		build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
			input[1] = &configuredDiscoveryFactory{name: "mutated"}
			return bravo, true, nil
		},
	}
	second := &configuredDiscoveryFactory{
		name:  "absent",
		trace: &trace,
	}
	third := &configuredDiscoveryFactory{
		name:  "build-alpha",
		trace: &trace,
		build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
			return alpha, true, nil
		},
	}
	input = []ProbeFactory{first, second, third}

	snapshot, err := DiscoverConfiguredRuntimes(context.Background(), input)
	if err != nil {
		t.Fatalf("DiscoverConfiguredRuntimes() error = %v", err)
	}
	if got, want := trace, []string{
		"factory:build-bravo",
		"factory:absent",
		"factory:build-alpha",
		"probe:alpha",
		"probe:bravo",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coordination order = %#v, want %#v", got, want)
	}
	assertConfiguredDiscoveryProbeCalls(t, alpha, 1)
	assertConfiguredDiscoveryProbeCalls(t, bravo, 1)

	observations := snapshot.Observations()
	if len(observations) != 2 {
		t.Fatalf("observations = %#v, want two", observations)
	}
	assertConfiguredDiscoveryObservation(
		t,
		observations[0],
		"alpha",
		"runtime.alpha",
		[]string{"apply_patch", "shell"},
		[]string{"model.a", "model.z"},
	)
	assertConfiguredDiscoveryObservation(
		t,
		observations[1],
		"bravo",
		"runtime.bravo",
		[]string{"go_test", "shell"},
		[]string{"model.b", "model.c"},
	)

	direct, err := loomruntime.DiscoverRuntime(context.Background(), []loomruntime.RuntimeProbe{
		&configuredDiscoveryProbe{id: "bravo", observations: bravo.observations},
		&configuredDiscoveryProbe{id: "alpha", observations: alpha.observations},
	})
	if err != nil {
		t.Fatalf("DiscoverRuntime(direct) error = %v", err)
	}
	if snapshot.Digest() != direct.Digest() ||
		!reflect.DeepEqual(snapshot.Observations(), direct.Observations()) {
		t.Fatalf("configured snapshot = %#v, direct snapshot = %#v", snapshot, direct)
	}

	observations[0].Instance.ObservedCapabilities[0] = "mutated"
	observations[0].ModelIDs[0] = "mutated"
	if got := snapshot.Observations()[0]; got.Instance.ObservedCapabilities[0] != "apply_patch" ||
		got.ModelIDs[0] != "model.a" {
		t.Fatalf("snapshot accessor mutation escaped: %#v", got)
	}
}

func TestDiscoverConfiguredRuntimesPrevalidatesInputs(t *testing.T) {
	t.Run("nil context", func(t *testing.T) {
		factory := &configuredDiscoveryFactory{name: "unused"}
		snapshot, err := DiscoverConfiguredRuntimes(nil, []ProbeFactory{factory})
		if !errors.Is(err, ErrInvalidConfiguredRuntimeDiscovery) {
			t.Fatalf("error = %v, want ErrInvalidConfiguredRuntimeDiscovery", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		if factory.calls != 0 {
			t.Fatalf("factory calls = %d, want 0", factory.calls)
		}
	})

	for _, tt := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{name: "canceled", ctx: canceledConfiguredDiscoveryContext(), want: context.Canceled},
		{name: "deadline", ctx: expiredConfiguredDiscoveryContext(), want: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			factory := &configuredDiscoveryFactory{name: "unused"}
			snapshot, err := DiscoverConfiguredRuntimes(tt.ctx, []ProbeFactory{factory})
			if err != tt.want {
				t.Fatalf("error = %v, want exact %v", err, tt.want)
			}
			assertZeroConfiguredDiscoverySnapshot(t, snapshot)
			if factory.calls != 0 {
				t.Fatalf("factory calls = %d, want 0", factory.calls)
			}
		})
	}

	t.Run("accepts exact maximum", func(t *testing.T) {
		var trace []string
		factories := make([]ProbeFactory, MaxProbeFactories)
		concrete := make([]*configuredDiscoveryFactory, MaxProbeFactories)
		wantTrace := make([]string, MaxProbeFactories)
		for index := range factories {
			name := "factory-" + string(rune('a'+index))
			concrete[index] = &configuredDiscoveryFactory{name: name, trace: &trace}
			factories[index] = concrete[index]
			wantTrace[index] = "factory:" + name
		}

		snapshot, err := DiscoverConfiguredRuntimes(context.Background(), factories)
		if err != nil {
			t.Fatalf("DiscoverConfiguredRuntimes(exact maximum) error = %v", err)
		}
		assertValidEmptyDiscoverySnapshot(t, snapshot)
		if !reflect.DeepEqual(trace, wantTrace) {
			t.Fatalf("factory call order = %#v, want %#v", trace, wantTrace)
		}
		for index, factory := range concrete {
			if factory.calls != 1 {
				t.Fatalf("factory %d calls = %d, want 1", index, factory.calls)
			}
		}
	})

	t.Run("too many factories", func(t *testing.T) {
		factory := &configuredDiscoveryFactory{name: "repeated"}
		factories := make([]ProbeFactory, MaxProbeFactories+1)
		for index := range factories {
			factories[index] = factory
		}
		snapshot, err := DiscoverConfiguredRuntimes(context.Background(), factories)
		if !errors.Is(err, ErrInvalidConfiguredRuntimeDiscovery) {
			t.Fatalf("error = %v, want ErrInvalidConfiguredRuntimeDiscovery", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		if factory.calls != 0 {
			t.Fatalf("factory calls = %d, want 0", factory.calls)
		}
	})

	t.Run("nil and typed nil factories", func(t *testing.T) {
		for _, invalid := range []ProbeFactory{nil, (*configuredDiscoveryFactory)(nil)} {
			first := &configuredDiscoveryFactory{name: "first"}
			last := &configuredDiscoveryFactory{name: "last"}
			snapshot, err := DiscoverConfiguredRuntimes(
				context.Background(),
				[]ProbeFactory{first, invalid, last},
			)
			if !errors.Is(err, ErrInvalidConfiguredRuntimeDiscovery) {
				t.Fatalf("factory %#v error = %v, want ErrInvalidConfiguredRuntimeDiscovery", invalid, err)
			}
			assertZeroConfiguredDiscoverySnapshot(t, snapshot)
			if first.calls != 0 || last.calls != 0 {
				t.Fatalf("prevalidation called factories: first=%d last=%d", first.calls, last.calls)
			}
		}
	})
}

func TestDiscoverConfiguredRuntimesRejectsFactoryResultsAndFailures(t *testing.T) {
	typedNilProbe := (*configuredDiscoveryProbe)(nil)
	for _, tt := range []struct {
		name    string
		probe   loomruntime.RuntimeProbe
		present bool
	}{
		{name: "nil present", probe: nil, present: true},
		{name: "typed nil present", probe: typedNilProbe, present: true},
		{name: "probe absent", probe: &configuredDiscoveryProbe{id: "probe"}, present: false},
		{name: "typed nil absent", probe: typedNilProbe, present: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			factory := &configuredDiscoveryFactory{
				name: "invalid-shape",
				build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
					return tt.probe, tt.present, nil
				},
			}
			snapshot, err := DiscoverConfiguredRuntimes(context.Background(), []ProbeFactory{factory})
			if !errors.Is(err, ErrInvalidConfiguredRuntimeDiscovery) {
				t.Fatalf("error = %v, want ErrInvalidConfiguredRuntimeDiscovery", err)
			}
			assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		})
	}

	t.Run("source error remains inspectable without factory disclosure", func(t *testing.T) {
		sourceErr := errors.New("source-build-failure")
		factory := &configuredDiscoveryFactory{
			name: "secret-factory-value",
			build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
				return nil, false, sourceErr
			},
		}
		snapshot, err := DiscoverConfiguredRuntimes(context.Background(), []ProbeFactory{factory})
		if !errors.Is(err, ErrRuntimeProbeFactoryFailed) || !errors.Is(err, sourceErr) {
			t.Fatalf("error = %v, want factory sentinel and source error", err)
		}
		if strings.Contains(err.Error(), factory.name) {
			t.Fatalf("error disclosed factory value: %v", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
	})

	for _, contextErr := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(contextErr.Error(), func(t *testing.T) {
			factory := &configuredDiscoveryFactory{
				name: "context-error",
				build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
					return nil, false, contextErr
				},
			}
			snapshot, err := DiscoverConfiguredRuntimes(context.Background(), []ProbeFactory{factory})
			if err != contextErr {
				t.Fatalf("error = %v, want exact %v", err, contextErr)
			}
			assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		})
	}

	t.Run("later factory failure prevents all observation and later calls", func(t *testing.T) {
		probe := &configuredDiscoveryProbe{id: "collected"}
		sourceErr := errors.New("second failed")
		first := &configuredDiscoveryFactory{
			name: "first",
			build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
				return probe, true, nil
			},
		}
		second := &configuredDiscoveryFactory{
			name: "second",
			build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
				return nil, false, sourceErr
			},
		}
		third := &configuredDiscoveryFactory{name: "third"}

		snapshot, err := DiscoverConfiguredRuntimes(
			context.Background(),
			[]ProbeFactory{first, second, third},
		)
		if !errors.Is(err, ErrRuntimeProbeFactoryFailed) || !errors.Is(err, sourceErr) {
			t.Fatalf("error = %v, want factory sentinel and source error", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		if first.calls != 1 || second.calls != 1 || third.calls != 0 {
			t.Fatalf("factory calls = (%d, %d, %d), want (1, 1, 0)", first.calls, second.calls, third.calls)
		}
		assertConfiguredDiscoveryProbeCalls(t, probe, 0)
	})
}

func TestDiscoverConfiguredRuntimesPropagatesDiscoveryValidation(t *testing.T) {
	t.Run("duplicate probe ids before observation", func(t *testing.T) {
		first := &configuredDiscoveryProbe{id: "same"}
		second := &configuredDiscoveryProbe{id: "same"}
		snapshot, err := DiscoverConfiguredRuntimes(context.Background(), []ProbeFactory{
			presentConfiguredDiscoveryFactory(first),
			presentConfiguredDiscoveryFactory(second),
		})
		if !errors.Is(err, loomruntime.ErrDuplicateRuntimeProbe) {
			t.Fatalf("error = %v, want ErrDuplicateRuntimeProbe", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		assertConfiguredDiscoveryProbeCalls(t, first, 0)
		assertConfiguredDiscoveryProbeCalls(t, second, 0)
	})

	t.Run("invalid probe before observation", func(t *testing.T) {
		valid := &configuredDiscoveryProbe{id: "valid"}
		invalid := &configuredDiscoveryProbe{id: ""}
		snapshot, err := DiscoverConfiguredRuntimes(context.Background(), []ProbeFactory{
			presentConfiguredDiscoveryFactory(valid),
			presentConfiguredDiscoveryFactory(invalid),
		})
		if !errors.Is(err, loomruntime.ErrInvalidRuntimeProbe) {
			t.Fatalf("error = %v, want ErrInvalidRuntimeProbe", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		assertConfiguredDiscoveryProbeCalls(t, valid, 0)
		assertConfiguredDiscoveryProbeCalls(t, invalid, 0)
	})

	t.Run("observation failure", func(t *testing.T) {
		sourceErr := errors.New("observation failed")
		probe := &configuredDiscoveryProbe{id: "probe", err: sourceErr}
		snapshot, err := DiscoverConfiguredRuntimes(
			context.Background(),
			[]ProbeFactory{presentConfiguredDiscoveryFactory(probe)},
		)
		if !errors.Is(err, loomruntime.ErrRuntimeDiscoveryFailed) || !errors.Is(err, sourceErr) {
			t.Fatalf("error = %v, want discovery sentinel and source error", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		assertConfiguredDiscoveryProbeCalls(t, probe, 1)
	})

	t.Run("invalid observation content", func(t *testing.T) {
		probe := &configuredDiscoveryProbe{
			id: "probe",
			observations: []loomruntime.RuntimeObservation{{
				Instance: configuredDiscoveryInstance("runtime.invalid", func(instance *loomruntime.RuntimeInstance) {
					instance.DeviceID = ""
				}),
			}},
		}
		snapshot, err := DiscoverConfiguredRuntimes(
			context.Background(),
			[]ProbeFactory{presentConfiguredDiscoveryFactory(probe)},
		)
		if !errors.Is(err, loomruntime.ErrInvalidRuntimeInstance) {
			t.Fatalf("error = %v, want ErrInvalidRuntimeInstance", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
	})

	t.Run("cancellation during observation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		probe := &configuredDiscoveryProbe{
			id: "probe",
			observe: func(context.Context) ([]loomruntime.RuntimeObservation, error) {
				cancel()
				return nil, context.Canceled
			},
		}
		snapshot, err := DiscoverConfiguredRuntimes(
			ctx,
			[]ProbeFactory{presentConfiguredDiscoveryFactory(probe)},
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
	})

	t.Run("probe result mutation cannot alter snapshot", func(t *testing.T) {
		observations := []loomruntime.RuntimeObservation{{
			Instance: configuredDiscoveryInstance("runtime.mutable", nil),
			ModelIDs: []string{"model.z", "model.a"},
		}}
		probe := &configuredDiscoveryProbe{id: "probe", observations: observations}
		snapshot, err := DiscoverConfiguredRuntimes(
			context.Background(),
			[]ProbeFactory{presentConfiguredDiscoveryFactory(probe)},
		)
		if err != nil {
			t.Fatalf("DiscoverConfiguredRuntimes() error = %v", err)
		}
		observations[0].Instance.ObservedCapabilities[0] = "mutated"
		observations[0].ModelIDs[0] = "mutated"
		got := snapshot.Observations()[0]
		if got.Instance.ObservedCapabilities[0] != "apply_patch" || got.ModelIDs[0] != "model.a" {
			t.Fatalf("probe result mutation escaped: %#v", got)
		}
	})
}

func TestDiscoverConfiguredRuntimesChecksContextAroundFactories(t *testing.T) {
	t.Run("canceled by successful factory", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		probe := &configuredDiscoveryProbe{id: "unobserved"}
		first := &configuredDiscoveryFactory{
			name: "canceling",
			build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
				cancel()
				return probe, true, nil
			},
		}
		last := &configuredDiscoveryFactory{name: "last"}
		snapshot, err := DiscoverConfiguredRuntimes(ctx, []ProbeFactory{first, last})
		if err != context.Canceled {
			t.Fatalf("error = %v, want exact context.Canceled", err)
		}
		assertZeroConfiguredDiscoverySnapshot(t, snapshot)
		if first.calls != 1 || last.calls != 0 {
			t.Fatalf("factory calls = (%d, %d), want (1, 0)", first.calls, last.calls)
		}
		assertConfiguredDiscoveryProbeCalls(t, probe, 0)
	})
}

func TestDiscoverConfiguredRuntimesPiFactoryAbsentIntegration(t *testing.T) {
	searchRoot := t.TempDir()
	isolationRoot := t.TempDir()
	if err := os.Chmod(isolationRoot, 0o700); err != nil {
		t.Fatalf("Chmod(isolation root) error = %v", err)
	}
	factory, err := piadapter.NewPiLocalRuntimeProbeFactory(
		piadapter.PiLocalRuntimeProbeFactoryConfig{
			ProbeID:            "probe.pi.local",
			InstanceID:         "runtime.pi.local",
			DeviceID:           "device.local",
			DisplayName:        "Local Pi",
			IsolationRoot:      isolationRoot,
			RuntimeSearchPaths: []string{searchRoot},
			Timeout:            time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewPiLocalRuntimeProbeFactory() error = %v", err)
	}

	snapshot, err := DiscoverConfiguredRuntimes(
		context.Background(),
		[]ProbeFactory{factory},
	)
	if err != nil {
		t.Fatalf("DiscoverConfiguredRuntimes(Pi absent) error = %v", err)
	}
	assertValidEmptyDiscoverySnapshot(t, snapshot)
}

func TestDiscoverConfiguredRuntimesStaticBoundary(t *testing.T) {
	const filename = "scan.go"
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", filename, err)
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", filename, err)
	}

	allowedImports := map[string]bool{
		"context":                          true,
		"errors":                           true,
		"fmt":                              true,
		"reflect":                          true,
		"loom-pi-rebuild/internal/runtime": true,
	}
	for _, imported := range parsed.Imports {
		importPath := strings.Trim(imported.Path.Value, `"`)
		if !allowedImports[importPath] {
			t.Fatalf("%s imports forbidden package %q", filename, importPath)
		}
	}
	for _, forbidden := range []string{
		"os.",
		"exec.",
		"net.",
		"sql.",
		"time.Sleep(",
		"ticker",
		"Ticker",
		"Sleep(",
		"go func",
		"journal",
		"projection",
		"Append(",
		"Activate",
		"Reserve",
		"RuntimeProfile",
		"piadapter",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("%s contains forbidden behavior marker %q", filename, forbidden)
		}
	}
}

type configuredDiscoveryFactory struct {
	name  string
	trace *[]string
	build func(context.Context) (loomruntime.RuntimeProbe, bool, error)
	calls int
}

func (f *configuredDiscoveryFactory) BuildProbe(
	ctx context.Context,
) (loomruntime.RuntimeProbe, bool, error) {
	f.calls++
	if f.trace != nil {
		*f.trace = append(*f.trace, "factory:"+f.name)
	}
	if f.build != nil {
		return f.build(ctx)
	}
	return nil, false, nil
}

type configuredDiscoveryProbe struct {
	id           string
	trace        *[]string
	observations []loomruntime.RuntimeObservation
	err          error
	observe      func(context.Context) ([]loomruntime.RuntimeObservation, error)
	calls        int
}

func (p *configuredDiscoveryProbe) ID() string {
	return p.id
}

func (p *configuredDiscoveryProbe) ObserveRuntime(
	ctx context.Context,
) ([]loomruntime.RuntimeObservation, error) {
	p.calls++
	if p.trace != nil {
		*p.trace = append(*p.trace, "probe:"+p.id)
	}
	if p.observe != nil {
		return p.observe(ctx)
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.observations, nil
}

func presentConfiguredDiscoveryFactory(probe loomruntime.RuntimeProbe) ProbeFactory {
	return &configuredDiscoveryFactory{
		name: "present",
		build: func(context.Context) (loomruntime.RuntimeProbe, bool, error) {
			return probe, true, nil
		},
	}
}

func configuredDiscoveryInstance(
	id string,
	change func(*loomruntime.RuntimeInstance),
) loomruntime.RuntimeInstance {
	instance := loomruntime.RuntimeInstance{
		ID:                   id,
		DeviceID:             "device.local",
		AdapterType:          "adapter.local",
		DisplayName:          "Local Runtime",
		ExecutableVersion:    "1.0.0",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"apply_patch", "go_test"},
		Capacity:             1,
	}
	if change != nil {
		change(&instance)
	}
	return instance
}

func assertConfiguredDiscoveryProbeCalls(
	t *testing.T,
	probe *configuredDiscoveryProbe,
	want int,
) {
	t.Helper()
	if probe.calls != want {
		t.Fatalf("probe %q calls = %d, want %d", probe.id, probe.calls, want)
	}
}

func assertConfiguredDiscoveryObservation(
	t *testing.T,
	observation loomruntime.RuntimeObservation,
	wantProbeID string,
	wantInstanceID string,
	wantCapabilities []string,
	wantModels []string,
) {
	t.Helper()
	if observation.SourceProbeID != wantProbeID ||
		observation.Instance.ID != wantInstanceID ||
		!reflect.DeepEqual(observation.Instance.ObservedCapabilities, wantCapabilities) ||
		!reflect.DeepEqual(observation.ModelIDs, wantModels) {
		t.Fatalf(
			"observation = %#v, want probe=%q instance=%q capabilities=%#v models=%#v",
			observation,
			wantProbeID,
			wantInstanceID,
			wantCapabilities,
			wantModels,
		)
	}
}

func assertValidEmptyDiscoverySnapshot(
	t *testing.T,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) {
	t.Helper()
	if snapshot.Digest() == "" || len(snapshot.Observations()) != 0 {
		t.Fatalf("snapshot = %#v, want valid nonzero-digest empty snapshot", snapshot)
	}
}

func assertZeroConfiguredDiscoverySnapshot(
	t *testing.T,
	snapshot loomruntime.RuntimeDiscoverySnapshot,
) {
	t.Helper()
	if snapshot.Digest() != "" || len(snapshot.Observations()) != 0 {
		t.Fatalf("snapshot = %#v, want zero snapshot", snapshot)
	}
}

func canceledConfiguredDiscoveryContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredConfiguredDiscoveryContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	cancel()
	return ctx
}
