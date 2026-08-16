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
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

func TestMissionExecutionConfigFromDaemonBuildPreservesExactRuntimeBinding(
	t *testing.T,
) {
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
		captured.LocalModelCatalog.ExecutablePath != "/private/phase1-live/bin/llama-server" ||
		captured.LocalModelCatalog.ModelPath != "/private/phase1-live/models/model.gguf" {
		t.Fatalf("captured local model catalog = %#v", captured.LocalModelCatalog)
	}

	for _, partial := range [][]string{
		{"--local-model-private-root", "/private/phase1-live"},
		{"--local-model-executable", "/private/phase1-live/bin/llama-server"},
		{"--local-model-path", "/private/phase1-live/models/model.gguf"},
		{
			"--local-model-private-root", "/private/phase1-live",
			"--local-model-executable", "/private/phase1-live/bin/llama-server",
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
		"--process-timeout", "10s",
		"--socket", filepath.Join(home, "Library/Application Support/Loom/run/loomd.sock"),
		"--codex-executable", "/usr/bin/codex",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("expanded args = %#v, want %#v", args, want)
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
