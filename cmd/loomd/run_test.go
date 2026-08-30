package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

func TestMissionExecutionConfigFromDaemonBuildPreservesExactRuntimeBinding(
	t *testing.T,
) {
	// The daemon build may be running under an ambient opt-in (launchctl
	// LOOM_ENABLE_WEB_TOOLS=1) that fabricates a remote-tool broker even for an
	// empty build config. Pin it off so the assertions below test the build
	// mapping itself, not the surrounding environment.
	t.Setenv("LOOM_ENABLE_WEB_TOOLS", "")
	searchPaths := []string{"/opt/pi-a", "/opt/pi-b"}
	catalog := &piadapter.PiLocalModelCatalogConfig{
		PrivateRoot:    "/private/model",
		ExecutablePath: "/private/model/llama-server",
		ModelPath:      "/private/model/model.gguf",
	}
	config := missionExecutionConfigFromDaemonBuild(daemonBuildConfig{
		CodexExecutable:  "/opt/codex/bin/codex",
		ClaudeExecutable: "/opt/claude/bin/claude",
		Observer: app.LocalRuntimeObservationDaemonConfig{
			RuntimeSearchPaths: searchPaths,
			RuntimeInstanceID:  "runtime-1",
			LocalModelCatalog:  catalog,
		},
	})
	if config == nil || config.RuntimeInstanceID != "runtime-1" ||
		config.CodexExecutable != "/opt/codex/bin/codex" ||
		config.ClaudeExecutable != "/opt/claude/bin/claude" ||
		!reflect.DeepEqual(config.RuntimeSearchPaths, searchPaths) ||
		config.LocalModelCatalog == nil ||
		*config.LocalModelCatalog != *catalog {
		t.Fatalf("execution config = %#v", config)
	}
	searchPaths[0] = "/changed"
	catalog.ModelPath = "/changed"
	if config.RuntimeSearchPaths[0] != "/opt/pi-a" ||
		config.LocalModelCatalog.ModelPath != "/private/model/model.gguf" {
		t.Fatalf("execution config aliases daemon input = %#v", config)
	}
	if config.RemoteToolBroker == nil || !config.RemoteToolBroker.EnrollmentOnly {
		t.Fatalf("runtime config must retain an enrollment-only remote transport: %#v", config.RemoteToolBroker)
	}
	if empty := missionExecutionConfigFromDaemonBuild(daemonBuildConfig{}); empty != nil {
		t.Fatalf("empty execution config = %#v", empty)
	}
}

func TestRunParsesExplicitConfigurationAndWritesDeterministicJSON(t *testing.T) {
	var captured app.LocalRuntimeObservationDaemonConfig
	runner := &fakeDaemonRunner{
		result: app.LocalRuntimeObservationDaemonResult{
			CompletedCycles: 1,
			DiscoveryEvents: 1,
			RuntimeFacts: []app.LocalRuntimeObservationFact{{
				RuntimeInstanceID: "runtime-1",
				ExecutableVersion: "1.0.0",
				Status:            "online",
				ModelIDs:          []string{"provider/model"},
				DiscoverySequence: 1,
			}},
		},
	}
	builder := func(config daemonBuildConfig) (daemonRunner, error) {
		captured = config.Observer
		return runner, nil
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"--state", "/tmp/loom-state/loom.db",
		"--isolation-root", "/tmp/loom-state/isolation",
		"--runtime-dir", "/opt/pi-a",
		"--runtime-dir", "/opt/pi-b",
		"--probe-id", "pi-local",
		"--instance-id", "runtime-1",
		"--device-id", "device-1",
		"--display-name", "Local Pi",
		"--interval", "1s",
		"--process-timeout", "2s",
		"--max-cycles", "1",
	}, &stdout, &stderr, builder)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	if !reflect.DeepEqual(captured.RuntimeSearchPaths, []string{"/opt/pi-a", "/opt/pi-b"}) ||
		captured.MaxCycles != 1 {
		t.Fatalf("captured config: %#v", captured)
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("decode stdout: %v output=%q", err, stdout.String())
	}
	if decoded["completed_cycles"] != float64(1) ||
		decoded["discovery_events"] != float64(1) {
		t.Fatalf("unexpected output: %#v", decoded)
	}
	if runner.runCalls != 1 || runner.closeCalls != 1 {
		t.Fatalf("runner calls run=%d close=%d", runner.runCalls, runner.closeCalls)
	}
}

