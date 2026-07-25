package piadapter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"sort"
	"strings"
	"testing"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

const piMetadataFixturePrefix = "pi-metadata-fixture"

func TestPiMetadataProcessRunnerConstructorValidation(t *testing.T) {
	fixture := makePiMetadataFixture(t, "success")
	valid := validPiMetadataProcessRunnerConfig(t, fixture)

	nonExecutable := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(nonExecutable, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	largeExecutable := filepath.Join(t.TempDir(), "too-large")
	file, err := os.OpenFile(largeExecutable, os.O_CREATE|os.O_WRONLY, 0o700)
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
	rootFile := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(rootFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	missingRoot := filepath.Join(t.TempDir(), "missing-root")
	rootTarget := privateTempDir(t)
	rootSymlink := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(rootTarget, rootSymlink); err != nil {
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
		change func(*PiMetadataProcessRunnerConfig)
	}{
		{name: "empty executable", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath = "" }},
		{name: "relative executable", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath = "pi" }},
		{name: "unclean executable", change: func(c *PiMetadataProcessRunnerConfig) {
			c.ExecutablePath = filepath.Dir(fixture) + string(os.PathSeparator) + "." + string(os.PathSeparator) + filepath.Base(fixture)
		}},
		{name: "nul executable", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath += "\x00" }},
		{name: "missing executable", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath = filepath.Join(t.TempDir(), "missing") }},
		{name: "executable directory", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath = t.TempDir() }},
		{name: "non executable", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath = nonExecutable }},
		{name: "executable too large", change: func(c *PiMetadataProcessRunnerConfig) { c.ExecutablePath = largeExecutable }},
		{name: "empty root", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot = "" }},
		{name: "relative root", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot = "state" }},
		{name: "unclean root", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot += string(os.PathSeparator) + "." }},
		{name: "nul root", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot += "\x00" }},
		{name: "missing root", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot = missingRoot }},
		{name: "root file", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot = rootFile }},
		{name: "root symlink", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot = rootSymlink }},
		{name: "root not private", change: func(c *PiMetadataProcessRunnerConfig) { c.IsolationRoot = publicRoot }},
		{name: "empty search paths", change: func(c *PiMetadataProcessRunnerConfig) { c.RuntimeSearchPaths = nil }},
		{name: "relative search path", change: func(c *PiMetadataProcessRunnerConfig) { c.RuntimeSearchPaths = []string{"bin"} }},
		{name: "unclean search path", change: func(c *PiMetadataProcessRunnerConfig) {
			c.RuntimeSearchPaths = []string{valid.RuntimeSearchPaths[0] + string(os.PathSeparator) + "."}
		}},
		{name: "nul search path", change: func(c *PiMetadataProcessRunnerConfig) {
			c.RuntimeSearchPaths = []string{valid.RuntimeSearchPaths[0] + "\x00"}
		}},
		{name: "missing search path", change: func(c *PiMetadataProcessRunnerConfig) {
			c.RuntimeSearchPaths = []string{filepath.Join(t.TempDir(), "missing")}
		}},
		{name: "search path file", change: func(c *PiMetadataProcessRunnerConfig) { c.RuntimeSearchPaths = []string{searchFile} }},
		{name: "zero timeout", change: func(c *PiMetadataProcessRunnerConfig) { c.Timeout = 0 }},
		{name: "negative timeout", change: func(c *PiMetadataProcessRunnerConfig) { c.Timeout = -time.Second }},
		{name: "timeout too large", change: func(c *PiMetadataProcessRunnerConfig) { c.Timeout = 30*time.Second + time.Nanosecond }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := clonePiMetadataProcessRunnerConfig(valid)
			test.change(&config)
			runner, err := NewPiMetadataProcessRunner(config)
			if !errors.Is(err, ErrInvalidPiMetadataProcessRunner) {
				t.Fatalf("NewPiMetadataProcessRunner() error = %v, want ErrInvalidPiMetadataProcessRunner", err)
			}
			if runner != nil {
				t.Fatalf("NewPiMetadataProcessRunner() runner = %#v, want nil", runner)
			}
		})
	}
}

