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
	builder := func(config app.LocalRuntimeObservationDaemonConfig) (daemonRunner, error) {
		captured = config
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
			builder: func(app.LocalRuntimeObservationDaemonConfig) (daemonRunner, error) {
				return nil, errors.New("local path must not escape")
			},
			want: exitUnavailable,
		},
		{
			name: "runtime",
			args: completeDaemonArgs(),
			builder: func(app.LocalRuntimeObservationDaemonConfig) (daemonRunner, error) {
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
		func(app.LocalRuntimeObservationDaemonConfig) (daemonRunner, error) {
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
	return nil
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
