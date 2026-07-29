package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"loom-pi-rebuild/internal/app"
)

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
				detail: "private observer output",
			},
			want: "daemon failed: observer\n",
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
	detail string
}

func (failure testDaemonFailure) Error() string {
	return failure.detail
}

func (failure testDaemonFailure) DaemonFailureCode() string {
	return failure.code
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
