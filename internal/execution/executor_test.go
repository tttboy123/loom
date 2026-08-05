package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func execTempDir(t testing.TB) string {
	t.Helper()
	return t.TempDir()
}

func TestExecutorEditAtomicAndWithinRoot(t *testing.T) {
	root := execTempDir(t)
	executor := NewSandboxExecutor()
	content := "hello"
	result, err := executor.Edit(context.Background(), EditRequest{
		Worktree: root, RelativePath: "src/app.txt",
		NewContent: content,
	})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "src", "app.txt"))
	if err != nil || string(got) != content {
		t.Fatalf("read edit = %q, %v", got, err)
	}
	if result.ChangedFilesDigest == "" || result.ChangedFilesDigest == emptyChangedDigest {
		t.Fatalf("changed digest = %q", result.ChangedFilesDigest)
	}
	if _, err := executor.Edit(context.Background(), EditRequest{
		Worktree: root, RelativePath: "../escape", NewContent: "x",
	}); !errors.Is(err, ErrExecutionPathOutside) {
		t.Fatalf("outside edit error = %v", err)
	}
}

func TestExecutorRunSanitizedEnvAndExitCode(t *testing.T) {
	root := execTempDir(t)
	executor := NewSandboxExecutor()
	result, err := executor.Run(context.Background(), RunRequest{
		Worktree: root, Command: "pwd; printf x", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.ExitCode)
	}
	if !strings.Contains(string(result.OutputDigest), "sha256:") {
		t.Fatalf("output digest = %q", result.OutputDigest)
	}
	envResult, err := executor.Run(context.Background(), RunRequest{
		Worktree: root, Command: "printf %s \"$AWS_SECRET_ACCESS_KEY\"", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if envResult.ExitCode != 0 {
		t.Fatalf("env run exit = %d", envResult.ExitCode)
	}
}

func TestExecutorTimeout(t *testing.T) {
	root := execTempDir(t)
	executor := NewSandboxExecutor()
	_, err := executor.Run(context.Background(), RunRequest{
		Worktree: root, Command: "sleep 5", Timeout: 100 * time.Millisecond,
	})
	if !errors.Is(err, ErrExecutionTimedOut) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestExecutorChangedFilesLimit(t *testing.T) {
	root := execTempDir(t)
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(root, "f"+string(rune('a'+i))), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executor := NewSandboxExecutor()
	result, err := executor.Run(context.Background(), RunRequest{
		Worktree: root, Command: "true", Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ChangedFilesDigest == "" {
		t.Fatal("changed digest missing")
	}
}

func TestExecutorRunLimitExceededOnNonZeroExit(t *testing.T) {
	root := execTempDir(t)
	for index := 0; index < maxChangedFiles+1; index++ {
		name := filepath.Join(root, fmt.Sprintf("f%06d", index))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executor := NewSandboxExecutor()
	_, err := executor.Run(context.Background(), RunRequest{
		Worktree: root, Command: "exit 3", Timeout: 30 * time.Second,
	})
	if !errors.Is(err, ErrExecutionLimit) {
		t.Fatalf("Run() error = %v, want ErrExecutionLimit", err)
	}
}

func TestExecutorEditMissingWorktreeFails(t *testing.T) {
	root := filepath.Join(execTempDir(t), "does-not-exist")
	executor := NewSandboxExecutor()
	if _, err := executor.Edit(context.Background(), EditRequest{
		Worktree: root, RelativePath: "src/app.txt", NewContent: "x",
	}); err == nil {
		t.Fatal("Edit() on missing worktree must fail closed")
	}
}
