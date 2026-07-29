package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

type codexStatusFixtureRunner struct {
	result  CodexStatusProcessResult
	err     error
	request CodexStatusProcessRequest
}

func (runner *codexStatusFixtureRunner) RunCodexStatus(
	_ context.Context,
	request CodexStatusProcessRequest,
) (CodexStatusProcessResult, error) {
	runner.request = request
	return runner.result, runner.err
}

func TestCodexNativeAuthObserverUsesExactStatusCommandAndClosedMapping(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &codexStatusFixtureRunner{
		result: CodexStatusProcessResult{
			Stdout:   []byte("Logged in using ChatGPT\n"),
			ExitCode: 0,
		},
	}
	observer, err := NewCodexNativeAuthObserver(CodexNativeAuthConfig{
		ExecutablePath: executable,
		Timeout:        5 * time.Second,
		MaxOutputBytes: 1024,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("NewCodexNativeAuthObserver() error = %v", err)
	}
	result, err := observer.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	if result.Status != CodexNativeAuthAvailable ||
		result.AuthMode != "native_auth" ||
		result.Reason != CodexNativeAuthReasonNone {
		t.Fatalf("Observe() = %#v", result)
	}
	if runner.request.ExecutablePath != executable ||
		!reflect.DeepEqual(runner.request.Arguments, []string{"login", "status"}) ||
		len(runner.request.Environment) != 0 ||
		runner.request.MaxOutputBytes != 1024 {
		t.Fatalf("request = %#v", runner.request)
	}
}

func TestCodexNativeAuthObserverAcceptsExactStatusFromOneOutputChannel(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		result CodexStatusProcessResult
		status CodexNativeAuthStatus
		reason CodexNativeAuthReason
	}{
		{
			name: "codex 0.144.1 stderr success",
			result: CodexStatusProcessResult{
				Stderr:   []byte("Logged in using ChatGPT\n"),
				ExitCode: 0,
			},
			status: CodexNativeAuthAvailable,
			reason: CodexNativeAuthReasonNone,
		},
		{
			name: "stderr not logged in",
			result: CodexStatusProcessResult{
				Stderr:   []byte("Not logged in\n"),
				ExitCode: 1,
			},
			status: CodexNativeAuthNotLoggedIn,
			reason: CodexNativeAuthReasonNotLoggedIn,
		},
		{
			name: "both channels are ambiguous",
			result: CodexStatusProcessResult{
				Stdout:   []byte("Logged in using ChatGPT\n"),
				Stderr:   []byte("Logged in using ChatGPT\n"),
				ExitCode: 0,
			},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "stdout multiline is ambiguous",
			result: CodexStatusProcessResult{
				Stdout: []byte(
					"Logged in using ChatGPT\nunreviewed diagnostic\n",
				),
				ExitCode: 0,
			},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "stderr multiline is ambiguous",
			result: CodexStatusProcessResult{
				Stderr: []byte(
					"Logged in using ChatGPT\nunreviewed diagnostic\n",
				),
				ExitCode: 0,
			},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observer, err := NewCodexNativeAuthObserver(CodexNativeAuthConfig{
				ExecutablePath: executable,
				Timeout:        5 * time.Second,
				MaxOutputBytes: 1024,
				Runner: &codexStatusFixtureRunner{
					result: test.result,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := observer.Observe(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != test.status || got.Reason != test.reason ||
				len(got.RawOutput) != 0 {
				t.Fatalf("Observe() = %#v", got)
			}
		})
	}
}

func TestCodexNativeAuthObserverRejectsUnknownOutputAndTimeout(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		runner *codexStatusFixtureRunner
		status CodexNativeAuthStatus
		reason CodexNativeAuthReason
	}{
		{
			name: "unknown output",
			runner: &codexStatusFixtureRunner{result: CodexStatusProcessResult{
				Stdout:   []byte("unreviewed output\n"),
				ExitCode: 0,
			}},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "leading whitespace is not an exact line",
			runner: &codexStatusFixtureRunner{result: CodexStatusProcessResult{
				Stdout:   []byte(" Logged in using ChatGPT\n"),
				ExitCode: 0,
			}},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "stdout plus stderr diagnostic is ambiguous",
			runner: &codexStatusFixtureRunner{result: CodexStatusProcessResult{
				Stdout:   []byte("Logged in using ChatGPT\n"),
				Stderr:   []byte("unreviewed diagnostic\n"),
				ExitCode: 0,
			}},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "timeout",
			runner: &codexStatusFixtureRunner{
				err: context.DeadlineExceeded,
			},
			status: CodexNativeAuthUnavailable,
			reason: CodexNativeAuthReasonTimeout,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observer, err := NewCodexNativeAuthObserver(CodexNativeAuthConfig{
				ExecutablePath: executable,
				Timeout:        5 * time.Second,
				MaxOutputBytes: 1024,
				Runner:         test.runner,
			})
			if err != nil {
				t.Fatalf("NewCodexNativeAuthObserver() error = %v", err)
			}
			result, err := observer.Observe(context.Background())
			if err != nil {
				t.Fatalf("Observe() error = %v", err)
			}
			if result.Status != test.status || result.Reason != test.reason {
				t.Fatalf("Observe() = %#v", result)
			}
			if len(result.RawOutput) != 0 {
				t.Fatalf("raw output escaped = %q", result.RawOutput)
			}
		})
	}
}

func TestSystemCodexLoginControllerStartsExactSingletonAndJoinsOnClose(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	argumentsPath := filepath.Join(root, "arguments")
	pidPath := filepath.Join(root, "pid")
	script := fmt.Sprintf(
		"#!/bin/sh\nprintf '%%s' \"$*\" > %q\nprintf '%%s' \"$$\" > %q\n/bin/sleep 30\n",
		argumentsPath,
		pidPath,
	)
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := NewSystemCodexLoginController(
		CodexLoginControllerConfig{
			ExecutablePath: executable,
			Timeout:        time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("NewSystemCodexLoginController() error = %v", err)
	}
	result, err := controller.Start(context.Background())
	if err != nil || result.Status != CodexLoginStarted {
		t.Fatalf("Start() = %#v, %v", result, err)
	}
	if _, err := controller.Start(context.Background()); !errors.Is(
		err,
		ErrCodexLoginBusy,
	) {
		t.Fatalf("concurrent Start() error = %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(pidPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("login fixture did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	arguments, err := os.ReadFile(argumentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(arguments) != "login" {
		t.Fatalf("arguments = %q", arguments)
	}
	if err := controller.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("login process %d survived Close(): %v", pid, err)
	}
}

func TestSystemCodexLoginControllerRejectsChangedExecutableIdentity(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(
		executable,
		[]byte("#!/bin/sh\nexit 0\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	controller, err := NewSystemCodexLoginController(
		CodexLoginControllerConfig{
			ExecutablePath: executable,
			Timeout:        time.Minute,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(
		replacement,
		[]byte("#!/bin/sh\nexit 1\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, executable); err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Start(context.Background()); !errors.Is(
		err,
		ErrCodexExecutableIdentityChanged,
	) {
		t.Fatalf("Start() error = %v", err)
	}
	if err := controller.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCodexNativeAuthObserverRejectsExecutableIdentityChange(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("first"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &codexStatusFixtureRunner{
		err: ErrCodexExecutableIdentityChanged,
	}
	observer, err := NewCodexNativeAuthObserver(CodexNativeAuthConfig{
		ExecutablePath: executable,
		Timeout:        5 * time.Second,
		MaxOutputBytes: 1024,
		Runner:         runner,
	})
	if err != nil {
		t.Fatalf("NewCodexNativeAuthObserver() error = %v", err)
	}
	result, err := observer.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe() error = %v", err)
	}
	if result.Status != CodexNativeAuthUnavailable ||
		result.Reason != CodexNativeAuthReasonIdentityChanged ||
		!errors.Is(runner.err, ErrCodexExecutableIdentityChanged) {
		t.Fatalf("Observe() = %#v", result)
	}
}

func TestSystemCodexStatusRunnerBoundsOutputAndCancelsProcessGroup(
	t *testing.T,
) {
	root := t.TempDir()
	outputExecutable := filepath.Join(root, "codex-output")
	if err := os.WriteFile(
		outputExecutable,
		[]byte("#!/bin/sh\ni=0\nwhile [ \"$i\" -lt 256 ]; do printf x; i=$((i+1)); done\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	runner := NewSystemCodexStatusRunner()
	outputResult, err := runner.RunCodexStatus(
		context.Background(),
		CodexStatusProcessRequest{
			ExecutablePath: outputExecutable,
			Arguments:      []string{"login", "status"},
			Environment:    []string{},
			MaxOutputBytes: 64,
		},
	)
	if !errors.Is(err, errCodexStatusOutputLimit) {
		t.Fatalf(
			"output limit error = %v, stdout bytes = %d, exit = %d",
			err,
			len(outputResult.Stdout),
			outputResult.ExitCode,
		)
	}

	pidFile := filepath.Join(root, "child.pid")
	cancelExecutable := filepath.Join(root, "codex-cancel")
	script := fmt.Sprintf(
		"#!/bin/sh\n/bin/sleep 30 &\nchild=$!\nprintf '%%s' \"$child\" > %q\nwait \"$child\"\n",
		pidFile,
	)
	if err := os.WriteFile(cancelExecutable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, runErr := runner.RunCodexStatus(
			ctx,
			CodexStatusProcessRequest{
				ExecutablePath: cancelExecutable,
				Arguments:      []string{"login", "status"},
				Environment:    []string{},
				MaxOutputBytes: 1024,
			},
		)
		runDone <- runErr
	}()
	startDeadline := time.Now().Add(10 * time.Second)
	for {
		if _, statErr := os.Stat(pidFile); statErr == nil {
			break
		}
		select {
		case runErr := <-runDone:
			cancel()
			t.Fatalf("child fixture exited before start: %v", runErr)
		default:
		}
		if time.Now().After(startDeadline) {
			cancel()
			t.Fatal("child process fixture did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	err = <-runDone
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	pidBytes, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var childPID int
	if _, scanErr := fmt.Sscanf(string(pidBytes), "%d", &childPID); scanErr != nil {
		t.Fatal(scanErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		signalErr := syscall.Kill(childPID, 0)
		if errors.Is(signalErr, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("child process %d survived process-group cancellation", childPID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSystemCodexStatusRunnerRevalidatesExecutableIdentityAfterRun(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	marker := filepath.Join(root, "started")
	script := fmt.Sprintf(
		"#!/bin/sh\n: > %q\n/bin/sleep 0.3\nprintf 'Logged in using ChatGPT\\n'\n",
		marker,
	)
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(
		replacement,
		[]byte("#!/bin/sh\nprintf 'Not logged in\\n'\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	replaced := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(marker); err == nil {
				replaced <- os.Rename(replacement, executable)
				return
			}
			if time.Now().After(deadline) {
				replaced <- errors.New("fixture did not start")
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	_, err := NewSystemCodexStatusRunner().RunCodexStatus(
		context.Background(),
		CodexStatusProcessRequest{
			ExecutablePath: executable,
			Arguments:      []string{"login", "status"},
			Environment:    []string{},
			MaxOutputBytes: 1024,
		},
	)
	if replaceErr := <-replaced; replaceErr != nil {
		t.Fatal(replaceErr)
	}
	if !errors.Is(err, ErrCodexExecutableIdentityChanged) {
		t.Fatalf("identity change error = %v", err)
	}
}