func TestPiMetadataProcessRunnerExactRequestPrevalidation(t *testing.T) {
	fixture := makePiMetadataFixture(t, "success")
	config := validPiMetadataProcessRunnerConfig(t, fixture)
	runner := mustPiMetadataProcessRunner(t, config)
	root := config.IsolationRoot

	tests := []struct {
		name    string
		ctx     context.Context
		request loomruntime.PiMetadataRequest
	}{
		{name: "nil context", ctx: nil, request: piVersionRequest()},
		{name: "unknown command", ctx: context.Background(), request: loomruntime.PiMetadataRequest{Command: "unknown", Args: []string{"--version"}}},
		{name: "version missing args", ctx: context.Background(), request: loomruntime.PiMetadataRequest{Command: loomruntime.PiMetadataVersion}},
		{name: "version extra arg", ctx: context.Background(), request: loomruntime.PiMetadataRequest{Command: loomruntime.PiMetadataVersion, Args: []string{"--version", "--extra"}}},
		{name: "models missing arg", ctx: context.Background(), request: loomruntime.PiMetadataRequest{Command: loomruntime.PiMetadataListModels, Args: piModelRequest().Args[:7]}},
		{name: "models reordered", ctx: context.Background(), request: loomruntime.PiMetadataRequest{Command: loomruntime.PiMetadataListModels, Args: []string{
			"--no-approve", "--offline", "--no-extensions", "--no-skills",
			"--no-prompt-templates", "--no-themes", "--no-context-files", "--list-models",
		}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := runner.RunPiMetadata(test.ctx, test.request)
			if !errors.Is(err, ErrInvalidPiMetadataRequest) {
				t.Fatalf("RunPiMetadata() error = %v, want ErrInvalidPiMetadataRequest", err)
			}
			if result != (loomruntime.PiMetadataResult{}) {
				t.Fatalf("RunPiMetadata() result = %#v, want zero", result)
			}
			assertDirectoryEmpty(t, root)
		})
	}
}