func TestRunReportsMCPStartupFailureWithoutTakingDaemonOffline(t *testing.T) {
	const secret = "private-mcp-config-content"
	root := productMCPStdioTestRoot(t)
	productMCPStdioWriteTestConfig(t, root, `{"version":1,"secret":"`+secret+`"`)
	runner := &fakeDaemonRunner{result: app.LocalRuntimeObservationDaemonResult{
		CompletedCycles: 1,
	}}
	var stdout, stderr bytes.Buffer
	code := run(
		context.Background(), completeDaemonArgs(), &stdout, &stderr,
		func(daemonBuildConfig) (daemonRunner, error) {
			registry, diagnostic := prepareProductMCPStdioStartup(
				context.Background(), root,
			)
			return newProductMCPStdioDaemonRunnerWithDiagnostics(
				runner, registry, diagnostic,
			), nil
		},
	)
	if code != exitSuccess || runner.runCalls != 1 || runner.closeCalls != 1 {
		t.Fatalf(
			"code=%d run=%d close=%d stdout=%q stderr=%q",
			code, runner.runCalls, runner.closeCalls, stdout.String(), stderr.String(),
		)
	}
	var diagnostic productMCPStdioOperationalDiagnostic
	if err := json.Unmarshal(bytes.TrimSpace(stderr.Bytes()), &diagnostic); err != nil {
		t.Fatalf("decode diagnostic: %v output=%q", err, stderr.String())
	}
	if diagnostic.Stage != "mcp_config" ||
		diagnostic.ErrorCode != "mcp_config_invalid" ||
		diagnostic.IncidentID == "" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	for _, forbidden := range []string{secret, root, `"args"`, `"env"`, `"content"`} {
		if strings.Contains(stderr.String(), forbidden) {
			t.Fatalf("stderr disclosed %q: %q", forbidden, stderr.String())
		}
	}
}

func TestRunAcceptsOptionalPrivateProductSocket(t *testing.T) {
	args := append(completeDaemonArgs(),
		"--socket", "/tmp/loom-private/loomd.sock",
		"--max-cycles", "1",
	)
	var stdout, stderr bytes.Buffer
	code := run(
		context.Background(),
		args,
		&stdout,
		&stderr,
		func(config daemonBuildConfig) (daemonRunner, error) {
			if config.SocketPath != "/tmp/loom-private/loomd.sock" {
				t.Fatalf("socket path = %q", config.SocketPath)
			}
			return &fakeDaemonRunner{
				result: app.LocalRuntimeObservationDaemonResult{
					CompletedCycles: 1,
				},
			}, nil
		},
	)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf(
			"run --socket code=%d stdout=%q stderr=%q",
			code,
			stdout.String(),
			stderr.String(),
		)
	}
}

