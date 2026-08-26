package harnessadapter

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSystemHarnessSessionRunnerUsesOneExactProcessForJSONL(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	script := `#!/bin/sh
while IFS= read -r line; do
  printf '{"echo":%s}\n' "$line"
done
printf 'bounded stderr' >&2
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	session, err := NewSystemHarnessSessionRunner().StartSession(
		context.Background(),
		HarnessSessionRequest{
			ExecutablePath: executable,
			Environment:    []string{"PATH=/usr/bin:/bin"},
			Directory:      root,
			MaxOutputBytes: 4096,
			Timeout:        5 * time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"first private input", "second private input"} {
		line, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if err := session.WriteLine(context.Background(), line); err != nil {
			t.Fatal(err)
		}
		if !allHarnessBytesZero(line) {
			t.Fatal("session input plaintext was not cleared after write")
		}
		response, readErr := session.ReadLine(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		var decoded struct {
			Echo string `json:"echo"`
		}
		if json.Unmarshal(response, &decoded) != nil || decoded.Echo != value {
			t.Fatalf("response = %q", response)
		}
		zeroHarnessBytes(response)
	}
	if err := session.CloseInput(); err != nil {
		t.Fatal(err)
	}
	result, err := session.Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || len(result.Stdout) != 0 ||
		string(result.Stderr) != "bounded stderr" {
		t.Fatalf("result = %#v", result)
	}
}

func TestHarnessSessionRequestAllowsPersistentSegmentLifetime(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if !validHarnessSessionRequest(HarnessSessionRequest{
		ExecutablePath: executable,
		Environment:    []string{"PATH=/usr/bin:/bin"},
		Directory:      root,
		MaxOutputBytes: 4096,
		Timeout:        8 * time.Hour,
	}) {
		t.Fatal("persistent Segment lifetime was rejected by session admission")
	}
}

func TestSystemHarnessSessionRunnerKillsCanceledProcessGroup(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	pidPath := filepath.Join(root, "child.pid")
	script := `#!/bin/sh
sleep 30 &
child=$!
printf '%s' "$child" > "$1"
wait "$child"
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	session, err := NewSystemHarnessSessionRunner().StartSession(
		ctx,
		HarnessSessionRequest{
			ExecutablePath: executable,
			Arguments:      []string{pidPath},
			Environment:    []string{"PATH=/usr/bin:/bin"},
			Directory:      root,
			MaxOutputBytes: 4096,
			Timeout:        10 * time.Second,
		},
	)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	pidText := waitForHarnessPIDFile(t, pidPath)
	cancel()
	if _, err := session.Wait(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v", err)
	}
	assertHarnessProcessGone(t, pidText)
}