func TestPiMetadataProcessRunnerSuccessIsolationAndCleanup(t *testing.T) {
	t.Setenv("LOOM_TEST_PARENT_SECRET", "secret-parent-value")
	fixture := makePiMetadataFixture(t, "success")
	searchRoot := privateTempDir(t)
	searchAlias := filepath.Join(t.TempDir(), "search-alias")
	if err := os.Symlink(searchRoot, searchAlias); err != nil {
		t.Fatal(err)
	}
	config := validPiMetadataProcessRunnerConfig(t, fixture)
	config.RuntimeSearchPaths = []string{searchAlias, searchRoot, "/bin"}
	writeFixtureControl(t, fixture, "expected-path", canonicalSearchPath(t, config.RuntimeSearchPaths))
	runner := mustPiMetadataProcessRunner(t, config)
	config.RuntimeSearchPaths[0] = "/mutated"

	version, err := runner.RunPiMetadata(context.Background(), piVersionRequest())
	if err != nil {
		t.Fatalf("version RunPiMetadata() error = %v; fixture diagnostic = %s", err, readFixtureDiagnostic(fixture))
	}
	if version != (loomruntime.PiMetadataResult{Stdout: "0.73.1\n"}) {
		t.Fatalf("version result = %#v", version)
	}
	assertDirectoryEmpty(t, config.IsolationRoot)

	models, err := runner.RunPiMetadata(context.Background(), piModelRequest())
	if err != nil {
		t.Fatalf("models RunPiMetadata() error = %v", err)
	}
	wantModels := "provider model context max-out thinking images\nprovider model 200K 32K no no\n"
	if models != (loomruntime.PiMetadataResult{Stdout: wantModels}) {
		t.Fatalf("models result = %#v, want %#v", models, loomruntime.PiMetadataResult{Stdout: wantModels})
	}
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiMetadataProcessRunnerBindingAndSymlinkBehavior(t *testing.T) {
	t.Run("executable symlink retarget cannot redirect", func(t *testing.T) {
		dir := t.TempDir()
		first := writeExecutableScript(t, dir, "first", "#!/bin/sh\nprintf '1.0.0\\n'\n")
		second := writeExecutableScript(t, dir, "second", "#!/bin/sh\nprintf '2.0.0\\n'\n")
		link := filepath.Join(dir, "pi")
		if err := os.Symlink(first, link); err != nil {
			t.Fatal(err)
		}
		config := validPiMetadataProcessRunnerConfig(t, link)
		runner := mustPiMetadataProcessRunner(t, config)
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(second, link); err != nil {
			t.Fatal(err)
		}
		result, err := runner.RunPiMetadata(context.Background(), piVersionRequest())
		if err != nil {
			t.Fatalf("RunPiMetadata() error = %v", err)
		}
		if result.Stdout != "1.0.0\n" {
			t.Fatalf("stdout = %q, want original target", result.Stdout)
		}
	})

	t.Run("executable digest drift", func(t *testing.T) {
		dir := t.TempDir()
		script := writeExecutableScript(t, dir, "pi", "#!/bin/sh\nprintf '1.0.0\\n'\n")
		config := validPiMetadataProcessRunnerConfig(t, script)
		runner := mustPiMetadataProcessRunner(t, config)
		if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '2.0.0\\n'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		assertPiBindingChanged(t, runner, piVersionRequest())
	})

	t.Run("root drift", func(t *testing.T) {
		fixture := makePiMetadataFixture(t, "success")
		config := validPiMetadataProcessRunnerConfig(t, fixture)
		runner := mustPiMetadataProcessRunner(t, config)
		if err := os.Chmod(config.IsolationRoot, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(config.IsolationRoot, 0o700) })
		assertPiBindingChanged(t, runner, piVersionRequest())
	})

	t.Run("root identity replacement", func(t *testing.T) {
		fixture := makePiMetadataFixture(t, "success")
		config := validPiMetadataProcessRunnerConfig(t, fixture)
		runner := mustPiMetadataProcessRunner(t, config)
		moved := config.IsolationRoot + "-old"
		if err := os.Rename(config.IsolationRoot, moved); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(moved) })
		if err := os.Mkdir(config.IsolationRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		assertPiBindingChanged(t, runner, piVersionRequest())
	})

	t.Run("search directory identity drift", func(t *testing.T) {
		fixture := makePiMetadataFixture(t, "success")
		search := privateTempDir(t)
		writeFixtureControl(t, fixture, "expected-path", search)
		config := validPiMetadataProcessRunnerConfig(t, fixture)
		config.RuntimeSearchPaths = []string{search}
		runner := mustPiMetadataProcessRunner(t, config)
		moved := search + "-old"
		if err := os.Rename(search, moved); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(search, 0o700); err != nil {
			t.Fatal(err)
		}
		assertPiBindingChanged(t, runner, piVersionRequest())
	})

	t.Run("interpreter bytes inside stable directory are residual", func(t *testing.T) {
		fixture := makePiMetadataFixture(t, "success")
		search := privateTempDir(t)
		interpreter := filepath.Join(search, "node")
		if err := os.WriteFile(interpreter, []byte("first"), 0o700); err != nil {
			t.Fatal(err)
		}
		config := validPiMetadataProcessRunnerConfig(t, fixture)
		config.RuntimeSearchPaths = []string{search}
		writeFixtureControl(t, fixture, "expected-path", canonicalSearchPath(t, config.RuntimeSearchPaths))
		runner := mustPiMetadataProcessRunner(t, config)
		if err := os.WriteFile(interpreter, []byte("second"), 0o700); err != nil {
			t.Fatal(err)
		}
		result, err := runner.RunPiMetadata(context.Background(), piVersionRequest())
		if err != nil {
			t.Fatalf("RunPiMetadata() error = %v; interpreter contents are explicitly outside binding; fixture diagnostic = %s", err, readFixtureDiagnostic(fixture))
		}
		if result.Stdout != "0.73.1\n" {
			t.Fatalf("stdout = %q", result.Stdout)
		}
	})
}

