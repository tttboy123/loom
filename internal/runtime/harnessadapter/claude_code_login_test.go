package harnessadapter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSystemClaudeCodeLoginControllerStartsExactBoundedAuthCommand(t *testing.T) {
	homePath := t.TempDir()
	tempPath := t.TempDir()
	executablePath := filepath.Join(t.TempDir(), "claude")
	argumentsPath := filepath.Join(homePath, "arguments")
	pidPath := filepath.Join(homePath, "pid")
	script := "#!/bin/sh\n" +
		"if ! IFS= read -r acknowledgement; then exit 8; fi\n" +
		"if [ -n \"$acknowledgement\" ]; then exit 9; fi\n" +
		"printf '%s\\n' \"$@\" > \"$HOME/arguments\"\n" +
		"printf '%s\\n' \"$$\" > \"$HOME/pid\"\n" +
		"sleep 60\n"
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := NewSystemClaudeCodeLoginController(
		ClaudeCodeLoginControllerConfig{
			ExecutablePath: executablePath,
			HomePath:       homePath,
			TempPath:       tempPath,
			Timeout:        time.Minute,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = controller.Close() })
	if err := controller.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(context.Background()); !errors.Is(err, ErrClaudeCodeLoginBusy) {
		t.Fatalf("second Start() error = %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		arguments, readErr := os.ReadFile(argumentsPath)
		if readErr == nil {
			if string(arguments) != "auth\nlogin\n" {
				t.Fatalf("login arguments = %q", arguments)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("login command did not start: %v", readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil || pid <= 0 {
		t.Fatalf("login pid = %q, %v", pidData, err)
	}
	if err := controller.Cancel(); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("login process %d survived Cancel(): %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := controller.Start(context.Background()); err != nil {
		t.Fatalf("Start() after Cancel() error = %v", err)
	}
	if err := controller.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSystemClaudeCodeLoginControllerRejectsExecutableIdentityChange(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := NewSystemClaudeCodeLoginController(
		ClaudeCodeLoginControllerConfig{
			ExecutablePath: executablePath,
			HomePath:       t.TempDir(),
			TempPath:       t.TempDir(),
			Timeout:        time.Minute,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	replacement := executablePath + ".replacement"
	if err := os.WriteFile(replacement, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, executablePath); err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(context.Background()); !errors.Is(
		err, ErrClaudeCodeExecutableIdentityChanged,
	) {
		t.Fatalf("Start() error = %v", err)
	}
	if err := controller.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSystemClaudeCodeLoginControllerRejectsDirectoryIdentityChange(t *testing.T) {
	for _, target := range []string{"home", "temp"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			executablePath := filepath.Join(root, "claude")
			homePath := filepath.Join(root, "home")
			tempPath := filepath.Join(root, "temp")
			if err := os.WriteFile(executablePath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(homePath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(tempPath, 0o700); err != nil {
				t.Fatal(err)
			}
			controller, err := NewSystemClaudeCodeLoginController(
				ClaudeCodeLoginControllerConfig{
					ExecutablePath: executablePath,
					HomePath:       homePath,
					TempPath:       tempPath,
					Timeout:        time.Minute,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			changedPath := homePath
			if target == "temp" {
				changedPath = tempPath
			}
			if err := os.Rename(changedPath, changedPath+".original"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(changedPath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := controller.Start(context.Background()); !errors.Is(
				err, ErrClaudeCodeExecutableIdentityChanged,
			) {
				t.Fatalf("Start() error = %v", err)
			}
			if err := controller.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSystemClaudeCodeLoginControllerCancelKillsDescendantProcessGroup(t *testing.T) {
	homePath := t.TempDir()
	tempPath := t.TempDir()
	executablePath := filepath.Join(t.TempDir(), "claude")
	leaderPath := filepath.Join(homePath, "leader-pid")
	childPath := filepath.Join(homePath, "child-pid")
	script := "#!/bin/sh\n" +
		"IFS= read -r acknowledgement || exit 8\n" +
		"[ -z \"$acknowledgement\" ] || exit 9\n" +
		"printf '%s\\n' \"$$\" > \"$HOME/leader-pid\"\n" +
		"sleep 60 &\n" +
		"child=$!\n" +
		"printf '%s\\n' \"$child\" > \"$HOME/child-pid\"\n" +
		"wait \"$child\"\n"
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	controller, err := NewSystemClaudeCodeLoginController(
		ClaudeCodeLoginControllerConfig{
			ExecutablePath: executablePath,
			HomePath:       homePath,
			TempPath:       tempPath,
			Timeout:        time.Minute,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = controller.Close() })
	if err := controller.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	readPID := func(path string) int {
		// Process admission can be delayed while the repository matrix compiles
		// Swift probes in parallel. This only bounds fixture startup; the actual
		// Cancel process-group termination contract below remains five seconds.
		deadline := time.Now().Add(30 * time.Second)
		for {
			data, readErr := os.ReadFile(path)
			if readErr == nil {
				pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
				if parseErr == nil && pid > 0 {
					return pid
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("process pid did not appear at %s: %v", path, readErr)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	leaderPID := readPID(leaderPath)
	childPID := readPID(childPath)
	if err := controller.Cancel(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		leaderErr := syscall.Kill(leaderPID, 0)
		childErr := syscall.Kill(childPID, 0)
		groupErr := syscall.Kill(-leaderPID, 0)
		if errors.Is(leaderErr, syscall.ESRCH) &&
			errors.Is(childErr, syscall.ESRCH) &&
			errors.Is(groupErr, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"login process group survived Cancel(): leader=%v child=%v group=%v",
				leaderErr, childErr, groupErr,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