func TestRunAcceptsLocalModelCatalogOnlyAsCompleteTuple(t *testing.T) {
	args := append(completeDaemonArgs(),
		"--local-model-private-root", "/private/phase1-live",
		"--local-model-runtime-archive", "/private/phase1-live/sources/llama-runtime.tar.gz",
		"--local-model-executable", "/private/phase1-live/bin/llama-server",
		"--local-model-path", "/private/phase1-live/models/model.gguf",
	)
	var captured app.LocalRuntimeObservationDaemonConfig
	var stdout, stderr bytes.Buffer
	code := run(
		context.Background(),
		args,
		&stdout,
		&stderr,
		func(config daemonBuildConfig) (daemonRunner, error) {
			captured = config.Observer
			return &fakeDaemonRunner{}, nil
		},
	)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("complete catalog code=%d stderr=%q", code, stderr.String())
	}
	if captured.LocalModelCatalog == nil ||
		captured.LocalModelCatalog.PrivateRoot != "/private/phase1-live" ||
		captured.LocalModelCatalog.RuntimeArchivePath != "/private/phase1-live/sources/llama-runtime.tar.gz" ||
		captured.LocalModelCatalog.ExecutablePath != "/private/phase1-live/bin/llama-server" ||
		captured.LocalModelCatalog.ModelPath != "/private/phase1-live/models/model.gguf" {
		t.Fatalf("captured local model catalog = %#v", captured.LocalModelCatalog)
	}

	for _, partial := range [][]string{
		{"--local-model-private-root", "/private/phase1-live"},
		{"--local-model-runtime-archive", "/private/phase1-live/sources/llama-runtime.tar.gz"},
		{"--local-model-executable", "/private/phase1-live/bin/llama-server"},
		{"--local-model-path", "/private/phase1-live/models/model.gguf"},
		{
			"--local-model-private-root", "/private/phase1-live",
			"--local-model-runtime-archive", "/private/phase1-live/sources/llama-runtime.tar.gz",
			"--local-model-executable", "/private/phase1-live/bin/llama-server",
		},
		{
			"--local-model-private-root", "/private/phase1-live",
			"--local-model-runtime-archive", "/private/phase1-live/sources/llama-runtime.tar.gz",
			"--local-model-executable", "/private/phase1-live/bin/llama-server",
			"--local-model-path", "",
		},
	} {
		stdout.Reset()
		stderr.Reset()
		code = run(
			context.Background(),
			append(completeDaemonArgs(), partial...),
			&stdout,
			&stderr,
			func(daemonBuildConfig) (daemonRunner, error) {
				t.Fatal("builder called for partial local model tuple")
				return nil, nil
			},
		)
		if code != exitInvalidInput || stderr.String() != "invalid input\n" {
			t.Fatalf("partial %v code=%d stderr=%q", partial, code, stderr.String())
		}
	}
}

func TestRunRejectsMissingAndMapsBuilderAndRuntimeFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		builder daemonBuilder
		want    int
	}{
		{
			name: "missing",
			want: exitInvalidInput,
		},
		{
			name: "builder",
			args: completeDaemonArgs(),
			builder: func(daemonBuildConfig) (daemonRunner, error) {
				return nil, errors.New("local path must not escape")
			},
			want: exitUnavailable,
		},
		{
			name: "runtime",
			args: completeDaemonArgs(),
			builder: func(daemonBuildConfig) (daemonRunner, error) {
				return &fakeDaemonRunner{err: errors.New("secret output")}, nil
			},
			want: exitRuntimeFailure,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(
				context.Background(), test.args, &stdout, &stderr, test.builder,
			)
			if code != test.want {
				t.Fatalf("code = %d, want %d", code, test.want)
			}
			if bytes.Contains(stderr.Bytes(), []byte("local path")) ||
				bytes.Contains(stderr.Bytes(), []byte("secret output")) {
				t.Fatalf("stderr disclosed internal error: %q", stderr.String())
			}
		})
	}
}

func TestRunWritesClosedDaemonFailureReasonCodes(t *testing.T) {
	for _, test := range []struct {
		name     string
		runErr   error
		closeErr error
		want     string
	}{
		{
			name: "observer",
			runErr: testDaemonFailure{
				code:   "observer",
				reason: "observer_models_stderr",
				detail: "private observer output",
			},
			want: "daemon failed: observer_models_stderr\n",
		},
		{
			name: "local_ipc",
			runErr: testDaemonFailure{
				code:   "local_ipc",
				detail: "private socket path",
			},
			want: "daemon failed: local_ipc\n",
		},
		{
			name:   "unknown_fails_closed",
			runErr: errors.New("private unknown error"),
			want:   "daemon failed: shutdown\n",
		},
		{
			name:     "close",
			closeErr: errors.New("private close error"),
			want:     "daemon failed: shutdown\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeDaemonRunner{
				err:      test.runErr,
				closeErr: test.closeErr,
			}
			var stdout, stderr bytes.Buffer
			code := run(
				context.Background(),
				completeDaemonArgs(),
				&stdout,
				&stderr,
				func(daemonBuildConfig) (daemonRunner, error) {
					return runner, nil
				},
			)
			if code != exitRuntimeFailure ||
				stderr.String() != test.want ||
				bytes.Contains(stderr.Bytes(), []byte("private")) {
				t.Fatalf(
					"code=%d stdout=%q stderr=%q want=%q",
					code,
					stdout.String(),
					stderr.String(),
					test.want,
				)
			}
		})
	}
}