func TestPiMetadataProcessRunnerProbeAndDiscoveryIntegration(t *testing.T) {
	fixture := makePiMetadataFixture(t, "success")
	config := validPiMetadataProcessRunnerConfig(t, fixture)
	writeFixtureControl(t, fixture, "expected-path", strings.Join(config.RuntimeSearchPaths, string(os.PathListSeparator)))
	runner := mustPiMetadataProcessRunner(t, config)
	probe, err := loomruntime.NewPiRuntimeProbe(loomruntime.PiRuntimeProbeConfig{
		ProbeID:     "probe.pi.process",
		InstanceID:  "runtime.pi.process",
		DeviceID:    "device.local",
		DisplayName: "Isolated Pi",
		Runner:      runner,
	})
	if err != nil {
		t.Fatalf("NewPiRuntimeProbe() error = %v", err)
	}

	snapshot, err := loomruntime.DiscoverRuntime(context.Background(), []loomruntime.RuntimeProbe{probe})
	if err != nil {
		t.Fatalf("DiscoverRuntime() error = %v", err)
	}
	observations := snapshot.Observations()
	if len(observations) != 1 {
		t.Fatalf("observations = %#v", observations)
	}
	got := observations[0]
	if got.SourceProbeID != "probe.pi.process" ||
		got.Instance.ExecutableVersion != "0.73.1" ||
		got.Instance.AdapterType != "pi-cli" ||
		!reflect.DeepEqual(got.ModelIDs, []string{"provider/model"}) {
		t.Fatalf("observation = %#v", got)
	}
	assertDirectoryEmpty(t, config.IsolationRoot)
}

func TestPiMetadataProcessRunnerFailuresAreTypedBoundedAndNonDisclosing(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want error
	}{
		{name: "nonzero exit", mode: "fail", want: ErrPiMetadataProcessFailed},
		{name: "stdout overflow", mode: "stdout-overflow", want: ErrPiMetadataProcessOutputTooLarge},
		{name: "stderr overflow", mode: "stderr-overflow", want: ErrPiMetadataProcessOutputTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := makePiMetadataFixture(t, test.mode)
			config := validPiMetadataProcessRunnerConfig(t, fixture)
			runner := mustPiMetadataProcessRunner(t, config)
			result, err := runner.RunPiMetadata(context.Background(), piVersionRequest())
			if !errors.Is(err, test.want) {
				t.Fatalf("RunPiMetadata() error = %v, want %v", err, test.want)
			}
			if result != (loomruntime.PiMetadataResult{}) {
				t.Fatalf("result = %#v, want zero", result)
			}
			for _, secret := range []string{"secret-child-stderr", fixture, config.IsolationRoot, "--version", "secret-parent-value"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaked %q: %v", secret, err)
				}
			}
			assertDirectoryEmpty(t, config.IsolationRoot)
		})
	}
}

