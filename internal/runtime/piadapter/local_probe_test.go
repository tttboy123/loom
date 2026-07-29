package piadapter

import (
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

func TestPiLocalRuntimeProbeFactoryConstructorValidation(t *testing.T) {
	search := privateTempDir(t)
	valid := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})

	rootFile := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(rootFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	rootTarget := privateTempDir(t)
	rootLink := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(rootTarget, rootLink); err != nil {
		t.Fatal(err)
	}
	publicRoot := t.TempDir()
	if err := os.Chmod(publicRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	searchFile := filepath.Join(t.TempDir(), "search-file")
	if err := os.WriteFile(searchFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		change func(*PiLocalRuntimeProbeFactoryConfig)
	}{
		{name: "empty probe id", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.ProbeID = "" }},
		{name: "empty instance id", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.InstanceID = "" }},
		{name: "empty device id", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.DeviceID = "" }},
		{name: "empty display name", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.DisplayName = "" }},
		{name: "zero timeout", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.Timeout = 0 }},
		{name: "negative timeout", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.Timeout = -time.Second }},
		{name: "timeout too large", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.Timeout = 30*time.Second + time.Nanosecond }},
		{name: "empty root", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot = "" }},
		{name: "relative root", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot = "state" }},
		{name: "unclean root", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot += string(os.PathSeparator) + "." }},
		{name: "nul root", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot += "\x00" }},
		{name: "missing root", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot = filepath.Join(t.TempDir(), "missing") }},
		{name: "root file", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot = rootFile }},
		{name: "root symlink", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot = rootLink }},
		{name: "root not private", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.IsolationRoot = publicRoot }},
		{name: "empty search paths", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.RuntimeSearchPaths = nil }},
		{name: "relative search path", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.RuntimeSearchPaths = []string{"bin"} }},
		{name: "unclean search path", change: func(c *PiLocalRuntimeProbeFactoryConfig) {
			c.RuntimeSearchPaths = []string{search + string(os.PathSeparator) + "."}
		}},
		{name: "nul search path", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.RuntimeSearchPaths = []string{search + "\x00"} }},
		{name: "missing search path", change: func(c *PiLocalRuntimeProbeFactoryConfig) {
			c.RuntimeSearchPaths = []string{filepath.Join(t.TempDir(), "missing")}
		}},
		{name: "search path file", change: func(c *PiLocalRuntimeProbeFactoryConfig) { c.RuntimeSearchPaths = []string{searchFile} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := clonePiLocalRuntimeProbeFactoryConfig(valid)
			test.change(&config)
			factory, err := NewPiLocalRuntimeProbeFactory(config)
			if !errors.Is(err, ErrInvalidPiLocalRuntimeProbeFactory) {
				t.Fatalf("NewPiLocalRuntimeProbeFactory() error = %v, want ErrInvalidPiLocalRuntimeProbeFactory", err)
			}
			if factory != nil {
				t.Fatalf("factory = %#v, want nil", factory)
			}
		})
	}

	t.Run("inaccessible symlink target", func(t *testing.T) {
		if goruntime.GOOS == "windows" {
			t.Skip("directory search permission fixture")
		}
		first := privateTempDir(t)
		blocked := privateTempDir(t)
		target := writeNamedLocalPiFixture(t, blocked, "target", "1.0.0", filepath.Join(t.TempDir(), "marker"))
		if err := os.Symlink(target, filepath.Join(first, "pi")); err != nil {
			t.Fatal(err)
		}
		config := validPiLocalRuntimeProbeFactoryConfig(t, []string{first})
		factory := mustPiLocalRuntimeProbeFactory(t, config)
		if err := os.Chmod(blocked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

		probe, found, err := factory.BuildProbe(context.Background())
		if err == nil {
			t.Skip("filesystem permissions are not enforced for this test user")
		}
		assertNoPiLocalProbe(t, probe, found)
		if !errors.Is(err, ErrPiLocalRuntimeCandidateInvalid) {
			t.Fatalf("BuildProbe() error = %v, want ErrPiLocalRuntimeCandidateInvalid", err)
		}
	})
}

