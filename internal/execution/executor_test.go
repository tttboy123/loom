package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutorReadAndGrepReturnBoundedTextWithoutFollowingSymlinks(t *testing.T) {
	root := execTempDir(t)
	content := []byte("alpha\nneedle one\nomega\nneedle two\n")
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	executor := NewSandboxExecutor()
	read, err := executor.Read(context.Background(), ReadRequest{
		Worktree: root, RelativePath: "notes.txt", OutputLimit: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	if string(read.Content) != "alpha\nneedle" || !read.Truncated ||
		read.ContentDigest != digestBytes(read.Content) ||
		read.OutputDigest != digestBytes(content) {
		t.Fatalf("Read() = %#v content=%q", read, read.Content)
	}
	grep, err := executor.Grep(context.Background(), GrepRequest{
		Worktree: root, RelativePath: "notes.txt", Pattern: "needle",
		OutputLimit: 128, MatchLimit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer grep.Close()
	if !bytes.Equal(grep.Content, []byte("2:needle one\n4:needle two\n")) || grep.Truncated ||
		grep.ContentDigest != digestBytes(grep.Content) {
		t.Fatalf("Grep() = %#v content=%q", grep, grep.Content)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Read(context.Background(), ReadRequest{
		Worktree: root, RelativePath: "escape.txt", OutputLimit: 64,
	}); !errors.Is(err, ErrExecutionPathOutside) {
		t.Fatalf("symlink Read() error = %v", err)
	}
}

func TestExecutorReadRejectsBinaryAndGrepRejectsInvalidPattern(t *testing.T) {
	root := execTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "binary"), []byte{'a', 0, 'b'}, 0o600); err != nil {
		t.Fatal(err)
	}
	executor := NewSandboxExecutor()
	if _, err := executor.Read(context.Background(), ReadRequest{
		Worktree: root, RelativePath: "binary", OutputLimit: 64,
	}); !errors.Is(err, ErrExecutionContent) {
		t.Fatalf("binary Read() error = %v", err)
	}
	if _, err := executor.Grep(context.Background(), GrepRequest{
		Worktree: root, RelativePath: "binary", Pattern: "[", OutputLimit: 64,
	}); !errors.Is(err, ErrInvalidExecutionInput) {
		t.Fatalf("invalid Grep() error = %v", err)
	}
}

func TestExecutorGrepSearchesWorkspaceRootWithoutFollowingSymlinks(t *testing.T) {
	root := execTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "game.js"), []byte("const snake = true;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := NewSandboxExecutor().Grep(context.Background(), GrepRequest{
		Worktree: root, RelativePath: ".", Pattern: "snake", OutputLimit: 128,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if string(result.Content) != "src/game.js:1:const snake = true;\n" {
		t.Fatalf("root Grep() content=%q", result.Content)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("snake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSandboxExecutor().Grep(context.Background(), GrepRequest{
		Worktree: root, RelativePath: ".", Pattern: "snake", OutputLimit: 128,
	}); !errors.Is(err, ErrExecutionPathOutside) {
		t.Fatalf("symlink root Grep() error=%v", err)
	}
}

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