func TestPiMetadataProcessRunnerCancellationTimeoutAndProcessGroupCleanup(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("Unix original-process-group proof")
	}

	t.Run("internal timeout kills same-group child", func(t *testing.T) {
		fixture := makePiMetadataFixture(t, "hang")
		marker := filepath.Join(t.TempDir(), "late-marker")
		writeFixtureControl(t, fixture, "marker", marker)
		config := validPiMetadataProcessRunnerConfig(t, fixture)
		config.Timeout = 100 * time.Millisecond
		runner := mustPiMetadataProcessRunner(t, config)
		result, err := runner.RunPiMetadata(context.Background(), piVersionRequest())
		if !errors.Is(err, ErrPiMetadataProcessTimeout) || result != (loomruntime.PiMetadataResult{}) {
			t.Fatalf("RunPiMetadata() = (%#v, %v), want timeout and zero result", result, err)
		}
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("same-group child marker exists or stat failed: %v", err)
		}
		assertDirectoryEmpty(t, config.IsolationRoot)
	})

	t.Run("caller cancellation remains inspectable", func(t *testing.T) {
		fixture := makePiMetadataFixture(t, "hang")
		marker := filepath.Join(t.TempDir(), "late-marker")
		writeFixtureControl(t, fixture, "marker", marker)
		config := validPiMetadataProcessRunnerConfig(t, fixture)
		runner := mustPiMetadataProcessRunner(t, config)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			waitForFixtureReady(filepath.Join(filepath.Dir(fixture), "ready"))
			cancel()
		}()
		result, err := runner.RunPiMetadata(ctx, piVersionRequest())
		if !errors.Is(err, context.Canceled) || result != (loomruntime.PiMetadataResult{}) {
			t.Fatalf("RunPiMetadata() = (%#v, %v), want context.Canceled and zero result", result, err)
		}
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("same-group child marker exists or stat failed: %v", err)
		}
		assertDirectoryEmpty(t, config.IsolationRoot)
	})
}

func TestPiMetadataProcessRunnerCleanupFailureAndCombinedErrors(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("permission-denied cleanup fixture")
	}
	tests := []struct {
		name       string
		mode       string
		cancel     bool
		wantSecond error
	}{
		{name: "cleanup only", mode: "cleanup-success"},
		{name: "process and cleanup", mode: "cleanup-fail", wantSecond: ErrPiMetadataProcessFailed},
		{name: "cancellation and cleanup", mode: "cleanup-hang", cancel: true, wantSecond: context.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := makePiMetadataFixture(t, test.mode)
			config := validPiMetadataProcessRunnerConfig(t, fixture)
			runner := mustPiMetadataProcessRunner(t, config)
			ctx := context.Background()
			var cancel context.CancelFunc
			if test.cancel {
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					waitForFixtureReady(filepath.Join(filepath.Dir(fixture), "ready"))
					cancel()
				}()
			}
			result, err := runner.RunPiMetadata(ctx, piVersionRequest())
			if result != (loomruntime.PiMetadataResult{}) {
				t.Fatalf("result = %#v, want zero", result)
			}
			if !errors.Is(err, ErrPiMetadataIsolationCleanup) {
				t.Fatalf("error = %v, want cleanup sentinel", err)
			}
			if test.wantSecond != nil && !errors.Is(err, test.wantSecond) {
				t.Fatalf("error = %v, want joined %v", err, test.wantSecond)
			}
			if test.wantSecond == nil && errors.Is(err, ErrPiMetadataProcessFailed) {
				t.Fatalf("cleanup-only error unexpectedly contains process failure: %v", err)
			}
			if strings.Contains(err.Error(), config.IsolationRoot) {
				t.Fatalf("cleanup error leaked root: %v", err)
			}
			if strings.Contains(err.Error(), "secret-cleanup-process-error") {
				t.Fatalf("cleanup error leaked child stderr: %v", err)
			}
			if err := os.Chmod(config.IsolationRoot, 0o700); err != nil {
				t.Fatalf("restore root mode: %v", err)
			}
			entries, readErr := os.ReadDir(config.IsolationRoot)
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), "loom-pi-metadata-") {
					t.Fatalf("unexpected cleanup residue %q", entry.Name())
				}
				if err := os.RemoveAll(filepath.Join(config.IsolationRoot, entry.Name())); err != nil {
					t.Fatalf("remove exact fixture residue: %v", err)
				}
			}
			assertDirectoryEmpty(t, config.IsolationRoot)
		})
	}
}