func TestSystemHarnessSessionRunnerRejectsOversizedLine(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(
		executable,
		[]byte("#!/bin/sh\ni=0\nwhile [ \"$i\" -lt 2000 ]; do printf x; i=$((i+1)); done\nprintf '\\n'\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	session, err := NewSystemHarnessSessionRunner().StartSession(
		context.Background(),
		HarnessSessionRequest{
			ExecutablePath: executable,
			Environment:    []string{"PATH=/usr/bin:/bin"},
			Directory:      root,
			MaxOutputBytes: 1024,
			Timeout:        5 * time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if line, err := session.ReadLine(context.Background()); !errors.Is(err, ErrHarnessProcessUnavailable) {
		zeroHarnessBytes(line)
		t.Fatalf("ReadLine() error = %v", err)
	}
	_ = session.Abort()
}

func TestSystemHarnessSessionRunnerAllowsBoundedLinesBeyondCumulativeLimit(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "codex")
	if err := os.WriteFile(
		executable,
		[]byte("#!/bin/sh\ni=0\nwhile [ \"$i\" -lt 3 ]; do printf '%0600d\\n' 0; i=$((i+1)); done\n"),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	session, err := NewSystemHarnessSessionRunner().StartSession(
		context.Background(),
		HarnessSessionRequest{
			ExecutablePath: executable,
			Environment:    []string{"PATH=/usr/bin:/bin"},
			Directory:      root,
			MaxOutputBytes: 1024,
			Timeout:        5 * time.Second,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Abort()
	for index := 0; index < 3; index++ {
		line, readErr := session.ReadLine(context.Background())
		if readErr != nil {
			t.Fatalf("ReadLine(%d) error = %v", index, readErr)
		}
		if len(line) != 600 {
			t.Fatalf("ReadLine(%d) bytes = %d", index, len(line))
		}
	}
}

func waitForHarnessPIDFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(content)) != "" {
			return strings.TrimSpace(string(content))
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("child process PID was not published")
	return ""
}

func assertHarnessProcessGone(t *testing.T, pidText string) {
	t.Helper()
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 1 {
		t.Fatalf("child pid = %q error=%v", pidText, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		killErr := syscall.Kill(pid, 0)
		if errors.Is(killErr, syscall.ESRCH) {
			return
		}
		if killErr != nil || time.Now().After(deadline) {
			t.Fatalf("child process %d remains: %v", pid, killErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSystemHarnessCommandRunnerUsesExactExecutableAndBoundedIO(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	script := `#!/bin/sh
read prompt
printf '{"type":"result","subtype":"success","is_error":false,"result":"%s","usage":{"input_tokens":1,"output_tokens":1}}' "$prompt"
printf 'bounded stderr' >&2
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := NewSystemHarnessCommandRunner()
	stdin := []byte("bounded prompt")
	result, err := runner.RunCommand(context.Background(), HarnessCommandRequest{
		ExecutablePath: executable,
		Arguments:      []string{"--print"},
		Environment:    []string{"HOME=" + root, "PATH=/usr/bin:/bin"},
		Directory:      root,
		Stdin:          stdin,
		MaxOutputBytes: 4096,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !allHarnessBytesZero(stdin) {
		t.Fatal("Harness stdin plaintext was not cleared after process exit")
	}
	if result.ExitCode != 0 ||
		!strings.Contains(string(result.Stdout), `"result":"bounded prompt"`) ||
		string(result.Stderr) != "bounded stderr" {
		t.Fatalf("result = %#v", result)
	}
}

func allHarnessBytesZero(content []byte) bool {
	for _, value := range content {
		if value != 0 {
			return false
		}
	}
	return true
}

func TestSystemHarnessCommandRunnerRejectsIdentityDriftAndBoundsOutput(t *testing.T) {
	root := t.TempDir()
	runner := NewSystemHarnessCommandRunner()

	symlinkTarget := filepath.Join(root, "target")
	if err := os.WriteFile(symlinkTarget, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, "claude")
	if err := os.Symlink(symlinkTarget, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.RunCommand(context.Background(), HarnessCommandRequest{
		ExecutablePath: symlink, Directory: root, Environment: []string{"PATH=/usr/bin:/bin"},
		MaxOutputBytes: 4096, Timeout: time.Second,
	}); !errors.Is(err, ErrHarnessProcessUnavailable) {
		t.Fatalf("symlink RunCommand() error = %v", err)
	}

	oversized := filepath.Join(root, "oversized")
	if err := os.WriteFile(oversized, []byte("#!/bin/sh\nyes x | head -c 5000\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.RunCommand(context.Background(), HarnessCommandRequest{
		ExecutablePath: oversized, Directory: root, Environment: []string{"PATH=/usr/bin:/bin"},
		MaxOutputBytes: 1024, Timeout: 5 * time.Second,
	}); !errors.Is(err, ErrHarnessProcessUnavailable) {
		t.Fatalf("oversized RunCommand() error = %v", err)
	}
}

func TestSystemHarnessCommandRunnerKillsTimedOutProcessGroup(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "claude")
	pidPath := filepath.Join(root, "child.pid")
	script := `#!/bin/sh
sleep 30 &
child=$!
printf '%s' "$child" > "$1"
wait "$child"
`
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := NewSystemHarnessCommandRunner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	childPID := make(chan string, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			pidBytes, err := os.ReadFile(pidPath)
			if err == nil && strings.TrimSpace(string(pidBytes)) != "" {
				childPID <- strings.TrimSpace(string(pidBytes))
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		childPID <- ""
		cancel()
	}()
	started := time.Now()
	_, err := runner.RunCommand(ctx, HarnessCommandRequest{
		ExecutablePath: executable, Arguments: []string{pidPath},
		Directory: root, Environment: []string{"PATH=/usr/bin:/bin"},
		MaxOutputBytes: 4096, Timeout: 10 * time.Second,
	})
	if !errors.Is(err, context.Canceled) || time.Since(started) > 7*time.Second {
		t.Fatalf("timed RunCommand() error = %v elapsed=%s", err, time.Since(started))
	}
	pidText := <-childPID
	pid, parseErr := strconv.Atoi(pidText)
	if parseErr != nil || pid <= 1 {
		t.Fatalf("child pid = %q error=%v", pidText, parseErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		killErr := syscall.Kill(pid, 0)
		if errors.Is(killErr, syscall.ESRCH) {
			break
		}
		if killErr != nil || time.Now().After(deadline) {
			t.Fatalf("timed-out child process %d remains: %v", pid, killErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestResolveHarnessExecutableReturnsRegularTargetWithoutChangingInstallation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "claude-2.1.196")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(root, "claude")
	if err := os.Symlink(target, launcher); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveHarnessExecutable(launcher)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
}