func TestRunFreezesCompleteObserverFailureReasonAllowlist(t *testing.T) {
	allowed := []string{
		"observer_probe_factory",
		"observer_probe_candidate",
		"observer_probe_construction",
		"observer_metadata_binding",
		"observer_version_process",
		"observer_version_timeout",
		"observer_version_output_limit",
		"observer_version_stderr",
		"observer_version_output",
		"observer_models_process",
		"observer_models_timeout",
		"observer_models_output_limit",
		"observer_models_stderr",
		"observer_models_output",
		"observer_models_duplicate",
		"observer_inventory",
		"observer_projection",
		"observer_plan",
		"observer_identity_metadata",
		"observer_write",
		"observer_unknown",
	}
	for _, reason := range allowed {
		if !validObserverFailureReason(reason) {
			t.Errorf("reason %q rejected", reason)
		}
	}
	for _, reason := range []string{
		"", "observer", "observer_private", "observer_write\nprivate",
	} {
		if validObserverFailureReason(reason) {
			t.Errorf("unsafe reason %q accepted", reason)
		}
	}
}

func TestRunClassifiesResultEncodingWithoutDisclosingWriterError(
	t *testing.T,
) {
	var stderr bytes.Buffer
	code := run(
		context.Background(),
		completeDaemonArgs(),
		failingDaemonWriter{},
		&stderr,
		func(daemonBuildConfig) (daemonRunner, error) {
			return &fakeDaemonRunner{}, nil
		},
	)
	if code != exitRuntimeFailure ||
		stderr.String() != "daemon failed: result\n" ||
		bytes.Contains(stderr.Bytes(), []byte("private")) {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunTreatsSignalCancellationAsGracefulBoundedOutput(t *testing.T) {
	runner := &fakeDaemonRunner{
		result: app.LocalRuntimeObservationDaemonResult{
			CompletedCycles: 2,
			NoWriteCycles:   2,
		},
		err: context.Canceled,
	}
	var stdout, stderr bytes.Buffer
	code := run(
		context.Background(),
		completeDaemonArgs(),
		&stdout,
		&stderr,
		func(daemonBuildConfig) (daemonRunner, error) {
			return runner, nil
		},
	)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"completed_cycles":2`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(`"no_write_cycles":2`)) {
		t.Fatalf("missing cancellation summary: %q", stdout.String())
	}
}

type fakeDaemonRunner struct {
	result     app.LocalRuntimeObservationDaemonResult
	err        error
	closeErr   error
	runCalls   int
	closeCalls int
}

func (r *fakeDaemonRunner) Run(
	context.Context,
) (app.LocalRuntimeObservationDaemonResult, error) {
	r.runCalls++
	return r.result, r.err
}

func (r *fakeDaemonRunner) Close() error {
	r.closeCalls++
	return r.closeErr
}

type testDaemonFailure struct {
	code   string
	reason string
	detail string
}

func (failure testDaemonFailure) Error() string {
	return failure.detail
}

func (failure testDaemonFailure) DaemonFailureCode() string {
	return failure.code
}

func (failure testDaemonFailure) DaemonFailureReason() string {
	return failure.reason
}

type failingDaemonWriter struct{}

func (failingDaemonWriter) Write([]byte) (int, error) {
	return 0, errors.New("private result writer")
}

func completeDaemonArgs() []string {
	return []string{
		"--state", "/tmp/loom-state/loom.db",
		"--isolation-root", "/tmp/loom-state/isolation",
		"--runtime-dir", "/opt/pi",
		"--probe-id", "pi-local",
		"--instance-id", "runtime-1",
		"--device-id", "device-1",
		"--display-name", "Local Pi",
		"--interval", "1s",
		"--process-timeout", "2s",
	}
}

func TestExpandLocalAppServiceArgsUsesStableUserPathsAndInstalledRuntime(t *testing.T) {
	home := t.TempDir()
	runtimeBin := filepath.Join(
		home,
		"Library", "Application Support", "Loom", "runtimes", "pi", "0.82.1",
		"node_modules", ".bin",
	)
	if err := os.MkdirAll(runtimeBin, 0o700); err != nil {
		t.Fatal(err)
	}

	args, err := expandLocalAppServiceArgs(
		[]string{"--local-app-service"},
		home,
		[]string{runtimeBin, "/usr/bin"},
		"/usr/bin/codex",
		"",
	)
	if err != nil {
		t.Fatalf("expand local app service args: %v", err)
	}
	want := []string{
		"--state", filepath.Join(home, "Library/Application Support/Loom/state/loom.db"),
		"--isolation-root", filepath.Join(home, "Library/Application Support/Loom/isolation"),
		"--runtime-dir", runtimeBin,
		"--runtime-dir", "/usr/bin",
		"--probe-id", "probe.pi.local-app",
		"--instance-id", "runtime.pi.earendil-works.0.82.1",
		"--device-id", "device.local",
		"--display-name", "Pi 0.82.1",
		"--interval", "10s",
		"--process-timeout", "30s",
		"--socket", filepath.Join(home, "Library/Application Support/Loom/run/loomd.sock"),
		"--codex-executable", "/usr/bin/codex",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("expanded args = %#v, want %#v", args, want)
	}
}

func TestExpandLocalAppServiceArgsCarriesExistingCredentialImportSource(t *testing.T) {
	home := t.TempDir()
	runtimeBin := filepath.Join(
		home,
		"Library", "Application Support", "Loom", "runtimes", "pi", "0.82.1",
		"node_modules", ".bin",
	)
	if err := os.MkdirAll(runtimeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	importRoot := filepath.Join(home, ".cc-switch")
	if err := os.Mkdir(importRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	importPath := filepath.Join(importRoot, "cc-switch.db")
	if err := os.WriteFile(importPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	args, err := expandLocalAppServiceArgs(
		[]string{localAppServiceFlag}, home, []string{runtimeBin}, "", "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(args, "--credential-import-source") ||
		!slices.Contains(args, importPath) {
		t.Fatalf("credential import source absent from canonical args: %#v", args)
	}
}

func TestAppendLocalAppPiModelArgsUsesOnlyInspectedLockedCatalog(t *testing.T) {
	home := t.TempDir()
	base := []string{"--state", "/private/state.sqlite"}
	wantRoot := filepath.Join(
		home, "Library", "Application Support", "Loom", "phase1-live",
	)
	wantExecutable := filepath.Join(
		wantRoot, "runtime", "llama-b10107", "llama-server",
	)
	wantArchive := filepath.Join(
		wantRoot, "sources", "llama-b10107-bin-macos-arm64.tar.gz",
	)
	wantModel := filepath.Join(
		wantRoot, "models", "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
	)
	inspections := 0
	args := appendLocalAppPiModelArgs(
		base,
		home,
		func(config piadapter.PiLocalModelServerConfig) (
			piadapter.PiLocalModelServerBinding,
			error,
		) {
			inspections++
			if config.PrivateRoot != wantRoot ||
				config.RuntimeArchivePath != wantArchive ||
				config.ExecutablePath != wantExecutable ||
				config.ModelPath != wantModel || config.Host != "127.0.0.1" ||
				config.Port != 18427 {
				t.Fatalf("Pi local model inspection config = %#v", config)
			}
			return piadapter.PiLocalModelServerBinding{}, nil
		},
	)
	want := append(append([]string(nil), base...),
		"--local-model-private-root", wantRoot,
		"--local-model-runtime-archive", wantArchive,
		"--local-model-executable", wantExecutable,
		"--local-model-path", wantModel,
	)
	if inspections != 1 || !reflect.DeepEqual(args, want) {
		t.Fatalf("inspections=%d args=%#v want=%#v", inspections, args, want)
	}
	base[1] = "/changed"
	if args[1] != "/private/state.sqlite" {
		t.Fatalf("Pi model args alias caller storage = %#v", args)
	}

	rejected := appendLocalAppPiModelArgs(
		[]string{"--state", "/private/state.sqlite"},
		home,
		func(piadapter.PiLocalModelServerConfig) (
			piadapter.PiLocalModelServerBinding,
			error,
		) {
			return piadapter.PiLocalModelServerBinding{}, piadapter.ErrInvalidPiLocalModel
		},
	)
	if !reflect.DeepEqual(rejected, []string{"--state", "/private/state.sqlite"}) {
		t.Fatalf("uninspected Pi model was published = %#v", rejected)
	}
}

func TestOptionalCredentialImportSourceFailsClosedOnUnsafePath(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "cc-switch.db")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if source := optionalCredentialImportSource(path); source == nil {
		t.Fatal("safe credential import source was rejected")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if source := optionalCredentialImportSource(path); source != nil {
		t.Fatal("world-readable credential import source was accepted")
	}
}

func TestExpandLocalAppServiceArgsRejectsMissingRuntime(t *testing.T) {
	_, err := expandLocalAppServiceArgs(
		[]string{"--local-app-service"},
		t.TempDir(),
		nil,
		"",
		"",
	)
	if err == nil {
		t.Fatal("missing local runtime was accepted")
	}
}

func TestParseLocalAppServiceInvocationBindsOptionalParent(t *testing.T) {
	parent, err := parseLocalAppServiceParentPID(
		[]string{"--local-app-service", "--parent-pid", "4242"},
	)
	if err != nil || parent != 4242 {
		t.Fatalf("parent = %d, err = %v", parent, err)
	}
	for _, args := range [][]string{
		{"--local-app-service", "--parent-pid", "0"},
		{"--local-app-service", "--parent-pid", "not-a-pid"},
		{"--local-app-service", "--unexpected"},
	} {
		if _, err := parseLocalAppServiceParentPID(args); err == nil {
			t.Fatalf("invalid local app invocation accepted: %q", args)
		}
	}
}

func TestPrepareLocalAppInvocationReexecsCanonicalArgsExactlyOnce(t *testing.T) {
	expanded := []string{
		"--state", "/tmp/loom/state/loom.db",
		"--isolation-root", "/tmp/loom/isolation",
		"--socket", "/tmp/loom/run/loomd.sock",
	}
	expandCalls := 0
	first, err := prepareLocalAppInvocation(
		[]string{localAppServiceFlag, "--parent-pid", "4242"},
		func() ([]string, error) {
			expandCalls++
			return append([]string(nil), expanded...), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical := append(
		append([]string(nil), expanded...),
		managedLocalAppParentFlag, "4242",
	)
	if !first.Reexec || first.ParentPID != 0 ||
		!reflect.DeepEqual(first.Args, wantCanonical) || expandCalls != 1 {
		t.Fatalf("first plan = %#v calls=%d", first, expandCalls)
	}
	second, err := prepareLocalAppInvocation(first.Args, func() ([]string, error) {
		t.Fatal("canonical invocation recursively expanded")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Reexec || second.ParentPID != 4242 ||
		!reflect.DeepEqual(second.Args, expanded) {
		t.Fatalf("second plan = %#v", second)
	}
	for _, argument := range second.Args {
		if argument == managedLocalAppParentFlag || argument == localAppServiceFlag {
			t.Fatalf("internal bootstrap argument reached run: %q", argument)
		}
	}
}

func TestPrepareLocalAppInvocationValidatesManagedParentHandoff(t *testing.T) {
	canonical := []string{
		"--state", "/tmp/loom/state/loom.db",
		"--isolation-root", "/tmp/loom/isolation",
		"--socket", "/tmp/loom/run/loomd.sock",
	}
	for _, args := range [][]string{
		append(append([]string(nil), canonical...), managedLocalAppParentFlag),
		append(append([]string(nil), canonical...), managedLocalAppParentFlag, "0"),
		append(append([]string(nil), canonical...), managedLocalAppParentFlag, "nope"),
		{managedLocalAppParentFlag, "4242"},
		append(
			append([]string(nil), canonical...),
			managedLocalAppParentFlag, "4242", "--interval", "1s",
		),
	} {
		if _, err := prepareLocalAppInvocation(args, localAppServiceArgs); err == nil {
			t.Fatalf("invalid managed-parent handoff accepted: %#v", args)
		}
	}
}

func TestManagedLocalAppDaemonCancelsAfterParentExit(t *testing.T) {
	parent := exec.Command("/usr/bin/true")
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	parentPID := parent.Process.Pid
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		cancelWhenLocalAppParentExits(ctx, cancel, parentPID)
		close(done)
	}()
	if err := parent.Wait(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("managed daemon did not cancel after parent exit")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("parent lifecycle observer did not join")
	}
}

func TestRunPublishesSafeBuildFailureReason(t *testing.T) {
	for _, reason := range []string{
		"build_observer", "build_state", "build_setup_runtime",
		"build_setup_credential", "build_setup_provider",
		"build_setup_native_auth", "build_decision", "build_execution",
		"build_ipc", "build_unknown",
	} {
		t.Run(reason, func(t *testing.T) {
			var stderr bytes.Buffer
			code := run(
				context.Background(), completeDaemonArgs(), &bytes.Buffer{}, &stderr,
				func(daemonBuildConfig) (daemonRunner, error) {
					return nil, newDaemonBuildFailure(reason, errors.New("/private/path"))
				},
			)
			if code != exitUnavailable {
				t.Fatalf("code = %d", code)
			}
			if got, want := stderr.String(), "daemon unavailable: "+reason+"\n"; got != want {
				t.Fatalf("stderr = %q, want %q", got, want)
			}
		})
	}
}

func TestRunBuildFailureAmbiguityFailsClosed(t *testing.T) {
	var stderr bytes.Buffer
	code := run(
		context.Background(), completeDaemonArgs(), &bytes.Buffer{}, &stderr,
		func(daemonBuildConfig) (daemonRunner, error) {
			return nil, errors.Join(
				newDaemonBuildFailure("build_state", errors.New("private state")),
				newDaemonBuildFailure("build_ipc", errors.New("private socket")),
			)
		},
	)
	if code != exitUnavailable {
		t.Fatalf("code = %d", code)
	}
	if got := stderr.String(); got != "daemon unavailable: build_unknown\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestDaemonBuildFailureNestedAmbiguityFailsClosed(t *testing.T) {
	err := newDaemonBuildFailure(
		"build_decision",
		newDaemonBuildFailure(
			"build_setup_native_auth",
			errors.New("private native auth path"),
		),
	)
	if got := daemonBuildFailureReason(err); got != "build_unknown" {
		t.Fatalf("reason = %q, want build_unknown", got)
	}
}

func TestMissionExecutionConfigFromDaemonBuildAllowsBrokeredOnlyWithoutLocalModel(
	t *testing.T,
) {
	config := missionExecutionConfigFromDaemonBuild(daemonBuildConfig{
		CodexExecutable:    "/opt/codex/bin/codex",
		ClaudeExecutable:   "/opt/claude/bin/claude",
		OpenCodeExecutable: "/opt/opencode/bin/opencode",
		Observer: app.LocalRuntimeObservationDaemonConfig{
			RuntimeSearchPaths: []string{"/opt/pi-a"},
			RuntimeInstanceID:  "runtime-1",
		},
	})
	if config == nil {
		t.Fatal("brokered-only execution config must not be nil")
	}
	if config.LocalModelCatalog != nil {
		t.Fatalf(
			"brokered-only execution config must not carry a local model catalog: %#v",
			config.LocalModelCatalog,
		)
	}
	if config.RuntimeInstanceID != "runtime-1" ||
		!reflect.DeepEqual(config.RuntimeSearchPaths, []string{"/opt/pi-a"}) ||
		config.CodexExecutable != "/opt/codex/bin/codex" ||
		config.ClaudeExecutable != "/opt/claude/bin/claude" ||
		config.OpenCodeExecutable != "/opt/opencode/bin/opencode" {
		t.Fatalf("execution config = %#v", config)
	}
}