func validPiMetadataProcessRunnerConfig(t *testing.T, executable string) PiMetadataProcessRunnerConfig {
	t.Helper()
	search := []string{"/usr/bin", "/bin"}
	writeFixtureControl(t, executable, "expected-path", canonicalSearchPath(t, search))
	return PiMetadataProcessRunnerConfig{
		ExecutablePath:     executable,
		IsolationRoot:      privateTempDir(t),
		RuntimeSearchPaths: search,
		Timeout:            3 * time.Second,
	}
}

func clonePiMetadataProcessRunnerConfig(input PiMetadataProcessRunnerConfig) PiMetadataProcessRunnerConfig {
	clone := input
	clone.RuntimeSearchPaths = append([]string(nil), input.RuntimeSearchPaths...)
	return clone
}

func mustPiMetadataProcessRunner(t *testing.T, config PiMetadataProcessRunnerConfig) loomruntime.PiMetadataRunner {
	t.Helper()
	runner, err := NewPiMetadataProcessRunner(config)
	if err != nil {
		t.Fatalf("NewPiMetadataProcessRunner() error = %v", err)
	}
	return runner
}

func piVersionRequest() loomruntime.PiMetadataRequest {
	return loomruntime.PiMetadataRequest{Command: loomruntime.PiMetadataVersion, Args: []string{"--version"}}
}

func piModelRequest() loomruntime.PiMetadataRequest {
	return loomruntime.PiMetadataRequest{Command: loomruntime.PiMetadataListModels, Args: []string{
		"--offline",
		"--no-approve",
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-themes",
		"--no-context-files",
		"--list-models",
	}}
}

