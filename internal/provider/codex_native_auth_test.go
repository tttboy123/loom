package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

type codexAuthProbeFixtureRunner struct {
	result  CodexNativeAuthProbeResult
	err     error
	request CodexNativeAuthProbeRequest
	calls   int
}

func (runner *codexAuthProbeFixtureRunner) ProbeCodexNativeAuth(
	_ context.Context,
	request CodexNativeAuthProbeRequest,
) (CodexNativeAuthProbeResult, error) {
	runner.calls++
	runner.request = request
	return runner.result, runner.err
}

func TestCodexNativeAuthObserverUsesActiveRefreshProbeAndClosedMapping(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &codexAuthProbeFixtureRunner{
		result: CodexNativeAuthProbeResult{
			Authenticated:      true,
			RequiresOpenAIAuth: true,
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
		runner.request.MaxOutputBytes != 1024 {
		t.Fatalf("request = %#v", runner.request)
	}
}

func TestCodexNativeAuthObserverMapsClosedActiveProbeResults(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		result CodexNativeAuthProbeResult
		status CodexNativeAuthStatus
		reason CodexNativeAuthReason
	}{
		{
			name: "non OpenAI authenticated account",
			result: CodexNativeAuthProbeResult{
				Authenticated: true,
			},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "refresh requires login",
			result: CodexNativeAuthProbeResult{
				RequiresOpenAIAuth: true,
			},
			status: CodexNativeAuthNotLoggedIn,
			reason: CodexNativeAuthReasonNotLoggedIn,
		},
		{
			name: "authenticated OpenAI account",
			result: CodexNativeAuthProbeResult{
				Authenticated: true, RequiresOpenAIAuth: true,
			},
			status: CodexNativeAuthAvailable,
			reason: CodexNativeAuthReasonNone,
		},
		{
			name:   "missing account result",
			result: CodexNativeAuthProbeResult{},
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
				Runner: &codexAuthProbeFixtureRunner{
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
		runner *codexAuthProbeFixtureRunner
		status CodexNativeAuthStatus
		reason CodexNativeAuthReason
	}{
		{
			name:   "unknown result",
			runner: &codexAuthProbeFixtureRunner{result: CodexNativeAuthProbeResult{}},
			status: CodexNativeAuthUnsupported,
			reason: CodexNativeAuthReasonUnknownOutput,
		},
		{
			name: "timeout",
			runner: &codexAuthProbeFixtureRunner{
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

func TestCodexNativeAuthObserverCachesAndExplicitlyInvalidatesClosedResult(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("fixture"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &codexAuthProbeFixtureRunner{result: CodexNativeAuthProbeResult{
		Authenticated: true, RequiresOpenAIAuth: true,
	}}
	observer, err := NewCodexNativeAuthObserver(CodexNativeAuthConfig{
		ExecutablePath: executable, Timeout: 5 * time.Second,
		MaxOutputBytes: 1024, Runner: runner,
	})
	if err != nil {
		t.Fatal(err)
	}
	for count := 0; count < 2; count++ {
		result, err := observer.Observe(context.Background())
		if err != nil || result.Status != CodexNativeAuthAvailable {
			t.Fatalf("cached observation %d = %#v, %v", count, result, err)
		}
	}
	if runner.calls != 1 {
		t.Fatalf("probe calls before invalidation = %d", runner.calls)
	}
	observer.Invalidate()
	if _, err := observer.Observe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 2 {
		t.Fatalf("probe calls after invalidation = %d", runner.calls)
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
	// Process startup can contend with the repository's Swift contract build.
	// Close below still has to join the already-started process immediately.
	deadline := time.Now().Add(15 * time.Second)
	pid := 0
	var pidErr error
	for {
		pidBytes, err := os.ReadFile(pidPath)
		if err == nil {
			pid, pidErr = strconv.Atoi(string(pidBytes))
			if pidErr == nil && pid > 0 {
				break
			}
			if pidErr == nil {
				pidErr = fmt.Errorf("non-positive pid %d", pid)
			}
		} else {
			pidErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("login fixture pid not ready: %v", pidErr)
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
	runner := &codexAuthProbeFixtureRunner{
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

func TestSystemCodexNativeAuthProbeRunnerUsesRefreshProtocolAndReturnsClosedState(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	argumentsPath := filepath.Join(root, "arguments")
	initializePath := filepath.Join(root, "initialize")
	accountPath := filepath.Join(root, "account")
	script := fmt.Sprintf(
		"#!/bin/sh\nprintf '%%s' \"$*\" > %q\nIFS= read -r initialize\nprintf '%%s' \"$initialize\" > %q\nprintf '%%s\\n' '{\"id\":\"loom-auth-initialize-v1\",\"result\":{\"userAgent\":\"codex-cli 0.144.1\",\"codexHome\":\"/private/home\",\"platformFamily\":\"unix\",\"platformOs\":\"macos\"}}'\nIFS= read -r account\nprintf '%%s' \"$account\" > %q\nprintf '%%s\\n' '{\"id\":\"loom-auth-account-read-v1\",\"result\":{\"account\":{\"type\":\"chatgpt\",\"email\":\"private@example.invalid\",\"planType\":\"plus\"},\"requiresOpenaiAuth\":false}}'\n",
		argumentsPath, initializePath, accountPath,
	)
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	result, err := NewSystemCodexNativeAuthProbeRunner().ProbeCodexNativeAuth(
		context.Background(),
		CodexNativeAuthProbeRequest{
			ExecutablePath: executable, MaxOutputBytes: 8 << 10,
		},
	)
	if err != nil || !result.Authenticated || result.RequiresOpenAIAuth {
		t.Fatalf("probe result = %#v, error = %v", result, err)
	}
	arguments, err := os.ReadFile(argumentsPath)
	if err != nil || string(arguments) != "-c mcp_servers={} app-server --stdio" {
		t.Fatalf("arguments = %q, error = %v", arguments, err)
	}
	initialize, err := os.ReadFile(initializePath)
	if err != nil {
		t.Fatal(err)
	}
	var initializeRequest struct {
		ID     string `json:"id"`
		Method string `json:"method"`
	}
	if json.Unmarshal(initialize, &initializeRequest) != nil ||
		initializeRequest.ID != codexAuthProbeInitializeID ||
		initializeRequest.Method != "initialize" {
		t.Fatalf("initialize request = %s", initialize)
	}
	account, err := os.ReadFile(accountPath)
	if err != nil {
		t.Fatal(err)
	}
	var accountRequest struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			RefreshToken bool `json:"refreshToken"`
		} `json:"params"`
	}
	if json.Unmarshal(account, &accountRequest) != nil ||
		accountRequest.ID != codexAuthProbeAccountID ||
		accountRequest.Method != "account/read" || !accountRequest.Params.RefreshToken {
		t.Fatalf("account request = %s", account)
	}
}

func TestLiveSystemCodexNativeAuthProbeRunner(t *testing.T) {
	executable := os.Getenv("LOOM_CODEX_AUTH_PROBE_EXECUTABLE")
	expected := os.Getenv("LOOM_CODEX_AUTH_PROBE_EXPECTED")
	if executable == "" || expected == "" {
		t.Skip("live Codex auth probe requires executable and expected status")
	}
	resolved, err := ResolveCodexNativeExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := NewSystemCodexNativeAuthProbeRunner().ProbeCodexNativeAuth(
		ctx,
		CodexNativeAuthProbeRequest{
			ExecutablePath: resolved, MaxOutputBytes: 64 << 10,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	switch expected {
	case "available":
		if !result.Authenticated || !result.RequiresOpenAIAuth {
			t.Fatalf("auth probe = %#v", result)
		}
	case "not_logged_in":
		if result.Authenticated || !result.RequiresOpenAIAuth {
			t.Fatalf("auth probe = %#v", result)
		}
	default:
		t.Fatalf("invalid expected status %q", expected)
	}
}

func TestSystemCodexNativeAuthProbeRunnerBoundsOutputAndCancelsProcessGroup(
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
	runner := NewSystemCodexNativeAuthProbeRunner()
	_, err := runner.ProbeCodexNativeAuth(
		context.Background(),
		CodexNativeAuthProbeRequest{
			ExecutablePath: outputExecutable,
			MaxOutputBytes: 64,
		},
	)
	if !errors.Is(err, errCodexAuthProbeOutputLimit) {
		t.Fatalf("output limit error = %v", err)
	}

	pidFile := filepath.Join(root, "child.pid")
	pidTempFile := pidFile + ".tmp"
	cancelExecutable := filepath.Join(root, "codex-cancel")
	script := fmt.Sprintf(
		"#!/bin/sh\n/bin/sleep 30 &\nchild=$!\nprintf '%%s' \"$child\" > %q\n/bin/mv %q %q\nwait \"$child\"\n",
		pidTempFile,
		pidTempFile,
		pidFile,
	)
	if err := os.WriteFile(cancelExecutable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, runErr := runner.ProbeCodexNativeAuth(
			ctx,
			CodexNativeAuthProbeRequest{
				ExecutablePath: cancelExecutable,
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

func TestSystemCodexNativeAuthProbeRunnerRevalidatesExecutableIdentityAfterRun(
	t *testing.T,
) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	marker := filepath.Join(root, "started")
	script := fmt.Sprintf(
		"#!/bin/sh\nIFS= read -r initialize\n: > %q\n/bin/sleep 0.3\nprintf '%%s\\n' '{\"id\":\"loom-auth-initialize-v1\",\"result\":{\"userAgent\":\"codex-cli 0.144.1\",\"codexHome\":\"/private/home\",\"platformFamily\":\"unix\",\"platformOs\":\"macos\"}}'\nIFS= read -r account\nprintf '%%s\\n' '{\"id\":\"loom-auth-account-read-v1\",\"result\":{\"account\":null,\"requiresOpenaiAuth\":true}}'\n",
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
	_, err := NewSystemCodexNativeAuthProbeRunner().ProbeCodexNativeAuth(
		context.Background(),
		CodexNativeAuthProbeRequest{
			ExecutablePath: executable,
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

func TestResolveCodexNativeExecutableFromOfficialNPMLauncher(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "lib", "node_modules", "@openai", "codex")
	wrapper := filepath.Join(packageRoot, "bin", "codex.js")
	native := filepath.Join(
		packageRoot, "node_modules", "@openai", "codex-darwin-arm64",
		"vendor", "aarch64-apple-darwin", "bin", "codex",
	)
	for _, path := range []string{filepath.Dir(wrapper), filepath.Dir(native)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(wrapper, []byte("#!/usr/bin/env node\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(native, []byte("native"), 0o700); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "bin", "codex")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wrapper, launcher); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveCodexNativeExecutable(launcher)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(native)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
}

func TestResolveCodexNativeExecutableRejectsArbitrarySymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "codex")
	if err := os.Symlink(target, launcher); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveCodexNativeExecutable(launcher); !errors.Is(err, ErrInvalidCodexNativeAuthConfig) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveCodexNativeExecutableAcceptsDirectRegularBinary(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(executable, []byte("native"), 0o700); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveCodexNativeExecutable(executable)
	if err != nil || resolved != executable {
		t.Fatalf("resolved=%q error=%v", resolved, err)
	}
}

func TestResolveCodexNativeExecutableRejectsSymlinkedNativeTarget(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "node_modules", "@openai", "codex")
	wrapper := filepath.Join(packageRoot, "bin", "codex.js")
	native := filepath.Join(packageRoot, "node_modules", "@openai", "codex-darwin-arm64", "vendor", "aarch64-apple-darwin", "bin", "codex")
	for _, directory := range []string{filepath.Dir(wrapper), filepath.Dir(native)} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(wrapper, []byte("#!/usr/bin/env node\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	realNative := filepath.Join(root, "real-codex")
	if err := os.WriteFile(realNative, []byte("native"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realNative, native); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "codex")
	if err := os.Symlink(wrapper, launcher); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveCodexNativeExecutable(launcher); !errors.Is(err, ErrInvalidCodexNativeAuthConfig) {
		t.Fatalf("error=%v", err)
	}
}