func TestPiLocalRuntimeProbeFactoryCopiesAndCanonicalizesSearchPaths(t *testing.T) {
	first := privateTempDir(t)
	firstAlias := filepath.Join(t.TempDir(), "first-alias")
	if err := os.Symlink(first, firstAlias); err != nil {
		t.Fatal(err)
	}
	second := privateTempDir(t)
	writeLocalPiFixture(t, second, "2.0.0", filepath.Join(t.TempDir(), "invoked"))
	searchPaths := []string{firstAlias, first, second}
	config := validPiLocalRuntimeProbeFactoryConfig(t, searchPaths)
	factory := mustPiLocalRuntimeProbeFactory(t, config)

	searchPaths[0] = filepath.Join(t.TempDir(), "mutated")
	config.RuntimeSearchPaths[1] = filepath.Join(t.TempDir(), "also-mutated")
	if got := len(factory.searchPaths); got != 2 {
		t.Fatalf("canonical search path count = %d, want 2", got)
	}
	probe, found, err := factory.BuildProbe(context.Background())
	if err != nil || !found || probe == nil {
		t.Fatalf("BuildProbe() = (%#v, %v, %v), want probe,true,nil", probe, found, err)
	}
}

func TestPiLocalRuntimeProbeFactoryContextAndAbsentInstallation(t *testing.T) {
	search := privateTempDir(t)
	config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
	factory := mustPiLocalRuntimeProbeFactory(t, config)

	probe, found, err := factory.BuildProbe(nil)
	assertNoPiLocalProbe(t, probe, found)
	if !errors.Is(err, ErrInvalidPiLocalRuntimeProbeRequest) {
		t.Fatalf("BuildProbe(nil) error = %v, want ErrInvalidPiLocalRuntimeProbeRequest", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	probe, found, err = factory.BuildProbe(ctx)
	assertNoPiLocalProbe(t, probe, found)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("BuildProbe(canceled) error = %v, want context.Canceled", err)
	}

	deadline, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	probe, found, err = factory.BuildProbe(deadline)
	assertNoPiLocalProbe(t, probe, found)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("BuildProbe(expired) error = %v, want context deadline", err)
	}

	probe, found, err = factory.BuildProbe(context.Background())
	if err != nil {
		t.Fatalf("BuildProbe(absent) error = %v", err)
	}
	assertNoPiLocalProbe(t, probe, found)
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiLocalRuntimeProbeFactoryCarriesCatalogThroughRealParser(t *testing.T) {
	root := piLocalModelPrivateRoot(t, "probe-catalog")
	serverConfig, digest := piLocalModelInspectorFixture(t, root)
	fixture := makePiMetadataFixture(t, "catalog-success")
	search := filepath.Dir(fixture)
	executable := filepath.Join(search, piLocalRuntimeExecutableName)
	if err := os.Rename(fixture, executable); err != nil {
		t.Fatal(err)
	}
	config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
	config.LocalModelCatalog = &PiLocalModelCatalogConfig{
		PrivateRoot:    serverConfig.PrivateRoot,
		ExecutablePath: serverConfig.ExecutablePath,
		ModelPath:      serverConfig.ModelPath,
	}
	expected, err := piLocalModelCatalogJSON(piLocalModelCatalogBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureControl(
		t,
		executable,
		"expected-path",
		canonicalSearchPath(t, config.RuntimeSearchPaths),
	)
	writeFixtureControl(t, executable, "expected-models-json", string(expected))
	factory, err := newPiLocalRuntimeProbeFactory(config, digest)
	if err != nil {
		t.Fatalf("newPiLocalRuntimeProbeFactory() error = %v", err)
	}
	probe, found, err := factory.BuildProbe(context.Background())
	if err != nil || !found || probe == nil {
		t.Fatalf("BuildProbe() = (%#v, %v, %v)", probe, found, err)
	}
	snapshot, err := loomruntime.DiscoverRuntime(
		context.Background(),
		[]loomruntime.RuntimeProbe{probe},
	)
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	observations := snapshot.Observations()
	wantModel := piRPCProviderID + "/" + piRPCModelID
	if len(observations) != 1 ||
		!reflect.DeepEqual(observations[0].ModelIDs, []string{wantModel}) ||
		observations[0].Instance.ExecutableVersion != "0.82.1" {
		t.Fatalf("observations = %#v", observations)
	}
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiLocalRuntimeProbeFactoryRejectsSameDigestCatalogIdentityReplacement(
	t *testing.T,
) {
	root := piLocalModelPrivateRoot(t, "probe-catalog-replacement")
	serverConfig, digest := piLocalModelInspectorFixture(t, root)
	fixture := makePiMetadataFixture(t, "catalog-success")
	search := filepath.Dir(fixture)
	executable := filepath.Join(search, piLocalRuntimeExecutableName)
	if err := os.Rename(fixture, executable); err != nil {
		t.Fatal(err)
	}
	config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
	config.LocalModelCatalog = &PiLocalModelCatalogConfig{
		PrivateRoot:    serverConfig.PrivateRoot,
		ExecutablePath: serverConfig.ExecutablePath,
		ModelPath:      serverConfig.ModelPath,
	}
	factory, err := newPiLocalRuntimeProbeFactory(config, digest)
	if err != nil {
		t.Fatalf("newPiLocalRuntimeProbeFactory() error = %v", err)
	}
	content, err := os.ReadFile(serverConfig.ModelPath)
	if err != nil {
		t.Fatal(err)
	}
	replaced := serverConfig.ModelPath + ".replaced"
	if err := os.Rename(serverConfig.ModelPath, replaced); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serverConfig.ModelPath, content, 0o600); err != nil {
		t.Fatal(err)
	}

	probe, found, err := factory.BuildProbe(context.Background())
	assertNoPiLocalProbe(t, probe, found)
	if !errors.Is(err, ErrPiLocalRuntimeProbeConstructionFailed) {
		t.Fatalf(
			"BuildProbe() error = %v, want ErrPiLocalRuntimeProbeConstructionFailed",
			err,
		)
	}
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiLocalRuntimeProbeFactoryUsesOnlyFixedOrderedConfiguredCandidates(t *testing.T) {
	t.Setenv("LOOM_PI_LOCAL_FACTORY_SECRET", "secret-parent-value")
	ambient := privateTempDir(t)
	ambientMarker := filepath.Join(t.TempDir(), "ambient-marker")
	writeLocalPiFixture(t, ambient, "9.9.9", ambientMarker)
	t.Setenv("PATH", ambient)

	first := privateTempDir(t)
	second := privateTempDir(t)
	firstMarker := filepath.Join(t.TempDir(), "first-marker")
	secondMarker := filepath.Join(t.TempDir(), "second-marker")
	writeLocalPiFixture(t, first, "1.0.0", firstMarker)
	writeLocalPiFixture(t, second, "2.0.0", secondMarker)
	if err := os.WriteFile(filepath.Join(first, "pi-alias"), []byte("#!/bin/sh\nexit 90\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(first, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	writeLocalPiFixture(t, nested, "8.8.8", filepath.Join(t.TempDir(), "nested-marker"))

	config := validPiLocalRuntimeProbeFactoryConfig(t, []string{first, second})
	factory := mustPiLocalRuntimeProbeFactory(t, config)
	probe, found, err := factory.BuildProbe(context.Background())
	if err != nil || !found || probe == nil {
		t.Fatalf("BuildProbe() = (%#v, %v, %v)", probe, found, err)
	}
	assertPathAbsent(t, firstMarker)
	assertPathAbsent(t, secondMarker)
	assertPathAbsent(t, ambientMarker)
	assertDirectoryEmpty(t, config.IsolationRoot)

	snapshot, err := loomruntime.DiscoverRuntime(context.Background(), []loomruntime.RuntimeProbe{probe})
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	observations := snapshot.Observations()
	if len(observations) != 1 {
		t.Fatalf("observations = %#v", observations)
	}
	got := observations[0]
	if got.SourceProbeID != config.ProbeID ||
		got.Instance.ID != config.InstanceID ||
		got.Instance.DeviceID != config.DeviceID ||
		got.Instance.DisplayName != config.DisplayName ||
		got.Instance.ExecutableVersion != "1.0.0" ||
		!reflect.DeepEqual(got.ModelIDs, []string{"provider/model"}) {
		t.Fatalf("observation = %#v", got)
	}
	if _, err := os.Stat(firstMarker); err != nil {
		t.Fatalf("first candidate was not invoked: %v", err)
	}
	assertPathAbsent(t, secondMarker)
	assertPathAbsent(t, ambientMarker)
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiLocalRuntimeProbeFactoryIgnoresAliasesNestedAndAmbientPath(t *testing.T) {
	ambient := privateTempDir(t)
	writeLocalPiFixture(t, ambient, "9.9.9", filepath.Join(t.TempDir(), "ambient-marker"))
	t.Setenv("PATH", ambient)
	search := privateTempDir(t)
	if err := os.WriteFile(filepath.Join(search, "pi-cli"), []byte("#!/bin/sh\nexit 90\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(search, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	writeLocalPiFixture(t, nested, "8.8.8", filepath.Join(t.TempDir(), "nested-marker"))
	factory := mustPiLocalRuntimeProbeFactory(t, validPiLocalRuntimeProbeFactoryConfig(t, []string{search}))

	probe, found, err := factory.BuildProbe(context.Background())
	if err != nil {
		t.Fatalf("BuildProbe() error = %v", err)
	}
	assertNoPiLocalProbe(t, probe, found)
}

func TestPiLocalRuntimeProbeFactoryInvalidFirstCandidateFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		write func(*testing.T, string)
	}{
		{name: "directory", write: func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "not executable", write: func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "oversized", write: func(t *testing.T, path string) {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o700)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(512*1024*1024 + 1); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "broken symlink", write: func(t *testing.T, path string) {
			if err := os.Symlink(filepath.Join(t.TempDir(), "missing-secret-target"), path); err != nil {
				t.Fatal(err)
			}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first := privateTempDir(t)
			second := privateTempDir(t)
			test.write(t, filepath.Join(first, "pi"))
			laterMarker := filepath.Join(t.TempDir(), "later-marker")
			writeLocalPiFixture(t, second, "2.0.0", laterMarker)
			config := validPiLocalRuntimeProbeFactoryConfig(t, []string{first, second})
			factory := mustPiLocalRuntimeProbeFactory(t, config)

			probe, found, err := factory.BuildProbe(context.Background())
			assertNoPiLocalProbe(t, probe, found)
			if !errors.Is(err, ErrPiLocalRuntimeCandidateInvalid) {
				t.Fatalf("BuildProbe() error = %v, want ErrPiLocalRuntimeCandidateInvalid", err)
			}
			assertPathAbsent(t, laterMarker)
			assertDirectoryEmpty(t, config.IsolationRoot)
		})
	}
}

func TestPiLocalRuntimeProbeFactoryBindingDriftFailsClosed(t *testing.T) {
	t.Run("root permission drift", func(t *testing.T) {
		search := privateTempDir(t)
		writeLocalPiFixture(t, search, "1.0.0", filepath.Join(t.TempDir(), "marker"))
		config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
		factory := mustPiLocalRuntimeProbeFactory(t, config)
		if err := os.Chmod(config.IsolationRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(config.IsolationRoot, 0o700) })
		assertPiLocalProbeBindingChanged(t, factory)
	})

	t.Run("search identity drift", func(t *testing.T) {
		search := privateTempDir(t)
		config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
		factory := mustPiLocalRuntimeProbeFactory(t, config)
		moved := search + "-old"
		if err := os.Rename(search, moved); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(moved) })
		if err := os.Mkdir(search, 0o700); err != nil {
			t.Fatal(err)
		}
		assertPiLocalProbeBindingChanged(t, factory)
	})

	t.Run("search permission drift", func(t *testing.T) {
		search := privateTempDir(t)
		config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
		factory := mustPiLocalRuntimeProbeFactory(t, config)
		if err := os.Chmod(search, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(search, 0o700) })
		assertPiLocalProbeBindingChanged(t, factory)
	})
}

func TestPiLocalRuntimeProbeFactorySymlinkTargetIsBoundAtConstruction(t *testing.T) {
	search := privateTempDir(t)
	targets := privateTempDir(t)
	firstMarker := filepath.Join(t.TempDir(), "first-marker")
	secondMarker := filepath.Join(t.TempDir(), "second-marker")
	first := writeNamedLocalPiFixture(t, targets, "pi-first", "1.0.0", firstMarker)
	second := writeNamedLocalPiFixture(t, targets, "pi-second", "2.0.0", secondMarker)
	link := filepath.Join(search, "pi")
	if err := os.Symlink(first, link); err != nil {
		t.Fatal(err)
	}
	config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
	factory := mustPiLocalRuntimeProbeFactory(t, config)
	probe, found, err := factory.BuildProbe(context.Background())
	if err != nil || !found || probe == nil {
		t.Fatalf("BuildProbe() = (%#v, %v, %v)", probe, found, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, link); err != nil {
		t.Fatal(err)
	}

	snapshot, err := loomruntime.DiscoverRuntime(context.Background(), []loomruntime.RuntimeProbe{probe})
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	if got := snapshot.Observations()[0].Instance.ExecutableVersion; got != "1.0.0" {
		t.Fatalf("version = %q, want bound first target", got)
	}
	if _, err := os.Stat(firstMarker); err != nil {
		t.Fatalf("first target marker: %v", err)
	}
	assertPathAbsent(t, secondMarker)
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiLocalRuntimeProbeFactoryErrorsDoNotDiscloseLocalState(t *testing.T) {
	secret := "secret-local-pi-path-marker"
	search := privateTempDir(t)
	if err := os.Symlink(filepath.Join(t.TempDir(), secret), filepath.Join(search, "pi")); err != nil {
		t.Fatal(err)
	}
	config := validPiLocalRuntimeProbeFactoryConfig(t, []string{search})
	factory := mustPiLocalRuntimeProbeFactory(t, config)
	probe, found, err := factory.BuildProbe(context.Background())
	assertNoPiLocalProbe(t, probe, found)
	if !errors.Is(err, ErrPiLocalRuntimeCandidateInvalid) {
		t.Fatalf("BuildProbe() error = %v", err)
	}
	for _, forbidden := range []string{secret, search, config.IsolationRoot, "secret-parent-value"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error disclosed %q: %v", forbidden, err)
		}
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		t.Fatalf("error wrapped raw *os.PathError: %v", err)
	}
}

func TestPiLocalRuntimeProbeFactoryProductionBoundary(t *testing.T) {
	path := filepath.Join(".", "local_probe.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenImports := map[string]bool{
		"database/sql":     true,
		"net":              true,
		"net/http":         true,
		"os/exec":          true,
		"path/filepath":    false,
		"internal/journal": true,
	}
	for _, spec := range file.Imports {
		pathValue := strings.Trim(spec.Path.Value, `"`)
		if forbiddenImports[pathValue] {
			t.Fatalf("local_probe.go imports forbidden package %q", pathValue)
		}
	}
	for _, forbidden := range []string{
		"LookPath(",
		"filepath.Walk",
		"WalkDir(",
		"filepath.Glob",
		"os.Getenv(",
		"os.Environ(",
		"exec.Command",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("local_probe.go contains forbidden concrete behavior %q", forbidden)
		}
	}
}

func validPiLocalRuntimeProbeFactoryConfig(t *testing.T, searchPaths []string) PiLocalRuntimeProbeFactoryConfig {
	t.Helper()
	return PiLocalRuntimeProbeFactoryConfig{
		ProbeID:            "probe.pi.local",
		InstanceID:         "runtime.pi.local",
		DeviceID:           "device.local",
		DisplayName:        "Local Pi",
		IsolationRoot:      privateTempDir(t),
		RuntimeSearchPaths: append([]string(nil), searchPaths...),
		Timeout:            10 * time.Second,
	}
}

func clonePiLocalRuntimeProbeFactoryConfig(input PiLocalRuntimeProbeFactoryConfig) PiLocalRuntimeProbeFactoryConfig {
	clone := input
	clone.RuntimeSearchPaths = append([]string(nil), input.RuntimeSearchPaths...)
	return clone
}

func mustPiLocalRuntimeProbeFactory(
	t *testing.T,
	config PiLocalRuntimeProbeFactoryConfig,
) *PiLocalRuntimeProbeFactory {
	t.Helper()
	factory, err := NewPiLocalRuntimeProbeFactory(config)
	if err != nil {
		t.Fatalf("NewPiLocalRuntimeProbeFactory() error = %v", err)
	}
	return factory
}

func assertNoPiLocalProbe(t *testing.T, probe loomruntime.RuntimeProbe, found bool) {
	t.Helper()
	if probe != nil || found {
		t.Fatalf("probe/found = (%#v, %v), want (nil, false)", probe, found)
	}
}

func assertPiLocalProbeBindingChanged(t *testing.T, factory *PiLocalRuntimeProbeFactory) {
	t.Helper()
	probe, found, err := factory.BuildProbe(context.Background())
	assertNoPiLocalProbe(t, probe, found)
	if !errors.Is(err, ErrPiLocalRuntimeProbeBindingChanged) {
		t.Fatalf("BuildProbe() error = %v, want ErrPiLocalRuntimeProbeBindingChanged", err)
	}
}

func writeLocalPiFixture(t *testing.T, directory, version, marker string) string {
	t.Helper()
	return writeNamedLocalPiFixture(t, directory, "pi", version, marker)
}

func writeNamedLocalPiFixture(t *testing.T, directory, name, version, marker string) string {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
printf invoked > %s
if [ "$#" -eq 1 ] && [ "$1" = "--version" ]; then
	printf '%%s\n' %s
	exit 0
fi
if [ "$#" -eq 8 ] && [ "$8" = "--list-models" ]; then
	printf 'provider model context max-out thinking images\n'
	printf 'provider model 200K 32K no no\n'
	exit 0
fi
exit 83
`, shellQuote(marker), shellQuote(version))
	return writeExecutableScript(t, directory, name, script)
}

func assertPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("path %q exists or stat failed: %v", path, err)
	}
}