func makePiMetadataFixture(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	executable := filepath.Join(dir, piMetadataFixturePrefix)
	script := fmt.Sprintf(`#!/bin/sh
set -u
fixture_dir=%s
mode=%s

validate_environment() {
	expected_path=$(/bin/cat "$fixture_dir/expected-path")
	invocation_dir=$(/usr/bin/dirname "$HOME")
	[ "${LOOM_TEST_PARENT_SECRET+x}" != x ] || return 1
	[ "$PATH" = "$expected_path" ] || return 1
	[ "$PWD" = "$HOME" ] || return 1
	[ "$TMPDIR" = "$invocation_dir/tmp" ] || return 1
	[ "$PI_CODING_AGENT_DIR" = "$invocation_dir/agent" ] || return 1
	[ "$PI_CODING_AGENT_SESSION_DIR" = "$invocation_dir/sessions" ] || return 1
	[ "$LANG" = C ] || return 1
	[ "$LC_ALL" = C ] || return 1
	[ "$NO_COLOR" = 1 ] || return 1
	[ "$TERM" = dumb ] || return 1
	[ "$PI_OFFLINE" = 1 ] || return 1
	[ "$PI_SKIP_VERSION_CHECK" = 1 ] || return 1
	[ "$PI_TELEMETRY" = 0 ] || return 1
	for path in "$HOME" "$TMPDIR" "$PI_CODING_AGENT_DIR" "$PI_CODING_AGENT_SESSION_DIR"; do
		[ -d "$path" ] || return 1
		mode_value=$(/usr/bin/stat -f '%%Lp' "$path" 2>/dev/null || /usr/bin/stat -c '%%a' "$path" 2>/dev/null)
		[ "$mode_value" = 700 ] || return 1
	done
	/usr/bin/env | while IFS='=' read -r key value; do
		case "$key" in
			HOME|TMPDIR|PI_CODING_AGENT_DIR|PI_CODING_AGENT_SESSION_DIR|PATH|LANG|LC_ALL|NO_COLOR|TERM|PI_OFFLINE|PI_SKIP_VERSION_CHECK|PI_TELEMETRY|PWD|SHLVL|_) ;;
			*) exit 1 ;;
		esac
	done
}

emit_metadata() {
	if [ "$#" -eq 1 ] && [ "$1" = "--version" ]; then
		printf '0.73.1\n'
		return 0
	fi
	if [ "$#" -eq 8 ] &&
		[ "$1" = "--offline" ] &&
		[ "$2" = "--no-approve" ] &&
		[ "$3" = "--no-extensions" ] &&
		[ "$4" = "--no-skills" ] &&
		[ "$5" = "--no-prompt-templates" ] &&
		[ "$6" = "--no-themes" ] &&
		[ "$7" = "--no-context-files" ] &&
		[ "$8" = "--list-models" ]; then
		printf 'provider model context max-out thinking images\nprovider model 200K 32K no no\n'
		return 0
	fi
	return 83
}

deny_cleanup() {
	invocation_dir=$(/usr/bin/dirname "$HOME")
	root=$(/usr/bin/dirname "$invocation_dir")
	/bin/chmod 500 "$root"
}

hang_with_child() {
	marker=$(/bin/cat "$fixture_dir/marker" 2>/dev/null || printf '%%s' "$fixture_dir/late-marker")
	( /bin/sleep 0.35; /usr/bin/touch "$marker" ) &
	printf ready > "$fixture_dir/ready"
	/bin/sleep 30
}

case "$mode" in
	success)
		validate_environment || {
			{ printf 'cwd='; /bin/pwd; printf 'expected_path='; /bin/cat "$fixture_dir/expected-path"; printf '\nenvironment:\n'; /usr/bin/env | /usr/bin/sort; } > "$fixture_dir/diagnostic"
			printf 'secret-environment-boundary-failed' >&2
			exit 81
		}
		emit_metadata "$@"
		;;
	fail)
		printf 'secret-child-stdout'
		printf 'secret-child-stderr' >&2
		exit 23
		;;
	stdout-overflow)
		/usr/bin/head -c 262145 /dev/zero
		;;
	stderr-overflow)
		/usr/bin/head -c 262145 /dev/zero >&2
		;;
	hang)
		hang_with_child
		;;
	cleanup-success)
		deny_cleanup
		printf '0.73.1\n'
		;;
	cleanup-fail)
		deny_cleanup
		printf 'secret-cleanup-process-error' >&2
		exit 23
		;;
	cleanup-hang)
		deny_cleanup
		hang_with_child
		;;
	*)
		exit 80
		;;
esac
`, shellQuote(dir), shellQuote(mode))
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFixtureControl(t, executable, "mode", mode)
	return executable
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func readFixtureDiagnostic(executable string) string {
	value, err := os.ReadFile(filepath.Join(filepath.Dir(executable), "diagnostic"))
	if err != nil {
		return err.Error()
	}
	return string(value)
}

func writeFixtureControl(t *testing.T, executable, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(filepath.Dir(executable), name), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForFixtureReady(path string) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func writeExecutableScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func privateTempDir(t *testing.T) string {
	t.Helper()
	path := t.TempDir()
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertPiBindingChanged(t *testing.T, runner loomruntime.PiMetadataRunner, request loomruntime.PiMetadataRequest) {
	t.Helper()
	result, err := runner.RunPiMetadata(context.Background(), request)
	if !errors.Is(err, ErrPiMetadataBindingChanged) {
		t.Fatalf("RunPiMetadata() error = %v, want ErrPiMetadataBindingChanged", err)
	}
	if result != (loomruntime.PiMetadataResult{}) {
		t.Fatalf("result = %#v, want zero", result)
	}
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		t.Fatalf("directory %q not empty: %v", path, names)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func canonicalSearchPath(t *testing.T, paths []string) string {
	t.Helper()
	canonical := make([]string, 0, len(paths))
	for _, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		if !containsString(canonical, resolved) {
			canonical = append(canonical, resolved)
		}
	}
	if len(canonical) == 0 {
		t.Fatal("no deterministic runtime search directory")
	}
	return strings.Join(canonical, string(os.PathListSeparator))
}
