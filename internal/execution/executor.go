package execution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultRunTimeout  = 60 * time.Second
	maxRunTimeout      = 600 * time.Second
	defaultOutputLimit = int64(1 << 20)
	maxChangedFiles    = 10_000
	maxChangedBytes    = int64(64 << 20)
	emptyChangedDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

// EmptyChangedFilesDigest is the fixed digest for an empty changed-file set.
func EmptyChangedFilesDigest() string {
	return emptyChangedDigest
}

// SandboxExecutor is the deterministic executor injected into the Adapter.
// It contains no authorization logic and never follows symlinks.
type SandboxExecutor struct{}

func NewSandboxExecutor() *SandboxExecutor { return &SandboxExecutor{} }

func (e *SandboxExecutor) Edit(ctx context.Context, request EditRequest) (EditResult, error) {
	if ctx == nil || ctx.Err() != nil {
		return EditResult{}, context.Canceled
	}
	root, err := secureWorktreeRoot(request.Worktree)
	if err != nil {
		return EditResult{}, err
	}
	relative, err := secureRelativePath(request.RelativePath)
	if err != nil {
		return EditResult{}, err
	}
	target := filepath.Join(root, relative)
	if !pathWithin(root, target) {
		return EditResult{}, ErrExecutionPathOutside
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return EditResult{}, err
	}
	if err := ensureNoSymlinkComponents(root, filepath.Dir(target)); err != nil {
		return EditResult{}, err
	}
	if len(request.NewContentBytes) == 0 && request.NewContent != "" {
		request.NewContentBytes = []byte(request.NewContent)
	}
	if request.NewContentDigest != "" {
		sum := sha256.Sum256(request.NewContentBytes)
		if hex.EncodeToString(sum[:]) != request.NewContentDigest {
			return EditResult{}, fmt.Errorf("%w: content digest mismatch", ErrInvalidExecutionInput)
		}
	}
	// Atomic apply: exclusive temp file in the same directory, then rename.
	temp, err := os.CreateTemp(filepath.Dir(target), ".loom-edit-*")
	if err != nil {
		return EditResult{}, err
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = os.Remove(tempPath)
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return EditResult{}, err
	}
	if _, err := temp.Write(request.NewContentBytes); err != nil {
		_ = temp.Close()
		return EditResult{}, err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return EditResult{}, err
	}
	if err := temp.Close(); err != nil {
		return EditResult{}, err
	}
	if err := os.Rename(tempPath, target); err != nil {
		return EditResult{}, err
	}
	digest, err := changedFilesDigest(root)
	if err != nil {
		return EditResult{}, err
	}
	return EditResult{ChangedFilesDigest: digest, RelativePath: relative}, nil
}

func (e *SandboxExecutor) Run(ctx context.Context, request RunRequest) (RunResult, error) {
	if ctx == nil || ctx.Err() != nil {
		return RunResult{}, context.Canceled
	}
	root, err := secureWorktreeRoot(request.Worktree)
	if err != nil {
		return RunResult{}, err
	}
	if strings.TrimSpace(request.Command) == "" {
		return RunResult{}, ErrInvalidExecutionInput
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	if timeout > maxRunTimeout {
		timeout = maxRunTimeout
	}
	limit := request.OutputLimit
	if limit <= 0 {
		limit = defaultOutputLimit
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, "/bin/sh", "-c", request.Command)
	command.Dir = root
	command.Env = sanitizedEnv(root)
	var output limitedBuffer
	output.max = limit
	command.Stdout = &output
	command.Stderr = &output
	started := time.Now()
	runErr := command.Run()
	duration := time.Since(started).Milliseconds()
	if runCtx.Err() == context.DeadlineExceeded {
		return RunResult{}, ErrExecutionTimedOut
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return RunResult{
				ExitCode:           exitErr.ExitCode(),
				OutputDigest:       digestBytes(output.Bytes()),
				ChangedFilesDigest: mustChangedDigest(root),
				DurationMS:         duration,
			}, nil
		}
		return RunResult{}, runErr
	}
	changed, err := changedFilesDigest(root)
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{
		ExitCode:           0,
		OutputDigest:       digestBytes(output.Bytes()),
		ChangedFilesDigest: changed,
		DurationMS:         duration,
	}, nil
}

func sanitizedEnv(worktree string) []string {
	return []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"HOME=" + worktree,
		"LANG=C",
		"LC_ALL=C",
		"TMPDIR=" + worktree,
	}
}

type limitedBuffer struct {
	max  int64
	data []byte
	over int64
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	if int64(len(b.data)) < b.max {
		room := b.max - int64(len(b.data))
		if int64(len(p)) > room {
			p = p[:room]
		}
		b.data = append(b.data, p...)
	}
	b.over += int64(original - len(p))
	return original, nil
}

func (b *limitedBuffer) Bytes() []byte { return b.data }

func secureWorktreeRoot(worktree string) (string, error) {
	if worktree == "" {
		return "", ErrInvalidExecutionInput
	}
	abs, err := filepath.Abs(worktree)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", ErrInvalidExecutionInput
	}
	return abs, nil
}

func secureRelativePath(path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", ErrExecutionPathOutside
	}
	clean := filepath.Clean(path)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrExecutionPathOutside
	}
	return clean, nil
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative != "" && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func ensureNoSymlinkComponents(root, dir string) error {
	current := root
	relative, err := filepath.Rel(root, dir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrExecutionPathOutside
	}
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink component %s", ErrExecutionPathOutside, current)
		}
	}
	return nil
}

// changedFilesDigest returns a deterministic digest over every regular file
// under the worktree (excluding .git and symlinks). Limits are enforced by
// count and total bytes.
func changedFilesDigest(root string) (string, error) {
	var entries []string
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if entry.IsDir() {
			if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return nil
		}
		if len(entries) >= maxChangedFiles {
			return fmt.Errorf("%w: changed file count", ErrExecutionLimit)
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		total += info.Size()
		if total > maxChangedBytes {
			return fmt.Errorf("%w: changed file bytes", ErrExecutionLimit)
		}
		entries = append(entries, relative)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return emptyChangedDigest, nil
	}
	sort.Strings(entries)
	hash := sha256.New()
	for _, entry := range entries {
		content, readErr := os.ReadFile(filepath.Join(root, entry))
		if readErr != nil {
			return "", readErr
		}
		_, _ = hash.Write([]byte(entry))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(content)
		_, _ = hash.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func mustChangedDigest(root string) string {
	digest, err := changedFilesDigest(root)
	if err != nil {
		return emptyChangedDigest
	}
	return digest
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func marshalJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func cloneBytes(value []byte) []byte {
	return append([]byte(nil), value...)
}

var _ = bytes.Compare
var _ = io.EOF
